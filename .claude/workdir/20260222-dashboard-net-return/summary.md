# Summary: Dashboard net return fields, total cost, and holding gain columns

**Date:** 2026-02-22
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `pages/dashboard.html` | Added TOTAL COST summary item; split single Return% column into Gain $ and Gain % columns with color coding |
| `pages/static/common.js` | Added `portfolioCost` state and `totalCost` getter; changed gain fields to use `total_net_return`/`total_net_return_pct` from API; capture `total_cost` in `loadPortfolio()`; reset all gain/cost fields on error |
| `internal/vire/models/portfolio.go` | Added `TotalNetReturn`, `TotalNetReturnPct` to Portfolio struct; added `NetReturn`, `NetReturnPct` to Holding struct |
| `tests/ui/dashboard_test.go` | Updated summary item count (3→4), expected labels, gain color check indices (items 2,3), column header checks for Gain $/Gain %, header debug logging |
| `internal/handlers/dashboard_stress_test.go` | New stress tests: auth, XSS, concurrency, template safety, gain bindings, summary labels |

## Tests
- All 13 dashboard UI tests pass (1 skipped: portfolio summary — no holdings data in test env)
- All handler stress tests pass (18 new stress tests)
- `go vet ./...` clean
- Pre-existing failures: `TestNewDefaultConfig_AuthDefaults` (config port default), `TestAuthGoogleLoginRedirect` (OAuth not configured) — unrelated

## Documentation Updated
- No documentation changes needed (minor frontend field changes)

## Devils-Advocate Findings
- 18 stress tests added covering: unauthenticated access, expired/garbage tokens, XSS via x-text safety, concurrent access, nil lookups, template data isolation, gain class binding safety, inline event handler detection, filtered holdings loop safety, JSON response shape resilience
- No critical issues found — Alpine.js x-text prevents XSS, gain classes use hardcoded values only

## Notes
- Column headers changed from "Return $"/"Return %" to "Gain $"/"Gain %" for consistency with summary section labels "TOTAL GAIN $"/"TOTAL GAIN %"
- Portfolio summary now has 4 items: TOTAL VALUE, TOTAL COST, TOTAL GAIN $, TOTAL GAIN %
- Per-holding net return fields use `net_return` and `net_return_pct` from API (with `gainClass()` color coding)
