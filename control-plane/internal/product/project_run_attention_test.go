package product

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// CLOUD-010: actual HTTP run preparation, real isolated Git changes and human
// integration, with a controlled exited-child observer (no model/executor).
func TestCloud010OlderTaskReviewSurvivesNewRunWithoutActiveCapacity(t *testing.T) {
	s, mux, p, old, base, selected := swarmHTTPFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	old = swarmReserveHTTP(t, mux, base, old, "task-a", "attention-old-task-reservation")
	op := old.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, old.ID, op.ID)
	swarmCommitFixture(t, path, "alpha.txt", "retained older output\n")
	var claimed bool
	var err error
	old, _, claimed, err = s.projectRuns.ClaimOperation(p.ID, p.Path, old.ID, old.Revision, op.ID)
	if err != nil || !claimed {
		t.Fatal(claimed, err)
	}
	route := *selected
	route.WorktreeID = op.WorktreeID
	child := projectRunCandidate(route, op.Implementation)
	child.ID, child.ProjectRunID, child.ProjectRunOperationID = op.ChildID, old.ID, op.ID
	child.Status, child.ExitReason = registry.StatusExited, "exit status 0"
	var observations atomic.Int32
	s.SetProjectRunSessionObserver(func(_ context.Context, projectID, runID, operationID string) ([]registry.Session, error) {
		observations.Add(1)
		if projectID == p.ID && runID == old.ID && operationID == op.ID {
			return []registry.Session{child}, nil
		}
		return nil, nil
	})
	old = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/reconcile", map[string]int{"expectedRevision": old.Revision}, 200)).ProjectRun
	connection := selected.AgentConnectionID
	request := projectRunCreateRequest{IdempotencyKey: "attention-newer-prepared-run", WorktreeID: old.WorktreeID, Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, AgentConnectionID: &connection, Outcome: old.Input.Outcome, ImplementationContext: *ImplementationContextSelection(&old.Input.Implementation)}
	newer := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/runs", request, 201)).ProjectRun
	read := func() (ProjectOverview, OrchestratorView) {
		t.Helper()
		before, beforeSource, observed := auditTree(t, s.stateDir), auditTree(t, p.Path), observations.Load()
		overview := overviewGet(t, mux, p.ID)
		var orchestrator OrchestratorView
		decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/orchestrator", nil, 200).Body.Bytes(), &orchestrator)
		if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeSource, auditTree(t, p.Path)) || observations.Load() != observed {
			t.Fatal("summary GET changed private/source state or observed a child")
		}
		return overview, orchestrator
	}
	overview, orchestrator := read()
	if overview.LatestRun.ID != newer.ID || overview.LatestRun.State != "ready" || overview.Phase.ID != "develop" || overview.Attention.Status != "recorded" || overview.Attention.Count == nil || *overview.Attention.Count != 1 || len(overview.Attention.Items) != 1 || overview.Attention.Items[0].ID != old.ID || overview.Attention.Items[0].Kind != "task-review" || overview.NextAction.RunID != old.ID {
		t.Fatalf("new plan hid retained task review: %+v", overview)
	}
	if overview.Execution.Running == nil || *overview.Execution.Running != 0 || overview.Execution.Reserved == nil || *overview.Execution.Reserved != 0 || orchestrator.Context.CurrentRun.ID != newer.ID || len(orchestrator.Context.ActiveRuns) != 0 || len(orchestrator.Context.NeedsAttention) != 1 || orchestrator.Context.NeedsAttention[0].ID != old.ID || orchestrator.NextAction.RunID != old.ID {
		t.Fatalf("review disappeared or was counted as active execution: %+v %+v", overview.Execution, orchestrator)
	}
	// Actual reviewed integration retires this task's review obligation. Other
	// not-yet-started tasks remain prepared work in the run, not verified output.
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if !review.CanIntegrate || !strings.Contains(review.Patch, "+retained older output") {
		t.Fatal("fixture has no real reviewable output", review)
	}
	old = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": old.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	overview, orchestrator = read()
	if len(overview.Attention.Items) != 0 || len(orchestrator.Context.NeedsAttention) != 0 || len(orchestrator.Context.ActiveRuns) != 0 {
		t.Fatal("integration did not retire task-review obligation independently of execution", overview.Attention)
	}
	// Explicit cancellation leaves the historical work/files available too.
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/runs/"+old.ID+"/cancel", map[string]int{"expectedRevision": old.Revision}, 200)
	overview, orchestrator = read()
	if *overview.Attention.Count != 0 || len(orchestrator.Context.NeedsAttention) != 0 || overview.LatestRun.ID != newer.ID || orchestrator.NextAction.RunID == old.ID {
		t.Fatal("cancelled historical work stayed open", overview.Attention, orchestrator.NextAction)
	}
	if contents, err := os.ReadFile(filepath.Join(path, "alpha.txt")); err != nil || string(contents) != "retained older output\n" {
		t.Fatal("attention update discarded retained work", err)
	}
}

func TestCloud010LegacyAttentionMigrationAndWriteRebuildAreNotGETSideEffects(t *testing.T) {
	s, mux, p, req, _, _ := projectRunHTTPFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	url := "/v1/product/projects/" + p.ID + "/runs"
	old := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, req, 201)).ProjectRun
	var err error
	old, err = s.projectRuns.Observe(p.ID, p.Path, old.ID, old.Revision, ProjectRunObservation{State: "awaiting-checks", Reason: "Recorded earlier result needs independent checks"})
	if err != nil {
		t.Fatal(err)
	}
	req.IdempotencyKey = "attention-migration-newer"
	newer := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, req, 201)).ProjectRun
	scope, _ := s.projectRuns.projectScope(p.ID, p.Path)
	head, err := s.projectRuns.readOverviewRunHead(p.ID, scope)
	if err != nil {
		t.Fatal(err)
	}
	head.Version, head.AttentionIDs = 1, nil
	if err = s.projectRuns.writeOverviewRunHead(head); err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, s.stateDir)
	legacy := overviewGet(t, mux, p.ID)
	if legacy.LatestRun.ID != newer.ID || legacy.Attention.Status != "unavailable" || legacy.Attention.Count != nil || len(legacy.Attention.Items) != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("legacy attention guessed zero or was migrated by GET", legacy)
	}
	// A real service startup upgrades valid existing history without source,
	// provider, child reconciliation or canonical run mutation.
	canonical := auditTree(t, filepath.Join(s.stateDir, "project-runs"))
	next, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	nextMux := http.NewServeMux()
	next.registerProjectOverview(next.guardProjectSourceRoutes(nextMux))
	restored := overviewGet(t, nextMux, p.ID)
	if restored.LatestRun.ID != newer.ID || restored.Attention.Count == nil || *restored.Attention.Count != 1 || restored.Attention.Items[0].ID != old.ID || !reflect.DeepEqual(canonical, auditTree(t, filepath.Join(s.stateDir, "project-runs"))) {
		t.Fatal("bounded startup lost older obligation or changed authority", restored)
	}
	// A stale generation remains unknown on GET, then an explicit run write
	// reconstructs from the complete validated history instead of one row.
	index, _ := s.projectRuns.overviewRunHeadPath(p.ID, false)
	if err = os.Remove(index); err != nil {
		t.Fatal(err)
	}
	before = auditTree(t, s.stateDir)
	unknown := overviewGet(t, mux, p.ID)
	if unknown.Attention.Count != nil || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("missing index guessed or repaired")
	}
	if _, err = s.projectRuns.Observe(p.ID, p.Path, newer.ID, newer.Revision, ProjectRunObservation{State: "ready", Reason: "Explicit newer-run reconciliation"}); err != nil {
		t.Fatal(err)
	}
	rebuilt := overviewGet(t, mux, p.ID)
	if rebuilt.Attention.Count == nil || *rebuilt.Attention.Count != 1 || rebuilt.Attention.Items[0].ID != old.ID || rebuilt.LatestRun.ID != newer.ID {
		t.Fatal("write rebuilt only newer run", rebuilt)
	}
}

func TestCloud010CorruptIndexedAttentionAndRevokedReadRevealNoPrivateDetails(t *testing.T) {
	s, mux, p, req, _, _ := projectRunHTTPFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	url := "/v1/product/projects/" + p.ID + "/runs"
	old := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, req, 201)).ProjectRun
	if _, err := s.projectRuns.Observe(p.ID, p.Path, old.ID, old.Revision, ProjectRunObservation{State: "awaiting-release", Reason: "Private old review"}); err != nil {
		t.Fatal(err)
	}
	req.IdempotencyKey = "attention-corruption-newer"
	newer := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, req, 201)).ProjectRun
	if got := overviewGet(t, mux, p.ID); len(got.Attention.Items) != 1 {
		t.Fatal("fixture did not warm attention cache", got)
	}
	denied := httptest.NewRequest("GET", "/v1/product/projects/"+p.ID+"/overview", nil)
	denied = denied.WithContext(management.WithTeamAccess(denied.Context(), management.TeamAccess{ProjectID: p.ID}))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, denied)
	if response.Code != 403 || strings.Contains(response.Body.String(), old.ID) || strings.Contains(response.Body.String(), "Private old review") {
		t.Fatal("revoked read exposed warmed attention", response.Body.String())
	}
	// Corrupt exact indexed old payload without changing directory generation.
	if err := os.WriteFile(filepath.Join(s.stateDir, "project-runs", p.ID, old.ID+".json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	s.projectOverview.entries = nil // cold read, not a cache-expiry timing test
	before := auditTree(t, s.stateDir)
	got := overviewGet(t, mux, p.ID)
	if got.LatestRun.ID != newer.ID || got.Attention.Status != "unavailable" || got.Attention.Count != nil || len(got.Attention.Items) != 0 || got.Status != "partial" || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("corrupt historical obligation became zero or leaked details", got)
	}
}

func TestCloud010AttentionIndexAndSummaryBounds(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, s, id, path, input)
	scope, _ := s.projectScope(id, path)
	head, err := s.readOverviewRunHead(id, scope)
	if err != nil {
		t.Fatal(err)
	}
	runs := make([]ProjectRun, projectRunHistoryLimit)
	for i := range runs {
		runs[i] = run
		runs[i].ID = fmt.Sprintf("run-%024d", i)
		runs[i].State = "awaiting-checks"
		runs[i].Reason = strings.Repeat("private long reason", 200)
		head.AttentionIDs = append(head.AttentionIDs, runs[i].ID)
	}
	if err = s.writeOverviewRunHead(head); err != nil {
		t.Fatal("maximum compact index exceeds declared bound", err)
	}
	index, _ := s.overviewRunHeadPath(id, false)
	bytes, _ := os.ReadFile(index)
	if len(bytes) > overviewRunHeadLimit {
		t.Fatal("index exceeded byte cap", len(bytes))
	}
	attention := projectRunAttentionProjection(runs, journeyState{})
	encoded, _ := json.MarshalIndent(attention, "", "  ")
	if len(attention) != projectRunHistoryLimit || len(encoded) > 32<<10 || strings.Contains(string(encoded), "private long reason") {
		t.Fatal("attention lost identities or retained full context", len(attention), len(encoded))
	}
	// Dense histories can contain both an old review obligation and an active
	// sibling. Attention must share the existing bounded assistant budget.
	for i := range runs {
		runs[i].State = "blocked"
		for j := range projectRunOperationLimit {
			runs[i].Operations = append(runs[i].Operations, ProjectRunOperation{ID: fmt.Sprintf("op-%024d", j), ChildID: fmt.Sprintf("s-%024d", j), Status: "running", UpdatedAt: "2026-10-08T12:34:56.123456789Z"})
		}
	}
	latest, active := orchestratorRunProjection(runs, journeyState{})
	combined, err := json.MarshalIndent(OrchestratorContext{CurrentRun: latest, ActiveRuns: active, NeedsAttention: projectRunAttentionProjection(runs, journeyState{})}, "", "  ")
	if err != nil || len(combined) >= orchestratorContextLimit-8*1024 || len(active) != projectRunHistoryLimit {
		t.Fatal("attention consumed assistant headroom or lost active identities", len(combined), err)
	}
	// Accepted/cancelled history is not an unresolved or capacity obligation.
	for _, state := range []string{"ready", "complete", "cancelled"} {
		run.State = state
		if kind := projectRunAttentionKind(run); kind != "" {
			t.Fatal(state, kind)
		}
	}
}

func TestCloud010ParentCancellationDoesNotRetireTaskReview(t *testing.T) {
	run := ProjectRun{ID: "older", State: "cancelled", Swarm: &ProjectSwarm{Tasks: []ProjectSwarmTask{{ID: "task", OperationID: "op"}}}, Operations: []ProjectRunOperation{{ID: "op", Status: "review-required", Allocation: "ready"}}}
	for _, status := range []string{"review-required", "succeeded", "failed"} {
		run.Operations[0].Status = status
		if projectRunAttentionKind(run) != "task-review" || orchestratorRunSummary(run).Activity != "none" {
			t.Fatal("parent cancellation hid retained output or invented active capacity", status)
		}
	}
	run.Operations[0].Status = "cancelled"
	if projectRunAttentionKind(run) != "" {
		t.Fatal("explicitly retired attempt remained a review obligation")
	}
}
