# Summary: Remove dashboard page title from nav

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `pages/partials/nav.html` | Removed `<span class="nav-title">{{.PageTitle}}</span>` |
| `pages/static/css/portal.css` | Removed `.nav-title` CSS class and mobile media query rule |
| `pages/static/css/portal.css` | Removed unused `.page-title`, `.dashboard-title`, `.dashboard-section-title` CSS classes |
| `internal/handlers/dashboard.go` | Removed `PageTitle` from template data |
| `internal/handlers/settings.go` | Removed `PageTitle` from template data |
| `internal/handlers/landing.go` | Removed `PageTitle` from template data |
| `internal/handlers/handlers_test.go` | Renamed PageTitle tests to PageIdentifier, updated assertions |

## Tests
- `TestDashboardHandler_PageIdentifier` — PASS
- `TestSettingsHandler_PageIdentifier` — PASS
- `go vet ./...` — clean
- browser-check dashboard: `.nav-title|gone` passed, 0 JS errors
- browser-check landing: smoke test passed, 0 JS errors

## Notes
- The nav menu's active link already indicates the current page, making the centered title redundant
- Browser `<title>` tags retained (VIRE DASHBOARD, VIRE SETTINGS) for tab identification
