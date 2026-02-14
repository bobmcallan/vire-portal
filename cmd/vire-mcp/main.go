package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	toml "github.com/pelletier/go-toml/v2"

	"github.com/bobmcallan/vire-portal/internal/vire/common"
)

// UserConfig holds per-user configuration that gets injected as X-Vire-* headers.
type UserConfig struct {
	Portfolios      []string `toml:"portfolios"`
	DisplayCurrency string   `toml:"display_currency"`
}

// NavexaConfig holds per-user Navexa API configuration.
type NavexaConfig struct {
	APIKey string `toml:"api_key"`
}

// ServerConfig holds MCP server settings.
type ServerConfig struct {
	Name      string `toml:"name"`
	Port      string `toml:"port"`
	ServerURL string `toml:"server_url"`
}

// Config holds all vire-mcp configuration.
type Config struct {
	Server  ServerConfig         `toml:"server"`
	User    UserConfig           `toml:"user"`
	Navexa  NavexaConfig         `toml:"navexa"`
	Logging common.LoggingConfig `toml:"logging"`
}

// newDefaultConfig returns a Config with sensible defaults.
func newDefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Name:      "Vire-MCP",
			Port:      "4243",
			ServerURL: "http://vire-server:4242",
		},
		Logging: common.LoggingConfig{
			Level:      "info",
			Outputs:    []string{"console", "file"},
			FilePath:   "logs/vire-mcp.log",
			MaxSizeMB:  100,
			MaxBackups: 3,
		},
	}
}

// loadConfig loads configuration from a TOML file with defaults and env overrides.
func loadConfig(path string) Config {
	cfg := newDefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Fatalf("Failed to read config file %s: %v", path, err)
			}
			// File not found — use defaults
		} else {
			if err := toml.Unmarshal(data, &cfg); err != nil {
				log.Fatalf("Failed to parse config file %s: %v", path, err)
			}
		}
	}

	// Apply environment overrides (matches vire-server patterns)
	if p := os.Getenv("VIRE_DEFAULT_PORTFOLIO"); p != "" {
		cfg.User.Portfolios = []string{p}
	}
	if dc := os.Getenv("VIRE_DISPLAY_CURRENCY"); dc != "" {
		cfg.User.DisplayCurrency = dc
	}
	if nk := os.Getenv("NAVEXA_API_KEY"); nk != "" {
		cfg.Navexa.APIKey = nk
	}
	if url := os.Getenv("VIRE_SERVER_URL"); url != "" {
		cfg.Server.ServerURL = url
	}
	if port := os.Getenv("VIRE_MCP_PORT"); port != "" {
		cfg.Server.Port = port
	}

	return cfg
}

func main() {
	stdio := flag.Bool("stdio", false, "Use stdio transport (for Claude Desktop)")
	configFile := flag.String("config", "vire-mcp.toml", "Path to config file")
	flag.Parse()

	cfg := loadConfig(*configFile)

	// Load version
	common.LoadVersionFromFile()

	// Setup logging
	logger := common.NewLoggerFromConfig(cfg.Logging)

	serverURL := cfg.Server.ServerURL
	proxy := NewMCPProxy(serverURL, logger, cfg.User, cfg.Navexa)

	// Create MCP server with tool definitions
	mcpServer := server.NewMCPServer(
		cfg.Server.Name,
		common.GetVersion(),
		server.WithToolCapabilities(true),
	)

	// Register all MCP tools
	registerTools(mcpServer, proxy)

	if *stdio {
		// Stdio transport — reads stdin, writes stdout
		if err := server.ServeStdio(mcpServer); err != nil {
			fmt.Fprintf(os.Stderr, "stdio server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	port := cfg.Server.Port

	// Streamable HTTP transport — listens on configured port
	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithStateLess(true),
	)

	log.Printf("Starting MCP Streamable HTTP on :%s", port)
	fmt.Fprintf(os.Stderr, "Starting MCP Streamable HTTP on :%s\n", port)

	if err := httpServer.Start(":" + port); err != nil {
		fmt.Fprintf(os.Stderr, "http server error: %v\n", err)
		os.Exit(1)
	}
}
