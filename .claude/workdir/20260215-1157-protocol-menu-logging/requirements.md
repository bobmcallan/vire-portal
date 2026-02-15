# Requirements: Protocol Update, Navigation Menu, Session-Aware Menu, Client Logging

**Date:** 2026-02-15
**Requested:** Update MCP proxy headers per vire README, add top navigation menu, session-aware menu visibility, fix client-side console logging

## Scope

### In scope
1. Add X-Vire-User-ID and X-Vire-Navexa-Key headers to MCP proxy (per-request, from authenticated user)
2. Create top navigation bar: left brand, centered "DASHBOARD" link, right burger menu with "SETTINGS" and "LOGOUT"
3. Navigation appears on all pages only when user is logged in (has vire_session cookie)
4. Fix client-side console logging: tie VIRE_DEBUG to environment (dev mode = debug logs)
5. Add logout handler (POST /api/auth/logout) to clear session cookie
6. Add placeholder settings page

### Out of scope
- Session validation middleware (checking JWT validity/expiry)
- OAuth integration
- User settings editing UI

## Approach

### Part 1: Protocol update (X-Vire-User-ID, X-Vire-Navexa-Key)

Per the vire README, the portal must inject these per-user headers on proxied requests:
- `X-Vire-User-ID` — user identifier (JWT `sub` claim)
- `X-Vire-Navexa-Key` — user's Navexa API key (stored per-user in BadgerDB)

**Changes:**

1. **`internal/models/user.go`** — Add `NavexaKey string` field:
   ```go
   type User struct {
       Username  string `json:"username" badgerhold:"key"`
       Email     string `json:"email"`
       Password  string `json:"password"`
       Role      string `json:"role"`
       NavexaKey string `json:"navexa_key,omitempty"`
   }
   ```

2. **`data/users.json`** — Add `navexa_key` field to seed users (can be empty string for now)

3. **`internal/mcp/context.go`** (new) — Define context key type and helper functions:
   ```go
   type userContextKey struct{}
   type UserContext struct {
       UserID    string
       NavexaKey string
   }
   func WithUserContext(ctx context.Context, uc UserContext) context.Context
   func GetUserContext(ctx context.Context) (UserContext, bool)
   ```

4. **`internal/mcp/handler.go`** — Add user lookup function to Handler:
   - Add `userLookupFn func(userID string) (*UserContext, error)` field
   - In `ServeHTTP`, extract `vire_session` cookie, decode JWT `sub` claim, call lookup fn, inject into request context via `r.WithContext()`
   - JWT decode: base64url decode the payload (middle segment), unmarshal JSON, read `sub` field. No signature verification needed (dev JWT is unsigned, real OAuth JWT verification is future work).

5. **`internal/mcp/proxy.go`** — In `applyUserHeaders`, also check context for UserContext and set X-Vire-User-ID and X-Vire-Navexa-Key. Modify `get`, `doJSON`, `del` to accept and pass through context with user info. The context already flows through — just add to `applyUserHeaders`:
   ```go
   func (p *MCPProxy) applyUserHeaders(req *http.Request) {
       for key, vals := range p.userHeaders {
           for _, v := range vals {
               req.Header.Set(key, v)
           }
       }
       // Per-request user context headers
       if uc, ok := GetUserContext(req.Context()); ok {
           if uc.UserID != "" {
               req.Header.Set("X-Vire-User-ID", uc.UserID)
           }
           if uc.NavexaKey != "" {
               req.Header.Set("X-Vire-Navexa-Key", uc.NavexaKey)
           }
       }
   }
   ```
   But wait — `applyUserHeaders` currently takes no context. The proxy methods (`get`, `doJSON`, `del`) already pass context to `http.NewRequestWithContext`. The request created with that context has it available via `req.Context()`. So `applyUserHeaders` can read `req.Context()` — but it currently receives `*http.Request` indirectly. Let me restructure: change `applyUserHeaders` to accept `*http.Request` (it already does implicitly since it sets headers on req). Actually, looking at the code:

   ```go
   func (p *MCPProxy) applyUserHeaders(req *http.Request) {
   ```

   It receives the full `*http.Request` so we can access `req.Context()`. Just add the context check.

6. **`internal/app/app.go`** — Pass a user lookup closure to `mcp.NewHandler`:
   ```go
   userLookup := func(userID string) (*mcp.UserContext, error) {
       var user models.User
       err := a.StorageManager.DB().FindOne(&user, badgerhold.Where("Username").Eq(userID))
       if err != nil {
           return nil, err
       }
       return &mcp.UserContext{UserID: user.Username, NavexaKey: user.NavexaKey}, nil
   }
   a.MCPHandler = mcp.NewHandler(a.Config, a.Logger, userLookup)
   ```

### Part 2: Top navigation menu

**`pages/partials/nav.html`** — Full rewrite:
- Left: VIRE brand link
- Center: DASHBOARD link
- Right: burger icon (three lines using CSS `<span>` elements), Alpine.js `x-data="{ open: false }"` toggle for dropdown
- Dropdown items: SETTINGS (`<a href="/settings">`), LOGOUT (`<form method="POST" action="/api/auth/logout">`)
- 80s terminal aesthetic: black borders, IBM Plex Mono, no border-radius

**`pages/static/css/portal.css`** — Add nav styles:
- `.nav-inner`: flexbox, justify-content: space-between, align-items: center
- `.nav-menu`: centered links
- `.nav-burger`: right-aligned, cursor pointer
- `.nav-burger-icon span`: three horizontal lines (display block, width 18px, height 2px, background #000, margin 4px 0)
- `.nav-dropdown`: absolute positioned, border 2px solid #000, background #fff
- `.nav-dropdown a, .nav-dropdown button`: block, full width, text-align left
- Mobile: responsive adjustments

### Part 3: Session-aware menu visibility

**All handlers** — Check for `vire_session` cookie and pass `LoggedIn` to template data:

1. **`internal/handlers/landing.go`** — In `ServePage`, check cookie:
   ```go
   _, err := r.Cookie("vire_session")
   data["LoggedIn"] = err == nil
   ```

2. **`internal/handlers/dashboard.go`** — Same pattern in `ServeHTTP`

3. **`pages/landing.html`** — Add conditional nav:
   ```html
   {{if .LoggedIn}}{{template "nav.html" .}}{{end}}
   ```

4. **`pages/dashboard.html`** — Already includes nav.html, keep it (dashboard is a post-login page)

5. **`internal/handlers/auth.go`** — Add `HandleLogout`:
   ```go
   func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
       http.SetCookie(w, &http.Cookie{
           Name:     "vire_session",
           Value:    "",
           Path:     "/",
           MaxAge:   -1,
           HttpOnly: true,
       })
       http.Redirect(w, r, "/", http.StatusFound)
   }
   ```

6. **`internal/server/routes.go`** — Add logout route:
   ```go
   mux.HandleFunc("POST /api/auth/logout", s.app.AuthHandler.HandleLogout)
   ```

7. **Settings page** — Add placeholder:
   - `pages/settings.html` — minimal page with nav and "SETTINGS — COMING SOON"
   - Route: `GET /settings` in routes.go
   - Can reuse PageHandler.ServePage pattern

### Part 4: Client-side console logging

The issue: `VIRE_DEBUG` defaults to `false` in `common.js`. The `window.VIRE_CLIENT_DEBUG` variable is never set from the server, so debug logs never appear.

**Fix:**

**`pages/partials/head.html`** — Add inline script BEFORE common.js:
```html
{{if .DevMode}}<script>window.VIRE_CLIENT_DEBUG = true;</script>{{end}}
<script defer src="/static/common.js"></script>
```

The inline script (no `defer`) executes immediately during parsing. When `common.js` loads (deferred), it reads `VIRE_CLIENT_DEBUG` and sets `VIRE_DEBUG = true`. This means dev mode automatically shows debug logs in the browser console.

## Files Expected to Change

| File | Change |
|------|--------|
| `internal/models/user.go` | Add NavexaKey field |
| `data/users.json` | Add navexa_key field to seed data |
| `internal/mcp/context.go` | New: context key and helpers for user context |
| `internal/mcp/handler.go` | Add user lookup fn, extract session in ServeHTTP |
| `internal/mcp/proxy.go` | Inject per-request user headers from context |
| `internal/app/app.go` | Pass user lookup closure to MCP handler |
| `pages/partials/nav.html` | Full rewrite: centered menu, burger dropdown |
| `pages/static/css/portal.css` | Add nav menu and burger styles |
| `internal/handlers/landing.go` | Check cookie, pass LoggedIn to template |
| `internal/handlers/dashboard.go` | Check cookie, pass LoggedIn to template |
| `internal/handlers/auth.go` | Add HandleLogout handler |
| `internal/server/routes.go` | Add logout and settings routes |
| `pages/landing.html` | Conditional nav include |
| `pages/dashboard.html` | Keep nav (already included) |
| `pages/settings.html` | New: placeholder settings page |
| `pages/partials/head.html` | Add inline DevMode debug script |
| `internal/mcp/mcp_test.go` | Update tests for new handler signature |
| `internal/handlers/handlers_test.go` | Add logout test, LoggedIn checks |
| `internal/server/routes_test.go` | Add new routes |

## Test Strategy
- MCP proxy: test X-Vire-User-ID and X-Vire-Navexa-Key forwarded when user context present
- MCP handler: test JWT extraction and user lookup
- Auth: test logout clears cookie and redirects
- Nav: test LoggedIn flag in handler template data
- Console logging: verify inline script rendered in dev mode, absent in prod
