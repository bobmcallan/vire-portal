package seed

import (
	"fmt"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// ReportPortalVersion reports the portal version and build to vire-server.
// It retries on failure using the same retry logic as RegisterService.
func ReportPortalVersion(apiURL, serviceUserID, version, build string, logger *common.Logger) error {
	c := client.NewVireClient(apiURL, "", "")

	var err error

	for attempt := 1; attempt <= seedRetryAttempts; attempt++ {
		err = c.ReportPortalVersion(serviceUserID, version, build)
		if err == nil {
			if logger != nil {
				logger.Info().
					Str("version", version).
					Str("build", build).
					Msg("portal version: reported successfully")
			}
			return nil
		}

		if logger != nil {
			logger.Warn().
				Int("attempt", attempt).
				Int("max_attempts", seedRetryAttempts).
				Str("error", err.Error()).
				Msg("portal version: report failed, retrying")
		}
		if attempt < seedRetryAttempts {
			time.Sleep(seedRetryDelay)
		}
	}

	return fmt.Errorf("portal version report failed after %d attempts: %w", seedRetryAttempts, err)
}
