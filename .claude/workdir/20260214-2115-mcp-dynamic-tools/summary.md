# Summary: MCP Dynamic Tool Route with Hardcoded Config

**Date:** 2026-02-14
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `go.mod` / `go.sum` | Added mcp-go dependency (github.com/mark3labs/mcp-go) |
| `internal/mcp/handler.go` | MCP endpoint handler — wraps mcp-go StreamableHTTPServer, implements http.Handler |
| `internal/mcp/tools.go` | 26 hardcoded tool definitions with 6 handler factory functions |
| `internal/mcp/handlers.go` | resolvePortfolio helper (with context propagation), errorResult helper |
| `internal/mcp/proxy.go` | MCPProxy — HTTP client with X-Vire-* header injection, context propagation, 300s timeout, 50MB response limit |
| `internal/mcp/mcp_test.go` | 38 tests: tool registration, proxy headers, HTTP methods, context cancellation, resolvePortfolio, path assertions |
| `internal/config/config.go` | Added APIConfig, UserConfig, KeysConfig structs with TOML + env var support |
| `internal/config/defaults.go` | Defaults for API URL (localhost:4242), empty user/keys |
| `internal/config/config_test.go` | 8 new tests for MCP config sections and env overrides |
| `internal/server/routes.go` | Registered POST /mcp route |
| `internal/server/middleware.go` | CSRF skip for /mcp, 10MB body limit for /mcp (vs 1MB default) |
| `internal/server/server.go` | WriteTimeout increased from 60s to 300s for MCP tools |
| `internal/server/routes_test.go` | 3 new tests: MCP endpoint accepts POST, CSRF exempt, correlation ID |
| `internal/app/app.go` | Added MCPHandler field, wired in initHandlers |
| `docker/portal.toml` | Added [api], [user], [keys] config sections |
| `docker/docker-compose.yml` | Two-service stack: portal + vire-server, VIRE_API_URL, depends_on, healthcheck |
| `docker/docker-compose.ghcr.yml` | Added VIRE_API_URL env var |
| `docker/README.md` | Two-service deployment instructions, MCP env vars |
| `scripts/test-scripts.sh` | Updated project name check for multi-service compose |
| `README.md` | MCP endpoint section, routes, config, architecture, Docker, Claude connection |
| `docs/requirements.md` | Updated API contracts for /mcp route |
| `.claude/skills/develop/SKILL.md` | Added internal/mcp/, POST /mcp route, MCP config settings, updated API integration |

## Tests
- 78 tests across 5 packages (config, handlers, mcp, server, storage/badger) — all pass
- Race detection: 0 races (`go test -race ./...`)
- go vet: clean
- Docker build: successful
- Script validation: 130/130 pass

## Documentation Updated
- `README.md` — MCP endpoint section with architecture diagram, 26-tool table, Claude connection, config, Docker
- `docs/requirements.md` — implemented routes table, MCP env vars
- `.claude/skills/develop/SKILL.md` — routes, config, key directories, API integration
- `docker/README.md` — two-service deployment, env vars, volumes

## Devils-Advocate Findings
- **CSRF middleware blocks POST /mcp** — Fixed: CSRF skip for /mcp path
- **WriteTimeout 60s vs proxy 300s** — Fixed: increased to 300s
- **resolvePortfolio context.Background()** — Fixed: propagates caller's context
- **Unbounded proxy response reads** — Fixed: io.LimitReader with 50MB cap
- **mcp-go integration feasibility** — Verified: StreamableHTTPServer implements http.Handler
- **1MB body limit** — Fixed: 10MB for /mcp route
- **Raw JSON vs formatted markdown** — Accepted: intentional design decision for generic proxy
- **Hardcoded catalog drift** — Accepted: step 1, dynamic catalog from /api/mcp/tools is next
- **Open MCP endpoint** — Accepted: OAuth deferred, Docker binds to container-internal 0.0.0.0
- **CORS wildcard on /mcp** — Deferred: accepted risk for local development
- **Race condition in context cancellation test** — Fixed: channel-based synchronization

## Notes
- 26 tools match vire-mcp's active tool catalog exactly
- All handlers return raw JSON from vire-server (no markdown formatting, no model parsing)
- Portal is a generic proxy — no tool-specific logic
- vire-server now exposes `GET /api/mcp/tools` with full catalog schema (documented in vire README)
- Next step: replace hardcoded tools with dynamic registration from /api/mcp/tools
- Future: OAuth 2.1 authentication for MCP endpoint, user profiles in BadgerDB
