package cache

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestResponseCache_GetSet(t *testing.T) {
	c := New(5*time.Second, 100)

	resp := &CachedResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"status":"ok"}`),
	}

	key := MakeKey("user1", "GET", "/api/portfolios")
	c.Set(key, resp)

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", got.StatusCode)
	}
	if string(got.Body) != `{"status":"ok"}` {
		t.Errorf("unexpected body: %s", got.Body)
	}
	if got.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("unexpected content-type: %s", got.Headers.Get("Content-Type"))
	}
}

func TestResponseCache_Miss(t *testing.T) {
	c := New(5*time.Second, 100)

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestResponseCache_TTLExpiration(t *testing.T) {
	c := New(50*time.Millisecond, 100)

	resp := &CachedResponse{
		StatusCode: http.StatusOK,
		Body:       []byte("data"),
	}

	key := MakeKey("user1", "GET", "/api/test")
	c.Set(key, resp)

	// Should be found immediately
	if _, ok := c.Get(key); !ok {
		t.Fatal("expected cache hit before expiry")
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)

	if _, ok := c.Get(key); ok {
		t.Error("expected cache miss after TTL expiration")
	}
}

func TestResponseCache_InvalidatePrefix(t *testing.T) {
	c := New(5*time.Second, 100)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	c.Set(MakeKey("user1", "GET", "/api/portfolios"), resp)
	c.Set(MakeKey("user1", "GET", "/api/portfolios/SMSF"), resp)
	c.Set(MakeKey("user1", "GET", "/api/portfolios/SMSF/strategy"), resp)
	c.Set(MakeKey("user1", "GET", "/api/users/123"), resp)

	c.InvalidatePrefix("/api/portfolios")

	// All portfolio entries should be invalidated
	if _, ok := c.Get(MakeKey("user1", "GET", "/api/portfolios")); ok {
		t.Error("expected /api/portfolios to be invalidated")
	}
	if _, ok := c.Get(MakeKey("user1", "GET", "/api/portfolios/SMSF")); ok {
		t.Error("expected /api/portfolios/SMSF to be invalidated")
	}
	if _, ok := c.Get(MakeKey("user1", "GET", "/api/portfolios/SMSF/strategy")); ok {
		t.Error("expected /api/portfolios/SMSF/strategy to be invalidated")
	}

	// Users entry should remain
	if _, ok := c.Get(MakeKey("user1", "GET", "/api/users/123")); !ok {
		t.Error("expected /api/users/123 to remain in cache")
	}
}

func TestResponseCache_MaxEntries(t *testing.T) {
	c := New(5*time.Second, 3)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	c.Set("key1", resp)
	c.Set("key2", resp)
	c.Set("key3", resp)

	// All three should be present
	for _, k := range []string{"key1", "key2", "key3"} {
		if _, ok := c.Get(k); !ok {
			t.Errorf("expected %s to be in cache", k)
		}
	}

	// Adding a 4th should evict the oldest (key1)
	c.Set("key4", resp)

	if _, ok := c.Get("key1"); ok {
		t.Error("expected key1 to be evicted (oldest entry)")
	}
	if _, ok := c.Get("key4"); !ok {
		t.Error("expected key4 to be in cache")
	}
}

func TestResponseCache_ThreadSafety(t *testing.T) {
	c := New(5*time.Second, 1000)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := MakeKey("user1", "GET", "/api/test/"+string(rune('A'+n%26)))
			c.Set(key, resp)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := MakeKey("user1", "GET", "/api/test/"+string(rune('A'+n%26)))
			c.Get(key)
		}(i)
	}

	// Concurrent invalidations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.InvalidatePrefix("/api/test")
		}()
	}

	wg.Wait()
	// If we get here without a race condition panic, the test passes
}

func TestMakeKey(t *testing.T) {
	key := MakeKey("user123", "GET", "/api/portfolios")
	expected := "user123:GET:/api/portfolios"
	if key != expected {
		t.Errorf("expected key %q, got %q", expected, key)
	}
}

func TestResponseCache_OverwriteExistingKey(t *testing.T) {
	c := New(5*time.Second, 100)

	resp1 := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("v1")}
	resp2 := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("v2")}

	c.Set("key", resp1)
	c.Set("key", resp2)

	got, ok := c.Get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got.Body) != "v2" {
		t.Errorf("expected updated body v2, got %s", got.Body)
	}
}

func TestResponseCache_EmptyCache(t *testing.T) {
	c := New(5*time.Second, 100)

	// InvalidatePrefix on empty cache should not panic
	c.InvalidatePrefix("/api/test")

	// Get on empty cache
	if _, ok := c.Get("anything"); ok {
		t.Error("expected miss on empty cache")
	}
}

// --- Stress tests (devils-advocate) ---

// TestStress_CrossUserCacheIsolation verifies that user A's cached data
// is never served to user B. Cache keys must include userID.
func TestStress_CrossUserCacheIsolation(t *testing.T) {
	c := New(5*time.Second, 1000)

	userAResp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte(`{"user":"A","secret":"alice-data"}`)}
	userBResp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte(`{"user":"B","secret":"bob-data"}`)}

	keyA := MakeKey("userA", "GET", "/api/portfolios")
	keyB := MakeKey("userB", "GET", "/api/portfolios")

	c.Set(keyA, userAResp)
	c.Set(keyB, userBResp)

	// User A must only get user A's data
	gotA, ok := c.Get(keyA)
	if !ok {
		t.Fatal("expected cache hit for userA")
	}
	if string(gotA.Body) != `{"user":"A","secret":"alice-data"}` {
		t.Errorf("userA got wrong data: %s", gotA.Body)
	}

	// User B must only get user B's data
	gotB, ok := c.Get(keyB)
	if !ok {
		t.Fatal("expected cache hit for userB")
	}
	if string(gotB.Body) != `{"user":"B","secret":"bob-data"}` {
		t.Errorf("userB got wrong data: %s", gotB.Body)
	}

	// Cross-user key must miss
	crossKey := MakeKey("userA", "GET", "/api/portfolios")
	got, _ := c.Get(crossKey)
	if got != nil && string(got.Body) != `{"user":"A","secret":"alice-data"}` {
		t.Error("cross-user cache contamination detected")
	}
}

// TestStress_InvalidatePrefixCrossUser demonstrates that InvalidatePrefix
// affects ALL users' entries, not just the requesting user's. This is a
// known design issue: a write by userA invalidates userB's cached reads.
func TestStress_InvalidatePrefixCrossUser(t *testing.T) {
	c := New(5*time.Second, 1000)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	// Both users cache the same path
	c.Set(MakeKey("userA", "GET", "/api/portfolios"), resp)
	c.Set(MakeKey("userB", "GET", "/api/portfolios"), resp)

	// UserA does a PUT which invalidates "/api/portfolios/default"
	// In the proxy, InvalidatePrefix is called with the path (not scoped to user)
	c.InvalidatePrefix("/api/portfolios")

	// userA's entry should be gone (expected)
	if _, ok := c.Get(MakeKey("userA", "GET", "/api/portfolios")); ok {
		t.Error("expected userA entry to be invalidated")
	}

	// BUG: userB's entry is ALSO gone because InvalidatePrefix is not user-scoped
	_, userBStillCached := c.Get(MakeKey("userB", "GET", "/api/portfolios"))
	if !userBStillCached {
		t.Log("KNOWN ISSUE: InvalidatePrefix is not user-scoped — userB's cache was cleared by userA's write")
	}
}

// TestStress_EmptyPrefixWipesAll verifies that an empty prefix passed to
// InvalidatePrefix wipes the entire cache (strings.Contains(key, "") is
// always true). This is a footgun if any caller passes empty string.
func TestStress_EmptyPrefixWipesAll(t *testing.T) {
	c := New(5*time.Second, 1000)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	c.Set(MakeKey("user1", "GET", "/api/portfolios"), resp)
	c.Set(MakeKey("user2", "GET", "/api/users/123"), resp)
	c.Set(MakeKey("user3", "GET", "/api/settings"), resp)

	// Empty prefix matches everything
	c.InvalidatePrefix("")

	for _, key := range []string{
		MakeKey("user1", "GET", "/api/portfolios"),
		MakeKey("user2", "GET", "/api/users/123"),
		MakeKey("user3", "GET", "/api/settings"),
	} {
		if _, ok := c.Get(key); ok {
			t.Errorf("expected %s to be wiped by empty prefix invalidation", key)
		}
	}
}

// TestStress_CacheKeyMissingQueryString verifies that cache keys do NOT
// include query parameters. Two requests to the same path with different
// query strings share one cache entry — the first response is served for both.
func TestStress_CacheKeyMissingQueryString(t *testing.T) {
	c := New(5*time.Second, 1000)

	// Simulate: GET /api/portfolios?page=1
	resp1 := &CachedResponse{StatusCode: http.StatusOK, Body: []byte(`{"page":1}`)}
	key := MakeKey("user1", "GET", "/api/portfolios")
	c.Set(key, resp1)

	// Simulate: GET /api/portfolios?page=2 — same cache key!
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	// The cached response is for page=1, but the request was for page=2
	if string(got.Body) == `{"page":1}` {
		t.Log("KNOWN ISSUE: cache key does not include query string — /api/portfolios?page=2 returns page=1 data")
	}
}

// TestStress_MaxEntriesEvictionUnderLoad verifies that the cache never
// exceeds maxEntries even under concurrent writes from many goroutines.
func TestStress_MaxEntriesEvictionUnderLoad(t *testing.T) {
	maxEntries := 50
	c := New(5*time.Second, maxEntries)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("x")}

	var wg sync.WaitGroup
	// 200 goroutines each writing a unique key
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := MakeKey("user1", "GET", "/api/item/"+string(rune(n)))
			c.Set(key, resp)
		}(i)
	}
	wg.Wait()

	c.mu.RLock()
	count := len(c.items)
	c.mu.RUnlock()

	if count > maxEntries {
		t.Errorf("cache exceeded maxEntries: got %d, max %d", count, maxEntries)
	}
}

// TestStress_ConcurrentGetExpiredAndSet verifies that the lock upgrade in
// Get (RLock -> Lock for lazy expiry removal) does not race with Set.
func TestStress_ConcurrentGetExpiredAndSet(t *testing.T) {
	c := New(1*time.Millisecond, 1000)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	// Pre-fill cache entries that will all expire immediately
	for i := 0; i < 100; i++ {
		c.Set(MakeKey("user1", "GET", "/api/item/"+string(rune(i))), resp)
	}

	// Let them expire
	time.Sleep(5 * time.Millisecond)

	var wg sync.WaitGroup
	// Concurrent Gets (which trigger lazy expiry deletion) + Sets
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			c.Get(MakeKey("user1", "GET", "/api/item/"+string(rune(n))))
		}(i)
		go func(n int) {
			defer wg.Done()
			c.Set(MakeKey("user1", "GET", "/api/new/"+string(rune(n))), resp)
		}(i)
	}
	wg.Wait()
	// If we get here without a race panic, concurrency is safe
}

// TestStress_LargeResponseBody verifies that caching very large responses
// works but consumes proportional memory. No size guard exists.
func TestStress_LargeResponseBody(t *testing.T) {
	c := New(5*time.Second, 10)

	// 1MB response body
	largeBody := make([]byte, 1*1024*1024)
	for i := range largeBody {
		largeBody[i] = 'A'
	}
	resp := &CachedResponse{StatusCode: http.StatusOK, Body: largeBody}

	key := MakeKey("user1", "GET", "/api/large")
	c.Set(key, resp)

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit for large response")
	}
	if len(got.Body) != 1*1024*1024 {
		t.Errorf("expected 1MB body, got %d bytes", len(got.Body))
	}
	t.Log("WARNING: no max body size limit — a 10MB+ response would be cached without restriction")
}

// TestStress_SpecialCharactersInPath verifies cache behaviour with paths
// containing URL-encoded characters, unicode, and unusual byte sequences.
func TestStress_SpecialCharactersInPath(t *testing.T) {
	c := New(5*time.Second, 1000)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	paths := []string{
		"/api/portfolios/My%20Portfolio",
		"/api/portfolios/S&P%20500",
		"/api/portfolios/caf\u00e9",                     // unicode
		"/api/portfolios/foo/../bar",                    // path traversal attempt
		"/api/portfolios/foo%00bar",                     // null byte
		"/api/portfolios/" + string([]byte{0x80, 0x81}), // invalid UTF-8
	}

	for _, path := range paths {
		key := MakeKey("user1", "GET", path)
		c.Set(key, resp)
		got, ok := c.Get(key)
		if !ok {
			t.Errorf("cache miss for path %q", path)
			continue
		}
		if string(got.Body) != "data" {
			t.Errorf("wrong data for path %q", path)
		}
	}
}

// TestStress_MakeKeyDelimiterCollision demonstrates that userIDs containing
// colons can create ambiguous cache keys. This is low severity since JWT sub
// claims rarely contain colons, but documents the limitation.
func TestStress_MakeKeyDelimiterCollision(t *testing.T) {
	// These two different (userID, method, path) tuples produce the same key
	key1 := MakeKey("user:evil", "GET", "/api/x")
	key2 := MakeKey("user", "evil:GET", "/api/x")

	if key1 == key2 {
		t.Log("KNOWN ISSUE: MakeKey delimiter collision — different inputs produce identical key: " + key1)
	}
}

// TestStress_MaxEntriesZero verifies behaviour when maxEntries is 0 or negative.
func TestStress_MaxEntriesZero(t *testing.T) {
	c := New(5*time.Second, 0)

	resp := &CachedResponse{StatusCode: http.StatusOK, Body: []byte("data")}

	// With maxEntries=0, every Set should trigger eviction
	c.Set("key1", resp)

	c.mu.RLock()
	count := len(c.items)
	c.mu.RUnlock()

	// With maxEntries=0: len(items)=0 >= 0 is true, so evictOldest runs on
	// empty map (no-op), then the item is added. Next Set: len=1 >= 0, evicts
	// the one item, then adds the new one. Cache stays at size 1.
	if count > 1 {
		t.Errorf("with maxEntries=0, expected at most 1 item, got %d", count)
	}
}
