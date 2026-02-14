package config

// NewDefaultConfig creates a configuration with default values.
func NewDefaultConfig() *Config {
	return &Config{
		Environment: "prod",
		Server: ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		API: APIConfig{
			URL: "http://localhost:4242",
		},
		User: UserConfig{
			Portfolios:      []string{},
			DisplayCurrency: "",
		},
		Import: ImportConfig{
			Users:     false,
			UsersFile: "data/users.json",
		},
		Storage: StorageConfig{
			Badger: BadgerConfig{
				Path: "./data/vire",
			},
		},
		Logging: LoggingConfig{
			Level:    "info",
			Format:   "text",
			Outputs:  []string{"console", "file"},
			FilePath: "logs/vire-portal.log",
		},
	}
}
