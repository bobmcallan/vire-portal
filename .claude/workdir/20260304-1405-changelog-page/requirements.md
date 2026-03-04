# Requirements: Changelog Page

## 1. Scope

**What it does:**
- Adds a new `/changelog` page displaying vire-server changelog entries
- Menu item in hamburger dropdown and mobile menu ONLY (not desktop nav bar)
- Infinite scroll: 10 items per page, loads next page when sentinel enters viewport
- Each entry shows: date, version badge, service badge, heading, body text
- Content parsed client-side: first line is heading (strip `##`), rest is body

**What it does NOT do:**
- No desktop nav bar link
- No dedicated Go handler — uses existing `PageHandler.ServePage()`
- No new Go packages or npm dependencies
- No markdown-to-HTML rendering — uses `white-space: pre-wrap` CSS

---

## 2. File Changes

### 2.1 NEW: `pages/changelog.html`

Full page template with inline Alpine.js component:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    {{template "head.html" .}}
    <title>VIRE CHANGELOG</title>
</head>
<body>
    {{if .LoggedIn}}{{template "nav.html" .}}{{end}}
    <main class="page" x-data="changelogPage()" x-init="init()">
        <div class="page-body">

            <template x-if="loading && entries.length === 0">
                <div class="help-loading">Loading changelog...</div>
            </template>

            <template x-if="error && entries.length === 0">
                <div class="warning-banner" x-text="error"></div>
            </template>

            <template x-if="entries.length > 0">
                <div>
                    <template x-for="entry in entries" :key="entry.id">
                        <section class="panel-headed changelog-entry" style="margin-bottom:1.5rem">
                            <div class="panel-header">
                                <span x-text="formatDate(entry.created_at)"></span>
                            </div>
                            <div class="panel-content">
                                <div class="changelog-meta">
                                    <span class="badge" x-text="entry.service"></span>
                                    <span class="badge badge-muted" x-text="entry.service_version || ''"></span>
                                </div>
                                <h3 class="changelog-heading" x-text="parseHeading(entry.content)"></h3>
                                <div class="changelog-body" x-text="parseBody(entry.content)"></div>
                            </div>
                        </section>
                    </template>

                    <div x-ref="sentinel" style="height:1px"></div>

                    <template x-if="loading && entries.length > 0">
                        <div class="help-loading" style="margin:1rem 0">Loading more...</div>
                    </template>

                    <template x-if="!hasMore && entries.length > 0">
                        <p class="text-muted text-sm" style="text-align:center;margin:1rem 0">End of changelog.</p>
                    </template>
                </div>
            </template>

            <template x-if="!loading && !error && entries.length === 0">
                <p class="text-muted text-sm">No changelog entries.</p>
            </template>

        </div>
    </main>
    {{template "footer.html" .}}

    <script>
    function changelogPage() {
        return {
            entries: [],
            loading: true,
            error: null,
            currentPage: 0,
            totalPages: 1,
            pageSize: 10,
            hasMore: true,
            observer: null,

            init() {
                this.loadNextPage();
                this.$nextTick(() => { this.setupObserver(); });
            },

            setupObserver() {
                const sentinel = this.$refs.sentinel;
                if (!sentinel) {
                    setTimeout(() => this.setupObserver(), 200);
                    return;
                }
                this.observer = new IntersectionObserver((items) => {
                    if (items[0].isIntersecting && !this.loading && this.hasMore) {
                        this.loadNextPage();
                    }
                }, { rootMargin: '200px' });
                this.observer.observe(sentinel);
            },

            loadNextPage() {
                if (this.loading && this.currentPage > 0) return;
                this.loading = true;
                const nextPage = this.currentPage + 1;
                fetch('/api/changelog?per_page=' + this.pageSize + '&page=' + nextPage)
                    .then(r => {
                        if (!r.ok) throw new Error('Failed to load changelog');
                        return r.json();
                    })
                    .then(data => {
                        const newItems = data.items || [];
                        this.entries = this.entries.concat(newItems);
                        this.currentPage = data.page || nextPage;
                        this.totalPages = data.pages || 1;
                        this.hasMore = this.currentPage < this.totalPages;
                        this.loading = false;
                        if (this.hasMore && !this.observer) {
                            this.$nextTick(() => this.setupObserver());
                        }
                    })
                    .catch(err => {
                        this.error = err.message;
                        this.loading = false;
                    });
            },

            formatDate(iso) {
                if (!iso) return '';
                const d = new Date(iso);
                return d.toLocaleDateString('en-AU', { day: '2-digit', month: 'short', year: 'numeric' });
            },

            parseHeading(content) {
                if (!content) return '';
                const firstLine = content.split('\n')[0] || '';
                return firstLine.replace(/^#+\s*/, '').trim();
            },

            parseBody(content) {
                if (!content) return '';
                const lines = content.split('\n');
                let start = 1;
                while (start < lines.length && lines[start].trim() === '') { start++; }
                return lines.slice(start).join('\n').trim();
            },
        };
    }
    </script>
</body>
</html>
```

### 2.2 MODIFY: `pages/partials/nav.html`

**Hamburger dropdown** — add after `<a href="/profile">Profile</a>`, before the admin conditional:
```html
<a href="/changelog">Changelog</a>
```

**Mobile menu** — add after `<a href="/help">Help</a>`:
```html
<a href="/changelog">Changelog</a>
```

Do NOT add to `.nav-links` (desktop bar).

### 2.3 MODIFY: `internal/server/routes.go`

Add one line after the `/help` route:
```go
mux.HandleFunc("GET /changelog", s.app.PageHandler.ServePage("changelog.html", "changelog"))
```

### 2.4 MODIFY: `pages/static/css/portal.css`

Add after existing badge/help styles section:
```css
/* === CHANGELOG === */

.changelog-meta {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
}

.changelog-heading {
    font-size: 0.9375rem;
    font-weight: 700;
    margin-bottom: 0.75rem;
    line-height: 1.4;
}

.changelog-body {
    white-space: pre-wrap;
    font-size: 0.8125rem;
    line-height: 1.7;
    color: #000;
}
```

---

## 3. Function Signatures

No new Go functions needed. Uses:
- `PageHandler.ServePage("changelog.html", "changelog")` — already exists in `internal/handlers/landing.go`
- `/api/changelog` proxy — already handled by catch-all `/api/` route in `routes.go`

---

## 4. Content Parsing

The `content` field is markdown. First line is `## Heading text`, followed by body.

- **parseHeading(content)**: split on `\n`, take line 0, strip `/^#+\s*/`, trim
- **parseBody(content)**: split on `\n`, skip line 0 + leading blank lines, join remainder, trim
- Render body with `white-space: pre-wrap` CSS — no markdown library needed

---

## 5. Infinite Scroll

- Sentinel `<div x-ref="sentinel" style="height:1px">` after last entry
- `IntersectionObserver` with `rootMargin: '200px'` triggers `loadNextPage()`
- Guard: `if (this.loading && this.currentPage > 0) return` prevents duplicate requests
- `hasMore = currentPage < totalPages` — set after each API response
- `setupObserver()` retries with `setTimeout(200ms)` if sentinel not yet in DOM
- End state: show "End of changelog." when `!hasMore && entries.length > 0`

---

## 6. Unit Tests

### `internal/server/routes_test.go` — Add:

```go
func TestRoutes_ChangelogPage(t *testing.T) {
    application := newTestApp(t)
    srv := New(application)

    testToken := createTestJWT("test-user-123", application.Config.Auth.JWTSecret)

    req := httptest.NewRequest("GET", "/changelog", nil)
    req.AddCookie(&http.Cookie{Name: "vire_session", Value: testToken})
    w := httptest.NewRecorder()

    srv.Handler().ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("expected status 200, got %d", w.Code)
    }

    body := w.Body.String()
    if !strings.Contains(body, "changelogPage") {
        t.Error("expected changelog page to contain changelogPage Alpine component")
    }
}
```

Follow exact pattern of `TestRoutes_DashboardPage` in same file.

---

## 7. UI Tests

### NEW: `tests/ui/changelog_test.go`

Package: `tests` (same as other UI test files)
Imports: `"testing"`, `commontest "github.com/bobmcallan/vire-portal/tests/common"`, `"github.com/chromedp/chromedp"`, `"strings"` (if needed)

**Tests:**

1. `TestChangelogPageLoads` — navigate to `/changelog`, verify `.page` visible, screenshot `changelog/page-loads.png`
2. `TestChangelogPageNavVisible` — verify `.nav` is visible on page, screenshot `changelog/nav-visible.png`
3. `TestChangelogPageNoJSErrors` — collect JS errors with `newJSErrorCollector`, verify no errors, screenshot `changelog/no-js-errors.png`
4. `TestChangelogPageNoTemplateMarkers` — get body text, check for `{{.`, `<no value>`, `{{template`, `{{if`, `{{range` markers, screenshot `changelog/no-template-markers.png`
5. `TestChangelogPageContent` — wait 2s after load, verify either `.changelog-entry` exists OR "No changelog entries" text present (both valid), screenshot `changelog/content.png`
6. `TestChangelogInHamburgerDropdown` — navigate to `/dashboard`, click `.nav-hamburger`, wait 300ms, verify `a[href='/changelog']` in dropdown, screenshot `changelog/hamburger-link.png`
7. `TestChangelogInMobileMenu` — navigate to `/dashboard` at 375px width, open mobile menu, verify `a[href='/changelog']` present, screenshot `changelog/mobile-link.png`

Follow patterns from `docs_test.go` and `nav_test.go` exactly.

---

## 8. Edge Cases

| Case | Handling |
|------|----------|
| Empty list (0 entries) | Show "No changelog entries." via `x-if="!loading && !error && entries.length === 0"` |
| Single entry, no more pages | `hasMore = false`, observer fires but guard prevents extra fetch |
| API error | Show `warning-banner` with error message |
| Content with no heading marker | `parseHeading` returns raw first line (strips nothing) |
| Content body only | `parseBody` returns empty string cleanly |
| Very long content | `pre-wrap` wraps, no max-height constraint |
| Missing `service_version` | `entry.service_version \|\| ''` renders empty badge |
| Rapid scroll triggering observer | Loading guard prevents concurrent fetches |
| Unauthenticated visit | `PageHandler.ServePage` renders without nav (LoggedIn=false) |

---

## 9. Dependencies

None new. All existing:
- Go: `PageHandler.ServePage` (internal/handlers/landing.go)
- JS: native `fetch()`, native `IntersectionObserver`, Alpine.js (already in head.html)
- CSS: existing `.panel-headed`, `.badge`, `.badge-muted`, `.help-loading`, `.warning-banner`, `.text-muted`
- API: `/api/changelog` — proxied to vire-server via existing catch-all proxy route

---

## Summary

| File | Action |
|------|--------|
| `pages/changelog.html` | CREATE — full template with infinite scroll |
| `pages/partials/nav.html` | MODIFY — add Changelog to hamburger + mobile menu |
| `internal/server/routes.go` | MODIFY — add GET /changelog route |
| `pages/static/css/portal.css` | MODIFY — add .changelog-meta/heading/body styles |
| `internal/server/routes_test.go` | MODIFY — add TestRoutes_ChangelogPage |
| `tests/ui/changelog_test.go` | CREATE — 7 UI tests |
