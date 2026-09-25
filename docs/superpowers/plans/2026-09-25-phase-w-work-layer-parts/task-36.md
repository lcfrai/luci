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
 * inside the ThreadLine, which indents them 22px, so the node sits on the line and the rows hang
 * from it (05 §4.1; the prototype's .rows, work-layer.html:136-137). */

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

#### Interfaces — complete: the deltas on ⚑ `tokens.css` and ⚑ `forced-colors.css`

In `tokens.css`'s `:root`, directly after `--thread` (`:57`):

```css
  /* The lit thread (02 C2): the same warm ink as --thread, at the prototype's strength
   * (work-layer.html:28). The line only; a lit node is --accent-ring, task 29's rule. */
  --thread-lit:  oklch(94.8% 0.0097 73 / 0.58);
```

In task 26's `:root[data-theme="paper"]` block, its `--thread` line (task 26's part, `:169`) is
replaced, and `--thread-lit` follows it:

```css
  /* No grain on paper (05 §4.4 C13, D M5): the thread gutter is the fifth mark, so it is drawn
   * opaque. 3.34:1 on --page clears WCAG 1.4.11's 3:1; lit is 8.46:1. tokens.test.ts pins both. */
  --thread:      oklch(62.0% 0.0120 67);
  --thread-lit:  oklch(40.0% 0.0120 67);
```

Appended to task 34's `forced-colors.css`, as its own block after task 34's:

```css
/* Task 36: the thread. Forced colours repaint background-color as Canvas, which would erase a
 * line drawn as a background, and force color on a link to LinkText. The line's pseudo-elements
 * and the node's glyph opt out and draw in system colours, lit as Highlight. */
@media (forced-colors: active) {
  .thr-line::before,
  .thr-trace::before {
    forced-color-adjust: none;
    background: CanvasText;
  }
  .thr-line::after {
    forced-color-adjust: none;
    background: Highlight;
  }
  .thr-node .gl-mark-node {
    forced-color-adjust: none;
    color: CanvasText;
  }
  .thr-node[data-lit="true"] .gl-mark-node {
    color: Highlight;
  }
}
```

`CanvasText` and `Highlight` are system colours, not CSS Color 4 named colours, so the named-colour
audit (`styles/designsystem.test.ts:307`, `NAMED_COLOURS`) does not flag them. Two selectors
beat task 29's: `.thr-node .gl-mark-node` is (0,2,0), over `.gl-mark-node` (0,1,0); the lit one is
(0,3,1) over `.gl-mark-node.gl-mark-lit` (0,2,0). Nothing here draws a `box-shadow`, so task 34's
ring audit (which collects `box-shadow` selectors) finds nothing of this task's to require; test 8
below is what holds the thread's counterpart.

In `tokens.test.ts` (task 26's file), `"--thread-lit"` joins `THEME_COLOUR_NAMES` after
`"--thread"`, and that list's `toHaveLength(34)` becomes `toHaveLength(35)`.

#### Tests to write first — `desktop/src/components/thread/thread.test.tsx`

```tsx
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThreadLine, ThreadNode, ThreadTrace, useThreadLit } from "./index";
import { __resetThreadLit } from "./useThreadLit";
// Read as text: jsdom applies no media query and runs no animation (decision 6).
import threadCss from "./thread.css?raw";
import forcedCss from "../../styles/forced-colors.css?raw";

afterEach(() => {
  cleanup();            // unmount first, so no listener outlives the store reset
  __resetThreadLit();
});

/** A stand-in for a group header (task 40): the element that IS the criterion binds lit. */
function Header(props: { criterionId: string }) {
  const { bind } = useThreadLit(props.criterionId);
  return <button {...bind}>criterion {props.criterionId}</button>;
}

const node = (c: string, href = `#brief/${c}`, label = `Brief, criterion ${c}`) => (
  <ThreadNode criterionId={c} href={href} label={label} />
);
const litIn = (root: HTMLElement) =>
  [...root.querySelectorAll('[data-lit="true"]')].map((e) => e.getAttribute("data-criterion"));

/** Every body whose selector list ends in `sel`, joined: `.thr-trace::before` is both the last
 *  of the shared geometry list and a rule of its own, and both bodies count. */
function rule(css: string, sel: string): string {
  const bodies: string[] = [];
  for (let at = css.indexOf(`${sel} {`); at >= 0; at = css.indexOf(`${sel} {`, at + 1)) {
    bodies.push(css.slice(at, css.indexOf("}", at)));
  }
  return bodies.join("\n");
}

describe("SW.14 thread primitives", () => {
  // Kills: a sourceless node drawn anyway (02 C2; AX Principle 7), and one nobody can name.
  it("renders no node without a source", () => {
    const { container } = render(<>{node("c-1", "")}{node("c-2", "#brief/c-2", " ")}</>);
    expect(container.innerHTML).toBe("");
  });

  // Kills: a decorative node (a span, or a link with no name or no target).
  it("makes the node a named link to its span", () => {
    render(node("c-1"));
    const link = screen.getByRole("link", { name: "Brief, criterion c-1" });
    expect(link.getAttribute("href")).toBe("#brief/c-1");
    expect(link.querySelector('svg[data-glyph="mark:thread_node"]')).not.toBeNull();
  });

  // Kills: lit held in React context or per lens, so a second tree never hears of it.
  it("lights every node of a criterion across two trees", async () => {
    const user = userEvent.setup();
    const a = render(<>{node("c-1")}</>).container;
    const b = render(<>{node("c-1")}{node("c-2")}<ThreadLine criterionId="c-1">x</ThreadLine></>).container;
    await user.hover(a.querySelector("a")!);
    expect(litIn(a)).toEqual(["c-1"]);
    expect(litIn(b)).toEqual(["c-1", "c-1"]);                     // b's node and b's line
    expect(b.querySelector('[data-criterion="c-1"] svg')!.classList.contains("gl-mark-lit")).toBe(true);
    await user.unhover(a.querySelector("a")!);
    expect(litIn(a).concat(litIn(b))).toEqual([]);
  });

  // Kills: a pointer-only thread, and a one-slot store that goes dark when the pointer leaves.
  it("lights on focus as well as hover", async () => {
    const user = userEvent.setup();
    const { container } = render(<>{node("c-1")}{node("c-2")}</>);
    await user.tab();                                              // the keyboard is on c-1
    expect(litIn(container)).toEqual(["c-1"]);
    await user.hover(screen.getByRole("link", { name: "Brief, criterion c-2" }));
    expect(litIn(container)).toEqual(["c-2"]);                     // hover wins (decision 3)
    await user.unhover(screen.getByRole("link", { name: "Brief, criterion c-2" }));
    expect(litIn(container)).toEqual(["c-1"]);                     // falls back, not dark
    await user.tab();
    await user.tab();
    expect(litIn(container)).toEqual([]);                          // focus left the thread
  });
```


```tsx
  // Kills: lit implemented as selection (05 §4.1, 07 D I1): a ring on the row, aria-selected.
  it("never marks a lit row selected", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <div role="listbox" aria-label="Work">
        <Header criterionId="c-1" />
        <ThreadLine criterionId="c-1">
          <div role="option" aria-selected="false" className="row">EP-1</div>
        </ThreadLine>
      </div>,
    );
    await user.hover(screen.getByRole("button"));
    expect(container.querySelector(".thr-line")!.getAttribute("data-lit")).toBe("true");
    const row = screen.getByRole("option");
    expect(row.getAttribute("aria-selected")).toBe("false");
    expect(row.className).toBe("row");
    expect(container.querySelectorAll('[aria-selected="true"]')).toHaveLength(0);
  });

  // Kills: a trace that animates height or top, and one that still moves under either switch.
  it("traces with transform only, and not when motion is reduced", () => {
    const { container, rerender } = render(<ThreadTrace play>x</ThreadTrace>);
    expect(container.firstElementChild!.getAttribute("data-play")).toBe("true");
    rerender(<ThreadTrace play={false}>x</ThreadTrace>);
    expect(container.firstElementChild!.hasAttribute("data-play")).toBe(false);

    const frames = /@keyframes thr-grow \{([\s\S]*?)\n\}/.exec(threadCss)![1];
    expect(frames.match(/[a-z-]+(?=:)/g)).toEqual(["transform", "transform"]);
    expect(rule(threadCss, '.thr-trace[data-play="true"]::before'))
      .toContain("animation: thr-grow var(--dur-slow) var(--ease) both");
    expect(rule(threadCss, ".thr-trace::before")).toContain("transform-origin: top");
    expect(threadCss).not.toMatch(/(transition|animation)[^;]*\b(height|top|bottom|width)\b/);

    const media = /@media \(prefers-reduced-motion: reduce\) \{([\s\S]*?)\n\}/.exec(threadCss)![1];
    const twin = threadCss.split("\n").filter((l) => l.startsWith(':root[data-motion="reduce"]'));
    const norm = (l: string) => l.replace(':root[data-motion="reduce"] ', "").trim();
    expect(twin.map(norm)).toEqual(media.split("\n").map((l) => l.trim()).filter(Boolean));
    expect(twin.map(norm)).toContain('.thr-trace[data-play="true"]::before { animation: none; }');
  });

  // Kills: a node drawn as a disc, or as this task's own shape instead of task 29's glyph.
  it("draws the node as a 5px ring", () => {
    render(node("c-1"));
    const svg = screen.getByRole("link").querySelector("svg")!;
    expect(svg.getAttribute("data-glyph")).toBe("mark:thread_node");
    expect(svg.getAttribute("width")).toBe("16");
    // Radius 2 at a 1px stroke: 4 + 1 = 5px across (task 29's path, 02 C2 thread.node).
    expect(svg.querySelector("path")!.getAttribute("d")).toMatch(/a2 2 0 1 0/);
    expect(rule(threadCss, ".thr-node")).not.toMatch(/fill|background/);
  });

  // Kills: a thread erased under Windows contrast themes (forced colours repaint backgrounds).
  it("keeps the thread under forced colours", () => {
    const at = forcedCss.indexOf("/* Task 36: the thread.");
    expect(at).toBeGreaterThan(-1);
    const block = forcedCss.slice(at);
    expect(block).toContain("@media (forced-colors: active)");
    expect(rule(block, ".thr-line::after")).toContain("background: Highlight");
    expect(block).toMatch(/\.thr-trace::before \{\s*forced-color-adjust: none;\s*background: CanvasText;/);
    expect(rule(block, '.thr-node[data-lit="true"] .gl-mark-node')).toContain("color: Highlight");
  });
});
```

In the forced block `.thr-line::before` shares its rule with `.thr-trace::before`, so test 8 reads
that pair by its second selector with a regex. `userEvent.hover` fires `pointerover` as well as
`pointerenter`, which is what React's `onPointerEnter` listens for; `user.tab()` reaches an `a[href]`
in jsdom. Neither was run while this plan was written (**UNVERIFIED** until step 1's red run).

#### Tests to write first — appended to ⚑ `desktop/src/styles/tokens.test.ts`

```ts
describe("the thread tokens (task 36)", () => {
  // Kills: paper's thread left at graphite's alpha (1.52:1, invisible without grain), and a lit
  // line no brighter than the unlit one. luminance() refuses an alpha value, so an alpha left on
  // either paper token fails here by throwing.
  it("gives paper's thread full strength", () => {
    const [thread, lit, page] = ["--thread", "--thread-lit", "--page"].map((n) => valueOf(PAPER, n)!);
    expect(contrast(thread, page)).toBeGreaterThanOrEqual(3);
    expect(contrast(lit, page)).toBeGreaterThan(contrast(thread, page));
    // Graphite's lit line is its unlit line's colour at a higher alpha (decision 7).
    const base = (v: string) => v.replace(/\s*\/\s*[\d.]+\s*\)$/, ")");
    expect(base(valueOf(ROOT, "--thread-lit")!)).toBe(base(valueOf(ROOT, "--thread")!));
  });
});
```

`valueOf`, `contrast`, `ROOT` and `PAPER` are task 26's (its part, `:238-270`).

