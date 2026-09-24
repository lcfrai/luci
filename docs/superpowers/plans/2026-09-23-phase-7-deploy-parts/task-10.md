### Task 10: Rollback, rebuild and cancel

**Half:** 7b. **Hours:** 2.5. **Depends on:** 09.

The three acts that re-run or stop something. Two produce a release and so go through the same
**reserve → act → confirm or abandon** core as Ship; this task extracts that core from task 09 so
there is one implementation of it. Cancel produces no release.

- **Rollback** is ruling R5's `rollback` shape. Its premise is verified (plan 7P-4):
  `RollbackDeployment` creates no PipelineRun and returns the **source's** id
  (`clients/kubernetes.go:5885-6038`, `:5982`, `:6030`). So it confirms with **`{}`** (Hub ask HA7-2).
- **Rebuild** is plan 7P-3: a `run` with `sourceDeploymentId`, built from the source's **exact
  commit**, so the pin the Hub copies from the source stays true. Panacea's `/redeploy`, which
  rebuilds the branch head (`handlers/projects.go:1886-1889`), is not used.
- **Cancel** stops a running build. The Hub release stays `confirmed`; its deployment reads
  `failed` (Panacea does not tell failed from cancelled, task 01 `BuildFailed`).

**Files:**
- `control-plane/internal/deploy/ship/core.go` (new) — `withReservation`, extracted from `ship.go`
- `control-plane/internal/deploy/ship/rerun.go` (new) — `Rollback`, `Rebuild`, `Cancel`
- `control-plane/internal/deploy/ship/ship.go` (modified) — `Ship` calls `withReservation`
- `control-plane/internal/deploy/ship/rerun_test.go` (new)
- `control-plane/internal/api/deploy_acts.go` (modified) — three routes
- `control-plane/internal/api/deploy_acts_test.go` (modified)

#### Interfaces — complete

```go
package ship

// withReservation is the one reserve → act → confirm-or-abandon sequence (task 09, steps 5-8).
// The act is data, not a closure (plan re-review b10 M-4): spec is written into the journal's
// reserved record, and o.act(ctx, spec) runs it from the record alone, so a retry after a restart
// continues at step 6 before the binding or the snapshot has resolved. o.act returns the run it
// started ("" for a rollback, which starts none) and the act's answer, or a *deploy.Error;
// confirmFor(spec.Kind, run) builds {pipelineRun} for a run and {} for a rollback; wordsFor(spec.Kind)
// picks BuildWords or RollbackWords, and step 6's sentence by the error's class.
// Every error it returns after the reserve succeeded is wrapped with ErrAfterReserve (task 09
// step 6), for Rollback and Rebuild as for Ship, so each route passes {"reserved": true}
// (re-review I-2).
// idempotencyKey names the act's Ship-journal record (task 09 step 1, review M22):
// withReservation writes reserved, shipping, started and done there, and holds the key in the
// journal's live set from step 1 until done or the confirm loop ends (plan re-review b10 I-1),
// so Rollback and Rebuild survive a restart as Ship does, and a retry of a live Rollback or
// Rebuild answers 409 conflict instead of abandoning it. A rollback's "started" record has run ""
// and its RollbackResult fields as the answer, and confirms {}.
func (o *Orchestrator) withReservation(
	ctx context.Context,
	projectID, hubProjectID, idempotencyKey string,
	reserve hubapi.ReserveRequest,
	spec ActSpec,
) (hubapi.Release, string, bool, error)

// ActSpec is everything step 6 needs, serialisable into the journal's reserved record.
type ActSpec struct {
	Kind         string // "ship" | "rebuild" | "rollback"
	Ref          deploy.ProjectRef
	Branch       string // Ship: Branch.Name; Rebuild: the branch its preconditions resolved (below); "" for a rollback
	Commit       string // "" for a rollback
	NoCache      *bool
	DeploymentID string // Rollback: the deployment to roll back TO; "" otherwise
}

// AbandonWords are step 6's abandon messages, one per class of act error (task 05's classes).
// A refusal (KindForbidden … KindUnauthorized) always abandons with e.Message, never these.
type AbandonWords struct {
	NoAnswer string // KindUnavailable, Status 0, 502 or 504
	Accepted string // KindUnavailable, Status 2xx: an answer the connector could not use
	Failed   string // every other error, the catch-all included
}

// BuildWords — Ship and Rebuild, word for word task 09 step 6's:
//   NoAnswer "Panacea did not answer in time. It may still have started the build; if a run
//            appears, it will show with no brief recorded."
//   Accepted "Panacea accepted the build but did not name its run. If a run appears, it will
//            show with no brief recorded."
//   Failed   "Panacea could not start the build."
// RollbackWords — Rollback:
//   NoAnswer "Panacea did not answer in time. It may still have rolled back; look at the
//            history before trying again."
//   Accepted "Panacea answered for a different release, so this rollback was abandoned. Look
//            at the history before trying again."   (task 04's id mismatch, a 2xx)
//   Failed   "Panacea answered the rollback with an error. Look at the history before trying
//            again."
var BuildWords, RollbackWords AbandonWords

type RollbackRequest struct {
	ProjectID      string
	DeploymentID   string // the deployment to roll back TO
	IdempotencyKey string
}

type RollbackResult struct {
	Release            hubapi.Release
	SourceDeploymentID string
	Image              string
	Warmth             deploy.Warmth // known only now: Panacea sets it only on the response (models/models.go:695-708)
	ColdStartSeconds   int
	Confirmed          bool
}

// Rollback:
//   preconditions: bound; HubProjectID != ""; the deployment is in the watcher's snapshot
//     (else 404 deploy_not_found "That release is not in this project's history.");
//     the deployment's Environment != "" (b07 round 2, I-1), else 409 deploy_precondition
//     reason "no_environment": "Panacea recorded no environment for this release, so a
//     rollback would land in production." — Panacea's Rollback passes Environment verbatim
//     (handlers/projects.go:1612) and tenantNamespace sends "" to the base namespace
//     (clients/kubernetes.go:5887, :4213). Checked BEFORE the reserve: nothing is reserved,
//     Panacea is never called. Task 08 rule 1b's label join is display only and is not read;
//   lock: task 09 step 2's lock on (ProjectID, that deployment's Environment) for "rollback",
//     taken here, after the preconditions and before the reserve (review M25);
//   reserve {kind:"rollback", panaceaProject, environment: <that deployment's environment>,
//            sourceDeploymentId: DeploymentID}  — the Hub copies the source's pin (R5);
//   act: ActSpec{Kind:"rollback", Ref: ref, DeploymentID}, which o.act runs as
//     Actor.Rollback(ref, DeploymentID); a result whose SourceDeploymentID != DeploymentID
//     is treated as KindUnavailable (task 04, Status 200) and abandoned; words: RollbackWords;
//   confirm: {} — NEVER a deploymentId (HA7-2);
//   then Tracker.Nudge.
func (o *Orchestrator) Rollback(ctx context.Context, req RollbackRequest) (RollbackResult, error)

type RebuildRequest struct {
	ProjectID      string
	DeploymentID   string // the release to rebuild
	NoCache        *bool
	IdempotencyKey string
}

// Rebuild:
//   preconditions, in order:
//     the deployment is in the snapshot (else 404 deploy_not_found);
//     deploy.ValidCommit(source.Commit) — Deployment.Commit can be a branch name when Panacea
//       could not resolve the head (clients/pipeline.go:218-224) — else 409 deploy_precondition
//       reason "no_exact_commit": "This release did not record a full commit, so it cannot be
//       rebuilt exactly. Ship the branch instead.";
//     a branch: the source's Hub release branchRef, and only that (the Hub's is the real ref).
//       No Hub release, or one with no branchRef → 409 deploy_precondition reason "no_branch":
//       "Panacea recorded only a label for this release's branch. Ship the branch instead."
//       source.Branch is never used: it is Panacea's run label, written by the API as
//       sanitizeLabelValue(branch) (clients/pipeline.go:609, :2351-2364: feat/x and feat_x both
//       read feat-x) or by the webhook-receiver as sanitizeLabel(event.Branch)
//       (webhook-receiver/pipeline/trigger.go:1270, :1553-1574: feat/x reads feat-x, feat_x
//       stays feat_x), and whether a label equals its branch cannot be told without a copy of
//       those rules (b02 round 2, R8; review I4 and M23; re-review b03 I-1; plan §12).
//   lock: task 09 step 2's lock on (ProjectID, source.Environment) for "rebuild", taken after
//     the preconditions and before the reserve (review M25);
//   reserve {kind:"run", panaceaProject, environment: source.Environment, branchRef, commit:
//            source.Commit, briefId: null, designKey: <the source release's designKey or null>,
//            sourceDeploymentId: source.ID} — the Hub pins "inherited" (Hub contract §7.9);
//   act: ActSpec{Kind:"rebuild", Ref: ref, Branch: branch, Commit: source.Commit, NoCache}, which
//     o.act runs as Actor.Ship(ref, {branch, source.Commit, NoCache}); words: BuildWords (a
//     Rebuild is a Ship);
//   confirm: {pipelineRun}; Tracker.Track + Nudge.
func (o *Orchestrator) Rebuild(ctx context.Context, req RebuildRequest) (Result, error)

// Cancel stops a run. No Hub call. deploy.AlreadyFinished is a success.
func (o *Orchestrator) Cancel(ctx context.Context, projectID, run string) (deploy.CancelResult, error)
```

Ship's lock set (task 09 step 2, review M25) guards Rollback and Rebuild too: **one** lock per
`(project, environment)`, compared with `Environments.SameEnvironment`, shared by all three acts.
Rollback locks the deployment's `Environment` and Rebuild `source.Environment`, each after its
preconditions and before its reserve, which is the environment each reserves (C2: never a branch
name). So a Ship and a rollback of one environment can no longer reserve and act at once. Held →
`409 conflict` with the holder's sentence, one per act (task 09 step 2): *"A Ship to this
environment is already in progress."*, *"A rebuild of this environment is already in
progress."*, *"A rollback of this environment is already in progress."* The abandon sentences
are per act as well: `BuildWords` for Ship and Rebuild, `RollbackWords` for Rollback (above;
plan re-review b10 M-4), so a failed rollback never says *"could not start the build"*.

#### Control-plane routes — complete wire

```
POST /v1/projects/{projectId}/deploy/deployments/{deploymentId}/rollback
  header  Idempotency-Key (required)
  request {}
  200 { "release": ReleasePin, "sourceDeploymentId": "…", "image": "…" | null,
        "warmth": "warm" | "cold" | null, "coldStartSeconds": 0, "confirmed": true | false }
  404 deploy_not_found            not in history; or Panacea's "deployment not found: …" (projects.go:1570),
                                  which is what a rollback to a run Tekton has pruned answers (plan 7P-4)
  409 deploy_conflict             Panacea: "cannot roll back: image … no longer exists in the registry
                                  (it may have been pruned) — redeploy from source instead" (projects.go:1603-1604)
  422 deploy_rejected             Panacea: "deployment has no image tag — cannot rollback" (projects.go:1575)
  409 deploy_precondition         reason ∈ not_bound | no_hub_project | no_environment (the target's
                                  Environment is "": Panacea would roll it into production, b07 round 2, I-1)
  409 conflict                    this environment's lock is held: the holder's sentence (task 09 step 2)
  deploy_not_connected · deploy_token_rejected
  503 upstream_unavailable        panacea (abandoned: details.status, and details.reserved true, re-review I-2)
                                  or hub (nothing happened)

POST /v1/projects/{projectId}/deploy/deployments/{deploymentId}/rebuild
  header  Idempotency-Key (required)
  request { "noCache": true | null }
  202 { "release": ReleasePin, "run": "…", "confirmed": true | false, "message": "" }
  409 deploy_precondition         reason ∈ not_bound | no_hub_project | no_exact_commit | no_branch
  (the rest as Ship, task 09)

POST /v1/projects/{projectId}/deploy/runs/{run}/cancel
  request {}
  200 { "outcome": "cancelled" | "already_finished", "message": "…" }
  404 deploy_not_found · 403 deploy_forbidden · 503 upstream_unavailable
```

Every `deploy_*` refusal carries Panacea's sentence as `message` (task 05's `writeDeployError`), so
the lens can offer **Rebuild** beside *"redeploy from source instead"* without parsing it — it offers
Rebuild because the code is `deploy_conflict` on a rollback, and prints the sentence verbatim.

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestRollbackReservesRollbackShape` | the reserve request is `{kind:"rollback", sourceDeploymentId:"d1", environment:<d1's>}` with no `pipelineRun`, `commit` or `branchRef` | synthesising a run for a rollback (R5: *"never synthesise a pipelineRun"*) |
| `TestRollbackConfirmsEmpty` | the confirm body is exactly `{}` | sending the source's id, which the Hub answers 409 (HA7-2) |
| `TestRollbackInheritsPin` | against `hubclient.NewFake()` with `d1` joined to a pinned release → the rollback release's pin is `pinned/inherited` with `d1`'s key | a rollback shown against today's brief head (P1) |
| `TestRollbackUnknownSource` | `d9` has no Hub release → pin `none/source_unknown`; the rollback still happens | refusing a rollback because it has no brief |
| `TestRollbackPrunedImageAbandons` | Panacea's 409 → `Abandon` with that sentence; route `409 deploy_conflict` | a reservation left dangling after a refusal |
| `TestRollbackRefusesNoEnvironment` | target `d-old` with `Environment: ""`, `Branch: "feat-x"`, `build "succeeded"`, and a snapshot row `feat-x` (branch `feat-x`) that task 08 rule 1b joins it to → `409 deploy_precondition` `no_environment` with *"Panacea recorded no environment for this release, so a rollback would land in production."*; the Hub fake saw no `Reserve` and the Panacea fake saw no call (b07 round 2, I-1) | reading rule 1b's `environmentRow` (or the row's `branch`) as the rollback's environment, which Panacea sends to `tenant-<project>`, production (`handlers/projects.go:1612`, `clients/kubernetes.go:5887`, `:4213`) |
| `TestRollbackIdMismatchAbandons` | the connector reports a different id (`KindUnavailable`, `Status 200`) → abandoned with `RollbackWords.Accepted`, `503` with `details.status 200` and `details.reserved true` | confirming a rollback whose premise broke; an id mismatch told *"did not answer in time"* (re-review M-4) |
| `TestRerunErrorsAreReserved` | after the reserve, the Panacea fake answers `KindUnavailable` with `Status` `0`, then `503`: Rollback → `Abandon` with `RollbackWords.NoAnswer`, then `.Failed`; Rebuild → `BuildWords.NoAnswer`, then `.Failed`. Each route answer is `503 upstream_unavailable` with that `details.status` and `details.reserved true`; a Rollback refused by `no_environment` (before any reserve) carries no `reserved` key | a Rollback or Rebuild whose error after a reserve reads *"could not be reached, so nothing was sent"* while a Hub release was reserved and abandoned; Ship's build words on a rollback (re-review I-2) |
| `TestRebuildUsesExactCommit` | source `Commit` 40 hex → Panacea body `{"branch":…, "commit":<that>}` | Panacea's `/redeploy` path, which builds the branch head |
| `TestRebuildRefusesSymbolicCommit` | source `Commit: "main"` → `409 deploy_precondition` `no_exact_commit`; no reserve | rebuilding "whatever main is now" under an inherited pin |
| `TestRebuildPrefersHubBranchRef` | source `Branch: "feat-x"`, its release `branchRef: "refs/heads/feat/x"` → Panacea gets `feat/x` | cloning a sanitised label that is not a branch (`Deployment.branch` is the run label, `pipeline.go:609`; settled in b02, deviation 3) |
| `TestRebuildRefusesLabelOnlyBranch` | source `Branch: "feat-x"`, no Hub release → `409 deploy_precondition` `no_branch` with *"Panacea recorded only a label for this release's branch. Ship the branch instead."*; the Hub fake saw no `Reserve` and the Panacea fake saw no call (b02 round 2, R8) | falling back to `source.Branch`, which records `refs/heads/feat-x` in the Hub and asks Panacea to clone a branch that does not exist |
| `TestRebuildInheritsPin` | the reserve carries `sourceDeploymentId`; the Hub fake returns `inherited` | a rebuild pinned to today's head |
| `TestCancelNoHubCall` | `Cancel` → the Hub fake saw no call | abandoning a confirmed release (the Hub allows abandon only from `reserved`) |
| `TestCancelAlreadyFinished` | `AlreadyFinished` → `200 {outcome:"already_finished"}` | an error toast when Undo loses the race to the build's end |
| `TestOneActPerEnvironment` | inside the Panacea fake's `Rollback` hook, a rebuild of a deployment in the same environment → `409 conflict` *"A rollback of this environment is already in progress."*; the reverse order, inside the `Ship` hook of the rebuild → *"A rebuild of this environment is already in progress."* (review M25) | two releases racing one environment; one shared sentence that names the wrong act |
| `TestShipAndRollbackShareTheLock` | the `EnvironmentNamer` is `deploytest.Fake` wrapped so `SameEnvironment` answers task 03's rule (`prod` == `production`); inside the Fake's `Ship` hook for branch `main` (env `prod`), `Rollback` of a snapshot deployment whose `Environment` is `production` → `409 conflict` *"A Ship to this environment is already in progress."*, the Hub fake saw one `Reserve` and the Fake no `Rollback`; after the Ship answers, the same `Rollback` proceeds (review M25) | a Ship lock per branch beside a rollback lock per environment, so a Ship and a rollback of one environment reserve and act at once; a lock keyed by the raw string, so `prod` and `production` race |
| `TestRollbackIsJournalled` | a rollback whose `Actor.Rollback` answered and whose `Confirm` the Hub fake refuses with 503; a **new** `Orchestrator` on the same `JournalDir` → `Resume` sends `Confirm(rl-1, {})` (no `pipelineRun`), and a retry with the same key makes zero `Actor.Rollback` calls and answers the `sourceDeploymentId`, `image`, `warmth` and `coldStartSeconds` the `started` record holds from Panacea's answer (task 09 step 1, review M22; plan re-review b10 M-4) | a rollback repeated after a restart, or its confirm lost with the process; a settled rollback that answers without Panacea's `warmth`, which is known only from that response |
| `TestRollbackShippingAbandonsWithRollbackWords` | a `shipping` record whose `act.kind` is `rollback`, no live act → a retry with the key, and `Resume` on such a record, each `Abandon(rl-1)` with `RollbackWords.NoAnswer` and zero `Actor.Rollback` calls (plan re-review b10 M-4) | a rollback's crash abandoned with the build sentence, so the Hub's `abandonMessage` says *"It may still have started the build"* of a rollback |
| `TestRollbackLiveRetryConflicts` | inside the Panacea fake's `Rollback` hook, a retry with the same key → `409 conflict` *"A rollback of this environment is already in progress."*, zero `Abandon`; the first act then confirms `{}` (plan re-review b10 I-1) | a retry that abandons a rollback still in flight |
| `TestShipStillPasses` | task 09's whole suite, unchanged, after the `withReservation` extraction | a refactor that changes Ship |

#### Steps

- [ ] Extract `withReservation` from `ship.go` first, with task 09's tests green before and after.
- [ ] Tests for this task; then `rerun.go` and the three routes.
- [ ] Run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/ship/ ./control-plane/internal/api/ -race -count=1
→ ok
```

#### What this task does NOT claim

- That a rollback's warmth is known before pressing. It is not: Panacea sets it only on the
  response (`models/models.go:695-708`). The lens shows it afterwards (task 16).
- That a rollback made in the Panacea portal is visible. It is not (plan 7P-4, Q-P7-5).
- That rebuilding a commit on a branch deleted from the remote works. Panacea will refuse at clone
  time and the refusal is shown; no pre-check copies Panacea's rule.
