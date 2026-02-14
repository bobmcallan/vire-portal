package server

import "net/http"

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// UI page routes (HTML templates)
	mux.HandleFunc("/", s.app.PageHandler.ServePage("landing.html", "home"))

	// Static files (CSS, JS, images)
	mux.HandleFunc("/static/", s.app.PageHandler.StaticFileHandler)

	// MCP endpoint (JSON-RPC over HTTP)
	if s.app.MCPHandler != nil {
		mux.Handle("/mcp", s.app.MCPHandler)
	}

	// API routes
	mux.HandleFunc("/api/health", s.app.HealthHandler.ServeHTTP)
	mux.HandleFunc("/api/version", s.app.VersionHandler.ServeHTTP)

	// 404 handler for unmatched API routes
	mux.HandleFunc("/api/", s.handleNotFound)

	return mux
}

// handleNotFound returns a JSON 404 for unmatched API routes.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":"Not Found","message":"The requested endpoint does not exist"}`))
}
