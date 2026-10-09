package proxy_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/proxy"
)

// A WebSocket-aware test client can silently swallow a second close while
// finishing its own handshake. Read the raw wire through peer acknowledgement
// and transport EOF: Chromium rejects a duplicate server close frame.
func TestRelaySendsOneCloseFrameBeforeTransportEOF(t *testing.T) {
	for _, tc := range []struct {
		name string
		code websocket.StatusCode
		exit string
	}{
		{"exit-zero", websocket.StatusNormalClosure, "exit status 0"},
		{"exit-nonzero", websocket.StatusNormalClosure, "exit status 7"},
		{"normal-without-exit", websocket.StatusNormalClosure, ""},
		{"overload", websocket.StatusTryAgainLater, ""},
		{"exit-then-overload", websocket.StatusTryAgainLater, "exit status 7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			upstreamReady := make(chan *websocket.Conn, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				upstreamReady <- c
			}))
			t.Cleanup(upstream.Close)
			var exitObserved atomic.Int32
			relayDone := make(chan error, 1)
			relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				if err != nil {
					relayDone <- err
					return
				}
				defer c.CloseNow()
				peer, _, err := websocket.Dial(r.Context(), "ws"+strings.TrimPrefix(upstream.URL, "http"), nil)
				if err != nil {
					relayDone <- err
					return
				}
				defer peer.CloseNow()
				relayDone <- proxy.RelayHooked(r.Context(), c, peer, proxy.Hooks{
					OnUpstreamExit: func() { exitObserved.Add(1) },
				})
			}))
			t.Cleanup(relay.Close)
			conn, reader := closeAckDial(t, ctx, relay.URL)
			t.Cleanup(func() { _ = conn.Close() })
			var peer *websocket.Conn
			select {
			case peer = <-upstreamReady:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			t.Cleanup(func() { _ = peer.CloseNow() })
			frames := [][]byte{{0, 'd', 'o', 'n', 'e', '\n'}}
			if tc.exit != "" {
				frames = append(frames, append([]byte{3}, []byte(tc.exit)...))
			}
			for _, frame := range frames {
				if err := peer.Write(ctx, websocket.MessageBinary, frame); err != nil {
					t.Fatal(err)
				}
			}
			peerClosed := make(chan error, 1)
			go func() { peerClosed <- peer.Close(tc.code, "private upstream reason") }()
			for _, want := range frames {
				op, got := closeAckReadFrame(t, reader)
				if op != 2 || !bytes.Equal(got, want) {
					t.Fatalf("terminal frame changed: opcode=%d data=%x", op, got)
				}
			}
			op, payload := closeAckReadFrame(t, reader)
			if op != 8 || len(payload) < 2 || binary.BigEndian.Uint16(payload[:2]) != uint16(tc.code) || !utf8.Valid(payload[2:]) {
				t.Fatalf("invalid close opcode=%d payload=%x", op, payload)
			}
			wantReason := ""
			if tc.code == websocket.StatusTryAgainLater {
				wantReason = "terminal output consumer too slow; reconnect for recent output only"
			}
			if string(payload[2:]) != wantReason {
				t.Fatal("close reason exposed upstream details or lost overload guidance")
			}
			closeAckWrite(t, conn, payload)
			// The peer has acknowledged the one permitted close. Any further frame is
			// a protocol defect, even when a Go WebSocket client would ignore it.
			trailing, err := io.ReadAll(io.LimitReader(reader, 256))
			if err != nil {
				t.Fatalf("transport did not settle after close acknowledgement: %v", err)
			}
			if len(trailing) != 0 {
				t.Fatalf("extra server bytes after close acknowledgement: %x", trailing)
			}
			select {
			case <-peerClosed:
			case <-ctx.Done():
				t.Fatal("upstream close did not settle")
			}
			select {
			case err := <-relayDone:
				if websocket.CloseStatus(err) != tc.code {
					t.Fatalf("relay close status=%d want=%d: %v", websocket.CloseStatus(err), tc.code, err)
				}
			case <-ctx.Done():
				t.Fatal("relay did not settle")
			}
			wantExits := int32(0)
			if tc.exit != "" {
				wantExits = 1
			}
			if got := exitObserved.Load(); got != wantExits {
				t.Fatalf("exit observations = %d, want %d", got, wantExits)
			}
		})
	}
}

func closeAckDial(t *testing.T, ctx context.Context, address string) (net.Conn, *bufio.Reader) {
	t.Helper()
	host := strings.TrimPrefix(address, "http://")
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", host)
	if err != nil {
		t.Fatal(err)
	}
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Extensions: permessage-deflate; client_max_window_bits\r\n\r\n", host)
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	req, _ := http.NewRequest("GET", address, nil)
	res, err := http.ReadResponse(reader, req)
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if res.StatusCode != 101 || res.Header.Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" || res.Header.Get("Sec-WebSocket-Extensions") != "" {
		conn.Close()
		t.Fatal("unexpected WebSocket handshake")
	}
	return conn, reader
}
func closeAckReadFrame(t *testing.T, r *bufio.Reader) (byte, []byte) {
	t.Helper()
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		t.Fatal(err)
	}
	// These short opaque fixtures require one uncompressed final frame. Server
	// masking, reserved bits and invalid control lengths must not be tolerated.
	if h[0]&0xf0 != 0x80 || h[1]&0x80 != 0 || h[1]&127 > 125 {
		t.Fatalf("invalid server frame header %x", h)
	}
	p := make([]byte, int(h[1]&127))
	if _, err := io.ReadFull(r, p); err != nil {
		t.Fatal(err)
	}
	return h[0] & 15, p
}
func closeAckWrite(t *testing.T, w io.Writer, payload []byte) {
	t.Helper()
	if len(payload) > 125 {
		t.Fatal("oversize close acknowledgement")
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		t.Fatal(err)
	}
	frame := []byte{0x88, 0x80 | byte(len(payload))}
	frame = append(frame, mask[:]...)
	for i, v := range payload {
		frame = append(frame, v^mask[i%4])
	}
	if _, err := w.Write(frame); err != nil {
		t.Fatal(err)
	}
}
