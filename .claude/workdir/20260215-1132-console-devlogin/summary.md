# Summary: Console Logging + Dev Login Fix

**Date:** 2026-02-15
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `pages/static/common.js` | Added `debugLog`/`debugError` logging functions ported from quaero |
| `pages/partials/head.html` | Added `<script>` tag to load common.js |
| `internal/handlers/auth.go` | Changed dev login redirect from `/` to `/dashboard` |
| `internal/handlers/handlers_test.go` | Updated test assertion for redirect target |
| `pages/static/css/portal.css` | Added `.btn-dev` class — full width, dark soft red text |
| `pages/landing.html` | Updated dev login button classes/layout |

## Tests
- Handler tests pass (`go test ./internal/handlers/...`)
- Dev login returns 302 redirect to `/dashboard` (verified via curl)
- common.js loads with logging functions (verified via curl)

## Documentation Updated
- README.md and SKILL.md updated as needed

## Devils-Advocate Findings
- No critical issues found

## Notes
- `window.VIRE_DEBUG` defaults to `false` — toggle in browser console with `window.VIRE_DEBUG = true`
- Dev login sets `vire_session` cookie and redirects to `/dashboard`
