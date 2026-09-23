### Task 08: The chained forecast, the queue order, the remote reading, and the convergence GET

This is the Merge lens's read model. It answers, for one product branch: which worktrees are
marked ready and in what order the human wants them landed; for each, whether it **lands clean
against the branch as it will be after the rows above it land** (`git merge-tree --write-tree`,
a fact, survey §3.3) or conflicts, in which paths, and because of which row above; what every
worktree of the branch is changing (task 07); whether the branch is checked out somewhere (which
turns landing off, brief A5); and where the branch stands against its remote — ahead, behind,
never pushed, and when it was last fetched (obligation 6; phase 7's seam S3-P7.3).

**Files:**
- `control-plane/internal/converge/forecast.go` — `Forecast`, `ForecastQueue`, the fixed-identity forecast commit, the cache (new)
- `control-plane/internal/converge/convergence.go` — `Convergence`, `QueueEntry`, `LaneView`, `Read` (new)
- `control-plane/internal/converge/checkedout.go` — `CheckedOutAt` (new)
- `control-plane/internal/remote/state.go` — `State`, `ReadState` (new)
- tests for each (new)
- `control-plane/internal/api/convergence.go` + test — GET convergence, PUT queue (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go` — `// ── task 08 ──` blocks
- `control-plane/internal/api/routes_audit_test.go` — one line (p2, appended)

#### Interfaces — complete

```go
package converge

// Forecast is one queued worktree's landing, computed against the branch as it will be once
// the clean rows above it have landed.
type Forecast struct {
	// State: "clean" | "conflicts" | "nothing" (no commits the branch lacks) |
	// "unknown" (not synced yet, or git could not answer — Why says which).
	State string `json:"state"`
	// After is the worktree ids assumed landed first: the clean rows above, in order.
	After []string `json:"after"`
	// ConflictsWith is the rows above whose committed paths meet Paths. Empty with State
	// "conflicts" means the worktree conflicts with the branch itself.
	ConflictsWith []string `json:"conflictsWith"`
	Paths         []string `json:"paths"` // conflicted paths, sorted
	Why           string   `json:"why"`
	// ForecastCommit is the deterministic forecast commit a clean row produced: the branch as
	// it would be after this row lands. Internal; task 15 evaluates evidence at it.
	ForecastCommit string `json:"-"`
}

// ForecastQueue walks the queue in order:
//
//	base := branch tip
//	for each lane: unsynced → unknown; lane tip an ancestor of base → nothing;
//	  merge-tree --write-tree --name-only base laneTip   (OKExit 1; gitx.Local)
//	  clean     → clean, After = assumed; base = forecastCommit(tree, base, laneTip); assumed += lane
//	  conflicts → conflicts, Paths; ConflictsWith = assumed lanes whose observation's Changed
//	              meets Paths; base does not advance (the rows below assume this one does not land)
//
// forecastCommit is `commit-tree <tree> -p <base> -p <laneTip> -m "panacea forecast"` with
// GIT_AUTHOR_NAME/COMMITTER_NAME "panacea-forecast", EMAIL "forecast@panacea.invalid" and
// DATE "@0 +0000", so the same inputs always produce the same id (plan decision 6; task 01's
// TestBehaviourFixedDateCommitTreeIsDeterministic). It is reachable from no ref.
//
// Results are cached in memory keyed by (branch tip, the ordered lane tips); a repeat of the
// same key runs no git at all. Every call runs in the project with Scope Project (task-02 §HGH).
func ForecastQueue(ctx context.Context, m gitx.Machine, projectPath, branchTip string, queue []LaneObservation) []Forecast

// CheckedOutAt returns the path of a worktree of the project repository that has refs/heads/<branch>
// checked out, or "" when none does. Read: `worktree list --porcelain` in the project, Scope
// Project (task-02 §HGH).
func CheckedOutAt(ctx context.Context, m gitx.Machine, projectPath, branch string) (string, error)
```

```go
package converge

// LaneSession is a live session on a lane, from the registry, so the lane card can say who is
// working there and 3-D2's warning can name them.
type LaneSession struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Harness string `json:"harness"`
	Status  string `json:"status"` // never "exited": exited sessions are not listed
}

// LaneView is one worktree of the branch: its observation plus two facts git cannot hold.
type LaneView struct {
	LaneObservation
	Ready    bool          `json:"ready"`    // registry.WorktreeFlags.Ready (seam S3.3)
	Sessions []LaneSession `json:"sessions"` // live sessions on it, oldest first
}

// QueueEntry is one ready worktree in landing order.
type QueueEntry struct {
	WorktreeID string   `json:"worktreeId"`
	Forecast   Forecast `json:"forecast"`
	// Settles is the criterion ids that are not settled at the branch tip and would be once
	// this row lands. Task 15 fills it when the GET names a briefKey and a resolver exists;
	// always [] from this task.
	Settles []string `json:"settles"`
}

// Convergence is GET …/convergence's body.
type Convergence struct {
	ProjectID    string       `json:"projectId"`
	BranchID     string       `json:"branchId"`
	Branch       string       `json:"branch"`
	Tip          string       `json:"tip"`          // refs/heads/<branch>; a land names it as expectedTip
	CheckedOutAt string       `json:"checkedOutAt"` // "" or the checkout's path; landing is off when set
	Remote       remote.State `json:"remote"`
	Queue        []QueueEntry `json:"queue"` // ready lanes: stored order first, then unordered ready lanes by CreatedAt
	Lanes        []LaneView   `json:"lanes"` // every cockpit worktree of the branch, CreatedAt order
	ReadAt       string       `json:"readAt"`
}

// Inputs is what Read needs; the handler fills it.
type Inputs struct {
	Machine     gitx.Machine
	Syncer      *Syncer
	Lanes       lanes.Store
	Flags       map[string]registry.WorktreeFlags
	Sessions    []registry.Session
	ProjectID   string
	ProjectPath string
	BranchID    string
	Branch      string
	Now         time.Time
}

// Read assembles Convergence. It errors only when the branch itself cannot be resolved;
// every per-lane failure is that lane's ReadError, and a remote failure is Remote.ReadError.
func Read(ctx context.Context, in Inputs) (Convergence, error)
```

```go
package remote

// State is a product branch against its remote, as of the last fetch.
type State struct {
	Remote    string `json:"remote"`    // "origin"; "" when the project has no origin
	Upstream  string `json:"upstream"`  // e.g. "origin/evidence-upload"; "" means never pushed
	Ahead     int    `json:"ahead"`     // 0 when Upstream is ""
	Behind    int    `json:"behind"`
	FetchedAt string `json:"fetchedAt"` // FETCH_HEAD's mtime, RFC 3339; "" when never fetched
	// FetchError is git's sentence for the most recent fetch that failed, from task 12's
	// Fetcher (its memory, not git); "" when the last fetch succeeded or none has run. Task 08
	// always writes ""; task 12 fills it.
	FetchError string `json:"fetchError"`
	ReadError  string `json:"readError"`
}

// ReadState, all gitx.Read in the project with Scope Project (task-02 §HGH):
//   remote get-url origin                                         (absent → Remote "")
//   rev-parse --abbrev-ref --symbolic-full-name <branch>@{upstream} (fails → never pushed)
//   rev-list --left-right --count refs/heads/<branch>...<branch>@{upstream}
//   rev-parse --git-path FETCH_HEAD, then os.Stat for its mtime
// It never fetches: fetching is task 12's, on demand or on its slow timer, never on a GET.
func ReadState(ctx context.Context, m gitx.Machine, projectPath, branch string) State
```

**Wire:**

```
GET /v1/projects/{projectId}/branches/{branchId}/convergence[?briefKey=<key>]
  200 → Convergence (above). briefKey is ignored by this task; task 15 uses it for Settles.
  404 project_not_found; 400 bad_request "No branch with id …" ; 409 git_too_old

PUT /v1/projects/{projectId}/branches/{branchId}/queue
  request  { "order": ["w-1a2b3c4d", "w-5e6f7a8b"] }
  200      { "order": [...] }   — as stored
  400 bad_request  an id that is not a cockpit worktree of this branch, or a duplicate
```

```json
{
  "projectId": "p-8c1d0e42", "branchId": "b-…", "branch": "evidence-upload",
  "tip": "a90d3f4c…", "checkedOutAt": "",
  "remote": { "remote": "origin", "upstream": "origin/evidence-upload", "ahead": 0, "behind": 0,
              "fetchedAt": "2026-09-23T05:36:00Z", "fetchError": "", "readError": "" },
  "queue": [
    { "worktreeId": "w-ui", "forecast": { "state": "clean", "after": [], "conflictsWith": [], "paths": [], "why": "" },
      "settles": [] },
    { "worktreeId": "w-api", "forecast": { "state": "conflicts", "after": ["w-ui"], "conflictsWith": ["w-ui"],
                                           "paths": ["src/upload/handler.ts"], "why": "" }, "settles": [] }
  ],
  "lanes": [
    { "worktreeId": "w-api", "name": "upload-api", "synced": true, "syncedAt": "…", "tip": "…", "base": "…",
      "commits": [{ "sha": "…", "subject": "Resume from the last stored chunk", "authorName": "…",
                    "at": "…", "sessions": ["s-3f2a91c0"] }],
      "truncated": false, "changed": [{ "path": "src/upload/handler.ts", "status": "modified" }],
      "uncommitted": [], "readError": "", "ready": true,
      "sessions": [{ "id": "s-3f2a91c0", "title": "…", "harness": "claude-code", "status": "running" },
                   { "id": "s-77aa0011", "title": "…", "harness": "shell", "status": "idle" }] }
  ],
  "readAt": "2026-09-23T05:40:02Z"
}
```

The audit line for the GET, in `catalogReadOnlyRoutes`:
*"Reads git (log, diff, merge-base, status with no optional locks, worktree list, rev-list), the
lane index, the flag snapshot and the registry. merge-tree and the forecast commit-tree write only
content-addressed objects reachable from no ref, and forecast commits use a fixed identity and
date, so a repeated GET writes nothing new (plan decision 6). No ref, index, working tree or store
changes."* The PUT is a write and needs no line.

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestForecastTwoCleanLanesChain` | lanes a, b touch different files → both clean; b's `After == [a]` | forecasting every row against the bare tip |
| `TestForecastSecondConflictsWithTheFirst` | a and b edit the same line → a clean, b conflicts, `Paths == [f]`, `ConflictsWith == [a]` | "conflicts" without the row that causes it |
| `TestForecastConflictWithTheBranchItself` | the branch and lane edit the same line → conflicts, `ConflictsWith == []` | blaming a row above for the branch's own change |
| `TestForecastAConflictDoesNotAdvanceTheBase` | a conflicts, b depends only on the tip → b clean with `After == []` | rows below assuming a conflicting row landed |
| `TestForecastNothingToLand` | lane tip is an ancestor of the tip → `nothing` | offering Land for an empty lane |
| `TestForecastUnsyncedIsUnknown` | a lane never synced → `unknown`, `Why` non-empty, no git call for it | a fatal shown as a conflict |
| `TestForecastIsDeterministic` | two calls with identical inputs → identical results, and the project's loose-object count is unchanged by the second | decision 6: a GET that grows the object store every poll |
| `TestForecastCacheSkipsGit` | the second call with the same key issues no git call (Script runner) | recomputing merge-tree every 2 s |
| `TestCheckedOutAt` | the project's primary has the branch checked out → its path; another branch → `""` | landing into a checked-out branch |
| `TestReadStateNeverPushed` | no upstream → `Upstream ""`, `Ahead 0`, no `rev-list` call | "0 ahead" for a branch that was never pushed |
| `TestReadStateAheadBehind` | a bare "remote" repo, the branch one commit ahead → `Ahead 1, Behind 0` | swapping the two numbers |
| `TestReadStateFetchedAtIsFetchHeadsMtime` | after a real `fetch origin` → `FetchedAt` within a second of now; a lane sync does not change it | the time of the last lane sync shown as a fetch |
| `TestReadStateNoOrigin` | no `origin` → `Remote ""`, no error | a 500 for a local-only project |
| `TestReadPutsReadyLanesInTheStoredOrder` | stored order [b, a], both ready, c ready but unordered → queue [b, a, c] | the queue ignoring the human's order |
| `TestReadLeavesNotReadyLanesOutOfTheQueue` | a not ready → in `lanes`, absent from `queue` | inferring readiness (known defect 9) |
| `TestReadListsOnlyLiveSessions` | an exited session on the lane → absent from `sessions` | a shared-index warning for a finished session |
| `TestConvergenceGETIsOnTheReadOnlyAllowlist` | the §10a audit passes with the new pattern listed | a GET added without its reason |
| `TestConvergenceGETChangesNoRef` | `for-each-ref` of the project before and after a GET → identical | a GET that fetches or moves a ref |
| `TestPutQueueRefusesAForeignID` | an id from another branch → 400; duplicate → 400; the stored order unchanged | a queue naming worktrees it cannot land |
| `TestPutQueueRoundTrips` | PUT [b, a] then GET → queue [b, a] | the order not reaching the forecast |

#### Steps

- [ ] **Step 1: Preflight.** Task 07 merged; task 01's merge-tree format and exit-code pins read.
- [ ] **Step 2:** tests; then `remote/state.go`, `checkedout.go`, `forecast.go`, `convergence.go`.
- [ ] **Step 3:** handler, routes, deps block, audit line.
- [ ] **Step 4:** run and commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/converge/ ./control-plane/internal/remote/ ./control-plane/internal/api/ -race -count=1 → PASS
curl -s http://127.0.0.1:8730/v1/projects/<p>/branches/<b>/convergence   → the shape above (manual, owner's laptop; recorded or "not run")
```

#### What this task does not claim

- **Uncommitted overlap is not forecast** (risk R9): merge-tree sees commits only. Files (task
  16) labels it "both edit this file — not committed, so not forecast".
- The forecast is exact for committed work **as of the last sync**; a land re-syncs and
  re-forecasts before it writes anything (task 09).
- `Ahead`/`Behind` are as of `FetchedAt`, and the lens says so.
