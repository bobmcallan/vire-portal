package server

import (
	"io"
	"net/http"
	"time"

	"github.com/bobmcallan/vire-portal/internal/cache"
	"github.com/bobmcallan/vire-portal/internal/handlers"
)

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// OAuth discovery endpoints (root + /mcp-suffixed per MCP spec)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.app.OAuthServer.HandleAuthorizationServer)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server/mcp", s.app.OAuthServer.HandleAuthorizationServer)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.app.OAuthServer.HandleProtectedResource)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", s.app.OAuthServer.HandleProtectedResource)
	// Return 404 for unregistered well-known paths (prevents the "/" catch-all
	// from serving HTML, which breaks MCP clients probing openid-configuration).
	mux.HandleFunc("/.well-known/", handleWellKnownNotFound)

	// OAuth flow endpoints
	mux.HandleFunc("POST /register", s.app.OAuthServer.HandleRegister)
	mux.HandleFunc("GET /authorize", s.app.OAuthServer.HandleAuthorize)
	mux.HandleFunc("POST /authorize", s.app.OAuthServer.HandleAuthorize)
	mux.HandleFunc("GET /authorize/resume", s.app.OAuthServer.HandleAuthorizeResume)
	mux.HandleFunc("POST /token", s.app.OAuthServer.HandleToken)

	// UI page routes (HTML templates)
	mux.HandleFunc("GET /dashboard", s.app.DashboardHandler.ServeHTTP)
	mux.HandleFunc("GET /mcp-info", s.app.MCPPageHandler.ServeHTTP)
	mux.HandleFunc("GET /error", s.app.PageHandler.ServePage("error.html", "error"))
	mux.HandleFunc("/", s.app.PageHandler.ServePage("landing.html", "home"))

	// Static files (CSS, JS, images)
	mux.HandleFunc("/static/", s.app.PageHandler.StaticFileHandler)

	// MCP endpoint (JSON-RPC over HTTP)
	if s.app.MCPHandler != nil {
		mux.Handle("/mcp", s.app.MCPHandler)
	}
	// Dev-mode MCP endpoint with encrypted UID authentication
	// Pattern: /mcp/{encrypted_uid}
	if s.app.MCPDevHandler != nil {
		mux.Handle("/mcp/", s.app.MCPDevHandler)
	}

	// Settings page
	mux.HandleFunc("GET /settings", s.app.SettingsHandler.HandleSettings)
	mux.HandleFunc("POST /settings", s.app.SettingsHandler.HandleSaveSettings)

	// Auth routes
	mux.HandleFunc("POST /api/auth/login", s.app.AuthHandler.HandleLogin)
	mux.HandleFunc("POST /api/auth/test-login", s.app.AuthHandler.HandleTestLogin) // Dev-mode only: returns JSON for browser tests
	mux.HandleFunc("POST /api/auth/logout", s.app.AuthHandler.HandleLogout)
	mux.HandleFunc("GET /api/auth/login/google", s.app.AuthHandler.HandleGoogleLogin)
	mux.HandleFunc("GET /api/auth/login/github", s.app.AuthHandler.HandleGitHubLogin)
	mux.HandleFunc("GET /auth/callback", s.app.AuthHandler.HandleOAuthCallback)
	mux.HandleFunc("GET /api/auth/callback/google", s.app.AuthHandler.HandleGoogleCallback)
	mux.HandleFunc("GET /api/auth/callback/github", s.app.AuthHandler.HandleGitHubCallback)

	// API routes
	mux.HandleFunc("/api/health", s.app.HealthHandler.ServeHTTP)
	mux.HandleFunc("/api/server-health", s.app.ServerHealthHandler.ServeHTTP)
	mux.HandleFunc("/api/version", s.app.VersionHandler.ServeHTTP)
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown)

	// Proxy unmatched API routes to vire-server
	mux.HandleFunc("/api/", s.handleAPIProxy)

	return mux
}

// handleShutdown handles POST /api/shutdown (dev mode only).
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if !s.app.Config.IsDevMode() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"Shutdown endpoint disabled in production"}`))
		return
	}

	s.logger.Info().Msg("shutdown requested via HTTP endpoint")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Shutting down gracefully...\n"))

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	if s.shutdownChan != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			s.shutdownChan <- struct{}{}
		}()
	}
}

func (s *Server) handleAPIProxy(w http.ResponseWriter, r *http.Request) {
	apiURL := s.app.Config.API.URL
	if apiURL == "" {
		http.Error(w, `{"error":"API server not configured"}`, http.StatusServiceUnavailable)
		return
	}

	// Extract user ID for cache keying
	var userID string
	if loggedIn, claims := handlers.IsLoggedIn(r, []byte(s.app.Config.Auth.JWTSecret)); loggedIn && claims != nil {
		userID = claims.Sub
	}

	// Check cache for GET requests (key includes query string)
	if r.Method == http.MethodGet && userID != "" {
		cacheKey := cache.MakeKey(userID, r.Method, r.URL.RequestURI())
		if cached, ok := s.cache.Get(cacheKey); ok {
			for key, values := range cached.Headers {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(cached.StatusCode)
			w.Write(cached.Body)
			return
		}
	}

	targetURL := apiURL + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, `{"error":"Failed to create proxy request"}`, http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Inject X-Vire-User-ID from session cookie for authenticated API calls
	if userID != "" {
		proxyReq.Header.Set("X-Vire-User-ID", userID)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		s.logger.Warn().Err(err).Str("path", r.URL.Path).Msg("API proxy request failed")
		http.Error(w, `{"error":"API server unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	const maxCacheableBody = 5 * 1024 * 1024 // 5MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCacheableBody+1))
	if err != nil {
		s.logger.Warn().Err(err).Str("path", r.URL.Path).Msg("Failed to read proxy response body")
		http.Error(w, `{"error":"Failed to read API response"}`, http.StatusBadGateway)
		return
	}

	// Cache successful GET responses (skip oversized bodies)
	if r.Method == http.MethodGet && userID != "" && resp.StatusCode >= 200 && resp.StatusCode < 300 && len(body) <= maxCacheableBody {
		cacheKey := cache.MakeKey(userID, r.Method, r.URL.RequestURI())
		headerCopy := make(http.Header)
		for key, values := range resp.Header {
			headerCopy[key] = append([]string(nil), values...)
		}
		s.cache.Set(cacheKey, &cache.CachedResponse{
			StatusCode: resp.StatusCode,
			Headers:    headerCopy,
			Body:       body,
		})
	}

	// Invalidate cache on write operations
	if r.Method != http.MethodGet && userID != "" {
		s.cache.InvalidatePrefix(r.URL.Path)
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// handleWellKnownNotFound returns 404 for unregistered .well-known paths.
func handleWellKnownNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":"Not Found"}`))
}
