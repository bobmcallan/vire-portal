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
| MCP OAuth 2.1 | Not started | Required for Claude connector directory listing |

## Architecture

```
Browser                    Portal (:4241)              vire-server
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
| POST | `/api/auth/login` | AuthHandler | Email/password login (forwards to vire-server) |
| POST | `/api/auth/logout` | AuthHandler | Clear session cookie |
| GET | `/api/auth/login/google` | AuthHandler | Redirect to vire-server Google OAuth |
| GET | `/api/auth/login/github` | AuthHandler | Redirect to vire-server GitHub OAuth |
| GET | `/auth/callback` | AuthHandler | Receive JWT from vire-server, set cookie |

## Configuration

### Portal

```toml
[auth]
jwt_secret = ""                                    # shared with vire-server
callback_url = "http://localhost:4241/auth/callback"
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
| `internal/handlers/auth.go` | JWT validation, `HandleLogin`, OAuth handlers |
| `internal/handlers/auth_test.go` | Auth unit tests |
| `internal/handlers/settings.go` | Settings page (includes dev JWT debug) |
| `internal/config/config.go` | AuthConfig struct, env overrides |
| `internal/config/defaults.go` | Default auth values |
| `internal/client/vire_client.go` | `ExchangeOAuth`, `GetUser`, `UpdateUser` |
| `internal/server/routes.go` | Route registration |
| `pages/settings.html` | Settings template with dev auth debug section |
| `pages/landing.html` | Landing page with login buttons |

## MCP OAuth 2.1 (Future)

For Claude connector directory listing, vire-portal needs to implement OAuth 2.1 as an authorization server. This involves:

- `/.well-known/oauth-authorization-server` metadata endpoint
- `/register` for Dynamic Client Registration (RFC 7591)
- `/authorize` endpoint with PKCE (S256)
- `/token` endpoint for code exchange and refresh
- Bearer token validation on the `/mcp` endpoint

The flow is a two-hop OAuth: Claude authenticates with Vire (MCP OAuth 2.1), and Vire delegates user authentication to Google/GitHub (standard OAuth 2.0). See `docs/authentication/mcp-ouath.md` for the full specification.

## Phases

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | Email/password login + JWT signing/validation | Complete |
| 2 | Google OAuth (server-side exchange) | Redirect stub in portal |
| 3 | GitHub OAuth | Redirect stub in portal |
| 4 | MCP OAuth 2.1 (Claude connector) | Not started |
