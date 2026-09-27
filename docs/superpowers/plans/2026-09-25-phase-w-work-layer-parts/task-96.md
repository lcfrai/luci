### Task 96: themes, part 2 — the high-contrast pair; personal preferences

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WD1.
**dependsOn:** 95 (the generator), after a sync-merge of `feat/work-layer` (rule H1). **Seams:**
extends **SW.11** by two more theme blocks; adds **no** interface, route or type. The preference
file is the desktop's own state and touches no Hub record.

Two seeds, one multiplier. `graphite-hc` and `paper-hc` are the same seeds as task 95's pair at
`--seed-contrast 1.5`, which is exactly why the generator exists: a high-contrast theme is a
number, not a second hand-written file. Grain is off in both, because a texture that exists to
stop a flat dark ground looking like paper is noise to someone who needs 7:1.

Then the small half: **four preferences**, and a deliberate absence. Theme, font size, the send
key for comments, and *assign issues I create to me*. There is **no home-view setting**, because
the home view is always the Desk — L-235's deliberate difference from Linear, and AX §5's whole
argument: the first screen is what needs you, not a list you chose once and stopped seeing.

#### Anchors, re-read on 2026-09-27

In this tree, verified: `desktop/src/styles/` holds `app.css`, `tokens.css`, `tokens.test.ts`,
`designsystem.test.ts` and **no `grain.css`** (`ls`, 2026-09-27); `tokens.css` carries a
`@media (prefers-reduced-motion: reduce)` block at `:173-174` and **no `forced-colors` block**
(`grep`, 2026-09-27) — task 34's layer is not merged here; there is **no `--row-h`** custom
property anywhere in `desktop/src/styles` (`grep`, 2026-09-27); `desktop/src/components/` has no
`settings` directory; `desktop/src/lib/` has no `prefs.ts`.

`OD-28` reads: *"Every id, path and time is set in **Commit Mono** (OFL, free). The sans is
**Segoe UI Variable**"* (`docs/superpowers/planning/2026-09-23-owner-decisions.md:165`). Today
`tokens.css:152` sets `--mono` to `"Cascadia Mono", ui-monospace, …` and the type comment at
`:144-148` says Phase 2 ships no webfont deliberately. **Neither face is in the tree.** That is a
gap, listed below; it is not silently fixed here.

In this branch's parts (**UNVERIFIED until merge**): task 34's `forced-colors` layer
(`tasks-21-40.md:483`); task 35's grain, *measured at 150 % in WebView2* (`:516`); task 33's
`stress.sets` and its long-title screen; task 95's `THEMES`, `deriveTheme` and `emit`. **`--row-h*`
is named by this task's scope but exists in no file today**, so it is task 29/30's token and its
name is taken on trust.

Design: `02-linear-design-language.md` C1 (four themes, no free-form picker, `:184-197`) and
`05-design.md` §1 L-234, L-235; AX §5.

#### Decisions the scope left open, made here

**(a) Contrast 1.5 scales the Δ table, and the ink targets are replaced, not scaled.** A
multiplier on a surface step is meaningful: `--surface` moves from +2.5 to +3.75 above the seed,
so planes separate further. A multiplier on a *ratio* is not: 3.43 × 1.5 is 5.145, an arbitrary
number. So at contrast ≥ 1.4 the ink targets become the stated floor — **7:1 for every ink a
person reads**, and `--ink-2` and `--ink-3` collapse towards `--ink`. That is the point: a
high-contrast theme has fewer distinct greys, on purpose.

**(b) `--ink-4` survives, and stays non-text.** It carries thread lines and hairline glyphs
(`tokens.css:43-47`). In the `-hc` themes it is re-solved to 4.5:1 rather than its 1.92:1, because
a structural line somebody cannot see is a structure that is not there — but it is still never
used for text, and the emitted comment still says so.

**(c) Hairlines stop being alpha in `-hc`.** `--line`, `--line-soft`, `--line-strong` and
`--thread` are alpha over the ink hue today (`tokens.css:51-56`), which is correct for a themed
ground and wrong at 7:1: alpha over a light ground is a mid grey. In `-hc` the generator emits
them **opaque**, solved against that theme's `--surface`, so a hairline holds its contrast on
every plane in the ramp.

**(d) Grain is off in `-hc`, by a token, not a conditional.** `grain.css` (task 35) reads
`--grain-opacity`, which the generator emits as `0` in both `-hc` blocks and as task 35's value in
the other two. No component learns which theme is active, and no `[data-theme$="-hc"]` selector
is needed in a component stylesheet.

**(e) `forced-colors` wins over every theme, and this task proves it rather than assuming it.**
Task 34's layer sits in a later cascade layer than the theme blocks; the generator emits its
themes inside `@layer tokens`, and task 34's block is outside it. A test asserts the order in the
emitted file, because getting it backwards is invisible until somebody with Windows High Contrast
on opens the app.

**(f) A preference is the desktop's, so it persists on the device (skeleton rule 3)** — and
nothing else may. The file holds **exactly four keys**, and a test asserts the key set is exactly
those four, so the first person to add `lastOpenIssueId` for convenience fails the build. The last
issue you opened is Hub data and ADR-H1 forbids it on disk.

**(g) Font size is a root multiplier with four stops, not a free number.** `small`, `default`,
`large`, `larger` map to `--type-scale` `0.9375`, `1`, `1.0625`, `1.125`, applied to the root font
size only. Row heights are in `rem` and follow; a free-form pixel box would let a person break
their own rows, and the long-title screen in `stress.sets` is the test that says which stops are
safe.

**(h) The send key is `Enter` or `Ctrl+Enter`, and it is a preference because teams disagree.**
Whichever is not *send* is *newline*. The setting names both behaviours in the label — *Enter
sends, Shift+Enter for a newline* — rather than showing a key name alone.

#### Files

| File | New / ⚑ | What it gains |
|---|---|---|
| `desktop/scripts/lib/themes.ts` | ⚑ (task 95's) | `graphite-hc`, `paper-hc`; the `-hc` ink targets |
| `desktop/scripts/tokens.gen.ts` | ⚑ (task 95's) | opaque hairlines and `--grain-opacity` per theme |
| `desktop/src/styles/tokens.css` | ⚑ | two more emitted blocks |
| `desktop/src/styles/tokens.test.ts` | ⚑ | the 7:1 floor, the layer order, the grain token |
| `desktop/src/styles/grain.css` | ⚑ (task 35's) | reads `--grain-opacity` instead of a constant |
| `desktop/src/lib/prefs.ts` | new | `Preferences`, `loadPrefs`, `savePrefs`, `applyPrefs` |
| `desktop/src/lib/prefs.test.ts` | new | the four-key suite |
| `desktop/src/components/settings/Preferences.tsx` | new | the four controls |
| `desktop/src/components/settings/Preferences.test.tsx` | new | the surface suite |

**The settings path is not in the file map.** Step 1 checks whether a settings surface exists by
then — task 84's `TeamStates.tsx` is *team* settings, not personal — and **extends it rather than
adding a second**. If none exists, `components/settings/` is created here and personal preferences
are its first tab. Either way there is exactly one settings surface in the app.

#### The two new theme specs, appended to task 95's `THEMES`

```ts
// desktop/scripts/lib/themes.ts — appended
{ name: "graphite-hc", selector: ':root[data-theme="graphite-hc"]', colorScheme: "dark",
  seeds: { base: { l: 16, c: 0.0045, h: 67 }, accent: { l: 70.3, c: 0.1092, h: 202.7 },
           contrast: 1.5, direction: "dark" } },
{ name: "paper-hc", selector: ':root[data-theme="paper-hc"]', colorScheme: "light",
  seeds: { base: { l: 97.2, c: 0.0035, h: 73 }, accent: { l: 48.0, c: 0.1092, h: 202.7 },
           contrast: 1.5, direction: "light" } },
```

```ts
/** Decision (a): at high contrast the targets are replaced, not multiplied. */
export const HC_CONTRAST_FLOOR = 1.4;
export const HC_INK_TARGETS: readonly { name: string; ratio: number; c: number; h: number }[] = [
  { name: "--ink",   ratio: 17.09, c: 0.0060, h: 73 },   // chroma drops: colour costs contrast
  { name: "--ink-2", ratio: 10.00, c: 0.0080, h: 68 },
  { name: "--ink-3", ratio:  7.00, c: 0.0080, h: 64 },   // the floor every read ink must clear
];
export const HC_INK_4_RATIO = 4.5;                        // structural, still never text (b)

/** Decision (d). Emitted per theme; grain.css reads it and no component knows the theme. */
export const GRAIN_OPACITY: Readonly<Record<string, string>> = {
  graphite: "0.018", paper: "0.012", "graphite-hc": "0", "paper-hc": "0",
};
```

`deriveTheme` picks `HC_INK_TARGETS` when `seeds.contrast >= HC_CONTRAST_FLOOR` and emits
hairlines opaque under the same condition (decision c). That is **two branches in the generator
and no new file**, which is the whole argument for task 95 having been built first.

#### `desktop/src/lib/prefs.ts`, complete

```ts
// desktop/src/lib/prefs.ts
//
// THE DESKTOP'S OWN state, and nothing else. Four keys. A fifth is a bug: if you are about to
// add "the last issue I had open" or "my default project", that is Hub data and ADR-H1 forbids
// it on this disk. prefs.test.ts asserts the key set is exactly these four.

export type ThemeName = "graphite" | "paper" | "graphite-hc" | "paper-hc";
export type FontSize = "small" | "default" | "large" | "larger";
export type SendKey = "enter" | "ctrl-enter";

export interface Preferences {
  readonly theme: ThemeName;
  readonly fontSize: FontSize;
  readonly sendKey: SendKey;
  readonly assignToMe: boolean;
}

export const PREF_KEYS = ["theme", "fontSize", "sendKey", "assignToMe"] as const;

export const DEFAULT_PREFS: Preferences = {
  theme: "graphite", fontSize: "default", sendKey: "ctrl-enter", assignToMe: false,
};

/** Decision (g). Applied to the root font size only; every row height is in rem and follows. */
export const TYPE_SCALE: Readonly<Record<FontSize, number>> = {
  small: 0.9375, default: 1, large: 1.0625, larger: 1.125,
};
```

```ts
export interface PrefsDeps {
  /** The Tauri app-config directory. Injected so every test runs in memory. */
  readonly read: () => Promise<string | null>;
  readonly write: (json: string) => Promise<void>;
  readonly root: () => HTMLElement;
}

/** Unknown keys are DROPPED, not merged; an unknown value falls back to its default. */
export function parsePrefs(json: string | null): Preferences;

export function loadPrefs(deps: PrefsDeps): Promise<Preferences>;
export function savePrefs(deps: PrefsDeps, next: Preferences): Promise<void>;

/** Sets `data-theme`, `--type-scale` and `color-scheme` on the root. Pure DOM, no store. */
export function applyPrefs(root: HTMLElement, p: Preferences): void;
```

`parsePrefs` dropping unknown keys is what makes decision (f) self-healing: a preference file
written by a future build that added a fifth key is read back as four, and `savePrefs` writes four.
The file can never grow by accident, only by a reviewed change to `PREF_KEYS`.

#### `Preferences.tsx`'s four controls, and the one absence

| Control | Shape | Note |
|---|---|---|
| Theme | four radio cards, each a live swatch of its own `--page`/`--surface`/`--ink`/`--accent` | no picker, no gallery (02 C1) |
| Text size | a four-stop segmented control | decision (g) |
| Sending a comment | two radios, each labelled with **both** behaviours | decision (h) |
| Assign issues I create to me | a switch | L-235's personal default |

There is **no home-view control**, and that absence is tested. It is not an omission: AX §5 and
L-235 make the Desk the home view for everyone, deliberately unlike Linear, because a home view a
person chose in their first week is a list they stop seeing by their third.

Below the four, one `--ink-3` line: *Everything else you see — your views, your drafts, what you
have read — lives in the Hub and follows you to another machine.* That sentence is the ADR-H1 rule
said to a person rather than enforced behind their back.

#### Tests to write first, each with the mutant it kills

`prefs.test.ts` (pure, `PrefsDeps` faked), `Preferences.test.tsx` (vitest +
`@testing-library/react`) and four additions to `tokens.test.ts`. Each is seen failing first.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `every -hc ink clears 7:1 on its ground` | `--ink`, `--ink-2`, `--ink-3` ≥ 7:1 against `--page`, `--surface` and `--inset` in `graphite-hc` **and** `paper-hc` | `-hc` emitted at contrast 1.0; targets multiplied rather than replaced |
| `-hc hairlines are opaque and visible` | no `/ 0.` alpha form in the `-hc` blocks' `--line*` or `--thread`; each ≥ 3:1 on `--surface` | alpha hairlines carried into high contrast, invisible on a light ground |
| `--ink-4 is still not text` | its `-hc` value solves to 4.5:1 and its emitted comment still reads `non-text only`; no component stylesheet uses it on a text property | a structural token promoted to text |
| `grain is absent in -hc` | `--grain-opacity` is `0` in both `-hc` blocks; `grain.css` contains no `data-theme` selector | a conditional in the component; grain left on at 7:1 |
| `forced-colors still wins over any theme` | in the emitted file, task 34's `forced-colors` block appears **after** every theme block and outside `@layer tokens`; a `data-theme` override of a system colour is absent | a theme rule that beats Windows High Contrast (task 34's layer) |
| `the preference file holds only four keys` | `savePrefs` writes exactly `PREF_KEYS`; a file containing `lastOpenIssueId` parses to four keys and is rewritten without it; the JSON contains no `is-`, no `df-` and no `vw-` | the last-open issue saved — Hub data on disk (ADR-H1) |
| `an unknown value falls back` | `{"theme":"neon"}` → `graphite`, no throw | a crash on a hand-edited file; an unknown theme applied |
| `there is no home-view control` | the rendered surface has no control whose label or `name` matches `/home|default view|start(ing)? (view|screen)/i` | a home-view setting that quietly undoes AX §5 |
| `a font-size change re-flows rows without breaking them` | in `stress.sets`'s long-title screen at every stop, each row's height equals its `--row-h*` × scale and no title overflows its cell | a pixel font size; a row height in `px` that stops following |
| `applying a theme sets three things` | `data-theme`, `--type-scale` and `color-scheme` on the root; nothing else is written; re-applying is idempotent | a theme applied by swapping a stylesheet (a flash on every change) |
| `the four themes are the only four` | the theme control renders exactly four options and `ThemeName` has exactly four members | a free-form picker (02 C1) |

#### Steps

- [ ] **Step 1: Check the base and find the settings surface.** Task 95 merged; task 34's
  `forced-colors` layer present (if not, its assertion is written and skipped with a named reason,
  not deleted); task 35's `grain.css` present; whether any personal settings surface exists. Decide
  extend-or-create once, here.
- [ ] **Step 2: The tests.** All eleven. `npm --prefix desktop test`: they fail.
- [ ] **Step 3: The two theme specs and the generator's two branches**, then re-emit and read the
  diff: two new blocks and the `--grain-opacity` line, nothing else.
- [ ] **Step 4: `prefs.ts`**, pure, green before any component.
- [ ] **Step 5: `Preferences.tsx`** and the `grain.css` token swap.
- [ ] **Step 6: Green and commit by pathspec**, the nine files of *Files*, naming the five ⚑ as
  touched.

#### Acceptance

```bash
npm --prefix desktop run tokens
npm --prefix desktop run tokens -- --check ; echo "check=$?"
npm --prefix desktop test -- --run src/lib/prefs.test.ts 2>&1 | tail -5
npm --prefix desktop test -- --run src/components/settings 2>&1 | tail -5
npm --prefix desktop test -- --run src/styles 2>&1 | tail -5
npm --prefix desktop test 2>&1 | tail -5
npm --prefix desktop run typecheck
grep -c 'data-theme="graphite-hc"' desktop/src/styles/tokens.css        # 1
grep -c 'data-theme="paper-hc"' desktop/src/styles/tokens.css           # 1
grep -n -- '--grain-opacity' desktop/src/styles/tokens.css | wc -l      # 4
grep -rn 'data-theme' desktop/src/styles/grain.css | wc -l              # 0
grep -rniE 'home view|default view|starting screen' desktop/src/components/settings | wc -l  # 0
```

Expected: `check=0`; each suite passes and the whole suite passes; `typecheck` silent, exit 0; the
greps print `1`, `1`, `4`, `0`, `0`. **None has been run; `prefs.ts`, `components/settings/` and
the two `-hc` blocks do not exist** (OD-27).

#### What this task does not claim

- **Not a measured 7:1 in WebView2 at 150 % scaling.** The ratios are computed in sRGB from the
  emitted numbers. Subpixel rendering, the display's gamma and Windows' own scaling are not
  modelled. **UNVERIFIED** until task 97's owner look.
- **Not Windows High Contrast conformance.** Task 34 owns the `forced-colors` layer; this task
  only asserts that no theme block beats it.
- **Not a theme gallery** (05 §1 L-234: *"No gallery"*) and not a custom-theme picker.
- **Not personal auto-assign rules.** *Assign issues I create to me* is one switch; rules — *when
  an issue is labelled `security`, assign it to me* — are **L-40, task 176**.
- **Not syncing preferences across machines.** A preference is the device's (skeleton rule 3); a
  second laptop starts at the defaults, and the `--ink-3` line under the controls says so.
- **Not a font face change.** See correction 3: OD-28's faces are not in the tree and this task
  does not quietly install them.
- **Not an accessibility audit.** Two ratio floors and a `forced-colors` ordering assertion are
  not a screen-reader pass; task 66 owns the a11y floor.

#### Corrections to the scope

1. **Contrast 1.5 cannot multiply a ratio.** The scope reads *"`--seed-contrast` … multiplies
   every lightness step"*, which is right for surfaces and meaningless for an ink target. Decision
   (a) replaces the ink targets above `HC_CONTRAST_FLOOR`; without that the `-hc` themes would
   emit inks at 5.1:1 and fail their own test.
2. **Hairlines must stop being alpha in `-hc`**, which no scope mentions. Alpha over a light
   ground is a mid grey; at 7:1 that is a structure a person cannot see.
3. **OD-28's two faces are not in the tree, and this task does not install them.** `tokens.css:152`
   still sets `--mono` to Cascadia Mono, and the type comment at `:144-148` says Phase 2 ships no
   webfont on purpose. OD-28 requires **Commit Mono** (OFL) and **Segoe UI Variable**
   (`2026-09-23-owner-decisions.md:165`). A font swap is one line in the preamble plus a bundled
   OFL file and a licence notice — but it is **not this task's**, it belongs to no task in the
   skeleton that this agent can find, and it changes the metrics of every row. **Raised as a
   skeleton gap.**
4. **`--row-h*` is named by the scope and exists in no file today.** The font-size test rests on
   it. If tasks 29-30 named it differently, the test follows the real token; the assertion is
   *heights are in rem and scale together*, not the spelling.
5. **`--grain-opacity` is a new token** task 35's scope does not carry. It is emitted per theme
   here so that no component learns which theme is active (decision d), and named in task 35's
   file as touched.

**Parity rows served:** **L-234** jointly with task 95 (the four themes, generated), and **L-235**
(personal preferences; PARTIAL, as priced — auto-assign **rules** are L-40, task 176). Task 97
proves them, and the owner's eyes accept the high-contrast pair.
