# Requirements: MCP OAuth Phase 2 — Discovery Endpoints

**Date:** 2026-02-16
**Requested:** Implement Phase 2 from `docs/authentication/mcp-oauth-implementation-steps.md` — add `.well-known` OAuth discovery endpoints so Claude CLI and Desktop can discover Vire's OAuth capabilities.

## Scope

**In scope:**
- `GET /.well-known/oauth-authorization-server` — OAuth Authorization Server Metadata (RFC 8414)
- `GET /.well-known/oauth-protected-resource` — OAuth Protected Resource Metadata
- New `internal/auth/` package with discovery handler
- Config: add `portal_url` to derive the issuer/base URL for metadata responses
- Route registration in `internal/server/routes.go`
- Wire up in `internal/app/app.go`
- Tests for both endpoints

**Out of scope:**
- DCR `/register` endpoint (Phase 3)
- `/authorize`, `/token` endpoints (Phases 4-5)
- Bearer token on `/mcp` (Phase 6)

## Approach

Create a new `internal/auth/` package with a `DiscoveryHandler` struct that holds the base URL (portal URL) and exposes two HTTP handler methods. The base URL is derived from config:
- If `VIRE_PORTAL_URL` env var is set, use that (for tunneling, production)
- Otherwise compute from `Server.Host` and `Server.Port` (e.g. `http://localhost:8500`)

The discovery handler is a simple struct — no dependencies on database, vire-server, or other services. It just returns static JSON based on the configured base URL.

### Config changes
- Add `PortalURL` field to `AuthConfig`
- Add `VIRE_PORTAL_URL` env override
- Add `portal_url` to TOML config files
- Add `BaseURL()` helper method on `Config` that returns `PortalURL` if set, otherwise builds from host+port

### Route registration
Add to `internal/server/routes.go`:
```go
mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.app.DiscoveryHandler.HandleAuthorizationServer)
mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.app.DiscoveryHandler.HandleProtectedResource)
```

### App wiring
Add `DiscoveryHandler` field to `App` struct, initialize in `initHandlers()`.

## Files Expected to Change

**New:**
- `internal/auth/discovery.go` — DiscoveryHandler with two endpoint handlers
- `internal/auth/discovery_test.go` — Tests for both endpoints

**Modified:**
- `internal/config/config.go` — Add `PortalURL` to `AuthConfig`, env override, `BaseURL()` helper
- `internal/config/defaults.go` — Default `PortalURL` to `""`
- `internal/config/config_test.go` — Test `BaseURL()` logic (if test file exists, otherwise add to new test)
- `internal/app/app.go` — Add `DiscoveryHandler` field, wire up
- `internal/server/routes.go` — Add `.well-known` routes
- `config/vire-portal.toml` — Add `portal_url` under `[auth]`
- `config/vire-portal.toml.example` — Same
