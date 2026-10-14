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
window rather than a confirm dialog. The Hub accepts **one** answer (`task-01.md:436`: a second is
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

**(f) A decide past its deadline is shown as defaulted, not hidden.** The Hub reads an unanswered
escalation past its deadline as defaulted (`task-01.md:437`). The row then reads *"The default
applied at 04:40: Let Alice go first"* with no buttons, and an answer the Hub refuses with `409
conflict` inside the Undo window lands the row in the same state, never in an error.

**Files:**
- `desktop/src/components/desk/teamRows.tsx` — the four `registerDeskRow` calls and the one
  `registerDeskSource`, run once at module load; `registerTeamDeskRows()` for tests (new)
- `desktop/src/components/desk/teamHorizon.tsx`, `teamDecide.tsx`, `teamSince.tsx`,
  `teamDecided.tsx` — one row component each (new)
- `desktop/src/components/desk/teamDesk.ts` — `teamDeskItems(snap)`, pure, and `useTeamDesk()`,
  the one fetch on Desk entry (new)
- `desktop/src/components/desk/teamRows.test.tsx` (new)
- `desktop/src/components/desk/teamDesk.css` — the rows' layout, importing the token layer first;
  not merged into W-62's `desk.css`, so neither task edits the other's stylesheet (new)
- `desktop/src/main.tsx` — one side-effect import, `import "./components/desk/teamRows";`, beside
  W-62's own registrations (modify; anchored on W-62's import, **UNVERIFIED until merge**)

**Interfaces:**
- Consumes: SW.17 (decision a); ST.8's `TeamDesk`, `HorizonItem`, `DecideItem`, `SinceItem`,
  `DecidedItem` (task 52); `teamSlice.teamDesk`, `setTeamDesk`, `teamSeenAt`, `markTeamSeen`,
  `beginAnswer`, `endAnswer`; `TeamClient.desk`, `.answer`, `.undo`; `AuthoredText` (task 53);
  `formatWaiting` (`lib/time.ts:49`).
- Produces: the four kinds, registered; nothing else reads them.

#### `teamDesk.ts` (complete)

```ts
import type { DeskItem } from "./deskModel"; // W-62, SW.17
import type { DecideItem, DecidedItem, HorizonItem, SinceItem, TeamDesk } from "../../store/types";

export type TeamKind = "team.horizon" | "team.decide" | "team.since" | "team.decided";

/** One project's batch of SINCE YOU LAST LOOKED items (decision d). */
export interface SinceBatch { projectId: string; since: string; items: SinceItem[]; }

/** Maps the snapshot's TeamDesk to DeskItems. Pure: same desk, same items, same order.
 *  ids are the record ids (es-…, cf-…) or "since:<projectId>", so an item keeps its identity
 *  across snapshots and the frame's hold line can tell a new arrival from a repeat. */
export function teamDeskItems(desk: TeamDesk | null): DeskItem<
  HorizonItem | DecideItem | SinceBatch | DecidedItem
>[];

/** Fetches desk(teamSeenAt) once when the Desk is entered and stores it with setTeamDesk.
 *  No interval: the frame takes one snapshot per visit and never re-ranks while open
 *  (W-62), so polling here would only build a hold line nobody asked for. */
export function useTeamDesk(client?: TeamClient): void;
```

`DeskItem` fields for each kind, fixed here:

| Kind | `id` | `arrivedAt` | `rankReason` | `ifNothing` |
|---|---|---|---|---|
| `team.horizon` | `conflictId` | `at` | *"two agents converge"* | *"the conflict thread continues between the two Orchestrators"* |
| `team.decide` | `escalationId` | `at` | *"blocks your worker"* | *"if you do nothing: <default label>, at <HH:MM>"* |
| `team.since` | `since:<projectId>` | newest item's `at` | *"from your Orchestrator"* | `""` |
| `team.decided` | `escalationId` | `at` | *"decided without you"* | *"it stands"* |

#### Row components (complete props)

```tsx
export function TeamHorizonRow(props: { item: DeskItem<HorizonItem> }): JSX.Element;

/** Options as buttons, in the Hub's order; the default carries "(default)". A text answer
 *  is not offered in TM1 (the Hub allows one, <= 300; task 55's panel is where you write). */
export function TeamDecideRow(props: {
  item: DeskItem<DecideItem>;
  client?: TeamClient;
  /** Injected for tests; 6000 in the product (decision c). */
  undoMs?: number;
}): JSX.Element;

export function TeamSinceRow(props: { item: DeskItem<SinceBatch> }): JSX.Element;

/** [Undo] calls client.undo(escalationId) once and then reads "Asked your Orchestrator to
 *  reopen cf-…"; it never touches the other side (task 52 decision d). */
export function TeamDecidedRow(props: { item: DeskItem<DecidedItem>; client?: TeamClient }): JSX.Element;
```

`TeamDecideRow`'s states, in order: **open** (buttons) → **chosen** (*"You chose: X · Undo"*, the
window) → **sent** (*"You chose X at 04:12"*) or **defaulted** (decision f). A text value in any
of these is `AuthoredText`; the option labels and consequences are `text·A`, so each carries the
label *"Your Orchestrator (agent)"* once per row, not once per option (one agent wrote them all).

- [ ] **Step 1: Write the failing tests first**

Vitest + RTL, with task 52's `desk.json` fixture through the real store, a fake `TeamClient`
recording every call, fake timers, and W-62's frame mounted (its `Desk` component, **UNVERIFIED
until merge**; before it, the same tests run against a two-line test double of
`registerDeskRow`/`registerDeskSource` that records registrations, which is enough for every row
below but the last).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `registers four kinds in their places` | the registry holds exactly `team.horizon`→`horizon`, `team.decide`→`needs_you`, `team.since`→`since`, `team.decided`→`decided_without_you` | a team row in FAILED or RUNNING, or a new section |
| `answering calls the route once` | click option 0, advance 6 s: `answer("es-51aa", 0)` called **once**; a second click in the window and a double click both leave it at one | a double answer, which the Hub refuses with `409` |
| `undo in the window sends nothing` | click, then Undo at 3 s, advance 10 s: `answer` never called; the row is open again | an Undo that only hides the row |
| `answering never navigates` | the lens and `selection` are unchanged after answering | a decide that jumps to the project view (`docs/AGENTIC-UX.md` §5) |
| `a refused answer reads as defaulted` | the fake answers `409 conflict`: the row reads *"The default applied at 04:40: Let Alice go first"*, no error line, no toast | a stale-answer error shown for Fleet's own default |
| `the deadline is a clock time` | the row contains `04:40` inside an element with `font-variant-numeric: tabular-nums` (computed), and no `/\bin \d+ ?(min|m|h)\b/` text | a countdown (OD-27) |
| `options carry their consequences and the default` | two options, each with its consequence; option 0 has *"(default)"*; the label *"Your Orchestrator (agent)"* appears once in the row | a decide without consequences, or with no author class |
| `informs batch` | a desk with five informs and one answer on one project and one inform on another: two `team.since` items, of six and one lines | one row per inform, the firehose AX §6 item 8 forbids |
| `no toast is created` | after rendering every fixture row, answering, undoing and marking seen, `store.toasts` is still `[]` | a toast per inform or per answer |
| `mark all seen moves the cursor` | the band's control: `teamSeenAt` equals the desk's `generatedAt`; the next `desk()` call sends it | a cursor that never moves, so every visit is all new |
| `horizon has no time and keeps accepted` | an `accepted` item renders; its text has no `min`, `~`, `ETA` or clock time | OD-27's ETA, or dropping HORIZON on `accepted` (§9.3) |
| `decided shows Fleet's summary and Undo only when reversible` | `undo.available` false: no Undo button; true: one, which calls `undo("es-51ac")` once and then reads *"Asked your Orchestrator to reopen cf-2b70"* | an Undo on a landed lane, or one that reaches the other side |
| `nothing comes from a teammate` | `teamDeskItems` over a desk whose fixture also has `strip` people with waiting agents: no item's payload references a teammate's `ag-…` except as `theirs.agentId` or `other` | a teammate's agent reaching your Desk by itself (§9.3) |
| `frame order holds` (W-62 mounted) | with an issue row (W-63) and a team decide both in NEEDS YOU, the frame orders them by W-62's rank rule, and opening the Desk twice keeps the same order | the team rows sorting themselves |

```bash
cd desktop && npm test -- teamRows 2>&1 | tail -5
```

Expected: the file fails to import `./teamRows`. That is the red.

- [ ] **Step 2: `teamDesk.ts`**

`teamDeskItems` first, table-tested on the fixture without rendering. `useTeamDesk` calls
`desk(teamSeenAt || "1970-01-01T00:00:00.000Z")` once per mount.

- [ ] **Step 3: The four rows**

Each row is layout on tokens and `AuthoredText`; `TeamDecideRow` holds its state machine in one
`useReducer`, and its one timer is the Undo window, started on choose and cleared on Undo or
unmount. `teamDesk.css` adds no transition and no animation.

- [ ] **Step 4: Register**

`teamRows.tsx` calls `registerDeskRow` four times and `registerDeskSource` once, for the kind
prefix `team.`; the side-effect import joins `main.tsx` next to W-62's.

- [ ] **Step 5: Acceptance**

```bash
cd desktop && npm run typecheck && npm test
```

Expected: `tsc` prints nothing; every file passes, `src/styles/designsystem.test.ts` and W-62's
own Desk tests (including its `expectNoPrediction` check on a full fixture Desk,
`tasks-61-80.md:95`) unchanged. Visual acceptance against task 51's state `6` is the owner's.

- [ ] **Step 6: Commit**

```bash
git add desktop/src/components/desk/team*.tsx desktop/src/components/desk/teamDesk.ts \
  desktop/src/components/desk/teamDesk.css desktop/src/components/desk/teamRows.test.tsx
git commit -F <msgfile> -- desktop/src/components/desk desktop/src/main.tsx
```

**What this task does NOT claim.**
- That the Desk exists on this branch. It is phase W task 62's; until its merge only the
  registration double runs, and the Desk shows nothing of team mode.
- That the horizon bar's panic controls (quiet mode, pause all) exist. W-62 does not claim them
  either (`tasks-61-80.md:98-99`).
- That a decide can be answered with free text here. The Hub allows it; TM1 answers by option only.
- That the read cursor follows the user between devices (task 52 decision e).

**Gaps (for the skeleton's owner).**
- **G54-1.** Task 54 depends on phase W task 62's Desk frame (SW.17), which the skeleton's row
  (`2026-09-25-phase-t-team-mode.md:381`, dependsOn `52, 53`) does not list, and which sits
  uneasily with OD-25 as the skeleton reads it (*"Nothing here waits on W"*, `:104`). Either
  dependsOn gains `W-62`, or the frame moves into this plan; W's plan says the former
  (`2026-09-25-phase-w-work-layer.md:100-101`, *"team mode adds rows to the frame task 62
  builds, never a second Desk"*).
- **G54-2.** `DeskSnapshot` (SW.17) is not yet written by W-62's part; decision (a) assumes it
  exposes store state.
