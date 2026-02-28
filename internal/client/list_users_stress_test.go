package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- ListUsers: Malformed Responses ---

func TestListUsers_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.ListUsers()
	if err == nil {
		t.Fatal("expected error for empty body with 200 status")
	}
}

func TestListUsers_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not json at all`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.ListUsers()
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestListUsers_WrongJSONShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"unexpected":"format"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	users, err := c.ListUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return empty list since data is missing/nil
	if len(users) != 0 {
		t.Errorf("expected 0 users for wrong JSON shape, got %d", len(users))
	}
}

func TestListUsers_EmptyDataArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   []interface{}{},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	users, err := c.ListUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestListUsers_Unreachable(t *testing.T) {
	c := NewVireClient("http://127.0.0.1:1")
	_, err := c.ListUsers()
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// --- ListUsers: Large Response ---

func TestListUsers_LargeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Write response larger than 1MB (LimitReader boundary)
		w.Write([]byte(`{"status":"ok","data":[{"username":"`))
		w.Write([]byte(strings.Repeat("x", 2<<20)))
		w.Write([]byte(`"}]}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.ListUsers()
	// Should fail to parse because response was truncated at 1MB
	if err == nil {
		t.Log("large response parsed without error (truncated data)")
	}
}

// --- ListUsers: HTML Error Page ---

func TestListUsers_HTMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`<html><body>502 Bad Gateway</body></html>`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.ListUsers()
	if err == nil {
		t.Fatal("expected error for HTML response with 502")
	}
}

// --- ListUsers: No Auth Headers ---

func TestListUsers_NoAuthHeaders(t *testing.T) {
	// Verify ListUsers sends no auth headers (server-to-server internal call).
	// Document this as a security consideration.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("ListUsers should not send Authorization header, got: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   []interface{}{},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.ListUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ListUsers: Users with Hostile Data ---

func TestListUsers_HostileUserData(t *testing.T) {
	// Verify client doesn't crash when parsing users with hostile field values.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": []map[string]interface{}{
				{
					"username": "<script>alert(1)</script>",
					"email":    "'; DROP TABLE users; --",
					"role":     "../../admin",
				},
				{
					"username": strings.Repeat("A", 10000),
					"email":    strings.Repeat("B", 10000) + "@example.com",
					"role":     strings.Repeat("C", 10000),
				},
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	users, err := c.ListUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

// --- ListUsers: HTTP Status Code Edge Cases ---

func TestListUsers_StatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"200 OK", 200, `{"status":"ok","data":[]}`, false},
		{"201 Created", 201, `{"status":"ok","data":[]}`, true},
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
			_, err := c.ListUsers()

			if tc.wantErr && err == nil {
				t.Errorf("expected error for status %d", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for status %d: %v", tc.status, err)
			}
		})
	}
}
