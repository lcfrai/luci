### Task 24: confirm, abandon, the deployment join — routes 38–40; and the janitor (start line S7, server side)

**Lane A. Depends on: 23. About 3 hours. Contract task H5 (and §12.5).**

After Panacea accepts or refuses the deploy, the control plane **confirms** the reservation with
the run it became, or **abandons** it with Panacea's reason; the deploy poller later **joins** the
Deployment it sees for that run (contract §7.9 steps 3–4). None of these touches the pin. The
janitor expires reservations nobody confirmed, collects unreferenced blobs, and cleans old
idempotency keys (§12.5). When this task is merged, **start line S7 is reached on the server side**.

**Files:**
- `control-plane/internal/hub/server/releases.go`, `store/releases.go` — the three routes (modify)
- `control-plane/internal/hub/server/janitor.go` + `janitor_integration_test.go` (new)
- `control-plane/internal/hub/server/releases_integration_test.go` — this task's tests (modify)
- `control-plane/cmd/hub/serve.go` — start and stop the janitor (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask24`; the fixture gains `ConfirmedReleaseID` (modify)

#### Contract — the routes, complete

**Route 38 — `POST /v1/releases/{releaseId}/confirm`**, body `hubapi.ConfirmRequest`,
`Idempotency-Key` expected. Readable project, else 404 `release_not_found`; a viewer → 403
`forbidden` `required: member`; a member who is not the reserver → **403 `not_reserver`**,
`details.releaseId`. Body: a run sends `pipelineRun` (1–253 of `[a-z0-9.-]`, else 400); a rollback
sends `{}` — Panacea creates no deployment for a rollback, and phase 7 always sends `{}` (HA7-2,
task 00). A rollback's `{"deploymentId":"…"}` (1–128 characters) is still accepted and **also
writes the deployment join**, except that a `deploymentId` equal to the rollback's own
`sourceDeploymentId` is **409 `conflict`**, `details.field=deploymentId`, nothing written: that
deployment is the source's, and joining it here would attribute it to two releases even when the
source was never joined (`source_unknown`). A confirm writes no `environment`: a null one stays
null until route 40 names it.

| Current state | Result |
|---|---|
| `reserved`, before `expiresAt` | `200 Release`, `state: confirmed`, `confirmedAt` set; `Emit release.confirmed {"pipelineRun"}` (`""` for a rollback) |
| `confirmed`, the same `pipelineRun` (or, for a rollback, `{}` / the same `deploymentId`) | `200 Release`, unchanged, no event |
| `confirmed` with a different run; `abandoned` | 409 `release_state`, `details.releaseId`, `details.state` |
| `reserved` after `expiresAt`, or `expired` | 409 `release_expired`, `details.releaseId` |
| another release in the org already confirmed that `(panaceaProject, pipelineRun)` | 409 `conflict` |

**Route 39 — `POST /v1/releases/{releaseId}/abandon`**, body `hubapi.AbandonRequest` (`reason`
`deploy_failed` or `cancelled`; `message` 0–500 characters — Panacea's refusal sentence). Same
reader and reserver rules as 38. From `reserved` (and not overdue) only, else 409 `release_state`
with the rendered state (`expired` for an overdue reservation). `200 Release`, `state: abandoned`;
`Emit release.abandoned {"reason"}`.

**Route 40 — `POST /v1/releases/{releaseId}/deployments`**, body `hubapi.DeploymentJoinRequest`, role
M (any member: the deploy poller runs as whoever is signed in). Confirmed releases only, else 409
`release_state`. `(panaceaProject, deploymentId)` joins **at most one** release in the org: another
release holding it → 409 `conflict`; this release already holding it → `200`; else `201
{"deploymentId","joinedAt"}` and `Emit release.deployment_joined {"deploymentId","environment"}`.

**The join's environment (HA7-1, task 00).** `DeploymentJoinRequest.environment` is optional
(`^[a-z0-9-]{1,63}$` when present, else 400 `field: environment`). In the join's transaction, the
release row locked `FOR UPDATE` first:

| Release's environment | Join names | Result |
|---|---|---|
| `null` | a value | `UPDATE releases SET environment` (task 07's guard allows exactly this, once, while `confirmed`); then the join |
| `null` | none | the join; the environment stays `null` |
| a value | the same value, or none | the join; no `UPDATE` |
| a value | a **different** value | 409 `conflict`, `details.field=environment`, `details.environment` = the recorded one; **nothing joined** |

The event's `data.environment` is the release's environment after the join (`""` while null). A
repeat of an existing join (`200`) that names a different environment is the same 409.

#### Contract — the janitor, complete

```go
// control-plane/internal/hub/server/janitor.go
package server

// JanitorConfig: Interval 60 s; Retention 24 h (blobs and idempotency keys).
type JanitorConfig struct {
	Interval  time.Duration
	Retention time.Duration
	Now       func() time.Time
	Log       *slog.Logger
}

// RunJanitor sweeps every Interval until ctx ends. Each sweep acquires one pool
// connection and runs pg_try_advisory_lock(hashtext('panacea-app-hub-janitor'))
// on it: a replica that does not get the lock does nothing this round, so two
// replicas never double-sweep (contract §12.3). With the lock, for EACH org —
// set_config('hub.org_id', org, true) in its own transaction, so row-level
// security binds the janitor too, and the org's audit head locked first:
//   1. reserved releases with expires_at < now → state 'expired', expired_at = now;
//      Emit release.expired {} with actor NULL;
//   2. project_blobs older than Retention that neither design_approvals.manifest_digest
//      nor design_approval_blobs references → deleted; Emit blob.deleted
//      {"size","contentType"} (audit only);
//   3. blobs of the org with no project_blobs row left → deleted;
//   4. idempotency_keys older than Retention → deleted (no event: not a record change
//      anyone reads).
// The lock is released and the connection returned on every path.
func RunJanitor(ctx context.Context, st *store.Store, blobs blobstore.Store, cfg JanitorConfig)

// Sweep is one round, exported for the tests.
func Sweep(ctx context.Context, st *store.Store, blobs blobstore.Store, cfg JanitorConfig) (SweepReport, error)

type SweepReport struct {
	Locked                    bool // false: another replica holds the janitor lock
	Expired, BlobsUnlinked    int
	BlobsDeleted, KeysDeleted int
}
```

#### Tests to write first

`releases_integration_test.go` (added):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestConfirmARun` | reserve → confirm `engagement-portal-build-x7k2p` → 200 `confirmed`; the pin unchanged; one event | a confirm that re-takes the pin |
| `TestConfirmIsIdempotentForTheSameRun` | the same run again → 200, no second event; a different run → 409 `release_state` | a retry turning into an error, or a second run overwriting the first |
| `TestOnlyTheReserverConfirmsOrAbandons` | Linus confirming Grace's reservation → 403 `not_reserver`; Vera → 403 `forbidden` | a teammate confirming a release they did not start |
| `TestConfirmAfterExpiryIs409` | clock + 16 min → 409 `release_expired` | a pin confirmed for a deploy that might not be this one (R3) |
| `TestAConfirmRetryReplays` | a lost response: the same `Idempotency-Key` → the stored 200 | the Ship handler's retry loop (§7.9) failing on its own success |
| `TestOneReleasePerRun` | two releases confirming one `(panaceaProject, pipelineRun)` → the second 409 `conflict` | two releases claiming one PipelineRun |
| `TestAbandonFromReservedOnly` | abandon → 200 `abandoned` with reason and message; abandon again → 409 `release_state`; confirm after → 409 `release_state` | an abandoned release revived |
| `TestJoinADeployment` | confirmed → 201; again → 200; another release joining the same `d-6f2` → 409 `conflict`; joining an unconfirmed release → 409 `release_state` | a Deployment attributed to two releases |
| `TestARollbackConfirmWithADeploymentJoins` | confirm a rollback with `{"deploymentId":"d-7a1"}` → `deployments` holds it | a rollback that never joins its deployment |
| `TestARollbackConfirmNamingItsSourceIs409` | a rollback with `sourceDeploymentId: d-4d0` (source unjoined, `source_unknown`) confirmed with `{"deploymentId":"d-4d0"}` → 409 `conflict`, `field: deploymentId`, no join row, still `reserved`; with `{}` → 200 | the source's deployment attributed to its rollback (HA7-2) |
| `TestTheJoinNamesANullEnvironment` | a run reserved with `environment: null`, confirmed (still `null`), joined `{d-1, "feat-x"}` → 201, `GET` shows `"feat-x"`, event `data.environment: "feat-x"`; join `{d-2, "feat-y"}` → 409 `conflict`, `details.environment: "feat-x"`, `d-2` not joined; join `{d-3}` → 201, still `"feat-x"` | the first Ship's release never learning its environment, or a second join re-pointing it (HA7-1) |
| `TestANullEnvironmentReleaseAbandonsAndExpires` | a null-environment run abandoned → 200 `abandoned`; another left past `expiresAt`, `Sweep` → `expired`; both keep `environment: null` | the review's `state = 'reserved'` check turning a refused deploy into a 500 (task 00) |
| `TestARollbackInheritsThePin` | release A pinned `@2`, confirmed, joined `d-4d0`; rollback with source `d-4d0` → `pinned`/`inherited`, A's key, digest and `headRevisionAtShip`, `sourceReleaseId` = A, and A's `commit`, `branchRef`, `designKey` | R5's union broken: a rollback with a fresh or empty pin |
| `TestAnUnpinnedSourceIsRecorded` | A with `none`/`no_brief`, confirmed and joined → rollback → `none`/`source_unpinned`, `sourceReleaseId` = A | — |

`janitor_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheJanitorExpiresOverdueReservations` | clock + 16 min; `Sweep` → `Expired: 1`; the row is `expired` with `expired_at`; one `release.expired` event with a null actor | a reservation live for ever |
| `TestTheJanitorCollectsOnlyUnreferencedBlobs` | an approved snapshot's blobs and one stray upload; clock + 25 h → the stray's association and bytes deleted, the approved ones kept; a `blob.deleted` event | a referenced snapshot deleted (contract §13 H4) |
| `TestTheJanitorWaits24Hours` | clock + 23 h → nothing collected | an upload deleted before the approval that follows it |
| `TestTheJanitorCleansIdempotencyKeys` | a key from 25 h ago is gone; one from 1 h ago stays | an unbounded table |
| `TestOnlyOneReplicaSweeps` | two concurrent `Sweep`s → exactly one `Locked: true` | double expiry events from two pods |
| `TestTheJanitorRespectsRowLevelSecurity` | two orgs; each org's sweep touches only its rows (asserted through the audit events' `org_id`) | a janitor that runs as the owner and crosses tenants |

Matrix `rowsTask24` (each cell reserves as its own column's user where it can, so the reserver
rule is exercised):

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `POST /v1/releases/{ReservedReleaseID}/confirm` (reserved by Grace) | 403 | 403 `not_reserver` | 200 | 404 | 404 | 404 |
| `POST /v1/releases/{ReservedReleaseID}/abandon` (reserved by Grace) | 403 | 403 `not_reserver` | 200 | 404 | 404 | 404 |
| `POST /v1/releases/{ConfirmedReleaseID}/deployments` (a fresh id per cell) | 403 | 201 | 201 | 404 | 404 | 404 |

#### Steps

- [ ] **Step 1:** Re-read contract §7.9, §12.3 (replicas), §12.5; ADR-H1 R3.
- [ ] **Step 2:** Tests and rows; they fail. Then the three routes, `janitor.go`, and `serve.go`'s
  start (after listening) and stop (before `Shutdown` returns).
- [ ] **Step 3:** Commit by pathspec. **Announce S7 (server side)** in the first line.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestConfirmARun|TestConfirmIsIdempotent|TestOnlyTheReserver|TestConfirmAfterExpiry|TestAConfirmRetry|TestOneReleasePerRun|TestAbandonFrom|TestJoinADeployment|TestARollbackConfirm|TestARollbackInherits|TestAnUnpinnedSource|TestTheJanitor|TestOnlyOneReplica|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 19
go test -tags integration ./control-plane/internal/hub/... -race -count=1                     # ok
```

#### What this task does not claim

- That a crash between Panacea's 202 and the confirm is handled beyond the retry window (ADR-H1
  R3: such a release becomes `expired`, visibly, although a run happened).
- That the deploy poller confirms by observation: it never pins (P2); if ruling R7's Panacea
  metadata ask is granted, it may *confirm* a reservation by the `releaseId` it finds — phase 7's.
