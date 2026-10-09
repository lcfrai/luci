package proxy_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/proxy"
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

		_ = proxy.RelayHooked(r.Context(), client, up, proxy.Hooks{})
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

// Compile-time signature. RelayHooked is the one relay (seam S8.7 in
// docs/SEAMS.md; task 26 deleted the three-argument Relay, whose only caller
// was this file): the attach handler, and Phase 8's pods after it, are written
// against this form.
var _ func(context.Context, *websocket.Conn, *websocket.Conn, proxy.Hooks) error = proxy.RelayHooked

// growEchoServer replies to every message with the same bytes plus one. The
// extra byte is what makes the direction test meaningful: a plain echo returns
// the same length in both directions, so swapping OnClient and OnUpstream
// would produce identical numbers and the test would pass while wrong.
func growEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			typ, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if err := c.Write(r.Context(), typ, append(data, '!')); err != nil {
				return
			}
		}
	}))
}

// relayWithHooks stands growEchoServer behind a proxy that relays with h, and
// returns a client connected to the proxy. Everything is torn down by
// t.Cleanup, so a failing test does not leak a listener into the next one.
func relayWithHooks(t *testing.T, h proxy.Hooks) (*websocket.Conn, context.Context) {
	t.Helper()

	upstreamSrv := growEchoServer(t)
	t.Cleanup(upstreamSrv.Close)

	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer client.CloseNow()

		up, _, err := websocket.Dial(r.Context(), "ws"+strings.TrimPrefix(upstreamSrv.URL, "http"), nil)
		if err != nil {
			return
		}
		defer up.CloseNow()

		_ = proxy.RelayHooked(r.Context(), client, up, h)
	}))
	t.Cleanup(proxySrv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(proxySrv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c, ctx
}

// TestRelayHookedCountsEachDirection pins the direction mapping. It needs no
// sleep and no eventually-loop: a hook fires between the read and the write,
// so by the time the echo comes back to the client both hooks for that
// message have already run.
func TestRelayHookedCountsEachDirection(t *testing.T) {
	var mu sync.Mutex
	var toSandbox, toDesktop []int

	c, ctx := relayWithHooks(t, proxy.Hooks{
		OnClient:   func(n int) { mu.Lock(); toSandbox = append(toSandbox, n); mu.Unlock() },
		OnUpstream: func(n int) { mu.Lock(); toDesktop = append(toDesktop, n); mu.Unlock() },
	})

	for _, payload := range [][]byte{{0x01, 'h', 'i'}, {0x01, 'x'}} {
		if err := c.Write(ctx, websocket.MessageBinary, payload); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if _, _, err := c.Read(ctx); err != nil {
			t.Fatalf("Read: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := fmt.Sprint(toSandbox), "[3 2]"; got != want {
		t.Errorf("OnClient sizes = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(toDesktop), "[4 3]"; got != want {
		t.Errorf("OnUpstream sizes = %s, want %s", got, want)
	}
}

// TestRelayHookedToleratesNilHookFields is the case a caller with nothing to
// observe takes: an empty Hooks value. A nil-check per message is cheaper than
// making callers supply two no-op closures.
func TestRelayHookedToleratesNilHookFields(t *testing.T) {
	c, ctx := relayWithHooks(t, proxy.Hooks{})

	if err := c.Write(ctx, websocket.MessageBinary, []byte{0x01, 'o', 'k'}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	_, got, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := "\x01ok!"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
