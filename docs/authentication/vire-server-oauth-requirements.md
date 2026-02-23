# vire-server: OAuth Provider Requirements

**Date:** 2026-02-23
**Status:** Requirements for Phase 2 & 3
**Audience:** vire-server developer

## Summary

The portal proxies all OAuth requests to vire-server — the browser never contacts vire-server directly. The portal makes server-side HTTP requests to vire-server and forwards responses (redirects, errors) back to the browser.

Portal proxy handlers call vire-server at:

```
GET {apiURL}/api/auth/login/google?callback={portalCallbackURL}
GET {apiURL}/api/auth/login/github?callback={portalCallbackURL}
GET {apiURL}/api/auth/callback/google?state=...&code=...&scope=...
GET {apiURL}/api/auth/callback/github?state=...&code=...
```

vire-server needs to handle these requests, redirect to the provider, receive the authorization code, exchange it for user info, mint a signed JWT, and redirect back to the portal callback with the token.

**Critical constraint:** vire-server is never exposed to the browser. All four endpoints above are called by the portal's Go HTTP client (server-side proxy), not by the browser. The portal forwards its external Host header on these requests so vire-server can build externally-reachable URLs.

## Current State

### What exists (portal)

| Component | Status | Detail |
|-----------|--------|--------|
| `GET /api/auth/login/google` | Done | Server-side proxy to vire-server; captures redirect Location, forwards to browser |
| `GET /api/auth/login/github` | Done | Server-side proxy to vire-server; captures redirect Location, forwards to browser |
| `GET /api/auth/callback/google` | Done | Server-side proxy to vire-server; forwards response (redirect or error) to browser |
| `GET /api/auth/callback/github` | Done | Server-side proxy to vire-server; forwards response (redirect or error) to browser |
| `GET /auth/callback?token=<jwt>` | Done | Sets `vire_session` cookie, redirects to `/dashboard` |
| JWT validation (HMAC-SHA256) | Done | `ValidateJWT(token, secret)` checks signature + expiry |
| MCP session completion on callback | Done | If `mcp_session_id` cookie present, completes MCP OAuth flow |
| Email/password login | Done | `POST /api/auth/login` forwards to vire-server |
| Host header forwarding | Done | Portal forwards external Host (e.g. `localhost:8880`) so vire-server can build `redirect_uri` |

### What exists (vire-server)

| Component | Status | Detail |
|-----------|--------|--------|
| `POST /api/auth/login` | Done | Email/password login with bcrypt, returns signed JWT |
| `POST /api/auth/oauth` | Done | Provider-based exchange (currently only handles `provider: "dev"`) |
| JWT signing (HMAC-SHA256) | Done | Shared secret with portal |
| User creation/lookup | Done | Users stored in SQLite, upsert support |

### What is missing (vire-server)

| Component | Priority | Detail |
|-----------|----------|--------|
| `GET /api/auth/login/google` | Phase 2 | Accept `?callback=`, redirect to Google |
| `GET /api/auth/callback/google` | Phase 2 | Receive code from Google, exchange, redirect to portal |
| `GET /api/auth/login/github` | Phase 3 | Accept `?callback=`, redirect to GitHub |
| `GET /api/auth/callback/github` | Phase 3 | Receive code from GitHub, exchange, redirect to portal |
| Google OAuth config | Phase 2 | `client_id`, `client_secret` |
| GitHub OAuth config | Phase 3 | `client_id`, `client_secret` |
| Provider user mapping | Phase 2 | Map Google/GitHub profile to vire user |

## Full OAuth Flow

All communication between the browser and vire-server is proxied through the portal. The browser never sees vire-server's internal address.

```
Browser                Portal (public)            vire-server (internal)    Provider
  |                        |                            |                       |
  |  click "Sign in        |                            |                       |
  |  with Google"          |                            |                       |
  |----------------------->|                            |                       |
  |                        |                            |                       |
  |                        |  GET /api/auth/login/      |                       |
  |                        |  google?callback=...       |                       |
  |                        |  Host: portal-public-host  |                       |
  |                        |--------------------------->|                       |
  |                        |                            |                       |
  |                        |             generate state |                       |
  |                        |   store {state -> callback}|                       |
  |                        |   build redirect_uri from  |                       |
  |                        |   Host header              |                       |
  |                        |                            |                       |
  |                        |  302 Location: https://    |                       |
  |                        |  accounts.google.com/...   |                       |
  |                        |  &redirect_uri=https://    |                       |
  |                        |  portal-host/api/auth/     |                       |
  |                        |  callback/google&state=... |                       |
  |                        |<---------------------------|                       |
  |                        |                            |                       |
  |  302 to Google         |                            |                       |
  |  (portal forwards      |                            |                       |
  |   Location header)     |                            |                       |
  |<-----------------------|                            |                       |
  |                                                                             |
  |  user authenticates with Google                                             |
  |---------------------------------------------------------------------------->|
  |                                                                             |
  |  302 to portal /api/auth/callback/google?code=...&state=...                 |
  |<----------------------------------------------------------------------------|
  |                                                                             |
  |  GET /api/auth/callback/google?code=...&state=...                           |
  |----------------------->|                            |                       |
  |                        |                            |                       |
  |                        |  GET /api/auth/callback/   |                       |
  |                        |  google?code=...&state=... |                       |
  |                        |  Host: portal-public-host  |                       |
  |                        |--------------------------->|                       |
  |                        |                            |                       |
  |                        |              validate state|                       |
  |                        |             lookup callback|                       |
  |                        |                            |  POST token endpoint  |
  |                        |                            |  exchange code        |
  |                        |                            |---------------------->|
  |                        |                            |  { access_token }     |
  |                        |                            |<----------------------|
  |                        |                            |                       |
  |                        |                            |  GET /userinfo        |
  |                        |                            |---------------------->|
  |                        |                            |  { email, name, ... } |
  |                        |                            |<----------------------|
  |                        |                            |                       |
  |                        |          create/update user|                       |
  |                        |          mint JWT (HS256)  |                       |
  |                        |                            |                       |
  |                        |  302 to portal             |                       |
  |                        |  /auth/callback?token={jwt}|                       |
  |                        |<---------------------------|                       |
  |                        |                            |                       |
  |  302 to /auth/callback |                            |                       |
  |  ?token={jwt}          |                            |                       |
  |  (portal forwards      |                            |                       |
  |   redirect response)   |                            |                       |
  |<-----------------------|                            |                       |
  |                                                                             |
  |  GET /auth/callback?token={jwt}                                             |
  |----------------------->|                            |                       |
  |                        |                            |                       |
  |  Set-Cookie:           |                            |                       |
  |  vire_session={jwt}    |                            |                       |
  |  302 to /dashboard     |                            |                       |
  |<-----------------------|                            |                       |
```

## vire-server Endpoints Required

### 1. `GET /api/auth/login/google`

Initiates the Google OAuth flow.

**Query parameters:**

| Param | Required | Description |
|-------|----------|-------------|
| `callback` | Yes | Portal callback URL to redirect to after exchange (e.g. `http://localhost:8880/auth/callback`) |

**Request context:**

The portal forwards its public Host header on this request. vire-server **must** use the request's Host header (not its own internal address) when building the `redirect_uri`, because the browser will follow Google's redirect back to that URI, and the browser can only reach the portal's public address.

**Behavior:**

1. Generate a random `state` value (min 32 bytes, base64url encoded)
2. Store `{ state -> callback }` with a TTL (10 minutes)
3. Build `redirect_uri` from the **request's Host header** and scheme:
   - `https://{request.Host}/api/auth/callback/google` (production)
   - `http://{request.Host}/api/auth/callback/google` (development)
   - The scheme can be determined from `X-Forwarded-Proto` header if present, or from config
4. Build Google authorization URL:
   - Endpoint: `https://accounts.google.com/o/oauth2/v2/auth`
   - `client_id`: from config `[auth.google].client_id`
   - `redirect_uri`: built in step 3 above
   - `response_type`: `code`
   - `scope`: `openid email profile`
   - `state`: generated state value
   - `access_type`: `offline` (optional, for refresh tokens)
5. Store the `redirect_uri` alongside the state (needed for token exchange in callback)
6. 302 redirect to Google authorization URL

**Error cases:**
- Missing `callback` param: 400 Bad Request
- Google OAuth not configured: 501 Not Implemented (or redirect to portal with `?error=provider_not_configured`)

### 2. `GET /api/auth/callback/google`

Receives the authorization code from Google after user consent. This request arrives proxied through the portal, with the portal's public Host header forwarded.

**Query parameters:**

| Param | Required | Description |
|-------|----------|-------------|
| `code` | Yes | Authorization code from Google |
| `state` | Yes | State parameter for CSRF validation |
| `scope` | No | Granted scopes (informational) |
| `error` | No | Error from Google (e.g. `access_denied`) |

**Behavior:**

1. If `error` param present: redirect to portal with `?error={error}`
2. Validate `state` against stored values; reject if missing or expired
3. Look up the portal `callback` URL and `redirect_uri` from the state store
4. Exchange `code` for tokens with Google:
   - POST `https://oauth2.googleapis.com/token`
   - Body: `{ code, client_id, client_secret, redirect_uri, grant_type: "authorization_code" }`
   - The `redirect_uri` **must exactly match** what was sent to Google in step 3 of the login endpoint — retrieve it from the state store
   - Response: `{ access_token, id_token, ... }`
5. Fetch user profile from Google:
   - GET `https://www.googleapis.com/oauth2/v2/userinfo` with `Authorization: Bearer {access_token}`
   - Response: `{ id, email, name, picture, verified_email }`
6. Create or update user in database:
   - Match by email
   - Set `provider: "google"`, store Google user ID
   - Set `name`, `picture` if provided
7. Mint HMAC-SHA256 signed JWT:
   ```json
   {
     "sub": "<vire-user-id>",
     "email": "<email>",
     "name": "<display-name>",
     "provider": "google",
     "iss": "vire-server",
     "iat": <now>,
     "exp": <now + 24h>
   }
   ```
8. Delete the state entry (single-use)
9. 302 redirect to `{callback}?token={jwt}`

**Error cases:**
- Missing `code` or `state`: 400 Bad Request
- Invalid/expired `state`: 403 Forbidden (or redirect to portal with error)
- Google token exchange fails: redirect to portal with `?error=exchange_failed`
- Google userinfo fetch fails: redirect to portal with `?error=profile_failed`

### 3. `GET /api/auth/login/github`

Same pattern as Google. Initiates the GitHub OAuth flow.

**Differences from Google:**

| | Google | GitHub |
|---|--------|--------|
| Authorization URL | `https://accounts.google.com/o/oauth2/v2/auth` | `https://github.com/login/oauth/authorize` |
| Scopes | `openid email profile` | `read:user user:email` |
| Config | `[auth.google]` | `[auth.github]` |
| Callback route | `/api/auth/callback/google` | `/api/auth/callback/github` |

**Query parameters:** Same as Google (`callback` required).

**Request context:** Same as Google — use the forwarded Host header to build `redirect_uri`.

### 4. `GET /api/auth/callback/github`

Receives the authorization code from GitHub.

**Differences from Google callback:**

| Step | Google | GitHub |
|------|--------|--------|
| Token exchange | POST `https://oauth2.googleapis.com/token` (JSON) | POST `https://github.com/login/oauth/access_token` (Accept: application/json) |
| User profile | GET `https://www.googleapis.com/oauth2/v2/userinfo` | GET `https://api.github.com/user` |
| Email (if private) | Included in userinfo | GET `https://api.github.com/user/emails` (pick primary verified) |
| User ID field | `id` (numeric string) | `id` (integer), `login` (username) |
| Provider claim | `"google"` | `"github"` |

**GitHub token exchange:**
```
POST https://github.com/login/oauth/access_token
Accept: application/json
Content-Type: application/json

{
  "client_id": "...",
  "client_secret": "...",
  "code": "...",
  "redirect_uri": "..."
}
```

Response: `{ "access_token": "...", "token_type": "bearer", "scope": "..." }`

**GitHub user profile:**
```
GET https://api.github.com/user
Authorization: Bearer {access_token}
```

Response: `{ "id": 12345, "login": "octocat", "email": "...", "name": "...", "avatar_url": "..." }`

If `email` is null (private), fetch emails:
```
GET https://api.github.com/user/emails
Authorization: Bearer {access_token}
```

Response: `[ { "email": "...", "primary": true, "verified": true }, ... ]`

Pick the first entry where `primary == true && verified == true`.

## redirect_uri: Critical Requirement

Because vire-server sits behind the portal, it cannot use its own address for `redirect_uri`. The browser must be redirected to the portal's public address after OAuth consent.

**How it works:**

1. The portal proxies login/callback requests to vire-server and forwards its own Host header (e.g. `localhost:8880`, `vire.app`)
2. vire-server reads the **request's Host header** and uses it to build `redirect_uri`
3. The `redirect_uri` must point to the **portal's** public address, e.g.:
   - Dev: `http://localhost:8880/api/auth/callback/google`
   - Prod: `https://vire.app/api/auth/callback/google`
4. The portal has a dedicated proxy handler at `/api/auth/callback/google` that forwards the callback request back to vire-server
5. Google requires the `redirect_uri` in the token exchange (step 4 of callback) to exactly match the one sent during authorization — store it in the state store

**State store must include redirect_uri:**

```go
type OAuthState struct {
    CallbackURL string    // portal callback URL (e.g. http://localhost:8880/auth/callback)
    RedirectURI string    // redirect_uri sent to provider (e.g. http://localhost:8880/api/auth/callback/google)
    CreatedAt   time.Time
}
```

## Configuration Required

### vire-server TOML

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

### Environment variable overrides

| Variable | Config key |
|----------|-----------|
| `VIRE_AUTH_JWT_SECRET` | `auth.jwt_secret` |
| `VIRE_AUTH_TOKEN_EXPIRY` | `auth.token_expiry` |
| `VIRE_AUTH_GOOGLE_CLIENT_ID` | `auth.google.client_id` |
| `VIRE_AUTH_GOOGLE_CLIENT_SECRET` | `auth.google.client_secret` |
| `VIRE_AUTH_GITHUB_CLIENT_ID` | `auth.github.client_id` |
| `VIRE_AUTH_GITHUB_CLIENT_SECRET` | `auth.github.client_secret` |

### Provider setup

The authorized redirect URIs must be registered under the **portal's public domain**, not vire-server's internal address.

**Google:**
1. Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Create OAuth 2.0 Client ID (Web application)
3. Add authorized redirect URIs:
   - Dev: `http://localhost:8880/api/auth/callback/google`
   - Prod: `https://vire.app/api/auth/callback/google`
4. Copy Client ID and Client Secret to config

**GitHub:**
1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Create a new OAuth App
3. Set Authorization callback URL:
   - Dev: `http://localhost:8880/api/auth/callback/github`
   - Prod: `https://vire.app/api/auth/callback/github`
4. Copy Client ID and Client Secret to config

## State Store

The state parameter links a login request to its callback. It must be:

- Random: minimum 32 bytes, base64url encoded
- Single-use: deleted after successful exchange
- Time-limited: 10-minute TTL
- Maps to: the portal `callback` URL that initiated the flow, and the `redirect_uri` sent to the provider

Recommended implementation: in-memory map with mutex (matches existing `CodeStore` and `SessionStore` patterns in the portal's MCP OAuth implementation).

```go
type OAuthState struct {
    CallbackURL string    // where to redirect after JWT is minted
    RedirectURI string    // redirect_uri sent to provider (needed for token exchange)
    CreatedAt   time.Time
}

type StateStore struct {
    mu     sync.RWMutex
    states map[string]OAuthState // state -> OAuthState
    ttl    time.Duration         // 10 minutes
}
```

## User Mapping

When a user authenticates via Google or GitHub, vire-server must create or update the user record.

### Matching strategy

1. Look up user by email (case-insensitive)
2. If found: update `name`, `picture`/`avatar_url`, record provider
3. If not found: create new user with:
   - `username`: email local part (or GitHub `login`)
   - `email`: from provider
   - `role`: `"user"` (default)
   - `provider`: `"google"` or `"github"`
   - No password (OAuth-only users cannot use email/password login)

### Account linking

A user who first logs in with Google and later with GitHub (same email) should get the same account. The email is the primary key for matching. Store the provider used for each login in the JWT `provider` claim so the portal can display it.

## JWT Contract

The JWT returned to the portal must match this format exactly. The portal's `ValidateJWT` function expects these fields:

```go
type JWTClaims struct {
    Sub      string `json:"sub"`      // vire user ID (required)
    Email    string `json:"email"`    // user email (required)
    Name     string `json:"name"`     // display name
    Provider string `json:"provider"` // "google", "github", "email"
    Iss      string `json:"iss"`      // must be "vire-server"
    Iat      int64  `json:"iat"`      // issued at (unix seconds)
    Exp      int64  `json:"exp"`      // expiry (unix seconds, required)
}
```

- **Signing:** HMAC-SHA256 with shared `jwt_secret`
- **Header:** `{ "alg": "HS256", "typ": "JWT" }`
- **Expiry:** 24 hours from issue time

## Error Handling

On any failure during the OAuth flow, vire-server should redirect back to the portal rather than showing a raw error page:

```
302 to {callback}?error={error_code}
```

Error codes the portal handles:

| Code | Meaning |
|------|---------|
| `provider_not_configured` | OAuth client ID/secret not set for this provider |
| `invalid_state` | State parameter missing, expired, or already used |
| `access_denied` | User denied consent at the provider |
| `exchange_failed` | Code-to-token exchange with provider failed |
| `profile_failed` | Could not fetch user profile from provider |
| `user_creation_failed` | Could not create/update user in database |

## Testing

### Unit tests (vire-server)

1. **State generation and validation** -- generate state, store it, retrieve it, verify TTL expiry, verify single-use deletion
2. **redirect_uri from Host header** -- verify redirect_uri is built from request's Host, not server's own address
3. **Google token exchange** -- mock Google token endpoint, verify request format (especially redirect_uri match), handle success and error responses
4. **GitHub token exchange** -- mock GitHub token endpoint, verify Accept header, handle success and error
5. **GitHub email fallback** -- mock `/user/emails` endpoint for private email case
6. **User creation** -- verify new user created with correct fields from provider profile
7. **User matching** -- verify existing user found by email, fields updated
8. **JWT minting** -- verify JWT contains correct claims, signed with correct secret
9. **Redirect URL construction** -- verify `{callback}?token={jwt}` format
10. **Error redirects** -- verify error codes redirected correctly

### Integration tests (vire-server)

1. Full flow with mock provider: `GET /api/auth/login/google?callback=...` with `Host: test-portal:8880` -> verify redirect to mock Google with `redirect_uri=http://test-portal:8880/api/auth/callback/google` -> `GET /api/auth/callback/google?code=...&state=...` -> verify redirect to callback with token
2. Same for GitHub

### Manual end-to-end test

1. Configure real Google/GitHub credentials in vire-server
2. Start vire-server and portal
3. Click "Sign in with Google" on landing page
4. Complete Google login
5. Verify redirect back to portal dashboard with valid session

## Implementation Order

1. **State store** -- reusable for both providers; must store `redirect_uri` alongside `callback`
2. **Host-based redirect_uri construction** -- build `redirect_uri` from request Host header
3. **Google login + callback** -- `GET /api/auth/login/google`, `GET /api/auth/callback/google`
4. **Google user mapping** -- create/update user from Google profile
5. **Google tests** -- unit + integration with mock provider
6. **GitHub login + callback** -- same pattern, different URLs and scopes
7. **GitHub email fallback** -- handle private email via `/user/emails`
8. **GitHub tests** -- unit + integration
9. **End-to-end test** -- manual test with real credentials

## Files Expected to Change (vire-server)

| File | Change |
|------|--------|
| `auth_handler.go` (or equivalent) | Add `HandleGoogleLogin`, `HandleGoogleCallback`, `HandleGitHubLogin`, `HandleGitHubCallback` |
| `state_store.go` (new) | OAuth state parameter storage with TTL (includes `redirect_uri`) |
| `google_oauth.go` (new) | Google token exchange and userinfo fetch |
| `github_oauth.go` (new) | GitHub token exchange, user fetch, email fallback |
| `user_store.go` / `user_handler.go` | Add find-by-email, create-from-oauth methods |
| `config.go` | Add `[auth.google]` and `[auth.github]` config sections |
| `routes.go` | Register new OAuth endpoints |
| `*_test.go` | Unit and integration tests for all of the above |
