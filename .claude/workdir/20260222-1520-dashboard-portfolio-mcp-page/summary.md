# Summary: Dashboard Portfolio Management + MCP Page

**Date:** 2026-02-22
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/handlers/mcp_page.go` | **New.** MCPPageHandler extracted from DashboardHandler; serves `/mcp-info` with MCP connection details, dev endpoint, and tools table |
| `pages/mcp.html` | **New.** MCP page template with connection panel and tools table using `panel-headed` component library |
| `internal/handlers/dashboard.go` | Simplified constructor from 6 to 4 params (removed `port`, `catalogFn`); removed `DashboardTool`, `DashboardConfigStatus`, `SetConfigStatus`, `SetDevMCPEndpointFn`; now serves portfolio management UI |
| `pages/dashboard.html` | Replaced MCP/Tools/Config sections with Alpine.js portfolio UI: dropdown selector, default checkbox, holdings table, strategy/plan markdown editors with save buttons |
| `pages/partials/nav.html` | Added "MCP" link in desktop and mobile navigation (after Dashboard) |
| `internal/app/app.go` | Added `MCPPageHandler` to App struct; rewired catalog adapter to use `MCPPageTool`; simplified DashboardHandler construction; added MCPPageHandler wiring with SetAPIURL and SetDevMCPEndpointFn |
| `internal/server/routes.go` | Added `GET /mcp-info` route mapped to `MCPPageHandler.ServeHTTP` |
| `pages/static/common.js` | Added `portfolioDashboard()` Alpine.js component: portfolio list fetch, selection, holdings/strategy/plan loading, default toggle, strategy/plan save with toast notifications |
| `pages/static/css/portal.css` | Replaced dashboard legacy CSS (`.dashboard`, `.dashboard-section`, etc.) with portfolio styles (`.portfolio-header`, `.portfolio-select`, `.portfolio-editor`, `.text-right`) |
| `internal/handlers/handlers_test.go` | Updated all `NewDashboardHandler` calls to 4-param signature; moved MCP/tool tests to `TestMCPPageHandler_*`; replaced `DashboardTool` with `MCPPageTool` |
| `internal/handlers/auth_stress_test.go` | Updated `NewDashboardHandler` call to 4-param signature |
| `internal/handlers/dashboard_stress_test.go` | Fixed XSS test false positive: changed check from `onerror=alert` to `<img src=x onerror` (Go html/template escapes angle brackets making the payload safe) |
| `tests/ui/dashboard_test.go` | Updated selectors: `.dashboard` to `.page`, `.dashboard-section` to `.panel-headed` |
| `tests/ui/dev_auth_test.go` | Updated selector: `.dashboard-section` to `.panel-headed` |
| `README.md` | Added `/mcp-info` route, updated dashboard description, added `mcp_page.go` and `mcp.html` to project structure, updated `common.js` description |
| `.claude/skills/develop/SKILL.md` | Added `/mcp-info` route to routes table |

## Tests

### Modified
- `TestDashboardHandler_Returns200` -- updated constructor, removed unused `tools` variable
- `TestMCPPageHandler_ContainsToolCatalog` -- renamed from `TestDashboardHandler_ContainsToolCatalog`, uses `MCPPageTool`
- `TestMCPPageHandler_ContainsMCPConnectionConfig` -- renamed from dashboard equivalent
- `TestMCPPageHandler_ShowsEmptyToolsMessage` -- renamed from dashboard equivalent
- `TestMCPPageHandler_XSSEscaping` -- renamed from dashboard equivalent
- `TestMCPPageHandler_ToolCount` -- renamed from dashboard equivalent
- `TestMCPPageHandler_ToolTableUseComponentLibraryClasses` -- renamed from dashboard equivalent
- `TestMCPPageHandler_PortInMCPEndpoint` -- renamed from dashboard equivalent
- `TestMCPPageHandler_PortZero` -- renamed from dashboard equivalent
- `TestMCPPageHandler_PortNegative` -- renamed from dashboard equivalent
- `TestMCPPageHandler_StressXSSInToolData` -- fixed false positive assertion

### Results
- All handler unit tests: **PASS** (54.8s)
- `go vet ./...`: **PASS** (clean)
- Smoke UI tests: **PASS** (all)
- Dashboard UI tests: **PASS** (2 skipped -- collapsible panels/tabs no longer exist)
- Nav UI tests: **PASS** (12/12)
- Dev auth tests: 2 pre-existing failures (`TestDevAuthSettingsMCPEndpoint`, `TestDevAuthSettingsMCPURL`) -- confirmed as pre-existing, unrelated to this change

## Documentation Updated
- `README.md` -- added `/mcp-info` route, updated `/dashboard` description, added new files to project structure
- `.claude/skills/develop/SKILL.md` -- added `/mcp-info` route to routes reference table

## Devils-Advocate Findings
- **XSS test false positive** (medium): `TestMCPPageHandler_StressXSSInToolData` was checking for `onerror=alert` in rendered output. Go `html/template` escapes `<` and `>` but leaves `onerror=alert` as harmless literal text within `&lt;img src=x onerror=alert(1)&gt;`. Fixed by checking for unescaped `<img src=x onerror` instead, which correctly validates that the XSS payload is neutralized.
- All other stress tests passed without issues.

## Notes
- The dashboard now fetches all portfolio data client-side via Alpine.js through the `/api/` proxy to vire-server. The Go handler is simplified to only provide auth context and page metadata.
- MCP page route is `/mcp-info` (not `/mcp`) to avoid conflict with the MCP protocol endpoint at `POST /mcp`.
- Strategy and plan are stored/retrieved as JSON objects via the vire-server REST API. The dashboard presents the `notes` field as editable markdown in textareas.
- Holdings table columns: Ticker, Name, Value, Weight%, Gain% -- chosen to avoid horizontal overflow.
- Portfolio names containing special characters are safely handled via `encodeURIComponent` in the Alpine.js component.
