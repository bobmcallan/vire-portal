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

// versionTransport injects X-Portal-Version and X-Portal-Build headers on every request.
type versionTransport struct {
	base    http.RoundTripper
	version string
	build   string
}

func (t *versionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.version != "" {
		req.Header.Set("X-Portal-Version", t.version)
	}
	if t.build != "" {
		req.Header.Set("X-Portal-Build", t.build)
	}
	return t.base.RoundTrip(req)
}

// VireClient communicates with the vire-server REST API.
type VireClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewVireClient creates a new client targeting the given vire-server URL.
// Version and build strings are injected as headers on every request.
func NewVireClient(baseURL, version, build string) *VireClient {
	return &VireClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &versionTransport{
				base:    http.DefaultTransport,
				version: version,
				build:   build,
			},
		},
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

// ListUsers fetches all user profiles from vire-server.
// GET /api/users -> { status: "ok", data: []UserProfile }
func (c *VireClient) ListUsers() ([]UserProfile, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/users")
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
		Status string        `json:"status"`
		Data   []UserProfile `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// OAuthResponse holds the token and user profile from an OAuth exchange.
type OAuthResponse struct {
	Token string      `json:"token"`
	User  UserProfile `json:"user"`
}

// ExchangeOAuth exchanges OAuth credentials for a JWT token via vire-server.
// POST /api/auth/oauth with { provider, code, state } -> { status: "ok", data: OAuthResponse }
func (c *VireClient) ExchangeOAuth(provider, code, state string) (*OAuthResponse, error) {
	body := map[string]string{
		"provider": provider,
		"code":     code,
		"state":    state,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/auth/oauth", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Status string        `json:"status"`
		Data   OAuthResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Data, nil
}

// SeedUser holds the fields needed to create or update a user during dev seeding.
type SeedUser struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Role      string `json:"role"`
	NavexaKey string `json:"navexa_key"`
}

// UpsertUser creates or updates a user on vire-server.
// POST /api/users/upsert with JSON body -> 200 or 201
func (c *VireClient) UpsertUser(user SeedUser) error {
	jsonData, err := json.Marshal(user)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/users/upsert", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AdminUser holds a user profile returned by the admin API endpoint.
type AdminUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Provider  string `json:"provider"`
	CreatedAt string `json:"created_at"`
}

// RegisterService registers this portal instance as a service user with vire-server.
// POST /api/services/register -> { status: "ok", service_user_id: "...", registered_at: "..." }
func (c *VireClient) RegisterService(serviceID, serviceKey string) (string, error) {
	body := map[string]string{
		"service_id":   serviceID,
		"service_key":  serviceKey,
		"service_type": "portal",
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/services/register", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ServiceUserID string `json:"service_user_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.ServiceUserID, nil
}

// AdminListUsers fetches all users via the admin API endpoint.
// GET /api/admin/users with X-Vire-Service-ID header -> { users: [...] }
func (c *VireClient) AdminListUsers(serviceID string) ([]AdminUser, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/admin/users", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vire-Service-ID", serviceID)

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
		Users []AdminUser `json:"users"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Users, nil
}

// AdminPrompt holds a prompt template returned by the admin API endpoint.
type AdminPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Hash        string `json:"hash"`
	IsOverride  bool   `json:"is_override"`
	UpdatedAt   string `json:"updated_at"`
}

// AdminListPrompts fetches all prompt templates via the admin API endpoint.
// GET /api/admin/prompts with X-Vire-User-ID header (admin role checked server-side)
func (c *VireClient) AdminListPrompts(userID string) ([]AdminPrompt, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/admin/prompts", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vire-User-ID", userID)

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

	// Try raw array first (vire-server returns [...]), fall back to wrapped {"prompts":[...]}
	var prompts []AdminPrompt
	if err := json.Unmarshal(body, &prompts); err != nil {
		var wrapped struct {
			Prompts []AdminPrompt `json:"prompts"`
		}
		if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		prompts = wrapped.Prompts
	}

	return prompts, nil
}

// AdminGetPrompt fetches a single prompt template via the admin API endpoint.
// GET /api/admin/prompts/{name} with X-Vire-User-ID header (admin role checked server-side)
func (c *VireClient) AdminGetPrompt(userID, name string) (*AdminPrompt, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/admin/prompts/"+name, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vire-User-ID", userID)

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

	var prompt AdminPrompt
	if err := json.Unmarshal(body, &prompt); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &prompt, nil
}

// AdminSetPrompt updates a prompt template's content via the admin API endpoint.
// PUT /api/admin/prompts/{name} with X-Vire-User-ID header, body: {"content":"..."}
func (c *VireClient) AdminSetPrompt(userID, name, content string) error {
	jsonData, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, c.baseURL+"/api/admin/prompts/"+name, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vire-User-ID", userID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AdminUpdateUserRole updates a user's role via the admin API endpoint.
// PATCH /api/admin/users/{id}/role with X-Vire-Service-ID header
func (c *VireClient) AdminUpdateUserRole(serviceID, userID, role string) error {
	jsonData, err := json.Marshal(map[string]string{"role": role})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, c.baseURL+"/api/admin/users/"+userID+"/role", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vire-Service-ID", serviceID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ProxyGet performs a GET request to vire-server at the given path,
// injecting the X-Vire-User-ID header for authentication.
// Returns the raw response body bytes on success (2xx), or an error.
func (c *VireClient) ProxyGet(path string, userID string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if userID != "" {
		req.Header.Set("X-Vire-User-ID", userID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
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

// ReportPortalVersion sends the portal version and build to vire-server.
// POST /api/portal/version with X-Vire-Service-ID header for auth.
func (c *VireClient) ReportPortalVersion(serviceID, version, build string) error {
	payload := map[string]string{
		"version": version,
		"build":   build,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/portal/version", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vire-Service-ID", serviceID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach vire-server: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
