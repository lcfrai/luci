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
import { designSceneErrors, newDesignFrame, newDesignNode } from "../api/designScene";

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
const SCENE_MODEL = "src/api/designScene.ts";
/** Persisted scalar document data, not editor chrome. Keep the exception private, immutable,
 *  exact in value and location; everything outside this single declaration is still audited. */
const SCENE_PALETTE_DECLARATION = /^const DESIGN_SCENE_DOCUMENT_PALETTE = Object\.freeze\(\{\n  paper: '#ffffff', ink: '#18181b', shape: '#e4e4e7', stroke: '#a1a1aa',\n\}\);$/m;

/** What a rule reads: the real tree, or the real tree with one file planted or replaced. Every
 *  rule below takes one, so that the canaries at the bottom of this file can hand it the fault
 *  it exists to catch. Against the real tree alone, every WEAKENING of a rule passes, because a
 *  clean tree has nothing for it to find — shared shape §3A.1 clause 1, applied to the audit. */
interface Tree {
  css: readonly string[];
  ts: readonly string[];
  read(file: string): string;
}
const REAL: Tree = { css: CSS, ts: TS, read };

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

function rulesOf(source: string): Rule[] {
  const css = stripCssComments(source);
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

/** docs/DESIGN-SYSTEM.md's slop pair, banned by value wherever they appear. */
const TELL = ["#6366f1", "#8b5cf6"];
const COLOUR_FN = /\b(?:rgba?|hsla?|lab|lch|oklab|oklch|color-mix)\(/g;
const TOKEN_IMPORT = /^@import "(?:\.\/|(?:\.\.\/)+styles\/)tokens\.css";$/;
const GRADIENT = /\b(?:linear|radial|conic)-gradient\(/g;

/* ── each rule, as a function from a tree to its offenders ──────────────────────────── */
// Every offender begins "<file>  ", which is what the canaries match on.

function hexInStylesheets(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    for (const hex of findHexLiterals(stripCssComments(t.read(file)))) {
      offenders.push(`${file}  ${hex}`);
    }
  }
  return offenders;
}

function hexInSource(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.ts) {
    const source = t.read(file).replace(/\r\n/g, "\n");
    const chrome = file === SCENE_MODEL ? source.replace(SCENE_PALETTE_DECLARATION, "") : source;
    for (const hex of findHexLiterals(chrome)) offenders.push(`${file}  ${hex}`);
  }
  return offenders;
}

function violetTell(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of [...t.css, ...t.ts]) {
    const text = t.read(file).toLowerCase();
    for (const tell of TELL) if (text.includes(tell)) offenders.push(`${file}  ${tell}`);
  }
  return offenders;
}

/** --drift is defined so Phase 3 does not invent a second purple, and contract §5.2 says so.
 *  It is the single permitted member of the arc, once, in the token layer. */
function violetArc(t: Tree): string[] {
  const offenders: string[] = [];
  const tokenHues = oklchHues(stripCssComments(t.read(TOKENS))).filter(inBannedArc);
  if (tokenHues.length !== 1 || tokenHues[0] !== DRIFT_HUE) {
    offenders.push(`${TOKENS}  in-arc hues [${tokenHues.join(", ")}], where only --drift's ${DRIFT_HUE} may be`);
  }
  for (const file of t.css) {
    if (file === TOKENS) continue;
    for (const hue of oklchHues(stripCssComments(t.read(file))).filter(inBannedArc)) {
      offenders.push(`${file}  oklch hue ${hue}`);
    }
  }
  return offenders;
}

function driftReferenced(t: Tree): string[] {
  return [...t.css, ...t.ts]
    .filter((file) => t.read(file).includes("var(--drift)"))
    .map((file) => `${file}  var(--drift)`);
}

function colourOutsideTokens(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    if (file === TOKENS) continue;
    for (const m of stripCssComments(t.read(file)).matchAll(COLOUR_FN)) {
      offenders.push(`${file}  ${m[0]}`);
    }
  }
  return offenders;
}

/** CSS Color 4's named colours. `transparent` and `currentcolor` are left out on purpose: they
 *  name no hue, and §3B clause 2 allows both. So are the CSS-wide keywords and `none`. */
const NAMED_COLOURS = new Set(
  (
    "aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue " +
    "blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk " +
    "crimson cyan darkblue darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki " +
    "darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen " +
    "darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue " +
    "dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite " +
    "gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory khaki " +
    "lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan " +
    "lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen " +
    "lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen " +
    "magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen " +
    "mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream " +
    "mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid " +
    "palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum " +
    "powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown " +
    "seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen " +
    "steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow yellowgreen"
  ).split(" "),
);

/** The properties whose value can carry a colour. box-shadow is shadowFaults' own. A source
 *  fragment with no capturing group, because `imperativeStyles` embeds it in its own groups. */
const COLOUR_KEBAB =
  "color|background(?:-color)?|border(?:-(?:top|right|bottom|left|block|inline)(?:-(?:start|end))?)?(?:-color)?|outline(?:-color)?|fill|stroke|caret-color|accent-color|text-decoration(?:-color)?|text-emphasis(?:-color)?|column-rule(?:-color)?|text-shadow|scrollbar-color|stop-color|flood-color|lighting-color";
const COLOUR_PROPERTY = new RegExp(`^(?:${COLOUR_KEBAB})$`);

/** The same, as camelCase: the keys of an inline style object, and the members of `el.style`. */
const COLOUR_CAMEL =
  "color|background(?:Color)?|border(?:Top|Right|Bottom|Left|Block|Inline)?(?:Start|End)?(?:Color)?|outline(?:Color)?|fill|stroke|caretColor|accentColor|textDecoration(?:Color)?|textShadow";
const COLOUR_KEY = new RegExp(`\\b(${COLOUR_CAMEL})\\s*:\\s*(["'\`])([^"'\`]*)\\2`, "g");

/** Every named colour in one value, as whole words. A custom property's name is not a word here
 *  (`--ink-red` is a token, not red), and a url() is skipped whole, because its payload is not
 *  CSS. A var() FALLBACK is read like any other value: `var(--ink, black)` puts black on screen
 *  the day the token is missing. */
function namedIn(value: string): string[] {
  const bare = value.replace(/url\((?:"[^"]*"|'[^']*'|[^)]*)\)/g, " ");
  return [...bare.matchAll(/(?<![\w-])[a-z]+(?![\w-])/gi)]
    .map((m) => m[0].toLowerCase())
    .filter((word) => NAMED_COLOURS.has(word));
}

/** Shared-shape §3B clause 2: a colour is a `var(--token)`. COLOUR_FN and the hex rules cover
 *  the other spellings. ADDED BY TASK 24's FIX ROUND 1 (review M2): `color: white` passed every
 *  rule, and the audit's commit message said "no colour outside the token layer".
 *
 *  FIX ROUND 2 (re-review M1) added the third loop: every form `imperativeStyles` reads, which
 *  is how `el.style.color = "white"` beside CanvasLayout.tsx's own imperative writes passed the
 *  whole suite. A value that is not a string literal is skipped, not reported: unlike a
 *  transition, which must be checked against a list, a colour held in a variable is no more
 *  readable here than the same variable as an object key, which the second loop also skips. */
function namedColours(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    if (file === TOKENS) continue;
    for (const rule of rulesOf(t.read(file))) {
      for (const { property, value } of rule.decls) {
        if (!COLOUR_PROPERTY.test(property)) continue;
        for (const name of namedIn(value)) offenders.push(`${file}  ${rule.selector}  ${property}: ${name}`);
      }
    }
  }
  for (const file of t.ts) {
    const source = t.read(file);
    for (const m of source.matchAll(COLOUR_KEY)) {
      for (const name of namedIn(m[3])) offenders.push(`${file}  inline ${m[1]}: ${name}`);
    }
    for (const { value, text } of imperativeStyles(source, COLOUR_CAMEL, COLOUR_KEBAB)) {
      if (value === null) continue;
      for (const name of namedIn(value)) offenders.push(`${file}  inline ${text} ${value}: ${name}`);
    }
  }
  return offenders;
}

function importNotFirst(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    if (file === TOKENS) continue;
    // .trim() is load-bearing: the checkout is CRLF, so every split line ends in "\r".
    const first = stripCssComments(t.read(file))
      .split("\n")
      .map((line) => line.trim())
      .find((line) => line !== "");
    if (first === undefined || !TOKEN_IMPORT.test(first)) {
      offenders.push(`${file}  ${first ?? "<empty file>"}`);
    }
  }
  return offenders;
}

function transitionAllOrNoProperty(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    for (const value of declarations(stripCssComments(t.read(file)), "transition")) {
      // `transition: 0.75s` is `transition: all` with the word left out.
      if (/^all\b/.test(value) || !/^[a-z-]+[\s,]/.test(`${value} `)) {
        offenders.push(`${file}  transition: ${value}`);
      }
    }
  }
  for (const file of t.ts) {
    for (const m of t.read(file).matchAll(/transition\s*:\s*["'`]\s*all\b/g)) {
      offenders.push(`${file}  ${m[0]}`);
    }
  }
  return offenders;
}

function transitionOffList(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    for (const rule of rulesOf(t.read(file))) {
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
  for (const file of t.ts) {
    const source = t.read(file);
    for (const m of source.matchAll(/\btransition\s*:\s*(["'`])([^"'`]*)\1/g)) {
      for (const fault of transitionFaults(m[2].trim(), file, null)) {
        offenders.push(`${file}  inline transition: ${m[2]}  (${fault})`);
      }
    }
    for (const m of source.matchAll(/\btransition(?:Property|Duration|TimingFunction|Delay|Behavior)\s*:/g)) {
      offenders.push(`${file}  inline ${m[0]}  (write the shorthand)`);
    }
    for (const { property, value, text } of imperativeTransitions(source)) {
      if (property !== "transition") {
        offenders.push(`${file}  inline ${text}  (write the unprefixed shorthand)`);
      } else if (value === null) {
        offenders.push(`${file}  inline ${text}  (not a string literal, so the list cannot read it)`);
      } else {
        for (const fault of transitionFaults(value.trim(), file, null)) {
          offenders.push(`${file}  inline ${text} ${value}  (${fault})`);
        }
      }
    }
  }
  return offenders;
}

/** Every transition a .ts/.tsx file writes onto an element OTHER than as an object-literal key,
 *  which the two loops above read:
 *    - an assignment to `.style.transition…` or `.style["transition…"]`;
 *    - `.setProperty("transition…", …)`;
 *    - a declaration inside a style string, `.cssText = "…"` or `setAttribute("style", "…")`.
 *  ADDED BY TASK 24's FIX ROUND 1 (review I3). CanvasLayout.tsx writes `el.style.transform`
 *  imperatively, which makes the canvas tile the likeliest place anyone will "smooth" a move,
 *  and `el.style.transition = "transform 200ms"` beside those lines passed the whole suite.
 *  `value` is null where the value is not a string literal: the list cannot read a variable, so
 *  the caller reports it outright rather than guess. */
function imperativeTransitions(source: string): ImperativeStyle[] {
  return imperativeStyles(source, "transition\\w*", "(?:-webkit-)?transition[\\w-]*");
}

interface ImperativeStyle {
  property: string;
  value: string | null;
  text: string;
}

/** The reader behind `imperativeTransitions`, parameterised by property so `namedColours` reads
 *  the same four forms. FACTORED OUT BY TASK 24's FIX ROUND 2 (re-review M1): the colour rule
 *  had the blind spot round 1 closed for transitions, because only the transition rule read
 *  imperative writes. One reader for both, so a fifth form added for one reaches the other.
 *  `camel` names an `el.style` member; `kebab` names a CSS property, as a bracketed key, a
 *  `setProperty` argument or a declaration in a style string. Both are regex SOURCE with no
 *  capturing group. A camel name is the whole member because `\s*=` must follow it at once, so
 *  `el.style.colorScheme = …` is not read as `color`. */
function imperativeStyles(source: string, camel: string, kebab: string): ImperativeStyle[] {
  const found: ImperativeStyle[] = [];
  const literalAt = (at: number): string | null =>
    /^(["'`])([^"'`]*)\1/.exec(source.slice(at))?.[2] ?? null;
  const assigned = new RegExp(
    `\\.style(?:\\.(?<camel>${camel})|\\[\\s*(?<q>["'\`])(?<kebab>${kebab})\\k<q>\\s*\\])\\s*=(?!=)\\s*`,
    "g",
  );
  for (const m of source.matchAll(assigned)) {
    const property = m.groups?.camel ?? m.groups?.kebab ?? "";
    found.push({ property, value: literalAt((m.index ?? 0) + m[0].length), text: m[0].trim() });
  }
  const set = new RegExp(`\\bsetProperty\\(\\s*(?<q>["'\`])(?<kebab>${kebab})\\k<q>\\s*,\\s*`, "g");
  for (const m of source.matchAll(set)) {
    found.push({ property: m.groups?.kebab ?? "", value: literalAt((m.index ?? 0) + m[0].length), text: m[0].trim() });
  }
  const wanted = new RegExp(`^(?:${kebab})$`);
  const styleString = /(?:\.cssText\s*\+?=|\bsetAttribute\(\s*["'`]style["'`]\s*,)\s*(["'`])([^"'`]*)\1/g;
  for (const m of source.matchAll(styleString)) {
    for (const declaration of m[2].split(";")) {
      const colon = declaration.indexOf(":");
      const property = declaration.slice(0, colon).trim();
      if (colon === -1 || !wanted.test(property)) continue;
      found.push({ property, value: declaration.slice(colon + 1).trim(), text: m[0].trim() });
    }
  }
  return found;
}

/** Clause 5's per-rule half: every font-variant-numeric outside the token layer is the token,
 *  and none sits before a value-bearing `font` shorthand that resets it. Then each named carrier
 *  restates it. `carriers` is a parameter so a canary can name a planted one. */
function numericFaults(t: Tree, carriers: readonly { file: string; selector: string }[]): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    if (file === TOKENS) continue;
    for (const rule of rulesOf(t.read(file))) {
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
  for (const carrier of carriers) {
    const rule = rulesOf(t.read(carrier.file)).find((r) => r.selector === carrier.selector);
    const last = rule?.decls.filter((d) => d.property === "font-variant-numeric").pop();
    if (last?.value !== "var(--numeric)") {
      offenders.push(`${carrier.file}  ${carrier.selector}  renders a live number without var(--numeric)`);
    }
  }
  return offenders;
}

/** Every looping selector, and those among them with no reduced-motion stand-in. */
function loops(t: Tree): { looping: string[]; offenders: string[] } {
  const looping: string[] = [];
  const offenders: string[] = [];
  for (const file of t.css) {
    const rules = rulesOf(t.read(file));
    for (const rule of rules) {
      if (rule.within.some((at) => at.startsWith("@keyframes") || REDUCED_MOTION.test(at))) continue;
      const loopsHere = rule.decls.some(
        (d) =>
          (d.property === "animation" && /\binfinite\b/.test(d.value)) ||
          (d.property === "animation-iteration-count" && d.value === "infinite"),
      );
      if (!loopsHere) continue;
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
  return { looping, offenders };
}

function shadowFaults(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    if (file === TOKENS) continue;
    for (const value of declarations(stripCssComments(t.read(file)), "box-shadow")) {
      if (value === "none") continue; // Removing elevation introduces no colour.
      if (!value.includes("var(--") && !value.includes("currentColor")) {
        offenders.push(`${file}  box-shadow: ${value}`);
      }
    }
  }
  return offenders;
}

function gradients(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of [...t.css, ...t.ts]) {
    for (const m of stripCssComments(t.read(file)).matchAll(GRADIENT)) {
      offenders.push(`${file}  ${m[0]}`);
    }
  }
  return offenders;
}

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
    expect(hexInStylesheets(REAL)).toEqual([]);
  });

  it("uses no literal hex outside the exact persisted Design document palette", () => {
    // Task 19 already lives by this rule: xterm cannot read a CSS variable, so its theme is
    // resolved from the token layer at construction rather than hard-coded. Any red here is
    // fixed the same way. Saved Design colours instead cross a literal-only wire schema;
    // only their exact private data declaration is removed, never the surrounding source file.
    expect(hexInSource(REAL)).toEqual([]);
    expect(read(SCENE_MODEL).replace(/\r\n/g, "\n")).toMatch(SCENE_PALETTE_DECLARATION);
  });

  it("keeps the persisted Design palette schema-valid and independent of the editor theme", () => {
    const frame = newDesignFrame({id: "screen-home", name: "Home", route: "/", purpose: "Home screen", states: [], widths: [390], requirementIds: [], accessibility: "Keyboard navigation"});
    const kinds = ["rectangle", "text", "button", "input", "divider"] as const;
    frame.nodes = kinds.map(kind => newDesignNode(kind, frame));
    expect(frame.background).toBe("#ffffff");
    expect(frame.nodes.map(node => [node.style.fill, node.style.stroke, node.style.color])).toEqual([
      ["#e4e4e7", "transparent", "#18181b"],
      ["transparent", "transparent", "#18181b"],
      ["#18181b", "transparent", "#ffffff"],
      ["transparent", "#a1a1aa", "#18181b"],
      ["#e4e4e7", "transparent", "#18181b"],
    ]);
    expect(designSceneErrors(JSON.parse(JSON.stringify({version: 1, frames: [frame]})))).toEqual([]);
    expect(designSceneErrors({version: 1, frames: [{...frame, background: "var(--surface)"}]})).not.toEqual([]);
  });

  it("contains the indigo/violet tell nowhere at all", () => {
    expect(violetTell(REAL)).toEqual([]);
  });

  it("lands no colour in the indigo/violet arc, bar the one status token Phase 2 never emits", () => {
    // The next test proves --drift, the one permitted member, is unused.
    expect(violetArc(REAL)).toEqual([]);
  });

  it("defines --drift and references it nowhere, because Phase 2 cannot derive drift", () => {
    expect(stripCssComments(read(TOKENS))).toMatch(/^\s*--drift\s*:/m);
    expect(driftReferenced(REAL)).toEqual([]);
  });

  it("keeps every colour function inside the token layer", () => {
    expect(colourOutsideTokens(REAL)).toEqual([]);
  });

  it("names no colour by keyword outside the token layer", () => {
    // MECHANICAL: every named colour in a colour-bearing declaration, an inline style key, or
    // any of the imperative forms `imperativeStyles` reads (an `el.style` assignment, a
    // bracketed key, `setProperty`, `cssText`, `setAttribute("style", …)`; fix round 2,
    // re-review M1). A string literal only: a colour held in a variable is not readable here.
    // `currentColor`, `transparent`, `inherit` and `none` name no hue and are allowed.
    expect(namedColours(REAL)).toEqual([]);
  });

  it("has every stylesheet but the token layer import the token layer first", () => {
    expect(importNotFirst(REAL)).toEqual([]);
  });

  it("never writes `transition: all`, nor a transition that names no property", () => {
    expect(transitionAllOrNoProperty(REAL)).toEqual([]);
  });

  it("transitions only what shared-shape §3B clause 4 names, at the duration it names", () => {
    // MECHANICAL: the property, the duration, and — for grid-template-columns — the one file
    // and selector. Also mechanical: no transition longhand outside the token layer, because a
    // `transition-duration` on its own line puts the duration where the list cannot see it,
    // and no transition in an inline style except one the list would accept on its own terms.
    // "An inline style" is every form a .ts/.tsx file can write one in: an object-literal key,
    // an assignment to `.style.transition…`, `setProperty("transition…", …)`, and a style
    // string (`cssText`, `setAttribute("style", …)`). The last three are `imperativeTransitions`,
    // added by task 24's fix round 1, when only the first was read (review I3). Fix round 2
    // moved its body into `imperativeStyles`, which `namedColours` also reads.
    // The token layer's longhands are its prefers-reduced-motion override, which is its job.
    //
    // LEFT TO REVIEW: clause 4 allows background-color and color on "row- and button-sized
    // elements only". A stylesheet does not know how large its element renders, so that
    // qualifier is the reviewer's, not this test's. Nor does the ruling constrain the easing.
    expect(transitionOffList(REAL)).toEqual([]);
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
    const body = rulesOf(read("src/styles/app.css")).find((r) => r.selector === "body");
    expect(body?.decls.filter((d) => d.property === "font-variant-numeric")).toEqual([
      { property: "font-variant-numeric", value: "var(--numeric)" },
    ]);
    expect(read("src/main.tsx")).toMatch(/^import "\.\/styles\/app\.css";$/m);
    expect(numericFaults(REAL, LIVE_NUMBER_CARRIERS)).toEqual([]);
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
    const { looping, offenders } = loops(REAL);
    // Not vacuous: the shared heartbeat is the one loop Phase 2 ships.
    expect(looping).toContain("src/components/common/common.css  .dot-live");
    expect(offenders).toEqual([]);
  });

  it("builds every shadow from a token or from currentColor", () => {
    // docs/DESIGN-SYSTEM.md: elevation is a 1px ring plus a layered shadow, never a flat
    // border — and never a colour invented at the call site.
    expect(shadowFaults(REAL)).toEqual([]);
  });

  it("ships no gradient anywhere", () => {
    // Tell #1 in docs/DESIGN-SYSTEM.md, and Phase 2 needs none: the grain layer is an SVG
    // noise `url()`, not a gradient. A surface that needs one changes the design system
    // first and this test second, in that order.
    expect(gradients(REAL)).toEqual([]);
  });
});

/* ── the canaries: each rule, handed the one fault it exists to catch ──────────────── */
// Every rule above is green against a tree with nothing wrong in it, and so is every weakened
// copy of it: a regex that lost an alternative, a condition that lost a clause. These cases are
// what fails instead. Each plants one file (or replaces one) holding one fault, chosen so that
// the condition under test is the only thing that can catch it (shared shape §3A.1 clause 4),
// and asserts the rule names that file.

const PLANT_CSS = "src/components/planted/planted.css";
const PLANT_TSX = "src/components/planted/Planted.tsx";
const HEAD = '@import "../../styles/tokens.css";\n';
const SHELL = "src/components/shell.css";

/** The real tree with `file` added, or replaced where it already exists. */
function plant(file: string, text: string): Tree {
  const add = (list: readonly string[], applies: boolean): readonly string[] =>
    applies && !list.includes(file) ? [...list, file].sort() : list;
  return {
    css: add(CSS, file.endsWith(".css")),
    ts: add(TS, /\.tsx?$/.test(file)),
    read: (f) => (f === file ? text : read(f)),
  };
}

interface Canary {
  fault: string;
  rule: (t: Tree) => string[];
  file: string;
  text: string;
}

const sheet = (body: string): string => `${HEAD}${body}\n`;
/** A planted source file whose one line writes to an element, as CanvasLayout.tsx's do. */
const imperative = (line: string): string =>
  `const MOVE = "transform 200ms";\nexport function place(el: HTMLElement): void {\n  el.style.transform = "translate3d(0px, 0px, 0)";\n  ${line}\n}\n`;
const numeric = (t: Tree): string[] => numericFaults(t, LIVE_NUMBER_CARRIERS);
const loopFaults = (t: Tree): string[] => loops(t).offenders;

const CANARIES: Canary[] = [
  { fault: "a hex in a stylesheet", rule: hexInStylesheets, file: PLANT_CSS, text: sheet(".p { color: #ff0000; }") },
  { fault: "a hex in source", rule: hexInSource, file: PLANT_TSX, text: 'export const INK = "#ff0000";\n' },
  { fault: "a changed persisted Design palette value", rule: hexInSource, file: SCENE_MODEL, text: read(SCENE_MODEL).replace("'#ffffff'", "'#fefefe'") },
  { fault: "a literal outside the persisted Design palette in its own model", rule: hexInSource, file: SCENE_MODEL, text: `${read(SCENE_MODEL)}\nconst chrome = "#ffffff";\n` },
  { fault: "the persisted Design palette copied to a component", rule: hexInSource, file: PLANT_TSX, text: read(SCENE_MODEL) },
  { fault: "an exported persisted Design palette available to chrome", rule: hexInSource, file: SCENE_MODEL, text: read(SCENE_MODEL).replace("const DESIGN_SCENE_DOCUMENT_PALETTE", "export const DESIGN_SCENE_DOCUMENT_PALETTE") },
  // Upper case on purpose: the tell is banned by value, however it is spelled.
  { fault: "the tell #6366f1", rule: violetTell, file: PLANT_TSX, text: 'export const A = "#6366F1";\n' },
  { fault: "the tell #8b5cf6", rule: violetTell, file: PLANT_TSX, text: 'export const B = "#8b5cf6";\n' },
  // Contract §5.2 places the slop pair at ~276° and ~303°; both must sit inside the arc.
  { fault: "a token at #6366f1's hue", rule: violetArc, file: TOKENS, text: `${read(TOKENS)}\n:root { --x: oklch(58% 0.2 276); }\n` },
  { fault: "a token at #8b5cf6's hue", rule: violetArc, file: TOKENS, text: `${read(TOKENS)}\n:root { --x: oklch(60% 0.2 303); }\n` },
  { fault: "an in-arc hue outside the token layer", rule: violetArc, file: PLANT_CSS, text: sheet(".p { color: oklch(60% 0.2 290); }") },
  { fault: "var(--drift) in use", rule: driftReferenced, file: PLANT_CSS, text: sheet(".p { color: var(--drift); }") },
  ...["rgb", "rgba", "hsl", "hsla", "lab", "lch", "oklab", "oklch", "color-mix"].map(
    (fn): Canary => ({ fault: `${fn}() outside the token layer`, rule: colourOutsideTokens, file: PLANT_CSS, text: sheet(`.p { color: ${fn}(1 2 3); }`) }),
  ),
  { fault: "a stylesheet that imports nothing", rule: importNotFirst, file: PLANT_CSS, text: ".p { color: var(--ink); }\n" },
  { fault: "a stylesheet that imports something else first", rule: importNotFirst, file: PLANT_CSS, text: '@import "./other.css";\n.p { color: var(--ink); }\n' },
  { fault: "transition: all", rule: transitionAllOrNoProperty, file: PLANT_CSS, text: sheet(".p { transition: all var(--dur-fast); }") },
  { fault: "a transition that names no property", rule: transitionAllOrNoProperty, file: PLANT_CSS, text: sheet(".p { transition: 0.75s; }") },
  { fault: "an inline transition: all", rule: transitionAllOrNoProperty, file: PLANT_TSX, text: 'export const s = { transition: "all 1s" };\n' },
  { fault: "a property off the list", rule: transitionOffList, file: PLANT_CSS, text: sheet(".p { transition: transform var(--dur-fast) var(--ease); }") },
  { fault: "a listed property at the wrong duration", rule: transitionOffList, file: PLANT_CSS, text: sheet(".p { transition: opacity 200ms var(--ease); }") },
  { fault: "the grid transition in another file", rule: transitionOffList, file: PLANT_CSS, text: sheet(".shell { transition: grid-template-columns var(--dur) var(--ease); }") },
  { fault: "the grid transition on another selector in shell.css", rule: transitionOffList, file: SHELL, text: `${read(SHELL)}\n.shell-x { transition: grid-template-columns var(--dur) var(--ease); }\n` },
  { fault: "a vendor-prefixed transition", rule: transitionOffList, file: PLANT_CSS, text: sheet(".p { -webkit-transition: transform var(--dur-fast); }") },
  { fault: "a transition longhand", rule: transitionOffList, file: PLANT_CSS, text: sheet(".p { transition-duration: 2s; }") },
  { fault: "an inline transition off the list", rule: transitionOffList, file: PLANT_TSX, text: 'export const s = { transition: "transform 1s" };\n' },
  { fault: "an inline transition longhand", rule: transitionOffList, file: PLANT_TSX, text: 'export const s = { transitionDuration: "1s" };\n' },
  // Task 24's fix round 1 (review I3): one canary per form `imperativeTransitions` reads. The
  // first is mutant T3, planted beside the three imperative writes CanvasLayout.tsx really makes.
  { fault: "a transition assigned to an element's style (T3)", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style.transition = "transform 200ms";') },
  { fault: "a transition assigned a value the list cannot read", rule: transitionOffList, file: PLANT_TSX, text: imperative("el.style.transition = MOVE;") },
  { fault: "a transition longhand assigned to an element's style", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style.transitionDuration = "1s";') },
  // "none" is the one value transitionFaults passes, so only the longhand's own branch can see
  // this. Any other longhand value is also a fault as a shorthand, which is how mutant W5 (a
  // longhand treated as a shorthand) survived the canary above.
  { fault: "a transition longhand whose value reads as no transition", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style.transitionProperty = "none";') },
  { fault: "a transition assigned through a bracketed key", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style["transition"] = "transform 200ms";') },
  { fault: "a transition set through setProperty", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style.setProperty("transition", "transform 200ms");') },
  { fault: "a transition longhand set through setProperty", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style.setProperty("transition-duration", "1s");') },
  { fault: "a transition inside cssText", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style.cssText = "width: 10px; transition: transform 200ms";') },
  // Fix round 2: SW's transition half. A longhand is the one transition property whose camel
  // and kebab spellings differ.
  { fault: "a transition longhand inside cssText", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.style.cssText = "transition-duration: 1s";') },
  { fault: "a transition inside a style attribute", rule: transitionOffList, file: PLANT_TSX, text: imperative('el.setAttribute("style", "transition: transform 200ms");') },
  // Review M2. HEX_LITERAL's other three lengths: a rule narrowed to six digits passed #ff0000.
  ...["#fff", "#ffff", "#ff000080"].map(
    (hex): Canary => ({ fault: `the hex ${hex} in a stylesheet`, rule: hexInStylesheets, file: PLANT_CSS, text: sheet(`.p { color: ${hex}; }`) }),
  ),
  { fault: "a named colour", rule: namedColours, file: PLANT_CSS, text: sheet(".p { color: white; }") },
  { fault: "a named colour inside a shorthand", rule: namedColours, file: PLANT_CSS, text: sheet(".p { border: 1px solid Black; }") },
  { fault: "a named colour as a var() fallback", rule: namedColours, file: PLANT_CSS, text: sheet(".p { background-color: var(--page, red); }") },
  { fault: "a named colour in an inline style", rule: namedColours, file: PLANT_TSX, text: 'export const s = { backgroundColor: "white" };\n' },
  // Task 24's fix round 2 (re-review M1): one canary per imperative form, the same four forms
  // `imperativeTransitions` reads. P1 and X2 are the re-review's two survivors, planted beside
  // the imperative writes CanvasLayout.tsx really makes.
  { fault: "a named colour assigned to an element's style (P1)", rule: namedColours, file: PLANT_TSX, text: imperative('el.style.color = "white";') },
  { fault: "a named colour assigned to a longhand colour key", rule: namedColours, file: PLANT_TSX, text: imperative('el.style.borderTopColor = "red";') },
  { fault: "a named colour assigned through a bracketed key", rule: namedColours, file: PLANT_TSX, text: imperative('el.style["background-color"] = "white";') },
  { fault: "a named colour set through setProperty (X2)", rule: namedColours, file: PLANT_TSX, text: imperative('el.style.setProperty("color", "white");') },
  // A hyphenated property on purpose. `color`, `border` and `transition` are spelled the same in
  // camelCase, so with those alone a style-string reader handed the camel fragment instead of
  // the kebab one passed every canary (mutant SW, predicted and survived).
  { fault: "a named colour inside cssText", rule: namedColours, file: PLANT_TSX, text: imperative('el.style.cssText = "width: 10px; background-color: white";') },
  { fault: "a named colour inside a style attribute", rule: namedColours, file: PLANT_TSX, text: imperative('el.setAttribute("style", "border: 1px solid black");') },
  { fault: "font-variant-numeric that is not the token", rule: numeric, file: PLANT_CSS, text: sheet(".p { font-variant-numeric: tabular-nums; }") },
  { fault: "the token before a font shorthand that resets it", rule: numeric, file: PLANT_CSS, text: sheet(".p { font-variant-numeric: var(--numeric); font: 500 10px/1 var(--mono); }") },
  {
    fault: "a live-number carrier that never restates the token",
    // Named as a carrier only where it exists: against the real tree the planted file does not,
    // and reading it would throw rather than answer.
    rule: (t) => numericFaults(t, t.css.includes(PLANT_CSS) ? [{ file: PLANT_CSS, selector: ".p" }] : []),
    file: PLANT_CSS,
    text: sheet(".p { font: 400 11px/1.4 var(--mono); }"),
  },
  { fault: "a loop with no reduced-motion block", rule: loopFaults, file: PLANT_CSS, text: sheet(".p { animation: beat var(--beat) var(--ease) infinite; }") },
  {
    fault: "a loop whose reduced-motion block only stops it",
    rule: loopFaults,
    file: PLANT_CSS,
    text: sheet(".p { animation: beat var(--beat) var(--ease) infinite; }\n@media (prefers-reduced-motion: reduce) {\n  .p { animation: none; }\n}"),
  },
  { fault: "a shadow with a literal colour", rule: shadowFaults, file: PLANT_CSS, text: sheet(".p { box-shadow: 0 1px 2px black; }") },
  ...["linear", "radial", "conic"].map(
    (kind): Canary => ({ fault: `a ${kind} gradient`, rule: gradients, file: PLANT_CSS, text: sheet(`.p { background-image: ${kind}-gradient(var(--ink), var(--page)); }`) }),
  ),
  { fault: "a gradient in source", rule: gradients, file: PLANT_TSX, text: 'export const s = { background: "linear-gradient(red, blue)" };\n' },
];

describe("the audit's own rules, fed a planted tree", () => {
  for (const canary of CANARIES) {
    it(`catches ${canary.fault}`, () => {
      const onFile = (offenders: string[]): string[] =>
        offenders.filter((o) => o.startsWith(`${canary.file}  `));
      // Nothing on that file in the real tree, so whatever is found below is the plant.
      expect(onFile(canary.rule(REAL))).toEqual([]);
      expect(onFile(canary.rule(plant(canary.file, canary.text)))).not.toEqual([]);
    });
  }

  // Fix round 2 (re-review M1). A canary proves a rule sees a fault; this proves the colour
  // rule's declared limit holds without throwing: a colour held in a variable is not a literal,
  // so it is skipped, as the object-key half skips `{ color: tone }`. Without the skip,
  // `namedIn(null)` throws and the whole audit reports nothing at all.
  it("reads an imperative colour held in a variable as nothing, and does not throw", () => {
    const planted = plant(PLANT_TSX, imperative("el.style.color = MOVE;"));
    expect(namedColours(planted).filter((o) => o.startsWith(`${PLANT_TSX}  `))).toEqual([]);
  });

  // Fix round 2. The canaries prove each imperative form is SEEN; this proves each is read as
  // the property it names, so a transition the list accepts passes in every form. Mutant SP
  // (the bracketed key's property dropped) survived every canary, because an off-list value is
  // an offender whatever the reason, and would have failed this on-list write as a prefixed one.
  it("accepts a transition the list allows, in every imperative form", () => {
    const allowed = "opacity var(--dur-fast) var(--ease)";
    for (const line of [
      `el.style.transition = "${allowed}";`,
      `el.style["transition"] = "${allowed}";`,
      `el.style.setProperty("transition", "${allowed}");`,
      `el.style.cssText = "width: 10px; transition: ${allowed}";`,
      `el.setAttribute("style", "transition: ${allowed}");`,
    ]) {
      const planted = plant(PLANT_TSX, imperative(line));
      expect(transitionOffList(planted).filter((o) => o.startsWith(`${PLANT_TSX}  `)), line).toEqual([]);
    }
  });
});
