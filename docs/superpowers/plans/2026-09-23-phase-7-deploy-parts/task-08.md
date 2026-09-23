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
// GET /v1/projects/{projectId}/deploy → 200 DeployView
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
  "releasesState": "ok" | "no_hub" | "no_hub_project" | "signed_out" | "unavailable",
  "environments": [ EnvironmentView ],   // Panacea’s order; production is always first (clients/kubernetes.go:5154-5155)
  "unmatched": [ EnvironmentView ],      // deployments whose environment names no row (plan 7P-8); usually []
  "history": [ ReleaseRow ],             // newest "at" first, at most 200
  "runs": { "<run>": RunView }           // runs this user started and the watcher is tracking
}

// EnvironmentView — three facts, never merged (plan §5 rule 9)
{
  "name": "feat-evidence-uplo",
  "branch": "feat/evidence-upload",      // the branch Panacea says it serves (projects.go:2118)
  "branchesSeen": ["feat/evidence-upload"], // distinct Deployment.branch values in this environment's history, newest first
  "collision": false,                    // branchesSeen has more than one entry: two branches share this environment (brief D2 a)
  "primary": false,
  "host": "engagement-portal-feat-evidence-uplo.panacea.kpmg" | null,
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
    "since": "…" | null                  // the deployment's createdAt, or the rollback's confirmedAt
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
  "environment": "feat-evidence-uplo" | null,
  "branch": "feat/evidence-upload" | null,
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
  "isCurrent": false,                    // this row is its environment's current
  "rollbackOf": null | { "sourceDeploymentId": "…", "sourceVersion": "…" | null },
  "refusal": null | { "message": "…" },  // refused rows: the Hub's abandonMessage, which is Panacea's sentence
  "release": ReleasePin | null           // null ⇒ "no brief is recorded for this release" (P9: shown, never hidden)
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

1. **Environment join** is `panacea.SameEnvironment` (task 03) — exact name plus Panacea's own
   `prod` ≡ `production` (`clients/kubernetes.go:4210-4217`). A deployment matching no row goes
   under `unmatched`, grouped by its raw environment name.
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
6. **Current**: the newest deployment, unless a confirmed rollback release for this environment
   has `confirmedAt` later than that deployment's `createdAt` — then `source "rollback"` and the
   rollback's `sourceDeploymentId`. The rollback appears in `history` as its own row with the
   source's release pin (the Hub copied it at reserve, R5).
7. **Pending rows**: a confirmed run release of this Panacea project whose `pipelineRun` matches
   no deployment yet. Removed once the deployment appears (rule 5 then takes over).
8. **Refused rows**: this user's `abandoned` releases with `abandonedAt` in the last 24 h.
   Teammates' refusals are theirs and are not shown.
9. **`branchesSeen` / `collision`** count distinct non-empty `Deployment.branch` values among the
   environment's rows. The comparison is exact string equality — no sanitising, no truncation
   (never a fourth copy of `sanitizeBranchEnv`, survey §3.2).
10. **Times** are the facts' own (`createdAt`, `confirmedAt`, `reservedAt`), RFC 3339 UTC with three
    fractional digits, like the Hub (brief B4: absolute, never relative).
11. **`history` is capped at 200 rows**, newest first; a project with more says so with the
    watcher's `DeploymentsAt` — the desktop virtualises past 50 (DESIGN-SYSTEM.md "Floor").

#### Tests to write first (`view_test.go`)

Every test builds a `deploywatch.Snapshot` by hand and asserts on `Assemble(snapshot, now)`.

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestProdEqualsProduction` | a deployment with `environment "prod"` joins the row named `production`; one with `"prod-eu"` goes to `unmatched` | a lens with an empty production row and an orphan group |
| `TestUnmatchedNeverDropped` | a deployment in environment `"legacy"` with no row appears under `unmatched` | a join that silently discards what it cannot place |
| `TestServingOnlyFromNewest` | the second-newest row's `LiveReady: &3` does not reach `serving`; the newest's nil → `measured:false` | painting yesterday's health on today |
| `TestServingZeroIsDown` | newest `LiveReady: &0` → `{measured:true, ready:0}` while `build.state == "succeeded"` | a green tick over a dead site (survey §3.3) |
| `TestReleaseByRunNameOnly` | a confirmed release whose `pipelineRun` is `"r1"`; deployments `"r1"` and `"r2"` on the same branch → only `r1` has the pin; `r2.release == null` | attaching a pin to the next deploy on the same branch — pinning by observation (P2) |
| `TestReservedIsNotAPin` | a `reserved` (unconfirmed) release with `pipelineRun: null` → no deployment gets it | a reservation that never became a run claiming a deployment |
| `TestRollbackIsCurrent` | newest deployment `d2` at 10:00; a confirmed rollback to `d1` confirmed 10:05 → `current {source:"rollback", deploymentId:"d1"}`; a `rollback` row carries `d1`'s release pin | the lens saying `d2` is serving after the user rolled back |
| `TestRollbackBeforeNewerDeploy` | the same rollback, then `d3` at 10:10 → `current` is `d3` | a rollback that sticks after a newer ship |
| `TestPendingRowUntilDeploymentAppears` | a confirmed release for run `r9` with no deployment → a `pending` row; add `r9` → the pending row is gone and `r9` has the pin | a Ship that shows nothing for the ~seconds before Panacea lists the run |
| `TestRefusedOnlyMineAndRecent` | my abandon 1 h ago → shown with its message; my abandon 25 h ago and a teammate's → absent | a teammate's failure appearing in my history |
| `TestCollisionExactStrings` | two deployments in one environment from `feat/phase-7-deploy-connector-a` and `…-b` → `collision: true`, both in `branchesSeen`; the same branch twice → `false` | detecting collision by comparing sanitised names — a fourth copy of the rule |
| `TestEmptyVersionIsNull` | `Version: ""` → `"version": null` | a blank cell that reads like zero |
| `TestNotConnectedIs200` | connection `not_connected` → the handler answers 200 with empty lists and the connection object | a 409 the lens would have to turn back into an onboarding screen |
| `TestViewHandlerMakesNoCall` | the handler test's `Deployer` and `Releases` fakes record zero calls during the `GET` | a GET that fetches from Panacea on the request path |

`catalogReadOnlyRoutes` line: `"GET /v1/projects/{projectId}/deploy": "Assembles the watcher's
in-memory snapshot. No network call, no write; the watcher's own cycle is on its own timer."`

#### Steps

- [ ] Tests first; `view.go`; the handler; register through `routes`.
- [ ] Run the control-plane suite; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/deploywatch/ ./control-plane/internal/api/ -race -count=1
→ ok
```

#### What this task does NOT claim

- That `Deployment.branch` carries the real branch name with its `/`. It is read from the
  PipelineRun **label** `panacea.io/branch` (`clients/kubernetes.go:5750`), and a Kubernetes label
  value cannot contain `/`; yet `Redeploy`'s comment calls the row's branch *"the real branch"*
  (`projects.go:1803-1805`). **Unverified.** If the label is sanitised, `branchesSeen` still
  detects collisions (two distinct sanitised values), but the desktop's "this branch's environment"
  must come from `environments[].branch`, which is what task 15 uses. Task 20 records the answer.
- Anything about how a pinned revision reads. The view carries `briefKey` and nothing of the brief;
  the text comes from phase 4's loader (R6) in the desktop.
