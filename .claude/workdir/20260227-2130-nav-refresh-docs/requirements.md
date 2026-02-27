# Nav Title, Refresh Button, Docs Page

## Feature 1: VIRE title links to dashboard when logged in

**File:** `pages/partials/nav.html` line 5

The nav partial is only rendered when the user is logged in (`{{if .LoggedIn}}{{template "nav.html" .}}{{end}}`), so the VIRE brand link should go to `/dashboard` instead of `/` (which clears the session).

**Change:**
```html
<!-- Before -->
<a href="/" class="nav-brand">VIRE</a>
<!-- After -->
<a href="/dashboard" class="nav-brand">VIRE</a>
```

## Feature 2: Move dashboard refresh button to the right

**File:** `pages/dashboard.html` lines 40-43, `pages/static/css/portal.css`

The refresh button is inside `.portfolio-header` (a flex container). Currently it sits after the Default checkbox with `gap: 1rem`. Push it to the right using `margin-left: auto`.

**Change in dashboard.html** (line 40):
```html
<!-- Add style to push button right -->
<button class="btn btn-secondary btn-sm" style="margin-left:auto" @click="refreshPortfolio()" :disabled="refreshing">
```

Or add a CSS rule in portal.css:
```css
.portfolio-header .btn {
    margin-left: auto;
}
```

## Feature 3: Add Docs page with Navexa instructions

### New file: `pages/docs.html`

A static content page using the existing `PageHandler.ServePage()` pattern (no new handler needed). Content:
- What Vire is and what it does
- What Navexa is, why you need an API key
- Step-by-step instructions to get a Navexa API key and add it in Settings
- Link to [navexa.com](https://www.navexa.com)
- Follow the 80s B&W monochrome design (IBM Plex Mono, no border-radius, `.page`, `.page-body`, `.dashboard-section`, `.section-title` classes)

### Route registration: `internal/server/routes.go` line ~38

```go
mux.HandleFunc("GET /docs", s.app.PageHandler.ServePage("docs.html", "docs"))
```

### Nav item: `pages/partials/nav.html`

Desktop nav (after MCP line 11):
```html
<li><a href="/docs" {{if eq .Page "docs"}}class="active"{{end}}>Docs</a></li>
```

Mobile nav (after MCP line 40):
```html
<a href="/docs">Docs</a>
```

Hamburger dropdown (after Settings line 23):
```html
<a href="/docs">Docs</a>
```

## Files Changed

| File | Change |
|------|--------|
| `pages/partials/nav.html` | VIRE → /dashboard, add Docs nav item (desktop + mobile + dropdown) |
| `pages/dashboard.html` | Refresh button margin-left:auto |
| `pages/static/css/portal.css` | Optional: CSS rule for refresh button positioning |
| `pages/docs.html` | NEW — Docs page with Navexa instructions |
| `internal/server/routes.go` | Add `GET /docs` route |
| `README.md` | Add docs route to routes table |
