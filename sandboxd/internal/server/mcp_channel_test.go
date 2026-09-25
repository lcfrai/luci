//go:build linux || darwin

package server_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/mcprelay"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
)

func relayListener(t *testing.T) *mcprelay.Listener {
	t.Helper()
	l, err := mcprelay.Listen("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("mcprelay.Listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func dialChannel(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if token != "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+token)
	}
	c, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+mcprelay.ChannelPath, opts)
	if c != nil {
		t.Cleanup(func() { c.CloseNow() })
	}
	return c, resp, err
}

// The published sandbox port is on loopback (Config.AttachToken), reachable
// by every local process; the channel carries the canvas MCP, so it takes the
// attach token exactly as /attach does.
func TestMCPChannel_requiresToken(t *testing.T) {
	l := relayListener(t)
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken, MCP: l})
	for _, token := range []string{"", "not-the-token"} {
		_, resp, err := dialChannel(t, srv, token)
		if err == nil {
			t.Fatalf("token %q: the handshake succeeded", token)
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("token %q: status = %v, want 401", token, resp)
		}
	}

	// With the token the route is the relay: a local connection becomes an OPEN.
	c, _, err := dialChannel(t, srv, testToken)
	if err != nil {
		t.Fatalf("dial with the token: %v", err)
	}
	local, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dial the relay: %v", err)
	}
	defer local.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	typ, b, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read the channel: %v", err)
	}
	f, err := mcprelay.Decode(b)
	if typ != websocket.MessageBinary || err != nil || f.Kind != mcprelay.KindOpen {
		t.Fatalf("got %v %x (%v), want a binary OPEN", typ, b, err)
	}
}

// A browser cannot set Authorization on a WebSocket, so the token already
// keeps pages out; the origin check is kept on as well, and a foreign Origin
// is refused even with the right token. The control plane sends none.
func TestMCPChannel_refusesABrowserOrigin(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken, MCP: relayListener(t)})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	opts.HTTPHeader.Set("Authorization", "Bearer "+testToken)
	opts.HTTPHeader.Set("Origin", "https://evil.example")
	c, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+mcprelay.ChannelPath, opts)
	if err == nil {
		c.CloseNow()
		t.Fatalf("a foreign Origin was accepted on %s", mcprelay.ChannelPath)
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v, want 403", resp)
	}
}

func TestMCPChannel_disabled(t *testing.T) {
	srv := newServer(t, server.Config{Command: "/bin/sh", AttachToken: testToken})
	for _, token := range []string{"", testToken} {
		_, resp, err := dialChannel(t, srv, token)
		if err == nil {
			t.Fatalf("token %q: the handshake succeeded with no relay configured", token)
		}
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			t.Fatalf("token %q: status = %v, want 404", token, resp)
		}
	}
}
