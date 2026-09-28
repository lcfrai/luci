# 01 — The work layer (Phase W): what we can take from open source

**Date:** 2026-09-28. **Status:** research, nothing adopted. **Scope:** Phase W only — issues,
cycles, triage, views, keyboard grammar, the in-memory replica, search.
**Sources for the Phase W scope:** `docs/superpowers/planning/2026-09-25-linear-parity/00-summary.md`
and `05-design.md` §3, §4.3, §4.6, §5.3–§5.4 (the W1–W5 task tables this document maps onto).

## The one-line answer

**Take the desktop half, write the Hub half.** Of W1's 48.5 planned days, 33 are desktop and 11
are Hub. Almost all of the desktop cost is commodity UI machinery — a command palette, a
virtualised list, a Markdown editor, a keyboard dispatcher, an optimistic-write client cache — and
every one of those has a mature MIT or Apache-2.0 library. **The Hub half is the differentiator and
must stay ours**: the issue record that points at a brief criterion, `thread state`, and "done
means proven" (OD-27) exist in no tracker on earth, because no tracker has a provenance ledger to
point at.

There is **no adoptable permissively-licensed Linear-class tracker**. Every serious one is
copyleft or source-available (register in §6). The single exception worth real attention is
**Gitea's Go issue model** (MIT), which is vendorable in pieces, not adoptable whole.

Honest saving, if every *adopt* below is taken: **roughly 11–15 of W1's 33 desktop days**, and
**~6–9 days across W2–W4** (views, timeline, editor polish). It does not reduce the Hub work at
all, and it does not shorten W3 (agents on the work), which is entirely ours.

---

## 1. How the licences were checked

Every licence below was read from the actual `LICENSE` file at the actual repository on
**2026-09-28**, not from a badge or from memory. Where a project relicensed, both licences and the
version boundary are recorded. The fetch that established each verdict is given per row.

Reachability: `raw.githubusercontent.com`, `github.com` and `npmjs.com` were all reachable from
this KPMG machine through the Netskope proxy. Blocked fetches are listed in §7.

**Two traps found in this pass, both of them relicensings that a badge or a memory would have got
wrong:**

- **Planka** — was MIT through v1.x; **v2.0 (2025) relicensed to a bespoke "Fair Use License"**
  with a paid Pro/Enterprise tier. `github.com/plankanban/planka` now describes itself as
  *"fair-code"* and *"Source Available"*. **Forbidden** under the licence rule (field-of-use
  restriction, not an OSI licence). Read 2026-09-28.
- **Focalboard** — `LICENSE.txt` is a *multi-licence* document: the **compiled binary** is MIT, the
  **source code is AGPL-3.0** with a commercial alternative from Mattermost, and only the admin
  tools and config files are Apache-2.0. A badge reading "MIT" is true of the wrong artifact.
  **Forbidden**, and its source must not be read. Also archived/deprecated upstream. Read
  2026-09-28 at `raw.githubusercontent.com/mattermost/focalboard/main/LICENSE.txt`.

---

## 2. What is commodity and what is ours

This is the judgement the task asked for, and it decides every row in §3.

### Commodity — take it

| Phase W capability | Why it is commodity |
|---|---|
| Command palette (W1 row 13) | Prefix-scoped fuzzy list over verbs. Solved identically in a dozen apps. |
| Virtualised list / board / timeline (W1 row 11, W4 timeline) | Windowing and measurement. Nobody's differentiator. |
| Keyboard dispatcher (W1 row 13) | Chord matching, sequence (`G`→`I`) handling, scope guards, `event.code` handling. |
| Markdown editor, slash menu, mentions, tables (W1 row 12) | ProseMirror territory since 2017. |
| Drag-and-drop reorder on the board (W1 row 11, W2 rank) | Sensors, collision detection, keyboard DnD, live-region announcements. |
| Accessible picker/combobox primitives (assignee, state, label, cycle) | Radix / react-aria do the ARIA correctly and we will not. |
| Optimistic write + rollback + a reactive in-memory store (W1 row 14) | Client cache machinery. |
| Free-text and fuzzy matching inside a loaded replica | Scoring algorithms. |
| Filter *evaluation* once a filter is parsed (W2 views) | Expression evaluation. |
| ULID/NUID-style sortable ids, CSV import parsing (W1 row 5) | Trivially solved. |

### Ours — the differentiator, do not outsource

| Phase W capability | Why no library can carry it |
|---|---|
| **The issue record that `serves` a brief criterion** (W1 rows 1–2) | The `serves` edge and `thread state` are the product spine (`CLAUDE.md`: "any feature that breaks that thread is wrong"). No tracker models a criterion, so no tracker's schema can hold the edge. |
| **"Done means proven" (OD-27)** — own landed work, own *observed* evidence, *done, not proven* as a first-class state, and *sync can never write `done`* | Every OSS tracker's state machine is a free-form workflow where any actor can set any state. Adopting one would **give away the rule**, not implement it. |
| **Grouping by criterion as the default skeleton** (05 §4.1) | 05 §4.4 makes it a *structural failing test*: "a work screen whose default grouping is workflow state fails". Every OSS board is grouped by state. |
| **The agent line on the row, `Shift+R` Start, Undo-Start with a cost** (W1 rows 13, 15) | Binds an issue to a lane, a worktree and a managed session. Nothing outside this repo has the objects. |
| **Cross-owner Orchestrator negotiation on an issue** (W3) | OD-14–OD-17. Zero prior art, open or closed. |
| **The provenance events / authorisation rows** (W1 row 3) | The Hub's ledger, already designed. |
| **The visual language** (W1 rows 8–10) | `docs/DESIGN-SYSTEM.md` exists *because* the first prototype looked generated. Any component library with its own look is a cost. |

The practical rule that follows: **adopt headless things, reject styled things; adopt the client,
write the server.**

---
## 3. The candidates

Licence column = the SPDX id **read from the repository's own licence file on 2026-09-28** unless
the row says otherwise. Maturity = stars and last push, from the GitHub API on 2026-09-28.

### The whole list at a glance

| Verdict | Candidates |
|---|---|
| **adopt** | TanStack Virtual · TanStack Table (headless) · dnd-kit · React Aria **or** Radix (one) · Tiptap core (`@tiptap/*` only) · ProseMirror · uFuzzy · TanStack DB (behind our interface) · participle · `expr` or `cel-go` · squirrel · `oklog/ulid` |
| **vendor** (copy in, with notice) | cmdk (scoring + list state) · tinykeys (sequence matcher) · Gitea `dependency.go` · Gitea `issue_index.go` · Gitea `issue_xref.go` |
| **study-only** | TaskChampion sync spec · Bugzilla saved searches · Kanboard automation · react-querybuilder's filter AST · vis-timeline / frappe-gantt interaction · Taiga swimlanes · downshift · react-hotkeys-hook scopes · TanStack Hotkeys (re-check) · ElectricSQL shape-and-cursor design · Bleve |
| **reject** | Every whole tracker · BlockNote · Meilisearch · Typesense · Fuse.js · kbar / ninja-keys · react-querybuilder (the UI) · Yjs *for W1* · Zero (licence unread) |

### 3.1 Whole-app trackers: is there one we can take?

**Short answer: no.** The category is almost entirely copyleft or source-available, and the three
permissive ones are the wrong shape. This section exists so nobody re-runs this search in a month.

| Candidate | Repo | Licence (verified) | Language / maturity | What it is | Verdict |
|---|---|---|---|---|---|
| **Gitea — `models/issues`** | `github.com/go-gitea/gitea` | **MIT** — `raw.githubusercontent.com/go-gitea/gitea/main/LICENSE`, read 2026-09-28 | **Go**; v1.27.3 released 2026-08-29; ~52 files in `models/issues` | A production Go issue model: `issue.go`, `label.go`, `milestone.go`, `dependency.go`, `comment.go`, `reaction.go`, `issue_xref.go`, `issue_search.go`, plus tests for each | **vendor (narrow) / study-only** |
| **Kanboard** | `github.com/kanboard/kanboard` | **MIT** — `.../kanboard/kanboard/main/LICENSE`, read 2026-09-28 ("Copyright (c) Frédéric Guillot") | PHP, ~9k stars | Minimalist kanban with swimlanes, a plugin API and a decent automation model | **study-only** |
| **WeKan** | `github.com/wekan/wekan` | **MIT** — `.../wekan/wekan/main/LICENSE`, read 2026-09-28 | Meteor / Node, ~20k stars | Trello-shaped kanban | **reject** |
| **Apache Allura** | `github.com/apache/allura` | **Apache-2.0** — `.../apache/allura/master/LICENSE`, read 2026-09-28 | Python 2-era codebase, ASF | Forge with a ticket tracker, saved bins/queries and a milestone model | **study-only (low value)** |
| **Bugzilla** | `github.com/bugzilla/bugzilla` | **MPL-2.0** (README + About, read 2026-09-28) | Perl, 5.2 branch | The original enterprise bug tracker; a genuinely good **saved-search and custom-field** model | **study-only** |
| **TaskChampion** | `github.com/GothenburgBitFactory/taskchampion` | **MIT** (About sidebar, read 2026-09-28) | Rust, 184 stars | The storage + **sync replica** engine behind Taskwarrior: an operation log, a local replica and a documented sync protocol | **study-only — read the sync spec** |
| **Huly Platform (Tracker)** | `github.com/hcengineering/platform` | **EPL-2.0** — `.../hcengineering/platform/main/LICENSE`, read 2026-09-28; About sidebar also says EPL-2.0. 27.8k stars | TypeScript monorepo | The closest open Linear clone: issues, sub-issues, components, milestones, a command palette, real-time | **reject (licence)** — see note |

**Why Gitea is *vendor-narrow* and not *adopt*.** Gitea's issue model is the only permissive,
production-grade, **Go** issue schema in existence, and our Hub is Go with Postgres. Three specific
files are worth lifting, with attribution, rather than re-deriving:

- `issue_xref.go` — cross-reference parsing (the "magic words" / linkback behaviour W5 needs for
  outside PRs on both hosts). It handles the awkward cases: references in comments, closing
  keywords, reference to a different repo, and the resulting timeline comment.
- `dependency.go` — blocked-by / blocking with **cycle detection**, which W1 row 2 needs
  (`M` then `B`/`X`/`R`/`D`) and which is the one place a naive implementation quietly breaks.
- `issue_index.go` — per-project monotonic issue numbers under concurrency (our `project.key` +
  number, W1 row 1). The race is real and Gitea's fix is tested.

Everything else in Gitea is shaped for a git forge, uses XORM, and models a repository rather than
a project with a brief. Taking the *schema* would import a state machine that contradicts OD-27
("done means proven"). So: three files with attribution and a `THIRD-PARTY.md` entry, ~1 day saved
on W1 rows 1–2 and ~1 day on W5, not an adoption.

**Huly, on EPL-2.0.** EPL-2.0 is **not on the allowed list** and is not a permissive licence. It is
file-level-ish copyleft (§3.1: any modified Source Code must be made available under EPL), it has a
**§4 commercial distribution indemnity obligation**, and — the part that decides it — **§3.2's
secondary-licence clause can pull GPL-2.0 into scope for a downstream recipient**. For a product
KPMG ships to regulated customers on their own cloud, that is a legal conversation we do not need
to have. **Record it, do not read the source.** (This is the same failure mode as Paperdesign/OD-11:
an excellent product behind a licence that costs more than the code saves.)

### 3.2 Desktop UI pieces — this is where the saving is

The desktop tree today is **React 19 + zustand + xterm and nothing else**
(`desktop/package.json`, read 2026-09-28). There is no incumbent component library to fight, which
makes each of these a clean add. All are headless — they ship behaviour, not a look — so none of
them drags in a visual language that `DESIGN-SYSTEM.md` would have to fight.

| Candidate | Repo | Licence (verified) | Maturity (2026-09-28) | Replaces | Verdict |
|---|---|---|---|---|---|
| **TanStack Virtual** | `TanStack/virtual` | **MIT** — `.../TanStack/virtual/main/LICENSE` | 7,123★, pushed 2026-09-21 | The virtualisation inside W1 row 11 (Work lens list/board over 50+ rows) and the `aria-rowcount` grid the a11y floor (row 17) needs | **adopt** |
| **TanStack Table** | `TanStack/table` | **MIT** — `.../TanStack/table/main/LICENSE` | 28,459★, pushed 2026-09-16 | Grouping, multi-sort, column sizing and the **display options** menu in W1 row 11; W2's saved-view column state | **adopt (headless only)** |
| **dnd-kit** | `clauderic/dnd-kit` | **MIT** — `.../clauderic/dnd-kit/master/LICENSE` | 17,679★, pushed 2026-09-12 | Board drag, W2 `rank`/`move`, W4 timeline drag-to-reschedule. Ships **keyboard DnD and live-region announcements**, which the a11y floor (W1 row 17) otherwise costs us days | **adopt** |
| **Radix Primitives** | `radix-ui/primitives` | **MIT** — `.../radix-ui/primitives/main/LICENSE` ("Copyright (c) 2022 WorkOS") | 19,338★, pushed 2026-08-08 | Every picker and menu in W1 rows 11–13: assignee `A`, state `S`, priority `P`, label `L`, estimate `E`, the bulk bar menu, the `?` sheet dialog, tooltips, the context menu | **adopt** |
| **React Aria (react-spectrum)** | `adobe/react-spectrum` | **Apache-2.0** — `.../adobe/react-spectrum/main/LICENSE` | 15,894★, pushed 2026-09-27 | Alternative to Radix. Stronger on `forced-colors`, focus management and grid/listbox semantics — exactly W1 row 17's Windows contrast layer | **adopt one of Radix *or* React Aria, not both** |
| **cmdk** | `dip/cmdk` (was `pacocoursey/cmdk`) | **MIT** — `.../pacocoursey/cmdk/main/LICENSE.md` ("Copyright (c) 2022 Paco Coursey") | 12,991★, **last push 2025-10-29** — nearly a year stale; latest release v1.1.1 | The command palette in W1 row 13 (list, scoring, groups, keyboard) | **vendor, do not adopt** — see note |
| **tinykeys** | `jamiebuilds/tinykeys` | **MIT** — `.../jamiebuilds/tinykeys/main/LICENSE` | 4,099★, pushed 2026-09-25; ~650 bytes | The chord matcher under W1 row 13's `keys.list` grammar | **adopt or vendor** — see note |
| **react-hotkeys-hook** | `JohannesKlauss/react-hotkeys-hook` | **MIT** — `.../react-hotkeys-hook/main/LICENSE` | 3,503★, pushed 2026-09-24 | Alternative to tinykeys; adds React scopes, which maps onto "fires only when focus is in a list" | **study-only** — its scope model is the useful idea |
| **downshift** | `downshift-js/downshift` | **MIT** — `.../downshift/master/LICENSE` ("Copyright (c) 2017 PayPal") | 12,303★, pushed 2026-06-30 | Combobox/autocomplete state for the mention picker and the criterion-attach suggestion (`T`, W1 row 12) | **study-only** (Radix/React Aria already cover it) |
| **uFuzzy** | `leeoniya/uFuzzy` | **MIT** — `.../leeoniya/uFuzzy/main/LICENSE` | 3,033★, pushed 2026-09-08 | Fuzzy match + **highlight ranges** for the palette and `/` search over the in-memory replica; ~5 KB | **adopt** |
| **Fuse.js** | `krisk/Fuse` | **Apache-2.0** — `.../krisk/Fuse/main/LICENSE` | 20,491★, pushed 2026-08-09 | Alternative to uFuzzy. Heavier, slower on large sets, but configurable field weights | **reject** (uFuzzy is faster and smaller for this shape) |
| **TanStack Hotkeys** | `TanStack/hotkeys` | **MIT** — `.../TanStack/hotkeys/main/LICENSE` ("Copyright (c) 2026 Tanner Linsley") | **729★**, pushed 2026-09-27 — brand new | Type-safe keyboard shortcuts with a React adapter; the closest thing to a *keyboard grammar* library | **study-only for now** — too new to depend on, but re-check before W1 row 13 starts |
| **kbar** | `timc1/kbar` | **MIT** (GitHub API) | 5,255★, pushed 2026-08-10 | Alternative to cmdk | **reject** — ships a look and an action-registry model we do not want |
| **ninja-keys** | `ssleptsov/ninja-keys` | **MIT** (GitHub API) | 1,706★, **last push 2024-07-14** | Alternative to cmdk | **reject** — stale, web-component, own styling |

**cmdk: vendor, not adopt.** It is MIT and it is good, but the repository has not been pushed since
**2025-10-29** and the org name has changed (`pacocoursey/cmdk` now resolves to `dip/cmdk`) — which
is both a maintenance signal and a supply-chain signal. It is ~1,000 lines. **Vendor the scoring
function and the list/group/item state machine into `desktop/src/vendor/cmdk/` with its MIT notice**
and drive it from our own markup, which we want anyway because the palette must render our chord
hints inline (05 §4.3) and open beside the acting row (L-221). Saving: ~1.5 of W1 row 13's 3 days.

**tinykeys: read it before you depend on it.** 650 bytes, MIT, and it matches on `event.key`.
05 §4.3 rule 4 requires matching on **`event.code`** so Caps Lock cannot turn a record act into an
agent run. So either adopt it and wrap, or — better — **vendor its sequence-matching logic** (the
`G`→`I` two-key sequences) and keep our own `event.code` comparison. The existing
`desktop/src/hooks/useKeyboardMap.ts` already owns the dispatch; this is an ~80-line addition, not
a dependency. Saving: small, but it removes the sequence-timeout bug class.

### 3.3 The editor (W1 row 12, 4.5 d — the single largest desktop line after the Work lens)

W1 row 12 wants "the editor (Markdown, slash menu, tables), comment UI, attachments". Writing a
rich-text editor from scratch is a multi-week job; this row is the clearest *adopt* in the document.

| Candidate | Repo | Licence (verified) | Maturity | Note | Verdict |
|---|---|---|---|---|---|
| **ProseMirror** | `ProseMirror/prosemirror-state` et al | **MIT** — `.../prosemirror-state/master/LICENSE` ("Copyright (C) 2015-2017 Marijn Haverbeke and others") | The substrate under Tiptap, Notion-likes and Atlassian | Document model, transforms, collab plugin, schema | **adopt (as Tiptap's base)** |
| **Tiptap** | `ueberdosis/tiptap` | **MIT** — `.../ueberdosis/tiptap/main/LICENSE.md` ("Copyright (c) 2025, Tiptap GmbH") | 38,554★, pushed 2026-09-25 | Headless ProseMirror wrapper with a React binding; **Markdown, slash menu, tables, mention, task list are all core/MIT** | **adopt — core only** |
| **Lexical** | `facebook/lexical` | **MIT** — `.../facebook/lexical/main/LICENSE` (Meta) | 23,901★, pushed 2026-09-27 | Meta's editor framework. Smaller runtime; Markdown support is thinner, tables are newer | **the fallback if Tiptap's Pro boundary bites** |
| **BlockNote** | `TypeCellOS/BlockNote` | **MPL-2.0 core + GPL-3.0 `xl-*` packages** | 10,233★, pushed 2026-09-25 | GitHub reports the repo licence as `NOASSERTION` | **reject** — see trap |

**Trap 1 — Tiptap Pro.** Tiptap's *core* is MIT and covers everything W1 row 12 names. But
`@tiptap-pro/*` — **collaboration, comments, versioning, document conversion and the AI
extensions** — requires a paid subscription and is **not** open source. The README says so
("Pro Extensions need a valid subscription"; read 2026-09-28). Two of those names are things Phase W
does: comments (W1 row 12) and versioning (W1 row 1's issue revisions). **We do not need the Pro
versions of either**, because our comments are Hub records (`ic-`) with anchors and our revisions
are Hub `issue-revision` objects — the editor only has to render a threaded gutter and emit an
anchor. **The rule for the build: `@tiptap/*` is allowed, `@tiptap-pro/*` is a licence-gate
failure.** Add `@tiptap-pro` to `tools/licencegate`'s denylist so CI enforces it rather than a
reviewer remembering.

**Trap 2 — BlockNote.** Core is **MPL-2.0** (which our rule allows, conditionally), but the `xl-*`
packages — multi-column, AI, PDF/DOCX export — are **GPL-3.0 with a commercial alternative**.
Verified by reading `packages/xl-multi-column/LICENSE` directly on 2026-09-28: it is the full GNU
GPL v3 text. The *permissive core with a copyleft add-on* trap, exactly. One `npm i` of an `xl-`
package by a future agent puts GPL-3.0 in a product KPMG ships. **Reject the whole project** rather
than police a package prefix inside a monorepo that already mixes licences.

**Saving:** Tiptap core plus our own comment gutter takes W1 row 12 from ~4.5 d to ~2.5 d, and it
makes W2's "Markdown, slash menu, tables" free rather than incremental. **~2 days.**

### 3.4 The in-memory replica and optimistic writes (W1 row 14, 5 d)

05 §5.3 sizes this at 5 days and calls out four jobs: a cursor on `/v1/changes`, ordering by the
feed's sequence, **rolling back an optimistic write the Hub refuses**, and loading only what the
caller may read. That is a client-cache engine, and three exist.

| Candidate | Repo | Licence (verified) | Maturity | Replaces | Verdict |
|---|---|---|---|---|---|
| **TanStack DB** | `TanStack/db` | **MIT** — `.../TanStack/db/main/LICENSE` ("Copyright (c) 2025 Kyle Mathews") | 3,920★, pushed 2026-09-27, **BETA** | Most of W1 row 14: normalised collections, **live incremental queries**, optimistic mutations with automatic rollback on server rejection, and a pluggable sync source | **adopt — with a named risk** |
| **ElectricSQL** | `electric-sql/electric` | **Apache-2.0** — `.../electric-sql/electric/main/LICENSE` | Widely deployed | A **Postgres→HTTP shape-sync** server. Would replace `/v1/changes` itself | **reject for the Hub, study-only** |
| **Yjs** | `yjs/yjs` | **MIT** — `.../yjs/yjs/main/LICENSE` (Kevin Jahns / RWTH Aachen) | The reference CRDT | Only relevant if two people edit one issue description simultaneously | **defer — not a W1 need** |
| **Rocicorp Zero** | `rocicorp/mono` | root `LICENSE` is **Apache-2.0**, read 2026-09-28; **per-package licence NOT verified** (`packages/zero/LICENSE` 404, npm page 403) | — | Same job as TanStack DB | **do not adopt until the package licence is read** |

**TanStack DB is the right shape and the wrong maturity, and both matter.** Right shape: it is a
*client* store that sits in front of *your* API — it does not demand a sync protocol, a Postgres
extension or a schema, so it does not force an architecture we have locked. `/v1/changes` becomes a
custom collection's sync source; the Hub stays authoritative; ADR-H1's "no client persists Hub data
to disk" is honoured because the collections are in memory. Optimistic mutation with rollback is
the library's headline feature and is precisely 05 §4.6's `optimistic.undo`. **Wrong maturity:** it
is BETA, 3,920 stars, and the API has moved this year. **Recommendation: adopt behind our own
`replica/` interface** (about 150 lines) so the store is swappable; if it stalls we drop in zustand
selectors and lose weeks, not the phase. Saving if it holds: **~2 to 2.5 of the 5 days**, and it
removes the hardest bug class (a rejected write leaving the UI ahead of the Hub).

**Why ElectricSQL is a reject here even though it is Apache-2.0.** It is excellent, and it is the
wrong layer: it syncs *Postgres shapes* to clients, which would put the desktop in a read path that
bypasses the Hub's authorisation rows and the ledger. That directly breaks the "loading only what
the caller may read" requirement and the provenance spine. **Study it for the shape-and-cursor
design of `/v1/changes`; do not put it in the path.**

**Zero is unverified and therefore excluded for now.** The monorepo root is Apache-2.0, but
Rocicorp has historically shipped its sync products under bespoke terms, and I could not read the
package's own licence: `packages/zero/LICENSE` returned 404 and `npmjs.com/package/@rocicorp/zero`
returned **403 through the corporate proxy**. Per the rule, a licence I could not read is a licence
we do not rely on. Re-check before anyone proposes it.

### 3.5 Hub-side (Go) pieces: the filter grammar, search and ids

W2's views (`vw-`, 3.5 d) need "nested filters" that round-trip to a URL and to the Hub. There is
**no open-source Linear-grade filter DSL** — Linear's is proprietary and nobody has cloned the
grammar. But the two hard halves are both solved.

| Candidate | Repo | Licence (verified) | Replaces | Verdict |
|---|---|---|---|---|
| **participle** | `alecthomas/participle` | **MIT** — `.../alecthomas/participle/master/COPYING` | Parsing the filter grammar into an AST from struct tags — the `is:open assignee:@me serves:c-…` half of W2 views. ~a day of hand-written lexer work | **adopt** |
| **expr** | `expr-lang/expr` | **MIT** — `.../expr-lang/expr/master/LICENSE` ("Copyright (c) 2018 Anton Medvedev") | Evaluating a compiled filter over a record, sandboxed and fast. Also W5's **Loops** (`lp-`) trigger conditions and W4's intake rules | **adopt** |
| **cel-go** | `google/cel-go` | **Apache-2.0** — `.../google/cel-go/master/LICENSE` | Alternative to `expr`. Standardised, non-Turing-complete, auditable — an advantage for an enterprise policy surface (OD-23's "put our policies in") | **adopt one of `expr` or `cel-go`; prefer `cel-go` if the same engine is to serve policy** |
| **squirrel** | `Masterminds/squirrel` | **MIT** — `.../Masterminds/squirrel/master/LICENSE` (GitHub reports `NOASSERTION`; the file is plain MIT). 7,986★ but **last push 2024-04-24** | Turning a parsed filter AST into parameterised SQL against Postgres without string-building an injection | **adopt with eyes open** — stable-and-finished rather than abandoned, but it *is* two years quiet; it is also ~2k lines and trivially replaceable if it rots |
| **Bleve** | `blevesearch/bleve` | **Apache-2.0** — `.../blevesearch/bleve/master/LICENSE` | W1 row 3's "title search" and W2's search-as-you-type, if Postgres FTS proves too coarse | **study-only** — Postgres is already in the Hub; add Bleve only if FTS measurably fails |
| **oklog/ulid** | `oklog/ulid` | **Apache-2.0** — `.../oklog/ulid/main/LICENSE` | Sortable ids behind `ic-`, `cy-`, `ft-`, `vw-` etc. (the Hub's key grammar keeps its own prefixes) | **adopt** |
| **react-querybuilder** | `react-querybuilder/react-querybuilder` | **MIT** — GitHub API, 1,738★, pushed 2026-09-25 | The *visual* nested-filter builder in W2 views | **reject** — it brings its own look and its own control markup; `DESIGN-SYSTEM.md` would have to fight it. **Study its AST shape**, which is a sane nested `{combinator, rules[]}` model worth copying as a data format |
| **Meilisearch** | `meilisearch/meilisearch` | **`MIT AND BUSL-1.1`** — read from `.../meilisearch/meilisearch/main/LICENSE` on 2026-09-28 | — | **reject — relicensed.** The repo is now dual: the Enterprise Edition is **BUSL-1.1**, which is forbidden. Anyone working from memory would call this "MIT" and be wrong about part of the tree |

**Saving:** roughly **1.5 to 2 days** off W2's views row, and it makes the filter grammar testable
as a grammar rather than as a pile of string handling. The *semantics* — what `serves:`,
`thread:`, `proven:` and `agent:` mean — stay ours, because they are the provenance vocabulary.

### 3.6 Board, timeline and drag (W1 row 11, W4 timeline 3 d)

| Candidate | Repo | Licence (verified) | Replaces | Verdict |
|---|---|---|---|---|
| **frappe-gantt** | `frappe/gantt` | **MIT** (About sidebar, read 2026-09-28), 6.1k★ | A starting point for W4's timeline | **study-only** — it renders its own SVG and its own visual language; we need `DESIGN-SYSTEM` bars and the cycle band |
| **vis-timeline** | `visjs/vis-timeline` | **dual `Apache-2.0 OR MIT`** (About sidebar, read 2026-09-28), 2.6k★ | W4's timeline: zoom, drag-to-reschedule, grouping | **study-only / vendor** — mature interaction model, heavy DOM, dated styling |
| **dnd-kit** | (above) | MIT | The drag half of both | **adopt** |

Honest read: **the timeline is the one place where taking a library probably costs more than it
saves.** Both candidates own their rendering, and W4's timeline has to show the cycle band, the
criterion grouping and the thread gutter. Take `dnd-kit` for the interaction and draw the bars.

---

## 4. What this actually saves, task by task

Sizes are from `05-design.md` §5.3 and §5.4 and are **the design's own unmeasured estimates**; the
"after" column is my estimate and is equally unmeasured. Nothing here is a measured number.

| Phase W task | Now | With adoption | Delta | What does it |
|---|---|---|---|---|
| W1 row 1 — issue head, revision, numbers (Hub) | 3.5 d | 3.0 d | −0.5 | Gitea `issue_index.go` (vendored) for the per-project number race |
| W1 row 2 — relations, labels, triage, thread state (Hub) | 2.0 d | 1.5 d | −0.5 | Gitea `dependency.go` cycle detection (vendored) |
| W1 row 3 — events, authz, erase, title search (Hub) | 2.5 d | 2.5 d | 0 | **Ours.** Postgres FTS; no Bleve |
| W1 row 4 — comments `ic-` (Hub) | 2.0 d | 2.0 d | 0 | **Ours** |
| W1 row 5 — Linear + CSV import (Hub) | 1.0 d | 1.0 d | 0 | `encoding/csv` is stdlib; Linear's API client is 100 lines |
| W1 rows 6–7 — session ref, trailers, evidence | 4.5 d | 4.5 d | 0 | **Ours entirely** |
| W1 row 8 — the look | 4.0 d | 4.0 d | 0 | **Ours entirely** — DESIGN-SYSTEM |
| W1 row 9 — drawn glyphs | 2.0 d | 2.0 d | 0 | **Ours** |
| W1 row 10 — prototype | 3.0 d | 3.0 d | 0 | **Ours** |
| W1 row 11 — Work lens, board, bulk bar | 5.0 d | 3.0 d | **−2.0** | TanStack Virtual + Table (headless), dnd-kit |
| W1 row 12 — issue view + editor + comment UI | 4.5 d | 2.5 d | **−2.0** | Tiptap core (MIT) + Radix |
| W1 row 13 — keys, triage numbers, peek, `?`, palette | 3.0 d | 1.5 d | **−1.5** | cmdk vendored, tinykeys' sequence logic, uFuzzy |
| W1 row 14 — in-memory replica, optimistic undo | 5.0 d | 2.5 d | **−2.5** | TanStack DB behind our `replica/` interface |
| W1 row 15 — issue chips in Develop/Merge | 0.5 d | 0.5 d | 0 | **Ours** |
| W1 row 16 — the Desk frame | 4.5 d | 4.0 d | −0.5 | Radix for the card menus only |
| W1 row 17 — accessibility floor | 1.5 d | 0.5 d | **−1.0** | React Aria / Radix carry `forced-colors`, grid roles and live regions |
| **W1 total** | **48.5 d** | **~38.0 d** | **≈ −10.5 d** | |
| W2 views `vw-` | 3.5 d | 2.0 d | −1.5 | participle + squirrel (+ `expr`/`cel-go`) |
| W2 sub-issues, templates, estimates, rank, move | 3.5 d | 3.0 d | −0.5 | dnd-kit for rank/move |
| W2 the rest | 13.5 d | 13.5 d | 0 | **Ours** |
| W4 timeline | 3.0 d | 2.5 d | −0.5 | dnd-kit only; bars are ours |
| W4 insights / assists / initiatives | 20.5 d | 20.5 d | 0 | **Ours** |
| W5 outside PRs on both hosts | 2.5 d | 1.5 d | −1.0 | Gitea `issue_xref.go` (vendored) — magic words, linkbacks |
| W5 loops `lp-` triggers | 3.5 d | 2.5 d | −1.0 | `expr`/`cel-go` as the trigger language |
| W3 (agents on the work) | 12.5 d | 12.5 d | **0** | **Nothing in open source touches this** |
| **Phase W total** | **~138 d** | **~121 d** | **≈ −17 d (~12%)** | |

**Read that number honestly.** Twelve per cent is real but it is not the difference between
shipping and not shipping. The reason is structural: **Phase W's cost is concentrated in the
things that are ours** — the Hub's records, the provenance edges, the proof rule, the look, and the
agent behaviour. That is a good sign about the product, not a bad one about open source.

**Where open source saves far more than days: defect classes.** Four of the adoptions remove bugs
we would otherwise ship and then find in review — keyboard DnD and live-region announcements
(dnd-kit), `forced-colors` and grid semantics (React Aria), optimistic rollback ordering
(TanStack DB), and relation-cycle detection (Gitea). The Phase W plan review came back
**20 Critical / 153 Important**; a meaningful slice of the Important findings in any list-heavy UI
plan are of exactly these kinds.

---

## 5. The adoption plan, if the owner says yes

Five steps, in order. Each is small enough to finish.

1. **Extend the licence gate before anything is installed.** `tools/licencegate` already exists in
   the `phase-5` worktree and already classifies MIT / Apache-2.0 / BSD-3 across npm, Go and Cargo
   trees (`classify.go`, `spdx.go`, `npmsource.go`, `gosource.go`, `cargosource.go`). Add an
   explicit **denylist by package name** for the traps this research found: `@tiptap-pro/*`,
   `@blocknote/xl-*`, and any `meilisearch` enterprise package. A prefix rule is cheaper than a
   reviewer's memory. **Half a day.**
2. **Pick one of Radix or React Aria and write it into `DESIGN-SYSTEM.md`.** Both are fine; two are
   not. My recommendation is **React Aria** for Phase W specifically, because W1 row 17's Windows
   contrast layer and grid semantics are its strongest area and our weakest. **A day, including the
   ADR.**
3. **Vendor the four pieces** into `desktop/src/vendor/` and `hub/internal/vendor/` with their
   notices and a `THIRD-PARTY.md`: cmdk's scoring + list state machine, tinykeys' sequence matcher,
   Gitea's `dependency.go` cycle check and `issue_index.go`. **A day.**
4. **Put TanStack DB behind `replica/`** — our interface, its implementation — so its BETA status is
   a swap and not a rewrite. **Half a day of design, inside W1 row 14.**
5. **Write the filter grammar as a grammar**, with participle, and compile it to SQL with squirrel.
   Do this *before* W2 rather than after, because the grammar's vocabulary (`serves:`, `thread:`,
   `proven:`) is a product decision that wants to be settled once.

**Do not do:** adopt a whole tracker; adopt a styled component library; adopt ElectricSQL into the
read path; take anything from Huly, Plane, Focalboard, Planka or Forgejo.

---

## 6. The excluded register — projects we looked at and did not read

**No source file of any project in this table was opened.** Each row is a record that the project
exists, what it is, and the licence that excludes it, so the search is not repeated.

| Project | What it is | Licence (verified 2026-09-28) | How verified |
|---|---|---|---|
| **Plane** | The closest OSS Linear clone: issues, cycles, modules, views. 59,960★, active | **AGPL-3.0** | `raw.githubusercontent.com/makeplane/plane/master/LICENSE.txt` — read the text, it is the AGPL preamble. `gh api` agrees: `AGPL-3.0` |
| **Focalboard** | Mattermost's board/tracker. Now `mattermost-community/focalboard`, 26,490★, last push 2026-05-18 | **AGPL-3.0** (source); MIT applies only to the compiled binary; Apache-2.0 only to admin tools | `.../mattermost/focalboard/main/LICENSE.txt`. `gh api` reports `NOASSERTION` — the classifier cannot read a multi-licence file, which is the point |
| **Planka** | Trello-shaped kanban | **"PLANKA Community License"**, a Fair-Use/Sustainable-Use derivative — *source-available*, not OSI. Was **MIT** (≤ v1.x) → **AGPL-3.0** → bespoke at **v2.0** | GitHub repo page + README, read 2026-09-28 (`raw.../master/LICENSE` 404s — there is no root licence file). A search result cites `LICENSES/PLANKA Community License DE.md` in-tree; **that file was not opened** |
| **Huly Platform (Tracker)** | Linear-class tracker, virtual office, HR | **EPL-2.0** | `.../hcengineering/platform/main/LICENSE` + About sidebar |
| **Vikunja** | To-do / project app, Go backend | **AGPL-3.0** | `.../go-vikunja/vikunja/main/LICENSE` |
| **Typesense** | Typo-tolerant search engine. 26.6k★ | **GPL-3.0** | GitHub About sidebar |
| **Meilisearch** | Search engine | **`MIT AND BUSL-1.1`** — Enterprise Edition is BUSL | `.../meilisearch/meilisearch/main/LICENSE` |
| **BlockNote `xl-*`** | Editor add-ons | **GPL-3.0** (core is MPL-2.0) | `.../TypeCellOS/BlockNote/main/packages/xl-multi-column/LICENSE` |
| **Forgejo** | Gitea fork with issues | **GPL-3.0** — read directly; relicensed at **v9.0 (Oct 2024)**, was **MIT** | `codeberg.org/forgejo/forgejo/raw/branch/forgejo/LICENSE`, read 2026-09-28. **Note: Forgejo is not on GitHub** — `gh api repos/forgejo/forgejo` 404s. This is the fork boundary that keeps Gitea usable to us and Forgejo not |
| **Tiptap Pro (`@tiptap-pro/*`)** | Comments, versioning, collab, AI | **Proprietary, subscription** (core `@tiptap/*` is MIT) | Tiptap README, read 2026-09-28 |
| **It's a Plan (`itsaplan`)** | New (2026) self-hosted Linear/Plane alternative with AI agents | **AGPL-3.0** (`packages/runner` is Apache-2.0) | `.../croffasia/itsaplan/main/LICENSE` |
| **Tegon** | 2024-era "AI-first Linear alternative" | **AGPL-3.0**, and the repository is **ARCHIVED** (`RedPlanetHQ/tegon`, 1,912★, last push 2025-03-30) | `gh api repos/RedPlanetHQ/tegon`, 2026-09-28 |
| **Redmine** | Long-running PM/tracker. 6,039★, active | **GPL-2.0** — GitHub reports `NOASSERTION` (its `COPYING` is GPLv2 with the project's own preamble) | `gh api repos/redmine/redmine`, 2026-09-28. **Licence text not re-read this pass** |
| **MantisBT** | Bug tracker since 2000. 1,806★, active | **GPL-2.0** — GitHub reports no detected licence | `gh api repos/mantisbt/mantisbt`, 2026-09-28. **Licence text not re-read this pass** |
| **OpenProject** | Enterprise PM. 16,225★, active | **GPL-3.0** | `gh api` + `.../opf/openproject/dev/LICENSE` |
| **Leantime** | PM for non-PM people | **AGPL-3.0** | Project's own licence statement. **Not re-read this pass** |
| **Tuleap** | ALM suite. Hosted on its own Gerrit; `gh api repos/Enalean/tuleap` 404s | **GPL-2.0** | Project's own licence statement. **Not re-read this pass** |
| **tldraw** | Canvas | Commercial licence (already excluded by `CLAUDE.md`) | Prior decision, OD-11 context |

### One correction to the brief, and it matters

**Taiga is no longer AGPL-3.0. It is MPL-2.0.** The task listed Taiga under "KNOWN COPYLEFT (AGPL),
do not read". Read directly on 2026-09-28, **both** `raw.githubusercontent.com/taigaio/taiga/main/LICENSE`
**and** `raw.githubusercontent.com/kaleidos-ventures/taiga-back/main/LICENSE` are the **Mozilla
Public License Version 2.0** text. Under our rule MPL-2.0 is **CONDITIONAL-ALLOWED**: file-level
copyleft, so any *Taiga file we modified* would have to be published, but our own files linking to
it would not.

Two caveats before anyone acts on that:
- **The version boundary is UNVERIFIED.** I did not establish which Taiga release relicensed, so a
  pre-relicence copy in someone's cache may still be AGPL.
- **It changes nothing practically.** Taiga is Python/Django + Angular, its data model is
  Scrum-shaped (user stories, sprints, points), and it carries none of what we need. `gh api
  repos/kaleidos-ventures/taiga` reports **MPL-2.0, 572★, last push 2023-12-13** — the project is
  also quiet. It stays a **study-only** row on the strength of its *swimlane and custom-field*
  model, and nothing more.

This is the whole argument for the "verify from the source" rule in one example — and the search
engines produced the opposite error in the same session, with two separate results asserting that
**Plane is "MIT-licensed"**. `LICENSE.txt` in `makeplane/plane` is the **AGPL-3.0** text. A badge, a
blog and a listicle were all wrong about the two projects most likely to be proposed.

---

## 7. What I could not verify, and what the proxy blocked

### Blocked or failed fetches (KPMG Netskope proxy / rate limits)

| URL | Result | Consequence |
|---|---|---|
| `https://www.npmjs.com/package/@rocicorp/zero` | **HTTP 403** | Could not read Zero's declared package licence. Zero is excluded on that basis |
| `https://raw.githubusercontent.com/rocicorp/mono/main/packages/zero/LICENSE` | **HTTP 404** (no per-package file) | Same |
| `https://api.github.com/repos/tegonhq/tegon` | **ECONNRESET**, then **403** | Resolved afterwards through authenticated `gh api`: the repo moved to `RedPlanetHQ/tegon`, is **AGPL-3.0** and is **archived** (§6) |
| `https://api.github.com/repos/react-querybuilder/react-querybuilder` (unauthenticated) | **HTTP 403** rate-limit | Worked afterwards through authenticated `gh api` |
| `https://we.phorge.it/source/phorge/browse/master/LICENSE` | **HTTP 403** | Phorge (Phabricator's maintained fork, believed Apache-2.0) **unverified**. Not pursued — PHP, wrong stack |
| `https://raw.githubusercontent.com/typesense/typesense/main/LICENSE` | **HTTP 404** | Fell back to the GitHub About sidebar: GPL-3.0 |
| `https://raw.githubusercontent.com/plankanban/planka/master/LICENSE` | **HTTP 404** | Planka has no root `LICENSE`; licences live in `LICENSES/`. Established from the repo page instead |
| `https://raw.githubusercontent.com/jamiebuilds/tinykeys/main/LICENSE.md`, `.../frappe/gantt/master/LICENSE`, `.../TypeCellOS/BlockNote/main/LICENSE`, `.../visjs/vis-timeline/master/LICENSE-APACHE-2.0`, `.../Masterminds/squirrel/master/LICENSE.txt` | **HTTP 404** (wrong filename) | All resolved by a second fetch at the correct path or the About sidebar |

Everything else — `raw.githubusercontent.com`, `github.com` HTML, and `gh api` — worked from this
machine. **No page's content was guessed.**

### Unverified claims in this document

1. **Every day estimate**, both the "now" column (the design's own unmeasured figures) and the
   "after" column (mine). Nothing here has been measured against a build.
2. **Rocicorp Zero's licence** (see above). Excluded rather than assumed.
3. **Taiga's relicensing version boundary** (see §6).
4. **Whether `@tiptap/extension-table` and the slash-menu suggestion plumbing are fully MIT in
   Tiptap v3** — I read the repo's root `LICENSE.md` (MIT) and the README's Pro statement, but I did
   not enumerate every `packages/*/package.json` `license` field. **Do that as step 1 of the licence
   gate change**, not after installing.
6. **cmdk's true maintenance status.** Last push 2025-10-29 and the owner is now `dip` rather than
   `pacocoursey`. I did not establish whether that is a rename, a transfer or a fork-of-record.
   This is why the recommendation is *vendor*, which is immune to the answer.
7. **TanStack DB's BETA API stability.** Pushed 2026-09-27, 3,920 stars, and the docs still say
   BETA. The mitigation (our own `replica/` interface) is a design choice, not a verified fact.
8. **Bundle sizes.** None measured. The Tauri WebView2 payload grows with Tiptap + ProseMirror
   (the largest addition by far, roughly a few hundred KB gzipped) and barely at all with uFuzzy or
   tinykeys. Measure before the prototype review, alongside 05 §4.4 rule 6's grain measurement.
9. **That Gitea's three named files compile standalone.** They import Gitea's `db` package and its
   XORM session; vendoring means porting the logic, not copying the file. The day estimates assume
   a port, but the port was not attempted.

### What I deliberately did not do

- **No source of any AGPL, GPL, SSPL, BUSL or source-available project was opened** — not Plane,
  Focalboard, Planka, Huly, Vikunja, OpenProject, Typesense, Forgejo, Leantime, itsaplan, Redmine,
  MantisBT or Tuleap. Only their `LICENSE` files and public repository pages were read, which is
  what the rule requires and permits.
- **`~/Projects/Paperdesign`, `kgoedecke/doop` and every AGPL canvas source were not touched**, per
  OD-11. Phase 5 is not this document's subject and nothing here reaches into it.



