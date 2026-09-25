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
