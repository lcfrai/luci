### Task 127: initiatives and the update composer

W4, stream WD, about 3 hours (an estimate, not a measurement). The desktop half of tasks 118
(initiatives), 119 (status updates) and 121's reminder. Depends on task 125 (`Initiative`,
`InitiativeView`, `InitiativeFeature`, `StatusUpdate`, `PostUpdateRequest`, `FeaturePin`,
`HEALTH_WORDS`, the `update.post` op with no inverse, the replica's `updatesOf` and `signals`,
`task-125.md`). Reads task 45's `PlanSurface`, `PLAN_SECTIONS` and `Convergence`
(`task-45.md:72-135`), task 49's `Editor` and `MarkdownDoc` (`task-49.md:516-537`, one editor),
task 62's `registerDeskRow` and `registerDeskSource` (scope, `…-scopes/tasks-61-80.md:72-99`, SW.17;
**no part on this tree**), task 87's `ScopeLog` (scope only, **UNVERIFIED**), task 83's `Feature`
(its `branchRef`, **UNVERIFIED**), task 126's `roadmapHarness` and task 20's `expectNoPrediction`.

Closes **L-76** and **L-87** with 119, **L-83**, **L-84**, **L-85** and **L-88** with 118, and
**L-77** with 121 (the digest itself is task 81's).

**What the reader sees** (05 §3.2 *status update*, §4.1 Plan, §5.4 W4; 04 §2.5; AX §5; AX
Principle 7):

- **Plan gains an *Initiatives* section**, after *My issues*: one row per initiative of the team,
  its name, its state word, its owner and *"target 14 Oct"* when a person set one. No row shows a
  score, a bar or a share.
- **The initiative view** lists one row per feature it may show (task 118's rule, only the caller's
  readable features): the name in sans with its branch in mono under it (§4.1), the lead, and the
  feature's **latest health word and its date**, *"At risk · 22 Sep"*, or *"No update yet"*. Beside
  it, the feature's criteria as convergence words with plain counts, *"settled 3 · open 2"* (a group
  size under Q1 A, never "3 of 5"). Below the rows, task 87's `ScopeLog`, reused, for what was added
  and removed and by whom. **There is no aggregate health, score or bar anywhere** (L-84); the graph
  Linear draws is replaced by that convergence and the scope log (L-85).
- **The owner** of the initiative edits the teams it spans in the header (*Teams: Checkout,
  Payments*, then *Edit teams*), through `SetInitiativeTeams`.
- **The composer**, for a feature or an initiative (L-76, L-87): the health word is a three-way
  choice with **none preselected**; the body is the one editor, `text·H`; the brief revision is
  pinned **when the composer opens**, *"Reports on `brief:bf-7k2@6`"* in mono, and if the brief
  moves while the person writes, the composer says *"The brief moved to @7 while you wrote"* and
  offers *Repin*; *Cite* adds keys of the feature's recorded evidence and landed issues as chips;
  the line beside *Post* says *"An update cannot be edited once posted"*.
- **The card** shows the word as text beside its glyph shape (never colour alone), the author, the
  time, the pinned key and each cite as a link (AX Principle 7). It has no edit control; a
  correction is a new update.
- **The reminder row** in the Desk's NEEDS YOU reads *"Update missing, 9 days · EP resumable
  uploads"* and opens the composer **in place**, under the row, never navigating (AX §5).

**Decisions made here, so nobody re-derives them:**

1. **Post needs a person's word and a person's words.** *Post* is disabled until one health word
   is picked and the body is not empty. The request is built only from the person's picks: there
   is no default for `health`, and `pinnedBrief` is always sent explicitly (task 119 would default
   it; the composer never relies on that, so what the card says is what the person saw).
2. **The pin is taken at open, not at post.** The composer reads the feature's brief head once when
   it opens and keeps it. A later head (the replica's brief kind for that brief) raises the
   *moved* line; only *Repin* changes the pin. For an initiative, the composer takes one pin per
   feature row it lists and sends them as `featurePins` (task 119's G1, `task-119.md:147-170`).
3. **The latest word is the feature's own**, from `InitiativeFeature.latestUpdate`: the view never
   computes a word, and `null` reads *"No update yet"*.
4. **Convergence counts are group sizes over task 45's five words**, from
   `InitiativeFeature.criteria[].word`. A word the desktop does not know (task 125's gap 3) is
   counted under its own name, never folded into another.
5. **Reminders are held in memory by a Desk source**, fed by the replica's `signals` (task 125,
   which stores them nowhere). One reminder per subject, the newest `daysSince` wins; posting an
   update for that subject clears it at once. Nothing is written to disk.

**Files** (the scope names these; `reminders.ts` is the in-memory source decision 5 needs, a gap):
- `desktop/src/components/roadmap/HealthWord.tsx` — the word and its glyph (new)
- `desktop/src/components/roadmap/InitiativeList.tsx` — Plan's section (new)
- `desktop/src/components/roadmap/InitiativeView.tsx` — the view (new)
- `desktop/src/components/roadmap/UpdateComposer.tsx` — the composer (new)
- `desktop/src/components/roadmap/UpdateCard.tsx` — the card (new)
- `desktop/src/components/roadmap/reminders.ts` — the in-memory reminder source (new)
- `desktop/src/components/roadmap/roadmap.css` — their styles (new)
- `desktop/src/components/roadmap/roadmap.test.tsx` — the tests below (new)
- `desktop/src/components/work/PlanSurface.tsx` ⚑ (task 45) — the *Initiatives* section (modified)
- `desktop/src/components/desk/deskRows.tsx` ⚑ (task 62) — registers `update.reminder` (modified)

#### Interfaces — complete: `HealthWord.tsx`

```tsx
// desktop/src/components/roadmap/HealthWord.tsx — a person's word, drawn as text plus a shape.
import type { HealthWord as Health } from "../../api/work";

export const HEALTH_LABEL: Readonly<Record<Health, string>> = {
  on_track: "On track", at_risk: "At risk", off_track: "Off track",
};

/** Three shapes, so the word never rests on colour (AX Principle 7; forced-colors keeps shapes). */
function Shape({ h }: { h: Health }): JSX.Element {
  const d = { on_track: "M8 2a6 6 0 1 1 0 12A6 6 0 0 1 8 2z",   // a disc
              at_risk: "M8 2l6.5 11.5h-13z",                      // a triangle
              off_track: "M3 3h10v10H3z" }[h];                    // a square
  return (
    <svg className="hw-shape" viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
      <path d={d} fill="currentColor" />
    </svg>
  );
}

/** "At risk" with its shape; the date, when given, as "At risk · 22 Sep" (mono date). */
export function HealthWord({ h, at, timeZone }: { h: Health; at?: string; timeZone?: string }): JSX.Element {
  const day = at ? new Intl.DateTimeFormat("en-AU", { day: "numeric", month: "short", timeZone }).format(new Date(at)) : null;
  return (
    <span className="hw" data-health={h}>
      <Shape h={h} />
      <span className="hw-word">{HEALTH_LABEL[h]}</span>
      {day && <><span className="hw-sep"> · </span><time className="hw-at" dateTime={at}>{day}</time></>}
    </span>
  );
}
```

#### Interfaces — complete: `UpdateCard.tsx`

```tsx
// desktop/src/components/roadmap/UpdateCard.tsx — one posted update. No edit control, ever.
import type { StatusUpdate } from "../../api/work";
import { HealthWord } from "./HealthWord";
import { KeyLink } from "../common/KeyLink";      // task 46's key chip that opens its target (UNVERIFIED)
import { Editor } from "../editor/Editor";         // task 49, read-only here
import { docFromAuthored } from "../editor/authorClass";   // task 49 (task-49.md:119-124)

export function UpdateCard({ u, timeZone }: { u: StatusUpdate; timeZone: string }): JSX.Element {
  const erased = u.erasedAt !== null;
  return (
    <article className="uc" aria-label={`Update by ${u.author.name}`}>
      <header className="uc-head">
        <HealthWord h={u.health} />
        <span className="uc-by">{u.author.name}</span>
        <time className="uc-at" dateTime={u.at}>
          {new Intl.DateTimeFormat("en-AU", { dateStyle: "medium", timeStyle: "short", timeZone }).format(new Date(u.at))}
        </time>
      </header>
      {erased
        ? <p className="uc-erased">Erased. The word, the pin and the cites stay.</p>
        : <Editor value={docFromAuthored(u.body)} onCommit={() => {}} readOnly ariaLabel="Update body" />}
      <p className="uc-pin">Reports on <KeyLink k={u.pinnedBrief} /></p>
      {u.featurePins.length > 1 && (
        <ul className="uc-pins" aria-label="Pinned briefs">
          {u.featurePins.map((fp) => <li key={fp.featureId}><KeyLink k={fp.pinnedBrief} /></li>)}
        </ul>
      )}
      {u.cites.length > 0 && (
        <ul className="uc-cites" aria-label="Cites">
          {u.cites.map((k) => <li key={k}><KeyLink k={k} /></li>)}
        </ul>
      )}
    </article>
  );
}
```

`KeyLink` is the key chip that opens what a key names through `/v1/refs` (task 46's issue view
renders keys this way; **UNVERIFIED** component name, the merged one is taken at build). Every
pin and cite is a link, so no claim on the card is without its span (AX Principle 7). `Editor` in
`readOnly` renders the body through the one editor, so a posted update looks as it did when
written; `docFromAuthored` is task 49's load from the wire (`task-49.md:119-124`).

#### Interfaces — complete: `UpdateComposer.tsx`

```tsx
// desktop/src/components/roadmap/UpdateComposer.tsx — L-76, L-87. A person's word, pinned at open.
import { useState } from "react";
import { HEALTH_WORDS, type FeaturePin, type HealthWord as Health, type SubjectRef } from "../../api/work";
import { mutate, useWork } from "../../work/replica/useWork";
import { Editor } from "../editor/Editor";
import { fieldAuthor, toAuthored, type MarkdownDoc } from "../editor/authorClass";
import { HEALTH_LABEL, HealthWord } from "./HealthWord";
import { KeyLink } from "../common/KeyLink";
import { citeCandidates, briefHeadKey } from "./roadmapSelectors";

export interface UpdateComposerProps {
  subject: SubjectRef;
  /** The features the update reports on: one for a feature, the view's rows for an initiative. */
  featureIds: readonly string[];
  onPosted(): void;          // after mutate; the caller closes the composer in place
  onCancel(): void;          // nothing sent
}

const EMPTY: MarkdownDoc = { markdown: "", authorship: [] };

export function UpdateComposer(p: UpdateComposerProps): JSX.Element {
  // Decision 2: the pins are read once, at open (a lazy initial state runs once), and kept.
  const heads = useWork((r) => p.featureIds.map((f) => briefHeadKey(r, f)));   // live, for the moved line
  const [pins, setPins] = useState<FeaturePin[]>(
    () => p.featureIds.map((f, i) => ({ featureId: f, pinnedBrief: heads[i] })));
  const moved = pins.filter((pin, i) => heads[i] !== pin.pinnedBrief);
  const [health, setHealth] = useState<Health | null>(null);                   // decision 1: none
  const [body, setBody] = useState<MarkdownDoc>(EMPTY);
  const [cites, setCites] = useState<string[]>([]);
  const candidates = useWork((r) => citeCandidates(r, p.featureIds));
  const agentWords = fieldAuthor(body.authorship) === "A";
  const canPost = health !== null && body.markdown.trim() !== "" && !agentWords;

  const post = () => {
    if (!canPost || health === null) return;
    const isFeature = p.subject.kind === "feature";
    mutate({ kind: "update.post", subject: p.subject, req: {
      health, body: toAuthored(body), cites,
      pinnedBrief: isFeature ? pins[0].pinnedBrief : undefined,
      featurePins: isFeature ? [] : pins,
    } });
    p.onPosted();
  };

  return (
    <form className="ucmp" aria-label="New update" onSubmit={(e) => { e.preventDefault(); post(); }}>
      <fieldset className="ucmp-health">
        <legend>Health</legend>
        {HEALTH_WORDS.map((h) => (
          <label key={h} className="ucmp-opt">
            <input type="radio" name="health" value={h} checked={health === h} onChange={() => setHealth(h)} />
            <HealthWord h={h} />
          </label>
        ))}
      </fieldset>
      <Editor value={body} onCommit={setBody} ariaLabel="Update body" />
      <div className="ucmp-pins">
        {pins.map((pin) => <p key={pin.featureId} className="ucmp-pin">Reports on <KeyLink k={pin.pinnedBrief} /></p>)}
        {moved.length > 0 && (
          <p className="ucmp-moved">
            The brief moved to @{heads[pins.indexOf(moved[0])]?.split("@")[1]} while you wrote.{" "}
            <button type="button" onClick={() => setPins(pins.map((pin, i) => ({ ...pin, pinnedBrief: heads[i] })))}>Repin</button>
          </p>
        )}
      </div>
      <div className="ucmp-cites">
        {cites.map((k) => (
          <span key={k} className="ucmp-chip"><KeyLink k={k} />
            <button type="button" aria-label={`Remove ${k}`} onClick={() => setCites(cites.filter((c) => c !== k))}>×</button></span>
        ))}
        <select aria-label="Cite" value="" onChange={(e) => e.target.value && setCites([...cites, e.target.value])}>
          <option value="">Cite</option>
          {candidates.filter((c) => !cites.includes(c.key)).map((c) => <option key={c.key} value={c.key}>{c.label}</option>)}
        </select>
      </div>
      {agentWords && <p className="ucmp-agent">Drafted words are marked. Edit them before posting.</p>}
      <div className="ucmp-actions">
        <button type="submit" disabled={!canPost}>Post</button>
        <span className="ucmp-final">An update cannot be edited once posted</span>
        <button type="button" onClick={p.onCancel}>Cancel</button>
      </div>
    </form>
  );
}
```

`roadmapSelectors.ts` (new, this task) holds two pure selectors over the replica:
`briefHeadKey(r, featureId)` → `"brief:<briefId>@<head revision>"` from task 83's `Feature`
(its branch brief id, **UNVERIFIED** field name) and H's brief head as the replica holds it; and
`citeCandidates(r, featureIds)` → `{key, label}[]`, the feature's issues in `landed`, `done` or
`shipped` as `issue:EP-142` keys plus each one's recorded evidence ids from task 10's proof on its
state moves, never a key from a project the caller cannot read. `HEALTH_LABEL` is used by the
test. The agent-words line is the hook for task 139's drafts: a `text·A` body cannot be posted,
matching task 119's refusal of agent text.

#### Interfaces — complete: `InitiativeView.tsx`

```tsx
// desktop/src/components/roadmap/InitiativeView.tsx — task 118's read. No aggregate, anywhere.
import { useEffect, useState } from "react";
import type { InitiativeFeature, InitiativeView as View } from "../../api/work";
import { useWorkClient } from "../../api/useWorkClient";      // task 37's client from the store
import { useWork, mutate } from "../../work/replica/useWork";
import { ViewHeader, type Route } from "../common/ViewHeader";
import { HealthWord } from "./HealthWord";
import { UpdateComposer } from "./UpdateComposer";
import { UpdateCard } from "./UpdateCard";
import { ScopeLog } from "../plan/ScopeLog";                   // task 87, reused (L-85)

/** "settled 3 · open 2": group sizes in the five words' order, then any unknown word by name. */
export function convergenceCounts(f: InitiativeFeature): string {
  const order = ["settled", "open", "contested", "drifting", "uncovered"];
  const n = new Map<string, number>();
  for (const c of f.criteria) n.set(c.word, (n.get(c.word) ?? 0) + 1);
  const words = [...order.filter((w) => n.has(w)), ...[...n.keys()].filter((w) => !order.includes(w))];
  return words.map((w) => `${w} ${n.get(w)}`).join(" · ");
}

export function InitiativeView(p: { initiativeId: string; route: Route | null; timeZone: string; meId: string }): JSX.Element {
  const work = useWorkClient();
  const [view, setView] = useState<View | null>(null);
  const [composing, setComposing] = useState(false);
  const seq = useWork((r) => r.lastSeqOf(p.initiativeId));    // re-read when its events arrive
  useEffect(() => { void work.initiative(p.initiativeId).then(setView); }, [work, p.initiativeId, seq]);
  const updates = useWork((r) => r.updatesOf({ kind: "initiative", id: p.initiativeId }));
  const branchOf = useWork((r) => (id: string) => r.feature(id)?.branchRef ?? null);
  if (view === null) return <div className="iv" aria-busy="true" />;
  const { initiative: ini } = view;
  const isOwner = ini.owner.id === p.meId;

  return (
    <div className="iv">
      <ViewHeader title={ini.name} route={p.route} actions={
        <button type="button" onClick={() => setComposing(true)}>Post update</button>} />
      <p className="iv-facts">
        {ini.state} · owner {ini.owner.name}
        {ini.targetDate && <> · target <time dateTime={ini.targetDate}>{ini.targetDate}</time></>}
        {" · "}Teams: {view.teams.join(", ")}
        {isOwner && <button type="button" className="iv-teams" onClick={() => {/* task 126's team picker, in place */}}>Edit teams</button>}
      </p>
      {composing && <UpdateComposer subject={{ kind: "initiative", id: ini.id }}
        featureIds={view.features.map((f) => f.featureId)}
        onPosted={() => setComposing(false)} onCancel={() => setComposing(false)} />}
      <ul className="iv-rows" aria-label="Features">
        {view.features.map((f) => (
          <li key={f.featureId} className="iv-row">
            <span className="iv-name">{f.name}</span>
            <span className="iv-branch">{branchOf(f.featureId)}</span>
            <span className="iv-lead">{f.lead.name}</span>
            <span className="iv-latest">
              {f.latestUpdate ? <HealthWord h={f.latestUpdate.health} at={f.latestUpdate.at} timeZone={p.timeZone} />
                              : "No update yet"}
            </span>
            <span className="iv-conv">{convergenceCounts(f)}</span>
          </li>
        ))}
      </ul>
      <ScopeLog subject={{ kind: "initiative", id: ini.id }} />
      <section aria-label="Updates">{updates.map((u) => <UpdateCard key={u.id} u={u} timeZone={p.timeZone} />)}</section>
    </div>
  );
}
```

`InitiativeView` is fetched on open and on each event for that initiative (it is a server-built
read, not a replica record); it is held in component state only. `r.lastSeqOf(id)` and
`r.feature(id)` are replica selectors (task 55's and task 83's, **UNVERIFIED** names). *Edit
teams* opens task 126's team picker in place and sends `setInitiativeTeams` with the owning team
first. `ScopeLog`'s `subject` prop is this task's ask of task 87 (its scope draws a cycle's scope
log; an initiative's is the union of its features' logs, a gap for 87). `mutate` is imported for
the teams edit (`initiative.teams`, task 125's op).

#### Interfaces — complete: `InitiativeList.tsx` and the ⚑ `PlanSurface.tsx` delta

```tsx
// desktop/src/components/roadmap/InitiativeList.tsx — Plan's Initiatives section (05 §4.1).
import { useWork } from "../../work/replica/useWork";

export function InitiativeList(p: { teamId: string; onOpen(initiativeId: string): void }): JSX.Element {
  const rows = useWork((r) => [...r.roadmap().initiatives.values()].filter((i) => i.teamId === p.teamId)
    .sort((a, b) => a.name.localeCompare(b.name)));
  if (rows.length === 0) return <p className="il-none">No initiatives yet</p>;
  return (
    <ul className="il" aria-label="Initiatives">
      {rows.map((i) => (
        <li key={i.id}><button type="button" className="il-row" onClick={() => p.onOpen(i.id)}>
          <span className="il-name">{i.name}</span>
          <span className="il-state">{i.state}</span>
          <span className="il-owner">{i.owner.name}</span>
          {i.targetDate && <span className="il-target">target <time dateTime={i.targetDate}>{i.targetDate}</time></span>}
        </button></li>
      ))}
    </ul>
  );
}

// desktop/src/components/work/PlanSurface.tsx (task 45) — PLAN_SECTIONS gains its fifth, last:
export const PLAN_SECTIONS = ["Triage", "Backlog", "Criteria", "My issues", "Initiatives"] as const;
// … after the "My issues" section:
<section className="wk-section" aria-labelledby="plan-initiatives"><h2 id="plan-initiatives">Initiatives</h2>
  <InitiativeList teamId={teamId} onOpen={openInitiative} /></section>
```

Task 45's test that pins `PLAN_SECTIONS` changes by the one appended name. `teamId` is the
project's team (task 45 has `projectId`; the team is the replica's `ProjectScope.teamId`) and
`openInitiative` sets task 45's `workSlice.surface` to the initiative view (a gap: task 45's
`surface` has no initiative value).

#### Interfaces — complete: `reminders.ts` and the ⚑ `deskRows.tsx` delta (SW.17)

```ts
// desktop/src/components/roadmap/reminders.ts — decision 5. Memory only; nothing on disk.
import type { SubjectRef } from "../../api/work";

/** One reminder per subject: task 121's update.reminder {subject, lead, daysSince}. */
export interface Reminder { subject: SubjectRef; lead: string; daysSince: number; arrivedAt: string; seq: number }

const bySubject = new Map<string, Reminder>();
const key = (s: SubjectRef) => `${s.kind}:${s.id}`;

/** Fed by the replica's signals (task 125). The newest daysSince for a subject wins. */
export function onReminderEvent(e: { seq: number; at: string; kind: string; data: Record<string, unknown> }): void {
  if (e.kind !== "update.reminder") return;
  const subject = e.data.subject as SubjectRef;
  const r: Reminder = { subject, lead: String(e.data.lead), daysSince: Number(e.data.daysSince), arrivedAt: e.at, seq: e.seq };
  const old = bySubject.get(key(subject));
  if (!old || old.seq < r.seq) bySubject.set(key(subject), r);
}

/** A posted update clears its subject's reminder at once (the composer's onDone path). */
export function clearReminder(subject: SubjectRef): void { bySubject.delete(key(subject)); }

export const reminders = (): readonly Reminder[] => [...bySubject.values()];

/** "Update missing, 9 days · EP resumable uploads" */
export const reminderLine = (daysSince: number, subjectName: string): string =>
  `Update missing, ${daysSince} ${daysSince === 1 ? "day" : "days"} · ${subjectName}`;
```

```tsx
// desktop/src/components/desk/deskRows.tsx (task 62) — task 127's registration.
import { useState } from "react";
import { registerDeskRow, registerDeskSource, type DeskItem } from "./deskFrame";   // SW.17
import { reminders, reminderLine, clearReminder, type Reminder } from "../roadmap/reminders";
import { UpdateComposer } from "../roadmap/UpdateComposer";
import { useSubjectName, useSubjectFeatureIds } from "../roadmap/roadmapSelectors";

registerDeskSource("update.reminder", () => reminders().map((r): DeskItem<Reminder> => ({
  kind: "update.reminder", id: `${r.subject.kind}:${r.subject.id}`, payload: r, arrivedAt: r.arrivedAt,
  rankReason: "An update was asked for", ifNothing: "The next reminder arrives next period",
})));

function ReminderRow({ item }: { item: DeskItem<Reminder> }): JSX.Element {
  const [open, setOpen] = useState(false);
  const name = useSubjectName(item.payload.subject);
  const featureIds = useSubjectFeatureIds(item.payload.subject);   // [id] for a feature; the rows for an initiative
  return (
    <div className="dr-reminder">
      <button type="button" className="dr-line" aria-expanded={open} onClick={() => setOpen(!open)}>
        {reminderLine(item.payload.daysSince, name)}
      </button>
      {open && <UpdateComposer subject={item.payload.subject}
        featureIds={featureIds}
        onPosted={() => { clearReminder(item.payload.subject); setOpen(false); }}
        onCancel={() => setOpen(false)} />}
    </div>
  );
}

registerDeskRow<Reminder>("update.reminder", "needs_you", (item) => <ReminderRow item={item} />,
  (item) => ({ tier: 2, waitingSinceMs: Date.parse(item.arrivedAt) }));
```

The row opens the composer **in its own place** (`open` state inside the row, no route change),
which is AX §5's rule. Only a post clears the reminder; *Cancel* closes the composer and keeps
it. An initiative reminder opens the composer with the initiative's feature ids
(`useSubjectFeatureIds`, from the replica's initiative and its features; both hooks are in
`roadmapSelectors.ts`). Tier 2: a missing update blocks nobody's agent and is reversible.
The Desk's own rule holds: arrivals do not re-rank while open (task 62).

```css
/* desktop/src/components/roadmap/roadmap.css */
.hw { display: inline-flex; align-items: center; gap: var(--s-1); font-family: var(--sans); font-size: var(--t-ui); }
.hw-shape { flex: none; color: var(--ink-2); }
.hw-at, .uc-at, .iv-branch, .ucmp-pin, .uc-pin { font-family: var(--mono); font-size: var(--t-fact);
  font-variant-numeric: tabular-nums; color: var(--ink-3); }
.uc { display: grid; gap: var(--s-2); padding: var(--s-3); border-radius: var(--r-card); box-shadow: var(--e-card); }
.uc-head { display: flex; gap: var(--s-2); align-items: baseline; }
.iv-rows { display: grid; gap: 0; }
.iv-row { display: grid; grid-template-columns: 2fr 1fr 1fr 1.4fr; grid-template-rows: auto auto;
  column-gap: var(--s-3); padding: var(--s-2) 0; min-height: var(--row-h); }
.iv-branch { grid-column: 1; grid-row: 2; }
.iv-conv, .il-state, .il-owner, .il-target, .iv-facts { font-family: var(--sans); font-size: var(--t-meta); color: var(--ink-3); }
.ucmp { display: grid; gap: var(--s-3); padding: var(--s-3); border-radius: var(--r-card); box-shadow: var(--e-raised); }
.ucmp-health { display: flex; gap: var(--s-3); border: 0; padding: 0; }
.ucmp-final, .ucmp-moved, .ucmp-agent { font-family: var(--sans); font-size: var(--t-meta); color: var(--ink-3); }
@media (forced-colors: active) { .hw-shape { color: CanvasText; } }
```

The health word takes **no hue**: the shape and the word carry it, so `forced-colors` loses
nothing and a colour-blind reader loses nothing (AX Principle 7; DESIGN-SYSTEM's no-status-by-
colour-alone). No transition anywhere in the file.

#### Tests to write first — `desktop/src/components/roadmap/roadmap.test.tsx`

Over task 126's `roadmapHarness` (the fake-Hub replica, `useMe`, `hub.writes()`), with project
`EP`, feature `ft-ep` (brief `bf-7k2` at head 6), initiative `in-1` spanning it and `ft-pay`.
Every test ends with `expectNoPrediction(container)` (SW.8).

```tsx
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { renderWith, hubView, feedEvent, briefMovesTo, deskWith } from "../../test/roadmapHarness";
import { UpdateComposer } from "./UpdateComposer";
import { UpdateCard } from "./UpdateCard";
import { InitiativeView } from "./InitiativeView";
import { statusUpdate } from "../../test/workFixtures";            // task 20, W4 rows added here

const composer = (extra = {}) => renderWith(<UpdateComposer subject={{ kind: "feature", id: "ft-ep" }}
  featureIds={["ft-ep"]} onPosted={() => {}} onCancel={() => {}} {...extra} />);

describe("task 127: initiatives and the update composer", () => {
  it("no health word is picked on open, and Post waits for one", async () => {
    const { container } = await composer();
    for (const r of screen.getAllByRole("radio")) expect((r as HTMLInputElement).checked).toBe(false);
    fireEvent.input(screen.getByLabelText("Update body"), { target: { textContent: "Uploads slip a week" } });
    expect(screen.getByRole("button", { name: "Post" })).toHaveProperty("disabled", true);  // kills "defaults to on track"
    fireEvent.click(screen.getByLabelText(/At risk/));
    expect(screen.getByRole("button", { name: "Post" })).toHaveProperty("disabled", false);
    expectNoPrediction(container);
  });

  it("Post sends the pin taken at open, even after the brief moves", async () => {
    const { container, hub } = await composer();
    expect(screen.getByText(/Reports on/).textContent).toContain("brief:bf-7k2@6");
    await briefMovesTo("bf-7k2", 7);
    expect(screen.getByText(/The brief moved to @7 while you wrote/)).toBeTruthy();
    fireEvent.click(screen.getByLabelText(/Off track/));
    fireEvent.input(screen.getByLabelText("Update body"), { target: { textContent: "Blocked on storage" } });
    fireEvent.click(screen.getByRole("button", { name: "Post" }));
    // kills "unpinned update" and "pin taken at post"
    expect(hub.writes()[0]).toMatchObject({ route: "PostFeatureUpdate", body: { pinnedBrief: "brief:bf-7k2@6", health: "off_track" } });
    expectNoPrediction(container);
  });

  it("a posted card has no edit control, and its word reads under forced-colors", async () => {
    const { container } = await renderWith(<UpdateCard u={statusUpdate({ health: "at_risk" })} timeZone="Australia/Sydney" />,
      { forcedColors: true });
    expect(within(container).queryByRole("button", { name: /edit/i })).toBeNull();          // kills "updates editable"
    expect(container.querySelector("[contenteditable=true]")).toBeNull();
    expect(screen.getByText("At risk")).toBeTruthy();                                      // kills "health by colour alone"
    expect(container.querySelector(".hw-shape")).not.toBeNull();
    expectNoPrediction(container);
  });

  it("a feature with latestUpdate null reads 'No update yet', and the view has no aggregate", async () => {
    hubView("in-1", { features: [{ featureId: "ft-ep", latestUpdate: null, criteria: [
      { criterionId: "c1", word: "settled" }, { criterionId: "c2", word: "settled" }, { criterionId: "c3", word: "open" }] }] });
    const { container } = await renderWith(<InitiativeView initiativeId="in-1" route={null} timeZone="Australia/Sydney" meId="u-a" />);
    expect(screen.getByText("No update yet")).toBeTruthy();                                // kills "a default word"
    expect(screen.getByText("settled 2 · open 1")).toBeTruthy();
    // kills "computed roll-up": no health word outside a feature row, no meter, no percent
    expect(container.querySelectorAll(".iv > .hw, [role=meter], [role=progressbar], progress")).toHaveLength(0);
    expectNoPrediction(container);
  });

  it("the reminder row opens the composer in place", async () => {
    const { container, router } = await deskWith([{ kind: "update.reminder", data: {
      subject: { kind: "feature", id: "ft-ep" }, lead: "u-a", daysSince: 9 } }]);
    const row = screen.getByRole("button", { name: "Update missing, 9 days · EP resumable uploads" });
    fireEvent.click(row);
    expect(router.navigations()).toEqual([]);                                             // kills "navigates away"
    expect(within(row.parentElement!).getByRole("form", { name: "New update" })).toBeTruthy();
    expectNoPrediction(container);
  });

  it("a cite chip opens its evidence", async () => {
    const { container, opened } = await composer();
    fireEvent.change(screen.getByLabelText("Cite"), { target: { value: "ev-1" } });
    fireEvent.click(within(container.querySelector(".ucmp-cites")!).getByRole("link", { name: /ev-1/ }));
    expect(opened()).toEqual(["ev-1"]);                                                   // kills "a claim with no span"
    expectNoPrediction(container);
  });
});
```

`briefMovesTo`, `hubView`, `deskWith` and `opened()` extend task 126's harness (the brief head
event, a canned `InitiativeView`, a Desk mounted with task 62's frame and the given signals, and
the keys `KeyLink` asked to open). `statusUpdate` is a task 20 fixture this task adds.
`expectNoPrediction` runs on every surface: the composer, the card, the view and the Desk row.

#### Steps

1. Add the harness helpers and the `statusUpdate` fixture; write the six tests. They fail: no
   `roadmap/` components.
2. Write `HealthWord.tsx` and `UpdateCard.tsx`; the card test passes.
3. Write `roadmapSelectors.ts` and `UpdateComposer.tsx`; the three composer tests pass.
4. Write `InitiativeView.tsx`, `InitiativeList.tsx` and the `PlanSurface.tsx` delta; update task
   45's `PLAN_SECTIONS` test by the one name; the view test passes.
5. Write `reminders.ts`, subscribe it to the replica's `signals` where task 62's Desk mounts, and
   add the `deskRows.tsx` registration; the reminder test passes.
6. `roadmap.css`; `npm --prefix desktop run typecheck`; commit with `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- src/components/roadmap/roadmap.test.tsx src/components/work/PlanSurface.test.tsx
# expect: roadmap.test.tsx 6 passed; PlanSurface.test.tsx as before, its sections test naming five; no failed
npm --prefix desktop run typecheck
# expect: exit 0
```

**Visual acceptance is the owner's eyes**: the card, the composer and the view are judged in the
running app against `prototype/work-layer.html`'s look, not by this report.

#### What this task does NOT claim

- Drafted updates (task 139): the composer refuses `text·A` bodies and draws the hook line only.
- That leads find the per-feature roll-up useful (05 §2.6): it is built, not proven.
- That the health word is honest: it is `text·H`, a person's claim, and the card says who made it.
- That a reminder survives a restart: reminders are in memory; after a restart the next period's
  event, or task 81's digest, brings it back.
- Any rendering speed for a large initiative (unmeasured).

#### Gaps found while writing this part

1. **The scope's file list** omits `reminders.ts` (decision 5) and `roadmapSelectors.ts` (the
   composer's pin and cite reads). The skeleton's File map should list both under task 127.
2. **Task 45's `surface`** has no value for an initiative view; opening one from Plan needs it (or
   a route). Task 45's part (`task-45.md:66`) owns `surface`.
3. **Task 87's `ScopeLog`** draws a cycle's scope; an initiative's is the union of its features'
   logs, which needs a `subject` prop (or a second component).
4. **Task 83's `Feature`** must carry `branchRef` and its branch brief id for the branch line and
   the pin; its part is not on this tree.
5. **Criterion words** (task 125's gap 3): the counts are only as right as the vocabulary the Hub
   sends in `InitiativeFeature.criteria[].word`.
