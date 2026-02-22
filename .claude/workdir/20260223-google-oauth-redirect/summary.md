# Summary: Fix Google OAuth redirect exposing internal Docker address

**Date:** 2026-02-23
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/handlers/auth.go` | Replaced direct browser redirect with server-side proxy. `HandleGoogleLogin` and `HandleGitHubLogin` now call `proxyOAuthRedirect()` which makes an HTTP request to vire-server, captures the redirect Location, and forwards it to the browser. Browser never sees internal Docker addresses. |
| `internal/handlers/auth_test.go` | Updated tests: mock vire-server returns 302 to Google/GitHub, verify proxy forwards Location. Added tests for server unreachable (`/error?reason=auth_unavailable`) and no redirect (`/error?reason=auth_failed`). |
| `internal/handlers/auth_stress_test.go` | Added stress tests: hostile Location headers (javascript:, data:, //evil.com), server timeout, multiple Location headers, internal address leakage prevention. ~452 lines added. |
| `internal/handlers/auth_integration_test.go` | Updated OAuth redirect chain test to use mock vire-server. |
| `tests/ui/auth_test.go` | Updated `TestAuthGoogleLoginRedirect`: verifies browser never lands on internal address, accepts either Google OAuth page or portal error page. |

## Tests
- All handler tests pass (`go test ./internal/handlers/...` — 105s)
- `go vet ./...` clean
- Auth UI test passes: browser correctly lands on `/error` page (not internal address) when Google OAuth is not configured
- Smoke UI tests: 8 pass, 0 fail

## Documentation Updated
- No documentation changes needed (internal security fix, transparent to users)

## Devils-Advocate Findings
- Stress tests cover hostile Location headers, SSRF concerns, timeout handling
- The proxy trusts vire-server's Location header (acceptable — vire-server is a trusted internal service)
- Callback URL is now properly `url.QueryEscape`'d (was previously concatenated raw)

## Notes
- The fix resolves the Docker deployment issue where `VIRE_API_URL=http://server:8080` caused browser redirects to an unreachable internal address
- Error handling: server unreachable → `/error?reason=auth_unavailable`; server responds without redirect → `/error?reason=auth_failed`
- The proxy uses a 10-second timeout and `http.ErrUseLastResponse` to capture the first redirect without following it
