# Requirements: OAuth Follow-up Tasks

**Date:** 2026-02-26
**Requested:** Execute remaining tasks from oauth-implementation-followup.md after RFC 9728 fix completion

## Scope

### In Scope
- Run full test suite and verify all tests pass
- Verify 401 response and OAuth discovery chain locally
- Commit and push changes
- Prepare for deployment to pprod

### Out of Scope
- Additional code changes (implementation already complete)
- Changes to vire-server

## Approach

1. Run `go test ./...` and `go vet ./...`
2. Start server and verify 401 + WWW-Authenticate response
3. Verify OAuth discovery chain works
4. Commit with conventional commit message
5. Push to origin

## Files Changed (Already Implemented)

| File | Change |
|------|--------|
| `internal/auth/discovery.go` | Added `bearer_methods_supported` field |
| `internal/mcp/handler.go` | Added 401 response with WWW-Authenticate header |
| `internal/auth/discovery_test.go` | Updated test for new field |
| `internal/mcp/handler_test.go` | Added 10 new tests for 401 scenarios |
| `internal/mcp/handler_stress_test.go` | Added stress tests for security/edge cases |
| `internal/mcp/mcp_test.go` | Fixed integration test auth |
| `internal/server/routes_test.go` | Added test for 401 with WWW-Authenticate |

## Acceptance Criteria

1. All tests pass
2. 401 response verified locally
3. OAuth discovery chain verified
4. Changes committed and pushed
