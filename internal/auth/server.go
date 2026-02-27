package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// OAuthServer holds all state for the MCP OAuth 2.1 Authorization Server.
type OAuthServer struct {
	baseURL   string
	jwtSecret []byte
	clients   *ClientStore
	sessions  *SessionStore
	codes     *CodeStore
	tokens    *TokenStore
	logger    *common.Logger
}

// NewOAuthServer creates a new OAuthServer with the given base URL and JWT secret.
// If apiURL is non-empty, a backend is created for write-through/read-through
// persistence to vire-server's internal OAuth API.
func NewOAuthServer(baseURL, apiURL string, jwtSecret []byte, logger *common.Logger) *OAuthServer {
	s := &OAuthServer{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		jwtSecret: jwtSecret,
		clients:   NewClientStore(),
		sessions:  NewSessionStore(),
		codes:     NewCodeStore(),
		tokens:    NewTokenStore(),
		logger:    logger,
	}

	if apiURL != "" {
		backend := NewOAuthBackend(apiURL, logger)
		s.clients.SetBackend(backend)
		s.sessions.SetBackend(backend)
		s.codes.SetBackend(backend)
		s.tokens.SetBackend(backend)
	}

	return s
}

// CompleteAuthorization looks up a pending session, creates an authorization code,
// stores it, deletes the session, and returns the redirect URL with code and state.
func (s *OAuthServer) CompleteAuthorization(sessionID, userID string) (string, error) {
	sess, ok := s.sessions.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found or expired")
	}

	code, err := generateRandomHex(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate auth code: %w", err)
	}

	authCode := &AuthCode{
		Code:          code,
		ClientID:      sess.ClientID,
		UserID:        userID,
		RedirectURI:   sess.RedirectURI,
		CodeChallenge: sess.CodeChallenge,
		Scope:         sess.Scope,
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	}
	s.codes.Put(authCode)
	s.sessions.Delete(sessionID)

	redirectURL, err := url.Parse(sess.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("invalid redirect URI: %w", err)
	}
	q := redirectURL.Query()
	q.Set("code", code)
	q.Set("state", sess.State)
	redirectURL.RawQuery = q.Encode()

	return redirectURL.String(), nil
}

// mintAccessToken creates a signed JWT access token.
// issuer is the token issuer URL, derived from the request's Host header.
func (s *OAuthServer) mintAccessToken(userID, scope, clientID, issuer string) (string, error) {
	now := time.Now()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claims := map[string]interface{}{
		"sub":       userID,
		"scope":     scope,
		"client_id": clientID,
		"iss":       issuer,
		"iat":       now.Unix(),
		"exp":       now.Add(1 * time.Hour).Unix(),
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT claims: %w", err)
	}

	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	sigInput := header + "." + payload
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + sig, nil
}

// generateRandomHex generates a random hex string of the given byte length.
func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateUUID generates a random UUID v4 string.
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
