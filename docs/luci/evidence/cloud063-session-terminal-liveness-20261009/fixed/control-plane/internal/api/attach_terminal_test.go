package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func terminalFixtureSession(s *registry.Session) {
	s.ProjectID, s.WorktreeID, s.BranchID = "project-terminal", "worktree-terminal", "branch-terminal"
}

func TestAttachTypedExitPublishesCurrentRoutesWithoutReaper(t *testing.T) {
	f := attachNewFixture(t, nil, terminalFixtureSession)
	// Serve the production router, not just the test attach registration. No
	// reaper runs in this fixture: the former implementation never converges.
	srv := httptest.NewServer(f.api)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/sessions/"+f.id+"/attach", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	f.sandboxd.push([]byte("\x03private terminal detail must not become REST metadata"))
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	attachWaitFor(t, 5*time.Second, func() bool {
		s, err := f.store.Get(f.id)
		return err == nil && s.Status == registry.StatusExited
	}, "typed Exit did not publish terminal status without a background sweep")
	for _, path := range []string{"/v1/sessions/" + f.id, "/v1/sessions"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatal("terminal read failed", res.StatusCode, err)
		}
		var row sessionJSON
		if path == "/v1/sessions" {
			var value sessionListResponse
			if err := json.Unmarshal(body, &value); err != nil || len(value.Sessions) != 1 {
				t.Fatal("invalid terminal list", err)
			}
			row = value.Sessions[0]
		} else if err := json.Unmarshal(body, &row); err != nil {
			t.Fatal(err)
		}
		if row.Status != string(registry.StatusExited) || row.ExitReason != "process exited" || strings.Contains(string(body), "private terminal detail") {
			t.Fatal("terminal REST projection did not preserve static process-only semantics")
		}
	}
	res, err := http.Get(srv.URL + "/v1/sessions/" + f.id + "/attach")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusConflict || f.be.destroyCount() != 0 {
		t.Fatal("ended attach was not refused or destroyed allocation", res.StatusCode)
	}
	f.sandboxd.close <- websocket.StatusNormalClosure
	_, _, err = c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatal("terminal transport lost normal close", err)
	}
}

func TestAttachLateTypedExitCannotTerminateReplacement(t *testing.T) {
	f := attachNewFixture(t, nil, terminalFixtureSession)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := f.dial(ctx, t)
	defer c.CloseNow()
	// A completed restart publishes a new private allocation under the same ID.
	// Keep the old socket alive and deliver its delayed terminal event afterward.
	_, err := f.store.Update(f.id, func(s *registry.Session) error {
		s.SandboxID, s.SandboxRef, s.AttachToken = "new-sandbox", "new-reference", "new-token"
		s.Endpoint, s.Status = "127.0.0.1:12345", registry.StatusRunning
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.sandboxd.push([]byte("\x03exit status 0"))
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	f.sandboxd.close <- websocket.StatusNormalClosure
	_, _, err = c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatal(err)
	}
	attachWaitFor(t, 5*time.Second, func() bool { s, err := f.store.Get(f.id); return err == nil && s.AttachCount == 0 }, "old relay did not settle")
	row, err := f.store.Get(f.id)
	if err != nil || row.Status != registry.StatusRunning || row.SandboxID != "new-sandbox" || row.SandboxRef != "new-reference" || row.AttachToken != "new-token" || row.Endpoint != "127.0.0.1:12345" || row.ExitReason != "" || f.be.destroyCount() != 0 {
		t.Fatal("old typed Exit altered replacement allocation", err)
	}
}

func TestAttachTerminalObservationFencesIdentityAndPersistsWithoutLeaseRelease(t *testing.T) {
	for _, name := range []string{"current", "backend", "sandbox", "reference", "token", "token-cleared", "captured-token-missing", "project", "worktree", "branch", "run", "operation", "already-exited", "mutable-observation"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sessions.json")
			store, err := registry.NewFileStore(path)
			if err != nil {
				t.Fatal(err)
			}
			observed := registry.Session{ID: "s-terminal", Backend: "docker", SandboxID: "sandbox-old", SandboxRef: "reference-old", AttachToken: "token-old", Endpoint: "127.0.0.1:23456", ProjectID: "project", WorktreeID: "worktree", BranchID: "branch", Status: registry.StatusRunning}
			current := observed.Clone()
			switch name {
			case "backend":
				current.Backend = "replacement"
			case "sandbox":
				current.SandboxID = "replacement"
			case "reference":
				current.SandboxRef = "replacement"
			case "token":
				current.AttachToken = "replacement"
			case "token-cleared":
				current.AttachToken = ""
			case "captured-token-missing":
				observed.AttachToken, current.AttachToken = "", ""
			case "project":
				current.ProjectID = "replacement"
			case "worktree":
				current.WorktreeID = "replacement"
			case "branch":
				current.BranchID = "replacement"
			case "run":
				current.ProjectRunID = "replacement"
			case "operation":
				current.ProjectRunOperationID = "replacement"
			case "already-exited":
				current.Status, current.ExitReason = registry.StatusExited, "existing reason"
			case "mutable-observation":
				current.UpdatedAt, current.LastOutputAt, current.AttachCount = time.Now(), time.Now(), 2
			}
			if err := store.Create(current); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			ledgerPath := filepath.Join(dir, "leases.json")
			ledgerBytes := []byte(`{"leases":[{"state":"backend-settled","sessionId":"s-terminal"}]}`)
			if err := os.WriteFile(ledgerPath, ledgerBytes, 0600); err != nil {
				t.Fatal(err)
			}
			a := &API{deps: Deps{Store: store, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, execution: &executionAdmission{path: ledgerPath}}
			a.markAttachedSessionExited(observed)
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := registry.NewFileStore(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := reopened.Get(current.ID)
			if err != nil {
				t.Fatal(err)
			}
			if name == "current" || name == "mutable-observation" {
				if got.Status != registry.StatusExited || got.ExitReason != "process exited" || got.Endpoint != "" || got.AttachToken != "" || got.SandboxID != observed.SandboxID || got.SandboxRef != observed.SandboxRef || got.AttachCount != current.AttachCount {
					t.Fatal("terminal observation not durably recorded with allocation retained")
				}
				a.markAttachedSessionExited(observed)
				repeat, err := os.ReadFile(path)
				if err != nil || string(repeat) != string(after) {
					t.Fatal("duplicate viewer rewrote terminal row", err)
				}
			} else if string(after) != string(before) {
				t.Fatal("incompatible or already-ended observation wrote registry")
			}
			ledgerAfter, err := os.ReadFile(ledgerPath)
			if err != nil || string(ledgerAfter) != string(ledgerBytes) {
				t.Fatal("terminal display observation released reservation", err)
			}
		})
	}
}

func TestAttachTerminalObserverJoinsOneAcceptedWriteAfterCancellation(t *testing.T) {
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int64
	o := &attachedExitObserver{commit: func() { calls.Add(1); close(started); <-release }}
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	o.observe()
	<-started
	for range 100 {
		o.observe()
	}
	go func() { o.finish(); close(finished) }()
	select {
	case <-finished:
		t.Fatal("accepted observation was abandoned")
	default:
	}
	unblock()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("observer did not settle")
	}
	o.observe()
	o.finish()
	if calls.Load() != 1 {
		t.Fatal("observer was replayed after finish")
	}
}

type terminalBlockingStore struct {
	registry.Store
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *terminalBlockingStore) Update(id string, fn func(*registry.Session) error) (registry.Session, error) {
	return s.Store.Update(id, func(row *registry.Session) error {
		err := fn(row)
		if err == nil && row.Status == registry.StatusExited {
			s.once.Do(func() { close(s.entered); <-s.release })
		}
		return err
	})
}

func TestAttachTerminalPersistenceCannotDelaySocketClose(t *testing.T) {
	f := attachNewFixture(t, nil, terminalFixtureSession)
	blocked := &terminalBlockingStore{Store: f.store, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(blocked.release) }) }
	defer release()
	f.api.deps.Store = blocked
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := f.dial(ctx, t)
	defer c.CloseNow()
	f.sandboxd.push([]byte("\x03exit status 0"))
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocked.entered:
	case <-ctx.Done():
		t.Fatal("terminal persistence was never attempted")
	}
	f.sandboxd.close <- websocket.StatusNormalClosure
	_, _, err := c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatal("registry persistence blocked the terminal close", err)
	}
	release()
	attachWaitFor(t, 5*time.Second, func() bool {
		row, err := f.store.Get(f.id)
		return err == nil && row.Status == registry.StatusExited && row.AttachCount == 0
	}, "accepted completion was not joined before relay bookkeeping settled")
	if f.be.destroyCount() != 0 {
		t.Fatal("process completion destroyed allocation")
	}
}

type terminalFailStore struct {
	registry.Store
	fail atomic.Bool
}

func (s *terminalFailStore) Update(id string, fn func(*registry.Session) error) (registry.Session, error) {
	if s.fail.Load() {
		return registry.Session{}, errors.New("controlled persistence failure")
	}
	return s.Store.Update(id, fn)
}

func TestAttachTerminalPersistenceFailureDoesNotInventCompletion(t *testing.T) {
	f := attachNewFixture(t, nil, terminalFixtureSession)
	row, err := f.store.Get(f.id)
	if err != nil {
		t.Fatal(err)
	}
	s := &terminalFailStore{Store: f.store}
	s.fail.Store(true)
	f.api.deps.Store = s
	o := f.api.newAttachedExitObserver(row)
	o.observe()
	o.finish()
	got, err := f.store.Get(f.id)
	if err != nil || !reflect.DeepEqual(got, row) {
		t.Fatal("failed durable observation changed live state", err)
	}
	if f.be.destroyCount() != 0 {
		t.Fatal("persistence failure destroyed allocation")
	}
}
