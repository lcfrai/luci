# Task 24 — The renderer: artboard frames, assets, measurement, capture, the render responder

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 01, 23 · **Hours:** 3 · **UI:** yes — `docs/DESIGN-SYSTEM.md` read first; owner's eyes are the acceptance (G12)

## Why

The canvas **is** real DOM (spec §2.1): each artboard renders in its own script-less, same-origin
iframe (spec §7.5; task 01 measured the construct), so the parent can measure it, capture it and
edit over it, and nothing inside can run. This task builds that renderer, answers the authority's
render requests (task 13) from it, and provides the hidden renderer that keeps layout tools alive
while the lens is closed (P5-Q).

It is the second line of G7: **the renderer builds DOM with `createElement` from allowlisted kinds**
and re-checks every property (task 23's `allowedStyle`) before `style.setProperty`. The only HTML
string it ever assigns is the fixed shell below.

## Files

- `canvas/editor/src/renderer/shell.ts`, `buildDom.ts`, `patch.ts`, `ArtboardFrame.tsx`, `assets.ts`,
  `measure.ts`, `capture.ts`, `RenderResponder.ts`, `HiddenRenderer.tsx`, `fonts.ts` (new) + tests
- `canvas/editor/src/g7.test.ts` (new) — the grep gate
- `canvas/editor/package.json` — the capture library task 01 chose (`modern-screenshot`, MIT, by
  default), through the licence gate

## Contracts

```ts
// canvas/editor/src/renderer/shell.ts — THE ONLY HTML STRING IN THE PACKAGE (G7)
export const ARTBOARD_SHELL =
  `<!doctype html><html><head><meta charset="utf-8">` +
  `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src blob: data:; font-src blob: data:; style-src 'unsafe-inline'">` +
  `<style>html,body{margin:0;padding:0;background:transparent}</style></head><body></body></html>`;

// Every artboard iframe, exactly: sandbox without allow-scripts (task 01 S2), not focusable
// (S10), not a pointer target (S4: the editor hit-tests with elementFromPoint).
export const FRAME_ATTRS = { sandbox: "allow-same-origin", tabIndex: -1, "aria-hidden": true } as const;
```

```ts
// canvas/editor/src/renderer/buildDom.ts

export interface BuildContext {
  doc: Document;                              // the iframe's document (a DOM Document, not the wire type)
  assetUrl: (ref: string) => string | null;   // a resolved blob: URL, or null while it loads
  tokens: Token[];                            // written to :root with setProperty('--name', value)
}

// buildNode makes one element for one wire Node, and never its children:
//   frame/artboard/text → createElement(tag) (tags from allowedTag); text → textContent, with
//   "\n" split into text nodes and createElement("br"); image → createElement("img"), src from
//   assetUrl, alt; input → createElement("input") with the kept attrs; svg → DOMParser
//   ("image/svg+xml") of node.svg, then importNode, then a walk that REMOVES any element or
//   attribute outside task 07's SVG lists (the one other string→DOM path, stated here so G7's
//   gate can name it); styles → for each [p, v] with allowedStyle(p, v), el.style.setProperty
//   (p, rewriteAssetUrls(v)); a refused style is dropped and counted (never thrown).
//   Every element gets data-node=<id> for hit-testing; hidden → the hidden attribute.
export function buildNode(ctx: BuildContext, n: Node): Element;

export interface BuildStats { elements: number; refusedStyles: number; refusedTags: number }
```

```ts
// canvas/editor/src/renderer/patch.ts

// ArtboardView keeps id → Element for one artboard and reconciles it with a DocStore: nodes whose
// revision changed are rebuilt in place (attributes and styles diffed, children re-ordered by the
// store's order), new ones created, deleted ones removed. Work is batched into ONE
// requestAnimationFrame per store version (G10: no interval; rAF only while a patch is pending).
export class ArtboardView {
  constructor(frame: HTMLIFrameElement, store: DocStore, artboardId: string, assets: AssetCache);
  get root(): HTMLElement | null;
  element(nodeId: string): Element | null;
  sync(): BuildStats;       // called by the rAF; idempotent
  dispose(): void;
}
```

```ts
// canvas/editor/src/renderer/assets.ts — blob: URLs for asset refs, shared by every frame of a canvas
export class AssetCache {
  constructor(canvasId: string, transport: CanvasTransport);
  url(ref: string): string | null; // null until loaded; loading schedules a patch
  dispose(): void;                 // revokes every URL
}

// canvas/editor/src/renderer/measure.ts
export function boxes(view: ArtboardView, nodeIds: string[]): Box[];     // relative to the artboard root
export function computed(view: ArtboardView, nodeIds: string[]): { nodeId: string; styles: Record<string, string> }[]; // the dialect's properties only

// canvas/editor/src/renderer/capture.ts
export const MAX_EDGE = 4096;
// capture renders one node to PNG at scale (clamped so the longest edge ≤ MAX_EDGE) with the
// library task 01 chose. Features it cannot draw (task 01's S8 matrix) are listed here in a comment.
export function capture(view: ArtboardView, nodeId: string, scale: 1 | 2): Promise<Png>;

// canvas/editor/src/renderer/fonts.ts
export interface BundledFont { family: string; weights: number[]; styles: string[]; variable: boolean; axes: string[]; url: string }
export function fontInfo(doc: Document, family: string, bundled: BundledFont[]): RenderResult["font"];
```

```tsx
// canvas/editor/src/renderer/ArtboardFrame.tsx
export function ArtboardFrame(props: {
  store: DocStore; artboardId: string; assets: AssetCache;
  onView?: (view: ArtboardView | null) => void; // the editor overlay and the responder read the view
}): JSX.Element;
// The iframe is sized to the artboard's width and height; a fit-content height follows the root's
// measured height after each sync.

// canvas/editor/src/renderer/RenderResponder.ts
// Registers with transport.onRenderRequest(canvasId, visible, …) for as long as it lives. A
// request at the store's head uses the live views; one at another revision loads that document
// into an off-screen store and frames, answers, and disposes them. Past the deadline → dropped.
// A request carrying `progress` (the approval's one-artboard capture, task 13, review I5) calls
// onProgress({canvasId, index, total, name}) as it starts; the desktop passes its slice's
// designApprovalProgress (task 27), which the approval toast reads. The editor package keeps no
// progress state of its own.
export function useRenderResponder(opts: {
  transport: CanvasTransport; canvasId: string; visible: boolean;
  views: () => Map<string, ArtboardView>; store: DocStore; bundledFonts: BundledFont[];
  onProgress?: (p: { canvasId: string; index: number; total: number; name: string }) => void;
}): void;

// canvas/editor/src/renderer/HiddenRenderer.tsx — P5-Q: every artboard of one canvas, off-screen
// (position:absolute; left:-100000px; opacity:0; pointer-events:none; contain:strict), registered
// with visible:false. The desktop mounts one per canvas whose branch has a running Claude Code
// session (task 27's selector).
export function HiddenRenderer(props: { transport: CanvasTransport; canvasId: string; bundledFonts: BundledFont[] }): JSX.Element;
```

## Tests first

(jsdom has no layout: DOM-building and patching are tested against
`document.implementation.createHTMLDocument()`; layout and capture are task 01's and the owner's.)

| Test | Asserts | Kills the mutant |
|---|---|---|
| `g7.test.ts` | the package's sources contain `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`, `eval(`, `new Function` **nowhere**, and `srcdoc` only where `ArtboardFrame` assigns `ARTBOARD_SHELL` | a string of agent HTML reaching the DOM |
| `buildDom.test.ts › noScript` | a wire node crafted with `tag: "script"` (a bypass of the server) builds nothing and counts `refusedTags` | a renderer trusting the server |
| `buildDom.test.ts › styleRefused` | `styles: {margin: "4px", color: "red"}` → only `color` set, `refusedStyles: 1` | the renderer as a pass-through |
| `buildDom.test.ts › svgWalk` | `svg: "<svg><script/><path d='M0 0' onload='x'/></svg>"` → a path with `d`, no script, no `onload` | DOMParser output inserted unwalked |
| `buildDom.test.ts › textNewlines` | `"a\nb"` → text, `<br>`, text | newlines collapsed |
| `buildDom.test.ts › assetRewrite` | `background-image: url(asset:sha256-…)` → the cache's `blob:` URL | an `asset:` URL the CSP blocks |
| `patch.test.ts › minimal` | a `set_props` event on one node rebuilds that element only (identity of siblings kept) | a full re-render per event (flicker, lost scroll) |
| `patch.test.ts › order` | a `move` event reorders children to the store's order | DOM order drifting from export order |
| `patch.test.ts › oneFramePerVersion` | 10 events before the rAF fires → one `sync` | a render per event |
| `responder.test.ts › head` | a screenshot request at head answers from the live view (capture stubbed) | a reload for every screenshot |
| `responder.test.ts › pastRevision` | a request at revision 3 while head is 9 loads document 3 through the transport | an approved-revision screenshot of the head |
| `responder.test.ts › deadline` | a request past its deadline gets no answer | late answers the server has already failed |
| `responder.test.ts › progress` | a request with `progress: {purpose: "approval", index: 3, total: 12, name: "Billing"}` calls `onProgress` once, before capture starts; a request without it never calls it | an approval toast that cannot count (review I5) |
| `fonts.test.ts` | a bundled family → `source: "bundled"`, its weights; an unknown family → `available: false`, `source: "missing"` | fonts reported available that will render as fallback |

## Steps

1. Read task 01's report; set `capture.ts`'s library and its "cannot draw" list from S8; if S9
   failed, `AssetCache` makes `data:` URLs instead. 2. Tests, then code. 3. Add the capture library
   to `package.json`; `go run ./tools/licencegate -root .` must stay 0. 4. Acceptance; commit.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/canvas/editor && npx vitest run && npx tsc --noEmit && cd ../.. && go run ./tools/licencegate -root . >/dev/null; echo "gate $?"
```
Expected: tests pass; `gate 0`.

**Owner's eyes (batched, `OD-12`; unverified until then):** a canvas with three artboards renders
each at its size; an agent's edit appears without a flicker; images and a bundled font show.

## Not claimed

- Layout fidelity in WebView2 (task 01 measured Edge; task 30's hands-on runs the shell).
- No selection, handles or panels (tasks 25, 26).
- DESIGN-SYSTEM rules apply to chrome; the renderer draws the user's design, which has its own.
