# Requirements: Version in Footer

**Date:** 2026-02-18
**Requested:** Add the version number for portal and server to the footer.

## Scope
- In scope:
  - Display portal version in footer
  - Display server version in footer
  - Handle server unavailable gracefully
- Out of scope:
  - Version comparison or update checking
  - Detailed build info (just version numbers)

## Approach

1. **Create server version fetcher** in `internal/handlers/` that calls vire-server's `/api/version` endpoint with a short timeout (2s). Returns version string or "unavailable" on error.

2. **Add version fields to template data** in all page handlers (landing, dashboard, settings):
   - `PortalVersion` - from `config.GetVersion()`
   - `ServerVersion` - from the new fetcher

3. **Update footer template** (`pages/partials/footer.html`) to display:
   - Portal version (always available)
   - Server version (may show "unavailable")

4. **Add UI test** to verify versions appear in footer.

## Files Expected to Change
- `internal/handlers/version.go` - add server version fetcher
- `internal/handlers/landing.go` - add version fields to data
- `internal/handlers/dashboard.go` - add version fields to data
- `internal/handlers/settings.go` - add version fields to data
- `pages/partials/footer.html` - display versions
- `tests/ui/smoke_test.go` or new test - verify footer versions
