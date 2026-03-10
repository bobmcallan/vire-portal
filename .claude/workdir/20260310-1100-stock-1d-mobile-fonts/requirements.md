# Requirements: Stock 1D Change + Mobile Font Sizing

## Scope

1. Add 1D price change % with coloured arrow to each holding row on desktop dashboard
2. Add the same 1D change to each holding card on mobile dashboard
3. Enlarge all small fonts on mobile dashboard to match portfolio value font weight/readability

## What this does NOT do

- No new API calls — `yesterday_price_change_pct` is already in the portfolio response per-holding
- No server-side Go changes — purely template + CSS
- No new JS methods — `holdingDailyPct()`, `changePct()`, `changeClass()` already exist

## File Changes

### 1. `pages/dashboard.html` — Desktop holdings row

**Location:** Lines 210-217 (the main holding data row)

Add a new `<td>` column for 1D change after Return %:
- Header: add `<th class="text-right">1D</th>` after the Return % header (line 205)
- Data: add `<td class="text-right" :class="changeClass(holdingDailyPct(h))" x-text="holdingDailyPct(h) != null ? changePct(holdingDailyPct(h)) : '-'"></td>`
- Update `colspan="6"` on movement row to `colspan="7"` (line 219)
- Add empty `<td></td>` in total footer row (line 234-238 area, match column count)

### 2. `pages/mobile.html` — Mobile holding card

**Location:** Lines 96-107 (mobile-holding-card template)

Add 1D change line in `mobile-holding-right` div (after the return % line):
```html
<span class="mobile-holding-1d" :class="changeClass(holdingDailyPct(h))"
      x-text="holdingDailyPct(h) != null ? changePct(holdingDailyPct(h)) + ' 1D' : ''"></span>
```

### 3. `pages/static/css/portal.css` — Mobile font sizing

**Current small font sizes on mobile:**
- `.mobile-synced` — 0.6875rem (11px)
- `.mobile-today-change` — 0.6875rem (11px)
- `.mobile-holding-ticker` — 0.8125rem (13px)
- `.mobile-holding-name` — 0.6875rem (11px)
- `.mobile-holding-right` — 0.8125rem (13px)
- `.mobile-full-link` — 0.8125rem (13px)
- `.mobile-changes-row` — 0.75rem (12px)
- `.label` — 0.75rem (12px)

**Target:** Match portfolio value readability. The `.mobile-value-amount` is 1.5rem (24px) — that's the hero number. "As big as" means readable-size, not literally 24px for labels. Bump all mobile small fonts to a comfortable baseline of 1rem (16px) body-size, with key data at 1.125rem.

**Changes:**
- `.mobile-dashboard .label` — 1rem
- `.mobile-synced` — 1rem
- `.mobile-today-change` — 1rem
- `.mobile-holding-ticker` — 1.125rem
- `.mobile-holding-name` — 1rem (remove max-width truncation or increase)
- `.mobile-holding-right` — 1.125rem
- `.mobile-changes-row` — 1rem
- `.mobile-full-link` — 1rem
- `.mobile-perf-item .text-bold` — 1.25rem
- `.mobile-holding-1d` (new) — 1rem
- `.mobile-holding-name` max-width — increase to 14rem

## Test Cases

### Unit tests (none needed — no Go changes)

### Stress tests for template validation
- Verify `<th>1D</th>` column header exists in desktop dashboard
- Verify colspan matches column count
- Verify `holdingDailyPct` binding exists in desktop holding row
- Verify `changePct` binding in desktop 1D column uses x-text (not x-html)
- Verify mobile holding card has 1D change span
- Verify mobile font size overrides exist in CSS

## Edge Cases
- `holdingDailyPct(h)` returns null when yesterday_close_price is null → show '-' on desktop, hide on mobile
- Closed positions (holding_value_market = 0) — hidden, no impact
