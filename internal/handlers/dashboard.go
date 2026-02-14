package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

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
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	port         int
	catalogFn    func() []DashboardTool
	configStatus DashboardConfigStatus
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(logger *common.Logger, devMode bool, port int, catalogFn func() []DashboardTool) *DashboardHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &DashboardHandler{
		logger:    logger,
		templates: templates,
		devMode:   devMode,
		port:      port,
		catalogFn: catalogFn,
	}
}

// SetConfigStatus sets the config status for display on the dashboard.
func (h *DashboardHandler) SetConfigStatus(status DashboardConfigStatus) {
	h.configStatus = status
}

// ServeHTTP renders the dashboard page.
func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tools := h.catalogFn()

	toolCount := len(tools)
	toolStatus := "NO TOOLS"
	if toolCount > 0 {
		toolStatus = fmt.Sprintf("%d", toolCount)
	}

	mcpEndpoint := fmt.Sprintf("http://localhost:%d/mcp", h.port)

	data := map[string]interface{}{
		"Page":        "dashboard",
		"DevMode":     h.devMode,
		"Tools":       tools,
		"ToolCount":   toolCount,
		"ToolStatus":  toolStatus,
		"MCPEndpoint": mcpEndpoint,
		"Port":        h.port,
		"Config":      h.configStatus,
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "dashboard.html").Str("error", err.Error()).Msg("failed to render dashboard")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
