package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// buildStressSignedJWT creates an HMAC-SHA256 signed JWT for stress testing.
func buildStressSignedJWT(sub string, secret []byte) string {
	return buildSignedJWT(map[string]interface{}{
		"sub": sub,
		"iss": "vire-server",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}, secret)
}

func buildExpiredJWT(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": sub,
		"iss": "vire-dev",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	return header + "." + payload + "."
}

// --- JWT Security: alg:none Attack ---

func TestValidateJWT_AlgNoneAttack_WithSecret(t *testing.T) {
	// CRITICAL: alg:none attack — craft a token with alg:none header and empty
	// signature, attempt to bypass signature verification when a secret IS configured.
	// If this passes, an attacker can forge arbitrary identities.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "admin",
		"iss": "vire-server",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	algNoneToken := header + "." + payload + "."

	secret := []byte("real-secret-key-32-bytes-long!!!")
	_, err := ValidateJWT(algNoneToken, secret)
	if err == nil {
		t.Fatal("SECURITY: alg:none token accepted when JWT secret is configured — signature bypass vulnerability")
	}
}

// --- JWT Security: Tampered Payload ---

func TestValidateJWT_StressTamperedPayload_EscalatePrivilege(t *testing.T) {
	// Sign a token as "alice", tamper to become "admin" without re-signing.
	secret := []byte("test-secret-key-32-bytes-long!!!")
	token := buildStressSignedJWT("alice", secret)

	parts := strings.SplitN(token, ".", 3)
	tamperedClaims, _ := json.Marshal(map[string]interface{}{
		"sub": "admin",
		"iss": "vire-server",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	parts[1] = base64.RawURLEncoding.EncodeToString(tamperedClaims)
	tamperedToken := strings.Join(parts, ".")

	_, err := ValidateJWT(tamperedToken, secret)
	if err == nil {
		t.Fatal("SECURITY: tampered JWT payload accepted — privilege escalation possible")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected signature error, got: %v", err)
	}
}

// --- JWT Security: Timing Attack on HMAC Comparison ---

func TestValidateJWT_StressTimingAttack_ConstantTimeCompare(t *testing.T) {
	// Verify signature comparison uses hmac.Equal (constant-time).
	// Both a completely wrong sig and a partially-right sig should produce
	// the same error message (no information leakage).
	secret := []byte("test-secret-key-32-bytes-long!!!")
	token := buildStressSignedJWT("alice", secret)
	parts := strings.SplitN(token, ".", 3)

	// Completely wrong signature
	wrongSig1 := base64.RawURLEncoding.EncodeToString([]byte("completely-wrong-signature-value"))
	token1 := parts[0] + "." + parts[1] + "." + wrongSig1

	// Almost-right signature (first byte correct, rest wrong)
	realSig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	almostRight := make([]byte, len(realSig))
	almostRight[0] = realSig[0]
	for i := 1; i < len(almostRight); i++ {
		almostRight[i] = ^realSig[i]
	}
	wrongSig2 := base64.RawURLEncoding.EncodeToString(almostRight)
	token2 := parts[0] + "." + parts[1] + "." + wrongSig2

	_, err1 := ValidateJWT(token1, secret)
	_, err2 := ValidateJWT(token2, secret)

	if err1 == nil || err2 == nil {
		t.Fatal("SECURITY: wrong signatures were accepted")
	}
	// Both should produce identical error messages (no info leak)
	if err1.Error() != err2.Error() {
		t.Errorf("different error messages for different wrong signatures may leak info: %q vs %q", err1.Error(), err2.Error())
	}
}

// --- JWT Security: Missing/Zero Exp ---

func TestValidateJWT_StressMissingExp(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "alice",
		"iss": "vire-dev",
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	_, err := ValidateJWT(token, []byte{})
	if err == nil {
		t.Fatal("JWT without exp claim accepted — tokens would never expire")
	}
}

func TestValidateJWT_StressZeroExp(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "alice",
		"exp": 0,
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	_, err := ValidateJWT(token, []byte{})
	if err == nil {
		t.Fatal("JWT with exp=0 accepted — should be rejected as missing/expired")
	}
}

func TestValidateJWT_StressNegativeExp(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "alice",
		"exp": -1,
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	_, err := ValidateJWT(token, []byte{})
	if err == nil {
		t.Fatal("JWT with negative exp accepted")
	}
}

// --- JWT Security: Malformed Tokens ---

func TestValidateJWT_StressMalformedTokens(t *testing.T) {
	secret := []byte("test-secret")

	malformed := []struct {
		name  string
		token string
	}{
		{"spaces", "a b.c d.e f"},
		{"null bytes", "a\x00b.c\x00d.e"},
		{"newlines", "a\nb.c\nd.e"},
		{"tabs", "a\tb.c\td.e"},
		{"unicode", "\u200b.\u200b.\u200b"},
		{"very long", strings.Repeat("A", 100000) + "." + strings.Repeat("B", 100000) + "." + strings.Repeat("C", 100000)},
		{"empty parts", ".."},
		{"json in header part", `{"alg":"none"}.` + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"x","exp":99999999999}`)) + "."},
	}

	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic
			_, _ = ValidateJWT(tc.token, secret)
		})
	}
}

// --- JWT Security: Empty Secret Accepts Any Token ---

func TestValidateJWT_StressEmptySecret_AcceptsForgedToken(t *testing.T) {
	// WARNING: When jwt_secret is empty (the default), ANY token with valid
	// structure and non-expired exp is accepted. This is by design for backwards
	// compat during migration, but it's a security risk if deployed with empty secret.
	forgedToken := buildTestJWT("admin_impersonator")

	claims, err := ValidateJWT(forgedToken, []byte{})
	if err != nil {
		t.Fatalf("expected empty secret to accept forged token (by design): %v", err)
	}
	if claims.Sub != "admin_impersonator" {
		t.Errorf("expected forged sub, got %q", claims.Sub)
	}
	// This is expected behavior but should be documented as a risk.
	t.Log("NOTE: Empty jwt_secret accepts any well-formed JWT. Ensure jwt_secret is set in production.")
}

// --- IsLoggedIn Stress Tests ---

func TestIsLoggedIn_StressEmptyCookieValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: ""})
	loggedIn, _ := IsLoggedIn(req, []byte{})
	if loggedIn {
		t.Error("expected not logged in with empty cookie value")
	}
}

func TestIsLoggedIn_StressExpiredToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: buildExpiredJWT("alice")})
	loggedIn, _ := IsLoggedIn(req, []byte{})
	if loggedIn {
		t.Error("expected not logged in with expired token")
	}
}

func TestIsLoggedIn_StressWrongSecretRejectsToken(t *testing.T) {
	secret := []byte("correct-secret-key-32-bytes!!!!!")
	wrongSecret := []byte("wrong-secret-key-32-bytes-long!!")
	token := buildStressSignedJWT("alice", secret)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
	loggedIn, _ := IsLoggedIn(req, wrongSecret)
	if loggedIn {
		t.Error("SECURITY: IsLoggedIn accepted token signed with wrong secret")
	}
}

func TestIsLoggedIn_StressConcurrentValidation(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	token := buildStressSignedJWT("alice", secret)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			loggedIn, claims := IsLoggedIn(req, secret)
			if !loggedIn || claims == nil || claims.Sub != "alice" {
				t.Error("concurrent IsLoggedIn failed")
			}
		}()
	}
	wg.Wait()
}

// --- OAuth Callback Security Tests ---

func TestOAuthCallback_StressMissingTokenNoCookie(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/auth/callback", nil)
	w := httptest.NewRecorder()

	handler.HandleOAuthCallback(w, req)

	// Must NOT set any session cookie on failure
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			t.Error("SECURITY: session cookie set despite missing token")
		}
	}
}

func TestOAuthCallback_StressHostileTokenValues(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte{})

	hostileTokens := []struct {
		name  string
		token string
	}{
		{"script tag", "<script>alert(1)</script>"},
		{"very long", strings.Repeat("A", 100000)},
		{"sql injection", "'; DROP TABLE sessions; --"},
		{"unicode zero width", "\u200b\u200b\u200b.\u200b.\u200b"},
		{"url encoded XSS", "%3Cscript%3Ealert(1)%3C%2Fscript%3E"},
	}
	// Note: CRLF injection (\r\n) and null bytes (\x00) are rejected by Go's
	// net/http at the transport level, so they can't reach our handler in practice.

	for _, tc := range hostileTokens {
		t.Run(tc.name, func(t *testing.T) {
			// URL-encode the token to construct a valid HTTP request URL
			encodedURL := "/auth/callback?token=" + url.QueryEscape(tc.token)
			req := httptest.NewRequest("GET", encodedURL, nil)
			w := httptest.NewRecorder()

			// Must not panic
			handler.HandleOAuthCallback(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("expected 302, got %d", w.Code)
			}
		})
	}
}

func TestOAuthCallback_StressOpenRedirectProtection(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte{})

	paths := []string{
		"/auth/callback?token=x&redirect=https://evil.com",
		"/auth/callback?token=x&next=https://evil.com",
		"/auth/callback?token=x&return_to=//evil.com",
		"/auth/callback?token=x&redirect_uri=https://evil.com/steal",
	}

	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.HandleOAuthCallback(w, req)

		location := w.Header().Get("Location")
		if location != "/dashboard" {
			t.Errorf("SECURITY: callback redirect influenced by query params: path=%s, location=%s", path, location)
		}
	}
}

func TestOAuthCallback_StressTokenInFragment(t *testing.T) {
	// Fragment (#) not sent to server — verify handler reads from query only.
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/auth/callback#token=evil", nil)
	w := httptest.NewRecorder()
	handler.HandleOAuthCallback(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_failed") {
		t.Errorf("expected missing_token error when token only in fragment, got location=%s", location)
	}
}

func TestOAuthCallback_StressConcurrentRequests(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte{})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			token := buildTestJWT(fmt.Sprintf("user-%d", n))
			req := httptest.NewRequest("GET", "/auth/callback?token="+token, nil)
			w := httptest.NewRecorder()

			handler.HandleOAuthCallback(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("concurrent callback %d got status %d", n, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

// --- OAuth Login Redirect Security ---

func TestGoogleLogin_StressOpenRedirectProtection(t *testing.T) {
	// Mock vire-server that returns a fixed Google redirect
	googleURL := "https://accounts.google.com/o/oauth2/auth?client_id=test"
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, googleURL, http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	// Query parameters on the request path should not influence the redirect target
	paths := []string{
		"/api/auth/login/google?redirect=https://evil.com",
		"/api/auth/login/google?callback=https://evil.com",
		"/api/auth/login/google?return_to=https://evil.com",
	}

	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.HandleGoogleLogin(w, req)

		location := w.Header().Get("Location")
		if location != googleURL {
			t.Errorf("SECURITY: Google login redirect influenced: path=%s, location=%s", path, location)
		}
		if strings.Contains(location, "evil.com") {
			t.Errorf("SECURITY: evil.com in redirect URL: %s", location)
		}
	}
}

func TestGitHubLogin_StressOpenRedirectProtection(t *testing.T) {
	// Mock vire-server that returns a fixed GitHub redirect
	githubURL := "https://github.com/login/oauth/authorize?client_id=test"
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, githubURL, http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	paths := []string{
		"/api/auth/login/github?redirect=https://evil.com",
		"/api/auth/login/github?callback=https://evil.com",
	}

	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.HandleGitHubLogin(w, req)

		location := w.Header().Get("Location")
		if strings.Contains(location, "evil.com") {
			t.Errorf("SECURITY: evil.com in GitHub redirect URL: %s", location)
		}
	}
}

func TestOAuthLogin_StressCallbackURLProperlyEncoded(t *testing.T) {
	// The callback URL is now url.QueryEscape'd in the proxy request to vire-server.
	// Verify that complex callback URLs are properly encoded.
	callbackURL := "http://localhost:8500/auth/callback?extra=param&another=val"
	expectedEncoded := url.QueryEscape(callbackURL)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cb := r.URL.Query().Get("callback")
		if cb != callbackURL {
			t.Errorf("expected callback=%s, got %s", callbackURL, cb)
		}
		// Verify the raw query has the callback properly encoded
		if !strings.Contains(r.URL.RawQuery, "callback="+expectedEncoded) {
			t.Errorf("callback not properly encoded in raw query: %s", r.URL.RawQuery)
		}
		http.Redirect(w, r, "https://accounts.google.com/o/oauth2/auth", http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, callbackURL, []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
}

// --- OAuth Proxy Security: Hostile Location Headers ---

func TestOAuthProxy_StressHostileLocationHeaders(t *testing.T) {
	// CRITICAL: The proxy forwards the Location header from vire-server to the browser.
	// If vire-server is compromised or returns hostile redirects, the browser follows them.
	// These tests document the trust boundary: vire-server Location is forwarded as-is.
	hostileLocations := []struct {
		name     string
		location string
	}{
		{"javascript URI", "javascript:alert(document.cookie)"},
		{"data URI HTML", "data:text/html,<script>alert(1)</script>"},
		{"data URI base64", "data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg=="},
		{"protocol-relative evil", "//evil.com/steal?cookies="},
		{"plain evil.com", "https://evil.com/phishing"},
		{"backslash URL", "https://evil.com\\@legit.com"},
		{"at-sign confusion", "https://evil.com%40legit.com"},
		{"unicode homograph", "https://\u0435vil.com"}, // Cyrillic 'е'
	}

	for _, tc := range hostileLocations {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", tc.location)
				w.WriteHeader(http.StatusFound)
			}))
			defer mockServer.Close()

			handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

			req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
			w := httptest.NewRecorder()

			// Must not panic
			handler.HandleGoogleLogin(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("expected 302, got %d", w.Code)
			}

			// Document: the proxy forwards Location as-is from vire-server.
			// This is acceptable because vire-server is trusted, but should be
			// documented as a trust boundary assumption.
			location := w.Header().Get("Location")
			t.Logf("Forwarded hostile Location %q -> %q", tc.location, location)
		})
	}
}

func TestOAuthProxy_StressMultipleLocationHeaders(t *testing.T) {
	// What if vire-server returns multiple Location headers?
	// Go's resp.Header.Get("Location") returns the first value only.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Location", "https://accounts.google.com/legit")
		w.Header().Add("Location", "https://evil.com/phishing")
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	location := w.Header().Get("Location")
	// Go's Header.Get returns the first value, so the second hostile one is ignored
	if strings.Contains(location, "evil.com") {
		t.Errorf("SECURITY: second Location header (evil.com) was used: %s", location)
	}
}

func TestOAuthProxy_StressServerHangs(t *testing.T) {
	// Vire-server accepts connection but responds very slowly.
	// The 10s timeout should prevent the portal from hanging indefinitely.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep slightly longer than the 10s client timeout.
		// Use context to detect client disconnect and return early.
		select {
		case <-r.Context().Done():
			return
		case <-time.After(12 * time.Second):
			return
		}
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	handler.HandleGoogleLogin(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_unavailable") {
		t.Errorf("expected auth_unavailable error redirect, got %s", location)
	}
	if elapsed > 12*time.Second {
		t.Errorf("timeout took too long: %v (expected ~10s)", elapsed)
	}
}

func TestOAuthProxy_StressServerReturns200NoLocation(t *testing.T) {
	// Vire-server returns 200 OK instead of a redirect -- no Location header.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("unexpected response"))
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_failed") {
		t.Errorf("expected auth_failed error redirect when no Location header, got %s", location)
	}
}

func TestOAuthProxy_StressServerReturns500WithLocation(t *testing.T) {
	// Vire-server returns 500 with a Location header.
	// FINDING: The proxy only checks for empty Location, not status code.
	// A 500 with Location is unusual but the proxy still forwards it.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://accounts.google.com/o/oauth2/auth")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	location := w.Header().Get("Location")
	// The proxy forwards Location regardless of HTTP status from vire-server.
	// This is acceptable because vire-server is trusted.
	t.Logf("FINDING: Location from 500 response forwarded: %s", location)
}

func TestOAuthProxy_StressServerUnreachable(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://127.0.0.1:1", "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_unavailable") {
		t.Errorf("expected auth_unavailable error, got %s", location)
	}
}

func TestOAuthProxy_StressErrorDoesNotLeakInternalAddresses(t *testing.T) {
	// CRITICAL: Error messages/redirects must NOT expose the internal API URL.
	// The whole point of the proxy is to hide Docker-internal addresses.
	internalURL := "http://server:8080"
	handler := NewAuthHandler(nil, false, internalURL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	location := w.Header().Get("Location")
	if strings.Contains(location, "server:8080") {
		t.Errorf("SECURITY: internal Docker address leaked in redirect: %s", location)
	}

	body := w.Body.String()
	if strings.Contains(body, "server:8080") {
		t.Errorf("SECURITY: internal Docker address leaked in response body")
	}
}

func TestOAuthProxy_StressConcurrentRequests(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://accounts.google.com/o/oauth2/auth?client_id=xxx")
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
			w := httptest.NewRecorder()
			handler.HandleGoogleLogin(w, req)

			if w.Code != http.StatusFound {
				t.Errorf("concurrent request got status %d", w.Code)
			}
			location := w.Header().Get("Location")
			if !strings.HasPrefix(location, "https://accounts.google.com/") {
				t.Errorf("concurrent request got wrong redirect: %s", location)
			}
		}()
	}
	wg.Wait()
}

func TestOAuthProxy_StressProviderPathVerification(t *testing.T) {
	// The provider string is hardcoded to "google" or "github" in the callers.
	// Verify the constructed URL path matches expectations.
	var receivedPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Location", "https://accounts.google.com/")
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	// Google login
	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)
	if receivedPath != "/api/auth/login/google" {
		t.Errorf("expected /api/auth/login/google, got %s", receivedPath)
	}

	// GitHub login
	req = httptest.NewRequest("GET", "/api/auth/login/github", nil)
	w = httptest.NewRecorder()
	handler.HandleGitHubLogin(w, req)
	if receivedPath != "/api/auth/login/github" {
		t.Errorf("expected /api/auth/login/github, got %s", receivedPath)
	}
}

func TestOAuthProxy_StressCallbackURLEscaping(t *testing.T) {
	// Verify the callback URL with special characters is properly URL-encoded.
	var receivedQuery string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Location", "https://accounts.google.com/")
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	callbackURL := "http://portal:8881/auth/callback?state=abc&nonce=xyz"
	handler := NewAuthHandler(nil, false, mockServer.URL, callbackURL, []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	expectedEncoded := "callback=" + url.QueryEscape(callbackURL)
	if receivedQuery != expectedEncoded {
		t.Errorf("callback not properly escaped:\nexpected: %s\ngot:      %s", expectedEncoded, receivedQuery)
	}
}

func TestOAuthProxy_StressLocationHeaderInjection(t *testing.T) {
	// Test CRLF injection in Location header from vire-server.
	// Go's net/http strips \r\n in header values, replacing them with spaces.
	// This prevents actual HTTP header injection on the wire, but the text
	// "Set-Cookie: stolen=true" remains in the Location URL string (harmlessly).
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://legit.com/path\r\nSet-Cookie: stolen=true")
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	// CRITICAL: No actual Set-Cookie header must be created from the injection.
	// Go's HTTP server sanitizes \r\n, so the injected text becomes part of the
	// Location URL string, not a separate header.
	for _, c := range w.Result().Cookies() {
		if c.Name == "stolen" {
			t.Error("SECURITY: CRLF injection in Location header created a real Set-Cookie header")
		}
	}

	// The Location header value will contain the sanitized text (spaces replace \r\n),
	// but this is not exploitable since browsers won't parse it as a separate header.
	t.Log("NOTE: Go sanitizes CRLF in header values. No actual header injection is possible.")
}

func TestOAuthProxy_StressEmptyLocationHeader(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "")
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_failed") {
		t.Errorf("expected auth_failed redirect for empty Location, got %s", location)
	}
}

func TestOAuthProxy_StressVeryLongLocation(t *testing.T) {
	// Vire-server returns an extremely long Location header.
	longLocation := "https://accounts.google.com/?" + strings.Repeat("x="+strings.Repeat("y", 1000)+"&", 1000)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", longLocation)
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()

	// Must not panic
	handler.HandleGoogleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 for very long Location, got %d", w.Code)
	}
}

func TestOAuthProxy_StressGitHubSameAsGoogle(t *testing.T) {
	// Verify GitHub login uses the same proxy path and has identical security properties.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "github") {
			t.Errorf("expected github in path, got %s", r.URL.Path)
		}
		w.Header().Set("Location", "https://github.com/login/oauth/authorize?client_id=xxx")
		w.WriteHeader(http.StatusFound)
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	// Test unreachable server
	handler2 := NewAuthHandler(nil, false, "http://127.0.0.1:1", "http://localhost:8500/auth/callback", []byte{})
	req := httptest.NewRequest("GET", "/api/auth/login/github", nil)
	w := httptest.NewRecorder()
	handler2.HandleGitHubLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_unavailable") {
		t.Errorf("GitHub login: expected auth_unavailable on unreachable server, got %s", location)
	}

	// Test successful redirect
	req = httptest.NewRequest("GET", "/api/auth/login/github", nil)
	w = httptest.NewRecorder()
	handler.HandleGitHubLogin(w, req)

	location = w.Header().Get("Location")
	if !strings.HasPrefix(location, "https://github.com/") {
		t.Errorf("expected GitHub OAuth URL, got %s", location)
	}
}

func TestOAuthProxy_StressResponseBodyNotConsumed(t *testing.T) {
	// The proxy reads the Location header but does not consume the response body.
	// Verify this doesn't cause issues even if the server sends a body.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://accounts.google.com/")
		w.WriteHeader(http.StatusFound)
		// Send a large body alongside the redirect
		w.Write([]byte(strings.Repeat("garbage body data ", 10000)))
	}))
	defer mockServer.Close()

	handler := NewAuthHandler(nil, false, mockServer.URL, "http://localhost:8500/auth/callback", []byte{})

	// Run multiple times to check for connection/resource leaks
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
		w := httptest.NewRecorder()
		handler.HandleGoogleLogin(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("iteration %d: expected 302, got %d", i, w.Code)
		}
	}
}

// --- Login Server Interaction ---

func newLoginRequest(username, password string) *http.Request {
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("username="+username+"&password="+password))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestLogin_StressServerReturnsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:8500/auth/callback", []byte{})

	req := newLoginRequest("dev_user", "dev123")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_failed") {
		t.Errorf("expected error redirect for invalid JSON, got %s", location)
	}
	// Must NOT set cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			t.Error("SECURITY: session cookie set despite invalid server response")
		}
	}
}

func TestLogin_StressServerReturnsEmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": ""},
		})
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:8500/auth/callback", []byte{})

	req := newLoginRequest("dev_user", "dev123")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=auth_failed") {
		t.Errorf("expected error redirect for empty token, got %s", location)
	}
}

func TestLogin_StressServerUnreachable(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://127.0.0.1:1", "http://localhost:8500/auth/callback", []byte{})

	req := newLoginRequest("dev_user", "dev123")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "reason=server_unavailable") {
		t.Errorf("expected server_unavailable error redirect when server unreachable, got %s", location)
	}
}

func TestLogin_StressServerLargeResponse(t *testing.T) {
	// Vire-server returns a very large response body.
	// The handler uses io.LimitReader(1<<20) to cap at 1MB.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","data":{"token":"`))
		w.Write([]byte(strings.Repeat("x", 2<<20))) // 2MB
		w.Write([]byte(`"}}`))
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:8500/auth/callback", []byte{})

	req := newLoginRequest("dev_user", "dev123")
	w := httptest.NewRecorder()

	// Must not panic or OOM
	handler.HandleLogin(w, req)

	// Should fail to parse (truncated response) or succeed with truncated token
	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
}

// --- Cookie Security Comprehensive Tests ---

func TestAllAuthCookies_StressHttpOnlyFlag(t *testing.T) {
	// Every session cookie must be HttpOnly to prevent XSS cookie theft.
	token := buildTestJWT("dev_user")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": token},
		})
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:8500/auth/callback", []byte{})

	scenarios := []struct {
		name   string
		method string
		path   string
	}{
		{"login", "POST", "/api/auth/login"},
		{"oauth callback", "GET", "/auth/callback?token=" + token},
		{"logout", "POST", "/api/auth/logout"},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			var req *http.Request
			if sc.name == "login" {
				req = newLoginRequest("dev_user", "dev123")
			} else {
				req = httptest.NewRequest(sc.method, sc.path, nil)
			}
			if sc.name == "logout" {
				req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			}
			w := httptest.NewRecorder()

			switch sc.name {
			case "login":
				handler.HandleLogin(w, req)
			case "oauth callback":
				handler.HandleOAuthCallback(w, req)
			case "logout":
				handler.HandleLogout(w, req)
			}

			for _, c := range w.Result().Cookies() {
				if c.Name == "vire_session" && !c.HttpOnly {
					t.Errorf("SECURITY: %s sets vire_session without HttpOnly flag", sc.name)
				}
			}
		})
	}
}

func TestAllAuthCookies_StressSameSiteAttribute(t *testing.T) {
	// Session cookies should have SameSite=Lax to prevent CSRF.
	token := buildTestJWT("dev_user")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": token},
		})
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:8500/auth/callback", []byte{})

	// Login
	req := newLoginRequest("dev_user", "dev123")
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			if c.SameSite != http.SameSiteLaxMode && c.SameSite != http.SameSiteStrictMode {
				t.Errorf("dev login cookie SameSite=%v, expected Lax or Strict", c.SameSite)
			}
		}
	}

	// Callback
	req2 := httptest.NewRequest("GET", "/auth/callback?token="+token, nil)
	w2 := httptest.NewRecorder()
	handler.HandleOAuthCallback(w2, req2)

	for _, c := range w2.Result().Cookies() {
		if c.Name == "vire_session" {
			if c.SameSite != http.SameSiteLaxMode && c.SameSite != http.SameSiteStrictMode {
				t.Errorf("callback cookie SameSite=%v, expected Lax or Strict", c.SameSite)
			}
		}
	}
}

// --- ExtractJWTSub Backwards Compatibility ---

func TestExtractJWTSub_StressExpiredTokenReturnsEmpty(t *testing.T) {
	// ExtractJWTSub now uses ValidateJWT which checks expiry.
	token := buildExpiredJWT("alice")
	sub := ExtractJWTSub(token)
	if sub != "" {
		t.Errorf("expected empty sub for expired token from ExtractJWTSub, got %q", sub)
	}
}

func TestExtractJWTSub_StressStillWorksForBackwardsCompat(t *testing.T) {
	token := buildTestJWT("alice")
	sub := ExtractJWTSub(token)
	if sub != "alice" {
		t.Errorf("expected sub=alice from deprecated ExtractJWTSub, got %q", sub)
	}
}

// --- Config Defaults Security ---

func TestConfigAuth_DefaultSecretIsEmpty(t *testing.T) {
	// Document that the default jwt_secret is empty, which skips signature verification.
	// This is acceptable for dev/migration but must be set in production.
	// The test serves as documentation and a reminder.
	t.Log("DEFAULT: jwt_secret is empty, signature verification is skipped. Set VIRE_AUTH_JWT_SECRET in production.")
}

// --- Hostile Provider in ExchangeOAuth ---

func TestExchangeOAuth_StressHostileProviderNames(t *testing.T) {
	hostile := []string{
		"<script>alert(1)</script>",
		"'; DROP TABLE providers; --",
		strings.Repeat("A", 10000),
		"dev\nX-Injected: evil",
		"../../etc/passwd",
		"",
	}

	for _, provider := range hostile {
		t.Run("provider_"+safeSubstring(provider, 20), func(t *testing.T) {
			// Verify hostile values round-trip through JSON without corruption
			body := map[string]string{"provider": provider, "code": "c", "state": "s"}
			jsonData, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("json.Marshal failed for provider %q: %v", provider, err)
			}
			var decoded map[string]string
			json.Unmarshal(jsonData, &decoded)
			if decoded["provider"] != provider {
				t.Errorf("provider value mangled: %q -> %q", provider, decoded["provider"])
			}
		})
	}
}

// --- HandleLogout Cookie Security ---

func TestLogout_StressCookieMissingSameSite(t *testing.T) {
	// FINDING: HandleLogout clears the cookie without setting SameSite.
	// The login and callback paths both set SameSite=Lax, but logout does not.
	// While this is less critical (the cookie value is empty and MaxAge=-1),
	// it's inconsistent and some browsers may treat missing SameSite differently.
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.HandleLogout(w, req)

	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			if c.SameSite != http.SameSiteLaxMode && c.SameSite != http.SameSiteStrictMode {
				t.Errorf("FINDING: logout cookie SameSite=%v, should be Lax or Strict for consistency with login/callback", c.SameSite)
			}
		}
	}
}

// --- HandleOAuthCallback: Token Not Validated ---

func TestOAuthCallback_StressArbitraryTokenStored(t *testing.T) {
	// FINDING: HandleOAuthCallback stores whatever token string is provided
	// as the session cookie WITHOUT validating it as a JWT. This means:
	// 1. Expired tokens from vire-server are stored
	// 2. Malformed strings are stored
	// 3. The browser will send these back on subsequent requests
	//
	// Downstream consumers (IsLoggedIn, withUserContext) DO validate,
	// so this is not directly exploitable -- but it means the user gets
	// a cookie that appears to be "logged in" but is actually rejected
	// on every protected page, creating a confusing UX.
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:8500/auth/callback", []byte("secret"))

	// Store an expired JWT via callback
	expiredToken := buildExpiredJWT("alice")
	req := httptest.NewRequest("GET", "/auth/callback?token="+url.QueryEscape(expiredToken), nil)
	w := httptest.NewRecorder()

	handler.HandleOAuthCallback(w, req)

	// The handler stores it without validation
	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected cookie to be set")
	}

	// Now verify that IsLoggedIn correctly rejects this expired token
	req2 := httptest.NewRequest("GET", "/dashboard", nil)
	req2.AddCookie(sessionCookie)
	loggedIn, _ := IsLoggedIn(req2, []byte("secret"))
	if loggedIn {
		t.Error("SECURITY: expired token from callback is accepted by IsLoggedIn")
	}

	// Document: the callback stores unvalidated tokens. This is a UX issue,
	// not a security vulnerability, because downstream validation catches it.
	t.Log("FINDING: HandleOAuthCallback stores tokens without validation. Expired/malformed tokens are stored but rejected downstream by IsLoggedIn.")
}

// --- JWT Expiry Boundary ---

func TestValidateJWT_StressExpJustExpired(t *testing.T) {
	// Token that expired 1 second ago
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "alice",
		"exp": time.Now().Unix() - 1,
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	_, err := ValidateJWT(token, []byte{})
	if err == nil {
		t.Error("token expired 1 second ago should be rejected")
	}
}

func TestValidateJWT_StressExpExactlyNow(t *testing.T) {
	// Token expiring at current second -- this is a race but exp < now.Unix()
	// means exp == now is still valid (not less than)
	now := time.Now().Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "alice",
		"exp": now,
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	// exp == now: the check is `exp < now.Unix()`. If time hasn't advanced,
	// exp == now means NOT less than, so it should be valid.
	// But this is inherently racy. Just verify it doesn't panic.
	_, _ = ValidateJWT(token, []byte{})
}

func TestValidateJWT_StressFutureIat(t *testing.T) {
	// Token with iat in the future -- we don't validate iat, just exp.
	// This is fine but worth documenting.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "alice",
		"iat": time.Now().Add(1 * time.Hour).Unix(),
		"exp": time.Now().Add(2 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	result, err := ValidateJWT(token, []byte{})
	if err != nil {
		t.Fatalf("future iat should not cause rejection (we only check exp): %v", err)
	}
	if result.Sub != "alice" {
		t.Errorf("expected sub alice, got %s", result.Sub)
	}
	t.Log("NOTE: ValidateJWT does not validate iat (issued-at). Tokens with future iat are accepted.")
}

func TestValidateJWT_StressVeryFarFutureExp(t *testing.T) {
	// Token with exp set to year 9999 -- should still be accepted
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": "alice",
		"exp": 253402300799, // 9999-12-31T23:59:59Z
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	token := header + "." + payload + "."

	result, err := ValidateJWT(token, []byte{})
	if err != nil {
		t.Fatalf("far-future exp should be accepted: %v", err)
	}
	if result.Sub != "alice" {
		t.Errorf("expected sub alice, got %s", result.Sub)
	}
	t.Log("NOTE: No max-exp validation. Tokens can have arbitrarily far future expiry.")
}

// --- Login Oversized Form Body ---

func TestLogin_StressOversizedFormBody(t *testing.T) {
	// Go's ParseForm has a built-in 10MB limit for POST bodies.
	// Verify the handler doesn't OOM on a very large form body.
	handler := NewAuthHandler(nil, true, "http://127.0.0.1:1", "http://localhost:8500/auth/callback", []byte{})

	// 5MB form body
	largeBody := "username=" + strings.Repeat("x", 5<<20) + "&password=test"
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Must not panic or OOM
	handler.HandleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
}

// --- HandleLogin: Credentials forwarded as JSON, not form data ---

func TestLogin_StressSpecialCharsInCredentials(t *testing.T) {
	// Verify that special characters in username/password are correctly
	// marshaled as JSON when forwarded to vire-server.
	var receivedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:8500/auth/callback", []byte{})

	specialChars := []struct {
		name     string
		username string
		password string
	}{
		{"quotes", `user"name`, `pass"word`},
		{"backslash", `user\name`, `pass\word`},
		{"unicode", `用户`, `密码`},
		{"newlines", "user\nname", "pass\nword"},
		{"angle brackets", `<user>`, `<pass>`},
		{"ampersand", `user&name`, `pass&word`},
	}

	for _, tc := range specialChars {
		t.Run(tc.name, func(t *testing.T) {
			formData := "username=" + url.QueryEscape(tc.username) + "&password=" + url.QueryEscape(tc.password)
			req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			handler.HandleLogin(w, req)

			// Verify the credentials were correctly forwarded
			if receivedBody["username"] != tc.username {
				t.Errorf("username mangled: expected %q, got %q", tc.username, receivedBody["username"])
			}
			if receivedBody["password"] != tc.password {
				t.Errorf("password mangled: expected %q, got %q", tc.password, receivedBody["password"])
			}
		})
	}
}

func safeSubstring(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// --- GetServerVersion Stress Tests ---

func TestGetServerVersion_StressConcurrentRequests(t *testing.T) {
	// Test concurrent calls to GetServerVersion to check for race conditions.
	// Each call creates a new http.Client, which should be safe but inefficient.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer mockServer.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			version := GetServerVersion(mockServer.URL)
			if version != "1.0.0" {
				t.Errorf("expected version '1.0.0', got %q", version)
			}
		}()
	}
	wg.Wait()
}

func TestGetServerVersion_StressLargeResponseBody(t *testing.T) {
	// Server returns a very large response body.
	// Without a limit on response body size, this could cause memory exhaustion.
	// The current implementation uses json.Decoder which reads incrementally,
	// but still buffers the entire body internally.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 10MB of garbage before the actual JSON
		w.Write([]byte("{"))
		w.Write([]byte(strings.Repeat("\"filler\":\""+strings.Repeat("x", 1000)+"\",", 5000)))
		w.Write([]byte(`"version":"1.0.0"}`))
	}))
	defer mockServer.Close()

	// This should not panic or OOM, but may be slow or return unavailable
	version := GetServerVersion(mockServer.URL)
	// Either it parses successfully or returns unavailable -- both are acceptable
	if version != "1.0.0" && version != "unavailable" {
		t.Errorf("expected '1.0.0' or 'unavailable', got %q", version)
	}
}

func TestGetServerVersion_StressMalformedJSON(t *testing.T) {
	malformedResponses := []struct {
		name string
		body string
		want string
	}{
		{"unclosed brace", `{"version":"1.0.0"`, "unavailable"},
		{"extra comma", `{"version":"1.0.0",}`, "unavailable"},
		{"wrong type for version", `{"version":123}`, "unavailable"},
		{"null version", `{"version":null}`, "unavailable"},
		{"array instead of object", `["version","1.0.0"]`, "unavailable"},
		{"string instead of object", `"not an object"`, "unavailable"},
		{"deeply nested", `{"a":{"b":{"c":{"d":{"e":{"version":"1.0.0"}}}}}}`, "unavailable"},
		{"unicode in response", `{"version":"1.0.0","描述":"测试"}`, "1.0.0"},
		{"HTML in response", `{"version":"1.0.0","<script>alert(1)</script>":"x"}`, "1.0.0"},
	}

	for _, tc := range malformedResponses {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tc.body))
			}))
			defer mockServer.Close()

			version := GetServerVersion(mockServer.URL)
			if version != tc.want {
				t.Errorf("expected %q, got %q", tc.want, version)
			}
		})
	}
}

func TestGetServerVersion_StressHostileVersionValues(t *testing.T) {
	// Test various hostile strings in the version field to ensure they're
	// returned as-is without causing injection issues in the template.
	hostileVersions := []struct {
		name    string
		version string
	}{
		{"script tag", `<script>alert('xss')</script>`},
		{"HTML entities", `&lt;script&gt;`},
		{"SQL injection", `'; DROP TABLE versions; --`},
		{"CRLF injection", "1.0.0\r\nX-Injected: evil"},
		{"null bytes", "1.0\x00.0"},
		{"unicode control chars", "\x1b[31mred\x1b[0m"},
		{"very long", strings.Repeat("x", 10000)},
		{"template injection", "{{.PortalVersion}}"},
		{"go template", `{{"production"}}`},
	}

	for _, tc := range hostileVersions {
		t.Run(tc.name, func(t *testing.T) {
			escaped, _ := json.Marshal(tc.version)
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(fmt.Sprintf(`{"version":%s}`, escaped)))
			}))
			defer mockServer.Close()

			// GetServerVersion should return the version string as-is
			version := GetServerVersion(mockServer.URL)
			if version != tc.version {
				t.Errorf("version mismatch: expected %q, got %q", tc.version, version)
			}
			// Note: The security check for XSS happens in the template rendering,
			// not in GetServerVersion. Go templates auto-escape by default.
		})
	}
}

func TestGetServerVersion_StressServerRedirects(t *testing.T) {
	// Test that GetServerVersion follows redirects (Go's default behavior).
	// A malicious server could redirect to internal endpoints.
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"redirected"}`))
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL+"/api/version", http.StatusFound)
	}))
	defer redirectServer.Close()

	// By default, Go's http.Client follows up to 10 redirects
	version := GetServerVersion(redirectServer.URL)
	if version != "redirected" {
		t.Errorf("expected 'redirected', got %q", version)
	}
	// FINDING: GetServerVersion follows redirects. If apiURL is ever user-controlled,
	// this could be exploited for SSRF. Since it comes from config, this is mitigated.
	t.Log("NOTE: GetServerVersion follows redirects. Ensure apiURL is never user-controlled.")
}

func TestGetServerVersion_StressRedirectLoop(t *testing.T) {
	// Test redirect loop handling -- Go's http.Client has a 10-redirect limit
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
	}))
	defer redirectServer.Close()

	// Should eventually error out after 10 redirects
	version := GetServerVersion(redirectServer.URL)
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on redirect loop, got %q", version)
	}
}

func TestGetServerVersion_StressSlowHeaders(t *testing.T) {
	// Server is slow to send response headers (connection established but headers delayed)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // Longer than 2s timeout
		w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer mockServer.Close()

	start := time.Now()
	version := GetServerVersion(mockServer.URL)
	elapsed := time.Since(start)

	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on slow headers, got %q", version)
	}
	if elapsed > 2500*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestGetServerVersion_StressSlowBody(t *testing.T) {
	// Server sends headers immediately but trickles body slowly
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ver`))
		time.Sleep(3 * time.Second) // Pause mid-response
		w.Write([]byte(`sion":"1.0.0"}`))
	}))
	defer mockServer.Close()

	start := time.Now()
	version := GetServerVersion(mockServer.URL)
	elapsed := time.Since(start)

	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on slow body, got %q", version)
	}
	if elapsed > 2500*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestGetServerVersion_StressWrongContentType(t *testing.T) {
	// Server returns non-JSON content type but valid JSON body
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer mockServer.Close()

	// Currently accepts any content type as long as body is valid JSON
	version := GetServerVersion(mockServer.URL)
	if version != "1.0.0" {
		t.Errorf("expected '1.0.0', got %q", version)
	}
	// FINDING: No content-type validation. A server returning HTML with embedded JSON
	// would have its version extracted. This is a minor concern since apiURL is trusted.
	t.Log("NOTE: GetServerVersion does not validate Content-Type header.")
}

func TestGetServerVersion_StressMultipleVersions(t *testing.T) {
	// Response contains multiple version fields -- JSON decoder takes the last one
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"1.0.0","version":"2.0.0"}`))
	}))
	defer mockServer.Close()

	version := GetServerVersion(mockServer.URL)
	// Go's JSON decoder behavior for duplicate keys is to use the last value
	if version != "2.0.0" {
		t.Errorf("expected '2.0.0' (last value), got %q", version)
	}
}

func TestGetServerVersion_StressConnectionRefused(t *testing.T) {
	// Point to a port that's not listening
	version := GetServerVersion("http://127.0.0.1:19999")
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on connection refused, got %q", version)
	}
}

func TestGetServerVersion_StressDNSLookupFail(t *testing.T) {
	// Use an invalid hostname that will fail DNS lookup
	version := GetServerVersion("http://this-host-does-not-exist-12345.invalid/api/version")
	if version != "unavailable" {
		t.Errorf("expected 'unavailable' on DNS failure, got %q", version)
	}
}

// --- Footer Template Stress Tests ---

func TestServePage_StressVersionInTemplate(t *testing.T) {
	// Verify that hostile version strings are properly escaped in the template
	handler := NewPageHandler(nil, true, []byte{})
	handler.SetAPIURL("") // Force "unavailable" for server version

	// Create a test server that returns a hostile version
	hostileVersion := `<script>alert('xss')</script>`
	escaped, _ := json.Marshal(hostileVersion)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"version":%s}`, escaped)))
	}))
	defer mockServer.Close()
	handler.SetAPIURL(mockServer.URL)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	body := w.Body.String()

	// Go templates auto-escape HTML by default
	// The raw script tag should be escaped
	if strings.Contains(body, "<script>alert") {
		t.Error("SECURITY: XSS vulnerability - script tag not escaped in template output")
	}
	// Should contain the escaped version
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Logf("Template output for hostile version: %s", body[strings.Index(body, "Server:"):strings.Index(body, "Server:")+100])
		// This is acceptable - Go templates escape by default
	}
}

func TestDashboardHandler_StressConcurrentVersionFetch(t *testing.T) {
	// Test that multiple concurrent dashboard requests don't cause issues
	// with the version fetch
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer mockServer.Close()

	handler := NewDashboardHandler(nil, true, []byte{}, nil)
	handler.SetAPIURL(mockServer.URL)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token := buildTestJWT("dev_user")
			req := httptest.NewRequest("GET", "/dashboard", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			body := w.Body.String()
			if !strings.Contains(body, "Server:") {
				t.Error("expected footer to contain 'Server:' label")
			}
		}()
	}
	wg.Wait()
}
