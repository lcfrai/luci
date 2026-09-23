### Task 09: Ship — reserve, deploy, confirm or abandon

**Half:** 7b. **Hours:** 3. **Depends on:** 04, 08. **Elsewhere:** Hub H0 (`hubclient.Releases`
fake; H5 for the integration run); phase 3 task 08 (`remote.State`, `Convergence.Tip`); Hub ask
HA7-1 for a first Ship of a never-deployed branch.

The Hub's graft G1 (ADR-H1 §4.2; Hub contract §7.9): **the Hub takes the pin before the deploy
exists**, and the only thing written afterwards is which run it became. That is how prohibition
P7 (*"a release link written by a second call"*) holds with the ledger on another machine. Phase 7
never chooses which revision is pinned; it sends the branch, and the Hub resolves the brief.

**Files:**
- `control-plane/internal/deploy/ship/ship.go` (new) — `BranchState`, `Branch`, `Releases`, `Config`, `Orchestrator`, `Request`, `Result`, `Ship`
- `control-plane/internal/deploy/ship/confirm.go` (new) — the confirm retry loop
- `control-plane/internal/deploy/ship/ship_test.go`, `confirm_test.go` (new)
- `control-plane/internal/api/deploy_acts.go` (new) — `POST /v1/projects/{projectId}/deploy/ship`
- `control-plane/internal/api/deploy_acts_test.go` (new)
- `control-plane/internal/api/api.go`, `control-plane/cmd/api/main.go` (modified) — `Deps.DeployShip`, the wiring; the `BranchState` adapter over phase 3's `remote.ReadState`

#### Interfaces — complete

```go
// Package ship sequences every act that produces a release: Ship here, and rollback, rebuild
// and cancel in rerun.go (task 10). It holds a deploy.Actor and the Hub's release calls; it never
// chooses a pinned revision.
package ship

// Branch is phase 3's reading of one local branch against its remote (phase 3 task 08:
// remote.State plus Convergence.Tip).
type Branch struct {
	Name      string // "feat/evidence-upload"
	Ref       string // "refs/heads/feat/evidence-upload" — the Hub's branchRef (Hub contract §3.3)
	Tip       string // 40 hex
	Upstream  string // "origin/feat/evidence-upload"; "" = never pushed
	Ahead     int
	Behind    int
	FetchedAt time.Time // zero = never fetched
	ReadError string    // phase 3's sentence when the reading failed
}

// BranchState is satisfied in main.go by an adapter over phase 3's remote.ReadState and the
// catalogue's branch lookup. Tests use a table.
type BranchState interface {
	Branch(ctx context.Context, projectID, branchID string) (Branch, error)
}

// Releases is the subset of hubclient.Releases (Hub contract §9.2) Ship needs.
type Releases interface {
	Reserve(ctx context.Context, projectID string, req hubapi.ReserveRequest) (hubapi.Release, error)
	Confirm(ctx context.Context, releaseID string, req hubapi.ConfirmRequest) (hubapi.Release, error)
	Abandon(ctx context.Context, releaseID string, req hubapi.AbandonRequest) (hubapi.Release, error)
}

type Tracker interface {
	Track(projectID, run string)
	Nudge(projectID string)
	Snapshot(projectID string) (deploywatch.Snapshot, bool)
}

type Config struct {
	Actor    deploy.Actor
	Bindings interface {
		Cached(projectID string) (binding.Binding, bool)
	}
	Branches BranchState
	Releases Releases
	Tracker  Tracker
	// EnvironmentOptional is true once the Hub grants HA7-1: a first Ship reserves with
	// environment null. False: such a Ship is refused with deploy_environment_unknown.
	EnvironmentOptional bool
	Now func() time.Time
	Log *slog.Logger
}

type Orchestrator struct{ /* cfg; per-(project,branch) mutex; idempotency cache */ }

func New(cfg Config) (*Orchestrator, error)

type Request struct {
	ProjectID      string // local "p-…"
	BranchID       string // local "b-…"
	Commit         string // what the desktop drew; must equal the branch tip now
	DesignKey      string // "" = none; the Hub validates approval and never infers one
	NoCache        *bool
	IdempotencyKey string // from the desktop's Idempotency-Key header; required
}

type Result struct {
	Release   hubapi.Release
	Run       string
	Confirmed bool   // false: Panacea started the run, the Hub confirm is still being retried
	Message   string // "" when Confirmed
}

// Precondition is a refusal made before anything leaves the control plane.
type Precondition struct {
	Reason  string            // see the table below
	Message string            // the sentence the lens shows
	Details map[string]string // "ahead", "behind" when relevant
}

func (p *Precondition) Error() string

// Ship runs the sequence below. It returns *Precondition, *deploy.Error, a hubclient error, or
// a Result. A repeat with the same IdempotencyKey within 15 minutes returns the first Result (or
// the first error) without calling anything.
func (o *Orchestrator) Ship(ctx context.Context, req Request) (Result, error)
```

#### The sequence — exactly

1. **Idempotency.** A cached result for `IdempotencyKey` → return it. The cache is in memory,
   15 minutes, keyed by `(IdempotencyKey, ProjectID)`. A desktop retry after a lost response
   therefore never reserves twice.
2. **One Ship per branch at a time.** A per-`(ProjectID, BranchID)` mutex, `TryLock`; held →
   `409 conflict` *"A Ship of this branch is already in progress."*
3. **Preconditions** (nothing has left the process):

   | Check | `Precondition.Reason` | Message |
   |---|---|---|
   | binding not `Bound` | `not_bound` | the binding's own `Message` (task 06) |
   | `HubProjectID == ""` | `no_hub_project` | *"This repository is not a shared project in the Hub yet, so the release could not be recorded. Ship stays off until it is."* |
   | `Branch.ReadError != ""` | `branch_unreadable` | phase 3's sentence |
   | `Upstream == ""` | `never_pushed` | *"This branch has never been pushed, so Panacea has nothing to build."* |
   | `Ahead > 0` | `unpushed` | *"N commits are not pushed, so they would not ship. Push first."* (`details.ahead`) |
   | `Behind > 0` | `behind` | *"The remote has N commits this clone has not fetched. Fetch, check them, then ship."* (`details.behind`) |
   | `Commit != Tip` | `stale_commit` | *"The branch has moved since this screen was drawn. Look again, then ship."* |
   | `!deploy.ValidCommit(Commit)` | `bad_commit` | *"That is not a full commit id."* |

   All map to `409 deploy_precondition` with `details.reason`.
4. **Environment** — from the watcher's snapshot: the `environments[]` row whose `branch == Branch.Name`
   (exact). None → `EnvironmentOptional` ? `null` : `409 deploy_environment_unknown` (plan §9 HA7-1).
5. **Reserve** — `Releases.Reserve(HubProjectID, {kind:"run", panaceaProject: <ref>, environment,
   branchRef: Branch.Ref, commit, briefId: null, designKey: DesignKey|null, sourceDeploymentId:
   null})`. Any error → returned as is (the relay rule, Hub contract §4.3). **Ship stops. Panacea is
   never called.**
6. **Deploy** — `Actor.Ship(ref, {Branch.Name, Commit, NoCache})`.
   - **Refused** (`KindForbidden`, `KindNotFound`, `KindConflict`, `KindRejected`, `KindUnauthorized`)
     → `Abandon(release, {reason:"deploy_failed", message: e.Message})`; return the Panacea error.
   - **5xx** (`KindUnavailable` with `Status >= 500`) → `Abandon(…, message: "Panacea could not
     start the build.")`; return the error.
   - **No answer** (`KindUnavailable`, `Status == 0` — timeout, reset) → `Abandon(…, message:
     "Panacea did not answer in time. It may still have started the build; if a run appears, it
     will show with no brief recorded.")`; return the error. This is the one ambiguous case:
     `POST …/deploy` spends up to 15 s creating the PipelineRun (`projects.go:2985-2993`), and a
     run created after our timeout would show unpinned — honestly, because confirming it by
     branch and commit would be pinning by observation (P2).
   - An `Abandon` that fails is logged; the reservation then expires on its own (Hub contract
     §7.9, `expiresAt`).
7. **Confirm** — `Releases.Confirm(release, {pipelineRun: run})`. `hubclient` retries a transport
   failure twice with the same `Idempotency-Key` (Hub contract §9.2). Still failing →
   `confirm.go` keeps retrying **in the background**, with a context detached from the request,
   at 2 s, 5 s, 15 s, then every 30 s, until `release.expiresAt`; the route answers `202` with
   `confirmed: false`. `409 release_expired` or `release_state` stops the loop with an error log
   (ADR-H1 risk R3).
8. **Track** — `Tracker.Track(ProjectID, run)`, `Tracker.Nudge(ProjectID)`.

#### Control-plane route — complete wire

```
POST /v1/projects/{projectId}/deploy/ship
  header  Idempotency-Key: <1–64 of [A-Za-z0-9_-]>           (required)
  request { "branchId": "b-8c1d0e42-evidence-upload", "commit": "<40 hex>",
            "designKey": "design:cv-7c01d2e3f4a5b6c7@11" | null, "noCache": true | null }
  202     { "release": ReleasePin, "run": "engagement-portal-build-x7k2p",
            "confirmed": true | false, "message": "" }
  400 bad_request                 missing/invalid Idempotency-Key; unknown field; missing branchId
  404 project_not_found           unknown local project
  409 conflict                    a Ship of this branch is in progress
  409 deploy_precondition         details.reason ∈ not_bound | no_hub_project | branch_unreadable |
                                  never_pushed | unpushed | behind | stale_commit | bad_commit
  409 deploy_environment_unknown  (until HA7-1)
  409 deploy_not_connected · deploy_token_rejected
  403 deploy_forbidden · 404 deploy_not_found · 409 deploy_conflict · 422 deploy_rejected
                                  Panacea refused; the release was abandoned with its sentence
  503 upstream_unavailable        details.upstream "panacea" (abandoned) or "hub" (nothing happened)
  (any Hub 4xx, unchanged)        e.g. 422 design_not_approved, 401 signed_out
```

`ReleasePin` is task 08's shape. The desktop sends this request **after** its 5 s hold (plan 7P-15,
task 16) and generates the key when the hold starts, so an Undo sends nothing and a lost response
is retried with the same key.

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestReserveFailureNeverCallsPanacea` | the Hub fake refuses reserve (`422 design_not_approved`) → error returned unchanged; `deploytest.Fake.Calls()` is empty | deploying first and recording later (P7) |
| `TestReserveCarriesBranchNotBrief` | the reserve request has `branchRef`, `commit`, `briefId: null` | the control plane choosing the brief — the Hub's job (§7.9) |
| `TestConfirmWithRunName` | Panacea returns run `r1` → `Confirm(release, {pipelineRun:"r1"})`, once | a confirm that omits or mangles the run, so the view's rule 5 never matches |
| `TestRefusalAbandonsWithSentence` | Panacea `409` *"…"* → `Abandon` with `reason "deploy_failed"` and exactly that message; the route answers `409 deploy_conflict` | a refused Ship leaving a reservation that expires silently |
| `TestTimeoutAbandonsAmbiguously` | Panacea times out → `Abandon` message is the ambiguous sentence; no confirm | confirming a run the control plane never saw named |
| `TestNeverPushedPrecondition` | `Upstream ""` → `409 deploy_precondition` `never_pushed`; neither Hub nor Panacea called | reserving a release for a commit Panacea cannot clone |
| `TestUnpushedPrecondition` | `Ahead 2` → `unpushed`, `details.ahead "2"` | shipping the upstream while the user thinks they shipped their work |
| `TestStaleCommitPrecondition` | `Commit` ≠ `Tip` → `stale_commit` | shipping a commit other than the one the screen showed |
| `TestEnvironmentUnknownUntilHA71` | no environment row for the branch, `EnvironmentOptional false` → `409 deploy_environment_unknown`; `true` → reserve with `environment: null` | re-deriving Panacea's environment name (a fourth `sanitizeBranchEnv`) |
| `TestIdempotentShip` | the same key twice → one reserve, one Panacea call, equal results | a desktop retry after a lost response double-shipping |
| `TestConcurrentShipSameBranch` | two goroutines, one branch → one proceeds, one gets `409 conflict` | two reservations racing one branch |
| `TestConfirmRetriedInBackground` | the Hub fails confirm three times, then succeeds → the route answered `202 confirmed:false`; the fake Hub eventually holds a `confirmed` release; the loop stopped | a Ship that reports success while the release stays reserved forever |
| `TestConfirmStopsAtExpiry` | the Hub answers `409 release_expired` → the loop stops, one error log | a loop that retries past the reservation's life |
| `TestTracksRun` | after success, `Tracker.Track(p, "r1")` and `Nudge(p)` called | a 15 s wait before the lens shows the build |

#### Steps

- [ ] Tests first, over `deploytest.Fake`, `hubclient.NewFake()` with an injected clock, and a
  table `BranchState`.
- [ ] `ship.go`, `confirm.go`, the route in `deploy_acts.go`; `writeDeployError` (task 05) for
  every error; `*Precondition` → `409 deploy_precondition` with `details.reason`.
- [ ] `main.go`: the `BranchState` adapter — `Deps.Projects.Branches` for the name, phase 3's
  `remote.ReadState` and `Convergence.Tip` for the rest. If phase 3 task 08 is not merged, this
  task is built and tested but **not merged** (plan §8, 7b).
- [ ] Integration, when Hub H5 is up in `compose.yaml`: `go test -tags integration
  ./control-plane/internal/deploy/ship/` with `panaceatest.Server` for Panacea and the real Hub.
- [ ] Commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/ship/ ./control-plane/internal/api/ -race -count=1
→ ok

go test -tags integration ./control-plane/internal/deploy/ship/ -count=1     (Hub H5 running)
→ ok: a Ship reserves, confirms, and the Hub release's pin equals the fake brief's latest complete revision
```

#### What this task does NOT claim

- A live deploy. That is task 20, with the owner (OD-12).
- That the Hub pins the right revision — that is the Hub's H5 acceptance (Hub contract §13). Phase
  7 asserts only that it sent the branch and never a brief.
- Recovery of a pin for a run created after a timeout. Without the metadata ask (R7, seam S7-N2)
  it cannot be done honestly; the abandon sentence says so.
