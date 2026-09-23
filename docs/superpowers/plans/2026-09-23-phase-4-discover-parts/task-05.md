### Task 05: `discover.Resolve` with a pinned project revision; `claudemd`

Project → branch → session. This task turns **one** brief key's worth of immutable text into the
managed block an agent reads. **Kept from the 2026-09-18 plan's task 4** (`:1572-2110`): `Origin`,
`Section`, the heading table and file order (`:1734-1743`), the override rule and its wording,
deterministic render with no timestamp, `Merge` that keeps every byte outside the markers and fails
closed on an unterminated block (`:2073-2101`). **Changed:** `Resolve` takes two *revisions*, not
two mutable briefs, and refuses a project revision the branch did not pin (`4-D3`); skipped and
drafted answers have rules; `claudemd` gains `Strip`, `Inspect` and `DiffLines`; the footer names
the inherited revision.

**Files:**
- `control-plane/internal/discover/inherit.go` + `inherit_test.go` (new)
- `control-plane/internal/claudemd/render.go` + `render_test.go` (new)
- `control-plane/internal/claudemd/testdata/golden-branch.md`, `golden-project.md` (new)

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
	Content Content
}

type Names struct {
	Project string // project.Scanner.ProjectName
	Branch  string // project.Scanner.BranchName; "" for a detached worktree
}

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
	Title           string    `json:"title"`   // "Engagement Portal — evidence-upload"; the project name alone when Names.Branch is ""
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
func Resolve(n Names, project, branch *Revision) (Document, error)
```

```go
// control-plane/internal/claudemd/render.go
package claudemd

const (
	BeginPrefix = "<!-- panacea:discover:begin"
	EndMarker   = "<!-- panacea:discover:end -->"
)

var (
	ErrUnterminatedBlock = errors.New("claudemd: begin marker with no end marker")
	ErrDuplicateBlock    = errors.New("claudemd: more than one managed block")
	ErrBadMarker         = errors.New("claudemd: begin marker is not brief=<id> rev=<n>")
)

// Render returns the managed block, both markers and a trailing newline.
// Deterministic: the same Document gives the same bytes, and nothing in it
// depends on the clock (a timestamp would rewrite the file on every start).
func Render(doc discover.Document) string

// Merge splices block into existing: appended after a blank line when there
// is no block, replacing the one block when there is, every other byte kept.
// Two begin markers is ErrDuplicateBlock; a begin with no end is
// ErrUnterminatedBlock. Both change nothing: the guess would delete somebody's
// writing.
func Merge(existing, block string) (string, error)

// Strip removes the managed block and the one newline after it. No block is
// not an error. The same two failures as Merge.
func Strip(existing string) (string, error)

// Marker is what a begin marker names.
type Marker struct {
	BriefID  string
	Revision int
}

// Block is the managed block found in a file.
type Block struct {
	Marker Marker
	Text   string // from BeginPrefix through EndMarker and its newline, verbatim
}

// Inspect finds the block. ok is false when there is none. Its only use is
// the drift check (contract §3.3 condition 3): NOTHING reads brief text back
// out of a file as a source.
func Inspect(existing string) (b Block, ok bool, err error)

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
// capped at 400 lines of output, for the "block edited by hand" view.
func DiffLines(want, got string) []DiffLine
```

**The golden branch block** — `testdata/golden-branch.md`, the fact sheet's `b-42@5` resolved
against its pinned `b-7@5` (fact sheet §2–§3), with Hub-shaped ids. The test builds the two
revisions in Go and compares bytes; **fix the renderer, never the golden file**:

```markdown
<!-- panacea:discover:begin brief=bf-00000000000000a2 rev=5 -->
# Engagement Portal — evidence-upload

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
`_Inherited from the project brief._`, the title is the project name and the branch name, and the
footer is `Brief bf-00000000000000b7, revision 5.` with no second sentence (P4-G).

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestResolveBranchOverProjectGolden` | `Render(Resolve(…))` equals `golden-branch.md` byte for byte | any drift in wording, order or spacing |
| `TestResolveProjectAloneGolden` | equals `golden-project.md` | a project render that prints a branch-only line |
| `TestResolveRefusesAProjectRevisionTheBranchDidNotPin` | branch `Inherits` = `…b7@5`, project passed = `…b7@6` → `ErrInheritMismatch` | a caller that resolves against the project head — critique-2 §1.1 |
| `TestResolveRefusesAnIncompleteRevision` | an open branch revision → `ErrIncomplete` | materialising half a brief |
| `TestSkippedBranchAnswerInheritsTheProject` | branch skips audience → section `OriginProject` with the project's text | a skip rendered as an empty section |
| `TestSkippedProjectAnswerIsOmitted` | project skips out-of-scope, branch silent → no "Out of scope" heading | an empty heading an agent reads as "nothing is out of scope" |
| `TestDraftedTextIsNeverRendered` | a revision constructed with a `model_drafted` answer and status forced complete: that answer's text is absent | `4-D2` broken by a caller that forged status |
| `TestRenderIsDeterministic` (kept) | two renders are equal | a timestamp in the block |
| `TestMergeReplacesOnlyTheManagedBlock` (kept, `:1941`) | text above and below survives; one begin marker | a merge that eats the tail |
| `TestMergeAppendsAfterABlankLine` (kept) | user content first, then `\n\n`, then the block | a block glued to the last line of prose |
| `TestMergeAndStripFailClosed` | unterminated and duplicate blocks → their errors, input unchanged | the guess that deletes a person's writing |
| `TestStripRemovesTheBlockAndNothingElse` | Merge then Strip returns the original, byte for byte, for three shapes (empty file, text above, text above and below) | a strip that leaves a blank line or eats one |
| `TestInspectReadsTheMarker` | `brief=bf-…a2 rev=5` → `Marker{"bf-…a2", 5}`; `rev=05`, `rev=0`, a missing `rev` → `ErrBadMarker` | a lenient parser that accepts a marker no render produced |
| `TestDiffLinesShowsAHandEdit` | changing one line of a rendered block gives exactly one `removed` and one `added` in place | a diff that reports the whole block changed |

**Steps:**

- [ ] **Step 1:** Write both test files and the two golden files. Confirm they fail to compile.
- [ ] **Step 2:** `inherit.go`: port the old `Resolve` (`:1787-1831`) to revisions; add the
  pinned-revision check, the skip, draft and completeness rules; follow-up headings from
  `Prompt` minus `?` (`:1745-1762`).
- [ ] **Step 3:** `render.go`: port `Render` and `Merge` (`:2030-2101`); the marker is
  `brief=<briefId> rev=<n>`; items render from `Section.Items[i].Text`; add the footer's second
  sentence; add `Strip`, `Inspect`, `DiffLines`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discover/... ./control-plane/internal/claudemd/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd
$ grep -n "time\." control-plane/internal/claudemd/render.go
(no output — the renderer cannot reach a clock)
```

**Does NOT claim:** that Claude Code reads this block (task 23's canary, and the owner's batched
step); where the block is written (task 09).
