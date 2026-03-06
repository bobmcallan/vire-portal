# Summary: Chart Gross Contributions, Holdings Alignment, Breadth Per-Ticker

**Status:** completed

## Changes
| File | Change |
|------|--------|
| pages/static/common.js | Added Gross Contributions dotted line dataset (conditional push behind breakdown toggle). Added `breadthSegments` getter for per-ticker segment data. |
| pages/dashboard.html | Breadth bar uses x-for per-ticker segments. Added holding-movement-content class to sub-row td. |
| pages/static/css/portal.css | Holding sub-row padding-top: 0.25rem. Removed breadth gradient pseudo-elements (::before). |
| internal/handlers/dashboard_stress_test.go | Updated breadth bar structure tests, added breadth style/segment/cloak tests, chart gross contributions checks. |
| tests/ui/dashboard_test.go | Updated breadth segment order check (flexible monotonic), holding-movement-row assertion flipped to expect existence. |

## Tests
- Unit tests: all pass (go vet clean, 0 failures)
- Stress tests: 6 new/updated tests for breadth and chart
- UI tests: updated for new structure
- Fix rounds: 0

## Architecture
- Architect: all checks passed, no issues
- Data flow, CSS naming, Alpine.js patterns all follow conventions

## Devils-Advocate
- Stress tests for XSS (:title binding safe, :style with || 0 guard)
- Breadth cloak test, segment order validation
- No security issues found

## Notes
- No server/API changes needed — all client-side
- Gross Contributions line only appears when value > 0
- Breadth segments sorted: falling → flat → rising (secondary: ticker alpha)
