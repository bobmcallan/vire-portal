---
name: vire-portal-architect
description: Architectural guardrails for vire-portal code changes. Apply whenever writing, reviewing, or modifying Go handlers, templates, Alpine.js components, auth flows, or API client code. Prevents structural defects before they're introduced.
---

# Vire Portal Architecture Rules

These rules are derived from the actual vire-portal codebase patterns. Every rule references real code.

## Rule 1: Data Ownership — vire-server Owns All Data

The portal is **stateless**. All user data, portfolio calculations, and business logic live in vire-server. The portal only renders and proxies.

**Correct:**
```go
// Handler fetches data from vire-server, passes to template
data, err := h.proxyGetFn("/api/portfolios", claims.Sub)
```

**Violation:**
```go
// Portal computing derived values from raw data
capitalGainPct := (currentValue - costBasis) / costBasis * 100
```

**Check for:**
- Arithmetic on portfolio/financial data in `internal/handlers/` — calculations belong in vire-server
- New fields derived from existing fields in handlers — fetch the computed value from the API instead
- Caching or storing user data locally (except `internal/cache/` response cache with TTL)

## Rule 2: Handler Pattern — Struct + Dependency Injection

All handlers follow the same structure. New handlers must match it.

**Pattern:**
```go
type XxxHandler struct {
    logger      *common.Logger
    templates   *template.Template
    devMode     bool
    jwtSecret   []byte
    apiURL      string
    proxyGetFn  func(path, userID string) ([]byte, error)  // SSR data
    userLookupFn func(userID string) (*models.User, error)  // user context
}

func NewXxxHandler(logger *common.Logger, devMode bool, jwtSecret []byte) *XxxHandler {
    return &XxxHandler{
        logger:    logger,
        templates: template.Must(template.ParseGlob(FindPagesDir() + "/*.html")),
        devMode:   devMode,
        jwtSecret: jwtSecret,
    }
}
```

**Auth check at top of every protected handler:**
```go
loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
if !loggedIn {
    http.Redirect(w, r, "/", http.StatusFound)
    return
}
```

**Check for:**
- Handlers accessing global state or singletons instead of injected dependencies
- Missing `IsLoggedIn` check on protected routes
- Direct HTTP calls to vire-server — use `proxyGetFn` or `VireClient` methods
- Template loading at request time — templates are pre-loaded in `New*()` constructors

## Rule 3: SSR Data Embedding — template.JS for Trusted Data

Pages use two SSR patterns. Choose the right one.

**Pattern A — Pure SSR (static content, no Alpine interactivity needed):**
```go
// Handler builds template data directly
data := map[string]interface{}{
    "Categories": categories,  // Go structs rendered by {{range}}
}
h.templates.ExecuteTemplate(w, "glossary.html", data)
```
Used by: `error.html`, `landing.html`, `glossary.html`

**Pattern B — JSON Hydration (Alpine.js needs the data):**
```go
// Handler marshals JSON, passes as template.JS
jsonBytes, _ := json.Marshal(apiResponse)
data := map[string]interface{}{
    "DataJSON": template.JS(string(jsonBytes)),
}
```
Template side:
```html
<script>window.__VIRE_DATA__ = { portfolios: {{.PortfoliosJSON}} };</script>
```
Alpine reads on init, then cleans up:
```javascript
const d = window.__VIRE_DATA__;
delete window.__VIRE_DATA__;
```
Used by: `cash.html`, `strategy.html`, `changelog.html`, `help.html`

**Dashboard exception:** Dashboard remains client-side Alpine (no SSR) due to complexity.

**Check for:**
- `template.JS` used with user-supplied input — only safe for trusted vire-server data
- Missing `delete window.__VIRE_DATA__` cleanup after Alpine reads it
- New pages using client-side fetch when SSR would be simpler
- `template.HTML` used where `template.JS` is needed (different escaping)

## Rule 4: Template Safety

Go's `html/template` auto-escapes HTML context. The portal uses:
- `{{.Field}}` — auto-escaped (safe for user-visible text)
- `template.JS` — for JSON in `<script>` blocks (bypasses HTML escaping)
- Alpine.js `x-text` — safe (text-only, no HTML injection)
- Alpine.js `x-html` — avoid unless rendering trusted server HTML

**Check for:**
- `template.HTML` wrapping user input — XSS risk
- `x-html` binding with user-controlled content
- Raw string concatenation in JavaScript with user data
- Missing CSRF token in forms (`_csrf` cookie + hidden field)

## Rule 5: Middleware Stack Awareness

Eight middleware layers wrap all requests (see `internal/server/middleware.go`). New routes automatically get:
- Correlation ID tracking (`X-Correlation-ID`)
- Request logging with duration
- Security headers (CSP, X-Frame-Options)
- CORS headers
- CSRF protection (skipped for `/api/*`, `/mcp*`, OAuth endpoints)
- Body size limits (1MB default, 10MB for `/mcp`)
- Panic recovery

**Check for:**
- Routes that need CSRF exemption — add to skip list in `csrfMiddleware`
- Routes needing larger body limits — update `maxBodySizeMiddleware`
- New middleware breaking the existing chain order

## Rule 6: API Client Pattern

All communication with vire-server goes through `internal/client/vire_client.go`.

**Existing methods:**
- `GetUser(userID)` → GET /api/users/{id}
- `SaveUser(userID, user)` → PUT /api/users/{id}
- `ProxyGet(path, userID)` → GET /api/* with X-Vire-User-ID header
- `AdminListUsers(userID)` → GET /api/admin/users
- `ExchangeOAuth(provider, code, state)` → POST /api/auth/oauth
- `RegisterService(serviceID, serviceKey)` → POST /api/services/register

**All methods inject these headers:**
- `X-Vire-User-ID` — authenticated user's JWT sub claim
- `X-Vire-Portfolios` — from config
- `X-Vire-Display-Currency` — from config

**Check for:**
- Direct `http.Get/Post` calls to vire-server URLs — use VireClient
- Missing error handling on VireClient responses
- New endpoints not following the response format: `{"status":"ok","data":{...}}`

## Rule 7: Error Handling Conventions

| Context | Pattern |
|---------|---------|
| API endpoints | `WriteError(w, statusCode, "message")` → `{"status":"error","error":"..."}` |
| Page handlers | `http.Redirect(w, r, "/error?reason=key", http.StatusFound)` |
| Internal errors | `h.logger.Error().Str("handler", "name").Err(err).Msg("description")` |
| Auth failures | Redirect to `/` (landing page) |

Error page reason keys are an allowlist in `ServeErrorPage()` — adding a new reason requires updating the map.

**Check for:**
- `http.Error()` used for page handlers (should redirect to `/error?reason=`)
- Missing structured logging on error paths
- New error reasons not added to the allowlist map
