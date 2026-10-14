### Task 86: *promote to feature* and the `B` key

**Slice:** W2. **Size:** 2 focused hours (an estimate, not a measurement). **Stream:** WD2.
**Depends on** task 85 (the features board, `FeatureStatus`, the Plan lens and `planSlice`), and
through it on task 83 (`Feature`, `PromoteRequest`, `PromoteResult`, the relay call). **Seams
used:** **SW.16** (`mutate`, `onRefused`, the op's BEFORE value), **SW.11** (`--undo-window`),
02 C8 (`confirm.inplace`, task 44's `ConfirmInPlace`), task 41's `listKeys.ts` (`isBareListKey`),
and §3.7 rule 1's *inform* on the Desk (task 65). The Hub half is task 76.

05 §1 L-23: *"Promote to feature" creates `ft-` plus a branch brief, seeded from the issue, and
keeps its `serves` links; the issue becomes the feature's first child.* That sentence is the whole
task: one act, one key, and four things it must not break.

#### Anchors, re-read on 2026-09-27

`task-76.md:31-70` (the promote rule, what `PromoteToFeature` returns: the feature, the reserved
branch ref, the seeded brief key), `:139-155` (`feature.created`, `issue.promoted`,
`issue.feature_set`); `task-85.md` (this branch: `FeatureStatus`, `PlanMount`, `planSlice`,
the board's card); `task-83.md` (this branch: `PromoteRequest`, `PromoteResult`, the `setFeature`
op and its `plan.feature` field path); `task-44.md:130-165` (`ConfirmInPlace`'s exact props:
`consequence`, `confirmLabel`, `onConfirm`, `onCancel`; never a dialog, never a scrim, never a
focus trap), `:61-67` (the two-press idiom); `task-41.md:319-334` (`isTextTarget`,
`isBareListKey`); `task-65.md:1-8` (the Desk's §3.7 rule 1 *inform* rows, and the SW.17 names it
had to assume).

**UNVERIFIED until merge:** `desktop/src/components/palette/commands.ts` and
`desktop/src/work/keys/keymap.ts` — **task 59 has no part file on the tree**, so the registration
idiom used below (`registerCommand({ id, title, chord, when, run })`) is this part's reading of
the scope's *"task 59's rule"*, not a quoted interface. If task 59 lands a different shape, this
task's two registrations move and nothing else does. Named as a gap.

#### Decisions the scope left open, made here

**(a) Promote confirms in place, on the row, and states all three consequences.** `ConfirmInPlace`
takes one sentence. The sentence names the three things that will exist: the feature's name, its
branch, and the brief. *"Creates feature Payment retries on branch `feat/payment-retries`, with a
brief seeded from this issue."* The branch renders in `--mono` inside the sentence (02 C8 says
facts inside it are plain words; a branch ref is a fact whose shape matters). The
`confirmLabel` is the act: *"Promote"*. No modal, no scrim, no focus trap — task 44's component,
unchanged, used as it is.

**(b) The name and the branch are shown before the press, and they come from the Hub's rule, not
from a guess.** The desktop does **not** slugify the issue title locally to show a branch. It
calls `PromoteToFeature` with `dryRun: true`, renders what comes back, and the confirming press
sends the same request with `dryRun: false` and the same idempotency key. One rule, on the Hub,
shown before it runs. If task 76's route has no `dryRun`, the fallback is to show the name only
and the branch after — and this part says so rather than inventing a second slug rule; recorded
as a gap.

**(c) `serves` is never re-sent, so the optimistic copy cannot drop it.** The promote op's
optimistic update touches `plan.feature` and `parentId` on the issue and nothing else. `serves`
is not in the op's field set, so it is neither copied nor rewritten — the mutant ("`serves`
dropped by the client's optimistic copy") is impossible by construction rather than caught by a
check, and the test asserts the field set as well as the result.

**(d) `B` on a teammate's in-flight issue is a record edit, and it informs.** §3.7 rule 1: a
record edit on work someone else has in flight raises an *inform* on their Desk (task 65). It does
**not** re-brief their agents, does not touch their worktree, and does not ask. So `B` sends one
op and, when the issue's assignee is not the caller and the issue is `started` or `in_review`,
the Hub's event raises the inform — the desktop does not compose it. The mutant is a client that
re-briefs; the test asserts no agent-facing call is made on this path at all.

**(e) Undo inside `--undo-window` removes the feature and restores the issue.** Promote's inverse
is a single Hub call that archives the `ft-` and clears the issue's `plan.feature` and
`parentId`. The client does not delete the brief: task 76 owns what happens to a seeded brief on
an undone promote, and a client-side delete would be a second rule. The mutant is an Undo that
clears the issue and leaves an orphan `ft-` on the board; the test reads the board after Undo.

**(f) The feature picker and promote are two commands, not one.** `B` on a row whose team already
has features opens the **picker** (set an existing feature). *Promote to feature* is its own
command, reached from the picker's footer (*"Promote to a new feature"*) and from the palette.
One key that sometimes creates and sometimes assigns is the kind of overload that makes a
destructive-feeling act happen by accident.

#### Files

| Path | Change |
|---|---|
| `desktop/src/components/plan/PromoteToFeature.tsx` | new: the in-place confirm and its preview, about 140 lines |
| `desktop/src/components/plan/PromoteToFeature.test.tsx` | new: the six tests below |
| `desktop/src/components/plan/FeaturePicker.tsx` | new: `B`'s picker, task 46's `Picker` reused whole |
| `desktop/src/components/plan/plan.css` ⚑ (tasks 84, 85) | the preview's mono branch rule, four lines |
| `desktop/src/work/keys/keymap.ts` ⚑ (task 59, **UNVERIFIED**) | `B` on a list row |
| `desktop/src/components/palette/commands.ts` ⚑ (task 59, **UNVERIFIED**) | *Promote to feature* and *Set feature*, with their chords |

#### Interfaces — complete

```tsx
// desktop/src/components/plan/PromoteToFeature.tsx — task 86. 05 §1 L-23, confirmed in place.
import { ConfirmInPlace } from "../common/ConfirmInPlace";
import type { IssueHead } from "../../api/work";
import type { Feature } from "../../api/workPlan";

/** What the Hub says promote WOULD do, from the dryRun call (decision (b)). Never computed
 *  locally: the branch ref is task 76's rule, and one rule is the point. */
export interface PromotePreview {
  name: string;
  branchRef: string;
  briefKey: string;
}

export interface PromoteToFeatureProps {
  issue: IssueHead;
  /** Resolved by the caller, so the component renders nothing until the preview is known and
   *  never shows a guessed branch that the Hub then contradicts. */
  preview: PromotePreview | null;
  onDone(feature: Feature): void;
  onCancel(): void;
}

export function PromoteToFeature(props: PromoteToFeatureProps): JSX.Element | null;

/** The one sentence C8 asks for, assembled here so the test can assert it exactly. */
export function promoteConsequence(p: PromotePreview): string;
  // `Creates feature ${p.name} on branch ${p.branchRef}, with a brief seeded from this issue.`
  // The branch is wrapped in <code class="mono"> by the component; the string is the plain form.
```

```ts
// keymap.ts ⚑ — one row added. 05 §4.3's table: `Y` `B` `D` · cycle · feature · due date.
{ key: "b", when: "keys.list", run: "work.setFeature" }
```

`when: "keys.list"` is the guard, and it is the same guard `Y` and `D` use. Under it sits task
41's `isBareListKey`: no Ctrl, Meta or Alt, and `isTextTarget(e.target)` false. A `B` typed into
the title field is the field's `B`.

```ts
// commands.ts ⚑ — two rows added, both carrying their chord so the palette teaches the key.
{ id: "work.setFeature",   title: "Set feature",        chord: "B", when: "keys.list" }
{ id: "work.promoteToFeature", title: "Promote to feature", chord: null, when: "keys.list" }
```

*Promote to feature* has **no** chord (decision (f)): it is reached from the picker's footer and
from the palette by name. A command with a chord that also creates is the accident this avoids.

#### Tests to write first

| Test | The mutant it kills |
|---|---|
| `previewShowsNameBranchAndBriefBeforeConfirming` — asserts the exact `promoteConsequence` string, that the branch node resolves through `--mono`, and that the brief key is visible | a confirm that says only "Promote?"; a branch rendered in sans |
| `confirmsInPlaceNeverAModal` — asserts no `role="dialog"`, no scrim element, and that `Tab` moves focus out of the confirm to the next row control | a modal dialog; a focus trap |
| `servesIsUnchangedAndIssueIsFirstChild` — reads the op's field set (`plan.feature`, `parentId` only), then the post-promote state: `serves` deep-equal to before, `features[0].childIds[0] === issue.id` | `serves` dropped by the client's optimistic copy |
| `bFiresOnlyWithListFocus` — `B` with focus in the title input asserts no op was sent and the input's value now contains `b` | `B` in a text field sets a feature |
| `bOnATeammatesInFlightIssueInformsAndTouchesNoAgent` — assignee is another user, state `started`; asserts exactly one op sent, an inform row appears on the Desk, and **zero** calls on the agent-facing client (asserted on a spy that fails on any call, not on an absence of an expected one) | the edit re-briefs their agents |
| `undoRemovesTheFeatureAndRestoresTheIssue` — Undo inside `--undo-window`; asserts the board has no `ft-` card and the issue's `plan.feature` is null | Undo leaves an orphan `ft-` |
| `refusalNamesTheCode` — the Hub refuses with `feature_name_taken`; the row reverts and the sentence names it | a silent revert |

#### Steps

1. Write `PromoteToFeature.test.tsx`'s first three tests against an absent component. Run red.
2. Write `promoteConsequence` and `PromoteToFeature.tsx` over task 44's `ConfirmInPlace`, with
   the `dryRun` preview call in the caller, not the component. Run green.
3. Write `FeaturePicker.tsx` over task 46's `Picker`, with the *"Promote to a new feature"*
   footer row. No new picker chrome.
4. Add the `keymap.ts` row and the two `commands.ts` rows. Write the two key tests. Run green.
5. Write the inform test with a spy client that fails on any agent-facing call. Run.
6. Write the Undo and refusal tests. Run. Then the whole desktop suite.

#### Acceptance

```
cd desktop && npx vitest run src/components/plan/PromoteToFeature.test.tsx
  7 passed, 0 skipped

cd desktop && npx vitest run src/work/keys
  the B row passes, and every existing chord test is untouched and green

cd desktop && grep -rn 'role="dialog"\|scrim' src/components/plan/PromoteToFeature.tsx
  no output — confirm.inplace, not a dialog (02 C8)

cd desktop && grep -rniE 'slug|kebab' src/components/plan/PromoteToFeature.tsx
  no output — the branch ref comes from the Hub, never from a local slug rule (decision (b))

cd desktop && npx vitest run
  green
```

#### What this task does NOT claim

- **No git branch is created.** The branch is **reserved** by task 76 and created by phase 3 at
  the first Start (**UNVERIFIED** until phase 3's branch-create path merges). Nothing here touches
  a clone.
- **No brief is written by the desktop.** Task 76 seeds it; this shows its key.
- **No agent is briefed or re-briefed.** Decision (d), asserted by a spy that fails on any call.
- **Not the board.** Task 85 owns the features board; this adds one card to it by asking the Hub.
- **No multi-issue promote.** One issue becomes one feature's first child. Bulk promote is not in
  L-23 and is not built.
- **No prediction.** The preview says what promote *will create*, which is a rule's output, not a
  forecast of an outcome (`OD-27`).

**Closes** L-23 jointly with task 76.

#### Gaps found while writing this part

1. **Task 59 has no part file on the tree**, so `keymap.ts`'s and `commands.ts`'s registration
   shapes are this part's reading of the scope (UNVERIFIED). Tasks 87 (`Y`) and 89 (`D`) have the
   same dependency and should be written after task 59, or all three will assume the same shape
   and all three will move together if it is wrong.
2. **`dryRun` is not in task 76's route** as written. Decision (b) needs it, or the preview shows
   the name only and the branch appears after the act — which is a worse confirm. Task 76 should
   add `dryRun` to `PromoteRequest`, or say why not.
3. **`feature_name_taken` is this part's guess at the refusal code.** `task-76.md` names the
   promote rule but no code for a duplicate name. Whatever task 76's code is, the refusal test
   uses it; until then the test carries an UNVERIFIED comment naming this line.
4. **The inform's wording is task 65's, not this part's.** This task asserts an inform row
   appears; what it says is the Desk's. If task 65's row kinds do not cover a plan edit, task 65
   gains one kind and this test's assertion follows it.
