# Summary: Landing Page Login Form

**Date:** 2026-02-22
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `pages/landing.html` | Replaced hidden dev-only login form with always-visible username/password form. Pre-populates dev credentials in dev mode. |
| `pages/static/css/portal.css` | Added styles for `.landing-divider`, `.landing-login-form`, `.btn-login`. Removed old `.landing-dev-login` styles. |
| `tests/ui/dev_auth_test.go` | Updated selectors from `.landing-dev-login` to `.landing-login-form` |

## Tests
- `TestDevAuthLandingNoCookie` - PASS (login form visible)
- `TestDevAuthLoginRedirect` - PASS (login redirects to dashboard)
- `TestDevAuthCookieAndJWT` - PASS (session validation)
- `TestDevAuthLogout` - PASS (logout works)
- `TestDevAuthSettingsMCPEndpoint` - FAIL (pre-existing, unrelated to this change)
- `TestDevAuthSettingsMCPURL` - FAIL (pre-existing, unrelated to this change)

## Implementation Details

The login form is now always visible on the landing page (when server is healthy). In dev mode, the username and password fields are pre-populated with `dev_user` and `dev123` using Go template conditionals.

Form structure:
```html
<form method="POST" action="/api/auth/login" class="landing-login-form">
    <input type="text" name="username" placeholder="Username" value="{{if .DevMode}}dev_user{{end}}" required>
    <input type="password" name="password" placeholder="Password" value="{{if .DevMode}}dev123{{end}}" required>
    <button type="submit" class="btn btn-login">SIGN IN</button>
</form>
```

## Notes
- The bin/pages directory has permission issues (owned by root) which prevented the run.sh script from updating pages. Tests were run using `go run` directly from source.
- Two pre-existing test failures in settings page tests are unrelated to this change.
