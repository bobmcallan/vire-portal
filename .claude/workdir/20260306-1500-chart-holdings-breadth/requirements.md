# Requirements: Chart Gross Contributions, Holdings Alignment, Breadth Per-Ticker

## Scope

**Changes:**
- `pages/static/common.js` — renderChart() adds Gross Contributions line; new `breadthSegments` getter
- `pages/dashboard.html` — breadth bar uses x-for per-ticker segments; holding sub-row class added
- `pages/static/css/portal.css` — holding-movement-row top padding; remove breadth gradient pseudo-elements
- `internal/handlers/dashboard_stress_test.go` — update stress tests
- `tests/ui/dashboard_test.go` — update UI tests

**No changes to:** server/API, Go handlers, routes, auth, other pages.

---

## 1. Chart: Gross Contributions Dotted Line

### common.js — renderChart() (around line 525)

After the `capitalLine` definition, add:
```javascript
const grossLine = this.grossContributions > 0
    ? labels.map(() => this.grossContributions)
    : null;
```

After the "Net Deposited" dataset (around line 574), conditionally push:
```javascript
if (grossLine) {
    datasets.push({
        label: 'Gross Contributions',
        data: grossLine,
        borderColor: '#888',
        borderWidth: 1,
        borderDash: [2, 2],
        pointRadius: 0,
        pointHoverRadius: 4,
        fill: false,
        tension: 0,
        hidden: !this.showChartBreakdown,
    });
}
```

Key: conditional push (same pattern as MA datasets at lines 578-618). Hidden behind `showChartBreakdown` toggle. Gray `#888` to distinguish from Net Deposited `#000`. Tooltip already handles all datasets generically.

### Edge cases
- `grossContributions === 0` or null: grossLine is null, dataset not added
- Single data point: flat line renders as single point
- Gross > portfolio value: line appears above — correct behavior

---

## 2. Holdings: Sub-Row Alignment and Spacing

### dashboard.html (line 186)

Add class to the td:
```html
<td class="holding-movement-content" colspan="6">
```

### portal.css

Update `.holding-movement-row td` (line 1203):
```css
.holding-movement-row td {
    padding-top: 0.25rem;           /* was: 0 — adds spacing above sub-row */
    padding-bottom: 0.5rem;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
    border-bottom: 2px solid #000;
}
```

Keep `colspan="6"` so the border spans full width. The content stays left-aligned (inherits from table).

---

## 3. Breadth Bar: Per-Ticker Color Coding

### common.js — new `breadthSegments` getter (after computeBreadth around line 892)

```javascript
get breadthSegments() {
    const active = this.holdings.filter(h => h.holding_value_market > 0);
    if (active.length === 0) return [];
    const totalWeight = active.reduce((sum, h) => sum + (h.holding_value_market || 0), 0);
    if (totalWeight === 0) return [];

    const segments = active.map(h => {
        const score = h.trend_score || 0;
        let status;
        if (score > 0.1) status = 'rising';
        else if (score < -0.1) status = 'falling';
        else status = 'flat';
        return {
            ticker: h.ticker,
            status: status,
            weight_pct: (h.holding_value_market / totalWeight) * 100,
        };
    });

    // Sort: falling first, then flat, then rising (secondary: ticker alpha)
    const order = { falling: 0, flat: 1, rising: 2 };
    segments.sort((a, b) => order[a.status] - order[b.status] || a.ticker.localeCompare(b.ticker));

    return segments;
},
```

Uses same thresholds as `computeBreadth()` (> 0.1 rising, < -0.1 falling, else flat).

**IMPORTANT:** This is a getter, NOT a regular method. It must be defined inside the Alpine component's return object using `get breadthSegments()` syntax. Alpine.js will re-evaluate it reactively when `this.holdings` changes.

### dashboard.html — breadth bar (lines 142-146)

Replace the 3 static segments:
```html
<div class="breadth-bar">
    <template x-for="seg in breadthSegments" :key="seg.ticker">
        <div class="breadth-segment"
             :class="'breadth-' + seg.status"
             :style="'width:' + (seg.weight_pct || 0) + '%'"
             :title="seg.ticker + ' ' + seg.status + ' (' + (seg.weight_pct || 0).toFixed(1) + '%)'">
        </div>
    </template>
</div>
```

Reuses existing color classes: `.breadth-falling` (#c06060), `.breadth-flat` (#aaa), `.breadth-rising` (#5a9e6f).

### portal.css — remove gradient pseudo-elements

Delete `.breadth-flat::before` (lines 1162-1170) and `.breadth-rising::before` (lines 1173-1181). Per-ticker segments don't need cross-segment gradients.

### Edge cases
- Single holding: one segment at 100% width
- All same status: bar appears as single solid color
- No active holdings: breadthSegments returns [], x-for renders nothing, hasBreadth still gates section
- Very small weight (<1%): renders as sub-pixel or invisible — acceptable
- No trend_score: defaults to 0, classified as "flat"

---

## 4. Stress Tests (dashboard_stress_test.go)

### StressChartBreakdownPropertyDeclared (line 1664)
Add checks:
- `"Gross Contributions"` label string exists in common.js
- `grossLine` variable exists in common.js

### StressBreadthBarStructure (line 1696)
Update:
- Add check: `x-for="seg in breadthSegments"` exists in template
- Add check: `:class="'breadth-' + seg.status"` exists in template

### New: StressBreadthSegmentsPropertyDeclared
- Read common.js, verify `breadthSegments` getter exists
- Verify it references `trend_score` and sorts by status order

### StressBreadthSegmentOrder (if it checks static DOM order)
- Update to check that `breadthSegments` getter has sort logic rather than static class order

---

## 5. UI Tests (tests/ui/dashboard_test.go)

### TestDashboardGrowthChart
- Update dataset count check: accept 3 or 4 (3 without gross contributions, 4 with)
- Update label check: accept optional "Gross Contributions" in labels array

### TestDashboardBreadthBar
- Change segment count: from exactly 3 to >= 1
- Verify all segments have one of: breadth-falling, breadth-flat, breadth-rising class
- Verify segments have title attributes containing ticker names

### Holdings sub-row
- Verify `.holding-movement-row` exists with content
