### Task 11: Remove a worktree; create, rename and delete a product branch

The lifecycle writes around convergence (brief §2.1). Each one changes an id phase 2 derives
(survey §1.3), and each has a way to lose work the user did not mean to lose, so each is built
around that loss:

- **Removing a worktree** can discard uncommitted files and commits that never landed, and can
  pull a directory out from under a running agent. It is the one place these lenses ask first
  (survey §6.2; `DESIGN-SYSTEM.md`'s undo-not-confirm rule presumes an undo, and discarding files
  has none). The question names the sessions, the dirty paths and the unlanded commits, and the
  server holds the human to what they were shown with a **confirm token** over that state.
  Unlanded commits are archived under `refs/panacea/archive/lanes/…` before anything is deleted,
  and so is work on the clone's other branches or a detached HEAD, which never syncs, under
  `refs/panacea/archive/offbranch/…` (review I6).
  And, per **OD-10**, removal deletes the worktree's conversation store.
- **Deleting a branch** archives its tip (A7), so commits a release, a land or an evidence record
  names stay reachable, and asks phase 4 to archive the brief head (P13; ask P4-3).
- **Renaming a branch** records `old id → new id` (A8) so branch-keyed stores can follow it.

**Sessions are never destroyed by this task.** Contract §5.2 permits exactly three destroys (kill,
the reaper, restart). A worktree with a live session is refused (`live_sessions`); the desktop
kills them through the existing kill verb as part of the same question (task 22), then removes.

**Files:**
- `control-plane/internal/lanes/remove.go` + test — `RemovalState`, `Inspect`, `Remove` (new)
- `control-plane/internal/converge/branches.go` + test — `CreateBranch`, `RenameBranch`, `DeleteBranch` (new)
- `control-plane/internal/api/worktrees.go` — DELETE (task 04's file)
- `control-plane/internal/api/branches.go` + test — POST, PATCH, DELETE branches; GET renames (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go`, `routes_audit_test.go` — `// ── task 11 ──`

#### Interfaces — complete

```go
package lanes

// RemovalState is what removing a lane would lose. The desktop shows all of it in one question.
type RemovalState struct {
	LiveSessions []string // "id\ttitle", registry sessions not exited
	Uncommitted  []string // paths from the clone's `status --porcelain=v2 -z` (task 07's parser); empty when its directory is gone
	Unlanded     []string // short shas: LaneRef ^refs/heads/<branch>, at most 20, after a SyncLane
	LaneTip      string
	// Elsewhere is work in the clone the lane does not reach (review I6): "refs/heads/<name>\t<short
	// sha>" for each clone branch other than <branch> whose tip refs/heads/<branch> does not
	// contain, and "HEAD\t<short sha>" for a detached HEAD it does not contain; sorted, at most 20.
	// None of it syncs, so without this the question omits it and RemoveTree deletes it.
	Elsewhere []string
	// Unlisted is git's sentence when the Elsewhere listing itself failed (a --no-merged walk that
	// hits a missing object, a Read timeout; re-review b04-1 issue 2). Elsewhere is then empty but
	// not known to be empty, so the archive takes the wildcard and NothingToLose is false.
	Unlisted string
	// ArchivedAt is the prefix Inspect's archive (below) fetched into, OffBranchPrefix(id, now), or
	// "" when there was nothing to archive. With Unarchivable set, some or none of it was written.
	ArchivedAt string
	// Unarchivable is git's sentence when Inspect's archive of Elsewhere (or, with Unlisted or
	// Refused set, of the clone's whole refs/heads/*) failed: a clone with a broken or hostile ref.
	// The archive is real, not a dry run, because a dry run skips the ref writes and so cannot see a
	// ref-write failure: a D/F pair among the clone's own packed refs, a path too long, a held lock
	// (re-review b04-1 issue 1, probed). It is in the question, and its presence is in the token, so
	// the human confirms the loss knowingly, and a clone an agent broke stays removable.
	// "" when the archive succeeded or had nothing to do.
	Unarchivable string
	// Refused is the key task-02 §HGH refused in the clone's config, so its status was never
	// run (Scope Clone). It is "" when the status was read. A refused clone must stay removable,
	// or an agent could make its own worktree permanent. So Refused is not an error:
	// NothingToLose is false and the question says the uncommitted files could not be listed.
	Refused string
	// Confirm is hex(sha256(sorted LiveSessions ‖ "\n" ‖ sorted Uncommitted ‖ "\n" ‖ Refused ‖ "\n" ‖
	// LaneTip ‖ "\n" ‖ sorted Elsewhere ‖ "\n" ‖ set(Unlisted) ‖ "\n" ‖ set(Unarchivable)))[:16],
	// where set(s) is "1" when s is non-empty and "" when not.
	// A DELETE carrying it proves the human saw this exact state. Git's wording of a failure is
	// shown, never hashed. The token must survive a retry, and nothing pins that wording across
	// attempts: with --quiet the D/F failure names only the clone's path (probed, fix batch b04
	// round 2), but without it each ref line names the attempt's <unix> prefix, and a timeout can
	// name a duration. ArchivedAt, which holds that <unix>, is not hashed either. What is at risk
	// is bound by Elsewhere's shas.
	Confirm string
}

// NothingToLose is true when there are no uncommitted paths, no unlanded commits, nothing
// Elsewhere, and Unlisted, Unarchivable and Refused are all "".
func (s RemovalState) NothingToLose() bool

// Inspect gathers RemovalState. It syncs the lane first so Unlanded is current, and it archives
// what the lane does not reach before it answers, so the question reports what an archive did,
// not what a probe predicted (re-review b04-1 issue 1). The clone's status runs with Scope Clone
// (task-02 §HGH). When §HGH refuses the clone's config, it sets Refused, and Unlanded comes from
// the project's existing lane ref, the last sweep's.
// Elsewhere (review I6), in the clone with Scope Clone, all Read:
//   for-each-ref --no-merged=refs/heads/<b> --format=%(refname)%00%(objectname) refs/heads/
//   symbolic-ref -q HEAD (OKExit 1); when detached: rev-parse --verify -q HEAD, then
//   merge-base --is-ancestor <sha> refs/heads/<b> (OKExit 1; 1 means Elsewhere)
// Any of these failing sets Unlisted to git's sentence and leaves Elsewhere empty. When the clone
// has no refs/heads/<b>, every refs/heads/* ref is Elsewhere. Then, when Elsewhere is non-empty
// or Unlisted or Refused is set, the archive, in the PROJECT (Local, Scope Project; the clone is
// only the upload-pack source, which task-02's TestHGHUploadPackSourceRunsNothing pins as
// running nothing of the clone's), into prefix = OffBranchPrefix(lane.ID, now.Unix()):
//   fetch --no-tags --no-write-fetch-head --quiet <FetchURL(clone)> <OffBranchRefspecs(st, prefix)>
// It sets ArchivedAt to prefix, and on failure Unarchivable to git's sentence. It only adds refs,
// so what a failed fetch did write stays. Each attempt has its own <unix>; an earlier attempt's
// archive is kept, like every archive ref (Q5), and a same-second retry force-updates the same refs.
func Inspect(ctx context.Context, m gitx.Machine, sync func(ctx context.Context, id string) error,
	projectPath string, lane Lane, live []string, now time.Time) (RemovalState, error)

// Removed is a removal that happened.
type Removed struct {
	WorktreeID string `json:"worktreeId"`
	ArchivedAs string `json:"archivedAs"` // "refs/panacea/archive/lanes/<id>/<unix>", or "" when nothing was unlanded
	// ArchivedElsewhere is st.ArchivedAt: the prefix this request's Inspect archived into, else "".
	ArchivedElsewhere string `json:"archivedElsewhere"`
}

// OffBranchPrefix is where Inspect archives a clone's other branches and detached HEAD:
// "refs/panacea/archive/offbranch/<id>/<unix>". Not under refs/panacea/archive/lanes/<id>/<unix>,
// which is itself a ref: git refuses a ref beneath another ref (a D/F conflict), so review I6's
// suggested ".../lanes/<id>/<unix>/<name>" could never be written beside step 1's ref.
func OffBranchPrefix(worktreeID string, unix int64) string

// OffBranchRefspecs maps st.Elsewhere to fetch refspecs under prefix, each named, never a
// wildcard: "+refs/heads/<name>:<prefix>/heads/<name>" per branch, "+HEAD:<prefix>/HEAD" for a
// detached HEAD. With st.Refused or st.Unlisted set, it is the one wildcard
// "+refs/heads/*:<prefix>/heads/*".
func OffBranchRefspecs(st RemovalState, prefix string) []string

// Remove, in this order, each step idempotent so a failed removal can simply be retried:
//  1. Unlanded non-empty → update-ref refs/panacea/archive/lanes/<id>/<unix> <LaneTip>   (project)
//  1a. No git. This request's Inspect already archived the clone's other branches (review I6),
//      so Removed.ArchivedElsewhere is st.ArchivedAt. An archive failure is therefore
//      Unarchivable inside the token the human confirmed, never a 500 here: a failure no probe
//      can see cannot make the clone unremovable (re-review b04-1 issue 1).
//  2. update-ref -d refs/panacea/lanes/<id> <LaneTip>                                    (project)
//  3. RemoveTree(CloneRoot, lane.Path) — skipped when the directory is already gone
//  4. Store.Remove(id)
// Steps 1-2 run with Scope Project (task-02 §HGH). Step 3 runs no git in the clone. The handler
// passes Inspect and Remove one now, so step 1's <unix> and ArchivedAt's are the same.
// The caller (the handler) then clears the ready flag, deletes the worktree's exited session
// records from the registry, and deletes its conversation store (OD-10).
func Remove(ctx context.Context, m gitx.Machine, store Store, projectPath string, lane Lane, st RemovalState, now time.Time) (Removed, error)
```

```go
package converge

// BranchJSON is project.Branch as the rail reads it.
type BranchJSON = project.Branch

// CreateBranch: `check-ref-format --branch <name>` (Read; refused → bad_request with git's
// sentence); from = rev-parse refs/heads/<fromBranch>^{commit}; create-only
// `update-ref refs/heads/<name> <from> 0000000000000000000000000000000000000000` (Local) —
// refused → branch_exists. No commit is made, so no identity is needed.
// All three branch verbs hold locks.Lock(ctx, projectID) for the whole verb: they move
// refs/heads/* and branch.<name>.* config, which a land or a push on the project reads (task-02
// Locks; review I7). locks is GitDeps.Locks.
func CreateBranch(ctx context.Context, m gitx.Machine, locks *gitx.Locks, projectPath, projectID, name, fromBranch string) (project.Branch, error)

// RenameResult is a rename that happened.
type RenameResult struct {
	Branch      project.Branch `json:"branch"`
	RenamedFrom string         `json:"renamedFrom"` // the old branch id
	// RemoteKeepsOldName is true when the branch had an upstream: the remote branch is not
	// renamed, and the next push publishes the new name (task 12 sets the upstream then).
	RemoteKeepsOldName bool `json:"remoteKeepsOldName"`
}

// RenameBranch refuses branch_has_worktrees (details.worktrees = the lanes' names; open question
// Q4) and branch_exists; otherwise `branch -m <old> <new>` (Local), which moves the reflog and
// branch.<old>.* config and updates any checkout's HEAD; then Store.AddRename and
// Store.Retarget(projectID, old, new). Scope Project (task-02 §HGH), and no hook runs (H4).
func RenameBranch(ctx context.Context, m gitx.Machine, locks *gitx.Locks, store lanes.Store, projectPath, projectID, old, new string, now time.Time) (RenameResult, error)

// DeleteResult is a deletion that happened.
type DeleteResult struct {
	ArchivedAs    string `json:"archivedAs"`    // refs/panacea/archive/<branch id>/<unix>
	ArchivedBrief bool   `json:"archivedBrief"` // Archiver answered nil; false when none is configured or it failed
}

// DeleteBranch refuses branch_has_worktrees and branch_checked_out; otherwise:
//   tip = rev-parse refs/heads/<name>
//   update-ref refs/panacea/archive/<branchID>/<unix> <tip> 0000…      (create-only)
//   update-ref -d refs/heads/<name> <tip>                              (compare-and-swap)
//   config --remove-section branch.<name>                              (absent section is fine)
//   Store.SetQueue(projectID, name, nil); Archiver.ArchiveHead(projectID, branchID) when non-nil
// The remote branch is not deleted. Every git call runs in the project with Scope Project
// (task-02 §HGH), and no hook runs (H4).
func DeleteBranch(ctx context.Context, m gitx.Machine, locks *gitx.Locks, store lanes.Store, arch briefref.Archiver, projectPath, projectID, branchID, name string, now time.Time) (DeleteResult, error)
```

```go
// gitdeps.go
	// ── task 11 ──
	Archiver briefref.Archiver // nil until phase 4 (ask P4-3); the interface is task 03's (review I7)
```

**Wire:**

```
DELETE /v1/projects/{projectId}/worktrees/{worktreeId}[?confirm=<token>]
  200 { "worktreeId": "w-…", "archivedAs": "refs/panacea/archive/lanes/w-…/1790156728",
        "archivedElsewhere": "refs/panacea/archive/offbranch/w-…/1790156728" }
  409 conflict  reason live_sessions      details.sessions "s-1\tupload-api\ns-2\tShell · upload-api"
  409 conflict  reason confirm_required   details.paths (a JSON array: task 04's detailList; review I2),
                                          details.commits, details.elsewhere (each newline-joined),
                                          details.unlisted, details.unarchivable (git's sentence, or
                                          absent), details.confirm
  409 conflict  reason confirm_stale      the same details, freshly read (an archive that failed
                                          after the token was shown lands here, with unarchivable)
  409 conflict  reason not_a_cockpit_worktree   (kind primary or linked: "remove it with git worktree remove")
  404 worktree_not_found

POST   /v1/projects/{projectId}/branches              { "name": "fix-session-timeout", "fromBranchId": "b-…" }
  201 Branch                                          400 bad_request (gitSays); 409 branch_exists
PATCH  /v1/projects/{projectId}/branches/{branchId}   { "name": "evidence-upload-v2" }
  200 RenameResult                                    409 branch_has_worktrees | branch_exists
DELETE /v1/projects/{projectId}/branches/{branchId}
  200 DeleteResult                                    409 branch_has_worktrees | branch_checked_out
GET    /v1/projects/{projectId}/renames
  200 { "renames": [ { "projectId", "fromBranchId", "toBranchId", "from", "to", "at" } ] }
```

The handler for a DELETE with nothing to lose proceeds without a token; with something to lose it
requires `confirm` to equal a freshly computed `RemovalState.Confirm`. It takes one `now` per
request and passes it to both `Inspect` and `Remove`. Every DELETE runs `Inspect`, so a 409
answer has already archived the clone's other branches. After `lanes.Remove`, in
`worktrees.go`: `Flags.SetWorktreeReady(id, false)`; `Store.Delete` for every **exited** session
record on the worktree (the registry's own purge; a live one cannot exist here); and
`os.RemoveAll(filepath.Join(Deps.ConversationsDir, sessionConversationOwner, id))`, with the id
checked against `sessionStoreSegment` first — both constants are `sessions.go`'s, read, not
edited. Audit line for the GET renames: *"Reads the lane index. Writes nothing."*

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRemoveRefusesWhileASessionIsLive` | a running session on the worktree → `live_sessions`, directory intact, no destroy called on the backend | a fourth destroy site (contract §5.2) |
| `TestRemoveAsksWhenWorkWouldBeLost` | uncommitted `a.txt` and one unlanded commit → `confirm_required` with both and a 16-hex `confirm` | discarding work silently |
| `TestRemoveConfirmPathsAreAJSONArray` (handler; re-review b05 issue 8) | a clone with uncommitted `docs/résumé.md` and `a b.txt` → 409 `confirm_required`; `JSON.parse`-equivalent `json.Unmarshal(details["paths"])` gives exactly `["a b.txt", "docs/résumé.md"]` in status order, unquoted | a newline-joined `details.paths` in the removal handler, or a C-quoted `"docs/r\303\251sum\303\251.md"` |
| `TestRemoveHonoursOnlyTheTokenShown` | token from state 1; then the agent commits; DELETE with the old token → `confirm_stale`, directory intact | a confirmation of a question the human never saw |
| `TestRemoveArchivesUnlandedCommits` | after removal, `refs/panacea/archive/lanes/<id>/<t>` names the old lane tip and `cat-file -e` finds every unlanded commit | unlanded work lost to `gc` (A7, R11) |
| `TestRemoveArchivesOtherClonesBranches` (review I6) | in the clone, `switch -c fix-x`, commit X, `switch <b>`; then `switch --detach`, commit Y. DELETE → `confirm_required` whose `details.elsewhere` names `refs/heads/fix-x` and `HEAD`; with that token → 200, `<archivedElsewhere>/heads/fix-x` names X and `<archivedElsewhere>/HEAD` names Y, `cat-file -e` finds both in the project, and the project's `for-each-ref refs/heads` is unchanged. Red first against `Inspect` without the listing: the first DELETE on a landed lane answers 200 and X is gone | work an agent committed on another branch deleted with its clone |
| `TestRemoveIgnoresBranchesTheLaneContains` | a clone branch `old` whose tip `refs/heads/<b>` contains → not in `Elsewhere`; a clean, landed lane with it → 200 without `confirm` | a question about work that is not at risk |
| `TestRemoveAnUnarchivableCloneIsAskedNotStuck` | two clones, red first: (a) the clone's `refs/heads/bad` is a loose ref file naming a missing object; (b) re-review b04-1 issue 1: the clone's `packed-refs` holds `refs/heads/d` and `refs/heads/d/e`, a D/F pair packed-refs does not refuse, which a `fetch --dry-run` passes and a real fetch fails. Each → `Unarchivable` holds git's sentence and `details.unarchivable` carries it; two DELETEs a second apart (injected `now`) answer the same `details.confirm`; the DELETE with that token → 200 and the clone is gone. Then a fresh lane with `fix-x`: token shown, then a fetch fault injected into `Inspect`'s archive alone (a Runner wrapper) → `confirm_stale` with `unarchivable` set; the directory, the lane ref and the index entry all intact | an agent making its own clone permanent; a dry-run probe that cannot see a ref-write failure; a token that hashes git's per-attempt wording; a failed archive followed by a delete |
| `TestRemoveWithAnUnlistedCloneArchivesTheWildcard` (re-review b04-1 issue 2) | a clean, landed lane whose clone has `fix-x` at X; a Runner wrapper fails the clone's `for-each-ref` alone → `Unlisted` holds its sentence, `Elsewhere` is empty, `NothingToLose()` is false, and the DELETE → `confirm_required` with `details.unlisted`; with that token → 200 and `<archivedElsewhere>/heads/fix-x` names X. Red first against a `NothingToLose` that ignores `Unlisted`: the first DELETE answers 200 and X is gone | a failed listing read as nothing elsewhere |
| `TestRemoveAHostileCloneStillWorks` | task-02 §HGH, red first: the clone's config holds `core.fsmonitor=<marker script>` and `filter.x.clean=<marker script>` → `Inspect` sets `Refused` to the key and writes no marker; `NothingToLose()` is false; the DELETE with that `confirm` removes the clone, after `Inspect`'s wildcard archive (review I6) wrote `<archivedElsewhere>/heads/<b>`, with still no marker | a hostile clone the cockpit can no longer remove |
| `TestRemoveWithNothingToLoseNeedsNoToken` | clean, landed lane → 200 without `confirm` | a question with nothing in it |
| `TestRemoveDeletesTheCloneTheLaneRefAndTheIndexEntry` | the directory, `refs/panacea/lanes/<id>` and the store entry are all gone | half a removal |
| `TestRemoveIsRetryable` | `RemoveTree` fails once (a Runner/FS fake) → second DELETE completes | a lane stuck in the index with no directory |
| `TestRemoveDeletesTheConversationStore` | `<ConversationsDir>/local/<id>` existed → gone after removal | OD-10 not honoured |
| `TestRemovePurgesExitedSessionRecords` | an exited session on the worktree → `Store.Get` answers not found | orphan rows the rail cannot place |
| `TestRemoveRefusesPrimaryAndLinked` | a primary or linked worktree id → `not_a_cockpit_worktree`, nothing touched | the cockpit deleting a user's own checkout |
| `TestCreateBranchIsCreateOnly` | an existing name → `branch_exists`, the existing ref unmoved | overwriting a branch |
| `TestCreateBranchRefusesAnIllegalName` | `a..b` → 400 with git's `check-ref-format` answer | a ref git cannot store |
| `TestRenameRecordsTheIdMap` | rename → `GET renames` holds `{from: b-old, to: b-new}`; `BranchID` of the new name equals `toBranchId` | branch-keyed stores seeing a delete plus a create (A8) |
| `TestRenameRefusesWithWorktrees` | a lane on the branch → `branch_has_worktrees` naming it | renaming under a running agent (Q4) |
| `TestRenameKeepsTheUpstreamFlag` | a branch with an upstream → `remoteKeepsOldName: true` | a silent divergence from the remote |
| `TestDeleteArchivesTheTip` | after delete, the archive ref names the old tip; `refs/heads/<b>` is gone; `branch.<b>.*` config gone | unpushed commits becoming unreachable (R11) |
| `TestDeleteCallsTheArchiverOnce` | a fake Archiver records `(projectID, branchID)` once; a nil Archiver → `archivedBrief: false` | P13's archive request never sent |
| `TestDeleteRefusesACheckedOutBranch` | → `branch_checked_out` | deleting the branch a checkout is on |
| `TestBranchVerbsWaitOnTheProjectLock` (review I7) | the test holds `Locks.Lock(ctx, p)` → a create, a rename and a delete on `p` run no git call 50 ms later (a recording Runner); release → all three proceed | a rename moving `branch.<name>.*` under a push |
| `TestDeleteLosesARaceCleanly` | the branch moves between read and delete → git refuses; the branch still exists | `update-ref -d` without its old value |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 04 and 07 merged, so `briefref` (task 03), `gitx.Locks`
  (task 02) and `GitDeps.Locks` (task 04) exist; nothing here waits on task 09 (review I7). Task 01's `TestBehaviourRemoveAllOnAClone`
  and `TestBehaviourUpdateRefCreateOnlyRefusesAnExistingRef` read.
- [ ] **Step 2:** tests; `remove.go`, `branches.go`; handlers, routes, deps, audit line.
- [ ] **Step 3:** run and commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/lanes/ ./control-plane/internal/converge/ ./control-plane/internal/api/ -race -count=1 → PASS
```

#### What this task does not claim

- Archive refs are never pruned by phase 3. Erasure (a regulated customer asking for a branch to
  be gone) is contract Q5 and now covers `refs/panacea/archive/*` and `refs/panacea/undone/*`.
- Renaming or deleting does nothing at the remote.
- A rename done in a terminal (`git branch -m`) is a delete plus a create to the cockpit, because
  it cannot be told apart (A8).
- Brief-head archiving happens only once phase 4 provides an `Archiver`.
- Review I6 archives the clone's `refs/heads/*` and a detached HEAD only. A stash, a tag, or a
  commit reachable only from the clone's reflog is not archived and is lost with the clone.
- Every DELETE's `Inspect` archives under its own `<unix>`, so a question asked three times
  leaves three archives of the same objects. Phase 3 keeps them all, like every archive ref (Q5).
- Unverified: git's exact sentence for each unarchivable case other than the D/F pair (probed:
  "error: some local refs could not be updated; try running 'git remote prune <clone>' …", the
  same on every attempt). The token hashes only whether `Unarchivable` is set, so nothing
  depends on the wording.
