### Task 11: Remove a worktree; create, rename and delete a product branch

The lifecycle writes around convergence (brief §2.1). Each one changes an id phase 2 derives
(survey §1.3), and each has a way to lose work the user did not mean to lose, so each is built
around that loss:

- **Removing a worktree** can discard uncommitted files and commits that never landed, and can
  pull a directory out from under a running agent. It is the one place these lenses ask first
  (survey §6.2; `DESIGN-SYSTEM.md`'s undo-not-confirm rule presumes an undo, and discarding files
  has none). The question names the sessions, the dirty paths and the unlanded commits, and the
  server holds the human to what they were shown with a **confirm token** over that state.
  Unlanded commits are archived under `refs/panacea/archive/lanes/…` before anything is deleted.
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
	Uncommitted  []string // paths from the clone's status; empty when its directory is gone
	Unlanded     []string // short shas: LaneRef ^refs/heads/<branch>, at most 20, after a SyncLane
	LaneTip      string
	// Confirm is hex(sha256(sorted LiveSessions ‖ "\n" ‖ sorted Uncommitted ‖ "\n" ‖ LaneTip))[:16].
	// A DELETE carrying it proves the human saw this exact state.
	Confirm string
}

// NothingToLose is true when there are no uncommitted paths and no unlanded commits.
func (s RemovalState) NothingToLose() bool

// Inspect gathers RemovalState. It syncs the lane first so Unlanded is current.
func Inspect(ctx context.Context, m gitx.Machine, sync func(ctx context.Context, id string) error,
	projectPath string, lane Lane, live []string) (RemovalState, error)

// Removed is a removal that happened.
type Removed struct {
	WorktreeID string `json:"worktreeId"`
	ArchivedAs string `json:"archivedAs"` // "refs/panacea/archive/lanes/<id>/<unix>", or "" when nothing was unlanded
}

// Remove, in this order, each step idempotent so a failed removal can simply be retried:
//  1. Unlanded non-empty → update-ref refs/panacea/archive/lanes/<id>/<unix> <LaneTip>   (project)
//  2. update-ref -d refs/panacea/lanes/<id> <LaneTip>                                    (project)
//  3. RemoveTree(CloneRoot, lane.Path) — skipped when the directory is already gone
//  4. Store.Remove(id)
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
func CreateBranch(ctx context.Context, m gitx.Machine, projectPath, projectID, name, fromBranch string) (project.Branch, error)

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
// Store.Retarget(projectID, old, new).
func RenameBranch(ctx context.Context, m gitx.Machine, store lanes.Store, projectPath, projectID, old, new string, now time.Time) (RenameResult, error)

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
// The remote branch is not deleted.
func DeleteBranch(ctx context.Context, m gitx.Machine, store lanes.Store, arch briefref.Archiver, projectPath, projectID, branchID, name string, now time.Time) (DeleteResult, error)
```

```go
// gitdeps.go
	// ── task 11 ──
	Archiver briefref.Archiver // nil until phase 4 (ask P4-3)
```

**Wire:**

```
DELETE /v1/projects/{projectId}/worktrees/{worktreeId}[?confirm=<token>]
  200 { "worktreeId": "w-…", "archivedAs": "refs/panacea/archive/lanes/w-…/1790156728" }
  409 conflict  reason live_sessions      details.sessions "s-1\tupload-api\ns-2\tShell · upload-api"
  409 conflict  reason confirm_required   details.paths, details.commits (newline-joined), details.confirm
  409 conflict  reason confirm_stale      the same details, freshly read
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
requires `confirm` to equal a freshly computed `RemovalState.Confirm`. After `lanes.Remove`, in
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
| `TestRemoveHonoursOnlyTheTokenShown` | token from state 1; then the agent commits; DELETE with the old token → `confirm_stale`, directory intact | a confirmation of a question the human never saw |
| `TestRemoveArchivesUnlandedCommits` | after removal, `refs/panacea/archive/lanes/<id>/<t>` names the old lane tip and `cat-file -e` finds every unlanded commit | unlanded work lost to `gc` (A7, R11) |
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
| `TestDeleteLosesARaceCleanly` | the branch moves between read and delete → git refuses; the branch still exists | `update-ref -d` without its old value |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 04 and 07 merged. Task 01's `TestBehaviourRemoveAllOnAClone`
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
