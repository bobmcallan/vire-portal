package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// StrategyHandler serves the strategy page with portfolio strategy and plan editors.
type StrategyHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	apiURL       string
}

// NewStrategyHandler creates a new strategy handler.
func NewStrategyHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error)) *StrategyHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &StrategyHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *StrategyHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// ServeHTTP renders the strategy page.
func (h *StrategyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

	// Redirect unauthenticated users to landing page
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	navexaKeyMissing := false
	if h.userLookupFn != nil && claims != nil && claims.Sub != "" {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil && !user.NavexaKeySet {
			navexaKeyMissing = true
		}
	}

	data := map[string]interface{}{
		"Page":             "strategy",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"NavexaKeyMissing": navexaKeyMissing,
		"PortalVersion":    config.GetVersion(),
		"ServerVersion":    GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "strategy.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "strategy.html").Str("error", err.Error()).Msg("failed to render strategy page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
