### Task 09: Ship — reserve, deploy, confirm or abandon

**Half:** 7b. **Hours:** 3. **Depends on:** 04, 08. **Elsewhere:** Hub H0 (`hubclient.Releases`
fake; H5 for the integration run); phase 3 task 08 (`remote.State`, `Convergence.Tip`). A first
Ship of a never-deployed branch needs **no** Hub change: Panacea names its environment before the
deploy (`Reader.EnvironmentFor`, task 03; review b02 withdrew HA7-1).

The Hub's graft G1 (ADR-H1 §4.2; Hub contract §7.9): **the Hub takes the pin before the deploy
exists**, and the only thing written afterwards is which run it became. That is how prohibition
P7 (*"a release link written by a second call"*) holds with the ledger on another machine. Phase 7
never chooses which revision is pinned; it sends the branch, and the Hub resolves the brief.

**Files:**
- `control-plane/internal/deploy/ship/ship.go` (new) — `BranchState`, `Branch`, `Releases`, `Config`, `Orchestrator`, `Request`, `Result`, `Ship`, `Readiness`, `PinPreview`
- `control-plane/internal/deploy/ship/confirm.go` (new) — the confirm retry loop
- `control-plane/internal/deploy/ship/ship_test.go`, `confirm_test.go`, `readiness_test.go` (new)
- `control-plane/internal/api/deploy_acts.go` (new) — `POST /v1/projects/{projectId}/deploy/ship`, `GET …/deploy/ship-readiness`
- `control-plane/internal/api/routes_audit_test.go` (modified) — the readiness route's `catalogReadOnlyRoutes` line
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
	WantBranch(projectID, branch string) // task 07: readiness asks for the branch's environment name
}

// EnvironmentNamer is the one Reader method Ship needs: Panacea's own environment name for the
// branch being shipped (task 01). deploy.Reader satisfies it.
type EnvironmentNamer interface {
	EnvironmentFor(ctx context.Context, p deploy.ProjectRef, branch string) (string, error)
}

type Config struct {
	Actor    deploy.Actor
	Bindings interface {
		Cached(projectID string) (binding.Binding, bool)
	}
	Branches BranchState
	Releases Releases
	Tracker  Tracker
	// Environments names the branch's environment at Ship time (step 4). There is no
	// "environment unknown" mode: Panacea answers for any branch (review b02, C2).
	Environments EnvironmentNamer
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
	Details map[string]string // "ahead", "behind" when relevant; "cause" = "key_spelling" (HA7-6) or "hub_unanswered" on no_hub_project
}

func (p *Precondition) Error() string

// Ship runs the sequence below. It returns *Precondition, *deploy.Error, a hubclient error, or
// a Result. A repeat with the same IdempotencyKey within 15 minutes returns the first Result (or
// the first error) without calling anything.
func (o *Orchestrator) Ship(ctx context.Context, req Request) (Result, error)

// Readiness is steps 3 and 4 of the sequence — the SAME function Ship calls — plus a preview of
// the pin the Hub would take, so the button and the server cannot disagree about why Ship is off.
// It reserves nothing and calls Panacea not at all.
func (o *Orchestrator) Readiness(ctx context.Context, projectID, branchID string) (Readiness, error)

type Readiness struct {
	Ready       bool
	Reason      string // "" when Ready; else a Precondition.Reason, or "not_connected" | "stale" | "signed_out" | "environment_invalid"
	Message     string
	Commit      string // the tip Ship would send; "" when unknown
	// Environment is Panacea's own name for the branch's environment, from the watcher's
	// Snapshot.BranchEnvs (readiness calls Tracker.WantBranch and makes no Panacea call); ""
	// until resolved. Not resolved is NOT a reason to refuse: Ship reads the name itself.
	Environment string
	// PinPreview reads the Hub's brief for the branch through hubclient.Briefs.ForBranch (Hub
	// contract §9.2) and applies the Hub's rule — latest complete revision (ADR-H1 Q1). nil when
	// the Hub or phase 4's brief is absent. It is a PREVIEW: the Hub takes the pin at reserve.
	PinPreview *PinPreview
}

type PinPreview struct {
	Status   string // "pinned" | "none"
	Reason   string // "ship_head" | "no_brief" | "no_complete_revision"
	BriefID  string // "" when none — the id only; the desktop resolves text through phase 4
	Revision int    // 0 when none
}
```

The readiness route (read-only; `catalogReadOnlyRoutes`: *"Evaluates Ship's preconditions and
reads the brief head; reserves nothing, writes nothing."*):

```
GET /v1/projects/{projectId}/deploy/ship-readiness?branchId=b-…
  200 { "ready": false, "reason": "unpushed", "message": "2 commits are not pushed, so they would not ship. Push first.",
        "commit": "<40 hex>" | null, "environment": "feat-evidence-upload" | null,
        "pinPreview": { "status": "pinned", "reason": "ship_head", "briefId": "bf-…", "revision": 6 } | null }
  404 project_not_found
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
   | `HubProjectID == ""`, `HubMiss == "not_linked"` | `no_hub_project` | *"This repository is not a shared project in the Hub yet, so the release could not be recorded. Ship stays off until it is."* — only when the Hub has answered no (plan re-review b05 R-4) |
   | `HubProjectID == ""`, `HubMiss == "key_spelling"` (task 06; HA7-6) | `no_hub_project`, `details.cause "key_spelling"` | *"The Hub cannot hold this repository yet: the cockpit names it `<RepoKey>`, which is not a form the Hub accepts. Ship stays off until the two agree."* — the cause, never *"not a shared project"* for a repository that may be one, and no host named, because `missCause` is not host-specific (review I6; plan re-review b05 R-3) |
   | `HubProjectID == ""`, `HubMiss == ""` (no Hub configured, or the Hub has never answered for this key) | `no_hub_project`, `details.cause "hub_unanswered"` | *"The Hub has not answered for this repository, so the cockpit cannot tell whether it is a shared project. Ship stays off until it does."* — never *"not a shared project"* about a repository the Hub has not been asked about. The view's `releasesState` tells the two apart (`no_hub` or `unavailable`, task 07 step 6), and task 16's Ship bar gives `unavailable` its own sentence (plan re-review b05 R-4) |
   | `Branch.ReadError != ""` | `branch_unreadable` | phase 3's sentence |
   | `Upstream == ""` | `never_pushed` | *"This branch has never been pushed, so Panacea has nothing to build."* |
   | `Ahead > 0` | `unpushed` | *"N commits are not pushed, so they would not ship. Push first."* (`details.ahead`) |
   | `Behind > 0` | `behind` | *"The remote has N commits this clone has not fetched. Fetch, check them, then ship."* (`details.behind`) |
   | `Commit != Tip` | `stale_commit` | *"The branch has moved since this screen was drawn. Look again, then ship."* |
   | `!deploy.ValidCommit(Commit)` | `bad_commit` | *"That is not a full commit id."* |

   All map to `409 deploy_precondition` with `details.reason`.
4. **Environment** — `env, err := Environments.EnvironmentFor(ref, Branch.Name)`: Panacea's own
   name, read now, before anything is reserved (review b02, C2). It is **never** looked up by
   comparing `Branch.Name` with an `environments[]` row: Panacea's rows carry the environment key
   (`feat-x` for `feat/x`, `projects.go:2109-2118`), so that comparison refuses almost every real
   branch. A branch never deployed still gets its name, so a first Ship needs no Hub change.
   - `err` → mapped by the same Kind → code table as step 6 (`503 upstream_unavailable` with
     `details.upstream "panacea"`, `deploy_token_rejected`, `deploy_forbidden`, …), **except**
     `KindNotFound` and `KindRejected`, which answer `409 deploy_environment_unreadable` with
     Panacea's message verbatim (task 03 rewrites this route's 404 to *"Panacea could not read
     this branch name. It may contain `/`…"*). `Environments` has already found the project, so
     `404 deploy_not_found` here would blame a project that exists (b02 round 2, R2). Nothing was
     reserved, so nothing is abandoned.
   - `env` fails the Hub's §7.9 rule (1–63 of `[a-z0-9-]`; Panacea keeps characters such as `.`,
     `projects.go:3814-3826`) → `409 deploy_environment_invalid`: *"Panacea names this branch's
     environment `<env>`, which the shared release record does not accept (1 to 63 of a–z, 0–9
     and -). Rename the branch to ship it from the cockpit."* The check is the Hub's published
     rule, not Panacea's naming rule, so it is no fourth copy.
   - The reserve's `environment` is `env` **verbatim** (`prod` for a production branch, the same
     label Panacea puts on the deployment — `environmentForBranch`, `clients/pipeline.go:2195-2207`,
     called at `:237`, which its comment says mirrors `mapBranchToEnvWithProd`, *"pinned by the
     parity tests"*); rows are joined to it with
     `SameEnvironment`. The two functions mirror, but their **inputs** differ: the access route
     passes the raw label, the deploy the resolved production branch (task 03's `EnvironmentFor`
     row; b02 round 2, R1). So for an annotation-only or mislabelled project the reserved
     `environment` can differ from the deployment's. Ship does not pre-check it against the
     primary row's `branch`, which is the newest production run's label, not a reliable git name
     (`kubernetes.go:5191-5202`, task 08 rule 1a); the view flags the pair after the deploy
     (task 08, rule 5a).
5. **Reserve** — `Releases.Reserve(HubProjectID, {kind:"run", panaceaProject: <ref>, environment,
   branchRef: Branch.Ref, commit, briefId: null, designKey: DesignKey|null, sourceDeploymentId:
   null})`. Any error → returned as is (the relay rule, Hub contract §4.3). **Ship stops. Panacea is
   never called.**
6. **Deploy** — `Actor.Ship(ref, {Branch.Name, Commit, NoCache})`.
   - **Refused** (`KindForbidden`, `KindNotFound`, `KindConflict`, `KindRejected`, `KindUnauthorized`)
     → `Abandon(release, {reason:"deploy_failed", message: e.Message})`; return the Panacea error.
     Panacea's **402** budget and risk refusals (`projects.go:2856`, `:2916-2918`) arrive here as
     `KindConflict` with their sentence (task 02's table), so a paused budget abandons at once with
     Panacea's reason and answers `409 deploy_conflict`, never `503 upstream_unavailable`
     (review I2).
   - **5xx** (`KindUnavailable` with `Status >= 500`) → `Abandon(…, message: "Panacea could not
     start the build.")`; return the error.
   - **No answer** (`KindUnavailable`, `Status == 0` — timeout, reset) → `Abandon(…, message:
     "Panacea did not answer in time. It may still have started the build; if a run appears, it
     will show with no brief recorded.")`; return the error. This is the one ambiguous case:
     `POST …/deploy` spends up to 15 s creating the PipelineRun (`projects.go:2985-2993`), and a
     run created after our timeout would show unpinned — honestly, because confirming it by
     branch and commit would be pinning by observation (P2).
   - **Any other error** — every `*deploy.Error` the three branches above do not name (task 02's
     table gives `KindUnavailable` with `0 < Status < 500` for a 429 and for its *"any other"*
     row, such as a proxy's 405, 408 or 410), and any error that is not a `*deploy.Error` at all →
     `Abandon(…, message: "Panacea could not start the build.")`; return the error. The branches are
     exhaustive: no error from `Actor.Ship` leaves a reservation to expire in 15 minutes with no
     reason (plan re-review b05 R-2). Panacea's API does not answer 429 on this route
     (`routes.go:1286` is not wrapped by `middleware.Heavy`, whose 429 is `heavy.go:38`; Panacea
     at 8292f3f26); whether an edge in front of it can is unverified.
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
                                  never_pushed | unpushed | behind | stale_commit | bad_commit;
                                  no_hub_project may carry details.cause "key_spelling" (HA7-6)
                                  or "hub_unanswered" (the Hub has not answered for the key)
  409 deploy_environment_invalid  Panacea's name for the branch's environment breaks the Hub's §7.9 rule (step 4)
  409 deploy_environment_unreadable  Panacea did not name the branch's environment (EnvironmentFor 404/422; step 4); nothing reserved
  409 deploy_not_connected · deploy_token_rejected
  403 deploy_forbidden · 404 deploy_not_found · 409 deploy_conflict · 422 deploy_rejected
                                  Panacea refused; the release was abandoned with its sentence
                                  (deploy_conflict includes Panacea's 402 budget and risk refusals)
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
| `TestBudgetRefusalAbandonsWithSentence` | the fake's `Ship` answers `KindConflict`, `Status 402`, *"new deploys are paused: this project has reached its monthly hosting budget; …"* (task 04's `ship-402-budget.json`) → `Abandon` once, `reason "deploy_failed"`, exactly that message; no `Confirm`, no `Track`; the route answers `409 deploy_conflict` with the sentence, and `details.upstream` is absent | a 402 treated as an outage: `503 upstream_unavailable`, the lens switching acts off, and a reservation left to expire after 15 minutes (review I2) |
| `TestTimeoutAbandonsAmbiguously` | Panacea times out → `Abandon` message is the ambiguous sentence; no confirm | confirming a run the control plane never saw named |
| `TestOtherErrorAbandons` | the fake's `Ship` answers `KindUnavailable`, `Status 429` (and, as a second case, a plain `errors.New`) → `Abandon` once, `reason "deploy_failed"`, message *"Panacea could not start the build."*; no `Confirm`, no `Track`; the error is returned (plan re-review b05 R-2) | a step 6 with no catch-all, so a 429 or a proxy's 405 leaves the reservation to expire after 15 minutes with no reason — I2's second half by another status |
| `TestADOKeySpellingNamesTheCause` | a binding `Bound` with `HubProjectID ""`, `HubMiss "key_spelling"`, `RepoKey "kpmgcomau.visualstudio.com/engagement portal/portal"` (a form the Hub refuses under its correction A1) → `GET …/ship-readiness` answers `ready: false`, `reason "no_hub_project"`, the key-spelling sentence with that `RepoKey`, and the sentence contains neither `Azure DevOps` nor `GitHub`; `POST …/ship` answers `409 deploy_precondition`, `details.reason "no_hub_project"`, `details.cause "key_spelling"`; neither the Hub nor Panacea called. `HubMiss "not_linked"` → the *"not a shared project"* sentence and no `cause`. `HubMiss ""` → the *"has not answered"* sentence and `details.cause "hub_unanswered"` (review I6; plan re-review b05 R-3, R-4) | a repository told it is *"not a shared project in the Hub yet"* when the real cause is the spelling split, so nobody knows to fix it; a readiness keyed on `HubMiss != "key_spelling"`, which says *"not a shared project"* about a key the Hub never answered for; a sentence that names Azure DevOps for a GitHub key |
| `TestNeverPushedPrecondition` | `Upstream ""` → `409 deploy_precondition` `never_pushed`; neither Hub nor Panacea called | reserving a release for a commit Panacea cannot clone |
| `TestUnpushedPrecondition` | `Ahead 2` → `unpushed`, `details.ahead "2"` | shipping the upstream while the user thinks they shipped their work |
| `TestStaleCommitPrecondition` | `Commit` ≠ `Tip` → `stale_commit` | shipping a commit other than the one the screen showed |
| `TestShipSlashBranch` | branch `feat/x`, the fake seeded `feat/x → feat-x` and a row `feat-x` (label `feat-x`) → the reserve carries `environment: "feat-x"` and Panacea is called | `row.branch == Branch.Name`, which answered *"never deployed"* for every `/` branch (review C2) |
| `TestShipLongBranch` | branch `lane/7-deploy-connector-watcher` seeded `→ lane-7-deploy-connec` → reserve with `environment: "lane-7-deploy-connec"` | a join that works only for 20 characters or fewer |
| `TestShipProductionBranch` | `main` seeded `→ prod`, row `production` → reserve with `environment: "prod"`, the deployment's own label | reserving with the row name, so the Hub holds `production` for Ships and `prod` for rollbacks |
| `TestShipFirstDeploy` | `feat/new` seeded `→ feat-new` and **no** row → the reserve carries `feat-new`; no refusal | refusing a never-deployed branch, the limitation HA7-1 used to carry |
| `TestShipEnvironmentInvalid` | seeded `release/1.2 → release-1.2` → `409 deploy_environment_invalid`; neither `Reserve` nor `Actor.Ship` called; `EnvironmentFor` unavailable → `503 upstream_unavailable`, no `Reserve` | reserving a name the Hub will refuse, or a read failure that reserves anyway |
| `TestShipEnvironmentUnreadable` | the fake's `EnvironmentFor` for `live-check/p7` answers `KindNotFound` with *"Panacea could not read this branch name…"* → `409 deploy_environment_unreadable` with that message verbatim; neither `Reserve` nor `Actor.Ship` called (b02 round 2, R2) | `404 deploy_not_found`, which tells the user a project they are looking at does not exist |
| `TestShipReservesTheAccessRoutesAnswer` | the fake's `EnvironmentFor("develop") → "develop"` (what Panacea's access route answers for an annotation-only `primary-branch=develop` project) → the reserve's `environment` is `develop` verbatim; Ship makes no other environment read and no comparison with the primary row (b02 round 2, R1: the divergence is pinned here and in task 03, and flagged by task 08 rule 5a after the deploy) | a Ship that "corrects" the name with a copy of `ResolveProductionBranch` |
| `TestShipNeverDerivesTheName` | the fake's `EnvironmentFor` unseeded for the branch → Ship fails with the fake's error; `Reserve` not called | a fallback that computes the name when Panacea has not given one (a fourth `sanitizeBranchEnv`) |
| `TestIdempotentShip` | the same key twice → one reserve, one Panacea call, equal results | a desktop retry after a lost response double-shipping |
| `TestConcurrentShipSameBranch` | two goroutines, one branch → one proceeds, one gets `409 conflict` | two reservations racing one branch |
| `TestConfirmRetriedInBackground` | the Hub fails confirm three times, then succeeds → the route answered `202 confirmed:false`; the fake Hub eventually holds a `confirmed` release; the loop stopped | a Ship that reports success while the release stays reserved forever |
| `TestConfirmStopsAtExpiry` | the Hub answers `409 release_expired` → the loop stops, one error log | a loop that retries past the reservation's life |
| `TestTracksRun` | after success, `Tracker.Track(p, "r1")` and `Nudge(p)` called | a 15 s wait before the lens shows the build |
| `TestReadinessIsShipsPreconditions` | for every row of the step-3 table, `Readiness(…).Reason` equals the `Precondition.Reason` a `Ship` of the same state returns | a second copy of the checks that drifts, so the button is on and the server refuses |
| `TestReadinessReservesNothing` | a ready branch → `Ready true`; the Hub fake saw no `Reserve`, `deploytest.Fake.Calls()` is empty, no mutex is held | a readiness call that reserves, so every lens refresh leaves a release to expire |
| `TestPinPreviewAppliesTheHubRule` | the fake brief has revisions 5 (complete) and 6 (incomplete) → `{pinned, ship_head, bf-…, 5}`; no complete revision → `{none, no_complete_revision}`; no brief → `{none, no_brief}`; Hub down → `PinPreview nil`, `Ready` unchanged | previewing the head instead of the latest complete revision (ADR-H1 Q1), or a Hub outage switching Ship off by itself |
| `TestReadinessRouteIsReadOnly` | `GET …/ship-readiness` is in `catalogReadOnlyRoutes`; the §10a audit passes | a readiness route registered outside the recorder |

#### Steps

- [ ] Tests first, over `deploytest.Fake`, `hubclient.NewFake()` with an injected clock, and a
  table `BranchState`.
- [ ] `ship.go`, `confirm.go`, the route in `deploy_acts.go`; `writeDeployError` (task 05) for
  every error; `*Precondition` → `409 deploy_precondition` with `details.reason`.
- [ ] `Readiness`: factor steps 3 and 4 into one unexported `check(ctx, projectID, branchID)`
  that `Ship` and `Readiness` both call — no second copy. The pin preview is one
  `Briefs.ForBranch` call; its error is logged and yields `PinPreview nil`, never a failure. The
  `GET …/ship-readiness` route and its `catalogReadOnlyRoutes` line. Task 16's Ship bar reads it.
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
