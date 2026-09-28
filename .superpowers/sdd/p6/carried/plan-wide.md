# Carried plan issues — plan-wide

Findings that touch no single task. Under the task rules these are fixed **now**, in the plan,
not carried to an implementer. Each row below is done unless it says otherwise.

Commit: see `git log -- docs/superpowers/plans/2026-09-25-phase-6-gateway.md` for this run.

---

## 1. FIXED — H-5 told phase H two things about `canLowerRoute`, and the second was wrong

Source: `.superpowers/sdd/p6/plan-rereview-b05-2.md:41-55`, "Important 1". The only
Important in the carried set that is not a task's.

The skeleton's H-5 ended *"in that gap's exact formula and always `false` here, since a Hub
project is a team project and its actor has `Solo` false (H-1)"*. The reasoning skips
`Compliance`: `task-01.md:612-615` is `if !a.Compliance && !a.Solo { return ErrNeedsCompliance }`,
so for a caller holding H-4's compliance role on a project whose route is above its floor the
same formula returns `true`. H-1 two paragraphs above already says a team project's route may
be lowered *"by an org compliance role (H-4)"*, so the two clauses contradicted each other,
and the plan never said how the Hub builds the `Actor`.

**Fixed** in `docs/superpowers/plans/2026-09-25-phase-6-gateway.md`, H-5's last sentence: the
formula now names the Hub's actor (`Member` from team membership, `Compliance` from H-4's
role, `Solo` always false), says it is `false` for every caller but a compliance one, says it
is **not a constant**, and names the failure — the compliance officer shown no *Move back to
Global* control and left to `curl` H-1's route.

---

## 2. FIXED — constraint 10's citation did not contain the rule it cites

Source: `.superpowers/sdd/p6/plan-rereview-b05-2.md:78-82`, "Minor 4".

`2026-09-25-phase-6-gateway.md:116` cited `2026-09-18-phase-2-backend-and-cockpit.md:54-58`.
**Re-verified in this worktree 2026-09-28** (`sed -n '52,61p'`): `:53-54` is that plan's own
trailer, naming Opus 5.5 — the other model — and the rule constraint 10 states is at
`:59-60`, outside the cited range.

**Fixed:** the citation is now `:53-60`, with one parenthesis saying why the whole range is
needed.

---

## 3. FIXED — constraint 10 covered ten parts and was silent on fourteen

Source: `.superpowers/sdd/p6/plan-rereview-b05-2.md:84-89`, "Minor 5".

Only parts 01-05 and 16-20 carry a fenced commit block; parts 06-15 and 21-24 end their
commit step with *"... and the attribution line"*, which names no line.

**Fixed:** constraint 10 now ends *"and where a part's commit step says only 'the attribution
line', that line is this one"*.

---

## 4. FIXED — the b05 round-1 map's skeleton anchors are stale

Source: `.superpowers/sdd/p6/plan-rereview-b05-2.md:68-76`, "Minor 3". The finding offered
two options; I took the second, because the first drifts again on the next insertion — and it
already has: b05 round 2 listed H-5 at `:308`, and today it is at `:309`.

**Fixed** in `2026-09-25-phase-6-gateway-review.md`, under the b05 section head: one paragraph
saying the numbers predate constraint 10's insertion, telling the reader to use the symbols
(S6.11, H-1, H-5, P8-6, Q5, and the task 21 and 22 summaries), and giving today's verified
anchors — S6.11 `:300`, H-1 `:305`, H-5 `:309`, P8-6 `:318`, Q5 `:331`, task 21 `:272`,
task 22 `:274` (all re-derived with `grep -n` on 2026-09-28). It also records that
`task-21.md:1039` now lands on the Share bullet and the Hub-spend bullet is `:1040`.

---

## 5. FIXED — b02's second owner question was raised and written down nowhere

Source: `.superpowers/sdd/p6/plan-rereview-b02-1.md:47`, "R-9".

**Fixed** in the review's b02 section: the "Owner question (new)" line now carries both. The
second is **open and is the owner's**, not mine to settle:

> Who compares the AU prices with the AWS price list for `ap-southeast-2` before AU budgets
> are trusted?

`models.yaml`'s AU figures copy Global's (`task-02.md:706`, `:714`, `:722`, `:731`), while
Bedrock's `au.` inference profiles are priced separately — by training knowledge,
**UNVERIFIED**, about 10% higher — so AU budgets bite late until someone owns the check at
`task-24.md:684`. Recommended option, planned behind a seam per rule 7: seed each AU figure
at a declared margin above Global (1.25x) and keep it UNVERIFIED, so the owner's answer
changes one constant. Carried to task 02 (`carried/task-02.md`, R-4) and task 24
(`carried/task-24.md`).

---

## 6. FIXED — two stale counts in the b03 round-1 map

Source: `.superpowers/sdd/p6/plan-rereview-b03-2.md:44`, the nit at the end of `m-4`.

Review `:202` said *"mutants 4 → 7 ... the fragment is 13 lines"* and `:220` said *"the
fragment is 13 lines"*; round 2 (`:233`, `:249`) supersedes both with 15 lines and mutants
7 → 9, without saying so — where b01 round 2 did say so (`:136`).

**Fixed:** both lines now carry an explicit "superseded by round 2" clause with the line that
supersedes them.

---

## 7. FIXED — b08's verification sentence understated its own grep

Source: `.superpowers/sdd/p6/plan-rereview-b08-1.md:71-75`, "M-3".

The ruling said "eight places" and the verification sentence "task 17's eight lines".
**Re-run in this worktree 2026-09-28:** `grep -c "state/projects"
docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-17.md` returns **13**.

**Fixed:** both sentences now say 13 and name the command.

---

## 8. NOT FIXED, recorded — the owner-decisions file in this worktree stops at OD-29

Not from any re-review; found while following rule 8. **Verified 2026-09-28:**

- `grep -o "OD-[0-9]\+" docs/superpowers/planning/2026-09-23-owner-decisions.md | sort -n`
  ends at **OD-29**.
- `OD-30` appears in this worktree only inside the phase 6 plan parts and review
  (`task-08.md`, `task-12.md`, `task-13.md`, `task-15.md`); `OD-31` only in the phase 6
  skeleton and review.
- `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` — the ruling OD-30 and
  task 08a are built to — is **not in this worktree**. It is in the main tree.

So the phase 6 plan cites three owner decisions and one ruling that a reader of this branch
cannot open. This is not mine to fix: the fix is a merge, and rule 1 forbids merging here
while the main tree runs its defect-fix wave. It lands with the same `feat/env` merge that
admits task 08a (`.superpowers/sdd/p6/held.md`). Until then, every implementer on this branch
reading an `OD-30`/`OD-31` citation is reading a forward reference.
