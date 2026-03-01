# Requirements: Dashboard Chart Alignment + Capital Timeline

**Status:** Ready for implementation
**Work dir:** `.claude/workdir/20260301-1910-dashboard-chart-fix/`

## Problem

Three issues visible on the dashboard:

1. **Available Cash and Capital Gain % blank** — The vire service returns `available_cash` and `capital_gain_pct` in the portfolio JSON. The portal code already parses them. The portal needs to be rebuilt/restarted.

2. **Chart not aligned to dashboard numbers** — The dashboard's TOTAL VALUE card shows `total_value` ($479K, including cash), but the chart's Portfolio Value line uses growth history `TotalValue` which is equity-only (~$413K). The chart should use `TotalCapital` (equity + cash) from the capital timeline.

3. **Capital Deployed is a flat line** — The chart draws Capital Deployed as `this.capitalInvested` repeated for every date. The actual net deployed changed over time (from $462K → $451K → $403K → $405K) as deposits/withdrawals occurred. The chart should use historical `NetDeployed` from the capital timeline.

## Scope

- Replace the growth data source from `/history` to `/capital-timeline` endpoint
- Update chart rendering to use `TotalCapital` and `NetDeployed`
- Build and restart the portal
- **NOT** in scope: vire-server changes (feedback already submitted as fb_2d9bad2f)

## Data Source Change

### Current: `/api/portfolios/{name}/history`

```json
{
  "data_points": [
    {"Date": "...", "TotalValue": 413205.93, "TotalCost": 387744.41, "GainLoss": ..., "HoldingCount": 10}
  ]
}
```

- `TotalValue` = equity holdings value only (no cash)
- No `NetDeployed` field

### New: `/api/portfolios/{name}/capital-timeline`

```json
{
  "data_points": [
    {
      "Date": "2026-02-26T00:00:00Z",
      "TotalValue": 413205.93,
      "TotalCost": 387744.41,
      "CashBalance": 65473.66,
      "TotalCapital": 478679.59,
      "NetDeployed": 405813.62,
      "HoldingCount": 10
    }
  ],
  "count": 68,
  "format": "weekly",
  "portfolio": "SMSF"
}
```

- `TotalCapital` = TotalValue + CashBalance (matches dashboard TOTAL VALUE)
- `NetDeployed` = cumulative deposits minus withdrawals (changes over time)

## File Changes

### 1. `pages/static/common.js`

#### a. `fetchGrowthData()` (line ~312)

Change the fetch URL from:
```javascript
'/api/portfolios/' + encodeURIComponent(this.selected) + '/history'
```
to:
```javascript
'/api/portfolios/' + encodeURIComponent(this.selected) + '/capital-timeline'
```

Parse the response the same way (uses `data.data_points || []` — same structure).

#### b. `filterAnomalies()` (line ~334)

Update field references from `TotalValue` to `TotalCapital`:
```javascript
// Before:
if (prev.TotalValue > 0) {
    const change = Math.abs(p.TotalValue - prev.TotalValue) / prev.TotalValue;
    if (change > 0.5) {
        p.TotalValue = prev.TotalValue;
    }
}

// After:
if (prev.TotalCapital > 0) {
    const change = Math.abs(p.TotalCapital - prev.TotalCapital) / prev.TotalCapital;
    if (change > 0.5) {
        p.TotalCapital = prev.TotalCapital;
    }
}
```

#### c. `renderChart()` (line ~353)

Update the three data arrays:
```javascript
// Before:
const totalValues = this.growthData.map(p => p.TotalValue);
const totalCosts = this.growthData.map(p => p.TotalCost);
const capitalLine = this.growthData.map(() => this.capitalInvested);

// After:
const totalValues = this.growthData.map(p => p.TotalCapital);
const totalCosts = this.growthData.map(p => p.TotalCost);
const capitalLine = this.growthData.map(p => p.NetDeployed);
```

#### d. Chart dataset label update

Change the "Capital Deployed" dataset styling to make it more prominent since it's now a meaningful time series (not just a reference line):
```javascript
{
    label: 'Net Deposited',
    data: capitalLine,
    borderColor: '#000',
    borderWidth: 1,
    borderDash: [2, 2],
    pointRadius: 0,
    pointHoverRadius: 4,  // Changed from 0 — now meaningful data
    fill: false,
    tension: 0,
}
```

### 2. No Go struct changes needed

The portal is a transparent proxy. The GrowthDataPoint struct in `internal/vire/models/portfolio.go` is for documentation only — the JS parses JSON directly from the proxied response.

### 3. Tests

#### `tests/ui/dashboard_test.go`

The `TestDashboardGrowthChart` test verifies chart visibility (canvas element visible, not hidden). It does NOT verify chart data content. **No test changes needed** unless the chart canvas ID or container class changes (they don't).

#### `internal/handlers/dashboard_stress_test.go`

Tests dashboard HTML rendering, not chart data. **No changes needed.**

#### New: Growth chart data source test

If the UI test mock server serves growth data, it may need to serve capital-timeline data instead. Check `tests/ui/dashboard_test.go` for mock setup.

## Edge Cases

1. **capital-timeline endpoint not available (404)**: Fall back to empty chart (same as current behavior when history fails). `hasGrowthData` will be `false`.

2. **No data points**: The `filterAnomalies()` function already returns `[]` for empty input.

3. **Missing TotalCapital/NetDeployed fields**: Use fallback: `p.TotalCapital || p.TotalValue || 0` and `p.NetDeployed || this.capitalInvested || 0`.

4. **Old cached responses**: The `vireStore` has a 30-second TTL. After portal restart, old cache is cleared. Browser cache should be busted by the rebuilt static assets.

## Build & Verify

After implementation:
1. `go test ./...` — all tests pass
2. `go vet ./...` — clean
3. `./scripts/run.sh restart` — portal rebuilds and starts
4. Verify dashboard shows Available Cash and Capital Gain % values
5. Verify chart Portfolio Value line aligns with dashboard TOTAL VALUE
6. Verify Capital Deployed line is NOT flat (changes over time)
