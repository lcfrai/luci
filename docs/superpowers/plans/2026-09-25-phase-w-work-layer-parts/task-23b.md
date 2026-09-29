### Task 23b: a criterion's check, end to end — the product `CheckRunner` (one bounded command in a check sandbox, exit code only) and the check line a person sets it on

W1, stream WP. **No new seam**: this task fills the `CheckRunner` interface task 23 declares, and
SW.4's done rule reads the result without changing. About 5 hours (an estimate, not a
measurement): 3 for the runner, 1 for the reaper's hold on a running check, 1 for the check line.
Depends on task 23 (the interface, `Check`, `Observation`, `Observer`, `ErrNoRunner`), task 47 (the
issue view's `Serves.tsx`, whose `ServeRow` carries the line) and **P3-24** (`sandboxd`, the
in-sandbox runner) sync-merged. Write it `dependsOn: 23, 47; P3-24`.

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
   `{exitCode, timedOut}` and there is no field for output, so no later caller can start
   deriving meaning from bytes: the plumbing does not exist.
4. **`Backend` stays seven methods**, so the registry and both backends are untouched (the reaper
   gains one hook, decision 6), and the Kata path (`OD-1`) needs nothing extra when phase 8 stream C lands.

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
landed commit out of the project and hands the bundle to the runner; the runner sends that one file
as the request's body, never inside JSON (P3-24's `fetch` shape, one cap: `MaxBundleBytes`), and the
check sandbox gets nothing else — no credentials, no gateway token, and no host mount but the one
every lane sandbox gets: the operator's CA bundle (`PANACEA_APP_CA_BUNDLE`), read-only, at the path
and under the variable `sessionMounts` and `sessionEnv` use (`api/sessions.go:296`, `:341-343`,
`:434-436`), and only when one is set, so a check on a TLS-intercepting network trusts what a lane
trusts. The network is any sandbox's, not less: `backend.Spec` has no network field and `dockerbe`
uses the default bridge, and cutting it would change `Backend` (decision 1). So a check reaches what
a lane sandbox reaches and holds no credential to use there; a check that needs a secret or a
service to pass cannot run here, and that is stated, not worked around.

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
3. **No new letter key, and no list key in the field.** The control is a button in the row's Tab
   order and the field takes only `Enter` and `Escape`; task 47's `T` skips a key typed in a field
   (task 41's `isBareListKey`), and a letter bound here would compete with it and tasks 53-54's map.
4. **Who may set one is SW.3's rule, not this line's**: the line is offered on the head to the
   person viewing it, and the Hub's revision write enforces who may add or change a check.

**Decision 6: a running check's sandbox is held against the reaper, and only while it runs.**
Phase 2's reaper, under its rule 4, destroys any sandbox stamped with this instance's
`LabelInstance` whose `LabelSandboxID` no registry session holds, once it is older than
`CreateGrace` (`reaper/reaper.go:471-527`; `dockerbe` stamps both labels, `docker.go:290-291`).
`CreateGrace` is `CreateGraceFor(cfg.ReadyTimeout)` (`cmd/api/main.go:246`), twice the 60-second
ready timeout by default (`reaper.go:102-106`, `backend/dial.go:24`), and a sweep runs every 30 s
(`cmd/api/config.go:78`). A check sandbox has no registry record, and must not have one (it is
not a session, and a record would put it on the session rail), so without a hold every check that
runs past about two minutes is destroyed mid-run: `RunCheck` errors, nothing is recorded, and the
issue closes only on a Record — S03-C1's failure again, for every real check. So the reaper gains
`Hold(sandboxID, max) (release func())`, and rule 4 skips an id that is held. The runner takes the
hold **before** `Create` and releases it after its deferred `Destroy` returns, on every path, with
`max` = `MaxCheckHold`, 35 minutes (task 23's per-run `AfterLand` bound): a stuck run cannot pin a
sandbox past it. The set is in memory, so after a crash or a restart nothing is held and a leftover
check sandbox is reaped exactly as before. The runner **refuses to run without a hold**
(`ErrNoHold`, before any `Create`), so a wiring slip is a logged error that records nothing, never
a sandbox reaped mid-check. `reaper.go` is a built phase 2 file: a ⚑ delta, like `cmd/api/main.go`.

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
  checkrunner.New(os.Getenv("SANDBOXD_ROLE"))`, read once at start, and one `/check/v1/` route
  beside `/git/v1/` and `/attach` at `server.go:85`, **wrapped** in the bearer check: sandboxd
  authorises inside each handler (`authorised`, `:105`; `attach` calls it first, `:167`), not on
  the mux, so a bare `HandleFunc("/check/v1/", s.check)` would be open (modified)
- `sandboxd/internal/server/server_test.go` ⚑ — `TestCheckRouteRefusedWithoutTheToken` (modified)
- `control-plane/internal/evidence/sandboxcheck.go` + `sandboxcheck_test.go` — `SandboxRunner`,
  which satisfies task 23's `CheckRunner` (`task-23.md:355-357`) (new)
- `control-plane/internal/reaper/reaper.go` ⚑ + `reaper_test.go` ⚑ (phase 2's) — `Reaper.Hold`,
  and rule 4 skips a held id (decision 6) (modified)
- `control-plane/cmd/api/main.go` — task 23 step 4's `Runner: nil` becomes the real runner
  (`task-23.md:579-581`), holding through the one reaper `serve` builds (`main.go:102`) (modified)
- `desktop/src/components/issue/CheckLine.tsx` + `CheckLine.test.tsx` — the check line, `MAX_CHECK_BYTES`,
  `checkBytes`, `withCheck` (new)
- `desktop/src/components/issue/Serves.tsx` ⚑ + `Serves.test.tsx` ⚑ (task 47's) — `ServeRow` gains
  `onSetCheck` and renders `CheckLine`; two tests: other pairs keep their checks, a typed `t` stays (modified)
- `desktop/src/components/issue/issue.css` ⚑ (task 47 appends to it) — `.iv-serve` wraps, and three
  `.iv-check*` rules (modified)

**Interfaces:**
- Produces: `evidence.SandboxRunner`, `ErrCheckUnsupported`, `ErrNoHold`, `MaxCheckHold`; `sandboxd`'s
  `/check/v1/run` route and its two wire types, consumed by task 23's `Observer` and by nothing else
  in W1; `reaper.Reaper.Hold`; the desktop's `CheckLine`, the one writer of `Serve.Check` in W1
- Consumes: task 23 — `evidence.CheckRun`, `CheckRunner`, `Observer.Runner`; phase 1 —
  `backend.Backend` (`Create`, `Get`, `Destroy`), `backend.Spec`, `backend.Sandbox.Endpoint`,
  `backend.Resources`; phase 2 — `reaper.Reaper` (gains `Hold`), `ids.New`, `ids.Token`,
  `Config.CABundle`; P3-24 — `sandboxd`'s server, its bearer auth and its bundle handling;
  task 47 — `Serves`, `ServeRow`, its `write`; task 37 — `Serve` with `check?: string`; task 56 —
  `mutate`'s `revise` op; task 20 — `makeServe`, `expectNoPrediction`
- Amends: no seam. No wire field is added to SW.1-SW.21 and no golden vector moves; phase 2's
  `reaper.Reaper` gains one method, `Hold`, and rule 4 skips a held id; with nothing held it is phase 2's

#### Interfaces — complete

```go
// sandboxd/internal/checkrunner/wire.go — POST /check/v1/run. RunRequest travels as headers,
// since the body is the bundle (P3-24's fetch shape, P3 task-24.md:176-178): X-Panacea-Ref,
// X-Panacea-Commit, X-Panacea-Timeout-Ms, and X-Panacea-Command, the command's UTF-8 bytes in
// standard base64, because a header value is not UTF-8-safe. The body, Content-Type
// application/x-git-bundle, is read through http.MaxBytesReader(MaxBundleBytes) into a spool
// file; past the cap the answer is 413 too_large and nothing runs. A header missing, repeated
// or malformed is 400. The bundle is the ONLY input. The goldens spell RunRequest as this JSON.
type RunRequest struct {
	Ref       string `json:"ref"`       // the ref the bundle holds
	Commit    string `json:"commit"`    // 40 hex; checked out detached
	Command   string `json:"command"`   // run as `sh -c Command`; 1 to 1000 bytes once decoded
	TimeoutMS int64  `json:"timeoutMs"` // clamped to [1000, 1800000] here, not trusted
}

// RunResponse has no field for output, and that is the point: nothing downstream can start
// deriving meaning from the process's bytes, because the bytes never leave the sandbox (OD-8).
// Nor a clock: the Observer stamps Observation.RanAt from its own (task-23.md:333-339).
type RunResponse struct {
	ExitCode int  `json:"exitCode"` // 124 when the timeout fired (coreutils' convention)
	TimedOut bool `json:"timedOut"`
}

// limits.go. One cap for any bundle entering a sandbox: P3-24's, not a second number.
const MaxBundleBytes = gitrunner.MaxBundleBytes // 1 << 30 (P3 task-24.md:349); UNVERIFIED until merge
```

```go
// sandboxd/internal/server/server.go, in New, beside /attach (:85). Server gains
// `check http.Handler`. The route carries its own bearer check, as attach does (:167).
s.check = checkrunner.New(os.Getenv("SANDBOXD_ROLE")) // read once, at start
s.mux.HandleFunc("/check/v1/", func(w http.ResponseWriter, r *http.Request) {
	if !s.authorised(r) {
		slog.Warn("rejected unauthenticated check", "remote", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.check.ServeHTTP(w, r)
})
```

```go
// control-plane/internal/evidence/sandboxcheck.go
package evidence

// SandboxRunner runs one check in a sandbox of its own: created from the project's image with
// Env {"SANDBOXD_ROLE": "check"}, Command "/bin/sleep", Args ["infinity"] and no agent process,
// given the bundle and nothing else, destroyed on every path. With CABundle set it adds what a
// lane sandbox gets and nothing more: one read-only Mount of it at /etc/panacea-app/ca-bundle.pem
// and NODE_EXTRA_CA_CERTS naming that path (api/sessions.go:296, :341-343, :434-436). The
// timeout sent is CheckRun's, zero as 10 minutes and anything above 30 as 30 (task 23's
// Check.Timeout rule). It is the product CheckRunner; task 23's hostRunner stays a test double.
type SandboxRunner struct {
	Backend   backend.Backend
	Image     string            // the check image; the project's sandbox image by default
	Resources backend.Resources // ValidateSpec refuses a zero (backend.go:280-282)
	NewID     func() string     // sandbox ids; must satisfy backend.Spec.ID's pattern
	Token     func() string     // a fresh attach token per check, never reused
	CABundle  string            // the host path of PANACEA_APP_CA_BUNDLE; "" means none
	// Hold is the reaper's (reaper.Reaper.Hold, decision 6): taken before Create, released after
	// Destroy. nil refuses every run with ErrNoHold, before Create.
	Hold func(sandboxID string, max time.Duration) (release func())
	Dial func(ctx context.Context, endpoint string) *http.Client // nil means the default
	Now  func() time.Time                                          // nil means time.Now
}

// MaxCheckHold bounds a hold: task 23's per-run AfterLand bound, the 30-minute cap plus the
// sandbox's create and destroy. A run still going past it has its sandbox reaped.
const MaxCheckHold = 35 * time.Minute

var ErrNoHold = errors.New("evidence: no reaper hold is wired, so a check sandbox would be reaped mid-run")

// RunCheck satisfies CheckRunner. A timeout is (124, nil); a runner that could not run at all
// is (0, err) — including ErrCheckUnsupported, which a backend with no reachable endpoint
// returns so the Observer records nothing rather than guessing.
func (r *SandboxRunner) RunCheck(ctx context.Context, run CheckRun) (exitCode int, err error)

var ErrCheckUnsupported = errors.New("evidence: this backend cannot run a check")
```

```go
// control-plane/internal/reaper/reaper.go ⚑ (phase 2's). Reaper gains holdMu sync.Mutex and
// held map[string]time.Time (sandbox id → when its hold lapses), made in New.

// Hold keeps rule 4 off one sandbox id until release is called or max has passed, whichever is
// first. A caller that creates a sandbox no registry record names holds it from before
// backend.Create until its Destroy returns. The set is in memory: a restart holds nothing.
// release is idempotent.
func (r *Reaper) Hold(sandboxID string, max time.Duration) (release func())

// Rule 4, after the tracked check (reaper.go:501-506), before the grace:
//	if id != "" && r.held(id, now) { continue } // a check in flight (task 23b), not an orphan
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
  const pair: Serve = { briefKey: s.briefKey, criterionId: s.criterionId };  // only the pair being set
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
: x)))}`. Nothing else in 47's file changes (its `T` already skips fields). The CSS, appended to `issue.css`:

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
| `TestRunTakesTheBundleAsTheBody` | the bundle is the raw body and the rest is four headers; with `MaxBundleBytes` at 1 KiB (a `newWithLimits` seam, as P3-24's), a 2 KiB body → 413 `too_large` and no process starts; a missing `X-Panacea-Commit` or a command header that is not base64 → 400 | a bundle inside JSON, where `MaxJSONBytes` (8 MiB) refuses a real repository's history or the whole bundle sits in memory |
| `TestRunChecksOutTheCommitNotTheTip` | a bundle with two commits and a command that reads a file added at the tip → asked for the parent, the exit is non-zero | a runner that checks out the bundle's ref |
| `TestRunReportsOnlyTheExitCode` | the response's JSON keys are exactly `exitCode` and `timedOut`, for a command that writes to both stdout and stderr | output plumbed back, which a later caller would read (`OD-8`) |
| `TestRunTimeoutIs124AndNotZero` | `sleep 5` with `X-Panacea-Timeout-Ms: 1000` → `{124, timedOut: true}` in under 3 s, and the process group is gone | a timeout that returns 0 and is recorded as proof |
| `TestRunClampsTheTimeout` | a timeout header of `0` and of `86400000` → the 1-second floor and the 30-minute cap, not the caller's number | a check that holds a sandbox for a day |
| `TestRunRejectsASecondRun` | a second POST while one is running → 409; the route is one check per sandbox, ever | a sandbox reused between checks, which is no longer a fresh clone |

And in `sandboxd/internal/server/server_test.go` ⚑, because `checkrunner`'s own `httptest` rig
cannot see `server.go`: `TestCheckRouteRefusedWithoutTheToken` — `New` with an attach token and
`SANDBOXD_ROLE=check` (`t.Setenv`): `POST /check/v1/run` with no `Authorization`, or a wrong bearer,
→ 401 and no check starts; with the right bearer → not 401. It kills the route mounted bare, a
second, weaker door beside `/git/v1/`.

Run: `go test ./sandboxd/... -race -count=1 -run 'Run|CheckRoute'`. Expected: FAIL, `undefined:
checkrunner.New`. Write the package and the wrapped route; re-run. Expected: `ok`.

- [ ] **Step 2: `SandboxRunner` in the control plane, tests first**

In `control-plane/internal/evidence/sandboxcheck_test.go`, against a `backend.Backend` stub that
records its `Spec` and an `httptest` stand-in for `sandboxd`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCheckSandboxIsDestroyedOnEveryPath` | exit 0, exit 1, a timeout, a dial failure and a panic in the client → `Destroy` called exactly once in each, with the id `Create` returned | a leaked sandbox, and a `defer` skipped on the error path |
| `TestCheckSpecCarriesNoAgentAndOnlyTheCABundle` | the recorded `Spec` passes `backend.ValidateSpec`, and has `Command == "/bin/sleep"`, `Args == ["infinity"]`, the runner's `Resources`; with no `CABundle`, `Mounts == nil` and `Env` exactly `{"SANDBOXD_ROLE": "check"}`; with one, exactly one `Mount`, read-only at `/etc/panacea-app/ca-bundle.pem`, and `NODE_EXTRA_CA_CERTS` naming it | a check beside the agent or reading the host; a role key `ValidateSpec` refuses; a check on an intercepting network that cannot fetch what a lane can |
| `TestCheckTokenIsFreshPerRun` | two runs → two different `AttachToken`s, and neither is logged | a shared token that outlives its sandbox |
| `TestCheckUnsupportedIsNotAFailedCheck` | `Sandbox.Endpoint == ""` → `(0, ErrCheckUnsupported)` and, through `Observer.Run`, **no Record** | an unreachable sandbox read as a failing check, which would look like a verdict |
| `TestRunCheckPassesTheClampedTimeout` | `CheckRun.Timeout` of 90 minutes → `X-Panacea-Timeout-Ms: 1800000`; of zero → 600000 | the cap living only in the sandbox; a zero sent as the 1-second floor, so every product check times out |
| `TestRunCheckSpeaksTheWireShape` | the stand-in sees one `POST /check/v1/run` with the bearer, the four headers (the command header decodes to the command's bytes) and a body byte-equal to `CheckRun.Bundle`'s file; its 413 → `(0, err)` | a host and a sandbox that disagree on the shape, which only a Docker run would show |
| `TestCheckSandboxIsHeldFromBeforeCreateUntilDestroy` | a recording `Hold`: taken with `MaxCheckHold` before `Create` sees the id, released only after `Destroy` returns, once, on exit 0, an error and a panic; a nil `Hold` → `(0, ErrNoHold)` and no `Create` | a check sandbox the reaper destroys mid-run (decision 6); a hold that leaks; a runner wired without one |

Run: `go test ./control-plane/internal/evidence/... -race -count=1 -run 'Check'`. Expected: FAIL,
`undefined: evidence.SandboxRunner`. Write `sandboxcheck.go`; re-run. Expected: `ok`.

Then the hold, in phase 2's `control-plane/internal/reaper/reaper_test.go` ⚑, beside
`TestSweepSparesAnOrphanYoungEnoughToBeMidCreate` (`:493`) and on its fake backend and clock:
`TestSweepSparesAHeldSandboxUntilReleaseOrItsBound` — a sandbox of this instance, sandbox id
`chk-1`, no record, created 10 minutes ago (past `CreateGrace`): held for 35 minutes, a sweep
leaves it; after `release`, the next sweep destroys it (*"no session record"*); held for 1 minute
with the clock moved 2, destroyed; a new `Reaper` (a restart) holds nothing, destroyed. It kills
rule 4 ignoring the hold, a hold that never lapses, and a hold that survives a restart. Run:
`go test ./control-plane/internal/reaper/... -race -count=1`. Expected: FAIL, `r.Hold undefined`;
write `Hold` and the rule 4 line; re-run. Expected: `ok`, every phase 2 reaper test still green.

- [ ] **Step 3: the check line on the desktop, tests first**

In `desktop/src/components/issue/CheckLine.test.tsx` (vitest and Testing Library, as task 47's
tests; fixtures from `../../test/workFixtures`, `makeServe` taking `check`) and two tests appended
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
| (in `Serves.test.tsx`) a check typed in the list stays in the line | head revision with one pair; *Set check*, then user-event's `type` puts `npm test` in the field → it reads `npm test`, and no *Choose a criterion* dialog opens | task 47's `onKey` without task 41's `isBareListKey`: the first `t` is swallowed and the picker opens |

Run: `npm --prefix desktop test -- CheckLine Serves`. Expected: FAIL on the missing module. Write
`CheckLine.tsx`, the `ServeRow` change and the CSS; re-run. Expected: `Test Files 2 passed`.

- [ ] **Step 4: wire the runner, and turn task 23's fallback off**

Replace task 23 step 4's `Observer{…, Runner: nil}` in `control-plane/cmd/api/main.go`
(`task-23.md:579-581`) with the real runner. `serve` builds it after the reaper (`main.go:102`),
whose `Hold` it takes, and every field has a named source, because `ValidateSpec` refuses a zero
`Resources` (`backend.go:280-282`) and `Create` needs an id and a token:

```go
Runner: &evidence.SandboxRunner{
	Backend: be, Image: cfg.SandboxImage, Resources: cfg.Resources(), // as api.Deps gets them (main.go:149-155)
	NewID: func() string { return ids.New("chk") }, Token: ids.Token, // as sessions mint theirs (sessions.go:869-870)
	CABundle: cfg.CABundle,                                           // as api.Deps.CABundle (main.go:164)
	Hold: rp.Hold,                                                    // decision 6
},
```

`ids.New("chk")` is `chk-` and 8 hex characters, inside `Spec.ID`'s pattern (`backend.go:182`),
so the reaper's log names a check sandbox as one. A `serve` that forgets `Hold` runs no check at
all (`ErrNoHold`, logged), never one reaped mid-run. Then, with a Docker daemon, end-to-end tests in
`control-plane/internal/api/observed_test.go`: a project with one issue serving one criterion
whose `Serve.Check` is `true`, a land, and the head reads `done · proven` with **no Record posted
by anyone**. There is no real-daemon guard in the tree to copy — `dockerbe`'s tests run on a fake
`APIClient` and `cmd/api/fakedaemon_test.go` is a fake daemon (read 2026-09-29) — so the file
carries the build tag `//go:build docker` and runs only with `-tags docker`, on a host with a
Docker daemon and `panacea-app/sandbox:dev` built (`cmd/api/config.go:75`). Name it
`TestObservedExitZeroClosesWithNoClickInTheProduct`: it is the one test that tells the demo's last
frame (05 §5.3 row 7) from a test double. Its twin, `TestObservedCheckOutlivesTheReaperInTheProduct`,
runs a check of `sleep 8` beside a real `reaper.New` (this instance, `CreateGrace` 2 s) whose
`Sweep` the test calls every second: the sandbox outlives six sweeps past its grace, the head
reads `done · proven`, and no *"destroyed an orphan sandbox"* line names a `chk-` id.

- [ ] **Step 5: Acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/evidence/... ./control-plane/internal/api/... ./control-plane/internal/backend/... ./control-plane/internal/reaper/... ./control-plane/cmd/api/... -race -count=1'
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- CheckLine Serves'
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
```

Expected: one `ok` per Go package, no `FAIL`; `go test ./sandboxd/... -race -count=1` prints `ok`
in phase 3's module; the desktop run prints `Test Files 2 passed` (8 tests are this task's, and
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
  `<podIP>:8080`, `backend.go:299-302`. With a CA bundle set, `katabe` refuses the mount with
  `ErrUnsupported`, `backend.go:168-169`, as it refuses a lane's: phase 8 provides roots another way.)
- **Not who may add or change a check.** That is SW.3's rule (task 07's addendum): only a
  person's own revision, never intake or a `text·A` one. Task 02's `ValidateCheckChange` decides
  it and every revision write calls it (the fake, task 04; the Hub, tasks 08 and 09; row W-02.1), so
  the check line needs no rule of its own and this task runs only what the Hub accepted.
- **Not a check that needs a secret, a service or another repository**, and not a sandbox without
  a network: it gets the bundle, a lane sandbox's network and CA bundle, and nothing else (decision 3). A check
  that cannot pass under that is a check this platform cannot observe, and the issue closes on a
  person's Record instead — `proven_self_attested` in solo mode, which is the honest word.
- **Not more than one command per criterion**, no matrix, no caching, no retry, and no scheduling:
  a check runs when the Observer asks, once.
- **Not a check proposed by an agent.** W3's records (SW.19, task 99) and its proposed evidence
  (task 104) put evidence under a criterion for a person to Record; they write no `Serve.Check`,
  and the check line is the only W1 writer of one (decision 5).
- Design: 05 §3.3, §5.3 row 7. Parity: serves **L-8** with tasks 22 and 23; proof 70. It closes no
  row of its own — it is what makes task 23's claim true in the product.
