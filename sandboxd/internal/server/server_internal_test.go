//go:build linux || darwin

package server

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/sandboxd/internal/hub"
	"github.com/lcfrai/luci/sandboxd/internal/wire"
)

// These drive the real fanOut, not exitFrame in isolation, because that is
// where the amendment actually lives: an unused exitFrame still compiles and
// still returns the right answer, so a fanOut that stopped calling it would
// pass a test written against the helper.
//
// They are white-box for the reason they always were. Over a real WebSocket,
// coder/websocket's client Close() performs a closing handshake that reads
// and discards any data frame the server sends while it is in flight (see
// vendor/github.com/coder/websocket/close.go, waitCloseHandshake) — so a
// wrongly-sent Exit frame on detach is not observable by a black-box test
// against a real connection.

// recordingConn is a wsConn that records every frame it is asked to write
// instead of putting anything on a network.
type recordingConn struct {
	mu     sync.Mutex
	frames [][]byte
}

func (r *recordingConn) Write(_ context.Context, _ websocket.MessageType, p []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]byte, len(p))
	copy(cp, p)
	r.frames = append(r.frames, cp)
	return nil
}

func (r *recordingConn) Close(websocket.StatusCode, string) error { return nil }

// exitReasons returns the payload of every Exit frame written, in order, so a
// test can assert both how many were sent and what they said.
func (r *recordingConn) exitReasons(t *testing.T) []string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, f := range r.frames {
		kind, payload, err := wire.Decode(f)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if kind == wire.Exit {
			out = append(out, string(payload))
		}
	}
	return out
}

// This is the amendment itself: detaching while the process is still
// running must never produce an Exit frame. Deleting the
// `if frame, send := exitFrame(h); send` guard in fanOut and writing
// unconditionally makes this fail — see task-3-report.md for the RED that
// confirms it.
func TestFanOutSendsNoExitFrameOnDetach(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("hub.Start: %v", err)
	}
	defer h.Close()

	_, out, cancel := h.Subscribe()
	conn := &recordingConn{}

	done := make(chan struct{})
	go func() {
		fanOut(context.Background(), conn, h, out)
		close(done)
	}()

	cancel() // detach while the process is still running

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fanOut did not return after cancel")
	}

	if got := conn.exitReasons(t); len(got) != 0 {
		t.Fatalf("got Exit frames %q on a detach, want none", got)
	}
}

// The mirror image: a real exit must still produce exactly one Exit frame,
// carrying the reason.
func TestFanOutSendsExitFrameWhenTheProcessExits(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh", Args: []string{"-c", "exit 0"}})
	if err != nil {
		t.Fatalf("hub.Start: %v", err)
	}
	defer h.Close()

	_, out, cancel := h.Subscribe()
	defer cancel()
	conn := &recordingConn{}

	done := make(chan struct{})
	go func() {
		fanOut(context.Background(), conn, h, out)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fanOut did not return after the process exited")
	}

	if got, want := conn.exitReasons(t), []string{"exit status 0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Exit frames = %q, want %q", got, want)
	}
}

// Attaching after the process has already gone must still report the exit.
// Subscribe hands such a client a pre-closed channel, so hub.Done is the only
// signal fanOut has; hub.finish closes done in the same critical section that
// sets closed, which is what makes this deterministic rather than a race
// between the two.
func TestFanOutSendsExitFrameToAClientThatAttachedAfterTheExit(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh", Args: []string{"-c", "exit 0"}})
	if err != nil {
		t.Fatalf("hub.Start: %v", err)
	}
	defer h.Close()

	<-h.Done()

	_, out, cancel := h.Subscribe()
	defer cancel()
	conn := &recordingConn{}

	// out is already closed, so fanOut returns on its first iteration; no
	// goroutine and no timeout are needed to observe it.
	fanOut(context.Background(), conn, h, out)

	if got, want := conn.exitReasons(t), []string{"exit status 0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Exit frames = %q, want %q", got, want)
	}
}
