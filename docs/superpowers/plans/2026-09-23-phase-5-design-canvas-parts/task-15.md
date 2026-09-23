# Task 15 — MCP read tools, `get_brief`, and the tool layer's ports

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 08, 09, 12, 13, 14 · **External:** phase 4 task 07 (`discoverhub`), coded against a port · **Hours:** 3

## Why

An agent orients before it draws (spec §3.4 step 1): what canvas, what brief, what the human has
selected, what is on the artboard, what it looks like. Fourteen read tools, each scoped to the
calling session's branch canvas (spec §7.5 "Scope"), each carrying the caller's `notices` (task 12).
`get_brief` is ours: it resolves the branch's brief at call time through phase 4, so there is no
mirror to drift (spec §7.6; phase 4 seam S4-P5.3).

The tool layer reaches outside the canvas module only through three **ports**, which the control
plane implements (tasks 20a, 21) and tests fake.

## Files

- `canvas/mcp/tools.go` (the `Tools` builder and error mapping), `ports.go`, `tools_read.go`, `views.go` (new)
- `canvas/mcp/tools_read_test.go` (new); `canvas/canvastest/ports.go` (new: fake ports)

## Contracts

```go
// canvas/mcp/ports.go

// Briefs is phase 4's HeadRef seen from here (S4-P5.3). designapi implements
// it over discoverhub.Service (task 20a).
type Briefs interface {
	Brief(ctx context.Context, projectID, branchID string) (Brief, error)
}

type Criterion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Section struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

// Brief is what get_brief returns. State is the provenance resolution in
// the agent's words; only "resolved" carries content.
type Brief struct {
	State                  string      `json:"state"` // "resolved" | "none" | "missing" | "erased" | "unavailable" | "signed-out" | "not-shared"
	Key                    string      `json:"key"`   // brief:bf-…@n, or ""
	Revision               int         `json:"revision"`
	HeadRevision           int         `json:"headRevision"`
	LatestCompleteRevision int         `json:"latestCompleteRevision"`
	Status                 string      `json:"status"` // the revision's own status
	Title                  string      `json:"title"`
	Headline               string      `json:"headline"`
	Audience               string      `json:"audience"`
	Constraints            string      `json:"constraints"`
	OutOfScope             string      `json:"outOfScope"`
	Criteria               []Criterion `json:"criteria"`
	Sections               []Section   `json:"sections"`
	Message                string      `json:"message"` // for every state but resolved: one sentence, e.g. "This branch has no brief yet; design from the conversation and say so in request_review."
}

// Approvals is the control plane's in-memory view of the Hub's approvals
// (ADR-H1 §5.4). designapi implements it (task 21).
type Approvals interface {
	Latest(ctx context.Context, canvasID string) (ApprovalInfo, bool, error)
}

type ApprovalInfo struct {
	DesignKey      string `json:"designKey"`
	DesignRevision int    `json:"designRevision"`
	ReviewDigest   string `json:"reviewDigest"`
	BriefKey       string `json:"briefKey"`
	ApprovedAt     string `json:"approvedAt"`
	ApprovedBy     string `json:"approvedBy"` // display name
	// BriefSuperseded is computed by the control plane through phase 4's
	// resolver (its P4-B: revision < latestCompleteRevision). The canvas
	// module never parses a brief key to work it out (plan G9).
	BriefSuperseded bool `json:"briefSuperseded"`
}
```

```go
// canvas/mcp/tools.go

type Deps struct {
	Authority *authority.Service
	Briefs    Briefs
	Approvals Approvals
	Now       func() time.Time
}

// RegisterAll registers every tool of tasks 15–17 on s. Tools are added by
// those tasks in that order; tools/list order is registration order.
func RegisterAll(s *Server, d Deps) error

// Every tool result's structuredContent is an object with at least
// {"revision": <head>, "notices": [authority.Notice]} beside its own fields.
```

**Error mapping, one table for every tool** (spec §8.4's closed set, plus `invalid` and
`invalid-arguments`):

| From | Code | Details |
|---|---|---|
| `authority.ErrNotFound`, `ErrNoCanvas`, `doc.CodeNotFound` | `not-found` | `nodeId` or `canvas`; the no-canvas message says *"create_artboard creates it"* |
| `*authority.ConflictError` | `conflict` | `currentRevision`, `nodes` |
| `*authority.ClaimedError` | `claimed` | `artboardId`, `holder` (the handle), `expiresAt` |
| a parse with no roots | `rejected-html` | `dropped` |
| `authority.ErrRendererUnavailable` | `renderer-unavailable` | message: *"No renderer is attached: the cockpit's desktop app must be running. Open the Design lens for the best fidelity."* |
| `authority.ErrTooLarge`, `dialect.ErrTooLarge` | `too-large` | `limit` |
| `authority.ErrOutOfScope`, a detached worktree | `out-of-scope` | — |
| other `*doc.OpError` | `invalid` | `index`, `opCode`, `nodeId` |
| schema failure (task 14) | `invalid-arguments` | `violations` |

### The read tools — input schemas, exactly

Every schema is `{"type":"object","additionalProperties":false, …}`; below, only `properties` and
`required` are shown for each. Shared definitions:
`NODE` = `{"type":"string","pattern":"^n-[0-9a-f]{12}$"}`,
`PARENT` = `{"type":"string","pattern":"^(n|pg)-[0-9a-f]{12}$"}`,
`AT` = `{"type":"string","enum":["head","approved"],"description":"head (default) or the approved revision"}`.

```jsonc
{
  "get_canvas_info":     { "properties": {}, "required": [] },
  "get_brief":           { "properties": {}, "required": [] },
  "get_selection":       { "properties": {}, "required": [] },
  "get_node":            { "properties": { "nodeId": NODE, "at": AT }, "required": ["nodeId"] },
  "get_children":        { "properties": { "nodeId": PARENT, "at": AT }, "required": ["nodeId"] },
  "get_tree_summary":    { "properties": { "nodeId": PARENT, "depth": {"type":"integer","minimum":1,"maximum":10}, "at": AT }, "required": ["nodeId"] },
  "find_nodes":          { "properties": { "name": {"type":"string","maxLength":120}, "kind": {"type":"string","enum":["artboard","frame","text","image","svg"]},
                                           "text": {"type":"string","maxLength":200}, "usesToken": {"type":"string","pattern":"^[a-z][a-z0-9-]{0,62}$"},
                                           "artboardId": NODE, "limit": {"type":"integer","minimum":1,"maximum":200} }, "required": [] },
  "get_html":            { "properties": { "nodeId": NODE, "at": AT, "ids": {"type":"boolean"}, "pretty": {"type":"boolean"} }, "required": ["nodeId"] },
  "get_jsx":             { "properties": { "nodeId": NODE, "format": {"type":"string","enum":["inline","tailwind"]}, "at": AT }, "required": ["nodeId", "format"] },
  "get_computed_styles": { "properties": { "nodeIds": {"type":"array","items": NODE,"minItems":1,"maxItems":50}, "at": AT }, "required": ["nodeIds"] },
  "get_screenshot":      { "properties": { "nodeId": NODE, "scale": {"type":"integer","enum":[1,2]}, "at": AT }, "required": ["nodeId"] },
  "get_asset":           { "properties": { "asset": {"type":"string","pattern":"^asset:sha256-[0-9a-f]{64}$"} }, "required": ["asset"] },
  "get_font_info":       { "properties": { "family": {"type":"string","minLength":1,"maxLength":100} }, "required": ["family"] },
  "list_activity":       { "properties": { "sinceRevision": {"type":"integer","minimum":0}, "limit": {"type":"integer","minimum":1,"maximum":200} }, "required": ["sinceRevision"] }
}
```

`at: "approved"` reads the revision of `Approvals.Latest`; with no approval it is `not-found`
(*"this design has no approval yet"*). Only `get_canvas_info` answers a branch with no canvas
without an error: `{"exists": false, …}`.

```go
// canvas/mcp/views.go — the structured results

type CanvasInfo struct {
	Exists           bool                   `json:"exists"`
	CanvasID         string                 `json:"canvasId"`
	Revision         int                    `json:"revision"`
	DesignKey        string                 `json:"designKey"` // design:<id>@<head>, "" at revision 0
	Pages            []authority.PageSummary `json:"pages"`
	FontsUsed        []string               `json:"fontsUsed"`
	Claims           []authority.Claim      `json:"claims"`
	Presence         []authority.Presence   `json:"presence"`
	Reviews          []doc.ReviewRequest    `json:"reviews"`
	RendererAttached bool                   `json:"rendererAttached"`
	Approval         *ApprovalView          `json:"approval"` // null when none
	Brief            BriefChip              `json:"brief"`
	Notices          []authority.Notice     `json:"notices"`
}

type ApprovalView struct {
	State          string `json:"state"` // "approved" | "changed" (head digest ≠ approved) | "superseded" (the brief moved past the approved key)
	DesignKey      string `json:"designKey"`
	DesignRevision int    `json:"designRevision"`
	BriefKey       string `json:"briefKey"`
	ApprovedAt     string `json:"approvedAt"`
	ApprovedBy     string `json:"approvedBy"`
}

type BriefChip struct {
	State string `json:"state"` // Brief.State
	Key   string `json:"key"`
}

type NodeView struct {
	doc.Node
	ChildIDs   []string       `json:"childIds"`
	ArtboardID string         `json:"artboardId"`
	Box        *authority.Box `json:"box"` // null without a renderer; never an error for get_node
}

type ChildView struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       doc.Kind `json:"kind"`
	Tag        string   `json:"tag"`
	ChildCount int      `json:"childCount"`
}

type FoundNode struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       doc.Kind `json:"kind"`
	ArtboardID string   `json:"artboardId"`
}
```

Result fields per tool (all beside `revision` and `notices`): `get_canvas_info` → `CanvasInfo`;
`get_brief` → `Brief`; `get_selection` → `{"nodes": [FoundNode], "at": "…"}`; `get_node` →
`{"node": NodeView}`; `get_children` → `{"children": [ChildView]}`; `get_tree_summary` →
`{"summary": "<indented outline, one line per node: name (kind, layout) id>"}`; `find_nodes` →
`{"nodes": [FoundNode], "truncated": bool}`; `get_html` → `{"html"}`; `get_jsx` → `{"jsx"}`;
`get_computed_styles` → `{"computed": [authority.Computed]}`; `get_screenshot` → an `image`
content block plus `{"nodeId","width","height","scale"}`; `get_asset` → an `image` content block
plus `{"asset": doc.Asset}`; `get_font_info` → `authority.FontInfo`; `list_activity` →
`{"rows": [authority.ActivityRow]}`.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestTools_scopeFromIdentity` | a session on branch A calling `get_node` on a node of branch B's canvas → `not-found` (the other canvas does not exist from here) | a tool resolving the canvas from an argument |
| `TestGetCanvasInfo_noCanvas` | `exists: false`, no error, and the sentence | an agent that must fail to orient |
| `TestGetCanvasInfo_approvalStates` | fake approval at rev 5: head digest equal → `approved`; a content change → `changed`; `BriefSuperseded: true` and an unchanged digest → `superseded`; both → `changed` (a human must look either way; the lens shows both) | a changed design reported as approved |
| `TestGetBrief_passesThrough` | each of the seven fake states comes back with its message; only `resolved` has text | brief text fabricated when phase 4 answered `unavailable` |
| `TestGetBrief_noCopy` | after `get_brief`, the canvas store contains no brief text (grep the MemStore's documents) | a mirror by the back door (spec §7.6) |
| `TestGetSelection_empty` | no editor → `nodes: []` | a stale selection |
| `TestGetNode_boxOptional` | no renderer → `box: null` and success | `get_node` failing without the lens |
| `TestGetHTML_roundTrip` | `get_html{ids:false}` output fed to `write_html` (task 16's handler via a test double) recreates equal content | a read form the write path cannot take |
| `TestGetScreenshot_imageBlock` | fake renderer PNG → content `[{"type":"image","mimeType":"image/png"}]` and dims | a base64 string hidden in text |
| `TestGetScreenshot_noRenderer` | `renderer-unavailable` with the sentence | a blank image |
| `TestAtApproved` | `get_html{at:"approved"}` returns revision 5's HTML while the head is 9 | a Develop agent reading work-in-progress as approved |
| `TestNotices_onEveryRead` | a human edit in a claimed artboard shows in the next `get_children`'s `notices` | notices only on writes |
| `TestTreeSummary_depth` | depth 1 lists only children; depth is capped at 10 | an unbounded outline on a 50,000-node canvas |

## Steps

1. Fake ports in `canvastest/ports.go`. 2. Tests, then `ports.go`, `tools.go`, `views.go`,
`tools_read.go`. 3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/mcp/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas/mcp`.

## Not claimed

- The real `Briefs` and `Approvals` are the control plane's (tasks 20a, 21). Phase 4's
  `discoverhub` is plan text; the port is what this task depends on.
- Descriptions are written for the model; their wording is reviewed with the guides in task 17.
