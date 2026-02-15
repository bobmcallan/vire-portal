# Summary: Remove BadgerDB, use vire-server API for user data

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/client/vire_client.go` | NEW: HTTP API client for vire-server (GetUser, UpdateUser) |
| `internal/client/vire_client_test.go` | NEW: Tests with httptest for API client |
| `internal/app/app.go` | Removed StorageManager, initStorage, importer; wires API client closures |
| `internal/handlers/settings.go` | Uses `*client.UserProfile` for lookup, `map[string]string` for save |
| `internal/handlers/dashboard.go` | Uses `*client.UserProfile`, checks `NavexaKeySet` instead of `NavexaKey == ""` |
| `internal/handlers/handlers_test.go` | Updated all handler tests to use client types, fixed POST unknown-user test |
| `internal/mcp/proxy.go` | Removed `X-Vire-Navexa-Key` header injection (server resolves via Option B) |
| `internal/mcp/context.go` | Removed `NavexaKey` field from `UserContext` |
| `internal/mcp/handler.go` | Simplified: creates `UserContext{UserID: sub}` from JWT, no server call needed |
| `internal/mcp/mcp_test.go` | Updated tests to remove NavexaKey references |
| `internal/config/config.go` | Removed `ImportConfig`, `StorageConfig`, `BadgerConfig` structs and env overrides |
| `internal/config/defaults.go` | Removed storage/import defaults |
| `internal/config/config_test.go` | Removed tests for deleted config fields |
| `internal/server/routes_test.go` | Removed `cfg.Storage.Badger.Path` references |
| `go.mod` / `go.sum` | Removed badgerhold/v4, x/crypto, and transitive deps (221 lines from go.sum) |
| `README.md` | Updated architecture, removed BadgerDB/importer references |
| `docs/migration-remove-badger.md` | Updated phase status (phases 2-4 marked complete) |
| `config/vire-portal.toml.example` | Removed storage/import sections |
| `.claude/skills/develop/SKILL.md` | Removed BadgerDB from key directories |

### Files Deleted
| File | Reason |
|------|--------|
| `internal/storage/badger/connection.go` | BadgerDB storage removed |
| `internal/storage/badger/kv_storage.go` | BadgerDB storage removed |
| `internal/storage/badger/kv_storage_test.go` | BadgerDB storage removed |
| `internal/storage/badger/manager.go` | BadgerDB storage removed |
| `internal/storage/factory.go` | Storage factory removed |
| `internal/interfaces/storage.go` | Storage interfaces removed |
| `internal/models/user.go` | User model removed (replaced by client.UserProfile) |
| `internal/importer/users.go` | User importer removed |
| `internal/importer/users_test.go` | Importer tests removed |
| `internal/importer/users_stress_test.go` | Importer stress tests removed |

## Architecture

**Before:** Portal stored users in BadgerDB, imported from JSON, looked up navexa_key locally, injected `X-Vire-Navexa-Key` header to vire-server.

**After:** Portal is stateless. User data lives on vire-server. Portal calls `GET /api/users/{id}` and `PUT /api/users/{id}` via `internal/client/vire_client.go`. MCP proxy sends only `X-Vire-User-ID` — server resolves navexa_key from its own user profile (Option B).

## Tests
- `internal/client` — 5 tests (GetUser success/not-found, UpdateUser success/error, server-down)
- `internal/handlers` — all handler tests updated for client types, pass
- `internal/mcp` — tests updated to remove NavexaKey, pass
- `internal/server` — routes tests updated, pass
- `internal/config` — removed deleted-field tests, pass (pre-existing docker test failure unrelated)
- `go build ./...` clean
- `go vet ./...` clean
- Server builds and runs, health endpoint responds

## Documentation Updated
- `README.md` — removed BadgerDB/importer architecture references
- `docs/migration-remove-badger.md` — phases 2-4 marked complete
- `config/vire-portal.toml.example` — removed storage/import sections
- `.claude/skills/develop/SKILL.md` — removed BadgerDB from key directories table

## Devils-Advocate Findings
- API client uses `io.LimitReader(resp.Body, 1<<20)` to prevent OOM from large responses
- HTTP client has 10s timeout to prevent hanging on unresponsive server
- No raw navexa_key exposure in portal — server returns only `navexa_key_set` (bool) and `navexa_key_preview` (last 4 chars)
- Header injection sanitized via `sanitizeHeaderValue()` (strips CR/LF)

## Notes
- Net reduction: ~1,669 lines of code removed
- Dependencies removed: badgerhold/v4, badger/v4, x/crypto (bcrypt), and all transitive deps
- Portal is now fully stateless — no local database, no data directory needed
- Dev login continues to work: unsigned JWT with sub=dev_user, server has dev_user auto-imported
- Pre-existing `TestDockerComposeProjectName` failure is unrelated to this change
