package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// OAuthBackend is an HTTP client for vire-server's internal OAuth API.
// It provides write-through and read-through persistence for OAuth stores.
type OAuthBackend struct {
	apiURL string
	client *http.Client
	logger *common.Logger
}

// NewOAuthBackend creates a new OAuthBackend targeting the given vire-server URL.
func NewOAuthBackend(apiURL string, logger *common.Logger) *OAuthBackend {
	return &OAuthBackend{
		apiURL: apiURL,
		client: &http.Client{Timeout: 5 * time.Second},
		logger: logger,
	}
}

// SaveSession persists a session to the backend.
func (b *OAuthBackend) SaveSession(sess *AuthSession) error {
	return b.postJSON("/api/internal/oauth/sessions", sess)
}

// GetSession retrieves a session by ID from the backend.
// Returns nil, nil if not found.
func (b *OAuthBackend) GetSession(sessionID string) (*AuthSession, error) {
	var sess AuthSession
	found, err := b.getJSON(fmt.Sprintf("/api/internal/oauth/sessions/%s", sessionID), &sess)
	if err != nil || !found {
		return nil, err
	}
	return &sess, nil
}

// GetSessionByClientID retrieves a session by client ID from the backend.
// Returns nil, nil if not found.
func (b *OAuthBackend) GetSessionByClientID(clientID string) (*AuthSession, error) {
	var sess AuthSession
	found, err := b.getJSON(fmt.Sprintf("/api/internal/oauth/sessions?client_id=%s", clientID), &sess)
	if err != nil || !found {
		return nil, err
	}
	return &sess, nil
}

// UpdateSessionUserID updates the user_id on a session in the backend.
func (b *OAuthBackend) UpdateSessionUserID(sessionID, userID string) error {
	body := map[string]string{"user_id": userID}
	return b.patchJSON(fmt.Sprintf("/api/internal/oauth/sessions/%s", sessionID), body)
}

// DeleteSession removes a session from the backend.
func (b *OAuthBackend) DeleteSession(sessionID string) error {
	return b.doRequest(http.MethodDelete, fmt.Sprintf("/api/internal/oauth/sessions/%s", sessionID), nil)
}

// SaveClient persists a client to the backend.
func (b *OAuthBackend) SaveClient(client *OAuthClient) error {
	return b.postJSON("/api/internal/oauth/clients", client)
}

// GetClient retrieves a client by ID from the backend.
// Returns nil, nil if not found.
func (b *OAuthBackend) GetClient(clientID string) (*OAuthClient, error) {
	var client OAuthClient
	found, err := b.getJSON(fmt.Sprintf("/api/internal/oauth/clients/%s", clientID), &client)
	if err != nil || !found {
		return nil, err
	}
	return &client, nil
}

// SaveCode persists an authorization code to the backend.
func (b *OAuthBackend) SaveCode(code *AuthCode) error {
	return b.postJSON("/api/internal/oauth/codes", code)
}

// GetCode retrieves an authorization code from the backend.
// Returns nil, nil if not found.
func (b *OAuthBackend) GetCode(code string) (*AuthCode, error) {
	var authCode AuthCode
	found, err := b.getJSON(fmt.Sprintf("/api/internal/oauth/codes/%s", code), &authCode)
	if err != nil || !found {
		return nil, err
	}
	return &authCode, nil
}

// MarkCodeUsed marks an authorization code as used in the backend.
func (b *OAuthBackend) MarkCodeUsed(code string) error {
	return b.patchJSON(fmt.Sprintf("/api/internal/oauth/codes/%s/used", code), nil)
}

// SaveToken persists a refresh token to the backend.
func (b *OAuthBackend) SaveToken(token *RefreshToken) error {
	return b.postJSON("/api/internal/oauth/tokens", token)
}

// LookupToken looks up a refresh token by plaintext from the backend.
// Returns nil, nil if not found.
func (b *OAuthBackend) LookupToken(plaintext string) (*RefreshToken, error) {
	body := map[string]string{"token": plaintext}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal token lookup: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, b.apiURL+"/api/internal/oauth/tokens/lookup", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("backend request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned %d", resp.StatusCode)
	}

	var token RefreshToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &token, nil
}

// RevokeToken revokes a refresh token in the backend.
func (b *OAuthBackend) RevokeToken(plaintext string) error {
	body := map[string]string{"token": plaintext}
	return b.postJSON("/api/internal/oauth/tokens/revoke", body)
}

// postJSON sends a POST request with JSON body to the backend.
func (b *OAuthBackend) postJSON(path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	return b.doRequest(http.MethodPost, path, data)
}

// patchJSON sends a PATCH request with JSON body to the backend.
func (b *OAuthBackend) patchJSON(path string, body interface{}) error {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
	}
	return b.doRequest(http.MethodPatch, path, data)
}

// getJSON sends a GET request and decodes the JSON response into target.
// Returns (false, nil) if the backend returns 404.
func (b *OAuthBackend) getJSON(path string, target interface{}) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, b.apiURL+path, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("backend request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("backend returned %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}
	return true, nil
}

// doRequest sends an HTTP request with the given method, path, and optional body.
func (b *OAuthBackend) doRequest(method, path string, body []byte) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, b.apiURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("backend request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("backend returned %d", resp.StatusCode)
	}
	return nil
}
