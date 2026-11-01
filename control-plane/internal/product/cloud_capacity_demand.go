package product

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

type CloudHostDemand struct {
	QueueID string
	ChildID string
}

// Capacity follows work already admitted by the ONE task scheduler. Queued
// wishes, blocked dependencies, other actors and uncertain originals do not
// become infrastructure demand. No provider I/O occurs under the queue lock.
func (s *Server) CloudHostDemand(ctx context.Context, actor management.ExecutionActorRef, harness, parent, digest string) ([]CloudHostDemand, error) {
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	items := []swarmQueueRecord{}
	if err == nil && !ledger.Paused {
		for _, item := range ledger.Items {
			if item.Actor == actor && (item.State == "reserved" || item.State == "dispatching" || item.State == "queued" && item.ReasonCode == "cloud_host_capacity") && item.OperationID != "" && item.ChildID != "" {
				items = append(items, item)
			}
		}
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return nil, err
	}
	runs := map[string]ProjectRun{}
	result := []CloudHostDemand{}
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		run, ok := runs[item.RunID]
		if !ok {
			run, err = s.GetProjectSwarmRun(actor.ProjectID, item.RunID)
			if err != nil {
				return nil, err
			}
			p, e := s.resolveProject(actor.ProjectID)
			if e != nil {
				return nil, e
			}
			if s.OrchestratorExecutionGate(actor.ProjectID) != nil || s.ValidateImplementationContext(p.ID, run.WorktreeID, p.Path, &run.Input.Implementation) != nil || s.validateProjectRunApprovals(actor.ProjectID, run.Input.Approvals) != nil || s.contractCurrentForRun(p, run) != nil {
				continue
			}
			runs[item.RunID] = run
		}
		if run.State == "cancelled" || run.Swarm == nil || run.Swarm.State != "ready" || run.Input.Route.ExecutionLocation != "cloud-microvm" || run.Input.Route.Harness != harness || item.InputDigest != queueInputDigest(run) {
			continue
		}
		allowed := len(run.Input.Route.CloudHostIDs) == 0
		for _, id := range run.Input.Route.CloudHostIDs {
			allowed = allowed || id == parent
		}
		if !allowed || len(run.Input.Route.CloudHostIDs) > 0 && run.Input.Route.CloudPoolDigest != digest {
			continue
		}
		if s.CheckPortfolioFocusAdmission(ctx, item.ID) != nil {
			continue
		}
		for _, op := range run.Operations {
			if op.ID == item.OperationID && op.ChildID == item.ChildID && op.TaskID == item.TaskID && op.Allocation == "ready" && (op.Status == "reserved" || item.State != "queued" && op.Status == "dispatching") {
				result = append(result, CloudHostDemand{item.ID, item.ChildID})
				break
			}
		}
	}
	return result, nil
}
