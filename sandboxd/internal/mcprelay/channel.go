package mcprelay

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/coder/websocket"
)

// Why a stream or a local connection ends: logged, and sent as RESET reasons.
// Fixed strings only: nothing here ever quotes a stream byte (G5).
const (
	reasonTooMany        = "too many streams"
	reasonWindow         = "window exceeded"
	reasonBadWindow      = "window acknowledges bytes never sent"
	reasonDataAfterClose = "data after close"
	reasonLocalRead      = "local read failed"
	reasonLocalWrite     = "local write failed"
	reasonUnexpectedOpen = "sandboxd opens every stream"
	reasonIDs            = "stream ids exhausted"
)

var (
	errChannelDone    = errors.New("mcprelay: channel closed")
	errTooManyStreams = errors.New("mcprelay: " + reasonTooMany)
	errIDsExhausted   = errors.New("mcprelay: " + reasonIDs)
)

// channel multiplexes local connections over one WebSocket. It never parses a
// stream's bytes (G5): it moves them, counts them and honours the window.
type channel struct {
	ctx     context.Context
	conn    *websocket.Conn
	log     *slog.Logger
	detach  func(*channel)
	control chan Frame    // frames the read loop sends; one goroutine writes them
	quit    chan struct{} // closed by fail

	once    sync.Once
	mu      sync.Mutex
	streams map[uint32]*stream
	nextID  uint64 // the next odd stream id; uint64 so exhaustion is visible
	done    bool
}

func newChannel(ctx context.Context, conn *websocket.Conn, log *slog.Logger, detach func(*channel)) *channel {
	ch := &channel{
		ctx: ctx, conn: conn, log: log, detach: detach,
		control: make(chan Frame, 128), quit: make(chan struct{}),
		streams: map[uint32]*stream{}, nextID: 1,
	}
	go ch.writeControl()
	return ch
}

// write puts one frame on the socket. coder/websocket serialises concurrent
// writers (conn.go:31), so each stream writes its own frames. A failed write
// ends the channel: the socket is gone for every stream, not only this one.
func (ch *channel) write(f Frame) error {
	b, err := Encode(f)
	if err != nil {
		return err
	}
	if err := ch.conn.Write(ch.ctx, websocket.MessageBinary, b); err != nil {
		ch.fail("channel write failed")
		ch.conn.CloseNow()
		return errChannelDone
	}
	return nil
}

func (ch *channel) writeControl() {
	for {
		select {
		case f := <-ch.control:
			if ch.write(f) != nil {
				return
			}
		case <-ch.quit:
			return
		}
	}
}

// sendControl queues a frame without blocking. The read loop must never wait
// on the socket: a read loop blocked writing while its peer's read loop is
// blocked writing too is a deadlock only load finds. A full queue drops a
// PONG (PING is best-effort) and gives any other frame its own goroutine.
func (ch *channel) sendControl(f Frame) {
	select {
	case ch.control <- f:
	default:
		if f.Kind != KindPong {
			go ch.write(f)
		}
	}
}

// open makes c a stream: an id, an OPEN frame, and a goroutine per direction.
func (ch *channel) open(c net.Conn) error {
	ch.mu.Lock()
	if ch.done {
		ch.mu.Unlock()
		return errChannelDone
	}
	if len(ch.streams) >= MaxStreams {
		ch.mu.Unlock()
		ch.log.Warn("mcp relay: connection refused", "reason", reasonTooMany, "streams", MaxStreams)
		return errTooManyStreams
	}
	if ch.nextID > 1<<32-1 {
		ch.mu.Unlock()
		ch.log.Warn("mcp relay: connection refused", "reason", reasonIDs)
		return errIDsExhausted
	}
	id := uint32(ch.nextID)
	ch.nextID += 2
	s := newStream(ch, id, c)
	ch.streams[id] = s
	ch.mu.Unlock()

	if ch.write(Frame{Kind: KindOpen, Stream: id}) != nil {
		return nil // fail has already reset c with the channel's other streams
	}
	ch.log.Info("mcp stream opened", "stream", id)
	go s.upstream()
	go s.downstream()
	return nil
}

// run is the read loop, until the socket fails. It never blocks on a local
// connection: DATA is queued on its stream, bounded by the window, so one
// stalled reader cannot stall the channel.
func (ch *channel) run(ctx context.Context) error {
	for {
		typ, b, err := ch.conn.Read(ctx)
		if err != nil {
			return err
		}
		if typ != websocket.MessageBinary {
			ch.fail("text message")
			ch.conn.Close(websocket.StatusUnsupportedData, "binary frames only")
			return errors.New("mcprelay: a text message on the channel")
		}
		f, err := Decode(b)
		if err != nil {
			ch.fail("malformed frame")
			ch.conn.Close(websocket.StatusProtocolError, "malformed frame")
			return err
		}
		ch.handle(f)
	}
}

func (ch *channel) handle(f Frame) {
	switch f.Kind {
	case KindPing:
		ch.sendControl(Frame{Kind: KindPong, Payload: f.Payload})
		return
	case KindPong:
		return
	}
	ch.mu.Lock()
	s := ch.streams[f.Stream]
	ch.mu.Unlock()
	if s == nil {
		// A frame for a stream that has already ended here (the peer had not
		// seen the end yet) is dropped. An OPEN is refused: sandboxd opens
		// every stream, and a control plane that tries has a bug to see.
		if f.Kind == KindOpen {
			ch.sendControl(Frame{Kind: KindReset, Stream: f.Stream, Payload: []byte(reasonUnexpectedOpen)})
		}
		return
	}
	switch f.Kind {
	case KindOpen:
		s.reset(reasonUnexpectedOpen)
	case KindData:
		s.data(f.Payload)
	case KindClose:
		s.peerClose()
	case KindReset:
		s.abort("peer reset: " + string(f.Payload))
	case KindWindow:
		s.window(WindowIncrement(f))
	}
}

func (ch *channel) remove(id uint32) {
	ch.mu.Lock()
	delete(ch.streams, id)
	ch.mu.Unlock()
}

// fail ends the channel, once. It leaves the listener first, so no new
// connection lands here after it refuses opens, then resets every stream's
// local connection. It sends nothing: a failed socket cannot carry a RESET,
// and a superseded one is about to be closed with 4001.
func (ch *channel) fail(reason string) {
	ch.once.Do(func() {
		ch.detach(ch)
		ch.mu.Lock()
		ch.done = true
		ss := make([]*stream, 0, len(ch.streams))
		for _, s := range ch.streams {
			ss = append(ss, s)
		}
		ch.mu.Unlock()
		close(ch.quit)
		for _, s := range ss {
			s.abort(reason)
		}
		ch.log.Info("mcp channel ended", "reason", reason, "streams_reset", len(ss))
	})
}

// supersede ends a channel another has replaced: its streams are reset and
// its peer is told why with 4001, so an old control plane stops serving.
func (ch *channel) supersede() {
	ch.fail("superseded")
	go ch.conn.Close(StatusSuperseded, "superseded")
}

// stream is one local connection carried over the channel. Its two
// goroutines each own one direction; the read loop only queues and signals.
// No method writes to the socket while holding mu: fail takes each stream's
// mu, and a write that fails calls fail.
type stream struct {
	ch   *channel
	id   uint32
	conn net.Conn

	mu         sync.Mutex
	sendOut    int      // bytes sent on the channel and not yet acknowledged by a WINDOW
	recvOut    int      // bytes received from the channel and not yet acknowledged by us
	inbox      [][]byte // received and not yet written to conn
	peerClosed bool     // the peer sent CLOSE
	sentClose  bool     // conn reached EOF and we sent CLOSE
	wroteEOF   bool     // the peer's CLOSE was delivered: conn's write side is shut
	up, down   int64    // bytes conn → channel and channel → conn, for the log
	ended      bool

	windowGrew chan struct{} // cap 1
	inboxGrew  chan struct{} // cap 1
	done       chan struct{} // closed when the stream ends
}

func newStream(ch *channel, id uint32, c net.Conn) *stream {
	return &stream{ch: ch, id: id, conn: c,
		windowGrew: make(chan struct{}, 1), inboxGrew: make(chan struct{}, 1), done: make(chan struct{})}
}

func signal(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

// upstream copies conn to the channel, reading no more than the window has
// room for, so a peer that stops acknowledging stops this stream and only it.
func (s *stream) upstream() {
	buf := make([]byte, MaxPayload)
	for {
		room, ok := s.sendRoom()
		if !ok {
			return
		}
		n, err := s.conn.Read(buf[:min(room, MaxPayload)])
		if n > 0 {
			s.mu.Lock()
			s.sendOut += n
			s.up += int64(n)
			s.mu.Unlock()
			if s.ch.write(Frame{Kind: KindData, Stream: s.id, Payload: buf[:n]}) != nil {
				return
			}
		}
		if err != nil {
			switch {
			case s.isDone():
			case errors.Is(err, io.EOF):
				s.closeUp()
			default:
				s.reset(reasonLocalRead)
			}
			return
		}
	}
}

// sendRoom waits until the window has room, and reports it.
func (s *stream) sendRoom() (int, bool) {
	for {
		s.mu.Lock()
		room, ended := InitialWindow-s.sendOut, s.ended
		s.mu.Unlock()
		if ended {
			return 0, false
		}
		if room > 0 {
			return room, true
		}
		select {
		case <-s.windowGrew:
		case <-s.done:
			return 0, false
		}
	}
}

func (s *stream) closeUp() {
	if s.ch.write(Frame{Kind: KindClose, Stream: s.id}) != nil {
		return
	}
	s.mu.Lock()
	s.sentClose = true
	both := s.wroteEOF
	s.mu.Unlock()
	if both {
		s.end()
	}
}

// downstream writes the peer's DATA to conn and acknowledges it once
// WindowAck bytes have been handed over. It alone blocks on a slow reader.
func (s *stream) downstream() {
	consumed := 0
	for {
		s.mu.Lock()
		var chunk []byte
		if len(s.inbox) > 0 {
			chunk = s.inbox[0]
			s.inbox[0] = nil
			s.inbox = s.inbox[1:]
		}
		eof, ended := chunk == nil && s.peerClosed, s.ended
		s.mu.Unlock()
		switch {
		case ended:
			return
		case eof:
			closeWrite(s.conn)
			s.mu.Lock()
			s.wroteEOF = true
			both := s.sentClose
			s.mu.Unlock()
			if both {
				s.end()
			}
			return
		case chunk == nil:
			select {
			case <-s.inboxGrew:
			case <-s.done:
				return
			}
			continue
		}
		if _, err := s.conn.Write(chunk); err != nil {
			if !s.isDone() {
				s.reset(reasonLocalWrite)
			}
			return
		}
		consumed += len(chunk)
		s.mu.Lock()
		s.down += int64(len(chunk))
		if consumed >= WindowAck {
			s.recvOut -= consumed // before the WINDOW leaves: the peer may refill at once
		}
		s.mu.Unlock()
		if consumed >= WindowAck {
			if s.ch.write(Frame{Kind: KindWindow, Stream: s.id, Payload: WindowPayload(uint32(consumed))}) != nil {
				return
			}
			consumed = 0
		}
	}
}

// data queues a DATA payload; the read loop calls it and it never blocks.
func (s *stream) data(p []byte) {
	s.mu.Lock()
	var reason string
	switch {
	case s.ended:
		s.mu.Unlock()
		return
	case s.peerClosed:
		reason = reasonDataAfterClose
	case s.recvOut+len(p) > InitialWindow:
		reason = reasonWindow
	default:
		s.recvOut += len(p)
		s.inbox = append(s.inbox, p)
	}
	s.mu.Unlock()
	if reason != "" {
		s.reset(reason)
		return
	}
	signal(s.inboxGrew)
}

func (s *stream) peerClose() {
	s.mu.Lock()
	s.peerClosed = true
	s.mu.Unlock()
	signal(s.inboxGrew)
}

func (s *stream) window(n uint32) {
	s.mu.Lock()
	bad := int64(n) > int64(s.sendOut)
	if !bad {
		s.sendOut -= int(n)
	}
	s.mu.Unlock()
	if bad {
		s.reset(reasonBadWindow)
		return
	}
	signal(s.windowGrew)
}

// reset aborts the stream from this side and tells the peer why.
func (s *stream) reset(reason string) {
	if s.abort(reason) {
		s.ch.sendControl(Frame{Kind: KindReset, Stream: s.id, Payload: []byte(reason)})
	}
}

// abort ends the stream at once, conn reset; it reports whether it was the end.
func (s *stream) abort(reason string) bool {
	if !s.finish(reason) {
		return false
	}
	resetConn(s.conn)
	return true
}

// end is the clean finish: both sides have sent CLOSE and conn has both.
func (s *stream) end() {
	if s.finish("closed") {
		s.conn.Close()
	}
}

// finish marks the stream ended, once, and logs it: its id, its byte counts
// and why. Never a payload byte (G5, spec §7.7).
func (s *stream) finish(reason string) bool {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return false
	}
	s.ended = true
	s.inbox = nil
	up, down := s.up, s.down
	s.mu.Unlock()
	close(s.done)
	s.ch.remove(s.id)
	s.ch.log.Info("mcp stream closed", "stream", s.id, "bytes_up", up, "bytes_down", down, "reason", reason)
	return true
}

func (s *stream) isDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}
