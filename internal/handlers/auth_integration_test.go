package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Integration Tests: Full Login Round-Trip ---

func TestLoginIntegration_FullRoundTrip(t *testing.T) {
	// Mock vire-server returning a valid JWT
	secret := []byte("")
	claims := map[string]interface{}{
		"sub":      "user42",
		"email":    "user42@example.com",
		"name":     "User 42",
		"provider": "email",
		"iss":      "vire-server",
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	jwt := buildSignedJWT(claims, secret)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/login" {
			t.Errorf("expected path /api/auth/login, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if body["username"] != "testuser" {
			t.Errorf("expected username testuser, got %s", body["username"])
		}
		if body["password"] != "testpass" {
			t.Errorf("expected password testpass, got %s", body["password"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"token": jwt,
			},
		})
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", secret)

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=testuser&password=testpass"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	// Verify 302 redirect to /dashboard
	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}

	// Verify vire_session cookie is set
	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie to be set")
	}

	// Verify cookie contains valid JWT with expected claims
	validatedClaims, err := ValidateJWT(sessionCookie.Value, secret)
	if err != nil {
		t.Fatalf("cookie JWT validation failed: %v", err)
	}
	if validatedClaims.Sub != "user42" {
		t.Errorf("expected sub user42, got %s", validatedClaims.Sub)
	}
	if validatedClaims.Email != "user42@example.com" {
		t.Errorf("expected email user42@example.com, got %s", validatedClaims.Email)
	}
	if validatedClaims.Iss != "vire-server" {
		t.Errorf("expected iss vire-server, got %s", validatedClaims.Iss)
	}
}

func TestLoginIntegration_VireServerReturnsInvalidJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{ broken }`))
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", []byte(""))

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=testuser&password=testpass"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/error?reason=auth_failed" {
		t.Errorf("expected redirect to /error?reason=auth_failed, got %s", location)
	}
}

func TestLoginIntegration_VireServerReturnsEmptyToken(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"token": "",
			},
		})
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", []byte(""))

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=testuser&password=testpass"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/error?reason=auth_failed" {
		t.Errorf("expected redirect to /error?reason=auth_failed, got %s", location)
	}
}

// --- OAuth Redirect Chain Tests ---

func TestOAuthRedirect_GoogleCallbackChain(t *testing.T) {
	apiURL := "http://localhost:4242"
	callbackURL := "http://localhost:8500/auth/callback"
	handler := NewAuthHandler(nil, true, apiURL, callbackURL, []byte(""))

	// Step 1: Verify HandleGoogleLogin builds correct redirect URL
	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	expectedRedirect := apiURL + "/api/auth/login/google?callback=" + callbackURL
	location := w.Header().Get("Location")
	if location != expectedRedirect {
		t.Errorf("expected redirect to %s, got %s", expectedRedirect, location)
	}

	// Step 2: Verify HandleOAuthCallback with token sets cookie and redirects
	token := buildTestJWT("google_user")
	req = httptest.NewRequest("GET", "/auth/callback?token="+token, nil)
	w = httptest.NewRecorder()
	handler.HandleOAuthCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	location = w.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie to be set")
	}
	if sessionCookie.Value != token {
		t.Error("expected cookie value to match the provided token")
	}
}

func TestOAuthRedirect_GitHubCallbackChain(t *testing.T) {
	apiURL := "http://localhost:4242"
	callbackURL := "http://localhost:8500/auth/callback"
	handler := NewAuthHandler(nil, true, apiURL, callbackURL, []byte(""))

	// Step 1: Verify HandleGitHubLogin builds correct redirect URL
	req := httptest.NewRequest("GET", "/api/auth/login/github", nil)
	w := httptest.NewRecorder()
	handler.HandleGitHubLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	expectedRedirect := apiURL + "/api/auth/login/github?callback=" + callbackURL
	location := w.Header().Get("Location")
	if location != expectedRedirect {
		t.Errorf("expected redirect to %s, got %s", expectedRedirect, location)
	}

	// Step 2: Verify HandleOAuthCallback with token sets cookie and redirects
	token := buildTestJWT("github_user")
	req = httptest.NewRequest("GET", "/auth/callback?token="+token, nil)
	w = httptest.NewRecorder()
	handler.HandleOAuthCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", w.Code)
	}
	location = w.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie to be set")
	}
	if sessionCookie.Value != token {
		t.Error("expected cookie value to match the provided token")
	}
}
