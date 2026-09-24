### Task 27: the audit route, erasure (graft G6), and `hub verify`'s chain walk — routes 44–45

**Lane A. Depends on: 26, 19. About 2.5 hours. Contract task H7.**

Two org-admin powers and one check. An org admin — the Entra app role `Hub.OrgAdmin`, assigned by
IT, read from each token — reads the org's audit log and **erases** a brief. Erasure is a
tombstone (G6): every revision keeps its **key, digest, number, author and time**, so every
approval, pin and piece of evidence that names it still resolves — as **`erased`**, a state
distinct from `missing` (ADR-H1 §5.6, Q2). An org admin gets **no read of project content** it is
not a member of: erasure addresses a key, not a text (§5.2). `hub verify` walks every org's hash
chain and exits non-zero at the first broken link (G5; correction A10 adds `teamId` and the `feed`
flag to what it checks).

**Files:**
- `control-plane/internal/hub/server/audit.go`, `erase.go` + `audit_integration_test.go` (new)
- `control-plane/internal/hub/store/erase.go`, `store/auditread.go` (new)
- `control-plane/cmd/hub/verify.go` — register the `audit` check (modify)
- `control-plane/cmd/hub/verify_integration_test.go` — the chain tests (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask27` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerAudit()` (modify: one line)

#### Contract — the routes, complete

**Route 44 — `GET /v1/audit?after=<seq>&limit=<1–500>`**, org admin only (403 `forbidden`,
`required: org_admin`, for everyone else — members included). `200 hubapi.AuditPage`: the same shape
as the change feed (task 26) **plus** `hash` and `prevHash` (64 lowercase hex) on each event, **every**
kind including the audit-only ones, and `brief.erased`'s `reason`. It runs under `Write` (task 14),
because reading the audit **is audited**: after the page is read it appends `audit.read
{"after","count"}` (audit only), subject the org id.

**Route 45 — `POST /v1/briefs/{briefId}/erase`**, body `hubapi.EraseRequest` (`reason` 1–500
characters after trimming, else 400), org admin only (403 `required: org_admin`). The brief must be
in the admin's org — found by id under row-level security alone, **with no team check** — else 404
`brief_not_found`. In one transaction: `SELECT hub.erase_brief($1)` (task 07b's `SECURITY DEFINER`
function: content removed and `erased_at` set on every revision still holding content; the head
archived if it was not; `erased_at` set on the head); `Emit brief.erased {"erasedRevisions",
"reason"}` with the brief's `teamId` and `projectId`. `200 hubapi.EraseResponse` —
`erasedRevisions` is the count **this** call erased (a second erase answers `0`, and is still
audited: an erasure attempt is a security event, ADR-H1 §5.6), `erasedAt` the brief's first erasure
time. Irreversible. **Nothing else changes**: approvals, releases, evidence and session links hold
keys and digests, never brief text (§7.12). Stored idempotency responses are not changed either,
because none holds text: route 26 stores its reply with every `content` null and rebuilds content
on replay from `brief_revisions` (A17, review I6, tasks 14 and 19). So a retry of a revision write
after erasure is still a replay, `201` with `Idempotent-Replay: true`, and the erased revision in
it has `content: null`, as `/v1/refs` answers. Every other stored replay, co-batched briefs and
this route's own included, survives the erasure (re-review b06 N1).

**`hub verify`'s `audit` check.** For each org (listed by `SELECT * FROM hub.org_ids()`, not by
reading `orgs`, which `hub_app` sees empty before a `set_config`: review M10, A18), walk
`audit_events` by `seq`:

1. `seq` is gapless from 1;
2. `prev_hash` is the previous row's `hash` (32 zero bytes for `seq` 1);
3. `hash = SHA-256(prev_hash ‖ canonical)`;
4. `canonical` parses as a `hubapi.AuditEnvelope` whose `orgId`, `seq`, `at`, `event`, `teamId`,
   `projectId`, `subject` and `actorUserId` equal the row's columns (A10);
5. `feed = store.InFeed(kind)` (A10);
6. finally, `audit_heads` holds the last row's `seq` and `hash`.

The first fault prints `FAULT audit: org <id> seq <n>: <which rule>` and exits 1; a healthy chain
prints `ok audit (<n> events in <m> orgs)`.

#### Tests to write first

`audit_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheAuditIsForOrgAdminsOnly` | Grace (team admin) → 403 `required: org_admin`; Ada → 200 | a team admin reading every team's history |
| `TestTheAuditShowsEverythingWithHashes` | after a refused write and a blob upload, Ada's page holds `auth.forbidden` and `blob.uploaded`, and each event's `prevHash` is the previous one's `hash` | an audit that hides security events |
| `TestReadingTheAuditIsAudited` | read page 1; read again → the second page ends with an `audit.read` event naming the first read's `after` and `count` | an unaccountable reader of everyone's actions |
| `TestErasureKeepsKeyAndDigest` | a brief A with two revisions, each written by route 26 with an `Idempotency-Key` (`kA1`, `kA2`) and content holding the marker `ZQX-MATTER-4471`; `kA2`'s batch also revises a brief B of the same project (B's content has no marker), so one stored response names both; an approval against A`@1` written with `kP` (route 30); a release pinned to A`@2`. **Before** erasure, `markerHits` finds the marker in exactly `brief_revisions.canonical` (this proves the scan can see a `bytea` column; `idempotency_keys.response` is one too, and holds none, task 19). Ada erases A with key `kE`, with a `reason` that does not hold the marker (the audit keeps the reason by design) → `erasedRevisions: 2`; `/v1/refs` of both keys → `erased`, content `null`, digests unchanged; the approval and the release still read, their keys resolving `erased`. **After** erasure, `markerHits` is empty, and every `idempotency_keys` row of before is still there. Each key sent again with its body replays, `Idempotent-Replay: true`: `kA2` → 201, item A has `content: null` and `erasedAt` set, item B equals the first answer's, no marker in the body; `kP` → the first answer byte for byte, and still one approval row; `kE` → 200, `erasedRevisions: 2`, and still one `brief.erased` event (re-review b06 N1). `markerHits(t, adminURL, marker)` lists every `text`, `varchar`, `jsonb` and `bytea` column of schema `hub` from `information_schema.columns` and counts rows holding the marker (`strpos(col::text, marker) > 0` on `text`, `varchar` and `jsonb`; `position(convert_to(marker, 'UTF8') IN col) > 0` on `bytea`; the marker is ASCII, so no encoding hides it), as the administrator (`hubtest.DB.AdminURL`), so row-level security hides nothing | an erasure that breaks the thread (`missing`); one that leaves text behind in `brief_revisions` **or in a stored response** (review I6: make task 19's `WriteResult.Stored` the full reply, and `markerHits` names `idempotency_keys.response`); a purge by the brief id, which deletes all three replays: `kA2` re-runs into 409 `brief_archived` though B's revision was written, `kP` into 422 `key_unresolvable`, and `kE` answers `erasedRevisions: 0` with a second `brief.erased` |
| `TestErasureIsForOrgAdminsOnly` | Grace → 403 `required: org_admin` | a team member destroying provenance |
| `TestAnOrgAdminCannotEraseAcrossOrgs` | Ada erasing a brief Xavier created in org 2 → 404 `brief_not_found` | an owner function reaching another tenant (T3's residual) |
| `TestASecondErasureIsZeroAndAudited` | erase twice → `2`, then `0`; two `brief.erased` events | an attempt that leaves no trace |
| `TestTheFeedHidesTheReason` | Grace's feed shows `brief.erased` with `erasedRevisions` and **no** `reason`; Ada's audit shows the reason | a client's matter number broadcast to the team |
| `TestErasureArchivesTheHead` | the head reads `archivedAt` set; writing a revision → 409 `brief_archived` | new text written into an erased brief |

`verify_integration_test.go` (added to task 20's):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestVerifyPassesOnAHealthyChain` | two orgs, 30 events → exit 0, exactly `ok audit (30 events in 2 orgs)` | a walk that lists orgs by `SELECT id FROM hub.orgs` and passes on zero (review M10) |
| `TestVerifyCatchesAnEditedEvent` | as the owner: drop `audit_events_hash_chains`, disable `audit_events_append_only`, alter one `canonical` → exit 1, rule 3 | the contract's H7 acceptance: *"fails after an owner-role edit of one audit row"* |
| `TestVerifyCatchesAMovedTeam` | the same, but change only `team_id` → exit 1, rule 4 | correction A10: an event moved into another team's feed without breaking the hash |
| `TestVerifyCatchesAFlippedFeedFlag` | flip `feed` on an `auth.forbidden` row → exit 1, rule 5 | an audit-only event leaked into the feed by editing one boolean |
| `TestVerifyCatchesADeletedEvent` | delete the middle row (triggers disabled) → exit 1, rule 1 or 2 | a silently removed history entry |

Matrix `rowsTask27`:

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/audit` | 403 | 403 | 403 | 200 | 403 | 403 |
| `POST /v1/briefs/{BriefID}/erase` (a fresh brief per cell) | 403 | 403 | 403 | 200 | 403 | 403 |

#### Steps

- [ ] **Step 1:** Re-read contract §7.12, §8, §10.4 (`erase_brief`), §12.1 (`hub verify`); ADR-H1 G5,
  G6, Q2.
- [ ] **Step 2:** Tests and rows; they fail. Then the stores, `audit.go`, `erase.go`, the verify check.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestTheAuditIs|TestTheAuditShows|TestReadingTheAudit|TestErasure|TestAnOrgAdminCannot|TestASecondErasure|TestTheFeedHides|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 11
go test -tags integration ./control-plane/cmd/hub/ -run TestVerify -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 8 (3 from task 20 + 5)
go test -tags integration ./control-plane/internal/hub/... ./control-plane/cmd/hub/ -race -count=1                # ok: the whole server
```

#### What this task does not claim

- Anchoring the chain head outside the database: later (ADR-H1 §5.9 item 4). Until then an owner
  who rewrites **every** row after a point, consistently, is caught only by comparing with a copy.
- Erasure of anything but brief text: nothing else holds it in v1 (§7.12).
- That erasure removes the text from **storage** (review M12). `erase_brief`'s `UPDATE … SET
  canonical = NULL` writes new row versions; the old ones stay in the table's pages as dead tuples
  until `VACUUM` reclaims them, the text stays in the write-ahead log, and every backup taken
  before the erasure keeps it for that backup's whole life. What erasure promises is that no live
  row and no replay answer holds the text (G6); `markerHits` checks live rows only. **The real
  erasure horizon is the database's backup retention**, which phase 8 sets: 7 days in dev and 35
  days in prod, with geo-redundant copies in prod (phase 8 task 08, `dev.tfvars`/`prod.tfvars`,
  `:229-232` on `feat/env` at `30b2419`; its variable admits 7 to 35). Azure's point-in-time
  restore keeps the log for the same window (from knowledge, unverified). So text erased today
  can be restored for up to 35 days in prod, and an erasure request is complete only after that
  window. Locally (`postgres:16-alpine`, task 09) there are no backups; the dead tuples and the
  log remain until `VACUUM` and a checkpoint recycle them.
- That the owner confirmed Q2's rule: the recommendation is built; the question stays open.
