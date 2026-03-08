# portal_get_page MCP Tool — Implementation Spec

**Feedback:** fb_28525072
**Date:** 2026-03-08

## 1. Scope

New MCP tool `portal_get_page` that allows MCP clients (Claude CLI/Desktop) to fetch the rendered HTML of a portal page. It makes an internal HTTP loopback request to the portal itself, using a short-lived JWT minted with the calling user's identity, and returns the full HTML response.

**Supported pages:** `dashboard`, `strategy`, `cash`, `glossary`, `changelog`, `help`, `mcp-info`, `profile`, `docs`

**Excluded pages:** `/` (landing — public, not useful), `/error` (not useful), `/admin/*` (security-sensitive), `/m` (mobile duplicate of dashboard)

**Format:** HTML only (no PDF/image).

**NOT in scope:** Refactoring existing handlers, adding new routes, changing auth flows.

## 2. File Changes

### A. `internal/mcp/get_page.go` (NEW FILE)

```go
package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// allowedPages maps page names to their URL paths.
var allowedPages = map[string]string{
	"dashboard": "/dashboard",
	"strategy":  "/strategy",
	"cash":      "/cash",
	"glossary":  "/glossary",
	"changelog": "/changelog",
	"help":      "/help",
	"mcp-info":  "/mcp-info",
	"profile":   "/profile",
	"docs":      "/docs",
}

// maxPageResponseSize limits loopback response body to 5MB.
const maxPageResponseSize = 5 << 20

// loopbackTimeout is the HTTP client timeout for loopback requests.
const loopbackTimeout = 15 * time.Second

// loopbackJWTTTL is the TTL for short-lived loopback JWTs.
const loopbackJWTTTL = 30 * time.Second
```

**Functions:**

#### `GetPageTool() mcp.Tool`
- Name: `"portal_get_page"`
- Description: `"Fetch a rendered portal page as HTML. Returns the full HTML of the requested page as seen by the authenticated user. Use this to review page layout, content accuracy, and data rendering without requiring screenshots."`
- Parameters:
  - `page` (string, required): Page name. Description: `"Page to fetch. One of: dashboard, strategy, cash, glossary, changelog, help, mcp-info, profile, docs"`

#### `GetPageToolHandler(portalBaseURL string, jwtSecret []byte) server.ToolHandlerFunc`

Handler logic:
1. Extract `page` parameter. Validate against `allowedPages` map (exact match — no string concatenation).
2. Extract `UserContext` from context via `GetUserContext()`. Return `errorResult` if missing.
3. Call `mintLoopbackJWT(userID, jwtSecret)` to create a 30-second JWT.
4. Build `http.NewRequestWithContext(ctx, "GET", portalBaseURL + allowedPages[page], nil)`.
5. Set `Cookie` header: `vire_session=<jwt>`.
6. Execute with `http.Client{Timeout: loopbackTimeout}`.
7. Read body with `io.LimitReader(resp.Body, maxPageResponseSize)`.
8. If `resp.StatusCode != 200`, return `errorResult` with status info.
9. Return body as `mcp.NewTextContent(string(body))`.

#### `mintLoopbackJWT(userID string, secret []byte) (string, error)`

Create a short-lived JWT:
- Header: `{"alg":"HS256","typ":"JWT"}`
- Payload: `{"sub": userID, "iss": "vire-portal-loopback", "iat": now, "exp": now+30s}`
- Signature: HMAC-SHA256 with `secret` (same as existing auth flow)
- If secret is empty (dev mode), sign with empty key (matches existing ValidateJWT behavior)
- Use `base64.RawURLEncoding` for all parts (no padding)
- Return the `header.payload.signature` string

### B. `internal/mcp/handler.go` (MODIFY)

**Struct change:** Add `portalBaseURL string` field to `Handler` struct.

**`NewHandler` changes (after line 89, the VersionTool registration):**
```go
// Register portal_get_page local tool
mcpSrv.AddTool(GetPageTool(), GetPageToolHandler(cfg.BaseURL(), []byte(cfg.Auth.JWTSecret)))
```

Set field in handler construction:
```go
portalBaseURL: cfg.BaseURL(),
```

**`RefreshCatalog` changes (after line 144-147, the VersionTool append):**
```go
// Always include portal_get_page local tool
tools = append(tools, mcpserver.ServerTool{
    Tool:    GetPageTool(),
    Handler: GetPageToolHandler(h.portalBaseURL, h.jwtSecret),
})
```

### C. `internal/mcp/get_page_test.go` (NEW FILE)

Unit tests:

1. **`TestGetPageTool_Definition`** — Verify tool name is `"portal_get_page"`, has description, has `page` parameter.

2. **`TestGetPageToolHandler_ValidPage`** — Start `httptest.NewServer` that returns `<html>dashboard</html>` for GET `/dashboard`. Create handler with server URL. Set UserContext in context. Call with `page: "dashboard"`. Assert response contains the HTML.

3. **`TestGetPageToolHandler_InvalidPage`** — Call with `page: "admin"`. Assert error result with "not a valid page" or similar.

4. **`TestGetPageToolHandler_EmptyPage`** — Call with empty page param. Assert error result.

5. **`TestGetPageToolHandler_NoUserContext`** — Call without UserContext in context. Assert error result about authentication.

6. **`TestGetPageToolHandler_PortalUnavailable`** — Use `http://localhost:1` (closed port). Assert error result.

7. **`TestGetPageToolHandler_NonOKResponse`** — Mock returns 500. Assert error result mentions status code.

8. **`TestMintLoopbackJWT_Valid`** — Mint JWT with known secret. Decode and verify claims: sub, iss, iat, exp.

9. **`TestMintLoopbackJWT_ShortExpiry`** — Verify exp - iat == 30.

10. **`TestMintLoopbackJWT_EmptySecret`** — Verify no error with empty secret (dev mode).

11. **`TestGetPageToolHandler_AllPages`** — Iterate all `allowedPages`, verify each returns success.

12. **`TestGetPageToolHandler_TraversalAttempt`** — Try `"../admin"`, `"admin/users"`, `"/admin"`. All should fail validation.

13. **`TestGetPageToolHandler_CookieSet`** — Verify the loopback request includes `vire_session` cookie (mock server checks).

## 3. Security

1. **Page whitelist:** Hardcoded `allowedPages` map. Exact match only. No path construction from user input.
2. **JWT TTL:** 30-second expiry limits token exposure.
3. **JWT issuer:** `"vire-portal-loopback"` distinguishes from real session tokens.
4. **Response size cap:** 5MB `io.LimitReader` prevents memory exhaustion.
5. **User isolation:** Loopback uses the MCP session's userID — users only see their own data.
6. **Admin exclusion:** Admin pages not in `allowedPages`.

## 4. Edge Cases

1. **Portal not listening yet:** Connection refused → clear error message.
2. **Empty JWT secret (dev mode):** Works — matches existing ValidateJWT empty-secret behavior.
3. **Dashboard without portfolio context:** SSR renders with whatever the default portfolio resolution produces.
4. **Large page response:** Capped at 5MB via LimitReader.
5. **Concurrent calls:** Handler is stateless, safe for concurrent use.

## 5. No UI Changes

This feature adds an MCP tool only. No HTML templates, CSS, or JavaScript changes. No UI tests needed.
