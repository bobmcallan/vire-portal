# Summary: MCP OAuth Phase 2 — Discovery Endpoints

**Date:** 2026-02-16
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `internal/auth/discovery.go` | **New**: `DiscoveryHandler` with `HandleAuthorizationServer` and `HandleProtectedResource` methods |
| `internal/auth/discovery_test.go` | **New**: Unit tests for both endpoints (correct metadata, different base URLs, method not allowed) |
| `internal/auth/discovery_stress_test.go` | **New**: 25 stress tests (concurrent access, hostile URLs, HEAD requests, large bodies, edge cases) |
| `internal/config/config.go` | Added `PortalURL` to `AuthConfig`, `BaseURL()` method, `VIRE_PORTAL_URL` env override |
| `internal/config/defaults.go` | Default `PortalURL` to `""` |
| `internal/app/app.go` | Added `DiscoveryHandler` field, wired up with `Config.BaseURL()` |
| `internal/server/routes.go` | Added `GET /.well-known/oauth-authorization-server` and `GET /.well-known/oauth-protected-resource` routes |
| `config/vire-portal.toml` | Added `portal_url` under `[auth]` |
| `config/vire-portal.toml.example` | Added `portal_url` with comment |
| `docs/authentication/mcp-oauth-implementation-steps.md` | Phase 2 marked complete |

## Tests

- `internal/auth/discovery_test.go` — unit tests for both discovery endpoints
- `internal/auth/discovery_stress_test.go` — 25 stress tests
- All tests pass (`go test ./...`)
- `go vet ./...` clean

## Documentation Updated

- `docs/authentication/mcp-oauth-implementation-steps.md` — Phase 2 marked complete

## Devils-Advocate Findings

- 3 bugs found and fixed by implementer (details sent via DM during task #3)
- HEAD request support added (returns headers without body — correct HTTP behavior)
- Input sanitization on `NewDiscoveryHandler` — trims whitespace and trailing slashes from baseURL
- 25 stress tests written covering hostile URLs, concurrent access, large request bodies

## Notes

- Discovery endpoints are stateless — no database or external service dependencies
- `BaseURL()` derives from `PortalURL` if set, otherwise from `Server.Host:Port`
- For local dev, `BaseURL()` returns `http://localhost:4241` automatically
- For tunneling/production, set `VIRE_PORTAL_URL=https://portal.vire.dev`
- Cache-Control header set to 1 hour — metadata is stable
- The `internal/auth/` package is established and will be extended in Phases 3-5 with DCR, authorize, and token handlers
