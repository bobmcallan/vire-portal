# Requirements: Rename Settings → Profile Page

**Date:** 2026-02-28
**Slug:** profile-page

## Scope

Rename "Settings" to "Profile" throughout the portal. The profile page shows user identity info (email, name, auth method) plus existing Navexa key settings. Email field is read-only when user authenticated via OAuth.

## Changes Required

### 1. Handler Rename (`internal/handlers/settings.go` → `internal/handlers/profile.go`)

- Rename file from `settings.go` to `profile.go`
- Rename struct `SettingsHandler` → `ProfileHandler`
- Rename constructor `NewSettingsHandler` → `NewProfileHandler`
- Rename methods `HandleSettings` → `HandleProfile`, `HandleSaveSettings` → `HandleSaveProfile`
- Update all comments and log messages from "settings" to "profile"
- Redirect after save: `/settings?saved=1` → `/profile?saved=1`
- Template name: `settings.html` → `profile.html`
- Page identifier in data map: `"settings"` → `"profile"`
- **New data fields** passed to template:
  - `UserEmail` — from JWT claims (`claims.Email`) or UserProfile
  - `UserName` — from JWT claims (`claims.Name`) or UserProfile (`user.Username`)
  - `AuthMethod` — from JWT claims (`claims.Provider`), e.g. "dev", "google", "github"
  - `IsOAuth` — boolean, true when Provider is "google" or "github" (email is locked)

### 2. Template Rename (`pages/settings.html` → `pages/profile.html`)

- Rename file
- Update `<title>` from `VIRE SETTINGS` → `VIRE PROFILE`
- Update form action from `/settings` to `/profile`
- Update success banner text: "Settings saved successfully." → "Profile saved successfully."
- Update unauthenticated message: link text to "manage your profile"
- **Add new section** above Navexa key section: "USER PROFILE"
  - EMAIL field: show `{{.UserEmail}}` — if `{{.IsOAuth}}`, render as a locked/read-only display field (not editable input), otherwise show as text
  - NAME field: show `{{.UserName}}`
  - AUTH METHOD field: show `{{.AuthMethod}}` uppercased
  - Use existing `.dashboard-field` / `.dashboard-label` pattern for display fields
- Keep Navexa API Key section unchanged below

### 3. Navigation (`pages/partials/nav.html`)

- Line 24: `<a href="/settings">Settings</a>` → `<a href="/profile">Profile</a>`
- Line 44: `<a href="/settings">Settings</a>` → `<a href="/profile">Profile</a>`

### 4. App Struct & Init (`internal/app/app.go`)

- Rename field `SettingsHandler *handlers.SettingsHandler` → `ProfileHandler *handlers.ProfileHandler`
- Update all references: `a.SettingsHandler` → `a.ProfileHandler`
- Update comments from "settings" to "profile"
- Update `NewSettingsHandler` → `NewProfileHandler`

### 5. Routes (`internal/server/routes.go`)

- Line 55-57: Change routes from `/settings` to `/profile`
- Update comment from "Settings page" to "Profile page"
- Change `s.app.SettingsHandler.HandleSettings` → `s.app.ProfileHandler.HandleProfile`
- Change `s.app.SettingsHandler.HandleSaveSettings` → `s.app.ProfileHandler.HandleSaveProfile`

### 6. Tests

#### `internal/handlers/handlers_test.go`
- Rename all `TestSettingsHandler_*` test functions to `TestProfileHandler_*`
- Update `NewSettingsHandler` → `NewProfileHandler` calls
- Update URLs from `/settings` to `/profile`
- Update assertions checking for "SETTINGS" → "PROFILE" in body content
- Update redirect assertions: `/settings?saved=1` → `/profile?saved=1`
- Add new test: `TestProfileHandler_GET_ShowsUserInfo` — verifies email, name, auth method rendered
- Add new test: `TestProfileHandler_GET_OAuthEmailLocked` — verifies email not in an editable input when IsOAuth=true

#### `internal/server/routes_test.go`
- Rename `TestRoutes_SettingsPage` → `TestRoutes_ProfilePage`
- Rename `TestRoutes_SettingsPostRoute` → `TestRoutes_ProfilePostRoute`
- Rename `TestRoutes_SettingsPostBlockedByCSRF` → `TestRoutes_ProfilePostBlockedByCSRF`
- Update all URL paths from `/settings` to `/profile`
- Update body assertions from "SETTINGS" → "PROFILE"

#### `tests/ui/settings_test.go` → `tests/ui/profile_test.go`
- Rename file
- Rename all `TestSettings*` → `TestProfile*` functions
- Update URLs from `/settings` to `/profile`
- Update screenshot prefixes from "settings" to "profile"
- Update error messages from "settings" to "profile"
- Add new tests:
  - `TestProfileUserInfoSection` — verifies user profile section visible with email, name, auth method
  - `TestProfileEmailLockedForOAuth` — verifies email field is not editable for OAuth users (dev mode uses "dev" provider, so this may need a separate check)

#### `tests/ui/nav_test.go`
- `TestNavSettingsInDropdown` → `TestNavProfileInDropdown`
- Update selector from `.nav-dropdown a[href='/settings']` to `.nav-dropdown a[href='/profile']`
- Update screenshot name from "settings-in-dropdown.png" to "profile-in-dropdown.png"
- Update error message from "settings link" to "profile link"

### 7. Other References

- `pages/dashboard.html`: check if any links to `/settings` (warning banner links)
- `internal/handlers/handlers_test.go`: nav dropdown test checking `href="/settings"`
- `README.md`: update route table if `/settings` is documented
- `.claude/skills/develop/SKILL.md`: update routes table

### 8. CSS

- Rename `.settings-key-status` → `.profile-key-status` and `.settings-key-missing` → `.profile-key-missing` in both CSS and template
- No new CSS needed — reuse existing `.dashboard-field` / `.dashboard-label` for user info display

## Files Expected to Change

| File | Action |
|------|--------|
| `internal/handlers/settings.go` | Delete (renamed) |
| `internal/handlers/profile.go` | Create (renamed from settings.go) |
| `pages/settings.html` | Delete (renamed) |
| `pages/profile.html` | Create (renamed from settings.html, add user info) |
| `pages/partials/nav.html` | Edit (Settings→Profile, /settings→/profile) |
| `internal/app/app.go` | Edit (SettingsHandler→ProfileHandler) |
| `internal/server/routes.go` | Edit (/settings→/profile) |
| `internal/server/routes_test.go` | Edit (rename tests, update paths) |
| `internal/handlers/handlers_test.go` | Edit (rename tests, update paths, add new tests) |
| `tests/ui/settings_test.go` | Delete (renamed) |
| `tests/ui/profile_test.go` | Create (renamed, add new tests) |
| `tests/ui/nav_test.go` | Edit (update dropdown test) |
| `pages/static/css/portal.css` | Edit (rename settings CSS classes) |
| `pages/dashboard.html` | Edit if links to /settings |
| `README.md` | Edit if /settings documented |
