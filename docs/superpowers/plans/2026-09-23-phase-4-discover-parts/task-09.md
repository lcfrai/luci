### Task 09: `contextwrite` — the Materialiser

Satisfies task 10's `api.Materialiser`. Given a worktree about to start a session, it asks
`discoverhub` what the session inherits (P4-A), renders that one key, and writes it into the file
Claude Code reads — **atomically, before the process starts, never committed** — then returns the
key it wrote. It also answers the drift check (prototype state 11). **Kept from the 2026-09-18
plan's task 5** (`:2120-2474`): the package comment's ordering rule, absolute-path and
existing-directory checks, `Merge` preserving user text, no temp file left behind. **Changed:** it
is called by the session handlers, not by the desktop; it chooses its target (P4-D); it writes the
exclude line (phase 3 ask P4-6), before the block and never after it (review M13); it strips when nothing is inherited; it uses
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
// after the request validates and before anything is recorded or created:
// on create, before the conversation store's MkdirAll too; on restart,
// before the old sandbox's destroy (ruling R1; review C1, M2). Called
// afterwards, the session inherits nothing, silently, and the feature looks
// built.
//
// THE COMMIT RULE: the managed block is never committed (phase 3's 3-D6). It
// is written only to a file the repository does not track, and that file's
// path is added to the clone's info/exclude. A file git tracks is never
// opened for writing, not even to strip a block from it (review I3): a
// tracked file that already holds a block is repository text, left
// byte-identical and listed in Inspect's Committed (re-review R-M1).
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

// HubBudget bounds step 4's Hub reads (review I4). A black-holed Hub costs
// hubclient ~31 s per read (Hub contract §9.2: 10 s per attempt, two
// retries), and the desktop's session start waits 10 s (client.ts:33), so
// without it 4-D1's refusal would reach the person as the desktop's timeout.
// 5 s leaves the other 5 s for the sandbox and the rest of the start.
// Tests shorten it with WithHubBudget.
const HubBudget = 5 * time.Second

func (m *Materialiser) WithHubBudget(d time.Duration) *Materialiser

var _ api.Materialiser = (*Materialiser)(nil)

// Materialise, in order:
//  1. WorktreePath must be absolute and an existing directory, else a plain
//     error (500: the handler resolved it a moment ago).
//  2. Ask Git.Tracked of both candidates, once per call, and keep both
//     answers: the STRIPPABLE candidates are exactly the ones git does not
//     track (review I3). Choose the target: FileName if git does not track
//     it; else AltFileName if git does not track that; else
//     *api.MaterialiseError tracked_elsewhere (this step still runs before
//     step 3; the WithoutBrief dead end on that path is review N1, open for
//     its own batch). A directory git does not manage (Git reports
//     ErrNotARepo) has both candidates strippable, target FileName and no
//     exclude line. Any other Git error → a plain error (500), never a
//     guess at "untracked".
//  3. WithoutBrief: strip any managed block from every STRIPPABLE candidate
//     and return "none:declined". A tracked candidate is never opened. A
//     strip that fails refuses: a session labelled "started without the
//     brief" must not find an old brief in a file this package writes. A
//     block already committed in a tracked file is repository text that no
//     Panacea write may remove (3-D6): it is left byte-identical, the start
//     is not refused for it (the owner's OQ-7, plan §10: the recommended
//     option, with its provenance cost named there), and Inspect lists it
//     in Committed, which task
//     19 shows with the file's path (review I3, re-review R-M1).
//     The reason follows the cause (re-review I2):
//     claudemd.ErrUnterminatedBlock / ErrDuplicateBlock → block_broken (the
//     markers are broken; the person repairs the file); a read or write
//     error → write_failed (the disk refused). Since the marker rule (plan
//     §4 Security, S3; task 03 rule 12, task 05's neutraliser) no brief text
//     can break the markers, so block_broken here is reached only by a hand
//     edit - including a stray or early end marker, which task 05 now counts
//     as ErrUnterminatedBlock instead of stripping up to it and leaving the
//     brief's tail behind a none:declined stamp.
//  4. ProjectID "" is a worktree outside every project, which the handler
//     enumerated cleanly (task 10's sessionLocate; review I2): there is no
//     project, so nothing to inherit, and Source is NOT asked (a Locator
//     would answer project.ErrUnknownProject, a plain error, a 500 on
//     every start). An enumeration failure never reaches here as "": the
//     handler refuses it as project_unknown, a reason in api's closed set
//     that this package never returns.
//     Otherwise ask Source, under context.WithTimeout(ctx, HubBudget) (5 s,
//     review I4): every Hub read Inheritable makes shares that one budget.
//     hubclient.ErrUnavailable → hub_unavailable; the budget's OWN expiry
//     (the child ctx's Err() is DeadlineExceeded while the caller's ctx.Err()
//     is nil) → hub_unavailable too, whatever error type Source returned
//     (HA4-10: the Hub's task 33 retries even the caller's own expiry and
//     sleeps without ctx, so up to 1.25 s more can pass, about 6.25 s in
//     all, inside session start's 10 s; task 11); the caller's own ctx
//     cancelled (the desktop went away) → a plain error, never a refusal
//     reason. Only this step is budgeted: steps 1-3 and 5-7 are local git
//     and disk, and an expiry there would be misnamed as the Hub's.
//     entraauth.ErrSignedOut, or a relayed 401 → signed_out (a brief may
//     exist and cannot be read: 4-D1 refuses; the owner's OQ-11, plan §10,
//     recommended option, with Sign in first in task 20; review M10;
//     a tenant_not_enrolled sign-out lands here too, and task 20 then
//     offers no Sign in, re-review b09 M-1);
//     ErrHubOff, ErrNotBound, ErrNoRemote, or a Hub 404 → nothing to
//     inherit. Any other error → a plain error.
//  5. Nothing to inherit: strip every strippable candidate; return "".
//     Something: Merge(existing, Render(doc)) into the target, and strip
//     the other candidate only if it is strippable, so at most one managed
//     block exists across the files this package writes. A tracked other
//     candidate is not opened, even if it holds a block: the target is
//     still written and the key still returned, and Inspect lists that
//     file in the worktree's Committed (review I3, re-review R-M1).
//     claudemd.ErrUnterminatedBlock / ErrDuplicateBlock → block_broken.
//  6. The exclude line FIRST (review M13): before a write that lands a
//     block in a target git manages, append "/CLAUDE.md" or
//     "/.claude/CLAUDE.md" to the path Git.ExcludePath returns, if absent.
//     A Git.ExcludePath error is a plain error (500, step 2's Git rule); a
//     failed read or write of the exclude file → write_failed. Either way
//     the target is not opened for writing, so no brief text lands in a
//     file git would stage (3-D6: an agent's `git add -A` commits an
//     untracked, un-excluded CLAUDE.md). A strip that lands no brief text
//     needs no line. ErrNotARepo (step 2): no line, nothing to exclude from.
//  7. Write only if the bytes changed, with statepath.WriteAtomic(..., Shared).
//     A file left holding only whitespace after a strip is removed.
//     Any write error → write_failed. A failure here leaves step 6's line
//     in place: it names a file that is absent or holds no new brief text,
//     and step 6 would add it again next start anyway ("if absent").
//  8. Return the key written.
// Calls for one worktree path are serialised; different worktrees run in
// parallel.
func (m *Materialiser) Materialise(ctx context.Context, req api.MaterialiseRequest) (api.MaterialiseResult, error)
```

```go
// control-plane/internal/contextwrite/git.go
package contextwrite

// Git is the two read-only questions this package asks git. Task 14 wires it
// over project.Runner (phase 2's read-only runner), each command run with
// <dir> as its working directory (Run's dir argument; no -C is passed):
//   Tracked:     git --icase-pathspecs ls-files -- <rel>
//                (exit 0 either way: empty output = untracked, any output =
//                tracked; review I7). --icase-pathspecs because a pathspec is
//                case-sensitive even under core.ignorecase=true (the Windows
//                default): plain `ls-files -- CLAUDE.md` prints nothing when
//                the index holds claude.md, yet opening CLAUDE.md on NTFS or
//                APFS opens that tracked file (re-review R-I1). Not the
//                :(icase) pathspec magic: under GIT_LITERAL_PATHSPECS=1 that
//                silently matches nothing ("untracked"), where the option
//                fails with exit 128, which is a 500. On a case-sensitive
//                Linux checkout a tracked claude.md now reads as tracked and
//                moves the target to .claude/CLAUDE.md: the safe way.
//   ExcludePath: git rev-parse --git-path info/exclude  (joined to dir if relative)
// NO exit code is read. project.ExecRunner.Run wraps a failure with %v
// (gitrunner.go:74), which drops the *exec.ExitError, so "exit 1 means
// untracked" cannot be seen through the runner, and --error-unmatch is not
// used. The one error text parsed is "not a git repository", from either
// command, which becomes ErrNotARepo; the plan knows it is a string match.
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
	StateCurrent    WorktreeState = "current"    // claudemd.SameText(block, render) for the key a start would write now: byte-exact but for CRLF vs LF (review M7)
	StateSuperseded WorktreeState = "superseded" // the block names an older key; sessions already running read it
	// StateEdited: the marker names the right key, and the text is not that
	// key's render (state 11). A key's render depends on nothing but the key -
	// the title comes from the revision's own branchRef, never from a project
	// or branch name (task 05's Document.Title, review I5) - so a rename is
	// never "edited"; only a change to the file's bytes is, other than an
	// editor turning LF into CRLF (claudemd.SameText, review M7). Diff comes
	// from claudemd.DiffLines, which splits on \r?\n.
	StateEdited     WorktreeState = "edited"
	StateStale      WorktreeState = "stale"      // a block, but nothing should be inherited now
	StateBroken     WorktreeState = "broken"     // unterminated or duplicated
)

// CommittedBlock is a managed block, or a marker of one, in a file git
// TRACKS. It is repository text: never written, never stripped (review I3).
// It is NOT a WorktreeState, because it describes a second file, and one
// State cannot describe two files without hiding one (re-review R-M1).
type CommittedBlock struct {
	Path      string `json:"path"`      // the tracked file, absolute
	MarkerKey string `json:"markerKey"` // its marker's key; "" when its markers are broken
}

type Inspection struct {
	WorktreeID  string              `json:"worktreeId"`
	Path        string              `json:"path"`        // the TARGET inspected, absolute
	State       WorktreeState       `json:"state"`       // the target's own state, always one of the seven above
	MarkerKey   string              `json:"markerKey"`   // "" when there is no block
	ExpectedKey string              `json:"expectedKey"` // "" when nothing should be inherited
	Diff        []claudemd.DiffLine `json:"diff"`        // StateEdited only; [] otherwise
	Committed   []CommittedBlock    `json:"committed"`   // every tracked candidate holding a block or marker, FileName first; [] when none
	// Excluded is the info/exclude line Materialise's step 6 adds before it
	// writes a block to this target: "/CLAUDE.md" or "/.claude/CLAUDE.md";
	// "" when git does not manage the directory (ErrNotARepo, step 2). It
	// comes from the target choice alone, with no git call and no read of
	// the exclude file. Task 19 says with it that git stops listing the file,
	// text of the person's own in it included (review M9).
	Excluded    string              `json:"excluded"`
	// AltTracked is step 2's Git.Tracked answer for AltFileName
	// (.claude/CLAUDE.md), kept, not asked again. It is false when git does
	// not manage the directory. Task 19 needs it: when the target is
	// FileName and AltTracked is true, committing CLAUDE.md makes both
	// candidates tracked, and step 2 then refuses every start in this
	// worktree (tracked_elsewhere, review N1), so the view must not offer
	// that commit as a way back (re-review b09 I-1).
	AltTracked  bool                `json:"altTracked"`
}
// Committed holds at most one entry today: an Inspection is returned only
// when one candidate is the untracked target, so at most the other is
// tracked. It is a list, ordered, so review N1's batch can report both
// files of a both-tracked repository without a shape change.

// Inspect reads, never writes. It is the ONLY place brief text is read back
// out of a worktree, and it reads it only to compare (contract §3.3).
//
// It asks Git.Tracked of both candidates, as Materialise's step 2 does, and
// chooses the target the same way. The target is inspected for its own
// state (the seven above), always, so a committed block elsewhere never
// hides an edited, broken or superseded target: state 11's diff and its
// rewrite warning still show, and a broken target still names itself
// (re-review R-M1). Separately, every TRACKED candidate that holds a block,
// or any marker of one (claudemd.Inspect ok, or ErrUnterminatedBlock /
// ErrDuplicateBlock), is listed in Committed, FileName before AltFileName,
// with its marker's key ("" when its markers are broken). Committed is []
// when there is none. With both candidates tracked there is no target, and
// Inspect answers what the shared target choice answers (tracked_elsewhere)
// whatever the files hold: no Panacea session starts in that repository
// (Materialise refuses every start there, WithoutBrief included, which is
// review N1, open for its own batch), and the refusal names the cause a
// person acts on. N1's batch revisits Committed on that path.
func (m *Materialiser) Inspect(ctx context.Context, req api.MaterialiseRequest) (Inspection, error)
```

**Tests first** (a fake `Source`, a fake `Git` answering `Tracked` per path, `t.TempDir()`
worktrees; the real `git ls-files` wiring is tested over real git in task 14):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWritesTheInheritedKeyAndReturnsIt` | `CLAUDE.md` holds `Render(doc)`; result is the key | a key returned for a file not written |
| `TestPreservesUserTextAroundTheBlock` | a hand-written CLAUDE.md keeps every byte outside the block | the old plan's defect class, reintroduced by a rewrite |
| `TestTrackedClaudeMdFallsBackToTheAltFile` | `Tracked(CLAUDE.md)=true` → block in `.claude/CLAUDE.md`; `CLAUDE.md` untouched | a tracked file modified — an agent's `git add -A` commits the brief (3-D6) |
| `TestBothTrackedIsRefused` | `*api.MaterialiseError{tracked_elsewhere}`; nothing written | a write into a tracked file as a last resort |
| `TestExcludeLineIsAddedOnce` | after three materialises, `info/exclude` has exactly one `/CLAUDE.md` line and its earlier lines intact | an exclude that grows each start |
| `TestExcludeFailureWritesNoBlock` (review M13) — table | something to inherit, `CLAUDE.md` untracked. Row 1: the fake `Git.ExcludePath` returns a path under a regular **file** (`<tmp>/f/exclude`, `f` a file: a write error on every OS) → `*api.MaterialiseError{write_failed}`. Row 2: `Git.ExcludePath` returns an error → a plain error, not a `*api.MaterialiseError`. In both rows, first with no `CLAUDE.md` and then with a hand-written one: afterwards the file is absent, or byte-identical with an unchanged mtime, and holds no begin marker | steps 6 and 7 back in the old order: the block lands, the exclude write fails, and an untracked, un-excluded `CLAUDE.md` holding the brief waits for an agent's `git add -A` (3-D6) |
| `TestNothingToInheritStripsAnOldBlock` | an old block and nothing inheritable → block removed, result `""` | a chip saying *no brief* over a file that still holds one |
| `TestAFileHoldingOnlyTheBlockIsRemovedOnStrip` | the file does not exist afterwards | an empty CLAUDE.md left in every worktree |
| `TestWithoutBriefStripsAndDeclines` | both candidates untracked and each holding a block → both stripped; result `none:declined`; `Source` never called | the override blocked by the Hub being down, the very case it exists for |
| `TestNeverStripsATrackedFile` — table | `CLAUDE.md` **tracked** and holding a block, `.claude/CLAUDE.md` untracked. Row 1, something to inherit: the block is written to `.claude/CLAUDE.md`, the key is returned, and `CLAUDE.md` is byte-identical with an unchanged mtime. Row 2, nothing to inherit: result `""`, `CLAUDE.md` byte-identical. Row 3, `WithoutBrief`: result `none:declined`, `CLAUDE.md` byte-identical. Row 4, the tracked file's markers broken (begin only): rows 1-3 still succeed, not `block_broken`. In every row the fake `Git` asked `Tracked` of both paths, and `Inspect` on the same request gives `Path` the target `.claude/CLAUDE.md` with its own state, and `Committed` exactly one entry, `Path` the tracked `CLAUDE.md` (review I3, re-review R-M1) | "strip both candidates" rewriting a tracked `CLAUDE.md` on every start and leaving the worktree dirty, so an agent's `git add -A` commits the change (3-D6); a tracked file's broken markers refusing a start whose target is another file |
| `TestHubUnavailableIsARefusal` | `ErrUnavailable` → `hub_unavailable`; the file untouched | a silent `""` when a brief may exist — 4-D1's whole point |
| `TestABlackHoledHubIsARefusalWithinTheBudget` | a `Source` that blocks until `ctx.Done()` and returns a plain `ctx.Err()` (not `ErrUnavailable`), `WithHubBudget(100ms)` → `hub_unavailable` within 1 s, the file untouched; the same `Source` with the caller's `ctx` cancelled at 50 ms and the budget at 10 s → a plain error, not a `*api.MaterialiseError` | a Hub read on the caller's unbounded `ctx`, which reaches the desktop as `timeout` instead of 4-D1's refusal; or a desktop that went away recorded as a Hub outage (review I4) |
| `TestTheHubBudgetIsBelowTheDesktopSessionTimeout` | `HubBudget < 10*time.Second` (the desktop's `DEFAULT_TIMEOUT_MS`, which session start uses) | a budget raised to the Hub's own 31 s, which re-opens I4 |
| `TestSignedOutIsARefusal` | `ErrSignedOut` → `signed_out` | as above |
| `TestHubOffAndUnboundInheritNothing` | `ErrHubOff`, `ErrNotBound`, `ErrNoRemote` → `""`, no refusal | Develop blocked for everyone without a Hub (Hub contract §9.5: nothing blocks Develop) |
| `TestNoProjectInheritsNothingWithoutAskingTheHub` | `ProjectID: ""` and an old block in `CLAUDE.md` → the block stripped, result `""`, no refusal, `Source` never called; `Inspect` on the same request never calls `Source` either (review I2) | `ProjectID ""` sent to the Hub, where `Locator.ProjectPath("")` is a plain error and every start outside a project is a 500 |
| `TestUnterminatedBlockIsBlockBroken` | `block_broken`; the file byte-identical | the guess that deletes a person's writing |
| `TestWithoutBriefOverAStrayEndMarkerIsBlockBroken` | a file holding begin, an early end, brief text, a second end; `WithoutBrief: true` → `*api.MaterialiseError{block_broken}`; the file byte-identical; `Source` never called | a strip that removes up to the first end and stamps `none:declined` over a file still holding brief text (review C2); a broken marker reported as `write_failed`, which names the wrong cause and offers no repair (re-review I2) |
| `TestWithoutBriefWriteErrorIsWriteFailed` | `CLAUDE.md` is a **directory** in the worktree (a read error on every OS; the fake `Git` reports it untracked); `WithoutBrief: true` → `*api.MaterialiseError{write_failed}`; `Source` never called | every strip failure collapsed into `block_broken`, sending a person to repair markers in a file that has none |
| `TestUnchangedBytesAreNotRewritten` | the file's mtime does not move on a second identical materialise | a rewrite per start — noise in every editor watching the file |
| `TestFileIsWorldReadable` (skipped on Windows) | mode `0o644` | the registry's `0o600` making the brief unreadable to the sandbox user (task 01) |
| `TestConcurrentStartsOnOneWorktreeSerialise` | two goroutines, two keys: the file ends as one complete render of one of them and `Merge` never sees a half-written file | interleaved writes |
| `TestInspectStates` — table | each of the seven states from a constructed file and expected key, with `Committed` `[]` (not `nil`: it marshals as `[]`); `edited` carries a diff with the changed line (a rename is not a changed line: `Source`'s document carries no name, so this package cannot be fed one; the rename end to end is task 07's `TestRenameDoesNotLookLikeAHandEdit`, review I5). One more row (review M7): a `current` file with every `\n` turned to `\r\n` is still `current`, with `Diff` `[]` | a drift check that only compares keys (brief §4.2 gap 3); one that compares raw bytes, so an editor's CRLF re-save reads as `edited` over every line |
| `TestInspectListsCommittedBesideTheTargetsState` — table | a tracked `CLAUDE.md` holding a block, and the untracked target `.claude/CLAUDE.md` in turn `current`, `edited`, `broken` and `superseded`: `State` is the target's own each time (`edited` still carries its diff), and `Committed` is one entry, the tracked file's path and marker key. A tracked `CLAUDE.md` holding only a begin marker → its entry has `MarkerKey ""`. Both candidates tracked, each holding a block → `tracked_elsewhere`, as `TestBothTrackedIsRefused` (re-review R-M1) | a committed block hidden behind the target's `current`; a committed block hiding an `edited` target, so the next start silently overwrites a hand edit; a broken target reported only as committed, so the person removes the committed block and every start is still refused `block_broken` |
| `TestInspectReportsTheExcludeLine` (review M9) — table | both untracked → `Excluded: "/CLAUDE.md"`, `AltTracked: false`; `CLAUDE.md` untracked and `.claude/CLAUDE.md` tracked → `Excluded: "/CLAUDE.md"`, `AltTracked: true` (re-review b09 I-1); `CLAUDE.md` tracked → `"/.claude/CLAUDE.md"`, `AltTracked: false`; the fake `Git` answering `ErrNotARepo` → `""`, `AltTracked: false`; in every row the fake `Git.ExcludePath` is never called, and the fake `Git.Tracked` is called once per candidate | a view that says git stops listing a file in a directory git does not manage; an `Excluded` naming the other candidate; an `AltTracked` hard-wired false, which lets task 19 offer a commit that stops every start |

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
