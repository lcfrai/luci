package backend

import "context"

// AllocationAbsenceObserver is an optional read-only maintenance capability.
// True proves both the reusable logical name and the recorded immutable object
// are absent. Present, foreign, renamed, removing, or uncertain objects cannot
// be projected as absence. The caller fences concurrent allocation and checks
// the same settled registry generation and capacity receipt before retirement.
// Backends without this proof retain the existing on-demand release behavior.
type AllocationAbsenceObserver interface {
	AllocationAbsent(ctx context.Context, id, immutableRef string) (bool, error)
}
