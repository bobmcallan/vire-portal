# Summary: Dashboard SSR Migration

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/handlers/dashboard.go` | Added `proxyGetFn` field, `SetProxyGetFn` method, SSR fetch logic for 5 endpoints (portfolios, portfolio, timeline, watchlist, glossary) |
| `internal/app/app.go` | Wired `DashboardHandler.SetProxyGetFn(vireClient.ProxyGet)` |
| `pages/dashboard.html` | Added `window.__VIRE_DATA__` hydration script block |
| `pages/static/common.js` | Added `_applyPortfolioData()`, `_applyTimelineData()` helpers; SSR hydration path in `init()`; refactored `loadPortfolio()` and `refreshPortfolio()` to use helpers |
| `internal/handlers/handlers_test.go` | 5 unit tests (embed JSON, nil proxyGet, partial failure, portfolios failure, no default) |
| `internal/handlers/dashboard_stress_test.go` | 4 stress tests (concurrent, large timeline, XSS, timeout) + 9 devils-advocate tests + 1 pre-existing fix |
| `internal/handlers/routes_test.go` | Fixed pre-existing test (allow SSR script block) |
| `tests/ui/dashboard_test.go` | 3 new SSR UI tests + 2 stale selector fixes |

## Tests
- Unit tests: all pass (`go test ./internal/...`)
- Stress tests: all pass (including devils-advocate's 9 additional tests)
- UI tests: 68 pass, 2 fail (pre-existing, unrelated), 17 skip (expected)
- New SSR UI tests: 1 pass (NoLoadingSpinner), 2 skip (DataPreRendered, VireDataCleared — test account lacks portfolio data)
- Fix rounds: 1 (SSR UI tests adjusted for test environment)

## Architecture
- Architect APPROVED: all 7 architecture rules pass
- Pattern matches strategy.go/cash.go SSR exactly
- No new dependencies, routes, or CSS changes

## Devils-Advocate
- Security APPROVED: no critical issues
- template.JS usage safe (trusted server-side JSON)
- Auth guards prevent data leak
- url.PathEscape handles special chars
- 9 additional stress tests added (path traversal, concurrent, XSS variants)

## Notes
- SSR only applies to initial page load with default portfolio
- Portfolio switching, refresh, closed positions remain client-side
- Fallback: if proxyGetFn nil or errors, Alpine falls back to client-side fetch (zero-change behavior)
- Pre-existing failures not addressed: TestDashboardHoldingTrendArrows (stale selector), TestGlossaryInHamburgerDropdown
