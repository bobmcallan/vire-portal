package auth

import (
	"sync"
	"testing"
	"time"
)

// --- ClientStore Tests ---

func TestClientStore_PutAndGet(t *testing.T) {
	store := NewClientStore()
	client := &OAuthClient{
		ClientID:     "client-1",
		ClientSecret: "secret-1",
		ClientName:   "Test App",
		RedirectURIs: []string{"http://localhost/callback"},
	}

	store.Put(client)

	got, ok := store.Get("client-1")
	if !ok {
		t.Fatal("expected to find client")
	}
	if got.ClientID != "client-1" {
		t.Errorf("expected client-1, got %s", got.ClientID)
	}
	if got.ClientName != "Test App" {
		t.Errorf("expected Test App, got %s", got.ClientName)
	}
}

func TestClientStore_GetNotFound(t *testing.T) {
	store := NewClientStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent client")
	}
}

func TestClientStore_Overwrite(t *testing.T) {
	store := NewClientStore()
	store.Put(&OAuthClient{ClientID: "c1", ClientName: "Original"})
	store.Put(&OAuthClient{ClientID: "c1", ClientName: "Updated"})

	got, ok := store.Get("c1")
	if !ok {
		t.Fatal("expected to find client")
	}
	if got.ClientName != "Updated" {
		t.Errorf("expected Updated, got %s", got.ClientName)
	}
}

func TestClientStore_ConcurrentAccess(t *testing.T) {
	store := NewClientStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cid := "client-" + string(rune('A'+id%26))
			store.Put(&OAuthClient{ClientID: cid, ClientName: "Test"})
			store.Get(cid)
		}(i)
	}

	wg.Wait()
}

// --- CodeStore Tests ---

func TestCodeStore_PutAndGet(t *testing.T) {
	store := NewCodeStore()
	code := &AuthCode{
		Code:      "code-123",
		ClientID:  "client-1",
		UserID:    "user-1",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	store.Put(code)

	got, ok := store.Get("code-123")
	if !ok {
		t.Fatal("expected to find code")
	}
	if got.ClientID != "client-1" {
		t.Errorf("expected client-1, got %s", got.ClientID)
	}
}

func TestCodeStore_GetNotFound(t *testing.T) {
	store := NewCodeStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent code")
	}
}

func TestCodeStore_ExpiredCode(t *testing.T) {
	store := NewCodeStore()
	store.Put(&AuthCode{
		Code:      "expired",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	})

	_, ok := store.Get("expired")
	if ok {
		t.Error("expected expired code to not be returned")
	}
}

func TestCodeStore_MarkUsed(t *testing.T) {
	store := NewCodeStore()
	store.Put(&AuthCode{
		Code:      "code-1",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})

	store.MarkUsed("code-1")

	got, ok := store.Get("code-1")
	if !ok {
		t.Fatal("expected to find code")
	}
	if !got.Used {
		t.Error("expected code to be marked as used")
	}
}

func TestCodeStore_Cleanup(t *testing.T) {
	store := NewCodeStore()
	store.Put(&AuthCode{
		Code:      "expired",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	})
	store.Put(&AuthCode{
		Code:      "valid",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})

	store.Cleanup()

	_, ok := store.Get("valid")
	if !ok {
		t.Error("expected valid code to survive cleanup")
	}

	// Expired code should already not be returned by Get, but verify it's cleaned up
	store.mu.RLock()
	_, exists := store.codes["expired"]
	store.mu.RUnlock()
	if exists {
		t.Error("expected expired code to be removed by cleanup")
	}
}

func TestCodeStore_ConcurrentAccess(t *testing.T) {
	store := NewCodeStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			code := "code-" + string(rune('A'+id%26))
			store.Put(&AuthCode{
				Code:      code,
				ExpiresAt: time.Now().Add(5 * time.Minute),
			})
			store.Get(code)
			store.MarkUsed(code)
		}(i)
	}

	wg.Wait()
}

// --- TokenStore Tests ---

func TestTokenStore_PutAndGet(t *testing.T) {
	store := NewTokenStore()
	token := &RefreshToken{
		Token:     "refresh-123",
		UserID:    "user-1",
		ClientID:  "client-1",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	store.Put(token)

	got, ok := store.Get("refresh-123")
	if !ok {
		t.Fatal("expected to find token")
	}
	if got.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", got.UserID)
	}
}

func TestTokenStore_GetNotFound(t *testing.T) {
	store := NewTokenStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent token")
	}
}

func TestTokenStore_ExpiredToken(t *testing.T) {
	store := NewTokenStore()
	store.Put(&RefreshToken{
		Token:     "expired",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	_, ok := store.Get("expired")
	if ok {
		t.Error("expected expired token to not be returned")
	}
}

func TestTokenStore_Delete(t *testing.T) {
	store := NewTokenStore()
	store.Put(&RefreshToken{
		Token:     "to-delete",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	store.Delete("to-delete")

	_, ok := store.Get("to-delete")
	if ok {
		t.Error("expected deleted token to not be found")
	}
}

func TestTokenStore_Cleanup(t *testing.T) {
	store := NewTokenStore()
	store.Put(&RefreshToken{
		Token:     "expired",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	store.Put(&RefreshToken{
		Token:     "valid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	store.Cleanup()

	_, ok := store.Get("valid")
	if !ok {
		t.Error("expected valid token to survive cleanup")
	}

	store.mu.RLock()
	_, exists := store.tokens["expired"]
	store.mu.RUnlock()
	if exists {
		t.Error("expected expired token to be removed by cleanup")
	}
}

func TestTokenStore_ConcurrentAccess(t *testing.T) {
	store := NewTokenStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tok := "token-" + string(rune('A'+id%26))
			store.Put(&RefreshToken{
				Token:     tok,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			})
			store.Get(tok)
			if id%3 == 0 {
				store.Delete(tok)
			}
		}(i)
	}

	wg.Wait()
}
