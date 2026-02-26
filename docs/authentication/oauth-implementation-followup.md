# OAuth MCP Integration: Implementation Follow-up

**Date:** 2026-02-26
**Status:** In Progress
**Related:** `../.claude/workdir/20260226-1314-oauth-rfc9728-fix/`

## Summary

This document tracks the remaining work to complete OAuth integration for MCP clients (Claude Desktop, ChatGPT).

## Current Progress

The RFC 9728 compliance fix is being implemented in `.claude/workdir/20260226-1314-oauth-rfc9728-fix/`:

- [x] Add 401 + WWW-Authenticate header to `/mcp` endpoint
- [x] Add `bearer_methods_supported` to protected resource metadata
- [x] Write tests for authentication requirement

## Remaining Tasks

### 1. Merge and Deploy

After the RFC 9728 fix is complete:

1. **Run full test suite**
   ```bash
   cd /home/bobmc/development/vire-portal
   go test ./... -v
   ```

2. **Verify locally**
   ```bash
   # Start the portal
   go run ./cmd/vire-portal

   # Test 401 response
   curl -v http://localhost:8883/mcp

   # Should see:
   # HTTP/1.1 401 Unauthorized
   # Www-Authenticate: Bearer resource_metadata="http://localhost:8883/.well-known/oauth-protected-resource"
   ```

3. **Test OAuth discovery chain**
   ```bash
   curl http://localhost:8883/.well-known/oauth-protected-resource | jq
   curl http://localhost:8883/.well-known/oauth-authorization-server | jq
   ```

4. **Commit and push**
   ```bash
   git add .
   git commit -m "feat(auth): add RFC 9728 compliant 401 response for MCP endpoint"
   git push
   ```

5. **Deploy to pprod**
   ```bash
   # Via Fly.io deployment
   fly deploy --app vire-pprod-portal
   ```

### 2. Production Verification

After deployment:

```bash
# Test 401 response
curl -v https://vire-pprod-portal.fly.dev/mcp

# Test discovery endpoints
curl https://vire-pprod-portal.fly.dev/.well-known/oauth-protected-resource | jq
curl https://vire-pprod-portal.fly.dev/.well-known/oauth-authorization-server | jq
```

**Expected 401 response:**
```
HTTP/1.1 401 Unauthorized
Www-Authenticate: Bearer resource_metadata="https://vire-pprod-portal.fly.dev/.well-known/oauth-protected-resource"
Content-Type: application/json

{"error":"unauthorized","error_description":"Authentication required to access MCP endpoint"}
```

**Expected protected resource metadata:**
```json
{
  "resource": "https://vire-pprod-portal.fly.dev",
  "authorization_servers": ["https://vire-pprod-portal.fly.dev"],
  "scopes_supported": ["portfolio:read", "portfolio:write", "tools:invoke"],
  "bearer_methods_supported": ["header"]
}
```

### 3. Test with MCP Clients

#### Claude Desktop

1. Open Claude Desktop settings
2. Add MCP server: `https://vire-pprod-portal.fly.dev/mcp`
3. **Expected**: Browser opens for OAuth authorization
4. Complete login with Google/GitHub
5. **Expected**: Redirect back to Claude, tools available

#### ChatGPT

1. Go to ChatGPT Connectors settings
2. Add MCP server: `https://vire-pprod-portal.fly.dev/mcp`
3. **Expected**: OAuth flow triggers
4. Complete authorization
5. **Expected**: Tools work consistently (not just once)

### 4. Monitor and Debug

If OAuth still doesn't work:

1. **Check portal logs**
   ```bash
   fly logs --app vire-pprod-portal
   ```

2. **Verify token endpoint works**
   ```bash
   # After getting a code from the authorize flow
   curl -X POST https://vire-pprod-portal.fly.dev/token \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "grant_type=authorization_code" \
     -d "code=<code>" \
     -d "redirect_uri=<redirect_uri>" \
     -d "client_id=<client_id>"
   ```

3. **Test authenticated MCP call**
   ```bash
   curl -X POST https://vire-pprod-portal.fly.dev/mcp \
     -H "Authorization: Bearer <access_token>" \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
   ```

## Known Issues to Watch

### 1. Token Refresh

ChatGPT "works once then stops" may indicate token refresh issues. After initial fix, monitor:
- Are refresh tokens being issued?
- Is the refresh flow working?

### 2. CORS

If browser-based OAuth fails, check CORS headers on:
- `/.well-known/*` endpoints
- `/token` endpoint

### 3. Session Duration

Verify token expiry is reasonable for MCP use cases (currently 24h).

## Files Modified

| File | Changes |
|------|---------|
| `internal/mcp/handler.go` | 401 + WWW-Authenticate for unauthenticated requests |
| `internal/auth/discovery.go` | Added `bearer_methods_supported` |
| `internal/mcp/handler_test.go` | Tests for auth requirement |
| `internal/mcp/handler_stress_test.go` | Stress tests |

## References

- [RFC 9728: OAuth 2.0 Protected Resource Metadata](https://www.rfc-editor.org/rfc/rfc9728)
- [MCP Authorization Specification](https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization)
- [Portal: OAuth Troubleshooting Findings](./oauth-troubleshooting-findings.md)
- [vire-server: OAuth Troubleshooting](../../vire/docs/oauth-troubleshooting.md)

## Success Criteria

- [x] Unauthenticated `/mcp` requests return 401 with WWW-Authenticate header
- [x] WWW-Authenticate header contains valid `resource_metadata` URL
- [x] Protected resource metadata includes `bearer_methods_supported`
- [x] All OAuth-related tests pass
- [x] Changes committed and pushed (`7808fde`)
- [ ] Claude Desktop triggers OAuth flow when adding MCP server
- [ ] ChatGPT maintains connection after OAuth (doesn't "stop working")
- [ ] Deployed to pprod
