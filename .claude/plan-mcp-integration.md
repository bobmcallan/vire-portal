# Plan: MCP Integration into vire-portal

## Summary

Port the MCP server from vire-mcp (in the vire repo) into vire-portal as an `internal/mcp/` package. The MCP endpoint mounts on the existing `net/http` server at `/mcp`. Tool calls are proxied to vire-server with X-Vire-* header injection. No OAuth — open MCP endpoint for local development.

## Key Design Decisions (Already Made)

1. Use `mcp-go` library (`github.com/mark3labs/mcp-go`) with Streamable HTTP transport
2. Hardcode tool catalog matching vire-mcp's 25 active tools (not fetched from vire-server)
3. Generic proxy handler returning RAW JSON: `mcp.NewToolResultText(string(respBody))`
4. X-Vire-* header injection from config
5. Config additions: `[api]`, `[user]`, `[keys]` sections in portal.toml
6. Docker compose: two-service stack (portal + vire-server)
7. NO OAuth — open MCP endpoint for local development
8. MCP handler mounted at `/mcp` on existing net/http server (not a separate listener)

## Architecture

```
Claude / MCP Client
  |
  | POST /mcp (Streamable HTTP)
  v
vire-portal (:8080)   <-- existing net/http server
  |  internal/mcp/ package
  |  - Hardcoded tool catalog (25 active tools)
  |  - Generic proxy handler
  |  - X-Vire-* header injection from config
  v
vire-server (:4242)   <-- separate container
```

## File Changes

### 1. Config Layer (`internal/config/`)

**config.go** — Add three new config sections to `Config` struct:

```go
type Config struct {
    Server  ServerConfig  `toml:"server"`
    API     APIConfig     `toml:"api"`     // NEW
    User    UserConfig    `toml:"user"`    // NEW
    Keys    KeysConfig    `toml:"keys"`    // NEW
    Storage StorageConfig `toml:"storage"`
    Logging LoggingConfig `toml:"logging"`
}

type APIConfig struct {
    URL string `toml:"url"` // vire-server URL, e.g. "http://vire-server:4242"
}

type UserConfig struct {
    Portfolios      []string `toml:"portfolios"`
    DisplayCurrency string   `toml:"display_currency"`
}

type KeysConfig struct {
    EODHD  string `toml:"eodhd"`
    Navexa string `toml:"navexa"`
    Gemini string `toml:"gemini"`
}
```

**defaults.go** — Add defaults:
- `API.URL`: `"http://localhost:4242"` (matches vire-server default)
- `User.Portfolios`: `[]string{}` (empty)
- `User.DisplayCurrency`: `""` (empty — vire-server uses its own default)
- `Keys.*`: `""` (empty — no API keys by default)

**config.go applyEnvOverrides** — Add env var support:
- `VIRE_API_URL` -> `config.API.URL`
- `VIRE_DEFAULT_PORTFOLIO` -> `config.User.Portfolios` (single value to slice)
- `VIRE_DISPLAY_CURRENCY` -> `config.User.DisplayCurrency`
- `EODHD_API_KEY` -> `config.Keys.EODHD`
- `NAVEXA_API_KEY` -> `config.Keys.Navexa`
- `GEMINI_API_KEY` -> `config.Keys.Gemini`

### 2. MCP Package (`internal/mcp/`)

New package with 4 files:

**proxy.go** — HTTP client that proxies requests to vire-server:
- `MCPProxy` struct with `serverURL`, `httpClient`, `logger`, `userHeaders`
- `NewMCPProxy(serverURL string, logger *slog.Logger, cfg *config.Config)` constructor
- Builds X-Vire-* headers from config (portfolios, display_currency, navexa, eodhd, gemini keys)
- Methods: `get(path)`, `post(path, data)`, `put(path, data)`, `patch(path, data)`, `del(path)` — all return `([]byte, error)`
- Uses `log/slog` (not vire's internal/common.Logger) to match portal conventions
- 300s HTTP timeout (matching vire-mcp for long operations like generate_report)
- Error handling: parse `{"error": "..."}` responses, return formatted error strings

**tools.go** — Hardcoded tool catalog (all 25 active tools):
- `RegisterTools(s *server.MCPServer, p *MCPProxy)` function
- Each tool uses `mcp.NewTool(name, mcp.WithDescription(...), mcp.WithString(...), ...)`
- Each tool's handler uses generic proxy pattern returning raw JSON
- Tools match exactly the active (non-commented) tools in vire-mcp/tools.go:
  1. get_version
  2. portfolio_compliance
  3. get_portfolio
  4. strategy_scanner
  5. stock_screen
  6. get_stock_data
  7. compute_indicators
  8. list_portfolios
  9. get_portfolio_stock
  10. generate_report
  11. list_reports
  12. get_summary
  13. set_default_portfolio
  14. get_config
  15. get_strategy_template
  16. set_portfolio_strategy
  17. get_portfolio_strategy
  18. delete_portfolio_strategy
  19. get_portfolio_plan
  20. set_portfolio_plan
  21. add_plan_item
  22. update_plan_item
  23. remove_plan_item
  24. check_plan_status
  25. get_quote
  26. get_diagnostics

**handlers.go** — Generic handler implementations:
- Each handler extracts parameters, constructs the API path, calls proxy, returns raw JSON
- Key difference from vire-mcp handlers: NO markdown formatting, NO model parsing
- Pattern: extract params -> build URL -> call proxy.get/post/etc -> `mcp.NewToolResultText(string(body))`
- For handlers needing `portfolio_name` resolution: use `resolvePortfolio(p, request)` helper that calls `GET /api/portfolios/default` if not provided
- Error results: `mcp.CallToolResult{Content: [...], IsError: true}`

**handler.go** — HTTP handler that mounts mcp-go on the portal's net/http server:
- `Handler` struct wrapping `*server.StreamableHTTPServer` from mcp-go
- `NewHandler(cfg *config.Config, logger *slog.Logger)` constructor:
  - Creates MCPProxy
  - Creates `server.MCPServer` with `server.WithToolCapabilities(true)`
  - Calls `RegisterTools(mcpServer, proxy)`
  - Creates `server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true))`
  - Extracts the HTTP handler from StreamableHTTPServer
- `ServeHTTP(w, r)` — delegates to the mcp-go StreamableHTTPServer handler

### 3. App Wiring (`internal/app/app.go`)

- Add `MCPHandler *mcp.Handler` to App struct
- In `initHandlers()`: create and store `mcp.NewHandler(a.Config, a.Logger)`

### 4. Route Registration (`internal/server/routes.go`)

- Add `mux.Handle("/mcp", s.app.MCPHandler)` to `setupRoutes()`
- The mcp-go StreamableHTTPServer handles POST (and GET/DELETE for session management)

### 5. Middleware Fixes (`internal/server/middleware.go`)

Three middleware issues identified by devil's-advocate review:

**C1. CSRF middleware blocks POST /mcp** — The CSRF middleware only skips `/api/` paths. MCP clients (Claude) don't send CSRF tokens. Fix: add `/mcp` to the skip list, same pattern as `/api/`.

**C2. 1MB body limit on /mcp** — The `maxBodySizeMiddleware(1<<20)` applies to all requests. While MCP messages are typically small, the limit produces confusing errors for MCP clients. Fix: skip body limit for `/mcp` paths.

**C3. 60s WriteTimeout vs 300s proxy timeout** — `server.go` sets `WriteTimeout: 60*time.Second`, but the proxy uses 300s for slow tools (generate_report, strategy_scanner). The server will kill the connection at 60s. Fix: increase `WriteTimeout` to `300 * time.Second` in `server.go`. MCP is the primary use case, and other routes (health, version, landing page) are fast and unaffected by a higher timeout.

Updates in `middleware.go`:
- CSRF middleware: add `strings.HasPrefix(r.URL.Path, "/mcp")` skip condition
- maxBodySizeMiddleware: add `/mcp` path exemption

### 5a. mcp-go Integration Verification

**Confirmed:** `StreamableHTTPServer` implements `http.Handler` via `ServeHTTP(w, r)`. The integration pattern:
```go
handler := server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true))
mux.Handle("/mcp", handler)  // mount on existing mux
```
No separate port, no separate http.Server. The `WithEndpointPath` option only applies to `Start()`, not when used as handler.

### 6. Config File (`docker/portal.toml`)

Add sections:
```toml
[api]
url = "http://vire-server:4242"

[user]
# portfolios = ["SMSF"]
# display_currency = "AUD"

[keys]
# eodhd = ""
# navexa = ""
# gemini = ""
```

### 7. Docker Compose (`docker/docker-compose.yml`)

New file:
```yaml
name: vire

services:
  portal:
    build:
      context: ..
      dockerfile: Dockerfile
    image: vire-portal:latest
    container_name: vire-portal
    ports:
      - "8080:8080"
    environment:
      - VIRE_SERVER_HOST=0.0.0.0
      - VIRE_API_URL=http://vire-server:4242
    depends_on:
      vire-server:
        condition: service_healthy
    restart: unless-stopped

  vire-server:
    image: ghcr.io/bobmcallan/vire-server:latest
    pull_policy: always
    container_name: vire-server
    ports:
      - "4242:4242"
    volumes:
      - vire-data:/app/data
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:4242/api/health"]
      interval: 30s
      timeout: 5s
      retries: 3
    restart: unless-stopped

volumes:
  vire-data:
```

### 8. go.mod

Add dependency: `github.com/mark3labs/mcp-go`

## Handler Pattern: Raw JSON vs Formatted Markdown

The key architectural difference from vire-mcp: all handlers return **raw JSON** from vire-server, not formatted markdown. The pattern:

```go
func handleGetPortfolio(p *MCPProxy) server.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        portfolioName := resolvePortfolio(p, request)
        if portfolioName == "" {
            return errorResult("portfolio_name required (no default configured)"), nil
        }
        body, err := p.get(fmt.Sprintf("/api/portfolios/%s", url.PathEscape(portfolioName)))
        if err != nil {
            return errorResult(fmt.Sprintf("Error: %v", err)), nil
        }
        return mcp.NewToolResultText(string(body)), nil
    }
}
```

This eliminates the entire `formatters.go` file from vire-mcp and all the `models` package imports. Claude formats the raw JSON itself.

## Test Plan

Tests go in `internal/mcp/mcp_test.go`:

1. **Tool registration**: verify all 26 tools are registered on the MCPServer
2. **Proxy construction**: verify X-Vire-* headers are built correctly from config
3. **Handler behavior**: use httptest to mock vire-server, verify:
   - Correct API path is called
   - Correct HTTP method is used
   - X-Vire-* headers are forwarded
   - Raw JSON response is returned as MCP text result
   - Errors return IsError=true results
4. **Config parsing**: verify new [api], [user], [keys] sections parse from TOML
5. **Env overrides**: verify VIRE_API_URL, VIRE_DEFAULT_PORTFOLIO, etc.
6. **Route integration**: verify POST /mcp hits the MCP handler (via server test pattern)
7. **CSRF skip**: verify POST /mcp is not blocked by CSRF middleware

## Verification Sequence

1. `go vet ./...` — passes
2. `go test ./...` — all tests pass (existing + new)
3. `go build ./cmd/portal/` — compiles
4. Docker build: `docker build -t vire-portal:latest .`
5. Docker compose: `docker compose -f docker/docker-compose.yml up`
6. Claude connects to `http://localhost:8080/mcp` and lists tools
