package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// ServiceConfig: Hostile Input Tests
// =============================================================================

func TestServiceConfig_HostileKeyValues(t *testing.T) {
	// Hostile VIRE_SERVICE_KEY values should be stored as-is (no crash).
	hostileKeys := []string{
		"'; DROP TABLE services; --",
		"<script>alert(1)</script>",
		"key\r\nX-Injected: evil",
		strings.Repeat("A", 100000), // 100KB key
		"$(whoami)",
		"`id`",
		"key; rm -rf /",
		"key\nkey2",
		"",
		" ",
	}

	for _, key := range hostileKeys {
		t.Run("key_"+key[:min(len(key), 20)], func(t *testing.T) {
			cfg := NewDefaultConfig()
			t.Setenv("VIRE_SERVICE_KEY", key)
			applyEnvOverrides(cfg)
			// Must not panic; key should be stored as-is
			if key != "" && cfg.Service.Key != key {
				t.Errorf("expected service key %q, got %q", key, cfg.Service.Key)
			}
		})
	}
}

func TestServiceConfig_HostilePortalIDValues(t *testing.T) {
	hostileIDs := []string{
		"../../etc/passwd",
		"<script>alert(1)</script>",
		"portal\r\nX-Injected: evil",
		strings.Repeat("A", 100000),
		"$(whoami)",
		"portal id with spaces",
		"portal;id",
		"portal\nid",
	}

	for _, id := range hostileIDs {
		t.Run("id_"+id[:min(len(id), 20)], func(t *testing.T) {
			cfg := NewDefaultConfig()
			t.Setenv("VIRE_PORTAL_ID", id)
			applyEnvOverrides(cfg)
			if cfg.Service.PortalID != id {
				t.Errorf("expected portal ID %q, got %q", id, cfg.Service.PortalID)
			}
		})
	}
}

// =============================================================================
// ServiceConfig: TOML Parsing Edge Cases
// =============================================================================

func TestServiceConfig_EmptyKeyInTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "svc.toml")

	content := `
[service]
key = ""
portal_id = ""
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Service.Key != "" {
		t.Errorf("expected empty service key, got %q", cfg.Service.Key)
	}
	if cfg.Service.PortalID != "" {
		t.Errorf("expected empty portal ID, got %q", cfg.Service.PortalID)
	}
}

func TestServiceConfig_MissingSection(t *testing.T) {
	// TOML file with no [service] section should keep defaults.
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "no-svc.toml")

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

	if cfg.Service.Key != "" {
		t.Errorf("expected empty default service key, got %q", cfg.Service.Key)
	}
	if cfg.Service.PortalID != "" {
		t.Errorf("expected empty default portal ID, got %q", cfg.Service.PortalID)
	}
}

func TestServiceConfig_PartialSection(t *testing.T) {
	// Only key is set; portal_id should keep default.
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "partial-svc.toml")

	content := `
[service]
key = "my-key"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Service.Key != "my-key" {
		t.Errorf("expected service key my-key, got %q", cfg.Service.Key)
	}
	if cfg.Service.PortalID != "" {
		t.Errorf("expected empty default portal ID, got %q", cfg.Service.PortalID)
	}
}

// =============================================================================
// ServiceConfig: Env Override Precedence
// =============================================================================

func TestServiceConfig_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "svc.toml")

	content := `
[service]
key = "file-key"
portal_id = "file-portal"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VIRE_SERVICE_KEY", "env-key")
	t.Setenv("VIRE_PORTAL_ID", "env-portal")

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Service.Key != "env-key" {
		t.Errorf("expected env key override, got %q", cfg.Service.Key)
	}
	if cfg.Service.PortalID != "env-portal" {
		t.Errorf("expected env portal ID override, got %q", cfg.Service.PortalID)
	}
}

func TestServiceConfig_EmptyEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "svc.toml")

	content := `
[service]
key = "file-key"
portal_id = "file-portal"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty env vars should NOT override file values
	t.Setenv("VIRE_SERVICE_KEY", "")
	t.Setenv("VIRE_PORTAL_ID", "")

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Service.Key != "file-key" {
		t.Errorf("empty env should not override file key, got %q", cfg.Service.Key)
	}
	if cfg.Service.PortalID != "file-portal" {
		t.Errorf("empty env should not override file portal ID, got %q", cfg.Service.PortalID)
	}
}

// =============================================================================
// ServiceConfig: Config Interaction Edge Cases
// =============================================================================

func TestServiceConfig_KeySetButAdminUsersEmpty(t *testing.T) {
	// Service key set but no admin users — should not cause issues.
	cfg := NewDefaultConfig()
	cfg.Service.Key = "my-key"
	cfg.AdminUsers = ""

	emails := cfg.AdminEmails()
	if len(emails) != 0 {
		t.Errorf("expected 0 emails with empty admin_users, got %d", len(emails))
	}
	// This config combination means: register service but skip admin sync.
}

func TestServiceConfig_AdminUsersSetButKeyEmpty(t *testing.T) {
	// Admin users set but no service key — should trigger warning in app.go.
	cfg := NewDefaultConfig()
	cfg.Service.Key = ""
	cfg.AdminUsers = "alice@example.com"

	emails := cfg.AdminEmails()
	if len(emails) != 1 {
		t.Errorf("expected 1 email, got %d", len(emails))
	}
	if cfg.Service.Key != "" {
		t.Error("expected empty service key")
	}
	// This config combination means: log warning, skip admin sync.
}

func TestServiceConfig_BothEmpty(t *testing.T) {
	cfg := NewDefaultConfig()
	if cfg.Service.Key != "" || cfg.Service.PortalID != "" || cfg.AdminUsers != "" {
		t.Error("expected all empty for default config")
	}
}

func TestServiceConfig_KeyWithSpecialChars(t *testing.T) {
	// Real service keys might have special characters.
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "special.toml")

	// TOML strings handle most special characters; test some edge cases.
	content := `
[service]
key = "key-with-dashes_and_underscores+plus=equals/slash"
portal_id = "portal.with.dots-and-dashes"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Service.Key != "key-with-dashes_and_underscores+plus=equals/slash" {
		t.Errorf("expected special chars in key, got %q", cfg.Service.Key)
	}
	if cfg.Service.PortalID != "portal.with.dots-and-dashes" {
		t.Errorf("expected dots and dashes in portal ID, got %q", cfg.Service.PortalID)
	}
}

func TestServiceConfig_MultiFileOverride(t *testing.T) {
	dir := t.TempDir()

	base := filepath.Join(dir, "base.toml")
	baseContent := `
[service]
key = "base-key"
portal_id = "base-portal"
`
	if err := os.WriteFile(base, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	override := filepath.Join(dir, "override.toml")
	overrideContent := `
[service]
key = "override-key"
`
	if err := os.WriteFile(override, []byte(overrideContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFiles(base, override)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	if cfg.Service.Key != "override-key" {
		t.Errorf("expected override key, got %q", cfg.Service.Key)
	}
	// portal_id should come from base (not overridden)
	if cfg.Service.PortalID != "base-portal" {
		t.Errorf("expected base portal ID preserved, got %q", cfg.Service.PortalID)
	}
}

// =============================================================================
// VIRE_PORTAL_ID vs VIRE_PORTAL_URL Confusion
// =============================================================================

func TestServiceConfig_PortalIDNotConfusedWithPortalURL(t *testing.T) {
	// VIRE_PORTAL_ID (service registration) vs VIRE_PORTAL_URL (auth callback).
	// These are completely different settings. Verify they don't cross-contaminate.
	cfg := NewDefaultConfig()

	t.Setenv("VIRE_PORTAL_ID", "portal-instance-1")
	t.Setenv("VIRE_PORTAL_URL", "https://portal.vire.dev")

	applyEnvOverrides(cfg)

	if cfg.Service.PortalID != "portal-instance-1" {
		t.Errorf("expected VIRE_PORTAL_ID = portal-instance-1, got %q", cfg.Service.PortalID)
	}
	if cfg.Auth.PortalURL != "https://portal.vire.dev" {
		t.Errorf("expected VIRE_PORTAL_URL = https://portal.vire.dev, got %q", cfg.Auth.PortalURL)
	}
	// Verify no cross-contamination
	if cfg.Service.PortalID == cfg.Auth.PortalURL {
		t.Error("VIRE_PORTAL_ID and VIRE_PORTAL_URL should be independent")
	}
}
