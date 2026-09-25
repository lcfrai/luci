### Task 09: Issue revisions, `serves`, restore and attachments; `/v1/refs` resolves `issue:`

Stream **WH**, seam **SW.1** (the revision's server side), about 3 hours (an estimate, not a
measurement). Depends on task 08 (the head, `checkServes`, `store.InsertIssueRevision`,
`readableIssue`) and Hub tasks **H19** (the batch revision write and its history), **H20**
(`/v1/refs`, the brief half) and **H21** (blobs), sync-merged into `feat/work-layer` first (rule H1).

This task makes an issue's text a **thread object**. After it, `issue:is-…@3` is a key the Hub
resolves exactly as it resolves `brief:bf-…@6`: resolved only when readable and only when its
stored bytes still hash to its digest. That is what lets task 19 tell an agent *which* revision it
was given and prove it later, byte for byte (the Hub half of L-191).

**Seven decisions.**

1. **A revision write is optimistic, like a brief's.** `WriteIssueRevisionRequest{baseRevision,
   content}` against a head that moved is 409 `revision_conflict`, and writes nothing. The head row
   is read `FOR UPDATE` (task 08's `readableIssue(…, lock=true)`), so two writers on base 3 give one
   201 and one 409, and revision numbers stay gapless (hub-contract's H3 row for briefs, `:2250`).
2. **Restore writes a new revision.** `POST …/revisions/{rev}/restore` writes revision *head+1*
   whose content is `rev`'s and whose `restoredFrom` is `rev`. `rev` itself is never touched: its
   digest and bytes are what an agent may have been told. Restoring an erased revision is 409
   `revision_erased`.
3. **A restore is optimistic too.** Task 03's `RestoreRevisionRequest{fromRevision}` gains
   `baseRevision`, **additively** (an added field is allowed after WS0, skeleton start line WS0):
   a person restores from the head they were looking at, and a restore racing an edit is 409, not a
   silent overwrite of the edit they never saw. `fromRevision` must equal the path's `{rev}` (400
   `bad_request`, `details.field = "fromRevision"`). Task 07's matrix row sends both.
4. **`serves` is checked at the key level, and the Hub does not read briefs' criteria.** Every
   revision runs task 08's `checkServes`: the brief key parses, the brief is in the **same project**,
   the revision exists and is not erased, and the criterion id is 1-128 characters. The scope also
   asks for *"its `criterionId` is a criterion of that brief revision"*. The Hub cannot answer that:
   brief content is opaque to it (`BriefRevision.Content json.RawMessage`, `types.go:176`) and
   criterion ids are positional, derived by phase 4's parser (provenance contract draft
   `:209-213`). **Decision:** the Hub checks what it can know; the membership check runs where the
   brief schema is known, in the writer (the desktop's `serves` picker, task 47, which only offers
   parsed criteria; and the control plane's relay, task 16, if its owner takes it). A Hub-side
   check would need a Hub reader of phase 4's schema that no Hub task plans. This is a **skeleton
   gap**, listed in the return. The thread cannot be broken across projects either way: another
   project's brief is refused by the key check.
5. **A revision with no `serves` needs a reason on the head.** Writing a revision whose `serves` is
   empty, on an issue whose `not_tied_reason` is `''`, is 422 `bad_request` naming `content.serves`
   (05 §2.3 point 4: *no silent open end*). The person sets `notTiedReason` by `PATCH` (task 08)
   first. This is the scope's "refused at serves removal" (task 10's paragraph), built where the
   removal happens.
6. **One event per revision, no text.** `issue.revised` `{key, revision, digest, servesCount,
   restoredFrom}` (task 13's name, SW.5); `restoredFrom` is `0` for an edit, since `data` holds no
   nulls. A write's reply is stored for replay with `content` null (task 08's decision 7), through
   `server.WriteReplayable`.
7. **An attachment is a project blob, named by digest.** `POST …/attachments {digest}` names a blob
   already uploaded to **this issue's project** (hub-contract routes 28-29, H21). An unknown digest,
   or one uploaded only to another project, is 404 `blob_not_found`, the same answer for both, so
   the route leaks nothing about other projects' files. A second attach of the same digest is 200
   with no event. Task 06's key makes a cross-project attachment a `23503` even with Go wrong.

**`/v1/refs` changes additively.** `server/refs.go` (H20's file, **UNVERIFIED until merge**)
gains one `case hubapi.KeyIssue`. `RefResult` gains `Issue *IssueRevision "issue"` (null for every
other kind) and `RefsResponse` gains `IssueHeads map[string]IssueHeadInfo "issueHeads"` (`{}` when
none), so the brief `heads` map keeps its meaning (`types.go:322-339`). The rules are §7.5's
(`:833-838`), unchanged: `resolved` when readable and its bytes re-hash to the stored digest;
`missing` when absent, unreadable, or failing the re-hash (logged as `internal`); `erased` with
`content: null` after task 14's erase.

**Files:**
- `control-plane/internal/hub/store/work_revisions.go` — the revision history, the attachment
  rows, the batch key read for `/v1/refs` (new)
- `control-plane/internal/hub/server/work_revisions.go` — the six routes and
  `resolveIssueKeys` (new)
- `control-plane/internal/hub/server/work_revisions_integration_test.go` — the tests below (new)
- `control-plane/internal/hub/server/refs.go` ⚑ — H20's file: one `case hubapi.KeyIssue`, and the
  `IssueHeads` map filled (modify, additive; **UNVERIFIED until merge**)
- `control-plane/internal/hub/hubapi/types.go` ⚑ — `RefResult.Issue`, `RefsResponse.IssueHeads`,
  `IssueHeadInfo` (modify, additive)
- `control-plane/internal/hub/hubapi/work.go` ⚑ — `RestoreRevisionRequest.BaseRevision`,
  `IssueAttachment`, `AttachRequest`, `CodeRevisionErased`, `EventIssueRevised`,
  `EventIssueAttached` (modify, additive)
- `control-plane/internal/hub/server/server.go` ⚑ — one line, `s.registerWorkRevisions()`
- `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` — `"09"` appended to
  `workMatrixLive`

**Interfaces:**
- Produces:
  - the routes `POST` and `GET /v1/issues/{issueId}/revisions`,
    `GET /v1/issues/{issueId}/revisions/{rev}`, `POST /v1/issues/{issueId}/revisions/{rev}/restore`,
    `POST` and `GET /v1/issues/{issueId}/attachments`; `issue:` keys on `GET /v1/refs`
  - `store.IssueRevisions`, `store.IssueRevisionsByKey`, `store.InsertAttachment`,
    `store.Attachments`, `store.ProjectBlob`
  - `(*Server).writeIssueRevision(ctx, tx, head store.IssueRow, content, restoredFrom)`, the one
    path that writes revision *n+1*, which task 15's import and W4's accepted proposals (task 141)
    call rather than writing their own
- Consumes:
  - task 08: `readableIssue`, `issueWithRevision`, `checkServes`, `store.InsertIssueRevision`,
    `store.MoveIssueHead`, `store.IssueRevisionAt`, `store.RevisionRow`, `threadStateOf`
  - task 02: `IssueKey`, `ParseIssueKey`, `IssueRevisionEnvelope`, `IssueRevisionDigest`,
    `ValidateIssueRevisionContent`; task 03: `IssueRevision`, `WriteIssueRevisionRequest`,
    `RestoreRevisionRequest`
  - H20: the resolver's loop over keys, its `heads` map and its re-hash-and-log path; H21: the
    `project_blobs` row a `PUT` of a blob writes; H14: `WriteReplayable`, `Read`, `Emit`
- Requires of later tasks:
  - **Task 14** makes `erased` answer for an erased issue key; this task already reads
    `canonical IS NULL` as `erased`, so task 14 needs no edit here.
  - **Task 16** adds `baseRevision` to its `RestoreIssueRevision` call (the added field).
  - **Task 18** makes phase 4's parser accept `issue:`; until then the control plane cannot ask.

#### Wire shapes

```jsonc
// POST /v1/issues/is-…/revisions                        member; 201; Idempotency-Key honoured
{ "baseRevision": 3,
  "content": { "title": { "text": "Resume a dropped upload", "author": "H" },
               "description": { "text": "Resume from the last acknowledged chunk.", "author": "H" },
               "serves": [ { "briefKey": "brief:bf-3f2a91c07b1e4d2a@6", "criterionId": "done:2" } ],
               "implements": [] } }
// → 201 IssueRevision (content included); 409 revision_conflict {"details":{"headRevision":"4"}}

// GET /v1/issues/is-…/revisions?after=&limit=           viewer; Page[IssueRevision], newest first,
//                                                        every content null (the list is a history)
// GET /v1/issues/is-…/revisions/2                        viewer; IssueRevision; content null if erased

// POST /v1/issues/is-…/revisions/2/restore              member; 201 IssueRevision (restoredFrom 2)
{ "fromRevision": 2, "baseRevision": 5 }

// POST /v1/issues/is-…/attachments                      member; 201 new, 200 already attached
{ "digest": "sha256:…" }
// → { "issueId": "is-…", "digest": "sha256:…", "contentType": "image/png", "size": 48213,
//     "addedAt": "…", "addedBy": { … } }
// GET /v1/issues/is-…/attachments                       viewer; Page[IssueAttachment], oldest first

// GET /v1/refs?key=issue:is-…@3&key=brief:bf-…@6
{ "results": [ { "key": "issue:is-…@3", "kind": "issue", "resolution": "resolved",
                 "revision": null, "approval": null, "issue": IssueRevision },
               { "key": "brief:bf-…@6", "kind": "brief", "resolution": "resolved",
                 "revision": BriefRevision, "approval": null, "issue": null } ],
  "heads": { "bf-…": { … } },
  "issueHeads": { "is-…": { "headRevision": 5, "erased": false } } }
```
