package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type ProjectSwarmReview struct {
	targetDigest string
	TaskID       string   `json:"taskId"`
	OperationID  string   `json:"operationId"`
	SourceCommit string   `json:"sourceCommit"`
	TargetCommit string   `json:"targetCommit"`
	ReviewDigest string   `json:"reviewDigest"`
	Paths        []string `json:"paths"`
	Patch        string   `json:"patch"`
	CanIntegrate bool     `json:"canIntegrate"`
	Reason       string   `json:"reason"`
}

func (s *Server) swarmReview(ctx context.Context, p project.Project, run ProjectRun, taskID string) (ProjectSwarmReview, error) {
	review := ProjectSwarmReview{TaskID: taskID, Paths: []string{}}
	task := swarmTask(&run, taskID)
	if task == nil {
		return review, ErrProjectRunNotFound
	}
	op := swarmOperation(&run, task.OperationID)
	if op == nil {
		return review, errors.New("Start this task's isolated attempt before reviewing its changes.")
	}
	review.OperationID = op.ID
	if op.Allocation != "ready" {
		return review, errors.New("The task checkout allocation has an unknown outcome. Inspect it without replaying allocation.")
	}
	if s.branchSessions == nil {
		review.Reason = "Session state is unavailable. Stop and observe the task before reviewing its changes."
		return review, nil
	}
	active, activeErr := s.swarmWorktreeActive(ctx, p.ID, op.WorktreeID)
	if activeErr != nil || active {
		review.Reason = "The task session is active or its process state is unknown. Stop and observe it before reviewing changes."
		return review, nil
	}
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		return review, err
	}
	if err = s.validateSwarmTaskGit(ctx, path); err != nil {
		return review, err
	}
	if err = s.validateSwarmGit(ctx, p.Path); err != nil {
		return review, err
	}
	source, err := s.captureGitReview(ctx, path, "")
	if err != nil {
		return review, err
	}
	target, err := s.captureGitReview(ctx, p.Path, "")
	if err != nil {
		return review, err
	}
	review.SourceCommit, review.TargetCommit = source.Head, target.Head
	review.targetDigest = target.Digest
	if source.Status != "" {
		review.Reason = "The agent checkout contains uncommitted changes. Ask the agent to inspect, verify and commit them before reviewing integration."
		return review, nil
	}
	if target.Status != "" {
		review.Reason = "The main checkout has uncommitted changes. Preserve and commit them before integration."
		return review, nil
	}
	if !deliverySHA.MatchString(source.Head) || !deliverySHA.MatchString(target.Head) {
		return review, errors.New("Both checkouts need identifiable commits before integration.")
	}
	if _, err = s.swarmGit(ctx, path, "merge-base", "--is-ancestor", op.BaseCommit, source.Head); err != nil {
		return review, errors.New("The task candidate no longer descends from its assigned starting commit.")
	}
	names, err := s.swarmGit(ctx, path, "diff", "--no-renames", "--name-only", "-z", op.BaseCommit, source.Head, "--")
	if err != nil {
		return review, err
	}
	if len(names) > 128<<10 {
		return review, errors.New("The task changes exceed the review limit. Split the work into smaller reviewed tasks.")
	}
	for _, name := range strings.Split(names, "\x00") {
		if name == "" {
			continue
		}
		if blockedProjectFile(name) || strings.EqualFold(name, "LUCI.md") || !validProjectTaskWritePath(name) {
			review.Reason = "This candidate changes protected project context, private files or unsupported paths. Keep those files out of task integration and use the reviewed amendment workflow for project context."
			return review, nil
		}
		review.Paths = append(review.Paths, name)
		if len(review.Paths) > 2000 {
			return review, errors.New("The task changes exceed the review limit.")
		}
		if len(task.WritePaths) > 0 && !swarmPathAllowed(name, task.WritePaths) {
			review.Reason = "The candidate changes files outside this task's reviewed write scope. Review the plan and start a new scoped attempt."
			return review, nil
		}
	}
	patch, err := s.swarmGit(ctx, path, "diff", "--no-ext-diff", "--no-textconv", "--binary", "--full-index", "--no-renames", op.BaseCommit, source.Head, "--")
	if err != nil {
		return review, err
	}
	if len(patch) > 512<<10 {
		return review, errors.New("The patch exceeds the complete review limit. Split the task before integrating; no truncated patch is accepted.")
	}
	review.Patch = patch
	review.ReviewDigest = implementationDigest(struct{ Source, Target, Base, Task, Operation, Patch string }{source.Digest, target.Digest, op.BaseCommit, task.ID, op.ID, patch})
	switch {
	case target.Head != swarmExpectedHead(run):
		review.Reason = "The main source moved outside this swarm's reviewed integrations. Prepare a new run for the changed source."
	case task.Integration != nil:
		review.Reason = "This task already has an integration attempt. Inspect its receipt before taking another action."
	case op.Status != "succeeded" && op.Status != "review-required" && op.Status != "failed":
		review.Reason = "Check agent status and stop its process before integrating committed changes."
	case source.Head == op.BaseCommit || len(review.Paths) == 0:
		review.Reason = "The agent has no committed task changes to integrate."
	case run.State == "cancelled":
		review.Reason = "This swarm is cancelled. Retained work remains available for manual review."
	default:
		if s.branchSessions == nil {
			review.Reason = "Session state is unavailable. Integration requires confirming that this agent has stopped."
			return review, nil
		}
		active, err := s.swarmWorktreeActive(ctx, p.ID, op.WorktreeID)
		if err != nil || active {
			review.Reason = "The task session is still active or its process state is unknown. Stop and observe it before integration."
			return review, nil
		}
		snapshot, err := s.swarmSourceSnapshot(ctx, p)
		if err != nil {
			return review, err
		}
		if !snapshot.CanSwitch {
			review.Reason = snapshot.Message
			return review, nil
		}
		review.CanIntegrate = true
		review.Reason = "Review this complete task diff. Integration combines these exact commits; run checks on the combined result before accepting it."
	}
	return review, nil
}
func swarmPathAllowed(path string, scopes []string) bool {
	for _, scope := range scopes {
		scope = strings.TrimSuffix(scope, "/")
		if path == scope || strings.HasPrefix(path, scope+"/") {
			return true
		}
	}
	return false
}
func (s *Server) reviewProjectSwarmTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	review, err := s.swarmReview(r.Context(), p, run, r.PathValue("taskId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"review": review})
}
func (s *Server) integrateProjectSwarmTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		ReviewDigest     string `json:"reviewDigest"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	guard := s.branchGuard(p.ID)
	if !guard.TryLock() {
		projectRunProblem(w, errors.New("Project source is busy. Wait for the current source operation before integrating."))
		return
	}
	defer guard.Unlock()
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if run.Revision != req.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	review, err := s.swarmReview(r.Context(), p, run, r.PathValue("taskId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if !review.CanIntegrate || review.ReviewDigest != req.ReviewDigest {
		projectRunProblem(w, errors.New("The reviewed source, target or task status changed. Refresh the complete integration review."))
		return
	}
	if err = s.OrchestratorExecutionGate(p.ID); err != nil {
		projectRunProblem(w, err)
		return
	}
	if err = s.validateProjectRunApprovals(p.ID, run.Input.Approvals); err != nil {
		projectRunProblem(w, err)
		return
	}
	taskID := r.PathValue("taskId")
	op := *swarmOperation(&run, review.OperationID)
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error {
		swarmTask(run, taskID).Integration = &ProjectSwarmIntegration{State: "integrating", SourceCommit: review.SourceCommit, TargetCommit: review.TargetCommit, ReviewDigest: review.ReviewDigest}
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	// From this receipt onward, interruption is an unknown outcome. Never retry a
	// filesystem mutation merely because the caller did not receive its result.
	sourcePath, _ := s.swarmPath(p.ID, run.ID, op.ID)
	integrationPath, _ := s.swarmPath(p.ID, run.ID, op.ID+"-integration")
	candidate := ""
	_, err = s.swarmGit(r.Context(), p.Path, "-c", "protocol.file.allow=always", "fetch", "--no-tags", "--no-write-fetch-head", "--", sourcePath, review.SourceCommit)
	if err == nil {
		if _, statErr := os.Lstat(integrationPath); !errors.Is(statErr, os.ErrNotExist) {
			err = errors.New("Integration checkout already exists and cannot be adopted.")
		}
	}
	if err == nil {
		_, err = s.swarmGit(r.Context(), p.Path, "worktree", "add", "--detach", integrationPath, review.TargetCommit)
	}
	if err == nil {
		_, err = s.swarmGit(r.Context(), integrationPath, "-c", "user.name=Luci", "-c", "user.email=luci@localhost", "merge", "--no-ff", "--no-edit", review.SourceCommit)
	}
	if err == nil {
		candidate, err = s.swarmGit(r.Context(), integrationPath, "rev-parse", "HEAD")
		candidate = strings.TrimSpace(candidate)
	}
	if err == nil && !deliverySHA.MatchString(candidate) {
		err = ErrProjectRunInvalid
	}
	if err == nil {
		run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error { swarmTask(run, taskID).Integration.Commit = candidate; return nil })
	}
	if err == nil {
		// Recheck the exact reviewed target immediately before publication. Git's
		// ff-only command refuses branch divergence; it never stashes or resets.
		targetErr := s.validateSwarmGit(r.Context(), p.Path)
		var target gitReview
		if targetErr == nil {
			target, targetErr = s.captureGitReview(r.Context(), p.Path, "")
		}
		if targetErr != nil || target.Head != review.TargetCommit || target.Status != "" || target.Digest != review.targetDigest {
			err = errors.New("The target changed during integration. The candidate remains retained for review.")
		} else {
			_, err = s.swarmGit(r.Context(), p.Path, "merge", "--ff-only", candidate)
		}
	}
	if err == nil {
		head, headErr := s.swarmGit(r.Context(), p.Path, "rev-parse", "HEAD")
		if headErr != nil || strings.TrimSpace(head) != candidate {
			err = errors.New("The integration target could not be confirmed.")
		}
	}
	final, saveErr := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error {
		receipt := swarmTask(run, taskID).Integration
		if err != nil {
			receipt.State = "unknown"
			run.State = "blocked"
			run.Reason = "Integration did not finish with a confirmed receipt. Source, target and integration checkout were retained. Inspect them before continuing."
		} else {
			receipt.State = "integrated"
			receipt.IntegratedAt = time.Now().UTC().Format(time.RFC3339Nano)
			run.State = "ready"
			run.Reason = "The reviewed task was integrated. Dependent tasks can now start; combined checks are still required."
			if swarmAllIntegrated(*run) {
				run.State = "awaiting-checks"
				run.Reason = "All task changes are integrated. Run checks against this combined source before accepting the result."
			}
		}
		return nil
	})
	if saveErr != nil {
		projectRunProblem(w, saveErr)
		return
	}
	s.writeProjectRun(w, 200, final)
}
func swarmAllIntegrated(run ProjectRun) bool {
	if run.Swarm == nil {
		return false
	}
	for _, task := range run.Swarm.Tasks {
		if task.Integration == nil || task.Integration.State != "integrated" {
			return false
		}
	}
	return true
}
