# Summary: Stock Page Restructure (fb_b26e9d0b)

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/stock.html` | Removed Stock Price Trend section (candle chart, SMA toggles, RSI). Renamed "POSITION WALK" to "POSITION P&L". Added "Realised P&L" column to trade history table with gain/loss colouring. |
| `pages/static/common.js` | Removed `renderPriceChart()`, `destroyPriceChart()`, SMA state/watches. Added `tradeRealisedPL(t)` helper. Rewrote `renderWalkChart()` as P&L chart with green/red fill split at zero, trade markers, emphasised zero gridline. |
| `pages/static/css/portal.css` | Removed `.stock-price-chart-section`, `.stock-trend-badge`, `.trend-*`, `.stock-rsi-value` styles. |
| `internal/handlers/stock_stress_test.go` | Added 6 stress tests: NoPriceChartSection, NoSMACheckboxes, PandLChartPresent, TradeRealisedPLColumn, TradeRealisedPLUsesXText, WalkChartCanvasExists. Updated existing tests for "POSITION P&L" rename. |
| `tests/ui/stock_test.go` | Updated "POSITION WALK" references to "POSITION P&L". Added PriceChartRemoved test. |

## Tests
- 6 stress tests added to `stock_stress_test.go`
- 1 UI test added (`PriceChartRemoved`)
- All handler tests pass (`go test ./internal/handlers/` — 0.087s)
- `go vet ./...` clean
- Pre-existing: `internal/seed` timeout (unrelated), `TestAdminPromptsHandler_ListXSSEscaping` hang (unrelated)

## Architecture
- Architect review: APPROVED — pure template/JS/CSS change, no handler modifications needed

## Devils-Advocate
- Reviewed for XSS (x-text used, not x-html for P&L values)
- Verified tradeRealisedPL null guard for Buy trades
- Confirmed walkChart destroy guard prevents memory leaks

## Notes
- P&L chart uses two datasets split at zero for green/red fill — cleaner than conditional background colours
- `tradeRealisedPL()` uses average cost method (position.holding_cost_avg) — approximate but correct for SMSF-style portfolios
- Zero gridline serves as breakeven reference — no separate breakeven line needed
