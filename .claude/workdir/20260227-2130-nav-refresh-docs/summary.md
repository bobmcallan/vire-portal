# Summary: Nav Title, Refresh Button, Docs Page

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/partials/nav.html` | VIRE brand link → `/dashboard`, added Docs nav item (desktop, dropdown, mobile) |
| `pages/dashboard.html` | Refresh button `style="margin-left:auto"` (right-aligned) |
| `pages/docs.html` | NEW — Docs page with 4 sections: What is Vire, Navexa, Getting Your API Key, Configuring Vire |
| `internal/server/routes.go` | Added `GET /docs` route via `PageHandler.ServePage("docs.html", "docs")` |
| `internal/handlers/docs_stress_test.go` | NEW — 25 stress tests for docs page security |
| `tests/ui/docs_test.go` | NEW — 7 UI tests for docs page |
| `tests/ui/nav_test.go` | Added 4 tests: brand href, docs link (desktop, mobile, dropdown) |
| `tests/ui/dashboard_test.go` | Updated refresh button test with alignment check |
| `README.md` | Added /docs to routes table |

## Tests
- 25 stress tests added (all pass)
- 11 UI tests added (nav brand, docs page, refresh button)
- UI test execution: 10 PASS, 11 FAIL, 17 SKIP — all failures are pre-existing dev auth blocker (unrelated)
- go test, go vet clean

## Architecture
- No new handler — uses existing `PageHandler.ServePage()` pattern
- Architect signed off: all clear, no issues

## Devils-Advocate
- No security issues found
- Nav brand change safe (nav only renders when logged in)
- Docs page pageName "docs" doesn't trigger auto-logout
- No XSS vectors, no template collisions, no route conflicts

## Notes
- Pre-existing dev auth login issue blocks 11 UI tests — not related to this feature
- Server running on http://localhost:8883, health endpoint responding
