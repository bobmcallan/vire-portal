# Requirements: Remove BadgerDB, use vire-server API for user data

**Date:** 2026-02-15
**Requested:** Progress migration phases 2-4 from docs/migration-remove-badger.md. vire-server now has user CRUD, auth/login, and user context middleware (Option B). Remove BadgerDB from portal and replace with API client calls.

## Scope
- Create HTTP API client for vire-server user/auth endpoints
- Replace all BadgerDB user operations with API client calls
- Remove BadgerDB storage layer, user model, importer, storage interfaces
- Remove badgerhold/bcrypt dependencies
- Remove storage/import config sections
- Simplify MCP proxy to only send X-Vire-User-ID (Option B — server resolves navexa_key)
- Keep dev login (unsigned JWT) working — server has dev_user imported

## Out of scope
- OAuth 2.1 (future)
- Real JWT validation (future)
- Auth middleware (future)

## Approach

### API Client (`internal/client/vire_client.go`)
Simple HTTP client targeting `config.API.URL` (vire-server at :4242).

Methods needed:
- `GetUser(userID string) (*UserProfile, error)` — GET /api/users/{id}
- `UpdateUser(userID string, fields map[string]string) (*UserProfile, error)` — PUT /api/users/{id}
- `Login(username, password string) (*LoginResult, error)` — POST /api/auth/login

Response types:
```go
type UserProfile struct {
    Username        string `json:"username"`
    Email           string `json:"email"`
    Role            string `json:"role"`
    NavexaKeySet    bool   `json:"navexa_key_set"`
    NavexaKeyPreview string `json:"navexa_key_preview"`
}
```

The server GET /api/users/{id} returns `{ status: ok, data: { username, email, role, navexa_key_set, navexa_key_preview } }`. It never returns the full navexa_key or password.

### Handler Changes

**SettingsHandler** — Change function signatures:
- `userLookupFn func(string) (*models.User, error)` → `userLookupFn func(string) (*client.UserProfile, error)`
- `userSaveFn func(*models.User) error` → `userSaveFn func(string, map[string]string) error`
- GET: use `NavexaKeySet` and `NavexaKeyPreview` from UserProfile (no raw key access)
- POST: call `UpdateUser(sub, map[string]string{"navexa_key": value})`

**DashboardHandler** — Change function signature:
- `userLookupFn func(string) (*models.User, error)` → `userLookupFn func(string) (*client.UserProfile, error)`
- Check `!user.NavexaKeySet` instead of `user.NavexaKey == ""`

**MCP Handler** — Simplify:
- `userLookupFn` now only returns UserID (no NavexaKey needed since server resolves it)
- Change signature to `func(userID string) (*UserContext, error)` (same type, but NavexaKey always empty)
- Simpler: just return `&UserContext{UserID: sub}` directly from JWT, no server call needed

**MCP Proxy** — Remove NavexaKey injection:
- In `applyUserHeaders()`, remove the `X-Vire-Navexa-Key` block (lines 79-81)
- Keep `X-Vire-User-ID` — server resolves navexa_key from user profile

**Auth Handler** — No change:
- Dev login stays as-is (unsigned JWT with sub=dev_user)
- Server has dev_user auto-imported

### App.go Changes
- Remove `StorageManager` field
- Remove `initStorage()` method
- Remove importer call
- Remove `Close()` storage logic
- Create `client.NewVireClient(cfg.API.URL)` in `New()`
- Replace userLookup closures:
  - `userLookupModels` → closure calling `vireClient.GetUser(userID)`
  - `userSave` → closure calling `vireClient.UpdateUser(userID, fields)`
  - MCP `userLookup` → closure that just creates `UserContext{UserID: sub}` from JWT (no server call)

### Config Changes
- Remove `ImportConfig` struct and `Import` field
- Remove `StorageConfig`, `BadgerConfig` structs and `Storage` field
- Remove `VIRE_BADGER_PATH` env override
- Remove from defaults

### Files to Delete
- `internal/storage/badger/` (entire directory — 4 files)
- `internal/storage/factory.go`
- `internal/interfaces/storage.go`
- `internal/models/user.go`
- `internal/importer/` (entire directory — 3 files)

### Dependencies to Remove
- `github.com/timshannon/badgerhold/v4` (and transitive badger/v4)
- `golang.org/x/crypto` (bcrypt — only used by importer)
- Run `go mod tidy`

## Files Expected to Change
- `internal/client/vire_client.go` — NEW: API client
- `internal/client/vire_client_test.go` — NEW: tests with httptest
- `internal/app/app.go` — remove storage, wire API client
- `internal/handlers/settings.go` — use client types
- `internal/handlers/dashboard.go` — use client types
- `internal/mcp/proxy.go` — remove NavexaKey header injection
- `internal/mcp/context.go` — remove NavexaKey from UserContext
- `internal/mcp/handler.go` — simplify user lookup
- `internal/config/config.go` — remove storage/import config
- `internal/config/defaults.go` — remove storage/import defaults
- `go.mod` / `go.sum` — remove dependencies
- Tests across all modified packages
