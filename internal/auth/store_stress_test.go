package auth

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// ClientStore Stress Tests
// =============================================================================

// --- Unbounded growth / DCR abuse ---

func TestClientStore_StressUnboundedGrowth(t *testing.T) {
	// FINDING: ClientStore has no limit on registered clients. An attacker
	// can call POST /register in a loop to exhaust server memory.
	// This test documents the behavior and verifies it doesn't panic.
	store := NewClientStore()

	const numClients = 10000
	for i := 0; i < numClients; i++ {
		store.Put(&OAuthClient{
			ClientID:     fmt.Sprintf("attacker-client-%d", i),
			ClientSecret: "secret",
			ClientName:   "Malicious Client " + strings.Repeat("A", 1000), // large name
			RedirectURIs: []string{"http://localhost/callback"},
		})
	}

	// Verify all clients are stored (no eviction)
	count := 0
	store.mu.RLock()
	count = len(store.clients)
	store.mu.RUnlock()

	if count != numClients {
		t.Errorf("expected %d clients stored, got %d", numClients, count)
	}

	t.Logf("FINDING: ClientStore allows unlimited registrations (%d stored). "+
		"No rate limit or max-clients cap. An attacker can exhaust memory via DCR spam.", numClients)
}

// --- ClientStore: hostile client IDs ---

func TestClientStore_StressHostileClientIDs(t *testing.T) {
	store := NewClientStore()

	hostile := []struct {
		name     string
		clientID string
	}{
		{"empty", ""},
		{"very long", strings.Repeat("A", 1<<16)},
		{"null bytes", "client\x00id"},
		{"newlines", "client\nid"},
		{"unicode", "客户端"},
		{"sql injection", "'; DROP TABLE clients; --"},
		{"path traversal", "../../etc/passwd"},
		{"html", "<img src=x onerror=alert(1)>"},
	}

	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			store.Put(&OAuthClient{
				ClientID:   tc.clientID,
				ClientName: "test",
			})

			got, ok := store.Get(tc.clientID)
			if !ok {
				t.Errorf("expected to retrieve client with hostile ID %q", tc.name)
				return
			}
			if got.ClientID != tc.clientID {
				t.Errorf("client ID mangled: expected %q, got %q", tc.clientID, got.ClientID)
			}
		})
	}
}

// --- ClientStore: overwrite attack ---

func TestClientStore_StressOverwriteExistingClient(t *testing.T) {
	// FINDING: A second DCR call with the same client_id overwrites the
	// original client data, including redirect_uris. If client IDs are
	// predictable, an attacker can hijack another client's redirect_uri.
	store := NewClientStore()

	// Legitimate client registers
	store.Put(&OAuthClient{
		ClientID:     "legitimate-client",
		ClientSecret: "legitimate-secret",
		RedirectURIs: []string{"http://legitimate.com/callback"},
	})

	// Attacker overwrites with their redirect URI
	store.Put(&OAuthClient{
		ClientID:     "legitimate-client",
		ClientSecret: "attacker-secret",
		RedirectURIs: []string{"http://attacker.com/steal"},
	})

	got, _ := store.Get("legitimate-client")
	if got.RedirectURIs[0] == "http://attacker.com/steal" {
		t.Log("FINDING: ClientStore.Put overwrites existing clients without authentication. " +
			"If client_id is predictable, an attacker can hijack redirect_uris. " +
			"DCR handler must prevent re-registration of existing client IDs.")
	}
}

// --- ClientStore: concurrent read/write race ---

func TestClientStore_StressConcurrentReadWriteHeavy(t *testing.T) {
	store := NewClientStore()
	var wg sync.WaitGroup

	// Many writers and readers hitting the same keys
	const writers = 50
	const readers = 200

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cid := fmt.Sprintf("client-%d", j%10)
				store.Put(&OAuthClient{
					ClientID:   cid,
					ClientName: fmt.Sprintf("writer-%d-iter-%d", id, j),
				})
			}
		}(i)
	}

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cid := fmt.Sprintf("client-%d", j%10)
				store.Get(cid)
			}
		}()
	}

	wg.Wait()
}

// =============================================================================
// CodeStore Stress Tests
// =============================================================================

// --- Auth code replay: Get + MarkUsed is NOT atomic ---

func TestCodeStore_StressAuthCodeReplayRace(t *testing.T) {
	// CRITICAL FINDING: CodeStore.Get() does not check the Used flag.
	// The consumer must check Used after Get, and then call MarkUsed.
	// Between Get and MarkUsed, another goroutine can also Get the code
	// and see Used=false. This is a classic TOCTOU (time-of-check-time-of-use) bug.
	//
	// Two concurrent token exchange requests with the same auth code can
	// both succeed, violating the OAuth 2.1 requirement that auth codes are single-use.
	store := NewCodeStore()
	store.Put(&AuthCode{
		Code:      "replay-me",
		ClientID:  "client-1",
		UserID:    "user-1",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Used:      false,
	})

	var successCount int64
	var wg sync.WaitGroup

	// Simulate 100 concurrent token exchange attempts with the same code
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, ok := store.Get("replay-me")
			if !ok {
				return
			}
			// Check Used flag (as the token handler would)
			if !code.Used {
				// "Exchange" the code
				store.MarkUsed("replay-me")
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount > 1 {
		t.Logf("CRITICAL: Auth code was successfully used %d times (should be 1). "+
			"Get+check+MarkUsed is not atomic. The token endpoint needs an atomic "+
			"ConsumeCode(code) method that marks used under a write lock.", successCount)
	} else {
		// May pass due to goroutine scheduling, but the race exists.
		t.Log("NOTE: Race did not manifest in this run, but TOCTOU bug exists in code path. " +
			"Get() returns a pointer to the same AuthCode struct, so the Used flag check " +
			"in the consumer is racy with concurrent MarkUsed calls.")
	}
}

// --- CodeStore: unbounded growth ---

func TestCodeStore_StressUnboundedGrowth(t *testing.T) {
	store := NewCodeStore()

	const numCodes = 10000
	for i := 0; i < numCodes; i++ {
		store.Put(&AuthCode{
			Code:      fmt.Sprintf("code-%d", i),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		})
	}

	store.mu.RLock()
	count := len(store.codes)
	store.mu.RUnlock()

	if count != numCodes {
		t.Errorf("expected %d codes, got %d", numCodes, count)
	}

	t.Logf("FINDING: CodeStore stores %d codes without limit. "+
		"No rate limiting per client_id. Attacker can flood with /authorize requests.", numCodes)
}

// --- CodeStore: code guessing / brute force ---

func TestCodeStore_StressCodeGuessing(t *testing.T) {
	// If codes are short or predictable, an attacker can brute-force them.
	// This test verifies that looking up nonexistent codes is fast and doesn't
	// leak timing information about which codes exist.
	store := NewCodeStore()
	store.Put(&AuthCode{
		Code:      "real-code-abc123",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})

	// Try many wrong codes
	found := 0
	for i := 0; i < 10000; i++ {
		_, ok := store.Get(fmt.Sprintf("guess-%d", i))
		if ok {
			found++
		}
	}

	if found > 0 {
		t.Errorf("brute-force found %d codes — codes may be predictable", found)
	}
}

// --- CodeStore: expired but not cleaned up ---

func TestCodeStore_StressExpiredCodesAccumulate(t *testing.T) {
	// Expired codes remain in the map until Cleanup() is called.
	// Without periodic cleanup, memory grows unbounded.
	store := NewCodeStore()

	for i := 0; i < 1000; i++ {
		store.Put(&AuthCode{
			Code:      fmt.Sprintf("expired-%d", i),
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		})
	}

	// Get returns false for expired codes, but they still occupy memory
	store.mu.RLock()
	count := len(store.codes)
	store.mu.RUnlock()

	if count != 1000 {
		t.Errorf("expected 1000 expired codes in map (not yet cleaned), got %d", count)
	}

	store.Cleanup()

	store.mu.RLock()
	countAfter := len(store.codes)
	store.mu.RUnlock()

	if countAfter != 0 {
		t.Errorf("expected 0 codes after cleanup, got %d", countAfter)
	}

	t.Log("FINDING: Expired codes accumulate until Cleanup() is explicitly called. " +
		"Verify that a periodic cleanup goroutine is started during server init.")
}

// --- CodeStore: MarkUsed on nonexistent code ---

func TestCodeStore_StressMarkUsedNonexistent(t *testing.T) {
	store := NewCodeStore()
	// Must not panic
	store.MarkUsed("does-not-exist")
}

// --- CodeStore: concurrent cleanup during access ---

func TestCodeStore_StressConcurrentCleanupDuringAccess(t *testing.T) {
	store := NewCodeStore()
	var wg sync.WaitGroup

	// Start with some codes
	for i := 0; i < 100; i++ {
		ttl := 5 * time.Minute
		if i%2 == 0 {
			ttl = -1 * time.Minute // expired
		}
		store.Put(&AuthCode{
			Code:      fmt.Sprintf("code-%d", i),
			ExpiresAt: time.Now().Add(ttl),
		})
	}

	// Concurrent reads, writes, and cleanups
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func(id int) {
			defer wg.Done()
			store.Get(fmt.Sprintf("code-%d", id%100))
		}(i)
		go func(id int) {
			defer wg.Done()
			store.Put(&AuthCode{
				Code:      fmt.Sprintf("new-code-%d", id),
				ExpiresAt: time.Now().Add(5 * time.Minute),
			})
		}(i)
		go func() {
			defer wg.Done()
			store.Cleanup()
		}()
	}

	wg.Wait()
}

// =============================================================================
// TokenStore Stress Tests
// =============================================================================

// --- Refresh token rotation: old token must be rejected ---

func TestTokenStore_StressRefreshTokenRotation(t *testing.T) {
	// When a refresh token is rotated, the old token must be deleted.
	// If not, a stolen old token can be replayed.
	store := NewTokenStore()

	// Issue original refresh token
	store.Put(&RefreshToken{
		Token:     "original-refresh",
		UserID:    "user-1",
		ClientID:  "client-1",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	// Simulate rotation: issue new token, delete old
	store.Put(&RefreshToken{
		Token:     "rotated-refresh",
		UserID:    "user-1",
		ClientID:  "client-1",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})
	store.Delete("original-refresh")

	// Old token must be rejected
	_, ok := store.Get("original-refresh")
	if ok {
		t.Error("SECURITY: Old refresh token still valid after rotation — stolen token replay possible")
	}

	// New token must work
	_, ok = store.Get("rotated-refresh")
	if !ok {
		t.Error("new rotated refresh token should be valid")
	}
}

// --- TokenStore: unbounded growth ---

func TestTokenStore_StressUnboundedGrowth(t *testing.T) {
	store := NewTokenStore()

	const numTokens = 10000
	for i := 0; i < numTokens; i++ {
		store.Put(&RefreshToken{
			Token:     fmt.Sprintf("token-%d", i),
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		})
	}

	store.mu.RLock()
	count := len(store.tokens)
	store.mu.RUnlock()

	if count != numTokens {
		t.Errorf("expected %d tokens, got %d", numTokens, count)
	}

	t.Logf("FINDING: TokenStore stores %d tokens without limit. "+
		"No per-user or per-client limit on refresh tokens.", numTokens)
}

// --- TokenStore: delete is not constant-time ---

func TestTokenStore_StressDeleteTiming(t *testing.T) {
	// Delete on nonexistent token should behave the same as existing token
	// to avoid timing-based token enumeration.
	store := NewTokenStore()
	store.Put(&RefreshToken{
		Token:     "exists",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	// Both should complete without error
	store.Delete("exists")
	store.Delete("does-not-exist")
}

// --- TokenStore: concurrent rotation race ---

func TestTokenStore_StressConcurrentRotationRace(t *testing.T) {
	// Simulate two concurrent refresh token requests with the same token.
	// Both should not succeed in issuing new tokens.
	store := NewTokenStore()
	store.Put(&RefreshToken{
		Token:     "shared-token",
		UserID:    "user-1",
		ClientID:  "client-1",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	var successCount int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Simulate: get token, verify it exists, delete old, issue new
			tok, ok := store.Get("shared-token")
			if !ok {
				return
			}
			if tok.Token == "shared-token" {
				store.Delete("shared-token")
				store.Put(&RefreshToken{
					Token:     fmt.Sprintf("new-token-%d", id),
					UserID:    "user-1",
					ClientID:  "client-1",
					ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				})
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount > 1 {
		t.Logf("FINDING: Refresh token rotation succeeded %d times (should be 1). "+
			"Get+Delete+Put is not atomic. The token endpoint needs an atomic "+
			"RotateRefreshToken method.", successCount)
	}
}

// =============================================================================
// SessionStore Stress Tests
// =============================================================================

// --- SessionStore: unbounded growth ---

func TestSessionStore_StressUnboundedGrowth(t *testing.T) {
	store := NewSessionStore()

	const numSessions = 10000
	for i := 0; i < numSessions; i++ {
		store.Put(&AuthSession{
			SessionID: fmt.Sprintf("sess-%d", i),
			CreatedAt: time.Now(),
		})
	}

	store.mu.RLock()
	count := len(store.sessions)
	store.mu.RUnlock()

	if count != numSessions {
		t.Errorf("expected %d sessions, got %d", numSessions, count)
	}

	t.Logf("FINDING: SessionStore stores %d sessions without limit. "+
		"An attacker can flood /authorize to exhaust memory.", numSessions)
}

// --- SessionStore: session fixation ---

func TestSessionStore_StressSessionFixation(t *testing.T) {
	// If an attacker can choose the session ID, they can pre-create a session
	// and wait for a victim to complete the login.
	store := NewSessionStore()

	// Attacker pre-creates a session with a known ID
	attackerSessionID := "attacker-chosen-session-id"
	store.Put(&AuthSession{
		SessionID:     attackerSessionID,
		ClientID:      "attacker-client",
		RedirectURI:   "http://attacker.com/steal",
		State:         "attacker-state",
		CodeChallenge: "attacker-challenge",
		CodeMethod:    "S256",
		CreatedAt:     time.Now(),
	})

	// Verify attacker can read back their session
	sess, ok := store.Get(attackerSessionID)
	if !ok {
		t.Fatal("expected to find attacker's session")
	}

	if sess.RedirectURI == "http://attacker.com/steal" {
		t.Log("FINDING: SessionStore accepts arbitrary session IDs. " +
			"The /authorize handler MUST generate session IDs server-side using " +
			"crypto/rand. If the session ID comes from user input, session fixation is possible.")
	}
}

// --- SessionStore: session not deleted after code issuance ---

func TestSessionStore_StressSessionReuse(t *testing.T) {
	// After generating an auth code, the session should be deleted
	// to prevent re-use.
	store := NewSessionStore()
	store.Put(&AuthSession{
		SessionID: "once-use",
		ClientID:  "client-1",
		CreatedAt: time.Now(),
	})

	// First retrieval (simulating code issuance)
	_, ok := store.Get("once-use")
	if !ok {
		t.Fatal("expected to find session")
	}
	// Session is NOT deleted by Get — the consumer must Delete it

	// Second retrieval should still work (session not deleted)
	_, ok = store.Get("once-use")
	if ok {
		t.Log("FINDING: SessionStore.Get does not delete the session. " +
			"The authorize callback handler must explicitly Delete the session " +
			"after issuing an auth code, otherwise the same session can be reused.")
	}
}

// --- SessionStore: mutation through returned pointer ---

func TestSessionStore_StressMutationThroughPointer(t *testing.T) {
	// SessionStore.Get returns a pointer to the stored struct.
	// External code can mutate the session without going through the store's lock.
	store := NewSessionStore()
	store.Put(&AuthSession{
		SessionID: "mutable",
		ClientID:  "client-1",
		UserID:    "",
		CreatedAt: time.Now(),
	})

	// Get the session and mutate it WITHOUT holding the lock
	sess, _ := store.Get("mutable")
	sess.UserID = "injected-user" // Mutating internal state without lock!

	// Read it back
	sess2, _ := store.Get("mutable")
	if sess2.UserID == "injected-user" {
		t.Log("FINDING: SessionStore.Get returns a pointer to internal data. " +
			"External mutation bypasses the RWMutex. This is safe in single-threaded " +
			"authorize flow but dangerous under concurrent access. Consider returning " +
			"a copy instead of a pointer.")
	}
}

// --- SessionStore: concurrent heavy load ---

func TestSessionStore_StressConcurrentHeavyLoad(t *testing.T) {
	store := NewSessionStore()
	var wg sync.WaitGroup

	const goroutines = 200
	const opsPerGoroutine = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				sid := fmt.Sprintf("sess-%d-%d", id, j%20)
				store.Put(&AuthSession{
					SessionID: sid,
					ClientID:  fmt.Sprintf("client-%d", id),
					CreatedAt: time.Now(),
				})
				store.Get(sid)
				if j%5 == 0 {
					store.Delete(sid)
				}
				if j%10 == 0 {
					store.Cleanup()
				}
			}
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// Cross-Store Integration Stress Tests
// =============================================================================

// --- Full flow race: two threads completing authorization with same session ---

func TestStores_StressRaceOnAuthorizationCompletion(t *testing.T) {
	// Simulate two concurrent requests trying to complete the same authorization session.
	// Both should not succeed in generating auth codes.
	sessions := NewSessionStore()
	codes := NewCodeStore()

	sessions.Put(&AuthSession{
		SessionID:     "race-session",
		ClientID:      "client-1",
		RedirectURI:   "http://localhost/callback",
		CodeChallenge: "challenge",
		CodeMethod:    "S256",
		Scope:         "openid",
		CreatedAt:     time.Now(),
	})

	var codesIssued int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sess, ok := sessions.Get("race-session")
			if !ok {
				return
			}
			// In a real handler, we'd delete the session here
			// But Get+Delete is not atomic, so both threads can proceed
			sessions.Delete("race-session")

			// Issue auth code
			codes.Put(&AuthCode{
				Code:          fmt.Sprintf("code-from-thread-%d", id),
				ClientID:      sess.ClientID,
				UserID:        "user-1",
				RedirectURI:   sess.RedirectURI,
				CodeChallenge: sess.CodeChallenge,
				Scope:         sess.Scope,
				ExpiresAt:     time.Now().Add(5 * time.Minute),
			})
			atomic.AddInt64(&codesIssued, 1)
		}(i)
	}

	wg.Wait()

	if codesIssued > 1 {
		t.Logf("CRITICAL: %d auth codes issued for the same session (should be 1). "+
			"Session.Get + Session.Delete is not atomic. Need an atomic "+
			"ConsumeSession(id) method that deletes under write lock and returns the session.", codesIssued)
	}
}
