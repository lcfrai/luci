package mcprelay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Off, given as the address, disables the relay: Listen returns a nil
// *Listener and no error, and the server then answers /mcp-channel with 404.
const Off = "off"

// HoldTimeout bounds how long an accepted connection waits for a channel.
const HoldTimeout = 30 * time.Second

// StatusSuperseded closes a channel that a newer channel has replaced.
const StatusSuperseded websocket.StatusCode = 4001

// Listener accepts local connections on ListenAddr and holds each until a
// channel is attached, for at most HoldTimeout: the harness starts with the
// PTY (server.New starts it) and dials MCP at once, before
// the control plane has even seen /healthz answer. A connection still
// unassigned after HoldTimeout is closed.
//
// The listener takes no credential (plan G4): every process in the sandbox
// may use it, as the sandbox's session. The channel is what is authenticated,
// by the attach token on /mcp-channel.
type Listener struct {
	ln    net.Listener
	log   *slog.Logger
	after afterFunc

	mu     sync.Mutex
	held   []*heldConn // accepted with no channel, oldest first; at most MaxStreams
	cur    *channel    // the channel new connections join; nil when none
	closed bool
}

type heldConn struct {
	c    net.Conn
	stop func() bool
}

// afterFunc is time.AfterFunc's shape, injectable so a test need not wait
// HoldTimeout out.
type afterFunc func(d time.Duration, f func()) (stop func() bool)

func realAfter(d time.Duration, f func()) func() bool { return time.AfterFunc(d, f).Stop }

// Listen binds addr, which must be a loopback IP and port; any other address,
// a host name included, is an error (G4). Off disables the relay.
func Listen(addr string, log *slog.Logger) (*Listener, error) {
	return listen(addr, log, realAfter)
}

func listen(addr string, log *slog.Logger, after afterFunc) (*Listener, error) {
	if addr == Off {
		return nil, nil
	}
	if err := loopbackOnly(addr); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mcprelay: listen %s: %w", addr, err)
	}
	if a, ok := ln.Addr().(*net.TCPAddr); !ok || !a.IP.IsLoopback() {
		ln.Close()
		return nil, fmt.Errorf("mcprelay: %s bound %s, which is not loopback", addr, ln.Addr())
	}
	return serveOn(ln, log, after), nil
}

// serveOn starts the accept loop on a bound listener; a test hands it one
// whose Accept fails, which a real socket cannot be made to do on demand.
func serveOn(ln net.Listener, log *slog.Logger, after afterFunc) *Listener {
	if log == nil {
		log = slog.Default()
	}
	l := &Listener{ln: ln, log: log, after: after}
	go l.accept()
	return l
}

func loopbackOnly(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("mcprelay: listen address %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("mcprelay: listen address %q is not a loopback IP; "+
			"the relay must be reachable only inside the sandbox (G4)", addr)
	}
	return nil
}

// Addr is the bound address.
func (l *Listener) Addr() net.Addr { return l.ln.Addr() }

func (l *Listener) accept() {
	for {
		c, err := l.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// A transient accept failure (EMFILE, say) must not end the relay
			// for the sandbox's lifetime; back off and keep accepting.
			l.log.Warn("mcp relay: accept failed", "err", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		l.dispatch(c)
	}
}

// dispatch gives c to the current channel, or holds it until one attaches.
// A channel that ended between the lookup and the open has already left the
// listener (channel.fail detaches before it refuses opens), so the retry
// finds its successor or holds c; it cannot spin on a dead channel.
func (l *Listener) dispatch(c net.Conn) {
	for {
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			resetConn(c)
			return
		}
		ch := l.cur
		if ch == nil {
			if len(l.held) >= MaxStreams {
				l.mu.Unlock()
				l.log.Warn("mcp relay: connection refused", "reason", reasonTooMany, "held", MaxStreams)
				resetConn(c)
				return
			}
			h := &heldConn{c: c}
			h.stop = l.after(HoldTimeout, func() { l.expire(h) })
			l.held = append(l.held, h)
			l.mu.Unlock()
			return
		}
		l.mu.Unlock()
		err := ch.open(c)
		if err == nil {
			return
		}
		if !errors.Is(err, errChannelDone) {
			resetConn(c) // too many streams, or ids exhausted: open logged why
			return
		}
	}
}

// expire closes h if it is still waiting for a channel.
func (l *Listener) expire(h *heldConn) {
	l.mu.Lock()
	found := false
	for i, x := range l.held {
		if x == h {
			l.held = append(l.held[:i], l.held[i+1:]...)
			found = true
			break
		}
	}
	l.mu.Unlock()
	if found {
		l.log.Info("mcp relay: held connection closed", "reason", "no channel", "hold", HoldTimeout.String())
		h.c.Close()
	}
}

// detach removes ch as the current channel, if it still is.
func (l *Listener) detach(ch *channel) {
	l.mu.Lock()
	if l.cur == ch {
		l.cur = nil
	}
	l.mu.Unlock()
}

// errRelayClosed is Serve's answer once the Listener is closed.
var errRelayClosed = errors.New("mcprelay: relay closed")

// Serve runs one channel: every held and every new connection becomes a
// stream over conn until conn fails. A second channel replaces the first,
// which is closed with status 4001 "superseded"; streams of the old channel
// are reset. Serve returns when conn closes.
//
// Serve sets conn's read limit itself: a full DATA frame is MaxFrameBytes,
// past coder/websocket's 32 KiB default, and the caller must not have to
// remember (backend.MaxMessageBytes' rule, applied to this channel).
func (l *Listener) Serve(ctx context.Context, conn *websocket.Conn) error {
	conn.SetReadLimit(MaxFrameBytes)
	ch := newChannel(ctx, conn, l.log, l.detach)
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		ch.fail("relay closed") // ends the writer newChannel started
		conn.Close(websocket.StatusGoingAway, "relay closed")
		return errRelayClosed
	}
	old := l.cur
	l.cur = ch
	held := l.held
	l.held = nil
	l.mu.Unlock()

	if old != nil {
		old.supersede()
	}
	l.log.Info("mcp channel attached", "held", len(held), "superseded", old != nil)
	for _, h := range held {
		h.stop()
		if err := ch.open(h.c); err != nil {
			if errors.Is(err, errChannelDone) {
				l.dispatch(h.c) // ch was itself superseded meanwhile
			} else {
				resetConn(h.c)
			}
		}
	}
	err := ch.run(ctx)
	ch.fail("channel closed")
	return err
}

// Close stops accepting, closes every held connection and ends the current
// channel. It does not wait for the channel's Serve to return.
func (l *Listener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	held, cur := l.held, l.cur
	l.held, l.cur = nil, nil
	l.mu.Unlock()

	err := l.ln.Close()
	for _, h := range held {
		h.stop()
		h.c.Close()
	}
	if cur != nil {
		cur.fail("relay closed")
		go cur.conn.Close(websocket.StatusGoingAway, "relay closed")
	}
	return err
}

// resetConn aborts a local connection: SO_LINGER 0 turns the close into a
// TCP RST, so the client learns at once that its stream is gone rather than
// reading a clean EOF it could mistake for a finished response.
func resetConn(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	_ = c.Close()
}
