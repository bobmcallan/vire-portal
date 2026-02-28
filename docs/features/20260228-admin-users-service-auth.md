# Admin Users & Service Authentication: Portal Requirements

**Date:** 2026-02-28
**Status:** Requirements
**Depends on:** [Server: Service Registration](file:///home/bobmc/development/vire/docs/features/20260228-service-registration.md)

## Overview

At startup, the portal syncs a configured list of admin user emails with vire-server. Users matching these emails are promoted to the `admin` role. The portal authenticates to the admin API by registering as a service user via a shared key handshake.

## Motivation

- `bobmcallan@gmail.com` is currently the only admin, configured manually
- Admin users should be declarative — defined in config, enforced at startup
- The portal cannot use breakglass credentials (only available in server logs)
- Multiple portal instances must work independently with unique identities
- Role updates via `PUT /api/users/{id}` are silently ignored by vire-server — the correct endpoint is `PATCH /api/admin/users/{id}/role`, which requires admin or service auth

## Configuration

### New Config Options

**TOML** (`vire-portal.toml`):
```toml
admin_users = "bobmcallan@gmail.com,alice@example.com"

[service]
key = ""
portal_id = ""
```

**Environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `VIRE_ADMIN_USERS` | `""` | Comma-separated list of admin user emails |
| `VIRE_SERVICE_KEY` | `""` | Shared secret for service registration (must match vire-server) |
| `VIRE_PORTAL_ID` | hostname | Unique ID for this portal instance |

**Behaviour when unset:**
- `VIRE_ADMIN_USERS` empty: no admin sync at startup
- `VIRE_SERVICE_KEY` empty: skip service registration, admin sync uses unauthenticated fallback (will fail on secured servers)
- `VIRE_PORTAL_ID` empty: defaults to machine hostname

### Config Struct

```go
type ServiceConfig struct {
    Key      string `toml:"key"`
    PortalID string `toml:"portal_id"`
}
```

Added to `Config`:
```go
AdminUsers string        `toml:"admin_users"`
Service    ServiceConfig `toml:"service"`
```

### `AdminEmails()` Method

Parses `AdminUsers` string into a deduplicated `[]string`:
- Split on commas
- Trim whitespace
- Lowercase
- Filter empty strings

## Startup Flow

```
Portal starts
    |
    v
1. Load config (admin_users, service.key, service.portal_id)
    |
    v
2. If service.key is set:
    |   POST /api/services/register
    |   { service_id, service_key, service_type: "portal" }
    |   --> receives service_user_id
    |
    v
3. If admin_users is set:
    |   GET /api/admin/users (with X-Vire-Service-ID header)
    |   --> list all users with emails and roles
    |
    |   For each configured admin email:
    |     Find matching user by email (case-insensitive)
    |     If role != "admin":
    |       PATCH /api/admin/users/{id}/role (with X-Vire-Service-ID header)
    |       { "role": "admin" }
    |
    |   Log summary: "admin sync: N checked, M updated, K not found"
    |
    v
4. Continue startup (handlers, server, etc.)
```

Both steps 2 and 3 run as a goroutine (non-blocking) with retry logic (3 attempts, 2s delay).

## VireClient Changes

### New Methods

**`RegisterService(serviceID, serviceKey string) (string, error)`**

```
POST /api/services/register
Content-Type: application/json

{
  "service_id": "portal-prod-1",
  "service_key": "<shared-secret>",
  "service_type": "portal"
}
```

Returns the `service_user_id` from the response (e.g. `service:portal-prod-1`).

**`AdminListUsers(serviceID string) ([]AdminUser, error)`**

```
GET /api/admin/users
X-Vire-Service-ID: service:portal-prod-1
```

Response format:
```json
{
  "users": [
    {
      "id": "alice",
      "email": "alice@example.com",
      "name": "Alice",
      "role": "user",
      "provider": "google",
      "created_at": "2026-01-15T10:00:00Z"
    }
  ]
}
```

**`AdminUpdateUserRole(serviceID, userID, role string) error`**

```
PATCH /api/admin/users/alice/role
X-Vire-Service-ID: service:portal-prod-1
Content-Type: application/json

{
  "role": "admin"
}
```

### AdminUser Type

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

This is separate from the existing `UserProfile` type because the admin endpoint returns different fields (notably `id` instead of `username`, and no `navexa_key_set`).

## Admin Sync Rewrite

### `seed/service.go` (new)

```go
func RegisterService(apiURL, serviceID, serviceKey string, logger *common.Logger) (string, error)
```

- Creates VireClient, calls `RegisterService()`
- Retry logic: 3 attempts, 2s delay (same pattern as `DevUsers`)
- Returns the `service_user_id` for use in subsequent admin calls
- Logs success/failure

### `seed/admin.go` (rewrite)

```go
func SyncAdmins(apiURL string, adminEmails []string, serviceUserID string, logger *common.Logger)
```

Changes from current implementation:
- Takes `serviceUserID` parameter (from registration step)
- Uses `AdminListUsers(serviceUserID)` instead of `ListUsers()` (correct endpoint)
- Uses `AdminUpdateUserRole(serviceUserID, userID, "admin")` instead of `UpdateUser()` (correct endpoint)
- Response format is `{"users": [...]}` not `{"status": "ok", "data": [...]}`
- User ID field is `id` not `username`
- Additive only — does not demote existing admins not in the config list

### `app.go` Startup Integration

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

## Multi-Instance Behaviour

| Scenario | Behaviour |
|----------|-----------|
| Portal 1 and Portal 2 start simultaneously | Both register independently, both sync admins — idempotent, no conflict |
| Portal 1 sets user to admin, Portal 2 sees user already admin | Portal 2 skips the update (no PUT call) |
| Portal 1 has `VIRE_PORTAL_ID=prod-1`, Portal 2 has `prod-2` | Separate service users: `service:prod-1`, `service:prod-2` |
| Same portal restarts | Re-registers with same ID, updates `modified_at` |

## Existing Unit Tests

The existing unit tests in `internal/seed/admin_test.go`, `internal/config/config_test.go`, and `internal/client/` use `httptest.NewServer` mocks and remain valid for testing parsing logic and error handling.

## Integration Tests

New integration tests using `testcontainers-go` (already a dependency) to verify the full flow against real vire-server + SurrealDB containers.

### Test Infrastructure

Create `tests/api/` directory with test environment setup following the pattern in `/home/bobmc/development/vire/tests/common/containers.go`:

- Pull `ghcr.io/bobmcallan/vire-server:latest` and `surrealdb/surrealdb:v3.0.0`
- Start both on a shared Docker network
- Expose vire-server port for portal API calls

### Test Cases

**Registration:**
- Portal registers with valid key — service user created
- Portal re-registers — idempotent, no error
- Wrong key — returns 403
- No key configured on server — returns 501

**Admin Sync (end-to-end):**
- Create users via vire-server API, then call `SyncAdmins` — roles updated correctly
- User already admin — no PATCH call made
- Configured email not found — logged, no error
- Service user cannot login via `/api/auth/login`

## Files to Change

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `ServiceConfig` struct, add to `Config`, add env overrides for `VIRE_SERVICE_KEY` and `VIRE_PORTAL_ID` |
| `internal/config/defaults.go` | Add default `Service` section |
| `internal/config/config_test.go` | Add tests for service config parsing and env overrides |
| `internal/client/vire_client.go` | Add `RegisterService()`, `AdminListUsers()`, `AdminUpdateUserRole()` methods |
| `internal/seed/service.go` | **New**: `RegisterService()` with retry logic |
| `internal/seed/admin.go` | Rewrite to use admin endpoints with `X-Vire-Service-ID` header |
| `internal/seed/admin_test.go` | Update mocked endpoints to match admin API format |
| `internal/app/app.go` | Wire service registration + admin sync at startup |
| `tests/api/` | **New**: Integration test directory with testcontainers setup |
| `README.md` | Add `VIRE_SERVICE_KEY`, `VIRE_PORTAL_ID` to config table |
| `.claude/skills/develop/SKILL.md` | Add service config to config table |

## Security Considerations

- `VIRE_SERVICE_KEY` must be 32+ characters, stored in environment variables (not committed to git)
- If compromised, an attacker can list users (emails, names, roles) and change roles — but cannot access portfolio data, API keys, or login as any user
- Rotation: change key on both server and portal, restart both
- The service key is sent in the POST body (not a header), so it does not appear in access logs
- TLS required in production (Fly.dev provides this)
