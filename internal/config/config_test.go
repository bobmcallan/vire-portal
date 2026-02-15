package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.Server.Host)
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
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadFromFiles_ValidTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "test.toml")

	content := `
[server]
port = 9090
host = "0.0.0.0"

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
	t.Setenv("VIRE_LOG_LEVEL", "error")
	t.Setenv("VIRE_LOG_FORMAT", "json")

	applyEnvOverrides(cfg)

	if cfg.Server.Port != 9999 {
		t.Errorf("expected env port 9999, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "env-host" {
		t.Errorf("expected env host env-host, got %s", cfg.Server.Host)
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
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080 for invalid env, got %d", cfg.Server.Port)
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
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.Server.Host)
	}
}

// --- MCP Config Tests ---

func TestNewDefaultConfig_APIDefaults(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.API.URL != "http://localhost:8080" {
		t.Errorf("expected default API URL http://localhost:8080, got %s", cfg.API.URL)
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

func TestLoadFromFiles_MCPSections(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "mcp.toml")

	content := `
[api]
url = "http://vire-server:4242"

[user]
portfolios = ["SMSF", "Personal"]
display_currency = "AUD"
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

func TestLoadFromFiles_MCPSectionsDefaultsPreserved(t *testing.T) {
	// When TOML only sets [api], [user] should keep defaults
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
	// User should remain defaults
	if len(cfg.User.Portfolios) != 0 {
		t.Errorf("expected empty portfolios, got %v", cfg.User.Portfolios)
	}
}

// --- Environment / Dev Mode Config Tests ---

func TestNewDefaultConfig_Environment(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Environment != "prod" {
		t.Errorf("expected default environment prod, got %s", cfg.Environment)
	}
}

func TestIsDevMode_Dev(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Environment = "dev"

	if !cfg.IsDevMode() {
		t.Error("expected IsDevMode() to return true for environment=dev")
	}
}

func TestIsDevMode_Prod(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.IsDevMode() {
		t.Error("expected IsDevMode() to return false for environment=prod")
	}
}

func TestIsDevMode_Empty(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Environment = ""

	if cfg.IsDevMode() {
		t.Error("expected IsDevMode() to return false for empty environment")
	}
}

func TestIsDevMode_Adversarial(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"dev", true},
		{"DEV", true},
		{"Dev", true},
		{" dev ", true},
		{"development", false},
		{"staging", false},
		{"prod", false},
		{"production", false},
		{"", false},
		{" ", false},
		{"devv", false},
		{"de v", false},
	}

	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			cfg := NewDefaultConfig()
			cfg.Environment = tc.env
			if got := cfg.IsDevMode(); got != tc.want {
				t.Errorf("IsDevMode() for %q = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestLoadFromFiles_Environment(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "env.toml")

	content := `
environment = "dev"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Environment != "dev" {
		t.Errorf("expected environment dev from TOML, got %s", cfg.Environment)
	}
	if !cfg.IsDevMode() {
		t.Error("expected IsDevMode() true after loading environment=dev from TOML")
	}
}

func TestApplyEnvOverrides_Environment(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_ENV", "dev")

	applyEnvOverrides(cfg)

	if cfg.Environment != "dev" {
		t.Errorf("expected environment dev from VIRE_ENV, got %s", cfg.Environment)
	}
	if !cfg.IsDevMode() {
		t.Error("expected IsDevMode() true after VIRE_ENV=dev")
	}
}

func TestApplyEnvOverrides_EnvironmentOverridesFile(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "prod.toml")

	content := `
environment = "prod"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VIRE_ENV", "dev")

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	// Env should override file value
	if cfg.Environment != "dev" {
		t.Errorf("expected VIRE_ENV to override file environment, got %s", cfg.Environment)
	}
}

// --- Extended Logging Config Tests ---

func TestNewDefaultConfig_LoggingExtended(t *testing.T) {
	cfg := NewDefaultConfig()

	if len(cfg.Logging.Outputs) != 2 {
		t.Errorf("expected 2 default logging outputs, got %d", len(cfg.Logging.Outputs))
	}
	if len(cfg.Logging.Outputs) >= 2 {
		if cfg.Logging.Outputs[0] != "console" {
			t.Errorf("expected first output 'console', got %s", cfg.Logging.Outputs[0])
		}
		if cfg.Logging.Outputs[1] != "file" {
			t.Errorf("expected second output 'file', got %s", cfg.Logging.Outputs[1])
		}
	}
	if cfg.Logging.FilePath != "logs/vire-portal.log" {
		t.Errorf("expected default file path logs/vire-portal.log, got %s", cfg.Logging.FilePath)
	}
	// MaxSizeMB and MaxBackups default to 0 (let arbor use its own defaults)
	if cfg.Logging.MaxSizeMB != 0 {
		t.Errorf("expected default max_size_mb 0, got %d", cfg.Logging.MaxSizeMB)
	}
	if cfg.Logging.MaxBackups != 0 {
		t.Errorf("expected default max_backups 0, got %d", cfg.Logging.MaxBackups)
	}
}

func TestLoadFromFiles_LoggingExtended(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "logging.toml")

	content := `
[logging]
level = "debug"
outputs = ["console"]
file_path = "/var/log/custom.log"
max_size_mb = 50
max_backups = 5
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if len(cfg.Logging.Outputs) != 1 || cfg.Logging.Outputs[0] != "console" {
		t.Errorf("expected outputs [console], got %v", cfg.Logging.Outputs)
	}
	if cfg.Logging.FilePath != "/var/log/custom.log" {
		t.Errorf("expected file path /var/log/custom.log, got %s", cfg.Logging.FilePath)
	}
	if cfg.Logging.MaxSizeMB != 50 {
		t.Errorf("expected max_size_mb 50, got %d", cfg.Logging.MaxSizeMB)
	}
	if cfg.Logging.MaxBackups != 5 {
		t.Errorf("expected max_backups 5, got %d", cfg.Logging.MaxBackups)
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

// --- Port Default Stress Tests ---

func TestNewDefaultConfig_APIDefaultMatchesServerPort(t *testing.T) {
	// After the port migration, both server and API defaults should be 8080.
	// This prevents confusion when running locally without Docker.
	cfg := NewDefaultConfig()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default server port 8080, got %d", cfg.Server.Port)
	}
	if cfg.API.URL != "http://localhost:8080" {
		t.Errorf("expected default API URL http://localhost:8080, got %s", cfg.API.URL)
	}
}

func TestApplyEnvOverrides_LegacyServerPort4241(t *testing.T) {
	// Edge case: If VIRE_SERVER_PORT is still set to 4241 (old default),
	// the server should use it as an override. Verify it works.
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_SERVER_PORT", "4241")

	applyEnvOverrides(cfg)

	if cfg.Server.Port != 4241 {
		t.Errorf("expected overridden port 4241, got %d", cfg.Server.Port)
	}
	// API URL should remain at 8080 unless explicitly overridden
	if cfg.API.URL != "http://localhost:8080" {
		t.Errorf("expected API URL unchanged at http://localhost:8080, got %s", cfg.API.URL)
	}
}

func TestApplyEnvOverrides_PortBoundaryValues(t *testing.T) {
	// Verify boundary port values don't crash
	tests := []struct {
		envVal   string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"65535", 65535},
		{"65536", 65536}, // technically invalid but should still parse
		{"-1", -1},       // negative, should still parse
	}

	for _, tc := range tests {
		t.Run("port_"+tc.envVal, func(t *testing.T) {
			cfg := NewDefaultConfig()
			t.Setenv("VIRE_SERVER_PORT", tc.envVal)
			applyEnvOverrides(cfg)
			if cfg.Server.Port != tc.expected {
				t.Errorf("expected port %d for env %q, got %d", tc.expected, tc.envVal, cfg.Server.Port)
			}
		})
	}
}

func TestApplyEnvOverrides_HostilePortValues(t *testing.T) {
	// Verify hostile env var values for port are safely ignored.
	hostileValues := []string{
		"not-a-number",
		"",
		"8080; rm -rf /",
		"8080\n9090",
		"99999999999999999999", // overflow
		"  8080  ",             // whitespace
	}

	for _, hostile := range hostileValues {
		t.Run("hostile_port_"+hostile, func(t *testing.T) {
			cfg := NewDefaultConfig()
			t.Setenv("VIRE_SERVER_PORT", hostile)
			applyEnvOverrides(cfg)
			// Should either keep default or parse the numeric portion
			// The point is: must not panic
		})
	}
}

func TestDockerfileExposePort(t *testing.T) {
	// Verify Dockerfile EXPOSEs 8080, not 4241.
	dockerfilePath := filepath.Join("..", "..", "docker", "Dockerfile")
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Skipf("could not read Dockerfile: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "EXPOSE 8080") {
		t.Error("Dockerfile should EXPOSE 8080 (internal default port)")
	}
	if strings.Contains(content, "EXPOSE 4241") {
		t.Error("Dockerfile should NOT EXPOSE 4241 — internal port is now 8080")
	}
}

func TestDockerComposeHealthcheck(t *testing.T) {
	// Verify healthcheck URL uses internal port 8080, not external port.
	composePath := filepath.Join("..", "..", "docker", "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Skipf("could not read docker-compose.yml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "localhost:8080/api/health") {
		t.Error("docker-compose healthcheck must use internal port 8080")
	}
	if strings.Contains(content, "localhost:4241/api/health") {
		t.Error("docker-compose healthcheck must NOT use old port 4241")
	}
}

func TestDockerComposeNoServerPortEnv(t *testing.T) {
	// Verify docker-compose.yml does NOT set VIRE_SERVER_PORT.
	// The Go default is already 8080, so setting it is unnecessary and confusing.
	composePath := filepath.Join("..", "..", "docker", "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Skipf("could not read docker-compose.yml: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "VIRE_SERVER_PORT") {
		t.Error("docker-compose.yml should NOT set VIRE_SERVER_PORT — Go default is already 8080")
	}
}
