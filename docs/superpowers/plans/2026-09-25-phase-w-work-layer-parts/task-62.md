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
   decision 8 holds for `"work"`. How a person opens the Desk (`G D`, the palette, the landing
   surface) is not this task's: making it the landing surface waits on the owner's review (task 39;
   gap 4).
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
import { registerVerb } from "../../work/keys/keymap";
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

  useEffect(() => {
    const offs = [registerVerb("focus.next", () => move(1)), registerVerb("focus.prev", () => move(-1)),
      registerVerb("held.apply", apply)];
    return () => offs.forEach((off) => off());
  });                                                          // re-registered each render: fresh closures

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

`registerVerb` returns its own unregister and the newest registration runs (`task-53.md:224-234`),
so re-registering on each render keeps the runner's closure current without a stale `cursor`, and
the unmount removes exactly the Desk's three. `CSS.escape` is in jsdom and WebView2. The `focus()`
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
