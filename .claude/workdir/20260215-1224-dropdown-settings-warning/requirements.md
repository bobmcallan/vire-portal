# Requirements: Fix Dropdown, Settings Page with Navexa Key, Dashboard Warning

**Date:** 2026-02-15
**Requested:** Fix permanently-open burger dropdown, build settings page for Navexa key entry, add dashboard warning when key is not set

## Scope

### In scope
1. Fix burger dropdown permanently visible (Alpine.js x-cloak issue)
2. Settings page with form to enter/update Navexa API key, saved to user profile in BadgerDB
3. Dashboard warning banner when user's Navexa API key is not set

### Out of scope
- Other user settings beyond Navexa key
- API key validation against Navexa service
- Session validation middleware

## Approach

### Issue 1: Dropdown permanently visible

**Root cause:** Alpine.js is loaded with `defer`, so there's a flash between HTML rendering and Alpine initialization where `x-show="open"` has no effect — the dropdown is visible. Screenshot confirms SETTINGS/LOGOUT dropdown is permanently dropped.

**Fix:** Add `x-cloak` attribute to the dropdown div, and add `[x-cloak] { display: none !important; }` CSS rule. Alpine automatically removes `x-cloak` once initialized.

**Files:**
- `pages/partials/nav.html` — add `x-cloak` to `.nav-dropdown` div
- `pages/static/css/portal.css` — add `[x-cloak]` rule at the top (after the universal reset)

### Issue 2: Settings page with Navexa key

**New handler:** `internal/handlers/settings.go` — `SettingsHandler` struct:
- Fields: `logger`, `templates`, `devMode`, `userLookupFn func(string) (*models.User, error)`, `userSaveFn func(*models.User) error`
- `HandleSettings(w, r)` — GET: extract user from `vire_session` cookie JWT `sub` claim, look up in BadgerDB, render form with current NavexaKey status (set/not set). Don't show the actual key value — just show whether it's set and a masked preview of last 4 chars.
- `HandleSaveSettings(w, r)` — POST: extract user, validate input, save NavexaKey to user record in BadgerDB, redirect to `/settings` with success flash (via query param `?saved=1`)

**JWT extraction helper:** Add `extractJWTSub(token string) string` to handlers package (same logic as `internal/mcp/handler.go:extractJWTSub`). Small duplication is acceptable to avoid coupling handlers to mcp package.

**App wiring in `internal/app/app.go`:**
```go
userLookup := func(userID string) (*models.User, error) {
    store := a.StorageManager.DB().(*badgerhold.Store)
    var user models.User
    err := store.FindOne(&user, badgerhold.Where("Username").Eq(userID))
    return &user, err
}
userSave := func(user *models.User) error {
    store := a.StorageManager.DB().(*badgerhold.Store)
    return store.Upsert(user.Username, user)
}
a.SettingsHandler = handlers.NewSettingsHandler(a.Logger, a.Config.IsDevMode(), userLookup, userSave)
```

**Settings page template `pages/settings.html`:**
- Include nav (if LoggedIn)
- Form with:
  - Label "NAVEXA API KEY"
  - If key is set: show "Key set: ****XXXX" (last 4 chars) with a "Change" link/button
  - Text input (type=password) for entering/updating the key
  - Save button (btn-primary style)
- Success message when `?saved=1` query param present
- Error message when not logged in

**Routes in `internal/server/routes.go`:**
- Change `GET /settings` from PageHandler to SettingsHandler.HandleSettings
- Add `POST /settings` for SettingsHandler.HandleSaveSettings

### Issue 3: Dashboard Navexa key warning

**DashboardHandler changes:**
- Add `userLookupFn func(string) (*models.User, error)` field to DashboardHandler
- In `ServeHTTP`: extract user from cookie JWT, look up user, check if NavexaKey is empty
- Pass `NavexaKeyMissing bool` to template data
- If cookie missing or user lookup fails, don't show warning (user isn't authenticated)

**Dashboard template `pages/dashboard.html`:**
- Add warning banner BEFORE the first dashboard-section:
```html
{{if .NavexaKeyMissing}}
<div class="warning-banner">
    <strong>WARNING:</strong> Navexa API key not configured.
    <a href="/settings">Set your API key in Settings</a> to enable portfolio sync.
</div>
{{end}}
```

**CSS `pages/static/css/portal.css`:**
- `.warning-banner`: border 2px solid #a33, padding 1rem 1.5rem, color #a33, margin-bottom 2rem, font-size 0.875rem
- `.warning-banner a`: color #a33, font-weight 700

**App wiring:** Pass the same userLookup function to DashboardHandler.

## Files Expected to Change

| File | Change |
|------|--------|
| `pages/partials/nav.html` | Add `x-cloak` to dropdown div |
| `pages/static/css/portal.css` | Add `[x-cloak]` rule, `.warning-banner` styles |
| `internal/handlers/settings.go` | New: SettingsHandler with GET/POST handlers |
| `pages/settings.html` | Rewrite: Navexa key form |
| `internal/handlers/dashboard.go` | Add userLookupFn, pass NavexaKeyMissing |
| `pages/dashboard.html` | Add warning banner |
| `internal/app/app.go` | Wire SettingsHandler, pass userLookup to DashboardHandler |
| `internal/server/routes.go` | Update settings routes (GET + POST) |
| `internal/handlers/handlers_test.go` | Tests for settings handler, dashboard warning |
| `internal/server/routes_test.go` | Tests for new routes |

## Test Strategy
- Settings GET: renders form, shows key status
- Settings POST: saves key to DB, redirects with saved=1
- Settings POST without login: error response
- Dashboard: warning visible when NavexaKey empty, hidden when set
- Dropdown: x-cloak hides dropdown before Alpine init (visual check)
