### Task 10: Landed history, exact undo, and revert

`AGENTIC-UX.md` §3's LANDED is "bulk-acceptable, low attention", and `DESIGN-SYSTEM.md` wants an
Undo window on optimistic actions. A land has **two** undos and the UI must not pretend they are
one (survey §7.4): while the land is the branch's tip and has not been pushed, moving the ref back
is exact; once something is built on it or it is pushed, the only honest undo is a revert commit,
landed like any other — and never a force-push (A3). This task serves the history the LANDED list
draws, with each row's undo state computed from git, and performs both undos.

**Files:**
- `control-plane/internal/converge/history.go` + test — `HistoryEntry`, `History` (new)
- `control-plane/internal/converge/undo.go` + test — `Undo`, `Revert`, their results (new)
- `control-plane/internal/api/lands.go` — GET history, POST undo, POST revert (task 09's file)
- `control-plane/internal/api/gitroutes.go`, `routes_audit_test.go` — `// ── task 10 ──`, one audit line

#### Interfaces — complete

```go
package converge

// HistoryEntry is one first-parent commit of a product branch, newest first.
type HistoryEntry struct {
	SHA        string   `json:"sha"`
	Parents    []string `json:"parents"`
	AuthorName string   `json:"authorName"`
	At         string   `json:"at"` // author date, RFC 3339
	Subject    string   `json:"subject"`
	// Kind: "land" (two parents and a Panacea-Land trailer), "revert" (a Panacea-Reverts
	// trailer), "bring-in" (two parents and a Panacea-Bring-In trailer, task 09's merge-mode
	// BringIn; re-review b05 issue 5), or "direct" — anything else, including a merge made in a
	// terminal. The Merge lens says "committed on <branch> directly, not a land" for "direct" and
	// "the cockpit brought in the remote's commits" for "bring-in" (its Subject names the
	// remote branch; the trailer holds the upstream tip, gitx.TrailerBringIn). A bring-in's Undo is
	// "none": undo and revert key on Panacea-Land and refuse it as not_a_land. A fast-forward
	// bring-in writes no commit, so its commits list as they were made (usually "direct").
	Kind         string   `json:"kind"`
	WorktreeID   string   `json:"worktreeId"`   // land: its Panacea-Land value
	WorktreeName string   `json:"worktreeName"` // land: from the lane index while the lane exists, else ""
	BriefKey     string   `json:"briefKey"`     // land: Panacea-Brief, verbatim; "" renders `none`
	BriefDigest  string   `json:"briefDigest"`
	// Sessions: for a land, the Panacea-Session values on the lane's commits (parent 1..parent 2,
	// at most 20 commits read); for a direct commit, its own. [] renders "no session recorded".
	Sessions []string `json:"sessions"`
	Reverts  string   `json:"reverts"` // revert: the land it takes back
	Pushed   bool     `json:"pushed"`  // reachable from the upstream; false when never pushed
	// Undo: "exact" (this is the tip, a land, not pushed, and no other lane contains it),
	// "revert" (a land that is pushed, built upon, or merged into another lane), or "none" (not a
	// land). UndoWhy says why an exact undo is not offered:
	// "pushed" | "built_upon" | "merged_into_lane" | "".
	// merged_into_lane (review I3): a refs/panacea/lanes/<id> other than the land's own contains
	// the land, typically
	// because that lane's agent merged origin/<branch> to resolve a conflict (task-19
	// resolvePrompt). Moving the branch back would not take the land back: that lane's next land
	// brings it, and its worktree's work, back without a word, and the forecast calls it clean.
	Undo    string `json:"undo"`
	UndoWhy string `json:"undoWhy"`
}

// History reads at most limit (1-200, default 50) first-parent entries of refs/heads/<branch>:
//   log --first-parent -n <limit> --format=%H%x00%P%x00%an%x00%aI%x00%s%x00<Land>%x00<Brief>%x00<BriefDigest>%x00<Session>%x00<Reverts>%x00<BringIn>%x1e
//   rev-list --first-parent refs/heads/<branch> ^<upstream>   (the unpushed set; skipped when never pushed)
//   log --format=<Session>%x1e -n 20 <sha>^1..<sha>^2         (per land row)
//   for-each-ref --contains <tip> --format=%(refname) refs/panacea/lanes/
//                                                             (only when the tip row is an unpushed land)
// all gitx.Read. The first three are cached by (tip, upstream tip, limit). The for-each-ref is
// not: a lane sync moves a lane ref without moving the tip, so a cached answer would go stale.
// An answer naming any lane but the land's own makes the tip row undo "revert", undoWhy
// "merged_into_lane" (review I3), by the same rule Undo refuses with.
func History(ctx context.Context, m gitx.Machine, projectPath, branch string, store lanes.Store, limit int) ([]HistoryEntry, error)

// UndoResult is an exact undo.
type UndoResult struct {
	Tip           string            `json:"tip"`    // the land's first parent, now the tip
	Undone        string            `json:"undone"` // kept at refs/panacea/undone/<sha>
	ReadyRestored bool              `json:"readyRestored"`
	Refreshed     []string          `json:"refreshed"`
	RefreshErrors map[string]string `json:"refreshErrors"`
}

// Undo takes back the land at the tip, exactly. Refusals (*Refusal, nothing written):
// branch_checked_out; undo_not_exact with details.why "not_a_land" | "not_the_tip" | "pushed" |
// "merged_into_lane"; branch_moved when the tip is not expectedTip. merged_into_lane (review I3)
// comes from, under the lock and after the tip check:
//   for-each-ref --contains <sha> --format=%(refname) refs/panacea/lanes/   (project, Read)
// Any ref other than the land's own lane (refs/panacea/lanes/<Panacea-Land id>) refuses, with
// details.worktrees = the lane ids, sorted, one per line (an id is a ref-name component, so git
// refuses a newline in it; task-04's line format), and the desktop offers the revert. The land's
// own lane is exempt: its next land brings back only its own work, which is what landing it again
// means, and the forecast shows that work as its own. Then, in order:
//   update-ref refs/panacea/undone/<sha> <sha>                           (keeps it and its trailer resolvable)
//   update-ref -m "panacea: undo land <id>" refs/heads/<branch> <sha>^1 <sha>   (compare-and-swap)
//   SetWorktreeReady(<Panacea-Land id>, true) when that lane still exists  (exact means as before)
//   l.RefreshClones(ctx, projectPath, branch), as Land's step 14 (origin/<branch> and, since
//   review I5, upstream/<branch>; task 09)
// It holds Locks.Lock(ctx, projectID) for the whole verb, as Land does, so it serialises with
// every land, fetch, push and branch verb on the project (task-02 Locks; review I7). Hardened per task-02 §HGH: the project calls use
// Scope Project, the refresh uses Scope Clone, and no hook runs (H4).
func (l *Lander) Undo(ctx context.Context, projectPath, projectID, branch, sha, expectedTip string) (UndoResult, error)

// RevertResult is a revert commit that landed.
type RevertResult struct {
	Revert        string            `json:"revert"`
	Before        string            `json:"before"`
	Reverts       string            `json:"reverts"`
	Refreshed     []string          `json:"refreshed"`     // as UndoResult (review I4)
	RefreshErrors map[string]string `json:"refreshErrors"`
}

// Revert lands a commit that takes back one land. Refusals: branch_checked_out; branch_moved;
// undo_not_exact/not_a_land for a sha that is not a land on this branch's first-parent line;
// no_git_identity; merge_conflict (details.paths = detailList, task 04; review I2). Then:
//   tree = MergeTreeArgs + --merge-base=<sha> <tip> <sha>^1, OKExit 1 → ParseMergeTree
//          (merge-tree --write-tree --name-only -z; task 07; git ≥ 2.40)
//   msg  = Message("Revert land <name> into <branch>", "This takes back <sha>.", [Panacea-Reverts: <sha>])
//   rev  = commit-tree <tree> -p <tip> -F -                              (the user's identity)
//   update-ref -m "panacea: revert <sha>" refs/heads/<branch> <rev> <tip>
//   l.RefreshClones(ctx, projectPath, branch), as Undo does (review I4): a revert moves the
//   branch, and an agent told to merge origin/<branch> must merge the branch as it now is
// The reverted lane's ready flag is not touched. It holds Locks.Lock(ctx, projectID) for the
// whole verb, as Undo does (review I7). Hardened per task-02 §HGH: the project calls use Scope
// Project, the refresh uses Scope Clone, and no hook or merge driver the repository names runs
// (H1, H3, H4).
func (l *Lander) Revert(ctx context.Context, projectPath, projectID, branch, sha, expectedTip string) (RevertResult, error)
```

**Wire:**

```
GET  /v1/projects/{projectId}/branches/{branchId}/lands?limit=50
  200 { "entries": [HistoryEntry, …] }

POST /v1/projects/{projectId}/branches/{branchId}/lands/{sha}/undo
  request { "expectedTip": "d5f0a17…" }
  200 UndoResult

POST /v1/projects/{projectId}/branches/{branchId}/lands/{sha}/revert
  request { "expectedTip": "e19b7c2…" }
  201 RevertResult

Refusals: 409 conflict with the reasons above; 400 bad_request for a sha that is not 40 hex or a
missing expectedTip; 500 git_failed with gitSays.
```

```json
{ "entries": [
  { "sha": "d5f0a17…", "parents": ["a90d3f4…", "7c2e5b1…"], "authorName": "Dylan", "at": "2026-09-23T05:40:00Z",
    "subject": "Land upload-ui into evidence-upload", "kind": "land", "worktreeId": "w-ui", "worktreeName": "upload-ui",
    "briefKey": "brief:b-42@6", "briefDigest": "sha256:…", "sessions": ["s-9a01bb23"], "reverts": "",
    "pushed": false, "undo": "exact", "undoWhy": "" },
  { "sha": "1c9e2f0…", "parents": ["a90d3f4…"], "authorName": "Dylan", "at": "…",
    "subject": "Log the rejected chunk's checksum", "kind": "direct", "worktreeId": "", "worktreeName": "",
    "briefKey": "", "briefDigest": "", "sessions": [], "reverts": "", "pushed": true, "undo": "none", "undoWhy": "" }
] }
```

Audit line for the GET: *"Reads first-parent history, trailers and the unpushed set with git log
and rev-list, and the lane index for names. Writes nothing."*

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestHistoryClassifiesLandsDirectAndReverts` | a land, a direct commit, a terminal `git merge --no-ff`, a revert, and a merge-mode bring-in → kinds land, direct, direct, revert, bring-in; the bring-in's `undo == "none"` | a terminal merge shown as a cockpit land, or the cockpit's own bring-in shown as committed outside it (re-review b05 issue 5) |
| `TestHistoryReadsTheMergeLinkVerbatim` | `briefKey` equals the trailer byte for byte; a land with no brief → `""` | a key parsed or defaulted |
| `TestHistoryLandSessionsComeFromTheLanesCommits` | lane commits with `s-1`, `s-2`, and one without → `["s-1","s-2"]` in log order, the bare one absent | reading the merge commit's own trailers |
| `TestHistoryUndoIsExactOnlyAtTheUnpushedTip` | tip land unpushed → exact; after a later land → `revert`/`built_upon`; after a push to a bare remote → `revert`/`pushed` | offering an exact undo that rewrites pushed history |
| `TestHistoryNeverPushedMeansNotPushed` | no upstream → every `pushed:false`, no rev-list against an upstream | an error for a local-only branch |
| `TestHistoryWorktreeNameAfterRemoval` | the lane removed from the index → `worktreeName ""`, `worktreeId` still set | a history row lost with its worktree |
| `TestUndoMovesTheBranchBackExactly` | tip returns to the land's first parent; `refs/panacea/undone/<sha>` names the merge; the lane ref is untouched | an undo that loses the lane's commits |
| `TestUndoRestoresReady` | ready false after the land → true after undo | a lane that silently drops out of the queue |
| `TestUndoRefusesAPushedLand` | pushed → `undo_not_exact`/`pushed`, nothing moved | a force-push by the back door (A3) |
| `TestUndoRefusesANonTip` | a land under a later commit → `undo_not_exact`/`not_the_tip` | rewriting history below a newer commit |
| `TestUndoRefusesALandALaneHasMerged` (review I3) | land A → M; lane B's clone runs `git merge origin/<branch>` (M) and a lane sync makes `refs/panacea/lanes/B` contain M; undo M → `undo_not_exact`/`merged_into_lane`, `details.worktrees == "B"`, the tip still M, no `refs/panacea/undone/<M>`; History's tip row → `undo:"revert"`, `undoWhy:"merged_into_lane"`; the revert of M then lands | an undo that only checks the tip, after which B's next land brings A's work back unannounced |
| `TestUndoExemptsTheLandsOwnLane` (review I3) | land A → M; A's own clone merges origin/<branch> and syncs, so `refs/panacea/lanes/A` contains M → History's tip row `undo:"exact"`, and Undo succeeds | refusing every undo whose lane later pulled the branch, which leaves the user only a revert for no reason |
| `TestHistoryMergedIntoLaneIsReadFresh` (review I3) | History (tip row exact); lane B merges M and syncs, the tip unchanged; History again → `undoWhy:"merged_into_lane"` | a lane-containment answer cached under a key that a lane sync does not change |
| `TestUndoRefusesAStaleView` | `expectedTip` ≠ tip → `branch_moved` | undoing a land the human no longer sees |
| `TestUndoLosesARaceCleanly` | the branch moves between the check and `update-ref` → `branch_moved`; the tip is the racer's | `update-ref` without its old value |
| `TestRevertTakesBackTheLandsChange` | after landing A then B, revert A → the tree equals "B applied without A", one parent, `Panacea-Reverts: <A>` | a revert of the wrong side of the merge |
| `TestRevertConflictWritesNothing` | a later commit edits A's lines, in a file named `docs/résumé.md` → `merge_conflict`, `json.Unmarshal(details.paths) == ["docs/résumé.md"]` (review I2), tip unchanged | a half-written revert; a revert's merge-tree without `-z` |
| `TestRevertOfADirectCommitIsRefused` | → `undo_not_exact`/`not_a_land` | reverting arbitrary commits through the land route |
| `TestRevertRefreshesTheClones` (review I4) | two live lanes; revert a land → each clone's `refs/remotes/origin/<branch>` equals `RevertResult.Revert`, and `Refreshed` names both lanes | a revert that leaves every clone's origin/<branch> at the reverted land, so a resolve prompt merges the land back in |
| `TestUndoAndLandSerialise` | undo and a land on the same branch concurrently under `-race` → one wins, the other is refused, the tip is one of the two outcomes | an undo that moves the tip under a land |
| `TestUndoAndRevertWaitOnTheProjectLock` (review I7) | the test holds `Locks.Lock(ctx, p)` → an undo and a revert on `p` run no git call 50 ms later (a recording Runner); release → both proceed | an undo or revert that skips the one lock push and fetch take |
| `TestHistoryGETIsReadOnly` | the §10a audit passes; `for-each-ref` before and after a GET is identical | a GET that writes |

#### Steps

- [ ] **Step 1: Preflight.** Task 09 merged; task 01's `TestBehaviourMergeTreeWithMergeBaseRevertsAMerge`
  passed on the host git.
- [ ] **Step 2:** tests; `history.go`, `undo.go`; handlers, routes, audit line.
- [ ] **Step 3:** run and commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/converge/ ./control-plane/internal/api/ -race -count=1 → PASS
```

#### What this task does not claim

- **A reverted lane cannot simply be landed again.** This is git's own behaviour for a reverted
  merge: the lane's commits are already in history, so a second land brings nothing back. The
  worktree's agent must revert the revert, or make new commits. The Merge lens says so on a
  reverted row (task 20); nothing here automates it.
- Undo does not reach the remote. After a push, the only undo is a revert, then a push.
- `refs/panacea/undone/*` is never pruned by phase 3; retention is contract Q5's erasure question.
