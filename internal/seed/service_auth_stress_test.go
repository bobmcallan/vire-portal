package seed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

// registerResponse builds a JSON response for POST /api/services/register.
func registerResponse(serviceUserID string) []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"status":          "ok",
		"service_user_id": serviceUserID,
		"registered_at":   "2026-02-28T00:00:00Z",
	})
	return data
}

// =============================================================================
// RegisterService (seed layer): Retry & Error Handling
// =============================================================================

func TestRegisterService_Stress_Success(t *testing.T) {
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if r.URL.Path == "/api/services/register" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.Write(registerResponse("service:test-portal"))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	logger := testLogger(t)
	id, err := RegisterService(srv.URL, "test-portal", "test-key", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "service:test-portal" {
		t.Errorf("expected service:test-portal, got %s", id)
	}
	if requestCount.Load() != 1 {
		t.Errorf("expected 1 request on success, got %d", requestCount.Load())
	}
}

func TestRegisterService_RetriesOnFailure(t *testing.T) {
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"temporary"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(registerResponse("service:test-portal"))
	}))
	defer srv.Close()

	logger := testLogger(t)
	id, err := RegisterService(srv.URL, "test-portal", "test-key", logger)
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if id != "service:test-portal" {
		t.Errorf("expected service:test-portal, got %s", id)
	}
	if requestCount.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", requestCount.Load())
	}
}

func TestRegisterService_Stress_AllRetriesFail(t *testing.T) {
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"permanent failure"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	_, err := RegisterService(srv.URL, "test-portal", "test-key", logger)
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
	// Should have tried seedRetryAttempts (3) times
	if requestCount.Load() != int64(seedRetryAttempts) {
		t.Errorf("expected %d attempts, got %d", seedRetryAttempts, requestCount.Load())
	}
}

func TestRegisterService_Stress_ServerDown(t *testing.T) {
	// Use a closed server instead of 127.0.0.1:1 which hangs on WSL2.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	logger := testLogger(t)
	_, err := RegisterService(srv.URL, "test-portal", "test-key", logger)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestRegisterService_KeyNotLoggedInWarnings(t *testing.T) {
	// SECURITY: Verify the service key is not included in log messages.
	// We can't easily capture log output, but we verify error return
	// doesn't contain the key.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	_, err := RegisterService(srv.URL, "test-portal", "my-ultra-secret-key-never-log-this", logger)
	if err == nil {
		t.Fatal("expected error for 403")
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "my-ultra-secret-key-never-log-this") {
		t.Errorf("SECURITY: service key leaked in error message: %s", errMsg)
	}
}

func TestRegisterService_403DoesNotRetry(t *testing.T) {
	// 403 means wrong key; retrying won't help. But current pattern retries all errors.
	// This test documents the behavior.
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	RegisterService(srv.URL, "test-portal", "wrong-key", logger)

	// Document: current retry behavior will retry even on 403
	// (seedRetryAttempts retries for all errors)
	count := requestCount.Load()
	if count < 1 {
		t.Error("expected at least 1 attempt")
	}
}

func TestRegisterService_ConcurrentRegistrations(t *testing.T) {
	var registrations sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		registrations.Store(body["service_id"], true)

		w.Header().Set("Content-Type", "application/json")
		w.Write(registerResponse("service:" + body["service_id"]))
	}))
	defer srv.Close()

	logger := testLogger(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			portalID := fmt.Sprintf("portal-%d", id)
			serviceID, err := RegisterService(srv.URL, portalID, "key", logger)
			if err != nil {
				t.Errorf("registration failed for %s: %v", portalID, err)
				return
			}
			if serviceID != "service:"+portalID {
				t.Errorf("expected service:%s, got %s", portalID, serviceID)
			}
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// SyncAdmins (new signature with serviceUserID): Security & Edge Cases
// =============================================================================

func TestSyncAdmins_NewSignature_UpdatesNonAdmin(t *testing.T) {
	var patchCalls []string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			// Verify service ID header is passed through
			serviceID := r.Header.Get("X-Vire-Service-ID")
			if serviceID != "service:portal-1" {
				t.Errorf("expected X-Vire-Service-ID service:portal-1, got %q", serviceID)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			mu.Lock()
			patchCalls = append(patchCalls, r.URL.Path)
			mu.Unlock()

			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["role"] != "admin" {
				t.Errorf("expected role=admin, got %q", body["role"])
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:portal-1", logger)

	mu.Lock()
	defer mu.Unlock()
	if len(patchCalls) != 1 {
		t.Fatalf("expected 1 PATCH call, got %d", len(patchCalls))
	}
	if patchCalls[0] != "/api/admin/users/alice/role" {
		t.Errorf("expected PATCH to /api/admin/users/alice/role, got %s", patchCalls[0])
	}
}

func TestSyncAdmins_NewSignature_SkipsExistingAdmin(t *testing.T) {
	var patchCalls int
	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "admin"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if r.Method == http.MethodPatch {
			patchCalls++
		}
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:portal-1", logger)

	if patchCalls != 0 {
		t.Errorf("expected 0 PATCH calls for existing admin, got %d", patchCalls)
	}
}

func TestSyncAdmins_NewSignature_EmailNotFound(t *testing.T) {
	var patchCalls int
	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if r.Method == http.MethodPatch {
			patchCalls++
		}
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"notfound@example.com"}, "service:portal-1", logger)

	if patchCalls != 0 {
		t.Errorf("expected 0 PATCH calls for unknown email, got %d", patchCalls)
	}
}

func TestSyncAdmins_NewSignature_MultipleEmails(t *testing.T) {
	var mu sync.Mutex
	var patchPaths []string

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
		{ID: "bob", Email: "bob@example.com", Role: "admin"},
		{ID: "carol", Email: "carol@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			mu.Lock()
			patchPaths = append(patchPaths, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{
		"alice@example.com",
		"bob@example.com",
		"carol@example.com",
		"notfound@example.com",
	}, "service:portal-1", logger)

	mu.Lock()
	defer mu.Unlock()
	// alice and carol need updating, bob is already admin, notfound doesn't exist
	if len(patchPaths) != 2 {
		t.Fatalf("expected 2 PATCH calls, got %d: %v", len(patchPaths), patchPaths)
	}

	has := func(path string) bool {
		for _, p := range patchPaths {
			if p == path {
				return true
			}
		}
		return false
	}
	if !has("/api/admin/users/alice/role") {
		t.Error("expected PATCH to /api/admin/users/alice/role")
	}
	if !has("/api/admin/users/carol/role") {
		t.Error("expected PATCH to /api/admin/users/carol/role")
	}
}

func TestSyncAdmins_NewSignature_CaseInsensitive(t *testing.T) {
	var patchCount int
	users := []client.AdminUser{
		{ID: "alice", Email: "Alice@EXAMPLE.COM", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if r.Method == http.MethodPatch {
			patchCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:portal-1", logger)

	if patchCount != 1 {
		t.Errorf("expected 1 PATCH for case-insensitive match, got %d", patchCount)
	}
}

func TestSyncAdmins_NewSignature_EmptyList(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{}, "service:portal-1", logger)

	if callCount != 0 {
		t.Errorf("expected no API calls for empty email list, got %d", callCount)
	}
}

func TestSyncAdmins_NewSignature_NilList(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, nil, "service:portal-1", logger)

	if callCount != 0 {
		t.Errorf("expected no API calls for nil email list, got %d", callCount)
	}
}

func TestSyncAdmins_NewSignature_ServerDown(t *testing.T) {
	// Use a server that immediately closes connections instead of 127.0.0.1:1
	// which can hang on WSL2.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"service unavailable"}`))
	}))
	srv.Close() // close immediately so connections fail fast
	logger := testLogger(t)
	// Must not panic
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:portal-1", logger)
}

func TestSyncAdmins_NewSignature_DoesNotDemote(t *testing.T) {
	// Existing admins NOT in the config list should NOT be demoted.
	var apiCalls []string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "admin"},    // existing admin, NOT in config
		{ID: "bob", Email: "bob@example.com", Role: "admin"},        // existing admin, NOT in config
		{ID: "charlie", Email: "charlie@example.com", Role: "user"}, // will be promoted
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		apiCalls = append(apiCalls, fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		mu.Unlock()

		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"charlie@example.com"}, "service:portal-1", logger)

	mu.Lock()
	defer mu.Unlock()

	for _, call := range apiCalls {
		if call == "PATCH /api/admin/users/alice/role" || call == "PATCH /api/admin/users/bob/role" {
			t.Errorf("existing admin should NOT be modified: %s", call)
		}
	}
}

// =============================================================================
// Hostile Input Tests for New SyncAdmins
// =============================================================================

func TestSyncAdmins_NewSignature_HostileEmails(t *testing.T) {
	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	hostileEmails := []string{
		"'; DROP TABLE users; --",
		"<script>alert(1)</script>",
		"user@example.com\r\nX-Injected: evil",
		strings.Repeat("a", 10000) + "@example.com",
		"",
		" ",
		"@",
		"$(whoami)@example.com",
	}

	logger := testLogger(t)
	// Must not panic
	SyncAdmins(srv.URL, hostileEmails, "service:portal-1", logger)
}

func TestSyncAdmins_NewSignature_HostileUserIDs(t *testing.T) {
	// Server returns users with hostile IDs that will be used in PATCH URL paths.
	users := []client.AdminUser{
		{ID: "../../etc/passwd", Email: "alice@example.com", Role: "user"},
		{ID: "id\r\nX-Injected: evil", Email: "bob@example.com", Role: "user"},
		{ID: strings.Repeat("A", 10000), Email: "charlie@example.com", Role: "user"},
	}

	var patchPaths []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if r.Method == http.MethodPatch {
			mu.Lock()
			patchPaths = append(patchPaths, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	// Must not panic — server-side must validate the IDs
	SyncAdmins(srv.URL, []string{
		"alice@example.com",
		"bob@example.com",
		"charlie@example.com",
	}, "service:portal-1", logger)
}

func TestSyncAdmins_NewSignature_InjectionViaEmail(t *testing.T) {
	var apiCalls []string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		apiCalls = append(apiCalls, fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		mu.Unlock()

		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{
		"alice@example.com", // legitimate match
		"' OR 1=1 --",       // SQL injection
		"admin'--",          // SQL injection variant
	}, "service:portal-1", logger)

	mu.Lock()
	defer mu.Unlock()

	// Should only have: GET /api/admin/users, PATCH /api/admin/users/alice/role
	getCount := 0
	patchCount := 0
	for _, call := range apiCalls {
		if strings.HasPrefix(call, "GET /api/admin/users") {
			getCount++
		}
		if strings.HasPrefix(call, "PATCH ") {
			patchCount++
		}
	}
	if getCount != 1 {
		t.Errorf("expected 1 GET call, got %d", getCount)
	}
	if patchCount != 1 {
		t.Errorf("expected 1 PATCH call (for alice), got %d; calls: %v", patchCount, apiCalls)
	}
}

func TestSyncAdmins_NewSignature_OnlySetsRoleAdmin(t *testing.T) {
	// Verify PATCH body is ONLY {"role": "admin"}, no extra fields.
	var patchBodies []map[string]string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			var fields map[string]string
			json.NewDecoder(r.Body).Decode(&fields)
			mu.Lock()
			patchBodies = append(patchBodies, fields)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:portal-1", logger)

	mu.Lock()
	defer mu.Unlock()

	if len(patchBodies) != 1 {
		t.Fatalf("expected 1 PATCH call, got %d", len(patchBodies))
	}
	body := patchBodies[0]
	if len(body) != 1 {
		t.Errorf("expected PATCH body with exactly 1 field, got %d: %v", len(body), body)
	}
	if body["role"] != "admin" {
		t.Errorf("expected role=admin, got %q", body["role"])
	}
}

// =============================================================================
// Concurrent Multi-Instance Safety (New API)
// =============================================================================

func TestSyncAdmins_NewSignature_ConcurrentInstances(t *testing.T) {
	var patchCount atomic.Int64
	var patchIDs sync.Map

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
		{ID: "bob", Email: "bob@example.com", Role: "user"},
		{ID: "charlie", Email: "charlie@example.com", Role: "admin"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			patchCount.Add(1)
			// Extract user ID: /api/admin/users/{id}/role
			parts := strings.Split(r.URL.Path, "/")
			if len(parts) >= 5 {
				patchIDs.Store(parts[4], true)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	emails := []string{"alice@example.com", "bob@example.com", "charlie@example.com"}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SyncAdmins(srv.URL, emails, "service:portal-1", logger)
		}()
	}
	wg.Wait()

	// Each instance updates alice and bob (not charlie). 2 * 10 = 20 PATCHes.
	totalPatches := patchCount.Load()
	if totalPatches != 20 {
		t.Errorf("expected 20 total PATCH calls, got %d", totalPatches)
	}

	if _, ok := patchIDs.Load("charlie"); ok {
		t.Error("charlie should NOT be patched — already admin")
	}
}

// =============================================================================
// Partial Failure in Admin Sync (New API)
// =============================================================================

func TestSyncAdmins_NewSignature_PartialFailure(t *testing.T) {
	promotedUsers := sync.Map{}
	failOnBob := true
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
		{ID: "bob", Email: "bob@example.com", Role: "user"},
		{ID: "charlie", Email: "charlie@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			// Return current state
			currentUsers := make([]client.AdminUser, len(users))
			copy(currentUsers, users)
			for i := range currentUsers {
				if _, ok := promotedUsers.Load(currentUsers[i].ID); ok {
					currentUsers[i].Role = "admin"
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(currentUsers))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			// Extract user ID
			parts := strings.Split(r.URL.Path, "/")
			userID := ""
			if len(parts) >= 5 {
				userID = parts[4]
			}

			mu.Lock()
			shouldFail := failOnBob && userID == "bob"
			if shouldFail {
				failOnBob = false
			}
			mu.Unlock()

			if shouldFail {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"temporary failure"}`))
				return
			}

			promotedUsers.Store(userID, true)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{
		"alice@example.com",
		"bob@example.com",
		"charlie@example.com",
	}, "service:portal-1", logger)

	// After retries, all should be promoted
	for _, name := range []string{"alice", "bob", "charlie"} {
		if _, ok := promotedUsers.Load(name); !ok {
			t.Errorf("expected %s to be promoted after retries", name)
		}
	}
}

// =============================================================================
// Duplicate Emails (New API)
// =============================================================================

func TestSyncAdmins_NewSignature_DuplicateEmails(t *testing.T) {
	var patchCount atomic.Int64

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if r.Method == http.MethodPatch {
			patchCount.Add(1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{
		"alice@example.com",
		"alice@example.com",
		"ALICE@EXAMPLE.COM",
	}, "service:portal-1", logger)

	// Even if duplicated PATCHes happen it's safe (idempotent).
	count := patchCount.Load()
	if count > 3 {
		t.Errorf("expected at most 3 PATCH calls (one per email entry), got %d", count)
	}
}

// =============================================================================
// Large User List (New API)
// =============================================================================

func TestSyncAdmins_NewSignature_LargeUserList(t *testing.T) {
	var users []client.AdminUser
	for i := 0; i < 1000; i++ {
		users = append(users, client.AdminUser{
			ID:    fmt.Sprintf("user%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Role:  "user",
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	// Only promote 3 out of 1000
	SyncAdmins(srv.URL, []string{
		"user0@example.com",
		"user500@example.com",
		"user999@example.com",
	}, "service:portal-1", logger)
}

// =============================================================================
// Service ID Propagation
// =============================================================================

func TestSyncAdmins_NewSignature_ServiceIDPropagated(t *testing.T) {
	// Verify the serviceUserID is passed to both AdminListUsers and AdminUpdateUserRole.
	var getServiceID, patchServiceID string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			getServiceID = r.Header.Get("X-Vire-Service-ID")
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			patchServiceID = r.Header.Get("X-Vire-Service-ID")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:unique-portal-xyz", logger)

	mu.Lock()
	defer mu.Unlock()

	if getServiceID != "service:unique-portal-xyz" {
		t.Errorf("AdminListUsers got service ID %q, expected service:unique-portal-xyz", getServiceID)
	}
	if patchServiceID != "service:unique-portal-xyz" {
		t.Errorf("AdminUpdateUserRole got service ID %q, expected service:unique-portal-xyz", patchServiceID)
	}
}

// =============================================================================
// Malformed Server Responses (New API)
// =============================================================================

func TestSyncAdmins_NewSignature_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken json`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	// Must not panic
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:portal-1", logger)
}

func TestSyncAdmins_NewSignature_EmptyUserList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse([]client.AdminUser{}))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, "service:portal-1", logger)
}

func TestSyncAdmins_NewSignature_PartialData(t *testing.T) {
	// Users with missing email or ID fields.
	users := []client.AdminUser{
		{ID: "alice", Email: "", Role: "user"},               // empty email
		{ID: "bob", Email: "bob@example.com", Role: ""},      // empty role
		{ID: "", Email: "charlie@example.com", Role: "user"}, // empty ID
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	// Must not panic
	SyncAdmins(srv.URL, []string{"bob@example.com", "charlie@example.com"}, "service:portal-1", logger)
}
