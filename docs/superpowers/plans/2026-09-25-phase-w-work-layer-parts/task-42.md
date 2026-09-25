### Task 42: Triage: the view, number keys, `T` with a text-match criterion suggestion

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the Work lens,
`IssueRow`, `work.css`) and task 41 (the hold model and `listKeys.ts`). **Also depends on task 46**,
which the skeleton's row does not name (a gap, named in the return): the duplicate key renders
task 46's `DuplicateOfPicker`, the priority refusal opens its `PriorityPicker`, the moves use its
`handMove`, and `CriterionPicker` is built on its `Picker` (`task-46.md:158-175`, `:247`,
`:295-305`, `:328-331`). Uses SW.1 (`serves [{briefKey, criterionId}]`), SW.8, SW.13
(`ViewHeader`'s `actions`), SW.14 (`ThreadNode`), SW.15 and SW.16 (`useWork`, `mutate`,
`onRefused`), and task 11's team policy and snooze.

Parity: closes **L-46** (the Triage view; its Desk rows are task 63), **L-47** (the number keys;
`Shift+R` accept-and-Start is task 61) and **L-48**'s surface (the refusal shown; the check is
task 11's `TriageExitRefused`, `task-11.md:187-199`), and **L-10** with task 48. Design: 05 §4.3
(`T` and `1`-`4`, the key table's rows at `05-design.md:1006` and `:1008`), §3.3 (`triage`), §2.3 point 4 (the
thread state, `05-design.md:613-618`), §5.3's demo (`05-design.md:1175`).

**What the reader does** (the demo, 05 §5.3): the imported backlog is in Triage, every row reading
*not tied · imported*. The reader moves to EP-142 and presses `T`. A picker opens beside the row
with *Uploads survive a dropped connection* first, its why reading `matches: dropped, connection`.
`Enter` attaches it: a new revision, and the row's thread state becomes the criterion's node.
`1` accepts it. Had the team required a priority, the row would flash `--fail`, say why, and open
the priority picker in place.

**Decisions made here, so nobody re-derives them:**

1. **Suggestion is shared words, shown as words.** `suggestCriteria` normalises both texts the same
   way (lower case; NFD with combining marks removed; split on anything that is not a letter or a
   digit; a fixed English stop-word list removed; tokens of one character removed), scores a
   criterion by the count of **distinct** shared terms, drops criteria with none, and breaks ties
   in brief order. There is no stemming (`upload` and `uploads` do not match) and no weighting.
   The picker shows `matches: dropped, connection`, never a number, a percentage or a bar (AX
   Principle 7; anti-pattern 12). A score is an internal sort key and is not exported.
2. **`1` accepts into the team's default state, and never into `triage`.** `TeamPolicy.defaultState`
   is the state a created issue takes (`task-11.md:370`); the Hub's own default is `backlog`
   (`task-11.md:184`). If a policy names `triage` (task 20's fixture does, `task-20.md:273`),
   accepting there would be a no-op, so `1` moves to `backlog`. The move is `handMove(to)`
   (`cause: "hand"`), written as the person.
3. **`2` is one act.** It opens task 46's `DuplicateOfPicker`, which already sends the relation and
   the `duplicate` move as **one** `mutate([...])` batch, so one Undo (`task-46.md:295-305`). Task 42
   adds no second duplicate path.
4. **`3` declines with an optional one-line reason.** The row opens an inline field in place (no
   dialog): `Enter` sends `setState` to `cancelled` with `handMove("cancelled", reason)`, an empty
   reason allowed; `Esc` closes it and sends nothing.
5. **`4` snoozes to the next 09:00 after tomorrow begins, in the reader's time zone.** Task 11 gives
   snooze a Hub field (`IssueHead.snoozedUntil`, `PatchIssueRequest.snoozedUntil`,
   `task-11.md:175-176`), so the scope's **UNVERIFIED** resolves: `4` is not a stub and no finding
   against task 03 is needed. A snoozed issue stays in Triage and sorts after the rest; it is
   never hidden (task 11, AX §7.4), and its row says `snoozed until Tue 09:00` in mono. Choosing
   another time and a reason is W2's *defer with a reason* (`H`, 05:1237), not this task's.
6. **The refusal names its reason and opens the fix.** The Hub answers a refused accept with 422
   `policy_refused`, `details.policy = "priorityToLeaveTriage"` (`task-11.md:379-381`). The replica
   rolls the optimistic move back (task 56) and calls `onRefused`; the Triage row then flashes
   `--fail` once, shows *Needs a priority to leave Triage* under its title, and opens
   `PriorityPicker` in place. Choosing a priority does **not** re-send the accept by itself: the
   reader presses `1` again, so no state move is ever sent twice without a key.
7. **The thread state reads on every row.** `threadState.kind === "not_tied"` renders
   `not tied · <reason>` in sans at `--ink-2`; `on_thread` renders one `ThreadNode` per served
   criterion (SW.14); `project_level` renders `project-level`. After `T`, the Hub's new revision
   carries the serve and the head's `threadState` becomes `on_thread` (task 09's rule; the desktop
   draws what the head says and never sets the thread state itself).
8. **Keys are the Triage list element's own.** `1`-`4` and `T` are bound in `TriageView`'s list
   `onKeyDown`, after `isBareListKey` (task 41). The Work list does not see them, and neither does
   a text field, the decline field included. Task 53 moves them into `KEYMAP` unchanged.

**Files** (the skeleton's file map lists `TriageView.tsx`; it omits `suggestCriterion.ts`,
`CriterionPicker.tsx` and their tests, a gap named in the return):
- `desktop/src/work/suggestCriterion.ts` (+ `suggestCriterion.test.ts`) — the pure matcher (new)
- `desktop/src/components/work/CriterionPicker.tsx` (+ `CriterionPicker.test.tsx`) — `T`'s picker; tasks 46 and 47 reuse it (new)
- `desktop/src/components/work/TriageView.tsx` (+ `TriageView.test.tsx`) — the view and its keys (new)
- `desktop/src/components/work/WorkLens.tsx` — a *Triage* action in its `ViewHeader`, switching the lens's view (modified, task 40's)
- `desktop/src/components/work/work.css` — `.wk-triage-*`, the `--fail` flash, the decline field (modified)

#### Interfaces — complete: `desktop/src/work/suggestCriterion.ts`

```ts
// desktop/src/work/suggestCriterion.ts — task 42. W1's criterion suggestion is text match
// (05 §2.3 point 4); W4's gateway suggestion (L-51, L-52) replaces the source, not the shape.

/** A criterion as the picker lists it: which brief it is in, in brief order. */
export interface BriefCriterion { briefKey: string; criterionId: string; text: string }

export interface CriterionSuggestion {
  briefKey: string;
  criterionId: string;
  text: string;
  /** The shared normalised terms, in the order they appear in the criterion: the why. */
  matched: readonly string[];
}

// A fixed list, so a suggestion never changes because a library did.
const STOP = new Set(("a an and are as at be but by for from has have if in into is it its not of on or " +
  "our so than that the their then there these this to was we were when where which while who will " +
  "with without after before must should can may").split(" "));

/** Lower case, marks removed, split on non-letters and non-digits, stop words and 1-char tokens out. */
export function terms(text: string): string[] {
  return text.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase()
    .split(/[^\p{L}\p{N}]+/u)
    .filter((t) => t.length > 1 && !STOP.has(t));
}

export function suggestCriteria(
  issue: { title: string; description: string },
  criteria: readonly BriefCriterion[],   // brief order: the tie-break
  limit = 3,
): CriterionSuggestion[] {
  const mine = new Set(terms(`${issue.title} ${issue.description}`));
  return criteria
    .map((c, order) => {
      const matched = [...new Set(terms(c.text))].filter((t) => mine.has(t));
      return { order, s: { briefKey: c.briefKey, criterionId: c.criterionId, text: c.text, matched } };
    })
    .filter((x) => x.s.matched.length > 0)
    .sort((x, y) => y.s.matched.length - x.s.matched.length || x.order - y.order)
    .slice(0, Math.max(0, limit))
    .map((x) => x.s);
}
```

`CriterionSuggestion` has no `score`, `confidence` or `rank` field: the order is the only place the
count shows, and the why is the words.

