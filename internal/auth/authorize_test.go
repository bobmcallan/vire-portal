package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHandleAuthorize_ValidFlow(t *testing.T) {
	srv := newTestOAuthServer()

	// Pre-register a client
	srv.clients.Put(&OAuthClient{
		ClientID:     "registered-client",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	})

	challenge := GenerateCodeChallenge("test-verifier")
	reqURL := "/authorize?client_id=registered-client&redirect_uri=" +
		url.QueryEscape("http://localhost:3000/callback") +
		"&response_type=code&code_challenge=" + challenge +
		"&code_challenge_method=S256&state=abc123&scope=openid"

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect: %v", err)
	}
	if parsed.Path != "/" {
		t.Errorf("expected redirect to /, got %s", parsed.Path)
	}
	sessionID := parsed.Query().Get("mcp_session")
	if sessionID == "" {
		t.Error("expected mcp_session query param")
	}

	// Verify cookie set
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mcp_session_id" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("expected mcp_session_id cookie")
	}
	if cookie.Value != sessionID {
		t.Errorf("cookie value should match session ID")
	}
	if !cookie.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if cookie.MaxAge != 600 {
		t.Errorf("expected MaxAge 600, got %d", cookie.MaxAge)
	}

	// Verify session stored
	sess, ok := srv.sessions.Get(sessionID)
	if !ok {
		t.Fatal("expected session to be stored")
	}
	if sess.ClientID != "registered-client" {
		t.Errorf("expected client_id registered-client, got %s", sess.ClientID)
	}
	if sess.CodeChallenge != challenge {
		t.Error("code challenge mismatch in session")
	}
	if sess.State != "abc123" {
		t.Errorf("expected state abc123, got %s", sess.State)
	}
	if sess.Scope != "openid" {
		t.Errorf("expected scope openid, got %s", sess.Scope)
	}
}

func TestHandleAuthorize_DefaultScope(t *testing.T) {
	srv := newTestOAuthServer()
	srv.clients.Put(&OAuthClient{
		ClientID:     "client-1",
		RedirectURIs: []string{"http://localhost/cb"},
	})

	challenge := GenerateCodeChallenge("verifier")
	reqURL := "/authorize?client_id=client-1&redirect_uri=" +
		url.QueryEscape("http://localhost/cb") +
		"&response_type=code&code_challenge=" + challenge +
		"&code_challenge_method=S256&state=xyz"

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	location := rec.Header().Get("Location")
	parsed, _ := url.Parse(location)
	sessionID := parsed.Query().Get("mcp_session")

	sess, _ := srv.sessions.Get(sessionID)
	if sess.Scope != "openid portfolio:read tools:invoke" {
		t.Errorf("expected default scope, got %s", sess.Scope)
	}
}

func TestHandleAuthorize_AutoRegisterClient(t *testing.T) {
	srv := newTestOAuthServer()

	challenge := GenerateCodeChallenge("verifier")
	reqURL := "/authorize?client_id=unknown-client&redirect_uri=" +
		url.QueryEscape("http://localhost/cb") +
		"&response_type=code&code_challenge=" + challenge +
		"&code_challenge_method=S256&state=xyz"

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}

	// Verify client was auto-registered
	client, ok := srv.clients.Get("unknown-client")
	if !ok {
		t.Fatal("expected client to be auto-registered")
	}
	if client.ClientName != "auto-registered" {
		t.Errorf("expected auto-registered name, got %s", client.ClientName)
	}
}

func TestHandleAuthorize_MissingParams(t *testing.T) {
	srv := newTestOAuthServer()

	tests := []struct {
		name  string
		query string
	}{
		{"missing client_id", "redirect_uri=http://localhost/cb&response_type=code&code_challenge=abc&code_challenge_method=S256&state=x"},
		{"missing response_type", "client_id=c1&redirect_uri=http://localhost/cb&code_challenge=abc&code_challenge_method=S256&state=x"},
		{"missing code_challenge", "client_id=c1&redirect_uri=http://localhost/cb&response_type=code&code_challenge_method=S256&state=x"},
		{"missing code_challenge_method", "client_id=c1&redirect_uri=http://localhost/cb&response_type=code&code_challenge=abc&state=x"},
		{"missing state", "client_id=c1&redirect_uri=http://localhost/cb&response_type=code&code_challenge=abc&code_challenge_method=S256"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/authorize?"+tt.query, nil)
			rec := httptest.NewRecorder()

			srv.HandleAuthorize(rec, req)

			// Should redirect to redirect_uri with error
			if rec.Code != http.StatusFound {
				t.Errorf("expected 302, got %d", rec.Code)
			}
			location := rec.Header().Get("Location")
			if location == "" {
				t.Fatal("expected Location header")
			}
			parsed, _ := url.Parse(location)
			if parsed.Query().Get("error") != "invalid_request" {
				t.Errorf("expected error=invalid_request in redirect, got %s", parsed.Query().Get("error"))
			}
		})
	}
}

func TestHandleAuthorize_MissingRedirectURI(t *testing.T) {
	srv := newTestOAuthServer()

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=c1&response_type=code", nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	// Should return error page, not redirect
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing redirect_uri, got %d", rec.Code)
	}
}

func TestHandleAuthorize_InvalidResponseType(t *testing.T) {
	srv := newTestOAuthServer()

	reqURL := "/authorize?client_id=c1&redirect_uri=" +
		url.QueryEscape("http://localhost/cb") +
		"&response_type=token&code_challenge=abc&code_challenge_method=S256&state=x"

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}
	parsed, _ := url.Parse(rec.Header().Get("Location"))
	if parsed.Query().Get("error") != "unsupported_response_type" {
		t.Errorf("expected unsupported_response_type error, got %s", parsed.Query().Get("error"))
	}
}

func TestHandleAuthorize_InvalidChallengeMethod(t *testing.T) {
	srv := newTestOAuthServer()

	reqURL := "/authorize?client_id=c1&redirect_uri=" +
		url.QueryEscape("http://localhost/cb") +
		"&response_type=code&code_challenge=abc&code_challenge_method=plain&state=x"

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}
	parsed, _ := url.Parse(rec.Header().Get("Location"))
	if parsed.Query().Get("error") != "invalid_request" {
		t.Errorf("expected invalid_request error, got %s", parsed.Query().Get("error"))
	}
}

func TestHandleAuthorize_MethodNotAllowed(t *testing.T) {
	srv := newTestOAuthServer()

	req := httptest.NewRequest(http.MethodPost, "/authorize", nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleAuthorize_RedirectURIMismatch(t *testing.T) {
	srv := newTestOAuthServer()
	srv.clients.Put(&OAuthClient{
		ClientID:     "strict-client",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	})

	challenge := GenerateCodeChallenge("verifier")
	reqURL := "/authorize?client_id=strict-client&redirect_uri=" +
		url.QueryEscape("http://evil.com/steal") +
		"&response_type=code&code_challenge=" + challenge +
		"&code_challenge_method=S256&state=xyz"

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	// Per OAuth spec, when redirect_uri doesn't match, return error page (not redirect to unvalidated URI)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for mismatched redirect_uri, got %d", rec.Code)
	}
}
