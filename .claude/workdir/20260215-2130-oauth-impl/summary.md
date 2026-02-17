# Summary: OAuth Authentication (Phase 1)

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Added `AuthConfig` struct (jwt_secret, callback_url), env overrides |
| `internal/config/defaults.go` | Added auth defaults (empty jwt_secret, localhost callback URL) |
| `internal/config/config_test.go` | Tests for auth config loading and env overrides |
| `internal/handlers/auth.go` | Rewritten: dev login via vire-server `/api/auth/oauth`, Google/GitHub redirect handlers, OAuth callback handler, JWT validation (HMAC-SHA256), `IsLoggedIn` helper |
| `internal/handlers/auth_test.go` | New: tests for JWT validation, IsLoggedIn, dev login, callback, redirects |
| `internal/handlers/auth_stress_test.go` | New: security stress tests (alg:none attack, tampering, expired tokens, hostile inputs) |
| `internal/handlers/landing.go` | Updated LoggedIn check to use JWT validation |
| `internal/handlers/dashboard.go` | Updated LoggedIn check to use JWT validation |
| `internal/handlers/settings.go` | Updated LoggedIn check, replaced ExtractJWTSub with claims from IsLoggedIn |
| `internal/client/vire_client.go` | Added `ExchangeOAuth` method |
| `internal/app/app.go` | Passes auth config to handlers |
| `internal/server/routes.go` | Added routes: GET /api/auth/login/google, /github, GET /auth/callback |
| `config/vire-portal.toml.example` | Added [auth] section |
| `docs/authentication.md` | Updated Phase 1 status |
| `.claude/skills/develop/SKILL.md` | Updated routes and config tables |

## Tests
- auth_test.go: JWT validation (valid, expired, tampered, empty secret), IsLoggedIn, dev login via mock server, callback handler, Google/GitHub redirects
- auth_stress_test.go: alg:none attack, signature bypass, timing-safe comparison, hostile inputs, oversized tokens
- All tests pass (`go test ./...`)
- go vet clean

## Documentation Updated
- docs/authentication.md — Phase 1 marked complete
- .claude/skills/develop/SKILL.md — routes and config tables updated
- config/vire-portal.toml.example — [auth] section added

## Devils-Advocate Findings
- Security stress tests all pass, no blocking issues
- HMAC comparison uses `hmac.Equal` (constant-time) — timing attack resistant
- alg:none attack correctly rejected when jwt_secret is configured

## Notes
- Server-side redirect pattern: portal redirects to vire-server for Google/GitHub login, receives JWT on callback
- Dev login flows through same `/api/auth/oauth` endpoint as real OAuth (provider: "dev")
- JWT validation is opt-in when jwt_secret is empty (backwards compat during migration)
- Server is running on localhost:8500 with auth flows verified
