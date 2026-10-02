package reaper_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

var clock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

// offendingHost is the address a LAN-published sandbox was bound to. Ruling 2
// requires it to reach the log, so it is a constant the assertions can look for.
const offendingHost = "0.0.0.0"

// fakeBackend answers List from a fixed slice and records every Destroy. Create
// and Attach are never called: the reaper only ever lists and destroys, and a
// panic here would be a real finding.
type fakeBackend struct {
	mu         sync.Mutex
	sandboxes  []backend.Sandbox
	listErr    error
	destroyErr map[string]error
	destroyed  []string
}

func (f *fakeBackend) Kind() backend.Kind { return backend.KindDocker }

func (f *fakeBackend) Create(context.Context, backend.Spec) (backend.Sandbox, error) {
	panic("reaper called Create")
}

func (f *fakeBackend) Attach(context.Context, string, string) (*websocket.Conn, error) {
	panic("reaper called Attach")
}

// Get follows backend.Sandbox's documented split: List keeps a LAN-published
// sandbox in the slice with NonLoopback set, and Get on the same id refuses
// with an error naming the address (dockerbe's offLoopbackError, docker.go:446).
// The fake honours that, because it is the channel the reaper's log reads the
// offending address from — a fake that answered Get cleanly would let the log
// go silent with every test still green.
func (f *fakeBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sandboxes {
		if s.ID != id {
			continue
		}
		if s.NonLoopback {
			return backend.Sandbox{}, fmt.Errorf("fake: 8080/tcp is published on %q instead of 127.0.0.1: %w",
				offendingHost, backend.ErrNonLoopbackPublish)
		}
		return s, nil
	}
	return backend.Sandbox{}, backend.ErrNotFound
}

func (f *fakeBackend) List(context.Context) ([]backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]backend.Sandbox, len(f.sandboxes))
	copy(out, f.sandboxes)
	return out, nil
}

func (f *fakeBackend) Destroy(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.destroyErr[id]; err != nil {
		return err
	}
	f.destroyed = append(f.destroyed, id)
	return nil
}

func (f *fakeBackend) Close() error { return nil }

func (f *fakeBackend) wasDestroyed(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.destroyed {
		if d == id {
			return true
		}
	}
	return false
}

func (f *fakeBackend) destroyCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.destroyed)
}

func newStore(t *testing.T) *registry.FileStore {
	t.Helper()
	store, err := registry.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}

func seed(t *testing.T, store registry.Store, s registry.Session) {
	t.Helper()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = clock.Add(-time.Hour)
	}
	if err := store.Create(s); err != nil {
		t.Fatalf("seed %s: %v", s.ID, err)
	}
}

func get(t *testing.T, store registry.Store, id string) registry.Session {
	t.Helper()
	s, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return s
}

// ourInstance is the instance id every reaper under test owns, unless a test
// builds one for another instance on purpose.
const ourInstance = "11111111-1111-4111-8111-111111111111"

// theirInstance is a second control plane on the same daemon.
const theirInstance = "22222222-2222-4222-8222-222222222222"

// managed is a sandbox this control plane created: every label set, endpoint on
// loopback, and old enough that no create can still be in flight for it. Tests
// override the fields they care about.
func managed(id string, state backend.State) backend.Sandbox {
	return ownedBy(ourInstance, id, state)
}

// ownedBy is managed for any instance. An empty instance is a sandbox created
// before per-instance ownership existed: LabelManaged and no LabelInstance.
func ownedBy(instance, id string, state backend.State) backend.Sandbox {
	sb := backend.Sandbox{
		ID:        id,
		Kind:      backend.KindDocker,
		Ref:       "container-" + id,
		Endpoint:  "127.0.0.1:49711",
		Instance:  instance,
		State:     state,
		CreatedAt: clock.Add(-time.Hour),
		Labels: map[string]string{
			backend.LabelManaged:   "true",
			backend.LabelSandboxID: id,
		},
	}
	if instance != "" {
		sb.Labels[backend.LabelInstance] = instance
	}
	return sb
}

// reaperFor is newReaper for a named instance, with its own registry: a second
// control plane on the same daemon.
func reaperFor(t *testing.T, instance string, store registry.Store, be backend.Backend) (*reaper.Reaper, *bytes.Buffer) {
	t.Helper()
	logs := &bytes.Buffer{}
	r := reaper.New(reaper.Config{
		Store:    store,
		Backend:  be,
		Instance: instance,
		Interval: time.Minute,
		Now:      func() time.Time { return clock },
		Log:      slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	return r, logs
}

func newReaper(t *testing.T, store registry.Store, be backend.Backend, idleAfter, idleTimeout time.Duration) (*reaper.Reaper, *bytes.Buffer) {
	t.Helper()
	logs := &bytes.Buffer{}
	r := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		Instance:    ourInstance,
		IdleAfter:   idleAfter,
		IdleTimeout: idleTimeout,
		Interval:    time.Minute, // no test calls Run
		Now:         func() time.Time { return clock },
		Log:         slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	return r, logs
}

// Rule 1. This is the rule that protects a night's work from a Docker Desktop
// restart, and it is the reason Sweep returns an error instead of swallowing one.
func TestSweepChangesNothingWhenTheBackendIsDown(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	be := &fakeBackend{listErr: errors.New("cannot connect to the Docker daemon")}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err == nil {
		t.Fatal("Sweep reported success while the backend was down")
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("status = %q, want %q: a failed List must never be read as \"every sandbox is gone\"", got, registry.StatusRunning)
	}
	if be.destroyCount() != 0 {
		t.Errorf("destroyed %d sandboxes with no usable list", be.destroyCount())
	}
	if r.BackendReady() {
		t.Error("BackendReady stayed true after List failed")
	}
}

// brokenRegistry is the real FileStore with one method replaced, so everything
// the reaper does apart from List behaves exactly as in production.
type brokenRegistry struct {
	registry.Store
	err error
}

func (b brokenRegistry) List() ([]registry.Session, error) { return nil, b.err }

// backendReady answers one question — did the backend answer — and contract
// §5.3 step 7 refreshes it on a sweep that reached the end. A sweep that dies on
// the REGISTRY has still learned the answer, so reporting the backend as down
// would send an operator after Docker for a fault in a JSON file on disk.
func TestBackendStaysReadyWhenTheRegistryIsTheThingThatFailed(t *testing.T) {
	t.Parallel()
	store := brokenRegistry{Store: newStore(t), err: errors.New("sessions.json is not JSON")}
	be := &fakeBackend{} // the daemon is perfectly healthy and answers List
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err == nil {
		t.Fatal("Sweep hid an unreadable registry")
	}
	if !r.BackendReady() {
		t.Error("BackendReady is false for a backend that just answered List; the fault was the registry's")
	}
}

// Rule 2.
func TestSweepMarksADisappearedSandboxExited(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	seed(t, store, registry.Session{ID: "s-2", Status: registry.StatusStarting}) // no sandbox yet
	be := &fakeBackend{}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	s1 := get(t, store, "s-1")
	if s1.Status != registry.StatusExited || s1.ExitReason != "sandbox disappeared" {
		t.Errorf("s-1 = (%q, %q), want (exited, \"sandbox disappeared\")", s1.Status, s1.ExitReason)
	}
	if got := get(t, store, "s-2").Status; got != registry.StatusStarting {
		t.Errorf("s-2 = %q: a session with no SandboxID has no sandbox to have lost", got)
	}
	if !r.BackendReady() {
		t.Error("BackendReady is false after a successful sweep")
	}
}

// Rule 2's grace condition, decision (f). The fixture above is the easy half —
// a record with no SandboxID has no sandbox to have lost. The hard half is the
// one task 11 actually writes: SandboxID is set BEFORE backend.Create is
// called, so a sweep landing in that window sees an id no container answers to
// yet, and exited is terminal. Both directions are here, because a grace that
// never expires is a leak rather than a fix.
func TestSweepSparesASessionWhoseCreateIsStillInFlight(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-new", SandboxID: "sb-new", Status: registry.StatusStarting,
		CreatedAt: clock.Add(-2 * time.Second), // Store.Create has run; ContainerCreate has not returned
	})
	seed(t, store, registry.Session{
		ID: "s-stale", SandboxID: "sb-stale", Status: registry.StatusStarting,
		CreatedAt: clock.Add(-10 * time.Minute), // nothing has been in flight for ten minutes
	})
	be := &fakeBackend{} // List succeeds and is empty: neither container exists
	r := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		Instance:    ourInstance,
		Interval:    time.Minute,
		CreateGrace: 90 * time.Second,
		Now:         func() time.Time { return clock },
		Log:         slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-new").Status; got != registry.StatusStarting {
		t.Errorf("s-new = %q, want starting: a sweep inside the create window must not make a terminal decision", got)
	}
	s := get(t, store, "s-stale")
	if s.Status != registry.StatusExited || s.ExitReason != "sandbox disappeared" {
		t.Errorf("s-stale = (%q, %q), want (exited, \"sandbox disappeared\"): the grace is a window, not an exemption", s.Status, s.ExitReason)
	}
}

// The grace has to survive a RESTART, and that is what decides which timestamp
// it reads. registry.FileStore pins CreatedAt across Update by design
// (filestore.go:210), so a session restarted an hour after birth still carries
// an hour-old CreatedAt — a grace measured from it protects a first create and
// nothing else, which is to say it misses every restart anybody actually
// performs. UpdatedAt moves on the write that names the new sandbox
// (sessions.go: the restart handler writes SandboxID before backend.Create), so
// the window opens exactly when the create begins.
func TestSweepSparesASessionRestartedLongAfterItWasCreated(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-restarting", SandboxID: "sb-new", Status: registry.StatusStarting,
		CreatedAt: clock.Add(-4 * time.Hour),   // born this morning
		UpdatedAt: clock.Add(-2 * time.Second), // restarted two seconds ago
	})
	seed(t, store, registry.Session{
		ID: "s-abandoned", SandboxID: "sb-old", Status: registry.StatusStarting,
		CreatedAt: clock.Add(-4 * time.Hour),
		UpdatedAt: clock.Add(-4 * time.Hour), // nothing has touched it since
	})
	be := &fakeBackend{} // neither container exists
	r := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		Instance:    ourInstance,
		Interval:    time.Minute,
		CreateGrace: 90 * time.Second,
		Now:         func() time.Time { return clock },
		Log:         slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-restarting").Status; got != registry.StatusStarting {
		t.Errorf("s-restarting = %q, want starting: its create began two seconds ago, whatever its age", got)
	}
	if got := get(t, store, "s-abandoned").Status; got != registry.StatusExited {
		t.Errorf("s-abandoned = %q, want exited: keying on UpdatedAt must not become an exemption for old records", got)
	}
}

// The grace has to span BOTH halves of backend.Create, and one ready timeout
// buys only the second. dockerbe computes its deadline after ContainerCreate and
// ContainerStart have returned (docker.go:207 and :211 versus :219), so a grace
// equal to the ready timeout is structurally short by however long the daemon
// spends creating and starting the container — and a session whose create is
// proceeding normally gets written `exited`, which is terminal.
//
// The magnitude is pinned through the real path rather than by asserting on
// CreateGraceFor's arithmetic: this test leaves Config.CreateGrace unset, so it
// exercises exactly what production gets.
func TestCreateGraceSpansContainerCreateAndNotOnlyTheReadyWait(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	// Past one ready timeout, inside two: the container was slow to create and
	// is now in its ready wait. This is a live create, not a lost one.
	seed(t, store, registry.Session{
		ID: "s-slow", SandboxID: "sb-slow", Status: registry.StatusStarting,
		UpdatedAt: clock.Add(-(backend.ReadyTimeout + 10*time.Second)),
	})
	// Past both: nothing takes this long, and a grace that never expires is a
	// leak rather than a fix.
	seed(t, store, registry.Session{
		ID: "s-lost", SandboxID: "sb-lost", Status: registry.StatusStarting,
		UpdatedAt: clock.Add(-(reaper.CreateGraceFor(backend.ReadyTimeout) + 10*time.Second)),
	})
	be := &fakeBackend{} // neither container exists
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-slow").Status; got != registry.StatusStarting {
		t.Errorf("s-slow = %q, want starting: one ready timeout does not cover ContainerCreate as well", got)
	}
	if got := get(t, store, "s-lost").Status; got != registry.StatusExited {
		t.Errorf("s-lost = %q, want exited: the grace is a window, not an exemption", got)
	}
}

// The grace is a window on STATUS as well as on age. A running session is one
// whose create finished, so a missing container really is a missing container
// however young the record is — and skipping it would hide the very
// disappearance rule 2 exists to report.
func TestSweepDoesNotExtendTheCreateGraceToARunningSession(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		CreatedAt: clock.Add(-2 * time.Second),
	})
	be := &fakeBackend{}
	r := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		Instance:    ourInstance,
		Interval:    time.Minute,
		CreateGrace: 90 * time.Second,
		Now:         func() time.Time { return clock },
		Log:         slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusExited {
		t.Errorf("s-1 = %q, want exited: its create finished, so the grace does not apply", got)
	}
}

// Rule 3.
func TestSweepMarksAStoppedSandboxExited(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateStopped)}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited || s.ExitReason != "process exited" {
		t.Errorf("s-1 = (%q, %q), want (exited, \"process exited\")", s.Status, s.ExitReason)
	}
	if be.wasDestroyed("sb-1") {
		t.Error("a stopped sandbox with a live record was destroyed in the sweep that ended its session;" +
			" rule 3a collects it on the next sweep, once the record is exited in the snapshot")
	}
}

// Rule 3, the other half of what it writes. A record that keeps its endpoint and
// its attach token after the process ended names a port the OS is free to hand
// to another container within seconds, and holds a secret for a sandbox that is
// about to be destroyed — the reason the API's own exit path clears both
// (sessions.go's markSessionExited). Rule 3 left them. That was rare before F3
// was fixed and is now the way EVERY session ends.
func TestSweepClearsTheEndpointAndTokenOfASessionItEnds(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		Endpoint: "127.0.0.1:49711", AttachToken: "tok-abc",
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateStopped)}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	s := get(t, store, "s-1")
	if s.Endpoint != "" || s.AttachToken != "" {
		t.Errorf("endpoint/token = %q/%q after the session exited, want both empty", s.Endpoint, s.AttachToken)
	}
	if s.SandboxID != "sb-1" {
		t.Errorf("sandboxId = %q, want sb-1: it is the provenance, and rule 3a needs it", s.SandboxID)
	}
}

// Rule 3a, and it is the rule F1 and F3 both need.
//
// F3: rule 3 writes `exited` over the record and leaves the container on disk,
// because rule 4 counts an exited record as tracking its sandbox. Before the
// daemon exited with its process that was rare; now that every session ends in
// a stopped container, it is every session.
//
// F1: a create the cockpit abandoned left a record `exited` — "the sandbox
// could not be created" — still naming a container that was `Created` an hour
// later, and no rule would take it (docs/verification/2026-09-27-phase-2-results.md).
func TestSweepDestroysTheSandboxOfAnExitedSession(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1",
		Status: registry.StatusExited, ExitReason: "process exited",
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateStopped)}}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("sb-1") {
		t.Error("the sandbox of an exited session was left behind; nothing else in the phase removes it")
	}
	// The record is the provenance: it is what says which container this was,
	// and it is already terminal, so the sweep must not rewrite it.
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited || s.ExitReason != "process exited" || s.SandboxID != "sb-1" {
		t.Errorf("s-1 = (%q, %q, %q), want (exited, \"process exited\", sb-1) unchanged",
			s.Status, s.ExitReason, s.SandboxID)
	}
	if !strings.Contains(logs.String(), "its session has already exited") {
		t.Errorf("the log does not say why the sandbox was destroyed; got %q", logs.String())
	}
}

// F1's shape exactly: the container the cockpit's abort left half-made. It is
// not stopped — it was created and never started — and the record went straight
// to exited without one, so only a rule that keys on the RECORD can find it.
func TestSweepReclaimsTheContainerAnAbandonedCreateLeftBehind(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "s-1",
		Status: registry.StatusExited, ExitReason: "the sandbox could not be created",
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("s-1", backend.StateStarting)}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("s-1") {
		t.Error("the container of a create that failed was left Created; this is F1's leak")
	}
}

// Rule 3a's NonLoopback guard, on the one path that reaches it. A successful
// rule-1a destroy lands in `gone` and rule 3a skips the sandbox for that reason
// alone, so the guard is only load-bearing when 1a's destroy FAILED — and
// deleting it left the package green until this test existed.
//
// What fails without it: a sandbox that is still published on the LAN, whose
// session happens to be exited, is destroyed here instead and logged Info as
// "its session has already exited". That is a live security defect relabelled
// as routine litter — the same mislabelling rule 4's own comment says the
// distinct messages exist to prevent — and the operator never learns the port
// was exposed.
func TestSweepDoesNotRelabelAnExposedSandboxAsAnExitedSessionsLitter(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1",
		Status: registry.StatusExited, ExitReason: "process exited",
	})
	exposed := managed("sb-1", backend.StateStopped)
	exposed.Endpoint = ""
	exposed.NonLoopback = true
	be := &fakeBackend{
		sandboxes:  []backend.Sandbox{exposed},
		destroyErr: map[string]error{"sb-1": errors.New("daemon says no")},
	}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	line := logs.String()
	if !strings.Contains(line, "still reachable from the LAN") {
		t.Errorf("rule 1a did not report the sandbox it could not take off the LAN:\n%s", line)
	}
	// Both of rule 3a's messages, its success and its retry, and neither of
	// rule 1a's: the substring is the shortest one that spans the two. Matching
	// only the Info form is how the first draft of this test let the mutant
	// through — the deleted guard sends this sandbox down rule 3a's FAILURE
	// path, which is worded differently.
	if strings.Contains(line, "session has already exited") {
		t.Errorf("rule 3a claimed a sandbox that is still published off 127.0.0.1, "+
			"which files a security defect under routine litter:\n%s", line)
	}
	if !strings.Contains(line, offendingHost) {
		t.Errorf("the operator is not told where the port is still published:\n%s", line)
	}
}

// A failed destroy is a retry, never a rewritten record: the container is still
// there, and saying otherwise would lose the only id that can find it again.
func TestSweepKeepsTheRecordOfAnExitedSessionWhoseSandboxWillNotDie(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1",
		Status: registry.StatusExited, ExitReason: "process exited",
	})
	be := &fakeBackend{
		sandboxes:  []backend.Sandbox{managed("sb-1", backend.StateStopped)},
		destroyErr: map[string]error{"sb-1": errors.New("daemon says no")},
	}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-1").SandboxID; got != "sb-1" {
		t.Errorf("SandboxID = %q, want sb-1: the next sweep has to be able to find it", got)
	}
}

// Rule 4. The crash-recovery path, and the one rule that can destroy a running
// agent if it is wrong, so it is tested from both sides in one test.
func TestSweepDestroysOrphansAndOnlyOrphans(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	unmanaged := managed("sb-3", backend.StateRunning)
	unmanaged.Labels = map[string]string{} // somebody else's container
	be := &fakeBackend{sandboxes: []backend.Sandbox{
		managed("sb-1", backend.StateRunning), // has a record
		managed("sb-2", backend.StateRunning), // orphan
		unmanaged,
	}}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("sb-2") {
		t.Error("the orphan survived; a crash between Create and the record write leaks a container forever")
	}
	if be.wasDestroyed("sb-1") || be.wasDestroyed("sb-3") {
		t.Errorf("destroyed something that was not an orphan: %v", be.destroyed)
	}
	if !strings.Contains(logs.String(), "sb-2") || !strings.Contains(logs.String(), "orphan") {
		t.Errorf("the destruction was silent or did not say it was an orphan; logs were:\n%s", logs.String())
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("s-1 = %q, want running", got)
	}
}

// Rule 4's grace, and it is the same hazard ruling 3 names on the other rule.
// POST /v1/sessions writes the record before it creates the container, so its
// containers are never orphans — but POST /v1/sessions/{id}/restart mints a
// FRESH sandbox id, calls backend.Create, and only then writes that id to the
// record (sessions.go:773-793). Between those two the container is live,
// managed, and matched by no record, which is rule 4's definition of an orphan.
// A sweep landing there destroys the sandbox of a session that is restarting
// normally. The window is small and the cost of waiting one sweep is nothing.
func TestSweepSparesAnOrphanYoungEnoughToBeMidCreate(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	fresh := managed("sb-fresh", backend.StateStarting)
	fresh.CreatedAt = clock.Add(-2 * time.Second) // Create returned; the record write has not
	old := managed("sb-old", backend.StateRunning)
	be := &fakeBackend{sandboxes: []backend.Sandbox{fresh, old}}
	r := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		Instance:    ourInstance,
		Interval:    time.Minute,
		CreateGrace: 90 * time.Second,
		Now:         func() time.Time { return clock },
		Log:         slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if be.wasDestroyed("sb-fresh") {
		t.Error("destroyed a sandbox whose record write may still be in flight; a restarting session just lost its agent")
	}
	if !be.wasDestroyed("sb-old") {
		t.Error("the grace became an exemption: an orphan older than the create window was left running")
	}
}

// The task-5 amendment to contract §2.4: List keeps a LAN-published sandbox in
// the slice with NonLoopback set and an empty Endpoint, and the reaper is the
// only actor that resolves it.
func TestSweepDestroysANonLoopbackSandboxAndSaysWhy(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	exposed := managed("sb-1", backend.StateRunning)
	exposed.Endpoint = ""
	exposed.NonLoopback = true
	be := &fakeBackend{sandboxes: []backend.Sandbox{exposed}}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("sb-1") {
		t.Fatal("a sandbox published off 127.0.0.1 survived the sweep")
	}
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited {
		t.Errorf("status = %q, want exited once the sandbox is actually gone", s.Status)
	}
	if !strings.Contains(s.ExitReason, "127.0.0.1") {
		t.Errorf("ExitReason = %q; it must say why, because this is the one disappearance the user did not ask for", s.ExitReason)
	}
	line := logs.String()
	if !strings.Contains(line, "sb-1") || !strings.Contains(line, "127.0.0.1") {
		t.Errorf("the log does not identify the sandbox and the reason:\n%s", line)
	}
	// Ruling 2 asks for the offending host IP by name. Without it an operator
	// learns THAT a sandbox was published off loopback but not WHERE, which is
	// the half that points at the misconfiguration.
	if !strings.Contains(line, offendingHost) {
		t.Errorf("the log does not say where the port was published, so the reap cannot be acted on:\n%s", line)
	}
	// Ruling 2: distinct from an orphan reap and from an idle-TTL reap, or "why
	// did my sandbox vanish" is answered by a line that names the wrong cause.
	if strings.Contains(line, "orphan") || strings.Contains(line, "idle timeout") {
		t.Errorf("the non-loopback destruction is logged as an orphan or an idle reap:\n%s", line)
	}
}

// Contract §9A. This is the shape task 11 actually leaves behind: watchSessionReady
// finds no endpoint, writes NOTHING rather than inventing a status, and the
// record sits at bare `starting` until this sweep. The terminal record this
// writes is the ONLY place the user ever learns the cause, so a grace that
// spared it — or a reason that said "sandbox disappeared" — would leave "my
// session sat at starting and then vanished" with no answer anywhere.
func TestSweepGivesAStartingSessionTheRealReasonItsSandboxWentAway(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusStarting,
		CreatedAt: clock.Add(-2 * time.Second), // well inside any create grace
	})
	exposed := managed("sb-1", backend.StateRunning)
	exposed.Endpoint = ""
	exposed.NonLoopback = true
	be := &fakeBackend{sandboxes: []backend.Sandbox{exposed}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited {
		t.Fatalf("status = %q, want exited: the sandbox was destroyed, so starting is no longer true", s.Status)
	}
	if !strings.Contains(s.ExitReason, "127.0.0.1") {
		t.Errorf("ExitReason = %q, want the real cause: the daemon published the sandbox off loopback", s.ExitReason)
	}
	if strings.Contains(s.ExitReason, "disappeared") {
		t.Errorf("ExitReason = %q names the symptom rather than the cause", s.ExitReason)
	}
}

// A LAN-published sandbox that no session record claims satisfies BOTH the
// non-loopback rule and rule 4's definition of an orphan. It must be destroyed
// once and logged once, as the thing it is: ruling 2 asks for three
// distinguishable destruction messages, and a sandbox that produces two of them
// for one event tells the operator it was collected as litter when it was
// actually removed off the LAN.
func TestSweepDestroysAnUnclaimedNonLoopbackSandboxOnceAndCallsItWhatItIs(t *testing.T) {
	t.Parallel()
	store := newStore(t) // no records at all: nothing claims this sandbox
	exposed := managed("sb-1", backend.StateRunning)
	exposed.Endpoint = ""
	exposed.NonLoopback = true
	be := &fakeBackend{sandboxes: []backend.Sandbox{exposed}}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n := be.destroyCount(); n != 1 {
		t.Errorf("Destroy was called %d times for one sandbox: %v", n, be.destroyed)
	}
	if strings.Contains(logs.String(), "orphan") {
		t.Errorf("a sandbox removed off the LAN was also reported as an orphan:\n%s", logs.String())
	}
}

// The same sandbox as above, on the path where the destroy FAILS. It is still
// unclaimed and still managed, so rule 4 would pick it up as an orphan in the
// same sweep — destroying it a second time and, worse, logging a live security
// defect as routine litter. The test above passes on the success path whether
// or not that is true, because a successful destroy lands in `gone`; this one
// takes the path `gone` does not cover.
func TestSweepDoesNotRelabelAFailedNonLoopbackDestroyAsAnOrphanReap(t *testing.T) {
	t.Parallel()
	store := newStore(t) // no records: the sandbox is unclaimed as well as exposed
	exposed := managed("sb-1", backend.StateRunning)
	exposed.Endpoint = ""
	exposed.NonLoopback = true
	be := &fakeBackend{
		sandboxes:  []backend.Sandbox{exposed},
		destroyErr: map[string]error{"sb-1": errors.New("daemon says no")},
	}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	line := logs.String()
	if strings.Contains(line, "orphan") {
		t.Errorf("a sandbox that is still exposed on the LAN was reported as an orphan:\n%s", line)
	}
	if !strings.Contains(line, "still reachable from the LAN") {
		t.Errorf("the failed destroy did not say what is still wrong:\n%s", line)
	}
	// The address matters MORE here than on the success path, and it was pinned
	// only there. This is the line an operator reads while the sandbox is still
	// exposed and they have to go and find it, so a copy of the attribute that
	// can be deleted without a test noticing is the wrong copy to leave
	// unasserted.
	if !strings.Contains(line, offendingHost) {
		t.Errorf("the failed destroy does not say where the port is still published:\n%s", line)
	}
}

func TestSweepLeavesTheSessionAloneWhenTheNonLoopbackDestroyFails(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	exposed := managed("sb-1", backend.StateRunning)
	exposed.NonLoopback = true
	be := &fakeBackend{
		sandboxes:  []backend.Sandbox{exposed},
		destroyErr: map[string]error{"sb-1": errors.New("daemon says no")},
	}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("status = %q; the process is still running and still on the LAN, so reporting exited would be a fabricated status", got)
	}
}

// Rule 5, both directions, plus decision (d).
func TestSweepDerivesIdleFromSilenceAndBackAgain(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		LastOutputAt: clock.Add(-2 * time.Minute),
	})
	// A session that has never produced a byte is measured from CreatedAt. The
	// zero time is not "silent for a very long while"; it is year 1, and it
	// would make every new session idle on the first sweep after creation.
	seed(t, store, registry.Session{
		ID: "s-2", SandboxID: "sb-2", Status: registry.StatusRunning,
		CreatedAt: clock.Add(-30 * time.Second),
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{
		managed("sb-1", backend.StateRunning),
		managed("sb-2", backend.StateRunning),
	}}
	r, _ := newReaper(t, store, be, 90*time.Second, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("first Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusIdle {
		t.Errorf("s-1 = %q after two minutes of silence, want idle", got)
	}
	if got := get(t, store, "s-2").Status; got != registry.StatusRunning {
		t.Errorf("s-2 = %q thirty seconds after creation, want running", got)
	}
	if be.destroyCount() != 0 {
		t.Errorf("idle derivation destroyed something with IdleTimeout unset: %v", be.destroyed)
	}

	if _, err := store.Update("s-1", func(s *registry.Session) error {
		s.LastOutputAt = clock.Add(-5 * time.Second)
		return nil
	}); err != nil {
		t.Fatalf("recording output: %v", err)
	}
	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("second Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("s-1 = %q after output resumed, want running", got)
	}
}

func TestIdleDerivationIsOffWhenIdleAfterIsZero(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		LastOutputAt: clock.Add(-10 * time.Hour),
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateRunning)}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("status = %q with LUCI_IDLE_AFTER=0, want running", got)
	}
}

// Rule 6. Opt-in, and the only rule that destroys a sandbox the user still has
// a record for, so the reason has to survive into the rail.
func TestIdleTimeoutDestroysTheSandboxAndRecordsWhy(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		LastOutputAt: clock.Add(-30 * time.Minute),
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateRunning)}}
	r, logs := newReaper(t, store, be, 90*time.Second, 10*time.Minute)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("sb-1") {
		t.Fatal("the idle timeout expired and the sandbox is still there")
	}
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited || s.ExitReason != "idle timeout" {
		t.Errorf("s-1 = (%q, %q), want (exited, \"idle timeout\")", s.Status, s.ExitReason)
	}
	// Ruling 2 again, from the other side: an idle reap must not read as either
	// of the other two destructions.
	line := logs.String()
	if !strings.Contains(line, "idle timeout") || strings.Contains(line, "orphan") || strings.Contains(line, "published outside") {
		t.Errorf("the idle-TTL destruction is not distinguishable in the log:\n%s", line)
	}
}
