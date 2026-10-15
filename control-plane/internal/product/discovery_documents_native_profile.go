package product

import (
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"time"
)

// Shared validation calls this in place of the old unconditional 120s maximum.
// Empty is the legacy text profile; only the service creates the native field.
func validDiscoveryAttemptProfile(profile string, started, expires time.Time) bool {
	if !expires.After(started) {
		return false
	}
	switch profile {
	case "":
		return expires.Sub(started) <= discoveryAttemptLifetime
	case backend.DocumentExtractionProfile:
		return expires.Sub(started) == backend.DocumentWholeBudget
	default:
		return false
	}
}
