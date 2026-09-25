### Task 01: W0 and the documents: the Hub-contract proposal; PROGRAM, FUNCTIONAL-SPEC, AGENTIC-UX's four amendments, 02's corrections

**Slice:** W0. **Size:** 2 focused hours (an estimate, not a measurement). **dependsOn:** none.
**Seams:** defines **SW.0** (the W0 proposal: the `issue:` key case and the `c-` example), consumed
by tasks 02 and 18. **Stream:** none; documents only, on `feat/work-layer`.

W0 is the part of Phase W that happens *now*, inside the current wave (05 §5.2). It is two small
changes to the Hub contract and five documents that must say what the owner already decided. No
code is written here: task 02 builds the grammar case in `hubapi`, and task 18 builds it in phase
4's `provenance` parser.

**Why a proposal and not an edit.** The Hub contract
(`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md`, cited
`hub-contract:N`) is under live edit on `feat/hub`. Rule H1 says the work layer never edits a Hub
file in flight. 05 §5.2 makes W0's two items **the one exception** to "new Hub tasks after task
43", and even that exception goes to the `feat/hub` owner as a written proposal. So this task
writes the proposal file on `feat/work-layer` and changes nothing on `feat/hub`.

**Why the two items are safe to send now.** Item 1 adds a grammar case that nobody mints until W1.
Item 2 fixes an example that is wrong today: `criterionId` values are `c-` ids, and `done:2` is a
leftover from an older criterion scheme. Both are harmless whatever the owner decides about
Phase W (05 §5.2).

**Anchors, read on 2026-09-25.** `feat/hub` at `9f2fa31` (the scope read it at `48351f8`; every
anchor below was re-read at `9f2fa31` and still holds): §3.1 at `hub-contract:91`, the prefix
table at `:103-109`, §3.2 at `:114`, its grammar block at `:119-122`, its rules at `:124-129`,
`done:2` at `:1065` and `:1079`, *"Nothing else is routed"* at `:609`, `type Hub interface` at
`:1329`; in `control-plane/internal/hub/hubapi/`, `keys.go:47-58` (the `KeyKind` block and the
two regexes), `keys.go:69` (`ParseBriefKey`), `keys.go:96-105` (`ParseKey`), and `done:2` in
`types_test.go:173` and `:531`. On `feat/work-layer`, read at `c5841bf`: `docs/PROGRAM.md:54-65`
(the *Revised phase order* table), `docs/FUNCTIONAL-SPEC.md:5-16` (the object model),
`docs/AGENTIC-UX.md:85` (§5), `:236`, `:237`, `:238` and `:245` (anti-patterns 1, 2, 3 and 10),
and in `02-linear-design-language.md` C3 at `:209` (faces at `:222`), C7 at `:256`
(`arrivals.hold` at `:267`), C8 at `:270` (the on-disk replica at `:274`).

**Files** (all on `feat/work-layer`; paths relative to the repository root):

| Path | Change |
|---|---|
| `docs/superpowers/planning/2026-09-25-linear-parity/w0-hub-contract-proposal.md` | **new**: the proposal to the `feat/hub` owner, items 1 and 2, and the later ask |
| `docs/PROGRAM.md` ⚑ | one **W** row after row **9** of the phase table (`:65`); a short *Phase W* section |
| `docs/FUNCTIONAL-SPEC.md` ⚑ | two rows, **Feature** and **Issue**, appended to the object model (`:16`) |
| `docs/AGENTIC-UX.md` ⚑ | Q13's four one-line amendments, each marked `(OD-27, Q13 n)` |
| `docs/superpowers/planning/2026-09-25-linear-parity/02-linear-design-language.md` ⚑ | the three corrections of 05 §5.5 row *02*, each a one-line *was:* note |

Every edit is an **insertion**. No existing sentence is rewritten; where a line is corrected, the
old text stays, struck through as a *was:* note beside the new one, so a reader of the diff sees
exactly what moved. ⚑ files are shared; this task touches them first and alone (the skeleton's
file map), so there is no ordering conflict inside Phase W.

---

#### The proposal file, complete

`docs/superpowers/planning/2026-09-25-linear-parity/w0-hub-contract-proposal.md` is written with
exactly this text. The implementer copies it byte for byte. Anchor lines are re-read at the sync
base on the day it is sent and corrected in place if they have moved; the grammar and the values
do not change.

`````markdown
# W0: two proposed edits to the Hub contract, from Phase W

**To:** the owner of `feat/hub` (`docs/superpowers/contracts/2026-09-23-hub-contract.md`).
**From:** Phase W, the work layer (`docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`).
**Status:** proposed. Nothing on `feat/hub` has been changed by Phase W. Read at `feat/hub` `9f2fa31`.

Both items are harmless whatever the owner decides about Phase W (05-design §5.2). Item 1 adds a
grammar case nobody mints until W1. Item 2 fixes an example that is wrong today.

## Item 1: the `issue` case in §3.2's key grammar

**Where:** §3.2 *Key grammar — normative for the Hub* (`hub-contract:114`), the fenced block at
`:119-122`, and the rules under it (`:124-129`).

**Add** one line to the block, after `design key`:

```
issue key    ^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$             e.g. issue:is-3f2a91c07b1e4d2a@3
```

**Add** one rule after the brief-revision rule (`:124-125`):

- An issue revision number is the Hub's, assigned by the issue-revision write, from 1, gapless per
  issue. Revision 0 means "the issue exists and has no revision yet" and never appears in a key.
  `EP-142` (a project key and a number) is a **display alias and never a key**: a team admin can
  change the project key, so no parser for the alias exists or may be written.

**Code it implies** (Phase W task 02 builds it on `feat/work-layer`, additive only, and it reaches
`feat/hub` through the orchestrator's merge, rule H1):

- `KeyIssue KeyKind = "issue"` appended to the const block at `hubapi/keys.go:49-52`;
- `issueKeyRe` beside `briefKeyRe` and `designKeyRe` at `keys.go:56-58`;
- `IssueKey{IssueID, Revision}`, `String()` and `ParseIssueKey`, shaped like `BriefKey` (`:62-77`);
- one `case issueKeyRe.MatchString(s): return KeyIssue, nil` in `ParseKey` (`keys.go:96-105`).

Phase 4's `provenance` parser must accept at least these forms (§3.2's own rule); Phase W task 18
adds the case there.

## Item 2: the stale `criterionId` example in §7.10

**Where:** §7.10 *Evidence*, the request at `hub-contract:1065` and the 201 body at `:1079`.
Criterion ids are `c-` ids; `done:2` is a leftover from an older scheme.

**Before** (both sites): `"criterionId": "done:2"`
**After** (both sites): `"criterionId": "c-91e2a0b3c4d5e6f7"`

The same stale value sits in two code examples, which should change with the contract's so the
golden rows keep matching it: `control-plane/internal/hub/hubapi/types_test.go:173` (the `7.10`
`EvidenceRecord` row) and `:531`. The rule at `:1072` (`criterionId` 1-128 characters) is
unchanged, so no code changes and no test changes meaning.

## What the later Phase W Hub tasks will ask (for information; nothing to do now)

So the owner sees the whole ask once. Each arrives as a **new Hub task after Hub task 43**, never an
edit to a Hub task in flight, and only after `feat/hub` is sync-merged into `feat/work-layer`.

1. **Twelve id prefixes**, minted with `ids.Wide` (§3.1, `hub-contract:93-100`): `is-` issue,
   `ic-` issue comment, `ft-` feature, `ml-` milestone, `cy-` cycle, `in-` initiative, `rq-`
   request, `lb-` label, `tp-` template, `vw-` saved view, `lp-` loop, `df-` draft; `dc-` (free
   document) is reserved for a later row. Checked disjoint from §3.1's ten (`org- u- tm- iv- pj-
   bf- ap- ln- rl- ev-`), the control plane's `s- p- b- w- m-`, and Phase T task 01's twelve
   (`dl- th- hm- ms- nt- es- tk- dv- ag- ot- ls- cf-`). Phase W reuses Phase T's `nt-` for status
   updates rather than minting its own.
   They live in a new `hubapi/workids.go`, **not** in §3.1's table or `keys.go`'s const block,
   because `TestPrefixesAndReasonsAreTheContracts` (`hubapi/keys_test.go:737-775`) holds `keys.go`
   and §3.1 to exactly ten entries each. Phase W's own test checks the twelve against the rest.
2. **A `Work` interface embedded in `type Hub interface`** (`hub-contract:1329`), one line, with
   one sub-interface per slice (W1 `Work`, then `WorkPlan`, `WorkAgents`, `WorkRoadmap`,
   `WorkIntake`).
3. **New rows appended to §7.0's route table after its last row, 45** (`hub-contract:607`,
   before *"Nothing else is routed"* at `:609`), each with its §5.3 authorisation-matrix rows in
   the same task, and exactly one metadata-only event per changed record (§8, `:1133`).
4. **Migrations** taking the next free number at sync time, append-only.
5. **One authorisation guard** Phase W needs from day one: no sync or integration principal may
   write an issue's fact states (`started`, `in_review`, `landed`) or `done`.
`````

---

#### The four document edits, exact text

**1. `docs/PROGRAM.md`.** Append one row after row **9** (`PROGRAM.md:65`), in the table's own
three-cell shape and bold-letter style (row **H** at `:59` is the model):

```markdown
| **W** | **The work layer — issues on the provenance thread, Linear parity-plus** (added 2026-09-25, `OD-25`–`OD-28`; lettered so no number moves) | No — built alongside team mode; W1 demos in local mode first (`OD-20`) |
```

Insert one section after *Phase 9* (`:124-125`), before `## Cross-cutting constraints` (`:127`):

```markdown
### Phase W — The work layer
Added 2026-09-25 (`OD-25` to `OD-28`). Issues are Hub records on the provenance thread: each one names the brief criteria it serves, so brief → criterion → design → issue → session → land → evidence → release reads as one chain. Built **alongside team mode** on its own streams, never on its critical path (`OD-25`); **everything, W0 to W5** (`OD-26`): W0 the key grammar and these documents, W1 the first slice a team can adopt (import, triage, the issue view, Start from a row, done only when proven), W2 planning, W3 agents on the work with team mode's Orchestrators, W4 roadmap, release and reporting, W5 intake and the ecosystem. Progress is proof per criterion and plain counts, never a percentage, bar, ETA or "N of M" (`OD-27`). Every id, path and time is Commit Mono; the sans is Segoe UI Variable (`OD-28`). Its plan is `docs/superpowers/plans/2026-09-25-phase-w-work-layer.md`.
```

**2. `docs/FUNCTIONAL-SPEC.md`.** Append two rows after **Environment** (`FUNCTIONAL-SPEC.md:16`):

```markdown
| **Feature** | many per project; its spec is its branch brief (`ft-`) | Yes — it is what Linear calls a project |
| **Issue** | many per project, optionally one feature; a mutable head (`is-`) and immutable revisions (`issue:is-…@r`) whose `serves` names brief criteria | Yes — shown as `EP-142`, a display alias that is never a key |
```

**3. `docs/AGENTIC-UX.md`.** Four one-line insertions (Q13, accepted under `OD-27`). Each goes on
the line **after** the text it amends, indented to continue the list item it follows, and ends
with its decision id. No existing line changes.

| # | After | Inserted line |
|---|---|---|
| 1 | anti-pattern 10 (`:245`) | `   *Amended (OD-27, Q13 1):* a plain group size for a set people committed to (a cycle, a triage queue) is allowed; a completion prediction is not.` |
| 2 | anti-pattern 1 (`:236`) | `   *Amended (OD-27, Q13 2):* "… a nameable transcript span, **or the Hub record and revision it shows**", since a person's issue has no span.` |
| 3 | §5's lead paragraph (`:87`) | `*Amended (OD-27, Q13 3):* the Desk's RUNNING shows three-line cards only for agents that changed since you looked, and one folded line for the steady.` |
| 4 | anti-pattern 2 (`:237`) | `   *Amended (OD-27, Q13 4):* a projection that names its rule and source span (*"next decision ~14:22 · rule: migration gate"*) is not an ETA.` |

Insert in the order 1, 4, 2, 3 (bottom of the file first), so each anchor line is still where
this table says when it is used. Amendment 3 sits under §5's heading (`:85`), after its first
paragraph (`:87`), because anti-pattern 3 (`:238`) stays the rule at rest.

**4. `02-linear-design-language.md`.** The three corrections of 05 §5.5 row *02*
(`05-design.md:1310`). The live claim is struck inline and one line is inserted after it.

Inline strikes: at `:274` wrap `Tauri stores it on disk;` as `~~Tauri stores it on disk;~~`; at
`:267` wrap `*"2 new · R to show"*` as `~~*"2 new · R to show"*~~`; at `:222` wrap
`**Owner decision needed:** the mono licence` in `~~ … ~~`. Then insert one line after each:

```markdown
  - *Corrected (Q5, ADR-H1:467-474):* the replica is held **in memory only**; nothing from the Hub is written to disk, so an erase cannot be outlived by a laptop copy.
  - *Corrected (05 §5.5 row 02):* the line reads *"2 new · N to show"*; `N` applies every held change at once; **`moves.hold`** sits beside it: a row whose sort place changed stays put, marked, until `N`.
  - *Decided (OD-28):* **Commit Mono** (OFL), bundled as files with its licence, for every id, path and time; the sans is **Segoe UI Variable**, the Windows system face, never bundled. D1 item 1 (`:360`) is closed by this line.
```

(the first after `:274`, the second after `:267`, the third after `:222`; bottom first).
`ADR-H1` is `.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md`, lines
467-474 read at `9f2fa31`: *"neither the renderer nor the control plane persists Hub data to
disk"*. These change no token and no code; tasks 26-27 build what they describe.

---

#### Tests first: the acceptance greps, each with the mutant it kills

A documents task has no unit tests, so its tests are greps, written down **before** the edits and
run once to watch them fail. The exact commands are the script under *Acceptance*; this table
names what each one proves.

| # | Checks | Expected after | Before | Mutant it kills |
|---|---|---|---|---|
| T1 | proposal lines starting `issue key` | `1` | no file | a proposal that names the case but gives no grammar line |
| T2 | the exact regex `^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$` in the proposal | `1` | no file | a loosened grammar (`[0-9]+`, which admits `@0` and `@01`) |
| T3 | proposal lines holding `done:2` | `2`, both inside item 2 | no file | a proposal that shows the fix but never the old value |
| T4 | proposal lines naming `:1065` and naming `:1079` | at least 1 each | no file | a proposal that forgets one of the two sites |
| T5 | `OD-27` in `AGENTIC-UX.md` | `4` | `0` | an amendment added without its decision id |
| T6 | `Q13 1` … `Q13 4` in `AGENTIC-UX.md` | `4`, each once | `0` | one amendment dropped |
| T7 | `Tauri stores` in `02` outside a `~~` strike | nothing | `:274` | a correction that leaves the on-disk claim live |
| T8 | `R to show` in `02` outside a `~~` strike | nothing | `:267` | `arrivals.hold` still bound to `R` |
| T9 | `moves.hold` in `02` | `1` | `0` | the `N` fix without `moves.hold` |
| T10 | `OD-28` in `02` | `1` | `0` | the faces question left open |
| T11 | the **W** row in `PROGRAM.md` | `1` | `0` | the phase missing from the order table |
| T12 | `### Phase W` in `PROGRAM.md` | `1` | `0` | a row with no section behind it |
| T13 | **Feature** and **Issue** rows in `FUNCTIONAL-SPEC.md` | `2` | `0` | one object added, the other forgotten |
| T14 | deletions in `git diff --numstat` against the start commit | `0` for `AGENTIC-UX.md`, `PROGRAM.md`, `FUNCTIONAL-SPEC.md`; `3` for `02` | — | an amendment that rewrites a line instead of inserting one |

T14's `3` for `02` is the three inline strikes (each rewrites one line by adding `~~`); nothing
else in `02` moves.
