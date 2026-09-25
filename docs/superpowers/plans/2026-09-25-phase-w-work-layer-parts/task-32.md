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
