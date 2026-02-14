package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/app"
	"github.com/bobmcallan/vire-portal/internal/config"
)

func newTestApp(t *testing.T) *app.App {
	t.Helper()

	cfg := config.NewDefaultConfig()
	cfg.Storage.Badger.Path = t.TempDir()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

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
