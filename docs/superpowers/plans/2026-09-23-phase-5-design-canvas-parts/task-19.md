# Task 19 — The relay supervisor, the two Claude Code flags, and session wiring

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 14, 18 · **Hours:** 2.5

## Why

Task 18 built the tunnel. This task **opens** it — one channel per running Claude Code session,
dialled by the control plane — serves `canvas/mcp` on the far end with the session stamped on every
request (architecture rule 4), and tells the CLI where the server is, through its **documented**
flags beside the ones `claudeArgv` already emits (`argv.go:80-125`). The CLI is not modified,
wrapped or parsed (G5).

## Files

- `control-plane/go.mod`, `go.sum` — `require github.com/kpmg-au-emu/panacea-app/canvas v0.0.0-00010101000000-000000000000` and `replace github.com/kpmg-au-emu/panacea-app/canvas => ../canvas`
- `control-plane/internal/mcprelay/tunnel.go`, `supervisor.go`, `dial.go` + tests (new)
- `control-plane/internal/harness/harness.go`, `argv.go`, `argv_test.go`
- `control-plane/internal/api/api.go` — `Deps.CanvasMCP bool` (one field; task 20a adds the next)
- `control-plane/internal/api/sessions.go` — `CanvasMCP` in create (`:342`) and restart (`:925`)

## Contracts

```go
// control-plane/internal/harness/harness.go — Request and Descriptor each gain one field

type Request struct {
	// … the seven existing fields, unchanged …
	CanvasMCP bool // point the harness at the canvas MCP relay (phase 5)
}

// in Descriptor:
	// SupportsMCPConfig is whether the harness takes an MCP server through a
	// documented launch flag. Claude Code only in phase 5 (seam S5-Hx.1).
	SupportsMCPConfig bool `json:"supportsMcpConfig"`
```

```go
// control-plane/internal/harness/argv.go

// CanvasMCPConfig is the --mcp-config value, exactly. The URL is the relay's
// listener inside the sandbox (mcprelay.ListenAddr, task 18).
const CanvasMCPConfig = `{"mcpServers":{"panacea-canvas":{"type":"http","url":"http://127.0.0.1:8731/mcp"}}}`

// CanvasAllowedTools pre-allows every tool of that server (plan P5-D, OQ-P5-1).
const CanvasAllowedTools = "mcp__panacea-canvas"

// validateMCP runs after validatePrompt. A CanvasMCP request is refused with
// ErrUnsupportedOption when !d.SupportsMCPConfig, and with ErrConversationID
// when r.Conversation is "": both flags may take several values, so each
// must be followed by an option, and --session-id is the one every Claude
// Code session carries (sessions.go:339, :924).
func validateMCP(d Descriptor, r Request) error

// claudeArgv, when r.CanvasMCP, emits FIRST:
//   --mcp-config <CanvasMCPConfig> --allowedTools <CanvasAllowedTools>
// and then the flags it emits today, in today's order. Nothing else changes.
```

```go
// control-plane/internal/mcprelay/dial.go

// DialChannel opens ws://<endpoint>/mcp-channel with "Authorization: Bearer
// <token>" — the session's attach token from the registry, the credential
// /attach uses (backend.Dial, dial.go:38-66, is the model) — and applies
// SetReadLimit(MaxFrameBytes).
func DialChannel(ctx context.Context, endpoint, token string) (*websocket.Conn, error)
```

```go
// control-plane/internal/mcprelay/tunnel.go

// Tunnel is a net.Listener over one channel: Accept returns a net.Conn per
// OPEN frame. It implements task 18's flow control from this side.
type Tunnel struct{ /* unexported */ }

func NewTunnel(conn *websocket.Conn, log *slog.Logger) *Tunnel
func (t *Tunnel) Accept() (net.Conn, error)
func (t *Tunnel) Close() error
func (t *Tunnel) Addr() net.Addr // a fixed "mcprelay" address
func (t *Tunnel) Done() <-chan struct{} // closed when the channel ends
```

```go
// control-plane/internal/mcprelay/supervisor.go

type SupervisorConfig struct {
	Sessions registry.Store
	Handler  http.Handler                  // the canvas MCP server's Handler (task 14), wrapped below
	Release  func(sessionID string)        // authority.Service.ReleaseSession (task 12)
	Interval time.Duration                 // 0 → 2 s
	Log      *slog.Logger
}

// Supervisor keeps exactly one channel open for every session that is
// running or idle, whose harness is claude-code, and whose record has an
// Endpoint and an AttachToken. Per channel it runs an http.Server on the
// Tunnel whose handler stamps mcp.WithIdentity from the session record —
// Author{Kind: session, UserID: "local", SessionID: s.ID, Handle: s.Title},
// ProjectID, BranchID — read from the registry on EVERY request, so a
// rename is seen at once and an exited session is refused. A channel whose
// session has gone, exited or changed sandbox is closed; after its last
// close, Release(sessionID) drops the session's claims. A failed dial is
// retried on the next interval with backoff to 30 s, logged once per session.
type Supervisor struct{ /* unexported */ }

func NewSupervisor(c SupervisorConfig) (*Supervisor, error)
func (s *Supervisor) Run(ctx context.Context) // until ctx is done; then closes every channel
func (s *Supervisor) Channels() []string       // session ids with a live channel, sorted (for /healthz-style diagnostics and tests)
```

```go
// control-plane/internal/api/api.go — Deps gains one field
	// CanvasMCP points every Claude Code session at the canvas MCP relay
	// (phase 5). From PANACEA_APP_CANVAS_MCP (default on, task 20b).
	CanvasMCP bool

// control-plane/internal/api/sessions.go — both harness.Build calls gain
//	CanvasMCP: a.deps.CanvasMCP && desc.SupportsMCPConfig,
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestClaudeArgv_canvasFlagsFirst` | with `CanvasMCP`, argv[0:4] is exactly `--mcp-config`, `CanvasMCPConfig`, `--allowedTools`, `mcp__panacea-canvas`, and argv[4] starts with `--` | the variadic flags swallowing the opening prompt |
| `TestClaudeArgv_promptStillLastAndWhole` | the opening prompt `"add tests, then refactor"` is the last element, whole (`argv.go:24-29`'s rule) | the MCP flags displacing the prompt |
| `TestClaudeArgv_noCanvas` | without `CanvasMCP`, argv equals phase 2's for the same request, element for element | a phase 2 regression |
| `TestBuild_mcpNeedsConversation` | `CanvasMCP` with no `Conversation` → `ErrConversationID` | a bare `--allowedTools` directly before the prompt |
| `TestBuild_shellRefusesMCP` | the shell harness with `CanvasMCP` → `ErrUnsupportedOption` | a terminal given MCP flags |
| `TestSessions_createAndRestartCarryFlag` | `handleSessionCreate` and `handleSessionRestart` both store argv with the flags when `Deps.CanvasMCP`; neither when false | restart dropping the canvas (the restart path rebuilds argv, `sessions.go:921-936`) |
| `TestSupervisor_oneChannelPerSession` | three running sessions against a fake sandboxd → three channels; an exited one → closed, `Release` called | a channel per poll (a leak) |
| `TestSupervisor_identityPerRequest` | a request through the tunnel reaches the handler with the session's id, title, project and branch; after a rename, the next request carries the new title | identity cached at dial time (a stale handle in every row) |
| `TestSupervisor_exitedRefused` | a session whose record turns `exited` mid-channel: the next request gets 503 before the handler | a dead session still drawing |
| `TestSupervisor_sandboxChanged` | a restart changes `SandboxID` and `Endpoint`: the old channel closes, a new one opens | a channel into a destroyed sandbox |
| `TestSupervisor_noShell` | a running shell session gets no channel | MCP served to a terminal |
| `TestTunnel_servesHTTP` | `http.Serve(tunnel, h)` answers a POST written as raw bytes into a stream from the sandboxd side (task 18's listener, in-process) | framing correct but HTTP never served |

## Steps

1. **Evidence first.** `docker run --rm panacea-app/sandbox:dev claude --help` (the image the
   sessions run; Claude Code 2.1.274, `images/sandbox/Dockerfile:17`). Record in `argv.go`'s comment,
   as the existing flags are recorded (`argv.go:91-97`), the help text for `--mcp-config` and
   `--allowedTools`, and whether each is variadic. **If either flag is absent or spelled otherwise,
   stop**: the plan's P5-D and the relay's usefulness both depend on it; report to the orchestrator.
2. Go module: add the `require` and `replace` to `control-plane/go.mod`; `go mod tidy` in
   `control-plane` with `GOFLAGS=-mod=mod GOPROXY=off`.
3. Harness tests, then the harness change; sessions tests, then the two lines.
4. Tunnel and supervisor, tests first, against task 18's listener in-process.
5. **One real run.** With the control plane running locally and task 20b not yet landed, wire a
   throwaway handler that answers `initialize` and `tools/list` with one `ping`-like tool; start a
   Claude Code session; in its terminal run `/mcp`. Record what the CLI shows in the report below.
   Remove the throwaway wiring before committing.
6. Write `docs/superpowers/planning/phase-5/relay-first-run.md`: the `--help` text, what `/mcp`
   showed, the MCP protocol version the CLI sent in `initialize` (from a debug log line of the
   throwaway handler — the one place this phase logs a request's body, and only for this run).
7. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./control-plane/internal/harness/... ./control-plane/internal/mcprelay/... ./control-plane/internal/api/... -race -count=1
```
Expected: three `ok` lines.

```bash
go build ./control-plane/... && go list -deps ./control-plane/cmd/api | grep -c "panacea-app/canvas/"
```
Expected: a count ≥ 1 (the control plane now links the canvas module) — and `go list -deps ./control-plane/cmd/api | grep -c "internal/hub/server"` still `0` (the Hub's boundary, contract §2).

## Not claimed

- The MCP server's tools are wired in task 20b; step 5's throwaway proves the path, not the tools.
- Whether `--allowedTools mcp__panacea-canvas` really suppresses every prompt is observed in step 5
  as far as one tool goes; task 30 observes it over a whole design session.
- Codex, OpenCode and Gemini CLI get no MCP (seam S5-Hx.1).
