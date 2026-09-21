### Task 5: The Docker backend

This is the task that makes `docs/PROGRAM.md`'s local-first pivot real. With it, the whole product runs on a Windows laptop with Docker Desktop and zero cloud spend. It is not scaffolding for the Kata backend and it is not a test double: `docs/PROGRAM.md` ships a "local machine" alongside cloud machines, and this package *is* that feature. Task 6's `katabe` is the one that is dormant.

Two rules from Global Constraints are load-bearing here and both are pinned by tests rather than by review. **Publishing a sandbox port to `0.0.0.0` is a security defect, not a preference** — it puts an unauthenticated-by-default shell on the corporate LAN, guarded only by a bearer token that is also on that LAN. `backend.ErrNonLoopbackPublish` exists for exactly this, `New` refuses a non-loopback `BindIP` unless someone sets `AllowNonLoopback` on purpose, and when the daemon reports an off-loopback binding for a container we just created, `Create` destroys it before returning the error. **And `Destroy` is idempotent** — removing an already-gone container is success, because the task 13 reaper and an explicit `DELETE /v1/sessions/{id}` race by design, and a reaper that errors on "already gone" stops sweeping.

The package is two files, which is §1.2 correction 2 of the contract adding `api.go` to the plan's File structure table. The table lists only `docker.go`; follow 1.2. The reason is testability: `APIClient` is the narrow slice of the Docker SDK this package uses, copied verbatim from `github.com/docker/docker@v28.3.3+incompatible/client/client_interfaces.go` lines 72–103, so `*client.Client` satisfies it structurally and the unit tests need no Docker daemon. Do not "tidy" those signatures — v28 renamed `types.Container` to `container.Summary` and `types.ContainerJSON` to `container.InspectResponse`, and a signature written from memory will not compile. `docker.go` carries `var _ APIClient = (*client.Client)(nil)` so a future SDK bump breaks the build here rather than at run time.

Four decisions where the contract was silent, made here so nobody re-derives them. **(a) `Create` waits for readiness and is all-or-nothing.** `dial.go`'s `ReadyTimeout` comment says "how long Create waits for a new sandbox to answer `/healthz`", but §2.4 never says what happens when it does not: the choice made here is destroy-then-error, because a container that never served `/healthz` has no session record, is therefore an orphan, and would be reaped anyway — destroying it inside `Create` makes the failure immediate and deterministic instead of arriving one reaper interval later. The cost is that `docker logs` for that container is gone, so the error message names the endpoint and the timeout that expired, which is what makes the failure diagnosable from the API response alone. **There is no `PANACEA_APP_READY_TIMEOUT` variable today** — contract §5's `PANACEA_APP_*` table is closed and contains no such entry, and neither this task nor task 13 as written adds one — so do not tell an operator to raise it. Task 13 **must** add `PANACEA_APP_READY_TIMEOUT` (default `60s`, parsed as a Go duration in `cmd/api/config.go`) and pass it into `dockerbe.Config.ReadyTimeout`; that is recorded as a cross-task dependency in the Interfaces block below. Until it lands, `ReadyTimeout` is a field only tests set and production always gets `backend.ReadyTimeout`. **(b) `Config.ReadyTimeout` is added**, defaulting to `backend.ReadyTimeout`, because a hard-coded 60 s constant makes the ready-failure path untestable. Testability is the whole of its value inside this task; operator control over it arrives only with task 13's environment variable. **(c) `List` never fails a whole sweep over one bad container — and this is an amendment to a locked contract, so raise it before writing step 7.** Contract §2.4 says `Get`/`List` must return `backend.ErrNonLoopbackPublish` when the only binding found is off loopback. For `Get` that is right and this task keeps it exactly. For `List` it is wrong in a way that costs more than it buys: erroring the sweep blinds task 13's reaper to the very container it most needs to destroy, and one defective container then stops every *other* sandbox from ever being reaped. But the obvious repair — return the sandbox with an empty `Endpoint` and a nil error — throws away the phase's central security signal, because a LAN-published sandbox then reads to `List` exactly like a container the daemon has simply not published yet. So `List` does all of the following: keeps the container in the returned slice, so the reaper can see it; sets `Endpoint` to `""`, so nothing can attach to it; sets `State` to `backend.StateStopped` rather than `StateRunning`, so nothing can mistake it for a container that is merely slow to publish and the reaper treats it as reapable; and logs one warning carrying `backend.ErrNonLoopbackPublish`, the sandbox id and the offending host IP, so the condition is visible from a sweep. `Get` and `Attach` on that same id still return `ErrNonLoopbackPublish` loudly. **Do not implement step 7 until this amendment has been accepted against contract §2.4 lines 680–690** — it is a locked document, and a security rule dropped unilaterally inside one task is precisely how a security rule gets dropped. **(d) `attachPort` is derived from `backend.ContainerPort`** rather than written as the literal `nat.Port("8080/tcp")` the contract shows, so the two constants cannot drift.

The `go.mod` change belongs to this task, and it is **seventeen** `require` lines, not two. `github.com/docker/docker@v28.3.3+incompatible` ships no `go.mod` of its own — the cached `.mod` for that version is the synthesized one-liner `module github.com/docker/docker` — so the module graph learns none of its transitive requirements from it. `go get` on the two obvious modules adds exactly two requires and nothing else, and the next build then fails with ten `cannot find module providing package ...: module lookup disabled by GOPROXY=off` errors. Every transitive module the `client` package compiles against therefore has to be named explicitly in the same `go get`; step 1 lists all seventeen, and every one is already in this machine's module cache, so the task runs offline — but only if the versions are pinned rather than resolved.

`go mod tidy` consults the proxy for the latest version of anything under-specified and `proxy.golang.org` is TLS-intercepted from this network, so **`GOFLAGS=-mod=mod GOPROXY=off` applies to the `go get` in step 1 and to nothing else.** This repo is always in workspace mode — `go.work` at the repository root `use`s `./control-plane`, and `go env GOWORK` reports that file from inside `control-plane` too — and in workspace mode every Go command except `go get` rejects that flag outright: `go: -mod may only be set to readonly or vendor when in workspace mode, but it is set to "mod"`. So step 2's `go build`, steps 4, 6 and 8's `go test` and step 9's `go vet` and `gofmt` run with `GOPROXY=off` alone. Workspace mode is already read-only, which is the property `-mod=mod` was there to avoid needing. If some later command genuinely needs `-mod=mod`, add `GOWORK=off` on the same line and expect to run it from inside `control-plane`. Step 2 diffs `control-plane/go.mod` to prove the `go 1.23` directive did not move. `control-plane` is not vendored and must not become vendored — it is never built inside a container in this phase.

**Files:**
- `control-plane/go.mod` — pinned `require` lines for the Docker SDK; the `go 1.23` directive must not move (modified)
- `control-plane/go.sum` — the checksums `go get` writes alongside those requires; step 9 diffs it and step 11 commits it (modified)
- `control-plane/internal/backend/dockerbe/api.go` — the narrow slice of the Docker SDK this package uses, so tests need no daemon (new)
- `control-plane/internal/backend/dockerbe/docker.go` — `Backend` over Docker: create, publish to `127.0.0.1` only, label discovery, idempotent destroy (new)
- `control-plane/internal/backend/dockerbe/docker_test.go` — the fake `APIClient` and every rule above, with no daemon (new)

**Interfaces:**
- Produces:
  - `dockerbe.APIClient` — the seven Docker SDK methods this package calls; `*client.Client` satisfies it structurally
  - `dockerbe.Config` — `Client`, `Host`, `BindIP`, `AllowNonLoopback`, `ReadyTimeout`, `Log`
  - `dockerbe.Backend` and `dockerbe.New(Config) (*Backend, error)` — a `backend.Backend`; consumed by task 13's wiring in `cmd/api/main.go` and by task 13's reaper
- Consumes (all from task 4, `control-plane/internal/backend`):
  - `backend.Backend`, `backend.Spec`, `backend.Sandbox`, `backend.Mount`, `backend.Resources` with `NanoCPUs()` and `MemoryBytes()`
  - `backend.Kind`/`backend.KindDocker`, `backend.State`/`StateStarting`/`StateRunning`/`StateStopped`
  - `backend.ValidateSpec`, `(Spec).EnvEntries()`
  - `backend.ContainerPort`, `backend.LabelManaged`, `LabelSandboxID`, `LabelSessionID`, `LabelCreatedAt`
  - `backend.Dial`, `backend.WaitReady`, `backend.ReadyTimeout`, `backend.HealthPath`
  - `backend.ErrNotFound`, `ErrNotRunning`, `ErrAlreadyExists`, `ErrUnavailable`, `ErrUnsupported`, `ErrNonLoopbackPublish`
- Requires of **task 13** (cross-task dependency, not satisfiable here):
  - `PANACEA_APP_READY_TIMEOUT`, default `60s`, a Go duration parsed in `cmd/api/config.go` and passed to `dockerbe.Config.ReadyTimeout`. It is **not** in contract §5's environment table; task 13 adds it. Without it `ReadyTimeout` is test-only and production always gets `backend.ReadyTimeout`.
  - Task 13's wiring passes its `*slog.Logger` as `dockerbe.Config.Log`. Nil is safe (`slog.Default()`), but then `List`'s off-loopback warning lands wherever the default handler points rather than in the API's log stream.
- Amends: contract §2.4 lines 680–690, `List` only — see decision (c). Raise and accept before step 7.

- [ ] **Step 1: Pin the Docker SDK in `control-plane/go.mod`**

The test file cannot compile before the module requirements exist, so this one step precedes the tests. It is a dependency pin, not an implementation.

`control-plane/go.mod` today requires `k8s.io/api`, `k8s.io/apimachinery` and `k8s.io/client-go` directly and carries a long indirect block that already includes `github.com/pkg/errors v0.9.1` and `github.com/go-logr/logr v1.4.2`. Neither Docker module is present, and neither is any of the fifteen transitive modules the `client` package compiles against.

**Name every one of them in this single `go get`.** Docker's module is pre-modules-era: the `.mod` file the cache holds for `v28.3.3+incompatible` is synthesized (`module github.com/docker/docker`, one line, no requires), so adding it teaches the module graph nothing about what it needs. Adding only the two obvious modules produces exactly two `go: added` lines and a build that then fails ten times over with `module lookup disabled by GOPROXY=off`. Add all seventeen, at exactly these versions — all seventeen are in this machine's module cache:

Run, from the repository root:

```bash
cd control-plane && GOFLAGS=-mod=mod GOPROXY=off GOTOOLCHAIN=local go get \
  github.com/docker/docker@v28.3.3+incompatible \
  github.com/docker/go-connections@v0.5.0 \
  github.com/containerd/errdefs@v1.0.0 \
  github.com/containerd/errdefs/pkg@v0.3.0 \
  github.com/containerd/log@v0.2.0 \
  github.com/distribution/reference@v0.6.0 \
  github.com/docker/go-units@v0.5.0 \
  github.com/moby/docker-image-spec@v1.3.1 \
  github.com/opencontainers/go-digest@v1.0.0 \
  github.com/opencontainers/image-spec@v1.1.1 \
  github.com/felixge/httpsnoop@v1.0.4 \
  github.com/go-logr/stdr@v1.2.2 \
  go.opentelemetry.io/otel@v1.28.0 \
  go.opentelemetry.io/otel/trace@v1.28.0 \
  go.opentelemetry.io/otel/metric@v1.28.0 \
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.53.0 \
  github.com/Microsoft/go-winio@v0.6.2
```

Expected: **seventeen** lines on stderr, one `go: added <module> <version>` per module named above, in the order `go` chooses. Exit code 0. There is no eighteenth line: nothing here is pulled in transitively, because docker's synthesized `.mod` declares no requirements — every module in the list is one you named. Nothing is downloaded either; with `GOPROXY=off` the cache is the only source, so a clean exit is itself the proof that the cache was complete.

Two entries in that list are not guessable and both are load-bearing. `github.com/containerd/errdefs/pkg` is a **separate module** from `github.com/containerd/errdefs`, versioned independently — v0.3.0, not v1.0.0 — and `client` imports `errdefs/pkg/errhttp`. `github.com/Microsoft/go-winio` is required by `go-connections/sockets_windows.go`; it is **absent from contract §2.4's cached-module table**, which was written from a Linux-shaped reading of the import graph, and omitting it on Windows fails the build with `missing go.sum entry for module providing package github.com/Microsoft/go-winio`. Three modules deliberately have no line of their own — `github.com/gogo/protobuf`, `github.com/pkg/errors` and `github.com/go-logr/logr` — because the existing `k8s.io` requires already pin them.

**Failure mode.** `GOPROXY=off` plus a module missing from the cache fails with `module lookup disabled by GOPROXY=off`. That means the cache is not what §2.4 of the contract recorded; do **not** fix it by unsetting `GOPROXY`, because `proxy.golang.org` is TLS-intercepted from this network and the command will hang and then fail on certificate verification. Check with `ls $(go env GOMODCACHE)/cache/download/github.com/docker/docker/@v/` and report the versions actually present.

**Failure mode.** Naming only the two Docker modules — the shape this task had before review — exits 0 with just two `go: added` lines and looks like success. It is not: step 2's `go build` then fails with ten `cannot find module providing package ...: module lookup disabled by GOPROXY=off` errors (`containerd/errdefs`, `containerd/errdefs/pkg/errhttp`, `distribution/reference`, `docker/go-units`, `moby/docker-image-spec`, `opencontainers/image-spec`, `opencontainers/go-digest`, `gogo/protobuf`, `otel/trace`, `otelhttp`) plus the `go-winio` go.sum error above. If you see that, you truncated the list; re-run step 1 in full rather than adding modules one error at a time.

**Failure mode.** Omitting `+incompatible` — `go get github.com/docker/docker@v28.3.3` — fails with `invalid version: module contains a go.mod file, so module path must match major version`. Docker's module is pre-modules-era at v28; the `+incompatible` suffix is part of the version string, not a typo.

Do **not** run `go mod tidy`. It re-resolves under-specified requirements against the proxy, which is the one thing this network cannot do.

- [ ] **Step 2: Prove the `go` directive did not move**

Run: `git diff control-plane/go.mod`

Expected: added lines only — seventeen new `require` lines, one per module in step 1, including

```
	github.com/docker/docker v28.3.3+incompatible
	github.com/docker/go-connections v0.5.0
```

Every one of the seventeen will carry `// indirect`, and they may land in the existing indirect block rather than a new direct one. That is correct, not a mistake to fix: `go get` marks a requirement direct only when a package in the main module imports it, and `internal/backend/dockerbe` does not exist until step 5. Do **not** run `go mod tidy` to "clean up" the comments — they are annotations, nothing in the build reads them, and they become accurate on their own the first time anyone runs `tidy` with network access. The line `go 1.23` must appear in the diff **not at all**. `github.com/pkg/errors v0.9.1 // indirect` may lose or keep its comment; either is fine.

If the diff shows `-go 1.23` / `+go 1.24` — or a new `toolchain go1.24.x` line — revert both with `git checkout -- control-plane/go.mod control-plane/go.sum` and re-run step 1. The command already carries `GOTOOLCHAIN=local`, which is what prevents this; if the directive moved anyway, something other than step 1 moved it. CI pins 1.23, and a higher directive makes a network-restricted agent attempt a toolchain download and fail.

Then confirm the workspace still builds, from the repository root and **without** `-mod=mod` — the flag is illegal from here on: `GOPROXY=off go build ./control-plane/...` — expected: no output, exit code 0. (`GOFLAGS=-mod=mod go build` here exits 1 with `go: -mod may only be set to readonly or vendor when in workspace mode`; that error means you carried step 1's prefix forward, not that step 1 failed.)

- [ ] **Step 3: Write the failing tests**

The tests run with no Docker daemon. `fakeDocker` implements `dockerbe.APIClient`, and the compile-time assertion `var _ dockerbe.APIClient = (*fakeDocker)(nil)` is what proves the interface in `api.go` is the interface the tests were written against — if `api.go` drifts, this file stops compiling instead of silently testing a different shape.

Three things are worth knowing before transcribing. First, `ContainerInspect` is called several times in one `Create` (existence check, endpoint poll, final read), so the fake consumes a queue of results and repeats the last one forever. Second, the readiness wait is real HTTP: the tests stand up an `httptest` server on `127.0.0.1` and hand the fake its port as the published one, so `backend.WaitReady` genuinely polls `/healthz`. Third, "not found" is not a string: `client.IsErrNotFound` is an alias for `cerrdefs.IsNotFound`, which returns true for any error implementing `interface{ NotFound() }`, so the fake's `notFoundErr` carries a `NotFound()` method exactly as Docker's own `objectNotFoundError` does — and the test therefore needs no `containerd/errdefs` import.

Create `control-plane/internal/backend/dockerbe/docker_test.go`:

```go
package dockerbe_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/dockerbe"
)

// notFoundErr is shaped exactly like the SDK's own objectNotFoundError: a
// NotFound() method, which is what cerrdefs.IsNotFound — and therefore
// client.IsErrNotFound — actually tests for.
type notFoundErr struct{ what string }

func (e notFoundErr) Error() string { return "Error: No such container: " + e.what }
func (e notFoundErr) NotFound()     {}

type createCall struct {
	config     *container.Config
	hostConfig *container.HostConfig
	networking *network.NetworkingConfig
	platform   *ocispec.Platform
	name       string
}

type inspectResult struct {
	resp container.InspectResponse
	err  error
}

// fakeDocker is the whole Docker daemon, for these tests. Results are queued;
// the last queued inspect repeats forever, because Create inspects more than
// once.
type fakeDocker struct {
	mu sync.Mutex

	pingErr error

	creates   []createCall
	createID  string
	createErr error

	starts   []string
	startErr error

	inspects     []inspectResult
	inspectNames []string

	listOpts []container.ListOptions
	list     []container.Summary
	listErr  error

	removes    []string
	removeOpts []container.RemoveOptions
	removeErr  error

	closed bool
}

var _ dockerbe.APIClient = (*fakeDocker)(nil)

func (f *fakeDocker) ContainerCreate(_ context.Context, cfg *container.Config, host *container.HostConfig, net *network.NetworkingConfig, plat *ocispec.Platform, name string) (container.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, createCall{config: cfg, hostConfig: host, networking: net, platform: plat, name: name})
	if f.createErr != nil {
		return container.CreateResponse{}, f.createErr
	}
	return container.CreateResponse{ID: f.createID}, nil
}

func (f *fakeDocker) ContainerStart(_ context.Context, id string, _ container.StartOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, id)
	return f.startErr
}

func (f *fakeDocker) ContainerInspect(_ context.Context, name string) (container.InspectResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inspectNames = append(f.inspectNames, name)
	if len(f.inspects) == 0 {
		return container.InspectResponse{}, notFoundErr{name}
	}
	r := f.inspects[0]
	if len(f.inspects) > 1 {
		f.inspects = f.inspects[1:]
	}
	return r.resp, r.err
}

func (f *fakeDocker) ContainerList(_ context.Context, opts container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listOpts = append(f.listOpts, opts)
	return f.list, f.listErr
}

func (f *fakeDocker) ContainerRemove(_ context.Context, id string, opts container.RemoveOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removes = append(f.removes, id)
	f.removeOpts = append(f.removeOpts, opts)
	return f.removeErr
}

func (f *fakeDocker) Ping(_ context.Context) (types.Ping, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pingErr != nil {
		return types.Ping{}, f.pingErr
	}
	return types.Ping{APIVersion: "1.51"}, nil
}

func (f *fakeDocker) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeDocker) snapshotRemoves() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removes...)
}

func (f *fakeDocker) snapshotCreates() []createCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]createCall{}, f.creates...)
}

// healthzAddr starts a real loopback server answering 200 on /healthz and
// returns its host:port. backend.WaitReady polls it for real.
func healthzAddr(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(backend.HealthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// deadAddr returns a loopback host:port with nothing listening on it.
func deadAddr(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NewServeMux())
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	return addr
}

func portOf(t *testing.T, addr string) string {
	t.Helper()
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		t.Fatalf("addr %q has no port", addr)
	}
	return addr[i+1:]
}

func labelsFor(sandboxID, sessionID string) map[string]string {
	return map[string]string{
		backend.LabelManaged:   "true",
		backend.LabelSandboxID: sandboxID,
		backend.LabelSessionID: sessionID,
		backend.LabelCreatedAt: "2026-09-18T01:02:03Z",
	}
}

// inspectOf builds an InspectResponse. hostPort "" means "no binding yet".
func inspectOf(ref, status, hostIP, hostPort string, labels map[string]string) container.InspectResponse {
	ports := nat.PortMap{}
	if hostPort != "" {
		ports[nat.Port("8080/tcp")] = []nat.PortBinding{{HostIP: hostIP, HostPort: hostPort}}
	}
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:      ref,
			Created: "2026-09-18T01:02:03.400000000Z",
			State:   &container.State{Status: status, Running: status == "running"},
		},
		Config:          &container.Config{Labels: labels},
		NetworkSettings: &container.NetworkSettings{NetworkSettingsBase: container.NetworkSettingsBase{Ports: ports}},
	}
}

func validSpec() backend.Spec {
	return backend.Spec{
		ID:          "s-3f2a91c0",
		SessionID:   "sess-1",
		Image:       "panacea-app/sandbox:dev",
		Command:     "claude",
		Args:        []string{"--model", "opus", "-p", "refactor auth, then test"},
		AttachToken: "tok-abc",
		Resources:   backend.Resources{CPUMillis: 2000, MemoryMiB: 4096},
		WorkDir:     "/work",
		Mounts:      []backend.Mount{{HostPath: "C:/repo", ContainerPath: "/work"}},
	}
}

func newBackend(t *testing.T, f *fakeDocker) *dockerbe.Backend {
	t.Helper()
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

// newBackendWithLog is newBackend with a capturable logger. List's refusal to
// error a whole sweep means the log line is the only place an off-loopback
// publish surfaces from a sweep, so that line is behaviour and is asserted,
// not decoration.
func newBackendWithLog(t *testing.T, f *fakeDocker) (*dockerbe.Backend, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: 5 * time.Second, Log: logger})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b, &buf
}

// A Docker Desktop that is installed but not running must fail here, with a
// sentence the operator can act on, not later with a nil dereference.
func TestNewIsErrUnavailableWhenNoDaemonAnswers(t *testing.T) {
	f := &fakeDocker{pingErr: errors.New("cannot connect to the Docker daemon")}

	_, err := dockerbe.New(dockerbe.Config{Client: f})
	if !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("New with a dead daemon = %v, want an error wrapping ErrUnavailable", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "docker desktop") {
		t.Errorf("New error %q does not tell the operator what to start", err)
	}
}

// Publishing to anything but loopback puts a shell on the LAN. It has to be a
// decision someone makes on purpose, not a default anyone can trip over.
func TestNewRefusesANonLoopbackBindIP(t *testing.T) {
	_, err := dockerbe.New(dockerbe.Config{Client: &fakeDocker{}, BindIP: "0.0.0.0"})
	if !errors.Is(err, backend.ErrNonLoopbackPublish) {
		t.Fatalf("New(BindIP 0.0.0.0) = %v, want an error wrapping ErrNonLoopbackPublish", err)
	}
}

func TestNewAllowsANonLoopbackBindIPOnlyWhenSaidOnPurpose(t *testing.T) {
	b, err := dockerbe.New(dockerbe.Config{Client: &fakeDocker{}, BindIP: "0.0.0.0", AllowNonLoopback: true})
	if err != nil {
		t.Fatalf("New(BindIP 0.0.0.0, AllowNonLoopback) = %v, want nil", err)
	}
	if b.Kind() != backend.KindDocker {
		t.Errorf("Kind() = %q, want %q", b.Kind(), backend.KindDocker)
	}
}

func TestCreateBuildsTheContractedContainerAndReturnsALoopbackEndpoint(t *testing.T) {
	addr := healthzAddr(t)
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{err: notFoundErr{"panacea-app-s-3f2a91c0"}},
		{resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", portOf(t, addr), labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	sb, err := b.Create(context.Background(), validSpec())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	creates := f.snapshotCreates()
	if len(creates) != 1 {
		t.Fatalf("ContainerCreate called %d times, want 1", len(creates))
	}
	c := creates[0]
	if c.name != "panacea-app-s-3f2a91c0" {
		t.Errorf("container name = %q, want %q", c.name, "panacea-app-s-3f2a91c0")
	}
	if c.networking != nil || c.platform != nil {
		t.Errorf("networkingConfig=%v platform=%v, want both nil (default bridge, image platform)", c.networking, c.platform)
	}
	if c.config.Tty || c.config.OpenStdin {
		t.Errorf("Tty=%v OpenStdin=%v, want false/false — sandboxd owns the PTY, not Docker", c.config.Tty, c.config.OpenStdin)
	}
	if c.config.WorkingDir != "/work" || c.config.Image != "panacea-app/sandbox:dev" {
		t.Errorf("WorkingDir=%q Image=%q", c.config.WorkingDir, c.config.Image)
	}
	if _, ok := c.config.ExposedPorts[nat.Port("8080/tcp")]; !ok {
		t.Errorf("ExposedPorts = %v, want 8080/tcp", c.config.ExposedPorts)
	}

	wantEnv := []string{
		"PANACEA_APP_COMMAND=claude",
		`PANACEA_APP_ARGS_JSON=["--model","opus","-p","refactor auth, then test"]`,
		"PANACEA_APP_ATTACH_TOKEN=tok-abc",
	}
	if !reflect.DeepEqual(c.config.Env, wantEnv) {
		t.Errorf("Env =\n%q\nwant\n%q", c.config.Env, wantEnv)
	}
	if got := c.config.Labels[backend.LabelManaged]; got != "true" {
		t.Errorf("label %s = %q, want %q — List discovers on this label alone", backend.LabelManaged, got, "true")
	}
	if got := c.config.Labels[backend.LabelSandboxID]; got != "s-3f2a91c0" {
		t.Errorf("label %s = %q, want %q", backend.LabelSandboxID, got, "s-3f2a91c0")
	}
	if got := c.config.Labels[backend.LabelSessionID]; got != "sess-1" {
		t.Errorf("label %s = %q, want %q", backend.LabelSessionID, got, "sess-1")
	}
	if _, err := time.Parse(time.RFC3339, c.config.Labels[backend.LabelCreatedAt]); err != nil {
		t.Errorf("label %s = %q, want RFC3339: %v", backend.LabelCreatedAt, c.config.Labels[backend.LabelCreatedAt], err)
	}
	if len(c.config.Labels) != 4 {
		t.Errorf("labels = %v, want exactly the four contract labels", c.config.Labels)
	}
	for _, v := range c.config.Labels {
		if v == "tok-abc" {
			t.Fatalf("the attach token is a label: docker inspect ends up in bug reports")
		}
	}

	bindings := c.hostConfig.PortBindings[nat.Port("8080/tcp")]
	want := []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: ""}}
	if !reflect.DeepEqual(bindings, want) {
		t.Errorf("PortBindings = %#v, want %#v — an empty HostIP publishes on every interface", bindings, want)
	}
	if c.hostConfig.AutoRemove {
		t.Errorf("AutoRemove = true: a self-removing container deletes the transcript claude --resume reads")
	}
	if c.hostConfig.RestartPolicy.Name != container.RestartPolicyDisabled {
		t.Errorf("RestartPolicy = %q, want %q", c.hostConfig.RestartPolicy.Name, container.RestartPolicyDisabled)
	}
	if c.hostConfig.Resources.NanoCPUs != 2_000_000_000 || c.hostConfig.Resources.Memory != 4096<<20 {
		t.Errorf("Resources = %d nanocpus / %d bytes", c.hostConfig.Resources.NanoCPUs, c.hostConfig.Resources.Memory)
	}
	if !reflect.DeepEqual(c.hostConfig.Binds, []string{"C:/repo:/work"}) {
		t.Errorf("Binds = %q, want %q", c.hostConfig.Binds, []string{"C:/repo:/work"})
	}

	if sb.Kind != backend.KindDocker || sb.Ref != "c0ffeecafe" || sb.ID != "s-3f2a91c0" {
		t.Errorf("Sandbox = %+v", sb)
	}
	if sb.Endpoint != "127.0.0.1:"+portOf(t, addr) {
		t.Errorf("Endpoint = %q, want 127.0.0.1:%s", sb.Endpoint, portOf(t, addr))
	}
	if sb.State != backend.StateRunning {
		t.Errorf("State = %q, want %q", sb.State, backend.StateRunning)
	}
}

// A caller that could set PANACEA_APP_ATTACH_TOKEN could replace the sandbox's
// secret. ValidateSpec is the first statement of Create in both backends.
func TestCreateRejectsAReservedEnvKey(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe"}
	b := newBackend(t, f)
	spec := validSpec()
	spec.Env = map[string]string{"PANACEA_APP_ATTACH_TOKEN": "mine"}

	_, err := b.Create(context.Background(), spec)
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("Create with a reserved env key = %v, want an error wrapping ErrUnsupported", err)
	}
	if len(f.snapshotCreates()) != 0 {
		t.Errorf("ContainerCreate was called despite an invalid spec")
	}
}

func TestCreateIsErrAlreadyExistsWhenTheNameIsTaken(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{resp: inspectOf("older", "running", "127.0.0.1", "51000", labelsFor("s-3f2a91c0", "sess-0"))},
	}
	b := newBackend(t, f)

	_, err := b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrAlreadyExists) {
		t.Fatalf("Create over an existing name = %v, want an error wrapping ErrAlreadyExists", err)
	}
	if len(f.snapshotCreates()) != 0 {
		t.Errorf("ContainerCreate was called over an existing container")
	}
}

// THE security test. A sandbox published on 0.0.0.0 is an unauthenticated shell
// on the corporate LAN. Create must not hand the caller an endpoint for it, and
// must not leave it running.
func TestCreateDestroysTheSandboxWhenDockerPublishesOffLoopback(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{err: notFoundErr{"panacea-app-s-3f2a91c0"}},
		{resp: inspectOf("c0ffeecafe", "running", "0.0.0.0", "51000", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	_, err := b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrNonLoopbackPublish) {
		t.Fatalf("Create with a 0.0.0.0 binding = %v, want an error wrapping ErrNonLoopbackPublish", err)
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: a LAN-published sandbox must not survive Create", got)
	}
}

func TestCreateDestroysTheSandboxThatNeverAnswersHealthz(t *testing.T) {
	dead := deadAddr(t)
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{err: notFoundErr{"panacea-app-s-3f2a91c0"}},
		{resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", portOf(t, dead), labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: 400 * time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Create of a sandbox that never serves /healthz = %v, want an error wrapping ErrNotRunning", err)
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: Create is all-or-nothing", got)
	}
}

// ReadyTimeout is one budget across both of Create's waits, not one per wait.
// A daemon that publishes the port slowly and a sandboxd that never answers
// /healthz must together cost ReadyTimeout, because an operator who set it and
// then waited twice it cannot tell which of the two went wrong.
//
// Nine unpublished inspects hold the port back for about nine endpoint polls,
// spending most of the budget before /healthz is even dialled. The bound is
// deliberately loose and one-sided: a fast machine can never fail it, and the
// two-budget bug overshoots it by enough that scheduling noise cannot hide it.
func TestCreateSpendsOneReadyBudgetAcrossBothWaits(t *testing.T) {
	const budget = time.Second
	dead := deadAddr(t)
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{{err: notFoundErr{"panacea-app-s-3f2a91c0"}}}
	unpublished := inspectOf("c0ffeecafe", "running", "", "", labelsFor("s-3f2a91c0", "sess-1"))
	for i := 0; i < 9; i++ {
		f.inspects = append(f.inspects, inspectResult{resp: unpublished})
	}
	f.inspects = append(f.inspects, inspectResult{
		resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", portOf(t, dead), labelsFor("s-3f2a91c0", "sess-1")),
	})
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: budget})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	start := time.Now()
	_, err = b.Create(context.Background(), validSpec())
	elapsed := time.Since(start)

	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Create of a slow-to-publish sandbox that never serves /healthz = %v, want an error wrapping ErrNotRunning", err)
	}
	if elapsed > budget+budget/2 {
		t.Fatalf("Create took %v with ReadyTimeout %v: the port wait and the /healthz wait must share one deadline, not take one each", elapsed, budget)
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: an expired ready budget still destroys the container", got)
	}
}

func TestCreateRemovesTheContainerWhenStartFails(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe", startErr: errors.New("driver failed programming external connectivity")}
	f.inspects = []inspectResult{{err: notFoundErr{"panacea-app-s-3f2a91c0"}}}
	b := newBackend(t, f)

	_, err := b.Create(context.Background(), validSpec())
	if err == nil {
		t.Fatalf("Create with a failing start = nil, want an error")
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: a created-but-unstarted container is a leak", got)
	}
}

func TestGetMapsInspectOntoASandbox(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", "51000", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	sb, err := b.Get(context.Background(), "s-3f2a91c0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sb.ID != "s-3f2a91c0" || sb.Ref != "c0ffeecafe" || sb.Endpoint != "127.0.0.1:51000" {
		t.Errorf("Sandbox = %+v", sb)
	}
	if sb.State != backend.StateRunning {
		t.Errorf("State = %q, want %q", sb.State, backend.StateRunning)
	}
	if !sb.CreatedAt.Equal(time.Date(2026, 9, 18, 1, 2, 3, 400000000, time.UTC)) {
		t.Errorf("CreatedAt = %s, want 2026-09-18T01:02:03.4Z", sb.CreatedAt)
	}
	if f.inspectNames[0] != "panacea-app-s-3f2a91c0" {
		t.Errorf("inspected %q, want the prefixed container name", f.inspectNames[0])
	}
}

func TestGetIsErrNotFoundForAnUnknownSandbox(t *testing.T) {
	b := newBackend(t, &fakeDocker{})

	_, err := b.Get(context.Background(), "s-nope")
	if !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("Get of an unknown sandbox = %v, want an error wrapping ErrNotFound", err)
	}
}

func TestGetRefusesToHandBackAnOffLoopbackEndpoint(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "running", "0.0.0.0", "51000", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	_, err := b.Get(context.Background(), "s-3f2a91c0")
	if !errors.Is(err, backend.ErrNonLoopbackPublish) {
		t.Fatalf("Get of a 0.0.0.0-published sandbox = %v, want an error wrapping ErrNonLoopbackPublish", err)
	}
}

// The daemon publishes the port after the container starts. A running sandbox
// with no binding yet is starting, not broken.
func TestGetOfAnUnpublishedContainerIsStartingWithNoEndpoint(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "created", "", "", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	sb, err := b.Get(context.Background(), "s-3f2a91c0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sb.State != backend.StateStarting || sb.Endpoint != "" {
		t.Fatalf("Sandbox = %+v, want starting with an empty endpoint", sb)
	}
}

func TestListDiscoversOnTheManagedLabelAndSkipsForeignContainers(t *testing.T) {
	f := &fakeDocker{list: []container.Summary{
		{
			ID:      "c1",
			Created: 1758157323,
			State:   "running",
			Labels:  labelsFor("s-aaaaaaaa", "sess-1"),
			Ports:   []container.Port{{IP: "127.0.0.1", PrivatePort: 8080, PublicPort: 51001, Type: "tcp"}},
		},
		{
			ID:     "c2",
			State:  "running",
			Labels: map[string]string{backend.LabelManaged: "true"}, // no sandbox id
		},
	}}
	b := newBackend(t, f)

	got, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List returned %d sandboxes, want 1 — a container with no %s label is not ours to manage", len(got), backend.LabelSandboxID)
	}
	if got[0].ID != "s-aaaaaaaa" || got[0].Endpoint != "127.0.0.1:51001" || got[0].Ref != "c1" {
		t.Errorf("Sandbox = %+v", got[0])
	}
	if !got[0].CreatedAt.Equal(time.Unix(1758157323, 0).UTC()) {
		t.Errorf("CreatedAt = %s, want the Unix Created field in UTC", got[0].CreatedAt)
	}
	if len(f.listOpts) != 1 || !f.listOpts[0].All {
		t.Fatalf("ListOptions = %+v, want All:true — the reaper must see exited containers", f.listOpts)
	}
	if want := []string{backend.LabelManaged + "=true"}; !reflect.DeepEqual(f.listOpts[0].Filters.Get("label"), want) {
		t.Errorf("label filter = %v, want %v", f.listOpts[0].Filters.Get("label"), want)
	}
}

func TestListMapsEveryDockerStateOntoABackendState(t *testing.T) {
	cases := map[string]backend.State{
		"running":    backend.StateRunning,
		"created":    backend.StateStarting,
		"restarting": backend.StateStarting,
		"exited":     backend.StateStopped,
		"paused":     backend.StateStopped,
		"dead":       backend.StateStopped,
		"removing":   backend.StateStopped,
	}
	for dockerState, want := range cases {
		f := &fakeDocker{list: []container.Summary{
			{ID: "c1", State: dockerState, Labels: labelsFor("s-aaaaaaaa", "sess-1")},
		}}
		b := newBackend(t, f)

		got, err := b.List(context.Background())
		if err != nil {
			t.Fatalf("%s: List: %v", dockerState, err)
		}
		if len(got) != 1 || got[0].State != want {
			t.Errorf("docker state %q mapped to %v, want %q", dockerState, got, want)
		}
	}
}

// List must not fail a whole sweep over one bad container: the reaper needs to
// see it in order to destroy it, and Get is where the refusal is loud. But it
// must not come back looking normal either. An off-loopback publish is the
// defect this phase exists to prevent, and a sandbox returned as running with
// an empty Endpoint is indistinguishable from one the daemon has not published
// yet — so it comes back StateStopped, with no endpoint, and with a warning.
func TestListMarksAnOffLoopbackBindingStoppedWithNoEndpoint(t *testing.T) {
	f := &fakeDocker{list: []container.Summary{
		{
			ID:     "c1",
			State:  "running",
			Labels: labelsFor("s-aaaaaaaa", "sess-1"),
			Ports:  []container.Port{{IP: "0.0.0.0", PrivatePort: 8080, PublicPort: 51001, Type: "tcp"}},
		},
	}}
	b, logBuf := newBackendWithLog(t, f)

	got, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List = %v, want nil: one bad container must not blind the reaper to the rest", err)
	}
	if len(got) != 1 {
		t.Fatalf("List = %+v, want exactly one sandbox", got)
	}
	if got[0].Endpoint != "" {
		t.Errorf("Endpoint = %q, want empty: nothing may attach to an off-loopback publish", got[0].Endpoint)
	}
	if got[0].State != backend.StateStopped {
		t.Errorf("State = %v, want %v: an off-loopback publish must not read as a container that is merely slow to publish", got[0].State, backend.StateStopped)
	}
	if line := logBuf.String(); !strings.Contains(line, "s-aaaaaaaa") || !strings.Contains(line, "loopback") {
		t.Errorf("log = %q, want one warning naming the sandbox and the off-loopback publish", line)
	}
}

// Task 13's reaper and an explicit DELETE race by design. A reaper that errors
// on "already gone" stops sweeping.
func TestDestroyOfAnAlreadyGoneSandboxIsSuccess(t *testing.T) {
	f := &fakeDocker{removeErr: notFoundErr{"panacea-app-s-3f2a91c0"}}
	b := newBackend(t, f)

	if err := b.Destroy(context.Background(), "s-3f2a91c0"); err != nil {
		t.Fatalf("Destroy of a gone sandbox = %v, want nil", err)
	}
}

func TestDestroyIsIdempotentAcrossRepeatedCalls(t *testing.T) {
	f := &fakeDocker{}
	b := newBackend(t, f)

	for i := 0; i < 3; i++ {
		if err := b.Destroy(context.Background(), "s-3f2a91c0"); err != nil {
			t.Fatalf("Destroy call %d = %v, want nil", i+1, err)
		}
	}
	if got := f.snapshotRemoves(); len(got) != 3 {
		t.Fatalf("removes = %v, want three calls", got)
	}
	if !f.removeOpts[0].Force || !f.removeOpts[0].RemoveVolumes {
		t.Errorf("RemoveOptions = %+v, want Force and RemoveVolumes", f.removeOpts[0])
	}
}

func TestDestroySurfacesARealDaemonFailure(t *testing.T) {
	f := &fakeDocker{removeErr: errors.New("connection refused")}
	b := newBackend(t, f)

	err := b.Destroy(context.Background(), "s-3f2a91c0")
	if !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("Destroy with a dead daemon = %v, want an error wrapping ErrUnavailable", err)
	}
}

func TestAttachIsErrNotRunningWhenTheSandboxHasNoEndpoint(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "exited", "", "", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	_, err := b.Attach(context.Background(), "s-3f2a91c0", "tok-abc")
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Attach to an exited sandbox = %v, want an error wrapping ErrNotRunning", err)
	}
}

func TestAttachOfAnUnknownSandboxIsErrNotFound(t *testing.T) {
	b := newBackend(t, &fakeDocker{})

	_, err := b.Attach(context.Background(), "s-nope", "tok-abc")
	if !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("Attach to an unknown sandbox = %v, want an error wrapping ErrNotFound", err)
	}
}

func TestCloseClosesTheDockerClient(t *testing.T) {
	f := &fakeDocker{}
	b := newBackend(t, f)

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !f.closed {
		t.Errorf("Close did not close the Docker client; a leaked npipe handle survives every restart")
	}
}

func TestBackendSatisfiesTheBackendInterface(t *testing.T) {
	var _ backend.Backend = (*dockerbe.Backend)(nil)
}
```

- [ ] **Step 4: Run the tests and watch them fail**

Run, from the repository root: `GOPROXY=off go test ./control-plane/internal/backend/dockerbe/...`

Expected: FAIL to build, with `no Go files in .../internal/backend/dockerbe` — the directory holds only `docker_test.go` and its package clause is `dockerbe_test`, so the package under test has no source at all. (If you have not created the directory yet, `matched no packages` instead.) Either is the right failure: nothing in the package exists. What must **not** appear is any error about resolving an import — if step 1 was run in full, every import in the test file resolves from the module cache.

**Failure mode.** `cannot find module providing package github.com/containerd/errdefs/pkg/errhttp: module lookup disabled by GOPROXY=off` — or the same error naming `containerd/errdefs`, `distribution/reference`, `docker/go-units`, `moby/docker-image-spec`, `opencontainers/image-spec`, `opencontainers/go-digest`, `gogo/protobuf`, `go.opentelemetry.io/otel/trace` or `otelhttp` — means step 1 was run with a truncated module list. It is not "step 1 did not land": the two Docker requires are there, and that is exactly the trap, because `go get` exited 0. Re-run step 1 as written and confirm `git diff control-plane/go.mod` shows seventeen new requires. `missing go.sum entry for module providing package github.com/Microsoft/go-winio` is the same cause, surfacing differently because Windows-only files are resolved later.

**Failure mode.** `go: -mod may only be set to readonly or vendor when in workspace mode, but it is set to "mod"` means you carried step 1's `GOFLAGS=-mod=mod` forward. Drop it. From step 2 onward every command in this task uses `GOPROXY=off` alone.

- [ ] **Step 5: Write the SDK slice**

Create `control-plane/internal/backend/dockerbe/api.go`:

```go
// Package dockerbe runs sandboxes as Docker containers. This is not
// scaffolding: docs/PROGRAM.md ships a "local machine" alongside cloud
// machines, and this package is that feature. katabe is the dormant one.
package dockerbe

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// APIClient is the slice of the Docker SDK this package uses. *client.Client
// satisfies it structurally, so production passes the real client and tests
// pass a fake — there is no Docker daemon on a CI agent.
//
// Signatures copied verbatim from
// github.com/docker/docker@v28.3.3+incompatible/client/client_interfaces.go,
// lines 72–103. They must not be "tidied": v28 renamed types.Container to
// container.Summary and types.ContainerJSON to container.InspectResponse, and a
// signature written from memory will not compile. docker.go asserts
// *client.Client still satisfies this, so an SDK bump breaks the build here
// rather than at run time.
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

- [ ] **Step 6: Run the tests and watch the failure move**

Run, from the repository root: `GOPROXY=off go test ./control-plane/internal/backend/dockerbe/...`

Expected: still FAIL to build, but now with `undefined: dockerbe.New`, `undefined: dockerbe.Config` and `undefined: dockerbe.Backend`. The point of this intermediate run is the *absence* of two other things. No error names `APIClient`, which proves `var _ dockerbe.APIClient = (*fakeDocker)(nil)` compiled and the fake matches the SDK slice exactly. And no error names a module, which proves step 1's seventeen requires cover `api.go`'s imports — `api.go` is the first non-test file to import the Docker SDK, so this is where a truncated step 1 surfaces if the test file somehow dodged it.

**Failure mode.** `cannot use (*fakeDocker)(nil) ... does not implement dockerbe.APIClient (wrong type for method ContainerInspect)` means a signature was retyped rather than copied — almost always `types.ContainerJSON` instead of `container.InspectResponse`, or a `container` parameter renamed to `containerID` *and* retyped. Copy the seven lines again from the block above.

- [ ] **Step 7: Write the Docker backend**

Create `control-plane/internal/backend/dockerbe/docker.go`:

```go
package dockerbe

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

const (
	// namePrefix makes `docker ps` legible during an incident, and gives
	// Create a cheap existence check: one inspect by name, not a list.
	namePrefix = "panacea-app-"

	// loopbackIPv4 is the only interface a sandbox port may be published to.
	loopbackIPv4 = "127.0.0.1"

	pingTimeout          = 5 * time.Second
	endpointPollInterval = 100 * time.Millisecond
	cleanupTimeout       = 20 * time.Second
)

// attachPort is derived from backend.ContainerPort rather than written as the
// literal "8080/tcp", so the container port and the published port cannot drift
// apart. nat.Port is a string type, so this is a plain conversion.
var attachPort = nat.Port(strconv.Itoa(backend.ContainerPort) + "/tcp")

// The real SDK client must keep satisfying the slice in api.go. An SDK bump
// that renames a type breaks the build here, not at run time.
var _ APIClient = (*client.Client)(nil)

var _ backend.Backend = (*Backend)(nil)

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

	// ReadyTimeout is one budget spanning both halves of Create's wait: for
	// the daemon to publish a host port, and for sandboxd to answer /healthz.
	// Whatever the first half spends, the second half does not get, so Create
	// returns within ReadyTimeout and not within twice it. Zero means
	// backend.ReadyTimeout. It is a field rather than a constant so the
	// ready-failure path is testable in under a second. Nothing sets it from
	// the environment yet: PANACEA_APP_READY_TIMEOUT does not exist, and task
	// 13 is where it has to be added if an operator is ever to raise it.
	ReadyTimeout time.Duration

	// Log receives the one warning List emits for a container published off
	// loopback. Nil means slog.Default(). This is not decoration: List cannot
	// return ErrNonLoopbackPublish without blinding the reaper to the very
	// container that needs destroying, so this line is the only place the
	// condition surfaces during a sweep. Get and Attach still refuse loudly.
	Log *slog.Logger
}

// Backend implements backend.Backend over Docker.
type Backend struct {
	api          APIClient
	bindIP       string
	readyTimeout time.Duration
	log          *slog.Logger
}

// New returns a Docker backend, or fails with backend.ErrUnavailable and a
// sentence the operator can act on. A Docker Desktop that is installed but not
// running must fail here, not later with a nil dereference.
func New(cfg Config) (*Backend, error) {
	bindIP := cfg.BindIP
	if bindIP == "" {
		bindIP = loopbackIPv4
	}
	if bindIP != loopbackIPv4 && !cfg.AllowNonLoopback {
		return nil, fmt.Errorf("dockerbe: BindIP %q would publish sandbox ports off loopback; set AllowNonLoopback to mean it: %w", bindIP, backend.ErrNonLoopbackPublish)
	}

	readyTimeout := cfg.ReadyTimeout
	if readyTimeout <= 0 {
		readyTimeout = backend.ReadyTimeout
	}

	logger := cfg.Log
	if logger == nil {
		logger = slog.Default()
	}

	api := cfg.Client
	built := false
	if api == nil {
		opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
		if cfg.Host != "" {
			opts = append(opts, client.WithHost(cfg.Host))
		}
		c, err := client.NewClientWithOpts(opts...)
		if err != nil {
			return nil, fmt.Errorf("dockerbe: cannot build a Docker client (check DOCKER_HOST): %v: %w", err, backend.ErrUnavailable)
		}
		api, built = c, true
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if _, err := api.Ping(pingCtx); err != nil {
		if built {
			_ = api.Close() // close only what we built; never the caller's client
		}
		return nil, fmt.Errorf("dockerbe: no Docker daemon answered — start Docker Desktop, or set PANACEA_APP_DOCKER_HOST: %v: %w", err, backend.ErrUnavailable)
	}
	return &Backend{api: api, bindIP: bindIP, readyTimeout: readyTimeout, log: logger}, nil
}

func (b *Backend) Kind() backend.Kind { return backend.KindDocker }

func (b *Backend) Close() error { return b.api.Close() }

// Create is all-or-nothing: every path that fails after ContainerCreate removes
// the container before returning. A half-created sandbox has no session record,
// so nothing but the reaper would ever find it again.
func (b *Backend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	if err := backend.ValidateSpec(spec); err != nil {
		return backend.Sandbox{}, err
	}
	name := namePrefix + spec.ID

	// A name collision is how ErrAlreadyExists is detected: inspect the name
	// first rather than decoding the daemon's 409.
	if _, err := b.api.ContainerInspect(ctx, name); err == nil {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: container %s: %w", name, backend.ErrAlreadyExists)
	} else if !client.IsErrNotFound(err) {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: inspect %s: %v: %w", name, err, backend.ErrUnavailable)
	}

	env := make([]string, 0, 3+len(spec.Env))
	for _, e := range spec.EnvEntries() {
		env = append(env, e.Name+"="+e.Value)
	}
	binds := make([]string, 0, len(spec.Mounts))
	for _, m := range spec.Mounts {
		bind := m.HostPath + ":" + m.ContainerPath
		if m.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	cfg := &container.Config{
		Image:        spec.Image,
		Labels:       containerLabels(spec),
		Env:          env,
		ExposedPorts: nat.PortSet{attachPort: struct{}{}},
		WorkingDir:   spec.WorkDir,
		Tty:          false, // sandboxd owns the PTY, not Docker
		OpenStdin:    false,
	}
	hostCfg := &container.HostConfig{
		// HostPort "" asks the daemon for an ephemeral port. HostIP is the
		// whole security property of this package.
		PortBindings:  nat.PortMap{attachPort: {{HostIP: b.bindIP, HostPort: ""}}},
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
		// AutoRemove false is load-bearing: destroy happens on explicit kill,
		// idle TTL or reap, never on disconnect, and a self-removing container
		// would delete the transcript `claude --resume` reads.
		AutoRemove: false,
		Binds:      binds,
		Resources: container.Resources{
			NanoCPUs: spec.Resources.NanoCPUs(),
			Memory:   spec.Resources.MemoryBytes(),
		},
	}

	// networkingConfig nil and NetworkMode unset means the default bridge.
	// Host networking is deliberately not used: Kata cannot support it and the
	// two backends must not diverge in what a sandbox can reach. platform nil
	// lets Docker Desktop pick the image's platform.
	created, err := b.api.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
	if err != nil {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: create %s: %v: %w", name, err, backend.ErrUnavailable)
	}
	if err := b.api.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, fmt.Errorf("dockerbe: start %s: %v: %w", name, err, backend.ErrUnavailable)
	}

	// One deadline for both waits. Computing it here rather than inside each
	// wait is what keeps Create's worst case at readyTimeout instead of twice
	// it.
	deadline := time.Now().Add(b.readyTimeout)

	endpoint, err := b.waitForEndpoint(ctx, name, deadline)
	if err != nil {
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, err
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		// The port arrived with the budget already spent. Calling WaitReady
		// with a non-positive timeout would poll zero times and report a last
		// error of <nil>; say what actually happened instead.
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, fmt.Errorf("dockerbe: sandbox %s published %s with no time left in the %s ready budget to answer %s: %w", spec.ID, endpoint, b.readyTimeout, backend.HealthPath, backend.ErrNotRunning)
	}
	if err := backend.WaitReady(ctx, endpoint, remaining); err != nil {
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, fmt.Errorf("dockerbe: sandbox %s never answered %s on %s: %w", spec.ID, backend.HealthPath, endpoint, err)
	}
	return b.Get(ctx, spec.ID)
}

// containerLabels is every label this package writes. The attach token is
// deliberately absent: a label is readable by anything that can reach the
// Docker socket and is echoed into `docker inspect` output that ends up in bug
// reports.
func containerLabels(spec backend.Spec) map[string]string {
	return map[string]string{
		backend.LabelManaged:   "true",
		backend.LabelSandboxID: spec.ID,
		backend.LabelSessionID: spec.SessionID,
		backend.LabelCreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// cleanup removes a container on a failed Create. It uses a context detached
// from the caller's, because the most common reason Create fails is that the
// caller's context expired, and that is exactly when the container must still
// be removed.
func (b *Backend) cleanup(ctx context.Context, ref string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	_ = b.api.ContainerRemove(cleanupCtx, ref, container.RemoveOptions{Force: true, RemoveVolumes: true})
}

// waitForEndpoint polls inspect until the daemon reports a host binding. The
// daemon publishes the port after the container starts, so a freshly started
// container legitimately has none for a few hundred milliseconds. deadline is
// Create's, shared with the /healthz wait that follows, so this function takes
// it rather than deriving one of its own.
func (b *Backend) waitForEndpoint(ctx context.Context, name string, deadline time.Time) (string, error) {
	for {
		resp, err := b.api.ContainerInspect(ctx, name)
		if err != nil {
			if client.IsErrNotFound(err) {
				return "", fmt.Errorf("dockerbe: container %s vanished while starting: %w", name, backend.ErrNotRunning)
			}
			return "", fmt.Errorf("dockerbe: inspect %s: %v: %w", name, err, backend.ErrUnavailable)
		}
		endpoint, err := b.endpointFromPortMap(portsOf(resp))
		if err != nil {
			return "", err
		}
		if endpoint != "" {
			return endpoint, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("dockerbe: container %s published no %s binding on %s: %w", name, attachPort, b.bindIP, backend.ErrNotRunning)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(endpointPollInterval):
		}
	}
}

func (b *Backend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	name := namePrefix + id
	resp, err := b.api.ContainerInspect(ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return backend.Sandbox{}, fmt.Errorf("dockerbe: sandbox %s: %w", id, backend.ErrNotFound)
		}
		return backend.Sandbox{}, fmt.Errorf("dockerbe: inspect %s: %v: %w", name, err, backend.ErrUnavailable)
	}
	if resp.ContainerJSONBase == nil {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: inspect %s returned no container body: %w", name, backend.ErrUnavailable)
	}
	endpoint, err := b.endpointFromPortMap(portsOf(resp))
	if err != nil {
		return backend.Sandbox{}, err
	}
	labels := map[string]string{}
	if resp.Config != nil && resp.Config.Labels != nil {
		labels = resp.Config.Labels
	}
	status := ""
	if resp.State != nil {
		status = resp.State.Status
	}
	return backend.Sandbox{
		ID:        id,
		Kind:      backend.KindDocker,
		Ref:       resp.ID,
		Endpoint:  endpoint,
		State:     stateFrom(status),
		CreatedAt: parseCreated(resp.Created, labels),
		Labels:    labels,
	}, nil
}

// List discovers by label, not by name pattern, and never fails the whole sweep
// over one bad container: task 13's reaper has to see a container in order to
// destroy it.
//
// This is a deliberate amendment to contract §2.4, which says Get/List must
// return backend.ErrNonLoopbackPublish. Get does. List must not, because one
// defective container would then stop every other sandbox from being reaped.
// What List does instead is make the container unmistakable rather than
// invisible: empty Endpoint so nothing can attach, StateStopped so it cannot be
// read as a container the daemon is merely slow to publish, and one warning
// naming the offending bind address. Get and Attach on the same id still refuse
// loudly with ErrNonLoopbackPublish.
func (b *Backend) List(ctx context.Context) ([]backend.Sandbox, error) {
	opts := container.ListOptions{
		All:     true, // include exited containers; the reaper needs to see them
		Filters: filters.NewArgs(filters.Arg("label", backend.LabelManaged+"=true")),
	}
	summaries, err := b.api.ContainerList(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("dockerbe: list: %v: %w", err, backend.ErrUnavailable)
	}
	out := make([]backend.Sandbox, 0, len(summaries))
	for _, s := range summaries {
		id := s.Labels[backend.LabelSandboxID]
		if id == "" {
			continue // managed-looking but not ours to manage
		}
		endpoint, epErr := b.endpointFromSummaryPorts(s.Ports)
		state := stateFrom(s.State)
		if epErr != nil {
			// Not returned: erroring the sweep blinds the reaper. Not hidden
			// either: StateStopped plus no endpoint means nothing attaches to
			// it and nothing mistakes it for a normal starting container.
			endpoint, state = "", backend.StateStopped
			b.log.Warn("sandbox port is published off loopback; refusing to expose it",
				"sandbox", id, "container", s.ID, "err", epErr)
		}
		out = append(out, backend.Sandbox{
			ID:        id,
			Kind:      backend.KindDocker,
			Ref:       s.ID,
			Endpoint:  endpoint,
			State:     state,
			CreatedAt: time.Unix(s.Created, 0).UTC(),
			Labels:    s.Labels,
		})
	}
	return out, nil
}

func (b *Backend) Attach(ctx context.Context, id, token string) (*websocket.Conn, error) {
	sb, err := b.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sb.State != backend.StateRunning || sb.Endpoint == "" {
		return nil, fmt.Errorf("dockerbe: sandbox %s is %s with endpoint %q: %w", id, sb.State, sb.Endpoint, backend.ErrNotRunning)
	}
	return backend.Dial(ctx, sb.Endpoint, token)
}

// Destroy is idempotent. Task 13's reaper and an explicit DELETE race by
// design, so "already gone" is the outcome both wanted.
func (b *Backend) Destroy(ctx context.Context, id string) error {
	err := b.api.ContainerRemove(ctx, namePrefix+id, container.RemoveOptions{Force: true, RemoveVolumes: true})
	if err == nil || client.IsErrNotFound(err) {
		return nil
	}
	return fmt.Errorf("dockerbe: remove %s: %v: %w", id, err, backend.ErrUnavailable)
}

func portsOf(resp container.InspectResponse) nat.PortMap {
	if resp.NetworkSettings == nil {
		return nil
	}
	return resp.NetworkSettings.Ports
}

// endpointFromPortMap reads the inspect shape. An empty endpoint with a nil
// error means "not published yet", which for a starting container is normal.
func (b *Backend) endpointFromPortMap(ports nat.PortMap) (string, error) {
	var offLoopback string
	for _, bd := range ports[attachPort] {
		if bd.HostPort == "" {
			continue
		}
		if bd.HostIP == b.bindIP {
			return net.JoinHostPort(b.bindIP, bd.HostPort), nil
		}
		if isWildcardIP(bd.HostIP) {
			offLoopback = bd.HostIP
		}
	}
	if offLoopback != "" {
		return "", fmt.Errorf("dockerbe: %s is published on %q instead of %s: %w", attachPort, offLoopback, b.bindIP, backend.ErrNonLoopbackPublish)
	}
	return "", nil
}

// endpointFromSummaryPorts reads the list shape, which reports the same fact in
// a different struct.
func (b *Backend) endpointFromSummaryPorts(ports []container.Port) (string, error) {
	var offLoopback string
	for _, p := range ports {
		if int(p.PrivatePort) != backend.ContainerPort || p.Type != "tcp" || p.PublicPort == 0 {
			continue
		}
		if p.IP == b.bindIP {
			return net.JoinHostPort(b.bindIP, strconv.Itoa(int(p.PublicPort))), nil
		}
		if isWildcardIP(p.IP) {
			offLoopback = p.IP
		}
	}
	if offLoopback != "" {
		return "", fmt.Errorf("dockerbe: port %d is published on %q instead of %s: %w", backend.ContainerPort, offLoopback, b.bindIP, backend.ErrNonLoopbackPublish)
	}
	return "", nil
}

// isWildcardIP names the three spellings of "every interface". An IPv6
// loopback binding ("::1") is neither wildcard nor usable here, so it is
// ignored and the container reads as not-yet-published.
func isWildcardIP(ip string) bool {
	return ip == "" || ip == "0.0.0.0" || ip == "::"
}

// stateFrom maps container.ContainerState (a string alias whose constants are
// created, running, paused, restarting, removing, exited, dead) onto the four
// states a backend is allowed to know about.
func stateFrom(status string) backend.State {
	switch status {
	case "running":
		return backend.StateRunning
	case "created", "restarting":
		return backend.StateStarting
	default:
		return backend.StateStopped
	}
}

// parseCreated prefers the daemon's own timestamp and falls back to the label
// this package wrote, so a sandbox created by an older build still sorts.
func parseCreated(raw string, labels map[string]string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, labels[backend.LabelCreatedAt]); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
```

One decision inside that file is easy to reverse by accident. `Create`'s two waits share a single deadline, computed once before the port poll and handed to `WaitReady` as `time.Until(deadline)`. Giving each wait its own budget is the natural way to write it and costs an operator the only diagnostic they will have: once task 13 lands `PANACEA_APP_READY_TIMEOUT` and someone sets it to 60 s, a `Create` that blocks for 120 s tells them nothing — a daemon that took 59 s to publish a port and a sandboxd that hung read identically, because the number they set no longer bounds the answer. The `remaining <= 0` branch is there because `backend.WaitReady` with a non-positive timeout never enters its poll loop and reports `last error: <nil>`, which reads as a broken dialer rather than an expired budget.

- [ ] **Step 8: Run the tests and watch them pass**

Run, from the repository root: `GOPROXY=off go test ./control-plane/internal/backend/dockerbe/... -race -count=1 -v`

Expected: every test `--- PASS`, then `ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/dockerbe	<seconds>s`, no race reports. Twenty-four test functions. Total wall time is under five seconds: only two tests deliberately wait, `TestCreateDestroysTheSandboxThatNeverAnswersHealthz` at 400 ms and `TestCreateSpendsOneReadyBudgetAcrossBothWaits` at roughly 1.2 s.

**Failure mode.** `TestCreateBuildsTheContractedContainerAndReturnsALoopbackEndpoint` hanging for 60 s and then failing means `Config.ReadyTimeout` was ignored and `backend.ReadyTimeout` was used unconditionally — check the `if readyTimeout <= 0` branch in `New`.

**Failure mode.** `TestCreateSpendsOneReadyBudgetAcrossBothWaits` reporting about 2 s against a 1 s budget means the two waits each started their own clock: `waitForEndpoint` derived a deadline instead of taking Create's, or `WaitReady` was handed `b.readyTimeout` rather than `time.Until(deadline)`. The same test failing with `last error: <nil>` in the message means the `remaining <= 0` guard is missing and `WaitReady` was called with a non-positive timeout.

**Failure mode.** `Endpoint = "", want 127.0.0.1:<port>` means `endpointFromPortMap` compared against the wrong key. `nat.PortMap` is keyed by `"8080/tcp"`, not `"8080"`; `attachPort` must include the `/tcp` suffix.

**Failure mode.** `Destroy of a gone sandbox = ... want nil` means `client.IsErrNotFound` did not recognise the fake's error. That function is an alias for `cerrdefs.IsNotFound`, which accepts either `errors.Is(err, cerrdefs.ErrNotFound)` **or** any error with a `NotFound()` method — so the fake's `notFoundErr` must declare `func (e notFoundErr) NotFound() {}` with a value receiver, and the fake must return the value, not a pointer.

**Failure mode.** A data race reported inside `fakeDocker` means a field was read in an assertion without the mutex. Use `snapshotRemoves`/`snapshotCreates`; the only fields read directly are those touched after every goroutine in the test has returned.

- [ ] **Step 9: Run the whole Go suite and re-check the module file**

Run, from the repository root: `GOPROXY=off go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected: every package `ok` or `[no test files]`; `dockerbe`, `backend` and `ids` all `ok`. Never `go test ./...` at the workspace root — the root is not a module and that form errors with `directory prefix . does not contain modules listed in go.work`.

Then, in order:

```bash
gofmt -l control-plane/internal/backend/dockerbe
GOPROXY=off go vet ./control-plane/internal/backend/dockerbe/...
git diff --stat control-plane/go.mod control-plane/go.sum
grep -n '^go ' control-plane/go.mod
```

No `-mod=mod` on the `go vet` line, and none on the `go test` line above it. Both are workspace-mode commands and both exit 1 with `go: -mod may only be set to readonly or vendor when in workspace mode` if you add it.

Expected: `gofmt -l` prints nothing; `go vet` prints nothing; the diff touches only `go.mod` and `go.sum`; `grep` prints exactly `3:go 1.23`.

That `git diff --stat` is also what closes contract §8 open question 5 ("task 5 must add explicit `require` lines"): record in the commit body that seventeen requires were added by name, and that contract §2.4's cached-module table is missing `github.com/Microsoft/go-winio v0.6.2`, which Windows builds need.

**Failure mode.** `grep` printing `go 1.24` means some earlier command bumped the directive. Fix it by editing the line back to `go 1.23` and re-running the suite — do not leave it, because CI pins 1.23 and a network-restricted agent will try to download a toolchain and fail with `go: downloading go1.24.x` followed by a TLS error.

- [ ] **Step 10: Check the two Docker assumptions against a real daemon**

The unit tests prove the code behaves as specified. They cannot prove the specification matches Docker, because the fake answers whatever the test says. Contract §8 open questions 3 and 4 are exactly that gap, and they are this task's to close:

- **Q3** — the `container.Summary.State` mapping in `stateFrom` was written from the SDK's constants, not from a live daemon.
- **Q4** — the claim that a container bound to `127.0.0.1` yields one usable IPv4 binding and possibly a separate IPv6 one, and that `0.0.0.0` is a defect rather than a variation, was reasoned from the SDK types and never observed.

This step needs Docker Desktop running. Use the **PowerShell** tool, not Git Bash, which mangles container paths. **Pipe exit-code trap: `docker ... | Select-Object -First 5` reports the pipeline's status, not `docker`'s** — run each command bare and read its output, and never conclude "it worked" from a piped invocation's exit code.

Run, one at a time:

```powershell
docker image ls panacea-app/sandbox:dev
docker run -d --name panacea-app-probe -p 127.0.0.1::8080 -l panacea-app.managed=true panacea-app/sandbox:dev
docker port panacea-app-probe
docker ps -a --filter label=panacea-app.managed=true --format "{{.Names}} {{.State}} {{.Ports}}"
docker stop panacea-app-probe
docker ps -a --filter label=panacea-app.managed=true --format "{{.Names}} {{.State}} {{.Ports}}"
docker rm -f panacea-app-probe
```

Expected from `docker port`: one line `8080/tcp -> 127.0.0.1:<ephemeral>`, and possibly a second `8080/tcp -> [::1]:<same>`. **Neither line may say `0.0.0.0`.** If one does, the assumption behind `isWildcardIP` is wrong for this Docker Desktop version and the finding is a blocker, not a footnote: stop and report it, because the whole security property of this package rests on that binding.

Expected from the first `docker ps -a`: the `State` column reads `running`. From the second, run after `docker stop`: `exited`. Those are the two values `stateFrom` treats as `StateRunning` and `StateStopped`, and they are why the probe is stopped before it is removed — `docker rm -f` on a running container leaves the `exited` half of Q3 unobserved. Record both observed strings verbatim; if either differs, `stateFrom` has a case it does not handle and that is a blocker for task 13's reaper, which decides what to destroy from exactly this field.

**If the image does not exist** (`docker image ls` prints only the header), do not substitute a random public image — the corporate proxy blocks the pull and a half-hour goes into the wrong problem. Build it first per `images/sandbox/`, or, if that is out of scope for this sitting, record Q3 and Q4 as **still unobserved** in the commit body and leave them for task 25's end-to-end verification. State what is unverified; a passing unit suite is not evidence that Docker behaves as assumed.

- [ ] **Step 11: Commit**

```bash
git add control-plane/internal/backend/dockerbe control-plane/go.mod control-plane/go.sum
git commit -m "feat(backend): Docker backend, published to loopback only

This is the task that makes the local-first pivot real: the product runs on a
laptop with Docker Desktop and no cloud spend, and this is also the genuine
local-machine feature, not a stand-in for katabe.

Two rules are pinned by tests rather than by review. A sandbox port published
anywhere but 127.0.0.1 is an unauthenticated shell on the corporate LAN, so New
refuses a non-loopback BindIP unless told to on purpose and Create destroys a
container the daemon published off loopback. And Destroy is idempotent, because
the reaper and an explicit DELETE race by design and a reaper that errors on
'already gone' stops sweeping.

api.go exists so the package is testable with no daemon: it is the seven SDK
methods used here, copied verbatim, with a compile-time assertion that the real
client still satisfies them. All seventeen Docker requires are pinned by name
rather than resolved, because proxy.golang.org is TLS-intercepted from this
network and docker's +incompatible module ships no go.mod, so nothing is
reachable transitively.

One deliberate amendment to contract 2.4: List does not return
ErrNonLoopbackPublish. Erroring a sweep over one bad container blinds the
reaper to the container that most needs destroying. It returns that sandbox
StateStopped with no endpoint and logs a warning instead; Get and Attach still
refuse loudly.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
