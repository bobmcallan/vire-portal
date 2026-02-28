# Admin Users Configuration at Startup

## Goal
Add a config option (`VIRE_ADMIN_USERS` env var / `admin_users` TOML field) that accepts a comma-separated list of emails. At startup, the portal syncs these users to have the `admin` role on vire-server. The operation is idempotent and safe for multi-instance deployments.

## Scope

### 1. Config (`internal/config/`)

**config.go:**
- Add `AdminUsers string` field to the `Config` struct (top-level, like `Environment`)
  ```go
  AdminUsers string `toml:"admin_users"`
  ```
- Add helper method `AdminEmails() []string` that parses the comma-separated string, trims whitespace, lowercases, and filters empty strings
- Add env override in `applyEnvOverrides()`:
  ```go
  if adminUsers := os.Getenv("VIRE_ADMIN_USERS"); adminUsers != "" {
      config.AdminUsers = adminUsers
  }
  ```

**defaults.go:**
- Add `AdminUsers: ""` to `NewDefaultConfig()`

### 2. VireClient (`internal/client/vire_client.go`)

Add `ListUsers()` method:
```go
func (c *VireClient) ListUsers() ([]UserProfile, error)
```
- Calls `GET /api/users` on vire-server
- Returns `[]UserProfile` (same struct already exists with Username, Email, Role fields)
- Response format: `{ "status": "ok", "data": [...] }`

### 3. Admin Sync (`internal/seed/admin.go`)

New file with function:
```go
func SyncAdmins(apiURL string, adminEmails []string, logger *common.Logger)
```

Flow:
1. Create VireClient
2. Call `ListUsers()` to get all current users
3. Build map of `lowercase(email) → UserProfile` for quick lookup
4. For each configured admin email:
   - Find user by email match (case-insensitive)
   - If user found and role != "admin": call `UpdateUser(username, {"role": "admin"})`
   - If user not found: log info (user hasn't registered yet)
5. Log summary: "admin sync: N users checked, M roles updated, K emails not found"

Properties:
- Runs in **all modes** (dev + prod), not just dev
- Spawned as goroutine at startup (non-blocking)
- Has retry logic (3 attempts with 2s delay, matching `DevUsers` pattern)
- **Idempotent**: setting role=admin on an admin is a no-op. Safe for concurrent multi-instance execution.
- Does NOT demote existing admins not in the list (additive only)

### 4. App Startup (`internal/app/app.go`)

In `New()`, after `initHandlers()`:
```go
if emails := cfg.AdminEmails(); len(emails) > 0 {
    go seed.SyncAdmins(cfg.API.URL, emails, logger)
}
```

### 5. Tests

**`internal/config/config_test.go`:**
- `TestAdminEmails_CommaSeparated` — parses "a@x.com,b@x.com" → ["a@x.com", "b@x.com"]
- `TestAdminEmails_Whitespace` — trims spaces: " a@x.com , b@x.com " → ["a@x.com", "b@x.com"]
- `TestAdminEmails_Empty` — returns nil/empty for ""
- `TestAdminEmails_SingleEmail` — single email works
- `TestAdminEmails_CaseNormalization` — lowercases all emails
- `TestApplyEnvOverrides_AdminUsers` — VIRE_ADMIN_USERS env var
- `TestLoadFromFiles_AdminUsers` — TOML `admin_users` field

**`internal/seed/admin_test.go`:**
- `TestSyncAdmins_UpdatesNonAdminUser` — user with role="user" gets updated to "admin"
- `TestSyncAdmins_SkipsExistingAdmin` — user already admin → no PUT call
- `TestSyncAdmins_EmailNotFound` — configured email not in user list → logs info, no error
- `TestSyncAdmins_MultipleEmails` — handles multiple emails correctly
- `TestSyncAdmins_ServerError` — handles vire-server errors gracefully
- `TestSyncAdmins_EmptyList` — no-op for empty email list
- `TestSyncAdmins_CaseInsensitive` — email matching is case-insensitive

**`internal/client/vire_client_test.go`:**
- `TestListUsers_Success` — parses user list response
- `TestListUsers_ServerError` — handles error response

### 6. Docs Updates

**README.md** — Add to config table:
| Admin users | `VIRE_ADMIN_USERS` | `""` |

**SKILL.md** — Add to config table:
| Admin users | `VIRE_ADMIN_USERS` | `""` |

## Files to Change

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `AdminUsers` field, `AdminEmails()` method, env override |
| `internal/config/defaults.go` | Add `AdminUsers: ""` default |
| `internal/config/config_test.go` | Add config tests |
| `internal/client/vire_client.go` | Add `ListUsers()` method |
| `internal/client/vire_client_test.go` | Add ListUsers tests |
| `internal/seed/admin.go` | New file: `SyncAdmins()` |
| `internal/seed/admin_test.go` | New file: admin sync tests |
| `internal/app/app.go` | Add startup call to `SyncAdmins` |
| `README.md` | Add config documentation |
| `.claude/skills/develop/SKILL.md` | Add config documentation |

## Race Condition Analysis

The operation is inherently **safe for multi-instance** deployment:
- Each instance reads current state, compares to desired state, updates only mismatches
- Multiple instances setting the same user to admin simultaneously is harmless (last write wins, all writes are identical)
- No distributed lock needed — idempotent convergence
- vire-server handles concurrent PUT requests safely (SurrealDB atomic writes)

## TOML Example
```toml
admin_users = "bobmcallan@gmail.com,alice@example.com"
```

## Env Example
```bash
export VIRE_ADMIN_USERS="bobmcallan@gmail.com,alice@example.com"
```
