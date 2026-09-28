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
