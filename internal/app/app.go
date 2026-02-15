package app

import (
	"fmt"
	"strings"

	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/handlers"
	"github.com/bobmcallan/vire-portal/internal/importer"
	"github.com/bobmcallan/vire-portal/internal/interfaces"
	"github.com/bobmcallan/vire-portal/internal/mcp"
	"github.com/bobmcallan/vire-portal/internal/models"
	"github.com/bobmcallan/vire-portal/internal/storage"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	"github.com/timshannon/badgerhold/v4"
)

// catalogAdapter converts MCP catalog tools to dashboard display tools.
func catalogAdapter(mcpHandler *mcp.Handler) func() []handlers.DashboardTool {
	return func() []handlers.DashboardTool {
		if mcpHandler == nil {
			return nil
		}
		catalog := mcpHandler.Catalog()
		tools := make([]handlers.DashboardTool, len(catalog))
		for i, ct := range catalog {
			tools[i] = handlers.DashboardTool{
				Name:        ct.Name,
				Description: ct.Description,
				Method:      ct.Method,
				Path:        ct.Path,
			}
		}
		return tools
	}
}

// App holds all application components and dependencies.
type App struct {
	Config         *config.Config
	Logger         *common.Logger
	StorageManager interfaces.StorageManager

	// HTTP handlers
	PageHandler         *handlers.PageHandler
	HealthHandler       *handlers.HealthHandler
	VersionHandler      *handlers.VersionHandler
	AuthHandler         *handlers.AuthHandler
	DashboardHandler    *handlers.DashboardHandler
	SettingsHandler     *handlers.SettingsHandler
	ServerHealthHandler *handlers.ServerHealthHandler
	MCPHandler          *mcp.Handler
}

// New initializes the application with all dependencies.
func New(cfg *config.Config, logger *common.Logger) (*App, error) {
	a := &App{
		Config: cfg,
		Logger: logger,
	}

	// Validate environment setting
	env := strings.ToLower(strings.TrimSpace(cfg.Environment))
	if cfg.IsDevMode() {
		logger.Warn().Msg("RUNNING IN DEV MODE — dev login enabled, do not use in production")
	} else if env != "prod" && env != "" {
		logger.Warn().
			Str("environment", cfg.Environment).
			Msg("unrecognized environment value, defaulting to prod behavior")
	}

	if err := a.initStorage(); err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Import users from JSON if configured
	if cfg.Import.Users {
		if err := importer.ImportUsers(a.StorageManager.DB(), logger, cfg.Import.UsersFile); err != nil {
			logger.Warn().Str("error", err.Error()).Msg("user import failed (non-fatal)")
		}
	}

	a.initHandlers()

	logger.Info().Msg("application initialization complete")

	return a, nil
}

// initStorage initializes the storage layer.
func (a *App) initStorage() error {
	storageManager, err := storage.NewStorageManager(a.Logger, a.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage manager: %w", err)
	}

	a.StorageManager = storageManager
	a.Logger.Debug().
		Str("storage", "badger").
		Str("path", a.Config.Storage.Badger.Path).
		Msg("storage layer initialized")

	return nil
}

// initHandlers initializes all HTTP handlers.
func (a *App) initHandlers() {
	a.PageHandler = handlers.NewPageHandler(a.Logger, a.Config.IsDevMode())
	a.HealthHandler = handlers.NewHealthHandler(a.Logger)
	a.VersionHandler = handlers.NewVersionHandler(a.Logger)
	a.AuthHandler = handlers.NewAuthHandler(a.Logger, a.Config.IsDevMode())

	// User lookup for models.User (used by settings and dashboard)
	userLookupModels := func(userID string) (*models.User, error) {
		store, ok := a.StorageManager.DB().(*badgerhold.Store)
		if !ok {
			return nil, fmt.Errorf("storage is not badgerhold")
		}
		var user models.User
		err := store.FindOne(&user, badgerhold.Where("Username").Eq(userID))
		if err != nil {
			return nil, err
		}
		return &user, nil
	}

	// User save closure for settings
	userSave := func(user *models.User) error {
		store, ok := a.StorageManager.DB().(*badgerhold.Store)
		if !ok {
			return fmt.Errorf("storage is not badgerhold")
		}
		return store.Upsert(user.Username, user)
	}

	// MCP user lookup (returns MCP-specific UserContext)
	userLookup := func(userID string) (*mcp.UserContext, error) {
		user, err := userLookupModels(userID)
		if err != nil {
			return nil, err
		}
		return &mcp.UserContext{UserID: user.Username, NavexaKey: user.NavexaKey}, nil
	}
	a.MCPHandler = mcp.NewHandler(a.Config, a.Logger, userLookup)

	a.ServerHealthHandler = handlers.NewServerHealthHandler(a.Logger, a.Config.API.URL)
	a.SettingsHandler = handlers.NewSettingsHandler(a.Logger, a.Config.IsDevMode(), userLookupModels, userSave)

	a.DashboardHandler = handlers.NewDashboardHandler(
		a.Logger,
		a.Config.IsDevMode(),
		a.Config.Server.Port,
		catalogAdapter(a.MCPHandler),
		userLookupModels,
	)
	a.DashboardHandler.SetConfigStatus(handlers.DashboardConfigStatus{
		Portfolios: strings.Join(a.Config.User.Portfolios, ", "),
	})

	a.Logger.Debug().Msg("HTTP handlers initialized")
}

// Close closes all application resources.
func (a *App) Close() error {
	if a.StorageManager != nil {
		if err := a.StorageManager.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
		a.Logger.Info().Msg("storage closed")
	}

	return nil
}
