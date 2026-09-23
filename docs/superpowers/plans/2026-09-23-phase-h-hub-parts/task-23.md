### Task 23: releases — reserve, and the pin the Hub takes (graft G1); list and get (routes 35–37)

**Lane A. Depends on: 22. About 3 hours. Contract task H5.**

Ship is **reserve → deploy → confirm** (contract §7.9). The Hub, not the caller, decides which
revision a release pins, **before the deploy exists**; the only thing written afterwards is which
run the release became (task 24). That is how provenance contract P7 — *"a release link written by a
second call"* is prohibited — survives the ledger living on another machine (ADR-H1 G1). And P2,
*never pinned by observation*, holds by construction: a pin exists only on a reserved release.

**Files:**
- `control-plane/internal/hub/server/releases.go` + `releases_integration_test.go` (new)
- `control-plane/internal/hub/store/releases.go` (new)
- `control-plane/internal/hub/server/server.go` — `Config.Now` (the one clock every route uses), `ReservationTTL`; `s.registerReleases()` (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask23`; the fixture gains `ReservedReleaseID` (modify)

#### Contract — the routes, complete

**Route 35 — `POST /v1/projects/{projectId}/releases`**, body `hubapi.ReserveRequest`, role M,
`Idempotency-Key` expected. Validation (400 `bad_request`, `field` named, unless stated):

| Field | Rule |
|---|---|
| `kind` | `run` or `rollback` |
| `panaceaProject`, `environment` | `^[a-z0-9-]{1,63}$` — Panacea's names |
| `sourceDeploymentId` | 1–128 characters; **required** for a rollback |
| `commit` | 40 lowercase hex; **required** for a run without `sourceDeploymentId`; **absent** for a rollback |
| `branchRef` | `""` or valid (§3.3); **absent** for a rollback |
| `briefId` | a `bf-` id; **absent** for a rollback. Not a brief of this project → **422 `key_unresolvable`** |
| `designKey` | a design key; **absent** for a rollback. No approval in this project → **422 `design_not_approved`** — the Hub never infers one |

**The pin — taken in the reservation's transaction, then never again** (contract §7.9's table). The
org's audit lock is already held (task 14), so no brief head can move while it is chosen.

| Case | Brief | Pin |
|---|---|---|
| run, no `sourceDeploymentId` | `briefId` if given; else the live branch brief for `branchRef` (when not `""`); else the live project brief; else none | none → `{status: none, reason: no_brief}`. Brief's `latest_complete_revision` = 0 → `{none, no_complete_revision}`. Else `{pinned, ship_head}` with `briefKey` = `brief:<id>@<latest complete>`, its `briefDigest`, and `headRevisionAtShip` = the brief's `head_revision` **now** |
| run or rollback, with `sourceDeploymentId` | the **confirmed** release **in this project** joined to `(panaceaProject, sourceDeploymentId)` (task 24's join) | pinned → `{pinned, inherited}`, copying its key, digest and `headRevisionAtShip`; unpinned → `{none, source_unpinned}`; no such release → `{none, source_unknown}`. `sourceReleaseId` = that release (when found). **A rollback also copies** its `commit`, `branchRef` and `designKey` |

Then: `reservedAt` = the transaction's clock; `expiresAt` = `reservedAt` +
`PANACEA_APP_HUB_RESERVATION_TTL` (default 15 min); `state: reserved`; `Emit release.reserved
{"kind","environment","pinStatus","briefKey"}` (`briefKey` `""` when none). `201 hubapi.Release`.

**Route 36 — `GET /v1/projects/{projectId}/releases?environment=&panaceaProject=&deploymentId=&pipelineRun=&state=&after=&limit=`**,
V: `200 Page[Release]`, newest `reservedAt` first. `deploymentId=` finds the release joined to that
deployment — **zero items means "no brief is recorded for this release"** (§7.9), which is how a
Deploy row for an unreserved Panacea deployment is told apart.

**Route 37 — `GET /v1/releases/{releaseId}`**, V: `200 Release`; 404 `release_not_found`.

**Reading an overdue reservation.** The janitor (task 24) marks a `reserved` release `expired`
within 60 s of `expiresAt`. Until it runs, every read **renders** such a release as `state:
"expired"`, so a caller never sees a reservation it can no longer confirm reported as live — and the
fake (task 05) and the Hub agree through the interface.

**One clock.** `server.Config.Now` (default `time.Now`) is the only clock a route uses; every
timestamp a route writes — `reservedAt` included — is written from it explicitly, never from
Postgres's `now()`, so a test can move time and expiry follows.

#### Tests to write first — `releases_integration_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestShipHeadPinsTheLatestCompleteRevision` | revisions 1 complete, 2 complete, 3 open; reserve a run on the branch → `pinned`/`ship_head`, `briefKey` `@2`, its digest, `headRevisionAtShip: 3` | pinning the head (an incomplete revision) — ADR-H1 Q1 |
| `TestThePinFallsBackToTheProjectBrief` | no branch brief → the project brief's latest complete | a release with no brief when one governs the branch |
| `TestNoBriefAndNoCompleteRevision` | no briefs → `none`/`no_brief`; a brief with only open revisions → `none`/`no_complete_revision`; both reserve **successfully** | Ship blocked by a missing brief, which §7.9 records rather than refuses |
| `TestAnExplicitBriefMustBeThisProjects` | `briefId` of another project → 422 `key_unresolvable` | a pin crossing projects |
| `TestADesignKeyMustBeApproved` | an unapproved key → 422 `design_not_approved`; an approved one → stored | a release claiming a design nobody approved |
| `TestThePinDoesNotMoveWhenTheBriefDoes` | reserve (pin `@2`); write revision 4 complete; `GET` the release → still `@2`, still `headRevisionAtShip: 3` | a pin re-resolved on read (P3, P7) |
| `TestTheRollbackShape` | a rollback with `commit`, `branchRef`, `briefId` or `designKey` → 400 naming it; without `sourceDeploymentId` → 400 | a rollback asserting a pin of its own (R5) |
| `TestARunNeedsACommit` | no `commit` and no `sourceDeploymentId` → 400 `field: commit`; an uppercase or 39-character commit → 400 | a release pointing at no code |
| `TestAnUnknownSourceIsRecordedNotRefused` | a rollback with a `sourceDeploymentId` no release is joined to → 201, `none`/`source_unknown`, `sourceReleaseId: null` | a rollback of a pre-Hub deployment refused |
| `TestExpiresAtIsTheTTL` | with TTL 15 min, `expiresAt` − `reservedAt` = 15 min exactly, both in the §1 time format | — |
| `TestAnOverdueReservationReadsExpired` | clock + 16 min, before any janitor → `GET` shows `state: expired` | a caller confirming a reservation it was told is live, then getting 409 |
| `TestReleaseListFiltersAndOrder` | three releases in two environments; each filter; newest first; `deploymentId=` of an unjoined id → no items | — |

Matrix `rowsTask23`:

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `POST /v1/projects/{ProjectID}/releases` | 403 | 201 | 201 | 404 | 404 | 404 |
| `GET /v1/projects/{ProjectID}/releases` | 200 | 200 | 200 | 404 | 404 | 404 |
| `GET /v1/releases/{ReservedReleaseID}` | 200 | 200 | 200 | 404 | 404 | 404 |

#### Steps

- [ ] **Step 1:** Re-read contract §7.9 (all), provenance contract P2, P3, P7, ruling R5.
- [ ] **Step 2:** Tests and rows; they fail. Then `store/releases.go`, `releases.go`, `Config.Now`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestShipHeadPins|TestThePinFallsBack|TestNoBriefAndNo|TestAnExplicitBrief|TestADesignKeyMust|TestThePinDoesNot|TestTheRollbackShape|TestARunNeeds|TestAnUnknownSource|TestExpiresAtIs|TestAnOverdueReservation|TestReleaseListFilters|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 14
```

#### What this task does not claim

- Confirm, abandon, the deployment join and the janitor's expiry: task 24.
- That phase 7 calls Panacea between reserve and confirm correctly: phase 7's.
- `inherited` and `source_unpinned`: they need a confirmed, joined release, so task 24 tests them.
