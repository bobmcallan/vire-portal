package app

import (
	"strings"

	"github.com/bobmcallan/vire-portal/internal/auth"
	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/handlers"
	"github.com/bobmcallan/vire-portal/internal/mcp"
	"github.com/bobmcallan/vire-portal/internal/seed"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
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
	Config *config.Config
	Logger *common.Logger

	// HTTP handlers
	PageHandler         *handlers.PageHandler
	HealthHandler       *handlers.HealthHandler
	VersionHandler      *handlers.VersionHandler
	AuthHandler         *handlers.AuthHandler
	DashboardHandler    *handlers.DashboardHandler
	SettingsHandler     *handlers.SettingsHandler
	ServerHealthHandler *handlers.ServerHealthHandler
	MCPHandler          *mcp.Handler
	MCPDevHandler       *mcp.DevHandler
	OAuthServer         *auth.OAuthServer
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

	a.initHandlers()

	if cfg.IsDevMode() {
		go seed.DevUsers(cfg.API.URL, logger)
	}

	logger.Info().Msg("application initialization complete")

	return a, nil
}

// initHandlers initializes all HTTP handlers.
func (a *App) initHandlers() {
	jwtSecret := []byte(a.Config.Auth.JWTSecret)

	a.PageHandler = handlers.NewPageHandler(a.Logger, a.Config.IsDevMode(), jwtSecret)
	a.HealthHandler = handlers.NewHealthHandler(a.Logger)
	a.VersionHandler = handlers.NewVersionHandler(a.Logger)
	a.AuthHandler = handlers.NewAuthHandler(a.Logger, a.Config.IsDevMode(), a.Config.API.URL, a.Config.Auth.CallbackURL, jwtSecret)

	vireClient := client.NewVireClient(a.Config.API.URL)

	// User lookup via vire-server API (used by settings and dashboard)
	userLookup := func(userID string) (*client.UserProfile, error) {
		return vireClient.GetUser(userID)
	}

	// User save via vire-server API (used by settings)
	userSave := func(userID string, fields map[string]string) error {
		_, err := vireClient.UpdateUser(userID, fields)
		return err
	}

	a.MCPHandler = mcp.NewHandler(a.Config, a.Logger)
	a.MCPDevHandler = mcp.NewDevHandler(
		a.MCPHandler,
		jwtSecret,
		a.Config.IsDevMode(),
		a.Config.BaseURL(),
		a.Logger,
	)

	a.ServerHealthHandler = handlers.NewServerHealthHandler(a.Logger, a.Config.API.URL)
	a.SettingsHandler = handlers.NewSettingsHandler(a.Logger, a.Config.IsDevMode(), jwtSecret, userLookup, userSave)

	a.DashboardHandler = handlers.NewDashboardHandler(
		a.Logger,
		a.Config.IsDevMode(),
		a.Config.Server.Port,
		jwtSecret,
		catalogAdapter(a.MCPHandler),
		userLookup,
	)
	a.DashboardHandler.SetConfigStatus(handlers.DashboardConfigStatus{
		Portfolios: strings.Join(a.Config.User.Portfolios, ", "),
	})
	if a.MCPDevHandler != nil {
		a.DashboardHandler.SetDevMCPEndpointFn(a.MCPDevHandler.GenerateEndpoint)
		a.SettingsHandler.SetDevMCPEndpointFn(a.MCPDevHandler.GenerateEndpoint)
	}

	a.OAuthServer = auth.NewOAuthServer(a.Config.BaseURL(), jwtSecret, a.Logger)
	a.AuthHandler.SetOAuthServer(a.OAuthServer)

	a.Logger.Debug().Msg("HTTP handlers initialized")
}

// Close closes all application resources.
func (a *App) Close() error {
	return nil
}
