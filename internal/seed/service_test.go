package seed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRegisterService_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/register" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:portal-prod-1",
			"registered_at":   "2026-02-28T10:00:00Z",
		})
	}))
	defer srv.Close()

	logger := testLogger(t)
	serviceUserID, err := RegisterService(srv.URL, "portal-prod-1", "my-secret", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serviceUserID != "service:portal-prod-1" {
		t.Errorf("expected service:portal-prod-1, got %s", serviceUserID)
	}
}

func TestRegisterService_RetryOnFailure(t *testing.T) {
	var attempts atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"not ready"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:portal-prod-1",
		})
	}))
	defer srv.Close()

	logger := testLogger(t)
	serviceUserID, err := RegisterService(srv.URL, "portal-prod-1", "my-secret", logger)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if serviceUserID != "service:portal-prod-1" {
		t.Errorf("expected service:portal-prod-1, got %s", serviceUserID)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestRegisterService_AllRetriesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	_, err := RegisterService(srv.URL, "portal-prod-1", "wrong-key", logger)
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}
}
