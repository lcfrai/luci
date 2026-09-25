# Task 18 — The relay: tunnel framing in both modules, `sandboxd`'s listener and `/mcp-channel`

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** — · **Hours:** 3

## Why

An agent in a sandbox must reach the MCP server in the control plane, and the control plane listens
on loopback only (spec §7.7, OQ-4). Today the control plane **dials into** `sandboxd`
(`control-plane/internal/backend/dial.go:38-66`, `ws://<endpoint>/attach` with a bearer token) and
nothing in the sandbox reaches the host. The relay keeps that direction:

1. `sandboxd` listens on `127.0.0.1:8731` **inside** the sandbox (G4).
2. The control plane dials `ws://<endpoint>/mcp-channel` with the attach token, the same credential
   `/attach` checks (`Server.authorised` in `sandboxd/internal/server/server.go`).
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
- `sandboxd/cmd/sandboxd/environ_linux.go`, `environ_darwin.go`, `serve_linux_test.go` — fix round
  1: the session never holds the attach token (review M3)
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
	// Built (393515c), beyond the first draft: the numbers the rules below already used, named.
	WindowAck     = 64 << 10 // a receiver sends WINDOW once it has handed this many bytes to its reader
	MaxReason     = 200      // bytes of a RESET's UTF-8 reason
	MaxPing       = 8        // bytes of a PING's payload, echoed in its PONG
)

type Frame struct {
	Kind    Kind
	Stream  uint32
	Payload []byte
}

var ErrFrame = errors.New("mcprelay: malformed frame")

// Encode and Decode refuse the same frames, all wrapping ErrFrame: short input (Decode), an
// unknown kind, a payload past MaxPayload, a kind on the wrong stream (OPEN DATA CLOSE RESET
// WINDOW on 0; PING PONG off 0), and a payload the kind does not allow (OPEN/CLOSE non-empty,
// empty DATA, RESET not UTF-8 or past MaxReason, WINDOW not 4 bytes, PING/PONG past MaxPing).
// A sender cannot put on the wire what its peer rejects.
func Encode(f Frame) ([]byte, error)
func Decode(b []byte) (Frame, error)       // a decoded Payload aliases b
func WindowPayload(n uint32) []byte        // a WINDOW frame's payload
func WindowIncrement(f Frame) uint32       // a decoded WINDOW frame's increment
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
02010203056869 {Kind:2 Stream:16909061 Payload:"hi"}
```

The last line is beyond the first draft: every other line uses stream 0 or 1, which a codec
reading only the low id byte would pass. The file as built also carries two more comment lines
(the Go syntax is `%+v` with a printable-ASCII payload quoted, so a codec that round-trips its
own mistake still fails the text).

**Amended in fix round 1** (review M4, M5, M6): `frame.go`, still identical in both modules,
also carries the **stream rules** each side's channel must follow, because the two channels are
written apart (this task's `channel.go`, task 19's peer) and the twin tests pin only the codec:

- Only sandboxd sends OPEN. An OPEN from the control plane, for a new id or a live one, is a
  protocol error: sandboxd closes the channel with `StatusProtocolError` (it used to answer with
  RESET, which let a peer flooding OPENs grow sandboxd's goroutines without bound).
- A DATA, CLOSE, RESET or WINDOW for a stream that has ended here, or was never opened, is
  dropped, not an error. sandboxd queues RESET but writes DATA, WINDOW and CLOSE directly, so a
  DATA or WINDOW can follow a RESET for the same stream.
- The flow-control rules below, and the fixed RESET reasons.
- A peer closed with 4001 that it did not cause should surface it, not stop quietly: only a
  holder of the attach token can attach a channel.

The fixed reasons move from `channel.go` into `frame.go` (unexported, so task 19's peer in the
same package uses the same strings), with `knownReason(string) bool`:

```go
const (
	reasonTooMany        = "too many streams"
	reasonWindow         = "window exceeded"
	reasonBadWindow      = "window acknowledges bytes never sent"
	reasonDataAfterClose = "data after close"
	reasonLocalRead      = "local read failed"
	reasonLocalWrite     = "local write failed"
	reasonUnexpectedOpen = "sandboxd opens every stream"
	reasonIDs            = "stream ids exhausted"
)
```

```go
// sandboxd/internal/mcprelay/listener.go

// Listener accepts local connections on ListenAddr and holds each until a
// channel is attached, for at most HoldTimeout: the harness starts with the
// PTY (server.New starts it) and dials MCP at once, before
// the control plane has even seen /healthz answer. A connection still
// unassigned after HoldTimeout is closed.
type Listener struct{ /* unexported */ }

const HoldTimeout = 30 * time.Second

const Off = "off"                                    // Listen(Off, …) returns nil, nil
const StatusSuperseded websocket.StatusCode = 4001

// Listen binds addr, which must be a loopback IP and port; any other address,
// a host name included, is an error (G4), checked before the bind and again on
// the bound address. "off" disables the relay.
func Listen(addr string, log *slog.Logger) (*Listener, error)
func (l *Listener) Addr() net.Addr
func (l *Listener) Close() error

// Serve runs one channel: every held and every new connection becomes a
// stream over conn until conn fails. A second channel replaces the first,
// which is closed with status 4001 "superseded"; streams of the old channel
// are reset. Serve returns when conn closes. Serve sets conn's read limit to
// MaxFrameBytes itself, so no caller can forget it.
func (l *Listener) Serve(ctx context.Context, conn *websocket.Conn) error
```

**As built** (`channel.go`, `listener.go`): a stream's local connection is aborted with a TCP
reset (`SO_LINGER` 0), never a clean EOF a client could take for a finished response. A 65th
live stream, or a 65th connection held before any channel, never becomes a stream: sandboxd
resets the local connection and logs `too many streams`; no RESET frame exists for it, because
no stream id was ever sent. (The control plane's side still answers a 65th OPEN with RESET
"too many streams", task 19.) A superseded or failed channel resets its streams' local
connections and sends nothing on the socket. The read loop only queues: DATA past the window,
WINDOW for bytes never sent, and DATA after CLOSE each reset the stream with a fixed reason.

```go
// sandboxd/internal/server/server.go — Config gains one field, New one route

	// MCP is the relay's listener; nil disables /mcp-channel (answers 404).
	MCP *mcprelay.Listener

// in New, beside /attach, and only when cfg.MCP != nil (nil: the route does not exist, 404):
//	s.mux.HandleFunc(mcprelay.ChannelPath, s.mcpChannel)
// mcpChannel: authorised(r) exactly as attach (401 without the bearer), then
// websocket.Accept with nil options — the origin check ON, unlike /attach: the control
// plane dials from Go and sends no Origin, which Accept allows, and a browser's is refused
// (403) — then SetReadLimit(mcprelay.MaxFrameBytes), then l.Serve.
// New neither opens nor closes cfg.MCP: its caller owns it.
```

```go
// sandboxd/cmd/sandboxd/main.go — one flag, defined through a helper so a test can pin its default
func mcpListenFlag(fs *flag.FlagSet) *string {
	return fs.String("mcp-listen", mcprelay.ListenAddr, `loopback address for the canvas MCP relay, or "off"`)
}
mcpListen := mcpListenFlag(flag.CommandLine)
srv, err := newServer(*command, argv, *token, *allowUnauthenticated, *mcpListen) // main.go:56 had four

// serve_posix.go: mcprelay.Listen(mcpListen) FIRST ("off" → nil → Config.MCP nil), its error
// returned; then server.New, which starts the PTY. The order matters: the harness dials MCP as
// the PTY starts it, and a bound port holds that dial where an unbound one refuses it. When
// server.New fails the listener is closed, so a restarted sandboxd can bind the port again.
func newServer(command string, argv []string, token string, allowUnauthenticated bool, mcpListen string) (http.Handler, error)

// serve_other.go (review M5): a string, not a *mcprelay.Listener, so the stub imports nothing new
// and still returns only its existing error.
func newServer(string, []string, string, bool, string) (http.Handler, error)
```

The listener is opened in `serve_posix.go`, not `main.go`, so the stub never opens a port it cannot
serve.

The image's entrypoint (`images/sandbox/entrypoint.sh`) is **unchanged**: the default is right, and
`backend.EnvEntries`' fixed three variables (`backend.go:181-211`; the draft said 177-207) stay three.

**Flow control.** Each side tracks, per stream, bytes sent minus bytes acknowledged by `WINDOW`, and
never exceeds `InitialWindow`; a receiver sends `WINDOW` after it has handed at least 64 KiB to its
reader. One slow stream therefore cannot stall another — the head-of-line risk of a shared socket.
A frame that would exceed the window is a protocol error: `RESET "window exceeded"` for the stream.

**Payload-blind** (G5, spec §7.7): `sandboxd` logs per stream only its id, byte counts and close
reason; never a payload byte. **Amended in fix round 1** (review M4): a peer's RESET reason is the
peer's text, so sandboxd logs it only when it is one of the fixed reasons, and any other as
`peer reset: a N-byte reason`. G5 then holds whatever the peer sends.

**The session never holds the attach token** (fix round 1, review M3). The token now opens
`/mcp-channel` as well as `/attach`, and a session process that held it could supersede the
control plane's channel (4001) and become the far end of the harness's MCP connections. So
`serve_posix.go`'s `newServer` starts the session with `withoutAttachToken(os.Environ())` and
first calls `protectEnviron()` (`environ_linux.go`: `prctl(PR_SET_DUMPABLE, 0)`;
`environ_darwin.go`: a no-op, darwin being a development build). The review's remedy stopped at the
environment. That is not enough on its own: the session runs as sandboxd's user (`dev`, uid 1001,
`images/sandbox/Dockerfile`), and `/proc/<sandboxd pid>/environ` keeps the environment sandboxd
was started with, token included. The Linux container read it (red run). A non-dumpable process's
`/proc` files are root's and need `CAP_SYS_PTRACE`, which the image grants none. The session's
`execve` makes it dumpable again, so only sandboxd changes: it writes no core dump and cannot be
ptraced by its session.

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
| `TestMCPChannel_requiresToken` | `/mcp-channel` without the bearer → 401 | the relay open to any local process (the published port is on loopback, `server.Config.AttachToken`'s comment) |
| `TestMCPChannel_disabled` | `Config.MCP == nil` → 404 | a half-configured relay |
| `TestLogs_payloadBlind` | a captured log after a stream carrying `"secret"` has no `secret` | payload in sandbox logs |

**Added in the build**, each for a mutant the table above let through or did not name:

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestFrameSourceIdentical` (control-plane side) | `frame.go` and `frame_test.go` equal the sandboxd twin byte for byte | a check one copy alone makes |
| `TestEncode_rejects`, `TestCodec_limitsAccepted`, `TestDecode_hostileNeverPanics` | Encode refuses what Decode refuses; the limits themselves are accepted; 20,000 random inputs never panic | a sender putting on the wire what its peer rejects |
| `TestPing_echoedAsPong` | PING's 8 bytes come back as PONG | a dead keepalive |
| `TestWindow_protocolErrors` | DATA past the window, WINDOW for bytes never sent, DATA after CLOSE → RESET with the fixed reason | a peer past the window left unchecked |
| `TestListener_survivesAnAcceptError` | a listener whose first Accept fails still serves the next connection | one transient accept error ending the relay (mutation S2 survived without it) |
| `TestMCPChannel_refusesABrowserOrigin` | the right token with `Origin: https://evil.example` → 403 | the origin check turned off (mutation S1 survived without it) |
| `TestMCPListenFlag` | `-mcp-listen` defaults to `127.0.0.1:8731` and parses `off` | the image's sandboxes silently without the relay |
| `TestNewServer_mcpListen` | `0.0.0.0:0` refused; `off` → 404; loopback → 401 | the flag never reaching the server |
| `TestNewServer_releasesTheRelayOnFailure` | a failed `server.New` leaves the port free | a restarted sandboxd unable to bind |

**Added in fix round 1** (review `task-t18-review.md`), each for a mutant or a defect the tables
above let through:

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestChannelEndedByItsPeer` | the peer drops the channel: an idle stream is reset, `Serve` returns, the next connection is held and served on the next channel | MA: `Serve` without its final `ch.fail` (review I1) |
| `TestFlowControl_partialWindow` | a full window, then a 1,000-byte WINDOW: sandboxd sends exactly 1,000 more bytes | MG: `upstream` reading a whole `MaxPayload` whenever the window has room (review M1; survived on Windows) |
| `TestLogs_payloadBlind` (extended) | a peer RESET with reason `secret` is logged as `peer reset: a 6-byte reason`; a fixed reason is logged verbatim | the peer's reason logged verbatim (review M4) |
| `TestPeerResetReason` | every fixed reason is quoted as itself; an empty, unknown or near-miss reason only by its length | N9: `knownReason` missing a fixed reason (it survived `TestLogs_payloadBlind`, which quotes one) |
| `TestPeerOpen_endsTheChannel` | a control-plane OPEN, for a new id or a live one, closes the channel with `StatusProtocolError` and resets its streams | the RESET answer to a peer OPEN, unbounded (review M6; MF survived) |
| `TestListener_closeEndsItsChannel` | `Close` ends the live channel with 1001 and resets its stream; a channel attached after `Close` is refused and leaves no writer goroutine | MD, and the `Serve`-after-`Close` goroutine leak (review M7) |
| `TestWithoutAttachToken` | the session's environment is sandboxd's minus every `PANACEA_APP_ATTACH_TOKEN` entry, and never nil | the token reaching the session (review M3) |
| `TestNewServer_sessionEnvLacksTheToken` (posix) | a PTY child's `env` has no token and keeps sandboxd's other variables | the same, end to end |
| `TestNewServer_sessionCannotReadTheTokenFromProc` (linux) | the test re-runs itself with the token in its initial environment; its PTY child cannot read it from `/proc/$PPID/environ` (skipped under `CAP_SYS_PTRACE`) | `protectEnviron` not called |

`TestMaxStreams` runs twice: on a channel (and a new connection opens once one stream ends, so
the bound is on live streams) and for connections held before any channel. The 65th is refused
with a TCP reset, which may land before `Dial` returns ("connect: connection reset by peer" on
Linux); the test accepts either. `TestSecondChannelSupersedes` also checks the superseded
`Serve` returns and a stream on the new channel moves.

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
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/sandboxd && GOWORK=off GOFLAGS=-mod=vendor go build ./cmd/sandboxd && echo vendored-ok
```
Expected: `vendored-ok` — the image's build (`Dockerfile:8`) still works. **Corrected in the
build:** the draft had no `GOWORK=off`, and from `sandboxd/` the root `go.work` is found, so
`-mod=vendor` means workspace vendoring and the build fails ("inconsistent vendoring … run: go
work vendor", rc 1, measured). The image copies `sandboxd` alone (`Dockerfile:7`), with no
`go.work`; `GOWORK=off` is that build. (`sandboxd.exe` is git-ignored, `*.exe`.)

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && MSYS_NO_PATHCONV=1 docker run --rm --name pa-p5-t18-gotest --network none -v "$(cygpath -w "$PWD"):/src" -w /src/sandboxd -e GOFLAGS=-mod=vendor -e GOWORK=off -e GOTOOLCHAIN=local golang:1.23-bookworm go test ./... -race -count=1
```
Expected: `ok` for every sandboxd package, `internal/server` and `cmd/sandboxd`'s posix tests
included. **Added in the build:** the first command cannot run the server tests on this Windows
machine, and waiting for CI would leave `/mcp-channel` untested before commit. The image is
local (`golang:1.23-bookworm`), `--network none` works because sandboxd is vendored, and it pins
go 1.23, which this machine's go1.26.2 does not.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go build ./sandboxd/... && GOOS=linux go build ./sandboxd/... && GOOS=darwin go build ./sandboxd/... && echo both-builds-ok
```
Expected: `both-builds-ok` — the first compiles `serve_other.go` (Windows), the second
`serve_posix.go`, so a signature changed in one file only fails here (review M5). The darwin
build is added in fix round 1: `protectEnviron` has one file per OS (`environ_linux.go`,
`environ_darwin.go`), and a darwin build without its file fails only there.

## Not claimed

- The control plane's side of the channel — dialling, serving HTTP on the streams, stamping the
  session — is task 19.
- Kata: the path is the same on `<podIP>:8080` (seam S5-P8.1); not exercised.
