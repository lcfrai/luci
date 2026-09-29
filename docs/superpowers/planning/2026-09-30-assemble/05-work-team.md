# 05 — Work layer and team collaboration: what to adopt instead of build

**Date:** 2026-09-30. **Status:** research in progress, nothing adopted.
**Capability:** Phase W (work layer, 190 tasks, plan only) and much of Phase T (team mode, 69
tasks, plan only): issues, projects, cycles, boards, Linear-class speed, presence, notifications,
seeing each member work, agents taking issues.
**Owner direction (2026-09-30):** third-party first, the tool ASAP, days not weeks.

## The answer

_Pending — filled in last._

## 1. What Phase W and Phase T actually ask for

Read from the plans (grep only) on 2026-09-30:
`.claude/worktrees/work/docs/superpowers/plans/2026-09-25-phase-w-work-layer.md` and
`.claude/worktrees/team/docs/superpowers/plans/2026-09-25-phase-t-team-mode.md`.

**Phase W — 190 tasks, 528.5 planned hours, ~66 focused days** (the plan's own table, lines
339-351; the design's figure is ~138 days for one implementer, `OD-26`). A PanaceaApp-native,
Linear-parity-plus tracker: W1 (70 tasks) issues on the thread, Work lens, issue view, keys,
replica, Desk, Linear/CSV import, `Shift+R` Start; W2 (28) cycles, views, rank, filters; W3 (16)
agents on the work; W4 (29) roadmap, release, reporting; W5 (32) intake and ecosystem; WL (13)
later rows. Its goal line: *"A team moves its backlog into PanaceaApp and never goes back."*

**Phase T — 69 tasks** in three slices: TM1-α (tasks 01-33) the Orchestrator runner, the `OD-23`
policy mechanism, Fleet as a delegated Hub caller; TM1 (34-57) activity and presence, leases,
budgets, the team strip, Desk rows, the Orchestrator panel; TM2 (58-69) the lane mirror, team
land queue, quotas.

Split by who could supply it:

| Need | Phase / tasks | Commodity (a product can own it) | Ours (no product has the objects) |
|---|---|---|---|
| Issues, sub-issues, labels, states, comments, mentions, search | W1 02-17, 37-50 | **Yes** — every tracker | — |
| Cycles/sprints, saved views, filters, rank, board, timeline, roadmap | W2, W4 | **Yes** | — |
| Import from Linear/CSV | W1 15, 25, 69; W5 150-153 | **Yes** — or unnecessary if the customer's tracker *is* the store | — |
| Intake, magic words, linkbacks, webhooks, Slack/Teams | W5 | **Yes** | — |
| Notifications, mentions, "since you last looked" | W1 62-65, TM 54 | **Mostly** — Teams + tracker notifications | Desk wording rules (`OD-27`) |
| Presence: who is on the repo, what files | TM 34-35, 52-53 | **Transport yes** (pub/sub) | The facts (observed files, sessions) come from our control plane |
| An agent takes an issue | W1 24, W3 98-113 | **The trigger yes** (assign/delegate/@mention to an agent identity) | The sandboxed managed session, the worktree, the gateway token |
| Issue `serves` a brief criterion; done means proven (`OD-27`) | W1 02-10, 21-23b | No tracker models a criterion | **The provenance edge and the evaluator** |
| Orchestrators negotiating across owners (`OD-14`-`OD-17`) | TM1-α 23-31, TM1 36-42 | Nothing, open or closed | **All of it** |
| Leases on files, a turn queue, the policy file in every sandbox | TM 13-15, 23-26 | Parts (a KV with TTL) | The rules |

The line that decides the recommendation: **about three quarters of Phase W is a tracker, and
KPMG already pays for two.** The provenance edge, the done evaluator and the Orchestrators are
the product; the tracker is not.

## 2. Candidates — trackers KPMG already runs

_Pending._

## 3. Candidates — SaaS trackers to buy

_Pending._

## 4. Candidates — self-hosted trackers

_Pending._

## 5. Candidates — presence, notifications, team surfaces

_Pending._

## 6. Recommendation

_Pending._

## 7. What we still build (glue)

_Pending._

## 8. Owner calls

_Pending._

## 9. What was not verified

_Pending._
