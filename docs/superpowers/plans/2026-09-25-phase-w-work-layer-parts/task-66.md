### Task 66: the accessibility floor: one live region, `aria-hidden` liveness, grid roles

Stream **WD2**, about 2.5 hours (an estimate, not a measurement). Depends on task 41 (the held
arrivals it announces as a digest) and task 62 (the Desk frame whose sections become grids), and
through them on tasks 40, 42, 43, 56 and 65. It closes no parity row alone: it is the floor under
L-101, L-113 and L-225, and under L-36's undo pause (05 §4.6), which task 56 already built.

After this task a screen reader hears one polite voice per window: my own act's answer at once, a
digest of arrivals at most once per 15 minutes, and never a heartbeat. The Work list, Triage and
each Desk section are grids that say their full size while they window, and the board's columns
are listboxes.

**Decisions made here, so nobody re-derives them:**

1. **One region, enforced by a source test, not by review.** The tree today has three live
   regions: `Toasts.tsx:42` (`aria-live="polite"` on the stack), `Toasts.tsx:19` (`role="status"`
   on every toast row, which is an implicit polite region) and `RailFooter.tsx:19` (`role="status"`
   on the error line); checked on 2026-09-26. W1's parts add more: task 43's drop offer
   (`task-43.md:41`), task 46's request line (`task-46.md:619`), task 52's refused lines
   (`task-52.md:315`, `:372`), task 61's Start chip notes and consequence (`task-61.md:397-416`)
   and task 62's hold line (`task-62.md:340`). Each keeps its visible text, loses its role, and
   speaks through `announce`. The guard test fails on any `aria-live`, `role="status"`,
   `role="alert"` or `role="log"` in `desktop/src` outside `LiveRegion.tsx`, so a later part that
   prints one (tasks 109, 110, 130-140 and 155 do) fails until it takes `<Said>`.
2. **Two kinds, and a key.** `announce(text, kind, key?)`: an **answer** (to my own act, a
   refusal of it, or an error I caused) is spoken at once; a **digest** (anything that arrived) is
   spoken at once only if no digest was spoken in the last 15 minutes, else it is held, and at the
   window's end the region speaks the newest held text once, followed by `(and N more)` when more
   than one was held. A `key` makes a text speak at most once per window lifetime (task 106's
   `negotiation:<seenKey>`). An answer never waits behind a digest and never resets its window.
3. **`useAnnounce()` is the short form** task 106 already imports (`task-106.md:145`, `:162-164`):
   `(text, key?) => announce(text, "digest", key)`. `useLiveRegion()` returns the full
   `{ announce }` of the scope.
4. **The region is one element in `AppShell`**, visually hidden (`.sr-only`), `aria-live="polite"`
   and `aria-atomic="true"`. Re-speaking the same text clears the node and sets it on the next
   animation frame; nothing else writes the node.
5. **Liveness is `aria-hidden`.** `StatusDot` gains `decorative`: no `role="img"`, no
   `aria-label`, no `title`, and `aria-hidden="true"` (`StatusDot.tsx:56-63` today carries all
   three). Work and Desk rows pass it, because the row's STATE word carries the status. Task 40's
   `AgentLine` root (`task-40.md:702`) is `aria-hidden` and gets an `id`; its row names it in
   `aria-describedby`, so the agent's words are read as the row's description and never as a live
   change. The heartbeat's flash and the `--fail` flash are CSS on hidden or unchanged nodes.
6. **What a row is.** In the Work list and Triage every group head and every issue is a
   `role="row"`; the agent line is not (decision 5). `aria-rowcount` is the number of rows the view
   holds (heads plus issues), not the number mounted; `aria-rowindex` is the item's 1-based place in
   that order. So 120 issues in 4 groups say `aria-rowcount="124"`. (The scope's example says
   *120*: that is the issue count, which the head rows add to; the mutant, the rendered count, is
   the same.)
7. **The Desk is one grid per section** (the scope's words): `DeskSection` (task 62) wraps its
   rows in `div[role="grid"][aria-labelledby=<its h2>]`; each `.dk-row` is `role="row"` with one
   `role="gridcell"` around the rendered row. No `aria-rowcount` is set there: the Desk never
   windows.
8. **The undo pause is task 56's**: `UndoChip` pauses on focus or hover (`task-56.md:60-65`) and
   its tests (`task-56.md:705`, `:750`) hold it. This task builds nothing there; its test 5 checks
   the one thing task 56 does not: the chip's countdown digits are `aria-hidden` and the answer
   *"EP-142 deferred. Undo, U"* is spoken once through the region.
9. **The `?` sheet already prints Space** (`task-54.md:423`, *"Space peeks: tap to open, hold to
   look"*) and no task 54 test checks it; this task adds the test, not the line.

**Files:**
- `desktop/src/components/common/LiveRegion.tsx` (+ `LiveRegion.test.tsx`) — the announcer, the
  one region, `useLiveRegion`, `useAnnounce`, `<Said>` (new)
- `desktop/src/components/AppShell.tsx` ⚑ — mounts `<LiveRegion />` once, beside `<Toasts />`
  (`AppShell.tsx:98`) (modified)
- `desktop/src/components/common/Toasts.tsx` ⚑ — drops both regions (`:19`, `:42`); each new toast's
  message is spoken as an answer (modified; **not in the skeleton's file map**)
- `desktop/src/components/rail/RailFooter.tsx` ⚑ — drops `role="status"` (`:19`), speaks its
  error as an answer (modified; **not in the skeleton's file map**)
- `desktop/src/components/common/StatusDot.tsx` ⚑ (+ `StatusDot.test.tsx` ⚑) — the `decorative`
  prop (modified; **not in the skeleton's file map**)
- `desktop/src/components/common/common.css` ⚑ — `.sr-only` (modified)
- Grid roles: `components/work/WorkLens.tsx`, `CriterionGroup.tsx`, `IssueRow.tsx`, `AgentLine.tsx`
  (task 40), `TriageView.tsx` (task 42), `BoardView.tsx` (task 43), `components/desk/DeskSection.tsx`
  (task 62) ⚑ (modified)
- Regions to `<Said>`: `BoardView.tsx` (task 43's drop offer), task 46's request line, task 52's two
  refused lines, `components/work/StartChip.tsx` (task 61), `components/desk/Desk.tsx` (task 62's
  hold line), task 65's two rows' answer lines ⚑ (modified)
- `desktop/src/a11y.floor.test.tsx` — the guard, the shell count and the fixture checks (new)

**Interfaces:**
- Produces: `announce(text, kind, key?)` through `useLiveRegion()`; `useAnnounce()`; `<Said
  text kind />`; `StatusDot`'s `decorative`; the grid roles above. Consumed by every later part that
  speaks (task 106 is the first on the tree) and by task 70's run (a reader check by the owner).
- Consumes: task 40's `items` order and `WINDOW_OVER` (`task-40.md:531-541`), task 41's held
  arrivals, task 62's `DeskSection`, task 56's `UndoChip`, task 54's `ShortcutSheet`, task 20's
  `expectNoPrediction`, phase 2's heartbeat (`hooks/heartbeat`).

#### Interfaces — complete: `desktop/src/components/common/LiveRegion.tsx`, part 1: the announcer

```ts
// desktop/src/components/common/LiveRegion.tsx — task 66. The window's one live region.
//
// Pure core: no React, no DOM, an injected clock and scheduler, so a test proves the 15-minute
// window with plain numbers (05 §4.4 rule 5; AX §6.8).
export type AnnounceKind = "answer" | "digest";

export const DIGEST_WINDOW_MS = 15 * 60 * 1000;

export interface Announcer {
  announce(text: string, kind: AnnounceKind, key?: string): void;
  /** The newest spoken text and a sequence number that moves on every utterance. */
  current(): { text: string; seq: number };
  subscribe(fn: () => void): () => void;
}

export interface AnnouncerClock {
  now(): number;
  later(ms: number, fn: () => void): void;      // setTimeout in the app, a queue in tests
}

export function createAnnouncer(clock: AnnouncerClock): Announcer {
  let spoken = { text: "", seq: 0 };
  let lastDigestAt = Number.NEGATIVE_INFINITY;
  let held: string[] = [];
  let flushArmed = false;
  const keys = new Set<string>();
  const subs = new Set<() => void>();
  const say = (text: string) => { spoken = { text, seq: spoken.seq + 1 }; subs.forEach((f) => f()); };
  const flush = () => {
    flushArmed = false;
    if (held.length === 0) return;
    const newest = held[held.length - 1];
    const more = held.length - 1;
    held = [];
    lastDigestAt = clock.now();
    say(more > 0 ? `${newest} (and ${more} more)` : newest);
  };
  return {
    announce(text, kind, key) {
      const t = text.trim();
      if (t === "") return;
      if (key !== undefined) { if (keys.has(key)) return; keys.add(key); }
      if (kind === "answer") { say(t); return; }             // decision 2: never waits, never resets
      const since = clock.now() - lastDigestAt;
      if (since >= DIGEST_WINDOW_MS && held.length === 0) { lastDigestAt = clock.now(); say(t); return; }
      held.push(t);
      if (!flushArmed) { flushArmed = true; clock.later(Math.max(0, DIGEST_WINDOW_MS - since), flush); }
    },
    current: () => spoken,
    subscribe: (fn) => { subs.add(fn); return () => subs.delete(fn); },
  };
}
```

The count in `(and N more)` is a count of things that already arrived, never a forecast
(`OD-27`); the words carry no time. The key set lives as long as the window: a negotiation fact
(task 106) or an import's end (task 69) is never spoken twice after a re-render.

#### Interfaces — complete: `LiveRegion.tsx`, part 2: the one element, the hooks, `<Said>`

```tsx
import { createContext, useContext, useEffect, useRef, useSyncExternalStore, type JSX } from "react";

const appAnnouncer = createAnnouncer({ now: () => Date.now(), later: (ms, fn) => { setTimeout(fn, ms); } });
const Ctx = createContext<Announcer>(appAnnouncer);

/** Tests (and a second window, if one is ever added) pass their own announcer. */
export const AnnouncerProvider = Ctx.Provider;

export function useLiveRegion(): Pick<Announcer, "announce"> {
  const a = useContext(Ctx);
  return { announce: a.announce };
}

/** Decision 3: the short form, a digest with an optional once-key (task 106's call shape). */
export function useAnnounce(): (text: string, key?: string) => void {
  const a = useContext(Ctx);
  return (text, key) => a.announce(text, "digest", key);
}

/** Speaks `text` once each time it changes to a non-empty value. Renders nothing. The visible
 *  fact stays where it is, as plain text; this is its voice. */
export function Said({ text, kind = "answer", onceKey }: { text: string | null; kind?: AnnounceKind; onceKey?: string }): null {
  const a = useContext(Ctx);
  const last = useRef<string | null>(null);
  useEffect(() => {
    if (text === null || text === "" || text === last.current) return;
    last.current = text;
    a.announce(text, kind, onceKey);
  }, [a, text, kind, onceKey]);
  return null;
}

/** Mounted once by AppShell (decision 4). */
export function LiveRegion(): JSX.Element {
  const a = useContext(Ctx);
  const cur = useSyncExternalStore(a.subscribe, a.current);
  const node = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = node.current;
    if (!el || cur.seq === 0) return undefined;
    el.textContent = "";                                   // the same text twice is spoken twice
    const id = requestAnimationFrame(() => { el.textContent = cur.text; });
    return () => cancelAnimationFrame(id);
  }, [cur]);
  return <div ref={node} className="sr-only" aria-live="polite" aria-atomic="true" data-live-region="" />;
}
```

```css
/* common.css ⚑ — task 66. Hidden from sight, never from the accessibility tree. */
.sr-only { position: absolute; inline-size: 1px; block-size: 1px; overflow: hidden;
  clip-path: inset(50%); white-space: nowrap; margin: -1px; padding: 0; border: 0; }
```

`current` returns the same object until the next utterance, so `useSyncExternalStore` sees a
stable snapshot. The node is written by the effect only; React renders it empty and never
reconciles its text, so a re-render does not re-speak.

#### Interfaces — complete: the phase 2 edits (`AppShell.tsx`, `Toasts.tsx`, `RailFooter.tsx`)

```tsx
// AppShell.tsx ⚑ — beside <Toasts /> at :98, once:
      <LiveRegion />

// Toasts.tsx ⚑ — ToastRow's root loses role="status" (:19); the stack loses aria-live (:42).
// ToastRow gains, first in its body:
      <Said text={toast.undo === undefined ? toast.message : `${toast.message}. Undo button on the toast`} />
// (phase 2's toast has a button and no key; "Undo, U" is task 56's chip, spoken by that chip)

// RailFooter.tsx ⚑ — the <p> at :19 loses role="status"; beside it:
      <Said text={lastError.message} />
```

A toast is always the answer to my own act in phase 2 (the optimistic-mutation Undo window,
`Toasts.tsx:1`), so it is an `answer`. The rail's error is the server's own sentence for a call I
made (`RailFooter.tsx:16-18`), also an `answer`, spoken verbatim, never paraphrased.

#### Interfaces — complete: `StatusDot`'s `decorative` and the hidden agent line

```tsx
// StatusDot.tsx ⚑ — the props gain one optional flag; the return becomes:
export function StatusDot(props: { status: DotStatus; live: boolean; label: string; decorative?: boolean }): JSX.Element {
  // ... unchanged down to className ...
  const a11y = props.decorative
    ? { "aria-hidden": true as const }
    : { role: "img" as const, "aria-label": label, title: label };
  return <span {...a11y} className={className} style={live ? beatDelayStyle() : undefined} />;
}

// AgentLine.tsx ⚑ (task 40, task-40.md:696-709) — one prop, two attributes on the root:
export function AgentLine(props: { sessionId: string; whoLabel: string; id?: string }): JSX.Element | null
    <div className="wk-agent" id={props.id} aria-hidden="true">
```

`decorative` keeps `label` required: the caller still says what the dot means, so a later
non-row use cannot drop the words by accident. Phase 2's rail rows (`SessionRow.tsx`,
`BranchRow.tsx`, `MachineChip.tsx`, `SessionPaneHeader.tsx`, `SessionPicker.tsx`; grep on
2026-09-26) keep the labelled dot: they have no STATE word beside it.

#### Interfaces — complete: the Work list as a grid (`WorkLens.tsx`, `CriterionGroup.tsx`, `IssueRow.tsx` ⚑)

```tsx
// WorkLens.tsx ⚑ — beside `items` (task-40.md:531-536): the row order the grid announces.
export const RowIndexContext = createContext<ReadonlyMap<string, number>>(new Map());
const rowIndex = useMemo(() => {
  const m = new Map<string, number>();
  for (const it of items) {
    if (it.kind === "line") continue;                           // decision 6: a line is not a row
    m.set(it.kind === "head" ? `head:${it.g.key}` : `row:${it.id}`, m.size + 1);
  }
  return m;
}, [items]);
// The list element (task-40.md:555) gains, only while a group exists:
//   role="grid" aria-label="Issues" aria-multiselectable="true" aria-rowcount={rowIndex.size}
// and its children render inside <RowIndexContext.Provider value={rowIndex}>.
// With no groups the list carries no role: the sentence "No issues in this feature yet." is the fact.

// CriterionGroup.tsx ⚑ (task-40.md:625-633):
//   .wk-group        role="rowgroup"
//   .wk-group-head   role="row" aria-rowindex={useContext(RowIndexContext).get(`head:${key}`)}
//   .wk-group-words  role="rowheader"        .wk-group-size  role="gridcell"
//   .wk-rows         role="none"             ThreadLine's root  role="none"

// IssueRow.tsx ⚑ (task-40.md:668-681):
const idx = useContext(RowIndexContext).get(`row:${issueId}`);
const agentId = lead === null ? undefined : `wk-agent-${issueId}`;
//   .wk-item  role="row" aria-rowindex={idx} aria-selected={selected} aria-describedby={agentId}
//   .wk-row   role="gridcell"
//   <AgentLine ... id={agentId} />
```

`ThreadLine` (SW.14) takes `role` through its root's rest props if it spreads them; if it does not,
its root gains `role="none"` as one ⚑ attribute (**UNVERIFIED**: SW.14's part prints its props, not
its root). Test 3 below requires the thread's SVG gutter to be `aria-hidden` inside a Work row;
if SW.14 does not set it, the gutter's root takes it here (`task-29.md:466` asserts only that an
SVG holds no live region).

#### Interfaces — complete: Triage, the board and the Desk

```tsx
// TriageView.tsx ⚑ (task 42) — its list element (the one with data-work-list) takes the Work
// list's three attributes; each triage item's root is role="row" with aria-rowindex (1-based in
// the list's order) and its content one role="gridcell". aria-rowcount is the triage count.

// BoardView.tsx ⚑ (task 43) — each column's card container:
//   role="listbox" aria-label={`${columnTitle}, ${cards.length}`} aria-orientation="vertical"
// and each card: role="option" aria-selected={selected} aria-setsize={cards.length}
//   aria-posinset={i + 1}. The column header stays outside the listbox, as a heading.

// DeskSection.tsx ⚑ (task 62, task-62.md:384-392) — the rows go inside one element:
//   <div role="grid" aria-labelledby={id}> {items.map(...)} </div>
// each .dk-row gains role="row", and deskRenderOf(item.kind)(item) sits in <div role="gridcell">.
// The zero lines stay outside the grid, as plain paragraphs.
```

A grid's roving focus is task 53's and task 62's cursor (`data-cursor`, `aria-current`); this task
adds no focus handling. `aria-selected` on a Work row mirrors task 40's `data-selected`.

#### Interfaces — complete: W1's inline regions become plain text plus `<Said>`

Each line keeps its element, its class and its words; it loses the role and gains a `<Said>` beside
it. The kind follows decision 2.

| Where | Today's role | Becomes | Kind |
|---|---|---|---|
| task 43's drop offer in a column header (`task-43.md:41`) | `status` | plain line + `<Said text={offer} />` | answer (my drag) |
| task 46's `iv-request` line (`task-46.md:619`) | `status` | plain line + `<Said>` | answer |
| task 52's refused lines (`task-52.md:315`, `:372`) | `status` | plain line + `<Said>` | answer (a refusal of my act) |
| task 61's Start chip notes (`task-61.md:397-411`) | `status` | plain `sc-note` + `<Said>` | answer |
| task 61's consequence (`task-61.md:416`) | `alert` | plain `sc-consequence` + `<Said>` | answer |
| task 62's hold line (`task-62.md:340`) | `status` | plain `dk-hold` + `<Said kind="digest" text={HOLD_LINE} />` | digest |
| task 65's row answers (`desk-answer`, `desk-answer-fail`) | none | + `<Said text={line} />` | answer |
| task 41's held line (`task-41.md:298`) | `note` | unchanged `note` + `<Said kind="digest">` of its words | digest |

`role="note"` is not a live region (WAI-ARIA 1.2), so task 41's line keeps it; its words reach the
region as a digest, which is task 41's *held arrivals* rule in the scope. Task 56's `UndoChip`
(`task-56.md:447`, `role="group"`) speaks its answer through `<Said text={`${label}. Undo, U`} />`
once when the chip appears, and its countdown `<span>` gains `aria-hidden="true"` (decision 8).

#### Tests to write first, part 1 — `LiveRegion.test.tsx` (the announcer, pure)

```ts
import { describe, expect, it } from "vitest";
import { createAnnouncer, DIGEST_WINDOW_MS } from "./LiveRegion";
import { expectNoPrediction } from "../../test/expectNoPrediction";

function rig() {
  let t = 0;
  const queue: { at: number; fn: () => void }[] = [];
  const said: string[] = [];
  const a = createAnnouncer({ now: () => t, later: (ms, fn) => queue.push({ at: t + ms, fn }) });
  a.subscribe(() => said.push(a.current().text));
  const advance = (ms: number) => {
    t += ms;
    for (const q of queue.splice(0).sort((x, y) => x.at - y.at)) {
      if (q.at <= t) q.fn(); else queue.push(q);
    }
  };
  return { a, said, advance };
}
const MIN = 60 * 1000;

describe("the announcer", () => {
  it("two digests 5 minutes apart speak once; an answer between them speaks at once", () => {
    const { a, said, advance } = rig();
    a.announce("EP-150 arrived in Triage", "digest");
    advance(2 * MIN);
    a.announce("EP-142 deferred. Undo, U", "answer");
    advance(3 * MIN);
    a.announce("EP-151 arrived in Triage", "digest");
    expect(said).toEqual(["EP-150 arrived in Triage", "EP-142 deferred. Undo, U"]);
    advance(DIGEST_WINDOW_MS);
    expect(said).toEqual(["EP-150 arrived in Triage", "EP-142 deferred. Undo, U", "EP-151 arrived in Triage"]);
    // mutants: every arrival spoken; answers throttled with the digests
  });
  it("held digests speak once at the window's end as the newest plus a count", () => {
    const { a, said, advance } = rig();
    a.announce("one", "digest");
    for (const w of ["two", "three", "four"]) { advance(MIN); a.announce(w, "digest"); }
    advance(DIGEST_WINDOW_MS);
    expect(said).toEqual(["one", "four (and 2 more)"]);
    // mutant: each held digest spoken at the flush
  });
  it("a key speaks once for the window's life", () => {
    const { a, said } = rig();
    a.announce("Alice proposed a lease", "digest", "negotiation:th-1:open");
    a.announce("Alice proposed a lease", "answer", "negotiation:th-1:open");
    expect(said).toHaveLength(1);
    // mutant: the key ignored for answers
  });
  it("says nothing that predicts", () => {
    const { a, said } = rig();
    a.announce("four (and 2 more)", "answer");
    const el = document.createElement("div");
    el.textContent = said.join(" ");
    expectNoPrediction(el);
  });
});
```
