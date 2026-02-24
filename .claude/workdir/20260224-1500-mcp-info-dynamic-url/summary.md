# Summary: Dynamic MCP endpoint URL on /mcp-info page

**Date:** 2026-02-24
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/handlers/mcp_page.go` | Added `baseURL` field, `SetBaseURL()` method; endpoint now uses `baseURL + "/mcp"` with localhost fallback |
| `pages/mcp.html` | JSON config example uses `{{.MCPEndpoint}}` instead of hardcoded `http://localhost:{{.Port}}/mcp` |
| `internal/app/app.go` | Calls `SetBaseURL(a.Config.BaseURL())` after handler construction |
| `internal/handlers/handlers_test.go` | Updated existing port test, added tests for deployed URLs (https) and empty baseURL fallback |
| `tests/ui/mcp_test.go` | Added `TestMCPEndpointURL` to verify dynamic endpoint renders in the page |
| `scripts/ui-test.sh` | Added `mcp` case alias mapping to `^TestMCP` pattern |

## Tests
- 4 unit tests updated/added in `handlers_test.go` (deployed URL, fallback, existing port tests)
- 1 UI test added in `mcp_test.go` (`TestMCPEndpointURL`)
- All 5 MCP UI tests pass (17.3s)
- `go test ./...` passes
- `go vet ./...` clean

## Documentation Updated
- No user-facing docs needed — behaviour change is transparent

## Devils-Advocate Findings
- No security issues found — `baseURL` comes from server config (not user input), and Go's `html/template` auto-escapes values in HTML context
- Trailing slash edge case handled by `config.BaseURL()` which calls `strings.TrimRight(url, "/")`

## Notes
- `config.BaseURL()` uses `VIRE_PORTAL_URL` env var when set, falls back to `http://{host}:{port}`
- Deployed environments (fly.dev) set `VIRE_PORTAL_URL`, so the fix works automatically
- Resolves feedback item fb_b36f8821
