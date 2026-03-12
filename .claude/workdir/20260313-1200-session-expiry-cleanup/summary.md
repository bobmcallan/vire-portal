# Summary: Session Expiry Timer & Client-Side Cleanup

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/partials/head.html` | Added `window.__VIRE_TOKEN_EXPIRY__` conditional script tag |
| `pages/partials/nav.html` | Added session expiry warning banner with Alpine `sessionExpiry()` component |
| `pages/static/common.js` | Added `sessionExpiry()` Alpine component; removed 4 pass-through functions (`holdingTodayChange`, `holdingBreadthClass`, `holdingBreadthArrow`, `holdingDailyPct`) |
| `pages/static/css/portal.css` | Added `.session-expiry-banner` and `.session-expiry-link` styles |
| `pages/dashboard.html` | Inlined 3 template references replacing removed functions |
| `pages/mobile.html` | Inlined 1 template reference replacing removed function |
| `internal/handlers/dashboard.go` | Added `TokenExpiry: claims.Exp` to template data |
| `internal/handlers/mobile_dashboard.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/stock.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/strategy.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/cash.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/profile.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/users.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/gemini.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/mcp_page.go` | Added `TokenExpiry: claims.Exp` |
| `internal/handlers/prompts.go` | Added `tokenExpiry int64` param to `serveList`/`serveEdit`, propagated from claims |
| `internal/handlers/landing.go` | Added nil-safe `TokenExpiry` to 5 public handlers |
| `internal/server/routes_test.go` | Updated XSS test to account for new script tag |
| `internal/handlers/dashboard_stress_test.go` | Updated inline script count for new script tag |
| `internal/handlers/session_expiry_stress_test.go` | New: 18 stress tests for session expiry |

## Tests
- Unit tests: 18 new stress tests in `session_expiry_stress_test.go`
- Updated: `routes_test.go`, `dashboard_stress_test.go`
- Results: handlers PASS (174s), server PASS (29s), build clean, vet clean
- Fix rounds: 1 (setInterval leak in destroy())

## Architecture
- Architect: APPROVED, all 8 rules pass

## Devils-Advocate
- 18 stress tests covering XSS, CSRF, timer bypass, race conditions, removed function cleanup
- One finding: setInterval not cleared in destroy() — fixed (added `_intervalId` tracking)

## Notes
- Implementer agent edits did not persist; team lead re-implemented directly
- 4 of 8 fb_24b3388d functions were already removed in prior work; remaining 4 removed here
- `holdingDailyPct` was dead code (never referenced in templates)
- Public pages use nil-safe TokenExpiry pattern; authenticated pages use claims.Exp directly
