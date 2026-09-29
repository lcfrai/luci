# 12 — Sceptic, round 2: why design 11 would fail

**Status: complete, 2026-09-29.** Reviews `11-design-r2.md` at `bd9b6299`. Read-only on every
file except this one.

Method: `06-sceptic.md` first, then `11-design-r2.md` in ranges, the fact sheets 11a and 11b, then
about 45 of the design's citations and every one this review makes, checked against the tree (main
tree at `feat/phase-2-cockpit`; the `phase-4`, `hub`, `phase-5` and `work` worktrees read-only).
Line numbers are the tree as read on 2026-09-29.

Abbreviations: `11:N` is `11-design-r2.md` at `bd9b6299`; `HC`, `HP` and `PC` are the Hub
contract, the Hub plan and the provenance contract draft, as 11b defines them; `p4` is
`.claude/worktrees/phase-4/control-plane/internal/` at `ca737d92`; `hub` is `.claude/worktrees/hub/`
at `cafe1303`; `work plan` and `phase-w review` are
`.claude/worktrees/work/docs/superpowers/plans/2026-09-25-phase-w-work-layer.md` and its
`-review.md`; `plan:N` is phase 4's plan, as 11 §7 defines it.

## 0. The verdict in one screen

**Needs fixes: 2 Critical, 7 Important, 8 Minor.** The central move holds: terms and decisions are
their own Hub records beside a brief that does not change by a byte. Round 1's ceremony and
truncation findings are answered for real. Two things stop any plan edit:

1. **C1, the grammar.** Q2's default adds two key forms to the Hub's grammar, although OD-36's row
   lists "the provenance key grammar" under *What does not move*. 11's §0 still calls Q2
   non-blocking.
   **Fix:** flip the default to a `{kind, id, revision}` struct, which the design itself prices
   about 1.5 h cheaper, or put Q2 to the owner before A20, Hub 45 or phase 4's 26 starts.
2. **C2, the stamp.** A branch with no complete brief revision starts under the project key. §5.1
   then hands its session the branch's overlay, while §5.2 gives that key the project's digests in
   one shared `Ref`. The result is a permanent false *words moved*, or a departure silently dropped.
   **Fix:** choose one rule and write it down.

The seven Important issues, each with its fix in §2.2:

| # | Issue | Where it fails |
|---|---|---|
| I1 | The inheritance record has no session id on create, and stamps creates that fail | `api/sessions.go:963` runs before `:1005` |
| I2 | The drift check reports a hand-edit whenever the project's words move | `inspect.go:208-215` |
| I3 | A20 in a documents-only 44a turns the Hub's contract-reading tests red; phase W adds kinds its own way | `keys_test.go:737-759` |
| I4 | §7.2 misses five changes to built code, and the honest total is near 66-70 h, not 54 h | §7 |
| I5 | Erasure leaves text in stored replies and in derived terms | §2.6 |
| I6 | Four transitions have no op, including 06 §5.1's *not here* for a decision | §2.4 |
| I7 | A branch's own decisions leave the thread when the branch lands | §6.2 |

**Next:** one fix round on 11, C1 and C2 first, because §2.1 and §5.1-5.2, which they change, are
what the rest of the design leans on.

## 1. The round-1 findings: answered, or renamed?

`10-decision-terms-beside-the-brief.md:66-72` names round 1's four structural failures. **Two are
answered for real** (the ceremony and the certified truncation). **Two are answered only in part**:
built code is now listed by file but still under-priced, and the Hub's grammar is still amended,
which OD-36 then ruled out in so many words. Each of `06-sceptic.md`'s findings, one line each:

| 06 | Finding | Verdict on `11-design-r2.md` |
|---|---|---|
| §1.2 | the criterion fragment duplicates phase W's `{briefKey, criterionId}` | **Answered.** The fragment is gone and no criterion citation is added (11:67-71) |
| §1.3 | the premise about the grammar was stale | **Answered**, and used as the argument for new forms, which is where C1 starts |
| §1.4 | the rounds' reuse was overstated | **Answered.** Task 21 is untouched; tasks 30 and 31 are priced as new work (11:77-79) |
| §1.5 | the provability seed re-asks `done` | **Answered.** Dropped; the interview gains no question (11:80-83) |
| §2.1 *(structural)* | built code priced as paper | **In part.** §7.2 now names built files, but misses five changes, and Hub 48's hours are half what the brief's own server tasks were priced at for one kind (I4) |
| §2.2 *(structural)* | the key grammar is the Hub's | **Renamed.** The brief's form does not move, but the design still amends the Hub contract's grammar and both parsers, the two costs 06 said the struct avoids (06:348-351) and OD-36 lists under *What does not move* (C1, I3) |
| §2.3 | re-anchoring was unpriced | **In part.** Phases W and T are untouched and new tasks go at each plan's end. But the Hub contract is edited in place in about twelve places (11b §1.9), and it has **517** inbound `hub-contract.md:NNN` citations across six worktrees (work 228, phase-4 127, team 118, hub 33, phase-5 11; grep, 2026-09-29). 11:756's "re-anchored once" leans on 44a, which re-anchors nothing: its own text says "Find each place by its quoted text, or add 9" (`task-44a.md:31-34`) |
| §3.1 *(structural)* | a four-gate form in front of an intermittent user | **Answered.** One gesture, no checkbox, Save live from the first character, no count (11:109-115, §3.2, §6.2) |
| §3.2 | three mechanisms that assume a chat | **Answered**: a round carries its base and is re-checked (item 1); no stall detector or closed state (item 2); throughput stated as a residual with Q4 (item 3) |
| §4.1 *(structural)* | a stamp over a truncation past 40 terms | **Answered**: two files hold every entry before the harness starts and every digest covers the whole set (11:569-573). Two new holes in the same place: C2 and M4 |
| §4.2 | a loose-file glossary for us, not for the product | **Answered**: the distinguishing sentence is written (11:142-145) |
| §4.3 | process state inside immutable content | **Answered.** Rounds are in neither `Content` nor the Hub (11:448-450) |
| §5.1 | a branch cannot dissent from a project decision | **Answered for words, renamed for decisions.** A decision departure can replace the project's, but cannot set it aside: no decision state means *not here* (I6) |
| §5.2 | retiring an inherited term has no op | **In form.** `depart` with `state: retired` (11:157) contradicts `create`'s only transition, `nothing → @1, in_force` (11:282, :288) (I6) |
| §5.3 | an id collision is resolve-time | **Answered** by a unique index at write (11:295-297). A departure's word and every `avoid` list stay unchecked (M3) |
| §6 | intellectual property | **Answered.** No upstream text; the upstream `LICENSE` is still unread (11:169), carried |
| §7 | the one change worth making now | **Taken in** by task 28 (11:171-174) |
| §7 | the shelf conditions | Two of three met. *Phase 4 ships first* is deliberately unmet, licensed by the owner's note (`owner-decisions.md:409`). *The same shape as `{briefKey, criterionId}`* is unmet by choice (11:180-186): that choice is C1 |

## 2. Issues

### 2.1 Critical

**C1. Q2's default contradicts OD-36's own words, and the design builds on it while saying no
question blocks the build.** §0 (11:51-54), §2.1 (11:195-214), Q2 (11:817), Hub 45, phase 4's 26,
44a's A20.
- **The owner's text.** OD-36, *What does not move*: "the brief content model, and the provenance
  key grammar, which the Hub contract owns" (`owner-decisions.md:409`). The decision note, under
  *What it does NOT change, and this is the point*: "The provenance key grammar, which the **Hub
  contract** owns, two parsers compile, and 64 plan files across six worktrees quote" (`10:54-55`).
- **What the design does.** It adds `term:` and `decision:` to the contract's §3.2 (A20), to both
  parsers (Hub 45, phase 4's 26) and to the resolver (Hub 20, 48). It reads *does not move* as
  *no existing form changes* (11:817). But the note names the two compiled parsers as the thing
  protected, and the design edits both.
- **Why it is still the owner's call, not a defect of reasoning.** The same note sketches
  `term:tm-7f2a@2 -> brief:bf-9c1d@4` (`10:14`), and `PC:104-107` made `kind:` extensible. The
  owner's record points both ways. The design was right to ask Q2. It was wrong to take the reading
  that contradicts the register row as the default.
- **What fails.** §0 calls Q2 "the one whose other answer changes the design's shape" and, in the
  same paragraph, says none of the seven questions "blocks the build" (11:51-54). If the owner means
  what the row says, then Hub 45, phase 4's 26, A20, the resolver rows in Hub 20 and 48, the set
  digest's `{"key","digest"}` items (11:536), the inheritance record's `keys` (11:539-540), the
  lens's key line (11:640-641) and task 16's key source are all rebuilt after they ship.
- **The alternative is cheaper by the design's own count**: about 1.5 h (11:817). It also removes
  most of I3: no §3.1 prefix row for the ten-check, no second `hubapi/keys.go` edit racing phase W,
  and no new `kind` value in `/v1/refs`.
- **Fix.** Make the struct `{kind, id, revision}` the default, since it is the plain reading of the
  row and 06's shelf clause (06:348-351). Keep key forms as the owner's alternative. Put Q2 in front
  of the owner before A20, Hub 45 or phase 4's 26 starts, and delete "none of which blocks the
  build" from §0.

**C2. Under a project key, §5.1's branch overlay and §5.2's shared `Ref` cannot both be true, so
either the stamp never matches or a branch's departures are dropped.** §5.1 (11:484-486), §5.2
(11:529-534, :552-554).
- **The facts.** `Inheritable` is "the latest complete branch revision, else the latest complete
  project revision, else none" (`p4 discoverhub/read.go:77-80`). A session on a branch whose own
  brief has no complete revision is therefore stamped with the **project** key. §5.1 says what the
  branch sees is its overlay. §5.2 says a project key's `Ref` carries "the project's" set digests,
  and "two sessions with one contextRef still share one Ref" (11:533-534). A branch record can exist
  before its brief is complete: it anchors to the branch brief's id (11:236), which exists from
  `headRevision: 0` (HC:759), and `settledIn` may name an open revision (11:329-330).
- **What fails, concretely.** Branch B's brief has two open revisions. On B, the project decision
  *uploads go direct* was departed from with *uploads go through presigned URLs*. A session on B
  starts under `brief:<project>@4`. There are two outcomes, and both are wrong:
  - **The Materialiser writes B's overlay.** The inheritance record then holds the overlay's
    digest, but the `Ref` for `@4` carries the project's. Develop shows *the project's words moved
    since this session started* (11:552-554) for as long as the session lives, although nothing
    moved. That is a false provenance claim.
  - **It writes the project's set.** The agent on B is then handed *uploads go direct*, the
    decision its branch departed from. That loses a decision on the Develop hop.
- **One `Ref`, many sets.** A session on branch C under the same key gets C's overlay, so one
  `Ref` stands for two different sets. "`Ref` stays one brief key resolved" (11:533) is false for
  every project key used on more than one branch.
- **Fix.** Choose one rule, write it down and price it:
  - **Either** the set digests are keyed by `(contextRef, branch)` and served beside the `Ref`, not
    inside it. OD-36's "`Ref` carries their digests" then needs the owner's reading.
  - **Or** a project key inherits the project's set only, and the lens says a branch's own entries
    reach sessions from its first complete revision.

### 2.2 Important

**I1. The inheritance record is written before its session exists, by a call that has no session
id.** §5.2 (11:538-547), §7.1 (11:690-691), §7.2's `api/materialiser.go` row (11:701).
- **The built order.** Create calls Materialise "after the request validates … and before anything
  is recorded or created" (`p4 api/materialiser.go:13-16`). `api/sessions.go:963` calls it; `:1005`
  mints `sessionID := ids.New("s")` afterwards, under "3. Ids and the secret". The handler promises
  that a failure after Materialise leaves "nothing behind but the brief block, which is
  git-excluded and stamps nothing" (`:972-973`, `:983-985`).
- **What fails.** `MaterialiseRequest.SessionID` (11:545) has no value on create. Minting the id
  earlier reorders phase 2's handler, which §7.1 says does not change. Six exits after `:963` abort
  a create before its record exists: the conversation store, line endings, git files, worktree
  admin, the conversation carry and the record write itself (`:977-1081`). Each would leave an
  inheritance entry for a session that never existed, which is exactly the stamp the handler
  promises not to leave. An id collision (`:1070-1079`) would append a failed start to another
  session's "immutable" record.
- **Fix.** `MaterialiseResult` returns the two sets' keys and digests. The handler writes the entry
  after `Store.Create` succeeds, beside `contextRef`. That changes `MaterialiseResult` and the
  handler (task 10's code in phase 2's package), so strike "phase 2 entirely" from §7.1 and price
  the change in task 29.

**I2. The drift check calls a block `edited` whenever the project's words move.** §5.3
(11:572-573); §7.4 lists task 12 as "Unchanged, and checked" (11:766-768).
- **The built rule.** When the marker names the right key, `p4 contextwrite/inspect.go:208-215`
  renders the block and compares. Anything not `SameText` is `StateEdited`, with a diff. The rule
  rests on "A key's render depends on nothing but the key" (`:32-35`). Its one exception,
  `PinErased` (`discoverhub/read.go:82-86`, `inspect.go:22-30`), exists because this same false
  `edited` was already found once in review (re-review b06-2 R2-I1).
- **What fails.** Under the design the render also depends on both sets. A teammate records a
  word, and every earlier block on the project now has the same key but other text: `edited`, with
  a diff that shows the new word as if a person had deleted it. Task 19 shows that as
  *hand-edited* (11a:443-445), and task 12's `materialisations` route reports it for every worktree
  of the branch (`task-12.md:18`, `:131`). §5.3 says "no new state" and maps only the two files.
- **Fix.** Compare the marker's two set digests before the text, and map a mismatch to
  `superseded`, since the block names another set. List `inspect.go`'s classification in §7.2, not
  only its marker parse. Take task 12 out of "Unchanged": its fresh render needs the two list reads.

**I3. The Hub refuses a contract edit that lands without its code, and the design lands A20
first.** §1 item 2.3 (11:103-105), 44a's A20 (11:756), the order (11:777-779).
- **The Hub's tests read the contract file.** `hubapi/keys_test.go:737-759` parses §3.1 and wants
  exactly ten prefixes there and in `keys.go`. `:530-551` parses §4.2's codes. The ledger names a
  third: "task 44a must run the hubapi tests after its contract edits and delete each
  pendingAmendment the body test names; the body test anchors on route lines verbatim"
  (`hub .superpowers/sdd/hub/progress.md:319`).
- **What fails at 44a.** 44a is "Documents only" (`task-44a.md:3-4`) and "IS STILL UNRUN and is out
  of order" (`progress.md:518-521`). With A20 in it, the Hub goes red the moment it lands, and
  stays red until Hub 45 and 48:
  - §3.1 holds twelve entities and `keys.go` ten;
  - §4.2 names `word_taken`, a constant no task names and Hub 48 is the first to need;
  - the body test, which by the ledger's account anchors on route lines, meets six routes whose
    bodies have no wire types yet (its source was not read, §4 item 3).
- **A second procedure.** Phase W already adds a Hub kind its own way: a proposal to the `feat/hub`
  owner (`work plan:167`, `:321`) and its task 02 appending `issue:` to `hubapi/keys.go` (`:235`).
  Its review cites the same ten-check (`phase-w review:369-376`). Two streams will each raise that
  count to their own number.
- **Not four tables.** The brief appears in twelve places in the contract (11b §1.9). The
  division-of-labour rule "It reads exactly one field of a revision's content: `status`"
  (HC:742-745) is contradicted by §2.4, where the Hub checks §2.2's field rules, case-folds words
  and runs a nine-op state machine. Task 25 builds the same field rules in `discover` (11:722), and
  nothing holds the two copies equal.
- **Fix.**
  - Land A20 with the code that turns each test green: Hub 45 for §3.1 and §3.2, Hub 48 for §4.2
    and §7.0. Or register pending amendments, as the body test's ratchet already does.
  - Agree the prefix count with W0.
  - Amend HC:742-745 in words.
  - Name one owner for §2.2's rules.
  - Under C1's default, most of this disappears.

**I4. The cost still leaves out built code, and Hub 48 is priced at half what the plan priced for
one kind with less in it.** §7.1-7.5 (11:683-792).
- **Built files and tests §7.2 does not list:**
  - **`Document`.** `Render(doc discover.Document)` (`render.go:76`) renders from a `Document`,
    "everything Render needs … produced by Resolve and by nothing else" (`discover/inherit.go:49-65`).
    For the block to hold the sets, either `discover` changes (built by task 05, with 12
    `inherit_test.go` tests), though §7.1 calls it untouched, or `Render`'s signature changes and
    the 33 "re-derive and run" tests do not compile unedited.
  - **The preview.** `discoverhub/read.go:738` renders a `Preview` block, a third render path.
  - **The create handler.** `api/sessions.go`, per I1.
  - **A call-count test.** `discoverhub/read_test.go:96-134` pins Resolve at two Hub calls and
    exactly zero list calls (one with evaluate). It cannot "run" (11:702) once the `Ref` path reads
    two lists.
  - **The Hub's contract-reading tests** (I3), and **task 12** (I2).
- **Hours against the plans' own prices** (Hub tasks 04 and 18-20 at `HP:405`, `:421-423`; phase 4
  at 11a §1):
  - **Hub 48, 4.5 h.** The brief's server side, for one kind whose content the Hub reads one field
    of, is tasks 18, 19 and 20: 9 h. Hub 48 prices six routes for two kinds, the write-time and
    cross-scope checks, the resolver, four events and the matrix rows.
  - **Hub 46, 3 h.** It must make the fake implement "every observable §7 rule" (HC:1352-1358),
    including nine ops and the case-folded word check. The brief's fake part 1 alone was 3 h.
  - **Phase 4's 31, 4 h.** It holds about the state set of tasks 18 and 19 together, which were 6 h.
  - **Phase 4's 27, 3.5 h.** It reads and writes two kinds. Tasks 07 and 08 were 6 h for one.
- **What that means.** Repricing these four, and adding I1, I2, I5's replay and the `Document`
  change, lands near **66-70 h before fix rounds** rather than 54 h, and **85-90 h** with the one
  round a task the ledgers show. That is a judgement from the plans' ratios, not a measurement.

**I5. Erasure keeps the text it exists to remove.** §2.6 (11:348-353), Hub 27's amendment (11:759).
- **Replays.** A17 (`HP:196`) exists because a stored reply holds text past an erasure. Route 26
  "stores no text", a replay rebuilds its reply from `brief_revisions`, and task 27's
  `TestErasureKeepsKeyAndDigest` scans every `text`, `jsonb` and `bytea` column. The two new batch
  routes return revisions with their fields, and no task gives them a text-free reply. An erased
  definition therefore survives 24 h in `idempotency_keys.response`, and for the whole life of that
  day's backup. The design's own reason for erasure is that "a definition can hold client text just
  as an answer can" (11:351).
- **Brief erasure.** "Erasing a brief changes no term … neither holds brief text" (11:352-353) is
  unsupported. The round is computed from "the answers holding a person's words" (11:430-431), and
  the model writes `is`, `context` and `why` from them. Hub 27's new test pins the hole as intended
  behaviour.
- **Fix.**
  - Give both routes route 26's text-free reply and rebuild, about 1.5 h on tasks 14's and 19's
    pattern.
  - Widen the scan test to term and decision text.
  - Have a brief's erasure list the terms and decisions whose `settledIn` names it: one query on the
    composite key (11:338-340).

**I6. Four transitions the design needs have no op that works, including the one 06 §5.1 was
about.** §2.3,
§2.4, §3.2 (11:255-289, :386-389).
- **A departure born `retired`.** "`depart` with `state: retired`" (11:156-157) is "a `create` with
  `scope: branch`" (11:288), and `create` is "nothing → `@1`, `in_force`" (11:282). A create
  followed by a retire needs the Hub-minted id inside the same batch, which an item cannot name.
- **A decision set aside.** "Sets it aside on this branch (retired)" (11:255-256) exists for words
  only. A decision is `in_force`, `superseded` or `withdrawn` (11:237), and `withdrawn` means "it
  should never have been recorded" (11:286). A branch saying *not here* to a project decision, which
  is 06 §5.1's own case, can still only replace it.
- **Undo of *Use the project's*.** That verb retires the branch's record (11:268-269), so Undo is
  `reinstate`, which re-runs the word check (11:287) against the project's in-force words
  (11:298-300). The Undo is refused `409 word_taken` inside its own five-second window.
- **Undo of `promote`.** No op returns a record to branch scope, and `reinstate` excludes
  `superseded`. "§2.4 has an op for every transition" (11:158) fails for what Undo writes.
- **Fix.** Let a departure be born `retired`, or `set_aside` for a decision. Make Undo a revision
  that restores the prior fields, as the brief's Undo writes a `restore` revision
  (`task-19.md:7-9`), exempt from the word check it already passed. Give `promote` an inverse or no
  Undo, and say which.

**I7. A branch's own decisions leave the thread when the branch lands.** §6.2 (11:650), §5.4.
- **The facts.** "Archiving a branch asks nothing" (11:650), and nothing adds a queue item
  (11:652-654). A branch record anchors to the branch brief (11:236), its archive "retires nothing"
  (11:246), and branch deletion archives the brief (HC:785-788).
- **What fails.** After a land, the branch's decisions are in force for a brief no session can
  start on, and in no project set. Main's code embodies them, but every later session on main is
  handed a set without them. That next agent is the *cold reader* of limb 2 (11:362-363) and may
  undo the choice. §5.4 says phase 3's merge does not change, so nothing shows them at convergence.
- **Fix.** At a land, the merge lens lists the branch's own decisions and departures with
  **Promote** as a verb beside each, not a prompt. Or record Q8 with a default and its cost.

### 2.3 Minor

**M1. `overrides` is written as a brief foreign key.** 11:338-340 makes `settledIn` and `overrides`
"composite foreign keys, `(org_id, brief_id, brief_revision)`". But `overrides` names "the project
entry this branch departs from" (11:238): a term or decision revision, not a brief. Built as
written, every departure is refused, or points at the wrong table. **Fix:** `overrides` references
`(org_id, term_id, revision)` in `term_revisions`, and its decision twin.

**M2. A batch item cannot name what its own batch creates, and item order is unstated.**
- `supersede` requires `replacedBy` "in this batch or earlier" (11:285), but the Hub mints ids at
  write (11:276-277), so an item cannot name a record created in the same call.
- `promote` of a departure also retires the project entry in the same batch (11:289). Applied in
  request order, the promote trips the design's own unique index: two in-force entries with one
  word in project scope. §2.4 names no order. The brief's route locks heads "in `briefId` order"
  (HC:808).
- **Fix:** allow a reference to an earlier item by its index, or take two writes. Apply state exits
  before entries.

**M3. The one-entry-per-word rule has three holes.**
- A departure is exempt from the word check (11:297-298), but `revise` may rename it (11:283). It
  can then take another project word, giving two entries for one word on that branch.
- No check compares a new word with the in-force terms' `avoid` lists. Rounds are per machine (Q5),
  so two machines can record *lane, not track* and *track, not lane*, and the block hands the agent
  both.
- "Case-folded" names no fold, yet the Hub's unique index and phase 4's overlay and sort
  (11:517-518) must agree.
- **Fix:** check a departure's word against the other project words; refuse a word that is an
  in-force term's `avoid`; name one fold, in one function.

**M4. No page rule for the list reads.** HC's lists default to 50 items and stop at 200 behind a
cursor (HC:48). §5.1 builds the overlay "from the two list reads" (11:484-485), and the digest
covers "every entry the branch sees" (11:537). Nothing says the Materialiser reads to the last page
before digesting, or that a budget expiry mid-list refuses the start. A default read digests the
first 50 and certifies them as the whole set, which is 06 §4.1's truncation one layer down.
**Fix:** one sentence in §5.1, and a task 27 test with 201 entries.

**M5. `why` holds what a choice gave up, but the block prints it as the reason.** §2.2 defines
`why` as "what it gave up" (11:231), filled from limb 3 (11:367-368). The block writes
`- <decision> Because: <why>` (11:503-504), while the Desk puts the reason after *because*
(`AGENTIC-UX.md:115-116`). An agent reads the cost as the reason, and no field holds why the chosen
option won. **Fix:** rename the field `gaveUp` and add `because`, or render it as *Gave up:*.

**M6. Task 26 cannot "start now".** §7.5 says "25, 26 and 28 can start now" (11:772-773), but §7.3
makes 26 depend on Hub 45 (11:723), which waits on 44a, itself unrun (`progress.md:518-521`). 26
must come before 12, which is on the critical path (`plan:430`). So the path also waits on
44a → 45, not only on task 32's +3 h. **Fix:** restate 26's start and the critical-path claim.

**M7. Words recorded while the brief is open reach no session.** §2.5 lets words settle in an open
revision (11:329-330). §5.1 gives `""` nothing, reasoning that `""` "means no brief existed"
(11:492-494). But `""` also means a brief with no complete revision: `Inheritable` ends "else none"
(`read.go:77-80`). For the whole drafting period the lens shows words as recorded and no agent
receives them. **Fix:** say *reaches sessions once the brief is complete* in the lens, or inherit
the project's set whenever the project exists. Correct the reasoning either way.

**M8. Four gaps an implementer would have to guess at.**
- **The resolver's field.** `/v1/refs` answers `kind: "term"`, but a result carries only `revision`
  (a `BriefRevision`) and `approval` (HC:822-828). Name the member that holds a term or decision.
- **Refreshing the digests.** The set digests move on `term.*` and `decision.*` events from another
  scope. The `Ref` is cached "for ever EXCEPT" its heads, per brief (`ref.go:70-72`), and the
  desktop invalidates by `brief.*` (HC:1382-1385). Task 16's amendment (11:751) must say the slice
  refreshes them per project.
- **A second model call.** *Record a decision* makes the assistant draft `context`, `why` and `undo`
  (11:376). That call has no budget, no outcomes and no task.
- **Matching and filtering rounds.** §4.3 drops a pending decision "now recorded" (11:458), but a
  decision has no key to match on: name the rule. The round's model input also omits the sets in
  force (11:429-431). Filter proposals against them before showing, or a Record meets a 409 the
  control plane could have avoided.

## 3. What held up

- **The central move is right, and it is OD-36's.** Terms and decisions are their own Hub records
  beside the brief. `discover.Content`, its inline golden, every stored digest and the brief's key
  do not change by a byte, so the 64 plan files that quote `brief:bf-` do not move (11:685-689).
  Nothing in §2 to §6 writes brief content.
- **The ceremony finding is fixed for real.** The three-limb test is the agent's. The person gets
  one gesture, or **Record all**, with an optimistic Undo; there is no checkbox, gate field, count,
  badge or queue item (§3.2, §6.2). I6 names the only two Undos that fail.
- **The truncation finding is fixed for real.** The whole set is on disk before the harness starts.
  The block always names the two files, and every digest covers the whole set, not the inline part
  (11:569-573). M4 is the one place it can still slip.
- **Rounds are process state.** They are in neither `Content` nor the Hub, and have no closed state
  to reopen (11:448-450, :463-466).
- **The plans' citations are protected.** New tasks go in a new section at each plan's end, and no
  task-table row is inserted (11:712-715).
- **The citations are honest.** About 45 of them were checked against the tree (commits named in
  the header), and each points at what it claims. The issues above are what the design leaves out,
  not what it misquotes. One source it leans on is looser than its code: `CONTEXT.md:131` glosses
  `""` as "no brief existed", where `Inheritable` means "nothing complete" (M7).

## 4. What this review did not verify

1. **Nothing was compiled or run.** Every test consequence above is read from source. Two examples:
   - `TestResolveBatchesInTwoCallsAtMost` fails if the fake's `lists()` counts the new reads. If it
     does not count them, the test passes and stops meaning its bound. Either way, someone must
     decide.
   - Whether the Hub's contract-reading tests go red at 44a follows from `keys_test.go:737-759`,
     and was not run.
2. **I4's hours are a judgement.** They come from the plans' own ratios, not from time spent.
3. **The Hub's body test and its `pendingAmendment` ratchet** are known from the ledger
   (`progress.md:316-319`), not from their source.
4. **Built phase 5 and phase 7 code was not re-read.** §5.4's *unchanged* rows are taken from 11b
   §4. In particular, whether any built consumer switches exhaustively on `/v1/refs`'s `kind` was
   not checked. HC:32-35's additive rule does not list a new enum value.
5. **The Hub worktree's three uncommitted files** (`feed.go`, `schema_integration_test.go`,
   `task-07a.md`) were not read.
6. **The count of 517 inbound citations is a floor.** It matches `hub-contract\.md:[0-9]+` only;
   `HC:` and `§`-only citations are not counted.
7. **The source was not rechecked.** aihero.dev was not fetched and its `LICENSE` is still unread.
   Nothing here copies it.
8. **This review argues Q2's default, and does not decide Q2.** What the owner meant in OD-36 is
   the owner's to say.

## Closing

Complete on 2026-09-29: sections 0 to 4 are written and committed, and the working notes are
deleted. This file reviews `11-design-r2.md` at `bd9b6299`, and nothing else was edited. Of the
design's seven owner questions, only Q2 is argued with here (C1). The rest stand as written, with
Q8 proposed by I7 if the owner wants a say on branch decisions at a land.

**Design quality: Needs fixes**
