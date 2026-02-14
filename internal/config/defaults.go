package config

// NewDefaultConfig creates a configuration with default values.
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
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
