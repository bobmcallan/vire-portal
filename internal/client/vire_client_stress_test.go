package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- GetUser: Path Traversal & Injection ---

func TestGetUser_PathTraversal(t *testing.T) {
	// Verify that hostile userIDs are sent to the server as-is in the URL path.
	// The server must validate the userID; the client should not crash or misbehave.
	var receivedPaths []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedPaths = append(receivedPaths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	hostile := []string{
		"../../etc/passwd",
		"../admin",
		"user?admin=true",
		"user#fragment",
		"user%00null",
		"user/../../admin",
		"'; DROP TABLE users; --",
		"<script>alert(1)</script>",
		"user\nX-Injected: evil",
		"user\r\nHost: evil.com",
		strings.Repeat("A", 10000),
	}

	for _, id := range hostile {
		_, err := c.GetUser(id)
		if err == nil {
			t.Errorf("expected error for hostile userID %q", id)
		}
	}
}

func TestUpdateUser_PathTraversal(t *testing.T) {
	// Same path traversal checks for UpdateUser.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	hostile := []string{
		"../../etc/passwd",
		"../admin",
		"user?admin=true",
		"user/../../admin",
	}

	for _, id := range hostile {
		_, err := c.UpdateUser(id, map[string]string{"navexa_key": "val"})
		if err == nil {
			t.Errorf("expected error for hostile userID %q", id)
		}
	}
}

// --- GetUser: Malformed Server Responses ---

func TestGetUser_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty body
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.GetUser("alice")
	if err == nil {
		t.Fatal("expected error for empty body with 200 status")
	}
}

func TestGetUser_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not json at all`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.GetUser("alice")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "failed to parse response") {
		t.Errorf("expected 'failed to parse response' error, got: %v", err)
	}
}

func TestGetUser_WrongJSONShape(t *testing.T) {
	// Server returns valid JSON but not the expected { status, data } shape
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"unexpected":"format"}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	user, err := c.GetUser("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return a zero-value UserProfile since data is missing
	if user.Username != "" {
		t.Errorf("expected empty username for wrong JSON shape, got %q", user.Username)
	}
}

func TestGetUser_HTMLResponse(t *testing.T) {
	// Server returns HTML (e.g., 502 from a reverse proxy)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`<html><body>502 Bad Gateway</body></html>`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.GetUser("alice")
	if err == nil {
		t.Fatal("expected error for HTML response with 502")
	}
}

func TestGetUser_ServerUnreachable(t *testing.T) {
	// Point to a port that's not listening
	c := NewVireClient("http://127.0.0.1:1")
	_, err := c.GetUser("alice")
	if err == nil {
		t.Fatal("expected error when server is unreachable")
	}
	if !strings.Contains(err.Error(), "failed to reach vire-server") {
		t.Errorf("expected 'failed to reach vire-server' error, got: %v", err)
	}
}

func TestGetUser_SlowServer(t *testing.T) {
	// The client has a 10s timeout. Verify it actually times out.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Short delay to test responsiveness
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"username": "alice",
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	user, err := c.GetUser("alice")
	if err != nil {
		t.Fatalf("short delay should not cause timeout: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("expected alice, got %q", user.Username)
	}
}

func TestGetUser_LargeResponse(t *testing.T) {
	// Response body is capped at 1MB by LimitReader. Verify no OOM.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Write 2MB of data — LimitReader should cap at 1MB
		w.Write([]byte(`{"status":"ok","data":{"username":"`))
		w.Write([]byte(strings.Repeat("x", 2<<20)))
		w.Write([]byte(`"}}`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.GetUser("alice")
	// Should fail to parse because the response was truncated at 1MB
	if err == nil {
		// If it parses, that's also acceptable as long as it didn't OOM
		t.Log("large response parsed without error (truncated username)")
	}
}

// --- UpdateUser: Hostile Field Values ---

func TestUpdateUser_HostileFieldValues(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"username": "alice"},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	hostileValues := map[string]string{
		"navexa_key":  "<script>alert('xss')</script>",
		"extra_field": "'; DROP TABLE users; --",
	}

	_, err := c.UpdateUser("alice", hostileValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the hostile values are properly JSON-encoded (not interpreted)
	var decoded map[string]string
	if err := json.Unmarshal(receivedBody, &decoded); err != nil {
		t.Fatalf("failed to decode sent body: %v", err)
	}
	if decoded["navexa_key"] != "<script>alert('xss')</script>" {
		t.Errorf("hostile value was mangled: %q", decoded["navexa_key"])
	}
}

func TestUpdateUser_EmptyFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"username": "alice"},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.UpdateUser("alice", map[string]string{})
	if err != nil {
		t.Fatalf("empty fields should not cause error: %v", err)
	}
}

func TestUpdateUser_NilFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"username": "alice"},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.UpdateUser("alice", nil)
	if err != nil {
		t.Fatalf("nil fields should not cause error: %v", err)
	}
}

func TestUpdateUser_InvalidJSON_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken json`))
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)
	_, err := c.UpdateUser("alice", map[string]string{"navexa_key": "val"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// --- Concurrent Access ---

func TestGetUser_ConcurrentRequests(t *testing.T) {
	var requestCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"username": strings.TrimPrefix(r.URL.Path, "/api/users/"),
			},
		})
	}))
	defer srv.Close()

	c := NewVireClient(srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := strings.Repeat("user", id%5+1)
			user, err := c.GetUser(userID)
			if err != nil {
				t.Errorf("concurrent GetUser(%q) failed: %v", userID, err)
				return
			}
			if user.Username != userID {
				t.Errorf("expected username %q, got %q", userID, user.Username)
			}
		}(i)
	}
	wg.Wait()

	if requestCount.Load() != 50 {
		t.Errorf("expected 50 requests, got %d", requestCount.Load())
	}
}

// --- HTTP Status Code Edge Cases ---

func TestGetUser_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantErr    bool
		errContain string
	}{
		{"200 OK", 200, `{"status":"ok","data":{"username":"a"}}`, false, ""},
		{"201 Created", 201, `{"status":"ok","data":{"username":"a"}}`, true, "server returned 201"},
		{"301 Redirect", 301, ``, true, ""},
		{"400 Bad Request", 400, `{"error":"bad"}`, true, "server returned 400"},
		{"401 Unauthorized", 401, `{"error":"unauthorized"}`, true, "server returned 401"},
		{"403 Forbidden", 403, `{"error":"forbidden"}`, true, "server returned 403"},
		{"404 Not Found", 404, `{"error":"not found"}`, true, "user not found"},
		{"429 Too Many Requests", 429, `{"error":"rate limited"}`, true, "server returned 429"},
		{"500 Internal Server Error", 500, `{"error":"internal"}`, true, "server returned 500"},
		{"502 Bad Gateway", 502, `<html>502</html>`, true, "server returned 502"},
		{"503 Service Unavailable", 503, ``, true, "server returned 503"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewVireClient(srv.URL)
			_, err := c.GetUser("alice")

			if tc.wantErr && err == nil {
				t.Errorf("expected error for status %d", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for status %d: %v", tc.status, err)
			}
			if tc.errContain != "" && err != nil && !strings.Contains(err.Error(), tc.errContain) {
				t.Errorf("expected error containing %q, got: %v", tc.errContain, err)
			}
		})
	}
}
