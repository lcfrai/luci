### Task 130: milestones: the criterion-subset picker and convergence

W4, stream WD, 05 §1 L-72: *"a named subset of the brief's criteria with a date, reached when
those criteria are settled at the branch head. Convergence, not a bar."* The desktop half of task
120's milestones. Depends on task 125 (`Milestone`, `MilestoneView`, `RoadmapOp`'s
`milestone.create`, `milestone.patch` and `milestone.criteria`). Estimate 0.25 (2 h). Closes
**L-72** with task 120.

After this task a person on a feature's Criteria tab names a milestone, gives it a date and picks
which of the brief's criteria it needs, and every milestone is listed with each criterion's word.
A milestone reads *reached* only when every one of its criteria is settled and still on the brief.
Nothing counts: there is no *"2 of 3"*, no bar, no percentage.

**Decisions made here, so nobody re-derives them:**

1. **The picker is the writer's check.** Task 120 answers `criterion_not_on_brief` only for a
   brief whose criterion list the Hub holds (a templated brief), and leaves a hand-written brief's
   membership to the writer, naming this picker (`task-120.md:21-30`). So the picker offers
   **only** the criteria of the feature's branch brief at its head revision, the ones task 45's
   `PlanCriterion` marks `inBrief` (`task-45.md:84-87`); it never offers a typed id, another
   brief's criterion or a drifting one.
2. **Name and date are required by the picker, though the wire allows a null date.**
   `CreateMilestoneRequest.date` is `string | null` (`task-125.md:208`); a milestone without a date
   cannot be placed on the timeline (task 128), so *Save* is disabled until both are set and at
   least one criterion is picked. An existing milestone whose date was cleared elsewhere lists as
   *"no date"* and is never given one silently.
3. **The Hub's words, the desktop's `drifting`.** Each criterion's word is the view's
   `CriterionWord.word` (`task-125.md:75`, `:130`): the Hub answers `settled`, `contested`, `open`
   or `uncovered` (`task-120.md:31-39`). The Hub cannot answer `drifting` (it has no criterion
   text), so the list draws it: an id in `milestone.criterionIds` that is not in the head brief
   reads *"not on the brief at @7"*, where 7 is the head revision.
4. ***Reached* is the Hub's flag and the head check, never a tally.** `shownReached(view, head)` is
   `view.reached && every criterionId is in the head brief`. It is computed at render, stored
   nowhere, and reads no issue state (task 114's decision 6; 05 §3.3: *done* is not *settled*).
   An unreached milestone names what it waits on, in brief order: *"waits on: Uploads survive a
   dropped connection (open)"*.
5. **Reached and unreached differ by shape and by word.** A filled diamond and the word *reached*;
   a hollow diamond and *not reached*. `shownReached` and `MilestoneGlyph` are exported so task
   128's timeline marks draw the same shape from the same rule (a gap for 128, below).
6. **A refusal rolls the picker back.** The picker writes through `mutate` (SW.16); task 56 reverts
   the replica on refusal, and the picker listens to `onRefused` for its own op id and shows the
   fact line, naming the criterion from `details.field` (`criterionIds[1]`, `task-120.md:291`):
   *"Not saved: criterion 2 is not on this brief."* The picked set returns to what the replica
   holds; the person's name and date stay in the fields.

**Files:**
- `desktop/src/components/roadmap/MilestonePicker.tsx` — create and re-pick (new)
- `desktop/src/components/roadmap/MilestonePicker.test.tsx` — the picker's tests (new)
- `desktop/src/components/roadmap/MilestoneList.tsx` — the list, `shownReached`, `MilestoneGlyph`
  (new)
- `desktop/src/components/roadmap/MilestoneList.test.tsx` — the list's tests (new)
- `desktop/src/components/plan/FeatureTabs.tsx` ⚑ (task 85) — the Criteria tab mounts both, below
  the criteria (modified)
- `desktop/src/components/plan/plan.css` ⚑ (task 85) — `.ms-*` rules (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 45, 56, 120, 125 and
127. There is **no part for tasks 85, 55 or 128**: `FeatureTabs`, the Criteria tab's props and the
timeline's mark are their scopes' names and **UNVERIFIED**. `mutate` and `useWork` are imported from
`work/replica/useWork` as task 127 imports them (`task-127.md:161`), unverified against task 55.

#### Interfaces — complete: `desktop/src/components/roadmap/MilestoneList.tsx`

```tsx
// desktop/src/components/roadmap/MilestoneList.tsx — every milestone of a feature, each criterion
// with its word. Convergence, not a bar: no count, no share, no progress element.
import type { JSX } from "react";
import type { MilestoneView } from "../../api/work";                     // task 125
import type { BriefCriterion } from "../../work/suggestCriterion";      // task 42
import { ThreadNode } from "../thread/ThreadNode";                      // SW.14, task 36

/** The feature's branch brief at its head revision: the only criteria a milestone may name. */
export interface HeadBrief {
  briefKey: string;                     // "brief:bf-…@7", task 127's briefHeadKey
  rev: number;                          // 7
  criteria: readonly BriefCriterion[];  // brief order, task 45's inBrief criteria
}

export type MilestoneWord = "settled" | "contested" | "open" | "uncovered" | "drifting" | string;

export interface MilestoneLine { criterionId: string; text: string | null; word: MilestoneWord }

/** Decision 3. Brief order first; ids the head no longer has after, as `drifting`. */
export function milestoneLines(view: MilestoneView, head: HeadBrief): MilestoneLine[] {
  const want = new Set(view.milestone.criterionIds);
  const hub = new Map(view.criteria.map((c) => [c.criterionId, c.word]));
  const onBrief = head.criteria.filter((c) => want.has(c.criterionId))
    .map((c) => ({ criterionId: c.criterionId, text: c.text, word: hub.get(c.criterionId) ?? "open" }));
  const inHead = new Set(head.criteria.map((c) => c.criterionId));
  const off = view.milestone.criterionIds.filter((id) => !inHead.has(id))
    .map((id) => ({ criterionId: id, text: null, word: "drifting" as const }));
  return [...onBrief, ...off];
}

/** Decision 4: the Hub's flag, and nothing off the brief. Never stored. */
export function shownReached(view: MilestoneView, head: HeadBrief): boolean {
  const inHead = new Set(head.criteria.map((c) => c.criterionId));
  return view.reached && view.milestone.criterionIds.every((id) => inHead.has(id));
}

/** Decision 5: shape carries the state, so forced colours lose nothing. */
export function MilestoneGlyph({ reached }: { reached: boolean }): JSX.Element {
  return (
    <svg className="ms-glyph" width="10" height="10" viewBox="0 0 10 10" aria-hidden="true"
         data-shape={reached ? "diamond-filled" : "diamond-hollow"}>
      <path d="M5 0.5 L9.5 5 L5 9.5 L0.5 5 Z" fill={reached ? "currentColor" : "none"}
            stroke="currentColor" strokeWidth="1" />
    </svg>
  );
}

const lineText = (l: MilestoneLine, rev: number) =>
  l.word === "drifting" ? `not on the brief at @${rev}` : l.word;

export function MilestoneList(p: { views: readonly MilestoneView[]; head: HeadBrief;
                                   fmtDay(day: string): string; onEdit(milestoneId: string): void }): JSX.Element {
  if (p.views.length === 0) return <p className="ms-empty">No milestone on this feature.</p>;
  return (
    <ul className="ms-list">
      {p.views.map((v) => {
        const reached = shownReached(v, p.head);
        const lines = milestoneLines(v, p.head);
        const waits = lines.filter((l) => l.word !== "settled");
        return (
          <li key={v.milestone.id} className="ms-item" data-reached={reached || undefined}>
            <div className="ms-head">
              <MilestoneGlyph reached={reached} />
              <span className="ms-name">{v.milestone.name}</span>
              <span className="ms-date">{v.milestone.date ? p.fmtDay(v.milestone.date) : "no date"}</span>
              <span className="ms-state">{reached ? "reached" : "not reached"}</span>
              <button type="button" className="ms-edit" onClick={() => p.onEdit(v.milestone.id)}>Criteria…</button>
            </div>
            {!reached && waits.length > 0 && (
              <p className="ms-waits">waits on: {waits.map((l) => `${l.text ?? l.criterionId} (${lineText(l, p.head.rev)})`).join("; ")}</p>
            )}
            <ul className="ms-criteria">
              {lines.map((l) => (
                <li key={l.criterionId} data-word={l.word}>
                  {l.text !== null
                    ? <ThreadNode criterionId={l.criterionId} href={`#criterion-${l.criterionId}`} label={l.text} />
                    : null}
                  <span className="ms-ctext">{l.text ?? l.criterionId}</span>
                  <span className="ms-word">{lineText(l, p.head.rev)}</span>
                </li>
              ))}
            </ul>
          </li>
        );
      })}
    </ul>
  );
}
```

A drifting line has no `ThreadNode`: it has no source span on the head brief, and a node with no
source is not drawn (SW.14, `task-36.md:171-173`). The criterion id shows in mono in its place, and
`.ms-date` is mono with `tabular-nums`. `ThreadNode` is task 36's
`desktop/src/components/thread/ThreadNode.tsx` (`task-36.md:68`, signature `:174`). A criterion
the Hub's view omits reads `open`, the word that can never make a milestone reached.
