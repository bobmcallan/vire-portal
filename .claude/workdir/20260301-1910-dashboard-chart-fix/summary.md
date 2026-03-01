# Summary: Dashboard Chart Alignment — Capital Timeline

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/static/common.js` | `fetchGrowthData()` URL: `/history` → `/capital-timeline` |
| `pages/static/common.js` | `filterAnomalies()` field: `TotalValue` → `TotalCapital` |
| `pages/static/common.js` | `renderChart()` Portfolio Value: `p.TotalCapital \|\| p.TotalValue \|\| 0` |
| `pages/static/common.js` | `renderChart()` Capital Deployed: `p.NetDeployed \|\| this.capitalInvested \|\| 0` |
| `pages/static/common.js` | Dataset label: `'Capital Deployed'` → `'Net Deposited'`, pointHoverRadius: 4 |
| `tests/ui/dashboard_test.go` | Updated chart dataset label assertions to `'Net Deposited'` |

## Tests
- Unit tests: all pass (`go test ./internal/... ./cmd/...`)
- UI tests: 23 passed, 0 failed, 10 skipped (`./scripts/ui-test.sh all`)
- Fix rounds: 0

## Architecture
- Architect verified: transparent proxy pattern preserved, no Go struct changes needed
- Low risk: isolated JS data layer change

## Devils-Advocate
- All edge cases validated (404 fallback, missing fields, XSS safety)
- No blocking issues found

## Service Feedback
- Submitted fb_2d9bad2f: requesting net_deployed in growth history data points

## Notes
- Available Cash and Capital Gain % were already implemented in portal code; they display correctly after portal rebuild
- The capital-timeline endpoint provides historical NetDeployed (changes over time, not flat)
- Chart Portfolio Value now uses TotalCapital (equity + cash), aligning with dashboard TOTAL VALUE card
