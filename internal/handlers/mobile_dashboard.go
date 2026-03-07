package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// MobileDashboardHandler serves the mobile-optimized dashboard page.
type MobileDashboardHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	apiURL       string
	proxyGetFn   func(path, userID string) ([]byte, error)
}

// NewMobileDashboardHandler creates a new mobile dashboard handler.
func NewMobileDashboardHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error)) *MobileDashboardHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &MobileDashboardHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *MobileDashboardHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// SetProxyGetFn sets the proxy GET function for SSR data fetching.
func (h *MobileDashboardHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
	h.proxyGetFn = fn
}

// ServeHTTP renders the mobile dashboard page.
func (h *MobileDashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

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

	// Extract portfolio from URL path: /m/{portfolio}
	var urlPortfolio string
	if pathSuffix := strings.TrimPrefix(r.URL.Path, "/m"); len(pathSuffix) > 1 && pathSuffix[0] == '/' {
		decoded, err := url.PathUnescape(pathSuffix[1:])
		if err == nil {
			urlPortfolio = decoded
		}
	}

	var portfoliosJSON, portfolioJSON, timelineJSON, selectedJSON template.JS
	portfoliosJSON = "null"
	portfolioJSON = "null"
	timelineJSON = "null"
	selectedJSON = `""`
	selectedPortfolio := ""

	if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
		ssrStart := time.Now()

		// 1. Fetch portfolio list
		t1 := time.Now()
		if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
			portfoliosJSON = template.JS(body)

			if h.logger != nil {
				h.logger.Info().Int64("duration_ms", time.Since(t1).Milliseconds()).Msg("mobile SSR: portfolios")
			}

			var pData struct {
				Portfolios []struct {
					Name string `json:"name"`
				} `json:"portfolios"`
				Default string `json:"default"`
			}
			if json.Unmarshal(body, &pData) == nil {
				selected := urlPortfolio
				if selected != "" {
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
					escapedName := url.PathEscape(selected)
					userID := claims.Sub
					var wg sync.WaitGroup
					wg.Add(2)

					go func() {
						defer wg.Done()
						t2 := time.Now()
						if pBody, err := h.proxyGetFn("/api/portfolios/"+escapedName, userID); err == nil {
							portfolioJSON = template.JS(pBody)
							if h.logger != nil {
								h.logger.Info().Int64("duration_ms", time.Since(t2).Milliseconds()).Str("portfolio", selected).Msg("mobile SSR: portfolio data")
							}
						} else if h.logger != nil {
							h.logger.Warn().Int64("duration_ms", time.Since(t2).Milliseconds()).Str("portfolio", selected).Str("error", err.Error()).Msg("mobile SSR: portfolio data failed")
						}
					}()

					go func() {
						defer wg.Done()
						t3 := time.Now()
						if tBody, err := h.proxyGetFn("/api/portfolios/"+escapedName+"/timeline", userID); err == nil {
							timelineJSON = template.JS(tBody)
							if h.logger != nil {
								h.logger.Info().Int64("duration_ms", time.Since(t3).Milliseconds()).Str("portfolio", selected).Msg("mobile SSR: timeline")
							}
						} else if h.logger != nil {
							h.logger.Warn().Int64("duration_ms", time.Since(t3).Milliseconds()).Str("portfolio", selected).Str("error", err.Error()).Msg("mobile SSR: timeline failed")
						}
					}()

					wg.Wait()
				}
			}
		} else if h.logger != nil {
			h.logger.Warn().Int64("duration_ms", time.Since(t1).Milliseconds()).Str("error", err.Error()).Msg("mobile SSR: portfolios failed")
		}

		if h.logger != nil {
			h.logger.Info().Int64("duration_ms", time.Since(ssrStart).Milliseconds()).Msg("mobile SSR: total")
		}
	}

	data := map[string]interface{}{
		"Page":              "mobile",
		"DevMode":           h.devMode,
		"LoggedIn":          loggedIn,
		"NavexaKeyMissing":  navexaKeyMissing,
		"UserRole":          userRole,
		"PortalVersion":     config.GetVersion(),
		"ServerVersion":     GetServerVersion(h.apiURL),
		"PortfoliosJSON":    portfoliosJSON,
		"PortfolioJSON":     portfolioJSON,
		"TimelineJSON":      timelineJSON,
		"SelectedPortfolio": selectedPortfolio,
		"SelectedJSON":      selectedJSON,
	}

	if err := h.templates.ExecuteTemplate(w, "mobile.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "mobile.html").Str("error", err.Error()).Msg("failed to render mobile dashboard")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
