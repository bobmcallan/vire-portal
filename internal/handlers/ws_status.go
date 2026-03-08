package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// WSStatusHandler serves WebSocket connections for status indicators.
// Clients send "ping" text messages; the server responds with JSON health status.
type WSStatusHandler struct {
	logger    *common.Logger
	apiURL    string
	mu        sync.RWMutex
	cached    wsHealthResult
	cacheTime time.Time
	cacheTTL  time.Duration
}

type wsHealthResult struct {
	Portal string `json:"portal"`
	Server string `json:"server"`
}

// NewWSStatusHandler creates a new WebSocket status handler.
func NewWSStatusHandler(logger *common.Logger, apiURL string) *WSStatusHandler {
	return &WSStatusHandler{
		logger:   logger,
		apiURL:   apiURL,
		cacheTTL: 5 * time.Second,
	}
}

// ServeHTTP upgrades to WebSocket and handles ping/response loop.
func (h *WSStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		msg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			break
		}

		if op != ws.OpText || string(msg) != "ping" {
			continue
		}

		status := h.getStatus()
		data, _ := json.Marshal(status)
		if err := wsutil.WriteServerMessage(conn, ws.OpText, data); err != nil {
			break
		}
	}
}

// getStatus returns the cached health result, refreshing if stale.
func (h *WSStatusHandler) getStatus() wsHealthResult {
	h.mu.RLock()
	if time.Since(h.cacheTime) < h.cacheTTL {
		result := h.cached
		h.mu.RUnlock()
		return result
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(h.cacheTime) < h.cacheTTL {
		return h.cached
	}

	h.cached = wsHealthResult{
		Portal: "up",
		Server: h.checkServerHealth(),
	}
	h.cacheTime = time.Now()
	return h.cached
}

// checkServerHealth checks if the upstream vire-server is healthy.
func (h *WSStatusHandler) checkServerHealth() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", h.apiURL+"/api/health", nil)
	if err != nil {
		return "down"
	}

	resp, err := (&http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
		},
	}).Do(req)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "up"
	}
	return "down"
}
