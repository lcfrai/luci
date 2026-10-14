### Task 84: custom states in team settings and on the row

**Slice:** W2. **Size:** 2 focused hours (an estimate, not a measurement). **Stream:** WD2.
**Depends on** task 83 (`CustomState`, `TeamStates`, the `setSubState` op, the relay calls).
**Seams used:** **SW.12** (`<StateGlyph/>`, task 28 — frozen, so this task adds **one prop** and
draws **no new glyph**), **SW.16** (the replica's `mutate` / `onRefused`), **SW.13**
(`<ViewHeader/>`) and SW.8 (`expectNoPrediction`). The Hub half is task 75, already on the tree.

05 §3.3 is the whole rule: *"Teams may add custom states inside the human bands only: before
`started` … and after `landed` … The five fact states are fixed and cannot be renamed, reordered
or removed."* This task builds the editor that enforces it on screen and the row that shows its
result, and nothing else.

#### Anchors, re-read on 2026-09-27

`task-75.md:32-72` (decisions (a)-(g): the whole list replaced and matched by key; a band change
on a held key is `409 state_in_use`; `order` is `0..n-1`, gapless, per band; `SetSubStateRequest.Key`
is a `*string` where absent and `null` both clear; an unknown key is `422 substate_out_of_band`),
`:153-154` (the kinds `team.states_changed`, `issue.sub_state_changed`); `task-83.md` (this
branch: `CustomState`, `StateBand`, `TeamStates`, the `setSubState` op and its field path);
`task-46.md:95-149` (`stateOptions.ts`: `STATE_ORDER`, `STATE_LABEL`, `FACT_WAITS_ON`,
`isFactState`, `StateOption`), `:236-283` (`StatePicker.tsx`), `:554-645` (`IssueHead.tsx`);
`task-28.md:199-260` (`StateGlyph`'s props and its `GlyphKey` switch); `task-40.md:887`
(`expectNoPrediction`'s contract).

**UNVERIFIED until merge:** the team-settings shell that `TeamStates.tsx` mounts into. No part on
this branch builds a settings page; task 87 needs one too (for cadence) and task 89 for estimates.
This part builds the panel as a self-contained component with its own route-less export, and names
the missing shell as a gap.

#### Decisions the scope left open, made here

**(a) The editor is a list with two bands and no third drop zone.** The five fact states render as
five fixed rows, in `STATE_ORDER`, with no grip, no rename field and no delete control — not
disabled controls, **absent** ones, because a disabled delete button still teaches that deleting
is a thing that might one day work. Between `started` and `landed` there is no gap a drag can
land in: the drop targets are computed from the two band arrays only, so the mutant "a custom
state can be placed between two fact states" has nowhere to go, rather than being caught by a
check that a later refactor could drop.

**(b) A custom state draws its band's shape, never a new one.** SW.12 is frozen to added props, so
`StateGlyph` gains exactly one: `custom?: { name: string; band: StateBand }`. When it is set, the
glyph key is `todo` for `before_started` and `landed` for `after_landed` — an existing key, an
existing path, no new art — and the name is drawn beside the glyph in sans by the caller, not by
the glyph. `StateGlyph` remains a pure square SVG; text has never been inside it and is not added.

**(c) The row reads the custom state first, the done rule second.** An issue that holds an
`after_landed` sub-state reads *In QA*, not *Done*, even when its evidence is complete, until a
person clears the sub-state (task 75 (e): clearing re-runs the proof decision and closes the issue
as the clearer). So `IssueRow` and `IssueHead` read `head.subState` before `head.state`, and the
proof mark (task 28's `done_not_proven`) is not drawn while a sub-state is held — the issue is not
done yet, so there is nothing for the mark to qualify. The mutant is a row that reads the done
rule's result and skips the custom state; the test asserts the visible word.

**(d) A refusal rolls the whole list back, and the panel says the code in words.** The editor's
save is one `mutate` of one op (`putCustomStates`), because task 75 (a) replaces the list whole.
`onRefused` restores the list exactly as it was — not the last good order, the **pre-edit** order,
which is the op's BEFORE value (SW.16) — and the panel shows the code's sentence: *"That state is
in use on 3 issues"* for `state_in_use`, naming the count the Hub returned in `details`. The
mutant is an optimistic order that survives `onRefused`; the test reads the DOM order after the
refusal, not the store.

**(e) Deleting a held state is the Hub's answer to show, not the editor's rule to duplicate.**
Task 75 decides what happens to issues holding a deleted key. The editor does not pre-check, does
not warn from a local count and does not block the control: it sends, and renders what comes back.
Duplicating the rule here would give two rules that drift, and the desktop's copy would be the
one nobody updates.

#### Files

| Path | Change |
|---|---|
| `desktop/src/components/plan/TeamStates.tsx` | new: the editor, about 180 lines |
| `desktop/src/components/plan/TeamStates.test.tsx` | new: the six tests below |
| `desktop/src/components/plan/plan.css` | new: this phase's stylesheet starts here; the band list and the fixed rows |
| `desktop/src/components/issue/pickers/stateOptions.ts` ⚑ (task 46) | `customStateOptions`, pure, added; nothing existing changes |
| `desktop/src/components/issue/pickers/StatePicker.tsx` ⚑ (task 46) | one optional prop `custom`, and the two band sections |
| `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46) | passes the team's states to the picker; reads `subState` first |
| `desktop/src/components/glyphs/StateGlyph.tsx` ⚑ (task 28) | one added prop, `custom?: {name, band}`; no new path |
| `desktop/src/components/work/IssueRow.tsx` ⚑ (task 41) | the visible word comes from `subState` when held |

#### Interfaces — complete: the `stateOptions.ts` delta

```ts
// Added by task 84. Pure, as the rest of this file is: no React, no replica.
import type { CustomState, StateBand } from "../../../api/workPlan";

/** One row of the state picker, custom or fact, in band order. `key` is "" for a fact state. */
export interface StateChoice {
  key: string;                 // the custom key, or "" for one of the five
  state: IssueState;           // the fact state this choice sets or sits beside
  band: StateBand | null;      // null for a fact state
  label: string;
  kind: StateOptionKind;       // task 46's four kinds, unchanged
  note: string;
  current: boolean;
}

/** The picker's whole list: the nine fact options, with a team's custom states interleaved at
 *  the two band edges. before_started states sit after `backlog` and before `todo`'s successor
 *  `started`; after_landed states sit after `landed` and before `done`. Nothing may be placed
 *  between `started` and `landed` — this function offers no such position (decision (a)). */
export function stateChoices(
  current: IssueState,
  currentSubState: string | null,
  serving: boolean,
  custom: readonly CustomState[],
): StateChoice[];

/** The two band arrays, each sorted by `order`, used by the editor and by stateChoices. */
export function byBand(custom: readonly CustomState[]): Record<StateBand, CustomState[]>;

/** Gapless 0..n-1 per band (task 75 (c)), re-derived after every drag so the body is always
 *  legal and a 400 on `states[i].order` is a client bug, not a user's. */
export function renumber(band: readonly CustomState[]): CustomState[];
```

#### Interfaces — complete: the `StateGlyph` delta

```diff
 export function StateGlyph(props: {
   state: IssueState;
   priority?: Priority;
   notProven?: boolean;
   size?: GlyphSize;
+  /** A custom state the issue holds (task 84). The glyph draws its BAND'S shape — the `todo`
+   *  shape before `started`, the `landed` shape after — so SW.12 gains no art. The name is
+   *  drawn beside the glyph by the caller; no text ever enters this SVG. */
+  custom?: { name: string; band: "before_started" | "after_landed" };
 }): JSX.Element {
-  const { state, priority = "none", notProven = false, size = 16 } = props;
+  const { state, priority = "none", notProven = false, size = 16, custom } = props;
+  // A held sub-state outranks the done rule's mark: the issue is not done yet (decision (c)).
+  if (custom) {
+    const shape: GlyphKey = custom.band === "before_started" ? "todo" : "landed";
+    return drawGlyph(shape, priority, size, custom.name); // aria-label only
+  }
   const key: GlyphKey = state === "done" && notProven ? "done_not_proven" : state;
```

`drawGlyph` is the body task 28 already has, lifted into a named function in the same commit so
the two paths cannot diverge; its last parameter is the `aria-label`, defaulting to
`STATE_LABEL[key]`. This is a refactor with no behaviour change and task 28's own snapshot test
is the proof — if it moves a pixel, the refactor is wrong, not the test.

#### Interfaces — complete: `TeamStates.tsx`

```tsx
// desktop/src/components/plan/TeamStates.tsx — task 84. The team's custom states, edited in
// two bands. The five fact states are rendered as fixed rows with no controls at all.
import { StateGlyph } from "../glyphs/StateGlyph";
import { byBand, renumber, STATE_LABEL, STATE_ORDER, isFactState } from "../issue/pickers/stateOptions";
import type { CustomState, StateBand, TeamStates as TeamStatesWire } from "../../api/workPlan";
import { useWork } from "../../work/replica/replica";

export interface TeamStatesProps {
  teamId: string;
  /** Team admins edit; everyone else reads. A viewer sees the same list, no controls. */
  canEdit: boolean;
}

export function TeamStates({ teamId, canEdit }: TeamStatesProps): JSX.Element;

/** The band a drag may land in, derived from the two arrays only. There is no index between
 *  `started` and `landed` (decision (a)); this function cannot return one. */
export function dropTargets(states: TeamStatesWire): ReadonlyArray<{ band: StateBand; index: number }>;
```

The panel's shape, top to bottom: a `<ViewHeader>` reading *States*, the sentence *"Add your own
states before Started and after Landed. The five in the middle are facts: they move when the work
moves."*, then the single list. In the list, a custom row carries a grip, its name in an inline
text input, and a delete control; a fact row carries its glyph, its label, and the fact it waits
on in `--t-dim` (`FACT_WAITS_ON`, task 46) — the same words the picker uses, from the same
constant, so the two never disagree. Saving is one button, disabled until something changed, and
it names what it will send: *"Save 3 states"*.

Motion: a drag moves rows by `transform` only, and the drop settles in one 120 ms ease — it
explains where the row went and stops. No list-wide reflow animation; nothing uses
`transition: all` (DESIGN-SYSTEM.md).

#### Tests to write first

`TeamStates.test.tsx`:

| Test | The mutant it kills |
|---|---|
| `offersNoSlotBetweenStartedAndLanded` — `dropTargets` over a team with states in both bands, asserting every returned target's band and that the DOM has no drop node between the two fact rows | a custom state can be placed between two fact states |
| `factRowsHaveNoRenameReorderOrDelete` — queries for a textbox, a grip and a delete control **within each of the five fact rows** and asserts zero | `started` is renamable; a disabled-but-present delete control |
| `refusalRestoresThePreEditOrder` — reorders twice, `onRefused` with `state_in_use` and `details.count: 3`, asserts the DOM order equals the original and the panel reads *"in use on 3 issues"* | the optimistic order is kept after `onRefused`; the code shown as a raw string |
| `renumberIsAlwaysGapless` — property test over 200 random drags | a body that would earn a 400 on `states[i].order` |
| `expectNoPrediction(container)` on the panel | a "3 of 5 teams use custom states" line, or any % |

`IssueRow.test.tsx` ⚑ and `IssueHead.test.tsx` ⚑ each gain one:

| Test | The mutant it kills |
|---|---|
| `heldAfterLandedStateOutranksDone` — an issue whose evidence is complete and which holds `in_qa` reads *In QA*, has the `landed` glyph shape, and shows **no** proof mark; clearing the sub-state makes it read *Done* | the row reads the done rule's result and skips the custom state |
| `pickerListsCustomStatesInBandOrder` — a team with two before and one after; the picker's option order is asserted as a list | custom states appended at the end, or sorted by name |

#### Steps

1. Write `stateOptions`'s three added functions' tests. Run red. Write them. Green.
2. Lift `drawGlyph` out of `StateGlyph` with no behaviour change; task 28's snapshot must pass
   untouched. Then add the `custom` prop and its two-line branch.
3. Write `TeamStates.test.tsx`'s five tests. Run red.
4. Write `TeamStates.tsx` and the `plan.css` rules. Run green.
5. Wire the picker's `custom` prop from `IssueHead`; write the two row tests; make them pass.
6. Run the whole desktop suite: SW.12's and SW.4's existing tests must be untouched and green.

#### Acceptance

```
cd desktop && npx vitest run src/components/plan/TeamStates.test.tsx
  5 passed, 0 skipped

cd desktop && npx vitest run src/components/issue src/components/work
  all pass, including the two added rows; task 28's StateGlyph snapshot unchanged

cd desktop && git diff --stat src/components/glyphs/StateGlyph.tsx
  one prop, one branch, one lifted function — under 25 changed lines, or decision (b) was broken

cd desktop && npx tsc --noEmit
  no output
```

#### What this task does NOT claim

- **Not the Hub's rule.** Task 75 decides what a deletion does to issues holding the key; this
  panel shows its answer (decision (e)).
- **No per-team glyph colour.** Status colours are not seeded (02 C1); a custom state borrows its
  band's shape and the same ink as every other state.
- **No new glyph.** SW.12 gains one prop and no art (decision (b)).
- **No settings shell.** The panel is a component; where it is mounted is the missing shell named
  in the gaps.
- **Not sub-state history.** The row shows the state held now. The feed shows the changes; no
  timeline of sub-states is built here.
- **No prediction.** Nothing counts how many issues sit in a custom state, or how long they stay
  (`OD-27`).

**Closes** L-8's W2 part — custom names inside the human bands — jointly with task 75. Alone it
closes nothing: without task 75's route there is nothing to save to.

#### Gaps found while writing this part

1. **No team-settings shell exists on this branch.** This part, task 87 (cadence) and task 89
   (estimates) all need one. Someone should own `desktop/src/components/plan/TeamSettings.tsx`
   with a tab list; the cheapest answer is to add it to task 87, which needs two panels anyway.
   Until then `TeamStates` is reachable only from a test.
2. **`details.count` on `state_in_use` is assumed.** Decision (d) renders *"in use on 3 issues"*
   from it; `task-75.md:38-42` names the code but not the details field. If task 75 returns no
   count, the sentence drops to *"That state is in use"* and the test follows.
3. **`head.subState` is assumed on `IssueHead`.** Task 83 puts the W2 fields in a nested `plan`
   object but treats the sub-state as a head field, following task 75 (d)'s route shape. If task
   72's tag puts it inside `plan`, decision (c)'s read path changes by one dot and the tests say so.
