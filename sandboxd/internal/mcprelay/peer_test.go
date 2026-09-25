package mcprelay

// The test side of the channel: a relay (a Listener served over a real
// WebSocket, as server.go serves it) and a peer standing in for the control
// plane, written apart from channel.go so that the two do not share a
// mistake. The peer honours the window as a sender and checks it as a
// receiver, recording every protocol violation sandboxd commits.

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const wait = 20 * time.Second // generous: -race on a loaded machine

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type relay struct {
	t      *testing.T
	l      *Listener
	log    *syncBuffer
	srv    *httptest.Server
	served chan error // each Serve's return value
}

func newRelay(t *testing.T, after afterFunc) *relay {
	t.Helper()
	if after == nil {
		after = realAfter
	}
	logs := &syncBuffer{}
	l, err := listen("127.0.0.1:0", slog.New(slog.NewTextHandler(logs, nil)), after)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return relayOn(t, l, logs)
}

// relayOn serves l's channel over a real WebSocket, as server.go does.
func relayOn(t *testing.T, l *Listener, logs *syncBuffer) *relay {
	t.Helper()
	r := &relay{t: t, l: l, log: logs, served: make(chan error, 16)}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c, err := websocket.Accept(w, req, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		r.served <- l.Serve(req.Context(), c)
	}))
	t.Cleanup(func() { l.Close(); r.srv.Close() })
	return r
}

// attach dials the channel as the control plane does.
func (r *relay) attach() *peer {
	r.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r.t.Cleanup(cancel)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(r.srv.URL, "http")+ChannelPath, nil)
	if err != nil {
		r.t.Fatalf("dial the channel: %v", err)
	}
	c.SetReadLimit(MaxFrameBytes)
	r.t.Cleanup(func() { c.CloseNow() })
	p := &peer{ctx: ctx, conn: c, streams: map[uint32]*peerStream{},
		opens: make(chan *peerStream, 256), pongs: make(chan []byte, 8), ended: make(chan struct{})}
	go p.readLoop()
	return p
}

// client is a process in the sandbox dialling the relay.
func (r *relay) client() *net.TCPConn {
	r.t.Helper()
	c, err := net.Dial("tcp", r.l.Addr().String())
	if err != nil {
		r.t.Fatalf("dial the relay: %v", err)
	}
	r.t.Cleanup(func() { c.Close() })
	return c.(*net.TCPConn)
}

type peer struct {
	ctx     context.Context
	conn    *websocket.Conn
	mu      sync.Mutex
	streams map[uint32]*peerStream
	opens   chan *peerStream
	pongs   chan []byte
	ended   chan struct{}
	endErr  error // set before ended closes

	vmu        sync.Mutex
	violations []string
}

func (p *peer) violate(format string, args ...any) {
	p.vmu.Lock()
	p.violations = append(p.violations, fmt.Sprintf(format, args...))
	p.vmu.Unlock()
}

// checkClean fails t with every violation sandboxd committed on this peer.
func (p *peer) checkClean(t *testing.T) {
	t.Helper()
	p.vmu.Lock()
	defer p.vmu.Unlock()
	for _, v := range p.violations {
		t.Errorf("protocol violation by sandboxd: %s", v)
	}
}

func (p *peer) send(f Frame) error {
	b, err := Encode(f)
	if err != nil {
		return err
	}
	return p.conn.Write(p.ctx, websocket.MessageBinary, b)
}

func (p *peer) readLoop() {
	defer func() {
		close(p.ended)
		p.mu.Lock()
		for _, s := range p.streams {
			s.mu.Lock()
			if s.reset == "" {
				s.reset = "channel ended" // a waiting read or write stops now, not at its deadline
			}
			s.notify()
			s.mu.Unlock()
		}
		p.mu.Unlock()
	}()
	for {
		typ, b, err := p.conn.Read(p.ctx)
		if err != nil {
			p.endErr = err
			return
		}
		if typ != websocket.MessageBinary {
			p.violate("a text message")
			continue
		}
		f, err := Decode(b)
		if err != nil {
			p.violate("a malformed frame: %v", err)
			continue
		}
		p.handle(f)
	}
}

func (p *peer) handle(f Frame) {
	switch f.Kind {
	case KindOpen:
		if f.Stream%2 != 1 {
			p.violate("OPEN on even stream %d", f.Stream)
		}
		s := &peerStream{p: p, id: f.Stream, changed: make(chan struct{})}
		p.mu.Lock()
		if p.streams[f.Stream] != nil {
			p.violate("a second OPEN for stream %d", f.Stream)
		}
		p.streams[f.Stream] = s
		p.mu.Unlock()
		select {
		case p.opens <- s:
		default:
			p.violate("more OPENs than the test can hold")
		}
		return
	case KindPing:
		_ = p.send(Frame{Kind: KindPong, Payload: f.Payload})
		return
	case KindPong:
		p.pongs <- append([]byte(nil), f.Payload...)
		return
	}
	p.mu.Lock()
	s := p.streams[f.Stream]
	p.mu.Unlock()
	if s == nil {
		p.violate("%s on stream %d, never opened", f.Kind, f.Stream)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch f.Kind {
	case KindData:
		if s.eof {
			p.violate("DATA after CLOSE on stream %d", s.id)
		}
		s.recvOut += len(f.Payload)
		if s.recvOut > InitialWindow {
			p.violate("stream %d: %d bytes unacknowledged, past the %d-byte window", s.id, s.recvOut, InitialWindow)
		}
		s.buf.Write(f.Payload)
	case KindClose:
		s.eof = true
	case KindReset:
		s.reset = string(f.Payload)
		if s.reset == "" {
			s.reset = "(no reason)"
		}
	case KindWindow:
		s.sendOut -= int(WindowIncrement(f))
		if s.sendOut < 0 {
			p.violate("stream %d: WINDOW acknowledged %d bytes never sent", s.id, -s.sendOut)
		}
	}
	s.notify()
}

// nextOpen waits for sandboxd's next OPEN.
func (p *peer) nextOpen(t *testing.T) *peerStream {
	t.Helper()
	select {
	case s := <-p.opens:
		return s
	case <-time.After(wait):
		t.Fatalf("no OPEN within %s", wait)
		return nil
	}
}

// peerStream is the control plane's end of one stream.
type peerStream struct {
	p       *peer
	id      uint32
	mu      sync.Mutex
	changed chan struct{} // closed and replaced on every change
	buf     bytes.Buffer  // received and not yet read
	recvOut int           // received and not yet acknowledged
	unacked int           // read since the last WINDOW
	sendOut int           // sent and not yet acknowledged
	eof     bool
	reset   string
	blocked time.Time // non-zero while write waits for window
}

func (s *peerStream) notify() {
	close(s.changed)
	s.changed = make(chan struct{})
}

// waitFor waits until cond, evaluated under s.mu, holds or d passes.
func (s *peerStream) waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.After(d)
	for {
		s.mu.Lock()
		ok, ch := cond(), s.changed
		s.mu.Unlock()
		if ok {
			return true
		}
		select {
		case <-ch:
		case <-deadline:
			return false
		}
	}
}

// read returns n bytes, fewer only at EOF, acknowledging them as the control
// plane will: a WINDOW each time WindowAck bytes have been read. It returns
// errors rather than failing t, so a test's goroutines may call it.
func (s *peerStream) read(n int) ([]byte, error) {
	if !s.waitFor(wait, func() bool { return s.buf.Len() >= n || s.eof || s.reset != "" }) {
		return nil, fmt.Errorf("stream %d: %d of %d bytes within %s", s.id, s.bufLen(), n, wait)
	}
	s.mu.Lock()
	if s.reset != "" {
		reset := s.reset
		s.mu.Unlock()
		return nil, fmt.Errorf("stream %d reset: %s", s.id, reset)
	}
	got := s.buf.Next(min(n, s.buf.Len()))
	out := append([]byte(nil), got...)
	s.unacked += len(out)
	ack := 0
	if s.unacked >= WindowAck {
		ack, s.unacked = s.unacked, 0
		s.recvOut -= ack
	}
	s.mu.Unlock()
	if ack > 0 {
		if err := s.p.send(Frame{Kind: KindWindow, Stream: s.id, Payload: WindowPayload(uint32(ack))}); err != nil {
			return nil, fmt.Errorf("stream %d: send WINDOW: %w", s.id, err)
		}
	}
	return out, nil
}

// readToEOF reads until sandboxd's CLOSE, in bounded pieces.
func (s *peerStream) readToEOF() ([]byte, error) {
	var all []byte
	for {
		got, err := s.read(MaxPayload)
		if err != nil {
			return all, fmt.Errorf("after %d bytes: %w", len(all), err)
		}
		all = append(all, got...)
		s.mu.Lock()
		done := s.eof && s.buf.Len() == 0
		s.mu.Unlock()
		if done {
			return all, nil
		}
	}
}

func (s *peerStream) bufLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Len()
}

// write sends data in DATA frames, never past the window: it waits for
// sandboxd's WINDOW as the control plane must.
func (s *peerStream) write(data []byte) error {
	for len(data) > 0 {
		var room int
		ok := s.waitFor(wait, func() bool {
			room = InitialWindow - s.sendOut
			if room <= 0 && s.blocked.IsZero() {
				s.blocked = time.Now()
				s.notify() // wakes a test waiting to see the stall; this waiter takes the new channel
			}
			return room > 0 || s.reset != ""
		})
		s.mu.Lock()
		if ok && room > 0 {
			s.blocked = time.Time{}
		}
		reset := s.reset
		n := min(room, MaxPayload, len(data))
		if ok && reset == "" {
			s.sendOut += n
		}
		s.mu.Unlock()
		switch {
		case reset != "":
			return fmt.Errorf("stream %d reset: %s", s.id, reset)
		case !ok:
			return fmt.Errorf("stream %d: no window within %s", s.id, wait)
		}
		if err := s.p.send(Frame{Kind: KindData, Stream: s.id, Payload: data[:n]}); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (s *peerStream) closeWrite() error { return s.p.send(Frame{Kind: KindClose, Stream: s.id}) }

// noOpen asserts that no OPEN arrives within d.
func (p *peer) noOpen(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case s := <-p.opens:
		t.Fatalf("unexpected OPEN for stream %d", s.id)
	case <-time.After(d):
	}
}
