### Task 29: drawn glyphs, part 2: agent shapes, selection, needs-you, `thread.node`; the licence check

Stream **WD1**, completes seam **SW.12** (`<AgentGlyph/>`, `<MarkGlyph/>`), about 3 hours (an
estimate, not a measurement). Depends on task 28 (`paths.ts`, `glyphs.css`, `GlyphSize`). No Hub
dependency.

Task 28 gave issues their shapes. This task gives the other two vocabularies theirs, **agents**
and **marks**, and it fixes a defect in the built rail on the way, because the Work lens and the
rail must speak one vocabulary or a reader learns two.

**The defect.** `StatusDot.tsx:18-27` maps `running` and `idle` both to `dot-disc`. They differ
by colour (`--run` against `--idle`) and by the breath. Under `prefers-reduced-motion` the breath
stops, and the built answer is a static outer halo on `.dot-live` (`common.css:108-123`), which a
reader must learn means *running*. The `tokens.css:100-105` comment records the gap on purpose
("the shape vocabulary does NOT separate idle from RUNNING — both are discs"). 05 §4.4 C7 closes
it with a shape: **running is a disc inside a 1px ring**, so running and idle differ with colour,
motion and the halo all removed.

**The agent vocabulary** (05 §4.4 C7; rule 1), one shape each, colour on top:

| Status | Shape | Colour | Breathes |
|---|---|---|---|
| `starting` | an open ring | `--idle` (ink) | no |
| `running` | a disc inside a 1px ring, a gap between | `--run` | the disc, on the one heartbeat |
| `idle` | a plain disc | `--idle` | no |
| `exited` | a square | `--exited` | no |
| `failed` | a square with its top-right corner cut | `--fail` | no |

`failed` is drawn only where a caller passes it, and a caller passes it only where the wire
carries an **observed** failure (a bound lane failed, 05 §4.4 rule 1). The component never infers
it, and the built wire has no such value (`SessionStatus` is four statuses, `store/types.ts:134`).
Starting is ink, not accent: the accent means *you can act here* (tokens rule 2), and a machine
starting is not something you act on. The built `StatusDot` keeps its `--accent` starting tone;
this task changes only its running shape, so the rail's colour vocabulary is untouched.

**The marks** (05 §4.1, §4.4 rule 4; 02 C2):
- **`selected`** is a leading check, because *lit is not selected* (05 §4.1): a row lit by the
  thread and a row the user selected must not look alike, and a ring is what `thread.lit` uses.
- **`needs_you`** is a shape in `--need`, so an escalation is never amber alone (rule 4; a
  Windows contrast theme repaints amber and must still show the mark).
- **`thread_node`** is the 5px ring at `--e-hair` geometry (1px) in `--thread`, `tokens.css:57`,
  and in `--accent-ring` when `lit` (02 C2 `thread.lit`: lighting is interactive state, so the
  accent is legal). Task 36's `ThreadNode` renders it.

**The breath.** A running glyph's disc breathes on the one heartbeat: `beatDelayStyle()`
(`desktop/src/hooks/heartbeat.ts:23`) on the disc, and the `--beat` period, so a hundred running
glyphs and the rail's dots are at the same point of the same breath. No component starts a timer.
The live part is `aria-hidden`; the glyph's `label` is its accessible text. `StatusDot.tsx:2` says
it is "the ONLY component that may call beatDelayStyle()"; that sentence becomes "StatusDot and
AgentGlyph are the only components…", and nothing else changes in that rule.

**The licence check** (05 §5.3 row 9; 02 C6). W1 adds **no icon family**: every glyph, including
the prototype's `i-*` icons when a later task needs them, is drawn by us. `licence.test.ts` holds
the rest of the dependency tree to `OD-11`'s list: MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC
and OFL-1.1, an SPDX `OR` passing if one alternative does and an `AND` only if all do.

**Two corrections to the scope, made here so nobody re-derives them:**

1. **The licence test reads `package-lock.json`, not each dependency's `package.json`.** A
   `node:fs` read fails the typecheck (no `@types/node`; `designsystem.test.ts:8-12`), and a
   dependency's own `package.json` is often outside its `exports` map, so it cannot be imported.
   npm's lockfile (version 3 here) copies each package's `license` field into
   `packages["node_modules/<name>"].license`, and it is imported as `?raw`. All eighteen direct
   dependencies were checked while this plan was written: every one is MIT, Apache-2.0, or an
   `OR` of the two.
2. **Two existing `StatusDot` tests change, and the scope did not say so.** The case table at
   `StatusDot.test.tsx:15-24` pins `["running", "dot-disc"]`, and the test at `:67-90` pins the
   *premise* that running and idle share a shape. Both are rewritten below, not deleted: the first
   pins the new shape, the second pins that the shapes differ.

**Files** (the skeleton's file map omits the three `common/` files and `tokens.css`; a gap, named
in the return):
- `desktop/src/components/glyphs/AgentGlyph.tsx` — `AgentStatus`, `AgentGlyph` (new)
- `desktop/src/components/glyphs/MarkGlyph.tsx` — `MarkKind`, `MarkGlyph` (new)
- `desktop/src/components/glyphs/AgentGlyph.test.tsx`, `MarkGlyph.test.tsx` (new)
- `desktop/src/components/glyphs/licence.test.ts` — the dependency licence check (new)
- `desktop/src/components/glyphs/paths.ts` ⚑ (task 28) — `AGENT_PATHS`, `MARK_PATHS` appended (modified)
- `desktop/src/components/glyphs/glyphs.css` ⚑ (task 28) — `.gl-agent-*`, `.gl-mark-*` appended (modified)
- `desktop/src/components/common/StatusDot.tsx` ⚑ (64 lines; the shape map at `:18-27`,
  `FLASH_MS` at `:29`, the one-caller sentence at `:2`) — `running` → `dot-run` (modified)
- `desktop/src/components/common/common.css` ⚑ (179 lines; `.dot` at `:37-47`, the shapes at
  `:50-62`, the reduced-motion block at `:108-123`) — `.dot-run`, one reduced-motion rule, the
  comment (modified)
- `desktop/src/components/common/StatusDot.test.tsx` ⚑ (90 lines) — two tests rewritten, one added
- `desktop/src/styles/tokens.css` ⚑ — the comment at `:100-105` only; no value changes (modified)

#### Interfaces — complete: `AgentGlyph.tsx`

```tsx
import type { JSX } from "react";
import { beatDelayStyle } from "../../hooks/heartbeat";
import { AGENT_PATHS } from "./paths";
import type { GlyphSize } from "./StateGlyph";
import "./glyphs.css";

/** SW.12. An agent's status as the Work layer shows it (05 §4.4 C7). The built wire carries
 *  starting, running, idle and exited; `failed` is passed only for an OBSERVED failure (a bound
 *  lane failed, 05 §4.4 rule 1) and is never inferred here or by a caller from silence. */
export type AgentStatus = "starting" | "running" | "idle" | "exited" | "failed";

export function AgentGlyph(props: { status: AgentStatus; size?: GlyphSize; label: string }): JSX.Element {
  const { status, size = 16, label } = props;
  const live = status === "running";
  return (
    <svg
      className={`gl gl-${size} gl-agent gl-agent-${status}`}
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={label}
      data-glyph={`agent:${status}`}
      focusable="false"
    >
      {AGENT_PATHS[size][status].map((s) => (
        <path
          key={s.d}
          d={s.d}
          className={s.part === "fill" && live ? "gl-agent-fill gl-agent-live" : `gl-agent-${s.part}`}
          style={s.part === "fill" && live ? beatDelayStyle() : undefined}
          aria-hidden={s.part === "fill" && live ? true : undefined}
        />
      ))}
    </svg>
  );
}
```

Each part is `ring` (stroked, no fill) or `fill` (solid, no stroke); only a running glyph's fill
part, its core disc, breathes. The whole `<svg>` is the image and its `label` is its text; the
breathing core carries
`aria-hidden` as well, so no assistive technology reading the tree child by child announces a
pulse. The component calls `beatDelayStyle()` once per render and never subscribes to the
heartbeat, so a running glyph re-renders only when its props change.

#### Interfaces — complete: `MarkGlyph.tsx`

```tsx
import type { JSX } from "react";
import { MARK_PATHS } from "./paths";
import type { GlyphSize } from "./StateGlyph";
import "./glyphs.css";

/** SW.12. Three marks that are not states: a selection, an escalation, a thread node. */
export type MarkKind = "selected" | "needs_you" | "thread_node";

const MARK_CLASS: Record<MarkKind, string> = {
  selected: "gl-mark-selected",
  needs_you: "gl-mark-need",
  thread_node: "gl-mark-node",
};
const MARK_LABEL: Record<MarkKind, string> = {
  selected: "Selected",
  needs_you: "Needs you",
  thread_node: "Provenance thread",
};

export function MarkGlyph(props: { kind: MarkKind; size?: GlyphSize; lit?: boolean }): JSX.Element {
  const { kind, size = 16, lit = false } = props;
  const classes = ["gl", `gl-${size}`, "gl-mark", MARK_CLASS[kind]];
  // Lit means "this carries the thread you are pointing at" (02 C2); only a node can be lit.
  if (lit && kind === "thread_node") classes.push("gl-mark-lit");
  return (
    <svg
      className={classes.join(" ")}
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={MARK_LABEL[kind]}
      data-glyph={`mark:${kind}`}
      focusable="false"
    >
      {MARK_PATHS[size][kind].map((s) => <path key={s.d} d={s.d} />)}
    </svg>
  );
}
```

A mark has no `label` prop because each means one thing. Its caller adds the words the mark
stands beside: "Needs you: approve the migration" is the row's text, and the mark is its shape.

#### Interfaces — complete: appended to `paths.ts`

`AgentStatus` and `MarkKind` join the file's type imports. Every path below was drawn with the
issue set, from the same clean-room rule, and shares no `d` string with it (a test holds that).

```ts
/** One part of an agent glyph: a stroked ring, or a solid fill. */
export interface AgentPart { d: string; part: "ring" | "fill" }

export const AGENT_PATHS: Record<GlyphSize, Record<AgentStatus, readonly AgentPart[]>> = {
  16: {
    starting: [{ part: "ring", d: "M3.5 8a4.5 4.5 0 1 0 9 0a4.5 4.5 0 1 0-9 0" }],
    running:  [{ part: "ring", d: "M2.75 8a5.25 5.25 0 1 0 10.5 0a5.25 5.25 0 1 0-10.5 0" },  // 1px, gap 1.75
               { part: "fill", d: "M5 8a3 3 0 1 0 6 0a3 3 0 1 0-6 0" }],
    idle:     [{ part: "fill", d: "M4 8a4 4 0 1 0 8 0a4 4 0 1 0-8 0" }],
    exited:   [{ part: "fill", d: "M5 4h6a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1Z" }],
    failed:   [{ part: "fill", d: "M5 4h4l3 3v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1Z" }],
  },
  14: {
    starting: [{ part: "ring", d: "M3.1 7a3.9 3.9 0 1 0 7.8 0a3.9 3.9 0 1 0-7.8 0" }],
    running:  [{ part: "ring", d: "M2.4 7a4.6 4.6 0 1 0 9.2 0a4.6 4.6 0 1 0-9.2 0" },          // 1px, gap 1.5
               { part: "fill", d: "M4.4 7a2.6 2.6 0 1 0 5.2 0a2.6 2.6 0 1 0-5.2 0" }],
    idle:     [{ part: "fill", d: "M3.5 7a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0-7 0" }],
    exited:   [{ part: "fill", d: "M4.4 3.5h5.2a.9.9 0 0 1 .9.9v5.2a.9.9 0 0 1-.9.9H4.4a.9.9 0 0 1-.9-.9V4.4a.9.9 0 0 1 .9-.9Z" }],
    failed:   [{ part: "fill", d: "M4.4 3.5h3.6l2.5 2.5v3.6a.9.9 0 0 1-.9.9H4.4a.9.9 0 0 1-.9-.9V4.4a.9.9 0 0 1 .9-.9Z" }],
  },
};

export const MARK_PATHS: Record<GlyphSize, Record<MarkKind, readonly Stroke[]>> = {
  16: {
    selected:    [{ d: "M3.75 8.5 6.5 11.25 12.25 5" }],
    needs_you:   [{ d: "M8 3.25v6M8 12.4v.2" }],        // a bar and a round-capped dot: "!"
    thread_node: [{ d: "M6 8a2 2 0 1 0 4 0a2 2 0 1 0-4 0" }],  // 5px across at a 1px stroke
  },
  14: {
    selected:    [{ d: "M3.25 7.4 5.7 9.85 10.75 4.4" }],
    needs_you:   [{ d: "M7 2.75v5.25M7 10.9v.2" }],
    thread_node: [{ d: "M5 7a2 2 0 1 0 4 0a2 2 0 1 0-4 0" }],  // the node is 5px at both sizes
  },
};
```

The running ring's gap (ring inner edge to core) is 1.75 units at 16px and 1.5 at 14px, so the
two parts never touch at either size; that is what makes it a ring round a disc and not a thick
disc. The failed square's cut is its top-right corner, 3 units at 16px and 2.5 at 14px.

#### Interfaces — complete: appended to `glyphs.css`

```css
/* ── Agents (Phase W task 29; 05 §4.4 C7) ─────────────────────────────────────────────
 * Shape first, colour on top: each status is told apart with colour removed. These are the
 * only rules in this file, with .gl-mark-*, allowed to name a status colour. */
.gl-agent-ring {
  fill: none;
  stroke: currentColor;
}

.gl-16 .gl-agent-ring {
  stroke-width: var(--icon-stroke);
}

.gl-14 .gl-agent-ring {
  stroke-width: var(--icon-sm-stroke);
}

/* 05 §4.4 C7: running's ring is 1px at both sizes. After the two above, same specificity. */
.gl-agent-running .gl-agent-ring {
  stroke-width: 1px;
}

.gl-agent-fill {
  fill: currentColor;
  stroke: none;
}

.gl-agent-starting,
.gl-agent-idle {
  color: var(--idle);
}

.gl-agent-running {
  color: var(--run);
}

.gl-agent-exited {
  color: var(--exited);
}

.gl-agent-failed {
  color: var(--fail);
}

/* The one heartbeat: the period is the token, the phase is beatDelayStyle()'s inline delay.
 * opacity only, so it stays on the compositor. Same curve and values as common.css's beat. */
@keyframes gl-beat {
  0%,
  100% {
    opacity: 0.45;
  }
  50% {
    opacity: 1;
  }
}

.gl-agent-live {
  animation: gl-beat var(--beat) var(--ease) infinite;
}

/* The audit's stand-in (designsystem.test.ts:693-708). The shape already separates running
 * from idle, so reduced motion only needs the breath to rest at full ink, never mid-fade. */
@media (prefers-reduced-motion: reduce) {
  .gl-agent-live {
    opacity: 1;
  }
}

/* ── Marks (Phase W task 29; 05 §4.1, §4.4 rule 4; 02 C2) ─────────────────────────── */
.gl-mark path {
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.gl-16.gl-mark path {
  stroke-width: var(--icon-stroke);
}

.gl-14.gl-mark path {
  stroke-width: var(--icon-sm-stroke);
}

/* The selection check takes its row's ink: selection is a shape, not a colour (05 §4.1). */

.gl-mark-need {
  color: var(--need);
}

/* --e-hair geometry: a 1px ring at both sizes (02 C2 thread.node). (0,2,1), after the two size
 * rules, so it wins over them by source order at equal specificity. */
.gl-mark.gl-mark-node path {
  stroke-width: 1px;
}

.gl-mark-node {
  color: var(--thread);
}

/* thread.lit is interactive state, so the accent is legal (02 C2; tokens rule 2). */
.gl-mark-node.gl-mark-lit {
  color: var(--accent-ring);
}
```

A bare `.gl-mark-node path` would be (0,1,1) and lose to `.gl-16.gl-mark path` (0,2,1), which is
why the node rule names both classes. The design-system audit walks this file too: the token
import is task 28's first line, no hex, no named colour, and `.gl-agent-live` has its stand-in.

#### Interfaces — complete: the rail's dot (`StatusDot.tsx`, `common.css`, `tokens.css`)

`StatusDot.tsx:18-27`, the shape map, becomes (one union member and one row change):

```ts
const SHAPE: Record<DotStatus, "dot-ring" | "dot-disc" | "dot-run" | "dot-square"> = {
  starting: "dot-ring",
  provisioning: "dot-ring",
  sleeping: "dot-ring",
  running: "dot-run",      // Phase W task 29: a disc inside a 1px ring, AgentGlyph's geometry
  idle: "dot-disc",
  exited: "dot-square",
  stopped: "dot-square",
  error: "dot-square",
};
```

`:2` becomes `// phase when live. This and glyphs/AgentGlyph.tsx are the ONLY components that may
call beatDelayStyle().` `FLASH_MS = 600` (`:29`), the flash effect and the contract comment at
`:9-15` are unchanged. `running` is in both `SessionStatus` and `MachineState`, so a running
machine gets the ring too, which is the point: one shape means *running*, wherever it is.

`common.css`, inserted after `.dot-square` (`:58-62`):

```css
/* Running: a disc inside a 1px ring (05 §4.4 C7; Phase W task 29), the same geometry as
   AgentGlyph's running glyph, so the rail and the Work lens speak one vocabulary. The element is
   the 4px disc; the ring is an outline 1px off it, so the gap is whatever is behind the dot and
   this file never guesses a background. 4 + 2 × (1 + 1) = the 8px box every other dot fills. */
.dot-run {
  inline-size: 4px;
  block-size: 4px;
  margin: 2px;
  border-radius: 50%;
  outline: 1px solid currentColor;
  outline-offset: 1px;
}
```

Inside the existing reduced-motion block (`:119-123`), after `.dot-live`:

```css
  /* A running dot is already a ring round a disc; the halo would fill its gap. */
  .dot-run.dot-live {
    box-shadow: none;
  }
```

The comment above that block (`:108-118`) is replaced by:

```css
/* REDUCED MOTION MUST NOT COLLAPSE THE STATUS VOCABULARY TO COLOUR.
   Since Phase W task 29 it cannot: running is .dot-run (a disc inside a ring) and idle is
   .dot-disc, so shape separates every pair in every motion mode. The static OUTER ring on
   .dot-live stays, as a second difference for any live dot that is not running, and it is
   switched off on .dot-run, whose ring it would fill. Non-inset ON PURPOSE, so it reads
   differently from .dot-ring's inset ring. Pinned by StatusDot.test.tsx's ?raw assertion. */
```

`tokens.css:102-104`, the sentence that records the gap, becomes: `Running is a disc inside a 1px
ring (dot-run, Phase W task 29), so running and idle are separated by shape as well as by colour
(--ink-3 against --run) and by motion (idle is still, running breathes on the --beat clock).`
The lines around it, and every value in the file, are unchanged.

An `outline` that follows `border-radius` needs Chromium 94 or Safari 16.4. WebView2 is evergreen
Chromium; the WKWebView floor on macOS is **UNVERIFIED** here (`tokens.css:7-8` states Safari
15.4 for OKLCH). On an older WKWebView the ring draws square round a round disc: still a second
shape, so running and idle stay apart, and task 33's sheet is where anyone would see it.

#### Tests to write first — `desktop/src/components/glyphs/AgentGlyph.test.tsx`

```tsx
import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import glyphsCss from "./glyphs.css?raw";
import { AgentGlyph } from "./AgentGlyph";
import type { AgentStatus } from "./AgentGlyph";
import { AGENT_PATHS, MARK_PATHS, STATE_PATHS } from "./paths";

const STATUSES: AgentStatus[] = ["starting", "running", "idle", "exited", "failed"];
const draw = (status: AgentStatus, size: 14 | 16 = 16): SVGSVGElement =>
  render(<AgentGlyph status={status} size={size} label={`agent ${status}`} />).container.querySelector("svg")!;
/** The drawing with colour removed: each part's kind and geometry, and nothing else. */
const shape = (svg: Element): string =>
  [...svg.querySelectorAll("path")]
    .map((p) => `${p.classList.contains("gl-agent-ring") ? "ring" : "fill"}:${p.getAttribute("d")}`)
    .join(";");
const allD = (table: Record<number, Record<string, ReadonlyArray<{ d: string }>>>): string[] =>
  Object.values(table).flatMap((set) => Object.values(set).flatMap((parts) => parts.map((p) => p.d)));

describe("AgentGlyph", () => {
  // Kills: today's colour-only difference (StatusDot.tsx:22-23, both dot-disc).
  it("gives running and idle different shapes", () => {
    expect(shape(draw("running"))).not.toBe(shape(draw("idle")));
    expect([...draw("running").querySelectorAll("path")].map((p) => p.classList[0]))
      .toEqual(["gl-agent-ring", "gl-agent-fill"]);
  });

  // Kills: any pair told apart by colour alone, at either size.
  it("separates all five statuses with colour removed", () => {
    for (const size of [14, 16] as const) {
      expect(new Set(STATUSES.map((s) => shape(draw(s, size)))).size, `size ${size}`).toBe(5);
    }
  });

  // Kills: a per-row timer, and a breath with its own period.
  it("breathes on the one heartbeat", () => {
    // setInterval only: React's scheduler may use timers of its own, and useHeartbeat's one
    // interval (heartbeat.ts:34) is the mechanism a per-row clock would have to reach for.
    const interval = vi.spyOn(globalThis, "setInterval");
    const live = Array.from({ length: 20 }, () => draw("running").querySelector(".gl-agent-live"));
    expect(interval).not.toHaveBeenCalled();
    for (const el of live) expect((el as SVGElement).style.animationDelay).toMatch(/^-\d+ms$/);
    expect(glyphsCss).toMatch(/\.gl-agent-live\s*\{\s*animation:\s*gl-beat var\(--beat\) var\(--ease\) infinite;/);
    for (const s of STATUSES.filter((x) => x !== "running")) {
      expect(draw(s).querySelector(".gl-agent-live"), s).toBeNull();
    }
  });

  // Kills: an announced breath.
  it("hides liveness from assistive technology", () => {
    const svg = draw("running");
    expect(svg.getAttribute("role")).toBe("img");
    expect(svg.getAttribute("aria-label")).toBe("agent running");
    expect(svg.querySelector(".gl-agent-live")?.getAttribute("aria-hidden")).toBe("true");
    expect(svg.querySelector("[aria-live]")).toBeNull();
  });

  // Kills: one icon family for issue and agent states (05 §4.4 work rule 1).
  it("shares no path with the issue glyphs", () => {
    const issue = new Set(allD(STATE_PATHS));
    expect(allD(AGENT_PATHS).filter((d) => issue.has(d))).toEqual([]);
    expect(allD(MARK_PATHS).filter((d) => issue.has(d))).toEqual([]);
  });
});
```

`STATE_PATHS` entries carry `dash` as well as `d`; `allD` reads `d` only, so a dashed ring and a
solid ring with one `d` would still count as shared, which is the stricter reading.

#### Tests to write first — `desktop/src/components/glyphs/MarkGlyph.test.tsx`

```tsx
import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import glyphsCss from "./glyphs.css?raw";
import { MarkGlyph } from "./MarkGlyph";
import type { MarkKind } from "./MarkGlyph";
import { MARK_PATHS } from "./paths";
import { stripCssComments } from "../../test/fakes";

const draw = (kind: MarkKind, lit = false): SVGSVGElement =>
  render(<MarkGlyph kind={kind} lit={lit} />).container.querySelector("svg")!;
const css = stripCssComments(glyphsCss);
const rule = (selector: string): string =>
  new RegExp(`${selector.replace(/[.]/g, "\\.")}\\s*\\{([^}]*)\\}`).exec(css)?.[1] ?? "";

describe("MarkGlyph", () => {
  // Kills: selection drawn as a ring alone (lit is not selected, 05 §4.1).
  it("draws selection as a check", () => {
    const d = draw("selected").querySelector("path")?.getAttribute("d") ?? "";
    expect(d).toBe(MARK_PATHS[16].selected[0].d);
    expect(d).not.toMatch(/[aAzZ]/);                 // an open polyline: no arc, never closed
    expect(draw("selected").classList.contains("gl-mark-node")).toBe(false);
  });

  // Kills: an escalation shown by amber alone (05 §4.4 rule 4).
  it("gives needs-you a shape", () => {
    const paths = [...draw("needs_you").querySelectorAll("path")];
    expect(paths.length).toBeGreaterThan(0);
    expect(paths.map((p) => p.getAttribute("d"))).toEqual([MARK_PATHS[16].needs_you[0].d]);
    expect(MARK_PATHS[16].needs_you[0].d).not.toBe(MARK_PATHS[16].selected[0].d);
    expect(rule(".gl-mark-need")).toContain("color: var(--need)");
  });

  // Kills: a thread node drawn as a filled disc, and lighting that is not the accent ring.
  it("draws thread_node as a ring, not a disc", () => {
    expect(rule(".gl-mark path")).toContain("fill: none");
    expect(rule(".gl-mark.gl-mark-node path")).toContain("stroke-width: 1px");
    expect(draw("thread_node").querySelector("path")?.getAttribute("d")).toMatch(/a2 2 0 1 0/);
    expect(draw("thread_node", true).classList.contains("gl-mark-lit")).toBe(true);
    expect(rule(".gl-mark-node.gl-mark-lit")).toContain("color: var(--accent-ring)");
    expect(draw("selected", true).classList.contains("gl-mark-lit")).toBe(false);
  });

  // Kills: an unlabelled mark.
  it("names every mark for assistive technology", () => {
    const labels = (["selected", "needs_you", "thread_node"] as const).map((k) => draw(k).getAttribute("aria-label"));
    expect(labels).toEqual(["Selected", "Needs you", "Provenance thread"]);
  });
});
```

#### Tests to write first — `StatusDot.test.tsx` (two rewritten, one added)

In the case table at `:15-24`, `["running", "dot-disc"]` becomes `["running", "dot-run"]`. The
test at `:67-90` is replaced whole:

```tsx
  // Kills: the rail keeping the old shape, and the reduced-motion halo filling the new ring.
  it("separates running from idle by shape in every motion mode", () => {
    const shapeOf = (el: HTMLElement): string | undefined =>
      /dot-(disc|ring|run|square)/.exec(el.className)?.[0];
    const { unmount } = render(<StatusDot status="running" live label="running" />);
    const runningShape = shapeOf(dot());
    unmount();
    render(<StatusDot status="idle" live={false} label="idle" />);
    expect(runningShape).toBe("dot-run");
    expect(shapeOf(dot())).toBe("dot-disc");

    const block = /@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{([\s\S]*?)\n\}/.exec(commonCss);
    expect(block).not.toBeNull();
    expect(block![1]).toMatch(/box-shadow:\s*0 0 0 1\.5px currentColor/);   // the halo stays
    expect(block![1]).toMatch(/\.dot-run\.dot-live\s*\{\s*box-shadow:\s*none;/);
  });
```

And one test is added after it:

```tsx
  // Kills: dot-run drawn as anything but a disc inside a ring off it.
  it("draws running as dot-run", () => {
    render(<StatusDot status="running" live={false} label="running" />);
    expect(dot().className).toContain("dot-run");
    expect(dot().className).not.toContain("dot-disc");
    const run = /\.dot-run\s*\{([^}]*)\}/.exec(commonCss)?.[1] ?? "";
    expect(run).toContain("outline: 1px solid currentColor");
    expect(run).toContain("outline-offset: 1px");
    expect(run).toContain("border-radius: 50%");
  });
```

#### Tests to write first — `desktop/src/components/glyphs/licence.test.ts`

```ts
// 05 §5.3 row 9; 02 C6; OD-11. Every direct dependency of the desktop app, and every bundled
// face, is under a licence on OD-11's list. Read from the lockfile: npm copies each package's
// own `license` field into packages["node_modules/<name>"].license, and ?raw needs no node:fs.
import { describe, expect, it } from "vitest";
import pkgRaw from "../../../package.json?raw";
import lockRaw from "../../../package-lock.json?raw";

/** Task 27's manifest, if it has landed. A glob, so this file does not depend on task 27. */
const FONT_LOCKS = import.meta.glob<Record<string, { licence: string }>>(
  "/src/assets/fonts/fonts.lock.json", { eager: true, import: "default" },
);

const ALLOWED = new Set(["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "OFL-1.1"]);

/** SPDX, the subset this tree uses. An OR passes if one alternative does; an alternative passes
 *  only if every AND term is allowed. Parentheses, WITH and anything unreadable FAIL CLOSED: a
 *  person reads that licence and extends this function, rather than a regex guessing. */
function permitted(expr: string | undefined): boolean {
  if (expr === undefined || expr.trim() === "" || /[()]|\bWITH\b/.test(expr)) return false;
  return expr.split(/\s+OR\s+/).some((alt) => alt.split(/\s+AND\s+/).every((t) => ALLOWED.has(t.trim())));
}

const pkg = JSON.parse(pkgRaw) as { dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
const lock = JSON.parse(lockRaw) as { lockfileVersion: number; packages: Record<string, { license?: string }> };
const DIRECT = [...Object.keys(pkg.dependencies ?? {}), ...Object.keys(pkg.devDependencies ?? {})].sort();

describe("the licence check", () => {
  // Kills: a vacuous pass over an empty or unreadable lockfile.
  it("reads every direct dependency from a version-3 lockfile", () => {
    expect(lock.lockfileVersion).toBeGreaterThanOrEqual(3);
    expect(DIRECT.length).toBeGreaterThan(10);
    expect(DIRECT.filter((n) => lock.packages[`node_modules/${n}`] === undefined)).toEqual([]);
  });

  // Kills: a GPL, AGPL or commercial dependency.
  it("allows only permissive licences", () => {
    const refused = DIRECT
      .map((n) => [n, lock.packages[`node_modules/${n}`]?.license] as const)
      .filter(([, licence]) => !permitted(licence))
      .map(([n, licence]) => `${n}  ${licence ?? "<no licence field>"}`);
    expect(refused).toEqual([]);
  });

  // Kills: a parser that lets a copyleft alternative or an unreadable expression through.
  it("reads SPDX the strict way", () => {
    expect(permitted("MIT")).toBe(true);
    expect(permitted("Apache-2.0 OR MIT")).toBe(true);
    expect(permitted("MIT OR GPL-3.0-only")).toBe(true);        // one allowed alternative suffices
    expect(permitted("MIT AND GPL-3.0-only")).toBe(false);
    expect(permitted("AGPL-3.0-only")).toBe(false);
    expect(permitted("LicenseRef-Commercial")).toBe(false);
    expect(permitted("(MIT OR Apache-2.0)")).toBe(false);       // fails closed
    expect(permitted("Apache-2.0 WITH LLVM-exception")).toBe(false);
    expect(permitted(undefined)).toBe(false);
  });

  // Kills: an icon family added in W1 (every glyph is drawn by us, 02 C6).
  it("adds no icon family", () => {
    expect(DIRECT.filter((n) => /icon|lucide|heroicons|phosphor|tabler|feather|remix/i.test(n))).toEqual([]);
  });

  // Kills: a bundled face under a licence outside the list (task 27's manifest, once it exists).
  it("holds every bundled face to the same list", () => {
    for (const manifest of Object.values(FONT_LOCKS)) {
      for (const face of Object.values(manifest)) expect(permitted(face.licence)).toBe(true);
    }
  });
});
```

The eighteen direct dependencies at `f7ea2a2`, read from `desktop/package-lock.json` while this
plan was written: `@tauri-apps/api`, `@tauri-apps/cli` (`Apache-2.0 OR MIT`),
`@tauri-apps/plugin-opener` (`MIT OR Apache-2.0`), `typescript` (`Apache-2.0`), and fourteen `MIT`.
The check passes today; its job is the next dependency.

#### Steps

1. Write the four test files and the `StatusDot.test.tsx` edits. Run `npm --prefix desktop test
   -- glyphs StatusDot`: the new files fail to import; the two rewritten StatusDot tests fail on
   `dot-disc`; the licence test passes already (it is a guard, and the tree is clean).
2. Append `AGENT_PATHS` and `MARK_PATHS` to `paths.ts`; write `AgentGlyph.tsx` and `MarkGlyph.tsx`;
   append the agent and mark rules to `glyphs.css`. Re-run: the glyph tests pass.
3. Change `StatusDot.tsx`'s map and its `:2` sentence; add `.dot-run` and the reduced-motion rule
   to `common.css` with its new comment; replace the `tokens.css` sentence. Re-run: all pass.
4. Mutate by hand and watch the named test go red, then revert: map `running` back to `dot-disc`;
   give `idle` running's two parts; add a `setInterval` to `AgentGlyph`; drop the core's
   `aria-hidden`; reuse `RING16` as running's ring; set the selection mark to the node ring; drop
   `.gl-mark-need`'s geometry (an empty `d`); change `permitted` to `alt.includes("MIT")`; add
   `"lucide-react"` to `devDependencies` (then remove it and restore the lockfile).
5. Run the full desktop suite: the design-system audit walks `glyphs.css` (the new loop has its
   stand-in; no named colour; no gradient) and `common.css` (the audit's `.dot-live` canary at
   `designsystem.test.ts:706` still finds its loop). Typecheck; commit.

#### Acceptance

```bash
npm --prefix desktop test -- glyphs StatusDot
# expect: AgentGlyph 5, MarkGlyph 4, licence 5, StateGlyph 8, StatusDot 6 — all passed

npm --prefix desktop test -- designsystem
# expect: all pass; the loop list now includes "src/components/glyphs/glyphs.css  .gl-agent-live"

npm --prefix desktop run typecheck
# expect: exit 0
```

StatusDot's six are the five built tests (`:14`, `:35`, `:41`, `:51`, `:67`; the first has one
table row changed and the last is rewritten) plus the one added. The implementer reports the
numbers the run prints; the plan's counts are not the evidence.

#### What this task does not claim

- **Not AX's seven statuses.** `docs/AGENTIC-UX.md` lists seven; the built wire has four
  (`StatusDot.tsx:9-15`, `store/types.ts:134`) and the Work layer adds only the observed `failed`.
- **Not the reduced-motion tick** (task 34), nor the forced-colors rendering of any glyph or dot.
- **Not a licence review by legal** (**UNVERIFIED**). The test reads what each package *declares*;
  a package that mis-declares its licence passes it.
- **Not that the lockfile is current.** The check reads `package-lock.json`; a dependency
  installed without updating it is invisible here, which is why the first test compares it with
  `package.json`.
- **Not that the new running dot reads well** in the rail at 125 % or 150 % (task 33), nor that
  an old WKWebView rounds the outline (**UNVERIFIED**; see above).
- **Not any Work surface using these glyphs.** Tasks 36 (`ThreadNode`), 40 and 46 are the first.
- Design: 05 §4.1, §4.4 (C6, C7, rules 1 and 4); 02 C2, C6. Parity: gives **L-184**'s states a
  shape each and **L-225**'s multi-select its check; it closes neither alone.

**Interfaces:**
- Produces (seam **SW.12**, completed):
  - `AgentStatus` (five); `AgentGlyph({ status, size?, label }): JSX.Element`
  - `MarkKind` (`selected`, `needs_you`, `thread_node`); `MarkGlyph({ kind, size?, lit? }): JSX.Element`
  - `paths.ts`: `AgentPart`, `AGENT_PATHS`, `MARK_PATHS`
  - `glyphs.css`: `.gl-agent-*` (ring, fill, the five statuses, live) and `.gl-mark-*` (selected,
    need, node, lit); `@keyframes gl-beat`
  - `StatusDot`: `running` → `dot-run`; `.dot-run` in `common.css`
- Consumes: task 28's `GlyphSize`, `Stroke`, `STATE_PATHS`, `glyphs.css`; `beatDelayStyle()`
  (`hooks/heartbeat.ts:23`); `--run`, `--fail`, `--need`, `--idle`, `--exited`, `--thread`,
  `--accent-ring`, `--beat`, `--ease`, `--icon-stroke`, `--icon-sm-stroke` (built or task 26)
- Amends: nothing in the Hub. Task 36's `ThreadNode` renders `MarkGlyph kind="thread_node"`.
