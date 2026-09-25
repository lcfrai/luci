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

#### Interfaces — complete: `screens/glyphs.tsx` and `screens/viewHeader.tsx`

```tsx
// desktop/src/components/devtools/screens/glyphs.tsx
//
// Every drawn glyph (tasks 28, 29) at both sizes. The rows are DERIVED from the path tables, so a
// glyph added to paths.ts appears here without an edit, and one the sheet cannot draw fails the
// test that counts cells. Registered for the four condition sets and for forced colours' layout.

import type { JSX, ReactNode } from "react";
import { AgentGlyph } from "../../glyphs/AgentGlyph";
import type { AgentStatus } from "../../glyphs/AgentGlyph";
import { MarkGlyph } from "../../glyphs/MarkGlyph";
import type { MarkKind } from "../../glyphs/MarkGlyph";
import { AGENT_PATHS, MARK_PATHS, NOTCH_TIP, STATE_PATHS } from "../../glyphs/paths";
import type { GlyphKey } from "../../glyphs/paths";
import { StateGlyph } from "../../glyphs/StateGlyph";
import type { GlyphSize, IssueState, Priority } from "../../glyphs/StateGlyph";
import { registerStressScreen } from "../stressRegistry";

export const SIZES: readonly GlyphSize[] = [14, 16];

export interface SheetRow {
  key: string;
  draw: (size: GlyphSize) => ReactNode;
}

const keys = <T extends string>(table: Record<T, unknown>): T[] => Object.keys(table) as T[];

export const SHEET_ROWS: readonly SheetRow[] = [
  ...keys<GlyphKey>(STATE_PATHS[16]).map((key): SheetRow => ({
    key: `state:${key}`,
    draw: (size) =>
      key === "done_not_proven"
        ? <StateGlyph state="done" notProven size={size} />
        : <StateGlyph state={key as IssueState} size={size} />,
  })),
  ...keys<Exclude<Priority, "none">>(NOTCH_TIP[16]).map((p): SheetRow => ({
    key: `priority:${p}`,
    draw: (size) => <StateGlyph state="todo" priority={p} size={size} />,
  })),
  ...keys<AgentStatus>(AGENT_PATHS[16]).map((status): SheetRow => ({
    key: `agent:${status}`,
    draw: (size) => <AgentGlyph status={status} size={size} label={status} />,
  })),
  ...keys<MarkKind>(MARK_PATHS[16]).map((kind): SheetRow => ({
    key: `mark:${kind}`,
    draw: (size) => <MarkGlyph kind={kind} size={size} />,
  })),
  { key: "mark:thread_node:lit", draw: (size) => <MarkGlyph kind="thread_node" size={size} lit /> },
];

export function GlyphSheet(): JSX.Element {
  return (
    <div className="ss-sheet">
      {SHEET_ROWS.map((row) => (
        <div key={row.key} className="ss-row">
          <span className="ss-key">{row.key}</span>
          {SIZES.map((size) => (
            <span key={size} className="ss-cell" data-cell={`${row.key}@${String(size)}`}>
              {row.draw(size)}
            </span>
          ))}
        </div>
      ))}
    </div>
  );
}

registerStressScreen("Glyphs at 14 and 16 px", () => <GlyphSheet />);
```

```tsx
// desktop/src/components/devtools/screens/viewHeader.tsx — task 31's header, each route state.
import { ViewHeader } from "../../common/ViewHeader";
import { registerStressScreen } from "../stressRegistry";

registerStressScreen("View header", () => (
  <>
    <ViewHeader title="Work" route="global" />
    <ViewHeader title="Work" route="au" />
    <ViewHeader title="Work" route={null} />
  </>
));
```

`STATE_PATHS[16]`'s keys are task 28's ten `GlyphKey`s (nine states and `done_not_proven`);
`NOTCH_TIP[16]`'s are the four drawn priorities. Twenty-three rows, forty-six cells.

#### Interfaces — complete: `screens/developStates.tsx`

The copy in each cell is the pane's own where the tree has it (`EmptyDevelop.tsx:89`,
`SessionPane.tsx:178`, `:210-217`, `attachLabel` at `SessionPaneFooter.tsx:33-48`); the offline
reason is fixture text, marked so.

```tsx
// desktop/src/components/devtools/screens/developStates.tsx
//
// The Develop pane in the eleven floor states (docs/DESIGN-SYSTEM.md:90). A FACSIMILE on the
// pane's real classes: SessionPane reads the one global store and mounts a live xterm, so eleven
// real panes cannot hold eleven states. StressSets.test.tsx fails any pane-/cols- class used here
// that develop.css does not define. A state with no Phase 2 design says so, in words.

import type { JSX, ReactNode } from "react";
import { StatusDot } from "../../common/StatusDot";
import { registerStressScreen } from "../stressRegistry";

export const FLOOR_STATES = [
  "loading", "empty", "sparse", "dense", "partial", "stale",
  "offline", "unauthorized", "error", "blocked", "success",
] as const;
export type FloorState = (typeof FLOOR_STATES)[number];

export const NOT_DESIGNED = "Not designed: Phase 2 has no pane for this state.";

function Pane(props: { status: "running" | "idle" | "exited"; foot: string; children?: ReactNode }): JSX.Element {
  return (
    <section className="pane" aria-label={`evidence-upload — ${props.status}`}>
      <header className="pane-head">
        <StatusDot status={props.status} live={props.status === "running"} label={props.status} />
        <span className="pane-title">evidence-upload</span>
        <span className="pane-sub">Claude Code</span>
      </header>
      <div className="pane-body">
        <pre className="ss-term">{"$ npm test\n  ✓ 41 passed"}</pre>
        {props.children}
      </div>
      <footer className="pane-foot">
        <span className="pane-foot-fact">{props.foot}</span>
      </footer>
    </section>
  );
}

const attach = (phase: string, text: string): JSX.Element => (
  <div className={phase === "connecting" ? "pane-attach pane-attach--centre" : "pane-attach pane-attach--corner"} data-attach-phase={phase}>
    {text}
  </div>
);

export const DEVELOP_STATES: Readonly<Record<FloorState, (() => ReactNode) | null>> = {
  loading: () => <Pane status="running" foot="0m · connecting 3s">{attach("connecting", "connecting 3s")}</Pane>,
  empty: () => (
    <div className="pane-empty">
      <p className="pane-empty-line">Select a branch in the rail to start working on it.</p>
    </div>
  ),
  sparse: () => <Pane status="running" foot="12m · attached" />,
  dense: () => (
    <div className="cols-scroll">
      {["running", "running", "idle", "running", "idle", "exited"].map((s, i) => (
        <div key={i} className="cols-pane">
          <Pane status={s as "running" | "idle" | "exited"} foot={`${String(3 + i)}m · attached`} />
        </div>
      ))}
    </div>
  ),
  partial: null,
  stale: () => <Pane status="running" foot="40m · reconnecting 8s">{attach("reconnecting", "reconnecting 8s")}</Pane>,
  offline: () => (
    <Pane status="running" foot="40m · not attached">
      {attach("failed", "Fixture reason: the control plane stopped answering.")}
    </Pane>
  ),
  unauthorized: null,
  error: () => (
    <Pane status="exited" foot="1h 2m · ended">
      <div className="pane-exit">
        <span className="pane-exit-why">the process exited</span>
        <button type="button" className="pane-exit-restart">Restart</button>
      </div>
    </Pane>
  ),
  blocked: null,
  success: () => <Pane status="idle" foot="18m · attached" />,
};
```

The same file, continued: the grid and its registration.

```tsx
export function DevelopStates(): JSX.Element {
  return (
    <div className="ss-states">
      {FLOOR_STATES.map((state) => {
        const draw = DEVELOP_STATES[state];
        return (
          <figure key={state} className="ss-state" data-state={state} data-designed={draw === null ? "false" : "true"}>
            <figcaption className="ss-key">{state}</figcaption>
            {draw === null ? <p className="ss-note">{NOT_DESIGNED}</p> : draw()}
          </figure>
        );
      })}
    </div>
  );
}

registerStressScreen("Develop pane, eleven states", () => <DevelopStates />, [
  "states", "themes", "reduced-deuteranopia", "forced-colors", "scaling",
]);
```

#### Interfaces — complete: `screens/index.ts` and the `DevToolbar.tsx` delta

```ts
// desktop/src/components/devtools/screens/index.ts
//
// Import order is list order. A later surface appends one line here and registers from its own
// file in this directory, never from its production module (the import-boundary test).
import "./glyphs";
import "./viewHeader";
import "./developStates";
// task 35: import "./grain";
```

`DevToolbar.tsx`, two imports after `import "../../devtools/fonts/candidates.css";` and one
element in place of the `{/* task 33: <StressSets /> */}` comment:

```tsx
import "./stress.css";
import "./screens";
import { StressSets } from "./StressSets";
```

```tsx
        <FlagCompare />
        <StressSets />
```

#### Interfaces — complete: `stress.css`

```css
/* desktop/src/components/devtools/stress.css
 *
 * stress.sets' stage (Phase W task 33). Dev only, loaded with the toolbar. The deuteranopia filter
 * covers #root and the stage and never the toolbar, so the controls stay readable. zoom is a
 * preview of scaling, labelled in the stage as not Windows display scaling. */
@import "../../styles/tokens.css";

.ss-defs {
  position: absolute;
  inline-size: 0;
  block-size: 0;
  overflow: hidden;
}

:root[data-vision="deuteranopia"] #root,
:root[data-vision="deuteranopia"] .ss-stage {
  filter: url("#ss-deutan");
}

.ss-stage {
  position: fixed;
  inset: 0;
  z-index: 8000;
  overflow: auto;
  background: var(--page);
  color: var(--ink);
}

.ss-stage[data-zoom="125"] .ss-screens { zoom: 1.25; }
.ss-stage[data-zoom="150"] .ss-screens { zoom: 1.5; }

.ss-bar {
  position: sticky;
  inset-block-start: 0;
  display: grid;
  gap: var(--s-2);
  padding: var(--s-3) var(--s-4);
  background: var(--surface);
  box-shadow: var(--e-hair);
}
```

The same file, continued:

```css
.ss-note {
  margin: 0;
  max-inline-size: 72ch;
  color: var(--ink-2);
  font-size: var(--t-ui);
  line-height: var(--t-ui-lh);
}

/* Machine facts: glyph keys, state ids, the forced-colours readout, the terminal stand-in.
 * voice.split (task 30): mono at --t-fact, never below --ink-2. */
.ss-fact,
.ss-key,
.ss-term {
  margin: 0;
  color: var(--ink-2);
  font-family: var(--mono);
  font-size: var(--t-fact);
  line-height: var(--t-fact-lh);
}

.ss-zoom {
  display: flex;
  gap: var(--s-3);
}

.ss-screens {
  display: grid;
  gap: var(--s-5);
  padding: var(--s-4);
}

.ss-screen {
  display: grid;
  gap: var(--s-3);
}

.ss-name {
  color: var(--ink);
  font-size: var(--t-ui);
  line-height: var(--t-ui-lh);
}

.ss-sheet {
  display: grid;
  gap: var(--s-1);
}

.ss-row {
  display: grid;
  grid-template-columns: 24ch repeat(2, var(--row-h));
  align-items: center;
  min-block-size: var(--row-h);
}

.ss-cell {
  display: grid;
  place-items: center;
  color: var(--ink);
}

.ss-states {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: var(--s-4);
}

.ss-state {
  display: grid;
  grid-template-rows: auto minmax(180px, auto);
  gap: var(--s-2);
  margin: 0;
}
```

No transition and no animation: the stage appears and goes, because nothing about opening it is
causal. `.ss-stage` sits at 8000, under the toolbar (9000) and the grain (9999).

#### Tests to write first — `desktop/src/components/devtools/StressSets.test.tsx`

```tsx
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import developCss from "../develop/develop.css?raw";
import stressCss from "./stress.css?raw";
import { AGENT_PATHS, MARK_PATHS, NOTCH_TIP, STATE_PATHS } from "../glyphs/paths";
import { resetFlags, setFlag } from "../../lib/flags";
import { stripCssComments } from "../../test/fakes";
import { DEUTERANOPIA_MATRIX, SET_LABEL, StressSets } from "./StressSets";
import { DevelopStates, DEVELOP_STATES, FLOOR_STATES, NOT_DESIGNED } from "./screens/developStates";
import { GlyphSheet } from "./screens/glyphs";

const root = () => document.documentElement;
const css = stripCssComments(stressCss);

afterEach(() => {
  resetFlags();
  vi.resetModules();
});

describe("stress.sets", () => {
  it("applies each new set's attribute or filter", async () => {
    // Kills: a set that changes nothing, a filter rule nothing points at, a zoom never applied.
    render(<StressSets />);
    await userEvent.click(screen.getByRole("radio", { name: SET_LABEL["reduced-deuteranopia"] }));
    expect(root().getAttribute("data-motion")).toBe("reduce");
    expect(root().getAttribute("data-vision")).toBe("deuteranopia");
    expect(css).toMatch(/:root\[data-vision="deuteranopia"\] #root,\s*:root\[data-vision="deuteranopia"\] \.ss-stage \{\s*filter: url\("#ss-deutan"\);/);
    expect(document.getElementById("ss-deutan")).not.toBeNull();

    await userEvent.click(screen.getByRole("radio", { name: SET_LABEL["forced-colors"] }));
    expect(root().hasAttribute("data-motion")).toBe(false);
    expect(root().hasAttribute("data-vision")).toBe(false);
    expect(document.querySelector(".ss-fact")?.getAttribute("data-forced")).toBe("off");

    await userEvent.click(screen.getByRole("radio", { name: SET_LABEL.scaling }));
    await userEvent.click(screen.getByRole("radio", { name: "150 %" }));
    expect(document.querySelector(".ss-stage")?.getAttribute("data-zoom")).toBe("150");
    expect(css).toMatch(/\.ss-stage\[data-zoom="150"\] \.ss-screens \{ zoom: 1\.5; \}/);

    await userEvent.click(screen.getByRole("button", { name: "Close the stage" }));
    expect(document.querySelector(".ss-stage")).toBeNull();
  });

  it("uses the cited deuteranopia matrix", () => {
    // Kills: a wrong or transposed matrix, a missing alpha row, the matrix applied in sRGB.
    render(<StressSets />);
    const filter = document.getElementById("ss-deutan");
    expect(filter?.getAttribute("color-interpolation-filters")).toBe("linearRGB");
    const values = (filter?.querySelector("feColorMatrix")?.getAttribute("values") ?? "")
      .split(/\s+/).map(Number);
    expect(values).toEqual([...DEUTERANOPIA_MATRIX]);
    expect(values.slice(0, 3)).toEqual([0.367322, 0.860646, -0.227968]);
    for (const row of [0, 1, 2]) {
      const [r, g, b, a, offset] = values.slice(row * 5, row * 5 + 5);
      expect(Math.abs(r + g + b - 1)).toBeLessThan(1e-5);
      expect([a, offset]).toEqual([0, 0]);
    }
    expect(values.slice(15)).toEqual([0, 0, 0, 1, 0]);
  });

  it("renders every glyph at both sizes on both grounds", () => {
    // Kills: a glyph in paths.ts that the sheet never draws, a size dropped, a ground that
    // unmounts the sheet.
    const expected = [
      ...Object.keys(STATE_PATHS[16]).map((k) => `state:${k}`),
      ...Object.keys(NOTCH_TIP[16]).map((k) => `priority:${k}`),
      ...Object.keys(AGENT_PATHS[16]).map((k) => `agent:${k}`),
      ...Object.keys(MARK_PATHS[16]).map((k) => `mark:${k}`),
      "mark:thread_node:lit",
    ];
    expect(expected).toHaveLength(23);
    render(<GlyphSheet />);
    for (const ground of ["graphite", "paper"] as const) {
      setFlag("theme", ground);
      for (const key of expected) {
        for (const size of [14, 16]) {
          const cell = document.querySelector(`[data-cell="${key}@${String(size)}"]`);
          expect(cell?.querySelector("svg"), `${ground} ${key}@${String(size)}`).not.toBeNull();
        }
      }
    }
  });
```
