package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Server.Port != 4241 {
		t.Errorf("expected default port 4241, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.Server.Host)
	}
	if cfg.Storage.Badger.Path != "./data/vire" {
		t.Errorf("expected default badger path ./data/vire, got %s", cfg.Storage.Badger.Path)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected default log format text, got %s", cfg.Logging.Format)
	}
}

func TestLoadFromFiles_NoFiles(t *testing.T) {
	cfg, err := LoadFromFiles()
	if err != nil {
		t.Fatalf("LoadFromFiles with no files should not error: %v", err)
	}
	if cfg.Server.Port != 4241 {
		t.Errorf("expected default port 4241, got %d", cfg.Server.Port)
	}
}

func TestLoadFromFiles_ValidTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "test.toml")

	content := `
[server]
port = 9090
host = "0.0.0.0"

[storage.badger]
path = "/tmp/test-db"

[logging]
level = "debug"
format = "json"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Storage.Badger.Path != "/tmp/test-db" {
		t.Errorf("expected badger path /tmp/test-db, got %s", cfg.Storage.Badger.Path)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected log format json, got %s", cfg.Logging.Format)
	}
}

func TestLoadFromFiles_PartialOverride(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "partial.toml")

	// Only override port; everything else should stay default
	content := `
[server]
port = 3000
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}
	// Host should remain the default
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.Server.Host)
	}
}

func TestLoadFromFiles_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	base := filepath.Join(dir, "base.toml")
	baseContent := `
[server]
port = 3000
host = "base-host"
`
	if err := os.WriteFile(base, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	override := filepath.Join(dir, "override.toml")
	overrideContent := `
[server]
port = 4000
`
	if err := os.WriteFile(override, []byte(overrideContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(base, override)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	// Port should be overridden by the second file
	if cfg.Server.Port != 4000 {
		t.Errorf("expected port 4000 from override, got %d", cfg.Server.Port)
	}
	// Host should come from the base file
	if cfg.Server.Host != "base-host" {
		t.Errorf("expected host base-host from base file, got %s", cfg.Server.Host)
	}
}

func TestLoadFromFiles_MissingFile(t *testing.T) {
	_, err := LoadFromFiles("/nonexistent/path.toml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadFromFiles_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "invalid.toml")

	if err := os.WriteFile(tomlPath, []byte("this is not valid {{toml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFiles(tomlPath)
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_SERVER_PORT", "9999")
	t.Setenv("VIRE_SERVER_HOST", "env-host")
	t.Setenv("VIRE_BADGER_PATH", "/env/path")
	t.Setenv("VIRE_LOG_LEVEL", "error")
	t.Setenv("VIRE_LOG_FORMAT", "json")

	applyEnvOverrides(cfg)

	if cfg.Server.Port != 9999 {
		t.Errorf("expected env port 9999, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "env-host" {
		t.Errorf("expected env host env-host, got %s", cfg.Server.Host)
	}
	if cfg.Storage.Badger.Path != "/env/path" {
		t.Errorf("expected env badger path /env/path, got %s", cfg.Storage.Badger.Path)
	}
	if cfg.Logging.Level != "error" {
		t.Errorf("expected env log level error, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected env log format json, got %s", cfg.Logging.Format)
	}
}

func TestApplyEnvOverrides_InvalidPort(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_SERVER_PORT", "not-a-number")

	applyEnvOverrides(cfg)

	// Port should remain default when env var is not a valid integer
	if cfg.Server.Port != 4241 {
		t.Errorf("expected default port 4241 for invalid env, got %d", cfg.Server.Port)
	}
}

func TestApplyFlagOverrides(t *testing.T) {
	cfg := NewDefaultConfig()

	ApplyFlagOverrides(cfg, 7777, "flag-host")

	if cfg.Server.Port != 7777 {
		t.Errorf("expected flag port 7777, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "flag-host" {
		t.Errorf("expected flag host flag-host, got %s", cfg.Server.Host)
	}
}

func TestApplyFlagOverrides_ZeroPortNoOverride(t *testing.T) {
	cfg := NewDefaultConfig()

	ApplyFlagOverrides(cfg, 0, "")

	// No override when port is 0 and host is empty
	if cfg.Server.Port != 4241 {
		t.Errorf("expected default port 4241, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.Server.Host)
	}
}

// --- MCP Config Tests ---

func TestNewDefaultConfig_APIDefaults(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.API.URL != "http://localhost:4242" {
		t.Errorf("expected default API URL http://localhost:4242, got %s", cfg.API.URL)
	}
}

func TestNewDefaultConfig_UserDefaults(t *testing.T) {
	cfg := NewDefaultConfig()

	if len(cfg.User.Portfolios) != 0 {
		t.Errorf("expected empty default portfolios, got %v", cfg.User.Portfolios)
	}
	if cfg.User.DisplayCurrency != "" {
		t.Errorf("expected empty default display currency, got %s", cfg.User.DisplayCurrency)
	}
}

func TestNewDefaultConfig_KeysDefaults(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Keys.EODHD != "" {
		t.Errorf("expected empty default EODHD key, got %s", cfg.Keys.EODHD)
	}
	if cfg.Keys.Navexa != "" {
		t.Errorf("expected empty default Navexa key, got %s", cfg.Keys.Navexa)
	}
	if cfg.Keys.Gemini != "" {
		t.Errorf("expected empty default Gemini key, got %s", cfg.Keys.Gemini)
	}
}

func TestLoadFromFiles_MCPSections(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "mcp.toml")

	content := `
[api]
url = "http://vire-server:4242"

[user]
portfolios = ["SMSF", "Personal"]
display_currency = "AUD"

[keys]
eodhd = "test-eodhd"
navexa = "test-navexa"
gemini = "test-gemini"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.API.URL != "http://vire-server:4242" {
		t.Errorf("expected API URL http://vire-server:4242, got %s", cfg.API.URL)
	}
	if len(cfg.User.Portfolios) != 2 || cfg.User.Portfolios[0] != "SMSF" || cfg.User.Portfolios[1] != "Personal" {
		t.Errorf("expected portfolios [SMSF Personal], got %v", cfg.User.Portfolios)
	}
	if cfg.User.DisplayCurrency != "AUD" {
		t.Errorf("expected display currency AUD, got %s", cfg.User.DisplayCurrency)
	}
	if cfg.Keys.EODHD != "test-eodhd" {
		t.Errorf("expected EODHD key test-eodhd, got %s", cfg.Keys.EODHD)
	}
	if cfg.Keys.Navexa != "test-navexa" {
		t.Errorf("expected Navexa key test-navexa, got %s", cfg.Keys.Navexa)
	}
	if cfg.Keys.Gemini != "test-gemini" {
		t.Errorf("expected Gemini key test-gemini, got %s", cfg.Keys.Gemini)
	}
}

func TestApplyEnvOverrides_APIURL(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_API_URL", "http://custom-server:9999")

	applyEnvOverrides(cfg)

	if cfg.API.URL != "http://custom-server:9999" {
		t.Errorf("expected API URL http://custom-server:9999, got %s", cfg.API.URL)
	}
}

func TestApplyEnvOverrides_DefaultPortfolio(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_DEFAULT_PORTFOLIO", "SMSF")

	applyEnvOverrides(cfg)

	if len(cfg.User.Portfolios) != 1 || cfg.User.Portfolios[0] != "SMSF" {
		t.Errorf("expected portfolios [SMSF], got %v", cfg.User.Portfolios)
	}
}

func TestApplyEnvOverrides_DisplayCurrency(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_DISPLAY_CURRENCY", "USD")

	applyEnvOverrides(cfg)

	if cfg.User.DisplayCurrency != "USD" {
		t.Errorf("expected display currency USD, got %s", cfg.User.DisplayCurrency)
	}
}

func TestApplyEnvOverrides_APIKeys(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Setenv("EODHD_API_KEY", "env-eodhd")
	t.Setenv("NAVEXA_API_KEY", "env-navexa")
	t.Setenv("GEMINI_API_KEY", "env-gemini")

	applyEnvOverrides(cfg)

	if cfg.Keys.EODHD != "env-eodhd" {
		t.Errorf("expected EODHD key env-eodhd, got %s", cfg.Keys.EODHD)
	}
	if cfg.Keys.Navexa != "env-navexa" {
		t.Errorf("expected Navexa key env-navexa, got %s", cfg.Keys.Navexa)
	}
	if cfg.Keys.Gemini != "env-gemini" {
		t.Errorf("expected Gemini key env-gemini, got %s", cfg.Keys.Gemini)
	}
}

func TestLoadFromFiles_MCPSectionsDefaultsPreserved(t *testing.T) {
	// When TOML only sets [api], [user] and [keys] should keep defaults
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "partial-mcp.toml")

	content := `
[api]
url = "http://vire-server:4242"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.API.URL != "http://vire-server:4242" {
		t.Errorf("expected API URL from file, got %s", cfg.API.URL)
	}
	// User and Keys should remain defaults
	if len(cfg.User.Portfolios) != 0 {
		t.Errorf("expected empty portfolios, got %v", cfg.User.Portfolios)
	}
	if cfg.Keys.EODHD != "" {
		t.Errorf("expected empty EODHD key, got %s", cfg.Keys.EODHD)
	}
}

func TestEnvOverridesFileConfig(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "test.toml")

	content := `
[server]
port = 3000
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VIRE_SERVER_PORT", "5555")

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	// Env should override file value
	if cfg.Server.Port != 5555 {
		t.Errorf("expected env override port 5555, got %d", cfg.Server.Port)
	}
}
