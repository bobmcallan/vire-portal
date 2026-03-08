# Summary: Admin Prompt Template Management Pages

**Feedback:** fb_3d4d763f
**Status:** completed

## Changes

| File | Change |
|------|--------|
| `internal/client/vire_client.go` | Added `AdminPrompt` struct + `AdminListPrompts`, `AdminGetPrompt`, `AdminSetPrompt` methods |
| `internal/handlers/prompts.go` | New `AdminPromptsHandler` with list, edit, save endpoints |
| `internal/handlers/prompts_test.go` | 13 unit tests (auth, access, errors, XSS, save) |
| `internal/app/app.go` | Wired `AdminPromptsHandler` into App struct and `initHandlers()` |
| `internal/server/routes.go` | Added 3 routes: GET list, GET edit, POST save |
| `pages/prompts.html` | Prompts list template (table: Name, Description, Override, Updated) |
| `pages/prompt-edit.html` | Prompt edit template (textarea, save button, Alpine hydration) |
| `pages/partials/nav.html` | Split "Admin" into "Users" + "Prompts" links (desktop + mobile) |
| `pages/static/common.js` | Added `promptEditor()` Alpine component |
| `pages/static/css/portal.css` | Added prompt textarea, meta, actions, save button styles |
| `tests/ui/prompts_test.go` | 9 UI tests (list layout, edit layout, elements, JS errors) |
| `tests/ui/nav_test.go` | 4 new tests (Users/Prompts links in desktop dropdown + mobile menu) |
| `internal/handlers/users_stress_test.go` | Fixed nav test to expect "Users" instead of "Admin" |

## Tests
- Unit tests: 13 added, all pass
- UI tests: 13 created/reviewed (9 prompts + 4 nav), all pass
- Test results: 67 pass, 3 fail (pre-existing), 17 skip
- Fix rounds: 0

## Architecture
- Architect APPROVED — follows users.go handler pattern exactly
- One minor fix applied: __VIRE_DATA__ cleanup

## Devils-Advocate
- Security review: all clear, no critical issues
- Auth gate identical to users.go
- XSS: Go html/template auto-escapes, Alpine x-model safe
- CSRF: acceptable for admin-only JSON endpoint

## Notes
- 3 pre-existing UI test failures unrelated to this feature
- Pre-existing client test timeout (RegisterService_Unreachable)
