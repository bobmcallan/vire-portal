# Dashboard Capital Fields - Debug Findings

**Date:** 2026-02-27
**Branch:** `claude/debug-dashboard-fields-HUF3B`
**Status:** Portal workaround applied. Server-side root cause remains.

## Summary

The dashboard's second summary row (CAPITAL INVESTED, CAPITAL GAIN $, SIMPLE RETURN %, ANNUALIZED %) was displaying incorrect values. Investigation confirms this is a **vire-server issue** — the portal was correctly consuming the API response, but the server's `capital_performance` object contains wrong data.

A portal-side workaround has been applied to derive two of the three affected fields from data already available client-side.

## Symptoms

| Field | Expected (previous) | Actual (broken) |
|---|---|---|
| CAPITAL INVESTED | 427,985.09 | 427,985.09 (correct) |
| CAPITAL GAIN $ | -1,916.88 | 48,149.01 |
| SIMPLE RETURN % | -0.45% | 11.25% |
| ANNUALIZED % | -2.26% | 72.20% |

The first summary row (TOTAL VALUE, TOTAL COST, NET RETURN $, NET RETURN %) was unaffected.

## Root Cause

The portal proxies `/api/portfolios/{name}` directly to vire-server without transformation (`internal/server/routes.go:handleAPIProxy`). The response includes a `capital_performance` object:

```json
{
  "capital_performance": {
    "net_capital_deployed": 427985.09,
    "current_portfolio_value": 476134.10,
    "simple_return_pct": 11.25,
    "annualized_return_pct": 72.20,
    "transaction_count": 15
  }
}
```

The server's `current_portfolio_value` is ~$476K, but the actual portfolio value (sum of holdings market values) is ~$426K. This ~$50K discrepancy causes all derived capital metrics to be wrong:

- **CAPITAL GAIN $** = `current_portfolio_value - net_capital_deployed` = 476,134 - 427,985 = 48,149 (wrong)
- **SIMPLE RETURN %** = 48,149 / 427,985 * 100 = 11.25% (wrong, consistent with wrong capital gain)
- **ANNUALIZED %** = 72.20% (wrong, server-computed)

### Why the first row is correct

The first row fields come from different parts of the API response (`total_net_return`, `total_net_return_pct`, `total_cost`) and are computed from actual holdings data, not the `capital_performance` object.

## Portal Workaround

**File:** `pages/static/common.js` (two locations: `loadPortfolio` and `refreshPortfolio`)

Changed capital gain and simple return to be derived from the actual holdings total (`this.totalValue`) rather than trusting the server's `current_portfolio_value`:

```javascript
// Before (trusting server)
this.capitalGain = Number(cp.current_portfolio_value) - this.capitalInvested;
this.simpleReturnPct = Number(cp.simple_return_pct) || 0;

// After (derived from holdings)
this.capitalGain = this.totalValue - this.capitalInvested;
this.simpleReturnPct = this.capitalInvested !== 0
    ? (this.capitalGain / this.capitalInvested) * 100 : 0;
```

### What this fixes

- CAPITAL GAIN $ — now correct (derived from actual holdings total)
- SIMPLE RETURN % — now correct (derived from corrected capital gain)

### What this does NOT fix

- ANNUALIZED % — still uses `cp.annualized_return_pct` from the server, which requires a time-period calculation not available client-side. This will remain incorrect until the server bug is fixed.

## Server Investigation Needed

The vire-server's capital performance calculation needs investigation. Likely causes:

1. `current_portfolio_value` may be including closed positions, pending orders, or cash balances that don't appear in the holdings total
2. The capital performance endpoint may be using a different data source or snapshot than the holdings endpoint
3. The `simple_return_pct` and `annualized_return_pct` are likely computed from the same incorrect `current_portfolio_value`

## Files Changed

| File | Change |
|---|---|
| `pages/static/common.js` | Derive `capitalGain` and `simpleReturnPct` from holdings total instead of server's `current_portfolio_value` |
