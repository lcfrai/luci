# 05 — Design: Language and Decisions on the brief revision

**Date 2026-09-28.** Scope decided by `00-summary.md` §3, which ranks
*terms and decisions as fields of a brief revision* first (product item 1) and puts the grill
rounds third. Inputs read for this design: `00-summary.md`, `01-define-skills.md`, and — read-only,
in the phase-4 worktree — `docs/superpowers/plans/2026-09-23-phase-4-discover-parts/`
`task-03.md`, `task-05.md`, `task-06.md`, `task-09.md`, `task-18.md`, `task-21.md`.

**This document edits no plan.** It is the design a later run applies to phase 4's plan, as task
amendments and new tasks. Every task number below refers to that plan.

**Method credit, and the line we do not cross.** The interview-with-a-paper-trail method is
Matt Pocock's `grill-with-docs` (<https://www.aihero.dev/grill-with-docs>, read 2026-09-28);
the glossary rules and the three-part decision test are his `domain-modeling`
(<https://github.com/mattpocock/skills>, MIT, read 2026-09-28). **No prompt text, wording or file
of theirs appears here or may appear in the product.** What follows is our mechanism, in our words,
against our content model.

---

## 0. The verdict this design rests on, in three lines

1. Phase 4 already has the **interview** and already has the **round screen** (task 13 drafts, task
   21 reviews). It does **not** have a glossary and does **not** have a decision record: `00-summary.md`
   §2 verified zero occurrences of `glossary`, `CONTEXT.md` or any decision record in the 87 KB plan.
2. So the genuine gap is **language and decisions**, not the interview. This design is that gap and
   nothing else — a thin, correct addition, per the brief's own instruction.
3. The reason to build it here rather than as files in a repo: a term that is a **field of a brief
   revision** inherits, free, every property the brief already has — Hub-numbered gapless revisions,
   immutability, a digest, the `4-D3` project→branch pin, materialisation before the PTY starts, and
   one `provenance.Ref` every surface resolves through. A repo-root file has none of them.

**What this is not.** Not a second decision system (§4), not a second brief format, not a new key
`Kind` (§5), not a gate on completeness (§3.4), and not `docs/adr/`.

---
## 1. Scope of this design

| Part | Depth here | Why |
|---|---|---|
| **Terms and decisions on the brief revision** (summary §3 product **1**) | **Full design** — §3, §4, §5, §6, §7 | The ranked top item, and the verified gap |
| **The grill interview that produces them** (summary §3 product **3**) | **Full design** — §2 | The brief for this document requires the interview answered concretely, and a glossary with no producer fills itself by hand. It is still *second* to build: §3–§5 ship and work with the five base questions alone |
| Brief → work generator (summary §3 product **2**, phase W) | **Sketch** — §9.1 | Different phase, different plan, different run |
| The no-op pass over `claudemd.Render` (summary §3 product **5**) | **Sketch** — §9.2 | ~1 h, and §5.4 makes it urgent: the block is about to grow |

**One capability, two shipping steps.** *Language and Decisions* (§3–§6) is useful the day it lands:
an agent proposing a term during Develop, and a person adding one by hand, both write into it.
*Grill rounds* (§2) is the second producer into the same collections. Build them in that order; the
plan tasks below are split so that is possible.

**A version note the plan editor must carry.** `discover.ContentVersion` is 1 and phase 4 has not
shipped a revision to any real Hub. Every field added below is added **to v1**, not to a v2. If
phase 4 ships before this lands, the same fields become `ContentVersion = 2` and `Decode` accepts
1 and 2 — the plan's own rule (`task-03.md`, the wire-format comment). Decide this once, at the top
of the amendment, not per field.

---

## 2. The interview

### 2.1 What a round is

A **round** is the *frontier*: every unanswered grill question whose prerequisites are all settled,
asked in one batch, each carrying the model's own drafted answer, then the flow stops and waits.

It is not a new screen. Phase 4 already has this screen and this op path:

- `OpDraft` writes answers as `AuthorModelDrafted`, and rule 10 forbids it overwriting a person's
  answer (`task-03.md`).
- A `model_drafted` answer is **not settled**, so a revision holding one is never complete, is never
  inherited and is never materialised (`4-D2`).
- `Content.Pending()` returns those drafts in question order; `StepReview` means *only drafted
  answers remain; accept or change each*; task 21's screen renders exactly that with **Accept** and
  **Change** per answer.

So a grill round is `OpGrillRound` = *add these questions, and draft these answers* — one op, one
revision, and then the existing review screen. **No second producer path, no second review path.**

### 2.2 How many questions a round holds

- `MaxRoundQuestions = 8`. The frontier is computed in full, then the first 8 in dependency order,
  then question order, are asked; the rest are simply the next round's frontier. Eight is the number
  a person can finish in one sitting, which is the assumption `docs/AGENTIC-UX.md` makes about them
  (intermittently present, answering in batches).
- `MaxNewQuestionsPerRound = 5` new questions may be minted by one round.
- `MaxGrillQuestions = 40` grill questions per brief, ever. `add_follow_up` and `grill_round` share
  the budget; the 41st is `ErrGrillBudget`.
- **No denominator, ever.** `Step.Total` stays 0 for the new kind, as it already is for every kind
  but `StepQuestion` (G5). The label is `ROUND 3`. `ROUND 3 OF 5` is a number we cannot defend and
  is banned by `docs/AGENTIC-UX.md` anti-pattern 2.
### 2.3 What decides the next round — dependency order, and nothing else

`Question` gains one field, and the frontier is a pure function of it:

```go
// DependsOn names the questions whose answers this question needs. A
// question may not be asked in the same round as anything it depends on.
// [] for every base question: the closed five depend on nothing.
DependsOn []Key `json:"dependsOn"`
```

```go
// control-plane/internal/discover/grill.go

// Frontier is every unanswered, non-dismissed question whose DependsOn are
// all settled, in question order. Pure: no model, no clock, no I/O. A
// question naming an unknown key is never in the frontier (an unmintable
// prerequisite is an unanswerable question) and is reported by Lint.
func Frontier(c Content) []Key

// Round is Frontier capped at MaxRoundQuestions.
func Round(c Content) []Key

// Lint reports questions that can never be asked: an unknown DependsOn, a
// cycle, or a self-reference. Apply refuses an OpGrillRound whose new
// questions would make Lint non-empty: ErrGrillCycle.
func Lint(c Content) []Key
```

**Never by importance, never by topic.** The round after this one is mechanically whatever this
one's answers made answerable. That is the whole ordering rule, and writing it down is what stops a
later implementer sorting by "what matters most" and re-introducing the question whose answer
depends on an answer nobody has given yet.

**Facts are the agent's job.** A question that needs a fact from the repository or the environment
is not asked. The round generator dispatches a sub-agent for it (task 13's `repoanalysis` is already
that shape) and treats the lookup as an unsettled prerequisite: only the questions downstream of it
wait. Everything else in the frontier is asked immediately. The lookup's result arrives as a
`model_drafted` answer in a later round, marked and reviewable like any other.

### 2.4 What ends the interview

Four terminators, and the first three are mechanical:

1. **Empty frontier.** Nothing is answerable that is not answered. This is the ordinary end.
2. **Budget.** `MaxRounds = 6`, or `MaxGrillQuestions` reached. `Grill.Closed` is set with
   `ClosedBy = "budget"`.
3. **Stall.** A round that settles no new question (every answer skipped, or the round produced no
   drafted answer a person changed or accepted) closes the interview: `ClosedBy = "stall"`. Without
   this a model that keeps re-asking around one refusal loops for ever.
4. **The person.** *That is enough* on the round screen. `ClosedBy = "person"`.

**The second gate.** An empty frontier does not finish the interview by itself; it moves the flow to
`StepReview` and then to a **close screen** the person confirms. Two gates, not one: the agent says
it has nothing left to ask, the person says they agree. `Grill.Closed` is only ever set by an op.

### 2.5 The failure mode the brief names: stopping before the criteria are provable

An interview that never ends is prevented by §2.4. An interview that stops **too early** is
prevented by seeding, not by judgement:

> **The provability seed.** When a grill starts, one question is minted per criterion of the `done`
> answer: *what would show this is true?*, with `DependsOn: ["done"]` and `Because` naming the
> criterion verbatim. They are ordinary questions, so the frontier cannot empty until each has been
> asked, and `Frontier` needs no new concept to enforce it.

The answer is recorded as that follow-up's answer. **It does not rewrite the criterion.** Criterion
ids are minted once and kept by exact text (`task-03.md` rule 5); rewriting criterion text to append
an observable would re-mint ids and silently break phase 3's evidence keys. The observable is a
*sibling* of the criterion, joined to it by the follow-up's `Because`.

A criterion added later mints its own provability question on the next round — the seed is computed
from the `done` answer at round time, not once at the start.
### 2.6 The types and ops the interview adds

```go
// Grill is the process state of the deep interview. It is NOT derivable from
// the answers (a round that everyone skipped leaves no trace in them), so it
// is stored - and it is the only stored process state in Content.
type Grill struct {
	Rounds   int    `json:"rounds"`   // rounds asked, >= 0
	Closed   bool   `json:"closed"`
	ClosedBy string `json:"closedBy"` // "" | "frontier" | "budget" | "stall" | "person"
}
```

`Content` gains `Grill *Grill` (nil before the first round: a brief that was never grilled is not a
grilled brief with zero rounds, and `null` says so).

| New `OpKind` | Carries | Rules |
|---|---|---|
| `grill_round` | `Questions []Question` and `Drafts []Answer`, both server-filled | Refuses when `Grill.Closed` (`ErrBadOp`); refuses when the frontier is empty; applies `add_follow_up` semantics per question then `draft` semantics per answer, in one revision; `Grill.Rounds++`; enforces `MaxNewQuestionsPerRound`, `MaxGrillQuestions`, and `Lint` |
| `grill_close` | `ClosedBy` | Refuses `ClosedBy: "person"` from a non-person author; sets `Closed` |

`StepKind` gains `StepRound` — *a round is waiting; answer or accept each*. `Step.Kind` is
`StepRound` exactly when `Pending()` is non-empty **and** the drafts were written by a
`grill_round`; otherwise the existing `StepReview` (the repo on-ramp) is unchanged. `Ordinal` is the
round number; `Total` is **0** (§2.2).

`ChangeKind` gains `ChangeGrillRound` and `ChangeGrillClose`, so the history rows read
*Round 3 asked, 8 questions* without diffing two revisions — the reason `Change` exists.

### 2.7 What the interview is not allowed to do

- It may not touch the five base questions' prompts, their order or their denominator.
- It may not mint a question with `DependsOn` naming a grill question in the **same** round: `Apply`
  refuses (`ErrGrillCycle`), because that is exactly the question the frontier rule exists to stop.
- It may not write a term or a decision by itself. A round **proposes** them as
  `AuthorModelDrafted` members (§3.3, §4.3), and a person settles them — the same `4-D2` rule that
  already governs answers. One rule, three collections.

---

## 3. The glossary

### 3.1 Where a term lives

**A field of the brief revision, beside `Answers`.** Not `CONTEXT.md` at a repo root, for two
reasons, one each:

1. `OD-3` puts shared state in the Hub. A glossary in one person's worktree is gone the moment a
   teammate opens the project.
2. A loose file has no revision. Nothing downstream can cite **which** definition an agent was
   handed — and that citation is the entire point (§5).

```go
// Term is one word this project uses in a particular way. ID is minted once
// and kept across every edit, rename and supersede: it is the identity, and
// a citation names it. Modelled on Criterion.
type Term struct {
	ID         string   `json:"id"`         // "t-" + 8 hex
	Word       string   `json:"word"`       // the canonical word, NFC, trimmed, <= 64 code points
	Is         string   `json:"is"`         // 1-2 sentences: what it IS, not what it does
	Avoid      []string `json:"avoid"`      // rejected synonyms; the words not to use
	Author     Author   `json:"author"`     // person | model_drafted | model_accepted
	Retired    bool     `json:"retired"`
	Because    string   `json:"because"`    // required on retire and on rename; "" otherwise
	Supersedes string   `json:"supersedes"` // the Term.ID this one replaces; "" otherwise
	DefinedAt  string   `json:"definedAt"`  // RFC 3339 UTC, three fractional digits
}
```

`Content` gains `Terms []Term` — marshalled `[]`, never `null`, like every other slice (rule 11).
### 3.2 What makes a term settled enough to record

Two tests, and it matters which one is code and which one is not.

**The mechanical test, enforced in `Apply`** (each is a named error, so the 400 can point at the
field task 11 already knows how to point at):

| Rule | Error |
|---|---|
| `Word` non-empty after trim, ≤ 64 code points, no line ending, not whitespace-only | `&TooLongError{Key: "t-…", Limit: 64}` / `ErrEmptyTerm` |
| `Word` unique among **non-retired** terms of this revision, compared case-folded (`strings.EqualFold` after NFC) | `&TermExistsError{ID}` → `ErrTermExists` |
| `Is` non-empty, ≤ `MaxTermRunes = 300` code points | `ErrEmptyTerm` / `&TooLongError{Limit: 300}` |
| `Avoid` ≤ 8 items, each ≤ 64 code points, each distinct from `Word` and from each other, case-folded | `ErrBadOp` |
| `Because` non-empty on `retire_term` and on `rename_term` | `ErrBadOp` |
| Marker rule 12 and the LF rule 14 apply unchanged to `Word`, `Is` and every `Avoid` item | `&ReservedTextError{Key: "t-…"}` |

**The authoring test, which is not code and must not pretend to be.** A term earns an entry when it
is a **noun specific to this project**, defined by what it *is*, with one canonical word chosen and
the rejected synonyms listed. General programming vocabulary is excluded even when the project is
full of it, and implementation detail is excluded absolutely: a glossary that acquires *how* becomes
a second brief, and then one provenance thread carries two competing descriptions.

That test is a judgement, so it is enforced by **who settles it, not by what passes a regex**: a
model-proposed term enters as `AuthorModelDrafted`, is rendered nowhere (§5.4) and is not resolved
into the document, until a person accepts or changes it. That is `4-D2` applied a third time, and it
is the same mechanism, the same screen shape and the same op names the answers already use.

### 3.3 Who can change one, and how

| Op | Who | Effect |
|---|---|---|
| `define_term` | person, or model as a draft | Appends a `Term`. `Author` is `person`, or `model_drafted` from a grill round or a Develop proposal |
| `accept_term` | person only | `model_drafted` → `model_accepted`, text unchanged. Changes the digest, exactly as `accept` on an answer does |
| `edit_term` | person only | Replaces `Is` and/or `Avoid` **in place**, same `ID`, `Author` → `person` |
| `rename_term` | person only | Replaces `Word` in place, same `ID`; requires `Because`; **appends the old word to `Avoid`** |
| `retire_term` | person only | `Retired: true`; requires `Because`. The term is kept in content for ever (immutability) and rendered nowhere |
| `supersede_term` | person only | `define_term` + `retire_term` in one revision: the new term carries `Supersedes`, and the retired word is appended to the new term's `Avoid` |

**Scope authority follows the brief's, with no new concept.** A *project* term is changed on the
project brief. A branch may **override** a project term by defining one with the same case-folded
`Word` — that is the resolution rule in §5.3 — but a branch op may never edit a project term in
place. `Apply` has no scope-crossing path to do so, because it only ever sees one content object.

### 3.4 Terms never gate completeness

`Status` is derived from `Answer.Settled()` over the **questions** and nothing else. A glossary is
open-ended by nature; if a pending term made a revision incomplete, a brief could never complete,
nothing would ever be inherited, and nothing would ever be materialised.

Write this into task 03 as a rule with a test, because it is the kind of thing a later implementer
"tidies up" into `Status` and does not notice until the Materialiser silently stops. Same for
decisions (§4).

The consequence, stated plainly so nobody is surprised: **a complete revision may hold a
`model_drafted` term.** It resolves and materialises with that term absent. The drafted term shows
in the Discover lens with the `◌` mark until a person deals with it.
### 3.5 What happens to work that referenced the old meaning

**Nothing is edited, so nothing breaks — and the change is visible rather than silent.** This is the
property that makes a glossary worth having; a glossary whose meaning moves underneath finished work
is worse than none.

The mechanism, in four steps, all of which already exist for criteria:

1. A revision is immutable. `edit_term` does not change revision 7; it produces revision 8.
2. Every citation is a **fragment of a revision key**: `brief:bf-…@7#t-1a2b3c4d` (§5.1). It names
   revision 7, for ever, and revision 7's text for `t-1a2b3c4d` can never change.
3. Every term carries a **digest over its meaning-bearing text** (§5.2). A consumer that stored
   `t-1a2b3c4d` + digest at revision 7 can compare it with the same id's digest at revision 9 in one
   equality check — no diffing, no parsing.
4. So the surface that shows it is the one phase 3 already has: evidence is keyed by brief key +
   criterion id + text digest (`task-03.md`). A term citation has an identical shape, so *"this
   session was handed a definition of **engagement** that has since changed"* is a comparison, not a
   feature.

**Two rules that make step 4 truthful:**

- **The digest covers `Word` and `Is`, never `Avoid`.** Adding a rejected synonym is a hint to the
  next agent; it does not change what the word means, and it must not invalidate a citation. Changing
  `Is` does change the meaning, and it must. Getting this backwards produces either a drift alarm
  that cries wolf on every tidy-up, or one that stays silent through a redefinition.
- **A retired term is kept in content, for ever.** An old citation must still resolve to the text
  the agent actually read. `Retired` controls rendering, never storage.

**What is deliberately not built:** no automatic rewrite of anything that used the old word, no
back-propagation into earlier revisions, and no block on renaming a term that something cites. The
platform reports; the person decides.

---

## 4. The decision record

### 4.1 The exact test a decision must pass

All three, simultaneously, plus one thing that makes the third real:

1. **Hard to reverse** — changing your mind later costs something real. If it is easy to reverse you
   will simply reverse it, and the record buys nothing.
2. **Surprising without context** — a future reader will look at the result and ask why on earth. If
   it is not surprising, nobody will ever wonder.
3. **A genuine trade-off** — there were real alternatives and one was chosen for stated reasons. If
   there was no alternative there is nothing to record beyond "we did the obvious thing".
4. **The rejected alternative is named.** Our addition, and it is the one that stops leg 3 being a
   checkbox somebody ticks. `Gate.Alternative` is a required non-empty string whenever `TradeOff` is
   true, and `TradeOff` is required. A trade-off with no named alternative is not a trade-off.

Miss any one and there is no decision record. The gate is meant to reject nearly everything: the
whole value of a decision log is that reading all of it is cheap.

### 4.2 The types

```go
type Gate struct {
	Irreversible bool   `json:"irreversible"`
	Surprising   bool   `json:"surprising"`
	TradeOff     bool   `json:"tradeOff"`
	Alternative  string `json:"alternative"` // what was NOT chosen; required, <= 300 code points
}

// Decision is one hard-to-reverse choice this project made. One paragraph,
// never a template: Title, then 1-3 sentences of context, decision and why.
type Decision struct {
	ID         string `json:"id"`         // "d-" + 8 hex
	Title      string `json:"title"`      // <= 120 code points
	Because    string `json:"because"`    // <= MaxDecisionRunes (800)
	Gate       Gate   `json:"gate"`
	Author     Author `json:"author"`
	Supersedes string `json:"supersedes"` // the Decision.ID this replaces; "" otherwise
	DecidedAt  string `json:"decidedAt"`  // RFC 3339 UTC, three fractional digits
}
```

`Content` gains `Decisions []Decision`, marshalled `[]`, never `null`.
### 4.3 The gate is enforced in the domain, not in a prompt

```go
// ErrGateNotMet: a record_decision whose Gate does not pass all three legs,
// or whose TradeOff names no Alternative. Apply returns it wrapped in
// *GateError, so errors.Is holds and the missing legs are recoverable.
var ErrGateNotMet = errors.New("discover: that is not a decision worth recording")

type GateError struct {
	Title   string
	Missing []string // any of "irreversible", "surprising", "tradeOff", "alternative"
}
```

A gate that lives only in a model prompt does not reject anything under pressure — it is one
sentence of instruction against a model that would rather be helpful. `Apply` refusing the op is the
only version of this that survives contact. Task 11 maps `ErrGateNotMet` to 400 `bad_request` with
`field: "gate"` and `details.missing`, and the lens names the missing legs in words (§6.3).

| New `OpKind` | Who | Rules |
|---|---|---|
| `record_decision` | person, or model as a draft | Appends. Gate enforced. `Supersedes` must name a live decision of this content or `""` |
| `accept_decision` | person only | `model_drafted` → `model_accepted`; text unchanged; digest changes |
| `edit_decision` | person only | `Title`, `Because`, `Gate` in place, same `ID`; the gate is re-checked |
| `supersede_decision` | person only | `record_decision` with `Supersedes` set, in one revision. The superseded decision is **kept in content** and rendered nowhere |

There is no `delete`. A decision that should never have been recorded is superseded, or it was never
recorded because the gate refused it.

### 4.4 How this relates to `OD-n`, to orchestrator rulings, and to the Hub's ledger

**Phase 4 invents no second decision system.** Three registers already exist or are planned; each
keeps its own authority, and the traffic between them runs one way.

| Register | About what | Authority | Lives in |
|---|---|---|---|
| **`OD-1`…`OD-35`** | PanaceaApp, the product we are building | The owner | `docs/superpowers/planning/2026-09-23-owner-decisions.md`, this repo |
| **Project decision `d-…`** | The **user's** project, inside their brief | The person who owns the brief, at its scope | `Content.Decisions` of a brief revision, in the Hub |
| **Orchestrator ruling** | One session's course, taken while the user was absent (`OD-16`) | The Orchestrator, escalating after the fact | The session's own record, phase T |

Three rules keep them apart, and all three should be written into the plan verbatim:

1. **A `d-` never becomes an `OD`, and an `OD` never appears in a user's brief.** They are about
   different products. A user's glossary and decision list are *their* project's, and the platform
   writes nothing of its own into them.
2. **An orchestrator ruling that meets the three-part gate is proposed as a `d-` with
   `Author: model_drafted`**, and appears in the Discover Language & Decisions surface for accept or
   change. Until a person settles it, it is rendered nowhere — `4-D2` again. A ruling that fails the
   gate stays in the session record and never enters the brief. This is the only path from a ruling
   to a project decision, and it is a proposal, never a write.
3. **The Hub's provenance ledger gains no record type.** The ledger is append-only **link** records
   — *"this subject was bound to this brief key"*
   (`docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md` §2.1, read
   2026-09-28). A decision is **content**, inside the revision the key names, covered by that
   revision's digest. A merge link or a release link that wants to name the decision it implements
   uses the existing brief key with a `#d-…` fragment (§5.1). Nothing else changes: not the ledger
   schema, not the write points, not phase 7.

**And no `docs/adr/`.** For the *product*, the artifact is a field of the brief. For *our own repo*,
`00-summary.md` §3 practice item 4 already rules: keep the `OD` register, adopt the three-part gate
as the test for what earns one, and create no parallel tree.

### 4.5 Decisions never gate completeness

Same rule, same reason, same test as §3.4. A complete revision may hold a `model_drafted` decision;
it resolves and materialises without it.
---

## 5. The provenance answer

This is the part that makes ours better than the interview it is modelled on. Theirs leaves two
files in a repository; a reader cannot tell which definition an agent was handed, and a teammate on
another machine has neither. Ours makes a term and a decision **carry the brief revision they belong
to, be immutable per revision like the brief, and be visible in Design, Develop and Deploy** — with
no machinery that does not already exist.

### 5.1 The key grammar — a fragment, not a fourth `Kind`

The grammar stays `brief:` · `design:` · `none:`, with one parser (`P5`). A `term:` or `adr:` kind
would start a second thread and defeat the entire design.

```
brief   ^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})(#((c|t|d)-[0-9a-f]{8}))?$
```

```go
type FragmentKind string

const (
	FragmentNone      FragmentKind = ""
	FragmentCriterion FragmentKind = "criterion" // c-
	FragmentTerm      FragmentKind = "term"      // t-
	FragmentDecision  FragmentKind = "decision"  // d-
)

// Key gains two fields. Fragment is the id as written ("t-1a2b3c4d");
// FragmentKind is its prefix, decoded once so no other package splits it.
Fragment     string
FragmentKind FragmentKind

// FragmentKey composes one: BriefKey(id, rev) + "#" + fragmentID. Panics on
// a fragment id that is not c-, t- or d- plus 8 hex: a programming error.
func FragmentKey(briefID string, revision int, fragmentID string) string
```

Three rules that keep this one grammar rather than two:

1. **A fragment never affects resolution.** `Build` resolves the revision and ignores the fragment.
   Two keys differing only by fragment resolve to the same `Ref` bytes plus a different `Fragment`.
2. **`contextRef` never carries a fragment.** A session inherits a *revision*, not one line of it.
   `Parse` accepts a fragment everywhere; task 09 and task 07 write keys without one, and a test
   pins that.
3. **A fragment that names no member is not an error.** `Ref.FragmentPresent` is false and the
   surface renders *this revision has no such decision*. A stale citation from a superseded revision
   is a normal, readable state, not a 500.

### 5.2 The digests

Mirroring `Criterion.Digest` (`"sha256:" + hex(SHA-256(UTF-8 of Text))`) exactly, because phase 3
already consumes that shape:

| Member | Digest over | Deliberately excluded |
|---|---|---|
| `Criterion` | `Text` | — (unchanged) |
| `Term` | `Word + "\n" + Is` | `Avoid` (a hint, not a meaning — §3.5), `Author`, `DefinedAt` |
| `Decision` | `Title + "\n" + Because` | `Gate` (the reason it was recordable, not what was decided), `Author`, `DecidedAt` |

`Ref` gains, beside `Criteria`:

```go
Terms     []Term     `json:"terms"`     // resolved, inherited, digests included
Decisions []Decision `json:"decisions"` // resolved, inherited, superseded dropped
```

Both are **functions of the key alone**, so both are cached for ever with the rest of the `Ref`
(contract §6.4). Neither has a moving counterpart: a criterion has a verdict that changes as
evidence arrives, which is why `CriterionReading` exists outside the `Ref` (review I14). **A term
has no verdict and a decision has no verdict.** Do not build a `TermReading`; there is nothing for
it to hold.
### 5.3 Inheritance — the two new rules in `discover.Resolve`

`Resolve(project, branch)` keeps every rule it has, including `ErrInheritMismatch` and
`ErrIncomplete`. It gains two, and `Document` gains two fields (`Terms []Term`, `Decisions
[]Decision`, each carrying an `Origin`):

**Terms union, branch overrides project by case-folded `Word`.**

- A branch term whose folded `Word` equals a project term's replaces it: `OriginOverride`, and the
  project term's `Is` is kept in `Replaced` (the same field, the same meaning, as a section's).
- A branch term with no project counterpart: `OriginBranch`. A project term with no branch
  counterpart: `OriginProject`.
- `Retired` terms are dropped, at both scopes, after the union — so a branch may **retire** a project
  term for its own work by defining a retired term with the same word and a `Because`.
- `model_drafted` terms are dropped. Never rendered, at either scope (§3.4).
- Order: case-folded `Word` ascending, stable. A new term therefore inserts in place instead of
  appending, which is what makes `DiffLines` show **one** added line rather than a moved block.

**Decisions concatenate, project first, then branch.**

- De-duplicated by `ID`; a branch cannot hold a project decision's id, so a collision is a corrupt
  content and `ErrBadOp` at write time, not a resolve-time merge.
- The supersede chain is followed **across scopes**: a branch decision with `Supersedes` naming a
  project decision drops the project's from the document. That is the mechanism by which a branch
  says *not here* about a project-wide choice, and it is one field, not a new concept.
- `model_drafted` decisions are dropped.
- Order is the order recorded, project first. A decision log reads chronologically; sorting it by
  title would make a later decision appear to precede the one it replaced.

### 5.4 What the Materialiser writes, so an agent inherits the vocabulary

**The Materialiser does not change.** `contextwrite.Materialise` already asks `discoverhub` what the
session inherits, renders that one key, writes it atomically into the worktree's `CLAUDE.md`
**before `ptysession.Start`**, and returns the key it wrote. Because a term is *inside* the revision
the key names, all of that applies to it unaltered. Task 09's edit is **new test rows only** — say so
in the amendment, or an implementer will go looking for code to write.

That ordering is what buys the property no repo-root file can have: **the agent that uses the word
is provably the agent that was handed the definition**, because the session is stamped with the key
whose block was on disk before the process existed.

`claudemd.Render` gains two sections in the **same** managed block, after `## Out of scope` and
before the `---` footer. No second file, no second write, no second atomic-write hazard.

```markdown
## Words we use
_Project vocabulary, with 2 words set on this branch._

- **Engagement** — One piece of audit work for one client, with its own team and its own deadline.
  Not: job; matter; case.
- **Evidence** — A file a client uploads against a request. It is never edited, only superseded.

## Decisions already taken
_Taken on the project brief, and on this branch._

- **Uploads are immutable.** Re-uploading creates a new evidence record and supersedes the old one,
  because an audit trail that can be edited is not an audit trail. Not chosen: overwrite in place.
```

Six render rules, all deterministic and all covered by a golden:

1. A term is `- **<Word>** — <Is>`, then, when `Avoid` is non-empty, a continuation line indented two
   spaces: `Not: ` + `Avoid` joined by `"; "`. One blank line between terms: an agent reads a dense
   list as one paragraph.
2. A decision is `- **<Title>** — <Because>`, then, on the same continuation shape,
   `Not chosen: <Gate.Alternative>`.
3. The italic origin line is **computed from counts**, exactly as the section origin lines are, and
   is omitted when every member has one origin and that origin is the document's own scope.
4. **An empty section is omitted entirely**, heading and all — the existing
   `TestSkippedProjectAnswerIsOmitted` rule: an empty heading is read as a claim that there are none.
5. The neutraliser and the LF conversion apply to `Word`, `Is`, every `Avoid` item, `Title`,
   `Because` and `Alternative`, exactly as they do to answer and criterion text. `strings.Count(block,
   "<!--") == 2` stays true.
6. **The context cap** (`writing-for-agents`, summary §3 product 5): `MaxInlineTerms = 40`,
   `MaxInlineDecisions = 20`. Past the cap, the remainder renders as **words only**, one line, comma
   separated, under `Also defined: `. Progressive disclosure for one line of cost, instead of an
   unbounded block paid for in every session on that branch, for ever.
### 5.5 What Design, Develop and Deploy show — and what each costs

| Surface | What it gains | Cost |
|---|---|---|
| **Develop** (the per-branch `CLAUDE.md`) | The two sections, §5.4 | Render + goldens; **no** Materialiser change |
| **Develop** (the session's brief panel) | The `Ref` already mounted there renders `Terms` and `Decisions` beside `Criteria` | One component each |
| **Design** (phase 5) | `design:` keys already link to a brief key in the ledger; the canvas's brief panel reads the same `Ref` | None in phase 4 |
| **Merge** (phase 3) | A commit or trailer may cite `brief:…@n#d-…`. The evidence store is already keyed by brief key + id + digest, so the shape fits with no schema change | None in phase 4 |
| **Deploy** (phase 7) | **Already carried.** The release link record holds brief key + digest (`provenance-contract-draft.md` §2.1). Terms and decisions are inside that revision and covered by that digest, so the release manifest names them by naming the key | **Zero contract change**; the Deploy lens renders `Ref.Decisions` |

**The release answer in one sentence, because it is the one a reviewer will ask for:** a release
already names a brief key and its digest, so *the decisions that release was built under* is a
lookup of an immutable revision, not a new field — and if someone later asks *which definition of
"evidence" shipped in 1.4.0*, that too is already answered.

---

## 6. What the user sees, in the Discover lens

Honours `docs/DESIGN-SYSTEM.md`, which task 18 and task 21 already hold to. **The owner's eyes are
the visual acceptance (`OD-12`); nothing below is a claim that it looks right.**

### 6.1 Where it sits

A third region in the lens, below the question flow and the settled answers, as **two cards**:
**Language** and **Decisions**. Not a separate route, not a tab: the thread the lens exists to show
is one thread, and a tab would hide half of it behind a click nobody makes.

Each card is collapsed to its heading, a count in `tabular-nums`, and — when anything is drafted —
the `◌` mark with the words *n drafted by the model*. Colour is never the only signal.

### 6.2 The Language card

- One row per term: **word** in the medium weight, `Is` on the line below in the secondary colour,
  `Not: …` in the tertiary, only when there is one.
- A drafted term carries `◌ drafted by the model` and, where the proposal came from a Develop
  session or a grill round, *proposed while answering "Definition of done"* — the same
  `draftedFrom` treatment task 21 already uses for answers.
- Three controls per drafted row: **Accept**, **Change**, **Not this word**. *Accept* is never
  pre-focused (task 21's rule).
- On a settled row: **Change** and **Retire**. Retire asks for one sentence of `Because` and says,
  in words, what will happen — *it stops appearing in new sessions; sessions that already read it
  keep the words they were given*.
- An inherited project term on a branch shows the word *project* in the tertiary colour, and
  **Override here** as its only control. Overriding shows the project's definition struck through
  above the new field, so the person sees what they are replacing.

### 6.3 The Decisions card and its gate

Recording a decision opens a small form, and **the gate is visible, not hidden**:

- `Title`, then `Because` (the helper says: context, decision, why, in one to three sentences).
- Three checkboxes, all required, each with its own one-line reason on screen — *hard to reverse*,
  *surprising without context*, *a real trade-off*.
- A required field, **What we did not choose**, which appears when the third box is ticked.
- **Save is disabled until all four**, and the reason is written next to the button in words, never
  in a tooltip: *Not yet: name what you did not choose.* A disabled control with no stated reason is
  the interface telling someone they are wrong without saying how.
- Below the form, one sentence that does not move: *Most choices are not decisions. If it is easy to
  reverse, just reverse it.*

### 6.4 The round screen (ships with §2)

The existing review screen with a round header: `ROUND 3` — **no denominator** (§2.2) — the
questions numbered within the round, each with its drafted answer pre-filled and editable, **Accept
all** at the top and per-question **Accept** / **Change** / **Skip**. A *That is enough* control ends
the interview (§2.4 terminator 4) and says what it does: *stops asking; the brief keeps every answer
you have given*.

**Held from the design system, explicitly:** no gradient and no indigo/violet; elevation is a 1px
ring plus layered shadow; `--r-card` on the cards and `--r-ctl` on controls; the one `--accent` for
interactive and current state only; `transform`/`opacity` only, never `transition: all`; **no
spinner** — an in-flight save changes the button's words to *Saving* and disables it; the one
heartbeat clock for any live dot; counts and `rev n` in `tabular-nums`; motion only where it explains
a cause — the key chip flashes and decays over ~600 ms on `opacity` **only** when a save mints a new
complete revision, never on revisit and never as an entrance.
---

## 7. The tests, red first

Written in the plan's own style: the test, what it asserts, and **the mutant it kills**. Each is red
before the code it names exists. Package paths are the plan's.

### 7.1 `discover` — content and ops (task 03)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGateRefusesTwoOutOfThree` | `record_decision` with each of the three legs false in turn → `ErrGateNotMet`, `GateError.Missing` names exactly the false one; nothing written | a gate that lives in the prompt and passes anything the model asserts |
| `TestTradeOffNeedsANamedAlternative` | all three legs true, `Alternative: ""` → `ErrGateNotMet` with `Missing: ["alternative"]` | leg 3 as a checkbox: three ticks and no thought |
| `TestTermsDoNotGateCompleteness` | five settled answers plus a `model_drafted` term and a `model_drafted` decision → `Status == complete`, and `Resolve` succeeds | a `Status` "tidied up" to include terms, after which nothing ever materialises |
| `TestTermWordIsUniqueCaseFolded` | `define_term` "Evidence" then "evidence" → `ErrTermExists` naming the existing id; then the same after the first is retired → accepted | two live definitions of one word, which is the failure a glossary exists to prevent |
| `TestRetireAppendsTheOldWordToAvoid` | `supersede_term` "job" → "engagement" leaves the new term's `Avoid` holding "job", and the old term present with `Retired: true` | a retire that deletes, so an old citation resolves to nothing |
| `TestEditTermKeepsTheID` | `edit_term` changes `Is`; `ID` unchanged; digest changed | an edit that mints a new id, breaking every citation silently |
| `TestAvoidDoesNotChangeTheDigest` | adding an `Avoid` item leaves `TermDigest` equal; changing `Is` changes it | a digest over the whole struct, which cries wolf on every tidy-up |
| `TestDecisionSupersedeKeepsBoth` | `supersede_decision` leaves both in `Content.Decisions`; `Resolve` renders one | a supersede that deletes the record of what was replaced |
| `TestMarkerAndCapsCoverTermsAndDecisions` | `panacea:discover:` in `Word`, `Is`, an `Avoid` item, `Title`, `Because` and `Alternative` each → `&ReservedTextError{Key: "t-…"/"d-…"}`; over-length each → `&TooLongError` with the right `Limit` | rules 12 and 13 applied to answers only, so the new collections are the hole |
| `TestAcceptTermChangesTheDigest` | `accept_term` on unchanged text changes the revision digest | an accept treated as a no-op, so `4-D2`'s settle is invisible downstream |

### 7.2 `discover` — the frontier (task 03 + new grill task)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFrontierExcludesTheDependent` | q2 `DependsOn: ["q1"]`, q1 unanswered → frontier holds q1 only; answer q1 → frontier holds q2 | the whole point: a question asked before the answer it needs |
| `TestRoundIsCappedAndTheRestWait` | 12 answerable questions → `Round` returns 8, in question order; the other 4 are the next frontier | a round that dumps everything and is abandoned |
| `TestGrillCycleIsRefused` | `grill_round` adding a→b→a → `ErrGrillCycle`, nothing written | a cycle that empties the frontier for ever, so the interview never ends |
| `TestStallClosesTheInterview` | a round whose every answer is skipped → `Grill.Closed`, `ClosedBy: "stall"` | the loop that re-asks around one refusal |
| `TestBudgetClosesTheInterview` | `MaxRounds` rounds → closed, `ClosedBy: "budget"`; a further `grill_round` → `ErrBadOp` | an interview that never ends |
| `TestProvabilitySeedPerCriterion` | a `done` answer with three criteria → the first round's frontier holds three questions, each `Because` quoting one criterion verbatim, each `DependsOn: ["done"]` | an interview that closes with criteria nobody could test |
| `TestCriterionAddedLaterIsSeeded` | edit `done` to add a fourth criterion after round 1 → round 2's frontier holds its question | a seed computed once at the start |
| `TestRoundStepHasNoTotal` | `Step` on a pending round → `Kind: StepRound`, `Ordinal: 3`, `Total: 0` | `ROUND 3 OF 5`, a number the system cannot defend (`AGENTIC-UX` anti-pattern 2) |
| `TestGrillNeverWritesASettledTerm` | a `grill_round` proposing a term → `Author: model_drafted`, absent from `Resolve` | the model settling vocabulary nobody agreed to |

### 7.3 `discover.Resolve` and `claudemd` (task 05)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestResolveUnionsTermsBranchWins` | project "evidence", branch "Evidence" → one term, `OriginOverride`, `Replaced` holding the project's `Is` | two definitions of one word in one block |
| `TestBranchRetiresAProjectTerm` | branch retires an inherited word → absent from the document, project revision untouched | a branch edit that reaches across scope |
| `TestBranchSupersedesAProjectDecision` | branch decision `Supersedes` the project's → only the branch's renders, project first order otherwise kept | a supersede that works within a scope and silently not across one |
| `TestTermsSortStablyByWord` | inserting a term changes exactly one line of `DiffLines` | an append-ordered list, which reports the whole block as edited |
| `TestEmptySectionsAreOmitted` | no terms, no decisions → neither heading appears in the block | an empty *Decisions already taken* heading, which an agent reads as "there were none" |
| `TestGoldenBranchWithLanguageAndDecisions` | `Render(Resolve(…))` equals `testdata/golden-branch.md` byte for byte, the two new sections included | any drift in wording, order or spacing |
| `TestInlineCapDegradesToWordsOnly` | 45 terms → 40 rendered in full, then one `Also defined: ` line naming 5 | an unbounded glossary paid for in every session for ever |
| `TestNeutraliserCoversTheNewStrings` | `<!--` in `Word`, `Is`, `Avoid`, `Title`, `Because`, `Alternative` → `<!-\-`; `strings.Count(block, "<!--") == 2` | the escape applied to answers and not to the new fields |
| `TestDraftedTermAndDecisionAreNeverRendered` | status forced complete with both drafted → absent | `4-D2` broken by a forged status |
### 7.4 `provenance` (task 06)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestParseFragment` | `brief:bf-…@7#t-1a2b3c4d` → `Fragment`, `FragmentKind: term`; `#x-1a2b3c4d`, `#t-1A2B3C4D`, `#t-` and a double `#` → `ErrInvalidKey`; `String()` round-trips | a lenient parser that accepts a key nothing can produce |
| `TestFragmentDoesNotChangeResolution` | two keys differing only by fragment → identical `Ref` but for `Fragment`/`FragmentPresent` | a fragment leaking into the resolution path, so a citation resolves differently from the brief |
| `TestUnknownFragmentIsNotAnError` | `#d-deadbeef` naming nothing → `Resolution: resolved`, `FragmentPresent: false` | a 500 on a stale citation from a superseded revision |
| `TestContextRefNeverCarriesAFragment` | what task 07 and task 09 write is fragment-free | a session inheriting one line of a brief instead of a revision |
| `TestRefCarriesTermsAndDecisions` | `Build` fills both from the resolved document, digests included, superseded and drafted absent | a `Ref` that shows the block's words and the API's words differently |
| `TestNoTermReading` | a Go-level check that no `Reading` type exists for terms or decisions | a moving-state type built for something that has no verdict |

### 7.5 The seam and the desktop

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMaterialiseWritesVocabularyBeforeStart` (task 09) | the file on disk holds the two sections **before** `ptysession.Start` is called; the mock start asserts it | the one ordering that makes the whole feature real, called afterwards, silently |
| `TestVocabularyIsNeverCommitted` (task 09) | `git status` clean after materialising into a tracked-file worktree; the exclude line present | a glossary committed into the user's repository |
| `TestGateDisabledSaveStatesItsReason` (task 18/new lens task) | Save disabled with the reason in text content, not a `title` attribute | a disabled control that will not say why |
| `TestDraftedTermShowsTheMark` | `◌` **and** the words *drafted by the model* | colour as the only signal |
| `TestRoundHeaderHasNoDenominator` | the round header renders `ROUND 3` and no `OF` | G5 broken in the one place it is easiest to break |

---

## 8. What a plan editor changes, task by task

**Amend** (phase 4 plan, `2026-09-23-phase-4-discover-parts/`):

| Task | Change | Size |
|---|---|---|
| **03** | `Term`, `Decision`, `Gate`, `Grill`; `Content.Terms`/`.Decisions`/`.Grill`; `Question.DependsOn`; nine new `OpKind`s (§3.3, §4.3) plus `grill_round`/`grill_close`; `ErrGateNotMet`/`GateError`, `ErrTermExists`/`TermExistsError`, `ErrEmptyTerm`, `ErrGrillCycle`, `ErrGrillBudget`; `MaxTermRunes`, `MaxDecisionRunes`, the caps of §2.2; rules 12/13/14 extended; **the completeness rule of §3.4 written as a rule**; the v1-not-v2 note (§1) | 4–5 h |
| **05** | `Document.Terms`/`.Decisions` with `Origin`; the two `Resolve` rules (§5.3); the two render sections and six render rules (§5.4); three new goldens | 3–4 h |
| **06** | The fragment grammar, `Key.Fragment`/`.FragmentKind`, `FragmentKey`, `Ref.Terms`/`.Decisions`, `Ref.Fragment`/`.FragmentPresent`; the no-`Reading` note (§5.2) | 2 h |
| **09** | **Tests only** — §7.5's two rows. No code change; say so explicitly | 0.5 h |
| **11** | The new ops on the wire; 400 mappings for `ErrGateNotMet` (`field: "gate"`, `details.missing`) and `ErrTermExists` | 1 h |
| **15, 16** | Client types and cache for `Ref.Terms`/`.Decisions` (cacheable for ever, like `Criteria`) | 1 h |
| **18** | The lens hosts the two cards and the round header; the keyboard rules extend to the card rows | 1 h |
| **04** | `ProposeFollowUp` gains `DependsOn` on the question it proposes (`[]` is legal and is today's behaviour) | 0.5 h |

**New tasks:**

| # | Task | Size |
|---|---|---|
| **24** | `discover/grill.go` — `Frontier`, `Round`, `Lint`, the provability seed, the round generator's model call and its degrade-to-nothing path | 4 h |
| **25** | Desktop — the Language and Decisions cards (§6.2, §6.3) | 3–4 h |
| **26** | The render goldens and the block-digest fallout: `claudemd`'s goldens, `SameText`, `DiffLines`, `Inspect` drift, the marker and neutraliser tables. **This is the real work of §5.4, not the Go types** | 3 h |
| **27** | Desktop — the round screen (§6.4), ships with §2 | 2–3 h |

**Total: 8 amended, 4 new, ~26–30 h** — against the summary's estimate of ~12–16 h for item 1 plus
~10–12 h for item 3, which this splits differently and sizes slightly higher, because §5.4's golden
fallout is a task of its own and §2.5's seed was not in the summary's scope.

**Order:** 03 → 05 → 26 → 06 → 11 → 15/16 → 25 → 18. Then, and only then, 24 → 27.
---

## 9. The next two, sketched

### 9.1 Break a settled brief into work (summary §3 product 2, phase W)

**The gap.** Phase W has `serves`, issue states and a *split into issues* action (task 88), and
triage suggestions that attach `serves` (task 136). Nothing turns a **settled brief** into the issue
set. So `serves` is attached by a human typing forty issues, and *uncovered* is a report about work
that was never generated rather than a gap in work that was.

**The shape.** One generator, one review screen, nothing saved before the review:

1. Input is a complete brief revision **and its criteria** — the criteria are the natural unit,
   because each is already an id, a text and a digest that phase 3 keys evidence by.
2. The generator proposes an issue per criterion plus the connective work between them, and attaches
   `serves: brief:…@n#c-…` **itself**. A proposed issue with no `serves` is a bug in the generator,
   not a choice.
3. A review screen merges, splits and re-points before anything is written. Merging two proposals
   unions their `serves`; splitting copies it.
4. Then *uncovered* means something: a criterion with no issue serving it, computed rather than
   asserted.

**Cost:** 2 new tasks, ~6 h, beside tasks 88 and 136. **Prerequisite:** §5.1's fragment grammar, so
`serves` can name a criterion in the same key form everything else uses — which is a reason to land
this design first.

### 9.2 The no-op pass over the managed block (summary §3 product 5)

**Do this first of everything; it is about an hour and §5.4 makes it urgent.** `claudemd.Render`'s
block opens with two human-facing lines — who wrote it, and not to edit it here. They change no
agent's behaviour and they are paid for in **every session on that branch, for ever**.

Move them into the marker comment, where the machine-facing `brief=` / `rev=` metadata already
lives. The marker is not rendered by a Markdown viewer, so a person opening the file still sees them;
an agent reading the file does not pay for them twice a day.

Run the same lens over the rest of the block **before** the two new sections land: for each line, ask
what an agent would do differently without it, and delete the ones with no answer. Apply it, do not
obey it — some of our repetition guards a silent failure and is load-bearing.

**Cost:** ~1 h, task 05 only, plus the goldens it moves — which task 26 is touching anyway.

---

## 10. What this design does not verify

1. **No page was fetched for this document.** Every claim about the source method is carried from
   `01-define-skills.md` and `00-summary.md`, which state they fetched
   <https://www.aihero.dev/grill-with-docs>, <https://www.aihero.dev/skills-grill-me> and the MIT
   repository <https://github.com/mattpocock/skills> on 2026-09-28. I re-fetched nothing; if the
   corporate proxy blocked anything, it blocked it for them and is recorded there.
2. **Phase 4's plan was read in the `phase-4` worktree**, at `docs/superpowers/plans/`
   `2026-09-23-phase-4-discover-parts/`, read-only: `task-03.md` in full to line 400, `task-05.md`
   to 175 and 262–335, `task-06.md` to 250, `task-09.md` to 60, `task-18.md` to 70, `task-21.md` to
   40. **Tasks 07, 08, 10, 11, 12, 13, 15, 16, 19, 22 and 23 were not opened.** The amendments named
   for 11, 15 and 16 in §8 are inferred from how `task-03.md` and `task-06.md` describe them, not
   from those files. A plan editor must open them before sizing.
3. **`docs/DESIGN-SYSTEM.md` was read by heading and token grep only.** §6's rules are carried from
   task 18's and task 21's own summaries of it, which quote the constraints directly. Re-read the
   file before writing the cards.
4. **The Hub contract was not opened.** §4.4's ledger claim rests on
   `2026-09-23-phases-4-5-7/provenance-contract-draft.md` §2.1 lines 160 and 183–188, read
   2026-09-28. Whether the Hub's brief-revision envelope caps `content` in a way the two new
   collections could reach is **unverified**: `task-03.md` cites a 64 KiB envelope refusal that the
   answer caps were sized to make unreachable. **The inline caps of §5.4 rule 6 bound the render,
   not the stored content** — sizing the storage against that envelope is an open question for the
   plan editor.
5. **No code was read.** Every type quoted is from the plan's "complete source" blocks, which the
   plan itself says the built code may differ from in wording.
6. **Nothing was installed, no plan was edited, and no file outside this directory was written.**
