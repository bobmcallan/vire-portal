# Summary: Dashboard Labels, NET RETURN % Rebase, Portal Version Headers

**Status:** completed
**Feedback:** fb_aa6c3aef, fb_a9a562df, fb_aeaad863

## Changes
| File | Change |
|------|--------|
| `pages/dashboard.html` | Renamed "TOTAL INVESTED" → "COST BASIS" (line 58), "CAPITAL INVESTED" → "TOTAL DEPOSITED" (line 74) |
| `pages/static/common.js` | Modified `totalGainPct` getter to compute `portfolioGain / capitalInvested * 100` when capital data available, falls back to server's cost-basis value |
| `internal/mcp/proxy.go` | Added `X-Vire-Portal-Version`, `X-Vire-Portal-Build`, `X-Vire-Portal-Commit` static headers in `NewMCPProxy()` |
| `tests/ui/dashboard_test.go` | Updated expected label arrays in `TestDashboardPortfolioSummary` and `TestDashboardCapitalPerformance` |
| `internal/handlers/dashboard_stress_test.go` | Updated expected label from "TOTAL INVESTED" to "COST BASIS" |
| `internal/mcp/mcp_test.go` | Added `TestNewMCPProxy_UserHeaders_PortalVersion`, updated `TestNewMCPProxy_UserHeaders_EmptyConfig` |

## Tests
- All unit tests pass (`go test ./...`)
- UI tests pass (`tests/ui` green)
- `go vet` clean
- Pre-existing `tests/api/service_auth_test.go` failures (Docker container startup) — unrelated

## Architecture
- Architect review: PASS — label changes are HTML-only, version headers follow existing X-Vire-* pattern
- Code quality review: PASS — no issues found

## Devils-Advocate
- Raised concern about version headers always being "dev" in production — verified as false positive (ldflags inject real values in CI/Docker builds)
- No other issues found

## Notes
- fb_aeaad863 (version handshake): Portal now sends version headers. Server-side validation is a separate task currently underway.
- NET RETURN % now shows return on total capital deployed (0.69% = 3298/477013) rather than cost basis (0.82% = 3298/401401) when capital data is available
