# Summary: Multi-Feature Implementation

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/stock.html` | Removed Filings Timeline section, reversed key events order |
| `pages/dashboard.html` | Added BUG compliance section (admin-only), UserRoleJSON in SSR data |
| `pages/profile.html` | Added timezone input with datalist |
| `pages/static/common.js` | Walk chart rewrite (close_price vs breakeven_price with fill), BUG getters, isAdmin, timezone auto-detect, removed expandedFilings/toggleFiling |
| `pages/static/css/portal.css` | BUG compliance styles, breadth arrow font-size 0.6rem→0.75rem |
| `internal/client/vire_client.go` | Added Timezone field to UserProfile |
| `internal/handlers/dashboard.go` | UserRoleJSON via json.Marshal + template.JS |
| `internal/handlers/profile.go` | Timezone save (conditional), UserTimezone template data |
| `internal/handlers/handlers_test.go` | Updated empty-fields test for conditional save |
| `internal/handlers/dashboard_stress_test.go` | 11 new stress tests (XSS, BUG guard, walk chart, etc.) |
| `internal/handlers/profile_stress_test.go` | New file, 7 timezone/profile stress tests |
| `tests/ui/stock_test.go` | FilingsSection → FilingsSectionRemoved |
| `tests/ui/dashboard_test.go` | 4 new subtests (UserRole SSR, BUG section, admin guard, title) |
| `tests/ui/profile_test.go` | 1 new subtest (TimezoneFieldVisible) |

## Tests
- Unit tests: 289 pass, 0 fail
- UI tests: 143 pass, 37 skip (expected), 1 pre-existing env failure (BreadthArrow)
- Stress tests: 18 new (11 dashboard, 7 profile), all pass
- Fix rounds: 1 (XSS fix for userRole in JS context)

## Architecture
- Architect APPROVED, no issues
- All patterns follow existing conventions (SSR hydration, template.JS, Alpine getters)

## Devils-Advocate
- 5 security areas analyzed
- Critical finding: userRole XSS in JS context — fixed by team lead (template.JSEscapeString → json.Marshal)
- Acceptable risks: client-side BUG visibility (cosmetic only), no timezone validation (unbounded IANA list)
- 18 stress tests cover all security vectors

## Notes
- Walk chart has fallback to old P&L chart when `close_price` not available in positionTimeline
- BUG findings are still sent to all users in SSR JSON — admin check is client-side visibility only
- Timezone auto-detect uses sessionStorage to prevent redundant POSTs
