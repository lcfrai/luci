### Task 18: brief heads — create, list, for-branch, get, rebind, archive (routes 19–24)

**Lane A. Depends on: 17. About 3 hours. Contract task H3.**

A brief head is the living brief: project-scoped, or branch-scoped by its **git ref**
(`refs/heads/…`), the one branch name teammates share through the remote (contract §3.3). The
Hub stores heads and immutable revisions, numbers and digests them; phase 4's domain logic — the
questions, the inheritance, what makes a brief complete — stays in the control plane (§7.5).
Revisions are task 19; this task is the heads and the reads phase 4's `API.resolve` needs.

**Files:**
- `control-plane/internal/hub/server/briefs.go` + `briefs_integration_test.go` (new)
- `control-plane/internal/hub/store/briefs.go` (new)
- `control-plane/internal/hub/server/render.go` — row → `hubapi.BriefHead` / `BriefRevision` (new; task 19 and 20 reuse it)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask18`; the fixture gains `BriefID` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerBriefs()` (modify: one line)

#### Contract — the routes, complete

| # | Route | Role | Success | Refusals | Events |
|---|---|---|---|---|---|
| 19 | `GET /v1/projects/{projectId}/briefs?scope=&branchRef=&includeArchived=false&after=&limit=` | V | `200 Page[BriefHead]`, oldest first | 404 `project_not_found`; 400 `field: scope` / `branchRef` | — |
| 20 | `POST /v1/projects/{projectId}/briefs` `CreateBriefRequest` | M | `201 BriefHead`, `headRevision: 0`, every `head*` field `null` | 404; 403 `required: member`; 400 `field: scope`; 400 `field: branchRef` (a project brief with a ref; a branch brief with none or an invalid one, `hubapi.ValidBranchRef`); 409 `brief_exists`, `details.briefId` = the live one | `brief.created {"scope","branchRef"}` |
| 21 | `GET /v1/projects/{projectId}/briefs/for-branch?branchRef=refs/heads/…` | V | `200 ForBranch` — `branchBrief`: the live branch brief for that ref with its head revision; `projectBrief`: the live project brief with its head revision; either `null` when absent; each also carries `latestComplete` (HA4-4, task 00: `null` when `latestCompleteRevision` is 0). **Never 404 for an absent brief** | 404 `project_not_found` only; 400 `field: branchRef` | — |
| 22 | `GET /v1/briefs/{briefId}` | V | `200 BriefWithRevision` with `latestComplete` (archived briefs included) | 404 `brief_not_found` | — |
| 23 | `PATCH /v1/briefs/{briefId}` `RebindRequest` | M | `200 BriefHead` with the new ref; no revision written | 404; 403; 400 (a project brief; an invalid ref); 409 `brief_exists` (another live brief on the new ref); 409 `brief_archived` | `brief.rebound {"branchRef","previousBranchRef"}` |
| 24 | `POST /v1/briefs/{briefId}/archive` `{}` | M | `200 BriefHead`, `archivedAt` set; every key still resolves; the ref is free | 404; 403; 409 `brief_archived` (already archived) | `brief.archived {}` |

- **A head's `head*` fields** (`headKey`, `headDigest`, `headStatus`) are `null` exactly while
  `headRevision` is 0 (contract §7.1); `latestCompleteRevision` is `0` while none is complete.
- **Rebind after a branch rename** changes `branch_ref` only; the next revision's envelope carries
  the new ref (§3.4) and earlier revisions keep the old one — each revision's digest covers the
  ref it was written under.
- **Archive after branch deletion** (provenance P13) touches no revision, and there is no
  un-archive in v1.
- **Uniqueness** is the database's (`briefs_live_project`, `briefs_live_branch`, task 07a): the
  handler turns the unique violation into 409 `brief_exists`, reading the live brief's id in the
  same transaction for `details.briefId`.

```go
// control-plane/internal/hub/server/render.go
package server

// renderHead and renderRevision are the one place a row becomes JSON, so every
// route agrees on null-versus-empty (contract §7.1) and on the digest's wire form.
// renderRevision parses content out of the stored canonical bytes (contract §3.4:
// "the Hub serves content by parsing C, never from a second copy"); an erased
// revision renders content null and keeps key, digest, savedAt, savedBy.
func renderHead(h store.BriefRow, createdBy hubapi.User) hubapi.BriefHead
func renderRevision(r store.RevisionRow, savedBy hubapi.User) (hubapi.BriefRevision, error)
```

#### Tests to write first — `briefs_integration_test.go`

Revisions are needed for some assertions; until task 19 lands, the tests insert them through the
owner connection (`hubtest.OwnerConn`) with a correct canonical and digest built by
`hubapi.BriefEnvelope` — the database's CHECK accepts nothing else.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCreateAProjectBriefAndABranchBrief` | both 201 with `headRevision: 0` and `null` head fields; events `brief.created` × 2 | a head that invents a revision |
| `TestScopeAndRefRules` | a project brief with `refs/heads/x` → 400; a branch brief with `main` (not a full ref) or `refs/heads/a..b` → 400; a local `b-3f2a91c0` id → 400 | a laptop's path-hashed `BranchID` reaching the Hub (§3.3) |
| `TestOneLiveBriefPerScope` | a second project brief, and a second branch brief on one ref → 409 `brief_exists` naming the first | two living briefs for one branch, with no rule for which Materialise renders |
| `TestForBranchNeverFails` | no briefs → `{branchBrief: null, projectBrief: null}`; only a project brief → `projectBrief` set; both → both, each with its head revision | a 404 that phase 4 renders as data loss |
| `TestForBranchIgnoresArchived` | archive the branch brief → `branchBrief: null` | a deleted branch's brief still steering sessions |
| `TestRebind` | rebind to `refs/heads/new` → 200, the new ref, the event with both refs; a project brief → 400; onto a ref with a live brief → 409 `brief_exists` | a rename that loses the brief |
| `TestArchiveFreesTheRef` | archive → 200; a new branch brief on the same ref → 201; the archived one is still readable by id; archiving again → 409 `brief_archived` | an archive that deletes |
| `TestReadsRenderContentFromCanonical` | a revision inserted as above → `GET /v1/briefs/{id}` returns `content` equal to the canonical envelope's `content`, byte for byte after `hubapi.Canonical` | a second copy of content drifting from the digested bytes |
| `TestListFiltersAndOrder` | three briefs; `scope=branch` → the branch ones; `includeArchived=false` hides the archived; oldest first | — |
| `TestForBranchLatestCompleteIsNullWithoutRevisions` | a fresh brief → `revision: null`, `latestComplete: null`, both keys present. The rows with revisions are task 19's `TestForBranchCarriesLatestComplete`, since revisions are written there | a response that omits the key phase 4 reads (HA4-4) |

The matrix gains `rowsTask18`, from §5.3 ("read a project and anything under it"; "create a brief;
write revisions; rebind; archive"):

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/projects/{ProjectID}/briefs` | 200 | 200 | 200 | 404 | 404 | 404 |
| `POST /v1/projects/{ProjectID}/briefs` (a fresh ref per cell) | 403 | 201 | 201 | 404 | 404 | 404 |
| `GET …/briefs/for-branch?branchRef=refs/heads/main` | 200 | 200 | 200 | 404 | 404 | 404 |
| `GET /v1/briefs/{BriefID}` | 200 | 200 | 200 | 404 | 404 | 404 |
| `PATCH /v1/briefs/{BriefID}` (a fresh ref per cell) | 403 | 200 | 200 | 404 | 404 | 404 |
| `POST /v1/briefs/{BriefID}/archive` | 403 | 200 | 200 | 404 | 404 | 404 |

#### Steps

- [ ] **Step 1:** Re-read contract §3.3, §7.1, §7.5 (heads), §8; the phase 4 plan's
  `discover.Resolve` pairing (`2026-09-18-phase-4-discover.md:4250-4261`, as the contract cites it).
- [ ] **Step 2:** Tests and rows; they fail. Then `store/briefs.go`, `render.go`, `briefs.go`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestCreateAProjectBrief|TestScopeAndRef|TestOneLiveBrief|TestForBranch|TestRebind|TestArchiveFrees|TestReadsRender|TestListFilters|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 11
```

#### What this task does not claim

- Revision writes: task 19. Key resolution: task 20.
- Phase 4's resolution rule (a branch brief if one exists, else the project brief): the Hub hands
  back both; phase 4 decides (contract §7.5, "the division of labour").
