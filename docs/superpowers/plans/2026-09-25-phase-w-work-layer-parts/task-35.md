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
