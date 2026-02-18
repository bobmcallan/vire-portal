// Command vire-mcp is a stdio-to-HTTP bridge for MCP.
// It connects to vire-portal's Streamable HTTP MCP endpoint as a client,
// discovers available tools, and re-exposes them over stdio for Claude Desktop.
//
// All tool logic, catalog management, and version handling live in vire-portal.
// vire-mcp is a pure transport adapter: stdio ↔ HTTP with OAuth 2.1.
//
// Configuration priority: defaults < TOML file < environment variables (VIRE_*).
// The TOML file is auto-discovered from vire-mcp.toml or config/vire-mcp.toml.
//
// Environment variables:
//
//	VIRE_PORTAL_URL  vire-portal URL (default: http://localhost:8500)
//	VIRE_LOG_LEVEL   log level       (default: info)
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// binDir returns the directory containing the running binary.
// Falls back to "." if the executable path cannot be determined.
func binDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// configSearchPaths returns TOML files to auto-discover (first match wins).
// Binary-relative paths are tried first so the config is found even when
// the working directory differs from the binary location (e.g. Claude Desktop
// launching via WSL). Paths are deduplicated via filepath.Abs.
func configSearchPaths() []string {
	candidates := []string{
		"vire-mcp.toml",
		"config/vire-mcp.toml",
	}

	dir := binDir()

	paths := []string{
		filepath.Join(dir, "vire-mcp.toml"),
		filepath.Join(dir, "config", "vire-mcp.toml"),
	}
	paths = append(paths, candidates...)

	// Deduplicate via absolute path.
	seen := make(map[string]bool, len(paths))
	deduped := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		deduped = append(deduped, p)
	}
	return deduped
}

func main() {
	cfg := loadConfig()

	// Console output goes to stderr so it won't interfere with stdio MCP on stdout.
	logger := common.NewLoggerFromConfig(common.LoggingConfig{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Outputs:    cfg.Logging.Outputs,
		FilePath:   cfg.Logging.FilePath,
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
	})

	portalURL := strings.TrimRight(cfg.Portal.URL, "/")
	logger.Info().Str("portal_url", portalURL).Msg("loaded configuration")

	// Allocate port for OAuth callback server.
	callbackPort, err := findFreePort()
	if err != nil {
		logger.Error().Str("error", err.Error()).Msg("failed to allocate OAuth callback port")
		os.Exit(1)
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", callbackPort)

	tokenStore := NewFileTokenStore(filepath.Join(homeDir(), ".vire", "credentials.json"))

	// Connect to vire-portal's Streamable HTTP MCP endpoint with OAuth.
	// AuthServerMetadataURL is set explicitly to skip the openid-configuration
	// probe, which can fail when a catch-all route returns HTML instead of 404.
	httpTransport, err := transport.NewStreamableHTTP(
		portalURL+"/mcp",
		transport.WithHTTPOAuth(transport.OAuthConfig{
			RedirectURI:           redirectURI,
			TokenStore:            tokenStore,
			PKCEEnabled:           true,
			AuthServerMetadataURL: portalURL + "/.well-known/oauth-authorization-server",
		}),
	)
	if err != nil {
		logger.Error().Str("error", err.Error()).Msg("failed to create HTTP transport")
		os.Exit(1)
	}

	mcpClient := client.NewClient(httpTransport)

	ctx := context.Background()
	if err := connectWithOAuth(ctx, mcpClient, callbackPort, logger); err != nil {
		logger.Error().Str("error", err.Error()).Msg("failed to connect to vire-portal")
		os.Exit(1)
	}
	defer mcpClient.Close()

	// Discover tools from vire-portal.
	toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		logger.Warn().Str("error", err.Error()).Msg("failed to list tools from portal")
	}

	var tools []mcp.Tool
	if toolsResult != nil {
		tools = toolsResult.Tools
	}

	// Create local stdio MCP server and register proxy handlers.
	mcpSrv := server.NewMCPServer("vire", common.GetVersion(), server.WithToolCapabilities(true))
	for _, tool := range tools {
		t := tool // capture for closure
		mcpSrv.AddTool(t, proxyHandler(mcpClient, t.Name, callbackPort, logger))
	}

	logger.Info().Int("tools", len(tools)).Str("portal_url", portalURL).Msg("vire-mcp ready")

	if err := server.ServeStdio(mcpSrv); err != nil {
		fmt.Fprintf(os.Stderr, "stdio server error: %v\n", err)
		os.Exit(1)
	}
}

// connectWithOAuth starts the MCP client and initializes the session,
// running the OAuth browser flow if either step requires authorization.
func connectWithOAuth(ctx context.Context, c *client.Client, callbackPort int, logger *common.Logger) error {
	// Start transport.
	if err := c.Start(ctx); err != nil {
		if err = runOAuthIfNeeded(err, callbackPort, logger); err != nil {
			return fmt.Errorf("start: %w", err)
		}
		if err = c.Start(ctx); err != nil {
			return fmt.Errorf("start after auth: %w", err)
		}
	}

	// Initialize MCP session.
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "vire-mcp", Version: common.GetVersion()}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		if err = runOAuthIfNeeded(err, callbackPort, logger); err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
		if _, err = c.Initialize(ctx, initReq); err != nil {
			return fmt.Errorf("initialize after auth: %w", err)
		}
	}

	return nil
}

// runOAuthIfNeeded checks whether err is an OAuthAuthorizationRequiredError.
// If so, it runs the browser OAuth flow and returns nil on success.
// Otherwise it returns the original error unchanged.
func runOAuthIfNeeded(err error, callbackPort int, logger *common.Logger) error {
	var oauthErr *transport.OAuthAuthorizationRequiredError
	if !errors.As(err, &oauthErr) {
		return err
	}
	logger.Info().Msg("OAuth authorization required, opening browser")
	if flowErr := doOAuthFlow(oauthErr.Handler, callbackPort, logger); flowErr != nil {
		return fmt.Errorf("OAuth flow: %w", flowErr)
	}
	return nil
}

// proxyHandler returns a tool handler that forwards calls to vire-portal
// via the MCP client. On token expiry it re-runs the OAuth flow and retries once.
func proxyHandler(c *client.Client, toolName string, callbackPort int, logger *common.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		req.Params.Name = toolName
		result, err := c.CallTool(ctx, req)
		if err != nil {
			var oauthErr *transport.OAuthAuthorizationRequiredError
			if errors.As(err, &oauthErr) {
				logger.Info().Str("tool", toolName).Msg("re-authenticating (token expired)")
				if flowErr := doOAuthFlow(oauthErr.Handler, callbackPort, logger); flowErr != nil {
					return nil, fmt.Errorf("re-auth failed: %w", flowErr)
				}
				return c.CallTool(ctx, req)
			}
			return nil, err
		}
		return result, nil
	}
}

// findFreePort returns a free TCP port on the loopback interface.
func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// homeDir returns the user's home directory.
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}
	return "."
}

// loadConfig builds configuration with priority: defaults < TOML file < env vars.
// Relative log file paths are resolved against the binary directory so that
// logs land next to the binary regardless of the working directory.
func loadConfig() *config.Config {
	var cfg *config.Config

	for _, path := range configSearchPaths() {
		if _, err := os.Stat(path); err == nil {
			loaded, err := config.LoadFromFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", path, err)
				break
			}
			cfg = loaded
			break
		}
	}

	if cfg == nil {
		// No TOML file — defaults + env vars.
		cfg = config.NewDefaultConfig()
		cfg.Logging.Level = "info"
		cfg.Logging.Outputs = []string{"console", "file"}
		cfg.Logging.FilePath = "logs/vire-mcp.log"
		cfg.Logging.MaxSizeMB = 10
		cfg.Logging.MaxBackups = 3
	}

	if v := os.Getenv("VIRE_PORTAL_URL"); v != "" {
		cfg.Portal.URL = v
	}
	if v := os.Getenv("VIRE_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}

	// Resolve relative log path against binary directory so logs land in
	// bin/logs/ even when the working directory differs (e.g. Claude Desktop).
	if cfg.Logging.FilePath != "" && !filepath.IsAbs(cfg.Logging.FilePath) {
		cfg.Logging.FilePath = filepath.Join(binDir(), cfg.Logging.FilePath)
	}

	return cfg
}
