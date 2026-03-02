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
	apiURL string
}

// NewVersionHandler creates a new version handler.
func NewVersionHandler(logger *common.Logger) *VersionHandler {
	return &VersionHandler{logger: logger}
}

// SetAPIURL sets the upstream API URL for server version fetching.
func (h *VersionHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// ServeHTTP handles GET /api/version.
// Returns portal version fields alongside server version fields.
func (h *VersionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	resp := map[string]string{
		"portal_version": config.GetVersion(),
		"portal_build":   config.GetBuild(),
		"portal_commit":  config.GetGitCommit(),
	}

	// Fetch and merge server version
	serverVersion := fetchServerVersionFields(h.apiURL)
	for k, v := range serverVersion {
		resp[k] = v
	}

	WriteJSON(w, http.StatusOK, resp)
}

// GetServerVersion fetches the version from the vire-server API.
// Returns the version string on success, or "unavailable" on any error.
func GetServerVersion(apiURL string) string {
	fields := fetchServerVersionFields(apiURL)
	if v, ok := fields["version"]; ok {
		return v
	}
	return "unavailable"
}

// fetchServerVersionFields fetches version fields from the vire-server API.
// Returns a map of field names to values, or an empty map on error.
func fetchServerVersionFields(apiURL string) map[string]string {
	if apiURL == "" {
		return map[string]string{"version": "unavailable"}
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(apiURL + "/api/version")
	if err != nil {
		return map[string]string{"version": "unavailable"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]string{"version": "unavailable"}
	}

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return map[string]string{"version": "unavailable"}
	}

	if data["version"] == "" {
		data["version"] = "unavailable"
	}

	return data
}
