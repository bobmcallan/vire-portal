package auth

import (
	"sync"
	"testing"
	"time"
)

func TestSessionStore_PutAndGet(t *testing.T) {
	store := NewSessionStore()
	sess := &AuthSession{
		SessionID:     "sess-1",
		ClientID:      "client-1",
		RedirectURI:   "http://localhost/callback",
		State:         "state-abc",
		CodeChallenge: "challenge123",
		CodeMethod:    "S256",
		Scope:         "openid",
		CreatedAt:     time.Now(),
	}

	store.Put(sess)

	got, ok := store.Get("sess-1")
	if !ok {
		t.Fatal("expected to find session")
	}
	if got.ClientID != "client-1" {
		t.Errorf("expected client-1, got %s", got.ClientID)
	}
	if got.State != "state-abc" {
		t.Errorf("expected state-abc, got %s", got.State)
	}
}

func TestSessionStore_GetNotFound(t *testing.T) {
	store := NewSessionStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent session")
	}
}

func TestSessionStore_ExpiredSession(t *testing.T) {
	store := NewSessionStore()
	store.Put(&AuthSession{
		SessionID: "old",
		CreatedAt: time.Now().Add(-11 * time.Minute), // past TTL
	})

	_, ok := store.Get("old")
	if ok {
		t.Error("expected expired session to not be returned")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore()
	store.Put(&AuthSession{
		SessionID: "sess-1",
		CreatedAt: time.Now(),
	})

	store.Delete("sess-1")

	_, ok := store.Get("sess-1")
	if ok {
		t.Error("expected deleted session to not be found")
	}
}

func TestSessionStore_Cleanup(t *testing.T) {
	store := NewSessionStore()
	store.Put(&AuthSession{
		SessionID: "expired",
		CreatedAt: time.Now().Add(-11 * time.Minute),
	})
	store.Put(&AuthSession{
		SessionID: "valid",
		CreatedAt: time.Now(),
	})

	store.Cleanup()

	_, ok := store.Get("valid")
	if !ok {
		t.Error("expected valid session to survive cleanup")
	}

	store.mu.RLock()
	_, exists := store.sessions["expired"]
	store.mu.RUnlock()
	if exists {
		t.Error("expected expired session to be removed by cleanup")
	}
}

func TestSessionStore_ConcurrentAccess(t *testing.T) {
	store := NewSessionStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sid := "sess-" + string(rune('A'+id%26))
			store.Put(&AuthSession{
				SessionID: sid,
				CreatedAt: time.Now(),
			})
			store.Get(sid)
			if id%3 == 0 {
				store.Delete(sid)
			}
		}(i)
	}

	wg.Wait()
}
