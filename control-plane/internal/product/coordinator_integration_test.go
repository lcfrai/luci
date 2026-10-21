package product

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestCoordinatorReviewedScopeIntegratesStoppedCandidateBeforeIndependentChecks(t *testing.T) {
	s, p, h, ctx, request, _ := coordinatorContinuationFixture(t)
	base := "/v1/product/projects/" + p.ID
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	tasks := currentPlan(state, p.ID).Tasks
	tasks[0].Role, tasks[0].WritePaths = "coding", []string{"result.txt"}
	tasks = append(tasks, ProjectTask{ID: "task-review", Title: "Independently review integrated source", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-save", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect the exact candidate before background integration."})
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, 200).Body.Bytes(), &plan)
	commitRoute := base + "/git/commit"
	journeyRequest(t, h, "POST", commitRoute, reviewedCommitRequest(t, h, commitRoute, "Review candidate write scope"), 200)
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/orchestrator/policy", map[string]any{"expectedRevision": request.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "integrationMode": "reviewed-scopes"}, 200).Body.Bytes(), &policy)
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Continue independently reviewed background integration", "")
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &OrchestratorAnswer{Summary: "Integrate only the exact independently reviewed candidate.", Options: []string{}, Impact: []string{}, NextAction: "build"}); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedContextDigest, request.ExpectedPolicyRevision = view.Context.Digest, policy.Revision
	request.OriginTurnID = turn.ID
	request.Run.ImplementationContext.PlanRevision = plan.Revision
	request.Run.ImplementationContext.TaskIDs = []string{"task-save", "task-review"}
	prepare := contractRequest(request.Run)
	prepare.IdempotencyKey = "background-integration-contract-123456"
	endpoint := base + "/orchestration-contracts"
	prepared := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	approved := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/"+prepared.Contract.ID+"/approve", map[string]any{"expectedRevision": prepared.Contract.Revision, "expectedDigest": prepared.Contract.Digest}, 200).Body.Bytes())
	request.Run.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	s, p, h, ctx, continuation, _ := coordinatorDeliveryRequestFixture(t, s, p, h, ctx, request)
	run, err := s.projectRuns.Get(p.ID, p.Path, continuation.RunID)
	if err != nil {
		t.Fatal(err)
	}
	swarmBase := base + "/runs/" + run.ID + "/swarm"
	run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", swarmBase+"/tasks/task-save/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "candidate-integration-123456"}, 201)).ProjectRun
	op := run.Operations[len(run.Operations)-1]
	workerPath := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, workerPath, "result.txt", "independently integrated implementation\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	// Keep actual background integration as the assertion: only the controlled
	// candidate/reviewer outcomes are supplied through the normal report routes.
	captureReport := func(operation ProjectRunOperation, report *TaskHandoffReport) TaskHandoffView {
		t.Helper()
		s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
			if req.OperationID != operation.ID || req.ChildID != operation.ChildID {
				return TaskHandoffCapture{}, errors.New("foreign controlled integration report")
			}
			return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest(operation.Implementation), Report: report}, nil
		})
		var response struct {
			Handoff TaskHandoffView `json:"handoff"`
		}
		decodeEvidenceResponse(t, journeyRequest(t, h, "POST", swarmBase+"/operations/"+operation.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &response)
		run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		return response.Handoff
	}
	captureReport(op, &TaskHandoffReport{Version: 1, Summary: "Candidate source awaits independent review.", ChangedFiles: []string{"result.txt"}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect result.txt", Outcome: "passed", Detail: "Candidate report only."}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}})
	var candidate struct {
		Review ProjectSwarmReview `json:"review"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", swarmBase+"/tasks/task-save/review", nil, 200).Body.Bytes(), &candidate)
	if candidate.Review.CanIntegrate || candidate.Review.IndependentReviewState != "pending" || swarmTask(&run, "task-save").Integration != nil {
		t.Fatal("candidate bypassed independent review", candidate.Review)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", swarmBase+"/tasks/task-review/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "background-independent-review-123456"}, 201)).ProjectRun
	reviewer := run.Operations[len(run.Operations)-1]
	var obligation struct {
		Obligation ProjectSwarmReviewObligationView `json:"obligation"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", swarmBase+"/tasks/task-save/review-obligations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "background-review-obligation-123456", "candidateOperationId": op.ID, "reviewerOperationId": reviewer.ID, "reviewDigest": candidate.Review.ReviewDigest}, 201).Body.Bytes(), &obligation)
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	assignment := swarmReviewAssignmentDigest(run, reviewer.ID)
	if reviewer.ID == op.ID || reviewer.TaskID == op.TaskID || assignment == "" {
		t.Fatal("independent reviewer lost exact candidate assignment")
	}
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, assignment)
	if err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, reviewer.ChildID, "succeeded", "Controlled independent reviewer completed.")
	if err != nil {
		t.Fatal(err)
	}
	handoff := captureReport(reviewer, &TaskHandoffReport{Version: 1, Summary: "The exact candidate satisfies the saved acceptance.", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect candidate diff", Outcome: "passed", Detail: "Independent source review; combined checks still required."}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}})
	var finding struct {
		Message     ProjectSwarmMessage `json:"message"`
		RunRevision int                 `json:"runRevision"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", swarmBase+"/operations/"+reviewer.ID+"/messages", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "background-review-finding-123456", "recipientTaskId": "task-save", "kind": "review-finding", "evidence": map[string]any{"kind": "handoff", "id": reviewer.ID, "digest": handoff.Digest}, "summary": "Independent reviewer accepted this exact candidate."}, 200).Body.Bytes(), &finding)
	journeyRequest(t, h, "POST", swarmBase+"/review-obligations/"+obligation.Obligation.ID+"/resolve", map[string]any{"expectedRevision": finding.RunRevision, "outcome": "passed", "reviewerHandoffDigest": handoff.Digest, "findingMessageId": finding.Message.ID}, 200)
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || swarmTask(&run, "task-save").Integration != nil {
		t.Fatal("fixture manually integrated before background continuation", err)
	}
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	coding := swarmTask(&run, "task-save")
	if err != nil || coding == nil || coding.Integration == nil || coding.Integration.State != "integrated" {
		t.Fatalf("reviewed candidate did not integrate: %+v %v", run, err)
	}
	commit := coding.Integration.Commit
	if err = s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	coding = swarmTask(&run, "task-save")
	if err != nil || coding == nil || coding.Integration == nil || coding.Integration.Commit != commit {
		t.Fatal("replayed reviewed integration", err)
	}
	rows, err := s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 || rows.Rows[0].Delivery == nil || rows.Rows[0].Delivery.CheckID == "" {
		t.Fatal("integrated source did not reserve independent check", err)
	}
	// Reservation starts an asynchronous check. Observe its exact durable final
	// result before fixture teardown can remove source or private history files.
	check := waitProjectCheck(t, s, p.ID, rows.Rows[0].Delivery.CheckID)
	if check.Status != "passed" || check.Commit != commit || check.SourceChanged || check.ResourcesHeld == nil || *check.ResourcesHeld || len(check.Commands) != 1 {
		t.Fatalf("integrated check did not settle against its exact source: status=%s commit=%s want=%s sourceChanged=%t resourcesKnown=%t resourcesHeld=%t commands=%d", check.Status, check.Commit, commit, check.SourceChanged, check.ResourcesHeld != nil, check.ResourcesHeld != nil && *check.ResourcesHeld, len(check.Commands))
	}
	command := check.Commands[0]
	if command.Status != "passed" || !command.CleanupConfirmed || command.ExitCode == nil || *command.ExitCode != 0 {
		t.Fatalf("integrated check command did not finish with confirmed cleanup: status=%s cleanup=%t exit=%v", command.Status, command.CleanupConfirmed, command.ExitCode)
	}
}
