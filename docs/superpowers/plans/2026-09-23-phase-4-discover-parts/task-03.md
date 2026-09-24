### Task 03: `discover` — the content model and its ops

Pure logic, no I/O, no model, no Hub. This task defines **the object phase 4 writes into every Hub
brief revision** (`content`, Hub contract §7.5), and the one function that changes it. Everything
downstream — the render, the key's meaning, the chip, the pin — is a function of this object, so
it is written here as complete source and nowhere else.

**What it keeps from the 2026-09-18 plan's task 2** (`:583-1210`): `BaseQuestions()` word for word
(`:872-935`), the one permitted denominator (`QUESTION n OF 5`, follow-ups carry none, `:587`),
append-or-replace-in-place for re-answering (known defect 7, `FUNCTIONAL-SPEC.md`; `:1087-1123`),
and `SplitCriteria` (`:1186-1202`).

**What changes, and why:**

| Old | New | Why |
|---|---|---|
| `Brief` with `Revision` and `Revision++` in every method | `Content` with no revision: the Hub numbers revisions | change C2 |
| `Skip()` closes the whole brief (`StatusSkipped`) | **per-question** skip: *Keep the project answer* on a branch, *Skip this question* on a project (prototype `:483`). v1 never writes brief status `skipped` | the prototype and the fact sheet (rev 2 is *"audience skipped"*) |
| `Source` only | `Author`: `person` · `model_drafted` · `model_accepted`. A `model_drafted` answer is **not settled** | `4-D2` |
| `criterionID(briefID, index)` | `Criterion{ID, Text}` stored on the done answer, ids minted once and kept by exact text | brief R6 |
| `Reopen` | gone: status is derived from the answers on every op | a stored status that can disagree with the answers is a second truth |
| — | `Inherits`: the project revision a branch revision resolves against | `4-D3` |
| — | `Change`: what produced the revision, for history rows | the prototype's *"Definition of done edited"* rows without diffing two revisions |
| `Answer.Prompt` | dropped — the question text is in `Questions` on the same revision | one copy |

**Files:**
- `control-plane/internal/discover/content.go` + `content_test.go` (new)
- `control-plane/internal/discover/ops.go` + `ops_test.go` (new)

**Interfaces — complete source:**

```go
// control-plane/internal/discover/content.go

// Package discover turns intent into durable context: the Brief. Its content
// object is what phase 4 writes into every Hub brief revision, and a
// revision is immutable and kept for ever - so this file is a WIRE FORMAT.
// Field names, JSON tags and the rule "every slice marshals as [], never
// null" are part of every digest ever computed. Change them only by adding a
// new ContentVersion that every reader also understands.
package discover

// ContentVersion is the version this build writes. Readers accept every
// version <= ContentVersion; v1 is the only one.
const ContentVersion = 1

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeBranch  Scope = "branch"
)

// Key identifies one question. Base keys are fixed; a follow-up's key is
// "f-" + 8 hex, minted when the follow-up is added.
type Key string

const (
	KeyPurpose     Key = "purpose"
	KeyAudience    Key = "audience"
	KeyConstraints Key = "constraints"
	KeyDone        Key = "done"
	KeyOutOfScope  Key = "out_of_scope"
)

// Status is derived, never set by hand: complete exactly when every question
// has a settled answer. The Hub also accepts "skipped"; v1 never writes it.
type Status string

const (
	StatusOpen     Status = "open"
	StatusComplete Status = "complete"
)

type Source string

const (
	SourcePicked  Source = "picked"
	SourceTyped   Source = "typed"
	SourceDrafted Source = "drafted"
)

// Author says who wrote an answer (4-D2). A model_drafted answer is held:
// it is shown, marked, and NOT settled, so a revision holding one is never
// complete and is never inherited, pinned or approved against.
type Author string

const (
	AuthorPerson        Author = "person"
	AuthorModelDrafted  Author = "model_drafted"
	AuthorModelAccepted Author = "model_accepted" // a person accepted the model's text unchanged
)

type Question struct {
	Key           Key      `json:"key"`
	ProjectPrompt string   `json:"projectPrompt"`
	BranchPrompt  string   `json:"branchPrompt"`
	Help          string   `json:"help"`
	Options       []string `json:"options"`
	FollowUp      bool     `json:"followUp"`
	Because       string   `json:"because"` // a follow-up's reason, e.g. "Suggested because of your answer to …"; "" for base questions
}

func (q Question) Prompt(s Scope) string // the old plan's rule, :861-866

// BaseQuestions is the closed set, word for word from the 2026-09-18 plan
// (:872-935). Closed on purpose: "QUESTION 2 OF 5" is honest only because
// this list cannot grow mid-flow.
func BaseQuestions() []Question

// Criterion is one line of a definition of done. ID is minted once and kept
// for as long as a line with exactly this Text survives edits (R6); phase 3
// keys evidence by brief key + ID, and by the text digest provenance adds.
type Criterion struct {
	ID   string `json:"id"` // "c-" + 8 hex
	Text string `json:"text"`
}

type Answer struct {
	Key         Key         `json:"key"`
	Text        string      `json:"text"` // "" exactly when Skipped
	Skipped     bool        `json:"skipped"`
	Source      Source      `json:"source"` // "" when Skipped
	Author      Author      `json:"author"`
	Criteria    []Criterion `json:"criteria"`    // KeyDone only; [] for every other key and when skipped
	DraftedFrom []string    `json:"draftedFrom"` // repo on-ramp: the manifest and directory names the draft was read from; [] otherwise
	AnsweredAt  string      `json:"answeredAt"`  // RFC 3339 UTC, exactly three fractional digits (Hub contract §1)
}

// Settled reports whether an answer counts towards completion: skipped, or
// text from a person or accepted by one.
func (a Answer) Settled() bool

// Inherit pins the project revision a branch revision resolves against
// (4-D3). With it, a branch key names immutable resolved text.
type Inherit struct {
	Key    string `json:"key"`    // "brief:bf-…@n"
	Digest string `json:"digest"` // that revision's Hub digest
}

type ChangeKind string

const (
	ChangeRecord          ChangeKind = "record"
	ChangeEdit            ChangeKind = "edit"
	ChangeSkip            ChangeKind = "skip"
	ChangeAccept          ChangeKind = "accept"
	ChangeAddFollowUp     ChangeKind = "add_follow_up"
	ChangeDismissFollowUp ChangeKind = "dismiss_follow_up"
	ChangeRestore         ChangeKind = "restore"
	ChangeReinherit       ChangeKind = "reinherit"
	ChangeDraft           ChangeKind = "draft"
)

// Change records what produced a revision. It is read by the history view
// and never rendered into CLAUDE.md.
type Change struct {
	Kind     ChangeKind `json:"kind"`
	Key      Key        `json:"key"`      // "" for restore, reinherit, draft
	Restores int        `json:"restores"` // restore: the revision whose answers came back; 0 otherwise
}

// Content is the Hub revision's content object, version 1.
type Content struct {
	V         int        `json:"v"`
	Status    Status     `json:"status"`
	Questions []Question `json:"questions"`
	Answers   []Answer   `json:"answers"`
	Inherits  *Inherit   `json:"inherits"` // branch: the project's latest complete revision when written, or null; project: always null
	Change    Change     `json:"change"`
}

// NewContent is the content before the first op: the base questions, no
// answers, status open. It is never written itself - revision 1 is the
// first op applied to it.
func NewContent() Content

// Decode reads a content object from any version this build understands.
// An unknown higher version is ErrUnknownVersion, never a guess.
func Decode(raw []byte) (Content, error)

func (c Content) AnswerFor(k Key) (Answer, bool)
func (c Content) Pending() []Key // model_drafted answers awaiting a person, in question order

type StepKind string

const (
	StepQuestion StepKind = "question"
	StepFollowUp StepKind = "follow_up"
	StepReview   StepKind = "review" // only drafted answers remain; accept or change each
	StepComplete StepKind = "complete"
)

// Step is what the flow shows next. Total is the size of the closed base set
// for StepQuestion and ZERO otherwise; the only legal rendering of it is the
// literal "QUESTION n OF m" (G5). Ordinal is likewise zero for every kind
// but StepQuestion. Pending is c.Pending() on every kind, so a question step
// can say drafts are waiting; Options and Pending are never nil.
type Step struct {
	Kind    StepKind `json:"kind"`
	Ordinal int      `json:"ordinal"`
	Total   int      `json:"total"`
	Key     Key      `json:"key"`
	Prompt  string   `json:"prompt"`
	Help    string   `json:"help"`
	Options []string `json:"options"`
	Because string   `json:"because"`
	Pending []Key    `json:"pending"`
}

// Step returns the next thing to show. It walks the questions in order and
// stops at the first whose answer is neither settled nor a pending draft; a
// model_drafted answer does not stop the walk (it waits for review) and does
// not count as settled. With no such question left it is StepComplete when
// the derived Status is complete, and StepReview otherwise: while drafts are
// pending, and for a content with no questions (a head or a restored revision
// written straight to route 26), which has nothing to ask and is never
// complete. So StepComplete holds exactly when the derived Status is complete.
func (c Content) Step(s Scope) Step

// SplitCriteria is the old plan's (:1186-1202), unchanged.
func SplitCriteria(text string) []string

// FormatTime is the one time format inside content.
func FormatTime(t time.Time) string // t.UTC().Format("2006-01-02T15:04:05.000Z")

var ErrUnknownVersion = errors.New("discover: content version is newer than this build")
```

```go
// control-plane/internal/discover/ops.go
package discover

type OpKind string

const (
	OpRecord          OpKind = "record"            // Key, Text, Source (picked | typed)
	OpEdit            OpKind = "edit"              // Key, Text — a settled, non-skipped answer, in place
	OpSkip            OpKind = "skip"              // Key
	OpAccept          OpKind = "accept"            // Key — model_drafted -> model_accepted, text unchanged
	OpAddFollowUp     OpKind = "add_follow_up"     // Question (its Key is minted here)
	OpDismissFollowUp OpKind = "dismiss_follow_up" // Key — a follow-up; removes the question and its answer
	OpRestore         OpKind = "restore"           // Restore, RestoreRevision — questions and answers of an earlier revision
	OpReinherit       OpKind = "reinherit"         // branch only; the 4-D3 fan-out
	OpDraft           OpKind = "draft"             // Drafts — the repo on-ramp
)

// Op is one change a person (or the fan-out, or the on-ramp) asked for. The
// JSON tags are the wire shape the desktop sends inside POST …/brief/ops
// (task 11); Restore and Drafts are filled by the server, never the client.
type Op struct {
	Kind            OpKind    `json:"kind"`
	Key             Key       `json:"key,omitempty"`
	Text            string    `json:"text,omitempty"`
	Source          Source    `json:"source,omitempty"`
	RestoreRevision int       `json:"restoreRevision,omitempty"`
	Question        *Question `json:"-"`
	Restore         *Content  `json:"-"`
	Drafts          []Answer  `json:"-"`
}

// Env is what Apply needs from outside and cannot compute.
type Env struct {
	Scope Scope
	// Inherits is the project brief's latest complete revision NOW. Set for
	// every op on a branch brief, so each branch revision pins what it
	// resolves against; nil for a project brief or when none is complete.
	Inherits *Inherit
	Now      time.Time
	NewID    func(prefix string) string // ids.New in production; nil is a server fault
}

var (
	ErrUnknownKey    = errors.New("discover: no question with that key")
	ErrEmptyAnswer   = errors.New("discover: an answer cannot be empty")
	ErrNotAnswered   = errors.New("discover: that question has no answer to edit")
	ErrNotDrafted    = errors.New("discover: that answer was not drafted by the model")
	ErrNotFollowUp   = errors.New("discover: that question is not a follow-up")
	ErrBadOp         = errors.New("discover: that change is not valid here")
	ErrNoChange      = errors.New("discover: that change leaves the brief as it was")
	// ErrReservedText: a text or prompt the op added or changed contains
	// ReservedMarker, the sequence the managed block's markers are made of
	// (plan §4 Security, S3). Apply returns it wrapped in *ReservedTextError,
	// so errors.Is(err, ErrReservedText) holds and the key is recoverable.
	ErrReservedText = errors.New("discover: text cannot contain the reserved sequence panacea:discover:")
	// ErrTooLong: an answer text the op added or changed is over
	// MaxAnswerRunes, or a prompt, help or option over MaxPromptRunes (rule
	// 13, review M6, plan §4 Security, S4). Apply returns it wrapped in
	// *TooLongError, so errors.Is(err, ErrTooLong) holds and the key is
	// recoverable.
	ErrTooLong = errors.New("discover: text is too long")
)

// The length caps (review M6), counted in Unicode code points
// (utf8.RuneCountInString) on the string as stored (rule 14's LF, then rule
// 1's trim for an answer; prompts, help and options are not trimmed; rule
// 13): the unit JSON Schema's maxLength uses, so task 04's and task 13's
// schemas (200/120 and 400) sit inside them and model text never meets the
// cap. They make the Hub's 64 KiB envelope refusal (hub-contract.md:751 at
// hub 53583f2) unreachable for ordinary text and give a person a refusal
// that names the answer; task 08's size check is the bound itself (S4).
const (
	MaxAnswerRunes = 4000
	MaxPromptRunes = 300
)

// TooLongError names the question whose text is over its cap, and the cap,
// so the 400 can point at the answer to shorten (task 11 puts Key and Limit
// in details; task 18 shows the message at that answer).
type TooLongError struct {
	Key   string
	Limit int // MaxAnswerRunes or MaxPromptRunes
}

func (e *TooLongError) Error() string {
	return ErrTooLong.Error() + " (question " + e.Key + ", at most " + strconv.Itoa(e.Limit) + " characters)"
}
func (e *TooLongError) Unwrap() error { return ErrTooLong }

// ReservedTextError names the question whose prompt, answer or criterion
// holds ReservedMarker, so the 400 can point at the answer to fix (task 11
// puts Key in details.key; task 18 shows the message at that answer).
type ReservedTextError struct{ Key string }

func (e *ReservedTextError) Error() string { return ErrReservedText.Error() + " (question " + e.Key + ")" }
func (e *ReservedTextError) Unwrap() error { return ErrReservedText }

// ReservedMarker is the prefix of every managed-block marker task 05 writes
// (`<!-- panacea:discover:begin …-->`, `<!-- panacea:discover:end -->`). No
// question prompt, answer text or criterion text may contain it.
const ReservedMarker = "panacea:discover:"

// Apply returns the content after op. It never mutates c. It sets V, the
// scope-appropriate Inherits, Change, and the derived Status on every
// result. It returns ErrNoChange - and the caller writes no revision - when
// questions, answers (ignoring AnsweredAt) and Inherits are all unchanged.
func Apply(c Content, op Op, env Env) (Content, error)
```

**The rules `Apply` enforces** (each is a test below):

1. `record` trims `Text` (after rule 14's line endings); empty is `ErrEmptyAnswer`. More than `MaxAnswerRunes` (4,000) code
   points after the trim is `&TooLongError{Key, Limit: 4000}` (rule 13, review M6). An unknown key
   is `ErrUnknownKey`. It
   replaces an existing answer **in place** (same index) or appends; it never touches another
   answer (defect 7). Author becomes `person`.
2. `edit` requires a settled, non-skipped answer (`ErrNotAnswered` otherwise), then behaves as
   `record` with `SourceTyped`.
3. `skip` replaces the answer with `{Skipped: true, Text: "", Source: "", Criteria: []}`, author
   `person`.
4. `accept` requires `AuthorModelDrafted` (`ErrNotDrafted`) and changes only the author, to
   `model_accepted`. Accepting unchanged text therefore **changes the digest** (critique-2 §2.7
   item 3).
5. For `KeyDone`, every op that sets text recomputes `Criteria` from `SplitCriteria(Text)`: an
   item whose text **exactly** equals an unused previous criterion keeps that criterion's id; any
   other item gets `env.NewID("c")`. Order follows the new text.
6. `add_follow_up` appends a question with `FollowUp: true` and a key `env.NewID("f")`; the base
   ordinals do not move.
7. `dismiss_follow_up` requires `FollowUp` (`ErrNotFollowUp`) and removes the question and its
   answer.
8. `restore` copies `Questions` and `Answers` from `op.Restore`; `Inherits` is `env.Inherits`,
   never the restored revision's — undo is about answers, and the project may have moved.
9. `reinherit` on a project brief is `ErrBadOp`; it changes nothing but `Inherits`.
10. `draft` sets each draft as `model_drafted` **only where the question has no answer by a
    person**; it never overwrites a person's answer.
11. Every result: `V = ContentVersion`; project scope → `Inherits = nil`; `Status = complete` iff
    every question has a settled answer; every slice non-nil.
12. **The marker rule (review C2, plan §4 Security, S3).** Before it returns, `Apply` checks every
    string **the op added or changed**: each question `Prompt`, answer `Text` and criterion `Text`
    in the result that has no counterpart in `c` (same question key, same answer key, same
    criterion id) or differs from it. That is the result, not the op's input, so a `draft` (model
    text), an `add_follow_up` prompt and a `restore` are covered by the same check. A string the
    op left as it was is **not** checked (re-review M4): the Hub stores `content` unchecked
    (`hub-contract.md:740-743` at hub `2f84662`), so a head written straight to route 26 may
    already hold the sequence, and a whole-result check would then refuse every op on that brief,
    including `skip`, `accept`, `dismiss_follow_up` and task 08's fan-out, with nothing to say
    which answer to fix. Such a head is still harmless in the file, because task 05's neutraliser
    is the guarantee and this rule is the early, explainable refusal. Any checked string
    containing `ReservedMarker` (`panacea:discover:`, case-sensitive, anywhere in the string) is
    `&ReservedTextError{Key: <the question key>}`, which `errors.Is` matches to `ErrReservedText`,
    and nothing is written. Task 11 maps it to 400 `bad_request` with `field: "text"` and
    `key`. The two model paths filter first, so a person never
    sees a 400 for text the model wrote: task 04's `ProposeFollowUp` degrades to no question and
    task 13's on-ramp drops that one draft; this check is their backstop. Without it, a pasted
    `<!-- panacea:discover:begin … -->` would still reach the file only through task 05's
    neutraliser, which keeps every `Merge` well formed; what this rule adds is the refusal at the
    answer, naming which one to fix, instead of text that is silently rewritten in every
    teammate's `CLAUDE.md` (re-review b01-2 R2-M2 item 3: the old sentence said `Merge` would go
    `block_broken`, which contradicted the neutraliser being the guarantee).
13. **The length caps (review M6, plan §4 Security, S4).** Over the same strings as rule 12 (in
    the result, added or changed by the op, so `draft`, `add_follow_up` and `restore` are
    covered and a head the Hub stored unchecked stays editable): an answer `Text` over
    `MaxAnswerRunes` (4,000), or a question's `ProjectPrompt`, `BranchPrompt`, `Help` or any
    `Options` item over `MaxPromptRunes` (300), is `&TooLongError{Key: <the question key>,
    Limit}`, and nothing is written. Criterion text needs no cap of its own: `SplitCriteria`
    takes it from the `done` answer's text, which the answer cap already bounds. `Because` is
    server-composed from a base prompt (task 04) and is not checked. Rule 12 runs first, so a
    string that breaks both reports the marker. Task 11 maps it to 400 `bad_request` with
    `field: "text"`, `key` and `limit`. The model paths never meet it: their schemas cap at 200,
    120 and 400 (tasks 04, 13), and a schema-invalid reply degrades there. What is counted is the
    string as stored: an answer `Text` after rule 14 and then rule 1's trim; a prompt, `Help` or
    option after rule 14 only, because nothing trims those (the base ones are constants, and a
    follow-up's come from task 04's schema-checked reply).
14. **Line endings (b10 re-review M-5).** Every string the op adds or changes (rule 12's set,
    before rule 12 looks at it) is stored with each `"\r\n"`, then each lone `"\r"`, replaced by
    `"\n"`. Stored brief text therefore holds only LF, whichever way it arrived: a Windows
    textarea, the API, or a model draft. This runs first, before rule 1's trim and emptiness
    check and before rules 12 and 13, so a `record` of `"a\r\nb"` over a stored `"a\nb"` is
    `ErrNoChange`, and the cap counts `"\r\n"` as one code point. A string the op left as it was
    is not rewritten (the same reason as rule 12: a head the Hub stored unchecked must stay
    editable); task 05's `Render` normalises again at the file, so such a head still cannot put a
    `"\r"` into a managed block.

**Tests first** — `content_test.go` and `ops_test.go`, package `discover`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestBaseQuestionsAreTheFiveInFileOrder` | keys `purpose, audience, constraints, done, out_of_scope`; prompts match the old plan's text byte for byte | a reworded or reordered base set, which silently changes every future digest |
| `TestContentMarshalsEverySliceAsAnArrayNeverNull` | `json.Marshal(NewContent())` and a content with a skipped answer contain no `null` except `"inherits":null` | an `omitempty` or a nil slice — which changes the canonical bytes the digest covers |
| `TestContentGoldenJSON` | a fixed content (one of each author, one skipped, one follow-up, an `Inherits`) marshals to a checked-in golden string | any tag rename |
| `TestDecodeRejectsANewerVersion` | `{"v":2,…}` is `ErrUnknownVersion` | a reader that guesses |
| `TestStepWalksTheClosedSetInOrder` (kept from `:631`) | ordinals 1–5 of total 5 | an off-by-one ordinal |
| `TestFollowUpsCarryNoDenominator` (kept from `:738`) | a follow-up step has `Total == 0` and the next base step still says `OF 5` | a denominator that grows with follow-ups (G5) |
| `TestReviewStepListsOnlyPendingDrafts` | with every base question answered except one drafted, the step is `review` and `Pending == [that key]` | a flow that counts a draft as answered |
| `TestEditChangesOneAnswerAndKeepsTheRest` (defect 7) | editing answer 2 of 5 leaves 1, 3, 4, 5 byte-identical and at their indexes | the prototype's splice |
| `TestSkipIsSettledAndCompletesTheBrief` | four records and one skip give `complete` | a skip that leaves the brief open for ever |
| `TestADraftedAnswerIsNotSettled` | five answers, one `model_drafted` → `open` | `4-D2` broken: a guess inherited by every agent |
| `TestAcceptChangesTheAuthorAndNothingElse` | text identical, author `model_accepted`, content not equal to before | an accept that is `ErrNoChange` (no new digest) |
| `TestAcceptOfAPersonsAnswerIsRefused` | `ErrNotDrafted` | an accept that relabels a person's text as the model's |
| `TestCriterionIdsSurviveAnInsertion` | done `"A\nB"` → ids `c1, c2`; edit to `"A\nX\nB"` → `c1, new, c2` | positional ids (R6) |
| `TestCriterionIdsAreNotReusedForChangedText` | `"A"` → `c1`; edit to `"A!"` → a new id | evidence attaching to a sentence it was not recorded against |
| `TestRestoreKeepsTheCurrentInherits` | restoring rev 3's answers on a branch sets `Inherits` from `env`, not from rev 3 | an undo that re-points a branch at an old project revision |
| `TestProjectContentNeverInherits` | any op in `ScopeProject` gives `Inherits == nil` even if `env.Inherits` is set | a project brief pinned to itself |
| `TestReinheritOnAProjectIsRefused` | `ErrBadOp` | a fan-out that writes project revisions |
| `TestDraftNeverOverwritesAPerson` | a person's purpose survives a `draft` op carrying a purpose | the on-ramp clobbering typed text |
| `TestNoChangeIsReported` | re-recording identical text with the same source is `ErrNoChange` | a no-op that writes a revision — every click would mint a key |
| `TestAnswerCannotContainAMarker` | table over `record`, `edit`, `add_follow_up` (prompt), `draft` (a drafted text) and `restore` (a content carrying one): each string holding `panacea:discover:` — a whole pasted `<!-- panacea:discover:begin brief=bf-1 rev=1 -->`, a bare `<!-- panacea:discover:end -->`, the sequence mid-sentence — is `errors.Is(err, ErrReservedText)`, `errors.As` gives a `*ReservedTextError` whose `Key` is the offending question's, and the input content is unchanged; `panacea:discover` without the colon and `Panacea:Discover:` are accepted | rule 12 dropped, checked on the op's text only (a model draft or a follow-up prompt forges the marker), matched on the whole marker instead of the reserved sequence, or a refusal that does not say which answer |
| `TestUnchangedReservedTextDoesNotBlockOtherOps` | a `c` whose `purpose` answer already holds `panacea:discover:` (a head written straight to route 26): `skip` of `audience`, `accept` of a drafted answer and `dismiss_follow_up` all succeed; an `edit` of `purpose` that keeps the sequence is `*ReservedTextError{Key: "purpose"}`; an `edit` of `purpose` that removes it succeeds | the whole-result check, under which one forged head refuses every op on the brief and the person cannot tell which answer to fix (re-review M4) |
| `TestAnswersAreCappedAt4000Characters` (review M6) | table over `record`, `edit`, `draft` and `restore`: a text of exactly 4,000 code points succeeds; 4,001 is `*TooLongError{Key, Limit: 4000}` and `errors.Is(err, ErrTooLong)`; 4,000 `é` (2 bytes each) succeeds, so the cap counts code points, not bytes; 4,000 letters plus surrounding spaces succeeds, so the trim runs first; a `done` answer of 4,001 is refused like any other | a cap in bytes, which refuses a short brief in any non-Latin script; a cap before the trim; a cap on `record` only, which `edit` or a restored head gets past |
| `TestPromptsAreCappedAt300Characters` (review M6) | an `add_follow_up` whose `BranchPrompt`, `ProjectPrompt`, `Help` or one `Options` item is 301 code points is `*TooLongError{Key: <the new key>, Limit: 300}`; at 300 each succeeds; a 301-code-point `Because` succeeds (not checked) | a follow-up prompt the Hub envelope carries with no bound; a check of the prompt that skips the options |
| `TestBaseQuestionsFitTheCaps` | every `BaseQuestions()` prompt, help and option is at most 300 code points, so the first op on a new brief never meets rule 13 on the server's own text | a reworded base question that refuses every new brief |
| `TestUnchangedLongTextDoesNotBlockOtherOps` (review M6) | a `c` whose `purpose` answer already holds 5,000 code points (a head written straight to route 26): `skip` of `audience` succeeds; an `edit` of `purpose` to 4,500 code points is `*TooLongError{Key: "purpose"}`; to 4,000 succeeds; an answer holding both `panacea:discover:` and 4,001 code points is `*ReservedTextError` (rule 12 first) | the whole-result check again, now for length |
| `TestStoredTextHasOnlyLF` (b10 re-review M-5) | table over `record`, `edit`, `draft` and `add_follow_up`: a text or prompt holding `"a\r\nb\rc\n"` is stored as `"a\nb\nc"` for an answer (rule 14, then the trim) and `"a\nb\nc\n"` for a prompt (no trim); a `done` answer's criteria hold no `"\r"`; a `record` of `"x\r\ny"` over a stored `"x\ny"` is `ErrNoChange`; 3,999 letters joined by one `"\r\n"` (4,001 code points as sent, 4,000 as stored) succeeds, so the cap counts after rule 14; a stored answer holding `"\r\n"` that the op leaves alone is returned unchanged | a normaliser that only handles `"\r\n"`, so a lone `"\r"` reaches the file; a normaliser run after the cap or the no-change check; one that rewrites strings the op did not touch |
| `TestApplyDoesNotMutateItsInput` | the input's answers slice is unchanged after `Apply` | aliasing between the head the service cached and the content it writes |

**Steps:**

- [ ] **Step 1:** Write `content_test.go` and `ops_test.go`. Run
  `go test ./control-plane/internal/discover/...` — expect a compile failure.
- [ ] **Step 2:** Write `content.go`: copy `BaseQuestions` and `SplitCriteria` from the old plan;
  write the types above.
- [ ] **Step 3:** Write `ops.go`. `Apply` deep-copies `c` first (slices of structs, the
  `Inherits` pointer, each `Criteria` and `DraftedFrom`), applies the op, then runs one
  `finish(&c, env)` that sets `V`, `Inherits`, `Change`, `Status` and replaces nil slices.
- [ ] **Step 4:** Run the acceptance commands; commit both files and their tests by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discover/... -race -count=1 -run 'Content|Step|Apply|Edit|Skip|Draft|Accept|Criterion|Restore|Reinherit|NoChange|FollowUp|Base|Decode|Marker|Reserved|Capped|LongText|OnlyLF|Prompt|Pending|Settled|AnswerFor|Split|FormatTime|Record|Status|Minted|Pins|Env'
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover
$ grep -n "omitempty" control-plane/internal/discover/content.go
(no output — no field of Content may be omitted, or the canonical bytes change)
$ grep -c "Revision" control-plane/internal/discover/content.go control-plane/internal/discover/ops.go
(0 for content.go; ops.go only in RestoreRevision — the domain never numbers revisions, C2)
```

**Does NOT claim:** anything about storage, numbering or digests (the Hub's, task 07); anything
about rendering (task 05). The golden JSON pins field names, not the Hub's canonical encoding,
which sorts keys itself.

**As built** (task 03, commits `8ea49b3`, `754fa8b`, fix round 1 `0372ae0`, and after; report
`.superpowers/sdd/p4/task-t03-report.md`). Where this differs from the text above, this wins.

- Rules the part left open, now fixed (each has a test):
  - `record` with a `Source` other than `picked` or `typed` is `ErrBadOp`: a person's answer
    never claims `drafted`.
  - A `done` answer whose `SplitCriteria` gives no item (`";"`) is `ErrEmptyAnswer`, for
    `record`, `edit` and `draft`: a definition of done with nothing to evaluate is empty.
  - `edit` checks the key first (`ErrUnknownKey`), then the answer (`ErrNotAnswered`), then the
    text (`ErrEmptyAnswer`). `accept` and `skip` check the key first too.
  - `add_follow_up` needs a non-blank `Prompt(env.Scope)` (`ErrBadOp`); it copies the caller's
    question and overwrites any key the caller set.
  - `accept` leaves `AnsweredAt` as it was (rule 4: only the author changes).
  - `draft` replaces only an answer that is not `Settled()`: never a person's, a skip, or a
    `model_accepted` one. An unknown draft key is `ErrUnknownKey`; an empty draft text is
    `ErrEmptyAnswer`; a draft op with nothing to write is `ErrNoChange`.
  - `restore` needs `RestoreRevision >= 1` and a `Restore` (`ErrBadOp`); it copies what it
    restores and does not trim it.
  - A minted id (`f-`, `c-`) is never one already in use: a follow-up key never matches a
    question or answer key, and a criterion id is never one the previous criteria held. After
    eight collisions `Apply` returns an unexported error (a 500, not a 400).
  - An `env.Scope` that is neither `project` nor `branch` is an unexported error (a server fault).
    So is a nil `env.NewID`, checked before the op runs, whether or not the op mints: there is
    no default generator, so this package reads no randomness of its own (fix round 1, review
    M-2).
  - Status: complete needs at least one question. A stored answer with no author is neither
    settled nor pending: `Step` asks it again. `Step` ends on the derived status, so a content
    with no questions (a head or a restored revision written straight to route 26) is
    `StepReview` with nothing pending, never `StepComplete` (fix round 1, review I-1). So
    `StepComplete` and `StatusComplete` never disagree.
  - `Decode` refuses a missing or zero `v` (unexported error, not `ErrUnknownVersion`) and fills
    every null slice with `[]`.
  - Rule 14 also normalises `Help`, each option and `Because` when the op adds or changes them;
    rules 12 and 13 still skip `Because`, and rule 12 checks prompts, answer text and criterion
    text only (task 05 renders nothing else).
  - On any error `Apply` returns the zero `Content`.
- Tests beyond the table: `TestPromptPicksTheScopesWording`, `TestDecodeGivesEverySliceAValue`,
  `TestNewContentIsTheBaseSetOpenAndUnanswered`, `TestPendingFollowsQuestionOrderNotAnswerOrder`,
  `TestSettledIsSkippedOrAPersonsText`, `TestAnswerForFindsByKey`, `TestSplitCriteria`,
  `TestFormatTimeIsUTCWithThreeFractionalDigits`, `TestRecordTrimsSetsThePersonAndStamps`,
  `TestRecordRefusals`, `TestEditNeedsASettledTypedOrPickedAnswer`,
  `TestStatusIsDerivedNeverCarried`, `TestMintedIdsNeverCollide`,
  `TestAddFollowUpMintsTheKeyAndAppends`, `TestDismissFollowUpRemovesTheQuestionAndItsAnswer`,
  `TestEveryBranchOpPinsTheCurrentProjectRevision`,
  `TestUnchangedPromptsAndCriteriaDoNotBlockOtherOps`, `TestAnEnvWithNoScopeIsRefused`,
  `TestStepAndStatusAgreeOnAnAnswerWithNoAuthor`, `TestAFollowUpKeyNeverMatchesAnExistingAnswer`,
  `TestADraftsDraftedFromIsCopied`, and from fix round 1 `TestAnEnvWithNoNewIDIsAServerFault`,
  `TestAContentWithNoQuestionsIsNeverComplete` and `TestApplyReturnsTheZeroContentOnAnyError`.
  `TestMintedIdsNeverCollide` also has a dropped-id row: an id the edit dropped is never minted
  again. The acceptance `-run` pattern above names them all.
- The table's `TestFollowUpsCarryNoDenominator` also pins `Ordinal == 0` for a follow-up, and
  `TestAnswersAreCappedAt4000Characters` skips the "spaces around 4,000" row for `restore`
  (nothing trims a restore).
