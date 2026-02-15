# Summary: Service Status Indicators

**Date:** 2026-02-15
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/handlers/server_health.go` | New handler proxying health checks to vire-server (3s timeout, returns 200/503) |
| `internal/handlers/handlers_test.go` | 4 tests: upstream healthy, unreachable, non-200, rejects non-GET |
| `internal/app/app.go` | Added `ServerHealthHandler` field and wiring in `initHandlers()` |
| `internal/server/routes.go` | Added `GET /api/server-health` route |
| `pages/partials/nav.html` | Added status indicator dots with Alpine `statusIndicators()` component |
| `pages/static/common.js` | Added `statusIndicators()` Alpine component (polls every 5s) |
| `pages/static/css/portal.css` | Added `.status-indicators`, `.status-dot`, `.status-up/startup/down` styles |
| `.claude/skills/develop/SKILL.md` | Added `GET /api/server-health` to routes table |

## Tests
- `TestServerHealthHandler_ReturnsOKWhenUpstreamHealthy` — PASS
- `TestServerHealthHandler_Returns503WhenUpstreamUnreachable` — PASS
- `TestServerHealthHandler_Returns503WhenUpstreamReturnsNon200` — PASS
- `TestServerHealthHandler_RejectsNonGET` — PASS
- `go vet ./...` — clean
- browser-check: dashboard indicators visible, 2 dots present, no JS errors
- browser-check: landing page smoke test, no JS errors
- browser-check: dropdown still works with indicators present

## Documentation Updated
- `.claude/skills/develop/SKILL.md` routes table — added `GET /api/server-health`

## Devils-Advocate Findings
- SSRF risk: mitigated — apiURL comes from server config, not user input
- Timeout handling: 3-second context timeout on upstream requests
- No rate limiting needed — browser polls every 5s per tab, server-side is a simple proxy
- AbortSignal.timeout browser support: adequate for modern browsers
- setInterval without cleanup: acceptable — indicators live for page lifetime

## Notes
- Polling (5s interval) chosen over WebSockets for simplicity — only 2 health checks
- Indicators start orange (startup), transition to green/red after first poll
- Portal dot is always green when visible (if portal is down, page wouldn't load)
- Server dot reflects vire-server reachability via proxy to avoid CORS
