package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Handler is the HTTP handler for the MCP endpoint.
// It wraps mcp-go's StreamableHTTPServer and delegates to it.
type Handler struct {
	streamable *mcpserver.StreamableHTTPServer
	logger     *common.Logger
	catalog    []CatalogTool
	jwtSecret  []byte
}

// catalogRetryDelay is the delay between retry attempts.
const catalogRetryDelay = 2 * time.Second

// NewHandler creates a new MCP handler with dynamic tool registration from vire-server.
func NewHandler(cfg *config.Config, logger *common.Logger) *Handler {
	mcpSrv := mcpserver.NewMCPServer(
		"vire-portal",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	proxy := NewMCPProxy(cfg.API.URL, logger, cfg)

	// Fetch tool catalog from vire-server with retry (non-fatal if unreachable)
	maxAttempts := cfg.MCP.CatalogRetries
	if maxAttempts < 0 {
		maxAttempts = 0
	}
	var catalog []CatalogTool
	var fetchErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		catalog, fetchErr = proxy.FetchCatalog(ctx)
		cancel()
		if fetchErr == nil {
			break
		}
		logger.Warn().
			Int("attempt", attempt).
			Int("max_attempts", maxAttempts).
			Str("error", fetchErr.Error()).
			Str("api_url", cfg.API.URL).
			Msg("failed to fetch tool catalog, retrying")
		if attempt < maxAttempts {
			time.Sleep(catalogRetryDelay)
		}
	}

	var validated []CatalogTool
	var toolCount int
	if fetchErr != nil {
		logger.Warn().
			Int("attempts", maxAttempts).
			Str("error", fetchErr.Error()).
			Str("api_url", cfg.API.URL).
			Msg("failed to fetch tool catalog after retries, starting with 0 tools")
	} else {
		validated = ValidateCatalog(catalog, logger)
		toolCount = RegisterToolsFromCatalog(mcpSrv, proxy, validated)
	}

	// Override get_version with combined handler that includes both
	// vire-portal and vire-server version info.
	mcpSrv.AddTool(VersionTool(), VersionToolHandler(proxy))

	streamable := mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithStateLess(true),
	)

	logger.Info().
		Int("tools", toolCount).
		Str("api_url", cfg.API.URL).
		Msg("MCP handler initialized")

	return &Handler{
		streamable: streamable,
		logger:     logger,
		catalog:    validated,
		jwtSecret:  []byte(cfg.Auth.JWTSecret),
	}
}

// Catalog returns a copy of the validated tool catalog.
func (h *Handler) Catalog() []CatalogTool {
	result := make([]CatalogTool, len(h.catalog))
	copy(result, h.catalog)
	return result
}

// ServeHTTP extracts user context from the session cookie (if present)
// and delegates to the mcp-go StreamableHTTPServer.
// If no valid user context is found, returns 401 with WWW-Authenticate header
// per RFC 9728 to trigger OAuth discovery.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = h.withUserContext(r)

	// Check if user context exists - if not, require authentication per RFC 9728
	if _, ok := GetUserContext(r.Context()); !ok {
		scheme := "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}

		// Sanitize host to prevent header injection attacks
		host := sanitizeHost(r.Host)
		resourceMetadata := fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource",
			scheme, host)

		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer resource_metadata="%s"`, resourceMetadata))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "unauthorized",
			"error_description": "Authentication required to access MCP endpoint",
		})
		return
	}

	h.streamable.ServeHTTP(w, r)
}

// sanitizeHost removes dangerous characters from the Host header to prevent
// header injection attacks. It strips CR, LF, and quote characters.
func sanitizeHost(host string) string {
	// Remove CR and LF to prevent header injection
	host = strings.ReplaceAll(host, "\r", "")
	host = strings.ReplaceAll(host, "\n", "")
	// Remove quotes to prevent breaking out of the resource_metadata value
	host = strings.ReplaceAll(host, `"`, "")
	return host
}

// withUserContext extracts user identity from Bearer token or vire_session cookie,
// validates the JWT (signature + expiry), and attaches UserContext to the request context.
// Bearer token takes priority (Claude CLI/Desktop), cookie is fallback (web dashboard).
// If anything fails, the original request is returned unchanged.
func (h *Handler) withUserContext(r *http.Request) *http.Request {
	// Try Bearer token first (Claude CLI/Desktop)
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := validateJWT(token, h.jwtSecret)
		if err == nil && claims.Sub != "" {
			ctx := WithUserContext(r.Context(), UserContext{UserID: claims.Sub})
			return r.WithContext(ctx)
		}
	}

	// Fall back to cookie (web dashboard)
	cookie, err := r.Cookie("vire_session")
	if err != nil || cookie.Value == "" {
		return r
	}

	// For cookie-based auth, use the same JWT validation.
	// If jwtSecret is empty, signature check is skipped (dev mode backwards compat).
	claims, err := validateJWT(cookie.Value, h.jwtSecret)
	if err == nil && claims.Sub != "" {
		ctx := WithUserContext(r.Context(), UserContext{UserID: claims.Sub})
		return r.WithContext(ctx)
	}

	// Legacy fallback: extract sub without validation when no JWT secret is configured.
	// This preserves backwards compat for dev setups where vire-server issues
	// tokens with a different or no secret.
	if len(h.jwtSecret) == 0 {
		sub := extractJWTSub(cookie.Value)
		if sub != "" {
			ctx := WithUserContext(r.Context(), UserContext{UserID: sub})
			return r.WithContext(ctx)
		}
	}

	return r
}

// jwtClaims holds decoded JWT payload claims for MCP Bearer token validation.
type jwtClaims struct {
	Sub      string `json:"sub"`
	Exp      int64  `json:"exp"`
	Iss      string `json:"iss"`
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
}

// validateJWT validates a JWT token: checks format, verifies HMAC-SHA256 signature
// (if secret is non-empty), and checks expiry. Returns claims on success.
func validateJWT(token string, secret []byte) (*jwtClaims, error) {
	parts := strings.SplitN(token, ".", 4)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}

	// Verify signature if secret is non-empty
	if len(secret) > 0 {
		sigInput := parts[0] + "." + parts[1]
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(sigInput))
		expectedSig := mac.Sum(nil)

		actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid JWT signature encoding: %w", err)
		}

		if !hmac.Equal(expectedSig, actualSig) {
			return nil, fmt.Errorf("invalid JWT signature")
		}
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid JWT payload: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("invalid JWT JSON: %w", err)
	}

	// Check expiry
	if claims.Exp > 0 && claims.Exp < time.Now().Unix() {
		return nil, fmt.Errorf("JWT expired")
	}

	return &claims, nil
}

// extractJWTSub base64url-decodes the JWT payload (middle segment)
// and returns the "sub" claim. Returns empty string on any failure.
// Used only as legacy fallback when no JWT secret is configured.
func extractJWTSub(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}

	return claims.Sub
}
