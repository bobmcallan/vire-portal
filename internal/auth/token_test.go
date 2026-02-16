package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHandleToken_AuthCodeGrant_Success(t *testing.T) {
	srv := newTestOAuthServer()

	verifier := "test-code-verifier-12345"
	challenge := GenerateCodeChallenge(verifier)

	srv.codes.Put(&AuthCode{
		Code:          "valid-code",
		ClientID:      "client-1",
		UserID:        "user-42",
		RedirectURI:   "http://localhost/callback",
		CodeChallenge: challenge,
		Scope:         "openid tools:invoke",
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"client_id":     {"client-1"},
		"redirect_uri":  {"http://localhost/callback"},
		"code_verifier": {verifier},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["access_token"] == nil || resp["access_token"].(string) == "" {
		t.Error("expected non-empty access_token")
	}
	if resp["token_type"] != "Bearer" {
		t.Errorf("expected token_type Bearer, got %v", resp["token_type"])
	}
	if resp["expires_in"] != float64(3600) {
		t.Errorf("expected expires_in 3600, got %v", resp["expires_in"])
	}
	if resp["refresh_token"] == nil || resp["refresh_token"].(string) == "" {
		t.Error("expected non-empty refresh_token")
	}
	if resp["scope"] != "openid tools:invoke" {
		t.Errorf("expected scope 'openid tools:invoke', got %v", resp["scope"])
	}

	// Verify the access token is a valid JWT
	accessToken := resp["access_token"].(string)
	parts := strings.SplitN(accessToken, ".", 3)
	if len(parts) != 3 {
		t.Errorf("access token should have 3 JWT parts, got %d", len(parts))
	}

	// Verify code is marked as used
	code, ok := srv.codes.Get("valid-code")
	if !ok {
		t.Fatal("expected code to still exist")
	}
	if !code.Used {
		t.Error("expected code to be marked as used")
	}

	// Verify Cache-Control header
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("expected Cache-Control no-store, got %s", cc)
	}
}

func TestHandleToken_AuthCodeGrant_CodeNotFound(t *testing.T) {
	srv := newTestOAuthServer()

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"nonexistent"},
		"client_id":     {"client-1"},
		"redirect_uri":  {"http://localhost/cb"},
		"code_verifier": {"verifier"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %s", resp["error"])
	}
}

func TestHandleToken_AuthCodeGrant_CodeAlreadyUsed(t *testing.T) {
	srv := newTestOAuthServer()

	srv.codes.Put(&AuthCode{
		Code:      "used-code",
		ClientID:  "client-1",
		Used:      true,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"used-code"},
		"client_id":     {"client-1"},
		"redirect_uri":  {"http://localhost/cb"},
		"code_verifier": {"verifier"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleToken_AuthCodeGrant_ClientMismatch(t *testing.T) {
	srv := newTestOAuthServer()

	srv.codes.Put(&AuthCode{
		Code:          "code-1",
		ClientID:      "client-a",
		CodeChallenge: GenerateCodeChallenge("verifier"),
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"code-1"},
		"client_id":     {"client-b"},
		"redirect_uri":  {"http://localhost/cb"},
		"code_verifier": {"verifier"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleToken_AuthCodeGrant_RedirectMismatch(t *testing.T) {
	srv := newTestOAuthServer()

	srv.codes.Put(&AuthCode{
		Code:          "code-1",
		ClientID:      "client-1",
		RedirectURI:   "http://localhost/callback",
		CodeChallenge: GenerateCodeChallenge("verifier"),
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"code-1"},
		"client_id":     {"client-1"},
		"redirect_uri":  {"http://evil.com/steal"},
		"code_verifier": {"verifier"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleToken_AuthCodeGrant_PKCEFailure(t *testing.T) {
	srv := newTestOAuthServer()

	srv.codes.Put(&AuthCode{
		Code:          "code-1",
		ClientID:      "client-1",
		RedirectURI:   "http://localhost/cb",
		CodeChallenge: GenerateCodeChallenge("correct-verifier"),
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"code-1"},
		"client_id":     {"client-1"},
		"redirect_uri":  {"http://localhost/cb"},
		"code_verifier": {"wrong-verifier"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %s", resp["error"])
	}
}

func TestHandleToken_AuthCodeGrant_MissingParams(t *testing.T) {
	srv := newTestOAuthServer()

	tests := []struct {
		name string
		form url.Values
	}{
		{"missing code", url.Values{"grant_type": {"authorization_code"}, "client_id": {"c"}, "redirect_uri": {"r"}, "code_verifier": {"v"}}},
		{"missing client_id", url.Values{"grant_type": {"authorization_code"}, "code": {"c"}, "redirect_uri": {"r"}, "code_verifier": {"v"}}},
		{"missing redirect_uri", url.Values{"grant_type": {"authorization_code"}, "code": {"c"}, "client_id": {"c"}, "code_verifier": {"v"}}},
		{"missing code_verifier", url.Values{"grant_type": {"authorization_code"}, "code": {"c"}, "client_id": {"c"}, "redirect_uri": {"r"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(tt.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			srv.HandleToken(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rec.Code)
			}
		})
	}
}

// --- Refresh Token Grant Tests ---

func TestHandleToken_RefreshGrant_Success(t *testing.T) {
	srv := newTestOAuthServer()

	srv.tokens.Put(&RefreshToken{
		Token:     "refresh-123",
		UserID:    "user-42",
		ClientID:  "client-1",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"refresh-123"},
		"client_id":     {"client-1"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["access_token"] == nil || resp["access_token"].(string) == "" {
		t.Error("expected non-empty access_token")
	}
	if resp["refresh_token"] == nil || resp["refresh_token"].(string) == "" {
		t.Error("expected non-empty refresh_token")
	}
	// Should be a new refresh token (rotation)
	if resp["refresh_token"].(string) == "refresh-123" {
		t.Error("expected rotated refresh token, got same one")
	}

	// Old token should be deleted
	_, ok := srv.tokens.Get("refresh-123")
	if ok {
		t.Error("expected old refresh token to be deleted after rotation")
	}
}

func TestHandleToken_RefreshGrant_NotFound(t *testing.T) {
	srv := newTestOAuthServer()

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"nonexistent"},
		"client_id":     {"client-1"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleToken_RefreshGrant_ClientMismatch(t *testing.T) {
	srv := newTestOAuthServer()

	srv.tokens.Put(&RefreshToken{
		Token:     "refresh-1",
		ClientID:  "client-a",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"refresh-1"},
		"client_id":     {"client-b"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleToken_UnsupportedGrantType(t *testing.T) {
	srv := newTestOAuthServer()

	form := url.Values{
		"grant_type": {"client_credentials"},
	}

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "unsupported_grant_type" {
		t.Errorf("expected unsupported_grant_type, got %s", resp["error"])
	}
}

func TestHandleToken_MethodNotAllowed(t *testing.T) {
	srv := newTestOAuthServer()

	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	rec := httptest.NewRecorder()

	srv.HandleToken(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
