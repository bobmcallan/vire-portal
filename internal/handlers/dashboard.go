package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// DashboardTool holds display-only fields for a tool in the dashboard.
type DashboardTool struct {
	Name        string
	Description string
	Method      string
	Path        string
}

// DashboardConfigStatus holds config status flags for the dashboard display.
type DashboardConfigStatus struct {
	Portfolios string
}

// DashboardHandler serves the dashboard page with dynamic data.
type DashboardHandler struct {
	logger         *common.Logger
	templates      *template.Template
	devMode        bool
	port           int
	jwtSecret      []byte
	catalogFn      func() []DashboardTool
	configStatus   DashboardConfigStatus
	userLookupFn   func(string) (*client.UserProfile, error)
	devMCPEndpoint func(userID string) string
	apiURL         string
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(logger *common.Logger, devMode bool, port int, jwtSecret []byte, catalogFn func() []DashboardTool, userLookupFn func(string) (*client.UserProfile, error)) *DashboardHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &DashboardHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		port:         port,
		jwtSecret:    jwtSecret,
		catalogFn:    catalogFn,
		userLookupFn: userLookupFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *DashboardHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// SetConfigStatus sets the config status for display on the dashboard.
func (h *DashboardHandler) SetConfigStatus(status DashboardConfigStatus) {
	h.configStatus = status
}

// SetDevMCPEndpointFn sets the function to generate dev-mode MCP endpoints.
func (h *DashboardHandler) SetDevMCPEndpointFn(fn func(userID string) string) {
	h.devMCPEndpoint = fn
}

// ServeHTTP renders the dashboard page.
func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

	// Redirect unauthenticated users to landing page
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	tools := h.catalogFn()

	toolCount := len(tools)
	toolStatus := "NO TOOLS"
	if toolCount > 0 {
		toolStatus = fmt.Sprintf("%d", toolCount)
	}

	mcpEndpoint := fmt.Sprintf("http://localhost:%d/mcp", h.port)

	navexaKeyMissing := false
	var devMCPEndpoint string
	if h.userLookupFn != nil && claims != nil && claims.Sub != "" {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil && !user.NavexaKeySet {
			navexaKeyMissing = true
		}
		if h.devMCPEndpoint != nil {
			devMCPEndpoint = h.devMCPEndpoint(claims.Sub)
		}
	}

	data := map[string]interface{}{
		"Page":             "dashboard",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"Tools":            tools,
		"ToolCount":        toolCount,
		"ToolStatus":       toolStatus,
		"MCPEndpoint":      mcpEndpoint,
		"DevMCPEndpoint":   devMCPEndpoint,
		"Port":             h.port,
		"Config":           h.configStatus,
		"NavexaKeyMissing": navexaKeyMissing,
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
