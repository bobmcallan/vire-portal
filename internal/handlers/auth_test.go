package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// buildSignedJWT creates an HMAC-SHA256 signed JWT for testing.
func buildSignedJWT(claims map[string]interface{}, secret []byte) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	sigInput := header + "." + payload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + sig
}

// --- ValidateJWT Tests ---

func TestValidateJWT_ValidSignedToken(t *testing.T) {
	secret := []byte("test-secret")
	claims := map[string]interface{}{
		"sub":      "user123",
		"email":    "user@example.com",
		"name":     "Test User",
		"provider": "dev",
		"iss":      "vire-server",
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := buildSignedJWT(claims, secret)

	result, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Sub != "user123" {
		t.Errorf("expected sub user123, got %s", result.Sub)
	}
	if result.Email != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", result.Email)
	}
	if result.Name != "Test User" {
		t.Errorf("expected name Test User, got %s", result.Name)
	}
	if result.Provider != "dev" {
		t.Errorf("expected provider dev, got %s", result.Provider)
	}
	if result.Iss != "vire-server" {
		t.Errorf("expected iss vire-server, got %s", result.Iss)
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	}
	token := buildSignedJWT(claims, secret)

	_, err := ValidateJWT(token, secret)
	if err == nil {
		t.Error("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected 'expired' in error message, got: %v", err)
	}
}

func TestValidateJWT_TamperedToken(t *testing.T) {
	secret := []byte("test-secret")
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := buildSignedJWT(claims, secret)

	// Tamper with the payload
	parts := strings.SplitN(token, ".", 3)
	tamperedClaims, _ := json.Marshal(map[string]interface{}{
		"sub": "admin",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	parts[1] = base64.RawURLEncoding.EncodeToString(tamperedClaims)
	tamperedToken := strings.Join(parts, ".")

	_, err := ValidateJWT(tamperedToken, secret)
	if err == nil {
		t.Error("expected error for tampered token")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected 'signature' in error message, got: %v", err)
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	secret := []byte("correct-secret")
	wrongSecret := []byte("wrong-secret")
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := buildSignedJWT(claims, secret)

	_, err := ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestValidateJWT_EmptySecret_SkipsSignatureCheck(t *testing.T) {
	// With empty secret, signature check is skipped (backwards compat)
	secret := []byte("signing-secret")
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := buildSignedJWT(claims, secret)

	// Validate with empty secret — should pass (skip signature check)
	result, err := ValidateJWT(token, []byte{})
	if err != nil {
		t.Fatalf("expected no error with empty secret, got: %v", err)
	}
	if result.Sub != "user123" {
		t.Errorf("expected sub user123, got %s", result.Sub)
	}
}

func TestValidateJWT_UnsignedTokenWithEmptySecret(t *testing.T) {
	// Dev-mode unsigned token (alg:none) should work with empty secret
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "dev_user",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	result, err := ValidateJWT(token, []byte{})
	if err != nil {
		t.Fatalf("expected no error for unsigned token with empty secret, got: %v", err)
	}
	if result.Sub != "dev_user" {
		t.Errorf("expected sub dev_user, got %s", result.Sub)
	}
}

func TestValidateJWT_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dots", "nodots"},
		{"one dot", "one.dot"},
		{"four dots", "a.b.c.d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateJWT(tt.token, []byte("secret"))
			if err == nil {
				t.Error("expected error for invalid token format")
			}
		})
	}
}

func TestValidateJWT_InvalidBase64Payload(t *testing.T) {
	_, err := ValidateJWT("header.!!!invalid!!!.sig", []byte{})
	if err == nil {
		t.Error("expected error for invalid base64 payload")
	}
}

func TestValidateJWT_InvalidJSONPayload(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	_, err := ValidateJWT("header."+payload+".sig", []byte{})
	if err == nil {
		t.Error("expected error for invalid JSON payload")
	}
}

func TestValidateJWT_MissingExpClaim(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user123"}`))
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	token := header + "." + payload + "."

	_, err := ValidateJWT(token, []byte{})
	if err == nil {
		t.Error("expected error for missing exp claim")
	}
}

// --- IsLoggedIn Tests ---

func TestIsLoggedIn_ValidCookie(t *testing.T) {
	secret := []byte("test-secret")
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := buildSignedJWT(claims, secret)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})

	loggedIn, result := IsLoggedIn(req, secret)
	if !loggedIn {
		t.Error("expected IsLoggedIn to return true")
	}
	if result == nil {
		t.Fatal("expected non-nil claims")
	}
	if result.Sub != "user123" {
		t.Errorf("expected sub user123, got %s", result.Sub)
	}
}

func TestIsLoggedIn_NoCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	loggedIn, claims := IsLoggedIn(req, []byte("secret"))
	if loggedIn {
		t.Error("expected IsLoggedIn to return false with no cookie")
	}
	if claims != nil {
		t.Error("expected nil claims with no cookie")
	}
}

func TestIsLoggedIn_InvalidCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "not-a-jwt"})

	loggedIn, claims := IsLoggedIn(req, []byte("secret"))
	if loggedIn {
		t.Error("expected IsLoggedIn to return false for invalid cookie")
	}
	if claims != nil {
		t.Error("expected nil claims for invalid cookie")
	}
}

func TestIsLoggedIn_ExpiredCookie(t *testing.T) {
	secret := []byte("test-secret")
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	}
	token := buildSignedJWT(claims, secret)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})

	loggedIn, result := IsLoggedIn(req, secret)
	if loggedIn {
		t.Error("expected IsLoggedIn to return false for expired token")
	}
	if result != nil {
		t.Error("expected nil claims for expired token")
	}
}

func TestIsLoggedIn_EmptySecret(t *testing.T) {
	// With empty secret, unsigned tokens should work
	token := buildTestJWT("dev_user")

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})

	loggedIn, result := IsLoggedIn(req, []byte{})
	if !loggedIn {
		t.Error("expected IsLoggedIn to return true with empty secret and valid unsigned token")
	}
	if result == nil || result.Sub != "dev_user" {
		t.Error("expected claims with sub=dev_user")
	}
}

// --- HandleLogin Tests (calls vire-server POST /api/auth/login) ---

func TestHandleLogin_CallsVireServer(t *testing.T) {
	// Mock vire-server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/login" {
			t.Errorf("expected request to /api/auth/login, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if body["username"] != "dev_user" {
			t.Errorf("expected username dev_user, got %s", body["username"])
		}
		if body["password"] != "dev123" {
			t.Errorf("expected password dev123, got %s", body["password"])
		}

		// Return a signed JWT
		secret := []byte("")
		token := buildSignedJWT(map[string]interface{}{
			"sub":      "dev_user",
			"email":    "bobmcallan@gmail.com",
			"provider": "email",
			"iss":      "vire-server",
			"exp":      time.Now().Add(24 * time.Hour).Unix(),
		}, secret)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"token": token,
				"user": map[string]interface{}{
					"username": "dev_user",
					"email":    "bobmcallan@gmail.com",
				},
			},
		})
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=dev_user&password=dev123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}

	// Should set vire_session cookie
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
	if sessionCookie.Value == "" {
		t.Error("expected non-empty cookie value")
	}
	if !sessionCookie.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", sessionCookie.SameSite)
	}
}

func TestHandleLogin_MissingCredentials(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte("secret"))

	tests := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"missing password", "username=dev_user"},
		{"missing username", "password=dev123"},
		{"blank username", "username=&password=dev123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			handler.HandleLogin(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("expected status 302, got %d", w.Code)
			}
			location := w.Header().Get("Location")
			if !strings.Contains(location, "error") {
				t.Errorf("expected redirect with error param, got %s", location)
			}
		})
	}
}

func TestHandleLogin_VireServerError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"status":"error","message":"invalid credentials"}`))
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=dev_user&password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	// Should redirect to / with error on failure
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Errorf("expected redirect with error param, got %s", location)
	}
}

func TestHandleLogin_VireServerUnreachable(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://127.0.0.1:19999", "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=dev_user&password=dev123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Errorf("expected redirect with error param, got %s", location)
	}
}

// --- HandleOAuthCallback Tests ---

func TestHandleOAuthCallback_WithToken(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte(""))

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.sig"
	req := httptest.NewRequest("GET", "/auth/callback?token="+token, nil)
	w := httptest.NewRecorder()

	handler.HandleOAuthCallback(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
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
		t.Fatal("expected vire_session cookie")
	}
	if sessionCookie.Value != token {
		t.Errorf("expected cookie value to be the token")
	}
	if !sessionCookie.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
}

func TestHandleOAuthCallback_NoToken(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("GET", "/auth/callback", nil)
	w := httptest.NewRecorder()

	handler.HandleOAuthCallback(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Errorf("expected redirect with error param, got %s", location)
	}
}

func TestHandleOAuthCallback_EmptyToken(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("GET", "/auth/callback?token=", nil)
	w := httptest.NewRecorder()

	handler.HandleOAuthCallback(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Errorf("expected redirect with error param for empty token, got %s", location)
	}
}

// --- HandleGoogleLogin / HandleGitHubLogin Tests ---

func TestHandleGoogleLogin_RedirectsToVireServer(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:4242", "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()

	handler.HandleGoogleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	expected := "http://localhost:4242/api/auth/login/google?callback=http://localhost:4241/auth/callback"
	if location != expected {
		t.Errorf("expected redirect to %s, got %s", expected, location)
	}
}

func TestHandleGitHubLogin_RedirectsToVireServer(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:4242", "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("GET", "/api/auth/login/github", nil)
	w := httptest.NewRecorder()

	handler.HandleGitHubLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	expected := "http://localhost:4242/api/auth/login/github?callback=http://localhost:4241/auth/callback"
	if location != expected {
		t.Errorf("expected redirect to %s, got %s", expected, location)
	}
}

// --- Redirect Hardcoding Tests ---

func TestHandleLogin_RedirectIsHardcoded(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := buildSignedJWT(map[string]interface{}{
			"sub": "dev_user",
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		}, []byte(""))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": token},
		})
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:4241/auth/callback", []byte(""))

	// Try hostile redirect parameters
	paths := []string{
		"/api/auth/login?redirect=https://evil.com",
		"/api/auth/login?next=https://evil.com",
		"/api/auth/login?return_to=//evil.com",
	}

	for _, path := range paths {
		req := httptest.NewRequest("POST", path, strings.NewReader("username=dev_user&password=dev123"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handler.HandleLogin(w, req)

		location := w.Header().Get("Location")
		if location != "/dashboard" {
			t.Errorf("redirect target influenced by query params: path=%s, location=%s", path, location)
		}
	}
}

func TestHandleOAuthCallback_RedirectIsHardcoded(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte(""))

	paths := []string{
		"/auth/callback?token=tok&redirect=https://evil.com",
		"/auth/callback?token=tok&next=https://evil.com",
	}

	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.HandleOAuthCallback(w, req)

		location := w.Header().Get("Location")
		if location != "/dashboard" {
			t.Errorf("redirect target influenced by query params: path=%s, location=%s", path, location)
		}
	}
}

// --- Cookie Attributes ---

func TestHandleOAuthCallback_CookieAttributes(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("GET", "/auth/callback?token=valid-token", nil)
	w := httptest.NewRecorder()

	handler.HandleOAuthCallback(w, req)

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite should be Lax, got %v", sessionCookie.SameSite)
	}
	if sessionCookie.Path != "/" {
		t.Errorf("cookie path should be /, got %s", sessionCookie.Path)
	}
}

// --- Logout still works with new constructor ---

func TestLogoutHandler_WorksWithNewConstructor(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte(""))

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
}

// --- Concurrent Login with mock server ---

func TestHandleLogin_ConcurrentRequests(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := buildSignedJWT(map[string]interface{}{
			"sub": "dev_user",
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		}, []byte(""))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","data":{"token":"%s"}}`, token)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:4241/auth/callback", []byte(""))

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=dev_user&password=dev123"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			handler.HandleLogin(w, req)
			if w.Code != http.StatusFound {
				t.Errorf("concurrent request got status %d", w.Code)
			}
			done <- true
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
