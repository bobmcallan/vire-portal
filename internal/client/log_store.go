package client

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

const (
	logFlushSize     = 50              // flush when buffer reaches this count
	logFlushInterval = 5 * time.Second // flush on this interval
	logMaxBatch      = 500             // server-enforced max per request
)

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

// HTTPLogStore implements writers.ILogStore by buffering log entries and
// POSTing them in batches to vire-server's /api/logs/ingest endpoint.
// Read methods return empty results — this is a write-only remote store.
type HTTPLogStore struct {
	baseURL    string
	serviceID  string
	httpClient *http.Client

	mu     sync.Mutex
	buf    []logIngestEntry
	timer  *time.Timer
	closed bool
}

// NewHTTPLogStore creates a new HTTPLogStore targeting the given vire-server URL.
func NewHTTPLogStore(baseURL, serviceID string) *HTTPLogStore {
	s := &HTTPLogStore{
		baseURL:    baseURL,
		serviceID:  serviceID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		buf:        make([]logIngestEntry, 0, logFlushSize),
	}
	s.timer = time.AfterFunc(logFlushInterval, s.timerFlush)
	return s
}

// Store adds a log entry to the buffer. Flushes if buffer reaches logFlushSize.
func (s *HTTPLogStore) Store(entry models.LogEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("log store closed")
	}
	s.buf = append(s.buf, logIngestEntry{
		Timestamp:     entry.Timestamp,
		Level:         levelString(entry.Level),
		Message:       entry.Message,
		CorrelationID: entry.CorrelationID,
		Prefix:        entry.Prefix,
		Error:         entry.Error,
		Function:      entry.Function,
		Fields:        entry.Fields,
	})
	if len(s.buf) >= logFlushSize {
		s.flushLocked()
	}
	return nil
}

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

func (s *HTTPLogStore) timerFlush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.flushLocked()
	s.timer.Reset(logFlushInterval)
}

// flushLocked drains the buffer and sends asynchronously. Caller must hold mu.
func (s *HTTPLogStore) flushLocked() {
	if len(s.buf) == 0 {
		return
	}
	entries := s.buf
	s.buf = make([]logIngestEntry, 0, logFlushSize)
	go s.send(entries)
}

// send POSTs entries to vire-server in batches of logMaxBatch.
func (s *HTTPLogStore) send(entries []logIngestEntry) {
	for i := 0; i < len(entries); i += logMaxBatch {
		end := i + logMaxBatch
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[i:end]

		payload := logIngestRequest{Source: "portal", Entries: batch}
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			fmt.Fprintf(os.Stderr, "vire-portal: log store encode error: %v\n", err)
			continue
		}

		req, err := http.NewRequest(http.MethodPost, s.baseURL+"/api/logs/ingest", &buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vire-portal: log store request error: %v\n", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Vire-Service-ID", s.serviceID)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vire-portal: log store send error: %v\n", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "vire-portal: log store server returned %d\n", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// Close flushes remaining entries synchronously and stops the timer.
func (s *HTTPLogStore) Close() error {
	s.mu.Lock()
	s.closed = true
	s.timer.Stop()
	remaining := s.buf
	s.buf = nil
	s.mu.Unlock()

	if len(remaining) > 0 {
		s.send(remaining)
	}
	return nil
}

// setFlushInterval overrides the flush interval (for testing).
func (s *HTTPLogStore) setFlushInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timer.Stop()
	s.timer = time.AfterFunc(d, s.timerFlush)
}

// Read-only methods — this is a write-only remote store.

func (s *HTTPLogStore) GetByCorrelation(correlationID string) ([]models.LogEvent, error) {
	return nil, nil
}

func (s *HTTPLogStore) GetByCorrelationWithLevel(correlationID string, minLevel log.Level) ([]models.LogEvent, error) {
	return nil, nil
}

func (s *HTTPLogStore) GetSince(since time.Time) ([]models.LogEvent, error) {
	return nil, nil
}

func (s *HTTPLogStore) GetRecent(limit int) ([]models.LogEvent, error) {
	return nil, nil
}

func (s *HTTPLogStore) GetCorrelationIDs() []string {
	return nil
}
