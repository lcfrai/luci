### Task 08: `discoverhub` — writes, the `4-D3` fan-out, archive and rebind

**The only write of brief text in the product** goes through `Service.Apply`, which turns one
domain op (task 03) into **one** Hub batch (`POST /v1/projects/{id}/brief-revisions`, all or
nothing — Hub contract §7.5). When a project edit produces a new complete project revision, the
same batch carries a new revision of every complete inheriting branch brief (`4-D3`), so no
session can start between the two and inherit a branch key whose pinned project text is stale.

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
//  6. Project scope, and the new content is complete: every live branch
//     brief of the project whose HEAD is complete and whose head content
//     pins a different project key gets discover.OpReinherit at its head.
//  7. One WriteRevisions call. A revision_conflict naming THIS brief is
//     returned unchanged (the person re-reads). One naming a fan-out item
//     re-reads heads and rebuilds the batch, at most 3 times, then returns
//     the conflict.
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
| `TestFanOutSkipsOpenAndArchivedBranchBriefs` | an open branch head and an archived one are not in the batch | re-revisioning a head that cannot be inherited |
| `TestAnIncompleteProjectEditDoesNotFanOut` | adding a follow-up to the project (status open) → a one-item batch | branch revisions pinning a project revision no session may inherit |
| `TestBranchOpsPinTheCurrentProjectRevision` | an answer on `b-42` after `b-7` moved to 6 writes `inherits.key == brief:…b7@6` | a branch revision pinning whatever the last one pinned |
| `TestPinnedDigestEqualsTheHubsDigest` | after the fan-out, `inherits.digest` equals the digest the fake returned for `b-7@6` | a pin to a digest nothing answers |
| `TestLocalDigestMatchesTheHubsVector` (`digest_test.go`) | the local envelope for §3.5 vector 1's inputs digests to `sha256:a72a06a1…e9fa` | a hand-rolled envelope with a field order or escape the Hub does not use |
| `TestConflictOnThePersonsBriefIsReturned` | the fake reports this brief's head moved → `*hubclient.Error` 409 `revision_conflict`, nothing written | a silent merge the person never saw |
| `TestConflictOnAFanOutItemIsRetried` | the fake moves `b-51` once during the first attempt → second attempt succeeds; `b-51`'s new revision is based on its new head | a project edit refused because a teammate touched another branch |
| `TestFanOutTooLargeWritesNothing` | 16 complete branch briefs with the 16-item limit → `*FanoutTooLargeError{Needed: 16, Limit: 15}`, no write | a batch split into two transactions |
| `TestProposeFollowUpWithNoModelWritesNothing` | nil model → `Assist == model_off`, `Written == []` | a revision written for a follow-up that never came |
| `TestRestoreWritesANewRevision` | restore rev 5 at head 6 → head 7 whose answers equal rev 5's and whose change is `restore` with `restores: 5` | an undo that deletes a revision (P3) |
| `TestRestoreOfAnErasedRevisionIsRefused` | `ErrErasedRestore` | erased text brought back by undo |
| `TestArchiveHeadLeavesKeysResolvable` | after `ArchiveHead`, `Resolve("brief:…a2@5")` is still `resolved` | P13 broken by an archive that deletes |
| `TestRebindMovesTheBrief` | `evidence-upload` → `evidence-intake`: `ForBranch(refs/heads/evidence-intake)` returns `b-42`; the old ref returns none | a renamed branch showing *no brief* |

**Steps:**

- [ ] **Step 1:** Write the tests; confirm they fail.
- [ ] **Step 2:** `digest.go` (until HA4-9), then `write.go`: `Apply` as specified, the fan-out
  builder a pure function `fanOut(project Revision, heads []branchHead) ([]RevisionWrite, error)`
  tested on its own.
- [ ] **Step 3:** `lifecycle.go`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discoverhub/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub
$ grep -n "WriteRevisions(" control-plane/internal/discoverhub/*.go | grep -v _test
(exactly one call site, in write.go — the one writer of brief text)
```

**Does NOT claim:** behaviour under a real Hub's concurrency (the fake's promise, Hub §9.2, and
H3's integration test); a batch over 16 items (HA4-3).
