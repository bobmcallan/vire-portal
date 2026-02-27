# Requirements: Dashboard Growth Chart & Capital Timeline

## Feedback Items

| ID | Summary | Portal Scope |
|----|---------|-------------|
| fb_ca924779 | Portfolio value vs invested capital chart with P&L gap | **Primary** — growth chart |
| fb_da8cabc1 | EPIC: Capital allocation timeline | **Partial** — chart TotalValue/TotalCost/net_capital_deployed; cash/accumulate timeline deferred to server |
| fb_1346e9a3 | Dashboard capital metrics + refresh | **Done** — capital metrics and refresh implemented in prior session |
| fb_cafb4fa0 | Portfolio indicators + raw time series | **Done** — indicators displayed; time_series available via `/history` endpoint |
| fb_742053d8 | Performance goal tracker | **Done** — capital performance visible; goal config deferred to server |

## Scope

Add a portfolio growth chart to the dashboard showing TotalValue vs TotalCost over time, with net_capital_deployed as a reference line. Chart rendered using Chart.js (lightweight, monochrome-compatible).

## Data Source

**Endpoint:** `GET /api/portfolios/{name}/history`
- Returns `{ portfolio, format, data_points, count }`
- `data_points[]` has: Date, TotalValue, TotalCost, NetReturn, NetReturnPct, HoldingCount
- Same underlying `GetDailyGrowth()` as compliance review, but lightweight (no signals/analysis)
- ~65 data points for current portfolio

**Fallback:** If `/history` returns error, chart section hidden gracefully.

**Capital reference:** `net_capital_deployed` from existing `/api/portfolios/{name}` response (already parsed as `capitalInvested`).

## Anomaly Filter

Growth data has known price corruption (ACDC on Feb 24-26 — TotalValue jumps from $360k to $17M). Filter rule:
- If day-over-day TotalValue change exceeds 50%, use previous day's value
- Applied client-side before rendering

## Chart Design (Monochrome)

Follows portal design system: IBM Plex Mono, #000/#fff/#888, no border-radius, no box-shadow.

**Chart.js configuration:**
- Line chart, no fill
- Line 1: TotalValue — 2px solid #000 (Portfolio Value)
- Line 2: TotalCost — 1px dashed #888 (Cost Basis)
- Line 3: net_capital_deployed — 1px dotted #000 (Capital Deployed, horizontal)
- X-axis: dates (MMM DD format)
- Y-axis: currency values
- Tooltip: date + all values formatted
- Grid: #eee horizontal lines only
- Font: IBM Plex Mono
- No legend box — inline labels
- Aspect ratio: responsive, ~3:1 on desktop

**Chart container:** Below indicators row, above holdings table. Conditional on growth data existing.

## Files to Change

| File | Change |
|------|--------|
| `pages/partials/head.html` | Add Chart.js CDN script |
| `pages/dashboard.html` | Add chart canvas container section |
| `pages/static/common.js` | Add growth data fetch, anomaly filter, chart rendering in portfolioDashboard() |
| `pages/static/css/portal.css` | Add chart container styles |
| `tests/ui/dashboard_test.go` | Add growth chart UI test |
| `README.md` | Update dashboard features |

## Approach

1. Add Chart.js v4 via CDN to `head.html` (defer load)
2. Add `<canvas>` element in dashboard.html, wrapped in conditional div
3. In `loadPortfolio()`, non-blocking fetch to `/api/portfolios/{name}/history`
4. Apply anomaly filter to growth data points
5. Render Chart.js line chart with monochrome styling
6. On `refreshPortfolio()`, re-fetch history and update chart
7. On portfolio change, destroy old chart and render new one
8. Chart is hidden when no growth data available

## Out of Scope (Server-Side)

- Cash balance / accumulate balance daily breakdown (fb_da8cabc1)
- Capital flow adjustments for daily/weekly change (fb_e69d6635)
- Performance goal tracking config (fb_742053d8)
- Price-only refresh (fb_1346e9a3 item 3)
- EMA/RSI value fixes (fb_cafb4fa0)
