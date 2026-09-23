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
- `control-plane/internal/converge/sync.go` — `Syncer`, `SyncState`, `LaneRef` (new)
- `control-plane/internal/converge/observe.go` — `LaneObservation`, `Commit`, `PathChange`, `Observe` (new)
- `control-plane/internal/converge/parse.go` — the `-z` parsers for `log`, `diff --name-status`, `status --porcelain=v2` (new)
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
	// unexported: per-lane mutexes, the state map
}

const (
	DefaultSyncInterval = 5 * time.Second
	MinSyncInterval     = 1 * time.Second
)

// Run sweeps every lane each Interval until ctx is done. A sweep never overlaps another.
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
| `TestObserveRunsNothingTheCloneNames` | task-02 §HGH end to end, red first against an unhardened `Exec`: clone config `core.fsmonitor=<marker script>` → `Observe` writes no marker; then `filter.x.clean=<marker script>` with `.gitattributes` `* filter=x` and a modified file → no marker, and `ReadError` names `clone_config_untrusted` and `filter.x.clean` | an observer built on an `Exec` without §HGH |
| `TestObserveListsCommitsTheBranchLacks` | two lane commits, one carrying `Panacea-Session: s-1` → `Commits` newest first, `Sessions` `[s-1]` and `[]` | reading the branch's own commits as the lane's |
| `TestObserveChangedIsSinceTheMergeBase` | the branch changes `a.txt` after the fork, the lane changes `b.txt` → `Changed` is `[b.txt]` only | `diff branch lane`, which reports the branch's work reversed |
| `TestObserveCapsCommits` | 60 lane commits → 50 and `Truncated` | an unbounded payload on the poll |
| `TestObserveParsesRenamesAndUntracked` | a committed rename → `renamed` with `From`; an untracked file → `untracked` | a `-z` parser that splits a rename wrong |
| `TestObserveReportsAHeadOffTheBranch` (review I6) | in the clone, `switch -c fix-x` and commit → `HeadElsewhere == "refs/heads/fix-x"`, and `Commits` does not hold that commit; `switch --detach` and commit → `HeadElsewhere` is that commit's full sha; `switch <b>` → `""` | a lane card that says "nothing to land" while the agent's work sits on another branch |
| `TestObserveTakesNoLock` | after `Observe`, the clone's `.git/index` bytes and mtime are unchanged | a status that refreshes the index beside an agent |
| `TestObserveNeverSyncedLane` | `synced == false` → `Synced:false`, empty `Tip`, no git call against `LaneRef` | a fatal "unknown revision" shown as an error |
| `TestObserveReadErrorIsThisLanesOnly` | the clone directory is gone → `ReadError` is git's sentence, `Commits` empty | a panic or a whole-request 500 |
| `TestPathsWithNewlinesAndSpaces` | a file named `a b\nc.txt` round-trips through all three parsers | a line-based parser |

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
go test ./control-plane/internal/converge/ -race -count=1   → PASS (18 tests)
go test ./control-plane/... -race -count=1                  → PASS
```

#### What this task does not claim

- No route serves an observation yet; task 08 does.
- A lane's view is up to one sync interval old between sweeps; every write that depends on it
  (land, remove) calls `SyncLane` first.
- Unreachable lane commits (a lane an agent rewrote) are left for `git gc` in the project
  repository. Task 15 pins the commits evidence names, and task 11 archives an unlanded lane
  before removing it; nothing else is kept.
