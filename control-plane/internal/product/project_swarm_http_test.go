package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func swarmHTTPFixture(t *testing.T, taskOverrides ...ProjectTask) (*Server, *http.ServeMux, project.Project, ProjectRun, string, *ProjectRunSelection) {
	return swarmHTTPFixtureWithLocation(t, "", taskOverrides...)
}
func swarmHTTPFixtureWithLocation(t *testing.T, location string, taskOverrides ...ProjectTask) (*Server, *http.ServeMux, project.Project, ProjectRun, string, *ProjectRunSelection) {
	t.Helper()
	s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
	if location == "cloud-microvm" {
		s.SetCloudMicroVMAvailability(func() bool { return true })
		req.ExecutionLocation = location
	}
	// This product-only fixture has no external execution lease ledger.
	s.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	// Autonomous workers use only an explicitly saved supported permission;
	// the ordinary interactive run fixture intentionally remains Ask.
	req.PermissionMode, selected.PermissionMode = "bypass", "bypass"
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	base := "/v1/product/projects/" + p.ID
	var plan ProjectPlan
	tasks := []ProjectTask{{ID: "task-a", Title: "Add first component", Status: "done", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Alpha works"}, {ID: "task-b", Title: "Add second component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"beta.txt"}}, {ID: "task-c", Title: "Use first component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}}}
	if len(taskOverrides) > 0 {
		tasks = taskOverrides
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": req.ImplementationContext.PlanRevision, "expectedSpecificationRevision": req.ImplementationContext.SpecificationRevision, "tasks": tasks}, 200).Body.Bytes(), &plan)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Save parallel task contracts"), 200)
	req.ImplementationContext.PlanRevision = plan.Revision
	req.ImplementationContext.TaskIDs = []string{"task-a", "task-b", "task-c"}
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", req, 201)).ProjectRun
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/swarm", map[string]int{"expectedRevision": run.Revision, "maxParallel": 2}, 201)).ProjectRun
	return s, mux, p, run, base + "/runs/" + run.ID + "/swarm", selected
}
func swarmReserveHTTP(t *testing.T, mux *http.ServeMux, base string, run ProjectRun, task, key string) ProjectRun {
	t.Helper()
	return projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/"+task+"/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": key}, 201)).ProjectRun
}
func swarmCommitFixture(t *testing.T, path, name, contents string) string {
	t.Helper()
	if err := validateOwnedFixtureWorktree(t, path); err != nil {
		t.Fatalf("refuse unsafe swarm fixture commit: %v", err)
	}
	file, err := guardedProjectPath(path, name, true)
	if err != nil {
		t.Fatalf("refuse unsafe swarm fixture file: %v", err)
	}
	if err := os.WriteFile(file, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	confinedFixtureGit(t, path, "add", "--", name)
	confinedFixtureGit(t, path, "-c", "user.name=Worker fixture", "-c", "user.email=worker@example.test", "commit", "-m", "Implement task")
	return confinedFixtureGit(t, path, "rev-parse", "HEAD")
}
func swarmReviewHTTP(t *testing.T, mux *http.ServeMux, base, task string) ProjectSwarmReview {
	t.Helper()
	var out struct {
		Review ProjectSwarmReview `json:"review"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/tasks/"+task+"/review", nil, 200).Body.Bytes(), &out)
	return out.Review
}
func TestProjectSwarmHTTPRealGitParallelIntegrationAndDependencyRelease(t *testing.T) {
	s, mux, p, run, base, selected := swarmHTTPFixture(t)
	// Board Done cannot release task-c. Neither can mere reservation/process exit.
	journeyRequest(t, mux, "POST", base+"/tasks/task-c/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "dependency-key-123456"}, 409)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "task-a-key-1234567890")
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "task-b-key-1234567890")
	if len(run.Operations) != 2 || run.Operations[0].Allocation != "ready" || run.Operations[1].Allocation != "ready" {
		t.Fatalf("allocations: %+v", run.Operations)
	}
	paths := []string{}
	children := map[string]registry.Session{}
	for _, op := range run.Operations {
		path, err := s.swarmPath(p.ID, run.ID, op.ID)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		if info, err := os.Stat(filepath.Join(path, ".git")); err != nil || !info.IsDir() {
			t.Fatal("worker needs independent Git metadata", err)
		}
		if _, err := os.Stat(filepath.Join(path, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
			t.Fatal("worker objects must not borrow mutable main objects", err)
		}
		if op.Implementation == nil || len(op.Implementation.TaskIDs) != 1 || op.Implementation.TaskIDs[0] != op.TaskID {
			t.Fatal("task context widened", op)
		}
		route := *selected
		route.WorktreeID = op.WorktreeID
		child := projectRunCandidate(route, op.Implementation)
		child.ID = op.ChildID
		child.ProjectRunID = run.ID
		child.ProjectRunOperationID = op.ID
		child.Status = registry.StatusExited
		child.ExitReason = "exited"
		children[op.ID] = child
		var claimed bool
		run, _, claimed, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, op.ID)
		if err != nil || !claimed {
			t.Fatal(err)
		}
	}
	if paths[0] == paths[1] || strings.HasPrefix(paths[0], p.Path+string(os.PathSeparator)) {
		t.Fatal("agent checkouts are not isolated", paths)
	}
	swarmCommitFixture(t, paths[0], "alpha.txt", "alpha\n")
	swarmCommitFixture(t, paths[1], "beta.txt", "beta\n")
	if _, err := os.Stat(filepath.Join(p.Path, "alpha.txt")); !os.IsNotExist(err) {
		t.Fatal("worker edited main before review")
	}
	s.SetProjectRunSessionObserver(func(_ context.Context, _, _, operationID string) ([]registry.Session, error) {
		child, ok := children[operationID]
		if !ok {
			return nil, nil
		}
		return []registry.Session{child}, nil
	})
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/reconcile", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	journeyRequest(t, mux, "POST", base+"/tasks/task-c/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "still-dependent-123456"}, 409)
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if !review.CanIntegrate || !strings.Contains(review.Patch, "+alpha") {
		t.Fatalf("review %+v", review)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	if swarmTask(&run, "task-a").Integration.State != "integrated" {
		t.Fatalf("integration not confirmed: %+v", run)
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-c", "task-c-key-1234567890")
	last := run.Operations[len(run.Operations)-1]
	if last.BaseCommit != swarmTask(&run, "task-a").Integration.Commit {
		t.Fatal("dependent task missed integrated dependency")
	}
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, last.ID)
	if data, err := os.ReadFile(filepath.Join(path, "alpha.txt")); err != nil || string(data) != "alpha\n" {
		t.Fatal("dependency output not present", err)
	}
	// Concurrent task B was based on the original main; reviewed merge combines
	// both independent commits without overwriting A or the current plan.
	review = swarmReviewHTTP(t, mux, base, "task-b")
	if !review.CanIntegrate {
		t.Fatal(review.Reason)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-b/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		if _, err := os.Stat(filepath.Join(p.Path, name)); err != nil {
			t.Fatal("parallel output lost", name, err)
		}
	}
	if run.State == "complete" || run.Acceptance != nil || len(run.Checks) != 0 {
		t.Fatal("integration fabricated verified completion")
	}
	// Reopen controller persistence: task allocation/contexts/integrations survive
	// process restart without a create, merge or child replay.
	before := auditTree(t, s.stateDir)
	reopened, err := NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(p.ID, p.Path, run.ID)
	if err != nil || len(loaded.Operations) != 3 || swarmTask(&loaded, "task-a").Integration.State != "integrated" {
		t.Fatal(loaded, err)
	}
	if !reflectSwarmTree(before, auditTree(t, s.stateDir)) {
		t.Fatal("reopen mutated private receipts")
	}
}
func reflectSwarmTree(a, b any) bool { return implementationDigest(a) == implementationDigest(b) }

func swarmFinishFixture(t *testing.T, s *Server, p project.Project, run ProjectRun, opID string) ProjectRun {
	t.Helper()
	var err error
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, opID)
	if err != nil {
		t.Fatal(err)
	}
	op := swarmOperation(&run, opID)
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, opID, op.ChildID, "succeeded", "Controlled test process exited; result still requires integration review.")
	if err != nil {
		t.Fatal(err)
	}
	return run
}
func TestProjectSwarmHTTPConflictRetainsBothSidesAndAllowsExplicitNewAttempt(t *testing.T) {
	tasks := []ProjectTask{{ID: "task-a", Title: "First change", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"shared.txt"}}, {ID: "task-b", Title: "Second change", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"shared.txt"}}, {ID: "task-c", Title: "Dependent change", Status: "todo", RequirementIDs: []string{"AC-save"}, DependsOn: []string{"task-a"}}}
	s, mux, p, run, base, _ := swarmHTTPFixture(t, tasks...)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "conflict-first-123456789")
	opA := run.Operations[0]
	pathA := mustSwarmPathFixture(t, s, p.ID, run.ID, opA.ID)
	sourceA := swarmCommitFixture(t, pathA, "shared.txt", "first independently reviewed version\n")
	run = swarmFinishFixture(t, s, p, run, opA.ID)
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "conflict-second-123456789")
	opB := run.Operations[1]
	pathB := mustSwarmPathFixture(t, s, p.ID, run.ID, opB.ID)
	sourceB := swarmCommitFixture(t, pathB, "shared.txt", "second independently reviewed version\n")
	run = swarmFinishFixture(t, s, p, run, opB.ID)
	reviewA := swarmReviewHTTP(t, mux, base, "task-a")
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": reviewA.ReviewDigest}, 200)).ProjectRun
	target := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
	reviewB := swarmReviewHTTP(t, mux, base, "task-b")
	if !reviewB.CanIntegrate {
		t.Fatal(reviewB.Reason)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-b/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": reviewB.ReviewDigest}, 200)).ProjectRun
	if swarmTask(&run, "task-b").Integration.State != "unknown" {
		t.Fatal("conflict must remain unresolved", run)
	}
	if confinedFixtureGit(t, p.Path, "rev-parse", "HEAD") != target || confinedFixtureGit(t, pathA, "rev-parse", "HEAD") != sourceA || confinedFixtureGit(t, pathB, "rev-parse", "HEAD") != sourceB {
		t.Fatal("conflict changed source or target")
	}
	integrationPath := mustSwarmPathFixture(t, s, p.ID, run.ID, opB.ID+"-integration")
	if !strings.Contains(confinedFixtureGit(t, integrationPath, "status", "--porcelain"), "AA shared.txt") {
		t.Fatal("conflict evidence was discarded")
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-b/abandon", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	if run.Operations[1].Integration == nil || run.Operations[1].Integration.State != "unknown" || swarmTask(&run, "task-b").Integration != nil {
		t.Fatal("retirement lost the retained receipt")
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "conflict-fresh-123456789")
	opNew := run.Operations[len(run.Operations)-1]
	if opNew.ID == opB.ID || opNew.BaseCommit != target {
		t.Fatal("retry replayed old attempt or missed integrated source")
	}
	if _, err := os.Stat(integrationPath); err != nil {
		t.Fatal("old conflict checkout must remain available", err)
	}
}
func TestProjectSwarmHTTPReviewRejectsDirtyOutsideScopeAndChangedTarget(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "review-guard-123456789")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	if err := os.WriteFile(filepath.Join(path, "alpha.txt"), []byte("uncommitted"), 0600); err != nil {
		t.Fatal(err)
	}
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if review.CanIntegrate || !strings.Contains(review.Reason, "uncommitted") {
		t.Fatal(review)
	}
	swarmCommitFixture(t, path, "alpha.txt", "reviewed change\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	review = swarmReviewHTTP(t, mux, base, "task-a")
	if !review.CanIntegrate {
		t.Fatal(review)
	}
	swarmCommitFixture(t, p.Path, "independent.txt", "target moved\n")
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 409)
	loaded, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || swarmTask(&loaded, "task-a").Integration != nil {
		t.Fatal("stale review allocated integration", err)
	}
	swarmCommitFixture(t, path, "outside.txt", "out of scope\n")
	review = swarmReviewHTTP(t, mux, base, "task-a")
	if review.CanIntegrate || !strings.Contains(review.Reason, "outside") {
		t.Fatal(review)
	}
}
func TestProjectSwarmHTTPUnknownAllocationRetiredWithoutReplay(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	runner := s.runner
	allocations := 0
	s.runner = func(ctx context.Context, command string, args []string, dir string) (string, error) {
		if strings.Contains(dir, ".luci-swarms") && len(args) > 0 {
			for _, arg := range args {
				if arg == "fetch" {
					allocations++
					return "", errors.New("controlled fetch interruption")
				}
			}
		}
		return runner(ctx, command, args, dir)
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "allocation-unknown-123456")
	if run.Operations[0].Allocation != "unknown" || allocations != 1 {
		t.Fatal(run.Operations, allocations)
	}
	old := run.Operations[0]
	oldPath := mustSwarmPathFixture(t, s, p.ID, run.ID, old.ID)
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/reserve", map[string]any{"expectedRevision": 1, "idempotencyKey": "allocation-unknown-123456"}, 200)
	if allocations != 1 {
		t.Fatal("lost response replayed allocation")
	}
	s.runner = runner
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "allocation-retry-123456789")
	if run.Operations[0].Allocation != "abandoned" || run.Operations[1].Allocation != "ready" || run.Operations[0].WorktreeID == run.Operations[1].WorktreeID {
		t.Fatal(run.Operations)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("unknown worktree was deleted", err)
	}
}
func TestProjectSwarmHTTPReconcileRecoversPublishedIntegrationWithoutMergingAgain(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "recover-first-123456789")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, "alpha.txt", "recoverable\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	review := swarmReviewHTTP(t, mux, base, "task-a")
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	var err error
	run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(run *ProjectRun) error {
		receipt := swarmTask(run, "task-a").Integration
		receipt.State = "unknown"
		receipt.IntegratedAt = ""
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	head := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
	before := auditTree(t, p.Path)
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/reconcile", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	if swarmTask(&run, "task-a").Integration.State != "integrated" || confinedFixtureGit(t, p.Path, "rev-parse", "HEAD") != head || !reflectSwarmTree(before, auditTree(t, p.Path)) {
		t.Fatal("recovery reran or changed Git")
	}
}

func TestProjectSwarmHTTPSubscriptionPreparationRequiresSingleSlot(t *testing.T) {
	s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
	req.PermissionMode, selected.PermissionMode = "bypass", "bypass"
	selected.Connection.Kind = "codex-subscription"
	selected.Connection.AuthMode = "chatgpt"
	selected.AgentConnectionKind = "codex-subscription"
	selected.AuthMode = "chatgpt"
	selected.CodexConnectionID = "private-subscription-generation"
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
	base += "/" + run.ID + "/swarm"
	response := journeyRequest(t, mux, "POST", base, map[string]int{"expectedRevision": run.Revision, "maxParallel": 2}, 409)
	if !strings.Contains(response.Body.String(), "one active coding session") {
		t.Fatal(response.Body.String())
	}
	stored, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || stored.Swarm != nil {
		t.Fatal("over-capacity preparation persisted work", err)
	}
	prepared := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, map[string]int{"expectedRevision": run.Revision, "maxParallel": 1}, 201))
	if prepared.Swarm == nil || prepared.Swarm.MaxParallel != 1 {
		t.Fatal(prepared)
	}
}
func TestProjectSwarmHTTPProtectedContextChangesNeverEnterReviewPatch(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "protected-scope-123456789")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, ".luci/plan.json", "{\"unreviewed\":\"fixture-secret-marker\"}\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if review.CanIntegrate || review.Patch != "" || !strings.Contains(review.Reason, "protected project context") {
		t.Fatal(review)
	}
}

func TestProjectSwarmHTTPRetiringBrokenCheckoutRestoresCatalogueWithoutDeletingFiles(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "broken-clone-123456789")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	retained := path + "-metadata-retained"
	if err := os.Rename(filepath.Join(path, ".git"), retained); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProjectSwarmWorktrees(context.Background(), p.ID); err == nil {
		t.Fatal("broken current task checkout should fail closed")
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "fixed-clone-123456789")
	rows, err := s.ProjectSwarmWorktrees(context.Background(), p.ID)
	if err != nil || len(rows) != 1 || rows[0].ID != run.Operations[1].WorktreeID {
		t.Fatal("retired damaged checkout poisoned replacement admission", rows, err)
	}
	if _, err := os.Stat(retained); err != nil {
		t.Fatal("retirement removed retained bytes", err)
	}
}
