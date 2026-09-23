### Task 09: `contextwrite` — the Materialiser

Satisfies task 10's `api.Materialiser`. Given a worktree about to start a session, it asks
`discoverhub` what the session inherits (P4-A), renders that one key, and writes it into the file
Claude Code reads — **atomically, before the process starts, never committed** — then returns the
key it wrote. It also answers the drift check (prototype state 11). **Kept from the 2026-09-18
plan's task 5** (`:2120-2474`): the package comment's ordering rule, absolute-path and
existing-directory checks, `Merge` preserving user text, no temp file left behind. **Changed:** it
is called by the session handlers, not by the desktop; it chooses its target (P4-D); it writes the
exclude line (phase 3 ask P4-6); it strips when nothing is inherited; it uses
`statepath.WriteAtomic` with `Shared` (task 01) instead of its own copy.

**Depends on** tasks 01, 05, 07 and **10** (it imports `api` for the request, result and error
types; `api` imports nothing of it).

**Files:**
- `control-plane/internal/contextwrite/materialise.go` + `materialise_test.go` (new)
- `control-plane/internal/contextwrite/inspect.go` + `inspect_test.go` (new)
- `control-plane/internal/contextwrite/git.go` — the `Git` interface (new)

**Interfaces — complete source:**

```go
// control-plane/internal/contextwrite/materialise.go

// Package contextwrite puts a session's inherited brief into its worktree.
//
// THE ORDERING RULE: the file must be on disk before the harness process
// starts. api.handleSessionCreate and handleSessionRestart call Materialise
// before anything is recorded or created (ruling R1). Called afterwards, the
// session inherits nothing, silently, and the feature looks built.
//
// THE COMMIT RULE: the managed block is never committed (phase 3's 3-D6). It
// is written only to a file the repository does not track, and that file's
// path is added to the clone's info/exclude.
package contextwrite

const (
	FileName    = "CLAUDE.md"         // the name is the contract with Claude Code; not configurable
	AltFileName = ".claude/CLAUDE.md" // P4-D, when CLAUDE.md is tracked — UNVERIFIED that the CLI reads it (task 23)
)

// Source is discoverhub.Service's Inheritable.
type Source interface {
	Inheritable(ctx context.Context, t discoverhub.Target) (discoverhub.Inheritable, error)
}

type Materialiser struct{ /* src, git, log, a mutex per worktree path */ }

func New(src Source, git Git, log *slog.Logger) *Materialiser

var _ api.Materialiser = (*Materialiser)(nil)

// Materialise, in order:
//  1. WorktreePath must be absolute and an existing directory, else a plain
//     error (500: the handler resolved it a moment ago).
//  2. Choose the target: FileName if git does not track it; else
//     AltFileName if git does not track that; else *api.MaterialiseError
//     tracked_elsewhere. A directory git does not manage (Git reports
//     ErrNotARepo) gets FileName and no exclude line.
//  3. WithoutBrief: strip any managed block from BOTH candidate files and
//     return "none:declined". A strip that fails is write_failed: a session
//     labelled "started without the brief" must not find an old brief.
//  4. Otherwise ask Source. hubclient.ErrUnavailable → hub_unavailable;
//     entraauth.ErrSignedOut, or a relayed 401 → signed_out;
//     ErrHubOff, ErrNotBound, ErrNoRemote, or a Hub 404 → nothing to
//     inherit. Any other error → a plain error.
//  5. Nothing to inherit: strip both candidates; return "".
//     Something: Merge(existing, Render(doc)) into the target and strip the
//     other candidate, so at most one managed block exists across the two.
//     claudemd.ErrUnterminatedBlock / ErrDuplicateBlock → block_broken.
//  6. Write only if the bytes changed, with statepath.WriteAtomic(..., Shared).
//     A file left holding only whitespace after a strip is removed.
//     Any write error → write_failed.
//  7. On the first write to a target, append "/CLAUDE.md" or
//     "/.claude/CLAUDE.md" to the path Git.ExcludePath returns, if absent.
//  8. Return the key written.
// Calls for one worktree path are serialised; different worktrees run in
// parallel.
func (m *Materialiser) Materialise(ctx context.Context, req api.MaterialiseRequest) (api.MaterialiseResult, error)
```

```go
// control-plane/internal/contextwrite/git.go
package contextwrite

// Git is the two read-only questions this package asks git. Task 14 wires it
// over project.Runner (phase 2's read-only runner):
//   Tracked:     git -C <dir> ls-files --error-unmatch -- <rel>   (exit 1 = untracked)
//   ExcludePath: git -C <dir> rev-parse --git-path info/exclude  (joined to dir if relative)
// It never writes through git. The exclude FILE is written by this package
// with statepath.WriteAtomic, which is a file write, not a git command.
type Git interface {
	Tracked(ctx context.Context, dir, rel string) (bool, error)
	ExcludePath(ctx context.Context, dir string) (string, error)
}

var ErrNotARepo = errors.New("contextwrite: not inside a git repository")
```

```go
// control-plane/internal/contextwrite/inspect.go
package contextwrite

type WorktreeState string

const (
	StateAbsent     WorktreeState = "absent"     // no block, and nothing to inherit
	StatePending    WorktreeState = "pending"    // no block yet; the next session start writes one
	StateCurrent    WorktreeState = "current"    // the block is a byte-exact render of the key a start would write now
	StateSuperseded WorktreeState = "superseded" // the block names an older key; sessions already running read it
	StateEdited     WorktreeState = "edited"     // the marker names the right key, the text is not its render (state 11)
	StateStale      WorktreeState = "stale"      // a block, but nothing should be inherited now
	StateBroken     WorktreeState = "broken"     // unterminated or duplicated
)

type Inspection struct {
	WorktreeID  string              `json:"worktreeId"`
	Path        string              `json:"path"`        // the file inspected, absolute
	State       WorktreeState       `json:"state"`
	MarkerKey   string              `json:"markerKey"`   // "" when there is no block
	ExpectedKey string              `json:"expectedKey"` // "" when nothing should be inherited
	Diff        []claudemd.DiffLine `json:"diff"`        // StateEdited only; [] otherwise
}

// Inspect reads, never writes. It is the ONLY place brief text is read back
// out of a worktree, and it reads it only to compare (contract §3.3).
func (m *Materialiser) Inspect(ctx context.Context, req api.MaterialiseRequest) (Inspection, error)
```

**Tests first** (a fake `Source`, a fake `Git`, `t.TempDir()` worktrees):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWritesTheInheritedKeyAndReturnsIt` | `CLAUDE.md` holds `Render(doc)`; result is the key | a key returned for a file not written |
| `TestPreservesUserTextAroundTheBlock` | a hand-written CLAUDE.md keeps every byte outside the block | the old plan's defect class, reintroduced by a rewrite |
| `TestTrackedClaudeMdFallsBackToTheAltFile` | `Tracked(CLAUDE.md)=true` → block in `.claude/CLAUDE.md`; `CLAUDE.md` untouched | a tracked file modified — an agent's `git add -A` commits the brief (3-D6) |
| `TestBothTrackedIsRefused` | `*api.MaterialiseError{tracked_elsewhere}`; nothing written | a write into a tracked file as a last resort |
| `TestExcludeLineIsAddedOnce` | after three materialises, `info/exclude` has exactly one `/CLAUDE.md` line and its earlier lines intact | an exclude that grows each start |
| `TestNothingToInheritStripsAnOldBlock` | an old block and nothing inheritable → block removed, result `""` | a chip saying *no brief* over a file that still holds one |
| `TestAFileHoldingOnlyTheBlockIsRemovedOnStrip` | the file does not exist afterwards | an empty CLAUDE.md left in every worktree |
| `TestWithoutBriefStripsAndDeclines` | both candidates stripped; result `none:declined`; `Source` never called | the override blocked by the Hub being down, the very case it exists for |
| `TestHubUnavailableIsARefusal` | `ErrUnavailable` → `hub_unavailable`; the file untouched | a silent `""` when a brief may exist — 4-D1's whole point |
| `TestSignedOutIsARefusal` | `ErrSignedOut` → `signed_out` | as above |
| `TestHubOffAndUnboundInheritNothing` | `ErrHubOff`, `ErrNotBound`, `ErrNoRemote` → `""`, no refusal | Develop blocked for everyone without a Hub (Hub contract §9.5: nothing blocks Develop) |
| `TestUnterminatedBlockIsBlockBroken` | `block_broken`; the file byte-identical | the guess that deletes a person's writing |
| `TestUnchangedBytesAreNotRewritten` | the file's mtime does not move on a second identical materialise | a rewrite per start — noise in every editor watching the file |
| `TestFileIsWorldReadable` (skipped on Windows) | mode `0o644` | the registry's `0o600` making the brief unreadable to the sandbox user (task 01) |
| `TestConcurrentStartsOnOneWorktreeSerialise` | two goroutines, two keys: the file ends as one complete render of one of them and `Merge` never sees a half-written file | interleaved writes |
| `TestInspectStates` — table | each of the seven states from a constructed file and expected key; `edited` carries a diff with the changed line | a drift check that only compares keys (brief §4.2 gap 3) |

**Steps:**

- [ ] **Step 1:** Write the three test files; confirm they fail.
- [ ] **Step 2:** `git.go`, then `materialise.go` in the eight steps above, then `inspect.go`
  (it shares the target choice and calls `Source` exactly as `Materialise` does, but writes nothing).
- [ ] **Step 3:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/contextwrite/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite
$ grep -rn "os.Rename\|os.WriteFile" control-plane/internal/contextwrite/*.go | grep -v _test
(no output — every write is statepath.WriteAtomic)
$ grep -rn '"add"\|"commit"\|"update-index"' control-plane/internal/contextwrite/
(no output — no git write, ever)
```

**Does NOT claim:** that Claude Code reads `.claude/CLAUDE.md` (OQ-2, task 23's canary); a cloud
worktree (seam S4-P8.1); that Docker Desktop's bind mount honours `0o644` for a non-root container
user on this laptop (task 23 checks by reading the file from inside the sandbox).
