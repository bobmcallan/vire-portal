package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

func TestAdminGeminiHandler_RequiresLogin(t *testing.T) {
	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %s", loc)
	}
}

func TestAdminGeminiHandler_RequiresAdmin(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "user", Role: "user"}, nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "regular-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", loc)
	}
}

func TestAdminGeminiHandler_RenderPage(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":42,"total_prompt_tokens":1000,"total_output_tokens":500}`), nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "GEMINI USAGE") {
		t.Error("expected GEMINI USAGE heading in page")
	}
	if !strings.Contains(body, "total_calls") {
		t.Error("expected usage data in page")
	}
}

func TestAdminGeminiHandler_NilProxy(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	// proxyGetFn NOT set

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "usage: null") {
		t.Error("expected usage: null when proxyGetFn is nil")
	}
}

func TestAdminGeminiHandler_SetAPIURL(t *testing.T) {
	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetAPIURL("http://test:8080")

	if handler.apiURL != "http://test:8080" {
		t.Errorf("expected apiURL to be set, got %s", handler.apiURL)
	}
}

func TestAdminGeminiHandler_SetProxyGetFn(t *testing.T) {
	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), nil)

	called := false
	fn := func(path, userID string) ([]byte, error) {
		called = true
		return nil, nil
	}
	handler.SetProxyGetFn(fn)

	if handler.proxyGetFn == nil {
		t.Error("expected proxyGetFn to be set")
	}

	handler.proxyGetFn("/test", "user")
	if !called {
		t.Error("expected proxyGetFn to be called")
	}
}
