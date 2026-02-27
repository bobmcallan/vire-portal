# MCP OAuth Session Persistence (fb_c4a661a8, fb_00e43378)

## Problem
Portal OAuth state (sessions, clients, codes, tokens) is stored in-memory only. When portal restarts on Fly.io, all active MCP client sessions are lost. Clients like ChatGPT must re-authorize.

## Solution
Write-through to vire-server's new internal OAuth REST API. In-memory stores remain as L1 cache; backend is L2/source-of-truth. On read miss, fall back to backend.

## Backend API (already implemented in vire-server)
Base: `{VIRE_API_URL}/api/internal/oauth/`

### Sessions
- `POST /sessions` — create (body: full session JSON)
- `GET /sessions/{id}` — get by session_id
- `GET /sessions?client_id=X` — get by client_id
- `PATCH /sessions/{id}` — update user_id (body: `{"user_id":"..."}`)
- `DELETE /sessions/{id}` — delete

### Clients
- `POST /clients` — create (body: full client JSON)
- `GET /clients/{id}` — get by client_id

### Codes
- `POST /codes` — create (body: full code JSON)
- `GET /codes/{code}` — get by code
- `PATCH /codes/{code}/used` — mark used

### Tokens
- `POST /tokens` — save (body includes plaintext `token`, server hashes)
- `POST /tokens/lookup` — lookup by plaintext token (body: `{"token":"..."}`)
- `POST /tokens/revoke` — revoke (body: `{"token":"..."}`)

## Files to Create

### `internal/auth/backend.go`
HTTP client for vire-server's internal OAuth API. Non-blocking — errors logged, not fatal.

```go
type OAuthBackend struct {
    apiURL string
    client *http.Client
    logger *common.Logger
}

func NewOAuthBackend(apiURL string, logger *common.Logger) *OAuthBackend
```

Methods (all return error, callers log and continue on failure):
- `SaveSession(sess *AuthSession) error`
- `GetSession(sessionID string) (*AuthSession, error)`
- `GetSessionByClientID(clientID string) (*AuthSession, error)`
- `UpdateSessionUserID(sessionID, userID string) error`
- `DeleteSession(sessionID string) error`
- `SaveClient(client *OAuthClient) error`
- `GetClient(clientID string) (*OAuthClient, error)`
- `SaveCode(code *AuthCode) error`
- `GetCode(code string) (*AuthCode, error)`
- `MarkCodeUsed(code string) error`
- `SaveToken(token *RefreshToken) error`
- `LookupToken(plaintext string) (*RefreshToken, error)`
- `RevokeToken(plaintext string) error`

HTTP timeout: 5s. Use `http.Client` with timeout.

### `internal/auth/backend_test.go`
Unit tests using `httptest.NewServer` to mock vire-server responses. Test each method.

## Files to Modify

### `internal/auth/session.go`
Add `backend *OAuthBackend` field to `SessionStore`. Modify:
- `Put()` — after local store, call `backend.SaveSession()` (log error, don't fail)
- `Get()` — if local miss, try `backend.GetSession()`, cache locally if found
- `GetByClientID()` — if local miss, try `backend.GetSessionByClientID()`
- `Delete()` — after local delete, call `backend.DeleteSession()`
- Add `SetBackend(b *OAuthBackend)` method

### `internal/auth/store.go`
Add `backend *OAuthBackend` field to `ClientStore`, `CodeStore`, `TokenStore`. Modify:

**ClientStore:**
- `Put()` — write-through to `backend.SaveClient()`
- `Get()` — read-through from `backend.GetClient()` on miss

**CodeStore:**
- `Put()` — write-through to `backend.SaveCode()`
- `Get()` — read-through from `backend.GetCode()` on miss
- `MarkUsed()` — write-through to `backend.MarkCodeUsed()`

**TokenStore:**
- `Put()` — write-through to `backend.SaveToken()`
- `Get()` — read-through from `backend.LookupToken()` on miss
- `Delete()` — write-through to `backend.RevokeToken()`

Each store gets `SetBackend(b *OAuthBackend)` method.

### `internal/auth/server.go`
- `NewOAuthServer()` accepts `apiURL string` parameter
- Creates `OAuthBackend` and passes to each store via `SetBackend()`
- If `apiURL` is empty, no backend (in-memory only — preserves existing behavior for tests)

### `internal/auth/dcr.go`
No changes needed — `HandleRegister` calls `s.clients.Put()` which already write-through.

### `internal/app/app.go`
- Pass `a.Config.API.URL` to `NewOAuthServer()`:
  ```go
  a.OAuthServer = auth.NewOAuthServer(a.Config.BaseURL(), a.Config.API.URL, jwtSecret, a.Logger)
  ```

## Design Rules
- Backend calls are fire-and-forget on writes — log errors, never fail the OAuth flow
- Backend calls on reads are best-effort — if backend unreachable, return "not found"
- No new config keys — reuse existing `VIRE_API_URL` (already configured)
- In-memory stores remain authoritative during the lifetime of the process
- Backend is source-of-truth across restarts
- HTTP timeout 5s for backend calls
- No UI changes — skip Phase 3/4 (UI tests)

## Patterns
- Error logging: use `s.logger.Warn().Err(err).Str("method","...").Msg("...")` pattern
- HTTP client: `http.Client{Timeout: 5 * time.Second}`
- JSON encoding: `json.NewEncoder/Decoder`
- URL construction: `fmt.Sprintf("%s/api/internal/oauth/sessions/%s", apiURL, id)`

## Existing Tests
There are extensive existing tests in `internal/auth/`:
- `session_test.go`, `store_test.go`, `authorize_test.go`, `token_test.go`, `dcr_test.go`
- `*_stress_test.go` files for concurrent access

Existing tests must continue to pass (they use in-memory stores without backend).
New tests in `backend_test.go` test the backend HTTP client with httptest mocks.
