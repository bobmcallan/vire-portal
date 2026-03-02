# Summary: Dashboard 3-Row Summary Layout

**Status:** completed
**Feedback:** fb_d88928ac

## Changes
| File | Change |
|------|--------|
| `pages/static/css/portal.css` | Renamed `.portfolio-summary-capital` → `.portfolio-summary-equity`, added `.portfolio-summary-cash` |
| `pages/static/common.js` | Added `grossContributions` and `totalDividends` properties, parsing in `loadPortfolio()` and `refreshPortfolio()` |
| `pages/dashboard.html` | Reorganized from 2-row to 3-row layout (Portfolio, Cash, Equity) |
| `tests/ui/dashboard_test.go` | Updated `TestDashboardPortfolioSummary` and `TestDashboardCapitalPerformance` for 3-row selectors/labels |
| `internal/handlers/dashboard_stress_test.go` | Updated label arrays, added binding checks for `grossContributions` and `totalDividends` |

## Layout
- **Row 1 (Portfolio):** TOTAL VALUE, CAPITAL RETURN $, CAPITAL RETURN %, SIMPLE RETURN %, ANNUALIZED %
- **Row 2 (Cash):** GROSS CASH BALANCE, AVAILABLE CASH, GROSS CONTRIBUTIONS, DIVIDENDS
- **Row 3 (Equity):** NET EQUITY CAPITAL, NET RETURN $, NET RETURN %

## Tests
- UI tests: 24 pass, 0 fail, 10 skipped (expected — no portfolio data in test env)
- Unit tests: pass (timeout on unrelated admin test — pre-existing)
- go vet: clean
- Fix rounds: 0

## Architecture
- Architect review: APPROVED — follows established patterns
- No new API calls — all data from existing `/api/portfolios/{name}` response

## Devils-Advocate
- No critical issues found
- All bindings use x-text (safe from XSS)
- Edge cases covered (null/zero/empty arrays)

## Notes
- Server running at http://localhost:8883, health check OK
- No README changes needed (internal UI layout change)
