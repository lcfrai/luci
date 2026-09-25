# Phase W task scopes: tasks 61 to 80

Scope paragraphs for tasks 61-80 of `2026-09-25-phase-w-work-layer.md` (W1: Start on the desktop,
the Desk frame and its rows, the accessibility floor, issue chips, create and import, the
local-mode demo; W1b: *shipped*; W2: the planning wire, the fake, the migration and the Hub's
planning aggregates). A part writer reads one paragraph and the skeleton's row; where they
disagree, **the row's id, dependsOn and seams win** (skeleton, *Task scopes*).

**Conventions used below.**

- Paths are relative to the repository root; desktop paths are under `desktop/`, Hub paths under
  `control-plane/internal/hub/`. A path marked ⚑ is shared (skeleton *File map*): the tasks that
  touch it run in task order, never in parallel worktrees.
- "05 §x" is `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2);
  "L-n" is row n of the parity matrix (`01-linear-feature-inventory.md`, priced in 05 §1). A row a
  task **closes** is built there; a row it only **serves** is closed by the task named.
- Seams are the skeleton's `SW.n` (*Seams* table). Every desktop surface test calls
  `expectNoPrediction(container)` (SW.8, task 20) and reads Work data through the replica
  (`useWork`, `mutate`, `onRefused`, SW.16) or, before task 55 lands, `workFixtures` (SW.8).
- "Mutant" names the one wrong implementation a test exists to kill.
- Hub server paths (`server/`, `store/`, `migrate/`) follow the Hub plan's file map and are not
  on `feat/hub` yet (skeleton *File map*, rule H1): a part cites them as planned. A sibling path
  named only by its owner's plan is **UNVERIFIED until merge** (rule H2).
- Desktop tasks follow the skeleton's *Global constraints*: list-focus keys only, one heartbeat,
  `tabular-nums`, `transform`/`opacity` motion, facts in Commit Mono at `--t-fact` (`OD-28`), no
  Hub data on disk, and no predicted number anywhere (`OD-27`). Tests run with
  `npm --prefix desktop test` and `npm --prefix desktop run typecheck`; Go with
  `go test ./control-plane/...` (Hub integration tests with `-tags integration`).

**61 — Start on the desktop: `Shift+R`, the *Undo Start* chip, *Stop and discard*, the 10 s first
sign.** The desktop half of SW.10 (task 24 owns the control-plane route in
`control-plane/internal/api/workstart.go` and `internal/workstart/{start,undo}.go`; its part fixes
the path and bodies, and the working names here, `start(issueId, revision)`,
`undoStart(sessionId)` and `stopAndDiscard(sessionId)`, yield to it). Files:
`desktop/src/components/work/StartChip.tsx` (+ test), `desktop/src/work/start/useStart.ts` (+ test),
`desktop/src/api/work.ts` ⚑ (the three relay calls and a `StartResult {sessionId, worktreeId,
route: "global" | "au", startState: "undoable" | "discard_only", createdAt}`), and one binding in
`desktop/src/work/keys/keymap.ts` (task 53's map). `Shift+R` fires only with list focus
(05 §4.3 rule 1) and tests `event.code === "KeyR" && event.shiftKey` (rule 4); on an unassigned
issue it first assigns me through `mutate` (SW.16, one `optimistic.undo` write), on my issue it
starts, and on another person's issue it does nothing but say so in place (*"EP-142 is Alice's.
Only her agents work on it."*, OD-16, rule 7). The row resolves at once with the chip (05 §4.6):
*Undo Start* for as long as `startState` is `undoable`, **past the 6 s window** and with no
countdown, then *Stop and discard* as a `confirm.inplace` two-step act. *Undo Start* shows what the
start cost as a measured fact only; in local mode there is no gateway meter, so it reads *"cost not
metered in local mode"*, never a guess. The first sign reads the session record's `lastOutputAt`
(`control-plane/internal/registry/session.go:99`): no output 10 s after `createdAt` shows
*"no output yet · 10s"* in mono (02 C8 `budget.ack`), and output shows *"last output 3s ago"*; both
tick on the one heartbeat (`desktop/src/hooks/heartbeat.ts`), never a per-row timer or spinner.
The bulk bar's *Start N worktrees* (task 44) calls this one `useStart().start` per id. **Tests
first:** (1) `Shift+R` with `key: "К"`, `code: "KeyR"` starts once with the row's id and pinned
revision (mutant: matches `event.key`); (2) Caps Lock `R` without `shiftKey` starts nothing
(mutant: case-folds the key); (3) `Shift+R` inside a `data-terminal-host` does nothing (mutant: no
`isInsideTerminal` check); (4) another person's issue calls no route (mutant: Start for any row);
(5) with fake timers at 30 s and `startState: "undoable"` the chip still reads *Undo Start*
(mutant: chip bound to `--undo-window`); (6) `discard_only` needs two presses before
`stopAndDiscard` is called (mutant: one press discards); (7) no `lastOutputAt` at 10 s shows the
fact line, at 9 s shows nothing (mutant: an elapsed bar or `%`, also caught by
`expectNoPrediction`); (8) `setInterval` is never called by the row (mutant: a per-row timer);
(9) a refused start rolls the assign back through `onRefused` and shows the Hub's reason (mutant:
the row stays assigned). **Depends on** 24, 56. **Design:** 05 §4.3 rules 1-4, §4.6 (*Undo,
revision 2*), §2.1 point 1, §5.3 (the demo's `Shift+R`). **Closes** L-47's *accept and Start*
half (with task 42), L-39's solo half, L-181 and L-198's W1 half; **serves** L-36 (Start's undo)
and L-190 (closed in W3). **Does not claim** cost figures in local mode, Delegate (`Shift+O`, W3),
or the budget as measured (recorded, not asserted, 05 §4.6).

**62 — the Desk frame: five fixed sections, never live while open, the row-kind registry.** The one
Desk (05 §5.6; team mode's TM-54 rows plug into it, never a second Desk). Files:
`desktop/src/components/desk/{Desk,DeskSection,DeskZero,deskModel,deskRows}.tsx` and `desk.css`
(new; `deskModel` holds no JSX and the part may name it `.ts`), `desktop/src/store/types.ts` ⚑
(`Lens` gains `"desk"` beside task 37's `"work"`, `types.ts:173`), `desktop/src/components/AppShell.tsx`
⚑ (one `LensRouter` arm, `AppShell.tsx:17`). SW.17, fixed here:
`type DeskSection = "needs_you" | "decided_without_you" | "failed" | "landed" | "running"`, in that
order (AX §5), plus two bands, `type DeskBand = "horizon" | "since"`: `horizon` renders nothing in
W1 (the horizon bar has no slice, Q4) and `since` (*SINCE YOU LAST LOOKED*, where §3.7's informs
land, as `prototype/work-layer.html:786` shows); `interface DeskItem<T> {kind: string; id: string;
payload: T; arrivedAt: string; rankReason: string; ifNothing: string}`; `interface DeskRank {tier:
0 | 1 | 2; waitingSinceMs: number}` (0 irreversible, 1 blocks someone, 2 the rest; AX §5's
*"ranked first: irreversible"*); `registerDeskRow<T>(kind, section: DeskSection | DeskBand,
render: (item: DeskItem<T>) => JSX.Element, rank: (item: DeskItem<T>) => DeskRank): void`; and,
because the skeleton's signature names no item source, `registerDeskSource(kind, select: (snap:
DeskSnapshot) => DeskItem[])`. The frame takes one snapshot on entry and **never re-ranks while
open**: an arrival shows the hold line *"A new item arrived. The Desk does not re-rank while you
read it."* and `N` applies it (05 §4.1, `arrivals.hold`); leaving and returning re-ranks. Section
headers carry a plain count in `tabular-nums` (the one place counts are allowed, 05 §4.4 rule 3);
an empty section keeps its header and a one-line zero (*"Nothing failed."*), so FAILED is never
hidden behind a calm state (07 D C5). `j`/`k` walk every row across sections and `Enter` dives,
through task 53's list-focus keymap. **Tests first:** (1) kinds registered in reverse order render
in AX order (mutant: registration order); (2) registering one kind twice throws (mutant: last
wins, which would let TM-54 silently replace an issue row); (3) an item offered while mounted is
not in the DOM until `N` (mutant: live re-render); (4) within a section, tier then oldest first
(mutant: newest first); (5) unmount and remount shows the held item ranked (mutant: the snapshot
survives the visit); (6) an empty FAILED still renders its header (mutant: empty sections hidden);
(7) `j` on the last NEEDS YOU row lands on the first DECIDED WITHOUT YOU row (mutant: `j` stops at
a section edge); (8) `expectNoPrediction` passes on a full fixture Desk. **Depends on** 37 (it reads
`workSlice` and fixtures; task 63 moves it onto the replica). **Design:** 05 §4.2, §5.3 row 16,
§5.6; AX §5 *Rules for the Desk*. **Closes** L-114 (with 63-65); **serves** L-112 and L-113
(closed by 65). **Does not claim** the horizon bar or quiet mode, any row content (63-65), or
making the Desk the landing surface before the owner's review (task 39); it adds the route only.

**63 — the Desk: issue rows in NEEDS YOU, aging, *blocking someone's agent*.** The first row kinds
in task 62's frame, and the move of the frame's snapshot from `workSlice` onto the replica
(`useWork`, SW.16). Files: `desktop/src/components/desk/deskRows.tsx` ⚑ (with 64, 65, 71 in task
order), `desktop/src/components/desk/rows/IssueNeedRow.tsx` (+ test),
`desktop/src/work/desk/needs.ts` (+ test: the pure selectors). Four kinds register under
`needs_you` (05 §4.2): `issue.triage_mine` (state `triage`, `assignee` is me), `issue.urgent_mine`
(`priority: "urgent"`, assignee me, not closed), `issue.mention` (an unresolved `Comment` whose
`mentions` names me; W1 does not infer "a question", so any unresolved mention qualifies), and
`issue.blocking` (my issue X `blocks` Y, X not `landed`, Y in `started` or `in_review`, i.e. a lane
is bound to it: *"EP-150 blocks EP-142 (Alice), started 1h 12m ago"*). The `in_review`
*"where you are the reviewer"* row is **not built**: SW.1's `IssueHead` has no reviewer field
(task 03's scope), which this file reports as a gap. Each row carries `rankReason` and `ifNothing`
(AX §5, L-114): blocking is tier 1, the rest tier 2; the urgent row earns `--need` and
`<MarkGlyph kind="needs_you"/>` here and only here (05 §4.4 rule 2: priority is otherwise a notch
in ink). **Age** (*"waiting 2h 10m"*, mono, `tabular-nums`) runs from the SW.5 feed event that made
the row true, else the head's `updatedAt`, and ticks on the one heartbeat. Row verbs reuse what
exists: triage's `1` accept and `4` snooze (task 42's actions, with Triage's own policy check),
`I` assign to me, `Enter` opens the issue view beside the Desk. **Tests first:** (1) a `triage`
issue assigned to Alice makes no row of mine (mutant: every triage issue); (2) an urgent row has
`--need`, a `high` one assigned to me has no row (mutant: priority as colour); (3) with the feed
event at 09:00 and the clock at 11:10 the age reads `waiting 2h 10m` and survives a re-render
(mutant: age from first render); (4) X blocks Y with Y in `todo` makes no row, Y `started` makes a
tier-1 row (mutant: any `blocks` relation); (5) after X lands, the row is gone on the next Desk
entry and still present while open (mutants: stale row; live removal under the reader); (6) a
resolved mention makes no row (mutant: ignores `resolvedAt`); (7) the blocking row ranks above an
older urgent row (mutant: age before tier); (8) `expectNoPrediction` on the section. **Depends on**
55, 62. **Design:** 05 §4.2 (NEEDS YOU row), §4.4 rule 2, §3.3; AX §5, §7.3. **Closes** L-15 and
L-24's Desk half (the relation itself is task 11/48); **serves** L-46 (items naming a person join
their Desk; closed by 42), L-112 and L-114. **Does not claim** the reviewer row, a `decide`
escalation from an Orchestrator (W3, TM-54), the cost of waiting in money (no gateway in W1; the
wait is shown as time), or inferring questions from text.

**64 — the Desk: FAILED with *Restart from checkpoint* and *Abandon*; LANDED; RUNNING's fold.**
Files: `desktop/src/components/desk/deskRows.tsx` ⚑, `desktop/src/components/desk/rows/{FailedRow,
LandedRow,RunningCard,RunningFold}.tsx` (+ tests), `desktop/src/work/desk/agents.ts` (+ test).
**FAILED** (`lane.failed`, 07 D C5): a session bound to my issue (`Session.IssueRef`, task 19) that
is `status: "exited"` with a non-empty `exitReason` (`registry/session.go:88-89`; the reaper sets it,
`reaper/reaper.go:636`) that is not my own stop; the row states the fact (*"exit at 02:14 ·
<exitReason>"*, mono) and two verbs: *Restart from checkpoint* calls phase 2's
`restartSession(id)` (`desktop/src/api/client.ts:153`, route `sessions.go:723`), which resumes in
the same worktree, so its commits are the checkpoint; *Abandon* is `confirm.inplace` with the
consequence line (*"the worktree and its uncommitted changes are deleted"*) and goes through SW.10's
`stopAndDiscard`; the session record stays listed (FUNCTIONAL-SPEC). An intake-rule failure joins
FAILED in W5. **LANDED** (`issue.landed`, `issue.done`): *"EP-142 landed and proven at 14:02"*
for observed evidence, *"proven, self-attested"* for a solo Record, *"done, not proven"* in its
glyph for a hand-set done, and *"done · c-91e2 still open (EP-150 not landed)"* when a served
criterion is not settled (05 §3.3), words from SW.4's result; a criterion-settled row appears only
where phase 4's loader gives the convergence word (P4-16, **UNVERIFIED until merge**). *Mark seen*
is `optimistic.undo`. Task 71 adds *shipped* here. **RUNNING** (05 §4.2, revision 2): an agent of
mine on my issue that **changed since my last Desk visit** (a new status, `idle`, or no output for
over 10 minutes by `lastOutputAt`) gets AX §5's card of exactly three lines: STATE (status word
and *"running 41m · last output 3s ago"*), MEANING (the issue alias and title as `text·H`; W1 has
no gateway to write a sentence and never reads PTY bytes, OD-8), NEXT (the next fact it waits on,
by rule, never a time, `OD-27`). The steady rest fold into one line, *"23 steady, all live"* (or
*"21 steady, 2 idle"*), that expands in place with `aria-expanded`. The last-visit time is held in
memory, a preference, not Hub data. **Tests first:** (1) an exited session I stopped makes no FAILED
row (mutant: every exit is a failure); (2) *Restart* calls `restartSession` once and the Desk does
not navigate (mutant: routes to Develop); (3) *Abandon* needs two presses and shows the consequence
line (mutant: one press); (4) the three LANDED wordings each render from their SW.4 result and
never swap (mutant: every landed row says *proven*); (5) output 11 minutes old gives a card of
exactly three lines, output 3 s old folds (mutants: four lines; no fold); (6) the fold counts only
steady agents and says `idle` when one is (mutant: *"all live"* regardless); (7) no row shows
`~4m`, a `%` or a bar (`expectNoPrediction`; mutant: AX's *"will need you in ~4m"* copied).
**Depends on** 62 (and on 19 and 24 through their seams). **Design:** 05 §4.2 (*RUNNING's shape*),
§3.3, §4.6; AX §5; 07 D C5. **Closes** L-114's FAILED, LANDED and RUNNING sections (with 62, 63,
65); **serves** L-113, L-184 and L-189 (both *exist*). **Does not claim** the issue's state after
*Abandon* (05 names no fact move back from `started`, a reported gap), a generated MEANING line
(W4 gateway), or that 30 agents fit one screen at 125 % (measured in task 70).

