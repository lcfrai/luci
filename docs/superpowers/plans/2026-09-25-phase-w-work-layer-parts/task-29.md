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
    // interval (heartbeat.ts:37) is the mechanism a per-row clock would have to reach for.
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
