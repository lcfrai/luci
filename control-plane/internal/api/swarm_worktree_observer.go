package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// An exited registry row alone cannot authorize Git review/integration: a
// delayed or failed destroy may have left its exact backend writing files.
func (a *API) observeSwarmWorktree(ctx context.Context, projectID, worktreeID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	rows, err := a.deps.Store.List()
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.ProjectID != projectID || row.WorktreeID != worktreeID {
			continue
		}
		if row.Status != registry.StatusExited {
			return true, nil
		}
		if a.deps.ResourceSnapshot != nil {
			if err := a.execution.confirmNoPendingLaunch(ctx, row); err != nil {
				return false, err
			}
		}
		stopped, err := a.exactBackendStopped(ctx, row.SandboxID)
		if err != nil {
			return false, err
		}
		if !stopped {
			return true, nil
		}
	}
	return false, nil
}

// Git review accepts an exactly observed stopped backend. Task capacity release
// additionally requires absence so a failed removal can retain its retry proof.
func (a *API) exactBackendStopped(ctx context.Context, sandboxID string) (bool, error) {
	return a.exactBackendTerminal(ctx, sandboxID, false)
}

func (a *API) exactBackendTerminal(ctx context.Context, sandboxID string, requireAbsent bool) (bool, error) {
	terminal, _, err := a.exactBackendTerminalObservation(ctx, sandboxID, requireAbsent)
	return terminal, err
}

// Return the same exact observation to the capacity-release caller so a
// stopped-but-present classification does not perform a second name lookup.
func (a *API) exactBackendTerminalObservation(ctx context.Context, sandboxID string, requireAbsent bool) (bool, backend.Sandbox, error) {
	if err := ctx.Err(); err != nil {
		return false, backend.Sandbox{}, err
	}
	if sandboxID == "" {
		return false, backend.Sandbox{}, errors.New("The stopped task has no exact backend identity; inspect and recover it before releasing its resources.")
	}
	sb, err := a.deps.Backend.Get(ctx, sandboxID)
	if ctx.Err() != nil {
		return false, backend.Sandbox{}, ctx.Err()
	}
	if err != nil {
		if unambiguousBackendNotFound(err) {
			return true, backend.Sandbox{}, nil
		}
		return false, backend.Sandbox{}, fmt.Errorf("The task backend could not be confirmed stopped: %w", err)
	}
	if sb.ID != sandboxID {
		return false, backend.Sandbox{}, errors.New("The stopped task backend returned a different identity.")
	}
	switch sb.State {
	case backend.StateGone:
		return true, sb, nil
	case backend.StateStopped:
		return !requireAbsent, sb, nil
	case backend.StateStarting, backend.StateRunning:
		return false, sb, nil
	default:
		return false, backend.Sandbox{}, errors.New("The task backend state is unknown; inspect it before releasing its resources.")
	}
}

// Only a single wrapped NotFound is an absence witness. A joined error can
// carry an unrelated transport failure even when errors.Is also finds NotFound.
func unambiguousBackendNotFound(err error) bool {
	for depth := 0; err != nil && depth < 64; depth++ {
		if _, joined := err.(interface{ Unwrap() []error }); joined {
			return false
		}
		if err == backend.ErrNotFound {
			return true
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, backend.ErrUnavailable) {
			return false
		}
		err = errors.Unwrap(err)
	}
	return false
}
