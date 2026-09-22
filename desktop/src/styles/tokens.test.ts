import { describe, expect, it } from "vitest";
import tokensCss from "./tokens.css?raw";
import appCss from "./app.css?raw";
import { declaredCustomProperties, findHexLiterals, stripCssComments } from "../test/fakes";

/** Contract §5.4: the names the already-written Phase 4 Discover plan depends on. Renaming
 *  one of these breaks `desktop/src/discover/` before it is written. */
const PHASE_4_NAMES = [
  "--page", "--canvas", "--surface", "--inset", "--hover",
  "--ink", "--ink-2", "--ink-3", "--ink-4",
  "--line-soft", "--line", "--line-strong",
  "--accent", "--accent-hi", "--accent-tint",
  "--run", "--need", "--fail", "--drift",
  "--r-chip", "--r-ctl", "--r-card", "--r-window",
  "--e-hair", "--e-card",
  "--ease", "--mono", "--sans",
];

/** Contract §5.4's nineteen, plus `--scrim`. The twentieth is this task's addition under Group D
 *  shared shape §3B, which routes a token a component needs into the token layer rather than
 *  letting the component write a raw colour function; task 23's `.pal-scrim` is the case. */
const PHASE_2_NAMES = [
  "--hover-2", "--field", "--thread",
  "--accent-lo", "--accent-ink", "--accent-ring",
  "--run-tint", "--need-tint", "--fail-tint", "--idle", "--exited",
  "--e-btn", "--e-raised", "--e-overlay",
  "--dur-fast", "--dur", "--dur-slow", "--beat", "--numeric",
  "--scrim",
];

/** Contract §5.2/§5.3: every derived colour documents the sRGB hex it round-trips to. All
 *  nineteen were re-derived through the inverse Oklab transform while this plan was written:
 *  each is exact, and no channel falls outside [-0.0005, 1.0005]. */
const DOCUMENTED_HEXES = [
  "#0a0908", "#0f0d0b", "#141210", "#1a1815", "#201e1b", "#262320", "#070605",
  "#f2ede7", "#a79f97", "#6c645d", "#453f39",
  "#2fb3bd", "#74d9e1", "#18848c", "#001517",
  "#43b95e", "#e2a33c", "#ef5f45", "#c98ae0",
];

describe("the token layer", () => {
  it("reads the stylesheets as raw text", () => {
    // Guards a vacuous pass. Vitest's css-disable plugin matches BOTH `tokens.css?raw` and
    // `app.css?raw` and rewrites them to `export default ""` unless vitest.config.ts opts the
    // `?raw` query in; every assertion below would then pass over an empty string and prove
    // nothing. A per-file `css.include` blanks whichever of the two it does not name.
    expect(tokensCss.length).toBeGreaterThan(2000);
    expect(appCss.length).toBeGreaterThan(200);
    expect(tokensCss).toContain(":root");
  });

  it("declares every custom property Phase 4 already depends on", () => {
    const declared = new Set(declaredCustomProperties(tokensCss));
    expect(PHASE_4_NAMES.filter((n) => !declared.has(n))).toEqual([]);
    expect(PHASE_4_NAMES).toHaveLength(28);
  });

  it("declares every custom property Phase 2 adds", () => {
    const declared = new Set(declaredCustomProperties(tokensCss));
    expect(PHASE_2_NAMES.filter((n) => !declared.has(n))).toEqual([]);
    expect(PHASE_2_NAMES).toHaveLength(20);
  });

  it("uses no literal hex in any declaration", () => {
    expect(findHexLiterals(stripCssComments(tokensCss))).toEqual([]);
  });

  it("documents the source hex of all nineteen derived colours, in comments only", () => {
    expect(findHexLiterals(tokensCss).sort()).toEqual([...DOCUMENTED_HEXES].sort());
  });

  it("never uses `transition: all`", () => {
    expect(stripCssComments(tokensCss)).not.toMatch(/transition\s*:\s*all\b/);
    expect(stripCssComments(appCss)).not.toMatch(/transition\s*:\s*all\b/);
  });

  it("ships the reduced-motion floor, the focus ring and the dark colour-scheme", () => {
    const css = stripCssComments(tokensCss);
    expect(css).toContain("color-scheme: dark");
    expect(css).toContain("@media (prefers-reduced-motion: reduce)");
    expect(css).toMatch(/:focus-visible\s*\{/);
    expect(css).toContain("outline: 2px solid var(--accent)");
  });

  it("has app.css import the token layer first and own no colour of its own", () => {
    const lines = stripCssComments(appCss).split("\n").map((l) => l.trim()).filter(Boolean);
    expect(lines[0]).toBe('@import "./tokens.css";');
    expect(findHexLiterals(appCss)).toEqual([]);
    expect(stripCssComments(appCss)).not.toMatch(/\b(?:rgb|rgba|hsl|hsla|oklch)\(/);
  });
});
