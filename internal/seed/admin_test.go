package seed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

const testServiceUserID = "service:portal-test-1"

// adminUsersResponse builds a JSON response for GET /api/admin/users.
func adminUsersResponse(users []client.AdminUser) []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"users": users,
	})
	return data
}

func TestSyncAdmins_UpdatesNonAdminUser(t *testing.T) {
	var patchCalls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			if r.Header.Get("X-Vire-Service-ID") != testServiceUserID {
				t.Errorf("expected X-Vire-Service-ID %s, got %s", testServiceUserID, r.Header.Get("X-Vire-Service-ID"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse([]client.AdminUser{
				{ID: "alice", Email: "alice@example.com", Role: "user"},
			}))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && strings.HasSuffix(r.URL.Path, "/role") && r.Method == http.MethodPatch {
			if r.Header.Get("X-Vire-Service-ID") != testServiceUserID {
				t.Errorf("expected X-Vire-Service-ID %s, got %s", testServiceUserID, r.Header.Get("X-Vire-Service-ID"))
			}
			patchCalls = append(patchCalls, r.URL.Path)
			var fields map[string]string
			json.NewDecoder(r.Body).Decode(&fields)
			if fields["role"] != "admin" {
				t.Errorf("expected role=admin in PATCH body, got %s", fields["role"])
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, nil)

	if len(patchCalls) != 1 {
		t.Fatalf("expected 1 PATCH call, got %d", len(patchCalls))
	}
	if patchCalls[0] != "/api/admin/users/alice/role" {
		t.Errorf("expected PATCH to /api/admin/users/alice/role, got %s", patchCalls[0])
	}
}

func TestSyncAdmins_SkipsExistingAdmin(t *testing.T) {
	var patchCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse([]client.AdminUser{
				{ID: "alice", Email: "alice@example.com", Role: "admin"},
			}))
			return
		}
		if r.Method == http.MethodPatch {
			patchCalls++
		}
	}))
	defer srv.Close()

	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, nil)

	if patchCalls != 0 {
		t.Errorf("expected 0 PATCH calls for existing admin, got %d", patchCalls)
	}
}

func TestSyncAdmins_EmailNotFound(t *testing.T) {
	var patchCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse([]client.AdminUser{
				{ID: "alice", Email: "alice@example.com", Role: "user"},
			}))
			return
		}
		if r.Method == http.MethodPatch {
			patchCalls++
		}
	}))
	defer srv.Close()

	SyncAdmins(srv.URL, []string{"notfound@example.com"}, testServiceUserID, nil)

	if patchCalls != 0 {
		t.Errorf("expected 0 PATCH calls for unknown email, got %d", patchCalls)
	}
}

func TestSyncAdmins_MultipleEmails(t *testing.T) {
	var mu sync.Mutex
	var patchPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse([]client.AdminUser{
				{ID: "alice", Email: "alice@example.com", Role: "user"},
				{ID: "bob", Email: "bob@example.com", Role: "admin"},
				{ID: "carol", Email: "carol@example.com", Role: "user"},
			}))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			mu.Lock()
			patchPaths = append(patchPaths, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
	}))
	defer srv.Close()

	SyncAdmins(srv.URL, []string{"alice@example.com", "bob@example.com", "carol@example.com", "notfound@example.com"}, testServiceUserID, nil)

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

func TestSyncAdmins_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"db down"}`))
	}))
	defer srv.Close()

	// Should not panic
	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, nil)
}

func TestSyncAdmins_EmptyList(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer srv.Close()

	SyncAdmins(srv.URL, []string{}, testServiceUserID, nil)

	if callCount != 0 {
		t.Errorf("expected no API calls for empty email list, got %d", callCount)
	}
}

func TestSyncAdmins_CaseInsensitive(t *testing.T) {
	var patchCalls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(adminUsersResponse([]client.AdminUser{
				{ID: "alice", Email: "Alice@Example.COM", Role: "user"},
			}))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/admin/users/") && r.Method == http.MethodPatch {
			patchCalls = append(patchCalls, r.URL.Path)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
	}))
	defer srv.Close()

	SyncAdmins(srv.URL, []string{"alice@example.com"}, testServiceUserID, nil)

	if len(patchCalls) != 1 {
		t.Fatalf("expected 1 PATCH call for case-insensitive match, got %d", len(patchCalls))
	}
}

// Verify SyncAdmins uses the AdminUser type from client package.
func TestSyncAdmins_UsesClientTypes(t *testing.T) {
	_ = client.AdminUser{}
}
