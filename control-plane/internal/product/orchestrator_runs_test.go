package product

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestOrchestratorKeepsOlderCodingScopeUntilExplicitTerminalReconciliation(t *testing.T) {
	s, mux, p, request, selected, routeCalls := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID
	runsURL := base + "/runs"
	a := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", runsURL, request, 201))
	a = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", runsURL+"/"+a.ID+"/operations", map[string]any{"expectedRevision": a.Revision, "idempotencyKey": "orchestrator-operation-a", "kind": "session"}, 201))
	op := a.Operations[0]
	claimed, _, claimedOK, err := s.projectRuns.ClaimOperation(p.ID, p.Path, a.ID, a.Revision, op.ID)
	if err != nil || !claimedOK {
		t.Fatal("claim", claimedOK, err)
	}
	child := projectRunCandidate(*selected, &a.Input.Implementation)
	child.ID, child.ProjectRunID, child.ProjectRunOperationID = op.ChildID, a.ID, op.ID
	child.Status = registry.StatusRunning
	var observerCalls atomic.Int32
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) {
		observerCalls.Add(1)
		return []registry.Session{child}, nil
	})
	a = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", runsURL+"/"+a.ID+"/reconcile", map[string]int{"expectedRevision": claimed.Revision}, 200))
	if a.Operations[0].Status != "running" {
		t.Fatal("fixture did not observe its owned child", a)
	}
	var newPlan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": request.ImplementationContext.PlanRevision, "expectedSpecificationRevision": request.ImplementationContext.SpecificationRevision, "tasks": []ProjectTask{{ID: "task-newer", Title: "Build the newer view", Status: "todo", RequirementIDs: []string{"AC-save"}}}}, 200).Body.Bytes(), &newPlan)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Save newer reviewed work scope"), 200)
	request.IdempotencyKey = "orchestrator-newer-prepared-b"
	request.ImplementationContext.PlanRevision = newPlan.Revision
	request.ImplementationContext.TaskIDs = []string{"task-newer"}
	b := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", runsURL, request, 201))
	read := func() OrchestratorView {
		t.Helper()
		var view OrchestratorView
		decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/orchestrator", nil, 200).Body.Bytes(), &view)
		return view
	}
	assertScope := func(view OrchestratorView) {
		t.Helper()
		if view.Context.CurrentRun == nil || view.Context.CurrentRun.ID != b.ID || len(view.Context.CurrentRun.Tasks) != 1 || view.Context.CurrentRun.Tasks[0].ID != "task-newer" {
			t.Fatal("new prepared scope missing", view.Context.CurrentRun)
		}
		if len(view.Context.ActiveRuns) != 1 {
			t.Fatal("older coding work hidden", view.Context.ActiveRuns)
		}
		active := view.Context.ActiveRuns[0]
		if active.ID != a.ID || active.Activity != "running" || len(active.Tasks) != 1 || active.Tasks[0].ID != "task-save" || active.Tasks[0].Title != "Implement saving" || len(active.Requirements) != 1 || active.Requirements[0].ID != "AC-save" || active.SpecificationRevision != a.Input.Implementation.SpecificationRevision || active.DesignRevision != a.Input.Implementation.DesignPlanRevision || active.Outcome != "preview" || len(active.Operations) != 1 || active.Operations[0].ChildID != child.ID {
			t.Fatal("immutable coding scope was replaced by the newer plan", active)
		}
		if view.NextAction.ID != "inspect-run" || view.NextAction.RunID != a.ID {
			t.Fatal("next action did not target actual unfinished work", view.NextAction)
		}
	}
	before := auditTree(t, s.stateDir)
	beforeObservations, beforeRoutes := observerCalls.Load(), routeCalls.Load()
	assertScope(read())
	assertScope(read())
	if observerCalls.Load() != beforeObservations || routeCalls.Load() != beforeRoutes || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("orchestrator GET observed a backend, resolved a provider, or persisted state")
	}
	a = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", runsURL+"/"+a.ID+"/cancel", map[string]int{"expectedRevision": a.Revision}, 200))
	view := read()
	assertScope(view)
	if active := view.Context.ActiveRuns[0]; !active.DispatchCancelled || active.State != "cancelled" || !strings.Contains(active.ActivityReason, "does not stop") || child.Status != registry.StatusRunning {
		t.Fatal("cancelling future dispatch falsely stopped the existing child", active)
	}
	child.Status, child.ExitReason = registry.StatusExited, "process exited"
	a = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", runsURL+"/"+a.ID+"/reconcile", map[string]int{"expectedRevision": a.Revision}, 200))
	view = read()
	if a.State != "cancelled" || a.Operations[0].Status != "cancelled" || len(view.Context.ActiveRuns) != 0 || view.Context.CurrentRun.ID != b.ID || view.NextAction.RunID == a.ID {
		t.Fatal("terminal observation did not clear activity independently of parent cancellation", a, view.Context.ActiveRuns, view.NextAction)
	}
}

func TestOrchestratorRunProjectionKeepsAllUnfinishedIdentitiesAndBoundedDetail(t *testing.T) {
	runs := make([]ProjectRun, projectRunHistoryLimit)
	for i := range runs {
		runs[i] = ProjectRun{ID: fmt.Sprintf("run-%032d", i), State: "coding", Reason: strings.Repeat("r", 4000), Input: ProjectRunInput{Outcome: "preview", Implementation: registry.ImplementationContext{SpecificationRevision: 10, DesignPlanRevision: 10, TaskIDs: []string{"saved-task"}, TaskTitles: []string{strings.Repeat("t", 2000)}}, Requirements: []RequirementSnapshot{{ID: "saved-requirement", Title: "Immutable requirement", Acceptance: strings.Repeat("a", 4000)}}}}
		for j := range 100 {
			runs[i].Operations = append(runs[i].Operations, ProjectRunOperation{ID: fmt.Sprintf("op-%032d", j), ChildID: fmt.Sprintf("s-%032d", j), Status: "running", UpdatedAt: "2026-10-06T12:34:56.123456789Z"})
		}
	}
	latest, active := orchestratorRunProjection(runs, journeyState{})
	if latest == nil || latest.ID != runs[0].ID || len(active) != len(runs) {
		t.Fatal("projection silently lost active identities", len(active))
	}
	seen := map[string]bool{}
	omitted := 0
	for _, run := range active {
		if seen[run.ID] || run.Activity != "running" || run.TaskCount != 1 || run.RequirementCount != 1 || run.OperationCount != 100 {
			t.Fatal("incorrect activity identity or scope count", run.ID)
		}
		seen[run.ID] = true
		if run.ScopeOmitted {
			omitted++
			if run.ContextFreshness == nil || run.ContextFreshness.Status != "changed" || len(run.ContextFreshness.Reason) > 80 {
				t.Fatal("omission lost context comparison or retained excessive prose", run.ID)
			}
			if !run.OperationsOmitted || len(run.Operations) != 0 || len(run.Tasks) != 0 || len(run.Requirements) != 0 || run.Reason != "" || !strings.Contains(run.ActivityReason, "Inspect this run") {
				t.Fatal("omission did not retain an actionable bounded summary", run.ID)
			}
		}
	}
	if omitted == 0 || len(runs[0].Operations) != 100 || len(runs[0].Reason) != 4000 || len(runs[0].Input.Requirements[0].Acceptance) != 4000 {
		t.Fatal("no compaction or original evidence changed")
	}
	// The full legal run-history and operation bounds must leave room in the
	// assistant snapshot for ordinary saved project documents. Large reasons
	// and operation lists count toward the same detail budget as task text.
	raw, err := json.MarshalIndent(OrchestratorContext{CurrentRun: latest, ActiveRuns: active}, "", "  ")
	if err != nil || len(raw) >= orchestratorContextLimit-8*1024 {
		t.Fatalf("run projection exceeded bounded context headroom: %d bytes, %v", len(raw), err)
	}
}

func TestOrchestratorRunActivityOrderAndExactNextAction(t *testing.T) {
	runs := []ProjectRun{
		{ID: "latest-prepared", State: "ready"},
		{ID: "reserved", State: "ready", Operations: []ProjectRunOperation{{ID: "op-reserved", Status: "reserved"}}},
		{ID: "unknown", State: "cancelled", Operations: []ProjectRunOperation{{ID: "op-unknown", Status: "unknown"}}},
		{ID: "dispatching", State: "coding", Operations: []ProjectRunOperation{{ID: "op-dispatching", Status: "dispatching"}}},
		{ID: "running", State: "cancelled", Operations: []ProjectRunOperation{{ID: "op-running", Status: "running"}}},
		{ID: "also-running", State: "coding", Operations: []ProjectRunOperation{{ID: "op-also", Status: "running"}, {ID: "op-prior", Status: "succeeded"}}},
		{ID: "terminal", State: "awaiting-checks", Operations: []ProjectRunOperation{{ID: "op-terminal", Status: "cancelled"}}},
	}
	latest, active := orchestratorRunProjection(runs, journeyState{})
	ids := []string{}
	for _, run := range active {
		ids = append(ids, run.ID)
	}
	if latest.ID != "latest-prepared" || !reflect.DeepEqual(ids, []string{"running", "also-running", "unknown", "dispatching", "reserved"}) || len(active[1].Operations) != 1 || !active[0].DispatchCancelled {
		t.Fatal("unfinished work was hidden, misordered or confused with parent state", ids, active)
	}
	questions := []OrchestratorQuestion{{TurnID: "question", Rationale: "Material ambiguity"}}
	for _, tc := range []struct {
		active    []OrchestratorRunSummary
		questions []OrchestratorQuestion
		id, runID string
	}{
		{active, questions, "inspect-run", "running"},
		{active[2:], questions, "reconcile-run", "unknown"},
		{active[4:], questions, "answer-question", ""},
		{active[4:], nil, "inspect-run", "reserved"},
	} {
		got := orchestratorNext(OrchestratorContext{CurrentRun: latest, ActiveRuns: tc.active}, tc.questions)
		if got.ID != tc.id || got.RunID != tc.runID {
			t.Fatal("next action hid unfinished work or bypassed an unresolved material question", got)
		}
	}
}

// The prior progress is an explicitly seeded recorded observation. No child,
// check command or deployment is executed; this test exercises the read model.
func TestOrchestratorPlanAmendmentRetiresCheckAndReleaseGuidance(t *testing.T) {
	for _, lifecycle := range []string{"awaiting-checks", "awaiting-release"} {
		t.Run(lifecycle, func(t *testing.T) {
			s, mux, p, request, _, calls := projectRunHTTPFixture(t)
			base := "/v1/product/projects/" + p.ID
			created := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", request, 201))
			saved, err := s.projectRuns.Observe(p.ID, p.Path, created.ID, created.Revision, ProjectRunObservation{State: lifecycle, Valid: true, Reason: "Recorded progress for plan A"})
			if err != nil {
				t.Fatal(err)
			}
			read := func() (OrchestratorView, ProjectRunContextFreshness) {
				t.Helper()
				raw := journeyRequest(t, mux, "GET", base+"/orchestrator", nil, 200).Body.Bytes()
				var overview OrchestratorView
				decodeEvidenceResponse(t, raw, &overview)
				// Decode the wire extension independently, so this regression can run
				// against the implementation before the summary has a freshness field.
				var wire struct {
					Context struct {
						CurrentRun struct {
							ContextFreshness ProjectRunContextFreshness `json:"contextFreshness"`
						} `json:"currentRun"`
					} `json:"context"`
				}
				decodeEvidenceResponse(t, raw, &wire)
				return overview, wire.Context.CurrentRun.ContextFreshness
			}
			initial, initialFreshness := read()
			if initialFreshness.Status != "current" {
				t.Error("unchanged captured inputs are not current", initialFreshness)
			}
			expectedAction := "check"
			if lifecycle == "awaiting-release" {
				expectedAction = "deploy"
			}
			if initial.NextAction.ID != expectedAction || initial.NextAction.RunID != saved.ID {
				t.Fatal("fixture did not reach the recorded next step", initial.NextAction)
			}
			if lifecycle == "awaiting-release" && (initial.NextAction.Label != "Review verified build" || initial.NextAction.Lens != "deploy") {
				t.Error("release guidance does not describe the exact-build review checkpoint", initial.NextAction)
			}
			state, err := s.readApprovedJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			tasks := snapshotProjectTasks(state.Plan.Tasks)
			tasks[0].Title = "Save with a visible audit trail"
			journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, 200)
			s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) {
				t.Error("overview observed a child")
				return nil, nil
			})
			s.SetProjectRunNativeReleaseObserver(func(context.Context, string, string) (ProjectRunNativeReleaseObservation, error) {
				t.Error("overview observed a release")
				return ProjectRunNativeReleaseObservation{}, nil
			})
			before, projectBefore, beforeCalls := auditTree(t, s.stateDir), auditTree(t, p.Path), calls.Load()
			overview, freshness := read()
			detail := projectRunHTTPResponse(t, journeyRequest(t, mux, "GET", base+"/runs/"+saved.ID, nil, 200))
			if overview.NextAction.ID != "plan" || overview.NextAction.Lens != "develop" {
				t.Error("amended plan still invites checks or release for the old build", overview.NextAction)
			}
			if freshness.Status != "changed" || freshness != detail.ContextFreshness {
				t.Error("overview disagrees with exact run freshness", freshness, detail.ContextFreshness)
			}
			summary := overview.Context.CurrentRun
			if summary == nil || summary.State != lifecycle || !summary.Valid || summary.Reason != saved.Observation.Reason || len(summary.Tasks) != 1 || summary.Tasks[0].Title != "Implement saving" || len(overview.Context.ActiveRuns) != 0 || !reflect.DeepEqual(detail.ProjectRun, saved) {
				t.Error("read changed recorded evidence or replaced captured scope", summary)
			}
			if overview.Context.Plan.Tasks[0].Title != tasks[0].Title || overview.Context.Plan.SpecificationRevision != overview.Context.Specification.Revision || overview.Context.Specification.Status != "approved" || overview.Context.DesignPlan.Status != "approved" {
				t.Error("fixture amended more than the task plan", overview.Context.Plan)
			}
			if calls.Load() != beforeCalls || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) || !reflect.DeepEqual(projectBefore, auditTree(t, p.Path)) {
				t.Error("read resolved a provider or mutated private/project state")
			}
		})
	}
}

func TestOrchestratorFreshnessPreservesHistoryAndNextActionPrecedence(t *testing.T) {
	s, mux, p, request, _, _ := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID
	original := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", request, 201)).ProjectRun
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Plan.Tasks[0].Title = "An amended task without a revision bump"
	for _, lifecycle := range []string{"ready", "coding", "awaiting-checks", "awaiting-release", "blocked", "complete", "cancelled"} {
		t.Run(lifecycle, func(t *testing.T) {
			history := original
			history.State = lifecycle
			history.Observation = &ProjectRunObservation{State: "awaiting-release", Valid: true, Reason: "Original checked evidence"}
			history.Acceptance = &ProjectRunAcceptance{CheckID: "check-history", ReleaseID: "release-history", AcceptedAt: "2026-10-07T00:00:00Z"}
			before, _ := json.Marshal(history)
			latest, active := orchestratorRunProjection([]ProjectRun{history}, state)
			if latest.State != lifecycle || !latest.Valid || latest.ContextFreshness == nil || *latest.ContextFreshness != projectRunContextFreshness(history, state, nil) || latest.ContextFreshness.Status != "changed" {
				t.Fatal("freshness replaced history or diverged from run detail", latest)
			}
			ctx := OrchestratorContext{Brief: state.Brief, Specification: currentSpecification(state), DesignPlan: currentDesignPlan(state), Plan: currentPlan(state, p.ID), CurrentRun: latest, ActiveRuns: active}
			if action := orchestratorNext(ctx, nil); action.ID != "plan" {
				t.Fatal("changed context was treated as current", action)
			}
			questions := []OrchestratorQuestion{{Rationale: "Resolve a material access decision"}}
			if action := orchestratorNext(ctx, questions); action.ID != "answer-question" {
				t.Fatal("context comparison hid material ambiguity", action)
			}
			for _, activity := range []struct{ status, action string }{{"running", "inspect-run"}, {"unknown", "reconcile-run"}, {"reserved", "inspect-run"}} {
				history.Operations = []ProjectRunOperation{{ID: "op", ChildID: "child", Status: activity.status}}
				ctx.CurrentRun, ctx.ActiveRuns = orchestratorRunProjection([]ProjectRun{history}, state)
				if action := orchestratorNext(ctx, nil); action.ID != activity.action || action.RunID != history.ID {
					t.Fatal("freshness hid unfinished recorded work", action)
				}
			}
			history.Operations = original.Operations
			after, _ := json.Marshal(history)
			if string(before) != string(after) {
				t.Fatal("projection rewrote captured inputs or historical evidence")
			}
		})
	}
}
