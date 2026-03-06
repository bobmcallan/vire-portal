# Mobile Dashboard - Implementation Spec

## Scope

**What it does:** A mobile-optimized dashboard page at `/m` and `/m/{portfolio...}` showing:
1. Portfolio value with D/W/M change indicators
2. Performance summary (net return $, net return %)
3. Simple growth chart (Chart.js, last 6 months, portfolio value line only)
4. Holdings list as cards (ticker, value, return $, return %)

**What it does NOT do:**
- No watchlist section
- No breadth bar
- No closed positions toggle
- No chart controls (MA toggles, breakdown)
- No portfolio default checkbox
- No glossary tooltips
- Does NOT replace or modify the existing desktop dashboard

## File Changes

### 1. `internal/handlers/mobile_dashboard.go` (NEW)

Create handler following `dashboard.go` pattern. Nearly identical SSR flow:
- Struct: `MobileDashboardHandler` with same fields as `DashboardHandler`
- Constructor: `NewMobileDashboardHandler(logger, devMode, jwtSecret, userLookupFn)`
- Methods: `SetAPIURL()`, `SetProxyGetFn()`
- ServeHTTP: Same auth, same SSR fetches (portfolios, portfolio, timeline — skip watchlist and glossary for mobile), render `mobile.html`
- URL portfolio extraction: trim `/m` prefix instead of `/dashboard`

### 2. `pages/mobile.html` (NEW)

Template using `portfolioDashboard()` Alpine component. Simplified layout:
- head.html partial, nav.html partial, footer.html partial
- Single `x-data="portfolioDashboard()"` on main
- SSR data injection via `window.__VIRE_DATA__`
- Sections: portfolio name header, value+changes, performance row, chart, holdings cards
- "Full Dashboard" link at bottom

### 3. `pages/static/css/portal.css` (MODIFY)

Add `/* MOBILE DASHBOARD */` section with:
- `.mobile-dashboard` - container styles
- `.mobile-portfolio-header` - portfolio name + refresh button
- `.mobile-value-card` - portfolio value display
- `.mobile-changes` - D/W/M change row
- `.mobile-performance` - return $ and % display
- `.mobile-chart` - chart container (180px height)
- `.mobile-holdings` - card list container
- `.mobile-holding-card` - individual holding card

### 4. `internal/app/app.go` (MODIFY)

- Add `MobileDashboardHandler *handlers.MobileDashboardHandler` to App struct
- Initialize in `initHandlers()` following DashboardHandler pattern
- Set API URL and proxy function

### 5. `internal/server/routes.go` (MODIFY)

Add routes:
```
mux.HandleFunc("GET /m", s.app.MobileDashboardHandler.ServeHTTP)
mux.HandleFunc("GET /m/{portfolio...}", s.app.MobileDashboardHandler.ServeHTTP)
```

### 6. `pages/partials/nav.html` (MODIFY)

Add "Mobile" link in mobile menu section (visible on mobile slide-out menu).

## Function Signatures

```go
// internal/handlers/mobile_dashboard.go
type MobileDashboardHandler struct {
    logger       *common.Logger
    templates    *template.Template
    devMode      bool
    jwtSecret    []byte
    userLookupFn func(string) (*client.UserProfile, error)
    apiURL       string
    proxyGetFn   func(path, userID string) ([]byte, error)
}

func NewMobileDashboardHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error)) *MobileDashboardHandler
func (h *MobileDashboardHandler) SetAPIURL(apiURL string)
func (h *MobileDashboardHandler) SetProxyGetFn(fn func(path, userID string) ([]byte, error))
func (h *MobileDashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

## Template Structure (mobile.html)

```html
<!DOCTYPE html>
<html lang="en">
<head>
    {{template "head.html" .}}
    <title>VIRE MOBILE</title>
</head>
<body>
    {{if .LoggedIn}}{{template "nav.html" .}}{{end}}
    <main class="page" x-data="portfolioDashboard()">
        <div class="page-body mobile-dashboard">
            <!-- Warning banner (if navexa key missing) -->
            <!-- Loading state -->
            <!-- Portfolio header: name + refresh button -->
            <!-- Portfolio value card with D/W/M changes -->
            <!-- Performance: net return $ and % -->
            <!-- Growth chart (simple, no controls) -->
            <!-- Holdings cards -->
            <!-- Full Dashboard link -->
        </div>
    </main>
    {{template "footer.html" .}}
    <script>
    window.__VIRE_DATA__ = {
        portfolios: {{.PortfoliosJSON}},
        portfolio: {{.PortfolioJSON}},
        timeline: {{.TimelineJSON}},
        watchlist: null,
        glossary: null,
        selectedPortfolio: {{.SelectedJSON}}
    };
    </script>
</body>
</html>
```

## CSS Classes

All under `/* MOBILE DASHBOARD */` section in portal.css:

- `.mobile-dashboard` - max-width: 100%; padding: 1rem
- `.mobile-portfolio-name` - font-size: 1rem; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase
- `.mobile-value-section` - border: 2px solid #000; padding: 1rem; margin-bottom: 1rem
- `.mobile-value-amount` - font-size: 1.5rem; font-weight: 700
- `.mobile-changes-row` - display: flex; gap: 1rem; font-size: 0.75rem
- `.mobile-performance-row` - display: flex; justify-content: space-between; border: 2px solid #000; padding: 1rem; margin-bottom: 1rem
- `.mobile-perf-item` - display: flex; flex-direction: column
- `.mobile-chart-section` - border: 2px solid #000; padding: 0.75rem; margin-bottom: 1rem
- `.mobile-chart-container` - height: 160px
- `.mobile-holdings-section` - margin-bottom: 1rem
- `.mobile-holding-card` - border: 2px solid #000; padding: 0.75rem; margin-bottom: 0.5rem; display: flex; justify-content: space-between; align-items: center
- `.mobile-holding-ticker` - font-weight: 700
- `.mobile-holding-value` - text-align: right
- `.mobile-full-link` - text-align: center; padding: 1rem; font-weight: 700

## Test Cases

### Unit Tests (`internal/handlers/mobile_dashboard_test.go`)

1. `TestMobileDashboardHandler_RedirectsUnauthenticated` - GET /m without session cookie → 302 to /
2. `TestMobileDashboardHandler_RendersForAuthenticated` - GET /m with valid JWT → 200
3. `TestMobileDashboardHandler_PortfolioPath` - GET /m/MyPortfolio → extracts portfolio name correctly

### Route Tests (`internal/server/routes_test.go` - add to existing)

4. `TestRoutes_MobileDashboard` - GET /m returns 302 (redirect, unauthenticated) or 200 (authenticated)
5. `TestRoutes_MobileDashboardWithPortfolio` - GET /m/TestPortfolio routes correctly

## Edge Cases

1. **Unauthenticated** → redirect to `/` (same as dashboard)
2. **No portfolios** → show "No portfolios found" message
3. **Portfolio loading** → show loading overlay
4. **No growth data** → hide chart section
5. **Empty holdings** → show empty state
6. **NavexaKeyMissing** → show warning banner
7. **URL portfolio not found** → fall back to default

## Dependencies

No new dependencies. Uses existing:
- `html/template` for rendering
- Alpine.js `portfolioDashboard()` component from common.js
- Chart.js for growth chart
- Existing CSS design system
