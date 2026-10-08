package api

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Preserve the only product handle through which an ambiguous allocator can
// be explicitly recovered. A failed/exited registry row is not a launch ACK.
// No backend calls or file mutations occur while product holds its run guard.
func (a *API) projectSwarmPendingLaunch(ctx context.Context, projectID, runID, operationID, childID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if childID == "" {
		return false, nil
	}
	row, err := a.deps.Store.Get(childID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return false, err
	}
	if err == nil && (row.ProjectID != projectID || row.ProjectRunID != runID || row.ProjectRunOperationID != operationID || row.SandboxID != childID) {
		return false, errLaunchRecoveryUnproved
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil {
		return false, err
	}
	for _, record := range ledger.Fences {
		if record.Scope != a.execution.scope || record.ChildID != childID {
			continue
		}
		if record.ProjectID != projectID || record.RunID != runID || record.OperationID != operationID {
			return false, errLaunchRecoveryUnproved
		}
		if record.State != "retired" {
			return true, nil
		}
	}
	for _, lease := range ledger.Leases {
		if lease.Scope != a.execution.scope || lease.SessionID != childID {
			continue
		}
		if lease.ProjectID != projectID || lease.SandboxID != childID {
			return false, errLaunchRecoveryUnproved
		}
		// A pre-backend lease can be safely retired by an atomic deletion,
		// but merely observing it leaves a late allocator able to dispatch.
		// Preserve this attempt until that deletion or an actual backend ACK.
		if lease.LaunchGuard != 1 || lease.State != "backend-settled" {
			return true, nil
		}
	}
	return false, nil
}
