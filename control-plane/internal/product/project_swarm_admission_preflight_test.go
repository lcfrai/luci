package product

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// CLOUD-021: ordinary source readers must not hide a deterministic refusal from
// the private dependency graph. Readiness is not permission to allocate: ready
// tasks still need current route authority, source exclusion and final CAS.
func TestSwarmReservationPrivateReadinessPrecedesSourceAdmission(t *testing.T) {
	s, mux, p, run, base, selected := swarmHTTPFixture(t)
	guarded := http.NewServeMux()
	s.registerProjectSwarm(s.guardProjectSourceRoutes(guarded))
	original := s.projectRunSelectionResolver
	calls := 0
	s.projectRunSelectionResolver = func(ctx context.Context, request ProjectRunSelectionRequest) (ProjectRunSelection, error) {
		calls++
		return original(ctx, request)
	}
	beforeState, beforeSource := auditTree(t, s.stateDir), auditTree(t, s.projectRoot())
	checkRefusal := func(t *testing.T, task, key string, revision int, expected string, wantCalls int) {
		t.Helper()
		release, err := s.AcquireGitSessionAdmission(context.Background(), p.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		prior := calls
		response := journeyRequest(t, guarded, "POST", base+"/tasks/"+task+"/reserve", map[string]any{"expectedRevision": revision, "idempotencyKey": key}, 409)
		var problem struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		decodeEvidenceResponse(t, response.Body.Bytes(), &problem)
		if problem.Error != "project_run_conflict" || problem.Message != expected {
			t.Fatalf("refusal=%+v; want %q", problem, expected)
		}
		if calls-prior != wantCalls {
			t.Fatalf("provider resolver calls=%d, want %d", calls-prior, wantCalls)
		}
		if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeSource, auditTree(t, s.projectRoot())) {
			t.Fatal("refused reservation changed private state, source or allocated workspace")
		}
	}
	t.Run("unintegrated dependency despite held reader", func(t *testing.T) {
		checkRefusal(t, "task-c", "preflight-dependency-123456", run.Revision, "Integrate this task's dependencies before starting it.", 0)
	})
	t.Run("stale revision stays stale", func(t *testing.T) {
		checkRefusal(t, "task-c", "preflight-stale-123456789", run.Revision-1, ErrProjectRunConflict.Error(), 0)
	})
	t.Run("ready task still needs source exclusion", func(t *testing.T) {
		checkRefusal(t, "task-a", "preflight-source-123456789", run.Revision, "Project source is busy. Wait before reserving this task.", 1)
	})
	t.Run("ready task preserves current authority", func(t *testing.T) {
		prior := s.projectRunSelectionResolver
		s.projectRunSelectionResolver = func(context.Context, ProjectRunSelectionRequest) (ProjectRunSelection, error) {
			calls++
			return ProjectRunSelection{}, errors.New("Current execution permission is revoked.")
		}
		defer func() { s.projectRunSelectionResolver = prior }()
		checkRefusal(t, "task-a", "preflight-authority-123456", run.Revision, "Current execution permission is revoked.", 1)
	})
	t.Run("ready task preserves route identity", func(t *testing.T) {
		selected.Connection.Revision++
		defer func() { selected.Connection.Revision-- }()
		checkRefusal(t, "task-a", "preflight-route-123456789", run.Revision, "The selected model connection or worktree changed. Restore its saved selection or start a new run.", 1)
	})
	// Positive integration remains the only way to release the dependent task.
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	run = swarmReserveHTTP(t, guarded, base, run, "task-a", "preflight-positive-a-123456")
	op := run.Operations[0]
	path := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	swarmCommitFixture(t, path, "alpha.txt", "accepted dependency\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	review := swarmReviewHTTP(t, guarded, base, "task-a")
	run = projectRunHTTPResponse(t, journeyRequest(t, guarded, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	if swarmTask(&run, "task-a").Integration.State != "integrated" {
		t.Fatal("dependency was not integrated")
	}
	run = swarmReserveHTTP(t, guarded, base, run, "task-c", "preflight-positive-c-123456")
	dependent := run.Operations[len(run.Operations)-1]
	if dependent.BaseCommit != swarmTask(&run, "task-a").Integration.Commit || dependent.Allocation != "ready" {
		t.Fatal("dependent reservation missed exact accepted base")
	}
	// Lost-response replay is readback before source/provider/readiness checks,
	// including the now occupied slot and stale original revision.
	prior := calls
	release, err := s.AcquireGitSessionAdmission(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	replay := projectRunHTTPResponse(t, journeyRequest(t, guarded, "POST", base+"/tasks/task-c/reserve", map[string]any{"expectedRevision": 1, "idempotencyKey": "preflight-positive-c-123456"}, 200)).ProjectRun
	release()
	if calls != prior || len(replay.Operations) != len(run.Operations) || replay.Operations[len(replay.Operations)-1].ID != dependent.ID {
		t.Fatal("idempotent reservation replay resolved or allocated new work")
	}
	_ = mux
}

func TestSwarmReservationPreflightIsRevalidatedAfterRouteResolution(t *testing.T) {
	s, _, p, run, base, _ := swarmHTTPFixture(t)
	guarded := http.NewServeMux()
	s.registerProjectSwarm(s.guardProjectSourceRoutes(guarded))
	original := s.projectRunSelectionResolver
	s.projectRunSelectionResolver = func(ctx context.Context, req ProjectRunSelectionRequest) (ProjectRunSelection, error) {
		_, err := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error { current.Swarm.State = "paused"; return nil })
		if err != nil {
			return ProjectRunSelection{}, err
		}
		return original(ctx, req)
	}
	source := auditTree(t, s.projectRoot())
	response := journeyRequest(t, guarded, "POST", base+"/tasks/task-a/reserve", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "preflight-final-cas-123456"}, 409)
	if !strings.Contains(response.Body.String(), ErrProjectRunConflict.Error()) {
		t.Fatal(response.Body.String())
	}
	latest, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || latest.Revision != run.Revision+1 || latest.Swarm.State != "paused" || len(latest.Operations) != 0 || !reflect.DeepEqual(source, auditTree(t, s.projectRoot())) {
		t.Fatal("preflight became allocation authority after state changed", err)
	}
}
