package config

// NewDefaultConfig creates a configuration with default values.
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 4241,
			Host: "localhost",
		},
		API: APIConfig{
			URL: "http://localhost:4242",
		},
		User: UserConfig{
			Portfolios:      []string{},
			DisplayCurrency: "",
		},
		Keys: KeysConfig{},
		Storage: StorageConfig{
			Badger: BadgerConfig{
				Path: "./data/vire",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}
