package cache

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// CachedResponse holds a cached API proxy response.
type CachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// entry wraps a cached response with expiry and insertion order tracking.
type entry struct {
	resp      *CachedResponse
	expiry    time.Time
	insertIdx int64
}

// ResponseCache caches API proxy responses to prevent duplicate round-trips to vire-server.
// Keys are "userID:method:path". Only GET requests should be cached.
// Thread-safe with sync.RWMutex.
type ResponseCache struct {
	mu         sync.RWMutex
	items      map[string]entry
	ttl        time.Duration
	maxEntries int
	nextIdx    int64
}

// New creates a new ResponseCache with the given TTL and max entry count.
func New(ttl time.Duration, maxEntries int) *ResponseCache {
	return &ResponseCache{
		items:      make(map[string]entry),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// MakeKey builds a cache key from userID, HTTP method, and path.
func MakeKey(userID, method, path string) string {
	return userID + ":" + method + ":" + path
}

// Get returns a cached response if found and not expired.
func (c *ResponseCache) Get(key string) (*CachedResponse, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(e.expiry) {
		// Expired: remove lazily
		c.mu.Lock()
		if e2, ok2 := c.items[key]; ok2 && time.Now().After(e2.expiry) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return nil, false
	}

	return e.resp, true
}

// Set stores a response in the cache. Evicts the oldest entry if at capacity.
func (c *ResponseCache) Set(key string, resp *CachedResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e := entry{
		resp:      resp,
		expiry:    time.Now().Add(c.ttl),
		insertIdx: c.nextIdx,
	}
	c.nextIdx++

	// If key already exists, update in place (no capacity change)
	if _, exists := c.items[key]; exists {
		c.items[key] = e
		return
	}

	// Evict oldest if at capacity
	if len(c.items) >= c.maxEntries {
		c.evictOldest()
	}

	c.items[key] = e
}

// InvalidatePrefix removes all entries whose key contains the given prefix path.
func (c *ResponseCache) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.items {
		if strings.Contains(key, prefix) {
			delete(c.items, key)
		}
	}
}

// evictOldest removes the entry with the lowest insertIdx. Must be called with mu held.
func (c *ResponseCache) evictOldest() {
	var oldestKey string
	var oldestIdx int64 = -1

	for key, e := range c.items {
		if oldestIdx == -1 || e.insertIdx < oldestIdx {
			oldestIdx = e.insertIdx
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}
