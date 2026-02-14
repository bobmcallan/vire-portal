package handlers

import (
	"log/slog"
	"net/http"

	"github.com/bobmcallan/vire-portal/internal/config"
)

// VersionHandler handles version information requests.
type VersionHandler struct {
	logger *slog.Logger
}

// NewVersionHandler creates a new version handler.
func NewVersionHandler(logger *slog.Logger) *VersionHandler {
	return &VersionHandler{logger: logger}
}

// ServeHTTP handles GET /api/version.
func (h *VersionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"version":    config.GetVersion(),
		"build":      config.GetBuild(),
		"git_commit": config.GetGitCommit(),
	})
}
