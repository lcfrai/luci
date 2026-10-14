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
result, and a guessed number is worse than none (`CLAUDE.md`: state what is unverified).

#### Tests to write first — `desktop/src/styles/grain.test.ts`

```ts
import { afterEach, describe, expect, it } from "vitest";
import grainCss from "./grain.css?raw";
import tokensCss from "./tokens.css?raw";
import forcedCss from "./forced-colors.css?raw";
import { resetFlags, setFlag } from "../lib/flags";
import { stripCssComments } from "../test/fakes";

/** tokens.css:164-170 at bf4caeb, whitespace-collapsed: the rule as it was before the move. */
const ORIGINAL = [
  'content: ""',
  "position: fixed",
  "inset: 0",
  "z-index: 9999",
  "pointer-events: none",
  "opacity: 0.022",
  `background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='120' height='120'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='3'/%3E%3C/filter%3E%3Crect width='120' height='120' filter='url(%23n)'/%3E%3C/svg%3E")`,
];

/** The declarations of the first rule whose selector list is exactly `selector`. */
function body(css: string, selector: string): string[] | null {
  const bare = stripCssComments(css);
  const at = bare.search(new RegExp(`(^|[}\\s])${selector.replace(/[.*+?^${}()|[\]\\$]/g, "\\$&")}\\s*\\{`, "m"));
  if (at < 0) return null;
  const open = bare.indexOf("{", at);
  return bare.slice(open + 1, bare.indexOf("}", open)).split(";").map((d) => d.replace(/\s+/g, " ").trim()).filter((d) => d !== "");
}

afterEach(() => resetFlags());

describe("grain", () => {
  it("moves the grain rule without changing it", () => {
    // Kills: a tuned opacity or a changed noise slipped in with the move; a copy left behind.
    expect(body(grainCss, ".grain::after")).toEqual(ORIGINAL);
    expect(stripCssComments(tokensCss)).not.toContain(".grain");
  });

  it("turns grain off in high-contrast themes", () => {
    // Kills: grain on a -hc theme, and on paper, where the design gives dark's mark no place.
    const off = /((?::root\[[^\]]+\] \.grain::after,?\s*)+)\{\s*display: none;\s*\}/.exec(stripCssComments(grainCss));
    expect(off).not.toBeNull();
    expect(off![1]).toContain(':root[data-theme$="-hc"] .grain::after');
    expect(off![1]).toContain(':root[data-theme="paper"] .grain::after');
  });

  it("turns grain off with the flag", () => {
    // Kills: a meter whose second pass still has grain, so it cannot compare.
    setFlag("grain", "off");
    expect(document.documentElement.getAttribute("data-grain")).toBe("off");
    expect(stripCssComments(grainCss)).toContain(':root[data-grain="off"] .grain::after');
    setFlag("grain", "on");
    expect(document.documentElement.hasAttribute("data-grain")).toBe(false);
  });

  it("keeps grain on the shell only, with one forced-colours rule", () => {
    // Kills: grain per component (a compositor layer each), and a second forced-colours copy.
    const sources = import.meta.glob<string>("/src/**/*.{ts,tsx,css}", { query: "?raw", import: "default", eager: true });
    const users = Object.entries(sources)
      .filter(([f]) => !/\.test\.tsx?$/.test(f) && !f.startsWith("/src/test/"))
      // The flag and the meter name "grain" as a flag, not as a class.
      .filter(([f]) => f !== "/src/lib/flags.ts" && !f.startsWith("/src/components/devtools/"))
      .filter(([f, text]) => (f.endsWith(".css") ? /\.grain\b/.test(stripCssComments(text)) : /"grain"/.test(text)))
      .map(([f]) => f)
      .sort();
    expect(users).toEqual([
      "/src/components/AppShell.tsx",
      "/src/styles/forced-colors.css",
      "/src/styles/grain.css",
    ]);
    expect(stripCssComments(forcedCss).match(/\.grain::after/g)).toHaveLength(1);
  });
});
```

`AppShell.tsx:82` builds the class list as `["shell", "grain"]`, which the `"grain"` pattern
finds. `flags.ts`, `FlagCompare.tsx` (`flag="grain"`) and `GrainMeter.tsx` (`setFlag("grain", …)`)
name the flag, not the class, and are the two exclusions. That no other source file holds the
literal today is **UNVERIFIED** until step 2 runs it; a hit would name its file.

#### Tests to write first — `desktop/src/components/devtools/GrainMeter.test.tsx`

```tsx
import { afterEach, describe, expect, it } from "vitest";
import { resetFlags, setFlag } from "../../lib/flags";
import { grainRule, measureGrain, percentile, resultLine, scrollPass, summarise } from "./GrainMeter";

afterEach(() => resetFlags());

describe("the grain meter", () => {
  it("reports p50 and p95 from recorded intervals", () => {
    // Kills: a mean reported as p95 (18.55), the max reported as p95 (50), an off-by-one rank.
    const intervals = [...Array.from({ length: 18 }, () => 16), 50, 33];
    expect(summarise(intervals)).toEqual({ frames: 20, p50: 16, p95: 33 });
    expect(percentile([1, 2, 3, 4], 50)).toBe(2);
    expect(percentile([], 95)).toBeNaN();
  });

  it("applies the recorded rule and prints the device pixel ratio", () => {
    // Kills: a rule that ignores the 16.7 ms frame, one that ignores the 1 ms margin, and a line
    // that drops the DPR the numbers were taken at.
    const s = (p95: number) => ({ frames: 300, p50: 16.6, p95 });
    expect(grainRule(s(16.4), s(15.9))).toBe("grain stays");
    expect(grainRule(s(17.0), s(16.5))).toBe("to the owner");
    expect(grainRule(s(14.0), s(12.5))).toBe("to the owner");
    expect(resultLine("grain on", s(16.4), 1.5)).toBe("grain on  p50 16.6 ms  p95 16.4 ms  frames 300  dpr 1.5");
  });

  it("records one interval per frame and stops at the time limit", async () => {
    // Kills: a pass that counts the first frame as an interval, or never stops.
    let t = 0;
    const raf = (cb: (now: number) => void): number => {
      const now = t;
      t += 16;
      queueMicrotask(() => cb(now));
      return 0;
    };
    const intervals = await scrollPass(document.createElement("div"), 100, raf);
    expect(intervals).toEqual([16, 16, 16, 16, 16, 16, 16]);
  });

  it("measures once with grain and once without, then restores the flag", async () => {
    // Kills: two passes under one condition, and a meter that leaves grain off afterwards.
    setFlag("grain", "on");
    const seen: (string | null)[] = [];
    const pass = async (): Promise<number[]> => {
      seen.push(document.documentElement.getAttribute("data-grain"));
      return [16, 17];
    };
    const { on, off } = await measureGrain(document.createElement("div"), pass);
    expect(seen).toEqual([null, "off"]);
    expect([on.frames, off.frames]).toEqual([2, 2]);
    expect(document.documentElement.hasAttribute("data-grain")).toBe(false);
  });
});
```

The fake frame clock starts at 0 and steps 16 ms; the pass stops on the first frame at or past
100 ms (the frame at 112), so it records seven intervals. `AppShell.test.tsx:192-193` and
`defects.test.tsx:193-201` are not edited: they pin that the shell carries `grain`, which this
task keeps.

#### Tests to change — `desktop/src/lib/flags.test.ts`

In *writes each flag to the root element*: `setFlag("grain", "off");` and
`expect(root().getAttribute("data-grain")).toBe("off");`. In *removes the attribute when a flag
returns to its default*: `setFlag("grain", "off"); setFlag("grain", "on");` and
`expect(root().hasAttribute("data-grain")).toBe(false);`.

#### Steps

1. Write `grain.test.ts`, `GrainMeter.test.tsx` and the `flags.test.ts` changes. Run
   `npm --prefix desktop test -- grain GrainMeter flags`: the two new files fail to import; the
   flags tests fail on the missing key.
2. Create `grain.css` by **moving** `tokens.css:159-171` (a cut, then the new header comment and
   the off rule), add the `app.css` import. The four grain tests pass; `AppShell` and `defects`
   still pass (`npm --prefix desktop test -- AppShell defects`).
3. Add `grain` to `flags.ts` and the fourth group to `FlagCompare.tsx`. The flags tests pass.
4. Write `GrainMeter.tsx`, `screens/grain.tsx`, the `index.ts` line and the `stress.css`
   rules. The four meter tests pass.
5. Mutate by hand, one at a time, and watch the named test go red, then revert: `opacity:
   0.03` in `grain.css`; drop the `-hc` selector; `sorted[Math.floor(…)]` in `percentile`;
   `setFlag("grain", "on")` before the second pass; leave the `finally` out of `measureGrain`.
6. Run the typecheck and the full desktop suite; commit with `git commit -F <msg> -- <paths>`.
7. **The measurement, by a person, on the laptop.** `npm --prefix desktop run tauri dev` at the
   laptop's own Windows scaling (150 % on the reference machine). Dev → Stress set → Themes; the
   stage zoom at 100 %; *Grain, measured* → Measure. Copy the two lines into the *Grain* section
   of `docs/verification/2026-09-25-phase-w-results.md`, with the machine line filled in, and
   commit that file. If the rule line reads *to the owner*, the numbers go to the owner at task 39
   with the proposal to turn grain off at a DPR of 1.5 and above; nothing turns it off before
   the owner says so.

#### Acceptance

```bash
npm --prefix desktop test -- grain GrainMeter AppShell defects flags
# expect: every test passes: grain's four, the meter's four, AppShell's and defects' grain
# assertions unchanged, and flags' tests with the new key

npm --prefix desktop run typecheck
# expect: exit 0, no output from tsc

grep -c "grain" desktop/src/styles/tokens.css
# expect: 0

grep -A3 "^## Grain" docs/verification/2026-09-25-phase-w-results.md
# expect (after step 7 only): the machine line, then two measured lines, each with "dpr"
```

#### What this task does not claim

- **A frame budget met.** The numbers are recorded, **UNVERIFIED** until step 7 runs, and the
  owner accepts them. The rule says which way they point; it does not decide.
- **That rAF intervals are GPU frame time.** They are the page's frame pacing, which is what a
  scrolling list feels like; compositor cost that does not delay the next frame is not seen.
- **Any machine but the one it ran on,** nor 125 % unless someone runs it there too.
- **That grain is right at all.** Whether it still adds a *made-object quality* on the new look is
  the owner's eyes at task 39, with the grain flag flipping it on one screen.
- **High-contrast themes.** None exists (W2); the rule is written ahead so none can ship with grain.
- Design: 05 §4.4 work rule 6, C13 (grain is dark's mark; paper carries the thread gutter);
  02 C8, C13; 07 D M5, D M6. Parity: no 01 row.

**Interfaces:**
- Produces: `desktop/src/styles/grain.css` owning `.grain::after` and its three off rules
  (`paper`, `-hc`, `data-grain="off"`); `Grain` and the `grain` flag (`data-grain="off"`);
  `GrainMeter`, `PASS_MS`, `ROWS`, `STEP_PX`, `FrameSummary`, `percentile`, `summarise`,
  `grainRule`, `resultLine`, `scrollPass`, `measureGrain`; the *Grain* section's shape
- Consumes: task 34's forced-colours grain rule (the one copy); task 33's
  `registerStressScreen`, `screens/index.ts`, `.ss-note`; task 32's `setFlag`, `getFlags`,
  `resetFlags`, `Group`, `FlagOption`, `.dt-reset`; task 26's `--t-fact`, `--t-fact-lh`, `--s-*`,
  `--row-h`; the built `.grain::after` (`tokens.css:163-171`), `AppShell.tsx:82`,
  `AppShell.test.tsx:192-193`, `defects.test.tsx:193-201` (all verified)
- Amends: nothing in the Hub. No wire shape changes.
