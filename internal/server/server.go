package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bobmcallan/vire-portal/internal/app"
	"github.com/bobmcallan/vire-portal/internal/cache"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// Server manages the HTTP server and routes.
type Server struct {
	app          *app.App
	router       *http.ServeMux
	server       *http.Server
	logger       *common.Logger
	cache        *cache.ResponseCache
	shutdownChan chan struct{}
}

// SetShutdownChannel sets the channel that will be signaled when HTTP shutdown is requested.
func (s *Server) SetShutdownChannel(ch chan struct{}) {
	s.shutdownChan = ch
}

// New creates a new HTTP server with the given app.
func New(application *app.App) *Server {
	s := &Server{
		app:    application,
		logger: application.Logger,
		cache:  cache.New(30*time.Second, 1000),
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
	s.logger.Info().
		Str("address", s.server.Addr).
		Str("url", fmt.Sprintf("http://%s", s.server.Addr)).
		Msg("HTTP server starting")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info().Msg("shutting down HTTP server")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info().Msg("HTTP server stopped")
	return nil
}

// Handler returns the HTTP handler for testing.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}
