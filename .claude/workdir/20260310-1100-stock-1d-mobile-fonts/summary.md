# Summary: Stock 1D Change Column + Mobile Font Sizing

**Status:** completed

## Changes
| File | Change |
|------|--------|
| pages/dashboard.html | Added 1D column header, data cell with holdingDailyPct/changePct/changeClass, colspan 6→7, footer empty td |
| pages/mobile.html | Added mobile-holding-1d span with 1D change % in holding cards |
| pages/static/css/portal.css | Scoped breadth segment backgrounds, bumped all mobile fonts to 1rem+, added mobile-holding-1d/label/perf-item styles |
| internal/handlers/dashboard_stress_test.go | 16 new stress tests (6 implementer + 10 devils-advocate) |
| tests/ui/dashboard_test.go | Fixed stale GainColors selector, added OneDayColumn UI test |
| tests/ui/mobile_dashboard_test.go | Added HoldingOneDayChange UI test |

## Tests
- 561 unit/stress tests: **all pass**
- 16 new stress tests added
- 2 new UI tests added
- 1 stale selector fixed
- UI tests (Docker): skipped

## Architecture
- Architect: APPROVED — all 8 rules verified

## Devils-Advocate
- 10 adversarial tests — XSS, null guards, colspan consistency, CSS specificity, font scope
- No issues found

## Notes
- Breadth segment CSS scoped to `.breadth-segment.breadth-*` to prevent background color leak to holding tbody rows
- No Go handler changes needed — purely template + CSS
