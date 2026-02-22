package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestAPIProxy_StressSuite runs all API proxy stress tests with a single
// shared test app to avoid repeated 30-second MCP catalog timeouts.
func TestAPIProxy_StressSuite(t *testing.T) {
	application := newTestApp(t)

	t.Run("PathTraversal_Simple", func(t *testing.T) {
		var receivedPath string
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		req := httptest.NewRequest("GET", "/api/portfolios/test", nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if receivedPath != "/api/portfolios/test" {
			t.Errorf("expected /api/portfolios/test, got %s", receivedPath)
		}
	})

	t.Run("PathTraversal_EncodedSlashes", func(t *testing.T) {
		var receivedPath string
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		// %2F-encoded slashes bypass Go router normalization.
		// The proxy decodes them and the HTTP client resolves ".."
		req := httptest.NewRequest("GET", "/api/portfolios/test%2F..%2F..%2Fetc%2Fpasswd", nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		// Document the behavior: the backend sees a path with ".." resolved
		t.Logf("NOTE: encoded path traversal resulted in backend path: %s (stays within /api/ namespace)", receivedPath)
	})

	t.Run("PathTraversal_DoubleDotNormalized", func(t *testing.T) {
		// /api/portfolios/test/../../health -> Go normalizes before routing
		srv := New(application)

		req := httptest.NewRequest("GET", "/api/portfolios/test/../../health", nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		// Should be redirected or handled by Go's path cleaning
		t.Logf("double-dot path returned status %d (normalized by Go router)", w.Code)
	})

	t.Run("UnauthenticatedAccess", func(t *testing.T) {
		// FINDING: The /api/ proxy does NOT check authentication.
		// Security depends on vire-server to validate requests.
		var gotRequest bool
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRequest = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"portfolios":[]}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		gotRequest = false
		req := httptest.NewRequest("GET", "/api/portfolios", nil)
		// No session cookie
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if !gotRequest {
			t.Log("NOTE: Unauthenticated request was NOT forwarded to backend")
		} else {
			t.Log("FINDING: /api/portfolios proxied without authentication. Security relies on vire-server.")
		}
	})

	t.Run("PUTWithoutCSRF", func(t *testing.T) {
		// FINDING: PUT requests to /api/ bypass CSRF middleware.
		var receivedMethod string
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		req := httptest.NewRequest("PUT", "/api/portfolios/test/strategy", strings.NewReader(`{"strategy":"hostile"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Log("CSRF protection applied to PUT /api/")
		} else if receivedMethod == "PUT" {
			t.Log("FINDING: PUT accepted without CSRF token. Mitigated by SameSite=Lax cookies.")
		}
	})

	t.Run("HeaderForwarding", func(t *testing.T) {
		var receivedHeaders http.Header
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		req := httptest.NewRequest("GET", "/api/portfolios", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.AddCookie(&http.Cookie{Name: "vire_session", Value: "session-value"})
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if receivedHeaders.Get("Authorization") == "Bearer test-token" {
			t.Log("NOTE: Authorization header forwarded through proxy.")
		}
		if receivedHeaders.Get("Cookie") != "" {
			t.Log("NOTE: Cookie header forwarded through proxy to backend.")
		}
	})

	t.Run("ResponseHeaderInjection", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Set-Cookie", "vire_session=evil-token; Path=/; HttpOnly")
			w.Header().Set("X-Injected", "evil-value")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		req := httptest.NewRequest("GET", "/api/portfolios", nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		for _, cookie := range w.Result().Cookies() {
			if cookie.Name == "vire_session" && cookie.Value == "evil-token" {
				t.Log("FINDING: backend Set-Cookie forwarded through proxy — could overwrite session cookie if backend compromised.")
			}
		}

		if w.Header().Get("X-Injected") == "evil-value" {
			t.Log("FINDING: arbitrary response headers from backend forwarded. If backend compromised, it can set cookies on portal domain.")
		}
	})

	t.Run("LargeResponse", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			chunk := strings.Repeat("x", 1024)
			for i := 0; i < 1024; i++ {
				w.Write([]byte(chunk))
			}
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		req := httptest.NewRequest("GET", "/api/portfolios", nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Body.Len() < 1024*1024 {
			t.Errorf("expected ~1MB response, got %d bytes", w.Body.Len())
		}
	})

	t.Run("BackendInvalidJSON", func(t *testing.T) {
		invalidResponses := []struct {
			name string
			body string
		}{
			{"not json", "this is not json"},
			{"html instead", "<html><body>Error</body></html>"},
			{"truncated json", `{"portfolios":[`},
			{"null", "null"},
			{"empty", ""},
		}

		for _, tc := range invalidResponses {
			t.Run(tc.name, func(t *testing.T) {
				backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tc.body))
				}))
				defer backend.Close()

				application.Config.API.URL = backend.URL
				srv := New(application)

				req := httptest.NewRequest("GET", "/api/portfolios", nil)
				w := httptest.NewRecorder()
				srv.Handler().ServeHTTP(w, req)

				if w.Body.String() != tc.body {
					t.Errorf("proxy modified response: expected %q, got %q", tc.body, w.Body.String())
				}
			})
		}
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		var requestCount int64
		var mu sync.Mutex

		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/api/portfolios", nil)
				w := httptest.NewRecorder()
				srv.Handler().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("concurrent request %d got status %d", n, w.Code)
				}
			}(i)
		}
		wg.Wait()

		mu.Lock()
		count := requestCount
		mu.Unlock()

		if count != 100 {
			t.Errorf("expected 100 proxied requests, got %d", count)
		}
	})

	t.Run("HostileQueryParams", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		hostileQueries := []string{
			"/api/portfolios?name=<script>alert(1)</script>",
			"/api/portfolios?redirect=https://evil.com",
			"/api/portfolios?" + strings.Repeat("a=b&", 10000),
			"/api/portfolios?name=test%00null",
		}

		for _, path := range hostileQueries {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Logf("hostile query returned %d (may be blocked)", w.Code)
			}
		}
	})

	t.Run("MethodPassThrough", func(t *testing.T) {
		var receivedMethods []string
		var mu sync.Mutex

		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			receivedMethods = append(receivedMethods, r.Method)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"method": r.Method})
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
		for _, method := range methods {
			req := httptest.NewRequest(method, "/api/portfolios/test", nil)
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)
			t.Logf("Method %s proxied with status %d", method, w.Code)
		}

		mu.Lock()
		defer mu.Unlock()
		if len(receivedMethods) != len(methods) {
			t.Errorf("expected %d methods proxied, got %d", len(methods), len(receivedMethods))
		}
	})
}

// --- Route tests that don't need a backend ---

func TestRoutes_MCPInfoPage_Unauthenticated(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("unauthenticated /mcp-info expected 302, got %d", w.Code)
	}
}

func TestRoutes_MCPInfoPage_Authenticated(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	secret := application.Config.Auth.JWTSecret
	token := createTestJWT("test-user", secret)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("authenticated /mcp-info expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "MCP CONNECTION") {
		t.Error("expected MCP CONNECTION section in page")
	}
}

func TestRoutes_StressCORSWildcard(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest("OPTIONS", "/api/portfolios", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin == "*" {
		t.Log("FINDING: Access-Control-Allow-Origin is * for all routes. Consider restricting for /api/ routes.")
	}
}
