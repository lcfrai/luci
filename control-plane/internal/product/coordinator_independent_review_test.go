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
	starts := 0
	s.swarmScheduler.runtime = SwarmSchedulerRuntime{
		CaptureActor: management.CaptureExecutionActor,
		Admit: func(context.Context, SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
			return SwarmAdmissionResult{State: "admitted", MemoryMiB: 1024, CPUMillis: 1000}, nil
		},
		Release: func(context.Context, string) error { return nil },
		Dispatch: func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
			starts++
			current, e := s.GetProjectSwarmRun(req.ProjectID, req.RunID)
			if e != nil || swarmReviewAssignmentDigest(current, req.OperationID) == "" {
				t.Fatal("reviewer dispatched without exact candidate assignment", e)
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
