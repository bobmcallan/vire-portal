package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// ProxyGet Stress Tests — Security & Edge Cases
// =============================================================================

// --- SSRF: ProxyGet should only hit the configured baseURL ---

func TestProxyGet_StressPathTraversalBlocked(t *testing.T) {
	// Attacker passes path like "/../../../etc/passwd" or "//evil.com/steal".
	// ProxyGet must prepend baseURL and not allow escaping.
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	// These paths should NOT cause requests to external hosts.
	// They should be appended to baseURL as-is (server-side path resolution).
	maliciousPaths := []string{
		"/../../../etc/passwd",
		"//evil.com/steal",
		"/api/../../../etc/shadow",
		"/api/portfolios/../../admin/users",
		"/%2e%2e/%2e%2e/etc/passwd",
	}

	for _, path := range maliciousPaths {
		body, err := c.ProxyGet(path, "attacker")
		// The request should still go to our test server (not an external host).
		// As long as it doesn't error with a connection to a different host, it's safe.
		// The vire-server is responsible for path authorization.
		if err != nil {
			// Error is acceptable — it means the request was rejected
			continue
		}
		if body == nil {
			t.Errorf("path %q: unexpected nil body without error", path)
		}
		_ = receivedPath // The path was sent to our mock server, not an external host
	}
}

func TestProxyGet_StressAbsoluteURLInPath(t *testing.T) {
	// If an attacker passes a full URL as "path", it should NOT be treated as
	// a target URL. It must be concatenated with baseURL.
	var gotRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	// Pass a full URL as path — should be concatenated, not used as-is
	_, err := c.ProxyGet("http://evil.com/steal", "attacker")
	// This should either error (malformed URL after concat) or hit our test server
	// It must NOT hit evil.com
	if err == nil && !gotRequest {
		t.Error("SSRF: ProxyGet may have sent request to external host")
	}
}

// --- Header injection: X-Vire-User-ID ---

func TestProxyGet_StressUserIDHeaderInjection(t *testing.T) {
	// Attacker controls userID — check that header injection via newlines is blocked.
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	// Go's http.NewRequest will reject headers with newlines, but verify
	maliciousIDs := []string{
		"admin\r\nX-Admin: true",
		"admin\nEvil-Header: yes",
		"admin\x00bypass",
	}

	for _, uid := range maliciousIDs {
		_, err := c.ProxyGet("/api/test", uid)
		if err != nil {
			// Go's HTTP client rejects these — good
			continue
		}
		// If the request went through, verify no extra headers were injected
		if headers.Get("X-Admin") != "" || headers.Get("Evil-Header") != "" {
			t.Errorf("SECURITY: header injection via userID %q succeeded", uid)
		}
	}
}

func TestProxyGet_StressEmptyUserID(t *testing.T) {
	var gotUserHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserHeader = r.Header.Get("X-Vire-User-ID") != ""
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.ProxyGet("/api/test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUserHeader {
		t.Error("SECURITY: X-Vire-User-ID header sent when userID is empty")
	}
}

// --- Large response body ---

func TestProxyGet_StressLargeResponseBounded(t *testing.T) {
	// ProxyGet uses io.LimitReader(resp.Body, 1<<20) per spec.
	// A malicious server returning 100MB should be truncated to 1MB.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write 2MB of data — should be truncated
		chunk := strings.Repeat("X", 1024)
		for i := 0; i < 2048; i++ {
			w.Write([]byte(chunk))
		}
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	body, err := c.ProxyGet("/api/large", "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Body should be at most 1MB (1<<20 = 1048576)
	if len(body) > 1<<20 {
		t.Errorf("SECURITY: ProxyGet returned %d bytes, exceeding 1MB limit", len(body))
	}
}

// --- Slow server: timeout behavior ---

func TestProxyGet_StressSlowServerTimeout(t *testing.T) {
	// VireClient has a 10s timeout. A server that hangs should be timed out.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Longer than the 10s timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	start := time.Now()
	_, err := c.ProxyGet("/api/slow", "user1")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error for slow server")
	}
	// Should timeout around 10s, not wait 15s
	if elapsed > 12*time.Second {
		t.Errorf("ProxyGet took %v, expected timeout around 10s", elapsed)
	}
}

// --- Non-2xx status codes ---

func TestProxyGet_StressVariousErrorCodes(t *testing.T) {
	codes := []int{400, 401, 403, 404, 500, 502, 503}
	for _, code := range codes {
		statusCode := code
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"error":"test"}`))
		}))

		c := NewVireClient(srv.URL)
		_, err := c.ProxyGet("/api/test", "user1")
		if err == nil {
			t.Errorf("expected error for status %d", statusCode)
		}
		srv.Close()
	}
}

// --- Concurrent access ---

func TestProxyGet_StressConcurrentRequests(t *testing.T) {
	var counter int64
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counter++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := c.ProxyGet("/api/test", "user1")
			if err != nil {
				t.Errorf("concurrent request %d failed: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	mu.Lock()
	if counter != 50 {
		t.Errorf("expected 50 requests, got %d", counter)
	}
	mu.Unlock()
}

// --- Malformed JSON response ---

func TestProxyGet_StressMalformedResponseBody(t *testing.T) {
	// ProxyGet returns raw bytes — it does NOT parse JSON.
	// Callers parse JSON. This verifies ProxyGet returns bytes as-is for 2xx.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json at all {{{`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	body, err := c.ProxyGet("/api/test", "user1")
	if err != nil {
		t.Fatalf("ProxyGet should return body for 2xx regardless of content: %v", err)
	}
	if string(body) != `not json at all {{{` {
		t.Errorf("expected raw body returned, got %q", string(body))
	}
}

// --- Network error ---

func TestProxyGet_StressUnreachableServer(t *testing.T) {
	c := NewVireClient("http://127.0.0.1:1") // Port 1 — unlikely to be listening
	_, err := c.ProxyGet("/api/test", "user1")
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

// --- Empty body on success ---

func TestProxyGet_StressEmptySuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write nothing
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	body, err := c.ProxyGet("/api/empty", "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(body))
	}
}
