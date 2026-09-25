### Task 44: display options and the bulk bar, with *Start N worktrees* and `confirm.inplace`

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the lens, the
row) and, beyond the skeleton's row (gaps, named in the return), task 41 (`listKeys.ts`), task 42
(`CriterionPicker`, the `serves` op), task 43 (`BoardView`'s `grouping` prop) and task 46 (the
pickers, `handMove`, the assign rule). Uses SW.11 (`--row-h`, `--row-h-compact`, `--undo-window`),
SW.12 (`MarkGlyph kind="selected"`, `task-29.md:145`), SW.13 (`Route`, `task-31.md:84`), SW.15
(`workSlice`) and SW.16 (`mutate`, one batch, one Undo).

Parity: closes **L-104** (display options) and **L-225** (bulk actions; its *Delegate N to my
Orchestrator* is W3's). Design: 05 §4.3 (`X`, `Shift+↓`/`Shift+↑`, `05-design.md:998`), §4.6 (the two
optimism classes), §4.4 C4 (density) and C7 (motion: an explicit re-sort is the one place a row
may animate position); 02 C8 (`02:276-278`, `confirm.inplace`).

**Decisions made here, so nobody re-derives them:**

1. **Display options are a preference, so they live in `prefsSlice`** and persist with it (rule 3:
   a preference is the desktop's). The key is `workDisplay`, a record by **view**, not by project:
   `"list"`, `"board"` and `"triage"`. A missing entry reads `DEFAULT_DISPLAY`. No issue id, title,
   alias or other Hub datum is ever stored in it; the type makes that impossible, and test 2 greps
   the persisted blob for `is-` to prove it. It joins `partialize` at schema version 1 with no bump,
   the way `sendChordsToTerminal` did (`desktop/src/store/store.ts:30-33`): the root-level shallow
   merge gives an old snapshot the default.
2. **The default grouping is `criterion`** (05 §4.4 C13). `state`, `assignee` and `priority` are
   choices. The board honours `criterion` and `state` only (`BoardView`'s `Grouping`, task 43);
   choosing `assignee` or `priority` on the board is not offered (the menu hides those two there).
3. **Density maps to the tokens, never to a literal.** `compact` sets `data-density="compact"` on
   the list; `work.css` reads `block-size: var(--row-h-compact)` there and `var(--row-h)` otherwise
   (task 26, `task-26.md:97-98`). No `28px` or `32px` appears in a component or in `work.css`.
4. **Row toggles are the rows' own.** `show.labels = false` hides label chips on `IssueRow` and on
   board cards; the issue view (task 46) reads no display option at all, so its labels stay.
5. **FLIP on an explicit re-sort, and nowhere else.** Changing `ordering` in the menu records every
   row's `getBoundingClientRect().top` (First), applies the order (Last), sets each moved row's
   `transform: translateY(first − last)` (Invert), then on the next frame transitions `transform`
   to none over `--dur-slow` on `--ease` (Play). A replica change never runs it: those go through
   task 41's hold. Under reduced motion the order simply changes.
6. **The selection is `issueSelection`, not `selection`.** The scope names it `selection` in
   `workSlice`, but the combined store already has a persisted `selection` (the cockpit's, in
   `uiSlice.ts:35`, `partialize` at `store.ts:45`); a second one would collide at the root. So task
   37's `workSlice` gains `issueSelection: ReadonlySet<string>` (issue ids), **not** in
   `partialize`, with `toggleIssue`, `selectIssues`, `clearIssueSelection`. A gap for task 37,
   named in the return.
7. **Keys are the list element's own**, after `isBareListKey` (task 41): `X` (`KeyX`, no Shift)
   toggles the focused row; `Shift+ArrowDown`/`Shift+ArrowUp` add the focused row and the next or
   previous one and move focus there; `Esc` with a non-empty selection clears it and stops there.
   A ghost row (task 41) is never selected. Task 53 folds these into `KEYMAP` unchanged.
8. **Selected and lit are two shapes** (05 §4.1 D I1). A selected row keeps task 40's ring and adds
   `MarkGlyph kind="selected"` in its leading slot with `aria-selected="true"`; lit (task 36) never
   sets either.
9. **Each bulk act is one `mutate` batch, so one Undo.** The bar sends an **array** of ops, as
   task 46's duplicate does (`task-46.md:298-301`). Its acts: state (the human band: `triage`,
   `backlog`, `todo`), priority, assignee, labels, attach a criterion, and cancel. `done` and
   `duplicate` are not bulk acts: a hand-set done on a serving issue needs its own reason (task 48)
   and a duplicate needs its own target.
10. **Assignee in bulk uses task 46's rule, per issue.** Task 46's `assign` calls `mutate` itself
    (`task-46.md:357-365`), so it cannot join a batch. This task asks task 46 to expose the op it
    builds, `assignOp(head, me, to): WorkOp | null`, with `assign` becoming one line over it (a gap
    for task 46, named in the return). Another owner's lane-bound issue becomes a
    `requestReassign` inside the batch, never a silent reassign (rule 7).
11. **Start N worktrees is the one consequential act, so it confirms in place.** It is offered at
    N ≥ 2. The first press calls nothing and opens `ConfirmInPlace` inside the bar with the
    consequence `Starts 3 agents on 3 worktrees · route Global` (the route is the project's, from
    its data class, `OD-24`; **no cost figure**, because none is observed before a run). The
    second press calls the injected `onStart(ids)` once. `Esc` or *Cancel* closes it. Task 61
    wires `onStart` to SW.10; until then the lens passes nothing and the act is not drawn.

**Files** (the skeleton's file map lists `DisplayMenu.tsx` and `BulkBar.tsx`; it omits
`ConfirmInPlace.tsx`, `store.ts` and `workSlice.ts` for this task, a gap named in the return):
- `desktop/src/components/work/DisplayMenu.tsx` (+ `DisplayMenu.test.tsx`) — the menu, on the built `Popover` (new)
- `desktop/src/components/work/BulkBar.tsx` (+ `BulkBar.test.tsx`) — the bar and its batches (new)
- `desktop/src/components/common/ConfirmInPlace.tsx` (+ `ConfirmInPlace.test.tsx`) — 02 C8's `confirm.inplace` (new)
- `desktop/src/store/slices/prefsSlice.ts` — `DisplayOptions`, `workDisplay`, `setWorkDisplay` (modified)
- `desktop/src/store/store.ts` ⚑ — `workDisplay` joins `Persisted` and `partialize` (modified)
- `desktop/src/store/slices/workSlice.ts` ⚑ (task 37) — `issueSelection` and its three actions (modified)
- `desktop/src/components/work/WorkLens.tsx`, `IssueRow.tsx`, `BoardView.tsx` — read the options; `X`, `Shift+↓`/`↑`; FLIP (modified)
- `desktop/src/components/work/work.css` — density, the bar, the selected mark (modified)

#### Interfaces — complete: the `prefsSlice.ts` addition

```ts
// ── task 44: display options, a preference per view (rule 3). No Hub datum fits these types. ──
export type WorkViewId = "list" | "board" | "triage";
export type DisplayGrouping = "criterion" | "state" | "assignee" | "priority";
export type DisplayOrdering = "priority" | "updated" | "created" | "number";

export interface DisplayOptions {
  grouping: DisplayGrouping;
  ordering: DisplayOrdering;
  density: "default" | "compact";
  show: { assignee: boolean; labels: boolean; priority: boolean; agentLine: boolean; updated: boolean };
}

export const DEFAULT_DISPLAY: DisplayOptions = Object.freeze({
  grouping: "criterion", ordering: "priority", density: "default",
  show: Object.freeze({ assignee: true, labels: true, priority: true, agentLine: true, updated: true }),
}) as DisplayOptions;

/** A missing entry, or a stored one from an older build missing a key, reads the default. */
export function displayFor(all: Partial<Record<WorkViewId, Partial<DisplayOptions>>>, view: WorkViewId): DisplayOptions {
  const d = all[view] ?? {};
  return { ...DEFAULT_DISPLAY, ...d, show: { ...DEFAULT_DISPLAY.show, ...(d.show ?? {}) } };
}

// PrefsSlice gains:
//   workDisplay: Partial<Record<WorkViewId, Partial<DisplayOptions>>>;
//   setWorkDisplay(view: WorkViewId, patch: Partial<Omit<DisplayOptions, "show">> & { show?: Partial<DisplayOptions["show"]> }): void;
// PrefsState's Pick gains "workDisplay"; initialPrefsState gains
//   workDisplay: Object.freeze({}) as Partial<Record<WorkViewId, Partial<DisplayOptions>>>;
// setWorkDisplay merges the patch into the view's entry (show merged one level), never replaces it.
```

`store.ts` adds `"workDisplay"` to the `Persisted` pick and `workDisplay: s.workDisplay` to
`partialize` (`store.ts:34-48`), with a sentence beside the `sendChordsToTerminal` note saying why
no version bump is needed. `displayFor` also absorbs a stored value from a later build that knows a
key this one does not, because the spread keeps only what the type reads.

#### Interfaces — complete: the `workSlice.ts` addition (task 37's file)

```ts
// ── task 44: the issue selection. NOT persisted: it names Hub objects (rule 3; ADR-H1). ──
//   issueSelection: ReadonlySet<string>;                       // is- ids, never aliases
//   toggleIssue(id: string): void;
//   selectIssues(ids: readonly string[]): void;                 // adds; Shift+↓/↑ call it with one id
//   clearIssueSelection(): void;
// initial: issueSelection: new Set<string>()  — and "issueSelection" is absent from partialize.
```

Test 2 checks that absence as well as the presence of `workDisplay`.

#### Interfaces — complete: `desktop/src/components/common/ConfirmInPlace.tsx`

```tsx
// desktop/src/components/common/ConfirmInPlace.tsx — task 44. 02 C8's confirm.inplace: the
// consequence is stated where the act was asked for, and the second press is the act. Never a
// dialog, never a scrim, never a focus trap: Tab leaves it like any other control.
import { useEffect, useRef, type JSX, type KeyboardEvent } from "react";
import "./common.css";

export function ConfirmInPlace(props: {
  consequence: string;   // one sentence, sans; facts inside it (route, counts) are plain words
  confirmLabel: string;  // the act itself, e.g. "Start 3 worktrees"
  onConfirm(): void;
  onCancel(): void;
}): JSX.Element {
  const { consequence, confirmLabel, onConfirm, onCancel } = props;
  const confirm = useRef<HTMLButtonElement>(null);
  useEffect(() => { confirm.current?.focus(); }, []);
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "Escape") return;
    e.preventDefault();
    e.stopPropagation();   // Esc closes this, not the list's selection underneath
    onCancel();
  };
  return (
    <div className="cip" role="group" aria-label="Confirm" onKeyDown={onKeyDown}>
      <span className="cip-consequence">{consequence}</span>
      <button type="button" ref={confirm} className="cip-confirm" onClick={onConfirm}>{confirmLabel}</button>
      <button type="button" className="cip-cancel" onClick={onCancel}>Cancel</button>
    </div>
  );
}
```

#### Interfaces — complete: `BulkBar.tsx` and `DisplayMenu.tsx`, the exported halves

```tsx
// desktop/src/components/work/BulkBar.tsx — task 44.
import type { JSX } from "react";
import type { IssueHead, User } from "../../api/work";          // SW.15
import type { Route } from "../common/ViewHeader";             // SW.13
import type { BriefCriterion } from "../../work/suggestCriterion";

/** The human band a bulk state move may choose (decision 9). */
export const BULK_STATES = ["triage", "backlog", "todo"] as const;

/** "Starts 3 agents on 3 worktrees · route Global". No cost: none is observed before a run. */
export function startConsequence(n: number, route: Route): string {
  return `Starts ${n} agents on ${n} worktrees · route ${route === "au" ? "AU" : "Global"}`;
}

export function BulkBar(props: {
  selected: readonly IssueHead[];         // in list order; ghosts never reach here
  me: User;
  route: Route;                           // the project's, from its data class (OD-24)
  criteria: readonly BriefCriterion[];    // for "Attach a criterion" (task 42's CriterionPicker)
  onStart?: (ids: readonly string[]) => void;   // task 61 wires SW.10; absent → no Start act
  onClear(): void;
}): JSX.Element | null;                   // null when nothing is selected

// desktop/src/components/work/DisplayMenu.tsx — task 44. On the built Popover (common/Popover.tsx).
import type { DisplayOptions, WorkViewId } from "../../store/slices/prefsSlice";

export function DisplayMenu(props: {
  view: WorkViewId;
  anchor: DOMRect | null;                 // Popover's anchor; null is closed
  onClose(): void;
}): JSX.Element | null;                   // reads and writes prefsSlice.workDisplay[view] itself

/** What the menu offers for a view: the board has no assignee or priority grouping (decision 2). */
export function groupingsFor(view: WorkViewId): readonly DisplayOptions["grouping"][];
```

The bar's batches, one `mutate` call each (task 56 carries arrays as one Undo, `task-46.md:298`):

```ts
mutate(ids.map((issueId) => ({ kind: "setState", issueId, move: handMove(to) })));            // state
mutate(ids.map((issueId) => ({ kind: "setPriority", issueId, priority })));                  // priority
mutate(heads.map((h) => assignOp(h, me, to)).filter((op) => op !== null));                   // assignee
mutate(ids.map((issueId) => ({ kind: "labels", issueId, add, remove })));                    // labels
mutate(heads.filter((h) => !serves(h, c)).map((h) => ({ kind: "serves", issueId: h.id, add: [c], remove: [] }))); // criterion
mutate(ids.map((issueId) => ({ kind: "setState", issueId, move: handMove("cancelled") })));  // cancel
```

An empty array is not sent. `serves(h, c)` reads the head revision's `serves` from the replica
(SW.16); an issue that already serves the criterion is left out, so the batch writes no revision
that changes nothing.

#### CSS, appended to `work.css` and `common.css`

```css
/* work.css — task 44. Density reads the tokens; FLIP animates transform only. */
.wk-list .wk-row { block-size: var(--row-h); }
.wk-list[data-density="compact"] .wk-row { block-size: var(--row-h-compact); }
.wk-row[data-flip] { transition: transform var(--dur-slow) var(--ease); }
@media (prefers-reduced-motion: reduce) { .wk-row[data-flip] { transition: none; } }
:root[data-motion="reduce"] .wk-row[data-flip] { transition: none; }
.wk-row[aria-selected="true"] .wk-lead { color: var(--ink); }
.wk-bulk { position: sticky; inset-block-end: var(--s-3); display: flex; align-items: center; gap: var(--s-2);
  margin-inline: auto; padding: var(--s-2) var(--s-3); border-radius: var(--r-card);
  background: var(--surface); box-shadow: var(--e-card); font-size: var(--t-ui); }
.wk-bulk-count { font-family: var(--mono); font-size: var(--t-fact); font-variant-numeric: tabular-nums; color: var(--ink-2); }

/* common.css — task 44. confirm.inplace: a row of text and two buttons, no overlay. */
.cip { display: inline-flex; align-items: center; gap: var(--s-2); }
.cip-consequence { color: var(--ink); }
```

`--dur-slow` (260ms) and `--ease` are built (`tokens.css:135-138`). `.wk-list`, `.wk-row` and
`.wk-lead` are task 40's class names (**UNVERIFIED** until its part exists; its names win).

#### Tests to write first

`DisplayMenu.test.tsx`, `BulkBar.test.tsx` and `ConfirmInPlace.test.tsx`; the lens-level ones render
on `workScenario()` through task 40's test seam (**UNVERIFIED** name), with a `mutate` spy that
records each call's argument and an Undo spy on the returned `{undo}`.

| Test | What it asserts | Mutant it kills |
|---|---|---|
| `the default grouping is criterion, and a choice survives a remount` | fresh store: the menu's grouping reads *Criterion*; choose *State*; unmount the lens; rehydrate the store from `partialize`'s output; remount: grouping reads *State* and the list's groups are states | a state default; view-local state |
| `the persisted prefs hold display keys and no issue data` | select three issues and set options; `JSON.stringify(partialize(store.getState()))` contains `"workDisplay"`, not `"issueSelection"`, and no match for `/is-[0-9a-f]{16}/` or `EP-\d` | saving rows or the selection with the prefs |
| `hiding labels removes row chips and not the issue view's` | give EP-142 a label; turn `show.labels` off on `list`; the row has no `.wk-label`; task 46's `IssueView` for EP-142 still shows the label | a global toggle |
| `bulk priority on three issues is one batch, reverted by one Undo` | `X` on three rows, bar → *Priority* → *Urgent*; the spy saw **one** call whose argument is an array of three `setPriority` ops; one Undo call reverts all three in the replica | three chips, or an Undo that reverts only the last |
| `Start 3 worktrees confirms in place, and Esc cancels` | three selected, `onStart` spy: first press calls nothing and shows `Starts 3 agents on 3 worktrees · route Global`; `Esc` closes it with no call; press twice: `onStart` called once with the three `is-` ids | an optimistic start |
| `Start is not offered for one issue` | one selected: no *Start* button | a bulk Start that is `Shift+R` in disguise |
| `ConfirmInPlace is not a dialog` | render it: no element with `role="dialog"` or `aria-modal`; focus is on the confirm button; `Tab` moves out of it | a modal |
| `compact rows read their height from the token` | `work.css` as text: the compact rule's value is `var(--row-h-compact)`; no `28px` or `32px` in `work.css` or any `.tsx` in `components/work` | a hard-coded 28px |
| `X in a field does nothing; Shift+↓ extends` | `X` in the lens's filter `input` selects nothing; `Shift+ArrowDown` from EP-142 selects EP-142 and the next row, and focus moves | a document listener; an extend that does not move |
| `no prediction` | with the bar open and the menu open, `expectNoPrediction(container)` passes; the bar's count reads `3 selected` | an `N of M` count |

#### Steps

- [ ] Write `ConfirmInPlace.test.tsx`, then `ConfirmInPlace.tsx` and its CSS; its test passes.
- [ ] Add `DisplayOptions`, `DEFAULT_DISPLAY`, `displayFor`, `workDisplay` and `setWorkDisplay` to
      `prefsSlice.ts`; add `workDisplay` to `store.ts`'s `Persisted` and `partialize`. The built
      `store.test.ts` still passes (it pins the version-1 shallow merge).
- [ ] Add `issueSelection` and its actions to `workSlice.ts` (task 37's file), outside `partialize`.
- [ ] Write `DisplayMenu.test.tsx`; then `DisplayMenu.tsx` on `Popover`, with `groupingsFor`. Wire
      the lens, `IssueRow` (the `show` toggles, `data-density`) and `BoardView` (`grouping`).
- [ ] Write `BulkBar.test.tsx`; then `BulkBar.tsx`, the batches, `startConsequence` and the
      `ConfirmInPlace` step; add `X`, `Shift+ArrowDown`/`ArrowUp` and `Esc` to the lens's list
      `onKeyDown` after `isBareListKey`; add FLIP to the ordering change.
- [ ] Mutate by hand and watch each named test go red, then revert: `grouping: "state"` in
      `DEFAULT_DISPLAY` (1); put `issueSelection` in `partialize` (2); read `show.labels` in
      `IssueView` (3); `ids.forEach((id) => mutate(...))` (4); call `onStart` on the first press (5);
      give `ConfirmInPlace` `role="dialog"` (7); write `28px` in the compact rule (8).
- [ ] Run the suite and the typecheck; commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/components/work/DisplayMenu.test.tsx src/components/work/BulkBar.test.tsx src/components/common/ConfirmInPlace.test.tsx'
# expected: 3 files, 10 tests, all passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, store.test.ts included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -rn "role=\"dialog\"\|aria-modal" desktop/src/components/common/ConfirmInPlace.tsx desktop/src/components/work/BulkBar.tsx'
# expected: no output
```

#### What this task does NOT claim

- **Set as default for everyone** (L-105), **saved views** (L-93) and **manual ordering** (L-103): W2.
- **Delegate N to my Orchestrator** (L-225's W3 half): W3.
- **Starting the lanes**: `onStart` is task 61's, over SW.10. This task proves the two-step
  confirm and the one call, not that three agents start.
- **A cost line** on *Start N worktrees*: none is observed before a run (`OD-27`: no predictions).
- **`X` and `Shift+↓` in `KEYMAP`** and the `?` sheet: task 53.
- That FLIP *looks* right: jsdom has no layout, so the test proves the `transform` and the
  reduced-motion rule, not the motion; the owner judges it at task 39.

#### Gaps found while writing this part

1. The scope's `selection` in `workSlice` collides with the cockpit's persisted `selection`
   (`uiSlice.ts:35`, `store.ts:45`); this part names it `issueSelection`. Task 37 must carry it.
2. The file map omits `ConfirmInPlace.tsx`, and task 44's edits to `store.ts` ⚑ and `workSlice.ts` ⚑.
3. Task 46's `assign` calls `mutate` itself; bulk needs `assignOp(head, me, to): WorkOp | null`.
4. The skeleton's row gives 44 `dependsOn: 40`; it also needs 41, 42, 43 and 46.
5. `DisplayOptions` has no `layout` field, so task 43's List/Board choice stays lens-local; adding
   `layout: "list" | "board"` here would need the `WorkViewId` split to change (the owner's call).

