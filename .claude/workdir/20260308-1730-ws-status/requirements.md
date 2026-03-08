# Requirements: WebSocket Status Indicators

## Scope

**What it does:**
- Replace HTTP polling status indicators with a WebSocket connection
- Client sends `ping` text messages every 5s, server responds with JSON `{"portal":"up","server":"up"}`
- Server caches vire-server health check result for 5s (not per-ping)
- Client reconnects on disconnect with exponential backoff + jitter
- Server closes idle connections after 30s (handles background tab throttling)

**What it does NOT do:**
- Remove existing HTTP health endpoints (`/api/health`, `/api/server-health`) — external monitoring uses them
- Add authentication to the WebSocket endpoint — mirrors existing unauthenticated health endpoints
- Server-push model — client drives with ping, server responds (no connection tracking)
- Change CSS or HTML structure — same status-dot styling, same nav template structure

## File Changes

### 1. NEW: `internal/handlers/ws_status.go`

WebSocket status handler using `gobwas/ws` (already in go.mod as indirect).

**Struct:**
```go
type WSStatusHandler struct {
    logger    *common.Logger
    apiURL    string
    mu        sync.RWMutex
    cached    wsHealthResult
    cacheTime time.Time
    cacheTTL  time.Duration
}

type wsHealthResult struct {
    Portal string `json:"portal"`
    Server string `json:"server"`
}
```

**Constructor:**
```go
func NewWSStatusHandler(logger *common.Logger, apiURL string) *WSStatusHandler
```

**Methods:**
```go
func (h *WSStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)
func (h *WSStatusHandler) getStatus() wsHealthResult
func (h *WSStatusHandler) checkServerHealth() string
```

**Behavior:**
- `ServeHTTP`: Upgrade to WebSocket via `ws.Upgrade(r, w, nil)`. Loop: set 30s read deadline, read message, if text frame with "ping" content, respond with JSON `{"portal":"up","server":"<cached>"}`. On error/close, break loop.
- `getStatus`: Returns cached result if within 5s TTL. Otherwise calls `checkServerHealth()`, caches, returns. Uses `sync.RWMutex` for thread safety.
- `checkServerHealth`: HTTP GET to `apiURL + "/api/health"` with 3s timeout. Returns "up" or "down". Pattern follows `server_health.go`.
- Portal is always "up" (if WS is connected, portal is reachable).

**Pattern reference:** `internal/handlers/server_health.go` for the upstream health check pattern.

### 2. MODIFY: `internal/server/middleware.go`

Add `Hijack()` method to `responseWriter` struct. `gobwas/ws.Upgrade()` calls `w.(http.Hijacker).Hijack()` directly (type assertion, not Unwrap).

```go
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
    if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
        return h.Hijack()
    }
    return nil, nil, fmt.Errorf("underlying ResponseWriter does not support Hijack")
}
```

Add imports: `"bufio"`, `"net"`.

### 3. MODIFY: `internal/app/app.go`

Add to App struct:
```go
WSStatusHandler *handlers.WSStatusHandler
```

Add to `initHandlers()` (after ServerHealthHandler):
```go
a.WSStatusHandler = handlers.NewWSStatusHandler(a.Logger, a.Config.API.URL)
```

### 4. MODIFY: `internal/server/routes.go`

Add route (before API routes section, after MCP endpoint):
```go
// WebSocket status endpoint
mux.Handle("/ws/status", s.app.WSStatusHandler)
```

Also add `/ws/` to CSRF skip list in middleware.go (websocket upgrade is GET so CSRF skip is not needed — GET is already safe).

### 5. MODIFY: `pages/static/common.js`

Replace `statusIndicators` Alpine component (lines 99-117):

```javascript
// Status Indicators (WebSocket)
Alpine.data('statusIndicators', () => ({
    portal: 'startup',
    server: 'startup',
    _ws: null,
    _timer: null,
    _backoff: 1000,
    _maxBackoff: 30000,
    init() {
        this._connect();
    },
    destroy() {
        if (this._timer) clearInterval(this._timer);
        if (this._ws) this._ws.close();
    },
    _connect() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = `${proto}//${location.host}/ws/status`;
        try {
            this._ws = new WebSocket(url);
        } catch {
            this.portal = 'down';
            this.server = 'down';
            this._scheduleReconnect();
            return;
        }
        this._ws.onopen = () => {
            this._backoff = 1000;
            this._ping();
            if (this._timer) clearInterval(this._timer);
            this._timer = setInterval(() => this._ping(), 5000);
        };
        this._ws.onmessage = (e) => {
            try {
                const d = JSON.parse(e.data);
                this.portal = d.portal || 'down';
                this.server = d.server || 'down';
            } catch { /* ignore malformed */ }
        };
        this._ws.onclose = () => {
            this.portal = 'down';
            this.server = 'down';
            if (this._timer) { clearInterval(this._timer); this._timer = null; }
            this._scheduleReconnect();
        };
        this._ws.onerror = () => {}; // onclose fires after onerror
    },
    _ping() {
        if (this._ws && this._ws.readyState === WebSocket.OPEN) {
            this._ws.send('ping');
        }
    },
    _scheduleReconnect() {
        const jitter = Math.random() * 500;
        setTimeout(() => this._connect(), this._backoff + jitter);
        this._backoff = Math.min(this._backoff * 2, this._maxBackoff);
    },
}));
```

### 6. MODIFY: `internal/server/middleware.go` — CSP header

Update `connect-src` to include `ws:` and `wss:` explicitly (though `'self'` covers same-origin WebSocket in most browsers, being explicit is safer for cross-browser compat):

Change:
```
connect-src 'self' https://cdn.jsdelivr.net
```
To:
```
connect-src 'self' ws: wss: https://cdn.jsdelivr.net
```

### 7. MODIFY: `go.mod`

Move `gobwas/ws` from indirect to direct:
```
require (
    ...
    github.com/gobwas/ws v1.4.0
    ...
)
```
Run `go mod tidy` after implementation.

## Test Cases

### Unit tests: `internal/handlers/ws_status_test.go`

| Test Function | Validates |
|---|---|
| `TestWSStatusHandler_Upgrade` | WebSocket upgrade succeeds, ping gets JSON response |
| `TestWSStatusHandler_PingResponse` | Response contains portal and server fields |
| `TestWSStatusHandler_ServerHealthCaching` | Second ping within 5s returns cached result (no duplicate HTTP call) |
| `TestWSStatusHandler_ServerDown` | When vire-server unreachable, server field is "down", portal is "up" |
| `TestWSStatusHandler_InvalidMessage` | Non-"ping" text message gets no response or is ignored |
| `TestWSStatusHandler_NonWebSocketRequest` | Regular HTTP GET to /ws/status returns 400 |

### Middleware test: `internal/server/middleware_test.go`

| Test Function | Validates |
|---|---|
| `TestResponseWriter_Hijack` | responseWriter delegates Hijack to underlying writer |
| `TestResponseWriter_HijackNotSupported` | Returns error when underlying writer doesn't support Hijack |

### UI tests: `tests/ui/status_ws_test.go`

| Test Function | Validates |
|---|---|
| `TestStatusWebSocket_Connected` | After page load, both P and S dots become green (status-up) |
| `TestStatusWebSocket_ServerDown` | When vire-server is down, S dot is red, P dot is green |

## Edge Cases

1. **Background tab throttling**: Browser throttles timers in background tabs. The 5s ping interval may stretch. Server's 30s read deadline closes the connection. Client reconnects when tab becomes active.
2. **Load balancer timeout**: Fly.io proxy timeout is 60s for idle WebSocket. The 5s ping interval keeps the connection alive.
3. **Multiple portal instances**: Each instance manages its own WebSocket connections and health cache. No cross-instance coordination needed.
4. **Concurrent health cache access**: `sync.RWMutex` protects the cached health result. Multiple WebSocket connections on the same instance share the cache.
5. **Reconnect thundering herd**: Random jitter (0-500ms) on reconnect prevents all clients from reconnecting simultaneously after server restart.
6. **WebSocket upgrade through middleware**: The `responseWriter` wrapper must implement `http.Hijacker` for the upgrade to succeed. Without it, `gobwas/ws.Upgrade()` panics with type assertion failure.

## Dependencies

- `github.com/gobwas/ws` v1.4.0 — already in go.mod as indirect (via chromedp). Move to direct.
- No new external dependencies needed.
