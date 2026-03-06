# Summary: Dashboard Enhancements (5 Feedback Items)

**Status:** completed

## Changes

| File | Change |
|------|--------|
| `pages/dashboard.html` | Row 2 daily-only badges (FB1), chart section wrapper + toggle (FB5), breadth segment order fix falling/flat/rising (FB4), per-holding trend rows + portfolio row + separator (FB3), movement sub-rows removed (FB3) |
| `pages/static/common.js` | `showChartBreakdown` property, `$watch` in init(), renderChart() hidden datasets + rebalance annotations, `fmtSyncedTime()` helper |
| `pages/static/css/portal.css` | `.portfolio-change-today`, chart section/toggle styles, soft breadth colours (#5a9e6f/#aaa/#c06060) + gradient pseudo-elements, breadth holding/portfolio/separator styles, removed `.holding-movement-row` |
| `pages/partials/head.html` | Added chartjs-plugin-annotation@3.0.1 CDN |
| `internal/handlers/dashboard_stress_test.go` | Updated binding checks for new classes, removed movement-row checks, 7 new stress tests (devils-advocate) |
| `tests/ui/dashboard_test.go` | Updated 4 tests: summary (daily badges), breadth bar (segment order, holding rows), trend arrows (targets breadth section), growth chart (section wrapper, toggle, skip fix) |

## Feedback Items Implemented

| ID | Severity | Description |
|----|----------|-------------|
| fb_875b4a77 | medium | NET RETURN $ and % simplified to daily-only change with "today" suffix |
| fb_63b1470f | high | Chart: Portfolio Value only by default, "Show breakdown" toggle, rebalance annotations |
| fb_cfd57276 | low | Breadth bar: soft muted colours, gradient transitions, segment order fix (falling left, rising right) |
| fb_5f275945 | low | Breadth bar: yesterday comparison (when server provides), sync timestamp ("as at HH:MM") |
| fb_a5c53886 | low | Per-holding trend rows in breadth section, movement sub-rows removed from holdings table |

## Tests

- Unit/stress tests: ALL PASS (go vet clean, go test ./... clean)
- UI tests: 64 pass, 0 fail, 18 skip
- Fix rounds: 3 (round 1: selector bug, round 2: stale Docker, round 3: pass)
- 7 new stress tests added by devils-advocate
- 4 UI tests updated by test-creator

## Architecture

- Architect: APPROVED, no blocking issues
- Minor note: `hasReturnDollarChanges` and `hasReturnPctChanges` are now dead code (harmless)

## Devils-Advocate

- $watch watcher leak found and fixed (moved from renderChart to init)
- XSS, annotation injection, CDN failure, CSS overflow all verified safe
- 7 stress tests added covering segment order, chart toggle, annotation plugin, breadth guards

## Notes

- Server running on localhost:8883 (v0.3.18, PID 180357)
- Chart annotation plugin loads from CDN; graceful degradation if unavailable
- Growth chart test skips in test environment (no portfolio data) — not a regression
