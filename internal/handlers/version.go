package handlers

import (
	"encoding/json"
	"net/http"
	"time"

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

// GetServerVersion fetches the version from the vire-server API.
// Returns the version string on success, or "unavailable" on any error.
func GetServerVersion(apiURL string) string {
	if apiURL == "" {
		return "unavailable"
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(apiURL + "/api/version")
	if err != nil {
		return "unavailable"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "unavailable"
	}

	var data struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "unavailable"
	}

	if data.Version == "" {
		return "unavailable"
	}

	return data.Version
}
