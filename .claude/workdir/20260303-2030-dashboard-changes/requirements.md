# Requirements: Dashboard Changes — Last Synced, D/W/M Changes, Rename Label

**Status:** Implementation Spec
**Date:** 2026-03-03

## Scope

Three focused dashboard changes:

1. **Rename "TOTAL VALUE" to "PORTFOLIO VALUE"** in the first summary row
2. **Add D/W/M change percentages** below the portfolio value, color-coded green (up) / red (down) with soft colors
3. **Add last_synced timestamp** to the dashboard, displayed in local time (converted from UTC)

### What this does NOT do
- No backend changes (data already exists in API response)
- No new API endpoints
- No changes to other pages

---

## API Data Available

The `/api/portfolios/{name}` response already includes:

```json
{
  "last_synced": "2026-03-03T09:12:40.963690449Z",
  "changes": {
    "yesterday": {
      "portfolio_value": {
        "pct_change": -0.87,
        "has_previous": true
      }
    },
    "week": {
      "portfolio_value": {
        "pct_change": -0.75,
        "has_previous": true
      }
    },
    "month": {
      "portfolio_value": {
        "pct_change": -4.18,
        "has_previous": true
      }
    }
  }
}
```

---

## File Changes

### 1. `pages/static/common.js`

**Add new properties to `portfolioDashboard()` return object** (after `ledgerDividendReturn: 0`):

```javascript
lastSynced: '',
changeDayPct: null,
changeWeekPct: null,
changeMonthPct: null,
hasChanges: false,
```

**In `loadPortfolio()`** — after parsing `ledgerDividendReturn`, add:

```javascript
// Parse last_synced (UTC → local time)
this.lastSynced = holdingsData.last_synced || '';
// Parse changes
const changes = holdingsData.changes;
if (changes) {
    this.changeDayPct = changes.yesterday?.portfolio_value?.has_previous ? changes.yesterday.portfolio_value.pct_change : null;
    this.changeWeekPct = changes.week?.portfolio_value?.has_previous ? changes.week.portfolio_value.pct_change : null;
    this.changeMonthPct = changes.month?.portfolio_value?.has_previous ? changes.month.portfolio_value.pct_change : null;
    this.hasChanges = this.changeDayPct !== null || this.changeWeekPct !== null || this.changeMonthPct !== null;
} else {
    this.changeDayPct = null;
    this.changeWeekPct = null;
    this.changeMonthPct = null;
    this.hasChanges = false;
}
```

**In `refreshPortfolio()`** — same parsing block after `this.availableCash`:

```javascript
// Parse last_synced (UTC → local time)
this.lastSynced = data.last_synced || '';
// Parse changes
const changes = data.changes;
if (changes) {
    this.changeDayPct = changes.yesterday?.portfolio_value?.has_previous ? changes.yesterday.portfolio_value.pct_change : null;
    this.changeWeekPct = changes.week?.portfolio_value?.has_previous ? changes.week.portfolio_value.pct_change : null;
    this.changeMonthPct = changes.month?.portfolio_value?.has_previous ? changes.month.portfolio_value.pct_change : null;
    this.hasChanges = this.changeDayPct !== null || this.changeWeekPct !== null || this.changeMonthPct !== null;
} else {
    this.changeDayPct = null;
    this.changeWeekPct = null;
    this.changeMonthPct = null;
    this.hasChanges = false;
}
```

**In the reset block (else branch of `holdingsRes.ok` around line 295)**  — add resets:

```javascript
this.lastSynced = '';
this.changeDayPct = null;
this.changeWeekPct = null;
this.changeMonthPct = null;
this.hasChanges = false;
```

**Add helper methods** (after `pct()` method):

```javascript
fmtSynced(utcStr) {
    if (!utcStr) return '';
    try {
        const d = new Date(utcStr);
        if (isNaN(d.getTime())) return '';
        return d.toLocaleString('en-AU', {
            day: 'numeric', month: 'short', year: 'numeric',
            hour: '2-digit', minute: '2-digit', hour12: false
        });
    } catch { return ''; }
},
changePct(val) {
    if (val == null) return '-';
    return (val >= 0 ? '+' : '') + Number(val).toFixed(1) + '%';
},
changeClass(val) {
    if (val == null || val === 0) return 'change-neutral';
    return val > 0 ? 'change-up' : 'change-down';
},
```

### 2. `pages/dashboard.html`

**Change 1: Rename label** (line 59)

Replace:
```html
<span class="label">TOTAL VALUE</span>
```
With:
```html
<span class="label">PORTFOLIO VALUE</span>
```

**Change 2: Add D/W/M changes** — After the PORTFOLIO VALUE summary item (after line 61, before the CAPITAL RETURN $ item), add a changes sub-row inside the portfolio-summary-item:

Replace the TOTAL VALUE summary item block (lines 58-61):
```html
<div class="portfolio-summary-item">
    <span class="label">PORTFOLIO VALUE</span>
    <span class="text-bold" x-text="fmt(totalValue)"></span>
    <span class="portfolio-changes" x-show="hasChanges" x-cloak>
        <span :class="changeClass(changeDayPct)" x-text="'D:' + changePct(changeDayPct)"></span>
        <span :class="changeClass(changeWeekPct)" x-text="'W:' + changePct(changeWeekPct)"></span>
        <span :class="changeClass(changeMonthPct)" x-text="'M:' + changePct(changeMonthPct)"></span>
    </span>
</div>
```

**Change 3: Add last_synced** — After the portfolio header row (after line 44), add a synced timestamp row:

```html
<div class="portfolio-synced" x-show="lastSynced" x-cloak>
    <span class="text-muted" x-text="'Synced ' + fmtSynced(lastSynced)"></span>
</div>
```

### 3. `pages/static/css/portal.css`

**Add after `.portfolio-summary-item .label` block** (after line 1052):

```css
.portfolio-changes {
    display: flex;
    gap: 0.75rem;
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.05em;
}

.change-up { color: #2d8a4e; }
.change-down { color: #b54747; }
.change-neutral { color: #888; }

.portfolio-synced {
    font-size: 0.6875rem;
    letter-spacing: 0.05em;
    margin-bottom: 1rem;
}
```

Note: `.change-up` uses the same green as `.gain-positive` (#2d8a4e). `.change-down` uses a softer red (#b54747) than `.gain-negative` (#a33) — per user request for "soft colors".

---

## Test Updates

### 4. `tests/ui/dashboard_test.go`

**TestDashboardPortfolioSummary** (line ~221): Change `'TOTAL VALUE'` to `'PORTFOLIO VALUE'` in the JS eval.

**TestDashboardPortfolioSummary** (line ~224): Change `'TOTAL VALUE'` to `'PORTFOLIO VALUE'` in the expected array.

**TestDashboardPortfolioSummary** (line ~237): Update error message from "TOTAL VALUE" to "PORTFOLIO VALUE".

### 5. `internal/handlers/dashboard_stress_test.go`

**TestDashboardHandler_StressPortfolioSummarySection** (line ~535): Change `"TOTAL VALUE"` to `"PORTFOLIO VALUE"` in summaryLabels array.

**TestDashboardHandler_StressGainClassBindingsSafe** (line ~428): Change `x-text="fmt(totalValue)"` check — this still works since the binding name hasn't changed, only the label.

### 6. New test in `tests/ui/dashboard_test.go`: TestDashboardChangesRow

```go
func TestDashboardChangesRow(t *testing.T) {
    // Verify D/W/M change percentages appear under portfolio value
    // Check .portfolio-changes element exists
    // Verify it contains D:, W:, M: text
    // Verify change-up or change-down classes are applied
}
```

### 7. New test in `tests/ui/dashboard_test.go`: TestDashboardLastSynced

```go
func TestDashboardLastSynced(t *testing.T) {
    // Verify .portfolio-synced element exists
    // Verify it contains "Synced" text
    // Verify it shows a date (not UTC, should be local)
}
```

---

## Edge Cases

1. **No changes data**: `hasChanges` is false → D/W/M row hidden via `x-show`
2. **Partial changes**: Some periods null → individual values show `-`
3. **No last_synced**: Hidden via `x-show="lastSynced"`
4. **Zero change**: Shows `change-neutral` class (gray), displays `+0.0%`
5. **Large negative/positive**: `changePct()` uses `.toFixed(1)` — no overflow risk
6. **Invalid UTC string**: `fmtSynced()` handles with try/catch, returns empty string
