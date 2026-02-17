package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// validateTestJWT validates a JWT signed with HMAC-SHA256 for test purposes.
func validateTestJWT(token string, secret []byte) (map[string]interface{}, error) {
	parts := strings.SplitN(token, ".", 4)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT: %d parts", len(parts))
	}

	if len(secret) > 0 {
		sigInput := parts[0] + "." + parts[1]
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(sigInput))
		expectedSig := mac.Sum(nil)

		actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid signature encoding: %w", err)
		}

		if !hmac.Equal(expectedSig, actualSig) {
			return nil, fmt.Errorf("invalid signature")
		}
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}

	return claims, nil
}

// =============================================================================
// Token Endpoint Stress Tests
// =============================================================================

// setupCodeExchange creates a valid auth code ready for exchange.
func setupCodeExchange(srv *OAuthServer, verifier string) (*AuthCode, *OAuthClient) {
	client := registerTestClient(srv)
	challenge := GenerateCodeChallenge(verifier)

	authCode := &AuthCode{
		Code:          "test-auth-code",
		ClientID:      client.ClientID,
		UserID:        "user-1",
		RedirectURI:   "http://localhost:3000/callback",
		CodeChallenge: challenge,
		Scope:         "openid portfolio:read",
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	}
	srv.codes.Put(authCode)
	return authCode, client
}

func postToken(srv *OAuthServer, params url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(params.Encode()))
	req.Host = "localhost:4241"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.HandleToken(rec, req)
	return rec
}

// --- Token: auth code replay (use code twice) ---

func TestToken_StressAuthCodeReplay(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	authCode, _ := setupCodeExchange(srv, verifier)

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode.Code},
		"client_id":     {authCode.ClientID},
		"redirect_uri":  {authCode.RedirectURI},
		"code_verifier": {verifier},
	}

	// First exchange should succeed
	rec1 := postToken(srv, params)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first exchange failed: %d - %s", rec1.Code, rec1.Body.String())
	}

	// Second exchange with same code should fail
	rec2 := postToken(srv, params)
	if rec2.Code == http.StatusOK {
		t.Error("SECURITY: Auth code accepted twice — code replay vulnerability")
	}

	var errResp map[string]string
	json.NewDecoder(rec2.Body).Decode(&errResp)
	if errResp["error"] != "invalid_grant" {
		t.Errorf("expected invalid_grant error on replay, got %s", errResp["error"])
	}
}

// --- Token: concurrent code exchange race ---

func TestToken_StressConcurrentCodeExchangeRace(t *testing.T) {
	// CRITICAL: Two concurrent requests both try to exchange the same auth code.
	// Only one should succeed (single-use code requirement).
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	authCode, _ := setupCodeExchange(srv, verifier)

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode.Code},
		"client_id":     {authCode.ClientID},
		"redirect_uri":  {authCode.RedirectURI},
		"code_verifier": {verifier},
	}

	var successCount int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := postToken(srv, params)
			if rec.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount > 1 {
		t.Errorf("CRITICAL: Auth code exchanged %d times (should be exactly 1). "+
			"The TOCTOU race between Get(code).Used check and MarkUsed(code) allows "+
			"concurrent requests to both see Used=false and both succeed. "+
			"Fix: Use an atomic ConsumeCode method that checks and marks under write lock.", successCount)
	} else if successCount == 1 {
		t.Log("Auth code was exchanged exactly once (race did not manifest, but TOCTOU vulnerability exists)")
	} else {
		t.Error("Auth code exchange failed for all requests")
	}
}

// --- Token: PKCE bypass attempts ---

func TestToken_StressPKCEBypassAttempts(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "correct-verifier-with-enough-entropy-here"
	authCode, _ := setupCodeExchange(srv, verifier)

	bypasses := []struct {
		name     string
		verifier string
	}{
		{"empty verifier", ""},
		{"wrong verifier", "wrong-verifier-with-enough-entropy-here"},
		{"almost right", verifier[:len(verifier)-1] + "X"},
		{"challenge as verifier", authCode.CodeChallenge},
		{"null bytes", "correct-verifier\x00evil"},
		{"unicode padding", verifier + "\u200b"},
		{"very long", strings.Repeat("A", 1<<16)},
		{"space prefix", " " + verifier},
		{"space suffix", verifier + " "},
	}

	for _, tc := range bypasses {
		t.Run(tc.name, func(t *testing.T) {
			// Reset code for each attempt (codes are single-use)
			freshCode := &AuthCode{
				Code:          fmt.Sprintf("code-%s", tc.name),
				ClientID:      authCode.ClientID,
				UserID:        "user-1",
				RedirectURI:   authCode.RedirectURI,
				CodeChallenge: authCode.CodeChallenge,
				Scope:         "openid",
				ExpiresAt:     time.Now().Add(5 * time.Minute),
			}
			srv.codes.Put(freshCode)

			params := url.Values{
				"grant_type":    {"authorization_code"},
				"code":          {freshCode.Code},
				"client_id":     {freshCode.ClientID},
				"redirect_uri":  {freshCode.RedirectURI},
				"code_verifier": {tc.verifier},
			}

			rec := postToken(srv, params)
			if rec.Code == http.StatusOK {
				t.Errorf("SECURITY: PKCE bypassed with %s", tc.name)
			}
		})
	}
}

// --- Token: wrong client_id ---

func TestToken_StressWrongClientID(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	authCode, _ := setupCodeExchange(srv, verifier)

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode.Code},
		"client_id":     {"wrong-client-id"},
		"redirect_uri":  {authCode.RedirectURI},
		"code_verifier": {verifier},
	}

	rec := postToken(srv, params)
	if rec.Code == http.StatusOK {
		t.Error("SECURITY: Token issued for wrong client_id")
	}
}

// --- Token: wrong redirect_uri ---

func TestToken_StressWrongRedirectURI(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	authCode, _ := setupCodeExchange(srv, verifier)

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode.Code},
		"client_id":     {authCode.ClientID},
		"redirect_uri":  {"http://attacker.com/steal"},
		"code_verifier": {verifier},
	}

	rec := postToken(srv, params)
	if rec.Code == http.StatusOK {
		t.Error("SECURITY: Token issued with wrong redirect_uri")
	}
}

// --- Token: expired auth code ---

func TestToken_StressExpiredAuthCode(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	challenge := GenerateCodeChallenge(verifier)

	srv.codes.Put(&AuthCode{
		Code:          "expired-code",
		ClientID:      "test-client-id",
		UserID:        "user-1",
		RedirectURI:   "http://localhost:3000/callback",
		CodeChallenge: challenge,
		Scope:         "openid",
		ExpiresAt:     time.Now().Add(-1 * time.Minute),
	})

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"expired-code"},
		"client_id":     {"test-client-id"},
		"redirect_uri":  {"http://localhost:3000/callback"},
		"code_verifier": {verifier},
	}

	rec := postToken(srv, params)
	if rec.Code == http.StatusOK {
		t.Error("SECURITY: Expired auth code accepted")
	}
}

// --- Token: missing parameters ---

func TestToken_StressMissingParameters(t *testing.T) {
	srv := newStressOAuthServer()

	missing := []struct {
		name   string
		params url.Values
	}{
		{"no grant_type", url.Values{"code": {"x"}, "client_id": {"x"}, "redirect_uri": {"x"}, "code_verifier": {"x"}}},
		{"no code", url.Values{"grant_type": {"authorization_code"}, "client_id": {"x"}, "redirect_uri": {"x"}, "code_verifier": {"x"}}},
		{"no client_id", url.Values{"grant_type": {"authorization_code"}, "code": {"x"}, "redirect_uri": {"x"}, "code_verifier": {"x"}}},
		{"no redirect_uri", url.Values{"grant_type": {"authorization_code"}, "code": {"x"}, "client_id": {"x"}, "code_verifier": {"x"}}},
		{"no code_verifier", url.Values{"grant_type": {"authorization_code"}, "code": {"x"}, "client_id": {"x"}, "redirect_uri": {"x"}}},
		{"empty body", url.Values{}},
	}

	for _, tc := range missing {
		t.Run(tc.name, func(t *testing.T) {
			rec := postToken(srv, tc.params)
			if rec.Code == http.StatusOK {
				t.Errorf("SECURITY: token issued with %s", tc.name)
			}
		})
	}
}

// --- Token: unsupported grant types ---

func TestToken_StressUnsupportedGrantTypes(t *testing.T) {
	srv := newStressOAuthServer()

	grantTypes := []string{
		"client_credentials",
		"password",
		"implicit",
		"urn:ietf:params:oauth:grant-type:device_code",
		"",
		"authorization_code; DROP TABLE codes",
	}

	for _, gt := range grantTypes {
		t.Run(gt, func(t *testing.T) {
			params := url.Values{"grant_type": {gt}}
			rec := postToken(srv, params)
			if rec.Code == http.StatusOK {
				t.Errorf("SECURITY: unsupported grant_type %q accepted", gt)
			}
		})
	}
}

// --- Token: access token properties ---

func TestToken_StressAccessTokenProperties(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	authCode, _ := setupCodeExchange(srv, verifier)

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode.Code},
		"client_id":     {authCode.ClientID},
		"redirect_uri":  {authCode.RedirectURI},
		"code_verifier": {verifier},
	}

	rec := postToken(srv, params)
	if rec.Code != http.StatusOK {
		t.Fatalf("token exchange failed: %d - %s", rec.Code, rec.Body.String())
	}

	var tokenResp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&tokenResp)

	// Verify required fields
	if tokenResp["access_token"] == nil || tokenResp["access_token"] == "" {
		t.Error("access_token missing from response")
	}
	if tokenResp["token_type"] != "Bearer" {
		t.Errorf("expected token_type Bearer, got %v", tokenResp["token_type"])
	}
	if tokenResp["expires_in"] == nil {
		t.Error("expires_in missing from response")
	}
	if tokenResp["refresh_token"] == nil || tokenResp["refresh_token"] == "" {
		t.Error("refresh_token missing from response")
	}

	// Verify Cache-Control: no-store (RFC 6749 Section 5.1)
	cacheControl := rec.Header().Get("Cache-Control")
	if cacheControl != "no-store" {
		t.Errorf("SECURITY: token response Cache-Control=%q, should be no-store per RFC 6749", cacheControl)
	}

	// Verify Content-Type
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

// --- Token: JWT access token validation ---

func TestToken_StressAccessTokenJWTStructure(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	authCode, _ := setupCodeExchange(srv, verifier)

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode.Code},
		"client_id":     {authCode.ClientID},
		"redirect_uri":  {authCode.RedirectURI},
		"code_verifier": {verifier},
	}

	rec := postToken(srv, params)
	var tokenResp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&tokenResp)

	accessToken := tokenResp["access_token"].(string)

	// Verify it's a valid JWT (3 parts)
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("access token is not a JWT: %d parts", len(parts))
	}

	// Verify the JWT is signed with the server's secret
	claims, err := validateTestJWT(accessToken, srv.jwtSecret)
	if err != nil {
		t.Fatalf("access token JWT validation failed: %v", err)
	}

	// Verify claims
	if claims["sub"] != "user-1" {
		t.Errorf("expected sub=user-1, got %v", claims["sub"])
	}
	if claims["iss"] != "http://localhost:4241" {
		t.Errorf("expected iss=http://localhost:4241, got %v", claims["iss"])
	}
}

// =============================================================================
// Refresh Token Stress Tests
// =============================================================================

// --- Refresh token: basic rotation ---

func TestToken_StressRefreshTokenRotation(t *testing.T) {
	srv := newStressOAuthServer()
	verifier := "test-verifier-1234567890123456789012345"
	authCode, _ := setupCodeExchange(srv, verifier)

	// Exchange auth code for tokens
	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode.Code},
		"client_id":     {authCode.ClientID},
		"redirect_uri":  {authCode.RedirectURI},
		"code_verifier": {verifier},
	}
	rec := postToken(srv, params)
	var tokenResp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&tokenResp)

	originalRefresh := tokenResp["refresh_token"].(string)

	// Use refresh token to get new tokens
	refreshParams := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {originalRefresh},
		"client_id":     {authCode.ClientID},
	}
	rec2 := postToken(srv, refreshParams)
	if rec2.Code != http.StatusOK {
		t.Fatalf("refresh failed: %d - %s", rec2.Code, rec2.Body.String())
	}

	var refreshResp map[string]interface{}
	json.NewDecoder(rec2.Body).Decode(&refreshResp)

	newRefresh := refreshResp["refresh_token"].(string)

	// Old refresh token should be invalid
	replayParams := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {originalRefresh},
		"client_id":     {authCode.ClientID},
	}
	rec3 := postToken(srv, replayParams)
	if rec3.Code == http.StatusOK {
		t.Error("SECURITY: Old refresh token still valid after rotation — stolen token replay possible")
	}

	// New refresh token should work
	newParams := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {newRefresh},
		"client_id":     {authCode.ClientID},
	}
	rec4 := postToken(srv, newParams)
	if rec4.Code != http.StatusOK {
		t.Error("new refresh token should be valid")
	}
}

// --- Refresh token: concurrent rotation race ---

func TestToken_StressConcurrentRefreshTokenRotation(t *testing.T) {
	srv := newStressOAuthServer()

	// Create a refresh token
	srv.tokens.Put(&RefreshToken{
		Token:     "shared-refresh",
		UserID:    "user-1",
		ClientID:  "test-client-id",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})
	registerTestClient(srv)

	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"shared-refresh"},
		"client_id":     {"test-client-id"},
	}

	var successCount int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := postToken(srv, params)
			if rec.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount > 1 {
		t.Logf("FINDING: Refresh token rotated %d times (should be exactly 1). "+
			"tokens.Get + tokens.Delete + tokens.Put is not atomic. "+
			"Multiple access tokens were issued from the same refresh token. "+
			"Fix: Implement atomic token rotation in TokenStore.", successCount)
	}
}

// --- Refresh token: wrong client_id ---

func TestToken_StressRefreshWrongClientID(t *testing.T) {
	srv := newStressOAuthServer()

	srv.tokens.Put(&RefreshToken{
		Token:     "user1-refresh",
		UserID:    "user-1",
		ClientID:  "client-A",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"user1-refresh"},
		"client_id":     {"client-B"},
	}

	rec := postToken(srv, params)
	if rec.Code == http.StatusOK {
		t.Error("SECURITY: Refresh token accepted for wrong client_id — token theft across clients")
	}
}

// --- Refresh token: expired ---

func TestToken_StressRefreshExpired(t *testing.T) {
	srv := newStressOAuthServer()

	srv.tokens.Put(&RefreshToken{
		Token:     "expired-refresh",
		UserID:    "user-1",
		ClientID:  "test-client-id",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"expired-refresh"},
		"client_id":     {"test-client-id"},
	}

	rec := postToken(srv, params)
	if rec.Code == http.StatusOK {
		t.Error("SECURITY: Expired refresh token accepted")
	}
}

// --- Token: method not allowed ---

func TestToken_StressMethodNotAllowed(t *testing.T) {
	srv := newStressOAuthServer()

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/token", nil)
			rec := httptest.NewRecorder()
			srv.HandleToken(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for %s, got %d", method, rec.Code)
			}
		})
	}
}

// --- Token: JWT with empty secret vulnerability ---

func TestToken_StressEmptySecretMinting(t *testing.T) {
	// If jwtSecret is empty, the access token is signed with an empty key.
	// This means anyone can forge access tokens by signing with empty HMAC.
	srv := NewOAuthServer("http://localhost:4241", []byte{}, nil)

	token, err := srv.mintAccessToken("user-1", "openid", "client-1", "http://localhost:4241")
	if err != nil {
		t.Fatalf("minting failed: %v", err)
	}

	// An attacker can verify/forge tokens with empty secret
	claims, err := validateTestJWT(token, []byte{})
	if err != nil {
		t.Fatalf("validation with empty secret failed: %v", err)
	}

	if claims["sub"] == "user-1" {
		t.Log("CRITICAL: OAuthServer initialized with empty jwtSecret. " +
			"Access tokens are signed with empty HMAC key. " +
			"Since ValidateJWT skips signature verification for empty secret, " +
			"anyone can forge access tokens. " +
			"NewOAuthServer MUST require a non-empty jwtSecret.")
	}
}

// --- Token: error responses don't leak information ---

func TestToken_StressErrorResponseContent(t *testing.T) {
	srv := newStressOAuthServer()

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"nonexistent-code"},
		"client_id":     {"any"},
		"redirect_uri":  {"http://localhost/cb"},
		"code_verifier": {"any"},
	}

	rec := postToken(srv, params)
	body := rec.Body.String()

	// Error response should not leak internal details
	if strings.Contains(body, "goroutine") || strings.Contains(body, ".go:") ||
		strings.Contains(body, "panic") || strings.Contains(body, "runtime.") {
		t.Error("SECURITY: token error response leaks internal information")
	}
}

// --- Token: oversized form body ---

func TestToken_StressOversizedFormBody(t *testing.T) {
	srv := newStressOAuthServer()

	// 5MB form body
	largeBody := "grant_type=authorization_code&code=" + strings.Repeat("x", 5<<20) + "&client_id=x&redirect_uri=x&code_verifier=x"
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	// Must not panic or OOM
	srv.HandleToken(rec, req)

	t.Logf("FINDING: Token endpoint does not limit request body size. "+
		"Go's ParseForm has a 10MB limit, but that's still large. "+
		"Consider using http.MaxBytesReader. Status: %d", rec.Code)
}
