# Requirements: Dashboard total gain from portfolio feed + column alignment

**Date:** 2026-02-22
**Requested:** Two dashboard fixes:
1. Total gain $ and % should come from portfolio-level API data, not computed from holdings
2. Right-align Value, Weight, Gain column headings to match their numeric content

## Scope
- In scope: JS data binding changes, CSS specificity fix, UI test updates
- Out of scope: Backend changes (API already returns portfolio-level totals)

## Approach

### Task 1: Total gain from portfolio feed
The `/api/portfolios/{name}` response already includes `total_gain` and `total_gain_pct` fields from the Portfolio model. Currently the JS computes totals by summing individual holdings — this is incorrect (e.g. percentage doesn't sum meaningfully).

**Fix in `pages/static/common.js`:**
- Add `portfolioGain: 0` and `portfolioGainPct: 0` state properties
- In `loadPortfolio()`, capture `holdingsData.total_gain` and `holdingsData.total_gain_pct`
- Change `totalGain` getter to return `this.portfolioGain`
- Change `totalGainPct` getter to return `this.portfolioGainPct`

### Task 2: Column heading alignment
`.tool-table th { text-align: left; }` (specificity 0,1,1) overrides `.text-right { text-align: right; }` (specificity 0,1,0).

**Fix in `pages/static/css/portal.css`:**
- Add `.tool-table .text-right { text-align: right; }` after the `.tool-table th` rule (specificity 0,2,0)

## Files Expected to Change
- `pages/static/common.js` — lines 177-204, 244-249
- `pages/static/css/portal.css` — after line 489
- `tests/ui/dashboard_test.go` — update tests to verify alignment and portfolio-level totals
