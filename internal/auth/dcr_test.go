package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestOAuthServer() *OAuthServer {
	return NewOAuthServer("http://localhost:4241", []byte("test-secret"), nil)
}

func TestHandleRegister_Success(t *testing.T) {
	srv := newTestOAuthServer()

	body := `{"client_name":"Test App","redirect_uris":["http://localhost/callback"]}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.HandleRegister(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	var client OAuthClient
	if err := json.NewDecoder(rec.Body).Decode(&client); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if client.ClientID == "" {
		t.Error("expected non-empty client_id")
	}
	if client.ClientSecret == "" {
		t.Error("expected non-empty client_secret")
	}
	if client.ClientName != "Test App" {
		t.Errorf("expected client_name 'Test App', got %s", client.ClientName)
	}
	if len(client.RedirectURIs) != 1 || client.RedirectURIs[0] != "http://localhost/callback" {
		t.Errorf("unexpected redirect_uris: %v", client.RedirectURIs)
	}
	if len(client.GrantTypes) != 2 {
		t.Errorf("expected 2 default grant_types, got %v", client.GrantTypes)
	}
	if len(client.ResponseTypes) != 1 || client.ResponseTypes[0] != "code" {
		t.Errorf("expected default response_types [code], got %v", client.ResponseTypes)
	}
	if client.TokenEndpointAuthMethod != "none" {
		t.Errorf("expected default auth method 'none', got %s", client.TokenEndpointAuthMethod)
	}

	// Verify stored in client store
	stored, ok := srv.clients.Get(client.ClientID)
	if !ok {
		t.Fatal("expected client to be stored")
	}
	if stored.ClientName != "Test App" {
		t.Errorf("stored client name mismatch: %s", stored.ClientName)
	}
}

func TestHandleRegister_CustomGrantTypes(t *testing.T) {
	srv := newTestOAuthServer()

	body := `{"client_name":"Custom","redirect_uris":["http://localhost/cb"],"grant_types":["authorization_code"],"response_types":["code"],"token_endpoint_auth_method":"client_secret_post"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	srv.HandleRegister(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	var client OAuthClient
	json.NewDecoder(rec.Body).Decode(&client)

	if len(client.GrantTypes) != 1 || client.GrantTypes[0] != "authorization_code" {
		t.Errorf("expected custom grant_types, got %v", client.GrantTypes)
	}
	if client.TokenEndpointAuthMethod != "client_secret_post" {
		t.Errorf("expected custom auth method, got %s", client.TokenEndpointAuthMethod)
	}
}

func TestHandleRegister_MissingRedirectURIs(t *testing.T) {
	srv := newTestOAuthServer()

	body := `{"client_name":"No Redirect"}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	srv.HandleRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var errResp map[string]string
	json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp["error"] != "invalid_request" {
		t.Errorf("expected error 'invalid_request', got %s", errResp["error"])
	}
}

func TestHandleRegister_EmptyRedirectURIs(t *testing.T) {
	srv := newTestOAuthServer()

	body := `{"client_name":"Empty URIs","redirect_uris":[]}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	srv.HandleRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRegister_InvalidJSON(t *testing.T) {
	srv := newTestOAuthServer()

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	srv.HandleRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRegister_MethodNotAllowed(t *testing.T) {
	srv := newTestOAuthServer()

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rec := httptest.NewRecorder()

	srv.HandleRegister(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleRegister_ContentType(t *testing.T) {
	srv := newTestOAuthServer()

	body := `{"client_name":"Test","redirect_uris":["http://localhost/cb"]}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	srv.HandleRegister(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}
