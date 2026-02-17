package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

const (
	seedRetryAttempts = 3
	seedRetryDelay    = 2 * time.Second
	usersFileName     = "import/users.json"
)

// usersFile is the JSON structure for the users seed file.
type usersFile struct {
	Users []client.SeedUser `json:"users"`
}

// DevUsers seeds dev users into vire-server from import/users.json.
// Non-fatal: if vire-server is unreachable after retries, logs warning and returns.
func DevUsers(apiURL string, logger *common.Logger) {
	path := findUsersFile()
	if path == "" {
		logger.Warn().Msg("seed: import/users.json not found, skipping dev user seeding")
		return
	}

	users, err := loadUsersFile(path)
	if err != nil {
		logger.Error().Str("error", err.Error()).Str("path", path).Msg("seed: failed to load users file")
		return
	}

	if len(users) == 0 {
		logger.Warn().Msg("seed: users file is empty, skipping dev user seeding")
		return
	}

	c := client.NewVireClient(apiURL)
	seedWithRetry(c, users, logger)
}

// findUsersFile searches for import/users.json relative to the executable
// directory first, then falls back to the current working directory.
func findUsersFile() string {
	// Try binary-relative path first
	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
		p := filepath.Join(binDir, usersFileName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fall back to CWD
	if _, err := os.Stat(usersFileName); err == nil {
		return usersFileName
	}

	return ""
}

// loadUsersFile reads and parses the users JSON file.
func loadUsersFile(path string) ([]client.SeedUser, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var f usersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return f.Users, nil
}

// seedWithRetry attempts to seed users with retries.
func seedWithRetry(c *client.VireClient, users []client.SeedUser, logger *common.Logger) {
	var err error
	for attempt := 1; attempt <= seedRetryAttempts; attempt++ {
		err = seedAll(c, users, logger)
		if err == nil {
			logger.Info().Int("users", len(users)).Msg("seed: dev users seeded successfully")
			return
		}
		logger.Warn().
			Int("attempt", attempt).
			Int("max_attempts", seedRetryAttempts).
			Str("error", err.Error()).
			Msg("seed: failed to seed users, retrying")
		if attempt < seedRetryAttempts {
			time.Sleep(seedRetryDelay)
		}
	}

	logger.Warn().
		Int("attempts", seedRetryAttempts).
		Str("error", err.Error()).
		Msg("seed: failed to seed dev users after retries, continuing without seeding")
}

// seedAll upserts each user, returning on first error.
func seedAll(c *client.VireClient, users []client.SeedUser, logger *common.Logger) error {
	for _, u := range users {
		if err := c.UpsertUser(u); err != nil {
			return fmt.Errorf("upsert %s: %w", u.Username, err)
		}
		if logger != nil {
			logger.Debug().Str("username", u.Username).Msg("seed: upserted user")
		}
	}
	return nil
}
