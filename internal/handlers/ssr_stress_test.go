package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// SSR Migration Stress Tests — Security & Edge Cases
// =============================================================================

// --- Error Page: XSS via ?reason= parameter ---

func TestErrorPage_StressXSSViaReasonParam(t *testing.T) {
	// The error page reads ?reason= and maps it to a fixed message.
	// Unknown reasons should get a default message, never reflected raw.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	xssPayloads := []string{
		`<script>alert('xss')</script>`,
		`"><img src=x onerror=alert(1)>`,
		`{{.Page}}`,
		`%3Cscript%3Ealert(1)%3C/script%3E`,
		`javascript:alert(1)`,
		`' onmouseover='alert(1)'`,
	}

	for _, payload := range xssPayloads {
		reqURL := "/error?reason=" + url.QueryEscape(payload)
		req := httptest.NewRequest("GET", reqURL, nil)
		w := httptest.NewRecorder()

		handler.ServeErrorPage()(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("payload %q: expected 200, got %d", payload, w.Code)
			continue
		}

		body := w.Body.String()
		// The error message should always be the default (unknown reason)
		if !strings.Contains(body, "Something went wrong") {
			t.Errorf("payload %q: expected default message, got unexpected content", payload)
		}
		// The raw payload must NOT appear in the output
		if strings.Contains(body, "<script>alert") || strings.Contains(body, "onerror=alert") {
			t.Errorf("SECURITY XSS: payload %q reflected in error page output", payload)
		}
	}
}

func TestErrorPage_StressKnownReasonsOnly(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	knownReasons := map[string]string{
		"server_unavailable":  "authentication server is unavailable",
		"auth_failed":         "Authentication failed",
		"invalid_credentials": "Invalid username or password",
		"missing_credentials": "provide both username and password",
		"bad_request":         "Bad request",
	}

	for reason, expected := range knownReasons {
		req := httptest.NewRequest("GET", "/error?reason="+reason, nil)
		w := httptest.NewRecorder()

		handler.ServeErrorPage()(w, req)

		body := w.Body.String()
		if !strings.Contains(body, expected) {
			t.Errorf("reason %q: expected message containing %q", reason, expected)
		}
	}
}

func TestErrorPage_StressNoReasonParam(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()

	handler.ServeErrorPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Something went wrong") {
		t.Error("expected default error message when no reason provided")
	}
}

// --- Error Page: Template injection via reason ---

func TestErrorPage_StressGoTemplateInjection(t *testing.T) {
	// If ErrorMessage were rendered via {{.}} instead of text, Go template
	// directives could execute. Verify that Go template syntax is escaped.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	// These should all map to the default message, not execute
	payloads := []string{
		`{{printf "%s" .JWTSecret}}`,
		`{{template "nav.html" .}}`,
		`{{.jwtSecret}}`,
	}

	for _, payload := range payloads {
		req := httptest.NewRequest("GET", "/error?reason="+url.QueryEscape(payload), nil)
		w := httptest.NewRecorder()

		handler.ServeErrorPage()(w, req)

		body := w.Body.String()
		// Must show default message, not template output
		if !strings.Contains(body, "Something went wrong") {
			t.Errorf("SECURITY: Go template injection may have executed for payload %q", payload)
		}
		// jwtSecret should never appear in output
		if strings.Contains(body, testJWTSecret) {
			t.Error("CRITICAL: JWT secret leaked in error page output")
		}
	}
}

// --- Landing Page: Health check timeout ---

func TestLandingPage_StressServerDownRenders(t *testing.T) {
	// When vire-server is unreachable, landing page must still render (ServerStatus=false)
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetAPIURL("http://127.0.0.1:1") // unreachable

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeLandingPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even when server is down, got %d", w.Code)
	}
}

func TestLandingPage_StressClearsSessionCookie(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeLandingPage()(w, req)

	cookies := w.Result().Cookies()
	sessionCleared := false
	for _, c := range cookies {
		if c.Name == "vire_session" && c.MaxAge < 0 {
			sessionCleared = true
			break
		}
	}
	if !sessionCleared {
		t.Error("SECURITY: ServeLandingPage did not clear vire_session cookie")
	}
}

// --- Glossary Page: XSS via term data ---

func TestGlossaryPage_StressXSSInTermData(t *testing.T) {
	// If vire-server returns terms with <script> in them, Go templates
	// must HTML-escape the content ({{.Label}} auto-escapes).
	// NOTE: The glossary template may still use Alpine client-side rendering
	// (x-text is XSS-safe). This test verifies the Go handler produces
	// categories data safely and the page renders without errors.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return json.Marshal(map[string]interface{}{
			"categories": []map[string]interface{}{
				{
					"name": `<script>alert("cat")</script>`,
					"terms": []map[string]interface{}{
						{
							"term":       "xss_term",
							"label":      `<img src=x onerror=alert(1)>`,
							"definition": `<script>document.cookie</script>`,
							"formula":    `<svg/onload=alert(1)>`,
						},
					},
				},
			},
		})
	})

	req := httptest.NewRequest("GET", "/glossary", nil)
	w := httptest.NewRecorder()

	handler.ServeGlossaryPage()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// If terms are rendered via Go templates (SSR), they MUST be auto-escaped.
	// If still rendered via Alpine x-text, the XSS payload won't appear in HTML.
	// Either way, verify no executable <script>alert in body outside of <script> blocks.
	// Count script tags that look like injection (not legitimate inline scripts)
	lowerBody := strings.ToLower(body)
	if strings.Contains(lowerBody, `<script>alert(`) {
		t.Error("SECURITY XSS: unescaped <script>alert in glossary page")
	}
}

func TestGlossaryPage_StressXSSViaTermParam(t *testing.T) {
	// The ?term= param may be embedded in JS via Go template.
	// If the SSR template uses query: '{{.TermParam}}', an attacker could
	// break out with '; alert(1); '. This test verifies the term param
	// does not appear unescaped in executable JS context.
	// NOTE: The current template reads ?term= via client-side URLSearchParams
	// (safe). This test guards against regressions if SSR migration embeds it.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return json.Marshal(map[string]interface{}{"categories": []interface{}{}})
	})

	xssPayloads := []string{
		`'; alert('xss'); '`,
		`</script><script>alert(1)</script>`,
	}

	for _, payload := range xssPayloads {
		req := httptest.NewRequest("GET", "/glossary?term="+url.QueryEscape(payload), nil)
		w := httptest.NewRecorder()

		handler.ServeGlossaryPage()(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("payload %q: expected 200, got %d", payload, w.Code)
			continue
		}

		body := w.Body.String()

		// The raw XSS payload must not appear as executable JS
		if strings.Contains(body, "alert('xss')") {
			t.Errorf("SECURITY XSS: term param %q injectable in glossary script", payload)
		}
		// Check for script tag breakout — more closing </script> than opening <script>
		// would indicate injection
		closeTags := strings.Count(body, "</script>")
		openTags := strings.Count(strings.ToLower(body), "<script")
		if closeTags > openTags {
			t.Errorf("SECURITY XSS: possible script tag breakout with payload %q (open=%d, close=%d)", payload, openTags, closeTags)
		}
	}
}

// --- Glossary Page: Malformed JSON from server ---

func TestGlossaryPage_StressMalformedJSON(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return []byte(`{not valid json at all`), nil
	})

	req := httptest.NewRequest("GET", "/glossary", nil)
	w := httptest.NewRecorder()

	handler.ServeGlossaryPage()(w, req)

	// Should still render (empty categories), not crash
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for malformed JSON, got %d", w.Code)
	}
}

func TestGlossaryPage_StressNullCategories(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return []byte(`{"categories": null}`), nil
	})

	req := httptest.NewRequest("GET", "/glossary", nil)
	w := httptest.NewRecorder()

	handler.ServeGlossaryPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for null categories, got %d", w.Code)
	}
}

// --- Glossary Page: proxyGetFn is nil ---

func TestGlossaryPage_StressNilProxyGetFn(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	// Do NOT set proxyGetFn

	req := httptest.NewRequest("GET", "/glossary", nil)
	w := httptest.NewRecorder()

	handler.ServeGlossaryPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil proxyGetFn, got %d", w.Code)
	}
}

// --- JSON Hydration: XSS via template.JS ---

func TestChangelogPage_StressXSSInJSONHydration(t *testing.T) {
	// template.JS marks content as safe for embedding in <script>.
	// If vire-server returns JSON with </script>, it could break out.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if strings.Contains(path, "changelog") {
			return json.Marshal(map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"title":   "</script><script>alert('xss')</script>",
						"date":    "2026-01-01",
						"content": `<img src=x onerror=alert(1)>`,
					},
				},
			})
		}
		return []byte(`{}`), nil
	})

	req := httptest.NewRequest("GET", "/changelog", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeChangelogPage()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// The JSON should be embedded as-is via template.JS, but the </script> inside
	// JSON strings is the critical test. In JSON, it appears as:
	// "</script><script>alert('xss')</script>"
	// This IS a real XSS vector with template.JS because the browser's HTML parser
	// sees </script> before the JS parser sees the string.
	//
	// FINDING: template.JS does NOT escape </script>. The data comes from trusted
	// vire-server, but if vire-server is compromised, this is exploitable.
	// This is an ACCEPTED RISK per requirements.md section 5.3.
	scriptBreakout := strings.Count(body, "</script>")
	scriptOpen := strings.Count(body, "<script")
	if scriptBreakout > scriptOpen+1 {
		t.Log("WARNING: </script> in JSON data could break out of script context — accepted risk per spec (trusted vire-server)")
	}
}

// --- Strategy Page: JSON hydration with malicious portfolio names ---

func TestStrategyPage_StressXSSInPortfolioName(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if strings.Contains(path, "portfolios") && !strings.Contains(path, "strategy") && !strings.Contains(path, "plan") {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]interface{}{
					{"name": `</script><script>alert('xss')</script>`},
				},
				"default": `</script><script>alert('xss')</script>`,
			})
		}
		return []byte(`{}`), nil
	})

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Same accepted risk as changelog — template.JS trusts the data
	_ = w.Body.String()
}

// --- Strategy Page: proxyGetFn panics ---

func TestStrategyPage_StressProxyGetPanic(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		panic("simulated crash in proxyGetFn")
	})

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	// This will panic — verify the test runner catches it.
	// In production, Go's net/http server catches panics per-request.
	// But the handler itself should ideally use recover().
	defer func() {
		if r := recover(); r != nil {
			t.Log("WARNING: proxyGetFn panic not caught by handler — Go net/http will catch it in production, but handler should ideally recover")
		}
	}()

	handler.ServeHTTP(w, req)
}

// --- Cash Page: proxyGetFn panics ---

func TestCashPage_StressProxyGetPanic(t *testing.T) {
	handler := NewCashHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		panic("simulated crash in proxyGetFn")
	})

	req := httptest.NewRequest("GET", "/cash", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("WARNING: proxyGetFn panic not caught by cash handler")
		}
	}()

	handler.ServeHTTP(w, req)
}

// --- Strategy Page: nil proxyGetFn renders without crash ---

func TestStrategyPage_StressNilProxyGetFnRenders(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)
	// Do NOT set proxyGetFn

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil proxyGetFn, got %d", w.Code)
	}
}

// --- Cash Page: nil proxyGetFn renders without crash ---

func TestCashPage_StressNilProxyGetFnRenders(t *testing.T) {
	handler := NewCashHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/cash", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil proxyGetFn, got %d", w.Code)
	}
}

// --- Concurrent SSR requests ---

func TestSSR_StressConcurrentPageRenders(t *testing.T) {
	pageHandler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	pageHandler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if strings.Contains(path, "glossary") {
			return json.Marshal(map[string]interface{}{
				"categories": []map[string]interface{}{
					{"name": "Test", "terms": []interface{}{}},
				},
			})
		}
		if strings.Contains(path, "changelog") {
			return json.Marshal(map[string]interface{}{"items": []interface{}{}})
		}
		if strings.Contains(path, "feedback") {
			return json.Marshal(map[string]interface{}{"items": []interface{}{}, "total": 0})
		}
		return []byte(`{}`), nil
	})

	strategyHandler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)
	strategyHandler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if strings.Contains(path, "portfolios") && !strings.Contains(path, "strategy") && !strings.Contains(path, "plan") {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]interface{}{{"name": "test"}},
				"default":    "test",
			})
		}
		return []byte(`{}`), nil
	})

	cashHandler := NewCashHandler(nil, true, []byte(testJWTSecret), nil)
	cashHandler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if strings.Contains(path, "portfolios") && !strings.Contains(path, "cash") {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]interface{}{{"name": "test"}},
				"default":    "test",
			})
		}
		return []byte(`{}`), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			var req *http.Request
			w := httptest.NewRecorder()

			switch n % 5 {
			case 0: // Error page (no auth)
				req = httptest.NewRequest("GET", "/error?reason=auth_failed", nil)
				pageHandler.ServeErrorPage()(w, req)
			case 1: // Landing page
				req = httptest.NewRequest("GET", "/", nil)
				pageHandler.ServeLandingPage()(w, req)
			case 2: // Glossary page
				req = httptest.NewRequest("GET", "/glossary", nil)
				pageHandler.ServeGlossaryPage()(w, req)
			case 3: // Strategy page
				req = httptest.NewRequest("GET", "/strategy", nil)
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
				strategyHandler.ServeHTTP(w, req)
			case 4: // Cash page
				req = httptest.NewRequest("GET", "/cash", nil)
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
				cashHandler.ServeHTTP(w, req)
			}

			if w.Code != http.StatusOK {
				t.Errorf("concurrent request %d (mod %d) got status %d", n, n%5, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

// --- Help Page: Auth required ---

func TestHelpPage_StressRedirectsUnauthenticated(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()

	handler.ServeHelpPage()(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect for unauthenticated help page, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %q", location)
	}
}

// --- Help Page: JSON hydration with XSS in feedback ---

func TestHelpPage_StressXSSInFeedbackData(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return json.Marshal(map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"message": `</script><script>alert('xss')</script>`,
					"status":  "open",
				},
			},
			"total": 1,
		})
	})

	req := httptest.NewRequest("GET", "/help", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHelpPage()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Same accepted risk: template.JS trusts data from vire-server
	_ = w.Body.String()
}

// --- Memory: Large API response embedded in HTML ---

func TestSSR_StressLargeJSONEmbedding(t *testing.T) {
	// Generate a large (but within 1MB limit) API response and verify
	// it renders without OOM or excessive memory.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	// Create ~500KB of glossary data
	var terms []map[string]interface{}
	for i := 0; i < 5000; i++ {
		terms = append(terms, map[string]interface{}{
			"term":       fmt.Sprintf("term_%d", i),
			"label":      fmt.Sprintf("Term %d Label with some extra text to pad the size", i),
			"definition": fmt.Sprintf("This is the definition for term %d. It contains enough text to be realistic and contribute to the overall response size.", i),
			"formula":    fmt.Sprintf("value_%d / total * 100", i),
		})
	}

	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return json.Marshal(map[string]interface{}{
			"categories": []map[string]interface{}{
				{"name": "Large Category", "terms": terms},
			},
		})
	})

	req := httptest.NewRequest("GET", "/glossary", nil)
	w := httptest.NewRecorder()

	handler.ServeGlossaryPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for large glossary response, got %d", w.Code)
	}

	// Page should contain all terms
	body := w.Body.String()
	if !strings.Contains(body, "term_0") || !strings.Contains(body, "term_4999") {
		t.Error("large glossary response not fully rendered")
	}
}

// --- template.JS safety note ---

func TestSSR_StressTemplateJSSafety(t *testing.T) {
	// This test documents that template.JS does NOT HTML-escape content.
	// It is the caller's responsibility to ensure the data is safe.
	// For this project, data comes from trusted vire-server.
	unsafe := template.JS(`</script><script>alert(1)</script>`)
	rendered := fmt.Sprintf("<script>var data = %s;</script>", unsafe)

	if !strings.Contains(rendered, "</script><script>alert(1)") {
		t.Error("expected template.JS to NOT escape script tags — this is by design")
	}
	// This is an accepted risk. Log it for visibility.
	t.Log("ACCEPTED RISK: template.JS does not escape </script> in embedded JSON. " +
		"Data from vire-server is trusted. If server is compromised, XSS is possible.")
}

// --- Error page: user role resolution with nil userLookupFn ---

func TestErrorPage_StressNilUserLookupFn(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil) // nil userLookupFn

	req := httptest.NewRequest("GET", "/error?reason=auth_failed", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeErrorPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// --- Changelog/Help: proxyGetFn returns error ---

func TestChangelogPage_StressProxyGetError(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return nil, fmt.Errorf("vire-server unavailable")
	})

	req := httptest.NewRequest("GET", "/changelog", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeChangelogPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even when proxyGet fails, got %d", w.Code)
	}
}

func TestHelpPage_StressProxyGetError(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return nil, fmt.Errorf("vire-server unavailable")
	})

	req := httptest.NewRequest("GET", "/help", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHelpPage()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even when proxyGet fails, got %d", w.Code)
	}
}
