### Task 25: evidence — phase 3's shared records (H-2), routes 41–42

**Lane A. Depends on: 20. About 1.5 hours. Contract task H6.**

Phase 3 records a judgement — *this commit settles criterion N of brief revision K, because …* —
against a repository, not a local project (H-3), and wants teammates to see it (H-2). Its
`evidence.Store` gains a Hub implementation whose `Shared()` is `true`; a repository with no Hub
project answers 422 `repo_not_linked`, and phase 3 keeps its local file store for that repository
(contract §14). `recordedBy` is the principal (H-4); the body has no recorder field.

**Files:**
- `control-plane/internal/hub/server/evidence.go` + `evidence_integration_test.go` (new)
- `control-plane/internal/hub/store/evidence.go` (new)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask25` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerEvidence()` (modify: one line)

#### Contract — the routes, complete

**Route 41 — `POST /v1/evidence`**, body `hubapi.EvidenceRequest`, `Idempotency-Key` expected.

| Check, in order | Failure |
|---|---|
| `repo` is a valid repo key for either host (`hubapi.HostOfRepoKey`); `criterionId` 1–128 characters; `criterionDigest` a digest; `commit` 40 lowercase hex; `verdict` `settled` or `contested`; `because` 1–2,000 characters **after trimming** (phase 3's P12 rule, `task-15.md:56` as the contract cites it) | 400, `field` named |
| `repo` is a remote of a project the caller **can read** | 422 `repo_not_linked`, `details.repo` — the same answer whether no project has it or one the caller cannot see |
| the caller is at least **member** of that project | 403 `forbidden` `required: member` |
| `briefKey` names a revision of a brief **in that project** | 422 `key_unresolvable`, `details.key` |

`201 hubapi.EvidenceRecord` with `projectId` and `recordedBy` = the principal; `repo_key` stored
**verbatim**, so a record survives the remote's later removal (§7.4). Append-only: no edit, no
delete (the database's `forbid_change`, task 07b). `Emit evidence.recorded
{"briefKey","criterionId","verdict","commit"}` — never `because` (§8).

**Route 42 — `GET /v1/evidence?repo=<repoKey>&digest=<d1>&digest=<d2>…`** — phase 3's `ByDigest`.
1–100 `digest` values (else 400 `field: digest`); `repo` valid (else 400). The repo's project must
be readable — **if it is not, `200` with no items**, never a 404 that would confirm the repo is
linked. `200 Page[EvidenceRecord]`, **oldest first** (by `recorded_at`, then id), filtered to
`criterion_digest IN (…)`.

**The seam to phase 3.** `hubclient.Evidence` (task 04) is what phase 3's Hub-backed
`evidence.Store` calls. `hubclient` formats `recordedBy` as `"<displayName> <<email>>"` for phase
3's string field `Record.RecordedBy` (contract §14, H-4) — task 34 does that, not the server.

#### Tests to write first — `evidence_integration_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRecordEvidence` | 201; `recordedBy` = Linus; `repo` verbatim; one event without `because` | a recorder asserted by the caller (H-4) |
| `TestAnUnlinkedRepoIs422` | a repo no project has, and a repo only Out Sider's project has → both 422 `repo_not_linked`, byte-identical bodies | a 404/403 difference that probes which repos are linked |
| `TestAViewerCannotRecord` | Vera → 403 `forbidden` `required: member` | a viewer's judgement recorded |
| `TestTheBriefKeyMustBeTheProjects` | a key of another project → 422 `key_unresolvable` | evidence pointing at a brief the project does not have |
| `TestBecauseRules` | `"   "` and 2,001 characters → 400 `field: because`; 2,000 → 201 | — |
| `TestByDigestIsOldestFirst` | three records over two digests; query both → oldest first; one digest → its records only | phase 3's evaluator taking the newest as the first judgement |
| `TestAnUnreadableRepoListsNothing` | Out Sider querying Grace's repo → 200, `items: []` | a 404 that confirms the link |
| `TestEvidenceSurvivesTheRemotesRemoval` | record; remove the remote (task 17); `GET /v1/evidence?repo=` as Grace → **no items** (the repo no longer maps to a readable project), but the row exists (owner read) and resolves again when the remote is re-added | a remote removal that deletes evidence (§7.4) |

Matrix `rowsTask25` (§5.3: `O`, `N`, `X` get `repo_not_linked`, not 404):

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `POST /v1/evidence` (repo {RepoKey}) | 403 | 201 | 201 | 422 `repo_not_linked` | 422 | 422 |
| `GET /v1/evidence?repo={RepoKey}&digest=…` | 200 (items) | 200 (items) | 200 (items) | 200 (0) | 200 (0) | 200 (0) |

#### Steps

- [ ] **Step 1:** Re-read contract §7.10, §14 (H-2, H-3, H-4); phase 3 task 15's `evidence.Record`
  (read-only, sibling `feat/phase-3-git`).
- [ ] **Step 2:** Tests and rows; they fail. Then `store/evidence.go`, `evidence.go`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestRecordEvidence|TestAnUnlinkedRepo|TestAViewerCannotRecord|TestTheBriefKeyMust|TestBecauseRules|TestByDigestIs|TestAnUnreadableRepo|TestEvidenceSurvives|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 10
```

#### What this task does not claim

- Phase 3's criterion state: it is computed locally and never stored (ADR-H1 §1.3).
- That phase 3's `criterionDigest` is computed canonically: phase 3's `DefinitionOfDone` owns it.
