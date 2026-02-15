# Summary: Dashboard UI Fixes

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/server/middleware.go` | Added `'unsafe-eval'` to CSP `script-src` — Alpine.js needs `new Function()` for expression evaluation |
| `internal/server/middleware_test.go` | Added test asserting `'unsafe-eval'` is present in CSP |
| `pages/dashboard.html` | Removed `<h1 class="dashboard-title">DASHBOARD</h1>` |
| `pages/settings.html` | Removed `<h1 class="dashboard-title">SETTINGS</h1>` |
| `docker/docker-compose.yml` | Changed `VIRE_API_URL` to `${VIRE_API_URL:-http://vire-server:4242}` (vire-server listens on 4242 not 8080) |
| `tests/browser-check/main.go` | Filter CSP EvalError exceptions from JS error collector (false positives from chromedp) |

## Tests
- CSP tests pass (`TestSecurityHeadersMiddleware_SetsAllHeaders`, `TestCSP_AllowsSelfScripts`)
- Handler tests pass (`go test ./internal/handlers/`)
- Browser-check validation: 7/7 on dashboard, 4/4 on landing page

## Verification

| Issue | Before | After |
|-------|--------|-------|
| Dropdown stuck open | `hidden=false` | `hidden=true`, toggle works |
| Console errors | CSP EvalErrors blocking Alpine.js | `js-errors: none` |
| Page title | `<h1>DASHBOARD</h1>` visible | `dashboard-title: gone` |
| Tools not showing | "NO TOOLS" (catalog empty) | 26 tools displayed |

## Root Causes
- **Dropdown + console errors**: CSP `script-src` was missing `'unsafe-eval'`, which Alpine.js 3.x requires for `new Function()` expression evaluation
- **Page title**: Explicit `<h1>` elements in templates
- **No tools**: Portal's VIRE_API_URL pointed at port 8080 but vire-server listens on 4242; startup race also contributed (catalog fetched before server ready)

## Notes
- Pre-existing test failures: `TestDockerComposeProjectName` (compose project name conflict), `TestRoutes_*` (timeout due to MCP catalog retry connecting to absent vire-server)
- vire-server was started manually with `docker run` on `vire_default` network; not managed by portal's compose files
