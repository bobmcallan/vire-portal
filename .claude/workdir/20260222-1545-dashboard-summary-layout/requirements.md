# Requirements: Dashboard Summary Layout + Gain Test Validation

**Date:** 2026-02-22
**Requested:**
1. Portfolio summary totals should span across the full content width
2. Validate gain display (totals + table) is shown and color-coded — add to UI tests

## Scope
- CSS: `.portfolio-summary` layout to span full width
- Tests: Strengthen gain validation in `TestDashboardGainColors` and `TestDashboardPortfolioSummary`
- Out of scope: Backend, JS logic (already correct)

## Approach
1. Add `justify-content: space-between` to `.portfolio-summary` + border separator
2. Rewrite `TestDashboardGainColors` to check both table and summary gain classes
3. Update `TestDashboardPortfolioSummary` to verify summary spans full width and values are populated
4. Execute all UI tests and verify output

## Files Expected to Change
- `pages/static/css/portal.css` (line 956-961)
- `tests/ui/dashboard_test.go` (lines 168-257)
