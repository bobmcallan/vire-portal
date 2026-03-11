package handlers

import (
	"html/template"
	"net/http"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// AdminGeminiHandler serves the admin Gemini usage page.
type AdminGeminiHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	proxyGetFn   func(path, userID string) ([]byte, error)
	apiURL       string
}

// NewAdminGeminiHandler creates a new admin Gemini handler.
func NewAdminGeminiHandler(
	logger *common.Logger,
	devMode bool,
	jwtSecret []byte,
	userLookupFn func(string) (*client.UserProfile, error),
) *AdminGeminiHandler {
	pagesDir := FindPagesDir()

	templates := ParseTemplates(pagesDir)

	return &AdminGeminiHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *AdminGeminiHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// SetProxyGetFn sets the proxy GET function for SSR data fetching.
func (h *AdminGeminiHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
	h.proxyGetFn = fn
}

// ServeHTTP renders the admin Gemini usage page.
func (h *AdminGeminiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Gate: require admin role
	var userRole string
	if claims != nil && claims.Sub != "" && h.userLookupFn != nil {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil {
			userRole = user.Role
		}
	}
	if userRole != "admin" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	// Fetch usage data for default period (week)
	var usageJSON template.JS = "null"
	if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
		if body, err := h.proxyGetFn("/api/admin/gemini/usage?period=week", claims.Sub); err == nil {
			usageJSON = SafeJS(body)
		}
	}

	data := map[string]interface{}{
		"Page":          "gemini",
		"DevMode":       h.devMode,
		"LoggedIn":      loggedIn,
		"UserRole":      userRole,
		"UsageJSON":     usageJSON,
		"PortalVersion": config.GetVersion(),
		"ServerVersion": GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "gemini.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "gemini.html").Str("error", err.Error()).Msg("failed to render gemini page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
