# Requirements: Implement OAuth Authentication (Phase 1)

**Date:** 2026-02-15
**Requested:** Implement OAuth from docs/authentication.md. vire-server endpoints are ready.

## Scope
- In scope: Phase 1 from authentication.md — dev login via `/api/auth/oauth`, JWT validation, callback route, Google/GitHub redirect stubs
- Out of scope: email/password login, actual Google/GitHub OAuth (server-side redirect handles that)

## Approach

Use the "Server-Side Redirect" pattern from the auth design doc. The portal:
1. Redirects Google/GitHub login links to vire-server's login endpoints with `?callback=` URL
2. Receives JWT on callback (`GET /auth/callback?token=xxx`)
3. Sets `vire_session` cookie
4. Validates JWT signature (HMAC-SHA256) on every request instead of just checking cookie existence

Dev login calls `POST {api_url}/api/auth/oauth` with `{ provider: "dev", code: "dev", state: "dev" }`.

vire-server endpoints (already implemented):
- `POST /api/auth/oauth` — exchanges credentials for JWT (supports dev/google/github providers)
- `GET /api/auth/login/google?callback=` — redirects to Google OAuth
- `GET /api/auth/login/github?callback=` — redirects to GitHub OAuth
- `GET /api/auth/callback/google` and `/github` — OAuth callbacks that redirect to portal with `?token=`

## Files to Change
- `internal/config/config.go` — add AuthConfig (jwt_secret, callback_url)
- `internal/config/defaults.go` — add auth defaults
- `internal/config/config_test.go` — test auth config
- `internal/handlers/auth.go` — rewrite: dev login via server, Google/GitHub redirect, callback handler, JWT validation
- `internal/handlers/landing.go` — update LoggedIn check to use JWT validation
- `internal/handlers/dashboard.go` — update LoggedIn check
- `internal/handlers/settings.go` — update LoggedIn check, replace ExtractJWTSub
- `internal/client/vire_client.go` — add ExchangeOAuth method
- `internal/app/app.go` — pass auth config to AuthHandler
- `internal/server/routes.go` — add new auth routes
- `config/vire-portal.toml.example` — add [auth] section
