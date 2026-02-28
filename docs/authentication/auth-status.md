# Authentication Status

**Date:** 2026-02-16

## Current State

Email/password login is implemented. The portal forwards credentials to vire-server `POST /api/auth/login`, which performs bcrypt verification and returns an HMAC-SHA256 signed JWT. Google and GitHub OAuth redirect stubs are in place. The dev login button on the landing page (dev mode only) submits the imported `dev_user` credentials through the real email/password auth path.

| Feature | Status | Notes |
|---------|--------|-------|
| Email/password login | Done | `POST /api/auth/login` forwards to vire-server, bcrypt verified, signed JWT |
| Dev login (convenience) | Done | Landing page form submits `dev_user`/`dev123` via email/password login (same path as real users) |
| JWT signing | Done | HMAC-SHA256, shared secret between portal and vire-server |
| JWT validation | Done | Signature + expiry checked on every request via `IsLoggedIn` |
| Google OAuth redirect | Stub | Portal redirects to vire-server; server-side exchange not yet wired |
| GitHub OAuth redirect | Stub | Same pattern as Google |
| MCP OAuth 2.1 | Done | DCR, authorize, token endpoints; Bearer token on `/mcp`; in-memory L1 cache with write-through to vire-server |

## Architecture

```
Browser                    Portal (:8500)              vire-server
  |                          |                            |
  |  POST /api/auth/login   |                            |
  |  { username, password }  |                            |
  |------------------------->|                            |
  |                          |  POST /api/auth/login      |
  |                          |  { username, password }    |
  |                          |--------------------------->|
  |                          |                            |  bcrypt verify
  |                          |  { token, user }           |
  |                          |<---------------------------|
  |                          |                            |
  |  Set-Cookie: vire_session|                            |
  |  302 -> /dashboard       |                            |
  |<-------------------------|                            |
```

The portal never holds OAuth client secrets. vire-server performs all credential verification, provider exchanges, and is the single JWT issuer.

## Auth Methods

### Email/Password Login

1. Browser POSTs to `/api/auth/login` with `{ username, password }` (form-encoded)
2. Portal forwards as JSON to vire-server `POST /api/auth/login`
3. vire-server verifies password via bcrypt, returns `{ status: "ok", data: { token, user } }` with signed JWT (`provider: "email"`)
4. Portal sets `vire_session` cookie, redirects to `/dashboard`

### Dev Login (convenience button)

The dev login button on the landing page (visible only in dev mode) submits a form with `username=dev_user` and `password=dev123` to `POST /api/auth/login`. This goes through the exact same email/password auth path as any real user. The `dev_user` account is imported into vire-server from `import/users.json`:

```json
{
  "username": "dev_user",
  "email": "bobmcallan@gmail.com",
  "password": "dev123",
  "role": "developer"
}
```

There is no synthetic `/api/auth/dev` endpoint or fake `provider: "dev"` bypass. Dev login exercises the real bcrypt verification and email provider JWT signing.

### Google OAuth (Phase 2 -- redirect stub)

1. Browser hits `GET /api/auth/login/google`
2. Portal redirects to `{apiURL}/api/auth/login/google?callback={callbackURL}`
3. vire-server redirects to Google, handles code exchange, issues JWT
4. vire-server redirects to `{callbackURL}?token={jwt}`
5. Portal sets `vire_session` cookie via `GET /auth/callback?token=...`

### GitHub OAuth (Phase 3 -- redirect stub)

Same pattern as Google with different provider URLs and scopes.

## JWT

### Format

```json
{
  "alg": "HS256",
  "typ": "JWT"
}
{
  "sub": "user-uuid",
  "email": "user@example.com",
  "name": "Display Name",
  "provider": "google|github|email",
  "iss": "vire-server",
  "iat": 1739750400,
  "exp": 1739836800
}
```

- **Signing:** HMAC-SHA256 with shared secret (`VIRE_AUTH_JWT_SECRET`)
- **Expiry:** 24 hours
- **Issuer:** `vire-server` (all auth methods)
- **Storage:** `vire_session` HttpOnly SameSite=Lax cookie

### Validation

`ValidateJWT(token, secret)` in `internal/handlers/auth.go`:
1. Splits token into 3 parts (header.payload.signature)
2. Verifies HMAC-SHA256 signature if secret is non-empty
3. Decodes and unmarshals payload into `JWTClaims`
4. Checks `exp` claim against current time

`IsLoggedIn(r, jwtSecret)` reads the `vire_session` cookie and calls `ValidateJWT`. Returns `(bool, *JWTClaims)`.

When `jwt_secret` is empty (default), signature verification is skipped. This is acceptable for local dev but must be set in production.

## Portal Routes

| Method | Route | Handler | Description |
|--------|-------|---------|-------------|
| GET | `/.well-known/oauth-authorization-server` | OAuthServer | OAuth 2.1 authorization server metadata |
| GET | `/.well-known/oauth-protected-resource` | OAuthServer | OAuth 2.1 protected resource metadata |
| POST | `/register` | OAuthServer | Dynamic Client Registration (RFC 7591) |
| GET | `/authorize` | OAuthServer | OAuth authorization endpoint (PKCE S256) |
| POST | `/token` | OAuthServer | Token exchange (authorization_code + refresh_token) |
| POST | `/api/auth/login` | AuthHandler | Email/password login (forwards to vire-server) |
| POST | `/api/auth/logout` | AuthHandler | Clear session cookie |
| GET | `/api/auth/login/google` | AuthHandler | Redirect to vire-server Google OAuth |
| GET | `/api/auth/login/github` | AuthHandler | Redirect to vire-server GitHub OAuth |
| GET | `/auth/callback` | AuthHandler | Receive JWT from vire-server, set cookie (+ MCP session completion) |

## Configuration

### Portal

```toml
[auth]
jwt_secret = ""                                    # shared with vire-server
callback_url = "http://localhost:8500/auth/callback"
```

Environment overrides: `VIRE_AUTH_JWT_SECRET`, `VIRE_AUTH_CALLBACK_URL`

### vire-server

```toml
[auth]
jwt_secret = ""       # must match portal
token_expiry = "24h"

[auth.google]
client_id = ""
client_secret = ""

[auth.github]
client_id = ""
client_secret = ""
```

## Dev Mode

Controlled by `VIRE_ENV=dev` or `environment = "dev"` in TOML config.

Effects on auth:
- Landing page shows DEV LOGIN button (submits `dev_user`/`dev123` via `/api/auth/login`)
- Settings page shows AUTH DEBUG section with JWT claims, token, and a reauthenticate button

## Key Files

| File | Purpose |
|------|---------|
| `internal/auth/server.go` | OAuthServer (central state, JWT minting, auth completion, backend initialization) |
| `internal/auth/backend.go` | OAuthBackend (HTTP client for vire-server internal OAuth API, write-through/read-through persistence) |
| `internal/auth/store.go` | ClientStore, CodeStore, TokenStore (L1 in-memory cache with write-through to backend) |
| `internal/auth/session.go` | SessionStore for pending MCP auth sessions (L1 in-memory cache with read-through to backend, TTL 10 min) |
| `internal/auth/pkce.go` | PKCE S256 verification (constant-time compare) |
| `internal/auth/dcr.go` | HandleRegister (POST /register, RFC 7591 DCR) |
| `internal/auth/authorize.go` | HandleAuthorize (GET /authorize, PKCE + session + redirect) |
| `internal/auth/token.go` | HandleToken (POST /token, auth_code + refresh_token grants) |
| `internal/auth/discovery.go` | .well-known OAuth discovery endpoints |
| `internal/handlers/auth.go` | JWT validation, `HandleLogin`, OAuth handlers, MCP session completion |
| `internal/handlers/auth_test.go` | Auth unit tests |
| `internal/handlers/profile.go` | Profile page (user info + API key management, includes dev JWT debug) |
| `internal/mcp/handler.go` | MCP handler (Bearer token + cookie auth, JWT validation) |
| `internal/config/config.go` | AuthConfig struct, env overrides |
| `internal/config/defaults.go` | Default auth values |
| `internal/client/vire_client.go` | `ExchangeOAuth`, `GetUser`, `UpdateUser` |
| `internal/server/routes.go` | Route registration |
| `pages/profile.html` | Profile template (user info section) with dev auth debug section |
| `pages/landing.html` | Landing page with login buttons |

## MCP OAuth 2.1

The portal implements a complete MCP OAuth 2.1 Authorization Server for Claude CLI, Claude Desktop, and claude.ai. The flow is a two-hop OAuth: Claude authenticates with Vire (MCP OAuth 2.1), and Vire delegates user authentication to Google/GitHub (standard OAuth 2.0). See `docs/authentication/mcp-ouath.md` for the full specification and `docs/authentication/mcp-oauth-implementation-steps.md` for implementation details.

Implementation:
- `/.well-known/oauth-authorization-server` -- OAuth metadata discovery
- `/.well-known/oauth-protected-resource` -- Protected resource metadata
- `POST /register` -- Dynamic Client Registration (RFC 7591, UUID client IDs, random secrets)
- `GET /authorize` -- Authorization endpoint (PKCE S256, session tracking via `mcp_session_id` cookie, lenient auto-registration for unknown clients)
- `POST /token` -- Token exchange (authorization_code grant with PKCE verification, refresh_token grant with token rotation)
- Bearer token on `POST /mcp` -- JWT signature + expiry validation, cookie fallback for web dashboard

Storage: L1 in-memory cache (RWMutex-protected) for clients, sessions (10 min TTL), auth codes (5 min TTL, single-use), and refresh tokens (7 day TTL). All writes are persisted to vire-server's internal OAuth API for L2 persistence across restarts. Reads fall back to backend on cache miss.

## Phases

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | Email/password login + JWT signing/validation | Complete |
| 2 | Google OAuth (server-side exchange) | Redirect stub in portal |
| 3 | GitHub OAuth | Redirect stub in portal |
| 4 | MCP OAuth 2.1 discovery (`.well-known` endpoints) | Complete |
| 5 | MCP OAuth 2.1 DCR (`POST /register`) | Complete |
| 6 | MCP OAuth 2.1 authorize + token + Bearer | Complete |
