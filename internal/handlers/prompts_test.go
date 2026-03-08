package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

func newAdminUserLookup() func(string) (*client.UserProfile, error) {
	return func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "admin",
			Email:    "admin@test.com",
			Role:     "admin",
		}, nil
	}
}

func newNonAdminUserLookup() func(string) (*client.UserProfile, error) {
	return func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "user",
			Email:    "user@test.com",
			Role:     "user",
		}, nil
	}
}

var mockPrompts = []client.AdminPrompt{
	{Name: "filing_summary", Description: "Filing summary extraction prompt", Content: "Summarize the filing", Hash: "abc123", IsOverride: false, UpdatedAt: "2026-01-01"},
	{Name: "portfolio_review", Description: "Portfolio review prompt", Content: "Review the portfolio", Hash: "def456", IsOverride: true, UpdatedAt: "2026-01-02"},
}

func TestAdminPromptsHandler_ListAdminAccess(t *testing.T) {
	listFn := func(userID string) ([]client.AdminPrompt, error) {
		return mockPrompts, nil
	}

	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newAdminUserLookup(), listFn, nil, nil)
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/prompts", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "filing_summary") {
		t.Error("expected filing_summary prompt name in response")
	}
	if !strings.Contains(body, "portfolio_review") {
		t.Error("expected portfolio_review prompt name in response")
	}
}

func TestAdminPromptsHandler_ListUnauthenticatedRedirect(t *testing.T) {
	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/admin/prompts", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
}

func TestAdminPromptsHandler_ListNonAdminRedirect(t *testing.T) {
	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newNonAdminUserLookup(), nil, nil, nil)

	req := httptest.NewRequest("GET", "/admin/prompts", nil)
	addAuthCookie(req, "regular-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}
}

func TestAdminPromptsHandler_ListAPIError(t *testing.T) {
	listFn := func(userID string) ([]client.AdminPrompt, error) {
		return nil, ErrTest
	}

	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newAdminUserLookup(), listFn, nil, nil)
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/prompts", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK even with error, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Failed to load prompt list") {
		t.Error("expected error message in response")
	}
}

func TestAdminPromptsHandler_ListNoListFn(t *testing.T) {
	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newAdminUserLookup(), nil, nil, nil)
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/prompts", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Admin API not configured") {
		t.Error("expected config error message in response")
	}
}

func TestAdminPromptsHandler_ListXSSEscaping(t *testing.T) {
	xssPrompts := []client.AdminPrompt{
		{
			Name:        "<script>alert('xss')</script>",
			Description: "<img src=x onerror=alert('xss')>",
			Content:     "safe content",
			Hash:        "abc",
			IsOverride:  false,
			UpdatedAt:   "2026-01-01",
		},
	}

	listFn := func(userID string) ([]client.AdminPrompt, error) {
		return xssPrompts, nil
	}

	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newAdminUserLookup(), listFn, nil, nil)
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/prompts", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "<script>") {
		t.Error("SECURITY: script tag not escaped in prompt data")
	}
	if !strings.Contains(body, "alert") {
		t.Error("expected escaped alert() function name to be visible")
	}
}

func TestAdminPromptsHandler_EditAdminAccess(t *testing.T) {
	getFn := func(userID, name string) (*client.AdminPrompt, error) {
		return &mockPrompts[0], nil
	}

	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newAdminUserLookup(), nil, getFn, nil)
	handler.SetAPIURL("http://localhost:8080")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/prompts/{name}", handler.ServeHTTP)

	req := httptest.NewRequest("GET", "/admin/prompts/filing_summary", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "filing_summary") {
		t.Error("expected prompt name in response")
	}
	if !strings.Contains(body, "Summarize the filing") {
		t.Error("expected prompt content in hydration JSON")
	}
}

func TestAdminPromptsHandler_EditUnauthenticatedRedirect(t *testing.T) {
	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), nil, nil, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/prompts/{name}", handler.ServeHTTP)

	req := httptest.NewRequest("GET", "/admin/prompts/filing_summary", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
}

func TestAdminPromptsHandler_EditNonAdminRedirect(t *testing.T) {
	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newNonAdminUserLookup(), nil, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/prompts/{name}", handler.ServeHTTP)

	req := httptest.NewRequest("GET", "/admin/prompts/filing_summary", nil)
	addAuthCookie(req, "regular-user-id")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}
}

func TestAdminPromptsHandler_EditAPIError(t *testing.T) {
	getFn := func(userID, name string) (*client.AdminPrompt, error) {
		return nil, ErrTest
	}

	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newAdminUserLookup(), nil, getFn, nil)
	handler.SetAPIURL("http://localhost:8080")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/prompts/{name}", handler.ServeHTTP)

	req := httptest.NewRequest("GET", "/admin/prompts/filing_summary", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK even with error, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Failed to load prompt") {
		t.Error("expected error message in response")
	}
}

func TestAdminPromptsHandler_SaveSuccess(t *testing.T) {
	var savedName, savedContent string
	setFn := func(userID, name, content string) error {
		savedName = name
		savedContent = content
		return nil
	}

	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newAdminUserLookup(), nil, nil, setFn)
	handler.SetAPIURL("http://localhost:8080")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/prompts/{name}", handler.ServeHTTP)

	payload, _ := json.Marshal(map[string]string{"content": "Updated content"})
	req := httptest.NewRequest("POST", "/admin/prompts/filing_summary", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %s", resp["status"])
	}
	if savedName != "filing_summary" {
		t.Errorf("expected name filing_summary, got %s", savedName)
	}
	if savedContent != "Updated content" {
		t.Errorf("expected content 'Updated content', got %s", savedContent)
	}
}

func TestAdminPromptsHandler_SaveNonAdmin(t *testing.T) {
	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), newNonAdminUserLookup(), nil, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/prompts/{name}", handler.ServeHTTP)

	payload, _ := json.Marshal(map[string]string{"content": "hack"})
	req := httptest.NewRequest("POST", "/admin/prompts/filing_summary", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookie(req, "regular-user-id")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}
}

func TestAdminPromptsHandler_SaveUnauthenticated(t *testing.T) {
	handler := NewAdminPromptsHandler(nil, false, []byte(testJWTSecret), nil, nil, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/prompts/{name}", handler.ServeHTTP)

	payload, _ := json.Marshal(map[string]string{"content": "hack"})
	req := httptest.NewRequest("POST", "/admin/prompts/filing_summary", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
}
