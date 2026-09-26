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
