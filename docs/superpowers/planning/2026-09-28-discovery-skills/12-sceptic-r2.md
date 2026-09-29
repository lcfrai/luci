# 12 — Sceptic, round 2: why design 11 would fail

**Status: in progress, 2026-09-29.** Reviews `11-design-r2.md` at `bd9b6299`. Read-only on every
file except this one. A relaunch that finds this file without its closing verdict line continues
from the first section still marked *pending*.

Method: `06-sceptic.md` first, then `11-design-r2.md` in ranges, then every citation the design
leans on checked against the tree (main tree at `feat/phase-2-cockpit`, the `phase-4` and `hub`
worktrees read-only). Line numbers below are the tree as read on 2026-09-29.

## 0. The verdict in one screen

*pending*

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
| §2.1 *(structural)* | built code priced as paper | **In part.** §7.2 now names built files, but misses five (I4) and the Hub hours sit at a third of what the brief's own Hub tasks were priced at (I4) |
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
  earlier reorders phase 2's handler, which §7.1 says does not change. Five exits between `:963`
  and `:1037` abort a create after Materialise: the conversation store, line endings, git files,
  worktree admin and the record. Each would leave an inheritance entry for a session that never
  existed, which is exactly the stamp the handler promises not to leave. An id collision
  (`:1070-1079`) would append a failed start to another session's "immutable" record.
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
  - §4.2 names `word_taken`, and no §7.3 task adds its constant;
  - the body test meets six routes that have no wire types.
- **A second procedure.** Phase W already adds a Hub kind its own way: a proposal to the `feat/hub`
  owner (`work plan:167`, `:321`) and its task 02 appending `issue:` to `hubapi/keys.go` (`:235`).
  Its review cites the same ten-check (`phase-w review:370-375`). Two streams will each raise that
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

### 2.3 Minor

*pending*

## 3. What held up

*pending*

## 4. What this review did not verify

*pending*

## Working notes (deleted before the verdict)

- Skeleton written first, per the watchdog rule.
- 06 read in full (16 findings in §1-5, plus §6 IP and §7 shelf). 11 read to line 355.
- Leads to verify: `overrides` FK is written as a brief FK (11:338-340) though it names a term or
  decision key (11:238); `depart` born `retired` vs `create` = `nothing -> @1, in_force` (11:282,
  288, 157); a decision departure has no "set aside" state (11:237 vs 255-256); a departure's word
  is never checked against *other* project words (11:297-300); `revise` re-sending an erased
  `settledIn` gets 422 (11:293, 240); HC's rule for adding a kind; HC:329, 557-607, 798-815,
  1115-1134; `owner-decisions.md:409`; `PC:104-107`.
- Read to 11:556. More leads: Undo of *Use the project's* is a `reinstate`, which re-runs the word
  check against the now in-force project word: 409 inside the Undo window (11:268-269, 287,
  298-300, 386-389); Undo of `promote` has no op (no demote; `reinstate` excludes `superseded`,
  11:287-289); the inheritance record is local while HC has session links (HC:937-944) and OD-3
  puts the provenance ledger in the Hub; `Ref` "cacheable on the same terms" (11:533) now depends
  on another scope's records; round model call sees no existing terms (11:429-431).
- 11 read in full. Cost leads: §7.5 says 26 "can start now" but §7.3 gives 26 a Hub 45 dependency
  and a *before 12* edge (12 is on the critical path); Hub 48 prices six routes + checks + resolver
  + events + matrix at 4.5 h, compare the brief's own Hub tasks; `MaterialiseRequest.SessionID`
  caller (phase 2 or 14?); `ref-golden.json` consumed by TS too?; 14-day sweeper unpriced?
- OD-36 row (`owner-decisions.md:409`) and note (`10:54-55`): "What does not move: ... the
  provenance key grammar, which the Hub contract owns, two parsers compile". The note's own sketch
  uses `term:tm-7f2a@2` (10:14), so the owner's record is itself two-sided. HC:116 says phase 4 owns
  the grammar, OD-36 says the Hub contract does: Q1 is a real conflict.
- HC:32-35 lists what is additive: new optional request field, response field, route, error code,
  event kind. Not a new key form, not a new `kind` value in `/v1/refs`. HC:742-745: the Hub "reads
  exactly one field of a revision's content: `status`"; 11 §2.4 has the Hub parse every term field,
  case-fold words and run a state machine. HC:838-840: `heads` is `/v1/refs`'s only time-varying
  part and the slice caches results for ever; 11's set digests are not in `/v1/refs` at all.
- VERIFIED `p4 inspect.go:202-215`: same marker key -> render -> not `SameText` -> `StateEdited`
  with a diff. Its comment `:32-35` rests on "A key's render depends on nothing but the key". Under
  11 the render also depends on the sets, so a teammate recording a word turns every stale block on
  the project `edited`, a hand-edit claim nobody made. 11 §5.3 says "no new state" and maps only the
  two files. -> Important.
- 11a §1: phase 4 hours: 07 reads 3 h, 08 writes 3 h, 09 Materialiser 3 h (then a fix round with 2
  security Importants: symlinks, fsmonitor), 18 lens 3 h, 19 states 3 h. 11a:338-344: no inline cap
  exists in the block today.
- VERIFIED `p4 discoverhub/read_test.go:96-134` `TestResolveBatchesInTwoCallsAtMost`: Resolve <= 2
  Hub calls and exactly 0 list calls (1 with evaluate). 11 §7.2 says the Ref path "reads the two
  lists" and all 11 Ref-path tests *run*: this one cannot, or its bound silently stops meaning it.
- 11b §1.9: "The contract has no procedure for adding a record kind"; the brief appears in 12
  places (prefix, key form, envelope, vectors, routes, objects, resolver slot, events, matrix row,
  hubclient, desktop invalidation, task acceptance). 11 §1 2.3 claims in-place edits "confined to
  its four closed tables". HC:1382-1385 desktop invalidates by prefix `brief.*`; HC:1348-1350 the
  client re-computes every revision digest; HC:170-174 no TS canonical.
- ADR-H1:52-57: OD-3 moved the ledger off the laptop: "A laptop cannot be that source of truth."
  HC:931-944 session links already put a session's stamped `contextRef` in the Hub (design only).
  But `contextRef` itself is local (CONTEXT.md:130-136), so the record has contextRef's durability:
  weaker than first thought. Keep as Minor/Important, not Critical.
- THREAD, strong: 11 §5.1 "What the branch sees is phase 4's overlay" vs §5.2 "a project key: the
  project's" set, and "two sessions with one contextRef still share one Ref". Inheritable falls back
  to the latest complete PROJECT revision (11a:301-305), so a branch with branch records and no
  complete branch revision starts under a project key: either the record never equals the Ref
  (permanent false "words moved") or the branch's departures are dropped. -> Important, maybe
  Critical.
- THREAD: words settled while the brief is open (11 §2.5 allows it) reach no session while
  `contextRef` is `""` (no complete revision): the whole drafting period.
- THREAD: branch-only decisions at merge: "archiving a branch asks nothing" (11:650), no promote
  path at convergence, so a decision shipped from a branch leaves every later session on main.
- Pagination: list reads are `?limit` max 200 (HC:48); 11 never says the Materialiser pages to the
  end before digesting. A default-50 read digests a truncated set.
- VERIFIED `p4 api/sessions.go:963` Materialise runs BEFORE `:1005` `sessionID := ids.New("s")`
  ("3. Ids and the secret"); `:972-973` and `:983-985` say a failure after Materialise leaves only
  the block, which "stamps nothing". 11 §5.2 puts `SessionID` in `MaterialiseRequest` and writes
  the inheritance record in the Materialiser's call: on create there is no id yet, and 5 failure
  exits between :963 and :1037 would leave a stamp for a session that never existed. Fix needs
  `MaterialiseResult` or the handler to change, which 11 §5.2/§7.1 say do not. -> Important.
- VERIFIED `Render(doc discover.Document)` (`render.go:76`), `Document` in built `discover/inherit.go:51-65`
  holds no sets; `discoverhub/read.go:738` renders a `Preview` block from the doc. Neither is in
  11 §7.2; §7.1 says `discover` is untouched.
- p4 task 12's `materialisations` route (task-12.md:18, :131) is the drift check over every worktree
  of a branch: it inherits the false `edited`, and must render with the sets; 11 §7.4 lists 12 as
  "Unchanged, and checked".
- HP task table (`HP:398-446`): the brief's server side is 18 (3) + 19 (3.5) + 20 (2.5) + part of
  27 (2.5), about 9-11 h for ONE kind whose content the Hub reads one field of; 0001 SQL was 07a+07b
  = 6.5 h and its fix round is still open. 11 prices TWO kinds, with a state machine and word checks
  in the Hub, at Hub 48 = 4.5 h and Hub 47 = 3.5 h. The fake must mirror every §7 rule (HC:1352-1358),
  so Hub 46 (3 h) re-implements word_taken, the op table and erasure in the fake.
- VERIFIED A17 (`HP:196`): route 26 "stores no text" in `idempotency_keys.response`; replays rebuild
  from `brief_revisions`; task 27's `TestErasureKeepsKeyAndDigest` scans every text/jsonb/bytea
  column. 11's term and decision batch routes carry text and 11 §2.6/Hub 48 never say the replay is
  text-free: an erased definition survives 24 h in replies and for ever in that day's backup.
- VERIFIED hub `hubapi/keys_test.go:737-759` `TestPrefixesAndReasonsAreTheContracts` parses the
  CONTRACT's §3.1 table and wants exactly ten prefixes in both; `:530-551`
  `TestHubCodesAreContractSection42s` parses §4.2. So A20 in 44a ("documents only", first) turns the
  Hub red until 45 lands, and no task adds the `word_taken` code constant. 11 §7.2 lists only
  "keys_test.go's 4 gain rows".
- Phase W already has a procedure for adding a Hub kind: a proposal to the feat/hub owner
  (`work plan:167`, `:321` w0-hub-contract-proposal.md) and its own task 02 appending `issue:` to
  `hubapi/keys.go` (`:235`) with an `issue-revision` envelope and three vectors. Two streams will
  both edit the ten-check and §3.1; 11 does not coordinate with W0.
- Citations checked and correct: content.go:57-59, :63-65; render.go:81-82, :262-267;
  materialise.go:1-21, :81-87, :110-113, :159, :169-191, :196; HC:325-331, :578-590, :603-610,
  :1115-1127; PC:182 (phase-3 tree); ref.go:70-77; task-16.md:165-167; task-19.md:7-11;
  AGENTIC-UX.md:109-119, :145-147, :237-246; DESIGN-SYSTEM.md:63, :69, :90; CONTEXT.md:204-211,
  :303; HP:386-394; T:20.md:28-36; T:27.md:38-42; 04-internal-fit.md:45.
- `ref.go:71-72`: "Cacheable for ever EXCEPT headRevision and latestCompleteRevision, which the
  desktop keeps per brief". Set digests move on another scope's events (`term.*`), not `brief.*`.
- `/v1/refs` result has `revision` (BriefRevision) and `approval` (DesignApproval) (HC:822-828);
  11 never says which member carries a term or decision revision.
- Brief erasure: 11 §2.6 "neither holds brief text", yet the round is computed from the answers
  (11 §4.1) and the model writes `is`/`context` from them.
