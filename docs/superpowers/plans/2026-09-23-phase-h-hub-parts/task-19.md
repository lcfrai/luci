### Task 19: the batch revision write and the revision history — routes 25–26

**Lane A. Depends on: 18. About 3.5 hours (3, plus 0.5 for the stored form, `rebuildRevisions`
and their test, re-review b06 N1). Contract task H3.**

`POST /v1/projects/{projectId}/brief-revisions` is **the only write of brief text** (contract
§7.5). One call carries 1–`hubapi.MaxRevisionBatch` (64) revisions (HA4-3, task 00), and the batch is **one transaction, all or nothing**,
because a project-brief edit re-revisions every inheriting branch brief (critique-2 §7.1 #3,
accepted under `OD-1`) and a half-applied fan-out would leave branches disagreeing with their
parent. Optimistic concurrency is by `baseRevision`: the caller re-reads and retries; the Hub never
merges.

**Files:**
- `control-plane/internal/hub/server/revisions.go` + `revisions_integration_test.go` (new)
- `control-plane/internal/hub/store/revisions.go` (new)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask19`; the fixture's brief gets a complete revision, `BriefKey` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerRevisions()` (modify: one line)

#### Contract — the routes, complete

**Route 26 — `POST /v1/projects/{projectId}/brief-revisions`**, body `hubapi.WriteRevisionsRequest`,
role M, `Idempotency-Key` expected (the control plane always sends one, contract §9.2).

The algorithm, inside task 14's `WriteLimit` with `hubapi.MaxRevisionBatchBytes` (4 MiB; every
other route keeps `httpx.MaxBodyBytes`), so the audit lock is already held:

1. Decode; `revisions` has 1–`hubapi.MaxRevisionBatch` items (else 400 `field: revisions`) with
   **distinct** `briefId`s (else 400 `field: revisions`).
2. Authorise: M of the project (404 `project_not_found`; 403 `required: member`).
3. Validate each item's `content` with `hubapi.ValidateContent` (400 `bad_request`,
   `field: revisions[<i>].content`, the message naming the rule broken).
4. **Lock the heads in `briefId` order**: `SELECT … FROM hub.briefs WHERE id = ANY($1) AND
   project_id = $2 ORDER BY id FOR UPDATE`. A brief missing from the result is 404
   `brief_not_found` (not this project's, or unreadable — the same answer). An archived one is
   409 `brief_archived`.
5. For each item **in `briefId` order**: `baseRevision` must equal the locked `head_revision`, else
   the **whole batch** is 409 `revision_conflict` with `details.briefId` (the first failing, in
   `briefId` order) and `details.headRevision` (its current head, as a decimal string).
6. For each item: `revision = baseRevision + 1`; build `hubapi.BriefEnvelope{BriefID, Revision,
   ProjectID, Scope, BranchRef (the head's current ref), Content}`; its canonical bytes may not
   exceed 64 KiB (400, `field: revisions[<i>].content`); `digest = SHA-256(canonical)`; `INSERT`
   the revision with `saved_by` = the principal; `UPDATE` the head: `head_revision = revision`,
   `latest_complete_revision = revision` **only if** `status = "complete"`, `updated_at = now`.
7. `Emit` one `brief.revised {"key","revision","digest","status"}` per revision, **in request
   order**.
8. Answer `201 hubapi.WriteRevisionsResponse`, the revisions **in request order**, each rendered
   by task 18's `renderRevision` from the row step 6 inserted (content parsed out of its
   canonical, never copied from the request). `WriteResult.Stored` is the same response with
   every `content` set to `null`: **no stored response holds brief text** (A17, review I6,
   re-review b06 N1).

**The replay, `rebuildRevisions`.** Route 26 runs under task 14's `WriteReplayable` with this
rebuild. It decodes the stored form, and for each item, in order, reads the `brief_revisions` row
`(briefId, revision)` under row-level security (`store.RevisionAt`) and sets `content` and
`erasedAt` from `renderRevision(row, the stored savedBy)`. Every other field is the stored one. So a replay before erasure is
the first answer byte for byte. After erasure, an erased revision replays with `content: null` and
its `erasedAt`, which is what `/v1/refs` answers for it; `hubclient` takes it, since its re-check
skips a revision whose content is null (task 34's `VerifyRevision`). A co-batched brief that was
not erased replays whole. A row that is not found, or a stored form that does not decode, is 500 `internal`,
never the stored bytes: revisions are never deleted, so that is corruption.

Why rebuilt and not purged: `hub.erase_brief` then deletes no stored response (task 07b). A purge
also deleted the replays of every text-free response that names the brief (approvals, evidence,
reserves, abandons, co-batched briefs, the erase itself), and their retries became second writes
or refusals of a success.

Locking in `briefId` order is what makes two concurrent overlapping batches safe even without the
org lock; with it, they are simply serialised, and the second sees the first's heads and conflicts.

**Route 25 — `GET /v1/briefs/{briefId}/revisions?after=&limit=`**, role V: `200
Page[RevisionSummary]`, **oldest first, without content**; the cursor is the last revision number
returned. 404 `brief_not_found`.

```go
// control-plane/internal/hub/store/revisions.go
package store

type RevisionRow struct {
	OrgID, BriefID, ProjectID, Scope, BranchRef, Status, SavedBy string
	Revision                                                     int
	Canonical                                                    []byte // nil when erased
	Digest                                                       [32]byte
	SavedAt                                                      time.Time
	ErasedAt                                                     *time.Time
}

// LockHeads is step 4. It returns the heads found, keyed by id; the caller decides
// what an absent one means.
func LockHeads(ctx context.Context, tx pgx.Tx, projectID string, briefIDs []string) (map[string]BriefRow, error)

// InsertRevision and AdvanceHead are step 6's two statements.
func InsertRevision(ctx context.Context, tx pgx.Tx, r RevisionRow) error
func AdvanceHead(ctx context.Context, tx pgx.Tx, briefID string, revision int, complete bool, at time.Time) error

// Revisions is route 25's page.
func Revisions(ctx context.Context, q pgx.Tx, briefID string, afterRevision, limit int) ([]RevisionRow, error)

// RevisionAt is one row, for rebuildRevisions; found is false when no row is visible.
func RevisionAt(ctx context.Context, q pgx.Tx, briefID string, revision int) (r RevisionRow, found bool, err error)
```

```go
// control-plane/internal/hub/server/revisions.go
// storedRevisions is step 8's WriteResult.Stored: resp with every Content null.
func storedRevisions(resp hubapi.WriteRevisionsResponse) hubapi.WriteRevisionsResponse

// rebuildRevisions is route 26's task-14 Rebuild (the replay, above).
func rebuildRevisions(tx *Tx, stored []byte) (any, error)
```

#### Tests to write first — `revisions_integration_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheFirstRevisionIsOne` | base 0 → revision 1; `key` = `brief:<id>@1`; `digest` = the envelope's; the head's `headKey`, `headDigest`, `headStatus` now set | a revision number taken from the request |
| `TestAStaleBaseIsARevisionConflict` | base 0 twice → the second is 409 `revision_conflict`, `details.headRevision: "1"`, `details.briefId` | last-writer-wins silently discarding a teammate's answers |
| `TestTwoConcurrentWritersOneWins` | two goroutines, same brief, base 0 → exactly one 201 and one 409; revisions table holds 1 only; no gap | a race that writes 1 twice or skips to 2 (contract §13 H3) |
| `TestABatchIsAllOrNothing` | four briefs, item 3's base stale → 409 naming brief 3; **no** head moved and no revision row added | a half-applied fan-out (critique-2 §7.1 #3) |
| `TestABatchOfSixtyFourMaximumRevisions` | 64 briefs, each `content` canonical, holding ~60,000 raw `<`, `>` and `&` characters and a U+2028, its envelope just under 64 KiB; the body encoded as `hubclient` encodes it (`SetEscapeHTML(false)`, task 00's wire rule) → 201, 64 revisions, one transaction; **the 201 body is under 8 MiB** and holds each `content` raw; 65 small items → 400 `field: revisions`; a body of 4 MiB + 1 → 400 `field: body` before any lock | phase 4's fan-out refused past 16 (HA4-3); a body cap that refuses the batch the item cap allows; `server.WriteJSON` escaping HTML, which makes this reply over 20 MiB and a committed batch an `ErrUnavailable` at `hubclient`'s 8 MiB cap (task 08) |
| `TestTheConflictNamedIsTheFirstInIdOrder` | two stale items → the one with the smaller `briefId` is named, whatever the request order | a nondeterministic error a retry loop cannot act on |
| `TestTheResponseIsInRequestOrder` | a batch sent as [b3, b1, b2] → the response in that order; events in that order | a caller matching responses to requests by position and getting the wrong digest |
| `TestContentRules` | `"status":"done"`, a float, a duplicate key, 17-deep nesting, a 64 KiB + 1 envelope → 400 each, naming `revisions[<i>].content` | the Hub storing content it cannot digest canonically |
| `TestLatestCompleteMovesOnlyOnComplete` | open, complete, open → `latestCompleteRevision` 0, 2, 2 | Q1's pin rule fed an incomplete revision (ADR-H1 Q1) |
| `TestForBranchCarriesLatestComplete` | revisions 1 complete, 2 open → task 18's route 21 answers `revision` @2 and `latestComplete` @1 with its content; revision 3 complete → both @3; `GET /v1/briefs/{id}` the same | phase 4 materialising a session from an open revision (HA4-4, task 00) |
| `TestTheEnvelopeCarriesTheCurrentRef` | write r1, rebind (task 18), write r2 → r1's stored canonical has the old ref, r2's the new | a digest that does not cover the ref it was written under |
| `TestAWriteToAnArchivedBriefIs409` | → `brief_archived` | — |
| `TestAnotherProjectsBriefIs404` | a batch naming a brief of another project → 404 `brief_not_found` and nothing written | a write that crosses projects |
| `TestARetryWithTheSameKeyReplays` | the same `Idempotency-Key` and body → the same 201, one revision | a lost response turning into the caller's own `revision_conflict` (§7.5's last bullet) |
| `TestTheStoredReplayHoldsNoContent` | one batch, key k, briefs b1 (content holding the marker `QVX-5183`) and b2 (holding `QVX-6294`) → 201, both contents raw; the `idempotency_keys` row (read as the administrator) holds both `briefId`s and neither marker; k again → 201, `Idempotent-Replay: true`, byte-identical to the first. Then `SELECT hub.erase_brief(b1)` as `hub_app` with `hub.org_id` set (route 45 is task 27's); k again → 201, `Idempotent-Replay: true`: item b1 has `content: null` and `erasedAt` set, key and digest unchanged; item b2 equals the first answer's b2; the body holds no `QVX-5183` | `StoreResponse` given the full reply (A17: 24 hours of text); a replay that answers the stored form, with both contents null; a rebuild that reads content from the stored bytes, not the row; a purge that turns b2's replay into 409 `brief_archived` (re-review b06 N1) |
| `TestHistoryIsOldestFirstWithoutContent` | three revisions → `[1,2,3]`, no `content` key in any item; `after=1&limit=1` → `[2]` | a history that ships every revision's text |

Matrix `rowsTask19`:

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/briefs/{BriefID}/revisions` | 200 | 200 | 200 | 404 | 404 | 404 |
| `POST /v1/projects/{ProjectID}/brief-revisions` (base = head, per cell) | 403 | 201 | 201 | 404 | 404 | 404 |

#### Steps

- [ ] **Step 1:** Re-read contract §3.4, §7.5 (the batch), §10.6, §10.7.
- [ ] **Step 2:** Tests and rows; they fail. Then `store/revisions.go`, `revisions.go`.
- [ ] **Step 3:** Replace task 18's test-only owner inserts with real writes where they are now
  possible (keep one owner-inserted case: task 20's tamper test needs it).
- [ ] **Step 4:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestTheFirstRevision|TestAStaleBase|TestTwoConcurrent|TestABatchIs|TestTheConflictNamed|TestTheResponseIsIn|TestContentRules|TestLatestComplete|TestTheEnvelopeCarries|TestAWriteToAnArchived|TestAnotherProjects|TestARetryWith|TestTheStoredReplay|TestHistoryIs|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 16
```

#### What this task does not claim

- Key resolution and the digest re-check on read: task 20.
- Phase 4's fan-out logic — which branch briefs a project edit re-revisions: phase 4's; the Hub
  guarantees only that the batch is atomic.
