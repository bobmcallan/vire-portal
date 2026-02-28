package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- AdminEmails Hostile Input Tests ---

func TestAdminEmails_HostileValues(t *testing.T) {
	// Hostile VIRE_ADMIN_USERS values should not crash AdminEmails().
	hostileInputs := []string{
		"'; DROP TABLE users; --",
		"<script>alert(1)</script>",
		"alice@example.com\r\nBcc: evil@attacker.com",
		"alice@example.com\x00bob@example.com",
		strings.Repeat("a@b.com,", 10000),
		strings.Repeat(",", 10000),
		"$(whoami)@example.com",
		"`id`@example.com",
		"user@example.com; rm -rf /",
	}

	for _, input := range hostileInputs {
		t.Run("hostile_"+input[:min(len(input), 30)], func(t *testing.T) {
			cfg := NewDefaultConfig()
			cfg.AdminUsers = input
			// Must not panic
			emails := cfg.AdminEmails()
			// All returned emails should be lowercase and trimmed
			for _, email := range emails {
				if email != strings.TrimSpace(email) {
					t.Errorf("email has untrimmed whitespace: %q", email)
				}
				if email != strings.ToLower(email) {
					t.Errorf("email not lowercased: %q", email)
				}
				if email == "" {
					t.Error("empty string should be filtered out")
				}
			}
		})
	}
}

func TestAdminEmails_OnlyCommas(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.AdminUsers = ",,,"
	emails := cfg.AdminEmails()
	if len(emails) != 0 {
		t.Errorf("expected 0 emails for only commas, got %d: %v", len(emails), emails)
	}
}

func TestAdminEmails_WhitespaceOnly(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.AdminUsers = "  ,  ,  "
	emails := cfg.AdminEmails()
	if len(emails) != 0 {
		t.Errorf("expected 0 emails for whitespace-only entries, got %d: %v", len(emails), emails)
	}
}

func TestAdminEmails_TrailingComma(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.AdminUsers = "alice@example.com,"
	emails := cfg.AdminEmails()
	if len(emails) != 1 {
		t.Errorf("expected 1 email with trailing comma, got %d: %v", len(emails), emails)
	}
}

func TestAdminEmails_LeadingComma(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.AdminUsers = ",alice@example.com"
	emails := cfg.AdminEmails()
	if len(emails) != 1 {
		t.Errorf("expected 1 email with leading comma, got %d: %v", len(emails), emails)
	}
}

// --- VIRE_ADMIN_USERS Env Override Edge Cases ---

func TestApplyEnvOverrides_AdminUsersEmpty(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.AdminUsers = "existing@example.com"
	t.Setenv("VIRE_ADMIN_USERS", "")
	applyEnvOverrides(cfg)

	// Empty env var should NOT override existing value
	if cfg.AdminUsers != "existing@example.com" {
		t.Errorf("empty VIRE_ADMIN_USERS should not override existing value, got %q", cfg.AdminUsers)
	}
}

func TestApplyEnvOverrides_AdminUsersHostile(t *testing.T) {
	cfg := NewDefaultConfig()
	t.Setenv("VIRE_ADMIN_USERS", "'; DROP TABLE users; --")
	applyEnvOverrides(cfg)

	// Should store the raw value -- AdminEmails() handles sanitization
	if cfg.AdminUsers != "'; DROP TABLE users; --" {
		t.Errorf("expected hostile value stored as-is, got %q", cfg.AdminUsers)
	}

	// AdminEmails should return it as a single "email"
	emails := cfg.AdminEmails()
	if len(emails) != 1 {
		t.Errorf("expected 1 email entry, got %d: %v", len(emails), emails)
	}
}

// --- TOML Env Override Precedence ---

func TestLoadFromFiles_AdminUsersEnvOverridesToml(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "admin.toml")

	content := `admin_users = "toml@example.com"`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VIRE_ADMIN_USERS", "env@example.com")

	cfg, err := LoadFromFiles(tomlPath)
	if err != nil {
		t.Fatalf("LoadFromFiles failed: %v", err)
	}

	// Env should override TOML
	if cfg.AdminUsers != "env@example.com" {
		t.Errorf("expected env override, got %q", cfg.AdminUsers)
	}
}

// --- Default Config ---

func TestNewDefaultConfig_AdminUsersEmpty(t *testing.T) {
	cfg := NewDefaultConfig()
	if cfg.AdminUsers != "" {
		t.Errorf("expected empty default AdminUsers, got %q", cfg.AdminUsers)
	}
	emails := cfg.AdminEmails()
	if len(emails) != 0 {
		t.Errorf("expected 0 emails for default config, got %d", len(emails))
	}
}
