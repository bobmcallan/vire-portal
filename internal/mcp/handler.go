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
	devMode    bool
	devUser    string
}

// catalogRetryAttempts is the number of times to retry fetching the catalog.
const catalogRetryAttempts = 3

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
	var catalog []CatalogTool
	var fetchErr error
	for attempt := 1; attempt <= catalogRetryAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		catalog, fetchErr = proxy.FetchCatalog(ctx)
		cancel()
		if fetchErr == nil {
			break
		}
		logger.Warn().
			Int("attempt", attempt).
			Int("max_attempts", catalogRetryAttempts).
			Str("error", fetchErr.Error()).
			Str("api_url", cfg.API.URL).
			Msg("failed to fetch tool catalog, retrying")
		if attempt < catalogRetryAttempts {
			time.Sleep(catalogRetryDelay)
		}
	}

	var validated []CatalogTool
	var toolCount int
	if fetchErr != nil {
		logger.Warn().
			Int("attempts", catalogRetryAttempts).
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

	h := &Handler{
		logger:    logger,
		catalog:   validated,
		jwtSecret: []byte(cfg.Auth.JWTSecret),
		devMode:   cfg.IsDevMode(),
		devUser:   "dev_user",
	}

	streamable := mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithStateLess(true),
		mcpserver.WithHTTPContextFunc(h.contextFunc),
	)
	h.streamable = streamable

	logger.Info().
		Int("tools", toolCount).
		Str("api_url", cfg.API.URL).
		Msg("MCP handler initialized")

	return h
}

// contextFunc is called by mcp-go to inject context values for tool handlers.
func (h *Handler) contextFunc(ctx context.Context, r *http.Request) context.Context {
	uc := h.extractUserContext(r)
	if uc.UserID != "" {
		return WithUserContext(ctx, uc)
	}
	return ctx
}

// extractUserContext extracts user identity from Bearer token or vire_session cookie.
// Returns UserContext with UserID if auth is found, or falls back to dev_user in dev mode.
func (h *Handler) extractUserContext(r *http.Request) UserContext {
	// Try Bearer token first (Claude CLI/Desktop)
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := validateJWT(token, h.jwtSecret)
		if err == nil && claims.Sub != "" {
			return UserContext{UserID: claims.Sub}
		}
	}

	// Fall back to cookie (web dashboard)
	cookie, err := r.Cookie("vire_session")
	if err == nil && cookie.Value != "" {
		claims, err := validateJWT(cookie.Value, h.jwtSecret)
		if err == nil && claims.Sub != "" {
			return UserContext{UserID: claims.Sub}
		}

		// Legacy fallback: extract sub without validation when no JWT secret
		if len(h.jwtSecret) == 0 {
			if sub := extractJWTSub(cookie.Value); sub != "" {
				return UserContext{UserID: sub}
			}
		}
	}

	// Dev mode fallback
	if h.devMode && h.devUser != "" {
		return UserContext{UserID: h.devUser}
	}

	return UserContext{}
}

// Catalog returns a copy of the validated tool catalog.
func (h *Handler) Catalog() []CatalogTool {
	result := make([]CatalogTool, len(h.catalog))
	copy(result, h.catalog)
	return result
}

// ServeHTTP extracts user context from the session cookie (if present)
// and delegates to the mcp-go StreamableHTTPServer.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = h.withUserContext(r)
	h.streamable.ServeHTTP(w, r)
}

// withUserContext extracts user identity and attaches UserContext to the request context.
// It reuses extractUserContext for consistency with the mcp-go context function.
func (h *Handler) withUserContext(r *http.Request) *http.Request {
	uc := h.extractUserContext(r)
	if uc.UserID != "" {
		ctx := WithUserContext(r.Context(), uc)
		return r.WithContext(ctx)
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
