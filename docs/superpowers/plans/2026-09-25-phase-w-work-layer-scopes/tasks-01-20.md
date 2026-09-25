# Phase W scopes: tasks 01 to 20 (W0, W1's Hub, the phase 4 deltas, the test kit)

One paragraph per task. A part writer reads its paragraph and the skeleton's row
(`2026-09-25-phase-w-work-layer.md`, *Task list*); where the two disagree, **the row's id,
dependsOn and seams win** and this paragraph is corrected. `§` means a section of
`docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2); `L-NN` is a row of
`01-linear-feature-inventory.md`, as 05 §1 prices it. A row listed as *closed* here is closed only
when the task's acceptance commands pass; a row closed jointly names the other tasks.

Paths are repository-relative. Hub server paths (`server/`, `store/`, `migrate/` under
`control-plane/internal/hub/`) are planned by the Hub plan and **UNVERIFIED until they land on
`feat/hub`**; sibling-plan paths are **UNVERIFIED until merge** (rules H1, H2). Every Go test runs
as `go test ./control-plane/... -race -count=1`, Hub integration rows with `-tags integration`
against `control-plane/cmd/hub/compose.yaml`; every desktop test as `npm --prefix desktop test`.

"Mutant" names the one wrong implementation a test exists to kill. Hub anchors below were read in
the `feat/hub` worktree at `48351f8` (`docs/superpowers/contracts/2026-09-23-hub-contract.md`,
cited `hub-contract:N`; the Hub plan `docs/superpowers/plans/2026-09-23-phase-h-hub.md`, cited
`H-plan:N`); a part re-reads them at its own sync base (rule H1).

---

**01 — W0 and the documents.** A documents-only task, W0 (05 §5.2, §5.5's *Hub*, *Docs*, *02* and
*`AGENTIC-UX.md`* rows), that defines seam **SW.0** and has no dependency. It writes one new file,
`docs/superpowers/planning/2026-09-25-linear-parity/w0-hub-contract-proposal.md`: a proposed edit
to the `feat/hub` owner (never a direct change, because the contract is under live edit) carrying
W0's two items. **Item 1** adds the `issue` case to hub-contract §3.2's grammar (`### 3.2` is at
hub-contract:114; the block the Hub mints and accepts sits under it), written out in full as
`issue key ^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$  e.g. issue:is-3f2a91c07b1e4d2a@3`, with
the rule that the revision number is the Hub's, gapless per issue from 1, 0 never in a key, and
that `EP-142` is a display alias and **never** a key (05 §3.6); it names the matching
`KeyIssue KeyKind = "issue"` and `issueKeyRe` it would add beside `briefKeyRe`/`designKeyRe` in
`hubapi/keys.go:47-58` and the `ParseKey` case at `keys.go:96-105` (task 02 builds them). **Item
2** replaces the stale `"criterionId": "done:2"` at hub-contract:1065 and :1079 with a `c-` id,
using one worked value (for example `c-91e2a0b3c4d5e6f7`), and states it is harmless whatever the
owner rules on Phase W. The proposal also lists, as *what the later WH tasks will ask*, the id
prefixes of 05 §3.6 (`is- ic- ft- ml- cy- in- rq- lb- tp- vw- lp- df-`, checked disjoint from
hub-contract §3.1's table and TM-05's `ag- tk- ls- cf- th- ms- es- nt- dl-`), the `Work` embed in
`type Hub interface` (hub-contract:1329) and new route rows after row 43 (hub-contract:605-609),
so the owner sees the whole ask once. The task then edits four documents: `docs/PROGRAM.md` gains
a **W** row in the *Revised phase order* table (`PROGRAM.md:54-65`, needs Azure: no; built
alongside team mode, `OD-25`) and a short Phase W section naming W0-W5 and the owner decisions
`OD-25` to `OD-28`; `docs/FUNCTIONAL-SPEC.md`'s object model (`FUNCTIONAL-SPEC.md:5-16`) gains
**Feature** (`ft-`, the spec is its branch brief) and **Issue** (`is-`, head and revision,
`serves`), each with its cardinality and "User sees the word? Yes"; `docs/AGENTIC-UX.md` gets
**Q13's four one-line amendments** (05 §6.1 row Q13, accepted under `OD-27`): (1) at anti-pattern
10 (`AGENTIC-UX.md:245`) plain group sizes for sets people committed to are allowed; (2)
anti-pattern 1 (`:236`) reads *"… a nameable transcript span, or the Hub record and revision it
shows"*; (3) §5 (`:85`) allows three-line cards for agents that changed since you looked and one
folded line for the steady; (4) anti-pattern 2 (`:237`) says a projection that names its rule and
source span is not an ETA; and `02-linear-design-language.md` gets **its three corrections** (05
§5.5 row *02*): C8's replica "on disk" becomes in memory (ADR-H1:467-474, Q5); C7's
`arrivals.hold` moves from `R` to `N`, `N` applies every held change, and `moves.hold` is added
beside it (D M10); C3's faces question is closed by `OD-28` (Commit Mono, OFL, bundled; Segoe UI
Variable, the system face, never bundled). Each amendment is a one-line insertion marked with its
decision id, never a rewrite. **Tests** are acceptance greps, each with the mutant it kills:
`grep -c "^issue key" …/w0-hub-contract-proposal.md` prints `1` (kills a proposal that names the
case but gives no grammar line); `grep -n "done:2"` finds the string only in the *before* half of
item 2 (kills a proposal that forgets one of the two sites); `grep -c "OD-27" docs/AGENTIC-UX.md`
prints at least `4` (kills an amendment added without its decision id); `grep -n "Tauri stores"
…/02-linear-design-language.md` returns nothing outside a struck *was:* note (kills a correction
that leaves the on-disk claim live); `grep -n "^| \*\*W\*\*" docs/PROGRAM.md` prints one row. It
**does not claim** that the Hub owner accepted the proposal, that any code parses `issue:` (task 02
and task 18 build it), or that the three `02` corrections change any token. Parity rows: it closes
none alone; it is the precondition for L-3 and L-34 (tasks 02, 08, 09).


