### Task 28: drawn glyphs, part 1: the nine issue states, *done, not proven*, the priority notch

Stream **WD1**, defines `<StateGlyph/>`, the first member of seam **SW.12**, about 3 hours (an
estimate, not a measurement). Depends on task 26 (`--icon`, `--icon-sm`, their strokes and
`--icon-ink`). No Hub dependency: the component takes a state name, not an issue.

A list of issues is read by its left edge. Every row's first mark says where that issue stands, and
a reader scanning forty rows reads forty of them before any title. So the mark has three jobs, each
one a rule a test can hold:

1. **Nine states, ten shapes, told apart by shape alone.** No colour (05 §4.4 work rule 1: issue
   states and agent states never share colour; issue glyphs are neutral ink). A reader who cannot
   tell green from amber, or a Windows contrast theme that repaints everything in `CanvasText`
   (task 34), still reads every state.
2. **Nothing fills as work progresses** (02 C13's not-a-clone test fails a filling status icon).
   `started` does not become a half-disc; the component takes **no progress input at all**, so no
   later caller can pass one. Progress is a fact on the issue page, not a pie on a row.
3. **Priority is a notch, not a second icon** (work rule 2, D I4). A notch cut into the glyph's
   leading edge, deeper from `low` to `urgent`, `none` uncut, so a row carries one glyph. Urgent is
   ink like the rest: `--need` means only *someone is waiting on you*, and an urgent issue earns
   amber by entering NEEDS YOU, not by being urgent.

***Done, not proven*** (05 §3.3) is its own shape: an issue set `done` by hand, without its own
evidence at or after its land. The shape is the `done` check inside a **ring left open**: the work
is marked finished and the loop is not closed. It applies only with `state="done"`; on any other
state `notProven` is ignored, because it cannot mean anything there.

**The drawing, and where it came from.** Every path below was drawn for PanaceaApp from no
third-party source (`OD-11`; no Linear, Plane, Lucide or other icon set was opened). It follows the
sketch in our own `prototype/work-layer.html:393-401` (the `g-triage` … `g-cancel` and `g-dnp`
symbols), with three changes: the prototype has no `duplicate`, drawn here as two overlapping rings;
its `g-dnp` is a dashed ring, which would read as `backlog` with a tick, so the open ring replaces
it; and its `g-review` centre is a filled dot, which is a fill, so a bar replaces it. The 14px and
16px sets are **drawn separately**, each on its own half-pixel grid at its own stroke (1.25px and
1.5px, 02 C6), never one drawing scaled.

**The shape families carry meaning.** A ring is a commitment (`backlog` dashed, `todo`, `started`,
`done`, *done, not proven*, `cancelled`); a rounded square is code that exists in git (`in_review`,
`landed`); a diamond is not yet placed (`triage`); two rings are one thing twice (`duplicate`).
That grouping is a reading aid, not a rule a test holds; the test holds only that all ten differ.

**One correction to the scope, made here so nobody re-derives it:** the notch is a **clip path
with an even-odd hole**, not a mask. A mask needs a luminance fill (`white`/`black`), and
`designsystem.test.ts:639-646` (`namedColours`) exists to keep named colours out of the tree; a
clip path needs no colour at all. Either needs a per-instance id, which comes from React 19's
`useId()` with every character outside `[A-Za-z0-9_-]` removed, so `url(#…)` never has to escape.

**Files:**
- `desktop/src/components/glyphs/StateGlyph.tsx` — `IssueState`, `Priority`, `StateGlyph` (new)
- `desktop/src/components/glyphs/paths.ts` — the path data for both sizes, the notch geometry,
  the labels; headed with its clean-room provenance (new)
- `desktop/src/components/glyphs/glyphs.css` — owns the `.gl-*` prefix and no other (new)
- `desktop/src/components/glyphs/StateGlyph.test.tsx` — the eight tests (new)

No existing file changes. Nothing renders a `StateGlyph` until tasks 40 and 46.

#### Interfaces — complete: `StateGlyph.tsx`, the exported half

```ts
/** SW.12. The nine issue states (05 §3.3). Task 37's desktop issue-state type must be
 *  assignable to this one; its type test holds that, since this task lands first. */
export type IssueState =
  | "triage" | "backlog" | "todo" | "started" | "in_review"
  | "landed" | "done" | "cancelled" | "duplicate";

/** SW.12. The five priorities (L-13). "none" draws no notch. */
export type Priority = "none" | "urgent" | "high" | "medium" | "low";

export type GlyphSize = 14 | 16;

export function StateGlyph(props: {
  state: IssueState;
  priority?: Priority;     // default "none"
  notProven?: boolean;     // honoured only when state === "done"
  size?: GlyphSize;        // default 16; 14 inside rows and chips (02 C6)
}): JSX.Element;
```

There is deliberately no `className`, `style`, `color` or `progress` prop. The glyph is
`currentColor` and `glyphs.css` sets no colour, so it takes its ink from its row: the row sets
`color: var(--icon-ink)` at rest and `--ink` when current, in the row's own stylesheet (02 C6). A
caller that wants it coloured is breaking rule 1, and has no prop to do it with.

#### Interfaces — complete: `desktop/src/components/glyphs/paths.ts`

```ts
// desktop/src/components/glyphs/paths.ts
//
// Drawn for PanaceaApp, by hand, from no third-party source (OD-11). No icon set, Linear, Plane
// or any other product's artwork was opened while drawing these. The only reference is our own
// sketch in prototype/work-layer.html. Changing a path is a design change: the owner's eyes at
// task 39 accepted the set as a whole.
//
// Two sets, drawn separately (02 C6): 16px on a 16-unit box at a 1.5px stroke, body inside
// [2.5, 13.5]; 14px on a 14-unit box at a 1.25px stroke, body inside [2.25, 11.75]. The 14px set is
// NOT the 16px set scaled: its insets, dash counts and inner marks are placed on its own grid.
// Every path is a stroke (fill="none"). None fills as work progresses (02 C13).
import type { GlyphSize, IssueState } from "./StateGlyph";

/** One stroked sub-path; `dash` only where the dash IS the shape (backlog). */
export interface Stroke { d: string; dash?: string }

/** The drawn key: the nine states, plus done-not-proven as its own tenth shape. */
export type GlyphKey = IssueState | "done_not_proven";

const RING16 = "M2.5 8a5.5 5.5 0 1 0 11 0a5.5 5.5 0 1 0-11 0";
const SQUARE16 = "M5 2.5h6a2.5 2.5 0 0 1 2.5 2.5v6a2.5 2.5 0 0 1-2.5 2.5H5a2.5 2.5 0 0 1-2.5-2.5V5A2.5 2.5 0 0 1 5 2.5Z";
const CHECK16 = "M5.75 8.25 7.4 9.9l3.1-3.4";

const RING14 = "M2.25 7a4.75 4.75 0 1 0 9.5 0a4.75 4.75 0 1 0-9.5 0";
const SQUARE14 = "M4.5 2.25h5a2.25 2.25 0 0 1 2.25 2.25v5a2.25 2.25 0 0 1-2.25 2.25h-5a2.25 2.25 0 0 1-2.25-2.25v-5A2.25 2.25 0 0 1 4.5 2.25Z";
const CHECK14 = "M4.9 7.15 6.35 8.6l2.8-3.05";

export const STATE_PATHS: Record<GlyphSize, Record<GlyphKey, readonly Stroke[]>> = {
  16: {
    triage:          [{ d: "M8 2.5 13.5 8 8 13.5 2.5 8Z" }],
    backlog:         [{ d: RING16, dash: "1.5 1.96" }],              // ten dashes round 34.56
    todo:            [{ d: RING16 }],
    started:         [{ d: RING16 }, { d: "M7 5.75 10.25 8 7 10.25Z" }],
    in_review:       [{ d: SQUARE16 }, { d: "M5.75 8h4.5" }],
    landed:          [{ d: SQUARE16 }, { d: "M8 5.25v5.5M6 8.75l2 2 2-2" }],
    done:            [{ d: RING16 }, { d: CHECK16 }],
    done_not_proven: [{ d: "M8 2.5A5.5 5.5 0 1 0 13.5 8" }, { d: CHECK16 }],  // 270° ring, open top-right
    cancelled:       [{ d: RING16 }, { d: "M4.1 11.9 11.9 4.1" }],
    duplicate:       [{ d: "M2.5 8a4 4 0 1 0 8 0a4 4 0 1 0-8 0M5.5 8a4 4 0 1 0 8 0a4 4 0 1 0-8 0" }],
  },
  14: {
    triage:          [{ d: "M7 2.25 11.75 7 7 11.75 2.25 7Z" }],
    backlog:         [{ d: RING14, dash: "1.25 2.48" }],             // eight dashes round 29.85
    todo:            [{ d: RING14 }],
    started:         [{ d: RING14 }, { d: "M6.1 5 8.9 7 6.1 9Z" }],
    in_review:       [{ d: SQUARE14 }, { d: "M5 7h4" }],
    landed:          [{ d: SQUARE14 }, { d: "M7 4.5v4.75M5.25 7.5 7 9.25 8.75 7.5" }],
    done:            [{ d: RING14 }, { d: CHECK14 }],
    done_not_proven: [{ d: "M7 2.25A4.75 4.75 0 1 0 11.75 7" }, { d: CHECK14 }],
    cancelled:       [{ d: RING14 }, { d: "M3.65 10.35 10.35 3.65" }],
    duplicate:       [{ d: "M2.25 7a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0-7 0M4.75 7a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0-7 0" }],
  },
};
```

The same file continues with the notch and the labels (`Priority` joins the type import):

```ts
/** The stroke per size (02 C6). glyphs.css sets the same values from --icon-stroke and
 *  --icon-sm-stroke; this copy is what the tests compare against. */
export const STROKE: Record<GlyphSize, number> = { 16: 1.5, 14: 1.25 };

/** How far in from the box's left edge the notch's tip reaches, per priority: deeper from low to
 *  urgent, and absent for "none". The deepest tip stops short of every inner mark's leftmost
 *  stroke edge (16px: the second duplicate ring at 4.75; 14px: the same ring at 4.125), so no
 *  priority ever bites a check, an arrow or a bar. */
export const NOTCH_TIP: Record<GlyphSize, Record<Exclude<Priority, "none">, number>> = {
  16: { low: 2.7, medium: 3.4, high: 4.05, urgent: 4.65 },
  14: { low: 2.4, medium: 3.0, high: 3.55, urgent: 4.1 },
};

/** The clip: the whole box, minus a wedge whose tip sits on the centre line. The wedge's
 *  half-height at the box edge is 0.9 × its depth, so a deeper notch is also a taller gap in the
 *  leading stroke: `low` nicks the stroke, `medium` cuts through it, `high` and `urgent` bite into
 *  the shape. Drawn as two sub-paths under clip-rule="evenodd", so the wedge is a hole. */
export function notchClip(size: GlyphSize, priority: Exclude<Priority, "none">): string {
  const c = size / 2;
  const tip = NOTCH_TIP[size][priority];
  const h = Number((tip * 0.9).toFixed(3));
  return `M0 0H${size}V${size}H0Z M0 ${c - h} L${tip} ${c} L0 ${c + h}Z`;
}

export const STATE_LABEL: Record<GlyphKey, string> = {
  triage: "Triage",
  backlog: "Backlog",
  todo: "To do",
  started: "Started",
  in_review: "In review",
  landed: "Landed",
  done: "Done",
  done_not_proven: "Done, not proven",
  cancelled: "Cancelled",
  duplicate: "Duplicate",
};

/** "In review, high priority"; "Done, not proven"; "To do". */
export function glyphLabel(key: GlyphKey, priority: Priority): string {
  return priority === "none" ? STATE_LABEL[key] : `${STATE_LABEL[key]}, ${priority} priority`;
}
```

#### Interfaces — complete: `StateGlyph.tsx`, the body

```tsx
import { useId } from "react";
import type { JSX } from "react";
import { STATE_PATHS, notchClip, glyphLabel } from "./paths";
import type { GlyphKey } from "./paths";
import "./glyphs.css";

// (the three exported types and the signature above sit here)

export function StateGlyph(props: {
  state: IssueState;
  priority?: Priority;
  notProven?: boolean;
  size?: GlyphSize;
}): JSX.Element {
  const { state, priority = "none", notProven = false, size = 16 } = props;
  // The mark only means something on done; anywhere else it is dropped, not drawn.
  const key: GlyphKey = state === "done" && notProven ? "done_not_proven" : state;
  // useId's ":" or "«»" are legal in an id but would need escaping inside url(#…).
  const clipId = `gl-notch-${useId().replace(/[^A-Za-z0-9_-]/g, "")}`;
  const notched = priority !== "none";
  return (
    <svg
      className={`gl gl-${size}`}
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={glyphLabel(key, priority)}
      data-glyph={key}
      focusable="false"
    >
      {notched && (
        <defs>
          <clipPath id={clipId} clipPathUnits="userSpaceOnUse">
            <path className="gl-notch" d={notchClip(size, priority)} clipRule="evenodd" />
          </clipPath>
        </defs>
      )}
      <g
        className="gl-body"
        clipPath={notched ? `url(#${clipId})` : undefined}
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        {STATE_PATHS[size][key].map((s) => (
          <path key={s.d} d={s.d} strokeDasharray={s.dash} />
        ))}
      </g>
    </svg>
  );
}
```

The stroke width is not an attribute: `glyphs.css` sets it from the tokens, so a token change moves
every glyph and no component carries its own copy. `data-glyph` is the drawn key, which the tests
and task 33's sheet read; it carries no priority, so no stylesheet can select on urgency.

#### Interfaces — complete: `desktop/src/components/glyphs/glyphs.css`

```css
@import "../../styles/tokens.css";

/* Drawn glyphs (Phase W tasks 28 and 29). Owns .gl and .gl-* and no other prefix.
 *
 * The issue-state rules below set NO colour, on purpose: a state glyph is currentColor and takes
 * its ink from its row (05 §4.4 work rule 1: issue states and agent states never share colour).
 * Task 29 appends the agent and mark rules, under .gl-agent-* and .gl-mark-*, which are the only
 * selectors in this file allowed to name a status colour. StateGlyph.test.tsx holds that split. */

.gl {
  display: inline-block;
  flex: none;
  vertical-align: middle;
}

/* Two drawings, two strokes (02 C6). Never one drawing scaled to both boxes. */
.gl-16 {
  width: var(--icon);
  height: var(--icon);
}

.gl-16 .gl-body {
  stroke-width: var(--icon-stroke);
}

.gl-14 {
  width: var(--icon-sm);
  height: var(--icon-sm);
}

.gl-14 .gl-body {
  stroke-width: var(--icon-sm-stroke);
}
```

No `transition`, no animation: a state glyph changes by being re-drawn when the state changes, and
the row's own `--dur-flash` (task 26) is what tells the eye that it changed.

#### Tests to write first — `desktop/src/components/glyphs/StateGlyph.test.tsx`

```tsx
import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import type { JSX } from "react";
import glyphsCss from "./glyphs.css?raw";
import { StateGlyph } from "./StateGlyph";
import type { IssueState, Priority } from "./StateGlyph";
import { NOTCH_TIP, STATE_PATHS } from "./paths";
import { stripCssComments } from "../../test/fakes";

const STATES: IssueState[] = [
  "triage", "backlog", "todo", "started", "in_review", "landed", "done", "cancelled", "duplicate",
];
const NOTCHED = ["low", "medium", "high", "urgent"] as const;
const PRIORITIES: Priority[] = ["none", ...NOTCHED];
const SIZES = [14, 16] as const;

/** Renders one element and returns its one <svg>; more than one fails here. */
function draw(el: JSX.Element): SVGSVGElement {
  const svgs = render(el).container.querySelectorAll("svg");
  expect(svgs).toHaveLength(1);
  return svgs[0];
}
/** The body's geometry, serialised: every stroked sub-path with its dash. */
const body = (svg: Element): string =>
  [...svg.querySelectorAll(".gl-body path")]
    .map((p) => `${p.getAttribute("d")}|${p.getAttribute("stroke-dasharray") ?? ""}`)
    .join(";");
const notch = (svg: Element): string | null =>
  svg.querySelector(".gl-notch")?.getAttribute("d") ?? null;
/** The rules of a stylesheet as selector → declarations. Flat: glyphs.css has no nesting. */
const rules = (css: string): Array<[string, string]> =>
  [...stripCssComments(css).matchAll(/([^{}@;]+)\{([^{}]*)\}/g)].map((m) => [m[1].trim(), m[2]]);

describe("StateGlyph", () => {
  // Kills: two states sharing a shape, and not-proven drawn as plain done.
  it("draws nine distinct state shapes and a distinct not-proven", () => {
    for (const size of SIZES) {
      const shapes = STATES.map((state) => body(draw(<StateGlyph state={state} size={size} />)));
      shapes.push(body(draw(<StateGlyph state="done" notProven size={size} />)));
      expect(new Set(shapes).size, `size ${size}`).toBe(10);
    }
  });

  // Kills: a filling icon (02 C13). The kill is at typecheck: a progress prop makes the directive
  // unused, which tsc reports. At run time the drawing ignores anything extra.
  it("accepts no progress input", () => {
    // @ts-expect-error — StateGlyph has no progress prop, and must never have one
    const half = draw(<StateGlyph state="started" progress={0.5} />);
    expect(body(half)).toBe(body(draw(<StateGlyph state="started" />)));
  });

  // Kills: an issue state coloured like an agent state (05 §4.4 work rule 1).
  it("uses currentColor and no status colour", () => {
    const svg = draw(<StateGlyph state="in_review" priority="urgent" />);
    const classes = new Set([...svg.querySelectorAll("*"), svg].flatMap((e) => (e.getAttribute("class") ?? "").split(" ").filter(Boolean)));
    expect([...classes].sort()).toEqual(["gl", "gl-16", "gl-body", "gl-notch"]);
    expect(svg.querySelector(".gl-body")?.getAttribute("stroke")).toBe("currentColor");
    expect(svg.querySelector(".gl-body")?.getAttribute("fill")).toBe("none");
    const stateRules = rules(glyphsCss).filter(([sel]) => !/\.gl-(agent|mark)/.test(sel));
    expect(stateRules.length).toBeGreaterThan(0);
    for (const [sel, decls] of stateRules) {
      expect(decls, sel).not.toMatch(/(?:^|[\s;])(?:color|fill|stroke)\s*:/);
      expect(decls, sel).not.toMatch(/--(?:run|need|fail|drift|accent)/);
    }
  });

  // Kills: priority drawn as signal bars (a second glyph or extra body paths per priority).
  it("draws priority as one notched glyph", () => {
    for (const size of SIZES) {
      const drawn = PRIORITIES.map((p) => draw(<StateGlyph state="todo" priority={p} size={size} />));
      expect(new Set(drawn.map(body)).size).toBe(1);                  // identical body
      expect(notch(drawn[0])).toBeNull();                              // none is uncut
      const cuts = drawn.slice(1).map(notch);
      expect(new Set(cuts).size).toBe(4);                              // four leading edges
      const tips = NOTCHED.map((p) => NOTCH_TIP[size][p]);
      expect([...tips].sort((a, b) => a - b)).toEqual(tips);           // deeper toward urgent
      NOTCHED.forEach((p, i) => expect(cuts[i]).toContain(`L${NOTCH_TIP[size][p]} ${size / 2}`));
    }
  });

  // Kills: rule 2's breach, an urgent glyph that differs from any other by anything but its cut.
  it("never colours urgent", () => {
    const shape = (svg: Element) => [...svg.querySelectorAll("*")].map((e) => `${e.tagName}.${e.getAttribute("class") ?? ""}`);
    const urgent = draw(<StateGlyph state="started" priority="urgent" />);
    expect(shape(urgent)).toEqual(shape(draw(<StateGlyph state="started" priority="low" />)));
    expect(urgent.outerHTML.replace(/aria-label="[^"]*"/, "")).not.toMatch(/urgent/i);
    expect(stripCssComments(glyphsCss)).not.toMatch(/urgent|priority/i);
  });

  // Kills: the not-proven mark on a state it cannot mean.
  it("shows not-proven only on done", () => {
    for (const state of STATES.filter((s) => s !== "done")) {
      const marked = draw(<StateGlyph state={state} notProven />);
      const plain = draw(<StateGlyph state={state} />);
      expect(body(marked), state).toBe(body(plain));
      expect(marked.getAttribute("aria-label")).toBe(plain.getAttribute("aria-label"));
    }
    expect(draw(<StateGlyph state="landed" notProven />).getAttribute("data-glyph")).toBe("landed");
  });

  // Kills: an unlabelled image, and a label that drops the priority or the not-proven.
  it("names every glyph for assistive technology", () => {
    for (const state of STATES) {
      for (const priority of PRIORITIES) {
        const svg = draw(<StateGlyph state={state} priority={priority} />);
        expect(svg.getAttribute("role")).toBe("img");
        expect(svg.getAttribute("aria-label")?.length ?? 0).toBeGreaterThan(0);
      }
    }
    expect(draw(<StateGlyph state="in_review" priority="high" />).getAttribute("aria-label"))
      .toBe("In review, high priority");
    expect(draw(<StateGlyph state="done" notProven />).getAttribute("aria-label")).toBe("Done, not proven");
    expect(draw(<StateGlyph state="todo" />).getAttribute("aria-label")).toBe("To do");
  });

  // Kills: one drawing scaled to both boxes (a shared path set, or a 16-unit viewBox at 14px).
  it("draws 14 and 16 separately", () => {
    for (const key of Object.keys(STATE_PATHS[16]) as Array<keyof (typeof STATE_PATHS)[16]>) {
      const d16 = STATE_PATHS[16][key].map((s) => s.d).join(" ");
      const d14 = STATE_PATHS[14][key].map((s) => s.d).join(" ");
      expect(d14, key).not.toBe(d16);
    }
    expect(draw(<StateGlyph state="todo" size={14} />).getAttribute("viewBox")).toBe("0 0 14 14");
    expect(draw(<StateGlyph state="todo" />).getAttribute("viewBox")).toBe("0 0 16 16");
    const css = new Map(rules(glyphsCss));
    expect(css.get(".gl-16 .gl-body")).toContain("stroke-width: var(--icon-stroke)");
    expect(css.get(".gl-14 .gl-body")).toContain("stroke-width: var(--icon-sm-stroke)");
    expect(STATE_PATHS[14].backlog[0].dash).not.toBe(STATE_PATHS[16].backlog[0].dash);
  });
});
```

Eight tests, one per scope mutant. The progress test's kill is at `npm --prefix desktop run
typecheck`, not at `vitest` (Vitest strips types without checking them), which is why the
acceptance runs both.

#### Steps

1. Write `StateGlyph.test.tsx`. Run `npm --prefix desktop test -- glyphs`: it fails to import.
2. Write `paths.ts`, `glyphs.css` and `StateGlyph.tsx` as above. Re-run: all eight pass.
3. Mutate by hand and watch the named test go red, then revert: give `todo` the `done` ring (copy
   `RING16` with `CHECK16`); add a `progress?: number` prop; add `.gl-16 { color: var(--need) }`;
   draw `high` as a second `<svg>` of bars; drop the `state === "done" &&` guard; delete the
   `aria-label`; make the 14px set reuse the 16px strings with `viewBox="0 0 16 16"`.
4. Render the ten glyphs at both sizes into a scratch page (task 33 builds the real sheet) and look
   at them at 100 % and 200 % zoom. This is the implementer checking the paths are not broken, not
   an acceptance of the look; the owner's eyes are task 39's.
5. `npm --prefix desktop run typecheck`, the full desktop suite (the design-system audit walks
   the new `.css` and `.tsx` files: token import first, no hex, no named colour), then commit.

#### Acceptance

```bash
npm --prefix desktop test -- glyphs
# expect: StateGlyph.test.tsx, 8 passed

npm --prefix desktop run typecheck
# expect: exit 0 (the @ts-expect-error is used, so it raises nothing)

npm --prefix desktop test -- designsystem
# expect: every audit test passes with glyphs.css and the two .ts/.tsx files in the tree
```

#### What this task does not claim

- **Not that the glyphs read well** at 125 % and 150 %, or on `paper`: task 33 renders them there,
  and the owner's eyes judge at task 39. Step 4 is a check for broken paths only.
- **Not the forced-colors rendering** (task 34). The glyphs are `currentColor` SVG, which a
  Windows contrast theme repaints, but nobody has looked.
- **Not any surface using them.** Tasks 40 (the Work lens) and 46 (the issue page) are the first.
- **Not the agent shapes, the marks or the licence check**: task 29, which appends to `glyphs.css`.
- **Not that no icon set resembles these.** They were drawn from our own prototype without opening
  another set; that they are original is a claim about process, not a search of every icon family.
- Design: 05 §3.3, §4.4 (C6, work rules 1 and 2); 02 C6, C13. Parity: gives **L-8**'s states and
  **L-13**'s five priorities their shapes; it closes neither alone (tasks 40 and 46 show them).

**Interfaces:**
- Produces (seam **SW.12**, first member):
  - `IssueState` (nine), `Priority` (five), `GlyphSize` (`14 | 16`)
  - `StateGlyph({ state, priority?, notProven?, size? }): JSX.Element` — `role="img"`, an
    `aria-label`, `data-glyph` = the drawn key
  - `paths.ts`: `Stroke`, `GlyphKey`, `STATE_PATHS`, `STROKE`, `NOTCH_TIP`, `notchClip`,
    `STATE_LABEL`, `glyphLabel`
  - `glyphs.css`, owning `.gl` and `.gl-*`; task 29 appends `.gl-agent-*` and `.gl-mark-*`
- Consumes: task 26's `--icon`, `--icon-stroke`, `--icon-sm`, `--icon-sm-stroke`; React 19's
  `useId` (`desktop/package.json:20`, `react ^19.1.0`, verified)
- Amends: nothing in the Hub. Task 37's desktop issue-state type must be assignable to
  `IssueState`; that type test is task 37's.
