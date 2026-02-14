# Plan: Dashboard Page + README Port Fix

## Summary

Two deliverables:
1. **Dashboard page** at `/dashboard` showing MCP connection config, available tools from catalog, and config status
2. **README.md port fix** changing all stale `8080` references to `4241` (the actual default port in `defaults.go`)

## Analysis

### Current State
- **Default port is 4241** (`internal/config/defaults.go:8`) but README.md references `8080` in 14 places
- **MCP catalog is fetched at startup** in `handler.go:37-67` but the validated catalog is not stored on the `Handler` struct -- it's consumed by `RegisterToolsFromCatalog` and discarded
- **PageHandler** uses `ServePage(templateName, pageName)` which passes a fixed `map[string]interface{}` with `Page` and `DevMode` keys -- no way to pass custom data like tools or config
- **Template loading** uses `template.ParseGlob("*.html")` so any new `pages/dashboard.html` is auto-discovered

### Key Design Decisions

1. **Expose catalog from MCP handler**: Store `[]CatalogTool` on the `Handler` struct so it can be accessed by the dashboard handler. Add a `Catalog() []CatalogTool` getter method.

2. **New DashboardHandler** (not PageHandler): The dashboard needs dynamic data (tools, config, port). Create a dedicated `DashboardHandler` struct in `internal/handlers/dashboard.go` that holds references to config and the MCP handler's catalog. It renders `dashboard.html` with a richer data map.

3. **Template approach**: Use the same template system (Go html/template) and shared partials (head.html, nav.html, footer.html). The dashboard template gets data injected including tools list, MCP connection snippets, and config status.

4. **80s B&W aesthetic**: Dashboard uses same CSS file with new dashboard-specific classes. Terminal-style layout with bordered sections, monochrome only, IBM Plex Mono.

5. **README port fix**: Context-aware -- local/default port references change to 4241, Cloud Run / Terraform / Dockerfile references stay at 8080.

## Implementation Plan

### Step 1: Expose catalog from MCP Handler

**File: `internal/mcp/handler.go`**
- Add `catalog []CatalogTool` field to `Handler` struct
- Store validated catalog on the struct after `ValidateCatalog` call
- Add `Catalog() []CatalogTool` public method that returns a copy

### Step 2: Create DashboardHandler

**File: `internal/handlers/dashboard.go`** (new)

To avoid circular imports between `handlers` and `mcp` packages, define a simple `DashboardTool` struct in the handlers package:

```go
type DashboardTool struct {
    Name        string
    Description string
    Method      string
    Path        string
}
```

`DashboardHandler` struct:
- `logger *common.Logger`
- `templates *template.Template`
- `devMode bool`
- `port int`
- `catalogFn func() []DashboardTool` -- adapter function avoids mcp import

Constructor: `NewDashboardHandler(logger, templates, devMode, port, catalogFn)`

Note: the `templates` are shared with PageHandler -- pass the same `*template.Template` from PageHandler or load independently. Since PageHandler already loads all templates including partials, the cleanest approach is to have the DashboardHandler load its own templates the same way (call `findPagesDir()` and ParseGlob). Or better: have PageHandler expose its templates via a `Templates()` getter, then pass them to DashboardHandler.

Decision: Export `FindPagesDir()` (capitalize) and have DashboardHandler load templates itself, same pattern as PageHandler. This avoids coupling.

Actually, simplest: make `findPagesDir` exported as `FindPagesDir` so both handlers can use it independently. DashboardHandler parses templates in its constructor just like PageHandler does.

`ServeHTTP` method renders `dashboard.html` with template data:
- `Page` = "dashboard"
- `DevMode` = devMode flag
- `Tools` = `[]DashboardTool` from catalogFn()
- `ToolCount` = len(Tools)
- `MCPEndpoint` = `http://localhost:{port}/mcp`
- `ClaudeCodeConfig` = formatted JSON string for Claude Code MCP config
- `ClaudeDesktopConfig` = formatted JSON string for Claude Desktop MCP config
- `Port` = server port
- `HasEODHD`, `HasNavexa`, `HasGemini` = booleans for key status
- `Portfolios` = comma-joined portfolio names or empty

### Step 3: Create dashboard template

**File: `pages/dashboard.html`** (new)

Structure:
```
<!DOCTYPE html>
<html>
<head>{{template "head.html" .}}<title>VIRE -- DASHBOARD</title></head>
<body>
  {{template "nav.html" .}}
  <main class="dashboard">
    <div class="dashboard-inner">
      <h1 class="dashboard-title">DASHBOARD</h1>

      [01] MCP CONNECTION section
        - Endpoint URL in a code block
        - Claude Code JSON config in a <pre> block
        - Claude Desktop JSON config in a <pre> block

      [02] AVAILABLE TOOLS section
        - Tool count header
        - Table of tool name | description | method | path
        - Or "[NO TOOLS REGISTERED]" if empty

      [03] CONFIG STATUS section
        - Grid showing key status: EODHD [OK]/[--], Navexa [OK]/[--], Gemini [OK]/[--]
        - Portfolios display
        - Port display
    </div>
  </main>
  {{template "footer.html" .}}
</body>
</html>
```

### Step 4: Add dashboard CSS to portal.css

**File: `pages/static/css/portal.css`**
- `.dashboard` layout (padding, max-width)
- `.dashboard-inner` container
- `.dashboard-title` header
- `.dashboard-section` bordered sections with 2px solid #000
- `.dashboard-section-title` for `[01] SECTION NAME` headers (bold, letterspacing)
- `.code-block` for pre-formatted config snippets (border, padding, overflow-x auto, white-space pre)
- `.tool-table` for tools listing
- `.config-grid` for key status
- `.status-ok` / `.status-missing` for indicators

All monochrome (#000, #fff, #888), no border-radius, no box-shadow.

### Step 5: Wire up in App and Routes

**File: `internal/app/app.go`**
- Add `DashboardHandler *handlers.DashboardHandler` to App struct
- In `initHandlers()`, create a `catalogFn` adapter that converts `mcp.CatalogTool` -> `handlers.DashboardTool`
- Instantiate DashboardHandler with config.Server.Port and the adapter function

**File: `internal/server/routes.go`**
- Add route: `mux.HandleFunc("GET /dashboard", s.app.DashboardHandler.ServeHTTP)`

### Step 6: Fix README.md port references

**File: `README.md`**

Changes to `4241`:
- Line 16: `**Port 4241**` (remove "required by Cloud Run" -- that's the override)
- Line 60: `http://localhost:4241`
- Line 100: Config table default `4241`
- Line 132: `vire-portal (:4241)`
- Line 154: `http://localhost:4241/mcp`
- Line 803: `port 4241`
- Line 806: `http://localhost:4241/mcp`
- Line 847: `-p 4241:4241`
- Line 922: `vire-portal (:4241)`

Keep at `8080` (Cloud Run / Terraform / Docker internals):
- Line 642: `EXPOSE 8080` (Dockerfile convention, Cloud Run override)
- Line 874: `container_port = 8080` (Terraform)
- Line 876: `VIRE_SERVER_PORT = "8080"` (Terraform env)
- Line 889: `Port: 8080` (Cloud Run properties)

Also add dashboard to the Routes table.

## Type/Import Strategy

The `handlers` package cannot import `mcp` (would create coupling). Solution:
- Define `DashboardTool` in `handlers/dashboard.go` with just display fields
- `app.go` imports both `mcp` and `handlers`, creates an adapter function that maps `[]mcp.CatalogTool` -> `[]handlers.DashboardTool`
- DashboardHandler stores `catalogFn func() []DashboardTool`

## Files Changed

| File | Action |
|------|--------|
| `internal/mcp/handler.go` | Modify: add catalog field + Catalog() getter |
| `internal/handlers/dashboard.go` | New: DashboardHandler + DashboardTool struct |
| `internal/handlers/landing.go` | Modify: export FindPagesDir |
| `pages/dashboard.html` | New: dashboard template |
| `pages/static/css/portal.css` | Modify: add dashboard styles |
| `internal/app/app.go` | Modify: wire DashboardHandler |
| `internal/server/routes.go` | Modify: add /dashboard route |
| `README.md` | Modify: fix port 8080 -> 4241, add dashboard route |
