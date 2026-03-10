# Requirements: Stock Page Restructure (fb_b26e9d0b)

## Scope

Three portal-only changes to the stock detail page:

1. **Remove Stock Price Trend section** — delete the entire section (HTML + JS + CSS)
2. **Redesign Position Walk chart as P&L chart** — show unrealised gain/loss area, breakeven line, trade markers with realised P&L annotations
3. **Add Realised P&L column to Trade History table** — calculate per-sell realised gain/loss using avg cost

Item 1 from the feedback (server-side caching) is OUT OF SCOPE — that's a vire server change.

## What this does NOT do

- No Go handler changes — purely template + CSS + JS
- No new API calls — all data is already in `stockDetail` SSR data
- No new endpoints

## Data Available (from portfolio_get_stock_detail)

### position_timeline (array)
```json
{"date": "2026-02-24T00:00:00Z", "units": 28982, "cost_basis": 39999.16, "close_price": 1.36, "market_value": 39415.52, "net_return": -583.64, "net_return_pct": -1.46}
```

### trades (array)
```json
{"id": "8589609", "type": "Buy", "date": "2026-02-24T00:00:00", "units": 28982, "price": 1.38, "fees": 3, "value": 39999.16}
```

### position (object)
- `realized_return` — total realised P&L (e.g. -2804.98)
- `unrealized_return` — current unrealised P&L
- `holding_cost_avg` — average cost basis per unit
- `true_breakeven_price` — breakeven accounting for realised P&L

---

## File Changes

### 1. `pages/stock.html` — Template changes

#### Remove Stock Price Trend (lines 113-145)
Delete the entire `<!-- Stock Price Trend -->` section including:
- The panel-headed section with candle chart
- SMA toggle checkboxes
- RSI display

#### Redesign Position Walk section (lines 74-84)
Rename header to "POSITION P&L" and keep the canvas element. The chart rendering changes are in JS.

#### Add Realised P&L column to Trade History table (lines 86-111)
- Add `<th class="text-right">Realised P&L</th>` header after Value
- Add data cell: for Sell trades, show realised P&L with gain/loss colouring. For Buy trades, show '-'.
- The realised P&L per sell = `(sell_price - avg_cost_at_time) * units - fees`. Since we don't have avg_cost_at_time per trade, use a simpler approach: `t.value - (position.holding_cost_avg * t.units)` which gives the approximate realised P&L.

  **Better approach:** Use a JS helper `tradeRealisedPL(t)` that for Sell trades computes: `t.value - (position.holding_cost_avg * t.units)`. This is approximate but correct for average cost method portfolios.

- Add total realised P&L in footer: use `position.realized_return` which is the server-computed total.

### 2. `pages/static/common.js` — JS changes

#### Remove price chart code
- Delete `renderPriceChart()` method (lines 1394-1466)
- Delete `destroyPriceChart()` method (lines 1467-1471)
- Remove `showSMA20`, `showSMA50`, `showSMA200` from state (lines 1328-1330)
- Remove `priceChart` from state (line 1332)
- Remove `renderPriceChart()` call from init (line 1540)
- Remove SMA watch calls from init (lines 1546-1548)

#### Add `tradeRealisedPL(t)` helper
```js
tradeRealisedPL(t) {
    if (t.type !== 'Sell' || !this.position) return null;
    return t.value - (this.position.holding_cost_avg * t.units);
},
```

#### Redesign `renderWalkChart()` as P&L chart

Replace the current walk chart with a P&L visualisation:

```js
renderWalkChart() {
    const ctx = document.getElementById('walkChart');
    if (!ctx) return;
    if (this.walkChart) { this.walkChart.destroy(); }
    if (typeof Chart === 'undefined') return;

    const timeline = this.positionTimeline;
    const labels = timeline.map(p => this.fmtDate(p.date));
    const plData = timeline.map(p => p.net_return);

    // Green above zero, red below — use two datasets with fill
    const aboveZero = plData.map(v => v >= 0 ? v : 0);
    const belowZero = plData.map(v => v < 0 ? v : 0);

    // Breakeven price line (cost_basis / units = avg cost per unit on that day)
    // Not needed as a separate line — the zero line IS the breakeven

    // Trade markers
    const buyPoints = [];
    const sellPoints = [];
    for (const t of this.trades) {
        const dateStr = this.fmtDate(t.date);
        const idx = labels.indexOf(dateStr);
        if (idx >= 0) {
            const point = { x: dateStr, y: plData[idx] };
            if (t.type === 'Buy') buyPoints.push(point);
            else sellPoints.push(point);
        }
    }

    this.walkChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                {
                    label: 'P&L (gain)',
                    data: aboveZero,
                    borderColor: '#2d8a4e',
                    backgroundColor: 'rgba(45, 138, 79, 0.15)',
                    borderWidth: 2,
                    pointRadius: 0,
                    fill: 'origin',
                },
                {
                    label: 'P&L (loss)',
                    data: belowZero,
                    borderColor: '#c06060',
                    backgroundColor: 'rgba(192, 96, 96, 0.15)',
                    borderWidth: 2,
                    pointRadius: 0,
                    fill: 'origin',
                },
                {
                    label: 'Buy',
                    data: buyPoints,
                    type: 'scatter',
                    pointRadius: 6,
                    pointStyle: 'triangle',
                    backgroundColor: '#16a34a',
                    borderColor: '#16a34a',
                },
                {
                    label: 'Sell',
                    data: sellPoints,
                    type: 'scatter',
                    pointRadius: 6,
                    pointStyle: 'triangle',
                    rotation: 180,
                    backgroundColor: '#dc2626',
                    borderColor: '#dc2626',
                },
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { display: true, position: 'bottom', labels: { boxWidth: 12, font: { size: 10, family: "'IBM Plex Mono', monospace" } } },
                tooltip: {
                    callbacks: {
                        label: (ctx) => {
                            if (ctx.dataset.label === 'Buy' || ctx.dataset.label === 'Sell') {
                                return ctx.dataset.label;
                            }
                            return 'P&L: $' + Number(ctx.parsed.y).toLocaleString('en-AU', { minimumFractionDigits: 0, maximumFractionDigits: 0 });
                        }
                    }
                }
            },
            scales: {
                x: { display: true, ticks: { maxTicksLimit: 8, font: { size: 9, family: "'IBM Plex Mono', monospace" } }, grid: { display: false } },
                y: {
                    display: true,
                    ticks: { font: { size: 9, family: "'IBM Plex Mono', monospace" } },
                    grid: { color: (ctx) => ctx.tick.value === 0 ? '#000' : '#eee', lineWidth: (ctx) => ctx.tick.value === 0 ? 2 : 1 },
                },
            },
        },
    });
},
```

The key design choices:
- **Two datasets** split at zero: green fill for gains, red fill for losses
- **Zero line emphasised** with thick black — this IS the breakeven reference
- **Buy/Sell scatter markers** preserved from existing code
- **Tooltip** shows P&L amount
- **No separate breakeven line needed** — the zero line represents breakeven; net_return already accounts for cost basis

### 3. `pages/static/css/portal.css` — CSS cleanup

Remove stock-price-chart-specific CSS if any exists (chart toggle labels, RSI section). Check for:
- `.stock-price-chart-section`
- `.stock-rsi-value`
- Any SMA-related styles

### 4. `internal/handlers/` — Stress tests

Add to appropriate stress test file:
- `TestStockPage_StressNoPriceChartSection` — verify no "STOCK PRICE TREND" or "stock-price-chart" in template
- `TestStockPage_StressNoSMACheckboxes` — verify no SMA references in template
- `TestStockPage_StressPandLChartPresent` — verify "POSITION P&L" header exists
- `TestStockPage_StressTradeRealisedPLColumn` — verify "Realised P&L" column header
- `TestStockPage_StressTradeRealisedPLUsesXText` — verify x-text not x-html for P&L column
- `TestStockPage_StressWalkChartCanvasExists` — verify walkChart canvas still present

## Edge Cases

- Holdings with no sells: `tradeRealisedPL()` returns null for Buy trades, shows '-'
- Holdings with no position_timeline: walk chart hidden (existing `hasWalkData` guard)
- Holdings with no trades: trade table hidden (existing `hasTrades` guard)
- `position` is null (stock not held): all sections hidden
- Net return crossing zero during timeline: chart correctly shows green/red transition
