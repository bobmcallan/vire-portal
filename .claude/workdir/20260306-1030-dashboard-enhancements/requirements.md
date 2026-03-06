# Dashboard Enhancements — Implementation Spec

## Scope

5 feedback items modifying the dashboard UI. Implementation order: FB1 → FB5 → FB4 → FB2 → FB3.

**What changes:**
- Row 2 metrics (NET RETURN $ and %) — simplified to daily-only
- Growth chart — toggle to show/hide breakdown lines + rebalance annotations
- Breadth bar — softer colours, yesterday comparison, sync timestamp, per-holding rows
- Holdings table — movement sub-rows removed (data moves to breadth section)

**What does NOT change:**
- Row 1 (PORTFOLIO VALUE D/W/M, AVAILABLE CASH, GROSS CONTRIBUTIONS) — unchanged
- Holdings table main rows (ticker, name, value, weight, return $, return %) — unchanged
- Watchlist section — unchanged
- API calls, data fetching, auth — unchanged

---

## FB1: Simplify Net Return to Daily-Only

### dashboard.html

**Lines 83-106 (Row 2: Performance)**

Replace the NET RETURN $ D/W/M badges (lines 87-91) with a single daily badge:
```html
<!-- OLD: lines 87-91 -->
<span class="portfolio-changes" x-show="hasReturnDollarChanges" x-cloak>
    <span :class="changeClass(changeReturnDayDollar)" x-text="'D:' + changeDollar(changeReturnDayDollar)"></span>
    <span :class="changeClass(changeReturnWeekDollar)" x-text="'W:' + changeDollar(changeReturnWeekDollar)"></span>
    <span :class="changeClass(changeReturnMonthDollar)" x-text="'M:' + changeDollar(changeReturnMonthDollar)"></span>
</span>

<!-- NEW -->
<span class="portfolio-change-today" x-show="changeReturnDayDollar != null" x-cloak>
    <span :class="changeClass(changeReturnDayDollar)" x-text="changeDollar(changeReturnDayDollar) + ' today'"></span>
</span>
```

Replace the NET RETURN % D/W/M badges (lines 96-100) with a single daily badge:
```html
<!-- OLD: lines 96-100 -->
<span class="portfolio-changes" x-show="hasReturnPctChanges" x-cloak>
    <span :class="changeClass(changeReturnDayPct)" x-text="'D:' + changePct(changeReturnDayPct)"></span>
    <span :class="changeClass(changeReturnWeekPct)" x-text="'W:' + changePct(changeReturnWeekPct)"></span>
    <span :class="changeClass(changeReturnMonthPct)" x-text="'M:' + changePct(changeReturnMonthPct)"></span>
</span>

<!-- NEW -->
<span class="portfolio-change-today" x-show="changeReturnDayPct != null" x-cloak>
    <span :class="changeClass(changeReturnDayPct)" x-text="changePct(changeReturnDayPct) + ' today'"></span>
</span>
```

### portal.css

Add new class after the existing `.portfolio-changes` styles:
```css
.portfolio-change-today {
    font-size: 0.6875rem;
    letter-spacing: 0.05em;
    margin-top: 0.25rem;
}
```

### common.js

No JS changes needed for FB1. The properties `changeReturnDayDollar`, `changeReturnDayPct` already exist.

---

## FB5: Chart Toggle + Rebalance Annotations

### dashboard.html

Replace the chart section (lines 109-114) with a wrapped version:
```html
<!-- Growth chart -->
<div class="growth-chart-section" x-show="hasGrowthData" x-cloak>
    <div class="growth-chart-controls">
        <label class="chart-toggle-label">
            <input type="checkbox" x-model="showChartBreakdown">
            <span>Show breakdown</span>
        </label>
    </div>
    <div class="growth-chart-container">
        <canvas id="growthChart"></canvas>
    </div>
</div>
<div class="growth-chart-ghost" x-show="!portfolioLoading && filteredHoldings.length > 0 && !hasGrowthData" x-cloak>
    <span class="text-muted">GROWTH CHART — NO DATA</span>
</div>
```

Note: The `x-show="hasGrowthData"` moves from `.growth-chart-container` to the new `.growth-chart-section` wrapper.

### common.js

**State properties** — add after `chartInstance: null` (line 221):
```js
showChartBreakdown: false,
```

**renderChart()** — modify datasets[1] and [2] to include `hidden` property:
```js
// Dataset 1 (Equity Value) — add hidden property:
hidden: !this.showChartBreakdown,

// Dataset 2 (Net Deposited) — add hidden property:
hidden: !this.showChartBreakdown,
```

**renderChart()** — add chart toggle watcher. After `this.chartInstance = new Chart(...)` (after line 598), add:
```js
// Watch for breakdown toggle changes
this.$watch('showChartBreakdown', (val) => {
    if (this.chartInstance) {
        this.chartInstance.data.datasets[1].hidden = !val;
        this.chartInstance.data.datasets[2].hidden = !val;
        this.chartInstance.update();
    }
});
```

**Rebalance annotations** — add annotation logic in renderChart(). Before `this.chartInstance = new Chart(...)`:
```js
// Compute rebalance markers (holding_count changes by 3+)
const rebalanceAnnotations = {};
for (let i = 1; i < this.growthData.length; i++) {
    const curr = this.growthData[i];
    const prev = this.growthData[i - 1];
    if (curr.holding_count != null && prev.holding_count != null) {
        const delta = Math.abs(curr.holding_count - prev.holding_count);
        if (delta >= 3) {
            rebalanceAnnotations['rebal' + i] = {
                type: 'line',
                xMin: labels[i],
                xMax: labels[i],
                borderColor: '#ccc',
                borderWidth: 1,
                borderDash: [4, 4],
                label: {
                    display: true,
                    content: 'Rebalance',
                    position: 'start',
                    font: { size: 9, family: "'IBM Plex Mono', monospace" },
                    color: '#888',
                    backgroundColor: 'transparent',
                },
            };
        }
    }
}
```

Then in the Chart options, add the annotation plugin config:
```js
plugins: {
    legend: { display: false },
    tooltip: { /* existing */ },
    annotation: Object.keys(rebalanceAnnotations).length > 0 ? {
        annotations: rebalanceAnnotations,
    } : undefined,
},
```

**IMPORTANT:** The Chart.js annotation plugin must be loaded. Add to `pages/partials/head.html`:
```html
<script src="https://cdn.jsdelivr.net/npm/chartjs-plugin-annotation@3.0.1/dist/chartjs-plugin-annotation.min.js"></script>
```
Place this AFTER the existing Chart.js CDN script tag.

### portal.css

Move `.growth-chart-container` styles to `.growth-chart-section`:
```css
.growth-chart-section {
    margin-bottom: 1.5rem;
    border: 2px solid #000;
    padding: 1rem;
}

.growth-chart-container {
    width: 100%;
    aspect-ratio: 3 / 1;
}

@media (max-width: 48rem) {
    .growth-chart-container {
        aspect-ratio: 2 / 1;
    }
}

.growth-chart-controls {
    margin-bottom: 0.75rem;
}

.chart-toggle-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8125rem;
    cursor: pointer;
}
```

Remove `border`, `padding`, and `margin-bottom` from `.growth-chart-container` (they move to `.growth-chart-section`).

---

## FB4: Breadth Bar Soft Colours

### portal.css

Replace breadth segment colours:
```css
/* OLD */
.breadth-rising { background: #2d8a4e; }
.breadth-flat { background: #888; }
.breadth-falling { background: #b54747; }

/* NEW — softer, muted palette */
.breadth-rising { background: #5a9e6f; }
.breadth-flat { background: #aaa; }
.breadth-falling { background: #c06060; }
```

Add `.breadth-segment` overflow hidden and gradient transitions:
```css
.breadth-segment {
    height: 100%;
    min-width: 0;
    position: relative;
    overflow: hidden;
}

/* Gradient transitions between segments */
.breadth-flat::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    width: 8px;
    height: 100%;
    background: linear-gradient(to right, #c06060, #aaa);
}

.breadth-rising::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    width: 8px;
    height: 100%;
    background: linear-gradient(to right, #aaa, #5a9e6f);
}
```

Note: The segment order in HTML is rising (left), flat (middle), falling (right). Wait — actually looking at the HTML:
```html
<div class="breadth-segment breadth-rising" ...></div>
<div class="breadth-segment breadth-flat" ...></div>
<div class="breadth-segment breadth-falling" ...></div>
```

But per fb_cfd57276 resolution notes: "Rising (bullish) should anchor to the RIGHT, Falling (bearish) to the LEFT." So the HTML order should be: falling, flat, rising. **The current HTML has it reversed.**

**Fix the HTML segment order in dashboard.html:**
```html
<!-- OLD (wrong order) -->
<div class="breadth-segment breadth-rising" :style="'width:' + (breadth?.rising_weight_pct || 0) + '%'"></div>
<div class="breadth-segment breadth-flat" :style="'width:' + (breadth?.flat_weight_pct || 0) + '%'"></div>
<div class="breadth-segment breadth-falling" :style="'width:' + (breadth?.falling_weight_pct || 0) + '%'"></div>

<!-- NEW (correct order: falling left, flat centre, rising right) -->
<div class="breadth-segment breadth-falling" :style="'width:' + (breadth?.falling_weight_pct || 0) + '%'"></div>
<div class="breadth-segment breadth-flat" :style="'width:' + (breadth?.flat_weight_pct || 0) + '%'"></div>
<div class="breadth-segment breadth-rising" :style="'width:' + (breadth?.rising_weight_pct || 0) + '%'"></div>
```

And update the gradient transitions accordingly:
```css
/* Gradient: falling→flat transition (on flat segment left edge) */
.breadth-flat::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    width: 8px;
    height: 100%;
    background: linear-gradient(to right, #c06060, #aaa);
}

/* Gradient: flat→rising transition (on rising segment left edge) */
.breadth-rising::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    width: 8px;
    height: 100%;
    background: linear-gradient(to right, #aaa, #5a9e6f);
}
```

Also update the breadth-counts order in dashboard.html to match (falling left, flat centre, rising right):
```html
<!-- OLD -->
<span x-text="(breadth?.rising_count || 0) + ' Rising'"></span>
<span x-text="(breadth?.flat_count || 0) + ' Flat'"></span>
<span x-text="(breadth?.falling_count || 0) + ' Falling'"></span>

<!-- NEW -->
<span x-text="(breadth?.falling_count || 0) + ' Falling'"></span>
<span x-text="(breadth?.flat_count || 0) + ' Flat'"></span>
<span x-text="(breadth?.rising_count || 0) + ' Rising'"></span>
```

---

## FB2: Breadth Bar Yesterday Comparison + Sync Timestamp

### dashboard.html

Modify the breadth summary row (lines 118-121):
```html
<!-- OLD -->
<div class="breadth-summary-row">
    <span class="breadth-trend" :class="trendArrowClass(breadth?.trend_score)" x-text="breadth?.trend_label || ''"></span>
    <span class="text-muted" x-text="breadth?.today_change != null ? fmtTodayChange(breadth.today_change) : ''"></span>
</div>

<!-- NEW -->
<div class="breadth-summary-row">
    <span class="breadth-trend" :class="trendArrowClass(breadth?.trend_score)" x-text="breadth?.trend_label || ''"></span>
    <span class="breadth-summary-right">
        <span :class="changeClass(breadth?.today_change)" x-text="breadth?.today_change != null ? fmtTodayChange(breadth.today_change) : ''"></span>
        <span class="text-muted" x-show="breadth?.yesterday_change != null" x-text="'(' + fmtTodayChange(breadth?.yesterday_change) + ' yesterday)'"></span>
        <span class="text-muted breadth-sync" x-show="lastSynced" x-text="'as at ' + fmtSyncedTime(lastSynced)"></span>
    </span>
</div>
```

### common.js

Add new helper `fmtSyncedTime()` after `fmtSynced()`:
```js
fmtSyncedTime(utcStr) {
    if (!utcStr) return '';
    try {
        const d = new Date(utcStr);
        if (isNaN(d.getTime())) return '';
        return d.toLocaleTimeString('en-AU', { hour: '2-digit', minute: '2-digit', hour12: false });
    } catch { return ''; }
},
```

### portal.css

```css
.breadth-summary-right {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8125rem;
}

.breadth-sync {
    font-size: 0.6875rem;
}
```

---

## FB3: Per-Holding Trend Rows in Breadth Section

### dashboard.html

**Add per-holding rows inside the breadth section** (after the breadth-summary-row, before the breadth-bar):
```html
<div class="breadth-bar-section" x-show="hasBreadth" x-cloak>
    <div class="breadth-summary-row">
        <!-- ... (FB2 content) ... -->
    </div>

    <!-- Per-holding trend rows (NEW) -->
    <div class="breadth-holdings">
        <template x-for="h in filteredHoldings.filter(x => x.holding_value_market > 0)" :key="h.ticker">
            <div class="breadth-holding-row">
                <span class="breadth-holding-ticker" x-text="h.ticker"></span>
                <span :class="trendArrowClass(h.trend_score)" x-text="trendArrow(h.trend_score)"></span>
                <span class="text-muted breadth-holding-label" x-text="h.trend_label || ''"></span>
                <span class="breadth-holding-change" :class="changeClass(holdingTodayChange(h))" x-text="fmtTodayChange(holdingTodayChange(h))"></span>
            </div>
        </template>
    </div>

    <div class="breadth-separator"></div>

    <!-- Portfolio summary line -->
    <div class="breadth-portfolio-row">
        <span class="breadth-holding-ticker">PORTFOLIO</span>
        <span :class="trendArrowClass(breadth?.trend_score)" x-text="trendArrow(breadth?.trend_score)"></span>
        <span class="text-muted breadth-holding-label" x-text="breadth?.trend_label || ''"></span>
        <span class="breadth-holding-change" :class="changeClass(breadth?.today_change)" x-text="breadth?.today_change != null ? fmtTodayChange(breadth.today_change) : ''"></span>
    </div>

    <div class="breadth-bar">
        <!-- segments (FB4 order) -->
    </div>
    <div class="breadth-counts">
        <!-- counts (FB4 order) -->
    </div>
</div>
```

**Remove the movement sub-rows from holdings table** (delete lines 165-172):
```html
<!-- DELETE these lines entirely -->
<tr class="holding-movement-row">
    <td :class="trendArrowClass(h.trend_score)" x-text="trendArrow(h.trend_score)"></td>
    <td class="text-muted" x-text="h.trend_label || ''"></td>
    <td :class="changeClass(h.yesterday_price_change_pct)" x-text="'D:' + changePct(h.yesterday_price_change_pct)"></td>
    <td :class="changeClass(h.last_week_price_change_pct)" x-text="'W:' + changePct(h.last_week_price_change_pct)"></td>
    <td :class="changeClass(h.last_month_price_change_pct)" x-text="'M:' + changePct(h.last_month_price_change_pct)"></td>
    <td class="text-right" :class="changeClass(holdingTodayChange(h))" x-text="fmtTodayChange(holdingTodayChange(h))"></td>
</tr>
```

### portal.css

Add new breadth holding styles:
```css
.breadth-holdings {
    margin-bottom: 0.75rem;
}

.breadth-holding-row,
.breadth-portfolio-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
}

.breadth-portfolio-row {
    font-weight: 700;
    margin-bottom: 0.75rem;
}

.breadth-holding-ticker {
    font-weight: 700;
    min-width: 5rem;
}

.breadth-holding-label {
    flex: 1;
}

.breadth-holding-change {
    text-align: right;
    min-width: 5rem;
}

.breadth-separator {
    border-top: 1px solid #ccc;
    margin: 0.5rem 0;
}
```

Remove `.holding-movement-row` CSS rules (if any exist in portal.css — check and remove).

---

## Test Updates

### tests/ui/dashboard_test.go

**TestDashboardBreadthBar** — update to also check:
- Segment order: first segment has class `breadth-falling`, last has `breadth-rising`
- Per-holding rows exist inside `.breadth-holdings`
- `.breadth-portfolio-row` exists with "PORTFOLIO" text
- `.breadth-separator` exists between holdings and portfolio row

**TestDashboardHoldingTrendArrows** — REWRITE entirely:
- Old test checks `.holding-movement-row` in holdings table (these are removed)
- New test checks `.breadth-holding-row` elements inside `.breadth-holdings`
- Verify each row has: ticker text, trend arrow, trend label, today change
- Verify color classes on trend arrows

**TestDashboardGrowthChart** — update to check:
- `.growth-chart-section` wrapper exists
- `.chart-toggle-label` exists with "Show breakdown" text
- Chart initially has only 1 visible dataset (Portfolio Value)
- After clicking checkbox, chart has 3 visible datasets

**TestDashboardPortfolioSummary** — update:
- Row 2 NET RETURN $ should show single daily badge with "today" suffix, not D/W/M
- Row 2 NET RETURN % should show single daily badge with "today" suffix, not D/W/M
- `.portfolio-change-today` class should exist

### internal/handlers/dashboard_stress_test.go

Update any stress tests that check for:
- W/M return change bindings (remove those checks)
- `holding-movement-row` references (remove or update)
- Add checks for new classes: `breadth-holding-row`, `breadth-portfolio-row`, `chart-toggle-label`, `portfolio-change-today`

---

## Edge Cases

1. **No holdings data** — `hasBreadth` is false, entire breadth section hidden. No per-holding rows shown.
2. **No changes data** — `changeReturnDayDollar == null` hides the daily badge. Safe.
3. **No yesterday_change from server** — `breadth?.yesterday_change != null` is false, yesterday comparison hidden. Safe.
4. **No lastSynced** — `x-show="lastSynced"` hides sync timestamp. Safe.
5. **No growth data** — `hasGrowthData` is false, chart section hidden. Ghost state shown.
6. **No holding_count in timeline** — rebalance annotations object is empty, annotation plugin does nothing.
7. **All holdings closed** — `filteredHoldings.filter(x => x.holding_value_market > 0)` returns empty, no holding rows in breadth.
8. **chartjs-plugin-annotation not loaded** — Chart.js ignores unknown plugin config, no crash.

---

## Files Modified Summary

| File | Changes |
|------|---------|
| `pages/dashboard.html` | Row 2 daily badges, chart section wrapper+toggle, breadth segment order, breadth holding rows, breadth portfolio row, remove movement sub-rows |
| `pages/static/common.js` | `showChartBreakdown` property, renderChart() hidden datasets + watcher + annotations, `fmtSyncedTime()` helper |
| `pages/static/css/portal.css` | `.portfolio-change-today`, chart section/toggle styles, soft breadth colours + gradients, breadth holding/portfolio/separator styles, remove movement-row styles |
| `pages/partials/head.html` | Add chartjs-plugin-annotation CDN script |
| `tests/ui/dashboard_test.go` | Update breadth bar test, rewrite trend arrows test, update chart test, update summary test |
| `internal/handlers/dashboard_stress_test.go` | Update binding checks for new classes, remove movement-row checks |
