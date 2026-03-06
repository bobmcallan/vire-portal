# Summary: SSR Migration (All Pages Except Dashboard)

**Status:** completed

## Changes

| File | Change |
|------|--------|
| `internal/client/vire_client.go` | Added `ProxyGet` method for authenticated GET requests to vire-server |
| `internal/client/vire_client_test.go` | 5 unit tests for ProxyGet |
| `internal/client/proxy_get_stress_test.go` | 11 stress tests (path traversal, SSRF, concurrency) |
| `internal/handlers/landing.go` | Added `proxyGetFn` field, `SetProxyGetFn`, `ServeErrorPage`, `ServeLandingPage`, `checkServerHealth`, `ServeGlossaryPage`, `ServeChangelogPage`, `ServeHelpPage`, glossary structs |
| `internal/handlers/strategy.go` | Added `proxyGetFn` field, `SetProxyGetFn`, SSR JSON embedding in ServeHTTP |
| `internal/handlers/cash.go` | Added `proxyGetFn` field, `SetProxyGetFn`, SSR JSON embedding in ServeHTTP |
| `internal/handlers/handlers_test.go` | 10 SSR handler unit tests |
| `internal/handlers/ssr_stress_test.go` | 25 stress tests (XSS, injection, concurrency, large payloads) |
| `internal/server/routes.go` | Updated 5 routes to use dedicated SSR handler methods |
| `internal/app/app.go` | Wired `ProxyGetFn` for PageHandler, StrategyHandler, CashHandler |
| `pages/error.html` | Replaced Alpine JS with `{{.ErrorMessage}}` Go template, removed script |
| `pages/landing.html` | SSR health status, removed `x-init="check()"`, keep retry button |
| `pages/glossary.html` | Full SSR rewrite with `{{range .Categories}}`, client-side filter only |
| `pages/changelog.html` | Added `window.__VIRE_DATA__` JSON hydration, SSR init in changelogPage() |
| `pages/help.html` | Added `window.__VIRE_DATA__` JSON hydration, SSR init in helpPage() |
| `pages/strategy.html` | Added `window.__VIRE_DATA__` JSON hydration |
| `pages/cash.html` | Added `window.__VIRE_DATA__` JSON hydration |
| `pages/static/common.js` | Updated `cashTransactions()` and `portfolioStrategy()` init to hydrate from SSR data |
| `tests/ui/error_test.go` | 3 new UI tests for SSR error page |
| `tests/ui/glossary_test.go` | 7 new UI tests for SSR glossary page |
| `tests/ui/changelog_test.go` | Reduced sleep times (2s → 500ms) |
| `tests/ui/cash_test.go` | Reduced sleep times (1s → 500ms) |

## Tests

- **Unit tests:** 15 new (5 ProxyGet + 10 handler SSR)
- **Stress tests:** 36 new (11 proxy + 25 handler)
- **UI tests:** 10 new (3 error + 7 glossary), 2 updated (changelog, cash sleep reductions)
- **Test results:** 61 UI pass, 0 fail, 17 skip (expected), 1 timeout (pre-existing)
- **go vet:** clean
- **go test ./...:** all packages pass
- **Fix rounds:** 0

## Architecture

- Architect review: APPROVED — all patterns verified, no issues
- No docs updates needed (internal changes only, same routes/config)

## Devils-Advocate

- No critical security issues found
- template.JS for trusted vire-server data: accepted risk, documented
- Error page ?reason= safe (allowlist mapping)
- ProxyGet: no SSRF/path traversal vectors (paths hardcoded in handlers)
- Glossary ?term= in script: Go html/template JS-escapes correctly

## Notes

- Dashboard intentionally excluded from SSR (remains client-side Alpine)
- All SSR pages have client-side fetch fallback if proxyGetFn is nil (backward compatible)
- `window.__VIRE_DATA__` cleaned up after Alpine reads it (memory)
- Server running on localhost:8883, health check OK
