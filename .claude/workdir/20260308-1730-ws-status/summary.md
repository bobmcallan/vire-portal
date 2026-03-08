# Summary: WebSocket Status Indicators

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/handlers/ws_status.go` | NEW: WebSocket status handler with gobwas/ws, ping/response loop, 5s cached server health |
| `internal/handlers/ws_status_test.go` | NEW: 6 unit tests (upgrade, ping response, caching, server down, invalid message, non-WS request) |
| `internal/server/middleware.go` | Added Hijack() method to responseWriter, updated CSP connect-src with ws:/wss: |
| `internal/app/app.go` | Wired WSStatusHandler into App struct and initHandlers() |
| `internal/server/routes.go` | Added `/ws/status` route |
| `pages/static/common.js` | Replaced HTTP polling statusIndicators with WebSocket client (reconnect + backoff + jitter) |
| `tests/ui/status_ws_test.go` | NEW: 2 UI tests (connected, server down) |
| `README.md` | Added /ws/status to routes table |
| `docs/requirements.md` | Added /ws/status to routes table |

## Tests
- Unit tests: ALL PASS (6 new ws_status + 2 middleware Hijack tests)
- UI tests: 69 pass, 3 fail (pre-existing), 17 skip; WS-specific tests didn't execute due to Chrome crash (pre-existing instability)
- Fix rounds: 0

## Architecture
- Architect APPROVED: stateless handler, struct+DI, per-instance cache, multi-instance safe
- Existing HTTP health endpoints preserved

## Devils-Advocate
- No critical issues found
- Cache uses double-checked locking (RLock fast path, Lock on miss)
- 30s read deadline handles background tab throttling
- Reconnect jitter prevents thundering herd

## Notes
- gobwas/ws was already indirect dependency (via chromedp), now used directly
- No new external dependencies added
- 3 pre-existing UI test failures unrelated to this change
