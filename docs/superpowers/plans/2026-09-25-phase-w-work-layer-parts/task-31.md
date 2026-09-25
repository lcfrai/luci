### Task 31: `chrome.dim` and one `ViewHeader` with the route badge (02 D2 step 2)

Stream **WD1**, defines seam **SW.13**, about 3 hours (an estimate, not a measurement). Depends on
task 26 (`--t-head`, `--t-fact`, `--s-*`, `--row-h`). No Hub, no Work table, no route computation:
the header shows the route it is given and nothing else.

Two things land together because they are one idea, 02 C5: **the chrome recedes and the work is
the brightest thing on screen.** `chrome.dim` turns the rail and sub-rail down to `--ink-2` labels
and `--ink-3` icons, and marks the current item with a 2px `--accent` tick on its leading edge.
`ViewHeader` is the one header every new surface uses: title, scope chip, route badge, controls on
the right. A second header pattern anywhere is a defect (AGENTIC-UX §9 item 8, *repeated
breadcrumb chrome*), and an audit rule here makes it one.

**The route badge is the visible half of `OD-19`.** A project on the AU path reads `AU`; the
default reads `Global`. When the route is not known the badge is **hidden**, never defaulted: a
header that guesses `Global` tells a person their Protected data went to the direct API when
nobody knows where it went. `null` means unknown, and the type forces every caller to say which.

**Four corrections to the scope, verified against the tree at `d1262e2`:**

1. **There is no Develop title row to move.** `DevelopLens.tsx` renders no title (its only
   `className` hits are layout and `cols-empty`), and `topbar/TopBar.tsx:22-43` renders the layout
   toggle, two disabled buttons, *New session* and the picker, with no title. The only built lens
   title is `LensPlaceholder.tsx:15`'s `<h2>`. This task moves that one. Giving Develop a header
   row is a layout change to a built lens and goes to the owner at task 39 (see *does not claim*).
2. **The rail's current item is `aria-selected="true"`, the sub-rail's is `aria-current="page"`.**
   `BranchRow.tsx:41`, `ProjectRow.tsx:34` and `SessionRow.tsx:34` set `aria-selected`;
   `SubRail.tsx:45` sets `aria-current`. The tick keys on each file's own attribute. Both rules
   today paint `--accent-tint` plus an inset `--accent-ring` (`rail.css:33-37`,
   `subrail.css:54-58`); the new look drops both, because a tinted, ringed row is the chrome
   shouting, and keeps the tick alone.
3. **`.rail-add` is accent on a resting row.** `rail.css:138` colours the hover-revealed `+` in
   `--accent-hi`. That is accent outside the current item, which the audit below forbids. It
   moves to `--ink-2`, `--ink` on hover and focus.
4. **`app.css` gains a heading reset, not a colour.** `:where(h1, h2, h3) { margin: 0; font:
   inherit; }` so no heading carries the browser's default size and `.vh-title` is the one place
   header type is set. It declares no colour, so `tokens.test.ts:85-90` (token import first, no
   colour of its own) still holds. The heading's weight and size come only from `--t-head`.

**Why a pseudo-element tick, not an inset shadow.** `box-shadow: inset 2px 0 0` paints the left
edge, which is the *trailing* edge in a right-to-left layout. A `::before` placed with
`inset-inline-start: 0` is the leading edge in both. It also gives task 34 one element to repaint
under `forced-colors`, where Chromium drops every `box-shadow`.

**Files:**
- `desktop/src/components/common/ViewHeader.tsx` — the component, `Route` (new)
- `desktop/src/components/common/viewheader.css` — owns `.vh-*` (new; the contract §5.5 rule: one
  component, one prefixed stylesheet, `@import "../../styles/tokens.css";` first)
- `desktop/src/components/common/ViewHeader.test.tsx` (new)
- `desktop/src/components/lens/LensPlaceholder.tsx` ⚑ (20 lines; the `<h2>` at `:15`) — the title
  moves onto `ViewHeader` (modified)
- `desktop/src/components/shell.css` ⚑ (`.lens-placeholder h2` at `:44-47`) — that rule is deleted
  (modified)
- `desktop/src/styles/app.css` ⚑ (82 lines; `code, kbd, pre, samp` at `:39-44`) — the heading reset
  after `:44` (modified)
- `desktop/src/components/rail/rail.css` ⚑ (147 lines; `.rail-row` `:17-29`, current `:33-37`,
  `.rail-chev` `:42-46`, `.rail-add` `:132-147`) — `chrome.dim`, the tick, the old look (modified)
- `desktop/src/components/subrail/subrail.css` ⚑ (62 lines; `.sub-back` `:18-26`, `.sub-row`
  `:36-50`, current `:54-58`, `.sub-glyph` `:59-62`) — the same (modified; not in the skeleton's
  file map, which 02 C5 needs)
- `desktop/src/styles/designsystem.test.ts` ⚑ (886 lines; `REAL: Tree` at `:73`) — four rules
  (modified)
- `desktop/src/components/rail/Rail.test.tsx` ⚑ (181 lines) — one render test (modified)

`AppShell.tsx` is untouched: it passes `lens`, `phase` and `what` to `LensPlaceholder` today
(`:30-62`) and still does; the placeholder passes `route={null}`, because no lens knows its project's
route before task 24.

- `desktop/src/components/AppShell.test.tsx` ⚑ (`:119` asserts `getByRole("heading", { name:
  "merge" })`) — the name becomes `"Merge"` (modified). The deleted `text-transform: capitalize`
  (`shell.css:46`) was the only thing that made the lowercase id read as a title; the title now
  comes from `SubRail.tsx:9-16`'s `LENSES` labels, the one place a lens is named.

#### Interfaces — complete: `ViewHeader.tsx` (seam SW.13)

```tsx
// Seam SW.13 (task 31; 02 C5, 05 §4.4 C5 and §4.1). The one view header: title, scope chip,
// route badge, controls on the right. No other file renders a view title; the designsystem
// audit "has one view header" holds that by construction.
import type { JSX, ReactNode } from "react";
import "./viewheader.css";

/** OD-19's two inference paths. At a call site `null` means *unknown*, never a default. */
export type Route = "global" | "au";

const ROUTE_LABEL: Readonly<Record<Route, string>> = { global: "Global", au: "AU" };

const ROUTE_TITLE: Readonly<Record<Route, string>> = {
  global: "Inference goes through the org gateway to the direct Anthropic API.",
  au: "Inference goes through the org gateway to Amazon Bedrock in Sydney.",
};

export function ViewHeader(props: {
  title: string;
  route: Route | null;
  badge?: ReactNode;
  actions?: ReactNode;
}): JSX.Element {
  const { title, route, badge, actions } = props;
  return (
    <header className="vh">
      <h1 className="vh-title">{title}</h1>
      {badge === undefined ? null : <span className="vh-scope">{badge}</span>}
      {route === null ? null : (
        <span
          className="vh-route"
          data-route={route}
          aria-label={`Inference route: ${ROUTE_LABEL[route]}`}
          title={ROUTE_TITLE[route]}
        >
          {ROUTE_LABEL[route]}
        </span>
      )}
      {actions === undefined ? null : <div className="vh-actions">{actions}</div>}
    </header>
  );
}
```

`route` is required, not optional, so a caller that forgets it fails the typecheck instead of
rendering a header with no badge that looks like a decision. The two badge words carry no colour
of their own: the route is not a status, and `AU` in a status hue would read as a warning. The
`title` text names the gateway because both paths go through it (`OD-19`); it names no model and
no key. `aria-label` contains the visible word, so the accessible name matches what is seen.

#### Interfaces — complete: `viewheader.css`

```css
@import "../../styles/tokens.css";

/* The one view header (task 31, seam SW.13). Owns .vh-*. 02 C5: title, scope, route, controls. */
.vh {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  min-block-size: var(--row-h);
  padding: var(--s-2) var(--s-4);
  box-shadow: inset 0 -1px 0 var(--line);
}
.vh-title {
  min-inline-size: 0;
  overflow: hidden;
  color: var(--ink);
  font-family: var(--sans);
  font-size: var(--t-head);
  font-weight: var(--t-head-weight);
  line-height: var(--t-head-lh);
  letter-spacing: var(--t-head-track);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.vh-scope {
  display: inline-flex;
  align-items: center;
  gap: var(--s-1);
  min-inline-size: 0;
  color: var(--ink-2);
  font-size: var(--t-ui);
  line-height: var(--t-ui-lh);
}
.vh-route {
  flex: none;
  padding: 0 var(--s-1);
  border-radius: var(--r-chip);
  box-shadow: inset 0 0 0 1px var(--line-strong);
  color: var(--ink-2);
  font-family: var(--mono);
  font-size: var(--t-fact);
  line-height: var(--t-fact-lh);
}
.vh-actions {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  margin-inline-start: auto;
}
```

`.vh-title` sets its own family although the body is already sans: `voice.split` (task 30) puts
mono on machine surfaces, and a header inside one must not inherit it. The route chip's ring is
`--line-strong` because at `--line` (7.5 % ink) a 16px-tall chip does not read as a chip.

#### Interfaces — complete: `LensPlaceholder.tsx`, `shell.css`, `app.css`

`LensPlaceholder.tsx`, whole file after the change (the section's label moves from
`aria-labelledby` to `aria-label`, because `ViewHeader`'s props carry no id and SW.13 is not
widened for one caller):

```tsx
// Contract §6.4-adjacent (§6.3's table): the honest empty state. What the lens will do, and
// which phase brings it. §7.7: every state is designed. Task 31: the title is ViewHeader's.
import type { JSX } from "react";
import type { Lens } from "../../store/types";
import { ViewHeader } from "../common/ViewHeader";
import { LENSES } from "../subrail/SubRail";

export function LensPlaceholder(props: {
  lens: Lens;
  phase: number;
  what: string;
}): JSX.Element {
  const { lens, phase, what } = props;
  const title = LENSES.find((entry) => entry.id === lens)?.label ?? lens;
  return (
    <section className="lens-placeholder" aria-label={title}>
      {/* No lens knows its project's route before task 24, so the badge is hidden, not guessed. */}
      <ViewHeader title={title} route={null} />
      <div className="lens-placeholder-body">
        <p>{what}</p>
        <p>Arrives in phase {phase}.</p>
      </div>
    </section>
  );
}
```

`shell.css`: `.lens-placeholder` (`:34-43`) becomes the column that holds the header and a
centred body, and `.lens-placeholder h2` (`:44-47`) is deleted:

```css
.lens-placeholder {
  display: grid;
  grid-template-rows: auto 1fr;
  block-size: 100%;
}
.lens-placeholder-body {
  display: grid;
  align-content: center;
  justify-items: center;
  gap: var(--s-2);
  padding: var(--s-5);
  color: var(--ink-2);
  text-align: center;
}
```

`app.css`, inserted after `code, kbd, pre, samp { … }` (`:39-44`):

```css
/* Task 31: no heading carries the browser's size or margin. The one view header sets its type
 * from --t-head; a heading anywhere else inherits the text around it. No colour here. */
:where(h1, h2, h3) {
  margin: 0;
  font: inherit;
}
```

`:where()` keeps the reset at zero specificity, so `.vh-title` wins without a fight.

#### Interfaces — complete: `chrome.dim` in `rail.css` and `subrail.css`

The labels are already `--ink-2` (`rail.css:24`, `:76`; `subrail.css:45`) and the rail's chevron
already `--ink-3` (`rail.css:44`). What changes: the current item, the sub-rail's back link,
branch name and glyph, and the `+`. In `rail.css`:

```css
.rail-row {
  position: relative; /* task 31: the tick's containing block. Added to the rule at :17-29. */
}
.rail-row[aria-selected="true"] {  /* replaces :33-37 */
  color: var(--ink);
}
.rail-row[aria-selected="true"]::before {
  content: "";
  position: absolute;
  inset-block: var(--s-1);
  inset-inline-start: 0;
  inline-size: 2px;
  background: var(--accent);
}
.rail-add {  /* :138's --accent-hi becomes */
  color: var(--ink-2);
}
.rail-add:hover,
.rail-add:focus-visible {
  color: var(--ink);
}
```

In `subrail.css`:

```css
.sub-back {  /* :22's --ink-3: a label, so --ink-2 */
  color: var(--ink-2);
}
.sub-branch {  /* :28's --ink: the chrome's own title, dimmed so the work stays brightest */
  color: var(--ink-2);
}
.sub-row {
  position: relative; /* added to the rule at :36-50 */
}
.sub-row[aria-current="page"] {  /* replaces :54-58 */
  color: var(--ink);
}
.sub-row[aria-current="page"]::before {
  content: "";
  position: absolute;
  inset-block: var(--s-1);
  inset-inline-start: 0;
  inline-size: 2px;
  background: var(--accent);
}
.sub-glyph {  /* added to :59-62: an icon, so --ink-3 */
  color: var(--ink-3);
}
.sub-row[aria-current="page"] .sub-glyph {
  color: var(--ink-2);
}
```

**The old look**, appended at the end of each file, for task 32's `flag.compare`. Nothing sets
`data-look` until task 32, so these rules are dead code until then, and the default is the new
look. `:root[data-look="old"]` adds (0,2,0) to each selector, so each old rule outranks its new
counterpart wherever it sits in the file.

```css
/* rail.css: the Phase 2 look, restored by flag.compare (task 32). Delete after task 39. */
:root[data-look="old"] .rail-row[aria-selected="true"] {
  background: var(--accent-tint);
  box-shadow: inset 0 0 0 1px var(--accent-ring);
}
:root[data-look="old"] .rail-row[aria-selected="true"]::before {
  content: none;
}
:root[data-look="old"] .rail-add {
  color: var(--accent-hi);
}
```

```css
/* subrail.css: the Phase 2 look, restored by flag.compare (task 32). Delete after task 39. */
:root[data-look="old"] .sub-back {
  color: var(--ink-3);
}
:root[data-look="old"] .sub-branch,
:root[data-look="old"] .sub-row[aria-current="page"] .sub-glyph {
  color: var(--ink);
}
:root[data-look="old"] .sub-glyph {
  color: inherit;
}
:root[data-look="old"] .sub-row[aria-current="page"] {
  background: var(--accent-tint);
  box-shadow: inset 0 0 0 1px var(--accent-ring);
}
:root[data-look="old"] .sub-row[aria-current="page"]::before {
  content: none;
}
```

The old block restores **every** declaration this task changed and nothing else, so a flip
compares exactly `chrome.dim` against Phase 2. The heading reset and `ViewHeader` have no old
twin: the Phase 2 placeholder's `<h2>` was the only heading, and a flag that brought back a
second header pattern would fail this task's own audit.

#### Tests to write first — `desktop/src/components/common/ViewHeader.test.tsx`

jsdom computes no style from an imported stylesheet, so the two CSS claims read the stylesheet's
bytes through Vite's `?raw`, the way `StatusDot.test.tsx:7` reads `common.css`.

```tsx
import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { stripCssComments } from "../../test/fakes";
import { ViewHeader } from "./ViewHeader";
import viewheaderCss from "./viewheader.css?raw";

/** The declarations of one flat rule in viewheader.css. */
function rule(selector: string): string {
  const css = stripCssComments(viewheaderCss);
  const at = css.indexOf(`${selector} {`);
  if (at < 0) throw new Error(`no rule ${selector}`);
  return css.slice(at, css.indexOf("}", at));
}

describe("ViewHeader", () => {
  it("renders the route badge in mono", () => {
    // Kills: the badge dropped, or set in the sans at --t-ui like the scope chip beside it.
    render(<ViewHeader title="Work" route="au" />);
    const badge = screen.getByLabelText("Inference route: AU");
    expect(badge.textContent).toBe("AU");
    expect(badge.className).toBe("vh-route");
    expect(rule(".vh-route")).toContain("font-family: var(--mono)");
    expect(rule(".vh-route")).toContain("font-size: var(--t-fact)");
  });

  it("reads Global for the default route", () => {
    // Kills: a swapped label map, which would show AU on a Global project.
    render(<ViewHeader title="Work" route="global" />);
    expect(screen.getByLabelText("Inference route: Global").textContent).toBe("Global");
    expect(screen.queryByText("AU")).toBeNull();
  });

  it("hides the badge when the route is unknown", () => {
    // Kills: `route ?? "global"`, a header that guesses the default (OD-19 made visible).
    const { container } = render(<ViewHeader title="Work" route={null} />);
    expect(container.querySelector(".vh-route")).toBeNull();
    expect(screen.queryByText("Global")).toBeNull();
    expect(screen.queryByText("AU")).toBeNull();
  });

  it("puts actions after the title in one row", () => {
    // Kills: a second layout (actions in a row of their own, or a header per slot).
    const { container } = render(
      <ViewHeader
        title="Work"
        route="global"
        badge={<span>EP · Resumable uploads</span>}
        actions={<button type="button">Filter</button>}
      />,
    );
    const headers = container.querySelectorAll("header");
    expect(headers).toHaveLength(1);
    const kids = [...headers[0].children].map((el) => el.className);
    expect(kids).toEqual(["vh-title", "vh-scope", "vh-route", "vh-actions"]);
    expect(within(headers[0]).getByRole("button", { name: "Filter" })).not.toBeNull();
    expect(rule(".vh")).toContain("display: flex");
    expect(rule(".vh-actions")).toContain("margin-inline-start: auto");
  });

  it("renders the title as the one level-1 heading, in the head step", () => {
    // Kills: a title at h2 in one view and h1 in another, and a title that skips --t-head.
    render(<ViewHeader title="Work" route={null} />);
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1, name: "Work" })).not.toBeNull();
    expect(rule(".vh-title")).toContain("font-size: var(--t-head)");
    expect(rule(".vh-title")).toContain("font-weight: var(--t-head-weight)");
  });

  it("renders no empty slot when badge and actions are absent", () => {
    // Kills: an empty .vh-actions that still takes the gap and pushes the badge off its place.
    const { container } = render(<ViewHeader title="Work" route="au" />);
    expect(container.querySelector(".vh-scope")).toBeNull();
    expect(container.querySelector(".vh-actions")).toBeNull();
  });
});
```

#### Tests to write first — `desktop/src/styles/designsystem.test.ts`

Four predicates in the file's own shape (`(t: Tree) => string[]`, each offender
`` `${file}  ${selector}` ``), inserted before the canaries at `:725`, so every one can be handed
the fault it exists to catch. Part 1, the header rule and the chrome inks:

```ts
/* ── task 31: one view header, and chrome.dim (02 C5) ───────────────────────────────── */
const VIEW_HEADER = "src/components/common/ViewHeader.tsx";
const VIEW_HEADER_CSS = "src/components/common/viewheader.css";
const RAIL_CSS = "src/components/rail/rail.css";
const SUBRAIL_CSS = "src/components/subrail/subrail.css";
const CHROME = [RAIL_CSS, SUBRAIL_CSS] as const;
const OLD_LOOK = ':root[data-look="old"]';
const CURRENT_ITEM = /\[aria-selected="true"\]|\[aria-current="page"\]/;
/** app.css's zero-specificity reset names h1 and is not a header. */
const HEADING_RESET = ":where(h1, h2, h3)";

const last = <T,>(list: readonly T[]): T | undefined => list[list.length - 1];

/** The top-level rules (inside no at-rule) whose selector list names `selector`. */
function topRules(css: string, selector: string): Rule[] {
  return rulesOf(css).filter(
    (r) => r.within.length === 0 && r.selector.split(",").map((s) => s.trim()).includes(selector),
  );
}

/** The value `property` ends with across those rules, as the cascade would resolve it. */
function finalValue(css: string, selector: string, property: string): string | undefined {
  const decls = topRules(css, selector).flatMap((r) => r.decls.filter((d) => d.property === property));
  return last(decls)?.value;
}

/** A second header pattern anywhere: an h1, a level-1 heading role, a *-view-title class. */
function secondHeaders(t: Tree): string[] {
  const out: string[] = [];
  for (const f of t.ts) {
    if (f === VIEW_HEADER) continue;
    const src = t.read(f);
    if (/<h1[\s>]/.test(src)) out.push(`${f}  <h1>`);
    if (/aria-level=\{?["']?1\b/.test(src)) out.push(`${f}  aria-level 1`);
    if (/[\w-]*view-title\b/.test(src)) out.push(`${f}  *-view-title`);
  }
  for (const f of t.css) {
    if (f === VIEW_HEADER_CSS) continue;
    for (const r of rulesOf(t.read(f))) {
      if (r.selector === HEADING_RESET) continue;
      if (/(?:^|[\s,>+~(])h1\b|view-title\b/.test(r.selector)) out.push(`${f}  ${r.selector}`);
    }
  }
  return out;
}

/** 02 C5: labels at --ink-2, icons at --ink-3, the current item at --ink. */
const CHROME_INK: ReadonlyArray<{ file: string; selector: string; ink: string }> = [
  { file: RAIL_CSS, selector: ".rail-row", ink: "var(--ink-2)" },
  { file: RAIL_CSS, selector: ".rail-machine", ink: "var(--ink-2)" },
  { file: RAIL_CSS, selector: ".rail-footer", ink: "var(--ink-2)" },
  { file: RAIL_CSS, selector: ".rail-add", ink: "var(--ink-2)" },
  { file: RAIL_CSS, selector: ".rail-chev", ink: "var(--ink-3)" },
  { file: SUBRAIL_CSS, selector: ".sub-row", ink: "var(--ink-2)" },
  { file: SUBRAIL_CSS, selector: ".sub-back", ink: "var(--ink-2)" },
  { file: SUBRAIL_CSS, selector: ".sub-branch", ink: "var(--ink-2)" },
  { file: SUBRAIL_CSS, selector: ".sub-glyph", ink: "var(--ink-3)" },
  { file: RAIL_CSS, selector: '.rail-row[aria-selected="true"]', ink: "var(--ink)" },
  { file: SUBRAIL_CSS, selector: '.sub-row[aria-current="page"]', ink: "var(--ink)" },
];

function chromeInk(t: Tree): string[] {
  return CHROME_INK.filter(({ file, selector, ink }) => finalValue(t.read(file), selector, "color") !== ink)
    .map(({ file, selector }) => `${file}  ${selector}`);
}
```

`finalValue` takes the **last** matching declaration because that is what the cascade does
between rules of equal specificity; a rule appended later that brightens a label is what the
canary plants, and a predicate that read the first one would pass it.
