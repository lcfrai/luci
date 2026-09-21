//go:build linux || darwin

package server

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/hub"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

// These pin exitFrame's decision directly: a client detaching (cancel) and
// the process exiting (finish) close the same subscriber channel with no
// distinguishing signal, so only Done() may authorise sending an Exit frame.
//
// This is white-box on purpose. Over a real WebSocket, coder/websocket's
// client Close() performs a closing handshake that reads and discards any
// data frame the server sends while it is in flight (see
// vendor/github.com/coder/websocket/close.go, waitCloseHandshake) — so a
// wrongly-sent Exit frame on detach is not observable by a black-box test
// against a real connection. The decision itself is exactly what needs
// pinning.

func TestExitFrameNotSentOnDetach(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("hub.Start: %v", err)
	}
	defer h.Close()

	_, out, cancel := h.Subscribe()
	cancel() // detach; the shell is still running
	if _, ok := <-out; ok {
		t.Fatal("subscriber channel should be closed after cancel")
	}

	if _, send := exitFrame(h); send {
		t.Fatal("exitFrame said to send on a detach; the process has not exited")
	}
}

func TestExitFrameSentWhenTheProcessExited(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh", Args: []string{"-c", "exit 0"}})
	if err != nil {
		t.Fatalf("hub.Start: %v", err)
	}
	defer h.Close()

	<-h.Done()

	frame, send := exitFrame(h)
	if !send {
		t.Fatal("exitFrame said not to send after the process exited")
	}
	kind, payload, err := wire.Decode(frame)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if kind != wire.Exit {
		t.Fatalf("kind = %v, want wire.Exit", kind)
	}
	if string(payload) != "exit status 0" {
		t.Fatalf("payload = %q, want %q", payload, "exit status 0")
	}
}
