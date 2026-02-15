# Authentication Design

## Status: Phase 1 Complete

**Date:** 2026-02-15

## Overview

The portal needs three authentication methods: Google OAuth, GitHub OAuth, and email/password. The dev login button should simulate the full OAuth redirect flow without requiring actual Google credentials, providing a testable path through the same code.

Phase 1 is complete: dev login calls vire-server `POST /api/auth/oauth`, JWTs are HMAC-SHA256 signed by vire-server, and the portal validates signatures and expiry on every request.

## Current State (Phase 1 Complete)

| Component | What Exists |
|-----------|-------------|
| Dev login | `POST /api/auth/dev` calls vire-server `POST /api/auth/oauth` with `provider: "dev"`, sets `vire_session` cookie with server-signed JWT, redirects to `/dashboard` |
| Session | `vire_session` HttpOnly SameSite=Lax cookie containing HMAC-SHA256 signed JWT from vire-server |
| JWT claims | `sub`, `email`, `name`, `provider`, `iss`, `iat`, `exp` — validated with HMAC-SHA256 signature (when `jwt_secret` is configured) |
| Logout | Clears cookie, redirects to `/` |
| LoggedIn check | `IsLoggedIn(r, jwtSecret)` — validates JWT signature and expiry |
| User resolution | `IsLoggedIn` returns `*JWTClaims` with `Sub` field → `vireClient.GetUser(claims.Sub)` |
| Google login | `GET /api/auth/login/google` → 302 redirect to `{apiURL}/api/auth/login/google?callback={callbackURL}` |
| GitHub login | `GET /api/auth/login/github` → 302 redirect to `{apiURL}/api/auth/login/github?callback={callbackURL}` |
| OAuth callback | `GET /auth/callback?token=<jwt>` → sets `vire_session` cookie, redirects to `/dashboard` |

## Target Architecture

```
Browser                    Portal                     vire-server
  │                          │                            │
  │  GET /api/auth/login/    │                            │
  │  google                  │                            │
  │─────────────────────────▶│                            │
  │                          │                            │
  │  302 → Google OAuth      │                            │
  │◀─────────────────────────│                            │
  │                          │                            │
  │  (user authenticates     │                            │
  │   with Google)           │                            │
  │                          │                            │
  │  GET /auth/callback?     │                            │
  │  code=xxx&state=yyy      │                            │
  │─────────────────────────▶│                            │
  │                          │  POST /api/auth/oauth      │
  │                          │  { provider, code, state } │
  │                          │───────────────────────────▶│
  │                          │  { token, user }           │
  │                          │◀───────────────────────────│
  │                          │                            │
  │  Set-Cookie: vire_session│                            │
  │  302 → /dashboard        │                            │
  │◀─────────────────────────│                            │
```

The portal handles the OAuth redirect dance (building the authorization URL, receiving the callback). vire-server handles the code exchange with the provider and issues a signed JWT.

### Dev Login Flow (same endpoint as OAuth)

```
Browser                    Portal                     vire-server
  │                          │                            │
  │  POST /api/auth/dev      │                            │
  │─────────────────────────▶│                            │
  │                          │  POST /api/auth/oauth      │
  │                          │  { provider: "dev",        │
  │                          │    code: "dev",            │
  │                          │    state: <generated> }    │
  │                          │───────────────────────────▶│
  │                          │  { token, user }           │
  │                          │◀───────────────────────────│
  │                          │                            │
  │  Set-Cookie: vire_session│                            │
  │  302 → /dashboard        │
  │◀─────────────────────────│
```

Dev login hits the same `POST /api/auth/oauth` endpoint as Google and GitHub. The portal supplies `provider: "dev"` with synthetic `code` and `state` values. vire-server's oauth handler switches on the provider:

- `google` / `github` → exchange code with external provider
- `dev` → skip external exchange, create/return test user (dev mode only, rejected in prod)

This means:
- Dev login exercises the **exact same code path** as real OAuth — same endpoint, same JWT signing, same cookie handling
- vire-server is the single JWT issuer (no unsigned tokens, no separate dev endpoint)
- Any bug in the OAuth flow is caught by dev login testing
- The `vire_session` cookie format is identical for all auth methods

## Auth Methods

### 1. Google OAuth

| Step | Component | Detail |
|------|-----------|--------|
| Initiate | Portal | `GET /api/auth/login/google` → build Google authorization URL with `client_id`, `redirect_uri`, `scope`, `state` → 302 redirect |
| Callback | Portal | `GET /auth/callback` → extract `code` and `state` from query params |
| Exchange | vire-server | `POST /api/auth/oauth` with `{ provider: "google", code, state }` → exchange code with Google, create/update user, return signed JWT |
| Session | Portal | Set `vire_session` cookie with JWT, redirect to `/dashboard` |

**Scopes:** `openid`, `email`, `profile`
**User fields:** email, name, picture

### 2. GitHub OAuth

Same flow as Google with different provider URLs and scopes.

**Scopes:** `read:user`, `user:email`
**User fields:** email, login, name, avatar_url

### 3. Email/Password

| Step | Component | Detail |
|------|-----------|--------|
| Login form | Portal | `POST /api/auth/login` with `{ email, password }` |
| Verify | vire-server | `POST /api/auth/login` → verify credentials (bcrypt), return signed JWT |
| Session | Portal | Set `vire_session` cookie with JWT, redirect to `/dashboard` |

Email/password registration is out of scope for the initial implementation. Users are created via the server's user import or management API.

### 4. Dev Login (same endpoint, synthetic credentials)

| Step | Component | Detail |
|------|-----------|--------|
| Click | Portal | `POST /api/auth/dev` (dev mode only, 404 in prod) |
| Exchange | Portal → vire-server | `POST /api/auth/oauth` with `{ provider: "dev", code: "dev", state: <generated> }` |
| Server logic | vire-server | Sees `provider: "dev"` → skips external code exchange, creates/returns test user with signed JWT (rejected if not dev mode) |
| Session | Portal | Set `vire_session` cookie with JWT, redirect to `/dashboard` |

Dev login uses the same `/api/auth/oauth` endpoint as Google and GitHub. The only difference is the provider value and the fact that no external redirect occurs. vire-server's handler branches on provider:

```
switch provider {
case "google":  exchangeWithGoogle(code)
case "github":  exchangeWithGitHub(code)
case "dev":     if !devMode { reject }; createTestUser()
}
```

## JWT Design

### Current (unsigned, portal-issued)

```json
{
  "alg": "none",
  "typ": "JWT"
}
{
  "sub": "dev_user",
  "email": "bobmcallan@gmail.com",
  "iss": "vire-dev",
  "exp": 1739750400
}
```

### Target (signed, server-issued)

```json
{
  "alg": "HS256",
  "typ": "JWT"
}
{
  "sub": "user-uuid-or-username",
  "email": "user@example.com",
  "name": "Display Name",
  "provider": "google|github|email|dev",
  "iss": "vire-server",
  "iat": 1739750400,
  "exp": 1739836800
}
```

- **Signing:** HMAC-SHA256 with shared secret between portal and vire-server
- **Expiry:** 24 hours (configurable)
- **Issuer:** `vire-server` (single issuer for all auth methods)
- **Validation:** Portal validates signature and expiry on every request (currently it only checks cookie existence)

## Portal Changes Required

### Configuration (Implemented)

Auth config in `internal/config/config.go`:

```go
type AuthConfig struct {
    JWTSecret   string `toml:"jwt_secret"`
    CallbackURL string `toml:"callback_url"`
}
```

TOML config:
```toml
[auth]
jwt_secret = ""
callback_url = "http://localhost:4241/auth/callback"
```

Environment overrides:
```
VIRE_AUTH_JWT_SECRET
VIRE_AUTH_CALLBACK_URL
```

The portal does not hold OAuth client secrets. All OAuth complexity is in vire-server (server-side redirect approach). The portal only needs `jwt_secret` (for validating signed tokens) and `callback_url` (for constructing redirect URLs).

**Note:** When `jwt_secret` is empty (the default), signature verification is skipped. This is acceptable for local dev but must be set in production.

### Auth Handler Changes (`internal/handlers/auth.go`)

| Method | Route | Change |
|--------|-------|--------|
| `HandleGoogleLogin` | `GET /api/auth/login/google` | New — build Google authorization URL, generate state, 302 redirect |
| `HandleGitHubLogin` | `GET /api/auth/login/github` | New — build GitHub authorization URL, generate state, 302 redirect |
| `HandleOAuthCallback` | `GET /auth/callback` | New — extract code/state, POST to vire-server, set cookie, redirect |
| `HandleDevLogin` | `POST /api/auth/dev` | Modify — call vire-server `POST /api/auth/oauth` with `provider: "dev"` instead of building unsigned JWT locally |
| `HandleLogout` | `POST /api/auth/logout` | No change (already works) |

### JWT Validation

Add middleware or helper to validate JWT signature and expiry:

```go
func ValidateJWT(token string, secret []byte) (*Claims, error) {
    // Split into header.payload.signature
    // Verify HMAC-SHA256 signature
    // Check expiry
    // Return claims
}
```

Update `LoggedIn` checks across handlers: replace `cookieErr == nil` with `ValidateJWT(cookie.Value, secret)`.

### New Routes

| Route | Handler | Description |
|-------|---------|-------------|
| `GET /api/auth/login/google` | AuthHandler | Redirect to Google OAuth |
| `GET /api/auth/login/github` | AuthHandler | Redirect to GitHub OAuth |
| `GET /auth/callback` | AuthHandler | OAuth callback (receives code + state) |

### Landing Page

No HTML changes needed. The existing links already point to `/api/auth/login/google` and `/api/auth/login/github`.

## vire-server Changes Required

### New Endpoints

| Route | Method | Description |
|-------|--------|-------------|
| `POST /api/auth/oauth` | Exchange credentials for JWT | Receives `{ provider, code, state }`. Switches on provider: `google`/`github` → exchange code with external provider; `dev` → skip exchange, create test user (dev mode only). All paths create/update user and return `{ token, user }` |
| `POST /api/auth/login` | Email/password login | Receives `{ email, password }`, verifies bcrypt, returns `{ token, user }` |
| `POST /api/auth/validate` | Validate JWT | Receives `Authorization: Bearer <jwt>`, returns user profile |

Note: there is no separate `/api/auth/dev-login` endpoint. Dev login flows through `/api/auth/oauth` with `provider: "dev"`.

### Configuration

```toml
[auth]
jwt_secret = "shared-secret-with-portal"
token_expiry = "24h"

[auth.google]
client_id = ""
client_secret = ""

[auth.github]
client_id = ""
client_secret = ""
```

The server holds the OAuth client secrets and performs the code exchange. The portal only needs the client IDs (for building the authorization URL) and the shared JWT secret (for validating tokens).

**Alternative:** The portal sends the code to vire-server which holds both client ID and secret. The portal doesn't need OAuth secrets at all — only the callback URL. This is cleaner:

```toml
# Portal config — no OAuth secrets
[auth]
jwt_secret = "shared-secret"
callback_url = "http://localhost:4241/auth/callback"
```

```toml
# Server config — holds all OAuth secrets
[auth]
jwt_secret = "shared-secret"
token_expiry = "24h"

[auth.google]
client_id = "..."
client_secret = "..."

[auth.github]
client_id = "..."
client_secret = "..."
```

With this approach the portal builds the authorization URL using the server's `GET /api/auth/config` endpoint which returns the client IDs and scopes (no secrets). Or simpler: the portal redirects to the server's login endpoint which does the full redirect.

### Recommended: Server-Side Redirect

The simplest approach — the portal doesn't handle OAuth at all:

```
Browser → GET /api/auth/login/google
Portal  → 302 to vire-server: GET {api_url}/api/auth/login/google?callback={portal_callback_url}
Server  → 302 to Google OAuth (with server's client_id, redirect_uri pointing back to server)
Google  → callback to server with code
Server  → exchanges code, creates user, generates JWT
Server  → 302 to portal callback: GET {portal_callback_url}?token={jwt}
Portal  → sets vire_session cookie, 302 to /dashboard
```

This keeps all OAuth complexity in vire-server. The portal only:
1. Redirects to `{api_url}/api/auth/login/{provider}?callback={callback_url}`
2. Receives the JWT as a query parameter on the callback
3. Sets the session cookie

## Implementation Phases

### Phase 1: `/api/auth/oauth` endpoint + Dev Login -- COMPLETE

**Portal (done):**
- Added `AuthConfig` to config (`jwt_secret`, `callback_url`) with env overrides (`VIRE_AUTH_JWT_SECRET`, `VIRE_AUTH_CALLBACK_URL`)
- Modified `HandleDevLogin` to call vire-server `POST /api/auth/oauth` with `provider: "dev"`
- Added `ValidateJWT` with HMAC-SHA256 signature verification and expiry check
- Added `IsLoggedIn(r, jwtSecret)` helper used by all handlers
- Added `GET /auth/callback` route (receives `?token=` from server)
- Added `GET /api/auth/login/google` and `GET /api/auth/login/github` redirect routes
- Added `ExchangeOAuth` to VireClient
- Updated all handler constructors to accept `jwtSecret`
- Removed `buildDevJWT()` — JWTs are now issued exclusively by vire-server

**vire-server (done):**
- `POST /api/auth/oauth` endpoint accepts `{ provider, code, state }`, switches on provider
- Returns `{ status: "ok", data: { token, user } }` with HMAC-SHA256 signed JWT

**Test:** Dev login calls the same endpoint that Google/GitHub will use. Produces a signed JWT. All handlers validate it. 29 auth unit tests + 33 stress tests pass.

### Phase 2: Google OAuth

**Portal:**
- Add `HandleGoogleLogin` — redirects to `{api_url}/api/auth/login/google?callback={callback_url}`

**vire-server:**
- Add `GET /api/auth/login/google` — builds Google authorization URL, 302 redirect
- Add `GET /api/auth/callback/google` — receives code from Google, exchanges, returns JWT to portal callback
- Add Google OAuth config

### Phase 3: GitHub OAuth

Same pattern as Google, different provider URLs and scopes.

### Phase 4: Email/Password

**Portal:**
- Add login form to landing page
- Add `POST /api/auth/login` handler — forwards credentials to vire-server

**vire-server:**
- Add `POST /api/auth/login` — bcrypt verification, returns signed JWT

## Security Considerations

| Concern | Mitigation |
|---------|-----------|
| CSRF on OAuth | State parameter generated by server, validated on callback |
| Token theft | HttpOnly, SameSite=Lax cookies; no JWT in localStorage |
| Unsigned JWT (current) | Phase 1 replaces with HMAC-SHA256 signed tokens |
| Secret management | JWT secret shared between portal and server via config/env; OAuth secrets only on server |
| Token expiry | 24h default; server validates exp claim |
| XSS | CSP headers already in place; HttpOnly cookies not accessible to JS |

## Files Expected to Change

### Portal
- `internal/config/config.go` — add `AuthConfig`
- `internal/config/defaults.go` — add auth defaults
- `internal/config/config_test.go` — test auth config loading
- `internal/handlers/auth.go` — rewrite dev login, add OAuth handlers, add callback
- `internal/handlers/auth_test.go` — new test file
- `internal/handlers/settings.go` — update `ExtractJWTSub` → `ValidateJWT`
- `internal/handlers/dashboard.go` — update JWT validation
- `internal/handlers/landing.go` — update `LoggedIn` check
- `internal/server/routes.go` — add new auth routes
- `internal/client/vire_client.go` — add auth API methods
- `config/vire-portal.toml.example` — add `[auth]` section
- `.claude/skills/develop/SKILL.md` — update routes table

### vire-server
- Auth handler with `/api/auth/oauth` (handles dev, google, github providers), `/api/auth/login`
- JWT signing utility
- Auth config (jwt_secret, token_expiry, OAuth provider configs)
- User creation/lookup on OAuth callback
