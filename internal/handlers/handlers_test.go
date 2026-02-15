package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bobmcallan/vire-portal/internal/models"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	handler := NewHealthHandler(nil)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

func TestHealthHandler_RejectsNonGET(t *testing.T) {
	handler := NewHealthHandler(nil)

	req := httptest.NewRequest("POST", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestVersionHandler_ReturnsJSON(t *testing.T) {
	handler := NewVersionHandler(nil)

	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := body["version"]; !ok {
		t.Error("expected version field in response")
	}
	if _, ok := body["build"]; !ok {
		t.Error("expected build field in response")
	}
	if _, ok := body["git_commit"]; !ok {
		t.Error("expected git_commit field in response")
	}
}

func TestVersionHandler_RejectsNonGET(t *testing.T) {
	handler := NewVersionHandler(nil)

	req := httptest.NewRequest("DELETE", "/api/version", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestRequireMethod_Matches(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	ok := RequireMethod(w, req, "GET")
	if !ok {
		t.Error("expected RequireMethod to return true for matching method")
	}
}

func TestRequireMethod_Mismatch(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	ok := RequireMethod(w, req, "GET")
	if ok {
		t.Error("expected RequireMethod to return false for mismatching method")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	WriteJSON(w, http.StatusCreated, data)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("expected key=value, got key=%s", body["key"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, "something went wrong")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body["error"] != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %s", body["error"])
	}
	if body["status"] != "error" {
		t.Errorf("expected status 'error', got %s", body["status"])
	}
}

// --- Auth Handler Tests ---

func TestDevLoginHandler_DevMode(t *testing.T) {
	handler := NewAuthHandler(nil, true)

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	handler.HandleDevLogin(w, req)

	// In dev mode, POST /api/auth/dev should redirect (302) and set a session cookie
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302 in dev mode, got %d", w.Code)
	}

	// Should set a vire_session cookie
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie to be set in dev mode")
	}
	if !sessionCookie.HttpOnly {
		t.Error("expected vire_session cookie to be httpOnly")
	}
	if sessionCookie.Value == "" {
		t.Error("expected non-empty vire_session cookie value")
	}

	// Cookie value should be a JWT (3 dot-separated base64 segments)
	parts := strings.Split(sessionCookie.Value, ".")
	if len(parts) != 3 {
		t.Errorf("expected JWT with 3 parts, got %d parts", len(parts))
	}

	// Should redirect to /dashboard
	location := w.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}

	// Decode JWT claims and verify email, iss, exp
	if len(parts) == 3 {
		claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatalf("failed to decode JWT payload: %v", err)
		}
		var claims map[string]interface{}
		if err := json.Unmarshal(claimsJSON, &claims); err != nil {
			t.Fatalf("failed to unmarshal JWT claims: %v", err)
		}
		if claims["email"] != "bobmcallan@gmail.com" {
			t.Errorf("expected email bobmcallan@gmail.com, got %v", claims["email"])
		}
		if claims["iss"] != "vire-dev" {
			t.Errorf("expected iss vire-dev, got %v", claims["iss"])
		}
		if claims["sub"] != "dev_user" {
			t.Errorf("expected sub dev_user, got %v", claims["sub"])
		}
		// exp should be ~24h from now (allow 5s tolerance)
		expFloat, ok := claims["exp"].(float64)
		if !ok {
			t.Fatal("expected exp claim to be a number")
		}
		expTime := time.Unix(int64(expFloat), 0)
		expected24h := time.Now().Add(24 * time.Hour)
		if expTime.Before(expected24h.Add(-5*time.Second)) || expTime.After(expected24h.Add(5*time.Second)) {
			t.Errorf("expected exp ~24h from now, got %v", expTime)
		}
	}
}

func TestDevLoginHandler_ProdMode(t *testing.T) {
	handler := NewAuthHandler(nil, false)

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	handler.HandleDevLogin(w, req)

	// In prod mode, POST /api/auth/dev should return 404
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 in prod mode, got %d", w.Code)
	}

	// Verify 404 response body contains proper content
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty 404 response body")
	}
	if !strings.Contains(body, "404") && !strings.Contains(strings.ToLower(body), "not found") {
		t.Errorf("expected 404 body to contain '404' or 'not found', got: %s", body)
	}

	// Should NOT set a vire_session cookie
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "vire_session" {
			t.Error("expected no vire_session cookie in prod mode")
		}
	}
}

// Note: Method filtering for POST /api/auth/dev is handled by Go 1.22+
// pattern routing ("POST /api/auth/dev"), not by the handler itself.
// The route-level test TestRoutes_DevAuthEndpoint_DevMode validates this.

// --- Dashboard Handler Tests ---

func TestDashboardHandler_Returns200(t *testing.T) {
	tools := []DashboardTool{
		{Name: "get_summary", Description: "Get portfolio summary", Method: "GET", Path: "/api/portfolios/{name}/summary"},
		{Name: "list_holdings", Description: "List holdings", Method: "GET", Path: "/api/portfolios/{name}/holdings"},
	}
	catalogFn := func() []DashboardTool { return tools }

	handler := NewDashboardHandler(nil, true, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty HTML body")
	}
}

func TestDashboardHandler_ContainsToolCatalog(t *testing.T) {
	tools := []DashboardTool{
		{Name: "get_summary", Description: "Get portfolio summary", Method: "GET", Path: "/api/portfolios/{name}/summary"},
		{Name: "compute_indicators", Description: "Compute technical indicators", Method: "POST", Path: "/api/indicators"},
	}
	catalogFn := func() []DashboardTool { return tools }

	handler := NewDashboardHandler(nil, false, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should display tool names
	if !strings.Contains(body, "get_summary") {
		t.Error("expected dashboard to contain tool name 'get_summary'")
	}
	if !strings.Contains(body, "compute_indicators") {
		t.Error("expected dashboard to contain tool name 'compute_indicators'")
	}

	// Should display tool descriptions
	if !strings.Contains(body, "Get portfolio summary") {
		t.Error("expected dashboard to contain tool description")
	}
}

func TestDashboardHandler_ContainsMCPConnectionConfig(t *testing.T) {
	catalogFn := func() []DashboardTool { return nil }

	handler := NewDashboardHandler(nil, false, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should contain MCP endpoint
	if !strings.Contains(body, "/mcp") {
		t.Error("expected dashboard to contain /mcp endpoint")
	}

	// Should contain Claude Code JSON config
	if !strings.Contains(body, "mcpServers") {
		t.Error("expected dashboard to contain mcpServers JSON config")
	}

	// Should contain the port in the endpoint URL
	if !strings.Contains(body, "4241") {
		t.Error("expected dashboard to contain port 4241 in MCP endpoint")
	}
}

func TestDashboardHandler_ShowsEmptyToolsMessage(t *testing.T) {
	catalogFn := func() []DashboardTool { return nil }

	handler := NewDashboardHandler(nil, false, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// With no tools, should show a "no tools" indicator
	if !strings.Contains(body, "NO TOOLS") {
		t.Error("expected dashboard to show NO TOOLS message when catalog is empty")
	}
}

func TestDashboardHandler_XSSEscaping(t *testing.T) {
	tools := []DashboardTool{
		{Name: "<script>alert('xss')</script>", Description: "<img onerror=alert(1) src=x>"},
	}
	catalogFn := func() []DashboardTool { return tools }

	handler := NewDashboardHandler(nil, false, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// html/template auto-escapes; verify raw HTML tags are NOT present
	if strings.Contains(body, "<script>") {
		t.Error("expected <script> tag to be escaped in dashboard output")
	}
	if strings.Contains(body, "<img onerror") {
		t.Error("expected <img onerror> to be escaped in dashboard output")
	}

	// The escaped forms should be present
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("expected escaped &lt;script&gt; in dashboard output")
	}
}

func TestDashboardHandler_ToolCount(t *testing.T) {
	tools := []DashboardTool{
		{Name: "tool_a", Description: "Tool A"},
		{Name: "tool_b", Description: "Tool B"},
		{Name: "tool_c", Description: "Tool C"},
	}
	catalogFn := func() []DashboardTool { return tools }

	handler := NewDashboardHandler(nil, false, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should show the tool count
	if !strings.Contains(body, "3") {
		t.Error("expected dashboard to display tool count of 3")
	}
}

// --- Dev Login Stress Tests ---

func TestDevLoginHandler_RedirectIsHardcoded(t *testing.T) {
	// Verify the redirect target cannot be influenced by request parameters.
	// An open redirect would allow phishing: POST /api/auth/dev?redirect=https://evil.com
	handler := NewAuthHandler(nil, true)

	// Try various hostile redirect parameters
	paths := []string{
		"/api/auth/dev?redirect=https://evil.com",
		"/api/auth/dev?next=https://evil.com",
		"/api/auth/dev?return_to=//evil.com",
		"/api/auth/dev?redirect_uri=https://evil.com/steal",
	}

	for _, path := range paths {
		req := httptest.NewRequest("POST", path, nil)
		w := httptest.NewRecorder()
		handler.HandleDevLogin(w, req)

		location := w.Header().Get("Location")
		if location != "/dashboard" {
			t.Errorf("redirect target influenced by query params: path=%s, location=%s", path, location)
		}
	}
}

func TestDevLoginHandler_CookieAttributes(t *testing.T) {
	// Verify session cookie has secure attributes to prevent theft.
	handler := NewAuthHandler(nil, true)

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()
	handler.HandleDevLogin(w, req)

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie")
	}

	// HttpOnly prevents XSS from reading the cookie
	if !sessionCookie.HttpOnly {
		t.Error("vire_session cookie must be HttpOnly to prevent XSS cookie theft")
	}

	// SameSite should be Lax or Strict to prevent CSRF
	if sessionCookie.SameSite != http.SameSiteLaxMode && sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("vire_session cookie SameSite should be Lax or Strict, got %v", sessionCookie.SameSite)
	}

	// Path should be / (not a subdirectory that could leak to other paths)
	if sessionCookie.Path != "/" {
		t.Errorf("vire_session cookie path should be /, got %s", sessionCookie.Path)
	}
}

func TestDevLoginHandler_JWTClaimsAreValid(t *testing.T) {
	// Verify JWT claims cannot be used to escalate privileges.
	handler := NewAuthHandler(nil, true)

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()
	handler.HandleDevLogin(w, req)

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie")
	}

	parts := strings.Split(sessionCookie.Value, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3-part JWT, got %d parts", len(parts))
	}

	// Verify header uses alg:none (dev-only, unsigned)
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("failed to decode JWT header: %v", err)
	}
	var header map[string]interface{}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("failed to unmarshal JWT header: %v", err)
	}
	if header["alg"] != "none" {
		t.Errorf("dev JWT should use alg:none, got %v", header["alg"])
	}

	// Verify issuer is vire-dev (not a production issuer)
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode JWT payload: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatalf("failed to unmarshal JWT claims: %v", err)
	}
	if claims["iss"] != "vire-dev" {
		t.Errorf("dev JWT issuer must be 'vire-dev' to distinguish from prod tokens, got %v", claims["iss"])
	}

	// Verify expiry is bounded (not infinite)
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("expected exp claim in JWT")
	}
	expTime := time.Unix(int64(expFloat), 0)
	maxExpiry := time.Now().Add(25 * time.Hour) // 24h + tolerance
	if expTime.After(maxExpiry) {
		t.Errorf("dev JWT expiry too far in future: %v (max 24h)", expTime)
	}

	// Signature part must be empty (unsigned token)
	if parts[2] != "" {
		t.Errorf("dev JWT should have empty signature, got %q", parts[2])
	}
}

func TestDevLoginHandler_ProdModeBlocksCompletely(t *testing.T) {
	// Verify prod mode returns no cookies, no redirect, and correct error.
	handler := NewAuthHandler(nil, false)

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()
	handler.HandleDevLogin(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 in prod mode, got %d", w.Code)
	}

	// Must not set any cookies at all
	cookies := w.Result().Cookies()
	if len(cookies) > 0 {
		names := make([]string, len(cookies))
		for i, c := range cookies {
			names[i] = c.Name
		}
		t.Errorf("expected no cookies in prod mode, got: %v", names)
	}

	// Must not include a Location header (no redirect)
	if location := w.Header().Get("Location"); location != "" {
		t.Errorf("expected no redirect in prod mode, got Location: %s", location)
	}
}

// --- Logout Handler Tests ---

func TestLogoutHandler_ClearsCookie(t *testing.T) {
	handler := NewAuthHandler(nil, true)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	// Should redirect to /
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}

	// Should set vire_session cookie with MaxAge -1 (delete)
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie to be set (for deletion)")
	}
	if sessionCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1 (delete), got %d", sessionCookie.MaxAge)
	}
	if sessionCookie.Value != "" {
		t.Errorf("expected empty cookie value, got %q", sessionCookie.Value)
	}
}

func TestLogoutHandler_WorksWithoutExistingCookie(t *testing.T) {
	handler := NewAuthHandler(nil, false)

	// No cookie on request
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
}

// --- ServePage LoggedIn Tests ---

func TestServePage_LoggedIn_WithCookie(t *testing.T) {
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// When logged in, the nav should be rendered
	body := w.Body.String()
	if !strings.Contains(body, "Dashboard") {
		t.Error("expected nav with Dashboard link when LoggedIn=true")
	}
}

func TestServePage_LoggedIn_WithoutCookie(t *testing.T) {
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	// No cookie
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// When not logged in, nav should NOT appear (no nav burger)
	body := w.Body.String()
	if strings.Contains(body, "nav-burger") {
		t.Error("expected no nav-burger when LoggedIn=false")
	}
}

func TestDevLoginHandler_ConcurrentRequests(t *testing.T) {
	// Verify concurrent dev logins don't cause race conditions.
	handler := NewAuthHandler(nil, true)

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/api/auth/dev", nil)
			w := httptest.NewRecorder()
			handler.HandleDevLogin(w, req)

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

// --- Logout Handler Stress Tests ---

func TestLogoutHandler_CookieAttributes(t *testing.T) {
	handler := NewAuthHandler(nil, true)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected vire_session cookie in logout response")
	}

	// HttpOnly must be set to prevent JS from reading the deletion cookie
	if !sessionCookie.HttpOnly {
		t.Error("logout cookie must be HttpOnly")
	}

	// Path must be / to match the original cookie path
	if sessionCookie.Path != "/" {
		t.Errorf("logout cookie path should be /, got %s", sessionCookie.Path)
	}
}

func TestLogoutHandler_RedirectIsHardcoded(t *testing.T) {
	// Verify logout redirect cannot be influenced by query params (open redirect)
	handler := NewAuthHandler(nil, true)

	paths := []string{
		"/api/auth/logout?redirect=https://evil.com",
		"/api/auth/logout?next=https://evil.com",
		"/api/auth/logout?return_to=//evil.com",
	}

	for _, path := range paths {
		req := httptest.NewRequest("POST", path, nil)
		w := httptest.NewRecorder()
		handler.HandleLogout(w, req)

		location := w.Header().Get("Location")
		if location != "/" {
			t.Errorf("logout redirect influenced by query params: path=%s, location=%s", path, location)
		}
	}
}

func TestLogoutHandler_ConcurrentRequests(t *testing.T) {
	handler := NewAuthHandler(nil, true)

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/api/auth/logout", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: "token"})
			w := httptest.NewRecorder()
			handler.HandleLogout(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("concurrent logout got status %d", w.Code)
			}
			done <- true
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestLogoutHandler_WorksInProdMode(t *testing.T) {
	// Logout should work regardless of dev/prod mode (unlike login)
	handler := NewAuthHandler(nil, false)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 for logout in prod mode, got %d", w.Code)
	}
}

// --- LoggedIn Edge Cases ---

func TestServePage_LoggedIn_EmptyCookieValue(t *testing.T) {
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	// Go's Cookie method returns the cookie even with empty value
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: ""})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Even with empty cookie value, r.Cookie() returns nil error
	// so LoggedIn will be true. This is acceptable because:
	// 1. Session validation is out of scope (noted in requirements)
	// 2. The nav showing for an invalid session is harmless (MCP calls will just have no user context)
}

func TestServePage_LoggedIn_GarbageCookieValue(t *testing.T) {
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "not-a-jwt-at-all"})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	// Page should render without crashing, nav will show (LoggedIn=true based on cookie presence)
}

// --- extractJWTSub Tests ---

func TestExtractJWTSub_ValidToken(t *testing.T) {
	// Build a valid dev JWT
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub":   "dev_user",
		"email": "test@test.com",
		"iss":   "vire-dev",
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	sub := ExtractJWTSub(token)
	if sub != "dev_user" {
		t.Errorf("expected sub 'dev_user', got %q", sub)
	}
}

func TestExtractJWTSub_InvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"no dots", "nodots"},
		{"one dot", "one.dot"},
		{"invalid base64 payload", "header.!!!invalid!!!.sig"},
		{"invalid json payload", "header." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".sig"},
		{"missing sub claim", "header." + base64.RawURLEncoding.EncodeToString([]byte(`{"email":"test"}`)) + ".sig"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := ExtractJWTSub(tt.token)
			if sub != "" {
				t.Errorf("expected empty sub for %q, got %q", tt.name, sub)
			}
		})
	}
}

// --- Settings Handler Tests ---

func TestSettingsHandler_GET_NoKey(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: ""}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	// Build a dev JWT cookie
	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "No API key configured") {
		t.Error("expected 'No API key configured' when user has no key")
	}
}

func TestSettingsHandler_GET_WithKey(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: "sk-test-abc123"}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "****c123") {
		t.Error("expected masked key preview '****c123'")
	}
}

func TestSettingsHandler_POST_SavesKey(t *testing.T) {
	var savedKey string
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: ""}, nil
	}
	saveFn := func(u *models.User) error {
		savedKey = u.NavexaKey
		return nil
	}

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=my-new-key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/settings?saved=1" {
		t.Errorf("expected redirect to /settings?saved=1, got %s", location)
	}
	if savedKey != "my-new-key" {
		t.Errorf("expected saved key 'my-new-key', got %q", savedKey)
	}
}

func TestSettingsHandler_POST_NoCookie(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=my-key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No cookie
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSettingsHandler_GET_NoCookie(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	req := httptest.NewRequest("GET", "/settings", nil)
	// No cookie
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "sign in") {
		t.Error("expected sign-in message when not logged in")
	}
}

func TestSettingsHandler_GET_SavedBanner(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: "key123"}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings?saved=1", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Settings saved successfully") {
		t.Error("expected 'Settings saved successfully' banner when saved=1")
	}
}

// --- Dashboard NavexaKeyMissing Tests ---

func TestDashboardHandler_NavexaKeyMissing_WhenEmpty(t *testing.T) {
	catalogFn := func() []DashboardTool { return nil }
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: ""}, nil
	}

	handler := NewDashboardHandler(nil, true, 4241, catalogFn, lookupFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "WARNING") {
		t.Error("expected WARNING banner when Navexa key is empty")
	}
	if !strings.Contains(body, "/settings") {
		t.Error("expected link to /settings in warning banner")
	}
}

func TestDashboardHandler_NavexaKeyMissing_WhenSet(t *testing.T) {
	catalogFn := func() []DashboardTool { return nil }
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: "some-key"}, nil
	}

	handler := NewDashboardHandler(nil, true, 4241, catalogFn, lookupFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "WARNING") {
		t.Error("expected no WARNING banner when Navexa key is set")
	}
}

func TestDashboardHandler_NavexaKeyMissing_NotLoggedIn(t *testing.T) {
	catalogFn := func() []DashboardTool { return nil }
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: ""}, nil
	}

	handler := NewDashboardHandler(nil, true, 4241, catalogFn, lookupFn)

	// No cookie
	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "WARNING") {
		t.Error("expected no WARNING banner when not logged in")
	}
}

// --- Settings Handler Stress Tests ---

func TestSettingsHandler_POST_EmptyCookie(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: ""})
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty cookie value, got %d", w.Code)
	}
}

func TestSettingsHandler_POST_GarbageJWT(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	garbageTokens := []string{
		"not-a-jwt",
		"a.b",
		"....",
		"a." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".c",
	}

	for _, token := range garbageTokens {
		req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=key"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w := httptest.NewRecorder()

		handler.HandleSaveSettings(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for garbage JWT %q, got %d", token, w.Code)
		}
	}
}

func TestSettingsHandler_POST_UnknownUser(t *testing.T) {
	// User lookup returns not-found error
	lookupFn := func(userID string) (*models.User, error) {
		return nil, fmt.Errorf("user not found")
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	token := buildTestJWT("nonexistent_user")
	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown user, got %d", w.Code)
	}
}

func TestSettingsHandler_POST_HostileInputs(t *testing.T) {
	// Verify hostile API key inputs are stored as-is (not interpreted) and don't crash.
	// html/template handles escaping on output — the storage layer should accept arbitrary strings.
	var savedKeys []string
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error {
		savedKeys = append(savedKeys, u.NavexaKey)
		return nil
	}

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	hostileInputs := []struct {
		name     string
		input    string
		expected string // after TrimSpace
	}{
		{"script tag", "<script>alert('xss')</script>", "<script>alert('xss')</script>"},
		{"sql injection", "'; DROP TABLE users; --", "'; DROP TABLE users; --"},
		{"html entities", "&lt;img onerror=alert(1)&gt;", "&lt;img onerror=alert(1)&gt;"},
		{"newlines", "key\nwith\nnewlines", "key\nwith\nnewlines"},
		{"null bytes", "key\x00with\x00nulls", "key\x00with\x00nulls"},
		{"unicode", "key\u200b\u00e9\u00fc\u2603", "key\u200b\u00e9\u00fc\u2603"},
		{"empty after trim", "   ", ""},
		{"spaces around key", "  real-key  ", "real-key"},
	}

	for _, tc := range hostileInputs {
		t.Run(tc.name, func(t *testing.T) {
			savedKeys = nil
			token := buildTestJWT("dev_user")
			formData := url.Values{"navexa_key": {tc.input}}
			req := httptest.NewRequest("POST", "/settings", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			handler.HandleSaveSettings(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("expected 302 for input %q, got %d", tc.name, w.Code)
			}
			if len(savedKeys) != 1 {
				t.Fatalf("expected 1 save call, got %d", len(savedKeys))
			}
			if savedKeys[0] != tc.expected {
				t.Errorf("expected saved key %q, got %q", tc.expected, savedKeys[0])
			}
		})
	}
}

func TestSettingsHandler_POST_VeryLongKey(t *testing.T) {
	// The 1MB body size limit from middleware protects against extreme payloads,
	// but test that a moderately long key doesn't crash the handler.
	var savedKey string
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error {
		savedKey = u.NavexaKey
		return nil
	}

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	longKey := strings.Repeat("A", 10000)
	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key="+longKey))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 for long key, got %d", w.Code)
	}
	if len(savedKey) != 10000 {
		t.Errorf("expected 10000-char key to be saved, got %d chars", len(savedKey))
	}
}

func TestSettingsHandler_POST_StorageFailure(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error {
		return fmt.Errorf("database connection lost")
	}

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on storage failure, got %d", w.Code)
	}
}

func TestSettingsHandler_POST_NilLookupAndSaveFn(t *testing.T) {
	// Misconfigured handler with nil function pointers should not panic
	handler := NewSettingsHandler(nil, true, nil, nil)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nil lookup/save functions, got %d", w.Code)
	}
}

func TestSettingsHandler_GET_XSSInNavexaKeyPreview(t *testing.T) {
	// If a hostile key was stored, verify the preview is HTML-escaped on output
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: `<img onerror=alert(1) src=x>`}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()

	// html/template auto-escapes; the raw tag should not appear
	if strings.Contains(body, "<img onerror") {
		t.Error("NavexaKeyPreview contains unescaped HTML — XSS vulnerability")
	}
	// The last 4 chars of the hostile key are "=x>" which should be escaped
	if strings.Contains(body, "=x>") && !strings.Contains(body, "=x&gt;") {
		t.Error("expected > to be escaped to &gt; in key preview")
	}
}

func TestSettingsHandler_GET_ShortKeyExposesFullKey(t *testing.T) {
	// Keys shorter than 4 chars: the preview shows the entire key.
	// Verify this works but document the information leakage concern.
	tests := []struct {
		key     string
		preview string
	}{
		{"abc", "abc"},    // 3-char key fully exposed
		{"ab", "ab"},      // 2-char key fully exposed
		{"a", "a"},        // 1-char key fully exposed
		{"abcd", "abcd"},  // 4-char key: shows last 4 (all of it)
		{"abcde", "bcde"}, // 5-char key: shows last 4
	}

	for _, tc := range tests {
		t.Run("key_len_"+tc.key, func(t *testing.T) {
			lookupFn := func(userID string) (*models.User, error) {
				return &models.User{Username: "dev_user", NavexaKey: tc.key}, nil
			}
			handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

			token := buildTestJWT("dev_user")
			req := httptest.NewRequest("GET", "/settings", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			handler.HandleSettings(w, req)

			body := w.Body.String()
			if !strings.Contains(body, "****"+tc.preview) {
				t.Errorf("expected preview '****%s' in body for key %q", tc.preview, tc.key)
			}
		})
	}
}

func TestSettingsHandler_GET_SavedQueryParamInjection(t *testing.T) {
	// Verify only "saved=1" triggers the banner; other values don't cause issues
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

	tests := []struct {
		query      string
		shouldShow bool
	}{
		{"?saved=1", true},
		{"?saved=0", false},
		{"?saved=true", false},
		{"?saved=<script>alert(1)</script>", false},
		{"?saved=", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run("query_"+tc.query, func(t *testing.T) {
			token := buildTestJWT("dev_user")
			req := httptest.NewRequest("GET", "/settings"+tc.query, nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			handler.HandleSettings(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}

			body := w.Body.String()
			hasBanner := strings.Contains(body, "Settings saved successfully")
			if hasBanner != tc.shouldShow {
				t.Errorf("query=%q: expected banner=%v, got %v", tc.query, tc.shouldShow, hasBanner)
			}

			// Verify hostile query values don't appear unescaped in output
			if strings.Contains(body, "alert(1)") {
				t.Errorf("query=%q: injected content found in output — XSS", tc.query)
			}
		})
	}
}

func TestSettingsHandler_POST_ConcurrentSaves(t *testing.T) {
	var saveCount atomic.Int32
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error {
		saveCount.Add(1)
		return nil
	}

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func(n int) {
			token := buildTestJWT("dev_user")
			key := fmt.Sprintf("key-%d", n)
			req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key="+key))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			handler.HandleSaveSettings(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("concurrent save %d got status %d", n, w.Code)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestSettingsHandler_POST_RedirectIsHardcoded(t *testing.T) {
	// Verify the redirect target after save cannot be influenced by request parameters
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	saveFn := func(u *models.User) error { return nil }

	handler := NewSettingsHandler(nil, true, lookupFn, saveFn)

	paths := []string{
		"/settings?redirect=https://evil.com",
		"/settings?next=https://evil.com",
		"/settings?return_to=//evil.com",
	}

	for _, path := range paths {
		token := buildTestJWT("dev_user")
		req := httptest.NewRequest("POST", path, strings.NewReader("navexa_key=key"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w := httptest.NewRecorder()

		handler.HandleSaveSettings(w, req)

		location := w.Header().Get("Location")
		if location != "/settings?saved=1" {
			t.Errorf("redirect target influenced by query params: path=%s, location=%s", path, location)
		}
	}
}

func TestSettingsHandler_GET_LookupFailure(t *testing.T) {
	// If the DB is unavailable during GET, the page should still render (just without key info)
	lookupFn := func(userID string) (*models.User, error) {
		return nil, fmt.Errorf("database unavailable")
	}
	handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even with DB failure, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "SETTINGS") {
		t.Error("expected settings page to render despite DB failure")
	}
}

// --- Dashboard NavexaKeyMissing Stress Tests ---

func TestDashboardHandler_NavexaKeyMissing_LookupFailure(t *testing.T) {
	// If user lookup fails, the warning should NOT show (fail open, don't crash)
	catalogFn := func() []DashboardTool { return nil }
	lookupFn := func(userID string) (*models.User, error) {
		return nil, fmt.Errorf("database unavailable")
	}

	handler := NewDashboardHandler(nil, true, 4241, catalogFn, lookupFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even with DB failure, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "WARNING") {
		t.Error("warning should not show when user lookup fails")
	}
}

func TestDashboardHandler_NavexaKeyMissing_GarbageCookie(t *testing.T) {
	catalogFn := func() []DashboardTool { return nil }
	lookupCalled := false
	lookupFn := func(userID string) (*models.User, error) {
		lookupCalled = true
		return &models.User{Username: userID, NavexaKey: ""}, nil
	}

	handler := NewDashboardHandler(nil, true, 4241, catalogFn, lookupFn)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "garbage-not-jwt"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// With a garbage JWT, ExtractJWTSub returns "" so lookup should not be called
	// OR if called with "", it shouldn't show the warning
	body := w.Body.String()
	if strings.Contains(body, "WARNING") && lookupCalled {
		t.Error("warning should not show when JWT is invalid")
	}
}

// --- Component Library Stress Tests ---

func TestNavTemplate_ContainsLogoutPostForm(t *testing.T) {
	// The logout must be a POST form (not a GET link) for CSRF protection.
	// A GET link would be vulnerable to CSRF via <img src="/api/auth/logout">.
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	// Verify logout is a POST form, not a GET link
	if !strings.Contains(body, `method="POST"`) || !strings.Contains(body, `/api/auth/logout`) {
		t.Error("logout must be a POST form action, not a GET link")
	}
	if strings.Contains(body, `href="/api/auth/logout"`) {
		t.Error("logout must NOT be an <a href> link — vulnerable to CSRF via GET")
	}
}

func TestNavTemplate_MobileMenuPresent(t *testing.T) {
	// Verify mobile menu elements exist when logged in.
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "nav-hamburger") {
		t.Error("expected nav-hamburger button for mobile menu")
	}
	if !strings.Contains(body, "mobile-menu") {
		t.Error("expected mobile-menu element")
	}
	if !strings.Contains(body, "mobile-overlay") {
		t.Error("expected mobile-overlay element to block interaction")
	}
}

func TestNavTemplate_DropdownPresent(t *testing.T) {
	// Verify Alpine.js dropdown component is present in nav.
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `x-data="dropdown()"`) {
		t.Error("expected Alpine.js dropdown component in nav")
	}
	if !strings.Contains(body, `x-data="mobileMenu()"`) {
		t.Error("expected Alpine.js mobileMenu component wrapping nav")
	}
}

func TestNavTemplate_NotRenderedWhenLoggedOut(t *testing.T) {
	// When not logged in, nav should not render at all.
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	// No cookie
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	if strings.Contains(body, "nav-hamburger") {
		t.Error("nav-hamburger should not appear when logged out")
	}
	if strings.Contains(body, "dropdown") {
		t.Error("dropdown should not appear when logged out")
	}
	if strings.Contains(body, `action="/api/auth/logout"`) {
		t.Error("logout form should not appear when logged out")
	}
}

func TestNavTemplate_ActiveStateForDashboard(t *testing.T) {
	// Verify Dashboard link gets "active" class when Page="dashboard".
	catalogFn := func() []DashboardTool { return nil }
	handler := NewDashboardHandler(nil, true, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The nav template uses {{if eq .Page "dashboard"}}class="active"{{end}}
	if !strings.Contains(body, `class="active"`) {
		t.Error("expected Dashboard link to have 'active' class on dashboard page")
	}
}

func TestFooterTemplate_ToastContainer(t *testing.T) {
	// Verify the toast notification container is rendered.
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "toast-container") {
		t.Error("expected toast-container in footer")
	}
	if !strings.Contains(body, `x-data="toasts()"`) {
		t.Error("expected Alpine.js toasts component in footer")
	}
}

func TestFooterTemplate_GitHubLink(t *testing.T) {
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "github.com/bobmcallan/vire-portal") {
		t.Error("expected GitHub link in footer")
	}
}

func TestHeadTemplate_AlpineJSNotDeferred(t *testing.T) {
	// common.js must NOT be deferred so Alpine.data() registrations run before Alpine.js.
	// Alpine.js IS deferred. If common.js is also deferred, components won't be registered.
	handler := NewPageHandler(nil, true)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	// common.js must NOT have defer
	if strings.Contains(body, `defer src="/static/common.js"`) {
		t.Error("common.js must NOT be deferred — Alpine.data() must register before Alpine.js init")
	}
	// Verify common.js IS included
	if !strings.Contains(body, `src="/static/common.js"`) {
		t.Error("expected common.js script tag in head")
	}
	// Alpine.js must have defer
	if !strings.Contains(body, "defer src=\"https://cdn.jsdelivr.net/npm/alpinejs") {
		t.Error("expected Alpine.js to be loaded with defer")
	}
}

func TestXCloakStyle_InCSS(t *testing.T) {
	// x-cloak prevents FOUC (Flash of Unstyled Content) for Alpine.js components.
	// The CSS must include [x-cloak] { display: none !important; }
	cssPath := filepath.Join(FindPagesDir(), "static", "css", "portal.css")
	cssBytes, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("failed to read portal.css: %v", err)
	}
	css := string(cssBytes)

	if !strings.Contains(css, "[x-cloak]") {
		t.Error("CSS missing [x-cloak] rule — Alpine.js components will FOUC")
	}
	if !strings.Contains(css, "display: none !important") && !strings.Contains(css, "display:none !important") {
		t.Error("CSS [x-cloak] must use display: none !important")
	}
}

func TestDashboardHandler_PageTitleSet(t *testing.T) {
	// Verify PageTitle is set for the nav template.
	catalogFn := func() []DashboardTool { return nil }
	handler := NewDashboardHandler(nil, true, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The nav template renders {{.PageTitle}} in <span class="nav-title">
	if !strings.Contains(body, "DASHBOARD") {
		t.Error("expected DASHBOARD to appear in page (from PageTitle or h1)")
	}
}

func TestSettingsHandler_PageTitleSet(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "SETTINGS") {
		t.Error("expected SETTINGS to appear in page (from PageTitle or h1)")
	}
}

func TestSettingsHandler_FormUsesComponentLibraryClasses(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()

	// Settings form should use component library classes
	if !strings.Contains(body, "form-group") {
		t.Error("expected form-group class from component library in settings form")
	}
	if !strings.Contains(body, "form-label") {
		t.Error("expected form-label class from component library in settings form")
	}
	if !strings.Contains(body, "form-input") {
		t.Error("expected form-input class from component library in settings form")
	}
}

func TestDashboardHandler_ToolTableUseComponentLibraryClasses(t *testing.T) {
	tools := []DashboardTool{
		{Name: "test_tool", Description: "A test tool", Method: "GET", Path: "/api/test"},
	}
	catalogFn := func() []DashboardTool { return tools }

	handler := NewDashboardHandler(nil, false, 4241, catalogFn, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Dashboard should use table classes from component library
	if !strings.Contains(body, "table-wrap") {
		t.Error("expected table-wrap class from component library in dashboard")
	}
	if !strings.Contains(body, "tool-table") {
		t.Error("expected tool-table class in dashboard tools section")
	}
}

func TestSettingsHandler_CSRFTokenInHiddenField(t *testing.T) {
	// The settings form must include a hidden CSRF token field.
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "test-csrf-value"})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()

	// Should contain hidden CSRF field
	if !strings.Contains(body, `name="_csrf"`) {
		t.Error("expected hidden _csrf field in settings form")
	}
	if !strings.Contains(body, `value="test-csrf-value"`) {
		t.Error("expected CSRF token value to be injected from cookie")
	}
}

func TestSettingsHandler_CSRFTokenXSSEscaped(t *testing.T) {
	// If a hostile CSRF cookie value is set, it must be HTML-escaped in the hidden field.
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: `"><script>alert(1)</script>`})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()

	// html/template should auto-escape the CSRF value
	if strings.Contains(body, `<script>alert(1)</script>`) {
		t.Error("CSRF token value contains unescaped script tag — XSS vulnerability")
	}
}

// --- Port Configuration Stress Tests ---

func TestDashboardHandler_PortInMCPEndpoint(t *testing.T) {
	// Verify port is correctly embedded in the MCP endpoint URL
	catalogFn := func() []DashboardTool { return nil }

	// Test with default port 8080
	handler := NewDashboardHandler(nil, false, 8080, catalogFn, nil)
	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "localhost:8080/mcp") {
		t.Error("expected MCP endpoint to use port 8080")
	}
}

func TestDashboardHandler_PortZero(t *testing.T) {
	// Edge case: port 0 should still render without crashing
	catalogFn := func() []DashboardTool { return nil }

	handler := NewDashboardHandler(nil, false, 0, catalogFn, nil)
	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for port 0, got %d", w.Code)
	}
}

func TestDashboardHandler_PortNegative(t *testing.T) {
	// Edge case: negative port should render without crashing
	catalogFn := func() []DashboardTool { return nil }

	handler := NewDashboardHandler(nil, false, -1, catalogFn, nil)
	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for negative port, got %d", w.Code)
	}
}

func TestDashboardHandler_WarningBannerCSS(t *testing.T) {
	// Verify warning banner uses correct component library class.
	catalogFn := func() []DashboardTool { return nil }
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user", NavexaKey: ""}, nil
	}

	handler := NewDashboardHandler(nil, true, 4241, catalogFn, lookupFn)

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "warning-banner") {
		t.Error("expected warning-banner class from component library")
	}
}

func TestSettingsHandler_SuccessBannerCSS(t *testing.T) {
	lookupFn := func(userID string) (*models.User, error) {
		return &models.User{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, lookupFn, func(u *models.User) error { return nil })

	token := buildTestJWT("dev_user")
	req := httptest.NewRequest("GET", "/settings?saved=1", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "success-banner") {
		t.Error("expected success-banner class from component library")
	}
}

// --- Test Helpers ---

func buildTestJWT(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": sub,
		"iss": "vire-dev",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	return header + "." + payload + "."
}
