package reaper_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type observedUpdateStore struct {
	registry.Store
	beforeUpdate func()
}

func (s *observedUpdateStore) Update(id string, fn func(*registry.Session) error) (registry.Session, error) {
	if s.beforeUpdate != nil {
		f := s.beforeUpdate
		s.beforeUpdate = nil
		f()
	}
	return s.Store.Update(id, fn)
}

type stoppedTaskBackend struct {
	*fakeBackend
	getHook     func()
	getValue    *backend.Sandbox
	getErr      error
	stopped     []backend.Sandbox
	stoppedErr  error
	stoppedHook func()
}

func (b *stoppedTaskBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	if b.getHook != nil {
		b.getHook()
	}
	if b.getErr != nil {
		return backend.Sandbox{}, b.getErr
	}
	if b.getValue != nil {
		return *b.getValue, nil
	}
	return b.fakeBackend.Get(ctx, id)
}

func (b *stoppedTaskBackend) DestroyStopped(_ context.Context, sb backend.Sandbox) error {
	if b.stoppedHook != nil {
		b.stoppedHook()
	}
	b.stopped = append(b.stopped, sb)
	return b.stoppedErr
}

type afterExitReadStore struct {
	registry.Store
	afterExit func()
}

func (s *afterExitReadStore) Get(id string) (registry.Session, error) {
	row, err := s.Store.Get(id)
	if err == nil && row.Status == registry.StatusExited && s.afterExit != nil {
		change := s.afterExit
		s.afterExit = nil
		change()
		return s.Store.Get(id)
	}
	return row, err
}

func TestStoppedTaskCleanupPreservesRestartAfterTerminalMark(t *testing.T) {
	for _, stage := range []string{"before cleanup read", "after cleanup read"} {
		t.Run(stage, func(t *testing.T) {
			store, row, b := stoppedTaskFixture(t)
			restart := func() {
				_, err := store.Update(row.ID, func(s *registry.Session) error {
					s.Status, s.SandboxID, s.SandboxRef = registry.StatusStarting, "sb-new", "container-new"
					s.AttachToken, s.Endpoint = "replacement-token", "127.0.0.1:49999"
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			var current registry.Store = store
			if stage == "before cleanup read" {
				current = &afterExitReadStore{Store: store, afterExit: restart}
			} else {
				b.stoppedHook = restart
			}
			if err := taskReaper(current, b, func(context.Context, registry.Session) error { return nil }).Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			got := get(t, store, row.ID)
			if got.Status != registry.StatusStarting || got.SandboxID != "sb-new" || got.SandboxRef != "container-new" || got.AttachToken != "replacement-token" || got.Endpoint != "127.0.0.1:49999" {
				t.Fatal("terminal cleanup overwrote replacement", got)
			}
			if b.destroyCount() != 0 {
				t.Fatal("replacement caused force-by-name cleanup")
			}
			if stage == "before cleanup read" && len(b.stopped) != 0 {
				t.Fatal("changed registry allocation was cleaned")
			}
			if stage == "after cleanup read" && (len(b.stopped) != 1 || b.stopped[0].Ref != row.SandboxRef) {
				t.Fatal("physical cleanup targeted the replacement rather than the observed immutable container")
			}
		})
	}
}

func stoppedTaskFixture(t *testing.T) (*registry.FileStore, registry.Session, *stoppedTaskBackend) {
	t.Helper()
	store := newStore(t)
	sb := managed("sb-task", backend.StateStopped)
	sb.Labels[backend.LabelSessionID] = "s-task"
	seed(t, store, registry.Session{ID: "s-task", ProjectID: "p-task", WorktreeID: "w-task", ProjectRunID: "run-task", ProjectRunOperationID: "op-task", SandboxID: sb.ID, SandboxRef: sb.Ref, Status: registry.StatusRunning, Endpoint: "127.0.0.1:49711", AttachToken: "private-test-token", AttachCount: 1, LastOutputAt: clock})
	return store, get(t, store, "s-task"), &stoppedTaskBackend{fakeBackend: &fakeBackend{sandboxes: []backend.Sandbox{sb}}}
}

func taskReaper(store registry.Store, b backend.Backend, proof func(context.Context, registry.Session) error) *reaper.Reaper {
	return reaper.New(reaper.Config{Store: store, Backend: b, Instance: ourInstance, Now: func() time.Time { return clock }, IdleAfter: time.Hour, ConfirmStoppedTaskCleanup: proof})
}

func TestObservedExitDoesNotOverwriteConcurrentRestart(t *testing.T) {
	for _, disappeared := range []bool{false, true} {
		t.Run(map[bool]string{false: "stopped", true: "disappeared"}[disappeared], func(t *testing.T) {
			store, row, b := stoppedTaskFixture(t)
			if disappeared {
				b.sandboxes = nil
			}
			wrapped := &observedUpdateStore{Store: store, beforeUpdate: func() {
				_, err := store.Update(row.ID, func(s *registry.Session) error {
					s.Status, s.SandboxID, s.SandboxRef = registry.StatusStarting, "sb-restarted", "container-new"
					s.Endpoint, s.AttachToken, s.AttachCount = "127.0.0.1:49999", "new-private-token", 2
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}}
			// The observation CAS applies even when expedited cleanup is disabled.
			if err := taskReaper(wrapped, b, nil).Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			got := get(t, store, row.ID)
			if got.Status != registry.StatusStarting || got.SandboxID != "sb-restarted" || got.SandboxRef != "container-new" || got.AttachToken != "new-private-token" || got.Endpoint != "127.0.0.1:49999" || got.AttachCount != 2 {
				t.Fatalf("old observation overwrote restart: %+v", got)
			}
			if b.destroyCount() != 0 || len(b.stopped) != 0 {
				t.Fatal("restart triggered cleanup")
			}
		})
	}
}

func TestConfirmedStoppedTaskIsRemovedInSameSweep(t *testing.T) {
	store, row, b := stoppedTaskFixture(t)
	calls := 0
	r := taskReaper(store, b, func(_ context.Context, actual registry.Session) error {
		calls++
		if actual.ID != row.ID || actual.SandboxRef != row.SandboxRef || get(t, store, row.ID).Status != registry.StatusRunning {
			t.Fatal("positive receipt was not captured before exit CAS")
		}
		return nil
	})
	if err := r.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := get(t, store, row.ID)
	if got.Status != registry.StatusExited || got.Endpoint != "" || got.AttachToken != "" || got.AttachCount != 0 {
		t.Fatal("missing exact terminal transition", got)
	}
	if calls != 1 || len(b.stopped) != 1 || b.stopped[0].Ref != row.SandboxRef || b.destroyCount() != 0 {
		t.Fatal("missing immutable same-sweep cleanup", calls, b.stopped, b.destroyed)
	}
}

func TestStoppedTaskCleanupRevalidatesSnapshotAndRetainsUncertainty(t *testing.T) {
	for _, scenario := range []string{"pending launch", "replacement", "running", "created", "inflight", "get unavailable", "proof changes registry", "remove conflict"} {
		t.Run(scenario, func(t *testing.T) {
			store, row, b := stoppedTaskFixture(t)
			proof := func(context.Context, registry.Session) error { return nil }
			switch scenario {
			case "pending launch":
				proof = func(context.Context, registry.Session) error { return errors.New("full Create has not returned") }
			case "replacement", "running", "created", "inflight":
				fresh := b.sandboxes[0]
				if scenario == "replacement" {
					fresh.Ref = "container-replacement"
				}
				if scenario == "running" {
					fresh.State = backend.StateRunning
				}
				if scenario == "created" {
					fresh.CreatedOnly = true
				}
				if scenario == "inflight" {
					fresh.CreationInFlight = true
				}
				b.getValue = &fresh
			case "get unavailable":
				b.getErr = backend.ErrUnavailable
			case "proof changes registry":
				proof = func(context.Context, registry.Session) error {
					_, err := store.Update(row.ID, func(s *registry.Session) error {
						s.Status, s.SandboxID, s.SandboxRef = registry.StatusStarting, "sb-new", "container-new"
						return nil
					})
					return err
				}
			case "remove conflict":
				b.stoppedErr = errors.New("container resumed; non-force removal refused")
			}
			r := taskReaper(store, b, proof)
			if err := r.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			if scenario == "remove conflict" {
				// The next exited-row sweep must not fall through to force-by-name.
				if err := r.Sweep(context.Background()); err != nil {
					t.Fatal(err)
				}
				if len(b.stopped) != 2 {
					t.Fatal("confirmed stopped cleanup was not retried")
				}
			} else if len(b.stopped) != 0 {
				t.Fatal("uncertain task was removed", b.stopped)
			}
			if b.destroyCount() != 0 {
				t.Fatal("uncertainty fell through to force cleanup")
			}
			if scenario == "proof changes registry" && get(t, store, row.ID).SandboxID != "sb-new" {
				t.Fatal("new generation changed")
			}
			if (scenario == "replacement" || scenario == "running" || scenario == "get unavailable") && get(t, store, row.ID).Status != registry.StatusRunning {
				t.Fatal("fresh running/unknown backend was marked exited")
			}
		})
	}
}

func TestTaskCleanupKeepsLegacyAndVerifiedSerialRunFallback(t *testing.T) {
	for _, scenario := range []string{"nil callback", "verified serial run"} {
		t.Run(scenario, func(t *testing.T) {
			store, _, b := stoppedTaskFixture(t)
			var proof func(context.Context, registry.Session) error
			if scenario == "verified serial run" {
				proof = func(context.Context, registry.Session) error { return backend.ErrUnsupported }
			}
			r := taskReaper(store, b, proof)
			if err := r.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			if b.destroyCount() != 0 {
				t.Fatal("legacy cleanup lost its existing sweep delay")
			}
			if err := r.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			if b.destroyCount() != 1 || len(b.stopped) != 0 {
				t.Fatal("legacy cleanup changed")
			}
		})
	}
}
