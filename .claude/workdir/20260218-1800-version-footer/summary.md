# Summary: Version in Footer

**Date:** 2026-02-18
**Status:** Completed

## What Changed

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

## Tests
- All 9 smoke tests pass (including new `TestSmokeFooterVersionDisplay`)
- Unit tests for `GetServerVersion` cover: network errors, HTTP errors, malformed JSON, missing fields, empty URL, timeout
- Stress tests cover: concurrent requests, large response body, hostile input values, redirects, slow server

## UI Verification

The footer now displays:
```
VIRE | Portal: 0.2.23 | Server: 0.3.31 | GitHub
```

When vire-server is unavailable, it shows:
```
VIRE | Portal: 0.2.23 | Server: unavailable | GitHub
```

## Notes
- The server version is fetched on each page load with a 2-second timeout
- If vire-server is slow or unavailable, "unavailable" is displayed gracefully
- The `GetServerVersion` function follows redirects (Go's default HTTP client behavior) - apiURL should never be user-controlled
