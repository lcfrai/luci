### Task 32: `flag.compare` in the dev toolbar: old and new look, the three sans candidates (02 D2 step 7)

Stream **WD1**, about 2 hours (an estimate, not a measurement). Depends on task 27 (`--sans`,
`--sans-small`, `--sans-display`) and task 31 (the `:root[data-look="old"]` blocks in `rail.css`
and `subrail.css`; task 30 wrote the same convention into `develop.css` and `palette.css`). No
Hub, no wire shape, nothing a user outside a dev build can see.

02 C12: every visual change lands behind a flag, and a dev-toolbar toggle flips old and new **on
the same screen**, so the owner's review (`OD-1`, task 39) is a two-second comparison rather than
two screenshots from two builds. The toggle writes attributes on `<html>`; the stylesheets tasks
26, 30 and 31 wrote already read them. Nothing remounts, nothing reloads, and a half-typed
input survives a flip.

**The sans question is answered and still shown** (05 §4.4 C3, revision 2; Q14). `OD-28` chose
Segoe UI Variable. The toolbar sets it beside Atkinson Hyperlegible Next and Geist so the owner
confirms the choice by eye at task 39. The two alternatives exist only in a dev build.

**Four decisions, verified against the tree at `bf4caeb`:**

1. **A flag at its default is no attribute.** Task 31's rules key on `[data-look="old"]` only
   ("with no `data-look` set, the new look is the default"), and task 26's paper block is
   `:root[data-theme="paper"]` only. So `setFlag` **removes** the attribute when a value returns
   to its default. A DOM nobody flipped is then the production DOM, byte for byte, and no rule
   can ever key on an attribute being present.
2. **`flags.ts` names no candidate at run time.** Task 38 renders the prototype "behind
   `flag.compare`", so production code may import `flags.ts` for `look`. If `flags.ts` held a
   list with `"atkinson"` or `"geist"` in it, the production bundle would carry both words and
   the acceptance grep would fail for the right reason. The candidate list, with its labels, is in
   `FlagCompare.tsx`, which production never loads. `SansCandidate` is a type, and types erase.
3. **The toolbar is one dynamic import behind `import.meta.env.DEV`.** `App.tsx:240` returns
   `<AppShell />` and nothing else. A static import of the toolbar, even if never rendered, would
   put its module and its stylesheets into the production bundle. The gate is a three-line
   module, `devtools/mount.ts`, whose export is `null` when `DEV` is false; Vite replaces
   `import.meta.env.DEV` with a literal at build time, so the `import()` sits in dead code. That
   the bundler then emits **no chunk** for it is **UNVERIFIED** for Vite 8 (Rolldown); the
   acceptance grep is the proof, and step 7 names the fallback.
4. **A container the scope did not name.** Task 33 mounts `StressSets` "in the dev toolbar beside
   `FlagCompare`", and task 35's grain flag joins it. One `DevToolbar.tsx` hosts the sections, so
   each later task adds one line to it rather than a second floating panel. It is new here and
   not in the skeleton's file map.

**Why `data-sans` needs all three sans tokens.** Task 27 declares `--sans-small:
"Segoe UI Variable Small", var(--sans)` and `--sans-display: "Segoe UI Variable Display",
var(--sans)`. Overriding `--sans` alone would leave both leading with a Segoe face that is
installed on every Windows laptop, so `--t-meta` and `--t-head` text would stay Segoe while the
body changed: a comparison of half a face. Each candidate rule sets all three.

**Why variable faces.** `--t-head-weight` is `560` (task 26, `tokens.css` after that task). A
static 500 or 600 file would synthesise or snap, and the owner would be judging a rendering
accident. Each candidate is one variable `woff2` with a weight range; whether each project ships a
variable `woff2` is **UNVERIFIED** until step 1 fetches it.

**Files:**
- `desktop/src/lib/flags.ts` — `Look`, `SansCandidate`, `ThemeName`, `Flags`, `getFlags`,
  `setFlag`, `useFlags`, `resetFlags`, `FLAG_DEFAULTS` (new)
- `desktop/src/lib/flags.test.ts` (new)
- `desktop/src/components/devtools/mount.ts` — the `DEV` gate (new; not in the skeleton's map)
- `desktop/src/components/devtools/DevToolbar.tsx` — the container (new; not in the map)
- `desktop/src/components/devtools/FlagCompare.tsx` — the three radio groups, `SANS_CANDIDATES`
  (new)
- `desktop/src/components/devtools/devtools.css` — owns `.dt-*` (new)
- `desktop/src/components/devtools/FlagCompare.test.tsx` (new)
- `desktop/src/devtools/fonts/candidates.css` — the two `@font-face`s and the two `data-sans`
  rules (new)
- `desktop/src/devtools/fonts/AtkinsonHyperlegibleNext-Variable.woff2`,
  `Geist-Variable.woff2`, `OFL-AtkinsonHyperlegibleNext.txt`, `OFL-Geist.txt`,
  `fonts.lock.json` — fetched at step 1 (new; names are ours, upstream names in the lock)
- `desktop/src/App.tsx` ⚑ (241 lines; `return <AppShell />;` at `:240`, imports `:11-20`) — mounts
  the toolbar (modified; not in the skeleton's file map)

`defects.test.tsx:193-201` pins that `App` adds no chrome: `container.firstElementChild` is the
shell. The toolbar therefore renders through a **portal into `document.body`**, outside App's
root, and that test holds unchanged in a dev and test build alike.

#### Interfaces — complete: `desktop/src/lib/flags.ts`

```ts
// desktop/src/lib/flags.ts
//
// flag.compare (Phase W task 32; 02 C12). Dev-session preferences that flip the look on the SAME
// screen: each flag is one attribute on <html> that a stylesheet already reads, so a flip is a
// style recalculation, never a remount or a reload.
//
// In memory only. Nothing here touches localStorage, indexedDB or a cookie; a reload is the
// defaults. A flag at its default is NO attribute, so a DOM nobody flipped is the production DOM.
//
// This module names no sans candidate at run time. The candidate list is FlagCompare.tsx's, which
// production never loads, and production code may import this file (task 38 reads `look`).

import { useSyncExternalStore } from "react";

export type Look = "old" | "new";
export type SansCandidate = "segoe" | "atkinson" | "geist";
export type ThemeName = "graphite" | "paper";

export interface Flags {
  look: Look;
  sans: SansCandidate;
  theme: ThemeName;
}

/** The two places a flag is named: its default here, its attribute below. Task 35 adds `grain`
 *  to both and to `Flags`, and nothing else in this file changes. */
export const FLAG_DEFAULTS: Readonly<Flags> = Object.freeze({
  look: "new",
  sans: "segoe",
  theme: "graphite",
});

const ATTRIBUTE: { readonly [K in keyof Flags]: string } = {
  look: "data-look",
  sans: "data-sans",
  theme: "data-theme",
};

let current: Readonly<Flags> = FLAG_DEFAULTS;
const listeners = new Set<() => void>();

function notify(): void {
  for (const listener of listeners) listener();
}

/** The same frozen object until a flag changes. useSyncExternalStore compares snapshots with
 *  Object.is, so a fresh object per call would re-render forever. */
export function getFlags(): Flags {
  return current;
}

export function setFlag<K extends keyof Flags>(key: K, value: Flags[K]): void {
  if (current[key] === value) return;
  current = Object.freeze({ ...current, [key]: value });
  const root = document.documentElement;
  if (value === FLAG_DEFAULTS[key]) root.removeAttribute(ATTRIBUTE[key]);
  else root.setAttribute(ATTRIBUTE[key], value);
  notify();
}

/** Defaults, and every flag attribute removed even if memory already said default: a test that
 *  set an attribute by hand leaves nothing behind. The toolbar's *Reset* calls it too. */
export function resetFlags(): void {
  const changed = current !== FLAG_DEFAULTS;
  current = FLAG_DEFAULTS;
  for (const name of Object.values(ATTRIBUTE)) document.documentElement.removeAttribute(name);
  if (changed) notify();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useFlags(): Flags {
  return useSyncExternalStore(subscribe, getFlags, getFlags);
}
```

#### Interfaces — complete: `mount.ts`, `DevToolbar.tsx` and the `App.tsx` edit

```ts
// desktop/src/components/devtools/mount.ts
//
// The one gate between the product and the dev toolbar (Phase W task 32). Vite replaces
// import.meta.env.DEV with a literal at build time, so in a production build the import() below
// is dead code. The acceptance grep over dist/ is what proves no chunk was emitted for it.

import { lazy } from "react";
import type { JSX, LazyExoticComponent } from "react";

export const DevToolbar: LazyExoticComponent<() => JSX.Element> | null = import.meta.env.DEV
  ? lazy(() => import("./DevToolbar"))
  : null;
```

```tsx
// desktop/src/components/devtools/DevToolbar.tsx
//
// The dev toolbar: flag.compare now, stress.sets (task 33) and the grain meter (task 35) beside
// it. Portalled into <body>, outside App's root, so the assembled-app test that App adds no
// chrome (defects.test.tsx:193-201) still holds. A <details>, closed by default, so it covers
// nothing until opened; no keyboard chord, so no cockpit chord is shadowed.

import "./devtools.css";
import "../../devtools/fonts/candidates.css";
import type { JSX } from "react";
import { createPortal } from "react-dom";
import { resetFlags } from "../../lib/flags";
import { FlagCompare } from "./FlagCompare";

export default function DevToolbar(): JSX.Element {
  return createPortal(
    <details className="dt">
      <summary className="dt-summary">Dev</summary>
      <div className="dt-body">
        <FlagCompare />
        {/* task 33: <StressSets /> */}
        <button type="button" className="dt-reset" onClick={resetFlags}>
          Reset
        </button>
      </div>
    </details>,
    document.body,
  );
}
```

`createPortal` returns a `ReactPortal`, which is a `ReactElement` and so a `JSX.Element`; the
return type typechecks without a cast (**UNVERIFIED** against `@types/react` 19 until step 6's
typecheck runs; the fallback is `<>{createPortal(…)}</>`).

`App.tsx`, the import line `:11` and the return at `:240`:

```tsx
import { Suspense, useEffect } from "react";
// … the imports at :12-20 unchanged, then:
import { DevToolbar } from "./components/devtools/mount";
```

```tsx
  return (
    <>
      <AppShell />
      {DevToolbar !== null && (
        <Suspense fallback={null}>
          <DevToolbar />
        </Suspense>
      )}
    </>
  );
```

The comment at `:229-230` ("this component renders AppShell and nothing else") gains one
sentence: *In a dev build it also renders the dev toolbar, which portals out of this root and owns
no store and no poller.*

#### Interfaces — complete: `FlagCompare.tsx`

```tsx
// desktop/src/components/devtools/FlagCompare.tsx
//
// flag.compare's three groups (02 C12; 05 §4.4 C3). Radio groups in a <fieldset>, so each group
// is one tab stop and the arrow keys move within it, native behaviour with no handler of ours.
// No heading element: task 31's audit fails any second header pattern.

import type { JSX } from "react";
import { setFlag, useFlags } from "../../lib/flags";
import type { Flags, Look, SansCandidate, ThemeName } from "../../lib/flags";

export interface FlagOption<V extends string> {
  value: V;
  label: string;
}

export const LOOKS: readonly FlagOption<Look>[] = [
  { value: "new", label: "New" },
  { value: "old", label: "Old (Phase 2)" },
];

/** Q14. `family` must equal the first family candidates.css gives the value's rule; the test
 *  reads both. Segoe UI Variable is OD-28's choice and has no rule: it is the default. */
export const SANS_CANDIDATES: readonly (FlagOption<SansCandidate> & { family: string })[] = [
  { value: "segoe", label: "Segoe UI Variable (OD-28)", family: "Segoe UI Variable Text" },
  { value: "atkinson", label: "Atkinson Hyperlegible Next", family: "Atkinson Hyperlegible Next" },
  { value: "geist", label: "Geist", family: "Geist" },
];

export const THEMES: readonly FlagOption<ThemeName>[] = [
  { value: "graphite", label: "Graphite" },
  { value: "paper", label: "Paper (provisional)" },
];

function Group<K extends keyof Flags>(props: {
  flag: K;
  legend: string;
  options: readonly FlagOption<Flags[K]>[];
}): JSX.Element {
  const flags = useFlags();
  return (
    <fieldset className="dt-group">
      <legend className="dt-legend">{props.legend}</legend>
      {props.options.map((option) => (
        <label key={option.value} className="dt-option">
          <input
            type="radio"
            name={`dt-${props.flag}`}
            value={option.value}
            checked={flags[props.flag] === option.value}
            onChange={() => setFlag(props.flag, option.value)}
          />
          {option.label}
        </label>
      ))}
    </fieldset>
  );
}

export function FlagCompare(): JSX.Element {
  return (
    <>
      <Group flag="look" legend="Look" options={LOOKS} />
      <Group flag="sans" legend="Sans" options={SANS_CANDIDATES} />
      <Group flag="theme" legend="Theme" options={THEMES} />
    </>
  );
}
```

`paper` is task 26's **provisional** block: task 26 says nothing sets `data-theme` until a sheet
renders it by hand. This toolbar is that hand. The label says *provisional* so nobody reads the
paper look as reviewed.

#### Interfaces — complete: `devtools.css`

```css
/* desktop/src/components/devtools/devtools.css
 *
 * The dev toolbar (Phase W tasks 32, 33, 35). Dev builds only: mount.ts loads it behind
 * import.meta.env.DEV. It still keeps the design system, because the owner reviews the product
 * with this panel open beside it. Elevation is the raised ring plus shadow, never a border. No
 * transition: it opens and closes without motion, because nothing about opening it is causal.
 * z-index sits under the grain overlay (9999, tokens.css .grain::after), so the grain covers it
 * as it covers the rest of the window. */
@import "../../styles/tokens.css";

.dt {
  position: fixed;
  inset-block-end: var(--s-3);
  inset-inline-end: var(--s-3);
  z-index: 9000;
  max-inline-size: 280px;
  background: var(--surface);
  border-radius: var(--r-card);
  box-shadow: var(--e-raised);
  color: var(--ink-2);
  font-family: var(--sans);
  font-size: var(--t-meta);
  line-height: var(--t-meta-lh);
}

.dt-summary {
  cursor: pointer;
  padding: var(--s-2) var(--s-3);
  color: var(--ink);
}

.dt-body {
  display: grid;
  gap: var(--s-3);
  padding: 0 var(--s-3) var(--s-3);
}

.dt-group {
  display: grid;
  gap: var(--s-1);
  margin: 0;
  padding: 0;
  border: 0;
}

.dt-legend {
  padding: 0;
  color: var(--ink-2);
}

.dt-option {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  min-block-size: var(--row-h);
  color: var(--ink);
}

.dt-option input {
  margin: 0;
  accent-color: var(--accent);
}

.dt-reset {
  justify-self: start;
  padding: var(--s-1) var(--s-2);
  border: 0;
  border-radius: var(--r-ctl);
  background: var(--surface);
  box-shadow: var(--e-btn);
  color: var(--ink);
  font: inherit;
}
```

Longhands, not the `font` shorthand, on `.dt`: the audit's numeric rule (`designsystem.test.ts:674`)
fails a shorthand that resets `font-variant-numeric` on a carrier. `.dt-reset`'s `font: inherit`
is `app.css:47`'s own form for controls. The accent on a checked radio marks the current choice,
which is what 02 C5 allows accent for.

#### Interfaces — complete: `desktop/src/devtools/fonts/candidates.css`

```css
/* desktop/src/devtools/fonts/candidates.css
 *
 * Q14's two alternative sans faces (Phase W task 32; 05 §4.4 C3, revision 2). DEV ONLY:
 * DevToolbar.tsx imports this file, and mount.ts loads DevToolbar behind import.meta.env.DEV, so
 * a production build carries neither face nor either name. OD-28 chose Segoe UI Variable, which
 * has no rule here because it is the default; these two exist so the owner confirms it by eye.
 *
 * Every rule sets all three sans tokens. Task 27's --sans-small and --sans-display lead with a
 * Segoe optical size that every Windows laptop has installed, so overriding --sans alone would
 * change the body and leave --t-meta and --t-head text in Segoe.
 *
 * One variable file per family, because --t-head-weight is 560. The weight ranges below are the
 * files' wght axes as step 1 reads them; until then they are UNVERIFIED. Each family's OFL text
 * and its fonts.lock.json entry sit in this directory, not in src/assets/fonts/, whose manifest
 * lists only what ships. */
@import "../../styles/tokens.css";

@font-face {
  font-family: "Atkinson Hyperlegible Next";
  src: url("./AtkinsonHyperlegibleNext-Variable.woff2") format("woff2");
  font-weight: 200 800;
  font-style: normal;
  font-display: block;
}

@font-face {
  font-family: "Geist";
  src: url("./Geist-Variable.woff2") format("woff2");
  font-weight: 100 900;
  font-style: normal;
  font-display: block;
}

:root[data-sans="atkinson"] {
  --sans: "Atkinson Hyperlegible Next", system-ui, sans-serif;
  --sans-small: "Atkinson Hyperlegible Next", system-ui, sans-serif;
  --sans-display: "Atkinson Hyperlegible Next", system-ui, sans-serif;
}

:root[data-sans="geist"] {
  --sans: "Geist", system-ui, sans-serif;
  --sans-small: "Geist", system-ui, sans-serif;
  --sans-display: "Geist", system-ui, sans-serif;
}
```

`:root[data-sans="…"]` is specificity (0,2,0) against task 27's `:root` (0,1,0), so the rule wins
whatever order Vite emits the two sheets in.

#### Interfaces — complete: `desktop/src/devtools/fonts/fonts.lock.json`

Task 27's manifest shape, so one reader could check both. Values are step 1's.

```json
{
  "atkinson-hyperlegible-next": {
    "family": "Atkinson Hyperlegible Next",
    "licence": "<SPDX id as the release states it, UNVERIFIED until fetched>",
    "licenceFile": "OFL-AtkinsonHyperlegibleNext.txt",
    "release": "<release or commit — UNVERIFIED until fetched>",
    "source": "<the Braille Institute or google/fonts URL it was fetched from>",
    "files": [
      { "ours": "AtkinsonHyperlegibleNext-Variable.woff2", "upstream": "<name>", "sha256": "<64 hex>" }
    ]
  },
  "geist": {
    "family": "Geist",
    "licence": "<SPDX id, UNVERIFIED until fetched; expected OFL-1.1>",
    "licenceFile": "OFL-Geist.txt",
    "release": "<release tag — UNVERIFIED until fetched>",
    "source": "https://github.com/vercel/geist-font/releases/tag/<tag>",
    "files": [
      { "ours": "Geist-Variable.woff2", "upstream": "<name>", "sha256": "<64 hex>" }
    ]
  }
}
```

The Geist repository and its licence are from memory, not fetched: **UNVERIFIED**. Atkinson
Hyperlegible Next's licence and weight range are **UNVERIFIED**, as 05 §4.4 C3 already records.
If either licence is not on `OD-11`'s allowed list (MIT, Apache, BSD, ISC, OFL), that face is
dropped from `SANS_CANDIDATES` and the owner is told, rather than fetched under another licence.

#### Tests to write first — `desktop/src/lib/flags.test.ts`

```ts
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FLAG_DEFAULTS, getFlags, resetFlags, setFlag, useFlags } from "./flags";

const root = () => document.documentElement;

afterEach(() => {
  resetFlags();
  vi.unstubAllGlobals();
});

describe("flags", () => {
  it("writes each flag to the root element", () => {
    // Kills: a flag kept in memory that never reaches CSS, or one written to <body>.
    setFlag("look", "old");
    setFlag("sans", "geist");
    setFlag("theme", "paper");
    expect(root().getAttribute("data-look")).toBe("old");
    expect(root().getAttribute("data-sans")).toBe("geist");
    expect(root().getAttribute("data-theme")).toBe("paper");
    expect(document.body.hasAttribute("data-look")).toBe(false);
  });

  it("removes the attribute when a flag returns to its default", () => {
    // Kills: data-look="new" left on <html>, so a flipped-back DOM is not the production DOM.
    setFlag("look", "old");
    setFlag("look", "new");
    setFlag("sans", "atkinson");
    setFlag("sans", "segoe");
    expect(root().hasAttribute("data-look")).toBe(false);
    expect(root().hasAttribute("data-sans")).toBe(false);
    expect(getFlags()).toEqual(FLAG_DEFAULTS);
  });

  it("returns one snapshot until a flag changes", () => {
    // Kills: a getSnapshot that builds a fresh object per call (a render loop under
    // useSyncExternalStore), and a no-op set that replaces the snapshot anyway.
    const before = getFlags();
    expect(getFlags()).toBe(before);
    setFlag("look", "new");
    expect(getFlags()).toBe(before);
    setFlag("look", "old");
    expect(getFlags()).not.toBe(before);
    expect(Object.isFrozen(getFlags())).toBe(true);
  });

  it("re-renders a subscriber on a change and not on a no-op", () => {
    // Kills: a toolbar whose radios lag the attribute, and a notify on every call.
    let renders = 0;
    const { result } = renderHook(() => {
      renders += 1;
      return useFlags();
    });
    const settled = renders;
    act(() => setFlag("theme", "graphite"));
    expect(renders).toBe(settled);
    act(() => setFlag("theme", "paper"));
    expect(result.current.theme).toBe("paper");
    expect(renders).toBe(settled + 1);
  });

  it("persists nothing", () => {
    // Kills: a flag written to disk, which would make a dev preference outlive the session and
    // leak into the next person's review.
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const open = vi.fn();
    vi.stubGlobal("indexedDB", { open });
    setFlag("look", "old");
    setFlag("sans", "atkinson");
    setFlag("theme", "paper");
    resetFlags();
    expect(setItem).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
    expect(document.cookie).toBe("");
  });
});
```

jsdom has no `indexedDB`, so the stub is the only one a write could reach. `renderHook`'s first
render count is read after mount (`settled`) because `StrictMode` is not in play in `renderHook`
and React may still render twice in development; the assertion is on the difference.

#### Tests to write first — `desktop/src/components/devtools/FlagCompare.test.tsx`

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import candidatesCss from "../../devtools/fonts/candidates.css?raw";
import { resetFlags } from "../../lib/flags";
import { stripCssComments } from "../../test/fakes";
import DevToolbar from "./DevToolbar";
import { FlagCompare, SANS_CANDIDATES } from "./FlagCompare";

afterEach(() => {
  resetFlags();
  vi.unstubAllEnvs();
  vi.resetModules();
});

/** The declarations of one rule, by exact selector. */
function ruleBody(css: string, selector: string): string | null {
  const bare = stripCssComments(css);
  const at = bare.indexOf(`${selector} {`);
  if (at < 0) return null;
  const open = bare.indexOf("{", at);
  return bare.slice(open + 1, bare.indexOf("}", open));
}

describe("flag.compare", () => {
  it("flips the look without remounting", async () => {
    // Kills: a flip that reloads, or re-keys the app so its state is lost.
    render(
      <>
        <input aria-label="probe" />
        <FlagCompare />
      </>,
    );
    const probe = screen.getByLabelText("probe");
    await userEvent.type(probe, "half a sentence");
    await userEvent.click(screen.getByRole("radio", { name: "Old (Phase 2)" }));
    expect(document.documentElement.getAttribute("data-look")).toBe("old");
    expect(screen.getByLabelText("probe")).toBe(probe);
    expect(probe).toHaveProperty("value", "half a sentence");
    expect(screen.getByRole("radio", { name: "Old (Phase 2)" })).toHaveProperty("checked", true);
  });

  it("applies each sans candidate", async () => {
    // Kills: a candidate the CSS ignores, a rule that sets --sans alone, a family-name typo.
    render(<FlagCompare />);
    for (const candidate of SANS_CANDIDATES) {
      await userEvent.click(screen.getByRole("radio", { name: candidate.label }));
      const body = ruleBody(candidatesCss, `:root[data-sans="${candidate.value}"]`);
      if (candidate.value === "segoe") {
        expect(document.documentElement.hasAttribute("data-sans")).toBe(false);
        expect(body).toBeNull();
        continue;
      }
      expect(document.documentElement.getAttribute("data-sans")).toBe(candidate.value);
      for (const token of ["--sans", "--sans-small", "--sans-display"]) {
        expect(body).toMatch(new RegExp(`${token}:\\s*"${candidate.family}",`));
      }
      expect(candidatesCss).toContain(`font-family: "${candidate.family}";`);
    }
  });

  it("offers OD-28's choice as the default", () => {
    // Kills: a toolbar that opens on a candidate the owner did not choose.
    render(<FlagCompare />);
    expect(screen.getByRole("radio", { name: "Segoe UI Variable (OD-28)" })).toHaveProperty("checked", true);
    expect(screen.getByRole("radio", { name: "New" })).toHaveProperty("checked", true);
  });

  it("renders outside the app root", () => {
    // Kills: a toolbar rendered in place, which breaks defects.test.tsx:199-200's "no chrome".
    const { container } = render(<DevToolbar />);
    expect(container.childElementCount).toBe(0);
    expect(document.body.querySelector("details.dt")).not.toBeNull();
  });

  it("renders nothing in production", async () => {
    // Kills: shipping dev chrome, a gate on MODE or on a variable Vite does not replace.
    vi.stubEnv("DEV", false);
    vi.resetModules();
    expect((await import("./mount")).DevToolbar).toBeNull();
    vi.stubEnv("DEV", true);
    vi.resetModules();
    expect((await import("./mount")).DevToolbar).not.toBeNull();
  });
});
```

`vi.stubEnv` takes a boolean for `DEV`, `PROD` and `SSR` in vitest's documentation; that 5.0.1
keeps that overload is **UNVERIFIED** until step 2 runs it. `?raw` on a CSS import is the tree's
existing pattern (`vitest.config.ts`'s `css.include`), and `ruleBody` strips comments before it
looks, because the file's header comment names both selectors.

#### Steps

1. Fetch the two faces into `desktop/src/devtools/fonts/`: one variable `woff2` each, the OFL text
   each release ships, and the `fonts.lock.json` entries (release, source, upstream name,
   SHA-256). Read each file's `wght` axis and set the two `font-weight` ranges from it. If a
   licence is not a permissive one, drop that face from `SANS_CANDIDATES` and record why.
2. Write `flags.test.ts` and `FlagCompare.test.tsx`. Run
   `npm --prefix desktop test -- flags devtools`: both fail to import.
3. Write `flags.ts`. The five flags tests pass.
4. Write `mount.ts`, `DevToolbar.tsx`, `FlagCompare.tsx`, `devtools.css` and `candidates.css`.
   The five toolbar tests pass.
5. Edit `App.tsx` (the import, the return, the comment). Run
   `npm --prefix desktop test -- defects designsystem`: `defects.test.tsx:193-201` passes
   unchanged; the audit passes over the two new stylesheets (token import first, no literal
   colour, no transition). If React logs a suspended-outside-`act` warning from the lazy toolbar
   in `defects`, that is a test-noise defect to fix here, not to ignore.
6. Mutate by hand, one at a time, and watch the named test go red, then revert:
   `root().setAttribute(ATTRIBUTE[key], value)` for every value (the default test);
   `return { ...current }` in `getFlags` (the snapshot test); drop `--sans-small` from the
   geist rule (the candidate test); `import.meta.env.MODE === "development"` in `mount.ts` (the
   production test).
7. Run `npm --prefix desktop run build`, then the grep below. If it prints a file, the dead
   branch was not eliminated: move the gate into `vite.config.ts` as a `resolve.alias` that maps
   `./components/devtools/mount` to a module exporting `DevToolbar = null` when
   `mode === "production"`, and re-run.
8. Run the typecheck and the full desktop suite. `git add` the new files; commit with
   `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- flags devtools defects designsystem
# expect: every test passes: flags' five, flag.compare's five, the assembled-app suite
# unchanged, and the audit over devtools.css and candidates.css

npm --prefix desktop run typecheck
# expect: exit 0, no output from tsc

npm --prefix desktop run build && grep -rli "geist\|atkinson" desktop/dist
# expect: the build succeeds, then grep prints nothing (exit 1)

grep -rl "DevToolbar\|dt-summary" desktop/dist
# expect: nothing; the toolbar's module and stylesheet are not in the production bundle
```

Then, by hand, in `npm --prefix desktop run tauri dev`: open *Dev*, type in the palette, flip
*Old* and back, and each sans. The typed text stays; the rail and the Develop pane change look.
That is the owner's two-second comparison working; it is not an acceptance of either look.

#### What this task does not claim

- **Which look or face is better.** The toolbar makes the comparison cheap; the owner decides at
  task 39 (`OD-1`), and `OD-28` stands until the owner changes it.
- **A keyboard chord for the toolbar.** There is none, so no cockpit chord (task 64's key map,
  AGENTIC-UX's `Space`) is shadowed. It is reached by pointer or by Tab.
- **macOS or Linux rendering.** Segoe UI Variable is absent there and the stacks fall to the
  platform face; nobody has looked, and no macOS build exists.
- **That a production build carries none of it** until step 7's grep has run. Dead-branch
  elimination of a dynamic import under Vite 8 is **UNVERIFIED**; the grep is the proof.
- **The two licences and weight ranges** until step 1 fetches the files (**UNVERIFIED**).
- **A theme setting.** `theme` is a dev flag over task 26's provisional `paper` block. When C11's
  theme picker ships a user-facing `data-theme`, that task takes `theme` out of `Flags`, because
  two writers of one attribute is a defect.
- **That every visual change is behind the flag.** Tasks 30 and 31 kept their old look under
  `:root[data-look="old"]`; later visual tasks must do the same for the toggle to mean anything,
  and task 39 deletes every old-look block once the owner has chosen.
- Design: 05 §4.4 C3, C12; 02 C12, D2 step 7; `OD-1`, `OD-28`. Parity: no 01 row; it serves the
  review gate.

**Interfaces:**
- Produces: `Look`, `SansCandidate`, `ThemeName`, `Flags`, `FLAG_DEFAULTS`, `getFlags()`,
  `setFlag(key, value)`, `useFlags()`, `resetFlags()` in `desktop/src/lib/flags.ts`; the
  attributes `data-look="old"`, `data-sans="atkinson" | "geist"`, `data-theme="paper"` on
  `<html>`, each absent at its default
- Produces for task 33: `DevToolbar.tsx`'s body, where `<StressSets />` goes beside
  `<FlagCompare />`; the `.dt-*` classes; `mount.ts`'s gate, which task 33's production test reuses
- Produces for task 35: `FLAG_DEFAULTS` and `ATTRIBUTE`, the two places `grain` is added, and
  `Group`'s shape for a fourth radio group in `FlagCompare`
- Produces for task 38: `useFlags().look`, the one production-safe read of the toggle
- Consumes: task 27's `--sans`, `--sans-small`, `--sans-display`; task 26's `--t-meta`,
  `--t-meta-lh`, `--s-1` … `--s-3`, `--row-h` and the `paper` block; task 30's and task 31's
  `:root[data-look="old"]` blocks; the built `--surface`, `--ink*`, `--accent`, `--r-card`,
  `--r-ctl`, `--e-raised`, `--e-btn` (`tokens.css:32`, `:110-122`, verified); `stripCssComments`
  (`test/fakes.ts:259`, verified)
- Amends: nothing in the Hub. No wire shape changes.
