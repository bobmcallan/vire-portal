package server

import (
	"net/http"
	"time"
)

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// UI page routes (HTML templates)
	mux.HandleFunc("GET /dashboard", s.app.DashboardHandler.ServeHTTP)
	mux.HandleFunc("/", s.app.PageHandler.ServePage("landing.html", "home"))

	// Static files (CSS, JS, images)
	mux.HandleFunc("/static/", s.app.PageHandler.StaticFileHandler)

	// MCP endpoint (JSON-RPC over HTTP)
	if s.app.MCPHandler != nil {
		mux.Handle("/mcp", s.app.MCPHandler)
	}

	// Settings page
	mux.HandleFunc("GET /settings", s.app.SettingsHandler.HandleSettings)
	mux.HandleFunc("POST /settings", s.app.SettingsHandler.HandleSaveSettings)

	// Auth routes
	mux.HandleFunc("POST /api/auth/dev", s.app.AuthHandler.HandleDevLogin)
	mux.HandleFunc("POST /api/auth/logout", s.app.AuthHandler.HandleLogout)

	// API routes
	mux.HandleFunc("/api/health", s.app.HealthHandler.ServeHTTP)
	mux.HandleFunc("/api/version", s.app.VersionHandler.ServeHTTP)
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown)

	// 404 handler for unmatched API routes
	mux.HandleFunc("/api/", s.handleNotFound)

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

// handleNotFound returns a JSON 404 for unmatched API routes.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":"Not Found","message":"The requested endpoint does not exist"}`))
}
