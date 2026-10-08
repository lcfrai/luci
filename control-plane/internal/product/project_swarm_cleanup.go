package product

import (
	"context"
	"errors"
)

// SwarmCleanupPending reports a positively identified stopped allocation whose
// exact capacity reservation is still retained. Producers must establish the
// current engine, immutable object, and private ownership before using it.
// It grants no release, replacement, integration, or cleanup authority.
func SwarmCleanupPending(cause error) error {
	return &swarmCleanupPendingError{cause: cause}
}

type swarmCleanupPendingError struct{ cause error }

func (e *swarmCleanupPendingError) Error() string {
	return "The exact stopped allocation is retained awaiting cleanup."
}
func (e *swarmCleanupPendingError) Unwrap() error { return e.cause }

// Only one positively classified error can be local to an item. In particular,
// a joined cancellation or another failure must not turn into permission to
// continue unrelated admission merely because one branch has this marker.
func swarmCleanupIsPending(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	pending := false
	for depth := 0; err != nil && depth < 64; depth++ {
		if _, joined := err.(interface{ Unwrap() []error }); joined {
			return false
		}
		if _, marked := err.(*swarmCleanupPendingError); marked {
			pending = true
		}
		err = errors.Unwrap(err)
	}
	return pending && err == nil
}
