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
