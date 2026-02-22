package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// MCPPageTool holds display-only fields for a tool on the MCP page.
type MCPPageTool struct {
	Name        string
	Description string
	Method      string
	Path        string
}

// MCPPageHandler serves the MCP info page showing connection details and tools.
type MCPPageHandler struct {
	logger         *common.Logger
	templates      *template.Template
	devMode        bool
	port           int
	jwtSecret      []byte
	catalogFn      func() []MCPPageTool
	devMCPEndpoint func(userID string) string
	apiURL         string
}

// NewMCPPageHandler creates a new MCP page handler.
func NewMCPPageHandler(logger *common.Logger, devMode bool, port int, jwtSecret []byte, catalogFn func() []MCPPageTool) *MCPPageHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &MCPPageHandler{
		logger:    logger,
		templates: templates,
		devMode:   devMode,
		port:      port,
		jwtSecret: jwtSecret,
		catalogFn: catalogFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *MCPPageHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// SetDevMCPEndpointFn sets the function to generate dev-mode MCP endpoints.
func (h *MCPPageHandler) SetDevMCPEndpointFn(fn func(userID string) string) {
	h.devMCPEndpoint = fn
}

// ServeHTTP renders the MCP info page.
func (h *MCPPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

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

	var devMCPEndpoint string
	if h.devMCPEndpoint != nil && claims != nil && claims.Sub != "" {
		devMCPEndpoint = h.devMCPEndpoint(claims.Sub)
	}

	data := map[string]interface{}{
		"Page":           "mcp",
		"DevMode":        h.devMode,
		"LoggedIn":       loggedIn,
		"Tools":          tools,
		"ToolCount":      toolCount,
		"ToolStatus":     toolStatus,
		"MCPEndpoint":    mcpEndpoint,
		"DevMCPEndpoint": devMCPEndpoint,
		"Port":           h.port,
		"PortalVersion":  config.GetVersion(),
		"ServerVersion":  GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "mcp.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "mcp.html").Str("error", err.Error()).Msg("failed to render MCP page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
