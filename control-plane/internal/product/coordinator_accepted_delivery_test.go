package product

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func approveAcceptedChangedContract(t *testing.T, s *Server, p project.Project, h http.Handler, ctx context.Context, req coordinatorContinuationRequest) CoordinatorReauthorizer {
	t.Helper()
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	review := s.acceptedCoordinatorReview(ctx, p, rows)
	if review == nil || review.Status != "review-required" || review.Selection == nil {
		t.Fatal("changed work was not offered for fresh human review", review)
	}
	prepare := contractRequest(req.Run)
	prepare.IdempotencyKey = "accepted-review-" + strings.TrimPrefix(review.EventID, "coord-event-")
	prepare.ImplementationContext = *review.Selection
	endpoint := "/v1/product/projects/" + p.ID + "/orchestration-contracts"
	prepared := contractResponse(t, journeyRequest(t, h, http.MethodPost, endpoint+"/prepare", prepare, http.StatusCreated).Body.Bytes())
	if prepared.Freshness.Status != "current" || prepared.Contract.Approval != nil {
		t.Fatal("changed contract not reviewable", prepared)
	}
	approved := contractResponse(t, journeyRequest(t, h, http.MethodPost, endpoint+"/"+prepared.Contract.ID+"/approve", map[string]any{"expectedRevision": prepared.Contract.Revision, "expectedDigest": prepared.Contract.Digest}, http.StatusOK).Body.Bytes())
	if approved.Freshness.Status != "current" || approved.Contract.Approval == nil {
		t.Fatal("changed contract not approved", approved)
	}
	_, policy, ok := management.ActorFromContext(ctx)
	if !ok {
		t.Fatal("actor policy unavailable")
	}
	return func(parent context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
		if ref.UserID != "alice" || ref.ProjectID != p.ID {
			return nil, errCoordinatorAuthority
		}
		return management.WithExecutionAuthority(parent, management.ExecutionAuthority{Actor: management.User{ID: "alice", Role: "member"}, Policy: policy}), nil
	}
}

// The accepted amendment first reconciles exact source, waits for a fresh
// human-reviewed contract, then resumes through ordinary create, integration
// and combined checks. This uses production handlers and a controlled worker.
func TestAcceptedAmendmentReachesReviewedIntegrationAndIndependentCheck(t *testing.T) {
	s, p, h, ctx, req, _ := coordinatorContinuationFixture(t)
	base := "/v1/product/projects/" + p.ID
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	tasks := currentPlan(state, p.ID).Tasks
	tasks[0].WritePaths = []string{"result.txt"}
	tasks = append(tasks, ProjectTask{ID: "task-review", Title: "Independently review changed implementation", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-save", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect the exact candidate and report an independent check."})
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, h, http.MethodPost, base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, http.StatusOK).Body.Bytes(), &plan)
	commitRoute := base + "/git/commit"
	journeyRequest(t, h, http.MethodPost, commitRoute, reviewedCommitRequest(t, h, commitRoute, "Review task write scope"), http.StatusOK)
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, http.MethodPost, base+"/orchestrator/policy", map[string]any{"expectedRevision": req.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "integrationMode": "reviewed-scopes"}, http.StatusOK).Body.Bytes(), &policy)
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedContextDigest, req.ExpectedPolicyRevision = view.Context.Digest, policy.Revision
	req.Run.ImplementationContext.PlanRevision = plan.Revision
	anchor := coordinatorReadyFixture(t, s, p, ctx, req)
	priorRuns, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	selection := req.Run.ImplementationContext
	binding, err := bindProjectRequirements(state, p.ID, []string{"AC-save"}, &selection.SpecificationRevision, &selection.DesignPlanRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	check := &CoordinatorCheckPlan{Commands: []CheckCommand{{Label: "Combined result", Executable: "node", Args: []string{"-e", "console.log('checked')"}}}, RequirementIDs: binding.RequirementIDs, RequirementSnapshots: binding.RequirementSnapshots, AcknowledgeProjectCommands: true}
	installControlledProjectCheckRuntime(t, s, func(context.Context, ProjectCheckCommandRequest) (string, error) { return "checked", nil })
	s.mu.Lock()
	rows, err := s.readContinuations(p, "alice")
	if err == nil {
		rows.Rows[0].Status = "blocked"
		rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, CheckPlan: check, UpdatedAt: anchor.CreatedAt}
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Improve this implementation while building", "")
	amended := snapshot.Plan
	amended.Tasks[0].Acceptance = "The amended implementation passes combined checks."
	amended.Tasks[1].Acceptance = "Independently inspect the amended implementation and report checked findings."
	answer := OrchestratorAnswer{Summary: "Amend task", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "plan", Summary: "Amend task", Plan: &amended}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err = s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	_, currentPolicy, authorized := management.ActorFromContext(ctx)
	if !authorized {
		t.Fatal("fresh fixture policy missing")
	}
	// The production gateway returns the fresh complete policy. A name-only
	// actor context would correctly stale the source-bound approval.
	s.coordinatorBackground.authorize = func(parent context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
		if ref.UserID != "alice" || ref.ProjectID != p.ID || harness != "codex" || !secrets {
			return nil, errCoordinatorAuthority
		}
		return management.WithExecutionAuthority(parent, management.ExecutionAuthority{Actor: management.User{ID: "alice", Role: "member"}, Policy: currentPolicy}), nil
	}
	for i := 0; i < 2; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 || rows.Rows[0].ID != anchor.ID {
		t.Fatalf("changed plan reused old contract or minted work: %+v %v", rows, err)
	}
	if len(rows.ContextCommits) != 1 || rows.ContextCommits[0].Status != "confirmed" {
		t.Fatal("accepted amendment did not reconcile exact source before approval", rows.ContextCommits)
	}
	committed := rows.ContextCommits[0]
	s.mu.Lock()
	rows.ContextCommits[0].Status, rows.ContextCommits[0].Commit = "pending", ""
	err = s.saveContinuations(rows)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.ContextCommits) != 1 || rows.ContextCommits[0].Status != "confirmed" || rows.ContextCommits[0].Commit != committed.Commit || len(rows.Rows) != 1 {
		t.Fatal("exact pending effect did not settle by readback without new work", err, rows)
	}
	ambiguous := committed
	ambiguous.Status, ambiguous.Commit = "pending", ""
	ambiguous.PathDigests = make(map[string]string, len(committed.PathDigests))
	for path := range committed.PathDigests {
		ambiguous.PathDigests[path] = strings.Repeat("0", 64)
	}
	s.mu.Lock()
	rows.ContextCommits[0] = ambiguous
	err = s.saveContinuations(rows)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 || rows.ContextCommits[0].Status != "pending" {
		t.Fatal("ambiguous commit replayed or admitted work", err, rows)
	}
	currentHead, headErr := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD")
	if headErr != nil || strings.TrimSpace(currentHead) != committed.Commit {
		t.Fatal("ambiguous readback mutated the exact source", headErr)
	}
	s.mu.Lock()
	rows.ContextCommits[0] = committed
	err = s.saveContinuations(rows)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	runs, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil || len(runs) != len(priorRuns) {
		t.Fatal("accepted event created a new run without approval", runs, err)
	}
	for i := range runs {
		if runs[i].ID != priorRuns[i].ID {
			t.Fatal("accepted event replaced a run without approval", runs, priorRuns)
		}
	}
	var listed struct {
		AcceptedReview *CoordinatorAcceptedReview `json:"acceptedReview"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, http.MethodGet, base+"/orchestrator/continuations", nil, http.StatusOK).Body.Bytes(), &listed)
	if listed.AcceptedReview == nil || listed.AcceptedReview.Status != "review-required" || listed.AcceptedReview.Selection == nil || len(listed.AcceptedReview.Selection.TaskIDs) != 2 || listed.AcceptedReview.Selection.TaskIDs[0] != "task-review" || listed.AcceptedReview.Selection.TaskIDs[1] != "task-save" {
		t.Fatal("changed accepted event was not reviewable against committed source", listed.AcceptedReview)
	}
	prepare := contractRequest(req.Run)
	prepare.IdempotencyKey = "accepted-amendment-review-123456"
	prepare.ImplementationContext = *listed.AcceptedReview.Selection
	endpoint := base + "/orchestration-contracts"
	prepared := contractResponse(t, journeyRequest(t, h, http.MethodPost, endpoint+"/prepare", prepare, http.StatusCreated).Body.Bytes())
	if prepared.Freshness.Status != "current" || prepared.Contract.Approval != nil {
		t.Fatal("changed contract was not a fresh human review", prepared)
	}
	for i := 0; i < 2; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 {
		t.Fatal("preparation silently approved or launched work", err, rows)
	}
	approved := contractResponse(t, journeyRequest(t, h, http.MethodPost, endpoint+"/"+prepared.Contract.ID+"/approve", map[string]any{"expectedRevision": prepared.Contract.Revision, "expectedDigest": prepared.Contract.Digest}, http.StatusOK).Body.Bytes())
	if approved.Freshness.Status != "current" || approved.Contract.Approval == nil {
		t.Fatal("changed contract approval missing", approved)
	}
	if selected := s.acceptedEventApprovedContract(ctx, p, "alice", *listed.AcceptedReview.Selection, rows.SavedRoute, true); selected == nil {
		t.Fatalf("approved changed contract was not selectable: input=%+v route=%+v approved=%+v", listed.AcceptedReview.Selection, rows.SavedRoute.Route, approved.Contract.Input.Route)
	}
	for i := 0; i < 2; i++ {
		if err = s.RunCoordinatorOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 2 || len(rows.Rows[1].AcceptedEvents) != 1 || rows.Rows[1].Status != "queued" {
		t.Fatalf("approved amendment did not reserve affected work: %+v %v", rows, err)
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, rows.Rows[1].RunID)
	if err != nil || run.Input.OrchestrationContract == nil || run.Input.OrchestrationContract.ID != approved.Contract.ID || run.Input.Implementation.PlanRevision != snapshot.Plan.Revision+1 {
		t.Fatal("replacement run did not capture approved changed contract and context", err)
	}
	swarmBase := base + "/runs/" + run.ID + "/swarm"
	run = projectRunHTTPResponse(t, journeyRequest(t, h, http.MethodPost, swarmBase+"/tasks/task-save/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "amended-candidate-123456"}, http.StatusCreated)).ProjectRun
	op := run.Operations[len(run.Operations)-1]
	workerPath := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, workerPath, "result.txt", "independently integrated implementation\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, request TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		if request.OperationID != op.ID {
			t.Fatalf("unexpected candidate handoff %s", request.OperationID)
		}
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("accepted amended candidate"), Report: &TaskHandoffReport{Version: 1, Summary: "The changed implementation is ready for independent review.", ChangedFiles: []string{"result.txt"}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect result", Outcome: "passed", Detail: "Candidate committed."}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	journeyRequest(t, h, http.MethodPost, swarmBase+"/operations/"+op.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, http.StatusOK)
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var candidateReview struct {
		Review ProjectSwarmReview `json:"review"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, http.MethodGet, swarmBase+"/tasks/task-save/review", nil, http.StatusOK).Body.Bytes(), &candidateReview)
	if candidateReview.Review.CanIntegrate || candidateReview.Review.IndependentReviewState != "pending" {
		t.Fatal("candidate skipped its required independent review", candidateReview.Review)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, h, http.MethodPost, swarmBase+"/tasks/task-review/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "accepted-reviewer-123456"}, http.StatusCreated)).ProjectRun
	reviewer := run.Operations[len(run.Operations)-1]
	var obligation struct {
		Obligation ProjectSwarmReviewObligationView `json:"obligation"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, http.MethodPost, swarmBase+"/tasks/task-save/review-obligations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "accepted-obligation-123456", "candidateOperationId": op.ID, "reviewerOperationId": reviewer.ID, "reviewDigest": candidateReview.Review.ReviewDigest}, http.StatusCreated).Body.Bytes(), &obligation)
	if obligation.Obligation.CandidateOperationID != op.ID {
		t.Fatal("review obligation lost candidate identity", obligation)
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	assignment := swarmReviewAssignmentDigest(run, reviewer.ID)
	if assignment == "" {
		t.Fatal("reviewer did not receive exact candidate assignment")
	}
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, assignment)
	if err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, reviewer.ChildID, "succeeded", "Independent reviewer checked the amended candidate.")
	if err != nil {
		t.Fatal(err)
	}
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("accepted amended candidate review"), Report: &TaskHandoffReport{Version: 1, Summary: "The amended implementation satisfies the saved acceptance.", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect candidate diff", Outcome: "passed", Detail: "The amended acceptance is covered."}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	journeyRequest(t, h, http.MethodPost, swarmBase+"/operations/"+reviewer.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, http.StatusOK)
	for i := 0; i < 2; i++ {
		current, readErr := s.readContinuations(p, "alice")
		if readErr != nil {
			t.Fatal(readErr)
		}
		if advanced, outcomeErr := s.continueCoordinatorReviewOutcome(ctx, p, current.Rows[1]); outcomeErr != nil || !advanced {
			actual, _ := s.projectRuns.Get(p.ID, p.Path, run.ID)
			view, viewErr := s.projectRuns.taskHandoffView(actual, *swarmOperation(&actual, reviewer.ID))
			t.Fatalf("review finding or pass did not settle %d advanced=%v err=%v current=%v assignment=%s saved=%s handoff=%+v handoffErr=%v", i, advanced, outcomeErr, s.coordinatorIndependentReviewCurrent(ctx, p, current.Rows[1]), swarmReviewAssignmentDigest(actual, reviewer.ID), swarmOperation(&actual, reviewer.ID).ReviewAssignmentDigest, view, viewErr)
		}
	}
	current, readErr := s.readContinuations(p, "alice")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if integrated, integrationErr := s.continueCoordinatorCandidateIntegration(ctx, p, current.Rows[1]); integrationErr != nil || !integrated {
		actual, _ := s.projectRuns.Get(p.ID, p.Path, run.ID)
		review, reviewErr := s.swarmReview(ctx, p, actual, "task-save")
		latestPolicy, _ := s.readCoordinatorPolicy(p)
		t.Fatalf("integration=%v err=%v policy=%s/%s authority=%v contract=%v reuse=%v reviewErr=%v ready=%v obligation=%+v reviewerAssignment=%s resolvedAssignment=%s initialReview=%+v currentReview=%+v", integrated, integrationErr, latestPolicy.ExecutionMode, latestPolicy.IntegrationMode, s.continuationAuthority(ctx, p, current.Rows[1]), s.contractCurrentForRun(p, actual), s.validateProjectSwarmReuseCurrent(ctx, p, actual), reviewErr, swarmIndependentReviewReady(actual, "task-save", review), actual.Swarm.ReviewObligations[0], swarmOperation(&actual, reviewer.ID).ReviewAssignmentDigest, swarmResolvedReviewAssignmentDigest(actual.Swarm.ReviewObligations[0]), candidateReview.Review, review)
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || run.Swarm.Tasks[0].Integration == nil || run.Swarm.Tasks[0].Integration.State != "integrated" {
		t.Fatal("amended candidate did not integrate", err)
	}
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || rows.Rows[1].Delivery == nil || rows.Rows[1].Delivery.CheckID == "" {
		t.Fatal("amended combined result did not reserve independent check", err)
	}
}
