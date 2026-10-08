package product

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// These identities come from private operation receipts, never from a path or
// arbitrary process identifier supplied by the caller. UpdatedAt fences the
// absence observation behind the retained dispatch claim.
type ProjectSwarmRecoveryRequest struct {
	ProjectID   string
	RunID       string
	OperationID string
	ChildID     string
	QueueID     string
	UpdatedAt   string
}

// The execution service proves a guarded pre-backend phase as well as current
// registry/backend absence. This receipt grants retirement, never execution.
type ProjectSwarmRecoveryProof struct {
	ChildID             string `json:"childId"`
	QueueID             string `json:"queueId,omitempty"`
	LeaseID             string `json:"leaseId"`
	Backend             string `json:"backend"`
	ObservedAt          string `json:"observedAt"`
	InventoryObservedAt string `json:"inventoryObservedAt"`
	FinalizedAt         string `json:"finalizedAt,omitempty"`
}

func (s *Server) SetProjectSwarmRecovery(prove func(context.Context, ProjectSwarmRecoveryRequest) (ProjectSwarmRecoveryProof, error), finalize func(context.Context, ProjectSwarmRecoveryRequest, ProjectSwarmRecoveryProof) error) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	s.projectSwarmRecoveryProve = prove
	s.projectSwarmRecoveryFinalize = finalize
}
func validSwarmRecoveryProof(proof ProjectSwarmRecoveryProof, childID string) bool {
	if proof.ChildID != childID || !prototypeReleaseID.MatchString(proof.ChildID) || proof.QueueID != "" && (!strings.HasPrefix(proof.QueueID, "queue-") || !prototypeReleaseID.MatchString(proof.QueueID)) || proof.LeaseID == "" || len(proof.LeaseID) > 200 || strings.ContainsAny(proof.LeaseID, "\x00\r\n") || proof.Backend == "" || len(proof.Backend) > 64 || !prototypeReleaseID.MatchString(proof.Backend) {
		return false
	}
	observed, e1 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
	inventory, e2 := time.Parse(time.RFC3339Nano, proof.InventoryObservedAt)
	if e1 != nil || e2 != nil || observed.Before(inventory) {
		return false
	}
	if proof.FinalizedAt != "" {
		finalized, err := time.Parse(time.RFC3339Nano, proof.FinalizedAt)
		return err == nil && !finalized.Before(observed)
	}
	return true
}

func swarmRetirementPending(op ProjectRunOperation) bool {
	return op.AbsenceRetirement != nil && op.AbsenceRetirement.FinalizedAt == ""
}

// ProjectSwarmRetirement is read-only and deliberately does not acquire the
// dispatch mutex: recovery finalization already holds it. Callers must not use
// a receipt to waive their independent current backend/lease ownership checks.
func (s *Server) ProjectSwarmRetirement(projectID, runID, operationID, childID, queueID string) (*ProjectSwarmRecoveryProof, error) {
	run, err := s.GetProjectSwarmRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.ChildID != childID || op.TaskID == "" || op.Status != "cancelled" || op.AbsenceRetirement == nil || op.AbsenceRetirement.QueueID != queueID || !validSwarmRecoveryProof(*op.AbsenceRetirement, childID) {
		return nil, errors.New("This exact operation has no durable missing-child retirement receipt.")
	}
	proof := *op.AbsenceRetirement
	// Completion is a product acknowledgement, not part of the immutable
	// absence evidence retained by the execution service.
	proof.FinalizedAt = ""
	return &proof, nil
}

// The caller holds projectRunDispatchMu across proof, tombstone persistence,
// and resource finalization. An allocator holding admission must finish first;
// one arriving later observes cancelled and cannot claim this identity again.
func (s *Server) recoverMissingSwarmChildLocked(ctx context.Context, p project.Project, run ProjectRun, operationID, childID string) (ProjectRun, error) {
	op := swarmOperation(&run, operationID)
	if op == nil || op.TaskID == "" || op.ChildID != childID || op.Allocation != "ready" {
		return run, errors.New("Select the exact retained operation and child before recovering a missing launch.")
	}
	task := swarmTask(&run, op.TaskID)
	if task == nil || task.OperationID != op.ID || task.Integration != nil {
		return run, errors.New("Resolve this task's retained integration before retiring its launch.")
	}
	if op.Status != "dispatching" && op.Status != "unknown" && !(op.Status == "cancelled" && op.AbsenceRetirement != nil) {
		return run, errors.New("Only an unconfirmed launch without a registered child can use this recovery action. Stop and observe any existing session first.")
	}
	if op.AbsenceRetirement != nil && op.AbsenceRetirement.FinalizedAt != "" {
		return run, nil
	}
	if s.projectSwarmRecoveryProve == nil || s.projectSwarmRecoveryFinalize == nil {
		return run, errors.New("The execution service cannot currently prove this launch absent. Its identity and capacity remain reserved.")
	}
	queueID, err := s.SwarmQueueIDForOperation(p.ID, run.ID, op.ID)
	if err != nil {
		return run, err
	}
	req := ProjectSwarmRecoveryRequest{ProjectID: p.ID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, QueueID: queueID, UpdatedAt: op.UpdatedAt}
	var proof ProjectSwarmRecoveryProof
	if op.AbsenceRetirement != nil {
		proof = *op.AbsenceRetirement
	} else {
		proof, err = s.projectSwarmRecoveryProve(ctx, req)
		if err != nil {
			return run, err
		}
		now := time.Now().UTC()
		observed, e1 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
		inventory, e2 := time.Parse(time.RFC3339Nano, proof.InventoryObservedAt)
		claimed, e3 := time.Parse(time.RFC3339Nano, req.UpdatedAt)
		if !validSwarmRecoveryProof(proof, op.ChildID) || proof.FinalizedAt != "" || proof.QueueID != queueID || e1 != nil || e2 != nil || e3 != nil || inventory.Before(claimed) || observed.After(now) || now.Sub(observed) > 5*time.Second || now.Sub(inventory) > 5*time.Second {
			return run, errors.New("The missing-child proof is stale or does not match this operation. Nothing was retired or released.")
		}
		retainedOperationID, retainedChildID := op.ID, op.ChildID
		run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
			target := swarmOperation(current, retainedOperationID)
			if target == nil || target.ChildID != retainedChildID || target.UpdatedAt != req.UpdatedAt || (target.Status != "dispatching" && target.Status != "unknown") {
				return ErrProjectRunConflict
			}
			target.AbsenceRetirement = &proof
			target.Status = "cancelled"
			target.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			target.Message = "The operator retired this exact launch after current registry/backend absence and a guarded pre-backend phase were proved. Workspace files and frozen context were retained."
			current.Reason = target.Message
			return nil
		})
		if err != nil {
			return run, err
		}
	}
	// The cancellation is durable before any resource lease can be freed. A
	// finalization error retains the tombstone and capacity for an explicit retry.
	if err = s.projectSwarmRecoveryFinalize(ctx, req, proof); err != nil {
		return run, err
	}
	if queueID != "" {
		if err = s.schedulerUpdate(queueID, func(item *swarmQueueRecord) error {
			if item.ProjectID != p.ID || item.RunID != run.ID || item.OperationID != "" && item.OperationID != operationID || item.ChildID != "" && item.ChildID != childID {
				return ErrProjectRunConflict
			}
			item.OperationID = operationID
			item.ChildID = childID
			item.MemoryMiB = 0
			item.CPUMillis = 0
			schedulerReason(item, "cancelled", "missing_child_retired", "The exact missing launch was retired after absence was proved. Retained output remains available; a fresh attempt requires new consent.")
			return nil
		}); err != nil {
			return run, err
		}
	}
	// A crash after the execution tombstone or queue update can repeat those
	// idempotent steps, but cannot authorize a new attempt until this final
	// acknowledgement is saved. Keep the original absence proof permanently.
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
		target := swarmOperation(current, operationID)
		if target == nil || target.ChildID != childID || target.Status != "cancelled" || target.AbsenceRetirement == nil || *target.AbsenceRetirement != proof {
			return ErrProjectRunConflict
		}
		target.AbsenceRetirement.FinalizedAt = time.Now().UTC().Format(time.RFC3339Nano)
		target.UpdatedAt = target.AbsenceRetirement.FinalizedAt
		target.Message = "The exact missing launch was retired and its execution reservation released. Files and frozen context remain retained; a fresh attempt requires new consent."
		current.Reason = target.Message
		return nil
	})
	if err != nil {
		return run, err
	}
	s.WakeSwarmScheduler()
	return run, nil
}
