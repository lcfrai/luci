# Phase 2: Sandbox Backend Interface + The Cockpit

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The whole product runs on a Windows laptop with Docker Desktop and zero cloud spend. A `Backend` interface fronts two real implementations (`docker`, `kata`); an HTTP control plane creates sandboxes, enforces origin, and relays WebSockets to them; sessions survive both a detach and an app restart; and the Tauri window is the approved cockpit from `prototype/app.html` — rail, sub-rail, Columns and Canvas, many sessions per worktree, split-button session creation, full session lifecycle, command palette, keyboard map.

**Architecture:** Four processes, one new seam.

```
Tauri window (React)                  control-plane (cmd/api)              sandbox container
  ┌──────────────────┐   ws /v1/sessions/{id}/attach   ┌───────────┐   ws /attach   ┌──────────────┐
  │ cockpit + xterm  │ ─────────────────────────────►  │  relay    │ ────────────►  │ sandboxd hub │
  │ zustand store    │ ◄─────────────────────────────  │  origin   │ ◄────────────  │  ring buffer │
  └──────────────────┘        REST /v1/sessions        │  registry │   bearer token │  one PTY     │
                                                        │  Backend  │                └──────────────┘
                                                        └─────┬─────┘
                                                   docker ◄───┴───► kata (dormant until Phase 8)
```

The governing constraint is unchanged and absolute: **agent CLIs are never parsed, wrapped, reimplemented or interpreted.** The only transformation between the PTY and xterm.js is framing. Nothing in this phase reads agent output to decide anything.

**Tech Stack:** Go 1.23 (`github.com/coder/websocket` v1.8.12, `github.com/docker/docker` v28.3.3+incompatible, `github.com/docker/go-connections` v0.5.0, `k8s.io/client-go` v0.31.1), Rust + Tauri 2, React 19 + TypeScript 6 + Vite 8, `zustand` 5, `@xterm/xterm` 6, Vitest 5 + `@testing-library/react` 16 + jsdom 30.

**Spec:** `docs/PROGRAM.md` (Phase 2), `docs/FUNCTIONAL-SPEC.md`, `docs/DESIGN-SYSTEM.md`, `docs/AGENTIC-UX.md`, `prototype/app.html`.

---

## Task groups

This phase is large. It is split into five groups that can be reviewed independently. Groups A→C are Half A (the backend); group D is Half B (the cockpit); group E proves it.

| Group | Tasks | What it delivers | Depends on |
|---|---|---|---|
| **A — Durable sessions in sandboxd** | 1–3 | A PTY that outlives a detach, with scrollback replay on re-attach | — |
| **B — The sandbox backend interface** | 4–8 | `Backend` with `docker` and `kata` implementations, the session registry, the harness catalogue, and read-only project/branch/worktree enumeration | A (task 3 contract) |
| **C — The control-plane API** | 9–13 | `cmd/api`: the error body, origin enforcement and the CORS grant, the read-only surface, session CRUD, the attach relay, config, the reaper and restart recovery | B |
| **D — The cockpit** | 14–24 | The approved UI made real against the Group C API. **D1 (14–19)** is the substrate — test infrastructure, tokens, domain model, client, store, terminal binding — and puts nothing on screen until 18. **D2 (20–24)** is the surfaces, the lifecycle verbs, the palette and the proof. | C |
| **E — Verification and seams** | 25–26 | End-to-end proof on Docker by hand; named seams for Phases 3–5 and 8 | D |

---

## Global Constraints

- Go module path prefix: `github.com/kpmg-au-emu/panacea-app`.
- **`go test ./...` is invalid at this workspace root.** Always `go test ./sandboxd/... ./control-plane/... -race -count=1`. The root is not a module, so `./...` resolves to nothing and errors with `directory prefix . does not contain modules listed in go.work`.
- The `go` directive must stay `go 1.23` in `go.work`, `sandboxd/go.mod` and `control-plane/go.mod`. CI pins 1.23; a higher directive makes a network-restricted agent attempt a toolchain download and fail. Do not let `go get` bump it — check `git diff` on the go.mod files before every commit.
- `sandboxd/vendor/` is committed and the sandbox image builds with `GOFLAGS=-mod=vendor`, because the corporate proxy intercepts TLS to `proxy.golang.org`. **Any new sandboxd dependency must be re-vendored (`cd sandboxd && go mod vendor`) and committed, or the image build breaks.** Group A adds no new sandboxd dependency, by design.
- `control-plane` is **not** vendored and does not need to be — it is never built inside a container in this phase. Its module downloads work from the host.
- Agent CLIs are never modified, wrapped or parsed. No feature in this phase may read PTY bytes to derive meaning.
- No secrets in source. Configuration is read from `PANACEA_APP_*` environment variables.
- The control plane listens on **loopback only** by default (`127.0.0.1:8730`). Sandbox ports are published to **`127.0.0.1` only**. Binding either to `0.0.0.0` exposes a shell to the LAN.
- Shell is Windows. Use the **PowerShell** tool for Docker — Git Bash mangles container paths. Beware the pipe exit-code trap: `cmd | head` reports `head`'s status, not `cmd`'s.
- Commit messages end with:
  `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`

### Deliberate refinements of the brief, with reasons

Three points where this plan does **not** do the obvious thing. Each is a correctness fix, not a preference.

1. **Disconnect does not destroy.** The brief says cmd/api should "destroy on disconnect". It must not. `docs/FUNCTIONAL-SPEC.md` requires that finished sessions stay listed and readable and that `Resume` restores context; `claude --resume` reads state from `~/.claude` inside the sandbox. Destroying on disconnect deletes the transcript the product is built around. **Destroy happens on explicit kill (`DELETE /v1/sessions/{id}`), on idle TTL, or on reap of an orphan.** A dropped WebSocket ends the relay only.
2. **`Backend.Attach` returns `*websocket.Conn`, not `io.ReadWriteCloser`.** `docs/PROGRAM.md` sketches the latter. A byte stream destroys message boundaries, and the wire protocol is message-framed — a 32 KiB stdout frame split across two reads would be decoded as two frames, the second with a garbage kind byte. `proxy.Relay` already takes `*websocket.Conn`.
3. **Phase 2 ships only mechanically-derived session status.** `docs/AGENTIC-UX.md` §4 rule 8 specifies seven statuses (PLANNING, EXECUTING, BLOCKED-ON-YOU, …). Five of those cannot be derived without reading agent output, which is forbidden. Phase 2 therefore ships the four facts it can actually observe — `starting`, `running`, `idle` (attached, no output for N seconds), `exited` — and names the gap. `BLOCKED-ON-YOU` requires the agent-runtime signal that `docs/AGENTIC-UX.md` §3 calls a "hard runtime dependency, not a UI task". It is not invented here.


### Cross-group decisions, adjudicated

Groups B–E are written from two binding contracts (the Go-side contract for tasks 4–13, the
TypeScript/UI contract for tasks 14–24). The UI contract escalated two disagreements with the Go
contract. Both are resolved **in favour of changing the Go side**, and the resolution is recorded
here rather than left implicit in one task, because in each case two groups must spell the same
thing the same way.

1. **The control plane must answer CORS. Blocking — without it the cockpit cannot make a single
   REST call.** The Go contract specifies an `OriginPolicy` that *rejects* a bad origin; it never
   specifies that the server *grants* a good one, and those are different mechanisms. A Tauri
   webview is a browser, and every build is cross-origin against `http://127.0.0.1:8730`:
   `http://localhost:1420` in dev (different host *and* port), `http://tauri.localhost` packaged
   on Windows, `tauri://localhost` packaged on macOS and Linux (different scheme). So `GET
   /v1/machines` fails at the response with "No 'Access-Control-Allow-Origin' header is present",
   and every `POST`/`PATCH`/`DELETE` carrying `Content-Type: application/json` fails one step
   earlier at a preflight `OPTIONS` the mux does not route. **WebSockets are exempt** — no
   preflight, no ACAO requirement — which is exactly why Phase 1 worked and why this would not
   surface until the first REST call in task 24.

   **Decision: `httpx.OriginPolicy.Middleware` (task 9) grants as well as rejects.** On every
   request whose `Origin` is present and allowed it sets `Access-Control-Allow-Origin` to that
   origin **echoed verbatim, never `*`**, plus `Vary: Origin`. On `OPTIONS` with an allowed
   origin it answers **204 before the mux** with `Access-Control-Allow-Methods: GET, POST, PATCH,
   DELETE, OPTIONS`, `Access-Control-Allow-Headers: Content-Type` and
   `Access-Control-Max-Age: 600`. No `Access-Control-Allow-Credentials` — no cookies are used.
   Echoing rather than wildcarding keeps the allowlist as the single decision point; `Vary:
   Origin` stops an interposed cache serving one origin's response to another. The desktop
   therefore keeps a plain injected `fetch` and stays unit-testable; the rejected alternative was
   `@tauri-apps/plugin-http`, which costs an npm dependency, a Rust crate, a `tauri.conf.json`
   plugin entry and a capability grant, makes the API client untestable outside a Tauri window,
   and buys no security — the Rust side would allowlist the same origin anyway.

2. **A worktree needs a settable `ready` flag, or known defect 9 is not fixed.**
   `docs/FUNCTIONAL-SPEC.md` lists "nothing ever sets `ready`, so user-created worktrees can never
   merge" among the ten defects **being fixed in this pass**, and the Go contract's worktree object
   has no `ready` field and no worktree mutation endpoint. Phase 2 could only delete the verb,
   which moves the defect rather than fixing it.

   **Decision: the flag ships in Phase 2.** `GET /v1/projects/{projectId}/worktrees` gains
   `"ready": false` on every worktree (task 10); `PATCH /v1/projects/{projectId}/worktrees/{worktreeId}`
   accepts `{"ready": true}` and returns 200 with the worktree object, 400 `bad_request` for any
   other field, 404 `worktree_not_found` for an unknown id (task 10). It persists as one more map
   in the existing registry document — `"worktreeFlags": {"w-4a77b1c9": {"ready": true}}` — written
   through the same `Store.Update` and the same atomic replace (task 7). **It must land in task 7
   rather than later**: added now it is an additive change inside `schemaVersion: 1`; added after
   the registry ships it is a schema bump and a migration. It is the only new persisted state
   Phase 2 needs. On the desktop, `Worktree.ready` stays optional in the TypeScript model and the
   "Mark ready" verb renders only when the server actually reports the field (task 22), so the
   cockpit degrades honestly if the server half is ever reverted.

Four further items the UI contract raised need **no Phase 2 change** and are recorded so nobody
re-opens them mid-task: the desktop→control-plane attach hop deliberately carries no secret (the
boundary is loopback bind + `Host` guard + `Origin` allowlist; a short-lived `attachTicket` is
Phase 8 work, before fleet distribution); a session event stream would remove the 2 s poll and is
Phase 3 work, not a second code path with one consumer today; `machines[].default` keeps its name
even though `isDefault` reads better, because it is now specified on both sides; and
`harness_unavailable` stays **409**, which changes no client behaviour because the client switches
on the `error` code and never on the status.

---

## File structure

Every file the phase creates or modifies, with the task that owns it. Written after the tasks
existed, so it is the list rather than the sketch. Where a responsibility is split across several
files the split is deliberate and the reason is in the owning task — most often that three tasks
writing one file guarantees three subagents colliding in the same region.

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `sandboxd/internal/ring/ring.go` | Bounded byte ring buffer. No I/O. | new | 1 |
| `sandboxd/internal/hub/hub.go` | One long-lived PTY, replay buffer, N subscribers | new | 2 |
| `sandboxd/internal/server/server.go` | `/attach` binds a subscriber to the hub; bearer auth | modified | 3 |
| `sandboxd/cmd/sandboxd/main.go` | `-args-json` flag; starts the hub once | modified | 3 |
| `images/sandbox/entrypoint.sh` | Passes `PANACEA_APP_ARGS_JSON`, `PANACEA_APP_ATTACH_TOKEN` | modified | 3 |
| `control-plane/internal/ids/ids.go` | Collision-free ids and attach tokens from `crypto/rand`. One call site, which is the fix for known defect 2. | new | 4 |
| `control-plane/internal/backend/backend.go` | `Backend`, `Spec`, `Sandbox`, `Resources`, sentinel errors, the shared `PANACEA_APP_*` and label constants | new | 4 |
| `control-plane/internal/backend/dial.go` | The one `/attach` dialer: bearer header, `SetReadLimit`, dial timeout; plus `WaitReady` | new | 4 |
| `control-plane/internal/backend/dockerbe/api.go` | The narrow slice of the Docker SDK this package uses, so tests need no daemon | new | 5 |
| `control-plane/internal/backend/dockerbe/docker.go` | `Backend` over Docker: create, publish to `127.0.0.1`, label discovery, idempotent destroy | new | 5 |
| `control-plane/go.mod` | Pinned `require` lines for the Docker SDK; the `go 1.23` directive must not move | modified | 5 |
| `control-plane/internal/backend/katabe/kata.go` | `Backend` over `internal/sandbox.Manager`. Dormant until Phase 8; compiles and is tested now. | new | 6 |
| `control-plane/internal/sandbox/manager.go` | Unchanged pod creation; `Spec` grows args/env/token/session id, plus `Get` and `List` | modified | 6 |
| `control-plane/internal/statepath/statepath.go` | Where on-disk state lives, per OS. One ladder, shared with Phase 4. | new | 7 |
| `control-plane/internal/registry/session.go` | The `Session` record and its four-member `Status` | new | 7 |
| `control-plane/internal/registry/registry.go` | `Store`, the in-memory index, `Update` as the only mutation primitive | new | 7 |
| `control-plane/internal/registry/filestore.go` | Single-document JSON persistence, `schemaVersion: 1`, worktree flags, the Windows-safe atomic replace | new | 7 |
| `control-plane/internal/harness/harness.go` | The catalogue: which harnesses exist, which are installed, which options each accepts | new | 8 |
| `control-plane/internal/harness/argv.go` | `Build` — harness + model + effort + permission mode + prompt → `(command, argv)` | new | 8 |
| `control-plane/internal/worktree/worktree.go` | **Seam for Phase 3.** `Provider` + the `Static` implementation Phase 2 ships | new | 8 |
| `control-plane/internal/project/project.go` | Read-only enumeration of projects, branches and worktrees via `git`; implements `Provider` and Phase 4's `Naming` | new | 8 |
| `control-plane/internal/httpx/problem.go` | The one error body, and the JSON read/write helpers every handler uses | new | 9 |
| `control-plane/internal/httpx/origin.go` | Origin allowlist, loopback `Host` guard, **and the CORS grant** | new | 9 |
| `control-plane/internal/api/api.go` | `Deps`, `New`, the `http.ServeMux`, the `/api/v1` alias Phase 4 needs | new | 10 |
| `control-plane/internal/api/catalog.go` | `/healthz`, `/machines`, `/projects`, `/branches`, `/worktrees` incl. the `ready` PATCH, `/harnesses` | new | 10 |
| `control-plane/internal/api/sessions.go` | Session create / list / get / patch / restart / delete, and `sessionJSON` so no secret can leak | new | 11 |
| `control-plane/internal/api/attach.go` | The five pre-upgrade checks, the read limit, the relay, last-output accounting | new | 12 |
| `control-plane/internal/proxy/proxy.go` | `RelayHooked` adds per-message byte **counters**; `Relay` keeps its signature | modified | 12 |
| `control-plane/internal/reaper/reaper.go` | The periodic sweep: orphans, disappeared sandboxes, idle derivation, idle TTL | new | 13 |
| `control-plane/cmd/api/config.go` | Every `PANACEA_APP_*` variable, parsed once, with defaults | new | 13 |
| `control-plane/cmd/api/main.go` | Wiring, restart recovery with the endpoint re-read, listener, graceful shutdown | modified | 13 |
| `desktop/package.json` | vitest/jsdom/RTL pinned, `zustand`, the `test` and `typecheck` scripts | modified | 14, 17 |
| `desktop/vitest.config.ts` | jsdom, `setupFiles`, `globals: false` | new | 14 |
| `desktop/tsconfig.node.json` | Adds `vitest.config.ts` so `npm run typecheck` sees it | modified | 14 |
| `desktop/src/test/{setup,fakes,fixtures}.ts` | `matchMedia`/`ResizeObserver`/`scrollIntoView` stubs, `FakeTerm`, `FakeWebSocket`, fixture builders | new | 14 |
| `desktop/src/styles/tokens.css` | The token layer, OKLCH, derived not sampled | new | 14 |
| `desktop/src/styles/app.css` | Resets, scrollbars, body type | new | 14 |
| `desktop/src/store/types.ts` | The domain model, mirroring the REST JSON field for field | new | 15 |
| `desktop/src/lib/{ids,immutable,time}.ts` | `makeId`; `replaceAt`/`replaceById`/`moveWithinVisible`/`idAtVisiblePosition`; elapsed formatting | new | 15 |
| `desktop/src/api/{client,index}.ts` | The typed control-plane client, `ApiError`, `attachUrl`, injected `fetch`/`WebSocket` | new | 16 |
| `desktop/src/store/{store,selectors,defaults}.ts`, `store/slices/*` | Three flat slices, `persist` v1, every derived read, `resolveDefaults`/`toCreateRequest` | new | 17 |
| `desktop/src/components/{AppShell,rail/*,subrail/*,topbar/*,common/*,menu/*,lens/*}` | Shell, four-level rail, six-lens sub-rail, top bar, split New-session button, shared primitives | new | 18 |
| `desktop/src/hooks/{heartbeat,useDismissable}.ts` | The one global clock; outside-click + Escape | new | 18 |
| `desktop/src/api/attachRegistry.ts` | Ref-counted sockets and xterm instances, the `starting` retry ladder, 10 Hz batching, pinned scroll | new | 19 |
| `desktop/src/api/poller.ts` | The one interval, paused while the document is hidden | new | 19 |
| `desktop/src/components/Terminal.tsx` | ~40 lines: construct xterm, `open()`, hand it to the registry | modified | 19 |
| `desktop/src/components/develop/*` | Develop lens, Columns, the session pane, Canvas and its persisted tiles, the lifecycle verbs | new | 20, 21, 22 |
| `desktop/src/components/palette/*`, `hooks/useKeyboardMap.ts` | The palette, the command list, the full chord table and its two deliberate non-interceptions | new | 23 |
| `desktop/src/App.tsx` | Cockpit shell: mount `AppShell`, start the poller, load the catalogue | rewritten | 24 |
| `desktop/src/App.css` | Deleted — superseded by `styles/app.css` and the per-component files | deleted | 24 |
| `desktop/src/{defects,styles/designsystem}.test.ts(x)` | The ten-defect regression suite and the mechanical design-system audit | new | 24 |
| `docs/verification/2026-09-18-phase-2-results.md` | The end-to-end Docker proof, by hand, with what was and was not observed | new | 25 |
| `desktop/src/components/Terminal.tsx` | xterm bound to the control plane | modified |
| `desktop/src/App.tsx` | Cockpit shell | rewritten |

---

# Group A — Durable sessions in sandboxd

Today `sandboxd` starts a fresh PTY per WebSocket connection and kills it on disconnect. That makes "re-attach" a lie: you would get a new shell with no history. Group A moves the PTY's lifetime to the container's lifetime and gives every attach a scrollback replay, which is what `docs/AGENTIC-UX.md` §6.5 ("session entry lands on the watermark") is built on.

---

### Task 1: Bounded ring buffer

A fixed-size byte ring holding the most recent output. Pure data structure, no I/O, no build tags — so it is testable on Windows, where the PTY packages are not.

**Files:**
- Create: `sandboxd/internal/ring/ring.go`
- Test: `sandboxd/internal/ring/ring_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `ring.New(capacity int) *Ring`
  - `(*Ring).Write(p []byte) (int, error)` — satisfies `io.Writer`, never fails, never blocks
  - `(*Ring).Snapshot() []byte` — oldest-to-newest copy of retained bytes
  - `(*Ring).Len() int`
  - `ring.DefaultCapacity = 256 * 1024`

- [ ] **Step 1: Write the failing tests**

Create `sandboxd/internal/ring/ring_test.go`:

```go
package ring_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ring"
)

func TestSnapshotOfEmptyRingIsEmpty(t *testing.T) {
	r := ring.New(16)
	if got := r.Snapshot(); len(got) != 0 {
		t.Fatalf("Snapshot() = %q, want empty", got)
	}
	if r.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", r.Len())
	}
}

func TestWritesUnderCapacityAreReturnedInOrder(t *testing.T) {
	r := ring.New(16)
	r.Write([]byte("abc"))
	r.Write([]byte("def"))
	if got := string(r.Snapshot()); got != "abcdef" {
		t.Fatalf("Snapshot() = %q, want %q", got, "abcdef")
	}
	if r.Len() != 6 {
		t.Fatalf("Len() = %d, want 6", r.Len())
	}
}

func TestOldestBytesAreDiscardedOnOverflow(t *testing.T) {
	r := ring.New(4)
	r.Write([]byte("abcdef"))
	if got := string(r.Snapshot()); got != "cdef" {
		t.Fatalf("Snapshot() = %q, want %q", got, "cdef")
	}
	if r.Len() != 4 {
		t.Fatalf("Len() = %d, want 4", r.Len())
	}
}

func TestWriteLargerThanCapacityKeepsTheTail(t *testing.T) {
	r := ring.New(4)
	r.Write([]byte("0123456789"))
	if got := string(r.Snapshot()); got != "6789" {
		t.Fatalf("Snapshot() = %q, want %q", got, "6789")
	}
}

func TestWrapAroundPreservesOrder(t *testing.T) {
	r := ring.New(5)
	r.Write([]byte("abc"))
	r.Write([]byte("de"))
	r.Write([]byte("fg"))
	if got := string(r.Snapshot()); got != "cdefg" {
		t.Fatalf("Snapshot() = %q, want %q", got, "cdefg")
	}
}

func TestWriteReportsFullLengthConsumed(t *testing.T) {
	r := ring.New(2)
	n, err := r.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != 6 {
		t.Fatalf("Write n = %d, want 6 (io.Writer must report all bytes consumed)", n)
	}
}

func TestSnapshotDoesNotAliasInternalStorage(t *testing.T) {
	r := ring.New(8)
	r.Write([]byte("abcd"))
	snap := r.Snapshot()
	snap[0] = 'X'
	if got := string(r.Snapshot()); got != "abcd" {
		t.Fatalf("Snapshot aliased internal storage: second call = %q", got)
	}
}

func TestBinarySafe(t *testing.T) {
	r := ring.New(8)
	payload := []byte{0x00, 0x1b, 0x5b, 0xff}
	r.Write(payload)
	if !bytes.Equal(r.Snapshot(), payload) {
		t.Fatalf("Snapshot() = %v, want %v", r.Snapshot(), payload)
	}
}

func TestConcurrentWritesDoNotRace(t *testing.T) {
	r := ring.New(1024)
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 200; j++ {
				r.Write([]byte(strings.Repeat("x", 7)))
				_ = r.Snapshot()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./sandboxd/internal/ring/...`
Expected: FAIL — `no required module provides package .../sandboxd/internal/ring`.

- [ ] **Step 3: Write the implementation**

Create `sandboxd/internal/ring/ring.go`:

```go
// Package ring holds the most recent N bytes written to it, discarding the
// oldest. It is the scrollback a re-attaching client replays. It knows nothing
// about terminal semantics: bytes in, the same bytes out.
package ring

import "sync"

// DefaultCapacity is the scrollback retained per session. 256 KiB is roughly
// 3000 lines of dense agent output — enough to re-establish context on return
// without holding a whole transcript in the sandbox's memory.
const DefaultCapacity = 256 * 1024

// Ring is a fixed-capacity, overwrite-oldest byte buffer. Safe for concurrent
// use.
type Ring struct {
	mu    sync.Mutex
	buf   []byte
	start int  // index of the oldest byte
	size  int  // bytes currently retained
	full  bool // true once size == cap(buf)
}

// New returns a Ring retaining at most capacity bytes. A capacity below 1 is
// raised to 1 so the zero case cannot panic.
func New(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{buf: make([]byte, capacity)}
}

// Write appends p, discarding the oldest bytes if necessary. It always reports
// len(p) consumed and never returns an error, as io.Writer requires of a sink
// that cannot fail.
func (r *Ring) Write(p []byte) (int, error) {
	total := len(p)
	if total == 0 {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	c := len(r.buf)
	// Only the last c bytes can survive; drop the rest up front.
	if total > c {
		p = p[total-c:]
	}
	for _, b := range p {
		end := (r.start + r.size) % c
		r.buf[end] = b
		if r.full {
			r.start = (r.start + 1) % c
		} else {
			r.size++
			if r.size == c {
				r.full = true
			}
		}
	}
	return total, nil
}

// Snapshot returns a copy of the retained bytes, oldest first.
func (r *Ring) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]byte, r.size)
	c := len(r.buf)
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(r.start+i)%c]
	}
	return out
}

// Len reports how many bytes are currently retained.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./sandboxd/internal/ring/... -race -count=1 -v`
Expected: PASS, nine tests, no race reports.

- [ ] **Step 5: Commit**

```bash
git add sandboxd/internal/ring
git commit -m "feat(ring): bounded scrollback buffer for session replay

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: The session hub — one PTY, many attaches

The hub owns a PTY for the container's lifetime. Attaching subscribes; detaching unsubscribes; neither starts nor stops the process. A new subscriber is handed the ring snapshot before any live output, so re-attaching shows you what you missed.

**Files:**
- Create: `sandboxd/internal/hub/hub.go`
- Test: `sandboxd/internal/hub/hub_test.go`

**Interfaces:**
- Consumes: `ptysession.Start`, `ring.New`, `ring.DefaultCapacity`
- Produces:
  - `hub.Config{Command string; Args []string; Env []string; Scrollback int}`
  - `hub.Start(cfg Config) (*Hub, error)` — starts the PTY once
  - `(*Hub).Subscribe() (replay []byte, out <-chan []byte, cancel func())`
  - `(*Hub).Write(p []byte) (int, error)` — stdin
  - `(*Hub).Resize(cols, rows uint16) error`
  - `(*Hub).Done() <-chan struct{}` — closed when the process exits
  - `(*Hub).ExitReason() string` — empty until `Done` is closed
  - `(*Hub).Close() error` — kills the process; only the entrypoint calls this
- **Shared contract with Task 3:** `Subscribe` must deliver `replay` *before* the first channel send, and the channel must be closed when the process exits. Task 3's `/attach` handler relies on both to order the `Stdout` replay frame ahead of live frames and to emit exactly one `Exit` frame.

- [ ] **Step 1: Write the failing tests**

Create `sandboxd/internal/hub/hub_test.go`:

```go
//go:build linux || darwin

package hub_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/hub"
)

// collect drains out until want appears or the deadline passes, seeding the
// accumulator with replay.
func collect(t *testing.T, replay []byte, out <-chan []byte, want []byte, d time.Duration) string {
	t.Helper()
	var acc bytes.Buffer
	acc.Write(replay)
	if bytes.Contains(acc.Bytes(), want) {
		return acc.String()
	}
	deadline := time.After(d)
	for {
		select {
		case b, ok := <-out:
			if !ok {
				t.Fatalf("channel closed before %q appeared; got %q", want, acc.String())
			}
			acc.Write(b)
			if bytes.Contains(acc.Bytes(), want) {
				return acc.String()
			}
		case <-deadline:
			t.Fatalf("did not see %q within %s; got %q", want, d, acc.String())
		}
	}
}

func TestHubStreamsOutputToASubscriber(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	replay, out, cancel := h.Subscribe()
	defer cancel()

	if _, err := h.Write([]byte("echo panacea-ok\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("panacea-ok"), 8*time.Second)
}

func TestReAttachReplaysEarlierOutput(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	replay, out, cancel := h.Subscribe()
	if _, err := h.Write([]byte("echo first-output\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("first-output"), 8*time.Second)

	// Detach. The process must keep running.
	cancel()

	replay2, out2, cancel2 := h.Subscribe()
	defer cancel2()
	if !bytes.Contains(replay2, []byte("first-output")) {
		t.Fatalf("replay after re-attach = %q, want it to contain %q", replay2, "first-output")
	}

	// And the same PTY still accepts input.
	if _, err := h.Write([]byte("echo second-output\n")); err != nil {
		t.Fatalf("Write after re-attach: %v", err)
	}
	collect(t, replay2, out2, []byte("second-output"), 8*time.Second)
}

func TestTwoSubscribersBothReceiveLiveOutput(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	r1, o1, c1 := h.Subscribe()
	defer c1()
	r2, o2, c2 := h.Subscribe()
	defer c2()

	if _, err := h.Write([]byte("echo both-see-this\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, r1, o1, []byte("both-see-this"), 8*time.Second)
	collect(t, r2, o2, []byte("both-see-this"), 8*time.Second)
}

func TestResizeReachesTheProcess(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	replay, out, cancel := h.Subscribe()
	defer cancel()

	if err := h.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if _, err := h.Write([]byte("stty size\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("40 120"), 8*time.Second)
}

func TestDoneClosesAndExitReasonIsSetWhenTheProcessExits(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh", Args: []string{"-c", "exit 3"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	select {
	case <-h.Done():
	case <-time.After(8 * time.Second):
		t.Fatal("Done() was not closed within 8s of the process exiting")
	}
	if got := h.ExitReason(); got == "" {
		t.Fatal("ExitReason() is empty after the process exited")
	}
}

func TestSubscriberChannelClosesWhenTheProcessExits(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh", Args: []string{"-c", "echo bye; exit 0"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	_, out, cancel := h.Subscribe()
	defer cancel()

	deadline := time.After(8 * time.Second)
	for {
		select {
		case _, ok := <-out:
			if !ok {
				return // closed, as required
			}
		case <-deadline:
			t.Fatal("subscriber channel was not closed within 8s of process exit")
		}
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	_, _, cancel := h.Subscribe()
	cancel()
	cancel() // must not panic or close a closed channel
}

func TestSlowSubscriberDoesNotStallTheHub(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	// A subscriber that never reads.
	_, _, stuck := h.Subscribe()
	defer stuck()

	// A subscriber that does read must still get output.
	replay, out, cancel := h.Subscribe()
	defer cancel()

	if _, err := h.Write([]byte("for i in 1 2 3 4 5; do echo line-$i; done; echo tail-marker\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("tail-marker"), 10*time.Second)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./sandboxd/internal/hub/...`
Expected on Linux/macOS: FAIL — package does not exist.
Expected on Windows: `[no test files]` and a build failure for the missing package once it is referenced. The hub is POSIX-only for the same reason `ptysession` is: `creack/pty` has no Windows support. **Run Group A's PTY-backed tests inside the sandbox image** (Task 3, Step 8) if you are on Windows.

- [ ] **Step 3: Write the implementation**

Create `sandboxd/internal/hub/hub.go`:

```go
//go:build linux || darwin

// Package hub owns a single PTY for the lifetime of the sandbox and fans its
// output out to zero or more attached clients. Detaching does not stop the
// process — that is the whole point: a session you can leave and come back to.
package hub

import (
	"fmt"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ptysession"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ring"
)

// subscriberBuffer is how many chunks a slow client may fall behind before the
// hub starts dropping its chunks. The hub must never block on a client: one
// stalled WebSocket would freeze the agent's terminal.
const subscriberBuffer = 256

// Config describes the process the hub runs.
type Config struct {
	Command    string
	Args       []string
	Env        []string
	Scrollback int // bytes retained for replay; 0 means ring.DefaultCapacity
}

// Hub is one PTY plus its scrollback and subscribers.
type Hub struct {
	sess *ptysession.Session
	buf  *ring.Ring

	mu     sync.Mutex
	subs   map[int]chan []byte
	nextID int
	closed bool

	done       chan struct{}
	doneOnce   sync.Once
	exitReason string
}

// Start launches the command in a PTY and begins pumping its output into the
// scrollback and to any subscribers.
func Start(cfg Config) (*Hub, error) {
	capacity := cfg.Scrollback
	if capacity <= 0 {
		capacity = ring.DefaultCapacity
	}
	sess, err := ptysession.Start(cfg.Command, cfg.Args, cfg.Env)
	if err != nil {
		return nil, err
	}
	h := &Hub{
		sess: sess,
		buf:  ring.New(capacity),
		subs: make(map[int]chan []byte),
		done: make(chan struct{}),
	}
	go h.pump()
	return h, nil
}

func (h *Hub) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := h.sess.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			h.buf.Write(chunk)
			h.broadcast(chunk)
		}
		if err != nil {
			h.finish(fmt.Sprintf("session ended: %v", err))
			return
		}
	}
}

func (h *Hub) broadcast(chunk []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- chunk:
		default:
			// Subscriber is not keeping up. Drop this chunk for that client
			// rather than stalling the PTY for everyone. The client still has
			// the ring on its next attach.
		}
	}
}

func (h *Hub) finish(reason string) {
	h.doneOnce.Do(func() {
		h.mu.Lock()
		h.exitReason = reason
		h.closed = true
		for id, ch := range h.subs {
			close(ch)
			delete(h.subs, id)
		}
		h.mu.Unlock()
		close(h.done)
	})
}

// Subscribe registers a client. It returns the scrollback to replay first, a
// channel of live chunks, and a cancel function that detaches without stopping
// the process. The channel is closed when the process exits.
//
// The replay is captured under the same lock that registers the subscriber, so
// no chunk can slip between the two and be lost or duplicated out of order.
func (h *Hub) Subscribe() (replay []byte, out <-chan []byte, cancel func()) {
	h.mu.Lock()
	snapshot := h.buf.Snapshot()
	ch := make(chan []byte, subscriberBuffer)
	if h.closed {
		h.mu.Unlock()
		close(ch)
		return snapshot, ch, func() {}
	}
	id := h.nextID
	h.nextID++
	h.subs[id] = ch
	h.mu.Unlock()

	var once sync.Once
	return snapshot, ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if c, ok := h.subs[id]; ok {
				delete(h.subs, id)
				close(c)
			}
		})
	}
}

// Write sends bytes to the process as if typed.
func (h *Hub) Write(p []byte) (int, error) { return h.sess.Write(p) }

// Resize informs the process that its terminal dimensions changed.
func (h *Hub) Resize(cols, rows uint16) error { return h.sess.Resize(cols, rows) }

// Done is closed when the process exits.
func (h *Hub) Done() <-chan struct{} { return h.done }

// ExitReason describes why the process ended. Empty until Done is closed.
func (h *Hub) ExitReason() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitReason
}

// Close kills the process and releases the PTY. Only the process entrypoint
// calls this — an attach handler must call its subscription's cancel instead.
func (h *Hub) Close() error {
	err := h.sess.Close()
	h.finish("closed")
	return err
}
```

- [ ] **Step 4: Run the tests to verify they pass**

On Linux/macOS run: `go test ./sandboxd/internal/hub/... -race -count=1 -v`
Expected: PASS, eight tests.

On Windows the package is excluded by build tag; verify in the container at Task 3 Step 8.

- [ ] **Step 5: Verify the Windows build is still clean**

Run: `go build ./sandboxd/... ./control-plane/...`
Expected: exits 0, no output. If it errors with `undefined: hub.Start`, a non-tagged file references the hub — the reference must be behind the same build tag.

- [ ] **Step 6: Commit**

```bash
git add sandboxd/internal/hub
git commit -m "feat(hub): PTY that survives detach, with scrollback replay

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: sandboxd serves the hub, with a bearer token and JSON argv

Two defects are closed here alongside the hub rewrite.

**Defect 1 — anything on the host can attach.** The probe run while writing this plan attached to a sandbox on `127.0.0.1:32773` with nothing but a WebSocket client. `sandboxd` sets `InsecureSkipVerify: true` on the assumption that only the control plane can reach it. On a laptop that is false: every process on the machine can reach a published port. A per-sandbox bearer token, injected at create time, closes it.

**Defect 2 — the opening prompt is corrupted by commas.** `-args` splits on `,`, and `PANACEA_APP_ARGS` is a comma-joined string. The moment a user types an opening prompt containing a comma — "add tests, then refactor" — the argv is wrong. `-args-json` fixes it.

**Files:**
- Modify: `sandboxd/internal/server/server.go`, `sandboxd/cmd/sandboxd/main.go`, `images/sandbox/entrypoint.sh`
- Test: `sandboxd/internal/server/server_test.go` (extend), `sandboxd/cmd/sandboxd/main_test.go` (replace placeholder)

**Interfaces:**
- Consumes: `hub.Start`, `hub.Config`, `wire.*`
- Produces:
  - `server.Config{Command string; Args []string; Env []string; AttachToken string; Scrollback int}`
  - `server.New(cfg Config) (*Server, error)` — **signature change: now returns an error**, because it starts the PTY eagerly
  - `(*Server).Close() error`
  - `main.parseArgs(argsCSV, argsJSON string) ([]string, error)`
  - `server.maxMessageBytes = 4 << 20` (unexported; mirrored as `backend.MaxMessageBytes` in Task 4)
- **Shared contract with Tasks 6 and 7 (both backends):** a sandbox container is started with env `PANACEA_APP_COMMAND`, `PANACEA_APP_ARGS_JSON` (a JSON array of strings), and `PANACEA_APP_ATTACH_TOKEN`. The control plane must send `Authorization: Bearer <token>` on `/attach` or receive **401**. `/healthz` stays unauthenticated so a readiness probe works. State this in both backends' Interfaces blocks.
- **Shared contract with Tasks 5, 12 and 18 — the WebSocket read limit.** `github.com/coder/websocket` v1.8.12 caps a single inbound message at `defaultReadLimit = 32768` bytes (`read.go:107`; the internal limit reader is seeded with `defaultReadLimit+1`, so 32769 bytes is the last size that still reads). Two messages in this design exceed that:
  - the **replay frame** this task introduces — up to `ring.DefaultCapacity` (262144) + 1 kind byte;
  - a **large paste** typed into a session, which arrives as one `Stdin` frame.

  Every `*websocket.Conn` on the Go side of the path must therefore call `SetReadLimit(4 << 20)` immediately after `Accept` or `Dial`. There are three such sites and all three must be done or the pipe breaks at a size that light testing never reaches: sandboxd's `/attach` (this task), the control plane's upstream dial (Task 5), and the control plane's client accept (Task 12). Browser `WebSocket` has no equivalent limit, so the desktop needs no change. Phase 1 survived only because a full 32 KiB PTY read encodes to exactly 32769 bytes — one byte inside the limit.

- [ ] **Step 1: Write the failing server tests**

Replace `sandboxd/internal/server/server_test.go` entirely:

```go
//go:build linux || darwin

package server_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

const testToken = "test-token-abc"

func newServer(t *testing.T, cfg server.Config) *httptest.Server {
	t.Helper()
	s, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv
}

func dial(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if token != "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+token)
	}
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach", opts)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { c.Close(websocket.StatusNormalClosure, "") })
	return c, ctx
}

// readUntil accumulates Stdout payloads until want appears.
func readUntil(t *testing.T, c *websocket.Conn, ctx context.Context, want []byte, d time.Duration) string {
	t.Helper()
	var acc bytes.Buffer
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithDeadline(ctx, deadline)
		_, data, err := c.Read(readCtx)
		cancel()
		if err != nil {
			t.Fatalf("Read: %v (got %q)", err, acc.String())
		}
		kind, payload, err := wire.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if kind != wire.Stdout {
			continue
		}
		acc.Write(payload)
		if bytes.Contains(acc.Bytes(), want) {
			return acc.String()
		}
	}
	t.Fatalf("never saw %q; got %q", want, acc.String())
	return ""
}

func TestHealthzNeedsNoToken(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAttachWithoutTokenIsRejected(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach", nil)
	if err == nil {
		t.Fatal("expected the handshake to fail without a token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestAttachWithWrongTokenIsRejected(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	opts.HTTPHeader.Set("Authorization", "Bearer not-the-token")
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach", opts)
	if err == nil {
		t.Fatal("expected the handshake to fail with a wrong token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestStreamsStdoutAndAcceptsStdin(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	c, ctx := dial(t, srv, testToken)

	if err := c.Write(ctx, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo panacea-ok\n"))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, c, ctx, []byte("panacea-ok"), 10*time.Second)
}

func TestAppliesResize(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	c, ctx := dial(t, srv, testToken)

	rp, _ := wire.ResizePayload{Cols: 120, Rows: 40}.MarshalBinary()
	if err := c.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Resize, rp)); err != nil {
		t.Fatalf("Write resize: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := c.Write(ctx, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("stty size\n"))); err != nil {
		t.Fatalf("Write stty: %v", err)
	}
	readUntil(t, c, ctx, []byte("40 120"), 10*time.Second)
}

// The behaviour this whole group exists for.
func TestReAttachReplaysScrollbackAndKeepsTheProcess(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})

	c1, ctx1 := dial(t, srv, testToken)
	if err := c1.Write(ctx1, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo before-detach\n"))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, c1, ctx1, []byte("before-detach"), 10*time.Second)
	c1.Close(websocket.StatusNormalClosure, "")

	c2, ctx2 := dial(t, srv, testToken)
	// The replay must arrive without us typing anything.
	readUntil(t, c2, ctx2, []byte("before-detach"), 10*time.Second)

	// And the same shell is still alive.
	if err := c2.Write(ctx2, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo after-reattach\n"))); err != nil {
		t.Fatalf("Write after re-attach: %v", err)
	}
	readUntil(t, c2, ctx2, []byte("after-reattach"), 10*time.Second)
}

// A paste larger than coder/websocket's 32 KiB default read limit must survive.
// Without SetReadLimit the connection is closed with StatusMessageTooBig and the
// session appears to die for no reason.
func TestLargeStdinFrameIsAccepted(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	c, ctx := dial(t, srv, testToken)

	// 100 KiB of printable text, then a newline so the shell acts on it.
	big := bytes.Repeat([]byte("x"), 100*1024)
	line := append([]byte("echo len-"), big...)
	line = append(line, '\n')

	if err := c.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Stdin, line)); err != nil {
		t.Fatalf("Write large stdin: %v", err)
	}
	// The shell echoes the line back through the PTY; seeing the prefix proves
	// the frame was read rather than rejected.
	readUntil(t, c, ctx, []byte("len-xxxxxxxxxx"), 15*time.Second)
}

func TestExitFrameIsSentWhenTheProcessEnds(t *testing.T) {
	srv := newServer(t, server.Config{
		Command: "/bin/sh", Args: []string{"-c", "exit 0"}, AttachToken: testToken,
	})
	c, ctx := dial(t, srv, testToken)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithDeadline(ctx, deadline)
		_, data, err := c.Read(readCtx)
		cancel()
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		kind, _, err := wire.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if kind == wire.Exit {
			return
		}
	}
	t.Fatal("no Exit frame within 10s of the process exiting")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run (Linux/macOS): `go test ./sandboxd/internal/server/...`
Expected: FAIL — `server.New(cfg)` returns one value, not two; `server.Config` has no field `AttachToken`.

- [ ] **Step 3: Rewrite the server**

Replace `sandboxd/internal/server/server.go`:

```go
//go:build linux || darwin

// Package server exposes the sandbox's single PTY over a WebSocket using the
// wire protocol. It never inspects terminal content.
package server

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/hub"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

// maxMessageBytes raises coder/websocket's 32 KiB default read limit. The
// replay frame carries the whole scrollback (up to ring.DefaultCapacity) in one
// message and a paste arrives as one Stdin frame; either exceeds the default and
// would close the connection with StatusMessageTooBig. Must match
// backend.MaxMessageBytes in the control plane.
const maxMessageBytes = 4 << 20

// Config describes the process every connection attaches to.
type Config struct {
	Command string
	Args    []string
	Env     []string

	// AttachToken is a shared secret the control plane must present on
	// /attach. On a developer laptop the sandbox port is published to
	// 127.0.0.1, which every local process can reach — the origin check in the
	// control plane does not protect this hop, so this token does.
	// An empty token disables the check and is only acceptable in tests.
	AttachToken string

	// Scrollback is the replay buffer size in bytes; 0 uses the hub default.
	Scrollback int
}

// Server binds HTTP requests to the sandbox's hub.
type Server struct {
	cfg Config
	h   *hub.Hub
	mux *http.ServeMux
}

// New starts the PTY and returns a Server serving /attach and /healthz. The
// PTY starts here, once, rather than per connection: a session must outlive
// the client that is looking at it.
func New(cfg Config) (*Server, error) {
	h, err := hub.Start(hub.Config{
		Command:    cfg.Command,
		Args:       cfg.Args,
		Env:        cfg.Env,
		Scrollback: cfg.Scrollback,
	})
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, h: h, mux: http.NewServeMux()}
	s.mux.HandleFunc("/attach", s.attach)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Close stops the underlying process. Only the entrypoint calls this.
func (s *Server) Close() error { return s.h.Close() }

func (s *Server) authorised(r *http.Request) bool {
	if s.cfg.AttachToken == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.AttachToken)) == 1
}

func (s *Server) attach(w http.ResponseWriter, r *http.Request) {
	if !s.authorised(r) {
		slog.Warn("rejected unauthenticated attach", "remote", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Origin is enforced upstream by the control plane (cmd/api), which is the
	// only component a browser context can reach. This hop is guarded by the
	// bearer token above.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		slog.Error("websocket accept failed", "err", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "closing")
	conn.SetReadLimit(maxMessageBytes)

	ctx := r.Context()
	replay, out, cancel := s.h.Subscribe()
	defer cancel()

	// Replay first, so a returning client sees what it missed before anything
	// live arrives.
	if len(replay) > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Stdout, replay)); err != nil {
			return
		}
	}

	go func() {
		for {
			select {
			case chunk, ok := <-out:
				if !ok {
					_ = conn.Write(ctx, websocket.MessageBinary,
						wire.Encode(wire.Exit, []byte(s.h.ExitReason())))
					conn.Close(websocket.StatusNormalClosure, "session ended")
					return
				}
				if err := conn.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Stdout, chunk)); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			// The client went away. Detach only — the process keeps running.
			return
		}
		kind, payload, err := wire.Decode(data)
		if err != nil {
			continue
		}
		switch kind {
		case wire.Stdin:
			if _, err := s.h.Write(payload); err != nil {
				return
			}
		case wire.Resize:
			var rp wire.ResizePayload
			if err := rp.UnmarshalBinary(payload); err == nil {
				_ = s.h.Resize(rp.Cols, rp.Rows)
			}
		}
	}
}
```

- [ ] **Step 4: Run the server tests to verify they pass**

Run (Linux/macOS): `go test ./sandboxd/internal/server/... -race -count=1 -v`
Expected: PASS, eight tests. If `TestLargeStdinFrameIsAccepted` fails with
`read limited at 32769 bytes`, the `conn.SetReadLimit(maxMessageBytes)` line is
missing or placed after the first `conn.Read`.

- [ ] **Step 5: Write the failing argv test**

Replace `sandboxd/cmd/sandboxd/main_test.go`:

```go
package main

import (
	"reflect"
	"testing"
)

func TestParseArgsPrefersJSON(t *testing.T) {
	got, err := parseArgs("ignored,csv", `["--model","opus","add tests, then refactor"]`)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	want := []string{"--model", "opus", "add tests, then refactor"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseArgsFallsBackToCSV(t *testing.T) {
	got, err := parseArgs("--help,--version", "")
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	want := []string{"--help", "--version"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseArgsEmptyMeansNoArgs(t *testing.T) {
	got, err := parseArgs("", "")
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want no arguments", got)
	}
}

func TestParseArgsRejectsMalformedJSON(t *testing.T) {
	if _, err := parseArgs("", "not json"); err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestParseArgsRejectsNonArrayJSON(t *testing.T) {
	if _, err := parseArgs("", `{"a":1}`); err == nil {
		t.Fatal("expected an error for a JSON object, got nil")
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./sandboxd/cmd/sandboxd/...`
Expected: FAIL — `undefined: parseArgs`.

- [ ] **Step 7: Rewrite the entrypoint**

Replace `sandboxd/cmd/sandboxd/main.go`:

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// parseArgs resolves the process arguments. argsJSON wins when present: the
// CSV form cannot carry an opening prompt containing a comma, which is the
// common case the moment a user types one.
func parseArgs(argsCSV, argsJSON string) ([]string, error) {
	if strings.TrimSpace(argsJSON) != "" {
		var out []string
		if err := json.Unmarshal([]byte(argsJSON), &out); err != nil {
			return nil, fmt.Errorf("args-json must be a JSON array of strings: %w", err)
		}
		return out, nil
	}
	if argsCSV == "" {
		return nil, nil
	}
	return strings.Split(argsCSV, ","), nil
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	command := flag.String("command", "/bin/bash", "command to run in the PTY")
	argsCSV := flag.String("args", "", "comma-separated arguments (legacy; prefer -args-json)")
	argsJSON := flag.String("args-json", "", "arguments as a JSON array of strings")
	token := flag.String("attach-token", os.Getenv("PANACEA_APP_ATTACH_TOKEN"),
		"shared secret required on /attach")
	flag.Parse()

	argv, err := parseArgs(*argsCSV, *argsJSON)
	if err != nil {
		slog.Error("bad arguments", "err", err)
		os.Exit(2)
	}

	srv, err := newServer(*command, argv, *token)
	if err != nil {
		slog.Error("could not start the session", "err", err)
		os.Exit(1)
	}

	slog.Info("sandboxd listening", "addr", *addr, "command", *command, "args", argv,
		"authenticated", *token != "")
	if err := http.ListenAndServe(*addr, srv); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
```

Create `sandboxd/cmd/sandboxd/serve_posix.go`:

```go
//go:build linux || darwin

package main

import (
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
)

func newServer(command string, argv []string, token string) (http.Handler, error) {
	return server.New(server.Config{Command: command, Args: argv, AttachToken: token})
}
```

Create `sandboxd/cmd/sandboxd/serve_other.go`:

```go
//go:build !linux && !darwin

package main

import (
	"errors"
	"net/http"
)

// sandboxd runs inside the Linux sandbox image. This stub exists only so the
// workspace builds on a Windows development machine; creack/pty has no Windows
// support and ConPTY is out of scope for this phase.
func newServer(string, []string, string) (http.Handler, error) {
	return nil, errors.New("sandboxd requires linux or darwin")
}
```

- [ ] **Step 8: Run the whole Go suite**

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`
Expected on Windows: `ok` for `ring`, `wire`, `cmd/sandboxd`, and the control-plane packages; `[no test files]` for `hub`, `ptysession`, `server`.
Expected on Linux/macOS: every package `ok`.

- [ ] **Step 9: Update the image entrypoint**

Replace `images/sandbox/entrypoint.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# COMMAND selects the harness. Defaults to a plain shell so a sandbox is
# useful even when no agent is requested.
COMMAND="${PANACEA_APP_COMMAND:-/bin/bash}"

# ARGS_JSON is a JSON array of strings. It is preferred over the legacy
# comma-separated ARGS because an opening prompt routinely contains a comma.
ARGS_JSON="${PANACEA_APP_ARGS_JSON:-}"
ARGS="${PANACEA_APP_ARGS:-}"

exec /usr/local/bin/sandboxd \
  -addr :8080 \
  -command "${COMMAND}" \
  -args "${ARGS}" \
  -args-json "${ARGS_JSON}"
```

`PANACEA_APP_ATTACH_TOKEN` is read from the environment by `main.go` and deliberately not passed on the command line, so it does not appear in `ps` output inside the sandbox.

- [ ] **Step 10: Rebuild the image and run the POSIX tests inside it**

This is how a Windows developer executes the build-tagged tests. Run in **PowerShell**:

```powershell
docker build -f images/sandbox/Dockerfile -t panacea-app/sandbox:dev .
docker run --rm -v "${PWD}/sandboxd:/src/sandboxd" -w /src/sandboxd golang:1.23-bookworm `
  sh -c "GOFLAGS=-mod=vendor go test ./internal/... ./cmd/... -count=1"
```

Expected: `ok` for `internal/hub`, `internal/ring`, `internal/ptysession`, `internal/server`, `cmd/sandboxd`.
Note: `-race` is omitted here because the vendored tree is built with `CGO_ENABLED=0` semantics in CI; the race detector runs on the Linux CI agent instead.

- [ ] **Step 11: Verify the re-attach behaviour against the real image**

Run in **PowerShell**:

```powershell
docker rm -f pa-reattach 2>$null | Out-Null
docker run -d --name pa-reattach -p 127.0.0.1:8099:8080 `
  -e PANACEA_APP_COMMAND=/bin/bash `
  -e PANACEA_APP_ATTACH_TOKEN=local-dev-token `
  panacea-app/sandbox:dev
Start-Sleep -Seconds 2
curl.exe -s -o NUL -w "%{http_code}`n" http://127.0.0.1:8099/healthz
curl.exe -s -o NUL -w "%{http_code}`n" http://127.0.0.1:8099/attach
docker rm -f pa-reattach | Out-Null
```

Expected: `200` for `/healthz`, then `401` for `/attach` without a token. A `400` instead of `401` means the token check is running after the WebSocket upgrade attempt — move it before `websocket.Accept`.

- [ ] **Step 12: Commit**

```bash
git add sandboxd images/sandbox/entrypoint.sh
git commit -m "feat(sandboxd): durable session hub, attach token, JSON argv

The PTY now lives for the container's lifetime rather than the connection's,
so detaching no longer kills the agent and re-attaching replays scrollback.
Adds a per-sandbox bearer token because a published loopback port is reachable
by every process on a developer laptop, and -args-json because the legacy
comma-separated form corrupts any opening prompt containing a comma.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

# Group B — The sandbox backend interface

Group A made a session outlive its client. Group B makes the control plane able to create one. It introduces the seam `docs/PROGRAM.md` names as the architectural consequence of the local-first pivot — one `Backend` interface, two real implementations — plus the three things the API layer in Group C cannot be written without: a session registry that survives a restart, a harness catalogue that constructs argv without ever wrapping a CLI, and a read-only view of the user's projects, branches and worktrees.

Two of these are product features, not scaffolding. The **Docker backend is the "local machine"** of `docs/FUNCTIONAL-SPEC.md` — the one badged `unsandboxed` because a local agent is bounded by your OS user rather than a microVM. The **Kata backend is Phase 1's work preserved**: it compiles, it is unit-tested against a fake clientset, and it stays dormant until Phase 8 turns it on with a configuration change rather than a rewrite.

---

### Task 4: The backend seam and the one dialer

Everything downstream of this task talks to sandboxes through one interface and dials them through one function. The dialer is a function rather than a line in each backend for a specific reason: `github.com/coder/websocket` caps a single inbound message at 32768 bytes, and every `*websocket.Conn` on the Go side of the path has to raise that limit immediately after the handshake. Three sites need it. A per-backend copy is three chances to forget.

The id generator is here too, because `docs/FUNCTIONAL-SPEC.md` known defect 2 is a home-rolled `uid()` that collides on fast successive calls. The fix is not a better algorithm per call site — it is one call site, using `crypto/rand`.

**Files:**
- Create: `control-plane/internal/ids/ids.go`
- Create: `control-plane/internal/backend/backend.go`
- Create: `control-plane/internal/backend/dial.go`
- Modify: `control-plane/go.mod` (one line: promote `github.com/coder/websocket` from indirect to direct)
- Test: `control-plane/internal/ids/ids_test.go`
- Test: `control-plane/internal/backend/backend_test.go`
- Test: `control-plane/internal/backend/dial_test.go`

**Interfaces:**

- Consumes — **the sandbox's HTTP contract, fixed by Group A task 3.** A sandbox container is started with the environment variables `PANACEA_APP_COMMAND` (the harness binary), `PANACEA_APP_ARGS_JSON` (a JSON array of strings) and `PANACEA_APP_ATTACH_TOKEN` (a shared secret). Inside the container `sandboxd` serves `GET /healthz` unauthenticated on port 8080, and `GET /attach` as a WebSocket upgrade that answers **401** unless the request carries `Authorization: Bearer <that token>`. The frame protocol on that socket is one leading kind byte then an opaque payload; this task never decodes it.
- Produces:
  - `ids.New(prefix string) string` — `prefix + "-" + 8 lowercase hex characters`
  - `ids.Token() string` — 256-bit URL-safe secret, 43 characters
  - `backend.Backend` interface — `Kind`, `Create`, `Get`, `List`, `Attach`, `Destroy`, `Close`
  - `backend.Spec`, `backend.Sandbox`, `backend.Mount`, `backend.Resources`, `backend.EnvEntry`
  - `backend.Kind` (`KindDocker`, `KindKata`), `backend.State` (`StateStarting`, `StateRunning`, `StateStopped`, `StateGone`)
  - `backend.MaxMessageBytes`, `backend.AttachPath`, `backend.HealthPath`, `backend.ContainerPort`
  - `backend.EnvCommand`, `backend.EnvArgsJSON`, `backend.EnvToken`
  - `backend.LabelManaged`, `backend.LabelSandboxID`, `backend.LabelSessionID`, `backend.LabelCreatedAt`, `backend.KubeLabelSandboxID`, `backend.KubeLabelSessionID`, `backend.KubeLabelComponent`
  - `backend.ErrNotFound`, `ErrNotRunning`, `ErrAlreadyExists`, `ErrUnavailable`, `ErrUnauthorized`, `ErrUnsupported`, `ErrNonLoopbackPublish`
  - `backend.ValidateSpec(Spec) error`, `(Spec).ArgsJSON() string`, `(Spec).EnvEntries() []EnvEntry`
  - `backend.Dial(ctx, endpoint, token string) (*websocket.Conn, error)`
  - `backend.WaitReady(ctx, endpoint string, timeout time.Duration) error`
  - `backend.DefaultResources`, `backend.DialTimeout`, `backend.ReadyTimeout`
- **Shared contract with tasks 5, 6, 11, 12 and 13.** `Attach` returns `*websocket.Conn` and deliberately not `io.ReadWriteCloser`: the wire protocol is message-framed, and a 32 KiB stdout frame split across two reads of a byte stream decodes as two frames, the second with a garbage kind byte. `proxy.Relay` already takes `*websocket.Conn`.

- [ ] **Step 1: Write the failing id tests**

Create `control-plane/internal/ids/ids_test.go`:

```go
package ids_test

import (
	"regexp"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

var (
	idPattern    = regexp.MustCompile(`^[a-z]-[0-9a-f]{8}$`)
	tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

func TestNewHasThePrefixAndEightHexCharacters(t *testing.T) {
	got := ids.New("s")
	if !idPattern.MatchString(got) {
		t.Fatalf("New(%q) = %q, want the shape s-3f2a91c0", "s", got)
	}
}

func TestNewKeepsWhicheverPrefixItIsGiven(t *testing.T) {
	for _, prefix := range []string{"s", "p", "b", "w", "m"} {
		got := ids.New(prefix)
		if got[:2] != prefix+"-" {
			t.Errorf("New(%q) = %q, want it to start %q", prefix, got, prefix+"-")
		}
	}
}

// docs/FUNCTIONAL-SPEC.md known defect 2: a home-rolled uid() collides on fast
// successive calls. 200 draws from 32 bits of crypto/rand collide with
// probability about 200*199/2/2^32, i.e. 1 in 216000 — rare enough that this
// test is not flaky. A time-seeded or counter-seeded generator collides inside
// 200 calls on Windows, where the clock granularity is ~15ms.
func TestNewDoesNotCollideOnFastSuccessiveCalls(t *testing.T) {
	const draws = 200
	seen := make(map[string]struct{}, draws)
	for i := 0; i < draws; i++ {
		id := ids.New("s")
		if _, dup := seen[id]; dup {
			t.Fatalf("New collided at draw %d: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestTokenIsUrlSafeAnd43Characters(t *testing.T) {
	got := ids.Token()
	if !tokenPattern.MatchString(got) {
		t.Fatalf("Token() = %q, want 43 URL-safe base64 characters (256 bits)", got)
	}
}

func TestTokensDiffer(t *testing.T) {
	a, b := ids.Token(), ids.Token()
	if a == b {
		t.Fatalf("two Token() calls returned the same secret %q", a)
	}
}
```

- [ ] **Step 2: Run the id tests and watch them fail**

Run: `go test ./control-plane/internal/ids/...`
Expected: FAIL — `no required module provides package github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids`.

- [ ] **Step 3: Write the id generator**

Create `control-plane/internal/ids/ids.go`:

```go
// Package ids mints identifiers and secrets. It exists so there is exactly one
// implementation: docs/FUNCTIONAL-SPEC.md known defect 2 is a home-rolled uid()
// that collides on fast successive calls, and the fix is not a better algorithm
// per call site, it is one call site.
//
// Prefixes, fixed across the product: "s-" session, "p-" project, "b-" branch,
// "w-" worktree, "m-" machine — except the local machine, whose id is the
// literal "local".
package ids

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// New returns prefix + "-" + 8 random lowercase hex characters, e.g.
// "s-3f2a91c0". 32 bits of entropy is ample for the tens-to-hundreds of
// sessions a laptop holds and keeps the id short enough to read out loud.
func New(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("ids: crypto/rand failed: " + err.Error())
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// Token returns a 256-bit URL-safe secret used as a sandbox's /attach bearer
// token. It is never logged and never returned over the REST surface.
func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("ids: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
```

`panic` is correct here and is not laziness: a failure of `crypto/rand` means the process cannot mint a secret, and continuing would mint a predictable one.

- [ ] **Step 4: Run the id tests and watch them pass**

Run: `go test ./control-plane/internal/ids/... -race -count=1 -v`
Expected: PASS, five tests.

- [ ] **Step 5: Write the failing backend tests**

Create `control-plane/internal/backend/backend_test.go`:

```go
package backend_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

func validSpec() backend.Spec {
	return backend.Spec{
		ID:          "s-3f2a91c0",
		SessionID:   "s-3f2a91c0",
		Image:       "panacea-app/sandbox:dev",
		Command:     "claude",
		Args:        []string{"--model", "opus", "add tests, then refactor"},
		AttachToken: "a-secret",
		Resources:   backend.DefaultResources,
	}
}

// The read limit is the whole reason Dial exists as a shared function. If this
// constant is ever lowered below sandboxd's 256 KiB replay frame, re-attach
// breaks at a size light testing never reaches.
func TestMaxMessageBytesIsAboveTheLibraryDefaultAndTheReplayFrame(t *testing.T) {
	const libraryDefault = 32768
	const replayFrame = 256*1024 + 1
	if backend.MaxMessageBytes <= libraryDefault {
		t.Fatalf("MaxMessageBytes = %d, must exceed coder/websocket's %d default", backend.MaxMessageBytes, libraryDefault)
	}
	if backend.MaxMessageBytes <= replayFrame {
		t.Fatalf("MaxMessageBytes = %d, must exceed the %d-byte replay frame", backend.MaxMessageBytes, replayFrame)
	}
}

func TestResourceConversions(t *testing.T) {
	r := backend.Resources{CPUMillis: 2000, MemoryMiB: 4096}
	if got := r.NanoCPUs(); got != 2_000_000_000 {
		t.Errorf("NanoCPUs() = %d, want 2000000000", got)
	}
	if got := r.MemoryBytes(); got != 4294967296 {
		t.Errorf("MemoryBytes() = %d, want 4294967296", got)
	}
	if got := r.KubeCPU(); got != "2000m" {
		t.Errorf("KubeCPU() = %q, want %q", got, "2000m")
	}
	if got := r.KubeMemory(); got != "4096Mi" {
		t.Errorf("KubeMemory() = %q, want %q", got, "4096Mi")
	}
}

func TestDefaultResourcesAreTheDocumentedOnes(t *testing.T) {
	if backend.DefaultResources.CPUMillis != 2000 || backend.DefaultResources.MemoryMiB != 4096 {
		t.Fatalf("DefaultResources = %+v, want {CPUMillis:2000 MemoryMiB:4096}", backend.DefaultResources)
	}
}

// The comma is the case PANACEA_APP_ARGS_JSON exists for. sandboxd's legacy
// -args form splits on commas and would turn one opening prompt into two
// arguments.
func TestArgsJSONSurvivesACommaInTheOpeningPrompt(t *testing.T) {
	s := validSpec()
	want := `["--model","opus","add tests, then refactor"]`
	if got := s.ArgsJSON(); got != want {
		t.Fatalf("ArgsJSON() = %s, want %s", got, want)
	}
}

// "[]" and not "": sandboxd treats an empty PANACEA_APP_ARGS_JSON as "fall back
// to the comma-separated form", which is the form this field replaces.
func TestArgsJSONOfNoArgsIsAnEmptyJSONArray(t *testing.T) {
	var s backend.Spec
	if got := s.ArgsJSON(); got != "[]" {
		t.Fatalf("ArgsJSON() = %q, want %q", got, "[]")
	}
}

func TestEnvEntriesPutsTheContractFirstAndSortsTheRest(t *testing.T) {
	s := validSpec()
	s.Env = map[string]string{"ZED": "3", "ALPHA": "1", "MID": "2"}
	got := s.EnvEntries()
	want := []backend.EnvEntry{
		{Name: "PANACEA_APP_COMMAND", Value: "claude"},
		{Name: "PANACEA_APP_ARGS_JSON", Value: `["--model","opus","add tests, then refactor"]`},
		{Name: "PANACEA_APP_ATTACH_TOKEN", Value: "a-secret"},
		{Name: "ALPHA", Value: "1"},
		{Name: "MID", Value: "2"},
		{Name: "ZED", Value: "3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvEntries() = %#v,\nwant %#v", got, want)
	}
}

func TestValidateSpecAcceptsAGoodSpec(t *testing.T) {
	if err := backend.ValidateSpec(validSpec()); err != nil {
		t.Fatalf("ValidateSpec(valid) = %v, want nil", err)
	}
}

// A caller that could set PANACEA_APP_ATTACH_TOKEN through Spec.Env could
// replace the sandbox's secret with one it chose.
func TestValidateSpecRejectsAReservedEnvKey(t *testing.T) {
	s := validSpec()
	s.Env = map[string]string{"PANACEA_APP_ATTACH_TOKEN": "mine"}
	err := backend.ValidateSpec(s)
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("ValidateSpec(reserved env) = %v, want an error wrapping ErrUnsupported", err)
	}
}

func TestValidateSpecRejectsUnusableFields(t *testing.T) {
	cases := map[string]func(*backend.Spec){
		"empty id":         func(s *backend.Spec) { s.ID = "" },
		"uppercase id":     func(s *backend.Spec) { s.ID = "S-3F2A91C0" },
		"id with a slash":  func(s *backend.Spec) { s.ID = "s-3f2a/91c0" },
		"over-long id":     func(s *backend.Spec) { s.ID = "s-000000000000000000000000000000000000000000" },
		"empty image":      func(s *backend.Spec) { s.Image = "" },
		"empty command":    func(s *backend.Spec) { s.Command = " " },
		"empty token":      func(s *backend.Spec) { s.AttachToken = "" },
		"zero cpu":         func(s *backend.Spec) { s.Resources.CPUMillis = 0 },
		"negative memory":  func(s *backend.Spec) { s.Resources.MemoryMiB = -1 },
	}
	for name, mangle := range cases {
		s := validSpec()
		mangle(&s)
		if err := backend.ValidateSpec(s); !errors.Is(err, backend.ErrUnsupported) {
			t.Errorf("%s: ValidateSpec = %v, want an error wrapping ErrUnsupported", name, err)
		}
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	all := []error{
		backend.ErrNotFound, backend.ErrNotRunning, backend.ErrAlreadyExists,
		backend.ErrUnavailable, backend.ErrUnauthorized, backend.ErrUnsupported,
		backend.ErrNonLoopbackPublish,
	}
	for i, a := range all {
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d and %d are not distinguishable: %v / %v", i, j, a, b)
			}
		}
	}
}
```

- [ ] **Step 6: Run the backend tests and watch them fail**

Run: `go test ./control-plane/internal/backend/...`
Expected: FAIL — `no required module provides package github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend`.

- [ ] **Step 7: Write the interface and the shared types**

Create `control-plane/internal/backend/backend.go`:

```go
// Package backend is the seam between the control plane and whatever actually
// runs a sandbox. Two implementations exist: dockerbe (a laptop, and the
// product's genuine "local machine" feature, per docs/PROGRAM.md) and katabe
// (AKS pod sandboxing, dormant until Phase 8).
//
// Attach returns *websocket.Conn and not io.ReadWriteCloser, deliberately. The
// wire protocol is message-framed: a 32 KiB stdout frame split across two reads
// of a byte stream decodes as two frames, the second with a garbage kind byte.
// proxy.Relay already takes *websocket.Conn.
package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// MaxMessageBytes raises coder/websocket's 32 KiB default read limit.
//
// SHARED RULE, THREE SITES. github.com/coder/websocket v1.8.12 caps a single
// inbound message at 32768 bytes (read.go:107). Two messages in this design
// exceed it: sandboxd's replay frame (up to 256 KiB of scrollback plus a kind
// byte) and a large paste, which arrives as one Stdin frame. Every
// *websocket.Conn on the Go side of the path must call
// SetReadLimit(MaxMessageBytes) immediately after Accept or Dial. The three
// sites are sandboxd's /attach, backend.Dial below, and the control plane's
// client Accept. Miss one and the pipe breaks at a size light testing never
// reaches. Browser WebSocket has no such limit, so the desktop needs no change.
const MaxMessageBytes = 4 << 20

// The sandbox's HTTP surface, as served by sandboxd.
const (
	AttachPath    = "/attach"
	HealthPath    = "/healthz"
	ContainerPort = 8080
)

// Environment variables injected into every sandbox. This is the downstream
// contract with sandboxd:
//   - EnvCommand   selects the harness binary
//   - EnvArgsJSON  is a JSON array of strings, because the legacy comma-joined
//     form corrupts any opening prompt containing a comma
//   - EnvToken     is the bearer token /attach requires; without it sandboxd
//     answers 401. /healthz stays unauthenticated so readiness works.
const (
	EnvCommand  = "PANACEA_APP_COMMAND"
	EnvArgsJSON = "PANACEA_APP_ARGS_JSON"
	EnvToken    = "PANACEA_APP_ATTACH_TOKEN"
)

// envReservedPrefix is the namespace a caller may not write into. Spec.Env is
// user-influenced; the three variables above are the security contract.
const envReservedPrefix = "PANACEA_APP_"

// Labels attached to every sandbox this control plane creates. Docker labels
// use dots by convention; the Kata pod labels already in internal/sandbox use
// slashes, because Kubernetes label keys do. Both are listed so neither backend
// invents a third spelling.
const (
	LabelManaged   = "panacea-app.managed"    // always "true"
	LabelSandboxID = "panacea-app.sandbox-id" // equals Spec.ID
	LabelSessionID = "panacea-app.session-id" // equals registry.Session.ID
	LabelCreatedAt = "panacea-app.created-at" // RFC3339, diagnostics only

	KubeLabelSandboxID = "panacea-app/sandbox-id"
	KubeLabelSessionID = "panacea-app/session-id"
	KubeLabelComponent = "panacea-app/component" // always "sandbox"
)

// Kind names a backend implementation. It is persisted in the registry and
// returned over the REST surface, so these strings are stable.
type Kind string

const (
	KindDocker Kind = "docker"
	KindKata   Kind = "kata"
)

// State is what the backend can observe about a sandbox. It is NOT the session
// status the user sees — registry.Status is. A backend knows nothing about
// idleness, because idleness is a fact about the relay, not about the
// container.
type State string

const (
	StateStarting State = "starting" // created, not yet answering /healthz
	StateRunning  State = "running"
	StateStopped  State = "stopped" // exists, process ended
	StateGone     State = "gone"    // no such sandbox
)

// Resources sizes a sandbox. Kata requires explicit CPU and memory because the
// guest VM is sized from them and under-requesting causes guest OOM; Docker
// takes the same numbers in different units, so the conversion lives here once.
type Resources struct {
	CPUMillis int // 1000 = one core
	MemoryMiB int
}

// DefaultResources is what a session gets when nothing overrides it.
var DefaultResources = Resources{CPUMillis: 2000, MemoryMiB: 4096}

// NanoCPUs renders CPUMillis for container.Resources.NanoCPUs.
func (r Resources) NanoCPUs() int64 { return int64(r.CPUMillis) * 1_000_000 }

// MemoryBytes renders MemoryMiB for container.Resources.Memory.
func (r Resources) MemoryBytes() int64 { return int64(r.MemoryMiB) << 20 }

// KubeCPU renders CPUMillis as a Kubernetes quantity, e.g. "2000m".
func (r Resources) KubeCPU() string { return strconv.Itoa(r.CPUMillis) + "m" }

// KubeMemory renders MemoryMiB as a Kubernetes quantity, e.g. "4096Mi".
func (r Resources) KubeMemory() string { return strconv.Itoa(r.MemoryMiB) + "Mi" }

// Mount is a host path exposed inside the sandbox. Phase 2 uses it for one
// thing: giving a local Docker session the worktree it is supposed to work in.
// Kata ignores Mounts and returns ErrUnsupported if any are set, because a host
// path on an AKS node is meaningless.
type Mount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// Spec describes one sandbox to create. Every field is set by the caller; a
// backend never defaults anything except where documented.
type Spec struct {
	ID          string            // caller-chosen, unique, matches ^[a-z0-9-]{1,40}$
	SessionID   string            // for the label only; a backend never reads it back
	Image       string            // e.g. "panacea-app/sandbox:dev"
	Command     string            // becomes PANACEA_APP_COMMAND
	Args        []string          // JSON-encoded into PANACEA_APP_ARGS_JSON
	Env         map[string]string // extra environment; PANACEA_APP_* keys are rejected
	AttachToken string            // becomes PANACEA_APP_ATTACH_TOKEN; never logged
	Resources   Resources
	WorkDir     string  // container-side working directory, "" leaves the image default
	Mounts      []Mount // docker only
}

// EnvEntry is one environment variable, in order.
type EnvEntry struct {
	Name  string
	Value string
}

// ArgsJSON encodes Args for PANACEA_APP_ARGS_JSON.
//
// No arguments encodes as "[]" and never as "": sandboxd treats an empty
// PANACEA_APP_ARGS_JSON as "fall back to the legacy comma-separated form", and
// that form splits an opening prompt on its commas.
func (s Spec) ArgsJSON() string {
	if len(s.Args) == 0 {
		return "[]"
	}
	b, err := json.Marshal(s.Args)
	if err != nil {
		// json.Marshal of a []string cannot fail. The branch exists so that if
		// Args ever stops being a []string the failure is an empty argv rather
		// than a corrupted one.
		return "[]"
	}
	return string(b)
}

// EnvEntries returns the sandbox's environment in a fixed order: the three
// PANACEA_APP_* variables first, then every Spec.Env key sorted. The order is
// fixed so both backends produce identical environments and a test can assert
// on a slice instead of a set.
func (s Spec) EnvEntries() []EnvEntry {
	out := make([]EnvEntry, 0, 3+len(s.Env))
	out = append(out,
		EnvEntry{Name: EnvCommand, Value: s.Command},
		EnvEntry{Name: EnvArgsJSON, Value: s.ArgsJSON()},
		EnvEntry{Name: EnvToken, Value: s.AttachToken},
	)
	keys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, EnvEntry{Name: k, Value: s.Env[k]})
	}
	return out
}

// idPattern is the sandbox id both backends accept. Docker container names and
// Kubernetes object names are both built from it, and both are stricter than a
// free-text string.
var idPattern = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

// ValidateSpec rejects a Spec no backend can honour. Both implementations call
// it as the first statement of Create, so "docker accepted it and kata did not"
// cannot happen. Every failure wraps ErrUnsupported, which the API layer maps
// to 400.
func ValidateSpec(spec Spec) error {
	if !idPattern.MatchString(spec.ID) {
		return fmt.Errorf("spec.ID %q must match ^[a-z0-9-]{1,40}$: %w", spec.ID, ErrUnsupported)
	}
	if strings.TrimSpace(spec.Image) == "" {
		return fmt.Errorf("spec.Image is empty: %w", ErrUnsupported)
	}
	if strings.TrimSpace(spec.Command) == "" {
		return fmt.Errorf("spec.Command is empty: %w", ErrUnsupported)
	}
	if spec.AttachToken == "" {
		return fmt.Errorf("spec.AttachToken is empty: an unauthenticated /attach is a shell any local process can reach: %w", ErrUnsupported)
	}
	for k := range spec.Env {
		if strings.HasPrefix(k, envReservedPrefix) {
			return fmt.Errorf("spec.Env key %q is reserved: a caller that could set %s could replace the sandbox's secret: %w", k, EnvToken, ErrUnsupported)
		}
	}
	if spec.Resources.CPUMillis <= 0 || spec.Resources.MemoryMiB <= 0 {
		return fmt.Errorf("spec.Resources must be positive, got %d millicpu and %d MiB: %w", spec.Resources.CPUMillis, spec.Resources.MemoryMiB, ErrUnsupported)
	}
	return nil
}

// Sandbox is what a backend knows about one sandbox.
//
// Endpoint is host:port as reachable FROM THE CONTROL PLANE, not from the
// desktop. The desktop never learns it — it attaches through
// /v1/sessions/{id}/attach. For dockerbe it is 127.0.0.1:<published port>; for
// katabe it is <podIP>:8080.
type Sandbox struct {
	ID        string
	Kind      Kind
	Ref       string // container id, or pod name
	Endpoint  string
	State     State
	CreatedAt time.Time
	Labels    map[string]string
}

// Backend creates, finds, attaches to and destroys sandboxes.
//
// Attach takes the token explicitly rather than reading it back from the
// sandbox, so the secret lives in exactly one durable place (the registry) and
// is not recoverable by anything that can merely list containers.
type Backend interface {
	Kind() Kind
	Create(ctx context.Context, spec Spec) (Sandbox, error)
	Get(ctx context.Context, id string) (Sandbox, error)
	List(ctx context.Context) ([]Sandbox, error)
	Attach(ctx context.Context, id, token string) (*websocket.Conn, error)
	Destroy(ctx context.Context, id string) error
	Close() error
}

// Sentinel errors. Every implementation wraps these with %w; the API layer maps
// them to status codes with errors.Is and never string-matches.
var (
	// ErrNotFound: no sandbox with that id. Maps to 404 session_not_found when
	// the registry also has no record, or 409 sandbox_not_running when it does.
	ErrNotFound = errors.New("backend: sandbox not found")
	// ErrNotRunning: the sandbox exists but is not accepting connections. 409.
	ErrNotRunning = errors.New("backend: sandbox is not running")
	// ErrAlreadyExists: Spec.ID is taken. 409 conflict.
	ErrAlreadyExists = errors.New("backend: sandbox already exists")
	// ErrUnavailable: the backend itself is down (no Docker daemon, no
	// kubeconfig). 503 backend_unavailable.
	ErrUnavailable = errors.New("backend: backend is unavailable")
	// ErrUnauthorized: /attach answered 401, i.e. the stored token is wrong. 500.
	ErrUnauthorized = errors.New("backend: attach token rejected")
	// ErrUnsupported: this backend cannot honour part of the Spec. 400.
	ErrUnsupported = errors.New("backend: unsupported by this backend")
	// ErrNonLoopbackPublish: the sandbox port was published to something other
	// than 127.0.0.1. Refusing is the point: a published 0.0.0.0 port is a
	// shell on the LAN. 500, and the sandbox is destroyed.
	ErrNonLoopbackPublish = errors.New("backend: sandbox port is not bound to 127.0.0.1")
)
```

- [ ] **Step 8: Promote the websocket module to a direct requirement**

`control-plane/go.mod` currently lists `github.com/coder/websocket v1.8.12 // indirect`. `backend/dial.go` imports it directly in the next step. Move that one line out of the indirect `require` block and into the first `require` block, dropping the `// indirect` comment, so the file describes reality:

```
require (
	github.com/coder/websocket v1.8.12
	k8s.io/api v0.31.1
	k8s.io/apimachinery v0.31.1
	k8s.io/client-go v0.31.1
)
```

This is a text edit only. Do **not** run `go mod tidy` or `go get` — the module and its checksum are already in `control-plane/go.sum`, and the proxy is TLS-intercepted from this network. Confirm afterwards that the `go 1.23` directive is untouched: `git diff control-plane/go.mod`.

- [ ] **Step 9: Write the failing dialer tests**

Create `control-plane/internal/backend/dial_test.go`:

```go
package backend_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// fakeSandbox is sandboxd's HTTP surface, reduced to what Dial and WaitReady
// touch: an unauthenticated /healthz and a bearer-guarded /attach.
type fakeSandbox struct {
	token      string
	healthCode atomic.Int32 // what /healthz answers
	gotAuth    atomic.Value // string: the Authorization header /attach saw
	// bigPayload, when non-empty, is written to the client immediately after
	// the upgrade. It exists to prove the read limit was raised.
	bigPayload []byte
}

func newFakeSandbox(t *testing.T, token string) (*fakeSandbox, string) {
	t.Helper()
	fs := &fakeSandbox{token: token}
	fs.healthCode.Store(int32(http.StatusOK))
	fs.gotAuth.Store("")

	mux := http.NewServeMux()
	mux.HandleFunc(backend.HealthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(fs.healthCode.Load()))
	})
	mux.HandleFunc(backend.AttachPath, func(w http.ResponseWriter, r *http.Request) {
		fs.gotAuth.Store(r.Header.Get("Authorization"))
		if fs.token != "" && r.Header.Get("Authorization") != "Bearer "+fs.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		if len(fs.bigPayload) > 0 {
			_ = c.Write(r.Context(), websocket.MessageBinary, fs.bigPayload)
		}
		// Hold the connection open until the client goes away.
		_, _, _ = c.Read(r.Context())
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return fs, strings.TrimPrefix(srv.URL, "http://")
}

func TestDialSendsTheBearerTokenAndOpensTheSocket(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "sekrit")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := backend.Dial(ctx, endpoint, "sekrit")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if got := fs.gotAuth.Load().(string); got != "Bearer sekrit" {
		t.Fatalf("/attach saw Authorization %q, want %q", got, "Bearer sekrit")
	}
}

// sandboxd answers 401 without the token. That must surface as a sentinel the
// API layer can switch on, not as an opaque handshake failure.
func TestDialWithTheWrongTokenIsErrUnauthorized(t *testing.T) {
	_, endpoint := newFakeSandbox(t, "sekrit")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := backend.Dial(ctx, endpoint, "wrong")
	if !errors.Is(err, backend.ErrUnauthorized) {
		t.Fatalf("Dial with a wrong token = %v, want an error wrapping ErrUnauthorized", err)
	}
}

func TestDialToAClosedPortIsErrNotRunning(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	endpoint := ln.Addr().String()
	ln.Close() // nothing is listening there now

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = backend.Dial(ctx, endpoint, "sekrit")
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Dial to a closed port = %v, want an error wrapping ErrNotRunning", err)
	}
}

// The reason Dial exists. coder/websocket caps one inbound message at 32768
// bytes; sandboxd's replay frame is up to 256 KiB. Without SetReadLimit this
// read fails with "read limited at 32769 bytes" and the session looks dead.
func TestDialRaisesTheReadLimitAboveTheLibraryDefault(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.bigPayload = make([]byte, 100*1024)
	for i := range fs.bigPayload {
		fs.bigPayload[i] = byte('a' + i%26)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := backend.Dial(ctx, endpoint, "")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read of a 100 KiB message: %v (SetReadLimit is missing or runs after the first Read)", err)
	}
	if len(data) != len(fs.bigPayload) {
		t.Fatalf("read %d bytes, want %d", len(data), len(fs.bigPayload))
	}
}

func TestWaitReadyReturnsOnceHealthzAnswers200(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.healthCode.Store(int32(http.StatusServiceUnavailable))

	go func() {
		time.Sleep(600 * time.Millisecond)
		fs.healthCode.Store(int32(http.StatusOK))
	}()

	if err := backend.WaitReady(context.Background(), endpoint, 10*time.Second); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
}

func TestWaitReadyTimesOutWithErrNotRunning(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.healthCode.Store(int32(http.StatusServiceUnavailable))

	err := backend.WaitReady(context.Background(), endpoint, 700*time.Millisecond)
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("WaitReady on a sandbox that never comes up = %v, want an error wrapping ErrNotRunning", err)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("WaitReady error %q does not name the last status; the caller cannot tell a 503 from a refused connection", err)
	}
}

// A cancelled context must abandon the poll promptly, not after the next
// interval: this runs while a user is watching a session that will not start.
func TestWaitReadyStopsWhenTheContextIsCancelled(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.healthCode.Store(int32(http.StatusServiceUnavailable))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := backend.WaitReady(ctx, endpoint, 30*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitReady after cancel = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("WaitReady took %s to notice a cancelled context", elapsed)
	}
}
```

- [ ] **Step 10: Run the dialer tests and watch them fail**

Run: `go test ./control-plane/internal/backend/...`
Expected: FAIL to build — `undefined: backend.Dial`, `undefined: backend.WaitReady`.

- [ ] **Step 11: Write the dialer**

Create `control-plane/internal/backend/dial.go`:

```go
package backend

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// DialTimeout bounds the /attach handshake. A sandbox that has answered
// /healthz should upgrade in milliseconds; ten seconds is a generous ceiling
// that still fails fast enough for a UI.
const DialTimeout = 10 * time.Second

// ReadyTimeout is how long a caller waits for a new sandbox to answer
// /healthz. A cold Docker image start on a laptop is seconds; docs/PROGRAM.md
// records Kata cold start as seconds too, which is why warm pools are
// mandatory in Phase 8.
const ReadyTimeout = 60 * time.Second

// readyPollInterval is deliberately short: the difference between a 250ms and
// a 1s poll is the difference between a session that feels instant and one
// that does not.
const readyPollInterval = 250 * time.Millisecond

// Dial opens the sandbox's /attach WebSocket.
//
// endpoint is host:port. token, when non-empty, is sent as
// "Authorization: Bearer <token>"; sandboxd answers 401 without it. The
// returned connection has already had SetReadLimit(MaxMessageBytes) applied —
// this is one of the three sites named in the MaxMessageBytes doc comment, and
// the caller must not have to remember.
func Dial(ctx context.Context, endpoint, token string) (*websocket.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, DialTimeout)
	defer cancel()

	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if token != "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+token)
	}

	conn, resp, err := websocket.Dial(dialCtx, "ws://"+endpoint+AttachPath, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("dial %s: %w", endpoint, ErrUnauthorized)
		}
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			return nil, fmt.Errorf("dial %s: %v: %w", endpoint, err, ErrNotRunning)
		}
		return nil, fmt.Errorf("dial %s: %w", endpoint, err)
	}
	conn.SetReadLimit(MaxMessageBytes)
	return conn, nil
}

// WaitReady polls GET http://endpoint/healthz until it answers 200 or the
// timeout expires. /healthz is deliberately unauthenticated, so this needs no
// token.
//
// It exists because a container start returning nil means "the process was
// launched", not "the server is listening". Attaching before that produces a
// connection-refused that looks, in the UI, exactly like a crashed agent.
func WaitReady(ctx context.Context, endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	url := "http://" + endpoint + HealthPath

	var last error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("healthz returned %d", resp.StatusCode)
		} else {
			last = err
		}

		// A plain time.Sleep here would keep polling a sandbox nobody is
		// waiting for any more, for up to one interval past the cancel.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyPollInterval):
		}
	}
	return fmt.Errorf("sandbox %s never became ready (last: %v): %w", endpoint, last, ErrNotRunning)
}
```

- [ ] **Step 12: Run the whole backend and ids suite and watch it pass**

Run: `go test ./control-plane/internal/ids/... ./control-plane/internal/backend/... -race -count=1 -v`
Expected: PASS — five `ids` tests, ten `backend` tests, no race reports.
If `TestDialRaisesTheReadLimitAboveTheLibraryDefault` fails with `read limited at 32769 bytes`, the `conn.SetReadLimit(MaxMessageBytes)` line is missing or sits after the first `conn.Read`.

- [ ] **Step 13: Run the full Go suite**

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`
Expected: every package `ok` or `[no test files]`. Never `go test ./...` — the workspace root is not a module and that form errors with `directory prefix . does not contain modules listed in go.work`.

Then confirm the Go directive was not bumped: `git diff control-plane/go.mod` must show only the `coder/websocket` line moving, and `go 1.23` unchanged.

- [ ] **Step 14: Commit**

```bash
git add control-plane/internal/ids control-plane/internal/backend control-plane/go.mod
git commit -m "feat(backend): the sandbox seam, one dialer, one id generator

Backend is the interface docs/PROGRAM.md's local-first pivot requires: one
contract, two real implementations. Dial is a shared function rather than a
line in each backend because every Go-side websocket.Conn must raise
coder/websocket's 32 KiB read limit, and sandboxd's replay frame is 256 KiB.
ids replaces the home-rolled uid() of known defect 2 with crypto/rand.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

<!-- PLAN-CONTINUES: tasks 5-26 pending; see docs/superpowers/contracts/ for the locked contracts -->

