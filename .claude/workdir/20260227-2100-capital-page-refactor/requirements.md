# Capital Page Refactor

## Problem
The capital page (`/capital`) is broken:
1. **Wrong endpoint**: `common.js:592` calls `/api/portfolios/{name}/cash-transactions` — endpoint doesn't exist on vire-server
2. **Summary shows 0.00**: Server response has no `summary` object — totals never populate
3. **Wrong sort order**: Transactions display oldest-first, should be latest-first

## Correct API Endpoints
- `GET /api/portfolios/{name}/cashflows` — returns `{ portfolio_name, version, transactions: [...] }`
- `GET /api/portfolios/{name}/cashflows/{id}` — single transaction
- `GET /api/portfolios/{name}/cashflows/performance` — capital performance metrics

### Transaction shape (from `/cashflows`)
```json
{
  "id": "ct_xxx",
  "type": "deposit|transfer_out|transfer_in|dividend|withdrawal|contribution",
  "date": "2026-01-20T00:00:00Z",
  "amount": 1080,
  "description": "Deposit",
  "category": "accumulate",  // optional
  "created_at": "...",
  "updated_at": "..."
}
```

Note: No `summary` object in the response. Totals must be computed client-side.

## Changes Required

### 1. `pages/static/common.js` — `cashTransactions()` function (~line 538-656)

**Fix endpoint** (line 592):
```javascript
// Before
'/api/portfolios/' + encodeURIComponent(this.selected) + '/cash-transactions'
// After
'/api/portfolios/' + encodeURIComponent(this.selected) + '/cashflows'
```

**Compute totals client-side** in `loadTransactions()`:
- Sum amounts where type is credit: `deposit`, `contribution`, `transfer_in`, `dividend` → totalDeposits
- Sum amounts where type is debit: `transfer_out`, `withdrawal` → totalWithdrawals
- Net = totalDeposits - totalWithdrawals

**Sort transactions** by date descending (latest first):
```javascript
this.transactions.sort((a, b) => new Date(b.date) - new Date(a.date));
```

### 2. No HTML changes expected
The `capital.html` template binds to the same properties — no changes needed.

### 3. No Go changes expected
The `/api/` proxy forwards all API paths to vire-server unchanged. `/cashflows` will proxy correctly.

## Files
| File | Change |
|------|--------|
| `pages/static/common.js` | Fix endpoint URL, compute totals client-side, sort desc |

## Testing
- UI tests should validate the capital page loads with data
- Verify totals compute correctly for credit/debit types
