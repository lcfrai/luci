package product

import (
	"context"
	"sort"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func (s *Server) coordinatorReusableTask(ctx context.Context, p project.Project, id, inputDigest, head, excludeRun string, prior []ProjectRun) (ProjectSwarmReuseReference, bool) {
	for _, old := range prior {
		if old.ID == excludeRun {
			continue
		}
		task := swarmTask(&old, id)
		if task == nil || task.Integration == nil || task.Integration.State != "integrated" || task.TaskInputDigest != inputDigest || !hexDigest(task.Integration.ReviewDigest) {
			continue
		}
		op := swarmOperation(&old, task.OperationID)
		if op == nil || op.TaskID != id || op.Allocation != "ready" || op.Status != "succeeded" && op.Status != "review-required" && op.Status != "failed" {
			continue
		}
		if err := s.validateSwarmReuseSource(ctx, p, *task, head); err != nil {
			continue
		}
		if _, err := s.projectRuns.taskHandoffView(old, *op); err != nil {
			continue
		}
		return ProjectSwarmReuseReference{RunID: old.ID, TaskID: id, OperationID: task.OperationID, IntegrationDigest: implementationDigest(task.Integration)}, true
	}
	return ProjectSwarmReuseReference{}, false
}

// An accepted change includes only prerequisites lacking an exact reusable
// integration. If one prerequisite must be rebuilt, its dependents also need
// fresh work; a previous integration cannot stand for the new dependency.
func (s *Server) coordinatorExpandAffectedPrerequisites(ctx context.Context, p project.Project, state journeyState, selectedIDs []string) ([]string, error) {
	selected := map[string]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	tasks := map[string]ProjectTask{}
	for _, task := range currentPlan(state, p.ID).Tasks {
		tasks[task.ID] = task
	}
	digests, err := swarmTaskInputDigests(state, p.ID)
	if err != nil {
		return nil, err
	}
	_, head, dirty, err := s.checkSource(ctx, p.Path)
	if err != nil || dirty || !deliverySHA.MatchString(head) {
		return nil, errCoordinatorSourceBusy
	}
	prior, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil {
		return nil, err
	}
	sort.Slice(prior, func(i, j int) bool {
		if prior[i].CreatedAt == prior[j].CreatedAt {
			return prior[i].ID > prior[j].ID
		}
		return prior[i].CreatedAt > prior[j].CreatedAt
	})
	visited := map[string]bool{}
	var walk func(string) (bool, error)
	walk = func(id string) (bool, error) {
		if visited[id] {
			return selected[id], nil
		}
		visited[id] = true
		task, ok := tasks[id]
		if !ok {
			return false, ErrProjectRunInvalid
		}
		changed := selected[id]
		for _, dep := range task.DependsOn {
			newWork, err := walk(dep)
			if err != nil {
				return false, err
			}
			changed = changed || newWork
		}
		if !changed {
			_, reusable := s.coordinatorReusableTask(ctx, p, id, digests[id], head, "", prior)
			changed = !reusable
		}
		if changed {
			selected[id] = true
		}
		return changed, nil
	}
	for _, id := range selectedIDs {
		if _, err := walk(id); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(selected))
	for id := range selected {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// The coordinator may select an unchanged prerequisite only from an existing
// reviewed integration. The ordinary preparation endpoint revalidates these
// exact references under exclusive source admission; no task completion is
// inferred from plan status or a worker's claim.
func (s *Server) coordinatorReuseReferences(ctx context.Context, p project.Project, run ProjectRun) ([]ProjectSwarmReuseReference, error) {
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, id := range run.Input.Implementation.TaskIDs {
		selected[id] = true
	}
	tasks := map[string]ProjectTask{}
	for _, task := range currentPlan(state, p.ID).Tasks {
		tasks[task.ID] = task
	}
	wanted := map[string]bool{}
	visited := map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if visited[id] {
			return nil
		}
		visited[id] = true
		task, ok := tasks[id]
		if !ok {
			return ErrProjectRunInvalid
		}
		for _, dep := range task.DependsOn {
			if !selected[dep] {
				wanted[dep] = true
			}
			if err := walk(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for id := range selected {
		if err := walk(id); err != nil {
			return nil, err
		}
	}
	if len(wanted) == 0 {
		return []ProjectSwarmReuseReference{}, nil
	}
	digests, err := swarmTaskInputDigests(state, p.ID)
	if err != nil {
		return nil, err
	}
	_, head, dirty, err := s.checkSource(ctx, p.Path)
	if err != nil || dirty || head != run.Input.Implementation.SourceCommit {
		return nil, errSwarmReuseProof
	}
	prior, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil {
		return nil, err
	}
	sort.Slice(prior, func(i, j int) bool {
		if prior[i].CreatedAt == prior[j].CreatedAt {
			return prior[i].ID > prior[j].ID
		}
		return prior[i].CreatedAt > prior[j].CreatedAt
	})
	ids := make([]string, 0, len(wanted))
	for id := range wanted {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	refs := make([]ProjectSwarmReuseReference, 0, len(ids))
	for _, id := range ids {
		ref, found := s.coordinatorReusableTask(ctx, p, id, digests[id], head, run.ID, prior)
		if !found {
			return nil, errSwarmReuseProof
		}
		refs = append(refs, ref)
	}
	// This also checks the exact finished operation and durable handoff object.
	if _, err := s.resolveSwarmReuse(ctx, p, state, run.Input.Implementation.TaskIDs, head, refs); err != nil {
		return nil, err
	}
	return refs, nil
}
