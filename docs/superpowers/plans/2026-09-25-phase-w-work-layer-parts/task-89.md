### Task 89: templates, estimates, due dates (`D`), manual order and move

Stream **WD2**, about 3 focused hours (an estimate, not a measurement). Depends on task 83 (the
W2 wire types and replica kinds), and on tasks 46 (the issue head and its `Popover` pickers), 53
(the key tables), 56 (`mutate`) and 68 (the composer). The Hub halves are tasks 78 (templates) and
79 (plan fields, rank, move, the resolved-blocker rule); a sync-merge of `feat/hub` precedes the
first green run (rule H1).

After this task five small planning facts are a person's act on the desktop: how big an issue is,
when it is due, where it sits in the team's shared order, which project it lives in, and what a
template fills in before any of that. None of them predicts anything, and the due date reaches the
Desk digest rather than a toast.

**Anchors, re-read on 2026-09-27.** In this branch's parts: task 79's wire
(`task-79.md:120-122`) and its examples (`:130-136`); its scale rule (a) (`:35-41`), its
no-op patch (b) (`:42-46`), its server-side rank keys (c) (`:47-54`), every-head-has-a-rank (d)
(`:55-60`), the refusal while work is bound (e) (`:61-65`), what a move clears (f) (`:66-73`), the
resolved-blocker direction (h) (`:85-93`), and the non-adjacent-neighbours rule (i) (`:284-292`),
which is what makes a drag inside a filtered list legal. Task 78's template routes
(`task-78.md:121-125`). Task 83's types are the scope's (`tasks-81-100.md:90-103`), **UNVERIFIED**.
On the tree `desktop/src/components/issue/` and `desktop/src/work/` do **not** exist yet (checked
2026-09-27); task 46 and task 53 create them. In the parity docs: 05 §1 rows 14, 16, 25, 26, 35,
37 and 103, and §4.3's key table.

#### Decisions the scope left open, made here

1. **`D` is two verbs on two row kinds, resolved by the dispatcher, not by the handler.** 05 §4.3
   gives `D` to the due date on an issue row; AX §5 gives `A`/`D` their decision meaning (accept /
   defer) on a decision row. Task 53's dispatcher routes a key by the focused row's **target
   kind**, so this task registers `plan.due` for `target: "issue"` and touches no decision row
   binding. One handler that branches on row kind is test 1's mutant, because the branch would sit
   in this task's file and drift from the Desk's.
2. **The estimate is off-scale until a person changes it.** Task 79 (a) leaves a stored estimate
   alone when the team's scale changes, so the picker shows the stored value with a mono *off this
   team's scale* line and offers the nearest legal values plus *clear*. It never silently rounds,
   and it never hides the old number.
3. **Zero is a value, not an absence.** A scale with zero allowed shows `0` as a pickable point,
   distinct from *no estimate*. `null` clears. The picker renders both, and the row renders `0`
   in `tabular-nums` and *no estimate* in mono.
4. **Due colour is a second signal only.** The date is `tabular-nums` and always carries its own
   word — *due today*, *2 days overdue* — so `forced-colors` and colour-blind readers lose
   nothing. Due-soon reaches the Desk digest (task 94) and **never** a toast or an OS notification
   (L-37, AX §10 anti-pattern 8). Test 3 asserts no `Notification` constructor and no toast call
   anywhere under the due modules.
5. **Drag writes one `POST /rank` with the two neighbours it dropped between.** The client sends
   `{after, before}` from the **rendered list**, which may be filtered; task 79 (i) accepts
   non-adjacent neighbours, so no client-side widening is needed and none is written. The key
   itself is the server's; the client never mints a rank string. Optimistic paint moves the row,
   and a refusal puts it back and names the code.
6. **The order is the team's, never this device's.** Nothing about row order is written to disk.
   Task 55's no-persistence grep is extended to the rank modules, so a local order is caught by a
   test rather than found in a support call.
7. **A move shows what it will clear before it runs.** The move dialog lists, from task 79 (f),
   exactly what this issue loses to this target: team labels, cycle, sub-state, feature, and the
   assignee when they are not a member of the target team. It is a list of facts about this issue,
   not generic help text, and it is read from the issue's own head.
8. **The old alias keeps working, and says so.** After a move, `O I` on the old alias opens the new
   issue with a mono *moved from EP-142* line under the head, read from task 79's alias record.
   A 404 on an alias a person still has in a document is test 4's mutant.
9. **A template's pre-fill is shown, and every field is removable before create.** The composer
   (task 68) gains a template row; picking one fills title, description, labels, `serves`,
   estimate and due, each rendered as it will be sent, each removable. Nothing a template sets is
   invisible (05 §3.2's rule against hidden defaults).

10. **The resolved blocker is rendered, not decided.** Task 79 (h) turns a `blocks` row into
    `relates` with one event when the blocker closes. The relations panel renders that event as
    the cause line of the now-`relates` row — *"unblocked when Priya closed EP-118"* — and the
    desktop never rewrites a relation itself.

#### Files

| Path | Change |
|---|---|
| `desktop/src/components/issue/EstimatePicker.tsx` (+ `.test.tsx`) | new: the team's scale, zero, clear, the off-scale line |
| `desktop/src/components/issue/DuePicker.tsx` (+ `.test.tsx`) | new: the date field, the words, `D` |
| `desktop/src/components/issue/TemplatePicker.tsx` (+ `.test.tsx`) | new: the team's then the org's templates |
| `desktop/src/components/issue/MoveDialog.tsx` (+ `.test.tsx`) | new: the target, the clears list, the refusal |
| `desktop/src/components/issue/plan.css` | new: the four surfaces' rules (tokens only) |
| `desktop/src/work/plan.ts` (+ `.test.ts`) | new: `dueWord`, `estimateOptions`, `neighboursOf`, `clearsOnMove` — pure |
| `desktop/src/components/issue/IssueRow.tsx` ⚑ (task 46) | the drag handle, the due chip, the estimate chip |
| `desktop/src/components/work/NewIssue.tsx` ⚑ (task 68) | one template row above the title |
| `desktop/src/work/keys/keymap.ts` ⚑ (task 53) | two rows: `D` → `plan.due`, `E` → `plan.estimate`, both `target: "issue"` |
| `desktop/src/api/client.ts` ⚑ (task 83) | `patchPlan`, `rank`, `move`, `templates`, `createTemplate` |

#### Interfaces — complete: `desktop/src/work/plan.ts`

```ts
import type { EstimateScale, IssueHead, Template } from "../api/work";   // task 83 (UNVERIFIED)

/** The word beside a due date. Never a prediction: it states today's relation to the date. */
export type DueWord =
  | { kind: "none" }
  | { kind: "overdue"; days: number; text: string }     // "2 days overdue"
  | { kind: "today";   text: "due today" }
  | { kind: "soon";    days: number; text: string }     // "due in 2 days" (<= soonDays)
  | { kind: "later";   text: string };                  // "due 12 Nov"

export function dueWord(dueOn: string | null, today: string, soonDays: number): DueWord;

/** The pickable points for a team's scale, with zero when the scale allows it.
 *  `stored` is included, marked off-scale, when it is not one of them (decision 2). */
export function estimateOptions(
  scale: EstimateScale,
  stored: number | null,
): { points: readonly number[]; offScale: number | null };

/** The two ids a drop sends, read from the RENDERED list (task 79 (i) accepts non-adjacent). */
export function neighboursOf(
  rendered: readonly string[],      // issue ids in rendered order
  movingId: string,
  toIndex: number,
): { after: string | null; before: string | null };

/** What this issue loses when it moves to this project, from task 79 (f) and the issue's head. */
export function clearsOnMove(
  head: IssueHead,
  target: { projectId: string; teamId: string; memberIds: ReadonlySet<string> },
): readonly string[];               // "team labels", "cycle", "sub-state", "feature", "assignee"
```

`neighboursOf` excludes the moving id from the rendered list before reading its neighbours, so a
drop one place down is `{after: theRowBelow, before: theRowBelowThat}` and not a no-op. A drop at
the top is `{after: null, before: firstRow}`, at the bottom `{after: lastRow, before: null}` —
the three shapes task 79 (c) names.

#### Interfaces — complete: `EstimatePicker.tsx` and `DuePicker.tsx`

```tsx
export function EstimatePicker(p: {
  head: IssueHead; scale: EstimateScale; anchor: DOMRect | null; onClose(): void;
}): JSX.Element {
  const { points, offScale } = estimateOptions(p.scale, p.head.estimate);
  const r = useReplica();
  const set = (v: number | null) => { r.mutate({ kind: "plan", id: p.head.id, estimate: v }); p.onClose(); };
  if (p.scale === "none") return <Popover anchor={p.anchor} onClose={p.onClose}>
    <p className="pl-mono">this team uses no estimates</p></Popover>;
  return (
    <Popover anchor={p.anchor} onClose={p.onClose}>
      {offScale !== null
        ? <p className="pl-mono">{offScale} is off this team&rsquo;s scale</p> : null}
      <ul className="pl-points">
        {points.map((n) => (
          <li key={n}><button type="button" className="pl-pt" data-on={n === p.head.estimate}
            onClick={() => set(n)}><span className="pl-n">{n}</span></button></li>))}
        <li><button type="button" className="pl-pt pl-mono" onClick={() => set(null)}>
          no estimate</button></li>
      </ul>
    </Popover>
  );
}
```

```tsx
export function DuePicker(p: {
  head: IssueHead; today: string; soonDays: number; anchor: DOMRect | null; onClose(): void;
}): JSX.Element {
  const r = useReplica();
  const w = dueWord(p.head.dueOn, p.today, p.soonDays);
  return (
    <Popover anchor={p.anchor} onClose={p.onClose}>
      <label className="pl-lbl">Due
        <input type="date" className="pl-date" value={p.head.dueOn ?? ""}
          onChange={(e) => r.mutate({ kind: "plan", id: p.head.id, dueOn: e.target.value || null })} />
      </label>
      {w.kind !== "none" ? <p className="pl-word" data-due={w.kind}>{w.text}</p> : null}
      <button type="button" className="pl-clear" onClick={() =>
        { r.mutate({ kind: "plan", id: p.head.id, dueOn: null }); p.onClose(); }}>Clear</button>
    </Popover>
  );
}
```

The row's due chip is `<span className="pl-due" data-due={w.kind}>{w.text}</span>` — the word is
the signal, `data-due` only tints it. There is no notification call in either module, and test 3
greps for one.

#### Interfaces — complete: `MoveDialog.tsx` and the template row

```tsx
export function MoveDialog(p: { head: IssueHead; onClose(): void }): JSX.Element {
  const r = useReplica();
  const [target, setTarget] = useState<Project | null>(null);
  const [said, setSaid] = useState<string | null>(null);
  const clears = target ? clearsOnMove(p.head, r.projectFacts(target.id)) : [];
  const bound = p.head.state === "started" || p.head.state === "in_review";   // task 79 (e)
  return (
    <Popover anchor={null} onClose={p.onClose}>
      <ProjectPicker onPick={setTarget} exclude={p.head.projectId} />
      {bound ? <p className="pl-mono">a lane is live on this issue; move it after review</p> : null}
      {clears.length > 0
        ? <ul className="pl-clears">{clears.map((c) => <li key={c}>{c} will be cleared</li>)}</ul>
        : null}
      <button type="button" disabled={!target || bound} onClick={async () => {
        const head = await r.move(p.head.id, target!.id).catch((e) => { setSaid(reasonOf(e)); return null; });
        if (head) { p.onClose(); openIssue(head.id); }       // lands on the new number
      }}>Move</button>
      <Said text={said} />
    </Popover>
  );
}
```

```tsx
// desktop/src/components/work/NewIssue.tsx ⚑ (task 68), one row above the title field:
<TemplateRow templates={r.templates(projectId)} onPick={(t: Template) => setD(fromTemplate(t, d))} />
// `fromTemplate` fills title, description, labels, serves, estimate and dueOn into the composer's
// own draft state, so every field is rendered by the composer it already had and every chip keeps
// its remove control (decision 9). No request is sent at pick time.
```

#### Interfaces — complete: the key rows and the drag

```ts
// desktop/src/work/keys/keymap.ts ⚑ — two rows, both scoped to an issue row (decision 1)
{ key: "D", scope: "list", target: "issue", verb: "plan.due",      label: "Due date…" },
{ key: "E", scope: "list", target: "issue", verb: "plan.estimate", label: "Estimate…" },
```

AX §5's `A`/`D` on a decision row keep their meaning because their rows carry
`target: "decision"`; the dispatcher picks by target, so neither file learns about the other.

```tsx
// desktop/src/components/issue/IssueRow.tsx ⚑ — the handle and the drop, in the row's props
onDrop={(toIndex: number) => {
  const { after, before } = neighboursOf(renderedIds, head.id, toIndex);
  r.mutate({ kind: "rank", id: head.id, after, before });   // optimistic move, one op, one undo
}}
```

The drag uses the list's existing keyboard-reorderable affordance as well (`Alt+↑`/`Alt+↓` on a
focused row send the same op), so ordering is reachable without a pointer — AX §7's rule that
every act has a key. Motion is `transform` only (DESIGN-SYSTEM.md) and explains the move.

#### Tests to write first, each with the mutant it kills

1. `D` on a decision row on the Desk defers it; `D` on an issue row opens the due picker.
   **Mutant:** one handler for both row kinds, branching inside this task's module.
2. `E` on an issue row whose team scale is `none` says *this team uses no estimates* and offers no
   point. **Mutant:** a Fibonacci list shown anyway, then refused by the Hub.
3. A due-soon issue produces no toast and no `Notification`. **Mutant:** a `Notification` fired
   (AX §10 anti-pattern 8); asserted by a grep over the due modules **and** a spy in the test.
4. A moved issue's old alias, opened by `O I`, lands on the new issue with a mono *moved from
   EP-142* line. **Mutant:** 404 on the alias a person still has in a document.
5. A dragged row in a **filtered** list sends the rendered neighbours and the Hub accepts them
   (task 79 (i)); the new order is the same for a second device reading the head.
   **Mutant:** a per-person local order written to disk (task 55's grep, extended here).
6. A drag is one Undo: Undo returns the row to its old place with one op.
   **Mutant:** the optimistic move and the server key are two entries.
7. A template's pre-filled `serves` is rendered as removable chips before create, and removing one
   removes it from the request. **Mutant:** silently applied, so the issue serves what nobody chose.
8. An off-scale stored estimate is shown with *off this team's scale* and is not rounded.
   **Mutant:** the client rewrites it to the nearest legal point (task 79 (a) says no).
9. A `started` issue's move button is disabled with the lane reason in place.
   **Mutant:** enabled, and the person meets a 422 after choosing a project.
10. A resolved blocker renders as a `relates` row with its cause line naming the closer.
    **Mutant:** the desktop rewrites the relation itself (task 79 (h) owns it).

#### Steps

- [ ] **Step 1: Check the base.** Task 46's `IssueRow`/`IssueHead`, task 53's key table shape and
  its `target` field, task 68's composer draft state, task 56's op kinds for `plan`, `rank`, `move`.
- [ ] **Step 2: The ten tests.** Run them: they fail.
- [ ] **Step 3: `work/plan.ts`.** Tests 5 (the pure half), 8 and the `dueWord` cases go green.
- [ ] **Step 4: The four pickers and the two key rows** (tests 1, 2, 3, 9).
- [ ] **Step 5: The row's drag and the template row** (tests 5, 6, 7), then the move landing and the
  moved-from line (test 4) and the relations cause line (test 10).
- [ ] **Step 6: Green**, then commit by pathspec, naming task 46's, 68's and 53's files as touched.

#### Acceptance

```bash
cd desktop && npx vitest run src/work/plan.test.ts src/components/issue/EstimatePicker.test.tsx \
  src/components/issue/DuePicker.test.tsx src/components/issue/TemplatePicker.test.tsx \
  src/components/issue/MoveDialog.test.tsx --reporter=dot 2>&1 | tail -3     # 10 passed
grep -rnE "new Notification|toast\(" desktop/src/components/issue/DuePicker.tsx desktop/src/work/plan.ts | wc -l   # 0
grep -rnE "localStorage|writeFile" desktop/src/work/plan.ts | wc -l          # 0
cd desktop && npx tsc --noEmit                                               # no output
```

Expected: `10 passed`, `0`, `0`, no output. None has been run; the code does not exist yet (OD-27).

#### What this task does not claim

- **Not the digest half of L-37.** Due-soon reaches the Desk digest in task 94; this task only
  refuses to shout.
- **Not recurring templates** (task 121, W4), and not template variables beyond the fields task
  78's `Template` carries.
- **Not the rank algebra.** The keys are the server's (task 79 (c)); the client sends two ids.
- **Not cross-org moves.** Task 79's route needs membership of both teams; a move outside the org
  is not in the matrix.
- **No measured drag latency** at 9,000 rows. The optimistic paint is local, the write is one
  request, and neither has been timed.

#### Gaps reported to the skeleton

1. **`target` on the key table is assumed.** Decision 1 needs task 53's rows to carry a target
   kind. If task 53's table has only `scope`, this task adds `target` there — a change to a
   task-53 file, stated and named in the commit.
2. **The Desk's `D` is in task 62's file.** Test 1 reads both bindings and so imports the Desk's
   row table; if task 62 registers its verb at mount rather than in a table, the test asserts
   through the real dispatcher instead (as task 68's test 1 does).
