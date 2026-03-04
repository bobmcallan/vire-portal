# Summary: Dashboard Cleanup (5 Feedback Items)

**Status:** completed

## Feedback Items Addressed
- **fb_d94e3836** (HIGH): Removed Simple Return % and Annualized % — contradictory values
- **fb_d347cac5** (MEDIUM): Removed Capital Return $ and Capital Return % — redundant with Net Return
- **fb_d84df828** (MEDIUM): Already dismissed (superseded by fb_d347cac5)
- **fb_03cc83b1** (LOW): Removed portfolio-level Trend/RSI indicators — misleading at portfolio level
- **fb_cb126cf5** (LOW): Renamed "NET EQUITY CAPITAL" to "NET EQUITY"

## Changes
| File | Change |
|------|--------|
| pages/dashboard.html | Removed 4 capital return items (16 lines), removed indicators div (6 lines), renamed label |
| pages/static/common.js | Removed 8 properties, cleaned up loadPortfolio/refreshPortfolio parsing and error paths, removed 2 indicators fetch blocks |
| pages/static/css/portal.css | Removed .portfolio-indicators and .indicator-item styles |
| internal/handlers/dashboard_stress_test.go | Updated 4 test functions: labels, bindings, boundary check |
| tests/ui/dashboard_test.go | Updated 2 test functions, deleted TestDashboardIndicators |

## Dashboard After Cleanup
- **Row 1:** PORTFOLIO VALUE (with D/W/M)
- **Row 2:** GROSS CASH BALANCE (with D/W/M), AVAILABLE CASH, GROSS CONTRIBUTIONS*, DIVIDENDS*
- **Row 3:** NET EQUITY (renamed, with D/W/M), NET RETURN $, NET RETURN %
- **Indicators bar:** REMOVED

## Tests
- Stress tests updated (4 functions)
- UI tests updated (2 functions) + 1 deleted (TestDashboardIndicators)
- Build verified, server running

## Architecture
- Architect found 3 issues in stress tests — all fixed by implementer
- Devils-advocate found CSS text-align issue in Row 1 single-item layout — fixed
- Reviewer confirmed zero orphaned references

## Review Sign-offs
- Architect: APPROVED (Task #4)
- Reviewer: APPROVED (Task #6)
- Devils-advocate: APPROVED (Task #7)
