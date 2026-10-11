package product

import (
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/ids"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Swarms coordinate existing reviewed sessions. A task is released only by an
// integration receipt, never a task-board checkbox or successful process exit.
type ProjectSwarm struct {
	State       string             `json:"state"`
	MaxParallel int                `json:"maxParallel"`
	Tasks       []ProjectSwarmTask `json:"tasks"`
}
type ProjectSwarmTask struct {
	ID          string                   `json:"id"`
	Title       string                   `json:"title"`
	DependsOn   []string                 `json:"dependsOn"`
	WritePaths  []string                 `json:"writePaths"`
	Acceptance  string                   `json:"acceptance"`
	OperationID string                   `json:"operationId,omitempty"`
	Integration *ProjectSwarmIntegration `json:"integration,omitempty"`
}
type ProjectSwarmIntegration struct {
	HandoffDigest string `json:"handoffDigest,omitempty"`
	State         string `json:"state"`
	Commit        string `json:"commit,omitempty"`
	SourceCommit  string `json:"sourceCommit"`
	TargetCommit  string `json:"targetCommit"`
	ReviewDigest  string `json:"reviewDigest"`
	IntegratedAt  string `json:"integratedAt,omitempty"`
}

func swarmTask(run *ProjectRun, id string) *ProjectSwarmTask {
	if run.Swarm != nil {
		for i := range run.Swarm.Tasks {
			if run.Swarm.Tasks[i].ID == id {
				return &run.Swarm.Tasks[i]
			}
		}
	}
	return nil
}
func swarmOperation(run *ProjectRun, id string) *ProjectRunOperation {
	for i := range run.Operations {
		if run.Operations[i].ID == id {
			return &run.Operations[i]
		}
	}
	return nil
}
func validateProjectSwarm(run ProjectRun) error {
	if run.Swarm == nil {
		for _, op := range run.Operations {
			if op.Handoff != nil || op.DependencyContext != nil || op.Checkpoint != nil || op.UnclaimedRetirement || op.TaskID != "" || op.WorktreeID != "" || op.Implementation != nil || op.Allocation != "" || op.AbsenceRetirement != nil || op.LaunchRecovery != nil || op.LaunchRecoveryProof != nil || op.Recovery != nil || op.Status == "review-required" {
				return ErrProjectRunInvalid
			}
		}
		return nil
	}
	swarm := run.Swarm
	if swarm.MaxParallel < 1 || swarm.MaxParallel > 4 || (swarm.State != "ready" && swarm.State != "paused" && swarm.State != "cancelled") || len(swarm.Tasks) == 0 || len(swarm.Tasks) != len(run.Input.Implementation.TaskIDs) {
		return ErrProjectRunInvalid
	}
	seen := map[string]bool{}
	planTasks := []ProjectTask{}
	for _, task := range swarm.Tasks {
		planTasks = append(planTasks, ProjectTask{ID: task.ID, Title: task.Title, Status: "todo", DependsOn: task.DependsOn, WritePaths: task.WritePaths, Acceptance: task.Acceptance})
		if !designIDPattern.MatchString(task.ID) || seen[task.ID] || len(task.Title) == 0 || len(task.Title) > 2000 || len(task.Acceptance) > 8000 {
			return ErrProjectRunInvalid
		}
		seen[task.ID] = true
		selected := false
		for _, id := range run.Input.Implementation.TaskIDs {
			selected = selected || id == task.ID
		}
		if !selected {
			return ErrProjectRunInvalid
		}
		if task.OperationID != "" {
			op := swarmOperation(&run, task.OperationID)
			if op == nil || op.TaskID != task.ID {
				return ErrProjectRunInvalid
			}
		}
		if in := task.Integration; in != nil {
			if (in.State != "integrating" && in.State != "integrated" && in.State != "unknown") || !deliverySHA.MatchString(in.SourceCommit) || !deliverySHA.MatchString(in.TargetCommit) || !hexDigest(in.ReviewDigest) || in.Commit != "" && !deliverySHA.MatchString(in.Commit) {
				return ErrProjectRunInvalid
			}
			if in.State == "integrated" {
				if in.Commit == "" {
					return ErrProjectRunInvalid
				}
				if _, err := time.Parse(time.RFC3339Nano, in.IntegratedAt); err != nil {
					return ErrProjectRunInvalid
				}
			}
		}
	}
	if validatePlan(ProjectPlan{Tasks: planTasks}) != nil {
		return ErrProjectRunInvalid
	}
	for _, task := range swarm.Tasks {
		for _, dep := range task.DependsOn {
			if !seen[dep] || dep == task.ID {
				return ErrProjectRunInvalid
			}
		}
	}
	for _, op := range run.Operations {
		if !validTaskHandoffReference(op) {
			return ErrProjectRunInvalid
		}
		if !validSwarmCheckpoint(run, op) {
			return ErrProjectRunInvalid
		}
		if op.UnclaimedRetirement && (op.Status != "cancelled" || op.LaunchRecovery != nil || op.LaunchRecoveryProof != nil || op.AbsenceRetirement != nil || op.Integration != nil) {
			return ErrProjectRunInvalid
		}
		if !validSwarmOperationRecovery(run, op) {
			return ErrProjectRunInvalid
		}
		if op.AbsenceRetirement != nil && (op.Status != "cancelled" || op.Allocation != "ready" || !validSwarmRecoveryProof(*op.AbsenceRetirement, op.ChildID)) {
			return ErrProjectRunInvalid
		}
		if !seen[op.TaskID] || !prototypeReleaseID.MatchString(op.WorktreeID) || !deliverySHA.MatchString(op.BaseCommit) || (op.Allocation != "allocating" && op.Allocation != "ready" && op.Allocation != "unknown" && op.Allocation != "abandoned") {
			return ErrProjectRunInvalid
		}
		if op.Allocation == "ready" && (op.Implementation == nil || op.Implementation.WorktreeID != op.WorktreeID || len(op.Implementation.TaskIDs) != 1 || op.Implementation.TaskIDs[0] != op.TaskID) {
			return ErrProjectRunInvalid
		}
		if op.Allocation == "ready" && op.DependencyContext != nil && op.Implementation.DependencyContextDigest != op.DependencyContext.Digest {
			return ErrProjectRunInvalid
		}
		if receipt := taskHandoffIntegration(run, op); receipt != nil && receipt.HandoffDigest != "" && (op.Handoff == nil || receipt.HandoffDigest != op.Handoff.Digest) {
			return ErrProjectRunInvalid
		}
	}
	return nil
}
func (s *ProjectRunStore) mutateSwarm(projectID, path, id string, revision int, change func(*ProjectRun) error) (ProjectRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(projectID, path, id, revision, func(row *storedProjectRun) (bool, error) {
		if err := change(&row.Run); err != nil {
			return false, err
		}
		return true, nil
	})
}
func swarmSlotPending(op ProjectRunOperation) bool {
	return swarmRetirementPending(op) || op.Status == "reserved" || op.Status == "dispatching" || op.Status == "running" || op.Status == "unknown" || op.Allocation == "allocating" || op.Allocation == "unknown"
}
func swarmScopesOverlap(a, b []string) bool {
	return len(a) == 0 || len(b) == 0 || len(projectTaskOverlappingPaths(a, b)) > 0
}

func swarmTaskReady(run ProjectRun, task ProjectSwarmTask) error {
	if run.Swarm == nil || run.Swarm.State != "ready" || run.State == "cancelled" || run.State == "complete" {
		return errors.New("Resume this swarm before reserving more work.")
	}
	if task.Integration != nil {
		return errors.New("This task has an integration receipt or an unresolved integration attempt. Inspect it before continuing.")
	}
	for _, dep := range task.DependsOn {
		t := swarmTask(&run, dep)
		if t == nil || t.Integration == nil || t.Integration.State != "integrated" {
			return errors.New("Integrate this task's dependencies before starting it.")
		}
	}
	pending := 0
	for _, op := range run.Operations {
		if !swarmSlotPending(op) {
			continue
		}
		pending++
		if op.TaskID == task.ID {
			return errors.New("This task already has an unresolved attempt. Observe its existing session instead of replaying it.")
		}
		other := swarmTask(&run, op.TaskID)
		if other == nil || swarmScopesOverlap(task.WritePaths, other.WritePaths) {
			return errors.New("A running task has overlapping or unspecified write scope. Wait for it to finish before dispatching this task.")
		}
	}
	if swarmSubscription(run) && pending >= 1 {
		return errors.New("This saved ChatGPT subscription permits one active coding session. Observe its existing child before starting another task.")
	}
	if pending >= run.Swarm.MaxParallel {
		return errors.New("All reserved agent slots are occupied. Observe existing work before starting another task.")
	}
	return nil
}

// Reservation is durable before worktree allocation. Neither a read nor a
// duplicate POST is permission to replay an interrupted allocation.
func (s *ProjectRunStore) ReserveSwarmTask(projectID, path, id string, revision int, key, taskID, base string, worktree func(string) string) (ProjectRun, ProjectRunOperation, bool, error) {
	return s.reserveSwarmTask(projectID, path, id, revision, key, taskID, base, worktree, nil)
}

func (s *ProjectRunStore) reserveSwarmTask(projectID, path, id string, revision int, key, taskID, base string, worktree func(string) string, checkpoint *ProjectSwarmCheckpoint) (ProjectRun, ProjectRunOperation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validRunKey(key) || !deliverySHA.MatchString(base) {
		return ProjectRun{}, ProjectRunOperation{}, false, ErrProjectRunInvalid
	}
	scope, err := s.projectScope(projectID, path)
	if err != nil {
		return ProjectRun{}, ProjectRunOperation{}, false, err
	}
	row, err := s.read(projectID, scope, id)
	if err != nil {
		return ProjectRun{}, ProjectRunOperation{}, false, err
	}
	if prior, ok := row.OperationKeys[reviewDigest(key)]; ok {
		op := swarmOperation(&row.Run, prior)
		if op == nil || op.TaskID != taskID || !sameSwarmCheckpointSelection(op.Checkpoint, checkpoint) {
			return ProjectRun{}, ProjectRunOperation{}, false, ErrProjectRunKeyConflict
		}
		return row.Run, *op, false, nil
	}
	var op ProjectRunOperation
	run, err := s.mutate(projectID, path, id, revision, func(row *storedProjectRun) (bool, error) {
		task := swarmTask(&row.Run, taskID)
		if task == nil {
			return false, ErrProjectRunNotFound
		}
		if err := swarmTaskReady(row.Run, *task); err != nil {
			return false, err
		}
		if len(row.Run.Operations) >= projectRunOperationLimit {
			return false, ErrProjectRunLimit
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		op = ProjectRunOperation{ID: newID("op-"), Kind: "session", ChildID: ids.New("s"), Attempt: len(row.Run.Operations) + 1, Status: "reserved", Allocation: "allocating", TaskID: taskID, BaseCommit: base, CreatedAt: now, UpdatedAt: now}
		if checkpoint != nil {
			saved := *checkpoint
			op.Checkpoint = &saved
		}
		op.WorktreeID = worktree(op.ID)
		op.DependencyContext, err = s.freezeTaskDependencies(row.Run, op)
		if err != nil {
			return false, err
		}
		op.RequestDigest = implementationDigest(op)
		row.Run.Operations = append(row.Run.Operations, op)
		row.OperationKeys[reviewDigest(key)] = op.ID
		task.OperationID = op.ID
		row.Run.Reason = "An isolated task slot is reserved; allocation has not yet been confirmed."
		return true, nil
	})
	return run, op, err == nil, err
}
func (s *ProjectRunStore) CompleteSwarmAllocation(projectID, path, id string, revision int, opID string, binding *registry.ImplementationContext, candidate registry.Session, allocationErr error) (ProjectRun, error) {
	return s.mutateSwarm(projectID, path, id, revision, func(run *ProjectRun) error {
		op := swarmOperation(run, opID)
		if op == nil || op.Allocation != "allocating" {
			return ErrProjectRunConflict
		}
		if allocationErr != nil {
			op.Allocation = "unknown"
			op.Status = "unknown"
			op.Message = "Worktree allocation did not finish with a confirmed receipt. Inspect the retained task worktree; do not automatically replay allocation."
			return nil
		}
		op.Allocation = "ready"
		op.Implementation = binding
		op.RequestDigest = ProjectRunSessionRequestDigest(candidate)
		op.Message = "Isolated checkout and task-specific reviewed context are reserved. Start the named session when ready."
		return nil
	})
}
