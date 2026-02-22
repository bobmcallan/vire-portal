# Requirements: Dashboard Portfolio Management + MCP Page

**Date:** 2026-02-22
**Requested:** Split MCP tools to new page, add portfolio management to dashboard

## Scope

### In scope
1. Move MCP Connection + Tools table to new "MCP" page (`/mcp-info`)
2. Add "MCP" nav link after Dashboard in nav menu
3. Dashboard: portfolio dropdown populated from `GET /api/portfolios` (via /api/ proxy to vire-server)
4. Dashboard: select default portfolio on page load
5. Dashboard: editable markdown textareas for strategy and plan (per portfolio)
6. Dashboard: checkbox to set portfolio as default
7. Dashboard: portfolio holdings table from `GET /api/portfolios/{name}`
8. Dashboard: remove "Config -> Portfolios" section

### Out of scope
- Settings page changes
- Auth changes
- New vire-server endpoints (all exist)

## Approach

### Architecture
- **Server-side**: Go handlers render page templates with auth context
- **Client-side**: Alpine.js handles all dynamic data fetching via `/api/` proxy
- **API calls**: Alpine.js → `/api/portfolios/...` → portal proxy → vire-server REST endpoints

### vire-server REST Endpoints (via /api/ proxy)
```
GET  /api/portfolios                              → list_portfolios
GET  /api/portfolios/{name}                       → get_portfolio (holdings)
GET  /api/portfolios/{name}/strategy              → get_portfolio_strategy
PUT  /api/portfolios/{name}/strategy              → set_portfolio_strategy
GET  /api/portfolios/{name}/plan                  → get_portfolio_plan
PUT  /api/portfolios/{name}/plan                  → set_portfolio_plan
PUT  /api/portfolios/default                      → set_default_portfolio
```

### Page Structure

**MCP Page (`/mcp-info`):**
- MCP Connection section (endpoint URL, config JSON) — moved from dashboard
- Dev-mode MCP endpoint (if dev mode) — moved from dashboard
- Tools table (name, description, method, path) — moved from dashboard

**Dashboard (`/dashboard`):**
- Warning banner (if Navexa key missing) — unchanged
- Portfolio dropdown + "Set as default" checkbox
- Portfolio holdings table (selected fields, responsive)
- Strategy textarea (scrollable, editable markdown, full-width)
- Plan textarea (scrollable, editable markdown, full-width)
- Save buttons for strategy and plan

### Key Decisions
- MCP page route: `/mcp-info` (avoids conflict with `/mcp` MCP protocol endpoint)
- Nav order: Dashboard, MCP
- Alpine.js fetches data client-side for interactivity
- Textareas use raw markdown (no rendered preview)
- Holdings table fields: ticker, name, value, weight%, gain% (no horizontal overflow)

## Files Expected to Change

### New files
- `pages/mcp.html` — MCP page template
- `internal/handlers/mcp_page.go` — MCP page handler

### Modified files
- `pages/dashboard.html` — Remove MCP/Tools/Config sections, add portfolio UI
- `pages/partials/nav.html` — Add "MCP" link after Dashboard
- `internal/handlers/dashboard.go` — Remove catalog/config logic, simplify for portfolio page
- `internal/server/routes.go` — Add `GET /mcp-info` route
- `internal/app/app.go` — Wire MCP page handler
- `pages/static/common.js` — Add Alpine.js portfolio component
- `pages/static/css/portal.css` — Add portfolio styles (textarea, table, dropdown)
