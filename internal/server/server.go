package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bobmcallan/vire-portal/internal/app"
)

// Server manages the HTTP server and routes.
type Server struct {
	app    *app.App
	router *http.ServeMux
	server *http.Server
	logger *slog.Logger
}

// New creates a new HTTP server with the given app.
func New(application *app.App) *Server {
	s := &Server{
		app:    application,
		logger: application.Logger,
	}

	s.router = s.setupRoutes()

	addr := fmt.Sprintf("%s:%d", application.Config.Server.Host, application.Config.Server.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.withMiddleware(s.router),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // 5 min: MCP tools (generate_report, etc.) can take minutes
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("HTTP server starting",
		"address", s.server.Addr,
		"url", fmt.Sprintf("http://%s", s.server.Addr),
	)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// Handler returns the HTTP handler for testing.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}
