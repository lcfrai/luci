### Task 33: `stress.sets` with its three new screens

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 29 (every glyph,
`GlyphSize`, the path tables) and task 32 (the dev toolbar, `flags.ts`, the `DEV` gate). No Hub,
no wire shape, nothing in a production build.

02 C12 names Linear's quality habit: every UI change is looked at under a fixed set of stress
conditions before the owner reviews it. 05 §4.4 C12 (revision 2; 07 D C4, D I8) adds three
screens: **reduced motion × deuteranopia**, **a Windows contrast theme**, and **125 % and 150 %
scaling**. This task builds the harness those looks run in, and the three screens that exist by
now: the glyph sheet, the view header and the Develop pane in the eleven floor states. It proves
that each set changes what it says it changes. It does not prove that anything passes: the sets
are how the owner and reviewers look (`CLAUDE.md`: visual acceptance is the owner's eyes).

**Six decisions, verified against the tree at `9b6089d`:**

1. **A set is a condition; a screen is what is viewed under it.** `themes`, `reduced-deuteranopia`,
   `forced-colors` and `scaling` change how everything looks. `states` and `load` choose content:
   only screens registered for them appear. So `registerStressScreen(name, render)` keeps the
   scope's two parameters and gains an optional third, `sets`, defaulting to the four conditions.
   Later calls with two arguments stay valid.
2. **Screens register from dev-only files.** Tasks 38-48 and 62 are production modules. If one of
   them called `registerStressScreen` itself, the registry and its fixture render would ship. Each
   surface registers from its own file under `components/devtools/screens/`, imported by
   `screens/index.ts` in the order it wants to appear. An import-boundary test fails any module
   outside `components/devtools/` that imports from it, except `App.tsx`'s one import of `mount`.
3. **The Develop pane is a facsimile on the real classes, not a mounted `SessionPane`.**
   `SessionPane.tsx:125-133` reads the singleton `useCockpit` (`store/store.ts:91`) and mounts an
   xterm with a live socket. Eleven of them would each attach, and one global store cannot hold
   eleven states at once. The look is the stylesheet, so the facsimile renders the pane's own
   classes (`develop.css:61-293`) with fixture text, and a test fails any class it uses that
   `develop.css` does not define. **Three of the eleven states have no Phase 2 design**:
   *partial*, *unauthorized* and *blocked*. Their cells say so in words. That is a finding for the
   owner, not a gap to paper over.
4. **`GrainMeter.tsx` is task 35's, not this task's.** The scope has both tasks create it. A file
   this task wrote empty would be a placeholder screen that measures nothing. Task 35 creates it
   and registers it through the same `screens/index.ts`.
5. **`motion` and `vision` join `Flags`.** Task 32 made `flags.ts` the one writer of dev
   attributes on `<html>`, in memory only, with one reset. The reduced-motion set writes
   `data-motion="reduce"` and `data-vision="deuteranopia"` through it, so the toolbar's *Reset*
   clears them and the no-persistence test covers them.
6. **Both grounds on the same screen.** Task 26's paper tokens exist only under
   `:root[data-theme="paper"]`, so a subtree cannot be paper while its neighbour is graphite. The
   glyph sheet shows each glyph at 14 and 16 px, and the *themes* set's ground switch flips
   `theme` for the whole window. It is two grounds on one screen, one after the other, which is
   what 02 C12's flip is for.

**Why the deuteranopia filter sits on `#root` and the stage, not on `<html>`.** The toolbar must stay
readable while the app is filtered. `#root` (`main.tsx:9`) holds the app; the stage is portalled
beside it; the toolbar is a third sibling and is not filtered.

**Files:**
- `desktop/src/components/devtools/stressRegistry.ts` — `StressSet`, `STRESS_SETS`,
  `StressScreen`, `registerStressScreen`, `stressScreens` (new)
- `desktop/src/components/devtools/StressSets.tsx` — the set and screen pickers, the stage, the
  filter (new)
- `desktop/src/components/devtools/stress.css` — owns `.ss-*` (new)
- `desktop/src/components/devtools/screens/index.ts`, `glyphs.tsx`, `viewHeader.tsx`,
  `developStates.tsx` — the three built-in screens (new)
- `desktop/src/components/devtools/StressSets.test.tsx` (new)
- `desktop/src/components/devtools/DevToolbar.tsx` ⚑ (task 32) — mounts `<StressSets />` and
  imports `./screens` (modified)
- `desktop/src/lib/flags.ts` ⚑ (task 32) — `motion`, `vision` (modified)
- `desktop/src/lib/flags.test.ts` ⚑ (task 32) — the two new flags in two existing tests
  (modified)

#### Interfaces — complete: the `flags.ts` delta

Task 32's three places, each gaining two keys. Nothing else in the file changes.

```ts
export type Motion = "system" | "reduce";
export type Vision = "normal" | "deuteranopia";

export interface Flags {
  look: Look;
  sans: SansCandidate;
  theme: ThemeName;
  /** "reduce" writes data-motion="reduce": every reduced-motion rule's twin (task 34). */
  motion: Motion;
  /** "deuteranopia" writes data-vision; stress.css filters #root and the stage by it. */
  vision: Vision;
}

export const FLAG_DEFAULTS: Readonly<Flags> = Object.freeze({
  look: "new",
  sans: "segoe",
  theme: "graphite",
  motion: "system",
  vision: "normal",
});

const ATTRIBUTE: { readonly [K in keyof Flags]: string } = {
  look: "data-look",
  sans: "data-sans",
  theme: "data-theme",
  motion: "data-motion",
  vision: "data-vision",
};
```

`motion: "system"` is no attribute: the OS setting then decides through the media query alone.
`"reduce"` forces the reduced look on a machine whose OS setting is off, which is how a reviewer
sees it without changing Windows.

#### Interfaces — complete: `desktop/src/components/devtools/stressRegistry.ts`

```ts
// desktop/src/components/devtools/stressRegistry.ts
//
// stress.sets (Phase W task 33; 02 C12). A SET is a condition or a content filter; a SCREEN is a
// named render viewed under it. Dev only: register from a file under ./screens/, never from a
// production module (StressSets.test.tsx's import-boundary test holds that).

import type { ReactNode } from "react";

export type StressSet =
  | "themes"
  | "states"
  | "load"
  | "reduced-deuteranopia"
  | "forced-colors"
  | "scaling";

/** In the order the picker lists them: 02 C12's four, then 05 §4.4 C12's two new conditions. */
export const STRESS_SETS: readonly StressSet[] = [
  "themes", "states", "load", "reduced-deuteranopia", "forced-colors", "scaling",
];

/** The sets that change how a screen looks rather than which screens appear. */
export const CONDITION_SETS: readonly StressSet[] = [
  "themes", "reduced-deuteranopia", "forced-colors", "scaling",
];

export interface StressScreen {
  name: string;
  render: () => ReactNode;
  sets: readonly StressSet[];
}

const screens: StressScreen[] = [];

/** Registration order is list order. A second call with a name already registered replaces that
 *  screen IN PLACE: Vite's hot reload re-runs a screen module, and a duplicate would follow. */
export function registerStressScreen(
  name: string,
  render: () => ReactNode,
  sets: readonly StressSet[] = CONDITION_SETS,
): void {
  const screen: StressScreen = { name, render, sets };
  const at = screens.findIndex((s) => s.name === name);
  if (at >= 0) screens[at] = screen;
  else screens.push(screen);
}

/** The screens registered for one set, in registration order. */
export function stressScreens(set: StressSet): readonly StressScreen[] {
  return screens.filter((s) => s.sets.includes(set));
}
```

No unregister and no subscription: every screen registers at import, before the toolbar's first
render, and a dev session never removes one.

#### Interfaces — complete: `desktop/src/components/devtools/StressSets.tsx`

One file, given in two blocks in source order. The first holds the data: the matrix, the words
each set shows, and the filter.

```tsx
// desktop/src/components/devtools/StressSets.tsx
//
// stress.sets (Phase W task 33; 02 C12; 05 §4.4 C12). Picks a set, then renders every screen
// registered for it on a full-window stage portalled beside #root. The toolbar is not filtered
// and not zoomed, so the controls stay readable whatever the set does to the app.

import type { JSX } from "react";
import { useState } from "react";
import { createPortal } from "react-dom";
import { setFlag } from "../../lib/flags";
import { STRESS_SETS, stressScreens } from "./stressRegistry";
import type { StressSet } from "./stressRegistry";

/** Deuteranopia, severity 1.0: Machado, Oliveira and Fernandes, "A Physiologically-based Model
 *  for Simulation of Color Vision Deficiency", IEEE TVCG 15(6), 2009, the severity-1.0 row of
 *  their deuteranomaly table. Defined on LINEAR RGB, so the filter declares linearRGB. Each RGB
 *  row sums to 1 (white stays white). Rows are feColorMatrix's: r g b a offset. */
export const DEUTERANOPIA_MATRIX: readonly number[] = [
  0.367322, 0.860646, -0.227968, 0, 0,
  0.280085, 0.672501, 0.047413, 0, 0,
  -0.01182, 0.04294, 0.968881, 0, 0,
  0, 0, 0, 1, 0,
];

export const SET_LABEL: Readonly<Record<StressSet, string>> = {
  themes: "Themes",
  states: "States",
  load: "Load",
  "reduced-deuteranopia": "Reduced motion × deuteranopia",
  "forced-colors": "Windows contrast theme",
  scaling: "125 % and 150 %",
};

export const SET_NOTE: Readonly<Record<StressSet, string>> = {
  themes: "Flip the ground on this screen. Paper is task 26's provisional block, not a reviewed theme.",
  states:
    "The eleven states of the design system's floor. A cell that says not designed has no Phase 2 design yet.",
  load: "Each screen here shows one surface at several sizes of work.",
  "reduced-deuteranopia":
    "Motion is reduced and a deuteranopia simulation covers the app. Look for any two states told apart by colour alone.",
  "forced-colors":
    "Page CSS cannot turn forced colours on. Turn on a Windows contrast theme: Settings, Accessibility, Contrast themes, or Left Alt + Left Shift + Print Screen.",
  scaling:
    "A CSS zoom preview. It is not the same as Windows display scaling, which changes text rendering, hit targets and the window frame. For the real check set Settings, System, Display, Scale.",
};

export const NO_SCREEN = "No screen is registered for this set yet.";

/** Portalled once into <body>; stress.css points #root and the stage at it by data-vision. */
function VisionFilter(): JSX.Element {
  return createPortal(
    <svg className="ss-defs" aria-hidden="true" focusable="false" width="0" height="0">
      <filter id="ss-deutan" colorInterpolationFilters="linearRGB">
        <feColorMatrix type="matrix" values={DEUTERANOPIA_MATRIX.join(" ")} />
      </filter>
    </svg>,
    document.body,
  );
}

/** The live answer to "is a contrast theme on?", read at render; jsdom's stub says no. */
function forcedColoursActive(): boolean {
  return window.matchMedia("(forced-colors: active)").matches;
}
```

The matrix values are from memory of the paper's table, not re-read: **UNVERIFIED** until step 1
checks them against the paper. The row-sum test holds whatever the source says, and a transposed
matrix fails it (its first column sums to 0.635587).

The second block, the component:

```tsx
type Zoom = 100 | 125 | 150;
const ZOOMS: readonly Zoom[] = [100, 125, 150];

/** Entering reduced-deuteranopia writes both flags; leaving it, or closing, clears both. */
function applySet(set: StressSet | null): void {
  const on = set === "reduced-deuteranopia";
  setFlag("motion", on ? "reduce" : "system");
  setFlag("vision", on ? "deuteranopia" : "normal");
}

export function StressSets(): JSX.Element {
  const [set, setSet] = useState<StressSet | null>(null);
  const [zoom, setZoom] = useState<Zoom>(100);
  const choose = (next: StressSet | null): void => {
    applySet(next);
    setSet(next);
  };
  const screens = set === null ? [] : stressScreens(set);
  return (
    <>
      <fieldset className="dt-group">
        <legend className="dt-legend">Stress set</legend>
        {STRESS_SETS.map((s) => (
          <label key={s} className="dt-option">
            <input type="radio" name="dt-stress" value={s} checked={set === s} onChange={() => choose(s)} />
            {SET_LABEL[s]}
          </label>
        ))}
        {set !== null && (
          <button type="button" className="dt-reset" onClick={() => choose(null)}>
            Close the stage
          </button>
        )}
      </fieldset>
      <VisionFilter />
      {set !== null &&
        createPortal(
          <section className="ss-stage" data-set={set} data-zoom={set === "scaling" ? zoom : undefined} aria-label="Stress stage">
            <div className="ss-bar">
              <p className="ss-note">{SET_NOTE[set]}</p>
              {set === "forced-colors" && (
                <p className="ss-fact" data-forced={forcedColoursActive() ? "active" : "off"}>
                  forced colours: {forcedColoursActive() ? "active" : "not active"}
                </p>
              )}
              {set === "scaling" && (
                <div className="ss-zoom" role="radiogroup" aria-label="Preview zoom">
                  {ZOOMS.map((z) => (
                    <label key={z} className="dt-option">
                      <input type="radio" name="ss-zoom" checked={zoom === z} onChange={() => setZoom(z)} />
                      {z} %
                    </label>
                  ))}
                </div>
              )}
            </div>
            <div className="ss-screens">
              {screens.length === 0 && <p className="ss-note">{NO_SCREEN}</p>}
              {screens.map((screen) => (
                <article key={screen.name} className="ss-screen" aria-label={screen.name}>
                  <h2 className="ss-name">{screen.name}</h2>
                  {screen.render()}
                </article>
              ))}
            </div>
          </section>,
          document.body,
        )}
    </>
  );
}
```

`forcedColoursActive()` is read at render and not subscribed: toggling a contrast theme repaints the
whole window, and the reviewer re-picks the set to refresh the line. A `change` listener would be
a second source of truth for a dev readout. The ground switch for *themes* is the toolbar's own
*Theme* group (task 32), which already sits one tab stop away; a second switch on the stage would
be two writers of one flag.
