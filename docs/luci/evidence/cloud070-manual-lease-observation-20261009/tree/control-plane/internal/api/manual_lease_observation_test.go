package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestManualLeaseObserverRetainsPresentUnknownAndUnowned(t *testing.T) {
	for _, scenario := range []string{"stopped", "removing", "running", "gone", "foreign object", "unavailable", "joined absent", "cancelled", "foreign scope", "missing row", "wrong project", "wrong sandbox", "wrong backend", "missing ref", "task row", "partial task row", "legacy guard", "reserved", "allocating", "replacing", "backend-dispatching", "duplicate", "corrupt ledger", "unsupported backend"} {
		t.Run(scenario, func(t *testing.T) {
			env, row, _ := manualLeaseFixture(t)
			a := env.api
			var reads atomic.Int64
			backendError := error(backend.ErrNotFound)
			sandbox := backend.Sandbox{}
			changeRow := func(fn func(*registry.Session)) {
				_, err := a.deps.Store.Update(row.ID, func(current *registry.Session) error { fn(current); return nil })
				if err != nil {
					t.Fatal(err)
				}
			}
			changeLease := func(fn func(*executionLeaseLedger)) {
				if err := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error { fn(l); return nil }); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "stopped", "removing", "running", "gone", "foreign object":
				backendError = nil
				sandbox = backend.Sandbox{ID: row.SandboxID, Ref: row.SandboxRef, State: backend.StateStopped}
				if scenario == "removing" {
					sandbox.State = backend.State("removing")
				}
				if scenario == "running" {
					sandbox.State = backend.StateRunning
				}
				if scenario == "gone" {
					sandbox.State = backend.StateGone
				}
				if scenario == "foreign object" {
					sandbox.ID, sandbox.Ref, sandbox.Instance = "s-other", "other-ref", "foreign"
				}
			case "unavailable":
				backendError = backend.ErrUnavailable
			case "joined absent":
				backendError = errors.Join(backend.ErrNotFound, backend.ErrUnavailable)
			case "cancelled":
				backendError = context.Canceled
			case "foreign scope":
				changeLease(func(l *executionLeaseLedger) { l.Leases[0].Scope = strings.Repeat("b", 64) })
			case "missing row":
				if err := a.deps.Store.Delete(row.ID); err != nil {
					t.Fatal(err)
				}
			case "wrong project":
				changeRow(func(s *registry.Session) { s.ProjectID = "p-other" })
			case "wrong sandbox":
				changeRow(func(s *registry.Session) { s.SandboxID = "s-other" })
			case "wrong backend":
				changeRow(func(s *registry.Session) { s.Backend = "kubernetes" })
			case "missing ref":
				changeRow(func(s *registry.Session) { s.SandboxRef = "" })
			case "task row":
				changeRow(func(s *registry.Session) { s.ProjectRunID = "run-other"; s.ProjectRunOperationID = "op-other" })
			case "partial task row":
				changeRow(func(s *registry.Session) { s.ProjectRunID = "run-other" })
			case "legacy guard":
				changeLease(func(l *executionLeaseLedger) { l.Leases[0].LaunchGuard = 0 })
			case "reserved":
				changeLease(func(l *executionLeaseLedger) {
					l.Leases[0].State = "reserved"
					l.Leases[0].SessionID = ""
					l.Leases[0].SandboxID = ""
				})
			case "allocating", "replacing", "backend-dispatching":
				changeLease(func(l *executionLeaseLedger) { l.Leases[0].State = scenario })
			case "duplicate":
				changeLease(func(l *executionLeaseLedger) {
					duplicate := l.Leases[0]
					duplicate.ID = "manual-duplicate"
					l.Leases = append(l.Leases, duplicate)
				})
			case "corrupt ledger":
				if err := writeExecutionLedger(a.execution.path, []byte(`{"version":1,"leases":[],"unrecognized":true}`)); err != nil {
					t.Fatal(err)
				}
			}
			a.deps.Backend = manualAbsenceBackend{Backend: env.be, get: func(context.Context, string) (backend.Sandbox, error) { reads.Add(1); return sandbox, backendError }}
			if scenario == "unsupported backend" {
				a.deps.Backend = swarmObserverBackend{Backend: env.be, get: func(context.Context, string) (backend.Sandbox, error) {
					reads.Add(1)
					return backend.Sandbox{}, backend.ErrNotFound
				}}
			}
			before := manualLeaseBytes(t, a)
			info, _ := os.Stat(a.execution.path)
			_ = a.ObserveManualLeasesOnce(context.Background())
			after, _ := os.Stat(a.execution.path)
			if !bytes.Equal(before, manualLeaseBytes(t, a)) || !os.SameFile(info, after) {
				t.Fatal("uncertain observation rewrote or released the reservation")
			}
			if (scenario == "corrupt ledger" || scenario == "unsupported backend") && reads.Load() != 0 {
				t.Fatal("corrupt global ledger allowed a backend lookup")
			}
		})
	}
}

func TestManualLeaseObserverRejectsGenerationAndReceiptChangesDuringAbsence(t *testing.T) {
	for _, change := range []string{"row generation", "row scope", "lease generation", "new replacement", "global corruption"} {
		t.Run(change, func(t *testing.T) {
			env, row, _ := manualLeaseFixture(t)
			a := env.api
			a.deps.Backend = manualAbsenceBackend{Backend: env.be, get: func(context.Context, string) (backend.Sandbox, error) {
				switch change {
				case "row generation", "row scope":
					_, err := a.deps.Store.Update(row.ID, func(s *registry.Session) error {
						if change == "row generation" {
							s.SandboxRef = "new-generation-ref"
						} else {
							s.WorktreeID = "w-new-scope"
						}
						return nil
					})
					if err != nil {
						t.Error(err)
					}
				case "lease generation", "new replacement":
					err := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
						if change == "lease generation" {
							l.Leases[0].UpdatedAt = l.Leases[0].UpdatedAt.Add(time.Second)
						} else {
							v := l.Leases[0]
							v.ID = "restart-next"
							v.State = "replacing"
							l.Leases = append(l.Leases, v)
						}
						return nil
					})
					if err != nil {
						t.Error(err)
					}
				case "global corruption":
					if err := writeExecutionLedger(a.execution.path, []byte(`{"broken":true}`)); err != nil {
						t.Error(err)
					}
				}
				return backend.Sandbox{}, backend.ErrNotFound
			}}
			_ = a.ObserveManualLeasesOnce(context.Background())
			if change == "global corruption" {
				if string(manualLeaseBytes(t, a)) != `{"broken":true}` {
					t.Fatal("corrupt ledger rewritten")
				}
				return
			}
			l, err := a.execution.readFenceLedger()
			if err != nil || len(l.Leases) == 0 {
				t.Fatal("stale absence dropped current generation", err)
			}
			if change == "new replacement" && len(l.Leases) != 2 {
				t.Fatal("replacement reservation was changed")
			}
		})
	}
}

func TestManualLeaseObserverFairBoundedPagesAndSlowPeer(t *testing.T) {
	env, row, lease := manualLeaseFixture(t)
	a := env.api
	// A full first page is foreign. A slow first owned allocation must then
	// leave its unattempted peer queued for the next turn.
	if err := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
		l.Leases = nil
		for i := 0; i < 128; i++ {
			v := lease
			v.ID = fmt.Sprintf("foreign-%d", i)
			v.Scope = strings.Repeat("c", 64)
			v.SessionID = fmt.Sprintf("s-foreign-%d", i)
			v.SandboxID = v.SessionID
			l.Leases = append(l.Leases, v)
		}
		l.Leases = append(l.Leases, lease)
		v := lease
		v.ID = "manual-peer"
		v.SessionID = "s-peer"
		v.SandboxID = v.SessionID
		l.Leases = append(l.Leases, v)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	peer := row
	peer.ID, peer.SandboxID, peer.SandboxRef = "s-peer", "s-peer", "peer-ref"
	sessionsSeed(t, a.deps.Store, peer)
	var reads atomic.Int64
	a.deps.Backend = manualAbsenceBackend{Backend: env.be, get: func(ctx context.Context, id string) (backend.Sandbox, error) {
		reads.Add(1)
		if id == row.SandboxID {
			<-ctx.Done()
			return backend.Sandbox{}, ctx.Err()
		}
		return backend.Sandbox{}, backend.ErrNotFound
	}}
	if err := a.ObserveManualLeasesOnce(context.Background()); err != nil || reads.Load() != 0 {
		t.Fatal("observer exceeded first scan page", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := a.ObserveManualLeasesOnce(ctx); err == nil {
		t.Fatal("slow observation had no deadline")
	}
	if err := a.ObserveManualLeasesOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	l, err := a.execution.readFenceLedger()
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Leases) != 129 || reads.Load() != 2 {
		t.Fatal("slow peer or page boundary starved the healthy allocation", len(l.Leases), reads.Load())
	}
	for _, v := range l.Leases {
		if v.ID == "manual-peer" {
			t.Fatal("healthy peer retained")
		}
	}
}

func TestManualLeaseObserverCancelsWithoutWaitingForLocalOrSharedLedger(t *testing.T) {
	for _, which := range []string{"local mutex", "shared lock", "child lock", "parent cancelled"} {
		t.Run(which, func(t *testing.T) {
			env, row, _ := manualLeaseFixture(t)
			a := env.api
			a.deps.Backend = manualAbsenceBackend{Backend: env.be, get: func(context.Context, string) (backend.Sandbox, error) { return backend.Sandbox{}, backend.ErrNotFound }}
			before := manualLeaseBytes(t, a)
			switch which {
			case "local mutex":
				a.execution.mu.Lock()
				defer a.execution.mu.Unlock()
			case "shared lock":
				file, err := os.OpenFile(filepath.Join(filepath.Dir(a.execution.path), "leases.lock"), os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				unlock, err := lockExecutionLedger(context.Background(), file)
				if err != nil {
					t.Fatal(err)
				}
				defer unlock()
			case "child lock":
				unlock, err := a.execution.lockChildLifecycle(context.Background(), row.ID)
				if err != nil {
					t.Fatal(err)
				}
				defer unlock()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			if which == "parent cancelled" {
				cancel()
			}
			done := make(chan struct{})
			go func() { _ = a.ObserveManualLeasesOnce(ctx); close(done) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("observer exceeded cancellation budget")
			}
			if !bytes.Equal(before, manualLeaseBytes(t, a)) {
				t.Fatal("cancelled or contended turn wrote capacity")
			}
		})
	}
}

func TestManualLeaseObserverAndManagedRestartShareLifecycleFence(t *testing.T) {
	env, row, _ := manualLeaseFixture(t)
	a := env.api
	// The durable lock is independently held, as by another controller. The
	// actual restart route must refuse before admission, Destroy, or Create.
	unlock, err := a.execution.lockChildLifecycle(context.Background(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	before := manualLeaseBytes(t, a)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest("POST", sessionsTarget("/v1/sessions/"+row.ID+"/restart"), nil).WithContext(ctx)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "current allocation is being observed or changed") {
		t.Fatal("managed restart did not share observation fence", w.Code)
	}
	if !bytes.Equal(before, manualLeaseBytes(t, a)) || len(env.be.specs) != 0 || len(env.be.destroys) != 0 {
		t.Fatal("fenced restart changed allocation or capacity")
	}
}

func TestManualLeaseObserverHoldsRestartGuardDuringBackendObservation(t *testing.T) {
	env, row, _ := manualLeaseFixture(t)
	a := env.api
	entered, proceed := make(chan struct{}), make(chan struct{})
	a.deps.Backend = manualAbsenceBackend{Backend: env.be, get: func(ctx context.Context, _ string) (backend.Sandbox, error) {
		close(entered)
		select {
		case <-proceed:
			return backend.Sandbox{}, backend.ErrNotFound
		case <-ctx.Done():
			return backend.Sandbox{}, ctx.Err()
		}
	}}
	done := make(chan error, 1)
	go func() { done <- a.ObserveManualLeasesOnce(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("observer did not reach backend")
	}
	w := env.do(t, "POST", "/v1/sessions/"+row.ID+"/restart", "")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "already starting or restarting") {
		t.Error("restart entered an active absence observation", w.Code)
	}
	close(proceed)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	l, err := a.execution.readFenceLedger()
	if err != nil || len(l.Leases) != 0 {
		t.Fatal("original absent generation not retired", err)
	}
}

type manualTaskPageProbe struct {
	registry.Store
	visited chan struct{}
}

func (p manualTaskPageProbe) TaskObservationPage(int, int, int) ([]registry.TaskObservationCandidate, int, error) {
	close(p.visited)
	return nil, 0, nil
}

func TestManualLeaseObservationJoinsCancellationWithoutBlockingTaskObserver(t *testing.T) {
	env, _, _ := manualLeaseFixture(t)
	a := env.api
	taskVisited, backendEntered, backendDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	a.deps.Store = manualTaskPageProbe{Store: a.deps.Store, visited: taskVisited}
	a.deps.Backend = manualAbsenceBackend{Backend: env.be, get: func(ctx context.Context, _ string) (backend.Sandbox, error) {
		if !a.execution.mu.TryLock() {
			t.Error("backend lookup held global ledger mutex")
		} else {
			a.execution.mu.Unlock()
		}
		close(backendEntered)
		<-ctx.Done()
		close(backendDone)
		return backend.Sandbox{}, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.observeExecutionCompletions(ctx) }()
	select {
	case <-backendEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("manual observer did not start")
	}
	select {
	case <-taskVisited:
	case <-time.After(2 * time.Second):
		t.Fatal("manual lookup prevented task observation")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("parent cancellation lost", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fanout did not join parent cancellation")
	}
	select {
	case <-backendDone:
	default:
		t.Fatal("fanout returned before backend observation settled")
	}
}

func TestManualLeaseManagedRestartInitializesAbsentPrivateDirectory(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	f.env.api.deps.ReadyTimeout = 5 * time.Second
	sessionsStubReady(t, nil)
	watch := sessionsWatchDone(t)
	body, _ := json.Marshal(map[string]any{"worktreeId": project.WorktreeID(f.owner.Path), "harness": "shell"})
	created := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, watch)
	a := f.env.api
	a.deps.ExecutionStateDir = filepath.Join(t.TempDir(), "new-capacity")
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.execution = newExecutionAdmission(a)
	if _, err := os.Stat(filepath.Dir(a.execution.path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("fixture directory already exists")
	}
	watch = sessionsWatchDone(t)
	response := f.env.do(t, "POST", "/v1/sessions/"+created.ID+"/restart", "")
	if response.Code != 200 {
		t.Fatal("managed restart with no earlier ledger directory was refused", response.Code)
	}
	sessionsAwaitWatch(t, watch)
	row, err := f.store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].SandboxID != row.SandboxID || ledger.Leases[0].State != "backend-settled" {
		t.Fatal("new restart did not preserve exact settled replacement charge", err)
	}
}
