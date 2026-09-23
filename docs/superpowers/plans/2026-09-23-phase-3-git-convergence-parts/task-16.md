### Task 16: The Files read model — changed paths across the branch's worktrees, one path's hunks, and tier-1 back-links

3-D9: the Files lens shows **changed paths only, across all of the branch's worktrees**. Merge
answers "what lands, in what order" per worktree; Files answers "what is changing, and who is
changing it" per path — the same facts on the other axis (survey §8.6). A path two worktrees touch
is marked, and the mark says which kind of fact it is: **"conflicts on landing"** when
`merge-tree` says so (a fact about committed work), and **"both edit this file — not committed,
so not forecast"** when it cannot yet (a suspicion, drawn as one; risk R9). Selecting a path opens
its hunks per worktree, and every hunk links to the commit and session that wrote it (**tier 1**,
survey §8.3), or says why it cannot: uncommitted, or no session recorded. Task 17 adds tier 2.

**Files:**
- `control-plane/internal/changes/changes.go` + test — `Changes`, `PathRow`, `Touch`, `Read` (new)
- `control-plane/internal/changes/diff.go` + test — `PathDiff`, `LaneDiff`, `Hunk`, `Line`, `BackLink`, `Diff` (new)
- `control-plane/internal/changes/blame.go` + test — tier 1 (new)
- `control-plane/internal/changes/parse.go` + test — unified-diff and `blame --porcelain` parsers (new)
- `control-plane/internal/api/changes.go` + test — the two GETs (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go`, `routes_audit_test.go` — `// ── task 16 ──`

#### Interfaces — complete

```go
package changes

// Touch is one worktree's change to one path.
type Touch struct {
	WorktreeID  string `json:"worktreeId"`
	Committed   string `json:"committed"`   // a converge.PathChange status from base..lane, or ""
	Uncommitted string `json:"uncommitted"` // a status from the clone, or ""
}

// PathRow is one changed path.
type PathRow struct {
	Path    string  `json:"path"` // repository-relative, forward slashes
	Dir     string  `json:"dir"`  // its directory, "" at the root — the lens groups by it
	Touches []Touch `json:"touches"`
	// Mark: "conflicts" — merge-tree of two worktrees' committed work conflicts in this path;
	// "both-edit" — two or more worktrees touch it and at least one has not committed, so it
	// cannot be forecast; "" otherwise (one worktree, or overlapping committed work that merges).
	Mark            string      `json:"mark"`
	ConflictBetween [][2]string `json:"conflictBetween"` // worktree id pairs whose merge-tree conflicts here
}

// Unreadable is a worktree whose changes are withheld, with git's sentence (prototype f-gitfail).
type Unreadable struct {
	WorktreeID string `json:"worktreeId"`
	Name       string `json:"name"`
	ReadError  string `json:"readError"`
}

// Changes is GET …/changes's body.
type Changes struct {
	ProjectID  string       `json:"projectId"`
	BranchID   string       `json:"branchId"`
	Branch     string       `json:"branch"`
	Paths      []PathRow    `json:"paths"` // sorted by Path
	Unreadable []Unreadable `json:"unreadable"`
	ReadAt     string       `json:"readAt"`
}

// Read builds Changes from task 07's observations (read by the handler exactly as task 08's
// GET reads them). For every pair of worktrees whose committed paths overlap it runs
//   merge-tree --write-tree --name-only <laneA> <laneB>      (gitx.Local, OKExit 1)
// and marks the overlapping paths merge-tree lists. Cached by the pair's tips.
func Read(ctx context.Context, m gitx.Machine, projectPath string, obs []converge.LaneObservation) (Changes, error)
```

```go
package changes

// Line is one line of a hunk.
type Line struct {
	Kind string `json:"kind"` // " " context, "+" added, "-" removed
	Text string `json:"text"`
}

// BackLink is where a hunk came from, as far as the tiers reach.
type BackLink struct {
	Tier     int      `json:"tier"`     // 0: none; 1: commit and session; 2: the tool call (task 17)
	Commit   string   `json:"commit"`   // tier ≥ 1
	CommitAt string   `json:"commitAt"` // the commit's author date, RFC 3339
	Sessions []string `json:"sessions"` // its Panacea-Session values; [] is "no session recorded"
	// SessionKinds maps each session id to "agent" or "shell" from the registry, so the lens
	// says "shell" rather than implying an agent (survey §5.3 item 4); absent when the registry
	// no longer has the session.
	SessionKinds map[string]string `json:"sessionKinds"`
	// Reason is why the tier stopped where it did: "uncommitted", "no session recorded",
	// "deletion: the newest commit that touched this file", or task 17's adapter reasons.
	Reason string `json:"reason"`
	// Task 17 adds: ToolUseID, ToolName, SpanAt.
}

// Hunk is one @@ block.
type Hunk struct {
	OldStart int      `json:"oldStart"`
	OldLines int      `json:"oldLines"`
	NewStart int      `json:"newStart"`
	NewLines int      `json:"newLines"`
	Header   string   `json:"header"` // the text after the second @@, e.g. a function signature
	Lines    []Line   `json:"lines"`
	Back     BackLink `json:"back"`
}

// LaneDiff is one worktree's change to the path.
type LaneDiff struct {
	WorktreeID  string `json:"worktreeId"`
	Name        string `json:"name"`
	Committed   []Hunk `json:"committed"`   // base..lane
	Uncommitted []Hunk `json:"uncommitted"` // the clone's working tree against its HEAD; Back.Tier 0, Reason "uncommitted"
	Binary      bool   `json:"binary"`
	Truncated   bool   `json:"truncated"` // more than MaxDiffLines across both lists
	ReadError   string `json:"readError"`
}

// PathDiff is GET …/changes/diff's body.
type PathDiff struct {
	Path  string     `json:"path"`
	Lanes []LaneDiff `json:"lanes"` // only the worktrees that touch the path
}

const MaxDiffLines = 2000

// Diff reads one path for each worktree that touches it. All gitx.Read:
//   diff --no-color --no-ext-diff -U3 <base> <laneTip> -- <path>                  (project)
//   diff --no-color --no-ext-diff -U3 HEAD -- <path>                              (the clone)
// An untracked file is read from disk as one all-added hunk (NUL byte in the first 8 KiB → Binary).
// Tier 1 per committed hunk:
//   blame --porcelain -L <newStart>,<newStart+newLines-1> <laneTip> -- <path>
//   → the commit blamed for most "+" lines (ties: newest); its sessions from
//     log -1 --format=<TrailerFormat(Session)> <commit>
// A pure-deletion hunk takes the newest commit in base..laneTip touching the path, Reason
// "deletion: the newest commit that touched this file" (it cannot be blamed on the new side).
func Diff(ctx context.Context, m gitx.Machine, projectPath, path string, obs []converge.LaneObservation, kinds func(sessionID string) (string, bool)) (PathDiff, error)
```

```go
// gitdeps.go
	// ── task 16 ──  (no new dependency: the handlers reuse task 07's Syncer and task 03's Lanes)
```

**Wire:**

```
GET /v1/projects/{projectId}/branches/{branchId}/changes
  200 Changes

GET /v1/projects/{projectId}/branches/{branchId}/changes/diff?path=src%2Fupload%2Fhandler.ts
  200 PathDiff
  400 bad_request  a path that is absolute, contains "..", a backslash, or a NUL
  400 bad_request  "No worktree of this branch changes that path." — the route serves only paths
                   some worktree of the branch changes, so it can never read an arbitrary file
```

```json
{ "path": "src/upload/handler.ts",
  "lanes": [
    { "worktreeId": "w-api", "name": "upload-api", "binary": false, "truncated": false, "readError": "",
      "committed": [
        { "oldStart": 12, "oldLines": 3, "newStart": 12, "newLines": 5, "header": "export async function receiveChunk(req: ChunkRequest) {",
          "lines": [ { "kind": " ", "text": "export async function receiveChunk(req: ChunkRequest) {" },
                     { "kind": "+", "text": "  const resumeFrom = await lastStoredChunk(req.uploadId);" } ],
          "back": { "tier": 1, "commit": "a41c9e2…", "commitAt": "2026-09-23T04:02:00Z",
                    "sessions": ["s-3f2a91c0"], "sessionKinds": { "s-3f2a91c0": "agent" }, "reason": "" } } ],
      "uncommitted": [] } ] }
```

Audit lines for both GETs: *"Reads git (diff, blame, log, status with no optional locks) and
files a worktree changed. merge-tree writes only content-addressed objects reachable from no ref
(plan decision 6). No ref, index, working tree or store changes."*

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestReadListsChangedPathsOnly` | three lanes; unchanged files absent; each path's `Touches` lists exactly the lanes that change it | a full file tree (3-D9 option b) |
| `TestReadMarksACommittedConflict` | two lanes commit conflicting edits to `h.ts` → `Mark "conflicts"`, `ConflictBetween [[a,b]]` | overlap reported as a conflict without merge-tree |
| `TestReadDoesNotMarkCleanOverlap` | two lanes edit different parts of one file, both committed → `Mark ""` | a suspicion drawn as a fact |
| `TestReadMarksUncommittedOverlapAsBothEdit` | lane a committed, lane b uncommitted on `limits.ts` → `Mark "both-edit"` | an uncommitted overlap forecast as clean (R9) |
| `TestReadWithholdsAnUnreadableLane` | one clone gone → its paths absent, `Unreadable` names it with git's sentence; the other lane's paths present | one failure blanking the lens |
| `TestReadCachesPairs` | the second Read with the same tips runs no merge-tree | pairwise merge-tree on every poll |
| `TestDiffParsesHunks` | a real two-hunk diff → both hunks, starts, counts and lines exact | an off-by-one in `@@ -a,b +c,d @@` |
| `TestDiffTier1NamesTheCommitAndSession` | a hunk from a commit carrying `Panacea-Session: s-1` → `Tier 1`, that commit, `["s-1"]`, kind `agent` from the fake registry | a back-link that lands on the top of a session (AGENTIC-UX:74) |
| `TestDiffTier1WithoutATrailer` | a commit with no trailer → `Tier 1`, `sessions []`, `reason "no session recorded"` | inventing a session from time (survey §5.2, rejected) |
| `TestDiffNamesAShellSession` | the trailer names a shell session → `sessionKinds` `shell` | a shell's commit implying an agent |
| `TestDiffBlamesTheMajorityCommit` | a hunk of 3 added lines, two from c2 and one from c1 → c2 | picking the first blamed line |
| `TestDiffPureDeletion` | a hunk of only removals → the newest commit touching the file, the deletion reason | a blame of an empty range erroring |
| `TestDiffUncommittedIsTierZero` | an edit in the clone → in `uncommitted`, `Tier 0`, `reason "uncommitted"`; the clone's index unchanged | an uncommitted hunk claiming a session |
| `TestDiffUntrackedFile` | a new untracked file → one all-added hunk; a file with a NUL byte → `Binary` | untracked files missing from Files |
| `TestDiffTruncates` | 3000 changed lines → 2000 and `Truncated` | an unbounded response |
| `TestDiffRefusesAPathNoWorktreeChanges` | `path=../../etc/hosts`, an absolute path, a backslash → 400; a real unchanged file → 400 with the "No worktree … changes that path" message | the route reading any file |
| `TestChangesGETsAreReadOnly` | the §10a audit passes; `for-each-ref` of the project unchanged | a GET that writes |

#### Steps

- [ ] **Step 1: Preflight.** Task 08 merged (observations reach the handler the same way).
- [ ] **Step 2:** tests; parsers; `changes.go`, `diff.go`, `blame.go`; handlers.
- [ ] **Step 3:** run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/changes/ ./control-plane/internal/api/ -race -count=1 → PASS
```

#### What this task does not claim

- **Tier 1 is advisory where the trailer is** (task 05): a commit the hook did not stamp says "no
  session recorded", in the same words whether it came from outside the cockpit or a bypass.
- A commit in a shared worktree names the session that ran `git commit`, which may not be the one
  whose lines they are (3-D2's note says so on the pane).
- Diff is text only; no syntax highlighting, no word diff.
