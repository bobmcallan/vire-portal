# Requirements: Dashboard Cleanup (5 Feedback Items)

## Feedback Items
- **fb_d94e3836** (HIGH): Remove Simple Return % and Annualized % (contradictory values)
- **fb_d347cac5** (MEDIUM): Remove Capital Return $ and Capital Return % (redundant)
- **fb_d84df828** (MEDIUM, dismissed): Superseded by fb_d347cac5
- **fb_03cc83b1** (LOW): Remove portfolio-level Trend/RSI indicators (misleading)
- **fb_cb126cf5** (LOW): Rename "NET EQUITY CAPITAL" to "NET EQUITY"

## 1. Scope

**Removing entirely:**
- CAPITAL RETURN $ and CAPITAL RETURN % metrics (Row 1 of portfolio summary)
- SIMPLE RETURN % and ANNUALIZED % metrics (Row 1 of portfolio summary)
- Portfolio-level TREND, RSI, DATA POINTS indicators bar
- The `/api/portfolios/{name}/indicators` client-side fetch calls (2 occurrences)
- CSS classes: `.portfolio-indicators`, `.indicator-item`

**Renaming:**
- "NET EQUITY CAPITAL" label becomes "NET EQUITY" (Row 3, equity performance)

**Stays untouched (explicitly):**
- Row 1: PORTFOLIO VALUE (with D/W/M changes)
- Row 2 (Cash): GROSS CASH BALANCE (with D/W/M), AVAILABLE CASH, GROSS CONTRIBUTIONS, DIVIDENDS
- Row 3 (Equity): NET EQUITY (renamed, with D/W/M), NET RETURN $, NET RETURN %
- Growth chart, holdings table, portfolio selector, refresh button, last synced
- `hasCapitalData` property (still gates GROSS CONTRIBUTIONS and DIVIDENDS in Row 2)
- `capitalInvested` property (still used in `renderChart()` at line 459)
- `grossContributions` property (still displayed in Row 2)
- The `/api/portfolios/{name}/indicators` API route itself (generic proxy catch-all)

---

## 2. File Changes

### File 1: `pages/dashboard.html`

**Change A — Remove 4 capital return items from Row 1 (lines 72-87)**

Delete these 16 lines:
```html
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">CAPITAL RETURN $ <span class="label-info" :data-tooltip="glossaryDef('net_capital_return')">i</span></span>
                    <span class="text-bold" :class="gainClass(capitalGain)" x-text="fmt(capitalGain)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">CAPITAL RETURN % <span class="label-info" :data-tooltip="glossaryDef('net_capital_return_pct')">i</span></span>
                    <span class="text-bold" :class="gainClass(capitalGainPct)" x-text="pct(capitalGainPct)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">SIMPLE RETURN % <span class="label-info" :data-tooltip="glossaryDef('simple_capital_return_pct')">i</span></span>
                    <span class="text-bold" :class="gainClass(simpleReturnPct)" x-text="pct(simpleReturnPct)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">ANNUALIZED % <span class="label-info" :data-tooltip="glossaryDef('annualized_capital_return_pct')">i</span></span>
                    <span class="text-bold" :class="gainClass(annualizedReturnPct)" x-text="pct(annualizedReturnPct)"></span>
                </div>
```

**Change B — Remove the entire indicators div (lines 136-141)**

Delete these lines:
```html
            <!-- Portfolio indicators -->
            <div class="portfolio-indicators" x-show="hasIndicators" x-cloak>
                <span class="indicator-item">TREND: <span class="text-bold" x-text="trend.toUpperCase()"></span></span>
                <span class="indicator-item">RSI: <span class="text-bold" x-text="rsiSignal.toUpperCase()"></span></span>
                <span class="indicator-item text-muted" x-text="dataPoints + ' DATA POINTS'"></span>
            </div>
```

**Change C — Rename label (line 118)**

Before: `NET EQUITY CAPITAL`
After: `NET EQUITY`

---

### File 2: `pages/static/common.js`

**Change A — Remove property declarations**

Remove these 8 lines from the `portfolioDashboard()` return object:
```js
        capitalGain: 0,
        simpleReturnPct: 0,
        annualizedReturnPct: 0,
        capitalGainPct: 0,
        trend: '',
        rsiSignal: '',
        dataPoints: 0,
        hasIndicators: false,
```

Keep: `capitalInvested: 0,`, `hasCapitalData: false,`, `grossContributions: 0,`

**Change B — Clean up `loadPortfolio()` capital performance block**

In the `if (cp && cp.transaction_count > 0)` block, remove the 4 lines:
```js
this.capitalGain = Number(holdingsData.net_capital_return) || 0;
this.capitalGainPct = Number(holdingsData.net_capital_return_pct) || 0;
this.simpleReturnPct = Number(cp.simple_capital_return_pct) || 0;
this.annualizedReturnPct = Number(cp.annualized_capital_return_pct) || 0;
```

Keep: `this.capitalInvested = ...`, `this.grossContributions = ...`, `this.hasCapitalData = true;`

In the else-block, clean up resets:
- Line 349: `this.capitalInvested = 0; this.capitalGain = 0;` → `this.capitalInvested = 0;`
- Remove line 350: `this.capitalGainPct = 0;`
- Remove line 351: `this.simpleReturnPct = 0; this.annualizedReturnPct = 0;`

**Change C — Clean up `loadPortfolio()` error path**

- Line 379: `this.capitalInvested = 0; this.capitalGain = 0; this.capitalGainPct = 0;` → `this.capitalInvested = 0;`
- Remove line 380: `this.simpleReturnPct = 0; this.annualizedReturnPct = 0;`
- Keep: `this.hasCapitalData = false;`

**Change D — Remove indicators fetch in `loadPortfolio()`**

Delete the entire block (lines ~383-393):
```js
                // Fetch indicators (non-blocking, non-fatal)
                vireStore.fetch('/api/portfolios/' + ... + '/indicators')
                    .then(async res => { ... }).catch(() => { ... });
```

**Change E — Clean up `refreshPortfolio()` capital performance block**

Same pattern as Change B. Remove 4 lines setting removed properties.

In else-block:
- `this.capitalInvested = 0; this.capitalGain = 0;` → `this.capitalInvested = 0;`
- Remove `this.capitalGainPct = 0;`
- Remove `this.simpleReturnPct = 0; this.annualizedReturnPct = 0;`

**Change F — Remove indicators re-fetch in `refreshPortfolio()`**

Delete the entire indicators fetch block (lines ~646-656).

---

### File 3: `pages/static/css/portal.css`

Remove `.portfolio-indicators` and `.indicator-item` styles (lines ~1126-1138):
```css
.portfolio-indicators { ... }
.indicator-item { ... }
```

---

### File 4: `internal/handlers/dashboard_stress_test.go`

**Change A — Update `TestDashboardHandler_StressOverviewSummary` (line 535)**

Before: `summaryLabels := []string{"PORTFOLIO VALUE", "CAPITAL RETURN $", "CAPITAL RETURN %", "SIMPLE RETURN %", "ANNUALIZED %"}`
After: `summaryLabels := []string{"PORTFOLIO VALUE"}`

**Change B — Remove capitalGainPct assertions in `TestDashboardHandler_StressNewFieldBindingsSafe` (lines ~571-577)**

Delete the 7 lines checking `capitalGainPct` binding and `gainClass`. Update the function comment to reflect remaining checks.

Before comment:
```go
// Verify the new dashboard fields (availableCash, capitalGainPct) use
// x-text bindings (safe) and that capitalGainPct uses gainClass (color).
```
After:
```go
// Verify the dashboard fields (availableCash, grossContributions, dividends)
// use x-text bindings (safe) with correct formatting helpers.
```

**Change C — Update equity labels (line 610)**

Before: `equityLabels := []string{"NET EQUITY CAPITAL", "NET RETURN $", "NET RETURN %"}`
After: `equityLabels := []string{"NET EQUITY", "NET RETURN $", "NET RETURN %"}`

**Change D — Update `TestDashboardHandler_StressChangesInsidePortfolioValueItem` (lines ~1225-1233)**

Replace the CAPITAL RETURN boundary check with a cash summary boundary check:
```go
cashIdx := strings.Index(body, "portfolio-summary-cash")
if cashIdx < 0 {
    t.Skip("Cash summary row not found")
}
if pcIdx > cashIdx {
    t.Error("portfolio-changes appears after cash summary — should be inside PORTFOLIO VALUE item")
}
```

---

### File 5: `tests/ui/dashboard_test.go`

**Change A — Update `TestDashboardPortfolioSummary` (lines ~203-237)**

- Change count check from `count != 1 && count != 5` to `count != 1`
- Simplify label check to only expect PORTFOLIO VALUE (no capital return labels)
- Remove capital gain color check block (lines ~275-294)

**Change B — Update equity labels in `TestDashboardCapitalPerformance` (lines ~590-607)**

- `'NET EQUITY CAPITAL'` → `'NET EQUITY'`
- Update error message accordingly

**Change C — Delete `TestDashboardIndicators` test entirely (lines ~692-739)**

Remove the entire function.

---

## 3. Properties Summary

| Property | Action | Reason |
|---|---|---|
| `capitalGain` | REMOVE | Only used by removed CAPITAL RETURN $ |
| `capitalGainPct` | REMOVE | Only used by removed CAPITAL RETURN % |
| `simpleReturnPct` | REMOVE | Only used by removed SIMPLE RETURN % |
| `annualizedReturnPct` | REMOVE | Only used by removed ANNUALIZED % |
| `trend` | REMOVE | Only used by removed indicators bar |
| `rsiSignal` | REMOVE | Only used by removed indicators bar |
| `dataPoints` | REMOVE | Only used by removed indicators bar |
| `hasIndicators` | REMOVE | Only used by removed indicators bar |
| `capitalInvested` | **KEEP** | Used in `renderChart()` |
| `hasCapitalData` | **KEEP** | Gates GROSS CONTRIBUTIONS and DIVIDENDS in Row 2 |
| `grossContributions` | **KEEP** | Displayed in Row 2 |

## 4. Edge Cases

1. **`hasCapitalData` still needed** — gates 2 items in cash row. Parsing block must remain.
2. **Growth chart uses `capitalInvested`** — "Net Deposited" line in `renderChart()`. Must keep parsing.
3. **No server-side changes** — portal stops consuming values, API still returns them.
4. **Proxy stress tests untouched** — tests in `proxy_stress_test.go` test generic proxy pass-through.

## 5. Remaining Dashboard After Cleanup

**Row 1 (Portfolio Overview):** 1 item — PORTFOLIO VALUE (with D/W/M)
**Row 2 (Cash Summary):** 2 or 4 items — GROSS CASH BALANCE (with D/W/M), AVAILABLE CASH, GROSS CONTRIBUTIONS*, DIVIDENDS*
**Row 3 (Equity Performance):** 3 items — NET EQUITY (renamed, with D/W/M), NET RETURN $, NET RETURN %
**Indicators bar:** REMOVED

*conditional on hasCapitalData
