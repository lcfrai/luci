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

// Capacity release and Git integration use the same exact backend observation.
// Unknown/transport failures must not be laundered through a joined NotFound.
func (a *API) exactBackendStopped(ctx context.Context, sandboxID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if sandboxID == "" {
		return false, errors.New("The stopped task has no exact backend identity; inspect and recover it before releasing its resources.")
	}
	sb, err := a.deps.Backend.Get(ctx, sandboxID)
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		if errors.Is(err, backend.ErrNotFound) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, backend.ErrUnavailable) {
			return true, nil
		}
		return false, fmt.Errorf("The task backend could not be confirmed stopped: %w", err)
	}
	if sb.ID != sandboxID {
		return false, errors.New("The stopped task backend returned a different identity.")
	}
	switch sb.State {
	case backend.StateGone, backend.StateStopped:
		return true, nil
	case backend.StateStarting, backend.StateRunning:
		return false, nil
	default:
		return false, errors.New("The task backend state is unknown; inspect it before releasing its resources.")
	}
}
