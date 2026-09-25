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
   rolls it back through `onRefused`, and the fact line names the loop from the refusal's
   `details.path` (*"EP-2 → EP-5 → EP-2"*); when `details` has no path the line names only the pair
   (a gap for task 114, below).
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
