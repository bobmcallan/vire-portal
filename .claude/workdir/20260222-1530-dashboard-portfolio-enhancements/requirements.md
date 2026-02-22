# Requirements: Dashboard Portfolio Enhancements

**Date:** 2026-02-22
**Requested:** 6 enhancements to the dashboard portfolio holdings UI

## Scope
- In scope: Dashboard page (pages/dashboard.html), Alpine.js component (pages/static/common.js), CSS (pages/static/css/portal.css), UI tests (tests/ui/dashboard_test.go)
- Out of scope: Backend API changes, other pages

## Approach

### 1. Default portfolio checkbox reflects setting
The checkbox already uses `:checked="isDefault"` with getter `this.selected === this.defaultPortfolio`. The `init()` sets `defaultPortfolio = data.default || ''`. This should work correctly. Verify and ensure the binding is solid.

### 2. Show closed positions checkbox
- Add `showClosed: false` to Alpine data
- Add checkbox above HOLDINGS table: "Show closed positions" (unchecked by default)
- Filter holdings: when `showClosed === false`, exclude items where `market_value === 0`
- Use computed `filteredHoldings` getter for the template

### 3. Value/weight & gain columns
Already present (Value, Weight%, Gain%). Ensure column headers and data bindings match: `market_value`, `weight`, `total_gain_pct` (aliased as `total_return_pct` in API).

### 4. Sort by ticker symbol
Sort `filteredHoldings` alphabetically by `ticker`.

### 5. Gain color coding
- Soft red for negative `total_gain_pct` (or `total_return_pct`)
- Soft green for positive
- No color for 0
- CSS classes: `.gain-positive { color: #2d8a4e; }`, `.gain-negative { color: #a33; }`
- Apply via `:class` binding using a `gainClass(val)` helper

### 6. Portfolio summary above table
- Display above the HOLDINGS table: total portfolio value, total $ gain, total % gain
- Color-coded same as individual rows
- Computed from holdings data in Alpine component

## Files Expected to Change
- `pages/dashboard.html` — template changes
- `pages/static/common.js` — Alpine.js component logic
- `pages/static/css/portal.css` — gain color classes, summary styles
- `tests/ui/dashboard_test.go` — update tests for new UI elements
