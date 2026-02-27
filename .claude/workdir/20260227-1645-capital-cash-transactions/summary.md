# Summary: Capital Cash Transactions Page

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/handlers/capital.go` | NEW — CapitalHandler (mirrors DashboardHandler pattern) |
| `internal/app/app.go` | Added CapitalHandler field + initialization in initHandlers() |
| `internal/server/routes.go` | Added `GET /capital` route |
| `pages/capital.html` | NEW — Cash transactions page template |
| `pages/static/common.js` | Added `cashTransactions()` Alpine.js component (127 lines) |
| `pages/static/css/portal.css` | Added `.pagination` and `.pagination-info` styles |
| `pages/partials/nav.html` | Added Capital link in desktop and mobile nav |
| `tests/ui/capital_test.go` | NEW — 14 UI tests |
| `README.md` | Added /capital route, test category, file tree entries |
| `.claude/skills/develop/SKILL.md` | Added /capital to routes table |

## Tests
- 14 UI tests created in `tests/ui/capital_test.go`
- Test execution: 8 PASSED, 12 FAILED, 11 SKIPPED
- All failures are pre-existing (dev auth `/api/auth/login` not working — same root cause as previous run)
- No new failures introduced
- `go build ./...` and `go vet ./...` pass clean

## Architecture
- Architect review: PASS — no fixes needed
- Handler pattern, route registration, template structure, nav links all follow existing conventions
- No new dependencies introduced

## Devils-Advocate
- 0 critical, 0 high, 1 medium, 7 low findings
- Medium: Must use x-text (not x-html) for transaction data — verified correct
- All recommendations followed in implementation (encodeURIComponent, pagination bounds, auth check)

## Feature Details
- Route: `GET /capital`
- Portfolio selector with default checkbox (same as dashboard/strategy)
- Summary row: TOTAL DEPOSITS, TOTAL WITHDRAWALS, NET CASH FLOW
- Paged table (100 items/page): DATE, TYPE, AMOUNT, DESCRIPTION
- Transaction colors: green for credits (deposit/contribution/transfer_in/dividend), red for debits (withdrawal/transfer_out)
- Pagination: PREV / PAGE X OF Y / NEXT
- Data fetched via `/api/portfolios/{name}/cash-transactions` (proxied to vire-server)

## Notes
- Pre-existing test failures from dev auth issue remain — not introduced by this feature
- No server restart performed (dev server not currently running)
