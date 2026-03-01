# MCP Tool Catalog Auto-Refresh — Implementation Spec

## Problem

Portal caches MCP tool catalog at startup and never refreshes. New tools added to vire-server are invisible to MCP clients until portal restart (fb_98b90af1).

## Design

**Version-change detection with `SetTools()` atomic replacement.**

Poll vire-server `/api/version` every 30s. When `build` field changes, fetch new catalog, validate, and use `MCPServer.SetTools()` to atomically replace all tools. mcp-go automatically sends `notifications/tools/list_changed` to connected sessions. `Handler.catalog` updated under `sync.RWMutex` for MCP page display.

## File Changes

### 1. `internal/mcp/handler.go` — Primary changes

**a) Add imports:** `"sync"` (context already imported)

**b) Add fields to Handler struct:**

```go
type Handler struct {
    streamable *mcpserver.StreamableHTTPServer
    logger     *common.Logger
    catalog    []CatalogTool
    jwtSecret  []byte
    mcpSrv     *mcpserver.MCPServer // for SetTools() during refresh
    proxy      *MCPProxy            // for FetchCatalog() during refresh
    catalogMu  sync.RWMutex         // protects catalog field
    stopWatch  chan struct{}         // closed to stop version watcher
}
```

**c) Modify `NewHandler()`:** Store `mcpSrv` and `proxy` in handler, init `stopWatch`, start `go h.watchServerVersion()`.

```go
h := &Handler{
    streamable: streamable,
    logger:     logger,
    catalog:    validated,
    jwtSecret:  []byte(cfg.Auth.JWTSecret),
    mcpSrv:     mcpSrv,
    proxy:      proxy,
    stopWatch:  make(chan struct{}),
}
go h.watchServerVersion()
return h
```

**d) Update `Catalog()` to use RWMutex:**

```go
func (h *Handler) Catalog() []CatalogTool {
    h.catalogMu.RLock()
    defer h.catalogMu.RUnlock()
    result := make([]CatalogTool, len(h.catalog))
    copy(result, h.catalog)
    return result
}
```

**e) Add constant:**

```go
const versionPollInterval = 30 * time.Second
```

**f) Add `RefreshCatalog()` method:**

```go
func (h *Handler) RefreshCatalog() (int, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    catalog, err := h.proxy.FetchCatalog(ctx)
    if err != nil {
        return 0, fmt.Errorf("fetch catalog: %w", err)
    }

    validated := ValidateCatalog(catalog, h.logger)

    tools := make([]mcpserver.ServerTool, 0, len(validated)+1)
    for _, ct := range validated {
        tools = append(tools, mcpserver.ServerTool{
            Tool:    BuildMCPTool(ct),
            Handler: GenericToolHandler(h.proxy, ct),
        })
    }
    // Always include combined version tool
    tools = append(tools, mcpserver.ServerTool{
        Tool:    VersionTool(),
        Handler: VersionToolHandler(h.proxy),
    })

    h.mcpSrv.SetTools(tools...)

    h.catalogMu.Lock()
    h.catalog = validated
    h.catalogMu.Unlock()

    return len(validated), nil
}
```

**g) Add `watchServerVersion()` method:**

```go
func (h *Handler) watchServerVersion() {
    lastBuild := h.fetchServerBuild()

    ticker := time.NewTicker(versionPollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-h.stopWatch:
            return
        case <-ticker.C:
            build := h.fetchServerBuild()
            if build == "" {
                continue
            }
            if lastBuild == "" {
                lastBuild = build
                h.triggerRefresh(build)
                continue
            }
            if build != lastBuild {
                h.logger.Info().
                    Str("old_build", lastBuild).
                    Str("new_build", build).
                    Msg("server build changed, refreshing tool catalog")
                lastBuild = build
                h.triggerRefresh(build)
            }
        }
    }
}
```

**h) Add `fetchServerBuild()` method:**

```go
func (h *Handler) fetchServerBuild() string {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    body, err := h.proxy.get(ctx, "/api/version")
    if err != nil {
        return ""
    }

    var resp struct {
        Build string `json:"build"`
    }
    if json.Unmarshal(body, &resp) != nil {
        return ""
    }
    return resp.Build
}
```

**i) Add `triggerRefresh()` method:**

```go
func (h *Handler) triggerRefresh(build string) {
    count, err := h.RefreshCatalog()
    if err != nil {
        h.logger.Warn().
            Str("build", build).
            Str("error", err.Error()).
            Msg("catalog refresh failed")
        return
    }
    h.logger.Info().
        Int("tools", count).
        Str("build", build).
        Msg("catalog refreshed")
}
```

**j) Add `Close()` method:**

```go
func (h *Handler) Close() {
    select {
    case <-h.stopWatch:
    default:
        close(h.stopWatch)
    }
}
```

### 2. `internal/app/app.go` — Wire Close()

Update `Close()`:

```go
func (a *App) Close() error {
    if a.MCPHandler != nil {
        a.MCPHandler.Close()
    }
    return nil
}
```

### 3. No other file changes

- `catalog.go`, `proxy.go`, `version.go`, `tools.go`, `config.go`, `main.go` — unchanged

## Test Cases

All tests in `internal/mcp/handler_test.go` (same package, access unexported fields).

### Test 1: `TestRefreshCatalog_UpdatesTools`
- Mock server returns 1 tool initially, 2 tools on second call
- Call `NewHandler()`, verify 1 catalog tool
- Call `RefreshCatalog()`, verify returns count=2
- Verify `Catalog()` returns 2 tools
- Verify `mcpSrv.ListTools()` contains tool_a, tool_b, get_version

### Test 2: `TestRefreshCatalog_ServerUnreachable`
- Create handler with working server, then close server
- Call `RefreshCatalog()`, verify error returned
- Verify original catalog preserved

### Test 3: `TestRefreshCatalog_VersionToolAlwaysPresent`
- Catalog without get_version
- After refresh, verify `mcpSrv.GetTool("get_version")` is non-nil

### Test 4: `TestWatchServerVersion_DetectsChange`
- Mock server with controllable build + catalog
- Call `triggerRefresh()` directly after changing mock state
- Verify catalog updated

### Test 5: `TestFetchServerBuild_Success`
- Mock version endpoint, verify correct build string returned

### Test 6: `TestFetchServerBuild_Unreachable`
- Use mockAPIServer (503), verify empty string returned

### Test 7: `TestHandlerClose_StopsWatcher`
- Call `Close()` twice, verify no panic, verify channel closed

### Test 8: `TestRefreshCatalog_ConcurrentAccess`
- 10 concurrent `Catalog()` reads + `RefreshCatalog()` writes
- Run with `-race` to detect data races

## Edge Cases

1. **Server unreachable at startup → comes up later**: `lastBuild=""`, first successful fetch triggers refresh
2. **Server goes down after startup**: `fetchServerBuild()` returns "", watcher skips, existing catalog preserved
3. **Catalog validation rejects all tools**: `SetTools()` called with only `get_version`, correct behavior
4. **Double Close**: Safe via select/default pattern
5. **get_version override**: Always appended last to tools list, wins in map
6. **Concurrent catalog access**: Protected by `catalogMu` RWMutex

## Constants

| Constant | Value | Rationale |
|---|---|---|
| `versionPollInterval` | 30s | Fast enough for deploy detection, light on server |
| Version fetch timeout | 5s | Lightweight endpoint |
| Catalog fetch timeout | 10s | Matches startup timeout |
