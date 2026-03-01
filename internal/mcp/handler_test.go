package mcp

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// buildTestJWT creates an unsigned JWT for testing (alg:none, no signature).
func buildTestJWT(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": sub,
		"iss": "vire-dev",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	return header + "." + payload + "."
}

// --- withUserContext Tests ---

func TestWithUserContext_ValidCookie(t *testing.T) {
	jwt := buildTestJWT("user42")

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true")
	}
	if uc.UserID != "user42" {
		t.Errorf("expected UserID user42, got %s", uc.UserID)
	}
}

func TestWithUserContext_NoCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/mcp", nil)

	h := &Handler{}
	result := h.withUserContext(req)

	_, ok := GetUserContext(result.Context())
	if ok {
		t.Error("expected GetUserContext to return ok=false when no cookie is set")
	}
}

func TestWithUserContext_InvalidJWT(t *testing.T) {
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "not-a-jwt"})

	h := &Handler{}
	result := h.withUserContext(req)

	_, ok := GetUserContext(result.Context())
	if ok {
		t.Error("expected GetUserContext to return ok=false for invalid JWT")
	}
}

// --- Bearer token tests ---

func TestWithUserContext_BearerToken(t *testing.T) {
	jwt := buildTestJWT("bearer-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true for Bearer token")
	}
	if uc.UserID != "bearer-user" {
		t.Errorf("expected UserID bearer-user, got %s", uc.UserID)
	}
}

func TestWithUserContext_BearerTokenTakesPriority(t *testing.T) {
	bearerJWT := buildTestJWT("bearer-user")
	cookieJWT := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+bearerJWT)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: cookieJWT})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true")
	}
	if uc.UserID != "bearer-user" {
		t.Errorf("expected Bearer to take priority, got UserID %s", uc.UserID)
	}
}

func TestWithUserContext_InvalidBearerFallsToCookie(t *testing.T) {
	cookieJWT := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer invalid-jwt")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: cookieJWT})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true from cookie fallback")
	}
	if uc.UserID != "cookie-user" {
		t.Errorf("expected cookie fallback, got UserID %s", uc.UserID)
	}
}

func TestWithUserContext_EmptyBearerIgnored(t *testing.T) {
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer ")

	h := &Handler{}
	result := h.withUserContext(req)

	_, ok := GetUserContext(result.Context())
	if ok {
		t.Error("expected no user context for empty Bearer token")
	}
}

func TestWithUserContext_NonBearerAuthIgnored(t *testing.T) {
	cookieJWT := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: cookieJWT})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true from cookie")
	}
	if uc.UserID != "cookie-user" {
		t.Errorf("expected cookie fallback for Basic auth, got UserID %s", uc.UserID)
	}
}

// --- extractJWTSub Tests ---

func TestExtractJWTSub_ValidJWT(t *testing.T) {
	jwt := buildTestJWT("user99")

	sub := extractJWTSub(jwt)
	if sub != "user99" {
		t.Errorf("expected sub user99, got %s", sub)
	}
}

func TestExtractJWTSub_InvalidBase64Payload(t *testing.T) {
	// Two parts but second part is invalid base64
	sub := extractJWTSub("header.!!!invalid-base64!!!.sig")
	if sub != "" {
		t.Errorf("expected empty string for invalid base64, got %s", sub)
	}
}

func TestExtractJWTSub_InvalidJSON(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte("not json at all"))
	sub := extractJWTSub("header." + payload + ".sig")
	if sub != "" {
		t.Errorf("expected empty string for invalid JSON, got %s", sub)
	}
}

func TestExtractJWTSub_MissingSub(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"vire-server","exp":999999999}`))
	sub := extractJWTSub("header." + payload + ".sig")
	if sub != "" {
		t.Errorf("expected empty string for missing sub, got %s", sub)
	}
}

func TestExtractJWTSub_EmptyString(t *testing.T) {
	sub := extractJWTSub("")
	if sub != "" {
		t.Errorf("expected empty string for empty input, got %s", sub)
	}
}

func TestExtractJWTSub_NoDotsInToken(t *testing.T) {
	sub := extractJWTSub("nodots")
	if sub != "" {
		t.Errorf("expected empty string for token with no dots, got %s", sub)
	}
}

func TestExtractJWTSub_SingleDot(t *testing.T) {
	sub := extractJWTSub("one.dot")
	// Has two parts (index 0 and 1), so second part is decoded
	// "dot" is valid base64 but decodes to garbage JSON
	if sub != "" {
		t.Errorf("expected empty string for single-dot token, got %s", sub)
	}
}

// --- ServeHTTP 401 Response Tests (RFC 9728 compliance) ---

func TestServeHTTP_UnauthenticatedReturns401(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestServeHTTP_UnauthenticatedHasWWWAuthenticateHeader(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth == "" {
		t.Error("expected WWW-Authenticate header to be set")
	}
	if !strings.Contains(wwwAuth, "Bearer") {
		t.Error("expected WWW-Authenticate to contain 'Bearer'")
	}
	if !strings.Contains(wwwAuth, `resource_metadata=`) {
		t.Error("expected WWW-Authenticate to contain 'resource_metadata='")
	}
}

func TestServeHTTP_WWWAuthenticateResourceMetadataHTTP(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Host = "localhost:8883"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	expected := `Bearer resource_metadata="http://localhost:8883/.well-known/oauth-protected-resource"`
	if wwwAuth != expected {
		t.Errorf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}
}

func TestServeHTTP_WWWAuthenticateResourceMetadataHTTPS(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Host = "portal.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	expected := `Bearer resource_metadata="https://portal.example.com/.well-known/oauth-protected-resource"`
	if wwwAuth != expected {
		t.Errorf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}
}

func TestServeHTTP_WWWAuthenticateResourceMetadataTLS(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Host = "secure.example.com"
	req.TLS = &tls.ConnectionState{} // Simulate TLS connection
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	expected := `Bearer resource_metadata="https://secure.example.com/.well-known/oauth-protected-resource"`
	if wwwAuth != expected {
		t.Errorf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}
}

func TestServeHTTP_UnauthenticatedJSONResponse(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got %q", resp["error"])
	}
	if resp["error_description"] != "Authentication required to access MCP endpoint" {
		t.Errorf("unexpected error_description: %q", resp["error_description"])
	}
}

// TestServeHTTP_AuthenticatedBearerNoWWWAuthenticate tests that authenticated
// requests with Bearer token do NOT get a WWW-Authenticate header (they pass through).
func TestServeHTTP_AuthenticatedBearerNoWWWAuthenticate(t *testing.T) {
	jwt := buildTestJWT("bearer-user")

	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	// Authenticated requests should NOT get a 401 or WWW-Authenticate header
	// They will likely error because streamable is nil, but that's expected
	// The key assertion is no WWW-Authenticate header
	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth != "" {
		t.Errorf("expected no WWW-Authenticate header for authenticated request, got %q", wwwAuth)
	}
}

// TestServeHTTP_AuthenticatedCookieNoWWWAuthenticate tests that authenticated
// requests with session cookie do NOT get a WWW-Authenticate header (they pass through).
func TestServeHTTP_AuthenticatedCookieNoWWWAuthenticate(t *testing.T) {
	jwt := buildTestJWT("cookie-user")

	h := &Handler{}

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt})
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	// Authenticated requests should NOT get a WWW-Authenticate header
	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth != "" {
		t.Errorf("expected no WWW-Authenticate header for authenticated request, got %q", wwwAuth)
	}
}

// =============================================================================
// Catalog Refresh Tests
// =============================================================================

// makeMockCatalogServer creates a test HTTP server that serves:
// - GET /api/mcp/tools with the catalog JSON from catalogFn()
// - GET /api/version with the build string from buildFn()
// and returns the server for use in tests.
func makeMockCatalogServer(t *testing.T, buildFn func() string, catalogFn func() string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			build := buildFn()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"build": build})
		case "/api/mcp/tools":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(catalogFn()))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	return srv
}

// makeHandler creates a Handler with a mock catalog server for testing.
// Returns the handler and the server. The server must be closed after use.
func makeHandlerWithServer(t *testing.T, srv *httptest.Server) *Handler {
	t.Helper()
	logger := testLogger()
	cfg := testConfig()
	cfg.API.URL = srv.URL
	cfg.MCP.CatalogRetries = 1

	h := NewHandler(cfg, logger)
	return h
}

// TestRefreshCatalog_UpdatesTools verifies that RefreshCatalog replaces the tool
// catalog atomically: handler.Catalog() returns updated tools, mcpSrv.ListTools()
// reflects the change.
func TestRefreshCatalog_UpdatesTools(t *testing.T) {
	// Phase 1: initial catalog with 1 tool
	phase := 1
	srv := makeMockCatalogServer(t,
		func() string { return "build-1" },
		func() string {
			if phase == 1 {
				return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]}]`
			}
			return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]},{"name":"tool_b","description":"Tool B","method":"GET","path":"/api/tool_b","params":[]}]`
		},
	)
	defer srv.Close()

	h := makeHandlerWithServer(t, srv)
	defer h.Close()

	// Verify initial catalog has 1 tool
	if got := len(h.Catalog()); got != 1 {
		t.Fatalf("expected 1 initial catalog tool, got %d", got)
	}

	// Phase 2: expand catalog to 2 tools and refresh
	phase = 2
	count, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("RefreshCatalog returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected RefreshCatalog to return 2, got %d", count)
	}

	// Catalog() should reflect the update
	catalog := h.Catalog()
	if len(catalog) != 2 {
		t.Errorf("expected 2 catalog tools after refresh, got %d", len(catalog))
	}

	// mcpSrv.ListTools() must contain tool_a, tool_b, and get_version
	tools := h.mcpSrv.ListTools()
	if tools["tool_a"] == nil {
		t.Error("expected tool_a in mcpSrv after refresh")
	}
	if tools["tool_b"] == nil {
		t.Error("expected tool_b in mcpSrv after refresh")
	}
	if tools["get_version"] == nil {
		t.Error("expected get_version in mcpSrv after refresh")
	}
}

// TestRefreshCatalog_ServerUnreachable verifies that when the server is down
// during a refresh, the error is returned and the original catalog is preserved.
func TestRefreshCatalog_ServerUnreachable(t *testing.T) {
	srv := makeMockCatalogServer(t,
		func() string { return "build-1" },
		func() string {
			return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]}]`
		},
	)

	h := makeHandlerWithServer(t, srv)
	defer h.Close()

	// Verify we have initial catalog
	if got := len(h.Catalog()); got != 1 {
		t.Fatalf("expected 1 initial catalog tool, got %d", got)
	}

	// Close the server to make it unreachable
	srv.Close()

	// RefreshCatalog should return error
	_, err := h.RefreshCatalog()
	if err == nil {
		t.Fatal("expected error when server is unreachable")
	}

	// Original catalog should be preserved
	if got := len(h.Catalog()); got != 1 {
		t.Errorf("expected original 1 catalog tool preserved, got %d", got)
	}
}

// TestRefreshCatalog_VersionToolAlwaysPresent verifies that get_version is always
// present in the mcpSrv tool list after a refresh, even when not in the catalog.
func TestRefreshCatalog_VersionToolAlwaysPresent(t *testing.T) {
	// Catalog without get_version
	srv := makeMockCatalogServer(t,
		func() string { return "build-1" },
		func() string {
			return `[{"name":"some_tool","description":"Some tool","method":"GET","path":"/api/some_tool","params":[]}]`
		},
	)
	defer srv.Close()

	h := makeHandlerWithServer(t, srv)
	defer h.Close()

	_, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("RefreshCatalog returned error: %v", err)
	}

	// get_version must always be present
	if got := h.mcpSrv.GetTool("get_version"); got == nil {
		t.Error("expected get_version to be present in mcpSrv after refresh")
	}
}

// TestWatchServerVersion_DetectsChange verifies that triggerRefresh updates the
// catalog when called directly after changing server state.
func TestWatchServerVersion_DetectsChange(t *testing.T) {
	build := "build-1"
	phase := 1
	srv := makeMockCatalogServer(t,
		func() string { return build },
		func() string {
			if phase == 1 {
				return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]}]`
			}
			return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]},{"name":"tool_b","description":"Tool B","method":"GET","path":"/api/tool_b","params":[]}]`
		},
	)
	defer srv.Close()

	h := makeHandlerWithServer(t, srv)
	defer h.Close()

	if got := len(h.Catalog()); got != 1 {
		t.Fatalf("expected 1 initial catalog tool, got %d", got)
	}

	// Simulate a build change and trigger refresh
	phase = 2
	build = "build-2"
	h.triggerRefresh(build)

	// Catalog should now have 2 tools
	if got := len(h.Catalog()); got != 2 {
		t.Errorf("expected 2 catalog tools after triggerRefresh, got %d", got)
	}
}

// TestFetchServerBuild_Success verifies that fetchServerBuild returns the correct
// build string when the /api/version endpoint is reachable.
func TestFetchServerBuild_Success(t *testing.T) {
	srv := makeMockCatalogServer(t,
		func() string { return "build-abc123" },
		func() string { return `[]` },
	)
	defer srv.Close()

	h := makeHandlerWithServer(t, srv)
	defer h.Close()

	got := h.fetchServerBuild()
	if got != "build-abc123" {
		t.Errorf("expected build-abc123, got %q", got)
	}
}

// TestFetchServerBuild_Unreachable verifies that fetchServerBuild returns empty
// string when the server is unreachable (mockAPIServer returns 503).
func TestFetchServerBuild_Unreachable(t *testing.T) {
	// Use the shared mockAPIServer which returns 503 for all requests
	logger := testLogger()
	cfg := testConfig()
	// mockAPIServer.URL is set in testConfig()
	cfg.MCP.CatalogRetries = 1

	h := NewHandler(cfg, logger)
	defer h.Close()

	got := h.fetchServerBuild()
	if got != "" {
		t.Errorf("expected empty string for unreachable server, got %q", got)
	}
}

// TestHandlerClose_ChannelClosed verifies that after Close(), the stopWatch
// channel is closed and readable (confirming the watcher goroutine can exit).
func TestHandlerClose_ChannelClosed(t *testing.T) {
	logger := testLogger()
	cfg := testConfig()
	cfg.MCP.CatalogRetries = 1

	h := NewHandler(cfg, logger)

	// Close should not panic
	h.Close()

	// Verify channel is closed by attempting to receive from it
	select {
	case <-h.stopWatch:
		// Channel is closed, expected
	default:
		t.Error("expected stopWatch channel to be closed after Close()")
	}

	// Second close should also not panic (idempotent)
	h.Close()
}

// TestRefreshCatalog_ConcurrentAccess runs concurrent reads and writes on the
// handler catalog to detect data races. Run with -race flag.
func TestRefreshCatalog_ConcurrentAccess(t *testing.T) {
	phase := 1
	srv := makeMockCatalogServer(t,
		func() string { return "build-1" },
		func() string {
			if phase == 1 {
				return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]}]`
			}
			return `[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/tool_a","params":[]},{"name":"tool_b","description":"Tool B","method":"GET","path":"/api/tool_b","params":[]}]`
		},
	)
	defer srv.Close()

	h := makeHandlerWithServer(t, srv)
	defer h.Close()

	var wg sync.WaitGroup
	// 10 concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_ = h.Catalog()
			}
		}()
	}
	// 1 writer (RefreshCatalog)
	wg.Add(1)
	go func() {
		defer wg.Done()
		phase = 2
		_, _ = h.RefreshCatalog()
	}()

	wg.Wait()
}
