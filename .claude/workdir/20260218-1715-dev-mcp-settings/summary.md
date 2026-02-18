# Summary: Dev MCP Settings Section

**Date:** 2026-02-18
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `internal/handlers/settings.go` | Added `devMCPEndpoint` field and `SetDevMCPEndpointFn` method |
| `pages/settings.html` | Added DEV MCP ENDPOINT section (shown in dev mode) |
| `internal/app/app.go` | Wired up `SetDevMCPEndpointFn` for SettingsHandler |
| `tests/common/browser.go` | Fixed `LoginAndNavigate` to actually navigate to target URL |
| `tests/ui/dev_auth_test.go` | Added `TestDevAuthSettingsMCPEndpoint` and `TestDevAuthSettingsMCPURL` tests |
| `bin/vire-portal.toml` | Set JWT secret for MCP encryption |
| `/home/bobmc/development/vire/bin/vire-service.toml` | Set matching JWT secret for vire-server |

## Tests Added

- `TestDevAuthSettingsMCPEndpoint` - Verifies DEV MCP section appears in settings
- `TestDevAuthSettingsMCPURL` - Validates the MCP URL format and encryption

## Test Results

All 6 dev auth tests pass:
- TestDevAuthLandingNoCookie
- TestDevAuthLoginRedirect
- TestDevAuthCookieAndJWT
- TestDevAuthLogout
- TestDevAuthSettingsMCPEndpoint (new)
- TestDevAuthSettingsMCPURL (new)

## Notes

- The DEV MCP ENDPOINT section only shows when:
  1. Server is in dev mode
  2. User is logged in
  3. JWT secret is configured (for encryption)
- The MCP URL format is: `http://localhost:8500/mcp/{encrypted_uid}`
- Fixed a bug in `LoginAndNavigate` that wasn't navigating to the target URL
