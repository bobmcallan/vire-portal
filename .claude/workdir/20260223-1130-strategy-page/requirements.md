# Requirements: New Strategy Page

**Date:** 2026-02-23
**Requested:** Move Strategy and Plan sections from Dashboard to a new dedicated "Strategy" page with its own nav item.

## Scope
- **In scope:**
  - New `GET /strategy` route with handler
  - New `pages/strategy.html` template
  - New `portfolioStrategy()` Alpine.js component in `common.js`
  - Nav item "Strategy" added after "Dashboard" (desktop + mobile)
  - Remove Strategy and Plan sections from dashboard.html
  - Remove strategy/plan data properties and save methods from `portfolioDashboard()`
  - Update existing dashboard tests (remove strategy/plan editor assertions)
  - New strategy page tests in `tests/ui/strategy_test.go`
  - Update nav tests for the new link

- **Out of scope:**
  - Changing the API endpoints (they stay as `/api/portfolios/{name}/strategy` and `/api/portfolios/{name}/plan`)
  - Redesigning the strategy/plan UI beyond the move
  - Adding new functionality to strategy/plan management

## Approach

### Handler: `internal/handlers/strategy.go`
Follow the `MCPPageHandler` pattern (simple GET-only page):
- Struct: `StrategyHandler` with logger, templates, devMode, jwtSecret, userLookupFn, apiURL
- Constructor: `NewStrategyHandler(...)` — parses templates from pagesDir
- `ServeHTTP`: auth check → check navexaKeyMissing → render `strategy.html` with `"Page": "strategy"`
- `SetAPIURL()` method

### App registration: `internal/app/app.go`
- Add `StrategyHandler *handlers.StrategyHandler` to `App` struct (line 46, after DashboardHandler)
- Initialize in `initHandlers()` after DashboardHandler block (after line 124)

### Route: `internal/server/routes.go`
- Add `mux.HandleFunc("GET /strategy", s.app.StrategyHandler.ServeHTTP)` at line 34 (after dashboard route)

### Template: `pages/strategy.html`
- Same structure as dashboard.html but with `portfolioStrategy()` Alpine component
- Contains portfolio selector (reused pattern), strategy editor, and plan editor
- No holdings table, no portfolio summary

### Alpine component: `pages/static/common.js`
- New `portfolioStrategy()` function with: portfolios, selected, defaultPortfolio, strategy, plan, loading, error
- `init()` fetches portfolios, `loadPortfolio()` fetches strategy + plan
- `saveStrategy()` and `savePlan()` methods (moved from portfolioDashboard)
- Remove strategy/plan properties and methods from `portfolioDashboard()`
- Dashboard `loadPortfolio()` no longer fetches strategy/plan endpoints

### Nav: `pages/partials/nav.html`
- Desktop (line 9): add `<li><a href="/strategy" {{if eq .Page "strategy"}}class="active"{{end}}>Strategy</a></li>` after Dashboard
- Mobile (line 36): add `<a href="/strategy">Strategy</a>` after Dashboard

### Tests
- Update `tests/ui/dashboard_test.go`: remove `TestDashboardStrategyEditor` and `TestDashboardPlanEditor`
- Update `tests/ui/nav_test.go`: add `TestNavStrategyLinkPresent`
- New `tests/ui/strategy_test.go`: auth, Alpine init, portfolio selector, strategy editor, plan editor

## Files Expected to Change
- `internal/handlers/strategy.go` — NEW
- `internal/app/app.go` — add StrategyHandler field + init
- `internal/server/routes.go` — add GET /strategy route
- `pages/strategy.html` — NEW
- `pages/static/common.js` — new portfolioStrategy(), trim portfolioDashboard()
- `pages/partials/nav.html` — add Strategy nav link
- `pages/dashboard.html` — remove strategy/plan sections
- `tests/ui/strategy_test.go` — NEW
- `tests/ui/dashboard_test.go` — remove strategy/plan tests
- `tests/ui/nav_test.go` — add strategy link test
