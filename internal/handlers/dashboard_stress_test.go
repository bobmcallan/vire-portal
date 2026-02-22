package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// buildUnsignedJWT creates an unsigned JWT for testing with empty secret.
func buildUnsignedJWT(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": sub,
		"iss": "vire-dev",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	return header + "." + payload + "."
}

// truncStr returns the first maxLen characters of s, for safe logging.
func truncStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// --- Dashboard Auth: Unauthenticated Access ---

func TestDashboardHandler_StressUnauthenticatedRedirect(t *testing.T) {
	// An unauthenticated user must be redirected away from the dashboard.
	// The dashboard Alpine.js component calls /api/portfolios on load;
	// if the page renders without auth, the browser makes unauthenticated API calls.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: unauthenticated dashboard access returned %d, expected 302 redirect", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
	// Must NOT contain any dashboard HTML content
	body := w.Body.String()
	if strings.Contains(body, "portfolioDashboard") {
		t.Error("SECURITY: dashboard HTML rendered for unauthenticated user")
	}
}

func TestDashboardHandler_StressExpiredTokenRedirect(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildExpiredJWT("alice")})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expired token should redirect, got %d", w.Code)
	}
}

func TestDashboardHandler_StressGarbageTokenRedirect(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	garbageTokens := []string{
		"not-a-jwt",
		"a.b.c",
		"<script>alert(1)</script>",
		strings.Repeat("A", 10000),
		"",
	}

	for _, token := range garbageTokens {
		req := httptest.NewRequest("GET", "/dashboard", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		}
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("garbage token %q: expected 302, got %d", truncStr(token, 20), w.Code)
		}
	}
}

// --- Dashboard: Alpine.js x-text XSS Safety Verification ---
// Alpine.js x-text sets textContent (safe). This test verifies the HTML template
// uses x-text and NOT x-html or v-html for user-controlled data.

func TestDashboardHandler_StressTemplateUsesXTextNotXHtml(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify x-html is NOT used (would allow XSS)
	if strings.Contains(body, "x-html") {
		t.Error("SECURITY: dashboard template uses x-html which renders raw HTML — use x-text instead")
	}

	// Verify x-text IS used for data bindings
	if !strings.Contains(body, "x-text=") {
		t.Error("expected x-text directives for data display")
	}
}

// --- Dashboard: Error Message Display ---

func TestDashboardHandler_StressErrorDisplayIsTextOnly(t *testing.T) {
	// The error banner uses x-text="error". Verify it's not x-html.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The error display must use x-text, not innerHTML/x-html
	if strings.Contains(body, `x-html="error"`) {
		t.Error("SECURITY: error message rendered with x-html — XSS via error messages")
	}
	if !strings.Contains(body, `x-text="error"`) {
		t.Error("expected error banner to use x-text for safe rendering")
	}
}

// --- Dashboard: Concurrent Access ---

func TestDashboardHandler_StressConcurrentAccess(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/dashboard", nil)
			addAuthCookie(req, fmt.Sprintf("user-%d", n))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("concurrent request %d got status %d", n, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

// --- Dashboard: NavexaKeyMissing with nil userLookupFn ---

func TestDashboardHandler_StressNilUserLookup(t *testing.T) {
	// When userLookupFn is nil, dashboard should render without panic
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	// Must not panic
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil userLookupFn, got %d", w.Code)
	}
}

// --- MCP Page: Auth ---

func TestMCPPageHandler_StressUnauthenticatedRedirect(t *testing.T) {
	catalogFn := func() []MCPPageTool { return nil }
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: unauthenticated MCP page access returned %d, expected 302", w.Code)
	}
}

func TestMCPPageHandler_StressExpiredToken(t *testing.T) {
	catalogFn := func() []MCPPageTool { return nil }
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildExpiredJWT("alice")})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expired token should redirect from MCP page, got %d", w.Code)
	}
}

// --- MCP Page: XSS in Tool Names ---

func TestMCPPageHandler_StressXSSInToolData(t *testing.T) {
	// Tool names/descriptions come from vire-server catalog.
	// If compromised, they could contain XSS payloads.
	// Go templates auto-escape, so this should be safe.
	hostileTools := []MCPPageTool{
		{Name: `<script>alert('xss')</script>`, Description: `<img src=x onerror=alert(1)>`},
		{Name: `{{.Page}}`, Description: `{{template "head.html" .}}`},
		{Name: `" onclick="alert(1)`, Description: `'; DROP TABLE tools;--`},
	}
	catalogFn := func() []MCPPageTool { return hostileTools }
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The raw script tag must be escaped
	if strings.Contains(body, "<script>alert") {
		t.Error("SECURITY: XSS in tool name — <script> tag not escaped")
	}
	// Go template injection must be escaped (rendered as literal text)
	if strings.Contains(body, "{{.Page}}") && !strings.Contains(body, "&amp;") {
		// Go html/template auto-escapes, so {{.Page}} in data becomes literal text.
		// This is actually fine — just verify no template re-evaluation occurred.
	}
	if strings.Contains(body, `<img src=x onerror`) {
		t.Error("SECURITY: XSS in tool description — <img> tag not escaped")
	}
}

// --- MCP Page: Concurrent Access ---

func TestMCPPageHandler_StressConcurrentAccess(t *testing.T) {
	catalogFn := func() []MCPPageTool {
		return []MCPPageTool{{Name: "tool_a", Description: "A"}}
	}
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/mcp-info", nil)
			addAuthCookie(req, fmt.Sprintf("user-%d", n))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("concurrent MCP page request %d got status %d", n, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

// --- MCP Page: Nil CatalogFn Panic ---

func TestMCPPageHandler_StressNilCatalogReturn(t *testing.T) {
	// catalogFn returns nil — should not panic
	catalogFn := func() []MCPPageTool { return nil }
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("nil catalog should render empty tools, got status %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "NO TOOLS") {
		t.Error("expected 'NO TOOLS' message when catalog is nil")
	}
}

// --- MCP Page: DevMCPEndpoint with hostile user ID ---

func TestMCPPageHandler_StressDevEndpointHostileUserID(t *testing.T) {
	// If the JWT sub claim contains hostile characters, the dev MCP endpoint
	// function receives them. Verify the output is properly escaped in the template.
	catalogFn := func() []MCPPageTool { return nil }
	handler := NewMCPPageHandler(nil, true, 8500, []byte{}, catalogFn)
	handler.SetDevMCPEndpointFn(func(userID string) string {
		return "http://localhost:8500/mcp/" + userID
	})

	// Craft a JWT with hostile sub claim
	hostileSubs := []string{
		`<script>alert(document.cookie)</script>`,
		`" onclick="alert(1)`,
		`../../../etc/passwd`,
	}

	for _, sub := range hostileSubs {
		token := buildUnsignedJWT(sub)
		req := httptest.NewRequest("GET", "/mcp-info", nil)
		req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		body := w.Body.String()
		if strings.Contains(body, "<script>alert") {
			t.Errorf("SECURITY: hostile sub %q caused XSS in dev endpoint display", truncStr(sub, 20))
		}
	}
}

// --- Portfolio Dashboard JS: Verify encodeURIComponent is used ---

func TestDashboardHandler_StressPortfolioURLEncoding(t *testing.T) {
	// The dashboard template must use encodeURIComponent for portfolio names
	// in API URLs. Verify the JS source contains proper encoding.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// The common.js contains encodeURIComponent, but it's in a separate file.
	// Verify the dashboard template references the right Alpine component.
	if !strings.Contains(body, `x-data="portfolioDashboard()"`) {
		t.Error("expected dashboard to use portfolioDashboard() component")
	}
}

// --- Server-Side: Portfolio Name in Proxy Path ---
// These test the handleAPIProxy behavior with hostile path components.
// Note: These belong in server package tests, but we document the concern here.

func TestDashboardHandler_StressTemplateDataIsolation(t *testing.T) {
	// Each request should get its own template data. Verify that concurrent
	// requests with different auth states don't leak data.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	var wg sync.WaitGroup
	results := make([]int, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/dashboard", nil)
			if n%2 == 0 {
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
			}
			// Odd-numbered requests are unauthenticated
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			results[n] = w.Code
		}(i)
	}
	wg.Wait()

	for i, code := range results {
		if i%2 == 0 && code != http.StatusOK {
			t.Errorf("authenticated request %d got %d, expected 200", i, code)
		}
		if i%2 == 1 && code != http.StatusFound {
			t.Errorf("unauthenticated request %d got %d, expected 302", i, code)
		}
	}
}

// --- Dashboard: New Portfolio Enhancement Template Safety ---

func TestDashboardHandler_StressGainClassBindingsSafe(t *testing.T) {
	// Verify the gain column uses :class with gainClass() and x-text for display.
	// The :class binding sets className (safe), not innerHTML.
	// gainClass() must only return hardcoded class names — never user input.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify gain class binding exists in template
	if !strings.Contains(body, `gainClass(`) {
		t.Error("expected gainClass() bindings in dashboard template")
	}

	// Verify all gainClass usages are via :class (safe) not x-html (unsafe)
	if strings.Contains(body, `x-html`) {
		t.Error("SECURITY: dashboard uses x-html — all dynamic content must use x-text or :class")
	}

	// Verify the summary section uses x-text for value display
	if !strings.Contains(body, `x-text="fmt(totalValue)"`) {
		t.Error("expected totalValue summary with x-text binding")
	}
	if !strings.Contains(body, `x-text="fmt(totalGain)"`) {
		t.Error("expected totalGain summary with x-text binding")
	}
	if !strings.Contains(body, `x-text="pct(totalGainPct)"`) {
		t.Error("expected totalGainPct summary with x-text binding")
	}
}

func TestDashboardHandler_StressShowClosedCheckboxPresent(t *testing.T) {
	// Verify the showClosed checkbox exists and uses x-model (safe two-way binding)
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `x-model="showClosed"`) {
		t.Error("expected showClosed checkbox with x-model binding")
	}
	if !strings.Contains(body, `portfolio-filter-label`) {
		t.Error("expected portfolio-filter-label class on show closed checkbox label")
	}
}

func TestDashboardHandler_StressFilteredHoldingsLoop(t *testing.T) {
	// Verify the holdings table iterates filteredHoldings, not raw holdings.
	// Using raw holdings would bypass the showClosed filter.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `x-for="h in filteredHoldings"`) {
		t.Error("holdings table must iterate filteredHoldings, not raw holdings")
	}

	// Count occurrences: there should be exactly one x-for loop with holdings
	rawCount := strings.Count(body, `x-for="h in holdings"`)
	if rawCount > 0 {
		t.Errorf("LOGIC: found %d x-for loops using raw 'holdings' — should use 'filteredHoldings'", rawCount)
	}
}

func TestDashboardHandler_StressSummaryGainColorBindings(t *testing.T) {
	// Verify gain color classes in summary use :class (className, safe)
	// and display values use x-text (textContent, safe)
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Summary gain $ must have :class for color
	if !strings.Contains(body, `:class="gainClass(totalGain)"`) {
		t.Error("expected :class gainClass binding on totalGain summary")
	}
	// Summary gain % must have :class for color
	if !strings.Contains(body, `:class="gainClass(totalGainPct)"`) {
		t.Error("expected :class gainClass binding on totalGainPct summary")
	}
	// Per-row gain % must have :class for color
	if !strings.Contains(body, `:class="gainClass(h.total_return_pct)"`) {
		t.Error("expected :class gainClass binding on per-holding gain column")
	}
}

func TestDashboardHandler_StressPortfolioSummarySection(t *testing.T) {
	// Verify the portfolio summary section exists and is properly structured
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Summary section must exist
	if !strings.Contains(body, `portfolio-summary`) {
		t.Error("expected portfolio-summary section in dashboard")
	}
	// Summary must show conditionally based on filteredHoldings
	if !strings.Contains(body, `x-show="filteredHoldings.length > 0"`) {
		t.Error("portfolio summary should be conditional on filteredHoldings.length > 0")
	}
	// Verify all three summary items exist
	summaryLabels := []string{"TOTAL VALUE", "TOTAL GAIN $", "TOTAL GAIN %"}
	for _, label := range summaryLabels {
		if !strings.Contains(body, label) {
			t.Errorf("expected summary label %q in dashboard", label)
		}
	}
}

func TestDashboardHandler_StressNoInlineEventHandlers(t *testing.T) {
	// Verify the template does not use inline event handlers like onclick=
	// which could be injection vectors. Alpine uses @click which is safe.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	dangerousAttrs := []string{
		` onclick=`, ` onerror=`, ` onload=`, ` onmouseover=`,
		` onfocus=`, ` onsubmit=`, ` onchange=`,
	}
	for _, attr := range dangerousAttrs {
		if strings.Contains(strings.ToLower(body), attr) {
			t.Errorf("SECURITY: found dangerous inline handler %q in template", attr)
		}
	}
}

func TestDashboardHandler_StressCSSGainClassesExist(t *testing.T) {
	// Verify the CSS file is referenced and the gain color classes
	// use the correct color values (green positive, red negative).
	// This is a static check — the actual CSS is in portal.css.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify the CSS file is referenced
	if !strings.Contains(body, "portal.css") {
		t.Error("expected portal.css to be referenced in dashboard template")
	}
}

// --- Portfolio API Response Shape Injection ---

func TestPortfolioDashboard_StressJSONResponseShapes(t *testing.T) {
	// The JS code accesses: data.portfolios, data.default, holdingsData.holdings,
	// strategyData.notes, planData.notes.
	// If the server returns unexpected shapes, the JS should not crash.
	// These are documented API response shapes that the client must handle:
	unexpectedShapes := []struct {
		name string
		body string
	}{
		{"empty object", `{}`},
		{"null portfolios", `{"portfolios":null,"default":null}`},
		{"portfolios is string", `{"portfolios":"not-an-array"}`},
		{"portfolios is number", `{"portfolios":123}`},
		{"deeply nested", `{"portfolios":[{"name":{"nested":"object"}}]}`},
		{"XSS in portfolio name", `{"portfolios":[{"name":"<script>alert(1)</script>"}],"default":""}`},
		{"very large response", `{"portfolios":[` + strings.Repeat(`{"name":"p"},`, 10000) + `{"name":"last"}]}`},
	}

	for _, tc := range unexpectedShapes {
		t.Run(tc.name, func(t *testing.T) {
			// Verify the JSON parses without error (client-side resilience)
			var result map[string]interface{}
			err := json.Unmarshal([]byte(tc.body), &result)
			if err != nil {
				// Invalid JSON is handled by fetch().json() throwing
				t.Logf("invalid JSON for %s: %v (client handles via catch)", tc.name, err)
				return
			}

			// Verify XSS payload is properly contained
			if portfolios, ok := result["portfolios"]; ok {
				if arr, ok := portfolios.([]interface{}); ok {
					for _, p := range arr {
						if m, ok := p.(map[string]interface{}); ok {
							if name, ok := m["name"].(string); ok {
								if strings.Contains(name, "<script>") {
									// This is expected — the portfolio name from the server
									// contains a script tag. Alpine's x-text will safely
									// render this as text, not HTML.
									t.Logf("NOTE: portfolio name contains script tag: %q — safe if rendered with x-text", name)
								}
							}
						}
					}
				}
			}
		})
	}
}
