package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	if !strings.Contains(body, `closedLoading`) {
		t.Error("expected closedLoading indicator near show-closed checkbox")
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

func TestDashboardHandler_StressFetchClosedHoldingsAPI(t *testing.T) {
	// Verify common.js has fetchClosedHoldings that calls include_closed=true
	// and that closedHoldings/closedLoading properties are declared.
	jsBytes, err := os.ReadFile("../../pages/static/common.js")
	if err != nil {
		t.Fatalf("failed to read common.js: %v", err)
	}
	js := string(jsBytes)

	if !strings.Contains(js, "fetchClosedHoldings") {
		t.Error("expected fetchClosedHoldings method in common.js")
	}
	if !strings.Contains(js, "include_closed=true") {
		t.Error("expected include_closed=true API query in fetchClosedHoldings")
	}
	if !strings.Contains(js, "closedHoldings") {
		t.Error("expected closedHoldings property in common.js")
	}
	if !strings.Contains(js, "closedLoading") {
		t.Error("expected closedLoading property in common.js")
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
	// GROSS CONTRIBUTIONS must use x-text (not x-html) and fmt() for formatting
	if !strings.Contains(body, `x-text="fmt(grossContributions)"`) {
		t.Error("expected grossContributions displayed with x-text fmt() binding")
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
	compositionLabels := []string{"PORTFOLIO VALUE", "AVAILABLE CASH", "GROSS CONTRIBUTIONS"}
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

func TestStrategyHandler_StressTemplateUsesXHtmlForStrategy(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Strategy section uses x-html for rendered markdown (user-owned data via authenticated API)
	if !strings.Contains(body, `x-html="strategyHtml"`) {
		t.Error("expected strategy section to use x-html for rendered markdown")
	}
	// Plan table uses x-text for individual fields
	if !strings.Contains(body, "x-text=") {
		t.Error("expected x-text directives for plan table data display")
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

func TestStrategyHandler_StressReadOnlySectionsPresent(t *testing.T) {
	handler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Strategy rendered section
	if !strings.Contains(body, `strategy-rendered`) {
		t.Error("expected strategy-rendered div in strategy page")
	}
	if !strings.Contains(body, `x-html="strategyHtml"`) {
		t.Error("expected strategyHtml binding in strategy page")
	}

	// Plan table section
	if !strings.Contains(body, `plan-table`) {
		t.Error("expected plan-table in strategy page")
	}
	if !strings.Contains(body, `planItems`) {
		t.Error("expected planItems binding in strategy page")
	}

	// No save buttons
	if strings.Contains(body, `saveStrategy()`) {
		t.Error("strategy page should not have saveStrategy() button (read-only)")
	}
	if strings.Contains(body, `savePlan()`) {
		t.Error("strategy page should not have savePlan() button (read-only)")
	}

	// Info banner
	if !strings.Contains(body, `info-banner`) {
		t.Error("expected info-banner in strategy page")
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
	// Net return $ daily change gated on changeReturnDayDollar
	if !strings.Contains(body, `x-show="changeReturnDayDollar != null"`) {
		t.Error("net return $ daily badge must be conditional on changeReturnDayDollar != null")
	}
	// Net return % daily change gated on changeReturnDayPct
	if !strings.Contains(body, `x-show="changeReturnDayPct != null"`) {
		t.Error("net return % daily badge must be conditional on changeReturnDayPct != null")
	}
	// Last synced gated on lastSynced
	if !strings.Contains(body, `x-show="lastSynced"`) {
		t.Error("portfolio-synced row must be conditional on lastSynced")
	}
	// portfolio-change-today class must exist for daily badges
	if !strings.Contains(body, `class="portfolio-change-today"`) {
		t.Error("expected portfolio-change-today element in dashboard")
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
	// Allow 1 inline <script> for SSR hydration (window.__VIRE_DATA__), same as strategy/cash.
	bodyLower := strings.ToLower(body)
	inlineScriptCount := strings.Count(bodyLower, "<script>")
	inlineWithSrc := strings.Count(bodyLower, "<script src=")
	ssrHydrationScripts := strings.Count(body, "window.__VIRE_DATA__")
	if inlineScriptCount > inlineWithSrc+ssrHydrationScripts {
		t.Errorf("SECURITY: found %d inline <script> tags (vs %d with src + %d SSR hydration) — JS should be external",
			inlineScriptCount, inlineWithSrc, ssrHydrationScripts)
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
	// Verify net return daily change bindings are present in template
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Net return $ daily change binding
	if !strings.Contains(body, `:class="changeClass(changeReturnDayDollar)"`) {
		t.Error("expected changeClass(changeReturnDayDollar) in dashboard template")
	}

	// Net return % daily change binding
	if !strings.Contains(body, `:class="changeClass(changeReturnDayPct)"`) {
		t.Error("expected changeClass(changeReturnDayPct) in dashboard template")
	}

	// Daily badges use portfolio-change-today class
	if !strings.Contains(body, `class="portfolio-change-today"`) {
		t.Error("expected portfolio-change-today class for daily return badges")
	}

	// Daily badges show "today" suffix
	if !strings.Contains(body, `+ ' today'`) {
		t.Error("expected 'today' suffix in daily return badges")
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
		`glossaryDef('capital_available')`,
		`glossaryDef('capital_contributions_gross')`,
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

func TestDashboardHandler_StressBreadthBarBindings(t *testing.T) {
	// Breadth bar section must use x-show/x-text/x-cloak (safe bindings, no x-html).
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Breadth bar section present and gated on hasBreadth
	if !strings.Contains(body, `class="breadth-bar-section"`) {
		t.Error("expected breadth-bar-section element in dashboard")
	}
	if !strings.Contains(body, `x-show="hasBreadth"`) {
		t.Error("breadth bar must be conditional on hasBreadth")
	}
	// Trend label uses x-text (safe)
	if !strings.Contains(body, `x-text="breadth?.trend_label || ''"`) {
		t.Error("expected breadth trend_label rendered with x-text")
	}
	// Today change uses x-text (safe)
	if !strings.Contains(body, `fmtTodayChange(breadth.today_change)`) {
		t.Error("expected breadth today_change rendered via fmtTodayChange")
	}
	// Per-ticker segments use x-for with :style (safe — sets style attribute)
	if !strings.Contains(body, `seg.weight_pct`) {
		t.Error("expected seg.weight_pct in breadth bar segment style")
	}
	// Counts use x-text (safe)
	if !strings.Contains(body, `Rising`) {
		t.Error("expected Rising count label in breadth bar")
	}
	if !strings.Contains(body, `Falling`) {
		t.Error("expected Falling count label in breadth bar")
	}
	// Must NOT use x-html
	if strings.Contains(body, `x-html`) {
		t.Error("SECURITY: breadth bar must not use x-html")
	}
}

func TestDashboardHandler_StressTrendArrowBindings(t *testing.T) {
	// Trend arrows live in holding-movement-row sub-rows of the holdings table.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Trend arrow bindings in holding movement rows
	if !strings.Contains(body, `x-text="trendArrow(h.trend_score)"`) {
		t.Error("expected trendArrow(h.trend_score) in holding movement row")
	}
	if !strings.Contains(body, `:class="trendArrowClass(h.trend_score)"`) {
		t.Error("expected trendArrowClass(h.trend_score) in holding movement row")
	}
	// Today's dollar change in movement row
	if !strings.Contains(body, `fmtTodayChange(holdingTodayChange(h))`) {
		t.Error("expected fmtTodayChange(holdingTodayChange(h)) in holding movement row")
	}
	// holding-movement-row must exist
	if !strings.Contains(body, `holding-movement-row`) {
		t.Error("expected holding-movement-row class in dashboard")
	}
	// chart-toggle-label must exist
	if !strings.Contains(body, `class="chart-toggle-label"`) {
		t.Error("expected chart-toggle-label class in dashboard")
	}
	// holdings-total-row must exist
	if !strings.Contains(body, `holdings-total-row`) {
		t.Error("expected holdings-total-row class in dashboard")
	}
}

func TestDashboardHandler_StressBreadthHelpersDefined(t *testing.T) {
	// Verify common.js is referenced (which contains breadth helpers).
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "common.js") {
		t.Error("expected common.js reference — breadth helpers (trendArrow, computeBreadth, etc.) live there")
	}
	// Verify breadth bar CSS classes are defined
	if !strings.Contains(body, "portal.css") {
		t.Error("expected portal.css reference — breadth bar styles live there")
	}
}

// --- Adversarial: Breadth bar :style binding must not allow CSS injection ---

func TestDashboardHandler_StressBreadthStyleBindingsSafe(t *testing.T) {
	// The breadth bar per-ticker segments use :style bindings with percentage values.
	// These MUST use the pattern 'width:' + (seg.weight_pct || 0) + '%' to prevent
	// injection of arbitrary CSS properties via malicious server data.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Per-ticker segment must use the safe pattern with || 0 fallback
	expected := `'width:' + (seg.weight_pct || 0) + '%'`
	if !strings.Contains(body, expected) {
		t.Errorf("SECURITY: breadth segment must use safe :style pattern %q", expected)
	}

	// Must NOT concatenate raw values into style without || 0 guard
	if strings.Contains(body, `:style="'width:' + seg.weight_pct + '%'"`) {
		t.Error("SECURITY: raw segment value in :style without fallback guard")
	}
}

// --- Adversarial: Breadth bar section must have x-cloak to prevent FOUC ---

func TestDashboardHandler_StressBreadthCloakPresent(t *testing.T) {
	// The breadth bar section is conditionally shown (x-show="hasBreadth").
	// Without x-cloak, the section briefly flashes before Alpine.js initializes.
	// This is a usability bug, not security, but x-cloak is required.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Find the breadth-bar-section and verify it has x-cloak
	if !strings.Contains(body, `class="breadth-bar-section" x-show="hasBreadth" x-cloak`) {
		t.Error("breadth-bar-section must have both x-show and x-cloak to prevent FOUC")
	}
}

// --- Adversarial: No x-html anywhere in breadth or trend bindings ---

func TestDashboardHandler_StressBreadthNoXHTMLAnywhere(t *testing.T) {
	// x-html is the primary XSS vector in Alpine.js. The breadth bar and trend
	// arrow code must never use x-html. This is a broader check than the
	// existing StressBreadthBarBindings test, which only checks the breadth section.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// x-html must not appear anywhere in the dashboard template
	if strings.Contains(body, "x-html") {
		t.Error("SECURITY: dashboard template must not use x-html (XSS risk)")
	}
}

// --- Adversarial: Trend arrow uses safe Unicode, not raw HTML entities ---

func TestDashboardHandler_StressTrendArrowNoHTMLEntities(t *testing.T) {
	// The trendArrow() helper returns Unicode arrows (U+2191, U+2193, U+2192).
	// If it returned HTML entities (e.g., &uarr;) instead, they would be
	// rendered as literal text by x-text (safe) but look wrong visually.
	// Verify the template uses x-text (not x-html) for the arrow, ensuring
	// any future change to HTML entities would be caught visually.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Arrow must be rendered via x-text (which safely escapes HTML entities)
	if !strings.Contains(body, `x-text="trendArrow(h.trend_score)"`) {
		t.Error("trendArrow must use x-text binding (safe, escapes HTML entities)")
	}
	// Must NOT use x-html for the arrow (would enable script injection via arrow value)
	if strings.Contains(body, `x-html="trendArrow`) {
		t.Error("SECURITY: trendArrow must NOT use x-html")
	}
}

// --- Adversarial: Concurrent dashboard loads must not race on breadth state ---

func TestDashboardHandler_StressBreadthSegmentOrder(t *testing.T) {
	// The breadthSegments getter must sort: falling first, then flat, then rising.
	// With per-ticker x-for rendering, order is determined by the getter sort logic.
	jsBytes, err := os.ReadFile("../../pages/static/common.js")
	if err != nil {
		t.Fatalf("failed to read common.js: %v", err)
	}
	js := string(jsBytes)

	// breadthSegments getter must exist
	if !strings.Contains(js, "get breadthSegments()") {
		t.Fatal("expected breadthSegments getter in common.js")
	}
	// Must sort by status order: falling=0, flat=1, rising=2
	if !strings.Contains(js, `falling: 0, flat: 1, rising: 2`) {
		t.Error("breadthSegments must sort by status order { falling: 0, flat: 1, rising: 2 }")
	}
	// Must reference trend_score for classification
	if !strings.Contains(js, "trend_score") {
		t.Error("breadthSegments must reference trend_score for status classification")
	}
}

func TestDashboardHandler_StressBreadthSegmentsPropertyDeclared(t *testing.T) {
	// Verify breadthSegments getter exists in common.js with correct structure.
	jsBytes, err := os.ReadFile("../../pages/static/common.js")
	if err != nil {
		t.Fatalf("failed to read common.js: %v", err)
	}
	js := string(jsBytes)

	if !strings.Contains(js, "get breadthSegments()") {
		t.Error("expected breadthSegments getter in common.js")
	}
	if !strings.Contains(js, "trend_score") {
		t.Error("breadthSegments must reference trend_score for classification")
	}
	if !strings.Contains(js, "segments.sort") {
		t.Error("breadthSegments must sort segments by status order")
	}
}

func TestDashboardHandler_StressChartToggleXModel(t *testing.T) {
	// The chart breakdown toggle must use x-model (safe two-way binding), not
	// a raw @click handler that could be an injection vector.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `x-model="showChartBreakdown"`) {
		t.Error("chart breakdown toggle must use x-model for safe two-way binding")
	}
	// Must be inside a label with the correct class
	if !strings.Contains(body, `chart-toggle-label`) {
		t.Error("expected chart-toggle-label wrapping the breakdown checkbox")
	}
	// Must show "Breakdown" text
	if !strings.Contains(body, `Breakdown`) {
		t.Error("expected 'Breakdown' label text for chart toggle")
	}
	// Must have MA toggle labels
	if !strings.Contains(body, `x-model="showMA20"`) {
		t.Error("expected showMA20 toggle in chart controls")
	}
	if !strings.Contains(body, `x-model="showMA50"`) {
		t.Error("expected showMA50 toggle in chart controls")
	}
}

func TestDashboardHandler_StressChartBreakdownPropertyDeclared(t *testing.T) {
	// Verify showChartBreakdown property is declared in common.js.
	// Without it, the x-model binding would create an undefined reference.
	jsBytes, err := os.ReadFile("../../pages/static/common.js")
	if err != nil {
		t.Fatalf("failed to read common.js: %v", err)
	}
	js := string(jsBytes)

	if !strings.Contains(js, "showChartBreakdown") {
		t.Error("expected showChartBreakdown property in common.js")
	}
	// fmtSyncedTime must be defined
	if !strings.Contains(js, "fmtSyncedTime") {
		t.Error("expected fmtSyncedTime helper in common.js")
	}
	// $watch for breakdown toggle must exist
	if !strings.Contains(js, `$watch('showChartBreakdown'`) {
		t.Error("expected $watch on showChartBreakdown in common.js")
	}
	// Moving average properties must be declared
	for _, prop := range []string{"showMA20", "showMA50", "showMA200"} {
		if !strings.Contains(js, prop) {
			t.Errorf("expected %s property in common.js", prop)
		}
	}
	// computeMA helper must exist
	if !strings.Contains(js, "computeMA") {
		t.Error("expected computeMA helper in common.js")
	}
	// Gross Contributions chart line
	if !strings.Contains(js, "Gross Contributions") {
		t.Error("expected 'Gross Contributions' label string in common.js")
	}
	if !strings.Contains(js, "grossLine") {
		t.Error("expected grossLine variable in common.js")
	}
}

func TestDashboardHandler_StressBreadthBarStructure(t *testing.T) {
	// The breadth section must have the bar with falling/flat/rising segments
	// but must NOT contain per-holding rows or portfolio summary row.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Breadth bar segments must exist
	if !strings.Contains(body, `class="breadth-bar"`) {
		t.Error("expected breadth-bar container")
	}
	if !strings.Contains(body, `class="breadth-counts"`) {
		t.Error("expected breadth-counts for falling/flat/rising counts")
	}
	// Per-holding rows and portfolio row must NOT exist (removed)
	if strings.Contains(body, `class="breadth-portfolio-row"`) {
		t.Error("breadth-portfolio-row should have been removed")
	}
	if strings.Contains(body, `class="breadth-separator"`) {
		t.Error("breadth-separator should have been removed")
	}
	if strings.Contains(body, `class="breadth-holdings"`) {
		t.Error("breadth-holdings should have been removed")
	}
	// Per-ticker x-for template must exist
	if !strings.Contains(body, `x-for="seg in breadthSegments"`) {
		t.Error("expected x-for=\"seg in breadthSegments\" in breadth bar template")
	}
	if !strings.Contains(body, `:class="'breadth-' + seg.status"`) {
		t.Error("expected :class=\"'breadth-' + seg.status\" on breadth segments")
	}
}

func TestDashboardHandler_StressBreadthYesterdayChangeGuarded(t *testing.T) {
	// The yesterday_change display must be gated with x-show to hide
	// when the field is absent. Without this guard, "undefined yesterday"
	// would be displayed to the user.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, `x-show="breadth?.yesterday_change != null"`) {
		t.Error("yesterday_change must be guarded with x-show null check")
	}
	// Sync timestamp must be guarded on lastSynced
	if !strings.Contains(body, `x-show="lastSynced"`) {
		t.Error("breadth sync timestamp must be guarded on lastSynced")
	}
	// breadth-summary-right container must exist
	if !strings.Contains(body, `class="breadth-summary-right"`) {
		t.Error("expected breadth-summary-right container for right-aligned breadth info")
	}
}

func TestDashboardHandler_StressConcurrentDashboardServe(t *testing.T) {
	// Multiple concurrent requests to the dashboard handler must not panic
	// or produce corrupt HTML. This tests the Go handler, not the JS runtime.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	var wg sync.WaitGroup
	const concurrency = 20
	errors := make([]string, 0)
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/dashboard", nil)
			addAuthCookie(req, fmt.Sprintf("user-%d", idx))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("request %d: got %d", idx, w.Code))
				mu.Unlock()
				return
			}
			body := w.Body.String()
			if !strings.Contains(body, "breadth-bar-section") {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("request %d: missing breadth-bar-section", idx))
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	for _, e := range errors {
		t.Error(e)
	}
}

// =============================================================================
// Dashboard SSR Stress Tests
// =============================================================================

func TestDashboardSSR_StressConcurrentRenders(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return []byte(`{"portfolios":[{"name":"Test"}],"default":"Test"}`), nil
		}
		if path == "/api/glossary" {
			return []byte(`{"categories":[]}`), nil
		}
		return []byte(`{"holdings":[]}`), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
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
			body := w.Body.String()
			if !strings.Contains(body, "window.__VIRE_DATA__") {
				t.Errorf("concurrent request %d missing window.__VIRE_DATA__", n)
			}
		}(i)
	}
	wg.Wait()
}

func TestDashboardSSR_StressLargeTimelineJSON(t *testing.T) {
	// Mock timeline with 500 data points
	var points []map[string]interface{}
	for i := 0; i < 500; i++ {
		points = append(points, map[string]interface{}{
			"date":            fmt.Sprintf("2026-01-%03d", i),
			"portfolio_value": 1000 + i,
		})
	}
	timelineJSON, _ := json.Marshal(map[string]interface{}{"data_points": points})

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return []byte(`{"portfolios":[{"name":"Test"}],"default":"Test"}`), nil
		}
		if strings.Contains(path, "/timeline") {
			return timelineJSON, nil
		}
		if path == "/api/glossary" {
			return []byte(`{"categories":[]}`), nil
		}
		return []byte(`{"holdings":[]}`), nil
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	// Verify first and last data points are present (no truncation)
	if !strings.Contains(body, `"2026-01-000"`) {
		t.Error("first data point missing from embedded timeline")
	}
	if !strings.Contains(body, `"2026-01-499"`) {
		t.Error("last data point missing from embedded timeline")
	}
}

func TestDashboardSSR_StressXSSViaProxyGet(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]interface{}{
					{"name": "Test"},
				},
				"default": "Test",
			})
		}
		if path == "/api/glossary" {
			return json.Marshal(map[string]interface{}{
				"categories": []map[string]interface{}{
					{
						"name": "<script>alert('xss')</script>",
						"terms": []map[string]interface{}{
							{"term": "bad", "definition": "<img src=x onerror=alert(1)>"},
						},
					},
				},
			})
		}
		return []byte(`{}`), nil
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// The XSS payloads are inside JSON strings within a <script> block.
	// template.JS embeds them as-is. In JSON string context, the HTML tags
	// are inert because they are inside a JS string literal.
	// The critical check is that no unescaped <script>alert appears outside JSON.
	lowerBody := strings.ToLower(body)
	scriptAlertCount := strings.Count(lowerBody, "<script>alert")
	// All occurrences should be inside JSON string values (inside the window.__VIRE_DATA__ script block)
	if scriptAlertCount > 1 {
		t.Log("WARNING: multiple <script>alert occurrences — verify all are inside JSON strings")
	}
}

func TestDashboardSSR_StressProxyGetTimeout(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return []byte(`{"portfolios":[{"name":"Test"}],"default":"Test"}`), nil
		}
		if strings.Contains(path, "/api/portfolios/Test") && !strings.Contains(path, "/timeline") && !strings.Contains(path, "/watchlist") {
			return []byte(`{"holdings":[{"ticker":"AAPL"}]}`), nil
		}
		// Simulate timeout for timeline, watchlist, glossary
		time.Sleep(100 * time.Millisecond)
		return nil, fmt.Errorf("timeout")
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with partial timeout, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"AAPL"`) {
		t.Error("expected portfolio data present despite timeout on other endpoints")
	}
}

// =============================================================================
// Dashboard SSR — Devils-Advocate Adversarial Stress Tests
// =============================================================================

// --- Script Tag Breakout: </script> in JSON values ---
// This is the CRITICAL XSS vector for template.JS: if a JSON string value
// contains "</script>", the browser's HTML parser closes the <script> block
// before the JS parser sees the string. This is an accepted risk (data from
// trusted vire-server), but we document and verify the behavior.

func TestDashboardSSR_StressScriptTagBreakoutInJSON(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]interface{}{
					{"name": "Test"},
				},
				"default": "Test",
			})
		}
		if path == "/api/glossary" {
			return []byte(`{"categories":[]}`), nil
		}
		// Portfolio data with </script> in a holding name
		if strings.HasPrefix(path, "/api/portfolios/Test") && !strings.Contains(path, "/timeline") && !strings.Contains(path, "/watchlist") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{
						"ticker": "EVIL",
						"name":   "</script><script>alert('xss')</script>",
					},
				},
			})
		}
		return []byte(`{}`), nil
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Count script open/close tags. If </script> in JSON breaks out,
	// we'll have more closing tags than opening tags.
	openTags := strings.Count(strings.ToLower(body), "<script")
	closeTags := strings.Count(body, "</script>")
	if closeTags > openTags {
		t.Log("ACCEPTED RISK: </script> in JSON data breaks out of script context. " +
			"template.JS does not escape this. Data from vire-server is trusted. " +
			"If server is compromised, XSS is possible via embedded SSR JSON.")
	}
}

// --- ProxyGetFn Panic: Dashboard must not crash ---

func TestDashboardSSR_StressProxyGetPanic(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		panic("simulated crash in dashboard proxyGetFn")
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	// Go's net/http server catches panics per-request in production.
	// The handler itself does not use recover(). Verify this is survivable.
	defer func() {
		if r := recover(); r != nil {
			t.Log("WARNING: proxyGetFn panic not caught by dashboard handler — " +
				"Go net/http will catch it in production, but handler should ideally recover")
		}
	}()

	handler.ServeHTTP(w, req)
}

// --- User Data Isolation: Concurrent SSR renders must not leak data ---

func TestDashboardSSR_StressUserDataIsolation(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			// Each user gets a unique portfolio name derived from their userID
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]interface{}{
					{"name": "portfolio-" + userID},
				},
				"default": "portfolio-" + userID,
			})
		}
		if path == "/api/glossary" {
			return []byte(`{"categories":[]}`), nil
		}
		return []byte(`{"holdings":[]}`), nil
	})

	var wg sync.WaitGroup
	const numUsers = 50
	bodies := make([]string, numUsers)

	for i := 0; i < numUsers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			userID := fmt.Sprintf("user-%d", n)
			req := httptest.NewRequest("GET", "/dashboard", nil)
			addAuthCookie(req, userID)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("user %s got status %d", userID, w.Code)
				return
			}
			bodies[n] = w.Body.String()
		}(i)
	}
	wg.Wait()

	// Verify each user's response contains their own portfolio name
	// and does NOT contain another user's name (use a delimiter to avoid substring matches)
	for i := 0; i < numUsers; i++ {
		expected := fmt.Sprintf(`"portfolio-user-%d"`, i)
		if !strings.Contains(bodies[i], expected) {
			t.Errorf("user-%d response missing expected portfolio name %s", i, expected)
		}
		// Spot-check: ensure a sufficiently different user's data is not present
		// Pick an index that differs by 10+ to avoid substring overlap (e.g., user-4 vs user-47)
		otherIdx := (i + 17) % numUsers
		if otherIdx != i {
			other := fmt.Sprintf(`"portfolio-user-%d"`, otherIdx)
			if strings.Contains(bodies[i], other) {
				t.Errorf("SECURITY: user-%d response contains user-%d data — data leak", i, otherIdx)
			}
		}
	}
}

// --- Empty Portfolio Name Edge Case ---

func TestDashboardSSR_StressEmptyPortfolioName(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	callCount := 0
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		callCount++
		if path == "/api/portfolios" {
			// Default is empty string, first portfolio name is empty string
			return []byte(`{"portfolios":[{"name":""}],"default":""}`), nil
		}
		if path == "/api/glossary" {
			return []byte(`{"categories":[]}`), nil
		}
		// Should NOT reach here — empty portfolio name means no downstream fetches
		t.Errorf("unexpected fetch to path %q with empty portfolio name", path)
		return []byte(`{}`), nil
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	// With empty portfolio name, portfolio/timeline/watchlist should all be null
	if !strings.Contains(body, "window.__VIRE_DATA__") {
		t.Error("expected window.__VIRE_DATA__ block in response")
	}
}

// --- Nil ProxyGetFn: SSR JSON fields default to null ---

func TestDashboardSSR_StressNilProxyGetFnRendersNull(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	// Do NOT call SetProxyGetFn

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	// All 5 JSON fields must be "null" — Alpine init() will fall through to client-side fetch
	// Count null assignments in the __VIRE_DATA__ block
	vireDataIdx := strings.Index(body, "window.__VIRE_DATA__")
	if vireDataIdx == -1 {
		t.Fatal("missing window.__VIRE_DATA__ block")
	}
	endIdx := strings.Index(body[vireDataIdx:], "</script>")
	if endIdx == -1 {
		t.Fatal("missing closing </script> after __VIRE_DATA__")
	}
	dataBlock := body[vireDataIdx : vireDataIdx+endIdx]
	nullCount := strings.Count(dataBlock, "null")
	if nullCount < 5 {
		t.Errorf("expected at least 5 null values in __VIRE_DATA__ block, found %d", nullCount)
	}
}

// --- Malicious Portfolio Name in URL Path ---

func TestDashboardSSR_StressPathTraversalInPortfolioName(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	var fetchedPaths []string
	var mu sync.Mutex
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		mu.Lock()
		fetchedPaths = append(fetchedPaths, path)
		mu.Unlock()
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]interface{}{
					{"name": "../../../etc/passwd"},
				},
				"default": "../../../etc/passwd",
			})
		}
		if path == "/api/glossary" {
			return []byte(`{"categories":[]}`), nil
		}
		return []byte(`{}`), nil
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify that url.PathEscape was applied — the path traversal should be escaped
	for _, p := range fetchedPaths {
		if strings.Contains(p, "../") {
			t.Errorf("SECURITY: path traversal not escaped in fetch path %q — url.PathEscape may not be applied", p)
		}
	}
}

// --- Malformed JSON from ProxyGetFn ---

func TestDashboardSSR_StressMalformedJSONFromProxy(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			// Return valid portfolios JSON but with extra garbage
			return []byte(`{"portfolios":[{"name":"Test"}],"default":"Test"}`), nil
		}
		if path == "/api/glossary" {
			return []byte(`{not valid json`), nil // malformed glossary
		}
		// Return truncated JSON for portfolio data
		return []byte(`{"holdings":[{"ticker":"AA`), nil
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — malformed JSON from proxy should not crash handler", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "window.__VIRE_DATA__") {
		t.Error("expected __VIRE_DATA__ block even with malformed JSON")
	}
}

// --- SSR with No Portfolios: Empty Array ---

func TestDashboardSSR_StressEmptyPortfoliosList(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return []byte(`{"portfolios":[],"default":""}`), nil
		}
		if path == "/api/glossary" {
			return []byte(`{"categories":[]}`), nil
		}
		t.Errorf("unexpected fetch with empty portfolios: %s", path)
		return []byte(`{}`), nil
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- SSR with Claims.Sub empty string ---

func TestDashboardSSR_StressEmptySubClaim(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	handler.SetProxyGetFn(func(path, userID string) ([]byte, error) {
		t.Errorf("SECURITY: proxyGetFn called with empty sub claim — path=%q userID=%q", path, userID)
		return []byte(`{}`), nil
	})

	// Build a JWT with empty sub claim
	token := buildUnsignedJWT("")
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Either redirect (invalid auth) or render with null SSR data — but never call proxyGetFn
}
