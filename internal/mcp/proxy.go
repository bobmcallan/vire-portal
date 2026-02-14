package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// maxResponseSize caps the proxy response body to prevent OOM from unexpectedly large responses.
const maxResponseSize = 50 << 20 // 50MB

// MCPProxy connects MCP tool calls to the REST API on vire-server.
type MCPProxy struct {
	serverURL   string
	httpClient  *http.Client
	logger      *common.Logger
	userHeaders http.Header
}

// NewMCPProxy creates a new MCP proxy targeting the given vire-server URL.
// User config is converted to X-Vire-* headers injected on every request.
func NewMCPProxy(serverURL string, logger *common.Logger, cfg *config.Config) *MCPProxy {
	headers := make(http.Header)
	if len(cfg.User.Portfolios) > 0 {
		headers.Set("X-Vire-Portfolios", strings.Join(cfg.User.Portfolios, ","))
	}
	if cfg.User.DisplayCurrency != "" {
		headers.Set("X-Vire-Display-Currency", cfg.User.DisplayCurrency)
	}

	return &MCPProxy{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		logger:      logger,
		userHeaders: headers,
	}
}

// UserHeaders returns the configured X-Vire-* headers for testing.
func (p *MCPProxy) UserHeaders() http.Header {
	return p.userHeaders
}

// ServerURL returns the configured server URL.
func (p *MCPProxy) ServerURL() string {
	return p.serverURL
}

// applyUserHeaders copies user context headers onto an outgoing request.
func (p *MCPProxy) applyUserHeaders(req *http.Request) {
	for key, vals := range p.userHeaders {
		for _, v := range vals {
			req.Header.Set(key, v)
		}
	}
}

// get performs a GET request to the given path on vire-server.
func (p *MCPProxy) get(ctx context.Context, path string) ([]byte, error) {
	p.logger.Debug().Str("method", "GET").Str("path", path).Msg("proxy request")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	p.applyUserHeaders(req)

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		p.logger.Error().Str("method", "GET").Str("path", path).Int64("duration_ms", duration.Milliseconds()).Str("error", err.Error()).Msg("proxy request failed")
		return nil, fmt.Errorf("server request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	p.logger.Debug().Int("status", resp.StatusCode).Int64("duration_ms", duration.Milliseconds()).Msg("proxy response")

	if resp.StatusCode >= 400 {
		return nil, parseErrorResponse(resp.StatusCode, body)
	}

	return body, nil
}

// post performs a POST request with a JSON body to the given path on vire-server.
func (p *MCPProxy) post(ctx context.Context, path string, data interface{}) ([]byte, error) {
	return p.doJSON(ctx, http.MethodPost, path, data)
}

// put performs a PUT request with a JSON body to the given path on vire-server.
func (p *MCPProxy) put(ctx context.Context, path string, data interface{}) ([]byte, error) {
	return p.doJSON(ctx, http.MethodPut, path, data)
}

// patch performs a PATCH request with a JSON body to the given path on vire-server.
func (p *MCPProxy) patch(ctx context.Context, path string, data interface{}) ([]byte, error) {
	return p.doJSON(ctx, http.MethodPatch, path, data)
}

// del performs a DELETE request to the given path on vire-server.
func (p *MCPProxy) del(ctx context.Context, path string) ([]byte, error) {
	p.logger.Debug().Str("method", "DELETE").Str("path", path).Msg("proxy request")

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	p.applyUserHeaders(req)

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		p.logger.Error().Str("method", "DELETE").Str("path", path).Int64("duration_ms", duration.Milliseconds()).Str("error", err.Error()).Msg("proxy request failed")
		return nil, fmt.Errorf("server request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	p.logger.Debug().Int("status", resp.StatusCode).Int64("duration_ms", duration.Milliseconds()).Msg("proxy response")

	if resp.StatusCode >= 400 {
		return nil, parseErrorResponse(resp.StatusCode, body)
	}

	return body, nil
}

// doJSON performs an HTTP request with JSON body.
func (p *MCPProxy) doJSON(ctx context.Context, method, path string, data interface{}) ([]byte, error) {
	p.logger.Debug().Str("method", method).Str("path", path).Msg("proxy request")

	var bodyReader io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.serverURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	p.applyUserHeaders(req)

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		p.logger.Error().Str("method", method).Str("path", path).Int64("duration_ms", duration.Milliseconds()).Str("error", err.Error()).Msg("proxy request failed")
		return nil, fmt.Errorf("server request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	p.logger.Debug().Int("status", resp.StatusCode).Int64("duration_ms", duration.Milliseconds()).Msg("proxy response")

	if resp.StatusCode >= 400 {
		return nil, parseErrorResponse(resp.StatusCode, body)
	}

	return body, nil
}

// parseErrorResponse extracts a meaningful error message from an HTTP error response.
func parseErrorResponse(statusCode int, body []byte) error {
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("%s", errResp.Error)
	}
	return fmt.Errorf("server returned %d: %s", statusCode, string(body))
}
