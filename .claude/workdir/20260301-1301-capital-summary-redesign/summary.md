# Summary: Capital Page Summary Redesign

**Status:** completed
**Feedback:** fb_d352f7fe
**Duration:** ~18 minutes

## Changes

| File | Change |
|------|--------|
| `pages/static/css/portal.css` | Added `.portfolio-summary-accounts` class (1px #888 border-bottom) |
| `pages/static/common.js` | Replaced `totalDeposits/totalWithdrawals/netCashFlow` with `accounts/totalCash/transactionCount/byCategory`. Added `hasAccounts`, `nonZeroCategories`, `hasCategoryBreakdown` computed getters. Updated `loadTransactions()` to read new server response fields. |
| `pages/capital.html` | Replaced 3-card summary (TOTAL CREDITS / TOTAL DEBITS / NET CASH FLOW) with 2 rows: account balances + TOTAL CASH, category breakdown (non-zero only) |
| `tests/ui/capital_test.go` | Replaced `TestCapitalSummaryRow` with `TestCapitalAccountBalances`. Added `TestCapitalCategoryBreakdown` with visibility check. Fixed `TestCapitalTransactionsTable` (5 columns). Fixed `TestCapitalTransactionColors` (amount index 3). |

## Tests

- Unit tests: all pass (`go vet` clean)
- UI tests round 1: 23 pass, 1 fail (TestCapitalCategoryBreakdown visibility edge case)
- UI tests round 2: 10 pass, 0 fail, 5 skip (data-dependent) — all green
- Fix rounds: 1 (visibility check for hidden x-show elements)

## Architecture

- Architect review: PASS — no issues found
- No Go handler changes needed (capital.go is page renderer only)
- API proxy unchanged — server response flows through unmodified

## Devils-Advocate

- All 7 stress test scenarios passed
- No security issues (x-text escapes HTML, amount sign is source of truth)
- Edge cases covered: empty accounts, null balances, unexpected category keys

## Notes

- Server response format change was breaking — portal showed $0.00 for all summary values
- Pre-existing auth test failures remain (out of scope, tracked separately)
- No Clear All button added — destructive operations remain MCP-only
