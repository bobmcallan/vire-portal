# Requirements: Stock Page UX + Dashboard Currency/Dividends

**Feedback:** fb_1980cdf6, fb_441c0d42
**Scope:** Stock page restructure (walk chart, trade history, gain breakdown, currency, dividends, remove news), dashboard currency verification.

---

## Overview

4 files to modify, no handler changes needed. The stock handler already fetches all required data via StockDetailJSON (position, trades, position_timeline).

---

## File Changes

### 1. `pages/stock.html` — Template Restructure

**Section order (top to bottom):**
1. Back link (existing)
2. Ticker heading (existing)
3. PORTFOLIO HOLDING — restructured with gain/loss breakdown, currency consolidation, dividend split, currency labels
4. POSITION WALK — NEW: Chart.js line chart (market_value + cost_basis over time with trade markers)
5. TRADE HISTORY — replace placeholder with real trade table
6. STOCK PRICE TREND — existing (keep as-is)
7. FILINGS TIMELINE — existing (keep as-is)
8. COMPANY OVERVIEW — existing (keep as-is)
9. STRATEGY ALIGNMENT — existing placeholder (keep as-is)
10. ~~NEWS & SENTIMENT~~ — REMOVE entirely

**PORTFOLIO HOLDING restructure:**

Replace the existing detail grid with a restructured version:

Row 1: Core position
- NAME (no change)
- VALUE — for AUD holdings: `$X (AUD)`. For USD holdings: `$X AUD / $Y USD` using position.holding_value_market and (position.holding_value_market / detail.price.current * position.units) or similar
  - Actually simpler: show `$X (original_currency)` — the value is already AUD-converted
  - For USD holdings, also show: `Local: $Y USD` as secondary line
- WEIGHT (no change)

Row 2: Returns breakdown
- CAPITAL RETURN — show `$X (realized: $Y / unrealized: $Z)` using position.realized_return, position.unrealized_return
- RETURN % — position.holding_return_net_pct (no change, just add currency label)
- TREND (no change)

Row 3: Dividends & Currency
- DIVIDENDS — `$X received / $Y forecast` using position.dividend_received, position.dividend_forecast. Hide if both 0.
- CURRENCY GAIN/LOSS — position.currency_gain_loss. Show only for non-AUD holdings. Hide if 0 or null.

Remove standalone CURRENCY and FX GAIN/LOSS fields. Remove standalone DIVIDENDS RECEIVED and DIVIDEND FORECAST fields.

**POSITION WALK section (new):**
```html
<section class="panel-headed" x-show="hasWalkData" x-cloak>
    <div class="panel-header">POSITION WALK</div>
    <div class="panel-content">
        <div class="walk-chart-scroll">
            <div class="walk-chart-sizer">
                <canvas id="walkChart"></canvas>
            </div>
        </div>
    </div>
</section>
```

Chart.js config:
- Line 1: market_value (blue/primary color) from position_timeline
- Line 2: cost_basis (dashed gray) from position_timeline
- Scatter overlay: trade markers (green up-arrow for Buy, red down-arrow for Sell) from trades[]
- X axis: dates
- Y axis: dollar value
- Tooltip: shows date, market_value, cost_basis, net_return

**TRADE HISTORY section (replace placeholder):**
```html
<section class="panel-headed" x-show="hasTrades" x-cloak>
    <div class="panel-header">TRADE HISTORY</div>
    <div class="panel-content">
        <div class="trade-summary">
            <span>Avg Cost: <strong x-text="fmt(position.holding_cost_avg)"></strong></span>
            <span>Breakeven: <strong x-text="fmt(position.true_breakeven_price)"></strong></span>
            <span>Units: <strong x-text="position.units"></strong></span>
        </div>
        <div class="table-wrap">
            <table class="tool-table">
                <thead><tr><th>Date</th><th>Type</th><th class="text-right">Units</th><th class="text-right">Price</th><th class="text-right">Fees</th><th class="text-right">Value</th></tr></thead>
                <template x-for="t in trades" :key="t.id">
                    <tbody><tr>
                        <td x-text="fmtDate(t.date)"></td>
                        <td><span class="trade-type-badge" :class="t.type === 'Buy' ? 'trade-buy' : 'trade-sell'" x-text="t.type"></span></td>
                        <td class="text-right" x-text="t.units.toLocaleString()"></td>
                        <td class="text-right" x-text="'$' + Number(t.price).toFixed(4)"></td>
                        <td class="text-right" x-text="fmt(t.fees)"></td>
                        <td class="text-right" x-text="fmt(t.value)"></td>
                    </tr></tbody>
                </template>
            </table>
        </div>
    </div>
</section>
```

**Remove NEWS & SENTIMENT:**
Delete the entire `<section>` block for NEWS & SENTIMENT.

### 2. `pages/static/common.js` — Alpine Component Updates

**stockDetail() changes:**

Add new state:
```javascript
trades: [],
position: null,
positionTimeline: [],
walkChart: null,
hasWalkData: false,
hasTrades: false,
```

Update init():
```javascript
// After existing detail assignment
if (this.detail) {
    this.position = this.detail.position || null;
    this.trades = this.detail.trades || [];
    this.positionTimeline = this.detail.position_timeline || [];
    this.hasWalkData = this.positionTimeline.length > 0;
    this.hasTrades = this.trades.length > 0;
}
// Render walk chart after price chart
this.$nextTick(() => {
    if (this.detail) {
        this.renderPriceChart();
        if (this.hasWalkData) this.renderWalkChart();
    }
});
```

Add new methods:
```javascript
renderWalkChart() {
    // Chart.js line chart with two datasets:
    // 1. Market Value (solid line)
    // 2. Cost Basis (dashed line)
    // Plus scatter points for trades (Buy = green, Sell = red)
    const ctx = document.getElementById('walkChart');
    if (!ctx) return;
    if (this.walkChart) { this.walkChart.destroy(); }

    const labels = this.positionTimeline.map(p => this.fmtDate(p.date));
    const marketValues = this.positionTimeline.map(p => p.market_value);
    const costBases = this.positionTimeline.map(p => p.cost_basis);

    // Map trades to chart points
    const buyPoints = [];
    const sellPoints = [];
    for (const t of this.trades) {
        const dateStr = this.fmtDate(t.date);
        const idx = labels.indexOf(dateStr);
        if (idx >= 0) {
            const point = { x: dateStr, y: marketValues[idx] };
            if (t.type === 'Buy') buyPoints.push(point);
            else sellPoints.push(point);
        }
    }

    this.walkChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                { label: 'Market Value', data: marketValues, borderColor: '#000', borderWidth: 2, pointRadius: 0, fill: false },
                { label: 'Cost Basis', data: costBases, borderColor: '#888', borderWidth: 1, borderDash: [5,5], pointRadius: 0, fill: false },
                { label: 'Buy', data: buyPoints, type: 'scatter', pointRadius: 6, pointStyle: 'triangle', backgroundColor: '#16a34a', borderColor: '#16a34a' },
                { label: 'Sell', data: sellPoints, type: 'scatter', pointRadius: 6, pointStyle: 'triangle', rotation: 180, backgroundColor: '#dc2626', borderColor: '#dc2626' },
            ]
        },
        options: { /* same minimal style as price chart */ }
    });
},

destroyWalkChart() {
    if (this.walkChart) { this.walkChart.destroy(); this.walkChart = null; }
},

// Currency display helper
holdingCurrency() {
    return this.position?.original_currency || this.holding?.original_currency || 'AUD';
},

isForeignCurrency() {
    return this.holdingCurrency() !== 'AUD';
},

// Dividend display
dividendDisplay() {
    const recv = this.position?.dividend_received || this.holding?.dividend_received || 0;
    const fcast = this.position?.dividend_forecast || this.holding?.dividend_forecast || 0;
    if (recv === 0 && fcast === 0) return null;
    return this.fmt(recv) + ' received / ' + this.fmt(fcast) + ' forecast';
},
```

Remove dead methods (from deleted NEWS section):
- `sentimentClass(s)`
- `credibilityClass(c)`

### 3. `pages/static/css/portal.css` — New Styles

```css
/* Walk chart */
.walk-chart-scroll { overflow-x: auto; }
.walk-chart-sizer { min-width: 600px; height: 300px; position: relative; }

/* Trade summary bar */
.trade-summary {
    display: flex;
    gap: 2rem;
    padding: 0.5rem 0;
    border-bottom: 1px solid #ddd;
    margin-bottom: 0.5rem;
    font-size: 0.8rem;
}

/* Trade type badges */
.trade-type-badge {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    font-size: 0.75rem;
    font-weight: 600;
}
.trade-buy { background: #dcfce7; color: #166534; }
.trade-sell { background: #fee2e2; color: #991b1b; }

/* Gain breakdown */
.gain-breakdown {
    font-size: 0.8rem;
    color: #888;
}
```

Remove or leave dead NEWS styles (`.sentiment-badge`, `.credibility-badge`, `.news-article` etc.) — leaving is harmless.

### 4. Dashboard Verification (fb_441c0d42)

**No code changes needed.** Verify that:
- `pages/dashboard.html:102-105` — CURRENCY GAIN/LOSS item shows in performance row when `hasCurrencyData`
- `pages/static/common.js:255` — `hasCurrencyData: false` state
- `pages/static/common.js:366-368` — `currencyGainLoss` and `hasCurrencyData` set from `holdingsData`

Dashboard per-holding currency/dividend detail was intentionally removed in a previous session (too cluttered). These details now belong on the stock detail page.

---

## Test Cases

### Unit Tests (existing stock_test.go — no new handler tests needed since handler unchanged)

No new unit tests required. Handler is unchanged.

### Stress Tests (expand stock_stress_test.go)

- StressStockWalkChartNullTimeline — verify x-show guard when position_timeline is empty/null
- StressStockTradeHistoryNullTrades — verify x-show guard when trades is empty/null
- StressStockDividendDisplayBothZero — verify dividend section hidden when both 0
- StressStockCurrencyGainLossAUDHolding — verify currency section hidden for AUD holdings
- StressStockNewsRemovedFromTemplate — verify NEWS & SENTIMENT section not in template

### UI Tests (expand tests/ui/stock_test.go)

- TestStockWalkChartSection — verify POSITION WALK section with canvas
- TestStockTradeHistorySection — verify TRADE HISTORY with table headers
- TestStockGainBreakdown — verify realized/unrealized display
- TestStockNewsRemoved — verify NEWS & SENTIMENT section is gone
- Update TestStockPriceChartSection — verify it still works (moved below walk chart)
- Update placeholder assertions — TRADE HISTORY is no longer a placeholder

---

## Edge Cases

- No position data (holding not in portfolio) — walk chart and trade history sections hide via x-show
- No trades — trade history hides, walk chart may still show if timeline exists
- No position_timeline — walk chart hides
- AUD-only holdings — no currency gain/loss shown, no dual currency display
- Both dividends zero — dividend row hidden
- Chart.js instance cleanup — destroy walk chart on re-render/destroy
