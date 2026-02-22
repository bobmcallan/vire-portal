# Summary: Dashboard total gain from portfolio feed + column alignment

**Date:** 2026-02-22
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `pages/static/common.js` | Total gain $ and % now use portfolio-level `total_gain`/`total_gain_pct` from API response instead of summing from individual holdings |
| `pages/static/css/portal.css` | Added `.tool-table .text-right` rule to fix CSS specificity override on column headers |
| `tests/ui/dashboard_test.go` | Added `TestDashboardColumnAlignment` test and summary label verification in `TestDashboardPortfolioSummary` |

## Tests
- All 14 dashboard UI tests pass (including 2 new tests)
- `go test ./...` passes
- `go vet ./...` clean

## Documentation Updated
- No documentation changes needed (minor frontend fix)

## Devils-Advocate Findings
- No issues raised — changes are safe (values use `Number() || 0` for null/missing API fields, state resets on portfolio switch and error paths)

## Notes
- The CSS specificity issue was caused by `.tool-table th` (specificity 0,1,1) overriding `.text-right` (specificity 0,1,0). Fixed with `.tool-table .text-right` (specificity 0,2,0).
- Portfolio-level `total_gain_pct` is more accurate than summing individual holding percentages (which is mathematically incorrect for portfolio-weighted returns).
