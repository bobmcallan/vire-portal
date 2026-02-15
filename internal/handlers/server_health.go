package handlers

import (
	"context"
	"net/http"
	"time"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// ServerHealthHandler proxies health checks to the upstream vire-server.
type ServerHealthHandler struct {
	logger *common.Logger
	apiURL string
}

// NewServerHealthHandler creates a new server health handler.
func NewServerHealthHandler(logger *common.Logger, apiURL string) *ServerHealthHandler {
	return &ServerHealthHandler{logger: logger, apiURL: apiURL}
}

// ServeHTTP handles GET /api/server-health.
func (h *ServerHealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", h.apiURL+"/api/health", nil)
	if err != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down"})
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down"})
}
