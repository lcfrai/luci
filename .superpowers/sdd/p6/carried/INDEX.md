# Carried plan issues — index

The OD-31 convergence ruling carries phase 6's residual Important and Minor findings into the
build rather than re-looping the plan. This directory is that carry: **one file per task, so
an implementer reads only its own.**

**If you are an implementer:** open `task-<your id>.md`. If there is no file for your id,
nothing is carried to you. Say in your task report what you did with each finding in your
file — task 25 checks this index against those reports when it closes the phase.

Built 2026-09-28. Plan documents only; no product code was changed.

---

## Counts

Findings, not lines. A finding shared by two tasks appears in both files and is counted in
both; the distinct total is at the bottom.

| Task | Title | Findings | Importants |
|---|---|---|---|
| 02 | Model catalogue and the generated LiteLLM config | 3 | — |
| 03 | Wire contracts, problem codes, `MessagesURL`, `HashToken` | 1 | — |
| 04 | LiteLLM admin client on its own deadlines | 1 | — |
| 07 | keysmith core: mint, refresh, revoke | 1 | — |
| 09 | Usage rollup and the team-budget reader | 4 | 1 |
| 11 | gatekeeper proxy: allowlist, `model` rewrite, headers, streaming | 2 | — |
| 12 | `cmd/gatekeeper`: wiring, `/v1/models`, `HEAD /api/hello` | 1 | — |
| 13 | Project route source: Hub, file, cache, fail closed | 1 | — |
| 14 | `panacea token` and the managed-settings fragment | 2 | 1 |
| 16 | Fake Anthropic upstream | 1 | — |
| **18** | **LiteLLM capability, ZDR and pitfall probe** | **11** | **3** |
| 19 | Helm chart, NetworkPolicies, Key Vault CSI | 1 | — |
| 21 | Control-plane inference API | 1 | — |
| 22 | Cockpit inference panel | 1 | — |
| **23** | **Local end-to-end proof with unmodified Claude Code** | **6** | **1** |
| 24 | Cloud proving scripts, written and not run | 2 | — |
| 25 | Close the phase | 1 | — |
| — | `plan-wide.md` (7 items, 6 fixed in this run, 1 recorded) | 7 | 1 |

**No file, nothing carried:** 01, 05, 06, 08a, 08b, 10, 15a, 15b, 17, 20.
Task **08a** is separately **HELD** — see `.superpowers/sdd/p6/held.md`.

**Distinct findings: 41** across the eight batches — b01 3, b02 9, b03 4, b04 1, b05 5,
b06 8, b07 6, b08 5 (four numbered plus one carried note). **7 Important, 34 Minor.**
Instances above sum higher because three findings are shared: R-6 (tasks 04, 09, 24),
R-8 (tasks 03, 13) and B04R2-1 (tasks 16, 21, 22).

---

## The one that must not be lost

**S-1**, `.superpowers/sdd/p6/plan-rereview-b01-2.md:7-20`, in `carried/task-18.md`.
Probe **P14 can still PASS on a 4xx that never reached the router.** P14 is the owner blocker
for OD-19's AU route; a vacuous PASS clears the one thing it exists to measure, and task 18's
whole AU-route blocker is then vacuous.

---

## Where the carried set came from

Every still-open Important and Minor in the **latest round** of each fix batch:
`plan-rereview-b01-2`, `b02-1`, `b03-2`, `b04-2`, `b05-2`, `b06-2`, `b07-2`, `b08-1`
(`.superpowers/sdd/p6/`). Where a batch has two rounds, round 2 re-checked round 1's findings
and recorded them resolved in the parts, so round 1 carries nothing.

**The review's own findings carry nothing.** Checked 2026-09-28 against
`docs/superpowers/plans/2026-09-25-phase-6-gateway-review.md`: it raises C-1, I-1 to I-10 and
M-1 to M-8, and every one of them is named in a batch heading — b01 (C-1, I-8), b02 (I-7,
I-5), b03 (I-4, I-6), b04 (I-10, I-1), b05 (I-9, I-3, I-2), b06 (M-1, M-2, M-3, M-7), b07
(M-4, M-5, M-6, M-8), b08 (the route-file path). The task brief that built this index said the
review's Importants and Minors "were never batched"; that was true when it was written and
b06 and b07 have since closed the Minors. Nothing of the review itself is left over.

---

## Two open owner questions ride along

Neither is mine to answer; both are recorded, and the recommended option is planned behind a
seam (rule 7).

1. **Q5, already in the plan** (`2026-09-25-phase-6-gateway.md:331`): may a solo project go
   back to Global? Built with the design's rule behind one constant,
   `localProjectsAreSolo`.
2. **New, recorded by this run** (`carried/plan-wide.md` item 5): who compares the AU prices
   with the AWS price list for `ap-southeast-2` before AU budgets are trusted?
