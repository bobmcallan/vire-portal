# Summary: Portal Log Ingestion via Vire Server API (fb_8f9d6e0c)

**Status:** completed

## Changes
| File | Change |
|------|--------|
| `internal/client/log_store.go` | NEW — HTTPLogStore implementing ILogStore with buffered HTTP batching (50 entries / 5s flush) |
| `internal/client/log_store_test.go` | NEW — 9 unit tests (size flush, timer flush, close, levels, fields, errors, read methods) |
| `internal/client/log_store_stress_test.go` | NEW — 3 stress tests (concurrent store, entry integrity, high volume) |
| `cmd/vire-portal/main.go` | Wire log store after app init when service key configured; close on shutdown |
| `internal/vire/common/logging.go` | Add AttachLogStore method, WithLogStore support in NewLoggerFromConfig |
| `internal/vire/common/config.go` | Add LogStore field to LoggingConfig struct |
| `go.mod` | Bump arbor v1.4.66 → v1.4.67 (adds WithLogStore API) |

## Tests
- 12 tests added (9 unit + 3 stress), all passing
- `go vet ./...` clean
- Server builds and runs on port 8883

## Architecture
- Architect: approved, no issues
- Pattern: follows VireClient HTTP conventions (10s timeout, X-Vire-Service-ID header)
- Write-only store; read methods return nil (local memory store handles queries)

## Devils-Advocate
- Found race condition in `setFlushInterval` — fixed with mutex protection
- All stress tests pass with `-race` flag

## Notes
- Log store only activates when VIRE_SERVICE_KEY is configured
- Best-effort delivery: network errors logged to stderr, portal continues normally
- Close() flushes synchronously on shutdown to minimize log loss
