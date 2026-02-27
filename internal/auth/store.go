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
	backend *OAuthBackend
}

// NewClientStore creates a new empty ClientStore.
func NewClientStore() *ClientStore {
	return &ClientStore{clients: make(map[string]*OAuthClient)}
}

// SetBackend configures the backend for write-through/read-through persistence.
func (s *ClientStore) SetBackend(b *OAuthBackend) {
	s.backend = b
}

// Put stores a client keyed by its ClientID.
func (s *ClientStore) Put(client *OAuthClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ClientID] = client

	if s.backend != nil {
		if err := s.backend.SaveClient(client); err != nil {
			s.logWarn("SaveClient", err)
		}
	}
}

// Get retrieves a client by ID.
func (s *ClientStore) Get(clientID string) (*OAuthClient, bool) {
	s.mu.RLock()
	c, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok && s.backend != nil {
		var err error
		c, err = s.backend.GetClient(clientID)
		if err != nil {
			s.logWarn("GetClient", err)
			return nil, false
		}
		if c == nil {
			return nil, false
		}
		// Cache locally
		s.mu.Lock()
		s.clients[c.ClientID] = c
		s.mu.Unlock()
		return c, true
	}

	return c, ok
}

// logWarn logs a backend error if a logger is available.
func (s *ClientStore) logWarn(method string, err error) {
	if s.backend != nil && s.backend.logger != nil {
		s.backend.logger.Warn().Err(err).Str("method", method).Msg("client backend error")
	}
}

// AuthCode represents an issued authorization code.
type AuthCode struct {
	Code          string    `json:"code"`
	ClientID      string    `json:"client_id"`
	UserID        string    `json:"user_id"`
	RedirectURI   string    `json:"redirect_uri"`
	CodeChallenge string    `json:"code_challenge"`
	Scope         string    `json:"scope"`
	ExpiresAt     time.Time `json:"expires_at"`
	Used          bool      `json:"used"`
}

// CodeStore holds issued authorization codes. Codes expire after 5 minutes and are single-use.
type CodeStore struct {
	mu      sync.RWMutex
	codes   map[string]*AuthCode
	backend *OAuthBackend
}

// NewCodeStore creates a new empty CodeStore.
func NewCodeStore() *CodeStore {
	return &CodeStore{codes: make(map[string]*AuthCode)}
}

// SetBackend configures the backend for write-through/read-through persistence.
func (s *CodeStore) SetBackend(b *OAuthBackend) {
	s.backend = b
}

// Put stores an authorization code.
func (s *CodeStore) Put(code *AuthCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code.Code] = code

	if s.backend != nil {
		if err := s.backend.SaveCode(code); err != nil {
			s.logWarn("SaveCode", err)
		}
	}
}

// Get retrieves an authorization code. Returns false if not found or expired.
func (s *CodeStore) Get(code string) (*AuthCode, bool) {
	s.mu.RLock()
	c, ok := s.codes[code]
	s.mu.RUnlock()

	if !ok && s.backend != nil {
		var err error
		c, err = s.backend.GetCode(code)
		if err != nil {
			s.logWarn("GetCode", err)
			return nil, false
		}
		if c == nil {
			return nil, false
		}
		// Cache locally
		s.mu.Lock()
		s.codes[c.Code] = c
		s.mu.Unlock()
		ok = true
	}

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

	if s.backend != nil {
		if err := s.backend.MarkCodeUsed(code); err != nil {
			s.logWarn("MarkCodeUsed", err)
		}
	}
}

// ConsumeCode atomically retrieves and marks a code as used in a single operation.
// Returns the code and true only if it exists, is not expired, and was not already used.
// This prevents TOCTOU races between Get() and MarkUsed().
func (s *CodeStore) ConsumeCode(code string) (*AuthCode, bool) {
	s.mu.Lock()
	c, ok := s.codes[code]

	if !ok && s.backend != nil {
		s.mu.Unlock()
		var err error
		c, err = s.backend.GetCode(code)
		if err != nil {
			s.logWarn("GetCode", err)
			return nil, false
		}
		if c == nil {
			return nil, false
		}
		s.mu.Lock()
		// Re-check: another goroutine may have cached it
		if existing, exists := s.codes[code]; exists {
			c = existing
		} else {
			s.codes[code] = c
		}
		ok = true
	}

	if !ok {
		s.mu.Unlock()
		return nil, false
	}
	if time.Now().After(c.ExpiresAt) {
		s.mu.Unlock()
		return nil, false
	}
	if c.Used {
		s.mu.Unlock()
		return nil, false
	}

	c.Used = true
	s.mu.Unlock()

	if s.backend != nil {
		if err := s.backend.MarkCodeUsed(code); err != nil {
			s.logWarn("MarkCodeUsed", err)
		}
	}

	return c, true
}

// logWarn logs a backend error if a logger is available.
func (s *CodeStore) logWarn(method string, err error) {
	if s.backend != nil && s.backend.logger != nil {
		s.backend.logger.Warn().Err(err).Str("method", method).Msg("code backend error")
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
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	ClientID  string    `json:"client_id"`
	Scope     string    `json:"scope"`
	ExpiresAt time.Time `json:"expires_at"`
}

// TokenStore holds refresh tokens. Tokens expire after 7 days.
type TokenStore struct {
	mu      sync.RWMutex
	tokens  map[string]*RefreshToken
	backend *OAuthBackend
}

// NewTokenStore creates a new empty TokenStore.
func NewTokenStore() *TokenStore {
	return &TokenStore{tokens: make(map[string]*RefreshToken)}
}

// SetBackend configures the backend for write-through/read-through persistence.
func (s *TokenStore) SetBackend(b *OAuthBackend) {
	s.backend = b
}

// Put stores a refresh token.
func (s *TokenStore) Put(token *RefreshToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Token] = token

	if s.backend != nil {
		if err := s.backend.SaveToken(token); err != nil {
			s.logWarn("SaveToken", err)
		}
	}
}

// Get retrieves a refresh token. Returns false if not found or expired.
func (s *TokenStore) Get(token string) (*RefreshToken, bool) {
	s.mu.RLock()
	t, ok := s.tokens[token]
	s.mu.RUnlock()

	if !ok && s.backend != nil {
		var err error
		t, err = s.backend.LookupToken(token)
		if err != nil {
			s.logWarn("LookupToken", err)
			return nil, false
		}
		if t == nil {
			return nil, false
		}
		// Cache locally
		s.mu.Lock()
		s.tokens[t.Token] = t
		s.mu.Unlock()
		ok = true
	}

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

	if s.backend != nil {
		if err := s.backend.RevokeToken(token); err != nil {
			s.logWarn("RevokeToken", err)
		}
	}
}

// logWarn logs a backend error if a logger is available.
func (s *TokenStore) logWarn(method string, err error) {
	if s.backend != nil && s.backend.logger != nil {
		s.backend.logger.Warn().Err(err).Str("method", method).Msg("token backend error")
	}
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
