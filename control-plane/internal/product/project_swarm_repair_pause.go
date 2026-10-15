package product

import "github.com/lcfrai/luci/control-plane/internal/project"

// Restoring an operator pause is only a queue-state change. It must not turn
// a missing or uncertain checkpoint into a fresh repair attempt. The existing
// coordinator and scheduler still recheck authority before any new effect.
func (s *Server) validateRepairQueueResume(p project.Project, item swarmQueueRecord) error {
	if item.RepairObligationID == "" || item.PausedFromState == "" {
		return ErrProjectRunConflict
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, item.RunID)
	if err != nil {
		return err
	}
	if run.Swarm == nil || run.Input.OrchestrationContract == nil || run.Swarm.State != "ready" || run.State == "cancelled" || run.State == "blocked" || run.State == "complete" || queueInputDigest(run) != item.InputDigest {
		return ErrProjectRunConflict
	}
	if err = validateSwarmTaskRoute(run); err != nil {
		return err
	}
	if err = s.validateProjectRunApprovals(p.ID, run.Input.Approvals); err != nil {
		return err
	}
	if err = s.contractCurrentForRun(p, run); err != nil {
		return err
	}
	task := swarmTask(&run, item.TaskID)
	source := swarmOperation(&run, item.RepairSourceOperationID)
	if task == nil || source == nil || source.TaskID != task.ID {
		return ErrProjectRunConflict
	}
	matching := false
	boundRepairID := ""
	for _, obligation := range run.Swarm.ReviewObligations {
		if obligation.ID != item.RepairObligationID {
			continue
		}
		if obligation.TaskID != task.ID || obligation.State != "repair-required" || obligation.CandidateOperationID != source.ID || obligation.CandidateChildID != source.ChildID {
			return ErrProjectRunConflict
		}
		matching = true
		boundRepairID = obligation.RepairOperationID
		break
	}
	if !matching {
		return ErrProjectRunConflict
	}
	_, repair, found, err := s.projectRuns.FindOperationByKey(p.ID, p.Path, run.ID, schedulerReserveKey(item), "session")
	if err != nil {
		return err
	}
	if found && (repair.TaskID != task.ID || repair.Checkpoint == nil || repair.Checkpoint.SourceOperationID != source.ID || repair.Checkpoint.SourceChildID != source.ChildID || repair.Checkpoint.ReviewDigest != item.RepairReviewDigest || item.OperationID != "" && item.OperationID != repair.ID || item.ChildID != "" && item.ChildID != repair.ChildID) {
		return ErrProjectRunConflict
	}
	if found && (boundRepairID != "" && boundRepairID != repair.ID || task.OperationID != repair.ID) || !found && (boundRepairID != "" || task.OperationID != source.ID) {
		return ErrProjectRunConflict
	}
	if item.PausedFromState == "paused" {
		if item.PausedFromCode != "repair_checkpoint_pending" || item.OperationID != "" {
			return ErrProjectRunConflict
		}
	} else if !found || item.OperationID != repair.ID || item.ChildID != repair.ChildID {
		return ErrProjectRunConflict
	}
	return nil
}
