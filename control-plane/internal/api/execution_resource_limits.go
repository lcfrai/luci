package api

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// This read-only optional check runs before the durable backend claim. A
// repeated check inside Create is race defense; its post-claim failure still
// remains unknown under the ordinary launch protocol, never absence proof.
func (a *API) validateExecutionResourceLimits(ctx context.Context, resources backend.Resources) error {
	validator, ok := a.deps.Backend.(backend.ResourceLimitValidator)
	if !ok {
		return nil
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := validator.ValidateResourceLimits(bounded, resources)
	if err == nil {
		return nil
	}
	if errors.Is(err, backend.ErrResourceLimitsUnsupported) {
		return admissionFailure("resource_limits_unsupported", err)
	}
	return admissionFailure("resource_observation_unavailable", err)
}
