# Task 18 — The relay: tunnel framing in both modules, `sandboxd`'s listener and `/mcp-channel`

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** — · **Hours:** 3

## Why

An agent in a sandbox must reach the MCP server in the control plane, and the control plane listens
on loopback only (spec §7.7, OQ-4). Today the control plane **dials into** `sandboxd`
(`control-plane/internal/backend/dial.go:38-66`, `ws://<endpoint>/attach` with a bearer token) and
nothing in the sandbox reaches the host. The relay keeps that direction:

1. `sandboxd` listens on `127.0.0.1:8731` **inside** the sandbox (G4).
2. The control plane dials `ws://<endpoint>/mcp-channel` with the attach token, the same credential
   `/attach` checks (`sandboxd/internal/server/server.go:105-116`).
3. Each TCP connection accepted on 8731 becomes one **stream** over that WebSocket, as opaque bytes.
   `sandboxd` never parses them (G5); the control plane serves HTTP on the far end (task 19).

No token enters the sandbox beyond the attach token it already holds (spec R7 removed). The framing
is ours because `hashicorp/yamux` is MPL-2.0 (P5-C). It exists twice — once per module, as
`MaxMessageBytes` does (`backend.go:29-38`, "SHARED RULE, THREE SITES") — pinned by one set of
golden frames checked into both.

## Files

- `sandboxd/internal/mcprelay/frame.go`, `listener.go`, `channel.go` + tests (new)
- `sandboxd/internal/server/server.go` — `Config.MCP`, the `/mcp-channel` route
- `sandboxd/cmd/sandboxd/main.go`, `serve_posix.go` — the `-mcp-listen` flag
- `sandboxd/cmd/sandboxd/serve_other.go` — the Windows stub (`//go:build !linux && !darwin`,
  `serve_other.go:1`) gains the same fifth parameter, or `go build ./sandboxd/...` fails on the
  Windows machine this task's acceptance runs on (review M5)
- `control-plane/internal/mcprelay/frame.go` + `frame_test.go` (new; the twin)
- `control-plane/internal/mcprelay/testdata/frames.golden` and `sandboxd/internal/mcprelay/testdata/frames.golden` — identical files

## Contracts

```go
// frame.go — identical in both modules, byte for byte except the package clause
package mcprelay

// Wire format, one frame per WebSocket BINARY message:
//
//   byte 0      kind
//   bytes 1-4   stream id, big-endian uint32 (odd: opened by sandboxd; 0: the channel itself)
//   bytes 5-    payload
//
// Kinds:
//   0x01 OPEN    sandboxd → cp   a new local connection; payload empty
//   0x02 DATA    both            payload 1..MaxPayload bytes of the stream
//   0x03 CLOSE   both            the sender will send no more on this stream (half-close); payload empty
//   0x04 RESET   both            abort the stream; payload a UTF-8 reason ≤ 200 bytes
//   0x05 WINDOW  both            payload a big-endian uint32: bytes the receiver has consumed and may be sent again
//   0x06 PING    both            stream 0; payload ≤ 8 bytes, echoed as PONG
//   0x07 PONG    both            stream 0
type Kind byte

const (
	KindOpen   Kind = 0x01
	KindData   Kind = 0x02
	KindClose  Kind = 0x03
	KindReset  Kind = 0x04
	KindWindow Kind = 0x05
	KindPing   Kind = 0x06
	KindPong   Kind = 0x07
)

const (
	HeaderBytes   = 5
	MaxPayload    = 32 << 10  // 32 KiB of stream data per frame
	MaxFrameBytes = HeaderBytes + MaxPayload
	InitialWindow = 256 << 10 // per stream: a sender may have this many unacknowledged bytes in flight
	MaxStreams    = 64        // a 65th OPEN is answered with RESET "too many streams"
	ChannelPath   = "/mcp-channel"
	ListenAddr    = "127.0.0.1:8731" // inside the sandbox; the URL the harness is given (task 19)
)

type Frame struct {
	Kind    Kind
	Stream  uint32
	Payload []byte
}

var ErrFrame = errors.New("mcprelay: malformed frame")

func Encode(f Frame) ([]byte, error) // errs on an oversized payload or a kind/stream mismatch
func Decode(b []byte) (Frame, error) // ErrFrame on short input, unknown kind, oversized payload
```

```
# testdata/frames.golden — one frame per line: a hex encoding, a space, the Frame in Go syntax.
# Both modules' TestGolden decode every line and re-encode it to the same hex.
0100000001 {Kind:1 Stream:1 Payload:[]}
020000000168656c6c6f {Kind:2 Stream:1 Payload:"hello"}
0300000001 {Kind:3 Stream:1 Payload:[]}
04000000016964 {Kind:4 Stream:1 Payload:"id"}
050000000100040000 {Kind:5 Stream:1 Payload:[0 4 0 0]}
060000000001 {Kind:6 Stream:0 Payload:[1]}
070000000001 {Kind:7 Stream:0 Payload:[1]}
```

```go
// sandboxd/internal/mcprelay/listener.go

// Listener accepts local connections on ListenAddr and holds each until a
// channel is attached, for at most HoldTimeout: the harness starts with the
// PTY (server.New starts it, server.go:69-90) and dials MCP at once, before
// the control plane has even seen /healthz answer. A connection still
// unassigned after HoldTimeout is closed.
type Listener struct{ /* unexported */ }

const HoldTimeout = 30 * time.Second

// Listen binds addr, which must be a loopback address; any other is an error
// (G4). "off" disables the relay.
func Listen(addr string, log *slog.Logger) (*Listener, error)
func (l *Listener) Close() error

// Serve runs one channel: every held and every new connection becomes a
// stream over conn until conn fails. A second channel replaces the first,
// which is closed with status 4001 "superseded"; streams of the old channel
// are reset. Serve returns when conn closes.
func (l *Listener) Serve(ctx context.Context, conn *websocket.Conn) error
```

```go
// sandboxd/internal/server/server.go — Config gains one field, New one route

	// MCP is the relay's listener; nil disables /mcp-channel (answers 404).
	MCP *mcprelay.Listener

// in New, beside /attach:
//	s.mux.HandleFunc(mcprelay.ChannelPath, s.mcpChannel)
// mcpChannel: authorised(r) exactly as attach (401 without the bearer), then
// websocket.Accept, SetReadLimit(mcprelay.MaxFrameBytes), then l.Serve.
```

```go
// sandboxd/cmd/sandboxd/main.go — one flag
mcpListen := flag.String("mcp-listen", mcprelay.ListenAddr, `loopback address for the canvas MCP relay, or "off"`)
srv, err := newServer(*command, argv, *token, *allowUnauthenticated, *mcpListen) // main.go:56 today has four

// serve_posix.go: "off" → Config.MCP nil; else mcprelay.Listen(mcpListen), its error returned.
func newServer(command string, argv []string, token string, allowUnauthenticated bool, mcpListen string) (http.Handler, error)

// serve_other.go (review M5): a string, not a *mcprelay.Listener, so the stub imports nothing new
// and still returns only its existing error.
func newServer(string, []string, string, bool, string) (http.Handler, error)
```

The listener is opened in `serve_posix.go`, not `main.go`, so the stub never opens a port it cannot
serve.

The image's entrypoint (`images/sandbox/entrypoint.sh`) is **unchanged**: the default is right, and
`backend.EnvEntries`' fixed three variables (`backend.go:177-207`) stay three.

**Flow control.** Each side tracks, per stream, bytes sent minus bytes acknowledged by `WINDOW`, and
never exceeds `InitialWindow`; a receiver sends `WINDOW` after it has handed at least 64 KiB to its
reader. One slow stream therefore cannot stall another — the head-of-line risk of a shared socket.
A frame that would exceed the window is a protocol error: `RESET "window exceeded"` for the stream.

**Payload-blind** (G5, spec §7.7): `sandboxd` logs per stream only its id, byte counts and close
reason; never a payload byte.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestGolden` (**both modules**) | every line of `frames.golden` decodes and re-encodes to its hex | the two codecs drifting |
| `TestGoldenFilesIdentical` (control-plane side) | its golden file equals `../../../sandboxd/internal/mcprelay/testdata/frames.golden` byte for byte | one copy edited alone |
| `TestDecode_rejects` | 4-byte input, kind `0x09`, a 32 KiB + 1 payload, `DATA` on stream 0 → `ErrFrame` | a panic on hostile input |
| `TestListen_loopbackOnly` | `Listen("0.0.0.0:8731")` and `Listen(":8731")` → error | the listener reachable from the sandbox's network |
| `TestListener_holdsUntilChannel` | a client connects and writes a request; the channel attaches 2 s later; the bytes arrive intact | the harness's first MCP connection lost at start |
| `TestListener_holdTimeout` (injected clock) | no channel for 30 s → the client's connection is closed | connections parked for ever |
| `TestRoundTrip_bytes` | 5 concurrent streams × 1 MiB each way, through a real WebSocket pair, arrive byte-identical | framing that corrupts on a split |
| `TestFlowControl_noHeadOfLine` | one stream whose reader never reads; another stream still moves 1 MiB | a stalled stream stalling the channel |
| `TestMaxStreams` | the 65th connection is reset with "too many streams" | unbounded streams |
| `TestSecondChannelSupersedes` | a second `Serve` closes the first with 4001 and resets its streams | two control planes both serving one sandbox |
| `TestMCPChannel_requiresToken` | `/mcp-channel` without the bearer → 401 | the relay open to any local process (the published port is on loopback, `server.go:33-39`) |
| `TestMCPChannel_disabled` | `Config.MCP == nil` → 404 | a half-configured relay |
| `TestLogs_payloadBlind` | a captured log after a stream carrying `"secret"` has no `secret` | payload in sandbox logs |

## Steps

1. Write `frame.go` and `frames.golden` in `sandboxd`, then copy both to the control plane (change
   only the package clause); the `TestGoldenFilesIdentical` test guards the copy.
2. Listener, channel, server route, flag — tests first.
3. `cd sandboxd && go mod vendor` is **not** needed: nothing new is required (G1).
4. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./sandboxd/... ./control-plane/internal/mcprelay/... -race -count=1
```
Expected: `ok` for `sandboxd/internal/mcprelay`, `sandboxd/internal/server`, `sandboxd/cmd/sandboxd`
and `control-plane/internal/mcprelay`. (`sandboxd/internal/server` is `//go:build linux || darwin`,
`server.go:1`: on Windows its tests do not build; CI's Ubuntu agent runs them.)

```bash
cd sandboxd && GOFLAGS=-mod=vendor go build ./cmd/sandboxd && echo vendored-ok
```
Expected: `vendored-ok` — the image's build (`Dockerfile:8`) still works.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go build ./sandboxd/... && GOOS=linux go build ./sandboxd/... && echo both-builds-ok
```
Expected: `both-builds-ok` — the first compiles `serve_other.go` (Windows), the second
`serve_posix.go`, so a signature changed in one file only fails here (review M5).

## Not claimed

- The control plane's side of the channel — dialling, serving HTTP on the streams, stamping the
  session — is task 19.
- Kata: the path is the same on `<podIP>:8080` (seam S5-P8.1); not exercised.
