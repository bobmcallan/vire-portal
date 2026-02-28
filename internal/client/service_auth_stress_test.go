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
	"time"
)

// =============================================================================
// RegisterService: Security & Edge Cases
// =============================================================================

func TestRegisterService_ServiceKeyNotInHeaders(t *testing.T) {
	// SECURITY: The service key MUST only be in the POST body, never in headers.
	// If it appears in headers, it could be captured in access logs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check all headers for the key
		for name, values := range r.Header {
			for _, v := range values {
				if strings.Contains(v, "my-secret-service-key") {
					t.Errorf("service key found in header %s: %s", name, v)
				}
			}
		}

		// Verify body contains the key
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "my-secret-service-key") {
			t.Error("service key should be in the request body")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:test-portal",
			"registered_at":   "2026-02-28T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("test-portal", "my-secret-service-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterService_KeyNotInErrorMessages(t *testing.T) {
	// SECURITY: When the server returns an error, the client error message
	// must NOT include the service key.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"invalid credentials"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("test-portal", "super-secret-key-do-not-expose")
	if err == nil {
		t.Fatal("expected error for 403")
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "super-secret-key-do-not-expose") {
		t.Errorf("SECURITY: service key leaked in error message: %s", errMsg)
	}
}

func TestRegisterService_CorrectEndpointAndMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/services/register" {
			t.Errorf("expected /api/services/register, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:portal-1",
			"registered_at":   "2026-02-28T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal-1", "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterService_CorrectBodyFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		if body["service_id"] != "portal-prod-1" {
			t.Errorf("expected service_id portal-prod-1, got %q", body["service_id"])
		}
		if body["service_key"] != "test-key-123" {
			t.Errorf("expected service_key test-key-123, got %q", body["service_key"])
		}
		if body["service_type"] != "portal" {
			t.Errorf("expected service_type portal, got %q", body["service_type"])
		}
		// Must only have these 3 fields
		if len(body) != 3 {
			t.Errorf("expected exactly 3 fields in body, got %d: %v", len(body), body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:portal-prod-1",
			"registered_at":   "2026-02-28T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	id, err := c.RegisterService("portal-prod-1", "test-key-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "service:portal-prod-1" {
		t.Errorf("expected service_user_id service:portal-prod-1, got %s", id)
	}
}

func TestRegisterService_403Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"invalid service key"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal-1", "wrong-key")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403 status, got: %v", err)
	}
}

func TestRegisterService_501NotImplemented(t *testing.T) {
	// Server hasn't configured service registration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(`{"error":"service registration not configured"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal-1", "key")
	if err == nil {
		t.Fatal("expected error for 501 response")
	}
}

func TestRegisterService_EmptyServiceID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		// Server would reject empty service_id, but client should still send
		if body["service_id"] != "" {
			t.Errorf("expected empty service_id, got %q", body["service_id"])
		}

		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"service_id required"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("", "key")
	if err == nil {
		t.Fatal("expected error for empty service ID")
	}
}

func TestRegisterService_EmptyServiceKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"invalid service key"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal-1", "")
	if err == nil {
		t.Fatal("expected error for empty service key")
	}
}

func TestRegisterService_HostileServiceID(t *testing.T) {
	// Hostile service IDs should not cause crashes or unexpected behavior.
	hostile := []string{
		"../../etc/passwd",
		"<script>alert(1)</script>",
		"portal\r\nX-Injected: evil",
		strings.Repeat("A", 10000),
		"portal\x00null",
		"'; DROP TABLE services; --",
		"portal id with spaces",
	}

	for _, id := range hostile {
		t.Run("hostile_"+id[:min(len(id), 20)], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":          "ok",
					"service_user_id": "service:" + id,
					"registered_at":   "2026-02-28T00:00:00Z",
				})
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL)
			// Must not panic
			c.RegisterService(id, "key")
		})
	}
}

func TestRegisterService_ServerUnreachable(t *testing.T) {
	c := NewVireClient("http://127.0.0.1:1")
	_, err := c.RegisterService("portal", "key")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestRegisterService_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken json`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal", "key")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestRegisterService_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal", "key")
	if err == nil {
		t.Fatal("expected error for empty response body")
	}
}

func TestRegisterService_HTMLErrorPage(t *testing.T) {
	// Reverse proxy returning HTML instead of JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`<html><body>502 Bad Gateway</body></html>`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal", "key")
	if err == nil {
		t.Fatal("expected error for HTML error page")
	}
}

func TestRegisterService_Idempotent(t *testing.T) {
	// Calling RegisterService twice with the same ID should work.
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:portal-1",
			"registered_at":   "2026-02-28T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	id1, err1 := c.RegisterService("portal-1", "key")
	id2, err2 := c.RegisterService("portal-1", "key")

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if id1 != id2 {
		t.Errorf("expected same service_user_id on re-registration, got %q and %q", id1, id2)
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}

func TestRegisterService_ConcurrentRegistrations(t *testing.T) {
	// Multiple portal instances registering simultaneously.
	var mu sync.Mutex
	registrations := make(map[string]int)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		registrations[body["service_id"]]++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:" + body["service_id"],
			"registered_at":   "2026-02-28T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			portalID := fmt.Sprintf("portal-%d", id%5)
			_, err := c.RegisterService(portalID, "key")
			if err != nil {
				t.Errorf("concurrent registration failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	total := 0
	for _, count := range registrations {
		total += count
	}
	if total != 20 {
		t.Errorf("expected 20 total registrations, got %d", total)
	}
}

func TestRegisterService_SlowServer(t *testing.T) {
	// Verify the client doesn't hang forever.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"service_user_id": "service:portal",
			"registered_at":   "2026-02-28T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal", "key")
	if err != nil {
		t.Fatalf("short delay should not cause error: %v", err)
	}
}

func TestRegisterService_LargeResponse(t *testing.T) {
	// Response body capped at 1MB by LimitReader.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service_user_id":"`))
		w.Write([]byte(strings.Repeat("x", 2<<20)))
		w.Write([]byte(`"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.RegisterService("portal", "key")
	// Should fail to parse because response was truncated
	if err == nil {
		t.Log("large response parsed without error (truncated)")
	}
}

// =============================================================================
// AdminListUsers: Security & Edge Cases
// =============================================================================

func TestAdminListUsers_ServiceIDHeader(t *testing.T) {
	// CRITICAL: Verify X-Vire-Service-ID header is correctly set.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/admin/users" {
			t.Errorf("expected /api/admin/users, got %s", r.URL.Path)
		}

		serviceID := r.Header.Get("X-Vire-Service-ID")
		if serviceID != "service:portal-prod-1" {
			t.Errorf("expected X-Vire-Service-ID service:portal-prod-1, got %q", serviceID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []interface{}{},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.AdminListUsers("service:portal-prod-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminListUsers_NoAuthorizationHeader(t *testing.T) {
	// AdminListUsers should use X-Vire-Service-ID, NOT Authorization bearer token.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("AdminListUsers should NOT send Authorization header, got: %s", auth)
		}
		if serviceID := r.Header.Get("X-Vire-Service-ID"); serviceID == "" {
			t.Error("expected X-Vire-Service-ID header to be set")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"users": []interface{}{}})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	c.AdminListUsers("service:portal-1")
}

func TestAdminListUsers_ResponseFormat(t *testing.T) {
	// Response is {"users": [...]} NOT {"status":"ok","data":[...]}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []map[string]interface{}{
				{
					"id":         "alice",
					"email":      "alice@example.com",
					"name":       "Alice",
					"role":       "user",
					"provider":   "google",
					"created_at": "2026-01-15T10:00:00Z",
				},
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	users, err := c.AdminListUsers("service:portal-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].ID != "alice" {
		t.Errorf("expected ID alice, got %s", users[0].ID)
	}
	if users[0].Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", users[0].Email)
	}
}

func TestAdminListUsers_HostileServiceIDInHeader(t *testing.T) {
	// Hostile service ID values should not cause CRLF injection in headers.
	hostile := []string{
		"service:portal\r\nX-Injected: evil",
		"service:portal\nBcc: evil@attacker.com",
		"service:\x00null",
		strings.Repeat("A", 10000),
	}

	for _, id := range hostile {
		t.Run("hostile_"+id[:min(len(id), 20)], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{"users": []interface{}{}})
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL)
			// Must not panic
			c.AdminListUsers(id)
		})
	}
}

func TestAdminListUsers_EmptyServiceID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceID := r.Header.Get("X-Vire-Service-ID")
		if serviceID != "" {
			t.Errorf("expected empty service ID header, got %q", serviceID)
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.AdminListUsers("")
	if err == nil {
		t.Fatal("expected error for empty service ID")
	}
}

func TestAdminListUsers_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.AdminListUsers("service:invalid")
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestAdminListUsers_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.AdminListUsers("service:portal-1")
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestAdminListUsers_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken json`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.AdminListUsers("service:portal-1")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestAdminListUsers_EmptyUserList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"users": []interface{}{}})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	users, err := c.AdminListUsers("service:portal-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestAdminListUsers_HostileUserData(t *testing.T) {
	// Server returns users with hostile field values.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []map[string]interface{}{
				{
					"id":    "../../etc/passwd",
					"email": "'; DROP TABLE users; --",
					"name":  "<script>alert(1)</script>",
					"role":  "admin\r\nX-Inject: evil",
				},
				{
					"id":    strings.Repeat("A", 10000),
					"email": strings.Repeat("B", 10000) + "@example.com",
					"name":  strings.Repeat("C", 10000),
					"role":  strings.Repeat("D", 10000),
				},
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	users, err := c.AdminListUsers("service:portal-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestAdminListUsers_WrongResponseFormat(t *testing.T) {
	// Server returns {"status":"ok","data":[...]} instead of {"users":[...]}
	// This was the old format; the new API uses {"users":[...]}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": []map[string]interface{}{
				{"username": "alice", "email": "alice@example.com", "role": "user"},
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	users, err := c.AdminListUsers("service:portal-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return empty list since "users" key is missing
	if len(users) != 0 {
		t.Errorf("expected 0 users for wrong response format, got %d", len(users))
	}
}

func TestAdminListUsers_ServerUnreachable(t *testing.T) {
	c := NewVireClient("http://127.0.0.1:1")
	_, err := c.AdminListUsers("service:portal-1")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestAdminListUsers_LargeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users":[{"id":"`))
		w.Write([]byte(strings.Repeat("x", 2<<20)))
		w.Write([]byte(`"}]}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.AdminListUsers("service:portal-1")
	if err == nil {
		t.Log("large response parsed without error (truncated)")
	}
}

// =============================================================================
// AdminUpdateUserRole: Security & Edge Cases
// =============================================================================

func TestAdminUpdateUserRole_CorrectEndpointAndMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/admin/users/alice/role" {
			t.Errorf("expected /api/admin/users/alice/role, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		serviceID := r.Header.Get("X-Vire-Service-ID")
		if serviceID != "service:portal-1" {
			t.Errorf("expected X-Vire-Service-ID service:portal-1, got %q", serviceID)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["role"] != "admin" {
			t.Errorf("expected role=admin, got %q", body["role"])
		}
		if len(body) != 1 {
			t.Errorf("expected exactly 1 field in body, got %d: %v", len(body), body)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	err := c.AdminUpdateUserRole("service:portal-1", "alice", "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminUpdateUserRole_ServiceIDHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceID := r.Header.Get("X-Vire-Service-ID")
		if serviceID != "service:my-portal" {
			t.Errorf("expected service:my-portal, got %q", serviceID)
		}
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("should NOT send Authorization header, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	c.AdminUpdateUserRole("service:my-portal", "alice", "admin")
}

func TestAdminUpdateUserRole_PathTraversalInUserID(t *testing.T) {
	// Hostile userIDs should not break the client. Server must validate.
	var receivedPaths []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedPaths = append(receivedPaths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	hostile := []string{
		"../../etc/passwd",
		"../admin",
		"alice?role=superadmin",
		"alice#fragment",
		"alice/../../admin",
		"'; DROP TABLE users; --",
		"alice\r\nX-Injected: evil",
		strings.Repeat("A", 10000),
	}

	for _, id := range hostile {
		// Must not panic
		c.AdminUpdateUserRole("service:portal-1", id, "admin")
	}
}

func TestAdminUpdateUserRole_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	err := c.AdminUpdateUserRole("service:portal-1", "alice", "admin")
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestAdminUpdateUserRole_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"user not found"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	err := c.AdminUpdateUserRole("service:portal-1", "nobody", "admin")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestAdminUpdateUserRole_OnlySetsRoleField(t *testing.T) {
	// SECURITY: Must only send {"role": "admin"}, nothing else.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var fields map[string]interface{}
		json.Unmarshal(body, &fields)

		if len(fields) != 1 {
			t.Errorf("expected exactly 1 field, got %d: %v", len(fields), fields)
		}
		role, ok := fields["role"]
		if !ok {
			t.Error("missing 'role' field")
		}
		if role != "admin" {
			t.Errorf("expected role=admin, got %v", role)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	c.AdminUpdateUserRole("service:portal-1", "alice", "admin")
}

func TestAdminUpdateUserRole_HostileRoleValue(t *testing.T) {
	// What if someone tries to set a hostile role value?
	hostile := []string{
		"superadmin",
		"admin; DROP TABLE roles",
		"<script>alert(1)</script>",
		strings.Repeat("admin", 10000),
	}

	for _, role := range hostile {
		t.Run("role_"+role[:min(len(role), 20)], func(t *testing.T) {
			var receivedRole string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]string
				json.NewDecoder(r.Body).Decode(&body)
				receivedRole = body["role"]
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL)
			c.AdminUpdateUserRole("service:portal-1", "alice", role)

			// Client should pass the role through as-is (server validates)
			if receivedRole != role {
				t.Errorf("expected role %q to be passed through, got %q", role, receivedRole)
			}
		})
	}
}

func TestAdminUpdateUserRole_ConcurrentUpdates(t *testing.T) {
	var updateCount atomic.Int64
	var mu sync.Mutex
	updatedUsers := make(map[string]bool)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		updateCount.Add(1)
		// Extract user ID from path: /api/admin/users/{id}/role
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 5 {
			mu.Lock()
			updatedUsers[parts[4]] = true
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	var wg sync.WaitGroup
	users := []string{"alice", "bob", "charlie", "dave", "eve"}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			user := users[idx%len(users)]
			err := c.AdminUpdateUserRole("service:portal-1", user, "admin")
			if err != nil {
				t.Errorf("concurrent update failed for %s: %v", user, err)
			}
		}(i)
	}
	wg.Wait()

	if updateCount.Load() != 50 {
		t.Errorf("expected 50 updates, got %d", updateCount.Load())
	}
}

func TestAdminUpdateUserRole_ServerUnreachable(t *testing.T) {
	c := NewVireClient("http://127.0.0.1:1")
	err := c.AdminUpdateUserRole("service:portal-1", "alice", "admin")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// =============================================================================
// Cross-Cutting: HTTP Status Code Edge Cases for Admin Methods
// =============================================================================

func TestAdminListUsers_StatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"200 OK", 200, `{"users":[]}`, false},
		{"201 Created", 201, `{"users":[]}`, true},
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

			c := NewVireClient(srv.URL)
			_, err := c.AdminListUsers("service:portal-1")

			if tc.wantErr && err == nil {
				t.Errorf("expected error for status %d", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for status %d: %v", tc.status, err)
			}
		})
	}
}

func TestAdminUpdateUserRole_StatusCodes(t *testing.T) {
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
		{"409 Conflict", 409, `{"error":"conflict"}`, true},
		{"500 Internal Server Error", 500, `{"error":"internal"}`, true},
		{"502 Bad Gateway", 502, `<html>502</html>`, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL)
			err := c.AdminUpdateUserRole("service:portal-1", "alice", "admin")

			if tc.wantErr && err == nil {
				t.Errorf("expected error for status %d", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for status %d: %v", tc.status, err)
			}
		})
	}
}

func TestRegisterService_StatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"200 OK", 200, `{"status":"ok","service_user_id":"service:p","registered_at":"2026-01-01T00:00:00Z"}`, false},
		{"201 Created", 201, `{"status":"ok","service_user_id":"service:p"}`, true},
		{"400 Bad Request", 400, `{"error":"bad request"}`, true},
		{"401 Unauthorized", 401, `{"error":"unauthorized"}`, true},
		{"403 Forbidden", 403, `{"error":"invalid key"}`, true},
		{"409 Conflict", 409, `{"error":"already registered"}`, true},
		{"500 Internal Server Error", 500, `{"error":"internal"}`, true},
		{"501 Not Implemented", 501, `{"error":"not configured"}`, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL)
			_, err := c.RegisterService("portal", "key")

			if tc.wantErr && err == nil {
				t.Errorf("expected error for status %d", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for status %d: %v", tc.status, err)
			}
		})
	}
}
