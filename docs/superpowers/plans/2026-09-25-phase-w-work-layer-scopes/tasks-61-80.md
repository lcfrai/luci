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

