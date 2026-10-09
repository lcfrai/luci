package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type schedulerDisplayBackend struct {
	backend.Backend
	reads atomic.Int32
}

// Observe the request's Done channel being attached to the bounded reservation
// read. This does not replace a clock, filesystem lock, API route or reader.
type schedulerDisplayContext struct {
	context.Context
	snapshotReturned atomic.Bool
	readAttached     chan struct{}
	once             sync.Once
	early            atomic.Int32
}

func (c *schedulerDisplayContext) Done() <-chan struct{} {
	if !c.snapshotReturned.Load() {
		c.early.Add(1)
	} else {
		c.once.Do(func() { close(c.readAttached) })
	}
	return c.Context.Done()
}

func TestSwarmSchedulerDisplayReservationWaitUsesRequestContext(t *testing.T) {
	env, be := schedulerDisplayFixture(t)
	g := env.api.execution
	before := reservationTree(t, filepath.Dir(g.path))
	lock, err := os.Open(filepath.Join(filepath.Dir(g.path), "leases.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	unlock, err := lockExecutionLedger(context.Background(), lock)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &schedulerDisplayContext{Context: base, readAttached: make(chan struct{})}
	env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		snapshot := executionFixtureSnapshot()
		ctx.snapshotReturned.Store(true)
		return snapshot
	}
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		env.api.guarded.ServeHTTP(w, httptest.NewRequest("GET", sessionsTarget("/v1/product/swarm-scheduler"), nil).WithContext(ctx))
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("display handler did not settle during test cleanup")
		}
	}()
	select {
	case <-ctx.readAttached:
	case <-done:
		t.Fatal("reservation read did not attach to the request context")
	case <-time.After(2 * time.Second):
		t.Fatal("reservation read did not reach the request context")
	}
	started := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(executionReservationReadTimeout / 2):
		t.Fatal("request cancellation did not settle the held reservation read before its independent timeout")
	}
	if ctx.early.Load() != 0 || w.Body.Len() != 0 {
		t.Fatal("request context observation was early or cancelled read published a response", ctx.early.Load(), w.Body.Len())
	}
	if before != reservationTree(t, filepath.Dir(g.path)) || be.reads.Load() != 0 || controlledAllocationCount(env.be) != 0 || env.be.destroyCount() != 0 {
		t.Fatal("cancellation mutated retained demand or accessed execution backend")
	}
	t.Logf("real request cancellation settled attached bounded reservation read in %s while exact lock remained held", time.Since(started))
}

func (b *schedulerDisplayBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	b.reads.Add(1)
	return b.Backend.Get(ctx, id)
}

func (b *schedulerDisplayBackend) List(ctx context.Context) ([]backend.Sandbox, error) {
	b.reads.Add(1)
	return b.Backend.List(ctx)
}

// Use the real Product/API registrar and production reservation reader, without
// creating projects, Git children, a sampler, a scheduler loop or any worker.
func schedulerDisplayFixture(t *testing.T) (*sessionsEnv, *schedulerDisplayBackend) {
	t.Helper()
	p, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	store := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), store, store, func(d *Deps) {
		d.Product = p
		d.ExecutionStateDir = filepath.Join(t.TempDir(), "capacity")
		d.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
		d.ResourceSnapshot = executionFixtureSnapshot
	})
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-display-owner"}
	d, err := env.api.execution.reserve(context.Background(), "display-retained-charge", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if err != nil || d.State != "admitted" {
		t.Fatal("retained charge setup", d, err)
	}
	be := &schedulerDisplayBackend{Backend: env.be}
	env.api.deps.Backend = be
	return env, be
}

func TestSwarmSchedulerDisplayCancellationPreservesSharedCharge(t *testing.T) {
	for _, stage := range []string{"before-request", "after-cached-snapshot"} {
		t.Run(stage, func(t *testing.T) {
			env, be := schedulerDisplayFixture(t)
			g := env.api.execution
			before := reservationTree(t, filepath.Dir(g.path))
			lock, err := os.Open(filepath.Join(filepath.Dir(g.path), "leases.lock"))
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Close()
			unlock, err := lockExecutionLedger(context.Background(), lock)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { unlock() }()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				calls++
				cancel() // The request is cancelled while the cached supplier runs.
				return executionFixtureSnapshot()
			}
			if stage == "before-request" {
				cancel()
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", sessionsTarget("/v1/product/swarm-scheduler"), nil).WithContext(ctx)
			started := time.Now()
			env.api.guarded.ServeHTTP(w, req)
			elapsed := time.Since(started)
			// The exact reservation lock is still held. The original lost context
			// waited for its independent 100 ms timeout and published a response.
			if elapsed >= executionReservationReadTimeout/2 {
				t.Errorf("cancelled display waited for the independent reservation timeout: %s", elapsed)
			}
			if w.Body.Len() != 0 {
				t.Errorf("cancelled display published %d response bytes", w.Body.Len())
			}
			wantCalls := 1
			if stage == "before-request" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Errorf("cached snapshot calls = %d, want %d", calls, wantCalls)
			}
			t.Logf("cancelled stage=%s elapsed=%s snapshotCalls=%d responseBytes=%d; exact reservation lock remains held", stage, elapsed, calls, w.Body.Len())
			if before != reservationTree(t, filepath.Dir(g.path)) {
				t.Fatal("cancelled read changed retained ledger names, modes, timestamps or bytes")
			}
			unlock()
			unlock = func() {}
			calls = 0
			env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				calls++
				return executionFixtureSnapshot()
			}
			live := httptest.NewRecorder()
			env.api.guarded.ServeHTTP(live, httptest.NewRequest("GET", sessionsTarget("/v1/product/swarm-scheduler"), nil))
			var response struct {
				Resources struct {
					Reservations reservationHTTPView `json:"reservations"`
				} `json:"resources"`
			}
			if live.Code != 200 || json.Unmarshal(live.Body.Bytes(), &response) != nil || response.Resources.Reservations.Status != "ready" || calls != 1 {
				t.Fatal("subsequent live read did not recover", live.Code, live.Body.String(), calls)
			}
			reservationWant(t, response.Resources.Reservations, 1)
			if before != reservationTree(t, filepath.Dir(g.path)) {
				t.Fatal("live read rewrote or released the retained charge")
			}
			if be.reads.Load() != 0 || controlledAllocationCount(env.be) != 0 || env.be.destroyCount() != 0 {
				t.Fatal("display read observed or mutated an execution backend")
			}
		})
	}
}
