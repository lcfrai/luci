### Task 07: The watcher — one process polls Panacea, the desktop hears a cursor

**Half:** 7a. **Hours:** 3. **Depends on:** 03, 06. **Elsewhere:** Hub H0 (`hubclient.Releases`
fake); Hub C2's cursor mechanism (ask HA7-4 — this part says what to do if C2 has not built it).

Plan decision 7P-5. The desktop's one-clock rule (`poller.ts:84`, `hooks/heartbeat.ts:34` — the only
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
- `control-plane/cmd/api/config.go`, `main.go` (modified) — three interval variables (plan §6); start `Run`

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
	Now func() time.Time
	Log *slog.Logger
}

type Watcher struct{ /* unexported */ }

func New(cfg Config) (*Watcher, error)

// Run blocks until ctx is done. ONE goroutine, one time.Ticker at ActiveInterval; each project
// is polled when it is due (Interval, or ActiveInterval while it has a tracked run).
func (w *Watcher) Run(ctx context.Context)

// Snapshot is the newest reading for one local project; ok false before the first cycle.
func (w *Watcher) Snapshot(projectID string) (Snapshot, bool)

// Track makes a run this user started (Ship, rebuild) drive the active cadence until its
// build is terminal. Nudge polls a project on the next tick. Both are called by task 09/10
// right after an act; neither makes a request itself.
func (w *Watcher) Track(projectID string, run string)
func (w *Watcher) Nudge(projectID string)

// WantBranch asks for a git branch's environment name to be resolved (Reader.EnvironmentFor)
// and kept in Snapshot.BranchEnvs. Task 08's route calls it for the lens's selected branch; it
// makes no request itself and Nudges the project when the branch is new. At most 16 branches
// per project are kept, the least recently wanted dropped first.
func (w *Watcher) WantBranch(projectID, branch string)

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
	Latest        *deploy.Deployment // nil = never deployed, or not yet read
	Deployments   []deploy.Deployment
	DeploymentsAt time.Time
	Runs          map[string]deploy.RunProgress // tracked runs only
	Scans         map[string]deploy.ScanReport  // by deployment id; newest deployment per environment only
	Releases      []hubapi.Release              // newest reservedAt first, at most 200
	ReleasesState string                        // "ok" | "no_hub" | "no_hub_project" | "signed_out" | "unavailable"
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
1. **Binding.** `Bindings.Resolve` when the cached binding is not `Bound`, or every fourth cycle;
   else `Cached`. Not bound → the snapshot carries the binding and nothing else; stop.
2. **`Environments`.** First, because a missing *project* surfaces here as its own error, which is
   what makes task 03's *"`Latest` 404 means never deployed"* safe.
   **Then `EnvironmentFor`** for each wanted branch (`WantBranch`) not yet in `BranchEnvs`, and
   for every wanted branch every fourth cycle — a changed production-branch label remaps `main`
   (`mapBranchToEnvWithProd`, `projects.go:3838-3849`). One request per new branch, not per
   cycle. Its failures follow the failure rule below; a failed re-read keeps the old answer.
3. **`Latest`.**
4. **`Deployments`** — only when (a) `Latest.ID` or `Latest.Build` changed, (b) the environment
   list changed (names, statuses, replica pairs), (c) a tracked run became terminal, or (d)
   `HistoryInterval` passed. It is unpaginated and runs three enrichment passes including a dozen
   PipelineRun reads (`projects.go:1227-1253`), so it is never read every cycle.
5. **`Run`** for each tracked run whose build is not terminal. Terminal → untracked.
   **`Scans`** for each environment's newest deployment, only when that deployment's `ID` or
   `Build` changed since the last read, cached by deployment id in `Snapshot.Scans` — so the
   environment strip's third fact (scans) costs one request per build, not per cycle.
6. **Hub releases** — `Releases(HubProjectID, {PanaceaProject: <ref>, Limit: 200})` when the
   Deployments were re-read, a tracked run changed, or `HistoryInterval` passed.
   `Binding.HubProjectID == ""` → `ReleasesState "no_hub_project"`; `entraauth.ErrSignedOut` →
   `"signed_out"`; `hubclient.ErrUnavailable` → `"unavailable"`, **keeping the previous releases**.
7. **The join** (`join.go`): for each release with `state == "confirmed"`, `kind == "run"`,
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
and calls `Observe(nil)`.

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

#### Tests to write first

All with `deploytest.Fake`, `hubclient.NewFake()`, a fake clock, and `Run` driven by a test ticker.

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestEnvironmentsBeforeLatest` | the fake's `Calls()` for one cycle start `Environments`, then any `EnvironmentFor`, then `Latest` | reading `Latest` first, so a deleted project reads as "never deployed" |
| `TestWantedBranchResolvedOnce` | `WantBranch(p, "feat/x")` with the fake seeded `feat/x → feat-x` and a row `feat-x`: the next cycle calls `EnvironmentFor(p,feat/x)` once and `BranchEnvs["feat/x"] == "feat-x"`; cycles 2 and 3 make no such call, cycle 4 re-reads; a 17th wanted branch drops the least recently wanted | resolving by comparing `feat/x` with a row's `name`/`branch` (review C2), or one request per branch per cycle |
| `TestDeploymentsNotEveryCycle` | three cycles with nothing changed → one `Deployments` call; `Latest.ID` changes → one more | reading the heavy list every 15 s |
| `TestHistoryIntervalForcesRead` | 61 s of fake time with no change → a second `Deployments` | history that never refreshes when only `LiveReady` moved |
| `TestTrackedRunDrivesActiveCadence` | `Track(p, run)`; the project is polled every 3 s until the fake advances the run to succeeded, then every 15 s | a run that stays on 3 s forever, or never gets it |
| `TestUnavailableEndsCycleForAll` | two bound projects; the first `Environments` fails `KindUnavailable` → the second project saw **no** call this cycle; both `Stale`, one cursor bump | a cycle that stalls for N × timeout in an outage |
| `TestUnauthorizedObserved` | `KindUnauthorized` → `Connection.Observe` called with it; the cycle ends | a lens that keeps drawing "connected" while every read 401s |
| `TestStaleKeepsLastData` | a good cycle, then a failing one → `Environments` unchanged, `Stale == true`, `FetchedAt` is the good cycle's time | clearing the lens during a blip |
| `TestJoinOnlyOwnConfirmedRuns` | releases: mine confirmed, mine reserved, a teammate's confirmed, mine rollback — only the first is joined, and only once across three cycles | joining a teammate's release, a reservation, or a rollback (7P-4: a rollback has no deployment of its own) |
| `TestJoinWaitsForDeployment` | a confirmed release whose run has no deployment yet → no join; next cycle it appears → one join | joining by observation of a run name that does not yet exist |
| `TestHubDownKeepsReleases` | `hubclient` fake failing → `ReleasesState "unavailable"`, previous releases kept, Panacea data still fresh | the lens's pins vanishing when the Hub blips |
| `TestOwnerProbe` | `AutoDeploy` → `KindForbidden` ⇒ `"no"`; `KindConflict` ⇒ `"yes"`; success ⇒ `"yes"`; `KindUnavailable` ⇒ `"unknown"` | inferring owner from `/me` groups (7P-9) |
| `TestCursorMonotonic` | the cursor never decreases across 50 random fake cycles, and is unchanged across two identical ones | a cursor that resets when a project unbinds, making the desktop miss a change |
| `TestSessionsHeader` | `GET /v1/sessions` with the watcher as a `CursorSource` carries `Panacea-Deploy-Cursor`; `Access-Control-Expose-Headers` lists it | a header the webview cannot read |

#### Steps

- [ ] Check what Hub C2 has built (`grep -rn "CursorSource\|Panacea-Hub-Cursor"
  control-plane/internal`) and pick the branch of *"The cursor"* above.
- [ ] Tests first; then `watcher.go`, `join.go`; then the cursor wiring.
- [ ] `config.go`: the three intervals through `parseDuration` (`config.go:317`), each refusing a
  value under 1 s. `main.go`: `go watcher.Run(ctx)` after `connection.Check`.
- [ ] Run the control-plane suite; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/deploywatch/ ./control-plane/internal/api/ -race -count=1
→ ok

grep -rn 'setInterval' desktop/src --include=*.ts --include=*.tsx | grep -v test
→ exactly the two lines poller.ts:84 and hooks/heartbeat.ts:34 (unchanged by this task)
```

#### What this task does NOT claim

- That 15 s is the right cadence for the owner's team. It is a variable.
- The exact spelling of `hubapi.ReleaseQuery`'s fields. The Hub contract names the type (§9.2)
  and the query parameters (§7.9: `environment`, `panaceaProject`, `deploymentId`, `pipelineRun`,
  `state`), not the Go fields; this part assumes `PanaceaProject` and `Limit` and the compiler will
  say otherwise.
- That the watcher sees a rollback. Nothing in Panacea records one (plan 7P-4); task 08 reads the
  Hub's rollback releases for that.
- Anything the desktop does with the cursor. Task 13.
