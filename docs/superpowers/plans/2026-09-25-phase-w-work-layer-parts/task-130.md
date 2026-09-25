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

#### Interfaces — complete: `desktop/src/components/roadmap/MilestonePicker.tsx`

```tsx
// desktop/src/components/roadmap/MilestonePicker.tsx — name, date, and a subset of the head
// brief's criteria. The writer's membership check (decision 1): nothing else can be picked.
import { useState, type JSX } from "react";
import type { MilestoneView } from "../../api/work";
import { mutate } from "../../work/replica/useWork";
import type { HeadBrief } from "./MilestoneList";

export interface MilestonePickerProps {
  featureId: string;
  head: HeadBrief;
  /** Absent: create. Present: re-pick this milestone's criteria (name and date edit in place). */
  editing?: MilestoneView;
  onDone(): void;
}

/** "criterionIds[1]" → "criterion 2", the person's count from one. */
export function refusalLine(details: Record<string, string>): string {
  const m = /^criterionIds\[(\d+)\]$/.exec(details.field ?? "");
  return m ? `Not saved: criterion ${Number(m[1]) + 1} is not on this brief.` : "Not saved: a criterion is not on this brief.";
}

export function MilestonePicker(p: MilestonePickerProps): JSX.Element {
  const held = p.editing?.milestone.criterionIds ?? [];
  const [name, setName] = useState(p.editing?.milestone.name ?? "");
  const [date, setDate] = useState(p.editing?.milestone.date ?? "");
  // Only head criteria can be in the set, even when editing one that holds a drifting id: the
  // drifting id is shown by the list and dropped only if the person saves without it.
  const [picked, setPicked] = useState<ReadonlySet<string>>(new Set(held));
  const [refused, setRefused] = useState<string | null>(null);
  const some = p.head.criteria.some((c) => picked.has(c.criterionId));
  // Decision 2 binds a new milestone; an edit changes criteria only, so a dateless one can re-pick.
  const ready = some && (!!p.editing || (name.trim() !== "" && /^\d{4}-\d{2}-\d{2}$/.test(date)));
  const order = p.head.criteria.filter((c) => picked.has(c.criterionId)).map((c) => c.criterionId);

  const save = () => {
    setRefused(null);
    const h = p.editing
      ? mutate({ kind: "milestone.criteria", milestoneId: p.editing.milestone.id, criterionIds: order, prev: held })
      : mutate({ kind: "milestone.create", featureId: p.featureId, req: { name: name.trim(), date, criterionIds: order } });
    void h.settled.then((r) => {
      if (r === "applied") return p.onDone();
      setPicked(new Set(held));                               // decision 6: back to what the replica holds
      setRefused(r.code === "criterion_not_on_brief" ? refusalLine(r.details) : r.message);
    });
  };

  return (
    <form className="ms-picker" onSubmit={(e) => { e.preventDefault(); if (ready) save(); }}>
      <label>Name <input value={name} onChange={(e) => setName(e.target.value)} maxLength={120}
        readOnly={!!p.editing} required /></label>
      <label>Date <input type="date" value={date} onChange={(e) => setDate(e.target.value)}
        readOnly={!!p.editing} required /></label>
      <fieldset>
        <legend>Criteria from {p.head.briefKey}</legend>
        {p.head.criteria.map((c) => (
          <label key={c.criterionId} className="ms-pick">
            <input type="checkbox" checked={picked.has(c.criterionId)}
              onChange={(e) => setPicked((s) => {
                const n = new Set(s); if (e.target.checked) n.add(c.criterionId); else n.delete(c.criterionId); return n;
              })} />
            <span>{c.text}</span>
            <span className="ms-cid" title="criterion id">{c.criterionId}</span>
          </label>
        ))}
      </fieldset>
      {refused && <p className="ms-refused" role="status">{refused}</p>}
      <button type="submit" disabled={!ready}>Save</button>
      <button type="button" onClick={p.onDone}>Cancel</button>
    </form>
  );
}
```

`.ms-cid` is the machine id *in peek*: mono, `--t-fact` size, shown on hover and focus only
(02's peek rule), so the words lead. Renaming and re-dating a milestone are `milestone.patch`
(`task-125.md:518`) from the timeline (task 129's drag, task 89's `D`); the picker's edit mode
changes criteria only, so one op carries one change and its Undo is exact. Renaming has no surface
in this task (a gap, below).

#### Tests to write first — `desktop/src/components/roadmap/MilestoneList.test.tsx`

```tsx
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import type { MilestoneView } from "../../api/work";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { MilestoneList, shownReached, type HeadBrief } from "./MilestoneList";

const BK = "brief:bf-1@7";
const head: HeadBrief = { briefKey: BK, rev: 7, criteria: [
  { briefKey: BK, criterionId: "done:1", text: "the resume test passes" },
  { briefKey: BK, criterionId: "done:2", text: "Uploads survive a dropped connection" },
  { briefKey: BK, criterionId: "done:3", text: "upload works for PDF, XLSX and images" },
] };
const view = (ids: string[], words: Record<string, string>, reached: boolean): MilestoneView => ({
  milestone: { id: "ml-1", featureId: "ft-1", name: "Beta", date: "2026-10-14", criterionIds: ids,
               createdAt: "2026-09-26T00:00:00Z", updatedAt: "2026-09-26T00:00:00Z" },
  criteria: ids.map((criterionId) => ({ criterionId, word: words[criterionId] ?? "open" })), reached,
});
const show = (v: MilestoneView) =>
  render(<MilestoneList views={[v]} head={head} fmtDay={() => "14 Oct"} onEdit={() => {}} />);

// mutant killed: "reached by majority" (two settled of three shown as reached)
it("one open criterion keeps the milestone unreached, and names it", () => {
  show(view(["done:1", "done:2", "done:3"], { "done:1": "settled", "done:3": "settled" }, false));
  expect(screen.getByText("not reached")).toBeTruthy();
  expect(screen.getByText(/^waits on: Uploads survive a dropped connection \(open\)$/)).toBeTruthy();
});

// mutant killed: "dropped criteria count as settled" (the Hub's reached taken alone)
it("a criterion removed from the brief keeps it unreached, even when the Hub says reached", () => {
  const v = view(["done:1", "done:9"], { "done:1": "settled", "done:9": "settled" }, true);
  expect(shownReached(v, head)).toBe(false);
  show(v);
  expect(screen.getByText("not on the brief at @7")).toBeTruthy();
  expect(screen.queryByText("reached")).toBeNull();
});

// mutant killed: a completion count ("2 of 3", "67%", a progress element)
it("the list shows words, never a count", () => {
  const { container } = show(view(["done:1", "done:2"], { "done:1": "settled" }, false));
  expect(container.textContent).not.toMatch(/\d+\s*(of|\/)\s*\d+|%/);
  expect(container.querySelector("progress, [role=progressbar], meter")).toBeNull();
  expectNoPrediction(container);
});

// mutant killed: "colour only" (under forced-colors, shape and word remain)
it("reached and unreached differ by shape and by word", () => {
  const a = show(view(["done:1"], { "done:1": "settled" }, true)).container;
  const b = show(view(["done:2"], {}, false)).container;
  expect(a.querySelector(".ms-glyph")!.getAttribute("data-shape")).toBe("diamond-filled");
  expect(b.querySelector(".ms-glyph")!.getAttribute("data-shape")).toBe("diamond-hollow");
  expect(a.querySelector(".ms-state")!.textContent).toBe("reached");
  expect(b.querySelector(".ms-state")!.textContent).toBe("not reached");
});
```

#### Tests to write first — `desktop/src/components/roadmap/MilestonePicker.test.tsx`

```tsx
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import type { HeadBrief } from "./MilestoneList";
import { MilestonePicker } from "./MilestonePicker";

let settle: (r: unknown) => void = () => {};
const mutate = vi.fn(() => ({ id: "op-1", undoable: true, undo() {},
  settled: new Promise((res) => { settle = res; }) }));
vi.mock("../../work/replica/useWork", () => ({ mutate }));

const BK = "brief:bf-1@7";
const head: HeadBrief = { briefKey: BK, rev: 7, criteria: [
  { briefKey: BK, criterionId: "done:1", text: "the resume test passes" },
  { briefKey: BK, criterionId: "done:2", text: "Uploads survive a dropped connection" },
] };
beforeEach(() => mutate.mockClear());

// mutant killed: "criteria from another brief" (the project's every brief, or a typed id)
it("offers exactly the head brief's criteria, in brief order", () => {
  const { container } = render(<MilestonePicker featureId="ft-1" head={head} onDone={() => {}} />);
  const offered = [...container.querySelectorAll(".ms-cid")].map((e) => e.textContent);
  expect(offered).toEqual(["done:1", "done:2"]);
  // no free-typed id field: the only non-checkbox inputs are the name and the date
  expect(container.querySelectorAll("input:not([type=checkbox])").length).toBe(2);
  expectNoPrediction(container);
});

it("Save stays disabled until a name, a date and one criterion are set", () => {
  render(<MilestonePicker featureId="ft-1" head={head} onDone={() => {}} />);
  const save = screen.getByRole("button", { name: "Save" }) as HTMLButtonElement;
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Beta" } });
  fireEvent.change(screen.getByLabelText("Date"), { target: { value: "2026-10-14" } });
  expect(save.disabled).toBe(true);
  fireEvent.click(screen.getByLabelText(/Uploads survive/));
  expect(save.disabled).toBe(false);
});

// mutant killed: "optimistic subset kept" after the Hub refuses
it("a criterion_not_on_brief refusal rolls the picked set back and names the criterion", async () => {
  const editing = { milestone: { id: "ml-1", featureId: "ft-1", name: "Beta", date: "2026-10-14",
    criterionIds: ["done:1"], createdAt: "", updatedAt: "" }, criteria: [], reached: false };
  render(<MilestonePicker featureId="ft-1" head={head} editing={editing} onDone={() => {}} />);
  fireEvent.click(screen.getByLabelText(/Uploads survive/));
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  expect(mutate).toHaveBeenCalledWith({ kind: "milestone.criteria", milestoneId: "ml-1",
    criterionIds: ["done:1", "done:2"], prev: ["done:1"] });
  await act(async () => settle({ code: "criterion_not_on_brief", message: "", details: { field: "criterionIds[1]" } }));
  expect(screen.getByRole("status").textContent).toBe("Not saved: criterion 2 is not on this brief.");
  expect((screen.getByLabelText(/Uploads survive/) as HTMLInputElement).checked).toBe(false);
  expect((screen.getByLabelText(/the resume test/) as HTMLInputElement).checked).toBe(true);
});
```

#### Steps

1. Write both test files; run them; they fail on the missing modules.
2. Write `MilestoneList.tsx` (`milestoneLines`, `shownReached`, `MilestoneGlyph`, the list); the list
   tests pass.
3. Write `MilestonePicker.tsx`; the picker tests pass.
4. In ⚑ `FeatureTabs.tsx`, the Criteria tab builds `HeadBrief` from its `inBrief` criteria and
   task 127's `briefHeadKey`, reads `useWork((r) => r.milestonesOf(featureId))` (task 125,
   `task-125.md:489`), and mounts the list with *New milestone* opening the picker in place.
5. Append the `.ms-*` rules to ⚑ `plan.css`: `.ms-date` and `.ms-cid` in `var(--mono)` with
   `tabular-nums`; no transition; the glyph's colour is `currentColor`, so forced colours keep it.
6. Run the acceptance; commit.

#### Acceptance

```bash
cd desktop && npx vitest run src/components/roadmap/MilestoneList.test.tsx \
  src/components/roadmap/MilestonePicker.test.tsx src/components/plan
# expect: MilestoneList 4 passed, MilestonePicker 3 passed; task 85's plan tests pass as before
cd desktop && npx tsc --noEmit
# expect: exit 0
cd desktop && grep -n -E "of [0-9]|progress|percent" src/components/roadmap/MilestoneList.tsx \
  src/components/roadmap/MilestonePicker.tsx
# expect: no output
```

#### What this task does NOT claim

- That a milestone is reached **at the branch head**. The Hub reads each criterion's latest recorded
  verdict under any revision of the branch brief (`task-120.md:31-39`); this task shows that word
  and adds only the head-brief check.
- The timeline's milestone marks (task 128 draws them from the same view; see gap 1).
- That the milestone's date will be met. A date is a person's.
- Renaming a milestone or deleting one (gaps 2 and 3).

#### Gaps found while writing this part

1. **Task 128 should draw its marks with `shownReached` and `MilestoneGlyph`.** Its scope reads
   *reached* from `MilestoneView` alone (`tasks-121-140.md:251-252`), which shows a milestone
   holding a criterion off the head brief as reached. The export exists here; 128's part should
   import it (or 128 lands after 130 in stream order).
2. **No rename surface.** `milestone.patch` carries `name` (`task-125.md:209`), but neither 128,
   129 nor this scope names where a person renames a milestone.
3. **No milestone delete.** Task 120's routes have none (`task-125.md:533-534`), so a wrong
   milestone can only be re-picked; undo of a create is refused.
4. **No part for tasks 85, 55 or 128** (Criteria tab props, `useWork`/`mutate`, the marks).
