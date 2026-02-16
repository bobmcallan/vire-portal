# Requirements: MCP OAuth Phases 3–6 — Complete OAuth 2.1 Authorization Server

**Date:** 2026-02-16
**Requested:** Implement remaining phases from `docs/authentication/mcp-oauth-implementation-steps.md` — DCR, authorize, token endpoints, and Bearer token support on /mcp.

## Scope

**In scope (Phases 3–6):**
- Phase 3: `POST /register` — Dynamic Client Registration (RFC 7591)
- Phase 4: `GET /authorize` — Authorization endpoint with two-hop flow into existing login
- Phase 5: `POST /token` — Token exchange with PKCE verification
- Phase 6: Bearer token validation on `POST /mcp`
- In-memory stores for clients, sessions, auth codes, refresh tokens
- Modify existing OAuth callback to support MCP flow branch
- Landing page modification to pass MCP session through login

**Out of scope:**
- Phases 7–9 (testing with Claude CLI/Desktop, tunneling) — manual verification
- Persistent storage (Postgres) — in-memory is fine for local dev
- Token revocation endpoint

## Approach

All four phases are tightly coupled — they form a single OAuth flow. Implement them together as one unit in the `internal/auth/` package.

### Architecture

The `internal/auth/` package gets a central `OAuthServer` struct that holds all state:

```go
type OAuthServer struct {
    baseURL    string
    jwtSecret  []byte
    clients    *ClientStore    // in-memory client registrations
    sessions   *SessionStore   // pending auth sessions (TTL 10 min)
    codes      *CodeStore      // issued auth codes (TTL 5 min)
    tokens     *TokenStore     // refresh tokens
    logger     *common.Logger
}
```

This replaces the standalone `DiscoveryHandler` — the `OAuthServer` exposes all handlers:
- `HandleAuthorizationServer` (existing, moved from DiscoveryHandler)
- `HandleProtectedResource` (existing, moved from DiscoveryHandler)
- `HandleRegister` (new — Phase 3)
- `HandleAuthorize` (new — Phase 4)
- `HandleToken` (new — Phase 5)

The MCP handler change (Phase 6) is separate — it updates `withUserContext` to check Bearer token first, cookie as fallback.

### Flow: Claude CLI → Vire OAuth

1. Claude fetches `/.well-known/oauth-authorization-server` → gets endpoints
2. Claude calls `POST /register` → gets `client_id` + `client_secret`
3. Claude opens browser to `GET /authorize?client_id=X&redirect_uri=http://localhost:PORT/callback&code_challenge=Y&state=Z&response_type=code`
4. Portal stores MCP session, redirects to `/?mcp_session=<id>` (landing page with login)
5. User logs in (email/password or Google/GitHub)
6. Login callback receives JWT from vire-server, detects `mcp_session` cookie
7. Instead of redirecting to /dashboard, generates auth code and redirects to Claude's callback: `http://localhost:PORT/callback?code=<auth-code>&state=Z`
8. Claude calls `POST /token` with `code`, `code_verifier`, `client_id` → gets `access_token` (JWT) + `refresh_token`
9. Claude calls `POST /mcp` with `Authorization: Bearer <access_token>` → tools work

### Key design decisions

- **MCP session tracking**: Use a cookie `mcp_session_id` set during `/authorize` and read during `/auth/callback`. This survives the Google/GitHub OAuth redirect chain.
- **Auth codes**: Short-lived (5 min), single-use, stored in-memory with PKCE challenge.
- **Access tokens**: JWTs signed with `VIRE_AUTH_JWT_SECRET` (same as session JWTs), 1 hour expiry, contain `sub`, `scope`, `client_id`, `iss`.
- **Refresh tokens**: Opaque UUIDs stored in-memory, 7 day expiry, rotated on use.
- **Lenient DCR**: Unknown `client_id` on `/authorize` is auto-registered (for Claude Desktop).
- **Bearer + cookie fallback**: MCP handler checks `Authorization: Bearer` first, then `vire_session` cookie. Both paths extract user ID into the same `UserContext`.

## Files Expected to Change

**New:**
- `internal/auth/store.go` — ClientStore, CodeStore, TokenStore (in-memory, mutex-protected)
- `internal/auth/session.go` — SessionStore for pending auth sessions (TTL-based)
- `internal/auth/dcr.go` — HandleRegister (POST /register)
- `internal/auth/authorize.go` — HandleAuthorize (GET /authorize)
- `internal/auth/token.go` — HandleToken (POST /token), JWT minting
- `internal/auth/pkce.go` — PKCE S256 verification
- `internal/auth/server.go` — OAuthServer struct tying everything together
- Test files for each of the above

**Modified:**
- `internal/auth/discovery.go` — Move handlers to OAuthServer methods (or keep delegating)
- `internal/app/app.go` — Replace DiscoveryHandler with OAuthServer, wire up new handlers
- `internal/server/routes.go` — Add /register, /authorize, /token routes
- `internal/handlers/auth.go` — Branch HandleOAuthCallback + HandleLogin for MCP session flow
- `internal/mcp/handler.go` — Add Bearer token extraction to withUserContext
- `pages/landing.html` — Pass mcp_session_id to login forms/OAuth links
