# Capital Page Summary Redesign

**Feedback:** fb_d352f7fe
**Feature doc:** /home/bobmc/development/vire/docs/features/20260228-cash-transaction-response-totals.md

## Problem

The vire-server `list_cash_transactions` response has been redesigned (server commit 678cb01). The old summary fields (`total_credits`, `total_debits`, `net_cash_flow`) no longer exist. The portal Capital page reads these removed fields and displays $0.00 for all summary values.

New server response shape:
```json
{
  "accounts": [
    {"name": "Trading", "type": "trading", "is_transactional": true, "balance": 548585.09},
    {"name": "Stake Accumulate", "type": "other", "is_transactional": false, "balance": 50600.00}
  ],
  "summary": {
    "total_cash": 599185.09,
    "transaction_count": 12,
    "by_category": {
      "contribution": 559185.09,
      "dividend": 50600.00,
      "transfer": 0,
      "fee": -500.00,
      "other": 0
    }
  },
  "transactions": [...]
}
```

## Scope

- Update `common.js` cashTransactions() to read new response fields
- Replace capital.html summary section with account balances + category breakdown
- Add 1 CSS class for account balances row
- Fix 3 stale UI tests
- NO Go handler changes (capital.go is page renderer only)
- NO Clear All button (destructive ops remain MCP-only)
- NO changes to transaction table, pagination, or toggleDefault()

## File Changes

### 1. `pages/static/css/portal.css` — Add CSS class

After `.portfolio-summary-capital` (around line 977), add:

```css
.portfolio-summary-accounts {
    border-bottom: 1px solid #888;
}
```

### 2. `pages/static/common.js` — Update cashTransactions() (line 542)

**Replace data properties** (lines 548-550):

OLD:
```js
totalDeposits: 0,
totalWithdrawals: 0,
netCashFlow: 0,
```

NEW:
```js
accounts: [],
totalCash: 0,
transactionCount: 0,
byCategory: {},
```

**Add computed properties** after `hasTransactions` getter (line 556):

```js
get hasAccounts() { return this.accounts.length > 0; },
get nonZeroCategories() {
    return Object.entries(this.byCategory).filter(([, v]) => v !== 0);
},
get hasCategoryBreakdown() { return this.nonZeroCategories.length > 0; },
```

**Update loadTransactions() parsing** (lines 605-608):

OLD:
```js
const summary = data.summary || {};
this.totalDeposits = summary.total_credits || 0;
this.totalWithdrawals = summary.total_debits || 0;
this.netCashFlow = summary.net_cash_flow || 0;
```

NEW:
```js
this.accounts = data.accounts || [];
const summary = data.summary || {};
this.totalCash = summary.total_cash || 0;
this.transactionCount = summary.transaction_count || 0;
this.byCategory = summary.by_category || {};
```

**Update error/empty reset** (lines 610-613):

OLD:
```js
this.transactions = [];
this.totalDeposits = 0;
this.totalWithdrawals = 0;
this.netCashFlow = 0;
```

NEW:
```js
this.transactions = [];
this.accounts = [];
this.totalCash = 0;
this.transactionCount = 0;
this.byCategory = {};
```

### 3. `pages/capital.html` — Replace summary section (lines 47-61)

Replace the existing summary block with two rows:

**Row 1 — Account balances + TOTAL CASH:**
```html
<!-- Account balances -->
<div class="portfolio-summary portfolio-summary-accounts" x-show="hasAccounts" x-cloak>
    <template x-for="acct in accounts" :key="acct.name">
        <div class="portfolio-summary-item">
            <span class="label" x-text="acct.name.toUpperCase()"></span>
            <span class="text-bold" x-text="fmt(acct.balance)"></span>
        </div>
    </template>
    <div class="portfolio-summary-item">
        <span class="label">TOTAL CASH</span>
        <span class="text-bold" :class="gainClass(totalCash)" x-text="fmt(totalCash)"></span>
    </div>
</div>
```

**Row 2 — Category breakdown (non-zero only):**
```html
<!-- Category breakdown -->
<div class="portfolio-summary" x-show="hasCategoryBreakdown" x-cloak>
    <template x-for="[cat, amount] in nonZeroCategories" :key="cat">
        <div class="portfolio-summary-item">
            <span class="label" x-text="cat.toUpperCase()"></span>
            <span class="text-bold" :class="gainClass(amount)" x-text="fmt(amount)"></span>
        </div>
    </template>
</div>
```

Design notes:
- Account balances use plain `text-bold` (informational, not directional)
- TOTAL CASH uses `gainClass(totalCash)` for directional coloring
- Category amounts use `gainClass(amount)` (contributions green, fees red)
- Visibility gated on `hasAccounts` / `hasCategoryBreakdown` (not `hasTransactions`)

### 4. `tests/ui/capital_test.go` — Fix 3 stale tests

**Replace `TestCapitalSummaryRow` (line 92) with `TestCapitalAccountBalances`:**

```go
func TestCapitalAccountBalances(t *testing.T) {
    ctx, cancel := newBrowser(t)
    defer cancel()

    err := loginAndNavigate(ctx, serverURL()+"/capital")
    if err != nil {
        t.Fatalf("login and navigate failed: %v", err)
    }

    _ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
    takeScreenshot(t, ctx, "capital", "account-balances.png")

    visible, err := isVisible(ctx, ".portfolio-summary-accounts")
    if err != nil {
        t.Fatalf("error checking account balances visibility: %v", err)
    }
    if !visible {
        t.Skip("account balances row not visible (no accounts available)")
    }

    count, err := elementCount(ctx, ".portfolio-summary-accounts .portfolio-summary-item")
    if err != nil {
        t.Fatalf("error counting account balance items: %v", err)
    }
    if count < 2 {
        t.Errorf("account balance item count = %d, want >= 2 (accounts + TOTAL CASH)", count)
    }

    hasTotalCash, err := commontest.EvalBool(ctx, `
        (() => {
            const row = document.querySelector('.portfolio-summary-accounts');
            if (!row) return false;
            const labels = row.querySelectorAll('.portfolio-summary-item .label');
            for (const label of labels) {
                if (label.textContent.trim() === 'TOTAL CASH') return true;
            }
            return false;
        })()
    `)
    if err != nil {
        t.Fatalf("error checking TOTAL CASH label: %v", err)
    }
    if !hasTotalCash {
        t.Error("TOTAL CASH label not found in account balances row")
    }
}
```

**Add `TestCapitalCategoryBreakdown` (new test):**

```go
func TestCapitalCategoryBreakdown(t *testing.T) {
    ctx, cancel := newBrowser(t)
    defer cancel()

    err := loginAndNavigate(ctx, serverURL()+"/capital")
    if err != nil {
        t.Fatalf("login and navigate failed: %v", err)
    }

    _ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
    takeScreenshot(t, ctx, "capital", "category-breakdown.png")

    hasCategoryRow, err := commontest.EvalBool(ctx, `
        (() => {
            const rows = document.querySelectorAll('.portfolio-summary:not(.portfolio-summary-accounts)');
            return rows.length > 0;
        })()
    `)
    if err != nil {
        t.Fatalf("error checking category breakdown: %v", err)
    }
    if !hasCategoryRow {
        t.Skip("category breakdown row not visible (all categories zero)")
    }

    labelsUppercase, err := commontest.EvalBool(ctx, `
        (() => {
            const row = document.querySelector('.portfolio-summary:not(.portfolio-summary-accounts)');
            if (!row) return false;
            const labels = row.querySelectorAll('.portfolio-summary-item .label');
            if (labels.length === 0) return false;
            for (const label of labels) {
                if (label.textContent.trim() !== label.textContent.trim().toUpperCase()) return false;
            }
            return true;
        })()
    `)
    if err != nil {
        t.Fatalf("error checking category labels: %v", err)
    }
    if !labelsUppercase {
        t.Error("category breakdown labels are not uppercase")
    }
}
```

**Fix `TestCapitalTransactionsTable` (line 146):**

Replace column check (lines 183-200) to expect 5 columns: `['DATE', 'ACCOUNT', 'CATEGORY', 'AMOUNT', 'DESCRIPTION']`

**Fix `TestCapitalTransactionColors` (line 203):**

Change amount cell index from `row.querySelectorAll('td')[2]` to `row.querySelectorAll('td')[3]` (line 233).

## Implementation Sequence

1. CSS — add `.portfolio-summary-accounts` class
2. JS — update `cashTransactions()` data properties, computed getters, loadTransactions parsing
3. HTML — replace summary block with account balances + category breakdown
4. Tests — fix stale tests to match new design

## Verification

1. `go test ./...` passes
2. `go vet ./...` clean
3. Capital page shows account balances with TOTAL CASH
4. Category breakdown shows non-zero categories only
5. Both rows hide when data is empty
6. Amount colors correct (green positive, red negative)
7. Labels uppercase
8. Transactions table unchanged
9. UI tests pass
