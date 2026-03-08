package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// =============================================================================
// versionTransport: Header Injection & Edge Cases
// =============================================================================

func TestVersionHeaders_CRLFInjection(t *testing.T) {
	// SECURITY: Version/build strings with CRLF should not inject extra headers.
	// Go's net/http sanitizes header values, but verify explicitly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that no injected header appears
		if r.Header.Get("X-Injected") != "" {
			t.Error("SECURITY: CRLF injection succeeded — X-Injected header found")
		}
		if r.Header.Get("Bcc") != "" {
			t.Error("SECURITY: CRLF injection succeeded — Bcc header found")
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	hostile := []struct {
		name    string
		version string
		build   string
	}{
		{"crlf_version", "1.0.0\r\nX-Injected: evil", "20260308"},
		{"crlf_build", "1.0.0", "20260308\r\nX-Injected: evil"},
		{"newline_version", "1.0.0\nBcc: evil@attacker.com", "20260308"},
		{"null_version", "1.0.0\x00injected", "20260308"},
		{"tab_version", "1.0.0\tinjected", "20260308"},
	}

	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			c := NewVireClient(srv.URL, tc.version, tc.build)
			// Must not panic
			c.ProxyGet("/api/glossary", "")
		})
	}
}

func TestVersionHeaders_VeryLongStrings(t *testing.T) {
	// Edge case: extremely long version/build strings.
	// Should not crash or cause memory issues.
	var receivedVersionLen, receivedBuildLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedVersionLen = len(r.Header.Get("X-Portal-Version"))
		receivedBuildLen = len(r.Header.Get("X-Portal-Build"))
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	longVersion := strings.Repeat("1", 100000)
	longBuild := strings.Repeat("2", 100000)

	c := NewVireClient(srv.URL, longVersion, longBuild)
	_, err := c.ProxyGet("/api/glossary", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Headers should be passed through (server-side should enforce limits)
	if receivedVersionLen == 0 {
		t.Error("expected version header to be sent")
	}
	if receivedBuildLen == 0 {
		t.Error("expected build header to be sent")
	}
}

func TestVersionHeaders_SpecialCharacters(t *testing.T) {
	// Hostile version strings should not cause panics.
	hostile := []string{
		"<script>alert(1)</script>",
		"'; DROP TABLE versions; --",
		"../../etc/passwd",
		"$(whoami)",
		"`command`",
		strings.Repeat("A", 10000),
		"",
		" ",
		"v1.0.0-beta+build.123",
		"version with spaces",
	}

	for _, v := range hostile {
		name := v
		if len(name) > 20 {
			name = name[:20]
		}
		t.Run("version_"+name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL, v, v)
			// Must not panic
			c.ProxyGet("/api/glossary", "")
		})
	}
}

func TestVersionHeaders_PresentOnAllMethods(t *testing.T) {
	// Verify version headers are injected on GET, POST, PUT, PATCH, not just ProxyGet.
	var headersSeen atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Portal-Version") == "3.0.0" {
			headersSeen.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/users/alice" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"data":   map[string]interface{}{"username": "alice", "email": "a@b.com", "role": "user"},
			})
		case r.URL.Path == "/api/services/register" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok", "service_user_id": "service:p", "registered_at": "2026-01-01T00:00:00Z",
			})
		case r.URL.Path == "/api/portal/version" && r.Method == http.MethodPost:
			w.Write([]byte(`{"status":"ok"}`))
		case r.URL.Path == "/api/admin/users/alice/role" && r.Method == http.MethodPatch:
			w.Write([]byte(`{"status":"ok"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "3.0.0", "20260308-000000")

	// Exercise multiple methods
	c.ProxyGet("/api/glossary", "")
	c.GetUser("alice")
	c.RegisterService("portal", "key")
	c.ReportPortalVersion("service:p", "3.0.0", "20260308-000000")
	c.AdminUpdateUserRole("service:p", "alice", "admin")

	if headersSeen.Load() != 5 {
		t.Errorf("expected version header on all 5 requests, saw it on %d", headersSeen.Load())
	}
}

func TestVersionHeaders_DoNotOverrideExplicitHeaders(t *testing.T) {
	// Version headers from transport should not interfere with explicit headers
	// like X-Vire-Service-ID or X-Vire-User-ID.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Version headers present
		if r.Header.Get("X-Portal-Version") != "1.0.0" {
			t.Errorf("missing X-Portal-Version")
		}
		// Explicit header also present
		if r.Header.Get("X-Vire-User-ID") != "alice" {
			t.Errorf("X-Vire-User-ID should be alice, got %s", r.Header.Get("X-Vire-User-ID"))
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "1.0.0", "20260308")
	c.ProxyGet("/api/glossary", "alice")
}

func TestVersionHeaders_ConcurrentRequests(t *testing.T) {
	// Multiple goroutines using the same client should not cause data races.
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		v := r.Header.Get("X-Portal-Version")
		b := r.Header.Get("X-Portal-Build")
		if v != "concurrent-v" {
			t.Errorf("expected concurrent-v, got %s", v)
		}
		if b != "concurrent-b" {
			t.Errorf("expected concurrent-b, got %s", b)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "concurrent-v", "concurrent-b")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.ProxyGet("/api/glossary", "")
		}()
	}
	wg.Wait()

	if requestCount.Load() != 50 {
		t.Errorf("expected 50 requests, got %d", requestCount.Load())
	}
}

func TestVersionHeaders_OnlyVersionSet(t *testing.T) {
	// Version set but build empty — only X-Portal-Version should appear.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Portal-Version") != "1.0.0" {
			t.Errorf("expected X-Portal-Version=1.0.0, got %s", r.Header.Get("X-Portal-Version"))
		}
		if r.Header.Get("X-Portal-Build") != "" {
			t.Errorf("expected no X-Portal-Build when build is empty, got %s", r.Header.Get("X-Portal-Build"))
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "1.0.0", "")
	c.ProxyGet("/api/glossary", "")
}

func TestVersionHeaders_OnlyBuildSet(t *testing.T) {
	// Build set but version empty — only X-Portal-Build should appear.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Portal-Version") != "" {
			t.Errorf("expected no X-Portal-Version when version is empty, got %s", r.Header.Get("X-Portal-Version"))
		}
		if r.Header.Get("X-Portal-Build") != "20260308" {
			t.Errorf("expected X-Portal-Build=20260308, got %s", r.Header.Get("X-Portal-Build"))
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "20260308")
	c.ProxyGet("/api/glossary", "")
}

// =============================================================================
// ReportPortalVersion: Security & Edge Cases
// =============================================================================

func TestReportPortalVersion_CorrectEndpointAndMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/portal/version" {
			t.Errorf("expected /api/portal/version, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	err := c.ReportPortalVersion("service:portal-1", "1.0.0", "20260308")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportPortalVersion_CorrectBodyFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		if body["version"] != "1.2.3" {
			t.Errorf("expected version=1.2.3, got %q", body["version"])
		}
		if body["build"] != "20260308-120000" {
			t.Errorf("expected build=20260308-120000, got %q", body["build"])
		}
		// Must only have these 2 fields
		if len(body) != 2 {
			t.Errorf("expected exactly 2 fields in body, got %d: %v", len(body), body)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	c.ReportPortalVersion("service:portal-1", "1.2.3", "20260308-120000")
}

func TestReportPortalVersion_ServiceIDInHeader(t *testing.T) {
	// CRITICAL: X-Vire-Service-ID must be set correctly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceID := r.Header.Get("X-Vire-Service-ID")
		if serviceID != "service:my-portal" {
			t.Errorf("expected service:my-portal, got %q", serviceID)
		}
		// Should NOT use Authorization header
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("should NOT send Authorization header, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	c.ReportPortalVersion("service:my-portal", "1.0.0", "20260308")
}

func TestReportPortalVersion_EmptyServiceID(t *testing.T) {
	// Empty service ID — server should reject, client should propagate error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vire-Service-ID") != "" {
			t.Errorf("expected empty service ID header, got %q", r.Header.Get("X-Vire-Service-ID"))
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	err := c.ReportPortalVersion("", "1.0.0", "20260308")
	if err == nil {
		t.Fatal("expected error for empty service ID")
	}
}

func TestReportPortalVersion_EmptyVersionAndBuild(t *testing.T) {
	// Empty version/build strings — should still POST (server decides if valid).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		if body["version"] != "" {
			t.Errorf("expected empty version, got %q", body["version"])
		}
		if body["build"] != "" {
			t.Errorf("expected empty build, got %q", body["build"])
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	err := c.ReportPortalVersion("service:portal", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportPortalVersion_HostileServiceID(t *testing.T) {
	// CRLF injection in service ID header.
	hostile := []string{
		"service:portal\r\nX-Injected: evil",
		"service:portal\nBcc: evil@attacker.com",
		"service:\x00null",
		strings.Repeat("A", 10000),
		"'; DROP TABLE services; --",
	}

	for _, id := range hostile {
		name := id
		if len(name) > 20 {
			name = name[:20]
		}
		t.Run("hostile_"+name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check no injected headers
				if r.Header.Get("X-Injected") != "" {
					t.Error("SECURITY: CRLF injection via service ID")
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL, "", "")
			// Must not panic
			c.ReportPortalVersion(id, "1.0.0", "20260308")
		})
	}
}

func TestReportPortalVersion_HostileVersionBuild(t *testing.T) {
	// Hostile values in POST body — JSON-encoded so injection is unlikely,
	// but verify the body is valid JSON.
	hostile := []struct {
		version string
		build   string
	}{
		{`"injected":"evil"`, "20260308"},
		{"1.0.0", `"injected":"evil"`},
		{`","extra":"field`, "20260308"},
		{strings.Repeat("A", 100000), strings.Repeat("B", 100000)},
		{"<script>alert(1)</script>", "20260308"},
	}

	for i, tc := range hostile {
		t.Run(fmt.Sprintf("hostile_%d", i), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var parsed map[string]interface{}
				if err := json.Unmarshal(body, &parsed); err != nil {
					t.Errorf("POST body is not valid JSON: %v", err)
				}
				// Must only have version and build keys
				if len(parsed) != 2 {
					t.Errorf("expected 2 fields, got %d: %v", len(parsed), parsed)
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL, "", "")
			// Must not panic
			c.ReportPortalVersion("service:portal", tc.version, tc.build)
		})
	}
}

func TestReportPortalVersion_StatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"200 OK", 200, `{"status":"ok"}`, false},
		{"201 Created", 201, `{"status":"ok"}`, true},
		{"400 Bad Request", 400, `{"error":"bad"}`, true},
		{"401 Unauthorized", 401, `{"error":"unauthorized"}`, true},
		{"403 Forbidden", 403, `{"error":"forbidden"}`, true},
		{"404 Not Found", 404, `{"error":"not found"}`, true},
		{"500 Internal Server Error", 500, `{"error":"internal"}`, true},
		{"502 Bad Gateway", 502, `<html>502</html>`, true},
		{"503 Service Unavailable", 503, ``, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL, "", "")
			err := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")

			if tc.wantErr && err == nil {
				t.Errorf("expected error for status %d", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for status %d: %v", tc.status, err)
			}
		})
	}
}

func TestReportPortalVersion_ServerUnreachable(t *testing.T) {
	c := NewVireClient("http://127.0.0.1:1", "", "")
	err := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestReportPortalVersion_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken json`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	// 200 with malformed body — should succeed since we don't parse response body
	err := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")
	if err != nil {
		t.Fatalf("unexpected error — 200 response should succeed regardless of body: %v", err)
	}
}

func TestReportPortalVersion_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	err := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")
	if err != nil {
		t.Fatalf("unexpected error — 200 with empty body should succeed: %v", err)
	}
}

func TestReportPortalVersion_HTMLErrorPage(t *testing.T) {
	// Reverse proxy returning HTML instead of JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`<html><body>502 Bad Gateway</body></html>`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	err := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")
	if err == nil {
		t.Fatal("expected error for HTML error page")
	}
}

func TestReportPortalVersion_ConcurrentReports(t *testing.T) {
	var reportCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reportCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := c.ReportPortalVersion(
				fmt.Sprintf("service:portal-%d", n),
				fmt.Sprintf("%d.0.0", n),
				"20260308",
			)
			if err != nil {
				t.Errorf("concurrent report %d failed: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	if reportCount.Load() != 20 {
		t.Errorf("expected 20 reports, got %d", reportCount.Load())
	}
}

func TestReportPortalVersion_LargeResponse(t *testing.T) {
	// Response body capped at 1MB by LimitReader.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(strings.Repeat("x", 2<<20)))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")
	// 200 status — should succeed even with large body since we don't parse it
	err := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")
	if err != nil {
		t.Fatalf("unexpected error for large response: %v", err)
	}
}

func TestReportPortalVersion_Idempotent(t *testing.T) {
	// Calling ReportPortalVersion multiple times should work.
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "", "")

	err1 := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")
	err2 := c.ReportPortalVersion("service:portal", "1.0.0", "20260308")

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}

// =============================================================================
// versionTransport: RoundTripper Contract
// =============================================================================

func TestVersionTransport_NilBaseTransport(t *testing.T) {
	// If somehow base is nil, RoundTrip should panic (expected Go behavior).
	// This documents that NewVireClient always sets http.DefaultTransport.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil base transport")
		}
	}()

	vt := &versionTransport{
		base:    nil,
		version: "1.0.0",
		build:   "20260308",
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	vt.RoundTrip(req)
}

func TestVersionTransport_RequestHeadersNotCorrupted(t *testing.T) {
	// Verify that after RoundTrip, the request headers contain version headers.
	// This tests the RoundTripper contract — req.Header.Set modifies in place.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL, "check-me", "build-check")
	req, _ := http.NewRequest("GET", srv.URL+"/api/test", nil)
	req.Header.Set("X-Custom", "preserved")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	// Original request's custom header should still be present
	if req.Header.Get("X-Custom") != "preserved" {
		t.Error("custom header was not preserved after RoundTrip")
	}
	// Version headers should have been set on the request
	if req.Header.Get("X-Portal-Version") != "check-me" {
		t.Error("version header not set on request after RoundTrip")
	}
}
