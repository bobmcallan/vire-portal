# Requirements: Portfolio Breadth Bar UI Component

**Feedback:** fb_3a0385ca
**Date:** 2026-03-06

## Scope

Two additions to the dashboard:
1. **Breadth bar section** above the holdings table — dollar-weighted gradient bar (green/grey/red) with counts and portfolio-level trend
2. **Per-holding trend enhancements** in the movement sub-row — direction arrow + today's dollar change

### What this does NOT do
- No Go backend changes
- No new API endpoints
- No changes to other pages

## API Data

Server v0.3.172+ may include `breadth` object. If absent, compute client-side from per-holding data.

### Client-side fallback logic
- Direction: `trend_score > 0.1` = rising, `< -0.1` = falling, else flat
- Weights: dollar-weighted by `holding_value_market`
- today_change: `sum((current_price - yesterday_close_price) * units)`
- trend_label from weighted score: >0.3 "Uptrend", >0.1 "Mixed-Up", >-0.1 "Mixed", >-0.3 "Mixed-Down", else "Downtrend"

## File Changes

### 1. pages/static/common.js

**New properties** (after `hasReturnPctChanges`):
```javascript
breadth: null,
hasBreadth: false,
```

**New helpers** (after `changeClass()`):
```javascript
trendArrow(score) — returns ↑/↓/→ based on score thresholds
trendArrowClass(score) — returns change-up/change-down/change-neutral
holdingTodayChange(h) — returns (current_price - yesterday_close_price) * units
fmtTodayChange(val) — formats as +$1.2K style
computeBreadth() — computes breadth from holdings when server doesn't provide it
```

**loadPortfolio()** — parse breadth (server or computed fallback), set hasBreadth
**refreshPortfolio()** — same breadth parsing

### 2. pages/dashboard.html

**Breadth bar** — insert above holdings `<section>`:
```html
<div class="breadth-bar-section" x-show="hasBreadth" x-cloak>
    <!-- trend label + today change -->
    <!-- 3-segment gradient bar (rising/flat/falling widths) -->
    <!-- counts: N Rising | N Flat | N Falling -->
</div>
```

**Movement sub-row** — modify to:
- Col 1 (Ticker): direction arrow via trendArrow(h.trend_score)
- Col 2 (Name): trend_label
- Col 3-5: D/W/M (unchanged)
- Col 6: today's $ change via fmtTodayChange(holdingTodayChange(h))

### 3. pages/static/css/portal.css

New classes: `.breadth-bar-section`, `.breadth-summary-row`, `.breadth-trend`, `.breadth-bar`, `.breadth-segment`, `.breadth-rising` (#2d8a4e), `.breadth-flat` (#888), `.breadth-falling` (#b54747), `.breadth-counts`

### 4. Tests

**Stress tests** (dashboard_stress_test.go):
- TestDashboardHandler_StressBreadthBarBindings
- TestDashboardHandler_StressTrendArrowBindings
- TestDashboardHandler_StressBreadthHelpersDefined

**UI tests** (dashboard_test.go):
- TestDashboardBreadthBar
- TestDashboardHoldingTrendArrows

## Edge Cases
- No holdings → hasBreadth false, section hidden
- All holdings same direction → one segment 100%, others 0%
- Missing yesterday_close_price → holdingTodayChange returns null
- Zero trend_score → flat (→ arrow, neutral grey)
