# Summary: MCP OAuth Session Persistence (fb_c4a661a8, fb_00e43378)

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/auth/backend.go` | NEW — HTTP client for vire-server internal OAuth API (11 methods) |
| `internal/auth/backend_test.go` | NEW — Comprehensive test suite with httptest mocks |
| `internal/auth/session.go` | Write-through/read-through persistence on SessionStore |
| `internal/auth/store.go` | Write-through/read-through on ClientStore, CodeStore, TokenStore; atomic ConsumeCode() |
| `internal/auth/server.go` | NewOAuthServer() accepts apiURL, creates OAuthBackend |
| `internal/auth/token.go` | Use ConsumeCode() instead of Get()+MarkUsed() (TOCTOU fix) |
| `internal/app/app.go` | Pass a.Config.API.URL to NewOAuthServer() |
| `internal/server/routes.go` | Block /api/internal/ in API proxy (security fix) |
| `docs/authentication/auth-status.md` | Updated storage architecture description |
| `docs/authentication/mcp-oauth-implementation-steps.md` | Added backend.go to files list |
| 4 test files | Updated NewOAuthServer() call signatures |

## Tests
- All unit tests pass (go test, excluding UI browser tests)
- Race detection passes (go test -race ./internal/auth)
- go vet clean
- go build clean
- Existing tests backward compatible (apiURL="" = in-memory only)

## Architecture
- L1 (in-memory) + L2 (vire-server SurrealDB) caching pattern
- Write-through: every Put/Delete persists to backend (fire-and-forget)
- Read-through: every Get falls back to backend on local miss
- Zero new config keys — reuses existing VIRE_API_URL
- Zero UI changes

## Devils-Advocate
- 1 CRITICAL: /api/internal/ proxy exposure — FIXED (routes.go block)
- 2 MEDIUM: token plaintext over HTTP (Docker-internal only), multi-instance race (acceptable for current scale)
- 3 LOW: PKCE safe, timing attacks impractical, error leakage patterns correct
- TOCTOU race in ConsumeCode — FIXED (atomic check-and-mark under write lock)

## Notes
- MCP clients (ChatGPT, etc.) will survive portal restarts on Fly.io
- Backend calls are non-fatal — OAuth flow never breaks due to persistence layer
- Put()/Delete() hold mutex during 5s backend call — acceptable for low-frequency OAuth flows
