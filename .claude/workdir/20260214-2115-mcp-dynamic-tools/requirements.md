# Requirements: MCP Dynamic Tool Route with Hardcoded Config

**Date:** 2026-02-14
**Requested:** Implement the dynamic MCP tool route in vire-portal using hardcoded configuration (matching vire-mcp.toml pattern). First test: deploy to Docker alongside vire-server, Claude connects to localhost:8080/mcp, tools are dynamically listed and execute via proxy to vire-server.

## Context

The two-service architecture (design-two-service-architecture.md) merges vire-mcp into vire-portal. This is migration steps 5-6: implement dynamic MCP tool registration and generic proxy handler. OAuth (step 4) and user management (step 7) are deferred — this phase uses hardcoded config like vire-mcp.toml does today.

## Scope

### In Scope
- Add MCP endpoint: POST /mcp (using mcp-go library, Streamable HTTP transport)
- Hardcode tool catalog (matching tools from vire README: get_quote, get_stock_data, portfolio_compliance, compute_indicators, strategy_scanner, plus other tools from vire-mcp)
- Generic proxy handler: construct HTTP requests from MCP tool calls, proxy to vire-server
- X-Vire-* header injection from hardcoded config (portfolios, display_currency, API keys)
- Config additions to portal.toml: [api] section (vire-server URL), [user] section (portfolios, display_currency), [keys] section (eodhd, navexa, gemini)
- Docker compose: two-service stack (vire-portal + vire-server)
- Verification: deploy to Docker, connect Claude, tools list and execute

### Out of Scope
- OAuth 2.1 (MCP auth) — deferred
- User profiles in BadgerDB — deferred
- Portal UI pages (settings, connect, dashboard) — deferred
- Fetching catalog from vire-server /api/mcp/tools — hardcoded first
- API key encryption — plaintext in config for now (like vire-mcp.toml)
- Changes to vire-server repo

## Approach
Port the tool registration and proxy logic from vire-mcp into vire-portal as internal/mcp/ package. Use mcp-go (github.com/mark3labs/mcp-go) for the MCP protocol server. Hardcode the tool catalog matching vire-mcp's current tools. Proxy tool calls to vire-server with X-Vire-* headers. No auth — open MCP endpoint for local development.

## Files Expected to Change
- go.mod / go.sum — add mcp-go dependency
- internal/mcp/handler.go — MCP endpoint handler
- internal/mcp/registry.go — Tool catalog (hardcoded)
- internal/mcp/proxy.go — Generic tool handler (proxy to vire-server)
- internal/mcp/headers.go — X-Vire-* header injection
- internal/mcp/types.go — ToolDefinition, ParamDefinition structs
- internal/config/config.go — Add [api], [user], [keys] config sections
- internal/config/defaults.go — Defaults for new config sections
- internal/server/routes.go — Register /mcp route
- internal/app/app.go — Wire MCP handler into dependency container
- docker/portal.toml — Add api, user, keys sections
- docker/docker-compose.yml — Add vire-server service
- README.md — MCP endpoint documentation
