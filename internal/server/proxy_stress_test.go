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

	t.Run("CacheHitSkipsBackend", func(t *testing.T) {
		var backendHits int
		var mu sync.Mutex
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			backendHits++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"portfolios":[{"name":"SMSF"}]}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		secret := application.Config.Auth.JWTSecret
		token := createTestJWT("cache-test-user", secret)
		srv := New(application)

		// First request: should hit backend
		req1 := httptest.NewRequest("GET", "/api/portfolios", nil)
		req1.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w1 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Fatalf("first request expected 200, got %d", w1.Code)
		}

		// Second request: should be served from cache (no backend hit)
		req2 := httptest.NewRequest("GET", "/api/portfolios", nil)
		req2.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w2 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Fatalf("second request expected 200, got %d", w2.Code)
		}
		if w2.Body.String() != w1.Body.String() {
			t.Error("cached response body differs from original")
		}

		mu.Lock()
		hits := backendHits
		mu.Unlock()
		if hits != 1 {
			t.Errorf("expected 1 backend hit (second served from cache), got %d", hits)
		}
	})

	t.Run("WriteInvalidatesCache", func(t *testing.T) {
		var backendHits int
		var mu sync.Mutex
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			backendHits++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		secret := application.Config.Auth.JWTSecret
		token := createTestJWT("invalidate-test-user", secret)
		srv := New(application)

		// GET /api/portfolios to populate cache
		req1 := httptest.NewRequest("GET", "/api/portfolios", nil)
		req1.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w1 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w1, req1)

		// Verify cache works (second GET should NOT hit backend)
		req1b := httptest.NewRequest("GET", "/api/portfolios", nil)
		req1b.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w1b := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w1b, req1b)

		mu.Lock()
		hitsBeforePut := backendHits
		mu.Unlock()
		if hitsBeforePut != 1 {
			t.Fatalf("expected 1 backend hit before PUT (second GET from cache), got %d", hitsBeforePut)
		}

		// PUT to /api/portfolios — invalidates cached entries containing this path
		req2 := httptest.NewRequest("PUT", "/api/portfolios", strings.NewReader(`{"name":"SMSF"}`))
		req2.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w2, req2)

		// GET again: cache was invalidated by PUT, should hit backend
		req3 := httptest.NewRequest("GET", "/api/portfolios", nil)
		req3.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w3 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w3, req3)

		mu.Lock()
		hits := backendHits
		mu.Unlock()
		// 3 hits: initial GET, PUT, post-invalidation GET
		if hits != 3 {
			t.Errorf("expected 3 backend hits (GET + PUT + cache-miss GET), got %d", hits)
		}
	})
}

// --- Dashboard capital performance stress tests ---

// TestDashboardCapitalPerformance_StressSuite tests the new capital performance,
// indicators, and refresh features for edge cases, injection, and race conditions.
func TestDashboardCapitalPerformance_StressSuite(t *testing.T) {
	application := newTestApp(t)

	t.Run("CapitalPerformance_UnexpectedValues", func(t *testing.T) {
		// Attack vector: API response contains unexpected values (null, NaN,
		// extremely large numbers, negative zero). The JS client uses
		// Number(...) || 0 which handles most edge cases. Verify the proxy
		// faithfully passes them through without modification.
		edgeCaseResponses := []struct {
			name string
			body string
		}{
			{"null_fields", `{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":null,"current_portfolio_value":null,"simple_return_pct":null,"annualized_return_pct":null,"transaction_count":1}}`},
			{"nan_strings", `{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":"NaN","current_portfolio_value":"NaN","simple_return_pct":"NaN","annualized_return_pct":"NaN","transaction_count":1}}`},
			{"huge_numbers", `{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":999999999999999,"current_portfolio_value":999999999999999,"simple_return_pct":99999.99,"annualized_return_pct":99999.99,"transaction_count":1}}`},
			{"negative_zero", `{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":-0,"current_portfolio_value":-0,"simple_return_pct":-0,"annualized_return_pct":-0,"transaction_count":1}}`},
			{"negative_values", `{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":-100000,"current_portfolio_value":-50000,"simple_return_pct":-150.5,"annualized_return_pct":-999.99,"transaction_count":1}}`},
			{"zero_capital_deployed", `{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":0,"current_portfolio_value":50000,"simple_return_pct":0,"annualized_return_pct":0,"transaction_count":1}}`},
			{"missing_capital_performance", `{"holdings":[],"total_cost":0}`},
			{"empty_capital_performance", `{"holdings":[],"total_cost":0,"capital_performance":{}}`},
			{"zero_transaction_count", `{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":100000,"current_portfolio_value":110000,"simple_return_pct":10,"annualized_return_pct":12,"transaction_count":0}}`},
		}

		for _, tc := range edgeCaseResponses {
			t.Run(tc.name, func(t *testing.T) {
				backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tc.body))
				}))
				defer backend.Close()

				application.Config.API.URL = backend.URL
				secret := application.Config.Auth.JWTSecret
				token := createTestJWT("edge-case-user", secret)
				srv := New(application)

				req := httptest.NewRequest("GET", "/api/portfolios/test-portfolio", nil)
				req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
				w := httptest.NewRecorder()
				srv.Handler().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("expected 200, got %d", w.Code)
				}
				// Proxy should pass through the response body unmodified
				if w.Body.String() != tc.body {
					t.Errorf("proxy modified response body for %s", tc.name)
				}
			})
		}
	})

	t.Run("IndicatorsEndpoint_ErrorResponses", func(t *testing.T) {
		// Attack vector: indicators API returns 500, malformed JSON, or timeout.
		// The client-side JS uses .catch(() => { this.hasIndicators = false })
		// so the proxy just needs to faithfully pass errors through.
		errorResponses := []struct {
			name       string
			statusCode int
			body       string
		}{
			{"server_error_500", http.StatusInternalServerError, `{"error":"internal server error"}`},
			{"bad_gateway_502", http.StatusBadGateway, `{"error":"bad gateway"}`},
			{"malformed_json", http.StatusOK, `{not valid json`},
			{"html_error_page", http.StatusOK, `<html><body>Error</body></html>`},
			{"empty_response", http.StatusOK, ``},
			{"null_response", http.StatusOK, `null`},
		}

		for _, tc := range errorResponses {
			t.Run(tc.name, func(t *testing.T) {
				backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.statusCode)
					w.Write([]byte(tc.body))
				}))
				defer backend.Close()

				application.Config.API.URL = backend.URL
				srv := New(application)

				req := httptest.NewRequest("GET", "/api/portfolios/test-portfolio/indicators", nil)
				w := httptest.NewRecorder()
				srv.Handler().ServeHTTP(w, req)

				if w.Code != tc.statusCode {
					t.Errorf("expected %d, got %d", tc.statusCode, w.Code)
				}
			})
		}
	})

	t.Run("XSSInIndicatorValues", func(t *testing.T) {
		// Attack vector: malicious server response injects scripts via
		// trend/rsi_signal strings. Alpine.js x-text uses textContent (not
		// innerHTML) so XSS is prevented at the template level. The proxy
		// should not attempt to sanitize — verify pass-through.
		xssPayloads := []struct {
			name string
			body string
		}{
			{"script_in_trend", `{"trend":"<script>alert('xss')</script>","rsi_signal":"neutral","data_points":10}`},
			{"event_handler_in_rsi", `{"trend":"bullish","rsi_signal":"\" onmouseover=\"alert(1)","data_points":10}`},
			{"img_onerror", `{"trend":"<img src=x onerror=alert(1)>","rsi_signal":"normal","data_points":10}`},
			{"svg_onload", `{"trend":"<svg/onload=alert(1)>","rsi_signal":"normal","data_points":10}`},
			{"unicode_homograph", `{"trend":"bul\u200blish","rsi_signal":"over\u200Dbought","data_points":10}`},
		}

		for _, tc := range xssPayloads {
			t.Run(tc.name, func(t *testing.T) {
				backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tc.body))
				}))
				defer backend.Close()

				application.Config.API.URL = backend.URL
				srv := New(application)

				req := httptest.NewRequest("GET", "/api/portfolios/test-portfolio/indicators", nil)
				w := httptest.NewRecorder()
				srv.Handler().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("expected 200, got %d", w.Code)
				}
				// Proxy should NOT modify the body (XSS prevention is client-side via x-text)
				if w.Body.String() != tc.body {
					t.Errorf("proxy modified XSS payload body — should pass through unmodified")
				}
			})
		}
	})

	t.Run("ForceRefresh_RapidCalls", func(t *testing.T) {
		// Attack vector: rapidly calling refreshPortfolio() — the JS guard
		// `if (this.refreshing) return` prevents concurrent calls, but the
		// proxy should handle rapid requests without breaking.
		var requestCount int64
		var mu sync.Mutex

		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"holdings":[],"total_cost":0,"capital_performance":{"net_capital_deployed":100000,"current_portfolio_value":110000,"simple_return_pct":10,"annualized_return_pct":12,"transaction_count":5}}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		// Fire 50 concurrent force_refresh requests
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/api/portfolios/test?force_refresh=true", nil)
				w := httptest.NewRecorder()
				srv.Handler().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("concurrent refresh %d got status %d", n, w.Code)
				}
			}(i)
		}
		wg.Wait()

		mu.Lock()
		count := requestCount
		mu.Unlock()
		// All 50 requests should reach the backend (force_refresh has no
		// server-side dedup in the proxy — rate limiting is vire-server's job)
		if count != 50 {
			t.Errorf("expected 50 backend requests, got %d", count)
		}
	})

	t.Run("ForceRefresh_CacheBypass", func(t *testing.T) {
		// Verify that force_refresh=true still gets cached in the proxy
		// (the query string is part of the cache key, so ?force_refresh=true
		// and the bare URL are separate cache entries).
		var backendHits int
		var mu sync.Mutex
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			backendHits++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"holdings":[]}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		secret := application.Config.Auth.JWTSecret
		token := createTestJWT("cache-bypass-user", secret)
		srv := New(application)

		// Request 1: normal GET (populates cache)
		req1 := httptest.NewRequest("GET", "/api/portfolios/test", nil)
		req1.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w1 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w1, req1)

		// Request 2: force_refresh GET (different cache key due to query string)
		req2 := httptest.NewRequest("GET", "/api/portfolios/test?force_refresh=true", nil)
		req2.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w2 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w2, req2)

		mu.Lock()
		hits := backendHits
		mu.Unlock()
		// Both should hit backend (different cache keys)
		if hits != 2 {
			t.Errorf("expected 2 backend hits (different cache keys), got %d", hits)
		}
	})

	t.Run("IndicatorsCacheIndependent", func(t *testing.T) {
		// The indicators endpoint should have its own cache entry,
		// independent from the portfolio data endpoint.
		var paths []string
		var mu sync.Mutex
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			paths = append(paths, r.URL.Path)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "/indicators") {
				w.Write([]byte(`{"trend":"bullish","rsi_signal":"overbought","data_points":65}`))
			} else {
				w.Write([]byte(`{"holdings":[]}`))
			}
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		secret := application.Config.Auth.JWTSecret
		token := createTestJWT("indicators-cache-user", secret)
		srv := New(application)

		// Fetch portfolio data
		req1 := httptest.NewRequest("GET", "/api/portfolios/test", nil)
		req1.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w1 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w1, req1)

		// Fetch indicators
		req2 := httptest.NewRequest("GET", "/api/portfolios/test/indicators", nil)
		req2.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w2 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w2, req2)

		// Fetch portfolio data again (should come from cache)
		req3 := httptest.NewRequest("GET", "/api/portfolios/test", nil)
		req3.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w3 := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w3, req3)

		mu.Lock()
		hitCount := len(paths)
		mu.Unlock()
		// Should be 2 backend hits: portfolio + indicators (third is cached)
		if hitCount != 2 {
			t.Errorf("expected 2 backend hits, got %d (paths: %v)", hitCount, paths)
		}
	})

	t.Run("PortfolioNameEncoding", func(t *testing.T) {
		// The JS client uses encodeURIComponent(this.selected). Verify
		// the proxy handles URL-encoded portfolio names correctly.
		var receivedPaths []string
		var mu sync.Mutex
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			receivedPaths = append(receivedPaths, r.URL.Path)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"holdings":[]}`))
		}))
		defer backend.Close()

		application.Config.API.URL = backend.URL
		srv := New(application)

		encodedNames := []struct {
			name    string
			urlPath string
		}{
			{"spaces", "/api/portfolios/My%20Portfolio"},
			{"ampersand", "/api/portfolios/Test%26Portfolio"},
			{"slash", "/api/portfolios/A%2FB"},
			{"unicode", "/api/portfolios/%E4%B8%AD%E6%96%87"},
			{"special_chars", "/api/portfolios/test%21%40%23%24"},
		}

		for _, tc := range encodedNames {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.urlPath, nil)
				w := httptest.NewRecorder()
				srv.Handler().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("portfolio name encoding %s: expected 200, got %d", tc.name, w.Code)
				}
			})
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
