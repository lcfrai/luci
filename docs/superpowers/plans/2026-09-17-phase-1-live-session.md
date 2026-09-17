# Phase 1: One Live Session Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Tauri desktop window renders a real, unmodified Claude Code session running inside a Kata microVM on the Panacea dev AKS, and keystrokes typed in the window reach that session.

**Architecture:** Three processes. `sandboxd` runs inside the sandbox pod, owns a PTY, and speaks a tiny binary frame protocol over a WebSocket. `control-plane` creates the sandbox pod and proxies a browser WebSocket to it. The Tauri desktop app renders the stream with xterm.js. Nothing parses or reinterprets agent output — bytes in, bytes out.

**Tech Stack:** Go 1.23 (control plane, sandboxd), `github.com/creack/pty`, `github.com/coder/websocket`, `k8s.io/client-go`, Rust + Tauri 2, React 18 + TypeScript + Vite, `@xterm/xterm`, Terraform `azurerm`.

**Spec:** `docs/PROGRAM.md` (Phase 1), `docs/FUNCTIONAL-SPEC.md`, `docs/DESIGN-SYSTEM.md`

## Global Constraints

- Go module path prefix: `github.com/kpmg-au-emu/panacea-app`.
- Go version floor: `1.23`. Rust edition `2021`. Node `20`.
- Agent CLIs are **never** modified, wrapped, or parsed. The only transformation permitted between the PTY and xterm.js is framing.
- Kata pods **must** declare explicit CPU and memory requests and limits — the guest VM is sized from the sum, and under-requesting causes guest OOM.
- Kata pods **must not** use host networking; it is unsupported.
- No secrets in source. Local development reads `PANACEA_APP_*` environment variables.
- Every Go package has tests. **`go test ./sandboxd/... ./control-plane/...` must pass before any commit.** Note: `./...` is invalid at a Go workspace root — the root is not itself a module, so the pattern resolves to nothing and errors. Always use explicit module patterns.
- The `go` directive in both `go.mod` files and in `go.work` must read `go 1.23`, matching the CI pin. A higher directive makes a pinned 1.23 agent attempt a toolchain download, which fails on a network-restricted build agent.
- Terraform targets the **dev** estate only in this phase. Never run against `deploy/main.tf` in the Panacea repo — that is the live cluster.

---

## File Structure

| Path | Responsibility |
|---|---|
| `go.work` | Go workspace joining the two modules |
| `sandboxd/internal/wire/frame.go` | Binary frame encode/decode. No I/O. |
| `sandboxd/internal/ptysession/session.go` | Spawn a command in a PTY; read, write, resize, close |
| `sandboxd/internal/server/server.go` | WebSocket handler binding a PTY to a socket |
| `sandboxd/cmd/sandboxd/main.go` | Entrypoint; flags for command and listen address |
| `control-plane/internal/sandbox/manager.go` | Create, look up and delete sandbox pods |
| `control-plane/internal/proxy/proxy.go` | Bidirectional WebSocket relay |
| `control-plane/cmd/api/main.go` | HTTP entrypoint |
| `images/sandbox/Dockerfile` | Sandbox image: Claude Code, git, sandboxd |
| `infra/terraform/kata.tf` | Kata-enabled AKS node pool |
| `desktop/src/lib/wire.ts` | TypeScript mirror of the frame protocol |
| `desktop/src/components/Terminal.tsx` | xterm.js bound to a WebSocket |
| `desktop/src/App.tsx` | Phase 1 shell — one terminal, one connect button |

---

### Task 1: Repository scaffold and CI

**Files:**
- Create: `go.work`, `sandboxd/go.mod`, `control-plane/go.mod`, `.gitignore`, `azure-pipelines.yml`, `README.md`

**Interfaces:**
- Consumes: nothing
- Produces: a buildable Go workspace with modules `github.com/kpmg-au-emu/panacea-app/sandboxd` and `.../control-plane`

- [ ] **Step 1: Create the Go modules**

```bash
mkdir -p sandboxd/cmd/sandboxd control-plane/cmd/api
cd sandboxd && go mod init github.com/kpmg-au-emu/panacea-app/sandboxd && cd ..
cd control-plane && go mod init github.com/kpmg-au-emu/panacea-app/control-plane && cd ..
go work init ./sandboxd ./control-plane
```

- [ ] **Step 2: Add a placeholder main so the workspace builds**

Create `sandboxd/cmd/sandboxd/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("sandboxd")
}
```

Create `control-plane/cmd/api/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("panacea-app control plane")
}
```

- [ ] **Step 3: Verify the workspace builds**

Run: `go build ./sandboxd/... ./control-plane/...`
Expected: exits 0, no output.

Do **not** use `go build ./...` here. At a Go workspace root there is no enclosing module, so the pattern
fails with `directory prefix . does not contain modules listed in go.work`.

- [ ] **Step 4: Add `.gitignore`**

```
node_modules/
dist/
target/
*.exe
.env
.terraform/
*.tfstate
*.tfstate.backup
```

- [ ] **Step 5: Add the pipeline**

Create `azure-pipelines.yml`:

```yaml
trigger:
  branches:
    include: [main, dev]

pool:
  vmImage: ubuntu-latest

steps:
  - task: GoTool@0
    inputs:
      version: '1.23'
  # Explicit module patterns: ./... is invalid at a Go workspace root.
  - script: go build ./sandboxd/... ./control-plane/...
    displayName: Build
  - script: go test ./sandboxd/... ./control-plane/... -race -count=1
    displayName: Test
  - script: go vet ./sandboxd/... ./control-plane/...
    displayName: Vet
```

- [ ] **Step 6: Commit**

```bash
git add go.work sandboxd control-plane .gitignore azure-pipelines.yml
git commit -m "chore: scaffold go workspace and CI"
```

---

### Task 2: The wire protocol

A frame is one byte of type followed by payload. This is the entire contract between the desktop app, the control plane and the sandbox. Keeping it dumb is the point — it must never need to understand terminal content.

**Files:**
- Create: `sandboxd/internal/wire/frame.go`
- Test: `sandboxd/internal/wire/frame_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `wire.Kind` (uint8) with constants `wire.Stdout=0x00`, `wire.Stdin=0x01`, `wire.Resize=0x02`, `wire.Exit=0x03`
  - `wire.Encode(k Kind, payload []byte) []byte`
  - `wire.Decode(frame []byte) (Kind, []byte, error)`
  - `wire.ResizePayload{Cols, Rows uint16}` with `MarshalBinary`/`UnmarshalBinary`
  - `wire.ErrEmptyFrame`

- [ ] **Step 1: Write the failing tests**

Create `sandboxd/internal/wire/frame_test.go`:

```go
package wire_test

import (
	"bytes"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		kind    wire.Kind
		payload []byte
	}{
		{"stdout", wire.Stdout, []byte("hello\x1b[0m")},
		{"stdin", wire.Stdin, []byte("ls -la\n")},
		{"empty payload", wire.Stdin, []byte{}},
		{"binary safe", wire.Stdout, []byte{0x00, 0xff, 0x1b, 0x5b}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKind, gotPayload, err := wire.Decode(wire.Encode(tc.kind, tc.payload))
			if err != nil {
				t.Fatalf("Decode returned error: %v", err)
			}
			if gotKind != tc.kind {
				t.Errorf("kind = %v, want %v", gotKind, tc.kind)
			}
			if !bytes.Equal(gotPayload, tc.payload) {
				t.Errorf("payload = %q, want %q", gotPayload, tc.payload)
			}
		})
	}
}

func TestDecodeEmptyFrameIsError(t *testing.T) {
	if _, _, err := wire.Decode(nil); err != wire.ErrEmptyFrame {
		t.Fatalf("err = %v, want ErrEmptyFrame", err)
	}
}

func TestResizePayloadRoundTrip(t *testing.T) {
	in := wire.ResizePayload{Cols: 120, Rows: 40}
	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var out wire.ResizePayload
	if err := out.UnmarshalBinary(b); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if out != in {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestResizePayloadRejectsShortInput(t *testing.T) {
	var out wire.ResizePayload
	if err := out.UnmarshalBinary([]byte{0x01}); err == nil {
		t.Fatal("expected error for short input, got nil")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./sandboxd/internal/wire/...`
Expected: FAIL — package `wire` does not exist.

- [ ] **Step 3: Write the implementation**

Create `sandboxd/internal/wire/frame.go`:

```go
// Package wire defines the frame protocol spoken between the desktop app,
// the control plane and sandboxd. It deliberately knows nothing about
// terminal semantics: payloads are opaque bytes.
package wire

import (
	"encoding/binary"
	"errors"
)

// Kind identifies what a frame carries.
type Kind uint8

const (
	Stdout Kind = 0x00 // server -> client: raw PTY output
	Stdin  Kind = 0x01 // client -> server: raw keystrokes
	Resize Kind = 0x02 // client -> server: ResizePayload
	Exit   Kind = 0x03 // server -> client: payload is the exit reason
)

// ErrEmptyFrame is returned by Decode when given a zero-length frame.
var ErrEmptyFrame = errors.New("wire: empty frame")

// Encode prefixes payload with its kind.
func Encode(k Kind, payload []byte) []byte {
	out := make([]byte, 0, len(payload)+1)
	out = append(out, byte(k))
	return append(out, payload...)
}

// Decode splits a frame into its kind and payload. The returned payload
// aliases frame; copy it if you need to retain it.
func Decode(frame []byte) (Kind, []byte, error) {
	if len(frame) == 0 {
		return 0, nil, ErrEmptyFrame
	}
	return Kind(frame[0]), frame[1:], nil
}

// ResizePayload is the body of a Resize frame.
type ResizePayload struct {
	Cols uint16
	Rows uint16
}

// MarshalBinary encodes the payload as four big-endian bytes.
func (r ResizePayload) MarshalBinary() ([]byte, error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b[0:2], r.Cols)
	binary.BigEndian.PutUint16(b[2:4], r.Rows)
	return b, nil
}

// UnmarshalBinary decodes a payload produced by MarshalBinary.
func (r *ResizePayload) UnmarshalBinary(b []byte) error {
	if len(b) < 4 {
		return errors.New("wire: resize payload too short")
	}
	r.Cols = binary.BigEndian.Uint16(b[0:2])
	r.Rows = binary.BigEndian.Uint16(b[2:4])
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./sandboxd/internal/wire/... -v`
Expected: PASS, all four tests.

- [ ] **Step 5: Commit**

```bash
git add sandboxd/internal/wire
git commit -m "feat(wire): binary frame protocol for PTY streaming"
```

---

### Task 3: PTY session

**Files:**
- Create: `sandboxd/internal/ptysession/session.go`
- Test: `sandboxd/internal/ptysession/session_test.go`

**Interfaces:**
- Consumes: `wire.ResizePayload`
- Produces:
  - `ptysession.Start(cmd string, args []string, env []string) (*Session, error)`
  - `(*Session).Read(p []byte) (int, error)`
  - `(*Session).Write(p []byte) (int, error)`
  - `(*Session).Resize(cols, rows uint16) error`
  - `(*Session).Wait() error`
  - `(*Session).Close() error`

- [ ] **Step 1: Add the dependency**

```bash
cd sandboxd && go get github.com/creack/pty@v1.1.24 && cd ..
```

- [ ] **Step 2: Write the failing tests**

Create `sandboxd/internal/ptysession/session_test.go`:

```go
//go:build linux || darwin

package ptysession_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ptysession"
)

// readUntil reads from r until want appears or the deadline passes.
func readUntil(t *testing.T, r io.Reader, want []byte, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	var acc bytes.Buffer
	buf := make([]byte, 1024)
	for time.Now().Before(deadline) {
		n, err := r.Read(buf)
		if n > 0 {
			acc.Write(buf[:n])
			if bytes.Contains(acc.Bytes(), want) {
				return
			}
		}
		if err != nil {
			break
		}
	}
	t.Fatalf("did not see %q within %s; got %q", want, d, acc.String())
}

func TestSessionEchoesCommandOutput(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	if _, err := s.Write([]byte("echo panacea-ok\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, s, []byte("panacea-ok"), 5*time.Second)
}

func TestSessionResizeIsReportedToTheProcess(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	if err := s.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if _, err := s.Write([]byte("stty size\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, s, []byte("40 120"), 5*time.Second)
}

func TestSessionCloseTerminatesTheProcess(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	done := make(chan struct{})
	go func() { _ = s.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit within 5s of Close")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./sandboxd/internal/ptysession/...`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Write the implementation**

Create `sandboxd/internal/ptysession/session.go`:

```go
// Package ptysession runs a command attached to a pseudo-terminal.
// It is deliberately ignorant of what the command is: the same code runs
// Claude Code, Codex, or a bare shell.
package ptysession

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Session is a running process attached to a PTY.
type Session struct {
	cmd  *exec.Cmd
	file *os.File

	closeOnce sync.Once
	closeErr  error
}

// Start launches cmd with args in a new PTY. If env is nil the parent
// environment is inherited. TERM is forced to xterm-256color so agent CLIs
// emit the colours and box drawing we expect.
func Start(cmd string, args []string, env []string) (*Session, error) {
	c := exec.Command(cmd, args...)
	if env == nil {
		env = os.Environ()
	}
	c.Env = append(env, "TERM=xterm-256color")

	f, err := pty.Start(c)
	if err != nil {
		return nil, err
	}
	return &Session{cmd: c, file: f}, nil
}

// Read returns raw PTY output.
func (s *Session) Read(p []byte) (int, error) { return s.file.Read(p) }

// Write sends raw bytes to the process as if typed.
func (s *Session) Write(p []byte) (int, error) { return s.file.Write(p) }

// Resize informs the process that its terminal dimensions changed.
func (s *Session) Resize(cols, rows uint16) error {
	return pty.Setsize(s.file, &pty.Winsize{Rows: rows, Cols: cols})
}

// Wait blocks until the process exits.
func (s *Session) Wait() error { return s.cmd.Wait() }

// Close kills the process and releases the PTY. Safe to call more than once.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		s.closeErr = s.file.Close()
	})
	return s.closeErr
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./sandboxd/internal/ptysession/... -v`
Expected: PASS, three tests. (Linux/macOS only — `creack/pty` has no Windows support. Windows local sessions are a Phase 2 concern requiring ConPTY.)

- [ ] **Step 6: Commit**

```bash
git add sandboxd/internal/ptysession sandboxd/go.mod sandboxd/go.sum
git commit -m "feat(ptysession): run a command attached to a PTY"
```

---

### Task 4: sandboxd WebSocket server

**Files:**
- Create: `sandboxd/internal/server/server.go`
- Test: `sandboxd/internal/server/server_test.go`
- Modify: `sandboxd/cmd/sandboxd/main.go`

**Interfaces:**
- Consumes: `wire.*`, `ptysession.Start`
- Produces:
  - `server.Config{Command string; Args []string; Env []string}`
  - `server.New(cfg Config) *Server`
  - `(*Server).ServeHTTP(w http.ResponseWriter, r *http.Request)` — upgrades `/attach` to a WebSocket

- [ ] **Step 1: Add the dependency**

```bash
cd sandboxd && go get github.com/coder/websocket@v1.8.12 && cd ..
```

- [ ] **Step 2: Write the failing test**

Create `sandboxd/internal/server/server_test.go`:

```go
//go:build linux || darwin

package server_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

func dial(t *testing.T, url string) (*websocket.Conn, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { c.Close(websocket.StatusNormalClosure, "") })
	return c, ctx
}

func TestServerStreamsStdoutAndAcceptsStdin(t *testing.T) {
	srv := httptest.NewServer(server.New(server.Config{Command: "/bin/sh"}))
	defer srv.Close()

	c, ctx := dial(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach")

	if err := c.Write(ctx, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo panacea-ok\n"))); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	var acc bytes.Buffer
	for time.Now().Before(deadline) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		kind, payload, err := wire.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if kind != wire.Stdout {
			continue
		}
		acc.Write(payload)
		if bytes.Contains(acc.Bytes(), []byte("panacea-ok")) {
			return
		}
	}
	t.Fatalf("never saw command output; got %q", acc.String())
}

func TestServerAppliesResize(t *testing.T) {
	srv := httptest.NewServer(server.New(server.Config{Command: "/bin/sh"}))
	defer srv.Close()

	c, ctx := dial(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach")

	rp, _ := wire.ResizePayload{Cols: 120, Rows: 40}.MarshalBinary()
	if err := c.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Resize, rp)); err != nil {
		t.Fatalf("Write resize: %v", err)
	}
	// Give the resize a moment to land before asking the shell.
	time.Sleep(200 * time.Millisecond)
	if err := c.Write(ctx, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("stty size\n"))); err != nil {
		t.Fatalf("Write stty: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	var acc bytes.Buffer
	for time.Now().Before(deadline) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		_, payload, _ := wire.Decode(data)
		acc.Write(payload)
		if bytes.Contains(acc.Bytes(), []byte("40 120")) {
			return
		}
	}
	t.Fatalf("resize not reflected; got %q", acc.String())
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./sandboxd/internal/server/...`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Write the implementation**

Create `sandboxd/internal/server/server.go`:

```go
// Package server exposes a PTY over a WebSocket using the wire protocol.
package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ptysession"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

// Config describes the process a connection should attach to.
type Config struct {
	Command string
	Args    []string
	Env     []string
}

// Server upgrades HTTP requests to a PTY-backed WebSocket.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New builds a Server serving /attach.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("/attach", s.attach)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) attach(w http.ResponseWriter, r *http.Request) {
	// The control plane is the only client and it dials in-cluster, so the
	// origin check is disabled here and enforced upstream.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		slog.Error("websocket accept failed", "err", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "closing")

	sess, err := ptysession.Start(s.cfg.Command, s.cfg.Args, s.cfg.Env)
	if err != nil {
		slog.Error("pty start failed", "err", err)
		_ = conn.Write(r.Context(), websocket.MessageBinary,
			wire.Encode(wire.Exit, []byte("failed to start: "+err.Error())))
		return
	}
	defer sess.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// PTY -> socket
	go func() {
		defer cancel()
		buf := make([]byte, 32*1024)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				if werr := conn.Write(ctx, websocket.MessageBinary,
					wire.Encode(wire.Stdout, buf[:n])); werr != nil {
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.Debug("pty read ended", "err", err)
				}
				_ = conn.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Exit, []byte("session ended")))
				return
			}
		}
	}()

	// socket -> PTY
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		kind, payload, err := wire.Decode(data)
		if err != nil {
			continue
		}
		switch kind {
		case wire.Stdin:
			if _, err := sess.Write(payload); err != nil {
				return
			}
		case wire.Resize:
			var rp wire.ResizePayload
			if err := rp.UnmarshalBinary(payload); err == nil {
				_ = sess.Resize(rp.Cols, rp.Rows)
			}
		}
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./sandboxd/internal/server/... -v -race`
Expected: PASS, two tests.

- [ ] **Step 6: Write the entrypoint**

Replace `sandboxd/cmd/sandboxd/main.go`:

```go
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	command := flag.String("command", "/bin/bash", "command to run in the PTY")
	args := flag.String("args", "", "comma-separated arguments")
	flag.Parse()

	var argv []string
	if *args != "" {
		argv = strings.Split(*args, ",")
	}

	srv := server.New(server.Config{Command: *command, Args: argv})
	slog.Info("sandboxd listening", "addr", *addr, "command", *command)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Verify it runs**

Run: `go run ./sandboxd/cmd/sandboxd -command /bin/sh &` then `curl -s localhost:8080/healthz -o /dev/null -w '%{http_code}'`
Expected: `200`. Stop it with `kill %1`.

- [ ] **Step 8: Commit**

```bash
git add sandboxd
git commit -m "feat(sandboxd): expose a PTY over a websocket"
```

---

### Task 5: Sandbox container image

**Files:**
- Create: `images/sandbox/Dockerfile`, `images/sandbox/entrypoint.sh`

**Interfaces:**
- Consumes: the `sandboxd` binary
- Produces: image `panaceaappacr.azurecr.io/panacea-app/sandbox:dev` running `sandboxd` on port 8080 as uid 1001

- [ ] **Step 1: Write the Dockerfile**

Create `images/sandbox/Dockerfile`:

```dockerfile
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY sandboxd/go.mod sandboxd/go.sum ./sandboxd/
RUN cd sandboxd && go mod download
COPY sandboxd ./sandboxd
RUN cd sandboxd && CGO_ENABLED=0 go build -o /out/sandboxd ./cmd/sandboxd

FROM node:20-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep procps \
    && rm -rf /var/lib/apt/lists/*

# Agent harnesses. Pinned so a sandbox image is reproducible.
RUN npm install -g @anthropic-ai/claude-code@latest

RUN useradd -m -u 1001 -s /bin/bash dev
COPY --from=build /out/sandboxd /usr/local/bin/sandboxd
COPY images/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER dev
WORKDIR /home/dev
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
```

- [ ] **Step 2: Write the entrypoint**

Create `images/sandbox/entrypoint.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# COMMAND selects the harness. Defaults to a plain shell so a sandbox is
# useful even when no agent is requested.
COMMAND="${PANACEA_APP_COMMAND:-/bin/bash}"
ARGS="${PANACEA_APP_ARGS:-}"

exec /usr/local/bin/sandboxd -addr :8080 -command "${COMMAND}" -args "${ARGS}"
```

- [ ] **Step 3: Build the image locally**

Run: `docker build -f images/sandbox/Dockerfile -t panacea-app/sandbox:dev .`
Expected: build succeeds.

- [ ] **Step 4: Verify the image serves a shell**

Run:
```bash
docker run --rm -d -p 8081:8080 --name pa-sandbox panacea-app/sandbox:dev
curl -s localhost:8081/healthz -o /dev/null -w '%{http_code}'
docker rm -f pa-sandbox
```
Expected: `200`.

- [ ] **Step 5: Verify Claude Code is present**

Run: `docker run --rm panacea-app/sandbox:dev bash -lc 'claude --version'`
Expected: a version string, exit 0.

- [ ] **Step 6: Commit**

```bash
git add images/sandbox
git commit -m "feat(sandbox): container image with claude code and sandboxd"
```

---

### Task 6: Kata node pool

Adds an isolated, tainted node pool to the **dev** AKS cluster. Kata gives each pod its own microVM and kernel.

**Files:**
- Create: `infra/terraform/providers.tf`, `infra/terraform/variables.tf`, `infra/terraform/kata.tf`, `infra/terraform/dev.tfvars`

**Interfaces:**
- Consumes: an existing AKS cluster id
- Produces: node pool `katapool` accepting pods with `runtimeClassName: kata-mshv-vm-isolation` and tolerating taint `panacea-app/sandbox=true:NoSchedule`

- [ ] **Step 1: Write the provider and variables**

Create `infra/terraform/providers.tf`:

```hcl
terraform {
  required_version = ">= 1.9.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}

provider "azurerm" {
  features {}
}
```

Create `infra/terraform/variables.tf`:

```hcl
variable "aks_cluster_id" {
  type        = string
  description = "Resource id of the existing dev AKS cluster"
}

variable "node_count" {
  type        = number
  default     = 1
  description = "Baseline node count; the autoscaler grows this on demand"
}

variable "max_count" {
  type        = number
  default     = 4
}

variable "vm_size" {
  type        = string
  default     = "Standard_D8s_v5"
  description = "Must be a Gen2 SKU that supports nested virtualisation"
}
```

- [ ] **Step 2: Write the node pool**

Create `infra/terraform/kata.tf`:

```hcl
# Pod Sandboxing (Kata Containers) requires Azure Linux, a Gen2
# nested-virtualisation-capable SKU, and workload_runtime KataMshvVmIsolation.
resource "azurerm_kubernetes_cluster_node_pool" "kata" {
  name                  = "katapool"
  kubernetes_cluster_id = var.aks_cluster_id
  vm_size               = var.vm_size
  os_sku                = "AzureLinux"
  os_type               = "Linux"
  workload_runtime      = "KataMshvVmIsolation"

  auto_scaling_enabled = true
  node_count           = var.node_count
  min_count            = var.node_count
  max_count            = var.max_count

  node_labels = {
    "panacea-app/role" = "sandbox"
  }

  node_taints = [
    "panacea-app/sandbox=true:NoSchedule",
  ]

  tags = {
    product = "panacea-app"
    phase   = "1"
  }

  lifecycle {
    # Node pools rotate nodes on os_disk or zone changes; make that explicit.
    ignore_changes = [node_count]
  }
}

output "kata_node_pool_name" {
  value = azurerm_kubernetes_cluster_node_pool.kata.name
}
```

- [ ] **Step 3: Write the dev variables file**

Create `infra/terraform/dev.tfvars`:

```hcl
# Replace with the dev cluster id from:
#   az aks show -g rg-aks-panacea-dev -n aks-panacea-dev --query id -o tsv
aks_cluster_id = "REPLACE_ME"
node_count     = 1
max_count      = 4
vm_size        = "Standard_D8s_v5"
```

- [ ] **Step 4: Validate the configuration**

Run: `cd infra/terraform && terraform init && terraform validate`
Expected: `Success! The configuration is valid.`

- [ ] **Step 5: Commit**

```bash
git add infra/terraform
git commit -m "feat(infra): kata node pool for sandbox microvms"
```

**Note for the operator:** do not `terraform apply` until the cluster id is filled in and a plan has been reviewed. Confirm the chosen SKU has nested-virtualisation quota in the target region before applying.

---

### Task 7: Sandbox manager

**Files:**
- Create: `control-plane/internal/sandbox/manager.go`
- Test: `control-plane/internal/sandbox/manager_test.go`

**Interfaces:**
- Consumes: `k8s.io/client-go/kubernetes.Interface`
- Produces:
  - `sandbox.Spec{ID, Image, Command string; CPU, Memory string}`
  - `sandbox.NewManager(client kubernetes.Interface, namespace string) *Manager`
  - `(*Manager).Create(ctx context.Context, spec Spec) (podName string, err error)`
  - `(*Manager).Delete(ctx context.Context, podName string) error`

- [ ] **Step 1: Add dependencies**

```bash
cd control-plane
go get k8s.io/client-go@v0.31.1 k8s.io/api@v0.31.1 k8s.io/apimachinery@v0.31.1
cd ..
```

- [ ] **Step 2: Write the failing tests**

Create `control-plane/internal/sandbox/manager_test.go`:

```go
package sandbox_test

import (
	"context"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateProducesAKataPodWithExplicitResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, "panacea-app")

	name, err := m.Create(context.Background(), sandbox.Spec{
		ID:      "abc123",
		Image:   "example/sandbox:dev",
		Command: "/bin/bash",
		CPU:     "500m",
		Memory:  "1Gi",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	pod, err := client.CoreV1().Pods("panacea-app").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata-mshv-vm-isolation" {
		t.Errorf("RuntimeClassName = %v, want kata-mshv-vm-isolation", pod.Spec.RuntimeClassName)
	}
	if pod.Spec.HostNetwork {
		t.Error("HostNetwork must be false: Kata does not support it")
	}

	c := pod.Spec.Containers[0]
	// Kata sizes the guest VM from the sum of requests and limits. Missing
	// values cause guest OOM, so both must be present.
	for _, key := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if _, ok := c.Resources.Requests[key]; !ok {
			t.Errorf("missing resource request %q", key)
		}
		if _, ok := c.Resources.Limits[key]; !ok {
			t.Errorf("missing resource limit %q", key)
		}
	}
	if c.SecurityContext == nil || c.SecurityContext.RunAsNonRoot == nil || !*c.SecurityContext.RunAsNonRoot {
		t.Error("container must run as non-root")
	}
	if pod.Labels["panacea-app/sandbox-id"] != "abc123" {
		t.Errorf("sandbox-id label = %q, want abc123", pod.Labels["panacea-app/sandbox-id"])
	}
}

func TestDeleteRemovesThePod(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, "panacea-app")

	name, err := m.Create(context.Background(), sandbox.Spec{
		ID: "gone", Image: "x:y", Command: "/bin/sh", CPU: "100m", Memory: "256Mi",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Delete(context.Background(), name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := client.CoreV1().Pods("panacea-app").Get(context.Background(), name, metav1.GetOptions{}); err == nil {
		t.Fatal("pod still present after Delete")
	}
}
```

The test file's import block must therefore include, alongside the others:

```go
corev1 "k8s.io/api/core/v1"
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/sandbox/...`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Write the implementation**

Create `control-plane/internal/sandbox/manager.go`:

```go
// Package sandbox creates the per-session Kata microVM pods that agent
// CLIs run inside.
package sandbox

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

// runtimeClass is the RuntimeClass installed by AKS pod sandboxing.
const runtimeClass = "kata-mshv-vm-isolation"

// Spec describes one sandbox.
type Spec struct {
	ID      string
	Image   string
	Command string
	CPU     string
	Memory  string
}

// Manager creates and removes sandbox pods in a single namespace.
type Manager struct {
	client    kubernetes.Interface
	namespace string
}

// NewManager returns a Manager bound to namespace.
func NewManager(client kubernetes.Interface, namespace string) *Manager {
	return &Manager{client: client, namespace: namespace}
}

// Create starts a sandbox pod and returns its name.
func (m *Manager) Create(ctx context.Context, spec Spec) (string, error) {
	name := "sandbox-" + spec.ID
	rc := runtimeClass
	nonRoot := true
	readOnlyRoot := true
	noEscalation := false
	uid := int64(1001)

	quantities := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(spec.CPU),
		corev1.ResourceMemory: resource.MustParse(spec.Memory),
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.namespace,
			Labels: map[string]string{
				"panacea-app/sandbox-id": spec.ID,
				"panacea-app/component":  "sandbox",
			},
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: &rc,
			// Kata does not support host networking.
			HostNetwork:   false,
			RestartPolicy: corev1.RestartPolicyNever,
			Tolerations: []corev1.Toleration{{
				Key:      "panacea-app/sandbox",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			}},
			NodeSelector: map[string]string{"panacea-app/role": "sandbox"},
			Containers: []corev1.Container{{
				Name:  "sandbox",
				Image: spec.Image,
				Env: []corev1.EnvVar{
					{Name: "PANACEA_APP_COMMAND", Value: spec.Command},
				},
				Ports: []corev1.ContainerPort{{Name: "attach", ContainerPort: 8080}},
				// Requests AND limits are mandatory: the Kata guest VM is
				// sized from them and under-requesting causes guest OOM.
				Resources: corev1.ResourceRequirements{
					Requests: quantities,
					Limits:   quantities,
				},
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:             &nonRoot,
					RunAsUser:                &uid,
					ReadOnlyRootFilesystem:   &readOnlyRoot,
					AllowPrivilegeEscalation: &noEscalation,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}

	created, err := m.client.CoreV1().Pods(m.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Delete removes a sandbox pod.
func (m *Manager) Delete(ctx context.Context, podName string) error {
	return m.client.CoreV1().Pods(m.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/sandbox/... -v`
Expected: PASS, two tests.

- [ ] **Step 6: Commit**

```bash
git add control-plane
git commit -m "feat(sandbox): create kata-isolated sandbox pods"
```

---

### Task 8: WebSocket proxy

**Files:**
- Create: `control-plane/internal/proxy/proxy.go`
- Test: `control-plane/internal/proxy/proxy_test.go`

**Interfaces:**
- Consumes: `github.com/coder/websocket`
- Produces: `proxy.Relay(ctx context.Context, client, upstream *websocket.Conn) error` — copies binary messages both ways until either side closes

- [ ] **Step 1: Add the dependency**

```bash
cd control-plane && go get github.com/coder/websocket@v1.8.12 && cd ..
```

- [ ] **Step 2: Write the failing test**

Create `control-plane/internal/proxy/proxy_test.go`:

```go
package proxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/proxy"
)

// echoServer replies to every binary message with the same bytes.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		for {
			typ, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if err := c.Write(r.Context(), typ, data); err != nil {
				return
			}
		}
	}))
}

func TestRelayCopiesBothDirections(t *testing.T) {
	upstreamSrv := echoServer(t)
	defer upstreamSrv.Close()

	// The proxy under test: accept a client, dial upstream, relay.
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer client.Close(websocket.StatusNormalClosure, "")

		up, _, err := websocket.Dial(r.Context(),
			"ws"+strings.TrimPrefix(upstreamSrv.URL, "http"), nil)
		if err != nil {
			return
		}
		defer up.Close(websocket.StatusNormalClosure, "")

		_ = proxy.Relay(r.Context(), client, up)
	}))
	defer proxySrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(proxySrv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	want := []byte{0x01, 'h', 'i'}
	if err := c.Write(ctx, websocket.MessageBinary, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	_, got, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./control-plane/internal/proxy/...`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Write the implementation**

Create `control-plane/internal/proxy/proxy.go`:

```go
// Package proxy relays WebSocket traffic between a desktop client and a
// sandbox. It never inspects payloads; the frame protocol is opaque here.
package proxy

import (
	"context"
	"errors"

	"github.com/coder/websocket"
)

// Relay copies messages between client and upstream until either closes or
// ctx is cancelled. It returns the first error observed.
func Relay(ctx context.Context, client, upstream *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 2)

	copyMessages := func(dst, src *websocket.Conn) {
		for {
			typ, data, err := src.Read(ctx)
			if err != nil {
				errs <- err
				return
			}
			if err := dst.Write(ctx, typ, data); err != nil {
				errs <- err
				return
			}
		}
	}

	go copyMessages(upstream, client)
	go copyMessages(client, upstream)

	err := <-errs
	cancel()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./control-plane/internal/proxy/... -v -race`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/proxy
git commit -m "feat(proxy): bidirectional websocket relay"
```

---

### Task 9: Desktop app with a live terminal

**Files:**
- Create: `desktop/` via the Tauri scaffolder, then `desktop/src/lib/wire.ts`, `desktop/src/components/Terminal.tsx`, `desktop/src/App.tsx`

**Interfaces:**
- Consumes: the `/attach` WebSocket endpoint
- Produces: a Tauri window rendering a live PTY

- [ ] **Step 1: Scaffold the Tauri app**

```bash
npm create tauri-app@latest desktop -- --template react-ts --manager npm --yes
cd desktop && npm install && npm install @xterm/xterm @xterm/addon-fit && cd ..
```

- [ ] **Step 2: Mirror the wire protocol in TypeScript**

Create `desktop/src/lib/wire.ts`:

```ts
// Mirror of sandboxd/internal/wire. Keep the constants in step with the Go
// definition — they are the contract between the two.
export const Kind = {
  Stdout: 0x00,
  Stdin: 0x01,
  Resize: 0x02,
  Exit: 0x03,
} as const;

export type KindValue = (typeof Kind)[keyof typeof Kind];

export function encode(kind: KindValue, payload: Uint8Array): Uint8Array {
  const out = new Uint8Array(payload.length + 1);
  out[0] = kind;
  out.set(payload, 1);
  return out;
}

export function decode(frame: Uint8Array): { kind: number; payload: Uint8Array } {
  if (frame.length === 0) throw new Error("wire: empty frame");
  return { kind: frame[0], payload: frame.subarray(1) };
}

export function encodeResize(cols: number, rows: number): Uint8Array {
  const body = new Uint8Array(4);
  new DataView(body.buffer).setUint16(0, cols);
  new DataView(body.buffer).setUint16(2, rows);
  return encode(Kind.Resize, body);
}
```

- [ ] **Step 3: Write the terminal component**

Create `desktop/src/components/Terminal.tsx`:

```tsx
import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Kind, decode, encode, encodeResize } from "../lib/wire";

export function Terminal({ url }: { url: string }) {
  const host = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!host.current) return;

    const term = new XTerm({
      fontFamily: '"Cascadia Mono", ui-monospace, Menlo, Consolas, monospace',
      fontSize: 12,
      theme: { background: "#0a0908", foreground: "#d6d0c8" },
      cursorBlink: true,
      // Let the agent CLI own the screen; we never reinterpret its output.
      convertEol: false,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host.current);
    fit.fit();

    const ws = new WebSocket(url);
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      ws.send(encodeResize(term.cols, term.rows));
      term.focus();
    };
    ws.onmessage = (ev) => {
      const { kind, payload } = decode(new Uint8Array(ev.data as ArrayBuffer));
      if (kind === Kind.Stdout) term.write(payload);
      else if (kind === Kind.Exit) term.write("\r\n\x1b[2m[session ended]\x1b[0m\r\n");
    };
    ws.onclose = () => term.write("\r\n\x1b[2m[disconnected]\x1b[0m\r\n");

    const onData = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(encode(Kind.Stdin, new TextEncoder().encode(data)));
      }
    });

    const onResize = () => {
      fit.fit();
      if (ws.readyState === WebSocket.OPEN) ws.send(encodeResize(term.cols, term.rows));
    };
    window.addEventListener("resize", onResize);

    return () => {
      window.removeEventListener("resize", onResize);
      onData.dispose();
      ws.close();
      term.dispose();
    };
  }, [url]);

  return <div ref={host} style={{ width: "100%", height: "100%" }} />;
}
```

- [ ] **Step 4: Wire it into the app shell**

Replace `desktop/src/App.tsx`:

```tsx
import { useState } from "react";
import { Terminal } from "./components/Terminal";

const DEFAULT_URL = "ws://localhost:8080/attach";

export default function App() {
  const [url, setUrl] = useState(DEFAULT_URL);
  const [connected, setConnected] = useState(false);

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh", background: "#0a0908", color: "#f2ede7" }}>
      <div style={{ display: "flex", gap: 8, padding: 8, borderBottom: "1px solid rgba(255,248,240,.075)" }}>
        <input
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          style={{ flex: 1, background: "#1a1715", color: "#f2ede7", border: 0, padding: "6px 9px", borderRadius: 7 }}
        />
        <button onClick={() => setConnected((c) => !c)} style={{ padding: "6px 12px", borderRadius: 7 }}>
          {connected ? "Disconnect" : "Connect"}
        </button>
      </div>
      <div style={{ flex: 1, minHeight: 0, padding: 8 }}>
        {connected ? <Terminal url={url} /> : <div style={{ opacity: 0.5, fontSize: 13 }}>Not connected.</div>}
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Run the desktop app against a local sandboxd**

Terminal one: `go run ./sandboxd/cmd/sandboxd -command /bin/bash`
Terminal two: `cd desktop && npm run tauri dev`

Click **Connect**. Expected: a live bash prompt in the window; typing `ls` and pressing Enter lists files.

- [ ] **Step 6: Commit**

```bash
git add desktop
git commit -m "feat(desktop): tauri shell rendering a live pty with xterm"
```

---

### Task 10: End-to-end verification against the dev cluster

This task has no new code. It proves the phase goal and records what was observed.

**Files:**
- Create: `docs/verification/2026-09-17-phase-1.md`

- [ ] **Step 1: Push the sandbox image**

```bash
az acr build --registry panaceaappacr --image panacea-app/sandbox:dev \
  --file images/sandbox/Dockerfile .
```
Expected: build succeeds and the tag is listed by `az acr repository show-tags --name panaceaappacr --repository panacea-app/sandbox`.

- [ ] **Step 2: Apply the Kata node pool**

```bash
cd infra/terraform
terraform plan -var-file=dev.tfvars -out=kata.plan
terraform apply kata.plan
```
Expected: one node pool created. Confirm with
`kubectl get nodes -l panacea-app/role=sandbox`.

- [ ] **Step 3: Confirm the RuntimeClass exists**

Run: `kubectl get runtimeclass kata-mshv-vm-isolation`
Expected: the RuntimeClass is listed. If it is absent, pod sandboxing is not enabled on the cluster — stop and enable it before continuing.

- [ ] **Step 4: Start one sandbox pod by hand**

```bash
kubectl create namespace panacea-app --dry-run=client -o yaml | kubectl apply -f -
kubectl run sandbox-manual -n panacea-app \
  --image=panaceaappacr.azurecr.io/panacea-app/sandbox:dev \
  --overrides='{"spec":{"runtimeClassName":"kata-mshv-vm-isolation","nodeSelector":{"panacea-app/role":"sandbox"},"tolerations":[{"key":"panacea-app/sandbox","operator":"Equal","value":"true","effect":"NoSchedule"}],"containers":[{"name":"sandbox","image":"panaceaappacr.azurecr.io/panacea-app/sandbox:dev","env":[{"name":"PANACEA_APP_COMMAND","value":"/bin/bash"}],"resources":{"requests":{"cpu":"500m","memory":"1Gi"},"limits":{"cpu":"500m","memory":"1Gi"}}}]}}'
kubectl wait -n panacea-app --for=condition=Ready pod/sandbox-manual --timeout=180s
```
Expected: the pod reaches Ready. Record how long it took — this is the Kata cold-start number that determines warm-pool sizing in Phase 2.

- [ ] **Step 5: Port-forward and connect the desktop app**

```bash
kubectl port-forward -n panacea-app pod/sandbox-manual 8080:8080
```
Then run the desktop app and connect to `ws://localhost:8080/attach`.
Expected: a bash prompt from inside the microVM.

- [ ] **Step 6: Run Claude Code in the sandbox and verify fidelity**

In the connected terminal, type `claude` and press Enter.

Verify and record each of these — they are the phase's actual acceptance criteria:
- The `╭─ > ─╮` input frame renders with unbroken box characters.
- Colour is correct (no stray escape codes printed as text).
- `⏵⏵ bypass permissions on (shift+tab to cycle)` appears, and **shift+tab actually cycles it**.
- Ask it to run a command; the numbered permission prompt renders, and pressing `1` selects Yes.
- Resizing the window reflows the TUI rather than corrupting it.
- Latency on keystroke echo is acceptable — measure roughly and write the number down.

- [ ] **Step 7: Record the results**

Create `docs/verification/2026-09-17-phase-1.md` containing: the commands run, the cold-start duration from Step 4, a pass or fail against each fidelity check in Step 6, the measured keystroke latency, and any defect found. State failures plainly — a failed fidelity check here changes the architecture, and hiding it wastes the whole phase.

- [ ] **Step 8: Tear down**

```bash
kubectl delete pod -n panacea-app sandbox-manual
```

- [ ] **Step 9: Commit**

```bash
git add docs/verification
git commit -m "docs: phase 1 end-to-end verification results"
```

---

## Self-review

**Spec coverage.** Phase 1 in `docs/PROGRAM.md` requires a real Claude Code session in a Kata microVM streamed to a Tauri window and typed into. Tasks 2–4 build the streaming, Task 5 packages the agent, Task 6 provides Kata isolation, Tasks 7–8 build the control plane path, Task 9 renders it, Task 10 proves it. The constraint that agent CLIs are never parsed is met — `wire` carries opaque bytes and xterm.js renders them directly.

**Deliberately out of scope**, deferred with a home: authentication (Phase 2), session persistence (Phase 2), warm pools (Phase 2, sized using the Task 10 cold-start measurement), reconnect after network loss (Phase 2), Windows local PTYs via ConPTY (Phase 2 — `creack/pty` is POSIX-only), capacity rotation (Phase 3).

**Type consistency.** `wire.Kind` constants are identical in `frame.go` and `wire.ts`. `ResizePayload` is four big-endian bytes in both. `sandbox.Spec` field names match their use in the Task 7 tests. `proxy.Relay` takes `*websocket.Conn` from `github.com/coder/websocket`, the same library used by sandboxd.

**Known risk carried forward.** The control plane in Tasks 7–8 is built and unit-tested but not yet joined to an HTTP entrypoint; Task 10 deliberately bypasses it with `kubectl port-forward` so that PTY fidelity is proven independently of the control plane. Wiring `cmd/api` to create a sandbox and relay to it is the first task of Phase 2.

---

Plan complete and saved to `docs/superpowers/plans/2026-09-17-phase-1-live-session.md`.
