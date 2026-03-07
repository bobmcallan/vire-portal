package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// PageHandler serves HTML pages rendered with Go templates.
type PageHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	apiURL       string
	userLookupFn func(string) (*client.UserProfile, error)
	proxyGetFn   func(path, userID string) ([]byte, error)
}

// NewPageHandler creates a new page handler that loads templates from the pages directory.
func NewPageHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error)) *PageHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &PageHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *PageHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// SetProxyGetFn sets the proxy GET function for SSR data fetching.
func (h *PageHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
	h.proxyGetFn = fn
}

// isMobileBrowser checks the User-Agent string for common mobile device patterns.
func isMobileBrowser(ua string) bool {
	ua = strings.ToLower(ua)
	mobileKeywords := []string{"iphone", "android", "mobile", "ipod", "windows phone", "blackberry", "opera mini", "iemobile"}
	for _, kw := range mobileKeywords {
		if strings.Contains(ua, kw) {
			// Exclude tablets (iPad, Android tablet without "mobile")
			if strings.Contains(ua, "ipad") {
				return false
			}
			return true
		}
	}
	return false
}

// FindPagesDir locates the pages directory.
func FindPagesDir() string {
	dirs := []string{
		"./pages",
		"../pages",
		"../../pages",
		".",
	}

	for _, dir := range dirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	return "."
}

// ServePage creates a handler function for serving a specific page template.
func (h *PageHandler) ServePage(templateName string, pageName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

		// Auto-logout on landing page: clear session cookie
		if pageName == "home" {
			http.SetCookie(w, &http.Cookie{
				Name:     "vire_session",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			loggedIn = false
		}

		var userRole string
		if loggedIn && h.userLookupFn != nil && claims != nil && claims.Sub != "" {
			if user, err := h.userLookupFn(claims.Sub); err == nil && user != nil {
				userRole = user.Role
			}
		}

		data := map[string]interface{}{
			"Page":          pageName,
			"DevMode":       h.devMode,
			"LoggedIn":      loggedIn,
			"UserRole":      userRole,
			"PortalVersion": config.GetVersion(),
			"ServerVersion": GetServerVersion(h.apiURL),
		}

		if err := h.templates.ExecuteTemplate(w, templateName, data); err != nil {
			if h.logger != nil {
				h.logger.Error().Str("template", templateName).Str("error", err.Error()).Msg("failed to render page")
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// ServeErrorPage renders the error page with server-side resolved error message.
func (h *PageHandler) ServeErrorPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
		var userRole string
		if loggedIn && h.userLookupFn != nil && claims != nil && claims.Sub != "" {
			if user, err := h.userLookupFn(claims.Sub); err == nil && user != nil {
				userRole = user.Role
			}
		}

		reason := r.URL.Query().Get("reason")
		messages := map[string]string{
			"server_unavailable":  "The authentication server is unavailable. Please try again shortly.",
			"auth_failed":         "Authentication failed. Please try again.",
			"invalid_credentials": "Invalid username or password.",
			"missing_credentials": "Please provide both username and password.",
			"bad_request":         "Bad request. Please try again.",
		}
		msg := messages[reason]
		if msg == "" {
			msg = "Something went wrong. Please try again."
		}

		data := map[string]interface{}{
			"Page":          "error",
			"DevMode":       h.devMode,
			"LoggedIn":      loggedIn,
			"UserRole":      userRole,
			"PortalVersion": config.GetVersion(),
			"ServerVersion": GetServerVersion(h.apiURL),
			"ErrorMessage":  msg,
		}
		h.templates.ExecuteTemplate(w, "error.html", data)
	}
}

// ServeLandingPage renders the landing page with server-side health check.
func (h *PageHandler) ServeLandingPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Auto-logout: clear session cookie
		http.SetCookie(w, &http.Cookie{
			Name: "vire_session", Value: "", Path: "/",
			MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode,
		})

		serverUp := checkServerHealth(h.apiURL)

		data := map[string]interface{}{
			"Page":          "home",
			"DevMode":       h.devMode,
			"LoggedIn":      false,
			"UserRole":      "",
			"PortalVersion": config.GetVersion(),
			"ServerVersion": GetServerVersion(h.apiURL),
			"ServerStatus":  serverUp,
		}
		h.templates.ExecuteTemplate(w, "landing.html", data)
	}
}

func checkServerHealth(apiURL string) bool {
	if apiURL == "" {
		return false
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(apiURL + "/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// GlossaryTerm represents a single term in the glossary.
type GlossaryTerm struct {
	Term       string `json:"term"`
	Label      string `json:"label"`
	Definition string `json:"definition"`
	Formula    string `json:"formula"`
}

// GlossaryCategory represents a category of glossary terms.
type GlossaryCategory struct {
	Name  string         `json:"name"`
	Terms []GlossaryTerm `json:"terms"`
}

// ServeGlossaryPage renders the glossary page with server-side fetched data.
func (h *PageHandler) ServeGlossaryPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
		var userRole string
		if loggedIn && h.userLookupFn != nil && claims != nil && claims.Sub != "" {
			if user, err := h.userLookupFn(claims.Sub); err == nil && user != nil {
				userRole = user.Role
			}
		}

		var categories []GlossaryCategory
		var fetchError string
		if h.proxyGetFn != nil {
			body, err := h.proxyGetFn("/api/glossary", "")
			if err != nil {
				fetchError = "Glossary data is not yet available."
			} else {
				var resp struct {
					Categories []GlossaryCategory `json:"categories"`
				}
				if json.Unmarshal(body, &resp) == nil {
					categories = resp.Categories
				}
			}
		}

		data := map[string]interface{}{
			"Page":          "glossary",
			"DevMode":       h.devMode,
			"LoggedIn":      loggedIn,
			"UserRole":      userRole,
			"PortalVersion": config.GetVersion(),
			"ServerVersion": GetServerVersion(h.apiURL),
			"Categories":    categories,
			"FetchError":    fetchError,
			"TermParam":     r.URL.Query().Get("term"),
		}
		h.templates.ExecuteTemplate(w, "glossary.html", data)
	}
}

// ServeChangelogPage renders the changelog page with JSON hydration.
func (h *PageHandler) ServeChangelogPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
		var userRole string
		if loggedIn && h.userLookupFn != nil && claims != nil && claims.Sub != "" {
			if user, err := h.userLookupFn(claims.Sub); err == nil && user != nil {
				userRole = user.Role
			}
		}

		var entriesJSON template.JS = "[]"
		if h.proxyGetFn != nil {
			body, err := h.proxyGetFn("/api/changelog?per_page=100&page=1", "")
			if err == nil {
				var resp struct {
					Items json.RawMessage `json:"items"`
				}
				if json.Unmarshal(body, &resp) == nil && resp.Items != nil {
					entriesJSON = template.JS(resp.Items)
				}
			}
		}

		data := map[string]interface{}{
			"Page":          "changelog",
			"DevMode":       h.devMode,
			"LoggedIn":      loggedIn,
			"UserRole":      userRole,
			"PortalVersion": config.GetVersion(),
			"ServerVersion": GetServerVersion(h.apiURL),
			"EntriesJSON":   entriesJSON,
		}
		h.templates.ExecuteTemplate(w, "changelog.html", data)
	}
}

// ServeHelpPage renders the help page with JSON hydration for feedback.
func (h *PageHandler) ServeHelpPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
		if !loggedIn {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		var userRole string
		if h.userLookupFn != nil && claims != nil && claims.Sub != "" {
			if user, err := h.userLookupFn(claims.Sub); err == nil && user != nil {
				userRole = user.Role
			}
		}

		var feedbackJSON template.JS = "[]"
		var feedbackTotal int
		if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
			body, err := h.proxyGetFn("/api/feedback?per_page=50", claims.Sub)
			if err == nil {
				var resp struct {
					Items json.RawMessage `json:"items"`
					Total int             `json:"total"`
				}
				if json.Unmarshal(body, &resp) == nil {
					if resp.Items != nil {
						feedbackJSON = template.JS(resp.Items)
					}
					feedbackTotal = resp.Total
				}
			}
		}

		data := map[string]interface{}{
			"Page":          "help",
			"DevMode":       h.devMode,
			"LoggedIn":      loggedIn,
			"UserRole":      userRole,
			"PortalVersion": config.GetVersion(),
			"ServerVersion": GetServerVersion(h.apiURL),
			"FeedbackJSON":  feedbackJSON,
			"FeedbackTotal": feedbackTotal,
		}
		h.templates.ExecuteTemplate(w, "help.html", data)
	}
}

// StaticFileHandler serves static files (CSS, JS, images).
func (h *PageHandler) StaticFileHandler(w http.ResponseWriter, r *http.Request) {
	pagesDir := FindPagesDir()
	staticDir := filepath.Join(pagesDir, "static")

	// Remove /static/ prefix from URL path
	path := r.URL.Path[len("/static/"):]
	fullPath := filepath.Join(staticDir, path)

	// Security: prevent directory traversal
	absStaticDir, _ := filepath.Abs(staticDir)
	absFullPath, _ := filepath.Abs(fullPath)
	if len(absFullPath) < len(absStaticDir) || absFullPath[:len(absStaticDir)] != absStaticDir {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, fullPath)
}
