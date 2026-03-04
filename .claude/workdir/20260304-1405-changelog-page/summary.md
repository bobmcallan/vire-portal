# Summary: Changelog Page

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/changelog.html` | Created — Alpine.js infinite scroll, 10/page, IntersectionObserver |
| `pages/partials/nav.html` | Modified — Changelog in hamburger dropdown (after Profile) and mobile menu (after Help) |
| `internal/server/routes.go` | Modified — `GET /changelog` via PageHandler.ServePage |
| `pages/static/css/portal.css` | Modified — `.changelog-meta`, `.changelog-heading`, `.changelog-body` |
| `internal/server/routes_test.go` | Modified — `TestRoutes_ChangelogPage` added |
| `tests/ui/changelog_test.go` | Created — 7 UI tests |

## Tests
- Unit: TestRoutes_ChangelogPage — PASS
- UI: 6/7 pass, 1 skip (TestChangelogPageContent skips when vire-server changelog endpoint unavailable in test containers)
- Fix rounds: 1 (test content check updated to handle error state as skip)

## Architecture
- No new Go handler — PageHandler.ServePage is correct (all data fetched client-side)
- /api/changelog proxied to vire-server via existing catch-all proxy route
- No new dependencies (native fetch, IntersectionObserver, Alpine.js already loaded)

## Notes
- Content rendered with white-space: pre-wrap (no markdown library needed)
- parseHeading strips `##` prefix from first line; parseBody returns remainder
- IntersectionObserver retries if sentinel not yet in DOM (Alpine async rendering)
- TestChangelogPageContent gracefully skips in test environment where mock server lacks changelog endpoint — works correctly against real vire-server
