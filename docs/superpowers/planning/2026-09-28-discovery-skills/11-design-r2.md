# 11 — Design round 2: the project glossary and the project decision record, beside the brief

**Status: DRAFT, being written a section at a time.** Round 1 (`05-design.md`) failed its sceptic
(`06-sceptic.md`). The owner then decided `OD-36` (`10-decision-terms-beside-the-brief.md`): terms
and decisions are their own Hub records that reference a brief revision. This round designs that.

Inputs, in the order read: `11a-facts-phase4.md`, `11b-facts-hub.md`, `06-sceptic.md`,
`10-decision-terms-beside-the-brief.md`, `00-summary.md`, `01-define-skills.md`,
`docs/AGENTIC-UX.md`, `docs/DESIGN-SYSTEM.md`.

## 0. The design in one screen

(to be written)

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
  batch route (§2.4). What does move in built code is new tasks (25-29 in phase 4, 45-46 in the
  Hub), never a re-plan of an approved one.
- **2.2 The key grammar is the Hub's** (`06:129-151`). *Changed.* The brief's form gains no
  fragment and does not change by a byte (§2.1), so none of the 64 plan files that quote
  `brief:bf-` moves. The two new forms **are** a Hub contract amendment and a change to both
  parsers, and are priced as that: 44a's A20 for the contract, Hub 45 for `hubapi`, phase 4's 26
  for `provenance`, the Hub's first and merged. This round opened the contract (11b reads it at
  `cafe1303`), which round 1 did not. Who owns the grammar stays an open owner question (§8); the
  design is the same under either answer, because both parsers change together.
- **2.3 Re-anchoring the frozen plans was unpriced** (`06:153-170`). *No longer applies to phases W
  and T*: nothing in this design lands in either. In the two plans it does touch, new tasks go in a
  new section at each plan's end with new part files (§7.3), so neither task table moves. The
  contract's in-place edits are confined to its four closed tables (prefixes, grammar, routes,
  events), exactly where 44a is already editing for A1-A19, and ride on that pass (§7.4).

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
     carries its base, the brief key and both vocabulary digests. A round whose base moved is
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
  remaining words and says where each is defined; and every stamp (the marker, `Ref.vocabulary`,
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
  departure is its own record, `overrides` naming the project entry's pinned key, written through
  the Hub's batch route rather than `discover.Apply`, so no single content object has to hold both.
  What the branch sees is phase 4's overlay of two lists (§2.3).
- **5.2 Retiring an inherited term had no op** (`06:284-290`). *Changed.* `depart` with `state:
  retired` sets a project word aside on one branch; `retire` retires it for the project;
  `reinstate` undoes either. §2.4 has an op for every transition.
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
  build, so if you need to redo some work, it is what it is"* (`owner-decisions.md:409`).

## 2. The two records

Two new kinds, each **its own Hub record**: a head plus immutable numbered revisions, the shape the
brief already has (11b §1.4, `HC:742-815`). Neither is a field of brief content, and writing either
never mints a brief revision. `HC` is the Hub contract,
`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md` at `cafe1303`.

### 2.1 Ids and keys: two new forms beside the two that exist

| | Term (one glossary entry) | Decision (one decision record) |
|---|---|---|
| Hub id | `te-` + 16 lowercase hex, `ids.Wide("te")` | `dr-` + 16 lowercase hex, `ids.Wide("dr")` |
| Key | `term:te-<16 hex>@<n>` | `decision:dr-<16 hex>@<n>` |
| Pattern | `^term:(te-[0-9a-f]{16})@([1-9][0-9]{0,8})$` | `^decision:(dr-[0-9a-f]{16})@([1-9][0-9]{0,8})$` |

- **The brief's key does not change by a byte**, nor does `design:` (`HC:119-122`) or phase 4's
  local `none:declined` (`key.go:23-29`). The grammar grows by two forms, which is what its `kind:`
  prefix exists for (`PC:104-107`); phase W already plans a fourth, `issue:is-…@r`
  (`.claude/worktrees/phase-4/docs/superpowers/planning/2026-09-25-linear-parity/04-internal-fit.md:45`).
- `te-` and `dr-` are in neither the Hub's prefix table (`hubapi/keys.go:16-27`) nor the control
  plane's `s-`, `p-`, `b-`, `w-`, `m-` (`HC:111`), and no `.md` or `.go` file in any worktree holds
  a `te-` or `dr-` id (grep, 2026-09-29; the only other planned prefix it met is phase W's `is-`).
- The illustration `term:tm-7f2a@2` in `10-decision-terms-beside-the-brief.md:14` is corrected:
  `tm-` is the team prefix (`HC:107`), and an id is 16 hex, not 4 (11b §1.2).
- Revisions are the Hub's, from 1, gapless per record, and 0 never appears in a key: the brief's
  rule (`HC:124-125`) applied to both. A key names one immutable revision, so what a session was
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
| `state` | a term: `in_force`, `retired`. A decision: `in_force`, `superseded`, `withdrawn` |
| `overrides` | branch scope only: the key, revision pinned, of the project entry this branch departs from; else `null` |
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
- **Depart** from a project entry: a branch record with `overrides: term:te-…@n` that either
  defines the word differently (`in_force`) or sets it aside on this branch (`retired`). Pinning
  `@n` means a later project revision never silently re-points it; the lens says the departure was
  made against an older definition (§6).
- **Promote** a branch entry to the project: a new revision of the **same** record with
  `scope: project`, so its id and history survive. When it was a departure, the same batch retires
  or supersedes the project entry it overrode, with `replacedBy` naming the promoted record.

What a branch sees is computed by phase 4, never by the Hub (the Hub stores and resolves, 11b §1.4):
the project's in-force entries, minus those a branch record overrides, plus the branch's own.

### 2.4 Ops: one for every transition

Every write goes through **one batch route per kind**, `POST /v1/projects/{projectId}/term-revisions`
and `…/decision-revisions`, shaped exactly like `brief-revisions` (`HC:798-815`): 1-16 items, one
transaction, all or nothing; each item names its `baseRevision`, `0` for a new record; the Hub
mints the id and numbers the revision `baseRevision + 1`, never merges, and answers 409
`revision_conflict` on a stale base. A retry carries the same `Idempotency-Key` (`HC:50-53`, `:814`).

| Op | Transition | The item |
|---|---|---|
| `create` | nothing → `@1`, `in_force` | `baseRevision: 0` and the fields |
| `revise` | `@n` → `@n+1`, same state | a better definition, or a rename: `word` changes, the id and its history stay |
| `retire` (term) | `in_force` → `retired` | optional `replacedBy`: two words turned out to be one thing |
| `supersede` (decision) | `in_force` → `superseded` | `replacedBy` required: the decision that replaced it, in this batch or earlier |
| `withdraw` (decision) | `in_force` → `withdrawn` | it should never have been recorded; nothing replaced it |
| `reinstate` | `retired` or `withdrawn` → `in_force` | the word check below runs again |
| `depart` (branch) | a project `@n` → a new branch record | a `create` with `scope: branch` and `overrides: <that key>` |
| `promote` (branch) | branch → project | a `revise` with `scope: project`; the batch also retires or supersedes the entry it overrode |
| `erase` | any revision → no content | an owner function, not a route (§2.6) |

**Write-time checks, in order**, each its own code: the grammar and §2.2's field rules (400);
`settledIn`, `overrides` and `replacedBy` each name a non-erased record in **this** project, and
`overrides` a project-scoped one (422 `key_unresolvable`, the design approval's code, `HC:887-897`);
then **no other in-force term in the same scope has the same case-folded word** (409 `word_taken`),
enforced by a unique index inside the write's transaction. So a collision is refused when it is
written and never discovered when it is resolved. A departure is not a collision: it is another
scope, and it names what it departs from.

Two reads, both new rows in §7.0's closed table (`HC:557-607`): `GET /v1/projects/{projectId}/terms`
and `…/decisions`, each the heads of one project, filterable to one branch brief, every revision's
fields as §2.2 lists them. The resolver `GET /v1/refs` (row 27, `HC:589`) accepts `term:` and
`decision:` keys among its 1-100 and answers `kind: "term"` or `"decision"`; a key outside the
grammar stays `invalid` (`HC:836`).

### 2.5 The reference to the brief revision it settled in

`settledIn` says **where** a word or a decision settled, not what it depends on: it stays in force
when the brief moves on, which is OD-36's lifecycle ground. Its three sources:

- **The interview** (§4). A round that changes the brief saves the brief revision first and the
  round's terms and decisions second, in their own batch, naming the revision just written; so a new
  brief's head 0 never needs naming. A round that settles only words names the current head. If the
  second batch fails, its proposals stay pending (§4) and are retried; each batch is all or nothing.
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
  current word, scope and state for the unique index. `settledIn` and `overrides` are composite
  foreign keys, `(org_id, brief_id, brief_revision)` as every brief reference is (11b §3.2), never a
  key stored as text. Every list in `0001` that names tables one by one (RLS, grants, the
  append-only array: 11b §3.4) gets its twin statement in `0002`.
- **Digests**: two new canonical envelopes, `{"v":1,"kind":"term-revision",…}` and
  `"decision-revision"`, members sorted as the brief's are (`discoverhub/digest.go:26-49`), digest
  `"sha256:"` + hex; each gets golden vectors beside the brief's (§1.7 of 11b).
- **Events**: `term.revised` and `decision.revised`
  `{"key","revision","digest","state","scope"}`, one per changed record, and `term.erased`,
  `decision.erased`. `data` never holds the text (`HC:1133-1134`).
- **Erasure**: `hub.erase_term` and `hub.erase_decision`, owner functions shaped like
  `hub.erase_brief` (`HC:1115-1127`): the text goes, the key, digest, number, `savedAt` and `savedBy`
  stay, and the key resolves `erased`. Erasing a brief changes no term, as it changes no approval:
  neither holds brief text.

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
writes a `withdraw` or `retire` revision, as the brief's own Undo writes a `restore` revision (phase
4 `task-19.md:7-9`, 11a §5): the record's history stays honest. When one round holds several
suggestions, **Record all** writes them in one batch, the Desk's *approve pattern*
(`AGENTIC-UX.md:109-110`). That is the answer to the sceptic's throughput point (§1, 3.2 item 3): a
round costs one gesture, not one review per row. No suggestion is ever raised as a modal
(`AGENTIC-UX.md:246`).

### 3.3 The discard rule: what is not recorded, and where it goes instead

| It fails | For example | Where it goes instead |
|---|---|---|
| Limb 1: cheap to undo | a table rather than a list on the settings page | it stays an ASSUMED row on the Desk, with its own verbs (`AGENTIC-UX.md:29`), and in the transcript |
| Limb 2: anyone would guess it | the API returns JSON | nowhere: the code says it |
| Limb 3: there was no fork | inference goes through the org's gateway (`OD-19` leaves no choice) | nowhere |
| It is a requirement, not a choice | it must work on mobile | the brief: a criterion or a constraint, through the interview's existing ops |
| It is *how*, not *what* | retry three times with a back-off | the code and its comments. A record that holds *how* becomes a second brief, and the thread then carries two descriptions (`05-design.md:250-254`, kept) |
| It is ours, not the project's | a choice about how PanaceaApp itself is built | our own owner-decision register, never a product record |

A **word** earns an entry when it is a noun specific to this project, defined by what it *is*, with
one word chosen and the others listed; general programming vocabulary stays out even in a project
full of it (`05-design.md:250-254`).

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
  a 20-second budget, the project and branch names and the answers holding a person's words, never
  file contents (`assist.go:15-20`, `:35-49`, `:182-185`). Its outcomes mirror the follow-up's six,
  none an error: with no model configured there are no rounds, and the lens keeps *Add a word* and
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
key and the branch's two vocabulary set digests, §5.2) and each suggestion with its fields and a
state, `pending` or `dismissed`.

It is **never** inside `discover.Content`, so no round mints a brief revision or moves a digest
(the sceptic's 4.3), and **never** in the Hub, so nothing about a round is shared, numbered or
audited. There is no `grill_round` op, no round counter and no `Grill{…}` struct anywhere.

### 4.3 How it dies

1. **Acted on.** A recorded suggestion leaves the round, because it is now a Hub record. A
   dismissed one stays, marked, so the next round does not offer it again.
2. **Its base moved.** When the brief head or either vocabulary digest differs from the round's
   base — a teammate revised something — the round is not shown as it was. Each pending suggestion
   is re-checked against the new base: dropped if its word is now in force or its decision now
   recorded, kept otherwise. A new model call happens only for a new complete revision. So a round
   is never applied against a stale base, and if two people race, the Hub's `baseRevision` and
   `word_taken` checks (§2.4) refuse the loser at write, with a row that says why.
3. **The brief is archived or erased.** The file is deleted.
4. **Fourteen days untouched.** The file is deleted; the next visit to a complete revision may make
   a fresh round. There is **no closed state and no stall detector**: an unanswered round closes
   nothing, so "skipped everything" and "not back yet" never need telling apart, and nothing ever
   needs reopening (the sceptic's 3.2 item 2).
5. **Another machine.** Round state is per machine. There the round is recomputed from the Hub's
   current brief and vocabulary. A suggestion is derived, unconfirmed data, so nothing a person
   said is lost; the one cost is that a suggestion dismissed on the first machine may be offered
   once more on the second (§8 records it as an owner question).

**A session's proposal** (§3.2, third source) is the Desk row's own state, wherever the Desk keeps
it, and dies with the row. It never reaches the Hub until a person records it.

## 5. Closing the thread

### 5.1 What the Materialiser writes

The Materialiser's one call (`materialise.go:196`) now writes **three files, in this order, all
before the harness process starts**: `.panacea/discover/decisions.md`, `.panacea/discover/terms.md`
(§5.3), and last the managed block. Last, so a block that names a set is never on disk before the
files that hold it: the ordering rule (`materialise.go:3-20`) applied inside one call.

What the branch sees is phase 4's overlay (§2.3), computed from the two list reads under the same
5-second Hub budget as the brief (`materialise.go:81-87`). If the Hub does not answer, the start is
refused `hub_unavailable` exactly as it is today: a session never starts on half its inheritance.

**The block**, in order (today's order is 11a §4):

1. The begin marker gains the two set digests and takes the two human lines the sceptic asked to
   move (06 §7): `<!-- panacea:discover:begin brief=<id> rev=<n> terms=sha256:<hex>
   decisions=sha256:<hex> ; written by Discover in Panacea App, and regenerated: change it there
   -->`. The two fixed lines (`render.go:81-82`) are deleted, so every session stops paying for them.
2. `# <title>`, and the brief's sections, unchanged.
3. `## Decisions already taken`, only when there is one: per decision, `- <decision> Because:
   <why>` and, for a branch record, an origin line in the style the brief's overrides already use
   (`_Set on this branch, departing from the project's: <old decision>_`, cf. `render.go:87-95`).
   `context`, `rejected` and `undo` are in the file, not the block.
4. `## Words used here`, only when there is one: per term, `- **<word>**: <is> Not <avoid>, <avoid>.`
   and the same origin line for a departure.
5. `---`, today's footer (`render.go:105-108`), then **always** one line naming the read route:
   `Every decision and every word, in full: .panacea/discover/decisions.md and
   .panacea/discover/terms.md.` It is there when everything fitted too, so an agent never has to
   know whether anything was left out to know where to look.
6. The end marker, unchanged.

**The inline budget.** Decisions get 4 KiB of the block and words 6 KiB, counted in rendered bytes,
**whole entries only**: a definition is never cut. Order is fixed so the bytes are stable: a
branch's own records first, then the project's; decisions oldest first, so adding one never
reorders the rest; words case-folded alphabetically, then by id. Past a budget the section ends
with `More in .panacea/discover/terms.md: <word>, <word>, …` — the remaining words by name, to 2
KiB more — so the agent knows *which* words exist and where each is defined.

Everything else in the block's rules holds: the neutraliser, LF, no timestamp (`render.go:5-8`,
`:47-54`), untracked and excluded (`materialise.go:169-184`), written only when the bytes change.

### 5.2 What `Ref` carries, and what the session keeps

- **`Ref` gains `vocabulary`: `{terms: "sha256:…", decisions: "sha256:…"}`**, the digests of the
  set now in force for the brief's own scope (a branch key: the branch's overlay; a project key: the
  project's). It is the third time-varying part of a `Ref`, beside `headRevision` and
  `latestCompleteRevision` (`ref.go:71-72`), so `Ref` stays one brief key resolved and cacheable
  on the same terms; two sessions with one `contextRef` still share one `Ref`.
- **A set digest** is `"sha256:"` + hex over a canonical envelope
  `{"v":1,"kind":"vocabulary-set","scope":…,"items":[{"key","digest"},…]}`, items sorted by key:
  every entry the branch sees, never only the ones that fitted inline.
- **The inheritance record**, new, phase 4's: one immutable entry per session start, written by the
  Materialiser beside the block, `{sessionId, start, contextRef, terms: {digest, keys},
  decisions: {digest, keys}}`. `MaterialiseRequest` gains `SessionID` for it (`api/materialiser.go:25`,
  phase 4's own seam from task 10); `MaterialiseResult` and the session record do not change, so
  phase 2 carries `contextRef` exactly as before.

**Why that is provable.** Every key in the record names an immutable revision that resolves for
ever (an erased one still resolves `erased`, keeping key and digest, §2.6); the set digest
recomputes from the keys; and the block and both files re-derive byte for byte from the keys and the
brief, because `Render` is pure. Develop compares the record's digests with `Ref.vocabulary`: equal
is *current*; different is *the project's words moved since this session started*, derived the way
the desktop already derives `superseded` from two numbers (`ref.go:75-76`).

### 5.3 The read route: every definition, past any budget

**The route is a file read, the one read every harness already has.** `.panacea/discover/terms.md`
and `.panacea/discover/decisions.md` hold every entry the branch sees, in full, each under its key
(`term:te-…@n`), with a first line naming the set digest the marker carries. `.panacea/` is already
the product's untracked projection directory: phase 5 puts its handoff bundle at
`.panacea/design/<canvasId>@<rev>/`, excluded through `.git/info/exclude` (`P5:231`, P5-G). The
Materialiser adds `/.panacea/discover/` to `info/exclude`, checks that git ignores it, and follows no
link below the worktree, the same three rules as the block (`materialise.go:169-184`, `confine.go`).

**So no stamp certifies a truncation** (the sceptic's 4.1). The marker, the files' first lines,
`Ref.vocabulary` and the inheritance record all carry the digest of the **whole** set, and the whole
set is on disk at the path the block names, before the harness starts. What the block holds inline
is a convenience with its own name, never the thing certified. The drift check covers the files
too: a missing file is `pending`, a hand-edited one `edited` (`inspect.go:18-47`); no new state.

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
| Phase 5 built: `review_mark`, `op.go:25`, `:59`, `:94`, `apply.go:1128-1140` | unchanged: a design is reviewed against a brief key, never a vocabulary |
| `P5:90`, the brief from `discoverhub.Service` | unchanged |
| `P5:183`, pin and mark | unchanged: a moved word or decision never supersedes an approval |
| `P5:216`, one key-source selector | unchanged |
| `P5:323-325`, G9: only `provenance` parses a key | holds for `term:` and `decision:` as written |
| `P5:534`, `design:` keys through `/v1/refs` | unchanged; the resolver also answers the two new kinds (§2.4) |
| `P5T:08`, the review digest | unchanged |
| `P5T:15`, MCP read tools | gains `get_terms` and `get_decisions` later, optional (§5.3) |
| `P5T:20`, `Superseded` in `designapi` | unchanged |
| `P5T:21`, approval | unchanged: the approval pins the brief key and digest only |
| `P5T:22`, handoff and `LinkSession` | unchanged; a design agent's session inherits the words through the Materialiser like any other |
| `P5T:27`, `selectDesignBriefKeys` | unchanged |
| `P5T:28`, the Design lens chips | unchanged |
| `P5T:30`, end to end | unchanged |
| Phase 7 built: the watcher, `hubside.go`, `pins.go` | unchanged: reads no brief key and still none |
| `P7:104`, Ship's body | unchanged: no vocabulary field (§8, owner question 3) |
| `P7:156` and `P7:841` | unchanged |
| `P7T:07`, `:08`, `:09`, `:10` | unchanged |
| `P7T:13`, `selectReleasePinKeys` | unchanged; the pin-keys-are-opaque test holds |
| `P7T:15`, `:17`, `:18`, pin cell, drawer, governance | unchanged |
| `provenance.Ref`, `ref.go:78-103` | **changes**: `vocabulary` (§5.2) |
| The one loader, phase 4 task 16 | **one entry**: `selectSessionInheritanceKeys`, the keys of an inheritance record a person has opened, per `task-16.md:165-167`'s rule that a later lens adds one key source and changes nothing else |

**Develop** is where the thread gains its hop: the Materialiser, the inheritance record and the
session view that compares it with `Ref.vocabulary`. Phase 3's merge and evidence code keys to a
criterion and never parses a key (11a §6), so it does not change either.

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
   that revision's history row (or *settled outside a brief*), and its key in the licensed mono
   (`term:te-…@2`, `DESIGN-SYSTEM.md:45`). A revised entry shows its newest words and a *History*
   disclosure; retired, superseded and withdrawn entries are behind it, never mixed in.

### 6.2 A branch's departures, and what the lens never shows

- **A departure** reads *On this branch: <definition>* with *Project: <definition>* beneath in muted
  text, the same shape as the brief's override note (`render.go:87-95`). When the project entry has
  moved past the pinned revision, the entry takes task 19's *project changed* treatment (11a §5)
  with **Keep this branch's** and **Use the project's**. No new visual state is invented.
- **Promote** is a verb on a branch entry, never a prompt; archiving a branch asks nothing.
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
| Refused at write | inline on the row, cause and way out: *A teammate settled "lane" a minute ago.* **Use theirs** · **Depart on this branch** (`word_taken`); *This entry changed since you opened it.* **Show the new one** (`revision_conflict`) |
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
- **Phases 3, 5 and 7**, built or planned (§5.4), and phase 2 entirely: `MaterialiseResult`, the
  session record and `contextRef` are untouched.

### 7.2 Built code, tests and goldens that change

| Built file (phase 4 tree unless marked) | Task that built it | What changes | Tests and goldens |
|---|---|---|---|
| `provenance/key.go` | 06, approved | two forms, two kinds; the "only copy" comment (`:47`) corrected | `key_test.go`'s 4 gain rows; the other 20 grammar tests *run* |
| `provenance/ref.go` | 06 | `vocabulary`, the third time-varying part | `ref-golden.json` (102 lines) regenerated; `TestRefGoldenJSON`, `TestReadingAndHeadJSON` edited; `TestRefHasNoDenominator` must still pass; 36 more *run* |
| `claudemd/render.go` | 05, approved | new marker, the two fixed lines gone, two sections, the budget, the route line; `Inspect`'s marker parse | all 3 goldens regenerated; the 9 marker-line and 9 wording tests edited; the 13 reserved-prefix tests *run* (the prefix stays); 33 re-derive and *run*; 3 new goldens |
| `contextwrite/materialise.go`, `inspect.go` | 09, **in fix** | three files in order, `/.panacea/discover/` excluded and confined, the inheritance record | of 60, the write-set and exclusion tests edited (about 8, a guess until run); new tests for the order, the confine and the record |
| `api/materialiser.go` | 10, approved | `MaterialiseRequest.SessionID` | 2-3 of `materialise_test.go`'s 14 gain the assertion |
| `discoverhub/read.go` | 07, approved | `Ref`'s heads path reads the two lists | 11 `Ref`-path tests *run* against a fake that now serves lists |
| Hub: `hubapi/keys.go`, `envelope.go`, the wire types | Hub 01-03, approved | prefixes `te`, `dr`; the forms; two envelopes; the objects | `keys_test.go`'s 4 gain rows; 4 new golden vectors beside `envelope_test.go`'s 1-4 |
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
| 24 | Prototype: the §6 states added to `prototype/phase-4-discover.html` (suggested, recorded, taken up, departure, project changed, refused, model off). The owner looks before 31's part is approved | — | 2 | 31 |
| 25 | `discover`: `Term`, `Decision`, §2.2's field rules (and the reserved `panacea:discover:` sequence, refused as answers refuse it), the branch overlay, the set digest; pure | 06 | 3 | 26-30 |
| 26 | `provenance`: the two key forms; `Ref.vocabulary`; `ref-golden.json` | 06, 25, Hub 45 | 2.5 | 12 |
| 27 | `discoverhub`: the two list reads, the two batch writes, the heads' vocabulary for `Ref` | 07, 08, 25, Hub 46 | 3.5 | 29, 30, 32 |
| 28 | `claudemd`: the new block and the two files' renderer; three goldens regenerated, three added | 05, 25 | 4 | 29 |
| 29 | `contextwrite`: the three files in order, `/.panacea/discover/` excluded and confined, the inheritance record; `api`: `SessionID` | 09's fix round closed, 10, 27, 28 | 4 | 14 |
| 30 | `discover` + `discoverhub`: the suggestion round, its local store and its five deaths (§4) | 04, 25, 27 | 3 | 32 |
| 31 | Desktop: the lens's two areas (§6). UI: stays unmerged until the owner has looked (`OD-1`) | 16, 18, 24 looked at, 32 | 4 | 23 |
| 32 | `discoverapi`: the relay routes for the two reads and two writes, the round's two routes, the per-session inheritance route beside task 12's `materialisations` | 11, 12, 27, 30 | 3 | 14, 15 |

Phase 4 new: **29 hours, nine tasks.**

**Hub**, ids continuing from 44b:

| # | Task | Depends on | Hours | Must come before |
|---|---|---|---|---|
| 45 | `hubapi`: prefixes `te`, `dr`; the two forms and `KeyKind`s; the two envelopes and four golden vectors; the wire objects | 44a | 3 | 46, 47; phase 4's 26 |
| 46 | `hubclient`: `Terms` and `Decisions`, the fake, the conformance and feed rows: **S0v**, phase 4 codes its vocabulary against the fake | 05, 45 | 3 | phase 4's 27 |
| 47 | `sql/0002_terms_decisions.sql`: four tables, the guards, the RLS and grant twins, the word index, the composite keys, `hub.erase_term` and `hub.erase_decision`, a database test for each | 07b's fix round closed, 45 | 3.5 | 48 |
| 48 | The server: the four routes, the resolver's two kinds, the four events, its own matrix rows (`rowsTask48` in `allRows`, the 16b pattern): **S4v (server side)** | 14, 16b, 20, 47 | 4 | phase 4's vocabulary on the real Hub |

Hub new: **13.5 hours, four tasks.** No new Hub task stands in front of the brief's own S4: S4
for briefs still opens after 06-20 (`HP:388-394`), and S4v follows it.

### 7.4 Amended tasks, all unbuilt, and what each gains

| Plan | Task | What changes | Hours added |
|---|---|---|---|
| Phase 4 | 14, wiring | wires the round store and the inheritance record | +0.5 |
| Phase 4 | 15, client and wire types | `Ref.vocabulary` mirrored field for field; the new objects; `word_taken`, `key_unresolvable` | +1 |
| Phase 4 | 16, slices and the loader | a `vocabularySlice`; one key source, `selectSessionInheritanceKeys` (§5.4) | +1 |
| Phase 4 | 17, `ContextChip` | the *words moved since this session started* state (§5.2) | +0.5 |
| Phase 4 | 19, the states that come apart | a departure's *project changed*; Undo writes `retire` or `withdraw` | +1 |
| Phase 4 | 22, contract amendments | the Go, UI and provenance contract lines for all of the above | +1 |
| Phase 4 | 23, end to end | a session inherits words: block, files and record re-derive byte for byte | +1 |
| Hub | 44a, the contract made true | **A20**: the two kinds folded into the contract in the same pass that already moves its lines for A1-A19, so they are re-anchored once | +1.5 |
| Hub | 14, the write transaction | `TestInFeedIsTheContractTable` gains the four event kinds | +0.25 |
| Hub | 20, `/v1/refs` brief half | the per-key table answers `term:` and `decision:` `missing` until 48, as `design:` does until 22 (`T:20.md:29-36`) | +0.25 |
| Hub | 27, audit and erasure | the two erase functions; its "Nothing else changes" sentence (`T:27.md:40-41`) names both kinds | +0.75 |
| Hub | 34, the digest re-check | covers the two revision kinds | +0.5 |
| Hub | 36, the poller | `Subscribe` prefixes gain `"term."` and `"decision."` | +0.25 |
| Hub | 40, `hubFeed` | handlers for the two prefixes | +0.5 |
| Hub | 43, the end-to-end proof | one word and one decision through the real Hub | +0.5 |
| Hub | 44b, the contract made true to the build | folds in what 45-48 built | +0.25 |

Phase 4 amended: **6 hours, seven tasks.** Hub amended: **4.75 hours, nine tasks.** Unchanged, and
checked: phase 4's 11, 12, 13, 18, 20 and 21 (11 and 12 keep their scope because 32 carries the new
routes; 21 stays one-shot, 06 §1.4); the Hub's 08, 16b and 22.

### 7.5 Where it slots, and the total

- **Phase 4's critical path** is Hub H0 → 07 → 08 → 09 → 11 → 12 → 14 → 23 (`plan:430`). 25, 26 and
  28 can start now, beside 09's fix round; 27 and 30 once Hub 46 exists; 29 once 09's round closes.
  The one task that joins the path is **32, between 12 and 14**, because 14 must wire the final
  Materialiser rather than wire it twice: **about +3 hours on the critical path**, if 25-30 and
  Hub 45-46 run beside it as their dependencies allow. If they do not, the path waits on them.
- **The Hub's order**: 44a with A20 runs first, as planned → 45 → 46 (S0v) → 47 after 07b's round
  closes → 48 after 20 (S4v). Hub 45 and 46 stand in front of phase 4's 26 and 27, and 44a
  stands in front of everything, as it already does.
- **The UI lane**: 24's prototype first, the owner's look, then 31 after 18; unmerged until the
  owner has seen it running (`OD-1`).

| | New | Amended | Total |
|---|---|---|---|
| Phase 4 | 29 h, nine tasks | 6 h, seven tasks | **35 h** |
| Hub | 13.5 h, four tasks | 4.75 h, nine tasks | **18.25 h** |
| `CONTEXT.md` | two entries, *Term* and *Decision record*, when the plans land | — | 0.5 h |
| **All** | | | **about 54 hours before fix rounds** |

Against the plans: phase 4's whole plan is 57 hours (11a §1), so this is about 60% more on phase
4; the Hub's is 133.5 (`HP:449-460`), about 14% more. With one fix round a task, on the ledgers'
record, expect roughly 70 hours: that multiplier is a judgement, not a measurement.

## 8. What it does not do, and the open owner questions

(to be written)

## Closing

(to be written)

## Working notes (removed when the design is complete)

- 11a §1-3 read: 9/23 p4 tasks approved (01-08, 10), 09 in fix (`6a354f69`, `ca737d92`), 11-23
  unbuilt incl. all desktop. `Content` is a wire format, v1; `Apply` pure; `ErrNoChange`.
  Keys: brief `bf-`16hex, design, `none:declined`; grammar copied in `provenance/key.go:48-52`
  and `hub/hubapi/keys.go:56-59`. `Ref` fields at `ref.go:71-103`; criterion state served beside.
- 11a §4-5 read: binary runs `materialiserUntilTask14` (writes nothing). Block = markers
  `brief=<id> rev=<n>`, title, 2 fixed lines, sections, footer; NO inline cap today; 3 goldens
  (branch 37 lines, project 36, neutralised 30). Inspect: 7 drift states, `SameText`. Merge/Strip.
  Interview: 5 closed questions, one follow-up (assist.go), lens tasks 18-21 unbuilt.
- 11a §6 read: content change = 76+37 tests + inline goldenJSON (AVOID under OD-36); key grammar
  28 tests + ref-golden.json; `Ref` 39 tests + ref-golden.json (102 lines); block: 3 goldens +
  29 named tests, 33 re-derive. Phase 3 `briefref.go` never parses a key.
- 11b §0-1.3 read: HC (hub worktree, 2385 lines) at `cafe1303`; additive = new route/field/event
  kind (`HC:32-35`); grammar two forms (`HC:119-122`); OWNER DISAGREEMENT: `HC:116`+`PC:104` say
  phase 4's `provenance` owns grammar; OD-36 + `10:54` say Hub contract. `tm-` = TEAM prefix
  (`HC:107`), so `term:tm-7f2a@2` in `10:14` collides; ids are prefix + 16 hex. Routes closed table
  §7.0 45 rows (`HC:557-607`).
- 11b §1.4-1.9 read: brief = head + immutable revisions; only write is batch `brief-revisions`
  1-16 items w/ `baseRevision` (409 `revision_conflict`); resolver `GET /v1/refs` 1-100 keys,
  result `{key,kind,resolution,revision,approval}`; brief-referencing kinds ap-/ln-/rl-/ev- carry
  `briefKey` (+digest), no text. Only 2 canonical envelopes (brief-revision, audit-event);
  event kind is a FORMAT not a list. hubclient split by consumer. NO procedure for new kind (§1.9
  table = the checklist to copy).
- 11b §2-3.2 read: Hub built = 01,02,03,06 approved; 04 approved outside stream; 05 not approved;
  07a/b fix round open; 44a UNRUN (runs first); no server/store/cmd/hub => only fake serves routes.
  S4 = phase 4 integrates after 06-20. Unbuilt tasks enumerating kinds: 44a,08,14,16b,20,22,26,27,
  34,36,40,43,44b. SQL 0001_init.sql 693 lines, 20 tables, 9 enums; brief refs are
  `(org_id,brief_id,brief_revision[,digest])` composite FKs; no table stores a brief key as text.
- 11b §3.3-4.1 read: migrations append-only (`ErrEdited`), only 0001 exists => new kinds need
  `0002_…sql` OR edit 0001 before any persistent DB (ledger says recreate). 5 name-lists in 0001:
  RLS array `:397-400`, INSERT grant `:421-424`, UPDATE/DELETE `:426-436`, comment `:437-440`,
  append-only array `:454-455`; no default privileges `:441`. Mutability shapes: append-only;
  guarded state machine; versioned-immutable = brief_revisions+briefs+3 guards. Canonical+digest
  CHECK only brief_revisions + audit_events. erase_brief brief-only. Phase 5 built thru 06
  (`review_mark` holds BriefKey ≤64 bytes); planned 08,15 (`get_brief` MCP!),20a,21,22,27,28,30.
- 11b §4.2-end read: phase 7 built code reads no brief key; planned P7 08,09,10,13,15,17,18.
  Both phases hand keys to p4's one loader (task 16 `keySources`, unbuilt). Every consumer
  never parses a key. No plan uses "glossary"; only ADR is ADR-H1; `CONTEXT.md:303` names OD-n
  as the decision record. Collisions: `Term` type x3 in Work layer; `tm-` team prefix.
  Open Q1: grammar owner (HC:116/PC:104 phase 4 vs OD-36/10:54 Hub contract).
- OD-36 register row `owner-decisions.md:409`: "What does not move: the brief content model, and
  the provenance key grammar" -- yet `10:14` illustrates a `term:` key. Reading taken: existing
  forms do not move; a new `kind:` form is the grammar's designed extension (`PC:104-107`). OWNER Q.
- CONTEXT.md 1-185: `contextRef` = ONE opaque string, phase 2 never interprets it (`:130-136`);
  "the ledger" = append-only link records (`:138-144`); Materialiser = ordering rule (`:175-181`).
  p4 task 12 has a per-branch `materialisations` route (worktree states), no stored record.
- CONTEXT.md 185-340: its entry shape (word · 1-2 sentences · *Fixed by* · *Not «X»* + why) is
  the model for a term record. Attention queue = only place for counts. Lens: no order/completion.
  "Adding a term" rule `:328-340`: file wins over entry; two files disagreeing = open question.
- 06 §1-2.1 read: 1.1 gap real; 1.2 criterion citation = phase W `serves[{briefKey,criterionId}]`
  (task-02.md:461) not `#c-`; 1.3 phase W plans a 4th key form `issue:is-…@r` (plan :69) =>
  precedent for new kinds; 1.4 rounds reuse overstated (task 21 is one-shot); 1.5 provability
  seed re-asks `done`; 2.1 built code priced as paper (10,940 lines, 98+ tests, 4 goldens).
- 06 §2.2-4.2 read: 2.2 grammar is the Hub's; `#c-` fragment = contract amendment, 64 plan files
  quote `brief:bf-`; 2.3 re-anchor cost in frozen plans W (190 parts, 2544 cites) and T (69,
  1344) unpriced; 3.1 40 questions/6 rounds + 3 checkboxes + disabled Save = wall; 3.2 (1) round
  vs async return, no expected-revision guard; (2) stall detector can't tell skipped vs away,
  no reopen; (3) model-proposed terms invisible until per-row human review; 4.1 cap 40 -> words
  only, no lookup, stamp certifies truncation; 4.2 loose CONTEXT.md forbidden for product but
  recommended for us w/o the distinguishing sentence.
- 06 §4.3-end read: 4.3 `Grill{Rounds}` in Content mints revisions; 5.1 branch dissent via
  cross-scope supersede refused by Apply (one content object); 5.2 retire inherited term: no op;
  5.3 id collision is resolve-time, not write-time. §6 IP: LICENSE unread; three-limb labels
  must not be verbatim on-screen copy. §7: delete the 2 human lines from the block (golden :4-5)
  -- fold into our golden regeneration. Shelf conditions: {briefKey, criterionId} shape.
- 01 §2-3 read (method, our paraphrase only): glossary entry = what a thing IS, 1-2 sentences,
  one word + rejected synonyms, project-specific only, no implementation detail; write inline the
  moment a term settles; decision gate = all three limbs or nothing; record = 1-3 sentences
  (context, decision, why); few decisions is the NORMAL outcome; files created lazily. Ours:
  Hub-held, projected into the worktree (OD-3); decisions NOT `docs/adr/`.
- 01 §1 read: rounds = frontier (decisions whose prerequisites settled), whole frontier per round,
  each with a recommended answer; facts are the agent's job, decisions the person's; stop on an
  empty frontier, then confirm. Label the round, never a total (G5 / anti-pattern 2).
- 00 §2 read: round 1 = fields in Content (Terms/Decisions, `t-`/`d-` ids, Gate bools enforced by
  Apply `ErrGateNotMet`), `#t-`/`#d-` fragments, cap + pointer for inherited project terms.
  Rejected by OD-36. Keep from it: terms/decisions never gate brief completeness; glossary means
  glossary; same caps/LF rules; move the two human lines out of the block.
- DESIGN-SYSTEM read: accent teal only for interactive/current; status colours + shape; state
  change flashes+decays ~600ms opacity/background; one heartbeat; tabular-nums; optimistic +
  Undo not confirm dialogs; hover never changes layout; every state designed; virtualise >50.
- AGENTIC-UX 18-87 read: intermittent presence, read cursor ("since you last looked"); ASSUMED is
  first-class (accept / reverse / rewind) -- an agent's unrecorded choice IS an ASSUMED row;
  runtime emits decision, reason, alternatives rejected, confidence; rule 4 one ranked queue,
  counts only there; rule 7 provenance to the transcript span; rule 10 no denominators; rule 12.
- AGENTIC-UX 87-252 read: the Desk's DECIDED WITHOUT YOU row (`:114-119`: chose X over Y,
  because, rejected, [Accept] [Do it the other way] [Rewind to here] span) IS the decision
  candidate surface => one gesture = a verb on that row. Answering never navigates; Undo window;
  anti-pattern 11 no modal from background; 6 approval rows need 4 things; 2 no N of M.
- Phase 5 read route facts: `get_brief` MCP tool (`P5T:15.md:10`, `:42-59`) resolves at call time;
  MCP reaches the sandbox by sandboxd 127.0.0.1:8731 -> `/mcp-channel` tunnel the control plane
  dials (`P5:141-151`), `--mcp-config` from `harness.claudeArgv`; unbuilt (p5 tasks 14,18,19).
  `.panacea/design/<canvasId>@<rev>/` handoff dir excluded via info/exclude (P5-G, `P5:231`) =>
  `.panacea/` is the established untracked projection dir. DECISION: read route = a projected
  file under `.panacea/discover/`, stamped like the block; MCP later, not required.
