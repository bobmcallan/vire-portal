# Summary: MCP OAuth Phases 3-6 — Complete OAuth 2.1 Authorization Server

**Date:** 2026-02-16
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `internal/auth/server.go` | **New**: OAuthServer struct — central state holder with JWT minting, auth completion, UUID/hex generation |
| `internal/auth/store.go` | **New**: ClientStore, CodeStore, TokenStore — in-memory, mutex-protected, with TTL-based expiry and cleanup |
| `internal/auth/session.go` | **New**: SessionStore for pending MCP auth sessions (10 min TTL) |
| `internal/auth/pkce.go` | **New**: PKCE S256 verification (constant-time compare) and code challenge generation |
| `internal/auth/dcr.go` | **New**: HandleRegister (POST /register) — RFC 7591 DCR with UUID client IDs and random secrets |
| `internal/auth/authorize.go` | **New**: HandleAuthorize (GET /authorize) — PKCE validation, session creation, mcp_session_id cookie, lenient auto-registration |
| `internal/auth/token.go` | **New**: HandleToken (POST /token) — authorization_code and refresh_token grants, PKCE verification, JWT minting, refresh token rotation |
| `internal/auth/discovery.go` | Added OAuthServer method wrappers for discovery handlers, deprecated standalone DiscoveryHandler |
| `internal/app/app.go` | Replaced DiscoveryHandler with OAuthServer, wired SetOAuthServer on AuthHandler |
| `internal/server/routes.go` | Added POST /register, GET /authorize, POST /token routes; updated discovery routes to use OAuthServer |
| `internal/handlers/auth.go` | Added OAuthCompleter interface, SetOAuthServer method, tryCompleteMCPSession helper, MCP flow branch in HandleLogin and HandleOAuthCallback |
| `internal/mcp/handler.go` | Added jwtSecret field, validateJWT function with HMAC-SHA256 signature + expiry verification, Bearer token priority with cookie fallback |
| `.claude/skills/develop/SKILL.md` | Updated routes table with new OAuth endpoints and auth descriptions |

## Tests

**New test files:**
- `internal/auth/server_test.go` — OAuthServer unit tests (mint, complete authorization)
- `internal/auth/store_test.go` — ClientStore, CodeStore, TokenStore unit tests
- `internal/auth/session_test.go` — SessionStore unit tests (put, get, TTL, cleanup)
- `internal/auth/pkce_test.go` — PKCE verification unit tests
- `internal/auth/dcr_test.go` — DCR endpoint unit tests
- `internal/auth/authorize_test.go` — Authorize endpoint unit tests
- `internal/auth/token_test.go` — Token endpoint unit tests
- `internal/auth/store_stress_test.go` — Store stress tests (concurrency, hostile inputs)
- `internal/auth/pkce_stress_test.go` — PKCE stress tests
- `internal/auth/dcr_stress_test.go` — DCR stress tests
- `internal/auth/authorize_stress_test.go` — Authorize stress tests
- `internal/auth/token_stress_test.go` — Token stress tests

**Test results:** All tests pass (`go test ./...`), `go vet ./...` clean.

## Documentation Updated

- `docs/authentication/mcp-oauth-implementation-steps.md` — Phases 3-6 marked complete, file lists added
- `.claude/skills/develop/SKILL.md` — Routes table updated with new OAuth endpoints

## Devils-Advocate Findings

- **CRITICAL (fixed):** Bearer token in `withUserContext` used `extractJWTSub` which does NO signature or expiry verification — anyone could forge a Bearer token with any `sub` claim. Fixed by adding `validateJWT` with HMAC-SHA256 signature check + expiry check. Legacy fallback (no validation) only applies to cookie path when JWT secret is unconfigured.
- **PKCE uses constant-time compare:** `subtle.ConstantTimeCompare` prevents timing attacks on code challenge verification.
- **Auth codes are single-use:** `MarkUsed` flag prevents replay attacks.
- **Refresh tokens are rotated:** Old token is deleted on use, new one issued.
- **Session TTL:** MCP auth sessions expire after 10 minutes.

## Notes

- All stores are in-memory — suitable for local dev, will need persistent storage for production.
- The OAuthServer replaces DiscoveryHandler (which is deprecated but still functional for backwards compat).
- Bearer token takes priority over cookie when both are present on `/mcp`.
- Lenient DCR mode auto-registers unknown client_ids on `/authorize` (for Claude Desktop compatibility).
- The `mcp_session_id` cookie survives Google/GitHub OAuth redirect chains, enabling the two-hop flow.
- No Cleanup goroutines are started automatically — expired entries are filtered on read. Manual `Cleanup()` methods exist for future use.
- Ready for Phase 7 testing with Claude CLI: `claude mcp add --transport http vire http://localhost:8500/mcp`
