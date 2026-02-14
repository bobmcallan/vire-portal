package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

	// Should redirect to /
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
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

	handler := NewDashboardHandler(nil, true, 4241, catalogFn)

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

	handler := NewDashboardHandler(nil, false, 4241, catalogFn)

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

	handler := NewDashboardHandler(nil, false, 4241, catalogFn)

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

	handler := NewDashboardHandler(nil, false, 4241, catalogFn)

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

	handler := NewDashboardHandler(nil, false, 4241, catalogFn)

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

	handler := NewDashboardHandler(nil, false, 4241, catalogFn)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should show the tool count
	if !strings.Contains(body, "3") {
		t.Error("expected dashboard to display tool count of 3")
	}
}
