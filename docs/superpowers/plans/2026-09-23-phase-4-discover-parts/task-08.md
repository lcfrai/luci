### Task 08: `discoverhub` — writes, the `4-D3` fan-out, archive and rebind

**The only write of brief text in the product** goes through `Service.Apply`, which turns one
domain op (task 03) into **one** Hub batch (`POST /v1/projects/{id}/brief-revisions`, all or
nothing — Hub contract §7.5). When a project edit produces a new complete project revision, the
same batch carries a new revision of every live inheriting branch brief, **complete or open**
(`4-D3`; review I8). What that buys, and what it does not:
- A branch whose head is **complete** gets a new complete revision pinning the new project
  revision in the same transaction, so no session starts between the two and inherits that
  branch's old pin.
- A branch whose head is **open** gets `reinherit` at its open head, so the text the person is
  editing carries the new project revision straight away. The branch's latest **complete**
  revision is immutable and still pins the old project revision, and P4-A says a session inherits
  it until the edit completes. That is the narrowing of `4-D3` named in C4 and asked as **OQ-8**.
  The chip reads `superseded` there, never `current` (task 16, `selectProvenanceState` rule 4).
- A teammate's branch op that computed its pin before the batch committed is re-pinned by the
  **two post-commit checks** in `Apply` steps 8 and 9. Anything the Hub's ordering still lets
  through is covered by the same chip rule.

**Depends on** task 07, on Hub ask **HA4-2** (`List`), and — for the digest a branch revision pins
before the project revision exists — on `hubapi.Canonical` (Hub contract §3.4) plus ask **HA4-9**
below.

**A new Hub ask this task adds, HA4-9.** A branch revision pins `{key, digest}` of the project
revision written **in the same batch**, so its digest must be known before the Hub computes it. The
envelope is fully determined (Hub contract §3.4: `v`, `kind`, `briefId`, `revision`, `projectId`,
`scope`, `branchRef`, `content`), and `hubapi.Canonical` is shared Go. Ask: export

```go
// in hubapi — the functions both sides use to build and digest a brief
// revision's canonical §3.4 envelope. The envelope is exported too, because
// the size check (review M6, step 7 below) measures those exact bytes.
func BriefRevisionEnvelope(briefID string, revision int, projectID, scope, branchRef string, content json.RawMessage) ([]byte, error)
func BriefRevisionDigest(briefID string, revision int, projectID, scope, branchRef string, content json.RawMessage) (string, error) // sha256 of BriefRevisionEnvelope
```

**Until it lands**, `discoverhub/digest.go` holds the local pair: `revisionEnvelope` builds the
§3.4 envelope with `hubapi.Canonical`, and `revisionDigest` hashes it. Both the pin and step 7's
size check call `revisionEnvelope`, so neither builds its own. `TestLocalDigestMatchesTheHubsVector`
pins the pair to the contract's §3.5 vector 1 byte for byte. `digest.go` goes only when the Hub
exports an envelope builder that returns the bytes at any size; then both callers move to it
(b10 re-review M-4, re-review b10-2 R-M3: the size check must not depend on a file the plan
deletes). If only the digest lands, the pin may move and `revisionEnvelope` stays.

**As built (task 08):** H0 already exports the digest-only shape as a method,
`hubapi.BriefEnvelope{…}.Canonical()` and `.Digest()` (hub `envelope.go:129-189`). `Canonical`
refuses an envelope over `hubapi.MaxEnvelopeBytes` and returns no bytes, so it cannot give the
size that `RevisionTooLargeError` and `FanoutTooLargeError` report. `digest.go` therefore stays,
and `TestLocalEnvelopeIsTheHubsEnvelope` holds `revisionEnvelope` to H0's builder byte for byte,
at the limit and for the escapes that differ between encoders, until the widened HA4-9 lands.
After the write, the Hub's returned digest for the project revision **must equal** the pinned one;
if it does not, `Apply` logs an `internal` error naming both and returns
`ErrDigestDisagreement` — every branch revision in that batch would name a digest nothing answers.

**Files:**
- `control-plane/internal/discoverhub/write.go` + `write_test.go` (new)
- `control-plane/internal/discoverhub/digest.go` + `digest_test.go` (new; deleted when HA4-9 lands)
- `control-plane/internal/discoverhub/lifecycle.go` + `lifecycle_test.go` — archive, rebind (new)
- `control-plane/internal/discoverhub/fakehub_test.go` (task 07's phase-4-owned wrapper) gains two
  race hooks for review I8's tests, beside task 07's `renameProject` and `failResolve`:
  - `afterList(n int, fn func())` — runs `fn` after the n-th `List` call through the wrapper
    returns (1-based, counted from the hook), before the caller sees the result;
  - `beforeWrite(n int, fn func())` — runs `fn` before the n-th `WriteRevisions` call reaches the
    embedded fake, so a test can commit a teammate's batch between a writer's read and its write.
  `fn` writes through the embedded fake directly, never through the wrapper, so it does not count
  towards `n`.

**Interfaces — complete source:**

```go
// control-plane/internal/discoverhub/write.go
package discoverhub

// OpProposeFollowUp is the one op the server expands itself: it asks the
// model (task 04) and, if a question comes back, applies
// discover.OpAddFollowUp. It is never a discover.OpKind, because the domain
// never calls a model.
const OpProposeFollowUp discover.OpKind = "propose_follow_up"

// Request is one change a person asked for, at the head they saw.
type Request struct {
	Target       Target
	BaseRevision int         // the head the person saw; 0 when no brief exists yet
	Op           discover.Op // Kind, Key, Text, Source, RestoreRevision from the wire
}

// Result is the view after the change, and what happened.
type Result struct {
	View    View
	Written []string               // the keys written, this brief's first; [] when nothing changed
	Assist  discover.AssistOutcome // set for OpProposeFollowUp; "" otherwise
}

var (
	ErrDigestDisagreement = errors.New("discoverhub: the Hub's digest is not the one the branch revisions pinned")
)

// The Hub's size limits (review M6, plan §4 Security, S4). HA4-3 has landed
// in H0 (hubapi/types.go:299-307, envelope.go:31), so they are the Hub's own
// constants, not local copies (as built, task 08):
//   - MaxEnvelopeBytes: a revision's canonical §3.4 envelope (64 KiB);
//   - MaxBatchBytes: the body of POST …/brief-revisions (4 MiB);
//   - MaxBatchItems: the items one batch carries (64).
const (
	MaxEnvelopeBytes = hubapi.MaxEnvelopeBytes
	MaxBatchBytes    = hubapi.MaxRevisionBatchBytes
	MaxBatchItems    = hubapi.MaxRevisionBatch
)

// FanoutTooLargeError is a project edit whose batch one Hub call cannot
// carry, for one of three causes: more branch revisions than the batch limit
// (HA4-3); a request body over MaxBatchBytes; or one fan-out item, a branch
// brief's reinherit, whose own envelope is over MaxEnvelopeBytes (b10
// re-review M-2). The third is the branch's text, stored unchecked, not the
// project editor's, so it is never a RevisionTooLargeError that tells the
// editor to shorten their own answer. Nothing is written.
type FanoutTooLargeError struct {
	Needed     int    // branch briefs that would need a new revision
	Limit      int    // the Hub's batch limit minus one
	Bytes      int    // the size that failed: the body's, or BranchRef's envelope; 0 when the count failed
	LimitBytes int    // MaxBatchBytes, or MaxEnvelopeBytes when BranchRef is set; 0 when the count failed
	BranchRef  string // the fan-out item's branchRef when its envelope failed; "" otherwise
}

func (e *FanoutTooLargeError) Error() string

// RevisionTooLargeError is a revision whose canonical §3.4 envelope would be
// over MaxEnvelopeBytes: the refusal the Hub would give as a 400 that names
// no answer (review M6), given here first with the op's question key. Task
// 03's caps make it unreachable for ordinary text; this check is the bound,
// because the caps count code points, not bytes: 4,000 CJK code points are
// 12,000 canonical bytes (Hub contract §3.4 writes non-ASCII as UTF-8), so
// six answers plus the done answer's criteria (28,000 code points) reach
// 84,000 bytes, past 65,536. It is only ever this brief's own item: a
// fan-out item over the limit is a *FanoutTooLargeError with BranchRef.
// Nothing is written.
type RevisionTooLargeError struct {
	Key   string // Request.Op.Key; "" for an op with none (restore, draft, propose_follow_up): the brief, not one answer
	Bytes int
	Limit int // MaxEnvelopeBytes
}

func (e *RevisionTooLargeError) Error() string

// Apply writes one change as one batch.
//
//  1. Hub off → ErrHubOff; project not bound → ErrNotBound.
//  2. No brief at the target and BaseRevision 0 → CreateBrief (brief_exists
//     from a race re-reads and continues with that brief).
//  3. The head content is read (from cache or Brief); the op is applied with
//     discover.Apply and Env.Inherits = the project's latest complete
//     revision now (branch scope only). ErrNoChange → Result with
//     Written = [] and no Hub write.
//  4. OpProposeFollowUp: ProposeFollowUp; anything but AssistProposed →
//     Result with that outcome, no write.
//  5. OpRestore: the target revision's content is read by key; an erased
//     target is ErrErasedRestore, and a missing one ErrNoBrief.
//  6. Project scope, and the new content is complete: every live (not
//     archived) branch brief of the project whose head content pins a
//     different project key gets discover.OpReinherit at its head, whether
//     that head is complete OR OPEN (review I8). An open head keeps its
//     status; only Inherits moves, so the person's edit in progress now
//     carries the new project revision. Their next op meets
//     revision_conflict and re-reads, exactly as on a complete head.
//  7. One writeBatch call. Before its one WriteRevisions, writeBatch checks
//     the sizes (review M6, S4): each item's canonical §3.4 envelope, built
//     by the builder the pin uses (digest.go's revisionEnvelope, then
//     hubapi.BriefRevisionEnvelope once HA4-9 lands), over MaxEnvelopeBytes →
//     *RevisionTooLargeError{Key: r.Op.Key} for this brief's item, and
//     *FanoutTooLargeError{BranchRef, Bytes, LimitBytes: MaxEnvelopeBytes}
//     for a fan-out item (b10 re-review M-2); then the request body, measured as
//     len(json.Marshal({"revisions": writes})), over MaxBatchBytes →
//     *FanoutTooLargeError{Needed, Limit, Bytes, LimitBytes}. json.Marshal
//     escapes <, > and & where the canonical form does not, so it is never
//     shorter than the canonical bytes; whether hubclient's encoder can be
//     longer is unverified (Hub plan text). A single item cannot pass 1 MiB:
//     64 KiB canonical is at most 6x that in json.Marshal (one canonical
//     byte "<" is six bytes there: a backslash, then u003c).
//     Steps 8 and 9 go through the same checks, and a failure there is
//     logged like any other failure in those steps.
//     A revision_conflict naming THIS brief is
//     returned unchanged (the person re-reads). One naming a fan-out item
//     re-reads heads and rebuilds the batch, at most 3 times, then returns
//     the conflict.
//  8. Project-side post-commit check (steps 6-7 ran): List again, AFTER the
//     batch committed. Any live branch head whose content pins a project
//     revision below the one just written - a brief created, or a branch
//     op committed, between step 6's List and the commit - gets one more
//     fan-out, built by the same fanOut and written by writeBatch with
//     step 7's retry rule. It runs once and never re-lists again.
//  9. Branch-side post-commit check (branch scope, a revision was
//     written): read the project's latest complete revision AFTER the
//     write committed. If it is above the pin just written - a project
//     batch committed between step 3 and this write - write one
//     discover.OpReinherit at the new head through writeBatch. A
//     revision_conflict here is dropped: whoever moved the head pinned
//     the project revision current at their own step 3.
//     In steps 8 and 9 a NULL pin (Content.Inherits nil: written before
//     the project brief's first completion, or racing it) counts as below
//     every revision, so it is re-pinned like any stale one (re-review
//     b06-1 R-M1). Step 6 already catches it as "a different project key".
//     Result.View is read AFTER step 9 (and after step 8), so it shows the
//     head step 9 moved, and the person's next op carries that head as its
//     BaseRevision instead of meeting a revision_conflict our own re-pin
//     caused (re-review b06-1 R-M3). A failed step 8 or 9 leaves
//     Result.View at the step-7 head.
//     Why both: a branch write that commits AFTER the project batch is
//     seen by step 9; one that commits BEFORE it was either in step 6's
//     batch (so step 7 retried) or created after step 6's List (so step
//     8 sees it). Each check runs once, so the pair cannot loop; a pin
//     left stale by a third concurrent writer reads superseded on the
//     chip (task 16), never current.
//     A failure in step 8 or 9 is logged at warn and does not fail Apply:
//     the person's change is written. Result.Written lists only the
//     step-7 batch.
func (s *Service) Apply(ctx context.Context, r Request) (Result, error)

var ErrErasedRestore = errors.New("discoverhub: that revision was erased and cannot be restored")
```

```go
// control-plane/internal/discoverhub/lifecycle.go
package discoverhub

// ArchiveHead archives the branch's live brief when the branch is deleted
// (contract P13; phase 3's briefref.Archiver, ask P4-3). Revisions are
// untouched and every key still resolves. No brief, Hub off, or project not
// bound: nil - there is nothing to archive.
func (s *Service) ArchiveHead(ctx context.Context, projectID, branchID string) error

// Rebind moves the branch's live brief to the renamed ref (Hub PATCH
// /v1/briefs/{id}); seam S4-P3.6, called by phase 3's RenameBranch. It takes
// NAMES, because after a rename the old branch id may no longer resolve.
// brief_exists on the new ref is returned unchanged: two briefs, one ref,
// is the person's to settle, not ours to merge.
func (s *Service) Rebind(ctx context.Context, projectID, oldBranchName, newBranchName string) error
```

**Tests first** (the fake from task 07's fixtures):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFirstAnswerCreatesTheBriefAndWritesRevisionOne` | no brief → `CreateBrief` then one batch; `Written == ["brief:bf-…@1"]` | a brief created with no revision, or two batches |
| `TestFiveAnswersAreFiveRevisions` | five records → heads 1..5; the fifth is `complete`; history shows five `record`/`skip` changes | a service that batches answers and breaks R9's `rev 5` |
| `TestNoChangeWritesNothing` | the same answer twice → second `Written == []`, head unchanged | a revision per click |
| `TestAProjectEditFansOutInOneBatch` | `b-7` at 5 complete, `b-42` and `b-51` complete pinning `b-7@5`; edit project audience → one `WriteRevisions` call with 3 items; `b-42@7` and `b-51@6` pin `b-7@6` and its digest | `4-D3` written as separate calls a session can start between |
| `TestFanOutReinheritsOpenHeadsAndSkipsArchived` | `b-42` complete, `b-51` **open** at rev 7 (rev 6 complete) and `b-60` archived, all pinning `b-7@5`; edit project audience → one batch with 3 items: `b-7@6`, `b-42@7` complete (it was complete at 6) and `b-51@8` **open**, both pinning `b-7@6`; `b-51@8`'s change is `reinherit` and its answers equal rev 7's; `b-60` untouched | a fan-out that skips open heads, so the edit in progress keeps the old project text (review I8); or one that re-revisions an archived head nobody can inherit |
| `TestFanOutLeavesAnOpenHeadsLatestCompletePinned` | after the batch above, `b-51`'s `latestCompleteRevision` is still 6 and `brief:…b51@6` still pins `b-7@5` | a fan-out that rewrites an immutable revision, or a test that hides the `4-D3` narrowing (OQ-8) |
| `TestARacingBranchWriteCommittedAfterTheListIsRepinned` | the fake runs a hook after the first `List` and before the batch commits: it creates `b-70` with rev 1 complete pinning `b-7@5` → after `Apply`, `List` was called twice and `b-70@2` pins `b-7@6` with change `reinherit` | the step-8 re-list missing, so a brief created mid-batch keeps the old pin |
| `TestABranchWriteThatLosesTheRaceRepinsItself` (re-review b06-1 R-M2, R-M3) | a first answer on a new branch creates `b-80` and reads `b-7@5` at step 3; the fake's hook then commits `b-7@6` before `b-80@1` commits. The case step 9 really covers is a brief that the project's step-6 `List` and step-8 re-list both missed, whose first revision commits after them; the hook's bare `b-7@6` stands in for that project batch (a real batch that carried an existing brief would make that brief's own write meet `revision_conflict` instead) → after `Apply`, `b-80`'s head is 2, change `reinherit`, pinning `b-7@6`; `Written` names only `b-80@1`; `Result.View`'s head revision is 2 | the step-9 check missing, so a teammate's branch op commits a pin the project batch already superseded; or a `Result.View` read before step 9, whose stale head makes the person's next op meet a conflict our own re-pin caused |
| `TestANullPinIsRepinned` (re-review b06-1 R-M1) | `b-90`'s rev 1 is complete with `inherits` nil (written before `b-7` ever completed); `b-7`'s first complete revision → the batch carries `b-90@2` `reinherit` pinning it (step 6). And a first answer on a new branch that reads no complete project revision at step 3, while the hook commits `b-7@1` complete before it → its head is 2, `reinherit`, pinning `b-7@1` (step 9) | a null pin that steps 8-9 never see as "below", so a branch inherits no project text after the project first completes, and no chip can flag it (OQ-8) |
| `TestProjectPostCommitCheckRunsOnce` (re-review b06-1 R-M2) | project scope: the fake creates a brief pinning `b-7@5` after step 6's `List`, and another during step 8's write → `List` is called exactly twice, the first is re-pinned, and the second is left as it is | a re-list loop |
| `TestBranchPostCommitCheckRunsOnce` (re-review b06-1 R-M2) | branch scope: the fake commits `b-7@6` before the branch write and `b-7@7` during step 9's write, each bare project revision standing in for a project batch, as in the row above (re-review b06-2 N2) → step 9 reads the project once and writes exactly one extra revision, pinning `b-7@6` | a re-pin loop between two writers |
| `TestAnIncompleteProjectEditDoesNotFanOut` | adding a follow-up to the project (status open) → a one-item batch | branch revisions pinning a project revision no session may inherit |
| `TestBranchOpsPinTheCurrentProjectRevision` | an answer on `b-42` after `b-7` moved to 6 writes `inherits.key == brief:…b7@6` | a branch revision pinning whatever the last one pinned |
| `TestPinnedDigestEqualsTheHubsDigest` | after the fan-out, `inherits.digest` equals the digest the fake returned for `b-7@6` | a pin to a digest nothing answers |
| `TestLocalDigestMatchesTheHubsVector` (`digest_test.go`) | the local envelope for §3.5 vector 1's inputs digests to `sha256:a72a06a1…e9fa` | a hand-rolled envelope with a field order or escape the Hub does not use |
| `TestLocalEnvelopeIsTheHubsEnvelope` (`digest_test.go`, as built) | `revisionEnvelope` equals H0's `hubapi.BriefEnvelope.Canonical()` byte for byte for `<`, `&`, U+2028, a control character, CJK and both scopes, and at exactly 64 KiB; one byte over, H0's builder refuses and the local one returns all 65,537 bytes | a local builder that drifts from the Hub's, or one that cannot measure an oversized envelope |
| `TestConflictOnThePersonsBriefIsReturned` | the fake reports this brief's head moved → `*hubclient.Error` 409 `revision_conflict`, nothing written | a silent merge the person never saw |
| `TestConflictOnAFanOutItemIsRetried` | the fake moves `b-51` once during the first attempt → second attempt succeeds; `b-51`'s new revision is based on its new head | a project edit refused because a teammate touched another branch |
| `TestFanOutTooLargeWritesNothing` | 64 live branch briefs (48 complete, 16 open) with the 64-item limit (HA4-3, landed) → `*FanoutTooLargeError{Needed: 64, Limit: 63}`, no write | a batch split into two transactions |
| `TestAnOversizedBatchWritesNothing` (review M6) | 18 live branch briefs, each head's answers holding 40,000 `<` characters in all, written straight to the fake (heads the Hub stored unchecked; a `reinherit` leaves their strings as they are, so task 03's caps do not see them). Each item's canonical envelope stays under 64 KiB, because `<` is one canonical byte, but `json.Marshal` writes it as six (a backslash, then `u003c`), about 240,000 bytes per item, 4.3 MB in all. Edit project audience → `*FanoutTooLargeError` with `Bytes > MaxBatchBytes` (4 MiB), `LimitBytes == MaxBatchBytes`, `Needed == 18`, and no `WriteRevisions` call; the same project with 2 such branches (about 480,000 bytes) writes | a check of the item count only, so the Hub answers the whole batch `invalid_json` 400 (`hub-contract.md:234`) and the person reads a sentence about JSON; a size measured from the canonical bytes, which is shorter than the body sent |
| `TestAnOversizedRevisionNamesTheAnswer` (review M6) | a `record` of `purpose` whose content, with the other answers already at the head, gives a canonical envelope of 64 KiB + 1 (CJK text, at most 4,000 code points per answer so task 03's cap passes, padded with ASCII to the exact size) → `*RevisionTooLargeError{Key: "purpose", Bytes: 65537, Limit: 65536}`, no write; at exactly 65,536 it writes | the Hub's own 400, which names no answer (`hub-contract.md:742`); an off-by-one at the limit |
| `TestAnOversizedFanOutItemNamesTheBranch` (b10 re-review M-2) | branch brief `b-51`, its head written straight to the fake (stored unchecked) with `inherits: null` and a canonical envelope 50 bytes under 64 KiB (as built: the pin adds about 112 bytes, not more than 150, so "150 under" would not overflow); the project's first complete revision (a `record` that completes `b-7`, at `b-7@5`) makes `b-51`'s reinherit pin it and its digest, which adds more than 50 bytes → `*FanoutTooLargeError{Needed: 1, BranchRef: b-51's branchRef, Bytes > 65536, LimitBytes: 65536}`; `errors.As` into `*RevisionTooLargeError` is false; no `WriteRevisions` call | the branch's overflow reported as a `RevisionTooLargeError` with `Key ""`, which tells the project editor to shorten their own answers, which are not the cause |
| `TestProposeFollowUpWithNoModelWritesNothing` | nil model → `Assist == model_off`, `Written == []` | a revision written for a follow-up that never came |
| `TestRestoreWritesANewRevision` | restore rev 5 at head 6 → head 7 whose answers equal rev 5's and whose change is `restore` with `restores: 5` | an undo that deletes a revision (P3) |
| `TestRestoreOfAnErasedRevisionIsRefused` | `ErrErasedRestore` | erased text brought back by undo |
| `TestArchiveHeadLeavesKeysResolvable` | after `ArchiveHead`, `Resolve("brief:…a2@5")` is still `resolved` | P13 broken by an archive that deletes |
| `TestRebindMovesTheBrief` | `evidence-upload` → `evidence-intake`: `ForBranch(refs/heads/evidence-intake)` returns `b-42`; the old ref returns none | a renamed branch showing *no brief* |

**Steps:**

- [ ] **Step 1:** Write the tests; confirm they fail.
- [ ] **Step 2:** `digest.go` (until HA4-9), then `write.go`: `Apply` as specified, the fan-out
  builder a pure function `fanOut(project Revision, heads []branchHead) ([]RevisionWrite, error)`
  tested on its own (it takes complete and open heads alike and drops archived ones), and one
  `writeBatch` that holds the only `WriteRevisions` call and is used by steps 7, 8 and 9 of
  `Apply`, and runs the two size checks before it (review M6). **Deviation from the review:** it
  said `fanOut` should check the batch size. The check sits in `writeBatch` instead, because
  `fanOut` does not build step 7's branch-scope batch or step 9's, and one check there bounds
  every batch this package sends. `fanOut` stays pure.
- [ ] **Step 3:** `lifecycle.go`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discoverhub/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub
$ grep -n "WriteRevisions(" control-plane/internal/discoverhub/*.go | grep -v _test
(exactly one call site, in write.go's writeBatch — the one writer of brief text)
```

**As built (task 08), where the code departs from the text above:**
1. HA4-3 had landed in H0: the limits are `hubapi.MaxEnvelopeBytes` (64 KiB),
   `hubapi.MaxRevisionBatchBytes` (4 MiB) and `hubapi.MaxRevisionBatch` (64), exported here as
   `MaxEnvelopeBytes`, `MaxBatchBytes` and `MaxBatchItems`. `Limit` is 63, and the count and body
   rows are scaled to them.
2. `fanOut(pin discover.Inherit, heads []branchHead, now time.Time, newID func(string) string) ([]item, error)`:
   it takes the pin (key and digest), not a `Revision`, because that is all a reinherit needs, and
   `discover.Apply` needs `Now` and `NewID`. It returns `item`s (a `RevisionWrite` plus the scope
   and ref the Hub stamps), because the size check builds each item's envelope from them. It is
   still pure. It drops archived heads, heads with no revision yet (nothing to pin: their first op
   pins the project revision current then), and heads already pinning `pin` (`ErrNoChange`).
3. Step 2 runs after step 3: `CreateBrief` is called only when a revision will be written, so a
   no-change op or a `propose_follow_up` that proposes nothing creates no empty brief.
4. A rebuild after a fan-out conflict re-runs the same build, which re-lists. In step 8 that is the
   retry's own re-read, bounded at 3; the check itself runs once.
5. Steps 8 and 9 count a pin to another project brief as below, like a null pin.
6. A `View` read that fails after a successful write returns its error with `Result.Written` set:
   the change is written, and the same op again is `ErrNoChange`.
7. `ArchiveHead` of a branch id the Locator does not know is `errUnknownBranch`, not nil, so a
   brief is never left live unseen; the caller archives before the branch leaves the Locator.
   `Rebind` with an empty name is `errUnknownBranch`; the same name twice is nil.
8. `fakehub_test.go` also gains `batches()` (every batch through the wrapper) and
   `eraseInResolve(keys…)`: the fake erases whole briefs and archives them (§7.12), so it never
   holds an erased revision on a live brief, and `TestRestoreOfAnErasedRevisionIsRefused` needs one.
   `beforeWrite` may be set more than once; each hook counts from its own call.

**Does NOT claim:** behaviour under a real Hub's concurrency (the fake's promise, Hub §9.2, and
H3's integration test); a batch over 16 items (HA4-3); that the post-commit checks close every
interleaving of three or more concurrent writers; that `hubclient`'s request body is never longer
than `json.Marshal`'s for the same batch (Hub plan text, not built). They close the two-writer race, and the
`superseded` chip rule (task 16) keeps `current` true for whatever is left. That a Hub `List` or
`Brief` read issued after a commit returns sees that commit (read-after-write) is the Hub
contract's promise and is not verified here.
