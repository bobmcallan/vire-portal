# Summary: Capital Page Refactor

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/static/common.js` | Fix endpoint `/cash-transactions` → `/cashflows`, compute totals client-side, sort transactions latest-first |

## Fixes
1. **Endpoint URL** — was hitting non-existent `/cash-transactions`, now uses correct `/cashflows`
2. **Summary totals** — server returns no summary object; now computed client-side by summing credit types (deposit, contribution, transfer_in, dividend) vs debit types
3. **Sort order** — transactions sorted by date descending (latest first)

## Notes
- Single file change, no Go code or HTML template changes needed
- The `/api/` proxy forwards `/cashflows` to vire-server unchanged
