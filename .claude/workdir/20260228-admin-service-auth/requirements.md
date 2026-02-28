# Requirements: Admin Users Service Authentication

**Source:** `docs/features/20260228-admin-users-service-auth.md`

## Scope

Rewrite the admin sync system to use service registration + admin API endpoints instead of the current `ListUsers()`/`UpdateUser()` approach (which silently fails — `PUT /api/users/{id}` ignores the role field).

## What Exists (from first implementation round)

| File | Current State |
|------|---------------|
| `internal/config/config.go` | Has `AdminUsers string` field, `AdminEmails()` method, `VIRE_ADMIN_USERS` env override |
| `internal/config/defaults.go` | Has `AdminUsers: ""` default |
| `internal/client/vire_client.go` | Has `ListUsers()` (GET /api/users — WRONG endpoint), `UpdateUser()` (PUT /api/users/{id} — silently ignores role) |
| `internal/seed/admin.go` | Uses `ListUsers()` + `UpdateUser()` — both wrong endpoints |
| `internal/app/app.go` | Line 79-81: simple `go seed.SyncAdmins(cfg.API.URL, emails, logger)` |
| Tests | admin_test.go (8 tests), admin_stress_test.go (35 tests) — all mock wrong endpoints |

## Changes Required

### 1. Config (`internal/config/config.go` + `defaults.go`)

Add `ServiceConfig` struct and field:
```go
type ServiceConfig struct {
    Key      string `toml:"key"`
    PortalID string `toml:"portal_id"`
}
```

Add to `Config`:
```go
Service ServiceConfig `toml:"service"`
```

Add env overrides in `applyEnvOverrides()`:
- `VIRE_SERVICE_KEY` → `config.Service.Key`
- `VIRE_PORTAL_ID` → `config.Service.PortalID`

Add default in `defaults.go`:
```go
Service: ServiceConfig{},
```

### 2. VireClient (`internal/client/vire_client.go`)

Add new type:
```go
type AdminUser struct {
    ID        string `json:"id"`
    Email     string `json:"email"`
    Name      string `json:"name"`
    Role      string `json:"role"`
    Provider  string `json:"provider"`
    CreatedAt string `json:"created_at"`
}
```

Add three new methods:

**`RegisterService(serviceID, serviceKey string) (string, error)`**
- POST /api/services/register
- Body: `{"service_id": "...", "service_key": "...", "service_type": "portal"}`
- Response: `{"status": "ok", "service_user_id": "service:portal-prod-1", "registered_at": "..."}`
- Returns `service_user_id`

**`AdminListUsers(serviceID string) ([]AdminUser, error)`**
- GET /api/admin/users
- Header: `X-Vire-Service-ID: <serviceID>`
- Response: `{"users": [...]}`  (NOT `{"status":"ok","data":[...]}`)

**`AdminUpdateUserRole(serviceID, userID, role string) error`**
- PATCH /api/admin/users/{id}/role
- Header: `X-Vire-Service-ID: <serviceID>`
- Body: `{"role": "admin"}`

Keep existing `ListUsers()` and `UpdateUser()` — they are used elsewhere.

### 3. Service Registration (`internal/seed/service.go` — NEW)

```go
func RegisterService(apiURL, serviceID, serviceKey string, logger *common.Logger) (string, error)
```

- Creates VireClient, calls `RegisterService()`
- Retry: 3 attempts, 2s delay (same `seedRetryAttempts`/`seedRetryDelay` from `seed.go`)
- Returns `service_user_id`

### 4. Admin Sync Rewrite (`internal/seed/admin.go`)

Change signature:
```go
func SyncAdmins(apiURL string, adminEmails []string, serviceUserID string, logger *common.Logger)
```

Internal changes:
- Use `AdminListUsers(serviceUserID)` instead of `ListUsers()`
- Use `AdminUpdateUserRole(serviceUserID, userID, "admin")` instead of `UpdateUser()`
- Response format is `{"users": [...]}` with `id` field (not `username`)
- Match by `strings.ToLower(email)` as before
- Additive only — does not demote

### 5. App Startup (`internal/app/app.go`)

Replace lines 79-81 with:
```go
if cfg.Service.Key != "" && len(cfg.AdminEmails()) > 0 {
    go func() {
        portalID := cfg.Service.PortalID
        if portalID == "" {
            portalID, _ = os.Hostname()
        }
        serviceUserID, err := seed.RegisterService(cfg.API.URL, portalID, cfg.Service.Key, logger)
        if err != nil {
            logger.Warn().Err(err).Msg("service registration failed, skipping admin sync")
            return
        }
        seed.SyncAdmins(cfg.API.URL, cfg.AdminEmails(), serviceUserID, logger)
    }()
} else if len(cfg.AdminEmails()) > 0 {
    logger.Warn().Msg("VIRE_ADMIN_USERS set but VIRE_SERVICE_KEY not configured — admin sync disabled")
}
```

### 6. Tests

**Config tests** (`internal/config/config_test.go`):
- ServiceConfig TOML parsing
- `VIRE_SERVICE_KEY` env override
- `VIRE_PORTAL_ID` env override

**Client tests** (`internal/client/vire_client_test.go`):
- RegisterService success/failure
- AdminListUsers with service header
- AdminUpdateUserRole with service header

**Seed tests** (`internal/seed/admin_test.go`):
- Rewrite all 8 tests to use admin API endpoints/format
- New signature with serviceUserID parameter

**Seed service tests** (`internal/seed/service_test.go` — NEW):
- RegisterService success
- RegisterService retry on failure

### 7. Docs

- `README.md`: Add `VIRE_SERVICE_KEY` and `VIRE_PORTAL_ID` to config table
- `.claude/skills/develop/SKILL.md`: Add service config rows to config table
