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
// act returns the run it started ("" for a rollback, which starts none) or a *deploy.Error.
// confirm builds the ConfirmRequest from act's result: {pipelineRun} for a run, {} for a rollback.
func (o *Orchestrator) withReservation(
	ctx context.Context,
	projectID, hubProjectID string,
	reserve hubapi.ReserveRequest,
	act func(ctx context.Context) (run string, err error),
	confirm func(run string) hubapi.ConfirmRequest,
) (hubapi.Release, string, bool, error)

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
//   reserve {kind:"rollback", panaceaProject, environment: <that deployment's environment>,
//            sourceDeploymentId: DeploymentID}  — the Hub copies the source's pin (R5);
//   act: Actor.Rollback(ref, DeploymentID); a result whose SourceDeploymentID != DeploymentID
//     is treated as KindUnavailable (task 04) and abandoned;
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
//       source.Branch is never used: it is Panacea's run label, sanitizeLabelValue(branch)
//       (clients/pipeline.go:609, :2351-2364), which turns feat/x into feat-x, and whether a label
//       equals its branch cannot be told without a copy of that rule (b02 round 2, R8).
//   reserve {kind:"run", panaceaProject, environment: source.Environment, branchRef, commit:
//            source.Commit, briefId: null, designKey: <the source release's designKey or null>,
//            sourceDeploymentId: source.ID} — the Hub pins "inherited" (Hub contract §7.9);
//   act: Actor.Ship(ref, {branch, source.Commit, NoCache});
//   confirm: {pipelineRun}; Tracker.Track + Nudge.
func (o *Orchestrator) Rebuild(ctx context.Context, req RebuildRequest) (Result, error)

// Cancel stops a run. No Hub call. deploy.AlreadyFinished is a success.
func (o *Orchestrator) Cancel(ctx context.Context, projectID, run string) (deploy.CancelResult, error)
```

The same per-`(project, environment)` `TryLock` as Ship's per-branch lock guards Rollback and
Rebuild: two acts on one environment at once are `409 conflict` *"Something is already changing this
environment."*

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
  409 conflict · deploy_precondition · deploy_not_connected · deploy_token_rejected
  503 upstream_unavailable        panacea (abandoned) or hub (nothing happened)

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
| `TestRollbackIdMismatchAbandons` | the connector reports a different id → abandoned, `503` | confirming a rollback whose premise broke |
| `TestRebuildUsesExactCommit` | source `Commit` 40 hex → Panacea body `{"branch":…, "commit":<that>}` | Panacea's `/redeploy` path, which builds the branch head |
| `TestRebuildRefusesSymbolicCommit` | source `Commit: "main"` → `409 deploy_precondition` `no_exact_commit`; no reserve | rebuilding "whatever main is now" under an inherited pin |
| `TestRebuildPrefersHubBranchRef` | source `Branch: "feat-x"`, its release `branchRef: "refs/heads/feat/x"` → Panacea gets `feat/x` | cloning a sanitised label that is not a branch (`Deployment.branch` is the run label, `pipeline.go:609`; settled in b02, deviation 3) |
| `TestRebuildRefusesLabelOnlyBranch` | source `Branch: "feat-x"`, no Hub release → `409 deploy_precondition` `no_branch` with *"Panacea recorded only a label for this release's branch. Ship the branch instead."*; the Hub fake saw no `Reserve` and the Panacea fake saw no call (b02 round 2, R8) | falling back to `source.Branch`, which records `refs/heads/feat-x` in the Hub and asks Panacea to clone a branch that does not exist |
| `TestRebuildInheritsPin` | the reserve carries `sourceDeploymentId`; the Hub fake returns `inherited` | a rebuild pinned to today's head |
| `TestCancelNoHubCall` | `Cancel` → the Hub fake saw no call | abandoning a confirmed release (the Hub allows abandon only from `reserved`) |
| `TestCancelAlreadyFinished` | `AlreadyFinished` → `200 {outcome:"already_finished"}` | an error toast when Undo loses the race to the build's end |
| `TestOneActPerEnvironment` | a rollback and a rebuild of one environment at once → one `409 conflict` | two releases racing one environment |
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
