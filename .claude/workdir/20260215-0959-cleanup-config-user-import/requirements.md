# Requirements: Remove API Keys from Config & Add User Import

**Date:** 2026-02-15
**Requested:** Remove EODHD/Gemini/Navexa keys from portal config; create user import system

## Scope

### In scope
- Remove `KeysConfig` struct and `[keys]` TOML section entirely (all 3 keys go)
- Remove EODHD/Gemini proxy headers (backend handles these)
- Remove Navexa proxy header from config (user-configured, not portal config)
- Remove env var bindings: `EODHD_API_KEY`, `NAVEXA_API_KEY`, `GEMINI_API_KEY`
- Remove `HasEODHD`, `HasNavexa`, `HasGemini` from `DashboardConfigStatus`
- Create `data/users.json` import file with dev + admin users (passwords, email)
- Add `[import] users = true` config option
- Load users into BadgerDB on startup when enabled
- Update all tests

### Out of scope
- Per-request Navexa key injection (future user settings feature)
- Auth/session middleware changes
- UI changes

## Approach

### 1. Remove API Keys
- Delete `KeysConfig` struct from `internal/config/config.go`
- Remove `Keys` field from `Config` struct
- Remove `[keys]` from all TOML files and examples
- Remove env var bindings for all 3 key env vars in `applyEnvOverrides()`
- Remove header injection for all 3 keys in `internal/mcp/proxy.go` (`NewMCPProxy`)
- Remove `HasEODHD`, `HasNavexa`, `HasGemini` from dashboard config status in `internal/app/app.go`
- Remove from `internal/handlers/` dashboard handler if referenced
- Update all tests in `config_test.go`, `mcp_test.go`, `handlers_test.go`

### 2. User Import System
- Add `ImportConfig` struct to config: `Import.Users bool`
- Add `[import]` section to TOML files
- Create `data/users.json` with structure:
  ```json
  {
    "users": [
      {
        "username": "dev",
        "email": "bobmcallan@gmail.com",
        "password": "dev123",
        "role": "developer"
      },
      {
        "username": "admin",
        "email": "bobmcallan@gmail.com",
        "password": "admin123",
        "role": "admin"
      }
    ]
  }
  ```
- Create `internal/importer/users.go` — reads JSON, hashes passwords (bcrypt), stores in BadgerDB
- Create user model in `internal/models/user.go` (or similar)
- Wire into `app.go` startup: after storage init, before handler init, check `cfg.Import.Users` and run import
- Key pattern in BadgerDB: store as badgerhold structs (like existing `KVEntry`)
- Import is idempotent: skip users that already exist

## Files Expected to Change
- `internal/config/config.go` — remove KeysConfig, add ImportConfig
- `internal/config/defaults.go` — remove Keys default, add Import default
- `internal/config/config_test.go` — update tests
- `internal/mcp/proxy.go` — remove key header injection
- `internal/mcp/mcp_test.go` — update tests
- `internal/app/app.go` — remove key status, add import call
- `internal/handlers/` — remove key status refs
- `internal/handlers/handlers_test.go` — update tests
- `docker/vire-portal.toml` — remove [keys], add [import]
- `docker/vire-portal.toml.example` — same
- `docker/vire-mcp.toml` — remove keys if present
- `data/users.json` — new file
- `internal/importer/users.go` — new file
- `internal/models/user.go` — new file (or equivalent)
