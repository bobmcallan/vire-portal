package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bobmcallan/vire-portal/internal/config"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Handler is the HTTP handler for the MCP endpoint.
// It wraps mcp-go's StreamableHTTPServer and delegates to it.
type Handler struct {
	streamable *mcpserver.StreamableHTTPServer
	logger     *slog.Logger
}

// catalogRetryAttempts is the number of times to retry fetching the catalog.
const catalogRetryAttempts = 3

// catalogRetryDelay is the delay between retry attempts.
const catalogRetryDelay = 2 * time.Second

// NewHandler creates a new MCP handler with dynamic tool registration from vire-server.
func NewHandler(cfg *config.Config, logger *slog.Logger) *Handler {
	mcpSrv := mcpserver.NewMCPServer(
		"vire-portal",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	proxy := NewMCPProxy(cfg.API.URL, logger, cfg)

	// Fetch tool catalog from vire-server with retry (non-fatal if unreachable)
	var catalog []CatalogTool
	var fetchErr error
	for attempt := 1; attempt <= catalogRetryAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		catalog, fetchErr = proxy.FetchCatalog(ctx)
		cancel()
		if fetchErr == nil {
			break
		}
		logger.Warn("failed to fetch tool catalog, retrying",
			"attempt", attempt, "max_attempts", catalogRetryAttempts,
			"error", fetchErr, "api_url", cfg.API.URL)
		if attempt < catalogRetryAttempts {
			time.Sleep(catalogRetryDelay)
		}
	}

	var toolCount int
	if fetchErr != nil {
		logger.Warn("failed to fetch tool catalog after retries, starting with 0 tools",
			"attempts", catalogRetryAttempts, "error", fetchErr, "api_url", cfg.API.URL)
	} else {
		validated := ValidateCatalog(catalog, logger)
		toolCount = RegisterToolsFromCatalog(mcpSrv, proxy, validated)
	}

	streamable := mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithStateLess(true),
	)

	logger.Info("MCP handler initialized",
		"tools", toolCount,
		"api_url", cfg.API.URL,
	)

	return &Handler{
		streamable: streamable,
		logger:     logger,
	}
}

// ServeHTTP delegates to the mcp-go StreamableHTTPServer.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.streamable.ServeHTTP(w, r)
}
