### Task 132: the insights panel and scatter; every point opens its issue

W4, stream WD, the desktop half of task 122 (Q1: *measured counts and durations, a secondary
surface*). Depends on task 122 (the query) and task 125 (`InsightsQuery`, `InsightsResult`,
`WorkClient.queryInsights`). Estimate 0.375 (3 h). Closes **L-150** (less *effort*) and **L-151**
with task 122, the time axis waiting on task 114 (gap 1).

After this task a person opens *Insights* from Plan's header, picks a measure, a grouping and a
range, and reads plain counts per group and a strip of measured durations with the Hub's p50, p75
and p90 marked. Every mark is a button that opens its issue, and the arrow keys walk them. There
is no trend line, no forecast band and no target: nothing on the panel is a number nobody measured.

**Decisions made here, so nobody re-derives them:**

1. **The Hub's numbers, drawn, never recomputed.** Percentile lines sit at `p50Ms`, `p75Ms` and
   `p90Ms` from `InsightsResult` (`task-125.md:172-175`) and are labelled with those values
   (*"p75 8h 00m"*). The client computes no mean, median or rank; a test holds the label to the
   Hub's value when the points' mean differs. `null` percentiles draw no line.
2. **A strip until the Hub sends `at`.** A scatter needs each point's end time on the x axis;
   `InsightPoint` is `{issue, ms}` (`task-125.md:170`) and task 122 names the missing `at` as its
   gap G2 (`task-122.md:52-58`). So this task draws **one axis**: duration along x, marks stacked
   in lanes where they would overlap, with the same percentile lines, and a caption says *"By
   duration. The time axis needs each point's end time, which the Hub does not send yet."* When
   114 adds `at`, the strip becomes the two-axis scatter by adding a y scale, and nothing else.
3. **Counts are a table, not bars.** `groups [{key, count}]` renders as a two-column table:
   the group's name (resolved from the replica: a person's name for `u-`, a label's for `lb-`, a
   feature's for `ft-`, a cycle's for `cy-`, the criterion's text for `bf-…/done:n`; `none` reads
   *No assignee*, *No label*, and so on) and the plain number in `tabular-nums`. No bar, no share,
   no total: an issue in two groups is counted in each (`task-122.md:29-35`), so a total would lie.
4. **Every mark is a button, in one tab stop.** Marks are HTML `<button>`s placed with
   `transform`, a roving `tabIndex` (one mark is `0`, the rest `-1`), in **point order**: ascending
   `ms`, ties by issue id, which is also left to right. `ArrowRight`/`ArrowLeft` move one mark,
   `Home`/`End` to the ends; `Enter` or a click opens the issue through the store's
   `openIssue(id)` (task 37, `task-37.md:560`). Each mark's name is *"EP-142, cycle time 5h 10m"*
   (AX Principle 7: every visual claim is a link to its source).
5. **The query is bounded as the Hub bounds it.** `from < to` and at most 366 days
   (`task-122.md:48-51`); the bar refuses anything else inline and sends nothing. Days are the
   person's, converted to the instant their day starts in their time zone.
6. **A secondary surface.** The only entry is an *Insights* action in Plan's `<ViewHeader
   actions>` (SW.13). The rail and the Desk import nothing from it (Q1); a test reads their source.
   The result lives in the panel's state only: never in the replica, the store or storage (task
   125 decision 3).

**Files:**
- `desktop/src/components/roadmap/InsightsQueryBar.tsx` — measure, group, range; `startOfDayIso`
  (new)
- `desktop/src/components/roadmap/InsightsScatter.tsx` — the strip, the lines, the marks (new)
- `desktop/src/components/roadmap/InsightsPanel.tsx` — the query, the table, the strip (new)
- `desktop/src/components/roadmap/insights.css` — `.in-*` rules (new)
- `desktop/src/components/roadmap/Insights.test.tsx` — the tests below (new)
- `desktop/src/components/work/PlanSurface.tsx` ⚑ (task 45) — the *Insights* action (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 37, 45, 108, 122 and
125. `formatDuration` is task 108's (`task-108.md:139-145`, `"1h 02m"`), in
`desktop/src/components/issue/StepFold.tsx`; this task imports it from there rather than writing a
second one. `useCockpit` is the store hook on the tree today (`desktop/src/store/store.ts:91`);
`openIssue` is task 37's slice member. The rail is `desktop/src/components/rail/Rail.tsx` (on the
tree); the Desk is task 62's `desktop/src/components/desk/Desk.tsx` (**UNVERIFIED**, planned).

#### Interfaces — complete: `desktop/src/components/roadmap/InsightsScatter.tsx`

```tsx
// desktop/src/components/roadmap/InsightsScatter.tsx — measured durations on one axis (decision
// 2), the Hub's percentiles as labelled lines, every mark a button to its issue.
import { useRef, useState, type JSX, type KeyboardEvent } from "react";
import type { InsightPoint, InsightsMeasure, InsightsResult } from "../../api/work";
import { formatDuration } from "../issue/StepFold";               // task 108

export const MEASURE_WORDS: Record<Exclude<InsightsMeasure, "count">, string> = {
  cycle_time: "cycle time", lead_time: "lead time", triage_time: "triage time",
};

/** Decision 4: ascending ms, ties by id; the same order left to right and under the arrows. */
export function pointOrder(points: readonly InsightPoint[]): InsightPoint[] {
  return [...points].sort((a, b) => a.ms - b.ms || (a.issue < b.issue ? -1 : a.issue > b.issue ? 1 : 0));
}

/** Lanes so marks never overlap: a mark goes in the first lane whose last mark ends before it. */
export function lanes(xs: readonly number[], markPx: number): number[] {
  const ends: number[] = [];
  return xs.map((x) => {
    let l = ends.findIndex((e) => e + markPx <= x);
    if (l === -1) { l = ends.length; ends.push(x); } else ends[l] = x;
    return l;
  });
}

export interface InsightsScatterProps {
  result: InsightsResult;
  measure: Exclude<InsightsMeasure, "count">;
  aliasOf(issueId: string): string;     // the replica's alias, "EP-142"
  openIssue(issueId: string): void;     // task 37's slice member
  widthPx: number;
}

const MARK = 8;

export function InsightsScatter(p: InsightsScatterProps): JSX.Element {
  const pts = pointOrder(p.result.points);
  const [at, setAt] = useState(0);
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  if (pts.length === 0) return <p className="in-empty">No closed issue in this range</p>;
  const max = pts[pts.length - 1].ms || 1;
  const x = (ms: number) => Math.round((ms / max) * (p.widthPx - MARK));
  const xs = pts.map((q) => x(q.ms));
  const lane = lanes(xs, MARK + 2);
  const word = MEASURE_WORDS[p.measure];
  const move = (i: number) => { const n = Math.max(0, Math.min(pts.length - 1, i)); setAt(n); refs.current[n]?.focus(); };
  const onKey = (e: KeyboardEvent) => {
    const k = { ArrowRight: at + 1, ArrowLeft: at - 1, Home: 0, End: pts.length - 1 }[e.key];
    if (k !== undefined) { e.preventDefault(); move(k); }
  };
  const lines = ([["p50", p.result.p50Ms], ["p75", p.result.p75Ms], ["p90", p.result.p90Ms]] as const)
    .filter((l): l is readonly [string, number] => l[1] !== null);
  const height = (Math.max(...lane) + 1) * (MARK + 2) + 24;
  return (
    <figure className="in-strip">
      <div className="in-plot" style={{ width: p.widthPx, height }} role="group"
           aria-label={`${pts.length} issues by ${word}`} onKeyDown={onKey}>
        {lines.map(([name, ms]) => (
          <div key={name} className="in-pline" data-p={name} style={{ transform: `translateX(${x(ms)}px)` }}>
            <span className="in-plabel">{name} {formatDuration(ms)}</span>
          </div>
        ))}
        {pts.map((q, i) => (
          <button key={q.issue} type="button" className="in-mark" ref={(el) => { refs.current[i] = el; }}
            tabIndex={i === at ? 0 : -1}
            aria-label={`${p.aliasOf(q.issue)}, ${word} ${formatDuration(q.ms)}`}
            style={{ transform: `translate(${xs[i]}px, ${24 + lane[i] * (MARK + 2)}px)` }}
            onFocus={() => setAt(i)} onClick={() => p.openIssue(q.issue)} />
        ))}
      </div>
      <figcaption className="in-cap">
        By duration. The time axis needs each point&rsquo;s end time, which the Hub does not send yet.
      </figcaption>
    </figure>
  );
}
```

`.in-mark` is an 8 px square with a 1 px ring in one hue (`currentColor` on `--t-fact`), a 24 px
hit area through padding, and `:focus-visible` a 2 px ring; no hover animation. `.in-plabel` is
`var(--mono)` with `tabular-nums`. A duration ending past 99 hours reads as task 108 formats it
(*"104h 05m"*); there is no day unit, so no rounding hides a measured hour.

#### Interfaces — complete: `desktop/src/components/roadmap/InsightsQueryBar.tsx`

```tsx
// desktop/src/components/roadmap/InsightsQueryBar.tsx — what to measure, over which days.
import { useState, type JSX } from "react";
import type { InsightsGroupBy, InsightsMeasure, InsightsQuery } from "../../api/work";

const MEASURES: [InsightsMeasure, string][] = [
  ["count", "Issues"], ["cycle_time", "Cycle time"], ["lead_time", "Lead time"], ["triage_time", "Triage time"]];
const GROUPS: [InsightsGroupBy | "", string][] = [["", "No grouping"], ["state", "State"],
  ["assignee", "Assignee"], ["label", "Label"], ["feature", "Feature"], ["cycle", "Cycle"],
  ["priority", "Priority"], ["criterion", "Criterion"]];   // the last two: task 122's G1, in 114

/** The instant `day` starts in `tz`, as RFC 3339 UTC. Two passes, so a day on a daylight-saving
 *  change starts at its own offset, not the day before's. */
export function startOfDayIso(day: string, tz: string): string {
  const [y, m, d] = day.split("-").map(Number);
  const guess = Date.UTC(y, m - 1, d);
  let t = guess - offsetMs(guess, tz);
  t = guess - offsetMs(t, tz);
  return new Date(t).toISOString();
}
function offsetMs(t: number, tz: string): number {
  const p = Object.fromEntries(new Intl.DateTimeFormat("en-US", { timeZone: tz, hourCycle: "h23",
    year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" })
    .formatToParts(t).map((x) => [x.type, x.value]));
  return Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute, +p.second) - t;
}

/** Decision 5, the Hub's bound: from < to, at most 366 days. Null when the range may be sent. */
export function rangeProblem(fromDay: string, toDay: string): string | null {
  const span = (Date.parse(toDay) - Date.parse(fromDay)) / 86_400_000;
  if (!(span > 0)) return "The range must end after it starts.";
  if (span > 366) return "The range can be at most 366 days.";
  return null;
}

export function InsightsQueryBar(p: { teamId: string; projectId: string | null; tz: string;
  initial: { from: string; to: string }; onQuery(q: InsightsQuery): void }): JSX.Element {
  const [measure, setMeasure] = useState<InsightsMeasure>("cycle_time");
  const [groupBy, setGroupBy] = useState<InsightsGroupBy | "">("");
  const [from, setFrom] = useState(p.initial.from);
  const [to, setTo] = useState(p.initial.to);
  const problem = rangeProblem(from, to);
  return (
    <form className="in-bar" onSubmit={(e) => {
      e.preventDefault();
      if (problem) return;                                       // nothing sent
      p.onQuery({ teamId: p.teamId, projectId: p.projectId, measure, groupBy: groupBy || null,
        from: startOfDayIso(from, p.tz), to: startOfDayIso(to, p.tz) });
    }}>
      <label>Measure <select value={measure} onChange={(e) => setMeasure(e.target.value as InsightsMeasure)}>
        {MEASURES.map(([v, t]) => <option key={v} value={v}>{t}</option>)}</select></label>
      <label>Group by <select value={groupBy} onChange={(e) => setGroupBy(e.target.value as InsightsGroupBy | "")}>
        {GROUPS.map(([v, t]) => <option key={v} value={v}>{t}</option>)}</select></label>
      <label>From <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} /></label>
      <label>To <input type="date" value={to} onChange={(e) => setTo(e.target.value)} /></label>
      <button type="submit" disabled={problem !== null}>Show</button>
      {problem && <p className="in-fact" role="status">{problem}</p>}
    </form>
  );
}
```

The *to* day is exclusive: the query ends where that day starts, the Hub's `to` (`task-122.md`
decision 6). The bar's default range is the last 30 whole days ending today, set by the panel.

#### Interfaces — complete: `desktop/src/components/roadmap/InsightsPanel.tsx`

```tsx
// desktop/src/components/roadmap/InsightsPanel.tsx — Plan's secondary surface (decision 6).
// The result is held here only: never the replica, the store or storage (task 125 decision 3).
import { useState, type JSX } from "react";
import type { InsightsQuery, InsightsResult, WorkClient } from "../../api/work";
import { ViewHeader } from "../common/ViewHeader";                 // SW.13, task 31
import { InsightsQueryBar } from "./InsightsQueryBar";
import { InsightsScatter } from "./InsightsScatter";
import "./insights.css";

const NONE_WORDS: Record<string, string> = { assignee: "No assignee", label: "No label",
  feature: "No feature", cycle: "No cycle", criterion: "Serves no criterion", priority: "No priority" };

export interface InsightsPanelProps {
  work: Pick<WorkClient, "queryInsights">;
  teamId: string; projectId: string | null; tz: string; today: string;   // today: YYYY-MM-DD in tz
  /** Resolves a group key to words (decision 3): the replica's names. */
  nameOf(groupBy: string, key: string): string;
  aliasOf(issueId: string): string;
  openIssue(issueId: string): void;
  onClose(): void;
}

const minusDays = (day: string, n: number) =>
  new Date(Date.parse(day) - n * 86_400_000).toISOString().slice(0, 10);

export function InsightsPanel(p: InsightsPanelProps): JSX.Element {
  const [asked, setAsked] = useState<InsightsQuery | null>(null);
  const [result, setResult] = useState<InsightsResult | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const run = (q: InsightsQuery) => {
    setAsked(q); setProblem(null);
    p.work.queryInsights(q).then(setResult, (e: Error) => { setResult(null); setProblem(e.message); });
  };
  const label = (key: string) =>
    key === "none" && asked?.groupBy ? NONE_WORDS[asked.groupBy] ?? "None" : p.nameOf(asked?.groupBy ?? "", key);
  return (
    <section className="in-panel" aria-label="Insights">
      <ViewHeader title="Insights" actions={<button type="button" onClick={p.onClose}>Close</button>} />
      <InsightsQueryBar teamId={p.teamId} projectId={p.projectId} tz={p.tz}
        initial={{ from: minusDays(p.today, 30), to: p.today }} onQuery={run} />
      {problem && <p className="in-fact" role="status">{problem}</p>}
      {result && asked && result.groups.length > 0 && (
        <table className="in-groups">
          <thead><tr><th scope="col">{asked.groupBy ?? "Group"}</th><th scope="col">Issues</th></tr></thead>
          <tbody>{result.groups.map((g) => (
            <tr key={g.key}><th scope="row">{label(g.key)}</th><td className="in-n">{g.count}</td></tr>
          ))}</tbody>
        </table>
      )}
      {result && asked && asked.measure !== "count" && (
        <InsightsScatter result={result} measure={asked.measure} aliasOf={p.aliasOf}
          openIssue={p.openIssue} widthPx={560} />
      )}
      {result && asked?.measure === "count" && result.groups.length === 0 && (
        <p className="in-empty">No issue in this range</p>
      )}
    </section>
  );
}
```

`ViewHeader`'s props are SW.13's (`title route badge actions`); `route` and `badge` are omitted, as
a secondary panel has no route of its own. `.in-n` is `tabular-nums`, right-aligned. With
`groupBy` null and `measure: "count"`, what `groups` holds is task 122's to say (gap 2).

⚑ `PlanSurface.tsx` (task 45) gains one action in its `ViewHeader actions`, *Insights*, which
mounts the panel in place of the sections and returns to them on *Close*. It gets no key chord in
this task, so it adds no row to the `?` sheet.
