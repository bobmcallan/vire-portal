package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// --- Stress Tests: Connection Exhaustion ---

func TestWSStatus_StressConcurrentConnections(t *testing.T) {
	// Many simultaneous WebSocket connections should not crash the server
	// or cause goroutine leaks.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	const numConns = 50
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	var wg sync.WaitGroup
	var connected int32
	var errors int32

	for i := 0; i < numConns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, _, _, err := ws.Dial(context.Background(), wsURL)
			if err != nil {
				atomic.AddInt32(&errors, 1)
				return
			}
			atomic.AddInt32(&connected, 1)
			defer conn.Close()

			// Send a ping and read response
			if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
				return
			}
			data, err := wsutil.ReadServerText(conn)
			if err != nil {
				return
			}
			var result wsHealthResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Errorf("malformed response from concurrent connection: %s", truncStr(string(data), 100))
			}
		}()
	}

	wg.Wait()

	if connected == 0 {
		t.Error("no connections succeeded out of 50 — server may be rejecting")
	}
	t.Logf("connected: %d, errors: %d", connected, errors)
}

// --- Stress Tests: Goroutine Leak ---

func TestWSStatus_StressNoGoroutineLeak(t *testing.T) {
	// After all connections close, goroutine count should return to baseline.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	const numConns = 20
	conns := make([]interface{ Close() error }, 0, numConns)

	for i := 0; i < numConns; i++ {
		conn, _, _, err := ws.Dial(context.Background(), wsURL)
		if err != nil {
			continue
		}
		conns = append(conns, conn)
	}

	// Close all connections
	for _, c := range conns {
		c.Close()
	}

	// Allow goroutines to wind down
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - baseline
	// Allow some slack (GC, timers, etc.) but not 20 leaked goroutines
	if leaked > 5 {
		t.Errorf("RESOURCE LEAK: %d goroutines leaked after closing %d WebSocket connections (baseline=%d, after=%d)",
			leaked, len(conns), baseline, after)
	}
}

// --- Stress Tests: Oversized Messages ---

func TestWSStatus_StressOversizedMessage(t *testing.T) {
	// Sending a very large message should not crash the server or consume
	// excessive memory. The server should handle it gracefully.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send a 1MB message — server should not crash
	bigMsg := strings.Repeat("A", 1<<20)
	err = wsutil.WriteClientText(conn, []byte(bigMsg))
	if err != nil {
		// Write error is acceptable (server may have closed)
		return
	}

	// The server should either ignore the message or close the connection.
	// Either way, a subsequent ping should either work or fail cleanly.
	_ = wsutil.WriteClientText(conn, []byte("ping"))
	// We don't check the response — the connection may be closed, which is fine.
}

// --- Stress Tests: Rapid Connect/Disconnect ---

func TestWSStatus_StressRapidConnectDisconnect(t *testing.T) {
	// Rapidly connecting and immediately disconnecting should not leak resources.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	for i := 0; i < 100; i++ {
		conn, _, _, err := ws.Dial(context.Background(), wsURL)
		if err != nil {
			continue
		}
		conn.Close()
	}

	// If we get here without panic or hang, the test passes.
}

// --- Stress Tests: Race Condition in Health Cache ---

func TestWSStatus_StressCacheConcurrency(t *testing.T) {
	// Multiple goroutines calling getStatus concurrently must not race.
	callCount := int32(0)
	fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeSrv.Close()

	h := NewWSStatusHandler(common.NewSilentLogger(), fakeSrv.URL)

	var wg sync.WaitGroup
	const numGoroutines = 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := h.getStatus()
			if result.Portal != "up" {
				t.Errorf("expected portal=up, got %s", result.Portal)
			}
			if result.Server != "up" {
				t.Errorf("expected server=up, got %s", result.Server)
			}
		}()
	}

	wg.Wait()

	// With proper caching and double-check locking, the upstream server
	// should only be called a small number of times, not 100.
	calls := atomic.LoadInt32(&callCount)
	if calls > 5 {
		t.Errorf("CACHE RACE: upstream called %d times for %d concurrent requests (expected <= 5)", calls, numGoroutines)
	}
}

// --- Stress Tests: XSS via WebSocket Response ---

func TestWSStatus_StressNoXSSInResponse(t *testing.T) {
	// The server response must be valid JSON, not raw HTML/script.
	// Even with a malicious upstream, the portal field is hardcoded "up".
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	response := string(data)

	// Must be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("SECURITY: response is not valid JSON: %s", truncStr(response, 200))
	}

	// Must not contain HTML/script tags
	if strings.Contains(response, "<script") || strings.Contains(response, "<img") || strings.Contains(response, "javascript:") {
		t.Errorf("SECURITY: response contains potential XSS payload: %s", truncStr(response, 200))
	}

	// portal and server fields should only be "up" or "down"
	for _, field := range []string{"portal", "server"} {
		val, ok := result[field].(string)
		if !ok {
			t.Errorf("field %q missing or not a string", field)
			continue
		}
		if val != "up" && val != "down" {
			t.Errorf("SECURITY: field %q has unexpected value %q (expected up/down)", field, truncStr(val, 50))
		}
	}
}

// --- Stress Tests: Binary Frame Handling ---

func TestWSStatus_StressBinaryFrame(t *testing.T) {
	// Sending binary frames instead of text should not crash the server.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send binary frame
	if err := wsutil.WriteClientBinary(conn, []byte{0x00, 0x01, 0x02, 0xFF}); err != nil {
		return // Write error is acceptable
	}

	// Connection should still be alive — send a valid ping
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		// Connection may be closed after binary frame, which is acceptable
		return
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		// Server may have closed after binary frame, which is acceptable
		return
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("response after binary frame is malformed: %s", truncStr(string(data), 100))
	}
}

// --- Stress Tests: Non-Ping Message Flood ---

func TestWSStatus_StressNonPingFlood(t *testing.T) {
	// Flooding with non-ping messages should not cause CPU spin or memory growth.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send 1000 non-ping messages rapidly
	for i := 0; i < 1000; i++ {
		if err := wsutil.WriteClientText(conn, []byte("not-a-ping")); err != nil {
			break
		}
	}

	// Connection should still work for a valid ping
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		// Connection may have been closed, which is acceptable for a flood
		return
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		return // Acceptable if server closed after flood
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("response after non-ping flood is malformed: %s", truncStr(string(data), 100))
	}
}

// --- Stress Tests: Empty and Whitespace Messages ---

func TestWSStatus_StressEmptyAndWhitespaceMessages(t *testing.T) {
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	edgeCases := []string{
		"",
		" ",
		"\t",
		"\n",
		"  ping  ",
		"PING",
		"Ping",
		"ping\n",
		"ping\x00",
	}

	for _, msg := range edgeCases {
		if err := wsutil.WriteClientText(conn, []byte(msg)); err != nil {
			return // Connection closed, acceptable
		}
	}

	// None of those should have generated a response. A valid ping should still work.
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		return
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		return
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("response after edge-case messages is malformed: %s", truncStr(string(data), 100))
	}
	if result.Portal != "up" {
		t.Errorf("expected portal=up, got %s", result.Portal)
	}
}

// --- Stress Tests: CSWSH (Cross-Site WebSocket Hijacking) ---

func TestWSStatus_StressCrossOriginAccess(t *testing.T) {
	// The WebSocket endpoint is intentionally unauthenticated (mirrors health endpoints).
	// This test documents that any origin can connect — which is by design since it
	// only exposes up/down status, no user data. If this changes in the future,
	// origin validation should be added.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	// gobwas/ws.Dial doesn't set Origin by default, but the server
	// doesn't check it either. This is acceptable because the endpoint
	// exposes no sensitive data.
	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}

	// The response should only contain status fields, never user data
	raw := make(map[string]interface{})
	json.Unmarshal(data, &raw)
	for key := range raw {
		if key != "portal" && key != "server" {
			t.Errorf("SECURITY: unexpected field %q in response — may leak data to cross-origin callers", key)
		}
	}
}

// --- Stress Tests: HTTP Method Rejection ---

func TestWSStatus_StressNonGETMethodsRejected(t *testing.T) {
	// WebSocket upgrade only works on GET. Other methods should fail.
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	methods := []string{"POST", "PUT", "DELETE", "PATCH", "HEAD"}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/ws/status", nil)
		w := httptest.NewRecorder()

		h.ServeHTTP(w, req)

		if w.Code == http.StatusSwitchingProtocols {
			t.Errorf("SECURITY: %s request succeeded WebSocket upgrade", method)
		}
	}
}

// --- Stress Tests: Cache Expiry ---

func TestWSStatus_StressCacheExpiry(t *testing.T) {
	// After cacheTTL expires, the server should make a new upstream check.
	callCount := int32(0)
	fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeSrv.Close()

	h := NewWSStatusHandler(common.NewSilentLogger(), fakeSrv.URL)
	// Override cacheTTL to be very short for testing
	h.cacheTTL = 50 * time.Millisecond

	srv := httptest.NewServer(h)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/status"

	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// First ping — should trigger upstream check
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := wsutil.ReadServerText(conn); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	firstCalls := atomic.LoadInt32(&callCount)
	if firstCalls != 1 {
		t.Errorf("expected 1 upstream call after first ping, got %d", firstCalls)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Second ping — should trigger a new upstream check
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := wsutil.ReadServerText(conn); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	secondCalls := atomic.LoadInt32(&callCount)
	if secondCalls < 2 {
		t.Errorf("expected >= 2 upstream calls after cache expiry, got %d", secondCalls)
	}
}

// --- Stress Tests: New HTTP Client Per Health Check ---

func TestWSStatus_StressHealthCheckCreatesNewClient(t *testing.T) {
	// checkServerHealth creates a new http.Client each call. Under load with
	// many concurrent getStatus calls that miss the cache, this could create
	// many short-lived transports. This test verifies correctness under load.
	callCount := int32(0)
	fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(10 * time.Millisecond) // Simulate slow upstream
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeSrv.Close()

	h := NewWSStatusHandler(common.NewSilentLogger(), fakeSrv.URL)
	h.cacheTTL = 0 // Disable caching to stress the health check path

	var wg sync.WaitGroup
	const numGoroutines = 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := h.getStatus()
			if result.Portal != "up" {
				t.Errorf("expected portal=up, got %s", result.Portal)
			}
		}()
	}

	wg.Wait()
	// Test passes if no panics, deadlocks, or goroutine leaks
}
