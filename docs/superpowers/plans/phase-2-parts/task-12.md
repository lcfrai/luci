### Task 12: The attach relay

This is the task the whole cockpit is for. Everything before it creates a sandbox; this is the one that puts a terminal in front of it. It is also the task where a mistake is silent: the five pre-upgrade checks, the read limit and the "destroy nothing" rule all fail *later*, under load or under a big paste, never on the happy path a demo takes.

**`SetReadLimit` is site three of three and all three must exist or the pipe breaks at a size light testing never reaches.** Contract §2.2 names them: sandboxd's `/attach` (task 3), `backend.Dial` (task 4) and the control plane's client `Accept` (here). `github.com/coder/websocket` v1.8.12 caps one inbound message at 32768 bytes (`read.go:107`); the offender is not stdout, which arrives in small frames, but sandboxd's replay frame — 256 KiB of scrollback plus a kind byte, 262145 bytes — and a large paste, which arrives from the desktop as one Stdin frame. Over the limit, `Read` fails with `read limited at 32769 bytes` (`read.go:492`) — **32769 and not 32768**, because `SetReadLimit` stores `n+1` so the fin frame fits (`read.go:97-104`) and the default is reported the same way, so the number is always one more than the limit in force — the library closes the socket with 1009, and the relay ends. The user sees a terminal that dies on reconnect to a busy agent and works perfectly on a fresh one. Use `backend.MaxMessageBytes`, never a literal: a second literal is how one of the three sites drifts.

**A dropped socket destroys nothing.** This is contract §3.16's closing rule and the mirror of task 11's rule 1. A laptop lid closing, a Wi-Fi handover or a desktop restart must cost the user a WebSocket and nothing else — the container keeps running, the registry record keeps its status, and the scrollback is still there when they attach again. Nothing in this file calls `Backend.Destroy`, and a test asserts it with a backend that records every call. Destruction belongs to the user's `DELETE` (task 11) and to the reaper (task 13), which are deliberate and observable; a relay that tidies up after itself would kill a running agent because someone closed a laptop.

**The counters observe sizes, never bytes.** `proxy.RelayHooked` is `Relay` plus a `Hooks` struct whose two functions receive `len(data)` and nothing else. `docs/PROGRAM.md` forbids deriving meaning from agent output, so there is no variant that passes the payload and none may be added — a hook holding a `[]byte` is an invitation. `Relay` keeps its exact signature and its existing test; it becomes a one-line call to `RelayHooked(ctx, c, u, Hooks{})`, so no caller changes. That is contract §1.2 correction 8: counting is not reading.

Two decisions where the contract is silent, made here so nobody re-derives them. **(a) Check 4 asks the backend, not just the registry.** §3.16 says "session `exited`, or the backend reports no endpoint → 409", so the handler reads the record *and* calls `Backend.Get`, which is what makes `ErrNonLoopbackPublish` a readable 500 before the upgrade rather than a close code after it. A sandbox in `StateStarting` is allowed through, because create returns 201 the moment the record exists and the cockpit attaches immediately; rejecting `starting` would break the create-then-attach flow the product is built on. **(b) The handler refuses a non-loopback sandbox and does not destroy it.** Contract §2.2's sentinel comment says "→ 500, and the sandbox is destroyed", but that sentence was written for `Create`, where the caller owns the container it just made. Here, destroying inside a read-only attach would violate the rule above and race the reaper, which the Group-A/B amendment already requires to destroy a `NonLoopback` sandbox and log it distinctly. So: 500, a loud log line, no destruction.

The throttle is contract §5.4. `OnUpstream` records `time.Now()` into a local and flushes it to the registry through `store.Update` **at most once every two seconds**, because per-frame persistence rewrites the whole JSON document thousands of times a second. `idle` therefore means exactly "no output has crossed the relay for N seconds" — not "the agent is waiting for you", which is `BLOCKED-ON-YOU` and needs a runtime signal `docs/AGENTIC-UX.md` §3 calls a hard dependency. Phase 2 ships four mechanically-derived statuses and no more. Deriving them is task 13's; this task only supplies the timestamp and `attachCount`.

**What tasks 9, 10 and 11 must already have produced, and the one line this task adds to `api.go`.** Task 10 owns `package api` and its names win — the group-C contract at `.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/group-c-package-api-contract.md` ruling 1 fixes the spellings and this file is written against them: the receiver is **`*API`** and there is no `Server` type, `func New(d Deps) (*API, error)` returns **two** values and rejects a nil dep with an error rather than a panic, the origin field is **`Origin`** and not `Origins`, and `Deps` carries seven fields `New` refuses to start without — `Backend`, `BackendReady`, `Store`, `Flags`, `Projects`, `Worktrees`, `Origin` — of which this task reads three. Task 10 does **not** register the attach route: `task-10.md:34` and the reserved comment in `New` both say task 12 adds `a.registerAttach(guarded)` itself, and ruling 2 requires it in the same step that defines the method. Step 9 does exactly that, and step 6 pins it with the one test that drives the route through the handler `New` returns. `httpx.LoopbackHost(host string) bool` is task 9's `Host` predicate — §1.1 calls the loopback guard "middleware and a predicate" without naming the predicate, so if task 9 exported it under another name, use that one rather than writing a second copy of the rule.

**Files:**
- `control-plane/internal/api/attach.go` — `registerAttach`, the five pre-upgrade checks, the upgrade, read-limit site three, the relay, the throttled `lastOutputAt` flush and `attachCount` (new)
- `control-plane/internal/api/attach_test.go` — the checks, the 262145-byte message in both directions, the throttle, the counters, the proof that a dropped socket destroys nothing, and the one test that goes through `New` (new)
- `control-plane/internal/api/api.go` — **one line**, `a.registerAttach(guarded)`, on the line task 10 reserved for it in `New`, added in step 9 alongside the method it names (modified)
- `control-plane/internal/proxy/proxy.go` — `Hooks` and `RelayHooked`; `Relay` keeps its signature (modified)
- `control-plane/internal/proxy/proxy_test.go` — counter coverage alongside the existing relay test (modified)

**Interfaces:**
- Produces:
  - `proxy.Hooks{OnUpstream, OnClient func(n int)}` and `proxy.RelayHooked(ctx, client, upstream *websocket.Conn, h Hooks) error` — consumed here and by nothing else in Phase 2
  - `(*api.API).registerAttach(*http.ServeMux)` — the name task 10 reserved for this task — and `(*api.API).handleAttach(http.ResponseWriter, *http.Request)`
  - unexported in package `api`, all of them: the methods `(*API).attachSandboxReady`, `(*API).relayAttached` and `(*API).addAttachCount`; the type `outputTracker` with `newOutputTracker`, `observe` and `flushFinal`; and the functions `attachCloseStatus`, `isWebSocketUpgrade` and `attachHeaderHasToken`, plus the constant `lastOutputFlushInterval`
  - unexported in the internal test file: `attachTestToken`, `attachSandboxd`/`attachNewSandboxd`, `attachBackend`, `attachCatalogue`, `attachCountingStore`, `attachFixture`/`attachNewFixture` and `attachWaitFor`. **Every package-level name this file adds is attach-prefixed**, which is the group-C contract's ruling 5: task 11's internal test file is in `package api` too, and the first draft of both tasks declared a bare `fakeBackend`
- Consumes:
  - task 4 — `backend.MaxMessageBytes`, `backend.Backend`, `backend.Sandbox`, `backend.State`/`StateStarting`/`StateStopped`/`StateGone`, `backend.ErrNotFound`, `ErrNotRunning`, `ErrUnavailable`, `ErrUnauthorized`, `ErrNonLoopbackPublish`, and `backend.Dial` in the test's fake backend
  - task 7 — `registry.Store`, `registry.Session`, `registry.StatusRunning`/`StatusStarting`/`StatusExited`, `registry.ErrNotFound`, `registry.NewFileStore`
  - task 9 — `httpx.WriteProblem`, `httpx.Problem`, `(*httpx.OriginPolicy).Allow`, `httpx.NewOriginPolicy`, `httpx.LoopbackHost`, and the codes `httpx.CodeForbiddenOrigin`, `CodeForbiddenHost`, `CodeSessionNotFound`, `CodeSandboxNotRunning`, `CodeUpgradeRequired`, `CodeNonLoopbackPublish` and `CodeBackendUnavailable` — **constants, never literals**: the desktop switches on them and a typo is a code the cockpit can never match
  - task 8 — `project.Project`/`Branch`/`Worktree` and `worktree.Static`, in the test's empty catalogue only
  - task 10 — `api.API` (receiver `a`, field `a.deps`), `api.Deps`, `api.New(Deps) (*API, error)`, `api.Enumerator`, and `(*API).internal(w, r, op, err)` for both 500s on this surface
- Amends: contract §3.2's "complete set" of error codes gains a sixteenth, `non_loopback_publish`, and **task 9 declares the constant** — group-C ruling 7. A sandbox the daemon reports published off loopback *is* running, so reusing `sandbox_not_running` would tell the cockpit something false. Decision (b) above narrows contract §2.2's `ErrNonLoopbackPublish` comment to the `Create` path it was written for; §3.16's "do not destroy anything" governs here and is unchanged.

- [ ] **Step 1: Write the failing tests for `proxy.RelayHooked`**

The proxy is the smaller half of this task and it goes first, because the attach handler cannot be written until the hook type exists.

Three things are under test and the third is the one that protects every existing caller. The counters must fire **once per message, in the right direction, with the payload length** — `OnClient` for desktop→sandbox, `OnUpstream` for sandbox→desktop. Nil hook fields must be safe, because `Relay` will pass an empty `Hooks{}`. And both functions must keep the exact signatures the rest of the phase compiles against, which is a compile-time assertion rather than a test body: a signature change that breaks task 13's wiring should fail the build here, not three tasks later.

There is no test that a hook cannot see payload bytes. That is a property of the type — `func(n int)` cannot receive a `[]byte` — and a test asserting it would be a test of the Go type system. The guard is review: if a future change gives `Hooks` a function taking bytes, it is a `docs/PROGRAM.md` violation and the reason is written in the doc comment in Step 3.

Append to `control-plane/internal/proxy/proxy_test.go`. Add `"fmt"` and `"sync"` to its import block — `fmt` because comparing two `[]int` by their printed form gives a failure message that names the sizes, which `reflect.DeepEqual` does not; `context`, `net/http`, `net/http/httptest`, `strings`, `testing`, `time` and the two non-stdlib imports are already there.

```go
// Compile-time signatures. Relay MUST NOT change: task 13's wiring, and any
// caller outside this phase, is written against the three-argument form.
var (
	_ func(context.Context, *websocket.Conn, *websocket.Conn) error              = proxy.Relay
	_ func(context.Context, *websocket.Conn, *websocket.Conn, proxy.Hooks) error = proxy.RelayHooked
)

// growEchoServer replies to every message with the same bytes plus one. The
// extra byte is what makes the direction test meaningful: a plain echo returns
// the same length in both directions, so swapping OnClient and OnUpstream
// would produce identical numbers and the test would pass while wrong.
func growEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			typ, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if err := c.Write(r.Context(), typ, append(data, '!')); err != nil {
				return
			}
		}
	}))
}

// relayWithHooks stands growEchoServer behind a proxy that relays with h, and
// returns a client connected to the proxy. Everything is torn down by
// t.Cleanup, so a failing test does not leak a listener into the next one.
func relayWithHooks(t *testing.T, h proxy.Hooks) (*websocket.Conn, context.Context) {
	t.Helper()

	upstreamSrv := growEchoServer(t)
	t.Cleanup(upstreamSrv.Close)

	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer client.CloseNow()

		up, _, err := websocket.Dial(r.Context(), "ws"+strings.TrimPrefix(upstreamSrv.URL, "http"), nil)
		if err != nil {
			return
		}
		defer up.CloseNow()

		_ = proxy.RelayHooked(r.Context(), client, up, h)
	}))
	t.Cleanup(proxySrv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(proxySrv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c, ctx
}

// TestRelayHookedCountsEachDirection pins the direction mapping. It needs no
// sleep and no eventually-loop: a hook fires between the read and the write,
// so by the time the echo comes back to the client both hooks for that
// message have already run.
func TestRelayHookedCountsEachDirection(t *testing.T) {
	var mu sync.Mutex
	var toSandbox, toDesktop []int

	c, ctx := relayWithHooks(t, proxy.Hooks{
		OnClient:   func(n int) { mu.Lock(); toSandbox = append(toSandbox, n); mu.Unlock() },
		OnUpstream: func(n int) { mu.Lock(); toDesktop = append(toDesktop, n); mu.Unlock() },
	})

	for _, payload := range [][]byte{{0x01, 'h', 'i'}, {0x01, 'x'}} {
		if err := c.Write(ctx, websocket.MessageBinary, payload); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if _, _, err := c.Read(ctx); err != nil {
			t.Fatalf("Read: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := fmt.Sprint(toSandbox), "[3 2]"; got != want {
		t.Errorf("OnClient sizes = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(toDesktop), "[4 3]"; got != want {
		t.Errorf("OnUpstream sizes = %s, want %s", got, want)
	}
}

// TestRelayHookedToleratesNilHookFields is the case Relay itself takes: an
// empty Hooks value. A nil-check per message is cheaper than making callers
// supply two no-op closures.
func TestRelayHookedToleratesNilHookFields(t *testing.T) {
	c, ctx := relayWithHooks(t, proxy.Hooks{})

	if err := c.Write(ctx, websocket.MessageBinary, []byte{0x01, 'o', 'k'}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	_, got, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := "\x01ok!"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
```

The two expected strings are arithmetic, so check them rather than trusting them: the payloads are `{0x01,'h','i'}` and `{0x01,'x'}`, three bytes and two, which `OnClient` sees unchanged — `[3 2]` — and `growEchoServer` adds one byte to each before `OnUpstream` sees it — `[4 3]`. A plain echo would make both directions read `[3 2]` and a swapped pair of hooks would pass.

- [ ] **Step 2: Run the proxy tests to verify they fail**

```bash
go test ./control-plane/internal/proxy/... -race -count=1
```

Expected: a build failure of the external test package, not a test failure. `proxy` itself still compiles — nothing in `proxy.go` has changed yet — so the error header names `proxy_test` built for `proxy.test`:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/proxy_test [github.com/kpmg-au-emu/panacea-app/control-plane/internal/proxy.test]
control-plane\internal\proxy\proxy_test.go:87:66: undefined: proxy.Hooks
control-plane\internal\proxy\proxy_test.go:87:87: undefined: proxy.RelayHooked
control-plane\internal\proxy\proxy_test.go:117:43: undefined: proxy.Hooks
control-plane\internal\proxy\proxy_test.go:136:13: undefined: proxy.RelayHooked
control-plane\internal\proxy\proxy_test.go:159:36: undefined: proxy.Hooks
control-plane\internal\proxy\proxy_test.go:187:36: undefined: proxy.Hooks
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/proxy [build failed]
FAIL
```

Six lines, one per **use site** — `undefined:` is reported where an identifier is used, not once per missing symbol, which is why `proxy.Hooks` appears four times. The line and column numbers depend on where the block landed in your file; the identifiers and the count are the part to check. Six is under the compiler's limit, so there is no trailing `too many errors`; if you see that line, something other than these two symbols is also wrong. The separators are backslashes because this is Windows and the paths are relative to the directory `go test` ran in.

- [ ] **Step 3: Add `Hooks` and `RelayHooked` to `internal/proxy/proxy.go`**

`Relay` becomes a one-line delegation. That is the whole point: its signature, its behaviour and its existing test are untouched, so no caller in the phase changes.

Replace the contents of `control-plane/internal/proxy/proxy.go` with:

```go
// Package proxy relays WebSocket traffic between a desktop client and a
// sandbox. It never inspects payloads; the frame protocol is opaque here.
package proxy

import (
	"context"
	"errors"

	"github.com/coder/websocket"
)

// Hooks observe the SIZES of relayed messages. They never receive payload
// bytes, and there is no variant that does: docs/PROGRAM.md forbids deriving
// meaning from agent output absolutely, and a hook holding a []byte is an
// invitation. A nil function is skipped.
//
// The one consumer is the attach handler, which turns OnUpstream into
// registry.Session.LastOutputAt and so into the "idle" status. That is the
// entire mechanism: idle means no output crossed this relay for N seconds, and
// nothing here can tell what the output said.
type Hooks struct {
	OnUpstream func(n int) // sandbox → desktop, i.e. the agent produced output
	OnClient   func(n int) // desktop → sandbox, i.e. the user typed
}

// Relay copies messages between client and upstream until either closes or
// ctx is cancelled. It returns the first error observed.
func Relay(ctx context.Context, client, upstream *websocket.Conn) error {
	return RelayHooked(ctx, client, upstream, Hooks{})
}

// RelayHooked is Relay with observation.
//
// A hook fires after a successful read and before the write, so a message the
// agent produced is counted even when the desktop has already gone away. That
// is deliberate: the timestamp records when the agent last did something, not
// when a socket last succeeded.
func RelayHooked(ctx context.Context, client, upstream *websocket.Conn, h Hooks) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 2)

	copyMessages := func(dst, src *websocket.Conn, observe func(int)) {
		for {
			typ, data, err := src.Read(ctx)
			if err != nil {
				errs <- err
				return
			}
			if observe != nil {
				observe(len(data))
			}
			if err := dst.Write(ctx, typ, data); err != nil {
				errs <- err
				return
			}
		}
	}

	go copyMessages(upstream, client, h.OnClient)
	go copyMessages(client, upstream, h.OnUpstream)

	err := <-errs
	cancel()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
```

`errs` stays buffered at two so the goroutine that loses the race can still send and exit rather than blocking forever on an unread channel. That was already true and is easy to lose in an edit.

- [ ] **Step 4: Run the proxy tests to verify they pass**

```bash
go test ./control-plane/internal/proxy/... -race -count=1
```

Expected — three tests, including the pre-existing one, and the compile-time assertions, which produce no output when they hold:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/proxy	5.5s
```

The duration varies a great deal and is not worth reading: the race detector plus six loopback WebSocket handshakes measured 15.7s and then 5.5s on two consecutive runs of the same tree here. If `TestRelayHookedCountsEachDirection` fails with `OnClient sizes = [4 3], want [3 2]` and `OnUpstream sizes = [3 2], want [4 3]`, the two `go copyMessages(...)` lines have `h.OnClient` and `h.OnUpstream` the wrong way round — the numbers are each other's, and that is the only reading of that failure. If instead the *wants* are what look wrong, the expected literals in step 1 were transcribed from an older draft: they are `[3 2]` and `[4 3]`, and the arithmetic is in the note under that block. That a swap is visible at all is `growEchoServer`'s doing; against a plain echo both directions read `[3 2]` and a swap passes.

- [ ] **Step 5: Write the attach test fixtures**

Four doubles and one fixture, in `control-plane/internal/api/attach_test.go`, `package api` — an **internal** test, because it registers the route through the unexported `a.registerAttach(mux)` and builds an `outputTracker` through the unexported `newOutputTracker`. An external `package api_test` could reach neither. Note the fixture does **not** name `handleAttach`: it goes through `registerAttach`, which is what makes step 8's expected build failure the two errors it is. Steps 6 and 7 append their tests to the same file.

**Every package-level name here begins with `attach`.** Task 11's `sessions_test.go` is in this same package, and the first drafts of both tasks declared a bare `fakeBackend`, which is a redeclaration error the day the second one lands. The group-C contract's ruling 5 fixes the rule that task 11's own plan had already written down: prefix by surface, so no cross-file coordination is needed — which is the point, because cross-file coordination is what failed.

`attachSandboxd` is the double worth explaining. It is not a second implementation of the wire protocol: it writes opaque bytes, because the relay is opaque and a test that decoded frames would be testing `sandboxd/internal/wire` from the wrong package. It does three things the real one does and that this task depends on — it demands `Authorization: Bearer <token>` exactly and answers 401 otherwise, it writes a replay frame the instant a client connects, and it calls `SetReadLimit(backend.MaxMessageBytes)` so a large paste reaching it is accepted rather than closing the socket at 32 KiB (that is site one, standing in for Group A's).

The store is the **real** `registry.FileStore`, wrapped in a counting decorator. That is deliberate: a hand-written in-memory `Store` would be a second implementation of task 7's contract and would owe `registrytest.RunStoreContract`, and the throttle assertions are about how many writes happen, which a decorator counts without reinventing anything. Its directory comes from `os.MkdirTemp` and **not** from `t.TempDir()`, and the comment in the fixture says why at length: it is the difference between a suite that passes and one that fails three runs in five on Windows.

```go
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
```

Continue the same file:

```go
// attachBackend is a backend.Backend that records every Destroy. Attach goes
// through the real backend.Dial, so the bearer header and read-limit site two
// are exercised rather than faked.
type attachBackend struct {
	sb        backend.Sandbox
	getErr    error
	attachErr error

	mu        sync.Mutex
	destroyed []string
}

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

func (attachCatalogue) Projects(context.Context) ([]project.Project, error)        { return nil, nil }
func (attachCatalogue) Branches(context.Context, string) ([]project.Branch, error) { return nil, nil }

func (attachCatalogue) Worktrees(context.Context, string) ([]project.Worktree, error) {
	return nil, nil
}

var _ Enumerator = attachCatalogue{}

// attachCountingStore is registry.Store plus a write counter. It is a DECORATOR
// over the real FileStore, not a second implementation, which is why this file
// does not call registrytest.RunStoreContract. Replace it with a from-scratch
// in-memory Store and that suite becomes mandatory.
type attachCountingStore struct {
	registry.Store
	updates atomic.Int64
}

func (s *attachCountingStore) Update(id string, mutate func(*registry.Session) error) (registry.Session, error) {
	s.updates.Add(1)
	return s.Store.Update(id, mutate)
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
	})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}

	// A local mux, not the handler New returns. Task 9's middleware answers
	// forbidden_host BEFORE forbidden_origin and §3.16 orders this handler's
	// own first two checks the other way round, so the tests that pin the
	// handler's order cannot go through New. registerAttach is what mounts the
	// pattern, so the pattern is pinned here; that New calls it is pinned by
	// TestNewActuallyRoutesTheAttachRoute in step 6.
	mux := http.NewServeMux()
	a.registerAttach(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &attachFixture{srv: srv, api: a, store: store, be: be, sandboxd: sandboxd, id: sess.ID}
}
```

And the two request helpers:

```go
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
```

Four of the nine `Deps` fields are here only because `New` refuses to start without them, and none of them is reachable from this surface: no attach test calls a handler that reads `Projects`, `Worktrees`, `Flags` or `BackendReady`. **Do not make `New` tolerant of a nil one instead** — that hides a real misconfiguration at boot, which is why task 10 chose a returned error over a panic and over a default.

- [ ] **Step 6: Write the failing tests for the five pre-upgrade checks**

Order is the substance here, not decoration. Contract §3.16 fixes it — origin, host, session, sandbox, upgrade — and each check must be provable *in the presence of a later failure*, or a handler that runs them in the wrong order still passes. So the origin test uses a session id that does not exist: if the session lookup ran first it would answer 404 and the test would fail. The same trick runs down the list.

All five answer with an HTTP status and a JSON `httpx.Problem`, never a WebSocket close, because a close frame reaches the desktop as "connection failed" with nothing to render. Every code comes from task 9's `Code*` constants rather than a literal, on both sides of the assertion: the desktop switches on these strings, and a typo in one handler is a code the cockpit can never match and no compiler would have flagged.

**Eight test functions cover five checks, and the eighth is not a check.** One each for origin, host and the upgrade; two for the session, which has an unknown branch and an exited branch; two for the sandbox, which has the backend's verdicts and the `starting` case that must be allowed through. `TestNewActuallyRoutesTheAttachRoute` is the eighth, the only test in this file that drives the handler `New` returns, and it is the group-C contract's ruling 2: without it, nothing here fails when `a.registerAttach(guarded)` is missing from `New`, because every other test uses a mux the fixture registered onto itself.

Append to `control-plane/internal/api/attach_test.go`:

```go
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
```

```go
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
// mux, so not one of them would notice if a.registerAttach(guarded) were
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
```

- [ ] **Step 7: Write the failing tests for the relay half**

Four tests, and the first is the one this task exists for. **262145 bytes is not a round number chosen for drama** — it is sandboxd's replay frame at its documented ceiling, 256 KiB of scrollback plus one kind byte, and it is eight times `coder/websocket`'s default read limit. The test sends it in both directions: downstream proves site two (`backend.Dial`) and the client's own limit, upstream proves site three, the `Accept` in this handler. Miss site three and a user's first big paste kills the terminal.

The throttle is unit-tested on `outputTracker` with an injected clock rather than through the handler, because a handler-level throttle test would have to burn real seconds and would still be timing-dependent. The handler test asserts only what it can assert deterministically: the hook runs before the relay's write, so by the time the client has read a pushed frame the registry write has already returned.

Append to `control-plane/internal/api/attach_test.go`:

```go
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
```

```go
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

	// And the sandbox is still there to come back to.
	again := f.dial(ctx, t)
	if err := again.Write(ctx, websocket.MessageBinary, []byte{0x01, 'y'}); err != nil {
		t.Fatalf("reattach Write: %v", err)
	}
	if _, _, err := again.Read(ctx); err != nil {
		t.Fatalf("reattach Read: %v", err)
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
```

- [ ] **Step 8: Run the attach tests to verify they fail**

```bash
go test ./control-plane/internal/api/... -race -count=1
```

Expected: a build failure of `package api` compiled for its own test binary, and **exactly two errors**, because the test file references exactly two things that do not exist yet. There is **no error in `api.go`**: the `a.registerAttach(guarded)` line goes in at step 9, in the same step as the method it names, which is the group-C contract's ruling 2 — add it here instead and `package api` stops compiling, so no step between the two can have a green suite.

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [github.com/kpmg-au-emu/panacea-app/control-plane/internal/api.test]
control-plane\internal\api\attach_test.go:299:4: a.registerAttach undefined (type *API has no field or method registerAttach)
control-plane\internal\api\attach_test.go:694:8: undefined: newOutputTracker
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [build failed]
FAIL
```

Two shapes, and the difference matters when you are reading these. A missing **package-level identifier** is the bare `undefined: newOutputTracker`. A missing **method** never prints that form: it is `x.M undefined (type T has no field or method M)`, which is why `registerAttach` reads the way it does — and note that the type it names is `*API`, because that is what task 10 declares and there is no `Server` in this package. Line and column numbers depend on how you transcribed the file; the two identifiers, the two shapes and the count are the check. `handleAttach` is **not** in the list: the fixture reaches it through `a.registerAttach(mux)`, and once that method is undefined the compiler has nothing to resolve the route line against.

`tr.observe` and `tr.flushFinal` produce no errors of their own. `tr` has no valid type once `newOutputTracker` is undefined, and the type checker suppresses the cascade rather than reporting three more. If you do see errors for them, `newOutputTracker` exists and returns the wrong thing.

- [ ] **Step 9: Add the route line to `New`, then write `internal/api/attach.go`**

**The line in `New` goes in first, and it goes in *this* step.** Task 10 reserved it (`task-10.md:34` and the comment in `New`) and the group-C contract's ruling 2 requires the call and the method it names to land together: a `New` that calls a method no file declares leaves `package api` uncompilable for every step in between, which is the defect task 11 shipped in its first draft. So edit `control-plane/internal/api/api.go`, one line, immediately below the two that are already there:

```go
	guarded := http.NewServeMux()
	a.registerCatalog(guarded)  // task 10
	a.registerSessions(guarded) // task 11
	a.registerAttach(guarded)   // task 12 — this line
```

**Do not solve a build error by deleting this line instead.** Everything else in this file drives the fixture's own mux, so deleting it ships an attach route the product never serves behind a fully green suite. `TestNewActuallyRoutesTheAttachRoute` is the one test that fails if it is gone; before committing, run

```bash
grep -cE '^[[:space:]]*a\.registerAttach\(guarded\)' control-plane/internal/api/api.go   # exactly 1
```

**The anchor is the check.** Task 10's reserving comment (`task-10.md:1073-1081`) carries the literal string `a.registerAttach(guarded)` in prose, so the unanchored `grep -c "a.registerAttach(guarded)"` prints `1` before this step and `2` after it — measured — and therefore cannot distinguish the two states at all; grepping the bare name `registerAttach` is worse for the same reason. `^[[:space:]]*a\.` matches only a line whose first non-space content is the call, which a `//` comment can never be: `0` before, `1` after.

Then create `control-plane/internal/api/attach.go`:

```go
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/proxy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// lastOutputFlushInterval bounds how often a relay writes LastOutputAt through
// to the registry. Contract §5.4: at most once every two seconds. The registry
// is one JSON document rewritten in full on every mutation, and an agent
// emitting a few hundred frames a second would otherwise rewrite it a few
// hundred times a second.
const lastOutputFlushInterval = 2 * time.Second

// registerAttach mounts the one route this file serves, on the same guarded
// mux task 10's registerCatalog and task 11's registerSessions use. api.New
// calls it, on the line task 10 reserved.
func (a *API) registerAttach(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sessions/{id}/attach", a.handleAttach)
}

// handleAttach upgrades GET /v1/sessions/{id}/attach and relays the sandbox's
// framed stdio to the desktop.
//
// THE ORDER OF THE CHECKS IS PART OF THE CONTRACT (§3.16). All five run BEFORE
// the upgrade, so a refusal is an HTTP status with a JSON body the desktop can
// render, rather than a close code a browser reports as "connection failed".
//
// AND THIS HANDLER DESTROYS NOTHING. Not on a dropped socket, not on a failed
// upstream dial, not on an off-loopback sandbox. A closing laptop lid must not
// kill a running agent. Destruction belongs to DELETE (task 11) and to the
// reaper (task 13), both of which are deliberate and observable.
func (a *API) handleAttach(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// a.deps.Log is never nil: New defaults it to slog.Default().
	log := a.deps.Log.With("route", "attach", "session", id)

	// 1. Origin. Checked here, with the full origin string, rather than by
	//    coder/websocket's OriginPatterns, which matches the host only and
	//    allows a missing Origin unconditionally (accept.go:208-229). Task 9's
	//    middleware checks it too, in front of the guarded mux; this is not
	//    redundant, because a WebSocket upgrade must not depend on a middleware
	//    that a future route table could mount it without.
	origin := r.Header.Get("Origin")
	if !a.deps.Origin.Allow(origin) {
		log.Warn("attach refused: origin not allowed", "origin", origin)
		httpx.WriteProblem(w, http.StatusForbidden, httpx.CodeForbiddenOrigin,
			"This origin may not reach the control plane.",
			map[string]string{"origin": origin})
		return
	}

	// 2. Host. The DNS-rebinding defence: a page on any domain can point a
	//    name at 127.0.0.1, and its requests then carry that name as Host.
	if !httpx.LoopbackHost(r.Host) {
		log.Warn("attach refused: non-loopback Host", "host", r.Host)
		httpx.WriteProblem(w, http.StatusForbidden, httpx.CodeForbiddenHost,
			"The control plane answers only to a loopback Host header.",
			map[string]string{"host": r.Host})
		return
	}

	// 3. The session.
	sess, err := a.deps.Store.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "read the session registry", err)
		return
	}

	// 4. The sandbox, as the record sees it and as the backend sees it.
	if sess.Status == registry.StatusExited || sess.SandboxID == "" {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning,
			"This session has no running sandbox.",
			map[string]string{"id": id, "status": string(sess.Status)})
		return
	}
	if !a.attachSandboxReady(w, r, log, id, sess.SandboxID) {
		return
	}

	// 5. Is this even an upgrade request?
	if !isWebSocketUpgrade(r) {
		w.Header().Set("Upgrade", "websocket")
		httpx.WriteProblem(w, http.StatusUpgradeRequired, httpx.CodeUpgradeRequired,
			"This endpoint is a WebSocket upgrade.", nil)
		return
	}

	// 6. Upgrade, then SITE THREE OF THREE of SetReadLimit. See
	//    backend.MaxMessageBytes: the library's default is 32768 and a replay
	//    frame is 262145. Never a literal here.
	client, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Safe: check 1 above already compared the full Origin string, which
		// is stricter than anything this option can express.
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Warn("attach failed: upgrade", "err", err)
		return
	}
	defer client.CloseNow()
	client.SetReadLimit(backend.MaxMessageBytes)

	a.relayAttached(r, log, client, sess)
}
```

Continue the same file:

```go
// attachSandboxReady is check 4's second half. It reports whether the relay
// may proceed, and writes the Problem itself when it may not.
func (a *API) attachSandboxReady(w http.ResponseWriter, r *http.Request, log *slog.Logger, id, sandboxID string) bool {
	sb, err := a.deps.Backend.Get(r.Context(), sandboxID)
	switch {
	case err == nil:
	case errors.Is(err, backend.ErrNonLoopbackPublish):
		// 500, loudly, and DO NOT destroy it here. Destroying from a
		// read-only attach would race the reaper, which the group-C
		// contract's ruling 10 already requires to destroy this sandbox and
		// log it distinctly. Contract §2.2's "and the sandbox is destroyed"
		// was written for Create, where the caller owns what it just made.
		//
		// The code is the sixteenth, non_loopback_publish, and task 9 declares
		// it (ruling 7). Mapping this to sandbox_not_running would be a lie:
		// the sandbox IS running, which is exactly the problem.
		log.Error("attach refused: sandbox port is not bound to 127.0.0.1",
			"sandbox", sandboxID, "err", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeNonLoopbackPublish,
			"The sandbox port is not bound to 127.0.0.1, so the control plane will not attach to it.",
			map[string]string{"id": id})
		return false
	case errors.Is(err, backend.ErrUnavailable):
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
			"The sandbox backend is not reachable.", nil)
		return false
	case errors.Is(err, backend.ErrNotFound), errors.Is(err, backend.ErrNotRunning):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning,
			"This session has no running sandbox.", map[string]string{"id": id})
		return false
	default:
		a.internal(w, r, "look the sandbox up in the backend", err)
		return false
	}

	// StateStarting is allowed through deliberately: create returns 201 before
	// the sandbox answers /healthz, and the cockpit attaches immediately.
	if sb.Endpoint == "" || sb.State == backend.StateStopped || sb.State == backend.StateGone {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning,
			"This session has no running sandbox.",
			map[string]string{"id": id, "state": string(sb.State)})
		return false
	}
	return true
}

// relayAttached owns everything after the upgrade. Its one job beyond copying
// bytes is to leave the world exactly as it found it.
func (a *API) relayAttached(r *http.Request, log *slog.Logger, client *websocket.Conn, sess registry.Session) {
	// 7. The upstream dial, AFTER the upgrade because §3.16 orders it so. The
	//    reason is sandboxd's replay frame: a dial that succeeds and is then
	//    discarded because the client never upgraded consumes a scrollback
	//    replay nobody ever reads. The cost is that a failure here can only be
	//    a close code, so make it a specific one.
	upstream, err := a.deps.Backend.Attach(r.Context(), sess.SandboxID, sess.AttachToken)
	if err != nil {
		code, reason := attachCloseStatus(err)
		log.Warn("attach failed: upstream dial", "sandbox", sess.SandboxID, "err", err)
		_ = client.Close(code, reason)
		return
	}
	defer upstream.CloseNow()

	a.addAttachCount(log, sess.ID, +1)
	defer a.addAttachCount(log, sess.ID, -1)

	tracker := newOutputTracker(a.deps.Store, sess.ID, lastOutputFlushInterval, time.Now, log)
	defer tracker.flushFinal()

	// 8. OnClient is deliberately nil. The user typing is not output, and
	//    idle is defined on output only (§5.4).
	if err := proxy.RelayHooked(r.Context(), client, upstream, proxy.Hooks{OnUpstream: tracker.observe}); err != nil {
		// A dropped desktop is the normal case, not an incident.
		log.Info("attach relay ended", "err", err)
	}

	_ = client.Close(websocket.StatusNormalClosure, "")
	_ = upstream.Close(websocket.StatusNormalClosure, "")
	// AND NOTHING IS DESTROYED. See the handler's doc comment.
}

// addAttachCount moves the displayed count of connected clients. It is a
// display value and never an input to status: a session with no client
// attached is still running.
func (a *API) addAttachCount(log *slog.Logger, id string, delta int) {
	_, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.AttachCount += delta
		if sess.AttachCount < 0 {
			// Below zero means a decrement outlived its increment, which a
			// control-plane restart can produce. Clamp rather than persist a
			// negative the cockpit would render.
			sess.AttachCount = 0
		}
		return nil
	})
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		log.Warn("could not record attachCount", "delta", delta, "err", err)
	}
}
```

And the rest of the file:

```go
// outputTracker turns "a message crossed the relay" into
// registry.Session.LastOutputAt, at most once every `every`.
//
// It receives sizes, never bytes — see proxy.Hooks. LastOutputAt is the only
// input to the idle derivation in task 13, and idle means exactly "no output
// for N seconds". It says nothing about what the agent is doing or waiting for.
type outputTracker struct {
	store registry.Store
	id    string
	every time.Duration
	now   func() time.Time
	log   *slog.Logger

	mu      sync.Mutex
	last    time.Time // most recent observation
	flushed time.Time // when the last registry write happened
	dirty   bool      // last has moved since the last write
}

func newOutputTracker(store registry.Store, id string, every time.Duration, now func() time.Time, log *slog.Logger) *outputTracker {
	return &outputTracker{store: store, id: id, every: every, now: now, log: log}
}

// observe is proxy.Hooks.OnUpstream. The size argument is ignored on purpose:
// only the fact and the time of output matter, and a tracker that cared how
// much was written would be one step from caring what it said.
//
// The registry write happens inline, on the relay's read goroutine. That
// blocks the relay for one document rewrite, which the throttle bounds to once
// every two seconds; doing it asynchronously would let an older timestamp land
// after a newer one.
func (t *outputTracker) observe(int) {
	t.mu.Lock()
	now := t.now()
	t.last = now
	t.dirty = true
	if !t.flushed.IsZero() && now.Sub(t.flushed) < t.every {
		t.mu.Unlock()
		return
	}
	t.flushed = now
	t.dirty = false
	t.mu.Unlock()

	t.write(now)
}

// flushFinal writes whatever the throttle held back. Contract §5.4's
// "unconditionally" means "not subject to the throttle", not "write even with
// nothing pending": a flush with nothing to say is a no-op, so calling it
// twice costs nothing and the deferred call is always safe.
func (t *outputTracker) flushFinal() {
	t.mu.Lock()
	dirty, stamp := t.dirty, t.last
	if dirty {
		t.dirty = false
		t.flushed = stamp
	}
	t.mu.Unlock()

	if dirty {
		t.write(stamp)
	}
}

func (t *outputTracker) write(stamp time.Time) {
	_, err := t.store.Update(t.id, func(sess *registry.Session) error {
		// Never move the timestamp backwards. Two clients can be attached to
		// one session; the later observation wins.
		if stamp.After(sess.LastOutputAt) {
			sess.LastOutputAt = stamp
		}
		return nil
	})
	// A deleted session during a relay is ordinary — the user pressed delete
	// while watching — so it is not worth a log line.
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		t.log.Warn("could not record lastOutputAt", "err", err)
	}
}

// attachCloseStatus maps a backend failure to a close frame. Reasons are kept
// short deliberately: RFC 6455 caps the close payload at 125 bytes, of which
// two are the code, so a reason over 123 bytes makes Close fail and the socket
// dies with no diagnosis at all.
func attachCloseStatus(err error) (websocket.StatusCode, string) {
	switch {
	case errors.Is(err, backend.ErrUnauthorized):
		return websocket.StatusPolicyViolation, "sandbox rejected the attach token"
	case errors.Is(err, backend.ErrNonLoopbackPublish):
		return websocket.StatusPolicyViolation, "sandbox port is not bound to 127.0.0.1"
	case errors.Is(err, backend.ErrNotFound),
		errors.Is(err, backend.ErrNotRunning),
		errors.Is(err, backend.ErrUnavailable):
		return websocket.StatusTryAgainLater, "sandbox is not accepting connections"
	default:
		return websocket.StatusInternalError, "attach failed"
	}
}

// isWebSocketUpgrade answers check 5 without attempting the upgrade, so the
// refusal is a 426 with a body rather than a failed handshake.
//
// Connection is a comma-separated token list — "keep-alive, Upgrade" is legal
// and common through a proxy — so it cannot be compared whole.
func isWebSocketUpgrade(r *http.Request) bool {
	return attachHeaderHasToken(r.Header, "Connection", "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// attachHeaderHasToken is prefixed for the same reason every other name in
// this task is: four tasks write into package api, and a bare headerHasToken
// is the collision ruling 5 exists to prevent.
func attachHeaderHasToken(h http.Header, key, want string) bool {
	for _, v := range h.Values(key) {
		for _, tok := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), want) {
				return true
			}
		}
	}
	return false
}
```

Nothing here is a guess about task 10's names: the group-C contract's ruling 1 fixes them and task 10's `api.go` is the authority. The receiver is `*API` with the receiver name `a`, the field holding `Deps` is `a.deps`, the members used here are `a.deps.Origin` (singular), `a.deps.Store`, `a.deps.Backend` and `a.deps.Log`, and both 500s go through task 10's `a.internal` so one place decides how much a caller is told. `a.deps.Log` is never nil — `New` defaults it to `slog.Default()` — which is why this file has no logger accessor of its own.

- [ ] **Step 10: Run the attach tests to verify they pass**

```bash
go test ./control-plane/internal/api/... -race -count=1 -v -run 'TestAttach|TestADroppedSocket|TestOutputTracker|TestNewActuallyRoutesTheAttachRoute'
```

Expected — **twelve top-level tests and five sub-tests, so seventeen `=== RUN` lines**. `-v` is worth it once, because the sub-test names are the checklist of contract §3.16. The filter is spelled out rather than `'Attach|OutputTracker'` for two reasons: the short form misses `TestADroppedSocketDestroysNothing` entirely, and by the time this task runs, task 11's `sessions_test.go` is in this package and its `TestSessionJSONBuildsTheAttachPathAndTheIsolation` matches a bare `Attach`. Each alternative below is anchored at `Test…` so it selects this task's tests and nothing else — checked against every test name tasks 10 and 11 declare:

```
=== RUN   TestAttachRejectsUnknownOriginBeforeAnythingElse
--- PASS: TestAttachRejectsUnknownOriginBeforeAnythingElse (0.03s)
=== RUN   TestAttachRejectsNonLoopbackHostBeforeTheSessionLookup
--- PASS: TestAttachRejectsNonLoopbackHostBeforeTheSessionLookup (0.03s)
=== RUN   TestAttachUnknownSessionIs404
--- PASS: TestAttachUnknownSessionIs404 (0.03s)
=== RUN   TestAttachExitedSessionIs409
--- PASS: TestAttachExitedSessionIs409 (0.03s)
=== RUN   TestAttachBackendVerdictsBeforeTheUpgrade
=== RUN   TestAttachBackendVerdictsBeforeTheUpgrade/sandbox_gone
=== RUN   TestAttachBackendVerdictsBeforeTheUpgrade/backend_down
=== RUN   TestAttachBackendVerdictsBeforeTheUpgrade/published_off_loopback
=== RUN   TestAttachBackendVerdictsBeforeTheUpgrade/no_endpoint_yet
=== RUN   TestAttachBackendVerdictsBeforeTheUpgrade/stopped
--- PASS: TestAttachBackendVerdictsBeforeTheUpgrade (0.14s)
    --- PASS: TestAttachBackendVerdictsBeforeTheUpgrade/sandbox_gone (0.02s)
    --- PASS: TestAttachBackendVerdictsBeforeTheUpgrade/backend_down (0.03s)
    --- PASS: TestAttachBackendVerdictsBeforeTheUpgrade/published_off_loopback (0.03s)
    --- PASS: TestAttachBackendVerdictsBeforeTheUpgrade/no_endpoint_yet (0.03s)
    --- PASS: TestAttachBackendVerdictsBeforeTheUpgrade/stopped (0.03s)
=== RUN   TestAttachStartingSandboxIsAllowedThrough
--- PASS: TestAttachStartingSandboxIsAllowedThrough (0.09s)
=== RUN   TestAttachWithoutUpgradeHeadersIs426
--- PASS: TestAttachWithoutUpgradeHeadersIs426 (0.03s)
=== RUN   TestNewActuallyRoutesTheAttachRoute
--- PASS: TestNewActuallyRoutesTheAttachRoute (0.04s)
=== RUN   TestAttachCarriesAMessageEightTimesTheDefaultReadLimit
--- PASS: TestAttachCarriesAMessageEightTimesTheDefaultReadLimit (0.22s)
=== RUN   TestAttachRecordsOutputAndBalancesTheCounter
--- PASS: TestAttachRecordsOutputAndBalancesTheCounter (0.17s)
=== RUN   TestADroppedSocketDestroysNothing
--- PASS: TestADroppedSocketDestroysNothing (0.13s)
=== RUN   TestOutputTrackerThrottlesRegistryWrites
--- PASS: TestOutputTrackerThrottlesRegistryWrites (0.10s)
PASS
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api	2.716s
```

That transcript is real, taken from a tree assembled out of this plan and run with `-race` on Windows. Two things about its shape rather than its content: `go test` prints a table test's sub-test `--- PASS:` lines **indented, as a group, after** the parent's, not interleaved with the `=== RUN` lines; and the durations vary run to run — the same suite measured between 2.4s and 4.2s over five consecutive runs here. All five were green, which is the point of the `os.MkdirTemp` in the fixture: with `t.TempDir()` this suite failed 3 runs in 5.

Two failures worth recognising on sight.

**`reading the echoed paste: failed to get reader: received close frame: status = StatusMessageTooBig and reason = "read limited at 32769 bytes"`** is a missing `SetReadLimit`. **32769 and not 32768**: `coder/websocket`'s `SetReadLimit` stores `n+1` so the fin frame fits (`read.go:97-104`), and the default 32768 is reported the same way, so the number is always one more than the limit and never tells you which site is missing. The side that failed does. Measured by deleting `client.SetReadLimit(backend.MaxMessageBytes)` from `handleAttach` and re-running: the failure lands on the **echoed paste**, which is site three, the `Accept` here. The same string on **`reading the replay frame`** is the other direction — site two (`backend.Dial`, task 4) or the test client's own limit in `dial`.

**`Destroy called 1 times after a dropped socket; it must be called zero times`** means a cleanup path in `relayAttached` destroys the sandbox. There must be none. If it arrived through a helper shared with task 11, that helper is wrong for this file: `DELETE` destroys, attach does not.

- [ ] **Step 11: Run the whole workspace, vet and gofmt**

```bash
gofmt -l control-plane/internal/api control-plane/internal/proxy
go vet ./control-plane/internal/api/... ./control-plane/internal/proxy/...
go test ./sandboxd/... ./control-plane/... -race -count=1
```

`gofmt -l` prints nothing when both directories are formatted; any path it prints is a file to run `gofmt -w` over. `go vet` prints nothing on success.

The third command is the phase's only correct full-test invocation. `go test ./...` at the workspace root is invalid and fails with `directory prefix . does not contain modules listed in go.work or their selected dependencies` — this repository is a `go.work` workspace, so the two module roots are named explicitly. Expect `ok` for every package with tests and `[no test files]` for the rest, including `registrytest`.

This step is also the one that proves the claim in the title of Step 3: **no caller of `Relay` changed.** If anything outside `internal/proxy` fails to build here, a signature moved that should not have.

- [ ] **Step 12: Commit**

`api.go` is in the list because step 9 added `a.registerAttach(guarded)` to `New`. Confirm it before committing — `grep -cE '^[[:space:]]*a\.registerAttach\(guarded\)' control-plane/internal/api/api.go` must print exactly `1`. Anchored, for the reason step 9 gives: task 10's reserving comment carries the same string, so every unanchored form counts it too and prints `2`.

```bash
git add control-plane/internal/api/api.go \
        control-plane/internal/api/attach.go \
        control-plane/internal/api/attach_test.go \
        control-plane/internal/proxy/proxy.go \
        control-plane/internal/proxy/proxy_test.go
git commit -m "feat(api): attach relay with pre-upgrade checks and size-only counters

The five checks in contract 3.16 run before the WebSocket upgrade so a refused
attach is an HTTP status with a body the cockpit can render, rather than a
close code a browser reports as connection failed. A sandbox the backend
cannot vouch for is refused the same way, including one published off
loopback, which is a 500 and a log line and is left for the reaper to destroy.

SetReadLimit here is the third of the three sites backend.MaxMessageBytes
names. The library caps one inbound message at 32768 bytes and sandboxd's
replay frame reaches 262145, so a missing site breaks reconnection to a busy
agent while every light test passes.

Relay keeps its signature and becomes a call to RelayHooked, which adds hooks
that receive message LENGTHS and never payloads. That is the whole of how idle
is derived: no output crossed the relay for N seconds. The timestamp is
flushed to the registry at most once every two seconds, because the registry
is one document rewritten in full.

A dropped socket destroys nothing. Closing a laptop lid must not kill a
running agent, so nothing on this path calls Destroy; deletion belongs to
DELETE and to the reaper, which are deliberate and observable.

api.go gains one line, a.registerAttach(guarded), on the line task 10 reserved
for it. One test drives the route through the handler New returns, because
every other test here uses a mux the fixture built and none of them would
notice an attach route that exists only in the tests.

A sandbox published off loopback answers 500 non_loopback_publish, the
sixteenth error code, declared by task 9. Reusing sandbox_not_running would
tell the cockpit something false: such a sandbox IS running.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

