package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type ProjectSwarmReview struct {
	targetDigest           string
	TaskID                 string                `json:"taskId"`
	OperationID            string                `json:"operationId"`
	SourceCommit           string                `json:"sourceCommit"`
	TargetCommit           string                `json:"targetCommit"`
	ReviewDigest           string                `json:"reviewDigest"`
	Paths                  []string              `json:"paths"`
	Patch                  string                `json:"patch"`
	CanIntegrate           bool                  `json:"canIntegrate"`
	IndependentReviewState string                `json:"independentReviewState"`
	Reason                 string                `json:"reason"`
	Handoff                *TaskHandoffReference `json:"handoff,omitempty"`
}

// Diagnostic labels describe a recorded unknown outcome, never replay authority.
var swarmIntegrationStages = map[string]string{
	"candidate-import": "candidate import", "checkout-create": "integration checkout creation",
	"candidate-merge": "candidate merge", "candidate-readback": "candidate readback",
	"candidate-record": "candidate receipt saving", "target-check": "target revalidation",
	"target-publish": "target publication", "target-readback": "target readback",
}
var swarmIntegrationFailureLabels = map[string]string{
	"operation-unconfirmed": "the operation could not be confirmed",
	"command-failed": "a Git command did not complete successfully",
	"interrupted": "the operation was interrupted",
	"target-changed": "the target no longer matched its reviewed state",
	"readback-unconfirmed": "the expected commit could not be confirmed",
	"persistence-unconfirmed": "the candidate receipt could not be confirmed",
}

func validSwarmIntegrationDiagnostic(in ProjectSwarmIntegration) bool {
	if in.Stage == "" && in.FailureCode == "" {
		return true // Preserve omitted historical JSON exactly.
	}
	_, stageKnown := swarmIntegrationStages[in.Stage]
	_, codeKnown := swarmIntegrationFailureLabels[in.FailureCode]
	if in.State != "unknown" || !stageKnown || !codeKnown {
		return false
	}
	switch in.FailureCode {
	case "target-changed":
		return in.Stage == "target-check"
	case "readback-unconfirmed":
		return in.Stage == "candidate-readback" || in.Stage == "target-readback"
	case "persistence-unconfirmed":
		return in.Stage == "candidate-record"
	}
	return true
}

func swarmIntegrationFailureCode(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "interrupted"
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ProcessState != nil && exit.ExitCode() != 0 {
		return "command-failed"
	}
	// An untyped boundary refusal or start error is not proof a command ran.
	return "operation-unconfirmed"
}

func swarmIntegrationFailureReason(stage, code string) string {
	return "Integration is unconfirmed during " + swarmIntegrationStages[stage] + ": " + swarmIntegrationFailureLabels[code] + ". Source and target references remain recorded. Inspect this attempt and refresh history before continuing."
}

// Keep the last acknowledged identity and revision on any candidate-write
// error, including a published write whose acknowledgment was lost. A later
// mutation must use that revision and conflict rather than overwrite a possibly
// newer durable receipt. The caller never retries this write.
func recordSwarmIntegrationCandidate(acknowledged ProjectRun, write func() (ProjectRun, error)) (ProjectRun, error) {
	updated, err := write()
	if err != nil {
		return acknowledged, err
	}
	return updated, nil
}

func (s *Server) swarmReview(ctx context.Context, p project.Project, run ProjectRun, taskID string) (ProjectSwarmReview, error) {
	review := ProjectSwarmReview{TaskID: taskID, Paths: []string{}}
	if err := s.validateProjectSwarmReuseCurrent(ctx, p, run); err != nil {
		return review, err
	}
	task := swarmTask(&run, taskID)
	if task == nil {
		return review, ErrProjectRunNotFound
	}
	op := swarmOperation(&run, task.OperationID)
	if op == nil {
		return review, errors.New("Start this task's isolated attempt before reviewing its changes.")
	}
	review.OperationID = op.ID
	review.Handoff = op.Handoff
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
	handoffDigest := ""
	if op.Handoff != nil {
		view, err := s.projectRuns.taskHandoffView(run, *op)
		if err != nil {
			return review, err
		}
		if view.Receipt.SourceCommit != source.Head || view.Receipt.SourceDirty {
			review.Reason = "The retained report was captured against different source. Refresh the task handoff, then review these changes again."
			return review, nil
		}
		handoffDigest = op.Handoff.Digest
	}
	review.ReviewDigest = implementationDigest(struct{ Source, Target, Base, Task, Operation, Patch, Handoff string }{source.Digest, target.Digest, op.BaseCommit, task.ID, op.ID, patch, handoffDigest})
	// A task may finish after another person's accepted amendment changed its
	// reviewed plan while its source branch stayed at the same commit. Keep its
	// complete patch available for inspection, but never offer integration of
	// old-context work as if it satisfied the current requirements. The same
	// review is reopened inside the exclusive integration mutation.
	s.mu.RLock()
	currentContext, contextErr := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	freshness := projectRunContextFreshness(run, currentContext, contextErr)
	switch {
	case freshness.Status != "current":
		review.Reason = "The reviewed project context changed while this task was running. Keep this candidate for inspection and use the new affected run; its old-context result cannot be integrated automatically."
	case target.Status != "":
		review.Reason = "The main checkout has uncommitted changes. Preserve and commit them before integration."
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
	if run.Input.OrchestrationContract == nil {
		review.IndependentReviewState = "legacy-unassessed"
	} else if swarmIndependentReviewReady(run, r.PathValue("taskId"), review) {
		review.IndependentReviewState = "passed"
	} else {
		review.IndependentReviewState = "pending"
		if review.CanIntegrate {
			review.CanIntegrate = false
			review.Reason = "The complete candidate is ready for an independent exact-attempt review. Retain its review obligation and finding before integrating."
		}
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
	task := swarmTask(&run, r.PathValue("taskId"))
	if task != nil && s.taskHandoffCollector != nil && swarmOperation(&run, task.OperationID) != nil && swarmOperation(&run, task.OperationID).DependencyContext != nil {
		run, err = s.captureTaskHandoff(r.Context(), p.Path, run, task.OperationID, false, true)
		if err != nil {
			projectRunProblem(w, err)
			return
		}
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
	if !swarmIndependentReviewReady(run, r.PathValue("taskId"), review) {
		projectRunProblem(w, errors.New("This candidate needs an independent exact-attempt review and retained finding before integration. Inspect the review obligation and its captured evidence."))
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
	if err = s.checkCoordinatorIntegrationGate(r.Context(), p, run); err != nil {
		projectRunProblem(w, err)
		return
	}
	taskID := r.PathValue("taskId")
	op := *swarmOperation(&run, review.OperationID)
	sourcePath, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	integrationPath, err := s.swarmPath(p.ID, run.ID, op.ID+"-integration")
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error {
		swarmTask(run, taskID).Integration = &ProjectSwarmIntegration{State: "integrating", SourceCommit: review.SourceCommit, TargetCommit: review.TargetCommit, ReviewDigest: review.ReviewDigest}
		if op.Handoff != nil {
			swarmTask(run, taskID).Integration.HandoffDigest = op.Handoff.Digest
		}
		return nil
	})
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	// From this receipt onward, interruption is an unknown outcome. Never retry a
	// filesystem mutation merely because the caller did not receive its result.
	candidate := ""
	stage, failureCode := "candidate-import", ""
	_, err = s.swarmGit(r.Context(), p.Path, "-c", "protocol.file.allow=always", "fetch", "--no-tags", "--no-write-fetch-head", "--", sourcePath, review.SourceCommit)
	if err == nil {
		stage = "checkout-create"
		if _, statErr := os.Lstat(integrationPath); !errors.Is(statErr, os.ErrNotExist) {
			err = errors.New("Integration checkout already exists and cannot be adopted.")
		}
	}
	if err == nil {
		_, err = s.swarmGit(r.Context(), p.Path, "worktree", "add", "--detach", integrationPath, review.TargetCommit)
	}
	if err == nil {
		stage = "candidate-merge"
		_, err = s.swarmGit(r.Context(), integrationPath, "-c", "user.name=Luci", "-c", "user.email=luci@localhost", "merge", "--no-ff", "--no-edit", review.SourceCommit)
	}
	if err == nil {
		stage = "candidate-readback"
		candidate, err = s.swarmGit(r.Context(), integrationPath, "rev-parse", "HEAD")
		candidate = strings.TrimSpace(candidate)
	}
	if err == nil && !deliverySHA.MatchString(candidate) {
		err = ErrProjectRunInvalid
		failureCode = "readback-unconfirmed"
	}
	if err == nil {
		stage = "candidate-record"
		run, err = recordSwarmIntegrationCandidate(run, func() (ProjectRun, error) {
			return s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error { swarmTask(run, taskID).Integration.Commit = candidate; return nil })
		})
		if err != nil {
			failureCode = "persistence-unconfirmed"
		}
	}
	if err == nil {
		stage = "target-check"
		// Recheck the exact reviewed target immediately before publication. Git's
		// ff-only command refuses branch divergence; it never stashes or resets.
		targetErr := s.validateSwarmGit(r.Context(), p.Path)
		var target gitReview
		if targetErr == nil {
			target, targetErr = s.captureGitReview(r.Context(), p.Path, "")
		}
		if targetErr != nil || target.Head != review.TargetCommit || target.Status != "" || target.Digest != review.targetDigest {
			err = errors.New("The target changed during integration. The candidate remains retained for review.")
			if targetErr == nil {
				failureCode = "target-changed"
			} else {
				failureCode = swarmIntegrationFailureCode(targetErr)
			}
		} else {
			stage = "target-publish"
			_, err = s.swarmGit(r.Context(), p.Path, "merge", "--ff-only", candidate)
		}
	}
	if err == nil {
		stage = "target-readback"
		head, headErr := s.swarmGit(r.Context(), p.Path, "rev-parse", "HEAD")
		if headErr != nil || strings.TrimSpace(head) != candidate {
			err = errors.New("The integration target could not be confirmed.")
			failureCode = "readback-unconfirmed"
			if headErr != nil {
				failureCode = swarmIntegrationFailureCode(headErr)
			}
		}
	}
	final, saveErr := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error {
		receipt := swarmTask(run, taskID).Integration
		if err != nil {
			receipt.State = "unknown"
			receipt.Stage = stage
			if failureCode == "" {
				failureCode = swarmIntegrationFailureCode(err)
			}
			receipt.FailureCode = failureCode
			run.State = "blocked"
			run.Reason = swarmIntegrationFailureReason(stage, failureCode)
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
		if task.ReviewOnly {
			op := swarmOperation(&run, task.OperationID)
			if op == nil || op.Handoff == nil || op.Status != "succeeded" && op.Status != "review-required" {
				return false
			}
			passed := false
			for _, item := range run.Swarm.ReviewObligations {
				candidate := swarmTask(&run, item.TaskID)
				candidateOp := swarmOperation(&run, item.CandidateOperationID)
				validContractedReview := run.Input.OrchestrationContract == nil || swarmContractReviewerRole(run, task, item.TaskID) && op.ReviewAssignmentDigest == swarmResolvedReviewAssignmentDigest(item) && candidateOp != nil && swarmReviewRepairSuccessor(run, item.TaskID, *candidateOp, item.CandidateCommit) == nil
				if validContractedReview && item.ReviewerOperationID == op.ID && item.State == "passed" && item.ReviewerHandoffDigest == op.Handoff.Digest && candidate != nil && candidate.Integration != nil && candidate.Integration.State == "integrated" {
					passed = true
					break
				}
			}
			if !passed {
				return false
			}
			continue
		}
		if task.Integration == nil || task.Integration.State != "integrated" {
			return false
		}
	}
	return true
}
