# Requirements: Stock Page, Trend Split, and Dashboard Enhancements

**Work directory:** `.claude/workdir/20260309-1100-stock-page-trend-split/`
**Date:** 2026-03-09
**Feedback items:** FB1 (fb_b065deb5), FB2 (fb_33ec005d), FB3 (fb_cea5afe5), FB4 (fb_4612327c), FB5 (fb_441c0d42)

## Overview

Five feedback items: label rename (FB1), new placeholder stock detail page (FB2), three dashboard display tweaks (FB3, FB4, FB5).

## FB1: Rename "Trend:" to "Stock Trend:" in holdings detail row
- File: `pages/dashboard.html` ~line 193
- Change `Trend:` to `Stock Trend:`

## FB2: New per-stock detail page `/stock/{ticker}`
- New handler: `internal/handlers/stock.go` (follow StrategyHandler pattern)
- New template: `pages/stock.html`
- New route: `GET /stock/{ticker...}` in `routes.go`
- Wire in `app.go`: StockHandler field, init, SetProxyGetFn
- Add "stock" to allowedPages in `internal/mcp/get_page.go`
- Dashboard ticker cells become links: `<a :href="'/stock/' + encodeURIComponent(h.ticker)">`
- Alpine component `stockDetail()` in `common.js`
- CSS styles for stock page in `portal.css`
- Submit feedback for missing data: trade history, price chart, filings, strategy alignment
- Unit tests: `internal/handlers/stock_test.go`
- UI tests: `tests/ui/stock_test.go`

## FB3: Fix "today" → "1D" on NET RETURN sub-labels
- `pages/dashboard.html`: change `+ ' today'` to `+ ' 1D'` on lines ~88 and ~95
- `pages/mobile.html`: same change on lines ~74 and ~81

## FB4: Add EQUITY VALUE headline
- Add Row 3 (Equity row) in `pages/dashboard.html` after Row 2
- Show `equityValue` (already computed in Alpine) with glossary tooltip

## FB5: Surface new server fields
1. Currency gain/loss at portfolio level: add state in `common.js`, display in Row 3
2. Currency labels per holding: show `original_currency` for non-AUD in detail row
3. Currency gain/loss per holding: show `currency_gain_loss` for non-zero in detail row
4. Dividend separation: show `dividend_received` and `dividend_forecast` separately in detail row

## Implementation Order
1. FB1 (label change)
2. FB3 ("1D" change)
3. FB4 (Equity Value row)
4. FB5 (new fields)
5. FB2 (stock page - largest)

## Files Summary
### New (4)
- `internal/handlers/stock.go`
- `pages/stock.html`
- `internal/handlers/stock_test.go`
- `tests/ui/stock_test.go`

### Modified (7)
- `pages/dashboard.html` — FB1, FB2, FB3, FB4, FB5
- `pages/mobile.html` — FB3
- `pages/static/common.js` — FB2, FB5
- `pages/static/css/portal.css` — FB2
- `internal/server/routes.go` — FB2
- `internal/app/app.go` — FB2
- `internal/mcp/get_page.go` — FB2
