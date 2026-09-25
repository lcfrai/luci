### Task 129: the timeline, part 2: drag to reschedule with Undo; dependencies, a violated order in signal

W4, stream WD, 05 §1 L-90 (*drag to reschedule*) and L-74 (*feature dependencies*). Depends on
task 128 (the timeline's layout and `timelineScale`), and through it on task 125 (the W4 wire types
and `RoadmapOp`). Estimate 0.375 (3 h). Closes **L-90**, and **L-74** with task 120.

After this task a person moves a feature's target mark or a milestone's mark along the timeline
with the pointer, and one write lands as them with an Undo chip; a person links two features with
*Blocks…* from the palette; and a dependency whose blocked feature is due before its blocker is
drawn in `--signal` with a sentence saying so. Nothing moves a date except a person.

**Decisions made here, so nobody re-derives them:**

1. **A drag is a preview until the drop, and the preview is `transform` only.** Pointer moves
   change a CSS `translateX` on the dragged mark and the mono label beside it; nothing is sent and
   nothing in the replica changes. The drop calls `mutate` **once** (SW.16, `task-56.md:346-349`).
   `Escape`, `pointercancel` or a drop on the start day sends nothing (DESIGN-SYSTEM motion: the
   motion explains the cause, a date following the hand).
2. **Dates are calendar days, never instants.** `targetDate` and `Milestone.date` are `YYYY-MM-DD`
   (`task-125.md:82`, `:125`). The drag snaps to the nearest whole day by `dayAt()`, which inverts
   task 128's scale using the one fact task 128 fixes: every calendar day in the person's time zone
   has the same width, so a daylight-saving day is no narrower. Day arithmetic is on the plain
   date (`addDays("2026-10-03", 1) === "2026-10-04"`), so no time zone can shift it.
3. **Two write paths, one op each.** A feature's target is task 83's feature patch op (the W2 op
   task 76's `PATCH /v1/features/{featureId}` serves; name **UNVERIFIED**, no task 83 part on the
   tree), written here as `{kind: "feature.patch", featureId, req: {targetDate}, prev: {targetDate}}`.
   A milestone's date is task 125's `milestone.patch` (`task-125.md:518`), with `prev`. Undo is task
   56's `MutateHandle.undo()`, which sends the inverse built from `prev`: the old date, as a new
   write by the person.
4. **The desktop's `orderViolated` is a preview and the Hub's value wins.** After a drop, before the
   Hub answers, `orderViolated(blocker, blocked)` is recomputed with the Hub's rule, copied from
   task 115's `depOf` (`task-115.md:657-661`): *both dates set and the blocked feature's date is
   before its blocker's*; either date missing is not a violation. On the next `feature_dep.*` or
   feature event the replica's `FeatureDependencyView.orderViolated` replaces the preview.
5. **A violated order is a word, then a colour.** Each violated connector carries the sentence
   *"targets 3 Oct, before its blocker EP-2's 10 Oct"* as visible text beside the blocked mark and
   as the connector's accessible name. `--signal` is added to it; nothing pulses, blinks or
   animates (AX §9's no-stage rule; 05 §4.4 motion).
6. **A loop is the Hub's refusal, not the desktop's check.** *Blocks…* paints the connector at once
   (task 56's optimism) and sends `featureDep.add` (`task-125.md:520`). A `dependency_cycle` refusal
   rolls it back through `onRefused`. Task 120's refusal carries only `details.from` and
   `details.blocks` (`task-120.md:304`), so the fact line finds the loop in the replica's own
   dependencies, a walk from `blocks` back to `from` (`findLoop`, below), and names it (*"EP-2 →
   EP-5 → EP-2"*). The walk only words the refusal; it never refuses. When the replica cannot see
   the path (a link through a feature the caller cannot read), the line reads *"EP-2 blocking EP-5
   would close a loop"* and names no feature it cannot see.
7. **A granted reader has no handle.** `useProjectAccess(projectId)` (`task-126.md:86-88`) answering
   `"granted"` renders marks without the drag handle, the date picker and the palette verbs; the
   write gate of `task-126.md:43-48` closes the keyboard path.

**Files:**
- `desktop/src/components/roadmap/useTimelineDrag.ts` — `dayAt`, `addDays`, the drag state
  machine as a hook (new)
- `desktop/src/components/roadmap/useTimelineDrag.test.ts` — the pure tests (new)
- `desktop/src/components/roadmap/TimelineDrag.tsx` — the handle, the preview label, the drop (new)
- `desktop/src/components/roadmap/DependencyLines.tsx` — the connectors, the violated sentence (new)
- `desktop/src/components/roadmap/orderViolated.ts` — the Hub's rule, copied (new)
- `desktop/src/components/roadmap/TimelineDrag.test.tsx` — drag, Escape, Undo, grant, loop (new)
- `desktop/src/components/roadmap/DependencyLines.test.tsx` — forced colours, the sentence (new)
- `desktop/src/components/roadmap/Timeline.tsx` ⚑ (task 128) — mounts `DependencyLines` (modified)
- `desktop/src/components/roadmap/TimelineRow.tsx` ⚑ (task 128) — wraps each mark (modified)
- `desktop/src/components/roadmap/timeline.css` ⚑ (task 128) — the handle, the signal (modified)
- `desktop/src/components/palette/commands.ts` ⚑ (task 59) — *Blocks…*, *Remove dependency*
  (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 56, 115, 120, 125 and
126. There is **no part for tasks 128, 59, 76, 83 or 89**, so `Timeline`, `TimelineRow`,
`timelineScale`'s exact signature, `buildCommands`, the feature patch op and the `D` picker are
their scopes' names and **UNVERIFIED**. The scale is taken as the scope writes it
(`tasks-121-140.md:245-247`): `timelineScale(zoom, from, to, widthPx, tz) → {x(day), ticks}`, with
`x` taking a `YYYY-MM-DD` day; if task 128's part takes a `Date`, `dayAt` adapts in one line.

#### Interfaces — complete: `desktop/src/components/roadmap/orderViolated.ts`

```ts
// desktop/src/components/roadmap/orderViolated.ts — the Hub's rule, copied from task 115's depOf
// (fake_work_roadmap.go, task-115.md:657-661). A preview only: the Hub's value wins on the next event.

/** YYYY-MM-DD compares as text, which is the Hub's `<` on the same strings. */
export function orderViolated(blockerTarget: string | null, blockedTarget: string | null): boolean {
  return blockerTarget !== null && blockedTarget !== null && blockedTarget < blockerTarget;
}

/** The sentence a violated connector carries, in words, never a colour alone. `fmt` is the
 *  person's short day format ("3 Oct"), the one task 128's axis labels use. */
export function violationSentence(
  blocked: { targetDate: string }, blocker: { alias: string; targetDate: string },
  fmt: (day: string) => string,
): string {
  return `targets ${fmt(blocked.targetDate)}, before its blocker ${blocker.alias}'s ${fmt(blocker.targetDate)}`;
}
```

#### Interfaces — complete: `desktop/src/components/roadmap/useTimelineDrag.ts`

```ts
// desktop/src/components/roadmap/useTimelineDrag.ts — a mark follows the hand; one write on drop.
import { useCallback, useRef, useState } from "react";
import type { TimelineScale } from "./timelineScale";   // task 128 (UNVERIFIED name)

/** Plain calendar arithmetic on YYYY-MM-DD; UTC is only the carrier, so no zone shifts a day. */
export function addDays(day: string, n: number): string {
  const [y, m, d] = day.split("-").map(Number);
  const t = new Date(Date.UTC(y, m - 1, d + n));
  return t.toISOString().slice(0, 10);
}

/** The whole day under px. Task 128's rule: every calendar day has one width in `tz`, so the
 *  width of the first day is the width of all of them (decision 2). Never a fraction of a day. */
export function dayAt(scale: TimelineScale, from: string, px: number): string {
  const x0 = scale.x(from);
  const w = scale.x(addDays(from, 1)) - x0;
  return addDays(from, Math.round((px - x0) / w));
}

/** What is being dragged. `start` is the date the drag began at, the Undo target. */
export type DragSubject =
  | { kind: "feature"; featureId: string; start: string }
  | { kind: "milestone"; milestoneId: string; start: string };

export type DragState =
  | { phase: "idle" }
  | { phase: "dragging"; subject: DragSubject; pointerId: number; originPx: number; day: string; dx: number };

export interface DragCallbacks {
  /** Called once, on a drop that changed the day. Returns nothing: the caller calls mutate. */
  onDrop(subject: DragSubject, day: string): void;
}

export interface TimelineDrag {
  state: DragState;
  /** Spread on the handle. Absent (undefined) for a granted reader: no handle at all. */
  handleProps(subject: DragSubject): {
    onPointerDown(e: React.PointerEvent): void;
    onPointerMove(e: React.PointerEvent): void;
    onPointerUp(e: React.PointerEvent): void;
    onPointerCancel(e: React.PointerEvent): void;
    onKeyDown(e: React.KeyboardEvent): void;   // Escape cancels
  };
}

export function useTimelineDrag(scale: TimelineScale, rangeFrom: string, cb: DragCallbacks): TimelineDrag {
  const [state, setState] = useState<DragState>({ phase: "idle" });
  const live = useRef(state);
  live.current = state;
  const cancel = useCallback(() => setState({ phase: "idle" }), []);
  const handleProps = (subject: DragSubject) => ({
    onPointerDown(e: React.PointerEvent) {
      (e.currentTarget as Element).setPointerCapture?.(e.pointerId);
      setState({ phase: "dragging", subject, pointerId: e.pointerId, originPx: e.clientX, day: subject.start, dx: 0 });
    },
    onPointerMove(e: React.PointerEvent) {
      const s = live.current;
      if (s.phase !== "dragging" || s.pointerId !== e.pointerId) return;
      const px = scale.x(s.subject.start) + (e.clientX - s.originPx);
      const day = dayAt(scale, rangeFrom, px);
      setState({ ...s, day, dx: scale.x(day) - scale.x(s.subject.start) });   // snapped: whole days
    },
    onPointerUp(e: React.PointerEvent) {
      const s = live.current;
      if (s.phase !== "dragging" || s.pointerId !== e.pointerId) return;
      setState({ phase: "idle" });
      if (s.day !== s.subject.start) cb.onDrop(s.subject, s.day);             // one write, or none
    },
    onPointerCancel: cancel,
    onKeyDown(e: React.KeyboardEvent) { if (e.key === "Escape" && live.current.phase === "dragging") cancel(); },
  });
  return { state, handleProps };
}
```

`dx` is the only thing the preview renders: a `transform` of `translateX(dx px)` on the mark and
its label. No `left`, no `width`, no `transition` on the drag (it follows the hand; the
snap is the cause, per DESIGN-SYSTEM's motion rule).

#### Interfaces — complete: `desktop/src/components/roadmap/TimelineDrag.tsx`

```tsx
// desktop/src/components/roadmap/TimelineDrag.tsx — wraps one mark (a target or a milestone).
import type { ReactNode } from "react";
import { mutate } from "../../work/replica/useWork";                // SW.16, as task 127 imports it
import { useProjectAccess } from "../../work/access/projectAccess"; // task 126
import type { WorkOp } from "../../work/replica/ops";
import type { DragSubject, TimelineDrag as Drag } from "./useTimelineDrag";

export interface TimelineDragProps {
  projectId: string;
  subject: DragSubject;
  drag: Drag;                 // one useTimelineDrag per Timeline, shared by its rows
  fmtDay(day: string): string;  // "14 Oct", the axis's format (task 128)
  children: ReactNode;          // the mark task 128 draws
}

/** The op a drop writes (decision 3). `feature.patch` is task 83's op name (UNVERIFIED). */
export function dropOp(subject: DragSubject, day: string): WorkOp {
  return subject.kind === "feature"
    ? ({ kind: "feature.patch", featureId: subject.featureId,
         req: { targetDate: day }, prev: { targetDate: subject.start } } as WorkOp)
    : ({ kind: "milestone.patch", milestoneId: subject.milestoneId,
         req: { date: day }, prev: { date: subject.start } } as WorkOp);
}

export function TimelineDrag({ projectId, subject, drag, fmtDay, children }: TimelineDragProps) {
  const granted = useProjectAccess(projectId) === "granted";
  const s = drag.state;
  const mine = s.phase === "dragging" && sameSubject(s.subject, subject);
  const dx = mine ? s.dx : 0;
  if (granted) return <span className="tl-mark">{children}</span>;   // decision 7: no handle
  return (
    <span className="tl-mark tl-mark--draggable" data-dragging={mine || undefined}>
      <span className="tl-mark-body" style={{ transform: `translateX(${dx}px)` }}
        role="slider" tabIndex={0}
        aria-label={`${subject.kind === "feature" ? "Target" : "Milestone"} ${fmtDay(subject.start)}`}
        aria-valuetext={fmtDay(mine ? s.day : subject.start)}
        {...drag.handleProps(subject)}>
        {children}
      </span>
      {mine && <span className="tl-drag-day" style={{ transform: `translateX(${dx}px)` }}>{fmtDay(s.day)}</span>}
    </span>
  );
}

const sameSubject = (a: DragSubject, b: DragSubject) =>
  a.kind === b.kind && (a.kind === "feature" ? a.featureId === (b as typeof a).featureId
                                             : a.milestoneId === (b as typeof a).milestoneId);

/** The Timeline's onDrop: one mutate, then the chip. Undo is the handle's inverse (task 56). */
export function useDropWriter(showChip: (entryId: string, label: string) => void, fmtDay: (d: string) => string) {
  return (subject: DragSubject, day: string) => {
    const h = mutate(dropOp(subject, day));
    showChip(h.id, `${subject.kind === "feature" ? "Target" : "Milestone"} ${fmtDay(day)}`);
  };
}
```

`showChip` mounts task 56's `<UndoChip ledger entryId label onUndo/>` (`task-56.md:745`), whose
lapse is `--undo-window` (SW.11). `.tl-drag-day` is `font-family: var(--mono)` with
`font-variant-numeric: tabular-nums`, so the date under the hand does not jitter as it changes.
The keyboard path is not here: task 89's `D` opens the date picker on the focused mark, and the
picker's pick calls the same `dropOp`, so both paths write the same op.

#### Interfaces — complete: `desktop/src/components/roadmap/DependencyLines.tsx`

```tsx
// desktop/src/components/roadmap/DependencyLines.tsx — one connector per dependency, drawn in SVG
// over the rows. The Hub's orderViolated wins; the preview covers only the time until it answers.
import type { Feature, FeatureDependencyView } from "../../api/work";
import { orderViolated, violationSentence } from "./orderViolated";

export interface DependencyLinesProps {
  deps: readonly FeatureDependencyView[];       // replica.depsOf, both directions (task 125)
  features: ReadonlyMap<string, Feature & { alias: string }>;
  /** The mark's centre, from task 128's layout; null when the feature has no target set. */
  anchor(featureId: string): { x: number; y: number } | null;
  /** Feature ids with a write in flight (task 56's pendingOf for the feature patch). */
  pending: ReadonlySet<string>;
  fmtDay(day: string): string;
}

/** Hub value unless one end has a write in flight; then the copied rule on the painted dates. */
export function violatedNow(d: FeatureDependencyView, features: DependencyLinesProps["features"],
                            pending: ReadonlySet<string>): boolean {
  if (!pending.has(d.from) && !pending.has(d.blocks)) return d.orderViolated;
  return orderViolated(features.get(d.from)?.targetDate ?? null, features.get(d.blocks)?.targetDate ?? null);
}

export function DependencyLines({ deps, features, anchor, pending, fmtDay }: DependencyLinesProps) {
  return (
    <svg className="tl-deps" aria-hidden="false" role="list">
      {deps.map((d) => {
        const a = anchor(d.from), b = anchor(d.blocks);
        if (!a || !b) return null;            // a feature with no target has no mark to join
        const bad = violatedNow(d, features, pending);
        const blocker = features.get(d.from)!, blocked = features.get(d.blocks)!;
        const text = bad
          ? violationSentence({ targetDate: blocked.targetDate! }, { alias: blocker.alias, targetDate: blocker.targetDate! }, fmtDay)
          : `${blocker.alias} blocks ${blocked.alias}`;
        return (
          <g key={`${d.from}>${d.blocks}`} role="listitem" aria-label={text}
             className={bad ? "tl-dep tl-dep--violated" : "tl-dep"} data-dep={`${d.from}>${d.blocks}`}>
            <path d={`M${a.x},${a.y} C${a.x + 24},${a.y} ${b.x - 24},${b.y} ${b.x},${b.y}`} />
            {bad && <text x={b.x + 8} y={b.y - 6} className="tl-dep-why">{text}</text>}
          </g>
        );
      })}
    </svg>
  );
}
```

```css
/* timeline.css ⚑ (task 128) — appended. No animation, no transition on a connector. */
.tl-dep path { fill: none; stroke: var(--line-strong); stroke-width: 1px; }
.tl-dep--violated path { stroke: var(--signal); stroke-dasharray: 4 3; }   /* shape, not only hue */
.tl-dep-why { font: 12px var(--sans); fill: var(--signal); }
.tl-mark--draggable .tl-mark-body { cursor: ew-resize; touch-action: none; }
.tl-drag-day { font-family: var(--mono); font-variant-numeric: tabular-nums; }
@media (forced-colors: active) {
  .tl-dep--violated path { stroke: CanvasText; }   /* the dash and the sentence carry it */
  .tl-dep-why { fill: CanvasText; }
}
```

`--line-strong` is task 26's token name (**UNVERIFIED**, no part read for it); the dash is there so
the violated connector differs by shape as well as by the sentence.

#### Interfaces — complete: the delta on ⚑ `desktop/src/components/palette/commands.ts` (task 59)

```ts
// Appended to buildCommands(state) (task 59's generator). Both need a focused feature and a
// writable project; a granted reader gets neither (task 126's write gate, decision 7).
  { id: "feature.blocks", label: "Blocks…", scope: "feature", writes: true,
    run: (ctx) => ctx.pickFeature({ exclude: [ctx.featureId], sameTeam: true },
      (blocks) => mutate({ kind: "featureDep.add", featureId: ctx.featureId, blocks })) },
  { id: "feature.unblock", label: "Remove dependency", scope: "feature", writes: true,
    when: (ctx) => ctx.replica.depsOf(ctx.featureId).some((d) => d.from === ctx.featureId),
    run: (ctx) => ctx.pickFrom(ctx.replica.depsOf(ctx.featureId).filter((d) => d.from === ctx.featureId),
      (d) => mutate({ kind: "featureDep.remove", featureId: ctx.featureId, blocks: d.blocks })) },
```

`Command`'s `scope`, `writes`, `when`, `pickFeature` and `pickFrom` are task 59's and task 126's
names as their scopes imply them (**UNVERIFIED**). *Blocks…* offers features of the same team only
(task 120 refuses a cross-team dependency, `task-120.md:42`, `:363`).

The loop's words, appended to `orderViolated.ts`:

```ts
/** The path blocks → … → from through deps the replica holds, or null. Words a dependency_cycle
 *  refusal only (decision 6); the Hub is the guard. Breadth first, so the shortest loop is named. */
export function findLoop(deps: readonly { from: string; blocks: string }[], from: string, blocks: string): string[] | null {
  const next = new Map<string, string[]>();
  for (const d of deps) next.set(d.from, [...(next.get(d.from) ?? []), d.blocks]);
  const prev = new Map<string, string>([[blocks, ""]]);
  const queue = [blocks];
  while (queue.length) {
    const at = queue.shift()!;
    if (at === from) {
      const path = [from];
      for (let p = prev.get(from)!; p !== ""; p = prev.get(p)!) path.unshift(p);
      return [from, ...path];                  // from → blocks → … → from
    }
    for (const n of next.get(at) ?? []) if (!prev.has(n)) { prev.set(n, at); queue.push(n); }
  }
  return null;
}
```

#### Tests to write first — `desktop/src/components/roadmap/useTimelineDrag.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { addDays, dayAt } from "./useTimelineDrag";
import { findLoop, orderViolated } from "./orderViolated";

/** A 20 px-per-day scale from 1 Oct, the shape task 128's timelineScale returns for day zoom. */
const days = (a: string, b: string) => (Date.UTC(+b.slice(0, 4), +b.slice(5, 7) - 1, +b.slice(8)) -
  Date.UTC(+a.slice(0, 4), +a.slice(5, 7) - 1, +a.slice(8))) / 86_400_000;
const scale = { x: (d: string) => days("2026-10-01", d) * 20, ticks: [] };

describe("dates are whole calendar days", () => {
  // mutant killed: "fractional dates" (a drop writes an instant or a part-day)
  it("a drop 2.4 days in lands on the nearest whole day, as YYYY-MM-DD", () => {
    const d = dayAt(scale as never, "2026-10-01", 48);
    expect(d).toBe("2026-10-03");
    expect(d).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
  // mutant killed: "addDays through local time" (Sydney's 4 Oct 2026 has 23 hours)
  it("adding a day across Sydney's daylight-saving change is one calendar day", () => {
    expect(addDays("2026-10-03", 1)).toBe("2026-10-04");
    expect(addDays("2026-10-04", 1)).toBe("2026-10-05");
  });
});

describe("the optimistic rule is the Hub's rule", () => {
  // The cases of task 120's TestOrderViolatedIsDerived (task-120.md:362) and task 115's depOf
  // (task-115.md:657-661). Gap: task 115 has no order row to read by path (below).
  const cases: [blocker: string | null, blocked: string | null, want: boolean][] = [
    ["2026-11-01", "2026-10-15", true],   // FT-1 blocks FT-2, FT-2 due first
    ["2026-11-01", "2026-11-20", false],  // FT-2 moved after its blocker
    ["2026-11-01", "2026-11-01", false],  // the same day is not before
    [null, "2026-10-15", false],          // either date missing is not a violation
    ["2026-11-01", null, false],
  ];
  // mutant killed: "two rules drift" (<= for <, or null read as a date)
  it.each(cases)("blocker %s, blocked %s → %s", (a, b, want) => expect(orderViolated(a, b)).toBe(want));
});

describe("a refused loop is named from the replica", () => {
  it("EP-2 blocking EP-5, with EP-5 → EP-2 held, is EP-2 → EP-5 → EP-2", () => {
    expect(findLoop([{ from: "ft-5", blocks: "ft-2" }], "ft-2", "ft-5")).toEqual(["ft-2", "ft-5", "ft-2"]);
  });
  it("no held path is null, so the line names no feature the caller cannot see", () => {
    expect(findLoop([{ from: "ft-7", blocks: "ft-2" }], "ft-2", "ft-5")).toBeNull();
  });
});
```

#### Tests to write first — `desktop/src/components/roadmap/TimelineDrag.test.tsx`

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { useTimelineDrag, type DragSubject } from "./useTimelineDrag";
import { TimelineDrag, dropOp } from "./TimelineDrag";
import { inverseOf } from "../../work/replica/ops";

const mutate = vi.fn();
let access: "member" | "granted" = "member";
vi.mock("../../work/replica/useWork", () => ({ mutate }));
vi.mock("../../work/access/projectAccess", () => ({ useProjectAccess: () => access }));

const x = (d: string) => (Date.UTC(+d.slice(0, 4), +d.slice(5, 7) - 1, +d.slice(8)) - Date.UTC(2026, 9, 1)) / 86_400_000 * 20;
const scale = { x, ticks: [] } as never;
const fmt = (d: string) => `${+d.slice(8)} Oct`;
const EP2: DragSubject = { kind: "feature", featureId: "ft-2", start: "2026-10-10" };

function Harness({ onDrop }: { onDrop: (s: DragSubject, day: string) => void }) {
  const drag = useTimelineDrag(scale, "2026-10-01", { onDrop });
  return <TimelineDrag projectId="pj-1" subject={EP2} drag={drag} fmtDay={fmt}><i>◆</i></TimelineDrag>;
}
function mount() {
  const writes: WorkOpLike[] = [];
  const onDrop = (s: DragSubject, day: string) => writes.push(dropOp(s, day) as WorkOpLike);
  const view = render(<Harness onDrop={onDrop} />);
  return { writes, view, handle: () => screen.queryByRole("slider") };
}
type WorkOpLike = { kind: string; req: Record<string, string>; prev: Record<string, string> };
const drag = (el: Element, toPx: number, end: "up" | "escape" = "up") => {
  fireEvent.pointerDown(el, { pointerId: 1, clientX: x("2026-10-10") });
  for (let px = x("2026-10-10"); px <= toPx; px += 7) fireEvent.pointerMove(el, { pointerId: 1, clientX: px });
  if (end === "escape") fireEvent.keyDown(el, { key: "Escape" });
  fireEvent.pointerUp(el, { pointerId: 1, clientX: toPx });
};

beforeEach(() => { mutate.mockReset(); access = "member"; });

// mutant killed: "a write per pointer move"
it("a drop sends exactly one patch, with the new date", () => {
  const { writes, handle } = mount();
  drag(handle()!, x("2026-10-14"));
  expect(writes).toEqual([{ kind: "feature.patch", featureId: "ft-2",
    req: { targetDate: "2026-10-14" }, prev: { targetDate: "2026-10-10" } }]);
});

// mutant killed: "cancel still writes"
it("Escape mid-drag sends nothing, and the mark returns", () => {
  const { writes, handle } = mount();
  drag(handle()!, x("2026-10-14"), "escape");
  expect(writes).toEqual([]);
  expect((handle() as HTMLElement).style.transform).toBe("translateX(0px)");
});

// mutant killed: "undo re-sends the new date": the inverse is built from prev
it("Undo writes one patch back to the old date", () => {
  // task 56's inverseOf reads a roadmap op's prev as its before (task-125.md:529-531)
  const inv = inverseOf(dropOp(EP2, "2026-10-14"), new Map(), {} as never) as unknown as WorkOpLike;
  expect(inv).toMatchObject({ kind: "feature.patch", req: { targetDate: "2026-10-10" }, prev: { targetDate: "2026-10-14" } });
  const ms = { kind: "milestone", milestoneId: "ml-1", start: "2026-10-20" } as const;
  const msInv = inverseOf(dropOp(ms, "2026-10-22"), new Map(), {} as never) as unknown as WorkOpLike;
  expect(msInv).toMatchObject({ kind: "milestone.patch", req: { date: "2026-10-20" } });
});

// mutant killed: "drag around the grant"
it("a granted reader's mark has no handle", () => {
  access = "granted";
  const { handle, view } = mount();
  expect(handle()).toBeNull();
  expect(view.container.querySelector(".tl-mark--draggable")).toBeNull();
});

it("the date under the hand is mono and states no forecast", () => {
  const { handle, view } = mount();
  fireEvent.pointerDown(handle()!, { pointerId: 1, clientX: x("2026-10-10") });
  fireEvent.pointerMove(handle()!, { pointerId: 1, clientX: x("2026-10-12") });
  expect(view.container.querySelector(".tl-drag-day")!.textContent).toBe("12 Oct");
  expectNoPrediction(view.container);
});
```

The Undo test pins the inverse this task's op produces. Sending it once, and the chip's lapse, are
task 56's tests (`task-56.md:728-760`), not repeated here.

#### Tests to write first — `desktop/src/components/roadmap/DependencyLines.test.tsx`

```tsx
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { DependencyLines } from "./DependencyLines";

const f = (id: string, alias: string, targetDate: string | null) => [id, { id, alias, targetDate }] as const;
const features = new Map([f("ft-2", "EP-2", "2026-10-10"), f("ft-5", "EP-5", "2026-10-03")]) as never;
const anchor = (id: string) => (id === "ft-2" ? { x: 180, y: 20 } : { x: 40, y: 60 });
const fmt = (d: string) => `${+d.slice(8)} Oct`;
const dep = (orderViolated: boolean) => [{ from: "ft-2", blocks: "ft-5", orderViolated }];

// mutant killed: "signal by colour alone" (forced-colors strips --signal; the words remain)
it("a violated order reads by its sentence, visible and as the accessible name", () => {
  const { container } = render(<DependencyLines deps={dep(true)} features={features} anchor={anchor}
    pending={new Set()} fmtDay={fmt} />);
  const g = screen.getByRole("listitem");
  expect(g.getAttribute("aria-label")).toBe("targets 3 Oct, before its blocker EP-2's 10 Oct");
  expect(container.querySelector(".tl-dep-why")!.textContent).toBe(g.getAttribute("aria-label"));
  expect(getComputedStyle(container.querySelector(".tl-dep--violated path")!).animationName ?? "none")
    .toMatch(/^(none|)$/);                                  // never animated
  expectNoPrediction(container);
});

// mutant killed: "the preview overrides the Hub" once nothing is in flight
it("with no write in flight, the Hub's false wins over the copied rule", () => {
  render(<DependencyLines deps={dep(false)} features={features} anchor={anchor} pending={new Set()} fmtDay={fmt} />);
  expect(screen.getByRole("listitem").getAttribute("aria-label")).toBe("EP-2 blocks EP-5");
});

it("while a drop is in flight, the copied rule previews the Hub's answer", () => {
  render(<DependencyLines deps={dep(false)} features={features} anchor={anchor} pending={new Set(["ft-5"])} fmtDay={fmt} />);
  expect(screen.getByRole("listitem").getAttribute("aria-label")).toMatch(/^targets 3 Oct/);
});
```

A fourth test sits in `TimelineDrag.test.tsx` beside the others, over task 56's real replica
harness (`task-56.md:526-540`, extended by task 125's `RoadmapRecords`): *Blocks…* on EP-2 → EP-5
with `ft-5 → ft-2` held paints a connector, the gate rejects with
`ApiError("dependency_cycle", …, 422)` and `details {from: "ft-2", blocks: "ft-5"}`, and then no
`[data-dep="ft-2>ft-5"]` remains and the fact line reads *"EP-2 → EP-5 → EP-2"* (mutant killed: "a
phantom dependency").

#### Steps

1. Write `useTimelineDrag.test.ts`; run it; it fails on the missing module.
2. Write `orderViolated.ts` (the rule, the sentence, `findLoop`) and `useTimelineDrag.ts`; the pure
   tests pass.
3. Write `TimelineDrag.test.tsx` and `DependencyLines.test.tsx`; they fail.
4. Write `TimelineDrag.tsx` and `DependencyLines.tsx`; mount them in ⚑ `TimelineRow.tsx` (each
   target and milestone mark) and ⚑ `Timeline.tsx` (one `useTimelineDrag`, one `DependencyLines`
   over the rows, `useDropWriter` as `onDrop`); append the CSS.
5. Add the two commands to ⚑ `commands.ts`; task 59's hint test must still pass (neither has a
   chord, so neither has a hint).
6. Run the acceptance; commit with the four new test files.

#### Acceptance

```bash
cd desktop && npx vitest run src/components/roadmap/useTimelineDrag.test.ts \
  src/components/roadmap/TimelineDrag.test.tsx src/components/roadmap/DependencyLines.test.tsx
# expect: 3 files passed; 18 tests passed (9 pure incl. 5 table rows, 6 drag, 3 lines); 0 failed
cd desktop && npx vitest run src/components/palette && npx tsc --noEmit
# expect: the palette tests pass as before; tsc exit 0
cd desktop && grep -n "transition" src/components/roadmap/timeline.css | grep -i -E "dep|drag"
# expect: no output (nothing on a connector or a drag animates)
```

#### What this task does NOT claim

- That any date a person sets is right, or achievable. It records who set it; nothing here judges
  it (`OD-27`).
- Automatic rescheduling of dependants. There is none: moving EP-2 moves no other feature, and a
  violated order is shown, not fixed.
- A drag on touch screens beyond pointer events, or smooth dragging at any row count (unmeasured).
- The timeline's layout, zoom, cycle band or grouping (task 128).
- That the feature patch op is task 83's name, or that task 128's `x` takes a day string: both
  are **UNVERIFIED** until those parts land.

#### Gaps found while writing this part

1. **No shared order fixture.** The scope says the two rules agree *"on task 115's conformance
   fixture"*; task 115's `WorkRowsW4` has no order row (`task-115.md:768-783`, W4-01 to W4-14).
   This part copies task 120's `TestOrderViolatedIsDerived` cases and task 115's `depOf`. Proposed:
   task 115 adds `W4-15 order_violated_derived`, and task 114 a golden
   `testdata/work-roadmap/order-violated-cases.json` that this test then reads by path.
2. **The cycle refusal names no path.** Task 120's `dependency_cycle` carries only `details.from`
   and `details.blocks` (`task-120.md:304`); the loop is found client-side (decision 6). A
   `details.path` from task 120 would let the line name a loop through a feature the caller cannot
   see without the desktop guessing.
3. **The dependency event kind has three names.** Task 125's `feature_dep.added`
   (`task-125.md:435`), task 115's `hubapi.EventWorkDependencyAdded` (`task-115.md:651`) and task
   120's `feature.dependency_added` (`task-120.md:304`). Task 125's copy test settles it; the
   skeleton should pick one.
4. **No part for tasks 128, 83, 59, 55 or 89.** `timelineScale`, `TimelineRow`, the feature patch
   op, `buildCommands`'s row shape and the `D` picker are scope names here; `mutate` as a module
   export of `useWork.ts` follows task 127's import (`task-127.md:161`), itself unverified
   against task 55, which has no part.
