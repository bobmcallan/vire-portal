package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

// =============================================================================
// Gemini Admin Handler — Adversarial / Stress Tests
// =============================================================================

// --- Helper ---

func newGeminiTestHandler(role string) *AdminGeminiHandler {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: userID,
			Email:    userID + "@test.com",
			Role:     role,
		}, nil
	}
	return NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookup)
}

// =============================================================================
// SafeJS HELPER FUNCTION TESTS
// =============================================================================

func TestStressGemini_SafeJSEscaping(t *testing.T) {
	cases := []struct {
		name  string
		input string
		bad   string // must NOT appear in output
	}{
		{"lowercase", `{"x":"</script>"}`, `</script>`},
		{"uppercase", `{"x":"</SCRIPT>"}`, `</SCRIPT>`},
		{"mixed case", `{"x":"</Script>"}`, `</Script>`},
		{"partial", `{"x":"</scriPT>"}`, `</scriPT>`},
		{"multiple", `{"a":"</script>","b":"</script>"}`, `</script>`},
		{"nested", `{"x":"</script></script>"}`, `</script>`},
	}

	for _, tc := range cases {
		result := string(SafeJS([]byte(tc.input)))
		if strings.Contains(result, tc.bad) {
			t.Errorf("SafeJS(%s): output still contains %q", tc.name, tc.bad)
		}
	}
}

// =============================================================================
// AUTH BYPASS TESTS
// =============================================================================

func TestStressGeminiAdminGate(t *testing.T) {
	roles := []struct {
		role     string
		wantCode int
		wantLoc  string
	}{
		{"user", http.StatusFound, "/dashboard"},
		{"viewer", http.StatusFound, "/dashboard"},
		{"", http.StatusFound, "/dashboard"},
	}

	for _, tc := range roles {
		userLookupFn := func(userID string) (*client.UserProfile, error) {
			return &client.UserProfile{Username: "user", Role: tc.role}, nil
		}

		handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)

		req := httptest.NewRequest("GET", "/admin/gemini", nil)
		addAuthCookie(req, "test-user")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != tc.wantCode {
			t.Errorf("role %q: expected %d, got %d", tc.role, tc.wantCode, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != tc.wantLoc {
			t.Errorf("role %q: expected redirect to %s, got %s", tc.role, tc.wantLoc, loc)
		}
	}
}

func TestStressGemini_UnauthenticatedRedirect(t *testing.T) {
	handler := newGeminiTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: unauthenticated admin gemini access returned %d, expected 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %s", loc)
	}
	body := w.Body.String()
	if strings.Contains(body, "GEMINI USAGE") {
		t.Error("SECURITY: admin gemini HTML rendered for unauthenticated user")
	}
}

func TestStressGemini_ExpiredTokenRedirect(t *testing.T) {
	handler := newGeminiTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildExpiredJWT("alice")})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expired token should redirect, got %d", w.Code)
	}
}

func TestStressGemini_GarbageTokenRedirect(t *testing.T) {
	handler := newGeminiTestHandler("admin")

	garbageTokens := []string{
		"not-a-jwt",
		"a.b.c",
		"<script>alert(1)</script>",
		strings.Repeat("A", 10000),
		"",
	}

	for _, token := range garbageTokens {
		req := httptest.NewRequest("GET", "/admin/gemini", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		}
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("garbage token %q: expected 302, got %d", token[:min(len(token), 20)], w.Code)
		}
	}
}

// =============================================================================
// ADMIN ROLE GATE — CASE SENSITIVITY & EDGE CASES
// =============================================================================

func TestStressGemini_RoleCaseSensitivity(t *testing.T) {
	hostileRoles := []string{"Admin", "ADMIN", "aDmIn", " admin", "admin ", "admin\n"}

	for _, role := range hostileRoles {
		handler := newGeminiTestHandler(role)

		req := httptest.NewRequest("GET", "/admin/gemini", nil)
		addAuthCookie(req, "case-test")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("SECURITY: role %q was accepted as admin (status %d)", role, w.Code)
		}
	}
}

func TestStressGemini_NilUserLookupRedirects(t *testing.T) {
	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: nil userLookupFn should redirect, got %d", w.Code)
	}
}

func TestStressGemini_UserLookupErrorRedirects(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return nil, fmt.Errorf("database connection failed")
	}
	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: userLookup error should redirect, got %d", w.Code)
	}
}

func TestStressGemini_UserLookupReturnsNilRedirects(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return nil, nil
	}
	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: nil user profile should redirect, got %d", w.Code)
	}
}

func TestStressGemini_NonAdminBodyNotLeaked(t *testing.T) {
	proxyGetFn := func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":999,"secret_data":"should_not_leak"}`), nil
	}

	handler := newGeminiTestHandler("user")
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "regular-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: non-admin got status %d, expected 302", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "secret_data") || strings.Contains(body, "should_not_leak") {
		t.Error("SECURITY: usage data leaked to non-admin user")
	}
}

// =============================================================================
// XSS — RAW API RESPONSE DATA IN template.JS
// =============================================================================

func TestStressGeminiXSSInUsageData(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	// json.Marshal escapes < as \u003c — safe
	xssPayload := map[string]interface{}{
		"total_calls": 1,
		"by_job_type": []map[string]interface{}{
			{"job_type": `</script><script>alert(1)</script>`, "calls": 1},
		},
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return json.Marshal(xssPayload)
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, `</script><script>alert(1)</script>`) {
		t.Error("SECURITY: XSS payload in usage data appears unescaped")
	}
}

func TestStressGemini_XSSRawBytesFromAPI(t *testing.T) {
	// SafeJS escapes </script in raw API response bytes, preventing XSS
	// even if the upstream API returns hand-crafted JSON with literal
	// </script> tags (not through json.Marshal).
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	// Simulate raw bytes with literal </script> — SafeJS must escape this
	rawXSS := `{"total_calls":1,"by_job_type":[{"job_type":"</script><script>alert('xss')</script>","calls":1}]}`

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return []byte(rawXSS), nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// SafeJS should escape </script to <\/script, preventing breakout
	if strings.Contains(body, `<script>alert('xss')</script>`) {
		t.Error("SECURITY: raw API response with </script> broke out of JSON hydration block — SafeJS not applied")
	}
}

func TestStressGemini_XSSEventHandlerInjectionViaMarshal(t *testing.T) {
	// Verify json.Marshal path escapes HTML event handlers
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	// json.Marshal will escape < to \u003c, making these safe
	proxyGetFn := func(path, userID string) ([]byte, error) {
		return json.Marshal(map[string]interface{}{
			"total_calls": 1,
			"by_model": []map[string]interface{}{
				{"model": `<img src=x onerror=alert(1)>`, "calls": 1},
			},
		})
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// json.Marshal escapes < to \u003c, so the literal <img> tag should not appear
	if strings.Contains(body, `<img src=x onerror=alert(1)>`) {
		t.Error("SECURITY: json.Marshal should escape <img> tags via \\u003c")
	}
}

// =============================================================================
// NIL/EMPTY FIELDS — TEMPLATE CRASH RESILIENCE
// =============================================================================

func TestStressGeminiNilFields(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":null,"total_prompt_tokens":null,"total_output_tokens":null,"by_job_type":null,"by_model":null,"by_day":null,"recent_calls":null}`), nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with nil fields, got %d", w.Code)
	}
}

func TestStressGemini_EmptyArrays(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":0,"total_prompt_tokens":0,"total_output_tokens":0,"by_job_type":[],"by_model":[],"by_day":[],"recent_calls":[]}`), nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with empty arrays, got %d", w.Code)
	}
}

func TestStressGemini_MalformedJSON(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	// API returns garbage — proxyGetFn still returns bytes
	proxyGetFn := func(path, userID string) ([]byte, error) {
		return []byte(`{invalid json!!!`), nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// The handler embeds raw bytes as template.JS — malformed JSON will
	// cause a JS parse error on the client but must not crash the server.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with malformed JSON, got %d", w.Code)
	}
}

func TestStressGemini_ProxyGetFnError(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return nil, fmt.Errorf("connection refused")
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with proxy error (graceful degradation), got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "usage: null") {
		t.Error("expected usage: null when proxy returns error")
	}
	// Internal error must not leak
	if strings.Contains(body, "connection refused") {
		t.Error("SECURITY: internal error details leaked into HTML output")
	}
}

// =============================================================================
// INTERNAL DETAILS EXPOSURE
// =============================================================================

func TestStressGemini_NoAPIURLInHTML(t *testing.T) {
	handler := newGeminiTestHandler("admin")
	handler.SetAPIURL("http://internal-server:8080")

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":1}`), nil
	}
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "internal-server:8080") {
		t.Error("SECURITY: internal API URL leaked into HTML output")
	}
}

// =============================================================================
// CONCURRENT ACCESS
// =============================================================================

func TestStressGemini_ConcurrentAccess(t *testing.T) {
	handler := newGeminiTestHandler("admin")
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":42}`), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/admin/gemini", nil)
			addAuthCookie(req, fmt.Sprintf("admin-%d", n))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("concurrent admin request %d got status %d", n, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestStressGemini_ConcurrentMixedRoles(t *testing.T) {
	adminLookup := func(userID string) (*client.UserProfile, error) {
		if strings.HasPrefix(userID, "admin-") {
			return &client.UserProfile{Role: "admin"}, nil
		}
		return &client.UserProfile{Role: "user"}, nil
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), adminLookup)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":42,"secret":"admin-only-data"}`), nil
	})

	var wg sync.WaitGroup
	results := make([]struct {
		code int
		body string
	}, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/admin/gemini", nil)
			if n%2 == 0 {
				addAuthCookie(req, fmt.Sprintf("admin-%d", n))
			} else {
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			results[n] = struct {
				code int
				body string
			}{w.Code, w.Body.String()}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if i%2 == 0 {
			if r.code != http.StatusOK {
				t.Errorf("admin request %d got %d, expected 200", i, r.code)
			}
		} else {
			if r.code != http.StatusFound {
				t.Errorf("non-admin request %d got %d, expected 302", i, r.code)
			}
			if strings.Contains(r.body, "admin-only-data") {
				t.Errorf("SECURITY: non-admin request %d got admin data in body", i)
			}
		}
	}
}

// =============================================================================
// LARGE PAYLOAD
// =============================================================================

func TestStressGemini_LargePayload(t *testing.T) {
	userLookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "admin", Role: "admin"}, nil
	}

	// Generate a large recent_calls array
	calls := make([]map[string]interface{}, 1000)
	for i := range calls {
		calls[i] = map[string]interface{}{
			"timestamp":     "2026-03-09T12:00:00Z",
			"job_type":      fmt.Sprintf("job_%d", i),
			"model":         "gemini-pro",
			"prompt_tokens": 100 + i,
			"output_tokens": 50 + i,
			"duration_ms":   200 + i,
		}
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		return json.Marshal(map[string]interface{}{
			"total_calls":         10000,
			"total_prompt_tokens": 999999,
			"total_output_tokens": 500000,
			"recent_calls":        calls,
		})
	}

	handler := NewAdminGeminiHandler(nil, true, []byte(testJWTSecret), userLookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with large payload, got %d", w.Code)
	}
}

// =============================================================================
// PAGE STRUCTURE VERIFICATION
// =============================================================================

func TestStressGemini_PageStructure(t *testing.T) {
	handler := newGeminiTestHandler("admin")
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":1}`), nil
	})

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "GEMINI USAGE") {
		t.Error("expected GEMINI USAGE heading")
	}
	if !strings.Contains(body, `class="page"`) {
		t.Error("expected .page class")
	}
	if !strings.Contains(body, "portal.css") {
		t.Error("expected portal.css reference")
	}
	if !strings.Contains(body, "footer") {
		t.Error("expected footer")
	}
	if !strings.Contains(body, "geminiUsage()") {
		t.Error("expected geminiUsage() Alpine component")
	}
}

func TestStressGemini_NavGeminiLinkForAdmin(t *testing.T) {
	handler := newGeminiTestHandler("admin")

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `href="/admin/gemini"`) {
		t.Error("expected Gemini nav link for admin user")
	}
}

func TestStressGemini_NavGeminiLinkHiddenForNonAdmin(t *testing.T) {
	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Role: "user"}, nil
	}
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), userLookup)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "regular-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, `href="/admin/gemini"`) {
		t.Error("SECURITY: non-admin user can see Gemini admin nav link")
	}
}

// =============================================================================
// NO INLINE EVENT HANDLERS
// =============================================================================

func TestStressGemini_NoInlineEventHandlers(t *testing.T) {
	handler := newGeminiTestHandler("admin")
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		return []byte(`{"total_calls":1}`), nil
	})

	req := httptest.NewRequest("GET", "/admin/gemini", nil)
	addAuthCookie(req, "admin-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	dangerousAttrs := []string{
		` onclick=`, ` onerror=`, ` onload=`, ` onmouseover=`,
		` onfocus=`, ` onsubmit=`, ` onchange=`,
	}
	for _, attr := range dangerousAttrs {
		if strings.Contains(strings.ToLower(body), attr) {
			t.Errorf("SECURITY: found dangerous inline handler %q in gemini template", attr)
		}
	}
}
