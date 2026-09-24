# Phase 2 — the Go-side contract for Groups B and C

> **Status: binding.** Every name, type, field, route, status code and environment
> variable below is fixed. Task authors for tasks 4–13 restate the parts their task
> touches inside that task's `Interfaces:` block — a fresh subagent sees only one
> task, so nothing here may be referenced by pointer ("see the contract"); it must
> be copied into the task.
>
> Where this document contradicts the **File structure** table at line 66 of
> `docs/superpowers/plans/2026-09-18-phase-2-backend-and-cockpit.md`, this document
> wins, and §1.2 says why.

**What was verified, and how.** Everything asserted about a third-party API below was
read out of the module cache at `C:\Users\au-dev-dmackie1\go\pkg\mod` or out of a real
binary on this machine. Nothing here is recalled.

| Claim | Checked against |
|---|---|
| `github.com/docker/docker` v28.3.3 client signatures | `…/docker@v28.3.3+incompatible/client/client_interfaces.go` lines 70–103 |
| `container.Summary`, `container.InspectResponse`, `container.Port` | `…/api/types/container/container.go` lines 123, 181; `port.go` |
| `HostConfig.PortBindings nat.PortMap`, embedded `Resources{Memory, NanoCPUs}` | `…/api/types/container/hostconfig.go` lines 430, 374–375, 468 |
| `container.RestartPolicyDisabled = "no"` | `…/api/types/container/hostconfig.go` line 284 |
| `Ping(ctx) (types.Ping, error)` | `…/client/ping.go` line 21 |
| `filters.NewArgs(filters.Arg(k,v))` | `…/api/types/filters/parse.go` lines 27, 32 |
| `nat.Port`, `nat.PortMap`, `nat.PortBinding` | `go-connections@v0.5.0/nat/nat.go` lines 12–26 |
| coder/websocket's origin check is host-only and permits a **missing** Origin | `coder/websocket@v1.8.12/accept.go` lines 206–234 |
| Tauri webview origin string | tauri `dev` branch, `crates/tauri/src/manager/webview.rs` (`window_origin`) and `crates/tauri-utils/src/config.rs` (`use_https_scheme`, "Defaults to `false`") |
| Claude Code flags | `claude --help` on this machine, version **2.1.267**; the sandbox image pins **2.1.274** |
| `--session-id`, `--fork-session`, where transcripts live — AMENDED by the resume fix (2026-09-23) | `claude --help` and the CLI's behaviour **inside `panacea-app/sandbox:dev`, version 2.1.274**: transcript at `~/.claude/projects/-home-dev-work/<uuid>.jsonl`; `--session-id` of an existing conversation → `Session ID <id> is already in use`; `--resume A --session-id B` without `--fork-session` → refused; `--resume A --fork-session --session-id B` → B written with A's history; `--resume <missing>` → `No conversation found with session ID: <id>` |

**Not verified, and marked as such throughout:** Codex, OpenCode and Gemini CLI
flags. None of those binaries exists on this machine, and
`images/sandbox/Dockerfile` installs only `@anthropic-ai/claude-code@2.1.274`. See
§2.9 and §8.

---

## 1. Package layout

### 1.1 Files Groups B and C create or modify

| Path | One-line responsibility | Task |
|---|---|---|
| `control-plane/internal/ids/ids.go` | Collision-free ids and attach tokens from `crypto/rand` | 4 |
| `control-plane/internal/backend/backend.go` | `Backend` interface, `Spec`, `Sandbox`, `Resources`, sentinel errors, shared constants | 4 |
| `control-plane/internal/backend/dial.go` | The one `/attach` dialer: bearer header, `SetReadLimit`, dial timeout; plus `WaitReady` | 4 |
| `control-plane/internal/backend/dockerbe/api.go` | The narrow slice of the Docker SDK this package uses, so tests can fake it | 5 |
| `control-plane/internal/backend/dockerbe/docker.go` | `Backend` over Docker: create, start, publish to `127.0.0.1`, label discovery, remove | 5 |
| `control-plane/internal/backend/katabe/kata.go` | `Backend` over `internal/sandbox.Manager`. Dormant until Phase 8; compiles and is tested now | 6 |
| `control-plane/internal/sandbox/manager.go` | **modified** — `Spec` grows args/env/token; adds `Get` and `List` | 6 |
| `control-plane/internal/statepath/statepath.go` | Where on-disk state lives, per OS. One function, shared with Phase 4 | 7 |
| `control-plane/internal/registry/session.go` | The `Session` record and its `Status` | 7 |
| `control-plane/internal/registry/registry.go` | `Store` interface, in-memory index, `Update` as the only mutation primitive | 7 |
| `control-plane/internal/registry/filestore.go` | Single-document JSON persistence, schema version, atomic replace that survives Windows | 7 |
| `control-plane/internal/harness/harness.go` | The catalogue: which harnesses exist, which are installed, which options each accepts | 8 |
| `control-plane/internal/harness/argv.go` | `Build` — harness + model + effort + permission mode + prompt → `(command, argv)` | 8 |
| `control-plane/internal/worktree/worktree.go` | **Phase 3 seam.** `Provider` interface + the `Static` implementation Phase 2 ships | 8 |
| `control-plane/internal/project/project.go` | Read-only enumeration of projects, branches and worktrees via `git`. Implements `worktree.Provider` and Phase 4's `Naming` | 8 |
| `control-plane/internal/httpx/problem.go` | The one error body, and the JSON read/write helpers every handler uses | 9 |
| `control-plane/internal/httpx/origin.go` | Origin allowlist + loopback `Host` guard, as middleware and as a predicate | 9 |
| `control-plane/internal/api/api.go` | `Deps`, `New`, the `http.ServeMux`, the `/api/v1` alias | 10 |
| `control-plane/internal/api/catalog.go` | `/healthz`, `/machines`, `/projects`, `/branches`, `/worktrees`, `/harnesses` | 10 |
| `control-plane/internal/api/sessions.go` | Session create / list / get / patch / delete / restart | 11 |
| `control-plane/internal/api/attach.go` | The WebSocket upgrade, origin check, read limit, relay, last-output accounting | 12 |
| `control-plane/internal/proxy/proxy.go` | **modified** — `RelayHooked` adds per-message byte counters; `Relay` keeps its signature | 12 |
| `control-plane/internal/reaper/reaper.go` | The periodic sweep: orphans, disappeared sandboxes, idle derivation, idle TTL. AMENDED by the reaper-scope fix (2026-09-24): destroys only this instance's sandboxes (§5.3) | 13 |
| `control-plane/internal/instance/instance.go` | **added by the reaper-scope fix (2026-09-24)** — this state directory's instance id: minted once, persisted, read back (§2.7) | — |
| `control-plane/cmd/api/config.go` | Every `PANACEA_APP_*` variable, parsed once, with defaults | 13 |
| `control-plane/cmd/api/main.go` | **modified** — wiring, restart recovery, listener, graceful shutdown | 13 |

### 1.2 Corrections to the plan's File structure table, and why

1. **`internal/api/api.go` is not one file.** The plan lists a single `api.go` for
   "REST + attach handlers". Three tasks (10, 11, 12) write into it; one file
   guarantees three subagents collide in the same region. `api.go` owns only `Deps`,
   `New` and routing; each later task adds its own file.
2. **`dockerbe` gains `api.go`.** The Docker backend must be unit-testable with no
   Docker daemon. That needs a narrow interface that the real `*client.Client`
   satisfies structurally, in its own file, so the fake in `docker_test.go` has
   something stable to implement.
3. **`registry` is three files.** Record, store and persistence are separable, and the
   persistence one holds the only Windows-specific code in the phase.
4. **`internal/statepath` is missing from the table and is required.** Phase 4's
   `discoverstore.DefaultDir()` (phase-4 plan line 3200) already hard-codes the
   `PANACEA_APP_STATE_DIR` → `LOCALAPPDATA` → `XDG_STATE_HOME` → `~/.local/state`
   ladder. Phase 2's registry needs the identical ladder. Two copies drift; one copy
   is a package. See §8.1.
5. **`internal/project` is missing from the table and is required.** The REST surface
   owes the desktop `projects`, `branches` and `worktrees`
   (`docs/FUNCTIONAL-SPEC.md` "Navigation", plan Group D), and Phase 4 needs a
   `Naming` implementation. Nothing in the table produces any of it.
6. **`internal/ids` is missing from the table and is required.** Session ids and
   per-sandbox attach tokens are generated in two packages. Known defect 2 in
   `docs/FUNCTIONAL-SPEC.md` is precisely a home-rolled id generator colliding: one
   package, one implementation, `crypto/rand`.
7. **`internal/reaper` is missing from the table.** The plan puts the reaper in
   `cmd/api/main.go`. A reaper inside `main` cannot be unit-tested, and its rules are
   the part most likely to destroy a running agent by mistake. It gets a package and a
   test.
8. **`internal/proxy/proxy.go` becomes "modified", not untouched.** Deriving `idle`
   needs a count of messages relayed. Counting is not reading — the hook receives
   `len(data)` and never the bytes. That is the only change.

### 1.3 What Groups B and C deliberately do not create

No database, no migrations, no ORM, no authentication. The control plane binds to
loopback and the registry is a JSON file. `docs/PROGRAM.md` puts identity in Phase 8,
and adding a SQL driver to a module pinned at `go 1.23` is a dependency risk this
phase has no reason to take.

---
## 2. Exact Go types

Everything in this section is the real, compiling declaration. A task author copies
the block verbatim; the executing engineer writes the bodies.

### 2.1 `internal/ids` — one id generator for the whole control plane

```go
// Package ids mints identifiers and secrets. It exists so there is exactly one
// implementation: docs/FUNCTIONAL-SPEC.md known defect 2 is a home-rolled uid()
// that collides on fast successive calls, and the fix is not a better algorithm
// per call site, it is one call site.
package ids

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// New returns prefix + "-" + 8 random lowercase hex characters, e.g. "s-3f2a91c0".
// 32 bits of entropy is ample for the tens-to-hundreds of sessions a laptop holds
// and keeps the id short enough to read out loud.
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

`panic` is correct here: a failure of `crypto/rand` means the process cannot mint a
secret, and continuing would mint a predictable one.

**Id prefixes, fixed:** `s-` session, `p-` project, `b-` branch, `w-` worktree,
`m-` machine (except the local machine, whose id is the literal `local`).

### 2.2 `internal/backend` — the interface

```go
// Package backend is the seam between the control plane and whatever actually
// runs a sandbox. Two implementations exist: dockerbe (a laptop, and the
// product's genuine "local machine" feature) and katabe (AKS pod sandboxing,
// dormant until Phase 8).
//
// Attach returns *websocket.Conn and not io.ReadWriteCloser, deliberately. The
// wire protocol is message-framed: a 32 KiB stdout frame split across two reads
// of a byte stream decodes as two frames, the second with a garbage kind byte.
// proxy.Relay already takes *websocket.Conn.
package backend

import (
	"context"
	"errors"
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
// sites are sandboxd's /attach (Group A task 3), backend.Dial (task 4) and the
// control plane's client Accept (task 12). Miss one and the pipe breaks at a
// size light testing never reaches. Browser WebSocket has no such limit, so the
// desktop needs no change.
const MaxMessageBytes = 4 << 20

// The sandbox's HTTP surface, as served by sandboxd.
const (
	AttachPath    = "/attach"
	HealthPath    = "/healthz"
	ContainerPort = 8080
)

// Environment variables injected into every sandbox. This is the downstream
// contract with sandboxd, fixed by Group A task 3:
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

// Labels attached to every sandbox this control plane creates. Docker labels use
// dots by convention; the Kata pod labels already in internal/sandbox use
// slashes, because Kubernetes label keys do. Both are listed so neither backend
// invents a third spelling.
const (
	LabelManaged   = "panacea-app.managed"    // always "true"
	LabelSandboxID = "panacea-app.sandbox-id" // equals Spec.ID
	LabelSessionID = "panacea-app.session-id" // equals registry.Session.ID
	LabelCreatedAt = "panacea-app.created-at" // RFC3339, used only for diagnostics

	KubeLabelSandboxID = "panacea-app/sandbox-id"
	KubeLabelSessionID = "panacea-app/session-id"
	KubeLabelComponent = "panacea-app/component" // always "sandbox"
)
```

AMENDED by the reaper-scope fix (2026-09-24, `task-25-26-reconcile.md` item 11): **ownership
is per control plane, and it is a label.** `LabelManaged` says "a Panacea control plane made
this"; it cannot say which one, so two control planes on one Docker daemon — two users of one
shared host, or a cockpit beside a test harness — read each other's live sandboxes as orphans
(§5.3 rule 4) and destroyed them at the first sweep. Two labels join the block above, one per
spelling, and a validator:

```go
	LabelInstance     = "panacea-app.instance" // the owning control plane's instance id
	KubeLabelInstance = "panacea-app/instance"

// ValidateInstance rejects an id no backend can stamp: it must match
// ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$ (a Kubernetes label value, lowercase).
// Empty is refused, not defaulted: an empty instance would own every sandbox
// that also carries none, which is every sandbox made before the label.
func ValidateInstance(id string) error // wraps ErrUnsupported
```

The id is **this state directory's**, minted once and persisted (§2.7 amendment,
`internal/instance`). Every sandbox is stamped with it (§2.4, §2.6), `List` returns only
sandboxes that carry this instance's id, and the reaper destroys only those (§5.3). A managed
sandbox with **no** instance label predates this amendment; nothing can prove whose it is, so
nothing destroys it automatically — it is logged once as foreign.

```go
// Kind names a backend implementation. It is persisted in the registry and
// returned over the REST surface, so these strings are stable.
type Kind string

const (
	KindDocker Kind = "docker"
	KindKata   Kind = "kata"
)

// State is what the backend can observe about a sandbox. It is NOT the session
// status the user sees — registry.Status is. A backend knows nothing about
// idleness, because idleness is a fact about the relay, not about the container.
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

// Mount is a host path exposed inside the sandbox. Phase 2 uses it for three
// things: giving a local Docker session the worktree it is supposed to work in,
// and — AMENDED by the resume fix (2026-09-23) — giving a session whose harness has a
// conversation that worktree's conversation store (§5.1 step 5), and — AMENDED by
// the sandbox git fix (2026-09-23) — the operator's PANACEA_APP_CA_BUNDLE,
// read-only (§4), and — AMENDED by its round 1 (2026-09-24) — a primary
// worktree's .git/worktrees, read-only over itself, so nothing in the sandbox
// can prune or rewrite the host's linked-worktree metadata (§5.1 step 5), and —
// AMENDED by its fix round (gitfix-rereview I-A, 2026-09-24) — the host git's global
// excludes and attributes files, read-only, each when the host's git reads one, so a
// commit in the sandbox holds what it would on the host (§5.1 step 5).
// Five things, then. Kata ignores Mounts and returns ErrUnsupported if any are set,
// because a host path on an AKS node is meaningless (§2.6 lists what Phase 8 must
// supply instead).
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
	Env         map[string]string // extra environment; keys starting PANACEA_APP_ are rejected
	AttachToken string            // becomes PANACEA_APP_ATTACH_TOKEN; never logged
	Resources   Resources
	WorkDir     string  // container-side working directory, "" leaves the image default
	Mounts      []Mount // docker only
}

// Sandbox is what a backend knows about one sandbox.
//
// Endpoint is host:port as reachable FROM THE CONTROL PLANE, not from the
// desktop. The desktop never learns it — it attaches through /v1/sessions/{id}/attach.
// For dockerbe it is 127.0.0.1:<published port>; for katabe it is <podIP>:8080.
type Sandbox struct {
	ID        string
	Kind      Kind
	Ref       string // container id, or pod name
	Endpoint  string
	State     State
	CreatedAt time.Time
	Labels    map[string]string
}

// AMENDED by the reaper-scope fix (2026-09-24): one more field. Each backend
// reads it from its own spelling of the label, so the reaper checks ownership
// without knowing which backend it sweeps. Empty means "no instance label".
// (The code also carries NonLoopback bool, task 5's amendment to §2.4, which
// this listing never showed; backend.go's Sandbox is the authority.)
//
//	Instance string // LabelInstance / KubeLabelInstance

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
	// ErrNotFound: no sandbox with that id. → 404 session_not_found (when the
	// registry also has no record) or 409 sandbox_not_running (when it does).
	ErrNotFound = errors.New("backend: sandbox not found")
	// ErrNotRunning: the sandbox exists but is not accepting connections. → 409.
	ErrNotRunning = errors.New("backend: sandbox is not running")
	// ErrAlreadyExists: Spec.ID is taken. → 409 conflict.
	ErrAlreadyExists = errors.New("backend: sandbox already exists")
	// ErrUnavailable: the backend itself is down (no Docker daemon, no
	// kubeconfig). → 503 backend_unavailable.
	ErrUnavailable = errors.New("backend: backend is unavailable")
	// ErrUnauthorized: /attach answered 401, i.e. the stored token is wrong. → 500.
	ErrUnauthorized = errors.New("backend: attach token rejected")
	// ErrUnsupported: this backend cannot honour part of the Spec. → 400.
	ErrUnsupported = errors.New("backend: unsupported by this backend")
	// ErrNonLoopbackPublish: the sandbox port was published to something other
	// than 127.0.0.1. Refusing is the point: a published 0.0.0.0 port is a shell
	// on the LAN. → 500, and the sandbox is destroyed.
	ErrNonLoopbackPublish = errors.New("backend: sandbox port is not bound to 127.0.0.1")
)
```

`strconv` is imported by `backend.go`; the block above omits it from the import list
only because the two methods that use it appear later in the file. The task author
must include `"strconv"` in the import block.

### 2.3 `internal/backend/dial.go` — the shared `/attach` dialer

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

// ReadyTimeout is how long Create waits for a new sandbox to answer /healthz.
// A cold Docker image start on a laptop is seconds; Kata is documented in
// docs/PROGRAM.md as "seconds" too, which is why warm pools are mandatory in
// Phase 8.
const ReadyTimeout = 60 * time.Second

// readyPollInterval is deliberately short: the difference between a 250ms and a
// 1s poll is the difference between a session that feels instant and one that
// does not.
const readyPollInterval = 250 * time.Millisecond

// Dial opens the sandbox's /attach WebSocket.
//
// endpoint is host:port. token, when non-empty, is sent as
// "Authorization: Bearer <token>"; sandboxd answers 401 without it. The returned
// connection has already had SetReadLimit(MaxMessageBytes) applied — this is one
// of the three sites named in the MaxMessageBytes doc comment, and the caller
// must not have to remember.
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
			return nil, fmt.Errorf("dial %s: %w", endpoint, ErrNotRunning)
		}
		return nil, fmt.Errorf("dial %s: %w", endpoint, err)
	}
	conn.SetReadLimit(MaxMessageBytes)
	return conn, nil
}

// WaitReady polls GET http://endpoint/healthz until it answers 200 or the
// context expires. /healthz is deliberately unauthenticated (Group A task 3) so
// this needs no token.
//
// It exists because ContainerStart returning nil means "the process was
// launched", not "the server is listening". Attaching before that produces a
// connection-refused that looks, in the UI, exactly like a crashed agent.
func WaitReady(ctx context.Context, endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	url := "http://" + endpoint + HealthPath

	var last error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
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
		time.Sleep(readyPollInterval)
	}
	return fmt.Errorf("sandbox %s never became ready: %w (last error: %v)", endpoint, ErrNotRunning, last)
}
```

### 2.4 `internal/backend/dockerbe` — Docker

**Module status, and it decides whether this task can be executed offline.**
`github.com/docker/docker@v28.3.3+incompatible` **is already in this machine's module
cache**, with every transitive dependency the `client` package needs also present as a
zip under `$(go env GOMODCACHE)/cache/download`:

| Module | Cached version |
|---|---|
| `github.com/docker/docker` | v28.3.3+incompatible (also v27.3.1, v28.5.2) |
| `github.com/docker/go-connections` | v0.5.0 (also v0.8.1) |
| `github.com/docker/go-units` | v0.5.0 |
| `github.com/containerd/errdefs` | v1.0.0 |
| `github.com/containerd/errdefs/pkg` | v0.3.0 |
| `github.com/containerd/log` | v0.2.0 |
| `github.com/distribution/reference` | v0.6.0 |
| `github.com/moby/docker-image-spec` | v1.3.1 |
| `github.com/opencontainers/go-digest` | v1.0.0 |
| `github.com/opencontainers/image-spec` | v1.1.1 |
| `github.com/pkg/errors` | v0.9.1 |
| `github.com/felixge/httpsnoop` | v1.0.4 |
| `github.com/go-logr/logr` | v1.4.2 |
| `github.com/go-logr/stdr` | v1.2.2 |
| `go.opentelemetry.io/otel`, `/trace`, `/metric` | v1.28.0 |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | v0.53.0 |

So task 5 is executable with the network down, provided the engineer adds the
`require` lines rather than letting `go get` resolve fresh versions. The task must
say so and must pin exactly these versions. `control-plane` is **not** vendored and
must not become vendored — it is never built inside a container in this phase.

```go
// Package dockerbe runs sandboxes as Docker containers. This is not scaffolding:
// docs/PROGRAM.md ships a "local machine" alongside cloud machines, and this
// package is that feature.
package dockerbe

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// APIClient is the slice of the Docker SDK this package uses. *client.Client
// satisfies it structurally, so production passes the real client and tests pass
// a fake — there is no Docker daemon on a CI agent.
//
// Signatures copied verbatim from
// github.com/docker/docker@v28.3.3+incompatible/client/client_interfaces.go,
// lines 72–103. They must not be "tidied": v28 renamed types.Container to
// container.Summary and types.ContainerJSON to container.InspectResponse, and a
// signature written from memory will not compile.
type APIClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	Ping(ctx context.Context) (types.Ping, error)
	Close() error
}
```

```go
package dockerbe

import (
	"github.com/docker/docker/client"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// Config configures the Docker backend.
type Config struct {
	// Client is the Docker API. Leave nil and New builds one from the
	// environment with API-version negotiation.
	Client APIClient

	// Host overrides DOCKER_HOST. Empty means "use the environment", which on
	// Windows with Docker Desktop is npipe:////./pipe/docker_engine.
	Host string

	// BindIP is the host interface sandbox ports are published to. It defaults
	// to 127.0.0.1 and New rejects anything else unless AllowNonLoopback is
	// set: publishing a sandbox to 0.0.0.0 puts an unauthenticated shell on the
	// LAN, guarded only by a bearer token that is also on that LAN.
	BindIP string

	// AllowNonLoopback exists so the refusal above is a decision someone has to
	// make on purpose. Nothing in Phase 2 sets it.
	AllowNonLoopback bool

	// AMENDED by the reaper-scope fix (2026-09-24). Required: New refuses an
	// invalid one with ErrUnsupported (backend.ValidateInstance) BEFORE it
	// pings the daemon. Create stamps it as LabelInstance; List returns only
	// containers that carry it.
	Instance string
}

// Backend implements backend.Backend over Docker.
type Backend struct {
	api    APIClient
	bindIP string
}

// New returns a Docker backend. When cfg.Client is nil it constructs
// client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()),
// optionally with client.WithHost(cfg.Host), and Pings it: a Docker Desktop that
// is installed but not running must fail here with backend.ErrUnavailable and a
// sentence the user can act on, not later with a nil-pointer dereference.
func New(cfg Config) (*Backend, error)

func (b *Backend) Kind() backend.Kind { return backend.KindDocker }
```

**Create — the exact shapes.** Every field named here was read out of v28.3.3.

```go
// container.Config
&container.Config{
	Image:  spec.Image,
	Labels: labels,                      // map[string]string, see below
	Env:    env,                         // []string of "K=V", PANACEA_APP_* first
	ExposedPorts: nat.PortSet{ attachPort: struct{}{} },
	WorkingDir:   spec.WorkDir,          // "" leaves the image default
	Tty:          false,                 // sandboxd owns the PTY, not Docker
	OpenStdin:    false,
}

// container.HostConfig
&container.HostConfig{
	PortBindings: nat.PortMap{
		attachPort: []nat.PortBinding{{HostIP: b.bindIP, HostPort: ""}},
	},
	RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
	AutoRemove:    false,                // "disconnect does not destroy"; the
	                                     // reaper owns removal, not the daemon
	Binds:         binds,                // from spec.Mounts, "host:container[:ro]"
	Resources: container.Resources{
		NanoCPUs: spec.Resources.NanoCPUs(),
		Memory:   spec.Resources.MemoryBytes(),
	},
}
```

where

```go
var attachPort = nat.Port("8080/tcp") // nat.Port is a string type; this literal is legal
```

- `HostPort: ""` asks the daemon for an ephemeral host port. `AutoRemove: false` is
  load-bearing: `docs/…phase-2…md` "Deliberate refinement 1" says destroy happens on
  explicit kill, idle TTL or reap, never on disconnect, and a self-removing container
  would delete the transcript `claude --resume` reads. AMENDED by the resume fix (2026-09-23): the
  transcript no longer lives in the container — it is in the worktree's conversation
  store on the host (§5.1 step 5), which outlives every sandbox — so this reason is
  gone; the destroy rule it cites still holds on its own.
- `container.Resources` is **embedded** in `HostConfig` (hostconfig.go line 468), so it
  is set as a field named `Resources`.
- Networking: leave `networkingConfig` nil and `NetworkMode` unset — the default bridge.
  Do **not** use host networking; `docs/PROGRAM.md` records that Kata cannot support it
  and the two backends must not diverge in what a sandbox can reach.
- `platform` is nil. The image is linux/amd64 and Docker Desktop picks it.
- Container name: `"panacea-app-" + spec.ID`. A name makes `docker ps` legible during
  an incident, and a name collision is how `ErrAlreadyExists` is detected — the SDK
  returns a 409 whose error `client.IsErrNotFound` is false; match it with
  `errdefs`-free string-free handling by attempting `ContainerInspect` on the name
  first and returning `ErrAlreadyExists` when it succeeds.

**Labels written on every container:**

```go
labels := map[string]string{
	backend.LabelManaged:   "true",
	backend.LabelInstance:  b.instance, // AMENDED by the reaper-scope fix (2026-09-24)
	backend.LabelSandboxID: spec.ID,
	backend.LabelSessionID: spec.SessionID,
	backend.LabelCreatedAt: time.Now().UTC().Format(time.RFC3339),
}
```

The attach token is **not** a label. A label is readable by anything that can talk to
the Docker socket and is echoed into `docker inspect` output that ends up in bug
reports.

**Environment written into every container** — `[]string`, in this order:

```
PANACEA_APP_COMMAND=<spec.Command>
PANACEA_APP_ARGS_JSON=<json.Marshal(spec.Args)>   // "[]" when empty, never ""
PANACEA_APP_ATTACH_TOKEN=<spec.AttachToken>
<each spec.Env entry, sorted by key>
```

`Create` returns `ErrUnsupported` if any `spec.Env` key has the prefix
`PANACEA_APP_`; otherwise a caller could silently replace the attach token.

**`List` — label-based discovery:**

```go
opts := container.ListOptions{
	All:     true, // include exited containers; the reaper needs to see them
	Filters: filters.NewArgs(filters.Arg("label", backend.LabelManaged+"=true")),
}
summaries, err := b.api.ContainerList(ctx, opts)
```

Map each `container.Summary` to a `backend.Sandbox`:

| `Sandbox` field | From |
|---|---|
| `ID` | `s.Labels[backend.LabelSandboxID]` — skip the container entirely if absent |
| `Ref` | `s.ID` |
| `State` | `s.State`: `"running"` → `StateRunning`, `"created"`/`"restarting"` → `StateStarting`, anything else → `StateStopped` |
| `CreatedAt` | `time.Unix(s.Created, 0).UTC()` |
| `Labels` | `s.Labels` |
| `Endpoint` | see below |
| `Instance` | `s.Labels[backend.LabelInstance]` — AMENDED by the reaper-scope fix (2026-09-24), see below |

AMENDED by the reaper-scope fix (2026-09-24): **List returns this instance's containers and no
others.** The daemon filter stays `LabelManaged=true`; the instance is matched in the loop, before
the sandbox-id check, so that a legacy container is seen at all:

- `LabelInstance == b.instance` → mapped as above.
- `LabelInstance` is another id → skipped silently: another control plane owns it, and on a shared
  host that is the normal case.
- `LabelInstance` absent → skipped, and warned about **once per container ID** as a foreign sandbox
  made before per-instance ownership, which nothing will destroy automatically. The warned-once set
  is pruned each call to the containers still on the daemon.

A daemon-side filter on the instance label was rejected: it would make a legacy container
invisible, and nobody would ever be told it is there. `Get` is unchanged except that it reports
`Instance` from the container's labels; `Get`, `Attach` and `Destroy` stay keyed by name, because
their ids come from this control plane's own registry.

**Reading a sandbox's published port — two paths, one rule.**

From `ContainerList`, `s.Ports` is `[]container.Port` with fields
`IP string`, `PrivatePort uint16`, `PublicPort uint16`, `Type string`
(`api/types/container/port.go`). Select the entry where
`PrivatePort == 8080 && Type == "tcp" && IP == b.bindIP`.

From `ContainerInspect`, `resp.NetworkSettings.Ports` is a `nat.PortMap`
(`network_settings.go` line 20). Select `Ports[nat.Port("8080/tcp")]` and, within it,
the binding whose `HostIP == b.bindIP`.

**The rule:** if the only binding found has `HostIP` of `"0.0.0.0"`, `"::"` or `""`,
`Get`/`List` must return `backend.ErrNonLoopbackPublish` rather than hand back an
endpoint. Docker on Windows reports IPv4 and IPv6 bindings separately, so a container
correctly bound to `127.0.0.1` yields exactly one usable entry and possibly an IPv6
one; a container bound to all interfaces yields `0.0.0.0` and is a defect, not a
variation. A running sandbox with no binding yet (the daemon publishes after start)
is `StateStarting` with an empty `Endpoint`, not an error.

`Endpoint` is then `net.JoinHostPort(b.bindIP, strconv.Itoa(int(publicPort)))`.

**`Destroy`** calls `ContainerRemove(ctx, ref, container.RemoveOptions{Force: true, RemoveVolumes: true})`
and treats `client.IsErrNotFound(err)` as success — destroy is idempotent, because the
reaper and an explicit `DELETE` can race.

**`Attach`** is `Get` then `backend.Dial(ctx, sb.Endpoint, token)`; an empty `Endpoint`
is `ErrNotRunning`.

### 2.5 `internal/sandbox` — what the Kata manager must grow

`control-plane/internal/sandbox/manager.go` exists, is unit-tested against
`k8s.io/client-go/kubernetes/fake`, and is dormant. Task 6 **adds to it** and changes
nothing that exists, so `manager_test.go` keeps compiling.

```go
// Spec describes one sandbox. The first five fields are unchanged.
type Spec struct {
	ID      string
	Image   string
	Command string
	CPU     string
	Memory  string

	// Added in Phase 2 so a Kata pod carries the same contract as a Docker
	// container. ArgsJSON is already-encoded JSON, not []string, so the manager
	// never needs encoding/json and the two backends cannot encode differently.
	ArgsJSON    string
	AttachToken string
	Env         map[string]string
	SessionID   string
	Instance    string // AMENDED by the reaper-scope fix (2026-09-24), see below
}

// Get returns the pod for a sandbox id, or ErrNotFound.
func (m *Manager) Get(ctx context.Context, id string) (*corev1.Pod, error)

// List returns every pod this manager created in its namespace — every
// instance's; katabe.List narrows it to its own (§2.6).
func (m *Manager) List(ctx context.Context) ([]corev1.Pod, error)

// ErrNotFound is returned by Get when no pod carries the label.
var ErrNotFound = errors.New("sandbox: no pod for that id")
```

`Get` and `List` both select on
`panacea-app/sandbox-id=<id>` and `panacea-app/component=sandbox` via
`metav1.ListOptions{LabelSelector: …}`. `Get` returns `ErrNotFound` on an empty list
and the first item otherwise.

AMENDED by the reaper-scope fix (2026-09-24): `Spec` gains `Instance string`, stamped as the
`panacea-app/instance` label (`backend.KubeLabelInstance`) and omitted when empty, like
`SessionID`. The selectors are unchanged; `katabe.List` does the instance match (§2.6).

The container's `Env` grows to the three `PANACEA_APP_*` variables plus `spec.Env`,
and the pod gains the `panacea-app/session-id` label. Everything else — runtime class,
tolerations, node selector, the read-only root with `home` and `tmp` emptyDirs, the
matched requests and limits — is untouched.

### 2.6 `internal/backend/katabe` — the adapter

```go
// Package katabe runs sandboxes as Kata Containers pods on AKS. It is DORMANT:
// docs/PROGRAM.md phase 8 is the first real Azure spend, and nothing in Phase 2
// selects this backend by default. It compiles and is unit-tested now so that
// turning it on in Phase 8 is a configuration change and not a rewrite.
package katabe

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
)

// Config configures the Kata backend.
type Config struct {
	Manager *sandbox.Manager
	// ReadyTimeout overrides backend.ReadyTimeout. Kata cold start is seconds
	// (docs/PROGRAM.md open risk 1), which is why Phase 8 needs warm pools.
	ReadyTimeout time.Duration
}

// Backend implements backend.Backend over Kata pods.
type Backend struct{ ... }

func New(cfg Config) (*Backend, error)
func (b *Backend) Kind() backend.Kind { return backend.KindKata }
```

- `Create` rejects a `Spec` with non-empty `Mounts` with `backend.ErrUnsupported`: a
  host path on an AKS node is meaningless, and silently ignoring the field would give
  an agent an empty worktree with no error anywhere.
- AMENDED by the resume fix (2026-09-23): **one more thing Phase 8 must provide.** Every Claude Code
  session now also carries a mount of its worktree's conversation store at
  `/home/dev/.claude/projects` (§5.1 step 5) — the only thing `claude --resume` needs
  from a previous sandbox. Kata must supply it as a volume that outlives the pod,
  scoped **per user and per worktree** (the host layout's `<owner>` segment is where
  the user goes), and holding `projects/` only: `~/.claude` itself is where `/login`
  writes credentials. Until then this refusal fails every Kata create, as the worktree
  mount already did.
- AMENDED by the sandbox git fix (2026-09-23): **and one more.** When `PANACEA_APP_CA_BUNDLE`
  is set (§4), every session also carries a **read-only** mount of that PEM bundle at
  `/etc/panacea-app/ca-bundle.pem` (`sessionCABundlePath`), named by `NODE_EXTRA_CA_CERTS`
  in `Spec.Env` (§5.1 step 5). Kata must supply it as a read-only volume — a ConfigMap or
  Secret projected at the same path is the natural shape — because a host path on an AKS
  node is meaningless here too. `Spec.Env` itself already reaches the pod
  (`sandbox.Spec.Env`), so the git configuration and identity variables need nothing
  from Phase 8. Until then, with the variable set, this refusal fails every Kata create.
- AMENDED by the sandbox git fix, round 1 (2026-09-24): **and a fourth.** A primary worktree's
  session also carries a **read-only** mount of `<worktree>/.git/worktrees` at
  `/home/dev/work/.git/worktrees` (§5.1 step 5). Under `OD-2` (a local clone per worktree) a
  Phase 8 pod's worktree has no linked worktrees to protect, so the need may disappear rather
  than move; until it is decided, Kata must supply it or refuse. The line-ending pairs are
  `Spec.Env` and need nothing from Phase 8, but `Deps.GitLineEndings` asks the host's git
  about a host path, which a pod does not have: Phase 8 answers it from wherever its clone lives.
- AMENDED by the sandbox git fix's round-1 fix round (gitfix-rereview I-A, 2026-09-24): **and a
  fifth.** Every session whose host's git reads a global excludes file or a global attributes file
  also carries a **read-only** mount of each, at `/etc/panacea-app/git/ignore` and
  `/etc/panacea-app/git/attributes` (§5.1 step 5). Kata must supply them as read-only volumes, the
  same shape as the CA bundle, or refuse. `Deps.GitGlobalFiles` asks the host's git about a host
  path too, and Phase 8 answers it for the user whose clone it is. The `core.excludesFile` and
  `core.attributesFile` pairs are `Spec.Env`, and name `/dev/null` when there is no file.
- `Endpoint` is `net.JoinHostPort(pod.Status.PodIP, "8080")`. An empty `PodIP` means
  the pod is not scheduled yet: `StateStarting`, empty `Endpoint`.
- `State` maps `pod.Status.Phase`: `Pending` → `StateStarting`, `Running` →
  `StateRunning` (and `StateStarting` while any container is not `Ready`),
  `Succeeded`/`Failed` → `StateStopped`.
- `Destroy` calls `Manager.Delete` and swallows `k8s.io/apimachinery/pkg/api/errors.IsNotFound`.
- Tests use `k8s.io/client-go/kubernetes/fake.NewSimpleClientset()`, exactly as
  `manager_test.go` already does. No cluster, no network.
- AMENDED by the reaper-scope fix (2026-09-24): **the same ownership seam as dockerbe, built and
  not merely recorded**, because the manager already selects by label. `Config` gains
  `Instance string`, required (`New` refuses an invalid one with `ErrUnsupported`, after the
  nil-`Manager` check). `Create` passes it as `sandbox.Spec.Instance`; `List` returns only pods
  whose `panacea-app/instance` label is this instance's; `sandboxFromPod` sets
  `Sandbox.Instance`. **Recorded, not built:** dockerbe warns once about an unlabelled managed
  container and katabe does not — it has no logger, and no unlabelled Kata pod can exist, because
  Kata has never been deployed. Separately, and older than this fix: Kata pods do not carry
  `backend.LabelManaged`, so §5.3 rule 4 never reaps a Kata orphan. Phase 8 decides whether the
  pod carries `LabelManaged` or rule 4 learns the Kube spelling.
### 2.7 `internal/statepath` — one answer to "where does state live"

```go
// Package statepath answers one question: where on this machine does the control
// plane keep state. It exists because two packages need the same answer —
// registry (Phase 2) and discoverstore (Phase 4) — and two copies of an OS
// ladder drift the moment one of them gains a case.
package statepath

import (
	"os"
	"path/filepath"
)

// EnvStateDir overrides every other rule.
const EnvStateDir = "PANACEA_APP_STATE_DIR"

// Dir returns the directory for a named kind of state, e.g. Dir("sessions") or
// Dir("discover"). The ladder, in order:
//
//	$PANACEA_APP_STATE_DIR/<sub>
//	%LOCALAPPDATA%\PanaceaApp\state\<sub>        (Windows)
//	$XDG_STATE_HOME/panacea-app/<sub>
//	~/.local/state/panacea-app/<sub>
//	<os.TempDir()>/panacea-app/<sub>             (only if the home directory is unknowable)
//
// Dir does not create anything. The caller does, because the caller knows
// whether a missing directory is an error.
func Dir(sub string) string {
	if dir := os.Getenv(EnvStateDir); dir != "" {
		return filepath.Join(dir, sub)
	}
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return filepath.Join(dir, "PanaceaApp", "state", sub)
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "panacea-app", sub)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "panacea-app", sub)
	}
	return filepath.Join(home, ".local", "state", "panacea-app", sub)
}
```

This ladder is byte-for-byte the one Phase 4's `discoverstore.DefaultDir()` already
contains (phase-4 plan lines 3200–3215). See §8.1.

AMENDED by the resume fix (2026-09-23): a third kind, `conversations`, sits beside `sessions`.
`cmd/api` derives it the way it derives the registry path —
`$PANACEA_APP_STATE_DIR/conversations`, else `statepath.Dir("conversations")` — and
makes it absolute, because it becomes a bind mount's host path.

AMENDED by the reaper-scope fix (2026-09-24): a fourth kind, `instance`, holds one file,
`instance-id` — this state directory's instance id (§2.2 amendment). `cmd/api` derives
`Config.InstancePath` the same way: `$PANACEA_APP_STATE_DIR/instance/instance-id`, else
`statepath.Dir("instance")/instance-id`. A new package owns it:

```go
// package instance
const FileName = "instance-id"

// Load returns this state directory's instance id, minting and persisting a
// random lowercase UUID the first time. minted reports which happened.
func Load(path string) (id string, minted bool, err error)
```

- **Random, not a hash of the path.** A path hash collides when two machines use the same default
  path against one remote daemon, and changes when one directory is reached by another spelling
  (an 8.3 short name, a symlink), which would disown a running fleet.
- **Crash-safe and race-safe.** The id is written to a temporary file in the same directory,
  synced, then hard-linked to `instance-id`, which fails rather than replaces: a crash leaves no
  empty id file, and the loser of a race between two starts on one directory adopts the winner's id.
- **A file that does not hold a valid id is an error, never replaced.** A new id would make every
  sandbox the old one made foreign — never collected — and its sessions would read as gone while
  their containers ran. An operator who means that deletes the file.
- `serve` calls `Load` **first**, before `newBackend`, because both the backend and the reaper
  need the id. It logs the one start that mints one, and the id on the "listening" line.

### 2.8 `internal/registry` — the session record and its store

```go
package registry

import "time"

// Status is the session status the user sees.
//
// PHASE 2 SHIPS FOUR, AND ONLY FOUR. docs/AGENTIC-UX.md §4 rule 8 specifies a
// closed set of seven (PLANNING, EXECUTING, BLOCKED-ON-YOU, BLOCKED-ON-MACHINE,
// PAUSED, DRIFTING, DONE). Five of those cannot be derived without reading agent
// output, which docs/PROGRAM.md forbids absolutely. The four below are the facts
// the control plane can actually observe. BLOCKED-ON-YOU needs the agent-runtime
// signal docs/AGENTIC-UX.md §3 calls "a hard runtime dependency, not a UI task".
// It is not invented here and must not be guessed from PTY bytes.
type Status string

const (
	// StatusStarting: the sandbox has been created and has not yet answered
	// /healthz.
	StatusStarting Status = "starting"
	// StatusRunning: the sandbox answers /healthz and its process is alive.
	StatusRunning Status = "running"
	// StatusIdle: running, but no output has crossed the relay for
	// Config.IdleAfter. Derived from a byte COUNT, never from byte content.
	StatusIdle Status = "idle"
	// StatusExited: the process ended, the sandbox was killed, or the sandbox
	// disappeared. Terminal. The record is kept — docs/FUNCTIONAL-SPEC.md:
	// "Finished sessions stay listed and readable — the transcript is the
	// artifact."
	StatusExited Status = "exited"
)

// Session is one row of the registry: everything the control plane must survive
// its own restart with.
//
// AttachToken and Endpoint are persisted and are NEVER serialised onto the REST
// surface. api.sessionJSON exists to make that a compile-time fact rather than a
// review question.
type Session struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	MachineID  string `json:"machineId"`
	ProjectID  string `json:"projectId"`
	BranchID   string `json:"branchId"`
	WorktreeID string `json:"worktreeId"`

	Harness        string `json:"harness"`        // harness.ID
	Model          string `json:"model"`
	Effort         string `json:"effort"`
	PermissionMode string `json:"permissionMode"`
	OpeningPrompt  string `json:"openingPrompt"`
	Command        string `json:"command"` // resolved argv[0], for restart
	Args           []string `json:"args"`  // resolved argv[1:], for restart

	// ConversationID — AMENDED by the resume fix (2026-09-23). The HARNESS's id for this
	// session's conversation: the UUID the control plane minted and passed as
	// `claude --session-id`, or "" for a harness with none (a shell). A later
	// resume passes it to `claude --resume`; ID above means nothing to the CLI.
	// Persisted, never on the REST surface (§3.10), and additive inside
	// schemaVersion 1.
	ConversationID string `json:"conversationId"`

	// EarlierConversationIDs — AMENDED by the resume fix's round-1 fix round
	// (resume-fix-review I2, 2026-09-24). This session's OWN conversations from
	// before its latest restart, oldest first, each one the CLI wrote a
	// transcript for (§3.14). A resume falls back to the newest of them when
	// ConversationID has no transcript yet (§3.9), which is the state a restart
	// leaves until someone speaks. Deep-copied by Clone like Args. Same rules as
	// ConversationID: persisted, never on the REST surface, additive inside
	// schemaVersion 1; an older record decodes it as nil.
	EarlierConversationIDs []string `json:"earlierConversationIds"`

	Status     Status `json:"status"`
	ExitReason string `json:"exitReason"`

	Backend     string `json:"backend"`     // backend.Kind
	SandboxID   string `json:"sandboxId"`   // backend.Spec.ID
	SandboxRef  string `json:"sandboxRef"`  // container id / pod name
	Endpoint    string `json:"endpoint"`    // host:port, revalidated on every boot
	AttachToken string `json:"attachToken"` // secret; see the type comment

	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LastOutputAt time.Time `json:"lastOutputAt"`
	AttachCount  int       `json:"attachCount"` // currently-connected clients

	// ContextRef is the Phase 4 brief this session inherited, or "". Phase 2
	// stores and returns it and never interprets it. See §8.3.
	ContextRef string `json:"contextRef"`
}
```

```go
package registry

import "errors"

// ErrNotFound is returned for an unknown session id.
var ErrNotFound = errors.New("registry: no such session")

// ErrExists is returned when Create is given an id that is already taken.
var ErrExists = errors.New("registry: session already exists")

// Store holds every session this control plane knows about.
//
// CONCURRENCY DISCIPLINE, AND IT IS NOT OPTIONAL: no caller ever performs a
// Get-then-Put. Three goroutines mutate a session concurrently — the HTTP
// handler, the attach relay writing LastOutputAt, and the reaper — and
// Get-then-Put loses whichever write finishes first. Update is the only mutation
// primitive; the mutate function runs while the store holds its write lock, so
// read-modify-write is atomic by construction. Put exists only for Create.
type Store interface {
	Create(s Session) error
	Get(id string) (Session, error)
	List() ([]Session, error)
	Update(id string, mutate func(*Session) error) (Session, error)
	Delete(id string) error
}

// FileStore is the Phase 2 implementation: an in-memory index that is written
// through to one JSON document on every mutation.
type FileStore struct{ ... }

// NewFileStore opens (and creates if absent) the store at path. A file that
// exists but cannot be parsed is renamed to <path>.corrupt-<RFC3339> and an
// empty store is returned, because refusing to start leaves the user with no
// cockpit and no way to fix it; the rename means nothing is lost.
func NewFileStore(path string) (*FileStore, error)

// DefaultPath is statepath.Dir("sessions") + "/sessions.json".
func DefaultPath() string
```

**Persistence format.** One document, not one file per session, because sessions are
tens rather than thousands, ordering matters to the rail, and a single
`os.Rename` is the only atomic primitive available on both Windows and POSIX.

```json
{
  "schemaVersion": 1,
  "savedAt": "2026-09-18T09:14:07.412Z",
  "sessions": [
    {
      "id": "s-3f2a91c0",
      "title": "evidence upload",
      "machineId": "local",
      "projectId": "p-8c1d0e42",
      "branchId": "b-8c1d0e42-evidence-upload",
      "worktreeId": "w-4a77b1c9",
      "harness": "claude-code",
      "model": "opus",
      "effort": "high",
      "permissionMode": "ask",
      "openingPrompt": "add tests, then refactor",
      "command": "claude",
      "args": ["--model", "opus", "--effort", "high", "add tests, then refactor"],
      "status": "running",
      "exitReason": "",
      "backend": "docker",
      "sandboxId": "s-3f2a91c0",
      "sandboxRef": "9c1f0b7ad3e2",
      "endpoint": "127.0.0.1:52431",
      "attachToken": "Qk9fV0lMTF9CRV9SRVBMQUNFRF9CWV8zMl9SQU5ET01f",
      "createdAt": "2026-09-18T09:14:03Z",
      "updatedAt": "2026-09-18T09:14:07Z",
      "lastOutputAt": "2026-09-18T09:14:06Z",
      "attachCount": 1,
      "contextRef": ""
    }
  ]
}
```

- `schemaVersion` is `1`. A document whose `schemaVersion` is greater than the binary
  understands is **not** silently migrated: `NewFileStore` returns an error naming both
  versions, because a newer app having already written the file means downgrading would
  drop fields.
- `savedAt` is diagnostic only. Nothing reads it.
- The file is written `0o600`, not `0o644`: it contains attach tokens. On Windows the
  mode is advisory, and that is worth one sentence in the package comment rather than a
  pretence of protection — the real boundary is that the file lives under the user's
  own `%LOCALAPPDATA%`.

**Atomic write, and the Windows part that people get wrong.**

```go
// writeAtomic replaces path with data.
//
// os.Rename on Windows is MoveFileEx with MOVEFILE_REPLACE_EXISTING, so it does
// replace an existing file atomically. What it does NOT do is succeed while
// another process has the target open — Windows Defender and search indexers
// open files briefly and opportunistically, and the failure surfaces as
// ERROR_ACCESS_DENIED on a rename that worked a hundred times before. A bounded
// retry is the whole fix. Writing in place instead would risk a truncated
// registry, which is the one failure that loses running sessions.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sessions-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}

	var last error
	for i := 0; i < 5; i++ {
		if last = os.Rename(tmpName, path); last == nil {
			return nil
		}
		time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
	}
	return fmt.Errorf("registry: replace %s: %w", path, last)
}
```

Write amplification is bounded by the throttle in §6.4: `LastOutputAt` is flushed at
most once every two seconds per session, not per PTY frame.

### 2.9 `internal/harness` — the catalogue and argv construction

```go
// Package harness decides which binary a session runs and with which arguments.
// It never wraps, parses or reimplements any CLI; it constructs argv and stops.
package harness

// ID names a harness. These strings are the wire values in
// GET /v1/harnesses and in registry.Session.Harness, so they are stable.
type ID string

const (
	ClaudeCode ID = "claude-code"
	Codex      ID = "codex"
	OpenCode   ID = "opencode"
	GeminiCLI  ID = "gemini-cli"
	Shell      ID = "shell"
)

// PermissionMode is the product's vocabulary, from docs/FUNCTIONAL-SPEC.md
// ("bypass / ask / plan"). Each harness maps it to its own flag, or to nothing.
type PermissionMode string

const (
	PermissionAsk    PermissionMode = "ask"
	PermissionBypass PermissionMode = "bypass"
	PermissionPlan   PermissionMode = "plan"
)

// Effort is Claude Code's effort level. docs/FUNCTIONAL-SPEC.md exposes four.
// The CLI also accepts "max"; the product does not offer it, so Build rejects it.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortXHigh  Effort = "xhigh"
)

// Model is one selectable model for a harness.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Descriptor is everything the cockpit needs to render the picker and everything
// Build needs to construct argv.
type Descriptor struct {
	ID      ID     `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`

	// Installed is whether images/sandbox/Dockerfile actually puts this binary
	// in the sandbox image. Only Claude Code and the shell are, today.
	Installed bool `json:"installed"`

	// FlagsVerified is whether this package's argv was checked against the real
	// CLI's --help. An unverified harness is offered by the catalogue as
	// unavailable rather than started with guessed flags, because a wrong flag
	// produces a usage error inside a terminal the user then has to read.
	FlagsVerified bool `json:"flagsVerified"`

	Models          []Model          `json:"models"`
	DefaultModel    string           `json:"defaultModel"`
	Efforts         []Effort         `json:"efforts"`
	PermissionModes []PermissionMode `json:"permissionModes"`

	SupportsOpeningPrompt bool `json:"supportsOpeningPrompt"`
	// AMENDED by the resume fix (2026-09-23): also whether the harness has a
	// conversation for the control plane to name. Resuming is the only thing
	// Request.Conversation is for, so a harness that cannot resume is given
	// none, no conversation store is mounted for it, and Build refuses one.
	SupportsResume        bool `json:"supportsResume"`

	// Note is shown in the picker when the harness is unavailable. It says why.
	Note string `json:"note"`
}

// Available reports whether a session may actually be started on this harness.
func (d Descriptor) Available() bool { return d.Installed && d.FlagsVerified }

// Request is one session's configuration.
type Request struct {
	Harness       ID
	Model         string
	Effort        Effort
	Permission    PermissionMode
	OpeningPrompt string
	Conversation  string // the id this session's conversation will carry; "" names none
	Resume        string // the conversation to continue from; "" starts fresh
}

// AMENDED by the resume fix (2026-09-23): Conversation and Resume are both the HARNESS's
// conversation ids (UUIDs), never a cockpit session id. As first written, Resume
// was documented as "a prior agent session id" and the API filled it with the
// cockpit's s-… id, which `claude --resume` has never heard of: Resume resumed
// nothing. The API now resolves it from the source session's record (§3.9).

// Catalogue returns every harness, in display order.
func Catalogue() []Descriptor

// Lookup returns one descriptor.
func Lookup(id ID) (Descriptor, bool)

// Build turns a Request into the command and argv a sandbox runs.
func Build(r Request) (command string, argv []string, err error)

var (
	ErrUnknownHarness   = errors.New("harness: unknown harness")
	ErrUnavailable      = errors.New("harness: harness is not available in the sandbox image")
	ErrUnsupportedOption = errors.New("harness: this harness does not accept that option")
	ErrUnknownModel     = errors.New("harness: model is not offered by this harness")
	// AMENDED by the resume fix (2026-09-23). A control-plane defect, never a user's: both
	// conversation ids come from the record, so the API answers it 500.
	ErrConversationID   = errors.New("harness: conversation ids are not usable")
)
```

**Claude Code — verified.** Checked against `claude --help` on this machine, version
**2.1.267**; `images/sandbox/Dockerfile` pins **2.1.274**. `Installed: true`,
`FlagsVerified: true`, `Command: "claude"`.

| Request field | argv contribution | Evidence |
|---|---|---|
| `Model` (non-empty) | `--model <value>` | `--model <model>` — "Provide an alias for the latest model (e.g. 'fable', 'opus', or 'sonnet') or a model's full name (e.g. 'claude-fable-5')" |
| `Effort` | `--effort <value>` | `--effort <level>` — "(low, medium, high, xhigh, max)" |
| `Permission == "bypass"` | `--permission-mode bypassPermissions` | `--permission-mode <mode>` choices: `acceptEdits`, `auto`, `bypassPermissions`, `manual`, `dontAsk`, `plan` |
| `Permission == "plan"` | `--permission-mode plan` | same |
| `Permission == "ask"` | **nothing** | deliberate; see below |
| `Resume` (non-empty) | `--resume <value> --fork-session` — AMENDED by the resume fix (2026-09-23); was `--resume <value>` | `-r, --resume [value]`; `--fork-session` "When resuming, create a new session ID instead of reusing the original" |
| `Conversation` (non-empty) — AMENDED by the resume fix (2026-09-23) | `--session-id <value>`, after any `--resume` and before the prompt | `--session-id <uuid>` "(must be a valid UUID)" |

AMENDED by the resume fix (2026-09-23). **A resume forks, and the fork is named.** Reusing the
original id is wrong here: the cockpit offers Resume on a running session, so reuse
puts two processes in two sandboxes appending to one transcript, and the CLI's own
running-session guard lives in `~/.claude/sessions`, which no sandbox shares. A bare
`--fork-session` gets an id only the CLI's output would reveal, so `Conversation`
names it. The CLI accepts that pairing and refuses `--session-id` with `--resume`
alone. `Build` refuses, with `ErrConversationID`: a non-UUID in either field, a
`Resume` with no `Conversation`, and the two being equal (`Session ID … is already in
use`). A `Conversation` on a harness with `SupportsResume: false` is
`ErrUnsupportedOption`. The validation slot is the old `resume` slot: model, effort,
permission, **resume and conversation**, prompt.
| `OpeningPrompt` (non-empty) | appended last, as one positional argument | `Usage: claude [options] [command] [prompt]` |

**Why `ask` emits no flag.** The CLI's choice list has no `default`; 2.1.x offers
`manual`, `auto` and `dontAsk`, and which of those is "prompt me for everything" was
not verifiable from `--help` text alone. Omitting the flag gets the CLI's own default,
which *is* the ask behaviour, and is provably correct without guessing a mapping. This
is recorded in §7 as the thing to confirm before Phase 8 adds an admin control over
permitted modes.

Models offered for Claude Code (aliases, from the `--model` help text):
`opus` (default), `sonnet`, `fable`. A free-text model string is accepted and passed
through unchanged — the CLI documents full model names, and pinning the list here
would make the app stale the week a new model ships.

Example: `Request{Harness: ClaudeCode, Model: "opus", Effort: EffortHigh, Permission: PermissionAsk, OpeningPrompt: "add tests, then refactor"}`

```
command = "claude"
argv    = ["--model", "opus", "--effort", "high", "add tests, then refactor"]
```

The comma inside the prompt is exactly the case `PANACEA_APP_ARGS_JSON` exists for
(Group A task 3, defect 2). The legacy comma-separated `-args` form would split it into
two arguments.

**Shell — verified, trivially.** `Command: "/bin/bash"`, `argv: ["-l"]`,
`Installed: true`, `FlagsVerified: true`, no models, no efforts, no permission modes.
`Build` returns `ErrUnsupportedOption` when a `Model`, `Effort`, `Permission` other
than `ask`, `Resume`, `Conversation` (AMENDED by the resume fix (2026-09-23)) or `OpeningPrompt` is set — a shell has no way to receive an
opening prompt that is not "execute this as a command", and silently executing a
sentence the user typed into a prompt box is a footgun.

**Codex, OpenCode, Gemini CLI — UNVERIFIED and not installed.** None of these binaries
exists on this machine or in `images/sandbox/Dockerfile`, which installs only
`@anthropic-ai/claude-code@2.1.274`. They ship in the catalogue as
`Installed: false`, `FlagsVerified: false`, with `Command` set to the conventional
binary name (`codex`, `opencode`, `gemini`) and a `Note`:

> "Not installed in the sandbox image. Add it to images/sandbox/Dockerfile and verify
> its flags against `--help` before enabling."

`Build` returns `ErrUnavailable` for all three. `docs/FUNCTIONAL-SPEC.md` lists them in
the picker; the cockpit therefore renders them disabled with the note, which is honest
about what exists rather than offering a button that produces a usage error inside a
terminal. Enabling one is a two-line change here plus a Dockerfile line plus a flag
check — that is Phase 2's exit note, not a guess made now.

### 2.10 `internal/worktree` — the Phase 3 seam

```go
// Package worktree resolves a worktree id to a directory on disk.
//
// PHASE 3 SEAM — this contract is restated in the Phase 3 plan and, in identical
// words, in Phase 4's contextwrite package:
//  1. Path returns an ABSOLUTE path to an EXISTING directory that is the root of
//     a checked-out worktree. It never creates, moves or removes anything.
//  2. Phase 3 owns creating, listing and removing worktrees. Phase 2 owns only
//     resolution and read-only enumeration.
//  3. Anything that must be on disk before an agent starts — Phase 4 writes
//     CLAUDE.md here — is written BEFORE the sandbox is created, never after.
//     Claude Code reads CLAUDE.md at session start; writing it afterwards means
//     the session inherits nothing, silently, with no error anywhere.
package worktree

import (
	"context"
	"errors"
)

// ErrUnknownID is returned for an id the provider does not know.
var ErrUnknownID = errors.New("worktree: unknown worktree id")

// ErrNotADir is returned when the resolved path is not an existing directory.
var ErrNotADir = errors.New("worktree: path is not an existing directory")

// Provider resolves worktree ids. The method set is deliberately one method:
// Phase 4's contextwrite.WorktreeResolver declares exactly this signature, so
// one type satisfies both with no adapter.
type Provider interface {
	Path(ctx context.Context, worktreeID string) (string, error)
}

// Static is the implementation Phase 2 ships when no projects directory is
// configured, and the one every test uses.
type Static map[string]string

// Path implements Provider. It verifies the directory exists, because a
// Provider that returns a plausible path for a directory that is not there
// pushes the failure into Docker's bind-mount error, which is unreadable.
func (s Static) Path(ctx context.Context, worktreeID string) (string, error)
```

### 2.11 `internal/project` — read-only enumeration

```go
// Package project enumerates the projects, branches and worktrees the cockpit's
// rail shows. It is READ-ONLY by design: docs/PROGRAM.md phase 3 owns worktree
// lifecycle, and a Phase 2 that could create or delete one would be building
// Phase 3 badly.
package project

type Project struct {
	ID   string `json:"id"`   // "p-" + first 8 hex of sha256(absolute path)
	Name string `json:"name"` // the directory's base name
	Path string `json:"path"` // absolute
}

type Branch struct {
	ID        string `json:"id"`   // "b-" + project hash + "-" + slug(name)
	ProjectID string `json:"projectId"`
	Name      string `json:"name"` // e.g. "evidence-upload"
	IsHead    bool   `json:"isHead"`
}

type Worktree struct {
	ID        string `json:"id"` // "w-" + first 8 hex of sha256(absolute path)
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
	Name      string `json:"name"` // base name of the worktree directory
	Path      string `json:"path"` // absolute
	IsPrimary bool   `json:"isPrimary"`
}

// Runner runs git. It is an interface so tests need neither git nor a repo.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Scanner enumerates one root directory of repositories.
type Scanner struct{ ... }

// NewScanner returns a scanner over root. root may be "", in which case every
// method returns an empty slice and no error — an unconfigured projects
// directory is a normal state, not a failure.
func NewScanner(root string, git Runner) *Scanner

func (s *Scanner) Projects(ctx context.Context) ([]Project, error)
func (s *Scanner) Branches(ctx context.Context, projectID string) ([]Branch, error)
func (s *Scanner) Worktrees(ctx context.Context, projectID string) ([]Worktree, error)

// Path implements worktree.Provider.
func (s *Scanner) Path(ctx context.Context, worktreeID string) (string, error)

// ProjectName and BranchName implement Phase 4's discoverapi.Naming. Per that
// contract they never return an error and fall back to the id itself, because a
// missing display name must not fail a brief.
func (s *Scanner) ProjectName(projectID string) string
func (s *Scanner) BranchName(branchID string) string
```

**Ids are derived, not stored.** `p-`/`w-` ids are the first 8 hex characters of the
SHA-256 of the absolute, `filepath.Clean`ed, lowercased path; `b-` ids are
`"b-" + <project hash> + "-" + slug(branch name)` where `slug` lowercases and replaces
every run of characters outside `[a-z0-9]` with `-`. Deriving means a restart produces
the same ids with no storage and no migration, and it is why `registry.Session` can
hold a `worktreeId` across a reboot.

**Git commands used, all read-only:**

| Purpose | Command |
|---|---|
| is this directory a repo | presence of a `.git` file or directory (no subprocess) |
| branches | `git for-each-ref --format=%(refname:short) refs/heads` |
| current branch | `git rev-parse --abbrev-ref HEAD` |
| worktrees | `git worktree list --porcelain` |

`Runner`'s production implementation runs `exec.CommandContext` with a 10-second
timeout and returns the combined output on failure so the error names the git message.
Scanning is depth-1 under `root`: subdirectories of `root` that contain `.git`. Deeper
recursion over a developer's whole disk is how a rail takes 40 seconds to paint.

### 2.12 `internal/httpx` — the error body and the origin guard

```go
package httpx

// Problem is the error body of every non-2xx JSON response on this API. One
// shape everywhere means the desktop's client has one error branch.
type Problem struct {
	// Error is a stable machine-readable code. The desktop switches on it.
	Error string `json:"error"`
	// Message is one human sentence, safe to render. It never contains a token,
	// a file path outside the user's own projects, or a Go error chain.
	Message string `json:"message"`
	// Details carries structured context, e.g. {"id": "s-3f2a91c0"}.
	Details map[string]string `json:"details,omitempty"`
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any)

// WriteProblem writes a Problem.
func WriteProblem(w http.ResponseWriter, status int, code, msg string, details map[string]string)

// DecodeJSON reads a JSON body into dst. It rejects unknown fields and bodies
// over 1 MiB, writes a 400 invalid_json Problem itself on failure, and reports
// whether the handler should continue.
func DecodeJSON[T any](w http.ResponseWriter, r *http.Request, dst *T) bool

// MaxBodyBytes bounds a request body. The REST surface carries an opening
// prompt, not a file.
const MaxBodyBytes = 1 << 20
```

```go
package httpx

// OriginPolicy decides whether a browser context may talk to this control plane.
//
// WHY NOT coder/websocket's built-in check: AcceptOptions.OriginPatterns matches
// on the Origin's HOST ONLY, ignoring the scheme (accept.go:217-229), and
// returns nil for a MISSING Origin (accept.go:208). Both are reasonable defaults
// for a web server and both are wrong here: a loopback control plane's whole
// threat model is a web page the user happens to have open. This policy compares
// the full origin string and treats an unknown origin as a hard 403 with a log
// line naming what was rejected and what is allowed — a silent 403 on a laptop
// is a bug report that costs an afternoon.
type OriginPolicy struct {
	allowed map[string]struct{}
	log     *slog.Logger
}

// NewOriginPolicy returns a policy allowing exactly these origins. An empty
// slice means DefaultOrigins.
func NewOriginPolicy(allowed []string, log *slog.Logger) *OriginPolicy

// DefaultOrigins is what a Tauri webview actually sends.
//
// HOW THIS WAS DETERMINED, because it is the field most likely to be wrong:
// tauri v2 computes the webview's origin in crates/tauri/src/manager/webview.rs.
// For a custom-scheme URL on Windows and Android it produces
// "{http|https}://{scheme}.localhost"; on macOS and Linux it leaves the custom
// scheme intact, giving "tauri://localhost". Which of http and https is chosen
// is WindowConfig.use_https_scheme, documented in crates/tauri-utils/src/config.rs
// as "Sets whether the custom protocols should use https://<scheme>.localhost
// instead of the default http://<scheme>.localhost on Windows and Android.
// Defaults to false." desktop/src-tauri/tauri.conf.json does not set it, so the
// Windows build sends http://tauri.localhost. In development the window loads
// build.devUrl directly, so the origin is http://localhost:1420.
//
// All five are allowed because the same binary serves a Windows dev run, a
// Windows packaged run and a macOS or Linux developer's machine, and an
// allowlist that is right on one of them and silently wrong on the others is
// worse than one that is explicit.
func DefaultOrigins() []string {
	return []string{
		"http://tauri.localhost",  // Tauri 2 packaged, Windows (this repo's default)
		"https://tauri.localhost", // Tauri 2 packaged, Windows with useHttpsScheme
		"tauri://localhost",       // Tauri 2 packaged, macOS and Linux
		"http://localhost:1420",   // vite dev server, tauri.conf.json build.devUrl
		"http://127.0.0.1:1420",   // the same dev server reached by address
	}
}

// Allow reports whether an Origin header value is acceptable. An EMPTY origin is
// allowed: curl, the Go test client and every non-browser tool send none, and
// the attack this defends against is a browser, which always sends one.
func (p *OriginPolicy) Allow(origin string) bool

// Middleware rejects a request whose Origin is present and not allowed with 403
// forbidden_origin, and whose Host is not loopback with 403 forbidden_host.
//
// The Host check is the DNS-rebinding defence and it is the one that actually
// matters: a page at http://evil.example can point a DNS name at 127.0.0.1 and
// its requests then carry Host: evil.example. Requiring Host to be 127.0.0.1,
// [::1] or localhost, with or without the port, closes it.
func (p *OriginPolicy) Middleware(next http.Handler) http.Handler
```

The attach handler calls `p.Allow(r.Header.Get("Origin"))` itself before
`websocket.Accept`, then passes `&websocket.AcceptOptions{InsecureSkipVerify: true}`
with a comment saying the check has already run. Doing it before the upgrade is what
makes the rejection a readable **403 with a JSON body** instead of a WebSocket close
frame the browser reports as "connection failed".

---
## 3. The REST surface

Base URL `http://127.0.0.1:8730`. Canonical prefix `/v1`. The mux additionally serves
`/api/v1/…` as an alias of `/v1/…` — one line,
`mux.Handle("/api/v1/", http.StripPrefix("/api", mux))` — because Phase 4's plan
already writes every Discover route and every Discover test against `/api/v1/discover`
(phase-4 plan lines 3610, 3718 onwards). See §8.2.

All bodies are JSON. All timestamps are RFC 3339 with a `Z` offset. All field names are
`camelCase`. This section is the source the desktop's TypeScript client is written
from; nothing below needs guessing.

### 3.1 The table

| Method | Path | Success | Notes |
|---|---|---|---|
| `GET` | `/v1/healthz` | 200 | Unauthenticated, no origin check, for a supervisor |
| `GET` | `/v1/machines` | 200 | |
| `GET` | `/v1/projects` | 200 | |
| `GET` | `/v1/projects/{projectId}/branches` | 200 | |
| `GET` | `/v1/projects/{projectId}/worktrees` | 200 | Optional `?branchId=` filter |
| `GET` | `/v1/harnesses` | 200 | The picker's data |
| `POST` | `/v1/sessions` | 201 | `Location: /v1/sessions/{id}` |
| `GET` | `/v1/sessions` | 200 | Optional `?worktreeId=`, `?status=` filters |
| `GET` | `/v1/sessions/{id}` | 200 | |
| `PATCH` | `/v1/sessions/{id}` | 200 | Rename only |
| `POST` | `/v1/sessions/{id}/restart` | 200 | Same config, clean context, same id |
| `DELETE` | `/v1/sessions/{id}` | 200 | Kill: destroys the sandbox, keeps the record |
| `DELETE` | `/v1/sessions/{id}?purge=true` | 204 | Kill and forget the record |
| `GET` | `/v1/sessions/{id}/attach` | 101 | WebSocket upgrade |

Route patterns use Go 1.22 `ServeMux` syntax (`"GET /v1/sessions/{id}"`), which the
module's `go 1.23` directive supports. `r.PathValue("id")` reads the segment.

### 3.2 The error body, used everywhere

```json
{
  "error": "session_not_found",
  "message": "No session with id s-3f2a91c0.",
  "details": { "id": "s-3f2a91c0" }
}
```

`details` is omitted when empty. The complete set of `error` codes:

| Code | Status | Raised when |
|---|---|---|
| `invalid_json` | 400 | Body is not JSON, has an unknown field, or exceeds 1 MiB |
| `bad_request` | 400 | A field is missing or has an unusable value |
| `unknown_harness` | 400 | `harness` is not in the catalogue |
| `unsupported_option` | 400 | The harness does not accept that option (e.g. an opening prompt on a shell) |
| `worktree_not_found` | 404 | `worktreeId` resolves to nothing |
| `project_not_found` | 404 | `projectId` resolves to nothing |
| `session_not_found` | 404 | No session with that id |
| `forbidden_origin` | 403 | `Origin` present and not allowlisted |
| `forbidden_host` | 403 | `Host` is not loopback |
| `conflict` | 409 | The session is in the wrong state for the verb |
| `sandbox_not_running` | 409 | Attach or restart on a session whose sandbox is gone |
| `harness_unavailable` | 409 | The harness exists in the catalogue but is not in the sandbox image |
| `upgrade_required` | 426 | `/attach` reached without WebSocket headers |
| `backend_unavailable` | 503 | No Docker daemon, no kubeconfig |
| `internal` | 500 | Anything else. `message` is generic; the detail goes to the log |

### 3.3 `GET /v1/healthz`

200, `{"status":"ok","version":"0.2.0","backend":"docker","backendReady":true}`.
`backendReady` is a cached result of the backend's `Ping`, refreshed by the reaper, so
a health check never blocks on a hung Docker socket. When `backendReady` is `false` the
status code is still 200 — the control plane is up; the backend is not. A supervisor
restarting the control plane would not fix Docker Desktop being closed.

### 3.4 `GET /v1/machines`

```json
{
  "machines": [
    {
      "id": "local",
      "name": "This PC",
      "kind": "local",
      "state": "running",
      "backend": "docker",
      "isolation": "container",
      "default": true,
      "limitations": [
        "Sessions run in Docker containers on this machine, not in a microVM.",
        "A container shares this machine's kernel; isolation is weaker than a cloud sandbox."
      ]
    }
  ]
}
```

- `kind`: `"local"` | `"cloud"`. Phase 2 returns exactly one machine, the local one.
  `docs/FUNCTIONAL-SPEC.md` says a cloud machine is claimed from a warm pool at first
  sign-in; warm pools are Phase 8.
- `state`: `provisioning` | `running` | `idle` | `sleeping` | `stopped` | `error`, the
  set from `docs/FUNCTIONAL-SPEC.md` "Machines". Phase 2 emits `running` or `error`
  only, and `error` carries a `message` field naming the cause (Docker not running).
- `isolation`: `"microvm"` | `"container"` | `"none"`. `docs/FUNCTIONAL-SPEC.md` badges
  the local machine `unsandboxed`; `"container"` is the honest value for what Phase 2
  actually builds, and `limitations` says what it means in a sentence.

### 3.5 `GET /v1/projects`

```json
{
  "projects": [
    { "id": "p-8c1d0e42", "name": "engagement-portal", "path": "C:\\work\\engagement-portal" }
  ]
}
```

An unconfigured `PANACEA_APP_PROJECTS_DIR` returns `{"projects":[]}` and 200. Empty is
a state the cockpit renders; it is not an error.

### 3.6 `GET /v1/projects/{projectId}/branches`

```json
{
  "branches": [
    { "id": "b-8c1d0e42-main", "projectId": "p-8c1d0e42", "name": "main", "isHead": false },
    { "id": "b-8c1d0e42-evidence-upload", "projectId": "p-8c1d0e42", "name": "evidence-upload", "isHead": true }
  ]
}
```

404 `project_not_found` for an unknown `projectId`.

### 3.7 `GET /v1/projects/{projectId}/worktrees[?branchId=]`

```json
{
  "worktrees": [
    {
      "id": "w-4a77b1c9",
      "projectId": "p-8c1d0e42",
      "branchId": "b-8c1d0e42-evidence-upload",
      "name": "evidence-upload",
      "path": "C:\\work\\engagement-portal-evidence-upload",
      "isPrimary": false,
      "sessionCount": 2
    }
  ]
}
```

`sessionCount` counts registry sessions on that worktree whose status is not `exited`.
It is a count of live things the user owns, not a progress metric —
`docs/AGENTIC-UX.md` deletes counts that stand in for completion, and this is not one.

### 3.8 `GET /v1/harnesses`

```json
{
  "harnesses": [
    {
      "id": "claude-code",
      "name": "Claude Code",
      "command": "claude",
      "installed": true,
      "flagsVerified": true,
      "available": true,
      "models": [
        { "id": "opus", "name": "Opus" },
        { "id": "sonnet", "name": "Sonnet" },
        { "id": "fable", "name": "Fable" }
      ],
      "defaultModel": "opus",
      "efforts": ["low", "medium", "high", "xhigh"],
      "permissionModes": ["ask", "bypass", "plan"],
      "supportsOpeningPrompt": true,
      "supportsResume": true,
      "note": ""
    },
    {
      "id": "shell",
      "name": "Shell",
      "command": "/bin/bash",
      "installed": true,
      "flagsVerified": true,
      "available": true,
      "models": [],
      "defaultModel": "",
      "efforts": [],
      "permissionModes": [],
      "supportsOpeningPrompt": false,
      "supportsResume": false,
      "note": "A plain terminal. No agent."
    },
    {
      "id": "codex",
      "name": "Codex",
      "command": "codex",
      "installed": false,
      "flagsVerified": false,
      "available": false,
      "models": [],
      "defaultModel": "",
      "efforts": [],
      "permissionModes": [],
      "supportsOpeningPrompt": true,
      "supportsResume": false,
      "note": "Not installed in the sandbox image. Add it to images/sandbox/Dockerfile and verify its flags against --help before enabling."
    }
  ]
}
```

`opencode` and `gemini-cli` have the same shape as `codex`. `available` is
`installed && flagsVerified`, computed server-side so the desktop never re-derives it.

### 3.9 `POST /v1/sessions`

Request:

```json
{
  "machineId": "local",
  "worktreeId": "w-4a77b1c9",
  "harness": "claude-code",
  "model": "opus",
  "effort": "high",
  "permissionMode": "ask",
  "openingPrompt": "add tests, then refactor",
  "title": "",
  "resume": "",
  "contextRef": ""
}
```

| Field | Type | Required | Default |
|---|---|---|---|
| `machineId` | string | no | `"local"` |
| `worktreeId` | string | **yes** | — |
| `harness` | string | no | `"claude-code"` |
| `model` | string | no | the harness's `defaultModel` |
| `effort` | string | no | `""` (no flag) |
| `permissionMode` | string | no | `"ask"` |
| `openingPrompt` | string | no | `""` |
| `title` | string | no | derived: harness name + " · " + worktree name |
| `resume` | string | no | `""` — AMENDED by the resume fix (2026-09-23): the **cockpit id** of the session to resume, which is what the desktop has always sent. The control plane resolves it to that session's stored `conversationId` and never passes it to the CLI. The source must be on the same harness and in the same worktree. |
| `contextRef` | string | no | `""`; stored, never interpreted (§8.3) |

201 with the session object of §3.10 and `Location: /v1/sessions/s-3f2a91c0`.

Errors: 400 `bad_request` (no `worktreeId`), 400 `unknown_harness`, 400
`unsupported_option`, 404 `worktree_not_found`, 409 `harness_unavailable`, 503
`backend_unavailable`.

AMENDED by the resume fix (2026-09-23) — a `resume` that cannot work is refused before
anything is recorded or created, each with a sentence the user can act on: 404
`session_not_found` (the source is no longer in the list), 400 `bad_request` (the
source ran on another harness, or in another worktree — the conversation store is per
worktree), 409 `conflict` (the source has no `conversationId`: it predates this
field). A harness that cannot resume still answers 400 `unsupported_option` from
`Build`, unchanged.

AMENDED by the resume fix's round-1 fix round (resume-fix-review I2, 2026-09-24) — the
conversation passed to `--resume` is the **newest of the source's own conversations the CLI
has written a transcript for**: its `conversationId` first, then its
`earlierConversationIds` newest first (§2.8). The control plane looks for
`<store>/-home-dev-work/<id>.jsonl` with one `stat` by the id it minted — it opens and reads
nothing (OD-8) — and `TestTheTranscriptLayoutIsPinnedToTheImagesCLI` fails the moment the
image installs a CLI other than the 2.1.274 that layout was observed on (the verification
table at the top of this document). When every one
is certainly absent, the resume is refused **before anything is recorded or created** with
409 `conflict`, "Nothing has been said in that session yet, so there is no conversation to
continue. Start a new session instead." — the alternative is a terminal that prints `No
conversation found with session ID` and exits. A `stat` that fails any other way is not a
refusal: that candidate is passed and the CLI judges, because OD-8 lets no feature require
the look to work. A refused resume creates no directory, not even the store.

**The response is returned as soon as the record exists and the sandbox has been
created — status `starting` — not after the sandbox is ready.** A create that blocks
for `ReadyTimeout` is a UI that appears frozen. The desktop opens the attach socket,
which retries while the status is `starting`.

### 3.10 The session object

Returned by create, list, get, patch, restart and delete.

```json
{
  "id": "s-3f2a91c0",
  "title": "Claude Code · evidence-upload",
  "machineId": "local",
  "projectId": "p-8c1d0e42",
  "branchId": "b-8c1d0e42-evidence-upload",
  "worktreeId": "w-4a77b1c9",
  "harness": "claude-code",
  "model": "opus",
  "effort": "high",
  "permissionMode": "ask",
  "openingPrompt": "add tests, then refactor",
  "status": "running",
  "exitReason": "",
  "backend": "docker",
  "isolation": "container",
  "createdAt": "2026-09-18T09:14:03Z",
  "updatedAt": "2026-09-18T09:14:07Z",
  "lastOutputAt": "2026-09-18T09:14:06Z",
  "attachCount": 1,
  "attachPath": "/v1/sessions/s-3f2a91c0/attach",
  "contextRef": ""
}
```

`status` is one of `starting`, `running`, `idle`, `exited` — and only those four
(§2.8). `exitReason` is free text for the user, e.g. `"killed"`, `"process exited"`,
`"idle timeout"`, `"sandbox disappeared while the app was not running"`.

**Never serialised:** `attachToken`, `endpoint`, `sandboxRef`, `sandboxId`, `command`,
`args`, and — AMENDED by the resume fix (2026-09-23) — `conversationId`, a harness implementation
detail the desktop must never be invited to send back as `resume`, and (2026-09-24)
`earlierConversationIds`, for the same reason. The token is a secret; the endpoint is a port on the host that nothing outside
the control plane should learn; `command`/`args` would put the user's opening prompt
into two places and invite the desktop to reconstruct argv itself. The task
implementing this defines a separate `api.sessionJSON` struct and a
`func toJSON(registry.Session, string) sessionJSON` — it does **not** put `json:"-"`
on `registry.Session` fields, because those fields must serialise into the registry
file.

### 3.11 `GET /v1/sessions[?worktreeId=&status=]`

```json
{ "sessions": [ { "...": "the object of §3.10" } ] }
```

Sorted by `createdAt` descending. Exited sessions are included — the transcript is the
artifact and `docs/FUNCTIONAL-SPEC.md` requires finished sessions stay listed. `status`
may be repeated (`?status=running&status=idle`).

### 3.12 `GET /v1/sessions/{id}`

200 with the session object, or 404 `session_not_found`.

### 3.13 `PATCH /v1/sessions/{id}`

```json
{ "title": "evidence upload — second attempt" }
```

The only patchable field. 200 with the session object. A body with any other field is
400 `invalid_json` (the decoder rejects unknown fields). An empty or
whitespace-only title is 400 `bad_request`. Renaming an exited session is allowed.

### 3.14 `POST /v1/sessions/{id}/restart`

No body. Destroys the sandbox, creates a new one from the **same** stored
`command`/`args`, keeps the same session id, sets `status` to `starting`, clears
`exitReason`, and mints a **new** attach token. 200 with the session object.

AMENDED by the resume fix (2026-09-23): **except a harness with a conversation**, whose argv is
rebuilt by `harness.Build` from the stored configuration (harness, model, effort,
permission mode, opening prompt) with a **new** `conversationId` and no `resume`.
Replaying is no longer possible: the stored argv names the old conversation with
`--session-id`, that conversation now outlives the sandbox, and the pinned CLI answers
`Session ID … is already in use`. Dropping `--resume` from a resumed session is the
"clean context" half of this section — a replay would fork the original again. The new
`command`, `args` and `conversationId` are written in the same pre-create update as
the new sandbox id and token. Everything else still replays byte for byte.

AMENDED by the resume fix's round-1 fix round (resume-fix-review I2, 2026-09-24): a
restart clears the context, **not the artifact**. After the destroy — so a first line the
old CLI wrote as it was stopped is seen — the restart looks for the old conversation's
transcript (§3.9's one `stat`). If it exists, or the look fails, the old `conversationId`
is appended to `earlierConversationIds` in the same pre-create update, so the conversation
stays named by its record and a resume issued before anyone speaks in the restarted
sandbox continues it. A conversation nobody spoke in has no transcript and is not listed.
The rebuild keeps `effort` and `permissionMode` (pinned by
`TestRestartKeepsThePermissionModeAndTheEffort`), and the rebuild and the store are
resolved **before the destroy**: a failure in either leaves the old sandbox running and the
record untouched (`TestARestartThatCannotPrepareTheStoreLeavesTheOldSandboxAlive`).

Same config, clean context — `docs/FUNCTIONAL-SPEC.md` "Restart". Resume is a separate
verb built from `POST /v1/sessions` with `resume` set, because resuming produces a new
session that continues an old transcript, and conflating the two makes "restart" lossy.

409 `conflict` if a restart is already in flight for that session.

AMENDED by the resume fix's round-1 fix round (resume-fix-review m4, 2026-09-24): also 409
`conflict` when the worktree no longer resolves; `Build`'s own answers through
`writeHarnessProblem` for a harness with a conversation (400 `unknown_harness`, 400
`unsupported_option`, 409 `harness_unavailable`, 500 `internal` for anything else); and 500
`internal` when the conversation store cannot be created. All of these come **before the
destroy**, so the old sandbox is kept.

AMENDED by the sandbox git fix's round-1 fix round (gitfix-rereview M-A, 2026-09-24): 500
`internal` **before the destroy** when the host's git cannot say how it converts line endings or
where its global excludes and attributes files are, for that worktree (§5.1 step 5). The old
sandbox is **kept**, still running, and the record still names it, so the caller can retry once
the host's git is fixed and has lost nothing.

### 3.15 `DELETE /v1/sessions/{id}[?purge=true]`

Without `purge`: destroys the sandbox, sets `status` to `exited` and `exitReason` to
`"killed"`, keeps the record. 200 with the session object. Idempotent: killing an
already-exited session is 200, not an error.

With `purge=true`: as above, then removes the record. 204, no body.

404 `session_not_found` for an unknown id in both cases.

### 3.16 `GET /v1/sessions/{id}/attach` — the upgrade

Not JSON. A WebSocket upgrade carrying the frame protocol of
`sandboxd/internal/wire`: one leading kind byte (`0x00` stdout, `0x01` stdin, `0x02`
resize, `0x03` exit) then an opaque payload, as binary messages.

Order of checks in the handler, and the order matters:

1. `Origin` present and not allowlisted → **403** `forbidden_origin`, JSON body.
2. `Host` not loopback → **403** `forbidden_host`.
3. Unknown session id → **404** `session_not_found`.
4. Session `exited`, or the backend reports no endpoint → **409** `sandbox_not_running`.
5. Not a WebSocket request → **426** `upgrade_required`.
6. `websocket.Accept`, then **`conn.SetReadLimit(backend.MaxMessageBytes)` — this is
   site three of the three named in §2.2.**
7. `backend.Attach(ctx, sandboxID, token)` — which applies the read limit on the
   upstream side, site two.
8. `proxy.RelayHooked`.

All of 1–5 happen **before** the upgrade so the failure is an HTTP status the desktop
can read, not a close code.

On relay end: decrement `attachCount`, flush `lastOutputAt`, **do not destroy
anything**. This is the plan's Deliberate refinement 1 and it is the single most
important behaviour in Group C.

---
## 4. Configuration

Every variable the control plane reads. All are optional; the defaults are a working
laptop. Parsed once, in `cmd/api/config.go`, into a `Config` struct — no package reads
`os.Getenv` at use time, so a test can construct a config and a misspelled variable is
visible in one file.

| Variable | Default | Unset behaviour |
|---|---|---|
| `PANACEA_APP_ADDR` | `127.0.0.1:8730` | Listens on loopback. **A host part that is not a loopback address makes the process exit 2 at startup with "refusing to bind <addr>: a sandbox attach socket on a non-loopback address is a shell on the LAN. Set PANACEA_APP_ALLOW_PUBLIC_BIND=1 to override." ** AMENDED by task 13: this row named `127.0.0.1`, `localhost` and `[::1]` exactly. The predicate is now `httpx.LoopbackHost` (§2.12) — the same one the `Host` guard applies to every request — so the whole of `127.0.0.0/8`, any IPv6 loopback and the fully-qualified `localhost.` are also accepted. Every address that adds is unroutable from another host, so the property this row exists to protect is unchanged; what it buys is that the bind refusal and the runtime guard cannot disagree about the same address, which two separate predicates eventually do. An empty host part (`:8730`) is **not** the default — it is every interface — and is refused. |
| `PANACEA_APP_ALLOW_PUBLIC_BIND` | unset | Unset means the refusal above is enforced. Any value other than `1` or `true` is also treated as unset. |
| `PANACEA_APP_BACKEND` | `docker` | `docker` or `kata`. An unrecognised value exits 2 naming both. `kata` is dormant (Phase 8); selecting it works but requires a kubeconfig. |
| `PANACEA_APP_SANDBOX_IMAGE` | `panacea-app/sandbox:dev` | The image every sandbox runs. Unset uses the dev tag the Phase 1 runbook builds. |
| `PANACEA_APP_STATE_DIR` | unset | Falls through `statepath.Dir`: `%LOCALAPPDATA%\PanaceaApp\state` on Windows, `$XDG_STATE_HOME/panacea-app`, then `~/.local/state/panacea-app`. AMENDED by the resume fix (2026-09-23): it also roots `conversations/`, every worktree's conversation store (§5.1 step 5). No new variable. AMENDED by the reaper-scope fix (2026-09-24): and `instance/instance-id`, this control plane's instance id (§2.7), so **two control planes on one Docker daemon need two state directories** — one directory is one owner. No new variable. |
| `PANACEA_APP_PROJECTS_DIR` | unset | `GET /v1/projects` returns `[]`, and `worktree.Provider` is `worktree.Static{}`, so every session create fails 404 `worktree_not_found`. That is correct and visible: no projects directory means no worktrees to work in. |
| `PANACEA_APP_ALLOWED_ORIGINS` | unset | Uses `httpx.DefaultOrigins()` (§2.12). When set it is a comma-separated list of **full origins** and **replaces** the defaults — it does not extend them, because an "extend" semantic makes it impossible to lock down. |
| `PANACEA_APP_IDLE_AFTER` | `90s` | How long without relayed output before `running` becomes `idle`. A Go duration string. `0` disables idle derivation; status then goes straight from `running` to `exited`. |
| `PANACEA_APP_IDLE_TIMEOUT` | `0` | How long a session may stay `idle` before its sandbox is destroyed. **`0` means never**, which is the default on purpose: an agent left thinking overnight must still be there in the morning. |
| `PANACEA_APP_REAP_INTERVAL` | `30s` | How often the reaper sweeps. Minimum enforced value `5s`. |
| `PANACEA_APP_SESSION_CPU_MILLIS` | `2000` | `backend.Resources.CPUMillis`. |
| `PANACEA_APP_SESSION_MEMORY_MIB` | `4096` | `backend.Resources.MemoryMiB`. |
| `PANACEA_APP_DOCKER_HOST` | unset | Unset means `client.FromEnv`, i.e. `DOCKER_HOST`, i.e. `npipe:////./pipe/docker_engine` on Windows with Docker Desktop. |
| `PANACEA_APP_KUBECONFIG` | unset | `kata` backend only. Unset means in-cluster config; outside a cluster that fails at startup with `backend_unavailable`. |
| `PANACEA_APP_NAMESPACE` | `panacea-app` | `kata` backend only. |
| `PANACEA_APP_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Anything else falls back to `info` with one warning line rather than exiting. |
| `PANACEA_APP_HARNESS_ALLOW_UNVERIFIED` | unset | When `1`, `harness.Build` will construct argv for a harness whose flags were never verified. Nothing in Phase 2 sets it; it exists so adding Codex is a flag flip plus a `--help` check, not a code change. |
| `PANACEA_APP_GIT_NAME` | unset | ADDED by the sandbox git fix (2026-09-23). With `PANACEA_APP_GIT_EMAIL`, the identity every commit in every sandbox is made as, passed in as `GIT_AUTHOR_*` and `GIT_COMMITTER_*`. **A pair or nothing:** one without the other exits 2 naming both. Unset, every session create and restart asks the host's own git, **in that worktree**, with `git var GIT_AUTHOR_IDENT`: the identity a commit there would carry on the host, whether it comes from the repository's config, an `includeIf`, global or system config, or git's own derivation from the account. If that fails too, the session still starts, the control plane logs "no git identity, so a commit in a sandbox will fail: set PANACEA_APP_GIT_NAME and PANACEA_APP_GIT_EMAIL, or give this machine's git an identity …" (once at startup, and at each create), and git in the sandbox refuses to commit with its own "Author identity unknown" rather than invent `dev@<container>`, because `user.useConfigOnly` is set (§5.1 step 5). |
| `PANACEA_APP_GIT_EMAIL` | unset | ADDED by the sandbox git fix. The other half of `PANACEA_APP_GIT_NAME`'s pair; that row is the whole behaviour. |
| `PANACEA_APP_CA_BUNDLE` | unset | ADDED by the sandbox git fix. An **absolute host path to a PEM file** of root certificates every sandbox trusts **in addition to** its own. It is mounted **read-only** at `/etc/panacea-app/ca-bundle.pem` and named by `NODE_EXTRA_CA_CERTS`, which Node — and so Claude Code — adds to its trust store. Unset mounts nothing and sets nothing. Set, it is checked at startup, and a relative path, a missing file, a directory, or a file holding no `CERTIFICATE` block x509 can parse each exits 2 with its own sentence. `SSL_CERT_FILE` is never set, because it replaces the system bundle, and TLS verification is never disabled under any setting. `kata` refuses the mount (§2.6). |

**Behind a TLS-intercepting proxy.** A corporate proxy that re-signs TLS makes every sandbox fail
Claude Code's startup check with `Failed to connect to api.anthropic.com: SELF_SIGNED_CERT_IN_CHAIN`,
because the image does not trust the proxy's root. The owner exports that root once and points
`PANACEA_APP_CA_BUNDLE` at it:

1. See which root the proxy presents. From Git Bash:
   `openssl s_client -showcerts -connect api.anthropic.com:443 < /dev/null`. The last certificate
   listed is self-signed; its `CN` is the root to export.
2. Export it from the Windows root store as PEM, in PowerShell, putting that `CN` in the filter:

   ```powershell
   New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\PanaceaApp" | Out-Null
   $root = Get-ChildItem Cert:\LocalMachine\Root | Where-Object Subject -like '*CN=<root CN>*' | Select-Object -First 1
   "-----BEGIN CERTIFICATE-----`n" + [Convert]::ToBase64String($root.RawData, 'InsertLineBreaks') + "`n-----END CERTIFICATE-----" | Set-Content -Encoding ascii "$env:LOCALAPPDATA\PanaceaApp\corp-root.pem"
   ```

3. `$env:PANACEA_APP_CA_BUNDLE = "$env:LOCALAPPDATA\PanaceaApp\corp-root.pem"`, then start the
   control plane. Sessions created from then on can reach the API; `/login` is still the user's own.

The file is this network's, not the product's: it is never committed. **Git over HTTPS is not
covered.** Git does not read `NODE_EXTRA_CA_CERTS`, and its own `GIT_SSL_CAINFO` replaces the
system bundle rather than extending it. Nothing in Phase 2 pushes or fetches from a sandbox; Phase 3
decides it with push.

**`PANACEA_APP_ATTACH_TOKEN` is deliberately NOT in this list.** It is generated
per sandbox by `ids.Token()` and injected into that sandbox's environment. A global
attach token would give every sandbox the same secret and make the Group A task 3
defect fix pointless. `sandboxd` reads that variable inside the container; the control
plane never does.

**`PANACEA_APP_MODEL_*`** (base URL, API key, model id) belong to Phase 4's
`modelclient` and are not read by anything in Phase 2. They are listed here only so a
task author does not think they are missing.

---

## 5. Lifecycle and status

### 5.1 When a sandbox is created

Exactly one place: the `POST /v1/sessions` handler, in this order.

1. Resolve the worktree: `worktree.Provider.Path(ctx, worktreeID)`. Failure → 404. This
   happens **first**, before anything is created, so a bad id costs nothing.
2. Resolve the harness: `harness.Lookup` then `harness.Build`. Failure → 400 or 409.
   AMENDED by the resume fix (2026-09-23): for a harness with a conversation (available and
   `SupportsResume`), first resolve a non-empty `resume` to the source record's
   `conversationId` (§3.9's refusals — AMENDED 2026-09-24: the newest of its own
   conversations with a transcript, else 409), then mint `conversationId := ids.UUID()`, pass
   both to `Build`, and create the worktree's conversation store on the host,
   `<ConversationsDir>/<owner>/<worktreeId>` with `<owner>` = `local` in Phase 2 — all
   before step 4, so a failure costs nothing.
3. Mint `sessionID := ids.New("s")` and `token := ids.Token()`.
4. `store.Create(session)` with `Status: StatusStarting`. **The record exists before
   the sandbox does.** If step 5 fails the record is the only evidence of what
   happened, and without it a half-created container becomes an orphan nobody can name.
5. `backend.Create(ctx, spec)` with `Mounts: [{HostPath: worktreePath, ContainerPath: "/home/dev/work"}]`
   and `WorkDir: "/home/dev/work"` for the Docker backend. AMENDED by the resume fix (2026-09-23): a
   harness with a conversation gets a second mount, `{HostPath: <its conversation
   store>, ContainerPath: "/home/dev/.claude/projects"}` — `projects/` and nothing
   wider, because `/login` writes `.credentials.json` into `~/.claude` itself.
   `images/sandbox/Dockerfile` creates that path as `dev`; without it Docker makes
   `~/.claude` root-owned to hold the mount point and `/login` breaks. AMENDED by the sandbox git
   fix (2026-09-23): every sandbox, whatever its harness, also gets `Spec.Env` from `sessionEnv` —
   `GIT_CONFIG_COUNT`/`KEY_n`/`VALUE_n` setting `safe.directory` to the mount point (the
   `sessionWorkDir` constant, never `*`) and `user.useConfigOnly=true`; the user's identity as
   `GIT_AUTHOR_NAME`/`EMAIL` and `GIT_COMMITTER_NAME`/`EMAIL` when one resolves (§4
   `PANACEA_APP_GIT_NAME`); and, when `PANACEA_APP_CA_BUNDLE` is set, a further (AMENDED, was
   "third": the order is at the end of this step), **read-only** mount of
   that bundle at `/etc/panacea-app/ca-bundle.pem` with `NODE_EXTRA_CA_CERTS` naming it. The
   environment is resolved before step 4, and restart rebuilds it rather than replaying it. A
   linked worktree still has no working git — its `.git` names a host path — and that is Phase 3's
   `3-D1`; the cockpit says so from `Worktree.isPrimary`. AMENDED by the sandbox git fix, round 1
   (2026-09-24): `GIT_CONFIG_COUNT` is **6** (AMENDED by its fix round: **8**, the last two pairs
   at the end of this step). The four added pairs are
   `gc.worktreePruneExpire=never`, `gc.auto=0`, and `core.autocrlf` and `core.eol` set to **the host's own
   answer for that worktree** from `Deps.GitLineEndings`, which is required. cmd/api asks the host's
   git in the worktree: `git config --get --type=bool --default false core.autocrlf` (the raw value
   only when that refuses `input`) and `git config --get --default native core.eol`, with
   `native` turned into the host's own ending, `crlf` on Windows. The container never sees the
   host's system or global config, and Git for Windows sets `core.autocrlf=true` at system scope,
   so without these a CRLF checkout reads as every file modified and `commit -a` writes CRLF blobs.
   The repository's own `.gitattributes` and `.git/config` are in the mount and apply unchanged. An
   error, or an answer that is not `true`/`false`/`input` and `lf`/`crlf`, fails the create or
   restart with 500 before step 4 and before the restart's destroy. A primary worktree, one whose
   `.git` is a directory, also gets a **read-only** mount of its `<worktree>/.git/worktrees` at
   `/home/dev/work/.git/worktrees`, made on the host when absent. Every linked worktree's `gitdir`
   there names a host path, so the sandbox's git calls each one prunable. The mount stops a
   `git worktree prune`, `repair`, `move`, `remove` or `add` in the sandbox from changing the
   host's metadata; no git setting stops an explicit prune. The refused prune still **exits 0**
   (gitfix-rereview E3: `error: failed to delete '.git/worktrees/wt4': Read-only file system`,
   then rc 0), so a prune's exit code is not proof that anything was deleted. It also makes a `git gc` in the
   sandbox exit 128, because gc's reflog expiry cannot lock a linked worktree's `HEAD` there;
   `gc.auto=0` stops the sandbox starting a detached auto gc, whose failure would leave a
   `.git/gc.log` that makes the host's own auto gc stand down for a day. The host's git keeps
   the repository. The mount order is the worktree, then
   that mount, then the conversation store, then the CA bundle, then (AMENDED by the fix round below)
   the excludes file, then the attributes file. AMENDED by the sandbox git fix's round-1 fix round
   (gitfix-rereview I-A, 2026-09-24): git reads a **global excludes file** even when
   `core.excludesFile` is unset, `$XDG_CONFIG_HOME/git/ignore` or else `~/.config/git/ignore`, and
   the sandbox's `HOME` has none. On the owner's host that file ignores
   `**/.claude/settings.local.json`, so an agent's `git add -A` committed a user's permission
   allow-list (measured, E4). Pairs 7 and 8 are `core.excludesFile` and `core.attributesFile`,
   from `Deps.GitGlobalFiles`, which is required. cmd/api asks the host's git in the worktree:
   `git config --get --type=path --default <git's default> core.excludesFile`, and the same for
   `core.attributesFile`, where the default is `$XDG_CONFIG_HOME/git/<ignore|attributes>` when
   that variable is set and not empty, else `~/.config/git/<ignore|attributes>`. A relative answer
   is joined to the worktree, and an empty value is no file. A path that is a **regular file** on
   the host, following a link as git does, is mounted from the file it resolves to, so what a
   Docker host does with a link as a mount source never matters (dotfile managers link
   `~/.config/git/ignore`), **read-only** at `/etc/panacea-app/git/ignore` or
   `/etc/panacea-app/git/attributes`, and its pair names that path. Anything else (nothing there,
   a directory, a file git cannot read, no path) gets no mount, and its pair names `/dev/null`,
   because the host's git reads no patterns there. The pair is never left unset, since then the
   sandbox's git would read its own `~/.config/git` default, which the host never reads and an
   agent can write. An error from the resolver, or a relative path from it, fails the create or
   restart with 500 before step 4 and before the restart's destroy. The **system** attributes
   file (`$(prefix)/etc/gitattributes`) is not forwarded: git has no key that names it, and Git
   for Windows' own holds only `diff=astextplain` lines, which change how a diff is shown and not
   what a commit holds. Failure → `store.Update` to
   `StatusExited` with the reason, then 500 or 503.
6. Return 201 immediately.
7. **In a goroutine**, `backend.WaitReady(ctx, sb.Endpoint, backend.ReadyTimeout)`; on
   success `store.Update` to `StatusRunning` and record the endpoint; on failure
   `backend.Destroy` and `store.Update` to `StatusExited` with
   `"sandbox never became ready"`.

Anything Phase 4 later needs on disk before the agent starts — `CLAUDE.md` — is written
between steps 1 and 5, never after. Claude Code reads it at process start.

### 5.2 When a sandbox is destroyed

Three causes, and no others:

1. **Explicit kill** — `DELETE /v1/sessions/{id}`.
2. **Idle TTL** — the reaper, only when `PANACEA_APP_IDLE_TIMEOUT` > 0, which it is not
   by default.
3. **Reap of an orphan** — a sandbox carrying `panacea-app.managed=true` whose
   `panacea-app.sandbox-id` matches no registry record. AMENDED by the reaper-scope fix
   (2026-09-24): **and whose `panacea-app.instance` is this control plane's.** Another
   control plane's sandbox matches no record here either, by construction; that is not an
   orphan. A managed sandbox with no instance label is never reaped by anyone (§5.3).

**A dropped WebSocket destroys nothing.** `docs/FUNCTIONAL-SPEC.md` requires finished
sessions stay listed and readable and that Resume restores context by running
`claude --resume`, which reads state from `~/.claude` inside the sandbox. Destroying on
disconnect deletes the artifact the product is built around. (AMENDED by the resume fix (2026-09-23):
the transcript `--resume` reads is in the worktree's conversation store, mounted at
`~/.claude/projects`, so it survives even the three permitted destroys. Without that,
Resume — which always runs in a NEW sandbox — found nothing.) The relay goroutine's
`defer` decrements `attachCount` and flushes `lastOutputAt`; that is all it does.

### 5.3 The reaper

`reaper.Reaper` runs `Sweep(ctx)` every `PANACEA_APP_REAP_INTERVAL` (default 30s) and
`Sweep` is exported so the test calls it directly with a fake clock and never sleeps.

```go
type Config struct {
	Store       registry.Store
	Backend     backend.Backend
	IdleAfter   time.Duration
	IdleTimeout time.Duration
	Interval    time.Duration
	Now         func() time.Time
	Log         *slog.Logger
}

func New(cfg Config) *Reaper
func (r *Reaper) Sweep(ctx context.Context) error
func (r *Reaper) Run(ctx context.Context)   // ticks until ctx is done
func (r *Reaper) BackendReady() bool        // cached for GET /v1/healthz
```

One sweep, in order:

1. `live, err := backend.List(ctx)`. On error, set `backendReady = false`, log once at
   warn, and **return without changing any session**. A Docker daemon that is
   restarting must not be read as "every sandbox is gone" — that would mark every
   running session `exited` and the user would lose the rail.
2. For every registry session with a non-empty `SandboxID` and status not `exited`,
   where the id is absent from `live`: `Status = StatusExited`,
   `ExitReason = "sandbox disappeared"`.
3. For every session present in `live` whose backend `State` is `StateStopped`:
   `Status = StatusExited`, `ExitReason = "process exited"`.
4. For every live sandbox with `LabelManaged == "true"` whose `LabelSandboxID` matches
   no registry record: `backend.Destroy`, log at info with the sandbox id. This is the
   crash-recovery path, and it is why the record is written before the container.
5. Idle derivation, only when `IdleAfter > 0`: a session with status `running` and
   `Now().Sub(LastOutputAt) > IdleAfter` becomes `idle`. A session with status `idle`
   whose `LastOutputAt` has moved goes back to `running`.
6. Idle TTL, only when `IdleTimeout > 0`: a session with status `idle` and
   `Now().Sub(LastOutputAt) > IdleTimeout` has its sandbox destroyed;
   `Status = StatusExited`, `ExitReason = "idle timeout"`.
7. Refresh `backendReady = true`.

Every state change goes through `store.Update`, never `Get` then `Create`.

AMENDED by the reaper-scope fix (2026-09-24, `task-25-26-reconcile.md` item 11). **Rule 4 destroyed
other control planes' sandboxes.** Every control plane on a daemon sees every managed container,
none of the others' are in its registry, and `Run` sweeps once at startup, before its ticker — so
starting a second control plane destroyed the first's sandboxes older than the create grace, and
`PANACEA_APP_REAP_INTERVAL=24h` protected nobody. The fix:

- `Config` gains `Instance string` (alongside `CreateGrace`, which the listing above also predates).
  `cmd/api` passes the id `instance.Load` returned — the same one the backend stamps (§2.7).
- The two rules that destroy a sandbox no record asked them to — rule 4, and task 5's rule 1a
  (destroy a `NonLoopback` sandbox) — act **only on a sandbox whose `Sandbox.Instance` equals
  `Config.Instance`**. This holds whether or not the backend's `List` is scoped; both are (§2.4,
  §2.6), and the reaper does not lean on it.
- Everything else is **foreign** and spared. A sandbox with no instance label is logged once at warn
  (it predates the label; nobody can prove whose it is; nothing will ever collect it). Another
  instance's is logged once at debug (its own reaper looks after it). The logged-once set is pruned
  each sweep to what is live.
- **An empty `Config.Instance` owns nothing, not everything.** Read the other way, `"" == ""` would
  hand this reaper every legacy sandbox on the daemon. `New` warns once; rules 2, 3, 5 and 6 still
  run, since they act only on records this registry holds.

Rules 2, 3, 5 and 6 are unchanged: they are keyed on this registry's own sandbox ids.

### 5.4 How `idle` is derived, without reading a byte

`proxy.Relay` gains a hooked variant:

```go
// Hooks observe the SIZES of relayed messages. They never receive payload bytes,
// and there is no variant that does: docs/PROGRAM.md forbids deriving meaning
// from agent output absolutely, and a hook holding a []byte is an invitation.
type Hooks struct {
	OnUpstream func(n int) // sandbox → desktop, i.e. the agent produced output
	OnClient   func(n int) // desktop → sandbox, i.e. the user typed
}

// RelayHooked is Relay with observation. Relay(ctx, c, u) is
// RelayHooked(ctx, c, u, Hooks{}) and keeps its existing signature and tests.
func RelayHooked(ctx context.Context, client, upstream *websocket.Conn, h Hooks) error
```

The attach handler passes `OnUpstream` that records `time.Now()` into a local variable
and, **at most once every two seconds**, flushes it to the registry via
`store.Update`. Per-frame persistence would rewrite the JSON document thousands of
times a second. The final value is flushed unconditionally when the relay ends.

`idle` therefore means exactly: *attached or not, no output has crossed the relay for
N seconds.* It is not "the agent is waiting for you" — that is `BLOCKED-ON-YOU`, which
needs the agent-runtime signal `docs/AGENTIC-UX.md` §3 calls a hard runtime dependency
and is not invented here.

### 5.5 What a control-plane restart recovers

`cmd/api/main.go` runs `recover(ctx)` once before the listener starts.

1. Load the registry from disk. A missing file is an empty registry, not an error.
2. `backend.List(ctx)`. If it fails, **leave every session's status alone**, log at
   warn, and let the reaper converge once the daemon is back. Marking everything
   `exited` because Docker Desktop had not finished starting is the failure mode that
   loses a night's work.
3. For each session whose `SandboxID` is in `live`:
   - **Re-read `Endpoint` from the backend and overwrite the stored one.** This is the
     Docker gotcha that will bite: a container that is restarted, or a daemon that is
     restarted, republishes the sandbox port on a **different** ephemeral host port.
     The endpoint on disk is stale the moment Docker Desktop restarts, and attaching to
     it yields connection-refused that looks exactly like a dead agent.
   - Set status from the backend state: `StateRunning` → `running` (the reaper
     re-derives `idle` on its first sweep), `StateStopped` → `exited` with
     `"process exited"`.
   - `AttachCount = 0` — no client is connected to a process that just started.
4. For each session whose `SandboxID` is **not** in `live` and whose status is not
   already `exited`: `exited`, with
   `"sandbox disappeared while the app was not running"`.
5. Orphans are left to the reaper's first sweep, which happens immediately on start.

The attach token survives in the registry file, which is why the file is `0o600` and
why `writeAtomic` chmods the temp file before the rename rather than after.

AMENDED by the resume fix (2026-09-23): `conversationId` survives the same way, and no step above
writes it, so a resume issued after a control-plane restart finds the conversation. It
is pinned both by `TestRecoverKeepsEveryConversationID` and by a resume through a
second API over the reopened registry file. AMENDED by the resume fix's round-1 fix round
(2026-09-24): `earlierConversationIds` survives the same way, pinned by the same test and by
`TestSessionsSurviveAReopen`.

AMENDED by the reaper-scope fix (2026-09-24): `live` is this instance's sandboxes only (§2.4), and
the instance id is this state directory's, so a restart on the same directory recovers exactly what
it made. **One consequence, stated rather than glossed:** a session created by a build from before
this fix has a container with no instance label. The first start of the new build does not list it,
so step 4 marks the session `exited` with `"sandbox disappeared while the app was not running"`, and
its container — foreign, so never reaped — keeps running until someone removes it by hand. Phase 2
has not shipped, so the only such containers are development ones; the warning `List` writes names
each one.

### 5.6 The complete status machine

```
                 create                 healthz 200
        (nothing) ──────► starting ──────────────────► running
                             │                          │  ▲
                             │ WaitReady failed         │  │ output resumes
                             ▼                          ▼  │
                          exited ◄──────────────────── idle
                             ▲     kill · process exit   │
                             │     sandbox gone          │
                             └───────────────────────────┘
                                   idle TTL (opt-in)
```

No other transitions exist. In particular there is no transition out of `exited`:
restart keeps the id but re-enters at `starting`, and resume creates a new session —
AMENDED by the resume fix (2026-09-23) — whose conversation is a named fork of the source's.

---
## 6. The task list

Five in Group B, five in Group C, as the plan's task-group table requires. Task 8 is
the widest of the ten; the note under it says what to do if an executor finds it long.

### Group B — the sandbox backend interface (tasks 4–8)

| # | Title | Scope in one line | Files | Depends on |
|---|---|---|---|---|
| **4** | The backend seam and the one dialer | `backend.Backend`, `Spec`, `Sandbox`, `Resources`, sentinel errors, shared constants, `Dial` with the bearer header and `SetReadLimit`, `WaitReady`, and `ids` | C `control-plane/internal/ids/ids.go`, `internal/backend/backend.go`, `internal/backend/dial.go`; T `ids/ids_test.go`, `backend/backend_test.go`, `backend/dial_test.go` | Group A task 3 (the `PANACEA_APP_*` env contract and bearer auth on `/attach`) |
| **5** | The Docker backend | `dockerbe`: narrow SDK interface, create with `127.0.0.1` port publishing, label discovery, endpoint read-back with the non-loopback refusal, idempotent destroy | C `internal/backend/dockerbe/api.go`, `internal/backend/dockerbe/docker.go`; M `control-plane/go.mod`, `go.work.sum`; T `internal/backend/dockerbe/docker_test.go` | 4 |
| **6** | The Kata backend, dormant | `katabe` over `internal/sandbox.Manager`; the manager grows `ArgsJSON`/`AttachToken`/`Env`/`SessionID` on `Spec`, plus `Get`, `List` and `ErrNotFound` | C `internal/backend/katabe/kata.go`, `katabe/kata_test.go`; M `internal/sandbox/manager.go`, `internal/sandbox/manager_test.go` | 4 |
| **7** | The session registry | `statepath.Dir`, `registry.Session`, `registry.Status`, `Store` with `Update` as the only mutation primitive, `FileStore` with `schemaVersion: 1` and the Windows-safe atomic replace | C `internal/statepath/statepath.go`, `internal/registry/session.go`, `internal/registry/registry.go`, `internal/registry/filestore.go`; T `statepath/statepath_test.go`, `registry/filestore_test.go`, `registry/registry_test.go` | — |
| **8** | Harnesses, worktrees and projects | `harness` catalogue + `Build`; `worktree.Provider` + `Static`; `project.Scanner` over `git`, implementing `Provider` and Phase 4's `Naming` | C `internal/harness/harness.go`, `internal/harness/argv.go`, `internal/worktree/worktree.go`, `internal/project/project.go`; T one `_test.go` per file | — |

**Note on task 8.** It is three small packages rather than one, and it is the only task
in Group B that could reasonably be two. Keep it whole: `harness.Build` and
`project.Scanner` share nothing, but splitting them produces a task 8a with no
consumer, and the executing agent for task 11 needs both on the page at once. If it
runs long, the natural cut is `project` into its own task numbered 8b, leaving the
numbering of 9–13 untouched.

### Group C — the control-plane API (tasks 9–13)

| # | Title | Scope in one line | Files | Depends on |
|---|---|---|---|---|
| **9** | The error body and the origin guard | `httpx.Problem`, `WriteJSON`, `WriteProblem`, `DecodeJSON`, `OriginPolicy` with the full-origin allowlist, the loopback `Host` guard and the explicit log line on rejection | C `internal/httpx/problem.go`, `internal/httpx/origin.go`; T `httpx/problem_test.go`, `httpx/origin_test.go` | — |
| **10** | The router and the read-only surface | `api.Deps`, `api.New`, the mux, the `/api/v1` alias, then `/healthz`, `/machines`, `/projects`, `/branches`, `/worktrees`, `/harnesses` | C `internal/api/api.go`, `internal/api/catalog.go`; T `api/api_test.go`, `api/catalog_test.go` | 8, 9 |
| **11** | Session CRUD | create (resolve → record → sandbox → 201 → async ready), list with filters, get, patch-rename, restart, delete with and without `purge`; `api.sessionJSON` so the token and endpoint cannot leak | C `internal/api/sessions.go`; T `api/sessions_test.go` | 4, 7, 8, 10 |
| **12** | The attach relay | the five pre-upgrade checks, `SetReadLimit` site three, `proxy.RelayHooked`, the throttled `lastOutputAt` flush, `attachCount`, and a test proving a dropped socket destroys nothing | C `internal/api/attach.go`; M `internal/proxy/proxy.go`; T `api/attach_test.go`, `proxy/proxy_test.go` | 4, 7, 11 |
| **13** | Config, reaper, wiring and restart recovery | `cmd/api/config.go` reading every `PANACEA_APP_*`, the loopback bind refusal, `reaper.Sweep`'s six rules, `recover()` with the endpoint re-read, graceful shutdown | C `internal/reaper/reaper.go`, `cmd/api/config.go`; M `cmd/api/main.go`, `cmd/api/main_test.go`; T `reaper/reaper_test.go`, `cmd/api/config_test.go` | 5, 6, 10, 11, 12 |

Legend: **C** create, **M** modify, **T** test.

Every task's commit step ends with the attribution line
`Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`, and every task
runs `go test ./sandboxd/... ./control-plane/... -race -count=1` — never
`go test ./...`, which errors at this workspace root with
`directory prefix . does not contain modules listed in go.work`.

---

## 7. Phase 4 reconciliation

Phase 4 was planned against a Phase 2 that did not exist. Four contracts cross the
boundary. Three are provided here under Phase 4's exact names; one is a genuine
discrepancy and is recorded rather than papered over.

### 7.1 `PANACEA_APP_STATE_DIR` and the state-directory ladder — provided, identical

Phase 4's `discoverstore.DefaultDir()` (phase-4 plan lines 3200–3215) reads
`PANACEA_APP_STATE_DIR`, then `LOCALAPPDATA`, then `XDG_STATE_HOME`, then
`~/.local/state`, then `os.TempDir()`. `statepath.Dir` (§2.7) is that ladder
character-for-character, parameterised by the subdirectory name. Phase 4's function
becomes `return statepath.Dir("discover")` when Phase 2 lands, and Phase 2's registry
is `statepath.Dir("sessions")`. **Nothing in Phase 4 changes if that substitution is
never made** — the two implementations agree — but a task author for Phase 4 should be
told the package exists.

### 7.2 `discoverstore.Store` — the seam holds, and Phase 2 does not fill it

Phase 4's Task 7 says "Phase 2 owns the app database. When Phase 2's database lands,
implement `Store` over it and delete `FileStore`." **Phase 2 ships no database** (§1.3).
`discoverstore.FileStore` therefore stays, and stays correct: it is the same
one-JSON-document-per-thing pattern `registry.FileStore` uses, in the same state
directory tree. The seam is honoured by Phase 2 declining to change it. If a database
is ever added, both stores move together.

One improvement Phase 2 offers and Phase 4 should adopt: `discoverstore.FileStore.Save`
writes through `os.Rename` with no retry. On Windows that rename intermittently fails
with `ERROR_ACCESS_DENIED` when a scanner has the target open (§2.8). Phase 2's
`writeAtomic` retries five times with backoff. Phase 4's store should use the same
helper.

### 7.3 `discoverapi.Naming` — provided, under Phase 4's exact method names

Phase 4 declares (plan lines 3606, 3995–4016):

```go
type Naming interface {
	ProjectName(projectID string) string
	BranchName(branchID string) string
}
```

with the contract that neither method returns an error and both fall back to the id
itself. `project.Scanner` (§2.11) implements exactly that method set with exactly that
fallback, so `discoverapi.Deps{Naming: scanner}` compiles with no adapter and
`StaticNaming` remains the test double. **No discrepancy.**

### 7.4 `contextwrite.WorktreeResolver` — provided, under Phase 4's exact signature

Phase 4's Task 5 declares
`Path(ctx context.Context, worktreeID string) (string, error)` and calls it a Phase 3
seam. `worktree.Provider` (§2.10) is that one-method interface verbatim, and
`project.Scanner` implements it. Phase 4's `contextwrite.NewWriter(resolver)` therefore
takes a `*project.Scanner` directly.

Phase 4 also ships `contextwrite.StaticResolver map[string]string`; Phase 2 ships
`worktree.Static map[string]string`. These are two names for one type. **Keep both** —
neither phase should be made to rename a type its tests are written against — and note
that they are structurally identical so either satisfies the other's interface.

**The seam hazard is restated here because Phase 2 owns the code path that breaks it:**
`Materialise` must run **before** the sandbox is created, never after. Claude Code
reads `CLAUDE.md` at process start. §5.1 puts the write between step 1 (resolve the
worktree) and step 5 (create the sandbox), and the Phase 3 and Phase 4 plans both say
the same sentence.

### 7.5 DISCREPANCY: the route prefix

Phase 4 mounts every Discover route under **`/api/v1/discover`** (plan line 3610, and
every test from line 3718 on). The Phase 2 plan's architecture diagram (line 13) and
this contract use **`/v1`**.

**Resolution, and it is deliberate:** `/v1` is canonical, and the mux additionally
serves `/api/v1/…` as an alias via
`mux.Handle("/api/v1/", http.StripPrefix("/api", mux))`. Phase 4's routes and its
entire test suite then land unchanged, and the desktop's Phase 2 client keeps the
shorter prefix the Group A header already committed to. The alias is one line in
`api.New` and is called out in task 10 so nobody deletes it as dead code.

The alternative — rewriting either plan's prefix — costs a rewrite of Phase 4's tests
or an edit to the already-written Group A header, for no behavioural gain.

### 7.6 `provenance.Ref` and `contextRef` — the field Phase 2 must carry

Phase 4's Task 12 mounts `ContextChip` "on every session header" (plan lines 5596,
5755) and builds it from `provenance.Ref{BriefID, Revision, …}`. That requires the
session to know which brief it inherited.

`registry.Session.ContextRef` (§2.8) is that field, and `POST /v1/sessions` accepts
`contextRef` and returns it (§3.9, §3.10). **Phase 2 stores it as an opaque string and
never interprets it** — it does not know what a brief is, does not validate the id, and
does not fail a session create when the brief is missing. Phase 4 resolves it through
its own endpoint.

This is the one place Phase 2 adds a field purely for a later phase. It is worth it:
retrofitting it means a registry schema bump and a migration for every existing
session, and `docs/PROGRAM.md` names provenance as the thread that must not break.

---

## 8. Open questions — things a task author must not silently resolve

1. **Claude Code's "ask" permission mode.** `--permission-mode` offers `acceptEdits`,
   `auto`, `bypassPermissions`, `manual`, `dontAsk`, `plan` — there is no `default`,
   and `--help` does not say which one is the out-of-the-box behaviour. §2.9 sidesteps
   it by emitting no flag at all for `ask`, which is provably the CLI's own default.
   Before Phase 8 adds an admin control over permitted modes, someone must run
   `claude --permission-mode manual` against a real session and confirm what it does.
2. **Codex, OpenCode and Gemini CLI flags are entirely unverified**, and none of the
   three binaries is in `images/sandbox/Dockerfile`. They ship as
   `installed: false, flagsVerified: false` with a note. Enabling one is a Dockerfile
   line, a descriptor change and a `--help` check — not a guess.
3. **`container.Summary.State` values.** `container.ContainerState` is a type alias for
   `string` with constants `created`, `running`, `paused`, `restarting`, `removing`,
   `exited`, `dead` (state.go lines 11–24). The mapping in §2.4 was written from those
   constants, not from a live daemon. It should be checked once against real
   `docker ps` output during task 5's verification step.
4. **Docker Desktop on Windows and the IPv6 port binding.** §2.4 asserts that a
   container bound to `127.0.0.1` yields one usable IPv4 binding and possibly a
   separate IPv6 one, and that a `0.0.0.0` binding is a defect rather than a variation.
   That was reasoned from the SDK types, not observed. Task 5's verification step must
   run one container and print `docker port` before the code trusts it.
5. **Whether `go mod tidy` will resolve the cached Docker versions offline.** Every
   module is present in the download cache (§2.4), but `go mod tidy` consults the proxy
   for the *latest* version of anything under-specified, and `proxy.golang.org` is
   TLS-intercepted from this network. Task 5 must add explicit `require` lines and run
   with `GOFLAGS=-mod=mod GOPROXY=off`, and must diff `control-plane/go.mod` afterwards
   to prove the `go 1.23` directive was not bumped.
6. **The Tauri origin in a packaged macOS or Linux build was not observed**, only
   derived from tauri's source (`window_origin` in `crates/tauri/src/manager/webview.rs`
   and `use_https_scheme` in `crates/tauri-utils/src/config.rs`). All five plausible
   values are in the default allowlist and a rejection logs the value it saw, so a
   wrong derivation costs one log line and one config change rather than a silent 403.
7. **Machine states beyond `running` and `error`.** `docs/FUNCTIONAL-SPEC.md` specifies
   six (`provisioning · running · idle · sleeping · stopped · error`) and implicit wake.
   Phase 2 has one machine that is either up or not. The other four states need cloud
   machines, i.e. Phase 8. The JSON field is the full enum so the cockpit does not need
   changing later; Phase 2 simply never emits the other four.
8. **`sessionCount` on a worktree is an O(n) scan of the registry** per worktree per
   request. At tens of sessions that is free. If the rail ever polls at 1 Hz with
   hundreds of sessions it is not, and the fix is an index in the registry rather than
   a cache in the API.
