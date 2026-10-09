package proxy_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/proxy"
)

// Each endpoint is controlled independently: an echo server would turn a client
// Exit lookalike into a genuine upstream frame and fail to test direction.
func exitHookRelay(t *testing.T, hooks proxy.Hooks) (client, peer *websocket.Conn, ctx context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	t.Cleanup(cancel)
	ready := make(chan *websocket.Conn, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		ready <- conn
	}))
	t.Cleanup(upstream.Close)
	finished := make(chan error, 1)
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			finished <- err
			return
		}
		defer conn.CloseNow()
		up, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(upstream.URL, "http"), nil)
		if err != nil {
			finished <- err
			return
		}
		defer up.CloseNow()
		finished <- proxy.RelayHooked(ctx, conn, up, hooks)
	}))
	t.Cleanup(relay.Close)
	var err error
	client, _, err = websocket.Dial(ctx, "ws"+strings.TrimPrefix(relay.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.CloseNow() })
	select {
	case peer = <-ready:
	case <-ctx.Done():
		t.Fatal("upstream did not accept before bounded deadline")
	}
	t.Cleanup(func() {
		_ = client.CloseNow()
		_ = peer.CloseNow()
		terminalRelayAwait(t, finished)
	})
	return client, peer, ctx
}

func exitHookForward(t *testing.T, ctx context.Context, from, to *websocket.Conn, typ websocket.MessageType, frame []byte) {
	t.Helper()
	if err := from.Write(ctx, typ, frame); err != nil {
		t.Fatal(err)
	}
	gotType, got, err := to.Read(ctx)
	if err != nil || gotType != typ || !bytes.Equal(got, frame) {
		t.Fatalf("relay changed frame: type=%v data=%x error=%v", gotType, got, err)
	}
}

func TestRelayUpstreamExitHookDiscriminatesDirectionAndType(t *testing.T) {
	var mu sync.Mutex
	var upstreamSizes, clientSizes []int
	var exits, sizesBeforeExit int
	client, peer, ctx := exitHookRelay(t, proxy.Hooks{
		OnUpstream: func(n int) { mu.Lock(); upstreamSizes = append(upstreamSizes, n); mu.Unlock() },
		OnClient:   func(n int) { mu.Lock(); clientSizes = append(clientSizes, n); mu.Unlock() },
		OnUpstreamExit: func() {
			mu.Lock()
			exits++
			sizesBeforeExit = len(upstreamSizes)
			mu.Unlock()
		},
	})
	assertExits := func(t *testing.T, want int) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if exits != want {
			t.Fatalf("exit observations = %d, want %d", exits, want)
		}
	}
	clientExit := []byte{0x03, 'c', 'l', 'i', 'e', 'n', 't'}
	exitHookForward(t, ctx, client, peer, websocket.MessageBinary, clientExit)
	assertExits(t, 0)
	var wantUpstreamSizes []int
	for _, tc := range []struct {
		name  string
		typ   websocket.MessageType
		frame []byte
	}{
		{"empty-binary", websocket.MessageBinary, nil},
		{"empty-text", websocket.MessageText, nil},
		{"text-discriminant", websocket.MessageText, []byte{0x03, 'x'}},
		{"text-lookalike", websocket.MessageText, []byte(`{"type":"exit","code":0}`)},
		{"stdout-discriminant", websocket.MessageBinary, []byte{0x00, 0x03}},
		{"stdout-lookalike", websocket.MessageBinary, []byte("\x00exit status 0\n[session ended]")},
		{"other-binary-kind", websocket.MessageBinary, []byte{0x02, 0x03}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exitHookForward(t, ctx, peer, client, tc.typ, tc.frame)
			assertExits(t, 0)
		})
		wantUpstreamSizes = append(wantUpstreamSizes, len(tc.frame))
	}
	for _, frame := range [][]byte{{0x03}, {0x03, 0xff, 0x00, 0xfe}, []byte("\x03exit status 17")} {
		exitHookForward(t, ctx, peer, client, websocket.MessageBinary, frame)
		wantUpstreamSizes = append(wantUpstreamSizes, len(frame))
		assertExits(t, 1)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(upstreamSizes, wantUpstreamSizes) || !reflect.DeepEqual(clientSizes, []int{len(clientExit)}) {
		t.Fatalf("size hooks changed: upstream=%v client=%v", upstreamSizes, clientSizes)
	}
	if sizesBeforeExit != len(wantUpstreamSizes)-2 {
		t.Fatalf("exit hook ran before its size hook: sizes=%d", sizesBeforeExit)
	}
}

func TestRelayUpstreamExitHookIsOncePerRelay(t *testing.T) {
	var exits atomic.Int32
	hooks := proxy.Hooks{OnUpstreamExit: func() { exits.Add(1) }}
	for i := int32(1); i <= 2; i++ {
		client, peer, ctx := exitHookRelay(t, hooks)
		for repeat := 0; repeat < 2; repeat++ {
			exitHookForward(t, ctx, peer, client, websocket.MessageBinary, []byte{0x03})
			if got := exits.Load(); got != i {
				t.Fatalf("exit observations = %d after relay %d, want %d", got, i, i)
			}
		}
	}
}

func TestRelayNilUpstreamExitHookPreservesTypedFrame(t *testing.T) {
	client, peer, ctx := exitHookRelay(t, proxy.Hooks{})
	exitHookForward(t, ctx, peer, client, websocket.MessageBinary, []byte{0x03, 0xff, 0x00})
}
