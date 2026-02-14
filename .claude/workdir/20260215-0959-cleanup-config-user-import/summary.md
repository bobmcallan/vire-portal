# Summary: Remove API Keys from Config & Add User Import

**Date:** 2026-02-15
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Removed `KeysConfig` struct and `Keys` field; removed EODHD/Navexa/Gemini env var bindings; added `ImportConfig` struct and `Import` field |
| `internal/config/defaults.go` | Removed `Keys` default; added `Import: ImportConfig{Users: false}` |
| `internal/config/config_test.go` | Updated tests for key removal and import config |
| `internal/mcp/proxy.go` | Removed X-Vire-EODHD-Key, X-Vire-Navexa-Key, X-Vire-Gemini-Key header injection |
| `internal/mcp/mcp_test.go` | Updated proxy tests |
| `internal/app/app.go` | Removed HasEODHD/HasNavexa/HasGemini from dashboard config status; added user import call on startup |
| `internal/handlers/` | Removed API key status fields from dashboard handler |
| `internal/handlers/handlers_test.go` | Updated handler tests |
| `internal/models/user.go` | New — User model with badgerhold tags |
| `internal/importer/users.go` | New — reads users.json, bcrypt-hashes passwords, stores in BadgerDB (idempotent) |
| `internal/importer/users_test.go` | New — tests for import logic |
| `internal/importer/users_stress_test.go` | New — stress tests for edge cases (long passwords, malformed JSON, etc.) |
| `data/users.json` | New — dev and admin users with passwords, email bobmcallan@gmail.com |
| `docker/vire-portal.toml` | Removed `[keys]` section; added `[import] users = true` |
| `docker/vire-portal.toml.example` | Same |
| `docs/requirements.md` | Updated to remove API key references, add import config |
| `README.md` | Updated configuration section |
| `.claude/skills/develop/SKILL.md` | Updated Reference section |

## Tests
- All tests pass (`go test ./...`)
- `go vet ./...` clean
- New test files: `internal/importer/users_test.go`, `internal/importer/users_stress_test.go`
- Stress tests cover: long passwords, malformed JSON, missing files, duplicate usernames

## Documentation Updated
- README.md — removed API key config references
- docs/requirements.md — updated config and API sections
- .claude/skills/develop/SKILL.md — updated Reference tables

## Devils-Advocate Findings
- **Long password bug**: bcrypt's 72-byte limit could halt import with very long passwords. Fixed with password length validation/cap before hashing.

## Notes
- Remaining references to EODHD/Gemini/Navexa in `internal/vire/` are service-layer models/interfaces (shared with backend), not portal config — correctly retained
- User import is idempotent: existing users are skipped on re-import
- Passwords stored as bcrypt hashes, never logged in plaintext
