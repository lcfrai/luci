### Task 128: the timeline, part 1: layout, day-to-quarter zoom, the cycle band, grouping

W4, stream WD; no seam. Depends on task 125 (the W4 desktop types: `Milestone`, `MilestoneView`,
`task-125.md:122-130`, and the replica's `milestonesOf`, as task 130 reads it at
`task-130.md:364`), and through the replica on task 83's W2 types (`Feature`, `Cycle`). Estimate
0.375 (3 h). Closes **L-89**, **L-91**, **L-92** and **L-101**'s W4 half (05 §1 F,
`05-design.md:218-221`, and G, `:235`).

After this task a person opens *Timeline* in Plan and sees the project's features as rows across
days, weeks, months or quarters; each row runs from the day its first issue started to the target
a person set, with its milestones marked; the team's cycles run along the top with the current one
marked and cooldowns left empty; and rows group by initiative, lead or label. Nothing on it is a
forecast: every mark is a date that happened or a date a person set.

**Decisions made here, so nobody re-derives them:**

1. **The scale is over calendar days, and a day is a day.** `timelineScale` takes `YYYY-MM-DD`
   days, and `x(day)` is `dayIndex(from → day) · dayWidth`, where `dayIndex` is computed on the
   date's own digits (`Date.UTC` of year, month, day, divided by 86 400 000). The time zone never
   enters `x`; it enters only where an instant becomes a day (`dayOf(instant)`, through
   `Intl.DateTimeFormat` with `timeZone: tz`). So 4 October 2026, the day Sydney's clocks go
   forward, is exactly as wide as 3 October, and task 129's `dayAt` inverts `x` by one division
   (`task-129.md:108-113`). The signature is the scope's, `timelineScale(zoom, from, to, widthPx,
   tz)`, because task 129 was written against it (`task-129.md:69-71`).
2. **Zoom is the window's length, not a magnification.** `day` shows 14 days, `week` 12 weeks,
   `month` 12 months, `quarter` 8 quarters, each starting on its unit's boundary at or before the
   focus day (Monday for weeks; the 1st for months; 1 January, April, July or October for
   quarters). The scale maps the window to the pane's width. Ticks fall on the same boundaries, as
   days, so a quarter tick is `2026-10-01` in every zone.
3. **A span is two facts, and never a third.** A row's start is the local day of the **earliest**
   `issue.state` event with `to: "started"` among the feature's issues (the event's `at` through
   `dayOf`); its end is `Feature.targetDate`, a date a person set. With no started issue, no span
   element is drawn, only the target mark; with no target, only a start mark, labelled *"started
   3 Oct · no target set"*. A span needs both ends, so neither is ever invented, and today's date
   is never used as an end. A span has one fill whatever its issues'
   states: no share of it is ever coloured done (02 C13, `OD-27`, AX §10.2).
4. **The target names who set it.** The label reads *"target 14 Oct · set by Priya"*: the actor of
   the latest `feature.updated` event whose data names `targetDate` (task 72's event data,
   **UNVERIFIED**, gap 2). When no such event is in the replica, it reads *"target 14 Oct"*, never
   a guessed name.
5. **Rows keep their order while you look.** Inside a group, rows are ordered by feature name, a
   stable key, not by date: a timeline read top to bottom in time would reorder under task 129's
   drag. A teammate's change that moves a row to another group (a new lead, initiative or label)
   goes through task 41's `reconcile` unchanged (`task-41.md:177-215`): the row stays where it
   was, marked, until `N` (`applyHeld`). The scope's test *"a teammate's target change keeps the
   row in place"* holds trivially under this order and is kept; the group-change row is added.
6. **Grouping is one of three, with the empty group last.** `initiative` (from
   `Feature.initiativeId`), `lead`, `label`. *No initiative*, *No lead* or *No label* is the last
   group, and a feature with two labels appears under each (its hold key is `featureId|label`,
   so task 41's one-home rule stays true).
7. **The cycle band is the Hub's cycles, drawn as they are.** Each `Cycle` is one segment from
   `startsOn` to the end of `endsOn`; the one whose `state` is `"current"` (the Hub's word, not the
   client's clock) carries the marker. A cooldown is the gap between two segments: no element.
8. **A saved view cannot keep the layout yet.** Task 72's `View.layout` is `"list" | "board"`
   (scope, `tasks-61-80.md`, task 72); the display menu switches to the timeline for the session
   (task 44's per-view preference, `prefsSlice.workDisplay`), and gap 1 asks task 72 for
   `"timeline"`.

**Files:**

- `desktop/src/components/roadmap/timelineScale.ts`, `timelineScale.test.ts` (new) — the scale,
  the windows, `dayOf`, `fmtDay`, `addDays`
- `desktop/src/components/roadmap/timelineRows.ts`, `timelineRows.test.ts` (new) — the row model
  and grouping, pure (the scope names no file for these; a gap in the file map, gap 5)
- `desktop/src/components/roadmap/Timeline.tsx`, `Timeline.test.tsx` (new) — the pane
- `desktop/src/components/roadmap/TimelineAxis.tsx`, `TimelineRow.tsx`, `CycleBand.tsx` (new)
- `desktop/src/components/roadmap/timeline.css` (new)
- `desktop/src/components/work/PlanSurface.tsx` ⚑ (task 45) — a *Sections · Timeline* switch in
  its header; `PLAN_SECTIONS` is unchanged (modified)
- `desktop/src/components/work/DisplayMenu.tsx` ⚑ (task 44) — the `timeline` view id, its three
  groupings and its zoom (modified)
- `desktop/src/work/keys/keymap.ts` ⚑ (task 53) — two zoom rows (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 41, 44, 45, 125, 126,
129 and 130. There is **no part for tasks 53, 54, 72, 77 or 83**, so `Feature`, `Cycle`,
`KeyBinding`, `registerVerb` and `View.layout` are their scopes' shapes (`tasks-61-80.md`, task
72's paragraph; `tasks-81-100.md:90-104`) and **UNVERIFIED**. `desktop/src/components/roadmap/`
does not exist; task 129 already plans three files in it. `prototype/work-layer.html` has no
timeline (one match for the word, a comment on the heartbeat at `:535`), so the look below is
`DESIGN-SYSTEM.md`'s tokens, not a mock.

#### Interfaces — complete: `desktop/src/components/roadmap/timelineScale.ts`

```ts
export type Zoom = "day" | "week" | "month" | "quarter";
export const ZOOMS: readonly Zoom[] = ["day", "week", "month", "quarter"];

export interface Tick { day: string; label: string; major: boolean }
export interface TimelineScale {
  /** Left edge of a YYYY-MM-DD day, in px from the window's first day. */
  x(day: string): number;
  dayWidth: number;
  ticks: readonly Tick[];
  /** The local day of an RFC 3339 instant in tz. The only place tz is used (decision 1). */
  dayOf(instant: string): string;
  /** "14 Oct", the axis's and task 129's short format. */
  fmtDay(day: string): string;
}

const MS_DAY = 86_400_000;
const MON = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const utc = (d: string): number => Date.UTC(+d.slice(0, 4), +d.slice(5, 7) - 1, +d.slice(8, 10));
const iso = (ms: number): string => new Date(ms).toISOString().slice(0, 10);

export const addDays = (d: string, n: number): string => iso(utc(d) + n * MS_DAY);
export const dayIndex = (from: string, d: string): number => Math.round((utc(d) - utc(from)) / MS_DAY);
export const fmtDay = (d: string): string => `${+d.slice(8, 10)} ${MON[+d.slice(5, 7) - 1]}`;

/** The first day of zoom's unit at or before d. Weeks start on Monday. */
export function unitStart(zoom: Zoom, d: string): string {
  const y = d.slice(0, 4), m = +d.slice(5, 7);
  if (zoom === "day") return d;
  if (zoom === "week") return addDays(d, -((new Date(utc(d)).getUTCDay() + 6) % 7));
  if (zoom === "month") return `${y}-${String(m).padStart(2, "0")}-01`;
  return `${y}-${String(m - ((m - 1) % 3)).padStart(2, "0")}-01`;
}
function nextUnit(zoom: Zoom, d: string): string {
  if (zoom === "day") return addDays(d, 1);
  if (zoom === "week") return addDays(d, 7);
  const step = zoom === "month" ? 1 : 3;
  const y = +d.slice(0, 4), m = +d.slice(5, 7) - 1 + step;
  return `${y + Math.floor(m / 12)}-${String((m % 12) + 1).padStart(2, "0")}-01`;
}
const LENGTH: Record<Zoom, number> = { day: 14, week: 12, month: 12, quarter: 8 };

/** Decision 2: the window a zoom shows around a focus day. `to` is the window's last day. */
export function windowFor(zoom: Zoom, focus: string): { from: string; to: string } {
  let end = unitStart(zoom, focus);
  const from = end;
  for (let i = 0; i < LENGTH[zoom]; i++) end = nextUnit(zoom, end);
  return { from, to: addDays(end, -1) };
}

function tickLabel(zoom: Zoom, d: string): string {
  if (zoom === "quarter") return `Q${(+d.slice(5, 7) + 2) / 3} ${d.slice(0, 4)}`;
  if (zoom === "month") return `${MON[+d.slice(5, 7) - 1]} ${d.slice(0, 4)}`;
  return fmtDay(d);
}
```

```ts
// timelineScale.ts, continued: the scale itself.
export function timelineScale(zoom: Zoom, from: string, to: string, widthPx: number, tz: string): TimelineScale {
  const dayWidth = widthPx / (dayIndex(from, to) + 1);
  const ticks: Tick[] = [];
  for (let d = unitStart(zoom, from); d <= to; d = nextUnit(zoom, d)) {
    if (d < from) continue;
    const major = zoom === "day" ? unitStart("week", d) === d : unitStart("quarter", d) === d;
    ticks.push({ day: d, label: tickLabel(zoom, d), major });
  }
  const local = new Intl.DateTimeFormat("en-CA", { timeZone: tz, year: "numeric", month: "2-digit", day: "2-digit" });
  return {
    x: (day) => dayIndex(from, day) * dayWidth,
    dayWidth,
    ticks,
    dayOf: (instant) => local.format(new Date(instant)),     // en-CA formats as YYYY-MM-DD
    fmtDay,
  };
}
```

`en-CA` is used only for its `YYYY-MM-DD` shape; no label is formatted with it. A 23-hour day is
one `dayWidth` wide because `x` never sees an instant (decision 1).

#### Interfaces — complete: `desktop/src/components/roadmap/timelineRows.ts`

```ts
import type { Feature, FeedEvent, MilestoneView } from "../../api/work";   // tasks 83, 125, 13
import type { GroupedIssues, RowFact } from "../../work/hold";                          // task 41

export type TimelineGrouping = "initiative" | "lead" | "label";
export const NO_GROUP: Record<TimelineGrouping, string> = {
  initiative: "No initiative", lead: "No lead", label: "No label",
};

export interface TimelineRowModel {
  key: string;                   // featureId, or featureId|label under label grouping (decision 6)
  feature: Feature;
  alias: string;                 // the host's display alias, as task 129 takes it
  start: string | null;          // decision 3: first started day, or null
  target: string | null;         // Feature.targetDate
  targetSetBy: string | null;    // decision 4: a display name, or null
  milestones: readonly MilestoneView[];
}
export interface TimelineGroup { key: string; label: string; rows: readonly TimelineRowModel[] }

export interface RowInputs {
  features: readonly (Feature & { alias: string })[];
  issuesOf(featureId: string): readonly string[];            // is-… ids with head.featureId set
  events: readonly FeedEvent[];                               // the replica's feed, oldest first
  milestonesOf(featureId: string): readonly MilestoneView[];
  initiativeName(id: string): string | null;
  labelName(id: string): string | null;
  nameOf(userId: string): string | null;                      // a display name from the replica
  dayOf(instant: string): string;                             // the scale's
}

/** Decision 3: the local day of the earliest started move among the feature's issues. */
export function firstStarted(issueIds: readonly string[], events: readonly FeedEvent[],
                             dayOf: (i: string) => string): string | null {
  const ids = new Set(issueIds);
  let first: string | null = null;
  for (const e of events) {
    if (e.kind !== "issue.state" || !ids.has(e.subject)) continue;
    if ((e.data as { to?: string }).to !== "started") continue;
    if (first === null || e.at < first) first = e.at;
  }
  return first === null ? null : dayOf(first);
}

/** Decision 4: who set the target, from the latest feature.updated naming targetDate. */
export function targetSetter(featureId: string, events: readonly FeedEvent[],
                             nameOf: (userId: string) => string | null): string | null {
  for (let i = events.length - 1; i >= 0; i--) {
    const e = events[i];
    if (e.kind !== "feature.updated" || e.subject !== featureId || e.actor === null) continue;
    if (!((e.data as { fields?: string[] }).fields ?? []).includes("targetDate")) continue;
    return nameOf(e.actor);
  }
  return null;
}
```

`FeedEvent`'s `{seq, kind, subject, at, actor, data}` is the shape task 133 builds on
(`task-133.md:342-343`); `feature.updated`'s `data.fields` is task 72's (**UNVERIFIED**, gap 2).

```ts
// timelineRows.ts, continued: rows, groups, and the hand-off to task 41's hold model.
export function rowOf(f: Feature & { alias: string }, inp: RowInputs, key = f.id): TimelineRowModel {
  return {
    key, feature: f, alias: f.alias,
    start: firstStarted(inp.issuesOf(f.id), inp.events, inp.dayOf),
    target: f.targetDate ?? null,
    targetSetBy: targetSetter(f.id, inp.events, inp.nameOf),
    milestones: inp.milestonesOf(f.id),
  };
}

/** Decisions 5 and 6: groups by name, the empty group last; rows by feature name. */
export function buildGroups(by: TimelineGrouping, inp: RowInputs): TimelineGroup[] {
  const groups = new Map<string, { label: string; rows: TimelineRowModel[] }>();
  const put = (gkey: string, label: string, row: TimelineRowModel) => {
    const g = groups.get(gkey) ?? { label, rows: [] };
    g.rows.push(row);
    groups.set(gkey, g);
  };
  for (const f of inp.features) {
    if (by === "initiative") {
      const name = f.initiativeId ? inp.initiativeName(f.initiativeId) : null;
      put(name ? f.initiativeId! : "", name ?? NO_GROUP.initiative, rowOf(f, inp));
    } else if (by === "lead") {
      const lead = (f.lead as User | null) ?? null;                 // gap 3: task 72 types it required
      put(lead ? lead.id : "", lead ? lead.displayName : NO_GROUP.lead, rowOf(f, inp));
    } else {
      const named = f.labels.map((id) => [id, inp.labelName(id)] as const).filter(([, n]) => n !== null);
      if (named.length === 0) put("", NO_GROUP.label, rowOf(f, inp));
      for (const [id, n] of named) put(id, n!, rowOf(f, inp, `${f.id}|${id}`));
    }
  }
  const byName = (a: { label: string }, b: { label: string }) => a.label.localeCompare(b.label);
  return [...groups.entries()]
    .map(([key, g]) => ({ key, label: g.label, rows: g.rows.sort((a, b) => a.feature.name.localeCompare(b.feature.name)) }))
    .sort((a, b) => (a.key === "" ? 1 : b.key === "" ? -1 : byName(a, b)));
}

/** Decision 5: task 41's model, unchanged. A row's `state` slot carries a constant, so only a
 *  group change counts as a move; task 41 compares nothing else it would misread. */
export function holdInput(groups: readonly TimelineGroup[]): GroupedIssues {
  return {
    groups: groups.map((g) => ({
      key: g.key,
      rows: g.rows.map((r): RowFact => ({ issueId: r.key, alias: r.alias, state: "backlog", cause: "" })),
    })),
    left: new Map(),
  };
}
```

`User` joins the file's type import. The `state: "backlog"` constant is the one place this part
bends task 41's issue-shaped `RowFact`; gap 4 asks task 41 for a neutral `RowFact.version`
instead, so a feature row's own change could flag itself.

#### Interfaces — complete: `desktop/src/components/roadmap/Timeline.tsx`

```tsx
import { useMemo, useState } from "react";
import type { Cycle } from "../../api/work";
import { applyHeld, initialHold, reconcile, type HoldState } from "../../work/hold";
import { HeldLine } from "../work/HeldLine";                                   // task 41
import { CycleBand } from "./CycleBand";
import { TimelineAxis } from "./TimelineAxis";
import { TimelineRow } from "./TimelineRow";
import { buildGroups, holdInput, type RowInputs, type TimelineGrouping } from "./timelineRows";
import { timelineScale, windowFor, type Zoom } from "./timelineScale";
import "./timeline.css";

export interface TimelineProps {
  inputs: Omit<RowInputs, "dayOf">;
  cycles: readonly Cycle[];            // the team's, from the replica (task 83)
  grouping: TimelineGrouping;
  zoom: Zoom;
  focus: string;                        // YYYY-MM-DD; the host sets it to today's local day once
  widthPx: number;                      // the pane's measured width (a ResizeObserver in the host)
  timeZone: string;
}
```

```tsx
// Timeline.tsx, continued.
export function Timeline(p: TimelineProps) {
  const { from, to } = windowFor(p.zoom, p.focus);
  const scale = useMemo(() => timelineScale(p.zoom, from, to, p.widthPx, p.timeZone),
    [p.zoom, from, to, p.widthPx, p.timeZone]);
  const groups = buildGroups(p.grouping, { ...p.inputs, dayOf: scale.dayOf });
  const byKey = new Map(groups.flatMap((g) => g.rows.map((r) => [r.key, r] as const)));
  const next = holdInput(groups);
  const [attention, setAttention] = useState<string | null>(null);
  const [hold, setHold] = useState<HoldState>(() => initialHold(next));
  const [seen, setSeen] = useState(next);
  if (seen !== next && JSON.stringify(seen.groups) !== JSON.stringify(next.groups)) {
    setSeen(next);
    setHold(reconcile(hold, next, { groupKey: attention }));           // task 41, unchanged
  }
  const label = (key: string) => groups.find((g) => g.key === key)?.label ?? key;
  const heldCount = hold.held.length;

  return (
    <div className="tl" role="grid" aria-label="Timeline" tabIndex={0}
      onKeyDown={(e) => {                                               // task 41 decision 4: own element
        if (e.code === "KeyN" && !e.ctrlKey && !e.metaKey && !e.altKey) setHold(applyHeld(hold, next));
      }}>
      <TimelineAxis scale={scale} widthPx={p.widthPx} />
      <CycleBand cycles={p.cycles} scale={scale} from={from} to={to} />
      <HeldLine count={heldCount} onApply={() => setHold(applyHeld(hold, next))} />
      {hold.shown.map((g) => (
        <section key={g.key || "none"} className="tl-group" role="rowgroup"
          onPointerEnter={() => setAttention(g.key)} onFocus={() => setAttention(g.key)}>
          <h3 className="tl-group-head">{label(g.key)}</h3>
          {g.rows.map((s) => {
            if (s.kind === "ghost") return <div key={s.issueId} className="tl-ghost">{s.alias} · {s.why}</div>;
            const row = byKey.get(s.row.issueId);
            return row ? <TimelineRow key={row.key} row={row} scale={scale} from={from} to={to}
              held={s.home !== g.key} /> : null;
          })}
        </section>
      ))}
    </div>
  );
}
```

The pane never scrolls itself: a change while the person reads adds to `HeldLine`'s count and
moves nothing (AX §10.7). `JSON.stringify` compares the hold input only, a few hundred bytes for
a project's features; the render cost at large row counts is not measured (not claimed).

#### Interfaces — complete: `TimelineRow.tsx`, `CycleBand.tsx`, `TimelineAxis.tsx`

```tsx
// desktop/src/components/roadmap/TimelineRow.tsx
import type { TimelineRowModel } from "./timelineRows";
import { addDays, type TimelineScale } from "./timelineScale";

export function TimelineRow(p: { row: TimelineRowModel; scale: TimelineScale; from: string; to: string; held: boolean }) {
  const { row, scale } = p;
  const inWindow = (d: string) => d >= p.from && d <= p.to;
  const targetWords = row.target
    ? `target ${scale.fmtDay(row.target)}${row.targetSetBy ? ` · set by ${row.targetSetBy}` : ""}`
    : row.start ? `started ${scale.fmtDay(row.start)} · no target set` : "no target set";
  return (
    <div className="tl-row" role="row" data-held={p.held || undefined} data-feature={row.feature.id}>
      <div className="tl-name" role="rowheader">
        <span className="tl-title">{row.feature.name}</span>
        <span className="tl-branch mono">{row.feature.branchRef}</span>
      </div>
      <div className="tl-lane" role="gridcell">
        {row.start && row.target && (                                   // decision 3: both facts, or none
          <div className="tl-span" style={{ transform: `translateX(${scale.x(row.start)}px)`,
            width: Math.max(0, scale.x(addDays(row.target, 1)) - scale.x(row.start)) }} />
        )}
        {row.start && !row.target && inWindow(row.start) && (
          <span className="tl-start" style={{ transform: `translateX(${scale.x(row.start)}px)` }} />
        )}
        {row.target && inWindow(row.target) && (
          <span className="tl-target" style={{ transform: `translateX(${scale.x(row.target)}px)` }} />
        )}
        {row.milestones.filter((m) => m.milestone.date && inWindow(m.milestone.date)).map((m) => (
          <span key={m.milestone.id} className="tl-milestone" data-reached={m.reached}
            style={{ transform: `translateX(${scale.x(m.milestone.date!)}px)` }}
            aria-label={`${m.milestone.name}, ${scale.fmtDay(m.milestone.date!)}, ${m.reached ? "reached" : "not reached"}`}>
            <span className="tl-milestone-name">{m.milestone.name}</span>
          </span>
        ))}
        <span className="tl-words mono tabular-nums">{targetWords}</span>
      </div>
    </div>
  );
}
```

```tsx
// desktop/src/components/roadmap/CycleBand.tsx — decision 7. Cycle is task 72's shape via 83:
// { id, teamId, number, name: string | null, startsOn, endsOn, state: "upcoming"|"current"|"closed" }.
import type { Cycle } from "../../api/work";
import { addDays, type TimelineScale } from "./timelineScale";

export function CycleBand(p: { cycles: readonly Cycle[]; scale: TimelineScale; from: string; to: string }) {
  const shown = p.cycles.filter((c) => c.endsOn >= p.from && c.startsOn <= p.to);
  return (
    <div className="tl-band" aria-label="Cycles">
      {shown.map((c) => {
        const a = c.startsOn < p.from ? p.from : c.startsOn;
        const b = c.endsOn > p.to ? p.to : c.endsOn;
        return (
          <span key={c.id} className="tl-cycle" data-cycle={c.id} data-current={c.state === "current" || undefined}
            style={{ transform: `translateX(${p.scale.x(a)}px)`, width: p.scale.x(addDays(b, 1)) - p.scale.x(a) }}
            title={`${c.name ?? `Cycle ${c.number}`}, ${p.scale.fmtDay(c.startsOn)} to ${p.scale.fmtDay(c.endsOn)}`}>
            {c.name ?? `Cycle ${c.number}`}{c.state === "current" ? " · current" : ""}
          </span>
        );
      })}
    </div>
  );
}

// desktop/src/components/roadmap/TimelineAxis.tsx
import type { TimelineScale } from "./timelineScale";

export function TimelineAxis(p: { scale: TimelineScale; widthPx: number }) {
  return (
    <div className="tl-axis" style={{ width: p.widthPx }} aria-hidden="true">
      {p.scale.ticks.map((t) => (
        <span key={t.day} className="tl-tick mono" data-major={t.major || undefined}
          style={{ transform: `translateX(${p.scale.x(t.day)}px)` }}>{t.label}</span>
      ))}
    </div>
  );
}
```

The current cycle is marked in words (*"· current"*) as well as by `data-current`'s wash, so it
reads under `forced-colors`. The band states no dates it was not given: a cycle past the last
one the Hub created (task 77's `ahead`) is not drawn.

#### `timeline.css` — complete

```css
/* Positions are transforms; nothing here animates a position or a width (DESIGN-SYSTEM motion). */
.tl { position: relative; font-family: var(--sans); color: var(--ink); overflow-x: auto; }
.tl:focus-visible { outline: 2px solid var(--accent-ring); outline-offset: 2px; }
.tl-axis, .tl-band { position: relative; height: 24px; margin-left: 240px; }
.tl-tick { position: absolute; top: 0; font-size: 11px; color: var(--ink-3); white-space: nowrap; }
.tl-tick[data-major] { color: var(--ink-2); }
.tl-cycle { position: absolute; top: 4px; height: 16px; padding: 0 6px; font-size: 11px;
  border-radius: var(--r-chip); box-shadow: var(--e-hair); color: var(--ink-2); overflow: hidden; }
.tl-cycle[data-current] { background: var(--accent-tint); color: var(--ink); }
.tl-group-head { font-size: 12px; font-weight: 600; color: var(--ink-2); margin: 16px 0 4px; }
.tl-row { display: grid; grid-template-columns: 240px 1fr; min-height: 40px; }
.tl-row[data-held] { box-shadow: inset 2px 0 0 var(--line-strong); }
.tl-name { display: flex; flex-direction: column; justify-content: center; padding-right: 12px; }
.tl-title { font-size: 13px; }
.tl-branch { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
.tl-lane { position: relative; }
.tl-span { position: absolute; left: 0; top: 14px; height: 12px; border-radius: var(--r-chip);
  background: var(--surface); box-shadow: var(--e-btn); }
.tl-target, .tl-start { position: absolute; left: 0; top: 10px; width: 2px; height: 20px; background: var(--ink-2); }
.tl-start { background: var(--ink-3); }
.tl-milestone { position: absolute; left: -5px; top: 15px; width: 10px; height: 10px;
  transform-origin: center; rotate: 45deg; box-shadow: 0 0 0 1px var(--ink-2); }
.tl-milestone[data-reached="true"] { background: var(--ink-2); }
.tl-milestone-name { position: absolute; left: 14px; top: -3px; rotate: -45deg; font-size: 11px; white-space: nowrap; }
.tl-words { position: absolute; right: 8px; top: 12px; font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
.tl-ghost { margin-left: 240px; font-size: 12px; color: var(--ink-3); }
```

`.tl-span` has one background and no child: there is no second fill a progress bar could live in
(decision 3). A reached milestone is filled and an unreached one outlined, so shape, not colour,
tells them apart; its `aria-label` says it in words. The `rotate` property keeps the diamond's
turn separate from the `transform` that places it.

#### The deltas — ⚑ `DisplayMenu.tsx` (task 44), ⚑ `PlanSurface.tsx` (task 45), ⚑ `keymap.ts` (task 53)

```ts
// DisplayMenu.tsx ⚑ (task 44, task-44.md:82-100). The view id and the options widen additively.
export type WorkViewId = "list" | "board" | "triage" | "timeline";
export type DisplayGrouping = "criterion" | "state" | "assignee" | "priority" | TimelineGrouping;
export interface DisplayOptions {
  grouping: DisplayGrouping;
  ordering: DisplayOrdering;
  density: "default" | "compact";
  show: { assignee: boolean; labels: boolean; priority: boolean; agentLine: boolean; updated: boolean };
  zoom?: Zoom;                       // timeline only; absent reads "month"
}
// groupingsFor("timeline") returns ["initiative", "lead", "label"]; displayFor(all, "timeline")
// reads { ...DEFAULT_DISPLAY, grouping: "initiative", zoom: "month" } when nothing is stored.
// The menu shows a Zoom segmented control (Day · Week · Month · Quarter) only for "timeline",
// and no Ordering control there (decision 5 fixes the order).
```

```tsx
// PlanSurface.tsx ⚑ (task 45, task-45.md:100-110). One switch in the ViewHeader's actions slot;
// PLAN_SECTIONS and its order test are unchanged.
const display = useDisplay("timeline");                       // task 44's prefsSlice.workDisplay reader
const [layout, setLayout] = useState<"sections" | "timeline">("sections");   // session only (decision 8)
// in <ViewHeader actions={…}>:
<Segmented value={layout} onChange={setLayout}
  options={[{ value: "sections", label: "Sections" }, { value: "timeline", label: "Timeline" }]} />
// in the body:
{layout === "timeline"
  ? <TimelineMount projectId={props.projectId} grouping={display.grouping as TimelineGrouping}
      zoom={display.zoom ?? "month"} timeZone={props.timeZone} now={props.now} />
  : /* the four sections, as task 45 wrote them */ sections}
```

`TimelineMount` (in `Timeline.tsx`, 30 lines) reads the replica once per change through
`useWork`: the project's features with their aliases, `issuesOf` from issue heads'
`featureId`, the feed's events, `milestonesOf` (task 125), the team's cycles, initiative and label
names, and `nameOf` from the replica's users; it measures the pane with one `ResizeObserver`
and sets `focus` to `dayOf(new Date(now()).toISOString())` **once, at mount**, so the window never
slides while it is open. `useDisplay` and `Segmented` are task 44's and SW.13's names
(**UNVERIFIED**; `task-44.md:193-197` shows the menu reading `prefsSlice.workDisplay[view]`
itself).

```ts
// keymap.ts ⚑ (task 53). Two rows, in task 53's KeyBinding shape as task 102 quotes it
// (task-102.md:244-246), scope "timeline", read verbs (no writes: true, task 126).
{ verb: "timeline.zoomIn",  keys: ["="], code: "Equal", shift: false, scope: "timeline", slice: "W4",
  label: "Zoom the timeline in" },
{ verb: "timeline.zoomOut", keys: ["-"], code: "Minus", shift: false, scope: "timeline", slice: "W4",
  label: "Zoom the timeline out" },
// Timeline.tsx registers both with registerVerb on mount and removes them on unmount:
// zoomIn moves one step left in ZOOMS (quarter → month → week → day), zoomOut one step right;
// both write display.zoom through task 44's preference. ACTIVE_SLICES gains "W4" if no earlier W4
// task added it (task 102 set W1-W3, task-102.md:246).
```

Being rows of `KEYMAP`, both appear in task 54's `?` sheet with no code in the sheet (the sheet
lists `KEYMAP`, per task 54's scope; **UNVERIFIED**, no part). `=` and `-` are chosen because
they carry no Work verb in 05 §4.3 and type no text in the pane (the timeline has no text field).

#### Tests to write first — `timelineScale.test.ts` and `timelineRows.test.ts`

```ts
// desktop/src/components/roadmap/timelineScale.test.ts
import { describe, expect, it } from "vitest";
import { timelineScale, unitStart, windowFor } from "./timelineScale";

const SYD = "Australia/Sydney";

describe("timelineScale", () => {
  // mutant killed: "UTC quarters" (quarter starts computed from instants shift to 30 Sep in UTC)
  it("quarter ticks fall on quarter starts, as local days", () => {
    const { from, to } = windowFor("quarter", "2026-09-26");
    expect(from).toBe("2026-07-01");
    const s = timelineScale("quarter", from, to, 800, SYD);
    expect(s.ticks.slice(0, 3).map((t) => t.day)).toEqual(["2026-07-01", "2026-10-01", "2027-01-01"]);
    expect(s.ticks[1].label).toBe("Q4 2026");
    expect(s.dayOf("2026-09-30T15:00:00Z")).toBe("2026-10-01");   // 01:00 on 1 Oct in Sydney
  });
  // mutant killed: "millisecond-linear scale" (the DST day would be 23/24 as wide)
  it("the day Sydney's clocks go forward is as wide as the day before", () => {
    const s = timelineScale("week", "2026-09-28", "2026-12-20", 1260, SYD);   // 84 days, 15 px each
    const w = (d: string, e: string) => s.x(e) - s.x(d);
    expect(w("2026-10-04", "2026-10-05")).toBe(w("2026-10-03", "2026-10-04"));
    expect(w("2026-10-03", "2026-10-04")).toBe(s.dayWidth);
  });
  // mutant killed: "weeks from Sunday"
  it("a week starts on Monday", () => {
    expect(unitStart("week", "2026-10-04")).toBe("2026-09-28");   // a Sunday → the Monday before
  });
});
```

```ts
// desktop/src/components/roadmap/timelineRows.test.ts
import { describe, expect, it } from "vitest";
import { buildGroups, firstStarted, holdInput, type RowInputs } from "./timelineRows";

const PRIYA = { id: "u-priya", displayName: "Priya", email: "p@example.test" };
const feat = (id: string, name: string, over: object = {}) => ({
  id, alias: id.toUpperCase(), projectId: "pr-1", name, branchRef: `feat/${name}`, briefId: "br-1",
  lead: PRIYA, targetDate: "2026-10-14", state: "active", priority: "none", labels: [],
  initiativeId: null, createdAt: "", createdBy: PRIYA, updatedAt: "", ...over,
}) as never;
const started = (seq: number, subject: string, at: string) =>
  ({ seq, kind: "issue.state", subject, at, actor: "u-priya", data: { from: "todo", to: "started" } }) as never;
const inputs = (over: Partial<RowInputs> = {}): RowInputs => ({
  features: [feat("ft-a", "alpha"), feat("ft-b", "bravo", { lead: null })],
  issuesOf: (f) => (f === "ft-a" ? ["is-1", "is-2"] : []), events: [],
  milestonesOf: () => [], initiativeName: () => null, labelName: () => null,
  nameOf: () => "Priya", dayOf: (i) => i.slice(0, 10), ...over,
});

describe("timeline rows", () => {
  // mutant killed: "span from createdAt"
  it("a feature with no started issue has no start", () => {
    const [g] = buildGroups("initiative", inputs());
    expect(g.rows.find((r) => r.feature.id === "ft-b")!.start).toBeNull();
  });
  it("the start is the earliest started move among the feature's issues", () => {
    const ev = [started(1, "is-2", "2026-10-03T01:00:00Z"), started(2, "is-1", "2026-10-01T01:00:00Z")];
    expect(firstStarted(["is-1", "is-2"], ev, (i) => i.slice(0, 10))).toBe("2026-10-01");
  });
  // mutant killed: "dropped rows"
  it("grouping by lead puts a lead-less feature in No lead, last", () => {
    const gs = buildGroups("lead", inputs());
    expect(gs.map((g) => g.label)).toEqual(["Priya", "No lead"]);
    expect(gs.flatMap((g) => g.rows).length).toBe(2);
  });
  // mutant killed: "one label wins" (a two-label feature must appear under each)
  it("a feature with two labels appears under each, keyed apart", () => {
    const gs = buildGroups("label", inputs({
      features: [feat("ft-a", "alpha", { labels: ["lb-1", "lb-2"] })],
      labelName: (id) => ({ "lb-1": "api", "lb-2": "ui" }[id] ?? null),
    }));
    expect(gs.map((g) => g.label)).toEqual(["api", "ui"]);
    expect(holdInput(gs).groups.flatMap((g) => g.rows.map((r) => r.issueId))).toEqual(["ft-a|lb-1", "ft-a|lb-2"]);
  });
  // mutant killed: "date order" (a target change would reorder rows under the reader)
  it("rows order by name, so a target change never reorders them", () => {
    const a = buildGroups("initiative", inputs());
    const b = buildGroups("initiative", inputs({
      features: [feat("ft-a", "alpha", { targetDate: "2027-03-01" }), feat("ft-b", "bravo", { lead: null })] }));
    expect(holdInput(b).groups).toEqual(holdInput(a).groups);
  });
});
```

#### Tests to write first — `desktop/src/components/roadmap/Timeline.test.tsx`

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { KEYMAP } from "../../work/keys/keymap";                          // task 53 (UNVERIFIED name)
import { CycleBand } from "./CycleBand";
import { Timeline, type TimelineProps } from "./Timeline";
import { timelineScale } from "./timelineScale";

const PRIYA = { id: "u-priya", displayName: "Priya", email: "p@example.test" };
const SAM = { id: "u-sam", displayName: "Sam", email: "s@example.test" };
const feat = (id: string, name: string, over: object = {}) => ({ id, alias: id.toUpperCase(),
  projectId: "pr-1", name, branchRef: `feat/${name}`, briefId: "br-1", lead: PRIYA,
  targetDate: "2026-10-14", state: "active", priority: "none", labels: [], initiativeId: null,
  createdAt: "", createdBy: PRIYA, updatedAt: "", ...over }) as never;
const cyc = (n: number, startsOn: string, endsOn: string, state = "upcoming") =>
  ({ id: `cy-${n}`, teamId: "tm-1", number: n, name: null, startsOn, endsOn, state }) as never;
const props = (features: never[], over: Partial<TimelineProps> = {}): TimelineProps => ({
  inputs: { features, issuesOf: () => ["is-1"], milestonesOf: () => [], initiativeName: () => null,
    labelName: () => null, nameOf: () => "Priya",
    events: [{ seq: 1, kind: "issue.state", subject: "is-1", at: "2026-10-02T00:00:00Z", actor: "u-priya",
      data: { from: "todo", to: "started" } } as never] },
  cycles: [], grouping: "lead", zoom: "month", focus: "2026-10-01", widthPx: 1200,
  timeZone: "Australia/Sydney", ...over,
});

describe("Timeline", () => {
  // mutant killed: "progress fill"
  it("a span holds one fill whatever its issues' states", () => {
    const { container } = render(<Timeline {...props([feat("ft-a", "alpha")])} />);
    const span = container.querySelector(".tl-span")!;
    expect(span).not.toBeNull();
    expect(span.children).toHaveLength(0);
    expect(container.querySelectorAll("[role=progressbar], progress, meter")).toHaveLength(0);
    expectNoPrediction(container);
  });
  // mutant killed: "cooldown drawn as a cycle"
  it("a cooldown is a gap in the band, not a cycle", () => {
    const s = timelineScale("month", "2026-10-01", "2026-12-31", 920, "Australia/Sydney");
    const { container } = render(<CycleBand scale={s} from="2026-10-01" to="2026-12-31"
      cycles={[cyc(1, "2026-10-01", "2026-10-14", "current"), cyc(2, "2026-10-22", "2026-11-04")]} />);
    expect(container.querySelectorAll("[data-cycle]")).toHaveLength(2);
    expect(container.querySelector("[data-current]")!.textContent).toBe("Cycle 1 · current");
    expect(screen.queryByText(/cooldown/i)).toBeNull();
  });
  // mutant killed: "the row jumps"
  it("a teammate's lead change keeps the row in place until N", () => {
    const before = [feat("ft-a", "alpha"), feat("ft-b", "bravo")];
    const { container, rerender } = render(<Timeline {...props(before)} />);
    fireEvent.pointerEnter(container.querySelector(".tl-group")!);          // attention on Priya's group
    rerender(<Timeline {...props([feat("ft-a", "alpha", { lead: SAM }), feat("ft-b", "bravo")])} />);
    const rows = () => [...container.querySelectorAll(".tl-row")].map((r) => r.getAttribute("data-feature"));
    expect(rows()).toEqual(["ft-a", "ft-b"]);                               // held in Priya's group
    expect(container.querySelector('[data-feature="ft-a"]')!.hasAttribute("data-held")).toBe(true);
    fireEvent.keyDown(container.querySelector(".tl")!, { code: "KeyN", key: "n" });
    expect(rows()).toEqual(["ft-b", "ft-a"]);                               // Priya's, then Sam's
  });
  it("names who set the target, and nothing when unknown", () => {
    render(<Timeline {...props([feat("ft-a", "alpha")])} />);
    expect(screen.getByText("target 14 Oct")).toBeTruthy();                // no feature.updated event
  });
  // mutant killed: "a chord missing from the sheet"
  it("the zoom chords are rows of the keymap", () => {
    const verbs = KEYMAP.filter((b) => b.scope === "timeline").map((b) => b.verb).sort();
    expect(verbs).toEqual(["timeline.zoomIn", "timeline.zoomOut"]);
  });
});
```

The last row stands for the scope's *"in `keys.list`"*: task 54's sheet lists `KEYMAP`, so a
row in it is a row on the sheet; task 54's own test is the one that renders the sheet.

#### Steps

1. Write `timelineScale.test.ts`; it fails. Write `timelineScale.ts`; it passes.
2. Write `timelineRows.test.ts`; it fails. Write `timelineRows.ts`; it passes.
3. Write `Timeline.test.tsx`; it fails. Write `CycleBand.tsx`, `TimelineAxis.tsx`,
   `TimelineRow.tsx`, `Timeline.tsx` (with `TimelineMount`) and `timeline.css`; it passes.
4. Widen ⚑ `DisplayMenu.tsx`'s types and add the timeline's groupings and zoom; run task 44's
   tests unchanged (the three existing view ids read as before).
5. Add the switch to ⚑ `PlanSurface.tsx`; run task 45's tests unchanged (`PLAN_SECTIONS` holds).
6. Add the two rows to ⚑ `keymap.ts` and `"W4"` to its active set if absent; run task 53's and
   54's tests.
7. Run the acceptance; commit.

#### Acceptance

```bash
cd desktop && npx vitest run src/components/roadmap src/components/work/DisplayMenu.test.tsx src/components/work/PlanSurface.test.tsx src/work/keys
# expect: timelineScale 3 passed; timelineRows 5 passed; Timeline 5 passed; task 44's, 45's and
#         53's tests pass unchanged
cd desktop && npx tsc --noEmit
# expect: exit 0
cd desktop && grep -n -E "transition|animation|Date\.now|getTime\(\)" src/components/roadmap/timelineScale.ts src/components/roadmap/timelineRows.ts src/components/roadmap/timeline.css
# expect: no output (decision 1: x never sees an instant; nothing on the timeline animates)
cd desktop && grep -n -E -i --exclude="*.test.tsx" "progress|percent|\beta\b|forecast|on track" src/components/roadmap/*.tsx
# expect: no output (decision 3, OD-27)
```

#### What this task does NOT claim

- Drag to reschedule, or dependencies (task 129).
- Smooth rendering at any number of rows, or any frame rate: nothing here is measured.
- That the timeline's layout survives a restart or lives in a saved view (decision 8, gap 1).
- That a span says how far along a feature is. It says when its first issue started and when a
  person wants it done; nothing between the two ends means anything.
- Initiatives as their own rows. The grouping names initiatives; task 127 owns their surface.

#### Gaps found while writing this part

1. **`View.layout` has no `"timeline"`** (task 72's scope types it `"list" | "board"`). A saved
   view cannot keep the timeline until 72 widens it and 83 mirrors it.
2. **Who set a feature's target.** `Feature` has no `targetSetBy`; this part reads the actor of
   the latest `feature.updated` event whose `data.fields` includes `targetDate`. Task 72 must say
   that `feature.updated` carries `fields` (or add `targetSetBy` to the feature).
3. **`Feature.lead` is required** in task 72's scope (`lead User`), so *No lead* can only be empty.
   The scope for 128 asks for *No lead*; either 72 makes `lead` nullable or the group is dead code.
4. **Task 41's `RowFact` is issue-shaped.** Its `state` slot is the only change key
   (`task-41.md:205`); a feature row passes a constant. A neutral `version` field would let a
   feature's own change carry a cause line.
5. **The file map.** The scope names no file for the row model; this part adds
   `timelineRows.ts`. No parts exist for tasks 53, 54, 72, 77 or 83, so `KEYMAP`, `registerVerb`,
   `Feature`, `Cycle` and the replica's feature reads are their scopes' names and **UNVERIFIED**.
6. **Feature aliases.** Task 129 takes `Feature & { alias }` (`task-129.md:252`); no written type
   says where a feature's display alias comes from. This part takes it from the host, as 129 does.
