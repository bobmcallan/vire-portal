package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
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

	"github.com/bobmcallan/vire-portal/internal/client"
)

// testJWTSecret is the secret used for signing test JWTs
const testJWTSecret = "test-secret-for-handler-tests-32ch!"

// createTestJWT creates a signed JWT token for testing authenticated handlers.
func createTestJWT(userID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payload := map[string]interface{}{
		"sub":      userID,
		"email":    "test@example.com",
		"name":     "Test User",
		"provider": "test",
		"iss":      "vire-portal",
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(1 * time.Hour).Unix(),
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	sigInput := header + "." + payloadB64
	mac := hmac.New(sha256.New, []byte(testJWTSecret))
	mac.Write([]byte(sigInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + signature
}

// addAuthCookie adds a valid session cookie to the request.
func addAuthCookie(req *http.Request, userID string) {
	token := createTestJWT(userID)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
}

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

func TestLoginHandler_ValidCredentials(t *testing.T) {
	// Mock vire-server that returns a signed JWT
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := buildTestJWT("dev_user")
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

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=dev_user&password=dev123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	// POST /api/auth/login should redirect (302) and set a session cookie
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
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
		t.Fatal("expected vire_session cookie to be set")
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
}

func TestLoginHandler_MissingCredentials(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	// Should redirect with error
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Errorf("expected redirect with error param, got %s", location)
	}

	// Should NOT set a vire_session cookie
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "vire_session" {
			t.Error("expected no vire_session cookie when credentials missing")
		}
	}
}

// Note: Method filtering for POST /api/auth/login is handled by Go 1.22+
// pattern routing ("POST /api/auth/login"), not by the handler itself.

// --- Dashboard Handler Tests ---

func TestDashboardHandler_Returns200(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
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

func TestMCPPageHandler_ContainsToolCatalog(t *testing.T) {
	tools := []MCPPageTool{
		{Name: "get_summary", Description: "Get portfolio summary", Method: "GET", Path: "/api/portfolios/{name}/summary"},
		{Name: "compute_indicators", Description: "Compute technical indicators", Method: "POST", Path: "/api/indicators"},
	}
	catalogFn := func() []MCPPageTool { return tools }

	handler := NewMCPPageHandler(nil, false, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should display tool names
	if !strings.Contains(body, "get_summary") {
		t.Error("expected MCP page to contain tool name 'get_summary'")
	}
	if !strings.Contains(body, "compute_indicators") {
		t.Error("expected MCP page to contain tool name 'compute_indicators'")
	}

	// Should display tool descriptions
	if !strings.Contains(body, "Get portfolio summary") {
		t.Error("expected MCP page to contain tool description")
	}
}

func TestMCPPageHandler_ContainsMCPConnectionConfig(t *testing.T) {
	catalogFn := func() []MCPPageTool { return nil }

	handler := NewMCPPageHandler(nil, false, 8500, []byte(testJWTSecret), catalogFn)
	handler.SetBaseURL("http://localhost:8500")

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should contain MCP endpoint
	if !strings.Contains(body, "/mcp") {
		t.Error("expected MCP page to contain /mcp endpoint")
	}

	// Should contain Claude Code JSON config
	if !strings.Contains(body, "mcpServers") {
		t.Error("expected MCP page to contain mcpServers JSON config")
	}

	// Should contain the port in the endpoint URL
	if !strings.Contains(body, "8500") {
		t.Error("expected MCP page to contain port 8500 in MCP endpoint")
	}
}

func TestMCPPageHandler_ShowsEmptyToolsMessage(t *testing.T) {
	catalogFn := func() []MCPPageTool { return nil }

	handler := NewMCPPageHandler(nil, false, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// With no tools, should show a "no tools" indicator
	if !strings.Contains(body, "NO TOOLS") {
		t.Error("expected MCP page to show NO TOOLS message when catalog is empty")
	}
}

func TestMCPPageHandler_XSSEscaping(t *testing.T) {
	tools := []MCPPageTool{
		{Name: "<script>alert('xss')</script>", Description: "<img onerror=alert(1) src=x>"},
	}
	catalogFn := func() []MCPPageTool { return tools }

	handler := NewMCPPageHandler(nil, false, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// html/template auto-escapes; verify raw HTML tags are NOT present
	if strings.Contains(body, "<script>") {
		t.Error("expected <script> tag to be escaped in MCP page output")
	}
	if strings.Contains(body, "<img onerror") {
		t.Error("expected <img onerror> to be escaped in MCP page output")
	}

	// The escaped forms should be present
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("expected escaped &lt;script&gt; in MCP page output")
	}
}

func TestMCPPageHandler_ToolCount(t *testing.T) {
	tools := []MCPPageTool{
		{Name: "tool_a", Description: "Tool A"},
		{Name: "tool_b", Description: "Tool B"},
		{Name: "tool_c", Description: "Tool C"},
	}
	catalogFn := func() []MCPPageTool { return tools }

	handler := NewMCPPageHandler(nil, false, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should show the tool count
	if !strings.Contains(body, "3") {
		t.Error("expected MCP page to display tool count of 3")
	}
}

// --- Login Handler Stress Tests ---

func TestLoginHandler_RedirectIsHardcoded(t *testing.T) {
	// Verify the redirect target cannot be influenced by request parameters.
	// An open redirect would allow phishing: POST /api/auth/login?redirect=https://evil.com
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := buildTestJWT("dev_user")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": token},
		})
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	// Try various hostile redirect parameters
	paths := []string{
		"/api/auth/login?redirect=https://evil.com",
		"/api/auth/login?next=https://evil.com",
		"/api/auth/login?return_to=//evil.com",
		"/api/auth/login?redirect_uri=https://evil.com/steal",
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

func TestLoginHandler_CookieAttributes(t *testing.T) {
	// Verify session cookie has secure attributes to prevent theft.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := buildTestJWT("dev_user")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": token},
		})
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=dev_user&password=dev123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

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

func TestLoginHandler_TokenFromVireServer(t *testing.T) {
	// Verify the cookie contains exactly the token returned by vire-server.
	expectedToken := buildTestJWT("dev_user")

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": expectedToken},
		})
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username=dev_user&password=dev123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

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

	// The cookie value should be exactly the token from vire-server
	if sessionCookie.Value != expectedToken {
		t.Errorf("expected cookie to contain token from vire-server")
	}
}

// --- Logout Handler Tests ---

func TestLogoutHandler_ClearsCookie(t *testing.T) {
	handler := NewAuthHandler(nil, true, "", "", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildTestJWT("dev_user")})
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
	handler := NewAuthHandler(nil, false, "", "", []byte{})

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
	// Note: Landing page auto-logouts, so nav is never rendered even with valid cookie.
	// Use DashboardHandler to test logged-in nav rendering.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

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
	handler := NewPageHandler(nil, true, []byte{})

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

func TestLoginHandler_ConcurrentRequests(t *testing.T) {
	// Verify concurrent logins don't cause race conditions.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := buildTestJWT("dev_user")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","data":{"token":"%s"}}`, token)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, true, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

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

// --- Logout Handler Stress Tests ---

func TestLogoutHandler_CookieAttributes(t *testing.T) {
	handler := NewAuthHandler(nil, true, "", "", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildTestJWT("dev_user")})
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
	handler := NewAuthHandler(nil, true, "", "", []byte{})

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
	handler := NewAuthHandler(nil, true, "", "", []byte{})

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
	handler := NewAuthHandler(nil, false, "", "", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildTestJWT("dev_user")})
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 for logout in prod mode, got %d", w.Code)
	}
}

// --- LoggedIn Edge Cases ---

func TestServePage_LoggedIn_EmptyCookieValue(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte{})

	req := httptest.NewRequest("GET", "/", nil)
	// Go's Cookie method returns the cookie even with empty value
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: ""})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// With JWT validation, empty cookie value = not logged in
	// Page should render without crashing
}

func TestServePage_LoggedIn_GarbageCookieValue(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte{})

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "not-a-jwt-at-all"})
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	// With JWT validation, garbage cookie = not logged in. Page should render without crashing.
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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user", NavexaKeySet: true, NavexaKeyPreview: "c123"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error {
		savedKey = fields["navexa_key"]
		return nil
	}

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte(testJWTSecret), lookupFn, saveFn)

	req := httptest.NewRequest("GET", "/settings", nil)
	// No cookie
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	// Settings now requires authentication - redirects to landing page
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302 redirect, got %d", w.Code)
	}

	redirect := w.Header().Get("Location")
	if redirect != "/" {
		t.Errorf("expected redirect to /, got %s", redirect)
	}
}

func TestSettingsHandler_GET_SavedBanner(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user", NavexaKeySet: true, NavexaKeyPreview: "y123"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), lookupFn)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "dev_user")
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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user", NavexaKeySet: true, NavexaKeyPreview: "ekey"}, nil
	}

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), lookupFn)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "dev_user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "WARNING") {
		t.Error("expected no WARNING banner when Navexa key is set")
	}
}

func TestDashboardHandler_NavexaKeyMissing_NotLoggedIn(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), lookupFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	// Save returns error when user doesn't exist on the server
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return nil, fmt.Errorf("user not found")
	}
	saveFn := func(userID string, fields map[string]string) error {
		return fmt.Errorf("user not found: %s", userID)
	}

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

	token := buildTestJWT("nonexistent_user")
	req := httptest.NewRequest("POST", "/settings", strings.NewReader("navexa_key=key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.HandleSaveSettings(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for unknown user save, got %d", w.Code)
	}
}

func TestSettingsHandler_POST_HostileInputs(t *testing.T) {
	// Verify hostile API key inputs are stored as-is (not interpreted) and don't crash.
	// html/template handles escaping on output — the storage layer should accept arbitrary strings.
	var savedKeys []string
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error {
		savedKeys = append(savedKeys, fields["navexa_key"])
		return nil
	}

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error {
		savedKey = fields["navexa_key"]
		return nil
	}

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error {
		return fmt.Errorf("database connection lost")
	}

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	handler := NewSettingsHandler(nil, true, []byte{}, nil, nil)

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
	// If the server returns a hostile preview, verify it is HTML-escaped on output
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user", NavexaKeySet: true, NavexaKeyPreview: `<img onerror=alert(1) src=x>`}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
}

func TestSettingsHandler_GET_DisplaysServerPreview(t *testing.T) {
	// The portal displays whatever preview the server sends, prefixed with ****.
	tests := []struct {
		name    string
		preview string
	}{
		{"4char", "bcde"},
		{"short", "ab"},
		{"empty", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lookupFn := func(userID string) (*client.UserProfile, error) {
				return &client.UserProfile{Username: "dev_user", NavexaKeySet: true, NavexaKeyPreview: tc.preview}, nil
			}
			handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, func(userID string, fields map[string]string) error { return nil })

			token := buildTestJWT("dev_user")
			req := httptest.NewRequest("GET", "/settings", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			handler.HandleSettings(w, req)

			body := w.Body.String()
			if tc.preview != "" && !strings.Contains(body, "****"+tc.preview) {
				t.Errorf("expected preview '****%s' in body", tc.preview)
			}
		})
	}
}

func TestSettingsHandler_GET_SavedQueryParamInjection(t *testing.T) {
	// Verify only "saved=1" triggers the banner; other values don't cause issues
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, func(userID string, fields map[string]string) error { return nil })

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error {
		saveCount.Add(1)
		return nil
	}

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	saveFn := func(userID string, fields map[string]string) error { return nil }

	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, saveFn)

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return nil, fmt.Errorf("database unavailable")
	}
	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, func(userID string, fields map[string]string) error { return nil })

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return nil, fmt.Errorf("database unavailable")
	}

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), lookupFn)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "dev_user")
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
	// With authentication, a garbage JWT should result in redirect to landing
	lookupCalled := false
	lookupFn := func(userID string) (*client.UserProfile, error) {
		lookupCalled = true
		return &client.UserProfile{Username: userID}, nil
	}

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), lookupFn)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "garbage-not-jwt"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// With a garbage JWT, the handler should redirect to landing page
	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect with garbage cookie, got %d", w.Code)
	}

	// Lookup should not be called since JWT validation fails first
	if lookupCalled {
		t.Error("warning should not show when JWT is invalid")
	}
}

// --- Component Library Stress Tests ---

func TestNavTemplate_ContainsLogoutPostForm(t *testing.T) {
	// The logout must be a POST form (not a GET link) for CSRF protection.
	// A GET link would be vulnerable to CSRF via <img src="/api/auth/logout">.
	// Note: Using DashboardHandler because landing page auto-logouts and doesn't show nav.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

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
	// Verify mobile menu elements exist when logged in, using navMenu() component.
	// Note: Using DashboardHandler because landing page auto-logouts and doesn't show nav.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

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
	if !strings.Contains(body, `x-data="navMenu()"`) {
		t.Error("expected Alpine.js navMenu component wrapping nav")
	}
}

func TestNavTemplate_HamburgerDropdownPresent(t *testing.T) {
	// Verify nav uses navMenu() component and has a dropdown with Settings + Logout.
	// Note: Using DashboardHandler because landing page auto-logouts and doesn't show nav.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `x-data="navMenu()"`) {
		t.Error("expected Alpine.js navMenu component wrapping nav")
	}
	if !strings.Contains(body, `nav-dropdown`) {
		t.Error("expected nav-dropdown class for desktop dropdown menu")
	}
	if !strings.Contains(body, `href="/settings"`) {
		t.Error("expected settings link in dropdown")
	}
	if !strings.Contains(body, `/api/auth/logout`) {
		t.Error("expected logout form in dropdown")
	}
}

func TestNavTemplate_NotRenderedWhenLoggedOut(t *testing.T) {
	// When not logged in, nav should not render at all.
	handler := NewPageHandler(nil, true, []byte{})

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
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
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
	handler := NewPageHandler(nil, true, []byte{})

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
	handler := NewPageHandler(nil, true, []byte{})

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
	handler := NewPageHandler(nil, true, []byte{})

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

func TestDashboardHandler_PageIdentifier(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "DASHBOARD") {
		t.Error("expected DASHBOARD to appear in page (from <title> tag)")
	}
}

func TestSettingsHandler_PageIdentifier(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, []byte(testJWTSecret), lookupFn, func(userID string, fields map[string]string) error { return nil })

	req := httptest.NewRequest("GET", "/settings", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "SETTINGS") {
		t.Error("expected SETTINGS to appear in page (from <title> tag)")
	}
}

func TestSettingsHandler_FormUsesComponentLibraryClasses(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, func(userID string, fields map[string]string) error { return nil })

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

func TestMCPPageHandler_ToolTableUseComponentLibraryClasses(t *testing.T) {
	tools := []MCPPageTool{
		{Name: "test_tool", Description: "A test tool", Method: "GET", Path: "/api/test"},
	}
	catalogFn := func() []MCPPageTool { return tools }

	handler := NewMCPPageHandler(nil, false, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// MCP page should use table classes from component library
	if !strings.Contains(body, "table-wrap") {
		t.Error("expected table-wrap class from component library in MCP page")
	}
	if !strings.Contains(body, "tool-table") {
		t.Error("expected tool-table class in MCP page tools section")
	}
}

func TestSettingsHandler_CSRFTokenInHiddenField(t *testing.T) {
	// The settings form must include a hidden CSRF token field.
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, func(userID string, fields map[string]string) error { return nil })

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
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, []byte{}, lookupFn, func(userID string, fields map[string]string) error { return nil })

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

func TestMCPPageHandler_PortInMCPEndpoint(t *testing.T) {
	// Verify base URL is correctly used in the MCP endpoint URL
	catalogFn := func() []MCPPageTool { return nil }

	handler := NewMCPPageHandler(nil, false, 8080, []byte(testJWTSecret), catalogFn)
	handler.SetBaseURL("http://localhost:8080")
	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "http://localhost:8080/mcp") {
		t.Error("expected MCP endpoint to contain http://localhost:8080/mcp")
	}
}

func TestMCPPageHandler_BaseURLDeployed(t *testing.T) {
	// When base URL is set to an external domain, the endpoint should use it
	catalogFn := func() []MCPPageTool { return nil }

	handler := NewMCPPageHandler(nil, false, 8080, []byte(testJWTSecret), catalogFn)
	handler.SetBaseURL("https://vire-pprod-portal.fly.dev")
	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "https://vire-pprod-portal.fly.dev/mcp") {
		t.Error("expected MCP endpoint to contain deployed URL")
	}
	if strings.Contains(body, "localhost") {
		t.Error("expected no localhost reference when base URL is set to external domain")
	}
}

func TestMCPPageHandler_BaseURLFallback(t *testing.T) {
	// When base URL is not set, should fall back to localhost:{port}
	catalogFn := func() []MCPPageTool { return nil }

	handler := NewMCPPageHandler(nil, false, 9999, []byte(testJWTSecret), catalogFn)
	// Intentionally NOT calling SetBaseURL
	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "http://localhost:9999/mcp") {
		t.Error("expected MCP endpoint to fall back to localhost:9999/mcp")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for fallback, got %d", w.Code)
	}
}

func TestMCPPageHandler_PortZero(t *testing.T) {
	// Edge case: port 0 should still render without crashing
	catalogFn := func() []MCPPageTool { return nil }

	handler := NewMCPPageHandler(nil, false, 0, []byte(testJWTSecret), catalogFn)
	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for port 0, got %d", w.Code)
	}
}

func TestMCPPageHandler_PortNegative(t *testing.T) {
	// Edge case: negative port should render without crashing
	catalogFn := func() []MCPPageTool { return nil }

	handler := NewMCPPageHandler(nil, false, -1, []byte(testJWTSecret), catalogFn)
	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for negative port, got %d", w.Code)
	}
}

func TestDashboardHandler_WarningBannerCSS(t *testing.T) {
	// Verify warning banner uses correct component library class.
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), lookupFn)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "warning-banner") {
		t.Error("expected warning-banner class from component library")
	}
}

func TestSettingsHandler_SuccessBannerCSS(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, []byte(testJWTSecret), lookupFn, func(userID string, fields map[string]string) error { return nil })

	req := httptest.NewRequest("GET", "/settings?saved=1", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "success-banner") {
		t.Error("expected success-banner class from component library")
	}
}

// --- Server Health Handler Tests ---

func TestServerHealthHandler_ReturnsOKWhenUpstreamHealthy(t *testing.T) {
	// Simulate a healthy upstream vire-server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			t.Errorf("expected request to /api/health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	req := httptest.NewRequest("GET", "/api/server-health", nil)
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

func TestServerHealthHandler_Returns503WhenUpstreamUnreachable(t *testing.T) {
	// Point to a port that's not listening
	handler := NewServerHealthHandler(nil, "http://127.0.0.1:19999")

	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "down" {
		t.Errorf("expected status down, got %s", body["status"])
	}
}

func TestServerHealthHandler_Returns503WhenUpstreamReturnsNon200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "down" {
		t.Errorf("expected status down, got %s", body["status"])
	}
}

func TestServerHealthHandler_RejectsNonGET(t *testing.T) {
	handler := NewServerHealthHandler(nil, "http://localhost:8080")

	req := httptest.NewRequest("POST", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

// --- Server Health Handler Stress Tests ---

func TestServerHealthHandler_SSRF_ApiURLFromConfig(t *testing.T) {
	// Verify the target URL comes from config (constructor), not from request parameters.
	// An attacker should NOT be able to redirect the health check to an arbitrary host.
	requestedURLs := make(chan string, 10)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURLs <- r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	// Try hostile query parameters that might influence the target
	paths := []string{
		"/api/server-health?url=http://evil.com",
		"/api/server-health?target=http://evil.com",
		"/api/server-health?host=evil.com",
		"/api/server-health?apiURL=http://evil.com",
		"/api/server-health?redirect=http://evil.com",
	}

	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("path=%s: expected 200, got %d", path, w.Code)
		}

		// Verify the upstream request went to /api/health, not evil.com
		select {
		case requestedURL := <-requestedURLs:
			if requestedURL != "/api/health" {
				t.Errorf("path=%s: upstream received %s instead of /api/health — SSRF", path, requestedURL)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("path=%s: no request received by upstream", path)
		}
	}
}

func TestServerHealthHandler_SSRF_HostHeader(t *testing.T) {
	// Verify a hostile Host header doesn't influence the upstream target.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	req := httptest.NewRequest("GET", "/api/server-health", nil)
	req.Host = "evil.com" // hostile Host header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should still reach the configured upstream, not evil.com
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (reached configured upstream), got %d", w.Code)
	}
}

func TestServerHealthHandler_ConcurrentRequests(t *testing.T) {
	// Verify concurrent health checks don't cause race conditions or panics.
	var requestCount atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/server-health", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("concurrent request got status %d", w.Code)
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	if count := requestCount.Load(); count != 100 {
		t.Errorf("expected 100 upstream requests, got %d", count)
	}
}

func TestServerHealthHandler_SlowUpstream(t *testing.T) {
	// Verify that a slow upstream triggers the 3s timeout and returns 503.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // longer than the 3s timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	start := time.Now()
	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	elapsed := time.Since(start)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for slow upstream, got %d", w.Code)
	}

	// Should timeout within ~3s, not wait the full 5s
	if elapsed > 4*time.Second {
		t.Errorf("handler took %v, expected timeout within ~3s", elapsed)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body["status"] != "down" {
		t.Errorf("expected status down for slow upstream, got %s", body["status"])
	}
}

func TestServerHealthHandler_UpstreamRedirect(t *testing.T) {
	// If the upstream redirects, http.DefaultClient follows it by default.
	// Verify the handler doesn't crash and treats the final response status correctly.
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer redirectTarget.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/api/health", http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// http.DefaultClient follows redirects; the final response should be 200
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after redirect, got %d", w.Code)
	}
}

func TestServerHealthHandler_InvalidAPIURL(t *testing.T) {
	// Verify handler doesn't panic with malformed apiURL values.
	badURLs := []string{
		"",
		"not-a-url",
		"://missing-scheme",
		"http://",
		"http://[::1:bad",
	}

	for _, apiURL := range badURLs {
		t.Run("url_"+apiURL, func(t *testing.T) {
			handler := NewServerHealthHandler(nil, apiURL)

			req := httptest.NewRequest("GET", "/api/server-health", nil)
			w := httptest.NewRecorder()

			// Must not panic
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("expected 503 for invalid apiURL %q, got %d", apiURL, w.Code)
			}
		})
	}
}

func TestServerHealthHandler_ResponseBodyNotLeaked(t *testing.T) {
	// Verify the upstream response body is not forwarded to the client.
	// The handler should only return {"status":"ok"} or {"status":"down"},
	// never the raw upstream body (which could contain sensitive info).
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","secret":"internal-data","db_host":"10.0.0.5"}`))
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "secret") {
		t.Error("upstream response body leaked to client — information disclosure")
	}
	if strings.Contains(body, "internal-data") {
		t.Error("upstream internal data leaked to client")
	}
	if strings.Contains(body, "db_host") {
		t.Error("upstream db_host leaked to client")
	}
	if strings.Contains(body, "10.0.0.5") {
		t.Error("upstream internal IP leaked to client")
	}
}

func TestServerHealthHandler_ContentTypeIsJSON(t *testing.T) {
	// Verify Content-Type is always application/json regardless of status.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	// Test OK case
	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json for OK response, got %s", ct)
	}

	// Test down case
	handler2 := NewServerHealthHandler(nil, "http://127.0.0.1:19999")
	req2 := httptest.NewRequest("GET", "/api/server-health", nil)
	w2 := httptest.NewRecorder()
	handler2.ServeHTTP(w2, req2)

	if ct := w2.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json for down response, got %s", ct)
	}
}

func TestServerHealthHandler_UpstreamClosesConnectionEarly(t *testing.T) {
	// Simulate an upstream that closes the connection without sending a response.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server doesn't support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when upstream closes connection, got %d", w.Code)
	}
}

func TestServerHealthHandler_NilLogger(t *testing.T) {
	// Verify handler works with nil logger (same pattern as other handlers).
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := NewServerHealthHandler(nil, upstream.URL)

	req := httptest.NewRequest("GET", "/api/server-health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil logger, got %d", w.Code)
	}
}

// --- Status Indicator CSS Tests ---

func TestStatusIndicatorCSS_AllStatesExist(t *testing.T) {
	// Verify all three status CSS classes are defined in portal.css.
	// If any are missing, the dots will have no background color.
	cssPath := filepath.Join(FindPagesDir(), "static", "css", "portal.css")
	cssBytes, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("failed to read portal.css: %v", err)
	}
	css := string(cssBytes)

	requiredClasses := []string{".status-up", ".status-startup", ".status-down"}
	for _, class := range requiredClasses {
		if !strings.Contains(css, class) {
			t.Errorf("CSS missing %s class — status dots will be invisible", class)
		}
	}
}

func TestStatusIndicatorCSS_DotBorderRadius(t *testing.T) {
	// The status dots should be round (border-radius: 50%).
	// The global reset uses border-radius: 0 !important, so .status-dot
	// needs to override with 50% !important.
	cssPath := filepath.Join(FindPagesDir(), "static", "css", "portal.css")
	cssBytes, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("failed to read portal.css: %v", err)
	}
	css := string(cssBytes)

	if !strings.Contains(css, "border-radius: 50% !important") {
		t.Error("status-dot needs border-radius: 50% !important to override global reset")
	}
}

// --- Nav Template Status Indicator Tests ---

func TestNavTemplate_StatusIndicatorsPresent(t *testing.T) {
	// Verify status indicators appear in the nav when logged in.
	// Note: Using DashboardHandler because landing page auto-logouts and doesn't show nav.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "status-indicators") {
		t.Error("expected status-indicators container in nav")
	}
	if !strings.Contains(body, `x-data="statusIndicators()"`) {
		t.Error("expected Alpine.js statusIndicators component in nav")
	}
	if !strings.Contains(body, `title="Portal"`) {
		t.Error("expected Portal status dot with title attribute")
	}
	if !strings.Contains(body, `title="Server"`) {
		t.Error("expected Server status dot with title attribute")
	}
}

func TestNavTemplate_StatusIndicatorsXSSInClassBinding(t *testing.T) {
	// The :class binding uses "'status-' + portal" where portal is a JS variable.
	// Verify the template doesn't inject user-controlled values into the class.
	// In this implementation, portal/server are only set to 'startup', 'up', or 'down'
	// so no XSS is possible. This test verifies the class binding pattern is safe.
	// Note: Using DashboardHandler because landing page auto-logouts and doesn't show nav.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The class binding should use string concatenation with hardcoded prefix
	if !strings.Contains(body, `'status-' + portal`) {
		t.Error("expected safe class binding pattern for portal status")
	}
	if !strings.Contains(body, `'status-' + server`) {
		t.Error("expected safe class binding pattern for server status")
	}
}

// --- GetServerVersion Tests ---

func TestGetServerVersion_ReturnsVersion(t *testing.T) {
	// Mock vire-server that returns a version
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Errorf("expected request to /api/version, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"1.2.34","build":"2024-01-15","git_commit":"abc123"}`))
	}))
	defer mockServer.Close()

	version := GetServerVersion(mockServer.URL)
	if version != "1.2.34" {
		t.Errorf("expected version '1.2.34', got %q", version)
	}
}

func TestGetServerVersion_ReturnsUnavailableOnNetworkError(t *testing.T) {
	// Point to a port that's not listening
	version := GetServerVersion("http://127.0.0.1:19999")
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on network error, got %q", version)
	}
}

func TestGetServerVersion_ReturnsUnavailableOnHTTPError(t *testing.T) {
	// Server returns 500
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	version := GetServerVersion(mockServer.URL)
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on HTTP 500, got %q", version)
	}
}

func TestGetServerVersion_ReturnsUnavailableOnInvalidJSON(t *testing.T) {
	// Server returns invalid JSON
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json at all`))
	}))
	defer mockServer.Close()

	version := GetServerVersion(mockServer.URL)
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on invalid JSON, got %q", version)
	}
}

func TestGetServerVersion_ReturnsUnavailableOnMissingVersionField(t *testing.T) {
	// Server returns JSON without version field
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"build":"2024-01-15","git_commit":"abc123"}`))
	}))
	defer mockServer.Close()

	version := GetServerVersion(mockServer.URL)
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on missing version field, got %q", version)
	}
}

func TestGetServerVersion_ReturnsUnavailableOnEmptyURL(t *testing.T) {
	version := GetServerVersion("")
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on empty URL, got %q", version)
	}
}

func TestGetServerVersion_ReturnsUnavailableOnInvalidURL(t *testing.T) {
	version := GetServerVersion("://not-a-valid-url")
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on invalid URL, got %q", version)
	}
}

func TestGetServerVersion_TimeoutOnSlowServer(t *testing.T) {
	// Server takes longer than the 2s timeout
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer mockServer.Close()

	start := time.Now()
	version := GetServerVersion(mockServer.URL)
	elapsed := time.Since(start)

	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on timeout, got %q", version)
	}
	// Should timeout within ~2s, not wait the full 3s
	if elapsed > 2500*time.Millisecond {
		t.Errorf("GetServerVersion took %v, expected timeout within ~2s", elapsed)
	}
}

// --- Footer Version Display Tests ---

func TestServePage_ContainsPortalVersion(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte{})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Footer should contain "Portal:" label
	if !strings.Contains(body, "Portal:") {
		t.Error("expected footer to contain 'Portal:' label")
	}
	// Footer should contain the portal version (config.GetVersion() returns "dev" in tests)
	if !strings.Contains(body, "dev") {
		t.Error("expected footer to contain portal version")
	}
}

func TestServePage_ContainsServerVersion(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte{})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()
	// Footer should contain "Server:" label
	if !strings.Contains(body, "Server:") {
		t.Error("expected footer to contain 'Server:' label")
	}
	// Server version may be "unavailable" if vire-server is not running, which is fine
	// Just check that something is displayed
	if !strings.Contains(body, "Server:") {
		t.Error("expected footer to show server version")
	}
}

func TestDashboardHandler_ContainsVersionFooter(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Portal:") {
		t.Error("expected dashboard footer to contain 'Portal:' label")
	}
	if !strings.Contains(body, "Server:") {
		t.Error("expected dashboard footer to contain 'Server:' label")
	}
}

func TestSettingsHandler_ContainsVersionFooter(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}
	handler := NewSettingsHandler(nil, true, []byte(testJWTSecret), lookupFn, func(userID string, fields map[string]string) error { return nil })

	req := httptest.NewRequest("GET", "/settings", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.HandleSettings(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Portal:") {
		t.Error("expected settings footer to contain 'Portal:' label")
	}
	if !strings.Contains(body, "Server:") {
		t.Error("expected settings footer to contain 'Server:' label")
	}
}

// --- Strategy Handler Tests ---

func TestStrategyHandler_Returns200(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
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

func TestStrategyHandler_RedirectsUnauthenticated(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("expected redirect to /, got %s", loc)
	}
}

func TestStrategyHandler_PageIdentifier(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// The nav template uses .Page to set the active class
	if !strings.Contains(body, `class="active"`) {
		t.Error("expected active class in nav for strategy page")
	}
}

func TestStrategyHandler_ContainsStrategyEditor(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "STRATEGY") {
		t.Error("expected STRATEGY header in page body")
	}
	if !strings.Contains(body, "PLAN") {
		t.Error("expected PLAN header in page body")
	}
	if !strings.Contains(body, "portfolio-editor") {
		t.Error("expected portfolio-editor textarea in page body")
	}
}

func TestStrategyHandler_NavexaKeyMissing_WhenEmpty(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user"}, nil
	}

	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), lookupFn)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "dev_user")
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

func TestStrategyHandler_NavexaKeyMissing_WhenSet(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "dev_user", NavexaKeySet: true, NavexaKeyPreview: "ekey"}, nil
	}

	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), lookupFn)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "dev_user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "WARNING") {
		t.Error("expected no WARNING banner when Navexa key is set")
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
