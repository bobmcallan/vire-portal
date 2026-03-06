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

// DashboardHandler serves the dashboard page with portfolio management UI.
type DashboardHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	apiURL       string
	proxyGetFn   func(path, userID string) ([]byte, error)
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

// SetProxyGetFn sets the proxy GET function for SSR data fetching.
func (h *DashboardHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
	h.proxyGetFn = fn
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

	var portfoliosJSON, portfolioJSON, timelineJSON, watchlistJSON, glossaryJSON template.JS
	portfoliosJSON = "null"
	portfolioJSON = "null"
	timelineJSON = "null"
	watchlistJSON = "null"
	glossaryJSON = "null"

	if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
		// 1. Fetch portfolio list
		if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
			portfoliosJSON = template.JS(body)

			// Parse to find default/selected portfolio name
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
					// 2. Fetch portfolio data (holdings, metrics, changes, breadth)
					if pBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected), claims.Sub); err == nil {
						portfolioJSON = template.JS(pBody)
					}
					// 3. Fetch timeline (growth chart data)
					if tBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/timeline", claims.Sub); err == nil {
						timelineJSON = template.JS(tBody)
					}
					// 4. Fetch watchlist
					if wBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/watchlist", claims.Sub); err == nil {
						watchlistJSON = template.JS(wBody)
					}
				}
			}
		}
		// 5. Fetch glossary (independent of portfolio selection)
		if gBody, err := h.proxyGetFn("/api/glossary", claims.Sub); err == nil {
			glossaryJSON = template.JS(gBody)
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
		"PortfoliosJSON":   portfoliosJSON,
		"PortfolioJSON":    portfolioJSON,
		"TimelineJSON":     timelineJSON,
		"WatchlistJSON":    watchlistJSON,
		"GlossaryJSON":     glossaryJSON,
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "dashboard.html").Str("error", err.Error()).Msg("failed to render dashboard")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
