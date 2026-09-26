### Task 62: the Desk frame: five fixed sections, never live while open, the row-kind registry

Stream **WD2**, about 3 hours (an estimate, not a measurement). Defines seam **SW.17**
(`2026-09-25-phase-w-work-layer.md:390`). Depends on task 37 (the `Lens` edit and `workSlice`,
`task-37.md:530-624`). Uses task 40's `WorkReader` and `workSource()` (`task-40.md:140-170`), task
20's fixtures and `expectNoPrediction` (`task-20.md:126-131`, `:299-330`), task 53's `registerVerb`,
`isListKeyTarget` and the `focus.next`, `focus.prev` and `held.apply` rows (`task-53.md:168-177`,
`:225-253`), and the built store (`store/slices/uiSlice.ts:61`, `setLens`; `dataSlice.ts:31-34`).
Tasks 63, 64, 65, 71, 127, 133, 170 and team mode's TM-54 plug rows into this frame; none of them
builds a second Desk (`2026-09-25-phase-w-work-layer.md:100-101`; 05 §5.6, `05-design.md:1313-1321`).

After this task the Desk exists as a lens: five fixed sections and one band, in AX §5's order,
each with its header, a plain count and a one-line zero, filled by whatever row kinds are
registered, ranked once when the reader arrives, and still while the reader is on it.

**What the reader sees** (AX §5 *Rules for the Desk*, `docs/AGENTIC-UX.md:142-149`; 05 §4.2,
`05-design.md:930-960`; `prototype/work-layer.html:760-787`):

- **Headers in a fixed order**: NEEDS YOU, DECIDED WITHOUT YOU, FAILED, LANDED, RUNNING, then the
  band SINCE YOU LAST LOOKED. Each header carries its count in mono `tabular-nums`, the one place
  on the Desk a count is shown (05 §4.4 rule 3).
- **An empty section keeps its header** and one zero line: *"Nothing was decided for you."*,
  *"Nothing failed."*, *"Nothing of yours landed since you last looked."*, *"No agents of yours are
  running."* (the prototype's words, `:782-785`, and one new line for RUNNING). NEEDS YOU's zero is
  `DeskZero`, *"Nothing needs you."*, which task 65 fills with what W1 can back. FAILED is never
  hidden behind a calm state (07 D C5).
- **Nothing moves while the reader is on it.** When a row would arrive, the Desk shows one line
  above the sections, *"A new item arrived. The Desk does not re-rank while you read it."*, with a
  *Show* button marked `N`; `N` or the button re-ranks. Leaving the Desk and coming back re-ranks.
- **Within a section**, rows run by tier (irreversible first, then *blocks someone*, then the rest,
  AX §5's *"ranked first: irreversible"*), and within a tier the longest-waiting first.
- **`j`/`k` walk every row** from the top of NEEDS YOU to the end of RUNNING and the band, across
  section edges; the cursor row has the ring (DESIGN-SYSTEM's selection shape).
- **The horizon band renders nothing** in W1: the horizon bar has no slice (Q4, 05 §5.6).

**Decisions made here, so nobody re-derives them:**

1. **SW.17 is this file's `deskModel.ts`, and it holds no JSX.** The scope allowed `.ts`
   (`tasks-61-80.md:69-70`); the skeleton's file map says `deskModel.tsx` (`:311`, gap 1). Every
   row kind imports `registerDeskRow`, `registerDeskSource`, `DeskItem`, `DeskRank` and
   `DeskSnapshot` from `desktop/src/components/desk/deskModel`. Task 127 wrote `./deskFrame` and
   task 170 `../desk/registry` (`task-127.md:400`, `task-170.md:295`): both follow this path (gap 2).
2. **A kind is registered once, as a row and a source, and a second registration throws.** The
   error names the kind (*"desk row kind issue.urgent_mine is already registered"*), so TM-54 can
   never silently replace an issue row (`team…/task-54.md:35-36`). A row without a source, or a
   source without a row, is a load-time error on the first `buildDesk`, naming the kind.
3. **The snapshot is taken once, on entry.** `Desk` calls `takeDeskSnapshot(inputs)` in its first
   render (`useState` initialiser) and builds the model from it. While mounted it subscribes to the
   cockpit store and the work source only to **compare**: a fresh build whose item keys
   (`kind + ":" + id`) include one not on screen sets `held`; nothing on screen changes. A row that
   would leave stays until `N` or the next visit (task 63's test 5 relies on this). `N` rebuilds.
4. **Leaving re-ranks because the frame unmounts.** `LensRouter` renders one arm, so switching lens
   unmounts `Desk` and drops its snapshot. On unmount the frame writes the time into
   `markDeskVisit(ms)`, an in-memory preference that tasks 64 and 133 read as *"since you last
   looked"*. Nothing is written to disk (Global constraints: no Hub data on disk).
5. **`DeskSnapshot` is fixed here, with every member a later part named.** Task 65 reads `me`,
   `sessions`, `heads`, `revisionAt`, `refOf`, `events` and `briefCriteria` and `headOf`
   (`task-65.md:354-362`, `:805-813`); task 170 reads `signals(kind)` (`task-170.md:303`); TM-54
   reads the cockpit state (`team…/task-54.md:37-40`). This task gives each a W1 value: `heads` and
   `headOf` from task 40's `WorkReader.byProject` over the store's projects; `sessions` from the
   store; `events` is `[]`, `revisionAt` and `refOf` return `null`, `briefCriteria` returns `null`
   and `signals` returns `[]` **until** a later task passes its source in `DeskInputs` (task 63 the
   feed, P4-16 the loader, task 127 and 170 their signals). A later task **adds** an input; it never
   renames a member. TM-54 reads `snap.cockpit.teamDesk`, not `snap.teamDesk` (gap 3).
6. **Rank is pure and total.** `rankItems` sorts by `tier` ascending, then `waitingSinceMs`
   descending (longest wait first), then `kind`, then `id`, so two runs over one snapshot give one
   order. A band is not ranked: `since` keeps `arrivedAt` ascending (TM-54's decision (b), task 65's
   `since` rank), and `horizon` draws nothing in W1.
7. **The count is the section's rows, drawn as a plain number.** No `of N`, no ratio, no bar: a
   count of rows on screen is an observation (`OD-27`). A folded row (task 64's *"23 steady"*) is one
   row; its own words carry its number.
8. **Keys go through task 53, never a listener of the Desk's own.** The frame's root is
   `[data-work-list]` with `data-key-scope="list"`, so task 53's `dispatchListKey` sees `J`, `K` and
   `N` there (`task-53.md:168-177`, `:249-253`). On mount the Desk registers `focus.next`,
   `focus.prev` and `held.apply` with `registerVerb` and unregisters on unmount; the registry runs
   the newest, and the Work lens is unmounted while the Desk is shown, so neither steals the other's
   keys. A row whose item carries `issueId` gets `data-issue-id` and `data-key-scope="issueRow"` on
   its wrapper, so task 53's issue verbs (`Enter`, `I`) reach it; what they do on the Desk is task 63's.
9. **The Desk is a lens, reached by `setLens("desk")`.** `Lens` gains `"desk"` beside task 37's
   `"work"` (`task-37.md:609`); `LensRouter` gains one arm. The sub-rail stays six rows, as task 37's
   decision 8 holds for `"work"`. A person opens it with `G` then `I`: task 54 prints that sequence
   as `go.desk` (`task-54.md:25`, `:123`) and leaves its runner to this task (`:10`, `:251`), so
   `useDeskGoVerb()` registers it from `AppShell`, beside task 54's `useGoVerbs`. Making the Desk
   the landing surface waits on the owner's review (task 39; gap 4).
10. **Words only, and no motion that re-sorts.** The frame animates nothing: a held arrival is a
    line of text, not a slide. DESIGN-SYSTEM's elevation for a row is the 1px ring plus the layered
    shadow; the section header is type and a hairline, as the prototype draws it (`:236-237`).

**Files** (the skeleton's file map lists `{Desk,DeskSection,DeskZero,deskModel,deskRows}.tsx` and
`desk.css` for tasks 62-65, `:311`):

- `desktop/src/components/desk/deskModel.ts` — SW.17: the types, the registry, `rankItems`,
  `buildDesk`, `takeDeskSnapshot`, `markDeskVisit` (new; `.ts`, decision 1)
- `desktop/src/components/desk/deskModel.test.ts` — the pure tests (new)
- `desktop/src/components/desk/deskRows.tsx` — the one registration module; its head is this
  task's, and tasks 63, 64, 65 and 71 append in task order (new)
- `desktop/src/components/desk/Desk.tsx` — the frame: snapshot, hold, keys (new)
- `desktop/src/components/desk/DeskSection.tsx` — a header, its count, its rows or its zero (new)
- `desktop/src/components/desk/DeskZero.tsx` — NEEDS YOU's zero line; task 65 fills it (new)
- `desktop/src/components/desk/desk.css` — owns `.dk-*` (new)
- `desktop/src/components/desk/Desk.test.tsx` — the surface and key tests (new)
- `desktop/src/store/types.ts` ⚑ — `Lens` gains `"desk"` (modified)
- `desktop/src/components/AppShell.tsx` ⚑ — one `LensRouter` arm (modified)

#### Interfaces — complete: `desktop/src/components/desk/deskModel.ts`, part 1: SW.17's types

```ts
// desktop/src/components/desk/deskModel.ts — SW.17, task 62. The one Desk frame's model.
//
// Pure: no JSX, no React, no clock of its own. Row kinds register here (deskRows.tsx, TM-54's
// teamRows.tsx); Desk.tsx takes one snapshot on entry and builds the model from it once.
import type { IssueHead, IssueRevision, WorkEvent } from "../../api/work";
import type { ProvenanceRef } from "../../api/discover";
import type { CockpitState } from "../../store/store";
import type { Session } from "../../store/types";
import type { WorkReader } from "../../work/source";

/** AX §5's fixed order. The array is the order; nothing sorts sections. */
export type DeskSection = "needs_you" | "decided_without_you" | "failed" | "landed" | "running";
export const DESK_SECTIONS: readonly DeskSection[] =
  Object.freeze(["needs_you", "decided_without_you", "failed", "landed", "running"]);

/** Bands sit outside the ranked sections: horizon above them (nothing in W1), since below. */
export type DeskBand = "horizon" | "since";

export interface DeskItem<T = unknown> {
  kind: string;            // the registered kind, "issue.urgent_mine", "team.decide"
  id: string;              // unique within the kind
  payload: T;
  arrivedAt: string;       // RFC 3339: when the fact that made the row true was observed
  rankReason: string;      // "ranked first: blocks Alice's agent" (AX §5, L-114)
  ifNothing: string;       // the consequence of doing nothing, as a rule, never a time
  /** Set by issue rows: the wrapper carries data-issue-id so task 53's issue verbs reach it. */
  issueId?: string;
}

/** 0 irreversible, 1 blocks someone, 2 the rest. waitingSinceMs: how long it has waited. */
export interface DeskRank { tier: 0 | 1 | 2; waitingSinceMs: number }

/** A signal a later task feeds in (task 127's reminders, task 170's unrouted requests). */
export interface DeskSignal { kind: string; id: string; at: string; data: Readonly<Record<string, unknown>> }

/** What the Desk read on entry. Every member is a value, fixed at takenAt (decision 3). */
export interface DeskSnapshot {
  takenAt: number;                                   // epoch ms, the entry
  lastVisitAt: number | null;                        // markDeskVisit's last write; null: first visit
  me: string | null;                                 // my u- id; null before the first load
  cockpit: Readonly<CockpitState>;                   // the store at entry (TM-54 reads teamDesk)
  sessions: readonly Session[];                      // live and ended, the store's
  heads: ReadonlyMap<string, IssueHead>;             // every readable head, by is- id
  headOf(issueId: string): IssueHead | undefined;
  events: readonly WorkEvent[];                      // SW.5's feed; [] until task 63 passes it
  revisionAt(issueId: string, rev: number): IssueRevision | null;
  refOf(key: string): ProvenanceRef | "resolving" | null;           // P4-16's loader, else null
  briefCriteria(briefKey: string): readonly string[] | "resolving" | null;
  signals(kind: string): readonly DeskSignal[];      // [] for a kind nobody feeds
}

/** What takeDeskSnapshot reads. Desk.tsx passes the live ones; a later task ADDS an input. */
export interface DeskInputs {
  now(): number;
  cockpit(): Readonly<CockpitState>;
  work(): WorkReader | null;                         // task 40's workSource()
  events?(): readonly WorkEvent[];
  refOf?(key: string): ProvenanceRef | "resolving" | null;
  briefCriteria?(briefKey: string): readonly string[] | "resolving" | null;
  signals?(kind: string): readonly DeskSignal[];
}

/** One section or band as the frame draws it. */
export interface DeskGroup {
  place: DeskSection | "since";
  items: readonly DeskItem[];
}
export interface DeskModel { groups: readonly DeskGroup[]; keys: ReadonlySet<string> }
```

`IssueHead` and `IssueRevision` are task 37's types (`task-37.md:129-134`). `WorkEvent` is **not**
in `api/work`: task 37 leaves it in task 20's fixtures (`task-37.md:59-60`, `task-20.md:203`), and
a production file must not import a test module, so the import above follows whichever module
task 55's feed exports it from (gap 5; task 65 makes the same assumption, `task-65.md:347`).
`ProvenanceRef` is phase 4's `api/discover` type as task 65 imports it
(`task-65.md:348`; **UNVERIFIED until P4 merges**, and if absent the member's type is `unknown`
until then). `IssueRevision` carries the revision's `content` (`task-20.md:321-326`).

#### Interfaces — complete: `deskModel.ts`, part 2: the registry, rank, build and snapshot

```ts
export type DeskRender<T> = (item: DeskItem<T>) => JSX.Element;
export type DeskRanker<T> = (item: DeskItem<T>, snap: DeskSnapshot) => DeskRank;
export type DeskSelect = (snap: DeskSnapshot) => readonly DeskItem[];

interface RowKind { kind: string; place: DeskSection | DeskBand; render: DeskRender<unknown>; rank: DeskRanker<unknown> }
const rows = new Map<string, RowKind>();
const sources = new Map<string, DeskSelect>();

/** SW.17. A second registration of one kind throws (decision 2). */
export function registerDeskRow<T>(kind: string, place: DeskSection | DeskBand,
  render: DeskRender<T>, rank: DeskRanker<T>): void {
  if (rows.has(kind)) throw new Error(`desk row kind ${kind} is already registered`);
  rows.set(kind, { kind, place, render: render as DeskRender<unknown>, rank: rank as DeskRanker<unknown> });
}

/** The items of one kind, read from the snapshot. A second source for one kind throws. */
export function registerDeskSource(kind: string, select: DeskSelect): void {
  if (sources.has(kind)) throw new Error(`desk source for ${kind} is already registered`);
  sources.set(kind, select);
}

/** The render for a kind; Desk.tsx calls it per row. */
export function deskRenderOf(kind: string): DeskRender<unknown> {
  const k = rows.get(kind);
  if (!k) throw new Error(`desk row kind ${kind} has no row registered`);
  return k.render;
}

/** Test seam: vitest does not reset module state between files (heartbeat.ts:68-69). */
export function __resetDeskRegistry(): void { rows.clear(); sources.clear(); lastVisit = null; }

/** Decision 6: tier, then the longest wait, then kind and id. Pure and total. */
export function rankItems(items: readonly DeskItem[], snap: DeskSnapshot): DeskItem[] {
  const keyed = items.map((item) => ({ item, r: rows.get(item.kind)!.rank(item, snap) }));
  keyed.sort((a, b) => a.r.tier - b.r.tier || b.r.waitingSinceMs - a.r.waitingSinceMs
    || a.item.kind.localeCompare(b.item.kind) || a.item.id.localeCompare(b.item.id));
  return keyed.map((k) => k.item);
}

export const itemKey = (i: Pick<DeskItem, "kind" | "id">): string => `${i.kind}:${i.id}`;

/** One pass over every source. The five sections in DESK_SECTIONS order, then "since". */
export function buildDesk(snap: DeskSnapshot): DeskModel {
  for (const kind of sources.keys()) if (!rows.has(kind)) throw new Error(`desk source ${kind} has no row`);
  for (const kind of rows.keys()) if (!sources.has(kind)) throw new Error(`desk row ${kind} has no source`);
  const by = new Map<string, DeskItem[]>();
  for (const [kind, select] of sources) {
    const place = rows.get(kind)!.place;
    if (place === "horizon") continue;                            // no horizon slice in W1 (Q4)
    for (const item of select(snap)) by.set(place, [...(by.get(place) ?? []), { ...item, kind }]);
  }
  const groups: DeskGroup[] = DESK_SECTIONS.map((s) => ({ place: s, items: rankItems(by.get(s) ?? [], snap) }));
  const since = [...(by.get("since") ?? [])].sort((a, b) =>
    Date.parse(a.arrivedAt) - Date.parse(b.arrivedAt) || itemKey(a).localeCompare(itemKey(b)));
  groups.push({ place: "since", items: since });
  return { groups, keys: new Set(groups.flatMap((g) => g.items.map(itemKey))) };
}

let lastVisit: number | null = null;
/** Decision 4: in memory only. Desk.tsx writes it on unmount. */
export function markDeskVisit(ms: number): void { lastVisit = ms; }
export function lastDeskVisitAt(): number | null { return lastVisit; }

/** Decision 5. Reads every input once; a member with no input yet gives its empty value. */
export function takeDeskSnapshot(inp: DeskInputs): DeskSnapshot {
  const cockpit = inp.cockpit();
  const work = inp.work();
  const heads = new Map<string, IssueHead>();
  const revs = new Map<string, IssueRevision>();
  for (const p of cockpit.projects) {
    for (const r of work?.byProject(p.id) ?? []) { heads.set(r.head.id, r.head); revs.set(`${r.head.id}@${r.revision.revision}`, r.revision); }
  }
  const events = inp.events?.() ?? [];
  return Object.freeze({
    takenAt: inp.now(), lastVisitAt: lastVisit, me: work?.me()?.id ?? null, cockpit,
    sessions: Object.freeze(Object.values(cockpit.sessions)), heads, headOf: (id: string) => heads.get(id),
    events, revisionAt: (id: string, rev: number) => revs.get(`${id}@${rev}`) ?? null,
    refOf: inp.refOf ?? (() => null), briefCriteria: inp.briefCriteria ?? (() => null),
    signals: inp.signals ?? (() => []),
  });
}
```

`buildDesk`'s pairing checks run on every build, not at registration, because `deskRows.tsx`
registers a row and its source in two calls and TM-54 in another module (decision 2). `revisionAt`
serves only the head revision each `byProject` row carries (`task-40.md:147-148`); a part that needs
an older revision passes its own read in `DeskInputs` (task 65's *adopted revision*). `Project.id`
is the store's `Project` (`dataSlice.ts:31`); that it is the Hub's project id too is task 24's
assumption (`task-61.md` gap 3).

#### Interfaces — complete: `desktop/src/components/desk/Desk.tsx`

```tsx
// desktop/src/components/desk/Desk.tsx — the one Desk frame (task 62; 05 §5.6).
//
// One snapshot on entry; while open it only compares (decision 3). Keys go through task 53.
import { useEffect, useMemo, useRef, useState } from "react";
import { useCockpit } from "../../store/store";
import { onWorkSourceChange, workSource } from "../../work/source";
import { useVerb } from "../../work/keys/keymap";
import { buildDesk, itemKey, markDeskVisit, takeDeskSnapshot, type DeskInputs, type DeskModel } from "./deskModel";
import { DeskSection } from "./DeskSection";
import "./deskRows";                                        // every row kind registers at load
import "./desk.css";

export const HOLD_LINE = "A new item arrived. The Desk does not re-rank while you read it.";

/** The live inputs. Tests pass their own through `inputs`. */
export const liveDeskInputs: DeskInputs = {
  now: () => Date.now(), cockpit: () => useCockpit.getState(), work: () => workSource(),
};

export interface DeskProps { inputs?: DeskInputs }

export function Desk({ inputs = liveDeskInputs }: DeskProps): JSX.Element {
  const build = (): DeskModel => buildDesk(takeDeskSnapshot(inputs));
  const [model, setModel] = useState<DeskModel>(build);          // the one ranking, on entry
  const [held, setHeld] = useState(false);
  const [cursor, setCursor] = useState<string | null>(null);
  const root = useRef<HTMLDivElement>(null);
  const shown = useRef(model); shown.current = model;

  // Compare, never re-render the rows (decision 3). Any change in either source is a check.
  useEffect(() => {
    const check = () => {
      const fresh = buildDesk(takeDeskSnapshot(inputs));
      for (const k of fresh.keys) if (!shown.current.keys.has(k)) { setHeld(true); return; }
    };
    const offStore = useCockpit.subscribe(check);
    const offSwap = onWorkSourceChange(check);
    const offWork = workSource()?.subscribe(check) ?? (() => {});
    return () => { offStore(); offSwap(); offWork(); markDeskVisit(inputs.now()); };
  }, [inputs]);

  const order = useMemo(() => model.groups.flatMap((g) => g.items.map(itemKey)), [model]);
  const move = (by: 1 | -1) => {
    if (order.length === 0) return;
    const at = cursor === null ? -1 : order.indexOf(cursor);
    const next = order[Math.min(order.length - 1, Math.max(0, at === -1 && by === -1 ? 0 : at + by))];
    setCursor(next);
    root.current?.querySelector<HTMLElement>(`[data-desk-key="${CSS.escape(next)}"]`)?.focus();
  };
  const apply = () => { setModel(build()); setHeld(false); };

  useVerb("focus.next", () => move(1));                      // task 53: stable registration,
  useVerb("focus.prev", () => move(-1));                     // the newest runner's closure
  useVerb("held.apply", apply);

  return (
    <div className="dk" ref={root} data-work-list="" data-key-scope="list" tabIndex={0} aria-label="Desk">
      {held ? (
        <div className="dk-hold" role="status">
          <span>{HOLD_LINE}</span>
          <button type="button" className="dk-show" onClick={apply}>Show <kbd>N</kbd></button>
        </div>) : null}
      {model.groups.map((g) => <DeskSection key={g.place} group={g} cursor={cursor} />)}
    </div>
  );
}
```

`useVerb` is task 53's hook (`task-53.md:310-316`): it registers once on mount, the runner reads
the newest closure through a ref, so `cursor` is never stale, and the unmount removes exactly the
Desk's three; the newest registration runs (`:224-234`). `CSS.escape` is in jsdom and WebView2. The `focus()`
on the cursor row is a keyboard move the reader asked for, not a re-sort.

#### Interfaces — complete: `DeskSection.tsx`, `DeskZero.tsx` and the head of `deskRows.tsx`

```tsx
// desktop/src/components/desk/DeskSection.tsx — one header, its count, its rows or its zero.
import { deskRenderOf, itemKey, type DeskGroup } from "./deskModel";
import { DeskZero } from "./DeskZero";

export const SECTION_TITLE: Record<DeskGroup["place"], string> = {
  needs_you: "NEEDS YOU", decided_without_you: "DECIDED WITHOUT YOU", failed: "FAILED",
  landed: "LANDED", running: "RUNNING", since: "SINCE YOU LAST LOOKED",
};
/** One line each; NEEDS YOU's is DeskZero, and "since" draws nothing when empty. */
export const SECTION_ZERO: Record<Exclude<DeskGroup["place"], "needs_you" | "since">, string> = {
  decided_without_you: "Nothing was decided for you.", failed: "Nothing failed.",
  landed: "Nothing of yours landed since you last looked.", running: "No agents of yours are running.",
};

export function DeskSection({ group, cursor }: { group: DeskGroup; cursor: string | null }): JSX.Element | null {
  const { place, items } = group;
  if (place === "since" && items.length === 0) return null;
  const id = `dk-h-${place}`;
  return (
    <section className="dk-sec" data-desk-section={place} aria-labelledby={id}>
      <h2 className="dk-h" id={id}>
        {SECTION_TITLE[place]}<span className="dk-n">{items.length}</span>
      </h2>
      {items.length === 0
        ? (place === "needs_you" ? <DeskZero /> : <p className="dk-zero">{SECTION_ZERO[place as keyof typeof SECTION_ZERO]}</p>)
        : items.map((item) => {
            const k = itemKey(item);
            return (
              <div key={k} className="dk-row" data-desk-key={k} data-desk-kind={item.kind} tabIndex={-1}
                {...(cursor === k ? { "data-cursor": "", "aria-current": "true" as const } : {})}
                {...(item.issueId ? { "data-issue-id": item.issueId, "data-key-scope": "issueRow" } : {})}>
                {deskRenderOf(item.kind)(item)}
              </div>);
          })}
    </section>
  );
}
```

```tsx
// desktop/src/components/desk/DeskZero.tsx — NEEDS YOU's zero (task 62). Task 65 fills it with
// what W1 can back ("EP-142 landed and proven at 14:02", task-65.md:879-893); until then, the line.
export const ZERO_LINE = "Nothing needs you.";
export function DeskZero(): JSX.Element {
  return <p className="dk-zero dk-zero-need"><b>{ZERO_LINE}</b></p>;
}
```

```tsx
// desktop/src/components/desk/deskRows.tsx — the one module where W's row kinds register, at load.
// Task 62 owns this head. Tasks 63, 64, 65 and 71 append their blocks below, in task order; each
// block registers its kinds' rows AND their sources (decision 2). TM-54 registers from its own
// teamRows.tsx, which Desk.tsx imports beside this file when team mode lands.
export { registerDeskRow, registerDeskSource } from "./deskModel";
export type { DeskItem, DeskRank, DeskSnapshot } from "./deskModel";
```

`deskRows.tsx` has no JSX of its own in this task; it keeps the `.tsx` name because tasks 63-65 put
row elements in it (the skeleton's name, `:311`). Its re-export lets a block write
`registerDeskRow(...)` after one import line of its own, as task 65's block does (`task-65.md:772`).

#### Interfaces — complete: `desktop/src/components/desk/desk.css`

```css
/* Desk frame (task 62). Section headers are type and a hairline; a row is the 1px ring plus the
   layered shadow at the control radius. Nothing here animates: the Desk never moves under the
   reader, and a held arrival is a line of words (decision 10). */
.dk { max-width: 860px; margin: 0 auto; padding: var(--s-4) var(--s-5); outline: none; }
.dk-hold { display: flex; align-items: center; gap: var(--s-3); margin: 0 0 var(--s-3);
  padding: var(--s-2) var(--s-3); border-radius: var(--r-ctl); background: var(--surface);
  box-shadow: var(--e-card); font: var(--t-ui) / var(--t-ui-lh) var(--sans); color: var(--ink-2); }
.dk-hold span { flex: 1; }
.dk-show { background: none; border: 0; padding: 0; color: var(--ink); cursor: pointer; }
.dk-show kbd { font-family: var(--mono); border-radius: var(--r-chip); box-shadow: var(--e-hair);
  padding: 0 var(--s-1); }
.dk-sec { margin-bottom: var(--s-5); }
.dk-h { display: flex; align-items: center; gap: var(--s-2); margin: var(--s-5) 0 var(--s-3);
  font: 700 11px / 1.2 var(--sans); letter-spacing: 0.09em; color: var(--ink-2); }
.dk-h::after { content: ""; flex: 1; height: 1px; background: var(--line-soft); }
.dk-n { font-family: var(--mono); font-weight: 400; letter-spacing: 0;
  font-variant-numeric: tabular-nums; color: var(--ink-3); }
.dk-row { margin-bottom: var(--s-1); border-radius: var(--r-ctl); background: var(--surface);
  box-shadow: var(--e-card); outline: none; }
.dk-row[data-cursor] { box-shadow: 0 0 0 1px var(--accent-ring), var(--e-card); }
.dk-zero { margin: 0; padding: var(--s-2) var(--s-3); font: var(--t-ui) / var(--t-ui-lh) var(--sans);
  color: var(--ink-2); }
.dk-zero b { color: var(--ink); font-weight: 600; }
@media (forced-colors: active) { .dk-row, .dk-hold { border: 1px solid CanvasText; box-shadow: none; } }
```

`--r-ctl`, `--r-chip`, `--e-card`, `--e-hair`, `--surface`, `--line-soft`, `--accent-ring`, `--ink-*`,
`--sans` and `--mono` are the tree's (`desktop/src/styles/tokens.css:32-152`); `--mono` becomes
Commit Mono under `OD-28` through task 26, and the rule above does not name a family. `--s-*`,
`--t-ui` and `--t-ui-lh` are task 26's (`task-26.md:109`, `:420`), and task 26 wins a differing name.
The header's 11 px, 700 and 0.09em are the prototype's (`prototype/work-layer.html:236`); if task
26 names a label step, the header takes it.

#### Interfaces — complete: the edits to `store/types.ts` and `AppShell.tsx`

```ts
// desktop/src/store/types.ts, :173 as task 37 left it (task-37.md:609), one member added.
export type Lens = "discover" | "design" | "develop" | "files" | "merge" | "deploy" | "work" | "desk";
```

```tsx
// desktop/src/components/desk/Desk.tsx, appended: go.desk's runner (task 54 prints G then I).
export function useDeskGoVerb(): void {
  useVerb("go.desk", () => useCockpit.getState().setLens("desk"));
}
```

```tsx
// desktop/src/components/AppShell.tsx — LensRouter (:17), one arm beside task 40's "work";
// and in AppShell's body, beside task 54's useGoVerbs(...), one line.
import { Desk, useDeskGoVerb } from "./desk/Desk";
    case "desk":
      return <Desk />;
  useDeskGoVerb();
```

`go.desk` is in task 54's `VerbId` union (`task-54.md:232`); if task 54 lands after this task, the
line waits for it, as `issue.start` waited for task 53.

The persisted `lens` may hold `"desk"`; it is a preference, not Hub data, and `migrate` needs no
step because an older build never wrote it (task 37's reasoning for `"work"`, `task-37.md:624`).
The sub-rail's six rows do not change (decision 9).

#### Tests to write first, part 1 — `desktop/src/components/desk/deskModel.test.ts` (the pure model)

```ts
import { beforeEach, describe, expect, it } from "vitest";
import { createElement } from "react";
import { WORK_IDS } from "../../test/workFixtures";
import { __resetDeskRegistry, buildDesk, markDeskVisit, registerDeskRow, registerDeskSource,
  takeDeskSnapshot, type DeskItem, type DeskRank, type DeskSnapshot } from "./deskModel";

const T0 = Date.parse("2026-09-25T11:10:00Z");
const item = (kind: string, id: string, at = "2026-09-25T09:00:00Z"): DeskItem =>
  ({ kind, id, payload: null, arrivedAt: at, rankReason: "", ifNothing: "" });
const draw = () => createElement("span");
function kind(k: string, place: Parameters<typeof registerDeskRow>[1], items: DeskItem[], rank: DeskRank = { tier: 2, waitingSinceMs: 0 }) {
  registerDeskRow(k, place, draw, (i) => (typeof i.payload === "object" && i.payload ? i.payload as DeskRank : rank));
  registerDeskSource(k, () => items);
}
const snap = (over: Partial<Parameters<typeof takeDeskSnapshot>[0]> = {}): DeskSnapshot => takeDeskSnapshot({
  now: () => T0, cockpit: () => ({ projects: [], sessions: {} }) as never, work: () => null, ...over });
const places = (s: DeskSnapshot) => buildDesk(s).groups.map((g) => g.place);

beforeEach(() => __resetDeskRegistry());

describe("deskModel", () => {
  it("renders AX's order whatever order kinds register in", () => {
    kind("t.run", "running", [item("t.run", "1")]);
    kind("t.land", "landed", [item("t.land", "1")]);
    kind("t.need", "needs_you", [item("t.need", "1")]);
    expect(places(snap())).toEqual(["needs_you", "decided_without_you", "failed", "landed", "running", "since"]);
    // mutant: sections in registration order
  });
  it("a second registration of one kind throws and names it", () => {
    kind("issue.urgent_mine", "needs_you", []);
    expect(() => registerDeskRow("issue.urgent_mine", "needs_you", draw, () => ({ tier: 2, waitingSinceMs: 0 })))
      .toThrow("desk row kind issue.urgent_mine is already registered");
    // mutant: last registration wins (TM-54 silently replaces an issue row)
  });
  it("a row with no source, or a source with no row, fails the build by name", () => {
    registerDeskRow("t.orphan", "failed", draw, () => ({ tier: 2, waitingSinceMs: 0 }));
    expect(() => buildDesk(snap())).toThrow("desk row t.orphan has no source");
    // mutant: an unfed row kind renders nothing and nobody notices
  });
  it("within a section: tier first, then the longest wait, and one order every run", () => {
    const a = { ...item("t.need", "a"), payload: { tier: 2, waitingSinceMs: 9_000_000 } };
    const b = { ...item("t.need", "b"), payload: { tier: 1, waitingSinceMs: 60_000 } };
    const c = { ...item("t.need", "c"), payload: { tier: 2, waitingSinceMs: 7_800_000 } };
    kind("t.need", "needs_you", [c, a, b]);
    const ids = (s: DeskSnapshot) => buildDesk(s).groups[0].items.map((i) => i.id);
    expect(ids(snap())).toEqual(["b", "a", "c"]);
    expect(ids(snap())).toEqual(ids(snap()));
    // mutants: newest first; age before tier (the blocking row under an older urgent one)
  });
  it("the since band keeps arrival order, and the horizon draws nothing in W1", () => {
    kind("t.since", "since", [item("t.since", "2", "2026-09-25T10:00:00Z"), item("t.since", "1", "2026-09-25T09:00:00Z")]);
    kind("t.hz", "horizon", [item("t.hz", "1")]);
    const m = buildDesk(snap());
    expect(m.groups.at(-1)!.items.map((i) => i.id)).toEqual(["1", "2"]);
    expect([...m.keys].some((k) => k.startsWith("t.hz:"))).toBe(false);
    // mutants: the band ranked by tier; a horizon row drawn from no slice
  });
  it("the snapshot gives empty values for inputs nobody passes, and reads each input once", () => {
    let reads = 0;
    const s = snap({ cockpit: () => { reads++; return { projects: [], sessions: {} } as never; } });
    expect([s.events, s.signals("request.unrouted"), s.refOf("brief:b@1"), s.revisionAt(WORK_IDS.ep142, 3)])
      .toEqual([[], [], null, null]);
    expect(reads).toBe(1);
    markDeskVisit(T0 - 60_000);
    expect(snap().lastVisitAt).toBe(T0 - 60_000);
    expect(s.me).toBeNull();                                   // no work source: no signed-in caller
    // mutants: a snapshot that re-reads the store on each member access; a visit written to disk
  });
});
```

#### Tests to write first, part 2 — `desktop/src/components/desk/Desk.test.tsx` (the frame, keys, guards)

Keys reach the Desk the way they do in the app: a `keydown` on an element inside the frame, then
task 53's `dispatchListKey` with that event, which is the one line `useKeyboardMap` runs for it
(`task-53.md:298-308`, `:330`). The registry is reset per case, so each case registers its own
kinds; the kinds `deskRows.tsx` registers at load are the business of tasks 63-65's tests.

```tsx
import { act, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { dispatchListKey, resetVerbsForTest } from "../../work/keys/keymap";
import { resetCockpit, useCockpit } from "../../store/store";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { Desk, HOLD_LINE } from "./Desk";
import { __resetDeskRegistry, lastDeskVisitAt, registerDeskRow, registerDeskSource, type DeskInputs, type DeskItem } from "./deskModel";
import deskSrc from "./Desk.tsx?raw";
import cssSrc from "./desk.css?raw";

const T0 = Date.parse("2026-09-25T11:10:00Z");
const it_ = (kind: string, id: string): DeskItem => ({ kind, id, payload: id, arrivedAt: "2026-09-25T09:00:00Z",
  rankReason: "", ifNothing: "" });
let feed: Record<string, DeskItem[]>;
function kinds() {
  for (const [k, place] of [["t.need", "needs_you"], ["t.land", "landed"], ["t.run", "running"]] as const) {
    registerDeskRow<string>(k, place, (i) => <span>{`${k} ${i.payload}`}</span>, () => ({ tier: 2, waitingSinceMs: 0 }));
    registerDeskSource(k, () => feed[k] ?? []);
  }
}
const inputs: DeskInputs = { now: () => T0, cockpit: () => useCockpit.getState(), work: () => null };
function press(el: Element, key: string, code: string) {
  const e = new KeyboardEvent("keydown", { key, code, bubbles: true, cancelable: true });
  el.dispatchEvent(e);
  dispatchListKey(e);                                              // useKeyboardMap's one line
}
const desk = () => screen.getByLabelText("Desk");
const sec = (place: string) => desk().querySelector<HTMLElement>(`[data-desk-section="${place}"]`)!;

beforeEach(() => {
  resetCockpit(); resetVerbsForTest(); __resetDeskRegistry();
  feed = { "t.need": [it_("t.need", "a")] };
  kinds();
});

describe("Desk frame", () => {
  it("an item offered while open is held behind one line until N", () => {
    render(<Desk inputs={inputs} />);
    feed["t.need"] = [it_("t.need", "a"), it_("t.need", "b")];
    act(() => { useCockpit.setState({}); });                      // any store change is a check
    expect(screen.queryByText("t.need b")).toBeNull();
    expect(screen.getByRole("status").textContent).toContain(HOLD_LINE);
    press(desk(), "n", "KeyN");
    expect(screen.getByText("t.need b")).toBeTruthy();
    expect(screen.queryByText(HOLD_LINE)).toBeNull();
    // mutant: a live re-render that slides the new row in under the reader
  });
  it("a row that would leave stays while open", () => {
    render(<Desk inputs={inputs} />);
    feed["t.need"] = [];
    act(() => { useCockpit.setState({}); });
    expect(screen.getByText("t.need a")).toBeTruthy();
    // mutant: live removal under the reader
  });
  it("leaving and returning re-ranks, and the visit is kept in memory", () => {
    const first = render(<Desk inputs={inputs} />);
    feed["t.need"] = [it_("t.need", "a"), it_("t.need", "b")];
    first.unmount();
    expect(lastDeskVisitAt()).toBe(T0);
    render(<Desk inputs={inputs} />);
    expect(screen.getByText("t.need b")).toBeTruthy();
    expect(screen.queryByText(HOLD_LINE)).toBeNull();
    // mutant: the snapshot survives the visit (a module-level model)
  });
  it("an empty FAILED keeps its header, its count and its zero line", () => {
    render(<Desk inputs={inputs} />);
    expect(within(sec("failed")).getByRole("heading").textContent).toBe("FAILED0");
    expect(within(sec("failed")).getByText("Nothing failed.")).toBeTruthy();
    expect(desk().querySelectorAll("[data-desk-section]").length).toBe(5); // "since" is empty: not drawn
    // mutant: empty sections hidden (a failed worktree behind a calm state, 07 D C5)
  });
  it("j walks across a section edge and k walks back", () => {
    feed["t.land"] = [it_("t.land", "x")];
    render(<Desk inputs={inputs} />);
    press(desk(), "j", "KeyJ");
    expect(document.activeElement?.getAttribute("data-desk-key")).toBe("t.need:a");
    press(document.activeElement!, "j", "KeyJ");
    expect(document.activeElement?.getAttribute("data-desk-key")).toBe("t.land:x");
    press(document.activeElement!, "k", "KeyK");
    expect(document.activeElement?.getAttribute("data-desk-key")).toBe("t.need:a");
    // mutant: j stops at a section edge
  });
  it("no prediction on a full Desk, and nothing in the frame ticks or slides", () => {
    feed = { "t.need": [it_("t.need", "a")], "t.land": [it_("t.land", "x")], "t.run": [it_("t.run", "r")] };
    render(<Desk inputs={inputs} />);
    expectNoPrediction(desk());
    expect(deskSrc).not.toMatch(/setInterval|requestAnimationFrame|scrollIntoView|localStorage/);
    expect(cssSrc).not.toMatch(/transition|@keyframes|animation/);
    // mutants: a per-frame timer; a slide-in for arrivals; the visit on disk
  });
});
```

The heading's text is the title and the count with no separator in the DOM (`FAILED0`) because the
count is its own `span`; a screen reader reads *"FAILED 0"* from the two nodes. The `now` input is
`T0`, so the unmount writes `T0`.

#### Steps

- [ ] Write `deskModel.test.ts`; it fails to import `./deskModel`. Write `deskModel.ts` until the
      six pass. Mutants, one at a time, then revert: iterate `rows` instead of `DESK_SECTIONS`
      (test 1); `rows.set` without the `has` check (test 2); drop the pairing checks (test 3); sort
      `waitingSinceMs` ascending, or compare it before `tier` (test 4); rank the `since` band, or
      draw `horizon` (test 5); read `inp.cockpit()` inside `headOf` (test 6).
- [ ] Write `DeskZero.tsx`, `DeskSection.tsx`, the head of `deskRows.tsx`, `desk.css` and `Desk.tsx`.
- [ ] Write `Desk.test.tsx`; mutants: `setModel(fresh)` in `check` (tests 1 and 2); a module-level
      model (test 3); `items.length === 0 ? null` in `DeskSection` (test 4); clamp `move` to the
      group (test 5); `transition: opacity` on `.dk-row` (test 6).
- [ ] Make the `types.ts` and `AppShell.tsx` edits. Tasks 37, 40 and 53's suites still pass.
- [ ] Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/components/desk'
# expected: 2 files, 12 tests, all passing (6 model, 6 frame)
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, tasks 37, 40 and 53 included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -nE "<[A-Za-z]|jsx" desktop/src/components/desk/deskModel.ts'
# expected: no output (decision 1: the model holds no JSX)
~/.claude/tools/claudezsh/claudezsh 'grep -cE "case \"desk\"" desktop/src/components/AppShell.tsx'
# expected: 1
```

#### What this task does NOT claim

- **Any row content.** Issue rows are task 63's, FAILED, LANDED and RUNNING task 64's, informs,
  the reassign *decide* and the filled zero task 65's, *shipped* task 71's, team mode's rows TM-54's.
- **The horizon bar and quiet mode** (Q4): the `horizon` band registers and draws nothing.
- **The Desk as the landing surface**: task 39's review decides (gap 4). `G I` and
  `setLens("desk")` open it; a palette row is task 59's.
- **Live feed events, the provenance loader or signals in the snapshot**: each is `[]` or `null`
  until the task that owns its source passes it (decision 5).
- **That 30 agents fit one screen at 125 %**: task 70 measures it.
- **Visual acceptance**: the owner's eyes (CLAUDE.md, working agreements).
- **Parity**: closes L-114 with tasks 63-65 (`05-design.md:253`); serves L-112 and L-113 (closed by
  task 65), as the scope says (`tasks-61-80.md:97-98`).

#### Gaps found while writing this part

1. The skeleton's file map (`:311`) names `deskModel.tsx`; this part writes `deskModel.ts`
   (decision 1, as the scope allowed) and adds `Desk.test.tsx` and `deskModel.test.ts`.
2. Three later parts import SW.17 from three paths: task 127 `./deskFrame` (`task-127.md:400`),
   task 170 `../desk/registry` (`task-170.md:295`), tasks 65 and 133 `./deskModel`. The path is
   `desktop/src/components/desk/deskModel`; tasks 127 and 170 follow it.
3. TM-54 reads `snap.teamDesk` (`team…/task-54.md:37-40`, its G54-2); this snapshot carries the
   store as `snap.cockpit`, so TM-54's one `select` reads `snap.cockpit.teamDesk`.
4. The skeleton names no entry to the Desk. Task 54 prints `G I` → `go.desk` and hands its runner
   here (`task-54.md:10`); the landing surface waits on task 39's review. Neither the skeleton's
   task 62 row (`:462`) nor its file map (`:313`) says so.
5. `WorkEvent` is task 20's fixture type, not `api/work`'s (`task-37.md:59-60`); tasks 62 and 65
   import it from `api/work`. Task 55's feed must export the production type, or task 37 must.
6. Task 65 names `deskTime`, the Desk's `HH:MM` formatter, as task 62's (`task-65.md:813`). It is
   not built here: `lib/time.ts` owns time words, and task 65 adds it there or beside its rows.
7. Tasks 63-65's tests render `Desk` with the kinds `deskRows.tsx` registers at load; they must not
   call `__resetDeskRegistry`, or they register their own kinds as this part's tests do.
