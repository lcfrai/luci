### Task 07: Lane sync and observed claims — what each worktree is changing, from git alone

Convergence happens in the project repository, so each clone's work has to get there. This task
builds the **lane syncer**: on the control plane's own timer, never behind a request, it copies
each clone's `refs/heads/<branch>` into the project as `refs/panacea/lanes/<worktree-id>`, which
`for-each-ref refs/heads` (`project.go:241`) never lists, so neither the rail nor a terminal's
`git branch` sees lanes. It then builds the **observation** of one lane, the survey's "observed
claim" (§3.5): the commits it has that its branch lacks, the paths those commits change since the
merge base, and the paths changed and not committed, read from the clone without taking a lock.

Nothing here is enforced and nothing asks an agent anything (brief A9). These are readings.

**Files:**
- `control-plane/internal/converge/sync.go` — `Syncer`, `SyncState`, `LaneRef`, `CloneRefresher` (new)
- `control-plane/internal/converge/observe.go` — `LaneObservation`, `Commit`, `PathChange`, `Observe` (new)
- `control-plane/internal/converge/parse.go` — the `-z` parsers for `log`, `diff --name-status`, `status --porcelain=v2`, and `ParseMergeTree`, the one `merge-tree -z` parser tasks 08, 09, 10 and 16 call (review I2) (new)
- `control-plane/internal/converge/*_test.go` (new)
- `control-plane/internal/api/gitdeps.go` — `// ── task 07 ──` block (phase 3's file)
- `control-plane/cmd/api/config.go` — `PANACEA_APP_LANE_SYNC_INTERVAL` (p2)
- `control-plane/cmd/api/main.go` — start and stop the syncer beside the reaper (p2)

#### Interfaces — complete

```go
// Package converge computes, from git alone, how a product branch's worktrees converge, and
// performs the one write that converges them: a land.
package converge

// LaneRef is where a lane's work lives in the project repository.
func LaneRef(worktreeID string) string { return "refs/panacea/lanes/" + worktreeID }

// ProjectPaths resolves a project id to its repository's absolute path. lanes.Catalog's
// Projects satisfies it through a one-line adapter in cmd/api.
type ProjectPaths interface {
	ProjectPath(ctx context.Context, projectID string) (string, error)
}

// SyncState is the last thing the syncer learned about one lane.
type SyncState struct {
	WorktreeID string
	Tip        string    // the clone's refs/heads/<branch> at the last successful sync
	SyncedAt   time.Time // zero until the first success
	// Err is git's sentence for the last failure (the clone's directory is gone, the branch was
	// deleted inside the clone), cleared by the next success.
	Err   string
	ErrAt time.Time
}

// Syncer keeps refs/panacea/lanes/* current. One goroutine sweeps; SyncLane may be called
// concurrently by a handler and serialises per lane.
type Syncer struct {
	Machine  gitx.Machine
	Lanes    lanes.Store
	Projects ProjectPaths
	Interval time.Duration // PANACEA_APP_LANE_SYNC_INTERVAL; default 5s, minimum 1s
	Log      *slog.Logger
	Now      func() time.Time
	// Locks and Refresher drive the sweep's tip-change refresh (review I4; Run, below). Locks is
	// GitDeps.Locks, task 04's one value. Refresher is task 09's *Lander, set by task 09's
	// gitdeps.go block once the Lander exists; until then (tasks 07 and 08) Run skips the
	// refresh and logs that once at warn. The interface is here because task 09 comes later.
	Locks     *gitx.Locks
	Refresher CloneRefresher
	// unexported: per-lane mutexes, the state map; told: worktree id → the origin/<branch> sha
	// that clone was last read back holding (Told, below); failedAt, retryAt and the backoff:
	// worktree id → the tip its last refresh failed at, when the sweep may try it again, and the
	// current wait (Run, c). All memory only, so a restarted control plane refreshes every clone
	// once, on its first sweep.
}

// CloneRefresher is task 09's Lander.RefreshLanes: RefreshClones for the named lanes of the
// branch only, so a sweep never re-fetches a clone that is already fresh (re-review b06-1
// issue 2). The caller holds the project's Locks lock.
type CloneRefresher interface {
	RefreshLanes(ctx context.Context, projectPath, branch string, ids []string) (refreshed []string, errs map[string]string)
}

// Told records that worktreeID's clone now holds received as refs/remotes/origin/<branch>.
// RefreshLanes calls it after each clone's fetch succeeds, with the sha it read back from the
// clone (rev-parse refs/remotes/origin/<branch>, Scope Clone), not the tip it read before the
// fetches. So told is what the clone has, even when the branch moves away and back between the
// two reads (re-review b06-1 issue 5, the ABA case). Told also clears that lane's failure record.
func (s *Syncer) Told(worktreeID, received string)

const (
	DefaultSyncInterval = 5 * time.Second
	MinSyncInterval     = 1 * time.Second
)

// Run sweeps every lane each Interval until ctx is done. A sweep never overlaps another. After
// it syncs the lanes and has released every lane lock (task-02 Locks' lock order), the sweep
// refreshes stale clones (review I4). For each (project, branch)
// that has lanes:
//  a. tip = rev-parse --verify --quiet refs/heads/<branch> in the PROJECT (Read, Scope Project);
//     exit 1 (the branch is gone) → skip, since task 11's branch verbs handle that;
//  b. stale = the lanes of the branch with told != tip, less each lane that is backing off: its
//     failedAt (c) equals tip and its retryAt has not come. stale empty → nothing more: no lock
//     and no git beyond (a);
//  c. else unlock, ok := Locks.TryLock(projectID) (task 02; re-review b06-1 issue 6). !ok → skip
//     the branch this sweep: whoever holds the lock (a land, a push, a fetch) is moving the tip
//     or refreshing, and the next sweep looks again, so a busy project costs the sweep nothing.
//     Then Refresher.RefreshLanes(ctx, path, branch, stale), unlock(). For each id in errs:
//     failedAt[id] = tip and retryAt[id] = now + backoff, where backoff starts at 1 min and
//     doubles on each failure at the same tip, to at most 10 min; the error is logged once per
//     (lane, tip), with the worktree id and git's sentence. A tip move makes the lane stale
//     again at once. So a clone that always fails (its directory gone, or a config the
//     allowlist refuses, which an agent can write on purpose) costs one fetch per tip move or
//     per backoff, never one per sweep, and never a fetch into the healthy clones beside it
//     (re-review b06-1 issue 2).
// This is what catches a branch the cockpit did not move: a user's commit or `git pull` in a
// terminal checkout, or a `git reset` or `commit --amend` there. Without it, a resolve prompt (task 19) tells an
// agent to merge an origin/<branch> the branch has left behind.
func (s *Syncer) Run(ctx context.Context)

// SyncLane syncs one lane now, and is what a land (task 09) and a removal (task 11) call so
// they act on the clone's latest committed work, not on the last sweep's. For each lane:
//  1. `rev-parse --verify --quiet refs/heads/<branch>` in the CLONE (Read);
//  2. `rev-parse --verify --quiet refs/panacea/lanes/<id>` in the PROJECT (Read);
//  3. when they differ, in the PROJECT (Local):
//     fetch --no-tags --no-write-fetch-head --quiet <FetchURL(clone)> +refs/heads/<branch>:refs/panacea/lanes/<id>
// The "+" follows a lane an agent rebased or amended. --no-write-fetch-head keeps FETCH_HEAD
// meaning "the last fetch from origin", which task 08 reads as the fetch time.
// Hardened per task-02 §HGH: step 1 runs with Scope Clone, steps 2 and 3 with Scope Project.
func (s *Syncer) SyncLane(ctx context.Context, worktreeID string) (SyncState, error)

// State is the last sync of one lane; ok is false before the first attempt.
func (s *Syncer) State(worktreeID string) (SyncState, bool)
```

```go
package converge

// Commit is one commit a lane has that its branch lacks.
type Commit struct {
	SHA        string    `json:"sha"`
	Subject    string    `json:"subject"`
	AuthorName string    `json:"authorName"`
	At         time.Time `json:"at"`       // author date
	Sessions   []string  `json:"sessions"` // Panacea-Session values; [] means "no session recorded"
}

// PathChange is one path a lane changes.
type PathChange struct {
	Path string `json:"path"` // repository-relative, forward slashes
	// Status is one of: added, modified, deleted, renamed, typechange, untracked, unmerged.
	Status string `json:"status"`
	From   string `json:"from,omitempty"` // the old path of a rename
}

// LaneObservation is one lane, read from git. Uncommitted work does not land and is listed so
// the row can say so.
type LaneObservation struct {
	WorktreeID  string       `json:"worktreeId"`
	Name        string       `json:"name"`
	Synced      bool         `json:"synced"`   // false until the syncer's first success
	SyncedAt    string       `json:"syncedAt"` // RFC 3339, "" when never
	Tip         string       `json:"tip"`      // refs/panacea/lanes/<id>, "" when never synced
	Base        string       `json:"base"`     // merge-base of the branch and the lane
	Commits     []Commit     `json:"commits"`  // branch..lane, newest first, at most MaxCommits
	Truncated   bool         `json:"truncated"`
	Changed     []PathChange `json:"changed"`     // committed: base..lane
	Uncommitted []PathChange `json:"uncommitted"` // the clone's status
	// HeadElsewhere is where the clone's HEAD is when it is NOT refs/heads/<branch> (review I6):
	// "refs/heads/fix-x" for another branch, the full sha for a detached HEAD, "" when HEAD is on
	// the lane's branch. Only refs/heads/<branch> syncs, so commits made there never reach the
	// lane and never land; the card says so (task 19), and removal archives them (task 11).
	HeadElsewhere string `json:"headElsewhere"`
	// ReadError is git's sentence when this lane could not be read. The other lanes are read
	// regardless (the prototype's m-gitfail state).
	ReadError string `json:"readError"`
}

const MaxCommits = 50

// Observe reads one lane. It never returns an error: a failure is ReadError on the lane,
// because one broken clone must not blank the whole queue. Every call is gitx.Read:
//   log -n 51 --format=%H%x00%s%x00%an%x00%aI%x00<TrailerFormat(Session)>%x1e refs/heads/<b>..<LaneRef>
//   merge-base refs/heads/<b> <LaneRef>
//   diff --name-status -z --find-renames <base> <LaneRef>
//   status --porcelain=v2 -z --untracked-files=all          (in the clone, Scope Clone: task-02 §HGH)
//   symbolic-ref -q HEAD, OKExit 1                           (in the clone, Scope Clone; review I6)
//     exit 0 and not refs/heads/<b> → HeadElsewhere = that ref;
//     exit 1 (detached) → rev-parse --verify -q HEAD in the clone → HeadElsewhere = the sha
// A §HGH refusal is the lane's ReadError, which names clone_config_untrusted and the key.
func Observe(ctx context.Context, m gitx.Machine, projectPath, branch string, lane lanes.Lane, st SyncState, synced bool) LaneObservation

// MergeTreeArgs is the one argument list every merge-tree call in phase 3 starts with (review
// I2): `merge-tree --write-tree --name-only -z`. Task 08 appends `<base> <laneTip>`, task 09
// `<old> <laneTip>`, task 10 `--merge-base=<sha> <tip> <sha>^1`, task 16 `<laneA> <laneB>`.
// Without -z, git C-quotes a path that holds a `"`, a `\`, a control character (a newline, a
// tab) or (under the default core.quotePath=true) any non-ASCII byte; a space alone is not
// quoted (b05-mt-probe.out: `a b.txt` prints bare). So "docs/résumé.md" arrives as
// "docs/r\303\251sum\303\251.md" and never meets the Changed paths the -z diff reads.
var MergeTreeArgs = []string{"merge-tree", "--write-tree", "--name-only", "-z"}

// ParseMergeTree reads `merge-tree --write-tree --name-only -z` stdout (task 01's pin):
// NUL-separated fields; field 0 is the 40-hex tree id; each field after it is one conflicted
// path, unquoted, until the first empty field or the end; everything after that empty field
// is the informational-messages section and is ignored. exit 0 must yield no paths and exit 1
// at least one; any other shape is an error naming the exit code, never a guess.
// Paths are returned sorted and de-duplicated.
func ParseMergeTree(stdout []byte, exitCode int) (tree string, conflicted []string, err error)
```

```go
// gitdeps.go
	// ── task 07 ──
	Syncer *converge.Syncer
```

```go
// cmd/api/config.go — one row: a Go duration, "5s" by default, refused below 1s.
	envLaneSyncInterval = "PANACEA_APP_LANE_SYNC_INTERVAL"
```

`main.go` starts `go syncer.Run(ctx)` after the reaper and waits for it on shutdown the way it
waits for `reaperDone`.

#### The tests to write first

Real git through `gitxtest`: a project repo, `lanes.Creator` clones, commits made in the clone as
an agent would make them. `Script` runners only where the claim is "this call is not made".

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSyncCopiesTheCloneTip` | commit in the clone; `SyncLane` → the project's `refs/panacea/lanes/<id>` equals the clone's `refs/heads/<b>` | a lane that never reaches the project |
| `TestSyncIsANoOpWhenNothingMoved` | the second `SyncLane` issues two `rev-parse`s and no `fetch` (Script runner) | a fetch on every sweep |
| `TestSyncFollowsARewrittenLane` | `commit --amend` in the clone → the lane ref follows (non-fast-forward) | a refspec without `+` |
| `TestSyncNeverWritesFetchHead` | the project's `FETCH_HEAD` is absent after a sync | lane syncs posing as the last remote fetch |
| `TestSyncNeverCreatesAHead` | `for-each-ref refs/heads` in the project is unchanged by a sync | a refspec typo that puts lanes in the rail |
| `TestSyncRecordsAVanishedClone` | delete one clone; a sweep → its `State.Err` is git's sentence; the other lane still syncs | one bad clone stopping the sweep |
| `TestSyncClearsTheErrorOnSuccess` | recreate the branch in the clone → next sweep clears `Err` | a stale error forever on screen |
| `TestSyncLaneSerialisesPerLane` | two concurrent `SyncLane`s on one lane under `-race` → one fetch | two fetches racing on one ref lock |
| `TestRunStopsWhenTheContextEnds` | cancel → `Run` returns within one interval | a goroutine that outlives shutdown |
| `TestSweepAsksForARefreshWhenTheTipMoves` (review I4) | a recording `CloneRefresher` that calls `Told` for each lane; a commit on the project's `refs/heads/<b>` made with plain git, as a terminal would → the next sweep calls `RefreshLanes(path, b, [A, B])` once, holding `Locks` (a `TryLock` by the test from inside the recording refresher is refused) | a sweep that never looks at the branch, so a user's own commit reaches no clone |
| `TestSweepDoesNotRefreshAnUnmovedTip` (review I4) | two more sweeps with the tip unchanged → no further `RefreshLanes` call, and one project `rev-parse` per sweep for the branch (Script runner) | a fetch into every clone every five seconds |
| `TestSweepRetriesAFailedRefresh` (review I4; re-review b06-1 issue 2) | the refresher returns an error for lane B and calls `Told` for A only; a fake `Now` → sweeps before 1 min make no `RefreshLanes` call and no project git call but (a)'s `rev-parse` (Script runner), and log B's error no second time; the first sweep after 1 min calls `RefreshLanes(…, [B])`, B only; B fails again → the next retry waits 2 min; a tip move instead → the next sweep calls `RefreshLanes(…, [A, B])` at once; once B succeeds, no further call | a clone marked fresh after its fetch failed, or never retried || `TestSweepSkipsABusyProjectAndSyncsTheRest` (review I4; re-review b06-1 issue 6) | the test holds projects P's and R's locks; lanes of P, R and Q → Q's lanes sync and Q refreshes within one sweep, P's and R's refreshes are skipped, and the sweep returns in under one interval (a sweep that waited on each busy lock would take two); both are refreshed the sweep after the locks are released | one long push stalling every lane on the machine, or K busy projects making a sweep K intervals long || `TestObserveRunsNothingTheCloneNames` | task-02 §HGH end to end, red first against an unhardened `Exec`: clone config `core.fsmonitor=<marker script>` → `Observe` writes no marker; then `filter.x.clean=<marker script>` with `.gitattributes` `* filter=x` and a modified file → no marker, and `ReadError` names `clone_config_untrusted` and `filter.x.clean` | an observer built on an `Exec` without §HGH |
| `TestObserveListsCommitsTheBranchLacks` | two lane commits, one carrying `Panacea-Session: s-1` → `Commits` newest first, `Sessions` `[s-1]` and `[]` | reading the branch's own commits as the lane's |
| `TestObserveChangedIsSinceTheMergeBase` | the branch changes `a.txt` after the fork, the lane changes `b.txt` → `Changed` is `[b.txt]` only | `diff branch lane`, which reports the branch's work reversed |
| `TestObserveCapsCommits` | 60 lane commits → 50 and `Truncated` | an unbounded payload on the poll |
| `TestObserveParsesRenamesAndUntracked` | a committed rename → `renamed` with `From`; an untracked file → `untracked` | a `-z` parser that splits a rename wrong |
| `TestObserveReportsAHeadOffTheBranch` (review I6) | in the clone, `switch -c fix-x` and commit → `HeadElsewhere == "refs/heads/fix-x"`, and `Commits` does not hold that commit; `switch --detach` and commit → `HeadElsewhere` is that commit's full sha; `switch <b>` → `""` | a lane card that says "nothing to land" while the agent's work sits on another branch |
| `TestObserveTakesNoLock` | after `Observe`, the clone's `.git/index` bytes and mtime are unchanged | a status that refreshes the index beside an agent |
| `TestObserveNeverSyncedLane` | `synced == false` → `Synced:false`, empty `Tip`, no git call against `LaneRef` | a fatal "unknown revision" shown as an error |
| `TestObserveReadErrorIsThisLanesOnly` | the clone directory is gone → `ReadError` is git's sentence, `Commits` empty | a panic or a whole-request 500 |
| `TestPathsWithNewlinesAndSpaces` | a file named `a b\nc.txt` round-trips through all four parsers | a line-based parser |
| `TestParseMergeTreeKeepsQuotedPathsRaw` (review I2) | real git: two branches edit `docs/résumé.md` and `a b\nc.txt` in conflict, the commits built with plumbing (`hash-object -w`, `mktree -z`, `commit-tree`) because NTFS forbids `\n` in a file name; `-z` because plain `mktree` reads newline-terminated entries and refuses this one (`fatal: input format error: c.txt`; `mktree -z` accepts it: git 2.53.0.windows.2, 2026-09-24, `scratchpad/p3/b05r2-mktree-probe.out`); `MergeTreeArgs` + both → exit 1, `ParseMergeTree` returns exactly those two paths, byte-equal to what the `-z` `diff --name-status` parser reads for the same commits; a clean pair → exit 0, a 40-hex tree, no paths | `merge-tree` without `-z`, or a parser that splits on `\n`: the path comes back as `"docs/r\303\251sum\303\251.md"` and meets no `Changed` |
| `TestParseMergeTreeRefusesAMismatchedShape` | exit 1 with no path field, exit 0 with a path field, and a field 0 that is not 40 hex → each an error naming the exit code | a conflict read as clean when git's output changes |

#### Steps

- [ ] **Step 1: Preflight.** Task 01 recorded `TestBehaviourNoWriteFetchHeadLeavesFetchHeadAlone`
  and `TestBehaviourStatusWithoutOptionalLocksLeavesTheIndex` passing. If either diverged, stop:
  the sync refspec or the status read has to change before anything builds on it.
- [ ] **Step 2:** tests, then `parse.go`, `sync.go`, `observe.go`.
- [ ] **Step 3:** the `gitdeps.go` block, config row, and `main.go` start/stop; the Go contract's
  §4 table gains `PANACEA_APP_LANE_SYNC_INTERVAL` (default `5s`, minimum `1s`).
- [ ] **Step 4:** run and commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/converge/ -race -count=1   → PASS (25 tests)
go test ./control-plane/... -race -count=1                  → PASS
```

#### What this task does not claim

- No route serves an observation yet; task 08 does.
- A lane's view is up to one sync interval old between sweeps; every write that depends on it
  (land, remove) calls `SyncLane` first.
- A clone's `origin/<branch>` is likewise up to one interval behind a branch move the cockpit did
  not make (review I4), or up to one backoff (at most 10 min) behind when its last refresh
  failed at this tip (re-review b06-1 issue 2). The cockpit's own moves refresh at once (tasks 09, 10), and
  `ResolveDialog` refreshes before it starts a session (task 19), so no resolve prompt names a
  stale ref. Until task 09 sets `Refresher`, the sweep refreshes nothing.
- Unreachable lane commits (a lane an agent rewrote) are left for `git gc` in the project
  repository. Task 15 pins the commits evidence names, and task 11 archives an unlanded lane
  before removing it; nothing else is kept.
