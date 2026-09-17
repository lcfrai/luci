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
