# Task 17 — MCP tokens, comments, export, guides, and the server's instructions

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 09, 15, 16 · **Hours:** 2.5

## Why

Paper's server teaches the agent (spec §6 point 4): the quality floor lives in the server, not in
every user's prompt. Ours does the same through the `initialize` instructions (short, always read)
and `get_guide` (long, on demand). Tokens are how an agent turns a repository's CSS variables into
a design system on the canvas (spec §2.4, §9.1 item 7's done-when); comments are the human-to-agent
channel on the canvas (§9.1 item 9); export is the design-to-code path's last step (§5.2).

## Files

- `canvas/mcp/tools_tokens.go`, `tools_comments.go`, `tools_export.go`, `guides.go`, `instructions.go` (new)
- `canvas/mcp/guides/canvas-rules.md`, `design-quality.md`, `design-to-code.md`, `code-to-design.md`, `figma-via-mcp.md` (new, embedded with `//go:embed`)
- tests beside them

## Contracts

```jsonc
{
  "get_tokens":                { "properties": {}, "required": [] },
  "set_tokens":                { "properties": {
        "upsert": {"type":"array","maxItems":200,"items": {"type":"object","additionalProperties":false,
                   "properties": {"name": {"type":"string","pattern":"^[a-z][a-z0-9-]{0,62}$"},
                                  "type": {"type":"string","enum":["color","radius","spacing","container","breakpoint","font-family","font-weight","font-size","line-height","letter-spacing"]},
                                  "value": {"type":"string","minLength":1,"maxLength":500}},
                   "required": ["name","type","value"]}},
        "delete": {"type":"array","maxItems":200,"items": {"type":"string","pattern":"^[a-z][a-z0-9-]{0,62}$"}} }, "required": [] },
  "list_comment_threads":      { "properties": { "status": {"type":"string","enum":["open","resolved","all"]} }, "required": [] },
  "get_comment_thread":        { "properties": { "threadId": {"type":"string","pattern":"^th-[0-9a-f]{12}$"} }, "required": ["threadId"] },
  "reply_to_comment_thread":   { "properties": { "threadId": {"type":"string","pattern":"^th-[0-9a-f]{12}$"},
                                                  "text": {"type":"string","minLength":1,"maxLength":4000} }, "required": ["threadId","text"] },
  "set_comment_thread_status": { "properties": { "threadId": {"type":"string","pattern":"^th-[0-9a-f]{12}$"},
                                                  "status": {"type":"string","enum":["open","resolved"]} }, "required": ["threadId","status"] },
  "export":                    { "properties": { "nodeIds": {"type":"array","items": NODE,"minItems":1,"maxItems":20},
                                                  "format": {"type":"string","enum":["png","html","jsx-inline","jsx-tailwind"]},
                                                  "scale": {"type":"integer","enum":[1,2]}, "at": AT }, "required": ["nodeIds","format"] },
  "get_guide":                 { "properties": { "topic": {"type":"string","enum":["canvas-rules","design-quality","design-to-code","code-to-design","figma-via-mcp"]} },
                                 "required": ["topic"] }
}
```

| Tool | Behaviour | Result fields (beside `revision`, `notices`) |
|---|---|---|
| `get_tokens` | every token, by name | `tokens`, `css` (task 09's `tokens.CSS`, so the agent can paste it into code) |
| `set_tokens` | one batch: `set_token` × upsert, `delete_token` × delete. A value refused by `dialect.Checker.TokenValue` is **reported, not fatal**; all refused → `invalid` | `set`, `deleted`, `dropped` |
| `list_comment_threads` | default `open` | `threads`: `[{"id","nodeId","artboardId","status","messageCount","lastMessage": doc.Message}]` |
| `get_comment_thread` | — | `thread` (`doc.Thread`) |
| `reply_to_comment_thread` | `add_message`, authored by the session | `messageId` |
| `set_comment_thread_status` | `set_thread_status` | `status` |
| `export` | `png` through the renderer (content: one `image` block per node); `html` → task 08's `ExportHTML` for artboards, `HTML(IDs:false)` for other nodes; `jsx-*` → task 09's `JSX` (content: one `text` block per node) | `files`: `[{"nodeId","format","name","width","height","bytes"}]` |
| `get_guide` | the embedded markdown, verbatim | `topic`, `markdown` (and one `text` block) |

Comment threads are **created by humans** in the lens (task 28): no MCP tool opens one. An agent
answers in place and resolves (spec §9.1 item 9's done-when: *an agent resolves a comment it acted on*).

```go
// canvas/mcp/instructions.go — the initialize instructions, verbatim. A test pins the text.
const Instructions = `This server is the design canvas for the branch your session is on. The canvas is real HTML and CSS: you draw by writing HTML, and a person can edit the same layers by hand.

Work in this order:
1. Orient: call get_canvas_info, then get_brief. The brief is what this branch is for. If it has no brief, say so when you request review.
2. Before your first typographic styling, call get_font_info for each family you plan to use.
3. Plan before drawing: unless the repository or the brief gives you a design system, decide a palette, type scale and spacing first, and create them with set_tokens. Use var(--token) in styles.
4. Write in small pieces: each write_html call should add about one visual group. Prefer duplicate_nodes, update_styles and set_text over rewriting HTML.
5. The dialect: inline styles only; layout is flex with padding and gap; position: absolute is the escape hatch. No margin, no grid, no tables, no classes, no scripts. Every write returns a dropped report — read it and correct yourself.
6. Verify: call get_screenshot after meaningful changes. Read exact values with get_computed_styles and get_html, not from pixels.
7. Other agents may be working on other artboards. Your first write to an artboard claims it; if a write fails with claimed, work elsewhere or ask the person. Read the notices on every result: they tell you what someone else changed in your artboards.
8. When you finish an artboard, call finish_working. When the design is ready for a person to look at, call request_review with a short note. You cannot approve a design; only a person can.
Never show layer ids to the person. The long form of these rules is get_guide("canvas-rules").`
```

### The guides — required content, each a test

| Topic | Must contain (checked by `TestGuides_required`) |
|---|---|
| `canvas-rules` | the dialect table (allowed and refused, from task 07's `Reasons`, generated so they cannot drift); the call order above at length; claims, `ifRevision`, notices; the error codes and what to do about each; the limits (64 KiB per write, 10 MiB per asset, 4,096 px screenshot edge) |
| `design-quality` | contrast, type scale, spacing rhythm, alignment, artboard fit (`height: fit-content`), repetition through duplication, icons as SVG never emoji, and `docs/DESIGN-SYSTEM.md`'s tells to avoid (the indigo-violet gradient, the gradient hero with a three-card grid, one radius everywhere) — written as advice for **the user's** product, which may have its own system |
| `design-to-code` | read with `at: "approved"`; `get_jsx` in the repository's style; `get_computed_styles` for exact values; `get_asset` for bytes; the handoff bundle's location `.panacea/design/<canvasId>@<rev>/` and its files (task 21) |
| `code-to-design` | read the repository's CSS variables and create them with `set_tokens`; draw with the codebase's own spacing and type |
| `figma-via-mcp` | *"v2: not supported yet"* and why (spec OQ-11: egress and the user's own Figma seat need the owner and security) — a guide that says so beats a tool that half-works |

No guide names Paper or claims compatibility with it (plan G6).

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestInstructions_pinned` | `initialize`'s `instructions` equals `Instructions`; it contains `get_brief`, `claimed`, `finish_working`, `request_review`, `cannot approve` | the rules drifting out of the server |
| `TestInstructions_noBrand` | neither `Instructions` nor any guide contains `paper` (case-insensitive) | G6 broken |
| `TestGuides_required` | each topic's must-contain list | a guide shipped empty |
| `TestGuides_dialectGenerated` | `canvas-rules`'s refused-property list equals `dialect.Reasons`' keys | the guide teaching a rule the parser does not enforce |
| `TestSetTokens_fromRepoCSS` | upserting 12 tokens parsed from a fixture `:root{…}` block succeeds as one batch; `get_tokens.css` round-trips it | the spec's done-when for tokens |
| `TestSetTokens_partial` | one bad value reported, the rest set | all-or-nothing on a 200-token import |
| `TestComments_agentResolves` | a thread created by the human (fixture); the agent replies and resolves; the thread shows the session as the reply's author | the spec's done-when for comments |
| `TestComments_noCreateTool` | no tool creates a thread | agents spamming the human's comment panel |
| `TestExport_pngNeedsRenderer` | `renderer-unavailable` without one; with the fake, one image block per node | an export that silently omits PNGs |
| `TestExport_htmlStandalone` | an artboard's export has no `http` and no `<script` | an export that references the canvas |

## Steps

1. Write the guides (markdown, embedded). The dialect section of `canvas-rules` is generated from
   task 07's tables by a `go generate` step, committed. 2. Tests, then code. 3. Acceptance; commit.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/mcp/... -race -count=1 && grep -ric "paper" canvas/mcp/guides canvas/mcp/instructions.go | grep -v ":0" | wc -l
```
Expected: `ok  	…/canvas/mcp`, then `0`.

## Not claimed

- Whether the instructions make agents draw well is the owner's judgement at task 30, not a test.
- No Figma import (v2).
