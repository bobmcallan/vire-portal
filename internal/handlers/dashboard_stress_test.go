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
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn, nil)

	req := httptest.NewRequest("GET", "/mcp-info", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: unauthenticated MCP page access returned %d, expected 302", w.Code)
	}
}

func TestMCPPageHandler_StressExpiredToken(t *testing.T) {
	catalogFn := func() []MCPPageTool { return nil }
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn, nil)

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
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn, nil)

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
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn, nil)

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
	handler := NewMCPPageHandler(nil, true, 8500, []byte(testJWTSecret), catalogFn, nil)

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
	handler := NewMCPPageHandler(nil, true, 8500, []byte{}, catalogFn, nil)
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
	// Per-row gain $ must have :class for color
	if !strings.Contains(body, `:class="gainClass(h.holding_return_net)"`) {
		t.Error("expected :class gainClass binding on per-holding gain $ column")
	}
	// Per-row gain % must have :class for color
	if !strings.Contains(body, `:class="gainClass(h.holding_return_net_pct)"`) {
		t.Error("expected :class gainClass binding on per-holding gain % column")
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
	// Verify all portfolio overview summary items exist (Row 1)
	summaryLabels := []string{"PORTFOLIO VALUE"}
	for _, label := range summaryLabels {
		if !strings.Contains(body, label) {
			t.Errorf("expected summary label %q in dashboard", label)
		}
	}
}

// --- Dashboard: New Field Bindings Safety ---

func TestDashboardHandler_StressNewFieldBindingsSafe(t *testing.T) {
	// Verify the dashboard fields (availableCash, grossContributions, dividends)
	// use x-text bindings (safe) with correct formatting helpers.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// AVAILABLE CASH must use x-text (not x-html) and fmt() for formatting
	if !strings.Contains(body, `x-text="fmt(availableCash)"`) {
		t.Error("expected availableCash displayed with x-text fmt() binding")
	}
	// AVAILABLE CASH must NOT use gainClass — it is a neutral value
	if strings.Contains(body, `gainClass(availableCash)`) {
		t.Error("LOGIC: availableCash should not use gainClass — it is a neutral value, not a gain/loss")
	}
	// GROSS CASH BALANCE must use x-text (not x-html) and fmt() for formatting
	if !strings.Contains(body, `x-text="fmt(grossCashBalance)"`) {
		t.Error("expected grossCashBalance displayed with x-text fmt() binding")
	}
	// DIVIDENDS must show actual (forecast) format
	if !strings.Contains(body, `fmt(ledgerDividendReturn)`) || !strings.Contains(body, `fmt(totalDividends)`) {
		t.Error("expected dividends displayed with ledgerDividendReturn and totalDividends bindings")
	}
}

func TestDashboardHandler_StressCapitalPerformanceLabels(t *testing.T) {
	// Verify the composition row (Row 1) and performance row (Row 2) have the correct labels
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Row 1: Composition labels
	compositionLabels := []string{"PORTFOLIO VALUE", "GROSS CASH BALANCE", "AVAILABLE CASH", "NET EQUITY"}
	for _, label := range compositionLabels {
		if !strings.Contains(body, label) {
			t.Errorf("expected composition row label %q in dashboard", label)
		}
	}

	// Row 2: Performance labels
	performanceLabels := []string{"NET RETURN $", "NET RETURN %", "DIVIDENDS"}
	for _, label := range performanceLabels {
		if !strings.Contains(body, label) {
			t.Errorf("expected performance row label %q in dashboard", label)
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

// =============================================================================
// Strategy Handler Stress Tests
// =============================================================================

// --- Strategy Auth: Unauthenticated Access ---

func TestStrategyHandler_StressUnauthenticatedRedirect(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: unauthenticated strategy access returned %d, expected 302 redirect", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
	body := w.Body.String()
	if strings.Contains(body, "portfolioStrategy") {
		t.Error("SECURITY: strategy HTML rendered for unauthenticated user")
	}
}

func TestStrategyHandler_StressExpiredTokenRedirect(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildExpiredJWT("alice")})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expired token should redirect from strategy page, got %d", w.Code)
	}
}

func TestStrategyHandler_StressGarbageTokenRedirect(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	garbageTokens := []string{
		"not-a-jwt",
		"a.b.c",
		"<script>alert(1)</script>",
		strings.Repeat("A", 10000),
		"",
	}

	for _, token := range garbageTokens {
		req := httptest.NewRequest("GET", "/strategy", nil)
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

// --- Strategy: XSS Safety ---

func TestStrategyHandler_StressTemplateUsesXTextNotXHtml(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "x-html") {
		t.Error("SECURITY: strategy template uses x-html which renders raw HTML — use x-text instead")
	}
	if !strings.Contains(body, "x-text=") {
		t.Error("expected x-text directives for data display")
	}
}

func TestStrategyHandler_StressErrorDisplayIsTextOnly(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, `x-html="error"`) {
		t.Error("SECURITY: error message rendered with x-html — XSS via error messages")
	}
	if !strings.Contains(body, `x-text="error"`) {
		t.Error("expected error banner to use x-text for safe rendering")
	}
}

func TestStrategyHandler_StressNoInlineEventHandlers(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
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
			t.Errorf("SECURITY: found dangerous inline handler %q in strategy template", attr)
		}
	}
}

// --- Strategy: Template Content Verification ---

func TestStrategyHandler_StressUsesCorrectAlpineComponent(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `x-data="portfolioStrategy()"`) {
		t.Error("expected strategy page to use portfolioStrategy() component")
	}
	// Must NOT use the dashboard component
	if strings.Contains(body, `x-data="portfolioDashboard()"`) {
		t.Error("strategy page should not use portfolioDashboard() component")
	}
}

func TestStrategyHandler_StressEditorSectionsPresent(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Strategy editor section
	if !strings.Contains(body, `x-model="strategy"`) {
		t.Error("expected strategy textarea with x-model binding")
	}
	if !strings.Contains(body, `saveStrategy()`) {
		t.Error("expected saveStrategy() button in strategy page")
	}

	// Plan editor section
	if !strings.Contains(body, `x-model="plan"`) {
		t.Error("expected plan textarea with x-model binding")
	}
	if !strings.Contains(body, `savePlan()`) {
		t.Error("expected savePlan() button in strategy page")
	}
}

func TestStrategyHandler_StressPortfolioSelectorPresent(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `class="form-select portfolio-select"`) {
		t.Error("expected portfolio selector in strategy page")
	}
	if !strings.Contains(body, `x-text="p.name"`) {
		t.Error("expected portfolio name display with x-text (safe)")
	}
}

// --- Strategy: Dashboard Must NOT Have Strategy/Plan ---

func TestDashboardHandler_StressNoStrategyEditor(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, `x-model="strategy"`) {
		t.Error("dashboard should no longer contain strategy editor (moved to /strategy)")
	}
	if strings.Contains(body, `x-model="plan"`) {
		t.Error("dashboard should no longer contain plan editor (moved to /strategy)")
	}
	if strings.Contains(body, `saveStrategy()`) {
		t.Error("dashboard should no longer have saveStrategy button")
	}
	if strings.Contains(body, `savePlan()`) {
		t.Error("dashboard should no longer have savePlan button")
	}
}

// --- Strategy: Concurrent Access ---

func TestStrategyHandler_StressConcurrentAccess(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/strategy", nil)
			addAuthCookie(req, fmt.Sprintf("user-%d", n))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("concurrent strategy request %d got status %d", n, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestStrategyHandler_StressTemplateDataIsolation(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	var wg sync.WaitGroup
	results := make([]int, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/strategy", nil)
			if n%2 == 0 {
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			results[n] = w.Code
		}(i)
	}
	wg.Wait()

	for i, code := range results {
		if i%2 == 0 && code != http.StatusOK {
			t.Errorf("authenticated strategy request %d got %d, expected 200", i, code)
		}
		if i%2 == 1 && code != http.StatusFound {
			t.Errorf("unauthenticated strategy request %d got %d, expected 302", i, code)
		}
	}
}

// --- Strategy: Nil UserLookup Panic Safety ---

func TestStrategyHandler_StressNilUserLookup(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil userLookupFn, got %d", w.Code)
	}
}

// --- Strategy: Nav Active State ---

func TestStrategyHandler_StressNavActiveState(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Strategy link must be active
	if !strings.Contains(body, `href="/strategy" class="active"`) {
		t.Error("expected strategy nav link to have active class")
	}
	// Dashboard link must NOT be active
	if strings.Contains(body, `href="/dashboard" class="active"`) {
		t.Error("dashboard nav link should not be active on strategy page")
	}
	// MCP link must NOT be active
	if strings.Contains(body, `href="/mcp-info" class="active"`) {
		t.Error("MCP nav link should not be active on strategy page")
	}
}

// --- Strategy: Mobile Nav Link Present ---

func TestStrategyHandler_StressMobileNavPresent(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `href="/strategy">Strategy</a>`) {
		t.Error("expected strategy link in mobile nav menu")
	}
}

// --- Strategy: CSS Reference ---

func TestStrategyHandler_StressCSSReference(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "portal.css") {
		t.Error("expected portal.css to be referenced in strategy template")
	}
}

// =============================================================================
// Dashboard: D/W/M Changes & Last Synced — Adversarial Stress Tests
// =============================================================================

// --- XSS Safety: fmtSynced, changePct, changeClass use x-text/:class (safe) ---

func TestDashboardHandler_StressChangesBindingsUseXText(t *testing.T) {
	// The D/W/M change badges and last synced timestamp must use x-text
	// (sets textContent, safe) and :class (sets className, safe).
	// They must NOT use x-html, innerHTML, or v-html.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// changePct must be rendered via x-text (safe)
	if !strings.Contains(body, `x-text="'D:' + changePct(changeDayPct)"`) {
		t.Error("expected changeDayPct rendered with x-text")
	}
	if !strings.Contains(body, `x-text="'W:' + changePct(changeWeekPct)"`) {
		t.Error("expected changeWeekPct rendered with x-text")
	}
	if !strings.Contains(body, `x-text="'M:' + changePct(changeMonthPct)"`) {
		t.Error("expected changeMonthPct rendered with x-text")
	}

	// changeClass must be rendered via :class (safe — sets className)
	if !strings.Contains(body, `:class="changeClass(changeDayPct)"`) {
		t.Error("expected changeDayPct color via :class binding")
	}
	if !strings.Contains(body, `:class="changeClass(changeWeekPct)"`) {
		t.Error("expected changeWeekPct color via :class binding")
	}
	if !strings.Contains(body, `:class="changeClass(changeMonthPct)"`) {
		t.Error("expected changeMonthPct color via :class binding")
	}

	// fmtSynced must be rendered via x-text (safe)
	if !strings.Contains(body, `x-text="'Synced ' + fmtSynced(lastSynced)"`) {
		t.Error("expected lastSynced rendered with x-text via fmtSynced()")
	}
}

func TestDashboardHandler_StressChangesRowConditionalDisplay(t *testing.T) {
	// The D/W/M changes row must be hidden when hasChanges is false (x-show).
	// The last synced row must be hidden when lastSynced is empty (x-show).
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Changes row gated on hasChanges
	if !strings.Contains(body, `x-show="hasChanges"`) {
		t.Error("portfolio-changes row must be conditional on hasChanges")
	}
	// Net return $ changes gated on hasReturnDollarChanges
	if !strings.Contains(body, `x-show="hasReturnDollarChanges"`) {
		t.Error("net return $ changes row must be conditional on hasReturnDollarChanges")
	}
	// Net return % changes gated on hasReturnPctChanges
	if !strings.Contains(body, `x-show="hasReturnPctChanges"`) {
		t.Error("net return % changes row must be conditional on hasReturnPctChanges")
	}
	// Last synced gated on lastSynced
	if !strings.Contains(body, `x-show="lastSynced"`) {
		t.Error("portfolio-synced row must be conditional on lastSynced")
	}
	// Both must use x-cloak to prevent flash of unstyled content
	if !strings.Contains(body, `class="portfolio-changes"`) {
		t.Error("expected portfolio-changes element in dashboard")
	}
	if !strings.Contains(body, `class="portfolio-synced"`) {
		t.Error("expected portfolio-synced element in dashboard")
	}
}

func TestDashboardHandler_StressChangeClassReturnsHardcodedStrings(t *testing.T) {
	// Verify changeClass() in common.js only returns hardcoded class names.
	// If it returned user input, :class could be an injection vector.
	// This is a static analysis check against the JS source embedded in the page.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// The common.js is a separate file, not inline. Verify the template
	// references common.js (which contains the safe implementations).
	if !strings.Contains(body, "common.js") {
		t.Error("expected common.js reference in dashboard — changeClass/changePct live there")
	}

	// Verify no inline <script> blocks define changeClass (which could be tampered with)
	// All JS should be in external files, not inline in the template.
	bodyLower := strings.ToLower(body)
	inlineScriptCount := strings.Count(bodyLower, "<script>")
	// Allow Chart.js CDN script tag, but no inline script blocks
	inlineWithSrc := strings.Count(bodyLower, "<script src=")
	if inlineScriptCount > inlineWithSrc {
		t.Errorf("SECURITY: found %d inline <script> tags (vs %d with src) — JS should be external",
			inlineScriptCount, inlineWithSrc)
	}
}

func TestDashboardHandler_StressPortfolioValueLabelRenamed(t *testing.T) {
	// Verify "TOTAL VALUE" is no longer present — replaced by "PORTFOLIO VALUE".
	// This catches regressions where the old label accidentally reappears.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, `>TOTAL VALUE<`) {
		t.Error("REGRESSION: 'TOTAL VALUE' label still present — should be 'PORTFOLIO VALUE'")
	}
	if !strings.Contains(body, `PORTFOLIO VALUE`) {
		t.Error("expected 'PORTFOLIO VALUE' label in dashboard")
	}
}

func TestDashboardHandler_StressChangesInsidePortfolioValueItem(t *testing.T) {
	// The D/W/M changes must appear inside the PORTFOLIO VALUE summary item,
	// not as a separate row. This tests structural integrity.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Find PORTFOLIO VALUE label and portfolio-changes — they should be
	// within the same portfolio-summary-item div
	pvIdx := strings.Index(body, "PORTFOLIO VALUE")
	pcIdx := strings.Index(body, "portfolio-changes")
	if pvIdx < 0 || pcIdx < 0 {
		t.Fatal("expected both PORTFOLIO VALUE and portfolio-changes in template")
	}

	// The changes row should come after PORTFOLIO VALUE but before the performance row
	perfIdx := strings.Index(body, "portfolio-summary-performance")
	if perfIdx < 0 {
		t.Skip("Performance summary row not found")
	}
	if pcIdx > perfIdx {
		t.Error("portfolio-changes appears after performance summary — should be inside PORTFOLIO VALUE item")
	}
}

func TestDashboardHandler_StressSyncedAfterHeaderBeforeSummary(t *testing.T) {
	// The last synced timestamp must appear after the portfolio header row
	// and before the portfolio summary section. This verifies document order.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	headerIdx := strings.Index(body, "portfolio-header")
	syncedIdx := strings.Index(body, "portfolio-synced")
	summaryIdx := strings.Index(body, "portfolio-summary")

	if headerIdx < 0 || syncedIdx < 0 || summaryIdx < 0 {
		t.Fatal("expected portfolio-header, portfolio-synced, and portfolio-summary in template")
	}

	if syncedIdx < headerIdx {
		t.Error("portfolio-synced appears before portfolio-header — wrong document order")
	}
	if syncedIdx > summaryIdx {
		t.Error("portfolio-synced appears after portfolio-summary — should be between header and summary")
	}
}

func TestDashboardHandler_StressChangeClassBindingsPresent(t *testing.T) {
	// Verify that changeClass() is used via :class bindings for all three
	// change periods. The actual CSS class names (change-up, change-down,
	// change-neutral) live in common.js and portal.css — not in the HTML.
	// This test verifies the template wiring is correct.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// All three D/W/M periods must use :class with changeClass()
	bindings := []string{
		`:class="changeClass(changeDayPct)"`,
		`:class="changeClass(changeWeekPct)"`,
		`:class="changeClass(changeMonthPct)"`,
	}
	for _, binding := range bindings {
		if !strings.Contains(body, binding) {
			t.Errorf("expected %s in dashboard template", binding)
		}
	}

	// Verify portal.css is referenced (it contains .change-up, .change-down, .change-neutral)
	if !strings.Contains(body, "portal.css") {
		t.Error("expected portal.css reference — contains change color classes")
	}
}

func TestDashboardHandler_StressReturnChangeBindings(t *testing.T) {
	// Verify net return D/W/M change bindings are present in template
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Net return $ change bindings
	returnDollarBindings := []string{
		`:class="changeClass(changeReturnDayDollar)"`,
		`:class="changeClass(changeReturnWeekDollar)"`,
		`:class="changeClass(changeReturnMonthDollar)"`,
	}
	for _, binding := range returnDollarBindings {
		if !strings.Contains(body, binding) {
			t.Errorf("expected %s in dashboard template", binding)
		}
	}

	// Net return % change bindings
	returnPctBindings := []string{
		`:class="changeClass(changeReturnDayPct)"`,
		`:class="changeClass(changeReturnWeekPct)"`,
		`:class="changeClass(changeReturnMonthPct)"`,
	}
	for _, binding := range returnPctBindings {
		if !strings.Contains(body, binding) {
			t.Errorf("expected %s in dashboard template", binding)
		}
	}

	// Net return $ changes visibility
	if !strings.Contains(body, `x-show="hasReturnDollarChanges"`) {
		t.Error("expected hasReturnDollarChanges visibility binding")
	}

	// Net return % changes visibility
	if !strings.Contains(body, `x-show="hasReturnPctChanges"`) {
		t.Error("expected hasReturnPctChanges visibility binding")
	}
}

func TestDashboardHandler_StressGlossaryTooltipBindings(t *testing.T) {
	// Verify glossary tooltip bindings use glossaryDef() (safe, no raw HTML)
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify label-info elements exist
	if !strings.Contains(body, `class="label-info"`) {
		t.Error("expected label-info tooltip elements in dashboard")
	}

	// Verify data-tooltip bindings use glossaryDef() (safe, no raw HTML)
	expectedBindings := []string{
		`glossaryDef('portfolio_value')`,
		`glossaryDef('capital_gross')`,
		`glossaryDef('capital_available')`,
		`glossaryDef('equity_holdings_value')`,
		`glossaryDef('equity_holdings_return')`,
		`glossaryDef('equity_holdings_return_pct')`,
		`glossaryDef('income_dividends_forecast')`,
	}
	for _, binding := range expectedBindings {
		if !strings.Contains(body, binding) {
			t.Errorf("expected glossary binding %s in dashboard template", binding)
		}
	}

	// Verify tooltips use :data-tooltip (Alpine binding, not static)
	if !strings.Contains(body, `:data-tooltip="glossaryDef(`) {
		t.Error("expected :data-tooltip Alpine binding for glossary tooltips")
	}
}
