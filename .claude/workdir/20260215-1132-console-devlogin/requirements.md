# Requirements: Console Logging + Dev Login Fix

**Date:** 2026-02-15
**Requested:** Port quaero console logging to vire-portal; fix dev login redirect + button styling

## Scope

### In scope
- Port `debugLog`/`debugError` from quaero to vire-portal's `common.js`
- Load `common.js` in `head.html`
- Fix dev login redirect: `/` -> `/dashboard`
- Add `.btn-dev` CSS class: full width, dark soft red text
- Update auth handler test for new redirect target

### Out of scope
- Session validation middleware
- Auth middleware for protected routes
- Alpine.js components

## Approach

### 1. Console logging (`pages/static/common.js`)
Port from quaero's `common.js`:
- `window.VIRE_DEBUG` flag (default false, overridable in browser console)
- `window.debugLog(component, message, ...args)` — conditional timestamped logging
- `window.debugError(component, message, error)` — always-on error logging with stack traces
- Keep existing `alpine:init` listener

### 2. Load common.js (`pages/partials/head.html`)
Add `<script defer src="/static/common.js"></script>` before Alpine.js script

### 3. Fix dev login redirect (`internal/handlers/auth.go`)
Change line 43: `http.Redirect(w, r, "/", http.StatusFound)` -> `http.Redirect(w, r, "/dashboard", http.StatusFound)`

### 4. Dev button styling (`pages/static/css/portal.css`)
Add `.btn-dev` class:
- Full width like `.btn-primary` and `.btn-secondary` in the login section
- Dark soft red text color (e.g., `#a33`)
- Same border, padding, and font as other buttons

### 5. Landing page (`pages/landing.html`)
Ensure dev button form/button has correct classes for full-width layout

### 6. Tests
- Update `internal/handlers/handlers_test.go` — dev login test expects redirect to `/dashboard`

## Files Expected to Change
- `pages/static/common.js` — add logging functions
- `pages/partials/head.html` — load common.js
- `internal/handlers/auth.go` — fix redirect
- `internal/handlers/handlers_test.go` — update redirect assertion
- `pages/static/css/portal.css` — add .btn-dev style
- `pages/landing.html` — ensure button classes are correct
