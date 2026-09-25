package mcprelay

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestListen_loopbackOnly(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8731", ":8731", "[::]:8731", "localhost:8731", "10.0.0.1:8731", "8731"} {
		l, err := Listen(addr, nil)
		if err == nil {
			l.Close()
			t.Errorf("Listen(%q) succeeded; only a loopback IP may be bound (G4)", addr)
		}
	}
	l, err := Listen("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("Listen(127.0.0.1:0): %v", err)
	}
	defer l.Close()
	if a := l.Addr().(*net.TCPAddr); !a.IP.IsLoopback() {
		t.Errorf("bound %s", a)
	}
	if off, err := Listen(Off, nil); off != nil || err != nil {
		t.Errorf("Listen(%q) = %v, %v; want nil, nil", Off, off, err)
	}
}

// expectClosed asserts c is closed by the far end, not merely quiet.
func expectClosed(t *testing.T, c net.Conn, why string) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(wait))
	n, err := c.Read(make([]byte, 64))
	var ne net.Error
	switch {
	case err == nil:
		t.Fatalf("%s: read %d bytes from a connection that should be closed", why, n)
	case errors.As(err, &ne) && ne.Timeout():
		t.Fatalf("%s: the connection is still open after %s", why, wait)
	}
}

// expectOpen asserts c is still open: a short read times out.
func expectOpen(t *testing.T, c net.Conn, why string) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, err := c.Read(make([]byte, 64))
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("%s: want an open, quiet connection; read returned %v", why, err)
	}
	c.SetReadDeadline(time.Time{})
}

func readN(t *testing.T, c net.Conn, n int) []byte {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(wait))
	defer c.SetReadDeadline(time.Time{})
	b := make([]byte, n)
	if _, err := io.ReadFull(c, b); err != nil {
		t.Fatalf("read %d bytes: %v", n, err)
	}
	return b
}

func TestListener_holdsUntilChannel(t *testing.T) {
	r := newRelay(t, nil)
	c := r.client()
	req := []byte("POST /mcp HTTP/1.1\r\nHost: 127.0.0.1:8731\r\nContent-Length: 2\r\n\r\n{}")
	if _, err := c.Write(req); err != nil {
		t.Fatalf("write before any channel: %v", err)
	}
	time.Sleep(2 * time.Second) // the control plane is still waiting for /healthz
	p := r.attach()
	s := p.nextOpen(t)
	got, err := s.read(len(req))
	if err != nil || !bytes.Equal(got, req) {
		t.Fatalf("held request arrived as %q, %v; want %q", got, err, req)
	}
	resp := []byte("HTTP/1.1 202 Accepted\r\nContent-Length: 0\r\n\r\n")
	if err := s.write(resp); err != nil {
		t.Fatalf("peer write: %v", err)
	}
	if got := readN(t, c, len(resp)); !bytes.Equal(got, resp) {
		t.Fatalf("response arrived as %q", got)
	}
	p.checkClean(t)
}

// fakeClock stands in for time.AfterFunc.
type fakeClock struct {
	mu     sync.Mutex
	timers []*fakeTimer
	added  chan *fakeTimer
}

type fakeTimer struct {
	d       time.Duration
	f       func()
	stopped bool
}

func newFakeClock() *fakeClock { return &fakeClock{added: make(chan *fakeTimer, 128)} }

func (c *fakeClock) after(d time.Duration, f func()) func() bool {
	tm := &fakeTimer{d: d, f: f}
	c.mu.Lock()
	c.timers = append(c.timers, tm)
	c.mu.Unlock()
	c.added <- tm
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		was := !tm.stopped
		tm.stopped = true
		return was
	}
}

// fire runs every timer not stopped, as if d had passed for all of them.
func (c *fakeClock) fire() {
	c.mu.Lock()
	var due []*fakeTimer
	for _, tm := range c.timers {
		if !tm.stopped {
			tm.stopped = true
			due = append(due, tm)
		}
	}
	c.mu.Unlock()
	for _, tm := range due {
		tm.f()
	}
}

func TestListener_holdTimeout(t *testing.T) {
	clk := newFakeClock()
	r := newRelay(t, clk.after)
	c := r.client()
	select {
	case tm := <-clk.added:
		if tm.d != HoldTimeout {
			t.Fatalf("held for %s, want %s", tm.d, HoldTimeout)
		}
	case <-time.After(wait):
		t.Fatalf("the connection was never held")
	}
	expectOpen(t, c, "before the hold timeout")
	clk.fire()
	expectClosed(t, c, "after the hold timeout")
	p := r.attach()
	p.noOpen(t, 500*time.Millisecond)
	if !strings.Contains(r.log.String(), "no channel") {
		t.Errorf("the log does not say why the connection closed:\n%s", r.log)
	}
}

// flakyListener fails its first Accept, as a process out of file descriptors
// would, then accepts normally.
type flakyListener struct {
	net.Listener
	once sync.Once
}

func (f *flakyListener) Accept() (net.Conn, error) {
	fail := false
	f.once.Do(func() { fail = true })
	if fail {
		return nil, errors.New("accept: too many open files")
	}
	return f.Listener.Accept()
}

// A transient accept failure must not end the relay for the sandbox's life.
func TestListener_survivesAnAcceptError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	logs := &syncBuffer{}
	l := serveOn(&flakyListener{Listener: ln}, slog.New(slog.NewTextHandler(logs, nil)), realAfter)
	r := relayOn(t, l, logs)
	c := r.client()
	p := r.attach()
	s := p.nextOpen(t)
	if err := exchange(c, s, []byte("after the failure"), []byte("still served")); err != nil {
		t.Fatalf("a stream after an accept error: %v", err)
	}
	if !strings.Contains(logs.String(), "accept failed") {
		t.Errorf("the accept error was not logged:\n%s", logs)
	}
}

// expectReset asserts the far end aborted c (a TCP RST), not a clean EOF a
// client could take for a finished response.
func expectReset(t *testing.T, c net.Conn, why string) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(wait))
	_, err := c.Read(make([]byte, 64))
	var ne net.Error
	switch {
	case err == nil:
		t.Fatalf("%s: read from a connection that should be reset", why)
	case errors.As(err, &ne) && ne.Timeout():
		t.Fatalf("%s: the connection is still open after %s", why, wait)
	case errors.Is(err, io.EOF):
		t.Fatalf("%s: a clean EOF, want a reset", why)
	}
}

// dialExpectReset dials the relay and asserts the connection is reset. The
// reset may land before Dial returns: Go reads the socket's error only after
// the handshake, and sandboxd's accept-and-reset can come first (seen on
// Linux as "connect: connection reset by peer").
func dialExpectReset(t *testing.T, r *relay, why string) {
	t.Helper()
	c, err := net.Dial("tcp", r.l.Addr().String())
	if err != nil {
		// Windows reports WSAECONNRESET, which syscall names on Windows only.
		if !errors.Is(err, syscall.ECONNRESET) && !strings.Contains(err.Error(), "forcibly closed") {
			t.Fatalf("%s: dial failed with %v, want a reset", why, err)
		}
		return
	}
	defer c.Close()
	expectReset(t, c, why)
}

func TestMaxStreams(t *testing.T) {
	t.Run("on a channel", func(t *testing.T) {
		r := newRelay(t, nil)
		p := r.attach()
		var first *peerStream
		var firstConn net.Conn
		for i := range MaxStreams {
			c := r.client()
			s := p.nextOpen(t)
			if i == 0 {
				first, firstConn = s, c
			}
		}
		dialExpectReset(t, r, "the 65th connection")
		p.noOpen(t, 300*time.Millisecond)
		if !strings.Contains(r.log.String(), reasonTooMany) {
			t.Errorf("the log does not say %q:\n%s", reasonTooMany, r.log)
		}
		// The bound is on live streams: once one ends, a new connection opens.
		if err := p.send(Frame{Kind: KindReset, Stream: first.id, Payload: []byte("done")}); err != nil {
			t.Fatalf("send RESET: %v", err)
		}
		expectReset(t, firstConn, "the stream the peer reset")
		r.client()
		p.nextOpen(t)
		p.checkClean(t)
	})
	t.Run("held before any channel", func(t *testing.T) {
		clk := newFakeClock()
		r := newRelay(t, clk.after)
		for i := range MaxStreams {
			r.client()
			select {
			case <-clk.added:
			case <-time.After(wait):
				t.Fatalf("connection %d was never held", i+1)
			}
		}
		dialExpectReset(t, r, "the 65th held connection")
		p := r.attach()
		for range MaxStreams {
			p.nextOpen(t)
		}
		p.noOpen(t, 300*time.Millisecond)
	})
}

func TestSecondChannelSupersedes(t *testing.T) {
	r := newRelay(t, nil)
	p1 := r.attach()
	c1 := r.client()
	p1.nextOpen(t)
	p2 := r.attach()

	select {
	case <-p1.ended:
	case <-time.After(wait):
		t.Fatalf("the first channel is still open after a second attached")
	}
	if got := websocket.CloseStatus(p1.endErr); got != StatusSuperseded {
		t.Errorf("the first channel closed with %v (%v), want %d", got, p1.endErr, StatusSuperseded)
	}
	expectReset(t, c1, "a stream of the superseded channel")
	select {
	case <-r.served:
	case <-time.After(wait):
		t.Fatalf("the superseded channel's Serve never returned")
	}

	c2 := r.client()
	s2 := p2.nextOpen(t)
	if err := exchange(c2, s2, []byte("to the new channel"), []byte("from it")); err != nil {
		t.Fatalf("a stream on the second channel: %v", err)
	}
	p2.checkClean(t)
}

func TestLogs_payloadBlind(t *testing.T) {
	r := newRelay(t, nil)
	p := r.attach()
	secret := []byte("secret")
	c := r.client()
	s := p.nextOpen(t)
	if err := exchange(c, s, secret, secret); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	waitLog(t, r, "reason=closed", 1)
	log := r.log.String()
	for _, form := range []string{string(secret), hex.EncodeToString(secret),
		base64.StdEncoding.EncodeToString(secret), "115 101 99 114 101 116"} {
		if strings.Contains(log, form) {
			t.Errorf("the log carries the payload as %q:\n%s", form, log)
		}
	}
	for _, want := range []string{"stream=1", "bytes_up=6", "bytes_down=6"} {
		if !strings.Contains(log, want) {
			t.Errorf("the log lacks %q:\n%s", want, log)
		}
	}
	if bytes.Contains([]byte(log), secret) {
		t.Errorf("the log carries a payload byte run")
	}
}
