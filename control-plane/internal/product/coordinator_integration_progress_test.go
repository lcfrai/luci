package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// A pending review on the first task must not hold a later reviewed task.
// This fixture uses the normal contract/continuation, handoff, review and
// integration handlers. Workers are controlled; the final source check uses
// the existing bounded native-command fixture, not a fabricated passing exit.
func TestCoordinatorPendingReviewDoesNotStrandLaterReviewedIntegration(t *testing.T) {
	s, p, h, ctx, request, _ := coordinatorContinuationFixture(t)
	base := "/v1/product/projects/" + p.ID
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	tasks := []ProjectTask{
		{ID: "task-a", Title: "Implement first component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Alpha contains the reviewed result"},
		{ID: "task-b", Title: "Implement second component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"beta.txt"}, Acceptance: "Beta contains the reviewed result"},
		{ID: "task-ra", Title: "Independently review first component", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect the exact alpha candidate"},
		{ID: "task-rb", Title: "Independently review second component", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-b", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect the exact beta candidate"},
	}
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, 200).Body.Bytes(), &plan)
	commitRoute := base + "/git/commit"
	journeyRequest(t, h, "POST", commitRoute, reviewedCommitRequest(t, h, commitRoute, "Review two independent implementation scopes"), 200)
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/orchestrator/policy", map[string]any{"expectedRevision": request.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "executionMode": "reviewed-tasks", "integrationMode": "reviewed-scopes"}, 200).Body.Bytes(), &policy)
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedContextDigest, request.ExpectedPolicyRevision = view.Context.Digest, policy.Revision
	request.Run.ImplementationContext.PlanRevision = plan.Revision
	request.Run.ImplementationContext.TaskIDs = []string{"task-a", "task-b", "task-ra", "task-rb"}
	request.MaxParallel = 2
	prepare := contractRequest(request.Run)
	prepare.IdempotencyKey = "integration-progress-contract-123456"
	endpoint := base + "/orchestration-contracts"
	prepared := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	approved := contractResponse(t, journeyRequest(t, h, "POST", endpoint+"/"+prepared.Contract.ID+"/approve", map[string]any{"expectedRevision": prepared.Contract.Revision, "expectedDigest": prepared.Contract.Digest}, 200).Body.Bytes())
	request.Run.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	check := CheckCommand{Label: "Check both independently reviewed components", Executable: "node", Args: []string{"-e", "const fs=require('fs');for(const n of ['alpha','beta']){if(fs.readFileSync(n+'.txt','utf8')!==n+' reviewed\\n')process.exit(7)};console.log('both reviewed components verified')"}}
	s, p, h, ctx, continuation, starts := coordinatorDeliveryRequestFixture(t, s, p, h, ctx, request, check)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	rows, err := s.readContinuations(p, "alice")
	if err != nil || len(rows.Rows) != 1 {
		t.Fatal("missing exact continuation", err)
	}
	row := rows.Rows[0]
	run, err := s.projectRuns.Get(p.ID, p.Path, continuation.RunID)
	if err != nil {
		t.Fatal(err)
	}
	swarmBase := base + "/runs/" + run.ID + "/swarm"
	// A reservation receipt can retain an unknown allocation. Do not write or
	// claim that worktree as if HTTP 201 proved allocation succeeded. Keep the
	// real runner; retain bounded error context only for this controlled fixture.
	commandRunner := s.runner
	var allocationCommandMu sync.Mutex
	var allocationCommandError string
	s.runner = func(ctx context.Context, executable string, args []string, dir string) (string, error) {
		output, commandErr := commandRunner(ctx, executable, args, dir)
		if commandErr != nil {
			allocationCommandMu.Lock()
			allocationCommandError = commandErr.Error()
			if len(allocationCommandError) > 4096 {
				allocationCommandError = allocationCommandError[:4096]
			}
			allocationCommandMu.Unlock()
		}
		return output, commandErr
	}
	requireAllocation := func(taskID string) ProjectRunOperation {
		t.Helper()
		task := swarmTask(&run, taskID)
		if task == nil {
			t.Fatalf("reserved task %s is absent", taskID)
		}
		op := swarmOperation(&run, task.OperationID)
		if op == nil {
			t.Fatalf("reserved operation for %s is absent", taskID)
		}
		if run.Swarm.State != "ready" || op.Allocation != "ready" || op.Status != "reserved" {
			allocationCommandMu.Lock()
			commandError := allocationCommandError
			allocationCommandMu.Unlock()
			t.Fatalf("task %s cannot be claimed: swarm=%s allocation=%s status=%s; last native command error (possibly a benign probe): %s", taskID, run.Swarm.State, op.Allocation, op.Status, commandError)
		}
		return *op
	}
	reload := func() {
		t.Helper()
		run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	captureReport := func(op ProjectRunOperation, report *TaskHandoffReport) {
		t.Helper()
		s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
			if req.OperationID != op.ID || req.ChildID != op.ChildID {
				return TaskHandoffCapture{}, errors.New("foreign controlled task report")
			}
			return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest(op.Implementation), Report: report}, nil
		})
		journeyRequest(t, h, "POST", swarmBase+"/operations/"+op.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200)
		reload()
	}
	for i, name := range []string{"alpha", "beta"} {
		taskID := []string{"task-a", "task-b"}[i]
		allocationCommandMu.Lock()
		allocationCommandError = ""
		allocationCommandMu.Unlock()
		run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", swarmBase+"/tasks/"+taskID+"/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "integration-progress-" + taskID + "-123456"}, 201)).ProjectRun
		op := requireAllocation(taskID)
		swarmCommitFixture(t, mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID), name+".txt", name+" reviewed\n")
		run = swarmFinishFixture(t, s, p, run, op.ID)
		report := &TaskHandoffReport{Version: 1, Summary: name + " candidate requires independent review", ChangedFiles: []string{name + ".txt"}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect " + name, Outcome: "passed", Detail: "Worker claim only"}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}
		if taskID == "task-b" {
			report.AddressedMessages = []TaskHandoffAddressedMessage{{Kind: "discovery", RecipientTaskID: "task-a", Summary: "Beta uses its own reviewed file scope."}}
		}
		captureReport(op, report)
	}
	if len(run.Swarm.Messages) != 1 || run.Swarm.Messages[0].RecipientTaskID != "task-a" || run.Swarm.Messages[0].Sender.TaskID != "task-b" || run.Swarm.Messages[0].Attribution != "owned-worker-report" {
		t.Fatal("coding handoff did not retain its scoped attributed claim", run.Swarm.Messages)
	}
	reviewTask := func(taskID, reviewerID string) {
		t.Helper()
		allocationCommandMu.Lock()
		allocationCommandError = ""
		allocationCommandMu.Unlock()
		run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", swarmBase+"/tasks/"+reviewerID+"/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "integration-progress-" + reviewerID + "-123456"}, 201)).ProjectRun
		candidate := *swarmOperation(&run, swarmTask(&run, taskID).OperationID)
		reviewer := requireAllocation(reviewerID)
		review, reviewErr := s.swarmReview(ctx, p, run, taskID)
		if reviewErr != nil || !review.CanIntegrate {
			t.Fatal("candidate cannot receive independent review", reviewErr, review)
		}
		journeyRequest(t, h, "POST", swarmBase+"/tasks/"+taskID+"/review-obligations", swarmReviewCreateRequest{ExpectedRevision: run.Revision, IdempotencyKey: "integration-progress-review-" + taskID, CandidateOperationID: candidate.ID, ReviewerOperationID: reviewer.ID, ReviewDigest: review.ReviewDigest}, 201)
		reload()
		capture, captureErr := s.CaptureProjectRunTaskContext(ctx, p.ID, run.ID, reviewer.ID, reviewer.WorktreeID, mustSwarmPathFixture(t, s, p.ID, run.ID, reviewer.ID), *ImplementationContextSelection(reviewer.Implementation))
		if captureErr != nil || capture == nil || capture.ReviewAssignmentDigest == "" {
			t.Fatal("reviewer did not receive its exact assignment", captureErr)
		}
		delivered := false
		for _, file := range capture.Files {
			if file.Path == "LUCI.md" && strings.Contains(string(file.Data), candidate.ID) && strings.Contains(string(file.Data), review.Patch) {
				delivered = true
			}
		}
		if !delivered {
			t.Fatal("reviewer's captured file omitted the exact candidate patch")
		}
		run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, capture.ReviewAssignmentDigest)
		if err != nil {
			t.Fatal(err)
		}
		run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, reviewer.ChildID, "succeeded", "Controlled independent reviewer stopped after inspecting the candidate.")
		if err != nil {
			t.Fatal(err)
		}
		captureReport(reviewer, &TaskHandoffReport{Version: 1, Summary: "Exact candidate satisfies its reviewed scope", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect exact candidate patch", Outcome: "passed", Detail: "Independent worker claim; combined checks remain required"}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}})
		for i := 0; i < 2; i++ {
			advanced, outcomeErr := s.continueCoordinatorReviewOutcome(ctx, p, row)
			if outcomeErr != nil || !advanced {
				t.Fatal("exact reviewer finding/resolution did not progress", i, outcomeErr, advanced)
			}
		}
		reload()
	}
	reviewTask("task-b", "task-rb")
	if run.Swarm.Tasks[0].ID != "task-a" {
		t.Fatal("fixture must put the pending task first", run.Swarm.Tasks)
	}
	firstReview, firstErr := s.swarmReview(ctx, p, run, "task-a")
	laterReview, laterErr := s.swarmReview(ctx, p, run, "task-b")
	if firstErr != nil || laterErr != nil || !firstReview.CanIntegrate || !laterReview.CanIntegrate || swarmIndependentReviewReady(run, "task-a", firstReview) || !swarmIndependentReviewReady(run, "task-b", laterReview) {
		t.Fatal("fixture did not isolate a task-local pending review ahead of a ready candidate", firstErr, laterErr)
	}
	before := implementationDigest(run)
	if advanced, foreignErr := s.continueCoordinatorCandidateIntegration(orchestratorActor(t, "bob"), p, row); foreignErr != nil || advanced {
		t.Fatal("foreign actor advanced integration", foreignErr, advanced)
	}
	reload()
	if implementationDigest(run) != before {
		t.Fatal("refused actor changed the retained run")
	}
	advanced, err := s.continueCoordinatorCandidateIntegration(ctx, p, row)
	if err != nil || !advanced {
		t.Fatal("the first task's pending review stranded a later reviewed candidate", err, advanced)
	}
	reload()
	a, b := swarmTask(&run, "task-a"), swarmTask(&run, "task-b")
	if a.Integration != nil || b.Integration == nil || b.Integration.State != "integrated" || len(run.Checks) != 0 {
		t.Fatal("integration accepted unreviewed work or lost the ready candidate", run)
	}
	if _, statErr := os.Stat(filepath.Join(p.Path, "alpha.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("unreviewed alpha reached the target checkout", statErr)
	}
	data, readErr := os.ReadFile(filepath.Join(p.Path, "beta.txt"))
	if readErr != nil || string(data) != "beta reviewed\n" {
		t.Fatal("reviewed beta was not actually integrated", readErr)
	}
	row, err = s.continueCoordinatorDelivery(ctx, p, row)
	if err != nil || row.Delivery != nil || starts.Load() != 0 {
		t.Fatal("partial integration started or claimed combined checks", err, row.Delivery, starts.Load())
	}
	before = implementationDigest(run)
	if advanced, repeatErr := s.continueCoordinatorCandidateIntegration(ctx, p, row); repeatErr != nil || advanced {
		t.Fatal("repeated observation replayed integration", repeatErr, advanced)
	}
	reload()
	if implementationDigest(run) != before {
		t.Fatal("no-progress observation changed immutable integration")
	}
	reviewTask("task-a", "task-ra")
	if advanced, integrationErr := s.continueCoordinatorCandidateIntegration(ctx, p, row); integrationErr != nil || !advanced {
		t.Fatal("first candidate did not integrate after its own independent review", integrationErr, advanced)
	}
	row, err = s.continueCoordinatorDelivery(ctx, p, row)
	if err != nil || row.Delivery == nil || row.Delivery.CheckID == "" {
		t.Fatal("fully reviewed integration did not reach the real check handler", err, row.Delivery)
	}
	result := finishEvidenceRun(t, s, p.ID, row.Delivery.CheckID)
	row, err = s.continueCoordinatorDelivery(ctx, p, row)
	reload()
	if err != nil || result.Status != "passed" || row.Delivery.Status != "verified" || starts.Load() != 1 || len(run.Checks) != 1 || len(run.Swarm.ReviewObligations) != 2 {
		t.Fatal("exact reviewed combined source did not reach checked completion", err, result, row.Delivery, starts.Load(), run.Checks)
	}
}
