package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

// HandleToken handles POST /token — token exchange endpoint.
func (s *OAuthServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		s.handleAuthCodeGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
	}
}

func (s *OAuthServer) handleAuthCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")

	if code == "" || clientID == "" || redirectURI == "" || codeVerifier == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "missing required parameters")
		return
	}

	authCode, ok := s.codes.Get(code)
	if !ok {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization code not found or expired")
		return
	}

	if authCode.Used {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization code already used")
		return
	}

	s.codes.MarkUsed(code)

	if authCode.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}

	if authCode.RedirectURI != redirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}

	if !VerifyPKCE(codeVerifier, authCode.CodeChallenge) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	issuer := baseURLFromRequest(r)
	accessToken, err := s.mintAccessToken(authCode.UserID, authCode.Scope, authCode.ClientID, issuer)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "failed to mint access token")
		return
	}

	refreshTokenStr, err := generateUUID()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "failed to generate refresh token")
		return
	}

	s.tokens.Put(&RefreshToken{
		Token:     refreshTokenStr,
		UserID:    authCode.UserID,
		ClientID:  authCode.ClientID,
		Scope:     authCode.Scope,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refreshTokenStr,
		"scope":         authCode.Scope,
	})
}

func (s *OAuthServer) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshTokenStr := r.FormValue("refresh_token")
	clientID := r.FormValue("client_id")

	if refreshTokenStr == "" || clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "missing required parameters")
		return
	}

	refreshToken, ok := s.tokens.Get(refreshTokenStr)
	if !ok {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "refresh token not found or expired")
		return
	}

	if refreshToken.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}

	// Rotate: delete old, create new
	s.tokens.Delete(refreshTokenStr)

	accessToken, err := s.mintAccessToken(refreshToken.UserID, refreshToken.Scope, refreshToken.ClientID, baseURLFromRequest(r))
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "failed to mint access token")
		return
	}

	newRefreshTokenStr, err := generateUUID()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "failed to generate refresh token")
		return
	}

	s.tokens.Put(&RefreshToken{
		Token:     newRefreshTokenStr,
		UserID:    refreshToken.UserID,
		ClientID:  refreshToken.ClientID,
		Scope:     refreshToken.Scope,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": newRefreshTokenStr,
		"scope":         refreshToken.Scope,
	})
}
