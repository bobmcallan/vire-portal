# Requirements: Service Status Indicators

**Date:** 2026-02-15
**Requested:** Two small round status indicators in the top-right nav bar: 'P' (portal) and 'S' (server). Green=on, orange=startup, red=down. Inspired by quaero's WebSocket-based status monitoring.

## Scope
- Add two status indicator dots to the nav bar (visible on all authenticated pages)
- Alpine.js component that polls health endpoints
- Portal indicator: poll `/api/health` (self)
- Server indicator: poll vire-server health via new proxy endpoint `/api/server-health`
- CSS for round dot indicators with color states
- Not in scope: full WebSocket implementation (polling is sufficient for two health checks)

## Approach

**Why polling over WebSockets:** The portal only needs two binary health checks. WebSockets add significant complexity (gorilla/websocket dependency, connection management, broadcasting). A simple fetch poll every 5 seconds is adequate and keeps the stack simple.

**Alpine.js component:** `statusIndicators()` in `common.js`
- State: `portal: 'startup'`, `server: 'startup'` (startup=orange on page load)
- On `init()`: start polling both endpoints every 5 seconds
- `/api/health` → 200 = green, else = red
- `/api/server-health` → 200 = green, else = red
- Both start orange, transition to green/red after first response

**Server-side:** New handler `/api/server-health` that proxies to vire-server's health endpoint.
- Returns 200 if vire-server responds OK
- Returns 503 if vire-server is unreachable
- This avoids CORS issues from the browser calling vire-server directly

**Nav placement:** After the nav-links `<ul>`, before the hamburger button. Two small dots with single-letter labels.

**CSS:** Small round dots (10px), soft colors with slight glow. Positioned inline in the nav.

## Files Expected to Change
- `pages/partials/nav.html` — add indicator markup
- `pages/static/common.js` — add `statusIndicators()` Alpine component
- `pages/static/css/portal.css` — add indicator styles
- `internal/handlers/server_health.go` — new proxy health handler
- `internal/handlers/server_health_test.go` — tests
- `internal/app/app.go` — wire new handler
- `internal/server/routes.go` — add route
