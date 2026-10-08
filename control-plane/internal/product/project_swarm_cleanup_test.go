package product

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestSwarmCleanupPendingDoesNotSuppressGlobalFailure(t *testing.T) {
	held := errors.New("capacity remains held")
	for _, tc := range []struct {
		name  string
		err   error
		local bool
	}{
		{"exact local pending", SwarmCleanupPending(held), true},
		{"wrapped local pending", fmt.Errorf("cleanup: %w", SwarmCleanupPending(held)), true},
		{"unclassified held", held, false},
		{"ledger corruption", errors.New("execution ledger is corrupt"), false},
		{"joined global failure", errors.Join(SwarmCleanupPending(held), errors.New("backend unavailable")), false},
		{"nested joined failure", SwarmCleanupPending(errors.Join(held, errors.New("backend unavailable"))), false},
		{"cancellation", SwarmCleanupPending(context.Canceled), false},
		{"deadline", fmt.Errorf("cleanup: %w", SwarmCleanupPending(context.DeadlineExceeded)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := swarmCleanupIsPending(tc.err); got != tc.local {
				t.Fatalf("item-local=%v, want %v", got, tc.local)
			}
			if tc.local && !errors.Is(tc.err, held) {
				t.Fatal("classification lost retained-capacity cause")
			}
		})
	}
}
