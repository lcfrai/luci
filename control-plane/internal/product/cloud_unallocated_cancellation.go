package product

import "context"

// Controller-only completion of an existing durable queue stop intent. This is
// not a VM/child exit fact. The supplied original-controller proof must establish
// no allocation before the private operation can become cancelled.
func (s *Server) RecordProjectSwarmUnallocatedCancellation(ctx context.Context, request SwarmSupersededRequest, confirm func(ProjectRunOperation) (bool, error)) (ProjectRun, error) {
	if confirm == nil || request.QueueID == "" || request.ProjectID == "" || request.Actor.ProjectID != request.ProjectID {
		return ProjectRun{}, ErrProjectRunConflict
	}
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	queue, err := s.ProjectSwarmQueueSnapshot(request.ProjectID)
	if err != nil {
		return ProjectRun{}, err
	}
	owned := false
	for _, item := range queue.Items {
		if item.ID == request.QueueID && item.ProjectID == request.ProjectID && item.RunID == request.RunID && item.TaskID == request.TaskID && item.OperationID == request.OperationID && item.ChildID == request.ChildID && (item.ReasonCode == "operator_stop_pending" || item.ReasonCode == "context_superseded_stop_pending") {
			owned = true
			break
		}
	}
	if !owned {
		return ProjectRun{}, ErrProjectRunConflict
	}
	p, err := s.resolveProject(request.ProjectID)
	if err != nil {
		return ProjectRun{}, err
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, request.RunID)
	if err != nil {
		return ProjectRun{}, err
	}
	op := swarmOperation(&run, request.OperationID)
	if op == nil || op.ChildID != request.ChildID || op.TaskID != request.TaskID || (op.Status != "reserved" && op.Status != "dispatching" && op.Status != "unknown" && op.Status != "cancelled") {
		return ProjectRun{}, ErrProjectRunConflict
	}
	proven, err := confirm(*op)
	if err != nil || !proven {
		return ProjectRun{}, ErrProjectRunConflict
	}
	if op.Status == "cancelled" {
		return run, nil
	}
	return s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
		selected := swarmOperation(current, request.OperationID)
		if ctx.Err() != nil || selected == nil || selected.ChildID != op.ChildID || selected.TaskID != op.TaskID || selected.Status != op.Status || selected.CreatedAt != op.CreatedAt {
			return ErrProjectRunConflict
		}
		selected.Status = "cancelled"
		selected.Message = "The original controller proved that this reserved cloud attempt never materialized. No worker exit, output or provider bill settlement is inferred."
		return nil
	})
}
