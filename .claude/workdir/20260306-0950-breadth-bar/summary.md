# Summary: Portfolio Breadth Bar UI Component

**Status:** completed
**Feedback:** fb_3a0385ca

## Changes
| File | Change |
|------|--------|
| pages/static/common.js | Added breadth/hasBreadth properties, trendArrow(), trendArrowClass(), holdingTodayChange(), fmtTodayChange(), computeBreadth() helpers. Breadth parsing in loadPortfolio()/refreshPortfolio(). |
| pages/dashboard.html | Added breadth-bar-section above holdings table (trend label, gradient bar, counts). Modified movement sub-row: arrow col 1, trend_label col 2, today $ change col 6. |
| pages/static/css/portal.css | New classes: .breadth-bar-section, .breadth-summary-row, .breadth-trend, .breadth-bar, .breadth-segment, .breadth-rising, .breadth-flat, .breadth-falling, .breadth-counts |
| internal/handlers/dashboard_stress_test.go | 5 new stress tests: breadth style bindings, cloak, no XHTML, trend arrow safety, concurrent serve |
| tests/ui/dashboard_test.go | 2 new UI tests: TestDashboardBreadthBar, TestDashboardHoldingTrendArrows. Fixed TestDashboardShowClosedCheckbox visibility check. |
| scripts/ui-test.sh | Increased TIMEOUT from 120s to 300s (was causing late-test timeouts) |

## Tests
- Unit/stress tests: all pass (go test ./..., go vet clean)
- UI tests: 59 pass, 0 fail, 18 skip
- Breadth bar tests: skip gracefully (no holdings in test env) — structure verified via stress tests
- Fix rounds: 1 (fmtTodayChange negative formatting bug found by devils-advocate, fixed by architect)

## Architecture
- Architect approved: data flow, CSS classes, HTML structure all follow established patterns
- Client-side breadth computation as fallback when server doesn't provide breadth object
- No backend changes required

## Devils-Advocate
- Found fmtTodayChange formatting bug: `-$5K` vs correct `-$5.0K` — fixed
- Added 5 adversarial stress tests: XSS binding safety, x-cloak presence, no HTML entities in arrows, concurrent dashboard serve
- No security issues found

## Notes
- Server v0.3.172+ may include server-side breadth object; client falls back to computing from holdings
- TestDashboardShowClosedCheckbox was timing out due to elementCount finding hidden checkbox in DOM; fixed with getBoundingClientRect visibility check
- ui-test.sh TIMEOUT increased 120s→300s; 73 chromedp tests exceed 2min sequentially
