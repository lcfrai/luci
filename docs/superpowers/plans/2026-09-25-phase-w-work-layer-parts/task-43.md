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

