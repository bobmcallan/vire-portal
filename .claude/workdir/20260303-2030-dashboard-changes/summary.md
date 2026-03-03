# Summary: Dashboard — Last Synced, D/W/M Changes, Rename Label

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `pages/dashboard.html` | Renamed "TOTAL VALUE" to "PORTFOLIO VALUE". Added D/W/M change badges under portfolio value. Added last_synced timestamp after portfolio header. |
| `pages/static/common.js` | Added `lastSynced`, `changeDayPct`, `changeWeekPct`, `changeMonthPct`, `hasChanges` properties. Added `fmtSynced()`, `changePct()`, `changeClass()` helpers. Parsing in both `loadPortfolio()` and `refreshPortfolio()`. |
| `pages/static/css/portal.css` | Added `.portfolio-changes`, `.change-up`, `.change-down`, `.change-neutral`, `.portfolio-synced` styles. |
| `tests/ui/dashboard_test.go` | Updated "TOTAL VALUE" → "PORTFOLIO VALUE" in `TestDashboardPortfolioSummary`. Added `TestDashboardChangesRow` and `TestDashboardLastSynced`. |
| `internal/handlers/dashboard_stress_test.go` | Updated "TOTAL VALUE" → "PORTFOLIO VALUE". Added 7 stress tests for changes/synced bindings, conditional display, label rename, structural ordering. |

## Tests
- Unit tests: all pass
- UI tests: 22 pass, 10 skip (expected — no portfolio data), 0 fail
- Stress tests: 7 new, all pass (72.8s total)
- go vet: clean

## Architecture
- Architect review: PASSED — data flow follows server → JSON → JS property → template binding pattern
- CSS follows semantic naming conventions (.portfolio-* prefix)
- All bindings use x-text (safe, not x-html)

## Devils-Advocate
- Security: No issues. All bindings safe. fmtSynced() handles hostile input via isNaN + try/catch. changePct()/changeClass() return hardcoded values only.
- 7 stress tests added covering XSS safety, conditional display, structural ordering, label regression

## Notes
- D/W/M colors: `.change-up` (#2d8a4e, soft green), `.change-down` (#b54747, soft red), `.change-neutral` (#888, gray)
- last_synced converts UTC → local time via `Date.toLocaleString('en-AU', ...)`
- Changes data comes from existing `changes` object in API response (no backend changes needed)
