package api

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type manualLeaseObservations struct {
	mu      sync.Mutex
	cursor  int
	pending []executionLease
	failed  bool
}

// Manual cleanup observation has its own budget; slow backend absence must not
// consume the task report observer's existing budget. Both finish before the
// service-owned scheduler turn returns, including parent cancellation.
func (a *API) observeExecutionCompletions(ctx context.Context) error {
	manual := make(chan error, 1)
	go func() { manual <- a.ObserveManualLeasesOnce(ctx) }()
	tasks := a.ObserveTaskSessionsOnce(ctx)
	return errors.Join(tasks, <-manual)
}

var errManualLeaseUnchanged = errors.New("manual allocation remains reserved")

// ObserveManualLeasesOnce retires only settled ordinary-session reservations
// whose exact backend allocation is now absent. It never destroys, allocates,
// replays work, supplies user authority, or infers an exit outcome. Typed Exit
// and absence from an active-only resource inventory are insufficient proofs.
// Task/check leases keep their separate completion and cleanup contracts.
func (a *API) ObserveManualLeasesOnce(ctx context.Context) (result error) {
	g := a.execution
	if g == nil || a.deps.Backend == nil || a.deps.Backend.Kind() != backend.KindDocker {
		return nil
	}
	if _, supported := a.deps.Backend.(backend.AllocationAbsenceObserver); !supported {
		return nil
	}
	state := &g.manualObserver
	if !state.mu.TryLock() {
		return nil
	}
	defer state.mu.Unlock()
	defer func() {
		if result != nil && !state.failed {
			a.sessionLog().Warn("manual capacity observation remains pending; exact ownership will be retried")
		}
		state.failed = result != nil
	}()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(state.pending) == 0 {
		ledger, err := g.readFenceLedger()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if state.cursor >= len(ledger.Leases) {
			state.cursor = 0
		}
		for visited := 0; visited < len(ledger.Leases) && visited < 128 && len(state.pending) < 8; visited++ {
			lease := ledger.Leases[state.cursor]
			state.cursor = (state.cursor + 1) % len(ledger.Leases)
			if lease.Scope == g.scope && lease.SessionID != "" && lease.SandboxID != "" && lease.LaunchGuard == 1 && lease.State == "backend-settled" {
				state.pending = append(state.pending, lease)
			}
		}
	}
	var failures []error
	for len(state.pending) > 0 {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		// Consume only an attempted candidate. Unattempted peers survive a slow
		// first lookup, so later ticks cannot repeatedly starve the same page.
		lease := state.pending[0]
		state.pending[0] = executionLease{}
		state.pending = state.pending[1:]
		if err := a.observeManualLease(ctx, lease); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func exactManualLease(ledger executionLeaseLedger, expected executionLease) bool {
	matches, exact := 0, false
	for _, current := range ledger.Leases {
		if current.SandboxID == expected.SandboxID || current.Scope == expected.Scope && current.SessionID == expected.SessionID {
			matches++
			exact = exact || current == expected
		}
	}
	return matches == 1 && exact
}

func manualLeaseRow(row registry.Session, lease executionLease, kind backend.Kind) bool {
	return row.ID == lease.SessionID && row.ProjectID == lease.ProjectID && row.SandboxID == lease.SandboxID &&
		row.Status == registry.StatusExited && row.Backend == string(kind) && row.SandboxRef != "" &&
		row.ProjectRunID == "" && row.ProjectRunOperationID == "" && row.StartFailure.IsZero() && lease.FenceProtocol == 0
}

func sameManualLeaseGeneration(current, observed registry.Session) bool {
	return current.ID == observed.ID && current.Status == registry.StatusExited &&
		current.Backend == observed.Backend && current.SandboxID == observed.SandboxID && current.SandboxRef == observed.SandboxRef &&
		current.ProjectID == observed.ProjectID && current.WorktreeID == observed.WorktreeID && current.BranchID == observed.BranchID &&
		current.ProjectRunID == "" && current.ProjectRunOperationID == "" && current.StartFailure.IsZero() &&
		current.AttachToken == observed.AttachToken && current.CreatedAt.Equal(observed.CreatedAt) && current.UpdatedAt.Equal(observed.UpdatedAt)
}

func (a *API) observeManualLease(ctx context.Context, lease executionLease) error {
	g := a.execution
	// Match start/stop/restart's lock order. The process guard is nonblocking;
	// the durable child lock protects another controller's managed restart.
	if !sessionRestarts.acquire(lease.SessionID) {
		return nil
	}
	defer sessionRestarts.release(lease.SessionID)
	unlock, err := g.lockChildLifecycle(ctx, lease.SessionID)
	if err != nil {
		return err
	}
	defer unlock()
	ledger, err := g.readFenceLedger()
	if err != nil {
		return err
	}
	if !exactManualLease(ledger, lease) {
		return nil
	}
	row, err := a.deps.Store.Get(lease.SessionID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !manualLeaseRow(row, lease, a.deps.Backend.Kind()) {
		return nil
	}
	// Both the reusable name and recorded immutable reference must be absent.
	// Every present/foreign/removing object and every ambiguous error retains.
	// Backend I/O never holds the host-wide admission ledger lock.
	absent, err := a.deps.Backend.(backend.AllocationAbsenceObserver).AllocationAbsent(ctx, lease.SandboxID, row.SandboxRef)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return errManualLeaseUnchanged
	}
	if !absent {
		return nil
	}
	// A background turn never queues behind an uncancellable local mutex.
	// The existing shared-file lock remains context bounded inside mutateLocked.
	if !g.mu.TryLock() {
		return nil
	}
	defer g.mu.Unlock()
	err = g.mutateLocked(ctx, func(current *executionLeaseLedger) error {
		if !exactManualLease(*current, lease) {
			return errManualLeaseUnchanged
		}
		fresh, err := a.deps.Store.Get(lease.SessionID)
		if err != nil || !sameManualLeaseGeneration(fresh, row) {
			return errManualLeaseUnchanged
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		for i, item := range current.Leases {
			if item == lease {
				current.Leases = append(current.Leases[:i], current.Leases[i+1:]...)
				return nil
			}
		}
		return errManualLeaseUnchanged
	})
	if errors.Is(err, errManualLeaseUnchanged) {
		return nil
	}
	return err
}
