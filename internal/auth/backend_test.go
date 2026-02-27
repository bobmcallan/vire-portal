package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOAuthBackend_SaveSession(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/internal/oauth/sessions" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	sess := &AuthSession{
		SessionID:     "sess-1",
		ClientID:      "client-1",
		RedirectURI:   "http://localhost/callback",
		State:         "state-abc",
		CodeChallenge: "challenge123",
		CodeMethod:    "S256",
		Scope:         "openid",
		CreatedAt:     time.Now(),
	}

	err := backend.SaveSession(sess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["session_id"] != "sess-1" {
		t.Errorf("expected session_id sess-1, got %v", gotBody["session_id"])
	}
	if gotBody["client_id"] != "client-1" {
		t.Errorf("expected client_id client-1, got %v", gotBody["client_id"])
	}
}

func TestOAuthBackend_SaveSession_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	err := backend.SaveSession(&AuthSession{SessionID: "sess-1", CreatedAt: time.Now()})
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestOAuthBackend_GetSession(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/internal/oauth/sessions/sess-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id":     "sess-1",
			"client_id":      "client-1",
			"redirect_uri":   "http://localhost/callback",
			"state":          "state-abc",
			"code_challenge": "challenge123",
			"code_method":    "S256",
			"scope":          "openid",
			"created_at":     now.Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	sess, err := backend.GetSession("sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.SessionID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sess.SessionID)
	}
	if sess.ClientID != "client-1" {
		t.Errorf("expected client-1, got %s", sess.ClientID)
	}
}

func TestOAuthBackend_GetSession_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	sess, err := backend.GetSession("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess != nil {
		t.Error("expected nil session for not found")
	}
}

func TestOAuthBackend_GetSessionByClientID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/internal/oauth/sessions" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("client_id") != "client-1" {
			t.Errorf("expected client_id=client-1 query param")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": "sess-1",
			"client_id":  "client-1",
			"created_at": time.Now().Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	sess, err := backend.GetSessionByClientID("client-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.SessionID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sess.SessionID)
	}
}

func TestOAuthBackend_GetSessionByClientID_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	sess, err := backend.GetSessionByClientID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess != nil {
		t.Error("expected nil session for not found")
	}
}

func TestOAuthBackend_UpdateSessionUserID(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/internal/oauth/sessions/sess-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	err := backend.UpdateSessionUserID("sess-1", "user-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["user_id"] != "user-42" {
		t.Errorf("expected user_id user-42, got %v", gotBody["user_id"])
	}
}

func TestOAuthBackend_DeleteSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/internal/oauth/sessions/sess-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	err := backend.DeleteSession("sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOAuthBackend_SaveClient(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/internal/oauth/clients" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	client := &OAuthClient{
		ClientID:     "client-1",
		ClientSecret: "secret-123",
		ClientName:   "Test App",
		RedirectURIs: []string{"http://localhost/callback"},
	}

	err := backend.SaveClient(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["client_id"] != "client-1" {
		t.Errorf("expected client_id client-1, got %v", gotBody["client_id"])
	}
}

func TestOAuthBackend_GetClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/internal/oauth/clients/client-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"client_id":     "client-1",
			"client_name":   "Test App",
			"redirect_uris": []string{"http://localhost/callback"},
		})
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	client, err := backend.GetClient("client-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.ClientID != "client-1" {
		t.Errorf("expected client-1, got %s", client.ClientID)
	}
	if client.ClientName != "Test App" {
		t.Errorf("expected Test App, got %s", client.ClientName)
	}
}

func TestOAuthBackend_GetClient_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	client, err := backend.GetClient("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Error("expected nil client for not found")
	}
}

func TestOAuthBackend_SaveCode(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/internal/oauth/codes" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	code := &AuthCode{
		Code:     "code-abc",
		ClientID: "client-1",
		UserID:   "user-42",
	}

	err := backend.SaveCode(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["code"] != "code-abc" {
		t.Errorf("expected code code-abc, got %v", gotBody["code"])
	}
}

func TestOAuthBackend_GetCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/internal/oauth/codes/code-abc" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":       "code-abc",
			"client_id":  "client-1",
			"user_id":    "user-42",
			"expires_at": time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	code, err := backend.GetCode("code-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code.Code != "code-abc" {
		t.Errorf("expected code-abc, got %s", code.Code)
	}
	if code.ClientID != "client-1" {
		t.Errorf("expected client-1, got %s", code.ClientID)
	}
}

func TestOAuthBackend_GetCode_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	code, err := backend.GetCode("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != nil {
		t.Error("expected nil code for not found")
	}
}

func TestOAuthBackend_MarkCodeUsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/internal/oauth/codes/code-abc/used" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	err := backend.MarkCodeUsed("code-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOAuthBackend_SaveToken(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/internal/oauth/tokens" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	token := &RefreshToken{
		Token:     "token-abc",
		UserID:    "user-42",
		ClientID:  "client-1",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	err := backend.SaveToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["token"] != "token-abc" {
		t.Errorf("expected token token-abc, got %v", gotBody["token"])
	}
}

func TestOAuthBackend_LookupToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/internal/oauth/tokens/lookup" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["token"] != "token-abc" {
			t.Errorf("expected token token-abc, got %v", body["token"])
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":      "token-abc",
			"user_id":    "user-42",
			"client_id":  "client-1",
			"scope":      "openid",
			"expires_at": time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	token, err := backend.LookupToken("token-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.Token != "token-abc" {
		t.Errorf("expected token-abc, got %s", token.Token)
	}
	if token.UserID != "user-42" {
		t.Errorf("expected user-42, got %s", token.UserID)
	}
}

func TestOAuthBackend_LookupToken_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	token, err := backend.LookupToken("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != nil {
		t.Error("expected nil token for not found")
	}
}

func TestOAuthBackend_RevokeToken(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/internal/oauth/tokens/revoke" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	backend := NewOAuthBackend(srv.URL, nil)
	err := backend.RevokeToken("token-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["token"] != "token-abc" {
		t.Errorf("expected token token-abc, got %v", gotBody["token"])
	}
}

func TestOAuthBackend_NilBackend(t *testing.T) {
	// Verify that nil backend doesn't panic — callers should check for nil
	var backend *OAuthBackend
	if backend != nil {
		t.Error("expected nil backend")
	}
}

func TestOAuthBackend_ConnectionRefused(t *testing.T) {
	// Use an unreachable URL to test connection errors
	backend := NewOAuthBackend("http://127.0.0.1:1", nil)
	err := backend.SaveSession(&AuthSession{SessionID: "sess-1", CreatedAt: time.Now()})
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}
