package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// StockHandler serves the per-stock detail page.
type StockHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	apiURL       string
	proxyGetFn   func(path, userID string) ([]byte, error)
}

// NewStockHandler creates a new stock handler.
func NewStockHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error)) *StockHandler {
	pagesDir := FindPagesDir()

	templates := ParseTemplates(pagesDir)

	return &StockHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *StockHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// SetProxyGetFn sets the proxy GET function for SSR data fetching.
func (h *StockHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
	h.proxyGetFn = fn
}

// ServeHTTP renders the stock detail page.
func (h *StockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Extract ticker from URL path: /stock/{ticker...}
	ticker := strings.TrimPrefix(r.URL.Path, "/stock/")
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	var userRole string
	if h.userLookupFn != nil && claims != nil && claims.Sub != "" {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil {
			userRole = user.Role
		}
	}

	var holdingJSON, portfolioNameJSON, tickerJSON template.JS
	var stockDetailJSON template.JS = "null"
	holdingJSON = "null"
	portfolioNameJSON = "null"
	tickerJSON = template.JS(`"` + template.JSEscapeString(ticker) + `"`)

	var selected string
	if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
		if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
			var pData struct {
				Portfolios []struct {
					Name string `json:"name"`
				} `json:"portfolios"`
				Default string `json:"default"`
			}
			if json.Unmarshal(body, &pData) == nil {
				selected = pData.Default
				if selected == "" && len(pData.Portfolios) > 0 {
					selected = pData.Portfolios[0].Name
				}
				if selected != "" {
					portfolioNameJSON = template.JS(`"` + template.JSEscapeString(selected) + `"`)
					if pBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected), claims.Sub); err == nil {
						var portfolio struct {
							Holdings []json.RawMessage `json:"holdings"`
						}
						if json.Unmarshal(pBody, &portfolio) == nil {
							for _, raw := range portfolio.Holdings {
								var hold struct {
									Ticker string `json:"ticker"`
								}
								if json.Unmarshal(raw, &hold) == nil && strings.EqualFold(hold.Ticker, ticker) {
									holdingJSON = SafeJS(raw)
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// Fetch stock detail data (candles, filings, news, fundamentals)
	if h.proxyGetFn != nil && claims != nil && claims.Sub != "" && selected != "" {
		path := "/api/portfolios/" + url.PathEscape(selected) + "/stock/" + url.PathEscape(ticker) + "/detail"
		if body, err := h.proxyGetFn(path, claims.Sub); err == nil {
			stockDetailJSON = SafeJS(body)
		}
	}

	data := map[string]interface{}{
		"Page":              "stock",
		"DevMode":           h.devMode,
		"LoggedIn":          loggedIn,
		"UserRole":          userRole,
		"PortalVersion":     config.GetVersion(),
		"ServerVersion":     GetServerVersion(h.apiURL),
		"Ticker":            ticker,
		"HoldingJSON":       holdingJSON,
		"PortfolioNameJSON": portfolioNameJSON,
		"TickerJSON":        tickerJSON,
		"StockDetailJSON":   stockDetailJSON,
		"TokenExpiry":       claims.Exp,
	}

	if err := h.templates.ExecuteTemplate(w, "stock.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "stock.html").Str("error", err.Error()).Msg("failed to render stock page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
