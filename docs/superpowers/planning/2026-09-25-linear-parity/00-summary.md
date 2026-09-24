# 00 — Summary: Linear parity, plus what Linear cannot follow

**Date:** 2026-09-25. **Status:** design for the owner's review; nothing is built. **Branch:**
`feat/phase-2-cockpit`. **Full design:** `05-design.md` in this folder.

**Owner's ask:** *"make sure that we are doing everything they are doing and more. Make it
prettier, better, everything … and the ability for agents across owners to work together."*

## Decide these two first

1. **Q1, progress displays.** Linear shows percentages, burn-ups and predicted dates;
   `AGENTIC-UX.md` bans them. **Recommend:** convergence per criterion plus plain group sizes, and
   nothing that predicts. It needs a one-line amendment to `AGENTIC-UX.md:245`.
2. **Q2, how far to build.** **Recommend W1 to W3 first (~64 focused days)**, then decide W4 and W5
   on what the first teams use.

## The answer

- **We build the work layer, not an integration.** An issue is a new Hub object that points at the
  brief criteria it serves, so work sits **on** the provenance thread (04's recommendation, kept).
- **Parity, counted over all 257 Linear items:** **MATCH 155 · EXCEED 75 · SKIP 27.** Every skip
  has a reason: 11 are support-desk, CRM and sales features; the rest are embeds and connectors
  that defeat egress control, grants Hub v1 lacks, banned predictions, mobile and web apps, and
  non-build items.
- **One app, not a tracker bolted on.** Work is a seventh lens and a project surface; every other
  lens shows an issue chip; the Desk absorbs Linear's Inbox and My Issues. Lower-case keys edit
  the record; `Shift+S` Starts an agent, `Shift+D` Delegates to your Orchestrator.
- **Prettier** is 02's uplift, adopted: generated themes, the thread as our signature, mono for
  machine facts, our own state glyphs that never fill with progress, a not-a-clone review test.
- **Faster** is architecture: an in-memory Hub replica on the Hub's change feed, optimistic undo
  for reversible acts, in-row confirmation for irreversible ones.

## The "more": four things that need our platform, not a screen

| The more | What a person sees | Lead over Linear (03's estimate) |
|---|---|---|
| **Agents as teammates** | Start or Delegate; a first sign of work within 10 s; one live line; agents *propose* evidence and a person records it; **done means proven**, not "PR merged" | months |
| **Orchestrators across owners** | collisions caught at plan time; typed negotiation (split, sequence, yield); inform or decide on your own Desk; nobody steers another person's agent | 6-12 months (GitHub best placed) |
| **One thread** | brief → criterion → design frame → issue → session → commit trailer → land → evidence → release | 2-3 quarters for the back half; the design canvas is the slow part |
| **ZDR inference, AU switch** | Global or AU per project; cost per user, issue and loop | weeks: hygiene alone, strong in combination |

No rival has more than two of the five parts of the whole line (03). The combination is the product.

## Delivery (focused days, one implementer, not measured)

| Slice | Promise | Days |
|---|---|---|
| **W0** now | `issue:` in the phase 4 key parser and Hub grammar; fix the stale `done:2` example | < 0.5 |
| **W1** minimum lovable | file it, `Shift+S`, it lands, a person records evidence, it turns `done`, the release shows it shipped | ~27.5 |
| **W2** planning | cycles, features, sub-issues, comments, views, generated themes | ~24.5 |
| **W3** agents on the work | Delegate, Orchestrator notes, proposed evidence, negotiation on issues (with TM1) | ~11.5 |
| **W4** roadmap and release | initiatives, timeline, updates, notes by criterion, insights, AI assists | ~22.5 |
| **W5** intake and ecosystem | requests, Azure Boards / GitHub / Jira sync, Teams, API, MCP, Loops | ~30.5 |
| | **Total** | **~117** |

W1 is about 3 weeks elapsed on three streams; the desktop is the long pole. All plan changes are
additive and listed in `05-design.md` §5.5; the work arrives as new Hub tasks after task 43.

**One gap found:** the Desk is the product's landing surface and TM-05 adds rows to it, but **no
plan builds it**. W1 builds a minimum (Q4).

## The other eight questions

Before W1: **Q3** issue text in the agent's context (yes, with guards) · **Q4** who builds the Desk
(Phase W's minimum in W1) · **Q5** replica in memory or on disk (memory; measure first).
Later: **Q6** keep the 27 skips · **Q7** call Linear's project a *feature* · **Q8** no title slug in
AU-only branch names · **Q9** build and also sync · **Q10** accept 02's four design picks.

## Unverified

Every size; Linear facts read through a summarising fetch tool (plan tiers, shortcuts, whether an
acceptance-criteria object or a ZDR option exists); that no plan builds the Desk; solo-mode
verbs; 02's speed budgets with an in-memory replica. No web page was read for 05; its Linear facts
come from 01 to 04, which list their URLs (all read 2026-09-25). Visual acceptance is the owner's
eyes.

## Files

`01-linear-feature-inventory.md` (257 items) · `02-linear-design-language.md` (design craft) ·
`03-agent-work-landscape.md` (ten products) · `04-internal-fit.md` (where it fits) ·
`05-design.md` (this design: matrix, the more, object model, UX, delivery, questions).
