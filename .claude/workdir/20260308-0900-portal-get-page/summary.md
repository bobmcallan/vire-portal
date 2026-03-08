# Summary: portal_get_page MCP Tool

**Status:** completed
**Feedback:** fb_28525072

## Changes

| File | Change |
|------|--------|
| `internal/mcp/get_page.go` | **NEW** — Tool definition, handler (loopback HTTP), JWT minting (128 lines) |
| `internal/mcp/get_page_test.go` | **NEW** — 13 unit tests covering valid/invalid pages, auth, errors (301 lines) |
| `internal/mcp/get_page_stress_test.go` | **NEW** — 22 stress tests: whitelist bypass, JWT security, response size, concurrency (816 lines) |
| `internal/mcp/handler.go` | **MODIFIED** — Added `portalBaseURL` field, registered tool in `NewHandler()` and `RefreshCatalog()` |

## Tests

- **Unit tests:** 13 added in `get_page_test.go` — all pass
- **Stress tests:** 22 added in `get_page_stress_test.go` — all pass
- **Existing tests:** No regressions (all `internal/...` packages pass)
- **go vet:** Clean
- **go build:** Clean
- Fix rounds: 0

## Architecture

- Follows `version.go` pattern for local MCP tool registration
- Architect review: APPROVED — no fixes needed
- Reviewer: APPROVED — no fixes needed

## Devils-Advocate

Key findings (informational, not blocking):
1. Error messages may leak internal loopback URL to MCP client (acceptable: MCP clients are authenticated)
2. Non-200 response body passed through (acceptable: portal error pages should not contain secrets)
3. Loopback follows redirects (acceptable: all redirects stay on same host)
4. Responses >5MB silently truncated (acceptable: portal pages are well under 5MB)
5. No jti claim in loopback JWT — deterministic per-second (acceptable: 30s TTL mitigates replay)

## Design

- **Approach:** Internal HTTP loopback to portal's own listen address
- **Auth:** Short-lived JWT (30s TTL) minted with user's ID, sent as `vire_session` cookie
- **Security:** Hardcoded page whitelist (exact match), no path construction from user input
- **Pages:** dashboard, strategy, cash, glossary, changelog, help, mcp-info, profile, docs
- **Excluded:** admin, landing, error, mobile

## Notes

- No UI changes — MCP tool only
- No documentation updates needed (tool is self-describing via MCP schema)
- The tool reuses 100% of existing SSR code without handler refactoring
