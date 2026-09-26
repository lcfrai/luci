package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// attachTestToken is a literal only because this is a test. Production tokens
// come from ids.NewToken and are never written down; nothing in non-test source
// in this repository may carry one.
const attachTestToken = "t-0123456789abcdef0123456789abcdef"

// attachSandboxd is sandboxd's /attach as this task needs it: bearer auth, a
// replay frame on connect, then an echo. It writes opaque bytes on purpose.
type attachSandboxd struct {
	srv    *httptest.Server
	replay []byte
	conns  atomic.Int64
	out    chan []byte
}

func attachNewSandboxd(t *testing.T, replay []byte) *attachSandboxd {
	t.Helper()
	f := &attachSandboxd{replay: replay, out: make(chan []byte, 8)}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/attach", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+attachTestToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		c.SetReadLimit(backend.MaxMessageBytes) // site one, standing in for Group A's
		f.conns.Add(1)

		if len(f.replay) > 0 {
			if err := c.Write(r.Context(), websocket.MessageBinary, f.replay); err != nil {
				return
			}
		}

		// One writer goroutine, so the echo and the pushes cannot interleave
		// on the same conn. The reader funnels into the same channel.
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, data, err := c.Read(r.Context())
				if err != nil {
					return
				}
				select {
				case f.out <- data:
				case <-r.Context().Done():
					return
				}
			}
		}()
		for {
			select {
			case <-done:
				return
			case <-r.Context().Done():
				return
			case b := <-f.out:
				if err := c.Write(r.Context(), websocket.MessageBinary, b); err != nil {
					return
				}
			}
		}
	})

	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// endpoint is host:port, the shape backend.Sandbox.Endpoint carries.
func (f *attachSandboxd) endpoint() string { return strings.TrimPrefix(f.srv.URL, "http://") }

// push makes the sandbox produce output nobody asked for, which is what an
// agent spends most of its time doing.
func (f *attachSandboxd) push(b []byte) { f.out <- b }

// attachBackend is a backend.Backend that records every Destroy. Attach goes
// through the real backend.Dial, so the bearer header and read-limit site two
// are exercised rather than faked.
type attachBackend struct {
	sb        backend.Sandbox
	getErr    error
	attachErr error

	// attaches counts upstream dials. The §10a.1 ordering test needs it: that
	// test asserts NOTHING happens before the gates, and dialling the sandbox is
	// something happening — a hoisted dial would leave the registry untouched
	// and slip past a check that only counted writes.
	attaches atomic.Int64

	mu        sync.Mutex
	destroyed []string
}

var _ backend.Backend = (*attachBackend)(nil)

func (b *attachBackend) Kind() backend.Kind { return backend.KindDocker }

func (b *attachBackend) Create(context.Context, backend.Spec) (backend.Sandbox, error) {
	return backend.Sandbox{}, errors.New("attachBackend: Create is not part of task 12")
}

func (b *attachBackend) Get(context.Context, string) (backend.Sandbox, error) {
	if b.getErr != nil {
		return backend.Sandbox{}, b.getErr
	}
	return b.sb, nil
}

func (b *attachBackend) List(context.Context) ([]backend.Sandbox, error) {
	return []backend.Sandbox{b.sb}, nil
}

func (b *attachBackend) Attach(ctx context.Context, _, token string) (*websocket.Conn, error) {
	b.attaches.Add(1)
	if b.attachErr != nil {
		return nil, b.attachErr
	}
	return backend.Dial(ctx, b.sb.Endpoint, token)
}

func (b *attachBackend) Destroy(_ context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.destroyed = append(b.destroyed, id)
	return nil
}

func (b *attachBackend) Close() error { return nil }

func (b *attachBackend) destroyCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.destroyed)
}

// attachCatalogue is an empty api.Enumerator. api.New validates seven Deps
// fields and refuses a nil one with an error, and four of them belong to
// surfaces this file never reaches, so they get the cheapest honest value:
// an empty catalogue, an empty worktree map and the real store for Flags. It
// is declared here rather than borrowed from task 11's sessions_test.go so
// this file still builds when it is the only one in the package.
type attachCatalogue struct{}

// attachNoMaterialiser is this file's own Materialiser (phase 4 task 10,
// review I1: package api cannot see api_test.go's builder). The relay starts
// no process, so any call is a defect.
type attachNoMaterialiser struct{ t *testing.T }

func (m attachNoMaterialiser) Materialise(context.Context, MaterialiseRequest) (MaterialiseResult, error) {
	m.t.Error("the attach relay called the Materialiser; it starts no process")
	return MaterialiseResult{}, nil
}

func (attachCatalogue) Projects(context.Context) ([]project.Project, error)        { return nil, nil }
func (attachCatalogue) Branches(context.Context, string) ([]project.Branch, error) { return nil, nil }

func (attachCatalogue) Worktrees(context.Context, string) ([]project.Worktree, error) {
	return nil, nil
}

var _ Enumerator = attachCatalogue{}

// attachCountingStore is registry.Store plus two counters. It is a DECORATOR
// over the real FileStore, not a second implementation, which is why this file
// does not call registrytest.RunStoreContract. Replace it with a from-scratch
// in-memory Store and that suite becomes mandatory.
//
// updates counts Update alone, because the throttle test asserts exact numbers
// and its own Create must not move them. writes counts EVERY mutating method,
// because the §10a.1 ordering test has to catch a write of any shape appearing
// ahead of a gate — including one this handler does not make today.
type attachCountingStore struct {
	registry.Store
	updates atomic.Int64
	writes  atomic.Int64
}

func (s *attachCountingStore) Create(sess registry.Session) error {
	s.writes.Add(1)
	return s.Store.Create(sess)
}

func (s *attachCountingStore) Update(id string, mutate func(*registry.Session) error) (registry.Session, error) {
	s.updates.Add(1)
	s.writes.Add(1)
	return s.Store.Update(id, mutate)
}

func (s *attachCountingStore) Delete(id string) error {
	s.writes.Add(1)
	return s.Store.Delete(id)
}

type attachFixture struct {
	srv      *httptest.Server
	api      *API
	store    *attachCountingStore
	be       *attachBackend
	sandboxd *attachSandboxd
	id       string
}

// attachNewFixture wires a running fake sandbox, a real FileStore holding one
// running session, a recording backend and the handler under test.
func attachNewFixture(t *testing.T, replay []byte, shape func(*registry.Session)) *attachFixture {
	t.Helper()

	sandboxd := attachNewSandboxd(t, replay)

	// NOT t.TempDir(), and this is not a preference. httptest.Server.Close does
	// not wait for a HIJACKED connection, so relayAttached's deferred
	// tracker.flushFinal() and addAttachCount(-1) are still rewriting
	// sessions.json while t.TempDir()'s RemoveAll walks the directory — and
	// task 7's FileStore widens the window, because it writes through
	// os.CreateTemp in the same directory and retries the rename for 300 ms.
	// On Windows that is "TempDir RemoveAll cleanup: The directory is not
	// empty", measured at 3 runs in 5, failing whichever test happened to end
	// with a live attach. The Cleanup below waits for the relay to unwind and
	// then ignores the removal error, so the worst case is a leftover
	// directory rather than a red suite.
	dir, err := os.MkdirTemp("", "panacea-attach-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	base, err := registry.NewFileStore(filepath.Join(dir, "sessions.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	store := &attachCountingStore{Store: base}

	now := time.Now()
	sess := registry.Session{
		ID:          "s-00000001",
		Title:       "attach",
		Harness:     "claude",
		Status:      registry.StatusRunning,
		Backend:     string(backend.KindDocker),
		SandboxID:   "sb-00000001",
		SandboxRef:  "c0ffeec0ffee",
		Endpoint:    sandboxd.endpoint(),
		AttachToken: attachTestToken,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if shape != nil {
		shape(&sess)
	}
	if err := store.Create(sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Registered BEFORE the server's own Cleanup so that it runs AFTER it:
	// t.Cleanup is LIFO, and the relay only unwinds once the client socket is
	// gone. AttachCount back to zero is the barrier, because relayAttached
	// defers flushFinal before the decrement and defers run in reverse.
	id := sess.ID
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			s, err := store.Get(id)
			if err != nil || s.AttachCount == 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		_ = os.RemoveAll(dir)
	})

	be := &attachBackend{sb: backend.Sandbox{
		ID:       sess.SandboxID,
		Kind:     backend.KindDocker,
		Ref:      sess.SandboxRef,
		Endpoint: sandboxd.endpoint(),
		State:    backend.StateRunning,
	}}

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := New(Deps{
		Version:      "test",
		Backend:      be,
		BackendReady: func() bool { return true },
		Store:        store,
		Flags:        base,
		Projects:     attachCatalogue{},
		Worktrees:    worktree.Static{},
		Origin:       httpx.NewOriginPolicy(nil, quiet),
		Log:          quiet,
		// Required by New; the relay never creates a sandbox.
		ConversationsDir: filepath.Join(t.TempDir(), "conversations"),
		// Required by New (phase 4 task 10); the relay starts no process.
		Materialiser: attachNoMaterialiser{t: t},
	})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}

	// A local recorder, not the handler New returns. Task 9's middleware answers
	// forbidden_host BEFORE forbidden_origin and §3.16 orders this handler's
	// own first two checks the other way round, so the tests that pin the
	// handler's order cannot go through New. registerAttach is what mounts the
	// pattern, so the pattern is pinned here; that New calls it is pinned by
	// TestNewActuallyRoutesTheAttachRoute below.
	//
	// It is a *routes and not an *http.ServeMux because group-C §1A changed the
	// registerX signature after this task was planned: a *routes records what it
	// registers so contract §10a's audit can walk the real table, and it is an
	// http.Handler in its own right, so httptest can serve it directly.
	mux := newRoutes()
	a.registerAttach(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &attachFixture{srv: srv, api: a, store: store, be: be, sandboxd: sandboxd, id: sess.ID}
}

// dial opens the attach socket as the desktop does.
func (f *attachFixture) dial(ctx context.Context, t *testing.T) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/v1/sessions/" + f.id + "/attach"
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("attach dial: %v", err)
	}
	// The desktop is a browser, which has no read limit. A Go test client
	// does, so raise it or the replay frame fails on THIS side and the test
	// blames the handler.
	c.SetReadLimit(backend.MaxMessageBytes)
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// plainGET issues an ordinary HTTP GET, which is what every pre-upgrade check
// answers. host, when non-empty, overrides the Host header.
func (f *attachFixture) plainGET(t *testing.T, path, origin, host string) (int, httpx.Problem) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.srv.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	var p httpx.Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem body: %v", err)
	}
	return resp.StatusCode, p
}

func TestAttachRejectsUnknownOriginBeforeAnythingElse(t *testing.T) {
	f := attachNewFixture(t, nil, nil)

	// A session id that does not exist: if the lookup ran first this is 404.
	status, p := f.plainGET(t, "/v1/sessions/s-nope/attach", "http://evil.example", "")
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
	if p.Error != httpx.CodeForbiddenOrigin {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeForbiddenOrigin)
	}
	if p.Details["origin"] != "http://evil.example" {
		t.Errorf("details.origin = %q, want the rejected origin echoed back", p.Details["origin"])
	}
}

func TestAttachRejectsNonLoopbackHostBeforeTheSessionLookup(t *testing.T) {
	f := attachNewFixture(t, nil, nil)

	// DNS rebinding: a name that resolves to 127.0.0.1 but is not loopback.
	status, p := f.plainGET(t, "/v1/sessions/s-nope/attach", "", "attacker.example:8730")
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
	if p.Error != httpx.CodeForbiddenHost {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeForbiddenHost)
	}
}

func TestAttachUnknownSessionIs404(t *testing.T) {
	f := attachNewFixture(t, nil, nil)

	status, p := f.plainGET(t, "/v1/sessions/s-nope/attach", "", "")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	if p.Error != httpx.CodeSessionNotFound {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeSessionNotFound)
	}
}

func TestAttachExitedSessionIs409(t *testing.T) {
	f := attachNewFixture(t, nil, func(s *registry.Session) {
		s.Status = registry.StatusExited
		s.ExitReason = "killed"
	})

	status, p := f.plainGET(t, "/v1/sessions/"+f.id+"/attach", "", "")
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if p.Error != httpx.CodeSandboxNotRunning {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeSandboxNotRunning)
	}
}

// TestAttachBackendVerdictsBeforeTheUpgrade is check 4's second half: the
// record says running, the backend disagrees. Every one of these must be an
// HTTP status, because after the upgrade the handler can only close a socket.
func TestAttachBackendVerdictsBeforeTheUpgrade(t *testing.T) {
	cases := []struct {
		name       string
		getErr     error
		sandbox    *backend.Sandbox
		wantStatus int
		wantCode   string
	}{
		{
			name:       "sandbox gone",
			getErr:     fmt.Errorf("get sb-00000001: %w", backend.ErrNotFound),
			wantStatus: http.StatusConflict,
			wantCode:   httpx.CodeSandboxNotRunning,
		},
		{
			name:       "backend down",
			getErr:     fmt.Errorf("docker: %w", backend.ErrUnavailable),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   httpx.CodeBackendUnavailable,
		},
		{
			name:       "published off loopback",
			getErr:     fmt.Errorf("sb-00000001 on 0.0.0.0: %w", backend.ErrNonLoopbackPublish),
			wantStatus: http.StatusInternalServerError,
			wantCode:   httpx.CodeNonLoopbackPublish,
		},
		{
			name:       "no endpoint yet",
			sandbox:    &backend.Sandbox{ID: "sb-00000001", State: backend.StateStarting},
			wantStatus: http.StatusConflict,
			wantCode:   httpx.CodeSandboxNotRunning,
		},
		{
			name:       "stopped",
			sandbox:    &backend.Sandbox{ID: "sb-00000001", Endpoint: "127.0.0.1:1", State: backend.StateStopped},
			wantStatus: http.StatusConflict,
			wantCode:   httpx.CodeSandboxNotRunning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := attachNewFixture(t, nil, nil)
			f.be.getErr = tc.getErr
			if tc.sandbox != nil {
				f.be.sb = *tc.sandbox
			}

			status, p := f.plainGET(t, "/v1/sessions/"+f.id+"/attach", "", "")
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (problem %q)", status, tc.wantStatus, p.Error)
			}
			if p.Error != tc.wantCode {
				t.Errorf("error = %q, want %q", p.Error, tc.wantCode)
			}
			if n := f.be.destroyCount(); n != 0 {
				t.Errorf("Destroy called %d times; a refused attach destroys nothing, including an off-loopback sandbox — that is the reaper's job", n)
			}
		})
	}
}

// TestAttachStartingSandboxIsAllowedThrough is the other side of the same
// check. Create returns 201 the moment the record exists and the cockpit
// attaches immediately, so rejecting StateStarting breaks the product's
// central flow.
func TestAttachStartingSandboxIsAllowedThrough(t *testing.T) {
	f := attachNewFixture(t, nil, func(s *registry.Session) { s.Status = registry.StatusStarting })
	f.be.sb.State = backend.StateStarting

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := f.dial(ctx, t)
	if err := c.Write(ctx, websocket.MessageBinary, []byte{0x01, 'h'}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatalf("Read: %v", err)
	}
}

func TestAttachWithoutUpgradeHeadersIs426(t *testing.T) {
	f := attachNewFixture(t, nil, nil)

	status, p := f.plainGET(t, "/v1/sessions/"+f.id+"/attach", "", "")
	if status != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want 426", status)
	}
	if p.Error != httpx.CodeUpgradeRequired {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeUpgradeRequired)
	}
}

// TestNewActuallyRoutesTheAttachRoute is the only test in this file that goes
// through the handler New returns. Every other one drives the fixture's own
// recorder, so not one of them would notice if a.registerAttach(guarded) were
// missing from New: the route would exist in the tests and nowhere else, and
// the suite would be entirely green over an endpoint the product never serves.
// That is the group-C contract's ruling 2, and 426 is the right signal for it —
// a registered route refusing a non-upgrade request, where an unregistered one
// is the mux's 404.
func TestNewActuallyRoutesTheAttachRoute(t *testing.T) {
	f := attachNewFixture(t, nil, nil)

	r := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+f.id+"/attach", nil)
	// httptest.NewRequest sets Host: example.com, which task 9's loopback guard
	// correctly answers 403 to. The desktop's Host is the address it dialled.
	r.Host = "127.0.0.1:8730"
	rec := httptest.NewRecorder()
	f.api.ServeHTTP(rec, r)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("GET %s through New = 404: the a.registerAttach(guarded) line is missing from New", r.URL.Path)
	}
	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want 426; body = %s", rec.Code, rec.Body.String())
	}
}

// TestAttachCarriesAMessageEightTimesTheDefaultReadLimit is SetReadLimit site
// three. 262145 = 256 KiB of scrollback + one kind byte, the largest frame
// sandboxd emits; the coder/websocket default is 32768.
func TestAttachCarriesAMessageEightTimesTheDefaultReadLimit(t *testing.T) {
	const big = 256<<10 + 1

	replay := make([]byte, big)
	for i := range replay {
		replay[i] = byte('a' + i%26)
	}
	replay[0] = 0x00 // stdout

	f := attachNewFixture(t, replay, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := f.dial(ctx, t)

	_, got, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("reading the replay frame: %v", err)
	}
	if len(got) != big {
		t.Fatalf("replay frame = %d bytes, want %d", len(got), big)
	}

	// Now the other direction: one large paste, arriving as a single Stdin
	// frame. This is the one the handler's own Accept limit governs.
	paste := make([]byte, big)
	paste[0] = 0x01 // stdin
	if err := c.Write(ctx, websocket.MessageBinary, paste); err != nil {
		t.Fatalf("writing the paste: %v", err)
	}
	_, echoed, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("reading the echoed paste: %v", err)
	}
	if len(echoed) != big {
		t.Errorf("echoed paste = %d bytes, want %d", len(echoed), big)
	}
}

// TestAttachRecordsOutputAndBalancesTheCounter covers the two registry effects
// of a relay and nothing else. The hook writes before the relay's write, so
// reading the pushed frame on the client is enough to know the flush landed.
func TestAttachRecordsOutputAndBalancesTheCounter(t *testing.T) {
	f := attachNewFixture(t, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := f.dial(ctx, t)

	f.sandboxd.push([]byte{0x00, 'o', 'u', 't'})
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatalf("Read: %v", err)
	}

	sess, err := f.store.Get(f.id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sess.LastOutputAt.IsZero() {
		t.Error("LastOutputAt is zero after the sandbox produced output")
	}
	if sess.AttachCount != 1 {
		t.Errorf("AttachCount = %d while one client is attached, want 1", sess.AttachCount)
	}

	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// The decrement runs on the handler's goroutine after the relay returns,
	// so this one genuinely has to be waited for.
	attachWaitFor(t, 5*time.Second, func() bool {
		s, err := f.store.Get(f.id)
		return err == nil && s.AttachCount == 0
	}, "AttachCount never returned to 0")
}

// attachWaitFor polls until cond holds. It exists so the two genuinely
// asynchronous assertions do not sleep a fixed interval and flake on a loaded
// machine. The attach prefix is ruling 5: a bare waitFor in package api is a
// name task 11 or task 13 will reach for too.
func attachWaitFor(t *testing.T, limit time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

// TestADroppedSocketDestroysNothing is contract §3.16's closing rule and the
// mirror of task 11's rule 1. A lid closing, a Wi-Fi handover or a desktop
// restart costs a WebSocket and nothing else: the container keeps running, the
// record keeps its status, and reattaching works.
func TestADroppedSocketDestroysNothing(t *testing.T) {
	f := attachNewFixture(t, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c := f.dial(ctx, t)
	if err := c.Write(ctx, websocket.MessageBinary, []byte{0x01, 'h'}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatalf("Read: %v", err)
	}

	// CloseNow, not Close: no close frame, which is what a dropped Wi-Fi
	// connection looks like to the handler.
	if err := c.CloseNow(); err != nil {
		t.Fatalf("CloseNow: %v", err)
	}

	attachWaitFor(t, 5*time.Second, func() bool {
		s, err := f.store.Get(f.id)
		return err == nil && s.AttachCount == 0
	}, "AttachCount never returned to 0 after the socket dropped")

	if n := f.be.destroyCount(); n != 0 {
		t.Fatalf("Destroy called %d times after a dropped socket; it must be called zero times", n)
	}

	sess, err := f.store.Get(f.id)
	if err != nil {
		t.Fatalf("the session record is gone after a dropped socket: %v", err)
	}
	if sess.Status != registry.StatusRunning {
		t.Errorf("status = %q after a dropped socket, want %q", sess.Status, registry.StatusRunning)
	}
	if sess.ExitReason != "" {
		t.Errorf("exitReason = %q after a dropped socket, want empty", sess.ExitReason)
	}

	// And the sandbox is still there to come back to. The first relay's
	// upstream socket is closed before AttachCount reaches zero, so the fake
	// sandboxd's first handler has already unwound and cannot steal this
	// exchange out of the channel it shares with the second.
	again := f.dial(ctx, t)
	if err := again.Write(ctx, websocket.MessageBinary, []byte{0x01, 'y'}); err != nil {
		t.Fatalf("reattach Write: %v", err)
	}
	if _, _, err := again.Read(ctx); err != nil {
		t.Fatalf("reattach Read: %v", err)
	}
}

// TestAttachWritesNothingUntilTheOriginAndUpgradeChecksHavePassed is the
// CONDITION ON THE §10a.1 EXCEPTION, and the reason this route may sit in
// catalogUpgradeGatedRoutes at all.
//
// Contract §10a.1 carries the argument and the citations; the one line of it
// that matters here is that the exception is conditional on ORDERING — no write
// until check 1 has compared the Origin and check 5 has seen the upgrade
// headers. A refactor moving a write ahead of either gate destroys the argument
// while the allowlist entry goes on declaring the route safe: a comment
// asserting a property the code no longer has, which is the failure shape this
// phase has hit three times.
//
// So the property is asserted rather than described. Each case is refused at a
// different gate, and each asserts that NOTHING happened: the registry saw no
// mutation of any kind, and the backend saw neither a Destroy nor an upstream
// Attach. Three counters rather than one, because the claim is not "no writes"
// but "no effects", and each counter closes a different way of having one —
//
//   - store.writes covers Create, Update AND Delete, not just the Update this
//     handler uses today, so a future write of another shape is caught;
//   - be.destroyCount() covers the destructive effect §3.16 forbids outright;
//   - be.attaches covers the upstream dial, which touches neither the registry
//     nor Destroy and so would otherwise sail past both. It is the effect a
//     refactor is most likely to hoist, because moving the dial earlier looks
//     like a latency win.
func TestAttachWritesNothingUntilTheOriginAndUpgradeChecksHavePassed(t *testing.T) {
	for _, tc := range []struct {
		name       string
		gate       string
		origin     string
		host       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "refused at the origin gate",
			gate:       "check 1, the Origin allowlist — the gate new WebSocket() cannot get past",
			origin:     "http://evil.example",
			wantStatus: http.StatusForbidden,
			wantCode:   httpx.CodeForbiddenOrigin,
		},
		{
			name:       "refused at the host gate",
			gate:       "check 2, the loopback Host guard",
			host:       "attacker.example:8730",
			wantStatus: http.StatusForbidden,
			wantCode:   httpx.CodeForbiddenHost,
		},
		{
			name:       "refused at the upgrade gate",
			gate:       "check 5, the upgrade headers — the gate <img src> cannot get past",
			wantStatus: http.StatusUpgradeRequired,
			wantCode:   httpx.CodeUpgradeRequired,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := attachNewFixture(t, nil, nil)

			// The baseline is taken AFTER the fixture, whose own Create is a
			// legitimate write. Only what the request does is under test.
			before := f.store.writes.Load()

			status, p := f.plainGET(t, "/v1/sessions/"+f.id+"/attach", tc.origin, tc.host)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (problem %q)", status, tc.wantStatus, p.Error)
			}
			if p.Error != tc.wantCode {
				t.Errorf("error = %q, want %q", p.Error, tc.wantCode)
			}

			if got := f.store.writes.Load() - before; got != 0 {
				t.Errorf("the registry was written %d times by a request refused at %s. "+
					"Contract §10a.1 lets this GET write ONLY because every write follows the "+
					"upgrade gate; a write that happens before one is reachable by a page that "+
					"never sees the response, and catalogUpgradeGatedRoutes now says something "+
					"untrue about this route.", got, tc.gate)
			}
			if n := f.be.destroyCount(); n != 0 {
				t.Errorf("Destroy called %d times by a request refused at %s; a refused attach "+
					"touches nothing", n, tc.gate)
			}
			if n := f.be.attaches.Load(); n != 0 {
				t.Errorf("Backend.Attach called %d times by a request refused at %s. The §10a.1 "+
					"argument is that NOTHING happens before the gates, not merely that nothing is "+
					"written — an upstream dial reaches the sandbox, and contract §3.16 puts it "+
					"after the upgrade for a second reason: a dial the client never completes "+
					"consumes a scrollback replay nobody reads.", n, tc.gate)
			}
		})
	}
}

// TestAttachUpstreamDialFailureClosesTheSocketAndDestroysNothing is the third
// clause of the handler's "destroys nothing" comment, and the only one the plan
// left unpinned: the dropped socket and the off-loopback sandbox are covered
// above, a failed upstream dial was not. It is also the one refusal that can
// only be a close code, because §3.16 puts the dial AFTER the upgrade.
func TestAttachUpstreamDialFailureClosesTheSocketAndDestroysNothing(t *testing.T) {
	f := attachNewFixture(t, nil, nil)
	f.be.attachErr = fmt.Errorf("sb-00000001: %w", backend.ErrUnauthorized)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// The upgrade itself succeeds — all five checks passed — so the failure
	// reaches the desktop as a close frame and not as a status.
	c := f.dial(ctx, t)
	_, _, err := c.Read(ctx)
	if err == nil {
		t.Fatal("Read succeeded; a failed upstream dial must close the client socket")
	}
	if got, want := websocket.CloseStatus(err), websocket.StatusPolicyViolation; got != want {
		t.Errorf("close status = %d, want %d (%v)", got, want, err)
	}

	if n := f.be.destroyCount(); n != 0 {
		t.Errorf("Destroy called %d times after a failed upstream dial; it must be called zero times", n)
	}
	sess, err := f.store.Get(f.id)
	if err != nil {
		t.Fatalf("the session record is gone after a failed upstream dial: %v", err)
	}
	if sess.Status != registry.StatusRunning {
		t.Errorf("status = %q, want %q", sess.Status, registry.StatusRunning)
	}
	// The increment lives after the dial, so a dial that never succeeded must
	// not leave a phantom client on the record.
	if sess.AttachCount != 0 {
		t.Errorf("AttachCount = %d after a dial that never connected, want 0", sess.AttachCount)
	}
}

// TestAttachCloseStatusMapsEveryBackendSentinel pins the close codes. They are
// the only diagnosis the desktop gets for a post-upgrade failure, so a sentinel
// falling through to StatusInternalError is a real loss of information.
func TestAttachCloseStatusMapsEveryBackendSentinel(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want websocket.StatusCode
	}{
		{"token rejected", backend.ErrUnauthorized, websocket.StatusPolicyViolation},
		{"published off loopback", backend.ErrNonLoopbackPublish, websocket.StatusPolicyViolation},
		{"not found", backend.ErrNotFound, websocket.StatusTryAgainLater},
		{"not running", backend.ErrNotRunning, websocket.StatusTryAgainLater},
		{"backend unavailable", backend.ErrUnavailable, websocket.StatusTryAgainLater},
		{"anything else", errors.New("a novel failure"), websocket.StatusInternalError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Wrapped, because every backend returns these through %w and a
			// switch written on == rather than errors.Is would pass unwrapped.
			got, reason := attachCloseStatus(fmt.Errorf("dial sb-1: %w", tc.err))
			if got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
			// RFC 6455 caps the whole close payload at 125 bytes, two of which
			// are the code. Over 123 and Close fails, so the socket dies with
			// no diagnosis at all — the opposite of this function's purpose.
			if len(reason) > 123 {
				t.Errorf("reason is %d bytes, over RFC 6455's 123-byte limit: %q", len(reason), reason)
			}
		})
	}
}

// TestIsWebSocketUpgradeReadsConnectionAsATokenList covers the branch the
// handshake tests cannot reach: coder/websocket's Dial sends "Connection:
// Upgrade" alone, so every other test here passes against a whole-string
// comparison. Through a proxy the header arrives as a list, and a whole-string
// comparison answers 426 to a browser that did everything right.
func TestIsWebSocketUpgradeReadsConnectionAsATokenList(t *testing.T) {
	for _, tc := range []struct {
		name       string
		connection []string
		upgrade    string
		want       bool
	}{
		{"the plain pair", []string{"Upgrade"}, "websocket", true},
		{"a proxy's token list", []string{"keep-alive, Upgrade"}, "websocket", true},
		{"list with odd spacing", []string{"keep-alive,Upgrade , TE"}, "websocket", true},
		{"repeated header lines", []string{"keep-alive", "Upgrade"}, "websocket", true},
		{"lower case throughout", []string{"upgrade"}, "WebSocket", true},
		{"no Connection header", nil, "websocket", false},
		{"Connection without the token", []string{"keep-alive"}, "websocket", false},
		{"a token that merely contains it", []string{"upgrades"}, "websocket", false},
		{"no Upgrade header", []string{"Upgrade"}, "", false},
		{"upgrading to something else", []string{"Upgrade"}, "h2c", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1/sessions/s-1/attach", nil)
			for _, v := range tc.connection {
				r.Header.Add("Connection", v)
			}
			if tc.upgrade != "" {
				r.Header.Set("Upgrade", tc.upgrade)
			}
			if got := isWebSocketUpgrade(r); got != tc.want {
				t.Errorf("isWebSocketUpgrade = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestOutputTrackerThrottlesRegistryWrites is contract §5.4's "at most once
// every two seconds". Without it, a chatty agent rewrites the whole registry
// document thousands of times a second.
func TestOutputTrackerThrottlesRegistryWrites(t *testing.T) {
	// t.TempDir() IS safe here, unlike in attachNewFixture: nothing in this
	// test attaches, so no handler goroutine is still writing when the test
	// body returns and the cleanup runs.
	base, err := registry.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	store := &attachCountingStore{Store: base}
	if err := store.Create(registry.Session{ID: "s-00000001", Status: registry.StatusRunning}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	clock := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	tr := newOutputTracker(store, "s-00000001", 2*time.Second,
		func() time.Time { return clock },
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	for i := 0; i < 1000; i++ {
		clock = clock.Add(time.Millisecond)
		tr.observe(64)
	}
	if got := store.updates.Load(); got != 1 {
		t.Fatalf("writes for 1000 frames inside one second = %d, want 1", got)
	}

	clock = clock.Add(2 * time.Second)
	tr.observe(64)
	if got := store.updates.Load(); got != 2 {
		t.Fatalf("writes after crossing the interval = %d, want 2", got)
	}

	clock = clock.Add(10 * time.Millisecond)
	tr.observe(64)
	tr.flushFinal()
	if got := store.updates.Load(); got != 3 {
		t.Fatalf("writes after the final flush = %d, want 3 — the last value must not be lost to the throttle", got)
	}

	tr.flushFinal()
	if got := store.updates.Load(); got != 3 {
		t.Errorf("writes after a second flushFinal = %d, want 3 — there was nothing new to write", got)
	}

	sess, err := store.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !sess.LastOutputAt.Equal(clock) {
		t.Errorf("LastOutputAt = %s, want %s", sess.LastOutputAt, clock)
	}
}
