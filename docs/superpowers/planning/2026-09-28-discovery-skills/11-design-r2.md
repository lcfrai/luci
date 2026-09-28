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

(to be written)

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

(to be written)

## 6. The Discover lens

(to be written)

## 7. The cost

(to be written)

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
