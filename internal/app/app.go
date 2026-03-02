package app

import (
	"os"
	"strings"

	"github.com/bobmcallan/vire-portal/internal/auth"
	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/handlers"
	"github.com/bobmcallan/vire-portal/internal/mcp"
	"github.com/bobmcallan/vire-portal/internal/seed"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// catalogAdapter converts MCP catalog tools to MCP page display tools.
func catalogAdapter(mcpHandler *mcp.Handler) func() []handlers.MCPPageTool {
	return func() []handlers.MCPPageTool {
		if mcpHandler == nil {
			return nil
		}
		catalog := mcpHandler.Catalog()
		tools := make([]handlers.MCPPageTool, len(catalog))
		for i, ct := range catalog {
			tools[i] = handlers.MCPPageTool{
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
	StrategyHandler     *handlers.StrategyHandler
	CapitalHandler      *handlers.CapitalHandler
	MCPPageHandler      *handlers.MCPPageHandler
	ProfileHandler      *handlers.ProfileHandler
	ServerHealthHandler *handlers.ServerHealthHandler
	MCPHandler          *mcp.Handler
	MCPDevHandler       *mcp.DevHandler
	OAuthServer         *auth.OAuthServer
	AdminUsersHandler   *handlers.AdminUsersHandler
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

	if cfg.Service.Key != "" && len(cfg.AdminEmails()) > 0 {
		go func() {
			portalID := cfg.Service.PortalID
			if portalID == "" {
				portalID, _ = os.Hostname()
			}
			serviceUserID, err := seed.RegisterService(cfg.API.URL, portalID, cfg.Service.Key, logger)
			if err != nil {
				logger.Warn().Err(err).Msg("service registration failed, skipping admin sync")
				return
			}
			seed.SyncAdmins(cfg.API.URL, cfg.AdminEmails(), serviceUserID, logger)
		}()
	} else if len(cfg.AdminEmails()) > 0 {
		logger.Warn().Msg("VIRE_ADMIN_USERS set but VIRE_SERVICE_KEY not configured — admin sync disabled")
	}

	logger.Info().Msg("application initialization complete")

	return a, nil
}

// initHandlers initializes all HTTP handlers.
func (a *App) initHandlers() {
	jwtSecret := []byte(a.Config.Auth.JWTSecret)

	vireClient := client.NewVireClient(a.Config.API.URL)

	// User lookup via vire-server API (used by profile, dashboard, and page handler)
	userLookup := func(userID string) (*client.UserProfile, error) {
		return vireClient.GetUser(userID)
	}

	// User save via vire-server API (used by profile)
	userSave := func(userID string, fields map[string]string) error {
		_, err := vireClient.UpdateUser(userID, fields)
		return err
	}

	a.PageHandler = handlers.NewPageHandler(a.Logger, a.Config.IsDevMode(), jwtSecret, userLookup)
	a.PageHandler.SetAPIURL(a.Config.API.URL)
	a.HealthHandler = handlers.NewHealthHandler(a.Logger)
	a.VersionHandler = handlers.NewVersionHandler(a.Logger)
	a.VersionHandler.SetAPIURL(a.Config.API.URL)
	a.AuthHandler = handlers.NewAuthHandler(a.Logger, a.Config.IsDevMode(), a.Config.API.URL, a.Config.Auth.CallbackURL, jwtSecret)

	a.MCPHandler = mcp.NewHandler(a.Config, a.Logger)
	a.MCPDevHandler = mcp.NewDevHandler(
		a.MCPHandler,
		jwtSecret,
		a.Config.IsDevMode(),
		a.Config.BaseURL(),
		a.Logger,
	)

	a.ServerHealthHandler = handlers.NewServerHealthHandler(a.Logger, a.Config.API.URL)
	a.ProfileHandler = handlers.NewProfileHandler(a.Logger, a.Config.IsDevMode(), jwtSecret, userLookup, userSave)
	a.ProfileHandler.SetAPIURL(a.Config.API.URL)

	a.DashboardHandler = handlers.NewDashboardHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.DashboardHandler.SetAPIURL(a.Config.API.URL)

	a.StrategyHandler = handlers.NewStrategyHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.StrategyHandler.SetAPIURL(a.Config.API.URL)

	a.CapitalHandler = handlers.NewCapitalHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.CapitalHandler.SetAPIURL(a.Config.API.URL)

	a.MCPPageHandler = handlers.NewMCPPageHandler(
		a.Logger,
		a.Config.IsDevMode(),
		a.Config.Server.Port,
		jwtSecret,
		catalogAdapter(a.MCPHandler),
		userLookup,
	)
	a.MCPPageHandler.SetAPIURL(a.Config.API.URL)
	a.MCPPageHandler.SetBaseURL(a.Config.BaseURL())

	if a.MCPDevHandler != nil {
		a.MCPPageHandler.SetDevMCPEndpointFn(a.MCPDevHandler.GenerateEndpoint)
		a.ProfileHandler.SetDevMCPEndpointFn(a.MCPDevHandler.GenerateEndpoint)
	}

	// Construct service user ID for admin API calls
	portalID := a.Config.Service.PortalID
	if portalID == "" {
		portalID, _ = os.Hostname()
	}
	serviceUserID := ""
	if a.Config.Service.Key != "" {
		serviceUserID = "service:" + portalID
	}

	a.AdminUsersHandler = handlers.NewAdminUsersHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
		vireClient.AdminListUsers,
		serviceUserID,
	)
	a.AdminUsersHandler.SetAPIURL(a.Config.API.URL)

	a.OAuthServer = auth.NewOAuthServer(a.Config.BaseURL(), a.Config.API.URL, jwtSecret, a.Logger)
	a.AuthHandler.SetOAuthServer(a.OAuthServer)

	a.Logger.Debug().Msg("HTTP handlers initialized")
}

// Close closes all application resources.
func (a *App) Close() error {
	if a.MCPHandler != nil {
		a.MCPHandler.Close()
	}
	return nil
}
