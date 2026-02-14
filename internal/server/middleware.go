package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// contextKey is the type for context keys used in middleware.
type contextKey string

const correlationIDKey contextKey = "correlation_id"

// withMiddleware wraps the router with the middleware chain.
func (s *Server) withMiddleware(handler http.Handler) http.Handler {
	// Applied in reverse order (last applied = first executed)
	handler = s.recoveryMiddleware(handler)
	handler = s.maxBodySizeMiddleware(1 << 20)(handler) // 1MB limit
	handler = s.csrfMiddleware(handler)
	handler = s.corsMiddleware(handler)
	handler = s.securityHeadersMiddleware(handler)
	handler = s.loggingMiddleware(handler)
	handler = s.correlationIDMiddleware(handler)
	return handler
}

// correlationIDMiddleware extracts or generates a correlation ID for request tracking.
func (s *Server) correlationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get("X-Request-ID")
		if correlationID == "" {
			correlationID = r.Header.Get("X-Correlation-ID")
		}
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		w.Header().Set("X-Correlation-ID", correlationID)

		ctx := context.WithValue(r.Context(), correlationIDKey, correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMiddleware logs HTTP requests and responses.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		durationMs := time.Since(start).Milliseconds()
		correlationID, _ := r.Context().Value(correlationIDKey).(string)

		level := slog.LevelDebug
		if rw.statusCode >= 500 {
			level = slog.LevelError
		} else if rw.statusCode >= 400 {
			level = slog.LevelWarn
		}

		s.logger.Log(r.Context(), level, "HTTP request",
			"correlation_id", correlationID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", durationMs,
			"bytes", rw.bytesWritten,
			"remote", r.RemoteAddr,
		)
	})
}

// corsMiddleware handles CORS headers.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware recovers from panics and returns 500 error.
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				correlationID, _ := r.Context().Value(correlationIDKey).(string)

				s.logger.Error("panic recovered",
					"correlation_id", correlationID,
					"error", fmt.Sprintf("%v", err),
					"path", r.URL.Path,
				)

				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware sets standard security headers on all responses.
func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net")
		next.ServeHTTP(w, r)
	})
}

// maxBodySizeMiddleware limits the size of request bodies.
func (s *Server) maxBodySizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// csrfMiddleware provides CSRF protection for server-rendered forms.
// Safe methods (GET, HEAD, OPTIONS) are allowed without a token.
// API routes (/api/) are skipped (they use Bearer tokens).
// Unsafe methods require a matching _csrf cookie and X-CSRF-Token header.
// A _csrf cookie is set on GET responses for JavaScript to read.
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip safe methods
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
			// Set CSRF cookie on GET for JS to read
			if r.Method == "GET" {
				cookie, err := r.Cookie("_csrf")
				if err != nil || cookie.Value == "" {
					token := generateCSRFToken()
					http.SetCookie(w, &http.Cookie{
						Name:     "_csrf",
						Value:    token,
						Path:     "/",
						HttpOnly: false, // JS needs to read it
						SameSite: http.SameSiteStrictMode,
					})
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		// Skip API routes (they use Bearer tokens)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Validate CSRF token for unsafe methods
		cookie, err := r.Cookie("_csrf")
		if err != nil || cookie.Value == "" {
			http.Error(w, "Forbidden: missing CSRF token", http.StatusForbidden)
			return
		}

		headerToken := r.Header.Get("X-CSRF-Token")
		if headerToken == "" || headerToken != cookie.Value {
			http.Error(w, "Forbidden: invalid CSRF token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// generateCSRFToken creates a random token for CSRF protection.
func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}
