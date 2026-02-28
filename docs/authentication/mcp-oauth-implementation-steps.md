# MCP OAuth Implementation Steps — Local Development

> Working plan for getting MCP OAuth running locally with Claude CLI and Claude Desktop
> Based on: `docs/authentication/mcp-ouath.md`

---

## Current State (after Phase 6)

**What exists today:**
- Portal (`localhost:8500`) with email/password login and Google/GitHub OAuth redirects via vire-server
- vire-server (`localhost:4242`) handles actual auth and issues JWTs
- MCP endpoint at `POST /mcp` that reads Bearer token or `vire_session` cookie for user identity
- JWT validation (HMAC-SHA256, optional signature check when `VIRE_AUTH_JWT_SECRET` is set)
- VireClient proxies auth and tool calls to vire-server
- Complete MCP OAuth 2.1 Authorization Server:
  - `POST /register` — Dynamic Client Registration (RFC 7591)
  - `GET /authorize` — Authorization endpoint with PKCE (S256), auto-registers unknown clients
  - `POST /token` — Token exchange (authorization_code + refresh_token grants)
  - Bearer token support on `/mcp` (Claude CLI/Desktop), cookie fallback (web dashboard)
  - In-memory stores for clients, sessions, auth codes, and refresh tokens
  - PKCE verification with constant-time comparison
- OAuth discovery endpoints (`.well-known/oauth-authorization-server` and `.well-known/oauth-protected-resource`)
- Integration tests, stress tests, and manual verification scripts

---

## Phase 1: Get Current Dev Login Working End-to-End [COMPLETE]

Goal: Verify the existing email/password and OAuth login flows work locally with vire-server.

> **Status:** Completed. All auth flows verified with tests and manual validation.

### 1.1 Confirm vire-server is running and healthy

- [x] Start vire-server on `localhost:4242`
- [x] Verify `GET http://localhost:4242/api/health` returns OK
- [x] Verify `POST http://localhost:4242/api/auth/login` accepts email/password and returns a JWT
- [x] Check that vire-server has test/dev user credentials seeded

### 1.2 Confirm portal login flow

- [x] Start portal on `localhost:8500`
- [x] Hit `http://localhost:8500` — should render login page
- [x] Submit email/password via the form — should POST to `/api/auth/login`, which forwards to vire-server
- [x] Verify `vire_session` cookie is set with a valid JWT
- [x] Verify redirect to `/dashboard` works and dashboard loads with user context
- [x] Check `/profile` page shows JWT debug info (dev mode)

### 1.3 Confirm Google/GitHub OAuth redirects

- [x] Click "Login with Google" — should redirect to vire-server's Google OAuth endpoint
- [x] vire-server needs Google OAuth credentials configured (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`)
- [x] For **local dev without real Google credentials**: verify the redirect chain is correct even if Google rejects the client — the plumbing is what matters at this stage
- [x] Same for GitHub

### 1.4 Fix any issues found

- [x] Config: Added missing `[auth]` section to `config/vire-portal.toml` (was only in `.toml.example`)
- [x] If `VIRE_AUTH_JWT_SECRET` is empty, JWT signature verification is skipped — acceptable for dev mode
- [x] Ensure vire-server's callback redirects back to `http://localhost:8500/auth/callback?token=<jwt>`
- [x] HandleLogout cookie now sets `SameSite=Lax` for consistency with login/callback paths

### 1.5 Tests added

- `internal/handlers/auth_integration_test.go` — Full login round-trip against mock vire-server, invalid JSON, empty token, Google/GitHub OAuth redirect chains
- `internal/handlers/auth_stress_test.go` — Security stress tests (alg:none attack, timing, hostile inputs, cookie attributes, concurrent access)
- `internal/mcp/handler_test.go` — `withUserContext` (valid/no/invalid cookie), `extractJWTSub` (valid, invalid base64, invalid JSON, missing sub, empty, no dots)
- `internal/mcp/handler_stress_test.go` — Hostile cookie values, hostile sub claims, concurrent access, binary garbage, type confusion

### 1.6 Verification script

Run `./scripts/verify-auth.sh` against a running server to manually validate all auth endpoints:
- Portal health, vire-server health (proxied)
- Login flow (POST /api/auth/login), cookie capture
- OAuth redirect (GET /api/auth/login/google), redirect URL validation
- OAuth callback (GET /auth/callback?token=...), cookie set
- MCP endpoint (POST /mcp with JSON-RPC initialize)

---

## Phase 2: Implement MCP OAuth Discovery Endpoints [COMPLETE]

Goal: Claude CLI and Desktop need to discover Vire's OAuth capabilities.

> **Status:** Completed. Discovery endpoints implemented, tested (unit + stress), and verified on running server.

### 2.1 Add `/.well-known/oauth-authorization-server`

**File:** `internal/auth/discovery.go` (new package)

Return JSON metadata:
```json
{
  "issuer": "http://localhost:8500",
  "authorization_endpoint": "http://localhost:8500/authorize",
  "token_endpoint": "http://localhost:8500/token",
  "registration_endpoint": "http://localhost:8500/register",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "code_challenge_methods_supported": ["S256"],
  "token_endpoint_auth_methods_supported": ["client_secret_post", "none"],
  "scopes_supported": ["openid", "portfolio:read", "portfolio:write", "tools:invoke"]
}
```

For local dev, `issuer` is `http://localhost:8500`. In production it becomes `https://portal.vire.dev`.

### 2.2 Add `/.well-known/oauth-protected-resource`

**File:** Same `internal/auth/discovery.go`

Return:
```json
{
  "resource": "http://localhost:8500",
  "authorization_servers": ["http://localhost:8500"],
  "scopes_supported": ["portfolio:read", "portfolio:write", "tools:invoke"]
}
```

### 2.3 Register routes

**File:** `internal/server/routes.go`

Add the two `.well-known` routes. These are unauthenticated GET endpoints.

---

## Phase 3: Implement Dynamic Client Registration (DCR) [COMPLETE]

Goal: Claude CLI calls `POST /register` to register itself as an OAuth client before starting the flow.

> **Status:** Completed. DCR endpoint implemented with UUID client IDs, random secrets, and lenient auto-registration.

### 3.1 Add client storage

**File:** `internal/auth/store.go` (new)

Implemented as L1 in-memory cache with write-through to vire-server backend:
```go
type ClientStore struct {
    mu      sync.RWMutex
    clients map[string]*OAuthClient
    backend *OAuthBackend  // write-through on Put, read-through on Get miss
}

type OAuthClient struct {
    ClientID     string
    ClientSecret string
    ClientName   string
    RedirectURIs []string
    CreatedAt    time.Time
}
```

The backend is vire-server's internal OAuth API (`/api/internal/oauth/`). In-memory stores are authoritative during process lifetime; backend is source-of-truth across restarts.

### 3.2 Add `/register` handler

**File:** `internal/auth/dcr.go` (new)

Accept RFC 7591 registration request:
```json
{
  "client_name": "Claude Code",
  "redirect_uris": ["http://localhost:PORT/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "client_secret_post"
}
```

Return:
```json
{
  "client_id": "<generated-uuid>",
  "client_secret": "<generated-secret>",
  "client_name": "Claude Code",
  "redirect_uris": ["http://localhost:PORT/callback"],
  ...
}
```

### 3.3 Handle Claude Desktop's pre-registered client_id

Claude Desktop may send a `client_id` it already has. If the ID isn't in the store, either:
- Auto-register it (lenient mode for dev)
- Reject it (strict mode)

For local dev, use lenient mode.

---

## Phase 4: Implement the Authorization Endpoint [COMPLETE]

Goal: `GET /authorize` starts the MCP OAuth flow, then chains into the existing Google/GitHub login.

> **Status:** Completed. Authorization endpoint with PKCE, session tracking via `mcp_session_id` cookie, lenient auto-registration, and MCP flow branch in login/callback handlers.

### 4.1 Add `/authorize` handler

**File:** `internal/auth/authorize.go` (new)

This is the core of the two-hop flow:

1. Receive request from Claude with: `client_id`, `redirect_uri`, `response_type=code`, `state`, `code_challenge`, `code_challenge_method=S256`, `scope`
2. Validate `client_id` against the client store
3. Validate `redirect_uri` matches the registered value
4. Store the MCP session state (all the above params) keyed by a session ID
5. Present a login page or redirect to identity provider

### 4.2 Add MCP session state storage

**File:** `internal/auth/session.go` (new)

In-memory store for pending authorization sessions:
```go
type AuthSession struct {
    SessionID     string
    ClientID      string
    RedirectURI   string
    State         string
    CodeChallenge string
    CodeMethod    string
    Scope         string
    CreatedAt     time.Time
    UserID        string  // filled after login
}
```

Include TTL-based expiry (e.g., 10 minutes).

### 4.3 Chain into existing login

After storing the MCP session, redirect to a login page that:
- Shows email/password form + Google/GitHub buttons (reuse existing login UI)
- Passes the MCP session ID through the flow (query param or cookie)

When the user completes login (via any method), the callback must:
1. Resolve the MCP session by ID
2. Set `UserID` on the session from the JWT claims
3. Generate a Vire authorization code
4. Redirect back to Claude's `redirect_uri` with `code=<vire-auth-code>&state=<original-state>`

### 4.4 Modify existing OAuth callback

**File:** `internal/handlers/auth.go` — `HandleOAuthCallback`

Add a branch: if an MCP session ID is present in the request, instead of setting a cookie and redirecting to `/dashboard`, generate an auth code and redirect to Claude's callback.

---

## Phase 5: Implement the Token Endpoint [COMPLETE]

Goal: Claude exchanges the authorization code for an access token.

> **Status:** Completed. Token endpoint with authorization_code and refresh_token grants, PKCE S256 verification (constant-time), JWT minting (HMAC-SHA256), refresh token rotation.

### 5.1 Add `/token` handler

**File:** `internal/auth/token.go` (new)

Handle `grant_type=authorization_code`:
1. Receive `code`, `client_id`, `client_secret`, `redirect_uri`, `code_verifier`
2. Look up the auth code → find associated session
3. Verify PKCE: `SHA256(code_verifier) == code_challenge`
4. Verify `client_id` and `redirect_uri` match the session
5. Generate access token (JWT) and refresh token
6. Return:
```json
{
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "<opaque-token>",
  "scope": "portfolio:read tools:invoke"
}
```

Handle `grant_type=refresh_token`:
1. Validate the refresh token
2. Issue a new access token (and optionally rotate the refresh token)

### 5.2 PKCE implementation

**File:** `internal/auth/pkce.go` (new)

```go
func VerifyPKCE(verifier, challenge string) bool {
    hash := sha256.Sum256([]byte(verifier))
    computed := base64.RawURLEncoding.EncodeToString(hash[:])
    return computed == challenge
}
```

### 5.3 Auth code storage

**File:** `internal/auth/store.go` (extend)

```go
type AuthCode struct {
    Code          string
    ClientID      string
    UserID        string
    RedirectURI   string
    CodeChallenge string
    Scope         string
    ExpiresAt     time.Time  // short-lived, ~5 min
    Used          bool
}
```

---

## Phase 6: Update `/mcp` to Accept Bearer Tokens [COMPLETE]

Goal: Claude sends `Authorization: Bearer <token>` on every MCP request. The handler must validate it.

> **Status:** Completed. Bearer token validation with HMAC-SHA256 signature verification + expiry check. Falls back to cookie for web dashboard. Legacy fallback (no validation) only when JWT secret is unconfigured.

### 6.1 Add Bearer token middleware

**File:** `internal/mcp/middleware.go` (new or extend existing)

```
1. Extract `Authorization: Bearer <token>` header
2. Validate the JWT (signature, expiry, issuer)
3. Extract user ID from claims
4. Inject into request context (replace current cookie-based approach)
5. If no Bearer token, fall back to cookie (backwards compat for web UI)
```

### 6.2 Update MCP handler

**File:** `internal/mcp/handler.go`

Update `extractUserContext()` to check Bearer token first, then cookie as fallback. This keeps the web dashboard's MCP calls working while also supporting Claude's Bearer token flow.

---

## Phase 7: Test with Claude CLI (Local)

### 7.1 Test with MCP Inspector first

```bash
npx @modelcontextprotocol/inspector
```
- Transport: Streamable HTTP
- URL: `http://localhost:8500/mcp`
- Open Auth Settings → Quick OAuth Flow
- Complete login, verify tools appear

### 7.2 Add Vire as MCP server in Claude CLI

```bash
claude mcp add --transport http vire http://localhost:8500/mcp
```

What happens:
1. Claude fetches `http://localhost:8500/.well-known/oauth-authorization-server`
2. Claude calls `POST http://localhost:8500/register` (DCR)
3. Claude opens browser to `http://localhost:8500/authorize?client_id=...&redirect_uri=http://localhost:PORT/callback&code_challenge=...&state=...`
4. User logs in via Vire Portal
5. Portal redirects to Claude's local callback with auth code
6. Claude exchanges code for token via `POST http://localhost:8500/token`
7. Claude stores token in `~/.claude.json`

### 7.3 Verify

```bash
claude mcp list                          # should show vire as connected
claude "Show me my SMSF portfolio"       # should invoke Vire tools
```

---

## Phase 8: Test with Claude Desktop (Local)

### 8.1 Add as custom connector

Claude Desktop → Settings → Connectors → Add custom connector:
- URL: `http://localhost:8500/mcp`
- Complete OAuth flow in browser popup

### 8.2 Known issues to handle

- Claude Desktop may use its own `client_id` without calling `/register` — the lenient DCR mode from Phase 3.3 handles this
- Callback URL will be `https://claude.ai/api/mcp/auth_callback` — this is a **problem for local dev** because Claude Desktop's OAuth popup redirects to claude.ai, not localhost
- **Workaround:** For local testing, use Claude Code CLI (which uses localhost callbacks). Claude Desktop connectors targeting localhost may require tunneling (see Phase 9).

---

## Phase 9: Local Dev Tunneling (Optional, for Claude Desktop)

If you need Claude Desktop (not CLI) to connect to local services:

### 9.1 Use a tunnel

```bash
# Option A: ngrok
ngrok http 8500

# Option B: Cloudflare Tunnel
cloudflared tunnel --url http://localhost:8500

# Option C: localtunnel
npx localtunnel --port 8500
```

This gives you a public URL like `https://abc123.ngrok.io` that Claude Desktop can reach.

### 9.2 Update OAuth metadata

The `.well-known` endpoints must return the tunnel URL as `issuer` and in all endpoint URLs. Either:
- Set `VIRE_PORTAL_URL` env var to the tunnel URL and use it in discovery responses
- Or make the discovery handler read the `Host` header and construct URLs dynamically

### 9.3 Update IdP callback URLs

If using Google/GitHub OAuth, update the allowed callback URLs in their developer consoles to include the tunnel URL.

---

## Implementation Order Summary

| Phase | What | Depends On | Enables | Status |
|-------|------|-----------|---------|--------|
| 1 | Verify existing dev login | vire-server running | Baseline | **Done** |
| 2 | Discovery endpoints | Phase 1 | Claude can find OAuth config | **Done** |
| 3 | DCR endpoint | Phase 2 | Claude can register as client | **Done** |
| 4 | `/authorize` endpoint | Phase 3 | OAuth flow start | **Done** |
| 5 | `/token` endpoint | Phase 4 | Token exchange | **Done** |
| 6 | Bearer token on `/mcp` | Phase 5 | Claude can call tools | **Done** |
| 7 | Test with Claude CLI | Phase 6 | End-to-end validation | |
| 8 | Test with Claude Desktop | Phase 7 | Desktop validation | |
| 9 | Tunneling (optional) | Phase 8 | Desktop with local services | |

---

## Files Created in Phase 1

```
internal/handlers/auth_integration_test.go  — Login round-trip, OAuth redirect chain tests
internal/mcp/handler_test.go                — MCP user context extraction tests
internal/mcp/handler_stress_test.go         — Hostile input and concurrency stress tests
scripts/verify-auth.sh                      — Manual auth endpoint validation script
```

## Files Modified in Phase 1

```
config/vire-portal.toml         — Added missing [auth] section
internal/handlers/auth.go       — HandleLogout: added SameSite=Lax on cookie clear
```

## Files Created in Phase 2

```
internal/auth/discovery.go              — .well-known OAuth discovery endpoints (authorization server + protected resource)
internal/auth/discovery_test.go         — Unit tests for discovery handlers and BaseURL config
internal/auth/discovery_stress_test.go  — Stress tests: method enforcement, URL injection, concurrency, RFC compliance
```

## Files Modified in Phase 2

```
internal/config/config.go       — Added PortalURL to AuthConfig, BaseURL() method, VIRE_PORTAL_URL env override
internal/config/defaults.go     — Added PortalURL default (empty)
internal/config/config_test.go  — Added BaseURL and PortalURL env override tests
internal/app/app.go             — Added DiscoveryHandler to App struct, initialized in initHandlers()
internal/server/routes.go       — Registered /.well-known/oauth-authorization-server and /.well-known/oauth-protected-resource
config/vire-portal.toml         — Added portal_url setting under [auth]
config/vire-portal.toml.example — Added portal_url setting under [auth]
```

## Files Created in Phases 3–6

```
internal/auth/server.go             — OAuthServer struct (central state: stores, JWT minting, auth completion, backend init)
internal/auth/backend.go            — OAuthBackend (HTTP client for vire-server internal OAuth API, write-through/read-through persistence)
internal/auth/store.go              — ClientStore, CodeStore, TokenStore (L1 in-memory cache with write-through to backend)
internal/auth/session.go            — SessionStore for pending MCP auth sessions (L1 in-memory cache with read-through to backend, TTL 10 min)
internal/auth/pkce.go               — PKCE S256 verification (constant-time compare)
internal/auth/dcr.go                — HandleRegister (POST /register, RFC 7591 DCR)
internal/auth/authorize.go          — HandleAuthorize (GET /authorize, PKCE + session + redirect)
internal/auth/token.go              — HandleToken (POST /token, auth_code + refresh_token grants)
internal/auth/server_test.go        — OAuthServer unit tests
internal/auth/store_test.go         — Store unit tests
internal/auth/session_test.go       — Session store unit tests
internal/auth/pkce_test.go          — PKCE unit tests
internal/auth/dcr_test.go           — DCR endpoint unit tests
internal/auth/authorize_test.go     — Authorize endpoint unit tests
internal/auth/token_test.go         — Token endpoint unit tests
internal/auth/store_stress_test.go  — Store stress tests
internal/auth/pkce_stress_test.go   — PKCE stress tests
internal/auth/dcr_stress_test.go    — DCR stress tests
internal/auth/authorize_stress_test.go — Authorize stress tests
internal/auth/token_stress_test.go  — Token stress tests
```

## Files Modified in Phases 3–6

```
internal/auth/discovery.go     — Added OAuthServer method wrappers for discovery handlers, deprecated DiscoveryHandler
internal/app/app.go            — Replaced DiscoveryHandler with OAuthServer, wired SetOAuthServer on AuthHandler
internal/server/routes.go      — Added POST /register, GET /authorize, POST /token routes
internal/handlers/auth.go      — Added OAuthCompleter interface, tryCompleteMCPSession, MCP flow branch in HandleLogin and HandleOAuthCallback
internal/mcp/handler.go        — Added jwtSecret field, validateJWT function, Bearer token signature/expiry verification, cookie fallback with validation
```
