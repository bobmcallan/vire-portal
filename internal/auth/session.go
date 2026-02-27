package auth

import (
	"sync"
	"time"
)

// sessionTTL is the time-to-live for pending MCP authorization sessions.
const sessionTTL = 10 * time.Minute

// AuthSession represents a pending MCP authorization session.
type AuthSession struct {
	SessionID     string    `json:"session_id"`
	ClientID      string    `json:"client_id"`
	RedirectURI   string    `json:"redirect_uri"`
	State         string    `json:"state"`
	CodeChallenge string    `json:"code_challenge"`
	CodeMethod    string    `json:"code_method"`
	Scope         string    `json:"scope"`
	CreatedAt     time.Time `json:"created_at"`
	UserID        string    `json:"user_id,omitempty"` // filled after login
}

// SessionStore holds pending MCP authorization sessions.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*AuthSession
	backend  *OAuthBackend
}

// NewSessionStore creates a new empty SessionStore.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*AuthSession)}
}

// SetBackend configures the backend for write-through/read-through persistence.
func (s *SessionStore) SetBackend(b *OAuthBackend) {
	s.backend = b
}

// Put stores a session keyed by its SessionID.
func (s *SessionStore) Put(session *AuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.SessionID] = session

	if s.backend != nil {
		if err := s.backend.SaveSession(session); err != nil {
			s.logWarn("SaveSession", err)
		}
	}
}

// Get retrieves a session by ID. Returns false if not found or expired.
func (s *SessionStore) Get(sessionID string) (*AuthSession, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok && s.backend != nil {
		var err error
		sess, err = s.backend.GetSession(sessionID)
		if err != nil {
			s.logWarn("GetSession", err)
			return nil, false
		}
		if sess == nil {
			return nil, false
		}
		// Cache locally
		s.mu.Lock()
		s.sessions[sess.SessionID] = sess
		s.mu.Unlock()
		ok = true
	}

	if !ok {
		return nil, false
	}
	if time.Now().After(sess.CreatedAt.Add(sessionTTL)) {
		return nil, false
	}
	return sess, true
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)

	if s.backend != nil {
		if err := s.backend.DeleteSession(sessionID); err != nil {
			s.logWarn("DeleteSession", err)
		}
	}
}

// GetByClientID returns the most recently created pending session for the
// given client ID, or nil if none exists. This supports the flow where a
// programmatic client (vire-mcp) creates a session via POST /authorize,
// then the browser opens GET /authorize?client_id=xxx with a truncated URL.
func (s *SessionStore) GetByClientID(clientID string) *AuthSession {
	s.mu.RLock()
	now := time.Now()
	var best *AuthSession
	for _, sess := range s.sessions {
		if sess.ClientID != clientID {
			continue
		}
		if now.After(sess.CreatedAt.Add(sessionTTL)) {
			continue
		}
		if best == nil || sess.CreatedAt.After(best.CreatedAt) {
			best = sess
		}
	}
	s.mu.RUnlock()

	if best == nil && s.backend != nil {
		sess, err := s.backend.GetSessionByClientID(clientID)
		if err != nil {
			s.logWarn("GetSessionByClientID", err)
			return nil
		}
		if sess != nil && !now.After(sess.CreatedAt.Add(sessionTTL)) {
			// Cache locally
			s.mu.Lock()
			s.sessions[sess.SessionID] = sess
			s.mu.Unlock()
			best = sess
		}
	}

	return best
}

// logWarn logs a backend error if a logger is available.
func (s *SessionStore) logWarn(method string, err error) {
	if s.backend != nil && s.backend.logger != nil {
		s.backend.logger.Warn().Err(err).Str("method", method).Msg("session backend error")
	}
}

// Cleanup removes expired sessions.
func (s *SessionStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, sess := range s.sessions {
		if now.After(sess.CreatedAt.Add(sessionTTL)) {
			delete(s.sessions, k)
		}
	}
}
