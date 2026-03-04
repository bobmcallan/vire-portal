# Requirements: Net Return D/W/M + Dashboard 2-Row Consolidation

## Feedback Items
- **fb_38399550**: D/W/M period breakdowns for NET RETURN $ and NET RETURN %
- **fb_1cd42c8f**: Consolidate dashboard layout from 3 rows to 2 rows

---

## 1. Scope

### What changes:

**API field migration (common.js):**
- Remove `equity_value` D/W/M parsing (field removed from API `period_changes`)
- Remove `changeEquityDayPct`, `changeEquityWeekPct`, `changeEquityMonthPct`, `hasEquityChanges` properties
- Add `net_equity_return` D/W/M parsing (raw_change values as dollar amounts)
- Add `net_equity_return_pct` D/W/M parsing (raw_change values as percentage points)
- Add 8 new properties: `changeReturnDayDollar`, `changeReturnWeekDollar`, `changeReturnMonthDollar`, `hasReturnDollarChanges`, `changeReturnDayPct`, `changeReturnWeekPct`, `changeReturnMonthPct`, `hasReturnPctChanges`
- Add 1 new helper: `changeDollar(val)` for compact dollar formatting with +/- sign

**Layout consolidation (dashboard.html):**
- Current 3 rows become 2 rows
- Row 1 (Composition): PORTFOLIO VALUE (with D/W/M) | GROSS CASH BALANCE | AVAILABLE CASH | NET EQUITY
- Row 2 (Performance): NET RETURN $ (with D/W/M) | NET RETURN % (with D/W/M) | DIVIDENDS
- Remove `portfolio-summary-cash` CSS class usage (items move to Row 1)
- Remove `portfolio-summary-equity` CSS class usage (items merge into Row 2)
- Both rows use base `portfolio-summary` class
- Row 2 gets new class `portfolio-summary-performance`
- GROSS CONTRIBUTIONS item removed from dashboard display (JS property retained)

**CSS (portal.css):**
- Add `.portfolio-summary-performance` rule (replaces `.portfolio-summary-equity` border style)

**Tests:**
- Update stress tests: label arrays, binding assertions
- Update UI tests: replace old equity tests, add return change tests

### What stays untouched:
- Portfolio value D/W/M (unchanged, still reads `portfolio_value.pct_change`)
- Growth chart, holdings table, portfolio selector, refresh button, last synced
- `hasCapitalData`, `capitalInvested`, `grossContributions` JS properties (all remain)
- `grossCashBalance`, `availableCash`, `totalDividends`, `ledgerDividendReturn` properties

---

## 2. File Changes

### File 1: `pages/static/common.js`

**Change A -- Replace equity change properties (lines 203-206)**

Remove:
```javascript
        changeEquityDayPct: null,
        changeEquityWeekPct: null,
        changeEquityMonthPct: null,
        hasEquityChanges: false,
```

Replace with:
```javascript
        changeReturnDayDollar: null,
        changeReturnWeekDollar: null,
        changeReturnMonthDollar: null,
        hasReturnDollarChanges: false,
        changeReturnDayPct: null,
        changeReturnWeekPct: null,
        changeReturnMonthPct: null,
        hasReturnPctChanges: false,
```

**Change B -- Add `changeDollar()` helper (after `changePct()` helper)**

Add this new method after `changePct`:
```javascript
        changeDollar(val) {
            if (val == null) return '-';
            const sign = val >= 0 ? '+' : '';
            const abs = Math.abs(val);
            if (abs >= 1000000) return sign + (val / 1000000).toFixed(1) + 'M';
            if (abs >= 1000) return sign + (val / 1000).toFixed(1) + 'K';
            return sign + Number(val).toFixed(0);
        },
```

| Input | Output |
|-------|--------|
| `-29406.14` | `"-29.4K"` |
| `1234.56` | `"+1.2K"` |
| `500` | `"+500"` |
| `-750` | `"-750"` |
| `2500000` | `"+2.5M"` |
| `0` | `"+0"` |
| `null` | `"-"` |

**Change C -- Update `loadPortfolio()` change parsing (lines 305-309)**

Remove equity_value parsing block:
```javascript
                        // Equity changes
                        this.changeEquityDayPct = changes.yesterday?.equity_value?.has_previous ? changes.yesterday.equity_value.pct_change : null;
                        this.changeEquityWeekPct = changes.week?.equity_value?.has_previous ? changes.week.equity_value.pct_change : null;
                        this.changeEquityMonthPct = changes.month?.equity_value?.has_previous ? changes.month.equity_value.pct_change : null;
                        this.hasEquityChanges = this.changeEquityDayPct !== null || this.changeEquityWeekPct !== null || this.changeEquityMonthPct !== null;
```

Replace with:
```javascript
                        // Net return $ changes (raw_change = dollar movement in return)
                        this.changeReturnDayDollar = changes.yesterday?.net_equity_return?.has_previous ? changes.yesterday.net_equity_return.raw_change : null;
                        this.changeReturnWeekDollar = changes.week?.net_equity_return?.has_previous ? changes.week.net_equity_return.raw_change : null;
                        this.changeReturnMonthDollar = changes.month?.net_equity_return?.has_previous ? changes.month.net_equity_return.raw_change : null;
                        this.hasReturnDollarChanges = this.changeReturnDayDollar !== null || this.changeReturnWeekDollar !== null || this.changeReturnMonthDollar !== null;
                        // Net return % changes (raw_change = percentage point movement)
                        this.changeReturnDayPct = changes.yesterday?.net_equity_return_pct?.has_previous ? changes.yesterday.net_equity_return_pct.raw_change : null;
                        this.changeReturnWeekPct = changes.week?.net_equity_return_pct?.has_previous ? changes.week.net_equity_return_pct.raw_change : null;
                        this.changeReturnMonthPct = changes.month?.net_equity_return_pct?.has_previous ? changes.month.net_equity_return_pct.raw_change : null;
                        this.hasReturnPctChanges = this.changeReturnDayPct !== null || this.changeReturnWeekPct !== null || this.changeReturnMonthPct !== null;
```

**Change D -- Update `loadPortfolio()` else/reset block (lines 319-322)**

Remove:
```javascript
                        this.changeEquityDayPct = null;
                        this.changeEquityWeekPct = null;
                        this.changeEquityMonthPct = null;
                        this.hasEquityChanges = false;
```

Replace with:
```javascript
                        this.changeReturnDayDollar = null;
                        this.changeReturnWeekDollar = null;
                        this.changeReturnMonthDollar = null;
                        this.hasReturnDollarChanges = false;
                        this.changeReturnDayPct = null;
                        this.changeReturnWeekPct = null;
                        this.changeReturnMonthPct = null;
                        this.hasReturnPctChanges = false;
```

**Change E -- Update `refreshPortfolio()` change parsing (lines 583-587)**

Same replacement as Change C.

**Change F -- Update `refreshPortfolio()` else/reset block (lines 597-600)**

Same replacement as Change D.

**IMPORTANT**: All occurrences of the 4 equity property assignments must be replaced with the 8 return property assignments. There are 2 parsing blocks (loadPortfolio + refreshPortfolio) and 2 reset blocks (loadPortfolio else + refreshPortfolio else).

---

### File 2: `pages/dashboard.html`

**Full replacement of lines 62-118** (all 3 current summary rows become 2 rows).

Remove everything from the portfolio overview div through the equity performance closing div.

Replace with:

```html
            <!-- Row 1: Composition -->
            <div class="portfolio-summary" x-show="filteredHoldings.length > 0" x-cloak>
                <div class="portfolio-summary-item">
                    <span class="label">PORTFOLIO VALUE <span class="label-info" :data-tooltip="glossaryDef('portfolio_value')">i</span></span>
                    <span class="text-bold" x-text="fmt(totalValue)"></span>
                    <span class="portfolio-changes" x-show="hasChanges" x-cloak>
                        <span :class="changeClass(changeDayPct)" x-text="'D:' + changePct(changeDayPct)"></span>
                        <span :class="changeClass(changeWeekPct)" x-text="'W:' + changePct(changeWeekPct)"></span>
                        <span :class="changeClass(changeMonthPct)" x-text="'M:' + changePct(changeMonthPct)"></span>
                    </span>
                </div>
                <div class="portfolio-summary-item">
                    <span class="label">GROSS CASH BALANCE <span class="label-info" :data-tooltip="glossaryDef('gross_cash_balance')">i</span></span>
                    <span class="text-bold" x-text="fmt(grossCashBalance)"></span>
                </div>
                <div class="portfolio-summary-item">
                    <span class="label">AVAILABLE CASH <span class="label-info" :data-tooltip="glossaryDef('net_cash_balance')">i</span></span>
                    <span class="text-bold" x-text="fmt(availableCash)"></span>
                </div>
                <div class="portfolio-summary-item">
                    <span class="label">NET EQUITY <span class="label-info" :data-tooltip="glossaryDef('net_equity_cost')">i</span></span>
                    <span class="text-bold" x-text="fmt(totalCost)"></span>
                </div>
            </div>

            <!-- Row 2: Performance -->
            <div class="portfolio-summary portfolio-summary-performance" x-show="filteredHoldings.length > 0" x-cloak>
                <div class="portfolio-summary-item">
                    <span class="label">NET RETURN $ <span class="label-info" :data-tooltip="glossaryDef('net_equity_return')">i</span></span>
                    <span class="text-bold" :class="gainClass(totalGain)" x-text="fmt(totalGain)"></span>
                    <span class="portfolio-changes" x-show="hasReturnDollarChanges" x-cloak>
                        <span :class="changeClass(changeReturnDayDollar)" x-text="'D:' + changeDollar(changeReturnDayDollar)"></span>
                        <span :class="changeClass(changeReturnWeekDollar)" x-text="'W:' + changeDollar(changeReturnWeekDollar)"></span>
                        <span :class="changeClass(changeReturnMonthDollar)" x-text="'M:' + changeDollar(changeReturnMonthDollar)"></span>
                    </span>
                </div>
                <div class="portfolio-summary-item">
                    <span class="label">NET RETURN % <span class="label-info" :data-tooltip="glossaryDef('net_equity_return_pct')">i</span></span>
                    <span class="text-bold" :class="gainClass(totalGainPct)" x-text="pct(totalGainPct)"></span>
                    <span class="portfolio-changes" x-show="hasReturnPctChanges" x-cloak>
                        <span :class="changeClass(changeReturnDayPct)" x-text="'D:' + changePct(changeReturnDayPct)"></span>
                        <span :class="changeClass(changeReturnWeekPct)" x-text="'W:' + changePct(changeReturnWeekPct)"></span>
                        <span :class="changeClass(changeReturnMonthPct)" x-text="'M:' + changePct(changeReturnMonthPct)"></span>
                    </span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">DIVIDENDS <span class="label-info" :data-tooltip="glossaryDef('dividend_forecast')">i</span></span>
                    <span class="text-bold" x-text="fmt(ledgerDividendReturn) + ' (' + fmt(totalDividends) + ')'"></span>
                </div>
            </div>
```

Key changes:
- Row 1: 4 items (was 1). Adds GROSS CASH BALANCE, AVAILABLE CASH, NET EQUITY.
- Row 1: No cash D/W/M badges (removed).
- Row 2: NET RETURN $ with D/W/M (changeDollar), NET RETURN % with D/W/M (changePct), DIVIDENDS.
- Row 2: Uses `portfolio-summary-performance` class.
- GROSS CONTRIBUTIONS removed from display.
- Cash D/W/M badges removed from display.
- NET EQUITY no longer shows D/W/M.

---

### File 3: `pages/static/css/portal.css`

**Add `.portfolio-summary-performance` rule** (after existing portfolio-summary rules):

```css
.portfolio-summary-performance {
    border-bottom: 1px solid #888;
}
```

---

### File 4: `internal/handlers/dashboard_stress_test.go`

**Change A -- Update `TestDashboardHandler_StressCapitalPerformanceLabels`**

Replace label arrays to reflect 2-row layout:
```go
// Row 1: Composition
compositionLabels := []string{"PORTFOLIO VALUE", "GROSS CASH BALANCE", "AVAILABLE CASH", "NET EQUITY"}
// Row 2: Performance
performanceLabels := []string{"NET RETURN $", "NET RETURN %", "DIVIDENDS"}
```

Remove "GROSS CONTRIBUTIONS" from expected labels.

**Change B -- Replace `TestDashboardHandler_StressCashEquityChangeBindings` entirely**

Rename to `TestDashboardHandler_StressReturnChangeBindings`.

Remove cash binding assertions (cash D/W/M badges removed from template).
Remove equity binding assertions (equity D/W/M badges removed from template).

Add:
- Net return $ bindings: `changeClass(changeReturnDayDollar)`, `changeClass(changeReturnWeekDollar)`, `changeClass(changeReturnMonthDollar)`
- Net return % bindings: `changeClass(changeReturnDayPct)`, `changeClass(changeReturnWeekPct)`, `changeClass(changeReturnMonthPct)`
- Visibility: `hasReturnDollarChanges`, `hasReturnPctChanges`

**Change C -- Update `TestDashboardHandler_StressNewFieldBindingsSafe`**

Remove assertion for `grossContributions` binding (item removed from dashboard).

**Change D -- Update `TestDashboardHandler_StressChangesRowConditionalDisplay`**

Add assertions for `hasReturnDollarChanges` and `hasReturnPctChanges` visibility bindings.
Remove `hasEquityChanges` and `hasCashChanges` assertions (badges removed from template).

**Change E -- Update `TestDashboardHandler_StressChangesInsidePortfolioValueItem`**

Update structural check from `portfolio-summary-cash` to `portfolio-summary-performance`.

**Change F -- Update `TestDashboardHandler_StressGlossaryTooltipBindings`**

Add `glossaryDef('net_equity_return')`, `glossaryDef('net_equity_return_pct')`, `glossaryDef('dividend_forecast')` to expected bindings if not already present.

---

### File 5: `tests/ui/dashboard_test.go`

**Change A -- Delete `TestDashboardEquityChangesRow` (lines 999-1071)**

Old equity D/W/M test. No longer applicable.

**Change B -- Delete `TestDashboardCashChangesRow` (lines 925-997)**

Cash D/W/M badges removed from template. Test no longer applicable.

**Change C -- Add `TestDashboardReturnDollarChanges`**

Validates D/W/M badges under NET RETURN $ in `.portfolio-summary-performance`:
- Check `.portfolio-summary-performance` visible
- Check first item has `.portfolio-changes` element
- Verify D:/W:/M: labels present
- Verify `change-up/down/neutral` CSS classes applied

**Change D -- Add `TestDashboardReturnPctChanges`**

Same pattern targeting second item in `.portfolio-summary-performance`:
- Check `.portfolio-summary-performance` visible
- Check second item has `.portfolio-changes` element
- Verify D:/W:/M: labels present
- Verify `change-up/down/neutral` CSS classes applied

**Change E -- Update `TestDashboardCapitalPerformance`**

Rewrite to validate 2-row layout:
- Row 1: 4 items (PORTFOLIO VALUE, GROSS CASH BALANCE, AVAILABLE CASH, NET EQUITY)
- Row 2: 2 or 3 items (NET RETURN $, NET RETURN %, optionally DIVIDENDS)
- Selector: `.portfolio-summary:not(.portfolio-summary-performance)` for Row 1
- Selector: `.portfolio-summary-performance` for Row 2

**Change F -- Update `TestDashboardPortfolioSummary`**

Update item count from 1 to 4.
Update label validation to check all 4 labels.
Update selector negation from `:not(.portfolio-summary-equity)` to `:not(.portfolio-summary-performance)`.

**Change G -- Update `TestDashboardChangesRow`**

Update selectors from `:not(.portfolio-summary-cash):not(.portfolio-summary-equity)` to `:not(.portfolio-summary-performance)`.

---

## 3. Edge Cases

1. **`net_equity_return` missing from API**: Properties stay null, badges hidden via `x-show`.
2. **Return crosses zero**: Uses `raw_change` (not `pct_change`) — human-readable even when returns cross zero.
3. **`has_previous` false**: All change properties null, badges hidden.
4. **Cash D/W/M removed**: JS properties retained, badges removed from template.
5. **`hasCapitalData` false**: DIVIDENDS hidden, Row 2 shows 2 items.
6. **Zero return change**: `changeDollar(0)` → `"+0"`, `changeClass(0)` → `"change-neutral"`.

---

## 4. Implementation Order

1. `pages/static/common.js` — properties, parsing, helper
2. `pages/dashboard.html` — layout restructure
3. `pages/static/css/portal.css` — add performance class
4. `internal/handlers/dashboard_stress_test.go` — update stress tests
5. `tests/ui/dashboard_test.go` — update UI tests

---

## 5. Verification Checklist

- [ ] `equity_value` no longer referenced in common.js
- [ ] `changeEquityDayPct`, `changeEquityWeekPct`, `changeEquityMonthPct`, `hasEquityChanges` fully removed
- [ ] `net_equity_return.raw_change` used for dollar D/W/M
- [ ] `net_equity_return_pct.raw_change` used for percentage D/W/M
- [ ] Both `loadPortfolio()` and `refreshPortfolio()` updated
- [ ] Dashboard has exactly 2 `portfolio-summary` divs
- [ ] No `portfolio-summary-cash` or `portfolio-summary-equity` in dashboard.html
- [ ] `portfolio-summary-performance` class on Row 2
- [ ] GROSS CONTRIBUTIONS no longer in dashboard.html
- [ ] Cash D/W/M badges no longer in dashboard.html
- [ ] All stress tests pass
- [ ] All UI tests pass
