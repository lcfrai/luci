### Task 134: recurring issues, triage rotation, the method page

W4, stream WD; no seam. Depends on task 121 (the janitor's W4 passes and the five policy routes,
`task-121.md:293-326`) and, through it, task 125 (the desktop W4 types and `WorkClient` members,
`task-125.md:146-154`, `:298-302`, `:352-356`). It is the desktop half of task 121 and the W4 half
of 05 §1 R's **L-253** (*"a user-facing version ships in W4"*, `05-design.md:445`). Estimate 0.3
(2.5 h).

After this task a team admin can set an issue template to repeat and set who is on triage duty,
everyone in the team can see who is on duty in the Triage view, a recurring issue says whose rule
made it, and the palette's *How we work* opens a page that states the product's rules beside the
checks a reviewer can run.

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 42, 46, 121 and 125
(`…-parts/task-42.md`, `task-46.md`, `task-121.md`, `task-125.md`). There is **no part for tasks
59, 84 or 89**, so `buildCommands`, `Command` and team settings' host are their scopes' names
(`…-scopes/tasks-41-60.md`, `tasks-81-100.md:105-120`, `:203-220`) and **UNVERIFIED**. The
desktop has `desktop/src/components/{palette,lens,common}/` and `desktop/src/test/{fakes,
fixtures,setup}.ts` today; `components/work/`, `components/plan/` and `docs/work/` do not exist
yet and are planned by earlier W tasks. `docs/AGENTIC-UX.md` §10 (`:232-251`) holds sixteen
numbered anti-patterns; §7 (`:175-194`) names the four levels. Both verified on this tree.

**Decisions made here, so nobody re-derives them:**

1. **The zone is the person's IANA name, never an offset.** *Repeat* reads
   `Intl.DateTimeFormat().resolvedOptions().timeZone` once at mount (tests pin it through the
   `timeZone` prop) and sends it as `tz`. Task 121 turns local wall time into instants with the
   zone's own rules, so an offset would put a Sydney 09:00 at 08:00 for half the year.
2. **Duty is the Hub's, drawn as read.** The on-duty line renders `TriageRotationView.onDuty` and
   `handoverOn` exactly; the desktop never computes `startsOn + k·periodDays` itself (task 121,
   decision 7, is the only derivation). A read that fails or 404s (`rotation_not_found`) draws no
   line, never a guessed name.
3. **Boundaries are local midnights, so the line names a date, not an hour.** The scope's example
   reads *"until Tue 09:00"*; task 121 decision 6 makes a rotation's boundary the local midnight of
   `handoverOn`. The line reads *"On triage duty: Priya, until Tue 6 Oct · rotation set by Sam"*.
   The disagreement is gap 3 below; this part follows the written Hub part.
4. **A recurring issue names its author, never a system.** Task 121 writes the issue with
   `source: "recurring"` and `createdBy` the schedule's setter. The row's origin line reads
   *"from schedule · weekly, Monday 09:00 · set by Priya"*. When the schedule is no longer on the
   template (cleared since), it reads *"from schedule · set by Priya"*: the author is on the issue
   itself, so it never goes missing.
5. **Only a team admin sees the settings.** Task 121's routes answer 403 to a member; the desktop
   does not draw what it cannot do (task 126, decision 7's write gate). A member sees the on-duty
   line and the recurring rows, which are reads.
6. **The method page is one Markdown file, rendered, with a doc test.** `docs/work/method.md` is the
   source; `MethodPage.tsx` imports it with Vite's `?raw` and renders it with task 49's
   `makeTokenizer().render(…)` (`desktop/src/components/editor/markdown.ts`, `task-49.md:246-256`:
   CommonMark, `html: false`, images off, links limited to `LIVE_LINK`). Every rule on the
   page cites an anti-pattern as `AX-10.N "Title"`; a Vitest reads both files by path and requires
   each citation's number and title to match `AGENTIC-UX.md` §10. Clean room (rule 4): the page is
   our own words about our own product; it quotes nothing from Linear's method.
7. **No new wire.** Every call is task 125's (`setTemplateSchedule`, `clearTemplateSchedule`,
   `setTriageRotation`, `triageRotation`). The gaps below are fields on those types, raised against
   tasks 114 and 125, not new routes.

**Files:**

- `desktop/src/components/plan/RepeatField.tsx`, `RepeatField.test.tsx` (new) — the *Repeat* field
- `desktop/src/components/plan/RecurringIssues.tsx` (new) — team settings' list of the team's issue
  templates, each with a `RepeatField` (the host; gap 1)
- `desktop/src/components/plan/TriageRotationSettings.tsx`, `.test.tsx` (new) — beside task 84's
  `TeamStates.tsx` in team settings
- `desktop/src/components/work/TriageView.tsx` ⚑ (task 42) — one on-duty line in its header
  (modified)
- `desktop/src/components/work/IssueRow.tsx` ⚑ (task 40) and
  `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46) — the origin line for a recurring issue
  (modified)
- `desktop/src/work/recurring.ts`, `recurring.test.ts` (new) — `repeatWords`, `originLine`,
  `dutyLine`, pure
- `docs/work/method.md` (new) — the method, in our words
- `desktop/src/components/work/MethodPage.tsx`, `MethodPage.test.tsx` (new)
- `desktop/src/components/work/method.doc.test.ts` (new) — the citation check
- `desktop/src/components/palette/commands.ts` ⚑ (task 59) — *How we work* (modified)

#### Interfaces — complete: `desktop/src/work/recurring.ts`

```ts
import type { IssueHead, TemplateSchedule, TriageRotationView, User } from "../api/work";

/** Task 121's G1 anchor. Task 125's `TemplateSchedule` has no `startsOn` yet (gap 2); until it
 *  does, the words leave the day out rather than guess it. */
export type AnchoredSchedule = TemplateSchedule & { startsOn?: string };
/** Gap 3: the rotation read names no setter. Absent, the "set by" clause is left out. */
export type RotationRead = TriageRotationView & { setBy?: User };

const WEEKDAY = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"] as const;
const DAY = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
const MONTH = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] as const;
const ONCE = { day: "daily", week: "weekly", month: "monthly" } as const;

/** The weekday of a YYYY-MM-DD local date, read from its own digits at UTC noon, so the
 *  reader's zone can never move it to the day before. */
function weekdayOf(date: string): number {
  return new Date(`${date}T12:00:00Z`).getUTCDay();
}
function ordinal(n: number): string {
  const v = n % 100;
  const suffix = v >= 11 && v <= 13 ? "th" : ({ 1: "st", 2: "nd", 3: "rd" } as Record<number, string>)[n % 10] ?? "th";
  return `${n}${suffix}`;
}

/** "daily, 09:00" · "every 3 days, 09:00" · "weekly, Monday 09:00" ·
 *  "every 2 weeks, Monday 09:00" · "monthly, the 5th, 09:00". */
export function repeatWords(s: AnchoredSchedule): string {
  const { n, unit } = s.every;
  const head = n === 1 ? ONCE[unit] : `every ${n} ${unit}s`;
  if (!s.startsOn || unit === "day") return `${head}, ${s.at}`;
  if (unit === "week") return `${head}, ${WEEKDAY[weekdayOf(s.startsOn)]} ${s.at}`;
  return `${head}, the ${ordinal(Number(s.startsOn.slice(8, 10)))}, ${s.at}`;
}

/** Decision 4. `null` for any issue a person, an orchestrator or intake made. The author is the
 *  issue's own `createdBy`, so a cleared schedule still names whose rule made the issue. */
export function originLine(
  head: Pick<IssueHead, "source" | "createdBy">,
  schedule: AnchoredSchedule | null,
): string | null {
  if (head.source !== "recurring") return null;          // gap 4: task 37's IssueSource
  const by = `set by ${head.createdBy.displayName}`;
  return schedule ? `from schedule · ${repeatWords(schedule)} · ${by}` : `from schedule · ${by}`;
}

/** "Tue 6 Oct", from the Hub's YYYY-MM-DD. Never from `Date.now()` (decisions 2 and 3). */
export function dayWords(date: string): string {
  const [, m, d] = date.split("-").map(Number);
  return `${DAY[weekdayOf(date)]} ${d} ${MONTH[m - 1]}`;
}

/** Decision 2: the Hub's read, drawn as read. */
export function dutyLine(v: RotationRead): string {
  const tail = v.setBy ? ` · rotation set by ${v.setBy.displayName}` : "";
  return `On triage duty: ${v.onDuty.displayName}, until ${dayWords(v.handoverOn)}${tail}`;
}

/** Decision 1: the person's IANA zone, read once at mount. */
export function localZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}
```

`handoverOn` is task 125's name. Task 114's wire struct says `nextHandoverOn`
(`task-114.md:285`); the two disagree, which is gap 5. This part follows 125, the desktop's type,
and task 125's golden check is where the rename lands.

#### Interfaces — complete: `desktop/src/components/plan/RepeatField.tsx`

```tsx
import { useState } from "react";
import type { ScheduleUnit, TemplateSchedule } from "../../api/work";
import { localZone, repeatWords, type AnchoredSchedule } from "../../work/recurring";

export interface RepeatFieldProps {
  templateId: string;
  /** What the host holds; there is no read of a template's schedule (gap 1). */
  value: AnchoredSchedule | null;
  /** Team admin only (decision 5). A member sees the words, never the controls. */
  canEdit: boolean;
  /** Tests pin the zone here; the app leaves it unset and the field reads it once. */
  timeZone?: string;
  set(templateId: string, s: TemplateSchedule): Promise<TemplateSchedule>;
  clear(templateId: string): Promise<void>;
  onSaved?(s: TemplateSchedule | null): void;
}

const UNITS: ScheduleUnit[] = ["day", "week", "month"];
const HHMM = /^([01]\d|2[0-3]):[0-5]\d$/;

export function RepeatField(p: RepeatFieldProps) {
  const [tz] = useState(() => p.timeZone ?? localZone());        // decision 1: once, at mount
  const [on, setOn] = useState(p.value !== null);
  const [n, setN] = useState(p.value?.every.n ?? 1);
  const [unit, setUnit] = useState<ScheduleUnit>(p.value?.every.unit ?? "week");
  const [at, setAt] = useState(p.value?.at ?? "09:00");
  const [error, setError] = useState("");

  if (!p.canEdit) {
    return <p className="repeat-words">{p.value ? `Repeats ${repeatWords(p.value)}` : "Does not repeat"}</p>;
  }
  const valid = !on || (Number.isInteger(n) && n >= 1 && n <= 365 && HHMM.test(at));   // T114's ranges
  async function save() {
    setError("");
    try {
      if (!on) { await p.clear(p.templateId); p.onSaved?.(null); return; }
      const s = await p.set(p.templateId, { templateId: p.templateId, every: { n, unit }, at, tz });
      p.onSaved?.(s);
    } catch (e) {
      setError(`Not saved: ${(e as Error).message}`);          // matter-of-fact, the Hub's reason
    }
  }
  return (
    <fieldset className="repeat-field">
      <legend>Repeat</legend>
      <label><input type="checkbox" checked={on} onChange={(e) => setOn(e.target.checked)} /> Repeats</label>
      {on && (
        <>
          <label>Every <input type="number" min={1} max={365} value={n}
            onChange={(e) => setN(Number(e.target.value))} aria-label="Every" /></label>
          <select aria-label="Unit" value={unit} onChange={(e) => setUnit(e.target.value as ScheduleUnit)}>
            {UNITS.map((u) => <option key={u} value={u}>{n === 1 ? u : `${u}s`}</option>)}
          </select>
          <label>at <input type="time" value={at} onChange={(e) => setAt(e.target.value)} aria-label="At" /></label>
          <span className="repeat-zone tabular-nums">{tz}</span>
        </>
      )}
      <button type="button" disabled={!valid} onClick={save}>Save</button>
      {error && <p role="alert">{error}</p>}
    </fieldset>
  );
}
```

The zone is shown, never chosen: the schedule runs in the zone of the person who set it, and a
person who wants another zone sets it from there. The first issue comes at the **next**
occurrence (task 121, decision 3); the field says nothing about when, because a date the Hub has
not fixed would be a forecast (`OD-27`).

**`RecurringIssues.tsx`** (the host, gap 1) lists the team's issue templates from task 83's
template read (**UNVERIFIED** name, `work.issueTemplates(teamId)`), one row per template: its
name, then a `RepeatField` with `canEdit` from the team role and `value` from the template's own
`schedule` member (gap 1 asks task 83 for it). It is 40 lines of list chrome and has no test of
its own; the field's tests cover every behaviour it has.

#### Interfaces — complete: `desktop/src/components/plan/TriageRotationSettings.tsx`

```tsx
import { useState } from "react";
import type { TriageRotation, TriageRotationView, User } from "../../api/work";
import { useIsTeamAdmin } from "../../work/access/projectAccess";   // task 126
import { dutyLine, type RotationRead } from "../../work/recurring";

export interface TriageRotationSettingsProps {
  teamId: string;
  members: User[];                       // the team's current members, in the host's order
  read: RotationRead | null;             // null: no rotation (404 rotation_not_found)
  set(teamId: string, r: TriageRotation): Promise<TriageRotationView>;
  /** The Hub fixes `startsOn` today in the app; tests pass a fixed date. */
  today: string;                          // YYYY-MM-DD
  onSaved?(v: TriageRotationView): void;
}

export function TriageRotationSettings(p: TriageRotationSettingsProps) {
  const isAdmin = useIsTeamAdmin(p.teamId);
  const [picked, setPicked] = useState<string[]>(p.read?.rotation.members ?? []);
  const [periodDays, setPeriodDays] = useState(p.read?.rotation.periodDays ?? 7);
  const [error, setError] = useState("");
  if (!isAdmin) return null;                                         // decision 5
  const toggle = (id: string) =>
    setPicked((xs) => (xs.includes(id) ? xs.filter((x) => x !== id) : [...xs, id]));
  const valid = picked.length >= 1 && picked.length <= 50 && periodDays >= 1 && periodDays <= 28;
  async function save() {
    setError("");
    try {
      const startsOn = p.read?.rotation.startsOn ?? p.today;         // re-saving keeps the anchor
      p.onSaved?.(await p.set(p.teamId, { teamId: p.teamId, members: picked, periodDays, startsOn }));
    } catch (e) {
      setError(`Not saved: ${(e as Error).message}`);
    }
  }
  return (
    <section className="triage-rotation" aria-label="Triage rotation">
      <h3>Triage rotation</h3>
      {p.read && <p className="duty-line">{dutyLine(p.read)}</p>}
      <ol className="rotation-order">
        {p.members.map((m) => (
          <li key={m.id}>
            <label><input type="checkbox" checked={picked.includes(m.id)} onChange={() => toggle(m.id)} />
              {m.displayName}</label>
            {picked.includes(m.id) && <span className="tabular-nums">{picked.indexOf(m.id) + 1}</span>}
          </li>
        ))}
      </ol>
      <label>Each person holds duty for <input type="number" min={1} max={28} value={periodDays}
        aria-label="Days on duty" onChange={(e) => setPeriodDays(Number(e.target.value))} /> days</label>
      <button type="button" disabled={!valid} onClick={save}>Save</button>
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
```

The order is the order people are ticked, shown as a plain number beside each name; the Hub keeps
it (task 121, decision 7). A person who leaves the team drops out of duty at the Hub's next read,
not here.

#### The on-duty line — ⚑ `TriageView.tsx` (task 42), and the origin line

```tsx
// added to TriageView.tsx: the host draws the header (task-42.md:168-169); this is the one line
// inside the view, above the list. Its read is a hook, so the list below never waits on it.
export function DutyLine({ projectId }: { projectId: string }): JSX.Element | null {
  const teamId = useWork((r) => r.scope().find((s) => s.projectId === projectId)?.teamId ?? null);
  const read = useTriageRotation(teamId);             // work.triageRotation(teamId); null on 404 or error
  return read ? <p className="duty-line">{dutyLine(read)}</p> : null;     // decision 2: no guess
}

// ⚑ IssueRow.tsx (task 40) and IssueHead.tsx (task 46): one line under the title, only when set.
const origin = originLine(head, scheduleOf(head.id));
{origin && <p className="issue-origin">{origin}</p>}
```

`useTriageRotation` is a 12-line hook over `createClient().work.triageRotation` beside
`DutyLine`: it refetches on the feed's `triage.handover` and `triage.rotation_set` events (task
121's kinds) and on nothing else, so the line never polls and never shifts while read (AX §10.7).
`scheduleOf(issueId)` looks up the schedule through the `template.schedule_ran` event that names
the issue (task 121, decision 5: `{templateId, issueId}`) and the host's template list; when either
is missing it is `null` and the short origin line is drawn (decision 4).

#### The document — complete: `docs/work/method.md`

````markdown
# How we work

Six rules the product keeps. Each names the check a reviewer runs to see it kept.

## One thread, from the brief to the release

A brief states what is wanted as criteria. An issue says which criteria it serves. A lane's work
lands against the issue, and a release lists what it ships by criterion. Each step points at the
one before it, so anything on screen can be traced to where it came from.
Check: AX-10.1 "No UI element without a nameable transcript span"

## Done means proven

An issue is done when the evidence for its criteria is in, not when someone says so. A claim
made without evidence is shown as a claim. Work is described by how far the criteria agree, never
by a share of a total.
Check: AX-10.2 "No percent-complete, progress bar, ETA, or "N of M met""

## People move the states that are theirs

Started, in review and landed are facts: they are written when a lane is bound, marked ready or
landed, and nobody sets them by hand. Done is a fact at landing, or a person's call with a reason
on record. The states around them are a person's. An agent may propose a change to an issue or a
brief; a person accepts it.
Check: AX-10.6 "No approval row without four things"

## One voice per owner

Each person has one Orchestrator that speaks for them to other teams. Teams reach each other
through those two voices, and a person hears a digest, not a stream of pings.
Check: AX-10.8 "No more than 1 notification per user per 15 minutes"

## Since you last looked

The Desk opens on what changed since you were last here: what landed, what failed, what waits on
you, and what was claimed without proof. Time is shown against your last visit.
Check: AX-10.9 "No "N ago" as a primary timestamp"

## No predictions

The product shows what happened and what a policy fixes, such as the next handover date. It
does not forecast when work will finish, and it shows no confidence figure it cannot back with a
record of how often it was right.
Check: AX-10.12 "No confidence number without its calibration history"
````

Every heading is a rule and every rule ends in exactly one `Check:` line, the citation form the
doc test reads. The page cites §10 by number and title so a reviewer can open `AGENTIC-UX.md` at
the same place; a renumbering in §10 fails the test rather than leaving a stale page.

#### Interfaces — complete: `desktop/src/components/work/MethodPage.tsx`

```tsx
import source from "../../../../docs/work/method.md?raw";           // Vite's raw import
import { makeTokenizer } from "../editor/markdown";                  // task 49 (task-49.md:246-256)

const html = makeTokenizer().render(source);                         // html: false, images off

/** The palette's "How we work". Read-only; there is nothing on the page to act on. */
export function MethodPage() {
  return <article className="method-page" dangerouslySetInnerHTML={{ __html: html }} />;
}
```

`dangerouslySetInnerHTML` is safe here for one reason, and the test holds it: task 49's
tokenizer sets `html: false`, so a raw tag in the file renders as text, and `validateLink` keeps
only `LIVE_LINK` targets. The source is a checked-in file, not user input.

**⚑ `commands.ts` (task 59):** one row, `{ id: "verb:how-we-work", label: "How we work", glyph:
GLYPH.verb, run: () => state.openView("method") }`, no `hint`. `openView` is task 59's name
(**UNVERIFIED**, as `task-186.md:220` records for *What's new*); the `verb:` prefix keeps the
existing block-order test whole (`task-186.md:224`).

#### Tests to write first — `desktop/src/work/recurring.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { makeIssueHead } from "../test/workFixtures";                 // task 20
import { dayWords, dutyLine, originLine, repeatWords } from "./recurring";

const PRIYA = { id: "u-priya", displayName: "Priya", email: "priya@example.test" };
const SAM = { id: "u-sam", displayName: "Sam", email: "sam@example.test" };
const weekly = { templateId: "tp-1", every: { n: 1, unit: "week" as const }, at: "09:00",
  tz: "Australia/Sydney", startsOn: "2026-09-28" };                   // a Monday

describe("recurring words", () => {
  // mutant killed: "weekday from the reader's zone" (new Date(date).getDay() is Sunday west of UTC)
  it("names the anchor's weekday from its own digits", () => {
    expect(repeatWords(weekly)).toBe("weekly, Monday 09:00");
    expect(repeatWords({ ...weekly, every: { n: 2, unit: "week" } })).toBe("every 2 weeks, Monday 09:00");
    expect(repeatWords({ ...weekly, every: { n: 1, unit: "month" }, startsOn: "2026-10-05" })).toBe("monthly, the 5th, 09:00");
  });
  // mutant killed: "a guessed day" (no anchor, the day is left out)
  it("leaves the day out when the schedule has no anchor", () => {
    const { startsOn: _drop, ...bare } = weekly;
    expect(repeatWords(bare)).toBe("weekly, 09:00");
  });
  // mutant killed: "system actor shown"
  it("a recurring issue names the person who set the schedule, never a system", () => {
    const head = makeIssueHead({ source: "recurring" as never, createdBy: PRIYA });
    const line = originLine(head, weekly);
    expect(line).toBe("from schedule · weekly, Monday 09:00 · set by Priya");
    expect(originLine(head, null)).toBe("from schedule · set by Priya");
    expect(line).not.toMatch(/system|bot|janitor/i);
    expect(originLine(makeIssueHead({ source: "person" }), weekly)).toBeNull();
  });
  it("draws the Hub's duty and date verbatim", () => {
    const read = { rotation: { teamId: "tm-1", members: [PRIYA.id, SAM.id], periodDays: 7,
      startsOn: "2026-09-01" }, onDuty: PRIYA, handoverOn: "2026-10-06", setBy: SAM };
    expect(dutyLine(read)).toBe("On triage duty: Priya, until Tue 6 Oct · rotation set by Sam");
    expect(dayWords("2026-10-06")).toBe("Tue 6 Oct");
  });
});
```

#### Tests to write first — `desktop/src/components/plan/RepeatField.test.tsx`

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { RepeatField } from "./RepeatField";

const props = (over = {}) => ({
  templateId: "tp-1", value: null, canEdit: true, timeZone: "Australia/Sydney",
  set: vi.fn(async (_t: string, s: never) => s), clear: vi.fn(async () => {}), ...over,
});

describe("RepeatField", () => {
  // mutant killed: "a fixed offset"
  it("sends the person's IANA zone as tz, never an offset", async () => {
    const p = props();
    render(<RepeatField {...p} />);
    fireEvent.click(screen.getByLabelText("Repeats"));
    fireEvent.click(screen.getByText("Save"));
    await vi.waitFor(() => expect(p.set).toHaveBeenCalled());
    const sent = p.set.mock.calls[0][1] as { tz: string; every: unknown; at: string };
    expect(sent).toEqual({ templateId: "tp-1", every: { n: 1, unit: "week" }, at: "09:00", tz: "Australia/Sydney" });
    expect(sent.tz).not.toMatch(/^(UTC)?[+-]\d/);
  });
  // mutant killed: "zone re-read per save" (the zone is read once, at mount)
  it("reads the zone once when no prop pins it", () => {
    const spy = vi.spyOn(Intl.DateTimeFormat.prototype, "resolvedOptions");
    const { rerender } = render(<RepeatField {...props({ timeZone: undefined })} />);
    rerender(<RepeatField {...props({ timeZone: undefined })} />);
    expect(spy).toHaveBeenCalledTimes(1);
  });
  // mutant killed: "members edit schedules"
  it("a member sees the words and no control", () => {
    const { container } = render(<RepeatField {...props({ canEdit: false,
      value: { templateId: "tp-1", every: { n: 1, unit: "week" }, at: "09:00", tz: "Australia/Sydney" } })} />);
    expect(screen.getByText("Repeats weekly, 09:00")).toBeTruthy();
    expect(screen.queryByRole("button")).toBeNull();
    expectNoPrediction(container);
  });
  // mutant killed: "clearing sends an empty schedule"
  it("turning Repeat off clears the schedule", async () => {
    const p = props({ value: { templateId: "tp-1", every: { n: 1, unit: "day" }, at: "08:00", tz: "Australia/Sydney" } });
    render(<RepeatField {...p} />);
    fireEvent.click(screen.getByLabelText("Repeats"));
    fireEvent.click(screen.getByText("Save"));
    await vi.waitFor(() => expect(p.clear).toHaveBeenCalledWith("tp-1"));
    expect(p.set).not.toHaveBeenCalled();
  });
});
```

#### Tests to write first — `TriageRotationSettings.test.tsx`, the duty line, the method page

```tsx
// desktop/src/components/plan/TriageRotationSettings.test.tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";

let admin = true;
vi.mock("../../work/access/projectAccess", () => ({ useIsTeamAdmin: () => admin }));
const { TriageRotationSettings } = await import("./TriageRotationSettings");

const PRIYA = { id: "u-priya", displayName: "Priya", email: "p@example.test" };
const SAM = { id: "u-sam", displayName: "Sam", email: "s@example.test" };
const base = () => ({ teamId: "tm-1", members: [PRIYA, SAM], read: null, today: "2026-09-26",
  set: vi.fn(async (_t: string, r: never) => ({ rotation: r, onDuty: SAM, handoverOn: "2026-10-03" })) });

describe("TriageRotationSettings", () => {
  beforeEach(() => { admin = true; });
  // mutant killed: "members edit duty"
  it("renders nothing for a member", () => {
    admin = false;
    const { container } = render(<TriageRotationSettings {...base()} />);
    expect(container.innerHTML).toBe("");
  });
  // mutant killed: "order by name" (the Hub keeps the ticked order)
  it("sends members in the order they were ticked", async () => {
    const p = base();
    const { container } = render(<TriageRotationSettings {...p} />);
    fireEvent.click(screen.getByLabelText("Sam"));
    fireEvent.click(screen.getByLabelText("Priya"));
    fireEvent.click(screen.getByText("Save"));
    await vi.waitFor(() => expect(p.set).toHaveBeenCalledWith("tm-1",
      { teamId: "tm-1", members: ["u-sam", "u-priya"], periodDays: 7, startsOn: "2026-09-26" }));
    expectNoPrediction(container);
  });
});
```

**The duty line** — two rows added to task 42's `TriageView.test.tsx`, over its fake client:

| Row | Arrange | Assert | Mutant killed |
|---|---|---|---|
| *draws the Hub's onDuty, not the client's* | `vi.setSystemTime("2026-10-09T00:00:00Z")`, a date on which `startsOn 2026-09-01`, `periodDays 7`, `[Priya, Sam]` gives Sam; the fake's `triageRotation` returns `onDuty: Priya`, `handoverOn: "2026-10-06"` | the text reads `On triage duty: Priya, until Tue 6 Oct`; `Sam` is absent | "client derivation" |
| *draws no line on 404* | the fake's `triageRotation` rejects with `rotation_not_found` | no element with class `duty-line`; the list still renders | "a guessed name" |

```tsx
// desktop/src/components/work/MethodPage.test.tsx
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { MethodPage } from "./MethodPage";

describe("MethodPage", () => {
  // mutant killed: "html: true" (a raw tag in the file would become live DOM)
  it("renders the six rules as headings and no live markup", () => {
    const { container } = render(<MethodPage />);
    expect([...container.querySelectorAll("h2")].map((h) => h.textContent)).toHaveLength(6);
    expect(container.querySelector("script, iframe, img, [onclick]")).toBeNull();
    expectNoPrediction(container);
  });
});

// desktop/src/components/work/method.doc.test.ts
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const root = resolve(__dirname, "../../../..");                     // work → components → src → desktop → repo
const ax = readFileSync(resolve(root, "docs/AGENTIC-UX.md"), "utf8");
const method = readFileSync(resolve(root, "docs/work/method.md"), "utf8");
const section10 = ax.slice(ax.indexOf("## 10. Anti-patterns")).split(/\n## /)[0];
const titles = new Map([...section10.matchAll(/^(\d+)\. \*\*(.+?)\*\*/gm)]
  .map((m) => [Number(m[1]), m[2].replace(/\.$/, "")]));

describe("the method page's citations", () => {
  // mutant killed: "stale numbering"
  it("every cited anti-pattern exists in AGENTIC-UX §10 with the same title", () => {
    const cites = [...method.matchAll(/^Check: AX-10\.(\d+) "(.*)"$/gm)];
    expect(cites).toHaveLength(6);                                    // one per rule
    for (const [, n, title] of cites) expect(titles.get(Number(n)), `AX-10.${n}`).toBe(title);
  });
  it("every rule ends in exactly one check", () => {
    const rules = method.split(/\n## /).slice(1);
    for (const r of rules) expect(r.match(/^Check: /gm)?.length, r.split("\n")[0]).toBe(1);
  });
});
```

The doc test reads `AGENTIC-UX.md` as it is on the tree, so the four amendments (Q13), when they
land in §10, are checked by the same test with no edit here; if one renumbers an item this page
cites, the test fails and names it.

#### Steps

1. Write `recurring.test.ts`; it fails. Write `recurring.ts`; it passes.
2. Write `RepeatField.test.tsx`; it fails. Write `RepeatField.tsx`, then `RecurringIssues.tsx` in
   team settings beside task 84's `TeamStates.tsx` (gap 1 names the host).
3. Write `TriageRotationSettings.test.tsx`; it fails. Write the component; mount it in team
   settings below *Recurring issues*.
4. Add the two duty-line rows to task 42's `TriageView.test.tsx`; they fail. Add `DutyLine` and
   `useTriageRotation` to ⚑ `TriageView.tsx`; they pass.
5. Add `originLine` to ⚑ `IssueRow.tsx` and ⚑ `IssueHead.tsx`; run tasks 40's and 46's tests
   unchanged (a non-recurring issue draws no new line).
6. Write `docs/work/method.md`, `method.doc.test.ts` and `MethodPage.test.tsx`; write
   `MethodPage.tsx`; add the palette row to ⚑ `commands.ts`. `desktop/vite.config.ts` sets no
   `server.fs.allow` today (`:16` opens a `server` block with none); if `npm run dev` refuses the
   `?raw` import from outside `desktop/`, add the repo root to `server.fs.allow` and say so in the
   commit.
7. Run the acceptance; commit.

#### Acceptance

```bash
cd desktop && npx vitest run src/work/recurring.test.ts src/components/plan src/components/work/TriageView.test.tsx src/components/work/MethodPage.test.tsx src/components/work/method.doc.test.ts src/components/palette
# expect: recurring 4 passed; RepeatField 4 passed; TriageRotationSettings 2 passed; TriageView's
#         rows plus 2 new passed; MethodPage 1 passed; method.doc 2 passed; palette's tests pass
cd desktop && npx tsc --noEmit
# expect: exit 0 (gap 4's "recurring" source is in task 37's union, or this fails at originLine)
cd desktop && grep -n -E "Date\.now|new Date\(\)" src/work/recurring.ts src/components/plan/RepeatField.tsx src/components/plan/TriageRotationSettings.tsx
# expect: no output (decision 2: no client clock in duty or schedule words)
grep -n -i "linear" docs/work/method.md
# expect: no output (clean room, rule 4)
```

#### What this task does NOT claim

- PagerDuty or Opsgenie duty (task 184), or duty in more than one zone per rotation.
- That the page is read, or that a team works this way: the page states the product's rules and
  the checks; it measures nothing about people.
- That the first recurring issue appears at a stated time. The Hub creates it at the next
  occurrence (task 121, decision 3); this task shows no date for it.
- A read of a template's current schedule (gap 1) or of who set a rotation (gap 3).
- The update-reminder policy's control. Task 127 draws the reminder row (`task-127.md:40`); which
  part sets `PUT /v1/reminder-policies` from the desktop is **UNVERIFIED** (gap 7).

#### Gaps found while writing this part

1. **No part for task 89 or 83.** The template editor that hosts *Repeat* and the template list
   are theirs; this part adds `RecurringIssues.tsx` as the host and reads a `schedule` member on
   each template, which no written type carries. There is also no `GET` for a template's
   schedule in task 121's routes (`task-121.md:306-310`), so after a reload the field cannot
   show what is set unless task 83's template read includes it.
2. **`TemplateSchedule.startsOn`** (task 121's G1) is not in task 125's type
   (`task-125.md:146-150`). Without it the words say *"weekly, 09:00"*, not *"weekly, Monday
   09:00"*; `AnchoredSchedule` carries it optionally until 125 adds it. `TriageRotation.tz` (the
   same G1) is also absent from 125.
3. **The rotation read names no setter.** The scope's *"rotation set by Sam"* needs a `setBy:
   User` on `TriageRotationView` (tasks 114 and 125). The scope's *"until Tue 09:00"* is an hour;
   task 121 makes boundaries local midnights, so this part names a date (decision 3).
4. **`IssueSource` has no `"recurring"`.** Task 37's union is `"person" | "orchestrator" |
   "intake"` (`task-37.md:97`); task 121 writes `source: "recurring"`. Task 37 (or 125) must add it.
5. **`handoverOn` vs `nextHandoverOn`.** Task 125 names the field `handoverOn`
   (`task-125.md:154`); task 114's wire struct says `nextHandoverOn` (`task-114.md:285`). One
   of them must change before 125's golden check can pass.
6. **No part for task 59.** `openView` and the palette's host are its scope's (**UNVERIFIED**).
7. **No desktop control sets a reminder policy** in the parts read here; task 121 serves the
   route and task 127 draws the reminder. The skeleton should name its owner.
