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
- `control-plane/internal/lanes/remove.go` + test — `RemovalState`, `Remove`, `OffBranchPrefix`, `OffBranchRefspecs` (new)
- `control-plane/internal/converge/inspect.go` + test — `Inspect`, through task 24's runner (new;
  in `converge` because `lanegit` imports `lanes`)
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
	Uncommitted  []string // paths from runner.Status's PorcelainV2Z (task 24; task 07's parser); empty when its directory is gone
	Unlanded     []string // short shas: LaneRef ^refs/heads/<branch>, at most 20, after a SyncLane
	LaneTip      string
	// Elsewhere is work in the clone the lane does not reach (review I6): "refs/heads/<name>\t<short
	// sha>" for each clone branch other than <branch> whose tip refs/heads/<branch> does not
	// contain, and "HEAD\t<short sha>" for a detached HEAD it does not contain; sorted, at most 20.
	// None of it syncs, so without this the question omits it and RemoveTree deletes it.
	Elsewhere []string
	// Unlisted is the runner's sentence (a RunnerError's GitSays) when runner.Refs answered but the
	// Elsewhere listing failed (a --merged walk that hits a missing object, git_timeout;
	// re-review b04-1 issue 2). Elsewhere is then empty but not known to be empty, so the archive
	// takes every ref in Listed and NothingToLose is false.
	Unlisted string
	// Listed is set only with Unlisted: "refs/heads/<name>\t<sha>" for every ref a second
	// runner.Refs{Names: [refs/heads]}, MergedInto "" (no walk), names, plus "HEAD\t<sha>" when
	// detached; sorted. It is what the archive carries in place of Elsewhere. Not hashed: with
	// Unlisted, set(Unlisted) is what the token binds, as the wildcard it replaces bound no sha.
	Listed []string
	// ArchivedAt is the prefix Inspect's archive (below) fetched into, OffBranchPrefix(id, now), or
	// "" when there was nothing to archive. With Unarchivable set, none of it was written
	// (ArchiveLane's project fetch is --atomic, and a failed update-ref of its reached refs
	// deletes what the fetch wrote; task 24), bar a failed rollback, which is logged.
	ArchivedAt string
	// Unarchivable is the sentence when Inspect's archive of Elsewhere (or, with Unlisted set, of
	// Listed) failed, from the runner's bundle or from task 24's ArchiveLane checks and fetch in the
	// project: a clone with a broken or hostile ref.
	// The archive is real, not a dry run, because a dry run skips the ref writes and so cannot see a
	// ref-write failure: a D/F pair among the clone's own packed refs (the bundle carries both refs
	// and the project's --atomic fetch cannot write both), a path too long, a held lock
	// (re-review b04-1 issue 1, probed). It is in the question, and its presence is in the token, so
	// the human confirms the loss knowingly, and a clone an agent broke stays removable.
	// "" when the archive succeeded or had nothing to do.
	Unarchivable string
	// Unreadable is set when the runner cannot answer: LaneGit.For fails (ErrNoRunner on katabe,
	// a runner-only sandbox that does not start), runner.Status fails, or runner.Refs gets no
	// answer (a RunnerError with Status 0) or a code other than git_failed and git_timeout (a 400,
	// review r4-3 C-1). A lane whose runner cannot answer stays removable, or
	// an agent could make its own worktree permanent by breaking its runner. So Unreadable is not
	// an error: NothingToLose is false and the question says the lane's files and branches could
	// not be read.
	Unreadable string // the runner's sentence when the lane's status or refs could not be read
	// Confirm is hex(sha256(sorted LiveSessions ‖ "\n" ‖ sorted Uncommitted ‖ "\n" ‖ set(Unreadable) ‖
	// "\n" ‖ LaneTip ‖ "\n" ‖ sorted Elsewhere ‖ "\n" ‖ set(Unlisted) ‖ "\n" ‖ set(Unarchivable)))[:16],
	// where set(s) is "1" when s is non-empty and "" when not.
	// A DELETE carrying it proves the human saw this exact state. Git's and the runner's wording of
	// a failure is shown, never hashed. The token must survive a retry, and nothing pins that
	// wording across attempts: the project fetch's source is now a bundle file whose name is random
	// per call (task 24), each ref line names the attempt's <unix> prefix, and a timeout can name a
	// duration. ArchivedAt, which holds that <unix>, is not hashed either. What is at risk is bound
	// by Elsewhere's shas.
	Confirm string
}

// NothingToLose is true when there are no uncommitted paths, no unlanded commits, nothing
// Elsewhere, and Unlisted, Unarchivable and Unreadable are all "".
func (s RemovalState) NothingToLose() bool
```

```go
package converge // not lanes: task 24's lanegit imports lanes, so lanes cannot import lanegit

// Inspect gathers lanes.RemovalState. It syncs the lane first so Unlanded is current, and it
// archives what the lane does not reach before it answers, so the question reports what an
// archive did, not what a probe predicted (re-review b04-1 issue 1). The host runs no git in the
// clone (R4.2): every read of it is lg.For(ctx, lane)'s runner (task 24). Remove needs no live
// session, so with none on the lane the pool starts a runner-only sandbox for this request.
// When For fails, or any runner call gets no answer, Inspect sets Unreadable and asks the runner
// nothing more, and Unlanded comes from the project's existing lane ref, the last sweep's. When
// lane.Path is already gone, For is not called: there is no clone left to lose from.
//   Uncommitted: runner.Status's PorcelainV2Z through task 07's parser. Any failure sets
//     Unreadable; Refs is still asked when the runner answered (a git_failed status).
//   Elsewhere (review I6): runner.Refs{Names: [refs/heads, refs/remotes/origin/<b>], MergedInto:
//     refs/heads/<b>}; each refs/heads/ ref other than <b> with Merged false. The origin/<b> row
//     is the claim the archive's basis uses (below), never Elsewhere, and is left out of the
//     token. "refs/heads" is a prefix, not a glob: for-each-ref
//     matches it up to a slash, so it lists refs/heads/a and refs/heads/b/c, and it passes the
//     runner's plain check-ref-format, which refuses "refs/heads/*" as 400 (task 24's
//     Validation and runner test 11; review r4-3 C-1). Merged must be positive evidence: a ref naming a
//     missing object is in neither --merged nor --no-merged (probed, git 2.53, both exit 0), so
//     task 24's refs op takes Merged from --merged=<MergedInto> only (task 24, the refs op and
//     its runner test 9; review r4-2 I-3).
//     Then the answer's Head: detached (Ref "") with a Sha is Elsewhere unless, in the PROJECT (Read, Scope Project), cat-file -e <sha>^{commit} finds it
//     and merge-base --is-ancestor <sha> <LaneTip> exits 0 (OKExit 1); the runner's sha is data.
//   A git_failed or git_timeout answer is followed by runner.Refs{Names: [refs/heads]} with
//     MergedInto "", which walks nothing. When that listing has no refs/heads/<b>, every ref it
//     names is Elsewhere; otherwise Unlisted is the first answer's GitSays and Listed that listing.
//   Any other RunnerError from Status or Refs (a 400 is a host bug, which task 24's shared
//     Validation goldens make a red host test) sets Unreadable, as no answer does: the lane is
//     asked about, never removed as if clean.
// Then, when Elsewhere or Listed is non-empty, the archive, into the PROJECT, with prefix =
// OffBranchPrefix(lane.ID, now.Unix()), with bundles = LaneGit.Bundles(ctx, lane) (task 24,
// seam S8-P3.4; locally LocalBundles):
//   lanegit.ArchiveLane(refs: each refname of Elsewhere (or Listed) [+ HEAD when detached, which
//                       is then among them] → the full sha runner.Refs (or Head) answered,
//                       basis, prefix, refspecs: OffBranchRefspecs(st, prefix))
// on the same runner, into projectPath (the *gitx.Exec behind m, as task 07 passes it). The
// runner bundles them and the host verifies and fetches the bundle file (task 24, R4.4); the
// project fetch keeps --no-auto-maintenance for task 07's reason (no detached gc in the project;
// review Minor 8). basis (review r4-3 I-1) is [LaneTip, the project's refs/heads/<b> tip, the
// lane's claim refs/remotes/origin/<b>]: LaneTip and the tip are project-held by construction
// (Inspect's SyncLane has just moved LaneRef), the claim is Refs' answer for that name (asked in
// the Elsewhere call's Names beside refs/heads, and kept out of Elsewhere), and ArchiveLane keeps
// only the shas cat-file -e finds in the project, then the runner only those the lane holds. So
// an archive carries the off-branch commits, not the lane's whole history, and ArchiveLane's one
// no-basis retry covers a verify that still fails. A ref the basis reaches (with Listed,
// refs/heads/<b> at LaneTip; a detached HEAD or a branch at origin/<b>) is not asked of the
// runner, since `bundle create` would leave it out: ArchiveLane writes its archive ref itself,
// at a commit the project holds (task 24's split; review r4-4 C-1). When every ref is reached,
// no runner is called. With Unreadable set nothing is
// archived, since no runner can bundle. ArchiveLane, not FetchLane: FetchLane admits only
// refs/heads/ refs and refs/panacea/lanes/ destinations, and ArchiveLane admits "HEAD" and
// destinations under prefix+"/" only (task 24, "ArchiveLane, in order", TestArchiveRoundTrip;
// review r4-2 C-1). Neither widens the other's namespace.
// It sets ArchivedAt to prefix, and on failure Unarchivable to the sentence (a RunnerError's
// GitSays, or "the runner answered <Code>" when GitSays is empty, as a 504 git_timeout's is; a
// *BundleRefused's Why; a *gitx.Error's), so Unarchivable is never "" on a failure. ArchiveLane
// never answers ErrBundleEmpty (task 24; review r4-4 C-1). A failed archive writes no ref (the
// --atomic fetch, and the rollback of a failed reached-ref write). Each attempt has its own
// <unix>; an earlier attempt's archive is kept, like
// every archive ref (Q5), and a same-second retry force-updates the same refs.
func Inspect(ctx context.Context, m gitx.Machine, lg *lanegit.Lanes,
	sync func(ctx context.Context, id string) error,
	projectPath string, lane lanes.Lane, live []string, now time.Time) (lanes.RemovalState, error)
```

```go
package lanes

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

// OffBranchRefspecs maps st.Elsewhere (or, with st.Unlisted set, st.Listed) to fetch refspecs
// under prefix, each named, never a wildcard, so each is "+<one of ArchiveLane's refs>:<dst>":
// "+refs/heads/<name>:<prefix>/heads/<name>" per branch, "+HEAD:<prefix>/HEAD" for a detached
// HEAD. With st.Unreadable set it is nil: nothing is archived.
func OffBranchRefspecs(st RemovalState, prefix string) []string

// Remove, in this order, each step idempotent so a failed removal can simply be retried:
//  1. Unlanded non-empty → update-ref refs/panacea/archive/lanes/<id>/<unix> <LaneTip>   (project)
//  1a. No git. This request's Inspect already archived the clone's other branches (review I6),
//      so Removed.ArchivedElsewhere is st.ArchivedAt. An archive failure is therefore
//      Unarchivable inside the token the human confirmed, never a 500 here: a failure no probe
//      can see cannot make the clone unremovable (re-review b04-1 issue 1).
//  2. update-ref -d refs/panacea/lanes/<id> <LaneTip>                                    (project)
//  2a. drop(ctx, lane.ID), task 24's Lanes.Drop (review r4-1 I-4): the pool destroys the lane's idle
//      runner-only sandbox now, not after Idle, and refuses For for the id from here on, so no
//      bind mount is live on lane.Path during step 3, and no sweep between steps 3 and 4 (the
//      lane is still in Store) can start a sandbox on a deleted path. A Drop error stops the
//      removal before step 3; a retry repeats 2a, which is idempotent.
//  3. fs.RemoveTree(ctx, CloneRoot, lane.Path), fs = lanes.FSOf(machine) (task 03, seam
//     S8-P3.6; locally LocalFS, task 03's RemoveTree) — skipped when fs.Exists says the
//     directory is already gone; an FSOf error stops the removal before step 1
//  4. Store.Remove(id)
// Steps 1-2 run with Scope Project (task-02 §HGH). Step 3 runs no git in the clone: RemoveTree is
// a file operation, not git, so it needs no runner (R4.2). Inspect releases its runner before it
// returns, so 2a does not wait on this request's own hold. The handler
// passes Inspect and Remove one now, so step 1's <unix> and ArchivedAt's are the same.
// The caller (the handler) then clears the ready flag, deletes the worktree's exited session
// records from the registry, and deletes its conversation store (OD-10).
// drop is the handler's GitDeps.LaneGit.Drop, passed as a func because lanegit imports lanes
// (Lanes.Store, For's lanes.Lane), so lanes cannot import lanegit.
func Remove(ctx context.Context, m gitx.Machine, drop func(ctx context.Context, laneID string) error,
	store Store, projectPath string, lane Lane, st RemovalState, now time.Time) (Removed, error)
```

```go
package converge

// BranchJSON is project.Branch as the rail reads it.
type BranchJSON = project.Branch

// ValidBranchName is the one name check, for CreateBranch's name and RenameBranch's new name
// (review Minor 6). In the project (Read, Scope Project):
//   out = check-ref-format --branch <name>
// Refused unless git exits 0 AND out is exactly <name> + "\n". The --branch form keeps two
// rules `check-ref-format refs/heads/<name>` lacks: it refuses a leading "-" and the name HEAD,
// saying "fatal: '<name>' is not a valid branch name", where the refs/heads/ form passes both
// and refuses `a..b` with exit 1 and nothing on stderr (probed, git 2.53). But --branch also
// EXPANDS: after a switch, `@{-1}` prints the previous branch's name and exits 0. The exit alone
// would pass a name update-ref then refuses (a 500), and `branch -m` expands it the same way, so
// a rename would take another branch's name. A printed name that differs is refused with the
// control plane's own sentence, "'<name>' is git shorthand for another branch; choose a literal
// name". Task 01 pins all three behaviours. A refusal is *BadBranchName{GitSays}, which the
// handlers answer 400 bad_request with details.gitSays.
func ValidBranchName(ctx context.Context, m gitx.Machine, projectPath, name string) error

// CreateBranch: ValidBranchName(name) (below; refused → bad_request); from = rev-parse
// refs/heads/<fromBranch>^{commit}; create-only
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

// RenameBranch refuses, in this order: ValidBranchName(new) (bad_request); branch_has_worktrees
// (details.worktrees = the lanes' names; open question Q4); branch_checked_out when
// CheckedOutAt(old) != "" (task 08; details.path), as DeleteBranch does (review Minor 16); and
// branch_exists. Otherwise `branch -m <old> <new>` (Local), which moves the reflog and
// branch.<old>.* config; then Store.AddRename and Store.Retarget(projectID, old, new). Scope
// Project (task-02 §HGH), and no hook runs (H4). The checked-out refusal is needed because
// `branch -m` also rewrites the HEAD of every worktree on <old>, exit 0 and no message (probed,
// git 2.53: the primary's HEAD became refs/heads/<new>), so the user's own checkout would move
// branch under them. The check and the rename both run under the project lock, but a checkout
// made in a terminal between them is not seen, the same window DeleteBranch and a land have.
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
//   config --remove-section branch.<name>                              (OKExit 128; below)
//   Store.SetQueue(projectID, name, nil); Archiver.ArchiveHead(projectID, branchID) when non-nil
// The remote branch is not deleted. Every git call runs in the project with Scope Project
// (task-02 §HGH), and no hook runs (H4).
// remove-section on an absent section is not a success to git: it exits 128, "fatal: no such
// section: branch.<name>" (review Minor 12; task 01 pins it). A branch CreateBranch made has no
// section, since update-ref writes no config, so OKExit 128. Any other failure (exit 255 when
// .git/config.lock is held) comes after the ref is deleted and archived, so the verb still
// answers 200 and logs at warn "branch <name> deleted; its config section stayed: <git's
// sentence>". That is git's own rule: `git branch -D` with the config locked deletes the branch,
// prints "warning: update of config-file failed" and exits 0 (task 01 pins it). 128 is also
// git's exit for a config file it cannot parse; that case leaves the section too, unlogged.
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
                                          details.unlisted, details.unarchivable, details.unreadable
                                          (git's or the runner's sentence, or absent), details.confirm
  409 conflict  reason confirm_stale      the same details, freshly read (an archive that failed
                                          after the token was shown lands here, with unarchivable)
  409 conflict  reason not_a_cockpit_worktree   (kind primary or linked: "remove it with git worktree remove")
  404 worktree_not_found

POST   /v1/projects/{projectId}/branches              { "name": "fix-session-timeout", "fromBranchId": "b-…" }
  201 Branch                                          400 bad_request (gitSays); 409 branch_exists
PATCH  /v1/projects/{projectId}/branches/{branchId}   { "name": "evidence-upload-v2" }
  200 RenameResult                                    400 bad_request (gitSays);
                                                      409 branch_has_worktrees | branch_checked_out | branch_exists
DELETE /v1/projects/{projectId}/branches/{branchId}
  200 DeleteResult                                    409 branch_has_worktrees | branch_checked_out
GET    /v1/projects/{projectId}/renames
  200 { "renames": [ { "projectId", "fromBranchId", "toBranchId", "from", "to", "at" } ] }
```

The handler for a DELETE with nothing to lose proceeds without a token; with something to lose it
requires `confirm` to equal a freshly computed `RemovalState.Confirm`. It takes one `now` per
request and passes it to both `Inspect` and `Remove`, and passes `GitDeps.LaneGit` (task 24) as
`Inspect`'s `lg`. Every DELETE runs `Inspect`, so a 409
answer has already archived the clone's other branches. After `lanes.Remove`, in
`worktrees.go`: `Flags.SetWorktreeReady(id, false)`; `Store.Delete` for every **exited** session
record on the worktree (the registry's own purge; a live one cannot exist here); and
`os.RemoveAll(filepath.Join(Deps.ConversationsDir, sessionConversationOwner, id))`, with the id
checked against `sessionStoreSegment` first — both constants are `sessions.go`'s, read, not
edited. Audit line for the GET renames: *"Reads the lane index. Writes nothing."*

#### The tests to write first

The removal tests run with `lanegittest.Fake` (task 24) as the lane's runner behind `Lanes.For`,
through task 24's seam: `Lanes{Dial: fakes.Dial, Backend: lanegittest.Backend{}}`. The Fake's own
git in the clone is test-side, so task 02's `Spy` sees only production calls, and "in the clone"
below means through the Fake.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRemoveDropsTheLaneRunnerFirst` (review r4-1 I-4), red first | a sessionless lane whose Inspect started a runner-only sandbox (`lanegittest.Backend` records `Create` and `Destroy`, and the order of `Destroy` against `RemoveTree`); `Remove` → one `Destroy` of that sandbox before `RemoveTree`, with no `Idle` wait; a sweep's `SyncLane` on the id between steps 3 and 4 → `ErrLaneDropped` and no `Create`; `Destroy` failing → the directory intact and the lane still in `Store` | a removal that deletes the clone under a live bind mount, or lets a sweep re-create the path |
| `TestRemoveRefusesWhileASessionIsLive` | a running session on the worktree → `live_sessions`, directory intact, no destroy called on the backend | a fourth destroy site (contract §5.2) |
| `TestRemoveAsksWhenWorkWouldBeLost` | uncommitted `a.txt` and one unlanded commit → `confirm_required` with both and a 16-hex `confirm` | discarding work silently |
| `TestRemoveConfirmPathsAreAJSONArray` (handler; re-review b05 issue 8) | a clone with uncommitted `docs/résumé.md` and `a b.txt` → 409 `confirm_required`; `JSON.parse`-equivalent `json.Unmarshal(details["paths"])` gives exactly `["a b.txt", "docs/résumé.md"]` in status order, unquoted | a newline-joined `details.paths` in the removal handler, or a C-quoted `"docs/r\303\251sum\303\251.md"` |
| `TestRemoveHonoursOnlyTheTokenShown` | token from state 1; then the agent commits; DELETE with the old token → `confirm_stale`, directory intact | a confirmation of a question the human never saw |
| `TestRemoveArchiveStartsNoMaintenance` (review Minor 8) | red first: the project's `.git/config` holds `maintenance.auto=true`; a clone with `fix-x` at X, `Inspect` under `t.Setenv("GIT_TRACE", <file>)` → the trace file holds the archive `fetch` and no `run_command: git maintenance` line | a removal leaving a detached gc on the project's `packed-refs.lock` that fails the next land |
| `TestRemoveArchivesUnlandedCommits` | after removal, `refs/panacea/archive/lanes/<id>/<t>` names the old lane tip and `cat-file -e` finds every unlanded commit | unlanded work lost to `gc` (A7, R11) |
| `TestRemoveArchivesOtherClonesBranches` (review I6; on `lanegittest.Fake`) | in the clone, `switch -c fix-x`, commit X, `switch <b>`; then `switch --detach`, commit Y. DELETE → `confirm_required` whose `details.elsewhere` names `refs/heads/fix-x` and `HEAD`; with that token → 200, `<archivedElsewhere>/heads/fix-x` names X and `<archivedElsewhere>/HEAD` names Y, `cat-file -e` finds both in the project, and the project's `for-each-ref refs/heads` is unchanged. The Fake recorded `Refs` with `Names: [refs/heads, refs/remotes/origin/<b>]`, which its Validation passes (`refs/heads/*` would be 400; review r4-3 C-1), and a `Bundle` whose `Basis` holds LaneTip, and the bundle carries two commits, X and Y, not the lane's history (review r4-3 I-1). Red first against `Inspect` without the listing: the first DELETE on a landed lane answers 200 and X is gone | work an agent committed on another branch deleted with its clone |
| `TestRemoveIgnoresBranchesTheLaneContains` | a clone branch `old` whose tip `refs/heads/<b>` contains → not in `Elsewhere`; a clean, landed lane with it → 200 without `confirm` | a question about work that is not at risk |
| `TestRemoveAnUnarchivableCloneIsAskedNotStuck` (on `lanegittest.Fake`) | two clones, red first: (a) the clone's `refs/heads/bad` is a loose ref file naming a missing object, which is not `Merged` and so is `Elsewhere`, and fails in the runner's bundle (`fatal: bad object refs/heads/bad`, probed on git 2.53) → `Unarchivable`; (b) re-review b04-1 issue 1: the clone's `packed-refs` holds `refs/heads/d` and `refs/heads/d/e`, a D/F pair packed-refs does not refuse, which a `fetch --dry-run` passes and a real fetch fails. Each → `Unarchivable` holds git's sentence and `details.unarchivable` carries it; two DELETEs a second apart (injected `now`) answer the same `details.confirm`; the DELETE with that token → 200 and the clone is gone. Then a fresh lane with `fix-x`: token shown, then a fetch fault injected into `Inspect`'s archive alone (a Runner wrapper) → `confirm_stale` with `unarchivable` set; the directory, the lane ref and the index entry all intact | an agent making its own clone permanent; a dry-run probe that cannot see a ref-write failure; a token that hashes git's per-attempt wording; a failed archive followed by a delete |
| `TestRemoveWithAnUnlistedCloneArchivesEveryBranch` (re-review b04-1 issue 2) | a clean, landed lane whose clone has `fix-x` at X; a `lanegit.Runner` wrapper answers `git_failed` to `Refs` with `MergedInto` set, and passes the plain listing → `Unlisted` holds its sentence, `Listed` names `<b>` and `fix-x`, `Elsewhere` is empty, `NothingToLose()` is false, and the DELETE → `confirm_required` with `details.unlisted`; with that token → 200 and `<archivedElsewhere>/heads/fix-x` names X, `<archivedElsewhere>/heads/<b>` names LaneTip, `Unarchivable` is `""`, and the Fake's `Bundle` got `Refs` `[refs/heads/fix-x]` only, since the basis reaches `<b>` (review r4-4 C-1: red first against an ArchiveLane that asks for `<b>`, whose bundle then lacks it). Red first against a `NothingToLose` that ignores `Unlisted`: the first DELETE answers 200 and X is gone | a failed listing read as nothing elsewhere; an archive refused because a ref sits at its basis |
| `TestRemoveArchivesARefAtTheBasis` (review r4-4 C-1) | red first. A landed lane whose clone's `origin/<b>` was refreshed to another lane's land, which LaneTip does not contain, with a detached HEAD there (so Elsewhere) and `fix-x` one commit past it → with the token, 200: `<archivedElsewhere>/HEAD` names `origin/<b>`'s commit and `<archivedElsewhere>/heads/fix-x` names X, the Fake's `Bundle` got `Refs` `[refs/heads/fix-x]` only. The same with every Elsewhere ref reached by the basis → no `Bundle` call, the archive refs written, `Unarchivable` `""` | a detached HEAD at the basis sinking the archive of the other refs' work |
| `TestRemoveALaneWhoseRunnerCannotAnswerStillWorks` | red first, three cases: `Lanes.For` answers `ErrNoRunner` (the katabe backend), a runner whose `Status` gets no answer (a `RunnerError` with `Status` 0), and a runner whose `Refs` answers 400 `bad_request` (review r4-3 C-1) → `Inspect` sets `Unreadable` to the runner's sentence, archives nothing, and answers no error; `NothingToLose()` is false; the DELETE → `confirm_required` with `details.unreadable`; with that `confirm` → 200, the clone, the lane ref and the index entry gone. Red first against a `NothingToLose` that ignores `Unreadable` (the first DELETE answers 200) and against an `Inspect` that returns For's error (a 500 forever) | a lane the cockpit can no longer remove because its runner is broken; a broken runner read as nothing to lose |
| `TestRemoveRunsNoHostGitInTheLane` (R4.2) | task 02's `Spy` on the `*gitx.Exec`, over a lane with `fix-x`, a detached HEAD and one uncommitted file: `Inspect` then `Remove` → no recorded spawn has `lane.Path` (or anything under it) as its dir, `-C`, `--git-dir`, `--work-tree` or argv source (the Fake's own git is test-side and unrecorded, so a hostile clone config is task 24's sandbox-side concern, not asserted here); the archive still lands in the project. Red first against the pre-R4 `Inspect`, whose in-clone `for-each-ref` the spy records | host git reading an agent-written config in the clone |
| `TestRemoveWithNothingToLoseNeedsNoToken` | clean, landed lane → 200 without `confirm` | a question with nothing in it |
| `TestRemoveDeletesTheCloneTheLaneRefAndTheIndexEntry` | the directory, `refs/panacea/lanes/<id>` and the store entry are all gone | half a removal |
| `TestRemoveIsRetryable` | `RemoveTree` fails once (a Runner/FS fake) → second DELETE completes | a lane stuck in the index with no directory |
| `TestRemoveDeletesTheConversationStore` | `<ConversationsDir>/local/<id>` existed → gone after removal | OD-10 not honoured |
| `TestRemovePurgesExitedSessionRecords` | an exited session on the worktree → `Store.Get` answers not found | orphan rows the rail cannot place |
| `TestRemoveRefusesPrimaryAndLinked` | a primary or linked worktree id → `not_a_cockpit_worktree`, nothing touched | the cockpit deleting a user's own checkout |
| `TestCreateBranchIsCreateOnly` | an existing name → `branch_exists`, the existing ref unmoved | overwriting a branch |
| `TestCreateBranchRefusesAnIllegalName` | `a..b` → 400 with git's `check-ref-format` answer | a ref git cannot store |
| `TestValidBranchNameRefusesShorthand` (review Minor 6) | the project switched `feature` then `main`. Table: `@{-1}` → 400 `bad_request` with the shorthand sentence (red first against an exit-only `--branch` check, which passes it, then fails `update-ref` as a 500); `-x` and `HEAD` → 400 with git's "not a valid branch name" (red against the `refs/heads/<name>` form, which passes both); `fix-session-timeout` and `a@b` → pass. After the table, `for-each-ref refs/heads` is unchanged | a name git expands to another branch's, or a branch named like an option |
| `TestRenameRecordsTheIdMap` | rename → `GET renames` holds `{from: b-old, to: b-new}`; `BranchID` of the new name equals `toBranchId` | branch-keyed stores seeing a delete plus a create (A8) |
| `TestRenameRefusesWithWorktrees` | a lane on the branch → `branch_has_worktrees` naming it | renaming under a running agent (Q4) |
| `TestRenameRefusesACheckedOutBranch` (review Minor 16) | red first: the project's primary has `old` checked out → 409 `branch_checked_out`, `details.path` is the primary's path; `refs/heads/old` still exists, the primary's `symbolic-ref HEAD` is still `refs/heads/old`, and `GET renames` is empty | `branch -m` moving the user's own checkout to the new name |
| `TestRenameRefusesShorthandForTheNewName` (review Minor 6) | the project switched `feature` then `main`; rename `x` to `@{-1}` → 400 `bad_request`; `x` is not renamed and `feature` is unmoved | a rename that takes another branch's name, because `branch -m` expands `@{-N}` too |
| `TestRenameKeepsTheUpstreamFlag` | a branch with an upstream → `remoteKeepsOldName: true` | a silent divergence from the remote |
| `TestDeleteArchivesTheTip` | after delete, the archive ref names the old tip; `refs/heads/<b>` is gone; `branch.<b>.*` config gone | unpushed commits becoming unreachable (R11) |
| `TestDeleteCallsTheArchiverOnce` | a fake Archiver records `(projectID, branchID)` once; a nil Archiver → `archivedBrief: false` | P13's archive request never sent |
| `TestDeleteRefusesACheckedOutBranch` | → `branch_checked_out` | deleting the branch a checkout is on |
| `TestDeleteABranchWithNoConfigSection` (review Minor 12) | red first against OKExit {0}: a branch `CreateBranch` made (no `branch.<b>` section) → 200, `refs/heads/<b>` gone, the archive ref names its tip | every cockpit-made branch failing its delete as `git_failed` after the ref is already gone |
| `TestDeleteWithALockedConfigStillAnswers` (review Minor 12) | `branch.<b>.remote` set and `.git/config.lock` held → 200, `refs/heads/<b>` gone, the archive ref names the tip, one warn line holding git's sentence, and `branch.<b>.remote` still set | a 500 for a delete that happened, whose retry then finds no branch |
| `TestBranchVerbsWaitOnTheProjectLock` (review I7) | the test holds `Locks.Lock(ctx, p)` → a create, a rename and a delete on `p` run no git call 50 ms later (a recording Runner); release → all three proceed | a rename moving `branch.<name>.*` under a push |
| `TestDeleteLosesARaceCleanly` | the branch moves between read and delete → git refuses; the branch still exists | `update-ref -d` without its old value |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 04, 07, 08 and 24 merged, so `briefref` (task 03), `gitx.Locks`
  (task 02), `GitDeps.Locks` (task 04), `converge.CheckedOutAt` (task 08, which DeleteBranch
  and RenameBranch refuse by; review Minor 16), and `lanegit.Lanes`, `ArchiveLane`, `GitDeps.LaneGit`
  and `lanegittest.Fake` (task 24) exist; nothing here waits on task 09 (review I7).
  Task 01's `TestBehaviourRemoveAllOnAClone`, `TestBehaviourUpdateRefCreateOnlyRefusesAnExistingRef`,
  `TestBehaviourCheckRefFormatBranchExpandsShorthand`, `TestBehaviourRemoveSectionExitCodes` and
  `TestBehaviourBranchRenameMovesACheckoutsHead` read.
- [ ] **Step 2:** tests; `remove.go`, `inspect.go`, `branches.go`; handlers, routes, deps, audit line.
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
