package auth

import (
	"sync"
	"time"
)

// sessionTTL is the time-to-live for pending MCP authorization sessions.
const sessionTTL = 10 * time.Minute

// AuthSession represents a pending MCP authorization session.
type AuthSession struct {
	SessionID     string
	ClientID      string
	RedirectURI   string
	State         string
	CodeChallenge string
	CodeMethod    string
	Scope         string
	CreatedAt     time.Time
	UserID        string // filled after login
}

// SessionStore holds pending MCP authorization sessions.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*AuthSession
}

// NewSessionStore creates a new empty SessionStore.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*AuthSession)}
}

// Put stores a session keyed by its SessionID.
func (s *SessionStore) Put(session *AuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.SessionID] = session
}

// Get retrieves a session by ID. Returns false if not found or expired.
func (s *SessionStore) Get(sessionID string) (*AuthSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
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
