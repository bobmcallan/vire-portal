package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application configuration.
type Config struct {
	Server  ServerConfig  `toml:"server"`
	API     APIConfig     `toml:"api"`
	User    UserConfig    `toml:"user"`
	Keys    KeysConfig    `toml:"keys"`
	Storage StorageConfig `toml:"storage"`
	Logging LoggingConfig `toml:"logging"`
}

// APIConfig contains vire-server API connection settings.
type APIConfig struct {
	URL string `toml:"url"`
}

// UserConfig contains per-user settings injected as X-Vire-* headers.
type UserConfig struct {
	Portfolios      []string `toml:"portfolios"`
	DisplayCurrency string   `toml:"display_currency"`
}

// KeysConfig contains API keys for external services.
type KeysConfig struct {
	EODHD  string `toml:"eodhd"`
	Navexa string `toml:"navexa"`
	Gemini string `toml:"gemini"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

// StorageConfig contains storage layer settings.
type StorageConfig struct {
	Badger BadgerConfig `toml:"badger"`
}

// BadgerConfig contains BadgerDB-specific settings.
type BadgerConfig struct {
	Path string `toml:"path"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

// LoadFromFile loads configuration with priority: defaults -> file -> env.
func LoadFromFile(path string) (*Config, error) {
	if path == "" {
		return LoadFromFiles()
	}
	return LoadFromFiles(path)
}

// LoadFromFiles loads configuration from multiple files with priority:
// defaults -> file1 -> file2 -> ... -> env.
// Later files override earlier files.
func LoadFromFiles(paths ...string) (*Config, error) {
	config := NewDefaultConfig()

	for i, path := range paths {
		if path == "" {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		err = toml.Unmarshal(data, config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file %s (file %d of %d): %w", path, i+1, len(paths), err)
		}
	}

	applyEnvOverrides(config)

	return config, nil
}

// applyEnvOverrides applies VIRE_* environment variable overrides to config.
func applyEnvOverrides(config *Config) {
	if port := os.Getenv("VIRE_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	if host := os.Getenv("VIRE_SERVER_HOST"); host != "" {
		config.Server.Host = host
	}
	if badgerPath := os.Getenv("VIRE_BADGER_PATH"); badgerPath != "" {
		config.Storage.Badger.Path = badgerPath
	}
	if level := os.Getenv("VIRE_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("VIRE_LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}

	// MCP / API overrides
	if apiURL := os.Getenv("VIRE_API_URL"); apiURL != "" {
		config.API.URL = apiURL
	}
	if portfolio := os.Getenv("VIRE_DEFAULT_PORTFOLIO"); portfolio != "" {
		config.User.Portfolios = []string{portfolio}
	}
	if currency := os.Getenv("VIRE_DISPLAY_CURRENCY"); currency != "" {
		config.User.DisplayCurrency = currency
	}

	// API key overrides (match vire-mcp convention)
	if key := os.Getenv("EODHD_API_KEY"); key != "" {
		config.Keys.EODHD = key
	}
	if key := os.Getenv("NAVEXA_API_KEY"); key != "" {
		config.Keys.Navexa = key
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		config.Keys.Gemini = key
	}

}

// ApplyFlagOverrides applies command-line flag overrides to config.
func ApplyFlagOverrides(config *Config, port int, host string) {
	if port > 0 {
		config.Server.Port = port
	}
	if host != "" {
		config.Server.Host = host
	}
}
