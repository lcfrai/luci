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

(to be written)

## 3. How a decision earns its record, without ceremony

(to be written)

## 4. The interview's rounds are process state

(to be written)

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
