# Capital Cash Transactions Page

## Feature
New page at `/capital` showing capital credits/debits (cash transactions) for the selected portfolio. Displays direct capital flow — cash added/withdrawn. Presented as a paged table (100 items default), sorted by date (newest first).

## API
The portal proxies `/api/` to vire-server automatically (see `routes.go:73`). The cash transactions endpoint is:
- `GET /api/portfolios/{name}/cash-transactions` — returns `{ transactions: [...], summary: { total_deposits, total_withdrawals, net_cash_flow } }`

Each transaction: `{ id, portfolio_name, type, date, amount, description, category, notes }`
Types: deposit, withdrawal, contribution, transfer_in, transfer_out, dividend

No new API proxy code needed — the existing `/api/` catch-all handles it.

## Files to Create

### `internal/handlers/capital.go`
Page handler following exact dashboard.go pattern:
- `CapitalHandler` struct with logger, templates, devMode, jwtSecret, userLookupFn, apiURL
- `NewCapitalHandler()` constructor (same as NewDashboardHandler)
- `SetAPIURL()` setter
- `ServeHTTP()` — auth check, redirect if not logged in, Navexa key check, render `capital.html` with Page="capital"

### `pages/capital.html`
Template following dashboard.html/strategy.html pattern:
- `{{template "head.html" .}}`, title "VIRE CAPITAL"
- `{{if .LoggedIn}}{{template "nav.html" .}}{{end}}`
- `<main class="page" x-data="cashTransactions()" x-init="init()">`
- Navexa warning banner (same as dashboard)
- Loading/error states
- Portfolio selector with default checkbox (same pattern as dashboard/strategy — NO refresh button)
- Summary row showing TOTAL DEPOSITS, TOTAL WITHDRAWALS, NET CASH FLOW (reuse `.portfolio-summary` class)
- Paged table in `.panel-headed` with header "CASH TRANSACTIONS"
  - Columns: DATE, TYPE, AMOUNT, DESCRIPTION
  - Use `.tool-table` class
  - Amount colored: green (gain-positive) for deposits/contributions/transfer_in/dividend, red (gain-negative) for withdrawals/transfer_out
- Pagination controls below table: PREV / page info / NEXT
- `{{template "footer.html" .}}`

### `tests/ui/capital_test.go`
UI tests following test-common rules. Must read `.claude/skills/test-common/SKILL.md` and `.claude/skills/test-create-review/SKILL.md`.

## Files to Modify

### `internal/app/app.go`
- Add field: `CapitalHandler *handlers.CapitalHandler`
- In `initHandlers()`: create with `NewCapitalHandler(logger, devMode, jwtSecret, userLookup)`, call `SetAPIURL()`

### `internal/server/routes.go`
- Add route: `mux.HandleFunc("GET /capital", s.app.CapitalHandler.ServeHTTP)` in the UI page routes section (after strategy, before mcp-info)

### `pages/static/common.js`
Add `cashTransactions()` function following `portfolioDashboard()` pattern:
- State: portfolios, selected, defaultPortfolio, transactions, summary (totalDeposits, totalWithdrawals, netCashFlow), loading, error, currentPage, pageSize (100)
- Computed: `isDefault`, `pagedTransactions` (slice for current page), `totalPages`, `hasTransactions`
- `init()` — load portfolios via `vireStore.fetch('/api/portfolios')`, select default, load transactions
- `loadTransactions()` — fetch `/api/portfolios/{name}/cash-transactions`, populate transactions + summary
- `toggleDefault()` — same pattern as dashboard
- `prevPage()`, `nextPage()` — pagination
- `txnClass(type)` — return 'gain-positive' for deposit/contribution/transfer_in/dividend, 'gain-negative' for withdrawal/transfer_out
- `fmt(val)` — same number formatter
- `formatDate(dateStr)` — format date for display

### `pages/partials/nav.html`
Add Capital link between Strategy and MCP:
```html
<li><a href="/capital" {{if eq .Page "capital"}}class="active"{{end}}>Capital</a></li>
```
Also add to mobile menu between Strategy and MCP.

### `pages/static/css/portal.css`
Add pagination styles:
```css
.pagination { display: flex; align-items: center; justify-content: center; gap: 1rem; padding: 1rem 0; font-size: 0.75rem; letter-spacing: 0.1em; }
.pagination-info { color: #888; }
```

### `README.md`
Add Capital page to features list if one exists.

## Design Rules
- Monochrome: #000, #fff, #888, #2d8a4e (green), #a33 (red)
- IBM Plex Mono font
- No border-radius, no box-shadow
- Reuse existing CSS classes (`.tool-table`, `.panel-headed`, `.portfolio-header`, `.portfolio-summary`, `.btn`)
- All labels uppercase
- x-cloak on conditional elements

## Patterns to Follow
- `vireStore.fetch()` for all API calls
- `debugError()` for error logging
- `vireStore.dedup()` for portfolio list
- `vireStore.invalidate()` after mutations
- Toast notifications via `window.dispatchEvent(new CustomEvent('toast', ...))`
