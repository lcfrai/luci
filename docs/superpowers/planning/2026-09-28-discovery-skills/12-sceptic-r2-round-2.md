# 12 — Sceptic, round 2 re-read: is design 11's Revision 1 closed at the root?

**Status: in progress, 2026-09-29.** Reviews `11-design-r2.md` at `b8b9031e` (Revision 1).
Read-only on every file except this one.

## 0. The verdict in one screen

_(written last)_

## 1. Round 1's Criticals and Importants: closed at the root?

Method: `12-sceptic-r2.md` first, then `11-design-r2.md` in ranges, then every citation below
against the tree: the main tree at `feat/phase-2-cockpit`; `p4` is
`.claude/worktrees/phase-4/control-plane/internal/` at `ca737d92`; `hub` is `.claude/worktrees/hub/`
at `cafe1303`. `11:N` is `11-design-r2.md` at `b8b9031e`; `HC`, `HP` and `T:` are as 11 §7 defines
them. Line numbers are the tree as read on 2026-09-29.

| Round 1 | Verdict | Evidence |
|---|---|---|
| **C1**, the grammar | **Closed at the root.** One consequence of the fix is new (I1 below) | The struct is the default (11:206-238). The only `term:` or `decision:` key forms left in the file are the owner's alternative (11:221) and Q2 (11:989). Q2 is asked before 44a, Hub 45 or phase 4's 26, and §0 no longer says no question blocks (11:58-62). By leaving `/v1/refs`, the struct also lost the one read of a past revision, and nothing replaces it |
| **C2**, the stamp | **Closed at the root** | One rule, *the set follows the stamp* (11:578-588), used the same way in §0 (11:43-45), §5.2 (11:643-646), the lens (11:790-793) and Q9 (11:996). It matches the built code: `Inheritable` carries one `Key` (p4 `discoverhub/read.go:79-86`), and the Materialiser stamps that key (p4 `contextwrite/materialise.go:385-393`) |
| **I1**, the record before its session | **Closed** | The create order is Materialise `:963`, id `:1005`, `Store.Create` `:1037`, then sandbox `:1087-1106`. The restart order is Materialise `:1724`, the update that sets `ContextRef` `:1806-1826`, then sandbox `:1836` (p4 `api/sessions.go`). Resume and fork both go through create (`:909-939`), so the two call sites cover every start |
| **I2**, false `edited` | **Closed as a rule.** One wording is left (M9a) | Set digests are compared before the text, and a mismatch reads `superseded` (11:700-712), placed where `stateOf` returns early today (p4 `inspect.go:202-207`). But the departure line re-opens I2 in a narrow case, through I1 below |
| **I3**, A20 turns the Hub red | **Closed** | Every test that reads the contract finds its section by number: `contractSection` (hub `hubapi/keys_test.go:362-380`), the body test's rows (`types_test.go:896-941`) and §8's kinds (`envelope_test.go:553-566`). The contract already has one appended amendment, §17 for OD-30 (HC:2301), so §18 is the contract's own pattern. The designer's reading of `pendingAmendment` is right: it holds code that runs ahead of the text, and nothing else (`types_test.go:881-885`, `:961-968`) |
| **I4**, cost | **Closed in part** | The five listed and the four repriced are in (11:850-906). The sums check: 36 + 6, 22.5 + 4.5, plus 0.5, makes 69.5. Still missing: the `Source`/`Inheritable` change (M9a), the read I1 needs, and a critical-path claim the design's own order cannot meet (M1) |
| **I5**, erasure keeps text | **Closed for stored replies and for a brief's erasure** (11:426-441, against `T:19.md:52-59`). A new hole opened in the design's own head column (I2 below) | — |
| **I6**, missing transitions | **Closed**: `depart` is born in one of three states, Undo writes `restore`, and a promote's Undo is one batch of restores (11:313-339) | One side effect: a false *project changed* (M6) |
| **I7**, a landed branch's decisions | **Closed by an option round 1 offered**: Q8, with its default and cost (11:995), plus the lens's *From archived branches* (11:795-802) | Q8 does not state the default's sharpest cost (M7) |

The designer's three departures from round 1 (11:1078-1088) are **all right**, and this review
withdraws the points they answer:
- Round 1's C1 said the struct would remove a §3.1 prefix row. It does not: §3.1 is the Hub's
  id-prefix table (HC:91-112), and `te-` and `dr-` are ids.
- The pending-amendment ratchet cannot hold contract text that runs ahead of its code.
- Q8 was one of the two fixes round 1 offered for I7.

## 2. Issues

### 2.1 Critical

None. Both round-1 Criticals are closed at the root (§1), and nothing below stops a plan edit.

### 2.2 Important

**I1. No route reads a past revision of a term or a decision, yet four parts of the design
render one.** This is a consequence of C1's fix. §2.4 (11:376-382), §5.1 (11:614-622), §5.2
(11:674-677), §6.1-6.2 (11:780-788).
- **What reads a pinned revision today.** A brief's branch revision pins a project revision, and
  the block prints that pinned text in its override note, `s.Replaced` (p4 `claudemd/render.go:92-93`).
  `discoverhub` fetches it "by its key, in one /v1/refs call" (p4 `discoverhub/read.go:572-605`).
  The brief has two reads for this: `/v1/refs` for a past revision's text, and
  `/v1/briefs/{id}/revisions` for its history (HC:790-796).
- **What the design leaves.** It has six new routes: two lists, "each the heads of one project",
  two batch writes and two erasures (11:376-380). "The resolver `GET /v1/refs` … does not change:
  no struct reaches it" (11:380-382). Nothing else reads revision *n* of a record.
- **Where the design needs such a read:**
  - A departure's origin line prints `<old decision>`, and a *not here* line prints
    `<the project's decision>` (11:614-622). Both name the revision the departure pins (11:279-284).
  - A departure shows *Project: <definition>* once the project entry has moved past its pin
    (11:786-788).
  - The *History* disclosure (11:780-782).
  - The claim that "every struct in the record names an immutable revision that reads back for
    ever", and that the block "re-derive[s] byte for byte" (11:674-677).
- **What fails if the lists are built as heads**, which is how 11:378 reads:
  - The Materialiser cannot print a departure's origin line from its pin. If it prints the head's
    text instead, the digests no longer cover that text. The overlay drops the overridden project
    entry from the branch's set (11:296-297). So when the project revises that entry, the rendered
    text moves but the marker's two digests do not. `Inspect` then renders, compares, and calls
    the block `edited` (p4 `inspect.go:208-215`). That is round 1's I2 again, for every branch
    that departs.
  - After any revise, what a session was given can no longer be read back, so §5.2's provability
    claim fails.
- **What it costs if the lists hold every revision** ("every revision's fields", 11:379): every
  session start reads the whole history of every record in the project, to the last page, inside
  the 5-second budget (11:590-594). It needs only the heads, plus the few revisions that
  departures pin.
- **Fix.**
  - Say the lists return heads, and state their order: "Order is stated per route" (HC:48-49).
  - Add one read per kind to §18: a record's revisions with their fields, or a `?revision=` on
    one record.
  - Have the Materialiser read the pinned revisions of the entries a branch overrides, one call
    per kind.
  - Price it at about 2 h across Hub 46 and 48b and phase 4's 27. This is part of what the struct
    costs, so Q2's "about 3 h more" for key forms (11:989) should shrink by the same amount.

**I2. An erased term's word survives in its head, and the scan test that would catch it cannot
see it.** §2.6 (11:404-434).
- **The head keeps a copy.** "Each head is a guarded state machine holding the current folded
  word, scope and state for the unique index" (11:407-408). Erasure empties every revision and
  takes the head out of force (11:331). "The text goes; the id, digest, number, `savedAt` and
  `savedBy` stay" (11:424-425). Nothing clears the head's folded word, and the function erasure
  is modelled on, `hub.erase_brief`, has no head text to clear (HC:1115-1127).
- **The test is blind to it.** `markerHits` matches `strpos(col::text, marker) > 0`, and the
  marker is `ZQX-MATTER-4471` (`T:27.md:72`). That match is case-sensitive, but the head holds
  the folded copy, `zqx-matter-4471`. So task 27's scan, extended to "a term and a decision
  holding its marker" (11:431-434), passes while the word is still in `terms`.
- **Why it makes the product wrong.** Erasure exists because "a definition can hold client text
  just as an answer can" (11:424). A word can be a client's codename. An org admin who erases it
  for a client is told the text is gone, and it is not.
- **Fix.**
  - `hub.erase_term` sets the head's folded word to `null`, and task 47's database test checks it.
  - Task 27's scan also searches for `FoldWord(marker)`, and puts the marker in `word`.
  - About 0.25 h, in Hub 47 and 48b.

### 2.3 Minor

**M1. The critical path grows by about 24 hours, not 3.5.** §0 (11:17-19), §7.5 (11:936-943).
This is round 1's M6, restated with numbers.
- **Now.** The path is Hub H0 → 07 → 08 → 09 → 11 → 12 → 14 → 23 (`plan:430`). Before task 14,
  what remains is 09's open fix round, then 11 (3 h) and 12 (2 h) (`plan:400-403`): about 5 h
  plus one fix round.
- **Under the design.** 14 now waits on 32, which waits on 29, 27 and 30 (11:884, :887). Here is
  the longest chain the design's own dependencies allow:
  - Hub 44a: 1.5 h, plus 2 h for A20 (HP:400, 11:920).
  - Hub 45: 3.5 h.
  - Phase 4's 25 (2.5 h), then 28 (4.5 h). Task 25 cannot "start now" (11:937): its overlay and
    sort call `hubapi.FoldWord` (11:360-364, :630-632), and Hub 45 builds that function (11:896).
    It reaches phase 4 only by merge (11:864).
  - Phase 4's 27 (6 h), 29 (5.5 h) and 32 (3.5 h).
  - That is about 29 h of work before 14, against about 5 h today.
- **The condition cannot be met.** "If 25-30 and Hub 44a-46 run beside it" (11:941-942) cannot
  hold: that chain is sequential by its own dependencies. And 32 carries seven routes plus task
  12's rework in 3.5 h, where 11 and 12 together took 5 h for about as many.
- **Fix.** State about +24 h on the critical path, before fix rounds, in §0 and §7.5. Or
  shorten the chain: land `FoldWord` and the field-rule function as a small first Hub commit ahead
  of the rest of 45, and let 25 start on it.

**M2. The two passes run an exit before the create it names.** §2.4 (11:313-318, :325, :412).
- `supersede` requires `replacedBy`, which may name "an item of this batch", and `retire` takes
  one too. Both take a record out of force, so both run in the first pass. The item they name is
  a `create`, which runs in the second pass.
- `replacedBy` is a foreign key to the head (11:412). So the natural batch, *create "track",
  retire "lane" in its favour*, names a head that does not exist yet when the retire applies.
- **Fix.** Mint every id and insert every new head before the first pass, or make the key
  `DEFERRABLE INITIALLY DEFERRED`. Say which in §2.4 and in task 47.

**M3. Record all is two batches, not one.** §3.2 (11:479-482) says **Record all** "writes them in
one batch" and that "the batch is all or nothing". But writes go through "one batch route per kind"
(11:307-308), and a round holds both kinds (11:516).
- The terms can therefore be recorded while the decisions are refused.
- An Undo of the whole round spans two restore batches.
- **Fix.** Say it makes two calls and in which order, what the rows show when only one call
  succeeds, and that its Undo restores both.

**M4. The third word rule needs the branch's view in the Hub, and it has a hole across scopes.**
§2.3 (11:296-303), §2.4 (11:355-358).
- Rule 3 checks "another in-force term on the same branch's view". But what a branch sees "is
  computed by phase 4, never by the Hub" (11:296). So the overlay, including its
  later-project-word shadow, is built three times: phase 4's 25 (11:880), the fake in Hub 46
  (11:897) and the server in Hub 48a (11:899). The one-owner rule that §2.4 applies to §2.2's field
  rules (11:366-371) is not applied to the overlay.
- **The hole.** A project write is allowed so that one branch cannot block the project's words
  (11:302-303). So a project word *track*, written while a branch term says *lane, not track*,
  goes in. That branch's block then says both "use lane, not track" and "track: …".
- **Fix.** Put the overlay in `hubapi` beside `FoldWord`, one function called from all three
  places. Extend the later-project-word shadow to a project word that a branch term avoids, with
  the same *project changed* treatment.

**M5. A *not here* departure is one gesture, but §2.2 requires fields for it.** A term revision
needs a `word` of 1-64 and an `is` of 1-300 code points (11:247-248). A decision revision needs a
`decision` of 1-300 (11:254). The `depart` item lists only `baseRevision`, `scope`, `overrides` and
the state it is born in (11:328). So as written, a *not here* departure is refused with 400 by the
first write-time check (11:341). §6 also names no verb that creates one: §6.2 only describes how a
departure reads, and the refused row offers **Depart on this branch** (11:818).
- **Fix.** Say a *not here* departure copies the pinned entry's fields. Name its verb in §6.2,
  for example **Not on this branch**, beside the project entry on a branch.

**M6. Undoing a promote leaves a false *project changed*.** A promote's Undo restores the project
entry it retired (11:335-337). A `restore` is a new revision, `@n` → `@n+1` (11:330), so the project
entry comes back at `@k+2`, with the same fields as `@k`. The restored departure still pins `@k`, and
a departure whose project entry "has moved past the pinned revision" takes task 19's *project
changed* treatment (11:786-789). So the person who pressed Undo is then asked to choose between
**Keep this branch's** and **Use the project's**, although nothing changed.
- **Fix.** Compare the pinned revision's fields rather than its number, or have the Undo batch
  re-pin `overrides` to the restored revision.

**M7. Q8's default does not state its sharpest cost, and the lens cannot tell a landed branch
from an abandoned one.** §6.2 (11:795-802), Q8 (11:995).
- **The cost.** After a land, main's code carries the branch's departures, but main's sessions
  get the project's set (§5.1). When the branch redefined a project decision, or set it aside,
  every agent on main is handed the project's entry instead, which main's code no longer follows.
  The stamp reads *current*. This lasts until a person opens the disclosure and presses Promote,
  and nothing prompts them to. Q8 prices the other answer at "about 2 h", but states the default's
  cost only as "nothing asks".
- **The label.** A branch brief is archived on branch deletion, "landed or not" (11:291; HC:785-788).
  The archive route takes `{}` and records no reason. So *From archived branches* offers
  **Promote** on the decisions of abandoned work, beside those of landed work, with nothing to
  tell them apart.
- **Fix.** Put the default's cost in Q8's row. Label each archived branch *landed* or *not
  landed*, taken from phase 3's land record, or say the lens cannot tell.

**M8. Three small corrections to the contract edit.** §7.4 (11:920, :926), §5.2 (11:652-654).
- **Point from the old sections to §18.** §17's precedent marks each amended heading in place,
  on the same line: "— **amended by OD-30, see §17**" (HC:298, :367, :369). §18 should do the same
  for §3.1, whose prefix table a new phase reads to choose prefixes (`work plan:185-187`), and for
  §4.2, §7.0 and §10.2. Without that, a reader of §3.1 never learns that `te-` and `dr-` are taken.
  But leave §8's heading unchanged: `contractEventKinds` matches `\n## 8. Events\n` exactly
  (hub `hubapi/envelope_test.go:560`).
- **`settled`.** Task 44b folds `settled` into §7.12 "in place" (`task-44b.md`, step 1), at the
  reply on HC:1126. If it adds a line, it moves the 1,259 lines below, which 240 inbound
  `hub-contract.md:NNN` citations name (a floor, by grep of every worktree, 2026-09-29). Keep it
  on that line.
- **The two set envelopes** (`term-set` and `decision-set`, 11:652-654) get no golden vector,
  although every marker, file, `Ref` and inheritance entry compares them for ever. §7.2 adds
  vectors for the revision envelopes only (11:860). A §3.5-style vector pins them (HC:176-197).
  Also, name the value of their `"scope":…` member.
- **The two revision envelopes are written only as `{"v":1,"kind":"term-revision",…}`**
  (11:415-417). §3.4 lists every member of the brief's envelope (HC:144-155). §18 should do the
  same: which of `word`, `is`, `avoid`, `because`, `scope`, `state`, `overrides`, `replacedBy`,
  `settledIn`, `author` and `sessionId` each digest covers. The digest certifies exactly those
  members, and the golden vectors cannot be written until they are named.

**M9. Six smaller gaps.**
- **(a) Which object carries the sets.** `Inspect` holds an `Inheritable`, not a `Ref` (p4
  `inspect.go:119-124`, `:179`). So "compares … with the `Ref`'s current two" (11:704-705) should
  read "with the two a start would write now". The sets reach the Materialiser only through its
  one `Source` method, `Inheritable(ctx, Target)` (p4 `materialise.go:51-53`). So
  `discoverhub.Inheritable` (`read.go:79-86`) gains the two sets. §7.2's `read.go` row does not
  list that change (11:859), and `binding_test.go:342` compares a whole `Inheritable` with
  `reflect.DeepEqual`.
- **(b) The one-call limit.** "At most one model call per complete revision" (11:518) is per
  machine and per round file. A fresh round comes after fourteen days (11:557-558), and another
  machine recomputes one (11:561-562). Say so.
- **(c) A second case-insensitive rule.** The Hub already indexes `lower(name)` for teams and
  projects (hub `migrate/sql/0001_init.sql:50`, `:99`), and the fake compares with
  `strings.ToLower` (`hubclient/fake_projects.go:159`). Say why a word uses `FoldWord` instead: a
  fold stored by Go does not depend on the database's collation. Also say that the fake calls
  `FoldWord`, not `ToLower`.
- **(d) A paged read is not a snapshot.** A batch that promotes one entry and retires another can
  land between two page reads. The set then holds both in-force entries for one word, or neither,
  under a digest that describes no moment the Hub held. **Fix:** note the feed position before and
  after the read, and re-read when it moved.
- **(e) One avoided word.** "Phase 3's merge" (11:753, :801): `CONTEXT.md:300` says *land*.
- **(f) Task 12 changes in four routes, not one.** §7.4 amends only `materialisations` (11:912).
  But task 12's two `provenance` routes return a `Ref`, which now carries the set digests. That
  means list reads, where each route's audit line promises "One Hub head read." Its `refs` route
  "reads through an in-memory cache of immutable revisions" (phase 4's `task-12.md:128-131`), and
  the set digests are neither immutable nor cached. List all four under task 32, with their new
  audit lines.

## 3. What held up

- **The grammar does not move.** No key form is added anywhere in the design. `hubapi.ParseKey`,
  `provenance.Parse` and `/v1/refs` are untouched. `te-` and `dr-` live in their own file, as
  phase W's prefixes do (`work plan:236`), so the ten-check keeps its ten: it reads only §3.1 (hub
  `hubapi/keys_test.go:740`).
- **§18 is the contract's own way to amend.** The contract already appends §17 for OD-30, and its
  tests read by section number (§1, I3). HC:32-35 counts a new route, field, code or event kind
  as additive. A20 moves no line.
- **One rule for the stamp.** *The set follows the stamp* is stated once and used the same way in
  every place that depends on it. The rule not taken is Q9, with its cost.
- **The handler writes after the stamp.** Both call sites were read, and every start goes through
  one of them.
- **A stored reply keeps no text.** It uses route 26's rebuild (`T:19.md:52-59`) for both kinds.
  A brief's erasure names what settled in it, and erases nothing a person did not choose.
- **No ceremony.** Every addition in this revision is a line or a verb, never a prompt, a count or
  a queue item: *Reaches sessions once this brief is complete*, *From archived branches* with
  **Promote**, and the refused rows.
- **The sums are honest.** 36 + 6 + 22.5 + 4.5 + 0.5 = 69.5 h. The fix-round multiplier is
  labelled a judgement (11:957-960). The four repricings are tied to the plans' own tasks.
- **Round 1's review was wrong in three places, and the designer said so** (11:1078-1088). All
  three corrections check against the tree (§1).

## 4. What this review did not verify

1. **Nothing was compiled or run.** In particular, M2's failure depends on how task 47 declares
   the foreign key. I2's failure depends on the scan's `strpos` (`T:27.md:72`), which is in a plan
   part, not in built code.
2. **The Hub's server-side tests that read the contract.** The authorisation harness (16b) and
   Hub 14's feed test are plan parts, and their built code was not read. That they read §7.0 and
   §8 by section, and so ignore §18, is taken from the design (11:1017-1018). A grep of the Hub's
   `control-plane` found no test that reads §7.0.
3. **M1's hours are the plans' own estimates** (`plan:390-414`, HP:400). The chain assumes the Hub
   runs 44a, 45 and 46 first, as §7.5 orders (11:944). If the Hub stream keeps its queue order
   (08 through 44b first, hub `.superpowers/sdd/hub/progress.md:522-523`), the path waits much
   longer.
4. **The count of 240 citations below HC:1126 is a floor.** It matches `hub-contract.md:NNN` in
   `.md`, `.go` and `.ts` files only. `HC:` and `§`-only citations are not counted.
5. **Phase 2's final gate is running in the main tree.** The `api/sessions.go` lines cited here
   and in 11 §5.2 are phase 4's copy at `ca737d92`, and may move when phase 2 lands.
6. **Q2, Q8 and Q9 are the owner's.** This review argues only that Q8's row should state the
   default's cost (M7).

## Closing

## Working notes (deleted at the end)

- Read 12 (all) and 11:1-384. Leads: (a) two-pass order vs `{"item":0}` forward refs (11:313-318):
  an exit item naming a create's id runs before that create; (b) rule 3 needs the branch's view in
  the Hub, but 11:296 says the Hub never computes it; (c) is there any read of a pinned past
  revision? the list is heads (11:378-380); (d) `settledIn: null` vs OD-36 "reference a brief
  revision"; (e) phase 4 gets `hubapi.FoldWord` "by merge" (11:364): an ordering dependency.
- Read 11:384-683. Leads: (f) Record all "one batch" (11:479) but one route per kind (11:307);
  (g) handler writes inheritance on create+restart only (11:669-670); PC:182 stamps at resume and
  fork too; (h) "reads back for ever" (11:674) needs a read of a pinned revision, and the list is
  heads; (i) erase keeps the head's folded word? (11:408 vs :424); (j) "at most one model call per
  complete revision" (11:518) vs 14-day re-round (11:557) and other machine (11:561).
- Read 11:681-966. Leads: (k) rule 3 needs the overlay in the Hub AND the fake (46) AND phase 4
  (25): three copies, against 11:296 and the one-owner rule; cross-scope avoid hole remains;
  (l) 44b "writes `settled` into §7.12" (11:926): an in-place contract edit, vs "no line moves";
  (m) is the body test section-scoped? (11:920 cites `keys_test.go:371`); (n) critical path:
  44a→45→46→p4 27→29 (~20 h + 44a) vs 09→11→12; is +3.5 h credible?; (o) Undo of promote
  restores the project entry at a new revision, so the departure shows a false *project changed*.
- Verified: I1 closed (sessions.go:963/1005/1037/1103/1806-1817; resume and fork go through
  create, :909-939). I3 closed: every contract-reading test reads its section by number
  (`keys_test.go:362-380`, `types_test.go:896-941`); HC already has §17 appended for OD-30 (HC:2301),
  so §18 is the contract's own amendment pattern. 44b folds in place by design (`task-44b.md`).
