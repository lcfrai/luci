package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/reaper"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

var bootClock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

// recoveryBackend answers List and nothing else. recoverSessions must never
// create, attach to or destroy anything: it runs before the listener, and a
// boot that destroys a container is a boot that loses work.
type recoveryBackend struct {
	mu        sync.Mutex
	sandboxes []backend.Sandbox
	listErr   error
}

func (f *recoveryBackend) Kind() backend.Kind { return backend.KindDocker }

func (f *recoveryBackend) Create(context.Context, backend.Spec) (backend.Sandbox, error) {
	panic("recovery called Create")
}

func (f *recoveryBackend) Attach(context.Context, string, string) (*websocket.Conn, error) {
	panic("recovery called Attach")
}

func (f *recoveryBackend) Destroy(context.Context, string) error {
	panic("recovery called Destroy")
}

func (f *recoveryBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sandboxes {
		if s.ID == id {
			return s, nil
		}
	}
	return backend.Sandbox{}, backend.ErrNotFound
}

func (f *recoveryBackend) List(context.Context) ([]backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]backend.Sandbox, len(f.sandboxes))
	copy(out, f.sandboxes)
	return out, nil
}

func (f *recoveryBackend) Close() error { return nil }

func bootStore(t *testing.T) *registry.FileStore {
	t.Helper()
	store, err := registry.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}

func bootSeed(t *testing.T, store registry.Store, s registry.Session) {
	t.Helper()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = bootClock.Add(-time.Hour)
	}
	if err := store.Create(s); err != nil {
		t.Fatalf("seed %s: %v", s.ID, err)
	}
}

func bootGet(t *testing.T, store registry.Store, id string) registry.Session {
	t.Helper()
	s, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return s
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// Contract §5.5 step 3. The endpoint on disk is stale the moment Docker Desktop
// restarts, and attaching to a stale endpoint is connection-refused, which
// looks exactly like a dead agent.
func TestRecoverRepublishesTheEndpoint(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		Endpoint: "127.0.0.1:49711", SandboxRef: "container-old", AttachCount: 3,
	})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-1", Kind: backend.KindDocker, Ref: "container-new",
		Endpoint: "127.0.0.1:52002", State: backend.StateRunning,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Endpoint != "127.0.0.1:52002" {
		t.Errorf("Endpoint = %q, want the port the daemon republished on", s.Endpoint)
	}
	if s.SandboxRef != "container-new" {
		t.Errorf("SandboxRef = %q, want container-new", s.SandboxRef)
	}
	if s.Status != registry.StatusRunning {
		t.Errorf("Status = %q, want running", s.Status)
	}
	// AttachCount is the one field nothing else can repair: it is a count of
	// live WebSockets, every one of them died with the previous process, and
	// task 12's relay defer that would have decremented it died with them.
	if s.AttachCount != 0 {
		t.Errorf("AttachCount = %d, want 0: no client is attached to a process that just came back", s.AttachCount)
	}
}

// Contract §5.5 steps 3 and 4.
func TestRecoverSettlesMissingAndStoppedSandboxes(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning, AttachCount: 2})
	bootSeed(t, store, registry.Session{ID: "s-2", SandboxID: "sb-2", Status: registry.StatusIdle, AttachCount: 1})
	bootSeed(t, store, registry.Session{ID: "s-3", Status: registry.StatusStarting})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-2", Kind: backend.KindDocker, Ref: "container-sb-2", State: backend.StateStopped,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s1 := bootGet(t, store, "s-1")
	if s1.Status != registry.StatusExited || s1.ExitReason != "sandbox disappeared while the app was not running" {
		t.Errorf("s-1 = (%q, %q)", s1.Status, s1.ExitReason)
	}
	if s1.AttachCount != 0 {
		t.Errorf("s-1 AttachCount = %d, want 0", s1.AttachCount)
	}
	s2 := bootGet(t, store, "s-2")
	if s2.Status != registry.StatusExited || s2.ExitReason != "process exited" {
		t.Errorf("s-2 = (%q, %q)", s2.Status, s2.ExitReason)
	}
	if s2.AttachCount != 0 {
		t.Errorf("s-2 AttachCount = %d, want 0", s2.AttachCount)
	}
	if got := bootGet(t, store, "s-3").Status; got != registry.StatusStarting {
		t.Errorf("s-3 = %q: a session with no SandboxID has no sandbox to recover", got)
	}
}

// Resume after a restart depends on one field recovery never has a reason to
// touch: the conversation id. Every branch that writes is here — republished,
// disappeared, stopped, and an exited record whose stale attach count is
// repaired — and each is read back from a REOPENED file, because that is what
// the next process sees.
func TestRecoverKeepsEveryConversationID(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := registry.NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	seeds := map[string]registry.Session{
		"s-1": {ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning, Endpoint: "127.0.0.1:49711"},
		"s-2": {ID: "s-2", SandboxID: "sb-2", Status: registry.StatusRunning},
		"s-3": {ID: "s-3", SandboxID: "sb-3", Status: registry.StatusIdle},
		"s-4": {ID: "s-4", SandboxID: "sb-4", Status: registry.StatusExited, AttachCount: 2},
	}
	conversations := map[string]string{
		"s-1": "11111111-1111-4111-8111-111111111111",
		"s-2": "22222222-2222-4222-8222-222222222222",
		"s-3": "33333333-3333-4333-8333-333333333333",
		"s-4": "44444444-4444-4444-8444-444444444444",
	}
	// And the conversations a restart left, which a resume falls back to: a
	// recovery write that dropped them would orphan every one.
	earlier := func(id string) []string { return []string{"eeeeeeee-0000-4000-8000-00000000000" + id[2:]} }
	for id, s := range seeds {
		s.ConversationID = conversations[id]
		s.EarlierConversationIDs = earlier(id)
		bootSeed(t, store, s)
	}
	be := &recoveryBackend{sandboxes: []backend.Sandbox{
		{ID: "sb-1", Kind: backend.KindDocker, Endpoint: "127.0.0.1:52002", State: backend.StateRunning},
		{ID: "sb-3", Kind: backend.KindDocker, State: backend.StateStopped},
	}}

	recoverSessions(context.Background(), store, be, quietLogger())

	reopened, err := registry.NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	for id, want := range conversations {
		got := bootGet(t, reopened, id)
		if got.ConversationID != want {
			t.Errorf("%s conversationId = %q after recovery and a reopen, want %q", id, got.ConversationID, want)
		}
		if !slices.Equal(got.EarlierConversationIDs, earlier(id)) {
			t.Errorf("%s earlierConversationIds = %v after recovery and a reopen, want %v", id, got.EarlierConversationIDs, earlier(id))
		}
	}
}

// The one row this task adds to contract §5.5 step 3, which names only
// StateRunning and StateStopped. A sandbox that is still starting is neither:
// the control plane went down between Create and the first /healthz answer.
func TestRecoverReportsAStillStartingSandboxAsStarting(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning, Endpoint: "127.0.0.1:49711",
	})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-1", Kind: backend.KindDocker, Ref: "container-sb-1",
		Endpoint: "127.0.0.1:52002", State: backend.StateStarting,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Status != registry.StatusStarting {
		t.Errorf("Status = %q, want starting: running would fabricate a status for a sandbox that is not answering yet, and exited is terminal", s.Status)
	}
	if s.ExitReason != "" {
		t.Errorf("ExitReason = %q; the session has not exited", s.ExitReason)
	}
}

// Contract §5.5 step 2, and the failure mode it names: marking everything
// exited because Docker Desktop had not finished starting loses a night's work.
func TestRecoverChangesNothingWhenTheBackendIsDown(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning, Endpoint: "127.0.0.1:49711",
	})
	be := &recoveryBackend{listErr: errors.New("docker daemon is still starting")}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Status != registry.StatusRunning || s.Endpoint != "127.0.0.1:49711" {
		t.Errorf("s-1 = (%q, %q), want it untouched so the reaper can converge later", s.Status, s.Endpoint)
	}
}

// Ruling 1 splits PANACEA_APP_KUBECONFIG in two: under docker it warns, under
// kata it is REFUSED, because honouring it needs a module this phase does not
// add and authenticating with the in-cluster service account instead would tell
// an operator something false about their own cluster credentials.
//
// The warning half is tested six ways in config_test.go, and that file also
// asserts SILENCE under kata — an assertion that is only the right answer
// because something louder happens here. Without this test, deleting the
// refusal leaves the variable silently ignored on the one backend that reads it
// with the whole suite still green, and the test that looks like it covers the
// case actively certifies the wrong behaviour.
//
// The empty-string path is deliberately not asserted on: it calls
// rest.InClusterConfig(), which answers differently off-cluster and in one.
func TestKubeconfigIsRefusedRatherThanIgnored(t *testing.T) {
	t.Parallel()
	_, err := kubeClient("/home/dev/.kube/config")
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("kubeClient with a kubeconfig = %v, want backend.ErrUnsupported", err)
	}
	if !strings.Contains(err.Error(), envKubeconfig) {
		t.Errorf("the refusal %q does not name the variable the operator set", err)
	}
}

// Contract §5.6: there is no transition out of exited. Decision (e): recovery
// does not destroy the lingering sandbox either, it says so and moves on.
//
// AttachCount is the exception, and it is not a status: both already-exited
// branches leave every other field alone and still repair the count, because
// nothing else in the phase can. A session that was exited with a client
// attached when the process died would otherwise show that client as connected
// for ever.
func TestRecoverNeverResurrectsAnExitedSession(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusExited, ExitReason: "process exited",
		AttachCount: 2,
	})
	// s-2 is the other exited branch: its sandbox is gone as well as its process.
	bootSeed(t, store, registry.Session{
		ID: "s-2", SandboxID: "sb-gone", Status: registry.StatusExited, ExitReason: "idle timeout",
		AttachCount: 1,
	})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-1", Kind: backend.KindDocker, Ref: "container-sb-1",
		Endpoint: "127.0.0.1:52002", State: backend.StateRunning,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Status != registry.StatusExited || s.ExitReason != "process exited" {
		t.Errorf("s-1 = (%q, %q), want the terminal state it already had", s.Status, s.ExitReason)
	}
	if s.AttachCount != 0 {
		t.Errorf("s-1 AttachCount = %d, want 0: the sockets it counts died with the previous process", s.AttachCount)
	}
	s2 := bootGet(t, store, "s-2")
	if s2.Status != registry.StatusExited || s2.ExitReason != "idle timeout" {
		t.Errorf("s-2 = (%q, %q), want the terminal state it already had", s2.Status, s2.ExitReason)
	}
	if s2.AttachCount != 0 {
		t.Errorf("s-2 AttachCount = %d, want 0", s2.AttachCount)
	}
}

// A LAN-published sandbox reports NonLoopback with an empty Endpoint (task 5).
// Recovery must overwrite the stored endpoint with that empty string rather
// than keeping the old one, and must NOT invent a status: the reaper destroys
// it on its first sweep, which happens immediately.
func TestRecoverClearsTheEndpointOfANonLoopbackSandbox(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning, Endpoint: "127.0.0.1:49711",
	})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-1", Kind: backend.KindDocker, Ref: "container-sb-1",
		Endpoint: "", NonLoopback: true, State: backend.StateRunning,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Endpoint != "" {
		t.Errorf("Endpoint = %q, want empty: nothing may attach to a sandbox on the LAN", s.Endpoint)
	}
	if s.Status != registry.StatusRunning {
		t.Errorf("Status = %q; the process is still running and the reaper owns the destruction", s.Status)
	}
}

// The grace the reaper actually runs with has to be longer than one ready
// timeout, because PANACEA_APP_READY_TIMEOUT budgets only the wait for
// /healthz while the window a mid-create session must survive is the whole of
// backend.Create. reaper.CreateGraceFor owns that relationship; this asserts
// the wiring goes through it, since `CreateGrace: cfg.ReadyTimeout` would
// compile and quietly halve the protection.
func TestTheReaperIsWiredWithMoreGraceThanOneReadyTimeout(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_READY_TIMEOUT": "90s"}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	rc := reaperConfigFor(cfg, nil, nil, nil)
	if rc.CreateGrace <= cfg.ReadyTimeout {
		t.Errorf("CreateGrace = %v with a %v ready timeout; it must outlast ContainerCreate as well as the ready wait",
			rc.CreateGrace, cfg.ReadyTimeout)
	}
	if want := reaper.CreateGraceFor(cfg.ReadyTimeout); rc.CreateGrace != want {
		t.Errorf("CreateGrace = %v, want %v: the derivation belongs to reaper.CreateGraceFor, not to this call site", rc.CreateGrace, want)
	}
	// The rest of the mapping, so a reordered struct literal cannot silently
	// swap two durations of the same type.
	for _, c := range []struct {
		name      string
		got, want time.Duration
	}{
		{"IdleAfter", rc.IdleAfter, cfg.IdleAfter},
		{"IdleTimeout", rc.IdleTimeout, cfg.IdleTimeout},
		{"Interval", rc.Interval, cfg.ReapInterval},
	} {
		if c.got != c.want {
			t.Errorf("reaper.Config.%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// Hermetic like every run() test (runHermetic): with the loopback rule broken,
// this would otherwise be a real control plane on every interface, on the
// default state directory and the real Docker daemon, whose startup sweep
// destroys every managed container it does not own. Port 0, so even that
// control plane would not hold 8730.
func TestRunRefusesANonLoopbackBindWithExitCode2(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	code := run(env(runHermetic(t, map[string]string{envAddr: "0.0.0.0:0"})), &out)
	if code != 2 {
		t.Errorf("run returned %d, want 2", code)
	}
	if !strings.Contains(out.String(), "a sandbox attach socket on a non-loopback address is a shell on the LAN") {
		t.Errorf("run printed %q, which does not explain the refusal", out.String())
	}
}
