package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

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

	// Extract portfolio from URL path: /dashboard/{portfolio}
	var urlPortfolio string
	if pathSuffix := strings.TrimPrefix(r.URL.Path, "/dashboard"); len(pathSuffix) > 1 && pathSuffix[0] == '/' {
		decoded, err := url.PathUnescape(pathSuffix[1:])
		if err == nil {
			urlPortfolio = decoded
		}
	}

	var portfoliosJSON, portfolioJSON, timelineJSON, watchlistJSON, glossaryJSON, selectedJSON template.JS
	portfoliosJSON = "null"
	portfolioJSON = "null"
	timelineJSON = "null"
	watchlistJSON = "null"
	glossaryJSON = "null"
	selectedJSON = `""`
	selectedPortfolio := ""

	if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
		ssrStart := time.Now()

		// 1. Fetch portfolio list
		t1 := time.Now()
		if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
			portfoliosJSON = template.JS(body)

			if h.logger != nil {
				h.logger.Info().Int64("duration_ms", time.Since(t1).Milliseconds()).Msg("dashboard SSR: portfolios")
			}

			// Parse to find default/selected portfolio name
			var pData struct {
				Portfolios []struct {
					Name string `json:"name"`
				} `json:"portfolios"`
				Default string `json:"default"`
			}
			if json.Unmarshal(body, &pData) == nil {
				// Priority: URL path > default > first portfolio
				selected := urlPortfolio
				if selected != "" {
					// Validate URL portfolio exists in the list
					found := false
					for _, p := range pData.Portfolios {
						if p.Name == selected {
							found = true
							break
						}
					}
					if !found {
						selected = ""
					}
				}
				if selected == "" {
					selected = pData.Default
				}
				if selected == "" && len(pData.Portfolios) > 0 {
					selected = pData.Portfolios[0].Name
				}
				selectedPortfolio = selected
				if b, err := json.Marshal(selected); err == nil {
					selectedJSON = template.JS(b)
				}
				if selected != "" {
					// 2. Fetch portfolio data (holdings, metrics, changes, breadth)
					t2 := time.Now()
					if pBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected), claims.Sub); err == nil {
						portfolioJSON = template.JS(pBody)
					}
					if h.logger != nil {
						h.logger.Info().Int64("duration_ms", time.Since(t2).Milliseconds()).Str("portfolio", selected).Msg("dashboard SSR: portfolio data")
					}

					// 3. Fetch timeline (growth chart data)
					t3 := time.Now()
					if tBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/timeline", claims.Sub); err == nil {
						timelineJSON = template.JS(tBody)
					}
					if h.logger != nil {
						h.logger.Info().Int64("duration_ms", time.Since(t3).Milliseconds()).Str("portfolio", selected).Msg("dashboard SSR: timeline")
					}

					// 4. Fetch watchlist
					t4 := time.Now()
					if wBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/watchlist", claims.Sub); err == nil {
						watchlistJSON = template.JS(wBody)
					}
					if h.logger != nil {
						h.logger.Info().Int64("duration_ms", time.Since(t4).Milliseconds()).Str("portfolio", selected).Msg("dashboard SSR: watchlist")
					}
				}
			}
		} else if h.logger != nil {
			h.logger.Warn().Int64("duration_ms", time.Since(t1).Milliseconds()).Str("error", err.Error()).Msg("dashboard SSR: portfolios failed")
		}

		// 5. Fetch glossary (independent of portfolio selection)
		t5 := time.Now()
		if gBody, err := h.proxyGetFn("/api/glossary", claims.Sub); err == nil {
			glossaryJSON = template.JS(gBody)
		}
		if h.logger != nil {
			h.logger.Info().Int64("duration_ms", time.Since(t5).Milliseconds()).Msg("dashboard SSR: glossary")
			h.logger.Info().Int64("duration_ms", time.Since(ssrStart).Milliseconds()).Msg("dashboard SSR: total")
		}
	}

	data := map[string]interface{}{
		"Page":              "dashboard",
		"DevMode":           h.devMode,
		"LoggedIn":          loggedIn,
		"NavexaKeyMissing":  navexaKeyMissing,
		"UserRole":          userRole,
		"PortalVersion":     config.GetVersion(),
		"ServerVersion":     GetServerVersion(h.apiURL),
		"PortfoliosJSON":    portfoliosJSON,
		"PortfolioJSON":     portfolioJSON,
		"TimelineJSON":      timelineJSON,
		"WatchlistJSON":     watchlistJSON,
		"GlossaryJSON":      glossaryJSON,
		"SelectedPortfolio": selectedPortfolio,
		"SelectedJSON":      selectedJSON,
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "dashboard.html").Str("error", err.Error()).Msg("failed to render dashboard")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
