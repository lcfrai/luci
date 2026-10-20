package product

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestCheckFailureRepairRequiresTypedSettledCommandSuffix(t *testing.T) {
	zero, failure, held := 0, 7, false
	commands := []CheckCommandResult{{AllocationID: "checkexec-first", Status: "passed", ExitCode: &zero, CleanupConfirmed: true}, {AllocationID: "checkexec-failed", Status: "failed", ExitCode: &failure, CleanupConfirmed: true}, {AllocationID: "checkexec-never-entered", Status: "skipped"}}
	row := ProjectCheckRun{Status: "failed", ResourcesHeld: &held, Commands: commands}
	if settledFailedCommands(row.Commands) { t.Fatal("ambiguous old skipped receipt granted repair") }
	markCheckFailureNotStartedSuffix(&row)
	if !row.Commands[2].NotStartedAfterFailure || row.Commands[2].ExitCode != nil || row.Commands[2].CleanupConfirmed || !settledFailedCommands(row.Commands) { t.Fatal("typed never-entered suffix became execution evidence", row) }
	for _, boundary := range []string{"unknown", "cancelled", "blocked", "held", "missing-held", "cleanup", "zero-exit", "missing-exit", "late-execution", "early-skip"} {
		t.Run(boundary, func(t *testing.T) {
			value := snapshotProjectCheck(row)
			switch boundary {
			case "unknown", "cancelled", "blocked": value.Status = boundary; value.Commands[1].Status = boundary; value.Commands[1].ExitCode = nil
			case "held": yes := true; value.ResourcesHeld = &yes
			case "missing-held": value.ResourcesHeld = nil
			case "cleanup": value.Commands[1].CleanupConfirmed = false
			case "zero-exit": value.Commands[1].ExitCode = &zero
			case "missing-exit": value.Commands[1].ExitCode = nil
			case "late-execution": value.Commands[2].Status = "passed"; value.Commands[2].ExitCode = &zero; value.Commands[2].CleanupConfirmed = true
			case "early-skip": value.Commands[0].Status = "skipped"; value.Commands[0].ExitCode = nil; value.Commands[0].NotStartedAfterFailure = true
			}
			if value.Status == "failed" && value.ResourcesHeld != nil && !*value.ResourcesHeld && settledFailedCommands(value.Commands) { t.Fatal("unsettled, reordered or pre-start receipt authorized repair", boundary) }
		})
	}
}

func TestCheckFailureRepairLegacyBytesAndFreshCheckIdentity(t *testing.T) {
	legacy := OrchestrationControls{}
	before, _ := json.Marshal(legacy)
	legacy.CheckFailureRepairs = false
	after, _ := json.Marshal(legacy)
	if string(before) != string(after) || strings.Contains(string(after), "checkFailureRepairs") { t.Fatal("default-off changed legacy canonical bytes") }
	legacy.CheckFailureRepairs = true
	enabled, _ := json.Marshal(legacy)
	if string(enabled) == string(before) { t.Fatal("explicit allowance was not digest-bound") }
	seen := map[string]bool{}
	for ordinal := 0; ordinal <= 2; ordinal++ {
		id := coordinatorDeliveryCheckID("continue-exact", ordinal)
		if seen[id] || ordinal == 0 && id != coordinatorCheckID("continue-exact") { t.Fatal("failed check identity was reused", ordinal) }
		seen[id] = true
	}
}

func TestCheckFailureRepairSharesTaskBudgetAndRejectsOldReview(t *testing.T) {
	limit := 2
	commit := strings.Repeat("a", 40)
	wave := ProjectSwarmCheckRepair{ID: "check-repair-one", CheckID: "check-one", Commit: commit, Tasks: []ProjectSwarmCheckRepairTask{{TaskID: "task-a", SourceOperationID: "op-old", BudgetOrdinal: 2, OperationID: "op-repair"}}}
	origin := swarmCheckRepairOrigin(wave, wave.Tasks[0])
	run := ProjectRun{Input: ProjectRunInput{OrchestrationContract: &OrchestrationContract{Input: OrchestrationPreparedInput{Controls: OrchestrationControls{CheckFailureRepairs: true, AutomaticRepairLimit: &limit}}}}, Swarm: &ProjectSwarm{Tasks: []ProjectSwarmTask{{ID: "task-a", OperationID: "op-repair"}}, CheckRepairs: []ProjectSwarmCheckRepair{wave}, ReviewObligations: []ProjectSwarmReviewObligation{{TaskID: "task-a", State: "repair-required", CandidateOperationID: "op-first", RepairOperationID: "op-old"}}}, Operations: []ProjectRunOperation{{ID: "op-old", TaskID: "task-a"}, {ID: "op-repair", TaskID: "task-a", CheckRepair: origin, Status: "unknown"}}}
	if swarmRepairsConsumed(run, "task-a") != 2 || swarmRepairBudgetForOperation(run, "op-repair") != nil { t.Fatal("shared budget or immutable unknown reservation lost") }
	if swarmReviewRepairSuccessor(run, "task-a", run.Operations[1], commit) == nil { t.Fatal("old failed combined bytes received another passing review") }
	if swarmReviewRepairSuccessor(run, "task-a", run.Operations[1], strings.Repeat("b", 40)) != nil { t.Fatal("changed exact check-origin candidate could not get fresh review") }
	if len(coordinatorRepairReviewCandidates(run)) != 2 || len(run.Swarm.ReviewObligations) != 1 { t.Fatal("scheduling manufactured a persisted reviewer finding") }
	limit = 1
	if swarmRepairBudgetForOperation(run, "op-repair") == nil { t.Fatal("budget reduction allowed new work") }
	limit = 2
	run.Swarm.Tasks[0].OperationID = "op-other"
	if swarmRepairBudgetForOperation(run, "op-repair") == nil { t.Fatal("obsolete wave candidate admitted again") }
}

func TestCheckRepairCoverageExcludesUnrelatedBudgetAndKeepsParallelDependencies(t *testing.T) {
	commit := strings.Repeat("a", 40)
	integration := func() *ProjectSwarmIntegration { return &ProjectSwarmIntegration{State: "integrated", Commit: commit} }
	run := ProjectRun{Input: ProjectRunInput{Requirements: []RequirementSnapshot{{ID: "AC-covered"}, {ID: "AC-other"}}, OrchestrationContract: &OrchestrationContract{Input: OrchestrationPreparedInput{Tasks: []ProjectTask{{ID: "task-a", RequirementIDs: []string{"AC-covered"}}, {ID: "task-b", RequirementIDs: []string{"AC-covered"}}, {ID: "task-c", RequirementIDs: []string{"AC-covered"}, DependsOn: []string{"task-a"}}, {ID: "task-other", RequirementIDs: []string{"AC-other"}}}}}}, Swarm: &ProjectSwarm{State: "ready", MaxParallel: 4, Tasks: []ProjectSwarmTask{{ID: "task-a", OperationID: "op-a", Integration: integration(), WritePaths: []string{"a.txt"}}, {ID: "task-b", OperationID: "op-b", Integration: integration(), WritePaths: []string{"b.txt"}}, {ID: "task-c", OperationID: "op-c", Integration: integration(), DependsOn: []string{"task-a"}, WritePaths: []string{"c.txt"}}, {ID: "task-other", OperationID: "op-other", Integration: integration(), WritePaths: []string{"other.txt"}}}, ReviewObligations: []ProjectSwarmReviewObligation{{TaskID: "task-other", State: "repair-required", RepairOperationID: "op-spent"}, {TaskID: "task-other", State: "repair-required", RepairOperationID: "op-spent-again"}}}}
	selected, err := checkRepairCoveredTasks(run, []string{"AC-covered"})
	if err != nil || !reflect.DeepEqual(selected, []string{"task-a", "task-b", "task-c"}) || swarmRepairsConsumed(run, "task-other") != 2 { t.Fatal("unrelated exhausted scope vetoed or expanded check repair", selected, err) }
	wave := ProjectSwarmCheckRepair{ID: "check-repair-wave", Commit: commit}
	for _, id := range selected { task := swarmTask(&run, id); wave.Tasks = append(wave.Tasks, ProjectSwarmCheckRepairTask{TaskID: id, SourceOperationID: task.OperationID, SourceIntegration: *task.Integration, BudgetOrdinal: 1}) }
	run.Swarm.CheckRepairs = []ProjectSwarmCheckRepair{wave}
	if swarmCheckRepairReady(run, *swarmTask(&run, "task-a"), wave.ID) != nil || swarmCheckRepairReady(run, *swarmTask(&run, "task-b"), wave.ID) != nil || swarmCheckRepairReady(run, *swarmTask(&run, "task-c"), wave.ID) == nil { t.Fatal("independent members were serialized or repaired dependency was ignored") }
	run.Swarm.CheckRepairs[0].Tasks[0].OperationID = "op-new-a"
	swarmTask(&run, "task-a").OperationID = "op-new-a"
	if swarmCheckRepairReady(run, *swarmTask(&run, "task-c"), wave.ID) != nil { t.Fatal("new integrated dependency did not release its dependent") }
	if _, err := checkRepairCoveredTasks(run, []string{"AC-unknown"}); err == nil { t.Fatal("unknown requirement granted task scope") }
}

func TestCheckRepairCompletionFollowsOnlyExactGenuineReviewRepair(t *testing.T) {
	commit := strings.Repeat("a", 40)
	member := ProjectSwarmCheckRepairTask{TaskID: "task-a", OperationID: "op-check-repair"}
	run := ProjectRun{Swarm: &ProjectSwarm{Tasks: []ProjectSwarmTask{{ID: "task-a", OperationID: "op-review-repair", Integration: &ProjectSwarmIntegration{State: "integrated"}}}, ReviewObligations: []ProjectSwarmReviewObligation{{TaskID: "task-a", CandidateOperationID: "op-check-repair", CandidateChildID: "child-candidate", CandidateCommit: commit, State: "repair-required", RepairOperationID: "op-review-repair"}}}, Operations: []ProjectRunOperation{{ID: "op-review-repair", TaskID: "task-a", Checkpoint: &ProjectSwarmCheckpoint{SourceOperationID: "op-check-repair", SourceChildID: "child-candidate", SourceCommit: commit}}}}
	if !checkRepairMemberIntegrated(run, member) { t.Fatal("genuine remaining-budget review repair could not complete the wave") }
	run.Operations[0].Checkpoint.SourceCommit = strings.Repeat("b", 40)
	if checkRepairMemberIntegrated(run, member) { t.Fatal("unrelated integrated retry discharged a check wave") }
}

func TestCheckRepairReviewerReplayIsLocalToEachIndependentCandidate(t *testing.T) {
	firstKey := coordinatorReviewerRetryKey("wave-one", "op-candidate-a")
	secondKey := coordinatorReviewerRetryKey("wave-one", "op-candidate-b")
	for _, state := range []string{"queued", "unknown", "completed"} {
		t.Run(state, func(t *testing.T) {
			items := []swarmQueueRecord{{SwarmQueueItem: SwarmQueueItem{ProjectID: "project-one", RunID: "run-one", TaskID: "reviewer-a", State: state}, BatchKeyDigest: reviewDigest(firstKey), AutoReviewContinuationID: "continue-one", AutoReviewActorID: "owner", AutoReviewCandidateID: "op-candidate-a"}}
			first, err := coordinatorReviewerRetryExists(items, "project-one", "run-one", "continue-one", "owner", "reviewer-a", "op-candidate-a", firstKey)
			if err != nil || !first { t.Fatal("exact queued/uncertain/settled candidate was duplicated", err) }
			second, err := coordinatorReviewerRetryExists(items, "project-one", "run-one", "continue-one", "owner", "reviewer-b", "op-candidate-b", secondKey)
			if err != nil || second { t.Fatal("first retained candidate starved another independent reviewer", err) }
			if _, err = coordinatorReviewerRetryExists(items, "project-one", "run-one", "continue-one", "owner", "reviewer-b", "op-candidate-b", firstKey); err == nil { t.Fatal("another reviewer adopted a retained candidate key") }
		})
	}
}

// Real owned Git/HTTP/store/review/check flow, with controlled worker outcomes.
// No provider, Docker or semantic correctness claim is made by this fixture.
func TestCoordinatorCheckFailureRepairFreshSourceReviewIntegrationAndCheck(t *testing.T) {
	s, p, h, ctx, request, _ := coordinatorContinuationFixture(t)
	base := "/v1/product/projects/" + p.ID
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil { t.Fatal(err) }
	tasks := currentPlan(state, p.ID).Tasks
	tasks[0].Role, tasks[0].WritePaths = "coding", []string{"result.txt"}
	tasks = append(tasks, ProjectTask{ID: "task-review", Title: "Review exact repaired source", Role: "reviewer", ReviewOfTaskID: "task-save", RequirementIDs: []string{"AC-save"}, Acceptance: "Independently inspect the changed candidate."})
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, 200).Body.Bytes(), &plan)
	journeyRequest(t, h, "POST", base+"/git/commit", reviewedCommitRequest(t, h, base+"/git/commit", "Save bounded check repair scopes"), 200)
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/orchestrator/policy", map[string]any{"expectedRevision": request.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "executionMode": "reviewed-tasks", "integrationMode": "reviewed-scopes", "automaticRepairLimit": 1, "checkFailureRepairs": true}, 200).Body.Bytes(), &policy)
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Continue exact bounded check repair", "")
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &OrchestratorAnswer{Summary: "Use the saved check repair allowance.", Options: []string{}, Impact: []string{}, NextAction: "build"}); err != nil { t.Fatal(err) }
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil { t.Fatal(err) }
	request.OriginTurnID, request.ExpectedContextDigest, request.ExpectedPolicyRevision = turn.ID, view.Context.Digest, policy.Revision
	request.Run.ImplementationContext.PlanRevision = plan.Revision
	request.Run.ImplementationContext.TaskIDs = []string{"task-review", "task-save"}
	prepare := contractRequest(request.Run); prepare.IdempotencyKey = "check-repair-contract-123456"
	prepared := contractResponse(t, journeyRequest(t, h, "POST", base+"/orchestration-contracts/prepare", prepare, 201).Body.Bytes())
	approved := contractResponse(t, journeyRequest(t, h, "POST", base+"/orchestration-contracts/"+prepared.Contract.ID+"/approve", map[string]any{"expectedRevision": prepared.Contract.Revision, "expectedDigest": prepared.Contract.Digest}, 200).Body.Bytes())
	request.Run.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	command := CheckCommand{Label: "Exact repaired result", Executable: "node", Args: []string{"-e", "if(require('fs').readFileSync('result.txt','utf8')!=='repaired combined check\\n')process.exit(7)"}}
	_, _, _, _, out, starts := coordinatorDeliveryRequestFixture(t, s, p, h, ctx, request, command, CheckCommand{Label: "Following command", Executable: "node", Args: []string{"-e", "process.exit(0)"}})
	initial := coordinatorIntegrateFixture(t, s, p, h, out)
	// This fixture integrated its controlled children through explicit task
	// routes. Cancel only the original, never-admitted automatic queue intents;
	// do not fabricate execution or retirement receipts for them.
	originalQueue, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil { t.Fatal(err) }
	for _, item := range originalQueue.Items { if item.RunID == initial.ID { journeyRequest(t, h, "POST", base+"/swarm-queue/"+item.ID+"/cancel", map[string]any{"expectedRevision": item.Revision}, 200) } }
	inputDigest := queueInputDigest(initial)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	failed := finishEvidenceRun(t, s, p.ID, out.Delivery.CheckID)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	if failed.Status != "failed" || !failed.Commands[1].NotStartedAfterFailure || starts.Load() != 1 { t.Fatal("fixture did not retain failed command plus unstarted suffix", failed) }
	readRow := func() coordinatorContinuationRecord { rows, e := s.readContinuations(p, "alice"); if e != nil { t.Fatal(e) }; return rows.Rows[findContinuation(rows, out.ID)] }
	row := readRow()
	for _, boundary := range []string{"unknown", "cancelled", "blocked", "held", "missing-cleanup", "changed-source", "foreign-check", "foreign-binding", "ambiguous-skip"} {
		t.Run(boundary, func(t *testing.T) {
			check := snapshotProjectCheck(failed)
			switch boundary {
			case "unknown", "cancelled", "blocked": check.Status = boundary
			case "held": yes := true; check.ResourcesHeld = &yes
			case "missing-cleanup": check.Commands[0].CleanupConfirmed = false
			case "changed-source": check.SourceChanged = true
			case "foreign-check": check.ID = "check-foreign"
			case "foreign-binding": check.CoordinatorOriginDigest = strings.Repeat("f", 64)
			case "ambiguous-skip": check.Commands[1].NotStartedAfterFailure = false
			}
			if coordinatorCheckRepairEligible(initial, row, check) { t.Fatal("ineligible receipt authorized a new repair", boundary) }
		})
	}
	row, handled, err := s.continueCoordinatorCheckRepair(ctx, p, row)
	if err != nil || !handled { t.Fatal("no durable wave", err) }
	run, err := s.projectRuns.Get(p.ID, p.Path, out.RunID)
	if err != nil || len(run.Swarm.CheckRepairs) != 1 || len(run.Operations) != len(initial.Operations) || swarmRepairsConsumed(run, "task-save") != 1 || queueInputDigest(run) != inputDigest { t.Fatal("wave changed old input, copied early or failed to charge budget", err) }
	wave := run.Swarm.CheckRepairs[0]
	restarted, _ := coordinatorDeliveryReopen(t, s)
	reopened, err := restarted.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || len(reopened.Operations) != len(initial.Operations) || !reflect.DeepEqual(reopened.Swarm.CheckRepairs, run.Swarm.CheckRepairs) || swarmRepairsConsumed(reopened, "task-save") != 1 { t.Fatal("restart lost a charged durable intent or allocated a replacement", err) }
	if wave.Commit != failed.Commit || wave.Commit == run.Input.Implementation.SourceCommit { t.Fatal("repair rebound old isolated input instead of failed combined source") }
	row, _, err = s.continueCoordinatorCheckRepair(ctx, p, row)
	if err != nil || row.Status != "queued" { t.Fatal(err) }
	row, _, err = s.continueCoordinatorCheckRepair(ctx, p, row)
	if err != nil { t.Fatal(err) }
	ledger, err := s.readSwarmScheduler()
	if err != nil { t.Fatal(err) }
	var queued swarmQueueRecord
	for _, item := range ledger.Items { if item.CheckRepairWaveID == wave.ID { queued = item } }
	if queued.ID == "" { t.Fatal("wave has no exact scheduler intent") }
	// Exercise the same internal origin used by scheduler reserve; no native
	// provider is started by this controlled source/review fixture.
	gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "check-repair"})
	gated = context.WithValue(gated, coordinatorCheckRepairContextKey{}, wave.ID)
	run, candidate, err := s.reserveSwarmTask(gated, p, run.ID, "task-save", schedulerReserveKey(queued), run.Revision)
	if err != nil || candidate.BaseCommit != failed.Commit || candidate.Checkpoint != nil || candidate.CheckRepair == nil || queueInputDigest(run) != inputDigest { t.Fatal("fresh repair checkout lost exact failed base", candidate, err) }
	before := len(run.Operations)
	if replay, prior, e := s.reserveSwarmTask(gated, p, run.ID, "task-save", schedulerReserveKey(queued), run.Revision); e != nil || len(replay.Operations) != before || prior.ID != candidate.ID { t.Fatal("reservation replay copied another checkout", e) }
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, candidate.ID)
	swarmCommitFixture(t, path, "result.txt", "repaired combined check\n")
	run = swarmFinishFixture(t, s, p, run, candidate.ID)
	collectCheckRepairHandoff(t, s, p, h, run, candidate.ID, []string{"result.txt"})
	run, _ = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if swarmAllIntegrated(run) { t.Fatal("old reviewer passed a fresh candidate") }
	if _, err = s.continueCoordinatorReviewerRetry(ctx, p, row); err != nil { t.Fatal(err) }
	ledger, _ = s.readSwarmScheduler()
	var reviewQueue swarmQueueRecord
	for _, item := range ledger.Items { if item.AutoReviewCandidateID == candidate.ID { reviewQueue = item } }
	if reviewQueue.ID == "" { t.Fatal("no new independent reviewer queued") }
	run, reviewer, err := s.reserveSwarmTask(ctx, p, run.ID, "task-review", schedulerReserveKey(reviewQueue), run.Revision)
	if err != nil || reviewer.ID == initial.Swarm.Tasks[0].OperationID || reviewer.ChildID == candidate.ChildID { t.Fatal("distinct reviewer not reserved", err) }
	if assigned, e := s.continueCoordinatorIndependentReview(ctx, p, row); e != nil || !assigned { t.Fatal("new exact assignment absent", e) }
	run, _ = s.projectRuns.Get(p.ID, p.Path, run.ID)
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, swarmReviewAssignmentDigest(run, reviewer.ID))
	if err == nil { run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, reviewer.ChildID, "succeeded", "Controlled distinct reviewer exited.") }
	if err != nil { t.Fatal(err) }
	collectCheckRepairHandoff(t, s, p, h, run, reviewer.ID, []string{})
	for i := 0; i < 2; i++ { if changed, e := s.continueCoordinatorReviewOutcome(ctx, p, row); e != nil || !changed { t.Fatal("fresh finding/resolve absent", e) } }
	if integrated, e := s.continueCoordinatorCandidateIntegration(ctx, p, row); e != nil || !integrated { t.Fatal("fresh reviewed integration absent", e) }
	run, _ = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if !swarmAllIntegrated(run) || len(run.Swarm.ReviewObligations) != 2 || run.Swarm.ReviewObligations[1].CandidateOperationID != candidate.ID || run.Swarm.ReviewObligations[1].ReviewerOperationID != reviewer.ID || !reflect.DeepEqual(initial.Operations, run.Operations[:len(initial.Operations)]) { t.Fatal("review/integration reused or overwrote old attempts") }
	row, _, err = s.continueCoordinatorCheckRepair(ctx, p, row)
	if err != nil || row.Delivery != nil || len(row.PriorDeliveries) != 1 || row.PriorDeliveries[0].CheckID != failed.ID { t.Fatal("failed identity was not archived", err) }
	row, err = s.continueCoordinatorDelivery(ctx, p, row)
	if err != nil || row.Delivery == nil || row.Delivery.CheckID == failed.ID { t.Fatal("fresh combined check reused failure", err) }
	passed := finishEvidenceRun(t, s, p.ID, row.Delivery.CheckID)
	row, err = s.continueCoordinatorDelivery(ctx, p, row)
	if err != nil || passed.Status != "passed" || row.Delivery.Status != "verified" || passed.Commit == failed.Commit || starts.Load() != 3 { t.Fatal("fresh exact check did not verify repaired source", row, passed, err) }
}

func collectCheckRepairHandoff(t *testing.T, s *Server, p project.Project, h http.Handler, run ProjectRun, operation string, changed []string) {
	t.Helper()
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: reviewDigest(operation), Report: &TaskHandoffReport{Version: 1, Summary: "Controlled independent inspection of the exact changed source.", ChangedFiles: changed, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect exact candidate", Outcome: "passed", Detail: "Reviewed the declared acceptance."}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/runs/"+run.ID+"/swarm/operations/"+operation+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200)
}
