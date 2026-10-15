package product

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func reviewHandlerRequest(t *testing.T, ctx context.Context, method string, body any, paths map[string]string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(ctx, method, "http://review.invalid/", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range paths {
		req.SetPathValue(key, value)
	}
	return req, httptest.NewRecorder()
}

// Exercise the real mutation handlers. A preflight that passed before a
// coordinator policy amendment cannot persist an automatic assignment or
// finding; an explicit authorized project route remains available.
func TestCoordinatorReviewHandlersFenceChangedPolicyBeforeDurableMutation(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixtureConfiguredPolicy(t, "", true, true,
		ProjectTask{ID: "task-a", Title: "Implement", Status: "done", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Implement"},
		ProjectTask{ID: "task-b", Title: "Review", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect"},
		ProjectTask{ID: "task-c", Title: "Dependent", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}},
	)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	s.SetCoordinatorExecutionAuthorizer(func(context.Context, string, string, bool) error { return nil })
	actor := coordinationHandler(s, "review-owner")
	ctx := orchestratorActor(t, "review-owner")
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	selection := *ImplementationContextSelection(&run.Input.Implementation)
	digest, err := s.continuationContextLocked(p, selection)
	if err != nil {
		t.Fatal(err)
	}
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ProjectID: p.ID, Actor: ContextActor{ID: "review-owner"}, RunID: run.ID, Status: "queued", PolicyRevision: policy.Revision, Actions: []CoordinatorAction{{}, {}, {Status: "confirmed"}}}, Run: projectRunCreateRequest{ImplementationContext: selection, Harness: run.Input.Route.Harness}, CanonicalDigest: digest, CheckPlan: &CoordinatorCheckPlan{}}
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "handler-gate-candidate-123456")
	candidate := run.Operations[0]
	path, err := s.swarmPath(p.ID, run.ID, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	swarmCommitFixture(t, path, "alpha.txt", "reviewable implementation\n")
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, candidate.ID, candidate.ChildID, "succeeded", "Observed stopped candidate")
	if err != nil {
		t.Fatal(err)
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "handler-gate-reviewer-123456")
	reviewer := run.Operations[1]
	review, err := s.swarmReview(ctx, p, run, "task-a")
	if err != nil || !review.CanIntegrate {
		t.Fatal("candidate was not reviewable", err, review)
	}
	create := swarmReviewCreateRequest{ExpectedRevision: run.Revision, IdempotencyKey: "handler-gate-create-123456", CandidateOperationID: candidate.ID, ReviewerOperationID: reviewer.ID, ReviewDigest: review.ReviewDigest}
	paths := map[string]string{"id": p.ID, "runId": run.ID, "taskId": "task-a"}
	changed := false
	gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "create", BeforeMutation: func() {
		coordinatorSavePolicy(t, actor, p.ID, policy.Revision, "review", []string{"plan"})
		changed = true
	}})
	req, w := reviewHandlerRequest(t, gated, http.MethodPost, create, paths)
	s.createProjectSwarmReviewObligation(w, req)
	after, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || !changed || w.Code < 400 || after.Revision != run.Revision || len(after.Swarm.ReviewObligations) != 0 || swarmTask(&after, "task-b").ReviewOnly {
		t.Fatal("changed policy crossed real create mutation", err, w.Code, after)
	}
	// This is a separate explicit route with current project authority. The
	// background coordinator's old delegation does not revoke that authority.
	req, w = reviewHandlerRequest(t, ctx, http.MethodPost, create, paths)
	s.createProjectSwarmReviewObligation(w, req)
	if w.Code != http.StatusCreated {
		t.Fatal("explicit reviewed assignment was incorrectly blocked", w.Code, w.Body.String())
	}
	after, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || len(after.Swarm.ReviewObligations) != 1 {
		t.Fatal("explicit assignment did not save", err)
	}
}

func TestCoordinatorFindingHandlerFencesPolicyAndContextAtMutation(t *testing.T) {
	for _, change := range []string{"policy", "approved-context"} {
		t.Run(change, func(t *testing.T) {
			s, mux, p, run, base, _ := swarmHTTPFixtureConfiguredPolicy(t, "", true, true)
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
			run = swarmReserveHTTP(t, mux, base, run, "task-a", "finding-gate-source-123456")
			op := run.Operations[0]
			request := swarmMessagePostRequest{ExpectedRevision: run.Revision, IdempotencyKey: "finding-gate-message-123456", RecipientTaskID: "task-b", Kind: "question", Evidence: ProjectSwarmMessageEvidence{Kind: "none"}, Summary: "Confirm the reviewed dependency."}
			called := false
			gated := context.WithValue(orchestratorActor(t, "review-owner"), coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "finding", BeforeMutation: func() {
				if change == "policy" {
					coordinatorSavePolicy(t, coordinationHandler(s, "review-owner"), p.ID, policy.Revision, "review", []string{"plan"})
				} else {
					state, readErr := s.readApprovedJourney(p.Path, p.ID)
					if readErr != nil || state.Plan == nil {
						t.Fatal("approved context is unavailable", readErr)
					}
					tasks := append([]ProjectTask(nil), state.Plan.Tasks...)
					tasks[0].Acceptance = "The newly approved requirement has changed."
					journeyRequest(t, mux, http.MethodPost, "/v1/product/projects/"+p.ID+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, 200)
				}
				called = true
			}})
			req, w := reviewHandlerRequest(t, gated, http.MethodPost, request, map[string]string{"id": p.ID, "runId": run.ID, "operationId": op.ID})
			s.postProjectSwarmMessage(w, req)
			after, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
			if err != nil || !called || w.Code < 400 || after.Revision != run.Revision || len(after.Swarm.Messages) != 0 {
				t.Fatal("automatic finding crossed changed authority or context", change, err, called, w.Code, w.Body.String())
			}
		})
	}
}

func TestCoordinatorResolutionHandlerFencesRevokedDelegation(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixtureConfiguredPolicy(t, "", true, true,
		ProjectTask{ID: "task-a", Title: "Implement", Status: "done", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"alpha.txt"}, Acceptance: "Implement"},
		ProjectTask{ID: "task-b", Title: "Review", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", RequirementIDs: []string{"AC-save"}, Acceptance: "Inspect"},
		ProjectTask{ID: "task-c", Title: "Dependent", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}},
	)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	s.SetCoordinatorExecutionAuthorizer(func(context.Context, string, string, bool) error { return nil })
	actor := coordinationHandler(s, "review-owner")
	ctx := orchestratorActor(t, "review-owner")
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	selection := *ImplementationContextSelection(&run.Input.Implementation)
	digest, err := s.continuationContextLocked(p, selection)
	if err != nil {
		t.Fatal(err)
	}
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ProjectID: p.ID, Actor: ContextActor{ID: "review-owner"}, RunID: run.ID, Status: "queued", PolicyRevision: policy.Revision, Actions: []CoordinatorAction{{}, {}, {Status: "confirmed"}}}, Run: projectRunCreateRequest{ImplementationContext: selection, Harness: run.Input.Route.Harness}, CanonicalDigest: digest, CheckPlan: &CoordinatorCheckPlan{}}
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "resolve-gate-candidate-123456")
	candidate := run.Operations[0]
	path, err := s.swarmPath(p.ID, run.ID, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	swarmCommitFixture(t, path, "alpha.txt", "candidate needs a check\n")
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, candidate.ID, candidate.ChildID, "succeeded", "Observed candidate")
	if err != nil {
		t.Fatal(err)
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "resolve-gate-reviewer-123456")
	reviewer := run.Operations[1]
	review, err := s.swarmReview(ctx, p, run, "task-a")
	if err != nil || !review.CanIntegrate {
		t.Fatal("candidate was not reviewable", err)
	}
	create := swarmReviewCreateRequest{ExpectedRevision: run.Revision, IdempotencyKey: "resolve-gate-create-123456", CandidateOperationID: candidate.ID, ReviewerOperationID: reviewer.ID, ReviewDigest: review.ReviewDigest}
	var created struct {
		Obligation ProjectSwarmReviewObligationView
	}
	decodeEvidenceResponse(t, journeyRequest(t, actor, http.MethodPost, base+"/tasks/task-a/review-obligations", create, 201).Body.Bytes(), &created)
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	run, _, _, err = s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, swarmReviewAssignmentDigest(run, reviewer.ID))
	if err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, reviewer.ID, reviewer.ChildID, "succeeded", "Observed reviewer")
	if err != nil {
		t.Fatal(err)
	}
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: implementationDigest("reviewed candidate"), Report: &TaskHandoffReport{Version: 1, Summary: "Candidate requires repair", ChangedFiles: []string{}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{{Command: "inspect diff", Outcome: "failed", Detail: "Missing check"}}, Questions: []TaskHandoffQuestion{}, Limitations: []string{}}}, nil
	})
	var handoff struct{ Handoff TaskHandoffView }
	decodeEvidenceResponse(t, journeyRequest(t, actor, http.MethodPost, base+"/operations/"+reviewer.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &handoff)
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var finding struct{ Message ProjectSwarmMessage }
	decodeEvidenceResponse(t, journeyRequest(t, actor, http.MethodPost, base+"/operations/"+reviewer.ID+"/messages", swarmMessagePostRequest{ExpectedRevision: run.Revision, IdempotencyKey: "resolve-gate-finding-123456", RecipientTaskID: "task-a", Kind: "review-finding", Evidence: ProjectSwarmMessageEvidence{Kind: "handoff", ID: reviewer.ID, Digest: handoff.Handoff.Digest}, Summary: "Candidate requires repair"}, 200).Body.Bytes(), &finding)
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	resolve := map[string]any{"expectedRevision": run.Revision, "outcome": "repair-required", "reviewerHandoffDigest": handoff.Handoff.Digest, "findingMessageId": finding.Message.ID}
	called := false
	gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "resolve", BeforeMutation: func() {
		coordinatorSavePolicy(t, actor, p.ID, policy.Revision, "review", []string{"plan"})
		called = true
	}})
	req, w := reviewHandlerRequest(t, gated, http.MethodPost, resolve, map[string]string{"id": p.ID, "runId": run.ID, "obligationId": created.Obligation.ID})
	s.resolveProjectSwarmReviewObligation(w, req)
	after, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || !called || w.Code < 400 || after.Revision != run.Revision || after.Swarm.ReviewObligations[0].State != "pending" {
		t.Fatal("changed policy crossed real review resolution mutation", err, called, w.Code, w.Body.String())
	}
}
