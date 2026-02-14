package handlers

import (
	"net/http"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	logger *common.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(logger *common.Logger) *HealthHandler {
	return &HealthHandler{logger: logger}
}

// ServeHTTP handles GET /api/health.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
