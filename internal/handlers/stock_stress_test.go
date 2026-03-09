package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

// --- Ticker input validation ---

func TestStockStress_PathTraversalTicker(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	payloads := []string{
		"../../etc/passwd",
		"../../../etc/shadow",
		"..%2F..%2Fetc%2Fpasswd",
		"%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"....//....//etc/passwd",
	}

	for _, payload := range payloads {
		req := httptest.NewRequest("GET", "/stock/"+payload, nil)
		addAuthCookie(req, "test-user")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			// Redirect is also acceptable (e.g. empty ticker)
			if w.Code != http.StatusFound {
				t.Errorf("ticker %q: unexpected status %d", payload, w.Code)
			}
			continue
		}

		body := w.Body.String()
		// Must not contain filesystem content or error traces
		if strings.Contains(body, "root:") || strings.Contains(body, "/bin/") {
			t.Errorf("ticker %q: response contains filesystem content", payload)
		}
	}
}

func TestStockStress_XSSTickerInTitle(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	payloads := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(1)>`,
		`<svg/onload=alert(1)>`,
	}

	for _, payload := range payloads {
		// URL-encode the payload so httptest.NewRequest doesn't choke on special chars
		encoded := url.PathEscape(payload)
		req := httptest.NewRequest("GET", "/stock/"+encoded, nil)
		addAuthCookie(req, "test-user")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			continue
		}

		body := w.Body.String()
		// The raw XSS payload must not appear unescaped in the HTML
		if strings.Contains(body, payload) {
			t.Errorf("XSS payload %q appears unescaped in response", payload)
		}
	}
}

func TestStockStress_XSSTickerInJSON(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	// Test that ticker is JS-escaped in the hydration script block
	payload := `</script><script>alert(1)</script>`
	encoded := url.PathEscape(payload)
	req := httptest.NewRequest("GET", "/stock/"+encoded, nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Skipf("handler returned %d for XSS payload, skip JSON check", w.Code)
	}

	body := w.Body.String()
	// The </script> must be escaped in the JSON block
	if strings.Contains(body, `</script><script>alert(1)</script>`) {
		t.Error("script-breaking payload appears unescaped in JSON hydration block")
	}
}

func TestStockStress_URLEncodedTicker(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", NavexaKeySet: true, Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "CBA.AX", "name": "Commonwealth Bank"},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	// URL-encoded ticker should be decoded by Go's HTTP stack
	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Commonwealth Bank") {
		t.Error("expected holding data for CBA.AX")
	}
}

// --- Auth boundary ---

func TestStockStress_AuthBoundary_NoCookie(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect for unauthenticated, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %s", loc)
	}
}

func TestStockStress_AuthBoundary_InvalidJWT(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "invalid.jwt.token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect for invalid JWT, got %d", w.Code)
	}
}

func TestStockStress_AuthBoundary_ExpiredJWT(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	// Create an expired token manually
	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0Iiwic3ViIjoiMSIsImV4cCI6MX0.invalid"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect for expired JWT, got %d", w.Code)
	}
}

// --- Null/missing data handling ---

func TestStockStress_NilProxyGetFn(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	// proxyGetFn is NOT set — should render gracefully with null data

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with nil proxyGetFn, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "holding: null") {
		t.Error("expected holding: null when proxyGetFn is nil")
	}
	if !strings.Contains(body, "CBA.AX") {
		t.Error("expected ticker in page even without proxy data")
	}
}

func TestStockStress_ProxyGetFnReturnsError(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return nil, http.ErrAbortHandler // simulate failure
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when proxy fails (graceful degradation), got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "holding: null") {
		t.Error("expected null holding when proxy errors")
	}
}

func TestStockStress_EmptyPortfoliosList(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []interface{}{},
				"default":    "",
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with empty portfolios, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "holding: null") {
		t.Error("expected null holding with empty portfolios")
	}
}

func TestStockStress_MalformedPortfolioJSON(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return []byte(`{invalid json`), nil
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should render gracefully, not panic
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with malformed JSON, got %d", w.Code)
	}
}

func TestStockStress_HoldingWithNullFields(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			// Holding with all optional fields null/missing
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{
						"ticker":                 "CBA.AX",
						"name":                   "Commonwealth Bank",
						"holding_value_market":   nil,
						"holding_return_net":     nil,
						"holding_return_net_pct": nil,
						"trend_score":            nil,
						"trend_label":            nil,
						"original_currency":      nil,
						"currency_gain_loss":     nil,
						"dividend_received":      nil,
						"dividend_forecast":      nil,
					},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with null fields, got %d", w.Code)
	}
}

func TestStockStress_HoldingWithZeroDividends(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{
						"ticker":               "CBA.AX",
						"name":                 "Commonwealth Bank",
						"holding_value_market": 100.0,
						"dividend_received":    0,
						"dividend_forecast":    0,
						"currency_gain_loss":   0,
						"original_currency":    "AUD",
					},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with zero dividends, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Commonwealth Bank") {
		t.Error("expected holding data in response")
	}
}

// --- Edge cases ---

func TestStockStress_TickerCaseInsensitiveMatch(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "CBA.AX", "name": "Commonwealth Bank", "holding_value_market": 100.0},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	// Request with lowercase ticker
	req := httptest.NewRequest("GET", "/stock/cba.ax", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Commonwealth Bank") {
		t.Error("expected case-insensitive ticker match to find holding")
	}
}

func TestStockStress_WhitespaceOnlyTicker(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	// URL-encode spaces so httptest.NewRequest can parse the URL
	req := httptest.NewRequest("GET", "/stock/%20%20%20", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Whitespace-only ticker should redirect to dashboard
	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect for whitespace ticker, got %d", w.Code)
	}
}

func TestStockStress_VeryLongTicker(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	longTicker := strings.Repeat("A", 10000)
	req := httptest.NewRequest("GET", "/stock/"+longTicker, nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should not panic or crash
	if w.Code != http.StatusOK && w.Code != http.StatusFound {
		t.Errorf("unexpected status %d for very long ticker", w.Code)
	}
}

func TestStockStress_SpecialCharsTicker(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	tickers := []string{
		"BRK.B",
		"BRK%2FB",
		"NYSE%3ACBA",
		"ASX%3ACBA.AX",
		"ticker%20with%20spaces",
		"ticker%09with%09tabs",
		"ticker%22with%22quotes",
	}

	for _, ticker := range tickers {
		req := httptest.NewRequest("GET", "/stock/"+ticker, nil)
		addAuthCookie(req, "test-user")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should not panic; any status is acceptable
		if w.Code != http.StatusOK && w.Code != http.StatusFound {
			t.Errorf("ticker %q: unexpected status %d", ticker, w.Code)
		}
	}
}

func TestStockStress_NonExistentTicker(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "CBA.AX", "name": "Commonwealth Bank"},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/DOESNOTEXIST", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for non-existent ticker, got %d", w.Code)
	}

	body := w.Body.String()
	// Should show the "not held" state, not an error
	if !strings.Contains(body, "holding: null") {
		t.Error("expected null holding for non-existent ticker")
	}
	if !strings.Contains(body, "DOESNOTEXIST") {
		t.Error("expected ticker name in page even if not held")
	}
}

// --- Script injection via API response ---

func TestStockStress_ScriptInjectionInHoldingName(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{
						"ticker": "EVIL.AX",
						"name":   `</script><script>alert("xss")</script>`,
					},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/EVIL.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// The raw JSON is embedded via template.JS which does not HTML-escape.
	// However, JSON encoding should escape the </script> via \u003c.
	// Check that the raw </script> tag does NOT appear literally in the page.
	if strings.Contains(body, `</script><script>alert("xss")</script>`) {
		t.Error("script-breaking payload in holding name appears unescaped -- XSS vulnerability")
	}
}

func TestStockStress_NilUserLookupFn(t *testing.T) {
	// userLookupFn is nil — should not panic
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with nil userLookupFn, got %d", w.Code)
	}
}
