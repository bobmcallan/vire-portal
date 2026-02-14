# Plan: Dynamic Tool Registration + Docker Restructure

## Overview

Three changes: (1) Replace 26 hardcoded MCP tools with dynamic registration from `GET /api/mcp/tools`, (2) Rename `portal.toml` to `vire-portal.toml`, (3) Move `Dockerfile` to `docker/Dockerfile`.

---

## Change 1: Dynamic Tool Registration

### New file: `internal/mcp/catalog.go`

Define catalog types that match the vire-server JSON schema:

```go
// CatalogTool represents one tool entry from GET /api/mcp/tools.
type CatalogTool struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Method      string         `json:"method"`
    Path        string         `json:"path"`
    Params      []CatalogParam `json:"params"`
}

// CatalogParam describes one parameter for a catalog tool.
type CatalogParam struct {
    Name        string `json:"name"`
    Type        string `json:"type"`        // string, number, boolean, array, object
    Description string `json:"description"`
    Required    bool   `json:"required"`
    In          string `json:"in"`           // path, query, body
    DefaultFrom string `json:"default_from"` // e.g. "user_config.default_portfolio"
}
```

Add a `FetchCatalog` function on `MCPProxy`:

```go
// FetchCatalog fetches the tool catalog from vire-server.
// Returns nil, nil if the server is unreachable (non-fatal at startup).
func (p *MCPProxy) FetchCatalog(ctx context.Context) ([]CatalogTool, error) {
    body, err := p.get(ctx, "/api/mcp/tools")
    if err != nil {
        return nil, err
    }
    var tools []CatalogTool
    if err := json.Unmarshal(body, &tools); err != nil {
        return nil, fmt.Errorf("failed to parse tool catalog: %w", err)
    }
    return tools, nil
}
```

Add a `BuildMCPTool` function that converts a `CatalogTool` into an `mcp.Tool`:

```go
func BuildMCPTool(ct CatalogTool) mcp.Tool {
    opts := []mcp.ToolOption{mcp.WithDescription(ct.Description)}
    for _, p := range ct.Params {
        if p.In == "path" || p.In == "query" || p.In == "body" {
            opt := buildParamOption(p)
            opts = append(opts, opt)
        }
    }
    return mcp.NewTool(ct.Name, opts...)
}
```

The `buildParamOption` helper maps `type` to the appropriate mcp-go option:
- `string` -> `mcp.WithString(name, ...)`
- `number` -> `mcp.WithNumber(name, ...)`
- `boolean` -> `mcp.WithBoolean(name, ...)`
- `array` -> `mcp.WithArray(name, mcp.WithStringItems(), ...)` (arrays are string items per existing patterns)
- `object` -> `mcp.WithString(name, ...)` (passed as JSON string, consistent with existing `strategy_json` / `plan_json` pattern)

Each param option includes `mcp.Description(p.Description)` and, if `p.Required`, `mcp.Required()`.

### Generic handler: `internal/mcp/catalog.go`

Add a `GenericToolHandler` that builds a handler from a `CatalogTool`:

```go
func GenericToolHandler(p *MCPProxy, ct CatalogTool) server.ToolHandlerFunc {
    return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // 1. Resolve path params: substitute {param} in ct.Path
        path := ct.Path
        bodyParams := map[string]interface{}{}
        queryParams := url.Values{}

        for _, param := range ct.Params {
            val := resolveParamValue(ctx, p, r, param)
            switch param.In {
            case "path":
                if val == nil || val == "" {
                    if param.Required {
                        return errorResult(fmt.Sprintf("Error: %s parameter is required", param.Name)), nil
                    }
                    continue
                }
                path = strings.ReplaceAll(path, "{"+param.Name+"}", url.PathEscape(fmt.Sprint(val)))
            case "query":
                if val != nil && val != "" {
                    queryParams.Set(param.Name, fmt.Sprint(val))
                }
            case "body":
                if val != nil {
                    bodyParams[param.Name] = val
                }
            }
        }

        if len(queryParams) > 0 {
            path += "?" + queryParams.Encode()
        }

        // 2. Execute HTTP request based on method
        var respBody []byte
        var err error
        switch strings.ToUpper(ct.Method) {
        case "GET":
            respBody, err = p.get(ctx, path)
        case "POST":
            respBody, err = p.post(ctx, path, bodyOrNil(bodyParams))
        case "PUT":
            respBody, err = p.put(ctx, path, bodyOrNil(bodyParams))
        case "PATCH":
            respBody, err = p.patch(ctx, path, bodyOrNil(bodyParams))
        case "DELETE":
            respBody, err = p.del(ctx, path)
        default:
            return errorResult(fmt.Sprintf("Error: unsupported method %s", ct.Method)), nil
        }

        if err != nil {
            return errorResult(fmt.Sprintf("Error: %v", err)), nil
        }
        return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(respBody))}}, nil
    }
}
```

The `resolveParamValue` helper:
1. Gets the value from `r.GetString` / `r.GetInt` / etc. based on param type
2. If empty and `param.DefaultFrom` is set, resolves from config:
   - `user_config.default_portfolio` -> first item from `p.UserHeaders().Get("X-Vire-Portfolios")`
   - Could be extended for other defaults later
3. If still empty and no default, returns nil

### Modify: `internal/mcp/tools.go`

- Remove `const ExpectedToolCount = 26`
- Remove `RegisterTools` function (all 26 hardcoded registrations)
- Remove all `create*Tool()` functions (lines 369-571)
- Remove hardcoded handler factories: `proxyGet`, `tickerGetHandler`, `portfolioGetHandler`, `portfolioPostHandler`, `portfolioDeleteHandler`, `bodyPostHandler`
- Keep the file but make it just contain `RegisterToolsFromCatalog`:

```go
// RegisterToolsFromCatalog registers MCP tools dynamically from catalog entries.
func RegisterToolsFromCatalog(s *server.MCPServer, p *MCPProxy, catalog []CatalogTool) int {
    for _, ct := range catalog {
        tool := BuildMCPTool(ct)
        handler := GenericToolHandler(p, ct)
        s.AddTool(tool, handler)
    }
    return len(catalog)
}
```

### Modify: `internal/mcp/handler.go`

Change `NewHandler` to:
1. Create the MCPProxy
2. Call `proxy.FetchCatalog(ctx)` with a timeout context (e.g., 10 seconds)
3. If catalog fetch fails, log a warning and continue with 0 tools (don't crash)
4. Call `RegisterToolsFromCatalog(mcpSrv, proxy, catalog)`
5. Log "MCP handler initialized" with the actual tool count

```go
func NewHandler(cfg *config.Config, logger *slog.Logger) *Handler {
    mcpSrv := mcpserver.NewMCPServer("vire-portal", "1.0.0",
        mcpserver.WithToolCapabilities(true),
    )

    proxy := NewMCPProxy(cfg.API.URL, logger, cfg)

    // Fetch tool catalog from vire-server (non-fatal if unreachable)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var toolCount int
    catalog, err := proxy.FetchCatalog(ctx)
    if err != nil {
        logger.Warn("failed to fetch tool catalog from vire-server, starting with 0 tools",
            "error", err, "api_url", cfg.API.URL)
    } else {
        toolCount = RegisterToolsFromCatalog(mcpSrv, proxy, catalog)
    }

    streamable := mcpserver.NewStreamableHTTPServer(mcpSrv,
        mcpserver.WithStateLess(true),
    )

    logger.Info("MCP handler initialized",
        "tools", toolCount,
        "api_url", cfg.API.URL,
    )

    return &Handler{streamable: streamable, logger: logger}
}
```

### Modify: `internal/mcp/handlers.go`

- Keep `errorResult` (still used by GenericToolHandler)
- Keep `resolvePortfolio` renamed/refactored into a simpler `resolveDefaultPortfolio` that is called by `resolveParamValue` when `default_from` is `user_config.default_portfolio`

### Modify: `internal/mcp/mcp_test.go`

The tests need significant rewrite since they currently test hardcoded tools:

- **Remove** `TestRegisterTools_AllToolsRegistered` (no more `ExpectedToolCount`)
- **Remove** `TestRegisterTools_ExpectedToolNames` (tools come from catalog now)
- **Remove** `TestRegisterTools_ToolsHaveDescriptions` (descriptions come from catalog)
- **Keep** all proxy HTTP tests (they test MCPProxy, which is unchanged)
- **Keep** all error handling tests
- **Keep** resolvePortfolio tests (the logic still exists)
- **Add** new tests:
  - `TestFetchCatalog_ParsesJSON` - mock server returns catalog JSON, verify parsing
  - `TestFetchCatalog_EmptyArray` - returns empty catalog gracefully
  - `TestFetchCatalog_ServerDown` - returns error, no crash
  - `TestBuildMCPTool_StringParam` - verify mcp.Tool has correct schema
  - `TestBuildMCPTool_RequiredParam` - verify required flag
  - `TestBuildMCPTool_ArrayParam` - verify array type
  - `TestBuildMCPTool_NoParams` - tool with no params
  - `TestGenericHandler_GET_PathParam` - substitutes `{ticker}` in path
  - `TestGenericHandler_POST_BodyParams` - builds JSON body from body params
  - `TestGenericHandler_QueryParams` - appends query params to URL
  - `TestGenericHandler_DefaultFrom` - resolves default from config
  - `TestGenericHandler_MissingRequiredParam` - returns error
  - `TestGenericHandler_PUT_Method` - correct HTTP method used
  - `TestGenericHandler_DELETE_Method` - correct HTTP method used
  - `TestRegisterToolsFromCatalog_Count` - register N tools, verify N registered
  - `TestNewHandler_CatalogUnavailable` - handler still created with 0 tools
  - Integration test with mock catalog mimicking the real tool structure

---

## Change 2: Rename `portal.toml` to `vire-portal.toml`

Files to update:

| File | Change |
|------|--------|
| `docker/portal.toml` | Rename to `docker/vire-portal.toml` |
| `cmd/portal/main.go` | Auto-discovery list: `"portal.toml"` -> `"vire-portal.toml"`, `"docker/portal.toml"` -> `"docker/vire-portal.toml"` |
| `Dockerfile` (before move) / `docker/Dockerfile` (after move) | `COPY --from=builder /build/docker/portal.toml .` -> `COPY --from=builder /build/docker/vire-portal.toml .` |
| `scripts/deploy.sh` | Line 49: `"$PROJECT_DIR/docker/portal.toml"` -> `"$PROJECT_DIR/docker/vire-portal.toml"` |
| `scripts/test-scripts.sh` | Line 40: `"docker/portal.toml"` -> `"docker/vire-portal.toml"` and line 611: `portal.toml` -> `vire-portal.toml` |
| `README.md` | All references to `portal.toml` and `docker/portal.toml` |
| `docker/README.md` | References to `portal.toml` |
| `.dockerignore` | Comment: `# Docker artifacts (keep portal.toml for COPY)` -> `# Docker artifacts (keep vire-portal.toml for COPY)` |

---

## Change 3: Move Dockerfile to `docker/Dockerfile`

Files to update:

| File | Change |
|------|--------|
| `Dockerfile` | Move to `docker/Dockerfile` |
| `docker/docker-compose.yml` | `dockerfile: Dockerfile` -> `dockerfile: docker/Dockerfile` |
| `scripts/build.sh` | Add `-f docker/Dockerfile` to `docker build` commands |
| `scripts/deploy.sh` | Line 50: `"$PROJECT_DIR/Dockerfile"` -> `"$PROJECT_DIR/docker/Dockerfile"` |
| `scripts/test-scripts.sh` | Line 42: `"Dockerfile"` -> `"docker/Dockerfile"`, Section 9 DOCKERFILE path, update all Dockerfile references |
| `.github/workflows/release.yml` | `file: Dockerfile` -> `file: docker/Dockerfile` |
| `README.md` | All references to root `Dockerfile` |
| `.dockerignore` | No change needed (Dockerfile location doesn't affect .dockerignore) |

### Docker build context consideration

The `docker/docker-compose.yml` already has `context: ..` which means the build context is the project root. Changing `dockerfile: Dockerfile` to `dockerfile: docker/Dockerfile` will correctly find the Dockerfile relative to the project root context.

For `scripts/build.sh`, the `docker build` commands run from `$PROJECT_ROOT`, so adding `-f docker/Dockerfile` and keeping `.` as context works correctly.

For `release.yml`, changing `file: Dockerfile` to `file: docker/Dockerfile` works since `context: .` (project root) is the default.

---

## Execution Order

1. **Rename** `docker/portal.toml` -> `docker/vire-portal.toml`
2. **Move** `Dockerfile` -> `docker/Dockerfile`
3. **Update all references** for both renames across scripts, compose, CI, docs
4. **Create** `internal/mcp/catalog.go` with types + FetchCatalog + BuildMCPTool + GenericToolHandler
5. **Rewrite** `internal/mcp/tools.go` to `RegisterToolsFromCatalog` (remove all hardcoded tools)
6. **Update** `internal/mcp/handler.go` to fetch catalog at startup
7. **Simplify** `internal/mcp/handlers.go` (keep errorResult, adapt resolvePortfolio)
8. **Rewrite** `internal/mcp/mcp_test.go` for new dynamic architecture
9. **Update** `scripts/test-scripts.sh` for new file locations and names
10. **Update** `README.md` and `docker/README.md`

---

## Key Design Decisions

1. **Non-fatal startup**: If vire-server is unreachable, portal starts with 0 tools. This matches Docker Compose where portal may start before vire-server is healthy.

2. **10-second timeout for catalog fetch**: Generous enough for slow startup, but won't block forever.

3. **No catalog refresh timer**: Startup-only fetch as specified. Can be added later.

4. **`default_from` resolution**: Only `user_config.default_portfolio` is supported initially (maps to first portfolio from config). The string is dot-path notation that can be extended.

5. **Array params use string items**: Consistent with existing mcp-go usage where arrays are `[]string` (e.g., `focus_signals`, `tickers`).

6. **Object params passed as JSON string**: Consistent with existing patterns (`strategy_json`, `plan_json`) where the user passes a JSON string that gets parsed.

7. **Generic handler returns raw JSON**: Same as current implementation -- no response formatting.

8. **URL path escaping**: `url.PathEscape` on all path param substitutions, consistent with current code.

---

## Risk Assessment

- **Breaking change**: Tool names and schemas now come from vire-server. If vire-server's catalog differs from the current 26 tools, behavior changes. This is intentional -- the portal should match what the server provides.
- **Testing**: Cannot test against real vire-server catalog in unit tests. All tests use mock catalog data. The mock data should mirror the real catalog structure.
- **Startup dependency**: Portal depends on vire-server for tool catalog. Mitigated by non-fatal startup and Docker Compose `depends_on: service_healthy`.
