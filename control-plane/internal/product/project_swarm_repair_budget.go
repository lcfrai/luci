package product

import (
	"errors"
	"reflect"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

var errAutomaticRepairLimit = errors.New("The approved automatic repair limit is exhausted. Review a changed plan and approve a new run before another repair.")

// Missing legacy fields keep the original limit without changing retained JSON
// or its approved digest. In particular, explicit zero is never a default.
func effectiveAutomaticRepairLimit(value *int) (int, error) {
	if value == nil {
		return maxSwarmReviewRepairs, nil
	}
	if *value < 0 || *value > maxSwarmReviewRepairs {
		return 0, ErrProjectRunInvalid
	}
	return *value, nil
}

func swarmAutomaticRepairLimit(run ProjectRun) (int, error) {
	if run.Input.OrchestrationContract == nil {
		return maxSwarmReviewRepairs, nil
	}
	return effectiveAutomaticRepairLimit(run.Input.OrchestrationContract.Input.Controls.AutomaticRepairLimit)
}

// Only a new reservation consumes a repair. Findings, reviewer generations and
// the global operation Attempt are not repair counts. Failed/unknown/cancelled
// reservations remain consumed; replay cannot refund their immutable identity.
func swarmRepairsConsumed(run ProjectRun, taskID string) int {
	seen := map[string]bool{}
	if run.Swarm != nil {
		for _, finding := range run.Swarm.ReviewObligations {
			if finding.TaskID == taskID && finding.RepairOperationID != "" {
				seen[finding.RepairOperationID] = true
			}
		}
	}
	return len(seen)
}

func swarmRepairBudgetForCheckpoint(run ProjectRun, sourceID string) error {
	source := swarmOperation(&run, sourceID)
	if source == nil || run.Swarm == nil {
		return ErrProjectRunConflict
	}
	limit, err := swarmAutomaticRepairLimit(run)
	if err != nil {
		return err
	}
	var latest *ProjectSwarmReviewObligation
	ordinal := 0
	for i := range run.Swarm.ReviewObligations {
		finding := &run.Swarm.ReviewObligations[i]
		if finding.TaskID == source.TaskID && finding.State == "repair-required" {
			ordinal++
			latest = finding
		}
	}
	if latest == nil {
		return nil // ordinary stopped-attempt continuation, not a review repair
	}
	if latest.CandidateOperationID != source.ID || latest.CandidateChildID != source.ChildID || latest.RepairOperationID != "" {
		return ErrProjectRunConflict
	}
	if ordinal > limit || swarmRepairsConsumed(run, source.TaskID) >= limit {
		return errAutomaticRepairLimit
	}
	return nil
}

// Derive the operation's repair identity from the run's immutable finding and
// checkpoint, never from a caller's queue flag or an asserted retry ordinal.
func swarmRepairBudgetForOperation(run ProjectRun, operationID string) error {
	op := swarmOperation(&run, operationID)
	if op == nil {
		return ErrProjectRunConflict
	}
	if run.Swarm == nil {
		return nil // serial reviewed runs have no independent-review repair lineage
	}
	limit, err := swarmAutomaticRepairLimit(run)
	if err != nil {
		return err
	}
	ordinal, matches, repairOrdinal := 0, 0, 0
	var latest *ProjectSwarmReviewObligation
	for i := range run.Swarm.ReviewObligations {
		finding := &run.Swarm.ReviewObligations[i]
		if finding.RepairOperationID == op.ID && (finding.TaskID != op.TaskID || finding.State != "repair-required") {
			return ErrProjectRunConflict
		}
		if finding.TaskID != op.TaskID || finding.State != "repair-required" {
			continue
		}
		ordinal++
		latest = finding
		if finding.RepairOperationID != op.ID {
			continue
		}
		matches++
		repairOrdinal = ordinal
		if op.Checkpoint == nil || op.Checkpoint.SourceOperationID != finding.CandidateOperationID || op.Checkpoint.SourceChildID != finding.CandidateChildID || op.Checkpoint.SourceCommit != finding.CandidateCommit {
			return ErrProjectRunConflict
		}
	}
	if matches > 1 {
		return ErrProjectRunConflict
	}
	if matches == 1 {
		if repairOrdinal > limit {
			return errAutomaticRepairLimit
		}
		return nil
	}
	if latest != nil && latest.CandidateOperationID != op.ID {
		return ErrProjectRunConflict // an unlinked task retry cannot bypass the cap
	}
	return nil
}

// The caller holds s.mu.RLock through its durable reservation/dispatch intent.
// This narrow check performs no nested s.mu acquisition and no provider reads.
func (s *Server) swarmRepairPolicyCurrentLocked(p project.Project, run ProjectRun) error {
	if run.Input.OrchestrationContract == nil {
		return nil
	}
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		return err
	}
	controls := run.Input.OrchestrationContract.Input.Controls
	savedLimit, savedErr := effectiveAutomaticRepairLimit(controls.AutomaticRepairLimit)
	currentLimit, currentErr := effectiveAutomaticRepairLimit(policy.AutomaticRepairLimit)
	if savedErr != nil || currentErr != nil || savedLimit != currentLimit || policy.Revision != controls.CoordinatorPolicyRevision || reviewDigest(policy) != controls.CoordinatorPolicyDigest {
		return ErrProjectRunConflict
	}
	return nil
}

// ValidateSwarmOperationExecutionContract is consumed immediately before a new
// native effect. Historical exact-child observation/retirement does not call it.
// Re-read the operation: a caller's old run snapshot is never effect authority.
func (s *Server) ValidateSwarmOperationExecutionContract(projectID string, captured ProjectRun, operationID string) error {
	if captured.ProjectID != projectID {
		return ErrProjectRunConflict
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return err
	}
	current, err := s.projectRuns.Get(p.ID, p.Path, captured.ID)
	if err != nil || queueInputDigest(current) != queueInputDigest(captured) {
		return ErrProjectRunConflict
	}
	old, op := swarmOperation(&captured, operationID), swarmOperation(&current, operationID)
	if old == nil || op == nil || old.ChildID != op.ChildID || old.TaskID != op.TaskID || old.WorktreeID != op.WorktreeID || !reflect.DeepEqual(old.Checkpoint, op.Checkpoint) || !reflect.DeepEqual(old.Implementation, op.Implementation) || current.State == "cancelled" || current.State == "blocked" {
		return ErrProjectRunConflict
	}
	if err = s.contractCurrentForRun(p, current); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err = s.swarmRepairPolicyCurrentLocked(p, current); err != nil {
		return err
	}
	return swarmRepairBudgetForOperation(current, operationID)
}
