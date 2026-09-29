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

### 2.2 Important

### 2.3 Minor

## 3. What held up

## 4. What this review did not verify

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
