package seed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// =============================================================================
// ReportPortalVersion (seed layer): Retry, Security & Edge Cases
// =============================================================================

func TestReportPortalVersion_Stress_Success(t *testing.T) {
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if r.URL.Path != "/api/portal/version" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount.Load() != 1 {
		t.Errorf("expected 1 request on success, got %d", requestCount.Load())
	}
}

func TestReportPortalVersion_Stress_AllRetriesFail(t *testing.T) {
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"permanent failure"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", logger)
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
	if requestCount.Load() != int64(seedRetryAttempts) {
		t.Errorf("expected %d attempts, got %d", seedRetryAttempts, requestCount.Load())
	}
}

func TestReportPortalVersion_Stress_ServerDown(t *testing.T) {
	// Use a closed server to avoid WSL2 connection hanging.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", logger)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestReportPortalVersion_NilLogger(t *testing.T) {
	// Nil logger should not cause panic.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	err := ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportPortalVersion_NilLoggerAllFail(t *testing.T) {
	// Nil logger on failure path should not panic.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	err := ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", nil)
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
}

func TestReportPortalVersion_EmptyServiceID(t *testing.T) {
	// Empty service ID should be sent through — server decides if valid.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vire-Service-ID") != "" {
			t.Errorf("expected empty service ID header, got %q", r.Header.Get("X-Vire-Service-ID"))
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "", "1.2.3", "20260308-120000", logger)
	if err == nil {
		t.Fatal("expected error for empty service ID")
	}
}

func TestReportPortalVersion_EmptyVersionBuild(t *testing.T) {
	// Empty version and build — should still POST.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["version"] != "" || body["build"] != "" {
			t.Errorf("expected empty version/build, got %q/%q", body["version"], body["build"])
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-1", "", "", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportPortalVersion_HostileServiceID(t *testing.T) {
	// CRLF injection in service ID.
	hostile := []string{
		"service:portal\r\nX-Injected: evil",
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
				if r.Header.Get("X-Injected") != "" {
					t.Error("SECURITY: CRLF injection via service ID in seed layer")
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer srv.Close()

			logger := testLogger(t)
			// Must not panic
			ReportPortalVersion(srv.URL, id, "1.0.0", "20260308", logger)
		})
	}
}

func TestReportPortalVersion_HostileVersionBuild(t *testing.T) {
	// Hostile version/build values in POST body.
	hostile := []struct {
		version string
		build   string
	}{
		{`"injected":"evil"`, "20260308"},
		{"<script>alert(1)</script>", "20260308"},
		{strings.Repeat("A", 100000), strings.Repeat("B", 100000)},
	}

	for i, tc := range hostile {
		t.Run(strings.Repeat("h", min(i+1, 5)), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := json.Marshal(nil)
				json.NewDecoder(r.Body).Decode(&body)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer srv.Close()

			logger := testLogger(t)
			// Must not panic
			ReportPortalVersion(srv.URL, "service:portal", tc.version, tc.build, logger)
		})
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

	logger := testLogger(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", logger)
		}()
	}
	wg.Wait()

	if reportCount.Load() != 10 {
		t.Errorf("expected 10 reports, got %d", reportCount.Load())
	}
}

func TestReportPortalVersion_SeedClientNoVersionHeaders(t *testing.T) {
	// IMPORTANT: The seed layer creates VireClient with empty version/build,
	// so the version report POST should NOT have X-Portal-Version/Build headers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Portal-Version") != "" {
			t.Error("seed client should NOT inject X-Portal-Version header")
		}
		if r.Header.Get("X-Portal-Build") != "" {
			t.Error("seed client should NOT inject X-Portal-Build header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportPortalVersion_403DoesRetry(t *testing.T) {
	// 403 means auth failure — retrying won't help.
	// Document: current retry behavior retries all errors.
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	ReportPortalVersion(srv.URL, "service:portal-1", "1.2.3", "20260308-120000", logger)

	count := requestCount.Load()
	if count < 1 {
		t.Error("expected at least 1 attempt")
	}
	// Document: retries all errors including 403
	if count != int64(seedRetryAttempts) {
		t.Errorf("expected %d retry attempts for 403, got %d", seedRetryAttempts, count)
	}
}

func TestReportPortalVersion_ErrorDoesNotLeakServiceID(t *testing.T) {
	// SECURITY: Error message should not contain the service user ID.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:ultra-secret-portal-id", "1.2.3", "20260308-120000", logger)
	if err == nil {
		t.Fatal("expected error")
	}

	// The error message should contain the failure reason, not the service ID
	errMsg := err.Error()
	if strings.Contains(errMsg, "ultra-secret-portal-id") {
		// Note: the error from client includes status code and server response body,
		// not the service ID. This test verifies that.
		t.Errorf("SECURITY: service ID leaked in error message: %s", errMsg)
	}
}
