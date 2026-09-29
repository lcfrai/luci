# 12 — Sceptic, round 3: are R2-I1 and R2-I2 closed at the root, and do the restatements agree?

**Status: in progress, 2026-09-29.** Scoped round 3. Reviews `11-design-r2.md` at `a2c1de78`
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

<!-- Working notes (replaced as the review lands):
- Round 2 read, lines 1-140: R2-I1 = no read of a past revision (fix: lists return heads, one
  revisions read per kind in §18, Materialiser reads pinned revisions, ~2 h across Hub 46, 48b and
  phase 4's 27; Q2's "about 3 h more" shrinks to about 1 h). R2-I2 = head keeps folded word (fix:
  erase_term nulls it, task 47's DB test checks it, task 27's scan searches FoldWord(marker) and
  puts the marker in `word`, ~0.25 h in Hub 47 and 48b).
- Round 2 read, lines 140-331: M1 path (Hub 44a 1.5+2, 45 3.5, p4 25 2.5, 28 4.5, 27 6, 29 5.5,
  32 3.5 = ~29 h vs ~5 h; fix: state ~+24 h or early Hub commit). M2 exit before create (mint ids
  first or DEFERRABLE). M3 Record all = two batches. M4 overlay built three times + cross-scope
  hole. M5 not-here needs fields + verb. M6 promote Undo false project changed. M7 Q8 cost +
  landed/abandoned label. M8 heading markers (not §8), `settled` on HC:1126 line, set-envelope
  vectors + scope value, revision envelope members. M9 a-g (Inheritable/Source, one-call limit,
  FoldWord vs lower(), paged read snapshot, "merge" -> land, task 12's four routes, settledIn null Q).
- 11 R2.1 (1108-1165) read: lists = heads ordered by id; new GET .../term-revisions?id= and
  .../decision-revisions?id= (1-100 ids, every revision, paged, role V); Materialiser (27) reads
  pins for a branch with a departure, pins travel in Inheritable; lens via 2 relays in 32; loader
  16. Price 2.75 h (46 +0.5, 48b +0.75, 27 +0.75, 32 +0.5, 16 +0.25). Q2: key forms ~1 h more.
  CHECK: erased pinned revision -> text null in block, digests unchanged -> false edited? ; paging
  inside 5 s; does the branch set digest cover the pin (overrides incl. revision)?
- 11 R2.2 (1167-1198) read: erase_term nulls folded word, CHECK pairs null with erasure, guard;
  Hub 47 DB test; 48b scan runs marker + FoldWord(marker), before erasure finds exactly head col.
  0.25 h in Hub 47. CHECK: does erasure reach a branch departure holding the same word / a
  not-here departure that copies the pinned fields (M5)? If not, codename survives = leak.
- 11 R2.3 1200-1289 read: M1 chain 44a 4 -> 45 5 -> 25 1.5 -> 28 4.5 -> 27 7.5 -> 29 5.5 -> 32 5.5
  = 33.5 h, +28.5 h; 26 depends on 06 alone; 31 codes against 32's part. M2: ids minted first,
  passes total (leaves force / every other), replacedBy+overrides DEFERRABLE, +0.25 Hub 47. M3:
  words first then decisions, one Undo both, +0.25 p4 31. M4: hubapi.Overlay in Hub 45 (+1.25),
  25 -1; shadow extended to avoid. M5: not-here COPIES pinned fields (word, is / decision) ->
  confirms erasure-copy question; set_aside for terms; verb "Not on this branch" +0.25 p4 31.
  M6: project changed compares fields+state.
- 11 R2.3 1289-1363 read: M7 cost in Q8 row + lens says it cannot tell (no hours). M8 markers on
  §3.1/§4.2/§7.0/§10.2 (not §8); settled on HC:1126; set envelope members (v, kind, projectId,
  scope, branchBriefId, items{digest,id,kind,revision}); revision envelope members listed; 44a
  +0.5, 45 +0.25. M9a Inheritable gains sets + pins, +0.5 p4 27. M9d feedSeq per page, 200/page,
  re-read; +0.25 48b, +0.25 27. CHECK HC:1091-1107: is feed seq commit-ordered per org? M9f four
  routes +1.5 p4 32. M9g -> Q10.
- 11 R2.4 1364-1427 read: Q10 row (default yes; other answer ~1 h Hub 47, 48a, p4 31; best
  before Hub 47). Figures table: p4 25 2.5->1.5, 27 6->7.5, 31 6.5->7, 32 3.5->5.5, 16 +1->+1.25;
  Hub 44a +2->+2.5, 45 3.5->5, 46 5->5.5, 47 4->4.5, 48b 4->5. ARITHMETIC CHECKED: p4 +3.25 =
  45.25, Hub +4 = 31, total 76.75 (+7.25); path 33.5 (+28.5). ~100 h with fix round.
  CHECK: "26 can start now" (M1) vs "Q2 asked before ... phase 4's 26 starts" (R2.1:1164-1165).
  CHECK §7.3 rows: does 27 depend on 28? 26 on 06 alone?
- 11 §0 13-62 read: 76.75 = 45.25 + 31 + 0.5; seven-task chain from Hub 44a, 28.5 h before 14;
  ten questions; Q2 before 44a, Hub 45 or p4 26; other nine block nothing. AGREES with R2.4.
- 11 §2.1-2.3 199-303 read: :237-238 points to R2.1 read; :261 set_aside for not-here; :281
  not-here born set_aside WITH n's fields (a copy of the project's text in a branch record);
  :296-297 Overlay; :298-303 old same-word shadow text, extension only via pointer to M4.
- 11 §2.4-2.5 305-400 read: :316-318 passes + ids first + refs at commit (M2 ok); :324/:326
  retire/withdraw accept set_aside; :327 reinstate retired|withdrawn -> in_force (an undone
  not-here reinstated becomes an in_force copy of the project's entry, not set_aside: Minor?);
  :328 depart carries @n's fields; :342 overrides/replacedBy name a NON-ERASED record (so what
  happens to a departure whose pinned project entry is erased later?); :373 V four reads; :376-382
  heads only, ordered by id, filterable to one branch brief, eight routes. R2-I1 heads part OK.
  Q: unfiltered list = all branches' + out-of-force heads? then "a set of up to 200 = one page"
  is about the list, not the set.
- 11 §2.6 402-441 read: erase routes are PER RECORD (/v1/terms/{termId}/erase); :424-425 folded
  word goes (R2.2 in place OK); :431-434 scan gains FoldWord(marker) OK. NOTHING cascades to a
  branch record whose `overrides` pins the erased record. With M5 (:281, :328) a not-here copies
  @n's word+is / decision+context+why+undo+rejected into its OWN record -> erasing the project
  entry leaves that verbatim copy + its head's folded word. Brief erasure has `settled` list; term
  erasure has no equivalent. CANDIDATE NEW IMPORTANT (erasure leak, introduced by R2 M5).
  Also: in_force departure's origin line reads the pinned revision -> null after erasure; what
  does the block print? check §5.1/§5.3 + p4 inspect for erased handling.
- Grep "erase" over 11: CONFIRMED no cascade anywhere (only :331, :342 write-time, :421-441,
  :898 task 47, R2.2). :342 checks non-erased only at write. The 48b scan erases one record, so
  it passes with the copy standing. An in_force departure keeps the same `word` too (older, R1).
- 11 §5.1-5.2 569-679 read: :594 -> M9d; :618/:622 not-here lines print; :619 "Origin lines
  print the pin (R2.1)" OK; :652-654 set envelope inline still shows only v/kind/scope/items but
  points to M8 (fine); :674-677 reads back via R2.1 OK. OVERLAY GAP: R2.3 M4 (:1250-1251) says
  Overlay = project in-force - overridden - shadowed + branch's own IN_FORCE; set = overlay
  (:580, :644); so set_aside records are NOT in the set, yet the block prints not-here lines and
  M5 (:1268-1269) says their own digest covers the line. Rule 3 needs set_aside excluded; the set
  needs it included. Minor: say set = Overlay view + the branch's set_aside records.
  Erased pin: in_force departure's origin line after its pinned project revision is erased ->
  text null; no rule for what the block prints (Minor).
- 11 §5.3-6.3 681-823 read: :705 "two a start would write now" OK (M9a); :750 R2.1 OK; :753
  land OK; :782 R2.1 OK; :787-789 fields or state OK (M6) BUT "fields" is ambiguous: §2.2's
  "Fields" includes settledIn/author/sessionId, and a restore's `restores` member differs; if
  compared, the false project changed returns. Minor: say text fields + state. :794 verb OK;
  :800-802 "not recorded" + land OK (M7).
- 11 §7.1-7.3 825-906 read: p4 new sum 2.5+1.5+1.5+7.5+4.5+5.5+3.5+7+5.5 = 39 OK (:889); Hub
  new 5+5.5+4.5+6+5 = 26 OK (:902). Deps: 25 <- 06, Hub 45; 27 <- 07, 08, 25, 28, Hub 46; 28 <-
  05, 25; 29 <- 09 fix, 10, 27, 28; 32 <- 11, 12, 27, 29, 30; chain 33.5 OK. :881 26 <- 06 only,
  "no Hub task in front of it" but §0:59 and R2.1:1164 say Q2 is asked before 26 starts: the
  row does not carry the Q2 gate (Minor wording). :897 46 fake: refs at batch end, FoldWord,
  feedSeq OK; :898 47 OK; :900 48b OK. :859 read.go row names Inheritable + binding_test OK.
- 11 §7.4-8.2 908-996 read: p4 amended 0.5+1+1.25+0.5+1+1+1 = 6.25 OK; Hub amended
  2.5+0.25+0.5+0.25+0.5+0.5+0.5 = 5 OK; §7.5 table 45.25 / 31 / 76.75 OK; 79% and 23% OK;
  54+16+7.25 ~ 76.75 OK; :938-943 chain OK; :941 "26 ... can start now" vs :946/§0:59 Q2 gate
  (Minor). Q2 :989 "about 1 h more" agrees with R2.1; Q8 :995 cost stated; §8.2 heading -> Q10.
- owner-questions 105-163 read: Q2 (struct ~2 h in 46/48b/27; key forms ~3 h; ~1 h more; holds
  up 44a, 45, 26 + 20/46/48b/16/27), Q8 (cost + cannot tell), Q9, Q10 (default yes; ~1 h in 47,
  48a, 31; holds up Hub 47) ALL AGREE with 11 §8.2 and R2.4. Q2 exception to line 3 stated.
- NEXT: verify HC:48-49 page max (200?), HC:1091-1107 feed seq commit order, HC:817-818, p4
  inspect.go :202-215 PinErased (erased pin handling for a brief).
- HC:48 limit max 200 OK. HC:1098-1107 feed = org audit seq, gapless, commit order, event in the
  change's txn under the org's audit lock -> M9d's equal-position test is sound. But the lists
  hold every head (all states; all branches unless filtered), so "a set of up to 200 entries is
  one page" is about heads, not the set (Minor wording; filter semantics unstated).
- p4 ca737d92: brief's erased pin is solved: inspect.go:22-29 + :212-213 (PinErased ->
  superseded, Diff [] so erased text not shown); read.go:82-86 PinErased, :561-566 documentOf
  renders own text alone (rule 7; b06-2 R2-I1). The sets' pins (R2.1) get NO such rule: erased
  pin -> origin line null -> branch set digest unchanged (overridden entry not in set) -> render
  differs -> `edited` with DiffLines showing erased text. + M5 copy survives erasure and is
  re-materialised at every branch start. => R3-I1 IMPORTANT (erasure leak + false edited).
  Round 2's own M5 fix suggested the copy: say so.
- plan:406 (15 <- 11, 12 contracts not code) OK; CONTEXT.md:62-67 land = one lane into its
  branch OK; :300 merge -> land OK; T:27.md:72 marker OK.
- git diff -U0 2726d918..HEAD on 11: every hunk above :1100 is one-for-one (-a,b +a,b); the only
  insertion is +1100,327 (Revision 2 appended). Rule 4 HELD: no cited line moved.
- Q10 in owner file "Holds up: Hub task 47" vs §0:61-62 "the other nine block nothing" and Q9's
  shape "Holds up: nothing built" (Minor wording).
-->

















