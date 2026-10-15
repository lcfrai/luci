package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func coordinatorContinuationFixture(t *testing.T) (*Server, project.Project, http.Handler, context.Context, coordinatorContinuationRequest, *int) {
	t.Helper()
	s, _, p, run, selection, _ := projectRunHTTPFixture(t)
	run.IdempotencyKey = ""
	run.PermissionMode = "bypass"
	selection.PermissionMode = "bypass"
	s.SetCoordinatorAuthorizer(func(ctx context.Context, id string) error {
		if id != p.ID {
			return errors.New("foreign")
		}
		return ctx.Err()
	})
	s.SetCoordinatorExecutionAuthorizer(func(ctx context.Context, id, harness string, secrets bool) error {
		if id != p.ID || harness != "codex" || !secrets {
			return errors.New("wrong route")
		}
		return ctx.Err()
	})
	s.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	starts := new(int)
	children := map[string]registry.Session{}
	s.swarmScheduler.runtime = SwarmSchedulerRuntime{CaptureActor: management.CaptureExecutionActor, Admit: func(context.Context, SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
		return SwarmAdmissionResult{State: "admitted", MemoryMiB: 1024, CPUMillis: 1000}, nil
	}, Release: func(context.Context, string) error { return nil }, Dispatch: func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
		current, err := s.GetProjectSwarmRun(req.ProjectID, req.RunID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		current, op, claimed, err := s.projectRuns.ClaimOperation(p.ID, p.Path, current.ID, current.Revision, req.OperationID)
		if err != nil || !claimed {
			return SwarmDispatchResult{}, errors.New("claim failed")
		}
		*starts++
		picked := *selection
		picked.WorktreeID = op.WorktreeID
		child := projectRunCandidate(picked, op.Implementation)
		child.ID = op.ChildID
		child.ProjectRunID = current.ID
		child.ProjectRunOperationID = op.ID
		child.Status = registry.StatusRunning
		children[op.ID] = child
		return SwarmDispatchResult{State: "running", SessionID: op.ChildID}, nil
	}}
	s.projectRunSessionObserver = func(_ context.Context, projectID, runID, opID string) ([]registry.Session, error) {
		if child, ok := children[opID]; ok {
			return []registry.Session{child}, nil
		}
		return nil, nil
	}
	mux := http.NewServeMux()
	s.Register(mux)
	h := contextHistoryActorHandler(mux, management.User{ID: "alice", Username: "alice", DisplayName: "Alice", Role: "member"})
	ctx := orchestratorActor(t, "alice")
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Continue the reviewed notebook implementation", "")
	answer := OrchestratorAnswer{Summary: "The saved tasks and acceptance are ready for the selected upstream harness.", Options: []string{}, Impact: []string{}, NextAction: "build"}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{"expectedRevision": 0, "mode": "review", "allowedTargets": []string{"plan"}, "executionMode": "reviewed-tasks", "deploymentMode": "manual"}, 200).Body.Bytes(), &policy)
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	req := coordinatorContinuationRequest{IdempotencyKey: "continue-fixture-key-12345", OriginTurnID: turn.ID, ExpectedContextDigest: view.Context.Digest, ExpectedPolicyRevision: policy.Revision, Run: run, MaxParallel: 1}
	endpoint := "/v1/product/projects/" + p.ID + "/orchestration-contracts"
	prepared := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/prepare", contractRequest(run), 201).Body.Bytes())
	approved := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/"+prepared.Contract.ID+"/approve", map[string]any{"expectedRevision": prepared.Contract.Revision, "expectedDigest": prepared.Contract.Digest}, 200).Body.Bytes())
	req.Run.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	return s, p, h, ctx, req, starts
}
func coordinatorCreate(t *testing.T, h http.Handler, p project.Project, req coordinatorContinuationRequest, status int) CoordinatorContinuation {
	t.Helper()
	var out CoordinatorContinuation
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", req, status).Body.Bytes(), &out)
	return out
}

func TestCoordinatorCapacityAdviceIsReadOnlyAndScopedToCurrentActor(t *testing.T) {
	s, p, h, _, _, _ := coordinatorContinuationFixture(t)
	calls := 0
	s.SetCoordinatorCloudCapacity(func(_ context.Context, projectID, actorID string) (CoordinatorCloudCapacity, error) {
		calls++
		if projectID != p.ID || actorID != "alice" {
			t.Fatal("capacity read lost project or person scope", projectID, actorID)
		}
		return CoordinatorCloudCapacity{Provider: "e2b-embed", Region: "us-east-1", ConfiguredAvailableSlots: 2}, nil
	})
	var response struct {
		CloudCapacity *CoordinatorCloudCapacity `json:"cloudCapacity"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", nil, 200).Body.Bytes(), &response)
	if calls != 1 || response.CloudCapacity == nil || response.CloudCapacity.ConfiguredAvailableSlots != 2 {
		t.Fatal("scoped read did not expose configured advice", calls, response.CloudCapacity)
	}
}
func TestCoordinatorContinuationRunsTypedPreparationQueueAndExactlyOnceAdmission(t *testing.T) {
	s, p, h, _, req, starts := coordinatorContinuationFixture(t)
	out := coordinatorCreate(t, h, p, req, 201)
	if out.IdempotencyKey != req.IdempotencyKey || out.Status != "queued" || out.RunID == "" || len(out.QueueIDs) != 1 || len(out.Actions) != 3 {
		t.Fatalf("continuation did not reach existing queue: %+v", out)
	}
	for _, a := range out.Actions {
		if a.Status != "confirmed" || a.ReferenceID != out.RunID {
			t.Fatalf("lost exact action %+v", a)
		}
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, out.RunID)
	if err != nil || run.Swarm == nil || run.Input.Route.Connection.ID != *req.Run.AgentConnectionID || run.Input.Implementation.PlanRevision != req.Run.ImplementationContext.PlanRevision {
		t.Fatal("route/context binding", err)
	}
	before, _ := s.ProjectSwarmQueueSnapshot(p.ID)
	replay := coordinatorCreate(t, h, p, req, 200)
	after, _ := s.ProjectSwarmQueueSnapshot(p.ID)
	if replay.ID != out.ID || !reflect.DeepEqual(before, after) || *starts != 0 {
		t.Fatal("duplicate continuation replayed preparation")
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 1 {
		t.Fatalf("existing scheduler did not dispatch exact queued task: %d", *starts)
	}
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 1 {
		t.Fatal("ordinary observation duplicated worker")
	}
	req.Run.Model = "different"
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", req, 409)
}

func TestCoordinatorContinuationRequiresApprovedContractBeforeNewWork(t *testing.T) {
	s, p, h, ctx, req, starts := coordinatorContinuationFixture(t)
	approved := req.Run.OrchestrationContract
	req.Run.OrchestrationContract = nil
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", req, http.StatusConflict)
	if rows, err := s.readContinuations(p, "alice"); err != nil || len(rows.Rows) != 0 {
		t.Fatal("unreviewed continuation was retained", rows, err)
	}
	if runs, err := s.projectRuns.List(p.ID, p.Path); err != nil || len(runs) != 0 || *starts != 0 {
		t.Fatal("unreviewed continuation created a run or child", runs, err, *starts)
	}
	wrong := *approved
	wrong.Digest = reviewDigest("wrong-approved-contract")
	req.Run.OrchestrationContract = &wrong
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", req, http.StatusConflict)
	if rows, err := s.readContinuations(p, "alice"); err != nil || len(rows.Rows) != 0 {
		t.Fatal("invalid approval was durably admitted", rows, err)
	}
	req.Run.OrchestrationContract = approved
	created := coordinatorCreate(t, h, p, req, http.StatusCreated)
	if created.RunID == "" {
		t.Fatal("approved continuation did not create an exact run")
	}
	rows, err := s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 {
		t.Fatal("approved row missing", err)
	}
	// A retained legacy nil-contract row remains readable, but it must never
	// take the new automatic integration path even with a ready candidate.
	legacy := rows.Rows[0]
	legacy.Run.OrchestrationContract = nil
	if integrated, err := s.continueCoordinatorCandidateIntegration(ctx, p, legacy); err != nil || integrated {
		t.Fatal("nil-contract continuation passed automatic integration", integrated, err)
	}
}
func TestCoordinatorFirstCloudActivationQueuesExactReviewedTasksWithoutLocalFallback(t *testing.T) {
	s, p, h, _, req, starts := coordinatorContinuationFixture(t)
	req.Run.ExecutionLocation = "cloud-microvm"
	// No cloud service means no run, queue or local fallback, even with a
	// reviewed task plan and a valid upstream model connection.
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", req, 503)
	if runs, err := s.projectRuns.List(p.ID, p.Path); err != nil || len(runs) != 0 || *starts != 0 {
		t.Fatal("unconfigured cloud activation started work", runs, err, *starts)
	}
	if retained, err := s.readContinuations(p, "alice"); err != nil || len(retained.Rows) != 0 {
		t.Fatal("known cloud setup refusal left an uncertain continuation", retained, err)
	}
	available := true
	s.SetCloudMicroVMAvailability(func() bool { return available })
	prepare := contractRequest(req.Run)
	prepare.IdempotencyKey = "prepare-cloud-contract-123456"
	prepare.Route.ExecutionLocation = "cloud-microvm"
	endpoint := "/v1/product/projects/" + p.ID + "/orchestration-contracts"
	prepared := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	approved := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/"+prepared.Contract.ID+"/approve", map[string]any{"expectedRevision": prepared.Contract.Revision, "expectedDigest": prepared.Contract.Digest}, 200).Body.Bytes())
	req.Run.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	out := coordinatorCreate(t, h, p, req, 201)
	if out.Status != "queued" || out.RunID == "" || len(out.QueueIDs) != 1 {
		t.Fatal("reviewed cloud activation did not reach the existing queue", out)
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, out.RunID)
	if err != nil || run.Input.Route.ExecutionLocation != "cloud-microvm" || run.Input.Implementation.PlanRevision != req.Run.ImplementationContext.PlanRevision {
		t.Fatal("cloud task lost placement or reviewed context", run.Input, err)
	}
	available = false // readback of an admitted identity must survive service loss
	duplicate := coordinatorCreate(t, h, p, req, 200)
	if duplicate.ID != out.ID || *starts != 0 {
		t.Fatal("duplicate cloud activation changed its durable identity", duplicate, *starts)
	}
	available = true
	if err = s.RunSwarmSchedulerOnce(context.Background()); err != nil || *starts != 1 {
		t.Fatal("existing scheduler did not dispatch the exact reviewed cloud queue once", err, *starts)
	}
}
func TestCoordinatorContinuationReadOnlyActorScopeAndAuthorityRefusal(t *testing.T) {
	s, p, h, ctx, req, _ := coordinatorContinuationFixture(t)
	out := coordinatorCreate(t, h, p, req, 201)
	path, err := s.continuationPath(p.ID, "alice", false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	var ownList struct {
		Continuations []CoordinatorContinuation `json:"continuations"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", nil, 200).Body.Bytes(), &ownList)
	if len(ownList.Continuations) != 1 || ownList.Continuations[0].IdempotencyKey != req.IdempotencyKey || ownList.Continuations[0].ID != out.ID {
		t.Fatal("read-only reconciliation cannot identify the exact uncertain create")
	}
	mux := http.NewServeMux()
	s.Register(mux)
	bob := contextHistoryActorHandler(mux, management.User{ID: "bob", Username: "bob", Role: "member"})
	base := "/v1/product/projects/" + p.ID + "/orchestrator/continuations"
	journeyRequest(t, bob, "GET", base+"/"+out.ID, nil, 404)
	var list struct {
		Continuations []CoordinatorContinuation `json:"continuations"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, bob, "GET", base, nil, 200).Body.Bytes(), &list)
	if len(list.Continuations) != 0 {
		t.Fatal("another person's continuation leaked")
	}
	s.SetCoordinatorExecutionAuthorizer(func(context.Context, string, string, bool) error { return errors.New("execution revoked") })
	journeyRequest(t, h, "POST", base+"/"+out.ID+"/advance", map[string]int{"expectedRevision": out.Revision}, 403)
	journeyRequest(t, h, "GET", base+"/"+out.ID, nil, 200)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read or revoked advance changed private receipt")
	}
	if _, err = s.OrchestratorView(ctx, p.ID, 0); err != nil {
		t.Fatal(err)
	}
}
func TestCoordinatorContinuationSourceExclusionPolicyScopeAndStaleContext(t *testing.T) {
	for _, kind := range []string{"source-lock", "execution-disabled", "context", "origin-actor"} {
		t.Run(kind, func(t *testing.T) {
			s, p, h, _, req, _ := coordinatorContinuationFixture(t)
			switch kind {
			case "source-lock":
				guard := s.branchGuard(p.ID)
				guard.Lock()
				defer guard.Unlock()
			case "execution-disabled":
				coordinatorSavePolicy(t, h, p.ID, req.ExpectedPolicyRevision, "review", []string{"plan"})
			case "context":
				req.ExpectedContextDigest = reviewDigest("different context")
			case "origin-actor":
				mux := http.NewServeMux()
				s.Register(mux)
				h = contextHistoryActorHandler(mux, management.User{ID: "bob", Username: "bob", Role: "member"})
			}
			journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", req, 409)
			runs, err := s.projectRuns.List(p.ID, p.Path)
			if err != nil || len(runs) != 0 {
				t.Fatal("refusal created run", err)
			}
		})
	}
}
func TestCoordinatorPolicyPartialSavePreservesExecutionAndRejectsUnsupportedDeployment(t *testing.T) {
	s, p, h, _, req, _ := coordinatorContinuationFixture(t)
	policy := coordinatorSavePolicy(t, h, p.ID, req.ExpectedPolicyRevision, "auto-drafts", []string{"plan"})
	if policy.ExecutionMode != "reviewed-tasks" || policy.DeploymentMode != "manual" {
		t.Fatal("draft control silently changed delivery policy", policy)
	}
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{"expectedRevision": policy.Revision, "mode": "review", "allowedTargets": []string{"plan"}, "deploymentMode": "automatic"}, 400)
	stored, err := s.readCoordinatorPolicy(p)
	if err != nil || stored.Revision != policy.Revision {
		t.Fatal("unsupported policy mutated authority", err)
	}
}

func coordinatorPendingFixture(t *testing.T, s *Server, p project.Project, ctx context.Context, req coordinatorContinuationRequest) coordinatorContinuationRecord {
	t.Helper()
	actor, _ := authenticatedContextActor(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := s.continuationContextLocked(p, req.Run.ImplementationContext)
	if err != nil {
		t.Fatal(err)
	}
	key := req.IdempotencyKey
	req.IdempotencyKey = ""
	id := newID("continue-")
	now := "2026-10-11T15:00:00Z"
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ID: id, IdempotencyKey: key, ProjectID: p.ID, Actor: actor, OriginTurnID: req.OriginTurnID, ContextDigest: req.ExpectedContextDigest, PolicyRevision: req.ExpectedPolicyRevision, Revision: 2, Status: "continuing", Message: "Exact operation reserved before process loss", Actions: []CoordinatorAction{{ID: id + "-create-run", Kind: "create-run", Status: "pending", StartedAt: now}}, QueueIDs: []string{}, CreatedAt: now, UpdatedAt: now}, KeyDigest: reviewDigest(key), RequestDigest: reviewDigest(req), CanonicalDigest: canonical, Run: req.Run, MaxParallel: req.MaxParallel}
	rows.Rows = append(rows.Rows, row)
	if err = s.saveContinuations(rows); err != nil {
		t.Fatal(err)
	}
	return row
}
func TestCoordinatorContinuationRestartReconcilesExactCreatedRunWithoutReplay(t *testing.T) {
	s, p, _, ctx, req, starts := coordinatorContinuationFixture(t)
	row := coordinatorPendingFixture(t, s, p, ctx, req)
	runID, _, err := s.invokeCoordinatorStep(ctx, p, row, "create-run")
	if err != nil || runID == "" {
		t.Fatal("could not stage lost response", err)
	}
	// The exact production call completed, but its caller died before journaling
	// the response. Only the pre-call pending action is durable.
	path, _ := s.continuationPath(p.ID, "alice", false)
	pending, _ := os.ReadFile(path)
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.SetCoordinatorAuthorizer(s.coordinatorAuthorize)
	restarted.SetCoordinatorExecutionAuthorizer(s.coordinatorExecuteAuthorize)
	restarted.SetProjectRunSelectionResolver(s.projectRunSelectionResolver)
	restarted.swarmScheduler.runtime = s.swarmScheduler.runtime
	restarted.branchSessions = s.branchSessions
	restarted.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	read, err := restarted.readContinuations(p, "alice")
	if err != nil || read.Rows[0].Actions[0].Status != "pending" {
		t.Fatal("restart lost action", err)
	}
	afterRead, _ := os.ReadFile(path)
	if string(pending) != string(afterRead) {
		t.Fatal("read repaired journal")
	}
	out, err := restarted.continueCoordinator(ctx, p, row.ID, "alice", row.Revision)
	if err != nil || out.RunID != runID || out.Status != "queued" || len(out.Actions) != 3 {
		t.Fatalf("readback did not safely continue: %+v %v", out, err)
	}
	runs, err := restarted.projectRuns.List(p.ID, p.Path)
	if err != nil || len(runs) != 1 || *starts != 0 {
		t.Fatal("unknown response duplicated a run/worker", err, *starts)
	}
	if out.Actions[0].StartedAt != row.Actions[0].StartedAt {
		t.Fatal("readback replaced original action identity")
	}
}
func TestCoordinatorContinuationUnknownAbsentActionNeverReplays(t *testing.T) {
	s, p, _, ctx, req, starts := coordinatorContinuationFixture(t)
	row := coordinatorPendingFixture(t, s, p, ctx, req)
	path, _ := s.continuationPath(p.ID, "alice", false)
	before, _ := os.ReadFile(path)
	for i := 0; i < 2; i++ {
		out, err := s.continueCoordinator(ctx, p, row.ID, "alice", row.Revision)
		if err != nil || len(out.Actions) != 1 || out.Actions[0].Status != "pending" {
			t.Fatal("unconfirmed action was replayed", out, err)
		}
	}
	after, _ := os.ReadFile(path)
	runs, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil || len(runs) != 0 || *starts != 0 || string(before) != string(after) {
		t.Fatal("absent observation invented mutation permission", err)
	}
}
