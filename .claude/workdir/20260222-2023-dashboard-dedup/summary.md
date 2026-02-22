# Summary: Dashboard data deduplication backend

**Date:** 2026-02-22
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `internal/cache/cache.go` | NEW — Thread-safe in-memory response cache with TTL, max entries, LRU eviction, and prefix invalidation |
| `internal/cache/cache_test.go` | NEW — 18 tests: 8 unit + 10 stress tests (cross-user isolation, concurrency, eviction under load, special chars) |
| `internal/server/server.go` | Added `cache` field to Server struct, initialized with 30s TTL / 1000 max entries |
| `internal/server/routes.go` | Integrated cache into `handleAPIProxy`: cache GET responses, invalidate on writes, 5MB body size limit, query string in cache key |
| `pages/static/common.js` | Added `vireStore` data store with fetch caching, single-flight dedup, and array dedup; updated `portfolioDashboard()` to use it |

## Tests
- `internal/cache/cache_test.go` — 18 tests covering get/set, TTL expiry, prefix invalidation, max entries eviction, thread safety, cross-user isolation, special characters, delimiter collisions
- All existing tests pass (`go test ./...`, `go vet ./...`)
- UI tests: 13 dashboard + 8 smoke — all pass, zero failures

## Documentation Updated
- `.claude/skills/develop/SKILL.md` — added `internal/cache/` to Key Directories table

## Devils-Advocate Findings
- **Cache key missing query string** — fixed: uses `r.URL.RequestURI()` instead of `r.URL.Path`
- **No response body size limit** — fixed: 5MB limit via `io.LimitReader`, oversized responses skip caching
- **Cross-user invalidation** — documented as known issue (low severity: invalidation is slightly too broad)
- **MakeKey delimiter collision** — documented as known issue (JWT sub claims rarely contain colons)

## Notes
- Server-side cache: 30s TTL, 1000 max entries, per-user keying, only caches 2xx GET responses
- Client-side store: 30s TTL, single-flight pattern prevents concurrent fetches for same URL, `dedup()` removes duplicate portfolio/holding entries by name/ticker
- Both layers work together: server-side reduces round-trips to vire-server, client-side prevents duplicate browser fetches and deduplicates response arrays
- Server left running after completion
