package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
)

// --- Session Expiry: TokenExpiry Embedded in Template ---

func TestSessionExpiry_StressTokenExpiryInDashboardHTML(t *testing.T) {
	// SECURITY: Authenticated pages MUST embed TokenExpiry so the client-side
	// timer can auto-logout. Without it, users sit on stale sessions with
	// silent 401 failures.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	token := buildStressSignedJWT("alice", []byte(testJWTSecret))
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "window.__VIRE_TOKEN_EXPIRY__") {
		t.Error("FINDING: dashboard page does not embed __VIRE_TOKEN_EXPIRY__ for authenticated user")
	}
}

func TestSessionExpiry_StressNoTokenExpiryForUnauthenticated(t *testing.T) {
	// Unauthenticated pages must NOT embed TokenExpiry — there is no session
	// to expire. Embedding a zero or absent value is fine as long as the
	// conditional template guard prevents rendering.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Unauthenticated dashboard redirects, so body should be minimal
	body := w.Body.String()
	if strings.Contains(body, "__VIRE_TOKEN_EXPIRY__") {
		t.Error("FINDING: __VIRE_TOKEN_EXPIRY__ present in unauthenticated response — information leak")
	}
}

func TestSessionExpiry_StressExpiredTokenNoExpiry(t *testing.T) {
	// An expired token should cause a redirect, not render a page with
	// TokenExpiry embedded. This prevents the client from seeing stale expiry data.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildExpiredJWT("alice")})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expired token should redirect, got status %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "__VIRE_TOKEN_EXPIRY__") {
		t.Error("FINDING: __VIRE_TOKEN_EXPIRY__ present in expired-token response")
	}
}

// --- Session Expiry: XSS via TokenExpiry Template Injection ---

func TestSessionExpiry_StressXSSViaTokenExpiry(t *testing.T) {
	// SECURITY: TokenExpiry is int64 from claims.Exp. Go's html/template
	// auto-escapes, and int64 can only render as a number. But let's verify
	// the rendered output actually looks like a number, not injectable content.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	token := buildStressSignedJWT("alice", []byte(testJWTSecret))
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// Find the TokenExpiry script tag and verify it contains only a number
	idx := strings.Index(body, "window.__VIRE_TOKEN_EXPIRY__ = ")
	if idx == -1 {
		t.Fatal("__VIRE_TOKEN_EXPIRY__ not found in output")
	}
	// Extract the value between "= " and ";"
	start := idx + len("window.__VIRE_TOKEN_EXPIRY__ = ")
	end := strings.Index(body[start:], ";")
	if end == -1 {
		t.Fatal("missing semicolon after __VIRE_TOKEN_EXPIRY__ value")
	}
	value := strings.TrimSpace(body[start : start+end])

	// Must be a valid unix timestamp (all digits, reasonable range)
	for _, c := range value {
		if c < '0' || c > '9' {
			t.Errorf("SECURITY: TokenExpiry value contains non-digit character %q in %q — potential XSS", string(c), value)
			return
		}
	}

	// Sanity: value should be a future timestamp (within 25 hours)
	var ts int64
	fmt.Sscanf(value, "%d", &ts)
	now := time.Now().Unix()
	if ts < now || ts > now+90000 {
		t.Errorf("TokenExpiry value %d is outside expected range [%d, %d]", ts, now, now+90000)
	}
}

// --- Session Expiry: Client-Side Timer Bypass (Documentation Test) ---

func TestSessionExpiry_StressClientTimerBypassIsHarmless(t *testing.T) {
	// ANALYSIS: An attacker can set window.__VIRE_TOKEN_EXPIRY__ to a far-future
	// value in the browser console to prevent auto-logout. This is fine because:
	// 1. The server validates JWT expiry on every request
	// 2. Expired tokens get 401, so the user can't do anything useful
	// 3. The timer is a UX convenience, not a security control
	//
	// Verify the server-side guard: expired JWT must be rejected.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	// Build a JWT that expired 1 second ago (simulating timer bypass scenario)
	expiredToken := buildSignedJWT(map[string]interface{}{
		"sub": "attacker",
		"iss": "vire-server",
		"exp": time.Now().Add(-1 * time.Second).Unix(),
	}, []byte(testJWTSecret))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: expiredToken})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: expired JWT was not rejected by server (status %d)", w.Code)
	}
}

// --- Session Expiry: Logout Endpoint CSRF ---

func TestSessionExpiry_StressLogoutFetchBypassesCSRF(t *testing.T) {
	// The _doLogout() function calls fetch('/api/auth/logout', {method: 'POST'})
	// without a CSRF token. This works because /api/ routes are exempt from
	// CSRF middleware (they use Bearer tokens). Verify the middleware skips /api/.
	//
	// If this changes, _doLogout would silently fail with 403.

	// Read the common.js and verify the fetch call exists
	js := readStaticFile(t, "common.js")
	if !strings.Contains(js, "fetch('/api/auth/logout', { method: 'POST' })") {
		t.Error("FINDING: _doLogout fetch call not found or changed — may need CSRF token")
	}

	// Verify nav.html logout forms use POST (not GET links)
	nav := readStaticFile(t, "../partials/nav.html")
	if strings.Contains(nav, `href="/api/auth/logout"`) {
		t.Error("SECURITY: logout uses GET link instead of POST form — vulnerable to CSRF via <img> tag")
	}
	if !strings.Contains(nav, `action="/api/auth/logout"`) {
		t.Error("logout form action not found in nav.html")
	}
}

// --- Session Expiry: setInterval Leak ---

func TestSessionExpiry_StressIntervalNotCleared(t *testing.T) {
	// FINDING (low severity): _updateTimeLeft starts a setInterval(update, 1000)
	// but the interval ID is never stored or cleared in destroy().
	// The destroy() method only clears the setTimeout timers.
	// This is a minor resource leak — the interval continues after component
	// destruction. In practice, the component lives for the page lifetime.
	//
	// Verify the pattern exists so this finding is documented.
	js := readStaticFile(t, "common.js")

	if !strings.Contains(js, "setInterval(update, 1000)") {
		t.Skip("setInterval pattern not found — may have been fixed")
	}

	// Check if the interval ID is stored for cleanup
	if !strings.Contains(js, "_interval") {
		t.Log("NOTE: setInterval in _updateTimeLeft is not stored for cleanup in destroy(). " +
			"Low severity — component lives for page lifetime, but technically a resource leak.")
	}
}

// --- Session Expiry: sessionExpiry Component in nav.html ---

func TestSessionExpiry_StressBannerInNav(t *testing.T) {
	// The session expiry banner must be inside nav.html so it appears on all
	// authenticated pages. Verify it uses x-cloak (hidden by default) and
	// x-show="warning" (only visible near expiry).
	nav := readStaticFile(t, "../partials/nav.html")

	if !strings.Contains(nav, `x-data="sessionExpiry()"`) {
		t.Error("FINDING: sessionExpiry() component not found in nav.html")
	}
	if !strings.Contains(nav, `x-show="warning"`) {
		t.Error("FINDING: warning banner missing x-show guard — would always be visible")
	}
	if !strings.Contains(nav, `x-cloak`) {
		t.Error("FINDING: session expiry banner missing x-cloak — may flash on page load")
	}
	if !strings.Contains(nav, `class="session-expiry-banner"`) {
		t.Error("FINDING: session-expiry-banner CSS class not found in nav.html")
	}
}

// --- Session Expiry: Multiple Handlers Consistency ---

func TestSessionExpiry_StressAllHandlersEmbedTokenExpiry(t *testing.T) {
	// Every authenticated handler must pass TokenExpiry to the template.
	// If any handler is missing it, that page won't have auto-logout.
	//
	// We test a representative set of handlers that render HTML pages.
	type handlerTest struct {
		name    string
		handler http.Handler
	}

	userLookup := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Username: userID,
			Email:    "test@test.com",
			Role:     "admin",
		}, nil
	}

	handlers := []handlerTest{
		{"dashboard", NewDashboardHandler(nil, true, []byte(testJWTSecret), userLookup)},
		{"strategy", NewStrategyHandler(nil, true, []byte(testJWTSecret), userLookup)},
		{"cash", NewCashHandler(nil, true, []byte(testJWTSecret), userLookup)},
		{"stock", NewStockHandler(nil, true, []byte(testJWTSecret), userLookup)},
	}

	token := buildStressSignedJWT("alice", []byte(testJWTSecret))

	for _, ht := range handlers {
		t.Run(ht.name, func(t *testing.T) {
			path := "/" + ht.name
			if ht.name == "stock" {
				path = "/stock/AAPL"
			}
			req := httptest.NewRequest("GET", path, nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			ht.handler.ServeHTTP(w, req)

			if w.Code == http.StatusFound || w.Code == http.StatusTemporaryRedirect {
				// Some handlers redirect — that's fine, they don't render HTML
				t.Skipf("handler redirects (status %d), skipping TokenExpiry check", w.Code)
			}

			body := w.Body.String()
			if !strings.Contains(body, "__VIRE_TOKEN_EXPIRY__") {
				t.Errorf("FINDING: %s handler does not embed __VIRE_TOKEN_EXPIRY__ for authenticated user", ht.name)
			}
		})
	}
}

// --- Session Expiry: head.html Conditional Guard ---

func TestSessionExpiry_StressHeadPartialConditionalRendering(t *testing.T) {
	// The head.html partial must conditionally render the TokenExpiry script.
	// If the guard is missing, unauthenticated pages would have an empty/zero value.
	head := readStaticFile(t, "../partials/head.html")

	// Must use Go template conditional
	if !strings.Contains(head, "{{if .TokenExpiry}}") {
		t.Error("FINDING: head.html missing {{if .TokenExpiry}} guard — " +
			"would render empty script tag on unauthenticated pages")
	}
	if !strings.Contains(head, "{{end}}") {
		t.Error("FINDING: head.html missing {{end}} for TokenExpiry conditional")
	}
}

// --- Session Expiry: Concurrent Access (Race Condition) ---

func TestSessionExpiry_StressConcurrentDashboardAccess(t *testing.T) {
	// Multiple rapid page loads should not cause race conditions in the handler.
	// Each request gets its own claims and tokenExpiry variable.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)
	token := buildStressSignedJWT("alice", []byte(testJWTSecret))

	const goroutines = 20
	errs := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/dashboard", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			body := w.Body.String()
			if !strings.Contains(body, "__VIRE_TOKEN_EXPIRY__") {
				errs <- "missing __VIRE_TOKEN_EXPIRY__ in concurrent request"
				return
			}
			errs <- ""
		}()
	}

	for i := 0; i < goroutines; i++ {
		if err := <-errs; err != "" {
			t.Errorf("RACE CONDITION: %s", err)
		}
	}
}

// --- Feature 2: Removed Functions Not Referenced ---

func TestSessionExpiry_StressRemovedFunctionsNotInJS(t *testing.T) {
	// The 4 pass-through functions must be removed from common.js.
	// If any remain, they are dead code.
	js := readStaticFile(t, "common.js")

	removed := []string{
		"holdingTodayChange(",
		"holdingBreadthClass(",
		"holdingBreadthArrow(",
		"holdingDailyPct(",
	}

	for _, fn := range removed {
		if strings.Contains(js, fn) {
			t.Errorf("FINDING: removed function %s still exists in common.js", fn)
		}
	}
}

func TestSessionExpiry_StressRemovedFunctionsNotInTemplates(t *testing.T) {
	// Template references to removed functions must be inlined.
	dashboard := readStaticFile(t, "../dashboard.html")
	mobile := readStaticFile(t, "../mobile.html")

	removedCalls := []string{
		"holdingTodayChange(",
		"holdingBreadthClass(",
		"holdingBreadthArrow(",
		"holdingDailyPct(",
	}

	for _, call := range removedCalls {
		if strings.Contains(dashboard, call) {
			t.Errorf("FINDING: dashboard.html still references removed function %s", call)
		}
		if strings.Contains(mobile, call) {
			t.Errorf("FINDING: mobile.html still references removed function %s", call)
		}
	}
}

func TestSessionExpiry_StressInlinedBreadthExpressions(t *testing.T) {
	// After inlining, dashboard.html and mobile.html should use the direct
	// breadth_status expressions instead of wrapper functions.
	dashboard := readStaticFile(t, "../dashboard.html")
	mobile := readStaticFile(t, "../mobile.html")

	// Check that inlined breadth class expression exists
	expected := "'breadth-' + (h.breadth_status || 'flat')"
	if !strings.Contains(dashboard, expected) {
		t.Error("FINDING: dashboard.html missing inlined breadth class expression")
	}
	if !strings.Contains(mobile, expected) {
		t.Error("FINDING: mobile.html missing inlined breadth class expression")
	}

	// Check inlined arrow expression
	arrowExpr := "h.breadth_status === 'rising'"
	if !strings.Contains(dashboard, arrowExpr) {
		t.Error("FINDING: dashboard.html missing inlined breadth arrow expression")
	}
	if !strings.Contains(mobile, arrowExpr) {
		t.Error("FINDING: mobile.html missing inlined breadth arrow expression")
	}
}

// --- Session Expiry: Token Expiry Value Type Safety ---

func TestSessionExpiry_StressTokenExpiryIsInt64(t *testing.T) {
	// TokenExpiry must be int64 (unix timestamp). If it were a string or float,
	// the template could render unexpected content.
	// Build a JWT with a specific exp value and verify it renders as that number.
	specificExp := time.Now().Add(30 * time.Minute).Unix()
	token := buildSignedJWT(map[string]interface{}{
		"sub": "alice",
		"iss": "vire-server",
		"exp": specificExp,
	}, []byte(testJWTSecret))

	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	expStr := fmt.Sprintf("%d", specificExp)
	idx := strings.Index(body, "window.__VIRE_TOKEN_EXPIRY__")
	if idx == -1 {
		t.Fatal("FINDING: __VIRE_TOKEN_EXPIRY__ not found in output")
	}
	// Extract the region around the assignment and verify the exp value is present
	region := body[idx : idx+80]
	if !strings.Contains(region, expStr) {
		t.Errorf("FINDING: expected TokenExpiry to contain %s, got region: %q", expStr, region)
	}
}

// --- Session Expiry: Logout Redirect Safety ---

func TestSessionExpiry_StressLogoutRedirectsToRoot(t *testing.T) {
	// _doLogout redirects to '/' via window.location.href. The server-side
	// HandleLogout also redirects to '/'. Verify the server behavior.
	handler := &AuthHandler{devMode: true, jwtSecret: []byte(testJWTSecret)}

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}
}

// --- Session Expiry: common.js sessionExpiry Function Integrity ---

func TestSessionExpiry_StressComponentStructure(t *testing.T) {
	// Verify the sessionExpiry() function exists and has the expected structure.
	js := readStaticFile(t, "common.js")

	checks := []struct {
		name    string
		pattern string
	}{
		{"function declaration", "function sessionExpiry()"},
		{"init method", "init()"},
		{"destroy method", "destroy()"},
		{"_doLogout method", "_doLogout()"},
		{"_startCountdown method", "_startCountdown()"},
		{"warning property", "warning: false"},
		{"timeLeft property", "timeLeft: ''"},
		{"logout fetch", "fetch('/api/auth/logout'"},
		{"redirect after logout", "window.location.href = '/'"},
		{"2-minute warning", "remainSec - 120"},
		{"clearTimeout in destroy", "clearTimeout(this._timer)"},
	}

	for _, c := range checks {
		if !strings.Contains(js, c.pattern) {
			t.Errorf("FINDING: sessionExpiry() missing %s (pattern: %q)", c.name, c.pattern)
		}
	}
}

// --- Session Expiry: Zero TokenExpiry Handling ---

func TestSessionExpiry_StressZeroTokenExpiryNotRendered(t *testing.T) {
	// When claims is nil (unauthenticated) or claims.Exp is 0,
	// the template conditional {{if .TokenExpiry}} should NOT render.
	// Go's {{if}} treats 0 as falsy for int64, so this works.
	//
	// This is a template-level test — build a JWT with exp=0 to verify.
	// Note: exp=0 will fail JWT validation (expired), so the handler
	// redirects. Verify no TokenExpiry in the redirect response.
	zeroExpToken := buildJWTWithExp("alice", 0)
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: zeroExpToken})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "__VIRE_TOKEN_EXPIRY__") {
		t.Error("FINDING: __VIRE_TOKEN_EXPIRY__ rendered with zero/invalid exp")
	}
}

// buildJWTWithExp creates an unsigned JWT with a specific exp value for testing.
func buildJWTWithExp(sub string, exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": sub,
		"iss": "vire-dev",
		"exp": exp,
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	return header + "." + payload + "."
}
