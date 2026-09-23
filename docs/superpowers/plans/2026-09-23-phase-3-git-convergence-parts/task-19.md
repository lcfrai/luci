### Task 19: Merge lens, part 1 — the queue, its forecasts, land, conflict resolution in the lane, and the banners

The Merge lens is **what lands, in what order, against which brief** (brief §2.7). This task
builds its working half: the ready queue in the human's order with each row's forecast, the
not-ready list, landing one or all in order, the conflict state that sends resolution **into the
lane** with one click (3-D3), and the banners the prototype draws for a lost race, a checked-out
branch, and a control plane that is not answering. Task 20 adds the remote bar, the landed
history and evidence.

**The composition is the prototype's** (`prototype/phase-3-convergence.html`, `mergeLens()` at
lines 573-611, states `m-now`, `m-conf`, `m-race`, `m-checked`, `m-gitfail`, `m-down`, `m-empty`,
`m-new`): a title row, the banner, the definition of done, **READY TO LAND, IN THIS ORDER** with
"each forecast assumes the rows above it land first", **NOT MARKED READY**, and (task 20)
**LANDED ON <BRANCH>**. Every element stands on a fact from task 08's read model; the prototype's
*Show sources* text is the list of those facts, and each component's doc comment names its fact.

**Files:**
- `desktop/src/components/lens/MergeLens.tsx` — task 26's stub, filled (the only lens file this task edits)
- `desktop/src/components/lens/merge/QueueList.tsx`, `LaneCard.tsx`, `NotReadyList.tsx`, `MergeBanner.tsx`, `ResolveDialog.tsx`, `DoDSlot.tsx`, `FactShape.tsx` (new)
- `desktop/src/components/lens/merge/model.ts` — pure view-model functions (new)
- `desktop/src/components/lens/merge/landing.ts` — `landOne`, `landAllInOrder` (new)
- `desktop/src/components/lens/merge/merge.css` (new; first line `@import "../../../styles/tokens.css";`)
- tests beside each (new)

#### Interfaces — complete

```ts
// merge/model.ts — every sentence the lens shows about a row comes from here, so a test pins it.
import type { Convergence, Forecast, LaneView, QueueEntry } from "../../../api/convergence";

/** The name of a worktree id within one Convergence, or the id when it is not a lane here. */
export function laneName(conv: Convergence, id: string): string;

/** The forecast line of a queued row:
 *  clean     → "lands clean"                     (after rows above: "lands clean after upload-ui")
 *  conflicts → "lands after upload-ui and then conflicts with it in src/upload/handler.ts"
 *              | "conflicts with evidence-upload in src/x.ts, src/y.ts" (conflictsWith empty)
 *  nothing   → "nothing to land: every commit is already on evidence-upload"
 *  unknown   → "not read yet" | the forecast's why, verbatim */
export function forecastLine(conv: Convergence, f: Forecast): string;

/** The uncommitted note, or null: "Not committed: a.ts, b.ts — uncommitted work does not land." */
export function uncommittedLine(lane: LaneView): string | null;

/** 3-D2: null unless two or more live sessions share the lane; else names them with their
 *  kind, e.g. "upload-api (agent) and a shell", for SHARED_INDEX_NOTE's lead-in. */
export function sharedSessionsLine(lane: LaneView): string | null;

/** 3-D3's opening prompt, shown and editable before anything starts:
 *  "Merge origin/<branch> into this branch and resolve the conflicts in <paths>. <names> landed
 *   first and changed them. Commit the merge when the conflicts are resolved." —
 *  "<names> landed first …" is omitted when the row conflicts with the branch itself. */
export function resolvePrompt(conv: Convergence, entry: QueueEntry): string;

/** Whether landing is possible at all, and the one reason when not: checked out, control plane
 *  down, or a row whose forecast is not clean. */
export function landBlock(conv: Convergence | undefined, entry: QueueEntry, controlPlaneDown: boolean): string | null;
```

```ts
// merge/landing.ts — the two landing verbs. Neither opens a confirm dialog: a land is undoable
// exactly until pushed (task 10), so DESIGN-SYSTEM.md's "optimistic mutation with an Undo window"
// applies — the verb runs, and its toast carries Undo.
export interface LandOutcome {
  landed: LandResult[];
  /** The first refusal, if any, with its reason and the worktree it stopped at. */
  stopped?: { worktreeId: string; reason: ConflictReason | undefined; message: string; gitSays?: string };
}

/** Land one row with expectedTip = conv.tip and the head brief key (selectHeadBriefKey), then
 *  reload(), then a toast "Landed upload-ui into evidence-upload." with Undo → api.undoLand. */
export function landOne(conv: Convergence, worktreeId: string, briefKey: string, briefDigest: string, reload: () => void): Promise<LandOutcome>;

/** Land every queued row top down, each with the previous result's merge as expectedTip,
 *  stopping at the first refusal. One toast at the end names what landed; Undo is offered only
 *  for the last land, the one an exact undo can reach. */
export function landAllInOrder(conv: Convergence, briefKey: string, briefDigest: string, reload: () => void): Promise<LandOutcome>;
```

```tsx
// Component props. Each reads the store through selectors.ts only (shared-shape rule 4).
export function MergeLens(): JSX.Element;                       // mounts useConvergenceLoader("merge")
export function MergeBanner(props: { conv: Convergence | undefined; lastOutcome: LandOutcome | null; down: boolean; readAt: string }): JSX.Element | null;
export function DoDSlot(props: { branchId: string }): JSX.Element; // the phase 4 seam, below
export function QueueList(props: { conv: Convergence; down: boolean; onLand(id: string): void; onResolve(entry: QueueEntry): void }): JSX.Element;
export function LaneCard(props: { conv: Convergence; entry: QueueEntry; position: number; lane: LaneView; down: boolean;
  onLand(): void; onNotReady(): void; onResolve(): void; onMove(delta: -1 | 1): void }): JSX.Element;
export function NotReadyList(props: { conv: Convergence; down: boolean; onMarkReady(id: string): void }): JSX.Element;
export function ResolveDialog(props: { conv: Convergence; entry: QueueEntry; onClose(): void }): JSX.Element;
export function FactShape(props: { kind: "ok" | "need" | "fail" | "none" | "drift"; label: string }): JSX.Element;
```

**`DoDSlot` is the phase 4 seam.** Until phase 4's `DefinitionOfDone` exists it renders one static
card: *"DEFINITION OF DONE — no brief is available on this machine yet, so lands record no brief."*
When phase 4 lands, the slot's body becomes `<DefinitionOfDone refValue={headRef} onOpenDiscover={…} />`
fed by phase 4's head `Ref` (contract §4.4) — a one-file change named in task 23's seams list.
The lens never computes criteria itself (phase 4 plan's CONTRACT for mounting phases).

**Reordering** goes through `moveWithinVisible(all, visibleIds, id, delta)` (`lib/immutable.ts:45`,
seam S3.4) over the queue's entries, then `api.setQueue` with the full new order, then `reload()`.
Alt+↑ / Alt+↓ on a focused card move it; the position numbers are `tabular-nums` (the one count
AGENTIC-UX §10 allows, inside the queue). Reordering animates by FLIP on `transform` only, 200 ms,
the house curve, and not at all under `prefers-reduced-motion` (`DESIGN-SYSTEM.md` Interaction
standards) — through WAAPI (`element.animate`), because `desktop/package.json` carries no motion
library and this task adds no dependency.

**Mark ready / Not ready** call phase 2's `api.setWorktreeReady` (seam S3.3) then `reload()`. A
lane with no commits the branch lacks has **Mark ready** disabled with the reason "Nothing committed
since its last land — commit first". Nothing infers readiness (known defect 9).

**Conflict → Resolve.** A conflicting row shows *Start a session to resolve…*, which opens
`ResolveDialog`: the prompt from `resolvePrompt`, in an editable text area, the harness the
branch's defaults resolve to, and **Start session**, which calls `useCreateSession()(worktreeId,
{ openingPrompt })`. It writes nothing into any running agent's terminal (3-D3). If the resolved
harness has `supportsOpeningPrompt: false`, the dialog says so and offers **Open a shell here**
instead (`{ harness: "shell" }`). This is the one dialog in the lens, and it is not a confirmation:
it is where the prompt is written.

**Banners**, one at a time, in this precedence: control plane down (the last reading dimmed,
landing off: "The control plane is not answering, so nothing below is current — it is the reading
from 15:40. Landing, pushing and fetching are off until it answers."); a lost race
(`branch_moved` from the last land: "Nothing landed. evidence-upload moved from a90d3f4 to 1c9e2f0
while upload-ui was landing, so git refused to move it again. The forecast below is recomputed
against 1c9e2f0." + git's sentence + **Land upload-ui again**); checked out (`conv.checkedOutAt`:
"Landing is off: evidence-upload is checked out in <path>. A land moves the branch without
touching any checkout, and that checkout's files would then describe the old commit. Switch that
checkout to another branch, or merge there with git merge."); any other refusal of the last land
(`no_git_identity`, `brief_unresolvable`, `git_failed`: the server's message and `gitSays`).

**Zero states** (prototype `m-empty`, `m-new`): "Nothing is waiting to land into <branch>." with
"<names> are on this branch and none is marked ready. A worktree is ready when a person says so;
nothing infers it."; and "No worktree on <branch> yet." with **New worktree** (task 22's action;
hidden until task 22 provides it).

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `forecastLine pins every state` | the five sentences above from fixtures, byte for byte | a forecast shown as a percentage or a count |
| `forecastLine names the row that causes a conflict` | `conflictsWith: [ui]` → "lands after upload-ui and then conflicts with it in …" | "conflicts" with no cause |
| `resolvePrompt names origin, the paths and the cause` | a conflicting entry → the prompt above; a conflict with the branch itself → no "landed first" clause | a prompt telling the agent to merge a ref it cannot see |
| `the queue renders in the server's order with positions` | queue [b, a] → cards b, a with 1, 2 in `tabular-nums` | the desktop re-sorting the queue |
| `reorder goes through moveWithinVisible and PUTs the whole order` | Alt+↓ on the first card → `setQueue` called with [a, b]; spy on `moveWithinVisible` called once | known defect 10's index-across-a-filter |
| `Land sends the tip the human saw` | click Land → `api.land` with `expectedTip === conv.tip` and the head brief key | landing against a newer tip silently |
| `Land all stops at the first refusal` | second land rejects with `merge_conflict` → one land, `stopped.worktreeId` the second, a toast naming the first only | continuing past a conflict |
| `a landed toast offers Undo that undoes` | after a land, the toast's undo calls `api.undoLand(merge, merge)` | an Undo that does nothing |
| `a lost race shows the banner and Land again` | a land rejected `branch_moved` with `details.tip` → the banner text with both shas and git's sentence; the button re-lands with the new tip | a race shown as a crash |
| `a checked-out branch turns landing off with the path` | `checkedOutAt` set → the banner with the path; every Land disabled with its reason | a land that moves a checkout's branch |
| `control plane down dims the last reading` | loader error `network_unavailable` with a last value → content has the stale class, Land/Mark ready disabled, banner time is the last `readAt` | a blank lens, or live-looking stale data |
| `a conflict opens ResolveDialog and starts a session with the prompt` | edit the text, Start → `useCreateSession` called with the worktree and the edited prompt; no attach socket is written | writing into a running agent's PTY (3-D3) |
| `ResolveDialog offers a shell when the harness cannot take a prompt` | `supportsOpeningPrompt: false` → the sentence and Open a shell here → `{ harness: "shell" }` | a dead Start button |
| `the shared-index warning names both sessions` | a lane with an agent and a shell session → "upload-api (agent) and a shell" + SHARED_INDEX_NOTE | the hazard invisible (3-D2) |
| `uncommitted work is named on the row` | `uncommitted: [a.ts]` → the uncommitted line | "lands clean" hiding work that will not land |
| `Mark ready is disabled for an empty lane, with its reason` | no commits → disabled, `title` is the reason | readiness offered for nothing |
| `zero states` | no ready lanes → the m-empty text; no lanes → m-new's text | an empty white box |
| `the lens adds no interval and no spinner` | no `setInterval`; no element with `role="progressbar"`; the landing-in-flight shape is `StatusDot status="running" live` | a spinner per row |
| `design-system audit` | `merge.css` has no `gradient(`, no `transition: all`, no `1px solid`, no hex or `oklch(` outside var(); radii only `var(--r-*)`; elevation only `var(--e-*)` | the tells DESIGN-SYSTEM.md bans |

`@testing-library/react`, no jest-dom (shared-shape §3C); `api` mocked as phase 2's suites do;
fixtures from task 18's builders.

#### Steps

- [ ] **Step 1: Preflight.** G2: `grep -c "export function MergeLens" desktop/src/components/lens/MergeLens.tsx` is 1 and
  `grep -c "MergeLens" desktop/src/components/AppShell.tsx` ≥ 1. Task 18 merged. `AppShell.tsx` is **not** opened.
- [ ] **Step 2:** `model.ts` tests and functions first; then the components with their tests.
- [ ] **Step 3:** `merge.css` from tokens only; `FactShape`'s triangle is `clip-path`, not borders.
- [ ] **Step 4:** run; commit by pathspec.

#### Design-system checks (UI)

Cards are `--surface` with `--e-card` and `--r-card`; buttons `--r-ctl` with `--e-btn`; the one
primary button per card (Land) uses `--accent` because it is the interactive state; status is
`--run` / `--need` / `--fail` **with a shape** (`FactShape`: disc ok, triangle needs-you, square
failed, hollow ring none, half disc superseded), never colour alone. No gradient, no glass, no
indigo. Motion: the reorder FLIP and a 600 ms background flash on a row whose forecast changed
(opacity/background only), both explaining a change the user caused or should notice; nothing
else moves. Live times are `tabular-nums`. **The look is unverified until the owner sees it
(OD-12); the report says so.**

#### Acceptance

```
cd desktop && npx vitest run src/components/lens/merge/ src/components/lens/MergeLens   → PASS
cd desktop && npx tsc --noEmit                                                            → no output
cd desktop && npx vitest run                                                              → PASS (designsystem.test.ts included)
```

#### What this task does not claim

- The definition of done renders only its honest empty state until phase 4 (gate G3).
- Nothing here pushes, opens a PR, shows history or records evidence (task 20).
- The look, and whether the queue reads well with five lanes, is the owner's to judge.
