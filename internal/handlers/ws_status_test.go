package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestWSStatusHandler_Upgrade(t *testing.T) {
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://localhost:0")

	srv := httptest.NewServer(h)
	defer srv.Close()

	conn, _, _, err := ws.Dial(testCtx(t), "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/status")
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write ping failed: %v", err)
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result.Portal != "up" {
		t.Errorf("expected portal=up, got %s", result.Portal)
	}
}

func TestWSStatusHandler_PingResponse(t *testing.T) {
	fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeSrv.Close()

	h := NewWSStatusHandler(common.NewSilentLogger(), fakeSrv.URL)

	srv := httptest.NewServer(h)
	defer srv.Close()

	conn, _, _, err := ws.Dial(testCtx(t), "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/status")
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write ping failed: %v", err)
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result.Portal != "up" {
		t.Errorf("expected portal=up, got %s", result.Portal)
	}
	if result.Server != "up" {
		t.Errorf("expected server=up, got %s", result.Server)
	}
}

func TestWSStatusHandler_ServerHealthCaching(t *testing.T) {
	callCount := 0
	fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeSrv.Close()

	h := NewWSStatusHandler(common.NewSilentLogger(), fakeSrv.URL)

	srv := httptest.NewServer(h)
	defer srv.Close()

	conn, _, _, err := ws.Dial(testCtx(t), "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/status")
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// First ping
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write ping failed: %v", err)
	}
	if _, err := wsutil.ReadServerText(conn); err != nil {
		t.Fatalf("read response 1 failed: %v", err)
	}

	// Second ping immediately (should use cache)
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write ping 2 failed: %v", err)
	}
	if _, err := wsutil.ReadServerText(conn); err != nil {
		t.Fatalf("read response 2 failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 upstream health check (cached), got %d", callCount)
	}
}

func TestWSStatusHandler_ServerDown(t *testing.T) {
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://127.0.0.1:1")

	srv := httptest.NewServer(h)
	defer srv.Close()

	conn, _, _, err := ws.Dial(testCtx(t), "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/status")
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write ping failed: %v", err)
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result.Portal != "up" {
		t.Errorf("expected portal=up, got %s", result.Portal)
	}
	if result.Server != "down" {
		t.Errorf("expected server=down, got %s", result.Server)
	}
}

func TestWSStatusHandler_InvalidMessage(t *testing.T) {
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://localhost:0")

	srv := httptest.NewServer(h)
	defer srv.Close()

	conn, _, _, err := ws.Dial(testCtx(t), "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/status")
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// Send non-ping message
	if err := wsutil.WriteClientText(conn, []byte("hello")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Send a valid ping after — if the connection is still alive, we get a response
	if err := wsutil.WriteClientText(conn, []byte("ping")); err != nil {
		t.Fatalf("write ping failed: %v", err)
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}

	var result wsHealthResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result.Portal != "up" {
		t.Errorf("expected portal=up, got %s", result.Portal)
	}
}

func TestWSStatusHandler_NonWebSocketRequest(t *testing.T) {
	h := NewWSStatusHandler(common.NewSilentLogger(), "http://localhost:0")

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ws/status")
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for non-WebSocket request, got %d", resp.StatusCode)
	}
}
