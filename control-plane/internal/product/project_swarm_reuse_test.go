package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func reuseIntegratedFixture(t *testing.T) (*Server, *http.ServeMux, project.Project, ProjectRun, *ProjectRunSelection) {
	t.Helper()
	s, mux, p, run, base, selected := swarmHTTPFixture(t)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "reuse-original-attempt-12345")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, "alpha.txt", "original integrated prerequisite\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	s.registerTaskHandoffs(mux)
	s.SetTaskHandoffCollector(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error) {
		return TaskHandoffCapture{Release: func() {}, Status: "captured", ContextDigest: strings.Repeat("a", 64), Report: &TaskHandoffReport{Version: 1, Summary: "Original prerequisite worker claim", ChangedFiles: []string{"alpha.txt"}, Decisions: []TaskHandoffDecision{}, ReportedChecks: []TaskHandoffReportedCheck{}, Questions: []TaskHandoffQuestion{}, Limitations: []string{"Behavior remains independently checked."}}}, nil
	})
	var captured struct {
		Run ProjectRun `json:"run"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/operations/"+op.ID+"/handoff/collect", map[string]any{"expectedRevision": run.Revision}, 200).Body.Bytes(), &captured)
	run = captured.Run
	review := swarmReviewHTTP(t, mux, base, "task-a")
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	return s, mux, p, run, selected
}

func reuseCreateAffected(t *testing.T, s *Server, mux *http.ServeMux, p project.Project, selected *ProjectRunSelection, key string) ProjectRun {
	t.Helper()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	spec, design := currentEvidenceApprovals(state, p.ID)
	plan := currentPlan(state, p.ID)
	connection := selected.Connection.ID
	req := projectRunCreateRequest{IdempotencyKey: key, WorktreeID: selected.WorktreeID, Harness: selected.Harness, Model: selected.Model, PermissionMode: selected.PermissionMode, AgentConnectionID: &connection, Outcome: "preview", ImplementationContext: ImplementationContextRequest{ProjectID: p.ID, BriefRevision: state.Brief.Revision, PlanRevision: plan.Revision, SpecificationRevision: spec.Revision, DesignPlanRevision: design.Revision, TaskIDs: []string{"task-c"}}}
	return projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/runs", req, 201)).ProjectRun
}
func reuseRef(run ProjectRun) ProjectSwarmReuseReference {
	task := swarmTask(&run, "task-a")
	return ProjectSwarmReuseReference{RunID: run.ID, TaskID: task.ID, OperationID: task.OperationID, IntegrationDigest: implementationDigest(task.Integration)}
}

// The same test body fails the original preparation seam before the new
// immutable digest is populated. It does not infer proof from an old task ID.
func TestProjectSwarmReusePreparationRecordsImmutableInputs(t *testing.T) {
	_, _, _, run, _, _ := swarmHTTPFixture(t)
	bytes, _ := json.Marshal(run.Swarm.Tasks)
	var tasks []map[string]any
	if json.Unmarshal(bytes, &tasks) != nil {
		t.Fatal("task projection invalid")
	}
	for _, task := range tasks {
		digest, _ := task["taskInputDigest"].(string)
		if !hexDigest(digest) {
			t.Fatal("normal preparation omitted immutable task inputs")
		}
	}
}

func TestProjectSwarmReuseNativeIntegrationReopenAndFrozenDependencyWithoutReplay(t *testing.T) {
	s, mux, p, old, selected := reuseIntegratedFixture(t)
	oldBytes, _ := json.Marshal(old)
	// A real amendment changes only C, preserving the prerequisite contract.
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	plan := currentPlan(state, p.ID)
	plan.Tasks[2].Acceptance = "Use Alpha in the amended presentation"
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": plan.Revision, "expectedSpecificationRevision": plan.SpecificationRevision, "tasks": plan.Tasks}, 200)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Review affected task amendment"), 200)
	next := reuseCreateAffected(t, s, mux, p, selected, "reuse-affected-run-123456")
	next = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+next.ID+"/swarm", map[string]any{"expectedRevision": next.Revision, "maxParallel": 2, "reusedDependencies": []ProjectSwarmReuseReference{reuseRef(old)}}, 201)).ProjectRun
	if len(next.Swarm.Tasks) != 1 || next.Swarm.Tasks[0].ID != "task-c" || len(next.Operations) != 0 || len(next.Swarm.ReusedDependencies) != 1 {
		t.Fatal("reuse fabricated selected workers", next.Swarm)
	}
	dep := next.Swarm.ReusedDependencies[0]
	if dep.RunID != old.ID || dep.Task.Integration.Commit != swarmTask(&old, "task-a").Integration.Commit {
		t.Fatal("lost native integration identity")
	}
	// Reopen the actual private run store; no volatile reservation can release C.
	reopened, err := NewProjectRunStore(s.projectRuns.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	s.projectRuns = reopened
	next = swarmReserveHTTP(t, mux, base+"/runs/"+next.ID+"/swarm", next, "task-c", "reuse-only-affected-child-123")
	if len(next.Operations) != 1 || next.Operations[0].TaskID != "task-c" {
		t.Fatal("prerequisite was rerun")
	}
	packet, err := s.projectRuns.taskDependencyPacket(next, next.Operations[0])
	if err != nil || len(packet.Dependencies) != 1 {
		t.Fatal("missing frozen external dependency", err)
	}
	prior := packet.Dependencies[0]
	if prior.RunID != old.ID || prior.OperationID != dep.OperationID || prior.Integration == nil || implementationDigest(prior.Integration) != dep.IntegrationDigest || prior.Status != "captured" || prior.Receipt == nil || prior.Receipt.Reported == nil || prior.Receipt.Reported.Summary != "Original prerequisite worker claim" || len(prior.RecordedChecks) != 0 {
		t.Fatal("dependency attribution or unknown-report scope changed", prior)
	}
	actual, err := s.projectRuns.Get(p.ID, p.Path, old.ID)
	after, _ := json.Marshal(actual)
	if err != nil || string(oldBytes) != string(after) {
		t.Fatal("reuse rewrote original run", err)
	}
	// Idempotent reservation remains exact observation, including stale revision.
	replay := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+next.ID+"/swarm/tasks/task-c/reserve", map[string]any{"expectedRevision": 1, "idempotencyKey": "reuse-only-affected-child-123"}, 200)).ProjectRun
	if len(replay.Operations) != 1 || replay.Operations[0].ID != next.Operations[0].ID {
		t.Fatal("lost ACK replay allocated replacement")
	}
}

func TestProjectSwarmReuseRefusesMissingChangedForeignOrUnintegratedProof(t *testing.T) {
	for _, kind := range []string{"missing", "foreign-run", "wrong-operation", "wrong-integration", "legacy-input", "unknown-integration", "task-changed", "dependency-bytes-changed", "nonancestor", "dirty", "authority"} {
		t.Run(kind, func(t *testing.T) {
			s, mux, p, old, selected := reuseIntegratedFixture(t)
			ref := reuseRef(old)
			switch kind {
			case "foreign-run":
				ref.RunID = "run-" + strings.Repeat("a", 24)
			case "wrong-operation":
				ref.OperationID = "op-" + strings.Repeat("a", 24)
			case "wrong-integration":
				ref.IntegrationDigest = strings.Repeat("a", 64)
			case "legacy-input", "unknown-integration":
				var err error
				old, err = s.projectRuns.mutateSwarm(p.ID, p.Path, old.ID, old.Revision, func(r *ProjectRun) error {
					if kind == "legacy-input" {
						swarmTask(r, "task-a").TaskInputDigest = ""
					} else {
						swarmTask(r, "task-a").Integration.State = "unknown"
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				ref = reuseRef(old)
			case "task-changed":
				state, err := s.readApprovedJourney(p.Path, p.ID)
				if err != nil {
					t.Fatal(err)
				}
				plan := currentPlan(state, p.ID)
				plan.Tasks[0].Acceptance = "A new prerequisite behavior"
				base := "/v1/product/projects/" + p.ID
				journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": plan.Revision, "expectedSpecificationRevision": plan.SpecificationRevision, "tasks": plan.Tasks}, 200)
				journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Changed prerequisite"), 200)
			case "dependency-bytes-changed":
				swarmCommitFixture(t, p.Path, "alpha.txt", "later conflicting prerequisite bytes\n")
			case "nonancestor":
				confinedFixtureGit(t, p.Path, "reset", "--hard", old.Input.Implementation.SourceCommit)
			}
			next := reuseCreateAffected(t, s, mux, p, selected, "reuse-refused-run-1234567")
			refs := []ProjectSwarmReuseReference{ref}
			if kind == "missing" {
				refs = nil
			}
			if kind == "dirty" {
				if err := os.WriteFile(filepath.Join(p.Path, "alpha.txt"), []byte("uncommitted edit\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "authority" {
				s.projectRunSelectionResolver = func(context.Context, ProjectRunSelectionRequest) (ProjectRunSelection, error) {
					return ProjectRunSelection{}, errors.New("current execution revoked")
				}
			}
			before := auditTree(t, s.stateDir)
			journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/runs/"+next.ID+"/swarm", map[string]any{"expectedRevision": next.Revision, "maxParallel": 2, "reusedDependencies": refs}, 409)
			loaded, err := s.projectRuns.Get(p.ID, p.Path, next.ID)
			if err != nil || loaded.Swarm != nil || len(loaded.Operations) != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("refused proof allocated or mutated private state", err)
			}
		})
	}
}

func TestProjectSwarmReuseRevalidatesRetainedProofBeforeAllocation(t *testing.T) {
	for _, kind := range []string{"late-source", "late-origin", "corrupt-handoff"} {
		t.Run(kind, func(t *testing.T) {
			s, mux, p, old, selected := reuseIntegratedFixture(t)
			next := reuseCreateAffected(t, s, mux, p, selected, "reuse-late-proof-run-123456")
			base := "/v1/product/projects/" + p.ID + "/runs/" + next.ID + "/swarm"
			next = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": next.Revision, "maxParallel": 2, "reusedDependencies": []ProjectSwarmReuseReference{reuseRef(old)}}, 201)).ProjectRun
			switch kind {
			case "late-source":
				swarmCommitFixture(t, p.Path, "alpha.txt", "changed after admitted reuse\n")
			case "late-origin":
				var err error
				_, err = s.projectRuns.mutateSwarm(p.ID, p.Path, old.ID, old.Revision, func(r *ProjectRun) error { swarmTask(r, "task-a").Integration.State = "unknown"; return nil })
				if err != nil {
					t.Fatal(err)
				}
			case "corrupt-handoff":
				dir, err := s.projectRuns.handoffDirectory(p.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, next.Swarm.ReusedDependencies[0].HandoffViewDigest+".json"), []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := auditTree(t, s.stateDir)
			expected := 409
			if kind == "corrupt-handoff" {
				expected = 500
			}
			journeyRequest(t, mux, "POST", base+"/tasks/task-c/reserve", map[string]any{"expectedRevision": next.Revision, "idempotencyKey": "reuse-refuse-late-allocation-123"}, expected)
			loaded, err := s.projectRuns.Get(p.ID, p.Path, next.ID)
			if err != nil || len(loaded.Operations) != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("late proof drift allocated or rewrote state", err)
			}
		})
	}
}

func TestProjectSwarmReuseConcurrentPreparationHasOneCASWinner(t *testing.T) {
	s, mux, p, old, selected := reuseIntegratedFixture(t)
	next := reuseCreateAffected(t, s, mux, p, selected, "reuse-concurrent-run-123456")
	request, _ := json.Marshal(map[string]any{"expectedRevision": next.Revision, "maxParallel": 2, "reusedDependencies": []ProjectSwarmReuseReference{reuseRef(old)}})
	target := "/v1/product/projects/" + p.ID + "/runs/" + next.ID + "/swarm"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := make(chan int, 2)
	for range 2 {
		go func() {
			r := httptest.NewRequest("POST", target, bytes.NewReader(request)).WithContext(ctx)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			results <- w.Code
		}()
	}
	counts := map[int]int{}
	for range 2 {
		select {
		case code := <-results:
			counts[code]++
		case <-ctx.Done():
			t.Fatal("bounded concurrent preparation did not settle")
		}
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatal("exact preparation CAS did not select one winner", counts)
	}
	loaded, err := s.projectRuns.Get(p.ID, p.Path, next.ID)
	if err != nil || loaded.Revision != next.Revision+1 || len(loaded.Operations) != 0 || len(loaded.Swarm.ReusedDependencies) != 1 {
		t.Fatal("concurrent preparation created work or duplicate witnesses", err)
	}
}

func TestProjectSwarmReuseInputDigestTracksOnlyApplicableContract(t *testing.T) {
	s, _, p, _, _, _ := swarmHTTPFixture(t)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := swarmTaskInputDigests(state, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"board-only", "unrelated-task", "unrelated-requirement", "review-metadata", "prerequisite-contract", "linked-requirement", "global-constraint", "design-content"} {
		t.Run(kind, func(t *testing.T) {
			var next journeyState
			data, _ := json.Marshal(state)
			if json.Unmarshal(data, &next) != nil {
				t.Fatal("fixture clone")
			}
			next.approvalAuthority = state.approvalAuthority
			next.importedContext = state.importedContext
			switch kind {
			case "board-only":
				next.Plan.Tasks[0].Status = "doing"
			case "unrelated-task":
				next.Plan.Tasks[1].Acceptance = "Unrelated Beta amendment"
			case "unrelated-requirement":
				next.Specification.Requirements = append(next.Specification.Requirements, SpecificationRequirement{ID: "AC-unrelated", Title: "Unrelated", Priority: "must", Acceptance: "Another scope"})
			case "review-metadata":
				next.Specification.Revision++
				next.Specification.UpdatedAt = "renewed review"
				next.DesignPlan.SpecificationRevision = next.Specification.Revision
				next.DesignPlan.Revision++
				next.DesignPlan.UpdatedAt = "renewed review"
			case "design-content":
				next.DesignPlan.ImplementationNotes += "Changed global design constraint"
			case "prerequisite-contract":
				next.Plan.Tasks[0].Acceptance = "A new Alpha contract"
			case "linked-requirement":
				next.Specification.Requirements[0].Acceptance = "Changed linked behavior"
			case "global-constraint":
				next.Specification.Constraints += "Changed global constraint"
			}
			got, err := swarmTaskInputDigests(next, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			unchanged := kind == "board-only" || kind == "unrelated-task" || kind == "unrelated-requirement" || kind == "review-metadata"
			if (got["task-a"] == original["task-a"]) != unchanged || (got["task-c"] == original["task-c"]) != unchanged {
				t.Fatal("task/dependency digest misclassified changed scope", kind)
			}
		})
	}
}
