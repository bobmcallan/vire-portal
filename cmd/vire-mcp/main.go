// Command vire-mcp is a thin stdio MCP server that reuses internal/mcp.
// It fetches the tool catalog from vire-server, registers tools dynamically,
// and serves over stdio for Claude Desktop integration.
//
// Configuration is via environment variables only (no TOML, no flags):
//
//	VIRE_API_URL             vire-server URL       (default: http://localhost:4242)
//	VIRE_DEFAULT_PORTFOLIO   default portfolio name (optional)
//	VIRE_DISPLAY_CURRENCY    display currency       (optional)
//	VIRE_LOG_LEVEL           log level              (default: warn)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/mcp"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

func main() {
	cfg := buildConfigFromEnv()

	// Logger writes to stderr only — stdout is reserved for stdio MCP transport.
	logger := common.NewLoggerFromConfig(common.LoggingConfig{
		Level:   cfg.Logging.Level,
		Outputs: []string{"console"},
	})

	common.LoadVersionFromFile()

	proxy := mcp.NewMCPProxy(cfg.API.URL, logger, cfg)

	// Fetch tool catalog from vire-server.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	catalog, err := proxy.FetchCatalog(ctx)
	cancel()
	if err != nil {
		logger.Warn().Str("error", err.Error()).Msg("failed to fetch tool catalog, starting with 0 tools")
		catalog = nil
	}

	validated := mcp.ValidateCatalog(catalog, logger)

	mcpSrv := server.NewMCPServer(
		"vire",
		common.GetVersion(),
		server.WithToolCapabilities(true),
	)

	toolCount := mcp.RegisterToolsFromCatalog(mcpSrv, proxy, validated)
	logger.Info().Int("tools", toolCount).Str("api_url", cfg.API.URL).Msg("vire-mcp ready")

	if err := server.ServeStdio(mcpSrv); err != nil {
		fmt.Fprintf(os.Stderr, "stdio server error: %v\n", err)
		os.Exit(1)
	}
}

// buildConfigFromEnv creates a config.Config from environment variables.
func buildConfigFromEnv() *config.Config {
	cfg := config.NewDefaultConfig()

	// Override defaults for vire-mcp context.
	cfg.API.URL = "http://localhost:4242"
	cfg.Logging.Level = "warn"
	cfg.Logging.Outputs = []string{"console"}

	// Apply VIRE_* env overrides (reuses the same logic as the portal).
	if v := os.Getenv("VIRE_API_URL"); v != "" {
		cfg.API.URL = v
	}
	if v := os.Getenv("VIRE_DEFAULT_PORTFOLIO"); v != "" {
		cfg.User.Portfolios = []string{v}
	}
	if v := os.Getenv("VIRE_DISPLAY_CURRENCY"); v != "" {
		cfg.User.DisplayCurrency = v
	}
	if v := os.Getenv("VIRE_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}

	return cfg
}
