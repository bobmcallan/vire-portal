# Summary: Add favicon

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `scripts/create-favicon.sh` | New script — generates 16x16 ICO with black bg, white V letter using pure bash printf |
| `pages/static/favicon.ico` | Generated favicon file |
| `pages/favicon.ico` | Copy for root serving |
| `pages/partials/head.html` | Added `<link rel="icon" href="/static/favicon.ico" type="image/x-icon">` |
| `tests/ui/ui_test.go` | Fixed `TestUIDashboardRenders` (stale `.dashboard-title` selector), added auth to nav tests |
| `tests/ui/ui_helpers_test.go` | Added `loginAndNavigate` helper for tests requiring authentication |

## Tests
- All 14 UI tests pass (4 skipped — features not present on page)
- All internal package tests pass (except pre-existing `TestDockerComposeProjectName`)
- `go vet ./...` clean
- Browser-check smoke tests: landing 1/1, dashboard 1/1

## Documentation Updated
- None required — favicon is a static asset, no API or behaviour change

## Notes
- Adapted from quaero's `create-favicon.sh` — changed blue background to black, Q letter to V
- No ImageMagick dependency — pure bash printf hex data
- Fixed pre-existing UI test failures: `TestUIDashboardRenders` referenced removed `.dashboard-title` class; nav tests (`TestUINavLinksPresent`, `TestUINavLinksHiddenOnMobile`, `TestUINavLinksVisibleOnDesktop`) needed dev auth before testing logged-in nav elements
