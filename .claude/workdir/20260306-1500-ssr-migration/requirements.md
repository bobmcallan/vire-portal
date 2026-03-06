# SSR Migration: All Pages Except Dashboard

## 1. Scope and Approach

**Goal:** Migrate 7 pages from client-side Alpine.js data fetching to server-side rendering. After migration, pages render with data populated in Go templates — no "Loading..." spinners on initial load. Interactive features (save, toggle, pagination, filter, submit) remain client-side via Alpine.js.

**Pages in scope (ordered by complexity):**

1. **Error** — trivial: read query param server-side
2. **Landing** — simple: server-side health check
3. **Glossary** — moderate: fetch glossary, render via Go templates, keep client-side filter
4. **Changelog** — moderate: fetch changelog, JSON hydration, keep client-side pagination
5. **Help** — moderate: fetch feedback list, JSON hydration, keep interactive submit/copy
6. **Strategy** — complex: fetch portfolios + strategy + plan, hybrid SSR + Alpine for saves
7. **Cash** — complex: fetch portfolios + transactions + accounts, hybrid SSR + Alpine for toggle/pagination

**Architecture approach:**

- Add a generic `ProxyGet` method to `VireClient` for authenticated GET requests to vire-server
- For **error, landing**: Server renders directly, minimal/no Alpine
- For **glossary**: Pure Go template rendering with `{{range}}`, Alpine filters by hiding/showing DOM elements
- For **changelog, help**: JSON hydration via `window.__VIRE_DATA__`, Alpine reads on init instead of fetching
- For **strategy, cash**: JSON hydration, Alpine handles saves/toggles/portfolio-switching

**Key constraint:** Interactive features stay client-side. SSR only handles initial page render.

---

## 2. New VireClient Method

**File:** `internal/client/vire_client.go`

```go
// ProxyGet performs a GET request to vire-server at the given path,
// injecting the X-Vire-User-ID header for authentication.
// Returns the raw response body bytes on success (2xx), or an error.
func (c *VireClient) ProxyGet(path string, userID string) ([]byte, error)
```

**Implementation:**
- Build URL: `c.baseURL + path`
- Create `http.NewRequest("GET", url, nil)`
- Set header `X-Vire-User-ID` = userID (if non-empty)
- Use `c.httpClient` (10s timeout)
- Read body via `io.ReadAll(io.LimitReader(resp.Body, 1<<20))`
- Return error if status not 2xx
- Return body bytes on success

---

## 3. File-by-File Changes

### 3.1 Error Page

**`internal/handlers/landing.go`** — add method to PageHandler:

```go
func (h *PageHandler) ServeErrorPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
        var userRole string
        if loggedIn && h.userLookupFn != nil && claims != nil && claims.Sub != "" {
            if user, err := h.userLookupFn(claims.Sub); err == nil && user != nil {
                userRole = user.Role
            }
        }

        reason := r.URL.Query().Get("reason")
        messages := map[string]string{
            "server_unavailable":  "The authentication server is unavailable. Please try again shortly.",
            "auth_failed":         "Authentication failed. Please try again.",
            "invalid_credentials": "Invalid username or password.",
            "missing_credentials": "Please provide both username and password.",
            "bad_request":         "Bad request. Please try again.",
        }
        msg := messages[reason]
        if msg == "" {
            msg = "Something went wrong. Please try again."
        }

        data := map[string]interface{}{
            "Page":          "error",
            "DevMode":       h.devMode,
            "LoggedIn":      loggedIn,
            "UserRole":      userRole,
            "PortalVersion": config.GetVersion(),
            "ServerVersion": GetServerVersion(h.apiURL),
            "ErrorMessage":  msg,
        }
        h.templates.ExecuteTemplate(w, "error.html", data)
    }
}
```

**`pages/error.html`** — replace Alpine with Go template:
- Replace `x-data x-text="errorMessage()"` with plain text `{{.ErrorMessage}}`
- Remove entire `<script>` block

**`internal/server/routes.go`** — change line 42:
```go
mux.HandleFunc("GET /error", s.app.PageHandler.ServeErrorPage())
```

### 3.2 Landing Page

**`internal/handlers/landing.go`** — add method:

```go
func (h *PageHandler) ServeLandingPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Auto-logout: clear session cookie
        http.SetCookie(w, &http.Cookie{
            Name: "vire_session", Value: "", Path: "/",
            MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode,
        })

        serverUp := checkServerHealth(h.apiURL)

        data := map[string]interface{}{
            "Page":          "home",
            "DevMode":       h.devMode,
            "LoggedIn":      false,
            "UserRole":      "",
            "PortalVersion": config.GetVersion(),
            "ServerVersion": GetServerVersion(h.apiURL),
            "ServerStatus":  serverUp,
        }
        h.templates.ExecuteTemplate(w, "landing.html", data)
    }
}

func checkServerHealth(apiURL string) bool {
    if apiURL == "" {
        return false
    }
    client := &http.Client{Timeout: 3 * time.Second}
    resp, err := client.Get(apiURL + "/api/health")
    if err != nil {
        return false
    }
    defer resp.Body.Close()
    return resp.StatusCode == http.StatusOK
}
```

**`pages/landing.html`** changes:
- Change `x-data="serverCheck()" x-init="check()"` to just `x-data="serverCheck()"`
- Modify `<script>` — initial status from Go template, keep `check()` for retry:

```html
<script>
function serverCheck() {
    return {
        status: '{{if .ServerStatus}}ok{{else}}down{{end}}',
        check() {
            this.status = 'checking';
            fetch('/api/server-health')
                .then(r => r.json())
                .then(d => { this.status = d.status === 'ok' ? 'ok' : 'down'; })
                .catch(() => { this.status = 'down'; });
        }
    };
}
</script>
```

**`internal/server/routes.go`** — change line 43:
```go
mux.HandleFunc("/", s.app.PageHandler.ServeLandingPage())
```

### 3.3 Glossary Page

**`internal/handlers/landing.go`** — add structs and method:

```go
type GlossaryTerm struct {
    Term       string `json:"term"`
    Label      string `json:"label"`
    Definition string `json:"definition"`
    Formula    string `json:"formula"`
}

type GlossaryCategory struct {
    Name  string         `json:"name"`
    Terms []GlossaryTerm `json:"terms"`
}

func (h *PageHandler) ServeGlossaryPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
        var userRole string
        if loggedIn && h.userLookupFn != nil && claims != nil && claims.Sub != "" {
            if user, err := h.userLookupFn(claims.Sub); err == nil && user != nil {
                userRole = user.Role
            }
        }

        var categories []GlossaryCategory
        var fetchError string
        if h.proxyGetFn != nil {
            body, err := h.proxyGetFn("/api/glossary", "")
            if err != nil {
                fetchError = "Glossary data is not yet available."
            } else {
                var resp struct {
                    Categories []GlossaryCategory `json:"categories"`
                }
                if json.Unmarshal(body, &resp) == nil {
                    categories = resp.Categories
                }
            }
        }

        data := map[string]interface{}{
            "Page":          "glossary",
            "DevMode":       h.devMode,
            "LoggedIn":      loggedIn,
            "UserRole":      userRole,
            "PortalVersion": config.GetVersion(),
            "ServerVersion": GetServerVersion(h.apiURL),
            "Categories":    categories,
            "FetchError":    fetchError,
            "TermParam":     r.URL.Query().Get("term"),
        }
        h.templates.ExecuteTemplate(w, "glossary.html", data)
    }
}
```

**`pages/glossary.html`** — full rewrite to SSR with client-side filter:

```html
<main class="page" x-data="glossaryFilter()">
    <div class="page-body">
        <div class="help-search-wrap" style="margin-bottom:1.5rem">
            <input type="text" class="form-input help-search"
                   placeholder="Search terms, definitions, formulas..."
                   x-model="query" @input="filter()">
        </div>

        {{if .FetchError}}
        <div class="warning-banner">{{.FetchError}}</div>
        {{end}}

        {{if not .FetchError}}
        {{range .Categories}}
        <section class="panel-headed glossary-category" style="margin-bottom:1.5rem">
            <div class="panel-header">{{.Name}}</div>
            <div class="panel-content">
                {{range .Terms}}
                <div class="help-term glossary-term-item">
                    <div class="help-term-header">
                        <span class="help-term-label">{{.Label}}</span>
                        <span class="help-term-id">{{.Term}}</span>
                    </div>
                    <p class="help-term-def">{{.Definition}}</p>
                    {{if .Formula}}
                    <p class="help-term-formula"><span class="help-term-formula-label">Formula:</span> {{.Formula}}</p>
                    {{end}}
                </div>
                {{end}}
            </div>
        </section>
        {{end}}
        {{if not .Categories}}
        <p class="text-muted text-sm">No glossary entries available.</p>
        {{end}}
        {{end}}

        <p class="text-muted no-results-msg" style="display:none">No results found.</p>
    </div>
</main>

<script>
function glossaryFilter() {
    return {
        query: '{{.TermParam}}',
        init() { if (this.query) this.filter(); },
        filter() {
            const q = this.query.toLowerCase().trim();
            let totalVisible = 0;
            document.querySelectorAll('.glossary-category').forEach(cat => {
                let visibleCount = 0;
                cat.querySelectorAll('.glossary-term-item').forEach(term => {
                    const text = term.textContent.toLowerCase();
                    const match = !q || text.includes(q);
                    term.style.display = match ? '' : 'none';
                    if (match) visibleCount++;
                });
                cat.style.display = visibleCount > 0 || !q ? '' : 'none';
                totalVisible += visibleCount;
            });
            const noResults = document.querySelector('.no-results-msg');
            if (noResults) {
                noResults.style.display = (q && totalVisible === 0) ? '' : 'none';
            }
        }
    };
}
</script>
```

**`internal/server/routes.go`** — change line 40:
```go
mux.HandleFunc("GET /glossary", s.app.PageHandler.ServeGlossaryPage())
```

### 3.4 Changelog Page

**`internal/handlers/landing.go`** — add method:

```go
func (h *PageHandler) ServeChangelogPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // (standard auth/role check)

        var entriesJSON template.JS = "[]"
        if h.proxyGetFn != nil {
            body, err := h.proxyGetFn("/api/changelog?per_page=100&page=1", "")
            if err == nil {
                var resp struct {
                    Items json.RawMessage `json:"items"`
                }
                if json.Unmarshal(body, &resp) == nil && resp.Items != nil {
                    entriesJSON = template.JS(resp.Items)
                }
            }
        }

        data["EntriesJSON"] = entriesJSON
        h.templates.ExecuteTemplate(w, "changelog.html", data)
    }
}
```

**`pages/changelog.html`** changes:
- Add before `</body>`: `<script>window.__VIRE_DATA__ = { entries: {{.EntriesJSON}} };</script>`
- Modify inline `changelogPage()` init:
```javascript
init() {
    const ssrData = window.__VIRE_DATA__;
    if (ssrData && ssrData.entries) {
        this.entries = ssrData.entries;
        this.loading = false;
        window.__VIRE_DATA__ = null;
        return;
    }
    // fallback fetch...
}
```
- Remove `<template x-if="loading">` block (or keep it, it will just never show)

### 3.5 Help Page

**`internal/handlers/landing.go`** — add method:

```go
func (h *PageHandler) ServeHelpPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
        if !loggedIn {
            http.Redirect(w, r, "/", http.StatusFound)
            return
        }
        // (role check)

        var feedbackJSON template.JS = "[]"
        var feedbackTotal int
        if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
            body, err := h.proxyGetFn("/api/feedback?per_page=50", claims.Sub)
            if err == nil {
                var resp struct {
                    Items json.RawMessage `json:"items"`
                    Total int             `json:"total"`
                }
                if json.Unmarshal(body, &resp) == nil {
                    if resp.Items != nil {
                        feedbackJSON = template.JS(resp.Items)
                    }
                    feedbackTotal = resp.Total
                }
            }
        }

        data["FeedbackJSON"] = feedbackJSON
        data["FeedbackTotal"] = feedbackTotal
        h.templates.ExecuteTemplate(w, "help.html", data)
    }
}
```

**`pages/help.html`** changes:
- Add before inline `<script>`: `<script>window.__VIRE_DATA__ = { feedbackItems: {{.FeedbackJSON}}, feedbackTotal: {{.FeedbackTotal}} };</script>`
- Modify `helpPage()` init:
```javascript
init() {
    const ssrData = window.__VIRE_DATA__;
    if (ssrData && ssrData.feedbackItems) {
        this.feedbackItems = ssrData.feedbackItems;
        this.feedbackTotal = ssrData.feedbackTotal || this.feedbackItems.length;
        this.feedbackLoading = false;
        window.__VIRE_DATA__ = null;
        return;
    }
    this.loadFeedback();
}
```

**`internal/server/routes.go`** — change:
```go
mux.HandleFunc("GET /help", s.app.PageHandler.ServeHelpPage())
```

### 3.6 Strategy Page

**`internal/handlers/strategy.go`** — add field, setter, modify ServeHTTP:

```go
type StrategyHandler struct {
    // existing fields...
    proxyGetFn func(path, userID string) ([]byte, error)
}

func (h *StrategyHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
    h.proxyGetFn = fn
}
```

In `ServeHTTP`, after auth/user checks:
```go
var portfoliosJSON, strategyJSON, planJSON template.JS
portfoliosJSON = "null"
strategyJSON = "null"
planJSON = "null"
if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
    if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
        portfoliosJSON = template.JS(body)
        var pData struct {
            Portfolios []struct{ Name string `json:"name"` } `json:"portfolios"`
            Default    string                                  `json:"default"`
        }
        if json.Unmarshal(body, &pData) == nil {
            selected := pData.Default
            if selected == "" && len(pData.Portfolios) > 0 {
                selected = pData.Portfolios[0].Name
            }
            if selected != "" {
                if sBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/strategy", claims.Sub); err == nil {
                    strategyJSON = template.JS(sBody)
                }
                if pBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/plan", claims.Sub); err == nil {
                    planJSON = template.JS(pBody)
                }
            }
        }
    }
}
data["PortfoliosJSON"] = portfoliosJSON
data["StrategyJSON"] = strategyJSON
data["PlanJSON"] = planJSON
```

**`pages/strategy.html`** — add before `</body>`:
```html
<script>
window.__VIRE_DATA__ = {
    portfolios: {{.PortfoliosJSON}},
    strategy: {{.StrategyJSON}},
    plan: {{.PlanJSON}}
};
</script>
```

**`pages/static/common.js`** — modify `portfolioStrategy()` init:
```javascript
async init() {
    try {
        const ssrData = window.__VIRE_DATA__;
        if (ssrData && ssrData.portfolios) {
            const data = ssrData.portfolios;
            this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
            this.defaultPortfolio = data.default || '';
            if (this.defaultPortfolio) {
                this.selected = this.defaultPortfolio;
            } else if (this.portfolios.length > 0) {
                this.selected = this.portfolios[0].name;
            }
            if (ssrData.strategy) {
                const sd = ssrData.strategy;
                this.strategy = sd.notes || JSON.stringify(sd.strategy || sd, null, 2);
            }
            if (ssrData.plan) {
                const pd = ssrData.plan;
                this.plan = pd.notes || JSON.stringify(pd.plan || pd, null, 2);
            }
            window.__VIRE_DATA__ = null;
            this.loading = false;
            return;
        }
        // Fallback: fetch client-side
        const res = await vireStore.fetch('/api/portfolios');
        // ... existing fetch logic unchanged
    } catch (e) {
        debugError('portfolioStrategy', 'init failed', e);
        this.error = 'Failed to connect to server';
    } finally {
        this.loading = false;
    }
}
```

### 3.7 Cash Page

Same hybrid pattern as strategy.

**`internal/handlers/cash.go`** — add field, setter, modify ServeHTTP:

```go
type CashHandler struct {
    // existing fields...
    proxyGetFn func(path, userID string) ([]byte, error)
}

func (h *CashHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
    h.proxyGetFn = fn
}
```

In `ServeHTTP`, after auth/user checks:
```go
var portfoliosJSON, transactionsJSON template.JS
portfoliosJSON = "null"
transactionsJSON = "null"
if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
    if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
        portfoliosJSON = template.JS(body)
        var pData struct {
            Portfolios []struct{ Name string `json:"name"` } `json:"portfolios"`
            Default    string                                  `json:"default"`
        }
        if json.Unmarshal(body, &pData) == nil {
            selected := pData.Default
            if selected == "" && len(pData.Portfolios) > 0 {
                selected = pData.Portfolios[0].Name
            }
            if selected != "" {
                if tBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/cash-transactions", claims.Sub); err == nil {
                    transactionsJSON = template.JS(tBody)
                }
            }
        }
    }
}
data["PortfoliosJSON"] = portfoliosJSON
data["TransactionsJSON"] = transactionsJSON
```

**`pages/cash.html`** — add before `</body>`:
```html
<script>
window.__VIRE_DATA__ = {
    portfolios: {{.PortfoliosJSON}},
    transactions: {{.TransactionsJSON}}
};
</script>
```

**`pages/static/common.js`** — modify `cashTransactions()` init:
```javascript
async init() {
    try {
        const ssrData = window.__VIRE_DATA__;
        if (ssrData && ssrData.portfolios) {
            const data = ssrData.portfolios;
            this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
            this.defaultPortfolio = data.default || '';
            if (this.defaultPortfolio) {
                this.selected = this.defaultPortfolio;
            } else if (this.portfolios.length > 0) {
                this.selected = this.portfolios[0].name;
            }
            if (ssrData.transactions) {
                const td = ssrData.transactions;
                const txns = td.transactions || [];
                txns.sort((a, b) => new Date(b.date) - new Date(a.date));
                this.transactions = txns;
                this.accounts = td.accounts || [];
                const summary = td.summary || {};
                this.totalCash = summary.capital_gross || 0;
                this.transactionCount = summary.transaction_count || 0;
                this.byCategory = summary.net_cash_by_category || {};
            }
            window.__VIRE_DATA__ = null;
            this.loading = false;
            return;
        }
        // Fallback: fetch client-side
        const res = await vireStore.fetch('/api/portfolios');
        // ... existing fetch logic unchanged
    } catch (e) {
        debugError('cashTransactions', 'init failed', e);
        this.error = 'Failed to connect to server';
    } finally {
        this.loading = false;
    }
}
```

### 3.8 App & Route Wiring

**`internal/app/app.go`** — add proxyGet wiring after vireClient creation:

```go
a.PageHandler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
    return vireClient.ProxyGet(path, userID)
})
a.StrategyHandler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
    return vireClient.ProxyGet(path, userID)
})
a.CashHandler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
    return vireClient.ProxyGet(path, userID)
})
```

**`internal/handlers/landing.go`** — add `proxyGetFn` field to `PageHandler`:

```go
type PageHandler struct {
    // existing fields...
    proxyGetFn func(path, userID string) ([]byte, error)
}

func (h *PageHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error)) {
    h.proxyGetFn = fn
}
```

**`internal/server/routes.go`** — update routes:
```go
mux.HandleFunc("GET /help", s.app.PageHandler.ServeHelpPage())
mux.HandleFunc("GET /changelog", s.app.PageHandler.ServeChangelogPage())
mux.HandleFunc("GET /glossary", s.app.PageHandler.ServeGlossaryPage())
mux.HandleFunc("GET /error", s.app.PageHandler.ServeErrorPage())
mux.HandleFunc("/", s.app.PageHandler.ServeLandingPage())
```

---

## 4. Test Cases

### Unit Tests

**VireClient.ProxyGet** (in `internal/client/vire_client_test.go`):
1. `TestProxyGet_Success` — mock 200, verify body returned
2. `TestProxyGet_SetsUserIDHeader` — verify X-Vire-User-ID header
3. `TestProxyGet_EmptyUserID` — no header when userID ""
4. `TestProxyGet_Non2xxReturnsError` — mock 500, verify error
5. `TestProxyGet_NetworkError` — unreachable, verify error

**Error page:**
6. `TestServeErrorPage_ResolvesReason` — `?reason=auth_failed` → "Authentication failed"
7. `TestServeErrorPage_UnknownReason` — `?reason=unknown` → "Something went wrong"
8. `TestServeErrorPage_NoReason` — no param → "Something went wrong"

**Landing page:**
9. `TestServeLandingPage_ServerUp` — mock health 200 → sign-in visible
10. `TestServeLandingPage_ServerDown` — mock health 503 → "unavailable"
11. `TestServeLandingPage_ClearsSessionCookie` — verify auto-logout preserved

**Strategy/Cash SSR:**
12. `TestStrategyHandler_SSR_EmbedJSON` — verify `window.__VIRE_DATA__` in body
13. `TestStrategyHandler_SSR_NilProxyGet` — nil fn → page renders
14. `TestCashHandler_SSR_EmbedJSON` — verify data embedded
15. `TestCashHandler_SSR_NilProxyGet` — nil fn → page renders

### UI Tests

16. Update strategy/cash/changelog tests — reduce sleep times
17. `TestGlossaryPage_NoLoadingSpinner` — verify no "Loading glossary..." text
18. `TestErrorPage_SSR_DisplaysMessage` — navigate to `/error?reason=auth_failed`, verify text

---

## 5. Edge Cases

1. **API unreachable during SSR:** All handlers gracefully handle proxyGet errors. Pages render with empty/null data. Never crash or 500.
2. **Empty JSON responses:** Templates handle nil/empty slices without panic.
3. **XSS safety:** `template.JS` for JSON in `<script>` tags (data from trusted vire-server). Go auto-escaping for `{{.ErrorMessage}}`.
4. **Portfolio names with special chars:** Use `url.PathEscape(selected)` in handler paths.
5. **Backward compatibility:** If `proxyGetFn` is nil, JS falls back to client-side fetch.
6. **Memory cleanup:** `window.__VIRE_DATA__ = null` after Alpine reads it.

---

## 6. Implementation Order

1. `VireClient.ProxyGet` + tests
2. Error page SSR
3. Landing page SSR
4. Glossary page SSR
5. Changelog page SSR
6. Help page SSR
7. Strategy page SSR
8. Cash page SSR
9. Wire in `app.go` + `routes.go`
10. Update UI tests
11. Full test suite
