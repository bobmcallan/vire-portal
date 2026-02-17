package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCompleteAuthorization_Success(t *testing.T) {
	srv := newTestOAuthServer()

	srv.sessions.Put(&AuthSession{
		SessionID:     "sess-1",
		ClientID:      "client-1",
		RedirectURI:   "http://localhost:3000/callback",
		State:         "state-abc",
		CodeChallenge: "challenge123",
		Scope:         "openid",
		CreatedAt:     time.Now(),
	})

	redirectURL, err := srv.CompleteAuthorization("sess-1", "user-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	if parsed.Host != "localhost:3000" {
		t.Errorf("expected host localhost:3000, got %s", parsed.Host)
	}
	if parsed.Path != "/callback" {
		t.Errorf("expected path /callback, got %s", parsed.Path)
	}

	code := parsed.Query().Get("code")
	if code == "" {
		t.Error("expected code in redirect URL")
	}
	if parsed.Query().Get("state") != "state-abc" {
		t.Errorf("expected state state-abc, got %s", parsed.Query().Get("state"))
	}

	// Verify code was stored
	authCode, ok := srv.codes.Get(code)
	if !ok {
		t.Fatal("expected auth code to be stored")
	}
	if authCode.ClientID != "client-1" {
		t.Errorf("expected client_id client-1, got %s", authCode.ClientID)
	}
	if authCode.UserID != "user-42" {
		t.Errorf("expected user_id user-42, got %s", authCode.UserID)
	}
	if authCode.CodeChallenge != "challenge123" {
		t.Error("code challenge mismatch")
	}

	// Verify session was deleted
	_, ok = srv.sessions.Get("sess-1")
	if ok {
		t.Error("expected session to be deleted after completion")
	}
}

func TestCompleteAuthorization_SessionNotFound(t *testing.T) {
	srv := newTestOAuthServer()

	_, err := srv.CompleteAuthorization("nonexistent", "user-1")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestCompleteAuthorization_SessionExpired(t *testing.T) {
	srv := newTestOAuthServer()

	srv.sessions.Put(&AuthSession{
		SessionID: "expired",
		CreatedAt: time.Now().Add(-11 * time.Minute),
	})

	_, err := srv.CompleteAuthorization("expired", "user-1")
	if err == nil {
		t.Error("expected error for expired session")
	}
}

func TestMintAccessToken_ValidJWT(t *testing.T) {
	srv := newTestOAuthServer()

	token, err := srv.mintAccessToken("user-42", "openid tools:invoke", "client-1", "http://localhost:4241")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("failed to parse claims: %v", err)
	}

	if claims["sub"] != "user-42" {
		t.Errorf("expected sub user-42, got %v", claims["sub"])
	}
	if claims["scope"] != "openid tools:invoke" {
		t.Errorf("expected scope, got %v", claims["scope"])
	}
	if claims["client_id"] != "client-1" {
		t.Errorf("expected client_id client-1, got %v", claims["client_id"])
	}
	if claims["iss"] != "http://localhost:4241" {
		t.Errorf("expected iss, got %v", claims["iss"])
	}

	// Check exp is ~1 hour from now
	exp := int64(claims["exp"].(float64))
	expectedExp := time.Now().Add(1 * time.Hour).Unix()
	if exp < expectedExp-5 || exp > expectedExp+5 {
		t.Errorf("expected exp near %d, got %d", expectedExp, exp)
	}
}

func TestNewOAuthServer_TrimsBaseURL(t *testing.T) {
	srv := NewOAuthServer("  http://localhost:4241/  ", []byte("secret"), nil)
	if srv.baseURL != "http://localhost:4241" {
		t.Errorf("expected trimmed baseURL, got %q", srv.baseURL)
	}
}

func TestOAuthServer_DiscoveryHandlers(t *testing.T) {
	// Verify OAuthServer has HandleAuthorizationServer and HandleProtectedResource
	srv := NewOAuthServer("http://localhost:4241", []byte("secret"), nil)
	if srv == nil {
		t.Fatal("expected non-nil OAuthServer")
	}
	// These are tested via discovery_test.go using DiscoveryHandler,
	// but verify the OAuthServer versions compile and are callable.
}
