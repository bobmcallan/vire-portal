package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// baseURLFromRequest derives the portal's external base URL from the incoming
// request's Host header and scheme. This allows OAuth metadata to return correct
// URLs regardless of port mapping, reverse proxy, or container networking.
func baseURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// DiscoveryHandler serves OAuth 2.0 discovery metadata endpoints.
// Deprecated: Use OAuthServer.HandleAuthorizationServer and OAuthServer.HandleProtectedResource instead.
type DiscoveryHandler struct {
	baseURL string
}

// NewDiscoveryHandler creates a new DiscoveryHandler with the given base URL.
// Deprecated: Use NewOAuthServer instead.
func NewDiscoveryHandler(baseURL string) *DiscoveryHandler {
	return &DiscoveryHandler{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/")}
}

// HandleAuthorizationServer serves GET /.well-known/oauth-authorization-server.
func (h *DiscoveryHandler) HandleAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	handleAuthorizationServer(w, r, baseURLFromRequest(r))
}

// HandleProtectedResource serves GET /.well-known/oauth-protected-resource.
func (h *DiscoveryHandler) HandleProtectedResource(w http.ResponseWriter, r *http.Request) {
	handleProtectedResource(w, r, baseURLFromRequest(r))
}

// HandleAuthorizationServer serves GET /.well-known/oauth-authorization-server.
func (s *OAuthServer) HandleAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	handleAuthorizationServer(w, r, baseURLFromRequest(r))
}

// HandleProtectedResource serves GET /.well-known/oauth-protected-resource.
func (s *OAuthServer) HandleProtectedResource(w http.ResponseWriter, r *http.Request) {
	handleProtectedResource(w, r, baseURLFromRequest(r))
}

func handleAuthorizationServer(w http.ResponseWriter, r *http.Request, baseURL string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	metadata := map[string]interface{}{
		"issuer":                                baseURL,
		"authorization_endpoint":                baseURL + "/authorize",
		"token_endpoint":                        baseURL + "/token",
		"registration_endpoint":                 baseURL + "/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "none"},
		"scopes_supported":                      []string{"openid", "portfolio:read", "portfolio:write", "tools:invoke"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(metadata)
}

func handleProtectedResource(w http.ResponseWriter, r *http.Request, baseURL string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	metadata := map[string]interface{}{
		"resource":              baseURL,
		"authorization_servers": []string{baseURL},
		"scopes_supported":      []string{"portfolio:read", "portfolio:write", "tools:invoke"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(metadata)
}
