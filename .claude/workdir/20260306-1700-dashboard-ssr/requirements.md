# Dashboard SSR Migration - Requirements

## 1. Scope

**Goal:** Migrate the dashboard page from client-side Alpine.js data fetching to server-side rendering via JSON hydration. On initial page load with the user's default portfolio, the server fetches all 5 API data sources and embeds the results as JSON in `window.__VIRE_DATA__`. Alpine reads this data on `init()` instead of making client-side fetches, eliminating the "Loading portfolios..." spinner.

**In scope:**
- Add `proxyGetFn` to `DashboardHandler`
- Server-side fetching of: portfolios list, portfolio data, timeline, watchlist, glossary
- JSON hydration in `dashboard.html` template via `window.__VIRE_DATA__`
- Modify `portfolioDashboard()` in `common.js` to check for SSR data before fetching
- Wire `proxyGetFn` in `app.go`
- Unit tests and stress tests for the new handler logic
- Fallback: if `proxyGetFn` is nil or any server-side fetch fails, Alpine falls back to client-side fetch (current behavior)

**NOT in scope (remains client-side only):**
- Portfolio switching via dropdown (re-fetches via client-side `loadPortfolio()`)
- Refresh button (force_refresh=true, bypasses SSR)
- Show/hide closed positions (lazy-loads via `fetchClosedHoldings()`)
- Chart toggle controls (breakdown, MA overlays)
- Default portfolio toggle
- WebSocket live updates (separate feature: fb_fa72a550)
- No changes to routes, CSS, or HTML structure

---

## 2. File Changes

### 2.1 `internal/handlers/dashboard.go`

**Changes:**
1. Add `proxyGetFn` field to `DashboardHandler` struct
2. Add `SetProxyGetFn` method
3. Modify `ServeHTTP` to fetch SSR data and embed JSON variables in template data

**Modified struct:**
```go
type DashboardHandler struct {
    logger       *common.Logger
    templates    *template.Template
    devMode      bool
    jwtSecret    []byte
    userLookupFn func(string) (*client.UserProfile, error)
    apiURL       string
    proxyGetFn   func(path, userID string) ([]byte, error)  // NEW
}
```

**New method:**
```go
func (h *DashboardHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error))
```

**Modified `ServeHTTP` logic** (follows `strategy.go` pattern exactly):

After the existing auth/user-lookup block, before building the `data` map, add the SSR data fetching block:

```go
var portfoliosJSON, portfolioJSON, timelineJSON, watchlistJSON, glossaryJSON template.JS
portfoliosJSON = "null"
portfolioJSON = "null"
timelineJSON = "null"
watchlistJSON = "null"
glossaryJSON = "null"

if h.proxyGetFn != nil && claims != nil && claims.Sub != "" {
    // 1. Fetch portfolio list
    if body, err := h.proxyGetFn("/api/portfolios", claims.Sub); err == nil {
        portfoliosJSON = template.JS(body)

        // Parse to find default/selected portfolio name
        var pData struct {
            Portfolios []struct {
                Name string `json:"name"`
            } `json:"portfolios"`
            Default string `json:"default"`
        }
        if json.Unmarshal(body, &pData) == nil {
            selected := pData.Default
            if selected == "" && len(pData.Portfolios) > 0 {
                selected = pData.Portfolios[0].Name
            }
            if selected != "" {
                // 2. Fetch portfolio data (holdings, metrics, changes, breadth)
                if pBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected), claims.Sub); err == nil {
                    portfolioJSON = template.JS(pBody)
                }
                // 3. Fetch timeline (growth chart data)
                if tBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/timeline", claims.Sub); err == nil {
                    timelineJSON = template.JS(tBody)
                }
                // 4. Fetch watchlist
                if wBody, err := h.proxyGetFn("/api/portfolios/"+url.PathEscape(selected)+"/watchlist", claims.Sub); err == nil {
                    watchlistJSON = template.JS(wBody)
                }
            }
        }
    }
    // 5. Fetch glossary (independent of portfolio selection)
    if gBody, err := h.proxyGetFn("/api/glossary", claims.Sub); err == nil {
        glossaryJSON = template.JS(gBody)
    }
}
```

Then add these to the `data` map:
```go
"PortfoliosJSON":   portfoliosJSON,
"PortfolioJSON":    portfolioJSON,
"TimelineJSON":     timelineJSON,
"WatchlistJSON":    watchlistJSON,
"GlossaryJSON":     glossaryJSON,
```

**New imports needed:** `"encoding/json"`, `"net/url"` (add to existing import block).

---

### 2.2 `pages/dashboard.html`

**Change:** Add `<script>` block before `</body>` to embed SSR data. Follow the exact pattern from `strategy.html` and `cash.html`.

Add between `{{template "footer.html" .}}` and `</body>`:
```html
    <script>
    window.__VIRE_DATA__ = {
        portfolios: {{.PortfoliosJSON}},
        portfolio: {{.PortfolioJSON}},
        timeline: {{.TimelineJSON}},
        watchlist: {{.WatchlistJSON}},
        glossary: {{.GlossaryJSON}}
    };
    </script>
```

---

### 2.3 `pages/static/common.js` -- `portfolioDashboard()`

**Changes:**

#### A. New helper method `_applyPortfolioData(holdingsData)`

Extract the portfolio data parsing logic from `loadPortfolio()` into a reusable private method. This method is used by: (1) SSR hydration, (2) `loadPortfolio()`, (3) `refreshPortfolio()`.

```javascript
_applyPortfolioData(holdingsData) {
    this.holdings = vireStore.dedup(holdingsData.holdings || [], 'ticker');
    this.totalDividends = Number(holdingsData.income_dividends_forecast) || 0;
    this.ledgerDividendReturn = Number(holdingsData.income_dividends_received) || 0;
    this.lastSynced = holdingsData.last_synced || '';
    // Parse changes
    const changes = holdingsData.changes;
    if (changes) {
        this.changeDayPct = changes.yesterday?.portfolio_value?.has_previous ? changes.yesterday.portfolio_value.pct_change : null;
        this.changeWeekPct = changes.week?.portfolio_value?.has_previous ? changes.week.portfolio_value.pct_change : null;
        this.changeMonthPct = changes.month?.portfolio_value?.has_previous ? changes.month.portfolio_value.pct_change : null;
        this.hasChanges = this.changeDayPct !== null || this.changeWeekPct !== null || this.changeMonthPct !== null;
        this.changeCashDayPct = changes.yesterday?.capital_gross?.has_previous ? changes.yesterday.capital_gross.pct_change : null;
        this.changeCashWeekPct = changes.week?.capital_gross?.has_previous ? changes.week.capital_gross.pct_change : null;
        this.changeCashMonthPct = changes.month?.capital_gross?.has_previous ? changes.month.capital_gross.pct_change : null;
        this.hasCashChanges = this.changeCashDayPct !== null || this.changeCashWeekPct !== null || this.changeCashMonthPct !== null;
        this.changeReturnDayDollar = changes.yesterday?.equity_holdings_value?.has_previous ? changes.yesterday.equity_holdings_value.raw_change : null;
        this.changeReturnWeekDollar = changes.week?.equity_holdings_value?.has_previous ? changes.week.equity_holdings_value.raw_change : null;
        this.changeReturnMonthDollar = changes.month?.equity_holdings_value?.has_previous ? changes.month.equity_holdings_value.raw_change : null;
        this.hasReturnDollarChanges = this.changeReturnDayDollar !== null || this.changeReturnWeekDollar !== null || this.changeReturnMonthDollar !== null;
        this.changeReturnDayPct = changes.yesterday?.equity_holdings_value?.has_previous ? changes.yesterday.equity_holdings_value.pct_change : null;
        this.changeReturnWeekPct = changes.week?.equity_holdings_value?.has_previous ? changes.week.equity_holdings_value.pct_change : null;
        this.changeReturnMonthPct = changes.month?.equity_holdings_value?.has_previous ? changes.month.equity_holdings_value.pct_change : null;
        this.hasReturnPctChanges = this.changeReturnDayPct !== null || this.changeReturnWeekPct !== null || this.changeReturnMonthPct !== null;
    } else {
        this.changeDayPct = null; this.changeWeekPct = null; this.changeMonthPct = null; this.hasChanges = false;
        this.changeCashDayPct = null; this.changeCashWeekPct = null; this.changeCashMonthPct = null; this.hasCashChanges = false;
        this.changeReturnDayDollar = null; this.changeReturnWeekDollar = null; this.changeReturnMonthDollar = null; this.hasReturnDollarChanges = false;
        this.changeReturnDayPct = null; this.changeReturnWeekPct = null; this.changeReturnMonthPct = null; this.hasReturnPctChanges = false;
    }
    this.portfolioTotalValue = Number(holdingsData.portfolio_value) || 0;
    this.portfolioGain = Number(holdingsData.equity_holdings_return) || 0;
    this.portfolioGainPct = Number(holdingsData.equity_holdings_return_pct) || 0;
    this.portfolioCost = Number(holdingsData.equity_holdings_cost) || 0;
    this.equityValue = Number(holdingsData.equity_holdings_value) || 0;
    this.grossCashBalance = Number(holdingsData.capital_gross) || 0;
    this.availableCash = Number(holdingsData.capital_available) || 0;
    const cp = holdingsData.capital_performance;
    if (cp && cp.transaction_count > 0) {
        this.capitalInvested = Number(cp.capital_contributions_net) || 0;
        this.grossContributions = Number(cp.capital_contributions_gross) || 0;
        this.hasCapitalData = true;
    } else {
        this.capitalInvested = 0; this.grossContributions = 0; this.hasCapitalData = false;
    }
    this.breadth = holdingsData.breadth || this.computeBreadth();
    this.hasBreadth = this.breadth !== null;
},
```

#### B. New helper method `_applyTimelineData(timelineData)`

```javascript
_applyTimelineData(timelineData) {
    const points = timelineData.data_points || [];
    this.growthData = this.filterAnomalies(points);
    this.hasGrowthData = this.growthData.length > 0;
},
```

#### C. Modify `init()` to check SSR data first

At the top of `init()`, before the existing client-side fetch, add the SSR hydration path:

```javascript
async init() {
    try {
        const ssrData = window.__VIRE_DATA__;
        if (ssrData && ssrData.portfolios) {
            // --- SSR path: hydrate from server-embedded data ---
            const data = ssrData.portfolios;
            this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
            this.defaultPortfolio = data.default || '';
            if (this.defaultPortfolio) {
                this.selected = this.defaultPortfolio;
            } else if (this.portfolios.length > 0) {
                this.selected = this.portfolios[0].name;
            }

            if (ssrData.portfolio) {
                this._applyPortfolioData(ssrData.portfolio);
            }
            if (ssrData.timeline) {
                this._applyTimelineData(ssrData.timeline);
            }
            if (ssrData.watchlist) {
                this.watchlist = ssrData.watchlist.items || [];
            }
            if (ssrData.glossary) {
                const map = {};
                for (const cat of (ssrData.glossary.categories || [])) {
                    for (const t of (cat.terms || [])) {
                        map[t.term] = t.definition;
                    }
                }
                this.glossary = map;
            }

            window.__VIRE_DATA__ = null;
            this.loading = false;

            // Set up watchers (same as client-side path)
            this.$watch('showClosed', (val) => {
                if (val) this.fetchClosedHoldings();
            });
            this.$watch('showChartBreakdown', () => this.renderChart());
            this.$watch('showMA20', () => this.renderChart());
            this.$watch('showMA50', () => this.renderChart());
            this.$watch('showMA200', () => this.renderChart());

            if (this.hasGrowthData) {
                this.$nextTick(() => this.renderChart());
            }
            return;
        }

        // --- Client-side fallback path (unchanged) ---
        // ... existing init() code ...
```

#### D. Refactor `loadPortfolio()` and `refreshPortfolio()`

Replace inline portfolio data parsing in `loadPortfolio()` and `refreshPortfolio()` with calls to `this._applyPortfolioData(data)`.

---

### 2.4 `internal/app/app.go`

**Change:** Add one line after the existing CashHandler `SetProxyGetFn` call (around line 172):

```go
a.DashboardHandler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
    return vireClient.ProxyGet(path, userID)
})
```

---

## 3. Function Signatures

```go
// internal/handlers/dashboard.go
func (h *DashboardHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error))
func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) // existing, modified
```

No new Go types, no new files, no new routes.

---

## 4. Template Structure

No HTML structure changes. Only addition is the `<script>` hydration block in `dashboard.html` using `{{.PortfoliosJSON}}`, `{{.PortfolioJSON}}`, `{{.TimelineJSON}}`, `{{.WatchlistJSON}}`, `{{.GlossaryJSON}}` — all of type `template.JS` (renders as raw JS, safe because values come from JSON server responses).

---

## 5. Unit Test Cases

Add to `internal/handlers/handlers_test.go`. Follow patterns from `TestStrategyHandler_SSR_EmbedJSON` and `TestCashHandler_SSR_EmbedJSON`.

### Test 1: `TestDashboardHandler_SSR_EmbedJSON`
- Create DashboardHandler with proxyGetFn returning mock data for all 5 endpoints
- Mock returns: portfolios with default "Test", portfolio data with holdings, timeline with data_points, watchlist with items, glossary with categories
- Assert status 200
- Assert body contains `window.__VIRE_DATA__`
- Assert body contains portfolio data, timeline data, watchlist data, glossary data

### Test 2: `TestDashboardHandler_SSR_NilProxyGet`
- Create DashboardHandler with NO SetProxyGetFn call
- Assert status 200
- Assert body contains `null` for all JSON fields (graceful fallback)

### Test 3: `TestDashboardHandler_SSR_ProxyGetPartialFailure`
- proxyGetFn succeeds for portfolios and portfolio data, fails for timeline/watchlist/glossary
- Assert status 200
- Assert portfolios/portfolio data present, timeline/watchlist/glossary are `null`

### Test 4: `TestDashboardHandler_SSR_ProxyGetPortfoliosFailure`
- proxyGetFn fails for /api/portfolios
- Assert status 200, all JSON fields are `null`

### Test 5: `TestDashboardHandler_SSR_NoDefaultPortfolio`
- proxyGetFn returns portfolios with no default, one portfolio named "First"
- Assert handler selects first portfolio and fetches its data

---

## 6. Stress Test Cases

Add to `internal/handlers/dashboard_stress_test.go` or existing stress test file.

### Stress 1: `TestDashboardSSR_StressConcurrentRenders`
- 20 concurrent requests with valid auth and proxyGetFn
- Assert all return 200 with `window.__VIRE_DATA__`

### Stress 2: `TestDashboardSSR_StressLargeTimelineJSON`
- Mock timeline with 500 data points
- Verify full JSON embedded without truncation

### Stress 3: `TestDashboardSSR_StressXSSViaProxyGet`
- Mock proxyGetFn returns JSON with XSS payloads in string values
- Assert no unescaped `<script>` tags in response

### Stress 4: `TestDashboardSSR_StressProxyGetTimeout`
- Mock proxyGetFn blocks for 5s on some endpoints
- Verify handler returns 200 with partial data (non-fatal failures)

---

## 7. UI Test Cases

Add to `tests/ui/dashboard_test.go`.

### UI Test 1: `TestDashboardSSR_NoLoadingSpinner`
- Login and navigate to /dashboard
- Assert "Loading portfolios..." is NOT visible (SSR pre-populated)
- Portfolio selector should be immediately visible

### UI Test 2: `TestDashboardSSR_DataPreRendered`
- Login and navigate to /dashboard
- Assert holdings table visible without waiting for fetch
- Assert Alpine loading state is false

### UI Test 3: `TestDashboardSSR_VireDataCleared`
- Login and navigate to /dashboard
- Wait for Alpine init
- Evaluate JS: `window.__VIRE_DATA__ === null`

---

## 8. Edge Cases

1. **proxyGetFn is nil:** All JSON fields default to "null". Alpine init() falls through to client-side fetch.
2. **proxyGetFn errors on /api/portfolios:** All fields stay "null", Alpine falls back.
3. **Partial fetch failures:** Each fetch independent. Failed fields are "null", Alpine checks each.
4. **Empty portfolios:** `{"portfolios":[],"default":""}` — no portfolio/timeline/watchlist fetches.
5. **User not authenticated:** Handler redirects to `/`, no SSR fetching.
6. **Portfolio name with special chars:** `url.PathEscape(selected)` handles encoding.
7. **Large timeline (500+ points):** Full JSON embedded, `filterAnomalies()` still runs on SSR data.
8. **Glossary independent:** Fetched regardless of portfolio selection.

---

## 9. Dependencies

No new dependencies. All packages already in use: `encoding/json`, `html/template`, `net/url`, `VireClient.ProxyGet`.

No new routes. Existing `GET /dashboard` unchanged.

---

## 10. Implementation Order

1. `internal/handlers/dashboard.go` — proxyGetFn field, SetProxyGetFn, SSR fetch in ServeHTTP
2. `internal/app/app.go` — Wire SetProxyGetFn (one line)
3. `pages/dashboard.html` — Add script hydration block
4. `pages/static/common.js` — Add _applyPortfolioData(), _applyTimelineData(), modify init(), refactor loadPortfolio()/refreshPortfolio()
5. `internal/handlers/handlers_test.go` — 5 unit tests
6. `internal/handlers/dashboard_stress_test.go` — 4 stress tests
7. `tests/ui/dashboard_test.go` — 3 UI tests
