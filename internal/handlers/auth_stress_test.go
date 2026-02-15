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
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

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
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

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
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

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
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

	req := httptest.NewRequest("GET", "/auth/callback#token=evil", nil)
	w := httptest.NewRecorder()
	handler.HandleOAuthCallback(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=missing_token") {
		t.Errorf("expected missing_token error when token only in fragment, got location=%s", location)
	}
}

func TestOAuthCallback_StressConcurrentRequests(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

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
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

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
		if !strings.HasPrefix(location, "http://localhost:8080/") {
			t.Errorf("SECURITY: Google login redirect influenced: path=%s, location=%s", path, location)
		}
		if strings.Contains(location, "evil.com") {
			t.Errorf("SECURITY: evil.com in redirect URL: %s", location)
		}
	}
}

func TestGitHubLogin_StressOpenRedirectProtection(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

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

func TestOAuthLogin_StressCallbackURLNotEncoded(t *testing.T) {
	// The callback URL is concatenated without url.QueryEscape.
	// If callbackURL contains & or # chars, the redirect URL becomes ambiguous.
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback?extra=param&another=val", []byte{})

	req := httptest.NewRequest("GET", "/api/auth/login/google", nil)
	w := httptest.NewRecorder()
	handler.HandleGoogleLogin(w, req)

	location := w.Header().Get("Location")
	// This is a defense-in-depth concern: callbackURL comes from config, not user input.
	// But the string concatenation is fragile.
	t.Logf("Redirect URL with complex callback: %s", location)
}

// --- Dev Login Server Interaction ---

func TestDevLogin_StressServerReturnsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:4241/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	handler.HandleDevLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=auth_failed") {
		t.Errorf("expected error redirect for invalid JSON, got %s", location)
	}
	// Must NOT set cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			t.Error("SECURITY: session cookie set despite invalid server response")
		}
	}
}

func TestDevLogin_StressServerReturnsEmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"token": ""},
		})
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:4241/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	handler.HandleDevLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=auth_failed") {
		t.Errorf("expected error redirect for empty token, got %s", location)
	}
}

func TestDevLogin_StressProdMode_NoCookies(t *testing.T) {
	handler := NewAuthHandler(nil, false, "http://localhost:8080", "http://localhost:4241/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	handler.HandleDevLogin(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 in prod mode, got %d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "vire_session" {
			t.Error("SECURITY: session cookie set in prod mode dev login")
		}
	}
	if location := w.Header().Get("Location"); location != "" {
		t.Errorf("expected no redirect in prod mode, got Location: %s", location)
	}
}

func TestDevLogin_StressServerUnreachable(t *testing.T) {
	handler := NewAuthHandler(nil, true, "http://127.0.0.1:1", "http://localhost:4241/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	handler.HandleDevLogin(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=auth_failed") {
		t.Errorf("expected error redirect when server unreachable, got %s", location)
	}
}

func TestDevLogin_StressServerLargeResponse(t *testing.T) {
	// Vire-server returns a very large response body.
	// The handler uses io.LimitReader(1<<20) to cap at 1MB.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","data":{"token":"`))
		w.Write([]byte(strings.Repeat("x", 2<<20))) // 2MB
		w.Write([]byte(`"}}`))
	}))
	defer srv.Close()

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:4241/auth/callback", []byte{})

	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()

	// Must not panic or OOM
	handler.HandleDevLogin(w, req)

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

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:4241/auth/callback", []byte{})

	scenarios := []struct {
		name   string
		method string
		path   string
	}{
		{"dev login", "POST", "/api/auth/dev"},
		{"oauth callback", "GET", "/auth/callback?token=" + token},
		{"logout", "POST", "/api/auth/logout"},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			req := httptest.NewRequest(sc.method, sc.path, nil)
			if sc.name == "logout" {
				req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})
			}
			w := httptest.NewRecorder()

			switch sc.name {
			case "dev login":
				handler.HandleDevLogin(w, req)
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

	handler := NewAuthHandler(nil, true, srv.URL, "http://localhost:4241/auth/callback", []byte{})

	// Dev login
	req := httptest.NewRequest("POST", "/api/auth/dev", nil)
	w := httptest.NewRecorder()
	handler.HandleDevLogin(w, req)

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

func safeSubstring(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
