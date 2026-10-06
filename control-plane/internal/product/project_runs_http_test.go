package product

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func projectRunHTTPFixture(t *testing.T) (*Server, *http.ServeMux, project.Project, projectRunCreateRequest, *ProjectRunSelection, *atomic.Int32) {
	t.Helper()
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	s.registerProjectRuns(mux)
	p := createJourneyProject(t, mux, "run-http")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	var plan ProjectPlan
	base := "/v1/product/projects/" + p.ID
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": 0, "expectedSpecificationRevision": spec.Revision, "tasks": []ProjectTask{{ID: "task-save", Title: "Implement saving", Status: "done", RequirementIDs: []string{"AC-save"}}}}, 200).Body.Bytes(), &plan)
	if err := os.WriteFile(filepath.Join(p.Path, "check.js"), []byte(`console.log('actual project check');process.exit(0)`), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Save reviewed implementation inputs"), 200)
	selected := &ProjectRunSelection{ProjectID: p.ID, WorktreeID: project.WorktreeID(p.Path), Path: p.Path, Harness: "codex", Model: "fixture-model", PermissionMode: "ask", AgentConnectionID: "connection-fixture", AgentConnectionRevision: 1, AgentConnectionKind: "api", AgentConnectionProtocol: "openai-responses", AuthMode: "api-key", Connection: AgentConnectionReceipt{ID: "connection-fixture", Revision: 1, Kind: "api", Protocol: "openai-responses", AuthMode: "api-key", Model: "fixture-model"}}
	calls := &atomic.Int32{}
	s.SetProjectRunSelectionResolver(func(_ context.Context, request ProjectRunSelectionRequest) (ProjectRunSelection, error) {
		calls.Add(1)
		if request.ProjectID != p.ID || request.WorktreeID != selected.WorktreeID {
			return ProjectRunSelection{}, errors.New("foreign selected worktree")
		}
		return *selected, nil
	})
	id := selected.AgentConnectionID
	req := projectRunCreateRequest{IdempotencyKey: "http-create-key-123456789", WorktreeID: selected.WorktreeID, Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, AgentConnectionID: &id, Outcome: "preview", ImplementationContext: ImplementationContextRequest{ProjectID: p.ID, BriefRevision: spec.BriefRevision, PlanRevision: plan.Revision, SpecificationRevision: spec.Revision, DesignPlanRevision: design.Revision, TaskIDs: []string{"task-save"}}}
	return s, mux, p, req, selected, calls
}

func projectRunHTTPResponse(t *testing.T, response *httptest.ResponseRecorder) ProjectRunView {
	t.Helper()
	var value struct {
		Run ProjectRunView `json:"run"`
	}
	decodeEvidenceResponse(t, response.Body.Bytes(), &value)
	return value.Run
}

func TestProjectRunHTTPCreateReplayReadOnlyAndStaleSelections(t *testing.T) {
	s, mux, p, req, selected, calls := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201))
	if run.State != "ready" || len(run.Operations) != 0 || run.Revision != 1 {
		t.Fatal(run)
	}
	initialCalls := calls.Load()
	if err := os.WriteFile(filepath.Join(p.Path, "agent-created.txt"), []byte("actual source changed after reservation"), 0600); err != nil {
		t.Fatal(err)
	}
	duplicate := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 200))
	if duplicate.ID != run.ID || calls.Load() != initialCalls || duplicate.Input.Implementation.SourceDigest != run.Input.Implementation.SourceDigest {
		t.Fatal("idempotent replay rebound inputs", duplicate)
	}
	changed := req
	changed.Outcome = "publish"
	journeyRequest(t, mux, "POST", base, changed, 409)
	before := auditTree(t, s.stateDir)
	journeyRequest(t, mux, "GET", base, nil, 200)
	journeyRequest(t, mux, "GET", base+"/"+run.ID, nil, 200)
	if calls.Load() != initialCalls || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("GET resolved a route or persisted state")
	}
	// A saved connection changes identity rather than silently selecting a
	// different account/model for this run's next operation.
	selected.Connection.Revision++
	journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "http-operation-key-123456", "kind": "session"}, 409)
	stored, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || len(stored.Operations) != 0 {
		t.Fatal(stored, err)
	}
	blocked := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/reconcile", map[string]int{"expectedRevision": run.Revision}, 200))
	if blocked.State != "blocked" || blocked.Observation == nil || blocked.Observation.Valid {
		t.Fatal(blocked)
	}
	journeyRequest(t, mux, "POST", base+"/"+run.ID+"/cancel", map[string]int{"expectedRevision": run.Revision}, 409)
	cancelled := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/cancel", map[string]int{"expectedRevision": blocked.Revision}, 200))
	if cancelled.State != "cancelled" {
		t.Fatal(cancelled)
	}
}

func TestProjectRunHTTPUnknownChildrenNeverReplayOrVerifyTasks(t *testing.T) {
	s, mux, p, req, _, _ := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201))
	operationRequest := map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "http-operation-key-123456", "kind": "session"}
	reserved := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", operationRequest, 201))
	op := reserved.Operations[0]
	claimed, _, ok, err := s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, reserved.Revision, op.ID)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	var observations atomic.Int32
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) {
		observations.Add(1)
		return nil, nil
	})
	unknown := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/reconcile", map[string]int{"expectedRevision": claimed.Revision}, 200))
	if unknown.State != "blocked" || unknown.Observation.Valid || unknown.Operations[0].Status != "unknown" {
		t.Fatal(unknown)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "new-source.txt"), []byte("later source"), 0600); err != nil {
		t.Fatal(err)
	}
	replayed := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", operationRequest, 200))
	if len(replayed.Operations) != 1 || replayed.Operations[0].ChildID != op.ChildID || replayed.Operations[0].Status != "unknown" {
		t.Fatal(replayed)
	}
	journeyRequest(t, mux, "POST", base+"/"+run.ID+"/accept", map[string]int{"expectedRevision": unknown.Revision}, 409)
	after, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || after.State == "complete" || after.Acceptance != nil || len(after.Operations) != 1 || observations.Load() != 2 {
		t.Fatal(after, err, observations.Load())
	}
	// Neither task status=done in the saved plan nor an absent child's hoped-for
	// exit produced check evidence or a replacement session.
	if len(after.Checks) != 0 || len(after.Releases) != 0 {
		t.Fatal(after)
	}
}

func TestProjectRunHTTPRetainsFailedChildAcrossReconciliation(t *testing.T) {
	s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201))
	reserved := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "http-operation-key-123456", "kind": "session"}, 201))
	op := reserved.Operations[0]
	claimed, _, _, err := s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, reserved.Revision, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	child := projectRunCandidate(*selected, &run.Input.Implementation)
	child.ID, child.ProjectRunID, child.ProjectRunOperationID = op.ChildID, run.ID, op.ID
	child.Status, child.ExitReason = registry.StatusExited, "backend_failed"
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) {
		return []registry.Session{child}, nil
	})
	for range 2 {
		result := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/reconcile", map[string]int{"expectedRevision": claimed.Revision}, 200))
		if result.State != "blocked" || result.Observation.Valid || result.Operations[0].Status != "failed" {
			t.Fatal("failed child became valid without new evidence", result)
		}
		claimed = result.ProjectRun
	}
}

func TestProjectRunHTTPReaperReasonsRetainUnverifiedOutcomes(t *testing.T) {
	for _, tc := range []struct{ reason, state, status string }{
		{"process exited", "awaiting-checks", "cancelled"},
		{"idle timeout", "awaiting-checks", "cancelled"},
		{"sandbox disappeared", "blocked", "unknown"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
			base := "/v1/product/projects/" + p.ID + "/runs"
			run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201))
			reserved := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "http-operation-key-123456", "kind": "session"}, 201))
			op := reserved.Operations[0]
			claimed, _, _, err := s.projectRuns.ClaimOperation(p.ID, p.Path, run.ID, reserved.Revision, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			child := projectRunCandidate(*selected, &run.Input.Implementation)
			child.ID, child.ProjectRunID, child.ProjectRunOperationID = op.ChildID, run.ID, op.ID
			child.Status, child.ExitReason = registry.StatusExited, tc.reason
			s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) {
				return []registry.Session{child}, nil
			})
			for range 2 {
				observed := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/reconcile", map[string]int{"expectedRevision": claimed.Revision}, 200))
				if observed.State != tc.state || observed.Operations[0].Status != tc.status || observed.Acceptance != nil || len(observed.Checks) != 0 || len(observed.Releases) != 0 {
					t.Fatal("process lifecycle was mistaken for a verified coding outcome", observed)
				}
				claimed = observed.ProjectRun
			}
			journeyRequest(t, mux, "POST", base+"/"+run.ID+"/accept", map[string]int{"expectedRevision": claimed.Revision}, 409)
		})
	}
}

func TestProjectRunHTTPRouteChangeDuringReleaseObservationBlocksAcceptance(t *testing.T) {
	s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", req, 201))
	useActualEvidenceProcesses(t, s)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var check ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(*state.Specification, *state.DesignPlan, "check.js"), 202).Body.Bytes(), &check)
	check = finishEvidenceRun(t, s, p.ID, check.ID)
	joined := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/checks", map[string]any{"expectedRevision": run.Revision, "checkId": check.ID}, 200))
	const releaseID = "native-route-race"
	if err = s.RecordProjectRunReleaseProvenance(context.Background(), p.ID, releaseID, "native-docker", check.Commit); err != nil {
		t.Fatal(err)
	}
	changeRoute := false
	observations := 0
	s.SetProjectRunNativeReleaseObserver(func(context.Context, string, string) (ProjectRunNativeReleaseObservation, error) {
		observations++
		if changeRoute {
			selected.Connection.Revision++
		}
		return ProjectRunNativeReleaseObservation{ReleaseID: releaseID, ProjectID: p.ID, Target: "native-docker", Commit: check.Commit, URL: "http://127.0.0.1:3456", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Ready: true, Limitations: "Deterministic owned native release observation"}, nil
	})
	observed := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/releases", map[string]any{"expectedRevision": joined.Revision, "releaseId": releaseID, "target": "native-docker"}, 200))
	changeRoute = true
	journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/accept", map[string]int{"expectedRevision": observed.Revision}, 409)
	retained, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || retained.State != "blocked" || retained.Acceptance != nil || retained.Observation == nil || retained.Observation.Valid || observations != 2 {
		t.Fatal("changed model route completed a run during release observation", retained, observations, err)
	}
	if retained.Input.Route.Connection.Revision != 1 {
		t.Fatal("historical selected route was overwritten", retained.Input.Route)
	}
}

func TestProjectRunHTTPPersistenceErrorsDoNotExposePrivatePaths(t *testing.T) {
	privatePath := filepath.Join(t.TempDir(), "private-account-state", "project-runs", "run-sensitive.json")
	err := fmt.Errorf("read durable project run: %w", &os.PathError{Op: "open", Path: privatePath, Err: os.ErrPermission})
	response := httptest.NewRecorder()
	projectRunProblem(response, err)
	if response.Code != http.StatusInternalServerError {
		t.Fatal(response.Code, response.Body.String())
	}
	for _, private := range []string{privatePath, "private-account-state", "run-sensitive.json"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatal("private persistence path leaked", response.Body.String())
		}
	}
}

func TestProjectRunHTTPExactEvidenceAcceptanceAndHistoricalValidity(t *testing.T) {
	s, mux, p, req, _, _ := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", req, 201))
	useActualEvidenceProcesses(t, s)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var check ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(*state.Specification, *state.DesignPlan, "check.js"), 202).Body.Bytes(), &check)
	check = finishEvidenceRun(t, s, p.ID, check.ID)
	joined := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/checks", map[string]any{"expectedRevision": run.Revision, "checkId": check.ID}, 200))
	var health atomic.Int32
	health.Store(200)
	var healthHits atomic.Int32
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { healthHits.Add(1); w.WriteHeader(int(health.Load())) }))
	defer app.Close()
	release := storedRelease{Release: Release{ID: "rel-http-preview", ProjectID: p.ID, Target: "local", Commit: check.Commit, Status: "ready", URL: app.URL}, ContainerID: strings.Repeat("b", 64)}
	if err = s.storeRelease(release); err != nil {
		t.Fatal(err)
	}
	if err = s.RecordProjectRunReleaseProvenance(context.Background(), p.ID, release.ID, release.Target, release.Commit); err != nil {
		t.Fatal(err)
	}
	original := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if name != "docker" {
			return original(ctx, name, args, dir)
		}
		if len(args) == 4 && args[0] == "inspect" {
			return previewInspection(release), nil
		}
		return "", errors.New("run evidence must not mutate Docker")
	}
	observed := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/releases", map[string]any{"expectedRevision": joined.Revision, "releaseId": release.ID, "target": "local"}, 200))
	complete := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/accept", map[string]int{"expectedRevision": observed.Revision}, 200))
	if complete.State != "complete" || complete.Acceptance == nil || complete.Acceptance.CheckID != check.ID {
		t.Fatal(complete)
	}
	assertAcceptedSummary := func(record ProjectRunView, valid bool) {
		t.Helper()
		before, hits := auditTree(t, s.stateDir), healthHits.Load()
		var overview OrchestratorView
		decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/orchestrator", nil, 200).Body.Bytes(), &overview)
		summary := overview.Context.CurrentRun
		if summary == nil || summary.ID != record.ID || summary.State != "complete" || summary.Valid != valid {
			t.Fatal("accepted run lost from current understanding", summary)
		}
		wantReason, wantAction := record.Reason, "maintain"
		if !valid {
			wantReason, wantAction = record.Observation.Reason, "inspect-run"
		}
		if summary.Reason != wantReason || strings.Contains(summary.Reason, "accept the selected outcome") || overview.NextAction.ID != wantAction || overview.NextAction.RunID != record.ID {
			t.Fatal("summary repeats acceptance or hides current invalidity", summary, overview.NextAction)
		}
		retained := projectRunHTTPResponse(t, journeyRequest(t, mux, "GET", base+"/runs/"+run.ID, nil, 200))
		if healthHits.Load() != hits || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) || !reflect.DeepEqual(retained.Acceptance, record.Acceptance) || !reflect.DeepEqual(retained.Observation, record.Observation) {
			t.Fatal("reading accepted understanding probed a release or rewrote historical evidence")
		}
	}
	assertAcceptedSummary(complete, true)
	health.Store(503)
	drifted := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/reconcile", map[string]int{"expectedRevision": complete.Revision}, 200))
	if drifted.State != "complete" || drifted.Observation.Valid || drifted.Acceptance == nil {
		t.Fatal("health loss erased history or claimed current validity", drifted)
	}
	assertAcceptedSummary(drifted, false)
	before, hits := auditTree(t, s.stateDir), healthHits.Load()
	journeyRequest(t, mux, "GET", base+"/runs/"+run.ID, nil, 200)
	if healthHits.Load() != hits || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("GET polled a release or repaired run history")
	}
	// Bounded immutable evidence remains after ordinary check retention ends.
	s.mu.Lock()
	for i := 0; i < 21; i++ {
		copy := snapshotProjectCheck(check)
		copy.ID = newID("check-")
		if err = s.storeCheckRun(copy); err != nil {
			break
		}
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	retained := projectRunHTTPResponse(t, journeyRequest(t, mux, "GET", base+"/runs/"+run.ID, nil, 200))
	if retained.Checks[0].OriginalAvailable || retained.Checks[0].CheckID != check.ID || retained.State != "complete" {
		t.Fatal(retained)
	}
}
