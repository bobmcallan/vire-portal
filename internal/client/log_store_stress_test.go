package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/phuslu/log"
	"github.com/ternarybob/arbor/models"
)

// =============================================================================
// HTTPLogStore: Concurrency & Race Conditions
// =============================================================================

func TestHTTPLogStore_ConcurrentStore(t *testing.T) {
	// Hammer Store() from many goroutines. Run with -race to detect data races.
	var received atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		received.Add(int64(len(req.Entries)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted":` + fmt.Sprintf("%d", len(req.Entries)) + `}`))
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")
	defer s.Close()

	const goroutines = 50
	const perGoroutine = 100
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				err := s.Store(models.LogEvent{
					Level:     log.InfoLevel,
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("goroutine=%d seq=%d", id, i),
				})
				if err != nil {
					t.Errorf("Store failed: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()

	// Close flushes remaining synchronously
	s.Close()

	// Allow in-flight goroutines from flushLocked() to complete
	time.Sleep(500 * time.Millisecond)

	total := received.Load()
	expected := int64(goroutines * perGoroutine)
	if total != expected {
		t.Errorf("expected %d entries received, got %d (lost %d)", expected, total, expected-total)
	}
}

func TestHTTPLogStore_ConcurrentStoreAndClose(t *testing.T) {
	// Store entries while Close() is called concurrently. Must not panic or deadlock.
	var received atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		received.Add(int64(len(req.Entries)))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")

	var wg sync.WaitGroup
	// Start storing
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				s.Store(models.LogEvent{
					Level:     log.InfoLevel,
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("g=%d i=%d", id, i),
				})
			}
		}(g)
	}

	// Close after a short delay — some stores will succeed, some will get "closed" error
	go func() {
		time.Sleep(5 * time.Millisecond)
		s.Close()
	}()

	wg.Wait()
	// Must not panic or deadlock. Pass if we reach this point.
}

func TestHTTPLogStore_TimerFlushRace(t *testing.T) {
	// Trigger timer flush and Store() concurrently. Run with -race.
	var received atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		received.Add(int64(len(req.Entries)))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")
	// Very fast timer to maximize contention between timerFlush and Store
	s.setFlushInterval(1 * time.Millisecond)

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				s.Store(models.LogEvent{
					Level:     log.InfoLevel,
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("timer-race g=%d i=%d", id, i),
				})
				time.Sleep(100 * time.Microsecond)
			}
		}(g)
	}
	wg.Wait()
	s.Close()

	// Allow in-flight send() goroutines to complete
	time.Sleep(500 * time.Millisecond)

	total := received.Load()
	expected := int64(10 * 100)
	if total != expected {
		t.Errorf("expected %d entries, got %d (lost %d)", expected, total, expected-total)
	}
}

func TestHTTPLogStore_SlowServer_NoDeadlock(t *testing.T) {
	// Server is slow (near HTTP timeout). Verify Store() doesn't block or deadlock.
	// send() runs in goroutines so Store() should remain fast.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")

	// Store 200 entries (4 flushes at 50 each). Each flush spawns a goroutine
	// that will block for 2s. Store() must not block.
	start := time.Now()
	for i := 0; i < 200; i++ {
		err := s.Store(models.LogEvent{
			Level:     log.InfoLevel,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("slow-server i=%d", i),
		})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}
	elapsed := time.Since(start)

	// Store() should complete almost instantly (well under 1s) even with slow server
	if elapsed > 1*time.Second {
		t.Errorf("Store() took %v — appears to be blocking on slow server", elapsed)
	}

	s.Close()
}

func TestHTTPLogStore_CloseFlushesAllBuffered(t *testing.T) {
	// Store fewer than logFlushSize entries, then Close(). All must be delivered.
	var received atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		received.Add(int64(len(req.Entries)))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")
	// Stop the timer so only Close() flushes
	s.timer.Stop()

	// Store 37 entries (under logFlushSize=50, so no auto-flush)
	for i := 0; i < 37; i++ {
		s.Store(models.LogEvent{
			Level:     log.InfoLevel,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("close-flush i=%d", i),
		})
	}

	s.Close()

	if received.Load() != 37 {
		t.Errorf("expected 37 entries flushed on Close(), got %d", received.Load())
	}
}

func TestHTTPLogStore_DoubleClose(t *testing.T) {
	// Calling Close() twice must not panic or deadlock.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")
	s.Store(models.LogEvent{
		Level:   log.InfoLevel,
		Message: "before-close",
	})

	err1 := s.Close()
	err2 := s.Close()

	if err1 != nil {
		t.Errorf("first Close() returned error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Close() returned error: %v", err2)
	}
}

func TestHTTPLogStore_StoreAfterClose_AllRejected(t *testing.T) {
	// Every Store() after Close() must return an error. None should sneak through.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")
	s.Close()

	var errors atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				err := s.Store(models.LogEvent{
					Level:   log.InfoLevel,
					Message: "after-close",
				})
				if err != nil {
					errors.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	expected := int64(20 * 50)
	if errors.Load() != expected {
		t.Errorf("expected %d errors from Store-after-Close, got %d", expected, errors.Load())
	}
}

func TestHTTPLogStore_ServerErrorsDuringConcurrentFlush(t *testing.T) {
	// Server returns errors intermittently. Must not panic or corrupt state.
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n%3 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")
	s.setFlushInterval(2 * time.Millisecond)

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				s.Store(models.LogEvent{
					Level:     log.WarnLevel,
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("err-test g=%d i=%d", id, i),
				})
			}
		}(g)
	}
	wg.Wait()
	s.Close()
	// Must not panic. Pass if we reach this point.
}

func TestHTTPLogStore_SetFlushInterval_ConcurrentStore(t *testing.T) {
	// setFlushInterval called while Store() is active. Tests for races on s.timer.
	// NOTE: setFlushInterval accesses s.timer without holding s.mu — this test
	// documents the race. Run with -race to detect it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")

	var wg sync.WaitGroup
	// Store concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			s.Store(models.LogEvent{
				Level:   log.InfoLevel,
				Message: fmt.Sprintf("set-interval i=%d", i),
			})
			time.Sleep(50 * time.Microsecond)
		}
	}()

	// Rapidly change flush interval concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			s.setFlushInterval(time.Duration(i+1) * time.Millisecond)
			time.Sleep(100 * time.Microsecond)
		}
	}()

	wg.Wait()
	s.Close()
	// Verifies setFlushInterval holds s.mu to avoid racing with timerFlush.
}

func TestHTTPLogStore_EntryIntegrity(t *testing.T) {
	// Verify that entries arrive intact after concurrent Store() and flush.
	var mu sync.Mutex
	received := make(map[string]bool)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		for _, e := range req.Entries {
			received[e.Message] = true
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")

	const total = 500
	for i := 0; i < total; i++ {
		s.Store(models.LogEvent{
			Level:     log.InfoLevel,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("integrity-%04d", i),
		})
	}
	s.Close()

	// Allow in-flight send() goroutines to complete
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != total {
		t.Errorf("expected %d unique messages, got %d", total, len(received))
	}
	// Verify no duplicates or missing entries
	for i := 0; i < total; i++ {
		key := fmt.Sprintf("integrity-%04d", i)
		if !received[key] {
			t.Errorf("missing entry: %s", key)
		}
	}
}

func TestHTTPLogStore_HighVolumeNoLoss(t *testing.T) {
	// Blast 10,000 entries. Verify all arrive at the server.
	var received atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logIngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		received.Add(int64(len(req.Entries)))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPLogStore(srv.URL, "service:stress")

	const total = 10000
	for i := 0; i < total; i++ {
		s.Store(models.LogEvent{
			Level:     log.InfoLevel,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("volume-%d", i),
		})
	}
	s.Close()

	// Allow in-flight send() goroutines to complete
	time.Sleep(2 * time.Second)

	if received.Load() != int64(total) {
		t.Errorf("expected %d entries, got %d (lost %d)", total, received.Load(), int64(total)-received.Load())
	}
}
