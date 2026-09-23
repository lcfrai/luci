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
	// trailer), or "direct" — anything else, including a merge made in a terminal. The Merge
	// lens says "committed on <branch> directly, not a land" for "direct".
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
	// Undo: "exact" (this is the tip, a land, not pushed), "revert" (a land that is pushed or
	// built upon), or "none" (not a land). UndoWhy says why an exact undo is not offered:
	// "pushed" | "built_upon" | "".
	Undo    string `json:"undo"`
	UndoWhy string `json:"undoWhy"`
}

// History reads at most limit (1-200, default 50) first-parent entries of refs/heads/<branch>:
//   log --first-parent -n <limit> --format=%H%x00%P%x00%an%x00%aI%x00%s%x00<Land>%x00<Brief>%x00<BriefDigest>%x00<Session>%x00<Reverts>%x1e
//   rev-list --first-parent refs/heads/<branch> ^<upstream>   (the unpushed set; skipped when never pushed)
//   log --format=<Session>%x1e -n 20 <sha>^1..<sha>^2         (per land row)
// all gitx.Read, cached by (tip, upstream tip, limit).
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
// branch_checked_out; undo_not_exact with details.why "not_a_land" | "not_the_tip" | "pushed";
// branch_moved when the tip is not expectedTip. Then, in order:
//   update-ref refs/panacea/undone/<sha> <sha>                           (keeps it and its trailer resolvable)
//   update-ref -m "panacea: undo land <id>" refs/heads/<branch> <sha>^1 <sha>   (compare-and-swap)
//   SetWorktreeReady(<Panacea-Land id>, true) when that lane still exists  (exact means as before)
//   the clones' origin/<branch> refresh, as Land's step 14
// It serialises with Land on the same branch.
func (l *Lander) Undo(ctx context.Context, projectPath, projectID, branch, sha, expectedTip string) (UndoResult, error)

// RevertResult is a revert commit that landed.
type RevertResult struct {
	Revert  string `json:"revert"`
	Before  string `json:"before"`
	Reverts string `json:"reverts"`
}

// Revert lands a commit that takes back one land. Refusals: branch_checked_out; branch_moved;
// undo_not_exact/not_a_land for a sha that is not a land on this branch's first-parent line;
// no_git_identity; merge_conflict (details.paths). Then:
//   tree = merge-tree --write-tree --merge-base=<sha> <tip> <sha>^1        (git ≥ 2.40; OKExit 1)
//   msg  = Message("Revert land <name> into <branch>", "This takes back <sha>.", [Panacea-Reverts: <sha>])
//   rev  = commit-tree <tree> -p <tip> -F -                              (the user's identity)
//   update-ref -m "panacea: revert <sha>" refs/heads/<branch> <rev> <tip>
// The reverted lane's ready flag is not touched.
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
| `TestHistoryClassifiesLandsDirectAndReverts` | a land, a direct commit, a terminal `git merge --no-ff`, and a revert → kinds land, direct, direct, revert | a terminal merge shown as a cockpit land |
| `TestHistoryReadsTheMergeLinkVerbatim` | `briefKey` equals the trailer byte for byte; a land with no brief → `""` | a key parsed or defaulted |
| `TestHistoryLandSessionsComeFromTheLanesCommits` | lane commits with `s-1`, `s-2`, and one without → `["s-1","s-2"]` in log order, the bare one absent | reading the merge commit's own trailers |
| `TestHistoryUndoIsExactOnlyAtTheUnpushedTip` | tip land unpushed → exact; after a later land → `revert`/`built_upon`; after a push to a bare remote → `revert`/`pushed` | offering an exact undo that rewrites pushed history |
| `TestHistoryNeverPushedMeansNotPushed` | no upstream → every `pushed:false`, no rev-list against an upstream | an error for a local-only branch |
| `TestHistoryWorktreeNameAfterRemoval` | the lane removed from the index → `worktreeName ""`, `worktreeId` still set | a history row lost with its worktree |
| `TestUndoMovesTheBranchBackExactly` | tip returns to the land's first parent; `refs/panacea/undone/<sha>` names the merge; the lane ref is untouched | an undo that loses the lane's commits |
| `TestUndoRestoresReady` | ready false after the land → true after undo | a lane that silently drops out of the queue |
| `TestUndoRefusesAPushedLand` | pushed → `undo_not_exact`/`pushed`, nothing moved | a force-push by the back door (A3) |
| `TestUndoRefusesANonTip` | a land under a later commit → `undo_not_exact`/`not_the_tip` | rewriting history below a newer commit |
| `TestUndoRefusesAStaleView` | `expectedTip` ≠ tip → `branch_moved` | undoing a land the human no longer sees |
| `TestUndoLosesARaceCleanly` | the branch moves between the check and `update-ref` → `branch_moved`; the tip is the racer's | `update-ref` without its old value |
| `TestRevertTakesBackTheLandsChange` | after landing A then B, revert A → the tree equals "B applied without A", one parent, `Panacea-Reverts: <A>` | a revert of the wrong side of the merge |
| `TestRevertConflictWritesNothing` | a later commit edits A's lines → `merge_conflict` with paths, tip unchanged | a half-written revert |
| `TestRevertOfADirectCommitIsRefused` | → `undo_not_exact`/`not_a_land` | reverting arbitrary commits through the land route |
| `TestUndoAndLandSerialise` | undo and a land on the same branch concurrently under `-race` → one wins, the other is refused, the tip is one of the two outcomes | an undo that moves the tip under a land |
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
