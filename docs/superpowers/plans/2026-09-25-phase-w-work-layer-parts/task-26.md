### Task 26: tokens: 02 D2 step 1, `--t-fact`, the provisional `paper` block

Stream **WD1**, the token half of seam **SW.11** (defined here; task 27 adds the faces), about 2
hours (an estimate, not a measurement). No dependency: the look needs no Hub, no Work table and no
route, so this task can land on the first day of W1.

This is the task every Phase W surface reads its sizes from. Nothing renders differently when it
lands: it declares names, and tasks 30 and 31 are the first to use them. That is on purpose. A
token that arrives with its first consumer gets its value chosen to suit that one screen; a token
that arrives alone, with a test pinning each value to the design line it came from, gets chosen once.

**What it adds to `:root`**, each with a one-line comment naming its source:

| Group | Tokens | Source |
|---|---|---|
| motion | `--dur-flash` 600ms, `--dur-peek` 120ms, `--undo-window` 6000ms | 02 C7; 05 §4.6 |
| spacing | `--s-1` … `--s-6` = 4 · 8 · 12 · 16 · 24 · 32 px | 02 C4 |
| density | `--row-h` 32px, `--row-h-compact` 28px, `--card-3line` 76px | 02 C4; 05 §4.4 C4 |
| type | `--t-micro` 11/14 mono (`kbd` hints only), **`--t-fact` 12/16 mono**, `--t-meta` 12/16, `--t-ui` 13/20, `--t-body` 14/22, `--t-head` 16/22 at 560, `--t-display` 24/28 mono | 02 C3; 05 §4.4 C3 |
| icons | `--icon` 16px / 1.5px, `--icon-sm` 14px / 1.25px, `--icon-ink` | 02 C6 |

Each type step is two tokens, a size and a line height (`--t-fact` and `--t-fact-lh`), because a
`font` shorthand needs both and a component that invents its own line height is how the rows of
one list stop lining up. `--t-head` carries two more, `--t-head-weight` and `--t-head-track`.

**`--t-fact` is the one new idea.** 02 C3 put every machine fact at `--t-micro`, 11px. 05 §4.4 C3
(revision 2) moved them to a step of their own, 12/16 mono, because an id, a path, a duration or
a cost is something a person reads to decide, and 11px mono in `--ink-3` is below what a tired
reader at 125 % scaling reads without leaning in (07 D I7). `--t-micro` stays, and is legal only on
a `kbd` hint. Task 30's audit rule A holds every other mono rule to `--t-fact`.

**The provisional `paper` block.** One light theme beside graphite, so W1 can show that nothing
in Phase W assumes a dark ground (the W1 part of **L-234**). It is re-derived by hand in OKLCH,
not an inversion of graphite: the warm hue stays (75° on the surfaces, 67° on the inks), the
surfaces sit between 89.5 % and 99.5 % lightness, and the four status colours are pulled down to
52-55 % lightness so each clears 3:1 on the light page and still means the same thing (02 C1:
status colour is never seeded and never changes meaning). The block is **provisional**: W2's
generator (tasks 95, 96) replaces it with an emitted block. **The names it declares are the
contract the generator must keep**, and a test here pins that set.

**Three corrections to the scope, made here so nobody re-derives them:**

1. **Six spacing steps, not eight.** 02 C4 names `--s-1 … --s-8` and lists six values. Six are
   declared. If an eighth is ever needed it is a design change, not a gap to fill with a guess.
2. **The documented-hex test is amended, not only appended to.** `tokens.test.ts:68-70` asserts
   the hex literals in the file's comments are *exactly* the nineteen graphite ones. Paper's
   nineteen colours document their hex the same way, so that test's expected set gains
   `PAPER_HEXES`. Without the change the test fails on the first paper comment.
3. **The selector is `:root[data-theme="paper"]`, not `[data-theme="paper"]`.** A bare attribute
   selector has the same specificity as `:root` (0,1,0) and wins only by source order; one
   stylesheet reordered later would silently put graphite back. `:root[...]` is (0,2,0) and wins
   wherever it sits. Nothing sets `data-theme` yet (a theme picker is C11 `first.keys`, later);
   task 33 sets it by hand to render the paper sheet.

Every paper value below was computed while this plan was written (OKLCH → linear sRGB → WCAG
relative luminance, the same arithmetic as the test helper), and every one is inside sRGB. The
contrast figures in its comments are those computations, to two places; the test re-derives them.
Graphite's own status values, copied onto paper's page, would measure run 2.31, need 2.02, fail
3.01 and drift 2.35 (computed the same way): fail alone scrapes past 3:1, which is why the test
below checks three grounds and also forbids an unchanged copy.

**Files:**
- `desktop/src/styles/tokens.css` ⚑ (191 lines, read at `f7ea2a2`: `:root` opens at `:19`,
  `--ink-3` 3.43:1 at `:47`, `--thread` at `:57`, motion at `:135-142`, faces at `:151-152`,
  `--numeric` at `:156` and `:root` closes at `:157`; the reduced-motion floor at `:174-183`, the
  focus ring at `:187-191`) — the `:root` additions after `:156`; the paper block after `:157`
  (modified)
- `desktop/src/styles/tokens.test.ts` ⚑ (91 lines; `describe("the token layer")` at `:41`, the
  hex test at `:68-70`) — `PAPER_HEXES`, the amended hex test, one new `describe` (modified)

No other file changes. `app.css` is untouched (task 27 edits it).

#### Interfaces — complete: the `:root` additions

Inserted after `--numeric: tabular-nums;` (`tokens.css:156`) and before `:root`'s closing brace.

```css
  /* ── Phase W, 02 D2 step 1 ────────────────────────────────────────────────────────
   * Declared before anything uses them (tasks 30 and 31 are first). Each line names the design
   * line it came from; tokens.test.ts pins every value, so changing one is a reviewed edit. */

  /* Motion, three more (02 C7). */
  --dur-flash: 600ms;       /* 02 C7; DESIGN-SYSTEM's ~600ms state-change flash and decay */
  --dur-peek: 120ms;        /* 02 C7: a peek enters on opacity + translateY(4px), leaves at --dur-fast */
  --undo-window: 6000ms;    /* 02 C7; 05 §4.6: the optimistic-undo countdown, paused while hovered */

  /* Spacing (02 C4): the only padding, margin and gap values in Phase W component CSS.
   * 02 names --s-1 … --s-8 and lists six values; six are declared. */
  --s-1: 4px;
  --s-2: 8px;
  --s-3: 12px;
  --s-4: 16px;
  --s-5: 24px;
  --s-6: 32px;

  /* Density (02 C4; 05 §4.4 C4). Changes row height and vertical padding only, never type. */
  --row-h: 32px;            /* a list row, a rail row, a RUNNING row */
  --row-h-compact: 28px;    /* the same rows in density: compact */
  --card-3line: 76px;       /* 3 × 20 + 2 × 8: a Desk card at rest, STATE / MEANING / NEXT, never taller */

  /* Type (02 C3; 05 §4.4 C3, revision 2). A size and a line height per step. */
  --t-micro: 11px;          /* mono: kbd hints ONLY. Not a fact size (05 §4.4 C3) */
  --t-micro-lh: 14px;
  --t-fact: 12px;           /* mono: every id, key, path, time, cost and duration (05 §4.4 C3) */
  --t-fact-lh: 16px;
  --t-meta: 12px;           /* sans: secondary labels, timestamps in words */
  --t-meta-lh: 16px;
  --t-ui: 13px;             /* sans: rows, controls, the rail */
  --t-ui-lh: 20px;
  --t-body: 14px;           /* sans: MEANING lines, briefs, Orchestrator prose */
  --t-body-lh: 22px;
  --t-head: 16px;           /* sans: view titles and section heads */
  --t-head-lh: 22px;
  --t-head-weight: 560;
  --t-head-track: -0.01em;
  --t-display: 24px;        /* mono: the zero-state line only, once */
  --t-display-lh: 28px;

  /* Icons (02 C6). A 14px glyph is drawn at 1.25px, never the 16px drawing scaled (task 28). */
  --icon: 16px;
  --icon-stroke: 1.5px;
  --icon-sm: 14px;          /* inside rows and chips */
  --icon-sm-stroke: 1.25px;
  --icon-ink: var(--ink-3); /* at rest; --ink-2 on hover and --ink when current, set by the component */
```

The prototype (`prototype/work-layer.html:40`) sets `--t-body:13px`. This
task follows 02 C3, where 13px is `--t-ui` and `--t-body` is 14px; a MEANING line is prose, and
prose gets the larger step. The prototype's `--t-kbd` is this plan's `--t-micro`.

#### Interfaces — complete: the provisional `paper` block

Inserted after `:root`'s closing brace (`tokens.css:157`) and before the grain rule. The inks are
declared on `:root` in both themes, so `--idle`, `--exited` and `--icon-ink` (each a `var()` of
`--ink-3`) follow the theme without being redeclared: a custom property's `var()` resolves on the
element that declares it, and both blocks declare on the root element.

```css
/* ── paper: a PROVISIONAL light theme (Phase W task 26) ─────────────────────────────
 * Re-derived by hand in OKLCH, not an inversion of graphite. W2's generator (tasks 95, 96)
 * replaces this block with an emitted one; the NAMES below are the contract it must keep, and
 * tokens.test.ts pins them. Contrast figures are WCAG ratios on this block's --page, to two places.
 * Selector is :root[...] (0,2,0) so it beats :root (0,1,0) wherever the block sits. */
:root[data-theme="paper"] {
  color-scheme: light;

  /* Surfaces. The naming order holds; the lightness order does not: a card (--surface) and a
   * field (--field) sit ABOVE the page on a light ground, the way --field sits below it in
   * graphite. Hue 75, a warm paper; chroma falls as lightness rises so white stays white. */
  --page:    oklch(97.0% 0.0045 75);   /* #f7f5f2 */
  --canvas:  oklch(95.0% 0.0055 75);   /* #f1eeea */
  --surface: oklch(99.0% 0.0025 75);   /* #fdfbfa */
  --inset:   oklch(93.0% 0.0065 75);   /* #eae7e3 */
  --hover:   oklch(91.5% 0.0070 75);   /* #e6e2de */
  --hover-2: oklch(89.5% 0.0075 75);   /* #dfdcd7 */
  --field:   oklch(99.5% 0.0020 75);   /* #fefdfc */

  /* Ink. Floors as graphite's: ink >= 15:1, ink-2 >= 7:1 (the floor for any fact), ink-3 >= 3:1
   * on page, canvas and surface. ink-4 is never text. */
  --ink:   oklch(21.0% 0.0120 67);     /* #1c1713 — 16.26:1 */
  --ink-2: oklch(40.0% 0.0140 67);     /* #4d4640 —  8.46:1 */
  --ink-3: oklch(55.5% 0.0140 67);     /* #79726b —  4.37:1 */
  --ink-4: oklch(78.0% 0.0100 67);     /* #bcb6b1 —  1.84:1, non-text only */

  /* Hairlines: alpha over the dark ink, the mirror of graphite's alpha over the light ink. */
  --line-soft:   oklch(21.0% 0.0120 67 / 0.05);
  --line:        oklch(21.0% 0.0120 67 / 0.09);
  --line-strong: oklch(21.0% 0.0120 67 / 0.16);
  --thread:      oklch(21.0% 0.0120 67 / 0.20);

  /* Accent: the same teal hue (202.7), darkened and de-chromaed to stay inside sRGB. On a
   * light ground the "hi" step is DARKER: it is the one that reads strongest as text. */
  --accent:      oklch(47.0% 0.0720 202.7);          /* #19676c — 6.05:1 */
  --accent-hi:   oklch(40.0% 0.0640 202.7);          /* #0c5257 — 8.20:1 */
  --accent-lo:   oklch(35.0% 0.0580 202.7);          /* #034348 — pressed */
  --accent-ink:  oklch(98.5% 0.0100 202.7);          /* #f3fcfd — text ON an accent fill, 6.34:1 */
  --accent-tint: oklch(47.0% 0.0720 202.7 / 0.10);
  --accent-ring: oklch(47.0% 0.0720 202.7 / 0.30);

  /* Status: the same four hues and meanings (02 C1), lightness re-derived for a light ground so
   * each clears 3:1 on --page. Graphite's values copied here measure 2.02 to 3.01. */
  --run:   oklch(52.0% 0.1400 148);    /* #187e36 — 4.74:1 */
  --need:  oklch(55.0% 0.1150 70);     /* #9c640d — 4.56:1 */
  --fail:  oklch(54.0% 0.1700 32.3);   /* #bd3b25 — 5.04:1 */
  --drift: oklch(53.0% 0.1400 316.7);  /* #8b4ea1 — 5.19:1 */
  --run-tint:  oklch(52.0% 0.1400 148  / 0.10);
  --need-tint: oklch(55.0% 0.1150 70   / 0.10);
  --fail-tint: oklch(54.0% 0.1700 32.3 / 0.10);

  /* Elevation: the same ring-plus-shadow rule, the shadows lighter, because a light ground shows
   * a shadow at a third of the alpha a dark one needs. */
  --e-hair:    0 0 0 1px var(--line);
  --e-btn:     0 0 0 1px var(--line), 0 1px 1px oklch(0% 0 0 / 0.08);
  --e-card:    0 0 0 1px var(--line), 0 1px 2px oklch(0% 0 0 / 0.08), 0 4px 12px oklch(0% 0 0 / 0.06);
  --e-raised:  0 0 0 1px var(--line-strong), 0 2px 4px oklch(0% 0 0 / 0.10), 0 14px 36px oklch(0% 0 0 / 0.12);
  --e-overlay: 0 0 0 1px var(--line-strong), 0 4px 8px oklch(0% 0 0 / 0.12), 0 30px 70px oklch(0% 0 0 / 0.20);
  --scrim: oklch(0% 0 0 / 0.28);
}
```

The need hue moves from 75.8° to 70°: at 55 % lightness a 75.8° amber reads as olive, and
*needs you* must still read as amber. The meaning is the hue family and the shape (task 29), not
the exact angle; W2's generator may choose a different angle and the test below does not pin it.

#### Tests to write first — `desktop/src/styles/tokens.test.ts`

Three edits to the file. **(1)** After `DOCUMENTED_HEXES` (`:34-39`), a second list:

```ts
/** Task 26: the nineteen paper colours, documented the same way as graphite's. */
const PAPER_HEXES = [
  "#f7f5f2", "#f1eeea", "#fdfbfa", "#eae7e3", "#e6e2de", "#dfdcd7", "#fefdfc",
  "#1c1713", "#4d4640", "#79726b", "#bcb6b1",
  "#19676c", "#0c5257", "#034348", "#f3fcfd",
  "#187e36", "#9c640d", "#bd3b25", "#8b4ea1",
];
```

**(2)** The hex test at `:68-70` is amended to name both sets (the title drops "nineteen"):

```ts
  it("documents the source hex of every derived colour, in comments only", () => {
    expect(findHexLiterals(tokensCss).sort()).toEqual([...DOCUMENTED_HEXES, ...PAPER_HEXES].sort());
  });
```

**(3)** Appended after the existing `describe` closes (`:91`): the helpers, then the new block.

```ts
/** The declarations of one rule block, comments stripped. Blocks in tokens.css never nest. */
function block(css: string, selector: string): string {
  const text = stripCssComments(css);
  const at = text.indexOf(`${selector} {`);
  if (at < 0) throw new Error(`no block ${selector}`);
  return text.slice(at, text.indexOf("\n}", at));
}

/** The raw value of `name` in a block, or undefined. */
function valueOf(css: string, name: string): string | undefined {
  const m = new RegExp(`(?:^|[\\s;{])${name}\\s*:\\s*([^;]+);`).exec(css);
  return m ? m[1].trim() : undefined;
}

/** OKLCH → linear sRGB (Björn Ottosson's published Oklab matrices), then WCAG 2 relative
 *  luminance. Written here so the test has no dependency; alpha values are refused. */
function luminance(oklch: string): number {
  const m = /^oklch\(\s*([\d.]+)%\s+([\d.]+)\s+([\d.]+)\s*\)$/.exec(oklch);
  if (!m) throw new Error(`not an opaque oklch(): ${oklch}`);
  const L = Number(m[1]) / 100;
  const h = (Number(m[3]) * Math.PI) / 180;
  const a = Number(m[2]) * Math.cos(h);
  const b = Number(m[2]) * Math.sin(h);
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const mm = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  const clamp = (x: number) => Math.min(1, Math.max(0, x));
  const r = clamp(4.0767416621 * l - 3.3077115913 * mm + 0.2309699292 * s);
  const g = clamp(-1.2684380046 * l + 2.6097574011 * mm - 0.3413193965 * s);
  const bl = clamp(-0.0041960863 * l - 0.7034186147 * mm + 1.707614701 * s);
  return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
}

function contrast(fg: string, bg: string): number {
  const [x, y] = [luminance(fg), luminance(bg)];
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}

const ROOT = block(tokensCss, ":root");
const PAPER = block(tokensCss, ':root[data-theme="paper"]');
const THEMES: ReadonlyArray<[string, string]> = [["graphite", ROOT], ["paper", PAPER]];
const GROUNDS = ["--page", "--canvas", "--surface"] as const;
const STATUS = ["--run", "--need", "--fail", "--drift"] as const;

/** The colour names W2's generator must keep emitting (tasks 95, 96): every colour graphite
 *  declares. Counts, not only names, so an addition to graphite is a reviewed edit here. */
const THEME_COLOUR_NAMES = [
  "--page", "--canvas", "--surface", "--inset", "--hover", "--hover-2", "--field",
  "--ink", "--ink-2", "--ink-3", "--ink-4",
  "--line-soft", "--line", "--line-strong", "--thread",
  "--accent", "--accent-hi", "--accent-lo", "--accent-ink", "--accent-tint", "--accent-ring",
  "--run", "--need", "--fail", "--drift", "--run-tint", "--need-tint", "--fail-tint",
  "--e-hair", "--e-btn", "--e-card", "--e-raised", "--e-overlay", "--scrim",
];

describe("02 D2 step 1 and the paper block", () => {
  // Kills: a missing token (any one of the thirty-three).
  it("declares 02 D2 step 1's tokens", () => {
    const want = [
      "--dur-flash", "--dur-peek", "--undo-window",
      "--s-1", "--s-2", "--s-3", "--s-4", "--s-5", "--s-6",
      "--row-h", "--row-h-compact", "--card-3line",
      "--t-micro", "--t-micro-lh", "--t-fact", "--t-fact-lh", "--t-meta", "--t-meta-lh",
      "--t-ui", "--t-ui-lh", "--t-body", "--t-body-lh", "--t-head", "--t-head-lh",
      "--t-head-weight", "--t-head-track", "--t-display", "--t-display-lh",
      "--icon", "--icon-stroke", "--icon-sm", "--icon-sm-stroke", "--icon-ink",
    ];
    const declared = new Set(declaredCustomProperties(ROOT));
    expect(want.filter((n) => !declared.has(n))).toEqual([]);
    expect(valueOf(ROOT, "--s-1")).toBe("4px");
    expect(["--s-2", "--s-3", "--s-4", "--s-5", "--s-6"].map((n) => valueOf(ROOT, n)))
      .toEqual(["8px", "12px", "16px", "24px", "32px"]);
    expect([valueOf(ROOT, "--row-h"), valueOf(ROOT, "--row-h-compact"), valueOf(ROOT, "--card-3line")])
      .toEqual(["32px", "28px", "76px"]);
  });

  // Kills: facts at 11px (05 §4.4 C3), and --t-micro drifting up to meet them.
  it("sets --t-fact to 12px and --t-micro to 11px", () => {
    expect([valueOf(ROOT, "--t-fact"), valueOf(ROOT, "--t-fact-lh")]).toEqual(["12px", "16px"]);
    expect([valueOf(ROOT, "--t-micro"), valueOf(ROOT, "--t-micro-lh")]).toEqual(["11px", "14px"]);
    expect([valueOf(ROOT, "--t-ui"), valueOf(ROOT, "--t-body")]).toEqual(["13px", "14px"]);
    expect(valueOf(ROOT, "--t-head-weight")).toBe("560");
  });

  // Kills: drift from 05 §4.6's six seconds and DESIGN-SYSTEM's ~600ms flash.
  it("keeps --undo-window at 6000ms and --dur-flash at 600ms", () => {
    expect(valueOf(ROOT, "--undo-window")).toBe("6000ms");
    expect(valueOf(ROOT, "--dur-flash")).toBe("600ms");
    expect(valueOf(ROOT, "--dur-peek")).toBe("120ms");
  });

  // Kills: a paper block whose inks fall below graphite's floors, on any of three grounds.
  it("clears the ink floors in graphite and paper", () => {
    const floors: ReadonlyArray<[string, number]> = [["--ink", 15], ["--ink-2", 7], ["--ink-3", 3]];
    const misses: string[] = [];
    for (const [theme, css] of THEMES) {
      for (const [ink, floor] of floors) {
        for (const ground of GROUNDS) {
          const r = contrast(valueOf(css, ink)!, valueOf(css, ground)!);
          if (r < floor) misses.push(`${theme} ${ink} on ${ground}: ${r.toFixed(2)} < ${floor}`);
        }
      }
    }
    expect(misses).toEqual([]);
  });

  // Kills: graphite's status values copied unchanged (fail alone would scrape 3.01 on --page).
  it("gives paper its own status lightness", () => {
    const misses: string[] = [];
    for (const name of STATUS) {
      const paper = valueOf(PAPER, name)!;
      expect(paper).not.toBe(valueOf(ROOT, name));
      for (const ground of GROUNDS) {
        const r = contrast(paper, valueOf(PAPER, ground)!);
        if (r < 3) misses.push(`${name} on ${ground}: ${r.toFixed(2)}`);
      }
    }
    expect(misses).toEqual([]);
  });

  // Kills: a paper block that drops a name the generator must keep, or an inverted scheme flag.
  it("gives paper every colour name graphite has, and a light colour-scheme", () => {
    const declared = new Set(declaredCustomProperties(PAPER));
    expect(THEME_COLOUR_NAMES.filter((n) => !declared.has(n))).toEqual([]);
    expect(THEME_COLOUR_NAMES).toHaveLength(34);
    expect(PAPER).toContain("color-scheme: light");
  });

  // Kills: a hex literal inside the paper block (the file-wide rule, made explicit for it).
  it("uses no literal hex in the paper block", () => {
    expect(findHexLiterals(PAPER)).toEqual([]);
    expect(PAPER).not.toMatch(/\b(?:rgb|rgba|hsl|hsla)\(/);
  });
});
```

Every `valueOf(...)!` above is on a name the declaration tests prove present; a missing one makes
`luminance` throw on `undefined`, so the test fails loudly rather than passing.

#### Steps

1. Write the three test edits above. Run `npm --prefix desktop test -- tokens`: the new
   `describe` fails (no `--t-fact`, and `block` throws on the missing paper selector), and the
   amended hex test fails (no paper hexes yet). The seven older tests still pass.
2. Insert the `:root` additions after `tokens.css:156`. Re-run: the three declaration and value
   tests pass; the paper tests still fail.
3. Insert the paper block after `:root`'s closing brace. Re-run: all pass.
4. Mutate by hand, one at a time, and watch the named test go red, then revert: `--t-fact: 11px`;
   `--undo-window: 5000ms`; paper `--ink-2: oklch(55.5% 0.0140 67)`; paper `--fail` set to
   graphite's `oklch(66.7% 0.1830 32.3)`; delete paper's `--thread`; paper `--page: #f7f5f2`.
5. Run the typecheck and the full desktop suite (nothing else reads these names yet, so nothing
   else should move).
6. `git add` nothing new; commit the two files with `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- tokens
# expect: every test in tokens.test.ts passes, the new describe's seven included

npm --prefix desktop run typecheck
# expect: exit 0, no output from tsc

grep -c "^  --t-fact" desktop/src/styles/tokens.css
# expect: 2   (--t-fact and --t-fact-lh; the scope's "1" did not count the line-height token)

grep -c '^:root\[data-theme="paper"\] {' desktop/src/styles/tokens.css
# expect: 1
```

The scope's acceptance said `grep -c "^  --t-fact"` prints `1`. With the line-height token it
prints `2`; `grep -c "^  --t-fact:"` prints `1`, and either is the check.

#### What this task does not claim

- **No visible change.** Nothing uses these tokens until tasks 30 and 31, and nothing sets
  `data-theme` until task 33 renders the paper sheet by hand.
- **Not a theme picker** (C11 `first.keys`, later), **not the high-contrast pair** (W2, tasks 95
  and 96), and not the generator.
- **Not that existing component CSS uses only `--s-*`.** Only the new Phase W directories are held
  to it, by task 30's audit rule C. The Phase 2 stylesheets keep their literal spacing.
- **Not that paper looks right.** The arithmetic says every ink and status colour clears its
  floor; whether the ground reads as paper or as grey is the owner's eyes at task 39.
- **Not that the status hues stay fixed across themes.** `--need` moves 5.8°; the test pins
  meaning by contrast and name, not by angle.
- Design: 05 §4.4 (C1, C3, C4, C7), §4.6; 02 C1, C3, C4, C6, C7, D2 step 1. Parity: the W1 part of
  **L-234** (a light theme beside the dark one; the generated four are W2).

**Interfaces:**
- Produces (seam **SW.11**, the token half):
  - motion `--dur-flash`, `--dur-peek`, `--undo-window`; spacing `--s-1` … `--s-6`; density
    `--row-h`, `--row-h-compact`, `--card-3line`
  - type `--t-{micro,fact,meta,ui,body,head,display}` each with `-lh`; `--t-head-weight`,
    `--t-head-track`; icons `--icon`, `--icon-stroke`, `--icon-sm`, `--icon-sm-stroke`, `--icon-ink`
  - `:root[data-theme="paper"]`, declaring the 34 `THEME_COLOUR_NAMES` and `color-scheme: light`;
    tasks 95 and 96 must emit the same 34 names
- Consumes: the built token layer (`tokens.css` at `f7ea2a2`) and `desktop/src/test/fakes.ts`'s
  `declaredCustomProperties`, `findHexLiterals`, `stripCssComments` (`:259-278`, verified)
- Amends: nothing in the Hub. No wire shape changes.
