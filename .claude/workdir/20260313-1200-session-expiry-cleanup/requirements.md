# Requirements: Session Expiry Timer + Client-Side Cleanup

## Feature 1: Session Expiry Timer

### Scope
Add a client-side timer that automatically logs the user out and redirects to the login page when their JWT token expires. This prevents users from sitting on stale sessions with silent 401 failures.

### Approach
1. Server embeds `TokenExpiry` (unix timestamp from JWT `exp` claim) in template data
2. `head.html` partial conditionally renders a script setting `window.__VIRE_TOKEN_EXPIRY__`
3. `common.js` adds a `sessionExpiry()` Alpine component that:
   - Reads `window.__VIRE_TOKEN_EXPIRY__`
   - Calculates time remaining
   - Sets `setTimeout` to POST `/api/auth/logout` and redirect to `/` when expired
   - Shows a warning banner 2 minutes before expiry

### File Changes

#### `pages/partials/head.html`
Add conditional script after the DevMode debug line (line 10):
```html
{{if .TokenExpiry}}<script>window.__VIRE_TOKEN_EXPIRY__ = {{.TokenExpiry}};</script>{{end}}
```

#### `pages/static/common.js`
Add `sessionExpiry()` Alpine component (near the top, after the `debugLog`/`debugError` helpers and CSRF injection, before `alpine:init`):
```javascript
function sessionExpiry() {
    return {
        warning: false,
        timeLeft: '',
        _timer: null,
        _warningTimer: null,
        init() {
            const exp = window.__VIRE_TOKEN_EXPIRY__;
            if (!exp) return;
            const nowSec = Math.floor(Date.now() / 1000);
            const remainSec = exp - nowSec;
            if (remainSec <= 0) {
                this._doLogout();
                return;
            }
            // Warning 2 minutes before expiry
            const warnSec = remainSec - 120;
            if (warnSec > 0) {
                this._warningTimer = setTimeout(() => {
                    this.warning = true;
                    this._updateTimeLeft(120);
                }, warnSec * 1000);
            } else {
                this.warning = true;
                this._updateTimeLeft(remainSec);
            }
            // Logout at expiry
            this._timer = setTimeout(() => this._doLogout(), remainSec * 1000);
        },
        _updateTimeLeft(sec) {
            const update = () => {
                const exp = window.__VIRE_TOKEN_EXPIRY__;
                const now = Math.floor(Date.now() / 1000);
                const left = exp - now;
                if (left <= 0) return;
                const m = Math.floor(left / 60);
                const s = left % 60;
                this.timeLeft = m > 0 ? m + 'm ' + s + 's' : s + 's';
            };
            update();
            setInterval(update, 1000);
        },
        _doLogout() {
            // POST to logout endpoint to clear server session, then redirect
            fetch('/api/auth/logout', { method: 'POST' })
                .finally(() => { window.location.href = '/'; });
        },
        destroy() {
            if (this._timer) clearTimeout(this._timer);
            if (this._warningTimer) clearTimeout(this._warningTimer);
        }
    };
}
```

#### `pages/partials/nav.html`
Add session expiry warning banner inside the nav component. Add `x-data="sessionExpiry()"` as a sibling div after the nav, with a warning banner:
```html
<div x-data="sessionExpiry()" x-show="warning" x-cloak class="session-expiry-banner">
    Session expires in <span x-text="timeLeft"></span> — <a href="/" class="session-expiry-link">login again</a>
</div>
```

#### `pages/static/css/portal.css`
Add styles for `.session-expiry-banner`:
```css
.session-expiry-banner {
    background: #ff6b35;
    color: #000;
    text-align: center;
    padding: 0.3rem 1rem;
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
}
.session-expiry-link {
    color: #000;
    text-decoration: underline;
}
```

#### Authenticated handlers — add `TokenExpiry` to template data
Every handler that calls `IsLoggedIn()` and passes data to a template needs `"TokenExpiry"` added.

Files to modify (add `"TokenExpiry": claims.Exp` to the template data map, with nil check `if claims != nil`):

1. `internal/handlers/dashboard.go` — `ServeHTTP` method, data map at line ~240
2. `internal/handlers/mobile_dashboard.go` — `ServeHTTP`, data map
3. `internal/handlers/stock.go` — `ServeHTTP`, data map
4. `internal/handlers/strategy.go` — `ServeHTTP`, data map
5. `internal/handlers/cash.go` — `ServeHTTP`, data map
6. `internal/handlers/profile.go` — `ServeHTTP` and `HandlePost`, data maps
7. `internal/handlers/users.go` — `ServeHTTP`, data map
8. `internal/handlers/prompts.go` — `ServeHTTP`, data map
9. `internal/handlers/gemini.go` — `ServeHTTP`, data map
10. `internal/handlers/mcp_page.go` — `ServeHTTP`, data map
11. `internal/handlers/landing.go` — all 5 SSR handler methods that check IsLoggedIn and render with data

Pattern for each handler:
```go
// In the template data map:
var tokenExpiry int64
if claims != nil {
    tokenExpiry = claims.Exp
}
// ... then in the data map:
"TokenExpiry": tokenExpiry,
```

### Unit Tests
- `TestValidateJWT_ExpiryInTemplateData` — verify that handler passes Exp to template
- `TestSessionExpiry_HeadPartialRendersScript` — verify head.html conditional renders correctly

---

## Feature 2: Remove Client-Side Calculation Pass-Throughs (fb_24b3388d)

### Scope
Remove the 4 remaining client-side helper functions that are trivial pass-throughs to server values. The other 4 (adjPct, adjDollar, computeBreadth, breadthSegments) were already removed.

### File Changes

#### `pages/static/common.js`
Remove these 4 functions from the `portfolioDashboard()` return object:

1. **Remove `holdingTodayChange(h)`** (line ~1159) — callers use `h.yesterday_price_change` directly
2. **Remove `holdingBreadthClass(h)`** (line ~1170) — callers use `'breadth-' + (h.breadth_status || 'flat')` directly
3. **Remove `holdingBreadthArrow(h)`** (line ~1173) — callers use inline ternary
4. **Remove `holdingDailyPct(h)`** (line ~1179) — not used in any template, dead code

#### `pages/dashboard.html`
Update template references (3 locations):

Line ~255: Replace `holdingBreadthClass(h)` and `holdingBreadthArrow(h)`:
```html
<!-- Before -->
<span class="holding-breadth-arrow" :class="holdingBreadthClass(h)" x-text="holdingBreadthArrow(h)"></span>
<!-- After -->
<span class="holding-breadth-arrow" :class="'breadth-' + (h.breadth_status || 'flat')" x-text="h.breadth_status === 'rising' ? '\u25B2' : h.breadth_status === 'falling' ? '\u25BC' : '\u25C6'"></span>
```

Line ~274: Same replacement pattern.

Line ~278: Replace `holdingTodayChange(h)`:
```html
<!-- Before -->
<span :class="changeClass(holdingTodayChange(h))" x-text="fmtTodayChange(holdingTodayChange(h))"></span>
<!-- After -->
<span :class="changeClass(h.yesterday_price_change)" x-text="fmtTodayChange(h.yesterday_price_change)"></span>
```

#### `pages/mobile.html`
Update template reference (1 location):

Line ~144: Replace `holdingBreadthClass(h)` and `holdingBreadthArrow(h)`:
```html
<!-- Before -->
<span class="holding-breadth-arrow" :class="holdingBreadthClass(h)" x-text="holdingBreadthArrow(h)"></span>
<!-- After -->
<span class="holding-breadth-arrow" :class="'breadth-' + (h.breadth_status || 'flat')" x-text="h.breadth_status === 'rising' ? '\u25B2' : h.breadth_status === 'falling' ? '\u25BC' : '\u25C6'"></span>
```

### Unit Tests
- Existing unit/stress tests should still pass after template references are updated
- No new unit tests needed — this is a pure removal of dead code

---

## Edge Cases
- Token expiry in the past on page load → immediate logout
- No token expiry (unauthenticated pages) → no timer, no banner
- Tab in background → setTimeout still fires (browser may throttle but will execute)
- Multiple tabs → each tab has its own timer, all logout independently
- User refreshes page → timer resets from server-provided expiry (fine, token hasn't changed)

## Dependencies
- None — all changes are portal-side only
