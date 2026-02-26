# OAuth Troubleshooting Findings: Claude Desktop and ChatGPT Integration

**Date:** 2026-02-26
**Issue:** OAuth flow not triggering when adding MCP server to Claude Desktop or ChatGPT

## Summary

The MCP server OAuth flow is not triggering correctly when adding the service to Claude Desktop or ChatGPT:
- **Claude Desktop**: OAuth authorization screen does not appear
- **ChatGPT**: Service adds once then stops working
- **Dev Mode**: Works correctly with custom URLs (e.g., `/mcp/{encrypted_uid}`)

## Root Cause Analysis

### Primary Issue: Missing WWW-Authenticate Header (RFC 9728 Non-Compliance)

The MCP specification (2025-03-26) and RFC 9728 require that when an unauthenticated client requests a protected resource, the server MUST:

1. Return `HTTP 401 Unauthorized`
2. Include `WWW-Authenticate` header with `resource_metadata` parameter pointing to the protected resource metadata endpoint

**Current behavior in vire-portal:**
- The `/mcp` endpoint (`internal/mcp/handler.go:108`) extracts user context but **never returns 401**
- If no valid token is present, the request continues with no user context
- **No WWW-Authenticate header is sent anywhere in the codebase**

### Expected OAuth Flow (Per MCP Spec)

```
┌─────────────────┐     ┌─────────────────┐
│  Claude Desktop │     │   Vire Portal   │
└────────┬────────┘     └────────┬────────┘
         │                       │
         │ 1. POST /mcp (no auth)│
         │──────────────────────>│
         │                       │
         │ 2. 401 + WWW-Authenticate:
         │    Bearer resource_metadata=
         │    ".../.well-known/oauth-protected-resource"
         │<──────────────────────│
         │                       │
         │ 3. GET /.well-known/oauth-protected-resource
         │──────────────────────>│
         │                       │
         │ 4. JSON metadata      │
         │<──────────────────────│
         │                       │
         │ 5. GET /.well-known/oauth-authorization-server
         │──────────────────────>│
         │                       │
         │ 6. JSON metadata      │
         │<──────────────────────│
         │                       │
         │ 7. Browser opens for  │
         │    OAuth authorization│
         │══════════════════════>│ (user interaction)
         │                       │
         │ 8. Callback with code │
         │<──────────────────────│
         │                       │
         │ 9. Exchange code for  │
         │    tokens (POST /token)
         │──────────────────────>│
         │                       │
         │ 10. Access token      │
         │<──────────────────────│
         │                       │
         │ 11. POST /mcp         │
         │     (Bearer token)    │
         │──────────────────────>│
         │                       │
         │ 12. Success response  │
         │<──────────────────────│
```

### Why Dev Mode Works

Dev mode (`/mcp/{encrypted_uid}`) bypasses OAuth entirely by embedding user identity in the URL path. The `DevHandler` (`internal/mcp/dev_handler.go`) decrypts the UID and injects it into the context before delegating to the main handler.

```
Dev Mode Flow:
/mcp/{encrypted_uid} → DevHandler.decryptUID() → Inject UserContext → MCPHandler
```

### Why ChatGPT "Adds Once Then Stops"

ChatGPT likely:
1. Makes an initial request that succeeds (no auth required currently)
2. Tries to call tools but gets empty/invalid results (no user context)
3. Stops because it can't determine why tools aren't working

### Why Claude Desktop OAuth Screen Doesn't Appear

Claude Desktop only triggers OAuth when it receives a 401 response. Since the server never returns 401, the OAuth flow is never initiated.

---

## Code Analysis

### Files Examined

| File | Issue |
|------|-------|
| `internal/mcp/handler.go:108-154` | `ServeHTTP` and `withUserContext` never return 401 - just proceed without user context |
| `internal/auth/discovery.go` | Metadata endpoints exist but no WWW-Authenticate header usage |
| `internal/server/routes.go` | Routes are correctly set up but MCP endpoint lacks auth middleware |
| `internal/auth/authorize.go` | OAuth flow is correctly implemented but never triggered |

### Current MCP Handler Behavior

```go
// internal/mcp/handler.go:108-111
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    r = h.withUserContext(r)  // Tries to extract user, but doesn't fail if missing
    h.streamable.ServeHTTP(w, r)  // Always delegates, even without auth
}
```

The `withUserContext` function (lines 117-154) extracts user context from Bearer token or cookie, but if validation fails, it simply returns the original request unchanged - it never returns an error or triggers authentication.

### Missing Implementation

1. **No 401 response for unauthenticated MCP requests**
2. **No WWW-Authenticate header with resource_metadata**
3. **No authentication requirement enforcement at `/mcp` endpoint**

---

## Recommended Fix

### Step 1: Modify MCP Handler to Require Authentication

File: `internal/mcp/handler.go`

Change `ServeHTTP` to return 401 with WWW-Authenticate header when no valid user context:

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    r = h.withUserContext(r)

    // Check if user context exists - if not, require authentication per RFC 9728
    if _, ok := FromUserContext(r.Context()); !ok {
        scheme := "http"
        if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
            scheme = "https"
        }
        resourceMetadata := fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource",
            scheme, r.Host)

        w.Header().Set("WWW-Authenticate",
            fmt.Sprintf(`Bearer resource_metadata="%s"`, resourceMetadata))
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error":             "unauthorized",
            "error_description": "Authentication required to access MCP endpoint",
        })
        return
    }

    h.streamable.ServeHTTP(w, r)
}
```

### Step 2: Update Protected Resource Metadata

File: `internal/auth/discovery.go`

Ensure `handleProtectedResource` includes `bearer_methods_supported`:

```go
func handleProtectedResource(w http.ResponseWriter, r *http.Request, baseURL string) {
    if r.Method != http.MethodGet && r.Method != http.MethodHead {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    metadata := map[string]interface{}{
        "resource":                 baseURL,
        "authorization_servers":    []string{baseURL},
        "scopes_supported":         []string{"portfolio:read", "portfolio:write", "tools:invoke"},
        "bearer_methods_supported": []string{"header"}, // Add this per RFC 9728
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "public, max-age=3600")
    json.NewEncoder(w).Encode(metadata)
}
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/mcp/handler.go` | Add 401 + WWW-Authenticate response for unauthenticated requests |
| `internal/auth/discovery.go` | Add `bearer_methods_supported` to protected resource metadata |

---

## Verification Plan

### 1. Local Testing with curl

```bash
# Should return 401 with WWW-Authenticate header
curl -v http://localhost:8883/mcp

# Expected response:
# HTTP/1.1 401 Unauthorized
# Www-Authenticate: Bearer resource_metadata="http://localhost:8883/.well-known/oauth-protected-resource"
# {"error":"unauthorized","error_description":"Authentication required to access MCP endpoint"}
```

### 2. Test OAuth Discovery Chain

```bash
# Verify protected resource metadata includes bearer_methods_supported
curl http://localhost:8883/.well-known/oauth-protected-resource

# Expected:
# {
#   "resource": "http://localhost:8883",
#   "authorization_servers": ["http://localhost:8883"],
#   "scopes_supported": ["portfolio:read", "portfolio:write", "tools:invoke"],
#   "bearer_methods_supported": ["header"]
# }

# Verify authorization server metadata
curl http://localhost:8883/.well-known/oauth-authorization-server
```

### 3. Test with Claude Desktop

```bash
# Add MCP server
claude mcp add --transport http http://localhost:8883/mcp

# Browser should open for OAuth authorization
# Complete login
# Verify tools are available
```

### 4. Test with ChatGPT

1. Add MCP server via ChatGPT connectors
2. Verify OAuth flow triggers
3. Verify tools work after authentication

### 5. Run Existing Tests

```bash
go test ./internal/mcp/... -v
go test ./internal/auth/... -v
```

---

## Related Documentation

- `docs/authentication/mcp-oauth-implementation-steps.md` - Original OAuth implementation steps
- `docs/authentication/vire-server-oauth-requirements.md` - Server-side OAuth requirements
- `docs/authentication/authentication.md` - General authentication overview

---

## References

- [RFC 9728: OAuth 2.0 Protected Resource Metadata](https://www.rfc-editor.org/rfc/rfc9728)
- [MCP 2025-03-26 Authorization Specification](https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization)
- [CVE-2025-6514 - MCP 401 Authentication Flow](https://jfrog.com/blog/2025-6514-critical-mcp-remote-rce-vulnerability/)
- [Microsoft Learn - Secure MCP Servers](https://learn.microsoft.com/en-us/azure/api-management/secure-mcp-servers)
- [Stack Overflow Blog - Authentication in MCP](https://stackoverflow.blog/2026/01/21/is-that-allowed-authentication-and-authorization-in-model-context-protocol/)
