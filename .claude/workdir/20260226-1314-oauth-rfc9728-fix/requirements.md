# Requirements: OAuth RFC 9728 Compliance Fix

**Date:** 2026-02-26
**Requested:** Fix OAuth flow not triggering for Claude Desktop and ChatGPT when adding MCP server

## Scope

### In Scope
- Add 401 Unauthorized response to `/mcp` endpoint when no valid authentication
- Add `WWW-Authenticate` header with `resource_metadata` parameter per RFC 9728
- Add `bearer_methods_supported` to protected resource metadata
- Write tests for the new authentication requirement
- Update documentation

### Out of Scope
- Changes to vire-server OAuth implementation
- Changes to dev mode (`/mcp/{encrypted_uid}`) endpoint
- Changes to OAuth flow endpoints (authorize, token, register)

## Approach

The MCP handler at `/mcp` currently extracts user context but never enforces authentication. Per RFC 9728 and MCP specification, when an unauthenticated request is made to a protected resource, the server MUST return 401 with a `WWW-Authenticate` header containing the protected resource metadata URL.

**Implementation Strategy:**
1. Modify `ServeHTTP` in `internal/mcp/handler.go` to check for valid user context
2. Return 401 with proper WWW-Authenticate header when authentication missing
3. Update `internal/auth/discovery.go` to include `bearer_methods_supported`
4. Write unit tests for the new authentication requirement

## Files Expected to Change

| File | Change |
|------|--------|
| `internal/mcp/handler.go` | Add 401 + WWW-Authenticate response |
| `internal/auth/discovery.go` | Add `bearer_methods_supported` field |
| `internal/mcp/handler_test.go` | Add tests for auth requirement |

## Root Cause

See detailed analysis in: `docs/authentication/oauth-troubleshooting-findings.md`

## Acceptance Criteria

1. Unauthenticated requests to `/mcp` return 401 with WWW-Authenticate header
2. WWW-Authenticate header contains `resource_metadata` URL
3. Protected resource metadata includes `bearer_methods_supported`
4. All existing tests pass
5. New tests cover the authentication requirement
