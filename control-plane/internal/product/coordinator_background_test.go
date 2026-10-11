package product

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func coordinatorReadyFixture(t *testing.T, s *Server, p project.Project, ctx context.Context, req coordinatorContinuationRequest) coordinatorContinuationRecord {
	t.Helper()
	row := coordinatorPendingFixture(t, s, p, ctx, req)
	row.Actions = []CoordinatorAction{}
	row.Status = "ready"
	row.Revision = 1
	var err error
	row.ExecutionActor, err = management.CaptureExecutionActor(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	rows, err := s.readContinuations(p, row.Actor.ID)
	if err == nil {
		rows.Rows[findContinuation(rows, row.ID)] = row
		err = s.saveContinuations(rows)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return row
}
func coordinatorFixtureReauthorize(ctx context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
	if ref.UserID != "alice" || ref.Local || harness != "codex" || !secrets {
		return nil, errCoordinatorAuthority
	}
	return management.WithExecutionAuthority(ctx, management.ExecutionAuthority{Actor: management.User{ID: "alice", Role: "member"}}), nil
}
func TestCoordinatorBackgroundContinuesWithoutBrowserAndReopensWithoutReplay(t *testing.T) {
	s, p, _, request, req, starts := coordinatorContinuationFixture(t)
	request, cancel := context.WithCancel(request)
	row := coordinatorReadyFixture(t, s, p, request, req)
	cancel() // The originating browser context is gone, not retained as authority.
	s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err := s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.mu.RLock()
	saved, err := s.readContinuations(p, "alice")
	s.mu.RUnlock()
	if err != nil || saved.Rows[0].Status != "queued" || len(saved.Rows[0].Actions) != 3 || *starts != 0 {
		t.Fatalf("background did not prepare queue: %+v %v", saved, err)
	}
	runID := saved.Rows[0].RunID
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.SetCoordinatorAuthorizer(s.coordinatorAuthorize)
	reopened.SetCoordinatorExecutionAuthorizer(s.coordinatorExecuteAuthorize)
	reopened.SetProjectRunSelectionResolver(s.projectRunSelectionResolver)
	reopened.swarmScheduler.runtime = s.swarmScheduler.runtime
	reopened.branchSessions = s.branchSessions
	reopened.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	reopened.coordinatorBackground.authorize = coordinatorFixtureReauthorize
	if err = reopened.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	runs, err := reopened.projectRuns.List(p.ID, p.Path)
	if err != nil || len(runs) != 1 || runs[0].ID != runID || *starts != 0 {
		t.Fatal("restart replayed typed operation", err)
	}
	retained, err := reopened.readContinuations(p, "alice")
	if err != nil || retained.Rows[0].ID != row.ID {
		t.Fatal("restart lost exact continuation", err)
	}
}
func TestCoordinatorBackgroundCurrentAuthorityOptOutAndUnknownAreReadOnly(t *testing.T) {
	for _, kind := range []string{"revoked", "policy", "unknown", "foreign", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			s, p, _, ctx, req, starts := coordinatorContinuationFixture(t)
			row := coordinatorReadyFixture(t, s, p, ctx, req)
			s.coordinatorBackground.authorize = coordinatorFixtureReauthorize
			switch kind {
			case "revoked":
				s.coordinatorBackground.authorize = func(context.Context, management.ExecutionActorRef, string, bool) (context.Context, error) {
					return nil, errors.New("current grant revoked")
				}
			case "foreign":
				s.coordinatorBackground.authorize = func(ctx context.Context, _ management.ExecutionActorRef, _ string, _ bool) (context.Context, error) {
					return management.WithExecutionAuthority(ctx, management.ExecutionAuthority{Actor: management.User{ID: "bob", Role: "member"}}), nil
				}
			case "policy":
				s.mu.Lock()
				policy, _ := s.readCoordinatorPolicy(p)
				policy.Revision++
				policy.ExecutionMode = "manual"
				scope, _ := s.approvalScopes(p.Path, p.ID)
				path, _ := s.coordinatorPolicyPath(p.ID, false)
				err := durableRunJSON(path, storedCoordinatorPolicy{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Policy: policy})
				s.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			case "unknown":
				s.mu.Lock()
				rows, _ := s.readContinuations(p, "alice")
				rows.Rows[0].Status = "unknown"
				rows.Rows[0].Actions = []CoordinatorAction{{ID: row.ID + "-create-run", Kind: "create-run", Status: "unknown", StartedAt: row.CreatedAt}}
				err := s.saveContinuations(rows)
				s.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			}
			path, _ := s.continuationPath(p.ID, "alice", false)
			before, _ := os.ReadFile(path)
			background := context.Background()
			if kind == "cancelled" {
				var cancel context.CancelFunc
				background, cancel = context.WithCancel(background)
				cancel()
			}
			err := s.RunCoordinatorOnce(background)
			if kind != "unknown" && err == nil {
				t.Fatal("invalid current authority/policy accepted")
			}
			if kind == "unknown" && err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(path)
			runs, _ := s.projectRuns.List(p.ID, p.Path)
			if string(before) != string(after) || len(runs) != 0 || *starts != 0 {
				t.Fatal("refusal/unknown caused effects")
			}
		})
	}
}
func TestCoordinatorBackgroundLoopStopsWithServer(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	coordinatorReadyFixture(t, s, p, ctx, req)
	entered := make(chan struct{})
	exited := make(chan struct{})
	s.SetCoordinatorRuntime(func(ctx context.Context, _ management.ExecutionActorRef, _ string, _ bool) (context.Context, error) {
		close(entered)
		<-ctx.Done()
		close(exited)
		return nil, ctx.Err()
	})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("saved work did not wake without a browser")
	}
	s.Close()
	select {
	case <-exited:
	default:
		t.Fatal("server close left coordinator active")
	}
	select {
	case <-s.coordinatorBackground.done:
	default:
		t.Fatal("coordinator loop did not settle")
	}
}
func TestCoordinatorSavedRouteIsExplicitActorScopedAndCASBound(t *testing.T) {
	s, p, h, _, req, _ := coordinatorContinuationFixture(t)
	req.RememberRoute = true
	first := coordinatorCreate(t, h, p, req, 201)
	var view struct {
		Continuations []CoordinatorContinuation `json:"continuations"`
		SavedRoute    *CoordinatorSavedRoute    `json:"savedRoute"`
	}
	base := "/v1/product/projects/" + p.ID + "/orchestrator/continuations"
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", base, nil, 200).Body.Bytes(), &view)
	if view.SavedRoute == nil || view.SavedRoute.Revision != 1 || view.SavedRoute.Route.Harness != req.Run.Harness || view.SavedRoute.Route.AgentConnectionID == nil || *view.SavedRoute.Route.AgentConnectionID != *req.Run.AgentConnectionID {
		t.Fatal("explicit nonsecret route missing", view)
	}
	path, _ := s.continuationPath(p.ID, "alice", false)
	before, _ := os.ReadFile(path)
	replay := coordinatorCreate(t, h, p, req, 200)
	if replay.ID != first.ID {
		t.Fatal("route replay changed continuation")
	}
	stale := 0
	req.ExpectedRouteRevision = &stale
	req.IdempotencyKey = "route-stale-request-12345"
	journeyRequest(t, h, "POST", base, req, 409)
	current := 1
	req.ExpectedRouteRevision = &current
	req.Run.Model = "different-model"
	req.IdempotencyKey = "route-swapped-request-12345"
	journeyRequest(t, h, "POST", base, req, 409)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read/replay/stale route rewrote saved configuration")
	}
}

func TestCoordinatorBackgroundCancelledFirstCandidateDoesNotStarvePeer(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	coordinatorReadyFixture(t, s, p, ctx, req)
	req.IdempotencyKey = "second-independent-key-12345"
	coordinatorReadyFixture(t, s, p, ctx, req)
	entered := make(chan struct{})
	stopped := make(chan error, 1)
	first := true
	s.coordinatorBackground.authorize = func(ctx context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
		if first {
			first = false
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return coordinatorFixtureReauthorize(ctx, ref, harness, secrets)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	go func() { stopped <- s.RunCoordinatorOnce(cancelled) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first candidate not observed")
	}
	cancel()
	select {
	case err := <-stopped:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled candidate did not settle")
	}
	if err := s.RunCoordinatorOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	queued, ready := 0, 0
	for _, row := range rows.Rows {
		if row.Status == "queued" {
			queued++
		}
		if row.Status == "ready" {
			ready++
		}
	}
	if queued != 1 || ready != 1 {
		t.Fatalf("slow first prevented independent peer: queued=%d ready=%d", queued, ready)
	}
}
