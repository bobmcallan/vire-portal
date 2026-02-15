# Requirements: Dashboard UI Fixes

**Date:** 2026-02-15
**Requested:** Fix four dashboard issues: stuck dropdown, console errors, remove page title, missing tools

## Scope
- Fix CSP blocking Alpine.js (causes dropdown + console errors)
- Remove page title from dashboard and settings pages
- Fix tools not appearing (startup race condition)

## Approach
1. Add `'unsafe-eval'` to CSP `script-src` in `internal/server/middleware.go` — Alpine.js requires `new Function()` for expression evaluation
2. Remove `<h1 class="dashboard-title">` from `pages/dashboard.html` and `pages/settings.html`
3. Redeploy — vire-server is already running; portal will fetch catalog on restart
4. Update CSP test in `internal/server/middleware_test.go`

## Files Expected to Change
- `internal/server/middleware.go` (CSP header)
- `internal/server/middleware_test.go` (CSP test)
- `pages/dashboard.html` (remove title)
- `pages/settings.html` (remove title)
