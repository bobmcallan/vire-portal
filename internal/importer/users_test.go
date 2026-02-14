package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/models"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	"github.com/timshannon/badgerhold/v4"
	"golang.org/x/crypto/bcrypt"
)

func testLogger() *common.Logger {
	return common.NewSilentLogger()
}

func testStore(t *testing.T) *badgerhold.Store {
	t.Helper()
	dir := t.TempDir()
	options := badgerhold.DefaultOptions
	options.Dir = dir
	options.ValueDir = dir
	options.Logger = nil
	store, err := badgerhold.Open(options)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func writeUsersJSON(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "users.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test users file: %v", err)
	}
	return path
}

func TestImportUsers_ValidJSON(t *testing.T) {
	store := testStore(t)
	dir := t.TempDir()

	jsonContent := `{
		"users": [
			{"username": "alice", "email": "alice@test.com", "password": "pass1", "role": "developer"},
			{"username": "bob", "email": "bob@test.com", "password": "pass2", "role": "admin"}
		]
	}`
	path := writeUsersJSON(t, dir, jsonContent)

	err := ImportUsers(store, testLogger(), path)
	if err != nil {
		t.Fatalf("ImportUsers failed: %v", err)
	}

	// Verify users were inserted
	var alice models.User
	if err := store.Get("alice", &alice); err != nil {
		t.Fatalf("failed to get alice: %v", err)
	}
	if alice.Email != "alice@test.com" {
		t.Errorf("expected alice email alice@test.com, got %s", alice.Email)
	}
	if alice.Role != "developer" {
		t.Errorf("expected alice role developer, got %s", alice.Role)
	}

	var bob models.User
	if err := store.Get("bob", &bob); err != nil {
		t.Fatalf("failed to get bob: %v", err)
	}
	if bob.Role != "admin" {
		t.Errorf("expected bob role admin, got %s", bob.Role)
	}
}

func TestImportUsers_Idempotent(t *testing.T) {
	store := testStore(t)
	dir := t.TempDir()

	jsonContent := `{
		"users": [
			{"username": "alice", "email": "alice@test.com", "password": "pass1", "role": "developer"}
		]
	}`
	path := writeUsersJSON(t, dir, jsonContent)

	// Import twice
	if err := ImportUsers(store, testLogger(), path); err != nil {
		t.Fatalf("first ImportUsers failed: %v", err)
	}
	if err := ImportUsers(store, testLogger(), path); err != nil {
		t.Fatalf("second ImportUsers failed: %v", err)
	}

	// Should still have exactly one user
	var users []models.User
	if err := store.Find(&users, nil); err != nil {
		t.Fatalf("failed to find users: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user after idempotent import, got %d", len(users))
	}
}

func TestImportUsers_PasswordBcryptHashed(t *testing.T) {
	store := testStore(t)
	dir := t.TempDir()

	jsonContent := `{
		"users": [
			{"username": "alice", "email": "alice@test.com", "password": "mysecret", "role": "developer"}
		]
	}`
	path := writeUsersJSON(t, dir, jsonContent)

	if err := ImportUsers(store, testLogger(), path); err != nil {
		t.Fatalf("ImportUsers failed: %v", err)
	}

	var alice models.User
	if err := store.Get("alice", &alice); err != nil {
		t.Fatalf("failed to get alice: %v", err)
	}

	// Password must NOT be stored as plaintext
	if alice.Password == "mysecret" {
		t.Error("password stored as plaintext, expected bcrypt hash")
	}

	// Password must be a valid bcrypt hash that matches the original
	if err := bcrypt.CompareHashAndPassword([]byte(alice.Password), []byte("mysecret")); err != nil {
		t.Errorf("stored password does not match original via bcrypt: %v", err)
	}
}

func TestImportUsers_InvalidJSON(t *testing.T) {
	store := testStore(t)
	dir := t.TempDir()

	path := writeUsersJSON(t, dir, "not valid json {{{")

	err := ImportUsers(store, testLogger(), path)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestImportUsers_MissingFile(t *testing.T) {
	store := testStore(t)

	err := ImportUsers(store, testLogger(), "/nonexistent/users.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestImportUsers_InvalidDB(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `{"users": []}`
	path := writeUsersJSON(t, dir, jsonContent)

	err := ImportUsers("not-a-store", testLogger(), path)
	if err == nil {
		t.Error("expected error for invalid db type, got nil")
	}
}

func TestImportUsers_LongPassword(t *testing.T) {
	store := testStore(t)
	dir := t.TempDir()

	// Password longer than 72 bytes (bcrypt limit)
	longPass := "a]b]c]d]e]f]g]h]i]j]k]l]m]n]o]p]q]r]s]t]u]v]w]x]y]z]1]2]3]4]5]6]7]8]9]0]"
	jsonContent := `{
		"users": [
			{"username": "longpw", "email": "long@test.com", "password": "` + longPass + `", "role": "developer"},
			{"username": "normal", "email": "normal@test.com", "password": "short", "role": "admin"}
		]
	}`
	path := writeUsersJSON(t, dir, jsonContent)

	err := ImportUsers(store, testLogger(), path)
	if err != nil {
		t.Fatalf("ImportUsers should not fail on long password: %v", err)
	}

	// Both users should be imported
	var users []models.User
	if err := store.Find(&users, nil); err != nil {
		t.Fatalf("failed to find users: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users (long password should not abort import), got %d", len(users))
	}

	// The long password user should have a valid bcrypt hash (of truncated input)
	var longpw models.User
	if err := store.Get("longpw", &longpw); err != nil {
		t.Fatalf("failed to get longpw user: %v", err)
	}
	// Verify the hash matches the first 72 bytes
	truncated := longPass[:72]
	if err := bcrypt.CompareHashAndPassword([]byte(longpw.Password), []byte(truncated)); err != nil {
		t.Errorf("stored password does not match truncated input via bcrypt: %v", err)
	}
}

func TestImportUsers_EmptyUsers(t *testing.T) {
	store := testStore(t)
	dir := t.TempDir()

	jsonContent := `{"users": []}`
	path := writeUsersJSON(t, dir, jsonContent)

	err := ImportUsers(store, testLogger(), path)
	if err != nil {
		t.Fatalf("ImportUsers with empty users should not error: %v", err)
	}

	var users []models.User
	if err := store.Find(&users, nil); err != nil {
		t.Fatalf("failed to find users: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}
