package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// CapitalHandler serves the capital page with cash transaction display.
type CapitalHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	apiURL       string
}

// NewCapitalHandler creates a new capital handler.
func NewCapitalHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error)) *CapitalHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &CapitalHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *CapitalHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// ServeHTTP renders the capital page.
func (h *CapitalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		"Page":             "capital",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"NavexaKeyMissing": navexaKeyMissing,
		"UserRole":         userRole,
		"PortalVersion":    config.GetVersion(),
		"ServerVersion":    GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "capital.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "capital.html").Str("error", err.Error()).Msg("failed to render capital page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
