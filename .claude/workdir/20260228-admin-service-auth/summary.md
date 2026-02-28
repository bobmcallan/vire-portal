# Summary: Admin Users Service Authentication

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/config/config.go` | Added `ServiceConfig` struct (Key, PortalID), added to `Config`, added `VIRE_SERVICE_KEY` and `VIRE_PORTAL_ID` env overrides |
| `internal/config/defaults.go` | Added `Service: ServiceConfig{}` default |
| `internal/client/vire_client.go` | Added `AdminUser` type, `RegisterService()`, `AdminListUsers()`, `AdminUpdateUserRole()` methods with correct admin API endpoints and `X-Vire-Service-ID` header |
| `internal/seed/service.go` | **New**: `RegisterService()` with retry logic (3 attempts, 2s delay) |
| `internal/seed/admin.go` | Rewritten: `SyncAdmins()` now takes `serviceUserID` param, uses `AdminListUsers` + `AdminUpdateUserRole` (correct endpoints) |
| `internal/app/app.go` | Startup flow: service registration before admin sync, warning when admin_users set without service_key |
| `internal/config/config_test.go` | Added tests for ServiceConfig TOML parsing and env overrides |
| `internal/client/vire_client_test.go` | Added tests for RegisterService, AdminListUsers, AdminUpdateUserRole |
| `internal/seed/admin_test.go` | Rewritten: all 8 tests updated to mock admin API endpoints/format |
| `internal/seed/service_test.go` | **New**: RegisterService success/retry tests |
| `internal/seed/admin_stress_test.go` | Updated to match new API format |
| `internal/client/service_auth_stress_test.go` | **New**: 53 adversarial tests (key leakage, injection, concurrent access) |
| `internal/seed/service_auth_stress_test.go` | **New**: 26 adversarial tests (retry, partial failure, hostile inputs) |
| `internal/config/service_config_stress_test.go` | **New**: 15 adversarial tests (hostile env, config interactions) |
| `README.md` | Added `VIRE_SERVICE_KEY` and `VIRE_PORTAL_ID` config rows |
| `.claude/skills/develop/SKILL.md` | Added service config rows |

## Tests
- Unit tests: all existing + new tests pass
- Stress tests: 94 adversarial tests (security, edge cases, race conditions)
- All tests pass, `go vet` clean

## Architecture
- ServiceConfig follows existing patterns (AuthConfig, APIConfig)
- VireClient methods follow GetUser/ExchangeOAuth pattern
- seed/service.go follows DevUsers retry pattern
- App startup: service registration -> admin sync (goroutine, non-blocking)
- Architect signed off (task #2)

## Devils-Advocate
- 94 stress tests written, all pass
- No security fixes needed
- Key only in POST body, never in headers or error messages
- Response bodies limited (1MB) and always closed
- Concurrent access safe (idempotent operations)
- Signed off (task #4)

## Key Design Points
- `PUT /api/users/{id}` silently ignores role field — replaced with `PATCH /api/admin/users/{id}/role`
- `GET /api/users` replaced with `GET /api/admin/users` (requires service auth)
- Service registration: `POST /api/services/register` with shared key handshake
- `X-Vire-Service-ID` header (separate from `X-Vire-User-ID`) carries portal identity
- Additive only — does not demote existing admins not in config list
- Multi-instance safe — idempotent registration and sync

## Notes
- Server-side implementation needed in vire-server per `/home/bobmc/development/vire/docs/features/20260228-service-registration.md`
- Integration tests against real containers deferred until server-side is implemented
- Existing `ListUsers()` and `UpdateUser()` kept (used by settings/dashboard)
