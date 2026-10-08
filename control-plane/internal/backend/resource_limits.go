package backend

import (
	"context"
	"errors"
)

// ResourceLimitValidator is an optional read-only preparation capability. It
// must run before the caller publishes a backend dispatch claim. A subsequent
// failure from Backend.Create is not proof that no allocation happened.
type ResourceLimitValidator interface {
	ValidateResourceLimits(context.Context, Resources) error
}

var ErrResourceLimitsUnsupported = errors.New("the configured execution engine cannot enforce the requested memory and swap limits")
