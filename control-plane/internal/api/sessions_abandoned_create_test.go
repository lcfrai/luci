package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// F1, part 1 and part 2 (docs/verification/2026-09-27-phase-2-results.md).
//
// The cockpit aborts POST /v1/sessions at its own timeout. The create ran on
// r.Context(), so that abort cancelled the Docker calls mid-flight and left a
// record `exited` still naming a container that stayed `Created` for an hour.
// Two properties, one per test: the create finishes what it started, and a
// create that fails takes its own container with it.

// abandonedCreate is a backend that can be held inside Create and that records
// whether the context it was given was cancelled. It is the only way to see
// which context the create ran on: every other fake ignores ctx, so the defect
// is invisible to them.
type abandonedCreate struct {
	mu       sync.Mutex
	ctxErr   error
	destroys []string

	entered chan struct{} // closed when Create is entered
	hold    chan struct{} // Create returns when this closes
	once    sync.Once
}

func newAbandonedCreate() *abandonedCreate {
	return &abandonedCreate{entered: make(chan struct{}), hold: make(chan struct{})}
}

func (b *abandonedCreate) Kind() backend.Kind { return backend.KindDocker }

func (b *abandonedCreate) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	b.once.Do(func() { close(b.entered) })
	select {
	case <-b.hold:
	case <-ctx.Done():
		b.mu.Lock()
		b.ctxErr = ctx.Err()
		b.mu.Unlock()
		return backend.Sandbox{}, ctx.Err()
	}
	return backend.Sandbox{
		ID: spec.ID, Kind: backend.KindDocker, Ref: "ref-" + spec.ID,
		Endpoint: "127.0.0.1:49200", State: backend.StateStarting, CreatedAt: time.Now().UTC(),
	}, nil
}

func (b *abandonedCreate) Get(_ context.Context, id string) (backend.Sandbox, error) {
	return backend.Sandbox{ID: id, Kind: backend.KindDocker, State: backend.StateRunning}, nil
}

func (b *abandonedCreate) List(context.Context) ([]backend.Sandbox, error) { return nil, nil }

func (b *abandonedCreate) Destroy(_ context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.destroys = append(b.destroys, id)
	return nil
}

func (b *abandonedCreate) Attach(context.Context, string, string) (*websocket.Conn, error) {
	return nil, backend.ErrNotFound
}

func (b *abandonedCreate) Close() error { return nil }

func (b *abandonedCreate) contextError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ctxErr
}

// sessionsDestroyed reads the fake's destroy log under its lock.
func sessionsDestroyed(be *sessionsBackend) []string {
	be.mu.Lock()
	defer be.mu.Unlock()
	return append([]string(nil), be.destroys...)
}

// Part 1: the create's own work does not run on the caller's context.
func TestCreateFinishesAfterTheCockpitGivesUpOnIt(t *testing.T) {
	t.Parallel()
	be := newAbandonedCreate()
	st := sessionsMustStore(t)
	// The shared env sets ReadyTimeout to 50 ms so its readiness watch cannot
	// slow a test down, and the create's own budget derives from it — 2 x 50 ms
	// is shorter than the window this test needs to hold Create open in. Two
	// seconds is still far below the test's own bounds and leaves the abort as
	// the only thing that could end the create early, which is the whole point.
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.Backend = be
		d.ReadyTimeout = 2 * time.Second
	})

	ctx, abort := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodPost, sessionsTarget("/v1/sessions"),
		strings.NewReader(sessionsCreateBody)).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	served := make(chan struct{})
	go func() {
		defer close(served)
		env.mux.ServeHTTP(rec, r)
	}()

	select {
	case <-be.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the handler never reached backend.Create")
	}
	// The cockpit's DEFAULT_TIMEOUT_MS fires: the fetch is aborted and the
	// connection goes away, which is what cancels r.Context().
	abort()
	select {
	case <-served:
		t.Fatal("the create returned the moment the cockpit gave up; it ran on the caller's context (F1)")
	case <-time.After(250 * time.Millisecond):
	}
	close(be.hold)
	<-served

	if err := be.contextError(); err != nil {
		t.Errorf("backend.Create's context ended with %v; a create must not run on the caller's", err)
	}
	sessions, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d session records, want 1", len(sessions))
	}
	if got := sessions[0].Status; got == registry.StatusExited {
		t.Errorf("the record is %q with reason %q; the abandoned create finished normally",
			got, sessions[0].ExitReason)
	}
	if sessions[0].Endpoint == "" {
		t.Error("the record carries no endpoint: the create's second write never happened")
	}
}

// Part 1b: the budget F1 gave the detached create is EXACTLY the grace the
// reaper spares a create with, and until now nothing held the two together.
// sessions.go derives one from the other — the comment on sessionCreateContext
// says "two numbers that must agree are derived from one function rather than
// written down twice" — but making the budget `time.Hour + CreateGraceFor(...)`
// left this package green.
//
// What fails when they drift: a create still running past the grace is one the
// reaper's rule 2 has stopped sparing, so it marks the session `exited`
// underneath a create that is still working, and the create then finishes onto
// a terminal record — the state the two tests in sessions_terminal_write_test.go
// exist for.
func TestTheCreateBudgetIsExactlyTheReapersCreateGrace(t *testing.T) {
	t.Parallel()
	const readyTimeout = 7 * time.Second
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.ReadyTimeout = readyTimeout
	})

	ctx, cancel := env.api.sessionCreateContext(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("the create context has no deadline: a create nothing bounds outlives every rule that spares it")
	}
	want := reaper.CreateGraceFor(readyTimeout)
	// The clock moves between the two calls, so the budget read back can be
	// short by a scheduling hiccup and never long by anything.
	if got := time.Until(deadline); got > want || got < want-time.Second {
		t.Errorf("the create budget is %v for a %v ReadyTimeout; the reaper spares such a create for %v. "+
			"A create outliving its grace is marked exited underneath itself",
			got, readyTimeout, want)
	}
}

// Part 2: a create that fails destroys whatever it created. Without this the
// container the failure left behind is nobody's until the reaper's rule 3a, and
// before task 25 it was nobody's at all.
func TestAFailedCreateDestroysItsOwnContainer(t *testing.T) {
	t.Parallel()
	env := sessionsNewEnv(t)
	env.be.createErr = backend.ErrUnavailable

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
	}

	sessions, err := env.st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d session records, want 1", len(sessions))
	}
	got := sessionsDestroyed(env.be)
	if len(got) != 1 || got[0] != sessions[0].SandboxID {
		t.Errorf("destroyed %v, want [%s]: a create that fails must take its own container with it",
			got, sessions[0].SandboxID)
	}
	if sessions[0].Status != registry.StatusExited {
		t.Errorf("the record is %q, want exited", sessions[0].Status)
	}
}
