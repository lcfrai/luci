package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

var errTaskOutputUnsettled = errors.New("This task's output is not yet safe to capture. Confirm its exact launch has finished or complete its retained-launch recovery first.")

func taskOutputImplementationDigest(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (a *API) authorizeTaskCheckpoint(ctx context.Context, req product.ProjectSwarmRecoveryRequest) error {
	if err := a.authorizeTaskOutput(ctx, req); err != nil {
		return err
	}
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil {
		return err
	}
	actor, err := a.captureQueuedActor(ctx, req.ProjectID)
	if err != nil {
		return err
	}
	_, err = a.authorizeExecution(ctx, actor, run.Input.Route.Harness, run.Input.Route.Connection.ID != "")
	return err
}

func (a *API) authorizeTaskOutput(ctx context.Context, req product.ProjectSwarmRecoveryRequest) error {
	return a.authorizeExecutionRetirement(ctx, req.ProjectID)
}

// acquireStoppedTaskOutput is shared by report capture and reviewed checkpoint
// adoption. The caller holds the product/source guards and keeps release until
// its final copy/publication. No execution admission lock crosses backend I/O.
func (a *API) acquireStoppedTaskOutput(ctx context.Context, req product.ProjectSwarmRecoveryRequest) (func(), error) {
	return a.lockStoppedTaskOutput(ctx, req, a.authorizeTaskOutput)
}

func (a *API) lockStoppedTaskOutput(ctx context.Context, req product.ProjectSwarmRecoveryRequest, authorize func(context.Context, product.ProjectSwarmRecoveryRequest) error) (func(), error) {
	if a.execution == nil || a.deps.Product == nil || req.ChildID == "" {
		return nil, errTaskOutputUnsettled
	}
	if err := authorize(ctx, req); err != nil {
		return nil, err
	}
	if !sessionRestarts.acquire(req.ChildID) {
		return nil, errExecutionLifecycleBusy
	}
	unlock, err := a.execution.lockChildLifecycle(ctx, req.ChildID)
	if err != nil {
		sessionRestarts.release(req.ChildID)
		return nil, err
	}
	var once sync.Once
	release := func() { once.Do(func() { unlock(); sessionRestarts.release(req.ChildID) }) }
	failed := true
	defer func() {
		if failed {
			release()
		}
	}()
	if err := authorize(ctx, req); err != nil {
		return nil, err
	}
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil || run.ProjectID != req.ProjectID || run.ID != req.RunID {
		return nil, errTaskOutputUnsettled
	}
	var op *product.ProjectRunOperation
	for i := range run.Operations {
		if run.Operations[i].ID == req.OperationID {
			op = &run.Operations[i]
		}
	}
	if op == nil || op.Kind != "session" || op.TaskID == "" || op.ChildID != req.ChildID || op.WorktreeID == "" || op.Implementation == nil || op.Allocation != "ready" || req.UpdatedAt != "" && req.UpdatedAt != op.UpdatedAt {
		return nil, errTaskOutputUnsettled
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil {
		return nil, err
	}
	for _, lease := range ledger.Leases {
		if lease.Scope != a.execution.scope || lease.SessionID != req.ChildID {
			continue
		}
		if lease.ProjectID != req.ProjectID || lease.SandboxID != req.ChildID || lease.LaunchGuard != 1 || lease.State != "backend-settled" {
			return nil, errTaskOutputUnsettled
		}
	}
	// A finalized retirement is a positive launch fence, including the case
	// where an interrupted launch never published a registry row.
	retired := false
	var retirementProof product.ProjectSwarmRecoveryProof
	if op.AbsenceRetirement != nil {
		proof := *op.AbsenceRetirement
		if proof.FinalizedAt == "" || !product.ValidProjectSwarmRecoveryProof(proof, req.ChildID) || proof.QueueID != req.QueueID {
			return nil, errTaskOutputUnsettled
		}
		proof.FinalizedAt = ""
		retirementProof = proof
		for _, receipt := range ledger.Retirements {
			if receipt.Scope == a.execution.scope && retiredExecutionMatches(receipt, req) && receipt.Proof == proof {
				retired = true
			}
		}
		if !retired {
			return nil, errTaskOutputUnsettled
		}
	}
	verifiedFences := 0
	for _, fence := range ledger.Fences {
		if fence.Scope != a.execution.scope || fence.ChildID != req.ChildID {
			continue
		}
		if !retired || fence.State != "retired" || fence.ProjectID != req.ProjectID || fence.RunID != req.RunID || fence.OperationID != req.OperationID || fence.QueueID != req.QueueID || op.LaunchRecovery == nil || fence.Intent != *op.LaunchRecovery || fence.Proof != retirementProof {
			return nil, errTaskOutputUnsettled
		}
		fencer, ok := a.deps.Backend.(backend.LaunchFencer)
		if !ok {
			return nil, errTaskOutputUnsettled
		}
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := fencer.VerifyLaunchFence(bounded, fence.Intent.FenceRequest, fence.Proof.FenceProof)
		cancel()
		if err != nil {
			return nil, err
		}
		verifiedFences++
	}
	if retired && op.AbsenceRetirement.Method == product.ProjectSwarmFenceRecoveryMethod && verifiedFences != 1 {
		return nil, errTaskOutputUnsettled
	}
	if !retired || op.AbsenceRetirement.Method != product.ProjectSwarmFenceRecoveryMethod {
		row, rowErr := a.deps.Store.Get(req.ChildID)
		if retired && errors.Is(rowErr, registry.ErrNotFound) {
			// The exact guarded pre-backend retirement prohibits a later bind.
		} else if rowErr != nil || row.ID != req.ChildID || row.ProjectID != req.ProjectID || row.ProjectRunID != req.RunID || row.ProjectRunOperationID != req.OperationID || row.WorktreeID != op.WorktreeID || row.Status != registry.StatusExited || row.SandboxID != req.ChildID || !retired && row.SandboxRef == "" || product.ProjectRunSessionRequestDigest(row) != op.RequestDigest {
			return nil, errTaskOutputUnsettled
		} else {
			bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
			sandbox, backendErr := a.deps.Backend.Get(bounded, row.SandboxID)
			probeErr := bounded.Err()
			cancel()
			if probeErr != nil {
				return nil, probeErr
			}
			if backendErr != nil {
				if !unambiguousBackendNotFound(backendErr) {
					return nil, backendErr
				}
			} else if sandbox.ID != row.SandboxID || row.SandboxRef != "" && sandbox.Ref != row.SandboxRef || sandbox.State != backend.StateStopped && sandbox.State != backend.StateGone {
				return nil, errTaskOutputUnsettled
			}
			current, err := a.deps.Store.Get(row.ID)
			if err != nil || current.Status != registry.StatusExited || current.SandboxID != row.SandboxID || current.SandboxRef != row.SandboxRef || current.ProjectID != row.ProjectID || current.WorktreeID != row.WorktreeID || product.ProjectRunSessionRequestDigest(current) != op.RequestDigest {
				return nil, errTaskOutputUnsettled
			}
		}
	}
	if err := authorize(ctx, req); err != nil {
		return nil, err
	}
	failed = false
	return release, nil
}
