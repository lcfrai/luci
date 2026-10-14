### Task 43: the board, and drag that offers a fact instead of faking it

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the row parts)
and task 41 (the hold model: a card never moves under the reader). Uses task 42's `serves` op and
task 46's `handMove`, `FACT_WAITS_ON` and `DuplicateOfPicker` (`task-46.md:113-117`, `:247`,
`:296-306`), which the skeleton's row does not name (a gap, named in the return); SW.4
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

#### The drag, inside `BoardView` (decision 5)

```tsx
const drag = useRef<{ issueId: string; from: string; x0: number; y0: number; el: HTMLElement } | null>(null);
const DEAD_ZONE = 4; // px: a click is not a drag

function onPointerDown(e: PointerEvent<HTMLElement>, issueId: string, from: string): void {
  if (e.button !== 0) return;
  e.currentTarget.setPointerCapture(e.pointerId);
  drag.current = { issueId, from, x0: e.clientX, y0: e.clientY, el: e.currentTarget };
  setOffer(null);
}
function onPointerMove(e: PointerEvent<HTMLElement>): void {
  const d = drag.current;
  if (d === null) return;
  const dx = e.clientX - d.x0, dy = e.clientY - d.y0;
  if (Math.hypot(dx, dy) < DEAD_ZONE && !d.el.dataset.dragging) return;
  d.el.dataset.dragging = "true";
  d.el.style.transform = `translate(${dx}px, ${dy}px)`;      // transform only: no left, top or width
}
function endDrag(e: PointerEvent<HTMLElement> | null): void {
  const d = drag.current;
  drag.current = null;
  if (d === null) return;
  const moved = d.el.dataset.dragging === "true";
  d.el.style.transform = "";
  delete d.el.dataset.dragging;
  if (!moved || e === null) return;                         // a click, or Esc
  const to = (props.hitTest ?? defaultHitTest)(e.clientX, e.clientY);
  if (to === null) return;
  act(d.issueId, columnOf(d.from), columnOf(to));           // dropIntent → at most one mutate
}
```

`columnOf` inverts `colKey` over the columns on screen; `act` is the table in decision 1. The
`Esc` handler on the board calls `endDrag(null)`. `defaultHitTest` is
`(x, y) => document.elementFromPoint(x, y)?.closest<HTMLElement>("[data-col]")?.dataset.col ?? null`.

#### `work.css`, appended

```css
/* task 43 — the board. Columns and cards use the elevation recipe (a 1px ring plus a layered
   shadow), never a flat border; the drag moves transform only. */
.wk-board { display: flex; gap: var(--s-3); align-items: flex-start; overflow-x: auto; padding: var(--s-4) var(--s-5); }
.wk-col { flex: 0 0 292px; display: flex; flex-direction: column; border-radius: var(--r-card);
  background: var(--canvas); box-shadow: var(--e-hair); }
.wk-col-head { padding: var(--s-2) var(--s-3); }
.wk-col-count { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); font-variant-numeric: tabular-nums; }
.wk-col-offer { font-size: var(--t-ui); color: var(--ink); padding: var(--s-2) var(--s-3); }
.wk-col-empty { font-size: var(--t-ui); color: var(--ink-2); padding: var(--s-3); }
.wk-cards { display: flex; flex-direction: column; gap: var(--s-2); padding: var(--s-2); min-block-size: 40px; }
.wk-card { position: relative; border-radius: var(--r-ctl); background: var(--surface); box-shadow: var(--e-card);
  padding: var(--s-2) var(--s-3); touch-action: none; }
.wk-card[data-dragging] { z-index: 2; opacity: 0.92; will-change: transform; }
.wk-card-set { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); }
```

`--canvas`, `--surface`, `--r-ctl`, `--r-card`, `--e-hair` and `--e-card` are built
(`desktop/src/styles/tokens.css:31-32`, `:111-112`, `:118`, `:120`); `--s-2` … `--s-5`, `--t-ui` and
`--t-fact` are task 26's (`task-26.md:91-108`). Radius follows size: the column is a card
(`--r-card`, 10px), the card inside it a control-sized surface (`--r-ctl`, 8px). There is no
`transition` in the block, so a returning card snaps back rather than animating layout.

#### Tests to write first — `desktop/src/components/work/BoardView.test.tsx`

Rendered on `workScenario()` through task 40's test seam (**UNVERIFIED** name, as in task 41), with
a `mutate` spy and a `hitTest` stub that returns whatever column key the test names. The world's
criteria are *the resume test passes* (`c-91e2a4b0`) then *upload works for PDF, XLSX and images*
(`c-40aa17c3`), in brief order; a third criterion no issue serves is added for test 6.

| Test | What it asserts | Mutant it kills |
|---|---|---|
| `default columns are criteria in brief order, Serves no criterion last` | with no `grouping` from task 44, the column headers read the two criteria in brief order, then the added third, then *Serves no criterion*; no header is a state word | a state-column default (C13) |
| `a drop on started sends nothing and offers Start` | state board; drag EP-167 (`todo`) to `s:started`; the spy was not called; the `started` column's `status` reads *Start a lane? Shift+R* and `FACT_WAITS_ON.started`; and, looping `dropIntent` over every state pair, no intent is a `setState` to a fact state | faking the fact |
| `a drop on landed offers no override` | drag to `s:landed`; no call; the column holds `FACT_WAITS_ON.landed` and **no** `button` | a manual `landed` |
| `backlog to todo sends exactly one op, as the person` | drag EP-166 (`backlog`) to `s:todo`; the spy saw one call, `{kind: "setState", issueId: ep166, move: {to: "todo", cause: "hand", …}}` | a no-op drop, or two ops |
| `a cross-criterion drop is one revision that drops the source` | drag EP-142 from `c-91e2a4b0` to `c-40aa17c3`; one call, `{kind: "serves", add: [c-40aa17c3], remove: [c-91e2a4b0]}` | appending both criteria |
| `an empty criterion column says so` | the third criterion's column holds *No issue serves this criterion. C files one.* | hiding uncovered criteria |
| `the drag moves transform only, and the CSS has no transition: all` | after `pointerDown` and a `pointerMove` of 40px, the card's `style.transform` is `translate(40px, 0px)` and `style.left`, `style.top`, `style.width` are empty; after `pointerUp` the transform is empty; `work.css` read as text contains no `transition: all` and no `transition` inside a `.wk-card` or `.wk-col` rule | a layout-animating drag |
| `no prediction on the board` | `expectNoPrediction(container)`; every column count matches `/^\d+ issues?$/` | an `N of M` column total |

#### Steps

- [ ] Write `BoardView.test.tsx`; it fails to import.
- [ ] Write the exported half (`Column`, `colKey`, `dropIntent`, the offer strings), then the
      columns from the selector (task 40's grouping, `grouping` prop), the cards from task 40's
      row parts, the hold model per column (task 41), the offers, the drag, `DuplicateOfPicker`.
- [ ] Mutate by hand and watch each named test go red, then revert: default `grouping` to
      `"state"` (1); make `started` return `setState` (2); add an *Override* button (3); return
      `none` for human-band drops (4); `remove: []` on a re-point (5); skip empty columns (6);
      set `style.left` in `onPointerMove` (7).
- [ ] Add the *Layout* group to `WorkLens`'s `ViewHeader` actions; append the CSS.
- [ ] Run the suite and the typecheck; commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/components/work/BoardView.test.tsx'
# expected: 1 file, 8 tests, all passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -n "transition" desktop/src/components/work/work.css'
# expected: no line in a .wk-card, .wk-col or .wk-board rule
```

#### What this task does NOT claim

- **Sub-groups and swimlanes** (L-102's W2 part), and **a shared manual order** inside a column
  (L-14, L-103): W2.
- **The timeline** layout (L-101's third): W4.
- **Starting a lane from the offer**: `Shift+R` is task 61's; this task names the key only.
- **The reason step for a hand-set done**: task 48; until then the offer points at the issue view.
- **The convergence word** in a column header: task 45 reads it from phase 4 for Plan; the board
  header shows the criterion, its node and a plain count only.
- That dragging *feels* right: visual acceptance is the owner's eyes (task 39), not these tests.

#### Gaps found while writing this part

1. The skeleton's row gives 43 `dependsOn: 40`; it also needs **41** (hold model), **42** (the
   `serves` op) and **46** (`handMove`, `FACT_WAITS_ON`, `DuplicateOfPicker`).
2. The List/Board choice has no home in task 44's `DisplayOptions`; it is lens-local here and does
   not survive a remount. Task 44 may add `layout: "list" | "board"` per view.
3. What a drop into *Serves no criterion* should do is not in the scope; decision 1 makes it an
   offer, because untying needs a reason (05 §2.3 point 4).

