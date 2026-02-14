package handlers

import (
	"net/http"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// VersionHandler handles version information requests.
type VersionHandler struct {
	logger *common.Logger
}

// NewVersionHandler creates a new version handler.
func NewVersionHandler(logger *common.Logger) *VersionHandler {
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
