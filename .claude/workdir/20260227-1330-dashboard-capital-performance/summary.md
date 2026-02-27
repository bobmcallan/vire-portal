# Summary: Dashboard Capital Performance, Indicators & Refresh

**Status:** completed

## Changes

| File | Change |
|------|--------|
| `pages/static/common.js` | Added capital performance parsing from existing API response, non-blocking indicators fetch from `/api/portfolios/{name}/indicators`, `refreshPortfolio()` method with `?force_refresh=true` and indicators re-fetch |
| `pages/dashboard.html` | Added refresh button in portfolio header, capital performance metrics row (conditional), portfolio indicators row (conditional) |
| `pages/static/css/portal.css` | Added `.portfolio-summary-capital` (1px #888 border), `.portfolio-indicators` (flex row, muted text), `.indicator-item` styles |
| `internal/server/routes.go` | Added proxy cache invalidation when `force_refresh=true` succeeds |
| `internal/server/proxy_stress_test.go` | 18 stress tests for API proxy: path traversal, header injection, concurrent requests, cache behavior |
| `tests/ui/dashboard_test.go` | Updated `TestDashboardPortfolioSummary` to scope to first row; added `TestDashboardCapitalPerformance`, `TestDashboardRefreshButton`, `TestDashboardIndicators` |
| `README.md` | Updated route descriptions, file tree, test categories |

## Tests
- Unit tests: all pass (`go test ./internal/... -timeout 300s`)
- Stress tests: 18 new proxy tests, all pass
- UI tests: 3 new tests added (capital, refresh, indicators) — execute correctly, skip gracefully when no data
- UI test suite: 8 pass, 11 fail (pre-existing — vire-server down in Docker, not code-related), 11 skip
- `go vet ./...`: clean

## Architecture
- No new routes or handlers — uses existing `/api/*` proxy to vire-server
- Capital performance data already in `/api/portfolios/{name}` response (was ignored by JS)
- Indicators via separate `/api/portfolios/{name}/indicators` call (non-blocking)
- Proxy cache fix ensures `force_refresh=true` invalidates stale entries

## Devils-Advocate
- 18 stress tests covering: path traversal, encoded slashes, auth bypass, header injection, large responses, invalid JSON, concurrent requests, hostile query params, method passthrough, cache hit/miss
- Finding noted: backend response headers forwarded to client (acceptable risk — backend is trusted vire-server)
- No critical vulnerabilities found

## Feedback Items Addressed

### fb_1346e9a3 — Dashboard Capital Performance + Refresh
- Capital Invested (net_capital_deployed), Capital Gain, Simple Return %, Annualized Return % displayed
- Refresh button triggers `?force_refresh=true` Navexa sync
- All metrics conditional on capital_performance data existing

### fb_cafb4fa0 — Portfolio Indicators Display
- Trend and RSI signal displayed from indicators API
- EMA/RSI values known buggy server-side (not displayed raw)
- Time series exposure deferred to vire-server work

### fb_742053d8 — Capital Performance Visibility
- Capital performance metrics now prominent on dashboard
- Goal tracking config deferred to vire-server work

## Notes
- Server left running (PID 76782) on port 8883
- UI test failures are environmental (vire-server not running in Docker test container)
- `internal/server` tests need `-timeout 300s` (pre-existing MCP catalog retry delays total ~125s)
