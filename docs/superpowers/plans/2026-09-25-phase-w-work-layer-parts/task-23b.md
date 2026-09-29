### Task 23b: a criterion's check, end to end — the product `CheckRunner` (one bounded command in a check sandbox, exit code only) and the check line a person sets it on

W1, stream WP. **No new seam**: this task fills the `CheckRunner` interface task 23 declares, and
SW.4's done rule reads the result without changing. About 4 hours (an estimate, not a
measurement): 3 for the runner, 1 for the check line. Depends on task 23 (the interface, `Check`,
`Observation`, `Observer`, `ErrNoRunner`), task 47 (the issue view's `Serves.tsx`, whose
`ServeRow` carries the line) and **P3-24** (`sandboxd`, the in-sandbox runner) sync-merged. Write
it `dependsOn: 23, 47; P3-24`.

**Why this task exists, in one sentence.** Without it `Observer.Runner` is nil on every product
path, `Run` returns `ErrNoRunner`, nothing is ever observed, and an issue closes only when a person
presses Record — `proven_self_attested`, the visibly weaker word — so rule 2 of this plan (*done
means proven*, `OD-27`) and the demo's no-click `done` (05 §5.3 row 7) are true in a test double
and false in the product. **And a runner alone is not enough**: before this task no surface in
the plan writes `Serve.Check` (every serves write, task 42's `T` and task 47's picker, sends the
pair alone), so the field would travel the wire empty for ever and there would be nothing to run.

**Anchors, read in this worktree on 2026-09-28 unless marked.**
- `backend.Backend` is seven methods — `Kind`, `Create`, `Get`, `List`, `Attach`, `Destroy`, `Close`
  — at `control-plane/internal/backend/backend.go:337-345`. **None of them runs a command or
  reports an exit code**, and this task adds none (decision 1).
- `backend.Sandbox.Endpoint` is the sandbox's own address, `127.0.0.1:<published port>` for
  `dockerbe` and `<podIP>:8080` for `katabe` (`backend.go:300-307`). It is the one thing both
  backends have in common at run time, and it is what decision 1 turns on.
- `backend.Spec` carries `Image`, `Command`, `Args`, `Env`, `AttachToken`, `Resources`, `WorkDir`
  and `Mounts []Mount // docker only` (`backend.go:181-192`) — **no network field**; `dockerbe`
  uses the default bridge (`dockerbe/docker.go:235-236`). `ValidateSpec` refuses a `Spec.Env` key
  with the reserved prefix `PANACEA_APP_` (`backend.go:62`, `:275-279`), and `EnvEntries` drops one
  that reaches it anyway (`:242-245`); it also refuses an empty `Command` (`:269-271`), which the
  image's entrypoint hands to `sandboxd` as its session (`images/sandbox/entrypoint.sh:6`, `:13-17`).
- A session's `Spec.Env` is `sessionEnv`'s fixed keys and nothing a caller chooses
  (`control-plane/internal/api/sessions.go:866`, `:1536`), so a key only this task sets is one no
  lane sandbox carries.
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
  `control-plane/internal/evidence/observe.go` (`task-23.md:71-77`, its files list at `:91-92`), and
  its `Check.Timeout` rule is *zero means 10 minutes; at most 30 minutes* (`task-23.md:330`). Its
  checks are the evaluated revision's `Serve.Check` values, and `Serve` carries no timeout
  (`task-02.md:252-256`), so every product check runs at the 10-minute default.

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
4. **`Backend` stays seven methods**, so the registry, the reaper and both backends are untouched by
   a W1 task, and the Kata path (`OD-1`) needs nothing extra when phase 8 stream C lands.

**Decision 2: the check runs in its own sandbox, and an agent's sandbox never offers the
operation.** The runner creates a short-lived **check sandbox** from the project's image with no
agent process, runs one command in it and destroys it. `sandboxd` refuses `check` unless its
sandbox was created with `SANDBOXD_ROLE=check` in `Spec.Env`, which only this runner sets, so the
`sh -c` verb is not reachable from a lane sandbox an agent is sitting in. The name is **not** a
`PANACEA_APP_*` one: that prefix is the backend's own, `ValidateSpec` refuses it from a caller,
and a `PANACEA_APP_ROLE` would fail `Create` on both backends before anything ran. The session
`Command` is `/bin/sleep` with `Args: ["infinity"]`: `ValidateSpec` needs a command and the
entrypoint makes it `sandboxd`'s session, so the one process beside the check is idle, never a
harness. This is what makes *observed* evidence evidence: the command ran on the landed commit in
a machine with nothing else in it, not in the lane where the work was done.

**Decision 3: the bundle is the only input, and it is read-only.** Task 23's `Observer` bundles the
landed commit out of the project and hands the bundle to the runner; the runner puts that one file
into the check sandbox and nothing else — no host mount, no credentials, no gateway token. The
network is any sandbox's, not less: `backend.Spec` has no network field and `dockerbe` uses the
default bridge, and cutting it would change `Backend` (decision 1). So a check reaches what a lane
sandbox reaches and holds no credential to use there; a check that needs a secret or a service to
pass cannot run here, and that is stated, not worked around.

**Decision 4: every bound is the runner's, not the check's.** One command, `sh -c`, 10 minutes by
default and never more than 30 — task 23's `Check.Timeout` rule, applied here and again in
`sandboxd`, which trusts no caller; task 23's `AfterLand` bounds each run at 35 minutes, the cap
plus the sandbox's create and destroy — output discarded unread, and the sandbox destroyed in a
`defer` that runs on every path including a panic. A timeout is **not** exit 0, so it writes
nothing, exactly like any other non-zero exit.

**Decision 5: a check is set on its serves row, and setting one is a revision.** The one place a
person writes `Serve.Check` is a **check line** under each row of task 47's `Serves` list: the
command in Commit Mono (`.iv-fact`, `OD-28`: a command is a machine fact), or the words *No check
— closes on a Record* when there is none. On the head revision a member presses **Set check** (or
**Edit check**), types, and `Enter` writes a new revision exactly as task 47 removes a criterion
(its decision 3): one `revise` op whose `serves` is the same list with that one pair's `check`
replaced, so undo is task 56's inverse `revise`, and optimism is safe because the act is
reversible. `Escape` abandons it; an empty line clears the check. Four rules come with it:
1. **The cap is the Hub's**: 1000 **UTF-8 bytes** (`MaxCheckBytes`, task 02), counted with
   `TextEncoder`, not `maxLength` (which counts UTF-16 units). Over it, `Enter` writes nothing
   and the line says *Too long for a check (1000 bytes at most)*: a sentence, never a count.
2. **Every other pair keeps its check.** The list is rewritten by identity, so a write that
   touches one pair cannot drop another's command — the mutant a desktop `Serve` without `check`
   would have built on every title edit. Task 37's `Serve` carries `check?: string` for this.
3. **No new letter key.** The control is a button in the row's Tab order and the field takes only
   `Enter` and `Escape`; a letter bound here would compete with the list's `T` and the global map
   (tasks 53-54), for an act done once per criterion.
4. **Who may set one is SW.3's rule, not this line's**: the line is offered on the head to the
   person viewing it, and the Hub's revision write enforces who may add or change a check.

**What *observed* does and does not mean, restated so the words stay honest.** In local mode this
runner is the person's own control plane, and the Hub records the move as that person's. The Hub
cannot tell a watched run from a claimed one. What *observed* adds is that no human typed the
result and the command, the commit, the exit code and the time are in the record. In the cloud
(phase 6, phase 8) the same runner runs on the platform, and that is when *observed* means
*observed by someone other than you* — no code change here, because decision 1 kept the path the
same on both backends.

**Files** (`sandboxd/internal/server/server.go` is built, `:85` read in this worktree; P3-24's
`/git/v1/` mount and bundle handling beside it are **UNVERIFIED until merge**):
- `sandboxd/internal/checkrunner/runner.go` + `wire.go` + `limits.go` + `*_test.go` — the
  `/check/v1/run` route, its shapes and its bounds; `checkrunner.New(role string)` (new). **Not**
  inside `sandboxd/internal/gitrunner`: that package stays git-only, so nothing widens the five
  ops (P3 `task-24.md:67`)
- `sandboxd/internal/server/server.go` ⚑ (P3 `task-24.md:71-72`) — in `New`, `s.check =
  checkrunner.New(os.Getenv("SANDBOXD_ROLE"))`, read once at start, and one
  `s.mux.HandleFunc("/check/v1/", s.check)` beside `/git/v1/` and `/attach` at `server.go:85`,
  behind the same bearer check (modified)
- `control-plane/internal/evidence/sandboxcheck.go` + `sandboxcheck_test.go` — `SandboxRunner`,
  which satisfies task 23's `CheckRunner` (`task-23.md:355-357`) (new)
- `control-plane/cmd/api/main.go` — task 23 step 4's `Runner: nil` becomes the real runner
  (`task-23.md:579-581`) (modified)
- `desktop/src/components/issue/CheckLine.tsx` + `CheckLine.test.tsx` — the check line, `MAX_CHECK_BYTES`,
  `checkBytes`, `withCheck` (new)
- `desktop/src/components/issue/Serves.tsx` ⚑ + `Serves.test.tsx` ⚑ (task 47's) — `ServeRow` gains
  `onSetCheck` and renders `CheckLine`; one test that the other pairs' checks survive (modified)
- `desktop/src/components/issue/issue.css` ⚑ (task 47 appends to it) — `.iv-serve` wraps, and three
  `.iv-check*` rules (modified)

**Interfaces:**
- Produces: `evidence.SandboxRunner`, `evidence.ErrCheckUnsupported`; `sandboxd`'s `/check/v1/run`
  route and its two wire types, consumed by task 23's `Observer` and by nothing else in W1; the
  desktop's `CheckLine`, the one writer of `Serve.Check` in W1
- Consumes: task 23 — `evidence.CheckRun`, `CheckRunner`, `Observer.Runner`; phase 1 —
  `backend.Backend` (`Create`, `Get`, `Destroy`), `backend.Spec`, `backend.Sandbox.Endpoint`,
  `backend.Resources`; P3-24 — `sandboxd`'s server, its bearer auth and its bundle handling;
  task 47 — `Serves`, `ServeRow`, its `write`; task 37 — `Serve` with `check?: string`; task 56 —
  `mutate`'s `revise` op; task 20 — `makeServe`, `expectNoPrediction`
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
	TimeoutMS int64  `json:"timeoutMs"` // clamped to [1000, 1800000] here, not trusted
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
// Env {"SANDBOXD_ROLE": "check"}, Command "/bin/sleep", Args ["infinity"] and no agent process,
// given the bundle and nothing else, destroyed on every path. The timeout sent is CheckRun's,
// zero as 10 minutes and anything above 30 as 30 (task 23's Check.Timeout rule). It is the
// product CheckRunner; task 23's hostRunner stays a test double.
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

```tsx
// desktop/src/components/issue/CheckLine.tsx
import { useState, type JSX, type KeyboardEvent } from "react";
import type { Serve } from "../../api/work";

/** hubapi.MaxCheckBytes (task 02): the Hub counts UTF-8 bytes, so this does too. */
export const MAX_CHECK_BYTES = 1000;
export const checkBytes = (s: string): number => new TextEncoder().encode(s).length;

/** The pair with its check set, or with no `check` key at all when `check` is "" (omitempty). */
export function withCheck(s: Serve, check: string): Serve {
  const { check: _old, ...pair } = s;
  return check === "" ? pair : { ...pair, check };
}

/** One serves pair's check (decision 5). Read-only unless `canEdit` (the head revision). */
export function CheckLine(props: { serve: Serve; canEdit: boolean; onSet(check: string): void }): JSX.Element {
  const { serve, canEdit, onSet } = props;
  const [draft, setDraft] = useState<string | null>(null);        // null: not editing
  const next = draft === null ? "" : draft.trim();
  const tooLong = checkBytes(next) > MAX_CHECK_BYTES;
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") { e.preventDefault(); setDraft(null); return; }
    if (e.key !== "Enter" || tooLong) return;
    e.preventDefault();
    if (next !== (serve.check ?? "")) onSet(next);
    setDraft(null);
  };
  if (draft !== null) {
    return (
      <span className="iv-check">
        <input className="iv-check-input" value={draft} autoFocus spellCheck={false}
          aria-label={`Check for criterion ${serve.criterionId}`} aria-invalid={tooLong}
          onChange={(e) => setDraft(e.target.value)} onKeyDown={onKey} />
        {tooLong ? <span className="iv-check-none" role="alert">Too long for a check (1000 bytes at most)</span> : null}
      </span>
    );
  }
  return (
    <span className="iv-check">
      {serve.check
        ? <code className="iv-fact" data-fact="check">{serve.check}</code>
        : <span className="iv-check-none">No check — closes on a Record</span>}
      {canEdit ? (
        <button type="button" className="iv-check-set" onClick={() => setDraft(serve.check ?? "")}>
          {serve.check ? "Edit check" : "Set check"}
        </button>
      ) : null}
    </span>
  );
}
```

In task 47's `Serves.tsx`, `ServeRow`'s props gain `onSetCheck(check: string): void`, its `<li>`
renders `<CheckLine serve={serve} canEdit={canRemove} onSet={onSetCheck} />` after the two facts,
and `Serves` passes `onSetCheck={(check) => write(serves.map((x) => (x === s ? withCheck(x, check)
: x)))}`. Nothing else in task 47's file changes. The CSS, appended to `issue.css`:

```css
/* Task 23b: the check line. Mono is the command (a machine fact); the words stay sans. */
.iv-serve { flex-wrap: wrap; }   /* task 47's row: the check line takes its own line under the pair */
.iv-check { display: flex; align-items: baseline; gap: var(--s-2); flex-basis: 100%; }
.iv-check-none, .iv-check-set { font-family: var(--sans); color: var(--ink-2); }
.iv-check-input { font-family: var(--mono); font-size: var(--t-fact); inline-size: 100%;
  border-radius: var(--r-sm); border: 0; box-shadow: 0 0 0 1px var(--ring); }
```

#### Steps

- [ ] **Step 1: the `/check/v1/run` route in `sandboxd`, tests first**

In `sandboxd/internal/checkrunner/runner_test.go` (no Docker; the handler is served by
`httptest.NewServer` and the check commands are `true`, `false`, `sleep` and `cat`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRunRefusedWithoutTheCheckRole` | `checkrunner.New("")` and `New("agent")` → 403 `role_not_check`, and no process is started; `New("check")` → 200 | the `sh -c` verb reachable from a lane sandbox an agent is sitting in |
| `TestRunRefusedWithoutTheToken` | the bearer check is the server's existing one, exercised on `/check/v1/` too | a second, weaker door beside `/git/v1/` |
| `TestRunChecksOutTheCommitNotTheTip` | a bundle with two commits and a command that reads a file added at the tip → asked for the parent, the exit is non-zero | a runner that checks out the bundle's ref |
| `TestRunReportsOnlyTheExitCode` | the response's JSON keys are exactly `exitCode`, `timedOut`, `ranAtMs`, for a command that writes to both stdout and stderr | output plumbed back, which a later caller would read (`OD-8`) |
| `TestRunTimeoutIs124AndNotZero` | `sleep 5` with `timeoutMs: 1000` → `{124, timedOut: true}` in under 3 s, and the process group is gone | a timeout that returns 0 and is recorded as proof |
| `TestRunClampsTheTimeout` | `timeoutMs: 0` and `timeoutMs: 86400000` → the 1-second floor and the 30-minute cap, not the caller's number | a check that holds a sandbox for a day |
| `TestRunRejectsASecondRun` | a second POST while one is running → 409; the route is one check per sandbox, ever | a sandbox reused between checks, which is no longer a fresh clone |

Run: `go test ./sandboxd/... -race -count=1 -run 'Run'`. Expected: FAIL, `undefined:
checkrunner.New`. Write the package and the one `HandleFunc`; re-run. Expected: `ok`.

- [ ] **Step 2: `SandboxRunner` in the control plane, tests first**

In `control-plane/internal/evidence/sandboxcheck_test.go`, against a `backend.Backend` stub that
records its `Spec` and an `httptest` stand-in for `sandboxd`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCheckSandboxIsDestroyedOnEveryPath` | exit 0, exit 1, a timeout, a dial failure and a panic in the client → `Destroy` called exactly once in each, with the id `Create` returned | a leaked sandbox, and a `defer` skipped on the error path |
| `TestCheckSpecCarriesNoHostMountAndNoAgent` | the recorded `Spec` passes `backend.ValidateSpec`, and has `Mounts == nil`, `Env` exactly `{"SANDBOXD_ROLE": "check"}`, `Command == "/bin/sleep"`, `Args == ["infinity"]` | a check running beside the agent or reading the host; a role key `ValidateSpec` refuses, so `Create` fails on both backends |
| `TestCheckTokenIsFreshPerRun` | two runs → two different `AttachToken`s, and neither is logged | a shared token that outlives its sandbox |
| `TestCheckUnsupportedIsNotAFailedCheck` | `Sandbox.Endpoint == ""` → `(0, ErrCheckUnsupported)` and, through `Observer.Run`, **no Record** | an unreachable sandbox read as a failing check, which would look like a verdict |
| `TestRunCheckPassesTheClampedTimeout` | `CheckRun.Timeout` of 90 minutes → `timeoutMs` 1800000 on the wire; of zero → 600000 | the cap living only in the sandbox; a zero sent as the 1-second floor, so every product check times out |

Run: `go test ./control-plane/internal/evidence/... -race -count=1 -run 'Check'`. Expected: FAIL,
`undefined: evidence.SandboxRunner`. Write `sandboxcheck.go`; re-run. Expected: `ok`.

- [ ] **Step 3: the check line on the desktop, tests first**

In `desktop/src/components/issue/CheckLine.test.tsx` (vitest and Testing Library, as task 47's
tests; fixtures from `../../test/workFixtures`, `makeServe` taking `check`) and one test appended
to task 47's `Serves.test.tsx`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| shows the command as a mono fact | `makeServe({ check: "npm test -- resume.spec.ts" })` → `code[data-fact="check"]` holds exactly that text; no `check` → *No check — closes on a Record* | a check hidden, or shown in the sans as prose |
| `Enter` writes the trimmed check, `Escape` writes nothing | *Set check*, type `  true  `, `Enter` → `onSet("true")` once; again with `Escape` → no call | an edit that writes on every keystroke, or that `Escape` cannot abandon |
| over 1000 bytes is refused before any write | 334 × `€` (1002 bytes, 334 UTF-16 units) → `Enter` makes no call, `role="alert"` says *Too long for a check*; 333 × `€` (999 bytes) → one call | a `maxLength` or `.length` cap, which lets the Hub 422 a check the field accepted |
| an empty line clears | on a pair with a check, clear the field, `Enter` → `onSet("")`; `withCheck(s, "")` has no `check` key | a cleared check sent as `check: ""` beside one that was never set |
| no control off the head | `canEdit={false}` → no button, the command still shown | an edit offered on an old revision |
| no prediction | `expectNoPrediction(container)` on every render above (task 20) | a *will pass* or a count beside the command |
| (in `Serves.test.tsx`) setting one pair's check keeps the other's | revision with pairs `a` (`check: "x"`) and `b`; set `b` to `y` → one `revise` whose `serves` is `[a with "x", b with "y"]`; removing `b` → `[a with "x"]` | a writer that rebuilds pairs as `{briefKey, criterionId}` and drops every other check |

Run: `npm --prefix desktop test -- CheckLine Serves`. Expected: FAIL on the missing module. Write
`CheckLine.tsx`, the `ServeRow` change and the CSS; re-run. Expected: `Test Files 2 passed`.

- [ ] **Step 4: wire the runner, and turn task 23's fallback off**

Replace task 23 step 4's `Observer{…, Runner: nil}` in `control-plane/cmd/api/main.go`
(`task-23.md:579-581`) with `Runner: &evidence.SandboxRunner{Backend: be, Image: cfg.SandboxImage,
…}`. Then, with a Docker daemon, one end-to-end test in
`control-plane/internal/api/observed_test.go`: a project with one issue serving one criterion
whose `Serve.Check` is `true`, a land, and the head reads `done · proven` with **no Record posted
by anyone**. There is no real-daemon guard in the tree to copy — `dockerbe`'s tests run on a fake
`APIClient` and `cmd/api/fakedaemon_test.go` is a fake daemon (read 2026-09-29) — so the file
carries the build tag `//go:build docker` and runs only with `-tags docker`, on a host with a
Docker daemon and `panacea-app/sandbox:dev` built (`cmd/api/config.go:75`). Name it
`TestObservedExitZeroClosesWithNoClickInTheProduct`: it is the one test that tells the demo's last
frame (05 §5.3 row 7) from a test double.

- [ ] **Step 5: Acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/evidence/... ./control-plane/internal/api/... ./control-plane/internal/backend/... -race -count=1'
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- CheckLine Serves'
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
```

Expected: one `ok` per Go package, no `FAIL`; `go test ./sandboxd/... -race -count=1` prints `ok`
in phase 3's module; the desktop run prints `Test Files 2 passed` (7 tests are this task's, and
task 47's four `Serves` tests stay green) and the typecheck exits 0. Then `go test -tags docker
./control-plane/internal/api/... -race -count=1 -run InTheProduct` on a Docker host prints `ok`;
record that it ran and where — not running it is not a pass, and `OD-27` says done means proven.
The check line's look is the owner's eyes, not a self-report. Commit with `git commit -F <msgfile>
-- <the paths above>`.

#### What this task does not claim

- **Not that a check runs on Kata.** W1's demo is local mode (skeleton `:17-19`, `OD-20`). The
  route is the sandbox's own, so `katabe` needs no code change, but nothing here proves it on a
  Kata pod; phase 8 stream C does. Until it has, a pod the runner cannot reach is a `RunCheck`
  error, which records nothing, so the issue there closes only on a person's Record — never a
  false *observed*. (`ErrCheckUnsupported` is for an empty `Endpoint`; `katabe` reports
  `<podIP>:8080`, `backend.go:299-302`.)
- **Not who may author a check.** The field is `Serve.Check` (task 02) and the authority is SW.3's
  (task 07): a check is set by a team member with the brief's write verb, never carried in by
  intake or by a `text·A` revision. This task runs what the revision holds; it does not judge it.
- **Not a check that needs a secret, a service or another repository**, and not a sandbox without
  a network: it gets the bundle, a lane sandbox's network and nothing else (decision 3). A check
  that cannot pass under that is a check this platform cannot observe, and the issue closes on a
  person's Record instead — `proven_self_attested` in solo mode, which is the honest word.
- **Not more than one command per criterion**, no matrix, no caching, no retry, and no scheduling:
  a check runs when the Observer asks, once.
- **Not a check proposed by an agent.** W3's records (SW.19, task 99) and its proposed evidence
  (task 104) put evidence under a criterion for a person to Record; they write no `Serve.Check`,
  and the check line is the only W1 writer of one (decision 5).
- Design: 05 §3.3, §5.3 row 7. Parity: serves **L-8** with tasks 22 and 23; proof 70. It closes no
  row of its own — it is what makes task 23's claim true in the product.
