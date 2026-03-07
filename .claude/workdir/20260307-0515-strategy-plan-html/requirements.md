# Requirements: Strategy & Plan Pages — Read-Only HTML Rendering

## Overview

Convert the Strategy page from editable textareas with SAVE buttons into a read-only rendered view. Strategy content (markdown in `.notes`) is rendered as formatted HTML using marked.js. Plan content (structured `.items[]` array) is rendered as a styled HTML table. A banner directs users to the MCP chat interface for edits.

## Feedback Items
- fb_26513249: Strategy/Plan pages show editable text boxes — should render as formatted HTML with a note to use Claude/MCP
- fb_5c137697: Strategy/Plan pages render raw markdown — parse to HTML, add banner for AI-assisted updates
- fb_7e014841: Admin MCP review page — DEFERRED (see bottom)

---

## Feature A: Strategy & Plan Read-Only HTML Rendering

### A1. Add marked.js CDN to head template

**File:** `pages/partials/head.html`

Add marked.js CDN script tag after the chart.js line, before Alpine:

```html
<script defer src="https://cdn.jsdelivr.net/npm/marked@15/marked.min.js"></script>
```

Use `defer` to match the existing pattern for chart.js and Alpine.

### A2. Replace strategy.html template content

**File:** `pages/strategy.html`

Replace the two `section.panel-headed` blocks containing textareas and SAVE buttons with:

**Info banner** (insert after portfolio selector, inside the `x-show="selected"` guard):

```html
<!-- Info banner -->
<div class="info-banner" x-show="selected" x-cloak>
    To update your strategy or plan, discuss changes with Claude (AI) via the MCP chat interface.
</div>
```

**Strategy section:**

```html
<section class="panel-headed" x-show="selected" x-cloak>
    <div class="panel-header">STRATEGY</div>
    <div class="panel-content strategy-rendered" x-html="strategyHtml"></div>
</section>
```

**Plan section:**

```html
<section class="panel-headed" x-show="selected" x-cloak>
    <div class="panel-header">PLAN</div>
    <div class="panel-content">
        <div x-show="planItems.length === 0" class="text-muted">No plan items defined.</div>
        <table class="plan-table" x-show="planItems.length > 0">
            <thead>
                <tr>
                    <th>Status</th>
                    <th>Action</th>
                    <th>Ticker</th>
                    <th>Description</th>
                    <th>Deadline</th>
                    <th>Notes</th>
                </tr>
            </thead>
            <tbody>
                <template x-for="item in planItems" :key="item.id">
                    <tr :class="item.status === 'completed' ? 'plan-completed' : ''">
                        <td><span class="plan-status" :class="'plan-status-' + item.status" x-text="item.status"></span></td>
                        <td><span class="plan-action" :class="'plan-action-' + (item.action || '').toLowerCase()" x-text="item.action || '-'"></span></td>
                        <td x-text="item.ticker || '-'" style="white-space:nowrap;"></td>
                        <td x-text="item.description"></td>
                        <td x-text="item.deadline ? new Date(item.deadline).toLocaleDateString('en-AU', {day:'numeric',month:'short',year:'numeric'}) : '-'" style="white-space:nowrap;"></td>
                        <td x-text="item.notes || '-'" class="plan-notes-cell"></td>
                    </tr>
                </template>
            </tbody>
        </table>
    </div>
</section>
```

SSR data block remains unchanged.

### A3. Update Alpine component `portfolioStrategy()` in common.js

**File:** `pages/static/common.js` (lines 1157-1293)

**Replace data properties:**
```javascript
// OLD:
strategy: '',
plan: '',
// NEW:
strategyHtml: '',
planItems: [],
```

**Add helper methods:**

```javascript
renderStrategy(data) {
    const notes = data.notes || '';
    if (notes && typeof marked !== 'undefined') {
        this.strategyHtml = marked.parse(notes);
    } else if (notes) {
        this.strategyHtml = '<pre>' + notes.replace(/</g, '&lt;') + '</pre>';
    } else {
        this.strategyHtml = '<p class="text-muted">No strategy defined.</p>';
    }
},

renderPlan(data) {
    this.planItems = data.items || [];
},
```

**Update init() SSR path:**
```javascript
// OLD:
if (ssrData.strategy) {
    const sd = ssrData.strategy;
    this.strategy = sd.notes || JSON.stringify(sd.strategy || sd, null, 2);
}
if (ssrData.plan) {
    const pd = ssrData.plan;
    this.plan = pd.notes || JSON.stringify(pd.plan || pd, null, 2);
}
// NEW:
if (ssrData.strategy) {
    this.renderStrategy(ssrData.strategy);
}
if (ssrData.plan) {
    this.renderPlan(ssrData.plan);
}
```

**Update loadPortfolio() success branches:**
```javascript
// OLD:
if (strategyRes.ok) {
    const strategyData = await strategyRes.json();
    this.strategy = strategyData.notes || JSON.stringify(strategyData.strategy || strategyData, null, 2);
} else {
    this.strategy = '';
}
if (planRes.ok) {
    const planData = await planRes.json();
    this.plan = planData.notes || JSON.stringify(planData.plan || planData, null, 2);
} else {
    this.plan = '';
}
// NEW:
if (strategyRes.ok) {
    this.renderStrategy(await strategyRes.json());
} else {
    this.strategyHtml = '<p class="text-muted">No strategy defined.</p>';
}
if (planRes.ok) {
    this.renderPlan(await planRes.json());
} else {
    this.planItems = [];
}
```

**Remove saveStrategy() and savePlan() methods entirely.** Keep toggleDefault() as-is.

### A4. Add CSS for rendered content and info banner

**File:** `pages/static/css/portal.css`

Add after the existing banner section:

```css
/* Info banner */
.info-banner {
    border: 2px solid #555;
    padding: 1rem 1.5rem;
    margin-bottom: 2rem;
    font-size: 0.875rem;
    color: #ccc;
}

/* Strategy rendered markdown */
.strategy-rendered {
    line-height: 1.6;
    color: #ccc;
}
.strategy-rendered h2 {
    font-size: 1.1rem;
    font-weight: 700;
    margin: 1.5rem 0 0.5rem;
    color: #fff;
    border-bottom: 1px solid #333;
    padding-bottom: 0.25rem;
}
.strategy-rendered h3 {
    font-size: 0.95rem;
    font-weight: 700;
    margin: 1.25rem 0 0.4rem;
    color: #fff;
}
.strategy-rendered p {
    margin: 0.5rem 0;
}
.strategy-rendered ul, .strategy-rendered ol {
    margin: 0.5rem 0 0.5rem 1.5rem;
}
.strategy-rendered li {
    margin: 0.25rem 0;
}
.strategy-rendered strong {
    color: #fff;
}
.strategy-rendered pre {
    background: #111;
    padding: 1rem;
    overflow-x: auto;
    font-size: 0.8rem;
}
.strategy-rendered code {
    background: #222;
    padding: 0.1rem 0.3rem;
    font-size: 0.85em;
}

/* Plan table */
.plan-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
}
.plan-table th {
    text-align: left;
    padding: 0.5rem 0.75rem;
    border-bottom: 2px solid #333;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: #888;
}
.plan-table td {
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid #222;
    vertical-align: top;
}
.plan-completed {
    opacity: 0.5;
}
.plan-status {
    font-size: 0.75rem;
    text-transform: uppercase;
    font-weight: 700;
}
.plan-status-completed { color: #2d8a4e; }
.plan-status-pending { color: #d97706; }
.plan-action {
    font-size: 0.75rem;
    font-weight: 700;
}
.plan-action-buy { color: #2d8a4e; }
.plan-action-sell { color: #c53030; }
.plan-action-watch { color: #d97706; }
.plan-notes-cell {
    max-width: 300px;
    font-size: 0.8rem;
    color: #999;
}
```

### A5. No changes to Go handler

`internal/handlers/strategy.go` requires ZERO changes. SSR already embeds the correct JSON. The rendering shift is purely client-side.

### A6. Update UI tests

**File:** `tests/ui/strategy_test.go`

**Modify TestStrategyEditor:**
- Change selector from `textarea.portfolio-editor` to `.strategy-rendered`
- Check for rendered strategy div visibility instead of textarea

**Modify TestStrategyPlanEditor:**
- Check for `.plan-table` visibility instead of counting textareas
- Keep PLAN panel-header check

**Add TestStrategyInfoBanner:**
- Navigate to `/strategy`, wait for Alpine
- Assert `.info-banner` is visible
- Assert it contains text "discuss changes with Claude"

**Add TestStrategyNoSaveButtons:**
- Navigate to `/strategy`, wait for Alpine
- Assert zero elements matching button containing text "SAVE"

---

## Feature B: Admin MCP Review Page — DEFERRED

**Status:** DEFERRED

**Rationale:** The MCP tool suite already provides direct API access to all portfolio data:
- `strategy_get` — full strategy data including notes
- `plan_get` — all plan items with status/actions
- `portfolio_get` — holdings and metrics
- `watchlist_get` — watchlist items
- `portfolio_get_summary` — portfolio summary
- `portfolio_generate_report` — full portfolio report

Claude can already read, analyze, and act on all this data through MCP tools without needing an HTML rendering intermediary. Building an admin page adds maintenance surface, security surface (XSS with arbitrary user data), and a handler/template/route/tests for a page whose primary consumer already has better access.

---

## Implementation Sequence

1. A4 — Add CSS (no functional impact)
2. A1 — Add marked.js CDN (no functional impact)
3. A3 — Update portfolioStrategy() in common.js (core logic)
4. A2 — Update strategy.html template (activates new rendering)
5. A6 — Update UI tests

Steps 1+2 can be parallel. Step 4 depends on 3. Step 5 depends on 4.

## Files Changed

| File | Change |
|------|--------|
| `pages/partials/head.html` | Add marked.js CDN script tag |
| `pages/strategy.html` | Replace textareas/SAVE with info-banner + x-html + plan table |
| `pages/static/common.js` | Rewrite portfolioStrategy(): remove saves, add render methods |
| `pages/static/css/portal.css` | Add info-banner, strategy-rendered, plan-table CSS |
| `tests/ui/strategy_test.go` | Update 2 existing tests, add 2 new tests |

## Risks

1. **marked.js CDN:** Fallback to `<pre>` with escaped content if CDN unavailable
2. **XSS:** Strategy notes are user-owned data via authenticated API, not third-party input. Low risk.
3. **Empty data:** Template handles missing fields with fallbacks
4. **Portfolio switching:** loadPortfolio() still fetches fresh data, calls renderStrategy/renderPlan
