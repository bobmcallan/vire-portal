package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/seed"
)

// --- RegisterService (VireClient) ---

func TestRegisterService_ValidRegistration(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	c := client.NewVireClient(env.ServerURL())
	serviceUserID, err := c.RegisterService("portal-test-1", validServiceKey)
	if err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}
	if serviceUserID != "service:portal-test-1" {
		t.Errorf("expected service_user_id service:portal-test-1, got %s", serviceUserID)
	}
}

func TestRegisterService_Idempotent(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	c := client.NewVireClient(env.ServerURL())

	// First registration
	id1, err := c.RegisterService("portal-idempotent", validServiceKey)
	if err != nil {
		t.Fatalf("first RegisterService failed: %v", err)
	}

	// Second registration (portal restart)
	id2, err := c.RegisterService("portal-idempotent", validServiceKey)
	if err != nil {
		t.Fatalf("second RegisterService failed: %v", err)
	}

	if id1 != id2 {
		t.Errorf("idempotent registration should return same ID: %s != %s", id1, id2)
	}
}

func TestRegisterService_WrongKey(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	c := client.NewVireClient(env.ServerURL())
	_, err := c.RegisterService("portal-bad-key", "wrong-key-that-does-not-match-server")
	if err == nil {
		t.Fatal("expected error for wrong service key")
	}
	if !contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestRegisterService_NoServerKey(t *testing.T) {
	env := NewServerEnv(t) // no VIRE_SERVICE_KEY set
	defer env.Cleanup()

	c := client.NewVireClient(env.ServerURL())
	_, err := c.RegisterService("portal-no-key", validServiceKey)
	if err == nil {
		t.Fatal("expected error when server has no service key configured")
	}
	if !contains(err.Error(), "501") {
		t.Errorf("expected 501 in error, got: %v", err)
	}
}

// --- AdminListUsers (VireClient) ---

func TestAdminListUsers_WithServiceAuth(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	// Register service
	c := client.NewVireClient(env.ServerURL())
	serviceUserID, err := c.RegisterService("portal-list-test", validServiceKey)
	if err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	// Create a user to find in the list
	createUser(t, env, "listuser", "listuser@test.com", "password123")

	// List users via admin API
	users, err := c.AdminListUsers(serviceUserID)
	if err != nil {
		t.Fatalf("AdminListUsers failed: %v", err)
	}

	if len(users) == 0 {
		t.Fatal("expected at least one user in admin list")
	}

	// Find our test user
	found := false
	for _, u := range users {
		if u.Email == "listuser@test.com" {
			found = true
			if u.ID == "" {
				t.Error("expected non-empty user ID")
			}
			if u.Role != "user" {
				t.Errorf("expected role user, got %s", u.Role)
			}
			break
		}
	}
	if !found {
		t.Error("listuser@test.com not found in admin user list")
	}
}

// --- AdminUpdateUserRole (VireClient) ---

func TestAdminUpdateUserRole_PromoteToAdmin(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	c := client.NewVireClient(env.ServerURL())

	// Register service
	serviceUserID, err := c.RegisterService("portal-promote-test", validServiceKey)
	if err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	// Create user to promote
	createUser(t, env, "promoteme", "promoteme@test.com", "password123")

	// Promote via admin API
	err = c.AdminUpdateUserRole(serviceUserID, "promoteme", "admin")
	if err != nil {
		t.Fatalf("AdminUpdateUserRole failed: %v", err)
	}

	// Verify via admin list
	users, err := c.AdminListUsers(serviceUserID)
	if err != nil {
		t.Fatalf("AdminListUsers failed: %v", err)
	}

	for _, u := range users {
		if u.ID == "promoteme" {
			if u.Role != "admin" {
				t.Errorf("expected role admin after promotion, got %s", u.Role)
			}
			return
		}
	}
	t.Error("promoted user not found in admin list")
}

// --- seed.RegisterService ---

func TestSeedRegisterService_Success(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	serviceUserID, err := seed.RegisterService(env.ServerURL(), "portal-seed-test", validServiceKey, nil)
	if err != nil {
		t.Fatalf("seed.RegisterService failed: %v", err)
	}
	if serviceUserID != "service:portal-seed-test" {
		t.Errorf("expected service:portal-seed-test, got %s", serviceUserID)
	}
}

func TestSeedRegisterService_FailsWithWrongKey(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	_, err := seed.RegisterService(env.ServerURL(), "portal-seed-bad", "wrong-key-wrong-key-wrong-key-wrong", nil)
	if err == nil {
		t.Fatal("expected error for wrong service key")
	}
}

// --- seed.SyncAdmins (end-to-end) ---

func TestSyncAdmins_EndToEnd(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	// Register service
	serviceUserID, err := seed.RegisterService(env.ServerURL(), "portal-sync-test", validServiceKey, nil)
	if err != nil {
		t.Fatalf("seed.RegisterService failed: %v", err)
	}

	// Create users with known emails
	createUser(t, env, "alice", "alice@example.com", "password123")
	createUser(t, env, "bob", "bob@example.com", "password123")
	createUser(t, env, "carol", "carol@example.com", "password123")

	// Sync admins — alice and carol should be promoted, bob stays user
	seed.SyncAdmins(env.ServerURL(), []string{"alice@example.com", "carol@example.com"}, serviceUserID, nil)

	// Verify roles via admin API
	c := client.NewVireClient(env.ServerURL())
	users, err := c.AdminListUsers(serviceUserID)
	if err != nil {
		t.Fatalf("AdminListUsers failed: %v", err)
	}

	roles := make(map[string]string)
	for _, u := range users {
		roles[u.Email] = u.Role
	}

	if roles["alice@example.com"] != "admin" {
		t.Errorf("expected alice to be admin, got %s", roles["alice@example.com"])
	}
	if roles["carol@example.com"] != "admin" {
		t.Errorf("expected carol to be admin, got %s", roles["carol@example.com"])
	}
	if roles["bob@example.com"] != "user" {
		t.Errorf("expected bob to remain user, got %s", roles["bob@example.com"])
	}
}

func TestSyncAdmins_AlreadyAdmin(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	serviceUserID, err := seed.RegisterService(env.ServerURL(), "portal-already-admin", validServiceKey, nil)
	if err != nil {
		t.Fatalf("seed.RegisterService failed: %v", err)
	}

	c := client.NewVireClient(env.ServerURL())

	// Create user and promote manually
	createUser(t, env, "prealice", "prealice@example.com", "password123")
	err = c.AdminUpdateUserRole(serviceUserID, "prealice", "admin")
	if err != nil {
		t.Fatalf("manual promotion failed: %v", err)
	}

	// SyncAdmins should skip without error
	seed.SyncAdmins(env.ServerURL(), []string{"prealice@example.com"}, serviceUserID, nil)

	// Verify still admin
	users, err := c.AdminListUsers(serviceUserID)
	if err != nil {
		t.Fatalf("AdminListUsers failed: %v", err)
	}
	for _, u := range users {
		if u.Email == "prealice@example.com" {
			if u.Role != "admin" {
				t.Errorf("expected prealice to remain admin, got %s", u.Role)
			}
			return
		}
	}
	t.Error("prealice not found in user list")
}

func TestSyncAdmins_EmailNotFound(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	serviceUserID, err := seed.RegisterService(env.ServerURL(), "portal-notfound", validServiceKey, nil)
	if err != nil {
		t.Fatalf("seed.RegisterService failed: %v", err)
	}

	// SyncAdmins with a nonexistent email — should not panic or error
	seed.SyncAdmins(env.ServerURL(), []string{"nobody@nonexistent.com"}, serviceUserID, nil)
}

func TestSyncAdmins_ServiceUserCannotLogin(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	// Register service
	_, err := seed.RegisterService(env.ServerURL(), "portal-login-check", validServiceKey, nil)
	if err != nil {
		t.Fatalf("seed.RegisterService failed: %v", err)
	}

	// Attempt to login as service user — must be rejected
	resp, err := env.HTTPPost("/api/auth/login", map[string]interface{}{
		"username": "service:portal-login-check",
		"password": "anypassword",
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		body := readBody(t, resp.Body)
		t.Errorf("expected 403 for service user login, got %d: %s", resp.StatusCode, string(body))
	}
}

// --- Response format verification ---

func TestAdminListUsers_ResponseFormat(t *testing.T) {
	env := NewServerEnvWithOptions(t, ServerEnvOptions{
		ExtraEnv: map[string]string{"VIRE_SERVICE_KEY": validServiceKey},
	})
	defer env.Cleanup()

	// Register service
	c := client.NewVireClient(env.ServerURL())
	serviceUserID, err := c.RegisterService("portal-format-test", validServiceKey)
	if err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}

	createUser(t, env, "formatuser", "format@test.com", "password123")

	// Make raw HTTP request to verify exact response format
	resp, err := env.HTTPRequest(http.MethodGet, "/api/admin/users", nil,
		map[string]string{"X-Vire-Service-ID": serviceUserID})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body := readBody(t, resp.Body)
	env.SaveResult("admin_list_response.json", body)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify the response has "users" key (not "data")
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := raw["users"]; !ok {
		t.Error("response must have 'users' key at top level")
	}

	// Verify user fields include "id" (not "username")
	var result struct {
		Users []map[string]interface{} `json:"users"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse users: %v", err)
	}

	for _, u := range result.Users {
		if _, ok := u["id"]; !ok {
			t.Errorf("user missing 'id' field: %v", u)
		}
		if _, ok := u["email"]; !ok {
			t.Errorf("user missing 'email' field: %v", u)
		}
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
