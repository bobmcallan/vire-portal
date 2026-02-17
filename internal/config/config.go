package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// AuthConfig contains authentication settings.
type AuthConfig struct {
	JWTSecret   string `toml:"jwt_secret"`
	CallbackURL string `toml:"callback_url"`
	PortalURL   string `toml:"portal_url"`
}

// Config represents the application configuration.
type Config struct {
	Environment string        `toml:"environment"`
	Server      ServerConfig  `toml:"server"`
	API         APIConfig     `toml:"api"`
	Portal      PortalConfig  `toml:"portal"`
	Auth        AuthConfig    `toml:"auth"`
	User        UserConfig    `toml:"user"`
	Logging     LoggingConfig `toml:"logging"`
}

// IsDevMode returns true when the environment is set to "dev" or "development" (case-insensitive, trimmed).
// The environment value is normalized at load time: "development" → "dev", "production" → "prod".
func (c *Config) IsDevMode() bool {
	return strings.ToLower(strings.TrimSpace(c.Environment)) == "dev"
}

// normalizeEnvironment maps environment aliases to their canonical short forms.
// "development" → "dev", "production" → "prod". All other values pass through unchanged.
func normalizeEnvironment(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "development":
		return "dev"
	case "production":
		return "prod"
	default:
		return env
	}
}

// BaseURL returns the portal's external base URL.
// Uses Auth.PortalURL if set, otherwise builds from server host and port.
func (c *Config) BaseURL() string {
	if c.Auth.PortalURL != "" {
		return strings.TrimRight(c.Auth.PortalURL, "/")
	}
	host := c.Server.Host
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, c.Server.Port)
}

// APIConfig contains vire-server API connection settings.
type APIConfig struct {
	URL string `toml:"url"`
}

// PortalConfig contains vire-portal connection settings.
// Used by vire-mcp to know which portal instance to connect to.
type PortalConfig struct {
	URL string `toml:"url"`
}

// UserConfig contains per-user settings injected as X-Vire-* headers.
type UserConfig struct {
	Portfolios      []string `toml:"portfolios"`
	DisplayCurrency string   `toml:"display_currency"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level      string   `toml:"level"`
	Format     string   `toml:"format"`
	Outputs    []string `toml:"outputs"`
	FilePath   string   `toml:"file_path"`
	MaxSizeMB  int      `toml:"max_size_mb"`
	MaxBackups int      `toml:"max_backups"`
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
	config.Environment = normalizeEnvironment(config.Environment)

	return config, nil
}

// applyEnvOverrides applies VIRE_* environment variable overrides to config.
func applyEnvOverrides(config *Config) {
	if env := os.Getenv("VIRE_ENV"); env != "" {
		config.Environment = env
	}
	if port := os.Getenv("VIRE_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	if host := os.Getenv("VIRE_SERVER_HOST"); host != "" {
		config.Server.Host = host
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

	// Auth overrides
	if jwtSecret := os.Getenv("VIRE_AUTH_JWT_SECRET"); jwtSecret != "" {
		config.Auth.JWTSecret = jwtSecret
	}
	if callbackURL := os.Getenv("VIRE_AUTH_CALLBACK_URL"); callbackURL != "" {
		config.Auth.CallbackURL = callbackURL
	}
	if portalURL := os.Getenv("VIRE_PORTAL_URL"); portalURL != "" {
		config.Auth.PortalURL = portalURL
		config.Portal.URL = portalURL
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
