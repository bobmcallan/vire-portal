package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bobmcallan/vire-portal/internal/app"
	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/server"
)

// configPaths is a custom flag type that allows multiple -config flags.
type configPaths []string

func (c *configPaths) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *configPaths) Set(value string) error {
	*c = append(*c, value)
	return nil
}

var (
	configFiles configPaths
	serverPort  = flag.Int("port", 0, "Server port (overrides config)")
	serverPortP = flag.Int("p", 0, "Server port (shorthand)")
	serverHost  = flag.String("host", "", "Server host (overrides config)")
	showVersion = flag.Bool("version", false, "Print version information")
)

func init() {
	flag.Var(&configFiles, "config", "Configuration file path (can be specified multiple times)")
	flag.Var(&configFiles, "c", "Configuration file path (shorthand)")
}

func main() {
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("vire-portal version %s\n", config.GetVersion())
		os.Exit(0)
	}

	// Merge port flags (shorthand takes precedence)
	finalPort := *serverPort
	if *serverPortP != 0 {
		finalPort = *serverPortP
	}

	// Auto-discover config file if not specified
	if len(configFiles) == 0 {
		for _, path := range []string{"portal.toml", "docker/portal.toml"} {
			if _, err := os.Stat(path); err == nil {
				configFiles = append(configFiles, path)
				break
			}
		}
	}

	// Load configuration
	cfg, err := config.LoadFromFiles(configFiles...)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Apply CLI flag overrides (highest priority)
	config.ApplyFlagOverrides(cfg, finalPort, *serverHost)

	// Initialize logger
	logger := setupLogger(cfg)

	logger.Info("configuration loaded",
		"port", cfg.Server.Port,
		"host", cfg.Server.Host,
		"config_files", fmt.Sprintf("%v", configFiles),
	)

	// Initialize application
	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	// Create HTTP server
	srv := server.New(application)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Give goroutine a moment to start
	time.Sleep(100 * time.Millisecond)

	logger.Info("server ready",
		"url", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port),
	)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}

	if err := application.Close(); err != nil {
		logger.Error("application shutdown failed", "error", err)
	}

	logger.Info("server stopped")
}

// setupLogger creates a structured logger based on config.
func setupLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
