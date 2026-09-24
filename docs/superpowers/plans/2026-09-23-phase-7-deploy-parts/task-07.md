### Task 07: The watcher — one process polls Panacea, the desktop hears a cursor

**Half:** 7a. **Hours:** 3. **Depends on:** 03, 06. **Elsewhere:** Hub H0 (`hubclient.Releases`
fake); Hub C2's cursor mechanism (ask HA7-4 — this part says what to do if C2 has not built it).

Plan decision 7P-5. The desktop's one-clock rule (`poller.ts:90`, `hooks/heartbeat.ts:34` — the only
two `setInterval` sites) is about the **renderer**. The control plane is a different process and
already runs its own timers (the reaper, `PANACEA_APP_REAP_INTERVAL`; the Hub's change-feed poll,
Hub contract §9.3). The watcher is one more, and it is the only thing that polls Panacea.

**Files:**
- `control-plane/internal/deploy/deploywatch/watcher.go` (new) — `Config`, `Watcher`, `Snapshot`, the cycle
- `control-plane/internal/deploy/deploywatch/join.go` (new) — the deployment join
- `control-plane/internal/deploy/deploywatch/watcher_test.go`, `join_test.go` (new)
- `control-plane/internal/api/api.go` (modified) — `Deps.CursorSources`, **only if Hub C2 has not added it** (HA7-4)
- `control-plane/internal/api/sessions.go` (modified, same condition) — the header loop on `GET /v1/sessions`
- `control-plane/internal/httpx/origin.go` (modified, same condition) — `Access-Control-Expose-Headers` only. The preflight's `corsMethods` and `corsHeaders` are task 05's change (review C1), and this task leaves them alone.
- C2's change poller (modified, **only if it has no listener**) — the `OnEvents` field (review M12)
- `control-plane/cmd/api/config.go`, `main.go` (modified) — three interval variables and the cycle deadline (plan §6); start `Run`; `watcher.HubEvents` on the change poller

#### Interfaces — complete

```go
// Package deploywatch polls the deploy target for every bound local project, holds the newest
// reading in memory, writes the Hub deployment join, and exposes a cursor that changes when any
// reading does. It persists nothing (plan 7P-13).
package deploywatch

// Releases is the subset of hubclient.Releases (Hub contract §9.2) the watcher uses.
type Releases interface {
	Releases(ctx context.Context, projectID string, q hubapi.ReleaseQuery) (hubapi.Page[hubapi.Release], error)
	JoinDeployment(ctx context.Context, releaseID, deploymentID string) (hubapi.DeploymentJoin, error)
}

// Me names the signed-in Hub principal, so the watcher joins only releases this user reserved.
type Me interface {
	Me(ctx context.Context) (hubapi.Me, error) // hubclient.Projects.Me
}

type Bindings interface {
	Resolve(ctx context.Context, projectID string) (binding.Binding, error)
	Cached(projectID string) (binding.Binding, bool)
}

type Connection interface {
	Observe(err error)
	Status() connection.Status
	Check(ctx context.Context) connection.Status
}

type Config struct {
	Deployer   deploy.Reader
	Governor   deploy.Governor // for the owner probe only (7P-9)
	Projects   binding.LocalProjects
	Bindings   Bindings
	Connection Connection
	Releases   Releases // nil = no Hub configured: Snapshot.ReleasesState "no_hub"
	Me         Me
	// Interval, ActiveInterval, HistoryInterval: plan §6. Zero = 15s, 3s, 60s.
	Interval, ActiveInterval, HistoryInterval time.Duration
	// CycleDeadline bounds one whole cycle, every project's reads together (review M14). Zero =
	// 15s: one per-attempt timeout (PANACEA_APP_PANACEA_TIMEOUT), so a hung GET costs one
	// attempt, never do's three (t02:95, about 46 s).
	CycleDeadline time.Duration
	Now func() time.Time
	Log *slog.Logger
}

type Watcher struct{ /* unexported */ }

func New(cfg Config) (*Watcher, error)

// Run blocks until ctx is done. ONE goroutine, one time.Ticker at ActiveInterval; each project
// is polled when it is due (Interval, or ActiveInterval while it has a tracked run). Each cycle
// runs under context.WithTimeout(ctx, CycleDeadline) — see "The per-cycle deadline" below.
func (w *Watcher) Run(ctx context.Context)

// Snapshot is the newest reading for one local project; ok false before the first cycle.
func (w *Watcher) Snapshot(projectID string) (Snapshot, bool)

// Track makes a run this user started (Ship, rebuild) drive the active cadence until its
// build is terminal. Nudge polls a project on the next tick. Both are called by task 09/10
// right after an act; neither makes a request itself.
func (w *Watcher) Track(projectID string, run string)
func (w *Watcher) Nudge(projectID string)

// HubEvents is the watcher's side of the Hub change poller (Hub task C2; Hub contract §9.3,
// :1377-1385 at 6b873d0: the control plane reads /v1/changes every PANACEA_APP_HUB_POLL_INTERVAL,
// 5 s, while signed in). The poller hands it each page's events; it makes no request. Every event
// whose kind starts "release." (§8, :1155-1159: reserved, confirmed, abandoned, expired,
// deployment_joined) and whose projectId is a bound local project's Binding.HubProjectID marks
// that project's releases due (step 6) and Nudges it — so a teammate's rollback is on the lens
// within one poll plus one tick (about 8 s), not after HistoryInterval (60 s) (review M12). Other
// kinds, and a projectId no local project is bound to, are ignored.
func (w *Watcher) HubEvents(events []hubapi.Event)

// WantBranch asks for a git branch's environment name to be resolved (Reader.EnvironmentFor)
// and kept in Snapshot.BranchEnvs. Task 08's route calls it for the lens's selected branch; it
// makes no request itself and Nudges the project when the branch is new. At most 16 branches
// per project are kept, the least recently wanted dropped first.
func (w *Watcher) WantBranch(projectID, branch string)

// SameEnvironment is its Reader's SameEnvironment, unchanged. Task 08's route and task 19's
// route pass it to deploywatch.Assemble, which cannot import deploy/panacea (b02 round 2, R4).
func (w *Watcher) SameEnvironment(a, b string) bool

// CursorName and Cursor implement api.CursorSource (HA7-4). Cursor is a decimal uint64 that
// increases whenever ANY project's Snapshot.Version does; "" before the first cycle.
func (w *Watcher) CursorName() string // "Deploy"
func (w *Watcher) Cursor() string

// Snapshot is everything task 08's view assembles from. Deploy data is held only here, in memory.
type Snapshot struct {
	ProjectID     string
	Binding       binding.Binding
	Environments  []deploy.Environment
	// BranchEnvs is git branch → the target's environment name for it (EnvironmentFor), for the
	// wanted branches only. Absent = not resolved yet. The name may match no row: a branch never
	// deployed still has one. Joined to Environments with SameEnvironment, never by comparing
	// the branch with Environment.Name or .Branch (review C2).
	BranchEnvs    map[string]string
	// BranchEnvErrs is git branch → Panacea's refusal of EnvironmentFor for it: KindNotFound,
	// KindRejected or KindForbidden (b02 round 2, R2). A branch is in BranchEnvs or here, never
	// both. It does not stale the snapshot: the refusal is about one name, not about Panacea.
	BranchEnvErrs map[string]*deploy.Error
	Latest        *deploy.Deployment // nil = never deployed, or not yet read
	Deployments   []deploy.Deployment
	DeploymentsAt time.Time
	Runs          map[string]deploy.RunProgress // tracked runs only
	Scans         map[string]deploy.ScanReport  // by deployment id; newest deployment per environment only
	Releases      []hubapi.Release              // newest reservedAt first, at most 200: the first page only
	ReleasesMore  bool                          // that page's next was non-null: older releases exist and are not in Releases (review M11)
	ReleasesAt    time.Time                     // the last successful Releases read for this binding; zero = never, so no pin can be known yet
	// PinsByRun is deployment id → the confirmed run release whose pipelineRun is that id, from a
	// targeted Releases(…, {PipelineRun: id}) read (step 6), made only while ReleasesMore. A nil
	// value is the Hub's answer "none"; an absent id is "not read yet". Cached for the binding's
	// life: a deployment old enough to fall off the page had its release confirmed long before.
	PinsByRun     map[string]*hubapi.Release
	ReleasesState string                        // "ok" | "no_hub" | "no_hub_project" | "hub_key_spelling" | "signed_out" | "unavailable"
	OwnerControls string                        // "yes" | "no" | "unknown" (7P-9)
	FetchedAt     time.Time                     // the last cycle in which every Panacea read succeeded
	Stale         bool                          // the last cycle's Panacea reads failed; data is FetchedAt old
	LastError     *deploy.Error                 // that failure, for the lens's sentence
	Version       uint64                        // increases when anything above except FetchedAt changes
}
```

#### One cycle, for one bound project — the order is load-bearing

0. **Connection gate** (once per cycle, before any project). `Connection.Status().State` is
   `NotConnected` → poll nothing. `AwaitingApproval` or `TokenRejected` → poll nothing, and every
   fourth cycle (60 s) call `Connection.Check`, so the lens's *"this screen checks again on its
   own"* (prototype `phase-7-deploy.html:830-831`) is true without a desktop timer.
1. **Binding.** `Bindings.Resolve` when the cached binding is not `Bound`, when step 2 marked the
   project for re-resolve, or every fourth cycle; else `Cached`. Not bound → the snapshot carries
   the binding and nothing else; stop. A `Binding.Panacea` different from the snapshot's resets its
   Panacea data before step 2 (review M13): one project's rows never appear under another's
   binding.
2. **`Environments`.** First, because a missing *project* surfaces here as its own error, which is
   what makes task 03's *"`Latest` 404 means never deployed"* safe.
   **`Environments` refused** (review M13): `KindNotFound` (the project was deleted or renamed),
   `KindForbidden` (this user's access, or the key's scope, was taken away), or any other kind but
   the failure rule's two (`KindUnavailable`, `KindUnauthorized`), is about this binding, not about Panacea, so it is **not** a cycle failure: the other projects are
   still read this cycle. This project's steps 3-8 are skipped. Its snapshot keeps its last
   Panacea data only as history — `Stale: true`, `LastError` the refusal (Panacea's sentence),
   `FetchedAt` unchanged — so the lens never shows it as current (task 08's `staleReason` carries
   the code `writeDeployError` gives the kind, t05:281-284: `deploy_not_found`, `deploy_forbidden`,
   `deploy_conflict` or `deploy_rejected`). The project is marked for
   re-resolve and Nudged, so the next cycle (3 s later, not 15 s) runs step 1's `Resolve`, whatever
   the every-fourth-cycle count says. Then: not `Bound` → step 1's rule, the snapshot carries the
   binding and nothing else, and the old project's data is **dropped**, not kept stale; `Bound` to a
   **different** Panacea project → step 1's reset (`Environments`, `BranchEnvs`, `BranchEnvErrs`,
   `Latest`, `Deployments`, `Runs`, `Scans`, `Releases`, `PinsByRun` emptied, `ReleasesAt` zero;
   `OwnerControls "unknown"` until the
   probe re-runs); still `Bound` to the same one
   (the list shows it, the read refuses) → it stays `Stale` with the refusal, re-resolved every
   fourth cycle, and a later good `Environments` clears it.
   **Then `EnvironmentFor`** for each wanted branch (`WantBranch`) in neither `BranchEnvs` nor
   `BranchEnvErrs`, and for every wanted branch every fourth cycle — a changed production-branch
   label remaps `main` (`mapBranchToEnvWithProd`, `projects.go:3838-3849`). One request per new
   branch, not per cycle. `KindUnavailable` and `KindUnauthorized` follow the failure rule below,
   and a failed re-read keeps the old answer. **Any other failure** (`KindNotFound`,
   `KindRejected`, `KindForbidden`) is about this one name: it goes into `BranchEnvErrs[branch]`
   (and out of `BranchEnvs`), does not stale the snapshot or end the cycle, and is retried only on
   the every-fourth-cycle cadence — so a branch the route cannot carry (a `%2F` the ingress
   decodes, task 03's 404 rewrite) costs one request a minute, not one per cycle (b02 round 2,
   R2). A later success moves it back into `BranchEnvs`.
3. **`Latest`.**
4. **`Deployments`** — only when (a) `Latest.ID` or `Latest.Build` changed, (b) the environment
   list changed (names, statuses, replica pairs), (c) a tracked run became terminal, or (d)
   `HistoryInterval` passed. It is unpaginated and runs three enrichment passes including a dozen
   PipelineRun reads (`projects.go:1227-1253`), so it is never read every cycle.
5. **`Run`** for each tracked run whose build is not terminal. Terminal → untracked.
   **`Scans(ctx, p, d.Environment, d.ID)`** for each environment's newest deployment `d` — the
   deployment's own `Environment`, verbatim, never the card's name and never empty for a branch
   row, because Panacea reads a missing `env` as production (review C3) — only when that
   deployment's `ID` or `Build` changed since the last read, cached by deployment id in
   `Snapshot.Scans` — so the environment strip's third fact (scans) costs one request per build,
   not per cycle. A deployment whose `Environment` is `""` (legacy only, from source: task 12's
   note, re-review b03 M-1) is **not** read: `Snapshot.Scans[d.ID]` stays unset and the fact reads
   `not_read` (task 08 rule 4), never production's verdict.
   **A `Scans` failure** (re-review b03, M-2): `KindUnavailable` and `KindUnauthorized` follow the
   failure rule below. **Any other kind** (`KindNotFound` from the Fake's unseeded pair,
   `KindRejected` for Panacea's 400, `KindForbidden` for a token the project refuses) leaves `Snapshot.Scans[d.ID]` unset, so
   the fact reads `not_read`, does **not** set `Stale` or `LastError`, and does not end the cycle.
   The read is not marked done, so it is retried on the next cycle in which the deployment's `ID`
   or `Build` changes or `HistoryInterval` passes. Panacea's handler itself has no 404 path — it
   answers 400 for an empty project name and 200 otherwise (`handlers/security_status.go:204-254`)
   — so live this path is taken only by a refusal, never by an unseeded or mangled `env` (that
   gets a 200 with `NOT_RUN` or empty rows).
6. **Hub releases** — `Releases(HubProjectID, {PanaceaProject: <ref>, Limit: 200})` when the
   Deployments were re-read, a tracked run changed, a `release.*` event for this Hub project
   arrived through `HubEvents` since the last read (review M12), or `HistoryInterval` passed.
   `Binding.HubProjectID == ""` → by `Binding.HubMiss` (task 06): `"not_linked"` →
   `ReleasesState "no_hub_project"`; `"key_spelling"` → `"hub_key_spelling"` (HA7-6, review I6);
   `""` → `"unavailable"`, because a configured Hub has not yet answered for this key (plan
   re-review b05 R-4; `Releases == nil` is `"no_hub"` before this step); `entraauth.ErrSignedOut` →
   `"signed_out"`; `hubclient.ErrUnavailable` → `"unavailable"`, **keeping the previous releases**.
   A good read sets `ReleasesAt` and `ReleasesMore = page.Next != nil` (Hub contract §1 *Lists*,
   `:48-49` at 6b873d0: `limit` at most 200, `{"items","next"}`).
   **Beyond the page** (review M11). The Hub returns the 200 newest releases, runs and rollbacks
   together, so a project with more has deployments whose release is not in `Releases`, and the
   view would call a real pin absent. While `ReleasesMore`, for each deployment in `Deployments`
   that matches no release in the page by task 08 rule 5 and has no `PinsByRun` entry, newest
   first and **at most 20 per project per cycle**, the watcher reads
   `Releases(HubProjectID, {PanaceaProject: <ref>, PipelineRun: d.ID, State: "confirmed", Limit: 1})`
   (§7.9 lists `pipelineRun=`; the review's option) and stores the `kind "run"` item, or `nil` for
   none, in `PinsByRun[d.ID]`. `pipelineRun=` and not `deploymentId=`: the join is written only by
   the reserver's own watcher (step 7), so a release whose reserver never saw the deployment has no
   `deployments[]` entry, but always has its `pipelineRun` (confirm carries it). These reads run in every
   cycle that still owes one, whether or not the page itself was re-read. A failure of this
   read leaves the id absent (retried next cycle) and does not change `ReleasesState`. 200 history
   rows cost at most 200 reads once, spread over ten cycles; a project with fewer than 200
   releases costs none. Until a row's read answers, task 08 says *"pin not loaded"*, never *"no
   brief is recorded"*. `PinsByRun` is emptied with the rest of the Hub data when the binding changes
   (step 1).
7. **The join** (`join.go`): for each release in `Releases` or `PinsByRun` with `state == "confirmed"`, `kind == "run"`,
   `reservedBy.id == Me.id`, a `pipelineRun`, and no `deployments[]` entry — if a deployment with
   `ID == pipelineRun` is in `Deployments` or `Latest`, call `JoinDeployment(release.id,
   pipelineRun)` once. `409 conflict` is logged and not retried. Only the reserver's own watcher
   joins; the view (task 08) never needs the join to show a pin, because it matches
   `pipelineRun == Deployment.ID` directly (plan §2 step 6), so a join written late loses nothing.
8. **Owner probe** (7P-9), once per binding and then every 10 minutes: `Governor.AutoDeploy(ref,
   <the primary environment's branch>)`. `KindForbidden` → `"no"`; success or `KindConflict`
   (*"branch has no environment yet"*, `projects.go:4132-4134`) → `"yes"`; anything else or no
   primary environment → `"unknown"`.
9. **Compare** the new snapshot with the old, ignoring `FetchedAt`; if different, `Version++` and
   the global cursor `++`.

**Failure rule.** The first `KindUnavailable` in a cycle ends the cycle for **every** project
(Panacea is down; ten projects times three timeouts would stall the loop for minutes). Each keeps
its last data with `Stale: true` and `LastError` set, and the cursor bumps once. `KindUnauthorized`
is reported with `Connection.Observe` and likewise ends the cycle. A later good cycle clears both
and calls `Observe(nil)`. An `EnvironmentFor` refusal of any other kind is not a cycle failure:
it is recorded per branch (step 2) and the cycle goes on. Nor is an `Environments` refusal
(`KindNotFound`, `KindForbidden`, or any kind but those two): it stales that one project, re-resolves its
binding next cycle, and the cycle goes on (step 2, review M13).

**The per-cycle deadline** (review M14). One goroutine means one hung read holds every project:
`do` retries a GET twice after 250 ms and 1 s (t02:95), each attempt up to
`PANACEA_APP_PANACEA_TIMEOUT` (15 s, plan §6), so one read could hold the loop for about 46 s.
So each cycle — the connection gate, every project's steps 1-8, and the Hub reads — runs under
**one** `context.WithTimeout(ctx, CycleDeadline)` (zero = 15 s), cancelled when the cycle returns.
Its expiry cancels the read in flight; `do`'s sleep is ctx-aware (t02:48), so a pending retry
returns at once. A Panacea read cut this way is the cycle's first `KindUnavailable` (a transport
failure, Status 0): the failure rule above applies unchanged — the cycle ends for every project,
each is `Stale` with that `LastError`, the cursor bumps once. A Hub read cut this way is
`hubclient.ErrUnavailable`: step 6's rule, previous releases kept. The next tick starts a fresh
cycle with a fresh deadline; `time.Ticker` drops the ticks missed meanwhile, so nothing queues.
**Order within a cycle:** projects with a tracked run first, then the rest in rotation, starting
after the last project the previous cycle reached — so a cycle cut short starves no project, and
a hang in another project's reads never comes before a tracked run's. The 3 s active cadence
holds while Panacea answers; while it hangs, every cycle ends within `CycleDeadline`, so the loop
tries again no later than `CycleDeadline` + 3 s after the last try began, and the lens says stale, never a frozen
*"running"*. What this does **not** do is back off on
`Retry-After`: that is seam S7-R1 (t02:103-105).

#### The cursor on `GET /v1/sessions` (HA7-4)

If Hub task C2 has built the generic mechanism, this task adds one line in `main.go`:
`deps.CursorSources = append(deps.CursorSources, watcher)`. If C2 built only its single
`Panacea-Hub-Cursor`, this task generalises it as HA7-4 describes — the `CursorSource` interface,
the loop in `handleSessionList` (`sessions.go:724`), and both names in `Access-Control-Expose-Headers` — and says so in
the commit as a declared amendment to C2's code. If C2 has not landed at all, this task adds the
mechanism exactly as HA7-4's source and C2 adopts it.

```go
// CursorSource — control-plane/internal/api (HA7-4's shape, verbatim).
type CursorSource interface {
	CursorName() string // one token of [A-Za-z]; the header is "Panacea-" + name + "-Cursor"
	Cursor() string     // opaque; "" means do not set the header
}
```

The header is set on `GET /v1/sessions` only, never computed there: `Cursor()` reads an atomic.
`GET /v1/sessions` stays read-only (§10a) — reading a counter writes nothing.

#### The Hub change poller feeds `HubEvents` (review M12)

The Hub already tells the control plane when a release changes: C2's poller reads `/v1/changes`
every 5 s, and the feed carries every `release.*` event (Hub contract §8, :1155-1159). Phase 7
adds **no second feed reader** — a second `after=` cursor would double the Hub's load and could
disagree with the first. The same three branches as the cursor above:

- C2's poller already hands each page to listeners → `main.go` adds `watcher.HubEvents` to them,
  one line.
- C2's poller has no listener → this task adds one field to it, `OnEvents []func([]hubapi.Event)`,
  called after each page is read and before the cursor advances, and says so in the commit as a
  declared amendment to C2's code (as HA7-4's *meanwhile* does). A listener never blocks the
  poller: `HubEvents` only marks and Nudges, under the watcher's mutex.
- C2 has not landed → the watcher still refreshes releases on `HistoryInterval`, and this task
  adds nothing to the Hub side; C2 wires `HubEvents` when it lands (plan §10.1, row Hub C2).

`hubapi.Event`'s Go field names (`Kind`, `ProjectID`) are assumed from §8's wire (`kind`,
`projectId`); the compiler will say otherwise.

#### Tests to write first

All with `deploytest.Fake`, `hubclient.NewFake()`, a fake clock, and `Run` driven by a test ticker.

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestEnvironmentsBeforeLatest` | the fake's `Calls()` for one cycle start `Environments`, then any `EnvironmentFor`, then `Latest` | reading `Latest` first, so a deleted project reads as "never deployed" |
| `TestWantedBranchResolvedOnce` | `WantBranch(p, "feat/x")` with the fake seeded `feat/x → feat-x` and a row `feat-x`: the next cycle calls `EnvironmentFor(p,feat/x)` once and `BranchEnvs["feat/x"] == "feat-x"`; cycles 2 and 3 make no such call, cycle 4 re-reads; a 17th wanted branch drops the least recently wanted | resolving by comparing `feat/x` with a row's `name`/`branch` (review C2), or one request per branch per cycle |
| `TestRefusedBranchNotReaskedEveryCycle` | `WantBranch(p, "live-check/p7")` with the fake answering `EnvironmentFor` `KindNotFound` (*"Panacea could not read this branch name…"*): cycle 1 makes one such call, cycles 2 and 3 none, cycle 4 one; `BranchEnvErrs["live-check/p7"]` carries the error, `BranchEnvs` lacks the branch, and `Stale == false` with `Latest` still read; the fake then answers `live-check-p7` → it moves to `BranchEnvs` and out of `BranchEnvErrs` (b02 round 2, R2) | a refused branch re-asked every 15 s, or one bad name staling the whole project |
| `TestWatcherPassesSameEnvironment` | the watcher's Reader is the fake wrapped so that `SameEnvironment` records its arguments and answers `true`: `w.SameEnvironment("x", "y")` is `true` and the wrapper saw `("x", "y")` (the bare Fake would answer `false`) | a watcher that hands the view a rule of its own (b02 round 2, R4) |
| `TestReleasesStateNamesKeySpelling` | a binding with `HubProjectID ""` and `HubMiss "key_spelling"` → `ReleasesState "hub_key_spelling"`; `HubMiss "not_linked"` → `"no_hub_project"`; `HubMiss ""` with a configured Hub → `"unavailable"`; none calls `Releases` (review I6; plan re-review b05 R-4) | a repository told it is *"not a shared project in the Hub yet"* above a Ship bar that names the real cause — two surfaces that disagree; a key the Hub never answered for (a first-read failure) shown as *"not a shared project"* |
| `TestDeploymentsNotEveryCycle` | three cycles with nothing changed → one `Deployments` call; `Latest.ID` changes → one more | reading the heavy list every 15 s |
| `TestScansReadWithTheDeploymentsEnvironment` | a newest deployment `{ID: "d-feat", Environment: "feat-x"}`, with the fake seeded `SetScans(p, "feat-x", "d-feat", …)` → the fake's `Calls()` holds `Scans(engagement-portal,feat-x,d-feat)` and `Snapshot.Scans["d-feat"]` is that report (review C3) | passing `""` or the card's name, which on Panacea shows production's verdict as the branch release's |
| `TestScansFailureIsNotStale` | a newest deployment `{ID:"d-feat", Environment:"feat-x"}` with **no** `SetScans` (the Fake answers `KindNotFound`) → `Snapshot.Scans` has no `"d-feat"`, `Stale` is `false`, `LastError` is `""`, and the same cycle still reads Hub releases; 61 s later → a second `Scans` call (re-review b03, M-2) | a scan read error that stales the whole project, or one cached as done so it is never retried |
| `TestScansSkipsEmptyEnvironment` | a newest deployment `{ID:"d-old", Environment:""}` → `Calls()` holds no `Scans` for `d-old` and `Snapshot.Scans` has no `"d-old"` (re-review b03, M-1) | calling `Scans(p, "", id)`, which on Panacea stores production's verdict under a legacy release |
| `TestHistoryIntervalForcesRead` | 61 s of fake time with no change → a second `Deployments` | history that never refreshes when only `LiveReady` moved |
| `TestTrackedRunDrivesActiveCadence` | `Track(p, run)`; the project is polled every 3 s until the fake advances the run to succeeded, then every 15 s | a run that stays on 3 s forever, or never gets it |
| `TestUnavailableEndsCycleForAll` | two bound projects; the first `Environments` fails `KindUnavailable` → the second project saw **no** call this cycle; both `Stale`, one cursor bump | a cycle that stalls for N × timeout in an outage |
| `TestEnvironmentsRefusedReResolves` | two bound projects, one good cycle; then A's `Environments` fails `KindNotFound` → in that cycle B is still read, A makes no `Latest` call, A is `Stale` with `LastError.Kind == KindNotFound` and its old `Environments` kept, `FetchedAt` the good cycle's. The resolver's `Deployer` is the fake wrapped so that `Projects` then omits A's project (the Fake has `AddProject` and no removal, t01:567) → the next tick (3 s, not 15 s) calls `Resolve` for A, whose binding is `NoMatch`, and A's snapshot holds no `Environments`, `Latest` or `Deployments`. Repeated with `KindForbidden`, and with a second Panacea project that now matches A's repository → `Bound` to it, and A's snapshot holds none of the first project's rows (review M13) | a refusal read as Panacea down, ending every project's cycle; a deleted project drawn fresh (`Stale false`) or its rows kept after the binding moved; a re-resolve that waits for the every-fourth-cycle count |
| `TestCycleDeadlineCutsAHungRead` | `CycleDeadline` 50 ms of real time (the one test off the fake clock); two bound projects; the watcher's Reader is the fake wrapped so that project A's `Environments` blocks until its `ctx` is done and then returns `KindUnavailable`, Status 0 (what `do` returns for a cancelled transport). One cycle returns within 1 s; B saw no call; both `Stale`, `LastError.Kind == KindUnavailable`; one cursor bump. Then `Track(B, run)`: the next cycle's first call is B's `Environments`, before A's (review M14) | a cycle with no deadline, so one hung read holds the loop for `do`'s three attempts (about 46 s); a deadline per read, not per cycle, so ten projects' hangs add up; a fixed project order that puts a tracked run behind a hanging project every cycle |
| `TestUnauthorizedObserved` | `KindUnauthorized` → `Connection.Observe` called with it; the cycle ends | a lens that keeps drawing "connected" while every read 401s |
| `TestStaleKeepsLastData` | a good cycle, then a failing one → `Environments` unchanged, `Stale == true`, `FetchedAt` is the good cycle's time | clearing the lens during a blip |
| `TestJoinOnlyOwnConfirmedRuns` | releases: mine confirmed, mine reserved, a teammate's confirmed, mine rollback — only the first is joined, and only once across three cycles | joining a teammate's release, a reservation, or a rollback (7P-4: a rollback has no deployment of its own) |
| `TestJoinWaitsForDeployment` | a confirmed release whose run has no deployment yet → no join; next cycle it appears → one join | joining by observation of a run name that does not yet exist |
| `TestPinsBeyondThePage` | the Hub fake holds 201 confirmed run releases for one Panacea project; `Deployments` holds 25 rows, the oldest, `d-old`, being the pipelineRun of the oldest release (not in the first page of 200) and 24 others with no release → the page read gives `ReleasesMore == true`; cycle 1 makes exactly 20 `Releases` calls with `PipelineRun` set, newest rows first; cycle 2 makes the other 5, one of them `d-old`, and `PinsByRun["d-old"]` is that release, the rest `nil`; cycle 3 makes none. With 199 releases → `ReleasesMore == false` and no `PipelineRun` call ever (review M11) | a pin beyond the 200th release read as *"no brief is recorded"* (P9 inverted); a targeted read per row per cycle; a targeted read made when the page was complete |
| `TestReleaseEventRefreshesReleases` | two bound projects with `HubProjectID` `pj-a` and `pj-b`, releases read at cycle 1; `HubEvents([{kind:"release.confirmed", projectId:"pj-a"}, {kind:"brief.revised", projectId:"pj-b"}, {kind:"release.abandoned", projectId:"pj-other"}])` at 5 s of fake time → the next tick (3 s later, not 15 s or 60 s) reads `Releases` for `pj-a` only; `HubEvents` itself made no call (the fake's `Calls()` unchanged until the tick) (review M12) | a teammate's rollback shown 60 s late; a `brief.*` event, or another project's, triggering release reads; a listener that reads the Hub on the poller's goroutine |
| `TestHubDownKeepsReleases` | `hubclient` fake failing → `ReleasesState "unavailable"`, previous releases kept, Panacea data still fresh | the lens's pins vanishing when the Hub blips |
| `TestOwnerProbe` | `AutoDeploy` → `KindForbidden` ⇒ `"no"`, including a `KindForbidden` whose `Message` is the scope sentence of task 03's `error-scope-owner-403.txt` — a real owner holding a restricted key (review I5); `KindConflict` ⇒ `"yes"`; success ⇒ `"yes"`; `KindUnavailable` ⇒ `"unknown"` | inferring owner from `/me` groups (7P-9) |
| `TestCursorMonotonic` | the cursor never decreases across 50 random fake cycles, and is unchanged across two identical ones | a cursor that resets when a project unbinds, making the desktop miss a change |
| `TestSessionsHeader` | `GET /v1/sessions` with the watcher as a `CursorSource` carries `Panacea-Deploy-Cursor`; `Access-Control-Expose-Headers` lists it | a header the webview cannot read |

#### Steps

- [ ] Check what Hub C2 has built (`grep -rn "CursorSource\|Panacea-Hub-Cursor"
  control-plane/internal`) and pick the branch of *"The cursor"* above. Then find C2's change
  poller (`grep -rn "Changes(ctx" control-plane/internal control-plane/cmd`) and pick the branch of
  *"The Hub change poller feeds `HubEvents`"*.
- [ ] Tests first; then `watcher.go`, `join.go`; then the cursor wiring.
- [ ] `config.go`: the three intervals and `PANACEA_APP_DEPLOY_CYCLE_DEADLINE` (plan §6) through
  `parseDuration` (`config.go:317`), each refusing a value under 1 s. `main.go`: `go watcher.Run(ctx)` after `connection.Check`.
- [ ] Run the control-plane suite; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/deploywatch/ ./control-plane/internal/api/ -race -count=1
→ ok

grep -rn 'setInterval(' desktop/src --include=*.ts --include=*.tsx | grep -v '\.test\.'
→ exactly two lines, both unchanged by this task: api/poller.ts's
  `const handle = setInterval(() => void tick(), POLL_INTERVAL_MS);` and hooks/heartbeat.ts's
  `timer = setInterval(() => {` (at a92c3b0: poller.ts:90 and heartbeat.ts:34; a later merge may
  move the numbers, not the count). The `(` is the anchor, review I1: a bare 'setInterval' also
  prints heartbeat.ts:29, `ReturnType<typeof setInterval>`, which is a type, not a timer.
```

#### What this task does NOT claim

- That 15 s is the right cadence for the owner's team. It is a variable.
- The exact spelling of `hubapi.ReleaseQuery`'s fields. The Hub contract names the type (§9.2)
  and the query parameters (§7.9: `environment`, `panaceaProject`, `deploymentId`, `pipelineRun`,
  `state`), not the Go fields; this part assumes `PanaceaProject`, `PipelineRun`, `State`, `Limit`
  and `Page.Next` (a `*string`, from §1's `"next": "<cursor>" | null`) and the compiler will say
  otherwise. Nor that the Hub fake honours `pipelineRun=` and pages at 200: `TestPinsBeyondThePage`
  is red until it does.
- That the watcher sees a rollback. Nothing in Panacea records one (plan 7P-4); task 08 reads the
  Hub's rollback releases for that.
- Anything the desktop does with the cursor. Task 13.
