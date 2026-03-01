package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// DashboardHandler serves the dashboard page with portfolio management UI.
type DashboardHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	apiURL       string
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error)) *DashboardHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &DashboardHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *DashboardHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// ServeHTTP renders the dashboard page.
func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

	// Redirect unauthenticated users to landing page
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	var userRole string
	navexaKeyMissing := false
	if h.userLookupFn != nil && claims != nil && claims.Sub != "" {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil {
			if !user.NavexaKeySet {
				navexaKeyMissing = true
			}
			userRole = user.Role
		}
	}

	data := map[string]interface{}{
		"Page":             "dashboard",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"NavexaKeyMissing": navexaKeyMissing,
		"UserRole":         userRole,
		"PortalVersion":    config.GetVersion(),
		"ServerVersion":    GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "dashboard.html").Str("error", err.Error()).Msg("failed to render dashboard")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
