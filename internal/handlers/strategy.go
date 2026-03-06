package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
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
	proxyGetFn   func(path, userID string) ([]byte, error)
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

// SetProxyGetFn sets the proxy GET function for SSR data fetching.
func (h *StrategyHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
	h.proxyGetFn = fn
}

// ServeHTTP renders the strategy page.
func (h *StrategyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var portfoliosJSON, strategyJSON, planJSON template.JS
	portfoliosJSON = "null"
	strategyJSON = "null"
	planJSON = "null"
	if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
		if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
			portfoliosJSON = template.JS(body)
			var pData struct {
				Portfolios []struct {
					Name string `json:"name"`
				} `json:"portfolios"`
				Default string `json:"default"`
			}
			if json.Unmarshal(body, &pData) == nil {
				selected := pData.Default
				if selected == "" && len(pData.Portfolios) > 0 {
					selected = pData.Portfolios[0].Name
				}
				if selected != "" {
					if sBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/strategy", claims.Sub); err == nil {
						strategyJSON = template.JS(sBody)
					}
					if pBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/plan", claims.Sub); err == nil {
						planJSON = template.JS(pBody)
					}
				}
			}
		}
	}

	data := map[string]interface{}{
		"Page":             "strategy",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"NavexaKeyMissing": navexaKeyMissing,
		"UserRole":         userRole,
		"PortalVersion":    config.GetVersion(),
		"ServerVersion":    GetServerVersion(h.apiURL),
		"PortfoliosJSON":   portfoliosJSON,
		"StrategyJSON":     strategyJSON,
		"PlanJSON":         planJSON,
	}

	if err := h.templates.ExecuteTemplate(w, "strategy.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "strategy.html").Str("error", err.Error()).Msg("failed to render strategy page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
