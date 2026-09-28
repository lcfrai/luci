### Task 23b: the product `CheckRunner` — one bounded command in a check sandbox, exit code only

W1, stream WP. **No new seam**: this task fills the `CheckRunner` interface task 23 declares, and
SW.4's done rule reads the result without changing. About 3 hours (an estimate, not a measurement).
Depends on task 23 (the interface, `Check`, `Observation`, `Observer`, `ErrNoRunner`) and **P3-24**
(`sandboxd`, the in-sandbox runner) sync-merged. Write it `dependsOn: 23; P3-24`.

**Why this task exists, in one sentence.** Without it `Observer.Runner` is nil on every product
path, `Run` returns `ErrNoRunner`, nothing is ever observed, and an issue closes only when a person
presses Record — `proven_self_attested`, the visibly weaker word — so rule 2 of this plan (*done
means proven*, `OD-27`) and the demo's no-click `done` (05 §5.3 row 7) are true in a test double
and false in the product.

**Anchors, read in this worktree on 2026-09-28 unless marked.**
- `backend.Backend` is six methods — `Kind`, `Create`, `Get`, `List`, `Attach`, `Destroy`, `Close`
  — at `control-plane/internal/backend/backend.go:337-345`. **None of them runs a command or
  reports an exit code**, and this task adds none (decision 1).
- `backend.Sandbox.Endpoint` is the sandbox's own address, `127.0.0.1:<published port>` for
  `dockerbe` and `<podIP>:8080` for `katabe` (`backend.go:300-307`). It is the one thing both
  backends have in common at run time, and it is what decision 1 turns on.
- `backend.Spec` carries `Image`, `Command`, `Args`, `Env`, `AttachToken`, `Resources`, `WorkDir`
  and `Mounts []Mount // docker only` (`backend.go:181-192`), and `ValidateSpec` rejects a caller
  key in `PANACEA_APP_*` (`backend.go:224-229`).
- `dockerbe.APIClient` is a deliberately narrow slice of the Docker SDK (`dockerbe/api.go:15-24`),
  `katabe` has no `rest.Config` and no use of the pod `exec` subresource (`katabe/kata.go`, grep).
- P3-24 builds `sandboxd`'s git runner: our own daemon **inside** the sandbox, a git RPC on its
  existing **authenticated HTTP listener** at `/git/v1/` with bearer auth and **five ops** (P3
  `task-24.md:1`, `:6`, `:27`, `:67`), mounted beside `/attach` at `sandboxd/internal/server/
  server.go:85` (P3 `task-24.md:71-72`), reading its token from `PANACEA_APP_ATTACH_TOKEN` (P3
  `task-24.md:55`). Its title is *lane git inside the sandbox, lane work as a bundle*, so a bundle
  already crosses that boundary. **No op runs a command**, and the wire shapes are P3
  `task-24.md:138-196`. Cited from phase 3's plan at `9935278`; **every phase 3 path is UNVERIFIED
  until merge** and the owner phase wins any conflict (rule H2).
- Task 23 declares `CheckRunner`, `CheckRun`, `Check`, `Observation` and `ErrNoRunner` in
  `control-plane/internal/evidence/observe.go` (`task-23.md:71-77`, its files list at `:89-90`).

**Decision 1, declared here because the review left it open: the check runs through `sandboxd`, and
`backend.Backend` gains nothing.** The two candidates were an `Exec`/`Run` method on `Backend` and
a sixth operation on P3-24's `sandboxd`. `sandboxd` wins on four counts, and the reasons are
recorded so nobody re-derives them:

1. **It is the only path that works on both backends.** `dockerbe` would need three more Docker SDK
   calls on a surface kept narrow on purpose (`api.go:15-24`), and `katabe` attaches over pod
   networking rather than the Kubernetes `exec` subresource (`backend.go:300-302`), so it has no
   exec client at all and would need a new one. Both already reach `Sandbox.Endpoint`.
2. **Half of it is already there.** `sandboxd` already runs git inside the sandbox and already
   takes lane work **as a bundle** (P3 `task-24.md:1`), on an authenticated listener the control
   plane already talks to. The check is one more route on that listener, not a second protocol.
3. **Only the exit code crosses the boundary** (`OD-8`). The operation's reply is
   `{exitCode, ranAt, timedOut}` and there is no field for output, so no later caller can start
   deriving meaning from bytes: the plumbing does not exist.
4. **`Backend` stays six methods**, so the registry, the reaper and both backends are untouched by
   a W1 task, and the Kata path (`OD-1`) needs nothing extra when phase 8 stream C lands.

**Decision 2: the check runs in its own sandbox, and an agent's sandbox never offers the
operation.** The runner creates a short-lived **check sandbox** from the project's image with no
agent process, runs one command in it and destroys it. `sandboxd` refuses `check` unless its
sandbox was created with `PANACEA_APP_ROLE=check`, which only this runner sets, so the `sh -c`
verb is not reachable from a lane sandbox an agent is sitting in. This is what makes *observed*
evidence evidence: the command ran on the landed commit in a machine with nothing else in it, not
in the lane where the work was done.

**Decision 3: the bundle is the only input, and it is read-only.** Task 23's `Observer` bundles the
landed commit out of the project and hands the bundle to the runner; the runner puts that one file
into the check sandbox and nothing else — no host mount, no credentials, no gateway token, no
network by default. A check that needs the network to pass is a check that cannot run here, and
that is stated, not worked around.

**Decision 4: every bound is the runner's, not the check's.** One command, `sh -c`, 30 minutes by
default and never more than 60 (task 23's `Check.Timeout` is the caller's request, clamped here),
output discarded unread, and the sandbox destroyed in a `defer` that runs on every path including
a panic. A timeout is **not** exit 0, so it writes nothing, exactly like any other non-zero exit.

**What *observed* does and does not mean, restated so the words stay honest.** In local mode this
runner is the person's own control plane, and the Hub records the move as that person's. The Hub
cannot tell a watched run from a claimed one. What *observed* adds is that no human typed the
result and the command, the commit, the exit code and the time are in the record. In the cloud
(phase 6, phase 8) the same runner runs on the platform, and that is when *observed* means
*observed by someone other than you* — no code change here, because decision 1 kept the path the
same on both backends.

**Files** (every `sandboxd/` path is phase 3's and **UNVERIFIED until merge**):
- `sandboxd/internal/checkrunner/runner.go` + `wire.go` + `limits.go` + `*_test.go` ⚑ — the
  `/check/v1/run` route, its shapes and its bounds (new). **Not** inside
  `sandboxd/internal/gitrunner`: that package stays git-only, so nothing widens the five ops
  (P3 `task-24.md:67`)
- `sandboxd/internal/server/server.go` ⚑ (P3 `task-24.md:71-72`) — one
  `s.mux.HandleFunc("/check/v1/", s.check)` beside `/git/v1/` and `/attach` at `server.go:85`,
  behind the same bearer check (modified)
- `control-plane/internal/evidence/sandboxcheck.go` + `sandboxcheck_test.go` — `SandboxRunner`,
  which satisfies task 23's `CheckRunner` (`task-23.md:355-357`) (new)
- `control-plane/cmd/api/main.go` — task 23 step 4's `Runner: nil` becomes the real runner
  (`task-23.md:579-581`) (modified)

**Interfaces:**
- Produces: `evidence.SandboxRunner`, `evidence.ErrCheckUnsupported`; `sandboxd`'s `/check/v1/run`
  route and its two wire types. Consumed by task 23's `Observer` and by nothing else in W1
- Consumes: task 23 — `evidence.CheckRun`, `CheckRunner`, `Observer.Runner`; phase 1 —
  `backend.Backend` (`Create`, `Get`, `Destroy`), `backend.Spec`, `backend.Sandbox.Endpoint`,
  `backend.Resources`; P3-24 — `sandboxd`'s server, its bearer auth and its bundle handling
- Amends: nothing. No seam changes, no wire field is added to SW.1-SW.21, no golden vector moves

#### Interfaces — complete

```go
// sandboxd/internal/checkrunner/wire.go — POST /check/v1/run. Decoded with
// DisallowUnknownFields; a body past MaxJSONBytes, an unknown field or trailing data is 400.
type RunRequest struct {
	Bundle    []byte `json:"bundle"`    // the git bundle, base64 in JSON; the ONLY input
	Ref       string `json:"ref"`       // the ref the bundle holds
	Commit    string `json:"commit"`    // 40 hex; checked out detached
	Command   string `json:"command"`   // run as `sh -c Command`; 1 to 1000 bytes
	TimeoutMS int64  `json:"timeoutMs"` // clamped to [1000, 3600000] here, not trusted
}

// RunResponse has no field for output, and that is the point: nothing downstream can start
// deriving meaning from the process's bytes, because the bytes never leave the sandbox (OD-8).
type RunResponse struct {
	ExitCode int   `json:"exitCode"` // 124 when the timeout fired (coreutils' convention)
	TimedOut bool  `json:"timedOut"`
	RanAtMS  int64 `json:"ranAtMs"` // the sandbox's clock, for the Record's because line
}
```

```go
// control-plane/internal/evidence/sandboxcheck.go
package evidence

// SandboxRunner runs one check in a sandbox of its own: created from the project's image with
// PANACEA_APP_ROLE=check and no agent process, given the bundle and nothing else, destroyed on
// every path. It is the product CheckRunner; task 23's hostRunner stays a test double.
type SandboxRunner struct {
	Backend   backend.Backend
	Image     string            // the check image; the project's sandbox image by default
	Resources backend.Resources
	NewID     func() string     // sandbox ids; must satisfy backend.Spec.ID's pattern
	Token     func() string     // a fresh attach token per check, never reused
	Dial      func(ctx context.Context, endpoint string) *http.Client // nil means the default
	Now       func() time.Time
}

// RunCheck satisfies CheckRunner. A timeout is (124, nil); a runner that could not run at all
// is (0, err) — including ErrCheckUnsupported, which a backend with no reachable endpoint
// returns so the Observer records nothing rather than guessing.
func (r *SandboxRunner) RunCheck(ctx context.Context, run CheckRun) (exitCode int, err error)

var ErrCheckUnsupported = errors.New("evidence: this backend cannot run a check")
```

#### Steps

- [ ] **Step 1: the `/check/v1/run` route in `sandboxd`, tests first**

In `sandboxd/internal/checkrunner/runner_test.go` (no Docker; the handler is served by
`httptest.NewServer` and the check commands are `true`, `false`, `sleep` and `cat`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRunRefusedWithoutTheCheckRole` | `PANACEA_APP_ROLE` unset or `agent` → 403 `role_not_check`, and no process is started | the `sh -c` verb reachable from a lane sandbox an agent is sitting in |
| `TestRunRefusedWithoutTheToken` | the bearer check is the server's existing one, exercised on `/check/v1/` too | a second, weaker door beside `/git/v1/` |
| `TestRunChecksOutTheCommitNotTheTip` | a bundle with two commits and a command that reads a file added at the tip → asked for the parent, the exit is non-zero | a runner that checks out the bundle's ref |
| `TestRunReportsOnlyTheExitCode` | the response's JSON keys are exactly `exitCode`, `timedOut`, `ranAtMs`, for a command that writes to both stdout and stderr | output plumbed back, which a later caller would read (`OD-8`) |
| `TestRunTimeoutIs124AndNotZero` | `sleep 5` with `timeoutMs: 1000` → `{124, timedOut: true}` in under 3 s, and the process group is gone | a timeout that returns 0 and is recorded as proof |
| `TestRunClampsTheTimeout` | `timeoutMs: 0` and `timeoutMs: 86400000` → the floor and the 60-minute cap, not the caller's number | a check that holds a sandbox for a day |
| `TestRunRejectsASecondRun` | a second POST while one is running → 409; the route is one check per sandbox, ever | a sandbox reused between checks, which is no longer a fresh clone |

Run: `go test ./sandboxd/... -race -count=1 -run 'Run'`. Expected: FAIL, `undefined:
checkrunner.New`. Write the package and the one `HandleFunc`; re-run. Expected: `ok`.

- [ ] **Step 2: `SandboxRunner` in the control plane, tests first**

In `control-plane/internal/evidence/sandboxcheck_test.go`, against a `backend.Backend` stub that
records its `Spec` and an `httptest` stand-in for `sandboxd`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCheckSandboxIsDestroyedOnEveryPath` | exit 0, exit 1, a timeout, a dial failure and a panic in the client → `Destroy` called exactly once in each, with the id `Create` returned | a leaked sandbox, and a `defer` skipped on the error path |
| `TestCheckSpecCarriesNoHostMountAndNoAgent` | the recorded `Spec` has `Mounts == nil`, no `PANACEA_APP_*` value from the caller, `Env["PANACEA_APP_ROLE"] == "check"`, and a `Command` that is not the agent CLI | a check running beside the agent, or reading the host |
| `TestCheckTokenIsFreshPerRun` | two runs → two different `AttachToken`s, and neither is logged | a shared token that outlives its sandbox |
| `TestCheckUnsupportedIsNotAFailedCheck` | `Sandbox.Endpoint == ""` → `(0, ErrCheckUnsupported)` and, through `Observer.Run`, **no Record** | an unreachable sandbox read as a failing check, which would look like a verdict |
| `TestRunCheckPassesTheClampedTimeout` | `CheckRun.Timeout` of 90 minutes → `timeoutMs` 3600000 on the wire | the cap living only in the sandbox |

Run: `go test ./control-plane/internal/evidence/... -race -count=1 -run 'Check'`. Expected: FAIL,
`undefined: evidence.SandboxRunner`. Write `sandboxcheck.go`; re-run. Expected: `ok`.

- [ ] **Step 3: wire it, and turn task 23's fallback off**

Replace task 23 step 4's `Observer{…, Runner: nil}` in `control-plane/cmd/api/main.go`
(`task-23.md:579-581`) with `Runner: &evidence.SandboxRunner{Backend: be, Image: cfg.SandboxImage,
…}`. Then, with a Docker daemon, one end-to-end test in
`control-plane/internal/api/observed_test.go`: a project with one issue serving one criterion
whose `Serve.Check` is `true`, a land, and the head reads `done · proven` with **no Record posted
by anyone**. Guard it exactly as `dockerbe`'s own tests guard a missing daemon — read
`control-plane/internal/backend/dockerbe/docker_test.go` and copy that guard rather than inventing
a second one. Name it `TestObservedExitZeroClosesWithNoClickInTheProduct`: it is the one test that
tells the demo's last frame (05 §5.3 row 7) from a test double.

- [ ] **Step 4: Acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/evidence/... ./control-plane/internal/api/... ./control-plane/internal/backend/... -race -count=1'
```

Expected: one `ok` per package, no `FAIL`; and `go test ./sandboxd/... -race -count=1` prints `ok`
in phase 3's module. Record whether the Docker-guarded test ran or skipped — a skip is not a pass,
and `OD-27` says done means proven. Commit with `git commit -F <msgfile> -- <the paths above>`.

#### What this task does not claim

- **Not that a check runs on Kata.** W1's demo is local mode (skeleton `:17-19`, `OD-20`). The
  route is the sandbox's own, so `katabe` needs no code change, but nothing here proves it on a
  Kata pod; phase 8 stream C does, and until it has, `katabe` answers `ErrCheckUnsupported` and an
  issue there closes `proven_self_attested`.
- **Not who may author a check.** The field is `Serve.Check` (task 02) and the authority is SW.3's
  (task 07): a check is set by a team member with the brief's write verb, never carried in by
  intake or by a `text·A` revision. This task runs what the revision holds; it does not judge it.
- **Not a check that needs the network, a secret, a service or another repository.** The sandbox
  gets the bundle and nothing else. A check that cannot pass under that is a check this platform
  cannot observe, and the issue closes `proven_self_attested` instead — which is the honest word.
- **Not more than one command per criterion**, no matrix, no caching, no retry, and no scheduling:
  a check runs when the Observer asks, once.
- Design: 05 §3.3, §5.3 row 7. Parity: serves **L-8** with tasks 22 and 23; proof 70. It closes no
  row of its own — it is what makes task 23's claim true in the product.
