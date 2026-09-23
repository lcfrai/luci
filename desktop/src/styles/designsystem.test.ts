// Contract §9 (task 24) and §5. The mechanical half of the design system: the rules a
// reviewer cannot hold in their head across ten stylesheets and forty components.
// docs/DESIGN-SYSTEM.md exists because the first prototype failed by looking
// machine-generated, and every rule below is one of that document's non-negotiables, or one of
// Group D shared-shape §3B's six, expressed as a check over the tree rather than as an
// instruction nobody re-reads.
//
// The tree is collected with Vite's `import.meta.glob`, eager and `?raw`, and not walked with
// node:fs. Both halves of that were measured rather than preferred. node:fs does not typecheck
// in this package: `@types/node` is not a dependency, so `import { readFileSync } from
// "node:fs"` is TS2591 under `npm run typecheck`, which is also why vite.config.ts suppresses
// the error on its node:process import. And a hand-written list of `?raw` imports would audit
// only the stylesheets somebody remembered to list. A glob is neither: it is expanded from the file
// system on every run, so a stylesheet a later phase adds is audited the moment it exists, and
// `?raw` hands over the bytes on disk. vitest.config.ts opts `.css?raw` past vitest's
// css-disable plugin; the first assertion below is what notices if that ever stops being true.
//
// The hex and comment regexes come from src/test/fakes.ts — there is one implementation of
// "what is a hex literal" in this repository and task 14 owns it. Failures name files, not
// line numbers: `src/App.css  #646cff`, and one grep finds the line.

import { describe, expect, it } from "vitest";
import { findHexLiterals, stripCssComments } from "../test/fakes";

const CSS_RAW = import.meta.glob<string>("/src/**/*.css", {
  query: "?raw",
  import: "default",
  eager: true,
});
const TS_RAW = import.meta.glob<string>("/src/**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
});

/** Glob keys are root-relative (`/src/components/shell.css`); failures print `src/…`. */
const rel = (key: string): string => key.replace(/^\//, "");

const CSS = Object.keys(CSS_RAW).map(rel).sort();

/** Non-test TypeScript. Test files are excluded deliberately: task 14's tokens.test.ts
 *  carries all nineteen documented hexes as data and this file names the two banned ones.
 *  An audit that may not name the thing it bans is an audit nobody can read. */
const TS = Object.keys(TS_RAW)
  .map(rel)
  .filter((f) => !/\.test\.tsx?$/.test(f) && !f.startsWith("src/test/"))
  .sort();

const TEXT = new Map<string, string>([
  ...Object.entries(CSS_RAW).map(([k, v]): [string, string] => [rel(k), v]),
  ...Object.entries(TS_RAW)
    .map(([k, v]): [string, string] => [rel(k), v])
    .filter(([f]) => TS.includes(f)),
]);

function read(file: string): string {
  const text = TEXT.get(file);
  if (text === undefined) throw new Error(`not collected: ${file}`);
  return text;
}

const TOKENS = "src/styles/tokens.css";

/* ── the rules, as predicates ───────────────────────────────────────────────────────── */

/** The indigo/violet arc, in OKLCH degrees. docs/DESIGN-SYSTEM.md bans the slop pair
 *  #6366f1 / #8b5cf6 outright; contract §5.2 places them at ~276° and ~303° and records
 *  that nothing in the palette lands in the arc except `--drift`, at 316.7°. */
const BANNED_HUE = { min: 250, max: 330 };
const DRIFT_HUE = 316.7;

function oklchHues(css: string): number[] {
  const out: number[] = [];
  for (const match of css.matchAll(/oklch\(([^)]*)\)/g)) {
    const parts = match[1].split("/")[0].trim().split(/\s+/);
    const hue = Number.parseFloat(parts[2] ?? "");
    if (Number.isFinite(hue)) out.push(hue);
  }
  return out;
}

function inBannedArc(hue: number): boolean {
  const h = ((hue % 360) + 360) % 360;
  return h >= BANNED_HUE.min && h < BANNED_HUE.max;
}

/** Every value of one property, whitespace-collapsed. Matched against the declaration
 *  rather than the line: at least one transition in the tree is written across three. */
function declarations(css: string, property: string): string[] {
  const re = new RegExp(`${property}\\s*:\\s*([^;}]*)`, "g");
  return [...css.matchAll(re)].map((m) => m[1].replace(/\s+/g, " ").trim());
}

/** One style rule: its selector, its declarations in source order, and the at-rules it sits
 *  inside. Enough of a parser for these stylesheets — flat rules, some inside @media or
 *  @keyframes — and no more: it does not handle a brace or a semicolon inside a string or a
 *  url(), and none of the ten stylesheets has one outside the token layer. */
interface Rule {
  selector: string;
  decls: { property: string; value: string }[];
  within: string[];
}

function rulesOf(file: string): Rule[] {
  const css = stripCssComments(read(file));
  const out: Rule[] = [];
  const within: string[] = [];
  let start = 0;
  for (let i = 0; i < css.length; i += 1) {
    const ch = css[i];
    if (ch === ";") {
      start = i + 1;   // an @import, between rules
    } else if (ch === "}") {
      within.pop();
      start = i + 1;
    } else if (ch === "{") {
      const prelude = css.slice(start, i).replace(/\s+/g, " ").trim();
      if (prelude.startsWith("@")) {
        within.push(prelude);
        start = i + 1;
        continue;
      }
      const end = css.indexOf("}", i);
      const decls = css
        .slice(i + 1, end)
        .split(";")
        .filter((d) => d.includes(":"))
        .map((d) => {
          const colon = d.indexOf(":");
          return {
            // A vendor prefix is the same property: `-webkit-transition` is a transition.
            property: d.slice(0, colon).trim().replace(/^-(?:webkit|moz|ms|o)-/, ""),
            value: d.slice(colon + 1).replace(/\s+/g, " ").trim(),
          };
        });
      out.push({ selector: prelude, decls, within: [...within] });
      i = end;
      start = end + 1;
    }
  }
  return out;
}

/** Split at top-level commas or spaces, so `cubic-bezier(0.2, 0, 0, 1)` stays one token. */
function splitTop(value: string, sep: "," | " "): string[] {
  const out: string[] = [];
  let depth = 0;
  let current = "";
  for (const ch of value) {
    if (ch === "(") depth += 1;
    else if (ch === ")") depth -= 1;
    if (depth === 0 && (sep === "," ? ch === "," : /\s/.test(ch))) {
      if (current.trim() !== "") out.push(current.trim());
      current = "";
    } else {
      current += ch;
    }
  }
  if (current.trim() !== "") out.push(current.trim());
  return out;
}

/** Shared-shape §3B clause 4, ruled after task 18 disclosed its nine declarations: the
 *  COMPLETE list of what may transition, and at which duration. It is not a record of what
 *  the tree happens to contain — it predates tasks 20 to 23's CSS, and when this audit was
 *  written every transition in the tree was measured against it and none fell outside. A
 *  declaration that needs a fifth entry is a design-system decision, taken before this list
 *  changes and not by changing it. */
const TRANSITIONABLE = new Map<string, { duration: string; only?: { file: string; selector: string } }>([
  ["opacity", { duration: "var(--dur-fast)" }],
  ["background-color", { duration: "var(--dur-fast)" }],
  ["color", { duration: "var(--dur-fast)" }],
  // The one layout transition, on one element: the rail collapsing (contract §6.1). Naming
  // the selector is what stops a second one appearing.
  [
    "grid-template-columns",
    { duration: "var(--dur)", only: { file: "src/components/shell.css", selector: ".shell" } },
  ],
]);

/** Why a transition value is off the list, or nothing if it is on it. `selector` is null for
 *  an inline style, which therefore can never be the `.shell` exception. */
function transitionFaults(value: string, file: string, selector: string | null): string[] {
  if (value === "none") return [];   // the absence of motion is not a transition
  const faults: string[] = [];
  for (const item of splitTop(value, ",")) {
    const [property = "", duration = ""] = splitTop(item, " ");
    const allowed = TRANSITIONABLE.get(property);
    if (allowed === undefined) {
      faults.push(`${property || "<no property>"} is not transitionable`);
    } else if (duration !== allowed.duration) {
      faults.push(`${property} runs at ${duration || "<no duration>"}, not ${allowed.duration}`);
    } else if (
      allowed.only !== undefined &&
      (file !== allowed.only.file || selector !== allowed.only.selector)
    ) {
      faults.push(`${property} is allowed on ${allowed.only.selector} in ${allowed.only.file} alone`);
    }
  }
  return faults;
}

/** A `font` shorthand with real values resets `font-variant-numeric` to `normal`; only the
 *  CSS-wide keywords leave the inherited value alone. */
const resetsNumeric = (value: string): boolean =>
  !/^(?:inherit|initial|unset|revert|revert-layer)$/.test(value);

/** Shared-shape §3B clause 5: the elements that render Phase 2's LIVE numbers. Each writes a
 *  value-bearing `font` shorthand, so each must restate the token after it or its digits
 *  jitter. This list is the REVIEWER's half of clause 5 and is named as such: a stylesheet
 *  cannot say which of its elements print a digit, so a new live number anywhere in the app
 *  needs a new row here, and nothing mechanical will ask for it. */
const LIVE_NUMBER_CARRIERS = [
  // SessionPaneFooter: the session elapsed, and the retry elapsed while a ladder runs.
  { file: "src/components/develop/develop.css", selector: ".pane-foot" },
  // SessionPane's AttachOverlay: "connecting 3s", once a second.
  { file: "src/components/develop/develop.css", selector: ".pane-attach" },
  // CanvasLayout: a tile's coordinates under the pointer (§7.3 names them).
  { file: "src/components/develop/develop.css", selector: ".cv-coords" },
];

const REDUCED_MOTION = /prefers-reduced-motion:\s*reduce/;

describe("the design-system audit", () => {
  it("walks the whole source tree and reads real bytes", () => {
    expect(CSS).toContain(TOKENS);
    expect(CSS).toContain("src/styles/app.css");
    // Ten stylesheets ship in Phase 2: tokens, app, shell, rail, subrail, topbar, common,
    // menu, develop, palette. Fewer means the glob missed a directory and every assertion
    // under it is vacuous — the failure mode task 14's step 2 measured on `?raw`.
    expect(CSS.length).toBeGreaterThanOrEqual(10);
    expect(TS.length).toBeGreaterThanOrEqual(20);
    // A blanked stylesheet arrives as "", and every rule below would pass over it.
    expect(read(TOKENS).length).toBeGreaterThan(2000);
    expect(CSS.filter((f) => read(f).trim() === "")).toEqual([]);
  });

  it("uses no literal hex in any stylesheet", () => {
    const offenders: string[] = [];
    for (const file of CSS) {
      for (const hex of findHexLiterals(stripCssComments(read(file)))) {
        offenders.push(`${file}  ${hex}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("uses no literal hex in any non-test source file", () => {
    // Task 19 already lives by this rule: xterm cannot read a CSS variable, so its theme is
    // resolved from the token layer at construction rather than hard-coded. Any red here is
    // fixed the same way, never by widening the exclusion list.
    const offenders: string[] = [];
    for (const file of TS) {
      for (const hex of findHexLiterals(read(file))) offenders.push(`${file}  ${hex}`);
    }
    expect(offenders).toEqual([]);
  });

  it("contains the indigo/violet tell nowhere at all", () => {
    const tell = ["#6366f1", "#8b5cf6"];
    const offenders = [...TEXT.keys()].filter((f) =>
      tell.some((t) => read(f).toLowerCase().includes(t)),
    );
    expect(offenders).toEqual([]);
  });

  it("lands no colour in the indigo/violet arc, bar the one status token Phase 2 never emits", () => {
    // --drift is defined so Phase 3 does not invent a second purple, and contract §5.2 says
    // so. It is the single permitted member of the arc, and the next test proves it is unused.
    expect(oklchHues(stripCssComments(read(TOKENS))).filter(inBannedArc)).toEqual([DRIFT_HUE]);

    const offenders: string[] = [];
    for (const file of CSS) {
      if (file === TOKENS) continue;
      for (const hue of oklchHues(stripCssComments(read(file))).filter(inBannedArc)) {
        offenders.push(`${file}  oklch hue ${hue}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("defines --drift and references it nowhere, because Phase 2 cannot derive drift", () => {
    expect(stripCssComments(read(TOKENS))).toMatch(/^\s*--drift\s*:/m);
    expect([...TEXT.keys()].filter((f) => read(f).includes("var(--drift)"))).toEqual([]);
  });

  it("keeps every colour function inside the token layer", () => {
    const colourFn = /\b(?:rgba?|hsla?|lab|lch|oklab|oklch|color-mix)\(/g;
    const offenders: string[] = [];
    for (const file of CSS) {
      if (file === TOKENS) continue;
      for (const m of stripCssComments(read(file)).matchAll(colourFn)) {
        offenders.push(`${file}  ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("has every stylesheet but the token layer import the token layer first", () => {
    const offenders: string[] = [];
    for (const file of CSS) {
      if (file === TOKENS) continue;
      // .trim() is load-bearing: the checkout is CRLF, so every split line ends in "\r".
      const first = stripCssComments(read(file))
        .split("\n")
        .map((line) => line.trim())
        .find((line) => line !== "");
      if (first === undefined || !/^@import "(?:\.\/|(?:\.\.\/)+styles\/)tokens\.css";$/.test(first)) {
        offenders.push(`${file}  ${first ?? "<empty file>"}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("never writes `transition: all`, nor a transition that names no property", () => {
    const offenders: string[] = [];
    for (const file of CSS) {
      for (const value of declarations(stripCssComments(read(file)), "transition")) {
        // `transition: 0.75s` is `transition: all` with the word left out.
        if (/^all\b/.test(value) || !/^[a-z-]+[\s,]/.test(`${value} `)) {
          offenders.push(`${file}  transition: ${value}`);
        }
      }
    }
    for (const file of TS) {
      for (const m of read(file).matchAll(/transition\s*:\s*["'`]\s*all\b/g)) {
        offenders.push(`${file}  ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("transitions only what shared-shape §3B clause 4 names, at the duration it names", () => {
    // MECHANICAL: the property, the duration, and — for grid-template-columns — the one file
    // and selector. Also mechanical: no transition longhand outside the token layer, because a
    // `transition-duration` on its own line puts the duration where the list cannot see it,
    // and no transition in an inline style except one the list would accept on its own terms.
    // The token layer's longhands are its prefers-reduced-motion override, which is its job.
    //
    // LEFT TO REVIEW: clause 4 allows background-color and color on "row- and button-sized
    // elements only". A stylesheet does not know how large its element renders, so that
    // qualifier is the reviewer's, not this test's. Nor does the ruling constrain the easing.
    const offenders: string[] = [];
    for (const file of CSS) {
      for (const rule of rulesOf(file)) {
        if (rule.within.some((at) => at.startsWith("@keyframes"))) continue;
        for (const { property, value } of rule.decls) {
          if (property === "transition") {
            for (const fault of transitionFaults(value, file, rule.selector)) {
              offenders.push(`${file}  ${rule.selector}  transition: ${value}  (${fault})`);
            }
          } else if (property.startsWith("transition-") && file !== TOKENS) {
            offenders.push(`${file}  ${rule.selector}  ${property}: ${value}  (write the shorthand)`);
          }
        }
      }
    }
    for (const file of TS) {
      const source = read(file);
      for (const m of source.matchAll(/\btransition\s*:\s*(["'`])([^"'`]*)\1/g)) {
        for (const fault of transitionFaults(m[2].trim(), file, null)) {
          offenders.push(`${file}  inline transition: ${m[2]}  (${fault})`);
        }
      }
      for (const m of source.matchAll(/\btransition(?:Property|Duration|TimingFunction|Delay|Behavior)\s*:/g)) {
        offenders.push(`${file}  inline ${m[0]}  (write the shorthand)`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("gives every live number tabular figures, and never lets a font shorthand undo them", () => {
    // MECHANICAL, clause 5's plumbing: the token exists and is tabular-nums; <body> takes it,
    // so everything inherits it; main.tsx loads the stylesheet that says so (until task 24 it
    // loaded nothing, and the inherited default reached no element at all); every
    // font-variant-numeric outside the token layer is the token; and wherever a rule restates
    // it beside a value-bearing `font` shorthand, it comes AFTER — the shorthand resets it, so
    // a restatement before the shorthand is dead.
    //
    // LEFT TO REVIEW: which elements print a digit. That is LIVE_NUMBER_CARRIERS above, a list
    // a person maintains; the last loop checks each named carrier does restate the token.
    expect(stripCssComments(read(TOKENS))).toMatch(/^\s*--numeric\s*:\s*tabular-nums\s*;/m);
    const body = rulesOf("src/styles/app.css").find((r) => r.selector === "body");
    expect(body?.decls.filter((d) => d.property === "font-variant-numeric")).toEqual([
      { property: "font-variant-numeric", value: "var(--numeric)" },
    ]);
    expect(read("src/main.tsx")).toMatch(/^import "\.\/styles\/app\.css";$/m);

    const offenders: string[] = [];
    for (const file of CSS) {
      if (file === TOKENS) continue;
      for (const rule of rulesOf(file)) {
        const props = rule.decls.map((d) => d.property);
        const numeric = props.lastIndexOf("font-variant-numeric");
        if (numeric === -1) continue;
        if (rule.decls[numeric].value !== "var(--numeric)") {
          offenders.push(`${file}  ${rule.selector}  font-variant-numeric: ${rule.decls[numeric].value}`);
        }
        const shorthand = rule.decls.findIndex((d, i) => i > numeric && d.property === "font" && resetsNumeric(d.value));
        if (shorthand !== -1) {
          offenders.push(`${file}  ${rule.selector}  font-variant-numeric before a font shorthand that resets it`);
        }
      }
    }
    for (const carrier of LIVE_NUMBER_CARRIERS) {
      const rule = rulesOf(carrier.file).find((r) => r.selector === carrier.selector);
      const last = rule?.decls.filter((d) => d.property === "font-variant-numeric").pop();
      if (last?.value !== "var(--numeric)") {
        offenders.push(`${carrier.file}  ${carrier.selector}  renders a live number without var(--numeric)`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("gives every looping animation a static stand-in when motion is reduced", () => {
    // Clause 6. tokens.css zeroes animation-duration under prefers-reduced-motion. A one-shot
    // loses nothing to that — it lands on its end state — but a LOOPING animation is a signal
    // for as long as it runs, and zeroing it deletes the signal. So a selector that loops must
    // also carry a non-animation declaration inside a reduced-motion block, or whatever it
    // distinguished collapses to colour alone.
    //
    // MECHANICAL: that such a stand-in exists for every looping selector in the tree.
    // LEFT TO REVIEW: that the stand-in is visibly distinct from its neighbours (`.dot-live`'s
    // ring is non-inset so it cannot be read as `.dot-ring`'s inset one), and that statuses
    // sharing a shape are separated at all — StatusDot.test.tsx pins the shape map itself.
    const looping: string[] = [];
    const offenders: string[] = [];
    for (const file of CSS) {
      const rules = rulesOf(file);
      for (const rule of rules) {
        if (rule.within.some((at) => at.startsWith("@keyframes") || REDUCED_MOTION.test(at))) continue;
        const loops = rule.decls.some(
          (d) =>
            (d.property === "animation" && /\binfinite\b/.test(d.value)) ||
            (d.property === "animation-iteration-count" && d.value === "infinite"),
        );
        if (!loops) continue;
        looping.push(`${file}  ${rule.selector}`);
        const standIn = rules.some(
          (r) =>
            r.selector === rule.selector &&
            r.within.some((at) => REDUCED_MOTION.test(at)) &&
            r.decls.some((d) => !/^(?:animation|transition)/.test(d.property)),
        );
        if (!standIn) offenders.push(`${file}  ${rule.selector}  loops with no reduced-motion stand-in`);
      }
    }
    // Not vacuous: the shared heartbeat is the one loop Phase 2 ships.
    expect(looping).toContain("src/components/common/common.css  .dot-live");
    expect(offenders).toEqual([]);
  });

  it("builds every shadow from a token or from currentColor", () => {
    // docs/DESIGN-SYSTEM.md: elevation is a 1px ring plus a layered shadow, never a flat
    // border — and never a colour invented at the call site.
    const offenders: string[] = [];
    for (const file of CSS) {
      if (file === TOKENS) continue;
      for (const value of declarations(stripCssComments(read(file)), "box-shadow")) {
        if (!value.includes("var(--") && !value.includes("currentColor")) {
          offenders.push(`${file}  box-shadow: ${value}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  it("ships no gradient anywhere", () => {
    // Tell #1 in docs/DESIGN-SYSTEM.md, and Phase 2 needs none: the grain layer is an SVG
    // noise `url()`, not a gradient. A surface that needs one changes the design system
    // first and this test second, in that order.
    const offenders: string[] = [];
    for (const file of [...CSS, ...TS]) {
      for (const m of stripCssComments(read(file)).matchAll(/\b(?:linear|radial|conic)-gradient\(/g)) {
        offenders.push(`${file}  ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });
});
