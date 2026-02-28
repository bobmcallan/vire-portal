# Summary: Settings → Profile Page Rename

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/handlers/profile.go` | Created (renamed from settings.go). ProfileHandler with new UserEmail, UserName, AuthMethod, IsOAuth fields |
| `internal/handlers/settings.go` | Deleted |
| `pages/profile.html` | Created (renamed from settings.html). Added USER PROFILE section with email, name, auth method display |
| `pages/settings.html` | Deleted |
| `pages/partials/nav.html` | Settings→Profile in dropdown and mobile menu |
| `internal/app/app.go` | SettingsHandler→ProfileHandler throughout |
| `internal/server/routes.go` | /settings→/profile routes |
| `pages/static/css/portal.css` | .settings-key-*→.profile-key-* |
| `pages/dashboard.html` | /settings→/profile warning banner links |
| `pages/strategy.html` | /settings→/profile warning banner links |
| `pages/capital.html` | /settings→/profile warning banner links |
| `pages/docs.html` | /settings→/profile configuration link |
| `internal/handlers/handlers_test.go` | 25 tests renamed, 2 new tests added (ShowsUserInfo, OAuthEmailLocked) |
| `internal/server/routes_test.go` | 3 tests renamed, URLs updated |
| `internal/handlers/docs_stress_test.go` | Stale /settings refs updated |
| `tests/ui/profile_test.go` | Created (renamed from settings_test.go). 13 tests including ProfileUserInfoSection |
| `tests/ui/settings_test.go` | Deleted |
| `tests/ui/nav_test.go` | TestNavSettingsInDropdown→TestNavProfileInDropdown |
| `README.md` | Route table, file tree, architecture diagram updated |
| `docs/` (9 files) | All stale /settings references cleaned |
| `.claude/skills/develop/SKILL.md` | Route table and test runner reference updated |
| `.claude/skills/test-common/SKILL.md` | Example code updated |

## Tests
- Unit tests: 30 ProfileHandler tests pass, 3 Profile route tests pass
- UI tests: 13 profile tests created, 10 total UI tests pass
- UI test failures: 10 (all pre-existing dev auth blocker, unrelated to this feature)
- Fix rounds: 1 (stale references caught by devils-advocate, fixed by team lead)

## Architecture
- Architect signed off: handler patterns, route registration, template data flow all follow conventions
- No new dependencies introduced
- Auth flow preserved (unauthenticated redirect, CSRF protection)

## Devils-Advocate
- Flagged stale /settings references in 3 templates + 2 test files — resolved
- XSS: html/template auto-escaping handles malicious user data
- CSRF: POST /profile still protected by middleware
- IsOAuth derived server-side from JWT — cannot be spoofed

## Notes
- Dev auth /api/auth/login endpoint remains broken (pre-existing, unrelated)
- The profile page now shows user identity (email, name, auth method) above the Navexa key section
- Email is displayed as read-only when authenticated via OAuth (Google/GitHub)
