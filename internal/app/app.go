package app

import (
	"os"
	"strings"
	"time"

	"github.com/bobmcallan/vire-portal/internal/auth"
	"github.com/bobmcallan/vire-portal/internal/cache"
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
	PageHandler            *handlers.PageHandler
	HealthHandler          *handlers.HealthHandler
	VersionHandler         *handlers.VersionHandler
	AuthHandler            *handlers.AuthHandler
	DashboardHandler       *handlers.DashboardHandler
	StrategyHandler        *handlers.StrategyHandler
	StockHandler           *handlers.StockHandler
	CashHandler            *handlers.CashHandler
	MCPPageHandler         *handlers.MCPPageHandler
	ProfileHandler         *handlers.ProfileHandler
	ServerHealthHandler    *handlers.ServerHealthHandler
	MobileDashboardHandler *handlers.MobileDashboardHandler
	MCPHandler             *mcp.Handler
	MCPDevHandler          *mcp.DevHandler
	OAuthServer            *auth.OAuthServer
	AdminUsersHandler      *handlers.AdminUsersHandler
	AdminPromptsHandler    *handlers.AdminPromptsHandler
	AdminGeminiHandler     *handlers.AdminGeminiHandler
	WSStatusHandler        *handlers.WSStatusHandler
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
			if err := seed.ReportPortalVersion(cfg.API.URL, serviceUserID, config.GetVersion(), config.GetBuild(), logger); err != nil {
				logger.Warn().Err(err).Msg("portal version report failed")
			}
		}()
	} else if len(cfg.AdminEmails()) > 0 {
		logger.Warn().Msg("VIRE_ADMIN_USERS set but VIRE_SERVICE_KEY not configured — admin sync disabled")
	} else if cfg.Service.Key != "" {
		go func() {
			portalID := cfg.Service.PortalID
			if portalID == "" {
				portalID, _ = os.Hostname()
			}
			serviceUserID, err := seed.RegisterService(cfg.API.URL, portalID, cfg.Service.Key, logger)
			if err != nil {
				logger.Warn().Err(err).Msg("service registration failed, skipping version report")
				return
			}
			if err := seed.ReportPortalVersion(cfg.API.URL, serviceUserID, config.GetVersion(), config.GetBuild(), logger); err != nil {
				logger.Warn().Err(err).Msg("portal version report failed")
			}
		}()
	}

	logger.Info().Msg("application initialization complete")

	return a, nil
}

// initHandlers initializes all HTTP handlers.
func (a *App) initHandlers() {
	jwtSecret := []byte(a.Config.Auth.JWTSecret)

	vireClient := client.NewVireClient(a.Config.API.URL, config.GetVersion(), config.GetBuild())

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
	a.WSStatusHandler = handlers.NewWSStatusHandler(a.Logger, a.Config.API.URL)
	a.ProfileHandler = handlers.NewProfileHandler(a.Logger, a.Config.IsDevMode(), jwtSecret, userLookup, userSave)
	a.ProfileHandler.SetAPIURL(a.Config.API.URL)

	a.DashboardHandler = handlers.NewDashboardHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.DashboardHandler.SetAPIURL(a.Config.API.URL)

	a.MobileDashboardHandler = handlers.NewMobileDashboardHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.MobileDashboardHandler.SetAPIURL(a.Config.API.URL)

	a.StockHandler = handlers.NewStockHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.StockHandler.SetAPIURL(a.Config.API.URL)

	a.StrategyHandler = handlers.NewStrategyHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.StrategyHandler.SetAPIURL(a.Config.API.URL)

	a.CashHandler = handlers.NewCashHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.CashHandler.SetAPIURL(a.Config.API.URL)

	// SSR debounce cache: 2s TTL prevents duplicate API calls on rapid page refreshes.
	// This is NOT a data freshness cache — just debouncing so F5-F5-F5 doesn't
	// hammer vire-server with 15 API calls in 2 seconds.
	ssrCache := cache.New(2*time.Second, 500)
	cachedProxyGet := func(path, userID string) ([]byte, error) {
		key := cache.MakeKey(userID, "GET", path)
		if cached, ok := ssrCache.Get(key); ok {
			return cached.Body, nil
		}
		body, err := vireClient.ProxyGet(path, userID)
		if err != nil {
			return nil, err
		}
		ssrCache.Set(key, &cache.CachedResponse{StatusCode: 200, Body: body})
		return body, nil
	}

	a.PageHandler.SetProxyGetFn(cachedProxyGet)
	a.StockHandler.SetProxyGetFn(cachedProxyGet)
	a.StrategyHandler.SetProxyGetFn(cachedProxyGet)
	a.CashHandler.SetProxyGetFn(cachedProxyGet)
	a.DashboardHandler.SetProxyGetFn(cachedProxyGet)
	a.MobileDashboardHandler.SetProxyGetFn(cachedProxyGet)

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

	a.AdminGeminiHandler = handlers.NewAdminGeminiHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
	)
	a.AdminGeminiHandler.SetAPIURL(a.Config.API.URL)
	a.AdminGeminiHandler.SetProxyGetFn(cachedProxyGet)

	a.AdminPromptsHandler = handlers.NewAdminPromptsHandler(
		a.Logger,
		a.Config.IsDevMode(),
		jwtSecret,
		userLookup,
		vireClient.AdminListPrompts,
		vireClient.AdminGetPrompt,
		vireClient.AdminSetPrompt,
	)
	a.AdminPromptsHandler.SetAPIURL(a.Config.API.URL)

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
