# 07 — The register work: the gate, the frontier, and what the audit found

**Date 2026-09-28.** Two changes landed, both documentation only. Commits `1cb8be9a` (gate +
frontier) and `686e35c2` (audit + the pending questions' *holds up* notes). No product code was
touched, no test suite run, no plan edited.

Source of the method: `00-summary.md` §3 practice items 3 and 4, which `06-sceptic.md` §7 kept —
*"the three-leg test for what earns an `OD`, the unblocked by / unblocks columns … they are habits,
not work, and they do not need a plan at all."* Method only: none of the upstream catalogue's
wording is in either file, and its three limb labels were deliberately re-worded, because
`06-sceptic.md` §8 item 1 records that nobody has checked our labels against the source's.

---

## 1. The gate, as written into the register

Near the top of `2026-09-23-owner-decisions.md`, before the first decision table. A question earns
an `OD` when **all three** are true: **expensive to undo**, **not derivable from the tree**, **a real
fork**. Each limb carries one sentence on what it excludes. Three explicit non-purposes are stated
in the file: it does not govern orchestrator rulings, it does not create `docs/adr/`, and it is not
retrospective.

## 2. The frontier

One table near the end of the register, not two extra columns on the five decision tables — their
rows are already paragraph-length and two more cells each would make them unreadable. **This is the
one place I departed from the task's literal wording**, and the file says why in its own first
paragraph.

| Measure | Count |
|---|---|
| Decisions carrying at least one edge | **29 of 35** |
| Decisions naming what they waited on (*unblocked by*) | 18 |
| Decisions naming what they released (*unblocks*) | 29 |
| Decisions left blank | **6** — OD-2, OD-4, OD-7, OD-9, OD-28, OD-35 |

**The rule I held myself to:** an edge is recorded only where the register, or a document it names,
states the link. A phase consequence restated from the row's own *Consequence* cell is not an edge —
that would have turned the table into a second copy of the register and made the frontier useless.

**Four candidate edges I declined,** as examples of the bar: OD-2 → "phase 3's worktree model" and
OD-4 → "phase 3's push/PR path" (both are the Consequence cell restated); OD-18 → OD-32 (the IaC
engine moves whatever the cloud is); OD-22 → OD-4 (a read-only credential is not obviously gated on
the two-host decision). Supersession chains are excluded by design and the file says so: a
superseded decision blocks nothing, it has stopped being in force.

**The density is itself a finding.** The task expected most decisions to have neither edge. The
register's own text makes 29 of 35 connected, which says these decisions were taken in dependent
clusters — the inference cluster (OD-19/21/24/33), the team cluster (OD-14/15/16/17/22/23) and the
identity cluster (OD-6/13/30) each read as one decision split across a round.

## 3. The backwards audit

All 35 rows assessed, **read-only**; nothing deleted, renumbered or rewritten. Six would not pass
the gate they now sit under: **OD-7** (backup repo), **OD-9** (Ctrl chords), **OD-10** (Forget
deletes the transcript), **OD-12** (batch the hands-on session), **OD-28** (the two fonts), **OD-35**
(no scope cut). The reason per row is in the register.

Two structural observations went in with them, and they matter more than the six:

1. **Rows carry build status as well as decisions** (OD-10, OD-30 and OD-31 have been amended in
   place with what was later built). The stated convention going forward: an amendment keeps its
   bold date marker, so the decision stays the first sentence of the Answer cell.
2. **The order is arrival, not structure** — OD-29 sits after OD-31 and OD-30 because that is when
   it was asked. The frontier table is now the reading order.

## 4. The pending questions

All nine open questions in `2026-09-25-owner-questions-pending.md` carry a *Holds up* line. Four
hold real work: Hub 6 (task 40's commit shape), Hub 7 (ADR-H1's text plus the realm ask to
Panacea), Hub 9 (task 41's component code and `feat/hub`'s onward merge), and P7-1 (the connect
panel's key advice, the largest of the nine). Five hold up no in-flight work at all, which is
recorded plainly rather than dressed up.

## 5. Open question for the owner — the one judgement I did not make

**Where does an owner answer go when it fails the gate?**

The gate says a row needs all three limbs. But this register is also the only record of what the
owner said in a question round, and some of those answers are genuinely notes — OD-28 chose two
fonts; OD-7 recorded where the backup lives. Under a strictly enforced gate they would have nowhere
to live, because there is no notes file, and creating one is the second decision store the gate
itself forbids.

Three options, no recommendation, because this is the owner's call:

1. **Gate the register, tolerate the notes.** Keep writing every owner answer as an `OD`, and accept
   that some rows are notes — the audit section already tells a reader which. Costs nothing; the
   register stays slightly diluted.
2. **Gate strictly, notes go to the plan they affect.** A failing answer is written into the phase
   plan or task it governs, and never gets a number. Keeps the register sharp; costs a
   findability problem, because an owner answer in a part file is not where anyone looks.
3. **One register, two row kinds.** Add a `note` marker to the row itself, so a reader sees the
   distinction at the row and not only in an audit at the end. Cheapest to read; costs a one-time
   pass over the six rows and a convention every future round must follow.

This is recorded, not decided (task rule 6), and it has been added to
`2026-09-25-owner-questions-pending.md` as the register's own open question.

## 6. What this work did not verify

- **No upstream page was fetched.** The method is carried from `00`–`06` in this directory; the IP
  caveat in `06-sceptic.md` §8 item 1 stays open, which is why our limb labels are our own.
- **Edges were taken from the register's own text only.** I did not open the phase plans to look for
  dependencies the register does not mention, so the frontier is a floor, not a survey.
- **Row-by-row correctness was not judged.** The audit judged form — would this question have earned
  a row — and changed no row.
- Two other workflows were live in this tree; I touched only the two planning files and this report.

---

## 7. Fix round — 2026-09-28, against review `07-register-review.md`

Three commits, documentation only: no product code, no test suite, no container.

### 7.1 C1 (Critical) — the gate moved, and every pre-existing anchor came back exactly

The gate was inserted at line 8, above the first decision table, and pushed every decision down
**37 lines** (the review said 36; the true offset is 37, measured old line 12 → new 49). 41
line-anchored citations across 12 documents pointed 37 lines early, two of them binding: K1 in
`03-techniques.md:45` and the *"no feature may require the adapter"* quote in
`04-enterprise-constraints.md:121`, both sourced to `owner-decisions.md:73-79`, both landing inside
the gate's own "What the gate is not for" list.

**Fix: the reviewer's cheap one.** "What earns an OD" now sits immediately before "The dependency
frontier". Commit `8e36b6e7`.

**Verification, not assumption.** `git show a109bf3e:…owner-decisions.md` was extracted to a temp
file and the current file's first 266 lines diffed against it whole:

| Check | Result |
|---|---|
| `diff` of old file vs current `head -266` | **identical, byte for byte** |
| The five lines the review named — 13, 21, 73–79, 114, 140 | all restored: OD-2, OD-10, the OD-8 transcript blockquote, the team-mode header, OD-19 |
| `git diff --numstat` for the move | 37 / 37 — a pure relocation, no content change |

Because the whole 1–266 region is identical, this is stronger than checking five anchors: **every
pre-existing anchor is exact, not only the sampled ones.** Nothing was left unrestored.

**The gate's own sentence is corrected.** *"It is not retrospective"* now says what it covers — ids,
not lines — names the 41-citation hazard as the thing the id argument does not reach, and adds the
rule that follows: anything added to this file later goes below the tables.

### 7.2 The review's "only one citation needs a bump" was an undercount: seven did

The review named `08-context-report.md:38`. Re-grepping for line citations written today found six
more, all in files the sibling context chain wrote against the shifted register and all correct
until the move: `CONTEXT.md` OD-2 `50→13`, OD-3/OD-5 `51,53→14,16`, OD-24 `189→152`, OD-19
`177→140`, OD-16 `164→127`, and the same three team-round numbers in the report's citation table.
Each was re-read against the file after the edit. Commit `343a0276`.

**Correction to that commit's own message.** It warns that it may carry one uncommitted line from the
sibling context chain (`CONTEXT.md:228`, "a grid of nine action classes"). It does not: the sibling
committed that line itself as `31229e7a` in the gap between the two commits, so `343a0276` is the six
anchor bumps and nothing else — confirmed by `git show --stat`. The warning was written before the
race resolved and is wrong in the safe direction.

### 7.3 I1 — the frontier now reads the same in both directions

Mirrored the four one-way edges into the parents' *Unblocks* cells: OD-5 → OD-14 (with the
`05-design.md:121` citation that carries it), OD-19 → OD-18, OD-1 → OD-12 and OD-31 → OD-12. The
preamble now states the invariant rather than leaving it to be inferred: every edge appears twice,
adding one means editing two cells. The failure the review named — an agent asking *"what does
answering OD-5 release?"* and not learning that the team-mode cluster waits on it — is answered in
OD-5's own row now.

### 7.4 I2 — OD-15, OD-16 → OD-17 is removed, not evidenced

I looked for a sentence to hang it on and there is none: OD-17's cell names neither parent, neither
parent's cell names OD-17, and `05-design.md` is disqualified because it is the artifact OD-17
scheduled. Same-round adjacency is not the bar the table states, so the edge goes. OD-17 keeps
OD-14. The removal is recorded in a short note under the table, with the reasoning, so the next
reader does not re-add it on the same intuition I had.

### 7.5 The three Minors

- **M1** — OD-30's *Unblocked by* now says which relation each parent carries: OD-6 as the
  Credential Manager store, *not* the issuer it supersedes; OD-13 as the reason the realm stays
  Panacea's.
- **M2** — the audit named six rows; **OD-27 is a seventh**, confirmed at source before writing it:
  `docs/AGENTIC-UX.md:77` and `:237` already banned percentages, bars, ETAs and "N of M", and
  `git log` puts that file's last touch at `9f8c8c41`, 2026-09-17 — eight days before the round. Six
  → seven in the audit heading, in the structural-observations line, and in the pending question
  that quotes the count; option (a)'s dilution is restated as a fifth of the register rather than a
  sixth.
- **M3** — OD-10's row no longer asserts that deletion is the only answer the word allows.
  `contracts/2026-09-18-phase-2-go-contract.md:2266` records the alternative as having been in force
  and built (*"Before, the purge left every transcript … kept forever"*), and reversing it cost a fix
  round, a 500-and-keep-the-record path and a seam handed to phase 3. The row now fails on what
  observation 1 already distinguishes: its second half is build status, not decision.

### 7.6 Not done, and not decided

- **The review's fuzzy-limb note was not adopted.** It suggests the gate say to apply the cheapest
  limb first, because limb 2 settled OD-27 where limb 1 did not. That changes how the test is run,
  which is the owner's call about their own record, not a plan editor's. Left for the round that
  answers the open question in `2026-09-25-owner-questions-pending.md`.
- **No product code, no test run, no container.** Files touched: the register, the pending questions,
  `CONTEXT.md`, `08-context-report.md` and this report. `control-plane/**`, `sandboxd/**`,
  `desktop/**` and the phase 6, phase 8 and work worktrees were not opened for writing.
- **Line numbers in sibling worktrees were not audited.** The 12-document count is the main tree;
  the same citations exist in the `team`, `work`, `phase-4`, `phase-5`, `phase-6`, `hub` and `env`
  worktrees, whose copies of this file are at their own commits. They are restored when those
  branches take this one.
