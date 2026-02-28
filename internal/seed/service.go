package seed

import (
	"fmt"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// RegisterService registers this portal instance as a service user with vire-server.
// It retries on failure using the same retry logic as DevUsers.
// Returns the service_user_id for use in admin API calls.
func RegisterService(apiURL, serviceID, serviceKey string, logger *common.Logger) (string, error) {
	c := client.NewVireClient(apiURL)

	var serviceUserID string
	var err error

	for attempt := 1; attempt <= seedRetryAttempts; attempt++ {
		serviceUserID, err = c.RegisterService(serviceID, serviceKey)
		if err == nil {
			if logger != nil {
				logger.Info().
					Str("service_user_id", serviceUserID).
					Msg("service registration: registered successfully")
			}
			return serviceUserID, nil
		}

		if logger != nil {
			logger.Warn().
				Int("attempt", attempt).
				Int("max_attempts", seedRetryAttempts).
				Str("error", err.Error()).
				Msg("service registration: failed, retrying")
		}
		if attempt < seedRetryAttempts {
			time.Sleep(seedRetryDelay)
		}
	}

	return "", fmt.Errorf("service registration failed after %d attempts: %w", seedRetryAttempts, err)
}
