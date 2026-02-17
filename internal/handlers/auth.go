package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// JWTClaims holds the decoded JWT payload claims.
type JWTClaims struct {
	Sub      string `json:"sub"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Iss      string `json:"iss"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
}

// ValidateJWT validates a JWT token string.
// If secret is non-empty, it verifies the HMAC-SHA256 signature.
// If secret is empty, signature verification is skipped (backwards compat).
// Always checks expiry.
func ValidateJWT(token string, secret []byte) (*JWTClaims, error) {
	parts := strings.SplitN(token, ".", 4)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	// Verify signature if secret is non-empty
	if len(secret) > 0 {
		sigInput := parts[0] + "." + parts[1]
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(sigInput))
		expectedSig := mac.Sum(nil)

		actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid JWT signature encoding: %w", err)
		}

		if !hmac.Equal(expectedSig, actualSig) {
			return nil, fmt.Errorf("invalid JWT signature")
		}
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid JWT payload encoding: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("invalid JWT payload JSON: %w", err)
	}

	// Check expiry
	if claims.Exp == 0 {
		return nil, fmt.Errorf("JWT missing exp claim")
	}
	if claims.Exp < time.Now().Unix() {
		return nil, fmt.Errorf("JWT expired")
	}

	return &claims, nil
}

// IsLoggedIn checks the vire_session cookie and validates the JWT.
// Returns (true, claims) if valid, (false, nil) otherwise.
func IsLoggedIn(r *http.Request, secret []byte) (bool, *JWTClaims) {
	cookie, err := r.Cookie("vire_session")
	if err != nil || cookie.Value == "" {
		return false, nil
	}

	claims, err := ValidateJWT(cookie.Value, secret)
	if err != nil {
		return false, nil
	}

	return true, claims
}

// OAuthCompleter completes an MCP authorization session by exchanging a session ID
// and user ID for a redirect URL with an authorization code.
type OAuthCompleter interface {
	CompleteAuthorization(sessionID string, userID string) (redirectURL string, err error)
}

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	logger      *common.Logger
	devMode     bool
	apiURL      string
	callbackURL string
	jwtSecret   []byte
	oauthServer OAuthCompleter
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(logger *common.Logger, devMode bool, apiURL string, callbackURL string, jwtSecret []byte) *AuthHandler {
	return &AuthHandler{
		logger:      logger,
		devMode:     devMode,
		apiURL:      apiURL,
		callbackURL: callbackURL,
		jwtSecret:   jwtSecret,
	}
}

// SetOAuthServer sets the OAuth server for MCP session completion.
func (h *AuthHandler) SetOAuthServer(s OAuthCompleter) {
	h.oauthServer = s
}

// HandleLogin handles email/password login.
// It forwards credentials to vire-server POST /api/auth/login,
// sets the returned JWT as a session cookie, and redirects to /dashboard.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/error?reason=bad_request", http.StatusFound)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		http.Redirect(w, r, "/error?reason=missing_credentials", http.StatusFound)
		return
	}

	body := map[string]string{
		"username": username,
		"password": password,
	}
	bodyJSON, _ := json.Marshal(body)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(h.apiURL+"/api/auth/login", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		if h.logger != nil {
			h.logger.Error().Str("error", err.Error()).Msg("failed to reach vire-server for login")
		}
		http.Redirect(w, r, "/error?reason=server_unavailable", http.StatusFound)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if h.logger != nil {
			h.logger.Error().Str("error", err.Error()).Msg("failed to read vire-server response")
		}
		http.Redirect(w, r, "/error?reason=auth_failed", http.StatusFound)
		return
	}

	if resp.StatusCode != http.StatusOK {
		if h.logger != nil {
			h.logger.Error().Int("status", resp.StatusCode).Str("body", string(respBody)).Msg("vire-server login failed")
		}
		http.Redirect(w, r, "/error?reason=invalid_credentials", http.StatusFound)
		return
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil || result.Data.Token == "" {
		if h.logger != nil {
			h.logger.Error().Str("error", fmt.Sprintf("parse error or empty token: %v", err)).Msg("invalid vire-server response")
		}
		http.Redirect(w, r, "/error?reason=auth_failed", http.StatusFound)
		return
	}

	// Check for MCP session — if present, complete the OAuth flow instead of normal login
	if mcpRedirect := h.tryCompleteMCPSession(w, r, result.Data.Token); mcpRedirect != "" {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "vire_session",
		Value:    result.Data.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// HandleGoogleLogin redirects to vire-server's Google OAuth endpoint.
// GET /api/auth/login/google -> 302 to {apiURL}/api/auth/login/google?callback={callbackURL}
func (h *AuthHandler) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	redirectURL := h.apiURL + "/api/auth/login/google?callback=" + h.callbackURL
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleGitHubLogin redirects to vire-server's GitHub OAuth endpoint.
// GET /api/auth/login/github -> 302 to {apiURL}/api/auth/login/github?callback={callbackURL}
func (h *AuthHandler) HandleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	redirectURL := h.apiURL + "/api/auth/login/github?callback=" + h.callbackURL
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleOAuthCallback handles the OAuth callback from vire-server.
// GET /auth/callback?token=<jwt> -> sets vire_session cookie, redirects to /dashboard.
// If mcp_session_id cookie is present, completes the MCP OAuth flow instead.
func (h *AuthHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/error?reason=auth_failed", http.StatusFound)
		return
	}

	// Check for MCP session — if present, complete the OAuth flow
	if mcpRedirect := h.tryCompleteMCPSession(w, r, token); mcpRedirect != "" {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "vire_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// tryCompleteMCPSession checks for an mcp_session_id cookie and, if present,
// completes the MCP OAuth flow by exchanging the session for an auth code redirect.
// Returns the redirect URL if MCP flow was completed, empty string otherwise.
func (h *AuthHandler) tryCompleteMCPSession(w http.ResponseWriter, r *http.Request, token string) string {
	if h.oauthServer == nil {
		return ""
	}

	mcpCookie, err := r.Cookie("mcp_session_id")
	if err != nil || mcpCookie.Value == "" {
		return ""
	}

	// Extract user ID from the JWT token
	sub := extractJWTSubFromToken(token)
	if sub == "" {
		if h.logger != nil {
			h.logger.Error().Msg("MCP session: failed to extract user ID from token")
		}
		return ""
	}

	redirectURL, err := h.oauthServer.CompleteAuthorization(mcpCookie.Value, sub)
	if err != nil {
		if h.logger != nil {
			h.logger.Error().Str("error", err.Error()).Msg("MCP session: failed to complete authorization")
		}
		return ""
	}

	// Clear the mcp_session_id cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "mcp_session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, redirectURL, http.StatusFound)
	return redirectURL
}

// extractJWTSubFromToken extracts the "sub" claim from a JWT without full validation.
// Used during MCP flow to get user ID from vire-server token.
func extractJWTSubFromToken(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}

	return claims.Sub
}

// HandleLogout clears the session cookie and redirects to the landing page.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "vire_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}
