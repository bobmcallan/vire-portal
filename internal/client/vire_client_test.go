package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUser_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/alice" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"username":           "alice",
				"email":              "alice@example.com",
				"role":               "user",
				"navexa_key_set":     true,
				"navexa_key_preview": "c123",
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	user, err := c.GetUser("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("expected username alice, got %s", user.Username)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", user.Email)
	}
	if !user.NavexaKeySet {
		t.Error("expected NavexaKeySet true")
	}
	if user.NavexaKeyPreview != "c123" {
		t.Errorf("expected NavexaKeyPreview c123, got %s", user.NavexaKeyPreview)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.GetUser("nobody")
	if err == nil {
		t.Fatal("expected error for not found user")
	}
	if err.Error() != "user not found: nobody" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetUser_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"database down"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.GetUser("alice")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestUpdateUser_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/alice" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type, got %s", r.Header.Get("Content-Type"))
		}

		var fields map[string]string
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if fields["navexa_key"] != "new-key-value" {
			t.Errorf("expected navexa_key=new-key-value, got %s", fields["navexa_key"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"username":           "alice",
				"email":              "alice@example.com",
				"role":               "user",
				"navexa_key_set":     true,
				"navexa_key_preview": "alue",
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	user, err := c.UpdateUser("alice", map[string]string{"navexa_key": "new-key-value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("expected username alice, got %s", user.Username)
	}
	if !user.NavexaKeySet {
		t.Error("expected NavexaKeySet true after update")
	}
}

func TestUpdateUser_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"update failed"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.UpdateUser("alice", map[string]string{"navexa_key": "val"})
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestUpsertUser_Created(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/upsert" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		var user SeedUser
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if user.Username != "alice" {
			t.Errorf("expected username alice, got %s", user.Username)
		}
		if user.Email != "alice@example.com" {
			t.Errorf("expected email alice@example.com, got %s", user.Email)
		}
		if user.Password != "pass123" {
			t.Errorf("expected password pass123, got %s", user.Password)
		}
		if user.Role != "admin" {
			t.Errorf("expected role admin, got %s", user.Role)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	err := c.UpsertUser(SeedUser{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "pass123",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertUser_Updated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	err := c.UpsertUser(SeedUser{Username: "alice", Email: "alice@example.com", Password: "pass"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertUser_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"db down"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	err := c.UpsertUser(SeedUser{Username: "alice"})
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestUpsertUser_Unreachable(t *testing.T) {
	c := NewVireClient("http://127.0.0.1:1")
	err := c.UpsertUser(SeedUser{Username: "alice"})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
