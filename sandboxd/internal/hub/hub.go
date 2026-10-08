//go:build linux || darwin

// Package hub owns a single PTY for the lifetime of the sandbox and fans its
// output out to zero or more attached clients. Detaching does not stop the
// process — that is the whole point: a session you can leave and come back to.
package hub

import (
	"sync"

	"github.com/lcfrai/luci/sandboxd/internal/ptysession"
	"github.com/lcfrai/luci/sandboxd/internal/ring"
)

// Config describes the process the hub runs.
type Config struct {
	Command    string
	Args       []string
	Env        []string
	Scrollback int // bytes retained for replay; 0 means ring.DefaultCapacity
}

// Hub is one PTY plus its scrollback and subscribers.
type Hub struct {
	sess *ptysession.Session
	buf  *ring.Ring

	mu     sync.Mutex
	subs   map[int]*subscriber
	nextID int
	closed bool

	done       chan struct{}
	doneOnce   sync.Once
	exitReason string
}

// Start launches the command in a PTY and begins pumping its output into the
// scrollback and to any subscribers.
func Start(cfg Config) (*Hub, error) {
	capacity := cfg.Scrollback
	if capacity <= 0 {
		capacity = ring.DefaultCapacity
	}
	sess, err := ptysession.Start(cfg.Command, cfg.Args, cfg.Env)
	if err != nil {
		return nil, err
	}
	h := &Hub{
		sess: sess,
		buf:  ring.New(capacity),
		subs: make(map[int]*subscriber),
		done: make(chan struct{}),
	}
	go h.pump()
	return h, nil
}

func (h *Hub) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := h.sess.Read(buf)
		if n > 0 {
			h.publish(buf[:n])
		}
		if err != nil {
			h.finish(h.reapExitReason())
			return
		}
	}
}

// publish appends chunk to the scrollback and fans it out to subscribers
// under one lock. Splitting those into two acquisitions would let a Subscribe
// land between them and see the chunk twice — once in its replay, once live
// — or, symmetrically, miss it from both.
func (h *Hub) publish(chunk []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf.Write(chunk)
	for id, sub := range h.subs {
		if !sub.enqueue(chunk) {
			delete(h.subs, id)
		}
	}
}

// reapExitReason waits for the child so it never lingers as a zombie in the
// sandbox, and turns its own exit status into the sentence a client renders
// in an Exit frame. The read error that got us here is not that sentence —
// it is the PTY closing, which reads as a plain I/O error and says nothing
// about how the process actually ended.
func (h *Hub) reapExitReason() string {
	if err := h.sess.Wait(); err != nil {
		return err.Error()
	}
	return "exit status 0"
}

func (h *Hub) finish(reason string) {
	h.doneOnce.Do(func() {
		// done is closed inside the same critical section that sets closed,
		// so the two can never be observed out of step. A Subscribe that
		// sees closed hands back a pre-closed channel and will never deliver
		// a live chunk; if done were closed in a later critical section,
		// that client could check Done, still read it as open, and report
		// the exit as a plain detach — no Exit frame for a process that has
		// already gone. Holding mu across the close is safe because close
		// never blocks and nothing waits on done under mu.
		//
		// Remove subscriptions while publishing is excluded. Their relays drain
		// accepted bytes, then close out only after done is already closed.
		// A later cancel still stops that relay even though it left the map.
		h.mu.Lock()
		h.exitReason = reason
		h.closed = true
		close(h.done)
		subs := h.subs
		h.subs = map[int]*subscriber{}
		h.mu.Unlock()

		for _, sub := range subs {
			sub.finish()
		}
	})
}

// Subscribe registers a client. It returns the scrollback to replay first, a
// channel of live chunks, and a cancel function that detaches without stopping
// the process. The channel is closed when the process exits.
//
// The replay is captured under the same lock that registers the subscriber, so
// no chunk can slip between the two and be lost or duplicated out of order.
func (h *Hub) Subscribe() (replay []byte, out <-chan []byte, cancel func()) {
	replay, out, _, cancel = h.SubscribeWithOverflow()
	return replay, out, cancel
}

// SubscribeWithOverflow reports lost output separately from a process exit or
// normal detach. Once overflow closes, the caller must stop this stream and
// explicitly reconnect for the bounded recent replay; the process stays alive.
func (h *Hub) SubscribeWithOverflow() (replay []byte, out <-chan []byte, overflow <-chan struct{}, cancel func()) {
	h.mu.Lock()
	snapshot := h.buf.Snapshot()
	if h.closed {
		h.mu.Unlock()
		ch := make(chan []byte)
		close(ch)
		return snapshot, ch, make(chan struct{}), func() {}
	}
	sub := newSubscriber()
	id := h.nextID
	h.nextID++
	h.subs[id] = sub
	h.mu.Unlock()

	var once sync.Once
	return snapshot, sub.out, sub.overflow, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			delete(h.subs, id)
			sub.cancel()
		})
	}
}

// Write sends bytes to the process as if typed.
func (h *Hub) Write(p []byte) (int, error) { return h.sess.Write(p) }

// Resize informs the process that its terminal dimensions changed.
func (h *Hub) Resize(cols, rows uint16) error { return h.sess.Resize(cols, rows) }

// Done is closed when the process exits.
func (h *Hub) Done() <-chan struct{} { return h.done }

// ExitReason describes why the process ended. Empty until Done is closed.
func (h *Hub) ExitReason() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitReason
}

// Close kills the process and releases the PTY. Only the process entrypoint
// calls this — an attach handler must call its subscription's cancel instead.
func (h *Hub) Close() error {
	err := h.sess.Close()
	h.finish("closed")
	return err
}
