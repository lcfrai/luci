//go:build linux || darwin

package server_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/sandboxd/internal/server"
	"github.com/lcfrai/luci/sandboxd/internal/wire"
)

const testToken = "test-token-abc"

func newServer(t *testing.T, cfg server.Config) *httptest.Server {
	t.Helper()
	s, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv
}

func dial(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if token != "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+token)
	}
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach", opts)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { c.Close(websocket.StatusNormalClosure, "") })
	return c, ctx
}

// readUntil accumulates Stdout payloads until want appears.
func readUntil(t *testing.T, c *websocket.Conn, ctx context.Context, want []byte, d time.Duration) string {
	t.Helper()
	var acc bytes.Buffer
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithDeadline(ctx, deadline)
		_, data, err := c.Read(readCtx)
		cancel()
		if err != nil {
			t.Fatalf("Read: %v (got %q)", err, acc.String())
		}
		kind, payload, err := wire.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if kind != wire.Stdout {
			continue
		}
		acc.Write(payload)
		if bytes.Contains(acc.Bytes(), want) {
			return acc.String()
		}
	}
	t.Fatalf("never saw %q; got %q", want, acc.String())
	return ""
}

func TestHealthzNeedsNoToken(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAttachWithoutTokenIsRejected(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach", nil)
	if err == nil {
		t.Fatal("expected the handshake to fail without a token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestAttachWithWrongTokenIsRejected(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	opts.HTTPHeader.Set("Authorization", "Bearer not-the-token")
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach", opts)
	if err == nil {
		t.Fatal("expected the handshake to fail with a wrong token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestAttachWithBareTokenAndNoBearerSchemeIsRejected(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	opts.HTTPHeader.Set("Authorization", testToken) // no "Bearer " scheme
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/attach", opts)
	if err == nil {
		t.Fatal("expected the handshake to fail without the Bearer scheme")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestStreamsStdoutAndAcceptsStdin(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	c, ctx := dial(t, srv, testToken)

	if err := c.Write(ctx, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo luci-ok\n"))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, c, ctx, []byte("luci-ok"), 10*time.Second)
}

func TestAppliesResize(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	c, ctx := dial(t, srv, testToken)

	rp, _ := wire.ResizePayload{Cols: 120, Rows: 40}.MarshalBinary()
	if err := c.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Resize, rp)); err != nil {
		t.Fatalf("Write resize: %v", err)
	}
	// No sleep between the two frames. The handler reads this connection on
	// one goroutine in message order, and hub.Resize is a synchronous
	// TIOCSWINSZ on the PTY master, so the new winsize is already in place
	// before the stty bytes are written to it. Task 2 relies on the same
	// ordering in ptysession's session_test.go.
	if err := c.Write(ctx, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("stty size\n"))); err != nil {
		t.Fatalf("Write stty: %v", err)
	}
	readUntil(t, c, ctx, []byte("40 120"), 10*time.Second)
}

// The behaviour this whole group exists for.
func TestReAttachReplaysScrollbackAndKeepsTheProcess(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})

	c1, ctx1 := dial(t, srv, testToken)
	if err := c1.Write(ctx1, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo before-detach\n"))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, c1, ctx1, []byte("before-detach"), 10*time.Second)
	c1.Close(websocket.StatusNormalClosure, "")

	c2, ctx2 := dial(t, srv, testToken)
	// The replay must arrive without us typing anything.
	readUntil(t, c2, ctx2, []byte("before-detach"), 10*time.Second)

	// And the same shell is still alive.
	if err := c2.Write(ctx2, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo after-reattach\n"))); err != nil {
		t.Fatalf("Write after re-attach: %v", err)
	}
	readUntil(t, c2, ctx2, []byte("after-reattach"), 10*time.Second)
}

// A paste larger than coder/websocket's 32 KiB default read limit must survive.
// Without SetReadLimit the connection is closed with StatusMessageTooBig and the
// session appears to die for no reason.
func TestLargeStdinFrameIsAccepted(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	c, ctx := dial(t, srv, testToken)

	// 100 KiB of printable text, then a newline so the shell acts on it.
	big := bytes.Repeat([]byte("x"), 100*1024)
	line := append([]byte("echo len-"), big...)
	line = append(line, '\n')

	if err := c.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Stdin, line)); err != nil {
		t.Fatalf("Write large stdin: %v", err)
	}
	// The shell echoes the line back through the PTY; seeing the prefix proves
	// the frame was read rather than rejected.
	readUntil(t, c, ctx, []byte("len-xxxxxxxxxx"), 15*time.Second)
}

func TestExitFrameIsSentWhenTheProcessEnds(t *testing.T) {
	srv := newServer(t, server.Config{
		Command: "/bin/sh", Args: []string{"-c", "exit 0"}, AttachToken: testToken,
	})
	c, ctx := dial(t, srv, testToken)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithDeadline(ctx, deadline)
		_, data, err := c.Read(readCtx)
		cancel()
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		kind, _, err := wire.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if kind == wire.Exit {
			return
		}
	}
	t.Fatal("no Exit frame within 10s of the process exiting")
}

// The three states New's fail-closed check cares about: a token configured
// (covered above by every test that sets AttachToken), no token and no
// opt-out (must refuse to start), and no token with an explicit opt-out
// (must start and serve unauthenticated).

func TestNewFailsWithoutAttachTokenOrOptOut(t *testing.T) {
	s, err := server.New(server.Config{Command: "/bin/sh"})
	if err == nil {
		t.Fatal("expected New to fail with no AttachToken and no AllowUnauthenticated")
	}
	// Nothing was started, so there is nothing to Close: a non-nil Server
	// here would mean a PTY leaked behind a returned error.
	if s != nil {
		s.Close()
		t.Fatal("New returned a Server alongside its error")
	}
}

// F3's seam. The entrypoint learns that the process it exists to run has ended
// through Done and through nothing else; before it existed, main held only the
// http.Handler and served until something killed it, leaving a container Up
// around a process that was gone
// (docs/verification/2026-09-27-phase-2-results.md, finding F3).
func TestDoneClosesWhenTheProcessExitsAndCarriesItsReason(t *testing.T) {
	s, err := server.New(server.Config{
		Command:     "/bin/sh",
		Args:        []string{"-c", "exit 0"},
		AttachToken: testToken,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	select {
	case <-s.Done():
	case <-time.After(8 * time.Second):
		t.Fatal("Done() was not closed within 8s of the process exiting; the entrypoint would serve for ever (F3)")
	}
	if s.ExitReason() == "" {
		t.Error("ExitReason() is empty after the process exited; the daemon's last log line would say nothing")
	}
}

func TestNewAllowsNoTokenWhenExplicitlyUnauthenticated(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AllowUnauthenticated: true})
	c, ctx := dial(t, srv, "") // no Authorization header at all

	if err := c.Write(ctx, websocket.MessageBinary,
		wire.Encode(wire.Stdin, []byte("echo insecure-ok\n"))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, c, ctx, []byte("insecure-ok"), 10*time.Second)
}

// A finite burst must reach a healthy websocket completely and before Exit,
// regardless of how the PTY fragments it. The stdin regression above remains
// unchanged; this adds an exact full-output and lifecycle witness.
func TestFiniteOutputBurstIsCompleteBeforeExit(t *testing.T) {
	srv := newServer(t, server.Config{
		Command: "/bin/sh", Args: []string{"-c", "stty -echo; printf READY; read line; printf '%0102400d' 0; printf '\\nEND\\n'; exit 0"}, AttachToken: testToken,
	})
	c, ctx := dial(t, srv, testToken)
	readUntil(t, c, ctx, []byte("READY"), 10*time.Second)
	if err := c.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Stdin, []byte("\n"))); err != nil {
		t.Fatalf("release finite output: %v", err)
	}
	var got []byte
	for {
		_, frame, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read complete finite output: %v (got %d bytes)", err, len(got))
		}
		kind, payload, err := wire.Decode(frame)
		if err != nil {
			t.Fatal(err)
		}
		if kind == wire.Exit {
			if string(payload) != "exit status 0" {
				t.Fatalf("unexpected process result %q", payload)
			}
			break
		}
		if kind != wire.Stdout {
			t.Fatalf("unexpected frame kind %v", kind)
		}
		got = append(got, payload...)
	}
	want := append(bytes.Repeat([]byte("0"), 100*1024), []byte("\r\nEND\r\n")...)
	if !bytes.Equal(got, want) {
		t.Fatalf("finite output truncated or reordered: received %d/%d bytes", len(got), len(want))
	}
}
