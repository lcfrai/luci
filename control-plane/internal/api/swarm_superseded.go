package api

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// stopOwnedSwarmTask is called only after a durable exact queue stop intent.
// It never allocates a replacement. A missing ACK, ownership
// mismatch, or incomplete lease settlement leaves the row charged for retry.
func (a *API) stopOwnedSwarmTask(ctx context.Context, req product.SwarmSupersededRequest) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	queue, err := a.deps.Product.ProjectSwarmQueueSnapshot(req.ProjectID)
	if err != nil {
		return false, err
	}
	owned := false
	claimOnly := false
	for _, item := range queue.Items {
		if item.ID == req.QueueID && item.ProjectID == req.ProjectID && item.RunID == req.RunID && item.TaskID == req.TaskID &&
			item.OperationID == req.OperationID && item.ChildID == req.ChildID && (item.ReasonCode == "context_superseded_stop_pending" || item.ReasonCode == "context_superseded_unallocated_claim" || item.ReasonCode == "operator_stop_pending") {
			owned = true
			claimOnly = item.ReasonCode == "context_superseded_unallocated_claim"
			break
		}
	}
	if !owned || req.Actor.ProjectID != req.ProjectID {
		return false, product.ErrProjectRunConflict
	}
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil {
		return false, err
	}
	// Cleanup is a control action, not a new model start. A current editor can
	// settle an exact owned child after Execute/model access is revoked.
	if a.deps.AuthorizeExecutionControl != nil {
		if err = a.deps.AuthorizeExecutionControl(ctx, req.Actor); err != nil {
			return false, err
		}
	} else if !req.Actor.Local || a.deps.ManagedExecution {
		return false, management.ErrExecutionAuthority
	}
	if req.OperationID == "" || req.ChildID == "" {
		if req.OperationID != "" || req.ChildID != "" || !claimOnly {
			return false, product.ErrProjectRunConflict
		}
		// No recorded child can be guessed from a queue ID. A concurrent
		// dispatch turn is serialized by the scheduler tick; if an operation
		// exists, wait for its identity to become visible instead of releasing.
		for _, op := range run.Operations {
			if op.TaskID == req.TaskID && op.Status != "cancelled" {
				id, lookupErr := a.deps.Product.SwarmQueueIDForOperation(req.ProjectID, req.RunID, op.ID)
				if lookupErr == nil && id == req.QueueID {
					return false, nil
				}
			}
		}
		return true, nil // releaseQueueLease still checks the private lease.
	}
	opFound := false
	opStatus := ""
	for _, op := range run.Operations {
		if op.ID == req.OperationID && op.ChildID == req.ChildID && op.TaskID == req.TaskID {
			opFound = true
			opStatus = op.Status
			break
		}
	}
	if !opFound {
		return false, product.ErrProjectRunConflict
	}
	if run.Input.Route.ExecutionLocation == "cloud-microvm" {
		if a.deps.CloudTasks == nil {
			return false, nil
		}
		current, found := a.cloudRecord(req.ProjectID, req.RunID, req.OperationID, req.ChildID)
		if !found {
			return false, nil // unknown Create/transfer is never treated as absent
		}
		if current.State != e2bbe.Absent {
			current, err = a.deps.CloudTasks.Runtime.Controller.Cancel(ctx, req.ChildID, current.Revision)
			if err != nil {
				return false, err // the controller may have saved the intent
			}
		}
		if current.State != e2bbe.Absent || current.ResourcesHeld {
			return false, nil
		}
		// An already captured/retired result can be observed for review after
		// the new run starts. Never relabel its natural exit as a cancellation.
		if current.CancelRequested && current.Capture.ArchiveSHA256 == "" && (opStatus == "running" || opStatus == "dispatching" || opStatus == "reserved" || opStatus == "unknown") {
			if _, err = a.deps.Product.RecordProjectSwarmMicroVMCancelled(ctx, req.ProjectID, req.RunID, req.OperationID, current); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if !sessionRestarts.acquire(req.ChildID) {
		return false, nil
	}
	defer sessionRestarts.release(req.ChildID)
	row, err := a.deps.Store.Get(req.ChildID)
	if err != nil {
		return false, nil // an unconfirmed late local launch can still appear
	}
	if row.ID != req.ChildID || row.ProjectID != req.ProjectID || row.ProjectRunID != req.RunID || row.ProjectRunOperationID != req.OperationID || row.SandboxID == "" {
		return false, product.ErrProjectRunConflict
	}
	if err = a.deps.Backend.Destroy(ctx, row.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
		return false, err
	}
	if _, err = a.deps.Store.Update(req.ChildID, func(current *registry.Session) error {
		if current.ProjectID != req.ProjectID || current.ProjectRunID != req.RunID || current.ProjectRunOperationID != req.OperationID || current.SandboxID != row.SandboxID {
			return product.ErrProjectRunConflict
		}
		current.Status = registry.StatusExited
		if current.ExitReason == "" {
			current.ExitReason = "superseded by accepted context amendment"
		}
		current.Endpoint, current.AttachToken = "", ""
		return nil
	}); err != nil {
		return false, err
	}
	// The product caller now asks releaseQueueLease to prove exact absence and
	// settled launch before it drops the scheduler charge.
	return true, nil
}
