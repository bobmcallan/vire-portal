# Summary: Protocol Update, Navigation Menu, Session-Aware Menu, Client Logging

**Date:** 2026-02-15
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `internal/models/user.go` | Added `NavexaKey` field to User struct |
| `data/users.json` | Added `navexa_key` field to seed users |
| `internal/mcp/context.go` | New: UserContext type with context key helpers |
| `internal/mcp/handler.go` | Added userLookupFn, JWT extraction in ServeHTTP, withUserContext method |
| `internal/mcp/proxy.go` | Per-request X-Vire-User-ID and X-Vire-Navexa-Key injection from context, CRLF sanitization |
| `internal/app/app.go` | Passes user lookup closure (BadgerDB) to MCP handler |
| `pages/partials/nav.html` | Full rewrite: centered DASHBOARD link, right burger with SETTINGS/LOGOUT dropdown |
| `pages/static/css/portal.css` | Added nav menu, burger, dropdown styles |
| `internal/handlers/landing.go` | Checks vire_session cookie, passes LoggedIn to templates |
| `internal/handlers/dashboard.go` | Checks vire_session cookie, passes LoggedIn to templates |
| `internal/handlers/auth.go` | Added HandleLogout (clears cookie, redirects to /) |
| `internal/server/routes.go` | Added POST /api/auth/logout and GET /settings routes |
| `pages/landing.html` | Conditional nav: `{{if .LoggedIn}}{{template "nav.html" .}}{{end}}` |
| `pages/dashboard.html` | Conditional nav based on LoggedIn |
| `pages/settings.html` | New: placeholder settings page |
| `pages/partials/head.html` | Inline `window.VIRE_CLIENT_DEBUG = true` when DevMode |
| `internal/mcp/mcp_test.go` | Updated NewHandler calls with userLookupFn, added user context header tests |
| `internal/handlers/handlers_test.go` | Added logout and LoggedIn tests |
| `internal/server/routes_test.go` | Added new route tests |

## Tests
- All Go tests pass (`go test ./...`)
- Go vet clean
- Docker container builds and deploys
- Health endpoint responds
- Dev login → dashboard with nav visible
- Logout clears cookie and redirects to landing
- Settings page renders

## Documentation Updated
- README.md — updated routes table
- .claude/skills/develop/SKILL.md — updated routes reference

## Devils-Advocate Findings
- CRLF header injection: fixed with `sanitizeHeaderValue()` stripping \r\n from user-controlled header values
- JWT parsing: gracefully handles malformed tokens (empty, no dots, invalid base64)
- Cookie handling: HttpOnly, MaxAge -1 on logout

## Notes
- `window.VIRE_CLIENT_DEBUG` set inline before `common.js` loads — dev mode now shows debug logs automatically
- Nav menu uses Alpine.js `x-data`/`x-show` for burger dropdown with `@click.outside` to close
- X-Vire-User-ID and X-Vire-Navexa-Key are per-request from authenticated session; static headers (Portfolios, DisplayCurrency) remain from config
- Settings page is a placeholder — UI for editing settings is future work
