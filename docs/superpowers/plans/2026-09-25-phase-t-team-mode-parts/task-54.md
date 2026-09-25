### Task 54: Desk rows: HORIZON, NEEDS YOU, SINCE YOU LAST LOOKED, DECIDED WITHOUT YOU

**Slice:** TM1. **Size:** 3 focused hours. **dependsOn:** 52 (ST.8's `TeamDesk`, `teamSlice`,
`TeamClient.answer`/`.undo`), 53 (`AuthoredText`), and **phase W task 62's Desk frame** (seam
SW.17, `feat/work-layer`), which the skeleton's row does not list (gap G54-1). **Seam:** consumes
ST.8 and SW.17.

The Desk is account-level and keeps its fixed section order (`docs/AGENTIC-UX.md` §5: *"NEEDS YOU
→ DECIDED WITHOUT YOU → FAILED → LANDED → RUNNING"*, sections never resort under the cursor, and
answering never navigates). Team mode adds **rows to existing sections and no new section**
(`05-design.md` §9.3). There is one Desk, and phase W builds its frame: task 62 of
`.claude/worktrees/work/docs/superpowers/plans/2026-09-25-phase-w-work-layer.md:462`, whose scope
says *"The one Desk (05 §5.6; team mode's TM-54 rows plug into it, never a second Desk)"*
(`…/2026-09-25-phase-w-work-layer-scopes/tasks-61-80.md:67-68`). This task writes four row kinds
and one item source, and registers them with that frame. It builds no frame, no section and no
band of its own.

| Row kind | Lands in (SW.17) | From ST.8 | What it shows |
|---|---|---|---|
| `team.horizon` | band `horizon` | `TeamDesk.horizon` | *"Alice's worker and your mobile worker converge on `auth/session.go`"*, the tier as how it was predicted (*"plans overlap"* P, *"both editing"* H, *"lanes overlap"* M, *"merge conflict"* S), no time (OD-27; task 52 decision c) |
| `team.decide` | section `needs_you` | `TeamDesk.needsYou` | the question; up to 4 options, each with its consequence; the safe default marked; *"decide by 04:40"* in `tabular-nums`; answered in place |
| `team.since` | band `since` | `TeamDesk.sinceYouLastLooked` | **one row per project**: every inform and every free-text answer your Orchestrator sent a peer, as a list under one line, never a toast |
| `team.decided` | section `decided_without_you` | `TeamDesk.decidedWithoutYou` | Fleet's own sentence (`summary`), with `[Undo]` only where `undo.available` |

A teammate's agent never appears on your Desk by itself (§9.3, `05-design.md:997-999`): every row
above comes from **your** Orchestrator's escalations and sent answers, or from Fleet's record of
what your Orchestrator agreed. The source reads nothing else.

#### Decisions this part makes

**(a) The frame is phase W's, and this task registers into it.** SW.17, as W task 62's scope fixes
it (`tasks-61-80.md:72-84`): `DeskSection = "needs_you" | "decided_without_you" | "failed" |
"landed" | "running"`; `DeskBand = "horizon" | "since"`; `DeskItem<T> {kind, id, payload,
arrivedAt, rankReason, ifNothing}`; `DeskRank {tier: 0 | 1 | 2; waitingSinceMs}`;
`registerDeskRow<T>(kind, section, render, rank)`; `registerDeskSource(kind, select: (snap:
DeskSnapshot) => DeskItem[])`. Registering one kind twice throws there, which is what stops this
task silently replacing an issue row. `DeskSnapshot`'s shape is W-62's part to write and is
**UNVERIFIED** here; this task assumes it exposes the cockpit store's state, so the source reads
`snap.teamDesk` (task 52's slice key). If W-62 names it differently, the one `select` function
changes and nothing else.

**(b) Ranks.** `team.decide` is tier `1` (*"blocks someone"*: a worker waits on it), and tier `0`
when any option's consequence is marked irreversible; ST.8 carries no such mark, so in TM1 it is
always `1`, and `waitingSinceMs = now − at`. `team.decided` is tier `2`, `team.horizon` and
`team.since` are bands and are not ranked. `ifNothing` for a decide is the default's label:
*"if you do nothing: Let Alice go first, at 04:40"*. It is Fleet's default, not a prediction.

**(c) Answering is optimistic, with an Undo window in the row, not a toast.** `docs/AGENTIC-UX.md`
§5 and `docs/DESIGN-SYSTEM.md`'s interaction standards ask for optimistic mutation with an Undo
window rather than a confirm dialog. The Hub accepts **one** answer (`task-01.md:427`: a second is
refused), so the window is local: choosing an option turns the row into *"You chose: Let Alice go
first · Undo"* for 6 s (the `Toast.ttl` phase 2 uses for an undoable action, `store/types.ts:217`),
then sends it, **once**, through `beginAnswer`/`endAnswer`'s guard (task 52). Undo within the
window sends nothing. The row is not removed while the Desk is open (the frame never re-ranks
while open); it stays in its answered state until the next visit.

**(d) SINCE YOU LAST LOOKED is batched per project, and "mark all seen" moves one cursor.** Twenty
informs are one row per project with twenty lines, oldest first, each `AuthoredText inline`. The
band's own *"mark all seen"* (W-62's control) calls `markTeamSeen(generatedAt)` as well; the next
`desk(since)` then returns nothing older.

**(e) The deadline is a clock time.** *"decide by 04:40"*, Australia/Sydney, `tabular-nums`, never
*"in 18 min"*: a countdown is a live ETA in all but name (OD-27). Aging uses phase 2's existing
`formatWaiting` (`lib/time.ts:49`), *"waiting 5h 51m"*, which `docs/AGENTIC-UX.md` §6 item 7 asks
for and OD-27 allows (it is a measured duration, not a prediction).
