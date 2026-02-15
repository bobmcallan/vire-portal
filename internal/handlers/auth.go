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

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	logger      *common.Logger
	devMode     bool
	apiURL      string
	callbackURL string
	jwtSecret   []byte
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

// HandleDevLogin handles the dev-mode login shortcut.
// In dev mode, it calls vire-server POST /api/auth/oauth with provider:"dev",
// sets the returned JWT as a session cookie, and redirects to /dashboard.
// In prod mode, it returns 404.
func (h *AuthHandler) HandleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	// POST to vire-server /api/auth/oauth
	body := map[string]string{
		"provider": "dev",
		"code":     "dev",
		"state":    "dev",
	}
	bodyJSON, _ := json.Marshal(body)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(h.apiURL+"/api/auth/oauth", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		if h.logger != nil {
			h.logger.Error().Str("error", err.Error()).Msg("failed to reach vire-server for dev login")
		}
		http.Redirect(w, r, "/?error=auth_failed", http.StatusFound)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if h.logger != nil {
			h.logger.Error().Str("error", err.Error()).Msg("failed to read vire-server response")
		}
		http.Redirect(w, r, "/?error=auth_failed", http.StatusFound)
		return
	}

	if resp.StatusCode != http.StatusOK {
		if h.logger != nil {
			h.logger.Error().Int("status", resp.StatusCode).Str("body", string(respBody)).Msg("vire-server dev login failed")
		}
		http.Redirect(w, r, "/?error=auth_failed", http.StatusFound)
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
		http.Redirect(w, r, "/?error=auth_failed", http.StatusFound)
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
func (h *AuthHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/?error=missing_token", http.StatusFound)
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

// HandleLogout clears the session cookie and redirects to the landing page.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "vire_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}
