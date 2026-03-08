# Summary: Portal Version Tracking

**Status:** completed
**Feature:** fb_3f4a5832

## Changes
| File | Change |
|------|--------|
| `internal/client/vire_client.go` | Added `versionTransport` RoundTripper (sets `X-Portal-Version` and `X-Portal-Build` on all requests), updated `NewVireClient(baseURL, version, build string)`, added `ReportPortalVersion` method |
| `internal/app/app.go` | Pass `config.GetVersion(), config.GetBuild()` to NewVireClient, add version report to startup goroutine, new else-if branch for service key without admin emails |
| `internal/seed/version.go` | NEW — `ReportPortalVersion` with retry logic (follows service.go pattern) |
| `internal/seed/version_test.go` | NEW — 3 tests (success, retry, all-fail) |
| `internal/seed/seed.go` | Updated NewVireClient call to 3-arg |
| `internal/seed/service.go` | Updated NewVireClient call to 3-arg |
| `internal/seed/admin.go` | Updated NewVireClient call to 3-arg |
| `internal/client/vire_client_test.go` | Updated all calls to 3-arg + 6 new tests (version headers + ReportPortalVersion) |
| `internal/client/version_stress_test.go` | NEW — 42 stress tests by devils-advocate |
| `internal/seed/version_stress_test.go` | NEW — stress tests by devils-advocate |
| `internal/client/*_stress_test.go` (4 files) | Updated NewVireClient calls to 3-arg |
| `internal/seed/seed_test.go` | Updated NewVireClient call to 3-arg |
| `tests/api/service_auth_test.go` | Updated NewVireClient call to 3-arg |

## Tests
- Unit tests added: 9 new (6 client + 3 seed)
- Stress tests added: 42 new by devils-advocate
- Test results: all unit/API tests PASS, go vet clean
- 3 UI test failures are PRE-EXISTING (no templates changed)
- Fix rounds: 0

## Architecture
- Architect review: APPROVED — RoundTripper pattern is idiomatic, follows existing conventions
- No docs updated (backend-only, no user-facing behavior change)

## Devils-Advocate
- 42 stress tests written covering CRLF injection, long strings, empty values, concurrency, race conditions
- All pass with -race
- No security issues found

## Notes
- Header names `X-Portal-Version` / `X-Portal-Build` are intentionally different from MCP proxy's `X-Vire-Portal-*` headers (different purpose)
- Startup POST requires service key — gracefully skipped when not configured
- Version report runs after service registration in the same goroutine
