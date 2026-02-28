package seed

import (
	"fmt"
	"strings"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// SyncAdmins ensures the given email addresses have the admin role on vire-server.
// It uses the admin API endpoints with service authentication.
// It is idempotent and safe for concurrent multi-instance execution.
// Users not found are logged and skipped. Existing admins are not modified.
func SyncAdmins(apiURL string, adminEmails []string, serviceUserID string, logger *common.Logger) {
	if len(adminEmails) == 0 {
		return
	}

	c := client.NewVireClient(apiURL)
	syncAdminsWithRetry(c, adminEmails, serviceUserID, logger)
}

// syncAdminsWithRetry attempts the admin sync with retries matching the DevUsers pattern.
func syncAdminsWithRetry(c *client.VireClient, adminEmails []string, serviceUserID string, logger *common.Logger) {
	var err error
	for attempt := 1; attempt <= seedRetryAttempts; attempt++ {
		err = syncAdminsOnce(c, adminEmails, serviceUserID, logger)
		if err == nil {
			return
		}
		if logger != nil {
			logger.Warn().
				Int("attempt", attempt).
				Int("max_attempts", seedRetryAttempts).
				Str("error", err.Error()).
				Msg("admin sync: failed, retrying")
		}
		if attempt < seedRetryAttempts {
			time.Sleep(seedRetryDelay)
		}
	}

	if logger != nil {
		logger.Warn().
			Int("attempts", seedRetryAttempts).
			Str("error", err.Error()).
			Msg("admin sync: failed after retries, continuing without sync")
	}
}

// syncAdminsOnce performs a single attempt at syncing admin roles using the admin API.
func syncAdminsOnce(c *client.VireClient, adminEmails []string, serviceUserID string, logger *common.Logger) error {
	users, err := c.AdminListUsers(serviceUserID)
	if err != nil {
		return err
	}

	// Build map: lowercase(email) -> AdminUser
	emailToUser := make(map[string]client.AdminUser, len(users))
	for _, u := range users {
		emailToUser[strings.ToLower(u.Email)] = u
	}

	updated := 0
	notFound := 0
	failed := 0
	var lastErr error

	for _, email := range adminEmails {
		e := strings.ToLower(email)
		user, found := emailToUser[e]
		if !found {
			notFound++
			if logger != nil {
				logger.Info().Str("email", e).Msg("admin sync: email not found in users")
			}
			continue
		}
		if user.Role == "admin" {
			continue
		}
		err := c.AdminUpdateUserRole(serviceUserID, user.ID, "admin")
		if err != nil {
			failed++
			lastErr = err
			if logger != nil {
				logger.Warn().Str("user_id", user.ID).Str("error", err.Error()).Msg("admin sync: failed to update user")
			}
			continue
		}
		updated++
		if logger != nil {
			logger.Info().Str("user_id", user.ID).Str("email", e).Msg("admin sync: updated role to admin")
		}
	}

	if logger != nil {
		logger.Info().
			Int("checked", len(adminEmails)).
			Int("updated", updated).
			Int("not_found", notFound).
			Int("failed", failed).
			Msg("admin sync: completed")
	}

	if lastErr != nil {
		return fmt.Errorf("admin sync: %d update(s) failed, last error: %w", failed, lastErr)
	}

	return nil
}
