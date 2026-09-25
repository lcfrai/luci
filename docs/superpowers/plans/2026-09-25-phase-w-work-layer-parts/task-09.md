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

#### `hubapi`, the additions (complete)

```go
// ── types.go (Hub task 03's file), additive: the resolver learns issue: keys ──
//   RefResult:    Issue      *IssueRevision           `json:"issue"`      // null unless kind is issue
//   RefsResponse: IssueHeads map[string]IssueHeadInfo `json:"issueHeads"` // by issue id; {} when none

// IssueHeadInfo is the time-varying part of an issue key's answer, as HeadInfo is
// for briefs: the caller caches the result forever and this per issue.
type IssueHeadInfo struct {
	HeadRevision int  `json:"headRevision"`
	Erased       bool `json:"erased"`
}

// ── work.go (task 03's file), additive ──────────────────────────────────────
//   RestoreRevisionRequest: BaseRevision int `json:"baseRevision"` // decision 3; required, ≥ 1

// AttachRequest names a blob already uploaded to the issue's project.
type AttachRequest struct {
	Digest Digest `json:"digest"`
}

// IssueAttachment is one attached file. Content type and size are the blob's.
type IssueAttachment struct {
	IssueID     string `json:"issueId"`
	Digest      Digest `json:"digest"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
	AddedAt     string `json:"addedAt"`
	AddedBy     User   `json:"addedBy"`
}

const CodeRevisionErased = "revision_erased" // 409, details.revision

const (
	EventIssueRevised  = "issue.revised"  // {key, revision, digest, servesCount, restoredFrom}
	EventIssueAttached = "issue.attached" // {digest}
)
```

`AttachRequest.Digest` joins the required-digest rows of `types_test.go` (`:574`), so a missing or
`null` digest is 400 naming `digest`, as every Hub digest field is.

#### `store/work_revisions.go` (the contract)

```go
package store

// IssueRevisions is newest first, keyset by revision; Canonical is not read (the
// list carries no content). Limit 1-200, default 50.
func IssueRevisions(ctx context.Context, q Querier, issueID string, pr hubapi.PageRequest) ([]RevisionRow, *string, error)

// IssueRevisionsByKey reads every (issue, revision) pair in one query, with the
// issue's project and head, for the resolver. Absent pairs are absent from the map.
func IssueRevisionsByKey(ctx context.Context, q Querier, keys []hubapi.IssueKey) (map[hubapi.IssueKey]KeyedRevision, error)

type KeyedRevision struct {
	RevisionRow
	HeadRevision int
	IssueErased  bool
}

// ProjectBlob reads the project_blobs row joined to blobs.size; found=false for a
// digest this project never uploaded (whatever other projects hold).
func ProjectBlob(ctx context.Context, q Querier, projectID string, d hubapi.Digest) (contentType string, size int, found bool, err error)

// InsertAttachment: INSERT … ON CONFLICT (issue_id, digest) DO NOTHING; inserted=false on a repeat.
func InsertAttachment(ctx context.Context, tx pgx.Tx, org, issueID, projectID string, d hubapi.Digest, by string, at time.Time) (inserted bool, err error)
func Attachments(ctx context.Context, q Querier, issueID string, pr hubapi.PageRequest) ([]AttachmentRow, *string, error)

type AttachmentRow struct {
	Digest      hubapi.Digest
	ContentType string
	Size        int
	AddedAt     time.Time
	AddedBy     string
}
```

#### `server/work_revisions.go` (the one revision path written out whole)

```go
package server

func (s *Server) registerWorkRevisions() {
	s.route("POST /v1/issues/{issueId}/revisions", s.postIssueRevision)
	s.route("GET /v1/issues/{issueId}/revisions", s.listIssueRevisions)
	s.route("GET /v1/issues/{issueId}/revisions/{rev}", s.getIssueRevision)
	s.route("POST /v1/issues/{issueId}/revisions/{rev}/restore", s.restoreIssueRevision)
	s.route("POST /v1/issues/{issueId}/attachments", s.attachToIssue)
	s.route("GET /v1/issues/{issueId}/attachments", s.listAttachments)
}

// writeIssueRevision writes revision head.HeadRevision+1 and moves the head. The
// caller holds the head FOR UPDATE and has checked baseRevision and the role.
// restoredFrom is 0 for an edit. It is the only writer of revision n+1.
func (s *Server) writeIssueRevision(ctx context.Context, tx *Tx, head store.IssueRow,
	content hubapi.IssueRevisionContent, restoredFrom int) (hubapi.IssueRevision, error) {
	if err := hubapi.ValidateIssueRevisionContent(content); err != nil {
		return hubapi.IssueRevision{}, Refuse(400, hubapi.CodeBadRequest, err.Error(), fieldOf(err, "content"))
	}
	if err := checkServes(ctx, tx, head.ProjectID, content.Serves, "content.serves"); err != nil {
		return hubapi.IssueRevision{}, err
	}
	if len(content.Serves) == 0 && head.NotTiedReason == "" { // decision 5
		return hubapi.IssueRevision{}, Refuse(422, hubapi.CodeBadRequest,
			"Say why this issue is not tied to a criterion before removing its last one.",
			map[string]string{"field": "content.serves"})
	}
	next := head.HeadRevision + 1
	env := hubapi.IssueRevisionEnvelope{V: 1, Kind: "issue-revision", IssueID: head.ID,
		Revision: next, ProjectID: head.ProjectID, Content: content}
	digest, canonical, err := hubapi.IssueRevisionDigest(env)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	var from *int
	if restoredFrom > 0 {
		from = &restoredFrom
	}
	if err := store.InsertIssueRevision(ctx, tx, store.RevisionRow{OrgID: tx.P.OrgID, IssueID: head.ID,
		ProjectID: head.ProjectID, Revision: next, Canonical: canonical, Digest: digest,
		WrittenAt: tx.Now, WrittenBy: tx.P.UserID, RestoredFrom: from, Serves: content.Serves}); err != nil {
		return hubapi.IssueRevision{}, err
	}
	if err := store.MoveIssueHead(ctx, tx, head.ID, next, tx.Now); err != nil {
		return hubapi.IssueRevision{}, err
	}
	key := hubapi.IssueKey{IssueID: head.ID, Revision: next}.String()
	tx.Emit(Event{Kind: hubapi.EventIssueRevised, TeamID: head.TeamID, ProjectID: head.ProjectID,
		Subject: head.ID, Data: map[string]any{"key": key, "revision": int64(next),
			"digest": digest.String(), "servesCount": int64(len(content.Serves)),
			"restoredFrom": int64(restoredFrom)}})
	return s.revisionOut(ctx, tx, head.ID, next) // decodes the stored canonical bytes, never content
}

// resolveIssueKeys answers every issue: slot of one /v1/refs request, in place.
// One IssueRevisionsByKey query; one ProjectRole per distinct project; a revision
// whose canonical bytes do not hash to its digest is missing and logged internal.
func (s *Server) resolveIssueKeys(ctx context.Context, tx *Tx, slots []int, keys []hubapi.IssueKey,
	out *hubapi.RefsResponse) error

func (s *Server) postIssueRevision(w http.ResponseWriter, r *http.Request)    // WriteReplayable; member; decision 1
func (s *Server) listIssueRevisions(w http.ResponseWriter, r *http.Request)   // Read; viewer; content null
func (s *Server) getIssueRevision(w http.ResponseWriter, r *http.Request)     // Read; viewer; 404 issue_not_found for an absent rev too
func (s *Server) restoreIssueRevision(w http.ResponseWriter, r *http.Request) // WriteReplayable; member; decisions 2-3
func (s *Server) attachToIssue(w http.ResponseWriter, r *http.Request)        // Write; member; decision 7
func (s *Server) listAttachments(w http.ResponseWriter, r *http.Request)      // Read; viewer
```

`revisionOut` decodes the reply's `content` from the canonical bytes just stored, so what the caller
reads is exactly what was hashed; a reply built from the request's struct could differ from the
stored bytes (a normalised string, a dropped field) and the client's re-hash (task 16's `checked`)
would then fail on a correct write. `fieldOf` maps `ValidateIssueRevisionContent`'s error to its
JSON path; task 02 names the field in the error, and if it does not, the whole `content` is named.

#### Tests to write first — `server/work_revisions_integration_test.go` (`//go:build integration`)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTwoWritersOnOneBase` | an issue at head 3; two writers with `baseRevision: 3` at once → one 201, one 409 `revision_conflict`; revisions are exactly 1..4 | an unlocked head read (last writer wins, or a gap) |
| `TestRestoreWritesANewRevision` | head 5; restore `@2` with `baseRevision: 5` → 201 revision 6, `restoredFrom: 2`, content equal to `@2`'s; `@2`'s digest and `written_at` unchanged; one `issue.revised` with `restoredFrom` 2 | an in-place restore; a restore that rewrites history |
| `TestAStaleRestoreIsRefused` | head 5; restore `@2` with `baseRevision: 4` → 409 `revision_conflict`, head still 5; `fromRevision: 3` on path `@2` → 400 naming `fromRevision` | a restore that overwrites an edit its author never saw (decision 3) |
| `TestServesMustStayInTheProject` | `serves` naming a brief of another project → 422 `key_unresolvable`, `details.field = "content.serves[0]"`; an erased brief revision → 422; a 129-character criterion id → 422 | an issue serving a criterion it cannot see, which breaks the thread |
| `TestRemovingTheLastServesNeedsAReason` | an issue serving one criterion with `not_tied_reason` `''`; a revision with `serves: []` → 422 naming `content.serves`; after `PATCH notTiedReason: spike` → 201, `threadState` `{not_tied, spike}` | a silent open end (05 §2.3 point 4) |
| `TestRefsAnswerEachSlotInOrder` | 100 keys mixing `brief:`, `design:` and `issue:`, one repeated → 100 results in request order; the repeated key answered in both slots; `issueHeads` has one entry per resolved or erased issue | a resolver that sorts, dedupes or drops the issue half |
| `TestAnotherTeamsIssueKeyIsMissing` | Out Sider asks `/v1/refs` for Grace's `issue:is-…@1` → `missing`, `issue: null`, no `issueHeads` entry; not 403 | a readability leak (hub-contract T4 `:2072`) |
| `TestATamperedRevisionIsMissing` | as `hub_owner` in the test's own scratch database: drop `issue_revisions_canonical_hashes`, disable `issue_revisions_guard`, change one byte of `@2`'s `canonical`, re-enable the trigger; `/v1/refs` for `@2` → `missing`, and one `internal` log line names the key | a resolver that trusts the stored digest without re-hashing |
| `TestAnErasedKeyReadsErased` | `erase_issue` (task 07's function, called directly) then `/v1/refs` → `erased`, `issue.content: null`, key and digest present; `GET …/revisions/1` → content null | erase answered as `missing`, which loses the fact that the key was real |
| `TestAttachmentsAreThisProjectsBlobs` | a blob uploaded to `Project` → 201, then 200 on repeat with one `issue.attached`; a digest never uploaded → 404 `blob_not_found`; a digest uploaded only to `Project2` → 404 `blob_not_found`, same body | attaching someone else's file; a 403 that says it exists |
| `TestTheListCarriesNoContent` | `GET …/revisions` → every item's `content` is `null`; `GET …/revisions/3` → content present | the history route leaking every revision's text in one call |
| `TestAReplayHoldsNoRevisionText` | a revision with a canary description; its `idempotency_keys.response` has no canary; the replay's body does | the whole reply stored (task 08's decision 7) |
| `TestTheReplyIsTheStoredBytes` | the 201's `content` re-hashes with `IssueRevisionDigest` to its `digest` | a reply built from the request struct instead of the stored bytes |

`TestATamperedRevisionIsMissing` alters constraints in its own `hubtest` scratch database only
(H06 drops it at cleanup); it never runs against a shared database. Task 08's
`TestAPatchWritesNoRevision` is the scope's "50 `PATCH`es leave `headRevision` unchanged" and is not
repeated here.

#### Steps

- [ ] **Step 1: Check the base.** Read H19's revision write, H20's `refs.go` and its re-hash-and-log
  path, H21's blob upload, all on the sync base. If `refs.go` is not there, the task **waits**
  (rule H1: its `dependsOn` names H20); it never writes a second resolver.
- [ ] **Step 2: The tests first**; add `"09"` to `workMatrixLive`. Run: they fail.
- [ ] **Step 3: The `hubapi` additions**, then the store, then the routes and the one `case` in
  `refs.go`, then the line in `server.go`. Run this task's tests, the matrix and task 07's
  meta-tests until green; then H20's own `/v1/refs` tests, unchanged.
- [ ] **Step 4: Commit by pathspec**, the eight files.

#### Acceptance

```bash
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(TwoWritersOnOneBase|RestoreWritesANewRevision|AStaleRestoreIsRefused|ServesMustStayInTheProject|RemovingTheLastServesNeedsAReason|RefsAnswerEachSlotInOrder|AnotherTeamsIssueKeyIsMissing|ATamperedRevisionIsMissing|AnErasedKeyReadsErased|AttachmentsAreThisProjectsBlobs|TheListCarriesNoContent|AReplayHoldsNoRevisionText|TheReplyIsTheStoredBytes)$' \
  2>&1 | grep -c '^--- PASS'                                                    # 13
go test ./control-plane/internal/hub/... -tags integration -race -count=1       # every package ok
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -run 'Field|Omit|Unknown|Digest'
```

Expected: `13`; then every package `ok` with no `FAIL` (H20's resolver tests included); then the
`hubapi` suite green with `RefResult.issue`, `RefsResponse.issueHeads` and `AttachRequest.digest`
in its field-name, never-omit and required-digest tables.

#### What this task does not claim

- **Not that a criterion id names a real criterion.** The Hub checks project, brief revision and
  grammar only (decision 4); membership is the writer's check, and a Hub-side one is a gap.
- **Not phase 4's parser.** `issue:` resolves on the Hub; `provenance.ParseKey` accepting it is
  task 18, and the control plane asking for it is task 19.
- **No editor, revision history UI or attachment upload UI** (tasks 47, 49-52).
- **Not erase.** This task answers `erased` for an erased revision; the erase route is task 14.
- **No measured resolver cost** for 100 mixed keys; one query per kind is the design, not a timing.

**Parity rows served:** **L-34** (autosave history and restore, as immutable digested revisions;
closed with the issue view's `Revisions`, task 47), **L-45** (attachments; closed with task 52),
**L-124** (the author class stored on every text; closed with task 12 and the issue view), and the
Hub half of **L-191** (what the agent was told is recoverable byte for byte; closed with task 19).
