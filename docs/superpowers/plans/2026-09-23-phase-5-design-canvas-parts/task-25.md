# Task 25 — The editor: viewport, selection, marquee, move and resize, snapping, canvas keys

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 24 · **Hours:** 3 · **UI:** yes — `docs/DESIGN-SYSTEM.md` read first (G12); owner's eyes are the acceptance

## Why

*A human finishes what agents started* (plan goal 2). Paper's canvas gives select, marquee, move,
resize, gap and padding handles, snapping and smart guides (spec §7.8). This task builds that
interaction layer as an **overlay above the frames**: the iframes take no pointer events (task 01
S4), the overlay hit-tests through `elementFromPoint`, previews a drag as a `transform` on the
overlay (G12), and commits **one batch on pointer-up** (G8) — never an optimistic change to the
document. Agents' work is visible as it happens: a claimed artboard shows who is working in it, on
the one heartbeat (G10).

## Files

- `canvas/editor/src/editor/CanvasEditor.tsx`, `viewport.ts`, `hitTest.ts`, `selection.ts`,
  `handles.tsx`, `drag.ts`, `snap.ts`, `keys.ts`, `presence.tsx`, `editor.css`, `ids.ts` (new) + tests
- `canvas/editor/src/index.ts` — exports `CanvasEditor`, `HiddenRenderer`, the types

## Contracts

```tsx
// canvas/editor/src/editor/CanvasEditor.tsx
export interface CanvasEditorProps {
  transport: CanvasTransport;
  canvasId: string;
  bundledFonts: BundledFont[];
  beat: number;                       // the host's heartbeat phase (desktop/src/hooks/heartbeat.ts); the editor owns no clock
  reduceAgentMotion: boolean;         // Paper's "reduce agent animations" (spec §7.8)
  onSelection?: (nodeIds: string[]) => void;
  onOpenThread?: (threadId: string) => void; // comment pins are drawn here; threads open in the host's panel
  renderPanels?: (ctx: PanelContext) => JSX.Element; // task 26 fills the panels; the host places them
}
export function CanvasEditor(props: CanvasEditorProps): JSX.Element;

export interface PanelContext {
  store: DocStore; selection: string[]; submit: (ops: Op[], label: string) => Promise<ApplyResult>;
  transport: CanvasTransport; canvasId: string;
}
```

```ts
// canvas/editor/src/editor/viewport.ts — world ↔ screen; the world layer moves by transform only
export interface Viewport { x: number; y: number; zoom: number } // zoom ∈ [0.05, 8]
export function screenToWorld(v: Viewport, sx: number, sy: number): { x: number; y: number };
export function worldToScreen(v: Viewport, wx: number, wy: number): { x: number; y: number };
export function zoomAt(v: Viewport, sx: number, sy: number, factor: number): Viewport; // keeps the point under the cursor fixed
export function fit(v: Viewport, bounds: { x: number; y: number; w: number; h: number }, screen: { w: number; h: number }): Viewport;

// canvas/editor/src/editor/hitTest.ts
// pick(): screen point → the node under it: the artboard whose frame contains the world point,
// then frame.contentDocument.elementFromPoint(local), then the nearest [data-node] ancestor.
// Depth rule: a click selects the outermost child of the current scope (the artboard at first);
// double-click enters that node as the scope; Ctrl/Cmd+click selects the deepest node; Escape
// leaves the scope. Locked nodes are skipped; hidden nodes are never hit.
export function pick(views: Map<string, ArtboardView>, v: Viewport, sx: number, sy: number, scope: string | null, deepest: boolean): string | null;

// canvas/editor/src/editor/drag.ts — what a drag commits (one batch, one label)
//   an artboard                → moveArtboard(x, y)                       "Move artboard"
//   a node with position:absolute → setStyles({left, top}) in px          "Move layer"
//   a node in flex flow        → move(id, targetParent, between(prev, next)): reorder or reparent
//                                into the frame under the pointer; the drop line is drawn    "Move layer"
//   resize (8 handles)         → setStyles({width, height}) in px (Shift keeps the ratio);
//                                an artboard's height set this way leaves fit-content        "Resize"
//   Alt/Option-drag            → duplicate with client-minted ids (ids.ts: n- + 12 hex from
//                                crypto.getRandomValues; the authority refuses a collision)   "Duplicate"
//   a flex frame's gap handle  → setStyles({gap})                           "Change gap"

// canvas/editor/src/editor/snap.ts — 4 screen px: artboard edges and centre, sibling edges and centres.
export function snap(moving: Box, candidates: Box[], zoom: number): { dx: number; dy: number; guides: Guide[] };
export interface Guide { axis: "x" | "y"; at: number; from: number; to: number }
```

```ts
// canvas/editor/src/editor/keys.ts — ONE element-scoped onKeyDown on the viewport (G11), active
// only while focus is inside it and no text is being edited. OD-9: the cockpit's chords win.
export const CANVAS_KEYS = {
  "v": "tool:move", "h": "tool:hand", "f": "tool:frame", "t": "tool:text", " ": "pan (hold)",
  "Delete": "delete", "Backspace": "delete", "Escape": "scope:up",
  "ArrowLeft": "nudge -1", "ArrowRight": "nudge +1", "ArrowUp": "nudge -1", "ArrowDown": "nudge +1", // Shift: 10
  "Mod+z": "undo", "Mod+Shift+z": "redo", "Mod+d": "duplicate", "Mod+a": "select all in scope",
  "Mod+c": "copy", "Mod+v": "paste", "Mod+x": "cut",        // task 29
  "Mod+0": "zoom 100%", "Mod+=": "zoom in", "Mod+-": "zoom out", "Shift+1": "zoom to fit",
} as const;
// Any other key, and every key while focus is outside the viewport, is left to bubble to the
// window listener untouched. Step 1 checks useKeyboardMap's chords; a collision is removed from
// this table (the cockpit wins) and named in the commit.
```

```tsx
// canvas/editor/src/editor/presence.tsx — the working indicator (spec §3.1, §7.8)
// A claimed artboard: a 1px ring in var(--run) plus the holder's handle in a chip above the
// artboard label (colour AND text: never colour alone). Its opacity pulses with props.beat; with
// reduceAgentMotion or prefers-reduced-motion it is steady. A lapsed claim's ring fades out over
// var(--dur) (opacity only). No spinner, no per-artboard timer (G10, G12).
```

**Visual rules** (G12, `DESIGN-SYSTEM.md`): the world sits on a flat `var(--canvas)` with **no dot
grid**; artboard labels in `var(--ink-2)` at 11px `var(--sans)`, the selected one in
`var(--accent)` (current state is the accent's job); selection outline 1px `var(--accent)`; handles
are 8px squares with a 1px ring and `var(--e-btn)`; smart guides 1px `var(--accent)` at 60% opacity
(never `var(--need)`: a guide is not a needs-you signal); the marquee a 1px `var(--accent)` outline
over `var(--accent-tint)`; motion `transform`/`opacity` only, `var(--ease)`, under 300ms; zoom and
pan never animate unless the user clicked *zoom to fit* (the motion explains the jump). Live numbers
(zoom %, the W×H readout while resizing) use `var(--numeric)`/`tabular-nums`.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `viewport.test.ts` | `zoomAt` keeps the cursor's world point fixed; zoom clamps to [0.05, 8]; `fit` centres the bounds | zoom that drifts under the cursor |
| `drag.test.ts › flexReorder` | dragging the 3rd of 5 flex children above the 1st commits one `move` with an order key between none and the 1st's | reorder by rewriting every sibling |
| `drag.test.ts › absolute` | an absolute child dragged 12px commits `left`/`top` only | a flex child given `left`/`top` (ignored in flow) |
| `drag.test.ts › artboard` | an artboard drag commits `x`/`y`, not styles | an artboard moved by CSS left/top (would enter the digest) |
| `drag.test.ts › oneBatch` | a drag of 3 selected layers is one `submit` with one label | three undo steps for one gesture |
| `drag.test.ts › noOptimisticApply` | during a drag the `DocStore` is unchanged; after `submit` resolves it changes only via the transport's event | the editor mutating the document (G8) |
| `snap.test.ts` | an edge 3px (at zoom 1) from a sibling's edge snaps; 5px does not; at zoom 2, 1.5 world px snaps | snapping in world units instead of screen |
| `hitTest.test.ts` | with a fake `elementFromPoint`, click → the artboard's child; double-click → scope; Ctrl-click → deepest; a locked node is skipped | selection that always picks the deepest (unusable on real designs) |
| `keys.test.ts › scoped` | `v` with focus outside the viewport does nothing and is not prevented; inside, it switches tool | the canvas stealing the cockpit's keys (OD-9) |
| `keys.test.ts › textEditing` | while a text overlay is open, `Delete` deletes a character, not the layer | a layer deleted mid-typing |
| `presence.test.ts` | a claim renders the handle text and the ring; `reduceAgentMotion` removes the animation style; no `setInterval` exists in the package (grep) | a colour-only indicator; a second clock |
| `editor.css` check | no `transition: all`, no `-gradient(`, no `backdrop-filter`, no hex colour — only `var(--…)` | DESIGN-SYSTEM regressions in chrome |

## Steps

1. Read `desktop/src/hooks/useKeyboardMap.ts` for the cockpit's chords; prune `CANVAS_KEYS`.
2. Tests (jsdom, with fake views and `elementFromPoint`), then code. 3. A standalone harness page
(`canvas/editor/dev/index.html` + `npx vite --port 1498`, using the memory transport) for the
implementer to drive by hand; it is not the owner's acceptance. 4. Acceptance; commit.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/canvas/editor && npx vitest run && npx tsc --noEmit && grep -rn "setInterval\|transition: all\|-gradient(" src | wc -l
```
Expected: tests pass; `0`.

**Owner's eyes (batched, `OD-12`; unverified until then):** select, drill in, marquee, move,
reorder in a flex row, resize, snap with guides, zoom and pan feel direct; the working indicator
reads as calm, not busy; nothing flickers when an agent edits beside the selection.

## Not claimed

- Panels, text editing and the code panel are task 26; paste task 29.
- Rotation handles are not built (the dialect allows `rotate`; the handle is v2).
- Multi-person cursors are v2 (OQ-P5-4).
