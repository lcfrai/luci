package product

import (
	"context"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestCoordinatorReviewMutationGateRejectsPolicyChangeForEveryEffect(t *testing.T) {
	for _, kind := range []string{"create", "finding", "resolve"} {
		t.Run(kind, func(t *testing.T) {
			s, _, p, run, _, _ := swarmHTTPFixtureConfiguredPolicy(t, "", true, true,
				ProjectTask{ID: "task-a", Title: "Build component", Status: "done", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Build component"},
				ProjectTask{ID: "task-b", Title: "Review component", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect candidate"},
				ProjectTask{ID: "task-c", Title: "Use component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}},
			)
			s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
			s.SetCoordinatorExecutionAuthorizer(func(context.Context, string, string, bool) error { return nil })
			policy, err := s.readCoordinatorPolicy(p)
			if err != nil {
				t.Fatal(err)
			}
			selection := *ImplementationContextSelection(&run.Input.Implementation)
			digest, err := s.continuationContextLocked(p, selection)
			if err != nil {
				t.Fatal(err)
			}
			row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ProjectID: p.ID, Actor: ContextActor{ID: "review-owner"}, RunID: run.ID, Status: "queued", PolicyRevision: policy.Revision}, Run: projectRunCreateRequest{ImplementationContext: selection, Harness: run.Input.Route.Harness}, CanonicalDigest: digest, CheckPlan: &CoordinatorCheckPlan{}}
			ctx := orchestratorActor(t, "review-owner")
			gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: kind, BeforeMutation: func() {
				coordinatorSavePolicy(t, coordinationHandler(s, "review-owner"), p.ID, policy.Revision, "review", []string{"plan"})
			}})
			if err := s.checkCoordinatorIndependentReviewGate(gated, p, run, kind); err != nil {
				t.Fatal("initial reviewed gate unavailable", err)
			}
			coordinatorIndependentReviewBeforeMutation(gated)
			if err := s.checkCoordinatorIndependentReviewGate(gated, p, run, kind); err == nil {
				t.Fatal("background effect crossed a changed policy at final mutation")
			}
			if err := s.checkCoordinatorIndependentReviewGate(ctx, p, run, kind); err != nil {
				t.Fatal("explicit authorized route inherited background gate", err)
			}
		})
	}
}

func TestCoordinatorWaitsForCandidateThenBindsReservedReviewerBeforeDispatch(t *testing.T) {
	s, mux, p, run, base, selected := swarmHTTPFixtureConfiguredPolicy(t, "", true, true,
		ProjectTask{ID: "task-a", Title: "Build component", Status: "done", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Build component"},
		ProjectTask{ID: "task-b", Title: "Review component", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect candidate and report checks"},
		ProjectTask{ID: "task-c", Title: "Use component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}},
	)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	s.SetCoordinatorExecutionAuthorizer(func(context.Context, string, string, bool) error { return nil })
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	s.registerSwarmScheduler(mux)
	actor := coordinationHandler(s, "review-owner")
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	ctx := orchestratorActor(t, "review-owner")
	selectionContext := *ImplementationContextSelection(&run.Input.Implementation)
	digest, err := s.continuationContextLocked(p, selectionContext)
	if err != nil {
		t.Fatal(err)
	}
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ProjectID: p.ID, Actor: ContextActor{ID: "review-owner"}, RunID: run.ID, Status: "queued", PolicyRevision: policy.Revision, Actions: []CoordinatorAction{{}, {}, {Status: "confirmed"}}}, Run: projectRunCreateRequest{ImplementationContext: selectionContext, Harness: run.Input.Route.Harness}, CanonicalDigest: digest, CheckPlan: &CoordinatorCheckPlan{}}
	row.ID = newID("continue-")
	row.IdempotencyKey = "review-retry-continuation-123456"
	row.KeyDigest = reviewDigest(row.IdempotencyKey)
	row.RequestDigest = reviewDigest("review retry fixture")
	row.ContextDigest = reviewDigest("reviewed context fixture")
	row.Revision, row.MaxParallel = 1, 1
	row.ExecutionActor, err = management.CaptureExecutionActor(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bindProjectRequirements(approved, p.ID, []string{"AC-save"}, &selectionContext.SpecificationRevision, &selectionContext.DesignPlanRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	row.CheckPlan = &CoordinatorCheckPlan{Commands: []CheckCommand{{Label: "Reviewed combined check", Executable: "node", Args: []string{"-e", "process.exit(0)"}}}, RequirementIDs: binding.RequirementIDs, RequirementSnapshots: binding.RequirementSnapshots, AcknowledgeProjectCommands: true}
	for i, kind := range []string{"create-run", "prepare-tasks", "queue-tasks"} {
		row.Actions[i] = CoordinatorAction{ID: row.ID + "-" + kind, Kind: kind, Status: "confirmed"}
	}
	s.mu.Lock()
	continuations, saveErr := s.readContinuations(p, row.Actor.ID)
	if saveErr == nil {
		continuations.Rows = append(continuations.Rows, row)
		saveErr = s.saveContinuations(continuations)
	}
	s.mu.Unlock()
	if saveErr != nil {
		t.Fatal(saveErr)
	}
	s.coordinatorBackground.authorize = func(bounded context.Context, _ management.ExecutionActorRef, _ string, _ bool) (context.Context, error) {
		return management.WithExecutionAuthority(bounded, management.ExecutionAuthority{Actor: management.User{ID: row.Actor.ID, Role: "member"}}), nil
	}
	starts, admissions := 0, 0
	admittedIDs := []string{}
	s.swarmScheduler.runtime = SwarmSchedulerRuntime{
		CaptureActor: management.CaptureExecutionActor,
		Admit: func(_ context.Context, req SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
			admissions++
			admittedIDs = append(admittedIDs, req.QueueID)
			return SwarmAdmissionResult{State: "admitted", MemoryMiB: 1024, CPUMillis: 1000}, nil
		},
		Release: func(context.Context, string) error { return nil },
		Dispatch: func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
			starts++
			current, e := s.GetProjectSwarmRun(req.ProjectID, req.RunID)
			op := swarmOperation(&current, req.OperationID)
			if e != nil || op == nil || op.TaskID == "task-b" && swarmReviewAssignmentDigest(current, req.OperationID) == "" || op.TaskID == "task-a" && (op.Checkpoint == nil || current.Swarm.ReviewObligations[0].RepairOperationID != op.ID) {
				t.Fatal("work dispatched without exact review or repair binding", e)
			}
			return SwarmDispatchResult{State: "waiting", ReasonCode: "test_no_runtime", Message: "No child started in this fixture."}, nil
		},
	}
	var queued struct{ Items []SwarmQueueItem }
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/queue", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "reviewer-queue-123456789", "priority": "normal", "taskIds": []string{"task-b"}}, 201).Body.Bytes(), &queued)
	if err := s.RunSwarmSchedulerOnce(ctx); err != nil || starts != 0 {
		t.Fatal("reviewer started before candidate", err, starts)
	}
	before, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || len(before.Items) != 1 || before.Items[0].OperationID != "" || before.Items[0].ReasonCode != "review_candidate_pending" {
		t.Fatal("reviewer was reserved before a candidate existed", before, err)
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "candidate-for-review-123456")
	candidate := run.Operations[0]
	path, err := s.swarmPath(p.ID, run.ID, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	selection := *selected
	selection.WorktreeID = candidate.WorktreeID
	child := projectRunCandidate(selection, candidate.Implementation)
	child.ID, child.ProjectRunID, child.ProjectRunOperationID, child.Status, child.ExitReason = candidate.ChildID, run.ID, candidate.ID, registry.StatusExited, "exited"
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.SetProjectRunSessionObserver(func(_ context.Context, _, _, operationID string) ([]registry.Session, error) {
		if operationID == candidate.ID {
			return []registry.Session{child}, nil
		}
		return nil, nil
	})
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/reconcile", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	if err := s.RunSwarmSchedulerOnce(ctx); err != nil || starts != 0 {
		t.Fatal("reviewer started without a committed candidate", err, starts)
	}
	uncommitted, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || uncommitted.Items[0].OperationID != "" || uncommitted.Items[0].ReasonCode != "review_candidate_unready" {
		t.Fatal("uncommitted candidate allocated a reviewer checkout", uncommitted, err)
	}
	swarmCommitFixture(t, path, "alpha.txt", "candidate content\n")
	s.WakeSwarmScheduler()
	if err := s.RunSwarmSchedulerOnce(ctx); err != nil || starts != 0 {
		t.Fatal("unassigned reviewer dispatched", err, starts)
	}
	reserved, err := s.GetProjectSwarmRun(p.ID, run.ID)
	queue, queueErr := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil || queueErr != nil || len(queue.Items) != 1 || queue.Items[0].State != "reserved" || queue.Items[0].ReasonCode != "review_assignment_pending" || queue.Items[0].MemoryMiB != 0 || swarmOperation(&reserved, queue.Items[0].OperationID) == nil {
		t.Fatal("reviewer reservation did not wait without a host lease", reserved, queue, err, queueErr)
	}
	assigned, err := s.continueCoordinatorIndependentReview(ctx, p, row)
	if err != nil || !assigned {
		t.Fatal("reauthorized coordinator did not assign exact candidate", err, assigned)
	}
	if repeated, err := s.continueCoordinatorIndependentReview(ctx, p, row); err != nil || repeated {
		t.Fatal("saved assignment was replayed", err, repeated)
	}
	if err := s.RunSwarmSchedulerOnce(ctx); err != nil || starts != 1 {
		t.Fatal("bound reviewer was not admitted once", err, starts)
	}
	latest, err := s.GetProjectSwarmRun(p.ID, run.ID)
	if err != nil || len(latest.Swarm.ReviewObligations) != 1 || latest.Swarm.ReviewObligations[0].CandidateOperationID != candidate.ID || latest.Swarm.ReviewObligations[0].ReviewerOperationID != queue.Items[0].OperationID {
		t.Fatal("review receipt lost exact attempts", latest, err)
	}
	reviewer := swarmOperation(&latest, queue.Items[0].OperationID)
	if reviewer == nil {
		t.Fatal("reviewer operation was lost")
	}
	assignment := swarmReviewAssignmentDigest(latest, reviewer.ID)
	latest, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, latest.ID, latest.Revision, reviewer.ID, assignment)
	if err != nil {
		t.Fatal(err)
	}
	latest, err = s.projectRuns.ObserveOperation(p.ID, p.Path, latest.ID, latest.Revision, reviewer.ID, reviewer.ChildID, "succeeded", "Independent reviewer finished.")
	if err != nil {
		t.Fatal(err)
	}
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("exact candidate review"), Report: &TaskHandoffReport{Version: 1, Summary: "Reviewed the candidate and found a missing acceptance check", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect candidate diff", Outcome: "failed", Detail: "Acceptance case missing"}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	var handoff struct{ Run ProjectRun }
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/operations/"+reviewer.ID+"/handoff/collect", map[string]any{"expectedRevision": latest.Revision}, 200).Body.Bytes(), &handoff)
	if recorded, e := s.continueCoordinatorReviewOutcome(ctx, p, row); e != nil || !recorded {
		t.Fatal("reviewer report did not produce one attributed finding", e, recorded)
	}
	withFinding, err := s.GetProjectSwarmRun(p.ID, run.ID)
	if err != nil || len(withFinding.Swarm.Messages) != 1 || withFinding.Swarm.Messages[0].Attribution != "coordinator-recorded" || withFinding.Swarm.ReviewObligations[0].State != "pending" {
		t.Fatal("finding was not retained separately from review decision", withFinding, err)
	}
	if resolved, e := s.continueCoordinatorReviewOutcome(ctx, p, row); e != nil || !resolved {
		t.Fatal("bounded failed review did not require repair", e, resolved)
	}
	failed, err := s.GetProjectSwarmRun(p.ID, run.ID)
	if err != nil || failed.Swarm.ReviewObligations[0].State != "repair-required" || failed.Swarm.ReviewObligations[0].FindingMessageID != withFinding.Swarm.Messages[0].ID || swarmAllIntegrated(failed) {
		t.Fatal("failure was treated as reviewed integration", failed, err)
	}
	if repeated, e := s.continueCoordinatorReviewOutcome(ctx, p, row); e != nil || repeated {
		t.Fatal("resolved review replayed", e, repeated)
	}
	// The old reviewed candidate remains retained. A repair intent must first
	// bind that failed finding, then create a separate protected checkpoint;
	// it must never turn a worker's failed report into an integration pass.
	s.SetProjectSwarmCheckpointGuard(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req ProjectSwarmRecoveryRequest) (func(), error) {
		if req.ProjectID != p.ID || req.RunID != run.ID || req.OperationID != candidate.ID || req.ChildID != candidate.ChildID {
			t.Fatal("repair checkpoint used another child", req)
		}
		return func() {}, nil
	})
	if advanced, e := s.continueCoordinatorRepair(ctx, p, row); e != nil || !advanced {
		t.Fatal("failed independent review did not retain a repair intent", e, advanced)
	}
	queueAfterFailure, e := s.ProjectSwarmQueueSnapshot(p.ID)
	if e != nil || len(queueAfterFailure.Items) != 2 || queueAfterFailure.Items[1].State != "paused" || queueAfterFailure.Items[1].ReasonCode != "repair_checkpoint_pending" || queueAfterFailure.Items[1].OperationID != "" || starts != 1 {
		t.Fatal("repair intent dispatched before protected checkpoint", queueAfterFailure, starts, e)
	}
	repairControl := "/v1/product/projects/" + p.ID + "/swarm-queue/" + queueAfterFailure.Items[1].ID
	journeyRequest(t, actor, "POST", repairControl+"/resume", map[string]int{"expectedRevision": queueAfterFailure.Items[1].Revision}, 409)
	journeyRequest(t, actor, "POST", repairControl+"/pause", map[string]int{"expectedRevision": queueAfterFailure.Items[1].Revision}, 200)
	s.swarmScheduler.mu.Lock()
	pausedLedger, pausedErr := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if pausedErr != nil || pausedLedger.Items[1].State != "paused" || pausedLedger.Items[1].PausedFromState != "paused" || pausedLedger.Items[1].PausedFromCode != "repair_checkpoint_pending" || pausedLedger.Items[1].RepairObligationID != failed.Swarm.ReviewObligations[0].ID {
		t.Fatal("operator pause replaced the internal checkpoint identity", pausedErr, pausedLedger)
	}
	journeyRequest(t, actor, "POST", repairControl+"/resume", map[string]int{"expectedRevision": pausedLedger.Items[1].Revision}, 200)
	s.swarmScheduler.mu.Lock()
	restoredPending, restoreErr := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if restoreErr != nil || restoredPending.Items[1].State != "paused" || restoredPending.Items[1].ReasonCode != "repair_checkpoint_pending" || restoredPending.Items[1].BatchKeyDigest != pausedLedger.Items[1].BatchKeyDigest || restoredPending.Items[1].Actor != pausedLedger.Items[1].Actor || restoredPending.Items[1].PausedFromState != "" || starts != 1 {
		t.Fatal("operator resume upgraded or changed the protected pending repair", restoreErr, restoredPending)
	}
	if advanced, e := s.continueCoordinatorRepair(ctx, p, row); e != nil || !advanced {
		t.Fatal("retained repair intent did not continue exact checkpoint", e, advanced)
	}
	repaired, e := s.GetProjectSwarmRun(p.ID, run.ID)
	queueAfterCheckpoint, queueErr := s.ProjectSwarmQueueSnapshot(p.ID)
	if e != nil || queueErr != nil || len(repaired.Operations) != 3 || len(queueAfterCheckpoint.Items) != 2 || queueAfterCheckpoint.Items[1].State != "reserved" || queueAfterCheckpoint.Items[1].OperationID != repaired.Operations[2].ID || repaired.Swarm.ReviewObligations[0].RepairOperationID != repaired.Operations[2].ID || starts != 1 {
		t.Fatal("repair checkpoint was not bound to failed finding and scheduler", repaired, queueAfterCheckpoint, e, queueErr)
	}
	// Simulate an interrupted response after the run saved its new operation but
	// before the private scheduler observed it. The same durable key must read
	// back that operation; no second checkout or attempt is allowed.
	s.swarmScheduler.mu.Lock()
	retainedQueue, e := s.readSwarmScheduler()
	if e == nil {
		retainedQueue.Items[1].State = "paused"
		retainedQueue.Items[1].ReasonCode = "repair_checkpoint_pending"
		retainedQueue.Items[1].OperationID = ""
		retainedQueue.Items[1].ChildID = ""
		e = s.saveSwarmScheduler(&retainedQueue)
	}
	s.swarmScheduler.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	if owner, e := s.SwarmQueueIDForOperation(p.ID, run.ID, repaired.Operations[2].ID); e != nil || owner != queueAfterCheckpoint.Items[1].ID {
		t.Fatal("paused repair lost private queue ownership before response readback", owner, e)
	}
	if advanced, e := s.continueCoordinatorRepair(ctx, p, row); e != nil || !advanced {
		t.Fatal("saved checkpoint could not be read back after interrupted queue update", e, advanced)
	}
	afterReadback, e := s.GetProjectSwarmRun(p.ID, run.ID)
	if e != nil || len(afterReadback.Operations) != 3 {
		t.Fatal("interrupted repair created a replacement checkout", e, afterReadback)
	}
	if e = s.schedulerSetReason(queueAfterCheckpoint.Items[1].ID, "queued", "capacity", "The owned repair awaits current host capacity."); e != nil {
		t.Fatal(e)
	}
	waitingRepair, e := s.ProjectSwarmQueueSnapshot(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	journeyRequest(t, actor, "POST", repairControl+"/pause", map[string]int{"expectedRevision": waitingRepair.Items[1].Revision}, 200)
	pausedRepair, e := s.ProjectSwarmQueueSnapshot(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	journeyRequest(t, actor, "POST", repairControl+"/resume", map[string]int{"expectedRevision": pausedRepair.Items[1].Revision}, 200)
	resumedRepair, e := s.ProjectSwarmQueueSnapshot(p.ID)
	if e != nil || resumedRepair.Items[1].State != "queued" || resumedRepair.Items[1].ReasonCode != "capacity" || resumedRepair.Items[1].OperationID != afterReadback.Operations[2].ID || starts != 1 {
		t.Fatal("queued repair resume lost its exact checkpoint or started a child", e, resumedRepair, starts)
	}
	// The scheduler may first settle the older reviewer queue for this project;
	// fairness then advances the reserved repair on its next bounded turn.
	if e := s.RunSwarmSchedulerOnce(ctx); e != nil {
		t.Fatal(e)
	}
	if e := s.RunSwarmSchedulerOnce(ctx); e != nil || starts != 2 || admittedIDs[len(admittedIDs)-1] != queueAfterCheckpoint.Items[1].ID {
		latestQueue, _ := s.ProjectSwarmQueueSnapshot(p.ID)
		t.Fatal("exact repaired checkout did not reach ordinary scheduler admission", e, starts, admissions, admittedIDs, latestQueue)
	}
	// A stopped, changed repair candidate can be reviewed again through the
	// same scheduler. Its old failed reviewer remains immutable evidence.
	repair := repaired.Operations[2]
	repairPath, e := s.swarmPath(p.ID, run.ID, repair.ID)
	if e != nil {
		t.Fatal(e)
	}
	swarmCommitFixture(t, repairPath, "alpha.txt", "candidate content with the missing acceptance check\n")
	repaired, e = s.GetProjectSwarmRun(p.ID, run.ID)
	if e != nil {
		t.Fatal(e)
	}
	repaired, _, _, e = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, repaired.Revision, repair.ID)
	if e != nil {
		t.Fatal(e)
	}
	repaired, e = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, repaired.Revision, repair.ID, repair.ChildID, "succeeded", "The repair candidate stopped with changed source.")
	if e != nil {
		t.Fatal(e)
	}
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("exact repaired candidate"), Report: &TaskHandoffReport{Version: 1, Summary: "Added the missing acceptance case to the changed implementation", ChangedFiles: []string{"alpha.txt"}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect repaired candidate diff", Outcome: "passed", Detail: "Changed source contains the acceptance case"}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	var repairedHandoff struct{ Run ProjectRun }
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/operations/"+repair.ID+"/handoff/collect", map[string]any{"expectedRevision": repaired.Revision}, 200).Body.Bytes(), &repairedHandoff)
	if advanced, e := s.continueCoordinatorReviewerRetry(ctx, p, row); e != nil || !advanced {
		t.Fatal("changed repair did not queue a distinct reviewer", advanced, e)
	}
	if repeated, e := s.continueCoordinatorReviewerRetry(ctx, p, row); e != nil || repeated {
		t.Fatal("retry intent replayed", repeated, e)
	}
	queuedRetry, e := s.ProjectSwarmQueueSnapshot(p.ID)
	if e != nil || len(queuedRetry.Items) != 3 || queuedRetry.Items[2].TaskID != "task-b" || queuedRetry.Items[2].OperationID != "" || starts != 2 {
		t.Fatal("retry bypassed queue admission", e, queuedRetry, starts)
	}
	s.swarmScheduler.mu.Lock()
	retryLedger, retryReadErr := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if retryReadErr != nil {
		t.Fatal(retryReadErr)
	}
	_, _, retryCancel, retryAuthErr := s.coordinatorReviewerRetryAuthority(ctx, p, retryLedger.Items[2])
	if retryCancel != nil {
		retryCancel()
	}
	if retryAuthErr != nil {
		t.Fatal("saved retry lost exact authority before scheduling", retryAuthErr)
	}
	for i := 0; i < 3; i++ {
		if e = s.RunSwarmSchedulerOnce(ctx); e != nil {
			t.Fatal(e)
		}
	}
	queuedRetry, e = s.ProjectSwarmQueueSnapshot(p.ID)
	if e != nil || queuedRetry.Items[2].OperationID == "" || queuedRetry.Items[2].ReasonCode != "review_assignment_pending" || starts != 2 {
		t.Fatal("retry did not reserve a new reviewer without starting it", e, queuedRetry, starts)
	}
	if advanced, e := s.continueCoordinatorIndependentReview(ctx, p, row); e != nil || !advanced {
		t.Fatal("changed repair did not bind the new reviewer", advanced, e)
	}
	if e = s.RunSwarmSchedulerOnce(ctx); e != nil || starts != 3 {
		t.Fatal("bound retry did not reach ordinary dispatch", e, starts)
	}
	final, e := s.GetProjectSwarmRun(p.ID, run.ID)
	if e != nil || len(final.Swarm.ReviewObligations) != 2 || final.Swarm.ReviewObligations[0].State != "repair-required" || final.Swarm.ReviewObligations[1].CandidateOperationID != repair.ID || final.Swarm.ReviewObligations[1].ReviewerOperationID != queuedRetry.Items[2].OperationID {
		t.Fatal("retry did not preserve exact failed and successor review receipts", e, final)
	}
	reviewRetry := swarmOperation(&final, queuedRetry.Items[2].OperationID)
	if reviewRetry == nil {
		t.Fatal("retry lost its reviewer operation")
	}
	final, _, _, e = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, final.Revision, reviewRetry.ID, swarmReviewAssignmentDigest(final, reviewRetry.ID))
	if e != nil {
		t.Fatal(e)
	}
	final, e = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, final.Revision, reviewRetry.ID, reviewRetry.ChildID, "succeeded", "The independent reviewer stopped after checking the repaired candidate.")
	if e != nil {
		t.Fatal(e)
	}
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("exact changed repair review"), Report: &TaskHandoffReport{Version: 1, Summary: "The repaired candidate passes the previously missing acceptance check", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect repaired candidate diff", Outcome: "passed", Detail: "The missing acceptance case is present"}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	var passedHandoff struct{ Run ProjectRun }
	decodeEvidenceResponse(t, journeyRequest(t, actor, "POST", base+"/operations/"+reviewRetry.ID+"/handoff/collect", map[string]any{"expectedRevision": final.Revision}, 200).Body.Bytes(), &passedHandoff)
	for i := 0; i < 2; i++ {
		if recorded, outcomeErr := s.continueCoordinatorReviewOutcome(ctx, p, row); outcomeErr != nil || !recorded {
			t.Fatal("changed candidate lacked an attributed passed-review receipt", i, outcomeErr, recorded)
		}
	}
	passed, e := s.GetProjectSwarmRun(p.ID, run.ID)
	if e != nil || passed.Swarm.ReviewObligations[0].State != "repair-required" || passed.Swarm.ReviewObligations[1].State != "passed" || !swarmIndependentReviewReady(passed, "task-a", ProjectSwarmReview{OperationID: repair.ID, SourceCommit: passed.Swarm.ReviewObligations[1].CandidateCommit, ReviewDigest: passed.Swarm.ReviewObligations[1].ReviewDigest}) {
		t.Fatal("new pass did not retain the prior failed finding and exact repaired source", e, passed)
	}
	if integrated, integrationErr := s.continueCoordinatorCandidateIntegration(ctx, p, row); integrationErr != nil || !integrated {
		t.Fatal("passed repaired candidate did not enter reviewed integration", integrationErr, integrated)
	}
	integratedRun, e := s.GetProjectSwarmRun(p.ID, run.ID)
	if e != nil || swarmTask(&integratedRun, "task-a").Integration == nil {
		t.Fatal("reviewed repair did not retain an integration receipt", e, integratedRun)
	}
	coordinatorSavePolicy(t, actor, p.ID, policy.Revision, "review", []string{"plan"})
	_, _, cancelled, staleErr := s.coordinatorReviewerRetryAuthority(ctx, p, retryLedger.Items[2])
	if cancelled != nil {
		cancelled()
	}
	if staleErr == nil {
		t.Fatal("revoked reviewed delegation could still authorize a saved automatic retry")
	}
	afterRevocation, e := s.GetProjectSwarmRun(p.ID, run.ID)
	if e != nil || len(afterRevocation.Operations) != len(integratedRun.Operations) || afterRevocation.Revision != integratedRun.Revision {
		t.Fatal("revoked retry changed the retained run", e, afterRevocation)
	}
	s.swarmScheduler.mu.Lock()
	unknownLedger, unknownErr := s.readSwarmScheduler()
	if unknownErr == nil {
		unknownLedger.Items[1].State, unknownLedger.Items[1].ReasonCode = "unknown", "repair_allocation_unconfirmed"
		unknownErr = s.saveSwarmScheduler(&unknownLedger)
	}
	s.swarmScheduler.mu.Unlock()
	if unknownErr != nil {
		t.Fatal(unknownErr)
	}
	journeyRequest(t, actor, "POST", repairControl+"/resume", map[string]int{"expectedRevision": unknownLedger.Items[1].Revision}, 409)
	s.swarmScheduler.mu.Lock()
	stillUnknown, unknownReadErr := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if unknownReadErr != nil || stillUnknown.Items[1].State != "unknown" || stillUnknown.Items[1].OperationID != unknownLedger.Items[1].OperationID || stillUnknown.Items[1].Revision != unknownLedger.Items[1].Revision {
		t.Fatal("uncertain repair was upgraded by ordinary resume", unknownReadErr, stillUnknown)
	}
}

func TestCancelledReviewerQueueSettlesBeforeCandidateWait(t *testing.T) {
	for _, runningCandidate := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent candidate", true: "running candidate"}[runningCandidate], func(t *testing.T) {
			s, mux, p, run, base, _ := swarmHTTPFixtureConfiguredPolicy(t, "", true, true,
				ProjectTask{ID: "task-a", Title: "Build component", Status: "done", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Build component"},
				ProjectTask{ID: "task-b", Title: "Review component", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect candidate"},
				ProjectTask{ID: "task-c", Title: "Use component", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}},
			)
			s.registerSwarmScheduler(mux)
			starts := 0
			s.swarmScheduler.runtime = SwarmSchedulerRuntime{CaptureActor: management.CaptureExecutionActor,
				Admit: func(context.Context, SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
					return SwarmAdmissionResult{State: "admitted"}, nil
				},
				Release: func(context.Context, string) error { return nil },
				Dispatch: func(context.Context, SwarmDispatchRequest) (SwarmDispatchResult, error) {
					starts++
					return SwarmDispatchResult{State: "waiting"}, nil
				},
			}
			actor := coordinationHandler(s, "review-owner")
			journeyRequest(t, actor, "POST", base+"/queue", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "cancelled-reviewer-queue-123456", "priority": "normal", "taskIds": []string{"task-b"}}, 201)
			if runningCandidate {
				run = swarmReserveHTTP(t, mux, base, run, "task-a", "candidate-running-cancel-123456")
			}
			_, err := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
				current.State, current.Swarm.State = "cancelled", "cancelled"
				if runningCandidate {
					candidate := swarmOperation(current, swarmTask(current, "task-a").OperationID)
					candidate.Status, candidate.Allocation = "running", "ready"
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.RunSwarmSchedulerOnce(orchestratorActor(t, "review-owner")); err != nil {
				t.Fatal(err)
			}
			queue, err := s.ProjectSwarmQueueSnapshot(p.ID)
			if err != nil || len(queue.Items) != 1 || queue.Items[0].State != "cancelled" || queue.Items[0].ReasonCode != "swarm_cancelled" || starts != 0 {
				t.Fatal("cancelled reviewer kept waiting or started", queue, starts, err)
			}
		})
	}
}
