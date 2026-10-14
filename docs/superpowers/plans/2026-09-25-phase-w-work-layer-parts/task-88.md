### Task 88: sub-issues on the issue view, and *split into issues*

Stream **WD2**, about 2.5 focused hours (an estimate, not a measurement). Depends on task 83 (the
W2 wire types and replica kinds), and on tasks 46 (the issue view), 48 (*done, not proven*), 49
(the editor) and 56 (`mutate`). The Hub half is task 78, whose parent route and parent rule this
task reads and never re-derives; a sync-merge of `feat/hub` precedes the first green run (rule H1).

After this task an issue holds its children under its head, a parent says **which child or which
criterion is holding it** rather than showing a progress count, and a person can select lines in
the description and turn them into children in one act, each child already serving what the
parent serves.

**Anchors, re-read on 2026-09-27.** In this branch's parts: task 78's routes
(`task-78.md:119-120`), its one-level-per-project rule and the loop cap (`:33-36`), its three
ways a parent reaches `done` and the hold (`:47-56`), and its inheritance of *done, not proven*
(`:61-63`); task 77's `IssuePlan.ArchivedAt` (`task-77.md:228-230`). Task 83's types are the
scope's (`tasks-81-100.md:90-103`), **UNVERIFIED** (no `task-83.md` at this writing). On the tree
`desktop/src/components/issue/` does **not** exist yet (checked 2026-09-27); it is created by task
46, so every path under it is **UNVERIFIED until 46 merges**. In the parity docs: 05 §1 rows 20,
21 and 22.

#### Decisions the scope left open, made here

1. **The client never infers `done` from children.** Task 78's server runs the roll-up
   (`task-78.md:51-55`); the desktop renders the head it is given. Where the head is not `done`
   and every child is closed, the view reads the **hold** from the head's own fields and prints
   it; it does not compute *all children closed, therefore done*. Test 1 is the guard.
2. **The hold line is mono and names one thing.** *held · EP-150 not landed* or *held · criterion
   "uploads resume" open*. One reason at a time, the first in this order: an open child (lowest
   number first), then an unsettled criterion (first served first). Two lines would be a list of
   excuses; one line is an action.
3. **No count, ever.** Linear's parent row shows *2 of 3 sub-issues*. That is a prediction of the
   third (AX §10), so the parent row shows the children's **states as glyphs in a row**, one per
   child, each linking to its issue. Five glyphs is five facts; *2 of 3* is a forecast.
4. **`is-` only.** The sub-issue list filters on the record's id prefix, so a `tk-` agent task in
   the fixture is not rendered even if the replica holds one (OD-15 and L-20: people's plan and
   agents' plan never share a list). The filter is a function, `isPersonIssue(id)`, so the rule is
   testable without a DOM, and team mode's agent task card (task 107) is the only place `tk-`
   appears.
5. **Split writes children first, text second.** *Split into issues* on a selection creates N
   children through the relay, and only after **every** create stands does it replace the selected
   lines with their aliases in one editor transaction. A refused create leaves the text exactly as
   it was and says which line failed. The opposite order would delete a person's words on a 500.
6. **The children inherit `serves`, visibly and removably.** Each pending child is shown in the
   split sheet with the parent's criteria pre-filled as chips, each with a remove control, before
   anything is sent. A pre-filled `serves` that cannot be seen is a hidden default (05 §3.2's
   rule against them) and test 3's mutant.
7. **One Undo for the whole split.** The N creates and the one text replacement go through task
   56's `mutate` as a single undo group; Undo removes all N children and restores the lines. Task
   56's op batching is the mechanism (**UNVERIFIED** until `task-56.md` is on the branch — if it
   has no group, this task adds one `groupId` field, stated here as a gap).
8. **The editor entry point is a selection command, not a key.** *Split into issues* appears in
   task 49's selection menu and in the palette when a selection of at least one list line exists.
   No new key row: 05 §4.3's table is full at `S`, and a destructive-looking text act should not
   be one keystroke away.

#### Files

| Path | Change |
|---|---|
| `desktop/src/components/issue/SubIssues.tsx` (+ `.test.tsx`) | new: the child list, the hold line, the glyph row |
| `desktop/src/components/issue/SplitIntoIssues.tsx` (+ `.test.tsx`) | new: the split sheet and its one-group write |
| `desktop/src/components/issue/subissues.css` | new: the list's rules (tokens only) |
| `desktop/src/work/parents.ts` (+ `.test.ts`) | new: `isPersonIssue`, `holdOf`, `splitPlan` — pure, no React |
| `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46) | mounts `<SubIssues/>` under the head |
| `desktop/src/components/editor/selection.ts` ⚑ (task 49) | one command row: *Split into issues* |
| `desktop/src/api/client.ts` ⚑ (task 83) | `setParent`, `children` |

#### Interfaces — complete: `desktop/src/work/parents.ts`

```ts
import type { IssueHead } from "../api/work";                 // task 83 (UNVERIFIED)

/** People's issues are `is-`; agent tasks are `tk-` and never appear in a person's plan
 *  (OD-15, 05 L-20). The prefix is the Hub's id shape, task 03's. */
export function isPersonIssue(id: string): boolean { return id.startsWith("is-"); }

export type Hold =
  | { kind: "child";     alias: string; state: string }       // "EP-150", "in_review"
  | { kind: "criterion"; title: string; word: string }        // "uploads resume", "open"
  | null;

/** The one reason the parent is not done, or null when it is done or has no hold.
 *  Reads only what the head and the replica already carry; it never decides `done`. */
export function holdOf(
  parent: IssueHead,
  children: readonly IssueHead[],
  criteria: readonly { criterionId: string; title: string; word: string }[],
  closedStates: ReadonlySet<string>,
): Hold;

export type PendingChild = {
  title: string;                                              // the selected line, trimmed
  serves: readonly { briefKey: string; criterionId: string }[];
  sourceLine: number;                                         // index into the selection
};

/** One pending child per non-empty selected line, each carrying the parent's criteria.
 *  Checklist markers ("- [ ] ", "- ", "* ", "1. ") are stripped from the title. */
export function splitPlan(
  selection: readonly string[],
  parent: Pick<IssueHead, "serves">,
): readonly PendingChild[];
```

`holdOf`'s order is decision 2's and is pinned by test 2: an open child wins over an unsettled
criterion, the lowest-numbered open child wins among children, and the first-served criterion wins
among criteria. `splitPlan` drops empty and whitespace-only lines, so a selection with a blank
line between two items makes two children, not three.

#### Interfaces — complete: `SubIssues.tsx`

```tsx
import { holdOf, isPersonIssue } from "../../work/parents";
import { StateGlyph } from "../glyphs/StateGlyph";            // SW.12, task 46/84

export function SubIssues(p: { parent: IssueHead; onOpen(id: string): void }): JSX.Element | null {
  const r = useReplica();
  const kids = r.children(p.parent.id).filter((k) => isPersonIssue(k.id));   // decision 4
  const hold = holdOf(p.parent, kids, r.criteriaOf(p.parent), r.closedStates(p.parent.teamId));
  if (kids.length === 0) return <AddChild parentId={p.parent.id} />;
  return (
    <section className="si" aria-label="Sub-issues">
      <header className="si-head">
        <span className="si-glyphs" aria-hidden="true">
          {kids.map((k) => <StateGlyph key={k.id} state={k.state} />)}
        </span>
        {hold
          ? <span className="si-hold">held · {hold.kind === "child"
              ? `${hold.alias} ${holdWord(hold.state)}`
              : `criterion "${hold.title}" ${hold.word}`}</span>
          : null}
      </header>
      <ul className="si-list">
        {kids.map((k) => (
          <li key={k.id} className="si-row">
            <StateGlyph state={k.state} />
            <button type="button" className="si-open" onClick={() => p.onOpen(k.id)}>
              <span className="si-alias">{k.alias}</span> {k.title}
            </button>
            <ProvenLine head={k} />        {/* task 48: "done, not proven" where it applies */}
          </li>
        ))}
      </ul>
      <AddChild parentId={p.parent.id} />
    </section>
  );
}
```

`.si-hold` and `.si-alias` are Commit Mono (OD-28); the titles are Segoe UI Variable. `ProvenLine`
is task 48's component, used unchanged — this task adds no second spelling of *done, not proven*.

#### Interfaces — complete: `SplitIntoIssues.tsx`

```tsx
export function SplitIntoIssues(p: {
  parent: IssueHead;
  selection: readonly string[];        // the selected lines, as the editor holds them
  anchor: DOMRect | null;              // beside the selection, never a modal over the work
  onClose(): void;
  onReplace(lines: readonly string[]): void;   // task 49 replaces the selection with these
}): JSX.Element {
  const [kids, setKids] = useState(() => splitPlan(p.selection, p.parent));
  const [busy, setBusy] = useState(false);
  const [said, setSaid] = useState<string | null>(null);
  const r = useReplica();

  async function go() {
    setBusy(true);
    const made = await r.group(async (g) => {                 // decision 7: one undo group
      const out: string[] = [];
      for (const k of kids) {
        const head = await g.createIssue({
          projectId: p.parent.projectId, title: k.title,
          serves: k.serves, parentId: p.parent.id,
        });
        out.push(head.alias);
      }
      return out;
    }).catch((e) => { setSaid(reasonOf(e)); return null; });  // decision 5: text untouched
    setBusy(false);
    if (made) { p.onReplace(made.map((a) => `- ${a}`)); p.onClose(); }
  }

  return (
    <Popover anchor={p.anchor} onClose={p.onClose}>
      <ol className="sp-list">
        {kids.map((k, i) => (
          <li key={k.sourceLine} className="sp-row">
            <input className="sp-title" value={k.title}
              onChange={(e) => setKids(edit(kids, i, { title: e.target.value }))} />
            <span className="sp-serves">
              {k.serves.map((s) => (
                <button key={s.criterionId} type="button" className="sp-chip"
                  onClick={() => setKids(edit(kids, i, { serves: without(k.serves, s) }))}>
                  {s.criterionId} <span aria-hidden="true">×</span>
                  <span className="sr-only">remove</span>
                </button>))}
              {k.serves.length === 0 ? <span className="sp-mono">serves nothing</span> : null}
            </span>
          </li>))}
      </ol>
      <button type="button" className="sp-go" disabled={busy} onClick={go}>
        Create {kids.length} issues <kbd>Enter</kbd></button>
      {said ? <p className="sp-said">{said}</p> : null}
      <Said text={said} />
    </Popover>
  );
}
```

`r.group` is task 56's undo group; if task 56 lands without one, this task adds `groupId` to
`WorkOp` and the undo stack keys on it (the gap below). `Popover` is
`desktop/src/components/common/Popover.tsx` (verified on the tree, `{anchor, onClose, children}`).
`Said` is task 34's live region: the reason is announced once, never as a toast.

The editor command row, complete:

```ts
// desktop/src/components/editor/selection.ts ⚑ (task 49)
{ id: "issue.split", label: "Split into issues",
  enabled: (sel) => sel.lines.filter((l) => l.trim() !== "").length >= 1 && sel.inIssueDescription,
  run: (sel, ctx) => ctx.openSplit(sel.lines) }
```

#### Tests to write first, each with the mutant it kills

1. A parent whose children are all closed, with one criterion still `open`, renders *held ·
   criterion "…" open* and **not** `done`. **Mutant:** the client infers `done` from the children
   alone, and the row disagrees with the Hub (task 78 decision (c)).
2. `holdOf` returns the lowest-numbered open child when both a child and a criterion hold.
   **Mutant:** the criterion wins, so the reader chases the wrong thing.
3. The sub-issue list renders no `tk-` row when the fixture carries one child of each prefix.
   **Mutant:** agent tasks listed as sub-issues (OD-15, L-20).
4. *Split into issues* on three selected checklist lines creates three children, each with the
   parent's `serves` pre-filled **and each chip removable before create**. **Mutant:** `serves`
   not carried; **mutant:** the chips are rendered without a remove control (a hidden default).
5. A refused second create leaves the description byte-identical and names the failed line.
   **Mutant:** the lines are replaced before the Hub confirms, and a 500 eats the person's words.
6. Undo after a three-child split removes all three and restores the text in one step.
   **Mutant:** three undos, the last of which restores text over two live children.
7. `expectNoPrediction` on the parent's head region. **Mutant:** *2 of 3 sub-issues*, the count
   Linear shows (decision 3).
8. `splitPlan` strips `- [ ] `, `- `, `* ` and `1. ` and drops blank lines. **Mutant:** the marker
   becomes part of the title.

#### Steps

- [ ] **Step 1: Check the base.** Task 46's `IssueView.tsx` and `Popover` mount points, task 48's
  `ProvenLine`, task 49's selection command table, and whether task 56's `mutate` has a group.
- [ ] **Step 2: The eight tests.** Run them: they fail.
- [ ] **Step 3: `work/parents.ts`.** Tests 2 and 8 go green with no component written.
- [ ] **Step 4: `SubIssues.tsx`** and the `IssueView` mount (tests 1, 3, 7).
- [ ] **Step 5: `SplitIntoIssues.tsx`** and the editor command row (tests 4, 5, 6).
- [ ] **Step 6: Green**, then commit by pathspec, naming task 46's and task 49's files as touched.

#### Acceptance

```bash
cd desktop && npx vitest run src/work/parents.test.ts \
  src/components/issue/SubIssues.test.tsx src/components/issue/SplitIntoIssues.test.tsx \
  --reporter=dot 2>&1 | tail -3                                      # 8 passed
grep -rn "tk-" desktop/src/components/issue/SubIssues.tsx | wc -l    # 0 (the filter is in parents.ts)
grep -rnE "[0-9]+ of [0-9]+|of \{.*\}.*sub-?issue" desktop/src/components/issue/ | wc -l   # 0
cd desktop && npx tsc --noEmit                                       # no output
```

Expected: `8 passed`, `0`, `0`, no output. None has been run; the code does not exist yet (OD-27).

#### What this task does not claim

- **Not the parent rule itself.** Task 78's server decides `done`; this view renders the head and
  the hold. A disagreement between them is task 78's bug, and test 1 is written so it surfaces
  here rather than hiding.
- **Not a depth limit.** Nesting beyond one level is the Hub's rule (`task-78.md:33-36`, a 32-step
  loop walk); the view renders whatever depth it is given, one level at a time.
- **Not agent tasks anywhere.** `tk-` records reach the agent's task card in task 107, never this
  list.
- **Not drag-to-reparent.** Setting a parent is the picker and the split; dragging a row onto
  another row is not built here and is not in the parity matrix.
- **No measured cost** of N creates over the relay; the split is sequential by design (the aliases
  must be known before the text is replaced) and has not been timed beyond a handful of lines.

#### Gaps reported to the skeleton

1. **Task 56's undo group is unconfirmed.** Decision 7 needs one op group with one undo entry. If
   `task-56.md` has no group, this task adds `groupId` to `WorkOp` and the undo stack keys on it —
   a change to a task-56 file, stated here and named in the commit.
2. **`createIssue` with `parentId` is not in task 68's request.** Task 68's composer sends no
   parent; task 78's parent is a separate `PUT`. This task sends `parentId` on create where the
   relay allows it and otherwise creates then `PUT`s, which makes decision 5's "every create
   stands" two calls per child. Asked of task 83, which owns the relay's shape.
