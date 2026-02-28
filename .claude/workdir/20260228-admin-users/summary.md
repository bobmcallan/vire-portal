# Summary: Admin Users Configuration at Startup

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/config/config.go` | Added `AdminUsers string` field, `AdminEmails()` method, `VIRE_ADMIN_USERS` env override |
| `internal/config/defaults.go` | Added `AdminUsers: ""` default |
| `internal/client/vire_client.go` | Added `ListUsers()` method (GET /api/users) |
| `internal/seed/admin.go` | New file: `SyncAdmins()` with retry logic, idempotent admin role sync |
| `internal/app/app.go` | Added startup goroutine call to `SyncAdmins` (all modes) |
| `internal/config/config_test.go` | 7+ tests for AdminEmails parsing, env/TOML overrides |
| `internal/client/vire_client_test.go` | ListUsers tests |
| `internal/seed/admin_test.go` | 8+ tests for SyncAdmins logic |
| `internal/seed/admin_stress_test.go` | Stress tests for admin sync (devils-advocate) |
| `internal/config/admin_stress_test.go` | Stress tests for config parsing (devils-advocate) |
| `internal/client/list_users_stress_test.go` | Stress tests for ListUsers (devils-advocate) |
| `README.md` | Added admin_users config row |
| `.claude/skills/develop/SKILL.md` | Added VIRE_ADMIN_USERS config row |

## Tests
- Unit tests: 40+ tests across config, client, and seed packages
- Stress tests: 35 adversarial tests (security, edge cases, race conditions)
- All tests pass, `go vet` clean
- Fix rounds: 1 (corrected testLogger() signature in stress test)

## Architecture
- Follows existing patterns: config field + env override + seed goroutine at startup
- VireClient.ListUsers() mirrors GetUser() pattern
- SyncAdmins() mirrors DevUsers() retry pattern
- Idempotent and safe for multi-instance (no locks needed)
- Additive only — does not demote existing admins not in the list
- Architect signed off (task #2)

## Devils-Advocate
- 35 stress tests written, all pass
- No security fixes needed
- Verified: hostile email inputs, race conditions, partial failures, resource leaks
- Signed off (task #4)

## Configuration

TOML:
```toml
admin_users = "bobmcallan@gmail.com,alice@example.com"
```

Env:
```bash
export VIRE_ADMIN_USERS="bobmcallan@gmail.com,alice@example.com"
```

## Notes
- Server running on port 8883 (left running per workflow)
- Two pre-existing test failures unrelated to this feature
