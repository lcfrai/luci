# 11 — Design round 2: the project glossary and the project decision record, beside the brief

**Status: complete, 2026-09-29; reviewed by `12-sceptic-r2.md` (Needs fixes) and revised once:
see Revision 1 at the end.** Round 1 (`05-design.md`) failed
its sceptic (`06-sceptic.md`). The owner then decided `OD-36`
(`10-decision-terms-beside-the-brief.md`): terms and decisions are their own Hub records that
reference a brief revision. This round designs that.

Inputs, in the order read: `11a-facts-phase4.md`, `11b-facts-hub.md`, `06-sceptic.md`,
`10-decision-terms-beside-the-brief.md`, `00-summary.md`, `01-define-skills.md`,
`docs/AGENTIC-UX.md`, `docs/DESIGN-SYSTEM.md`.

## 0. The design in one screen

**Terms and decisions become two new Hub record kinds beside the brief. Each names the brief
revision it settled in, the Materialiser hands every session the whole set under one stamp, and
the brief does not change by a byte.** The price is about 69.5 hours before fix rounds: 42 in
phase 4, 27 in the Hub and 0.5 for `CONTEXT.md`, with one task, about 3.5 hours, joining phase 4's
critical path (§7.5).

1. **Two records** (§2). A **term** (`te-` id) is one word, what it *is*, the words to avoid and
   why. A **decision** (`dr-` id) is one sentence with its context, what it gave up, what undoing
   it would cost and up to three rejected alternatives. One revision of either is named by a
   `{kind, id, revision}` struct, never a new key form, so the key grammar does not move (§2.1).
   Each is a head plus immutable numbered revisions, written through its own batch route under
   `baseRevision`, with an op for every transition and an inverse for every gesture. A project
   owns them; a branch may add its own, depart from the project's (redefine an entry, or set it
   aside: *not here*) and promote. `settledIn` names the brief key where each settled, or `null`
   outside a brief. A duplicate word is refused when it is written.
2. **One gesture** (§3). The agent proposing a decision applies three limbs (costly to undo,
   surprising to a cold reader, a real fork) and proposes nothing that fails one. The person
   records with **Record**, or **Record all** for a round: optimistic, with Undo, and no checkbox,
   no gate field and no disabled Save. What fails a limb stays an ASSUMED row on the Desk, goes into
   the brief or the code, or goes nowhere (§3.3).
3. **Rounds are process state** (§4). Once a brief revision is complete, the assistant makes one
   suggestion round of at most three decisions and five words, held in one local file in the
   control plane's state: never in brief content, never in the Hub. It dies in five ways and has no
   closed state, so nothing ever needs reopening.
4. **The thread** (§5). Before the harness starts, the Materialiser writes
   `.panacea/discover/decisions.md` and `terms.md` in full, then the managed block: whole entries
   up to an inline budget, and always a line naming both files. The marker, both files, `Ref`'s new
   `glossaryDigest` and `decisionsDigest`, and a per-start inheritance record certify the **whole**
   set, so no stamp ever covers a truncation. The set is the one the session's stamp names: a
   project key gives the project's, a branch key its branch's (§5.1). Design and Deploy change
   nothing (§5.4).
5. **The lens** (§6). A word shows *suggested* (the drafted glyph, muted), then *recorded* (one
   flash of about 600 ms), then *taken up* (a dotted underline wherever the brief's answers use it),
   and carries *settled at brief rev n*. No count, no badge and no queue item of its own.

**What moves in built code** (§7.2): phase 4's `ref.go`, `render.go`, `materialise.go`,
`inspect.go`, `api/materialiser.go` and `discoverhub/read.go`, phase 2's create and restart
handlers in `api/sessions.go`, and the Hub's `hubapi` and `hubclient` with their contract-reading
tests, each through a new task, never a re-plan of an approved one. **What does not**: the content
model and its 113 tests, the key grammar and both its parsers, the brief's key and the 64 plan
files that quote it, the interview's questions, phase 2's session record, and phases 3, 5 and 7.
The Hub contract gains an appended §18 and moves no line (§7.4, A20).

**What the owner is asked** (§8.2): nine questions, each with its default already taken. **Q2
must be asked before 44a, Hub 45 or phase 4's 26 starts**: whether terms and decisions are named
by a struct, the default and OD-36's row read as written, or by two new key forms. Its other
answer, taken after those tasks ship, rebuilds them. The other eight block nothing: an unanswered
one is built as its default.

## 1. The sceptic's findings, answered by number

Each of `06-sceptic.md`'s sixteen findings in sections 1 to 5, then its sections 6 and 7. *Changed*
means this design differs from round 1 on that point; *no longer applies* means OD-36 removed the
thing the finding was about.

### Section 1, redundancy

- **1.1 The gap is real** (`06:22-29`). *Kept, and it is the whole of this design.* A project's
  words and its decisions are captured nowhere today; §2 makes each its own Hub record. Nothing
  else of round 1's wider scope is carried into this round.
- **1.2 The criterion citation is not a gap** (`06:31-47`). *Changed: the `#c-` fragment is
  dropped entirely.* A criterion is cited the way phase W already cites one, `{briefKey,
  criterionId}` (`06:38`), and this design adds no criterion citation of any kind. A term or a
  decision names the brief by `settledIn`, a whole brief key, never a fragment of one (§2.5). The
  brief-to-work generator that round 1 made depend on the fragment is not part of this design.
- **1.3 The premise under round 1's §5.1 is stale** (`06:49-58`). *Agreed, and used.* A new
  `kind:` is the grammar's designed extension (`PC:104-107`) and phase W already plans
  `issue:is-…@r`, so this design adds `term:` and `decision:` as two more forms (§2.1). The thread
  survives for the reason it survives `issue:`: each new record **references** a brief key, so
  the brief revision stays the anchor every surface cites.
- **1.4 The rounds' reuse was overstated** (`06:60-70`). *Changed: no reuse is claimed.* Task 21
  stays the one-shot import it is and is not amended (§7.4). The suggestion round is new code,
  task 30 (3 h), and new screen states, task 31 (4 h), priced as new work (§7.3).
- **1.5 The provability seed re-asks what `done` asks** (`06:72-86`). *Changed: dropped.* No
  question is minted per criterion, the `done` question stays exactly as built, and the evidence
  phase 3 keys to a criterion's id and digest remains the only provability mechanism. The interview
  gains no question at all (§4.1).

### Section 2, cost

- **2.1 Built code was priced as paper** (`06:92-127`). *Changed.* §7 prices each built file by
  name, with the task that built it and the tests and goldens it moves (from 11a §6), and says what
  the hours leave out: fix rounds. The largest of the four built surfaces, the content model (76 +
  37 tests, the inline golden, wire format v1), is **not touched**, so no `ContentVersion` question
  arises and `discover.Apply` gains no op: terms and decisions have their own ops, in the Hub's
  batch route (§2.4). What does move in built code is new tasks (25-29 and 32 in phase 4, 45-48b in
  the Hub), never a re-plan of an approved one. Revision 1 adds the five built changes round 2
  missed and reprices four tasks against the plans' own work (§7.2, §7.3).
- **2.2 The key grammar is the Hub's** (`06:129-151`). *Changed, and settled in Revision 1.* The
  brief's form gains no fragment and does not change by a byte (§2.1), so none of the 64 plan
  files that quote `brief:bf-` moves. Terms and decisions add **no** key form either: one revision
  is named by a `{kind, id, revision}` struct, 06's own alternative (`06:348-351`) and OD-36's *the
  grammar does not move* read as written. Neither parser changes, and the contract gains an
  appended §18 rather than grammar. Two key forms stay the owner's alternative, asked before any
  task that would build them (§8.2, Q2).
- **2.3 Re-anchoring the frozen plans was unpriced** (`06:153-170`). *No longer applies to phases W
  and T*: nothing in this design lands in either. In the two plans it does touch, new tasks go in a
  new section at each plan's end with new part files (§7.3), so neither task table moves. The Hub
  contract is **not** edited in place: A20 is an appended §18, after §17 at its end, so none of its
  517 inbound citations moves, and no contract-reading test goes red before the code it checks
  lands (§7.4).

### Section 3, the agent and our user

- **3.1 Five pickable questions, not a chat window** (`06:176-203`). *Changed.* No question is
  added to the interview. A round is at most three decisions and five words, made only after a
  revision is complete, each with its recommended answer already filled in, one gesture each or
  **Record all** for the round (§3.2, §4.1). Round 1's decision form, with three mandatory
  checkboxes, a fourth conditional field and Save disabled until all four, is gone: the test is the
  agent's (§3.1), the person's one mandatory field is one sentence, and Save is live from its first
  character. The bar held to is `CLAUDE.md`'s wall of configuration, not only the visual rules.
- **3.2 Three mechanisms that do not transfer** (`06:205-220`).
  1. *Wait between rounds assumes a session.* **Changed.** A round is local process state that
     carries its base, the brief key and both set digests. A round whose base moved is
     re-checked, never applied, and there is no `grill_round` op to guard; every write that follows
     is guarded by `baseRevision` and by the word index (§2.4, §4.3 item 2).
  2. *A stall detector needs an interlocutor.* **Changed.** There is no stall detector, no closed
     state and nothing to reopen; an untouched round dies of age without closing anything (§4.3
     item 4).
  3. *Model-proposed words stay invisible until per-row review.* **Changed in part.** A proposal
     still reaches no agent until a person records it: a draft is never inherited, the brief's own
     `4-D2` rule (`content.go:57-59`). But recording a whole round is one gesture, and suggestions
     appear where the person already is. What remains is that throughput is bounded by the
     person's presence: §8 records it, with an owner question about pre-authorising words.

### Section 4, the thread

- **4.1 `Also defined:` hands an agent a word with no way to its meaning** (`06:233-244`).
  *Changed.* The read route exists: two files under `.panacea/discover/` hold every entry in full,
  on disk before the harness starts; the block **always** names them; past its budget it lists the
  remaining words and says where each is defined; and every stamp (the marker, the `Ref`'s digests,
  the inheritance record) certifies the whole set, never the inline part (§5.1-5.3). No stamp stops
  being true at any entry count.
- **4.2 A loose-file glossary is forbidden for the product and recommended for us**
  (`06:246-252`). *No longer applies to the product*, whose words are Hub records with numbered
  revisions (`OD-3`, OD-36). The projected files are a rendering of stamped revisions, never a
  source: nothing reads a definition back out of them, the rule `Inspect` already keeps for the
  brief (`render.go:266-267`). **The sentence that distinguishes the two**: our own `CONTEXT.md`
  is right for this repository because this repository has no Hub, so its revision is the git
  commit and its citations are `file:line`; a product project has a Hub, so a loose file there
  would be a second, unrevisioned source beside the record, which is the defect.
- **4.3 Process state inside immutable content** (`06:254-260`). *Changed.* Round state is in
  neither `Content` nor the Hub; it is the control plane's local process state (§4.2), and a brief
  revision is minted by a brief change and by nothing else.

### Section 5, contradictions an implementer would meet

- **5.1 A branch's dissent was refused by `Apply`** (`06:268-282`). *No longer applies.* A
  departure is its own record, `overrides` naming the project entry's pinned revision, written
  through the Hub's batch route rather than `discover.Apply`, so no single content object has to
  hold both. What the branch sees is phase 4's overlay of two lists (§2.3). Since Revision 1 a
  branch can also say *not here* to a project **decision**, which is this finding's own case: a
  departure born `set_aside` (§2.3).
- **5.2 Retiring an inherited term had no op** (`06:284-290`). *Changed.* A departure is born
  `retired` for a word or `set_aside` for a decision, and sets the project's entry aside on one
  branch; `retire` retires a word for the project. Undo writes `restore`, so every gesture has an
  inverse, promote included (§2.4, Revision 1).
- **5.3 The id collision was resolve-time** (`06:292-296`). *Changed.* Ids are the Hub's
  (`ids.Wide`), so two records cannot share one. The collision that matters is the **word**, and
  the Hub refuses it at write, inside the transaction, by a unique index: 409 `word_taken` (§2.4).
  A departure is another scope by construction, so it is never a collision.

### Sections 6 and 7

- **6, intellectual property** (`06:300-319`). The three limbs are written in our own words
  (§3.1), and no limb label is ever on-screen copy: the lens shows the agent's own sentences in
  `undo`, `context` and `why`, never a checklist. Nothing of the upstream text is in this document.
  The upstream `LICENSE` file is **still unread** (`10-decision-terms-beside-the-brief.md:80-85`);
  nothing here depends on it, because nothing was copied.
- **7, the one change worth making now** (`06:323-343`). *Taken in.* The two human lines move into
  the marker comment (§5.1 item 1) inside task 28, which regenerates the same three goldens anyway,
  so the regeneration is paid once. If the owner wants it sooner it can still land alone first, at
  the price of regenerating the goldens twice.
- **7, the shelf conditions** (`06:345-351`). Of the three, *`ContentVersion = 2`* is moot, because
  content does not change, and *drop the criterion fragment* is met (1.2 above). *Phase 4 ships
  first* is **deliberately not met**: 29 and 32 land before phase 4's task 14, so that 14 wires the
  final Materialiser once. The owner's note on OD-36 licenses exactly that: *"we are still in
  build, so if you need to redo some work, it is what it is"* (`owner-decisions.md:409`). The
  third condition's last clause, *give terms and decisions the same shape* as `{briefKey,
  criterionId}` (`06:348-349`), is **met since Revision 1**: one revision is named by a `{kind, id,
  revision}` struct (§2.1), so no key form is added and neither parser changes. Round 2 took two
  key forms instead, and its sceptic showed that reading contradicts OD-36's row
  (`12-sceptic-r2.md`, C1). Key forms are Q2's other answer (§8.2).

## 2. The two records

Two new kinds, each **its own Hub record**: a head plus immutable numbered revisions, the shape the
brief already has (11b §1.4, `HC:742-815`). Neither is a field of brief content, and writing either
never mints a brief revision. `HC` is the Hub contract,
`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md` at `cafe1303`.

### 2.1 Ids, and how one revision is named: no new key form

| | Term (one entry of the glossary) | Decision (one entry of the decision record) |
|---|---|---|
| Hub id | `te-` + 16 lowercase hex, `ids.Wide("te")` | `dr-` + 16 lowercase hex, `ids.Wide("dr")` |
| One revision, named | `{"kind":"term","id":"te-<16 hex>","revision":<n>}` | `{"kind":"decision","id":"dr-<16 hex>","revision":<n>}` |
| Where the prefix is written | `hubapi/discoverids.go`, new, and the contract's appended §18 (§7.4, A20) | the same |

- **The key grammar does not move.** That is OD-36's *What does not move* read as written: "the
  brief content model, and the provenance key grammar, which the Hub contract owns"
  (`owner-decisions.md:409`), the grammar "two parsers compile" (`10:54-55`). No form joins the
  contract's §3.2, `hubapi.ParseKey` and `provenance.Parse` do not change, `/v1/refs` answers no new
  `kind`, and the brief's key, `design:` (`HC:119-122`) and phase 4's local `none:declined`
  (`key.go:23-29`) stay as they are. A revision is named by the three-member struct above, the way
  phase W names a criterion by `{briefKey, criterionId}` rather than by a key form: 06's shelf
  clause (`06:348-351`). Two key forms, `term:te-…@n` and `decision:dr-…@n`, are the owner's
  alternative, and the owner is asked before any task that would build either (§8.2, Q2).
- **`kind` is always written**, even where a field's position implies it, so a struct read out of
  context (an inheritance record, a set digest item, an event) names its record without a lookup.
  Nothing joins the three members into one string and nothing parses one: the struct is a JSON
  object on the wire and a composite key `(org_id, <kind>_id, revision)` in the Hub (§2.6).
- `te-` and `dr-` are in neither the Hub's prefix table (`hubapi/keys.go:16-27`) nor the control
  plane's `s-`, `p-`, `b-`, `w-`, `m-` (`HC:111`), nor phase W's twelve (`is-` … `df-`) and TM-05's
  `nt-` (`work plan:185-187`, where `work plan` is
  `.claude/worktrees/work/docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`); no `.md` or
  `.go` file in any worktree holds a `te-` or `dr-` id
  (grep, 2026-09-29). They stay out of `keys.go`'s `Prefix` block, as phase W's stay out in
  `hubapi/workids.go` (`work plan:236`), so the ten-check (`keys_test.go:756-758`) keeps its ten.
- The illustration `term:tm-7f2a@2` in `10-decision-terms-beside-the-brief.md:14` is corrected:
  `tm-` is the team prefix (`HC:107`), an id is 16 hex, not 4 (11b §1.2), and it is a struct.
- Revisions are the Hub's, from 1, gapless per record, and 0 never appears in a struct: the brief's
  rule (`HC:124-125`) applied to both. A struct names one immutable revision, so what a session was
  given can always be read back; only the head moves.

### 2.2 Fields

Every text field passes the block's neutraliser and line-ending rule (`render.go:47-54`) and holds no
line ending. The limits are round 1's (`05-design.md:243-248`), kept.

| Term field | Rule |
|---|---|
| `word` | the one word the project uses for the thing; 1-64 code points; unique among the in-force terms of its scope, compared case-folded, **checked by the Hub at write** (§2.4) |
| `is` | what the thing *is*, one or two sentences, 1-300 code points; never how it is built |
| `avoid` | 0-8 words not to use for it, each 1-64 code points, distinct from `word` and each other |
| `because` | 0-300 code points: why this word and not the avoided ones; the agent fills it, a person may leave it empty |

| Decision field | Rule |
|---|---|
| `decision` | what was decided, one sentence, 1-300 code points: **the only field a person must write** |
| `context` · `why` · `undo` | 0-300 each: why it came up; what it gave up; what reversing it would cost. The agent fills all three (§3) |
| `rejected` | 0-3 alternatives, each 1-120 code points: the *over Y* of the Desk's *chose X over Y* |

| Both kinds, every revision | Rule |
|---|---|
| `scope` | `project` (anchored to the `pj-` id), or `branch` (anchored to that branch's brief id, `bf-…`) |
| `state` | a term: `in_force`, `retired`. A decision: `in_force`, `superseded`, `withdrawn`, and `set_aside`, which only a departure holds: *not on this branch* |
| `overrides` | branch scope only: the struct (§2.1), revision pinned, of the project entry of the same kind this branch departs from; else `null` |
| `replacedBy` | on `retired` or `superseded`: the id of the record that took over; else `null` |
| `settledIn` | the brief key where it settled (§2.5), or `null` when that was a session with no brief |
| `author` | `person`, or `model_accepted` — the brief's own words, `content.go:63-65`. No `model_drafted` record exists: a draft is a proposal, which is process state (§4) |
| `sessionId` | the control-plane session it was proposed in, verbatim, as a session link holds one (`HC:937-944`); else `null` |
| `savedAt` · `savedBy` | the Hub's, as on a brief revision |

A branch scope anchors to the branch **brief's** id, not to a ref name, so a renamed branch needs no
new op: the brief's own `PATCH` rebind carries it (`HC:780-783`), and its archive retires nothing.

### 2.3 Scope: the project's, and what a branch may do

A term or a decision is the **project's** by default: one set, shared by every brief in the project,
which is OD-36's scope ground (`owner-decisions.md:409`). A branch may do three things, each its own
op (§2.4) and each its own record, so no one content object ever holds an entry and its dissent:

- **Add** a branch-only word or decision: `scope: branch`, `overrides: null`.
- **Depart** from a project entry: a branch record whose `overrides` names that entry's revision
  `n`, born in one of two states. `in_force` defines the word or the decision differently on this
  branch. *Not here* is a term born `retired` or a decision born `set_aside`: the entry does not
  apply on this branch and nothing replaces it, which is 06 §5.1's own case. Pinning `n` means a
  later project revision never silently re-points it; the lens says the departure was made against
  an older definition (§6).
- **Promote** a branch entry to the project: a new revision of the **same** record with
  `scope: project`, so its id and history survive. When it was a departure, the same batch retires
  or supersedes the project entry it overrode, with `replacedBy` naming the promoted record. Only an
  `in_force` entry is promoted: setting a project entry aside for the whole project is that
  entry's own `retire` or `supersede` (§2.4), not the promotion of a *not here*.

A branch record anchors to a brief that ends archived when its branch is deleted, landed or not
(`HC:785-788`). An archived brief takes **no new** branch record: 409 `brief_archived`, as a write to
the brief itself is (`HC:787`). **Promote** and `retire` of the records already anchored to it stay
open, and that is how a landed branch's own decisions and words reach the project (§6.2).

What a branch sees is computed by phase 4, never by the Hub (the Hub stores and resolves, 11b §1.4):
the project's in-force entries, minus those a branch record overrides, plus the branch's own.
One more rule keeps that overlay to one entry per word. A project word written **after** a branch
already defines the same word on its own is shadowed on that branch as if the branch had departed
from it, and the lens gives the entry task 19's *project changed* treatment (§6.2): **Keep this
branch's** makes the departure explicit, a `revise` that sets `overrides`; **Use the project's**
retires the branch's record in favour of it. The Hub allows that project write, because refusing
it would let one branch block the project's glossary.

### 2.4 Ops: one for every transition

Every write goes through **one batch route per kind**, `POST /v1/projects/{projectId}/term-revisions`
and `…/decision-revisions`, shaped exactly like `brief-revisions` (`HC:798-815`): 1-16 items, one
transaction, all or nothing; each item names its `baseRevision`, `0` for a new record; the Hub
mints the id and numbers the revision `baseRevision + 1`, never merges, and answers 409
`revision_conflict` on a stale base. A retry carries the same `Idempotency-Key` (`HC:50-53`, `:814`).

**Inside one batch.** An item may name a record that an earlier item of the same batch creates, by
that item's index, `{"item": 0}`, wherever a struct or an id is due; the Hub puts in the id it
mints, because no caller can know it. Heads are locked in id order, as the brief's are in `briefId`
order (`HC:808`). Items apply in **two passes**: first every item that takes a record out of force,
then every item that puts one in force or keeps it there, each pass in request order. So a promote
that retires what it overrode never trips its own unique index.

| Op | Transition | The item |
|---|---|---|
| `create` | nothing → `@1`, `in_force` | `baseRevision: 0` and the fields |
| `revise` | `@n` → `@n+1`, same state | a better definition, or a rename: `word` changes, the id and its history stay |
| `retire` (term) | `in_force` → `retired` | optional `replacedBy`: two words turned out to be one thing |
| `supersede` (decision) | `in_force` → `superseded` | `replacedBy` required: an earlier record, or an item of this batch |
| `withdraw` (decision) | `in_force` → `withdrawn` | it should never have been recorded; nothing replaced it |
| `reinstate` | `retired` or `withdrawn` → `in_force` | the word check below runs again |
| `depart` (branch) | a project entry's `@n` → a new branch record, born `in_force`, or *not here*: `retired` for a term, `set_aside` for a decision | `baseRevision: 0`, `scope: branch`, `overrides` naming `@n`, and the state it is born in |
| `promote` (branch) | an `in_force` branch record → project | a `revise` with `scope: project`; the batch also retires or supersedes the entry it overrode |
| `restore` | `@n` → `@n+1`, holding the fields, scope and state of an earlier revision | `restores: <that revision>`; **what Undo writes**, below |
| `erase` | every revision → no content, and the head leaves force, as an erased brief's head is archived | never an item of the batch route: the org admin's erase route, through an owner function (§2.6) |

**Undo** (§3.2) writes `restore`, as the brief's Undo writes a `restore` revision (phase 4
`task-19.md:7-9`), so nothing is deleted and the history says what happened. It restores the
revision before the gesture: *Use the project's* is undone by restoring the branch record, and a
promote by one batch that restores the promoted record and the project entry it retired, applied
exits first. So every gesture has an inverse. A `create` has no revision before it, so its Undo is
`withdraw` or `retire`, which leave force and need no word check. `restore` skips rule 2 below,
because the state it restores is one the Hub already held; no write skips rule 1.

**Write-time checks, in order**, each its own code: the struct and §2.2's field rules (400);
`settledIn`, `overrides` and `replacedBy` each name a non-erased record in **this** project, and
`overrides` a project-scoped one of the same kind (422 `key_unresolvable`, the design approval's
code, `HC:887-897`); a new branch record's brief is not archived (409 `brief_archived`, §2.3); then
the word check, 409 `word_taken`, naming the entry it met. The word check is three rules:

1. **One in-force entry per word in a scope**, by a unique index on the head's folded word inside
   the write's transaction. A collision is refused when it is written, never found when resolved.
2. **A branch word against the project's.** A branch record that departs from nothing is checked
   against the project's in-force words, so a branch cannot hold a second meaning of a project word
   without saying that it departs; the lens offers **Depart on this branch** (§6.3). A departure,
   and a `revise` that renames one, is checked against every project word but the one it
   overrides. If a project write of the same word races this rule, the result is §2.3's
   later-project-word case, which the overlay already handles, so no lock is needed.
3. **No word both used and avoided.** A `word` that another in-force term on the same branch's view
   lists in `avoid`, or an `avoid` naming such a term's `word`, is refused. Two machines therefore
   cannot record *lane, not track* and *track, not lane* (rounds are per machine, Q5). Retiring the
   other term in the same batch clears it, since exits apply first.

**One fold, in one function**: `hubapi.FoldWord`, NFC and then Unicode simple case folding, from
`golang.org/x/text`, already in both modules as an indirect dependency (the Hub tree's
`control-plane/go.mod:62`, phase 4's `:56`). The head stores its result for rule 1's index, rules
2 and 3 compare with it, and phase 4's overlay and sort (§5.1) call the same function, which
reaches phase 4 by merge with the rest of `hubapi` (§7.2).

**One owner for §2.2's rules: the Hub.** They are one `hubapi` function beside the wire types. The
batch routes run it and refuse 400; phase 4's `discoverhub` runs the same function before it writes,
and task 25 builds no copy, so the two sides cannot drift. This is the one place the Hub reads a
record's fields for meaning, and the record is not a brief: the contract's "It reads exactly one
field of a revision's content: `status`" (`HC:742-745`) is about brief content, which stays opaque.
The contract's §18 says that in words (§7.4, A20).

**Roles are the brief's** (`HC:581-588`): `V` for the two reads, `M` for the two batch writes
(`HC:329`), and erasure the org admin's alone, as the brief's is (row 45, `HC:607`).

Two reads, which with the two batch writes and the two erasures (§2.6) make six new routes. They
are written in the contract's appended §18, not as rows inserted in §7.0's closed table
(`HC:557-607`, A20): `GET /v1/projects/{projectId}/terms` and `…/decisions`, each the heads of one
project, filterable to one branch brief, every revision's fields as §2.2 lists them, and each branch
head carrying its brief's `archivedAt`. A struct therefore resolves from its kind's list. The
resolver `GET /v1/refs` (row 27, `HC:589`) does not change: no struct reaches it, and it answers no
new `kind`.

### 2.5 The reference to the brief revision it settled in

`settledIn` says **where** a word or a decision settled, not what it depends on: it stays in force
when the brief moves on, which is OD-36's lifecycle ground. Its three sources:

- **The suggestion round** (§4): the complete revision the round was computed from, its base. A
  round whose base moved is re-checked before it is shown (§4.3 item 2), so a recorded suggestion
  never names a revision it was not checked against, and since a round exists only for a complete
  revision, head 0 never needs naming. A round changes no brief text, so it never has a brief
  revision to write first. If the batch is refused, its suggestions stay pending and the row says
  why (§6.3); each batch is all or nothing.
- **A session**: its `contextRef` when that is a brief key; `null` when it is `""` or
  `none:declined`, and the lens says *settled outside a brief* rather than inventing one.
- **A person in the lens**: the head revision of the brief the lens is showing, or `null` at head 0.

The Hub checks that a non-null `settledIn` resolves in this project and is not erased. It does
**not** require the revision to be `complete`, because words settle while a brief is still open.

### 2.6 Storage, digests, events and erasure

- **Tables** `terms` + `term_revisions` and `decisions` + `decision_revisions`, in a new
  `0002_terms_decisions.sql`, because a written migration is append-only (`ErrEdited`, 11b §3.3).
  The revision tables are append-only, with the canonical-envelope and digest `CHECK` that only
  `brief_revisions` and `audit_events` carry today; each head is a guarded state machine holding the
  current folded word, scope and state for the unique index. Every reference is a composite
  foreign key, never a key stored as text: `settledIn` is `(org_id, brief_id, brief_revision)`, as
  every brief reference is (11b §3.2); `overrides` names a revision of its own kind,
  `(org_id, term_id, revision)` in `term_revisions` and its twin in `decision_revisions`; and
  `replacedBy` names a head, `(org_id, term_id)` or `(org_id, decision_id)`. Every list in `0001`
  that names tables one by one (RLS, grants, the
  append-only array: 11b §3.4) gets its twin statement in `0002`.
- **Digests**: two new canonical envelopes, `{"v":1,"kind":"term-revision",…}` and
  `"decision-revision"`, members sorted as the brief's are (`discoverhub/digest.go:26-49`), digest
  `"sha256:"` + hex; each gets golden vectors beside the brief's (§1.7 of 11b).
- **Events**: `term.revised` and `decision.revised`
  `{"kind","id","revision","digest","state","scope"}`, one per changed record, and `term.erased`,
  `decision.erased`. `data` never holds the text (`HC:1133-1134`).
- **Erasure**: `hub.erase_term` and `hub.erase_decision`, owner functions shaped like
  `hub.erase_brief` (`HC:1115-1127`), each reached the way the brief's is through row 45: `POST
  /v1/terms/{termId}/erase` and `POST /v1/decisions/{decisionId}/erase`, org admin only, with the
  same `reason`, because a definition can hold client text just as an answer can. The text goes;
  the id, digest, number, `savedAt` and `savedBy` stay, and the revision reads `erased` in its list.
- **No stored reply keeps the text.** A route-26 reply is stored whole for 24 hours in
  `idempotency_keys.response`, which is why A17 exists (`HP:196`; `HP:` and `T:` as §7 defines
  them). So the two batch routes store a text-free reply, and a replay rebuilds its revisions from
  `term_revisions` and `decision_revisions`, an erased one with its text `null`: route 26's
  `rebuildRevisions` under task 14's `WriteReplayable` (`T:19.md:52-59`), applied to both kinds.
  Task 27's `TestErasureKeepsKeyAndDigest`, whose `markerHits` already scans every `text`,
  `varchar`, `jsonb` and `bytea` column (`T:27.md:72`), gains a term and a decision holding its
  marker, each written under an `Idempotency-Key`, erased, then found nowhere and replayed
  text-free.
- **A brief's erasure names what settled in it.** A term's `is` or a decision's `context` may have
  been drafted from the very answers a brief's erasure removes (§4.1), so "neither holds brief
  text" cannot be assumed. Erasing a brief still changes no term and no decision by itself, as it
  changes no approval, because a word like *lane* is rarely client text. But its reply gains
  `settled`: the struct of every term and decision revision whose `settledIn` names the erased
  brief, one query on that composite key. The org admin who erased the brief then erases what
  holds client words, through the two routes above. Nothing is erased that a person did not choose.

## 3. How a decision earns its record, without ceremony

### 3.1 The test belongs to the agent, and it is three limbs or nothing

In our words (the upstream labels are never on-screen copy, 06 §6):

1. **Costly to undo.** Reversing it later moves data, code or people; it is not a setting.
2. **Surprising to a cold reader.** An agent or a teammate arriving without this conversation would
   guess the other way.
3. **A real fork.** A defensible alternative existed, and choosing this one gave something up.

The **agent proposing** a decision applies the test, and proposes nothing unless all three hold.
What it wrote for each limb becomes a field: `undo` for the first, `context` for the second, `why`
and `rejected` for the third (§2.2). The person never sees a checkbox, the record carries no gate
field, and the Hub checks lengths, never limbs: round 1's three booleans and `ErrGateNotMet` are gone.

### 3.2 Where a proposal comes from, and the one gesture that records it

| Source | Where the person meets it | The gesture |
|---|---|---|
| The Discover assistant, once the brief's answers settle (§4) | a suggestion row in the Discover lens, its recommended answer already filled in | **Record**; or pick the other answer, then Record |
| The person | the lens's *Record a decision*: one field, the `decision` sentence | **Save**, enabled from the first character. The assistant then drafts `context`, `why` and `undo` as a suggested revision the person may take (`model_accepted`) or leave |
| A working session, once the Desk exists | the Desk's *decided without you* row (`docs/AGENTIC-UX.md:114-119`), which already shows *chose X over Y*, *because* and *rejected* | when the agent judged all three limbs, the row gains one verb, **Accept and record**, beside *Accept*, *Do it the other way* and *Rewind to here*, which do not change |

The third source is designed here and **not built by this design**: the Desk's row is in
`AGENTIC-UX.md` and two planning notes, and in no phase plan and no code (grep of every worktree,
2026-09-29). Until it exists, decisions come from the assistant and the person.

A term is the same shape: the row shows the word, its one-line *is* and the avoided words, and
**Use this word** records it.

**Every gesture is optimistic, with an Undo window, never a confirm dialog**
(`docs/DESIGN-SYSTEM.md:69`), and resolves in place (`AGENTIC-UX.md:147`). Undo, for five seconds,
writes a `restore` revision, as the brief's own Undo does (phase 4 `task-19.md:7-9`, 11a §5); after
a first Record, which has no revision before it, it writes `withdraw` or `retire` (§2.4). Either
way the record's history stays honest, and every gesture has an inverse. When one round holds several
suggestions, **Record all** writes them in one batch, the Desk's *approve pattern*
(`AGENTIC-UX.md:109-110`). The batch is all or nothing (§2.4), so a refused one records nothing:
the row that caused the refusal says why (§6.3), the rest stay suggested, and a second **Record
all** takes them. That is the answer to the sceptic's throughput point (§1, 3.2 item 3): a
round costs one gesture, not one review per row. No suggestion is ever raised as a modal
(`AGENTIC-UX.md:246`).

### 3.3 The discard rule: what is not recorded, and where it goes instead

| It fails | For example | Where it goes instead |
|---|---|---|
| Limb 1: cheap to undo | a table rather than a list on the settings page | it stays an ASSUMED row on the Desk, with its own verbs (`AGENTIC-UX.md:29`), and in the transcript |
| Limb 2: anyone would guess it | the service answers in JSON | nowhere: the code says it |
| Limb 3: there was no fork | inference goes through the org's gateway (`OD-19` leaves no choice) | nowhere |
| It is a requirement, not a choice | it must work on mobile | the brief: a criterion or a constraint, through the interview's existing ops |
| It is *how*, not *what* | retry three times with a back-off | the code and its comments. A record that holds *how* becomes a second brief, and the thread then carries two descriptions (`05-design.md:250-254`, kept) |
| It is ours, not the project's | a choice about how PanaceaApp itself is built | our own owner-decision register, never a product record |

A **word** earns an entry when it is a noun specific to this project, defined by what it *is*, with
one word chosen and the others listed; general programming words stay out, however often a
project uses them (`05-design.md:250-254`).

**Few decisions is the normal outcome**, not a gap. A project with none shows no empty-state prompt,
no count and no "0 decisions": counts live only in the queue (`AGENTIC-UX.md:245`), and the lens's
decisions area is absent until the first one exists (§6).

## 4. The interview's rounds are process state

### 4.1 What a round is here, and how small

The built interview stays exactly as shipped: five closed questions and at most one follow-up
(11a §5). **No question is added to it**, so none of round 1's 40 questions over 6 rounds survives.
A round is something else: once a brief revision is `complete`, the Discover assistant makes **one
suggestion round** from it, the method's frontier (01 §1): the decisions whose prerequisites the
answers have now settled, each with a recommended answer and its alternatives, and the words the
answers use in a way specific to this project. Facts stay the assistant's job; the person decides.

- **At most three decisions and five words.** An empty frontier is the common answer, and it shows
  nothing at all.
- **At most one model call per complete revision**, shaped like the follow-up's: effort `medium`,
  a 20-second budget, the project and branch names, the answers holding a person's words and the
  two sets in force, never file contents (`assist.go:15-20`, `:35-49`, `:182-185`). Before a round
  is shown, each proposal is checked against those sets with §2.4's word rules, and one that would
  be refused is dropped, so a Record never meets a 409 the control plane could have seen. Its
  outcomes mirror the follow-up's six, none an error: with no model configured there are no
  rounds, and the lens keeps *Add a word* and
  *Record a decision*, which need no model.
- **Labelled, never counted**: *Suggested from your answers*. No round number and no total
  (`AGENTIC-UX.md:237`); the interview's one counter stays the literal *QUESTION n OF m* on a base
  question (`content.go:325-329`), and a round has none.
- A round **never gates the brief**: a revision is complete without one, and terms and decisions
  never count toward completeness (kept from round 1, 00 §2).

### 4.2 Where round state lives

In the **control plane**, as phase 4's local process state: one file per brief,
`<state>/discover/rounds/<briefId>.json`, written with the `statepath.WriteAtomic` the Materialiser
already uses (`materialise.go:185-191`). It holds the base the round was computed against (the brief
key and the branch's two set digests, §5.2) and each suggestion with its fields and a
state, `pending` or `dismissed`.

It is **never** inside `discover.Content`, so no round mints a brief revision or moves a digest
(the sceptic's 4.3), and **never** in the Hub, so nothing about a round is shared, numbered or
audited. There is no `grill_round` op, no round counter and no `Grill{…}` struct anywhere.

### 4.3 How it dies

1. **Acted on.** A recorded suggestion leaves the round, because it is now a Hub record. A
   dismissed one stays, marked, so the next round does not offer it again.
2. **Its base moved.** When the brief head or either set digest differs from the round's
   base — a teammate revised something — the round is not shown as it was. Each pending suggestion
   is re-checked against the new base: dropped if its word is now in force or its decision now
   recorded, kept otherwise. A decision has no word to match on, so it matches an in-force decision
   whose `decision` sentence is equal under `hubapi.FoldWord` (§2.4). A new model call happens
   only for a new complete revision. So a round
   is never applied against a stale base, and if two people race, the Hub's `baseRevision` and
   `word_taken` checks (§2.4) refuse the loser at write, with a row that says why.
3. **The brief is archived or erased.** The file is deleted.
4. **Fourteen days untouched.** The file is deleted; the next visit to a complete revision may make
   a fresh round. There is **no closed state and no stall detector**: an unanswered round closes
   nothing, so "skipped everything" and "not back yet" never need telling apart, and nothing ever
   needs reopening (the sceptic's 3.2 item 2).
5. **Another machine.** Round state is per machine. There the round is recomputed from the Hub's
   current brief, terms and decisions. A suggestion is derived, unconfirmed data, so nothing a person
   said is lost; the one cost is that a suggestion dismissed on the first machine may be offered
   once more on the second (§8 records it as an owner question).

**A session's proposal** (§3.2, third source) is the Desk row's own state, wherever the Desk keeps
it, and dies with the row. It never reaches the Hub until a person records it.

## 5. Closing the thread

### 5.1 What the Materialiser writes

The Materialiser's one call (`materialise.go:196`) now writes **three files, in this order, all
before the harness process starts**: `.panacea/discover/decisions.md`, `.panacea/discover/terms.md`
(§5.3), and last the managed block. Last, so a block that names a set is never on disk before the
files that hold it: the ordering rule (`materialise.go:3-16`) applied inside one call.

**The set follows the stamp: one brief key, one set** (the sceptic's C2). A session inherits the
set of the brief its `contextRef` names, and nothing else. A **branch** brief key gives that
branch's overlay (§2.3). A **project** brief key gives the project's in-force entries and nothing of
any branch. A branch whose own brief has no complete revision is stamped with the project's key,
because `Inheritable` is "the latest complete branch revision, else the latest complete project
revision, else none" (`discoverhub/read.go:77-80`). So its sessions get the project's set, and its
own entries reach sessions from its brief's first complete revision, exactly as its own answers do.
The lens says so on each such entry (§6.2), so nothing is dropped silently. This rule is what keeps
`Ref` one brief key resolved (§5.2). The other rule, set digests served per `(contextRef, branch)`
beside the `Ref`, would need the owner's reading of OD-36's "`Ref` carries their digests", so it
is not taken.

Both lists are read **to their last page** (`?after=` until `next` is `null`, `HC:48-49`) before
anything is digested, inside the same 5-second Hub budget as the brief (`materialise.go:81-87`).
If the Hub does not answer, or the budget ends mid-list, the start is refused `hub_unavailable`
exactly as it is today. A session never starts on half its inheritance, and no digest ever covers
a first page as if it were the set.

**No brief key, no inheritance.** A start whose `contextRef` is `""` or `none:declined` gets no
entry in any block and neither file: the Materialiser removes both files wherever it strips the
block today, so a session labelled *started without the brief* finds no Discover text in a file
this package writes (`materialise.go:110-113`, `:159`). Its inheritance record holds two empty
sets. The thread starts at a brief key because `Ref`, which carries the set digests, is a brief
key resolved. `""` means **nothing complete to inherit**: `Inheritable` ends "else none", which
covers a project whose first brief is still open as well as one with no brief at all
(`CONTEXT.md:130-132` names only the second). So words recorded while the first brief is open
reach sessions once it is complete, and the lens says so (§6.2); a person who declines the brief
declines them too.

**The block**, in order (today's order is 11a §4):

1. The begin marker gains the two set digests and takes the two human lines the sceptic asked to
   move (06 §7): `<!-- panacea:discover:begin brief=<id> rev=<n> terms=sha256:<hex>
   decisions=sha256:<hex> ; written by Discover in Panacea App, and regenerated: change it there
   -->`. The two fixed lines (`render.go:81-82`) are deleted, so every session stops paying for them.
2. `# <title>`, and the brief's sections, unchanged.
3. `## Decisions already taken`, only when there is one: per decision, `- <decision> Gave up:
   <why>`, because `why` holds what the choice gave up (§2.2), never the reason it won; and, for a
   branch record, an origin line in the style the brief's overrides already use (`_Set on this
   branch, departing from the project's: <old decision>_`, cf. `render.go:87-95`). A *not here*
   departure prints its origin line alone: `- _Not on this branch: <the project's decision>_`.
   `context`, `rejected` and `undo` are in the file, not the block.
4. `## Words used here`, only when there is one: per term, `- **<word>**: <is> Not <avoid>, <avoid>.`
   and the same origin line for a departure; a *not here* word prints
   `- _Not on this branch: **<word>**_`.
5. `---`, today's footer (`render.go:105-108`), then **always** one line naming the read route:
   `Every decision and every word, in full: .panacea/discover/decisions.md and
   .panacea/discover/terms.md.` It is there when everything fitted too, so an agent never has to
   know whether anything was left out to know where to look.
6. The end marker, unchanged.

**The inline budget.** Decisions get 4 KiB of the block and words 6 KiB, counted in rendered bytes,
**whole entries only**: a definition is never cut. Order is fixed so the bytes are stable: a
branch's own records first, then the project's; decisions oldest first, so adding one never
reorders the rest; words by `hubapi.FoldWord` (§2.4), then by id. Past its budget the words section
ends with `More in .panacea/discover/terms.md: <word>, <word>, …` — the remaining words by name, to
2 KiB more — so the agent knows *which* words exist and where each is defined. The decisions
section ends with `More in .panacea/discover/decisions.md.`, since a decision has no one-word name
to list; few projects reach it, because few decisions is the normal outcome (§3.3).

Everything else in the block's rules holds: the neutraliser, LF, no timestamp (`render.go:5-8`,
`:47-54`), untracked and excluded (`materialise.go:169-184`), written only when the bytes change.

### 5.2 What `Ref` carries, and what the session keeps

- **`Ref` gains `glossaryDigest` and `decisionsDigest`**, each `"sha256:…"`, the digests of the
  two sets now in force for the brief's own scope: a branch key, the branch's overlay; a project
  key, the project's and never a branch's (§5.1). That rule is what makes *two sessions with one
  `contextRef` share one `Ref`* true: every session stamped with a key was given that key's set.
  Not one member named *vocabulary*: the Work layer already uses that word for a team's filter
  names (11b §5.2). They are the third time-varying part of a `Ref`, beside `headRevision` and
  `latestCompleteRevision` (`ref.go:71-72`), and are refreshed the way those are, with one
  difference: a `term.*` or `decision.*` event invalidates the `Ref` of **every** brief in the
  event's project, because a project write moves every branch's overlay (task 16, §7.4).
- **A set digest** is `"sha256:"` + hex over a canonical envelope
  `{"v":1,"kind":"term-set" or "decision-set","scope":…,"items":[{"kind","id","revision","digest"},…]}`,
  sorted by id: every entry of the set, never only the ones that fitted inline.
- **The inheritance record**, new, phase 4's: one immutable entry per process start,
  `{sessionId, start, contextRef, terms: {digest, entries}, decisions: {digest, entries}}`, where
  `entries` are the structs (§2.1). It is kept where `contextRef` is kept, in the control plane and
  not the Hub (`contextRef` is the registry session record's, stamped at every create, restart,
  resume and fork, `PC:182`): one file per session, `<state>/discover/inheritance/<sessionId>.json`,
  which gains one entry per start through `statepath.WriteAtomic`, as §4.2's round file is written,
  and is never otherwise rewritten. It is never in the worktree, where the agent it describes could
  edit it.
- **The session handler writes it, after the stamp** (the sceptic's I1). Create mints its session
  id only after Materialise (`api/sessions.go:963`, `:1005`), and six exits between them abort a
  create before its record exists (`:977-1081`): a Materialiser-side write would record starts
  that never happened, the stamp the handler promises not to leave (`:973-974`, `:985-986`). So
  `MaterialiseResult` gains the two sets, each `{digest, entries}`, `MaterialiseRequest` does not
  change, and the handler appends the entry only once the record write that stamps `contextRef`
  has succeeded: after `Store.Create` on create (`:1037`), after the update that sets
  `sess.ContextRef` on restart (`:1817`), and before the sandbox exists. A failed append is handled
  as a failed sandbox create is (`:1103`): the session is marked exited with the reason, and
  no harness starts. This changes task 10's handler in phase 2's package, priced in task 29 (§7.3).

**Why that is provable.** Every struct in the record names an immutable revision that reads back
for ever (an erased one still reads `erased`, keeping its id and digest, §2.6); the set digest
recomputes from the structs; and the block and both files re-derive byte for byte from them and the
brief, because `Render` is pure. Develop compares the record's digests with the `Ref`'s two: equal
is *current*; different is *the project's words moved since this session started*, derived the way
the desktop already derives `superseded` from two numbers (`ref.go:75-76`).

### 5.3 The read route: every definition, past any budget

**The route is a file read, the one read every harness already has.** `.panacea/discover/terms.md`
and `.panacea/discover/decisions.md` hold every entry of the session's set, in full, each under its
id and revision (`te-3f2a91c07b1e4d2a`, rev 2), with a first line naming the set digest the marker
carries. `.panacea/` is already
the product's untracked projection directory: phase 5 puts its handoff bundle at
`.panacea/design/<canvasId>@<rev>/`, excluded through `.git/info/exclude` (`P5:231`, P5-G). The
Materialiser adds `/.panacea/discover/` to `info/exclude`, checks that git ignores it, and follows no
link below the worktree, the same three rules as the block (`materialise.go:169-184`, `confine.go`).
A *not ignored* answer, from a tracked file there or a `.gitignore` negation, refuses the start
`write_failed` before any of the three files is written, as the block's own check does
(`materialise.go:176-181`).

**So no stamp certifies a truncation** (the sceptic's 4.1). The marker, the files' first lines,
the `Ref`'s two digests and the inheritance record all carry the digest of the **whole** set, and
the whole set is on disk at the path the block names, before the harness starts. What the block holds inline
is a convenience with its own name, never the thing certified.

**The drift check reads the set digests before the text** (the sceptic's I2). `Inspect` renders
the marker's key and calls anything not `SameText` `edited` (`contextwrite/inspect.go:208-215`),
on the rule that "a key's render depends on nothing but the key" (`:32`). Here the render also
depends on both sets, which move whenever a teammate records a word, so a text comparison alone
would show that new word as a person's deletion. So once the marker's key matches, `Inspect`
compares the marker's two set digests with the `Ref`'s current two, and a mismatch is
`StateSuperseded`: the block names another set, as a marker naming another key is (`:205-206`) and
as `PinErased` already is (`:212-213`). Only when both match does it render and compare, so
`edited` still means a person changed the block. The files take the same rule: missing is
`pending`, a first line naming another set digest `superseded`, a hand edit `edited`
(`inspect.go:18-47`); no new state. Task 12's `materialisations` route reports the same states for
every worktree of the branch (`task-12.md:18`, `:131`), so its fresh render needs the two list
reads, and task 12 is amended (§7.4).

**Why a file first and MCP second.** An MCP tool reaches only a harness the control plane has
pointed at the sandbox's listener, and today that is Claude Code through `--mcp-config` from
`harness.claudeArgv` (`P5:141-151`); the channel is phase 5's and unbuilt (11b §4.1). A file reaches
every harness, in local mode and in the cloud, with nothing to configure. When phase 5's channel
lands, `get_terms` and `get_decisions` join `get_brief` on it (`P5T:15.md:10-11`), answering at call
time, as `get_brief` does, while the file stays what the session was **given**. The two answer
different questions, and neither is required for the other.

### 5.4 Every downstream consumer, one line each

From 11b §4, in its order. **Design and Deploy change nothing**: OD-36's *What moves* names the
Materialiser, `Ref` and the lens only (`owner-decisions.md:409`), and nothing below needs more.

| Consumer (11b §4) | Under this design |
|---|---|
| Phase 5 built: `review_mark`, `op.go:25`, `:59`, `:94`, `apply.go:1128-1140` | unchanged: a design is reviewed against a brief key, never a word or a decision |
| `P5:90`, the brief from `discoverhub.Service` | unchanged |
| `P5:183`, pin and mark | unchanged: a moved word or decision never supersedes an approval |
| `P5:216`, one key-source selector | unchanged |
| `P5:323-325`, G9: only `provenance` parses a key | holds: a term or a decision is a struct, never a key, so nothing new is parsed (§2.1) |
| `P5:534`, `design:` keys through `/v1/refs` | unchanged; `/v1/refs` answers no new kind (§2.4) |
| `P5T:08`, the review digest | unchanged |
| `P5T:15`, MCP read tools | gains `get_terms` and `get_decisions` later, optional (§5.3) |
| `P5T:20`, `Superseded` in `designapi` | unchanged |
| `P5T:21`, approval | unchanged: the approval pins the brief key and digest only |
| `P5T:22`, handoff and `LinkSession` | unchanged; a design agent's session inherits the words through the Materialiser like any other |
| `P5T:27`, `selectDesignBriefKeys` | unchanged |
| `P5T:28`, the Design lens chips | unchanged |
| `P5T:30`, end to end | unchanged |
| Phase 7 built: the watcher, `hubside.go`, `pins.go` | unchanged: reads no brief key and still none |
| `P7:104`, Ship's body | unchanged: no field for words or decisions (§8, owner question 3) |
| `P7:156` and `P7:841` | unchanged |
| `P7T:07`, `:08`, `:09`, `:10` | unchanged |
| `P7T:13`, `selectReleasePinKeys` | unchanged; the pin-keys-are-opaque test holds |
| `P7T:15`, `:17`, `:18`, pin cell, drawer, governance | unchanged |
| `provenance.Ref`, `ref.go:78-103` | **changes**: `glossaryDigest`, `decisionsDigest` (§5.2) |
| The one loader, phase 4 task 16 | **no new key source**: a `KeySource` returns keys (`task-16.md:165-167`), and an inheritance record's entries are structs, which resolve from `discoverSlice`'s two lists (task 16's amendment, §7.4); its `contextRef` is a session's key as before |

**Develop** is where the thread gains its hop: the Materialiser, the inheritance record and the
session view that compares it with the `Ref`'s two digests. Phase 3's merge and evidence code keys
to a criterion and never parses a key (11a §6), so it does not change either.

## 6. The Discover lens

The lens stays a view on a branch, not a stage: no order, no numbering, no completion state
(`CONTEXT.md:204-211`; `AGENTIC-UX.md:79`, rule 12). The brief stays its subject. Words and
decisions are two quiet areas **below** the brief, each **absent until its first entry exists**;
*Add a word* and *Record a decision* sit in the lens header as plain text actions, so an empty
project shows no empty panel and no prompt (§3.3).

### 6.1 How a word shows as it settles

1. **Suggested.** A suggestion from the round (§4) appears where the entry will live, marked with
   the brief's own drafted glyph `◌` and *suggested from your answers*, in muted text: the same
   mark task 21 gives a drafted answer (11a §5). Verbs: **Use this word**, **Change**, **Not this
   one**. A decision's: **Record**, the alternatives as picks, **Not worth recording**.
2. **Recorded.** On the gesture the row turns solid in place: the glyph goes, the text takes full
   contrast, and the change **flashes and decays over about 600 ms, `opacity` and `background`
   only** (`docs/DESIGN-SYSTEM.md:63`). That flash is the one motion here, and it explains
   something: this row just became a record. Undo sits inline for five seconds (§3.2).
3. **Taken up by the brief.** From the same moment every occurrence of the word in the brief's
   answers on screen gains a dotted underline, faded in by the same flash. Hover or focus opens the
   definition in a popover that changes no layout. The person sees the word join the brief's
   language, which is the thing OD-36 exists for.
4. **Its provenance.** Each entry carries, in secondary text, *settled at brief rev 4* as a link to
   that revision's history row (or *settled outside a brief*), and its id and revision in the
   licensed mono (`te-3f2a91c07b1e4d2a` · rev 2, `DESIGN-SYSTEM.md:45`). A revised entry shows
   its newest words and a *History*
   disclosure; retired, superseded and withdrawn entries are behind it, never mixed in.

### 6.2 A branch's departures, and what the lens never shows

- **A departure** reads *On this branch: <definition>* with *Project: <definition>* beneath in muted
  text, the same shape as the brief's override note (`render.go:87-95`). When the project entry has
  moved past the pinned revision, the entry takes task 19's *project changed* treatment (11a §5)
  with **Keep this branch's** and **Use the project's**. No new visual state is invented.
- **Not yet reaching sessions.** An entry on a branch whose brief has no complete revision, or on
  a project whose first brief is still open, carries one secondary line: *Reaches sessions once
  this brief is complete.* It states §5.1's rule where it applies. It is not a warning: no colour
  and no count, and it goes when the brief completes.
- **Promote** is a verb on a branch entry, never a prompt; archiving a branch asks nothing.
- **After a branch ends** (the sceptic's I7). A landed branch's own decisions and words are in no
  session's set: their brief is archived (`HC:785-788`), and main's sessions get the project's
  set (§5.1). So the lens on the project lists, beneath the project's own entries, the in-force
  entries of its archived branch briefs, behind one disclosure, *From archived branches*, each
  with its branch's name and **Promote** beside it, and one line: *Sessions get these once
  promoted.* The disclosure is absent when there are none. There is no prompt, no queue item and
  no count, and archiving still asks nothing. Asking at the land itself, in phase 3's merge, is the
  owner's Q8 (§8.2).
- **No count anywhere**: no *12 words*, no *3 of 5*, no badge on the rail
  (`AGENTIC-UX.md:237`, `:245`). **Nothing in this feature adds a queue item of its own**: the
  suggestions wait in the lens, where the person already is when they appear, and the one
  queue-bearing source is the Desk's row (§3.2), which is already a queue item.
- **Colour**: the accent teal only on the verbs and focus (`DESIGN-SYSTEM.md:38`); *recorded* is
  the normal state, so it is neutral, never a status colour (`:39`).

### 6.3 Every state designed (`DESIGN-SYSTEM.md:90`)

| State | What shows |
|---|---|
| Loading | the areas' skeleton rows under the one global heartbeat; never a spinner per row |
| Empty | the area is absent; the header actions remain |
| Sparse, dense | one row per entry; past 50 rows the list is virtualised |
| Hub unavailable | the last list read, marked *not current*, verbs disabled with the reason, as `unavailable` is treated elsewhere |
| Refused at write | inline on the row, cause and way out, naming who and never when (no relative time as a primary display, phase 4 `task-19.md:9-11`): *"lane" is already settled for this project, by <savedBy>.* **Use theirs** · **Depart on this branch** (`word_taken`, rules 1 and 2 of §2.4); *"track" is a word this project avoids, for "lane".* **Use "lane"** · **Change** (`word_taken`, rule 3); *This entry changed since you opened it.* **Show the new one** (`revision_conflict`) |
| Model off | no suggestions; the header actions work without a model |

Keyboard: the rows join the lens's traversal, `j`/`k` to move and `a` to take the suggested answer,
as the Desk's rows do (`AGENTIC-UX.md:147`). Words and their definitions are sans; only keys and ids
are mono.

## 7. The cost

Hours are the plans' own convention, an engineer's estimate before review; **no fix round is
included**, and on both ledgers most tasks have needed one (task 09's is open now, 11a §1). Test
counts are 11a §6's text scan, not a compile: a test marked *runs* must pass unedited, which is
unverified until it is run. `plan:` is phase 4's plan,
`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md`; `HP:` the Hub's,
`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub.md`; `T:` the Hub plan's parts,
`…/2026-09-23-phase-h-hub-parts/task-` (11b §2).

### 7.1 What does not change, which is most of the saving

- **The brief content model**: `discover/content.go` and every stored revision's digest; the inline
  `goldenJSON`, the 76 `discover` tests and the 37 elsewhere that build content (11a §6 A). No
  `ContentVersion` 2, no widened v1: the sceptic's shelf condition (06 §7) is met by not touching it.
- **The brief's key**, and every one of the 64 plan files that quote `brief:bf-` (06 §2.2): no
  fragment, no re-anchoring of phases W or T (06 §2.3).
- **The key grammar** (§2.1): no form is added, so `provenance/key.go`, `hubapi.ParseKey` and
  their grammar tests do not change, and neither does `/v1/refs`.
- **`discover.Document`** and `Resolve`, with the 12 `inherit_test.go` tests: the sets are
  arguments of a new render function, never fields of what `Resolve` produces
  (`discover/inherit.go:49-51`), and `Render(doc)` keeps its signature (`render.go:76`).
- **Phases 3, 5 and 7**, built or planned (§5.4), and phase 2's session record and `contextRef`.
  Phase 2's create and restart handlers **do** change (§5.2, the sceptic's I1): the next table.

### 7.2 Built code, tests and goldens that change

| Built file (phase 4 tree unless marked) | Task that built it | What changes | Tests and goldens |
|---|---|---|---|
| `provenance/ref.go` | 06 | `glossaryDigest`, `decisionsDigest`, the third time-varying part | `ref-golden.json` (102 lines) regenerated; `TestRefGoldenJSON`, `TestReadingAndHeadJSON` edited; `TestRefHasNoDenominator` must still pass; 36 more *run* |
| `claudemd/render.go` | 05, approved | new marker, the two fixed lines gone, two sections with their *not here* lines, the budget, the route line, in a new `RenderInherited(doc, terms, decisions)`; `Render(doc)` calls it with two empty sets; `Inspect`'s marker parse | all 3 goldens regenerated; the 9 marker-line and 9 wording tests edited; the 13 reserved-prefix tests *run* (the prefix stays); the 33 re-derive tests compile unedited and *run*; 3 new goldens |
| `contextwrite/materialise.go`, `inspect.go` | 09, **in fix** | three files in order, `/.panacea/discover/` excluded and confined, both files removed on a start with no brief key; `Inspect`'s **classification**, set digests before text (§5.3), not only its marker parse | of 60, the write-set and exclusion tests edited (about 8, a guess until run); new tests for the order, the confine, and a moved set read as `superseded`, never `edited` |
| `api/materialiser.go` | 10, approved | `MaterialiseResult` gains the two sets; `MaterialiseRequest` does not change | 2-3 of `materialise_test.go`'s 14 gain the assertion |
| `api/sessions.go`, phase 2's handler | phase 2; its `Materialise` call is 10's | create and restart append the inheritance entry after the record write (`:1037`, `:1817`); a failed append marks the session exited (§5.2) | new: an abort before the record leaves no entry; a failed append starts no harness |
| `discoverhub/read.go` | 07, approved | `Ref`'s heads path reads the two lists to the last page; the preview (`:738`) renders through `RenderInherited` | 11 `Ref`-path tests *run* against a fake that serves lists; `TestResolveBatchesInTwoCallsAtMost` (`read_test.go:96-134`) is **edited**: its bound gains the lists' pages |
| Hub: `hubapi`, the wire types | Hub 01-03, approved | new `discoverids.go`; two envelopes; the objects; `FoldWord` and the field-rule function (§2.4) | the ten-check untouched; a new §18 prefix test; 4 new golden vectors beside `envelope_test.go`'s 1-4 |
| Hub: the contract-reading tests | Hub 01-03 | the body test gains rows anchored in §18; the §4.2 code test (`keys_test.go:530-551`) also reads §18's codes; `EraseResponse`'s `settled` is a `pendingAmendment` until 44b (`types_test.go:885`) | each extended in the commit of the code it checks (§7.4, A20) |
| Hub: `hubclient` interfaces, the fake, `conformance/feed.go` | Hub 04-05; 05 not approved | `Terms`, `Decisions`; two event kinds | new conformance rows, one `add("term.revised", …)` each |

`hubapi` and `hubclient` reach the phase 4 tree by merge, as H0 did (11a §3), so each Hub change
above lands once in the Hub worktree and is merged, never edited twice. `0001_init.sql` is **not**
edited (11b §3.3).

### 7.3 New tasks

Each plan gets its new tasks as **a new section at its end**, with new part files, and its task
table is not touched: phase 4's table is `plan:390-414` of 600 lines, the Hub's `HP:396-447` of 683,
and a row inserted in either would move every cited line below it (rule of this program: new
material goes at the end of a heavily cited file).

**Phase 4**, ids continuing from 23:

| # | Task | Depends on | Hours | Must come before |
|---|---|---|---|---|
| 24 | Prototype: the §6 states added to `prototype/phase-4-discover.html` (suggested, recorded, taken up, departure, *not here*, project changed, refused, model off, *reaches sessions once complete*, *from archived branches*). The owner looks before 31's part is approved | — | 2.5 | 31 |
| 25 | `discover`: `Term`, `Decision`, the branch overlay with its later-project-word shadow and its *not here* departures (§2.3), the set digest; pure. No copy of §2.2's rules: they are the Hub's one function (§2.4) | 06 | 2.5 | 26-30 |
| 26 | `provenance`: the `Ref`'s two digests; `ref-golden.json`. No key form (§2.1), so no Hub task in front of it | 06, 25 | 1.5 | 12 |
| 27 | `discoverhub`: the two list reads to the last page inside the budget (§5.1), the two batch writes, the two set digests for `Ref`, the preview through `RenderInherited`, and `TestResolveBatchesInTwoCallsAtMost`'s new bound | 07, 08, 25, 28, Hub 46 | 6 | 29, 30, 32 |
| 28 | `claudemd`: `RenderInherited`, the new block and the two files' renderer, the *not here* and *Gave up:* lines; three goldens regenerated, three added | 05, 25 | 4.5 | 27, 29 |
| 29 | `contextwrite`: the three files in order, `/.panacea/discover/` excluded and confined (a not-ignored answer refused `write_failed`, §5.3), both files removed wherever the block is stripped (§5.1), and `Inspect`'s set-digest classification (§5.3); `api`: `MaterialiseResult`'s two sets, and the create and restart handlers' append after the stamp (§5.2) | 09's fix round closed, 10, 27, 28 | 5.5 | 32, 14 |
| 30 | `discover` + `discoverhub`: the suggestion round, its local store and its five deaths (§4); and the one-decision draft behind *Record a decision* (§3.2), under the round's model budget and outcomes | 04, 25, 27 | 3.5 | 32 |
| 31 | Desktop: the lens's two areas (§6), with *not here*, *reaches sessions once complete* and *From archived branches*. UI: stays unmerged until the owner has looked (`OD-1`) | 16, 18, 24 looked at, 32 | 6.5 | 23 |
| 32 | `discoverapi`: the relay routes for the two reads and two writes, the round's two routes, the per-session inheritance route beside task 12's `materialisations`, and that route's fresh render through `RenderInherited` with the two lists (§5.3, task 12's amendment) | 11, 12, 27, 29, 30 | 3.5 | 14, 15 |

Phase 4 new: **36 hours, nine tasks.** Hours are priced against the plan's own tasks for the same
work: 27 against 07 and 08, 6 h for one kind (11a §1); 31 against 18 and 19, 6 h (`plan:409-410`).

**Hub**, ids continuing from 44b:

| # | Task | Depends on | Hours | Must come before |
|---|---|---|---|---|
| 45 | `hubapi`: `discoverids.go` with `te` and `dr` and a test holding it to §18's prefix table; the two envelopes and four golden vectors; the wire objects and their body-test rows, anchored in §18; `FoldWord`; §2.2's field-rule function, the reserved `panacea:discover:` sequence among its rules | 44a | 3.5 | 46, 47 |
| 46 | `hubclient`: `Terms` and `Decisions`; the fake, holding every observable rule of §2.4 (the ops, the two passes, the item references, the word check's three rules); the conformance and feed rows: **S0g**, phase 4 codes its terms and decisions against the fake | 05, 45 | 5 | phase 4's 27 |
| 47 | `sql/0002_terms_decisions.sql`: four tables, the guards with `set_aside`, the RLS and grant twins, the folded-word index, §2.6's composite keys, `hub.erase_term` and `hub.erase_decision`, a database test for each | 07b's fix round closed, 45 | 4 | 48a |
| 48a | The server's writes: the two batch routes with every op of §2.4, the item references and the two passes, the write-time checks and the word check's three rules, `word_taken` with the §4.2 code test reading §18, and the text-free reply with its rebuild (§2.6) | 14, 16b, 19, 47 | 6 | 48b |
| 48b | The server's reads and erasures: the two lists with `archivedAt`; the two erase routes through 47's functions; `settled` on the brief's erase reply, a `pendingAmendment` until 44b; task 27's scan test with a term and a decision; the four events and the feed test reading §18; its own matrix rows (`rowsTask48` in `allRows`, the 16b pattern): **S4g (server side)** | 20, 27, 48a | 4 | phase 4's terms and decisions on the real Hub |

Hub new: **22.5 hours, five tasks.** Hub 48 is split because, priced against the brief's own server
side, tasks 18, 19 and 20 at 9 h for one kind (`HP:421-423`), its 4.5 h was half of what two kinds
cost; 46 against task 04's fake, 3 h for part 1 alone (`HP:405`). No new Hub task stands in front
of the brief's own S4: S4 for briefs still opens after 06-20 (`HP:388-394`), and S4g (g for
glossary) follows it.

### 7.4 Amended tasks, all unbuilt, and what each gains

| Plan | Task | What changes | Hours added |
|---|---|---|---|
| Phase 4 | 12, `materialisations` | **changes in behaviour**: its fresh render needs the two lists, and a moved set reads `superseded` (§5.3). Written by 32, which already sits between 12 and 14, so 12's part is built as approved and reworked once | 0 here; in 32 |
| Phase 4 | 14, wiring | wires the round store and the handlers' inheritance append (§5.2) | +0.5 |
| Phase 4 | 15, client and wire types | the `Ref`'s two digests mirrored field for field; the new objects and the struct; `word_taken`, `key_unresolvable`, `brief_archived` | +1 |
| Phase 4 | 16, slices and the loader | `discoverSlice` gains the two lists; a `term.*` or `decision.*` event invalidates every brief's `Ref` in its project (§5.2); no new key source (§5.4) | +1 |
| Phase 4 | 17, `ContextChip` | the *words moved since this session started* state (§5.2) | +0.5 |
| Phase 4 | 19, the states that come apart | a departure's *project changed*; Undo writes `restore`, or after a create `retire` or `withdraw` (§2.4) | +1 |
| Phase 4 | 22, contract amendments | the Go, UI and provenance contract lines for all of the above | +1 |
| Phase 4 | 23, end to end | a session inherits words: block, files and record re-derive byte for byte; a branch whose brief is open gets the project's set | +1 |
| Hub | 44a, the contract made true | **A20 is appended, never folded**: a new §18, *Amendment — OD-36, terms and decisions*, after §17 at the contract's end, holding the two prefixes, the six routes and their bodies, the codes, the events, the tables and functions, and §7.5's one-field rule read as brief content only. No line above it moves, so none of the contract's 517 inbound citations breaks (the sceptic's count), and no contract-reading test changes: each reads its section by number (`keys_test.go:371`), and none reads §18 until 45, 48a and 48b extend it beside their code | +2 |
| Hub | 27, audit and erasure | its "Nothing else changes" sentence (`T:27.md:40-41`) names both kinds; its scan test's term and decision, and `settled`, are 48b's | +0.25 |
| Hub | 34, the digest re-check | covers the two revision kinds | +0.5 |
| Hub | 36, the poller | `Subscribe` prefixes gain `"term."` and `"decision."` | +0.25 |
| Hub | 40, `hubFeed` | handlers for the two prefixes | +0.5 |
| Hub | 43, the end-to-end proof | one word and one decision through the real Hub | +0.5 |
| Hub | 44b, the contract made true to the build | writes `settled` into §7.12 and deletes its `pendingAmendment`; checks §18 against what 45-48b built | +0.5 |

Phase 4 amended: **6 hours, seven tasks**, and 12's change inside 32. Hub amended: **4.5 hours,
seven tasks.** Round 2's amendments to Hub 14 and 20 are gone: 14's feed test takes the four kinds
in 48b, beside the code that emits them, and 20 has nothing to answer under the struct. Unchanged,
and checked: phase 4's 11, 13, 18, 20 and 21 (11 keeps its scope because 32 carries the new
routes; 21 stays one-shot, 06 §1.4); the Hub's 08, 14, 16b, 20 and 22.

### 7.5 Where it slots, and the total

- **Phase 4's critical path** is Hub H0 → 07 → 08 → 09 → 11 → 12 → 14 → 23 (`plan:430`). 25, 26 and
  28 can start now, beside 09's fix round: 26 waits on no Hub task, because under the struct no
  form is added (§2.1). 27 and 30 start once Hub 46 exists, and 46 waits on 45, which waits on 44a,
  still unrun and out of order (the Hub worktree's `.superpowers/sdd/hub/progress.md:518-521`). 29
  starts once 09's round closes and 27 is done. The one task that joins the path is **32, between
  12 and 14**, because 14 must wire the final Materialiser rather than wire it twice: **about +3.5
  hours on the critical path**, if 25-30 and Hub 44a-46 run beside it as their dependencies allow.
  If they do not, the path waits on them, and 44a is the first it would wait on.
- **The Hub's order**: 44a with A20 → 45 → 46 (S0g) → 47 after 07b's round closes → 48a after 19 →
  48b after 20 and 27 (S4g). Hub 45 and 46 stand in front of phase 4's 27, and 44a stands in
  front of everything, as it already does. **Q2 is asked before 44a runs** (§8.2).
- **The UI lane**: 24's prototype first, the owner's look, then 31 after 18; unmerged until the
  owner has seen it running (`OD-1`).

| | New | Amended | Total |
|---|---|---|---|
| Phase 4 | 36 h, nine tasks | 6 h, seven tasks | **42 h** |
| Hub | 22.5 h, five tasks | 4.5 h, seven tasks | **27 h** |
| `CONTEXT.md` | two entries, *Term* and *Project decision* (§8.2, Q6), when the plans land | — | 0.5 h |
| **All** | | | **about 69.5 hours before fix rounds** |

Against the plans: phase 4's whole plan is 57 hours (11a §1), so this is about 74% more on phase
4; the Hub's is 133.5 (`HP:449-460`), about 20% more. With one fix round a task, on the ledgers'
record, expect roughly 90 hours: that multiplier is a judgement, not a measurement. Round 2
priced about 54 hours, and this is about 16 more. The four tasks the sceptic repriced against the
plans' own work (Hub 46 and 48, phase 4's 27 and 31) carry 12.5 of them, what the rules it found
missing (I1, I2, I5, I6, I7) add elsewhere is about 6.5, and the struct takes about 3 back: the
key forms in Hub 45 and phase 4's 26, the resolver rows in Hub 20 and 48, and task 16's key
source (§8.2, Q2). None of it is scope beyond OD-36.

## 8. What it does not do, and the open owner questions

### 8.1 What it does not do

- **The interview**: no new question, no change to the five or to the follow-up, no provability seed.
- **The brief**: no field in its content, no fragment on its key, no revision minted by a word.
- **Design and Deploy**: no approval or release pins a word or a decision (Q3 below).
- **No record without a person**: the assistant suggests, a person records; no Authority rule
  records on its own (Q4).
- **No shared round state**: rounds and dismissals stay on one machine (Q5).
- **No MCP tool now**: `get_terms` and `get_decisions` wait for phase 5's channel (§5.3).
- **No session-sourced decisions yet**: *Accept and record* waits for the Desk's row, which no
  plan builds (§3.2).
- **No words from an imported repository**: task 21 stays as it is.
- **No scope wider than a project**: no team or org glossary; a branch may add, depart and promote.
- **Not ours**: it replaces neither this repository's `CONTEXT.md` nor the owner-decision register.
- **No count, badge or progress** anywhere, and no queue item of its own (§6.2).

### 8.2 Open owner questions, each with the default taken

| # | Question | Default taken, and what the other answer costs |
|---|---|---|
| Q1 | Who owns the key grammar: phase 4's `provenance` (`HC:116`, `PC:104`, `key.go:1-2`) or the Hub contract (OD-36, `owner-decisions.md:409`; `10:54`)? (11b Q1) | **No answer needed to build under Q2's default**, which adds no form, so neither parser changes. Under Q2's other answer both parsers change together, the Hub's first (45 before 26), and the design is the same whoever owns the grammar |
| Q2 | OD-36 says the key grammar "does not move". Are terms and decisions named by a struct, or by two new key forms? **Asked before 44a, Hub 45 or phase 4's 26 starts**: the other answer, taken after they ship, rebuilds them | **Taken: the struct** `{kind, id, revision}` (§2.1), the plain reading of OD-36's row (`owner-decisions.md:409`), of its note, which names the two parsers as the thing protected (`10:54-55`), and of 06's shelf clause (`06:348-351`). The owner's record also points the other way: the note sketches `term:tm-7f2a@2 -> brief:bf-9c1d@4` (`10:14`) and `PC:104-107` made `kind:` extensible. Key forms `term:te-…@n` and `decision:dr-…@n` would add §3.2 forms through §18, both parsers (Hub 45, phase 4's 26), `/v1/refs` rows (Hub 20, 48b) and task 16's key source: about 3 h more |
| Q3 | Should a release pin the decisions in force at Ship? | **No.** OD-36's *What moves* names the Materialiser, `Ref` and the lens only, and a session's inheritance is already provable through Develop. Yes costs one pin member in Hub 23 and lines in phase 7's 08 and 17: about 3 h |
| Q4 | May Authority pre-authorise the assistant to record **words** (never decisions) on a branch without asking? | **No**: every record is a person's gesture, `author` `person` or `model_accepted`. Yes costs a new `author` value, one Authority action class and its lens state: about 3 h |
| Q5 | Should rounds and dismissals be shared through the Hub? | **No**, local only; the cost is that a dismissed suggestion may be offered once more on another machine. Yes is a mutable Hub kind for process state, which is what §4 exists to avoid |
| Q6 | The product's words for `CONTEXT.md`. `CONTEXT.md:303` already calls our own register "one decision record", and `CONTEXT.md:149` calls an approval row "a decision under Authority" | **Term** and **Project decision**, each with a *Not «…»* line: not «vocabulary» (the Work layer's team names, 11b §5.2), not «ADR», and not an approval row. The Work layer's planned Go type `Term` (`task-138.md:111`) is another package and is left alone |
| Q7 | Should a session started without the brief (`none:declined`) still inherit the project's words and decisions? | **No** (§5.1): declining the brief declines everything Discover writes, so the label *started without the brief* stays true. Yes needs a block with no brief in its marker, a new `Inspect` state and a golden: about 3 h |
| Q8 | When a branch lands, should the product ask about the branch's own decisions and words at the land itself? | **No** (§6.2): once a branch ends, its in-force entries wait in the project's lens under *From archived branches*, with **Promote** beside each, and nothing asks. Yes puts that list in phase 3's land step, a phase 3 change that OD-36's *What moves* does not name: about 2 h and its tests. Promoting them all at the land without asking is not offered: it would change every other branch's set with no person's gesture (§8.1) |
| Q9 | Should a branch's own entries reach its sessions before the branch's brief has a complete revision? | **No** (§5.1): the set follows the stamp, so a project key gives the project's set, and the lens says when a branch's entries will reach sessions. Yes serves the set digests per `(contextRef, branch)` beside the `Ref` rather than in it, which reads OD-36's "`Ref` carries their digests" loosely: about 3 h across phase 4's 26, 27, 29 and 17 |

### 8.3 What this design did not verify

- No code was compiled or run; test counts are 11a §6's text scan, and every *runs* in §7.2 is a
  prediction. The hours are estimates before review.
- The upstream `LICENSE` file is unread (§1, section 6).
- Whether the harness reads `.claude/CLAUDE.md` is still phase 4's open item (task 23,
  `materialise.go:45-48`); the route line names worktree-relative paths, so it reads the same from
  either file, if either is read.
- Which of the Hub's schema tests enumerate tables is unchecked: `schema_integration_test.go` is
  uncommitted-modified in the Hub worktree today (11b §2.2).
- The round's limits (three decisions, five words, fourteen days) and the inline budgets (4 and 6
  KiB) are judgements, not measurements: the prototype (task 24) is where the owner tests them.
- Round 2's four late rules (the later-project-word shadow, the cross-scope word check, the erase
  routes, *no brief key, no inheritance*) and this revision's rules are priced inside the repriced
  tasks of §7.3; that they fit there is a judgement from the plans' ratios, not a measurement.
- **Read from source, not run** (Revision 1): that `contractSection` finds a section by its number
  alone, so §18 is invisible to every test until one is extended to read it (`keys_test.go:371`);
  that the `pendingAmendment` ratchet covers a type ahead of its contract text and not the reverse
  (`types_test.go:881-885`); and the create handler's order (`api/sessions.go:963-1103`).
- Hub 14's `TestInFeedIsTheContractTable` is known from its plan part only, so that it reads the
  event table by section, and so ignores §18 until 48b, is unverified.
- `hubapi.FoldWord`'s choice of NFC and simple case folding is untried against the lens's search
  and sort; the prototype (task 24) is where a mismatch would show.

## Closing

**Complete as a design on 2026-09-29; reviewed and revised once.** It answers the sixteen findings of
`06-sceptic.md` by number and its sections 6 and 7 besides (§1). It meets OD-36 as the register
states it (`owner-decisions.md:409`): the brief content model does not move, the brief's key does
not change by a byte, and terms and decisions are their own Hub records that reference a brief
revision. Where doing that properly changes built phase 4 and Hub code, it changes it and prices
it by file (§7.2), as the owner's note on OD-36 allows, rather than designing around it.

What it still owes, and to whom:

1. **Its sceptic, round 2**: done, `12-sceptic-r2.md` (Needs fixes), answered by Revision 1 below.
   A re-read of Revision 1's sections, C1 and C2 first, is owed before any plan is edited: they
   change §2.1, §5.1 and §5.2, which the rest of the design leans on.
2. **The owner**: §0 and §8.2, about ten minutes, and **Q2 before 44a runs**. Every other question
   has a default, so an unanswered one is built as its default.
3. **The plans**: the new tasks as a new section at the end of each plan (§7.3) and the amended
   unbuilt tasks (§7.4), with 44a's A20 first. Those edits belong to the phase 4 and Hub streams in
   their own worktrees, not to this file.
4. **The owner's eyes on task 24's prototype** before task 31's part is approved (`OD-1`).

Nothing in this file has been compiled, run or shown to the owner (§8.3).
**Next: item 1**, a re-read of Revision 1 against `12-sceptic-r2.md`, C1 and C2 first.

## Revision 1

**Status: complete, 2026-09-29.** Answers `12-sceptic-r2.md` at `b1cc3895`: both Critical and all
seven Important findings at their root, and the eight Minor ones, which all fall in sections this
revision already touched. `§N:L` is section N at line L of this file after the revision; the
sceptic's own `11:N` citations are this file at `bd9b6299`. No other file was edited.

| # | Finding | Answered at |
|---|---|---|
| C1 | Q2's default adds two key forms to the grammar OD-36 froze, while §0 said no question blocks the build | **Root, §2.1:206-238**: one revision is named by a `{kind, id, revision}` struct, and no key form is added. Then §0:21-29 and :58-62 (Q2 asked before 44a, Hub 45 or phase 4's 26; "none of which blocks the build" deleted), §8.2:988-989 (Q1, and Q2 flipped with its cost), §2.4:376-382 (`/v1/refs` unchanged), §5.2:652-657 (set items and entries are structs), §5.4:733-734 and :750 (no new key source), §6.1:778-781, §7.3:881 and :896 (26 and Hub 45 shrink), §1:103-109 and :188-196 (06's shelf clause met) |
| C2 | Under a project key, §5.1's branch overlay and §5.2's one shared `Ref` cannot both hold | **Root, §5.1:578-588**, one rule: the set follows the stamp. A project key gives the project's set only, and a branch's own entries reach sessions from its brief's first complete revision. Then §5.2:643-651 (why one `contextRef` is one `Ref`), §6.2:790-793 (the lens says so on each entry), §0:43-45, and §8.2:996 (Q9: the rule not taken, and its cost) |
| I1 | The inheritance record is written before its session exists, by a call with no session id | **§5.2:663-672**: `MaterialiseResult` returns the two sets, and the create and restart handlers append the entry after the record write (`api/sessions.go:1037`, `:1817`); a failed append marks the session exited. §7.1:847-848 strikes "phase 2 entirely"; §7.2:858 and task 29 (§7.3:884) price it |
| I2 | The drift check calls a block `edited` whenever the project's words move | **§5.3:700-712**: set digests before text, a mismatch `superseded`. §7.2:856 lists `Inspect`'s classification; §7.4:912 takes task 12 out of *Unchanged*, its change written by 32 (§7.3:887) |
| I3 | A20 in a documents-only 44a turns the Hub's contract-reading tests red; phase W adds kinds its own way | **§7.4:920**: A20 is an appended §18, never folded, so no test reads it until 45, 48a and 48b extend each one beside their code, and none of the contract's inbound citations moves. §2.1:227-233 keeps `te` and `dr` out of `keys.go`, as phase W keeps its prefixes in `workids.go`, so the ten-check keeps ten and no count needs agreeing with W0. §2.4:366-371 names the Hub the one owner of §2.2's rules and reads `HC:742-745` as brief content only. Also §7.2:861, §1:110-115 |
| I4 | §7.2 misses five changes to built code; the honest total is near 66-70 h, not 54 | **§7.1:835-849 and §7.2:850-866**: the handler, the preview, the call-count test, `Inspect`, the Hub's contract-reading tests, and `Document` kept by a new `RenderInherited`. **§7.3:879-906**: 27, 31, Hub 46 and 48 repriced against `plan:409-410`, `HP:405` and `:421-423`, Hub 48 split in two. **§7.5:950-965**: about 69.5 h before fix rounds, about 90 with one round a task, and where the 16 hours came from. Also §0:17-19 |
| I5 | Erasure keeps text in stored replies and in derived terms | **§2.6:426-441**: both batch routes store a text-free reply and rebuild on replay; task 27's scan test gains a term and a decision; a brief's erasure reply lists what settled in it, `settled`. Priced in Hub 48a and 48b (§7.3:899-900) |
| I6 | Four transitions have no op, including 06 §5.1's *not here* for a decision | **§2.4:313-339**: a departure is born `in_force`, `retired` or `set_aside` (the `depart` row, :328); Undo writes `restore`, which skips rule 2 only; promote's Undo is one batch of restores, applied exits first. Also §2.2:261, §2.3:279-289, §3.2:474-478, §1:162-171 |
| I7 | A branch's own decisions leave the thread when the branch lands | **§6.2:795-802**: the project's lens lists archived branches' in-force entries with **Promote**, and no prompt. §2.3:291-294 keeps promote and retire open on an archived brief; §2.4:379-380 serves `archivedAt`; §8.2:995 records Q8 with its default and cost |
