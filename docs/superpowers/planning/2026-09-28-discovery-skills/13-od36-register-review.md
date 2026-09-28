# OD-36 in the register, and CONTEXT.md's residuals: independent review

Reviewer: an independent, read-only review of `cdfa6b95` (OD-36 into
`docs/superpowers/planning/2026-09-23-owner-decisions.md`) and `ce9621b0` (the `CONTEXT.md`
residuals I1, I2 and I4, with the minors). The implementer's return is
`13-od36-register-report.md`. I wrote only this file.

**Verdict: Approved.** All three checks hold. Findings: no Critical, no Important, four Minor
(§4), and none of them breaks a citation or misstates a decision.
1. OD-36's row says what `10-…` decided, passes all three limbs of the gate, and its OD-3 edge is
   written in both columns.
2. The register's `head -266` `cmp`s equal to `a109bf3e`. No inbound anchor anywhere ends past line
   222. Eleven citing sites were re-opened: nine land, and the two that miss were already recorded
   by review-2 (M3).
3. `CONTEXT.md`'s I1, I2 and I4 are closed with correct line numbers. The two lines that moved
   inside the Authority entry are cited by nothing.

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

- **Left out, rightly.** The queue-slot status (`10-…:75-76`), which backwards-audit observation 1
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
   06 is approved, 09 is in fix and 18 to 21 are unbuilt (`11a-facts-phase4.md:55`, `:58`, and its
   rows for 11 to 23; review status, not code). ✔
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

**Holds.** Every inbound line anchor lands in lines 1-266, and those 266 lines are byte-identical
to `a109bf3e`. All of this ran through `claudezsh` against the real tree.

- **The identity.** `a109bf3e`'s register is exactly 266 lines. The working file's `head -266`
  `cmp`s equal to it, and both hash to sha256 `0702ec3c…4b864`. The working file equals `HEAD`
  equals `cdfa6b95`, 430 lines.
- **What moved, against the parent `a5a60ee3` (398 lines).** Text changed in place at 270, 303-305,
  316 and 331. One line was inserted at 359. Parent 359-398 `cmp`s equal to working 360-399. Lines
  400-430 are appended. This matches the report's §6 exactly.
- **The inbound anchors.** In the main tree (`git grep`, plus `grep -r` over `.superpowers/` and
  `RESUME.md`) no `owner-decisions.md:N[-M]` ends later than 188. Across all ten worktrees
  (`grep -r`, 216 hits) the latest is 222: phase-6 `task-26.md:3`, `task-27.md:3` and
  `.superpowers/sdd/p6/progress.md:560`, all `:203-222`. Nothing cites 267 or later by number. The
  bare `:NNN` forms in `07-register-*.md` stay inside 1-266 too.
- **The report's own anchors past 266** (`:303`, `:316`, `:331`, `:359`, `:390`, `:401-430`,
  `:411-425`, `:427-430`) were checked against the new file, and all land.

**Eleven citing sites re-opened:**

| Citing site | Cites | Register line now | |
|---|---|---|---|
| phase-6 `task-26.md:3` | `:203-222` | `:203` is the OD-33 heading and `:223` OD-34's: the range is OD-33 exactly | ✔ |
| `CONTEXT.md:47` | `:13` | OD-2 | ✔ |
| `CONTEXT.md:171` | `:14,16` | OD-3, OD-5 | ✔ |
| `CONTEXT.md:242-243` | `:152`, `:140` | OD-24, OD-19 | ✔ |
| `CONTEXT.md:255` | `:127` | OD-16 | ✔ |
| `08-context-report.md:255` | `:172` | "Q4: the Desk frame is in W1." | ✔ |
| team `task-52.md:64` | `:164` | OD-27 | ✔ |
| team `task-51.md:48` | `:165` | OD-28 | ✔ |
| phase-5 plan `:10` | `:84-100` | `:84` is the OD-11 heading | ✔ |
| `2026-09-25-identity-keycloak-broker.md:4` | `:166`, for OD-30 | OD-31 | ✘ pre-existing |
| `…identity-keycloak-broker.md:571`, main and team | `:186-188` | the OD-32 to OD-35 preamble | ✘ pre-existing |

The two ✘ are `07-register-review-2.md` M3's (`:140-144`). They are byte-identical at `a109bf3e`,
so this work did not cause them, and they are not this round's to fix.

## 3. CONTEXT.md: I1, I2 and I4, and the cited lines

**Holds.** I1, I2 and I4 are closed, each new citation lands on a line that says what the entry
claims, and no line another document cites has moved. Two label-level nits are in §4 (M1, M2).

| | Review-2 asked | Now | The cited lines | |
|---|---|---|---|---|
| I1 | `:71` for the ranked queue and counts; `:31` only for the pane sentence, without "rule" | `:214-216` | `AGENTIC-UX.md:71` is §4 rule 4, *"One ranked queue … Fails if a count appears anywhere outside the queue"*. `:245` is §10 item 10, *"No badge or count outside the queue"*. `:31` is §2 item 4's heading, its sentence at `:32`, now labelled §2 | ✔ |
| I2 | "landing" on the two bare sense-2 uses | `:60`, `:64` | `grep queue CONTEXT.md` leaves `:192`, `:221`, `:291`, `:292`: sense 1 or the rule itself | ✔ |
| I4 | "design approvals" in the Hub entry; one clause parting the approval row | `:166`, `:149`, `:234-235` | The Hub contract's only approval heading is §7.7 (`hub-contract.md:872`). `AGENTIC-UX.md:241` is §10 item 6, *"No approval row without four things: the ask, …"* | ✔ |

- **The do-not-use row** at `:292` ("bare 'the queue' is rule 4's attention queue") now agrees with
  the entry's §4 rule 4.
- **The minors, re-opened.** M1: `FUNCTIONAL-SPEC.md:15` is the Sub-agent row, "Yes, as nested
  steps". M2: `AGENTIC-UX.md:37` is §2 item 6. M3: Hub `:966` (the control plane, phase 7's client),
  `:971` ("If step 1 fails, **Ship does not happen**"), `:984` (`no_brief`, `no_complete_revision`)
  and `:1015` ("else none"). M4: `FUNCTIONAL-SPEC.md:46` (claimed from a warm pool). M5:
  `ui-contract.md:45` and `:64` name the Desk, and register `:172` puts it in W1. M6: "trust level"
  is now only in `08-context-report.md` and review-2 (main tree). All correct.
- **Line positions.** `CONTEXT.md` is 340 lines at the parent, at `ce9621b0` and in the working tree
  (working == `ce9621b0`). Text changed on 29, 60, 64, 149, 154-156, 166, 215-216, 233-237, 249-252
  and 323-324. Inside 233-237 two lines moved down one (`*Not «permission mode»*` 235→236 and
  `per-session flag` 236→237), which `08-context-report.md:237-240` discloses.
- **Inbound anchors.** `grep -r` for `CONTEXT.md:N` over the main tree, hidden directories included,
  and every worktree finds 14, all in this folder: 3-10, 47, 107, 108-110, 125, 171, 216, 220, 228,
  242-243, 255, 303 and 328-340. None is a moved line, and each still says what its citer says.
  `:216` is review-2's record of the I1 defect, so its text changing is the fix.
- **The other two edited files.** `08-context-report.md` §6 is 10 lines for 10
  (`@@ -182,10 +182,10 @@`), and its only inbound anchors (`:38`, `:48`, `:49`) are above it.
  `10-…`'s status line is 4 for 4 at `:3-6`, so `11b-facts-hub.md`'s anchors into it (`:14`,
  `:27-28`, `:54`, `:61`) are unmoved.
- **I3's residual** is in Open question 2 at `:323-324`. `CLAUDE.md:5`, `:15` and `:27` do say
  "phases" or "Design phase", and `:295` is the build-order row. The default taken is written in
  `08-context-report.md:266-267`, not in `CONTEXT.md`. There is no room in place for it: `:325` is
  the question's closing blank line, and `:328-340` is cited by `11b-facts-hub.md:653`. Acceptable.

## 4. Findings

**Critical: none. Important: none. Minor: four.** Nothing below breaks a citation or misstates a
decision.

**M1. "hard rule 10" is not a name `AGENTIC-UX.md` uses.** `CONTEXT.md:215` labels `:245` "hard rule
10". The document calls §10 *Anti-patterns* (`:232`), and "hard rule" appears nowhere in it. Bare
"rule 10" is §4 rule 10 at `:77` (*"Replace progress with convergence"*), a different rule. That is
the two-list collision I1 and M2 were fixed for. The line number is right, so a reader who opens it
lands correctly. The label is review-2's own wording (`08-context-review-2.md:77`).
*Fix, in place on `:215`:* "§10 item 10, `:245`", which is one character shorter.

**M2. The approval-row sentence calls the row the decision.** `CONTEXT.md:234-235` says an approval
row "is a person's decision on one action an agent asked for", and `:149` says "that is a decision
under **Authority**". The cited line makes the row the thing a person decides:
- `AGENTIC-UX.md:241` gives it "the ask, ≥12 lines of surrounding transcript, the consequence of
  inaction, and a standing-policy option";
- `:190` puts "standing policy on every approval row".

"Never a Hub record" rests on the Hub contract having one approval kind (§7.7, `:872`), not on
`:241`. The wording follows review-2's suggested fix, and it does part the two senses, which was
the finding. *Fix, in place:* end `:234` at "is" and make `:235` read "one agent's ask waiting on a
person's decision (`docs/AGENTIC-UX.md:241`), never a Hub record"; on `:149`, "an ask under".

**M3. The new last section's shape will not survive OD-37.** Three sentences disagree about where
the next decision goes:
- the heading names OD-36's topic (`:401`, *The project glossary and decision record — 2026-09-28,
  OD-36*);
- `:304` says "OD-36 onwards sit in its last section";
- `:404` says "Later decisions append below this one".

If OD-37 is a new row in this table, the heading is wrong. If it gets its own section, `:304` is
wrong. Also, *Considered and not recorded: OD-5 → OD-36* (`:427-430`) sits 68 lines below the
frontier it annotates, while the other such paragraph (`:361-367`) sits directly under the table.
None of these lines is cited. *Fix, in place:* retitle `:401` as a general section for decisions
made after the gate, with OD-36 as its first row, or make `:304` say "are appended at the end of
the file".

**M4. Two small accuracy slips in the returns, neither changing a conclusion.**
- `ce9621b0`'s body and `08-context-report.md:218` say "Every edit is in place". Two lines of the
  Authority entry moved down one, as the same report says at `:239-240`. Nothing cites them.
- `13-od36-register-report.md:16-17` finds no main-tree register anchor "above `:186`". Its
  `grep -o "owner-decisions.md:[0-9]*"` reads only a range's first number. The latest is `:186-188`
  (`2026-09-25-identity-keycloak-broker.md:571`), still far below 266.

**On the implementer's five concerns.** (1) `CLAUDE.md:23`'s bare "approvals" is the I4 ambiguity
in the file `CONTEXT.md` backs. It is outside this task's files, and the next `CLAUDE.md` edit
should say "design approvals". (2) Below the tables is the register's own rule (`:303`), and
`:303-305` and `:359` lead there. (3) The self-sourced OD-3 edge is accepted in §1. (4) The
unverified items stand, and my grep in §1 supports the "no language" claim. (5) I staged only this
file.

**Forward note, not a finding.** The team-mode plan adds a third approval: a `la-` land approval,
a Hub record written by a second member (`team` worktree, `…phase-t-team-mode-parts/task-49.md:15`,
`:135`, `:212`). It approves a member's land, not an agent's ask, so it does not contradict
`CONTEXT.md:234-235`. It is shown on the merge lens (`:360-362`). The Hub entry's "design
approvals" (`:166`) will need it once team mode is built.

## 5. What I did not verify

- **The owner's words.** Like the implementer, I took the quote from `10-…:34-35`. I did not see
  the original message.
- **Build status of phase 4.** "06 approved, 09 in fix, 18 to 21 unbuilt" is `11a-facts-phase4.md`
  §1, which another run is still writing. I checked plan headings in the phase-4 worktree, not code.
- **"Discover captures no language".** One grep over the phase-4 worktree's `control-plane/`. The
  desktop half was not searched.
- **Whether OD-36 is the right decision.** I judged form only — does the row say what was decided,
  and does it pass the gate — as the register's own backwards audit does (`:398-399`).
- **The worktrees' own copies of the register.** Their anchors were checked against the main tree's
  file, not against each worktree's copy. Review-2 reasoned that they resolve against both
  (`07-register-review-2.md:170-173`); I did not re-check each copy.
- **Search scope.** The `trust level` search used ripgrep, which skips hidden directories such as
  `.claude/worktrees/`, so it covers the main tree only. The anchor searches used `grep -r` and
  include the worktrees.

**Task quality: Approved**
