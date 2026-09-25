### Task 36: the thread primitives: node, line, lit, trace

Stream **WD1**, defines seam **SW.14** (`<ThreadNode/>`, `<ThreadLine/>`, `<ThreadTrace/>`,
`useThreadLit(criterionId)`), about 3 hours (an estimate, not a measurement). Depends on task 29
(`MarkGlyph kind="thread_node"`, `glyphs.css`) and task 30 (`voice.split`, the sans/mono rule the
node's label follows). No Hub dependency. **Runs after task 34** as well: it appends to task 34's
`forced-colors.css`, a ⚑ file, and shared files run in task order (the skeleton's row names 29
and 30 only; a gap, named in the return).

The thread is the one thing on a work screen that Linear has no object for (05 §2.3): the brief
criterion an issue serves, drawn as a hairline the issue hangs from, with a node where the line
meets the issue. This task builds the four pieces every later surface places (tasks 40, 47, 67,
85). It places none of them itself.

**The four pieces** (02 C2; 05 §4.1, §4.4 C2 and C13, §4.5 rule 4):

| Piece | What it draws | Its one rule |
|---|---|---|
| `ThreadNode` | the 5px ring, as task 29's `MarkGlyph kind="thread_node"`, inside a link to the source span | no source, no node: `null` when `href` is empty (02 C2; AX Principle 7) |
| `ThreadLine` | the 1px hairline in the leading gutter from which a group's rows hang | it reads lit; it never binds hover itself, so moving down a list does not light every group in turn |
| `ThreadTrace` | on drill-in, the line grows from source to target, `scaleY(0 → 1)` from the top over `--dur-slow` on `--ease` | motion that explains *this came from that*; under reduced motion the line is simply there |
| `useThreadLit` | one module-level store: hovering **or** focusing a criterion lights its node and line in every mounted surface at once | **lit is not selected** (05 §4.1, 07 D I1): it never rings a row and never sets `aria-selected` |

**Decisions made here, so nobody re-derives them:**

1. **The node lights in `--accent-ring`, the line in `--thread-lit`.** Task 29 already lights the
   node through `.gl-mark-node.gl-mark-lit { color: var(--accent-ring) }`, which is 02 C2's
   `thread.lit` word for word ("rings every artifact that carries it, in `--accent-ring`"). The
   line has no such rule in 02, and the prototype lights it in `--thread-lit`
   (`prototype/work-layer.html:28`, `:142-144`). This task adds `--thread-lit` and uses it for the
   line only; it does not touch task 29's `glyphs.css`. Whether a node at `--accent-ring` (L 70.3 %,
   alpha 0.28) reads brighter than an unlit node at `--thread` (L 94.8 %, alpha 0.16) is not
   computed here and is the owner's call at task 39; if it does not, the change is one line in
   task 29's file (a gap, named in the return).
2. **Who binds lit.** A caller binds the element that *is* the criterion: the group header
   (task 40), the `serves` line (task 47), the Develop chip (task 67). `ThreadNode` binds itself,
   because a node is the criterion's mark. `ThreadLine` and `ThreadTrace` bind nothing.
3. **Hover wins over focus, and both are held.** The store keeps two slots, `hover` and `focus`;
   the lit criterion is `hover ?? focus`. Leaving a header with the pointer while the keyboard
   cursor sits on another criterion falls back to that one, instead of going dark.
4. **`href` is opaque here.** The desktop has no router and no `href` anywhere in `src/` today
   (grep, 2026-09-25). A node's `href` is whatever its caller's source resolver makes; task 47's
   `serves` line and task 67's chip decide the form. A hash `href` changes `location.hash` without
   reloading the webview, so nothing here can navigate the app away. A node with an empty `label`
   is also `null`: a link nobody can name is as sourceless to a screen-reader user as one with no
   `href`. That tightens SW.14's sentence ("`null` when `href` is empty"); it adds no prop.
5. **`paper`'s thread at full strength.** On `paper` there is no grain (05 §4.4 C13, D M5), so the
   thread gutter is the fifth mark and is drawn opaque. Task 26's paper `--thread` is
   `oklch(21.0% 0.0120 67 / 0.20)`, 1.52:1 on paper's `--page` once composited. This task sets it
   to `oklch(62.0% 0.0120 67)`, **3.34:1**, which clears WCAG 1.4.11's 3:1 for a graphical object,
   and paper's `--thread-lit` to `oklch(40.0% 0.0120 67)`, **8.46:1**. Both figures were computed
   while this plan was written (OKLCH → linear sRGB → WCAG, compositing in encoded sRGB as
   Chromium does); `tokens.test.ts` recomputes them. Graphite's `--thread` is not changed.
6. **Every reduced-motion rule is written twice**, as `@media (prefers-reduced-motion: reduce)`
   and as `:root[data-motion="reduce"]`, with identical declarations (task 33's hook, task 34's
   rule). jsdom evaluates neither, so the test reads `thread.css` as text, the way
   `components/common/StatusDot.test.tsx:7` (the `?raw` import) and `:82` already do.
7. **Graphite's `--thread-lit` is `oklch(94.8% 0.0097 73 / 0.58)`.** The prototype writes
   `rgba(255,241,224,.58)` (`prototype/work-layer.html:28`). Its `--thread` on the same line is the
   same `rgba(255,241,224,…)`, and `tokens.css:57` already renders that as the warm ink
   `oklch(94.8% 0.0097 73)` at the prototype's alpha. This task keeps that mapping, so the lit and
   unlit lines are one colour at two strengths. The literal conversion of `#fff1e0`,
   `oklch(96.5% 0.0271 72.8)`, is not used.

**Files** (the skeleton's file map lists the four source files; it omits `thread.css`'s tests,
`tokens.css`, `tokens.test.ts` and `forced-colors.css` for this task, a gap named in the return):
- `desktop/src/components/thread/useThreadLit.ts` — the lit store and hook (new)
- `desktop/src/components/thread/ThreadNode.tsx`, `ThreadLine.tsx`, `ThreadTrace.tsx` (new)
- `desktop/src/components/thread/thread.css` — owns `.thr-*`; no other file writes the prefix (new)
- `desktop/src/components/thread/index.ts` — the four exports, SW.14's one import path (new)
- `desktop/src/components/thread/thread.test.tsx` — eight tests (new)
- `desktop/src/styles/tokens.css` ⚑ — `--thread-lit` in `:root` after `--thread` (`:57`); paper's
  `--thread` raised and `--thread-lit` added (task 26's block) (modified)
- `desktop/src/styles/tokens.test.ts` ⚑ — `--thread-lit` joins `THEME_COLOUR_NAMES`; one test (modified)
- `desktop/src/styles/forced-colors.css` ⚑ (task 34) — the thread block appended (modified)

#### Interfaces — complete: `desktop/src/components/thread/useThreadLit.ts`

```ts
import { useSyncExternalStore } from "react";

/** SW.14. One store for the whole app: which criterion the reader is pointing at. Two slots,
 *  so a pointer leaving a header falls back to the criterion the keyboard is on (decision 3).
 *  Lit is NOT selected (05 §4.1, 07 D I1): nothing here touches selection or aria-selected. */
let hover: string | null = null;
let focus: string | null = null;
let litId: string | null = null;
const listeners = new Set<() => void>();

function publish(): void {
  const next = hover ?? focus;
  if (next === litId) return;          // no re-render for a move inside the same criterion
  litId = next;
  for (const l of listeners) l();
}

function subscribe(l: () => void): () => void {
  listeners.add(l);
  return () => listeners.delete(l);
}

const getSnapshot = (): string | null => litId;

export type ThreadLitBind = {
  onPointerEnter(): void;
  onPointerLeave(): void;
  onFocus(): void;
  onBlur(): void;
};

/** `null` for an issue that serves no criterion: never lit, and its bind does nothing. */
export function useThreadLit(criterionId: string | null): { lit: boolean; bind: ThreadLitBind } {
  const current = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  const bind: ThreadLitBind = {
    onPointerEnter() {
      if (criterionId === null) return;
      hover = criterionId;
      publish();
    },
    onPointerLeave() {
      if (hover !== criterionId) return;   // a later enter elsewhere already won
      hover = null;
      publish();
    },
    onFocus() {
      if (criterionId === null) return;
      focus = criterionId;
      publish();
    },
    onBlur() {
      if (focus !== criterionId) return;
      focus = null;
      publish();
    },
  };
  return { lit: criterionId !== null && current === criterionId, bind };
}

/** Test seam, the heartbeat's pattern (`hooks/heartbeat.ts:70`): module state outlives a test. */
export function __resetThreadLit(): void {
  hover = null;
  focus = null;
  litId = null;
  listeners.clear();
}
```

React's `onFocus` and `onBlur` bubble (they are `focusin`/`focusout` underneath), so a caller
binds a group header or a whole `serves` line and a focus anywhere inside it counts. `bind` is a
new object each render; callers spread it (`{...bind}`), so nothing memoises on it.

#### Interfaces — complete: `desktop/src/components/thread/index.ts`

```ts
/** SW.14: the one import path. Surfaces import from "components/thread", never a file in it. */
export { ThreadNode } from "./ThreadNode";
export { ThreadLine } from "./ThreadLine";
export { ThreadTrace } from "./ThreadTrace";
export { useThreadLit } from "./useThreadLit";
export type { ThreadLitBind } from "./useThreadLit";
```

#### Interfaces — complete: `desktop/src/components/thread/ThreadNode.tsx`

```tsx
import type { JSX } from "react";
import { MarkGlyph } from "../glyphs/MarkGlyph";
import { useThreadLit } from "./useThreadLit";
import "./thread.css";

/** SW.14. The criterion's mark where the thread meets an artifact: task 29's 5px ring, inside a
 *  link to the source span. No source, no node (02 C2; AX Principle 7): an empty `href` or an
 *  empty `label` renders nothing, so a sourceless thread is deleted, not drawn decoratively. */
export function ThreadNode(props: { criterionId: string; href: string; label: string }): JSX.Element | null {
  const { criterionId, href, label } = props;
  // The hook runs before the early return, so the hook order never depends on the props.
  const { lit, bind } = useThreadLit(criterionId);
  if (href.trim() === "" || label.trim() === "") return null;
  return (
    <a
      className="thr-node"
      href={href}
      aria-label={label}
      data-criterion={criterionId}
      data-lit={lit ? "true" : undefined}
      {...bind}
    >
      <MarkGlyph kind="thread_node" size={16} lit={lit} />
    </a>
  );
}
```

The link's `aria-label` is its accessible name, so the glyph's own "Provenance thread" label is not
read after it. The caller writes `label` as the source it opens ("Brief, criterion c-91e2: resume
after a dropped upload"); this task fixes no wording.

#### Interfaces — complete: `desktop/src/components/thread/ThreadLine.tsx`

```tsx
import type { JSX, ReactNode } from "react";
import { useThreadLit } from "./useThreadLit";
import "./thread.css";

/** SW.14. The 1px hairline in the leading gutter from which a group's rows hang (05 §4.1). It
 *  READS lit and binds nothing (decision 2); it never clones or re-classes its children, so a
 *  lit line cannot ring a row or set aria-selected on one (lit is not selected). */
export function ThreadLine(props: { criterionId: string; children: ReactNode }): JSX.Element {
  const { lit } = useThreadLit(props.criterionId);
  return (
    <div className="thr-line" data-criterion={props.criterionId} data-lit={lit ? "true" : undefined}>
      {props.children}
    </div>
  );
}
```

#### Interfaces — complete: `desktop/src/components/thread/ThreadTrace.tsx`

```tsx
import type { JSX, ReactNode } from "react";
import "./thread.css";

/** SW.14. On drill-in the line grows from source to target: scaleY(0 → 1) from the top, over
 *  --dur-slow on --ease. It says "this came from that", so it plays once per drill-in; under
 *  reduced motion (either switch, decision 6) the line is simply there. The caller owns `play`:
 *  true on the render that follows a drill-in, false after (task 47 sets it). */
export function ThreadTrace(props: { play: boolean; children: ReactNode }): JSX.Element {
  return (
    <div className="thr-trace" data-play={props.play ? "true" : undefined}>
      {props.children}
    </div>
  );
}
```

#### Interfaces — complete: `desktop/src/components/thread/thread.css`

```css
@import "../../styles/tokens.css";

/* SW.14, task 36. Owns .thr-*; no other file writes the prefix.
 * The gutter axis is x = 7px: the left pixel of task 29's 16px node box, whose ring is centred at
 * 8. A surface (tasks 40, 47, 67, 85) puts its ThreadNode at the gutter's left edge and its rows
 * at --thr-indent, so the node sits on the line and the rows hang from it (05 §4.1). */

.thr-node {
  display: inline-flex;
  flex: 0 0 auto;
  color: inherit;
  text-decoration: none;
}

/* The line: the unlit hairline is ::before, the lit one ::after at opacity 0. Lit fades the
 * overlay in; nothing else moves, so lit changes the thread's strength and no row's box. */
.thr-line {
  position: relative;
  padding-left: 22px;
}

.thr-line::before,
.thr-line::after,
.thr-trace::before {
  content: "";
  position: absolute;
  left: 7px;
  top: 0;
  bottom: 6px;
  width: 1px;
  pointer-events: none;
}

.thr-line::before {
  background: var(--thread);
}

.thr-line::after {
  background: var(--thread-lit);
  opacity: 0;
  transition: opacity var(--dur-fast) var(--ease);
}

.thr-line[data-lit="true"]::after {
  opacity: 1;
}

/* The trace: drawn lit, because on drill-in the thread is the subject of the view. */
.thr-trace {
  position: relative;
  padding-left: 22px;
}

.thr-trace::before {
  background: var(--thread-lit);
  transform-origin: top;
}

.thr-trace[data-play="true"]::before {
  animation: thr-grow var(--dur-slow) var(--ease) both;
}

@keyframes thr-grow {
  from { transform: scaleY(0); }
  to { transform: scaleY(1); }
}

/* Decision 6: each reduced-motion rule twice, identical declarations. The line is simply there. */
@media (prefers-reduced-motion: reduce) {
  .thr-trace[data-play="true"]::before { animation: none; }
  .thr-line::after { transition: none; }
}

:root[data-motion="reduce"] .thr-trace[data-play="true"]::before { animation: none; }
:root[data-motion="reduce"] .thr-line::after { transition: none; }
```

`transition: opacity var(--dur-fast)` is the one entry `TRANSITIONABLE` allows for opacity
(`styles/designsystem.test.ts:181`); the prototype's `--dur` (`work-layer.html:143`) would fail that
audit, so the fade is 120 ms, not 180 ms. `thr-grow` is not a loop, so the looping-animation audit
(`designsystem.test.ts:693`) does not require its stand-in; the reduced-motion block is decision 6's.

