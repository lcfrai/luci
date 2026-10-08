package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type pendingCreateBackend struct {
	backend.Backend
	entered chan backend.Spec
	proceed chan struct{}
	mu      sync.Mutex
	active  map[string]bool
}

func (b *pendingCreateBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	b.entered <- spec
	select {
	case <-b.proceed:
	case <-ctx.Done():
		return backend.Sandbox{}, ctx.Err()
	}
	sb, err := b.Backend.Create(ctx, spec)
	if err == nil {
		b.mu.Lock()
		b.active[spec.ID] = true
		b.mu.Unlock()
	}
	return sb, err
}
func (b *pendingCreateBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	b.mu.Lock()
	active := b.active[id]
	b.mu.Unlock()
	if !active {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	return backend.Sandbox{ID: id, State: backend.StateRunning}, nil
}
func (b *pendingCreateBackend) Destroy(ctx context.Context, id string) error {
	b.mu.Lock()
	active := b.active[id]
	delete(b.active, id)
	b.mu.Unlock()
	if !active {
		return backend.ErrNotFound
	}
	return b.Backend.Destroy(ctx, id)
}

// Two actual HTTP creates and a concurrent HTTP stop share one durable host
// budget. The controlled backend holds its first Create before any child exists;
// it must not permit an early Stop/NotFound to free the pending slot.
func TestExecutionPendingCreateRefusesConcurrentStopAndSecondAllocation(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	a := f.env.api
	var logs bytes.Buffer
	a.deps.Log = slog.New(slog.NewTextHandler(&logs, nil))
	t.Cleanup(func() {
		if t.Failed() {
			t.Log(logs.String())
		}
	})
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	var settings struct {
		Revision int `json:"revision"`
	}
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	policy := executionFixturePolicy()
	policy.MaxActive, policy.MaxPerProject = 1, 1
	body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": policy, "dispatchPaused": false})
	if w := f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	b := &pendingCreateBackend{Backend: f.env.be, entered: make(chan backend.Spec, 2), proceed: make(chan struct{}), active: map[string]bool{}}
	a.deps.Backend = b
	var once sync.Once
	release := func() { once.Do(func() { close(b.proceed) }) }
	t.Cleanup(release)
	empty := ""
	req := sessionCreateRequest{WorktreeID: project.WorktreeID(f.owner.Path), Harness: "claude-code", PermissionMode: "ask", AgentConnectionID: &empty, OpeningPrompt: "Controlled pending-create fixture"}
	body, _ = json.Marshal(req)
	responses := make(chan *httptest.ResponseRecorder, 1)
	go func() { responses <- f.env.do(t, "POST", "/v1/sessions", string(body)) }()
	var first backend.Spec
	select {
	case first = <-b.entered:
	case response := <-responses:
		t.Fatal("first create did not reach backend", response.Code, response.Body.String())
	case <-time.After(10 * time.Second):
		t.Fatal("first create timed out")
	}
	if w := f.env.do(t, "DELETE", "/v1/sessions/"+first.SessionID, ""); w.Code != 409 {
		t.Fatal("stop changed an in-flight create", w.Code, w.Body.String())
	}
	row, err := f.store.Get(first.SessionID)
	if err != nil || row.Status != registry.StatusStarting || f.env.be.destroyCount() != 0 {
		t.Fatal("pending child was stopped", row, err)
	}
	// The second request traverses real route resolution and admission, but
	// cannot allocate despite a fresh complete inventory with no containers.
	if w := f.env.do(t, "POST", "/v1/sessions", string(body)); w.Code != 409 {
		t.Fatal("second create bypassed pending capacity", w.Code, w.Body.String())
	}
	select {
	case extra := <-b.entered:
		t.Fatal("another create reached backend", extra.ID)
	default:
	}
	release()
	select {
	case w := <-responses:
		sessionsDecodeOne(t, w, 201)
	case <-time.After(10 * time.Second):
		t.Fatal("acknowledged create did not complete")
	}
	sessionsAwaitWatch(t, done)
	if w := f.env.do(t, "DELETE", "/v1/sessions/"+first.SessionID, ""); w.Code != 200 {
		t.Fatal("confirmed child could not stop", w.Code, w.Body.String())
	}
	w = f.env.do(t, "POST", "/v1/sessions", string(body))
	sessionsDecodeOne(t, w, 201)
	sessionsAwaitWatch(t, done)
	if controlledAllocationCount(f.env.be) != 2 {
		t.Fatal("normal settled stop did not release exactly one slot")
	}
}

func TestExecutionUnsettledLaunchSurvivesReopenAndBlocksAbsenceReleaseAndGit(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-owner"}
	policy := executionFixturePolicy()
	policy.MaxActive = 1
	if d, e := g.reserve(context.Background(), "q-pending", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, policy); e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	if e := g.bind(context.Background(), "q-pending", "s-pending"); e != nil {
		t.Fatal(e)
	}
	row := sessionsSeed(t, g.api.deps.Store, registry.Session{ID: "s-pending", ProjectID: actor.ProjectID, WorktreeID: "w-pending", SandboxID: "s-pending", Status: registry.StatusStarting})
	if e := g.markBackendStarting(context.Background(), "q-pending", "s-pending", "s-pending"); e != nil {
		t.Fatal(e)
	}
	g.api.markSessionExited(row.ID, "the sandbox could not be created")
	row, _ = g.api.deps.Store.Get(row.ID)
	g.api.deps.Backend = &executionGoneBackend{Backend: g.api.deps.Backend}
	// Reconstruct the admission service from only its existing private files;
	// there is no in-memory active-call flag to rescue this interrupted launch.
	g = newExecutionAdmission(g.api)
	g.api.execution = g
	if e := g.release(context.Background(), "q-pending"); !errors.Is(e, ErrExecutionLeaseHeld) {
		t.Fatal("absence released unacknowledged launch", e)
	}
	before, e := os.ReadFile(g.path)
	if e != nil {
		t.Fatal(e)
	}
	if e := g.confirmNoPendingLaunch(context.Background(), row); e == nil {
		t.Fatal("Git observation ignored unacknowledged launch")
	}
	after, e := os.ReadFile(g.path)
	if e != nil || string(before) != string(after) {
		t.Fatal("read-only Git observation rewrote private state", e)
	}
	if d, e := g.reserve(context.Background(), "q-next", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, policy); e != nil || d.State == "admitted" {
		t.Fatal("fresh empty inventory discarded interrupted launch", d, e)
	}
	if busy, e := g.api.observeSwarmWorktree(context.Background(), actor.ProjectID, row.WorktreeID); e == nil && !busy {
		t.Fatal("uncertain worker checkout became integratable")
	}
	// Only the trusted allocator's full successful return may acknowledge the
	// exact claim; an unrelated response must not settle it.
	if e := g.markBackendSettled(context.Background(), "q-pending", row.ID, "s-foreign"); e == nil {
		t.Fatal("foreign backend response settled launch")
	}
	if e := g.markBackendSettled(context.Background(), "q-pending", row.ID, row.SandboxID); e != nil {
		t.Fatal(e)
	}
	if e := g.release(context.Background(), "q-pending"); e != nil {
		t.Fatal("acknowledged absent child retained capacity", e)
	}
}

type acknowledgedCreateBackend struct {
	backend.Backend
	returnedID string
}

func (b acknowledgedCreateBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	sb, err := b.Backend.Create(ctx, spec)
	if err == nil && b.returnedID != "" {
		sb.ID = b.returnedID
	}
	return sb, err
}

func TestExecutionCreateAcknowledgmentFailureCleansOnlyExpectedChildAndRetainsClaim(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "journal failure", true: "foreign returned identity"}[foreign], func(t *testing.T) {
			g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-owner"}
			if d, e := g.reserve(context.Background(), "q", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); e != nil || d.State != "admitted" {
				t.Fatal(d, e)
			}
			if e := g.bind(context.Background(), "q", "s-owned"); e != nil {
				t.Fatal(e)
			}
			base := g.api.deps.Backend.(*sessionsBackend)
			returnedID := ""
			if foreign {
				returnedID = "s-foreign"
			}
			g.api.deps.Backend = acknowledgedCreateBackend{Backend: base, returnedID: returnedID}
			called := false
			_, err := g.api.allocatePreparedSession(context.Background(), preparedSessionAllocation{
				Record:             registry.Session{ID: "s-owned", SandboxID: "s-owned", ProjectID: actor.ProjectID, Harness: "claude-code", Status: registry.StatusStarting},
				Spec:               backend.Spec{ID: "s-owned", SessionID: "s-owned"},
				BeforeBackendStart: func(ctx context.Context) error { return g.markBackendStarting(ctx, "q", "s-owned", "s-owned") },
				AfterBackendCreate: func(context.Context, string) error {
					called = true
					return errors.New("controlled acknowledgment journal unavailable")
				},
			})
			if err == nil || called == foreign {
				t.Fatal("unconfirmed response was acknowledged", called, err)
			}
			if base.destroyCount() != 1 || base.destroys[0] != "s-owned" {
				t.Fatal("cleanup escaped exact allocated identity", base.destroys)
			}
			row, e := g.api.deps.Store.Get("s-owned")
			if e != nil || row.Status != registry.StatusExited || row.ExitReason != "execution launch acknowledgment could not be recorded" {
				t.Fatal("failed receipt stranded live starting row", row, e)
			}
			g.api.deps.Backend = &executionGoneBackend{Backend: base}
			if e := g.release(context.Background(), "q"); !errors.Is(e, ErrExecutionLeaseHeld) {
				t.Fatal("cleanup guessed pending claim settled", e)
			}
		})
	}
}

func TestExecutionTerminatedPreBackendRowCannotLaunchWithRetainedLease(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-owner"}
	if d, e := g.reserve(context.Background(), "q", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	if e := g.bind(context.Background(), "q", "s-owned"); e != nil {
		t.Fatal(e)
	}
	sessionsSeed(t, g.api.deps.Store, registry.Session{ID: "s-owned", SandboxID: "s-owned", ProjectID: actor.ProjectID, Status: registry.StatusExited})
	if e := g.markBackendStarting(context.Background(), "q", "s-owned", "s-owned"); e == nil {
		t.Fatal("terminated child crossed the backend launch edge")
	}
}
