# Requirements: Dashboard net return fields and total cost

**Date:** 2026-02-22
**Requested:** Three dashboard changes:
1. Portfolio summary: use `total_net_return` / `total_net_return_pct` for gain totals (currently blank with `total_gain`)
2. Portfolio summary: add TOTAL COST field using `total_cost` from API
3. Holdings table: show `net_return` ($ with color) and `net_return_pct` (% with color) per ticker

## Scope
- In scope: JS data binding, HTML template, Go model fields, UI tests
- Out of scope: Backend/vire-server changes (API already returns these fields)

## Approach

### Change 1: Fix portfolio summary gain fields
**File: `pages/static/common.js`** lines 185-186, 249-250
- Change `portfolioGain` to read `holdingsData.total_net_return` instead of `holdingsData.total_gain`
- Change `portfolioGainPct` to read `holdingsData.total_net_return_pct` instead of `holdingsData.total_gain_pct`

### Change 2: Add TOTAL COST to summary
**File: `pages/static/common.js`** — add `portfolioCost: 0` state, `totalCost` getter, capture in `loadPortfolio()`
**File: `pages/dashboard.html`** lines 48-61 — add 4th summary item between TOTAL VALUE and TOTAL GAIN $:
```html
<div class="portfolio-summary-item">
    <span class="label">TOTAL COST</span>
    <span class="text-bold" x-text="fmt(totalCost)"></span>
</div>
```

### Change 3: Holdings table net_return columns
**File: `pages/dashboard.html`** lines 74-91 — replace single "Gain%" column with two columns:
- Header: `<th class="text-right">Gain $</th>` and `<th class="text-right">Gain %</th>`
- Body: `fmt(h.net_return)` with `gainClass(h.net_return)` and `pct(h.net_return_pct)` with `gainClass(h.net_return_pct)`

### Change 4: Go model update
**File: `internal/vire/models/portfolio.go`**
- Portfolio: add `TotalNetReturn float64 json:"total_net_return"` and `TotalNetReturnPct float64 json:"total_net_return_pct"`
- Holding: add `NetReturn float64 json:"net_return"` and `NetReturnPct float64 json:"net_return_pct"`

### Change 5: Update tests
**File: `tests/ui/dashboard_test.go`**
- Update summary item count from 3 → 4
- Update expected labels: ['TOTAL VALUE', 'TOTAL COST', 'TOTAL GAIN $', 'TOTAL GAIN %']
- Update gain color check indices (items 2 and 3 now, not 1 and 2)
- Update column header checks for new Gain $ / Gain % headers

## Files Expected to Change
- `pages/dashboard.html`
- `pages/static/common.js`
- `internal/vire/models/portfolio.go`
- `tests/ui/dashboard_test.go`
