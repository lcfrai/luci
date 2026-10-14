### Task 95: themes, part 1 — `tokens.gen.ts` from three LCH seeds; `graphite` and `paper` generated

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement; the largest of the W2
design tasks and the first to split if it runs over — the `paper` derivation can go to its own
task). **Stream:** WD1. **dependsOn:** 33 (`stress.sets`, which renders both themes), after a
sync-merge of `feat/work-layer` (rule H1). **Seams:** extends **SW.11** by the theme blocks only;
no interface, route or type changes.

`tokens.css` today is **hand-set**: 191 lines of deliberate OKLCH values with the reasoning beside
each one. That is why it looks considered, and it is also why there is exactly one theme. A light
theme made by inverting lightness would fail its own floors — invert `--ink-3` at 50.9 % on a
white ground and it lands at 3:1 against nothing you can name. So `paper` is **re-derived from the
seeds**, not flipped (02 D2 row 5, `02-linear-design-language.md:373`: *"a light theme that is
re-derived, not inverted"*).

This task turns `tokens.css` from a hand-written file into an **emitted** one, and the test that
matters is that nothing about `graphite` changes when it does. The generator runs at **build
time**, not runtime, because CSS relative-colour syntax would need a webview floor nobody has
pinned (02 C1, **UNVERIFIED**, and not relied on). The emitted file is committed, so `npm run
dev` needs no generator step and a reviewer reads real numbers in a diff.

#### Anchors, re-read on 2026-09-27

In this tree, verified by reading the file: `desktop/src/styles/tokens.css` is 191 lines; the
surface ramp is `--page` `oklch(14.0% 0.0040 67)` through `--hover-2` `oklch(26.0% 0.0070 67)`
with `--field` `oklch(12.5% 0.0035 67)` deliberately **below** `--page` (`:31-41`, and the comment
saying why); the ink ramp is `--ink` `94.8% 0.0097 73`, `--ink-2` `70.7% 0.0148 68`, `--ink-3`
`50.9% 0.0149 64`, `--ink-4` `37.2% 0.0130 67` with the floor stated as *"ink-3 is the FLOOR for
anything a user must read (3.43:1 on --page)"* and *"ink-4 is 1.92:1 and is never text"*
(`:41-47`); hairlines are alpha over the ink hue (`:49-56`); the accent block is `:63-68`
(`--accent`, `-hi`, `-lo`, `-ink`, `-tint`, `-ring`); status is `--run` `69.8% 0.1671 148`,
`--need` `75.8% 0.1370 75.8`, `--fail` `66.7% 0.1830 32.3`, `--drift` `72.7% 0.1378 316.7`
(`:74-77`); `--idle` and `--exited` resolve to `var(--ink-3)` (`:106-107`); radius `:110-113`;
elevation `:117-122`; `--scrim` `:130`; motion and `--beat` `:134-142`; `--mono` `:152`.

`desktop/src/styles/tokens.test.ts` is 91 lines and already asserts: every Phase 2 custom property
is declared (`:58-62`), **no literal hex in any declaration** (`:63-66`), the documented hexes
appear in comments only (`:67-70`), no `transition: all` (`:71-74`), the reduced-motion floor, the
focus ring and `color-scheme: dark` (`:75-81`), and that `app.css` imports the token layer first
and owns no colour (`:82-89`). `desktop/src/styles/designsystem.test.ts` sits beside it.
`desktop/package.json` scripts are `dev`, `build`, `preview`, `tauri`, `test`, `test:watch`,
`typecheck` (`:6-14`) — **there is no `tokens` script and no `desktop/scripts/` directory**.

Design: `02-linear-design-language.md` C1 (`:184-197`) and D2 row 5 (`:373`).

#### Decisions the scope left open, made here

**(a) The generator emits only what is derivable. Everything else it copies.** Radius, elevation
geometry, motion, `--beat`, the type stacks and `--scrim` are not colours of a theme and do not
vary per ground; they are emitted once, outside the theme blocks, from a **preamble file kept
verbatim**. So `tokens.gen.ts` owns roughly 40 of the file's declarations and passes the rest
through unchanged, comments included. A reviewer diffing the first emission sees only the theme
blocks move.

**(b) The comments survive, and that is a hard requirement.** `tokens.css`'s value is half its
reasoning. The generator emits the same section banners and the same per-token comments, with the
computed hex appended (`/* #0a0908 — the window ground */`), because `tokens.test.ts:67-70`
already asserts the documented hexes appear in comments and nowhere else. A generator that emits
bare numbers would pass `tsc` and gut the file.

**(c) The ramp is expressed as offsets from the base seed, in the units the file already uses.**
`--seed-base` is `oklch(16% 0.0045 67)`, which is exactly today's `--canvas`. Every surface is
`base + Δ` in lightness with chroma rising alongside, so the warmth the comment describes is a
property of the formula and not of nine hand-typed numbers:

| Token | ΔL from base | Chroma | Note |
|---|---|---|---|
| `--field` | −3.5 | 0.0035 | a well, below `--page` (the file's own exception) |
| `--page` | −2.0 | 0.0040 | the window ground |
| `--canvas` | 0.0 | 0.0045 | **the seed itself** |
| `--surface` | +2.5 | 0.0050 | cards, popovers, menus |
| `--inset` | +5.0 | 0.0060 | a selected row, a chip |
| `--hover` | +7.5 | 0.0065 | hover on a row |
| `--hover-2` | +10.0 | 0.0070 | hover on something already at `--inset` |

Each Δ is multiplied by `--seed-contrast` and each result is clamped to `[0, 100]`. Hue is the
seed's, unchanged, for every surface.

**(d) `paper` flips the *direction* of the ramp, not the numbers.** A light theme's surfaces rise
*towards* the ground the same way a dark theme's do, so the same Δ table is read with the sign
inverted and a different base seed: `--seed-base: oklch(97.2% 0.0035 73)`. `--canvas` is the seed,
`--surface` is **darker** by 2.5, `--hover-2` darker by 10. Ink runs the other way: on `paper`,
`--ink` is near-black and `--ink-3` must clear 3:1 **downward**. This is the whole of *re-derived,
not inverted*: one table, two directions, two seeds — never `100 - L`.

**(e) Ink lightness is solved, not tabled.** `--ink`, `--ink-2` and `--ink-3` are found by
bisecting lightness at fixed chroma and hue until the WCAG contrast ratio against `--page` hits
the file's stated targets — 17.09:1, 7.63:1 and 3.43:1 — to within 0.01. That is why `graphite`'s
emitted values match today's hand-set ones: those numbers were chosen for those ratios.
`--ink-4` is **not** a text colour (`tokens.css:43`), so it is placed by lightness offset from
`--ink-3` and carries no ratio target; its emitted comment keeps saying `non-text only`.

**(f) Status colours are not seeded, and their hue is frozen.** `--run`, `--need`, `--fail` and
`--drift` keep the hue and chroma of `tokens.css:74-77` in every theme. Only lightness is
re-solved, to hold each token's stated ratio against that theme's `--page`. Status must mean the
same thing on every screen (02 C1), and a test asserts hue equality across themes to three
decimals.

**(g) No dependency is added.** OKLCH → sRGB and the WCAG ratio are about 70 lines of arithmetic
(the Oklab matrices and the sRGB transfer function), written in `oklch.ts` with its own test
against six published reference values. Pulling in `culori` or `colorjs.io` for 70 lines of maths
would add a licence question and a supply-chain surface to a **build-time** script. The six
reference conversions are the guard that the arithmetic is right.

#### Files

| File | New / ⚑ | What it gains |
|---|---|---|
| `desktop/scripts/tokens.gen.ts` | new | the generator: seeds → `tokens.css` |
| `desktop/scripts/lib/oklch.ts` | new | OKLCH↔sRGB, WCAG ratio, the bisector |
| `desktop/scripts/lib/oklch.test.ts` | new | the six reference conversions |
| `desktop/scripts/lib/themes.ts` | new | the Δ table, the seeds, the ratio targets |
| `desktop/scripts/tokens.preamble.css` | new | decision (a)'s verbatim non-colour block |
| `desktop/scripts/tokens.gen.test.ts` | new | the generator's own suite |
| `desktop/src/styles/tokens.css` | ⚑ | becomes the **emitted output**, two theme blocks |
| `desktop/src/styles/tokens.test.ts` | ⚑ | gains the floors-per-theme and re-run assertions |
| `desktop/src/styles/designsystem.test.ts` | ⚑ | its audits run over both theme blocks |
| `desktop/package.json` | ⚑ | `"tokens": "tsx scripts/tokens.gen.ts"` |
| `desktop/src/dev/stress.sets.tsx` | ⚑ (task 33's) | a theme switch on every set |

**The file map names no path for `tokens.gen.ts`.** It is placed at `desktop/scripts/tokens.gen.ts`
— a directory that does not exist today — beside its `lib/`, so the generator is not shipped in
the bundle and `tsc --noEmit` over `src` is unaffected. `tsx` is the one devDependency added, and
it is MIT (**UNVERIFIED** here: the registry fetch is blocked from this session; step 1 records the
string, and the fallback is `node --experimental-strip-types`, which needs no dependency at all).

#### `desktop/scripts/lib/themes.ts`, complete

```ts
// desktop/scripts/lib/themes.ts
export interface Seeds {
  /** The ground. Every surface, ink and hairline derives from it. */
  readonly base: Oklch;
  /** The teal. -hi, -lo, -ink, -tint and -ring derive from it. */
  readonly accent: Oklch;
  /** Multiplies every lightness step. 1.0 default, 1.5 high contrast. Range 0.8-1.5. */
  readonly contrast: number;
  /** "dark": surfaces rise from the ground. "light": they fall (decision d). */
  readonly direction: "dark" | "light";
}

export interface Oklch { readonly l: number; readonly c: number; readonly h: number; readonly a?: number }

export interface ThemeSpec {
  readonly name: "graphite" | "paper" | "graphite-hc" | "paper-hc";
  readonly selector: string;            // ":root" | ':root[data-theme="paper"]'
  readonly colorScheme: "dark" | "light";
  readonly seeds: Seeds;
}

/** Decision (c)'s table. ΔL is scaled by contrast and signed by direction. */
export const SURFACE_STEPS: readonly { name: string; dl: number; c: number; note: string }[] = [
  { name: "--field",   dl: -3.5, c: 0.0035, note: "a form field is a WELL, not a raised plane" },
  { name: "--page",    dl: -2.0, c: 0.0040, note: "the window ground" },
  { name: "--canvas",  dl:  0.0, c: 0.0045, note: "rail, sub-rail, the Canvas backdrop" },
  { name: "--surface", dl:  2.5, c: 0.0050, note: "cards, popovers, menus" },
  { name: "--inset",   dl:  5.0, c: 0.0060, note: "a selected row, a chip" },
  { name: "--hover",   dl:  7.5, c: 0.0065, note: "hover on a row" },
  { name: "--hover-2", dl: 10.0, c: 0.0070, note: "hover on something already at --inset" },
];
```

```ts
/** Decision (e). Each ink is SOLVED to its ratio against that theme's --page. */
export const INK_TARGETS: readonly { name: string; ratio: number; c: number; h: number; note: string }[] = [
  { name: "--ink",   ratio: 17.09, c: 0.0097, h: 73, note: "primary text" },
  { name: "--ink-2", ratio:  7.63, c: 0.0148, h: 68, note: "secondary text" },
  { name: "--ink-3", ratio:  3.43, c: 0.0149, h: 64, note: "the FLOOR for anything a user must read" },
];
/** Not a text colour (tokens.css:43): placed by offset, no ratio target. */
export const INK_4_DL = -13.7;   // from --ink-3, scaled by contrast

/** Decision (f). Hue and chroma frozen; lightness re-solved per ground. */
export const STATUS_TARGETS: readonly { name: string; ratio: number; c: number; h: number }[] = [
  { name: "--run",   ratio: 7.90, c: 0.1671, h: 148 },
  { name: "--need",  ratio: 9.04, c: 0.1370, h: 75.8 },
  { name: "--fail",  ratio: 6.06, c: 0.1830, h: 32.3 },
  { name: "--drift", ratio: 5.50, c: 0.1378, h: 316.7 },
];

export const THEMES: readonly ThemeSpec[] = [
  { name: "graphite", selector: ":root", colorScheme: "dark",
    seeds: { base: { l: 16, c: 0.0045, h: 67 }, accent: { l: 70.3, c: 0.1092, h: 202.7 },
             contrast: 1.0, direction: "dark" } },
  { name: "paper", selector: ':root[data-theme="paper"]', colorScheme: "light",
    seeds: { base: { l: 97.2, c: 0.0035, h: 73 }, accent: { l: 48.0, c: 0.1092, h: 202.7 },
             contrast: 1.0, direction: "light" } },
  // graphite-hc and paper-hc are task 96's: the same two seeds at contrast 1.5.
];
```

**`--drift` gains a stated ratio it does not have today.** `tokens.css:77` is the one status
token whose comment carries no measured number. The generator cannot solve for a target that is
not written down, so 5.50:1 is set here from the value the hand-set token already reaches, and the
emitted comment records it. That is a **correction**, listed below.

#### `desktop/scripts/lib/oklch.ts`, complete

```ts
// desktop/scripts/lib/oklch.ts — build time only. Never imported from src/.
export interface Oklch { l: number; c: number; h: number; a?: number }
export interface Srgb { r: number; g: number; b: number }   // 0..1, NOT gamma-encoded away

/** OKLCH (l in 0..100) → linear sRGB → sRGB. Out-of-gamut components are reported, not clipped. */
export function oklchToSrgb(v: Oklch): { rgb: Srgb; inGamut: boolean };

/** "#0a0908". Throws when out of gamut, so a bad seed fails the build rather than the eye. */
export function toHex(v: Oklch): string;

/** WCAG 2.1 relative luminance and contrast ratio. */
export function luminance(rgb: Srgb): number;
export function contrastRatio(a: Oklch, b: Oklch): number;

/**
 * Decision (e)'s solver. Bisects lightness in `range` at fixed c and h until the ratio against
 * `ground` is within `tol` of `target`. Throws after 60 halvings or if the bracket does not
 * contain a solution — a seed that cannot reach its floor must fail loudly.
 */
export function solveLightness(args: {
  ground: Oklch; c: number; h: number; target: number; tol?: number;
  range?: readonly [number, number];
}): number;

/** "oklch(14.0% 0.0040 67)" — three-decimal lightness, four-decimal chroma, one-decimal hue. */
export function fmt(v: Oklch): string;
```

The six reference conversions `oklch.test.ts` pins, each from the CSS Color 4 specification's own
examples and each checked to ±0.001 in sRGB: white, black, the sRGB primaries red and blue, a
mid grey, and `oklch(70.3% 0.1092 202.7)` → `#2fb3bd`, which is the accent this file already
documents at `tokens.css:63`. If the arithmetic is wrong, that last one moves.

#### `desktop/scripts/tokens.gen.ts`, complete

```ts
// desktop/scripts/tokens.gen.ts
//   npm --prefix desktop run tokens          → writes src/styles/tokens.css
//   npm --prefix desktop run tokens -- --check → exits 1 if the file on disk differs
import { THEMES, SURFACE_STEPS, INK_TARGETS, INK_4_DL, STATUS_TARGETS } from "./lib/themes";

export interface Emission { readonly css: string; readonly perTheme: Record<string, Record<string, string>> }

/** Pure. Given the specs and the verbatim preamble, returns the whole file. */
export function emit(themes: typeof THEMES, preamble: string): Emission;

/** Derives one theme's colour block. Exported so the test asserts values, not text. */
export function deriveTheme(spec: (typeof THEMES)[number]): Record<string, string>;

export const HEADER = `/* desktop/src/styles/tokens.css
 *
 * GENERATED by desktop/scripts/tokens.gen.ts. Do not hand-edit: \`npm run tokens -- --check\`
 * fails in CI if you do. Change a SEED in scripts/lib/themes.ts and re-run.
 *
 * Three seeds per theme — base, accent, contrast — and every surface, ink, hairline and accent
 * step derives from them in OKLCH. Status colours are NOT seeded: --run, --need, --fail and
 * --drift keep their hue and chroma in every theme and only re-solve lightness, so a colour
 * means the same thing on every screen.
 */`;
```

`--check` is what makes decision (b) enforceable and is the acceptance command CI runs. The
emitted file is committed; `npm run build` does **not** invoke the generator, so a broken script
can never break a build, and a stale file can never ship, because the test re-runs `emit()` in
process and compares.

#### The two ⚑ test files

`tokens.test.ts` ⚑ keeps all seven of its existing assertions — they now run over a longer file,
and the no-literal-hex rule (`:63-66`) is the reason the generator must emit hexes **in comments
only**. It gains four:

```ts
it("is byte-identical to a fresh run of the generator", () => {
  expect(emit(THEMES, readFileSync(PREAMBLE, "utf8")).css).toBe(tokensCss);
});

it("clears the ink floors in every emitted theme", () => {
  for (const t of THEMES) {
    const d = deriveTheme(t);
    expect(ratio(d["--ink-3"], d["--page"])).toBeGreaterThanOrEqual(3);
    expect(ratio(d["--ink-3"], d["--surface"])).toBeGreaterThanOrEqual(3);
    expect(ratio(d["--ink-3"], d["--inset"])).toBeGreaterThanOrEqual(3);
    expect(ratio(d["--ink-2"], d["--page"])).toBeGreaterThanOrEqual(7);
  }
});

it("does not move graphite", () => { /* every token within 0.5% lightness of the frozen table */ });

it("keeps status hue identical across themes", () => { /* to three decimals */ });
```

The frozen table in the third assertion is the 20 hand-set values read out of `tokens.css` at
`HEAD` before this task changes it, written into the test as data. That is the assertion that
makes this task safe: **the default look does not shift.**

`designsystem.test.ts` ⚑ runs its existing audits — the banned indigo/violet arc, `transition:
all`, the elevation shape — over **each** theme block rather than the file once, so a `paper`
value cannot slip into the banned arc while `graphite` passes.

#### Tests to write first, each with the mutant it kills

`oklch.test.ts` and `tokens.gen.test.ts` under `desktop/scripts/` (added to the vitest include),
plus the four additions to `tokens.test.ts`. Each is seen failing first.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `six reference conversions` | white, black, red, blue, mid grey and `oklch(70.3% 0.1092 202.7)` → `#2fb3bd`, each to ±0.001 | a transposed Oklab matrix; a missing sRGB transfer function |
| `out of gamut fails loudly` | `oklch(70% 0.4 202)` → `toHex` throws; `inGamut` false | a silent channel clip that ships an unreachable colour |
| `every emitted ink clears its floor in both themes` | `--ink-3` ≥ 3:1 on `--page`, `--surface` and `--inset`; `--ink-2` ≥ 7:1 on `--page`; in `graphite` **and** `paper` | **`paper` made by inverting lightness**, which fails `--ink-3` on `--surface` |
| `paper is derived, not inverted` | no code path computes `100 - l`; `paper`'s `--surface` is **darker** than its `--canvas` while `graphite`'s is lighter; `paper`'s ink hue is the seed's, not `graphite`'s | `100 - L` inversion; a hue flip |
| `the emitted file is byte-identical to a fresh run` | `emit(...)` equals the file on disk, and `--check` exits 1 after a one-character hand edit | a hand edit to `tokens.css` that the eye would never catch |
| `graphite does not move` | all 20 frozen surface/ink/accent values within 0.5 % lightness, 0.0005 chroma and 0.5° hue | a seed typo that silently re-skins the app |
| `no literal hex outside comments` | the existing `tokens.test.ts:63-66` rule still passes on the emitted file | a generator emitting hex as the value |
| `status hue is identical across themes` | `--run`, `--need`, `--fail`, `--drift` hue equal to three decimals in every theme; lightness differs | status derived from the accent seed |
| `status still clears its ratio on paper` | each status token ≥ its stated ratio against `paper`'s `--page` | a dark-theme lightness carried onto a light ground |
| `the comments survive` | every emitted token line carries its `/* #hex — note */`; the section banners are present; the preamble block is byte-identical to `tokens.preamble.css` | a generator that emits bare numbers and guts the file's reasoning |
| `--mono stays a real monospace` | the emitted `--mono` still ends `monospace` and still begins `"Cascadia Mono"` (`tokens.css:152`) | the preamble rewritten or dropped, which would break xterm.js |
| `the generator is not in the bundle` | `grep` finds no `scripts/` import under `desktop/src`; `tsc --noEmit` over `src` passes with `scripts/` excluded | the build-time solver shipped to the webview |
| `stress.sets renders both themes` | task 33's three screens render under `graphite` and `paper` with no thrown error and no `undefined` custom property | a token declared in one theme only |

#### Steps

- [ ] **Step 1: Freeze `graphite`.** Read the 20 values out of `tokens.css` at `HEAD` into the
  test's frozen table, and record `tsx`'s licence string (or take the
  `node --experimental-strip-types` fallback and add no dependency at all).
- [ ] **Step 2: `oklch.ts` and its six references.** Pure arithmetic, green before anything uses
  it. This is the step that must not be hurried: every later assertion rests on it.
- [ ] **Step 3: `themes.ts` and `deriveTheme` for `graphite` only**, then the *graphite does not
  move* test. If a token cannot be reached from the seeds within tolerance, the Δ table is wrong
  and it is fixed here, not by special-casing the token.
- [ ] **Step 4: `paper`**, then the floors-per-theme and *derived, not inverted* tests.
- [ ] **Step 5: `emit`, the preamble split and `--check`**, then re-emit `tokens.css` and read the
  diff: it must show the theme blocks and nothing else.
- [ ] **Step 6: Green and commit by pathspec**, the eleven files of *Files*, naming the four ⚑ as
  touched, with the diff's shape stated in the commit body.

#### Acceptance

```bash
npm --prefix desktop run tokens
git -C . diff --stat -- desktop/src/styles/tokens.css
npm --prefix desktop run tokens -- --check ; echo "check=$?"
npm --prefix desktop test -- --run scripts/ 2>&1 | tail -5
npm --prefix desktop test -- --run src/styles 2>&1 | tail -5
npm --prefix desktop test 2>&1 | tail -5
npm --prefix desktop run typecheck
grep -c 'GENERATED by desktop/scripts/tokens.gen.ts' desktop/src/styles/tokens.css   # 1
grep -rn 'scripts/lib/oklch' desktop/src | wc -l                                     # 0
grep -rn '100 - ' desktop/scripts/lib/themes.ts | wc -l                              # 0
```

Expected: the generator writes the file; `check=0`; both suites pass and the whole suite passes;
`typecheck` silent, exit 0; the greps print `1`, `0`, `0`. **None has been run; `desktop/scripts/`
does not exist** (OD-27).

#### What this task does not claim

- **Not the owner's visual acceptance of `paper`.** Contrast ratios are arithmetic; whether the
  light theme *looks* like this product is the owner's eyes in task 97. A passing floor is not a
  passing design.
- **Not runtime theming.** The generator runs at build time (02 C1). Switching themes at runtime
  is a `data-theme` attribute over emitted blocks — cheap — but CSS relative-colour syntax is
  **not** used and its webview floor stays **UNVERIFIED**.
- **Not the high-contrast pair.** `graphite-hc` and `paper-hc` are task 96, at contrast 1.5.
- **Not the preferences UI.** Choosing a theme is task 96.
- **Not a theme gallery or a custom-theme picker** (02 C1: four themes, no free-form picker).
- **Not WCAG conformance as a whole.** These are per-token ratio floors against a named ground,
  computed in sRGB. Text over an image, focus-ring contrast at 150 % scaling and forced-colors
  behaviour are tasks 34 and 96.
- **No measurement in WebView2.** Every number here comes from the arithmetic, not from a screen.
- **Not `tsx`'s licence.** **UNVERIFIED**; step 1 reads it, and the fallback adds no dependency.

#### Corrections to the scope

1. **`--drift` has no stated ratio in `tokens.css`.** `:77` is the one status token whose comment
   carries no measured number, so a generator has no target to solve for. 5.50:1 is set here from
   what the hand-set value reaches; **if the owner wants a different floor, this is the line to
   change.**
2. **The scope's `--seed-base` is `--canvas`, not `--page`.** `oklch(16% 0.0045 67)` is exactly
   today's `--canvas` (`tokens.css:32`), while `--page` is 14.0 %. Read as *"the ground"* that is
   ambiguous; the Δ table above resolves it by making `--canvas` the zero point and `--page` a
   −2.0 step. No emitted value changes either way, but the table would be wrong by 2 % if read the
   other way.
3. **Four files, not one.** The scope names `tokens.gen.ts` alone. The arithmetic (`oklch.ts`),
   the data (`themes.ts`) and the verbatim non-colour block (`tokens.preamble.css`) are separate
   because the first needs its own reference test and the last must be byte-preserved.
4. **`--field` breaks the ramp on purpose**, and a generator would flatten it. `tokens.css:36-41`
   says a form field is a well below `--page`; the Δ table keeps it at −3.5 and keeps the comment,
   because that exception is a decision, not an accident.
5. **Task 33's `stress.sets` needs a theme switch** to render both themes, and its scope
   (`tasks-21-40.md:454`) does not mention one. One control, added here in task 33's file, named
   as touched.

**Parity rows served:** **L-234**'s generated half — `graphite` and `paper`, derived from seeds —
jointly with task 96. Task 97 proves it, and the owner's eyes accept it.
