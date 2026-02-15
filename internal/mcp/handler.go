package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	streamable   *mcpserver.StreamableHTTPServer
	logger       *common.Logger
	catalog      []CatalogTool
	userLookupFn func(userID string) (*UserContext, error)
}

// catalogRetryAttempts is the number of times to retry fetching the catalog.
const catalogRetryAttempts = 3

// catalogRetryDelay is the delay between retry attempts.
const catalogRetryDelay = 2 * time.Second

// NewHandler creates a new MCP handler with dynamic tool registration from vire-server.
func NewHandler(cfg *config.Config, logger *common.Logger, userLookupFn func(userID string) (*UserContext, error)) *Handler {
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

	streamable := mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithStateLess(true),
	)

	logger.Info().
		Int("tools", toolCount).
		Str("api_url", cfg.API.URL).
		Msg("MCP handler initialized")

	return &Handler{
		streamable:   streamable,
		logger:       logger,
		catalog:      validated,
		userLookupFn: userLookupFn,
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
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = h.withUserContext(r)
	h.streamable.ServeHTTP(w, r)
}

// withUserContext extracts the vire_session cookie, decodes the JWT sub claim,
// looks up the user, and attaches UserContext to the request context.
// If anything fails, the original request is returned unchanged.
func (h *Handler) withUserContext(r *http.Request) *http.Request {
	if h.userLookupFn == nil {
		return r
	}

	cookie, err := r.Cookie("vire_session")
	if err != nil || cookie.Value == "" {
		return r
	}

	sub := extractJWTSub(cookie.Value)
	if sub == "" {
		return r
	}

	uc, err := h.userLookupFn(sub)
	if err != nil || uc == nil {
		return r
	}

	ctx := WithUserContext(r.Context(), *uc)
	return r.WithContext(ctx)
}

// extractJWTSub base64url-decodes the JWT payload (middle segment)
// and returns the "sub" claim. Returns empty string on any failure.
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
