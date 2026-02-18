# Summary: Version Footer + Test Authentication Fixes

**Date:** 2026-02-18
**Status:** Completed

## Phase 1: Version in Footer

| File | Change |
|------|--------|
| `internal/handlers/version.go` | Added `GetServerVersion()` function to fetch version from vire-server |
| `internal/handlers/landing.go` | Added `PortalVersion` and `ServerVersion` to template data |
| `internal/handlers/dashboard.go` | Added `PortalVersion` and `ServerVersion` to template data |
| `internal/handlers/settings.go` | Added `PortalVersion` and `ServerVersion` to template data |
| `pages/partials/footer.html` | Added version display line: `Portal: x.x.x | Server: x.x.x` |
| `tests/ui/smoke_test.go` | Added `TestSmokeFooterVersionDisplay` to verify footer versions |
| `internal/handlers/handlers_test.go` | Added unit tests for `GetServerVersion()` |
| `internal/handlers/auth_stress_test.go` | Added stress tests for `GetServerVersion()` |

## Phase 2: Test Authentication Fixes

After adding authentication requirements to dashboard/settings pages, many tests broke because they were accessing these pages without authentication. Fixed by:

| File | Change |
|------|--------|
| `internal/server/routes_test.go` | Added `createTestJWT()` helper, updated all dashboard/settings tests to include auth cookies |
| `internal/handlers/handlers_test.go` | Added `createTestJWT()` and `addAuthCookie()` helpers, updated ~30 tests to use proper authentication |

### Key Changes:
1. Dashboard and settings handlers now redirect unauthenticated users (302 to `/`)
2. Tests updated to include valid JWT session cookies
3. Nav template tests updated to use DashboardHandler (landing page auto-logouts)
4. Settings page test for unauthenticated access now expects 302 redirect

### MCP Address Behavior Confirmed:
- **HTTP streaming only** (uses `mcpserver.NewStreamableHTTPServer`, no stdio support)
- **Dynamic URLs**: Each page load generates a new MCP endpoint URL with different encrypted UID (random nonce in AES-GCM encryption)
- Tests verify MCP URL format: `http://localhost:8500/mcp/{base64url-encrypted-uid}`

## Tests
- All unit tests pass (`go test ./...`)
- All handler tests pass (`go test ./internal/handlers/...`)
- All server tests pass (`go test ./internal/server/...`)
- All smoke tests pass (`go test -v ./tests/ui -run TestSmoke`)
- All dev auth tests pass (`go test -v ./tests/ui -run TestDevAuth`)
- Go vet clean (`go vet ./...`)

## UI Verification

Footer displays:
```
VIRE | Portal: 0.2.23 | Server: 0.3.31 | GitHub
```

Dev MCP settings section shows:
```
DEV MCP ENDPOINT

Use this URL to connect Claude Desktop to your Vire instance:

http://localhost:8500/mcp/{encrypted-uid}
```

## Notes
- Landing page auto-logouts users (clears session cookie, sets `LoggedIn: false`)
- Dashboard and settings require authentication (redirect to `/` if not logged in)
- The server version is fetched on each page load with a 2-second timeout
- MCP endpoint URL changes on each page load due to random nonce in encryption
