# Requirements: Dashboard Capital Performance & Indicators

**Date:** 2026-02-27
**Requested:** Implement three feedback items — fb_1346e9a3, fb_cafb4fa0, fb_742053d8

## Feedback Items

### fb_1346e9a3 — Dashboard Capital Performance Metrics + Refresh
1. Show Capital Invested (net_capital_deployed from capital_performance)
2. Show Capital Gain (current_portfolio_value - net_capital_deployed)
3. Show Simple Return % and Annualized Return %
4. Price refresh on page load via force_refresh query param

### fb_cafb4fa0 — Portfolio Indicators Display
- Display portfolio-level indicators (trend, RSI signal) from indicators API
- NOTE: EMA values are known to be buggy server-side — display trend/RSI only
- Time series exposure is a server-side change (out of scope)

### fb_742053d8 — Capital Performance Visibility
- Overlaps with fb_1346e9a3 — capital performance data already returned by API
- Goal tracking config is a server-side feature (out of scope for portal)
- Portal contribution: make capital performance metrics prominent on dashboard

## Scope

### In Scope (Portal Changes)
- Parse `capital_performance` from existing `/api/portfolios/{name}` response
- Display: Capital Invested, Capital Gain $, Simple Return %, Annualized Return %
- Add Refresh button using `?force_refresh=true` on portfolio API
- Fetch `/api/portfolios/{name}/indicators` and display trend + RSI signal
- Update dashboard UI tests for new elements
- Update CSS for new sections

### Out of Scope (Requires vire-server changes)
- Exposing raw daily portfolio value time series (fb_cafb4fa0 server-side)
- Fixing EMA/RSI calculation bugs (fb_cafb4fa0 server-side)
- Auto-derive capital from trade history (fb_742053d8 server-side)
- Performance goal configuration (fb_742053d8 server-side)
- Price-only refresh endpoint (force_refresh does full Navexa sync)

## Approach

### Data Sources
1. **Capital performance** — already included in `/api/portfolios/{name}` response as `capital_performance` field:
   ```json
   {
     "total_deposited": 488585.09,
     "total_withdrawn": 60600,
     "net_capital_deployed": 427985.09,
     "current_portfolio_value": 423784.84,
     "simple_return_pct": -0.98,
     "annualized_return_pct": -4.91,
     "first_transaction_date": "2025-12-19T00:00:00Z",
     "transaction_count": 15
   }
   ```
   The dashboard JS currently ignores this field. Just parse it in `loadPortfolio()`.

2. **Portfolio indicators** — separate API call to `/api/portfolios/{name}/indicators`:
   ```json
   {
     "trend": "bullish",
     "rsi": 98.56,
     "rsi_signal": "overbought",
     "data_points": 65
   }
   ```

3. **Refresh** — existing `?force_refresh=true` query param on portfolio endpoint triggers full Navexa sync.

### Dashboard Layout
Current layout (1 row, 4 items):
```
TOTAL VALUE | TOTAL COST | NET RETURN $ | NET RETURN %
```

New layout (2 rows + indicators + refresh):
```
[Portfolio Select ▼]  [Default ☐]  [↻ Refresh]

TOTAL VALUE | TOTAL COST | NET RETURN $ | NET RETURN %
────────────────────────────────────────────────────────
CAPITAL INVESTED | CAPITAL GAIN $ | SIMPLE RETURN % | ANNUALIZED %
────────────────────────────────────────────────────────
TREND: BULLISH   RSI: 98.6 OVERBOUGHT   65 DATA POINTS
```

- Row 1: existing cost-basis metrics (unchanged)
- Row 2: capital performance metrics (new, only shown when capital_performance exists)
- Row 3: portfolio indicators (new, only shown when indicators data available)
- Refresh button: in portfolio header row, triggers force_refresh + cache invalidation

### Design Constraints
- Monochrome only (#000, #fff, #888)
- No border-radius, no box-shadow
- IBM Plex Mono font
- Follow existing `.portfolio-summary` pattern for layout
- Gain/loss colors: #2d8a4e (green) / #a33 (red)

## Files Expected to Change

| File | Change |
|------|--------|
| `pages/static/common.js` | Add capital performance + indicators data properties, parse in loadPortfolio(), add refreshPortfolio() method, fetch indicators |
| `pages/dashboard.html` | Add capital performance row, indicators row, refresh button |
| `pages/static/css/portal.css` | Styling for capital row, indicators row, refresh button |
| `tests/ui/dashboard_test.go` | Update TestDashboardPortfolioSummary (count changes from 4 to 8 when capital data present), add tests for new elements |

## Key Context for Implementation

### common.js:176-292 — portfolioDashboard() component
- `loadPortfolio()` at line 239 fetches `/api/portfolios/{name}` and parses holdings
- Response already contains `capital_performance` field — just not parsed
- `vireStore.fetch()` handles caching with 30s TTL
- `vireStore.invalidate()` clears cache by URL prefix

### dashboard.html:48-65 — portfolio summary section
- Uses `.portfolio-summary` flex container with `.portfolio-summary-item` children
- Shows when `filteredHoldings.length > 0`

### dashboard_test.go:180-286 — TestDashboardPortfolioSummary
- Currently asserts exactly 4 summary items
- Checks specific label text: TOTAL VALUE, TOTAL COST, NET RETURN $, NET RETURN %
- Will need updating for new layout

### portal.css:960-973 — portfolio summary styling
- `.portfolio-summary` uses flexbox with justify-content: space-between
- `.portfolio-summary-item` uses flexbox column with 0.25rem gap
