package config

// NewDefaultConfig creates a configuration with default values.
func NewDefaultConfig() *Config {
	return &Config{
		Environment: "prod",
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		API: APIConfig{
			URL: "http://localhost:8080",
		},
		Portal: PortalConfig{
			URL: "http://localhost:8080",
		},
		Auth: AuthConfig{
			JWTSecret:   "",
			CallbackURL: "http://localhost:8080/auth/callback",
			PortalURL:   "",
		},
		User: UserConfig{
			Portfolios:      []string{},
			DisplayCurrency: "",
		},
		Logging: LoggingConfig{
			Level:    "info",
			Format:   "text",
			Outputs:  []string{"console", "file"},
			FilePath: "logs/vire-portal.log",
		},
	}
}
