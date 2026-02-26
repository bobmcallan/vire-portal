package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAuthorizationServer_ReturnsCorrectMetadata(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:8500")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	req.Host = "localhost:8500"
	rec := httptest.NewRecorder()

	h.HandleAuthorizationServer(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("expected Cache-Control public, max-age=3600, got %s", cc)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if body["issuer"] != "http://localhost:8500" {
		t.Errorf("expected issuer http://localhost:8500, got %v", body["issuer"])
	}
	if body["authorization_endpoint"] != "http://localhost:8500/authorize" {
		t.Errorf("expected authorization_endpoint http://localhost:8500/authorize, got %v", body["authorization_endpoint"])
	}
	if body["token_endpoint"] != "http://localhost:8500/token" {
		t.Errorf("expected token_endpoint http://localhost:8500/token, got %v", body["token_endpoint"])
	}
	if body["registration_endpoint"] != "http://localhost:8500/register" {
		t.Errorf("expected registration_endpoint http://localhost:8500/register, got %v", body["registration_endpoint"])
	}

	assertStringSlice(t, body, "response_types_supported", []string{"code"})
	assertStringSlice(t, body, "grant_types_supported", []string{"authorization_code", "refresh_token"})
	assertStringSlice(t, body, "code_challenge_methods_supported", []string{"S256"})
	assertStringSlice(t, body, "token_endpoint_auth_methods_supported", []string{"client_secret_post", "none"})

	scopes := toStringSlice(t, body, "scopes_supported")
	if !contains(scopes, "tools:invoke") {
		t.Errorf("expected scopes_supported to contain tools:invoke, got %v", scopes)
	}
	if !contains(scopes, "openid") {
		t.Errorf("expected scopes_supported to contain openid, got %v", scopes)
	}
}

func TestHandleAuthorizationServer_DifferentBaseURL(t *testing.T) {
	h := NewDiscoveryHandler("https://portal.vire.dev")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	req.Host = "portal.vire.dev"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	h.HandleAuthorizationServer(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if body["issuer"] != "https://portal.vire.dev" {
		t.Errorf("expected issuer https://portal.vire.dev, got %v", body["issuer"])
	}
	if body["authorization_endpoint"] != "https://portal.vire.dev/authorize" {
		t.Errorf("expected authorization_endpoint https://portal.vire.dev/authorize, got %v", body["authorization_endpoint"])
	}
	if body["token_endpoint"] != "https://portal.vire.dev/token" {
		t.Errorf("expected token_endpoint https://portal.vire.dev/token, got %v", body["token_endpoint"])
	}
	if body["registration_endpoint"] != "https://portal.vire.dev/register" {
		t.Errorf("expected registration_endpoint https://portal.vire.dev/register, got %v", body["registration_endpoint"])
	}
}

func TestHandleAuthorizationServer_MethodNotAllowed(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:8500")
	req := httptest.NewRequest(http.MethodPost, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()

	h.HandleAuthorizationServer(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestHandleProtectedResource_ReturnsCorrectMetadata(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:8500")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	req.Host = "localhost:8500"
	rec := httptest.NewRecorder()

	h.HandleProtectedResource(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("expected Cache-Control public, max-age=3600, got %s", cc)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if body["resource"] != "http://localhost:8500" {
		t.Errorf("expected resource http://localhost:8500, got %v", body["resource"])
	}

	assertStringSlice(t, body, "authorization_servers", []string{"http://localhost:8500"})
	assertStringSlice(t, body, "scopes_supported", []string{"portfolio:read", "portfolio:write", "tools:invoke"})
	// Per RFC 9728, bearer_methods_supported indicates how Bearer tokens can be presented
	assertStringSlice(t, body, "bearer_methods_supported", []string{"header"})
}

func TestHandleProtectedResource_MethodNotAllowed(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:8500")
	req := httptest.NewRequest(http.MethodPost, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()

	h.HandleProtectedResource(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

// --- helpers ---

func assertStringSlice(t *testing.T, body map[string]interface{}, key string, expected []string) {
	t.Helper()
	got := toStringSlice(t, body, key)
	if len(got) != len(expected) {
		t.Errorf("expected %s length %d, got %d: %v", key, len(expected), len(got), got)
		return
	}
	for i, v := range expected {
		if got[i] != v {
			t.Errorf("expected %s[%d] = %q, got %q", key, i, v, got[i])
		}
	}
}

func toStringSlice(t *testing.T, body map[string]interface{}, key string) []string {
	t.Helper()
	raw, ok := body[key]
	if !ok {
		t.Fatalf("key %q not found in response", key)
	}
	arr, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("key %q is not an array", key)
	}
	result := make([]string, len(arr))
	for i, v := range arr {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("%s[%d] is not a string: %v", key, i, v)
		}
		result[i] = s
	}
	return result
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
