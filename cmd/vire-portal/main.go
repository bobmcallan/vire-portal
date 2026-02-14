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
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
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
		for _, path := range []string{"vire-portal.toml", "docker/vire-portal.toml"} {
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

	logger.Info().
		Int("port", cfg.Server.Port).
		Str("host", cfg.Server.Host).
		Str("environment", cfg.Environment).
		Str("config_files", fmt.Sprintf("%v", configFiles)).
		Msg("configuration loaded")

	// Initialize application
	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error().Str("error", err.Error()).Msg("failed to initialize application")
		os.Exit(1)
	}

	// Create HTTP server
	srv := server.New(application)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			logger.Error().Str("error", err.Error()).Msg("server failed to start")
			os.Exit(1)
		}
	}()

	// Give goroutine a moment to start
	time.Sleep(100 * time.Millisecond)

	logger.Info().
		Str("url", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)).
		Msg("server ready")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Str("error", err.Error()).Msg("server shutdown failed")
	}

	if err := application.Close(); err != nil {
		logger.Error().Str("error", err.Error()).Msg("application shutdown failed")
	}

	logger.Info().Msg("server stopped")
}

// setupLogger creates an arbor logger based on config.
func setupLogger(cfg *config.Config) *common.Logger {
	arborCfg := common.LoggingConfig{
		Level:      cfg.Logging.Level,
		Outputs:    cfg.Logging.Outputs,
		FilePath:   cfg.Logging.FilePath,
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
	}
	return common.NewLoggerFromConfig(arborCfg)
}
