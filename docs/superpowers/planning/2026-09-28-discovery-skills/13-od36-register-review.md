# OD-36 in the register, and CONTEXT.md's residuals: independent review

Reviewer: an independent, read-only review of `cdfa6b95` (OD-36 into
`docs/superpowers/planning/2026-09-23-owner-decisions.md`) and `ce9621b0` (the `CONTEXT.md`
residuals I1, I2 and I4, with the minors). The implementer's return is
`13-od36-register-report.md`. I wrote only this file.

**Verdict:** pending.

## 1. OD-36's row against the decision note, the gate and the frontier

**Holds.** The row says what `10-decision-terms-beside-the-brief.md` decided. It argues all three
limbs of the gate with evidence, and its one `OD` edge is written in both columns.

**The row (register `:409`) against the note, clause by clause:**

| Row clause | Note | Match |
|---|---|---|
| Beside; a glossary and a decision record; each its own Hub record referencing a brief revision | `10-…:10-11` | ✔ |
| The owner's words | `10-…:34-35` | ✔ verbatim |
| Lifecycle, Scope, Supersede | `10-…:38-48` | ✔ no ground added or dropped |
| What moves: the Materialiser (09), `Ref` (06), the lens (18 to 21) | `10-…:59-62` | ✔ |
| What does not move: the content model, the key grammar | `10-…:52-55` | ✔ |
| `05-design.md` not adopted; design round 2 first | `10-…:66-76`; `06-sceptic.md:383` is `**Recommendation quality: Needs fixes**` | ✔ |

- **Left out, rightly.** The queue-slot status (`10-…:74-76`), which backwards-audit observation 1
  (`:390-393`) says a row should not carry. The example key `term:tm-7f2a@2` (`10-…:14`), whose
  4-hex id does not match `bf-<16 hex>` at `CONTEXT.md:107`: its form is design round 2's to set.
- **One addition the note does not make:** "possible only because OD-3 made the brief one". It is
  true of the premise: a record that references a brief revision needs Hub-numbered revisions
  (`CONTEXT.md:107`, "The Hub numbers revisions; nothing else may").
- **Task numbers**, against the phase-4 worktree's plan headings: `task-06.md` is `provenance`,
  `type Ref struct` at `:123`; `task-09.md` is `contextwrite` — the Materialiser; `task-18.md` and
  `task-19.md` are the lens. `task-20.md` (`4-D1` at session start) and `task-21.md` (import) are
  desktop Discover tasks the note groups under "the lens". The row copies the note's grouping.

**The gate (`:411-425`, against the limbs at `:275-283`):**

1. **Expensive to undo** — names data (records folded back, each fold minting a revision), code
   (three parts), and a re-plan before build. "Three built parts" reads under "Once built": today
   06 and 09 are built and 18 to 21 are not (`11a-facts-phase4.md` §1, not re-checked). ✔
2. **Not derivable** — the evidence is a real wrong guess, `05-design.md`. Spot check:
   `glossary|vocabulary|decision record` over the phase-4 worktree's `control-plane/` hits only two
   `internal/harness` comments about permission-mode words. "Captures no language" survives. ✔
3. **A real fork** — the option given up is named (one object, one revision history) and so is the
   cost (`Ref` changes; built code moves). ✔

**The frontier.** `:359` has OD-36 unblocked by OD-3; `:331` has OD-3's *Unblocks* ending with
OD-36. OD-36's *Unblocks* names no `OD`, so no second mirror is owed. The table has 30 rows; with
the six edge-less ids at `:316` that makes 36, so "30 of the 36" is right. OD-5 → OD-36 is argued
out at `:427-430`, and the reason holds: the premise is a shared brief, not where the Hub runs.

The edge's only statement is OD-36's own row. `:314` asks that "this file, or a document it names,
states the link", which is met literally. The implementer disclosed it; I accept it.

## 2. No cited line of the register moved

(in progress) Notes so far, all run through `claudezsh` against the real tree:
- `a109bf3e`'s register is exactly 266 lines. The working file's `head -266` `cmp`s equal to it
  (both sha256 `0702ec3c…4b864`). The working file equals `HEAD` equals `cdfa6b95`: 430 lines.
- Against the parent `a5a60ee3` (398 lines): in-place changes at 270, 303-305, 316 and 331 only;
  one line inserted at 359; parent 359-398 `cmp` equal to working 360-399; 400-430 appended.
- Inbound `owner-decisions.md:N[-M]` anchors: main tree (`git grep`) highest end line 188
  (`2026-09-25-identity-keycloak-broker.md:571`, `:186-188`); worktrees (`grep -r`, 216 hits)
  highest 222 (phase-6 `task-26.md:3`, `task-27.md:3`, `progress.md:560`, all `:203-222`).
- Re-opened ten citing sites; two mismatches found are 07-register-review-2 M3's, pre-existing.

## 3. CONTEXT.md: I1, I2 and I4, and the cited lines

(in progress) Notes so far:
- `CONTEXT.md` is 340 lines at parent, at `ce9621b0` and in the working tree (working == `ce9621b0`).
- Text changed on lines 29, 60, 64, 149, 154-156, 166, 215-216, 233-237, 249-252, 323-324. In
  233-237 two lines moved down one (`*Not «permission mode»*` 235→236, `per-session flag` 236→237);
  disclosed at `08-context-report.md:237-240`.
- Inbound `CONTEXT.md:N` anchors (`grep -r`, main tree incl. hidden dirs and all worktrees): 14, all
  in this folder, at 3-10, 47, 107, 108-110, 125, 171, 216, 220, 228, 242-243, 255, 303, 328-340.
  None falls in a moved line; each re-opened line still says what its citer says (216 is review-2's
  record of the defect itself).
- I1: `AGENTIC-UX.md:71` is §4 rule 4 ("One ranked queue … Fails if a count appears anywhere outside
  the queue"); `:245` is §10 *Anti-patterns* item 10 ("No badge or count outside the queue"); `:31`
  is §2 item 4's heading, its sentence at `:32`. The phrase "hard rule" appears nowhere in the doc.
- I2: `CONTEXT.md:60` and `:64` now say "landing-queue row" / "landing queue"; `grep queue` finds no
  other bare sense-2 use (`:192`, `:221`, `:291`, `:292` are sense 1 or the rule itself).
- I4: `:166` "design approvals"; `:149` "not an *approval row*"; `:234-235` defines it, citing
  `AGENTIC-UX.md:241` (§10 item 6, "No approval row without four things: the ask, …"). The Hub
  contract's only approval heading is `:872` §7.7. Team plan `task-49.md:15` adds a `la-` land
  approval as a Hub record (a member's, not an agent's, ask).
- Minors M1 (`FUNCTIONAL-SPEC.md:15`), M3 (Hub `:966`, `:971`, `:984`, `:1015`), M4 (`:46`), M5
  (`ui-contract.md:45`, `:64`; register `:172`) each re-opened and correct.

## 4. Findings

(in progress)

## 5. What I did not verify

(in progress)
