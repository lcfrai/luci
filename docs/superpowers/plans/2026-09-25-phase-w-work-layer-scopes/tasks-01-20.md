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
none alone; it is the precondition for L-3 and L-34 (tasks 02, 08, 09). Note for the proposal:
`hubapi/types_test.go:173` and `:531` also carry `"criterionId":"done:2"`; item 2 names them so
the owner fixes the code examples with the contract's.

**02 — `hubapi`: Work id prefixes, the `issue:` key, `DisplayAlias`, the `issue-revision`
canonical kind and golden vectors.** W1, stream WH, seam **SW.1** (first half); depends on 01
(SW.0's grammar line) and Hub tasks H01-H03 (`ids.Wide`, `hubapi.Canonical`, the wire types), all
sync-merged from `feat/hub` first (rule H1). Files: `control-plane/internal/hub/hubapi/keys.go` ⚑
(Hub task 01's file, **additive only**), `keys_test.go` ⚑, and new `hubapi/workids.go`,
`workids_test.go`, `workcanonical.go`, `workcanonical_test.go` and
`hubapi/testdata/issue-revision/{v1,v2}.json`. In `keys.go` it appends `KeyIssue KeyKind =
"issue"` to the const block at `keys.go:49-52`, `issueKeyRe =
^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$` beside `briefKeyRe`/`designKeyRe` (`:56-58`), a
`type IssueKey struct { IssueID string; Revision int }` with `String()` and `ParseIssueKey(s)
(IssueKey, error)` shaped exactly like `BriefKey`/`ParseBriefKey` (`:62-77`), and one `case` in
`ParseKey` (`:96-105`). `workids.go` holds the prefix constants of 05 §3.6 (`PrefixIssue = "is"`,
`PrefixIssueComment = "ic"`, `"ft"`, `"ml"`, `"cy"`, `"in"`, `"rq"`, `"lb"`, `"tp"`, `"vw"`,
`"lp"`, `"df"`; `dc-` is reserved in a comment for L-120, *later*), each minted with
`ids.Wide(prefix)` and checked with the existing `ValidID(prefix, id)` (`keys.go:31-40`);
`ValidProjectKey(k string) bool` for `^[A-Z]{2,5}$` (04 §2.4: 2 to 5 capitals, set by a team
admin); and `DisplayAlias(projectKey string, number int) string` → `"EP-142"`, with a doc comment
that **no parser for it exists or may be written** (05 §3.6: an admin can change the key).
`workcanonical.go` gives `IssueRevisionEnvelope{V int "v"; Kind string "kind"; IssueID string
"issueId"; Revision int "revision"; ProjectID string "projectId"; Content IssueRevisionContent
"content"}` with `Kind` always `"issue-revision"`, and `IssueRevisionContent{Title AuthoredText
"title"; Description AuthoredText "description"; Serves []Serve "serves"; Implements []string
"implements"}` where `AuthoredText{Text string "text"; Author string "author"}` takes `"H"` or
`"A"` (05 §3.2, TM-05's `text·H`/`text·A`) and `Serve{BriefKey string "briefKey"; CriterionID
string "criterionId"}` (the types themselves are declared once, here, and reused by task 03);
`IssueRevisionDigest(env) (Digest, []byte, error)` calls `hubapi.Canonical` and `DigestOf`, the
one function the Hub writes with and `hubclient` verifies with (hub-contract §3.4: no TypeScript
implementation). `ValidateIssueRevisionContent` enforces title ≤ 200 and description ≤ 16 KiB (04
§2.4), `serves` and `implements` present as arrays (never `null`), each `briefKey` a valid brief key
and each `implements` entry a valid design key. The golden vectors follow hub-contract §3.5's
method (Python `json.dumps(sort_keys=True, separators=(",",":"), ensure_ascii=False)`, vector 1
also through `printf '%s' … | sha256sum`): **v1** an ASCII revision with one `serves` pair and an
empty `implements`; **v2** a title with `é` (U+00E9) and a description with a TAB and U+0001, an
`"A"` author and two `serves` pairs; each file holds the envelope, the exact canonical bytes and
the digest, and the part records the computed digests (never a placeholder). **Tests, each with
its mutant:** `ParseKey("issue:is-3f2a91c07b1e4d2a@3")` is `KeyIssue` and round-trips through
`ParseIssueKey`/`String` (kills a missing `case`); `@0`, `@01`, 15 hex, upper-case hex and
`issue:bf-…@1` are `ErrInvalidKey` (kills a loosened regex); `ParseKey(DisplayAlias("EP", 142))`
is `ErrInvalidKey` (kills an alias accepted as a key); a disjointness test lists hub-contract
§3.1's prefixes, the control plane's `s- p- b- w- m-`, TM-05's `ag- tk- ls- cf- th- ms- es- nt-
dl-` and the twelve new ones and fails on any repeat (kills `ms` reused for milestones);
`ValidProjectKey` rejects `E`, `EPICS1` and `ep` (kills a lower-case or 6-letter key);
both golden vectors reproduce byte for byte and digest for digest (kills a renamed or dropped
envelope field, a `kind` of `"issue"`, and `omitempty` on `serves`); a 201-character title and a
16 KiB + 1 description fail validation (kills an off-by-one limit). It **does not claim** that the
Hub stores or serves an issue (tasks 08-09), that `/v1/refs` resolves `issue:` (task 09), or that
phase 4's parser accepts it (task 18). Parity rows served: **L-3** (the alias; closed with 08 and
the desktop rows), **L-34** (immutable, digested revisions; closed with 09 and 46), **L-124**
(author class on every text; closed with 03, 12 and the issue view).


