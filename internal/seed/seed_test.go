package seed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

func TestLoadUsersFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	data := `{"users":[{"username":"alice","email":"alice@example.com","password":"pass","role":"admin","navexa_key":""}]}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	users, err := loadUsersFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "alice" {
		t.Errorf("expected username alice, got %s", users[0].Username)
	}
	if users[0].Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", users[0].Email)
	}
	if users[0].Role != "admin" {
		t.Errorf("expected role admin, got %s", users[0].Role)
	}
}

func TestLoadUsersFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadUsersFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadUsersFile_NotFound(t *testing.T) {
	_, err := loadUsersFile("/nonexistent/path/users.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadUsersFile_EmptyUsers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	if err := os.WriteFile(path, []byte(`{"users":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	users, err := loadUsersFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestSeedAll_Success(t *testing.T) {
	var received []client.SeedUser
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/upsert" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var u client.SeedUser
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		received = append(received, u)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := client.NewVireClient(srv.URL)
	users := []client.SeedUser{
		{Username: "alice", Email: "alice@example.com", Password: "pass1", Role: "admin"},
		{Username: "bob", Email: "bob@example.com", Password: "pass2", Role: "user"},
	}

	err := seedAll(c, users, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(received))
	}
	if received[0].Username != "alice" {
		t.Errorf("expected first user alice, got %s", received[0].Username)
	}
	if received[1].Username != "bob" {
		t.Errorf("expected second user bob, got %s", received[1].Username)
	}
}

func TestSeedAll_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"db down"}`))
	}))
	defer srv.Close()

	c := client.NewVireClient(srv.URL)
	users := []client.SeedUser{
		{Username: "alice", Email: "alice@example.com", Password: "pass", Role: "admin"},
	}

	err := seedAll(c, users, nil)
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestSeedAll_StopsOnFirstError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"fail"}`))
	}))
	defer srv.Close()

	c := client.NewVireClient(srv.URL)
	users := []client.SeedUser{
		{Username: "alice"},
		{Username: "bob"},
		{Username: "charlie"},
	}

	err := seedAll(c, users, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (stop on first error), got %d", callCount)
	}
}

func TestSeedAll_Unreachable(t *testing.T) {
	c := client.NewVireClient("http://127.0.0.1:1")
	users := []client.SeedUser{
		{Username: "alice"},
	}

	err := seedAll(c, users, nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
