# Requirements: Portal Log Ingestion via Vire Server API (fb_8f9d6e0c)

## Overview

Create an HTTP-based `ILogStore` implementation that buffers portal log entries and POSTs them in batches to vire-server's `POST /api/logs/ingest` endpoint. Wire it into the portal's logger after app initialization. This is a write-only remote store; read methods return empty results.

No UI changes — purely backend. No UI tests needed.

---

## Server-Side Endpoint (ALREADY IMPLEMENTED — DO NOT MODIFY)

```
POST /api/logs/ingest
Auth: X-Vire-Service-ID header (requires admin or portal role)
Max: 500 entries per request
```

Request payload:
```json
{
  "source": "portal",
  "entries": [
    {
      "timestamp": "2026-03-05T12:00:00Z",
      "level": "info",
      "message": "...",
      "correlation_id": "optional",
      "prefix": "optional",
      "error": "optional",
      "caller": "optional",
      "fields": {"key": "value"}
    }
  ]
}
```

Response: `{"accepted": N}`

---

## File 1: `internal/client/log_store.go` (CREATE)

**Package:** `client`

### Struct

```go
// HTTPLogStore implements writers.ILogStore by buffering log entries and
// POSTing them in batches to vire-server's /api/logs/ingest endpoint.
// Read methods return empty results — this is a write-only remote store.
type HTTPLogStore struct {
    baseURL    string
    serviceID  string
    httpClient *http.Client

    mu      sync.Mutex
    buf     []logIngestEntry
    timer   *time.Timer
    closed  bool
}
```

### Constants

```go
const (
    logFlushSize     = 50               // flush when buffer reaches this count
    logFlushInterval = 5 * time.Second  // flush on this interval
    logMaxBatch      = 500              // server-enforced max per request
)
```

### Internal types (unexported, matching server JSON contract)

```go
type logIngestEntry struct {
    Timestamp     time.Time              `json:"timestamp"`
    Level         string                 `json:"level"`
    Message       string                 `json:"message"`
    CorrelationID string                 `json:"correlation_id,omitempty"`
    Prefix        string                 `json:"prefix,omitempty"`
    Error         string                 `json:"error,omitempty"`
    Function      string                 `json:"caller,omitempty"`
    Fields        map[string]interface{} `json:"fields,omitempty"`
}

type logIngestRequest struct {
    Source  string           `json:"source"`
    Entries []logIngestEntry `json:"entries"`
}
```

### Constructor

```go
func NewHTTPLogStore(baseURL, serviceID string) *HTTPLogStore
```

- Creates `http.Client` with 10s timeout (matching VireClient pattern)
- Initializes empty `buf` slice with capacity `logFlushSize`
- Starts flush timer: `s.timer = time.AfterFunc(logFlushInterval, s.timerFlush)`

### Store(entry models.LogEvent) error

- Lock mu
- If closed, return `fmt.Errorf("log store closed")`
- Convert LogEvent to logIngestEntry:
  - Timestamp = entry.Timestamp
  - Level = levelString(entry.Level) — helper that maps phuslu/log Level int to string
  - Message = entry.Message
  - CorrelationID = entry.CorrelationID
  - Prefix = entry.Prefix
  - Error = entry.Error
  - Function = entry.Function
  - Fields = entry.Fields
- Append to buf
- If len(buf) >= logFlushSize, call flushLocked()
- Unlock, return nil

### levelString(level log.Level) string (unexported helper)

Map phuslu/log level constants to server-expected strings:
```go
func levelString(l log.Level) string {
    switch l {
    case log.TraceLevel:
        return "trace"
    case log.DebugLevel:
        return "debug"
    case log.InfoLevel:
        return "info"
    case log.WarnLevel:
        return "warn"
    case log.ErrorLevel:
        return "error"
    case log.FatalLevel:
        return "fatal"
    case log.PanicLevel:
        return "panic"
    default:
        return "info"
    }
}
```

### timerFlush() (unexported)

```go
func (s *HTTPLogStore) timerFlush() {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.closed {
        return
    }
    s.flushLocked()
    s.timer.Reset(logFlushInterval)
}
```

### flushLocked() (unexported, caller holds mu)

```go
func (s *HTTPLogStore) flushLocked() {
    if len(s.buf) == 0 {
        return
    }
    entries := s.buf
    s.buf = make([]logIngestEntry, 0, logFlushSize)
    go s.send(entries)
}
```

### send(entries []logIngestEntry) (unexported)

- Chunk entries into batches of logMaxBatch (500) if needed
- For each batch:
  - Create logIngestRequest{Source: "portal", Entries: batch}
  - JSON encode to bytes.Buffer
  - Create http.NewRequest POST to baseURL+"/api/logs/ingest"
  - Set headers: Content-Type: application/json, X-Vire-Service-ID: serviceID
  - Execute with httpClient.Do(req)
  - On error: fmt.Fprintf(os.Stderr, ...) — do NOT panic
  - On non-200: fmt.Fprintf(os.Stderr, ...) — do NOT panic
  - Always close resp.Body

### Close() error

- Lock mu
- Set closed = true
- Stop timer
- If len(buf) > 0, call send(buf) SYNCHRONOUSLY (not in goroutine)
- Clear buf
- Unlock, return nil

### Read-only methods (all return nil/empty)

```go
func (s *HTTPLogStore) GetByCorrelation(correlationID string) ([]models.LogEvent, error)         { return nil, nil }
func (s *HTTPLogStore) GetByCorrelationWithLevel(correlationID string, minLevel log.Level) ([]models.LogEvent, error) { return nil, nil }
func (s *HTTPLogStore) GetSince(since time.Time) ([]models.LogEvent, error)                     { return nil, nil }
func (s *HTTPLogStore) GetRecent(limit int) ([]models.LogEvent, error)                          { return nil, nil }
func (s *HTTPLogStore) GetCorrelationIDs() []string                                             { return nil }
```

### Imports

```go
import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "sync"
    "time"

    "github.com/phuslu/log"
    "github.com/ternarybob/arbor/models"
)
```

### Testability helper

```go
// setFlushInterval overrides the flush interval (for testing).
func (s *HTTPLogStore) setFlushInterval(d time.Duration) {
    s.timer.Stop()
    s.timer = time.AfterFunc(d, s.timerFlush)
}
```

---

## File 2: `internal/client/log_store_test.go` (CREATE)

**Package:** `client`

Use `httptest.NewServer` to mock the vire-server endpoint.

### Test Cases

1. **TestHTTPLogStore_StoreAndFlushOnSize** — Store 50 entries, verify server receives a single POST with 50 entries, source="portal", X-Vire-Service-ID header present.

2. **TestHTTPLogStore_FlushOnTimer** — Use setFlushInterval(100ms). Store 1 entry, wait 200ms, verify server received a POST with 1 entry.

3. **TestHTTPLogStore_CloseFlushesRemaining** — Store 3 entries, call Close(), verify server received them synchronously.

4. **TestHTTPLogStore_LevelConversion** — Store entries at each log level (trace through panic), verify JSON payload contains correct level strings.

5. **TestHTTPLogStore_FieldMapping** — Store an entry with all fields populated (CorrelationID, Prefix, Error, Function, Fields map), verify JSON maps correctly (Function → "caller" in JSON).

6. **TestHTTPLogStore_ServerError_NoCrash** — Server returns 500. Store entries, trigger flush. Verify no panic.

7. **TestHTTPLogStore_Unreachable_NoCrash** — Use http://127.0.0.1:1 as baseURL. Store entries, Close(). Verify no panic.

8. **TestHTTPLogStore_ReadMethods_ReturnEmpty** — Verify all read methods return nil/empty.

9. **TestHTTPLogStore_StoreAfterClose_ReturnsError** — Close the store, then Store(). Verify error returned.

---

## File 3: `cmd/vire-portal/main.go` (MODIFY)

### Add import

```go
"github.com/bobmcallan/vire-portal/internal/client"
```

### Add log store wiring after app init (after line 112, before shutdownChan)

```go
// Wire remote log store if service key is configured
var logStore *client.HTTPLogStore
if cfg.Service.Key != "" {
    portalID := cfg.Service.PortalID
    if portalID == "" {
        portalID, _ = os.Hostname()
    }
    serviceID := "service:" + portalID
    logStore = client.NewHTTPLogStore(cfg.API.URL, serviceID)
    logger.AttachLogStore(logStore)
    logger.Info().Msg("remote log store attached")
}
```

### Add log store close on shutdown (after application.Close() at line 153)

```go
if logStore != nil {
    logStore.Close()
}
```

---

## Summary of Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/client/log_store.go` | CREATE | HTTPLogStore: ILogStore with buffered HTTP batching |
| `internal/client/log_store_test.go` | CREATE | 9 unit tests covering flush, close, errors, field mapping |
| `cmd/vire-portal/main.go` | MODIFY | Wire log store after app init, close on shutdown |

## Dependencies

No new Go module dependencies. Uses existing:
- `github.com/phuslu/log` (already in go.mod)
- `github.com/ternarybob/arbor/models` (already in go.mod)
- `github.com/ternarybob/arbor/writers` (already in go.mod, needed for ILogStore interface compliance)

## Edge Cases

- **Service key not configured:** Log store is not created. Portal operates as before with only local logging.
- **Server unreachable:** Errors logged to stderr. Portal continues normally. Logs may be lost — acceptable for best-effort remote persistence.
- **Server returns 500:** Same as unreachable — log to stderr, don't crash.
- **Buffer overflow:** logFlushSize (50) triggers flush. logMaxBatch (500) is the server cap — will never be reached with 50-entry flushes.
- **Graceful shutdown:** Close() flushes synchronously before process exits.
- **Store after Close:** Returns error, entry is dropped.
