# Summary: MCP Tool Catalog Auto-Refresh

**Status:** completed
**Feedback:** fb_98b90af1

## Changes
| File | Change |
|------|--------|
| `internal/mcp/handler.go` | Added `mcpSrv`, `proxy`, `catalogMu`, `stopWatch` fields to Handler. Added `RefreshCatalog()`, `watchServerVersion()`, `fetchServerBuild()`, `triggerRefresh()`, `Close()` methods. Updated `Catalog()` with RWMutex. Background goroutine polls server version every 30s and refreshes catalog on change. |
| `internal/app/app.go` | Updated `Close()` to call `MCPHandler.Close()` for graceful goroutine shutdown. |

## Tests
- 8 new unit tests added to `internal/mcp/handler_test.go`
- All MCP tests pass with `-race` flag
- `go vet` clean
- Full test suite passing

## Architecture
- Architect review: PASS — follows existing goroutine patterns (seed.DevUsers, seed.RegisterService)
- Uses mcp-go `SetTools()` for atomic tool replacement with automatic `tools/list_changed` notification

## Devils-Advocate
- 27 tests pass, no bugs found
- Concurrency safe (RWMutex + MCPServer internal mutex)
- No goroutine leaks (Close() stops watcher)
- Double-Close safe

## Design
- Polls `/api/version` every 30s, compares `build` field
- On change: fetch catalog, validate, `SetTools()` atomic replace
- Connected MCP clients (Claude CLI/Desktop) auto-notified via `notifications/tools/list_changed`
- MCP info page (`/mcp-info`) reflects new tools via `catalogAdapter` closure
- Handles server-unavailable-at-startup → refreshes when server comes up

## Notes
- No config changes needed (30s interval as constant)
- No UI changes
- No new dependencies
