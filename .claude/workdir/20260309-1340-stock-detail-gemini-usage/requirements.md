# Requirements: Stock Detail Page + Gemini Usage Admin

**Status:** fb_1f03ad9d
**Scope:** Two features — (1) expand stock detail page with real API data, (2) new admin Gemini usage page.
**Item 3 (dashboard labels):** Already done in previous session, skip.

---

## Feature 1: Stock Detail Page (expand existing skeleton)

### Scope
- Call the new `GET /api/portfolios/{name}/stock/{ticker}/detail` endpoint via proxyGetFn
- Pass raw JSON through as `StockDetailJSON` template.JS
- Expand `stockDetail()` Alpine component to render 5 new sections from the detail data
- Keep Trade History and Strategy Alignment as placeholders (API doesn't return position data)

### Handler Changes: `internal/handlers/stock.go`

After the existing portfolio/holding fetch (lines 83-117), add one more proxyGetFn call:

```go
var stockDetailJSON template.JS = "null"
if h.proxyGetFn != nil && claims != nil && claims.Sub != "" && selected != "" {
    path := "/api/portfolios/" + url.PathEscape(selected) + "/stock/" + url.PathEscape(ticker) + "/detail"
    if body, err := h.proxyGetFn(path, claims.Sub); err == nil {
        stockDetailJSON = template.JS(body)
    }
}
```

Add `"StockDetailJSON": stockDetailJSON` to the template data map.

**Note:** The `selected` variable is currently scoped inside the `if h.proxyGetFn != nil` block. Hoist it to be available after the block, or move the detail fetch inside the same block.

### Template Changes: `pages/stock.html`

Add `stockDetail: {{.StockDetailJSON}}` to `window.__VIRE_DATA__`.

Replace the 4 placeholder sections with data-driven sections:

1. **STOCK PRICE TREND** (replaces "PRICE CHART" placeholder)
   - Line chart: closing prices from `candles[]` (x=date, y=close)
   - Overlay SMA lines from `signals.price`: sma_20, sma_50, sma_200
   - Trend badge from `signals.trend` + `signals.trend_description`
   - RSI value from `signals.technical.rsi` with signal label
   - Chart.js (already loaded globally for dashboard)
   - Checkbox toggles for each SMA line (like dashboard growth chart)
   - `x-show="detail && detail.candles && detail.candles.length > 0"`

2. **FILINGS TIMELINE** (replaces "FILINGS" placeholder)
   - Table: date, headline, type, price_sensitive badge
   - Expandable rows: click to show key_facts[] list
   - `x-show="detail && detail.filing_summaries && detail.filing_summaries.length > 0"`
   - Use Alpine x-data for expand/collapse state per filing

3. **NEWS & SENTIMENT** (new section)
   - Summary paragraph from `news_intelligence.summary`
   - Sentiment badge: `news_intelligence.overall_sentiment`
   - Article list: title (linked), source, credibility badge, summary
   - `x-show="detail && detail.news_intelligence"`

4. **COMPANY OVERVIEW** (new section)
   - Business model from `company_timeline.business_model`
   - Key events list from `company_timeline.key_events[]` (date, event, detail)
   - Fundamentals grid: market_cap, pe_ratio, dividend_yield, sector, industry, beta, eps
   - `x-show="detail && (detail.fundamentals || detail.company_timeline)"`

5. **TRADE HISTORY** — keep placeholder "Trade history will be available in a future update."
6. **STRATEGY ALIGNMENT** — keep placeholder

### Alpine Component: `pages/static/common.js` — `stockDetail()`

Expand the existing component:

```javascript
// Add to state:
detail: null,        // raw stock detail API response
showSMA20: true,
showSMA50: true,
showSMA200: false,
expandedFilings: {}, // filing index -> boolean
priceChart: null,    // Chart.js instance

// Add to init():
this.detail = ssrData.stockDetail || null;
this.$nextTick(() => { if (this.detail) this.renderPriceChart(); });

// New methods:
renderPriceChart()    // Create Chart.js line chart from candles + SMA overlays
destroyPriceChart()   // Cleanup
toggleFiling(index)   // Toggle expandedFilings[index]
sentimentClass(s)     // Return CSS class for sentiment badge
credibilityClass(c)   // Return CSS class for credibility badge
fmtMarketCap(val)     // Format large numbers (B/M)
fmtDate(dateStr)      // Format ISO date to display
```

### CSS: `pages/static/css/portal.css`

Add styles for new sections (stock page styles already exist for .stock-detail-grid etc.):

```css
.stock-price-chart-section { ... }    /* Chart container */
.stock-trend-badge { ... }            /* Trend label badge */
.stock-rsi-value { ... }              /* RSI display */
.filing-row { cursor: pointer; }      /* Clickable filing rows */
.filing-row-expanded { ... }          /* Expanded state */
.filing-key-facts { ... }             /* Key facts list */
.filing-price-sensitive { ... }       /* Price sensitive badge */
.sentiment-badge { ... }              /* Sentiment indicator */
.credibility-badge { ... }            /* Article credibility */
.company-event { ... }                /* Key event item */
.fundamentals-grid { ... }            /* Fundamentals display */
```

---

## Feature 2: Gemini Usage Admin Page

### Scope
- New admin-only page at `/admin/gemini`
- Follows AdminUsersHandler pattern exactly
- SSR JSON hydration for initial "week" data
- Client-side period switching (Day/Week/Month) via Alpine fetch

### New File: `internal/handlers/gemini.go`

```go
type AdminGeminiHandler struct {
    logger       *common.Logger
    templates    *template.Template
    devMode      bool
    jwtSecret    []byte
    userLookupFn func(string) (*client.UserProfile, error)
    proxyGetFn   func(path, userID string) ([]byte, error)
    apiURL       string
}

func NewAdminGeminiHandler(
    logger *common.Logger,
    devMode bool,
    jwtSecret []byte,
    userLookupFn func(string) (*client.UserProfile, error),
) *AdminGeminiHandler

func (h *AdminGeminiHandler) SetAPIURL(apiURL string)
func (h *AdminGeminiHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error))
func (h *AdminGeminiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

ServeHTTP:
1. Check login, get user role
2. Gate: require admin role (redirect to /dashboard if not)
3. Fetch usage data: `proxyGetFn("/api/admin/gemini/usage?period=week", claims.Sub)`
4. Embed as `UsageJSON` template.JS
5. Render `gemini.html`

### New File: `pages/gemini.html`

```html
<!-- Standard head/nav/footer -->
<!-- main x-data="geminiUsage()" -->

<!-- Period tabs: Day / Week / Month -->
<!-- Headline stats: total_calls, total_prompt_tokens, total_output_tokens -->
<!-- Table: by_job_type breakdown -->
<!-- Table: by_model breakdown -->
<!-- Table: by_day breakdown -->
<!-- Table: recent calls (last 50) -->
```

### Alpine Component: `pages/static/common.js` — `geminiUsage()`

```javascript
function geminiUsage() {
    return {
        period: 'week',
        usage: null,
        loading: false,
        error: '',

        init() {
            const ssrData = window.__VIRE_DATA__;
            if (ssrData && ssrData.usage) {
                this.usage = ssrData.usage;
                window.__VIRE_DATA__ = null;
                return;
            }
            this.loadUsage();
        },

        async loadUsage() { ... },  // fetch /api/admin/gemini/usage?period=X
        async switchPeriod(p) { ... },  // update period, call loadUsage
        fmtTokens(n) { ... },  // format large numbers with commas
        fmtDuration(ms) { ... },  // format ms to seconds
        fmtTimestamp(ts) { ... },  // format ISO timestamp
    };
}
```

### App Wiring: `internal/app/app.go`

1. Add `AdminGeminiHandler *handlers.AdminGeminiHandler` to App struct
2. In `initHandlers()`:
```go
a.AdminGeminiHandler = handlers.NewAdminGeminiHandler(
    a.Logger,
    a.Config.IsDevMode(),
    jwtSecret,
    userLookup,
)
a.AdminGeminiHandler.SetAPIURL(a.Config.API.URL)
a.AdminGeminiHandler.SetProxyGetFn(cachedProxyGet)
```

### Routes: `internal/server/routes.go`

Add: `mux.HandleFunc("GET /admin/gemini", s.app.AdminGeminiHandler.ServeHTTP)`

### Nav: `pages/partials/nav.html`

Add admin link: `<a href="/admin/gemini" class="nav-admin-item">Gemini *</a>`

### MCP get_page: `internal/mcp/get_page.go`

Add "gemini" to allowedPages list.

---

## Test Cases

### Unit Tests

**`internal/handlers/stock_test.go`** (new or expand existing):
- TestStockHandler_StockDetailSSR — verify StockDetailJSON is embedded when proxyGetFn returns data
- TestStockHandler_StockDetailSSR_NilProxy — verify StockDetailJSON is "null" when no proxyGetFn
- TestStockHandler_StockDetailSSR_Error — verify StockDetailJSON is "null" when proxyGetFn errors

**`internal/handlers/gemini_test.go`** (new):
- TestAdminGeminiHandler_RequiresLogin — unauthenticated redirects to /
- TestAdminGeminiHandler_RequiresAdmin — non-admin redirects to /dashboard
- TestAdminGeminiHandler_RenderPage — admin sees gemini.html with UsageJSON
- TestAdminGeminiHandler_NilProxy — renders with "null" usage data
- TestAdminGeminiHandler_SetAPIURL — verify SetAPIURL works
- TestAdminGeminiHandler_SetProxyGetFn — verify SetProxyGetFn works

### Stress Tests

**`internal/handlers/stock_stress_test.go`** (expand):
- StressStockDetailJSONEscaping — verify XSS-safe JSON embedding
- StressStockDetailLargePayload — verify large API responses don't crash

**`internal/handlers/gemini_stress_test.go`** (new):
- StressGeminiAdminGate — verify non-admin can't access
- StressGeminiXSSInUsageData — verify malicious JSON is safely embedded
- StressGeminiNilFields — verify nil/empty fields don't crash template

### UI Tests

**`tests/ui/stock_test.go`** (expand):
- TestStockPriceChartSection — verify chart section visible with candle data
- TestStockFilingsSection — verify filings table renders
- TestStockNewsSection — verify news section renders
- TestStockCompanyOverview — verify fundamentals render

**`tests/ui/gemini_test.go`** (new):
- TestGeminiPageRequiresAdmin — non-admin can't access
- TestGeminiPageRendersStats — admin sees usage stats
- TestGeminiPeriodSwitching — period tabs work
- TestGeminiNavLink — admin nav shows "Gemini *" link

---

## Edge Cases
- Stock detail API returns empty/null for some sections (e.g. no news, no filings) — sections should hide gracefully
- Stock detail API errors — fall back to just showing holding data (existing behavior)
- Gemini usage API returns empty data — show "No usage data" message
- Gemini usage with no by_ticker data — hide ticker table
- XSS in API responses — all data embedded via template.JS (Go's safe JSON escaping)
- Non-admin accessing /admin/gemini — redirect to /dashboard

## Dependencies
- Chart.js (already loaded globally)
- No new packages needed
