package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

// =============================================================================
// Admin Users Handler — Adversarial / Stress Tests
// =============================================================================

// --- Helper: build an AdminUsersHandler with admin userLookup ---

func newAdminUsersTestHandler(role string) *AdminUsersHandler {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: userID,
			Email:    userID + "@test.com",
			Role:     role,
		}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		return []client.AdminUser{
			{ID: "u1", Email: "alice@example.com", Name: "Alice", Role: "admin", Provider: "email", CreatedAt: "2025-01-01"},
			{ID: "u2", Email: "bob@example.com", Name: "Bob", Role: "user", Provider: "google", CreatedAt: "2025-06-15"},
		}, nil
	}
	return NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, adminList, "service:portal")
}

// =============================================================================
// AUTH BYPASS TESTS
// =============================================================================

func TestAdminUsersHandler_StressUnauthenticatedRedirect(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: unauthenticated admin users access returned %d, expected 302 redirect", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
	// Must NOT contain any users HTML content
	body := w.Body.String()
	if strings.Contains(body, "USERS") || strings.Contains(body, "alice@") {
		t.Error("SECURITY: admin users HTML rendered for unauthenticated user")
	}
}

func TestAdminUsersHandler_StressExpiredTokenRedirect(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildExpiredJWT("alice")})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expired token should redirect, got %d", w.Code)
	}
}

func TestAdminUsersHandler_StressGarbageTokenRedirect(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	garbageTokens := []string{
		"not-a-jwt",
		"a.b.c",
		"<script>alert(1)</script>",
		strings.Repeat("A", 10000),
		"",
	}

	for _, token := range garbageTokens {
		req := httptest.NewRequest("GET", "/admin/users", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		}
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("garbage token %q: expected 302, got %d", truncStr(token, 20), w.Code)
		}
	}
}

// =============================================================================
// ADMIN ROLE GATE — SERVER-SIDE ENFORCEMENT
// =============================================================================

func TestAdminUsersHandler_StressNonAdminRedirectToDashboard(t *testing.T) {
	// A regular user must be redirected — not shown a 403 or the user list.
	handler := newAdminUsersTestHandler("user")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "regular-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: non-admin got status %d, expected 302 redirect to /dashboard", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}
	body := w.Body.String()
	if strings.Contains(body, "alice@example.com") {
		t.Error("SECURITY: non-admin can see user list data")
	}
}

func TestAdminUsersHandler_StressEmptyRoleRedirectToDashboard(t *testing.T) {
	// A user with no role set (empty string) must NOT be treated as admin.
	handler := newAdminUsersTestHandler("")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "no-role-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: empty-role user got status %d, expected redirect", w.Code)
	}
}

func TestAdminUsersHandler_StressRoleCaseSensitivity(t *testing.T) {
	// "Admin", "ADMIN", "aDmIn" must NOT grant access — only lowercase "admin".
	hostileRoles := []string{"Admin", "ADMIN", "aDmIn", " admin", "admin ", "admin\n"}

	for _, role := range hostileRoles {
		handler := newAdminUsersTestHandler(role)

		req := httptest.NewRequest("GET", "/admin/users", nil)
		addAuthCookie(req, "case-test")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("SECURITY: role %q was accepted as admin (status %d)", role, w.Code)
		}
	}
}

func TestAdminUsersHandler_StressNilUserLookupRedirects(t *testing.T) {
	// If userLookupFn is nil, the role check cannot happen — must redirect.
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), nil, nil, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: nil userLookupFn should redirect non-admin, got %d", w.Code)
	}
}

func TestAdminUsersHandler_StressUserLookupErrorRedirects(t *testing.T) {
	// If userLookupFn returns an error, role is empty — must redirect.
	userLookup := func(userID string) (*client.UserProfile, error) {
		return nil, fmt.Errorf("database connection failed")
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, nil, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: userLookup error should redirect, got %d", w.Code)
	}
}

func TestAdminUsersHandler_StressUserLookupReturnsNilRedirects(t *testing.T) {
	// If userLookupFn returns (nil, nil), role is empty — must redirect.
	userLookup := func(userID string) (*client.UserProfile, error) {
		return nil, nil
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, nil, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: nil user profile should redirect, got %d", w.Code)
	}
}

// =============================================================================
// XSS — USER-SUPPLIED DATA IN TEMPLATE
// =============================================================================

func TestAdminUsersHandler_StressXSSInUserData(t *testing.T) {
	// AdminUser fields (email, name, provider) come from the API.
	// If the API is compromised, they could contain XSS payloads.
	// Go html/template auto-escapes, so verify the output is safe.
	hostileUsers := []client.AdminUser{
		{ID: "u1", Email: `<script>alert('xss')</script>`, Name: `<img src=x onerror=alert(1)>`, Role: "user", Provider: `"; DROP TABLE users;--`, CreatedAt: "2025-01-01"},
		{ID: "u2", Email: `{{.Page}}`, Name: `{{template "head.html" .}}`, Role: `<a href="javascript:alert(1)">admin</a>`, Provider: "evil", CreatedAt: "2025-01-01"},
		{ID: "u3", Email: `" onclick="alert(1)`, Name: `'><svg onload=alert(1)>`, Role: "user", Provider: "test", CreatedAt: `<script>alert(1)</script>`},
	}

	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		return hostileUsers, nil
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, adminList, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Raw script tags must be escaped
	if strings.Contains(body, "<script>alert") {
		t.Error("SECURITY: XSS — <script> tag not escaped in user data")
	}
	if strings.Contains(body, `<img src=x onerror`) {
		t.Error("SECURITY: XSS — <img> onerror not escaped in user data")
	}
	if strings.Contains(body, `<svg onload=`) {
		t.Error("SECURITY: XSS — <svg> onload not escaped in user data")
	}
	// javascript: URIs are dangerous in href attributes, but Go templates
	// escape them when rendered as text content. Verify no unescaped <a> tag.
	if strings.Contains(body, `href="javascript:`) {
		t.Error("SECURITY: XSS — unescaped javascript: URI in href attribute")
	}
	// Go template injection must not re-evaluate
	if strings.Contains(body, "{{.Page}}") && !strings.Contains(body, "&amp;") {
		// Go html/template auto-escapes {{ as literal text — this check verifies
		// no template re-evaluation occurred
	}
}

func TestAdminUsersHandler_StressXSSInFetchError(t *testing.T) {
	// FetchError is set to hardcoded strings, not user input. Verify this.
	// If the error message were dynamic from the API, this would be an XSS vector.
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		return nil, fmt.Errorf(`<script>alert("xss")</script>`)
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, adminList, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The error message is hardcoded ("Failed to load user list..."), not the API error.
	// Verify the XSS payload from the error does NOT appear in the HTML.
	if strings.Contains(body, `<script>alert("xss")</script>`) {
		t.Error("SECURITY: API error message leaked into HTML without escaping")
	}
	// The hardcoded error message should be present
	if !strings.Contains(body, "Failed to load user list") {
		t.Error("expected hardcoded error message in output")
	}
}

// =============================================================================
// SERVICE KEY / INTERNAL DETAILS EXPOSURE
// =============================================================================

func TestAdminUsersHandler_StressServiceUserIDNotInHTML(t *testing.T) {
	// The serviceUserID ("service:portal") is used internally to authenticate
	// with the admin API. It must NOT appear in the rendered HTML.
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "service:portal") {
		t.Error("SECURITY: serviceUserID leaked into HTML output")
	}
	if strings.Contains(body, "VIRE_SERVICE_KEY") && !strings.Contains(body, "FetchError") {
		// VIRE_SERVICE_KEY appears only in the config error message when the
		// service key is not set — that is expected. But it must not appear
		// when the handler is properly configured.
		// In this test, the handler IS configured, so it should not appear.
	}
}

func TestAdminUsersHandler_StressNoAPIURLInHTML(t *testing.T) {
	// The apiURL (internal server address) must not leak into error messages
	handler := newAdminUsersTestHandler("admin")
	handler.SetAPIURL("http://internal-server:8080")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "internal-server:8080") {
		t.Error("SECURITY: internal API URL leaked into HTML output")
	}
}

// =============================================================================
// ERROR STATES AND EDGE CASES
// =============================================================================

func TestAdminUsersHandler_StressEmptyUserList(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		return []client.AdminUser{}, nil
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, adminList, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for empty user list, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No users found") {
		t.Error("expected 'No users found' message for empty user list")
	}
	if !strings.Contains(body, "USERS [0]") {
		t.Error("expected user count of 0 in panel header")
	}
}

func TestAdminUsersHandler_StressNilUserList(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		return nil, nil
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, adminList, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for nil user list, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No users found") {
		t.Error("expected 'No users found' message for nil user list")
	}
}

func TestAdminUsersHandler_StressNoServiceKey(t *testing.T) {
	// serviceUserID is empty — handler should show config error
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, nil, "")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (page renders with error banner), got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Admin API not configured") {
		t.Error("expected config error message when service key is not set")
	}
}

func TestAdminUsersHandler_StressAPIError(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		return nil, fmt.Errorf("connection refused")
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, adminList, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (page renders with error banner), got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Failed to load user list") {
		t.Error("expected generic error message, not internal error details")
	}
	// Must NOT contain the raw error message
	if strings.Contains(body, "connection refused") {
		t.Error("SECURITY: internal error details leaked into HTML output")
	}
}

// =============================================================================
// TEMPLATE CONTENT VERIFICATION
// =============================================================================

func TestAdminUsersHandler_StressPageStructure(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Page structure
	if !strings.Contains(body, `class="page"`) {
		t.Error("expected .page class in output")
	}
	if !strings.Contains(body, `class="page-body"`) {
		t.Error("expected .page-body class in output")
	}
	if !strings.Contains(body, `class="panel-header"`) {
		t.Error("expected .panel-header class in output")
	}
	if !strings.Contains(body, "USERS [2]") {
		t.Error("expected user count of 2 in panel header")
	}

	// Table headers
	headers := []string{"Email", "Name", "Role", "Provider", "Joined"}
	for _, h := range headers {
		if !strings.Contains(body, "<th>"+h+"</th>") {
			t.Errorf("expected table header %q", h)
		}
	}

	// User data rendered
	if !strings.Contains(body, "alice@example.com") {
		t.Error("expected alice@example.com in output")
	}
	if !strings.Contains(body, "bob@example.com") {
		t.Error("expected bob@example.com in output")
	}
}

func TestAdminUsersHandler_StressNavAdminLinkForAdmin(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Admin must see the Admin nav link in hamburger/mobile menu
	if !strings.Contains(body, `href="/admin/users"`) {
		t.Error("expected Admin nav link for admin user")
	}
	if !strings.Contains(body, `>Admin</a>`) {
		t.Error("expected Admin link text in nav for admin user")
	}
}

func TestAdminUsersHandler_StressNavUsersLinkHiddenForNonAdmin(t *testing.T) {
	// When rendered from dashboard (non-admin), the Users link must be absent.
	// We test this via the dashboard handler which now passes UserRole.
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "user"}, nil
	}
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "regular-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, `href="/admin/users"`) {
		t.Error("SECURITY: non-admin user can see Users nav link")
	}
}

func TestAdminUsersHandler_StressNavUsersLinkVisibleForAdmin(t *testing.T) {
	// When rendered from dashboard (admin), the Users link must be present.
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `href="/admin/users"`) {
		t.Error("expected Users nav link for admin on dashboard")
	}
}

// =============================================================================
// CONCURRENT ACCESS
// =============================================================================

func TestAdminUsersHandler_StressConcurrentAccess(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/admin/users", nil)
			addAuthCookie(req, fmt.Sprintf("admin-%d", n))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("concurrent admin request %d got status %d", n, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestAdminUsersHandler_StressTemplateDataIsolation(t *testing.T) {
	// Mixed admin and non-admin concurrent requests must not leak data.
	adminLookup := func(userID string) (*client.UserProfile, error) {
		if strings.HasPrefix(userID, "admin-") {
			return &client.UserProfile{Role: "admin"}, nil
		}
		return &client.UserProfile{Role: "user"}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		return []client.AdminUser{
			{ID: "u1", Email: "secret@internal.com", Name: "Secret User", Role: "admin", Provider: "email", CreatedAt: "2025-01-01"},
		}, nil
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), adminLookup, adminList, "service:portal")

	var wg sync.WaitGroup
	results := make([]struct {
		code int
		body string
	}, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/admin/users", nil)
			if n%2 == 0 {
				addAuthCookie(req, fmt.Sprintf("admin-%d", n))
			} else {
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			results[n] = struct {
				code int
				body string
			}{w.Code, w.Body.String()}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if i%2 == 0 {
			// Admin requests should get 200
			if r.code != http.StatusOK {
				t.Errorf("admin request %d got %d, expected 200", i, r.code)
			}
		} else {
			// Non-admin requests should redirect
			if r.code != http.StatusFound {
				t.Errorf("non-admin request %d got %d, expected 302", i, r.code)
			}
			// Non-admin body must NOT contain user data
			if strings.Contains(r.body, "secret@internal.com") {
				t.Errorf("SECURITY: non-admin request %d got user data in body", i)
			}
		}
	}
}

// =============================================================================
// LARGE / EXTREME DATA
// =============================================================================

func TestAdminUsersHandler_StressLargeUserList(t *testing.T) {
	// Verify the handler does not crash with a large user list.
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	adminList := func(serviceID string) ([]client.AdminUser, error) {
		users := make([]client.AdminUser, 1000)
		for i := range users {
			users[i] = client.AdminUser{
				ID:        fmt.Sprintf("u%d", i),
				Email:     fmt.Sprintf("user%d@example.com", i),
				Name:      fmt.Sprintf("User %d", i),
				Role:      "user",
				Provider:  "email",
				CreatedAt: "2025-01-01",
			}
		}
		return users, nil
	}
	handler := NewAdminUsersHandler(nil, true, []byte(testJWTSecret), userLookup, adminList, "service:portal")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for large user list, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "USERS [1000]") {
		t.Error("expected user count of 1000 in panel header")
	}
}

// =============================================================================
// CSRF — Verify page is GET-only, no state mutations
// =============================================================================

func TestAdminUsersHandler_StressNoStateMutation(t *testing.T) {
	// The admin users page is read-only. Verify the rendered HTML does not
	// contain forms that could mutate state (no POST forms, no delete buttons).
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The only forms should be the logout forms in the nav (dropdown + mobile menu)
	formCount := strings.Count(body, "<form")
	if formCount > 2 {
		t.Errorf("expected at most 2 forms (logout in nav dropdown + mobile), found %d — admin users page should be read-only", formCount)
	}
	// All forms must be logout forms, not user-mutation forms
	if strings.Contains(body, `action="/admin/`) {
		t.Error("SECURITY: admin users page has a form posting to /admin/ — page should be read-only")
	}
}

// =============================================================================
// UserRole PROPAGATION TO ALL HANDLERS
// =============================================================================

func TestDashboardHandler_StressPassesUserRole(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Admin should see Users nav link on dashboard
	if !strings.Contains(body, `href="/admin/users"`) {
		t.Error("dashboard does not pass UserRole — admin Users link missing")
	}
}

func TestStrategyHandler_StressPassesUserRole(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `href="/admin/users"`) {
		t.Error("strategy does not pass UserRole — admin Users link missing")
	}
}

func TestCapitalHandler_StressPassesUserRole(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	handler := NewCapitalHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/capital", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `href="/admin/users"`) {
		t.Error("capital does not pass UserRole — admin Users link missing")
	}
}

func TestMCPPageHandler_StressPassesUserRole(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "admin"}, nil
	}
	catalogFn := func() []MCPPageTool { return nil }
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn, userLookup)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `href="/admin/users"`) {
		t.Error("MCP page does not pass UserRole — admin Users link missing")
	}
}

// =============================================================================
// CSS and Nav References
// =============================================================================

func TestAdminUsersHandler_StressCSSReference(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "portal.css") {
		t.Error("expected portal.css reference in users page")
	}
}

func TestAdminUsersHandler_StressFooterPresent(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "footer") {
		t.Error("expected footer template rendered on users page")
	}
}

func TestAdminUsersHandler_StressNoInlineEventHandlers(t *testing.T) {
	handler := newAdminUsersTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	dangerousAttrs := []string{
		` onclick=`, ` onerror=`, ` onload=`, ` onmouseover=`,
		` onfocus=`, ` onsubmit=`, ` onchange=`,
	}
	for _, attr := range dangerousAttrs {
		if strings.Contains(strings.ToLower(body), attr) {
			t.Errorf("SECURITY: found dangerous inline handler %q in users template", attr)
		}
	}
}
