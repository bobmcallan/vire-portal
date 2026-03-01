# Summary: Portfolio Value Field Changes — Dashboard Updates

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/vire/models/portfolio.go` | Added `AvailableCash`, `CapitalGain`, `CapitalGainPct` to Portfolio; `TotalProceeds` to Holding |
| `pages/static/common.js` | Added `availableCash`, `capitalGainPct` properties; parse `available_cash`, `capital_gain`, `capital_gain_pct` from server; removed 3 client-side computations; removed ExternalBalance from chart; simplified `totalGainPct` getter |
| `pages/dashboard.html` | Summary row 1: 4→5 items (renamed COST BASIS→NET EQUITY CAPITAL, added AVAILABLE CASH). Capital row: 4→5 items (added CAPITAL GAIN %) |
| `internal/handlers/dashboard_stress_test.go` | Updated expected summary labels (5 items) and capital labels (5 items) |
| `tests/ui/dashboard_test.go` | Updated expected counts, labels, and index references in TestDashboardPortfolioSummary, TestDashboardCapitalPerformance, TestDashboardGainColors |

## Tests
- Handler tests: ALL PASS (including stress tests)
- UI tests: 23 PASS, 0 FAIL, 10 SKIP (data-dependent), 1 TIMEOUT (known dev auth issue)
- Fix rounds: 0

## Architecture
- Architect review: APPROVED — patterns consistent, no new endpoints, data flow correct
- No breaking changes to existing contracts

## Devils-Advocate
- Edge cases verified: negative available_cash, missing fields (omitempty), old server compatibility
- XSS safety: all bindings use x-text (safe)
- No NaN/Infinity possible from Number()||0 pattern

## Reviews
- Code quality: APPROVED — zero issues
- Documentation: APPROVED — all labels and field mappings match implementation

## Notes
- Client-side computations replaced with server-provided values (capitalGain, capitalGainPct, simpleReturnPct)
- ExternalBalance deprecated field cleaned up from growth chart
- Dashboard summary rows now show 5 items each (was 4)
