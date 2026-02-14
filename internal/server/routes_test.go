package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/app"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

func newTestApp(t *testing.T) *app.App {
	t.Helper()

	cfg := config.NewDefaultConfig()
	cfg.Storage.Badger.Path = t.TempDir()

	logger := common.NewSilentLogger()

	application, err := app.New(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create test app: %v", err)
	}

	t.Cleanup(func() {
		application.Close()
	})

	return application
}

func TestRoutes_HealthEndpoint(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

func TestRoutes_VersionEndpoint(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := body["version"]; !ok {
		t.Error("expected version field in response")
	}
}

func TestRoutes_APINotFound(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/api/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRoutes_LandingPage(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty HTML body")
	}

	// Verify key landing page content
	if !containsString(body, "VIRE") {
		t.Error("expected landing page to contain VIRE")
	}
	if !containsString(body, "IBM+Plex+Mono") {
		t.Error("expected landing page to reference IBM Plex Mono font")
	}
	if !containsString(body, "portal.css") {
		t.Error("expected landing page to reference portal.css")
	}
}

func TestRoutes_MiddlewareApplied(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Verify correlation ID middleware is applied
	if w.Header().Get("X-Correlation-ID") == "" {
		t.Error("expected X-Correlation-ID header from middleware")
	}

	// Verify CORS middleware is applied
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header from middleware")
	}
}

func TestRoutes_SecurityHeadersApplied(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Verify security headers middleware is applied
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options header from security middleware")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected X-Frame-Options header from security middleware")
	}
	if w.Header().Get("X-XSS-Protection") == "" {
		t.Error("expected X-XSS-Protection header from security middleware")
	}
	if w.Header().Get("Referrer-Policy") == "" {
		t.Error("expected Referrer-Policy header from security middleware")
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Error("expected Content-Security-Policy header from security middleware")
	}
}

func TestRoutes_CSRFCookieOnLandingPage(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Landing page GET should set a CSRF cookie
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "_csrf" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected _csrf cookie to be set on landing page response")
	}
}

// --- MCP Route Tests ---

func TestRoutes_MCPEndpointAcceptsPost(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	// POST /mcp should return a valid MCP response (not 403, 404, or 501)
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Must not be blocked by CSRF (403), missing route (404), or unimplemented (501)
	if w.Code == http.StatusForbidden {
		t.Error("POST /mcp blocked by CSRF middleware — should be exempt")
	}
	if w.Code == http.StatusNotFound {
		t.Error("POST /mcp returned 404 — route not registered")
	}
	if w.Code == http.StatusNotImplemented {
		t.Error("POST /mcp returned 501 — handler not implemented")
	}
	// A working MCP endpoint returns 200 for initialize
	if w.Code != http.StatusOK {
		t.Errorf("POST /mcp expected 200, got %d", w.Code)
	}
}

func TestRoutes_MCPNotBlockedByCSRF(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	// POST /mcp without CSRF token should NOT be rejected with 403
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Error("POST /mcp blocked by CSRF middleware — /mcp should be exempt")
	}
}

func TestRoutes_MCPHasCorrelationID(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Header().Get("X-Correlation-ID") == "" {
		t.Error("expected X-Correlation-ID header on /mcp response")
	}
}

// newTestAppWithConfig creates a test app with a custom config.
func newTestAppWithConfig(t *testing.T, cfg *config.Config) *app.App {
	t.Helper()

	cfg.Storage.Badger.Path = t.TempDir()

	logger := common.NewSilentLogger()

	application, err := app.New(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create test app: %v", err)
	}

	t.Cleanup(func() {
		application.Close()
	})

	return application
}

// --- Dev Mode Route Tests ---

func TestRoutes_DevAuthEndpoint_DevMode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Environment = "dev"
	application := newTestAppWithConfig(t, cfg)
	srv := New(application)

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// In dev mode, should redirect (302) with session cookie
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302 for dev login in dev mode, got %d", w.Code)
	}

	// Should set a vire_session cookie
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			found = true
			if !c.HttpOnly {
				t.Error("expected vire_session cookie to be httpOnly")
			}
			break
		}
	}
	if !found {
		t.Error("expected vire_session cookie from dev login endpoint")
	}
}

func TestRoutes_DevAuthEndpoint_ProdMode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	// Default is "prod"
	application := newTestAppWithConfig(t, cfg)
	srv := New(application)

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// In prod mode, should return 404
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for dev login in prod mode, got %d", w.Code)
	}
}

func TestRoutes_DevAuthEndpoint_NotBlockedByCSRF(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Environment = "dev"
	application := newTestAppWithConfig(t, cfg)
	srv := New(application)

	// POST /api/auth/dev without CSRF token should NOT be rejected with 403
	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Error("POST /api/auth/dev blocked by CSRF middleware — should be exempt")
	}
}

// --- Landing Page Dev Mode Tests ---

func TestRoutes_LandingPage_DevMode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Environment = "dev"
	application := newTestAppWithConfig(t, cfg)
	srv := New(application)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !containsString(body, "DEV LOGIN") {
		t.Error("expected landing page to contain DEV LOGIN button in dev mode")
	}
	if !containsString(body, "/api/auth/dev") {
		t.Error("expected landing page to contain /api/auth/dev action in dev mode")
	}
}

func TestRoutes_LandingPage_ProdMode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	// Default is "prod"
	application := newTestAppWithConfig(t, cfg)
	srv := New(application)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if containsString(body, "DEV LOGIN") {
		t.Error("expected landing page to NOT contain DEV LOGIN button in prod mode")
	}
	if containsString(body, "/api/auth/dev") {
		t.Error("expected landing page to NOT contain /api/auth/dev URL in prod mode")
	}
}

// --- Dashboard Route Tests ---

func TestRoutes_DashboardPage(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty HTML body")
	}

	// Verify key dashboard content
	if !containsString(body, "DASHBOARD") {
		t.Error("expected dashboard page to contain DASHBOARD")
	}
	if !containsString(body, "IBM+Plex+Mono") {
		t.Error("expected dashboard page to reference IBM Plex Mono font")
	}
	if !containsString(body, "portal.css") {
		t.Error("expected dashboard page to reference portal.css")
	}
}

func TestRoutes_DashboardContainsMCPConfig(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain MCP endpoint URL
	if !containsString(body, "/mcp") {
		t.Error("expected dashboard to contain MCP endpoint reference /mcp")
	}

	// Should contain Claude Code config snippet
	if !containsString(body, "mcpServers") {
		t.Error("expected dashboard to contain mcpServers config snippet")
	}
}

func TestRoutes_DashboardContainsToolSection(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain tools section (even if empty, should show NO TOOLS message)
	if !containsString(body, "TOOLS") {
		t.Error("expected dashboard to contain TOOLS section")
	}
}

func TestRoutes_DashboardContainsConfigStatus(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.User.Portfolios = []string{"SMSF", "Personal"}
	application := newTestAppWithConfig(t, cfg)
	srv := New(application)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain config status section
	if !containsString(body, "CONFIG") {
		t.Error("expected dashboard to contain CONFIG section")
	}

	// Should show portfolios
	if !containsString(body, "SMSF") {
		t.Error("expected dashboard to show portfolio name")
	}
}

func TestRoutes_DashboardXSSEscape(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// html/template auto-escapes, so <script> should never appear unescaped
	if containsString(body, "<script>") {
		t.Error("dashboard contains unescaped <script> tag — potential XSS")
	}
}

func TestRoutes_DashboardHasMiddleware(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Verify correlation ID middleware is applied
	if w.Header().Get("X-Correlation-ID") == "" {
		t.Error("expected X-Correlation-ID header on /dashboard response")
	}

	// Verify security headers
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options header on /dashboard response")
	}
}

// --- README Port Verification Tests ---

func TestREADME_PortConventions(t *testing.T) {
	// Verify README documents the correct port conventions:
	// - Code default: 8080 (Cloud Run standard)
	// - Docker local dev: 4241 (via config override)
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Skipf("could not read README.md: %v (test must run from project root or internal/server/)", err)
	}

	content := string(readme)

	// Config table must show 8080 as the default port
	if !containsString(content, "| `8080` |") {
		t.Error("expected README config table to show 8080 as default port")
	}

	// Docker local dev sections should reference 4241
	if !containsString(content, "localhost:4241") {
		t.Error("expected README to reference localhost:4241 for local dev")
	}

	// MCP config should show 4241 for local connection
	if !containsString(content, "localhost:4241/mcp") {
		t.Error("expected README MCP config to use localhost:4241/mcp for local dev")
	}
}

func TestREADME_ContainsDashboardRoute(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Skipf("could not read README.md: %v", err)
	}

	content := string(readme)

	if !containsString(content, "/dashboard") {
		t.Error("expected README.md to document the /dashboard route")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
