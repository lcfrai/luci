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
// in hubapi — the one function both sides use to digest a brief revision
func BriefRevisionDigest(briefID string, revision int, projectID, scope, branchRef string, content json.RawMessage) (string, error)
```

**Until it lands**, `discoverhub/digest.go` builds the §3.4 envelope and calls `hubapi.Canonical`,
and `TestLocalDigestMatchesTheHubsVector` pins it to the contract's §3.5 vector 1 byte for byte.
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

// FanoutTooLargeError is a project edit that would need more branch
// revisions than one Hub batch holds (HA4-3). Nothing is written.
type FanoutTooLargeError struct {
	Needed int // branch briefs that would need a new revision
	Limit  int // the Hub's batch limit minus one
}

func (e *FanoutTooLargeError) Error() string

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
//  7. One writeBatch call. A revision_conflict naming THIS brief is
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
| `TestABranchWriteThatLosesTheRaceRepinsItself` | a branch op on `b-42` reads `b-7@5` at step 3; the fake commits the project's `b-7@6` batch between step 3 and the branch write → after `Apply`, `b-42`'s head is one above the person's revision, change `reinherit`, pinning `b-7@6`; `Written` names only the person's revision | the step-9 check missing, so a teammate's branch op commits a pin the project batch already superseded |
| `TestPostCommitChecksRunOnce` | the fake moves the project again during step 8's write → `List` is called exactly twice and step 9 writes at most one extra revision | a re-pin loop between two writers |
| `TestAnIncompleteProjectEditDoesNotFanOut` | adding a follow-up to the project (status open) → a one-item batch | branch revisions pinning a project revision no session may inherit |
| `TestBranchOpsPinTheCurrentProjectRevision` | an answer on `b-42` after `b-7` moved to 6 writes `inherits.key == brief:…b7@6` | a branch revision pinning whatever the last one pinned |
| `TestPinnedDigestEqualsTheHubsDigest` | after the fan-out, `inherits.digest` equals the digest the fake returned for `b-7@6` | a pin to a digest nothing answers |
| `TestLocalDigestMatchesTheHubsVector` (`digest_test.go`) | the local envelope for §3.5 vector 1's inputs digests to `sha256:a72a06a1…e9fa` | a hand-rolled envelope with a field order or escape the Hub does not use |
| `TestConflictOnThePersonsBriefIsReturned` | the fake reports this brief's head moved → `*hubclient.Error` 409 `revision_conflict`, nothing written | a silent merge the person never saw |
| `TestConflictOnAFanOutItemIsRetried` | the fake moves `b-51` once during the first attempt → second attempt succeeds; `b-51`'s new revision is based on its new head | a project edit refused because a teammate touched another branch |
| `TestFanOutTooLargeWritesNothing` | 16 live branch briefs (12 complete, 4 open) with the 16-item limit → `*FanoutTooLargeError{Needed: 16, Limit: 15}`, no write | a batch split into two transactions |
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
  `Apply`.
- [ ] **Step 3:** `lifecycle.go`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discoverhub/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub
$ grep -n "WriteRevisions(" control-plane/internal/discoverhub/*.go | grep -v _test
(exactly one call site, in write.go's writeBatch — the one writer of brief text)
```

**Does NOT claim:** behaviour under a real Hub's concurrency (the fake's promise, Hub §9.2, and
H3's integration test); a batch over 16 items (HA4-3); that the post-commit checks close every
interleaving of three or more concurrent writers. They close the two-writer race, and the
`superseded` chip rule (task 16) keeps `current` true for whatever is left. That a Hub `List` or
`Brief` read issued after a commit returns sees that commit (read-after-write) is the Hub
contract's promise and is not verified here.
