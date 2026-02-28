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
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// testLogger creates a minimal logger for testing.
func testLogger(t *testing.T) *common.Logger {
	t.Helper()
	return common.NewLogger("error")
}

// --- Hostile Email Input Tests ---

func TestSyncAdmins_HostileEmailStrings(t *testing.T) {
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
		"user@example.com\nBcc: evil@attacker.com",
		strings.Repeat("a", 10000) + "@example.com",
		"user@" + strings.Repeat("a", 10000) + ".com",
		"",
		" ",
		"@",
		"@@",
		"user@",
		"@example.com",
		"user name@example.com",
		"user\x00@example.com",
		"user\t@example.com",
		"$(whoami)@example.com",
		"`id`@example.com",
		"user@example.com; rm -rf /",
	}

	logger := testLogger(t)
	SyncAdmins(srv.URL, hostileEmails, testServiceUserID, logger)
}

// --- SQL/NoSQL Injection via Email ---

func TestSyncAdmins_InjectionViaEmail(t *testing.T) {
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
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	injectionEmails := []string{
		"alice@example.com",
		"' OR 1=1 --",
		"admin'--",
	}

	logger := testLogger(t)
	SyncAdmins(srv.URL, injectionEmails, testServiceUserID, logger)

	mu.Lock()
	defer mu.Unlock()

	getCount := 0
	patchCount := 0
	for _, call := range apiCalls {
		if strings.HasPrefix(call, "GET /api/admin/users") {
			getCount++
		}
		if strings.HasPrefix(call, "PATCH /api/admin/users/") {
			patchCount++
		}
	}
	if getCount != 1 {
		t.Errorf("expected exactly 1 GET /api/admin/users call, got %d", getCount)
	}
	if patchCount != 1 {
		t.Errorf("expected exactly 1 PATCH call (for alice), got %d; calls: %v", patchCount, apiCalls)
	}
}

// --- Privilege Escalation: Role Field Injection ---

func TestSyncAdmins_OnlySetsRoleAdmin(t *testing.T) {
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
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
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
	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, logger)

	mu.Lock()
	defer mu.Unlock()

	if len(patchBodies) != 1 {
		t.Fatalf("expected 1 PATCH call, got %d", len(patchBodies))
	}

	body := patchBodies[0]
	if len(body) != 1 {
		t.Errorf("expected PATCH body with exactly 1 field, got %d fields: %v", len(body), body)
	}
	if body["role"] != "admin" {
		t.Errorf("expected role=admin, got role=%q", body["role"])
	}
}

// --- Concurrent Multi-Instance Safety ---

func TestSyncAdmins_ConcurrentInstances(t *testing.T) {
	var patchCount atomic.Int64
	var patchUserIDs sync.Map

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
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			patchCount.Add(1)
			// Extract user ID: /api/admin/users/{id}/role
			path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
			userID := strings.TrimSuffix(path, "/role")
			patchUserIDs.Store(userID, true)

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
			SyncAdmins(srv.URL, emails, testServiceUserID, logger)
		}()
	}
	wg.Wait()

	totalPatches := patchCount.Load()
	if totalPatches != 20 {
		t.Errorf("expected 20 total PATCH calls (2 non-admin users * 10 instances), got %d", totalPatches)
	}

	if _, ok := patchUserIDs.Load("charlie"); ok {
		t.Error("charlie should NOT be updated — already an admin")
	}
}

// --- Partial Failure: AdminUpdateUserRole Fails Mid-Batch ---

func TestSyncAdmins_PartialUpdateFailure(t *testing.T) {
	var patchCount atomic.Int64
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
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
			userID := strings.TrimSuffix(path, "/role")
			patchCount.Add(1)

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
	SyncAdmins(srv.URL, []string{"alice@example.com", "bob@example.com", "charlie@example.com"}, testServiceUserID, logger)

	for _, name := range []string{"alice", "bob", "charlie"} {
		if _, ok := promotedUsers.Load(name); !ok {
			t.Errorf("expected %s to be promoted to admin after retries", name)
		}
	}
}

// --- AdminListUsers Server Down ---

func TestSyncAdmins_ListUsersServerDown(t *testing.T) {
	logger := testLogger(t)
	SyncAdmins("http://127.0.0.1:1", []string{"alice@example.com"}, testServiceUserID, logger)
}

// --- AdminListUsers Returns Empty List ---

func TestSyncAdmins_EmptyUserList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse([]client.AdminUser{}))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, logger)
}

// --- AdminListUsers Returns Malformed JSON ---

func TestSyncAdmins_ListUsersMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken json`))
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, logger)
}

// --- AdminListUsers Returns Partial/Corrupt Data ---

func TestSyncAdmins_ListUsersPartialData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			users := []client.AdminUser{
				{ID: "alice", Email: "", Role: "user"},               // empty email
				{ID: "bob", Email: "bob@example.com", Role: ""},      // empty role
				{ID: "", Email: "charlie@example.com", Role: "user"}, // empty ID
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"bob@example.com", "charlie@example.com"}, testServiceUserID, logger)
}

// --- Case-Insensitive Email Matching Security ---

func TestSyncAdmins_CaseInsensitiveEmailMatch(t *testing.T) {
	var patchUserIDs []string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "Alice@Example.COM", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse(users))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
			userID := strings.TrimSuffix(path, "/role")
			mu.Lock()
			patchUserIDs = append(patchUserIDs, userID)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, logger)

	mu.Lock()
	defer mu.Unlock()

	if len(patchUserIDs) != 1 || patchUserIDs[0] != "alice" {
		t.Errorf("expected alice to be updated via case-insensitive match, got: %v", patchUserIDs)
	}
}

// --- Additive-Only: Does Not Demote ---

func TestSyncAdmins_DoesNotDemoteExistingAdmins(t *testing.T) {
	var apiCalls []string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "alice", Email: "alice@example.com", Role: "admin"},
		{ID: "bob", Email: "bob@example.com", Role: "admin"},
		{ID: "charlie", Email: "charlie@example.com", Role: "user"},
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
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{"charlie@example.com"}, testServiceUserID, logger)

	mu.Lock()
	defer mu.Unlock()

	for _, call := range apiCalls {
		if call == "PATCH /api/admin/users/alice/role" || call == "PATCH /api/admin/users/bob/role" {
			t.Errorf("existing admin should NOT be modified: %s", call)
		}
	}
}

// --- Empty Email List ---

func TestSyncAdmins_EmptyEmailList(t *testing.T) {
	requestMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{}, testServiceUserID, logger)

	if requestMade {
		t.Error("expected no API calls for empty email list")
	}
}

func TestSyncAdmins_NilEmailList(t *testing.T) {
	requestMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, nil, testServiceUserID, logger)

	if requestMade {
		t.Error("expected no API calls for nil email list")
	}
}

// --- Duplicate Emails in Config ---

func TestSyncAdmins_DuplicateEmails(t *testing.T) {
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
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
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
	}, testServiceUserID, logger)

	count := patchCount.Load()
	if count > 3 {
		t.Errorf("expected at most 3 PATCH calls (one per email entry), got %d", count)
	}
}

// --- Large User List Performance ---

func TestSyncAdmins_LargeUserList(t *testing.T) {
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
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	logger := testLogger(t)
	SyncAdmins(srv.URL, []string{
		"user0@example.com",
		"user500@example.com",
		"user999@example.com",
	}, testServiceUserID, logger)
}

// --- User ID Path Traversal via Server Response ---

func TestSyncAdmins_UserIDPathTraversal(t *testing.T) {
	var patchPaths []string
	var mu sync.Mutex

	users := []client.AdminUser{
		{ID: "../../etc/passwd", Email: "alice@example.com", Role: "user"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		patchPaths = append(patchPaths, r.URL.Path)
		mu.Unlock()

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
	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, logger)
}
