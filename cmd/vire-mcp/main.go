// Command vire-mcp is a thin stdio MCP server that reuses internal/mcp.
// It fetches the tool catalog from vire-server, registers tools dynamically,
// and serves over stdio for Claude Desktop integration.
//
// Configuration priority: defaults < TOML file < environment variables (VIRE_*).
// The TOML file is auto-discovered from vire-mcp.toml or config/vire-mcp.toml.
//
// Environment variables:
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

// configSearchPaths lists TOML files to auto-discover (first match wins).
var configSearchPaths = []string{
	"vire-mcp.toml",
	"config/vire-mcp.toml",
}

func main() {
	cfg := loadConfig()

	// Create logger from config. Console output goes to stderr (arbor default),
	// so it won't interfere with stdout which is reserved for stdio MCP transport.
	logger := common.NewLoggerFromConfig(common.LoggingConfig{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Outputs:    cfg.Logging.Outputs,
		FilePath:   cfg.Logging.FilePath,
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
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

// loadConfig builds configuration with priority: defaults < TOML file < env vars.
// Auto-discovers the TOML file from configSearchPaths.
func loadConfig() *config.Config {
	// Try to find and load a TOML config file.
	for _, path := range configSearchPaths {
		if _, err := os.Stat(path); err == nil {
			cfg, err := config.LoadFromFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", path, err)
				break
			}
			return cfg
		}
	}

	// No TOML file found — build from defaults + env vars.
	cfg := config.NewDefaultConfig()
	cfg.API.URL = "http://localhost:4242"
	cfg.Logging.Level = "warn"
	cfg.Logging.Outputs = []string{"console"}

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
