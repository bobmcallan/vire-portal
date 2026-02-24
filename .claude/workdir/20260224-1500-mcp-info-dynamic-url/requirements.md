# Requirements: Dynamic MCP endpoint URL on /mcp-info page

**Date:** 2026-02-24
**Requested:** Fix hardcoded `http://localhost:8080/mcp` on the `/mcp-info` page so it derives the URL dynamically from `config.BaseURL()`.
**Feedback:** fb_b36f8821 — dismissed by vire-server as a portal issue.

## Scope
- In scope: Make the MCP endpoint URL on `/mcp-info` dynamic based on `config.BaseURL()`
- In scope: Update the JSON config example in the template to use the dynamic URL
- In scope: Update tests to verify dynamic URL behaviour
- Out of scope: Changes to vire-server, config schema changes

## Approach

The portal already has `config.BaseURL()` which returns:
- `Auth.PortalURL` if set (e.g. `https://vire-pprod-portal.fly.dev`), or
- `http://{host}:{port}` built from server config (falls back to `localhost` for `0.0.0.0`)

The `MCPPageHandler` already follows the setter pattern (`SetAPIURL`). The fix:

1. **`internal/handlers/mcp_page.go`**: Add `baseURL` field + `SetBaseURL(url string)` method. In `ServeHTTP`, use `h.baseURL + "/mcp"` instead of `fmt.Sprintf("http://localhost:%d/mcp", h.port)`.
2. **`pages/mcp.html`**: Replace hardcoded `http://localhost:{{.Port}}/mcp` with `{{.MCPEndpoint}}` in the JSON example.
3. **`internal/app/app.go`**: Call `a.MCPPageHandler.SetBaseURL(a.Config.BaseURL())` after construction.
4. **`internal/handlers/handlers_test.go`**: Update `TestMCPPageHandler_PortInMCPEndpoint` to test with a custom base URL and add a test for deployed URLs (https).

## Files Expected to Change
- `internal/handlers/mcp_page.go` (add baseURL field, setter, use in ServeHTTP)
- `pages/mcp.html` (use {{.MCPEndpoint}} in JSON example)
- `internal/app/app.go` (call SetBaseURL)
- `internal/handlers/handlers_test.go` (update/add tests)
