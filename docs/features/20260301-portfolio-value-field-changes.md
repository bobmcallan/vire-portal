# Portfolio Value Field Changes — Server Update 2026-03-01

The Vire server `get_portfolio` response has been updated to fix double-counting in `total_value` and redefine `total_cost`. Three new fields have been added. The portal must update its rendering to match.

## Changed Fields

### `total_value` — REDEFINED
- **Before**: `total_value_holdings + total_cash` (double-counted deployed capital)
- **After**: `total_value_holdings + available_cash`
- **Impact**: Value drops significantly for portfolios with cash transactions (e.g. was $906k, now ~$479k)
- **Portal action**: No code change needed if rendering `total_value` directly — the server now returns the correct number. Remove any client-side recalculation of this field.

### `total_cost` — REDEFINED
- **Before**: `sum(avg_cost * units)` for open positions only (cost basis)
- **After**: `sum(total_invested - total_proceeds)` for all holdings including closed (net capital deployed in equities from trade history, FX-adjusted)
- **Impact**: Value changes. Now represents how much capital is locked in equities, not remaining cost basis.
- **Portal action**: Update labels. This is NOT "cost basis" or "total invested" — it is "net equity capital". If the portal displays this as "TOTAL INVESTED", rename to "NET EQUITY CAPITAL" or similar.

## New Fields

### `available_cash`
- **Formula**: `total_cash - total_cost`
- **Meaning**: Uninvested cash remaining after subtracting capital locked in equities
- **Notes**: Can be negative when total_cost > total_cash (e.g. equity appreciated beyond original investment). This is valid.
- **Portal action**: Render in dashboard summary. Suggested label: "AVAILABLE CASH" or "UNINVESTED CASH".

### `capital_gain`
- **Formula**: `total_value - capital_performance.net_capital_deployed`
- **Meaning**: Overall portfolio gain/loss vs net capital deployed
- **Notes**: Only present when `capital_performance` exists (cash transactions configured). Uses `omitempty` — absent when zero.
- **Portal action**: Render as "CAPITAL GAIN $". Replace any client-side gain calculation that was using the old (incorrect) `total_value - total_cost`.

### `capital_gain_pct`
- **Formula**: `(capital_gain / net_capital_deployed) * 100`
- **Meaning**: Capital gain as percentage of deployed capital
- **Notes**: Only present when `capital_performance` exists and `net_capital_deployed > 0`. Uses `omitempty`.
- **Portal action**: Render as "CAPITAL GAIN %". Replace any client-side percentage derived from old fields.

### `total_proceeds` (per-holding)
- **Formula**: Sum of all sell proceeds for this holding (units * price - fees)
- **Meaning**: How much cash was received from selling portions of this holding
- **Portal action**: Optional — display in holding detail view if useful.

## Unchanged Fields

| Field | Notes |
|-------|-------|
| `total_value_holdings` | Equity market value — unchanged |
| `total_cash` | Full ledger balance — unchanged |
| `total_net_return` | P&L on open positions — unchanged |
| `total_net_return_pct` | P&L percentage — unchanged |
| `capital_performance` | XIRR, simple return, deposits/withdrawals — unchanged |
| `yesterday_total` / `last_week_total` | Now use available_cash instead of total_cash (server-side fix, no portal change needed) |

## Portal Files Affected

### `pages/static/common.js`
- **Lines 264, 494** — `total_value` rendering in dashboard. No change needed (server value is now correct).
- **Lines 267, 497** — `total_cost` rendering. Update label from "TOTAL INVESTED" to "NET EQUITY CAPITAL".
- **Lines 265-266, 495-496** — `total_net_return` / `total_net_return_pct`. Unchanged.
- **Lines 271, 501** — `capital_performance.net_capital_deployed`. Unchanged.
- **Lines 276, 506** — `capital_performance.annualized_return_pct`. Unchanged.
- **Line 364** — Growth chart adds `ExternalBalance` to `TotalValue`:
  ```javascript
  // REMOVE THIS — ExternalBalance is deprecated (always 0)
  const totalValues = this.growthData.map(p => p.TotalValue + (p.ExternalBalance || 0));
  // REPLACE WITH:
  const totalValues = this.growthData.map(p => p.TotalValue);
  ```
- **Line 616** — `total_cash` on capital page. Unchanged.
- Add rendering for new fields: `available_cash`, `capital_gain`, `capital_gain_pct`.

### `pages/dashboard.html`
- Add dashboard cards for `available_cash`, `capital_gain`, `capital_gain_pct`.
- Rename any "TOTAL INVESTED" label to "NET EQUITY CAPITAL".

### `pages/capital.html`
- Cash transactions page — unchanged (uses `total_cash` and account balances).

### `internal/vire/models/portfolio.go`
- Add `AvailableCash`, `CapitalGain`, `CapitalGainPct` fields to the Portal's Portfolio struct.
- Add `TotalProceeds` field to the Holding struct.
- Remove `ExternalBalance` from `GrowthDataPoint` struct (deprecated, always 0).

## Redundant Client-Side Calculations to Remove

The portal should stop computing these values and use server-provided fields instead:

1. **Capital gain**: Remove any `total_value - total_cost` or `total_value - total_deposited` calculation. Use `capital_gain` and `capital_gain_pct` directly.
2. **Simple return %**: Remove any `(total_value - X) / X * 100` calculation. Use `capital_performance.simple_return_pct`.
3. **Total value recalculation**: Remove any `total_value_holdings + total_cash` addition. Use `total_value` directly.
4. **Available cash derivation**: Remove any `total_cash - total_cost` computation. Use `available_cash` directly.
5. **ExternalBalance addition**: Remove `+ (p.ExternalBalance || 0)` from growth chart (common.js line 364). Field is deprecated and always 0.

## Dashboard Card Mapping

Suggested mapping from server fields to dashboard cards:

| Dashboard Card | Server Field | Notes |
|---------------|-------------|-------|
| HOLDINGS VALUE | `total_value_holdings` | Equity market value |
| PORTFOLIO VALUE | `total_value` | Equity + available cash |
| NET EQUITY CAPITAL | `total_cost` | Capital locked in equities |
| AVAILABLE CASH | `available_cash` | Uninvested cash |
| TOTAL CASH | `total_cash` | Full ledger balance |
| NET RETURN $ | `total_net_return` | P&L on positions |
| NET RETURN % | `total_net_return_pct` | P&L percentage |
| CAPITAL GAIN $ | `capital_gain` | vs net deployed |
| CAPITAL GAIN % | `capital_gain_pct` | vs net deployed |
| ANNUALISED % | `capital_performance.annualized_return_pct` | XIRR |
| SIMPLE RETURN % | `capital_performance.simple_return_pct` | Simple return |

## Example Response (SMSF)

```json
{
  "total_value_holdings": 427561,
  "total_value": 478561,
  "total_cost": 426985,
  "total_cash": 477985,
  "available_cash": 51000,
  "total_net_return": 3298,
  "total_net_return_pct": 0.77,
  "capital_gain": 1548,
  "capital_gain_pct": 0.32,
  "capital_performance": {
    "total_deposited": 477013,
    "total_withdrawn": 0,
    "net_capital_deployed": 477013,
    "simple_return_pct": 0.32,
    "annualized_return_pct": 20.84
  }
}
```

## Server Reference

Full field definitions: `vire/docs/features/20260301-portfolio-value-definitions.md`
