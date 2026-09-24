# Task 16 — MCP write tools

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 07, 12, 14 · **Hours:** 3.5 (3, plus the sandbox's `canvas-upload` helper and its two tests, review I12)

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
- `images/sandbox/canvas-upload.cjs` — new: the upload helper (below; review I12)
- `images/sandbox/Dockerfile` — one `COPY` line, and `canvas-upload` added to the existing `chmod`
  (`:21-22`). No other phase 5 task edits this file
- `.gitattributes` — new, at the repository root: one line, `images/sandbox/** text eol=lf`, so the
  helper's shebang (and the existing `entrypoint.sh`'s) stays LF on a Windows checkout (below;
  re-review RR-I12-1)
- `canvas/mcp/canvasupload_test.go` — new: runs the helper against the real tool

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
  "upload_asset":    { "properties": { "data": {"type":"string","minLength":1,"maxLength":13981016,"description":"base64 of at most 10 MiB. For a file on disk, run `canvas-upload <path>` in a shell instead: it sends the file as this call and prints the result"},
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
| `upload_asset` | store the bytes (`PutAsset`, magic-byte sniffed; SVG sanitised by task 07's `SanitiseSVG("", …)` first: an asset renders in its own image document, so its ids stay unprefixed), then `add_asset`. Base64 in the arguments suits only small bytes the model writes itself (an SVG it drew); a file on disk goes through `canvas-upload` (below), which makes this same call | `asset` (`doc.Asset`) — its `ref` goes in `<img src>` or `url()` | — |
| `start_working` | no batch: `authority.StartWorking` | `claims` | — |
| `finish_working` | no batch: `authority.FinishWorking`; clears the working indicator | `released` (count) | — |
| `request_review` | `request_review` op; **never approves** (plan rule 4, ADR-H1 R5) | `reviewId` | — |

Every batch is labelled with the tool name (the feed's row, `authority.ActivityRow.Label`).

**Deliberately absent** (spec §8.4): approve, bind or re-bind a brief, delete a canvas, break
another session's claim, list other projects' canvases, undo another author's work.

### `canvas-upload`, the sandbox's upload helper (review I12)

**Why.** `upload_asset` takes the bytes as base64 in the tool's arguments, so the model must write
them as its own output. A 300 KB logo is about 400,000 characters, far past what a model emits in
one call, so the *code-to-design* flow (put the repository's logo on the canvas) fails for all but
tiny images. The spec's row for this tool records that the reference server reads local paths
instead (`planning/phase-5/paper-design-cleanroom-spec.md:587`). The endpoint needs no model to write the bytes: `127.0.0.1:8731/mcp` is
reachable by every process in the sandbox, is stateless (no `Mcp-Session-Id`, `task-14.md:139`),
and takes the caller's identity from the channel, not the request (plan G4). So a shell command
can make the same call as the session.

**What.** `images/sandbox/canvas-upload.cjs`, installed as `/usr/local/bin/canvas-upload` by:

```dockerfile
COPY images/sandbox/canvas-upload.cjs /usr/local/bin/canvas-upload
RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/canvas-upload
```

(the `COPY` after `:21`'s, and `:22`'s `chmod` gains the second path). The image runs Node 20
(`FROM node:20-bookworm-slim`, `Dockerfile:10`), so the helper needs nothing installed. It is
**CommonJS** (`.cjs`, `require`), with the shebang `#!/usr/bin/env node`. Installed without an
extension and with no `package.json` above it, it is CommonJS to any Node, while ES-module syntax
runs only on a Node that detects module syntax. The host's Node 24.15.0 does (an extensionless
`import` probe ran, 2026-09-24); whether the image's Node 20 does is unverified. So a host run
cannot catch ES-module syntax, and the test pins the source statically. It uses only
`node:fs`, `node:path` and `node:http`. About 60 lines.

**Line endings (re-review RR-I12-1).** This machine's Git sets `core.autocrlf=true` (from
`C:/Program Files/Git/etc/gitconfig`), the repository has no `.gitattributes`, and this worktree
already checks out `images/sandbox/Dockerfile` and `entrypoint.sh` as `i/lf w/crlf`
(`git ls-files --eol images/sandbox/`, 2026-09-24). The image is built from the working tree
(`docker build -f images/sandbox/Dockerfile .`, `docs/verification/phase-1-operator-runbook.md:54`),
so a CRLF checkout installs `#!/usr/bin/env node\r` and the kernel's shebang parse fails:
`/usr/bin/env: 'node\r': No such file or directory`, exit 127, which no guide row teaches. A host
test cannot see it: `node <file>` drops a hashbang line whatever its ending, and `bufio.Scanner`
strips a trailing `\r`. So the root fix is the attribute, not a `sed` in the Dockerfile: this task
adds the root `.gitattributes` line `images/sandbox/** text eol=lf`, which also repairs the
existing `entrypoint.sh` on the next fresh checkout, and the test reads the helper's raw bytes and
refuses any `\r` (below). A worktree checked out before the attribute keeps its CRLF copies until
they are checked out again; the raw-byte check fails on such a copy, which is the point, since it
reads the same bytes `docker build` copies.

`canvas-upload <file>`
1. The URL is `PANACEA_APP_CANVAS_MCP_URL`, default `http://127.0.0.1:8731/mcp` (the default is the
   only value a sandbox uses; the variable exists for the test). The control plane never reads it,
   and it is not task 20b's on/off `PANACEA_APP_CANVAS_MCP` (plan G4; re-review RR-I12-3).
2. The media type comes from the extension: `.png` `image/png`, `.jpg` and `.jpeg` `image/jpeg`,
   `.webp` `image/webp`, `.svg` `image/svg+xml`. Any other → stderr *"canvas-upload: .gif is not an
   image type the canvas takes (png, jpeg, webp, svg)"*, exit 2. The server sniffs the bytes anyway
   (`TestUploadAsset_sniff`), so a misnamed file is refused there.
3. A missing argument, an unreadable file, or one over 10 MiB (10,485,760 bytes, the per-asset limit
   `task-17.md:79` states) → one stderr line, exit 2, **no request sent**.
4. One `POST` of `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"upload_asset",
   "arguments":{"data":<base64>,"mediaType":<type>}}}` with `Content-Type: application/json` and
   `Accept: application/json, text/event-stream`. **No `Origin` header** (`task-14.md:122` answers
   one with 403) and no `MCP-Protocol-Version` (optional outside `initialize`, `:127`). That is why it
   uses `node:http`, which adds no `Origin` of its own, not `fetch`, whose headers are the Fetch
   standard's to decide; the success case of `TestCanvasUpload_helper` is the proof, since the real
   server refuses an `Origin`. The body is at most
   about 14 MB, under the server's 16 MiB cap (`:125`).
5. `200` with `result.isError: false` → `result.structuredContent` as one JSON line on stdout
   (`asset`, `revision`, `notices`), exit 0. The agent puts `asset.ref` in `<img src>` or `url()`.
6. `result.isError: true` → one stderr line
   `canvas-upload: <result.structuredContent.error.code>: <the result's text content>`, exit 1.
   The code is printed because a `ToolError` carries it only in `structuredContent.error.code`;
   the text is the message alone (`task-14.md:85-87`), and `canvas-rules` teaches what to do per
   code (`task-17.md:79`; re-review RR-I12-2). A JSON-RPC `error`, any other status, or a failed
   transport → one stderr line, exit 1. A refused connection, and a connection closed or reset
   before a response (what `sandboxd`'s listener does after 30 s with no channel attached,
   `task-18.md:101-107`), both read *"canvas-upload: the design canvas is not reachable at
   <url>"*.

It holds no token and names no session: the batch is the session's because the channel is.

**What that means for security (plan G4).** Every process in a sandbox can act on the canvas as
that sandbox's session, within that session's scope (its own project and branch, `task-11.md:218`,
its claims, no approve tool). A build script or a prompt-injected dependency can do what the agent
can do through MCP, and no more. The helper adds no power; it names one that already exists.

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
| `TestCanvasUpload_helper` (`canvasupload_test.go`) | the real server with the write tools behind an `httptest` server (unstarted first, so its `127.0.0.1:<port>` goes into `allowedHosts`; each request's context gets `WithIdentity` of session S), then `node <tmp>/canvas-upload`, a copy of `../../images/sandbox/canvas-upload.cjs` with **no extension**, as the image installs it, with `PANACEA_APP_CANVAS_MCP_URL` pointed at it. First, statically, on the raw bytes of `../../images/sandbox/canvas-upload.cjs` read with `os.ReadFile` (never `bufio.Scanner`, which strips a trailing `\r`): the bytes contain no `\r` at all, the file begins with exactly `#!/usr/bin/env node\n`, and no line begins `import ` or `export ` (the host's Node would run either, the image's may not). Then: a 300 KB PNG fixture → exit 0, stdout's `asset.ref` is in the document, its bytes equal the file's, the `add_asset` batch's author is S; the same PNG renamed `.jpg` → exit 1, stderr is one line beginning `canvas-upload: invalid: `; a `.gif` and a file of 10 MiB + 1 byte → exit 2, and the server counted **no** request; nothing listening, and a listener that accepts and closes without answering (as `sandboxd`'s does with no channel), → exit 1, *"not reachable"* each. No `node` on `PATH` fails the test, naming `Dockerfile:10`: it is never skipped | the review's I12 (no way to place a real image); a helper that sends `Origin` (403) or ES-module syntax; a size check after the upload; a CRLF helper whose shebang fails only once installed (re-review RR-I12-1); a stderr line without the error code (RR-I12-2) |
| `TestImage_shipsCanvasUpload` | `../../images/sandbox/Dockerfile` has the line `COPY images/sandbox/canvas-upload.cjs /usr/local/bin/canvas-upload`, and a `chmod +x` line naming `/usr/local/bin/canvas-upload`; `../../.gitattributes` has the line `images/sandbox/** text eol=lf` | a helper the guide teaches but the image lacks; the attribute dropped, which the raw-byte check alone would catch only on a Windows checkout (re-review RR-I12-1) |
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
- `canvas-upload` has run on the host's Node, not in the image: task 30 step 1 runs it inside a
  real sandbox. An agent runs it through Claude Code's own Bash tool, so the person sees Claude
  Code's usual permission prompt for it; `--allowedTools` pre-approves only `mcp__panacea-canvas`
  (task 19). Pre-approving the helper too is the owner's call (plan OQ-P5-6, recommended no) and a
  one-entry change there, not made: its pattern syntax is unverified against Claude Code 2.1.274.
