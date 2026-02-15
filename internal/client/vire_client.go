package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// UserProfile holds the user profile returned by vire-server.
type UserProfile struct {
	Username         string `json:"username"`
	Email            string `json:"email"`
	Role             string `json:"role"`
	NavexaKeySet     bool   `json:"navexa_key_set"`
	NavexaKeyPreview string `json:"navexa_key_preview"`
}

// VireClient communicates with the vire-server REST API.
type VireClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewVireClient creates a new client targeting the given vire-server URL.
func NewVireClient(baseURL string) *VireClient {
	return &VireClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetUser fetches user profile from vire-server.
// GET /api/users/{id} -> { status: "ok", data: UserProfile }
func (c *VireClient) GetUser(userID string) (*UserProfile, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/users/" + userID)
	if err != nil {
		return nil, fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found: %s", userID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status string      `json:"status"`
		Data   UserProfile `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Data, nil
}

// UpdateUser updates user fields on vire-server.
// PUT /api/users/{id} with JSON body -> { status: "ok", data: UserProfile }
func (c *VireClient) UpdateUser(userID string, fields map[string]string) (*UserProfile, error) {
	jsonData, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, c.baseURL+"/api/users/"+userID, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status string      `json:"status"`
		Data   UserProfile `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Data, nil
}
