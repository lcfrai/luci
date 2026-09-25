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

