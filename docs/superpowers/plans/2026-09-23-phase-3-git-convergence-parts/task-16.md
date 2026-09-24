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
- `control-plane/internal/changes/untracked.go` + test — `readUntracked`, `MaxUntrackedBytes`, `openForRead` (new); `finalpath_windows.go` / `finalpath_other.go` — the opened handle's final path (new; review Minor 9)
- `control-plane/go.mod` — `golang.org/x/sys` moves from indirect to direct (same v0.24.0)
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
//   converge.MergeTreeArgs + <laneA> <laneB>   (merge-tree --write-tree --name-only -z;
//                                               gitx.Local, OKExit 1) → converge.ParseMergeTree
// and marks the overlapping paths merge-tree lists, compared byte for byte with the -z
// PathChange paths (review I2: never the C-quoted form). Cached by the pair's tips. It runs in the
// project with Scope Project (task-02 §HGH).
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
	// CommitAt is the commit's committer date (%cI), RFC 3339, not its author date: --amend and
	// rebase keep the author date and move the committer date (probe scratchpad/p3/b11-dates.out),
	// and task 17 matches tool calls at or before this instant (review Minor 11).
	CommitAt string   `json:"commitAt"`
	Sessions []string `json:"sessions"` // its Panacea-Session values; [] is "no session recorded"
	// SessionKinds maps each session id to "agent" or "shell" from the registry, so the lens
	// says "shell" rather than implying an agent (survey §5.3 item 4); absent when the registry
	// no longer has the session.
	SessionKinds map[string]string `json:"sessionKinds"`
	// Reason is why the tier stopped where it did: "uncommitted", "no session recorded",
	// "deletion: the newest commit that touched this file", or task 17's adapter reasons.
	Reason string `json:"reason"`
	// Tier 2, declared here so the wire shape never changes; always "" from this task, and
	// filled by task 17's Linker.
	ToolUseID string `json:"toolUseId"`
	ToolName  string `json:"toolName"`
	SpanAt    string `json:"spanAt"` // the tool call's timestamp, RFC 3339
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
	Truncated   bool   `json:"truncated"` // more than MaxDiffLines across both lists, or an untracked file past MaxUntrackedBytes
	ReadError   string `json:"readError"`
}

// PathDiff is GET …/changes/diff's body.
type PathDiff struct {
	Path  string     `json:"path"`
	Lanes []LaneDiff `json:"lanes"` // only the worktrees that touch the path
}

const MaxDiffLines = 2000

// Diff reads one path for each worktree that touches it. All gitx.Read, with Scope Project in
// the project and Scope Clone in the clone (task-02 §HGH; blame and log run in the project):
//   diff --no-color --no-ext-diff --no-textconv -U3 <base> <laneTip> -- <path>    (project)
//   diff --no-color --no-ext-diff --no-textconv -U3 HEAD -- <path>                (the clone)
// --no-textconv is on every diff and blame here (review Minor 9, the C1 class). Both run a
// textconv driver by default, and a .gitattributes the agent wrote can name one the host
// defines: this machine's system config holds diff.astextplain.textconv. Probe (git 2.53,
// scratchpad/p3/b11-probe.out): a marker driver ran on diff and on blame, and on neither with
// the flag. H1 refuses an agent-written diff.* key; the flag stops a host-defined driver.
// An untracked file is never read through git: readUntracked (below) reads it as one all-added
// hunk (NUL byte in the first 8 KiB → Binary).
// Tier 1 per committed hunk:
//   blame --porcelain --no-textconv -L <newStart>,<newStart+newLines-1> <laneTip> -- <path>
//   → the commit blamed for most "+" lines (ties: newest); its sessions from
//     log -1 --format=%cI%x00<TrailerFormat(Session)> <commit>   (%cI is BackLink.CommitAt)
// A pure-deletion hunk takes the newest commit in base..laneTip touching the path, Reason
// "deletion: the newest commit that touched this file" (it cannot be blamed on the new side).
// The path is always the argument (a PathRow.Path, raw -z bytes). The parsers never read a
// path back from a `diff --git`, `---`/`+++` or porcelain `filename` line, because git C-quotes
// those the way it quotes merge-tree without -z (review I2). Every `-- <path>` above is matched
// literally, never as a glob, by task-02 §HGH H1's --literal-pathspecs (re-review b05 issue 9);
// test `TestDiffBracketedPathIsLiteral` below.
func Diff(ctx context.Context, m gitx.Machine, projectPath, path string, obs []converge.LaneObservation, kinds func(sessionID string) (string, bool)) (PathDiff, error)

// MaxUntrackedBytes caps what readUntracked reads of one file (review Minor 9).
const MaxUntrackedBytes = 1 << 20

// readUntracked reads <cloneTop>/<path> for an untracked row. The clone is mounted into a live
// sandbox, so a check on the name alone loses a race to the agent (the A4 class, re-review b03
// I-2), and every check below that matters is made on the file actually opened:
//   1. os.Lstat of every component from cloneTop to <path>: each a real directory and <path> a
//      regular file, by task-03 CopyGeneratedHooks' rule (no symlink, no junction or other
//      reparse point; under `go 1.23` a junction reports ModeIrregular and not IsDir, a
//      symlink ModeSymlink: probe scratchpad/p3/b11-go.out, Go 1.26.2 toolchain).
//   2. open through the unexported seam openForRead (default os.Open); f.Stat() must be a
//      regular file.
//   3. the opened handle's final path must lie under cloneTop's final path, both read the same
//      way (Windows: GetFinalPathNameByHandle, golang.org/x/sys/windows, already in go.mod as
//      indirect v0.24.0; elsewhere: readlink of /proc/self/fd/<n>), compared case-insensitively
//      on Windows with a separator after the prefix. Step 3, not step 1, is what defeats a parent
//      folder swapped for a junction between the check and the open. os.SameFile cannot: on
//      Windows it loads an Lstat's file id lazily, by path, at compare time (GOROOT
//      src/os/types_windows.go loadFileId), so it compares the swapped-in target with itself.
//      Probed (b11-go.out): Lstat p/f, p swapped for a junction to an outside folder, open p/f
//      → the outside file's bytes, and SameFile(Lstat, f.Stat()) = true.
//   4. read through io.LimitReader(f, MaxUntrackedBytes+1), so a 2 GB untracked file costs
//      1 MiB, never the whole file. Past the cap → Truncated, the text cut at its last whole line.
// A refusal at 1 to 3 → no hunk, LaneDiff.ReadError "<path> is a link or not a regular file in
// this worktree; not read". Nothing is ever read outside the clone.
func readUntracked(cloneTop, path string) (lines []Line, binary, truncated bool, err error)
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
| `TestReadMarksAConflictOnANonASCIIPath` (review I2) | the same with `docs/résumé.md` → that row's `Mark "conflicts"`; no second row appears under the quoted name | `merge-tree` without `-z`: the quoted path matches no row, and the conflict reads as "merges" |
| `TestReadDoesNotMarkCleanOverlap` | two lanes edit different parts of one file, both committed → `Mark ""` | a suspicion drawn as a fact |
| `TestReadMarksUncommittedOverlapAsBothEdit` | lane a committed, lane b uncommitted on `limits.ts` → `Mark "both-edit"` | an uncommitted overlap forecast as clean (R9) |
| `TestReadWithholdsAnUnreadableLane` | one clone gone → its paths absent, `Unreadable` names it with git's sentence; the other lane's paths present | one failure blanking the lens |
| `TestReadCachesPairs` | the second Read with the same tips runs no merge-tree | pairwise merge-tree on every poll |
| `TestDiffParsesHunks` | a real two-hunk diff → both hunks, starts, counts and lines exact | an off-by-one in `@@ -a,b +c,d @@` |
| `TestDiffTier1NamesTheCommitAndSession` | a hunk from a commit carrying `Panacea-Session: s-1` → `Tier 1`, that commit, `["s-1"]`, kind `agent` from the fake registry | a back-link that lands on the top of a session (AGENTIC-UX:74) |
| `TestDiffBracketedPathIsLiteral` (re-review b05 issue 9) | one lane edits `app/[slug]/page.tsx` and `app/s/page.tsx` in one commit, one line each → `Diff` of `app/[slug]/page.tsx` has exactly 1 hunk, with that file's line; a pure deletion in `app/[slug]/page.tsx` names only commits touching it | the pathspec read as a glob: 2 hunks, the second file's back-links filed under this path |
| `TestDiffTier1WithoutATrailer` | a commit with no trailer → `Tier 1`, `sessions []`, `reason "no session recorded"` | inventing a session from time (survey §5.2, rejected) |
| `TestDiffCommitAtIsTheCommitterDate` (review Minor 11) | a commit made with `GIT_AUTHOR_DATE`=`GIT_COMMITTER_DATE`=T0, then `commit --amend` with `GIT_COMMITTER_DATE`=T1 → the hunk's `commitAt` is T1 | the author date, which amend and rebase keep |
| `TestDiffNamesAShellSession` | the trailer names a shell session → `sessionKinds` `shell` | a shell's commit implying an agent |
| `TestDiffBlamesTheMajorityCommit` | a hunk of 3 added lines, two from c2 and one from c1 → c2 | picking the first blamed line |
| `TestDiffPureDeletion` | a hunk of only removals → the newest commit touching the file, the deletion reason | a blame of an empty range erroring |
| `TestDiffUncommittedIsTierZero` | an edit in the clone → in `uncommitted`, `Tier 0`, `reason "uncommitted"`; the clone's index unchanged | an uncommitted hunk claiming a session |
| `TestDiffUntrackedFile` | a new untracked file → one all-added hunk; a file with a NUL byte → `Binary` | untracked files missing from Files |
| `TestDiffUntrackedRefusesALink` (review Minor 9) | `Diff` given a LaneObservation that names each path below as untracked (so git's own listing is not what refuses it); in the clone, `s.txt` a symlink to a file outside the clone, and `j/f.txt` under `j` a junction (`mklink /J`) to an outside folder → each `ReadError` names the path, no hunk, and no byte of either outside file is in the body; `t.txt` a regular file → read | an untracked file read by name, following a link the agent made |
| `TestDiffUntrackedChecksTheOpenedFile` (review Minor 9) | through the seam `openForRead`: a hook that, after step 1's Lstat walk, renames `d` aside and makes `d` a junction to an outside folder holding `f.txt`, then opens → `ReadError`, no outside byte. Red first with step 3 removed and step 2 checked by `os.SameFile` instead (it passes on Windows: `b11-go.out`) | a check on the name the agent swaps between check and open |
| `TestDiffUntrackedCapsTheRead` (review Minor 9) | a 3 MiB untracked text file, read through `openForRead` wrapped in a counting reader → at most `MaxUntrackedBytes+1` bytes read, `Truncated`, the last line whole | the whole file read, then cut |
| `TestDiffRunsNoTextconv` (review Minor 9) | a test system file holding `diff.x.textconv=<marker script>`, the lane's `.gitattributes` holding `*.txt diff=x`, a committed and an uncommitted edit to `a.txt` → `Diff` returns both, blame included, and no marker exists. Red first without the flag on each of the three calls | a host-defined textconv driver the agent's attributes name, run on the host |
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
- A lane that merged `upstream/<branch>` to resolve a bring-in conflict (task 19) carries the
  remote's files in its `Changed` until it lands, so the Files lens can mark a path only a
  teammate touched as "both edit this file" against another lane (re-review b05 issue 7). The
  pairing is advisory; the forecast and the land are exact. After the land the pairing clears.
- `--no-textconv` stops textconv only. The clone's `diff HEAD` and `status` hash the working tree
  through any **clean** filter the host's system or global config defines, when the agent's
  `.gitattributes` names it (review b08, "Observed, not in this batch"). Whether every Scope Clone
  call carries a no-filter env belongs to task-02 §HGH, which R2 keeps in one place; it is raised
  to the orchestrator and is not changed here. This machine's system and global configs define no
  `filter.*` (task-03's LFS note).
- An untracked file past `MaxUntrackedBytes` shows its first 1 MiB of lines, marked `Truncated`.
