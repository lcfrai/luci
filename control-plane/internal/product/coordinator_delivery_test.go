package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func coordinatorDeliveryFixture(t *testing.T, commands ...CheckCommand) (*Server, project.Project, http.Handler, context.Context, CoordinatorContinuation, *atomic.Int32) {
	t.Helper()
	s, p, h, ctx, request, _ := coordinatorContinuationFixture(t)
	return coordinatorDeliveryRequestFixture(t, s, p, h, ctx, request, commands...)
}

func coordinatorDeliveryRequestFixture(t *testing.T, s *Server, p project.Project, h http.Handler, ctx context.Context, request coordinatorContinuationRequest, commands ...CheckCommand) (*Server, project.Project, http.Handler, context.Context, CoordinatorContinuation, *atomic.Int32) {
	t.Helper()
	if len(commands) == 0 {
		commands = []CheckCommand{{Label: "Reviewed behavior check", Executable: "node", Args: []string{"-e", "const fs=require('fs');if(fs.readFileSync('result.txt','utf8')!=='independently integrated implementation\\n'){process.exit(7)};console.log('integrated result verified')"}}}
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bindProjectRequirements(state, p.ID, []string{"AC-save"}, &request.Run.ImplementationContext.SpecificationRevision, &request.Run.ImplementationContext.DesignPlanRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	request.CheckPlan = &CoordinatorCheckPlan{Commands: commands, RequirementIDs: binding.RequirementIDs, RequirementSnapshots: binding.RequirementSnapshots, AcknowledgeProjectCommands: true}
	starts := &atomic.Int32{}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("This controlled check lifecycle requires Node", err)
	}
	installControlledProjectCheckRuntime(t, s, func(ctx context.Context, req ProjectCheckCommandRequest) (string, error) {
		starts.Add(1)
		s.mu.RLock()
		rows, readErr := s.readContinuations(p, "alice")
		s.mu.RUnlock()
		if readErr != nil || len(rows.Rows) != 1 || rows.Rows[0].Delivery == nil || rows.Rows[0].Delivery.CheckID != req.CheckID {
			return "", errors.New("command executed without its durable coordinator identity")
		}
		return RunCommand(ctx, node, req.Command.Args, req.SourceDirectory)
	})
	out := coordinatorCreate(t, h, p, request, 201)
	return s, p, h, ctx, out, starts
}

// The coding outcome is controlled; review, private operation receipts, native
// Git isolation and integration are real. No provider or Docker claim is made.
func coordinatorIntegrateFixture(t *testing.T, s *Server, p project.Project, h http.Handler, out CoordinatorContinuation, terminal ...string) ProjectRun {
	t.Helper()
	run, err := s.projectRuns.Get(p.ID, p.Path, out.RunID)
	if err != nil {
		t.Fatal(err)
	}
	base := "/v1/product/projects/" + p.ID + "/runs/" + run.ID + "/swarm"
	run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", base+"/tasks/task-save/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "delivery-reserve-task-123456"}, 201)).ProjectRun
	op := run.Operations[len(run.Operations)-1]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, "result.txt", "independently integrated implementation\n")
	if len(terminal) == 0 {
		run = swarmFinishFixture(t, s, p, run, op.ID)
	} else {
		run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, op.ID)
		if err == nil {
			run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, terminal[0], "Controlled failed worker left committed source for explicit review.")
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	if reviewerTask := swarmTask(&run, "task-review"); reviewerTask != nil && reviewerTask.Role == "reviewer" {
		s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
			if req.OperationID != op.ID {
				return TaskHandoffCapture{}, errors.New("foreign candidate handoff")
			}
			return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("preview candidate review"), Report: &TaskHandoffReport{Version: 1, Summary: "Candidate source is ready for independent review.", ChangedFiles: []string{"result.txt"}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect result.txt", Outcome: "passed", Detail: "Candidate committed."}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
		})
		journeyRequest(t, h, "POST", base+"/operations/"+op.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200)
		run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		var candidateResponse struct {
			Review ProjectSwarmReview `json:"review"`
		}
		decodeEvidenceResponse(t, journeyRequest(t, h, "GET", base+"/tasks/task-save/review", nil, 200).Body.Bytes(), &candidateResponse)
		candidateReview := candidateResponse.Review
		if candidateReview.CanIntegrate || candidateReview.IndependentReviewState != "pending" {
			t.Fatal("candidate bypassed independent review", candidateReview)
		}
		run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", base+"/tasks/task-review/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "delivery-review-reserve-123456"}, 201)).ProjectRun
		reviewer := run.Operations[len(run.Operations)-1]
		var obligation struct {
			Obligation ProjectSwarmReviewObligationView `json:"obligation"`
		}
		decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/tasks/task-save/review-obligations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "delivery-review-obligation-123456", "candidateOperationId": op.ID, "reviewerOperationId": reviewer.ID, "reviewDigest": candidateReview.ReviewDigest}, 201).Body.Bytes(), &obligation)
		run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		assignment := swarmReviewAssignmentDigest(run, reviewer.ID)
		if assignment == "" {
			t.Fatal("independent reviewer has no exact assignment")
		}
		run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, assignment)
		if err != nil {
			t.Fatal(err)
		}
		run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, reviewer.ChildID, "succeeded", "Independent reviewer completed the controlled review.")
		if err != nil {
			t.Fatal(err)
		}
		s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
			if req.OperationID != reviewer.ID {
				return TaskHandoffCapture{}, errors.New("foreign reviewer handoff")
			}
			return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("preview independent review"), Report: &TaskHandoffReport{Version: 1, Summary: "Independently checked the candidate against the saved acceptance.", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect candidate diff", Outcome: "passed", Detail: "The saved acceptance is covered."}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
		})
		var handoff struct {
			Handoff TaskHandoffView `json:"handoff"`
		}
		decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/operations/"+reviewer.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &handoff)
		run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		var finding struct {
			Message     ProjectSwarmMessage `json:"message"`
			RunRevision int                 `json:"runRevision"`
		}
		decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/operations/"+reviewer.ID+"/messages", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "delivery-review-finding-123456", "recipientTaskId": "task-save", "kind": "review-finding", "evidence": map[string]any{"kind": "handoff", "id": reviewer.ID, "digest": handoff.Handoff.Digest}, "summary": "The exact candidate passed independent review."}, 200).Body.Bytes(), &finding)
		journeyRequest(t, h, "POST", base+"/review-obligations/"+obligation.Obligation.ID+"/resolve", map[string]any{"expectedRevision": finding.RunRevision, "outcome": "passed", "reviewerHandoffDigest": handoff.Handoff.Digest, "findingMessageId": finding.Message.ID}, 200)
		run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	var reviewed struct {
		Review ProjectSwarmReview `json:"review"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", base+"/tasks/task-save/review", nil, 200).Body.Bytes(), &reviewed)
	if !reviewed.Review.CanIntegrate {
		t.Fatal("controlled native source was not reviewable", reviewed.Review.Reason)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, h, "POST", base+"/tasks/task-save/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": reviewed.Review.ReviewDigest}, 200)).ProjectRun
	return run
}

func coordinatorDeliveryAdvance(t *testing.T, h http.Handler, p project.Project, row CoordinatorContinuation, status int) CoordinatorContinuation {
	t.Helper()
	var next CoordinatorContinuation
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations/"+row.ID+"/advance", map[string]int{"expectedRevision": row.Revision}, status).Body.Bytes(), &next)
	return next
}

func coordinatorReserveCheckFixture(t *testing.T, s *Server, p project.Project, ctx context.Context, id string) (coordinatorContinuationRecord, ProjectRun) {
	t.Helper()
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	row := rows.Rows[findContinuation(rows, id)]
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil {
		t.Fatal(err)
	}
	integration, ready := coordinatorIntegratedRun(run, row)
	if !ready {
		t.Fatal("fixture did not authoritatively integrate every task")
	}
	digest, commit, dirty, err := s.checkSource(ctx, p.Path)
	if err != nil || dirty {
		t.Fatal("fixture source unavailable", err, dirty)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row.Delivery = &CoordinatorDelivery{ActionID: row.ID + "-checks", CheckID: coordinatorCheckID(row.ID), Status: "pending", Commit: commit, SourceDigest: digest, IntegrationDigest: integration, StartedAt: now, UpdatedAt: now}
	row, err = s.persistCoordinatorDelivery(ctx, p, row, "pending", "Controlled process-loss boundary before the typed check call.")
	if err != nil {
		t.Fatal(err)
	}
	return row, run
}

func coordinatorDeliveryReopen(t *testing.T, s *Server) (*Server, http.Handler) {
	t.Helper()
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restarted.Close() })
	restarted.SetCoordinatorAuthorizer(s.coordinatorAuthorize)
	restarted.SetCoordinatorExecutionAuthorizer(s.coordinatorExecuteAuthorize)
	restarted.SetProjectCheckRuntime(s.projectCheckRuntime)
	mux := http.NewServeMux()
	restarted.Register(mux)
	h := contextHistoryActorHandler(mux, management.User{ID: "alice", Username: "alice", DisplayName: "Alice", Role: "member"})
	return restarted, h
}

func TestCoordinatorDeliveryHTTPWaitsForReviewedIntegrationAndJoinsExactNativeCheck(t *testing.T) {
	s, p, h, ctx, out, starts := coordinatorDeliveryFixture(t)
	before := auditTree(t, s.stateDir)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	if out.Status != "queued" || out.Delivery != nil || starts.Load() != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("board completion or queued work released checks")
	}
	coordinatorIntegrateFixture(t, s, p, h, out)
	before = auditTree(t, s.stateDir)
	journeyRequest(t, h, "GET", "/v1/product/projects/"+p.ID+"/orchestrator/continuations/"+out.ID, nil, 200)
	if starts.Load() != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("read-only continuation executed or repaired delivery")
	}
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	if out.Delivery == nil || out.Delivery.CheckID != coordinatorCheckID(out.ID) || out.Status != "checking" {
		t.Fatal("checks did not reserve exact identity", out)
	}
	check := finishEvidenceRun(t, s, p.ID, out.Delivery.CheckID)
	if check.Status != "passed" || check.Commands[0].ExitCode == nil || *check.Commands[0].ExitCode != 0 || !check.Commands[0].CleanupConfirmed {
		t.Fatal("controlled command did not produce native evidence", check)
	}
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	run, err := s.projectRuns.Get(p.ID, p.Path, out.RunID)
	if err != nil || starts.Load() != 1 || out.Status != "checks-verified" || out.Delivery.Status != "verified" || len(run.Checks) != 1 || run.Checks[0].CheckID != check.ID || len(run.Releases) != 0 || run.Acceptance != nil {
		t.Fatal("exact check was not independently joined, or delivery was fabricated", out, run, err)
	}
	conversation, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || conversation.Activity == nil || len(conversation.Activity.Work) == 0 ||
		conversation.Activity.Work[0].Finding == nil || conversation.Activity.Work[0].Finding.Status != "verified" ||
		conversation.Activity.Work[0].Finding.CheckID != check.ID || conversation.Activity.Work[0].RunID != run.ID {
		t.Fatalf("verified check finding was absent from the actor's primary conversation: %+v %v", conversation.Activity, err)
	}
	coordinatorDeliveryAdvance(t, h, p, out, 200)
	if starts.Load() != 1 {
		t.Fatal("confirmed check was replayed")
	}
}

func TestCoordinatorDeliveryRestartConsumesLostResponseWithoutReplay(t *testing.T) {
	s, p, h, ctx, out, starts := coordinatorDeliveryFixture(t)
	coordinatorIntegrateFixture(t, s, p, h, out)
	row, run := coordinatorReserveCheckFixture(t, s, p, ctx, out.ID)
	if err := s.invokeCoordinatorChecks(ctx, p, row, run); err != nil {
		t.Fatal(err)
	}
	finishEvidenceRun(t, s, p.ID, row.Delivery.CheckID)
	// The actual typed handler completed, but the continuation's post-call
	// publication was lost. Only its pre-call pending journal survives.
	path, _ := s.continuationPath(p.ID, "alice", false)
	before, _ := os.ReadFile(path)
	restarted, handler := coordinatorDeliveryReopen(t, s)
	journeyRequest(t, handler, "GET", "/v1/product/projects/"+p.ID+"/orchestrator/continuations/"+row.ID, nil, 200)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("restart observation mutated continuation")
	}
	observed := coordinatorDeliveryAdvance(t, handler, p, row.CoordinatorContinuation, 200)
	joined, err := restarted.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || starts.Load() != 1 || observed.Delivery.Status != "verified" || len(joined.Checks) != 1 || joined.Checks[0].CheckID != row.Delivery.CheckID {
		t.Fatal("lost result caused replay or lost exact evidence", observed, joined, err)
	}
}

func TestCoordinatorDeliveryAbsentPendingCheckNeverAdoptsAnotherOrReplays(t *testing.T) {
	s, p, h, ctx, out, starts := coordinatorDeliveryFixture(t)
	coordinatorIntegrateFixture(t, s, p, h, out)
	row, run := coordinatorReserveCheckFixture(t, s, p, ctx, out.ID)
	// An ordinary direct user check has matching commands/source/requirements,
	// but no coordinator origin and no authority to settle this pending action.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	installControlledProjectCheckRuntime(t, s, func(ctx context.Context, req ProjectCheckCommandRequest) (string, error) {
		starts.Add(1)
		return RunCommand(ctx, node, req.Command.Args, req.SourceDirectory)
	})
	var unrelated ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/checks", map[string]any{
		"commands": row.CheckPlan.Commands, "execution": "docker", "acknowledgeHostExecution": true,
		"requirementIds": []string{"AC-save"}, "expectedSpecificationRevision": run.Input.Implementation.SpecificationRevision,
		"expectedDesignPlanRevision": run.Input.Implementation.DesignPlanRevision,
	}, 202).Body.Bytes(), &unrelated)
	finishEvidenceRun(t, s, p.ID, unrelated.ID)
	if unrelated.ID == row.Delivery.CheckID || unrelated.CoordinatorOriginDigest != "" {
		t.Fatal("ordinary user checks acquired coordinator origin")
	}
	restarted, handler := coordinatorDeliveryReopen(t, s)
	for i := 0; i < 2; i++ {
		row.CoordinatorContinuation = coordinatorDeliveryAdvance(t, handler, p, row.CoordinatorContinuation, 200)
		if row.Delivery.Status != "unknown" || starts.Load() != 1 {
			t.Fatal("missing exact check replayed or adopted unrelated evidence", row)
		}
	}
	joined, _ := restarted.projectRuns.Get(p.ID, p.Path, run.ID)
	if len(joined.Checks) != 0 {
		t.Fatal("unrelated check was joined by heuristic")
	}
}

func TestCoordinatorDeliveryRejectsSourceAuthorityApprovalAndPolicyDrift(t *testing.T) {
	for _, drift := range []string{"source", "authority", "approval", "policy"} {
		t.Run(drift, func(t *testing.T) {
			s, p, h, ctx, out, starts := coordinatorDeliveryFixture(t)
			coordinatorIntegrateFixture(t, s, p, h, out)
			row, run := coordinatorReserveCheckFixture(t, s, p, ctx, out.ID)
			if err := s.invokeCoordinatorChecks(ctx, p, row, run); err != nil {
				t.Fatal(err)
			}
			finishEvidenceRun(t, s, p.ID, row.Delivery.CheckID)
			status := 409
			switch drift {
			case "source":
				if err := os.WriteFile(filepath.Join(p.Path, "result.txt"), []byte("unreviewed later source\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "authority":
				s.SetCoordinatorExecutionAuthorizer(func(context.Context, string, string, bool) error { return errors.New("revoked") })
				status = 403
			case "approval":
				state, err := s.readApprovedJourney(p.Path, p.ID)
				if err != nil || s.recordJourneyApproval(context.Background(), p.Path, p.ID, "design", "new-reviewer", &state) != nil {
					t.Fatal("could not change private approval", err)
				}
			case "policy":
				coordinatorSavePolicy(t, h, p.ID, row.PolicyRevision, "auto-drafts", []string{"plan"})
			}
			before, _ := s.readCheckHistory(p.ID)
			journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations/"+row.ID+"/advance", map[string]int{"expectedRevision": row.Revision}, status)
			after, _ := s.readCheckHistory(p.ID)
			joined, _ := s.projectRuns.Get(p.ID, p.Path, run.ID)
			if starts.Load() != 1 || !reflect.DeepEqual(before, after) || len(joined.Checks) != 0 {
				t.Fatal("drift accepted stale evidence or replayed the command")
			}
		})
	}
}

func TestCoordinatorDeliveryCheckHandlerRejectsSourceActorAndCommandDriftAfterReservation(t *testing.T) {
	for _, drift := range []string{"source", "actor", "commands"} {
		t.Run(drift, func(t *testing.T) {
			s, p, h, ctx, out, starts := coordinatorDeliveryFixture(t)
			coordinatorIntegrateFixture(t, s, p, h, out)
			row, run := coordinatorReserveCheckFixture(t, s, p, ctx, out.ID)
			switch drift {
			case "source":
				if err := os.WriteFile(filepath.Join(p.Path, "result.txt"), []byte("source changed before command admission\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "actor":
				ctx = orchestratorActor(t, "bob")
			case "commands":
				row.CheckPlan.Commands[0].Args = []string{"-e", "process.exit(0)"}
			}
			if err := s.invokeCoordinatorChecks(ctx, p, row, run); err == nil {
				t.Fatal("drift admitted an internal check")
			}
			checks, err := s.readCheckHistory(p.ID)
			if err != nil || len(checks) != 0 || starts.Load() != 0 {
				t.Fatal("refusal created execution", err, checks)
			}
		})
	}
}

func TestCoordinatorDeliveryFailedNativeCheckCannotBecomeVerified(t *testing.T) {
	s, p, h, _, out, starts := coordinatorDeliveryFixture(t, CheckCommand{Label: "Failing behavior check", Executable: "node", Args: []string{"-e", "process.exit(7)"}})
	coordinatorIntegrateFixture(t, s, p, h, out)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	check := finishEvidenceRun(t, s, p.ID, out.Delivery.CheckID)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	run, _ := s.projectRuns.Get(p.ID, p.Path, out.RunID)
	if starts.Load() != 1 || check.Status != "failed" || *check.Commands[0].ExitCode != 7 || out.Delivery.Status != "blocked" || len(run.Checks) != 0 {
		t.Fatal("actual command failure became verified", out, check)
	}
}

func TestCoordinatorDeliveryCreateRequiresExactDeclaredRequirementCoverage(t *testing.T) {
	for _, invalid := range []string{"missing-ids", "missing-snapshots", "changed-acceptance", "foreign-requirement", "expanded-coverage"} {
		t.Run(invalid, func(t *testing.T) {
			s, p, h, _, request, starts := coordinatorContinuationFixture(t)
			state, err := s.readApprovedJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			binding, err := bindProjectRequirements(state, p.ID, []string{"AC-save"}, &request.Run.ImplementationContext.SpecificationRevision, &request.Run.ImplementationContext.DesignPlanRevision, true)
			if err != nil {
				t.Fatal(err)
			}
			request.CheckPlan = &CoordinatorCheckPlan{Commands: []CheckCommand{{Label: "Explicit command", Executable: "node", Args: []string{"check.js"}}}, RequirementIDs: binding.RequirementIDs, RequirementSnapshots: binding.RequirementSnapshots, AcknowledgeProjectCommands: true}
			status := 409
			switch invalid {
			case "missing-ids":
				request.CheckPlan.RequirementIDs = nil
				status = 400
			case "missing-snapshots":
				request.CheckPlan.RequirementSnapshots = nil
				status = 400
			case "changed-acceptance":
				request.CheckPlan.RequirementSnapshots[0].Acceptance = "A different command coverage assertion"
			case "foreign-requirement":
				request.CheckPlan.RequirementIDs[0] = "AC-other"
				request.CheckPlan.RequirementSnapshots[0].ID = "AC-other"
			case "expanded-coverage":
				request.CheckPlan.RequirementIDs = append(request.CheckPlan.RequirementIDs, "AC-other")
				request.CheckPlan.RequirementSnapshots = append(request.CheckPlan.RequirementSnapshots, RequirementSnapshot{ID: "AC-other", Title: "Not selected", Acceptance: "Not authorized coverage"})
			}
			journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/continuations", request, status)
			runs, err := s.projectRuns.List(p.ID, p.Path)
			if err != nil || len(runs) != 0 || *starts != 0 {
				t.Fatal("invalid declared coverage created execution", err)
			}
		})
	}
}
