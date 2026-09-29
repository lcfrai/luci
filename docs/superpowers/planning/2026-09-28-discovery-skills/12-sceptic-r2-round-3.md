# 12 — Sceptic, round 3: are R2-I1 and R2-I2 closed at the root, and do the restatements agree?

**Status: complete, 2026-09-29.** Scoped round 3. Reviews `11-design-r2.md` at `a2c1de78`
(Revision 2, commits `8ba9b802` … `a2c1de78`) against `12-sceptic-r2-round-2.md` at `2726d918`, and
the owner questions filed in `2026-09-25-owner-questions-pending.md`. Read-only on every file except
this one.

## 0. The verdict in one screen

**Needs fixes: 0 Critical, 1 Important, 6 Minor.** Revision 2 closes both of round 2's Importants
as asked, and all nine Minors. Its restatements agree: §0, §7.3-7.5 and §R2.4 carry the same
76.75 h (45.25 + 31 + 0.5) and the same +28.5 h before task 14. Q2's *about 1 h more*, Q8's cost and
Q10 read the same in §8.2, §R2.1, §R2.4 and the owner-questions file. Every edit above the
*Revision 2* heading replaces a line with a line, so no cited line moved (§1).

One thing still makes the product wrong. It is new, and two of the fixes make it together:

1. **R3-I1, erasure does not reach a departure.** Important: an erasure leak, plus a false `edited`
   that shows the erased text.
   - A *not here* now holds a verbatim copy of the project entry it sets aside. That is M5, and
     round 2 asked for the copy.
   - An in-force departure's origin line prints its pin (R2.1).
   - Erasure is per record and reaches neither. So after an org admin erases a client's codename
     from the project, every branch that set it aside still hands it to each new session. A branch
     that redefined it reads `edited`, and the diff shows the erased text.
   - Phase 4 already solved the second case for the brief's own pin: `PinErased`, task 07's rule 7.

   **Fix:** erase a *not here* with the entry it copies, list the other departures in the erase
   reply, and apply rule 7 to the sets' pins. About 1.75 h.

The Minors, most useful first:

| # | Issue | Where |
|---|---|---|
| R3-M1 | The set the block prints is not the view `Overlay` returns: `set_aside` records are in the first and not the second | §R2.3 M4, §5.1 |
| R3-M2 | *Project changed* compares "fields", and §2.2's fields include `author`, `sessionId` and `settledIn` | §6.2, §R2.3 M6 |
| R3-M3 | Task 26 "can start now", but Q2 is asked before it starts | §7.3:881, §7.5:941 |
| R3-M4 | The lists hold every head, so "a set of up to 200 entries is one page" is really about heads. The branch filter is unstated | §2.4:378-380, §R2.3 M9d |
| R3-M5 | `reinstate` of an undone *not here* puts a copy of the project's entry in force | §2.4:327 |
| R3-M6 | Q10's *Holds up: Hub task 47* reads as a block, though §0 says the nine block nothing | owner questions `:162` |

**Next:** one short fix round on R3-I1, then a re-read of that fix alone. The Minors are wording that
a plan edit can carry, and none needs another round.

## 1. Every prior finding: closed or open

Method: round 2's file first, then `11-design-r2.md` at `a2c1de78` in ranges, then
`git diff -U0 2726d918 HEAD` on 11, then the citations below against the tree. `11:N` is 11 at
`a2c1de78`. `p4` is `.claude/worktrees/phase-4/control-plane/internal/` at `ca737d92`. `HC` is the Hub
contract at `cafe1303`. `OQ:N` is `2026-09-25-owner-questions-pending.md`.

| Finding | Verdict | Evidence |
|---|---|---|
| **R2-I1**, no read of a past revision | **CLOSED at the root** | The lists are heads, ordered by id (11:378-380). §18 gains one revisions read per kind (11:1122-1134), and the Materialiser, the lens and the loader all use it (11:1137-1148). Origin lines print the pin (11:619). Priced at 2.75 h (11:1156-1159). One case it does not reach, an erased pin, is new: R3-I1(b) |
| **R2-I2**, the head keeps the folded word | **CLOSED as named** | `hub.erase_term` nulls the word under a `CHECK` and the guard (11:425, :1176-1180), and Hub 47 tests it. 48b scans for `FoldWord(marker)` too, and must find the head's copy before erasure (11:434, :1191-1195). But "no copy of a word outlives its erasure" (11:1175) is not yet true of other records: R3-I1(a) |
| R2-M1, critical path | CLOSED | +28.5 h appears in §0 (11:17-19), §7.5 (11:936-943) and §R2.3. The chain sums to 33.5 h, and each link matches §7.3's dependencies (11:880-887, :896-900). One wording point remains: R3-M3 |
| R2-M2, an exit before its create | CLOSED | Ids are minted first, the passes are total, the keys are deferred, the fake checks at the batch's end, and a test runs both orders (11:316-318, :897-898, :1224-1234) |
| R2-M3, **Record all** | CLOSED | Two batches, words first; what shows when only one succeeds; one Undo for both (11:479-482, :1237-1245) |
| R2-M4, the overlay and the hole | CLOSED | `hubapi.Overlay` is the one copy, and the shadow covers an avoided word both ways (11:296-297, :1249-1262). One point remains: R3-M1 |
| R2-M5, *not here* | CLOSED as asked; **this round withdraws the copy** | Fields, `set_aside` for both kinds, the inverse and **Not on this branch** are all in (11:261, :281, :324-328, :794). The copy round 2 asked for causes R3-I1(a) |
| R2-M6, a promote's Undo | CLOSED | Compared by fields and state, never by number (11:787-788, :1285-1289). One point remains: R3-M2 |
| R2-M7, Q8's cost; landed or not | CLOSED | The cost is in Q8's row (11:995) and at `OQ:131-134`, and the lens says it cannot tell (11:799-800). A land is one lane into its branch (`CONTEXT.md:62-67`), so nothing records a branch reaching main |
| R2-M8, the contract edit | CLOSED | Markers go on §3.1, §4.2, §7.0 and §10.2, never §8 (11:920, :1303-1308). `settled` stays on `HC:1126` (11:926). Both envelopes are listed member by member, with six vectors (11:1311-1325) |
| R2-M9 (a)-(g) | CLOSED | (a) 11:705, :859. (b) :518. (c) :360, :1337-1341. (d) :594, :1342-1349. (e) :753, :802. (f) :887, :912, :1351-1361. (g) Q10, :1370. (d) is sound: the feed is the org's audit sequence, "gapless, in commit order", and each event is written under the org's audit lock (`HC:1098-1107`), so an equal position means no write landed between the pages |
| Round 1's C1, C2, I1, I3, I6, I7 | CLOSED | As round 2 found (its §1), and nothing in Revision 2 re-opens them |
| Round 1's I4, cost | CLOSED | It closes in full now: M9a, the revisions read and the path are all priced (11:1375-1387) |
| Round 1's I2, false `edited` | CLOSED for a moved set | It comes back by one new route, an erased pin: R3-I1(b) |
| Round 1's I5, erasure keeps text | CLOSED for replies and for a brief's erasure | It comes back by one new route, a *not here*'s copy: R3-I1(a) |

**The restatements agree.**
- **Hours.** Phase 4's new tasks sum to 39 (11:879-889) and its amended ones to 6.25 (:913-919,
  :928). The Hub's sum to 26 (:896-902) and 5 (:920-928). So 45.25 + 31 + 0.5 = 76.75, the same in
  §0, §7.5's table (:950-955) and §R2.4 (:1387). Each task's change in §R2.4 matches its reasons
  (:1377-1386), and 45.25 / 57 and 31 / 133.5 are the 79% and 23% of :957-958.
- **Q2.** Key forms cost about 1 h more: the same at 11:989, :1161-1165 and `OQ:119-123`. The file's
  "about 2 h" is the Hub's and the Materialiser's part of R2.1's 2.75 h, which is what key forms
  would save.
- **Q10.** The same default, cost and deadline at 11:1370 and `OQ:152-163`. Filing it was right:
  round 2's M9g said the reading is the owner's.
- **No cited line moved.** `git diff -U0 2726d918 HEAD` on 11 shows every hunk above line 1100 as
  `-a,b +a,b`, and one insertion, `+1100,327`, which is Revision 2.

## 2. Issues

### 2.1 Critical

None. The struct, the stamp rule and §18 hold, and nothing below stops a plan edit.

### 2.2 Important

**R3-I1. Erasing a project entry reaches no departure from it.** This is an erasure leak, and a false
`edited` that shows the erased text. It is new: R2.1's pins and M5's copy make it together. Where:
§2.3:280-281, §2.4:328, §2.6:421-425, §5.1:616-622, §R2.1, §R2.2.
- **Why the departures survive.** A departure is "its own record" (11:275-276). Erasure is per
  record: `POST /v1/terms/{termId}/erase`, and `hub.erase_term` empties that record and nulls its
  own head's folded word (11:422-425, :1176-1180). The only erasure check on `overrides` runs at
  write time (11:342). No text says that a later erasure of the pinned entry reaches the records
  that pin it.
- **(a) A *not here* keeps a verbatim copy.** M5 gives it "`@n`'s fields, copied" (11:281, :328,
  :1268): a term's `word`, `is`, `avoid` and `because`, or a decision's five text fields. Round 2
  asked for that copy, and it was wrong to. Take a project term *Falcon*, a client's codename, set
  aside on branch X, which the org admin then erases:
  - X's departure still holds the word and its definition, and its head still holds `falcon`.
  - The lists and the revisions read serve them to every `V` caller.
  - Every new session on X is handed them again: the block's `- _Not on this branch: **Falcon**_`
    (11:622; a decision prints its text, :618) and `terms.md`, which holds "every entry of the
    session's set, in full" (11:683-685). These are fresh files at each start, not only the stale
    file that plan OQ-9 accepts for the brief (p4 `contextwrite/inspect.go:28-29`).
  - The erase reply names none of it, though a brief's erasure names what settled in it
    (`settled`, 11:435-441).
  - The scan cannot catch it: 48b's term is erased alone (11:431-434, :1191-1195). So "no copy of
    a word outlives its erasure" (11:1175) is asserted but not tested.
- **(b) An erased pin gives a false `edited`.** Origin lines print the pin (11:619), and an erased
  revision reads with its text `null` (11:1126-1127). No rule says what the Materialiser prints then.
  The overlay subtracts the overridden entry from the branch's set (11:296-297), so its erasure
  moves neither set digest. `Inspect` therefore passes the set check (11:704-708), renders, and finds
  the text changed. It calls the block `edited`, with `DiffLines(render, text)` showing the erased
  definition as a person's edit. R2.1's reason the digests hold, "a revision is immutable"
  (11:1150-1151), fails at the one mutation the Hub allows.
  - Phase 4 has already solved exactly this for the brief's own pin. `Inheritable.PinErased`
    renders the branch's text alone (task 07's rule 7). `Inspect` reads `superseded` with an empty
    Diff, "so the erased text is not shown" (p4 `contextwrite/inspect.go:22-29`, `:212-213`;
    `discoverhub/read.go:82-86`, `:561-566`). The sets' pins get no such rule, and the lens's
    *Project: <definition>* (11:786) has the same gap.
- **Fix.**
  1. **Erase what was copied.** `hub.erase_term` and `hub.erase_decision` also erase every
     `set_aside` record whose `overrides` pins a revision of the erased record, in the same owner
     function, with one `*.erased` event each. Its fields are that record's text by construction
     (11:281), so this erases nothing a person did not choose (11:441).
  2. **Name the rest.** The erase reply gains `departures`: the struct of every other record whose
     `overrides` pins the erased one, as a brief's reply gains `settled`. An in-force departure is
     the branch's own text, but its `word` is usually the same codename, so the admin chooses. It is
     a `pendingAmendment` until 44b, as `settled` is.
  3. **Apply rule 7 to the sets' pins.** `Inheritable` carries `PinErased` per pin (M9a already
     carries the pins). A departure with an erased pin prints its own text without an origin line,
     and `Inspect` reads `superseded` with an empty Diff. The lens shows *the project's entry was
     erased* in place of *Project: …*.
  4. **Tests.** Give 48b's marked term a *not here* and an in-force departure. After the term's
     erasure, the scan finds the marker only in the in-force departure the reply names, and after
     that one's erasure, nowhere. 29's `Inspect` test adds an erased pin, which reads `superseded`
     with an empty Diff.
  - **Price: about 1.75 h.** Hub 47 +0.5 and 48b +0.25; phase 4's 27, 28, 29 and 31 +0.25 each.
    §0, §7.3-7.5 and §R2.4 then carry about 78.5 h.
  - **Or take the copy back out.** A `set_aside` revision holds no text, §2.2's rules exempt it,
    and its line prints the pin, as an origin line does. Rule 7 then covers it too. That changes
    M5's field rule and R2.2's `CHECK` (a `set_aside` head holds no folded word), for about the same
    hours.

### 2.3 Minor

**R3-M1. The set the block prints is not the view `Overlay` returns.** `hubapi.Overlay` returns "the
project's in-force entries, minus each one an `in_force` or `set_aside` branch record overrides …,
plus the branch's own `in_force` records" (11:1250-1251). The set is the overlay (11:580, :644), and
task 25 builds it with `Overlay` (11:880). But the block prints *not here* lines (11:618, :622), and
M5 relies on "the departure's own digest cover[ing] the text its block line prints" (11:1268-1269).
That digest is in the set only if `set_aside` records are, and rule 3's view must leave them out
(11:1269-1270).
- **Fix:** say the set is `Overlay`'s view plus the branch's `set_aside` records, each an item of
  the set digest, and that rule 3 reads the view alone.

**R3-M2. *Project changed* compares "fields", and that word is wider than M6 needs.** It shows when
the head differs from the pin "in its fields or its state" (11:787-788, :1285). §2.2's fields
include `settledIn`, `author` and `sessionId` (11:264-266). The revision envelope also has
`restores` (11:1320). A promote's Undo is a `restore` written by the person who pressed Undo, so
compared member by member it differs from `@k`, and the false *project changed* comes back.
- **Fix:** name what is compared: the text fields of §2.2's first two tables, plus `state`.

**R3-M3. Task 26 "can start now", but Q2 is asked before it starts.** Task 26's row depends on 06
alone, with "no Hub task in front of it" (11:881). It "can start now" (11:941, :1215). Yet Q2 "must
be asked before 44a, Hub 45 or phase 4's 26 starts" (11:58-59, :1164-1165, `OQ:109-110`).
§7.5:946 says only "before 44a runs".
- **Fix:** add *Q2* to 26's *Depends on* cell, and say "before 44a, Hub 45 or 26" at 11:946.

**R3-M4. The lists hold every head, so M9d's one-page case is about heads.** The lists return
"heads only", in every state, "filterable to one branch brief" (11:378-380). "A set of up to 200
entries is one page" (11:1346) is therefore about 200 heads: retired, superseded and withdrawn ones,
and every branch's unless filtered. The filter is not described. A branch start needs the project's
heads and one branch's.
- **Fix:** say whether `?branchBriefId=` returns the project's heads with that branch's, and state
  the one-page case in heads.

**R3-M5. `reinstate` of an undone *not here* puts it in force.** An undone *not here* is `retired`
or `withdrawn` with `overrides` set (11:324, :326). `reinstate` takes either to `in_force` (11:327).
The result is an in-force departure that copies the project's entry. Under R3-I1's other fix, which
takes the copy out, it becomes an entry with no text, which §2.2 refuses.
- **Fix:** a record whose last state before its exit was `set_aside` reinstates to `set_aside`.
  Or say that the lens does not offer **Reinstate** on it.

**R3-M6. Q10's *Holds up* line reads as a block.** `OQ:162` says "Holds up: Hub task 47". That file
defines *Holds up* as "what stays open, unwritten or unverified until it is answered" (`OQ:6-10`).
But the design says the nine block nothing (11:61-62), and Q10's own row says the other answer after
47 is "one more migration" (11:1370).
- **Fix:** use Q9's shape: "Holds up: nothing built. Hub 47 builds the default; after it ships, the
  other answer is one more migration." This is the reviser's file.

## 3. What held up

- **Both Importants are closed where they started.** R2.1 adds one read, not a key form, and every
  part that prints or compares a past revision now names that read: the Materialiser, the lens and
  the loader (11:1137-1148). R2.2 clears the copy in the statement that erases. It also makes the
  scan prove it can see the fold before proving the fold is gone (11:1193-1195), the way the
  brief's case proves it can see a `bytea` column.
- **Rule 4 held.** Every edit above *Revision 2* replaces a line with a line (§1). Each in-place
  pointer names the §R2 section that holds the longer text.
- **The hours add up** wherever they are stated (§1). The critical path comes with its condition.
  M1's two dependency cuts follow the plan's own convention: task 15 codes against 11's and 12's
  contracts, not their code (`plan:406`).
- **The three designs the reviser marked untested are sound against the tree.**
  - `feedSeq`: the feed is gapless and in commit order, under the org's audit lock
    (`HC:1098-1107`).
  - The deferred keys: `replacedBy` and `overrides` point at primary keys, which stay immediate,
    and the unique index stays immediate, as M2 says.
  - The heading markers: `contractSection` finds a section by the prefix `"\n### " + number + " "`
    (hub `hubapi/keys_test.go:371`), so a marker after the title still finds §3.1, §4.2 and §10.2.
- **Each departure from round 2 is argued.**
  - `set_aside` for terms closes a hole that round 2 missed.
  - Declining the early Hub commit (it saves half an hour) and the re-pin (it writes a revision for
    a change nobody made) are both reasoned.
  - A lens that says it cannot tell a landed branch from an abandoned one is truer than a label
    with nothing behind it.
- **No ceremony was added.** **Not on this branch** is a verb, and the lens's new line states a
  fact.

## 4. What this review did not verify

1. **Nothing was compiled or run.** R3-I1(b) is read from the design and from phase 4's built
   `Inspect` (p4 `contextwrite/inspect.go:198-215`). It depends on the branch's set not naming the
   overridden entry, as 11:296-297 and :1250-1251 say.
2. **The marked heading lines themselves.** This review read `contractSection`, but not the loops
   that walk each section's lines (`keys_test.go:537`, `:740`, `:779`; `envelope_test.go:518`). So
   it has not checked that none of them trips on a heading that carries a marker. 44a's run is the
   proof, as the reviser says.
3. **R3-I1's 1.75 h is this review's estimate**, at the plans' ratios.
4. **Phase 2's final gate is running in the main tree.** The phase 4 lines cited are `ca737d92`'s.
   This review staged and committed only its own file.
5. **Q2, Q8, Q9 and Q10 are the owner's.** This review argues only with Q10's *Holds up* wording
   (R3-M6).

## Closing

Complete on 2026-09-29. Sections 0 to 4 are written and committed, and the working notes are deleted.
This file reviews `11-design-r2.md` and the owner questions at `a2c1de78`, and no other file was
edited. The one Important, R3-I1, needs about 1.75 h of design and one short re-read. The six Minors
can go into the plan edit. Q2 stays the first owner question, to be asked before Hub 44a, Hub 45 or
phase 4's 26.

**Design quality: Needs fixes**
