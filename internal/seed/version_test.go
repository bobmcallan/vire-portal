package seed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestReportPortalVersion_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/portal/version" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Vire-Service-ID") != "service:portal-prod-1" {
			t.Errorf("expected X-Vire-Service-ID header, got %s", r.Header.Get("X-Vire-Service-ID"))
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body["version"] != "1.2.3" {
			t.Errorf("expected version=1.2.3, got %s", body["version"])
		}
		if body["build"] != "20260308-120000" {
			t.Errorf("expected build=20260308-120000, got %s", body["build"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-prod-1", "1.2.3", "20260308-120000", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportPortalVersion_RetryOnFailure(t *testing.T) {
	var attempts atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"not ready"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-prod-1", "1.2.3", "20260308-120000", logger)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestReportPortalVersion_AllRetriesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	err := ReportPortalVersion(srv.URL, "service:portal-prod-1", "1.2.3", "20260308-120000", logger)
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}
}
