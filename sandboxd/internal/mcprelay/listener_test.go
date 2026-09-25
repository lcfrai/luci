package mcprelay

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"runtime"
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

// A channel usually ends because its peer drops it: the control plane
// restarts, or the network goes. Its streams must be reset then, idle ones
// included, and the next connection held for the next channel rather than
// opened on the dead one (review t18 I1: mutant MA, Serve without its final
// fail, survived every other test).
func TestChannelEndedByItsPeer(t *testing.T) {
	r := newRelay(t, nil)
	p1 := r.attach()
	c1 := r.client()
	p1.nextOpen(t)
	p1.conn.CloseNow()
	expectReset(t, c1, "an idle stream of a channel its peer dropped")
	select {
	case <-r.served:
	case <-time.After(wait):
		t.Fatalf("Serve did not return after its peer dropped the channel")
	}
	c2 := r.client()
	expectOpen(t, c2, "a connection made while no channel is attached")
	p2 := r.attach()
	s2 := p2.nextOpen(t)
	if err := exchange(c2, s2, []byte("held"), []byte("served")); err != nil {
		t.Fatalf("the held connection on the next channel: %v", err)
	}
	p2.checkClean(t)
}

// writeControlGoroutines counts live channel writer goroutines.
func writeControlGoroutines() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), "mcprelay.(*channel).writeControl")
		}
		buf = make([]byte, 2*len(buf))
	}
}

// Close ends the live channel (its peer told why, its streams reset), and a
// channel that attaches after Close is refused without leaving its writer
// goroutine behind (review t18 M7: mutant MD survived, and that goroutine
// leaked).
func TestListener_closeEndsItsChannel(t *testing.T) {
	r := newRelay(t, nil)
	p := r.attach()
	c := r.client()
	p.nextOpen(t)
	r.l.Close()
	select {
	case <-p.ended:
	case <-time.After(wait):
		t.Fatalf("the channel is still open after its relay closed")
	}
	if got := websocket.CloseStatus(p.endErr); got != websocket.StatusGoingAway {
		t.Errorf("the channel closed with %v (%v), want %d", got, p.endErr, websocket.StatusGoingAway)
	}
	expectReset(t, c, "a stream of the closed relay's channel")
	select {
	case <-r.served:
	case <-time.After(wait):
		t.Fatalf("the channel's Serve never returned after Close")
	}

	late := r.attach()
	select {
	case err := <-r.served:
		if !errors.Is(err, errRelayClosed) {
			t.Errorf("Serve after Close returned %v, want %v", err, errRelayClosed)
		}
	case <-time.After(wait):
		t.Fatalf("Serve after Close never returned")
	}
	select {
	case <-late.ended:
	case <-time.After(wait):
		t.Fatalf("a channel attached after Close is still open")
	}
	deadline := time.Now().Add(wait)
	for n := writeControlGoroutines(); n != 0; n = writeControlGoroutines() {
		if time.Now().After(deadline) {
			t.Fatalf("%d channel writer goroutines outlive a closed relay", n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Only sandboxd opens streams. An OPEN from the peer, for a new id or a live
// one, is a bug in the peer and ends the channel as a malformed frame does:
// answering each with a RESET let a peer that floods OPENs and reads nothing
// grow sendControl's overflow goroutines without bound (review t18 M6).
func TestPeerOpen_endsTheChannel(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   func(live uint32) uint32
	}{
		{"a new id", func(uint32) uint32 { return 2 }},
		{"a live id", func(live uint32) uint32 { return live }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRelay(t, nil)
			p := r.attach()
			c := r.client()
			s := p.nextOpen(t)
			if err := p.send(Frame{Kind: KindOpen, Stream: tc.id(s.id)}); err != nil {
				t.Fatalf("send OPEN: %v", err)
			}
			select {
			case <-p.ended:
			case <-time.After(wait):
				t.Fatalf("the channel is still open after the peer sent an OPEN")
			}
			if got := websocket.CloseStatus(p.endErr); got != websocket.StatusProtocolError {
				t.Errorf("the channel closed with %v (%v), want %d", got, p.endErr, websocket.StatusProtocolError)
			}
			expectReset(t, c, "a stream of the channel the peer broke")
			select {
			case <-r.served:
			case <-time.After(wait):
				t.Fatalf("Serve never returned")
			}
		})
	}
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
	// A RESET's reason is the peer's text, not sandboxd's, so G5 must not rest
	// on what the peer puts there: a reason that is not one of the fixed ones
	// is logged by its length only (review t18 M4). A fixed one is logged.
	for _, reason := range [][]byte{secret, []byte(reasonTooMany)} {
		cr := r.client()
		sr := p.nextOpen(t)
		if err := p.send(Frame{Kind: KindReset, Stream: sr.id, Payload: reason}); err != nil {
			t.Fatalf("send RESET: %v", err)
		}
		expectReset(t, cr, "a stream the peer reset")
	}
	waitLog(t, r, "mcp stream closed", 3)
	log := r.log.String()
	for _, want := range []string{"peer reset: a 6-byte reason", "peer reset: " + reasonTooMany} {
		if !strings.Contains(log, want) {
			t.Errorf("the log lacks %q:\n%s", want, log)
		}
	}
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
