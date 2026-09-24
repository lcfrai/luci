### Task 08: The view — `GET /v1/projects/{projectId}/deploy`

**Half:** 7a. **Hours:** 3. **Depends on:** 07.

The one response the Deploy lens renders from. It is **assembled from the watcher's snapshot in
memory** — the handler makes no network call — so it answers in microseconds and is safe to fetch
on every cursor change. Every field names the fact it stands on (`AGENTIC-UX.md` §2 property 7:
*if you cannot name the fact that justifies an element, the element must not exist*).

**Files:**
- `control-plane/internal/deploy/deploywatch/view.go` (new) — `View`, `EnvironmentView`, `ReleaseRow`, `ReleasePin`, `Assemble`
- `control-plane/internal/deploy/deploywatch/view_test.go` (new)
- `control-plane/internal/api/deploy_view.go` (new) — the route
- `control-plane/internal/api/deploy_view_test.go` (new)
- `control-plane/internal/api/api.go` (modified) — `Deps.DeployWatcher`, `registerDeployView`
- `control-plane/internal/api/routes_audit_test.go` (modified) — one line

#### The wire — complete

```jsonc
// GET /v1/projects/{projectId}/deploy[?branch=<git branch>] → 200 DeployView
// ?branch= is the lens's selected branch, verbatim ("feat/evidence-upload"). The handler calls
// Watcher.WantBranch with it (a map write, no request) and answers from BranchEnvs.
// 404 project_not_found (phase 2's code) for an unknown local project id. Every other state —
// not connected, not bound, Panacea down, Hub down — is a 200 whose fields say so: the lens has a
// sentence for each, and a problem would give it none.
{
  "projectId": "p-8c1d0e42",
  "connection": ConnectionStatus,        // exactly GET /v1/deploy/connection's body (task 05)
  "binding": Binding,                    // exactly the binding PUT's body (task 06)
  "fetchedAt": "2026-09-23T10:02:11.123Z" | null,   // the last cycle whose Panacea reads all succeeded
  "stale": false,                        // true: the last cycle failed; everything below is fetchedAt old
  "staleReason": null | { "code": "upstream_unavailable" | "deploy_token_rejected", "message": "…" },
  "ownerControls": "yes" | "no" | "unknown",
  "releasesState": "ok" | "no_hub" | "no_hub_project" | "hub_key_spelling" | "signed_out" | "unavailable",
  "selected": null | {                   // null when no ?branch= was sent (review b02, C2)
    "branch": "feat/evidence-upload",    // ?branch=, echoed
    "target": "feat-evidence-upload" | null,      // Panacea's OWN environment name for the branch (Reader.EnvironmentFor); null = not resolved yet, or refused (see error)
    "environment": "feat-evidence-upload" | null, // the `name` of the environments[] row SameEnvironment joins `target` to; null = no row (never deployed) or not resolved
    "error": null | { "code": "not_found" | "rejected" | "forbidden", "message": "…" }  // Snapshot.BranchEnvErrs[branch]: Panacea refused to name this branch's environment; target is then null (b02 round 2, R2)
  },
  "environments": [ EnvironmentView ],   // Panacea’s order; production is always first (clients/kubernetes.go:5154-5155)
  "unmatched": [ EnvironmentView ],      // deployments whose environment names no row (plan 7P-8), one group per raw name; a legacy deployment with environment "" that rule 1b cannot join is in ONE group whose name is "" verbatim (the desktop titles it "no environment recorded", task 15); usually []
  "history": [ ReleaseRow ],             // newest "at" first, at most 200
  "runs": { "<run>": RunView }           // runs this user started and the watcher is tracking
}

// EnvironmentView — three facts, never merged (plan §5 rule 9)
{
  "name": "feat-evidence-upload",        // 20 characters: sanitizeBranchEnv cuts only past 20 (projects.go:3818-3820)
  "branch": "feat-evidence-upload",      // Panacea's row LABEL (projects.go:2118): == name for every non-base row, the real branch only for production (:2109-2111). Display only; never a join key, save rule 1b's one case (a deployment with environment "", joined by its run label as Panacea does)
  "branchesSeen": ["feat-evidence-upload"], // distinct Deployment.branch values in this environment's history, newest first — Panacea's run LABELS as written, never normalised (a Ship: sanitizeLabelValue, / and _ → -, pipeline.go:609, :2351-2364; a push: sanitizeLabel, keeps _, webhook-receiver trigger.go:1270, :1553-1574), not git branch names
  "collision": false,                    // branchesSeen has more than one entry: more than one LABEL in this environment — two branches, or one branch pushed and Shipped (rule 9; brief D2 a)
  "primary": false,
  "host": "engagement-portal-feat-evidence-upload.panacea.kpmg" | null,
  "status": "running",                   // Panacea's word, verbatim, never interpreted
  "replicas": { "ready": 1, "desired": 1 } | null,   // null = not measured (projects.go:2125-2140)
  "build": {                             // FACT 1: the newest deployment's pipeline outcome
    "state": "succeeded" | "failed" | "running" | "pending" | "historical" | "unknown" | "none",
    "deploymentId": "engagement-portal-build-x7k2p" | null
  },
  "serving": {                           // FACT 2: pods, now — from the newest deployment's liveReadyReplicas
    "measured": true,
    "ready": 0                           // present only when measured; 0 means measured and DOWN
  },
  "scans": {                             // FACT 3: the security report for the newest deployment
    "state": "reported" | "not_run" | "not_read",
    "overall": "FAIL" | "INCOMPLETE" | "WARN" | "PASS" | "SKIPPED" | null,
    "blocking": [ { "scan": "trivy", "reason": "2 CRITICAL" } ],
    "incomplete": [ { "scan": "dast", "reason": "scanner errored" } ],
    "detailsSource": "live" | "snapshot" | "mixed" | "unverified" | "none" | null
  },
  "current": {                           // what the environment serves now (plan 7P-4)
    "source": "deployment" | "rollback" | "none",
    "deploymentId": "…" | null,          // for "rollback": the deployment rolled back TO
    "since": "…" | null                  // the deployment's completedAt (createdAt + duration), or the rollback's confirmedAt (rule 6)
  }
}

// ReleaseRow — one row of history
{
  "kind": "run" | "rollback" | "pending" | "refused",
  //   run      — a Panacea deployment (every row Panacea lists)
  //   rollback — a confirmed Hub rollback release; Panacea records no row for it (7P-4)
  //   pending  — a confirmed Hub run release whose deployment has not appeared yet
  //   refused  — one of THIS user's releases abandoned in the last 24 h, with Panacea's sentence
  "deploymentId": "engagement-portal-build-x7k2p" | null,   // null for rollback and refused
  "environment": "feat-evidence-upload" | null,     // the row's raw environment, verbatim ("prod" for a production deployment: environmentForBranch, clients/pipeline.go:2195-2207, called at :237)
  "environmentRow": "feat-evidence-upload" | null,  // the `name` of the environments[] row that `environment` joins by SameEnvironment ("production" for "prod"); null = the row sits under `unmatched`. The desktop looks environments up by THIS, never by `environment` (review I8)
  "branch": "feat-evidence-upload" | null,  // run rows: Panacea's run label, verbatim (a Ship's sanitizeLabelValue, pipeline.go:609, or a push's sanitizeLabel, trigger.go:1270; plan §12); Hub rows: the release's branchRef without refs/heads/
  "commit": "4e7a91c0000000000000000000000000000000aa" | null,
  "version": "manual-4e7a91c0" | null,   // null → the lens says "version unknown", never blank
  "build": "succeeded" | "failed" | "running" | "pending" | "historical" | "unknown" | null,
  "buildRaw": "Succeeded" | null,
  "failedStep": "" , "errorText": "",
  "at": "…",                             // Deployment.createdAt; rollback: confirmedAt; pending/refused: reservedAt
  "deployedBy": "",                      // Panacea's; "" = not recorded
  "trigger": "manual" | "git-push" | "redeploy" | "promote" | "prod-branch-change" | "",
  "reconstructed": false,
  "gates": { "trivy": { "verdict": "failed", "criticals": 2, "highs": null, "findings": null } } | null,
  "provisioning": [ { "component": "database", "status": "failed", "detail": "…" } ],
  "isCurrent": false,                    // this row is its environment's current (rule 6): the run row of the current succeeded deployment, or the current rollback row — never the run row a rollback returned to
  "rollbackOf": null | { "sourceDeploymentId": "…", "sourceVersion": "…" | null },
  "refusal": null | { "message": "…" },  // refused rows: the Hub's abandonMessage, which is Panacea's sentence
  "release": ReleasePin | null,          // null ⇒ "no brief is recorded for this release" (P9: shown, never hidden)
  "environmentMismatch": null | { "recorded": "develop", "deployed": "prod" }
  // run rows only (rule 5a, b02 round 2, R1): the joined Hub release's `environment` and this
  // deployment's `environment` are not SameEnvironment — the Hub recorded the release against one
  // environment and Panacea deployed it to another. `environment`/`environmentRow` above are the
  // DEPLOYMENT's, never the Hub's
}

// ReleasePin — from the Hub Release (Hub contract §7.9), verbatim fields only
{
  "releaseId": "rl-3f2a91c07b1e4d2a",
  "state": "reserved" | "confirmed" | "abandoned" | "expired",
  "pin": {
    "status": "pinned" | "none",
    "reason": "ship_head" | "inherited" | "no_brief" | "no_complete_revision" | "source_unpinned" | "source_unknown",
    "briefKey": "brief:bf-3f2a91c07b1e4d2a@6" | null,   // passed to phase 4's loader; NEVER parsed here or in the desktop (P5)
    "briefDigest": "sha256:…" | null,
    "headRevisionAtShip": 6 | null
  },
  "designKey": "design:cv-7c01d2e3f4a5b6c7@11" | null,
  "reservedBy": { "id": "u-…", "displayName": "Dylan Mackie" },
  "reservedAt": "…"
}

// RunView — a tracked run
{
  "run": "engagement-portal-build-x7k2p",
  "build": "running",
  "startedAt": "…" | null,
  "currentStep": "build-kaniko" | "",
  "completedSteps": ["fetch-source", "detect"],   // no total, no bar: Panacea does not say what remains
  "failedStep": "", "errorText": "",
  "ignoredFailures": []
}
```

#### Assembly rules (`view.go`) — each is a test below

1. **Environment join** is `deploy.Reader.SameEnvironment` — for Panacea (task 03), exact name plus
   Panacea's own `prod` ≡ `production` (`clients/kubernetes.go:4210-4217`). `deploywatch` never
   imports `deploy/panacea`, so the rule reaches the view as a parameter:
   **`Assemble(s Snapshot, same func(a, b string) bool, now time.Time) View`**, and the watcher
   passes its `Reader.SameEnvironment` (b02 round 2, R4). A deployment matching no row goes
   under `unmatched`, grouped by its raw environment name. **Every `ReleaseRow` carries the
   joined row's `name` as `environmentRow`** (Hub rows join their release's `environment` the
   same way), so no consumer joins again (review I8).
1a. **Selected branch** (review b02, C2): with `?branch=b`, `selected.target` is
   `Snapshot.BranchEnvs[b]` (absent → `null`), and `selected.environment` is the `name` of the
   row `SameEnvironment(target, row.Name)` holds for, else `null`. `selected.error` is
   `Snapshot.BranchEnvErrs[b]` (task 07), with the code of its `deploy.Kind` and Panacea's message
   verbatim, else `null`; a branch has an error or a target, never both. A git branch is **never**
   compared with a row's `name` or `branch`, nor with `Deployment.branch`: Panacea's labels are
   `/`-free and cut at 20 (environment) or 63 (run label) characters, and whether `_` survives
   depends on the writer (the environment and a Ship's run label fold it into `-`; a push's run
   label keeps it: rule 9, plan §12), so no string comparison with the git name can be right, and
   deriving the label would be a fourth copy of Panacea's rules.
1b. **No environment recorded** (review M5). A deployment whose `Environment` is `""` (a legacy
   run with no `panacea.io/environment` label, read back as `""` at `clients/kubernetes.go:5681`;
   every current writer sets one, task 12) is joined **by its run label**: to the row whose
   `branch` equals its `Deployment.branch` exactly, byte for byte, with no normalising. That is
   Panacea's own attribution: its environments list indexes runs by `panacea.io/branch` and looks
   each row up by its branch key (`kubernetes.go:5396-5415`, `:5455`), so the row already counts
   that run in its `status`. It compares two Panacea labels, never a git branch (rule 1a). The
   joined `ReleaseRow` keeps `environment:""` and carries `environmentRow` as usual. A deployment
   with `""` whose label matches no row's `branch`, or is `""` too, goes to **one** `unmatched`
   group whose `name` is `""`, verbatim; the desktop titles it *"no environment recorded"*
   (task 15), and its rows carry `environmentRow:null`. It never joins a row by `SameEnvironment`,
   which is false for `""` against every name (task 03). Panacea's `tenantNamespace` does send `""`
   to the base namespace (`kubernetes.go:4213`; the scan read too, `handlers/tier_store.go:106-113`),
   but that is a default for a missing argument: the environments list, where the rows come from,
   attributes the run by its label, so a feature branch's legacy run is never production's.
2. **Newest per environment** is the first deployment in Panacea's newest-first list whose
   environment joins. Only it contributes `serving` and `scans`; historical rows carry `gates` as
   recorded and **never** `serving` (`models/models.go:720-722`).
3. **Serving** is `{measured:false}` when the newest deployment's `LiveReady` is nil — also when
   there is no deployment. It is never inferred from `replicas`, `status` or the build.
4. **Scans** are `not_read` until the watcher has a report for the newest deployment's id;
   `not_run` when the report's `Scans` is empty and `Overall` is `SKIPPED`; else `reported`.
5. **Release for a deployment**: the Hub release with `kind "run"`, `state "confirmed"` and
   `pipelineRun == deployment.id`; else one whose `deployments[]` holds the id; else `null`. **Never**
   a release chosen by branch, environment or time — that would be pinning by observation (P2).
5a. **Recorded against another environment** (b02 round 2, R1): when rule 5 joins a release to a
   deployment and `!same(release.environment, deployment.Environment)`, the row carries
   `environmentMismatch {recorded: release.environment, deployed: deployment.Environment}`. The
   row's `environment` and `environmentRow` stay the deployment's — the label the deploy really
   wrote (`clients/pipeline.go:608`) — and the release is never joined by the Hub's
   `environment`, so it does not appear as the current release of an environment it never
   reached. This is how phase 7 sees Panacea's access route disagree with its deploy (task 03's
   `EnvironmentFor` row: `branchEnv` reads the production-branch label only), from facts after the
   deploy, with no copy of `ResolveProductionBranch`.
6. **Current** is what the environment serves now (review I3). It is the later of two candidates:
   - **the live build**: the environment's deployment with `build "succeeded"` and a non-zero
     `CompletedAt` whose `CompletedAt` is latest. Ordered by completion, never by `CreatedAt`:
     `createdAt` is PipelineRun creation (`clients/kubernetes.go:5680`, `:5754`), so a build
     Shipped before a rollback can go live after it. A `failed`, `running`, `pending`,
     `historical` or `unknown` deployment is **never** current, however new: a failed build left
     the old pods serving, and a running one has not replaced them yet. `build` (FACT 1) still
     reports the newest deployment, so the card can say both *"the newest build failed"* and
     what is serving.
   - **the rollback**: the confirmed rollback release, joined to this environment by `same`,
     whose `confirmedAt` is latest.
   The later wins: `{source:"deployment", deploymentId, since: CompletedAt}` or
   `{source:"rollback", deploymentId: sourceDeploymentId, since: confirmedAt}`. An equal instant
   goes to the rollback (Panacea rounds `duration` to a second; the Hub confirms after Panacea's
   flip). Neither candidate → `{source:"none", deploymentId:null, since:null}`, never a guess;
   an environment whose only rows are backfilled `historical` rows reads `none`.
   `isCurrent` is `true` on exactly one row per environment: the current deployment's `run` row,
   or the current `rollback` row. The `run` row of the deployment rolled back **to** stays
   `false` (its build is history); task 16 hides Roll back on it through `current.deploymentId`.
   The rollback appears in `history` as its own row with the source's release pin (the Hub copied
   it at reserve, R5).
7. **Pending rows**: a confirmed run release of this Panacea project whose `pipelineRun` matches
   no deployment yet. Removed once the deployment appears (rule 5 then takes over).
8. **Refused rows**: this user's `abandoned` releases with `abandonedAt` in the last 24 h.
   Teammates' refusals are theirs and are not shown.
9. **`branchesSeen` / `collision`** count distinct non-empty `Deployment.branch` values among the
   environment's rows. The comparison is exact string equality — no sanitising, no truncation
   (never a fourth copy of `sanitizeBranchEnv`, survey §3.2; `_` is never folded into `-`). The
   values are Panacea's run labels, so they are compared only with each other, never with a git
   branch name. Panacea has **two** label writers (plan §12): the API's `sanitizeLabelValue`
   (`clients/pipeline.go:609`, `:2351-2364`) turns `_` into `-`; the webhook-receiver's
   `sanitizeLabel` (`webhook-receiver/pipeline/trigger.go:1270`, `:1553-1574`) keeps `_`. So
   `collision` means "more than one **label**", never "more than one branch" (re-review b03, I-1):
   - **False positive:** one branch `feat_x`, pushed (label `feat_x`) and then Shipped (label
     `feat-x`), gives `branchesSeen` `["feat-x","feat_x"]` and `collision` `true`. Task 15's banner
     words this case as possible.
   - **Blind spot (b02 round 2, R7):** git branches that differ only in `/` versus `_` (or `-`)
     can still share one label. `feat/x` and `feat_x`, both Shipped, both read `feat-x`
     (`sanitizeLabelValue`); `feat/x` pushed and `feat_x` Shipped also both read `feat-x`. They
     share the environment `feat-x` too, so `branchesSeen` holds one entry and `collision` is
     `false` while the two overwrite each other. Panacea records nothing that tells them apart,
     and deriving it would copy its rule, so the view cannot see these twins; task 15 says so
     under the banner.
10. **Times** are the facts' own (`createdAt`, `completedAt`, `confirmedAt`, `reservedAt`), RFC 3339 UTC with three
    fractional digits, like the Hub (brief B4: absolute, never relative).
11. **`history` is capped at 200 rows**, newest first; a project with more says so with the
    watcher's `DeploymentsAt` — the desktop virtualises past 50 (DESIGN-SYSTEM.md "Floor").

#### Tests to write first (`view_test.go`)

Every test builds a `deploywatch.Snapshot` by hand and asserts on `Assemble(snapshot, same, now)`.
`same` is a test stub that holds exact equality plus the one pair `prod` ≡ `production`, quoting
`clients/kubernetes.go:4210-4217` (`tenantNamespace` sends both to the base namespace). The
Fake's exact-only `SameEnvironment` (task 03) is not used here, because it cannot pass the `prod`
tests (b02 round 2, R4). `TestWatcherPassesSameEnvironment` (task 07's watcher test file) pins that
the watcher passes its `Reader.SameEnvironment`, not the stub.

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestProdEqualsProduction` | a deployment with `environment "prod"` joins the row named `production`; one with `"prod-eu"` goes to `unmatched` | a lens with an empty production row and an orphan group |
| `TestSelectedSlashBranch` | rows `production` (label `main`) and `feat-x` (label `feat-x`); `BranchEnvs{"feat/x": "feat-x"}`; `?branch=feat/x` → `selected {branch:"feat/x", target:"feat-x", environment:"feat-x"}` | `row.branch == branch` or `row.name == branch`, which finds nothing for `feat/x` (review C2) |
| `TestSelectedLongBranch` | `BranchEnvs{"lane/7-deploy-connector-watcher": "lane-7-deploy-connec"}` and a row `lane-7-deploy-connec` → `selected.environment == "lane-7-deploy-connec"` | a join that works only for names of 20 characters or fewer |
| `TestSelectedProduction` | `BranchEnvs{"main": "prod"}` and the row `production` → `selected {target:"prod", environment:"production"}` | an exact join that leaves the production branch with no card |
| `TestSelectedNeverDeployed` | `BranchEnvs{"feat/new": "feat-new"}`, no such row → `selected {target:"feat-new", environment:null}`; `BranchEnvs` without the branch → `target:null` | reading "no row" as "not resolved", or the reverse |
| `TestReleaseRowCarriesItsRow` | a deployment labelled `prod` → its `ReleaseRow` has `environment:"prod"`, `environmentRow:"production"`; a confirmed rollback release with `environment:"prod"` → the `rollback` row has `environmentRow:"production"`; an `unmatched` deployment → `environmentRow:null` | a desktop left to re-join `prod` to `production` by exact name, losing *Why it is down* and *Follow readiness* (review I8) |
| `TestUnmatchedNeverDropped` | a deployment in environment `"legacy"` with no row appears under `unmatched` | a join that silently discards what it cannot place |
| `TestNoEnvironmentJoinsByLabel` | rows `production` (`branch "main"`) and `feat-evidence-upload` (`branch "feat-evidence-upload"`, the name `sanitizeBranchEnv` gives `feat/evidence-upload`). Deployments with `environment ""`: label `main` → joins `production` (`environmentRow:"production"`, `environment:""` kept); label `feat-evidence-upload` → joins that row; labels `feat_evidence_upload` (a push's label, `_` kept) and `""` → both in **one** `unmatched` group with `name ""`, `environmentRow:null`; nothing joins by `SameEnvironment` | a group titled `""` per deployment, a legacy run dropped, a label normalised before the compare (a fourth copy of Panacea's rules), or `""` read as production (review M5) |
| `TestServingOnlyFromNewest` | the second-newest row's `LiveReady: &3` does not reach `serving`; the newest's nil → `measured:false` | painting yesterday's health on today |
| `TestServingZeroIsDown` | newest `LiveReady: &0` → `{measured:true, ready:0}` while `build.state == "succeeded"` | a green tick over a dead site (survey §3.3) |
| `TestReleaseByRunNameOnly` | a confirmed release whose `pipelineRun` is `"r1"`; deployments `"r1"` and `"r2"` on the same branch → only `r1` has the pin; `r2.release == null` | attaching a pin to the next deploy on the same branch — pinning by observation (P2) |
| `TestReservedIsNotAPin` | a `reserved` (unconfirmed) release with `pipelineRun: null` → no deployment gets it | a reservation that never became a run claiming a deployment |
| `TestRollbackIsCurrent` | `d1` succeeded (created 09:00, completed 09:04); `d2` succeeded (created 10:00, completed 10:03); a confirmed rollback to `d1` confirmed 10:05 → `current {source:"rollback", deploymentId:"d1", since:"…10:05…"}`; the `rollback` row has `isCurrent:true` and `d1`'s release pin; the `d1` and `d2` run rows have `isCurrent:false` | the lens saying `d2` is serving after the user rolled back |
| `TestRollbackBeforeNewerDeploy` | the same rollback, then `d3` created 10:10 and completed 10:14, succeeded → `current {source:"deployment", deploymentId:"d3", since:"…10:14…"}`; only `d3`'s row is `isCurrent` | a rollback that sticks after a newer build went live |
| `TestFailedNewestIsNotCurrent` | `d1` succeeded (completed 09:04); `d2` created 10:00, `build "failed"`, `CompletedAt` 10:02 → `current {source:"deployment", deploymentId:"d1", since:"…09:04…"}`, `d1` `isCurrent:true`, `d2` `isCurrent:false`, `build.state "failed"`; the same with `d2` `running` (zero `CompletedAt`) → still `d1` (review I3) | the newest deployment read as current, which marks a failed build as serving and hides *Roll back* on the one that is |
| `TestBuildLiveAfterRollbackIsCurrent` | `d1` succeeded (completed 09:04); `d2` created 10:00; a rollback to `d1` confirmed 10:05; `d2` succeeded with `CompletedAt` 10:08 → `current {source:"deployment", deploymentId:"d2", since:"…10:08…"}`; with `d2` still `running` at the same snapshot → `current` is the rollback (review I3) | ordering by `CreatedAt`, which says the rollback is serving after the Ship it undid went live on top of it |
| `TestCurrentOrdersByCompletion` | `d1` created 10:00 and completed 10:20, `d2` created 10:05 and completed 10:10, both `succeeded`, no rollback, listed in Panacea's served order (`d2` first, newest by creation) → `current {source:"deployment", deploymentId:"d1", since:"…10:20…"}`, only `d1` `isCurrent` (plan re-review b04-1, I-2: Panacea's `cancel-superseded` fires only within one trigger path, `clients/pipeline.go:102-112` *"CROSS-PATH RESIDUAL"*, so a push run and a Ship can overlap on one environment and the one created first can finish last) | taking the first `succeeded` row in served order (newest by creation) and only then comparing its `CompletedAt` — which passes every other test here, because in them creation order and completion order agree |
| `TestCurrentTieGoesToRollback` | `d2` `CompletedAt` 10:05:00 and a rollback to `d1` confirmed 10:05:00.000 → `source "rollback"`; `d2` `CompletedAt` 10:05:01 → `d2` | a tie broken by list order, so the answer flips between cycles |
| `TestHistoricalOnlyIsNone` | the environment's only deployment is a backfilled `historical` row (zero `CompletedAt`) and no rollback → `current {source:"none", deploymentId:null, since:null}`, no row `isCurrent` | calling a reconstructed row live because it is the only one |
| `TestPendingRowUntilDeploymentAppears` | a confirmed release for run `r9` with no deployment → a `pending` row; add `r9` → the pending row is gone and `r9` has the pin | a Ship that shows nothing for the ~seconds before Panacea lists the run |
| `TestRefusedOnlyMineAndRecent` | my abandon 1 h ago → shown with its message; my abandon 25 h ago and a teammate's → absent | a teammate's failure appearing in my history |
| `TestCollisionExactStrings` | two deployments in one environment (`feat-phase-7-deploy`: the 20-character cut leaves a trailing `-`, which `TrimRight` removes) whose `Deployment.branch` labels are `feat-phase-7-deploy-connector-a` and `feat-phase-7-deploy-connector-b` — the run labels Panacea writes for `feat/phase-7-deploy-connector-a`/`-b`, never the git names (b02 round 2, R7) → `collision: true`, both in `branchesSeen`; the same label twice → `false` | detecting collision by comparing sanitised names — a fourth copy of the rule |
| `TestCollisionKeepsPushLabel` | two deployments in environment `feat-x`, labels `feat_x` (a push: the webhook's `sanitizeLabel` keeps `_`, `trigger.go:1270`) and `feat-x` (a Ship) → `collision: true`, `branchesSeen` holds both strings exactly, `_` intact (re-review b03, I-1) | a view that folds `_` into `-` before counting — a copy of `sanitizeLabelValue`, and one that hides which writer made each label |
| `TestSelectedRefused` | `BranchEnvErrs{"live-check/p7": {Kind: KindNotFound, Message: "Panacea could not read this branch name…"}}` → `selected {target:null, environment:null, error:{code:"not_found", message:<verbatim>}}`; a branch in neither map → `error:null, target:null` (b02 round 2, R2) | a refusal drawn as *"Asking Panacea…"* for ever |
| `TestReleaseRecordedAgainstAnotherEnvironment` | a confirmed run release with `environment:"develop"` and `pipelineRun:"r1"`; deployment `r1` labelled `prod`; row `production` → the `r1` row has `environment:"prod"`, `environmentRow:"production"`, `environmentMismatch:{recorded:"develop", deployed:"prod"}`, and the release pin; a release `environment:"production"` on a `prod` deployment → `environmentMismatch:null` (b02 round 2, R1: an annotation-only production branch, where the access route answers `develop` and the deploy lands in `prod`) | trusting the Hub's environment over the deployment's, so a release shows as current in an environment it never reached |
| `TestEmptyVersionIsNull` | `Version: ""` → `"version": null` | a blank cell that reads like zero |
| `TestNotConnectedIs200` | connection `not_connected` → the handler answers 200 with empty lists and the connection object | a 409 the lens would have to turn back into an onboarding screen |
| `TestViewHandlerMakesNoCall` | the handler test's `Deployer` and `Releases` fakes record zero calls during the `GET`, including `GET …/deploy?branch=feat/x`, which records one `WantBranch(p, "feat/x")` on the watcher and no `EnvironmentFor` | a GET that fetches from Panacea on the request path |

`catalogReadOnlyRoutes` line: `"GET /v1/projects/{projectId}/deploy": "Assembles the watcher's
in-memory snapshot. No network call and no stored write: ?branch= only adds the branch to the
watcher's in-memory wanted set, which its own cycle resolves on its own timer."`

#### Steps

- [ ] Tests first; `view.go`; the handler; register through `routes`.
- [ ] Run the control-plane suite; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/deploywatch/ ./control-plane/internal/api/ -race -count=1
→ ok
```

#### What this task does NOT claim

- That `Deployment.branch` carries the real branch name with its `/`. **Settled in review b02: it
  does not.** It is read from the PipelineRun **label** `panacea.io/branch`
  (`clients/kubernetes.go:5750`), written by two writers that disagree (plan §12; re-review b03,
  I-1): the API writes `sanitizeLabelValue(branch)` (`clients/pipeline.go:609`; the function at
  `:2351-2364` turns `/` and `_` into `-`, drops other characters and cuts at 63), and the
  webhook-receiver writes `sanitizeLabel(event.Branch)` (`webhook-receiver/pipeline/trigger.go:1270`;
  `:1553-1574` turns `/` into `-` but keeps `_`). Panacea's own comment says the portal chip
  *"carries the DNS-sanitized panacea.io/branch LABEL (slash→dash)"* (`kubernetes.go:3052`).
  `Redeploy`'s *"the real branch"* (`projects.go:1803-1805`) is wrong for slash branches.
- That `collision` counts branches. It counts distinct labels (rule 9): one branch pushed and then
  Shipped can show two, and two branches can show one. "This branch's environment" comes from
  `selected` (rule 1a), never from `environments[].branch` or `Deployment.branch`.
- That `CompletedAt` is the instant the new pods took traffic (rule 6, review I3). It is when the
  PipelineRun finished, and Panacea runs some tasks *"beside/after the deploy"*
  (`clients/pipeline.go:1302-1306`), so it can lag go-live. A rollback confirmed inside that lag
  reads as undone by the build although the rollback flipped last. And a run whose rollout
  succeeded but whose status is `Failed` because of a later task would read as not current while
  its pods serve. Whether any such task can fail the run is **unverified** (it turns on each
  task's `onError`). FACT 2 (`serving`) still shows the pods either way; Panacea records nothing
  closer to go-live on a deployment row.
- That rule 6 orders a build and a rollback correctly when they land within clock skew of each
  other (plan re-review b04-1, M-3). `CompletedAt` comes from the Kubernetes API server's clock
  (`creationTimestamp` and `completionTime`); `confirmedAt` comes from the Hub's. The tie rule
  covers only an exact tie. The skew is unmeasured. The rule compared `createdAt` with
  `confirmedAt` across the same two clocks before b04, so this is not new.
- That `CompletedAt` is exact for every row (plan re-review b04-1, M-4). `CreatedAt + duration` is
  exact only when `CreatedAt` is the PipelineRun's creation time. Panacea's store keeps the
  **earliest** non-zero `CreatedAt` when it merges observations (`clients/deployment_store.go:56`,
  `:88`), and a backfilled row takes its `CreatedAt` from the release's `MintedAt`
  (`clients/deployment_backfill.go:172`). A backfilled row later merged with a live observation
  keeps `MintedAt`, while `duration` runs from `creationTimestamp`, so its `CompletedAt` is early
  by the minting-to-creation gap. The size of that gap is unverified (likely seconds); it matters
  only for a build and a rollback that land that close together.
- Anything about how a pinned revision reads. The view carries `briefKey` and nothing of the brief;
  the text comes from phase 4's loader (R6) in the desktop.
