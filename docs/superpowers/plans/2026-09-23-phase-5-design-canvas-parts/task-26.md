# Task 26 — The editor: layers, layout, fill, border, effects, type, tokens, code, text in place

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 25 · **Hours:** 3 · **UI:** yes — `docs/DESIGN-SYSTEM.md` read first (G12); owner's eyes are the acceptance

## Why

Spec §9.1 item 6's done-when: *the same page is edited by hand without touching HTML*. Paper's
panels map one to one onto CSS (spec §2.1), so ours do too: every control writes allowlisted
properties through one `set_props` batch, a token picker sits in every field (spec §2.4, [S3]), the
code panel shows the selection as HTML, JSX or Tailwind JSX (Paper's *Copy as React*, §5.2), and
text is edited in place.

## Files

- `canvas/editor/src/panels/LeftPanel.tsx` (pages, `Layers.tsx`, `ThemeTab.tsx`),
  `RightPanel.tsx` (`LayoutPanel.tsx`, `FillPanel.tsx`, `BorderPanel.tsx`, `EffectsPanel.tsx`,
  `TypePanel.tsx`), `TokenPicker.tsx`, `ColorField.tsx`, `LengthField.tsx`, `CodePanel.tsx`,
  `TextEdit.tsx`, `panels.css`, `virtualList.ts` (new) + tests
- `canvas/editor/package.json` — `culori` (MIT, spec L11) for OKLCH, through the gate

## Contracts

```tsx
export function LeftPanel(props: { ctx: PanelContext; tab: "layers" | "theme"; onTab: (t: "layers" | "theme") => void }): JSX.Element;
export function RightPanel(props: { ctx: PanelContext }): JSX.Element;   // empty-state sentence when nothing is selected
export function CodePanel(props: { ctx: PanelContext; format: "html" | "jsx-inline" | "jsx-tailwind"; onFormat: (f: "html" | "jsx-inline" | "jsx-tailwind") => void }): JSX.Element;
// TextEdit is mounted by CanvasEditor on double-click of a text node (task 25's hit test):
export function TextEdit(props: { view: ArtboardView; node: Node; viewport: Viewport; onCommit: (text: string) => void; onCancel: () => void }): JSX.Element;
```

### Controls → properties (each change is one `submit` of `set_props` over the whole selection)

| Panel | Control | Writes |
|---|---|---|
| Layout | *Flex* / *Block* / *None* | `display: flex` / `block` / `none` |
| Layout | direction ↓ → ↑ ← | `flex-direction: column` / `row` / `column-reverse` / `row-reverse` |
| Layout | wrap toggle | `flex-wrap: wrap` / `nowrap` |
| Layout | gap (a length field; the token picker offers spacing tokens) | `gap` |
| Layout | 3×3 alignment grid | `justify-content` + `align-items` (start / center / end), and *space between* as a fourth option |
| Layout | padding, linked or four sides | `padding`, or `padding-top` … `padding-left` (a linked field clears the four sides) |
| Layout | *Absolute* toggle, and X/Y | `position: absolute` + `left`/`top` in px; off removes all three |
| Layout | W, H with *Fixed* / *Fill* / *Fit* | `width`/`height`: px / `100%` (Fill; with `flex-grow: 1` in a flex parent's main axis) / `fit-content` |
| Layout | min/max W and H | `min-width`, `max-width`, `min-height`, `max-height` |
| Fill | fills list: solid, linear gradient, image | `background-color`; `background-image: linear-gradient(…)` or `url(asset:…)`, with `background-size` cover/contain |
| Fill | opacity, blend | `opacity`; `mix-blend-mode` |
| Border | width, style, colour, per side | `border` or `border-top` … |
| Border | radius, linked or per corner | `border-radius`, or the four corner properties |
| Border | outline, offset | `outline`, `outline-offset` |
| Effects | shadows list (x, y, blur, spread, colour, inset) | `box-shadow` |
| Effects | blur, backdrop blur, and the filter list | `filter`, `backdrop-filter` |
| Type | family (with `get_font_info`'s availability shown), weight, size, line height, letter spacing | `font-family`, `font-weight`, `font-size`, `line-height`, `letter-spacing` |
| Type | align, transform, decoration, wrap, truncate | `text-align`, `text-transform`, `text-decoration`, `white-space`/`text-wrap`, `text-overflow: ellipsis` + `overflow: hidden` + `white-space: nowrap` |
| Type | colour | `color` |
| Type | OpenType features, variable axes | `font-feature-settings`, `font-variation-settings` (text fields) |
| Layers | rename (double-click), lock, hide, drag to reorder or reparent | `set_props{name}`, `{locked}`, `{hidden}`; `move` with `between()` |
| Theme | add / edit / delete a token | `set_token`, `delete_token` |

**Mixed values** across a multi-selection show as *Mixed* and write only when changed. A field
whose value is a `var(--x)` shows the token's name as a chip; the picker lists only tokens of the
matching type (`color` for colour fields, `spacing`/`radius`/`font-size`/… for lengths).

**Colour** (`ColorField`): hex, `oklch()`, `display-p3` text accepted; a swatch; an OKLCH picker (L,
C, H) drawn with `culori`'s conversions; the result is written as `oklch(L C H)` unless the user typed
another form (spec §2.1: colour spaces per element).

**Text in place** (`TextEdit`): by default a parent-side `<textarea>` over the text node, positioned
from `boxes()` and styled from `computed()` (task 24) — task 01's S7 decides whether an in-frame
editor is possible; this task builds the overlay either way. Enter without Shift commits one
`set_props{text}`; Shift+Enter is a newline; Escape cancels; blur commits.

**Code panel**: `transport.exportCode(nodeId, format)` for the single selected node; a read-only
`<pre>` in `var(--mono)`; *Copy*. No editor library (P5-J).

**Lists over 50 rows are virtualised** (`DESIGN-SYSTEM.md` floor): layers and tokens, with
`virtualList.ts` (fixed row height, overscan 8). Reordering never reflows the row under the pointer
(`AGENTIC-UX.md` §4 rule 11).

**Visual rules** (G12): panels are `var(--surface)` with `var(--line-soft)` hairlines between
sections; fields `var(--field)`, radius `var(--r-ctl)`; section titles `var(--ink-3)` 11px caps
tracking; numeric fields `tabular-nums`; the accent only on the focused field and the current
toggle; no gradient in any control's chrome (a gradient *swatch* is the user's content).

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `LayoutPanel.test.tsx` (one row per Layout control above) | each control submits exactly the stated properties in one batch | a control writing a property the dialect refuses (it would be silently dropped server-side) |
| `LayoutPanel.test.tsx › fill` | *Fill* on a flex child's main axis writes `flex-grow: 1` and `width: 100%` | a Fill that does nothing in a flex row |
| `LayoutPanel.test.tsx › absoluteOff` | turning *Absolute* off removes `position`, `left`, `top` (null values) | stale insets left behind |
| `mixed.test.tsx` | two nodes with different gaps show *Mixed*; editing padding does not write gap | a multi-edit that flattens other values |
| `TokenPicker.test.tsx` | a colour field lists only colour tokens and writes `var(--brand)` | a radius token offered for a colour |
| `ColorField.test.tsx` | `#2fb3bd` round-trips; an OKLCH pick writes `oklch(…)`; invalid text is not submitted | garbage sent to the server |
| `Layers.test.tsx › virtualised` | 5,000 layers render at most ~40 row elements | a DOM of 5,000 rows |
| `Layers.test.tsx › reorder` | a drag between rows 2 and 3 submits one `move` with `between(order2, order3)` | reorder by renumbering |
| `TextEdit.test.tsx` | Enter commits one `set_props{text}`; Shift+Enter inserts `\n`; Escape submits nothing | a commit per keystroke |
| `CodePanel.test.tsx` | switching format calls `exportCode` with it; *Copy* writes the text to the clipboard (stubbed) | a code panel showing stale code after an edit |
| `panels.css` check | no hex colours, no `transition: all`, no `-gradient(` | chrome regressions |

## Steps

1. Tests with the memory transport (task 23). 2. Panels. 3. `culori` through the gate.
4. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/canvas/editor && npx vitest run && npx tsc --noEmit && cd ../.. && go run ./tools/licencegate -root . >/dev/null; echo "gate $?"
```
Expected: tests pass; `gate 0`.

**Owner's eyes (batched, `OD-12`; unverified until then):** an agent-built settings page can be
restyled, re-spaced, retyped and reordered by hand, without opening the code panel.

## Not claimed

- Constraints (how children respond when a parent resizes, spec §2.2) are not a panel in v1: flex
  and *Fill*/*Fit* cover the common case.
- No component or instance controls (v2).
