### Task 35: grain, measured at 150 % in WebView2

Stream **WD1**, about 2 hours (an estimate, not a measurement). Depends on task 31 (the chrome the
grain sits over). Uses task 32's flags and task 33's stage, both earlier by number. No Hub, no wire
shape.

The grain is a fixed 2.2 % noise overlay on the whole shell today: `.grain::after` at
`tokens.css:163-171` (its comment `:159-162`), applied once, by `AppShell.tsx:82`, pinned by
`AppShell.test.tsx:192-193` and `defects.test.tsx:193-201`. 05 §4.4 work rule 6 (07 D M6) says it
is **measured, not assumed**, before the prototype review: a full-window pseudo-element over a
scrolling list is a compositor cost at 150 % Windows scaling, and nobody has measured it.

This task moves the rule unchanged into its own file, gives it three off switches, and builds a
meter that runs **inside the real Tauri app in WebView2** at the laptop's own scaling. No browser
automation and no remote debugging port are needed, which matters: the corporate network blocks
headless screenshots (`CLAUDE.md`). The person running it copies two lines into the results
file. The numbers decide nothing on their own; the rule below says what they mean, and the owner
accepts them.

**The rule, recorded rather than asserted** (02 C8): if p95 with grain is within 1 ms of p95
without, **and** under 16.7 ms, grain stays. Otherwise the two lines go to the owner with the
proposal to turn grain off at a device pixel ratio of 1.5 and above.

**Four decisions, verified against the tree at `3e4efd1` and the written parts:**

1. **The forced-colours rule is task 34's, and there is one.** The scope has this file add "off
   under `forced-colors`" and says "one rule, not two". Task 34 lands first and already writes
   `.grain::after { display: none; }` in `forced-colors.css`, imported last. This file does not
   repeat it; a test counts one such rule in the tree.
2. **`grain.css` is imported from `app.css`,** after task 27's `fonts.css` import, so it loads with
   the ground and before any component sheet, as the rule does from `tokens.css` today. The three
   off rules are `:root`-prefixed, so they win over the base rule by specificity, not by order.
3. **`-hc` themes do not exist yet.** `ThemeName` is `"graphite" | "paper"` (task 32) and the
   high-contrast pair is W2 (task 33's *does not claim*). The rule
   `:root[data-theme$="-hc"] .grain::after` is written now so W2 cannot ship a high-contrast theme
   with grain, and the test holds it.
4. **The meter registers from `screens/grain.tsx`,** task 33's rule for dev screens, not from
   `GrainMeter.tsx` itself, which stays a plain component with pure, tested arithmetic.

**Files:**
- `desktop/src/styles/grain.css` — the moved rule and the off switches (new)
- `desktop/src/styles/grain.test.ts` (new)
- `desktop/src/styles/tokens.css` ⚑ (the grain rule and comment at `:159-171`) — both removed
  (modified)
- `desktop/src/styles/app.css` ⚑ (82 lines; `@import "./tokens.css";` at `:10`, task 27's
  `fonts.css` after it) — one `@import` (modified)
- `desktop/src/lib/flags.ts` ⚑ (tasks 32, 33) — `grain` (modified)
- `desktop/src/lib/flags.test.ts` ⚑ — the new flag in the two attribute tests (modified)
- `desktop/src/components/devtools/FlagCompare.tsx` ⚑ (task 32) — a fourth group (modified)
- `desktop/src/components/devtools/GrainMeter.tsx` — the meter (new; the skeleton's file map
  does not list it)
- `desktop/src/components/devtools/GrainMeter.test.tsx` (new)
- `desktop/src/components/devtools/screens/grain.tsx` — the registration (new)
- `desktop/src/components/devtools/screens/index.ts` ⚑ (task 33) — one import (modified)
- `docs/verification/2026-09-25-phase-w-results.md` — the *Grain* section, filled by hand (new;
  the skeleton gives this task no results file)

#### Interfaces — complete: `desktop/src/styles/grain.css`

The rule's six declarations are `tokens.css:164-170`, byte for byte; only the file changes.

```css
/* desktop/src/styles/grain.css
 *
 * Grain (Phase W task 35; moved unchanged from tokens.css). 1.5–3% film grain over dark
 * surfaces. It kills banding on the dark ramp and adds a made-object quality. Applied ONCE, on the
 * app shell (AppShell.tsx), as a fixed pseudo-element — not per component, which would cost a
 * compositor layer each. pointer-events:none so it never eats a click.
 *
 * Measured, not assumed (05 §4.4 work rule 6): GrainMeter scrolls a list with and without it at
 * the laptop's own scaling, and the numbers go in docs/verification. Grain is one of dark's marks
 * (05 §4.4 C13); paper carries the thread gutter instead. Off in three places below, and a fourth,
 * forced colours, is task 34's one rule in forced-colors.css. */
@import "./tokens.css";

.grain::after {
  content: "";
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
  opacity: 0.022;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='120' height='120'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='3'/%3E%3C/filter%3E%3Crect width='120' height='120' filter='url(%23n)'/%3E%3C/svg%3E");
}

/* Off on a light ground (paper has its own mark), in any high-contrast theme (W2 names them
 * *-hc), and when the dev flag turns it off for the meter's second pass. */
:root[data-theme="paper"] .grain::after,
:root[data-theme$="-hc"] .grain::after,
:root[data-grain="off"] .grain::after {
  display: none;
}
```

**Why paper too.** 05 §4.4 C13 (D M5): *grain is dark-only, so `paper` carries its own fifth
mark*. The scope named `-hc` and the flag; paper follows from the same sentence, and leaving grain
on over a light ground would be the rule the design already withdrew.

`app.css`, after task 27's `@import "./fonts.css";`:

```css
@import "./grain.css";
```

#### Interfaces — complete: the `flags.ts` and `FlagCompare.tsx` deltas

```ts
export type Grain = "on" | "off";

export interface Flags {
  // … task 32's three and task 33's two, unchanged
  /** "off" writes data-grain="off"; grain.css hides the overlay. GrainMeter's second pass. */
  grain: Grain;
}

// FLAG_DEFAULTS gains  grain: "on",
// ATTRIBUTE gains      grain: "data-grain",
```

```tsx
export const GRAINS: readonly FlagOption<Grain>[] = [
  { value: "on", label: "On" },
  { value: "off", label: "Off" },
];

// in FlagCompare, after the Theme group:
      <Group flag="grain" legend="Grain" options={GRAINS} />
```

`Grain` joins the type import from `../../lib/flags` in `FlagCompare.tsx`.

#### Interfaces — complete: `desktop/src/components/devtools/GrainMeter.tsx`

```tsx
// desktop/src/components/devtools/GrainMeter.tsx
//
// Grain, measured (Phase W task 35; 05 §4.4 work rule 6; 07 D M6). Scrolls a 500-row fixture by
// requestAnimationFrame for 5 s with grain, then 5 s without, records each frame's interval, and
// prints p50 and p95 with the device pixel ratio it ran at. Run it inside the Tauri app, in
// WebView2, at the laptop's own Windows scaling. The fixture is deliberately NOT virtualised: it
// is a load for the compositor, not a product list (DESIGN-SYSTEM's 50-row rule is for those).

import type { JSX } from "react";
import { useRef, useState } from "react";
import { getFlags, setFlag } from "../../lib/flags";

export const PASS_MS = 5000;
export const ROWS = 500;
export const STEP_PX = 12;

export interface FrameSummary {
  frames: number;
  p50: number;
  p95: number;
}

/** Nearest rank: the smallest recorded interval with at least p % of intervals at or below it. */
export function percentile(sorted: readonly number[], p: number): number {
  if (sorted.length === 0) return Number.NaN;
  const rank = Math.min(sorted.length, Math.max(1, Math.ceil((p / 100) * sorted.length)));
  return sorted[rank - 1];
}

export function summarise(intervals: readonly number[]): FrameSummary {
  const sorted = [...intervals].sort((a, b) => a - b);
  return { frames: sorted.length, p50: percentile(sorted, 50), p95: percentile(sorted, 95) };
}

/** 02 C8's rule, recorded rather than asserted: the owner accepts the numbers either way. */
export function grainRule(on: FrameSummary, off: FrameSummary): "grain stays" | "to the owner" {
  return on.p95 - off.p95 <= 1 && on.p95 < 16.7 ? "grain stays" : "to the owner";
}

export function resultLine(pass: "grain on" | "grain off", s: FrameSummary, dpr: number): string {
  return `${pass}  p50 ${s.p50.toFixed(1)} ms  p95 ${s.p95.toFixed(1)} ms  frames ${String(s.frames)}  dpr ${String(dpr)}`;
}

/** One pass: one scroll step per frame, bouncing at the ends, for `ms`. The interval is the
 *  frame timestamp rAF hands in, so the page's own work between frames is inside it. */
export function scrollPass(
  list: HTMLElement,
  ms: number,
  raf: (cb: (t: number) => void) => unknown = requestAnimationFrame,
): Promise<number[]> {
  return new Promise((resolve) => {
    const intervals: number[] = [];
    let start = -1;
    let last = -1;
    let dir = 1;
    const tick = (t: number): void => {
      if (start < 0) {
        start = t;
      } else {
        intervals.push(t - last);
      }
      last = t;
      const max = list.scrollHeight - list.clientHeight;
      if (list.scrollTop >= max) dir = -1;
      else if (list.scrollTop <= 0) dir = 1;
      list.scrollTop += dir * STEP_PX;
      if (t - start >= ms) resolve(intervals);
      else raf(tick);
    };
    raf(tick);
  });
}

/** Grain on, then off, through the flag; the flag goes back to what it was, even on a throw. */
export async function measureGrain(
  list: HTMLElement,
  pass: (list: HTMLElement) => Promise<number[]> = (l) => scrollPass(l, PASS_MS),
): Promise<{ on: FrameSummary; off: FrameSummary }> {
  const before = getFlags().grain;
  try {
    setFlag("grain", "on");
    const on = summarise(await pass(list));
    setFlag("grain", "off");
    const off = summarise(await pass(list));
    return { on, off };
  } finally {
    setFlag("grain", before);
  }
}
```

The same file, continued: the component.

```tsx
const ROW_TEXT = Array.from({ length: ROWS }, (_, i) => `WRK-${String(i + 1)}  Fixture row for the grain meter`);

export function GrainMeter(): JSX.Element {
  const listRef = useRef<HTMLDivElement>(null);
  const [lines, setLines] = useState<readonly string[]>([]);
  const [running, setRunning] = useState(false);

  const measure = async (): Promise<void> => {
    const list = listRef.current;
    if (list === null || running) return;
    setRunning(true);
    try {
      const { on, off } = await measureGrain(list);
      const dpr = window.devicePixelRatio;
      setLines([resultLine("grain on", on, dpr), resultLine("grain off", off, dpr), `rule: ${grainRule(on, off)}`]);
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="ss-meter">
      <p className="ss-note">
        Set the stage zoom to 100 % first. Copy the two result lines into the Grain section of
        docs/verification/2026-09-25-phase-w-results.md.
      </p>
      <button type="button" className="dt-reset" disabled={running} onClick={() => void measure()}>
        {running ? "Measuring for 10 s" : "Measure"}
      </button>
      <output className="ss-meter-out">
        {lines.map((line) => (
          <span key={line}>{line}</span>
        ))}
      </output>
      <div ref={listRef} className="ss-meter-list">
        {ROW_TEXT.map((text) => (
          <div key={text} className="ss-meter-row">{text}</div>
        ))}
      </div>
    </div>
  );
}
```

*Measuring for 10 s* is a static label, not a count: the design system allows no spinner and one
clock, and a ticking countdown would be a second clock.

#### Interfaces — complete: the registration and its styles

```tsx
// desktop/src/components/devtools/screens/grain.tsx — task 35's meter on task 33's stage. The
// themes set only: the measurement is of the dark look, where grain is drawn.
import { GrainMeter } from "../GrainMeter";
import { registerStressScreen } from "../stressRegistry";

registerStressScreen("Grain, measured", () => <GrainMeter />, ["themes"]);
```

`screens/index.ts`: the comment line `// task 35: import "./grain";` becomes `import "./grain";`.

Appended to `stress.css` (task 33), before its forced-colours block:

```css
.ss-meter {
  display: grid;
  gap: var(--s-3);
}

.ss-meter-out {
  display: grid;
  gap: var(--s-1);
  color: var(--ink);
  font-family: var(--mono);
  font-size: var(--t-fact);
  line-height: var(--t-fact-lh);
  font-variant-numeric: tabular-nums;
}

.ss-meter-list {
  block-size: 480px;
  overflow: auto;
  background: var(--surface);
}

.ss-meter-row {
  min-block-size: var(--row-h);
  padding-inline: var(--s-3);
  color: var(--ink-2);
  font-family: var(--mono);
  font-size: var(--t-fact);
  line-height: var(--row-h);
}
```

#### Interfaces — complete: the *Grain* section of the results file

```markdown
## Grain (task 35)

Machine: <model>, Windows <build>, display scaling <125 | 150> %, WebView2 <version>.
Run: `npm --prefix desktop run tauri dev`, Dev → Stress set → Themes → *Grain, measured*.

    <paste the grain on line>
    <paste the grain off line>

Rule (02 C8): <grain stays | to the owner>. Owner's decision: <pending | date and words>.
```

Nothing is written into this section until a person runs the meter: an empty template is not a
result, and a guessed number is worse than none (`OD-27`).
