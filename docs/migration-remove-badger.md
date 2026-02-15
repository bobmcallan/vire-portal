# Migration: Remove BadgerDB from Portal, Move User Storage to vire-server

## Overview

vire-portal currently stores user data locally in BadgerDB (via badgerhold). This creates a split data model: user credentials and settings live in the portal while portfolio/market data lives in vire-server's file-based storage. This document describes how to consolidate all data storage into vire-server, making the portal a stateless frontend.

## Current State

### What Portal Stores in BadgerDB

| Data | Model | Operations | Used By |
|------|-------|-----------|---------|
| User profile | `models.User` (Username, Email, Password, Role, NavexaKey) | FindOne, Upsert, Insert, Get | Settings, Dashboard, MCP proxy, Importer |
| Key-value pairs | `KVEntry` (Key, Value) | Get, Set, Delete, GetAll | KeyValueStorage interface (currently unused by handlers) |

### Portal Storage Touchpoints

| Handler | Route | Operation | What It Does |
|---------|-------|-----------|-------------|
| DashboardHandler | `GET /dashboard` | Read | Checks if user's NavexaKey is empty → shows warning banner |
| SettingsHandler | `GET /settings` | Read | Reads NavexaKey, shows last 4 chars as preview |
| SettingsHandler | `POST /settings` | Read + Write | Reads user, updates NavexaKey, saves back |
| MCPHandler | `POST /mcp` | Read | Reads user's NavexaKey for X-Vire-Navexa-Key proxy header |
| Importer | Startup | Read + Write | Loads users from `data/users.json`, inserts if not exists |
| AuthHandler | `POST /api/auth/dev` | None | JWT only, no storage |

### vire-server Current State

- **No user storage** — relies on X-Vire-* headers from portal
- **No authentication** — all endpoints are open
- **File-based storage** — portfolios, strategies, plans, market data
- **Has KeyValueStorage interface** — used for runtime config

## Target State

Portal becomes stateless. All user CRUD goes through vire-server API endpoints. Portal still handles:
- HTML template rendering
- Session management (JWT cookies)
- MCP proxy (forwards to vire-server with user context headers)
- Static asset serving

## Required vire-server Endpoints

### User Management

```
POST   /api/users                    → Create user
GET    /api/users/{id}               → Get user by username
PUT    /api/users/{id}               → Update user (email, role, navexa_key)
DELETE /api/users/{id}               → Delete user
POST   /api/users/import             → Bulk import from JSON (replaces portal importer)
```

**Request/Response:**

```
POST /api/users
Content-Type: application/json

{
  "username": "dev_user",
  "email": "bob@example.com",
  "password": "plaintext-hashed-server-side",
  "role": "developer"
}

→ 201 Created
{
  "status": "ok",
  "data": {
    "username": "dev_user",
    "email": "bob@example.com",
    "role": "developer"
  }
}
```

```
GET /api/users/dev_user

→ 200 OK
{
  "status": "ok",
  "data": {
    "username": "dev_user",
    "email": "bob@example.com",
    "role": "developer",
    "navexa_key_set": true,
    "navexa_key_preview": "****ab12"
  }
}
```

Note: GET never returns the full navexa_key or password. Only a boolean flag and last-4 preview.

```
PUT /api/users/dev_user
Content-Type: application/json

{
  "navexa_key": "full-key-here"
}

→ 200 OK
{
  "status": "ok",
  "data": {
    "username": "dev_user",
    "navexa_key_set": true,
    "navexa_key_preview": "****here"
  }
}
```

```
POST /api/users/import
Content-Type: application/json

{
  "users": [
    {
      "username": "dev_user",
      "email": "bob@example.com",
      "password": "dev123",
      "role": "developer"
    }
  ]
}

→ 200 OK
{
  "status": "ok",
  "data": {
    "imported": 1,
    "skipped": 0
  }
}
```

### Authentication

```
POST   /api/auth/login               → Verify credentials, return JWT
POST   /api/auth/validate            → Validate JWT, return user context
```

```
POST /api/auth/login
Content-Type: application/json

{
  "username": "dev_user",
  "password": "dev123"
}

→ 200 OK
{
  "status": "ok",
  "data": {
    "token": "eyJ...",
    "user": {
      "username": "dev_user",
      "email": "bob@example.com",
      "role": "developer"
    }
  }
}
```

```
POST /api/auth/validate
Authorization: Bearer eyJ...

→ 200 OK
{
  "status": "ok",
  "data": {
    "username": "dev_user",
    "email": "bob@example.com",
    "role": "developer",
    "navexa_key_set": true
  }
}
```

### User Settings (Navexa Key)

The `PUT /api/users/{id}` endpoint handles navexa_key updates. No separate settings endpoint needed — the portal settings page calls PUT with the key field.

### User Context for MCP

Currently the portal reads the navexa_key from BadgerDB and injects it as `X-Vire-Navexa-Key`. After migration:

**Option A: Portal fetches key from server, injects header (minimal server change)**
- Portal calls `GET /api/users/{id}` with a field parameter to get the full key
- Requires a privileged endpoint: `GET /api/users/{id}/navexa-key` (server-to-server only)
- Portal continues to inject X-Vire-Navexa-Key header

**Option B: Server resolves user context from X-Vire-User-ID (recommended)**
- Portal sends only `X-Vire-User-ID` header (already does this)
- Server's userContextMiddleware looks up the user's navexa_key from its own storage
- Removes the need for portal to ever handle the raw key
- MCP proxy becomes simpler — no key injection needed

**Recommendation: Option B.** The server already receives `X-Vire-User-ID` on every request. It should resolve the navexa_key internally rather than having the portal fetch and forward it.

## Portal Changes Required

### Remove

| Component | Location | Action |
|-----------|----------|--------|
| BadgerDB connection | `internal/storage/badger/` | Delete entire directory |
| Storage interfaces | `internal/interfaces/storage.go` | Delete file |
| Storage factory | `internal/storage/factory.go` | Delete file |
| User model | `internal/models/user.go` | Delete file |
| User importer | `internal/importer/` | Delete entire directory |
| BadgerDB config | `internal/config/config.go` | Remove `StorageConfig`, `BadgerConfig`, `ImportConfig` |
| Storage init | `internal/app/app.go` | Remove `initStorage()`, `StorageManager` field, `Close()` storage logic |
| badgerhold dependency | `go.mod` | Remove `github.com/timshannon/badgerhold/v4` |
| bcrypt dependency | `go.mod` | Remove (password hashing moves to server) |
| Data directory | `data/` | Remove (no local data) |

### Modify

| Component | Location | Change |
|-----------|----------|--------|
| `app.go` initHandlers | `internal/app/app.go` | Replace userLookup/userSave closures with HTTP client calls to vire-server |
| SettingsHandler | `internal/handlers/settings.go` | GET: call `GET /api/users/{id}`, POST: call `PUT /api/users/{id}` |
| DashboardHandler | `internal/handlers/dashboard.go` | Call `GET /api/users/{id}` to check navexa_key status |
| MCP handler | `internal/mcp/handler.go` | If Option B: stop injecting X-Vire-Navexa-Key, server resolves it |
| MCP proxy | `internal/mcp/proxy.go` | If Option B: remove `applyUserHeaders` navexa_key logic |
| AuthHandler | `internal/handlers/auth.go` | Dev login: call `POST /api/auth/login` on server, real login: same |
| Config | `internal/config/` | Remove storage/import config, keep API URL |

### Add

| Component | Location | Purpose |
|-----------|----------|---------|
| API client | `internal/client/vire_client.go` | HTTP client for vire-server user/auth endpoints |
| Client tests | `internal/client/vire_client_test.go` | Tests with httptest mock server |

## vire-server Changes Required

| Component | Action |
|-----------|--------|
| User model | Add User struct (username, email, password_hash, role, navexa_key) |
| User storage | Add to existing file-based storage (or new user-data store) |
| User handlers | Implement CRUD endpoints listed above |
| Auth handlers | Implement login + validate endpoints |
| Password hashing | bcrypt (move from portal importer) |
| User import | Implement bulk import endpoint (move from portal importer) |
| User context middleware | Enhance to resolve navexa_key from user storage when X-Vire-User-ID is present |
| Routes | Register new `/api/users/*` and `/api/auth/*` routes |

## Migration Sequence

### Phase 1: Add server endpoints (no portal changes)
1. Add User model and storage to vire-server
2. Implement `/api/users/*` CRUD endpoints
3. Implement `/api/auth/login` and `/api/auth/validate`
4. Implement `/api/users/import` for bulk import
5. Enhance userContextMiddleware to resolve navexa_key from X-Vire-User-ID
6. Test all new endpoints

### Phase 2: Add portal API client (parallel storage)
1. Create `internal/client/vire_client.go` in portal
2. Wire into handlers alongside existing BadgerDB calls
3. Run both in parallel — write to both, read from server with BadgerDB fallback
4. Verify server responses match BadgerDB data

### Phase 3: Remove BadgerDB from portal
1. Switch all handlers to use API client exclusively
2. Remove BadgerDB code, storage interfaces, importer, models
3. Remove badgerhold/bcrypt dependencies from go.mod
4. Remove data directory and storage config
5. Update tests to mock API client instead of badgerhold store
6. Update documentation and skills

### Phase 4: Cleanup
1. Remove parallel storage code
2. Remove BadgerDB config from TOML files
3. Update Docker compose (no volume mount for data/)
4. Update run.sh (no data/ copy to bin/)

## Config Changes

### Before (portal config)
```toml
[storage.badger]
path = "./data/vire"

[import]
users = true
users_file = "data/users.json"
```

### After (portal config)
```toml
[api]
url = "http://localhost:4242"
# All user data managed by vire-server
```

### Server config additions
```toml
[storage.users]
path = "data/users"

[auth]
jwt_secret = "..."
token_expiry = "24h"
```

## Risks and Considerations

- **Offline mode:** Portal cannot function without vire-server (currently can serve pages with local user data). Acceptable trade-off since MCP tools already require vire-server.
- **Latency:** Every user lookup becomes an HTTP call instead of local read. Mitigate with short-lived in-memory cache in portal (cache user context for duration of request).
- **Secret handling:** Navexa key never leaves vire-server if using Option B. Currently it transits through portal as an HTTP header.
- **Dev mode:** Dev login still needs to work without real auth. Server should support a dev auth endpoint that mirrors portal's current `buildDevJWT()`.
- **Import:** User import moves to server. Portal startup no longer seeds users — either the server does it or users are imported via API call.
