# Dashboard 3-Row Summary Layout

**Feedback:** fb_d88928ac
**Status:** In Progress

## Scope

Reorganize the dashboard summary header from 2-row to 3-row layout. Add two new data fields (`grossContributions`, `totalDividends`). No new API calls — all data comes from existing `/api/portfolios/{name}` response.

**Current (2 rows):**
- Row 1: TOTAL VALUE, GROSS CASH BALANCE, AVAILABLE CASH, CAPITAL RETURN $ (cond), CAPITAL RETURN % (cond), SIMPLE RETURN % (cond), ANNUALIZED % (cond) — 7 items mixed
- Row 2: NET EQUITY CAPITAL, NET RETURN $, NET RETURN % — 3 items

**Target (3 rows):**
- Row 1 (Portfolio): TOTAL VALUE, CAPITAL RETURN $, CAPITAL RETURN %, SIMPLE RETURN %, ANNUALIZED % — 5 items
- Row 2 (Cash): GROSS CASH BALANCE, AVAILABLE CASH, GROSS CONTRIBUTIONS, DIVIDENDS — 4 items
- Row 3 (Equity): NET EQUITY CAPITAL, NET RETURN $, NET RETURN % — 3 items

---

## File Changes

### 1. `pages/static/css/portal.css` (lines 1004-1010)

**Rename** `.portfolio-summary-capital` to `.portfolio-summary-equity` (line 1004):
```css
.portfolio-summary-equity {
    border-bottom: 1px solid #888;
}
```

**Add** `.portfolio-summary-cash` class after it:
```css
.portfolio-summary-cash {
    border-bottom: 1px solid #888;
}
```

Keep `.portfolio-summary-accounts` unchanged (capital.html only).

Row 1 inherits `border-bottom: 2px solid #000` from base `.portfolio-summary`. Rows 2+3 use `1px solid #888`.

### 2. `pages/static/common.js`

**A. Add properties** (after `availableCash: 0,` around line 194):
```javascript
        grossContributions: 0,
        totalDividends: 0,
```

**B. In `loadPortfolio()` — compute totalDividends** (after `this.holdings = ...` line 265):
```javascript
                    this.totalDividends = this.holdings.reduce((sum, h) => sum + (Number(h.dividend_return) || 0), 0);
```

**C. In `loadPortfolio()` capital_performance if-block** (after `this.hasCapitalData = true;` line 280):
```javascript
                        this.grossContributions = Number(cp.gross_capital_deposited) || 0;
```

**D. In `loadPortfolio()` capital else-block** (lines 282-285, add):
```javascript
                        this.grossContributions = 0;
```

**E. In `loadPortfolio()` error path** (lines 288-297, after `this.availableCash = 0;`):
```javascript
                    this.grossContributions = 0;
                    this.totalDividends = 0;
```

**F. In `refreshPortfolio()` — mirror all additions:**
- After holdings dedup: add `this.totalDividends = this.holdings.reduce(...)`
- In capital if-block: add `this.grossContributions = Number(cp.gross_capital_deposited) || 0;`
- In capital else-block: add `this.grossContributions = 0;`

### 3. `pages/dashboard.html` (replace lines 57-102)

**Row 1 — Portfolio overview:**
```html
            <!-- Portfolio overview -->
            <div class="portfolio-summary" x-show="filteredHoldings.length > 0" x-cloak>
                <div class="portfolio-summary-item">
                    <span class="label">TOTAL VALUE</span>
                    <span class="text-bold" x-text="fmt(totalValue)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">CAPITAL RETURN $</span>
                    <span class="text-bold" :class="gainClass(capitalGain)" x-text="fmt(capitalGain)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">CAPITAL RETURN %</span>
                    <span class="text-bold" :class="gainClass(capitalGainPct)" x-text="pct(capitalGainPct)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">SIMPLE RETURN %</span>
                    <span class="text-bold" :class="gainClass(simpleReturnPct)" x-text="pct(simpleReturnPct)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">ANNUALIZED %</span>
                    <span class="text-bold" :class="gainClass(annualizedReturnPct)" x-text="pct(annualizedReturnPct)"></span>
                </div>
            </div>
```

**Row 2 — Cash summary:**
```html
            <!-- Cash summary -->
            <div class="portfolio-summary portfolio-summary-cash" x-show="filteredHoldings.length > 0" x-cloak>
                <div class="portfolio-summary-item">
                    <span class="label">GROSS CASH BALANCE</span>
                    <span class="text-bold" x-text="fmt(grossCashBalance)"></span>
                </div>
                <div class="portfolio-summary-item">
                    <span class="label">AVAILABLE CASH</span>
                    <span class="text-bold" x-text="fmt(availableCash)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">GROSS CONTRIBUTIONS</span>
                    <span class="text-bold" x-text="fmt(grossContributions)"></span>
                </div>
                <div class="portfolio-summary-item" x-show="hasCapitalData" x-cloak>
                    <span class="label">DIVIDENDS</span>
                    <span class="text-bold" x-text="fmt(totalDividends)"></span>
                </div>
            </div>
```

**Row 3 — Equity performance:**
```html
            <!-- Equity performance -->
            <div class="portfolio-summary portfolio-summary-equity" x-show="filteredHoldings.length > 0" x-cloak>
                <div class="portfolio-summary-item">
                    <span class="label">NET EQUITY CAPITAL</span>
                    <span class="text-bold" x-text="fmt(totalCost)"></span>
                </div>
                <div class="portfolio-summary-item">
                    <span class="label">NET RETURN $</span>
                    <span class="text-bold" :class="gainClass(totalGain)" x-text="fmt(totalGain)"></span>
                </div>
                <div class="portfolio-summary-item">
                    <span class="label">NET RETURN %</span>
                    <span class="text-bold" :class="gainClass(totalGainPct)" x-text="pct(totalGainPct)"></span>
                </div>
            </div>
```

### 4. `tests/ui/dashboard_test.go`

**A. Update `TestDashboardPortfolioSummary` (lines 180-297):**

- Line 204: Change selector to `.portfolio-summary:not(.portfolio-summary-cash):not(.portfolio-summary-equity) .portfolio-summary-item`
- Line 208: Allow count 1 or 5 (`count != 5 && count != 1`)
- Line 215: Update querySelector to `.portfolio-summary:not(.portfolio-summary-cash):not(.portfolio-summary-equity)`
- Line 219: Update expected labels to `['TOTAL VALUE', 'CAPITAL RETURN $', 'CAPITAL RETURN %', 'SIMPLE RETURN %', 'ANNUALIZED %']`
- Lines 234-240: Update `.portfolio-summary-capital` → `.portfolio-summary-equity`, expected count from 5 to 3
- Lines 278-295: Update gain class check to reference equity row items

**B. Update `TestDashboardCapitalPerformance` (lines 507-582):**

Replace contents with checks for:

**Cash row (Row 2):**
- Visibility: `.portfolio-summary-cash`
- Count: `.portfolio-summary-cash .portfolio-summary-item` — expect 2 or 4
- Labels: `['GROSS CASH BALANCE', 'AVAILABLE CASH', 'GROSS CONTRIBUTIONS', 'DIVIDENDS']`

**Equity row (Row 3):**
- Visibility: `.portfolio-summary-equity`
- Count: `.portfolio-summary-equity .portfolio-summary-item` — expect 3
- Labels: `['NET EQUITY CAPITAL', 'NET RETURN $', 'NET RETURN %']`
- Gain coloring on `.portfolio-summary-equity` items

### 5. `internal/handlers/dashboard_stress_test.go`

**A. `TestDashboardHandler_StressPortfolioSummarySection` (line ~535):**
Update summaryLabels to: `["TOTAL VALUE", "CAPITAL RETURN $", "CAPITAL RETURN %", "SIMPLE RETURN %", "ANNUALIZED %"]`

**B. `TestDashboardHandler_StressCapitalPerformanceLabels` (line ~593):**
Replace capitalLabels with two arrays:
- cashLabels: `["GROSS CASH BALANCE", "AVAILABLE CASH", "GROSS CONTRIBUTIONS", "DIVIDENDS"]`
- equityLabels: `["NET EQUITY CAPITAL", "NET RETURN $", "NET RETURN %"]`

**C. `TestDashboardHandler_StressNewFieldBindingsSafe` (line ~545):**
Add binding checks:
- `x-text="fmt(grossContributions)"`
- `x-text="fmt(totalDividends)"`

---

## Edge Cases

1. **!hasCapitalData**: Row 1 shows 1 item (TOTAL VALUE). Row 2 shows 2 items (GROSS CASH BALANCE, AVAILABLE CASH). Row 3 shows all 3 items.
2. **No holdings**: All 3 rows hidden (`x-show="filteredHoldings.length > 0"`).
3. **Zero dividends**: Shows "0.00" via `fmt()`.
4. **Missing `dividend_return` on holdings**: `Number(h.dividend_return) || 0` safely returns 0.
5. **Missing `gross_capital_deposited`**: `Number(cp.gross_capital_deposited) || 0` safely returns 0.

---

## Implementation Order

1. CSS — rename class, add new class
2. JS — add properties and data parsing
3. HTML — restructure into 3 rows
4. UI tests — update selectors and expectations
5. Stress tests — update label arrays and binding checks
