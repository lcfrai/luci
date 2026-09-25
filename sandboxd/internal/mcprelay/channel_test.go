package mcprelay

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random: %v", err)
	}
	return b
}

// readAllLocal reads c to EOF, bounded by wait.
func readAllLocal(c net.Conn) ([]byte, error) {
	c.SetReadDeadline(time.Now().Add(wait))
	defer c.SetReadDeadline(time.Time{})
	return io.ReadAll(c)
}

// exchange moves up (client to peer) and down (peer to client) over one
// stream at once, each direction half-closed at its end, and reports the first
// difference.
func exchange(c *net.TCPConn, s *peerStream, up, down []byte) error {
	errs := make(chan error, 4)
	go func() {
		if _, err := c.Write(up); err != nil {
			errs <- fmt.Errorf("client write: %w", err)
			return
		}
		errs <- c.CloseWrite()
	}()
	go func() {
		got, err := s.readToEOF()
		switch {
		case err != nil:
			errs <- fmt.Errorf("peer read: %w", err)
		case !bytes.Equal(got, up):
			errs <- fmt.Errorf("stream %d upstream: %d bytes arrived, want %d, identical %v",
				s.id, len(got), len(up), bytes.Equal(got, up))
		default:
			errs <- nil
		}
	}()
	go func() {
		if err := s.write(down); err != nil {
			errs <- fmt.Errorf("peer write: %w", err)
			return
		}
		errs <- s.closeWrite()
	}()
	go func() {
		got, err := readAllLocal(c)
		switch {
		case err != nil:
			errs <- fmt.Errorf("client read: %w", err)
		case !bytes.Equal(got, down):
			errs <- fmt.Errorf("stream %d downstream: %d bytes arrived, want %d", s.id, len(got), len(down))
		default:
			errs <- nil
		}
	}()
	var first error
	for range 4 {
		if err := <-errs; err != nil && first == nil {
			first = err
		}
	}
	return first
}

func TestRoundTrip_bytes(t *testing.T) {
	const streams, size = 5, 1 << 20
	r := newRelay(t, nil)
	p := r.attach()
	var wg sync.WaitGroup
	errs := make(chan error, streams)
	for i := range streams {
		c := r.client()
		s := p.nextOpen(t)
		up, down := randomBytes(t, size), randomBytes(t, size)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := exchange(c, s, up, down); err != nil {
				errs <- fmt.Errorf("stream %d (client %d): %w", s.id, i, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	p.checkClean(t)
	waitLog(t, r, "reason=closed", streams)
}

// waitLog waits until the relay's log holds want at least n times.
func waitLog(t *testing.T, r *relay, want string, n int) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for strings.Count(r.log.String(), want) < n {
		if time.Now().After(deadline) {
			t.Fatalf("the log holds %q %d times, want %d:\n%s", want, strings.Count(r.log.String(), want), n, r.log)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestFlowControl_noHeadOfLine stalls one stream both ways: its local client
// never reads, and the peer never reads what the client sends. A second
// stream must still move 1 MiB each way, and sandboxd must never put more
// than the window in flight on the stalled one.
func TestFlowControl_noHeadOfLine(t *testing.T) {
	const flood = 16 << 20
	r := newRelay(t, nil)
	p := r.attach()
	stalled := r.client()
	stalled.SetReadBuffer(4 << 10)
	sa := p.nextOpen(t)
	moving := r.client()
	sb := p.nextOpen(t)

	go func() { _, _ = stalled.Write(make([]byte, flood)) }() // blocks once the window closes
	drained := make(chan error, 1)
	go func() { drained <- sa.write(make([]byte, flood)) }() // blocks once sandboxd stops acknowledging
	if !sa.waitFor(wait, func() bool { return !sa.blocked.IsZero() }) {
		t.Fatalf("the stalled stream never ran out of window: its local reader absorbed %d bytes", flood)
	}
	if !sa.waitFor(wait, func() bool { return sa.buf.Len() >= InitialWindow }) {
		t.Fatalf("sandboxd sent %d of the stalled client's bytes, want the whole %d-byte window", sa.bufLen(), InitialWindow)
	}

	if err := exchange(moving, sb, randomBytes(t, 1<<20), randomBytes(t, 1<<20)); err != nil {
		t.Fatalf("the moving stream: %v", err)
	}
	// The kernel may absorb more of the stalled stream meanwhile (Linux grows
	// a socket's buffers), so sandboxd may acknowledge a little more; what it
	// must not do is let the whole flood through a client that reads nothing.
	select {
	case err := <-drained:
		if err == nil {
			t.Fatalf("the stalled stream drained %d bytes to a client that never read: nothing was stalled", flood)
		}
	default:
	}
	sa.mu.Lock()
	queued := sa.buf.Len()
	sa.mu.Unlock()
	if queued > InitialWindow {
		t.Errorf("sandboxd sent %d bytes the peer never acknowledged, past the %d-byte window", queued, InitialWindow)
	}
	p.checkClean(t)
}

func TestPing_echoedAsPong(t *testing.T) {
	r := newRelay(t, nil)
	p := r.attach()
	want := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	if err := p.send(Frame{Kind: KindPing, Payload: want}); err != nil {
		t.Fatalf("send PING: %v", err)
	}
	select {
	case got := <-p.pongs:
		if !bytes.Equal(got, want) {
			t.Fatalf("PONG carried %v, want %v", got, want)
		}
	case <-time.After(wait):
		t.Fatalf("no PONG within %s", wait)
	}
}

// bareStream is a stream on a channel with no socket: what the read loop
// would send is left on ch.control for the test to read.
func bareStream(t *testing.T) (*stream, *channel, net.Conn) {
	t.Helper()
	ch := &channel{log: slog.New(slog.NewTextHandler(io.Discard, nil)), streams: map[uint32]*stream{},
		control: make(chan Frame, 8), quit: make(chan struct{})}
	local, far := net.Pipe()
	t.Cleanup(func() { local.Close(); far.Close() })
	s := newStream(ch, 1, local)
	ch.streams[1] = s
	return s, ch, far
}

func wantReset(t *testing.T, ch *channel, reason string) {
	t.Helper()
	select {
	case f := <-ch.control:
		if f.Kind != KindReset || f.Stream != 1 || string(f.Payload) != reason {
			t.Fatalf("sent %s stream %d %q, want RESET stream 1 %q", f.Kind, f.Stream, f.Payload, reason)
		}
	default:
		t.Fatalf("no RESET %q", reason)
	}
	if ch.streams[1] != nil {
		t.Errorf("the reset stream is still on the channel")
	}
}

// The peer must stay inside the window; a frame past it is a protocol error.
func TestWindow_protocolErrors(t *testing.T) {
	t.Run("data past the window", func(t *testing.T) {
		s, ch, _ := bareStream(t)
		for range InitialWindow / MaxPayload {
			s.data(make([]byte, MaxPayload))
		}
		if len(ch.control) != 0 {
			t.Fatalf("a full window, exactly, was refused: %v", <-ch.control)
		}
		s.data([]byte{1})
		wantReset(t, ch, reasonWindow)
	})
	t.Run("window for bytes never sent", func(t *testing.T) {
		s, ch, _ := bareStream(t)
		s.sendOut = 10
		s.window(10)
		if len(ch.control) != 0 {
			t.Fatalf("an exact acknowledgement was refused: %v", <-ch.control)
		}
		s.window(1)
		wantReset(t, ch, reasonBadWindow)
	})
	t.Run("data after close", func(t *testing.T) {
		s, ch, _ := bareStream(t)
		s.peerClose()
		s.data([]byte{1})
		wantReset(t, ch, reasonDataAfterClose)
	})
}
