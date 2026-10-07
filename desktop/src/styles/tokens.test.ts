import { describe, expect, it } from "vitest";
import tokensCss from "./tokens.css?raw";
import appCss from "./app.css?raw";
import {
  declaredCustomProperties,
  findHexLiterals,
  stripCssComments,
} from "../test/fakes";

/** Contract §5.4: the names the already-written Phase 4 Discover plan depends on. Renaming
 *  one of these breaks `desktop/src/discover/` before it is written. */
const PHASE_4_NAMES = [
  "--page",
  "--canvas",
  "--surface",
  "--inset",
  "--hover",
  "--ink",
  "--ink-2",
  "--ink-3",
  "--ink-4",
  "--line-soft",
  "--line",
  "--line-strong",
  "--accent",
  "--accent-hi",
  "--accent-tint",
  "--run",
  "--need",
  "--fail",
  "--drift",
  "--r-chip",
  "--r-ctl",
  "--r-card",
  "--r-window",
  "--e-hair",
  "--e-card",
  "--ease",
  "--mono",
  "--sans",
];

/** Contract §5.4's nineteen, plus `--scrim`. The twentieth is this task's addition under Group D
 *  shared shape §3B, which routes a token a component needs into the token layer rather than
 *  letting the component write a raw colour function; task 23's `.pal-scrim` is the case. */
const PHASE_2_NAMES = [
  "--hover-2",
  "--field",
  "--thread",
  "--accent-lo",
  "--accent-ink",
  "--accent-ring",
  "--run-tint",
  "--need-tint",
  "--fail-tint",
  "--idle",
  "--exited",
  "--e-btn",
  "--e-raised",
  "--e-overlay",
  "--dur-fast",
  "--dur",
  "--dur-slow",
  "--beat",
  "--numeric",
  "--scrim",
];

/** Relative luminance from opaque OKLCH tokens, through the inverse Oklab transform.
 * This checks readability rather than freezing a historical low-contrast hex value. */
function luminance(token: string, source = tokensCss): number {
  const match = new RegExp(
    `${token}:\\s*oklch\\(\\s*([\\d.]+)%\\s+([\\d.]+)\\s+([\\d.]+)\\s*\\)`,
  ).exec(source);
  if (!match) throw new Error(`Missing opaque OKLCH token ${token}`);
  const L = Number(match[1]) / 100,
    C = Number(match[2]),
    h = (Number(match[3]) * Math.PI) / 180;
  const a = C * Math.cos(h),
    b = C * Math.sin(h);
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  const r = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const blue = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;
  return 0.2126 * r + 0.7152 * g + 0.0722 * blue;
}

describe("the token layer", () => {
  it("keeps text and action labels readable in every light/dark accent combination", () => {
    const css = stripCssComments(tokensCss);
    const block = (selector: string) =>
      css.slice(css.indexOf(selector)).split("}")[0];
    const base = block(":root {");
    for (const theme of ["dark", "light"]) {
      const themeCss =
        (theme === "light" ? block(':root[data-theme="light"] {') : "") + base;
      for (const surface of [
        "--page",
        "--canvas",
        "--surface",
        "--inset",
        "--hover",
        "--hover-2",
        "--field",
      ]) {
        for (const ink of ["--ink", "--ink-2", "--ink-3"]) {
          const values = [
            luminance(ink, themeCss),
            luminance(surface, themeCss),
          ].sort((a, b) => b - a);
          expect(
            (values[0] + 0.05) / (values[1] + 0.05),
            `${theme} ${ink} on ${surface}`,
          ).toBeGreaterThanOrEqual(4.5);
        }
      }
      for (const accent of ["teal", "blue", "rose"]) {
        const accentCss =
          accent === "teal"
            ? themeCss
            : (theme === "light"
                ? block(`:root[data-theme="light"][data-accent="${accent}"] {`)
                : block(`:root[data-accent="${accent}"] {`)) + themeCss;
        for (const fill of ["--accent", "--accent-hi"]) {
          const values = [
            luminance("--accent-ink", accentCss),
            luminance(fill, accentCss),
          ].sort((a, b) => b - a);
          expect(
            (values[0] + 0.05) / (values[1] + 0.05),
            `${theme} ${accent} action label`,
          ).toBeGreaterThanOrEqual(4.5);
        }
      }
    }
  });
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

  it("keeps every readable ink above 4.5:1 across the shared surface ramp", () => {
    for (const ink of ["--ink", "--ink-2", "--ink-3"]) {
      for (const surface of [
        "--page",
        "--canvas",
        "--surface",
        "--inset",
        "--hover",
        "--hover-2",
        "--field",
      ]) {
        expect(
          (luminance(ink) + 0.05) / (luminance(surface) + 0.05),
          `${ink} on ${surface}`,
        ).toBeGreaterThanOrEqual(4.5);
      }
    }
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
    const lines = stripCssComments(appCss)
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean);
    expect(lines[0]).toBe('@import "./tokens.css";');
    expect(findHexLiterals(appCss)).toEqual([]);
    expect(stripCssComments(appCss)).not.toMatch(
      /\b(?:rgb|rgba|hsl|hsla|oklch)\(/,
    );
  });
});
