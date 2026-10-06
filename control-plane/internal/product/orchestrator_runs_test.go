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
	latest, active := orchestratorRunProjection(runs)
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
	latest, active := orchestratorRunProjection(runs)
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
