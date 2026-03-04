package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/phuslu/log"
	"github.com/ternarybob/arbor/models"
)

func makeLogEvent(level log.Level, msg string) models.LogEvent {
	return models.LogEvent{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   msg,
	}
}

// TestHTTPLogStore_StoreAndFlushOnSize stores 50 entries and verifies the server
// receives a single POST with 50 entries, correct source, and service ID header.
func TestHTTPLogStore_StoreAndFlushOnSize(t *testing.T) {
	var (
		mu       sync.Mutex
		received []logIngestRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/logs/ingest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Vire-Service-ID") != "service:test-portal" {
			t.Errorf("expected X-Vire-Service-ID header, got %s", r.Header.Get("X-Vire-Service-ID"))
		}
		var req logIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		mu.Lock()
		received = append(received, req)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":50}`))
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:test-portal")
	defer s.Close()

	for i := 0; i < 50; i++ {
		if err := s.Store(makeLogEvent(log.InfoLevel, "test message")); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// Wait for async send
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 request, got %d", len(received))
	}
	if received[0].Source != "portal" {
		t.Errorf("expected source=portal, got %s", received[0].Source)
	}
	if len(received[0].Entries) != 50 {
		t.Errorf("expected 50 entries, got %d", len(received[0].Entries))
	}
}

// TestHTTPLogStore_FlushOnTimer stores 1 entry and verifies the server receives it
// after the timer fires (100ms flush interval).
func TestHTTPLogStore_FlushOnTimer(t *testing.T) {
	var requestCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		var req logIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if len(req.Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(req.Entries))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":1}`))
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:test-portal")
	s.setFlushInterval(100 * time.Millisecond)
	defer s.Close()

	if err := s.Store(makeLogEvent(log.InfoLevel, "timer test")); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Wait longer than flush interval
	time.Sleep(250 * time.Millisecond)

	if atomic.LoadInt32(&requestCount) < 1 {
		t.Error("expected at least 1 request after timer flush")
	}
}

// TestHTTPLogStore_CloseFlushesRemaining stores 3 entries and verifies Close()
// flushes them synchronously before returning.
func TestHTTPLogStore_CloseFlushesRemaining(t *testing.T) {
	var (
		mu       sync.Mutex
		received []logIngestRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		mu.Lock()
		received = append(received, req)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":3}`))
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:test-portal")

	for i := 0; i < 3; i++ {
		if err := s.Store(makeLogEvent(log.InfoLevel, "close test")); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// Close should flush synchronously
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 request after close, got %d", len(received))
	}
	if len(received[0].Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(received[0].Entries))
	}
}

// TestHTTPLogStore_LevelConversion verifies all log levels are converted to
// the correct server-expected strings.
func TestHTTPLogStore_LevelConversion(t *testing.T) {
	var (
		mu      sync.Mutex
		entries []logIngestEntry
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		mu.Lock()
		entries = append(entries, req.Entries...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":7}`))
	}))
	defer srv.Close()

	levels := []struct {
		level    log.Level
		expected string
	}{
		{log.TraceLevel, "trace"},
		{log.DebugLevel, "debug"},
		{log.InfoLevel, "info"},
		{log.WarnLevel, "warn"},
		{log.ErrorLevel, "error"},
		{log.FatalLevel, "fatal"},
		{log.PanicLevel, "panic"},
	}

	s := NewHTTPLogStore(srv.URL, "service:test-portal")

	for _, tc := range levels {
		if err := s.Store(makeLogEvent(tc.level, tc.expected)); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(entries) != len(levels) {
		t.Fatalf("expected %d entries, got %d", len(levels), len(entries))
	}

	for i, tc := range levels {
		if entries[i].Level != tc.expected {
			t.Errorf("entry %d: expected level %q, got %q", i, tc.expected, entries[i].Level)
		}
	}
}

// TestHTTPLogStore_FieldMapping verifies all fields are correctly mapped to JSON,
// including Function -> "caller" in JSON.
func TestHTTPLogStore_FieldMapping(t *testing.T) {
	var (
		mu      sync.Mutex
		entries []logIngestEntry
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		mu.Lock()
		entries = append(entries, req.Entries...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":1}`))
	}))
	defer srv.Close()

	ts := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	event := models.LogEvent{
		Timestamp:     ts,
		Level:         log.InfoLevel,
		Message:       "field test",
		CorrelationID: "corr-123",
		Prefix:        "prefix-abc",
		Error:         "some error",
		Function:      "main.myFunc",
		Fields:        map[string]interface{}{"key": "value", "count": 42},
	}

	s := NewHTTPLogStore(srv.URL, "service:test-portal")

	if err := s.Store(event); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.CorrelationID != "corr-123" {
		t.Errorf("expected CorrelationID=corr-123, got %s", e.CorrelationID)
	}
	if e.Prefix != "prefix-abc" {
		t.Errorf("expected Prefix=prefix-abc, got %s", e.Prefix)
	}
	if e.Error != "some error" {
		t.Errorf("expected Error=some error, got %s", e.Error)
	}
	if e.Function != "main.myFunc" {
		t.Errorf("expected Function=main.myFunc (caller), got %s", e.Function)
	}
	if e.Message != "field test" {
		t.Errorf("expected Message=field test, got %s", e.Message)
	}
	if e.Fields["key"] != "value" {
		t.Errorf("expected Fields[key]=value, got %v", e.Fields["key"])
	}

	// Verify "caller" JSON tag — Function field should serialize as "caller"
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("failed to marshal entry: %v", err)
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}
	if rawMap["caller"] != "main.myFunc" {
		t.Errorf("expected JSON key 'caller'=main.myFunc, got %v", rawMap["caller"])
	}
	if _, ok := rawMap["function"]; ok {
		t.Error("expected no 'function' JSON key — should be 'caller'")
	}
}

// TestHTTPLogStore_ServerError_NoCrash verifies no panic when the server returns 500.
func TestHTTPLogStore_ServerError_NoCrash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:test-portal")
	s.setFlushInterval(50 * time.Millisecond)

	for i := 0; i < 5; i++ {
		if err := s.Store(makeLogEvent(log.InfoLevel, "error test")); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// Trigger timer flush and wait — should not panic
	time.Sleep(150 * time.Millisecond)

	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestHTTPLogStore_Unreachable_NoCrash verifies no panic when server is unreachable.
func TestHTTPLogStore_Unreachable_NoCrash(t *testing.T) {
	s := NewHTTPLogStore("http://127.0.0.1:1", "service:test-portal")

	for i := 0; i < 3; i++ {
		if err := s.Store(makeLogEvent(log.InfoLevel, "unreachable test")); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// Close flushes synchronously — should not panic even if server unreachable
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestHTTPLogStore_ReadMethods_ReturnEmpty verifies all read methods return nil/empty.
func TestHTTPLogStore_ReadMethods_ReturnEmpty(t *testing.T) {
	s := NewHTTPLogStore("http://127.0.0.1:1", "service:test-portal")
	defer s.Close()

	events, err := s.GetByCorrelation("corr-123")
	if err != nil || events != nil {
		t.Errorf("GetByCorrelation: expected nil, nil; got %v, %v", events, err)
	}

	events, err = s.GetByCorrelationWithLevel("corr-123", log.InfoLevel)
	if err != nil || events != nil {
		t.Errorf("GetByCorrelationWithLevel: expected nil, nil; got %v, %v", events, err)
	}

	events, err = s.GetSince(time.Now())
	if err != nil || events != nil {
		t.Errorf("GetSince: expected nil, nil; got %v, %v", events, err)
	}

	events, err = s.GetRecent(10)
	if err != nil || events != nil {
		t.Errorf("GetRecent: expected nil, nil; got %v, %v", events, err)
	}

	ids := s.GetCorrelationIDs()
	if ids != nil {
		t.Errorf("GetCorrelationIDs: expected nil, got %v", ids)
	}
}

// TestHTTPLogStore_StoreAfterClose_ReturnsError verifies Store returns an error
// after Close has been called.
func TestHTTPLogStore_StoreAfterClose_ReturnsError(t *testing.T) {
	s := NewHTTPLogStore("http://127.0.0.1:1", "service:test-portal")

	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	err := s.Store(makeLogEvent(log.InfoLevel, "after close"))
	if err == nil {
		t.Error("expected error when storing after close, got nil")
	}
}
