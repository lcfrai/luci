# Task 16 — MCP write tools

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 07, 12, 14 · **Hours:** 3

## Why

Thirteen tools that turn an agent's intent into op batches (spec §8.2). Each follows Paper's verb
where the verb is simply the obvious name (spec OQ-5, plan G6). Every one is **one batch**: one
revision, one undo step, one feed row (P5-K). Every one claims the artboards it touches on first
touch (task 12), accepts `ifRevision`, and returns the new revision, the ids it touched and the
caller's notices. `write_html` also returns the **dropped report** (spec §7.3): a write never fails
silently.

## Files

- `canvas/mcp/tools_write.go`, `place.go` (artboard placement, sibling order) — new
- `canvas/mcp/tools_write_test.go` — new

## Contracts

Shared definitions as task 15, plus
`IFREV` = `{"type":"integer","minimum":0,"description":"fail with conflict if any node this changes moved past this revision"}`,
`SHARED` = `{"type":"boolean","description":"join a shared claim instead of taking an exclusive one"}`,
`STYLES` = `{"type":"object","additionalProperties":{"type":["string","null"]},"description":"CSS property → value; null removes"}`
(the one object whose keys are free: task 14's validator allows `additionalProperties` to be a
schema **only here**, for `styles`, and checks each value's type), `ARTBOARDS` =
`{"type":"array","items": NODE,"minItems":1,"maxItems":50}`.

```jsonc
{
  "create_page":     { "properties": { "name": {"type":"string","minLength":1,"maxLength":120} }, "required": ["name"] },
  "create_artboard": { "properties": { "name": {"type":"string","maxLength":120}, "pageId": {"type":"string","pattern":"^pg-[0-9a-f]{12}$"},
                                        "width": {"type":"integer","minimum":1,"maximum":10000},
                                        "height": {"type":"integer","minimum":1,"maximum":20000,"description":"omit for fit-content"},
                                        "styles": STYLES }, "required": [] },
  "write_html":      { "properties": { "targetId": NODE, "mode": {"type":"string","enum":["insert-children","replace","replace-children"]},
                                        "html": {"type":"string","minLength":1,"maxLength":65536},
                                        "index": {"type":"integer","minimum":0}, "ifRevision": IFREV, "shared": SHARED },
                       "required": ["targetId", "mode", "html"] },
  "set_text":        { "properties": { "items": {"type":"array","minItems":1,"maxItems":200,"items": {"type":"object","additionalProperties":false,
                                        "properties": {"nodeId": NODE, "text": {"type":"string","maxLength":10000}}, "required": ["nodeId","text"]}},
                                        "ifRevision": IFREV, "shared": SHARED }, "required": ["items"] },
  "update_styles":   { "properties": { "items": {"type":"array","minItems":1,"maxItems":100,"items": {"type":"object","additionalProperties":false,
                                        "properties": {"nodeIds": {"type":"array","items": NODE,"minItems":1,"maxItems":200}, "styles": STYLES},
                                        "required": ["nodeIds","styles"]}}, "ifRevision": IFREV, "shared": SHARED }, "required": ["items"] },
  "rename_nodes":    { "properties": { "items": {"type":"array","minItems":1,"maxItems":200,"items": {"type":"object","additionalProperties":false,
                                        "properties": {"nodeId": NODE, "name": {"type":"string","maxLength":120}}, "required": ["nodeId","name"]}},
                                        "shared": SHARED }, "required": ["items"] },
  "duplicate_nodes": { "properties": { "nodeIds": {"type":"array","items": NODE,"minItems":1,"maxItems":50}, "targetParentId": PARENT, "shared": SHARED },
                       "required": ["nodeIds"] },
  "move_nodes":      { "properties": { "nodeIds": {"type":"array","items": NODE,"minItems":1,"maxItems":100}, "targetParentId": PARENT,
                                        "index": {"type":"integer","minimum":0}, "ifRevision": IFREV, "shared": SHARED },
                       "required": ["nodeIds", "targetParentId"] },
  "delete_nodes":    { "properties": { "nodeIds": {"type":"array","items": NODE,"minItems":1,"maxItems":100}, "ifRevision": IFREV, "shared": SHARED },
                       "required": ["nodeIds"] },
  "upload_asset":    { "properties": { "data": {"type":"string","minLength":1,"maxLength":13981016,"description":"base64 of at most 10 MiB"},
                                        "mediaType": {"type":"string","enum":["image/png","image/jpeg","image/webp","image/svg+xml"]} },
                       "required": ["data", "mediaType"] },
  "start_working":   { "properties": { "artboardIds": ARTBOARDS, "shared": SHARED }, "required": ["artboardIds"] },
  "finish_working":  { "properties": { "artboardIds": {"type":"array","items": NODE,"maxItems":50} }, "required": [] },
  "request_review":  { "properties": { "artboardIds": ARTBOARDS, "note": {"type":"string","minLength":1,"maxLength":2000} },
                       "required": ["artboardIds", "note"] }
}
```

### Behaviour, per tool

| Tool | Ops, as one batch | Result fields (beside `revision`, `notices`) | Annotations |
|---|---|---|---|
| `create_page` | `create_page` at the end | `pageId` | — |
| `create_artboard` | `create_node` artboard: `width` (default 1440) in px, `height` in px or `fit-content`, then `styles` merged; placed on `pageId` (default: the first page) at `X` = the right edge of the page's right-most artboard + 80, `Y` = that artboard's `Y` (or 0, 0). **Creates the canvas if the branch has none** (`Ensure`, P5-L). Claims the new artboard | `artboardId`, `x`, `y` | — |
| `write_html` | `dialect.Parse` (task 07), then: `insert-children` → `create_node` × n under `targetId` at `index` (default: last); `replace` → `delete_node` target + create at its place (**an artboard target is refused**: *"use replace-children to rewrite an artboard's content"*); `replace-children` → delete every child + create | `created` (every new id, pre-order), `roots`, `dropped` (`dialect.Report`) | `destructiveHint` true (replace modes delete) |
| `set_text` | `set_props{Text}` per item; a non-text node is `invalid` at its index | `updated` | — |
| `update_styles` | `set_props{Styles}` per node; each declaration through `dialect.CheckDeclaration` first — refused ones are **reported, not fatal**; if every declaration of every item is refused → `rejected-html`-style error with code `invalid` and the report | `updated`, `dropped` | — |
| `rename_nodes` | `set_props{Name}` | `updated` | — |
| `duplicate_nodes` | `create_node` × each subtree with new ids (an svg copy's ids are re-prefixed to its new node id by `Checker.SVG` inside `Apply`, task 06, review I8), placed right after its source (or at the end of `targetParentId`); a duplicated artboard is offset +80 px on X | `copies`: `[{"sourceId","copyId","idMap":{old:new}}]` (Paper's descendant map, spec §3.2 row 25) | — |
| `move_nodes` | `move` × n, in the given order, starting at `index` (default: last); ids preserved; a cycle → `invalid` (`cycle`) | `moved` | — |
| `delete_nodes` | `delete_node` × n (a node inside another deleted node is skipped, not an error) | `deleted` | `destructiveHint` true |
| `upload_asset` | store the bytes (`PutAsset`, magic-byte sniffed; SVG sanitised by task 07's `SanitiseSVG("", …)` first: an asset renders in its own image document, so its ids stay unprefixed), then `add_asset` | `asset` (`doc.Asset`) — its `ref` goes in `<img src>` or `url()` | — |
| `start_working` | no batch: `authority.StartWorking` | `claims` | — |
| `finish_working` | no batch: `authority.FinishWorking`; clears the working indicator | `released` (count) | — |
| `request_review` | `request_review` op; **never approves** (plan rule 4, ADR-H1 R5) | `reviewId` | — |

Every batch is labelled with the tool name (the feed's row, `authority.ActivityRow.Label`).

**Deliberately absent** (spec §8.4): approve, bind or re-bind a brief, delete a canvas, break
another session's claim, list other projects' canvases, undo another author's work.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestCreateArtboard_createsCanvas` | on a branch with no canvas, the call creates it and the artboard; `get_canvas_info` then `exists: true` | an agent that can never start (P5-L) |
| `TestCreateArtboard_placement` | three artboards land at x = 0, w₁+80, w₁+w₂+160 | artboards stacked on one another |
| `TestWriteHTML_oneBatch` | a 30-node fragment moves the revision by 1 and is one activity row | a revision per node |
| `TestWriteHTML_droppedReport` | `<div style="margin:4px"><script>x</script>Hi</div>` → created div, `dropped` lists `margin` and `script` | silent drops |
| `TestWriteHTML_rejected` | `<script>x</script>` → `rejected-html`, nothing written | an empty success |
| `TestWriteHTML_replaceArtboardRefused` | `replace` on an artboard → `invalid` with the replace-children sentence | an agent deleting an artboard (and its claim and comments) by accident |
| `TestWriteHTML_claimed` | session T writing into S's artboard → `claimed` naming S's handle | parallel agents overwriting (brief R5) |
| `TestWriteHTML_ifRevision` | stale `ifRevision` on `replace-children` → `conflict` | replace escaping the precondition |
| `TestUpdateStyles_partial` | `{color: red, margin: 4px}` → color applied, margin in `dropped`, success | a whole batch refused for one bad property |
| `TestUpdateStyles_allRefused` | `{margin: 4px}` only → error, nothing written | a no-op reported as a success |
| `TestDuplicate_idMap` | duplicating a 6-node card returns a map of all 6 ids | agents re-reading the tree after every copy |
| `TestMove_cycle` | moving a frame into its own child → `invalid`, `cycle` | a corrupted tree |
| `TestDelete_nested` | deleting a parent and its child together succeeds once | an error for a redundant id |
| `TestUploadAsset_sniff` | PNG bytes declared `image/jpeg` → `invalid`; an SVG with `<script>` is stored sanitised | a mislabelled or scripted asset |
| `TestRequestReview_neverApproves` | after `request_review`, the fake `Approvals` port has no call and the review is `open` | a tool path to approval |
| `TestNoApproveTool` | `tools/list` has no tool whose name contains `approve`, `bind`, `delete_canvas` or `break` | a forbidden tool added later |

## Steps

1. Tests over a `MemStore` authority, the fake ports, and the fake renderer. 2. Implement.
3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/mcp/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas/mcp`.

## Not claimed

- No real agent has called these tools yet (task 19, then task 30).
- The quality of what an agent draws is the guides' job (task 17), not these tools'.
