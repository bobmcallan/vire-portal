package auth

import (
	"sync"
	"time"
)

// OAuthClient represents a dynamically registered OAuth client.
type OAuthClient struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	CreatedAt               int64    `json:"client_id_issued_at,omitempty"`
}

// ClientStore holds DCR-registered OAuth clients.
type ClientStore struct {
	mu      sync.RWMutex
	clients map[string]*OAuthClient
}

// NewClientStore creates a new empty ClientStore.
func NewClientStore() *ClientStore {
	return &ClientStore{clients: make(map[string]*OAuthClient)}
}

// Put stores a client keyed by its ClientID.
func (s *ClientStore) Put(client *OAuthClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ClientID] = client
}

// Get retrieves a client by ID.
func (s *ClientStore) Get(clientID string) (*OAuthClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clients[clientID]
	return c, ok
}

// AuthCode represents an issued authorization code.
type AuthCode struct {
	Code          string
	ClientID      string
	UserID        string
	RedirectURI   string
	CodeChallenge string
	Scope         string
	ExpiresAt     time.Time
	Used          bool
}

// CodeStore holds issued authorization codes. Codes expire after 5 minutes and are single-use.
type CodeStore struct {
	mu    sync.RWMutex
	codes map[string]*AuthCode
}

// NewCodeStore creates a new empty CodeStore.
func NewCodeStore() *CodeStore {
	return &CodeStore{codes: make(map[string]*AuthCode)}
}

// Put stores an authorization code.
func (s *CodeStore) Put(code *AuthCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code.Code] = code
}

// Get retrieves an authorization code. Returns false if not found or expired.
func (s *CodeStore) Get(code string) (*AuthCode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.codes[code]
	if !ok {
		return nil, false
	}
	if time.Now().After(c.ExpiresAt) {
		return nil, false
	}
	return c, true
}

// MarkUsed marks an authorization code as used.
func (s *CodeStore) MarkUsed(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.codes[code]; ok {
		c.Used = true
	}
}

// Cleanup removes expired codes.
func (s *CodeStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, c := range s.codes {
		if now.After(c.ExpiresAt) {
			delete(s.codes, k)
		}
	}
}

// RefreshToken represents an issued refresh token.
type RefreshToken struct {
	Token     string
	UserID    string
	ClientID  string
	Scope     string
	ExpiresAt time.Time
}

// TokenStore holds refresh tokens. Tokens expire after 7 days.
type TokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*RefreshToken
}

// NewTokenStore creates a new empty TokenStore.
func NewTokenStore() *TokenStore {
	return &TokenStore{tokens: make(map[string]*RefreshToken)}
}

// Put stores a refresh token.
func (s *TokenStore) Put(token *RefreshToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Token] = token
}

// Get retrieves a refresh token. Returns false if not found or expired.
func (s *TokenStore) Get(token string) (*RefreshToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tokens[token]
	if !ok {
		return nil, false
	}
	if time.Now().After(t.ExpiresAt) {
		return nil, false
	}
	return t, true
}

// Delete removes a refresh token.
func (s *TokenStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

// Cleanup removes expired tokens.
func (s *TokenStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, t := range s.tokens {
		if now.After(t.ExpiresAt) {
			delete(s.tokens, k)
		}
	}
}
