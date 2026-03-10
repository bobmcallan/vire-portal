package handlers

import (
	"encoding/json"
	"fmt"
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

func TestStockStress_StockDetailJSONEscaping(t *testing.T) {
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
		if strings.Contains(path, "/detail") {
			// Return JSON with script-breaking content via json.Marshal
			// json.Marshal escapes < as \u003c — this is the safe path
			return json.Marshal(map[string]interface{}{
				"fundamentals": map[string]interface{}{
					"sector": `</script><script>alert("xss")</script>`,
				},
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "EVIL.AX", "name": "Test"},
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
	if strings.Contains(body, `</script><script>alert("xss")</script>`) {
		t.Error("XSS payload in stock detail JSON appears unescaped")
	}
}

func TestStockStress_StockDetailXSSRawBytes(t *testing.T) {
	// SafeJS escapes </script in raw API response bytes, preventing XSS
	// even if the upstream API returns hand-crafted JSON with literal
	// </script> tags.
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	// Simulate raw bytes with literal </script> — SafeJS must escape this
	rawXSS := `{"fundamentals":{"sector":"</script><script>alert('xss')</script>"}}`

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/detail") {
			return []byte(rawXSS), nil
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "CBA.AX", "name": "Test"},
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
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// SafeJS should escape </script to <\/script
	if strings.Contains(body, `<script>alert('xss')</script>`) {
		t.Error("SECURITY: raw API bytes with </script> broke out of JSON hydration block — SafeJS not applied")
	}
}

func TestStockStress_StockDetailXSSInFilingHeadlines(t *testing.T) {
	// Filing headlines come from API — test XSS in filing data
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"filing_summaries": []map[string]interface{}{
					{
						"date":            "2026-03-01",
						"headline":        `<img src=x onerror=alert(1)>`,
						"type":            `"><script>alert(2)</script>`,
						"price_sensitive": true,
						"key_facts":       []string{`<svg onload=alert(3)>`, "Normal fact"},
					},
				},
				"news_intelligence": map[string]interface{}{
					"overall_sentiment": `"><script>alert(4)</script>`,
					"summary":           `<img src=x onerror=alert(5)>`,
					"articles": []map[string]interface{}{
						{
							"title":       `<script>alert(6)</script>`,
							"url":         `javascript:alert(7)`,
							"source":      `"><script>alert(8)</script>`,
							"credibility": `high" onmouseover="alert(9)`,
							"summary":     `<svg/onload=alert(10)>`,
						},
					},
				},
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "CBA.AX", "name": "Test"},
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
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// json.Marshal escapes < to \u003c in the hydration block.
	// Alpine x-text sets textContent (not innerHTML) so it's safe.
	// Verify that literal HTML tags don't appear unescaped outside the JSON string.
	if strings.Contains(body, `<script>alert(`) {
		t.Error("SECURITY: XSS in filing/news data — script tag not escaped")
	}
	// Note: onerror=alert may appear inside JSON string values within the
	// <script> hydration block. This is safe because:
	// 1. It's inside a JavaScript string literal (JSON value)
	// 2. Alpine renders via x-text (textContent), not innerHTML
	// We only check for < > which would break out of JSON context.
	if strings.Contains(body, `<img src=x onerror`) {
		t.Error("SECURITY: XSS in filing/news data — img tag not escaped by json.Marshal")
	}
	if strings.Contains(body, `<svg/onload`) {
		t.Error("SECURITY: XSS in filing/news data — svg tag not escaped by json.Marshal")
	}
}

func TestStockStress_StockDetailNullSections(t *testing.T) {
	// All detail sections null — page must render without crash
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
		if strings.Contains(path, "/detail") {
			return []byte(`{"candles":null,"filing_summaries":null,"news_intelligence":null,"fundamentals":null,"company_timeline":null,"signals":null}`), nil
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "CBA.AX", "name": "Test"},
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
		t.Fatalf("expected 200 with null detail sections, got %d", w.Code)
	}
}

func TestStockStress_StockDetailAPIError(t *testing.T) {
	// Detail API errors but portfolio/holding succeeds — graceful degradation
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
		if strings.Contains(path, "/detail") {
			return nil, fmt.Errorf("service unavailable")
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with detail API error, got %d", w.Code)
	}

	body := w.Body.String()
	// Holding should still be rendered
	if !strings.Contains(body, "Commonwealth Bank") {
		t.Error("expected holding data even when detail API fails")
	}
	// StockDetailJSON should be null
	if !strings.Contains(body, "stockDetail: null") {
		t.Error("expected stockDetail: null when detail API errors")
	}
	// Internal error should not leak
	if strings.Contains(body, "service unavailable") {
		t.Error("SECURITY: internal error details leaked into HTML")
	}
}

func TestStockStress_StockDetailNoPortfolio(t *testing.T) {
	// When selected portfolio is empty, detail should not be fetched
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	detailCalled := false
	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []interface{}{},
				"default":    "",
			})
		}
		if strings.Contains(path, "/detail") {
			detailCalled = true
			return []byte(`{"candles":[]}`), nil
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
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if detailCalled {
		t.Error("stock detail API should not be called when no portfolio is selected")
	}
}

func TestStockStress_StockDetailLargePayload(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	// Generate a large candles array
	candles := make([]map[string]interface{}, 5000)
	for i := range candles {
		candles[i] = map[string]interface{}{
			"date":  "2026-01-01",
			"open":  100.0 + float64(i),
			"high":  101.0 + float64(i),
			"low":   99.0 + float64(i),
			"close": 100.5 + float64(i),
		}
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"candles": candles,
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with large payload, got %d", w.Code)
	}
}

func TestStockStress_StockDetailLargeFilings(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	// Generate many filings with many key_facts each
	filings := make([]map[string]interface{}, 200)
	for i := range filings {
		facts := make([]string, 20)
		for j := range facts {
			facts[j] = fmt.Sprintf("Key fact %d for filing %d with a moderately long description to test memory usage", j, i)
		}
		filings[i] = map[string]interface{}{
			"date":            fmt.Sprintf("2026-01-%02d", (i%28)+1),
			"headline":        fmt.Sprintf("Filing headline number %d with details", i),
			"type":            "Annual Report",
			"price_sensitive": i%3 == 0,
			"key_facts":       facts,
		}
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"filing_summaries": filings,
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with large filings, got %d", w.Code)
	}
}

func TestStockStress_TickerInjectionInDetailPath(t *testing.T) {
	// Verify ticker is URL-escaped in the detail API path
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	var capturedPath string
	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/detail") {
			capturedPath = path
			return []byte(`{"candles":[]}`), nil
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "../../../etc/passwd", "name": "Evil"},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	// Use a path-traversal ticker
	req := httptest.NewRequest("GET", "/stock/..%2F..%2F..%2Fetc%2Fpasswd", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// The ticker should be URL-escaped in the API path
	if capturedPath != "" && strings.Contains(capturedPath, "../") {
		t.Error("SECURITY: path traversal in ticker not escaped in detail API path")
	}
}

// --- Template structure (new sections) ---

func TestStockStress_WalkChartNullTimeline(t *testing.T) {
	// When position_timeline is null/empty, POSITION P&amp;L section should be guarded by x-show
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"position":          map[string]interface{}{"units": 100},
				"trades":            []interface{}{},
				"position_timeline": nil,
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// The template must have x-show="hasWalkData" guard on POSITION P&amp;L
	if !strings.Contains(body, "POSITION P&amp;L") {
		t.Error("POSITION P&amp;L section not found in template")
	}
	if !strings.Contains(body, "hasWalkData") {
		t.Error("POSITION P&amp;L section missing hasWalkData x-show guard")
	}
}

func TestStockStress_TradeHistoryNullTrades(t *testing.T) {
	// When trades is null/empty, TRADE HISTORY section should be guarded by x-show
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"position": map[string]interface{}{"units": 100},
				"trades":   nil,
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// TRADE HISTORY must have x-show="hasTrades" guard (no longer a placeholder)
	if !strings.Contains(body, "TRADE HISTORY") {
		t.Error("TRADE HISTORY section not found in template")
	}
	if !strings.Contains(body, "hasTrades") {
		t.Error("TRADE HISTORY section missing hasTrades x-show guard")
	}
}

func TestStockStress_DividendDisplayBothZero(t *testing.T) {
	// When both dividend_received and dividend_forecast are 0, dividend row should be hidden
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"position": map[string]interface{}{
					"units":              100,
					"dividend_received":  0,
					"dividend_forecast":  0,
					"original_currency":  "AUD",
					"currency_gain_loss": 0,
				},
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Template must have dividendDisplay() guard
	if !strings.Contains(body, "dividendDisplay") {
		t.Error("DIVIDENDS section missing dividendDisplay() guard")
	}
}

func TestStockStress_CurrencyGainLossAUDHolding(t *testing.T) {
	// For AUD holdings, CURRENCY GAIN/LOSS should be hidden
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"position": map[string]interface{}{
					"units":             100,
					"original_currency": "AUD",
				},
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{
						"ticker":            "CBA.AX",
						"name":              "Commonwealth Bank",
						"original_currency": "AUD",
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
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Template should use isForeignCurrency() guard for currency gain/loss
	if !strings.Contains(body, "isForeignCurrency") {
		t.Error("CURRENCY GAIN/LOSS section missing isForeignCurrency() guard")
	}
}

func TestStockStress_NewsRemovedFromTemplate(t *testing.T) {
	// NEWS & SENTIMENT section must be completely removed from the template
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"news_intelligence": map[string]interface{}{
					"overall_sentiment": "Positive",
					"summary":           "Test summary",
				},
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// NEWS & SENTIMENT section must NOT appear in the rendered template
	if strings.Contains(body, "NEWS") && strings.Contains(body, "SENTIMENT") {
		t.Error("NEWS & SENTIMENT section should be removed from template but still present")
	}
	if strings.Contains(body, "sentiment-badge") {
		t.Error("sentiment-badge class should be removed from template")
	}
	if strings.Contains(body, "credibility-badge") {
		t.Error("credibility-badge class should be removed from template")
	}
}

// --- Devils-advocate: deeper adversarial stress tests ---

// stockDetailProxy is a helper that creates a proxyGetFn returning given holding + detail.
func stockDetailProxy(holding map[string]interface{}, detail map[string]interface{}) func(string, string) ([]byte, error) {
	return func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/detail") {
			if detail == nil {
				return []byte("null"), nil
			}
			return json.Marshal(detail)
		}
		if strings.Contains(path, "/api/portfolios/Main") && !strings.Contains(path, "/stock/") {
			holdings := []map[string]interface{}{}
			if holding != nil {
				holdings = append(holdings, holding)
			}
			return json.Marshal(map[string]interface{}{"holdings": holdings})
		}
		return []byte("null"), nil
	}
}

func TestStockStress_XSSInTradeData(t *testing.T) {
	// Trade type field with script-breaking payload must be escaped by SafeJS
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Test"}
	detail := map[string]interface{}{
		"trades": []map[string]interface{}{
			{
				"id":    "t1",
				"date":  "2026-01-15",
				"type":  `</script><script>alert('xss')</script>`,
				"units": 100,
				"price": 50.0,
				"fees":  10.0,
				"value": 5010.0,
			},
		},
		"position": map[string]interface{}{
			"units":                100,
			"holding_cost_avg":     50.10,
			"true_breakeven_price": 49.50,
		},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, `<script>alert('xss')</script>`) {
		t.Error("SECURITY: XSS in trade type not escaped in hydration JSON")
	}
}

func TestStockStress_XSSInPositionTimelineDate(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Test"}
	detail := map[string]interface{}{
		"position_timeline": []map[string]interface{}{
			{
				"date":         `</script><script>alert(1)</script>`,
				"market_value": 1000.0,
				"cost_basis":   900.0,
			},
		},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, `<script>alert(1)</script>`) {
		t.Error("SECURITY: XSS in position_timeline date not escaped")
	}
}

func TestStockStress_PositionAllNullFields(t *testing.T) {
	// Position with every field null — Alpine optional chaining must handle this
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Commonwealth Bank"}
	detail := map[string]interface{}{
		"position": map[string]interface{}{
			"units":                  nil,
			"holding_cost_avg":       nil,
			"true_breakeven_price":   nil,
			"realized_return":        nil,
			"unrealized_return":      nil,
			"holding_return_net_pct": nil,
			"dividend_received":      nil,
			"dividend_forecast":      nil,
			"currency_gain_loss":     nil,
			"original_currency":      nil,
			"holding_value_market":   nil,
		},
		"trades":            nil,
		"position_timeline": nil,
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with all-null position, got %d", w.Code)
	}
}

func TestStockStress_PositionKeyMissing(t *testing.T) {
	// detail has no position/trades/position_timeline keys at all
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Commonwealth Bank"}
	detail := map[string]interface{}{
		"candles": []interface{}{},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with missing position key, got %d", w.Code)
	}
}

func TestStockStress_EmptyTimeline(t *testing.T) {
	// Empty array (not null) — hasWalkData should be false
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Commonwealth Bank"}
	detail := map[string]interface{}{
		"position_timeline": []interface{}{},
		"trades":            []interface{}{},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with empty timeline, got %d", w.Code)
	}
}

func TestStockStress_TradeHistoryPlaceholderGone(t *testing.T) {
	// The old placeholder text must be removed after implementation
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(nil, nil))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "Trade history will be available in a future update") {
		t.Error("old TRADE HISTORY placeholder text should be replaced with real trade table")
	}
}

func TestStockStress_WalkChartCanvasPresent(t *testing.T) {
	// POSITION P&amp;L section must include a walkChart canvas
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Commonwealth Bank"}
	detail := map[string]interface{}{
		"position_timeline": []map[string]interface{}{
			{"date": "2026-01-01", "market_value": 1000, "cost_basis": 900},
		},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `id="walkChart"`) {
		t.Error("expected walkChart canvas element in template")
	}
}

func TestStockStress_CurrencyGainLossUSDHolding(t *testing.T) {
	// USD holding must have currency_gain_loss in hydration JSON
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{
		"ticker":               "AAPL",
		"name":                 "Apple Inc",
		"holding_value_market": 15000.0,
		"original_currency":    "USD",
		"currency_gain_loss":   -250.50,
	}
	detail := map[string]interface{}{
		"position": map[string]interface{}{
			"original_currency":  "USD",
			"currency_gain_loss": -250.50,
			"realized_return":    500.0,
			"unrealized_return":  700.0,
		},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/AAPL", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "-250.5") {
		t.Error("expected currency_gain_loss value in hydration JSON for USD holding")
	}
}

func TestStockStress_GainBreakdownFieldsPresent(t *testing.T) {
	// Verify realized/unrealized return fields survive in hydration JSON
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{
		"ticker":             "CBA.AX",
		"name":               "Commonwealth Bank",
		"holding_return_net": 1500.0,
	}
	detail := map[string]interface{}{
		"position": map[string]interface{}{
			"realized_return":   500.0,
			"unrealized_return": 1000.0,
		},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "realized_return") {
		t.Error("expected realized_return in hydration JSON for gain breakdown")
	}
	if !strings.Contains(body, "unrealized_return") {
		t.Error("expected unrealized_return in hydration JSON for gain breakdown")
	}
}

func TestStockStress_LargeTradeHistoryAndTimeline(t *testing.T) {
	// 500 trades + 500 timeline points — no crash or timeout
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	trades := make([]map[string]interface{}, 500)
	for i := range trades {
		tradeType := "Buy"
		if i%3 == 0 {
			tradeType = "Sell"
		}
		trades[i] = map[string]interface{}{
			"id":    fmt.Sprintf("t%d", i),
			"date":  fmt.Sprintf("2025-%02d-%02d", (i%12)+1, (i%28)+1),
			"type":  tradeType,
			"units": 10 + i,
			"price": 50.0 + float64(i)*0.5,
			"fees":  9.95,
			"value": float64(10+i) * (50.0 + float64(i)*0.5),
		}
	}
	timeline := make([]map[string]interface{}, 500)
	for i := range timeline {
		timeline[i] = map[string]interface{}{
			"date":         fmt.Sprintf("2025-%02d-%02d", (i%12)+1, (i%28)+1),
			"market_value": 10000.0 + float64(i)*100,
			"cost_basis":   9000.0 + float64(i)*80,
		}
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Commonwealth Bank"}
	detail := map[string]interface{}{
		"trades":            trades,
		"position_timeline": timeline,
		"position":          map[string]interface{}{"units": 5000},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with large trade/timeline, got %d", w.Code)
	}
}

func TestStockStress_NoHoldingButDetailExists(t *testing.T) {
	// Stock not in portfolio but detail has data (e.g. watchlisted)
	// holding is null, position/trades/timeline should still render
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	detail := map[string]interface{}{
		"candles":           []map[string]interface{}{{"date": "2026-01-01", "close": 100.0}},
		"position":          nil,
		"trades":            nil,
		"position_timeline": nil,
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(nil, detail))

	req := httptest.NewRequest("GET", "/stock/UNKNOWN", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "holding: null") {
		t.Error("expected null holding for stock not in portfolio")
	}
	// Should not contain error or panic traces
	if strings.Contains(body, "runtime error") {
		t.Error("runtime error in response body")
	}
}

func TestStockStress_FullTradeDataRenders(t *testing.T) {
	// Full trade data with mixed Buy/Sell should render cleanly
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	holding := map[string]interface{}{"ticker": "CBA.AX", "name": "Commonwealth Bank"}
	detail := map[string]interface{}{
		"position": map[string]interface{}{
			"units":                200,
			"holding_cost_avg":     95.50,
			"true_breakeven_price": 93.20,
			"realized_return":      500.0,
			"unrealized_return":    1200.0,
		},
		"trades": []map[string]interface{}{
			{"id": "t1", "date": "2025-06-15", "type": "Buy", "units": 100, "price": 90.0, "fees": 9.95, "value": 9009.95},
			{"id": "t2", "date": "2025-09-20", "type": "Buy", "units": 50, "price": 95.0, "fees": 9.95, "value": 4759.95},
			{"id": "t3", "date": "2026-01-10", "type": "Sell", "units": 20, "price": 105.0, "fees": 9.95, "value": 2090.05},
		},
		"position_timeline": []map[string]interface{}{
			{"date": "2025-06-15", "market_value": 9000, "cost_basis": 9010, "net_return": -10},
			{"date": "2025-09-20", "market_value": 14250, "cost_basis": 13770, "net_return": 480},
			{"date": "2026-01-10", "market_value": 13650, "cost_basis": 11670, "net_return": 1980},
		},
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(stockDetailProxy(holding, detail))

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Trade IDs and timeline values must be in hydration JSON
	if !strings.Contains(body, `"t1"`) || !strings.Contains(body, `"t3"`) {
		t.Error("expected trade IDs in hydration JSON")
	}
	if !strings.Contains(body, "9000") || !strings.Contains(body, "1980") {
		t.Error("expected position_timeline data in hydration JSON")
	}
}

func TestStockStress_XSSRawBytesInTradeField(t *testing.T) {
	// Simulate a raw API response (not json.Marshal) with literal </script> in trade data
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", Role: "user"}, nil
	}

	rawXSS := `{"trades":[{"id":"t1","date":"2026-01-01","type":"</script><script>alert('xss')</script>","units":1,"price":1,"fees":0,"value":1}]}`

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/detail") {
			return []byte(rawXSS), nil
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "CBA.AX", "name": "Test"},
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
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// SafeJS must escape </script to <\/script
	if strings.Contains(body, `<script>alert('xss')</script>`) {
		t.Error("SECURITY: raw API bytes with </script> in trade data broke out of JSON hydration")
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

// --- Stock page restructure template validation ---

func stockStressRenderPage(t *testing.T) string {
	t.Helper()
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"position": map[string]interface{}{
					"holding_cost_avg": 1.38,
					"realized_return":  -100.0,
				},
				"trades": []map[string]interface{}{
					{"id": "1", "type": "Buy", "date": "2026-01-01", "units": 1000, "price": 1.38, "fees": 10, "value": 1390},
					{"id": "2", "type": "Sell", "date": "2026-02-01", "units": 500, "price": 1.50, "fees": 10, "value": 740},
				},
				"position_timeline": []map[string]interface{}{
					{"date": "2026-01-01", "net_return": -50},
					{"date": "2026-02-01", "net_return": 100},
				},
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	return w.Body.String()
}

func TestStockPage_StressNoPriceChartSection(t *testing.T) {
	body := stockStressRenderPage(t)
	if strings.Contains(body, "STOCK PRICE TREND") {
		t.Error("STOCK PRICE TREND section should be removed from template but still present")
	}
	if strings.Contains(body, "stock-price-chart") {
		t.Error("stock-price-chart element should be removed from template but still present")
	}
}

func TestStockPage_StressNoSMACheckboxes(t *testing.T) {
	body := stockStressRenderPage(t)
	for _, sma := range []string{"showSMA20", "showSMA50", "showSMA200"} {
		if strings.Contains(body, sma) {
			t.Errorf("SMA reference %q should be removed from template but still present", sma)
		}
	}
}

func TestStockPage_StressPandLChartPresent(t *testing.T) {
	body := stockStressRenderPage(t)
	if !strings.Contains(body, "POSITION P&amp;L") {
		t.Error("POSITION P&L header not found in template")
	}
}

func TestStockPage_StressTradeRealisedPLColumn(t *testing.T) {
	body := stockStressRenderPage(t)
	if !strings.Contains(body, "Realised P&amp;L") {
		t.Error("Realised P&L column header not found in trade history table")
	}
}

func TestStockPage_StressTradeRealisedPLUsesXText(t *testing.T) {
	body := stockStressRenderPage(t)
	if !strings.Contains(body, "tradeRealisedPL") {
		t.Error("tradeRealisedPL helper not referenced in template")
	}
	// Must use x-text, not x-html (XSS safety)
	if strings.Contains(body, "x-html=\"tradeRealisedPL") {
		t.Error("tradeRealisedPL must use x-text, not x-html")
	}
}

func TestStockPage_StressWalkChartCanvasExists(t *testing.T) {
	body := stockStressRenderPage(t)
	if !strings.Contains(body, `id="walkChart"`) {
		t.Error("walkChart canvas not found in template")
	}
}

// --- Devils-advocate adversarial tests ---

func TestStockPage_StressNullPositionRealisedPL(t *testing.T) {
	// When position is null (stock not held), tradeRealisedPL must not crash.
	// The template guards trade table with x-show="hasTrades" and the JS helper
	// returns null when position is nil, so the column should show '-'.
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
		if strings.Contains(path, "/detail") {
			// position is null, but trades and timeline exist
			return json.Marshal(map[string]interface{}{
				"position": nil,
				"trades": []map[string]interface{}{
					{"id": "1", "type": "Sell", "date": "2026-02-01", "units": 500, "price": 1.50, "fees": 10, "value": 740},
				},
				"position_timeline": []map[string]interface{}{},
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with null position, got %d", w.Code)
	}
	// Page must render without error — the tradeRealisedPL guard handles null position
}

func TestStockPage_StressNoOrphanedPriceChartJS(t *testing.T) {
	// Verify the rendered page does not contain orphaned JS function references
	// from the removed price chart feature
	body := stockStressRenderPage(t)
	orphans := []string{"renderPriceChart", "destroyPriceChart", "showSMA20", "showSMA50", "showSMA200"}
	for _, ref := range orphans {
		if strings.Contains(body, ref) {
			t.Errorf("orphaned JS reference %q still present in rendered page", ref)
		}
	}
}

func TestStockPage_StressNoOrphanedPriceChartCSS(t *testing.T) {
	// Verify removed CSS classes are no longer in the template
	body := stockStressRenderPage(t)
	removedClasses := []string{"stock-price-chart-section", "stock-trend-badge", "stock-rsi-value"}
	for _, cls := range removedClasses {
		if strings.Contains(body, cls) {
			t.Errorf("orphaned CSS class %q still present in rendered page", cls)
		}
	}
}

func TestStockPage_StressAllZeroTimeline(t *testing.T) {
	// When all net_return values are zero, the P&L chart split (aboveZero/belowZero)
	// produces all-zero arrays. The template must still render without error.
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
		if strings.Contains(path, "/detail") {
			return json.Marshal(map[string]interface{}{
				"position": map[string]interface{}{
					"holding_cost_avg": 1.00,
					"realized_return":  0.0,
				},
				"trades": []map[string]interface{}{
					{"id": "1", "type": "Buy", "date": "2026-01-01", "units": 100, "price": 1.00, "fees": 0, "value": 100},
				},
				"position_timeline": []map[string]interface{}{
					{"date": "2026-01-01", "net_return": 0},
					{"date": "2026-01-02", "net_return": 0},
					{"date": "2026-01-03", "net_return": 0},
				},
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

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with all-zero timeline, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="walkChart"`) {
		t.Error("walkChart canvas should still be present with all-zero timeline")
	}
	if !strings.Contains(body, "hasWalkData") {
		t.Error("hasWalkData guard missing — all-zero timeline should still show chart section")
	}
}

func TestStockPage_StressRealisedPLNotXHTML(t *testing.T) {
	// Adversarial: ensure no x-html binding exists anywhere in stock.html template output
	// This prevents future regressions where someone might switch from x-text to x-html
	body := stockStressRenderPage(t)
	if strings.Contains(body, "x-html=") {
		t.Error("stock page template contains x-html binding — all dynamic content must use x-text for XSS safety")
	}
}
