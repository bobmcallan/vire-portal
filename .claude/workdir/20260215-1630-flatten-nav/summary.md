# Summary: Flatten nav — remove dropdown

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `pages/partials/nav.html` | Replaced dropdown with flat links: Dashboard, Settings, Logout |
| `pages/static/css/portal.css` | Removed entire DROPDOWN section (~70 lines), added `.nav-logout` and `.nav-logout-form` styles |
| `internal/handlers/handlers_test.go` | Renamed `TestNavTemplate_DropdownPresent` → `TestNavTemplate_FlatLinksPresent`, updated assertions |
| `tests/ui/ui_test.go` | Replaced `TestUIDropdownsClosedOnLoad` + `TestUIDropdownOpensAndCloses` with `TestUINavLinksPresent` |
| `.claude/skills/browser-check/SKILL.md` | Updated examples: dropdown references → flat nav checks |
| `.claude/skills/develop/SKILL.md` | Updated browser-check examples in Phase 2b |

## Tests
- `TestNavTemplate_FlatLinksPresent` — PASS
- `go vet ./...` — clean
- curl verified: logged-in HTML contains nav-links, nav-logout, Dashboard, Settings, Logout; no dropdown-menu or dropdown-trigger
- Puppeteer screenshot confirms flat nav rendering

## Notes
- The `dropdown()` Alpine component definition remains in `common.js` (unused but harmless)
- Settings link now has `active` class support: `{{if eq .Page "settings"}}class="active"{{end}}`
- Logout styled as a button matching nav link appearance (form POST for CSRF safety)
- Mobile menu unchanged — already had flat links
