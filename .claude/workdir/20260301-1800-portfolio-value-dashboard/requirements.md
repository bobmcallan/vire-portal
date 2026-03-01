# Requirements: Portfolio Value Field Changes — Dashboard Updates

## Server Context

The vire-server `get_portfolio` response has been updated (2026-03-01). Three new fields are
now present in the JSON response. The portal's API proxy (`/api/` routes) passes the server
response through unmodified, so the Go model struct changes are for internal consistency
only — the JavaScript consumes the proxied JSON directly.

Reference: `docs/features/20260301-portfolio-value-field-changes.md`

## Scope

### Does

1. Add `AvailableCash`, `CapitalGain`, `CapitalGainPct` fields to the Go `Portfolio` struct.
2. Add `TotalProceeds` field to the Go `Holding` struct.
3. Remove `ExternalBalance` from `GrowthDataPoint` struct (if present — check first).
4. Add `availableCash` and `capitalGainPct` data properties to the JS `portfolioDashboard()` component.
5. Parse `available_cash`, `capital_gain`, `capital_gain_pct` from the server response in both `loadPortfolio()` and `refreshPortfolio()`.
6. Use server-provided `capital_gain` directly (stop computing `this.totalValue - this.capitalInvested`).
7. Use server-provided `capital_gain_pct` directly.
8. Use server-provided `simple_return_pct` from `capital_performance` directly (stop computing `(this.capitalGain / this.capitalInvested) * 100`).
9. Remove the `totalGainPct` getter override — use `portfolioGainPct` (from `total_net_return_pct`) directly.
10. Remove `(p.ExternalBalance || 0)` from growth chart line (common.js line 364).
11. Rename "COST BASIS" label to "NET EQUITY CAPITAL" in dashboard summary row 1.
12. Add "AVAILABLE CASH" item to dashboard summary row 1 (between NET EQUITY CAPITAL and NET RETURN $).
13. Add "CAPITAL GAIN %" item to dashboard summary row 2 (between CAPITAL GAIN $ and SIMPLE RETURN %).
14. Update expected labels in stress test and UI test.

### Does NOT

- Change server-side calculations or API response shapes.
- Change the capital transactions page (`pages/capital.html`).
- Change `fmt()` or `pct()` formatting functions.
- Change the holdings table structure or per-row columns.
- Change the growth chart datasets (still 3 datasets: Portfolio Value, Cost Basis, Capital Deployed).
- Change portfolio indicators section.
- Change portfolio selector or refresh button logic.
- Add any new API endpoints or proxy logic.

## File Changes

### 1. `internal/vire/models/portfolio.go`

**Portfolio struct:** Add 3 new fields after `TotalNetReturnPct`:

```go
AvailableCash     float64   `json:"available_cash,omitempty"`
CapitalGain       float64   `json:"capital_gain,omitempty"`
CapitalGainPct    float64   `json:"capital_gain_pct,omitempty"`
```

Use `omitempty` on all three (they are absent when zero or when capital data is unavailable).

**Holding struct:** Add `TotalProceeds` after `TotalCost`:

```go
TotalProceeds      float64        `json:"total_proceeds,omitempty"`
```

**GrowthDataPoint struct:** Check if it contains `ExternalBalance`. If so, remove it. If not, no change needed.

### 2. `pages/static/common.js`

#### 2a. Add data properties (after `hasCapitalData: false`)

```javascript
capitalGainPct: 0,
availableCash: 0,
```

#### 2b. Remove `totalGainPct` getter override (lines 220-225)

Current:
```javascript
get totalGainPct() {
    if (this.hasCapitalData && this.capitalInvested !== 0) {
        return (this.portfolioGain / this.capitalInvested) * 100;
    }
    return this.portfolioGainPct;
},
```

Replace with:
```javascript
get totalGainPct() {
    return this.portfolioGainPct;
},
```

#### 2c. Update `loadPortfolio()` parsing (lines ~263-282)

After `this.portfolioCost = ...` add:
```javascript
this.availableCash = Number(holdingsData.available_cash) || 0;
```

In the capital performance `if` block, replace client-side computations:
```javascript
// OLD:
this.capitalGain = this.totalValue - this.capitalInvested;
this.simpleReturnPct = this.capitalInvested !== 0
    ? (this.capitalGain / this.capitalInvested) * 100 : 0;

// NEW:
this.capitalGain = Number(holdingsData.capital_gain) || 0;
this.capitalGainPct = Number(holdingsData.capital_gain_pct) || 0;
this.simpleReturnPct = Number(cp.simple_return_pct) || 0;
```

In the `else` branch, add resets:
```javascript
this.capitalGainPct = 0;
this.availableCash = 0;
```

#### 2d. Update `refreshPortfolio()` parsing (lines ~493-512)

Apply the exact same pattern as 2c. The refresh function has an identical parsing block.

#### 2e. Remove ExternalBalance from growth chart (line 364)

```javascript
// OLD:
const totalValues = this.growthData.map(p => p.TotalValue + (p.ExternalBalance || 0));
// NEW:
const totalValues = this.growthData.map(p => p.TotalValue);
```

### 3. `pages/dashboard.html`

#### 3a. Summary row 1: Change from 4 items to 5 items

New order: TOTAL VALUE | NET EQUITY CAPITAL | AVAILABLE CASH | NET RETURN $ | NET RETURN %

- "COST BASIS" → "NET EQUITY CAPITAL" (same binding: `fmt(totalCost)`)
- Insert new item after NET EQUITY CAPITAL:
```html
<div class="portfolio-summary-item">
    <span class="label">AVAILABLE CASH</span>
    <span class="text-bold" x-text="fmt(availableCash)"></span>
</div>
```
- AVAILABLE CASH does NOT use `gainClass()` — it is a neutral value.

#### 3b. Capital performance row: Change from 4 items to 5 items

New order: TOTAL DEPOSITED | CAPITAL GAIN $ | CAPITAL GAIN % | SIMPLE RETURN % | ANNUALIZED %

Insert new item after CAPITAL GAIN $:
```html
<div class="portfolio-summary-item">
    <span class="label">CAPITAL GAIN %</span>
    <span class="text-bold" :class="gainClass(capitalGainPct)" x-text="pct(capitalGainPct)"></span>
</div>
```

### 4. `internal/handlers/dashboard_stress_test.go`

Update expected summary labels array:
```go
// OLD:
summaryLabels := []string{"TOTAL VALUE", "COST BASIS", "NET RETURN $", "NET RETURN %"}
// NEW:
summaryLabels := []string{"TOTAL VALUE", "NET EQUITY CAPITAL", "AVAILABLE CASH", "NET RETURN $", "NET RETURN %"}
```

Also update capital performance labels if checked:
```go
// OLD:
[]string{"TOTAL DEPOSITED", "CAPITAL GAIN $", "SIMPLE RETURN %", "ANNUALIZED %"}
// NEW:
[]string{"TOTAL DEPOSITED", "CAPITAL GAIN $", "CAPITAL GAIN %", "SIMPLE RETURN %", "ANNUALIZED %"}
```

### 5. `tests/ui/dashboard_test.go`

#### TestDashboardPortfolioSummary
- Update item count check from 4 to 5
- Update expected labels array to: `['TOTAL VALUE', 'NET EQUITY CAPITAL', 'AVAILABLE CASH', 'NET RETURN $', 'NET RETURN %']`
- Update capital row count check from 4 to 5
- Update minimum items check from `< 4` to `< 5`

#### TestDashboardCapitalPerformance
- Update count check from 4 to 5
- Update expected labels to: `['TOTAL DEPOSITED', 'CAPITAL GAIN $', 'CAPITAL GAIN %', 'SIMPLE RETURN %', 'ANNUALIZED %']`
- Update comment about gain class indices (items 1-4 instead of 1-3)

#### TestDashboardGainColors
- Update return item indices: NET RETURN $ and NET RETURN % are now at indices 3 and 4 (were 2 and 3) due to the extra AVAILABLE CASH item
- Update minimum items check from `< 4` to `< 5`

## Edge Cases

1. **`available_cash` is negative:** Valid when `total_cost > total_cash`. Display as-is with `fmt()`. No `gainClass()`.
2. **`capital_gain` / `capital_gain_pct` zero or absent:** Server uses `omitempty`. `Number(undefined) || 0` returns 0. Capital row only shows when `hasCapitalData` is true.
3. **No capital performance data:** Entire capital row hidden via `x-show="hasCapitalData"`. All capital properties reset to 0.
4. **Old server version:** Fields will be `undefined` in JSON. `Number(undefined) || 0` returns 0. Dashboard degrades gracefully.
5. **Growth chart ExternalBalance removal:** Was always 0. No visual change.

## Implementation Order

1. `internal/vire/models/portfolio.go` — Add struct fields
2. `pages/static/common.js` — Add data properties, update parsing, remove computations
3. `pages/dashboard.html` — Update summary rows
4. `internal/handlers/dashboard_stress_test.go` — Update expected labels
5. `tests/ui/dashboard_test.go` — Update expected counts and labels

## Verification

```bash
go test ./internal/handlers/ -run TestDashboard -v
go test ./internal/handlers/ -v -race
./scripts/ui-test.sh dashboard
```
