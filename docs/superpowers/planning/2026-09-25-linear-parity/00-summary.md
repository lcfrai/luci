# 00 — Summary: Linear parity, plus what Linear cannot follow

**Date:** 2026-09-25. **Status:** design for the owner's review, **revision 2**, answering both
critiques (`06-critique-product.md`, `07-critique-design.md`); nothing is built. **Branch:**
`feat/phase-2-cockpit`. **Full design:** `05-design.md`; §8 there maps every finding to its fix.

**Owner's ask:** *"make sure that we are doing everything they are doing and more. Make it
prettier, better, everything … and the ability for agents across owners to work together."*

## Decide these three first

1. **Q11, the order against OD-17** (team mode next). **Recommend B:** W1 runs beside TM1 on two
   added streams. Its demo runs in local mode about 4-5 weeks in; TM1's gateway and phase 8 streams
   then give it the cloud. TM1's critical path is untouched.
2. **Q2, how far to build.** **Recommend W1 to W3** (~82.5 focused days, plus TM1's ≈95.5, which W3
   needs): about 10-12 weeks elapsed to agents across owners negotiating on issues.
3. **Q1, progress displays.** **Recommend** convergence per criterion plus plain group sizes. Only
   completion predictions stay banned (a percentage, a bar, an ETA, "N of M"). It comes with Q13's
   four one-line amendments to `AGENTIC-UX.md`.

## The answer

- **We build the work layer, not an integration.** An issue is a Hub object that points at the
  brief criteria it serves, so work sits **on** the provenance thread.
- **Parity, counted over all 257 Linear items:** MATCH 139 · EXCEED 67 · **PARTIAL 24** · SKIP 27.
  Priced per stop: **W1 49 complete · W3 115 · W5 188**, the rest *later* or through other phases.
  Skips: 11 support-desk, CRM and sales; 4 embeds and connectors that defeat egress control; 3
  need grants Hub v1 lacks; 3 platform choices (mobile, web, disk); 2 banned predictions; 2
  non-build; partner agents that bring their own inference (L-195); GitLab, outside OD-4 (L-205).
- **Not Linear in teal.** The Work lens is **grouped by criterion**, so its skeleton is the thread.
  Rows never move under the reader. A row carries one agent line, yours; a teammate shows as a
  person and the files they touch, never as dots.
- **Done means proven, fixed.** An issue is done on its **own** landed work and **own** evidence,
  observed by the platform where it can be, so no click. A hand-set done reads *done, not proven*,
  and sync can never write it.
- **Keys match Linear's letters on purpose.** No agent key sits on a chord Linear uses: `Shift+R`
  runs an agent, `Shift+O` delegates to your Orchestrator.
- **Prettier, in W1, before the owner's review:** the dim chrome, mono facts at 12px, about 12 drawn
  glyphs, and faces chosen by eye (Q10: Berkeley Mono, ~$100 licence, or Commit Mono, free; Q14:
  Segoe UI Variable recommended over Geist). A contrast-theme and screen-reader floor is included.

## The "more": four things that need our platform, not a screen

| The more | What a person sees | Lead over Linear (03) |
|---|---|---|
| **Agents as teammates** | Run or delegate; a first sign of work within 10 s; one live line; observed proof closes the issue | months |
| **Orchestrators across owners** | collisions caught at plan time; typed negotiation; inform or decide on your own Desk; nobody steers another's agent, and an edit to someone's issue reaches them, never their agents | 6-12 months |
| **One thread** | brief → criterion → design frame → issue → session → land → evidence → release | 2-3 quarters |
| **ZDR inference, AU switch** | Global or AU per project; cost per user, issue and loop | weeks alone |

The whole line has five parts: brief, design, per-person agents, cross-person negotiation, and
release on one thread. No rival has more than two (03). The combination is the product.

## Delivery (focused days; elapsed under Q11 B; none measured)

| Slice | Promise | Days | Elapsed |
|---|---|---|---|
| **W0** now | `issue:` in the key grammar; fix the stale `done:2` example | < 0.5 | now |
| **W1** adoptable | import a Linear backlog; comments, Markdown, the new look; Run on any row; landed and proven on the Desk | ~48.5 | ~4-5 wk, local mode |
| **W1b** | *shipped to production, 14:02* on the issue | ~1 | with phase 7 |
| **W2** planning | cycles, features, sub-issues, views, custom states, https links, themes | ~20.5 | beside TM1's tail |
| **W3** agents on the work | delegate, notes, proposed evidence, negotiation on issues | ~12.5 | after TM1: ~10-12 wk |
| **W4** roadmap | initiatives, timeline, notes by criterion, insights, AI assists, cross-team | ~26.5 | |
| **W5** ecosystem | requests, one-way sync plus comments, Teams, API, MCP, Loops | ~28.5 | ~15-18 wk |
| | **Total** (plus TM1 ≈95.5, phase 6 and phase 8, not Phase W's) | **~138** | |

W1 grew from 27.5 days because it is no longer a demo (05 §5.1 itemises the +21). One Desk is
built, in W1; TM1 adds its rows.

## The other questions

Before W1: **Q3** issue text in the agent's context (yes, with guards) · **Q4** the Desk frame in
W1 · **Q5** replica in memory · **Q10** the mono · **Q13** the four AX amendments · **Q14** the
sans. Later: **Q6** keep the 27 skips · **Q7** call Linear's project a *feature* · **Q8** no title
slug in AU-only branch names · **Q9** build, and sync one way in · **Q12** cross-team in W4.

## Unverified

Every size and elapsed figure; local mode's inference once the gateway exists; that Linear binds
nothing on `Shift+R`, `Shift+O` or `Shift+A`; the 2.3:1 contrast figure; Atkinson's licence; that
`/v1/changes` filters per caller. No web page was read for this revision; Linear facts come from
01 and 02 (URLs read 2026-09-25). Visual acceptance is the owner's eyes.

**Files:** `01` 257 items · `02` design craft · `03` ten products · `04` internal fit · `05` the
design · `06`, `07` the critiques it answers.
