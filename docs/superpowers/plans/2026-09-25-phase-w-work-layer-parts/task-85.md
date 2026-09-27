### Task 85: the features board and feature tabs; status beside the facts

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WD2.
**Depends on** task 83 (`Feature`, `FeatureStatus`, `View.pinnedAsTab`, the relay calls).
**Seams used:** **SW.13** (`<ViewHeader/>`, task 31), **SW.14** (`<ThreadNode/>`, `<ThreadLine/>`,
task 36), **SW.16** (the replica), SW.8 (`expectNoPrediction`), and ⚑ `AppShell.tsx`'s lens arm
(task 40's idiom). The Hub half is task 76.

05 §4.1: *"a feature and its branch are one row, the branch in mono under the name"*. 05 §1 L-67
to L-71 give the rest: a lead, members, a target date, a priority, labels, a person's status word,
and tabs. This task builds the board, the feature view and its tabs, and puts the facts beside
the word so a stale status is visible rather than believed.

#### Anchors, re-read on 2026-09-27

`task-83.md` (this branch: `Feature`, `FeatureStatus`, `View.pinnedAsTab`, `WORK_PLAN_ROUTES`);
`task-76.md:31-70` (the Hub's feature rules and what `PromoteToFeature` returns), `:139-155`
(the event kinds); `task-31.md:74-130` (`ViewHeader`'s props, seam SW.13), `:131-185` (`.vh-*`);
`task-36.md:19-20` (`ThreadNode` is `null` with no `href`; `ThreadLine` never binds hover),
`:68-70` (the one import path, `components/thread/index.ts`); `task-40.md:96` (the prototype
leaves the lens, not the tree), `:116-120` (`WorkLens`, `WorkMount`, the `AppShell` arm),
`:482-490` (`WorkMount`'s shape, the model for `PlanMount`); `task-71.md:1-14` (shipped is
**derived** from the release manifest, never stored — the Release tab's only source);
`task-41.md` (`IssueRow`, reused whole on the Issues tab). Design: `docs/DESIGN-SYSTEM.md`
(elevation is a 1px ring plus layered shadow; `transform`/`opacity` only), `prototype/work-layer.html`.

**UNVERIFIED until merge:** phase 4's brief renderer, which the Brief tab embeds read-only. Its
import path and props are not on this branch; decision (d) names the seam this task codes against
and the fallback if phase 4's desktop tasks land a different one.

#### The word correction, recorded

05 §1 L-68 writes the tab as *Lanes*. 05 §4.1's word table says the screen says **worktree** for
a lane. The screen wins: the tab reads **Worktrees**, and `lane` stays an internal word. A test
asserts no visible string in this task's components contains *lane*, case-insensitive, so the
internal word cannot leak later through a tooltip or an empty state.

#### Decisions the scope left open, made here

**(a) `G P` opens a Plan lens; the feature view is a route inside it, not a lens of its own.**
`AppShell`'s `Lens` union gains `"plan"` (an addition, the same way task 37 added `"work"`), and
`PlanMount` reads a `planSlice` with `{ projectId, featureId | null }`. With no `featureId` the
board renders; with one, the feature view. Two lenses would mean two chords and two back
behaviours for one idea.

**(b) The card is one row, and the branch is inside it.** 05 §4.1 is literal: a feature and its
branch are **one row**. So the card's title line is the name, and the branch renders in `--mono`,
`--t-dim`, directly under it inside the same row box — not a second row, not a chip, not a
subtitle component that could later gain its own controls. The test asserts one row element per
feature and the mono branch inside it.

**(c) The status word and the facts sit on one line, the word first, the facts in mono.** The
word is a person's (`planned`·`active`·`paused`·`done`·`cancelled`, L-70). Beside it, three
counted facts: worktrees live, issues landed, released where. A feature set `active` with nothing
running reads `active · 0 worktrees live · 0 issues landed · not released`. The facts are never
hidden once a status is set — that is the mutant, and it is the whole reason the line exists:
the word is a claim, the facts are the evidence, and a stale claim is visible beside its absence
of evidence rather than being corrected by the app.

**(d) The Brief tab embeds phase 4's renderer read-only, and embeds no second one.** The tab
imports one component and passes `{ briefKey, readOnly: true }`. If phase 4's desktop renderer is
not merged, the tab renders a one-line placeholder naming the brief key and the reason — never a
second renderer, not even a small one, because two brief renderers is exactly the provenance
break CLAUDE.md calls wrong regardless of how good it is in isolation.

**(e) Released-where comes from task 71's derived read, not from a field.** `shipped` is derived
from the release manifest and is never stored on an issue (task 71). The Release tab and the
facts line call task 71's read; if it has no answer, they say *not released*, which is a fact
about what is known, not a prediction that it will be.

#### Files

| Path | Change |
|---|---|
| `desktop/src/components/plan/FeaturesBoard.tsx` (+ `.test.tsx`) | new: the board, the cards, the group-by-status columns |
| `desktop/src/components/plan/FeatureView.tsx` (+ `.test.tsx`) | new: the header, the facts line, the tab host |
| `desktop/src/components/plan/FeatureTabs.tsx` (+ `.test.tsx`) | new: the six built-in tabs plus pinned views |
| `desktop/src/components/plan/FeatureStatus.tsx` (+ `.test.tsx`) | new: the word, its picker, and the facts beside it |
| `desktop/src/components/plan/plan.css` ⚑ (task 84) | the board, card, tab and facts rules |
| `desktop/src/components/AppShell.tsx` ⚑ (tasks 38, 40) | `Lens` gains `"plan"`; the arm mounts `PlanMount` |
| `desktop/src/store/slices/planSlice.ts` | new: `{ projectId, featureId, tab }`, task 37's `workSlice` idiom |
| `desktop/src/work/keys/keymap.ts` ⚑ (task 59) | `G P` opens the Plan lens |

#### Interfaces — complete

```tsx
// desktop/src/components/plan/FeatureStatus.tsx — task 85. The word a person chose, and the
// facts beside it. The facts are never hidden by the word (decision (c)).
import type { FeatureStatus as Word } from "../../api/workPlan";

/** Counted facts, all present tense, all read from somewhere that knows. `releasedTo` is task
 *  71's derived read: null means nothing is known, rendered "not released", never "pending". */
export interface FeatureFacts {
  worktreesLive: number;
  issuesLanded: number;
  issuesTotal: number;
  releasedTo: string | null;
}

export interface FeatureStatusProps {
  status: Word;
  facts: FeatureFacts;
  canEdit: boolean;
  onPick(next: Word): void;   // one op through SW.16's mutate; Undo inside --undo-window
}

export function FeatureStatus(props: FeatureStatusProps): JSX.Element;
```

`issuesTotal` is carried so the Issues tab can say *12 issues*, and it is **never** rendered
beside `issuesLanded` as *4 of 12* — that is Linear's project progress, the exact shape `OD-27`
forbids and `expectNoPrediction` catches. The two numbers appear in different places, each as a
count of a thing that exists.

```tsx
// desktop/src/components/plan/FeatureTabs.tsx — task 85.
export type BuiltInTab = "brief" | "issues" | "criteria" | "worktrees" | "release";
export const BUILT_IN_TABS: readonly BuiltInTab[] = [
  "brief", "issues", "criteria", "worktrees", "release",
];
export const TAB_LABEL: Readonly<Record<BuiltInTab, string>> = {
  brief: "Brief", issues: "Issues", criteria: "Criteria",
  worktrees: "Worktrees",   // 05 §4.1's word, not L-68's "Lanes" — see the correction above
  release: "Release",
};

/** A saved view pinned as a tab (L-68). Its id is the tab key, so a pinned view and a built-in
 *  tab can never collide: built-ins are bare words, views are "vw-…". */
export interface PinnedTab { viewId: string; name: string }

export interface FeatureTabsProps {
  featureId: string;
  active: string;
  pinned: readonly PinnedTab[];
  onSelect(tab: string): void;
}
export function FeatureTabs(props: FeatureTabsProps): JSX.Element;
```

The tab strip is `role="tablist"` with roving tabindex, arrow keys moving the active tab and
`Home`/`End` jumping — the pattern task 31's `ViewHeader` already uses for its control group, so
the two behave alike.

The **Criteria** tab is the provenance tab: one `<ThreadLine>` in the gutter, one `<ThreadNode>`
per criterion linked to its source span in the brief (SW.14). A criterion with no source renders
no node (task 36's rule), which is the visible difference between *unsourced* and *sourced*, and
is not filled in with a grey placeholder.

```tsx
// desktop/src/components/plan/FeaturesBoard.tsx — task 85.
export interface FeaturesBoardProps { projectId: string }
export function FeaturesBoard(props: FeaturesBoardProps): JSX.Element;

// desktop/src/components/plan/FeatureView.tsx — task 85.
export interface FeatureViewProps { featureId: string }
export function FeatureView(props: FeatureViewProps): JSX.Element;

/** AppShell's "plan" arm, task 40's WorkMount idiom: reads planSlice, renders the board or the
 *  feature view, and with no Hub configured renders the same empty state WorkMount does. */
export function PlanMount(): JSX.Element;
```

The card, in order: the name; the branch in `--mono` under it, same row; the lead's face
(SW.11) and up to three member faces; the target date as a date; the priority notch; the labels.
Lead, target date, priority and labels edit **in place** (L-67, L-71): a click opens the same
pickers task 46 built, one op each through `mutate`, with Undo inside `--undo-window`. Nothing
opens a modal.

Elevation is DESIGN-SYSTEM.md's: a 1px ring plus a layered shadow, never a flat border. A card
lifts on hover by `transform: translateY(-1px)` and nothing else; drag between status columns
moves by `transform` only and settles in one 120 ms ease.

#### Tests to write first

| Test | File | The mutant it kills |
|---|---|---|
| `expectNoPrediction` on the board and on **every one of the six tabs**, driven by `BUILT_IN_TABS` so a seventh tab cannot be added without a check | `FeatureTabs.test.tsx` | a card shows *7 of 12 issues* or a progress bar — Linear's project progress |
| `activeWithNothingRunningStillShowsTheFacts` — status `active`, zero worktrees; asserts the line reads `0 worktrees live` | `FeatureStatus.test.tsx` | the facts are hidden once a status is set |
| `noVisibleStringSaysLane` — renders the board, the view and all six tabs, walks `textContent` and every `title`, `aria-label` and `placeholder`, asserts no `/lane/i` | `FeatureTabs.test.tsx` | the internal word leaks (05 §4.1) |
| `oneRowPerFeatureWithTheBranchInside` — asserts one row element per feature, and that the branch node's computed `font-family` resolves through `--mono` and is a descendant of that row | `FeaturesBoard.test.tsx` | a separate branch row; a branch rendered in sans |
| `leadTargetPriorityLabelsEditInPlaceWithUndo` — four edits, each asserting no `role="dialog"` appeared and that Undo restores the prior value | `FeaturesBoard.test.tsx` | a modal edit; an edit with no Undo |
| `releasedToNullReadsNotReleased` | `FeatureStatus.test.tsx` | *pending release*, a prediction |
| `criteriaTabDrawsNoNodeWithoutASource` | `FeatureTabs.test.tsx` | a grey placeholder node that implies a source |
| `briefTabEmbedsOneRendererOrSaysWhyNot` | `FeatureTabs.test.tsx` | a second brief renderer (decision (d)) |
| `gpOpensThePlanLens` | `keymap.test.ts` ⚑ | `G P` bound but unrouted, or bound while a text field has focus |

#### Steps

1. `planSlice.ts` and the `Lens` addition; `keymap.ts`'s `G P`. Run the existing shell tests —
   they must be untouched and green, because SW.15's union only grew.
2. Write `FeatureStatus.test.tsx`'s three tests. Run red. Write `FeatureStatus.tsx`. Green.
3. Write `FeatureTabs.test.tsx`'s five tests. Run red. Write `FeatureTabs.tsx` with the five
   built-ins, the pinned views and the tablist keyboard behaviour. Green.
4. Write `FeaturesBoard.test.tsx`'s two. Run red. Write `FeaturesBoard.tsx` and the card CSS.
5. Write `FeatureView.tsx` and `PlanMount`; wire the `AppShell` arm.
6. Run the whole desktop suite.

#### Acceptance

```
cd desktop && npx vitest run src/components/plan
  every test above passes; the lane test names each surface it walked

cd desktop && npx vitest run
  green: the Lens union and the keymap addition broke nothing

cd desktop && grep -rniE '\blanes?\b' src/components/plan --include=*.tsx | grep -v '^.*//'
  no output outside comments — the internal word never reaches a string literal

cd desktop && grep -rn 'transition: all' src/components/plan
  no output (DESIGN-SYSTEM.md)
```

Visual acceptance is the owner's eyes, not a self-report: the tests above prove structure and
words, not that the board looks right. Corporate network blocks headless screenshots (CLAUDE.md).

#### What this task does NOT claim

- **No target line, no forecast on the target date** (`OD-27`). The date is shown as a date. There
  is no burn-up, no "on track", no colour that means late.
- **No multi-team feature.** L-67's multi-team part is W4 (tasks 117, 126). Here a feature belongs
  to one project.
- **No promote.** Task 86 builds *Promote to feature* and the `B` key; this task renders features
  that already exist.
- **No brief rendering.** Decision (d): one embedded renderer or an honest placeholder.
- **No release logic.** Task 71 derives shipped; this reads its answer.
- **No cycles.** Task 87 owns cycles and the scope log; the Issues tab lists issues, not cycles.
- **Not a visual claim.** The tests assert structure, words and the absence of predictions.

**Closes** L-68, L-69, L-70 and L-71, and L-67's W2 part (lead, members, dates), jointly with
task 76.

#### Gaps found while writing this part

1. **05 §1 L-68 and 05 §4.1 disagree on the word** (*Lanes* vs *worktree*). Resolved here in
   favour of §4.1's word table; 05 should be corrected so the next reader does not re-decide it.
2. **Phase 4's brief renderer has no fixed import path on this branch** (UNVERIFIED). Decision (d)
   names the props this task codes against; if phase 4 ships a different shape, this tab is a
   one-file change and the placeholder test is what catches it.
3. **`worktreesLive` has no named source.** Phase 3 owns worktrees; no part on this branch says
   which read gives a feature's live count. Assumed to be a selector over the replica's worktree
   rows keyed by `branchRef`; if phase 3 keys them differently, `FeatureFacts` is unchanged and
   only the selector moves.
4. **The board has no empty state specified** by 05. This part renders the header, the status
   columns, and one sentence — *"No features yet. Promote an issue to make one."* — which is a
   forward reference to task 86 and should be confirmed by the owner.
