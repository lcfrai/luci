# Task 14 — The MCP protocol layer

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 05 (`doc.Author` only) · **Hours:** 2.5

## Why

Plan decision P5-A: the server needs five JSON-RPC methods over Streamable HTTP `POST`, stateless,
with identity from the channel — small enough to own, and owning it keeps the authority free of
dependencies and of the SDK's Go-version question. This task is **only** the protocol: framing,
negotiation, the tool registry, argument validation, errors. It knows no canvas. The tool table is
separate (tasks 15–17), so swapping in `modelcontextprotocol/go-sdk` later touches this package alone.

## Files

- `canvas/mcp/jsonrpc.go`, `transport.go`, `server.go`, `registry.go`, `schema.go`, `errors.go`, `identity.go` (new)
- `canvas/mcp/*_test.go` (new)
- `canvas/mcp/ports.go` (new) — **declarations only**: `Briefs`, `Brief`, `Criterion`, `Section`,
  `Approvals`, `ApprovalInfo`, written verbatim from the block at `task-15.md:23-79`, which stays
  the one source of their text. Standard library only (`context`); no function bodies, no canvas
  import, so "this task knows no canvas" still holds for every line of behaviour. It is here, not
  in task 15, because task 20a's `designapi` names all five types and must compile before 15
  lands (plan b02 round 2, re-review RR-I6-1). `mcp.Deps` stays in task 15's `tools.go`: it
  names `*authority.Service`, which 14 does not depend on. No change to the hours.

## Contracts

```go
// canvas/mcp/server.go
package mcp

const ServerName = "panacea-canvas" // no Paper name anywhere (plan G6)

// SupportedVersions, newest first. From knowledge of the published
// revisions; the plan's "not verified" list names this.
var SupportedVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26"}

const MaxRequestBytes = 16 << 20 // upload_asset's 10 MiB, base64-encoded, plus the envelope

type ServerInfo struct {
	Name    string `json:"name"`    // ServerName
	Title   string `json:"title"`   // "Panacea canvas"
	Version string `json:"version"` // the control plane's version
}

type Server struct{ /* unexported */ }

// NewServer makes a server whose initialize answers with info and
// instructions (the short rules, task 17). allowedHosts are the Host header
// values accepted, e.g. "127.0.0.1:8731", "localhost:8731".
func NewServer(info ServerInfo, instructions string, allowedHosts []string, log *slog.Logger) *Server

func (s *Server) Register(t Tool) error // unique name ^[a-z][a-z0-9_]{1,63}$; schema parsed by schema.go
func (s *Server) Handler() http.Handler // serves exactly one path, /mcp
```

```go
// canvas/mcp/registry.go

type Tool struct {
	Name        string
	Title       string
	Description string          // one paragraph the model reads; says what the tool refuses
	InputSchema json.RawMessage // a JSON Schema object (schema.go's subset), additionalProperties false
	ReadOnly    bool            // annotations.readOnlyHint
	Destructive bool            // annotations.destructiveHint (delete_nodes, replace-mode write_html)
	Handler     func(ctx context.Context, id Identity, args json.RawMessage) (Result, error)
}

type Content struct {
	Type     string `json:"type"`               // "text" | "image"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`     // base64, images only
	MimeType string `json:"mimeType,omitempty"` // "image/png"
}

// Result is a tool's answer. Structured becomes structuredContent (2025-06-18
// and later) and is also written as one JSON text block, so a client that
// reads only content sees it too.
type Result struct {
	Content    []Content
	Structured any
	IsError    bool
}

// ToolError is every typed error a tool returns (spec §8.4). A handler
// returning one becomes Result{IsError: true} with the message as text and
// {"error": {"code", "message", "details"}} as structuredContent. Any other
// error from a handler is logged and becomes code "internal", message
// "The canvas server failed; the error is in its log." — never the Go error.
type ToolError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"` // one sentence the agent can act on
	Details map[string]any `json:"details"`
}

func (e *ToolError) Error() string
```

```go
// canvas/mcp/identity.go

// Identity is who called. The relay (task 19) stamps it on the request
// context from the channel the request came through; nothing in a request
// can set it (plan rule 4).
// It carries the author and scope as plain values so this package imports
// only canvas/doc; the tool layer (task 15) turns it into authority.Caller.
type Identity struct {
	Author    doc.Author // Kind is always AuthorSession on this path
	ProjectID string     // the session's local project
	BranchID  string     // the session's local branch; "" for a detached worktree (every tool: out-of-scope)
}

func WithIdentity(ctx context.Context, id Identity) context.Context
func IdentityFrom(ctx context.Context) (Identity, bool)
```

### The wire, exactly

| Request | Response |
|---|---|
| any method other than `POST` on `/mcp` | `405`, `Allow: POST`, no body (the server offers no GET stream, which the transport permits) |
| an `Origin` header | `403`, no body — DNS-rebinding defence; a CLI client sends none |
| a `Host` not in `allowedHosts` | `403` |
| `Content-Type` not `application/json` | `415` |
| body over 16 MiB | `413` |
| no identity on the context | `500` and an error log: a wiring bug, never a client's |
| `MCP-Protocol-Version` present and not supported (any method but `initialize`) | `400` with a JSON-RPC error `-32600` |
| a JSON array (a batch) | `400`, JSON-RPC error `-32600` *"batch requests are not supported"* |
| not JSON, or not a JSON-RPC 2.0 object | `400`, `-32700` or `-32600` |
| a notification or a response (no `id`) | `202`, no body |
| `initialize` | `200` `{"protocolVersion": <the client's if supported, else SupportedVersions[0]>, "capabilities": {"tools": {"listChanged": false}}, "serverInfo": ServerInfo, "instructions": <text>}` |
| `ping` | `200` `{}` |
| `tools/list` | `200` `{"tools": [{"name","title","description","inputSchema","annotations": {"readOnlyHint","destructiveHint","openWorldHint": false}}]}` in registration order, no `nextCursor` |
| `tools/call` of an unknown tool | `200`, JSON-RPC error `-32602` *"no tool named …"* |
| `tools/call` whose `arguments` fail the schema | `200`, a **tool result** with `isError: true`, code `invalid-arguments`, and every violation listed — so the model can correct itself (the 2025-11-25 revision's treatment of input errors) |
| `tools/call` | `200` `{"content": […], "structuredContent": {…}, "isError": false}` |
| any other method (`resources/list`, `prompts/list`, …) | `200`, JSON-RPC error `-32601` |

Every response is `Content-Type: application/json`. No `Mcp-Session-Id` is issued or required
(stateless, P5-A): a restart of the control plane loses nothing a client holds.

```go
// canvas/mcp/schema.go

// The JSON Schema subset tool schemas may use: type (object, string,
// integer, number, boolean, array, or a list of those, e.g.
// ["string","null"]), properties, required, additionalProperties (false on
// every object, except that it may be exactly {"type":["string","null"]} —
// the styles map of task 16, whose keys are CSS properties), enum, minLength,
// maxLength, pattern, minimum, maximum, items, minItems, maxItems,
// description. Register refuses a schema using anything else, so the
// validator is never asked a question it cannot answer.
func Validate(schema json.RawMessage, args json.RawMessage) []Violation

type Violation struct {
	Path    string `json:"path"`    // e.g. "items[2].styles"
	Message string `json:"message"` // e.g. "unknown property \"colour\""
}
```

**Logging.** One line per `tools/call`: tool, session id, duration, result bytes, `isError`, error
code. **Never the arguments or the result** — they carry the user's design and text.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestInitialize_negotiates` | client `2025-06-18` → echoed; client `2024-11-05` → `2025-11-25`; `instructions` and `serverInfo.name` present | a server that always answers its own version |
| `TestNotification_202` | `notifications/initialized` → 202, empty body | a 200 with a body a client rejects |
| `TestBatch_refused` | `[{…},{…}]` → 400, `-32600` | a batch half-processed |
| `TestOrigin_refused` | `Origin: http://evil.example` → 403 | the DNS-rebinding hole the transport's guidance names |
| `TestHost_refused` | `Host: 10.0.0.5:8731` → 403 | a relay forwarding someone else's traffic |
| `TestGet_405` | `GET /mcp` → 405 `Allow: POST` | a GET that hangs a client waiting for a stream |
| `TestToolsList_shape` | two registered tools listed in order with `annotations.openWorldHint: false` | missing annotations |
| `TestCall_schemaViolationIsToolError` | missing `nodeId` and an unknown `colour` → `isError: true`, two violations | a JSON-RPC error the model never sees |
| `TestCall_toolError` | a handler returning `&ToolError{Code: "claimed"}` → `isError: true`, `structuredContent.error.code == "claimed"`, and a text block | typed errors flattened to text |
| `TestCall_internalHidesGoError` | a handler returning `errors.New("open /secret/path")` → message has no `/secret` | internals leaking to the agent |
| `TestCall_noIdentity500` | a request without `WithIdentity` → 500 | an unauthenticated call reaching a tool |
| `TestRegister_refusesUnsupportedSchema` | `oneOf`, a missing `additionalProperties: false` → error | a validator silently ignoring a keyword |
| `TestLog_neverArguments` | a captured log after a call with the text `"secret design words"` does not contain it | user content in logs |

## Steps

1. Tests with `httptest` and a fake tool. 2. Implement. 3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/mcp/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas/mcp`.

## Not claimed

- **Not tested against Claude Code** — that is task 19's first run and task 30's end to end. The
  protocol details above are from knowledge of the published revisions (plan "not verified").
- No server-to-client requests, no resources, no prompts, no SSE responses: none is needed.
