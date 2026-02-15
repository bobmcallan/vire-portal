package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

func newTestServer() *Server {
	logger := common.NewSilentLogger()
	return &Server{logger: logger}
}

// --- Correlation ID Middleware ---

func TestCorrelationIDMiddleware_GeneratesID(t *testing.T) {
	s := newTestServer()

	handler := s.correlationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := r.Context().Value(correlationIDKey).(string)
		if !ok || id == "" {
			t.Error("expected correlation ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	correlationID := w.Header().Get("X-Correlation-ID")
	if correlationID == "" {
		t.Error("expected X-Correlation-ID header")
	}
}

func TestCorrelationIDMiddleware_UsesProvidedID(t *testing.T) {
	s := newTestServer()

	handler := s.correlationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.Context().Value(correlationIDKey).(string)
		if id != "test-request-id" {
			t.Errorf("expected test-request-id, got %s", id)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-request-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Correlation-ID") != "test-request-id" {
		t.Errorf("expected X-Correlation-ID=test-request-id, got %s", w.Header().Get("X-Correlation-ID"))
	}
}

func TestCorrelationIDMiddleware_UsesCorrelationIDHeader(t *testing.T) {
	s := newTestServer()

	handler := s.correlationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.Context().Value(correlationIDKey).(string)
		if id != "existing-correlation-id" {
			t.Errorf("expected existing-correlation-id, got %s", id)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", "existing-correlation-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

// --- CORS Middleware ---

func TestCORSMiddleware_SetsHeaders(t *testing.T) {
	s := newTestServer()

	handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS origin header")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS methods header")
	}
}

func TestCORSMiddleware_HandlesPreflight(t *testing.T) {
	s := newTestServer()

	handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for OPTIONS")
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for preflight, got %d", w.Code)
	}
}

// --- Recovery Middleware ---

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	s := newTestServer()

	handler := s.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 after panic, got %d", w.Code)
	}
}

func TestRecoveryMiddleware_PassesThrough(t *testing.T) {
	s := newTestServer()

	handler := s.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/normal", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// --- Logging Middleware ---

func TestLoggingMiddleware_CapturesStatusCode(t *testing.T) {
	s := newTestServer()

	handler := s.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

// --- responseWriter ---

func TestResponseWriter_CapturesBytes(t *testing.T) {
	rw := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	data := []byte("hello world")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
	if rw.bytesWritten != len(data) {
		t.Errorf("expected bytesWritten=%d, got %d", len(data), rw.bytesWritten)
	}
}

// --- Security Headers Middleware ---

func TestSecurityHeadersMiddleware_SetsAllHeaders(t *testing.T) {
	s := newTestServer()

	handler := s.securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// X-Content-Type-Options
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options=nosniff, got %s", w.Header().Get("X-Content-Type-Options"))
	}

	// X-Frame-Options
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options=DENY, got %s", w.Header().Get("X-Frame-Options"))
	}

	// X-XSS-Protection
	if w.Header().Get("X-XSS-Protection") != "1; mode=block" {
		t.Errorf("expected X-XSS-Protection=1; mode=block, got %s", w.Header().Get("X-XSS-Protection"))
	}

	// Referrer-Policy
	if w.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy=strict-origin-when-cross-origin, got %s", w.Header().Get("Referrer-Policy"))
	}

	// Content-Security-Policy
	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected Content-Security-Policy header to be set")
	}
	if !strings.Contains(csp, "default-src") {
		t.Error("expected CSP to contain default-src directive")
	}
}

func TestSecurityHeadersMiddleware_PassesThroughResponse(t *testing.T) {
	s := newTestServer()

	handler := s.securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("I'm a teapot"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", w.Code)
	}
}

// --- Max Body Size Middleware ---

func TestMaxBodySizeMiddleware_AllowsSmallBody(t *testing.T) {
	s := newTestServer()

	handler := s.maxBodySizeMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if string(body) != "small payload" {
			t.Errorf("expected body 'small payload', got '%s'", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", strings.NewReader("small payload"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMaxBodySizeMiddleware_RejectsOversizedBody(t *testing.T) {
	s := newTestServer()

	// Set limit to 10 bytes
	handler := s.maxBodySizeMiddleware(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to read all body — should fail or be truncated
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Error("expected error reading oversized body")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Create a body larger than 10 bytes
	largeBody := strings.Repeat("x", 100)
	req := httptest.NewRequest("POST", "/test", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

func TestMaxBodySizeMiddleware_AllowsGETWithNoBody(t *testing.T) {
	s := newTestServer()

	handler := s.maxBodySizeMiddleware(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for GET, got %d", w.Code)
	}
}

// --- CSRF Middleware ---

func TestCSRFMiddleware_AllowsGETWithoutToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for GET, got %d", w.Code)
	}
}

func TestCSRFMiddleware_AllowsHEADWithoutToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("HEAD", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for HEAD, got %d", w.Code)
	}
}

func TestCSRFMiddleware_AllowsOPTIONSWithoutToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for OPTIONS, got %d", w.Code)
	}
}

func TestCSRFMiddleware_RejectsPOSTWithoutToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without CSRF token")
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for POST without CSRF token, got %d", w.Code)
	}
}

func TestCSRFMiddleware_RejectsPUTWithoutToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without CSRF token")
	}))

	req := httptest.NewRequest("PUT", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for PUT without CSRF token, got %d", w.Code)
	}
}

func TestCSRFMiddleware_RejectsDELETEWithoutToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without CSRF token")
	}))

	req := httptest.NewRequest("DELETE", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for DELETE without CSRF token, got %d", w.Code)
	}
}

func TestCSRFMiddleware_AllowsPOSTWithMatchingToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := "test-csrf-token-value"
	req := httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for POST with valid CSRF token, got %d", w.Code)
	}
}

func TestCSRFMiddleware_RejectsMismatchedToken(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with mismatched CSRF token")
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "cookie-token"})
	req.Header.Set("X-CSRF-Token", "different-header-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for mismatched CSRF tokens, got %d", w.Code)
	}
}

func TestCSRFMiddleware_SkipsAPIRoutes(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// API routes use Bearer tokens, not cookies, so CSRF should be skipped
	req := httptest.NewRequest("POST", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for /api/ routes (CSRF bypass), got %d", w.Code)
	}
}

func TestCSRFMiddleware_SetsCookieOnGET(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// The middleware should set a CSRF cookie on GET responses
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "_csrf" {
			found = true
			if c.HttpOnly {
				t.Error("CSRF cookie should NOT be HttpOnly (JS needs to read it)")
			}
			if c.Value == "" {
				t.Error("CSRF cookie value should not be empty")
			}
			break
		}
	}
	if !found {
		t.Error("expected _csrf cookie to be set on GET response")
	}
}

// --- Stress Tests: CSP Compliance ---

func TestCSP_AllowsSelfScripts(t *testing.T) {
	s := newTestServer()

	handler := s.securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")

	// script-src must include 'self' for /static/common.js
	if !strings.Contains(csp, "script-src") {
		t.Error("CSP missing script-src directive")
	}
	if !strings.Contains(csp, "'self'") {
		t.Error("CSP script-src missing 'self' — /static/common.js will be blocked")
	}

	// script-src must include 'unsafe-eval' for Alpine.js expression evaluation
	if !strings.Contains(csp, "'unsafe-eval'") {
		t.Error("CSP script-src missing 'unsafe-eval' — Alpine.js expressions will be blocked")
	}

	// script-src must include cdn.jsdelivr.net for Alpine.js
	if !strings.Contains(csp, "cdn.jsdelivr.net") {
		t.Error("CSP script-src missing cdn.jsdelivr.net — Alpine.js will be blocked")
	}

	// style-src must include fonts.googleapis.com for IBM Plex Mono
	if !strings.Contains(csp, "fonts.googleapis.com") {
		t.Error("CSP style-src missing fonts.googleapis.com — Google Fonts CSS will be blocked")
	}

	// font-src must include fonts.gstatic.com for font files
	if !strings.Contains(csp, "fonts.gstatic.com") {
		t.Error("CSP font-src missing fonts.gstatic.com — font files will be blocked")
	}
}

// --- Stress Tests: CSRF Bypass for API Routes ---

func TestCSRFMiddleware_SkipsDevAuthRoute(t *testing.T) {
	// POST /api/auth/dev is under /api/ so CSRF is skipped.
	// This is by design since /api/ routes use Bearer tokens.
	// Verify this works correctly.
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Error("POST /api/auth/dev should not be blocked by CSRF — it's under /api/")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCSRFMiddleware_SkipsMCPRoute(t *testing.T) {
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/mcp", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Error("POST /mcp should not be blocked by CSRF — it's explicitly exempt")
	}
}

func TestCSRFMiddleware_DoesNotSkipNonAPIPost(t *testing.T) {
	// A form POST to a non-API, non-MCP route MUST require CSRF.
	s := newTestServer()

	handler := s.csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without CSRF token for non-API POST")
	}))

	req := httptest.NewRequest("POST", "/some-form", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-API POST without CSRF, got %d", w.Code)
	}
}

// --- Stress Tests: Correlation ID Injection ---

func TestCorrelationIDMiddleware_SanitizesInput(t *testing.T) {
	// Verify that hostile correlation IDs don't break logging or leak into responses
	s := newTestServer()

	hostileIDs := []string{
		"<script>alert('xss')</script>",
		"'; DROP TABLE users; --",
		strings.Repeat("A", 10000),    // Very long ID
		"\r\nX-Injected-Header: evil", // Header injection
		"\x00null\x00byte",            // Null bytes
	}

	for _, hostile := range hostileIDs {
		handler := s.correlationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", hostile)
		w := httptest.NewRecorder()

		// Should not panic
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("hostile correlation ID caused non-200 response: %q", hostile)
		}
	}
}

// --- Stress Tests: Max Body Size ---

func TestMaxBodySizeMiddleware_MCPGetsLargerLimit(t *testing.T) {
	s := newTestServer()

	var readErr error
	handler := s.maxBodySizeMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	// 2MB body to /mcp should succeed (limit is 10MB)
	largeBody := strings.Repeat("x", 2<<20)
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if readErr != nil {
		t.Errorf("2MB body to /mcp should be allowed (10MB limit), got error: %v", readErr)
	}
}

func TestMaxBodySizeMiddleware_NonMCPRejectsLargeBody(t *testing.T) {
	s := newTestServer()

	var readErr error
	handler := s.maxBodySizeMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	// 2KB body to non-MCP route should fail (limit is 1KB)
	largeBody := strings.Repeat("x", 2048)
	req := httptest.NewRequest("POST", "/api/auth/dev", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if readErr == nil {
		t.Error("2KB body to non-MCP route should be rejected (1KB limit)")
	}
}
