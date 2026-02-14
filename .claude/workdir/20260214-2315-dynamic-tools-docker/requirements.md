# Requirements: Dynamic Tool Registration + Docker Restructure

**Date:** 2026-02-14
**Requested:** Replace hardcoded MCP tools with dynamic registration from vire-server's `/api/mcp/tools` catalog. Rename portal.toml to vire-portal.toml. Move Dockerfile into docker/. Ensure service runs locally in Docker with single-user TOML config (no OAuth).

## Context

The previous task implemented MCP with 26 hardcoded tool definitions. vire-server now exposes `GET /api/mcp/tools` with a machine-readable catalog. This task replaces the hardcoded tools with dynamic registration, renames config, and consolidates Docker files.

## Scope

### In Scope
- Replace hardcoded tools in internal/mcp/tools.go with dynamic registration from `GET /api/mcp/tools`
- Fetch tool catalog from vire-server on startup
- Build mcp-go tool schemas from catalog entries dynamically
- Generic proxy handler driven by catalog (method, path, params with in=path/query/body, default_from)
- Rename portal.toml to vire-portal.toml everywhere (config auto-discovery, Dockerfile, scripts, docs)
- Move Dockerfile from project root to docker/Dockerfile
- Update all references: docker-compose.yml build context, .dockerignore, deploy.sh, build.sh, test-scripts.sh, README, docs
- Verify service runs in Docker with two-service compose (portal + vire-server)
- Single-user config in vire-portal.toml (no OAuth)

### Out of Scope
- OAuth 2.1 — deferred
- User profiles in BadgerDB — deferred
- Catalog refresh on timer (startup-only for now)
- Portal UI pages — deferred

## Approach

### Dynamic Tool Registration
Fetch `GET /api/mcp/tools` from vire-server at startup. For each catalog entry, build an mcp-go tool schema from the name, description, and params. Register a generic handler that:
1. Resolves `path` params by substituting `{param}` placeholders (with `default_from` support)
2. Builds JSON body from `body` params
3. Appends `query` params to URL
4. Sends HTTP request with correct method and X-Vire-* headers
5. Returns raw JSON as MCP tool result

### Catalog Schema (from vire README)
```json
{
  "name": "portfolio_compliance",
  "description": "...",
  "method": "POST",
  "path": "/api/portfolios/{portfolio_name}/review",
  "params": [
    {"name": "portfolio_name", "type": "string", "in": "path", "default_from": "user_config.default_portfolio"},
    {"name": "focus_signals", "type": "array", "in": "body"}
  ]
}
```

## Files Expected to Change
- internal/mcp/tools.go — Replace hardcoded tools with dynamic registration from catalog
- internal/mcp/handler.go — Fetch catalog on startup, register tools dynamically
- internal/mcp/proxy.go — May need catalog fetch method
- internal/mcp/mcp_test.go — Update tests for dynamic registration
- internal/mcp/catalog.go — New: catalog types and fetch logic
- cmd/portal/main.go — Update config file auto-discovery (vire-portal.toml)
- Dockerfile → docker/Dockerfile — Move and update paths
- docker/docker-compose.yml — Update build context for docker/Dockerfile
- docker/portal.toml → docker/vire-portal.toml — Rename
- .dockerignore — Update for new Dockerfile location
- scripts/deploy.sh — Update Dockerfile path, config file name
- scripts/build.sh — Update Dockerfile path
- scripts/test-scripts.sh — Update Dockerfile path, config file name
- README.md — Update all references
- docs/requirements.md — Update references
- .claude/skills/develop/SKILL.md — Update references
- docker/README.md — Update references
