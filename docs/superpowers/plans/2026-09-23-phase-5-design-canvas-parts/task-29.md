# Task 29 — Paste and copy: HTML, SVG, images, text

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 20, 26 · **Hours:** 1.5 · **UI:** yes (a toast and a drop target) — G12; owner's eyes are the acceptance

## Why

Paper's cheapest import is paste — HTML with inline styles as editable layers, SVG, images (spec
§5.1, §9.1 item 12). Its done-when: *pasting a styled fragment yields editable nodes and a dropped
report*. Pasted HTML goes through **the same Go sanitiser** as `write_html` (task 07): the desktop
never parses a pasted string into DOM (G7).

## Files

- `control-plane/internal/designapi/paste.go` + test (new) — route 27
- `canvas/editor/src/editor/paste.ts`, `copy.ts` + tests (new); `keys.ts` wires `Mod+c/v/x` (task 25's table)

## Contracts

| # | Method | Path | Request | `200` response |
|---|---|---|---|---|
| 27 | POST | `/v1/design/canvases/{canvasId}/paste` | `{"targetId":"n-…","mode":"insert-children"\|"replace-children","html":"…","index":0}` (`html` ≤ 64 KiB) | `{"revision": n, "created": ["n-…"], "roots": ["n-…"], "dropped": dialect.Report}` |

Route 27 is `write_html`'s behaviour (task 16) for a **human** caller: `dialect.Parse`, then one
batch labelled `Paste`; nothing surviving is `422 op_rejected` with `details.reason = "rejected-html"`
and the report in the body's `details.dropped` (JSON-encoded, since `details` values are strings).
Humans are never blocked by claims (task 12).

```ts
// canvas/editor/src/editor/paste.ts — handles the viewport's `paste` EVENT (element-scoped,
// clipboardData is synchronous and needs no permission prompt), never navigator.clipboard.read
export type PastePlan =
  | { kind: "html"; html: string }                  // text/html → route 27
  | { kind: "svg"; html: string }                   // image/svg+xml, or text starting "<svg" → route 27 (the dialect keeps the svg node)
  | { kind: "image"; file: Blob; mediaType: string } // png, jpeg, webp → putAsset, then one create_node image op
  | { kind: "text"; text: string }                  // text/plain → one create_node text op (a <p>)
  | { kind: "none" };
export function planPaste(data: DataTransfer): PastePlan; // precedence: image file > svg > html > text

// Target: the selected frame (insert-children at the end); else the selected node's parent,
// right after it; else the artboard under the last pointer position; else the first artboard of
// the current page; else a new 1440 × fit-content artboard created in the same batch.
export function pasteTarget(store: DocStore, selection: string[], pointerArtboard: string | null): { targetId: string | null; index?: number; createArtboard: boolean };

// canvas/editor/src/editor/copy.ts — Mod+C writes text/html (the export route, format html, ids
// off) and text/plain (the text content) for the selection; Mod+X is copy then one delete batch.
export function copySelection(transport: CanvasTransport, canvasId: string, selection: string[], data: DataTransfer): Promise<void>;
```

After a paste, the editor selects the created roots and the host shows one toast: *"Pasted 12
layers."* and, when something was dropped, *"3 things were left out: margin (use padding and gap),
a script, 2 remote images."* — grouped by `Drop.what`, at most three groups named, then *"and N
more"*. The full report opens from the toast.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `paste.go › sameAsWriteHTML` | the route's result for `testdata/card.html` equals `write_html`'s for the same fragment (nodes, ignoring ids) | a second, weaker sanitiser path for humans |
| `paste.go › rejected` | `<script>x</script>` → 422 with the report | an empty success |
| `paste.go › localKeyRequired` | through `Mount` with a real `localkey.Middleware` (task 20's harness): route 27 with no `Authorization`, no `Origin` and `Host: 127.0.0.1:8730` → 401 `local_key_required` and **the authority unchanged** (revision and node count as before); `Bearer <wrong key>` and `Bearer ` (empty) → the same; the right key → not 401 | a sandbox pasting into the canvas as `human` over REST (review C1, re-review M-3) |
| `TestMount_wrapsEveryRoute` (task 20's test, extended) | the count of mounted design routes is now **27** (route 27 added); each answers 401 `local_key_required` with no key | a new route mounted around the guard (re-review M-3) |
| `paste.test.ts › precedence` | a DataTransfer holding a PNG file and text/html plans `image` | a screenshot pasted as its HTML wrapper |
| `paste.test.ts › noDomParse` | `planPaste` returns the HTML string untouched; nothing in `paste.ts` touches `DOMParser` or `innerHTML` | the desktop parsing pasted HTML (G7) |
| `paste.test.ts › target` | each rule of `pasteTarget`, including the new-artboard case | paste doing nothing on an empty canvas |
| `paste.test.ts › toastGrouping` | 7 drops of 5 kinds → three named groups and *"and 2 more"* | a wall of drop lines |
| `copy.test.ts` | Mod+C puts `text/html` from the export route and `text/plain` on the DataTransfer | copy producing ids (`data-node`) the next paste would fight |

## Steps

1. Route test and route. 2. Editor tests and code. 3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./control-plane/internal/designapi/... -race -count=1 && cd canvas/editor && npx vitest run src/editor
```
Expected: `ok`; tests pass.

**Owner's eyes (batched, `OD-12`):** paste a styled fragment copied from a web page and from
another design tool; the dropped report reads as advice, not an error.

## Not claimed

- Paste from Figma's clipboard format is not handled (its mechanism is unpublished, spec §5.1;
  Figma through its MCP is v2).
- No drag-and-drop of files yet; paste only.
