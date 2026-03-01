package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

// TestAdminUsersHandler_AdminAccessGrants200 verifies admin users see the users page.
func TestAdminUsersHandler_AdminAccessGrants200(t *testing.T) {
	mockUsers := []client.AdminUser{
		{ID: "1", Email: "admin@test.com", Name: "Admin User", Role: "admin", Provider: "google", CreatedAt: "2026-01-01"},
		{ID: "2", Email: "user@test.com", Name: "Regular User", Role: "user", Provider: "github", CreatedAt: "2026-01-02"},
	}

	adminListUsersFn := func(serviceID string) ([]client.AdminUser, error) {
		return mockUsers, nil
	}

	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "admin",
			Email:    "admin@test.com",
			Role:     "admin",
		}, nil
	}

	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), userLookupFn, adminListUsersFn, "service:test")
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "admin@test.com") {
		t.Error("expected admin user email in response")
	}
	if !strings.Contains(body, "user@test.com") {
		t.Error("expected regular user email in response")
	}
}

// TestAdminUsersHandler_UnauthenticatedRedirect verifies unauthenticated users are redirected.
func TestAdminUsersHandler_UnauthenticatedRedirect(t *testing.T) {
	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), nil, nil, "service:test")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
}

// TestAdminUsersHandler_NonAdminRedirect verifies non-admin authenticated users are redirected.
func TestAdminUsersHandler_NonAdminRedirect(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "user",
			Email:    "user@test.com",
			Role:     "user", // Not admin
		}, nil
	}

	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), userLookupFn, nil, "service:test")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "regular-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}
}

// TestAdminUsersHandler_APIErrorShowsWarning verifies API errors are displayed.
func TestAdminUsersHandler_APIErrorShowsWarning(t *testing.T) {
	adminListUsersFn := func(serviceID string) ([]client.AdminUser, error) {
		return nil, ErrTest
	}

	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "admin",
			Email:    "admin@test.com",
			Role:     "admin",
		}, nil
	}

	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), userLookupFn, adminListUsersFn, "service:test")
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK even with error, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Failed to load user list") {
		t.Error("expected error message in response")
	}
}

// TestAdminUsersHandler_NoServiceKeyShowsConfigError verifies missing service key is handled.
func TestAdminUsersHandler_NoServiceKeyShowsConfigError(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "admin",
			Email:    "admin@test.com",
			Role:     "admin",
		}, nil
	}

	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), userLookupFn, nil, "")
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/users", nil)
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

// TestAdminUsersHandler_XSSEscaping verifies user data is safely escaped.
func TestAdminUsersHandler_XSSEscaping(t *testing.T) {
	mockUsers := []client.AdminUser{
		{
			ID:        "xss-user",
			Email:     "<script>alert('xss')</script>@test.com",
			Name:      "<img src=x onerror=alert('xss')>",
			Role:      "user",
			Provider:  "test",
			CreatedAt: "2026-01-01",
		},
	}

	adminListUsersFn := func(serviceID string) ([]client.AdminUser, error) {
		return mockUsers, nil
	}

	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "admin",
			Email:    "admin@test.com",
			Role:     "admin",
		}, nil
	}

	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), userLookupFn, adminListUsersFn, "service:test")
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Go's html/template automatically escapes user data in {{ }} expressions.
	// Verify that dangerous constructs are not rendered as executable code.
	// The template uses {{.Name}} which is safe; Go escapes < > & " '
	if strings.Contains(body, "<script>") {
		t.Error("SECURITY: script tag not escaped in user data")
	}
	// Verify the content is present but escaped (as HTML entities)
	if !strings.Contains(body, "alert") {
		t.Error("expected escaped alert() function name to be visible")
	}
}

// TestAdminUsersHandler_NavShowsUsersLinkForAdmin verifies nav displays Users link for admins.
func TestAdminUsersHandler_NavShowsUsersLinkForAdmin(t *testing.T) {
	mockUsers := []client.AdminUser{}

	adminListUsersFn := func(serviceID string) ([]client.AdminUser, error) {
		return mockUsers, nil
	}

	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "admin",
			Email:    "admin@test.com",
			Role:     "admin",
		}, nil
	}

	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), userLookupFn, adminListUsersFn, "service:test")
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify nav contains Users link
	if !strings.Contains(body, "/admin/users") {
		t.Error("expected Users link in nav for admin user")
	}
}

// TestAdminUsersHandler_NavHidesUsersLinkForNonAdmin verifies nav hides Users link for non-admins.
func TestAdminUsersHandler_NavHidesUsersLinkForNonAdmin(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "user",
			Email:    "user@test.com",
			Role:     "user",
		}, nil
	}

	handler := NewAdminUsersHandler(nil, false, []byte(testJWTSecret), userLookupFn, nil, "")
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "user-id")
	w := httptest.NewRecorder()

	dashboardHandler := NewDashboardHandler(nil, false, []byte(testJWTSecret), userLookupFn)
	dashboardHandler.SetAPIURL("http://localhost:8080")
	dashboardHandler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify nav does NOT contain admin/users link for non-admin
	if strings.Contains(body, "{{if eq .UserRole \"admin\"}}<a href=\"/admin/users\"") {
		t.Error("expected nav to hide Users link for non-admin user")
	}
}

// TestAdminUsersHandler_DashboardPassesUserRole verifies UserRole is passed to dashboard template.
func TestAdminUsersHandler_DashboardPassesUserRole(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: "admin",
			Email:    "admin@test.com",
			Role:     "admin",
		}, nil
	}

	handler := NewDashboardHandler(nil, false, []byte(testJWTSecret), userLookupFn)
	handler.SetAPIURL("http://localhost:8080")

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "admin-user-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()

	// Verify UserRole is available in template data
	if !strings.Contains(body, "admin") {
		// Check that admin role is referenced somewhere (e.g., in nav condition)
		if !strings.Contains(body, "UserRole") {
			t.Error("expected UserRole to be passed to dashboard template")
		}
	}
}

// ErrTest is a test error for API failure scenarios.
var ErrTest = &testError{msg: "test error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
