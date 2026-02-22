# Summary: Dashboard Portfolio Enhancements

**Date:** 2026-02-22
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `pages/static/common.js` | Added `showClosed`, `filteredHoldings`, `totalValue`, `totalGain`, `totalGainPct` computed getters, and `gainClass()` helper to `portfolioDashboard()` |
| `pages/dashboard.html` | Added portfolio summary section, "show closed positions" checkbox, gain color binding on Gain% column, changed `x-for` loop from `holdings` to `filteredHoldings` |
| `pages/static/css/portal.css` | Added `.gain-positive`, `.gain-negative` color classes, `.portfolio-summary` layout, `.portfolio-filter-label` style |
| `internal/handlers/dashboard_stress_test.go` | Added 7 new stress tests validating template safety of new UI elements (by devils-advocate) |
| `tests/ui/dashboard_test.go` | Added `TestDashboardShowClosedCheckbox`, `TestDashboardPortfolioSummary`, `TestDashboardGainColors`; updated `TestDashboardHoldingsTable` with sort verification (by reviewer) |

## Tests
- 7 new handler stress tests added (all pass)
- 3 new UI tests added, 1 updated (all pass)
- All existing dashboard tests continue to pass
- Pre-existing `TestNewDefaultConfig_AuthDefaults` failure in config package (unrelated)

## Documentation Updated
- No user-facing documentation changes required (frontend-only enhancements)

## Devils-Advocate Findings
- Verified all dynamic content uses `x-text` (safe textContent) and `:class` bindings, not `x-html` (innerHTML)
- Confirmed `gainClass()` only returns hardcoded CSS class names, never user input
- Validated no inline event handlers (`onclick=`, etc.) in template
- Verified `filteredHoldings` is used in the loop (not raw `holdings`), ensuring the filter cannot be bypassed
- All findings were addressed in the implementation

## Notes
- The `gain_value` field is used for dollar gain in the summary; if the API returns a different field name, the `totalGain` computed property may show 0
- Holdings sort is alphabetical by ticker via `localeCompare`
- The "show closed positions" filter excludes holdings where `market_value === 0` (strict equality)
- The `TestAuthGoogleLoginRedirect` UI test failure is pre-existing (Google OAuth not configured in dev)
