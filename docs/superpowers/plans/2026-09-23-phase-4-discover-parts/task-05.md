### Task 05: `discover.Resolve` with a pinned project revision; `claudemd`

Project → branch → session. This task turns **one** brief key's worth of immutable text into the
managed block an agent reads. **Kept from the 2026-09-18 plan's task 4** (`:1572-2110`): `Origin`,
`Section`, the heading table and file order (`:1734-1743`), the override rule and its wording,
deterministic render with no timestamp, `Merge` that keeps every byte outside the markers and fails
closed on an unterminated block (`:2073-2101`). **Changed:** `Resolve` takes two *revisions*, not
two mutable briefs, and refuses a project revision the branch did not pin (`4-D3`); skipped and
drafted answers have rules; `claudemd` gains `Strip`, `Inspect`, `SameText` and `DiffLines`, all
reading a CRLF file as well as an LF one (review M7); the footer names
the inherited revision; and (review C2, plan §4 Security, S3) `Render` neutralises `<!--` in every
interpolated string, while `Merge`, `Strip` and `Inspect` count markers, so a stray, early or
second end marker is `ErrUnterminatedBlock` where the old `Merge` stopped at the first end.

**Files:**
- `control-plane/internal/discover/inherit.go` + `inherit_test.go` (new)
- `control-plane/internal/claudemd/render.go` + `render_test.go` (new)
- `control-plane/internal/claudemd/testdata/golden-branch.md`, `golden-project.md`,
  `golden-neutralised.md` (new)
- `control-plane/internal/claudemd/testdata/.gitattributes` — one line, `* -text` (new). This
  checkout has `core.autocrlf = true` and no `.gitattributes` (`survey-phase-2-substrate.md:69-70`),
  so a fresh clone or worktree would check the golden files out with CRLF and fail every
  byte-for-byte golden test against `Render`'s LF output (review M7, same root cause)

**Interfaces — complete source:**

```go
// control-plane/internal/discover/inherit.go
package discover

// Revision is one stored brief revision as this package needs it. Key is the
// Hub's key, carried opaquely: this package never parses it (P5).
type Revision struct {
	BriefID string
	Number  int
	Key     string
	Scope   Scope
	// BranchRef is the revision envelope's own branchRef (Hub contract §3.4):
	// "refs/heads/evidence-upload" for a branch revision, "" for a project one.
	// The digest covers it, and a rebind never changes it: PATCH writes no
	// revision, and only the next revision's envelope carries the new ref
	// (Hub contract §7.5). Review I5.
	BranchRef string
	Content   Content
}

// There is no Names type. The block's title is computed from the revision the
// key names and nothing else - never from the Hub project's current name
// (PATCH /v1/projects/{id} changes it) or the branch's current name (a rebind
// changes it). A render that read either would make every worktree's block
// look hand-edited after a rename (review I5; TestRenameDoesNotLookLikeAHandEdit).

type Origin string

const (
	OriginProject  Origin = "project"  // inherited unchanged
	OriginBranch   Origin = "branch"   // set on the branch; the project has no answer
	OriginOverride Origin = "override" // set on the branch, replacing the project's answer
)

type Section struct {
	Key      Key         `json:"key"`
	Heading  string      `json:"heading"`
	Body     string      `json:"body"`
	Items    []Criterion `json:"items"`    // KeyDone only
	Origin   Origin      `json:"origin"`
	Replaced string      `json:"replaced"` // OriginOverride: the project text replaced; "" otherwise
}

// Document is everything Render needs. It is produced by Resolve and by
// nothing else.
type Document struct {
	// Title is a function of the key alone (review I5). A branch key:
	// "Branch brief — " + the branch revision's BranchRef minus "refs/heads/"
	// ("Branch brief — evidence-upload"). A project key: "Project brief".
	// Computed by Title below, the rule's only copy (re-review b05-1 R-M2).
	// No project name and no current branch name, so a rename changes no byte
	// of any block, and every title byte is covered by the key's digest.
	Title           string    `json:"title"`
	BriefID         string    `json:"briefId"` // the revision the key names: the branch's when branch != nil
	Revision        int       `json:"revision"`
	Key             string    `json:"key"`
	ProjectBriefID  string    `json:"projectBriefId"` // the inherited project revision, when there is one
	ProjectRevision int       `json:"projectRevision"`
	Sections        []Section `json:"sections"`
}

var ErrInheritMismatch = errors.New("discover: the project revision is not the one this branch revision inherits")
var ErrIncomplete = errors.New("discover: only a complete revision can be resolved into a document")

// Resolve builds the document for exactly one key.
//
//   - branch == nil: the project revision alone; every section OriginProject.
//   - branch != nil: project MUST be the revision branch.Content.Inherits
//     names (compared by Key), or nil when Inherits is nil. Anything else is
//     ErrInheritMismatch: rendering a branch key against a project revision
//     it did not pin is how one key comes to name two texts (critique-2 §1.1).
//   - A branch answer that is Skipped inherits the project answer (OriginProject).
//   - A project answer that is Skipped, with no branch answer, is omitted.
//   - A model_drafted answer is never rendered.
//   - Each revision passed must be StatusComplete, else ErrIncomplete: only a
//     complete revision is inherited (ADR-H1 Q1, extended by P4-A).
//
// Sections follow the base heading order, then follow-ups - project first,
// then branch - in the order they were added.
//
// Title follows Document.Title's rule: Resolve sets Document.Title to
// Title(branch). Resolve takes no name argument: nothing mutable can reach
// the block.
func Resolve(project, branch *Revision) (Document, error)

// Title is Document.Title's rule, and the only copy of it (re-review b05-1
// R-M2): "Branch brief — " + branch.BranchRef minus "refs/heads/" when
// branch != nil, else "Project brief". It reads nothing but BranchRef, and
// it does not look at Content.Status, so an open revision gets the same
// title its complete successor on the same ref will: provenance.Build calls
// it for an open history row, which Resolve refuses with ErrIncomplete
// (task 06). No package writes "Branch brief" or "Project brief" itself.
func Title(branch *Revision) string

// MarkerSafe reports whether id can stand in claudemd's begin marker:
// non-empty, with no white space (unicode.IsSpace), no "<" and no ">". It
// is the rule's only copy (task 05 fix round 1, review I1): Resolve refuses
// a revision whose BriefID fails it, and claudemd's begin-marker parser
// accepts exactly the ids that pass it, so every block Render(Resolve(…))
// writes reads back, and the two sides cannot drift.
func MarkerSafe(id string) bool
```

```go
// control-plane/internal/claudemd/render.go
package claudemd

const (
	BeginPrefix = "<!-- panacea:discover:begin"
	EndMarker   = "<!-- panacea:discover:end -->"
)

var (
	// A marker with no partner: a begin with no end after it, an end with no
	// begin before it (a stray end, or an end ahead of the begin), or a second
	// end marker. Review C2: a forged early end would otherwise split the block.
	ErrUnterminatedBlock = errors.New("claudemd: a managed-block marker with no partner")
	ErrDuplicateBlock    = errors.New("claudemd: more than one managed block")
	ErrBadMarker         = errors.New("claudemd: begin marker is not brief=<id> rev=<n>")
)

// commentOpener is what every marker starts with; Render never emits it
// from a Document string.
const commentOpener = "<!--"

// neutralised replaces commentOpener inside a Document string.
const neutralised = `<!-\-`

// Render returns the managed block, both markers and a trailing newline.
// Deterministic: the same Document gives the same bytes, and nothing in it
// depends on the clock (a timestamp would rewrite the file on every start).
// Every string it interpolates from doc (headings from prompts, answer and
// criterion text, the title's branch ref) has each "<!--" replaced by
// `<!-\-` first. This is defence in depth behind task 03's rule 12
// (ErrReservedText): a rendered block contains exactly two "<!--" sequences,
// its own two markers, so no content can forge either one.
// In the same pass, each "\r\n" and then each lone "\r" in those strings
// becomes "\n" (b10 re-review M-5). Task 03 rule 14 stores only LF, but a
// head the Hub stored unchecked may still hold "\r"; so Render's output holds
// no "\r" at all, which is what makes Merge's CRLF conversion exact.
func Render(doc discover.Document) string

// Merge splices block into existing: appended after a blank line when there
// is no block, replacing the one block when there is, every other byte kept.
// Appending: an empty existing gets the block alone; one that ends in a line
// ending gets one more line ending (the blank line), then the block; one that
// does not gets two, then the block (b10 re-review M-6).
// It first counts every BeginPrefix and every EndMarker in existing. The
// only well-formed shapes are none of either, or exactly one begin followed
// later by exactly one end. Two or more begins is ErrDuplicateBlock; any
// other shape (a begin with no end, an end with no begin before it, an end
// ahead of the begin, two ends) is ErrUnterminatedBlock. Both change
// nothing: the guess would delete somebody's writing, or leave brief text
// behind as if it were theirs.
//
// Line endings (review M7: Windows first, and an editor may re-save the file
// with CRLF). Every newline this package looks for - after a marker, the one
// Strip removes, the blank line Merge appends after - is matched as \r?\n.
// Merge writes the block, and the blank line before an appended one, in the
// file's own ending: CRLF when existing's first line ending is "\r\n", LF
// otherwise (and for an empty file). For CRLF, Merge first reads block's
// "\r\n" as "\n", then writes every "\n" as "\r\n", so a block that already
// holds CRLF never becomes "\r\r\n" (Render's output holds no "\r"; a caller
// passing a CRLF block is still exact). So Merge then Strip is byte-exact for
// a CRLF file too, and no merge leaves a file of mixed endings it did not
// already have. A lone "\r" is not a line ending here.
func Merge(existing, block string) (string, error)

// Strip removes the managed block and the one line ending after it, "\n" or
// "\r\n", so no "\r" is left behind. When the block is the last thing in the
// file (nothing follows it but that line ending, if it has one), it also
// removes the blank line
// Merge put before an appended block: the one line ending ("\n" or "\r\n")
// just before BeginPrefix, and only when that line ending is itself preceded
// by another. So "A\n\n<block>\n" becomes "A\n", and "A\r\n\r\n<block>\r\n"
// becomes "A\r\n" (b10 re-review M-6). When anything follows the block, it
// sits between a person's paragraphs and nothing before it is removed:
// "A\n\n<block>\nB\n" becomes "A\n\nB\n" (here <block> ends at the end
// marker; the "\n" after it is the one Strip removes). A file that did not end in a line
// ending before Merge gets one back, because Merge gave "A" and "A\n" the
// same bytes. No marker of either kind is not an error. The same failures,
// by the same count, as Merge.
func Strip(existing string) (string, error)

// Marker is what a begin marker names.
type Marker struct {
	BriefID  string
	Revision int
}

// Block is the managed block found in a file.
type Block struct {
	Marker Marker
	Text   string // from BeginPrefix through EndMarker and its line ending, verbatim (a CRLF block keeps its "\r")
}

// Inspect finds the block. ok is false when there is no marker of either
// kind; a stray or early end marker is ErrUnterminatedBlock, by the same
// count as Merge, never an `edited` block. Markers match as in Merge, with
// \r?\n (review M7). Its only use is
// the drift check (contract §3.3 condition 3): NOTHING reads brief text back
// out of a file as a source.
func Inspect(existing string) (b Block, ok bool, err error)

// SameText reports whether got and want are the same lines, split as
// DiffLines splits them: on \r?\n, a final line ending ending the last line
// rather than starting another. It is the drift check's only comparison of a
// found block with a fresh render (task 09's current/edited split), so a
// block an editor re-saved with CRLF is current, not edited over every line
// (review M7), and so is one whose end marker lost the file's final line
// ending (task 05 fix round 1, review M1): SameText is false exactly when
// DiffLines, but for its 400-line cap, shows a change. Any other byte
// difference, a lone "\r" included, is a difference.
func SameText(got, want string) bool

type DiffOp string

const (
	DiffSame    DiffOp = "same"
	DiffAdded   DiffOp = "added"   // in the file, not in a fresh render
	DiffRemoved DiffOp = "removed" // in a fresh render, not in the file
)

type DiffLine struct {
	Op   DiffOp `json:"op"`
	Text string `json:"text"`
}

// DiffLines is a line diff (longest common subsequence) of want against got,
// capped at 400 lines of output, for the "block edited by hand" view. Lines
// are split on \r?\n and compared, and returned, without their ending, so a
// CRLF block's diff shows only the lines whose text changed (review M7).
func DiffLines(want, got string) []DiffLine
```

**The golden branch block** — `testdata/golden-branch.md`, the fact sheet's `b-42@5` resolved
against its pinned `b-7@5` (fact sheet §2–§3), with Hub-shaped ids. The test builds the two
revisions in Go and compares bytes; **fix the renderer, never the golden file**:

```markdown
<!-- panacea:discover:begin brief=bf-00000000000000a2 rev=5 -->
# Branch brief — evidence-upload

Written by Discover in Panacea App. Every session started on this branch reads it first.
Change it in Discover, not here: this block is regenerated and your edits inside it will be lost.

## What this is for
_Set on this branch, replacing the project brief: Client-facing portal for audit engagements: requests, evidence and status._

Let client users upload evidence files against an open request.

## Who it is for
_Inherited from the project brief._

KPMG staff broadly

## What must not break
_Set on this branch, replacing the project brief: Existing client data; the published API_

Existing uploads and their download links

## Definition of done
_Set on this branch, replacing the project brief: Tests pass; No change to the published API without a version bump_

- Upload works for PDF, XLSX and images
- A virus scan runs before any file is visible
- Tests pass

## Out of scope
_Set on this branch, replacing the project brief: Billing and invoicing_

Bulk upload; the reviewer side of evidence

---

Brief bf-00000000000000a2, revision 5. Inherits project brief bf-00000000000000b7, revision 5.
<!-- panacea:discover:end -->
```

The project golden (`golden-project.md`) is `b-7@5` alone: every section carries
`_Inherited from the project brief._`, the title is `# Project brief` (no project name and no
branch name: the block is a function of its key, P4-G as amended by review I5), and the footer is
`Brief bf-00000000000000b7, revision 5.` with no second sentence. The same project key therefore
renders the same bytes on every branch and every teammate's machine.

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestResolveBranchOverProjectGolden` | `Render(Resolve(…))` equals `golden-branch.md` byte for byte | any drift in wording, order or spacing |
| `TestResolveProjectAloneGolden` | equals `golden-project.md` | a project render that prints a branch-only line |
| `TestTitleComesFromTheRevision` (review I5) | branch `BranchRef` `refs/heads/evidence-upload` → `Title` `Branch brief — evidence-upload`; a project revision → `Project brief`; `Title` of an **open** branch revision on the same ref → `Branch brief — evidence-upload`, equal to `Resolve`'s title for its complete successor (re-review b05-1 R-M2); `Title(nil)` → `Project brief`; a Go-level check that `Resolve`'s signature is `func(project, branch *Revision) (Document, error)` (no name can be passed) | a title read from the project's or the branch's current name, which a rename changes under an unchanged key (the rename end to end is task 07's `TestRenameDoesNotLookLikeAHandEdit`); a `Title` that refuses or blanks an open revision, which leaves `provenance.Build` to hand-write the rule a second time |
| `TestResolveRefusesAProjectRevisionTheBranchDidNotPin` | branch `Inherits` = `…b7@5`, project passed = `…b7@6` → `ErrInheritMismatch` | a caller that resolves against the project head — critique-2 §1.1 |
| `TestResolveRefusesAnIncompleteRevision` | an open branch revision → `ErrIncomplete` | materialising half a brief |
| `TestSkippedBranchAnswerInheritsTheProject` | branch skips audience → section `OriginProject` with the project's text | a skip rendered as an empty section |
| `TestSkippedProjectAnswerIsOmitted` | project skips out-of-scope, branch silent → no "Out of scope" heading | an empty heading an agent reads as "nothing is out of scope" |
| `TestDraftedTextIsNeverRendered` | a revision constructed with a `model_drafted` answer and status forced complete: that answer's text is absent | `4-D2` broken by a caller that forged status |
| `TestRenderIsDeterministic` (kept) | two renders are equal | a timestamp in the block |
| `TestMergeReplacesOnlyTheManagedBlock` (kept, `:1941`) | text above and below survives; one begin marker | a merge that eats the tail |
| `TestMergeAppendsAfterABlankLine` (kept) | user content first, then `\n\n`, then the block | a block glued to the last line of prose |
| `TestMergeAndStripFailClosed` | unterminated and duplicate blocks → their errors, input unchanged | the guess that deletes a person's writing |
| `TestRenderNeutralisesCommentOpeners` | a Document whose answer text, criterion text, follow-up prompt and title (from a `BranchRef` of `refs/heads/<!--x` — `<` and `!` are legal in a git ref) each hold `<!-- panacea:discover:end -->` or `<!-- x -->` renders to a checked-in golden (`testdata/golden-neutralised.md`) in which each appears as `<!-\- …`; `strings.Count(block, "<!--") == 2`; `Inspect(Render(doc))` is ok with the doc's marker, and `Strip(Merge("", block))` is `""` | the escape dropped, or applied to answers but not to prompts or the title (review C2) |
| `TestStrayEndMarkerIsBroken` | table, for each of `Merge`, `Strip` and `Inspect`: an end marker with no begin; an end before the begin; begin, end, then a second end (the forged-early-end shape: begin, forged end, brief tail, real end); a begin, a begin and one end → `ErrUnterminatedBlock` for the first three and `ErrDuplicateBlock` for the last; the input is unchanged; a file with no marker of either kind is still `ok == false` / unchanged with no error | a strip that removes up to the first end and leaves the brief's tail behind as "user text", a merge that re-appends that tail on every start, or an Inspect that reports it as `edited` for ever |
| `TestStripRemovesTheBlockAndNothingElse` | Merge then Strip returns the original, byte for byte, for three shapes (`<block>` is Render's output, its trailing line ending included): an empty file; text above (`A\n`, where Merge gives `A\n\n<block>` and Strip removes the blank line with the block); text above and below (spelled in Go, as the test does: `"A\n\n" + old + "B\n"`, where `old` is an earlier render, its trailing line ending included; Merge gives `"A\n\n" + block + "B\n"`, and Strip gives `"A\n\nB\n"`, removing nothing before a block that text follows. b10 re-review R-I1: the row once read `A\n\n<old block>\nB\n`, which with `<block>` holding its line ending demands the Strip that eats the blank line above; a file with a real blank line between the block and B, `"A\n\n" + block + "\nB\n"`, Strips to `"A\n\n\nB\n"`). **Blank-line rows (b10 re-review M-6):** `A` with no final line ending Merges to `A\n\n<block>` and Strips to `A\n`; an appended block whose blank line the person deleted (`A\n<block>`) Strips to `A\n`, with no line of `A` eaten; a file that begins with the block (`<block>B\n`) Strips to `B\n`. **CRLF rows (review M7):** the two non-empty shapes with `\r\n` endings: Merge's output has no `\n` that is not preceded by `\r`, and Strip returns the original byte for byte; and a file Merged with LF, then re-saved with every `\n` turned to `\r\n` (the editor case), Strips to the CRLF original with no `\r` left anywhere a line ending was removed | a strip that leaves the appended block's blank line behind, or removes the blank line above a block that text follows (eating a person's paragraph break); a marker matched on `\n` only, which leaves a `\r` behind or misses a CRLF end marker; a Merge that writes an LF block into a CRLF file |
| `TestCRLFResaveIsNotAnEdit` (review M7) | `Render(doc)` with every `\n` turned to `\r\n`: `Inspect` is ok with the doc's marker, `SameText(b.Text, Render(doc))` is true, and `DiffLines(Render(doc), b.Text)` is all `same` with no `\r` in any `Text`; the same file with one line's text changed: `SameText` false and exactly one `removed` and one `added`; `SameText("a\rb\n", "ab\n")` is false | a drift check that compares raw bytes, so every CRLF re-save is `edited` over every line; a normaliser that strips every `\r`, not just the one before `\n` |
| `TestRenderWritesOnlyLF` (b10 re-review M-5) | a Document built straight from a head the Hub stored unchecked, whose answer text, criterion text and follow-up prompt hold `"x\r\ny\rz"`: `Render` holds no `"\r"`, and those lines read `x`, `y`, `z`; `Merge("A\r\n", block)` has every line ending `"\r\n"`, no `"\r\r\n"`, and `SameText` of its `Inspect` block with `Render(doc)` is true; `Merge("A\n", block)` holds no `"\r"`; and `Merge("A\r\n", block with CRLF)` equals `Merge("A\r\n", block)` | a Merge that converts with `strings.ReplaceAll(block, "\n", "\r\n")` over a block that already holds `"\r"`, giving `"\r\r\n"` and a block `edited` for ever; a Render that passes `"\r"` through into an LF file, leaving it with mixed endings |
| `TestInspectReadsTheMarker` | `brief=bf-…a2 rev=5` → `Marker{"bf-…a2", 5}`; `rev=05`, `rev=0`, a missing `rev` → `ErrBadMarker` | a lenient parser that accepts a marker no render produced |
| `TestDiffLinesShowsAHandEdit` | changing one line of a rendered block gives exactly one `removed` and one `added` in place | a diff that reports the whole block changed |

**Steps:**

- [ ] **Step 1:** Write both test files and the three golden files. Confirm they fail to compile.
- [ ] **Step 2:** `inherit.go`: port the old `Resolve` (`:1787-1831`) to revisions; add the
  pinned-revision check, the skip, draft and completeness rules; follow-up headings from
  `Prompt` minus `?` (`:1745-1762`); `Title`, which `Resolve` calls for `Document.Title`.
- [ ] **Step 3:** `render.go`: port `Render` and `Merge` (`:2030-2101`); the marker is
  `brief=<briefId> rev=<n>`; items render from `Section.Items[i].Text`; add the footer's second
  sentence; add `Strip`, `Inspect`, `DiffLines`. The old `Merge` finds the first begin and the
  first end after it; replace that with the marker count above, shared by `Merge`, `Strip` and
  `Inspect` through one unexported `locate(existing) (begin, end int, err error)`, and run every
  interpolated string through the `<!--` neutraliser (review C2). `locate`'s `end` is past the
  end marker's `\r?\n`; add `SameText`, and give `Merge` the file's line ending (review M7).
  Write `testdata/.gitattributes` before the golden files are first committed.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discover/... ./control-plane/internal/claudemd/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd
$ grep -n "time\." control-plane/internal/claudemd/render.go
(no output — the renderer cannot reach a clock)
$ grep -c "<!--" control-plane/internal/claudemd/testdata/golden-neutralised.md
2
```

**Does NOT claim:** that Claude Code reads this block (task 23's canary, and the owner's batched
step); where the block is written (task 09).

**As built** (task t05, commit `eb6075d` and after; report `.superpowers/sdd/p4/task-t05-report.md`).
Where this differs from the text above, this wins:

1. **The override note is one line.** `Section.Replaced` for the definition of done is the project's
   criteria joined by `"; "` (the golden's `Tests pass; No change to the published API without a
   version bump`, from two criteria typed on two lines); for every other key it is the project
   answer's text. `Render` joins the note's lines with one space, dropping empty ones, so a
   multi-line project answer cannot break the italic line (`TestOverrideNoteIsOneLine`).
2. **Only a settled, non-skipped answer carries text.** The draft rule, generalised: a skip, a held
   draft, or an answer with no text or no person or accepted author is no answer. So a branch
   draft inherits the project's answer (`OriginProject`), and a project draft under a silent branch
   is omitted. A `model_accepted` answer renders (`TestDraftedTextIsNeverRendered`).
3. **Items come from the answer's `Criteria`**, ids kept, never re-split from the text; every other
   section's `Items` is `[]`, and `Document.Sections` is `[]`, never null.
4. **A malformed revision is a server fault.** `Resolve(nil, nil)`, a revision in the other scope's
   place, a `BriefID` that fails `MarkerSafe` (empty, or holding white space by `unicode.IsSpace`,
   `<` or `>`), a `Number` below 1, and (fix round 1, review M2) a `BranchRef` no Hub envelope
   holds are the unexported `errBadRevision` (a 500, like task 03's `errNoNewID`), with a zero
   `Document`, checked before completeness and the pin. The `BranchRef` rule: a project
   revision's is `""`; a branch revision's is `refs/heads/` and a non-empty name holding no ASCII
   control character, space or DEL (Hub contract §3.3 `:133-137`, §7.5 `:759`, and its `briefs` table's
   `CHECK ((scope = 'project' AND branch_ref = '') OR (scope = 'branch' AND branch_ref LIKE
   'refs/heads/_%' …))` at `:1573-1574`), so the title is one line and names a branch.
   So every block `Render(Resolve(…))` writes reads back through `Inspect`
   (`TestResolveRefusesAMalformedRevision`). **The id rule has one copy** (fix round 1, review
   I1): the exported `discover.MarkerSafe`, which `Resolve` applies to both ids and `claudemd`'s
   begin-marker parser calls, so the two sides cannot drift (`TestMarkerSafe`; `claudemd`'s
   `TestEveryResolvedIDReadsBack`: for each of the review's candidate ids, `Resolve` succeeds
   exactly when `Inspect` accepts a begin marker holding it, and then `Inspect(Render(doc))` names
   it). Round 1 had two copies, `discover.markerSafe` and `claudemd.markerID`, each pinned only
   for `<`, a space and `""`: the review's RV1, RV1c and RV2 dropped `>` or narrowed white space to
   `' '` in either copy, and both packages stayed green.
5. **Every question skipped** still renders the key: the intro, then `No question in this brief has
   an answer.`, then the footer (`TestAllSkippedRendersTheKeyAndNoSection`). The old plan's `No
   brief has been recorded for this branch yet.` would be false for a complete brief; the wording is
   the owner's to accept (OD-12).
6. **Refusals change nothing, visibly.** `Merge` and `Strip` return `existing` unchanged beside the
   error (not `""`, which a caller writing the result would turn into an empty file); `Inspect`
   returns a zero `Block` on every error.
7. **The begin marker's line is exactly** `BeginPrefix + " brief=" + id + " rev=" + n + " -->"`,
   then `\r?\n`: `id` passes `discover.MarkerSafe` (As built 4); `n` matches `[1-9][0-9]*` and fits
   an `int`. Anything else is `ErrBadMarker` (20 rows in `TestInspectReadsTheMarker`: 16 at
   `eb6075d`, V4's at `c2005fd`, and `>`, a tab and a no-break space in the id at fix round 1).
8. **`DiffLines`** trims the common prefix and suffix, runs the LCS on the middle, bounded at 2^20
   cells (past it: every render line removed, then every file line added, still a true diff), puts
   the render's line (removed) before the file's (added) at a change, and is never nil. The 400-line
   cap counts from the top.
9. **A follow-up's heading** is its prompt for the holding revision's scope, trimmed, minus a final
   `?`; the key itself when that leaves nothing. When both revisions hold the key, the revision
   whose answer the section shows is asked first: an `OriginProject` section is headed in the
   project's words, an override or branch section in the branch's (mutation round 1's X7 found the
   old plan's branch-first rule heading an inherited project answer with the branch's wording;
   fixed in `c2005fd`). Pinned by `TestFollowUpHeadingIsTrimmedOrTheKey` (fix round 1, review I2):
   `"  Is there a deadline?  "` is headed `Is there a deadline`; `"?"`, `"  ?  "` and `""` are
   headed by the key. The other revision is asked only when the shown one lacks the question, so
   a key both hold, whose shown prompt leaves nothing, is headed by the key, not by the other
   scope's wording (the fix round's predicted survivor P2; ops.go refuses a blank prompt, so only
   a `?`-only prompt or an unchecked head reaches this).
10. **Tests beyond the table:** `TestResolveOverrideCarriesTheReplacedText`,
    `TestResolveOrdersSectionsAndHeadsFollowUps`, `TestResolveNamesTheKeyAndThePin`,
    `TestResolveRefusesAMalformedRevision`, `TestAllSkippedRendersTheKeyAndNoSection`,
    `TestOverrideNoteIsOneLine`, `TestDiffLinesIsCapped`, `TestDiffLineWireShape` (the `op` strings
    are wire values task 12 carries); and at fix round 1 `TestMarkerSafe`,
    `TestFollowUpHeadingIsTrimmedOrTheKey`, `TestEveryResolvedIDReadsBack` and
    `TestSameTextAgreesWithDiffLines`. The Strip row's third shape is spelled in Go (b10 re-review
    R-I1, above). `TestMergeAppendsAfterABlankLine` has two mixed-ending rows: the first line
    ending decides. Fix round 1 rows (review I2), each the rule a surviving mutant broke: `Merge`
    of a file that starts with a line ending (`"\nA\n"`, `"\n"`, `"\r\nA\r\n"`; RV3's `i >= 0`
    panicked on them); `Strip("A\n\n" + block + "\n")` is `"A\n\n\n"`, and its CRLF twin, because
    a line ending after the block's own is something that follows it (RV11); and the override
    note stays one line over a project text holding `"\r\n"`, a lone `"\r"`, or both with a blank
    line, because the note is cleaned before it is joined (RV15).
11. **`SameText` is `DiffLines`' notion of the same** (fix round 1, review M1). Both split on
    `\r?\n`, and a final line ending ends the last line rather than starting another, so
    `SameText(got, want)` is `slices.Equal(splitLines(got), splitLines(want))`: false exactly when
    `DiffLines`, but for its cap, shows a change. A block whose end marker is the file's last byte
    (an editor that drops the final newline, or text typed on the end marker's line) is therefore
    `current`, where round 1's `SameText` called it edited over a diff with no change in it. A lone
    `\r` is still a difference, and a blank line added after the block is outside `Block.Text`
    (`TestSameTextAgreesWithDiffLines`; task 09's `StateCurrent` comment amended to match).
