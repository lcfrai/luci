### Task 43: the board, and drag that offers a fact instead of faking it

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the row parts)
and task 41 (the hold model: a card never moves under the reader). Uses task 42's `serves` op and
task 46's `handMove`, `FACT_WAITS_ON` and `DuplicateOfPicker` (`task-46.md:113-117`, `:247`,
`:295-305`), which the skeleton's row does not name (a gap, named in the return); SW.4
(`StateMove{to, cause, reason}`), SW.12, SW.13, SW.14 (`ThreadNode`, `useThreadLit`) and SW.16.

Parity: closes **L-101**'s board (the list is task 40; the timeline is W4's) and **L-102**'s W1
part, *dragging into a fact state offers the fact instead of faking it; a manual move shows both*
(`05-design.md:236`). Design: 05 §4.1 (by criterion), §3.3 (the fact states), §4.4 C13 (a work
screen whose default grouping is workflow state fails the structural check) and C7.

**What the reader sees** (`prototype/work-layer.html:201-212`, `:646-665`): a row of columns, one
per brief criterion in brief order, each headed by the criterion's thread node, its text in sans
and a plain count (`3 issues`); the last column is *Serves no criterion*. A card is the row's parts
restacked: `StateGlyph`, the alias in mono, the right-hand fact, the title in sans, the owner line,
and the reader's own `AgentLine`. An empty criterion column says *No issue serves this criterion.
C files one.* (`work-layer.html:662`): the uncovered criterion, as a fact.

**Decisions made here, so nobody re-derives them:**

1. **Drop rules are one pure function**, `dropIntent`, exported from `BoardView.tsx` and tested
   without a pointer. The component only asks it what a drop means and does what it says.

   | From → to | Intent | What is sent |
   |---|---|---|
   | criterion A → criterion B | re-point | one `serves` op: `remove [A]`, `add [B]` (`add []` if B is already served) |
   | *Serves no criterion* → criterion B | attach | one `serves` op: `add [B]` |
   | criterion A → *Serves no criterion* | offer | nothing; the column reads *An issue leaves the thread with a reason, from its issue view.* (05 §2.3 point 4: *an open end is allowed; a silent one is not*) |
   | state → `triage`, `backlog`, `todo` (or a W2 custom state) | hand move | one `setState`, `handMove(to)`, written as the person |
   | state → `cancelled` | hand move | one `setState`, `handMove("cancelled")`, an empty reason (as Triage's `3`) |
   | state → `duplicate` | pick | nothing yet; `DuplicateOfPicker` opens in the column, and its one batch is the write |
   | state → `started` | offer | nothing; *Start a lane? `Shift+R`* above `FACT_WAITS_ON.started` (the act is task 61's) |
   | state → `in_review`, `landed` | offer | nothing; `FACT_WAITS_ON[to]` (task 46's words), and **no override control** |
   | state → `done`, the issue serves a criterion | offer | nothing; *Done by hand needs a reason. Open the issue.* until task 48's reason step exists |
   | state → `done`, the issue serves nothing | hand move | one `setState`, `handMove("done")` (task 46 decision: done on a non-serving issue is ordinary) |
   | the same column, or outside every column | none | nothing |

2. **The offer is a fact, not a dialog.** It renders inside the target column's header as a
   `role="status"` line, stays until the next drag, `Esc`, or a click elsewhere, and has no button
   that writes the state. For `started` the only act named is `Shift+R`, whose key is task 61's.
3. **A person-set state shows beside the fact it waits on** (L-102's *a manual move shows both*).
   A card whose last move has `cause: "hand"` and whose state is `backlog` or `todo` shows one mono
   line under its owner line: `set by Dylan · next: moves when a worktree is bound to this issue`,
   the second half being `FACT_WAITS_ON.started`. A fact-caused card shows its cause line from task
   41's `causeLine` instead.
4. **Columns are criteria by default and state only by choice.** `grouping` is a prop; the Work lens
   passes task 44's `DisplayOptions.grouping`, whose default is `"criterion"`. Until task 44 lands
   the lens passes `"criterion"` and nothing else. A card serving two criteria is drawn in both
   columns; dragging it re-points only the column it was dragged **from**.
5. **Drag is pointer events and `transform` only**, with no new dependency. `pointerdown` on a card
   captures the pointer; `pointermove` past 4px sets `style.transform = translate(dx, dy)` and
   `data-dragging`; `pointerup` asks `hitTest(x, y)` for the column under the pointer, then
   `dropIntent`, then clears the transform so the card is back where it was. The card **always**
   returns to its slot: a write moves it only when the replica's next state arrives, through the
   hold model, and only if the reader's attention has left both columns (task 41 decision 2).
   `hitTest` defaults to `document.elementFromPoint(...)?.closest("[data-col]")` and is a prop so
   jsdom tests can stub geometry. `Esc` during a drag cancels it.
6. **The keyboard needs no drag.** Every drop has a key path: `T` (task 42; issue rows in task 53)
   for the criterion, `S` (task 46's `StatePicker`, bound in task 53) for the state. This task
   binds no new key; cards are `tabindex="-1"` in one roving list for task 53's `J`/`K`.
7. **The List/Board switch is local to the lens** (`useState<"list" | "board">`), in its
   `ViewHeader` actions as a two-button `role="group"` labelled *Layout* with `aria-pressed`
   (`work-layer.html:553`). Task 44's `DisplayOptions` has no layout field, so the choice does not
   survive a remount (a gap for task 44, named in the return).

**Files** (the skeleton's file map lists `BoardView.tsx`; its test is implied):
- `desktop/src/components/work/BoardView.tsx` (+ `BoardView.test.tsx`) — the board, `dropIntent` and the drag (new)
- `desktop/src/components/work/WorkLens.tsx` — the *Layout* group in the `ViewHeader` actions (modified, task 40's)
- `desktop/src/components/work/work.css` — `.wk-board`, `.wk-col`, `.wk-card`, the offer line (modified)

#### Interfaces — complete: `desktop/src/components/work/BoardView.tsx`, the exported half

```tsx
// desktop/src/components/work/BoardView.tsx — task 43. Columns are criteria by default (05 §4.4 C13).
import type { JSX } from "react";
import type { IssueState, Serve } from "../../api/work";                  // SW.15, task 37
import { FACT_WAITS_ON, handMove } from "../issue/pickers/StatePicker";    // task 46
import type { StateMove } from "../../api/work";

export type Grouping = "criterion" | "state";   // task 44's DisplayOptions.grouping, board subset

/** A column's identity: `data-col` on its element is `colKey(column)`. */
export type Column =
  | { kind: "criterion"; briefKey: string; criterionId: string }
  | { kind: "no_criterion" }
  | { kind: "state"; state: IssueState };

export const colKey = (c: Column): string =>
  c.kind === "criterion" ? `c:${c.briefKey}#${c.criterionId}` : c.kind === "state" ? `s:${c.state}` : "none";

export const START_OFFER = "Start a lane? Shift+R";
export const UNTIE_OFFER = "An issue leaves the thread with a reason, from its issue view.";
export const DONE_OFFER = "Done by hand needs a reason. Open the issue.";
export const EMPTY_CRITERION = "No issue serves this criterion. C files one.";

export type DropIntent =
  | { kind: "none" }
  | { kind: "serves"; add: readonly Serve[]; remove: readonly Serve[] }
  | { kind: "setState"; move: StateMove }
  | { kind: "pickDuplicate" }
  | { kind: "offer"; lines: readonly string[] };   // shown in the target column; nothing is sent

export function dropIntent(card: { state: IssueState; serves: readonly Serve[] }, from: Column, to: Column): DropIntent {
  if (colKey(from) === colKey(to)) return { kind: "none" };
  if (to.kind === "no_criterion") return from.kind === "criterion" ? { kind: "offer", lines: [UNTIE_OFFER] } : { kind: "none" };
  if (to.kind === "criterion") {
    if (from.kind === "state") return { kind: "none" };
    const target: Serve = { briefKey: to.briefKey, criterionId: to.criterionId };
    const has = card.serves.some((s) => s.briefKey === target.briefKey && s.criterionId === target.criterionId);
    const remove = from.kind === "criterion" ? [{ briefKey: from.briefKey, criterionId: from.criterionId }] : [];
    return { kind: "serves", add: has ? [] : [target], remove };
  }
  if (from.kind !== "state") return { kind: "none" };
  switch (to.state) {
    case "started": return { kind: "offer", lines: [START_OFFER, FACT_WAITS_ON.started] };
    case "in_review": case "landed": return { kind: "offer", lines: [FACT_WAITS_ON[to.state]] };
    case "done": return card.serves.length > 0 ? { kind: "offer", lines: [DONE_OFFER] } : { kind: "setState", move: handMove("done") };
    case "duplicate": return { kind: "pickDuplicate" };
    default: return { kind: "setState", move: handMove(to.state) };   // triage, backlog, todo, cancelled
  }
}

export function BoardView(props: {
  projectId: string;
  grouping: Grouping;                                   // "criterion" unless task 44 says otherwise
  /** Geometry seam: the column under a point. jsdom has no layout, so tests stub it. */
  hitTest?: (x: number, y: number) => string | null;    // returns a colKey
}): JSX.Element;
```

`dropIntent` is total over the three column kinds and the nine states, and it never returns a
`setState` whose `to` is `started`, `in_review` or `landed`, which is the property test 2's loop
checks. A drop **between** a criterion board and a state board cannot happen (one board has one
grouping), so those pairs return `none`. The component turns an intent into at most one `mutate`:
`serves` → `{kind: "serves", issueId, add, remove}` (task 42's op), `setState` →
`{kind: "setState", issueId, move}`; `offer` sets the column's offer lines; `pickDuplicate` opens
`DuplicateOfPicker` anchored in the target column.

