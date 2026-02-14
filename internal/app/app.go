package app

import (
	"fmt"
	"log/slog"

	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/handlers"
	"github.com/bobmcallan/vire-portal/internal/interfaces"
	"github.com/bobmcallan/vire-portal/internal/mcp"
	"github.com/bobmcallan/vire-portal/internal/storage"
)

// App holds all application components and dependencies.
type App struct {
	Config         *config.Config
	Logger         *slog.Logger
	StorageManager interfaces.StorageManager

	// HTTP handlers
	PageHandler    *handlers.PageHandler
	HealthHandler  *handlers.HealthHandler
	VersionHandler *handlers.VersionHandler
	MCPHandler     *mcp.Handler
}

// New initializes the application with all dependencies.
func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	a := &App{
		Config: cfg,
		Logger: logger,
	}

	if err := a.initStorage(); err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	a.initHandlers()

	logger.Info("application initialization complete")

	return a, nil
}

// initStorage initializes the storage layer.
func (a *App) initStorage() error {
	storageManager, err := storage.NewStorageManager(a.Logger, a.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage manager: %w", err)
	}

	a.StorageManager = storageManager
	a.Logger.Debug("storage layer initialized",
		"storage", "badger",
		"path", a.Config.Storage.Badger.Path,
	)

	return nil
}

// initHandlers initializes all HTTP handlers.
func (a *App) initHandlers() {
	a.PageHandler = handlers.NewPageHandler(a.Logger)
	a.HealthHandler = handlers.NewHealthHandler(a.Logger)
	a.VersionHandler = handlers.NewVersionHandler(a.Logger)
	a.MCPHandler = mcp.NewHandler(a.Config, a.Logger)

	a.Logger.Debug("HTTP handlers initialized")
}

// Close closes all application resources.
func (a *App) Close() error {
	if a.StorageManager != nil {
		if err := a.StorageManager.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
		a.Logger.Info("storage closed")
	}

	return nil
}
