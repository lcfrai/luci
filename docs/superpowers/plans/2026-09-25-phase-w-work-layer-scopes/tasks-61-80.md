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

**65 — the Desk: §3.7's informs and the reassign *decide*; answer in place; the zero state.** Files:
`desktop/src/components/desk/deskRows.tsx` ⚑, `desktop/src/components/desk/rows/{InformRow,
ReassignDecide}.tsx` (+ tests), `desktop/src/components/desk/DeskZero.tsx` (task 62's file, now
filled), `desktop/src/work/desk/informs.ts` (+ test). **Informs** register under the `since` band
(`prototype/work-layer.html:786`): (a) §3.7 rule 1, derived locally with no Hub record: a session of
mine whose `IssueRef` names `issue:is-…@3` (task 19) while the head is `@4`, written by someone
else, gives *"EP-142 changed since your agent read @3 (Alice, 11:02): serves c-40aa added."* and one
verb, *re-brief my agents at @4*, which moves the session's `IssueRef` through task 19's path so the
Materialiser rewrites the managed block (SW.7); nothing reaches the agent until I press it, and
whether a running CLI re-reads the block before its next turn is **UNVERIFIED** (we never type into
it, OD-8); (b) §3.7 rule 3's solo form: a relation someone else added to my in-flight issue is
recorded and informs me; (c) §3.3's drift: a served `criterionId` no longer in the brief's current
revision (04:263) informs with a one-key *re-point* (P4-16's loader, **UNVERIFIED until merge**).
**The reassign decide** (§3.7 rule 2) sits in NEEDS YOU at tier 0: when Alice reassigns my issue
while a lane is bound (`started` or `in_review`), I get *hand over the worktree to Alice* or *keep
working, decline* (keys `1`, `2`); the safe default pauses and moves nothing, and every surface
shows *reassign requested by Alice* until I answer; in solo mode, hand-over restarts nothing: my
session gets a checkpoint commit and stops. **Its Hub half is a gap** (reported): SW.1 has no
pending-reassign record, so this task builds against `NewFake()` with two added methods,
`RequestReassign(issueId, toUserId)` and `AnswerReassign(issueId, accept bool)`, and an added
`IssueHead.reassignRequest *{to User; by User; at string}`, all allowed by WS0's *added method or
field* rule, and marks the real route UNVERIFIED until a WH task owns it. **Answer in place** (AX
§5, L-113): every Desk verb resolves the row where it stands through `mutate` (`optimistic.undo`,
`U`), a refusal returns it with a `--fail` flash and the Hub's reason (02 C8), a one-line reply to a
mention posts a `Comment` inline, and nothing navigates; `H` defer with a reason is W2 (task 94).
**The zero state**, only what W1 can back: *"Nothing needs you. EP-142 landed and proven at 14:02.
3 agents live on 2 issues."*, no horizon line. **Tests first:** (1) a newer head revision written
by me makes no inform (mutant: any revision gap); (2) *re-brief* is the only path that changes the
session's `IssueRef` (mutant: the head change auto-re-briefs, which breaks rule 1); (3) a reassign
on a `todo` issue is a plain assign, on a `started` one it is a decide and the assignee is unchanged
until answered (mutants: decide for every reassign; assignee moved at once); (4) *keep working*
leaves the session running and clears the request (mutant: the default pauses); (5) answering the
last NEEDS YOU row shows the zero sentence and a non-empty FAILED stays visible below it
(mutants: zero unreachable; FAILED hidden, 07 D C5); (6) no verb changes the route or lens (mutant:
a verb that dives); (7) the zero sentence passes `expectNoPrediction` and has no `~` time (mutant:
AX's *"Next decision expected ~14:22"* copied). **Depends on** 63, 64 (and 19 through SW.7).
**Design:** 05 §3.7 rules 1-3, §3.3 (drift), §4.2 (answer in place, zero state); AX §5, anti-pattern
16. **Closes** L-112 and L-113 (with 62-64); **serves** L-114. **Does not claim** rule 3's
Orchestrator proposal (W3), the horizon bar or quiet mode (Q4), defer with a reason (task 94), or
that the reassign's Hub route exists.

**66 — the accessibility floor: one live region, `aria-hidden` liveness, grid roles.** Files:
`desktop/src/components/common/LiveRegion.tsx` (+ test, new), `desktop/src/components/AppShell.tsx`
⚑ (mounts it once), `desktop/src/components/common/Toasts.tsx` (phase 2's toast stack already
carries `aria-live="polite"` at `Toasts.tsx:42`; this task routes its text through `LiveRegion`
and removes the second region), `desktop/src/components/common/StatusDot.tsx` (a `decorative`
prop: `aria-hidden="true"` and no `aria-label`, `StatusDot.tsx:58`, for Work and Desk rows where
the row's STATE word carries the status), and the row containers of tasks 40, 42 and 62-65
(`WorkLens`, `IssueRow`, `TriageView`, `Desk`, `DeskSection`) for roles; neither `Toasts.tsx`
nor `StatusDot.tsx` is in the skeleton's file map (reported). The API: `announce(text: string,
kind: "answer" | "digest"): void` from `useLiveRegion()`: an **answer** to the user's own act is
spoken at once (*"EP-142 deferred. Undo, U"*); a **digest** (held arrivals from task 41, Desk
arrivals from task 62) is coalesced to at most one per 15 minutes (AX §6.8; 05 §4.4 rule 5). The
heartbeat, dots and flashes are `aria-hidden` (05 §4.4 rule 5). Lists are ARIA grids (Work list,
Triage, the board's columns as `listbox`) with `aria-rowcount` set to the full count because they
virtualise over 50 rows, and each rendered row carries `aria-rowindex`; the Desk is one grid per
section. The **Undo pause** (05 §4.6, §5.3 row 17): the `--undo-window` countdown on task 56's chip
stops while its row has focus or hover and resumes on leave; no skeleton row names it, so it lands
here if task 56's part has not built it (the part checks first and never builds it twice). The
`?` sheet (task 54) gains *"Space: peek"*. **Tests first:** (1) AppShell with the Work lens, the
Desk and a toast open holds exactly one `[aria-live]` (mutant: the toast stack's region kept, or a
region per section); (2) two digests 5 minutes apart speak once, an answer between them speaks at
once (mutants: every arrival spoken; answers throttled); (3) every heartbeat element in the Work
and Desk fixtures is `aria-hidden` (mutant: `StatusDot` keeps its label in a row); (4) 120 issues
virtualised render `aria-rowcount="120"` and row 61 carries `aria-rowindex="61"` (mutant: the
rendered count); (5) with focus on the row, 10 s of fake time leaves Undo available, and 6 s after
blur it is gone (mutant: the window ignores focus); (6) the sheet lists `Space` as peek (mutant:
missing line); (7) `expectNoPrediction` on the region's text. **Depends on** 41, 62. **Design:**
05 §4.4 rules 4-5 (rule 4, `forced-colors`, is task 34), §4.6, §5.3 row 17; 07 D I9. **Closes** no
parity row alone; it is the floor under L-101, L-113 and L-225, and L-36's undo pause. **Does not
claim** a screen-reader test with a real reader (NVDA or Narrator by the owner is **UNVERIFIED**),
or WCAG conformance.

**67 — issue chips in Develop and Merge.** Deltas on phase 4's and phase 3's desktop files (rule
H2: the owner's file wins, and this task waits for it on the sync base). Files:
`desktop/src/provenance/ContextChip.tsx` (P4 task 17, P4 plan `:332`; **UNVERIFIED until merge**)
gains the `kind: "issue"` arm of `ProvenanceRef` (task 18's `Ref.Kind`), rendering the display alias
and revision in mono (`EP-142@3`, `--t-fact`) with `<ThreadNode/>` (SW.14) lit for its criterion
by `useThreadLit`; `desktop/src/components/develop/SessionPaneHeader.tsx` ⚑ (P4 plan `:364` mounts
the brief chip there) mounts the issue chip **beside** the brief chip, fed by the loader's second
key source (task 19, `Session.IssueRef`); `desktop/src/components/lens/merge/LaneCard.tsx` (P3
task 19, P3 plan `:556-558`; **UNVERIFIED until merge**) names each lane's issue from the lane
index (task 21). Every chip opens the **one** issue view (tasks 46-48) at the revision the chip
names, not the head (05 §4.5 rule 1); a revision older than the head says so (*"EP-142@3 · head
is @4"*); an erased issue resolves as *erased* (hub-contract §7.12) and still opens. **Tests
first:** (1) a session with `issue:is-…@3` renders `EP-142@3` beside the brief chip (mutant: the
issue chip replaces the brief chip); (2) clicking it opens the issue view at `@3` while the head is
`@4` (mutant: opens the head); (3) an erased ref renders *erased* and no title (mutant: a stale
cached title); (4) a session with no `IssueRef` renders no issue chip and no empty slot (mutant:
a placeholder chip); (5) a lane row names `EP-142` from the lane index, not from the branch name
(mutant: parses the branch, which AU-only projects keep title-free, Q8); (6) the alias follows a
project-key change without the key changing (mutant: the alias cached as a key, 05 §3.6);
(7) `expectNoPrediction` on the pane header. **Depends on** 48; P4-17, P3-19 (and 18, 19, 21
through their seams). **Design:** 05 §4.5 (Develop and Merge rows, rule 1 and rule 4), §3.6; 04
§5.1. **Closes** L-191's visible half (what the agent was told, with task 19) and L-98's
revision-pinned link inside the app (with task 47; https links are task 93); **serves** L-201.
**Does not claim** Deploy's chip (task 71), Design's `implements` (later, 05 §5.5), or proposed
evidence under each criterion (W3, task 104).

**68 — create an issue from any list, a Desk row, a span or a hunk.** Files:
`desktop/src/components/work/NewIssue.tsx` (+ test, a composer in `Popover.tsx` beside the focused
row, never a modal over the work), `desktop/src/work/keys/keymap.ts` (the `C` binding, task 53's
map), `desktop/src/components/palette/commands.ts` ⚑ (*New issue*, with its chord; after task 59),
and the span and hunk surfaces that offer it: task 60's peek (`peek.span`, `peek.hunk`) and phase
3's `components/lens/files/SpanDrawer.tsx` and `PathDiff.tsx` (P3 task 21; **UNVERIFIED until
merge**, and absent until then, so their entry points are added only if they exist on the sync
base). The write is `CreateIssueRequest` (SW.1) through `mutate` (SW.16): title and description as
`AuthoredText` of author class `text·H` (L-124); state from `TeamPolicy.defaultState` (L-9; intake,
not a member, lands in `triage`); `serves` optional, pickable with `T`. **The link to its origin**:
from a Desk row about an issue, a `relates` relation to it; from a span or a hunk, an origin
`{kind: "span" | "hunk", ref}` shown in the issue view (AX §8.2). SW.1's `CreateIssueRequest` has no
origin field, so this task adds `origin *IssueOrigin{kind, ref}` as an **added field** under WS0's
rule (reported as a skeleton gap; the Hub's store keeps it verbatim and never renders a span's text
into the issue). `C` fires only with list focus (05 §4.3 rule 1). **Tests first:** (1) `C` in a list
opens the composer; `C` in a text field types `c` and in a terminal reaches xterm (mutant: a
`document` listener); (2) from a Desk row about EP-142 the request carries a `relates` to it
(mutant: the link dropped); (3) from a span the request carries `origin.kind: "span"` and its ref
(mutant: the ref pasted into the description as text); (4) a team whose `defaultState` is
`backlog` creates in `backlog` (mutant: hard-coded `triage`); (5) the body is `text·H` (mutant: no
author class); (6) a 422 keeps the composer open with the Hub's reason and leaves no ghost row
(mutant: the optimistic row stays); (7) `expectNoPrediction` on the composer. **Depends on** 46, 53
(and 55 through SW.16). **Design:** 05 §1 row L-29, §4.3 (`C`), §4.5 rule 3; AX §8.2. **Closes**
L-29 and L-9's create half; **serves** L-124. **Does not claim** templates (task 78/89), the
prefill deep link (task 93), drafts (task 80/92), or create from Discover's criteria (W4).

**69 — the import dialog; `first.landing`'s rehearsal as the real issue EP-1.** Files:
`desktop/src/components/work/ImportDialog.tsx` (+ test), `desktop/src/api/work.ts` ⚑ (the import
relay call over task 25's `control-plane/internal/api/workimport.go`; its path and body are task
25's), `desktop/src/work/firstRun/rehearsal.ts` (+ test). **The dialog** (05 §5.3's demo, first
frame): source *Linear* (a personal API key, `type="password"`, held in component state for the
run only, never in the store, never on disk, never logged; the control plane pages Linear's public
API, task 25) or *CSV* (a file the control plane parses, task 25); a target project; then the
import, which posts SW.6's batch and lands every item in `triage` as *not tied · imported*
(`notTiedReason: "imported"`, task 15). While it runs it shows a running count only, *"imported 400
so far"*, never a total, a bar or *"N of M"* (`OD-27`); it ends with `ImportResult`'s plain counts,
*"142 created · 3 already imported"* (idempotent by `(source, externalId)`, SW.6), and one verb,
*Open Triage* (`G T`). **`first.landing`** (02 C11 as amended by 05 §4.4 C11): in a project with no
issues, the Work lens's zero state offers one verb, *Rehearse on EP-1*: it creates a **real** issue,
EP-1, in that project, whose one-line description names the rehearsal task, and the user's first
`Shift+R` starts it (task 61), so their first Desk item is a real LANDED row with a real span and
hunk. W1 runs it in **local mode** (`OD-20`), not a cloud sandbox; **the tiny starter repo it needs
does not exist in the tree** (searched `docs/` and the repo on 2026-09-25; a reported gap), so the
part either names the starter repo's owner task or ships the verb disabled with that fact as its
label, never a fake agent (AX Principle 7). **Tests first:** (1) a CSV with a repeated `externalId`
reports `created` and `existing` from the response, not from the file's row count (mutant: shows
the row count as created); (2) a paged Linear import never renders the word `of` beside a count or a
`progressbar` role (mutant: `400 of 1000`; also `expectNoPrediction`); (3) the key never reaches
`localStorage`, the persisted store or a log call (spies; mutant: key kept in `workSlice`);
(4) closing the dialog clears the key (mutant: kept for the next open); (5) the imported items show
*not tied · imported* in Triage (mutant: lands in `backlog`); (6) *Rehearse on EP-1* creates one
issue and starts nothing until `Shift+R` (mutant: auto-starts an agent); (7) a project with issues
offers no rehearsal (mutant: offered every visit). **Depends on** 25, 42. **Design:** 05 §5.3 (the
demo), §4.4 (C11 row); 02 C11. **Closes** L-220's W1 half (Linear and CSV into `triage`); **serves**
L-46. **Does not claim** Jira, Azure Boards or GitHub Issues import (W5, tasks 150-153), Linear's
comments or attachments in the batch (SW.6 carries none), or a measured import speed.

**70 — W1's local-mode demo, end to end, and its results.** Files:
`docs/verification/2026-09-25-phase-w-results.md` (new; the W1 section; the house model is
`docs/verification/2026-09-17-phase-1-results.md`), and a run script under
`docs/verification/phase-w/` (`w1-demo.md`: the steps a person runs, each with the command, the
observation to record and its time). The stack: the local Hub (`control-plane/cmd/hub/compose.yaml`)
through `devissuer` (WS1, task 17), the control plane on loopback, the desktop, **local mode**
sandboxes (`OD-20`), two signed-in people (the second makes the teammate's comment). The run
follows 05 §5.3's demo, frame by frame: import a CSV (task 69) → Triage shows *not tied · imported*;
`T` on EP-142 attaches its criterion from the text-match suggestion (task 42); `Shift+R` (task 61),
first sign within 10 s recorded as measured seconds; the teammate comments (task 51); the lane lands
(phase 3's land, task 22's `landed` move), the row flashes in place and does not move (tasks 40-41);
the watched resume test passes at the landed commit and EP-142 turns `done` by itself (task 23); the
Desk's last frame reads *"Nothing needs you. EP-142 landed and proven at 14:02."* (task 65). Beside
it, the §4.6 budgets are **recorded, not asserted** (`budget.paint`, `budget.palette`,
`budget.peek`, `budget.ack`), the Desk at 125 % and 150 % with 30 fixture agents is shown to the
owner, and each of the four themes' screens goes to the owner's eyes (`CLAUDE.md`: a screenshot is
not evidence). **Checks, each with the defect it would catch:** `go test ./control-plane/... -race
-count=1` and the `-tags integration` rows green (a Work route regression); `npm --prefix desktop
test` and `run typecheck` green; a `git grep -nE "localStorage|indexedDB|writeTextFile" --
desktop/src/work desktop/src/components/{work,issue,desk}` that finds no Hub-record write (task 55's
rule); a Hub query after the run showing the `done` move's actor is the person and its cause the
observed run (rule 2), and no sync principal wrote a fact state. **The results section** lists, per
parity row W1 closes (05 §1's W1 and W1-split rows: L-2, L-3, L-8's W1 half, L-9, L-10, L-13,
L-15, L-17's team half, L-24, L-29, L-34, L-36, L-38, L-39's solo half, L-41, L-42, L-46-L-48,
L-98's W1 half, L-101's list and board, L-102's W1 half, L-104, L-106, L-108, L-112-L-114, L-124,
L-126, L-127's W1 half, L-146, L-181, L-191, L-198, L-200 and L-201's in-product halves, L-220's
W1 half, L-221-L-229, L-234's W1 half; plus L-45 and L-117, which 05 §5.3 rows 4 and 12 moved
from W2 into W1), the task that built it and what was observed, and what stays **UNVERIFIED** (the budgets, the cloud
path, screen-reader use, `first.landing`'s starter repo). **Depends on** 01-69. **Design:** 05
§5.3 (the demo, revision 2), §4.6, §5.7. **Closes** nothing new; it proves W1's rows. **Does not
claim** the cloud demo (the owner's, after phase 6 stream A and phase 8 stream C; skeleton *Start
lines*), team mode, or any number as a measurement beyond the run it was taken in.

**71 — shipped: issues in the release manifest; *shipped to production, 14:02*.** W1b, a delta on
phase 7 (rule H2; phase 7's files win, and this task waits for P7-17 and P7-19 on the sync base).
Files: `control-plane/internal/deploy/sessions/sessions.go` (P7 task 19, P7 plan `:580`; **UNVERIFIED
until merge**) gains the `Panacea-Issue: is-…` trailer (SW.9) beside `Panacea-Session` in the same
walk of the shipped commit range, returning `ReleaseIssue{issueId, commits []string}` (ids only,
never titles, 05 §3.6); `control-plane/internal/api/deploy_drill.go` (P7 plan `:586`) serves them on
the release's drill route beside its sessions, with one `routes_audit_test.go` ⚑ catalog line;
`desktop/src/deploy/ReleaseIssues.tsx` (+ test) in the drawer (P7 task 17, `ReleaseDrawer.tsx`,
`:606`) lists each issue by alias with its chip (task 67); `desktop/src/deploy/api.ts` (P7 plan
`:593`) gains `ReleaseIssues`. **Shipped is derived, never a state and never stored on the issue**
(05 §3.3, rule 3): the issue view's head (task 46) and the Desk's LANDED section (a new kind
`issue.shipped` in `deskRows.tsx` ⚑, SW.17) read *"shipped to production, 14:02"* (the environment
and the release's confirm time, mono) from the manifest, with the pinned brief key it shipped
under (*"pinned `brief:bf-…@6`"*, 04 §2.4). A landed issue in no release reads *"Landed, not shipped
yet"* (`prototype/work-layer.html:687`). **Tests first:** (1) a range with two commits carrying the
same trailer yields one `ReleaseIssue` with both commits (mutant: one row per commit); (2) a commit
whose message *mentions* `is-…` in its body but not as a trailer yields nothing (mutant: a
substring scan instead of SW.9's trailer grammar); (3) the route's JSON carries no title field
(mutant: titles copied into the manifest); (4) the issue view shows *shipped to production* only
when a confirmed release lists it, and *staging* never reads as *production* (mutants: shipped on
`landed`; the first environment wins); (5) the Hub issue head has no `shipped` field after a
release (mutant: the fact stored on the head); (6) the Desk row appears on the next entry, not
while open (task 62's rule); (7) `expectNoPrediction` on the drawer block. **Depends on** 22, 64;
P7-17, P7-19. **Design:** 05 §5.3 (*W1b*), §3.3 (*Shipped is never a state*), §4.5 (Deploy row),
§2.3. **Closes** L-146; **serves** L-148 (W4, task 131) and L-140 (W5). **Does not claim** release
notes by criterion or freeze (W4), requester updates (W5), or commits made outside the product
(outside PRs, W5).

**72 — `hubapi` and `hubclient.WorkPlan`: the W2 wire types and methods.** Stream WH, seam
**SW.18**, after the W1 proof (task 70) and a sync-merge of `feat/hub` (rule H1). Files:
`control-plane/internal/hub/hubapi/work_plan.go` + `work_plan_test.go` (new), `hubapi/workids.go`
(no new prefix: `ft-`, `cy-`, `tp-`, `lb-`, `vw-`, `df-` are task 02's), and
`control-plane/internal/hub/hubclient/work.go` ⚑ (task 04's file: `Work` gains an embedded
`WorkPlan`). Complete Go in the part, in `types.go`'s idiom (task 03's scope). **Head fields, added
to `IssueHead`** (additive; none enters the `issue-revision` canonical form, so task 02's golden
vectors are unchanged): `estimate *int`, `dueOn *string` (a date), `rank string` (a fractional
index, `[0-9a-z]{1,32}`), `parentId *string`, `featureId *string`, `cycleId *string`, `subState
*string`, `movedFrom *IssueAlias`. **Records:** `CustomState{key, teamId, name, band
"before_started" | "after_landed", order int}` (05 §3.3; a key, not an id, since 05 §3.6 lists no
prefix; an `after_landed` state holds `done` until a person clears it); `Feature{id ft-, projectId,
name, branchRef, briefId, lead User, targetDate *string, state FeatureState, priority, labels,
initiativeId *string, createdAt, createdBy, updatedAt}` with `FeatureState` `planned`·`active`·
`paused`·`done`·`cancelled` (04 §2.5); `Cycle{id cy-, teamId, number, name *string, startsOn,
endsOn, state "upcoming" | "current" | "closed"}`; `Cadence{teamId, lengthDays, cooldownDays,
ahead int (1-15), firstStartsOn, updatedAt, updatedBy}` (L-57-L-59); `ScopeEntry{cycleId, issueId,
change "added" | "removed", at, by *User}`; `ClosureProposal{id, assignee User, issueIds,
policyAuthor User, reason, createdAt}` (L-11); `Template{id tp-, teamId *string (nil is org), kind
"issue", name, title, description AuthoredText, serves, labels, priority, estimate}`;
`LabelGroup{id lb-, teamId *string, name, exclusive bool}` and `Label.groupId`, `Label.teamId` nil
for org labels (L-17-L-19); `View{id vw-, teamId, projectId *string, name, filter FilterGroup, layout
"list" | "board", groupBy, subGroupBy, orderBy, display []string, isTeamDefault bool}` with
`FilterGroup{op "and" | "or", terms []FilterTerm, groups []FilterGroup}` (depth ≤ 3) and
`FilterTerm{field, op, values}`; `Draft{id df-, kind "issue" | "comment", issueId *string, body
AuthoredText, updatedAt, expiresAt}` (private to its author, L-33, L-44); `IssueAlias{projectKey,
number}`. **Team policy additions:** `estimateScale`, `estimateZero`, `staleAfterDays`, `staleMode
"propose" | "auto"`, `archiveAfterDays`, `stopAtLanded` unchanged. **Request bodies:**
`PatchIssuePlanRequest` (pointer fields for the head fields above), `RankRequest{before, after
*string}`, `MoveIssueRequest{toProjectId}`, `SetSubStateRequest{key *string}`,
`PromoteToFeatureRequest{name, slug}`, `PutCadenceRequest`, `AnswerClosureRequest{close []string}`,
and one create/patch body per record. `WorkPlan`'s methods, one per route task 74's migration and
tasks 75-80 build (the part lists each with its route). **Tests first**, in `types_test.go`'s four
styles: strict decode and equal re-encode per record (kills a tag like `due_on`); empty slices
encode `[]` (kills `omitempty`); `band` rejects `started` (kills a custom state inside the fact
band); a `FilterGroup` four deep is refused (kills unbounded nesting); `rank` outside the charset is
refused; W1's golden vectors still pass byte for byte (kills a head field leaking into the
revision); `var _ hubclient.Work = hubclient.NewFake()` compiles only once task 73 lands, so this
task adds the interface and a failing conformance stub. **Depends on** 70. **Design:** 05 §3.2,
§3.3, §5.4 (W2); 04 §2.5. **Closes** no row alone; it is the wire of every W2 row. **Does not
claim** recurring schedules on templates (W4), initiatives (W4), or brief templates (W4, L-122).

