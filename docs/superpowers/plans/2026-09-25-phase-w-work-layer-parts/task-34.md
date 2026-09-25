### Task 34: the `forced-colors` layer; the reduced-motion running shape and static tick

Stream **WD1**, about 2 hours (an estimate, not a measurement). Depends on task 29 (the running
shape, `.gl-agent-live`, `.dot-run`) and task 33 (the `data-motion="reduce"` hook and the
forced-colours screen this layer is looked at through). No Hub, no wire shape.

Two floors land together because both are about a signal surviving when the usual carrier is
gone (05 §4.4 work rule 4 and C7, revision 2; 07 D C4, D I8):

- **Windows contrast themes.** Under `forced-colors: active` Chromium forces every `box-shadow`
  to `none` and repaints backgrounds in `Canvas`. This design system draws elevation, the focused
  pane, the selected palette row and every panel edge as a `box-shadow`, and the rail's status
  dots as a background. In a contrast theme today they all vanish. A `@media (forced-colors:
  active)` layer redraws each as a `border` or `outline` in system colours.
- **Reduced motion.** With the breath stopped, running keeps task 29's own shape (a disc in an
  open ring), so shape alone separates running, idle, exited and failed. The state flash becomes
  a **static 2px leading tick in `--ink`** that stays while the row is unseen. And every
  reduced-motion rule is written twice, as the media query and as the `:root[data-motion="reduce"]`
  twin that task 33's set writes, so a reviewer can see the reduced look without changing Windows.

**Five corrections to the scope, verified against the tree at `b8bd64e` and the written parts:**

1. **The unseen attribute is `data-changed="unseen"`,** not the scope's `data-unseen="true"`.
   Task 41's part is written and sets `data-changed="unseen"` on every held row, saying task 34
   draws it (`task-41.md:53-56`). One attribute, the one already in a plan: this task keys on it.
2. **The tick is in `tokens.css`, not `glyphs.css`.** It marks a row, not a glyph, and
   `glyphs.css` owns `.gl-*` and no other prefix (task 28's file map). The only global
   reduced-motion rules in the tree are the floor at `tokens.css:173-183`; the tick is the floor's
   second rule and sits beside it. `glyphs.css` gains only the `.gl-agent-live` twin.
3. **The ring rule covers every `box-shadow`, not only `--e-*`, `--thread` and `--accent-ring`.**
   Every one of the tree's 36 non-token `box-shadow` declarations is a ring, an elevation or a
   one-pixel edge (`rail.css:10`, `subrail.css:9`, `topbar.css:10` and nine more are `--line`
   edges; `common.css:56`, `:101` and `:121` are the dots' `currentColor` rings). Chromium drops all
   of them. A rule that checked three token families would pass a contrast theme with no panel
   edges and no hollow *starting* dot. Two exclusions: a value of `none`, and a selector under
   `:root[data-look="old"]` (task 30's convention; task 39 deletes the old look).
4. **The dots need an opt-out, not an outline.** `.dot`'s fill is `background: currentColor`
   (`common.css:37-47`), which forced colours repaint as `Canvas`: every rail dot would disappear,
   running or not. `.dot` takes `forced-color-adjust: none` and `color: CanvasText`, so the fill,
   the ring and task 29's `.dot-run` outline all draw in the one system ink.
5. **`lease.ring` does not exist yet.** 02 C9 puts it on the team strip's chip, which no Phase W
   task before this one builds. The ring audit fails it the day its stylesheet lands without a
   forced-colours line, which is the point of an audit over the tree rather than a list.

**What stays as it is.** The focus ring is already an `outline` (`tokens.css:187-191`) and forced
colours keep outlines, so it is not touched. Glyphs are SVG in `currentColor` (tasks 28, 29), which
forced colours map to `CanvasText`, so their shapes carry the meaning on their own.
`prototype/work-layer.html:379-386` sketches forced-colours rules; they are rewritten here against
the real class names, not copied.

**Files:**
- `desktop/src/styles/forced-colors.css` — the layer (new)
- `desktop/src/main.tsx` ⚑ (13 lines; `import App` at `:7`) — imports the layer after `./App`
  (modified; the scope named `app.css`, see below)
- `desktop/src/styles/tokens.css` ⚑ (191 lines; the floor at `:173-183`) — the floor's twin, the
  tick and its twin, after `:183` (modified)
- `desktop/src/components/glyphs/glyphs.css` ⚑ (task 29) — the `.gl-agent-live` twin (modified)
- `desktop/src/components/common/common.css` ⚑ (179 lines; the reduced block at `:119-123`) —
  its twin (modified)
- `desktop/src/components/palette/palette.css` ⚑ (the reduced block at `:131-135`) — its twin
  (modified)
- `desktop/src/components/devtools/devtools.css`, `stress.css` ⚑ (tasks 32, 33) — each gains its
  own forced-colours lines, so no production file names a dev class (modified)
- `desktop/src/styles/designsystem.test.ts` ⚑ — four rules and their canaries (modified)
- `desktop/src/styles/tokens.test.ts` ⚑ (91 lines) — the tick (modified)
- `desktop/src/components/glyphs/AgentGlyph.test.tsx` ⚑ (task 29) — one test (modified)

**Why the layer is imported last, from `main.tsx`, not from `app.css`.** A forced-colours rule
and a component rule of equal specificity resolve by source order. `main.tsx:1-4` imports
`app.css` first so the token layer sits **under** every component sheet; a layer imported there
would lose every tie (`.top-btn { border: 0 }` would erase its forced border). So `main.tsx`
imports `./styles/forced-colors.css` **after** `import App from "./App";` (`:7`), which Vite emits
after every component sheet. `app.css` is not edited.

#### Interfaces — complete: `desktop/src/styles/forced-colors.css`

```css
/* desktop/src/styles/forced-colors.css
 *
 * Windows contrast themes (Phase W task 34; 05 §4.4 work rule 4; 07 D I8). Under
 * forced-colors: active, Chromium forces every box-shadow to none and repaints backgrounds as
 * Canvas. Here every ring, elevation and edge the tree draws as a shadow is redrawn as a border or
 * outline in a system colour. designsystem.test.ts fails any box-shadow selector with no line here.
 *
 * Imported LAST, from main.tsx after ./App, so it wins every tie with a component sheet. Borders,
 * not outlines, for rings: the focus ring (tokens.css :focus-visible) is an outline and must stay
 * one. The dev toolbar's own lines live in devtools.css and stress.css. */
@import "./tokens.css";

@media (forced-colors: active) {
  /* Elevation and control rings. */
  .pop, .toast, .toast-undo, .pane, .pane-attach--corner, .pane-exit-restart, .pane-empty-go,
  .cv-tile, .cv-tile[data-cv-dragging="resize"], .menu-rename-input, .pal, .pal-hint,
  .rail-kbd, .top-seg, .top-btn, .vh-route {
    border: 1px solid CanvasText;
  }

  /* Focus within a pane or tile is a selection: 2px Highlight, drawn inside the box. */
  .pane--focused, .cv-tile:focus-within, .cv-tile[data-cv-dragging="resize"]:focus-within {
    outline: 2px solid Highlight;
    outline-offset: -2px;
  }

  /* One-pixel panel edges, each on its own side. */
  .rail, .sub {
    border-inline-end: 1px solid CanvasText;
  }
  .rail-machine, .sub-head, .top, .vh, .pal-input, .pane-head {
    border-block-end: 1px solid CanvasText;
  }
  .rail-footer, .pane-exit, .pane-foot, .pane > .pane-gitnote {
    border-block-start: 1px solid CanvasText;
  }
  .cv-resize {
    border-inline-end: 2px solid CanvasText;
    border-block-end: 2px solid CanvasText;
  }

  /* The selected row: 2px Highlight, and the tick (task 31) in Highlight. Not while focused, so
   * the focus ring is never replaced; the tick still marks the selection then. */
  .rail-row[aria-selected="true"]:not(:focus-visible),
  .sub-row[aria-current="page"]:not(:focus-visible) {
    outline: 2px solid Highlight;
    outline-offset: -2px;
  }
  .rail-row[aria-selected="true"]::before,
  .sub-row[aria-current="page"]::before {
    forced-color-adjust: none;
    background: Highlight;
  }

  /* The cursor row: 1px. The palette's active option is its cursor. */
  .pal-row-on {
    outline: 1px solid Highlight;
    outline-offset: -1px;
  }

  /* Status dots draw their fill as a background. Opting out keeps every dot shape (disc, inset
   * ring, the running ring, the halo) in one system ink; the shape carries the status. */
  .dot, .dot-ring, .dot-live, .dot-flash::after {
    forced-color-adjust: none;
  }
  .dot {
    color: CanvasText;
  }

  /* The unseen tick (tokens.css, reduced motion only) keeps its ink. */
  [data-changed="unseen"]::before {
    forced-color-adjust: none;
    background: CanvasText;
  }

  /* Grain over system colours is noise over someone's chosen contrast. Task 35 does not repeat
   * this rule; its test counts one. */
  .grain::after {
    display: none;
  }
}
```

#### Interfaces — complete: `main.tsx`, and the dev sheets' own lines

`main.tsx`, after `import App from "./App";` (`:7`):

```ts
// LAST, after every component sheet App pulls in: a forced-colours rule must win each tie with
// the component rule it redraws (Phase W task 34). Vite emits CSS in module-graph order.
import "./styles/forced-colors.css";
```

Appended to `devtools.css` (task 32) and `stress.css` (task 33) respectively:

```css
@media (forced-colors: active) {
  .dt, .dt-reset {
    border: 1px solid CanvasText;
  }
}
```

```css
@media (forced-colors: active) {
  .ss-bar {
    border-block-end: 1px solid CanvasText;
  }
}
```

#### Interfaces — complete: the reduced-motion twins and the tick

`tokens.css`, after the floor (`:183`):

```css
/* The floor's twin. data-motion="reduce" (task 33's stress set, through flags.ts) forces the same
 * floor on a machine whose OS setting is off. Every reduced-motion rule in the tree has a twin
 * with IDENTICAL declarations; designsystem.test.ts fails one that drifts. */
:root[data-motion="reduce"] *,
:root[data-motion="reduce"] *::before,
:root[data-motion="reduce"] *::after {
  animation-duration: 0.01ms !important;
  animation-iteration-count: 1 !important;
  transition-duration: 0.01ms !important;
  scroll-behavior: auto !important;
}

/* The unseen tick (05 §4.4 C7, revision 2; AX §6.1's read cursor). With motion reduced a state
 * change cannot flash, so a row whose change the reader has not seen carries a static 2px
 * leading tick in --ink. The attribute is the contract: the lens that owns "seen" sets and clears
 * data-changed="unseen" (task 41), and the tick goes when the attribute goes. --ink, never
 * accent: it is news, not selection. inset-inline-start makes it the leading edge in RTL too. */
[data-changed="unseen"] {
  position: relative;
}

@media (prefers-reduced-motion: reduce) {
  [data-changed="unseen"]::before {
    content: "";
    position: absolute;
    inset-block: 0;
    inset-inline-start: 0;
    inline-size: 2px;
    background: var(--ink);
    pointer-events: none;
  }
}

:root[data-motion="reduce"] [data-changed="unseen"]::before {
  content: "";
  position: absolute;
  inset-block: 0;
  inset-inline-start: 0;
  inline-size: 2px;
  background: var(--ink);
  pointer-events: none;
}
```

`glyphs.css`, after task 29's reduced block; `common.css`, after `:123`; `palette.css`, after
`:135`. Each is its file's media rule, restated under the attribute:

```css
:root[data-motion="reduce"] .gl-agent-live {
  opacity: 1;
}
```

```css
:root[data-motion="reduce"] .dot-live {
  box-shadow: 0 0 0 1.5px currentColor;
}

:root[data-motion="reduce"] .dot-run.dot-live {
  box-shadow: none;
}
```

```css
:root[data-motion="reduce"] .pal {
  animation: none;
}
```

The second `common.css` twin is task 29's addition to the reduced block (`task-29.md:378-383`).
If a Work row (task 40) draws its own `::before`, the tick and it collide on one pseudo-element;
task 40's part was not on the tree at this writing, so that is **UNVERIFIED**, and the fix would
be task 40's (an `::after` or a child), because the tick is the floor.

#### Tests to write first — `desktop/src/styles/designsystem.test.ts`, the four predicates

Appended after task 31's predicates, reusing `rulesOf`, `splitTop`, `REDUCED_MOTION` and task
31's `OLD_LOOK`:

```ts
const FORCED = /forced-colors:\s*active/;
const TWIN = ':root[data-motion="reduce"]';
/** CSS Color 4's system colours: legal only inside a forced-colours block. */
const SYSTEM_COLOUR =
  /(?<![\w-])(?:CanvasText|Canvas|Highlight|HighlightText|LinkText|VisitedText|ActiveText|ButtonFace|ButtonText|ButtonBorder|Field|FieldText|GrayText|Mark|MarkText|SelectedItem|SelectedItemText|AccentColor|AccentColorText)(?![\w-])/g;

const parts = (selector: string): string[] => splitTop(selector, ",").map((s) => s.trim());
const inForced = (r: Rule): boolean => r.within.some((at) => FORCED.test(at));

/** Every selector whose box-shadow Chromium drops needs a forced-colours line that draws it
 *  another way: a border, an outline, or an opt-out. Old-look and twin selectors are exempt:
 *  the first is deleted at task 39, the second restates a rule that is itself checked. */
function ringsWithoutForcedColours(t: Tree): string[] {
  const all = t.css.flatMap((file) => rulesOf(t.read(file)).map((r) => ({ file, r })));
  const redrawn = new Set(
    all
      .filter(({ r }) => inForced(r))
      .filter(({ r }) => r.decls.some((d) => /^(?:border|outline|forced-color-adjust)/.test(d.property)))
      .flatMap(({ r }) => parts(r.selector).map((p) => p.replace(/:not\(:focus-visible\)$/, ""))),
  );
  const offenders: string[] = [];
  for (const { file, r } of all) {
    if (inForced(r) || r.within.some((at) => at.startsWith("@keyframes"))) continue;
    if (!r.decls.some((d) => d.property === "box-shadow" && d.value !== "none")) continue;
    for (const p of parts(r.selector)) {
      if (p.startsWith(OLD_LOOK) || p.startsWith(TWIN) || redrawn.has(p)) continue;
      offenders.push(`${file}  ${p}  box-shadow with no forced-colors border, outline or opt-out`);
    }
  }
  return offenders;
}

/** Each rule inside a reduced-motion media block has, in the same file, a top-level twin whose
 *  every selector is prefixed with the attribute and whose declarations are identical. */
function twinFaults(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    const rules = rulesOf(t.read(file));
    for (const r of rules.filter((x) => x.within.some((at) => REDUCED_MOTION.test(at)))) {
      const want = parts(r.selector).map((p) => `${TWIN} ${p}`).join(", ");
      const twin = rules.find((x) => x.within.length === 0 && parts(x.selector).join(", ") === want);
      if (twin === undefined) offenders.push(`${file}  ${r.selector}  has no ${TWIN} twin`);
      else if (JSON.stringify(twin.decls) !== JSON.stringify(r.decls)) {
        offenders.push(`${file}  ${r.selector}  twin declarations differ`);
      }
    }
  }
  return offenders;
}

/** A system colour outside a forced-colours block bypasses the palette in the normal look. */
function systemColoursOutside(t: Tree): string[] {
  const offenders: string[] = [];
  for (const file of t.css) {
    for (const r of rulesOf(t.read(file))) {
      if (inForced(r)) continue;
      for (const d of r.decls) {
        for (const m of d.value.matchAll(SYSTEM_COLOUR)) offenders.push(`${file}  ${r.selector}  ${m[0]}`);
      }
    }
  }
  return offenders;
}

/** The forced-colours rules that declare `property: value` for exactly `selector`. */
function forcedDecl(t: Tree, selector: string, property: string): string[] {
  return t.css.flatMap((file) =>
    rulesOf(t.read(file))
      .filter((r) => inForced(r) && parts(r.selector).includes(selector))
      .flatMap((r) => r.decls.filter((d) => d.property === property).map((d) => d.value)),
  );
}
```

`SYSTEM_COLOUR` is case-sensitive on purpose: CSS keywords are case-insensitive, but the tree
writes them in this case, and a lowercase `canvas` is far likelier to be a word in a comment than a
colour. `rulesOf` strips comments first (`designsystem.test.ts:116`), so comments never reach it.
