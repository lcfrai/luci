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
