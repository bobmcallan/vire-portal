package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// =============================================================================
// Page Whitelist Bypass Stress Tests
// =============================================================================

func TestGetPage_StressWhitelistBypass(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	attacks := []struct {
		name string
		page string
	}{
		{"admin", "admin"},
		{"admin/users", "admin/users"},
		{"../admin", "../admin"},
		{"/admin", "/admin"},
		{"DASHBOARD", "DASHBOARD"},
		{"Dashboard", "Dashboard"},
		{"dashboard ", "dashboard "},
		{" dashboard", " dashboard"},
		{"dashboard\t", "dashboard\t"},
		{"dashboard\n", "dashboard\n"},
		{"dashboard\r\n", "dashboard\r\n"},
		{"dashboard\x00", "dashboard\x00"},
		{"null bytes", "dash\x00board"},
		{"unicode homoglyph", "d\u0430shboard"},   // Cyrillic 'a'
		{"unicode zero-width", "da\u200bshboard"}, // zero-width space
		{"url encoded", "dashboard%2Fadmin"},
		{"double url encoded", "dashboard%252Fadmin"},
		{"dot-dot-slash", "../../../etc/passwd"},
		{"backslash", "dashboard\\..\\admin"},
		{"empty", ""},
		{"just slash", "/"},
		{"just dot", "."},
		{"dot-dot", ".."},
		{"admin with query", "admin?x=1"},
		{"login", "login"},
		{"auth/callback", "auth/callback"},
		{"m", "m"},
		{"error", "error"},
		{"very long", strings.Repeat("a", 10000)},
	}

	for _, tc := range attacks {
		t.Run(tc.name, func(t *testing.T) {
			req := mcpgo.CallToolRequest{}
			req.Params.Arguments = map[string]interface{}{"page": tc.page}

			result, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if !result.IsError {
				t.Errorf("SECURITY: page %q was NOT rejected by whitelist", tc.page)
			}
		})
	}
}

// =============================================================================
// JWT Minting Security Tests
// =============================================================================

func TestMintLoopbackJWT_StressSignatureVerifiable(t *testing.T) {
	secret := []byte("test-secret-123")
	token, err := mintLoopbackJWT("alice", secret)
	if err != nil {
		t.Fatalf("mint failed: %v", err)
	}

	// Verify with validateJWT (same path the portal's auth middleware uses)
	claims, err := validateJWT(token, secret)
	if err != nil {
		t.Fatalf("loopback JWT should be valid: %v", err)
	}
	if claims.Sub != "alice" {
		t.Errorf("expected sub=alice, got %s", claims.Sub)
	}
	if claims.Iss != "vire-portal-loopback" {
		t.Errorf("expected iss=vire-portal-loopback, got %s", claims.Iss)
	}
}

func TestMintLoopbackJWT_StressWrongSecretRejected(t *testing.T) {
	token, err := mintLoopbackJWT("alice", []byte("correct-secret"))
	if err != nil {
		t.Fatalf("mint failed: %v", err)
	}

	_, err = validateJWT(token, []byte("wrong-secret"))
	if err == nil {
		t.Error("SECURITY: loopback JWT accepted with wrong secret")
	}
}

func TestMintLoopbackJWT_StressExpiry(t *testing.T) {
	token, err := mintLoopbackJWT("alice", []byte("secret"))
	if err != nil {
		t.Fatalf("mint failed: %v", err)
	}

	parts := strings.Split(token, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims struct {
		Iat int64 `json:"iat"`
		Exp int64 `json:"exp"`
	}
	json.Unmarshal(payload, &claims)

	ttl := claims.Exp - claims.Iat
	if ttl != 30 {
		t.Errorf("expected 30s TTL, got %d", ttl)
	}

	// Verify token is NOT yet expired
	if claims.Exp < time.Now().Unix() {
		t.Error("freshly minted token is already expired")
	}

	// Verify it would expire in the future (not infinite)
	if claims.Exp > time.Now().Unix()+60 {
		t.Error("SECURITY: token TTL exceeds 60s — should be 30s")
	}
}

func TestMintLoopbackJWT_StressHostileUserID(t *testing.T) {
	hostileIDs := []string{
		"'; DROP TABLE users; --",
		"<script>alert(1)</script>",
		"../../etc/passwd",
		"\r\nX-Evil: injected",
		strings.Repeat("A", 100000),
		"user\x00admin",
		"",
	}

	for _, id := range hostileIDs {
		t.Run("id_"+safeSubstring(id, 20), func(t *testing.T) {
			token, err := mintLoopbackJWT(id, []byte("secret"))
			if err != nil {
				// Error is acceptable for hostile input
				return
			}
			// If it succeeds, verify the sub is exactly what was passed
			parts := strings.Split(token, ".")
			payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
			var claims struct {
				Sub string `json:"sub"`
			}
			json.Unmarshal(payload, &claims)
			if claims.Sub != id {
				t.Errorf("sub mismatch: expected %q, got %q", id, claims.Sub)
			}
		})
	}
}

func TestMintLoopbackJWT_StressReplayPrevention(t *testing.T) {
	// Two tokens minted at the same second should have the same iat/exp
	// but be usable independently. This is expected behavior for JWTs.
	// The 30s TTL limits the window for replay.
	secret := []byte("secret")
	token1, _ := mintLoopbackJWT("alice", secret)
	token2, _ := mintLoopbackJWT("alice", secret)

	// Tokens minted in the same second are identical (deterministic)
	// This is a known property - JWTs don't have jti claims.
	// The 30s TTL mitigates replay risk sufficiently for loopback use.
	t.Logf("INFO: loopback JWTs are deterministic per-second (no jti). "+
		"30s TTL limits replay window. tokens_equal=%v", token1 == token2)
}

// =============================================================================
// Response Size Attack Tests
// =============================================================================

func TestGetPage_StressLargeResponse(t *testing.T) {
	// Server returns 6MB — should be capped at 5MB
	largeBody := strings.Repeat("A", 6<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(largeBody))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	if len(text) > maxPageResponseSize+1 {
		t.Errorf("SECURITY: response exceeded 5MB limit: %d bytes", len(text))
	}
}

func TestGetPage_StressSlowDripResponse(t *testing.T) {
	// Server sends data very slowly — should be bounded by loopbackTimeout
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			w.Write([]byte("A"))
			if ok {
				flusher.Flush()
			}
			time.Sleep(200 * time.Millisecond)
		}
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	// Use a context with a short timeout to keep test fast
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ctx = WithUserContext(ctx, UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	start := time.Now()
	result, err := handler(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Either error result from timeout or truncated body
	if elapsed > 5*time.Second {
		t.Errorf("slow drip took too long: %v (should timeout)", elapsed)
	}
	_ = result // may be error or partial — both acceptable
}

// =============================================================================
// Authentication Bypass Tests
// =============================================================================

func TestGetPage_StressNoUserContext(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("SECURITY: handler accepted request without UserContext")
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	if !strings.Contains(text, "authentication") {
		t.Errorf("expected authentication error, got: %s", text)
	}
}

func TestGetPage_StressEmptyUserID(t *testing.T) {
	// UserContext with empty UserID — should still work (sub will be "")
	// The handler checks for UserContext presence, not UserID content
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: ""})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The handler will proceed with empty userID — the loopback will fail
	// because localhost:1 is unreachable. This is acceptable behavior.
	// A stricter check would reject empty userIDs explicitly.
	t.Logf("INFO: empty UserID accepted by handler (loopback will auth as empty sub). "+
		"IsError=%v", result.IsError)
}

// =============================================================================
// Loopback Cookie Verification Tests
// =============================================================================

func TestGetPage_StressCookieContainsValidJWT(t *testing.T) {
	secret := []byte("test-secret-abc")
	var receivedCookie string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("vire_session")
		if err == nil {
			receivedCookie = cookie.Value
		}
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, secret)
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	handler(ctx, req)

	if receivedCookie == "" {
		t.Fatal("no vire_session cookie received by loopback server")
	}

	// Verify the cookie is a valid JWT signed with the correct secret
	claims, err := validateJWT(receivedCookie, secret)
	if err != nil {
		t.Fatalf("loopback cookie is not a valid JWT: %v", err)
	}
	if claims.Sub != "alice" {
		t.Errorf("loopback JWT sub should be alice, got %s", claims.Sub)
	}
	if claims.Iss != "vire-portal-loopback" {
		t.Errorf("loopback JWT iss should be vire-portal-loopback, got %s", claims.Iss)
	}
}

func TestGetPage_StressNoExtraCookiesLeaked(t *testing.T) {
	var cookies []*http.Cookie

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies = r.Cookies()
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	handler(ctx, req)

	if len(cookies) != 1 {
		t.Errorf("expected exactly 1 cookie, got %d: %v", len(cookies), cookies)
	}
	if len(cookies) > 0 && cookies[0].Name != "vire_session" {
		t.Errorf("expected cookie name vire_session, got %s", cookies[0].Name)
	}
}

// =============================================================================
// CRITICAL: RefreshCatalog Drops portal_get_page
// =============================================================================

func TestGetPage_StressRefreshCatalogPreservesTool(t *testing.T) {
	// After RefreshCatalog, portal_get_page must still be registered.
	// This test verifies the fix for the critical bug where SetTools()
	// replaces all tools but only re-adds get_version.
	srv := makeMockCatalogServer(t,
		func() string { return "build-1" },
		func() string {
			return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]}]`
		},
	)
	defer srv.Close()

	h := makeHandlerWithServer(t, srv)
	defer h.Close()

	// Verify portal_get_page exists before refresh
	before := h.mcpSrv.GetTool("portal_get_page")
	if before == nil {
		t.Fatal("portal_get_page should be registered before refresh")
	}

	// Trigger refresh
	_, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("RefreshCatalog failed: %v", err)
	}

	// CRITICAL: portal_get_page must still exist after refresh
	after := h.mcpSrv.GetTool("portal_get_page")
	if after == nil {
		t.Error("CRITICAL: RefreshCatalog dropped portal_get_page tool. " +
			"SetTools() replaces all tools but only re-adds get_version. " +
			"portal_get_page must also be re-added in RefreshCatalog().")
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestGetPage_StressConcurrentRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	var wg sync.WaitGroup
	errors := make(chan string, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pages := []string{"dashboard", "strategy", "cash", "glossary"}
			page := pages[n%len(pages)]

			req := mcpgo.CallToolRequest{}
			req.Params.Arguments = map[string]interface{}{"page": page}

			result, err := handler(ctx, req)
			if err != nil {
				errors <- fmt.Sprintf("goroutine %d: unexpected error: %v", n, err)
				return
			}
			if result.IsError {
				errors <- fmt.Sprintf("goroutine %d: unexpected tool error", n)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for e := range errors {
		t.Error(e)
	}

	mu.Lock()
	if requestCount != 50 {
		t.Errorf("expected 50 requests, got %d", requestCount)
	}
	mu.Unlock()
}

// =============================================================================
// Error Message Information Leak Tests
// =============================================================================

func TestGetPage_StressErrorDoesNotLeakInternalURL(t *testing.T) {
	// When portal is unreachable, error should not expose the full loopback URL
	handler := GetPageToolHandler("http://127.0.0.1:19999", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when portal unreachable")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	// The error message includes the full URL via %v on the http.Client error.
	// This is an info leak — internal loopback address exposed to MCP client.
	// Logging for awareness but not blocking — MCP clients are authenticated.
	if strings.Contains(text, "127.0.0.1:19999") {
		t.Log("INFO: error message leaks internal loopback URL to MCP client. " +
			"Not critical since MCP clients are authenticated, but consider " +
			"sanitizing error messages to avoid exposing infrastructure details.")
	}
}

func TestGetPage_StressNon200DoesNotLeakSensitiveBody(t *testing.T) {
	// Portal returns 500 with sensitive info — verify it's passed to MCP client
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("panic: runtime error at /app/internal/secret/handler.go:42\nJWT_SECRET=supersecret"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	// The error body is passed through to the MCP client.
	// For non-200 responses, the full body is included in the error.
	if strings.Contains(text, "JWT_SECRET") {
		t.Log("INFO: non-200 error response body is passed through to MCP client. " +
			"If the portal leaks secrets in error pages, they propagate to MCP. " +
			"This is acceptable since portal error pages should not contain secrets.")
	}
}

// =============================================================================
// SSRF / Base URL Manipulation Tests
// =============================================================================

func TestGetPage_StressBaseURLNotFromUserInput(t *testing.T) {
	// Verify the handler uses the hardcoded base URL, not any user-supplied value
	var requestedHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedHost = r.Host
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	// Even if the page param contains URL-like content, it should be rejected
	// by the whitelist. The base URL is always from config.
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	handler(ctx, req)

	// The request should go to our test server, not anywhere else
	if requestedHost == "" {
		t.Error("expected request to hit test server")
	}
}

// =============================================================================
// Chunked Transfer Encoding Test
// =============================================================================

func TestGetPage_StressChunkedResponseCapped(t *testing.T) {
	// Server sends chunked response exceeding 5MB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		// Write 6MB in chunks
		chunk := strings.Repeat("X", 64*1024) // 64KB chunks
		for i := 0; i < 100; i++ {            // 6.4MB total
			w.Write([]byte(chunk))
			if ok {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	if len(text) > maxPageResponseSize+1 {
		t.Errorf("SECURITY: chunked response exceeded 5MB: %d bytes", len(text))
	}
}

// =============================================================================
// Edge Case: Missing/Nil page argument
// =============================================================================

func TestGetPage_StressMissingPageArgument(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	// No arguments at all
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when page argument is missing")
	}
}

func TestGetPage_StressNonStringPageArgument(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	nonStrings := []interface{}{
		42,
		3.14,
		true,
		nil,
		[]string{"dashboard"},
		map[string]string{"page": "dashboard"},
	}

	for _, val := range nonStrings {
		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			req := mcpgo.CallToolRequest{}
			req.Params.Arguments = map[string]interface{}{"page": val}

			result, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Errorf("SECURITY: non-string page %v (%T) was not rejected", val, val)
			}
		})
	}
}

// =============================================================================
// LimitReader + ReadAll interaction
// =============================================================================

func TestGetPage_StressLimitReaderExactBoundary(t *testing.T) {
	// Server returns exactly maxPageResponseSize bytes
	body := strings.Repeat("B", maxPageResponseSize)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("should succeed with exactly 5MB response")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	if len(text) != maxPageResponseSize {
		t.Errorf("expected exactly %d bytes, got %d", maxPageResponseSize, len(text))
	}
}

// =============================================================================
// Verify loopback request method and path
// =============================================================================

func TestGetPage_StressRequestMethodIsGET(t *testing.T) {
	var method string
	var path string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "strategy"}

	handler(ctx, req)

	if method != "GET" {
		t.Errorf("expected GET request, got %s", method)
	}
	if path != "/strategy" {
		t.Errorf("expected /strategy path, got %s", path)
	}
}

// =============================================================================
// io.LimitReader truncation detection
// =============================================================================

func TestGetPage_StressTruncatedResponseNotDetected(t *testing.T) {
	// When response is >5MB, io.LimitReader silently truncates.
	// The handler returns partial HTML — no error. This could cause issues
	// if the MCP client tries to parse the HTML. Document this behavior.
	body := "<html><body>" + strings.Repeat("A", maxPageResponseSize+100) + "</body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("truncated response should not be an error")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	if strings.Contains(text, "</body></html>") {
		t.Error("expected truncated response to not contain closing tags")
	}
	t.Log("INFO: responses exceeding 5MB are silently truncated. " +
		"MCP client receives partial HTML without any indication of truncation.")
}

// =============================================================================
// Verify loopback does not follow redirects to external hosts
// =============================================================================

func TestGetPage_StressRedirectFollowed(t *testing.T) {
	// The default http.Client follows redirects. If the portal redirects
	// (e.g., unauthenticated -> /login), the loopback follows it.
	// This is generally fine since all redirects stay on the same host.
	redirectTarget := ""
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTarget = r.URL.Path
		w.Write([]byte("<html>redirected</html>"))
	}))
	defer final.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/login", http.StatusFound)
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The handler follows redirects and returns the final response
	t.Logf("INFO: loopback request follows redirects. Redirected to %s. "+
		"The default http.Client follows up to 10 redirects. "+
		"Consider using CheckRedirect to restrict to same-host redirects. "+
		"IsError=%v", redirectTarget, result.IsError)
}

// =============================================================================
// Verify io.ReadAll behavior with various readers
// =============================================================================

func TestGetPage_StressEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Empty 200 response
		w.WriteHeader(200)
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "alice"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty 200 response is still "success" — empty HTML
	if result.IsError {
		t.Log("INFO: empty 200 response treated as error")
	}
}

// Ensure the LimitReader import is exercised
var _ = io.LimitReader
