package api

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type swarmObserverBackend struct {
	backend.Backend
	get func(context.Context, string) (backend.Sandbox, error)
}

func TestSwarmCapacityReleaseRetainsUncertainOrMismatchedBackend(t *testing.T) {
	for _, tc := range []struct {
		name, rowSandbox, rowProject string
		sb                           backend.Sandbox
		err                          error
		wantHeld                     bool
	}{
		{"running", "s-exact", "p-owner", backend.Sandbox{ID: "s-exact", State: backend.StateRunning}, nil, true},
		{"stopped", "s-exact", "p-owner", backend.Sandbox{ID: "s-exact", State: backend.StateStopped}, nil, false},
		{"absent", "s-exact", "p-owner", backend.Sandbox{}, backend.ErrNotFound, false},
		{"transport ambiguity", "s-exact", "p-owner", backend.Sandbox{}, errors.Join(backend.ErrNotFound, backend.ErrUnavailable), true},
		{"canceled ambiguity", "s-exact", "p-owner", backend.Sandbox{}, errors.Join(backend.ErrNotFound, context.Canceled), true},
		{"backend wrong identity", "s-exact", "p-owner", backend.Sandbox{ID: "s-other", State: backend.StateStopped}, nil, true},
		{"registry wrong sandbox", "s-other", "p-owner", backend.Sandbox{}, backend.ErrNotFound, true},
		{"registry missing sandbox", "", "p-owner", backend.Sandbox{}, backend.ErrNotFound, true},
		{"registry foreign project", "s-exact", "p-foreign", backend.Sandbox{}, backend.ErrNotFound, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-owner"}
			if d, err := g.reserve(context.Background(), "q-exact", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil || d.State != "admitted" {
				t.Fatal(d, err)
			}
			if err := g.bindSandbox(context.Background(), "q-exact", "s-row", "s-exact"); err != nil {
				t.Fatal(err)
			}
			sessionsSeed(t, g.api.deps.Store, registry.Session{ID: "s-row", ProjectID: tc.rowProject, SandboxID: tc.rowSandbox, Status: registry.StatusExited})
			g.api.deps.Backend = swarmObserverBackend{Backend: g.api.deps.Backend, get: func(context.Context, string) (backend.Sandbox, error) { return tc.sb, tc.err }}
			err := g.release(context.Background(), "q-exact")
			if errors.Is(err, ErrExecutionLeaseHeld) != tc.wantHeld || err != nil && !tc.wantHeld {
				t.Fatal("incorrect release", err)
			}
			if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
				if (len(l.Leases) == 1) != tc.wantHeld {
					t.Fatal("capacity lost or stranded", l.Leases)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func (b swarmObserverBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	return b.get(ctx, id)
}

func TestSwarmExitedRowRequiresExactBackendStopBeforeGitReview(t *testing.T) {
	for _, tc := range []struct {
		name          string
		sb            backend.Sandbox
		err           error
		busy, failure bool
	}{
		{"still running", backend.Sandbox{ID: "s-exact", State: backend.StateRunning}, nil, true, false},
		{"still starting", backend.Sandbox{ID: "s-exact", State: backend.StateStarting}, nil, true, false},
		{"stopped", backend.Sandbox{ID: "s-exact", State: backend.StateStopped}, nil, false, false},
		{"gone", backend.Sandbox{ID: "s-exact", State: backend.StateGone}, nil, false, false},
		{"not found", backend.Sandbox{}, backend.ErrNotFound, false, false},
		{"unavailable", backend.Sandbox{}, backend.ErrUnavailable, false, true},
		{"ambiguous not found", backend.Sandbox{}, errors.Join(backend.ErrNotFound, backend.ErrUnavailable), false, true},
		{"deadline not found", backend.Sandbox{}, errors.Join(backend.ErrNotFound, context.DeadlineExceeded), false, true},
		{"wrong identity", backend.Sandbox{ID: "s-other", State: backend.StateStopped}, nil, false, true},
		{"unknown state", backend.Sandbox{ID: "s-exact"}, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			sessionsSeed(t, env.st, registry.Session{ID: "s-row", ProjectID: "p-owner", WorktreeID: "w-exact", SandboxID: "s-exact", Status: registry.StatusExited})
			calls := 0
			env.api.deps.Backend = swarmObserverBackend{Backend: env.be, get: func(_ context.Context, id string) (backend.Sandbox, error) {
				calls++
				if id != "s-exact" {
					t.Fatalf("checked another sandbox: %s", id)
				}
				return tc.sb, tc.err
			}}
			busy, err := env.api.observeSwarmWorktree(context.Background(), "p-owner", "w-exact")
			if busy != tc.busy || (err != nil) != tc.failure || calls != 1 {
				t.Fatalf("busy=%v err=%v calls=%d", busy, err, calls)
			}
			if busy, err := env.api.observeSwarmWorktree(context.Background(), "p-foreign", "w-exact"); busy || err != nil || calls != 1 {
				t.Fatal("another project's row affected this checkout")
			}
		})
	}
}
