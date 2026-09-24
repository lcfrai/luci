### Task 17: The OD-8 transcript adapter — tier-2 back-links, pinned, canary-tested, read-only, degrading

`AGENTIC-UX.md:206` calls hunk → transcript-span back-links "the strongest differentiator
available". OD-8 allows it under guards, and this task is those guards made mechanical:

> A harness's own on-disk records — for Claude Code, its JSON-lines transcript — may be read,
> read-only, through a per-harness adapter that is **pinned to the CLI version in the sandbox
> image**, covered by a **canary test** that fails when the format moves, and whose every consumer
> **degrades to commit- and session-level facts** when the adapter cannot read a file. No feature
> may *require* the adapter to function. (OD-8, quoted.)

The control plane already knows which file to read, **by an id it minted**: session create mints
the conversation id and tells the CLI with `--session-id` (`sessions.go`, step 2a), and the
worktree's conversation store is bind-mounted over `~/.claude/projects`, where claude 2.1.274
writes `~/.claude/projects/-home-dev-work/<conversation id>.jsonl` (observed in the image, per the
comment at `sessions.go:39-42` and the Go contract's source table). On the host that is
`<ConversationsDir>/local/<worktree id>/-home-dev-work/<conversation id>.jsonl`. No PTY byte is
read, and nothing here derives meaning from terminal output.

**The line shape, verified 2026-09-23 on the host's Claude Code 2.1.267** (one read-only pass over
this machine's own transcripts; **not** on 2.1.274), **corrected by review I1** (a read-only
structural count over the 40 newest transcripts, 76,592 lines, Claude Code 2.1.236, 2.1.260,
2.1.267 and 2.1.280, no text printed): every line carries `type`, but **only `user`,
`assistant`, `system` and `attachment` lines carry `version`**. `user` and `assistant` lines
carry `timestamp`, `uuid`, `sessionId` and `cwd` (the 2.1.267 pass; review I1 counted `version`
only, so no other key's presence on the version-less types is claimed). 25,111 of those lines
(about a third) carry no `version`, of eleven types: `last-prompt`, `mode`, `permission-mode`,
`atis-latch`, `ai-title`, `queue-operation`, `file-history-snapshot`, `file-history-delta`,
`cost-state`, `relocated`, `worktree-state`. The adapter reads only `user` and `assistant` lines
(below). An `assistant` line's `message.content[]` holds `tool_use` items with `id`, `name`, `input`; `Edit`'s input has `file_path`, `old_string`,
`new_string`, `replace_all`, and `Write`'s has `file_path`, `content`; the following `user` line
carries `toolUseResult` with `filePath` and `structuredPatch[]` of `{oldStart, oldLines, newStart,
newLines, lines[]}` (each line prefixed ` `, `+` or `-`), and `sourceToolAssistantUUID`.

**Files:**
- `control-plane/internal/transcript/claudecode/adapter.go` — `PinnedVersion`, `Adapter`, `ToolCall`, `Span`, `Unreadable` (new)
- `control-plane/internal/transcript/claudecode/match.go` — `Match` (new)
- `control-plane/internal/transcript/claudecode/*_test.go`, `testdata/` — the canaries and fixtures (new)
- `control-plane/internal/changes/diff.go` — `Linker`; `BackLink` gains three fields (task 16's file)
- `control-plane/internal/api/transcript.go` + test — the span GET and the `Linker` glue (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go`, `routes_audit_test.go` — `// ── task 17 ──`

#### Interfaces — complete

```go
// Package claudecode reads Claude Code's own JSON-lines transcript, read-only, for one purpose:
// linking a diff hunk to the tool call that wrote it (OD-8). It is pinned to the CLI version
// images/sandbox/Dockerfile installs. Nothing may require it: every caller degrades to tier 1.
package claudecode

// PinnedVersion is the @anthropic-ai/claude-code version in images/sandbox/Dockerfile.
// TestPinnedVersionMatchesTheImage fails the moment the image moves and this does not.
const PinnedVersion = "2.1.274"

// WorkDir is where the worktree is mounted in the sandbox; api's test pins it equal to
// sessionWorkDir. A tool call's file_path is WorkDir + "/" + the repository-relative path.
const WorkDir = "/home/dev/work"

// ProjectsSubdir is the directory claude names after the working directory it started in.
const ProjectsSubdir = "-home-dev-work"

// MaxBytes bounds one read. A larger transcript is Unreadable, not read partially.
const MaxBytes = 64 << 20

// Unreadable is every reason the adapter declines. Reason is the sentence the Files lens shows
// beside the tier-1 link.
type Unreadable struct{ Reason string }

func (u *Unreadable) Error() string { return "claudecode: " + u.Reason }

// ToolCall is one file-writing tool call.
type ToolCall struct {
	ID       string    // the tool_use id
	Name     string    // "Edit" | "Write" | "MultiEdit"
	FilePath string    // repository-relative, forward slashes; calls outside WorkDir are dropped
	At       time.Time // the assistant line's timestamp
	// Added is the lines the call added, normalised (CRLF → LF, trailing whitespace trimmed):
	// the "+" lines of its result's structuredPatch when present, else new_string / content /
	// edits[].new_string split into lines.
	Added []string
}

// Span is what the Files lens shows for one tool call: where it was written and why.
type Span struct {
	ToolUseID  string    `json:"toolUseId"`
	Tool       string    `json:"tool"`
	FilePath   string    `json:"filePath"`
	At         time.Time `json:"at"`
	OldString  string    `json:"oldString"` // Edit only; capped at 4 KiB
	NewString  string    `json:"newString"` // Edit's new_string or Write's content; capped at 4 KiB
	// Before is the assistant text of the same message before the tool call, and of the
	// assistant message before it when that is empty — the agent's stated reason. Capped 2 KiB.
	Before string `json:"before"`
}

// Adapter reads transcripts from the conversation stores under ConversationsDir.
type Adapter struct {
	ConversationsDir string // api.Deps.ConversationsDir
	Owner            string // "local": sessions.go's sessionConversationOwner
}

// Path is the host path of one conversation's transcript.
func (a *Adapter) Path(worktreeID, conversationID string) string

// ToolCalls reads every file-writing tool call. It declines with *Unreadable when: the file
// does not exist ("the transcript is gone — Forget and worktree removal delete it"); it is over
// MaxBytes; any line that carries a `version` carries one other than PinnedVersion ("written by
// claude <v>; the adapter reads <PinnedVersion>"); or more than 1% of non-empty lines fail
// ("the transcript's format is not the one the adapter reads"). Review I1 fixes what counts:
//   - a line fails when it is not a JSON object with a string `type`, or when it is a `user` or
//     `assistant` line with no `version` or with a shape the parser cannot read;
//   - a line of any other `type` that carries no `version` (the eleven types in the shape
//     paragraph above, and any new one) is skipped: not read, not a failure, still in the
//     denominator; one that carries a `version` is checked against the pin and otherwise skipped.
// So the pin is enforced on every line that names a version and required on the two types the
// adapter reads; a transcript with no user or assistant line yields no calls and no error.
// It opens the file read-only, reads it whole, and closes it before returning — on Windows an open handle would block Forget's delete (OD-10). Results
// are cached by (path, size, mtime).
func (a *Adapter) ToolCalls(ctx context.Context, worktreeID, conversationID string) ([]ToolCall, error)

// Span reads one tool call's span.
func (a *Adapter) Span(ctx context.Context, worktreeID, conversationID, toolUseID string) (Span, error)

// Match picks the call that wrote a hunk's added lines: calls on the same path, at or before
// commitAt, scored by how many of the hunk's normalised "+" lines appear in the call's Added;
// the best score wins if it covers at least half the hunk's added lines (ties: the latest call).
// ok false means none did — a shell command, a formatter, or a region edited again later.
func Match(calls []ToolCall, path string, added []string, commitAt time.Time) (call ToolCall, ok bool)
```

```go
// changes/diff.go — task 16 declared BackLink's ToolUseID, ToolName and SpanAt, always empty
// there; this task fills them. Diff gains one trailing parameter, `linker Linker`.

// Linker upgrades a tier-1 link to tier 2. Diff calls it for each committed hunk whose tier-1
// link names exactly one session; ok false leaves tier 1 and sets Reason to reason. A nil
// Linker is valid and means tier 1 everywhere — nothing requires the adapter.
type Linker interface {
	Link(ctx context.Context, sessionID, path string, added []string, commitAt time.Time) (toolUseID, toolName string, at time.Time, ok bool, reason string)
}
```

The `Linker` implementation lives in `api/transcript.go`: `Store.Get(sessionID)`; a harness other
than `claude-code` → not ok, "transcript links exist for Claude Code only"; an empty
`ConversationID` → "this session has no conversation id"; otherwise `Adapter.ToolCalls` then
`Match`, and an `*Unreadable` becomes the reason verbatim.

**Wire: `GET /v1/sessions/{id}/transcript/spans/{toolUseId}`**

```json
{ "toolUseId": "toolu_01…", "tool": "Edit", "filePath": "src/upload/handler.ts", "at": "2026-09-23T04:01:47Z",
  "oldString": "  const stored = await storeChunk(req);",
  "newString": "  const resumeFrom = await lastStoredChunk(req.uploadId);\n  if (req.index < resumeFrom) return { skipped: true };\n  const stored = await storeChunk(req);",
  "before": "The handler has to skip chunks it already stored, or a resumed upload duplicates them." }
```

`404 session_not_found`; `409 conflict` with `details.reason` `transcript_unreadable` and
`details.why` the `Unreadable.Reason` (added to task 04's list). Audit line: *"Reads one transcript
file, read-only, through the OD-8 adapter. Writes nothing."*

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestPinnedVersionMatchesTheImage` | reads `images/sandbox/Dockerfile`, finds `@anthropic-ai/claude-code@<v>`, `v == PinnedVersion` | an image bump the adapter never re-validated |
| `TestWorkDirMatchesSessions` (api package) | `claudecode.WorkDir == sessionWorkDir` | a mount point moved without the adapter |
| `TestCapturedFixtureParses` | reads `testdata/captured-` + `PinnedVersion` + `.jsonl`. When it is absent the test **skips only while** `PinnedVersion == "2.1.274"` **and** the test file's `const od12CaptureWaiver = "2.1.274"` equals it, with the message "capture testdata/captured-2.1.274.jsonl in the owner's hands-on session (OD-12)"; with any other pin it **fails** ("the pin moved to <v> and no captured-<v>.jsonl exists"), so a pin bump forces a new capture (review I1). Present: it yields ≥ 1 Edit and ≥ 1 Write with non-empty `Added`, and no `*Unreadable` | the real format differing from the synthetic shape; a canary that never fails |
| `TestSyntheticFixtureParses` | `testdata/synthetic-shape.jsonl` — lines written by hand to the shape review I1 counted: `user`, `assistant`, `system` and `attachment` lines with `version` set to the pin, and one line of each of the eleven version-less types carrying no `version`; **its README says so** — yields the Edit, the Write and the MultiEdit with their `Added`, and no `*Unreadable` | a parser broken on the known shape; a fixture written to a shape no transcript has |
| `TestVersionlessLinesAreIgnored` (review I1) | a fixture that is 40% `file-history-snapshot` lines with no `version`, plus one Edit exchange → `ToolCalls` returns the Edit, and `Linker` gives the hunk tier 2 | the 1% failure rule or the pin check firing on every real transcript |
| `TestAnAssistantLineWithoutAVersionFails` | the Edit's `assistant` line has its `version` removed and it is the only failing line of 50 → counted: 2% fails → `*Unreadable` format reason | the pin skipped on the very lines the adapter reads |
| `TestAVersionedOtherLineIsPinned` | an `attachment` line with `version: 2.1.300` → `*Unreadable` naming both versions | checking the pin on `user` and `assistant` lines only |
| `TestPrefersTheStructuredPatch` | a result line whose structuredPatch adds a line new_string does not show verbatim (context trimmed) → `Added` is the patch's `+` lines | matching on a string the file never held |
| `TestAnotherVersionDegrades` | one `assistant` line with `version: 2.1.300` → `*Unreadable` naming both versions | reading a format the adapter was not pinned to |
| `TestMissingFileDegrades` | → `*Unreadable` "the transcript is gone …" | a 500 after Forget |
| `TestGarbageDegrades` | 5% unparseable lines → `*Unreadable` format reason | a partial parse producing wrong links |
| `TestCallsOutsideTheWorktreeAreDropped` | `file_path: /home/dev/.claude/x` → not in the result | linking a hunk to a config write |
| `TestMatchPicksTheCallThatWroteTheLines` | two Edits to one file; the hunk's lines are the second's → the second | the first edit to the file winning |
| `TestMatchIgnoresCallsAfterTheCommit` | the only matching call is after `commitAt` → not ok | a later edit claiming an earlier commit |
| `TestMatchDeclinesAShellWrite` | the lines were written by a `Bash` tool_use running `sed` → not ok | a guess where there is no tool call (survey §8.3) |
| `TestMatchNormalisesLineEndings` | CRLF in the transcript, LF in the diff → ok | Windows line endings breaking every match |
| `TestTheAdapterNeverWrites` | after ToolCalls and Span, the file's bytes and mtime are unchanged; a source scan of the package finds no `os.Create`, `os.WriteFile`, or `OpenFile` with a write flag | OD-8's "rewriting its contents is not" allowed |
| `TestTheAdapterClosesTheFile` | on Windows, `os.Remove` of the file immediately after `ToolCalls` succeeds | Forget failing because we hold a handle |
| `TestDiffWithoutALinkerIsTierOne` | `changes.Diff` with a nil Linker → tier 1 on every committed hunk, no error | a feature requiring the adapter |
| `TestLinkerDeclinesOtherHarnesses` | a `codex` session → not ok, "transcript links exist for Claude Code only" | reading a harness nobody pinned (3-D7 c) |
| `TestDiffUpgradesToTierTwo` | a real clone, a commit stamped `s-1`, a transcript fixture for `s-1`'s conversation id → the hunk's `tier 2`, `toolUseId`, `toolName` | the chain never completing |
| `TestSpanRouteReturnsTheCallAndItsReason` | → the JSON above from the fixture; the §10a audit passes | a span without its "why" |

#### Steps

- [ ] **Step 1: Preflight.** Task 16 merged. `grep -n "claude-code@" images/sandbox/Dockerfile`
  still reads `2.1.274`; if not, `PinnedVersion` moves with it and `od12CaptureWaiver` does
  **not**, so `TestCapturedFixtureParses` fails until `testdata/captured-<new pin>.jsonl` exists
  (review I1). Only the owner may move the waiver, and only by an OD-12 decision on record.
- [ ] **Step 2: Try to capture the real fixture.** In `panacea-app/sandbox:dev`, a transcript with
  an Edit needs a signed-in Claude Code, which is the owner's hands-on step (OD-12). If a real
  2.1.274 transcript already exists in a conversation store on this machine, copy **one** Edit
  and one Write exchange into `testdata/captured-2.1.274.jsonl`, with every line of user text
  replaced by `"redacted"` and the file reviewed for secrets before commit. If none exists, do not
  fabricate one: leave the test skipping and put the capture on task 23's owner checklist.
- [ ] **Step 3:** the synthetic fixture `testdata/synthetic-shape.jsonl` and its
  `testdata/README.md` ("hand-written to the line shape counted on claude 2.1.236–2.1.280 on
  2026-09-23 (review I1): versioned `user`, `assistant`, `system` and `attachment` lines, and one
  line of each of the eleven version-less types; not captured from 2.1.274"). No line of it is
  copied from a real transcript.
- [ ] **Step 4:** tests; `adapter.go`, `match.go`; `Linker`; the span handler; audit line; the new
  reason `transcript_unreadable` in `gitdeps.go`.
- [ ] **Step 5:** run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/transcript/... ./control-plane/internal/changes/ ./control-plane/internal/api/ -race -count=1
  → PASS; TestCapturedFixtureParses SKIP with the OD-12 message (until captured)
git grep -nwE "internal/proxy|attach" -- 'control-plane/internal/transcript/*.go'
  → no output (no PTY path reaches the adapter)
```

The grep matches whole words in Go source only. `-w` does not match the transcript's own
`attachment` line type, and `*.go` leaves `testdata/` out, whose fixtures carry `attachment`
lines by design (Step 1). What it guards is a reference to the PTY attach path
(`control-plane/internal/api/attach.go`: `registerAttach`, `handleAttach`, `relayAttached`) or
to the proxy.

#### What this task does not claim

- **Tier 2 is unverified against the pinned CLI** until the captured fixture exists; it is built
  and tested against the shape counted on 2.1.236–2.1.280 (review I1). Because a match needs the file path **and** the added
  lines to agree, a format difference makes tier 2 decline (tier 1 stays), not link wrongly —
  that is inferred from the matching rule, and the captured-fixture test is what proves it.
- Claude Code only (3-D7 c). Codex, OpenCode and Gemini CLI get tier 1.
- The managed Claude policy inside a sandbox (risk R4) does not touch this: nothing here uses a
  Claude hook or MCP.
