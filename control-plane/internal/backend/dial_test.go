package backend_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// fakeSandbox is sandboxd's HTTP surface, reduced to what Dial and WaitReady
// touch: an unauthenticated /healthz and a bearer-guarded /attach.
type fakeSandbox struct {
	token      string
	healthCode atomic.Int32 // what /healthz answers
	gotAuth    atomic.Value // string: the Authorization header /attach saw
	// bigPayload, when non-empty, is written to the client immediately after
	// the upgrade. It exists to prove the read limit was raised.
	bigPayload []byte
}

func newFakeSandbox(t *testing.T, token string) (*fakeSandbox, string) {
	t.Helper()
	fs := &fakeSandbox{token: token}
	fs.healthCode.Store(int32(http.StatusOK))
	fs.gotAuth.Store("")

	mux := http.NewServeMux()
	mux.HandleFunc(backend.HealthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(fs.healthCode.Load()))
	})
	mux.HandleFunc(backend.AttachPath, func(w http.ResponseWriter, r *http.Request) {
		fs.gotAuth.Store(r.Header.Get("Authorization"))
		if fs.token != "" && r.Header.Get("Authorization") != "Bearer "+fs.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		if len(fs.bigPayload) > 0 {
			_ = c.Write(r.Context(), websocket.MessageBinary, fs.bigPayload)
		}
		// Hold the connection open until the client goes away.
		_, _, _ = c.Read(r.Context())
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return fs, strings.TrimPrefix(srv.URL, "http://")
}

func TestDialSendsTheBearerTokenAndOpensTheSocket(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "sekrit")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := backend.Dial(ctx, endpoint, "sekrit")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if got := fs.gotAuth.Load().(string); got != "Bearer sekrit" {
		t.Fatalf("/attach saw Authorization %q, want %q", got, "Bearer sekrit")
	}
}

// sandboxd answers 401 without the token. That must surface as a sentinel the
// API layer can switch on, not as an opaque handshake failure.
func TestDialWithTheWrongTokenIsErrUnauthorized(t *testing.T) {
	_, endpoint := newFakeSandbox(t, "sekrit")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := backend.Dial(ctx, endpoint, "wrong")
	if !errors.Is(err, backend.ErrUnauthorized) {
		t.Fatalf("Dial with a wrong token = %v, want an error wrapping ErrUnauthorized", err)
	}
}

func TestDialToAClosedPortIsErrNotRunning(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	endpoint := ln.Addr().String()
	ln.Close() // nothing is listening there now

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = backend.Dial(ctx, endpoint, "sekrit")
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Dial to a closed port = %v, want an error wrapping ErrNotRunning", err)
	}
}

// A sandbox mid-startup can answer /attach with a plain HTTP error before the
// process is up at all — no upgrade, and not a 401 either. That must still
// map to a sentinel (fix round 1, M3): the API layer uses errors.Is and never
// string-matches, so an unmatched error can only become a generic 500.
func TestDialWithANonUpgradeResponseIsErrNotRunning(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(backend.AttachPath, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "starting", http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	endpoint := strings.TrimPrefix(srv.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := backend.Dial(ctx, endpoint, "sekrit")
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Dial against a 503 handshake = %v, want an error wrapping ErrNotRunning", err)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("Dial error %q does not name the handshake status", err)
	}
}

// The reason Dial exists. coder/websocket caps one inbound message at 32768
// bytes; sandboxd's replay frame is up to 256 KiB. Without SetReadLimit this
// read fails with "read limited at 32769 bytes" and the session looks dead.
//
// The payload is sized at MaxMessageBytes-1, not merely "bigger than 32 KiB"
// (fix round 1, M6): a smaller fixed payload passes against any
// SetReadLimit(N) for N above that literal, including one that has drifted
// from sandboxd's mirrored constant. Only MaxMessageBytes-1 proves Dial
// passed the real constant through.
func TestDialRaisesTheReadLimitAboveTheLibraryDefault(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.bigPayload = make([]byte, backend.MaxMessageBytes-1)
	for i := range fs.bigPayload {
		fs.bigPayload[i] = byte('a' + i%26)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := backend.Dial(ctx, endpoint, "")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read of a 100 KiB message: %v (SetReadLimit is missing or runs after the first Read)", err)
	}
	if len(data) != len(fs.bigPayload) {
		t.Fatalf("read %d bytes, want %d", len(data), len(fs.bigPayload))
	}
}

func TestWaitReadyReturnsOnceHealthzAnswers200(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.healthCode.Store(int32(http.StatusServiceUnavailable))

	go func() {
		time.Sleep(600 * time.Millisecond)
		fs.healthCode.Store(int32(http.StatusOK))
	}()

	if err := backend.WaitReady(context.Background(), endpoint, 10*time.Second); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
}

func TestWaitReadyTimesOutWithErrNotRunning(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.healthCode.Store(int32(http.StatusServiceUnavailable))

	err := backend.WaitReady(context.Background(), endpoint, 700*time.Millisecond)
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("WaitReady on a sandbox that never comes up = %v, want an error wrapping ErrNotRunning", err)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("WaitReady error %q does not name the last status; the caller cannot tell a 503 from a refused connection", err)
	}
}

// WaitReady's timeout must be a real upper bound on total time, not merely on
// the loop condition (fix round 1, M5): a sandbox that accepts the TCP
// connection and then hangs must not let one slow attempt burn past the
// caller's deadline. Before the fix, a fixed 2s per-attempt client timeout
// stacked on top of the 700ms deadline requested here, giving ~2.7s; the
// bound below is well short of that.
func TestWaitReadyDoesNotOvershootTheTimeoutOnASlowAttempt(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(backend.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	endpoint := strings.TrimPrefix(srv.URL, "http://")

	start := time.Now()
	err := backend.WaitReady(context.Background(), endpoint, 700*time.Millisecond)
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("WaitReady against a hanging healthz = %v, want an error wrapping ErrNotRunning", err)
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("WaitReady took %s for a 700ms timeout; want well under the old ~2.7s overshoot", elapsed)
	}
}

// A cancelled context must abandon the poll promptly, not after the next
// interval: this runs while a user is watching a session that will not start.
func TestWaitReadyStopsWhenTheContextIsCancelled(t *testing.T) {
	fs, endpoint := newFakeSandbox(t, "")
	fs.healthCode.Store(int32(http.StatusServiceUnavailable))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := backend.WaitReady(ctx, endpoint, 30*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitReady after cancel = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("WaitReady took %s to notice a cancelled context", elapsed)
	}
}
