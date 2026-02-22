# Requirements: Dashboard data deduplication backend

**Date:** 2026-02-22
**Requested:** The dashboard page is loading items twice (e.g., Portfolios, SMSF). Implement a data backend to prevent duplicate loading of the same content.

## Scope
- In scope: Server-side API response cache, client-side data store with dedup, integration into existing API proxy
- Out of scope: Changes to vire-server, changes to the dashboard HTML template structure

## Root Cause Analysis

The portal proxies all `/api/*` requests to vire-server with zero caching (`internal/server/routes.go:100-146`). The dashboard Alpine.js component (`pages/static/common.js:122-271`) calls:
- `GET /api/portfolios` on init (line 158)
- `GET /api/portfolios/{name}`, `GET /api/portfolios/{name}/strategy`, `GET /api/portfolios/{name}/plan` on portfolio selection (lines 184-188)

Without any caching or deduplication layer, every page load or navigation triggers fresh API calls. If the vire-server returns duplicate portfolio entries, or if `init()` triggers multiple times (e.g., browser back/forward, Alpine re-initialization), duplicate items appear in the UI.

## Approach

### 1. Server-side API response cache (`internal/cache/cache.go`)

A simple thread-safe in-memory cache:
- **Key**: `userID:GET:path` (only cache GET requests)
- **Value**: `{statusCode int, headers http.Header, body []byte, expiry time.Time}`
- **TTL**: 30 seconds (configurable)
- **Max entries**: 1000 (prevent unbounded growth)
- **Eviction**: expired entries cleaned up on access + periodic sweep
- **Write-through invalidation**: PUT/POST/DELETE requests invalidate entries with matching path prefix

### 2. Integrate cache into API proxy (`internal/server/routes.go`)

Update `handleAPIProxy`:
- Before proxying a GET request, check cache for a hit
- On cache hit, return cached response (skip vire-server round-trip)
- On cache miss, proxy to vire-server and store the response in cache
- On non-GET (PUT/POST/DELETE), invalidate related cache entries after proxying

### 3. Client-side data store (`pages/static/common.js`)

Add a `vireStore` singleton:
- Caches fetch results by URL with configurable TTL (30s default)
- Single-flight pattern: prevents concurrent fetches for the same URL
- `dedup(array, key)` utility: deduplicates arrays by a key field
- Update `portfolioDashboard().init()` to use `vireStore.fetch()` and deduplicate portfolios by name
- Update `portfolioDashboard().loadPortfolio()` to use `vireStore.fetch()`

### 4. Tests

- `internal/cache/cache_test.go` — unit tests for cache operations (get/set, TTL expiry, invalidation, max entries, thread safety)
- Update `internal/server/proxy_stress_test.go` if proxy behavior changes

## Files Expected to Change
- `internal/cache/cache.go` (NEW) — response cache implementation
- `internal/cache/cache_test.go` (NEW) — cache unit tests
- `internal/server/routes.go` — integrate cache into `handleAPIProxy`
- `internal/server/server.go` — initialize cache and pass to routes
- `pages/static/common.js` — add `vireStore`, update `portfolioDashboard()`
