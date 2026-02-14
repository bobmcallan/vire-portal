package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestServer() *Server {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
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
