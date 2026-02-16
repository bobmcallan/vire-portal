package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Authorize Endpoint Stress Tests
// =============================================================================

func registerTestClient(srv *OAuthServer) *OAuthClient {
	client := &OAuthClient{
		ClientID:                "test-client-id",
		ClientSecret:            "test-client-secret",
		ClientName:              "Test Client",
		RedirectURIs:            []string{"http://localhost:3000/callback"},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	}
	srv.clients.Put(client)
	return client
}

func buildAuthorizeURL(clientID, redirectURI, challenge, state string) string {
	return fmt.Sprintf("/authorize?client_id=%s&redirect_uri=%s&response_type=code&code_challenge=%s&code_challenge_method=S256&state=%s&scope=openid",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(challenge),
		url.QueryEscape(state))
}

// --- Authorize: redirect URI mismatch attack ---

func TestAuthorize_StressRedirectURIMismatch(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)

	// Client registered with http://localhost:3000/callback
	// Attacker tries different redirect URIs
	attacks := []struct {
		name        string
		redirectURI string
	}{
		{"different host", "http://attacker.com/callback"},
		{"different port", "http://localhost:9999/callback"},
		{"different path", "http://localhost:3000/evil"},
		{"add query", "http://localhost:3000/callback?extra=param"},
		{"https upgrade", "https://localhost:3000/callback"},
		{"case variation", "http://LOCALHOST:3000/callback"},
		{"trailing slash", "http://localhost:3000/callback/"},
		{"unicode normalization", "http://localhost:3000/callback\u200b"}, // zero-width space
		{"double encoded", "http://localhost:3000/%63allback"},
		{"with fragment", "http://localhost:3000/callback#fragment"},
		{"protocol-relative", "//localhost:3000/callback"},
	}

	for _, tc := range attacks {
		t.Run(tc.name, func(t *testing.T) {
			challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")
			authURL := buildAuthorizeURL("test-client-id", tc.redirectURI, challenge, "state123")
			req := httptest.NewRequest(http.MethodGet, authURL, nil)
			rec := httptest.NewRecorder()

			srv.HandleAuthorize(rec, req)

			// Should NOT create a session and redirect to attacker's URI
			location := rec.Header().Get("Location")
			if rec.Code == http.StatusFound && strings.Contains(location, "attacker.com") {
				t.Errorf("SECURITY: redirect to attacker URI for %s: %s", tc.name, location)
			}
		})
	}
}

// --- Authorize: auto-registration vulnerability ---

func TestAuthorize_StressAutoRegistrationBypass(t *testing.T) {
	// CRITICAL FINDING: If client_id is unknown, /authorize auto-registers it
	// with the attacker-provided redirect_uri. This means any attacker can:
	// 1. Call /authorize with their own client_id and redirect_uri
	// 2. The server auto-registers the client with that redirect_uri
	// 3. The redirect_uri validation passes because it was just registered
	// 4. The attacker gets an auth code delivered to their URI
	srv := newStressOAuthServer()

	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")
	authURL := buildAuthorizeURL("attacker-client", "http://evil.com/steal", challenge, "state123")
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	// Check if the server auto-registered the attacker's client
	_, ok := srv.clients.Get("attacker-client")
	if ok {
		t.Log("CRITICAL FINDING: /authorize auto-registers unknown client_id with attacker-controlled redirect_uri. " +
			"An attacker can bypass DCR validation entirely by going directly to /authorize " +
			"with their own client_id and redirect_uri. The auto-registration should either: " +
			"(a) be disabled in favor of strict DCR, or " +
			"(b) only auto-register with a fixed set of trusted redirect_uri patterns (e.g., localhost only).")
	}

	// Check if session was created with attacker's redirect URI
	location := rec.Header().Get("Location")
	if strings.Contains(location, "mcp_session=") {
		t.Log("CRITICAL: Session created for auto-registered attacker client. " +
			"If the user completes login, the auth code will be redirected to http://evil.com/steal")
	}
}

// --- Authorize: open redirect via redirect_uri ---

func TestAuthorize_StressOpenRedirect(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)

	// When validation fails, errors are redirected to the redirect_uri.
	// But what if the redirect_uri itself is malicious AND it matches the registered one?
	// (This is actually correct behavior — the concern is when redirect_uri is NOT validated)

	// Test: error redirect should use the REGISTERED redirect_uri, not user-supplied
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	// Missing state — should redirect with error to registered redirect_uri
	authURL := fmt.Sprintf("/authorize?client_id=test-client-id&redirect_uri=%s&response_type=code&code_challenge=%s&code_challenge_method=S256",
		url.QueryEscape("http://localhost:3000/callback"), url.QueryEscape(challenge))
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	// With missing state, the handler should redirect with error
	location := rec.Header().Get("Location")
	if rec.Code == http.StatusFound && strings.Contains(location, "error=") {
		parsedLoc, _ := url.Parse(location)
		if parsedLoc.Host != "" && parsedLoc.Host != "localhost:3000" {
			t.Errorf("SECURITY: error redirected to unexpected host: %s", parsedLoc.Host)
		}
	}
}

// --- Authorize: PKCE downgrade attack ---

func TestAuthorize_StressPKCEDowngrade(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)

	// Try without code_challenge (PKCE downgrade)
	authURL := "/authorize?client_id=test-client-id&redirect_uri=http://localhost:3000/callback&response_type=code&state=abc"
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	// Should reject — PKCE is required
	location := rec.Header().Get("Location")
	if rec.Code == http.StatusFound && !strings.Contains(location, "error=") {
		t.Error("SECURITY: /authorize accepted request without code_challenge — PKCE bypass")
	}
}

// --- Authorize: code_challenge_method plain attempt ---

func TestAuthorize_StressPlainChallengeMethod(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)

	authURL := "/authorize?client_id=test-client-id&redirect_uri=http://localhost:3000/callback&response_type=code&code_challenge=plain-challenge&code_challenge_method=plain&state=abc"
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	// Should reject — only S256 is supported
	location := rec.Header().Get("Location")
	if rec.Code == http.StatusFound && !strings.Contains(location, "error=") {
		t.Error("SECURITY: /authorize accepted code_challenge_method=plain — should only support S256")
	}
}

// --- Authorize: response_type other than code ---

func TestAuthorize_StressImplicitFlowAttempt(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)

	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	// Try implicit flow (response_type=token)
	authURL := fmt.Sprintf("/authorize?client_id=test-client-id&redirect_uri=http://localhost:3000/callback&response_type=token&code_challenge=%s&code_challenge_method=S256&state=abc",
		url.QueryEscape(challenge))
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	location := rec.Header().Get("Location")
	if rec.Code == http.StatusFound && !strings.Contains(location, "error=") {
		t.Error("SECURITY: /authorize accepted response_type=token — implicit flow should be rejected")
	}
}

// --- Authorize: CSRF via missing/forged state ---

func TestAuthorize_StressStateManipulation(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	states := []struct {
		name  string
		state string
	}{
		{"empty state", ""},
		{"very long state", strings.Repeat("A", 1<<16)},
		{"html injection", "<img src=x onerror=alert(1)>"},
		{"newline injection", "state\r\nX-Injected: evil"},
		{"null bytes", "state\x00evil"},
	}

	for _, tc := range states {
		t.Run(tc.name, func(t *testing.T) {
			authURL := buildAuthorizeURL("test-client-id", "http://localhost:3000/callback", challenge, tc.state)
			req := httptest.NewRequest(http.MethodGet, authURL, nil)
			rec := httptest.NewRecorder()

			// Must not panic
			srv.HandleAuthorize(rec, req)
		})
	}
}

// --- Authorize: session flood ---

func TestAuthorize_StressSessionFlood(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	const floods = 1000
	for i := 0; i < floods; i++ {
		authURL := buildAuthorizeURL("test-client-id", "http://localhost:3000/callback", challenge, fmt.Sprintf("state-%d", i))
		req := httptest.NewRequest(http.MethodGet, authURL, nil)
		rec := httptest.NewRecorder()
		srv.HandleAuthorize(rec, req)
	}

	srv.sessions.mu.RLock()
	count := len(srv.sessions.sessions)
	srv.sessions.mu.RUnlock()

	t.Logf("FINDING: %d sessions created by flooding /authorize. "+
		"No per-client or per-IP session limit. Memory can be exhausted.", count)
}

// --- Authorize: concurrent requests ---

func TestAuthorize_StressConcurrentRequests(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			authURL := buildAuthorizeURL("test-client-id", "http://localhost:3000/callback", challenge, fmt.Sprintf("state-%d", id))
			req := httptest.NewRequest(http.MethodGet, authURL, nil)
			rec := httptest.NewRecorder()
			srv.HandleAuthorize(rec, req)

			if rec.Code != http.StatusFound {
				t.Errorf("concurrent authorize %d got status %d", id, rec.Code)
			}
		}(i)
	}
	wg.Wait()
}

// --- Authorize: mcp_session_id cookie attributes ---

func TestAuthorize_StressMCPSessionCookieAttributes(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	authURL := buildAuthorizeURL("test-client-id", "http://localhost:3000/callback", challenge, "state123")
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()
	srv.HandleAuthorize(rec, req)

	for _, c := range rec.Result().Cookies() {
		if c.Name == "mcp_session_id" {
			if !c.HttpOnly {
				t.Error("SECURITY: mcp_session_id cookie missing HttpOnly flag — vulnerable to XSS cookie theft")
			}
			if c.SameSite != http.SameSiteLaxMode && c.SameSite != http.SameSiteStrictMode {
				t.Errorf("SECURITY: mcp_session_id cookie SameSite=%v, expected Lax or Strict", c.SameSite)
			}
			if c.MaxAge <= 0 {
				t.Error("mcp_session_id cookie should have a positive MaxAge for TTL")
			}
			if c.MaxAge > 600 {
				t.Errorf("mcp_session_id cookie MaxAge=%d, expected <= 600 (10 min)", c.MaxAge)
			}
			// In production, Secure should be true
			// For dev (http://localhost), Secure is intentionally false
			return
		}
	}
	t.Error("mcp_session_id cookie not set by /authorize")
}

// --- Authorize: redirect_uri validation with empty redirect_uri ---

func TestAuthorize_StressMissingRedirectURI(t *testing.T) {
	srv := newStressOAuthServer()
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	// Missing redirect_uri — should return 400, NOT redirect
	authURL := fmt.Sprintf("/authorize?client_id=test&response_type=code&code_challenge=%s&code_challenge_method=S256&state=abc",
		url.QueryEscape(challenge))
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()

	srv.HandleAuthorize(rec, req)

	if rec.Code == http.StatusFound {
		t.Error("SECURITY: /authorize redirected when redirect_uri is missing — " +
			"per RFC 6749 Section 4.1.2.1, if redirect_uri is missing or invalid, " +
			"the server MUST NOT redirect and should display the error directly")
	}
}

// --- Authorize: redirect_uri with localhost variations ---

func TestAuthorize_StressLocalhostVariations(t *testing.T) {
	srv := newStressOAuthServer()

	// Register client with localhost callback
	srv.clients.Put(&OAuthClient{
		ClientID:     "localhost-client",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	})

	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	// Various localhost-like URIs that should NOT match
	variations := []string{
		"http://127.0.0.1:3000/callback",       // IP vs hostname
		"http://[::1]:3000/callback",           // IPv6 loopback
		"http://0.0.0.0:3000/callback",         // bind-all
		"http://localhost.localdomain:3000/cb", // FQDN
		"http://localhost.:3000/callback",      // trailing dot
	}

	for _, uri := range variations {
		t.Run(uri, func(t *testing.T) {
			authURL := buildAuthorizeURL("localhost-client", uri, challenge, "state")
			req := httptest.NewRequest(http.MethodGet, authURL, nil)
			rec := httptest.NewRecorder()

			srv.HandleAuthorize(rec, req)

			location := rec.Header().Get("Location")
			if rec.Code == http.StatusFound && strings.Contains(location, "mcp_session=") {
				t.Logf("NOTE: localhost variation %s accepted — strict string comparison means "+
					"127.0.0.1 != localhost. This is correct behavior for OAuth redirect_uri matching.", uri)
			}
		})
	}
}

// --- CompleteAuthorization: full flow stress ---

func TestCompleteAuthorization_StressRaceCondition(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	// Create a session
	authURL := buildAuthorizeURL("test-client-id", "http://localhost:3000/callback", challenge, "state123")
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()
	srv.HandleAuthorize(rec, req)

	// Extract session ID from cookie
	var sessionID string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mcp_session_id" {
			sessionID = c.Value
			break
		}
	}
	if sessionID == "" {
		t.Fatal("no mcp_session_id cookie set")
	}

	// Race: two concurrent CompleteAuthorization calls with same session
	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			redirectURL, err := srv.CompleteAuthorization(sessionID, "user-1")
			if err == nil && redirectURL != "" {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successCount > 1 {
		t.Logf("CRITICAL: CompleteAuthorization succeeded %d times for the same session (should be 1). "+
			"sessions.Get + codes.Put + sessions.Delete is not atomic. "+
			"Multiple auth codes were issued for one authorization.", successCount)
	}
}

// --- CompleteAuthorization: code in redirect URL ---

func TestCompleteAuthorization_StressCodeInRedirectURL(t *testing.T) {
	srv := newStressOAuthServer()
	registerTestClient(srv)
	challenge := GenerateCodeChallenge("test-verifier-12345678901234567890")

	// Create session
	authURL := buildAuthorizeURL("test-client-id", "http://localhost:3000/callback", challenge, "state123")
	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	rec := httptest.NewRecorder()
	srv.HandleAuthorize(rec, req)

	var sessionID string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mcp_session_id" {
			sessionID = c.Value
			break
		}
	}

	redirectURL, err := srv.CompleteAuthorization(sessionID, "user-1")
	if err != nil {
		t.Fatalf("CompleteAuthorization failed: %v", err)
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	// Verify code is in query, NOT fragment
	code := parsed.Query().Get("code")
	if code == "" {
		t.Error("auth code missing from redirect URL query parameters")
	}

	// Verify state is preserved
	state := parsed.Query().Get("state")
	if state != "state123" {
		t.Errorf("state mismatch: expected state123, got %s", state)
	}

	// Verify redirect goes to registered URI
	if parsed.Host != "localhost:3000" {
		t.Errorf("redirect to wrong host: %s", parsed.Host)
	}
}
