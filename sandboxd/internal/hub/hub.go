//go:build linux || darwin

// Package hub owns a single PTY for the lifetime of the sandbox and fans its
// output out to zero or more attached clients. Detaching does not stop the
// process — that is the whole point: a session you can leave and come back to.
package hub

import (
	"sync"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ptysession"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ring"
)

// subscriberBuffer is how many chunks a slow client may fall behind before the
// hub starts dropping its chunks. The hub must never block on a client: one
// stalled WebSocket would freeze the agent's terminal.
const subscriberBuffer = 256

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
	subs   map[int]chan []byte
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
		subs: make(map[int]chan []byte),
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
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			h.publish(chunk)
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
	for _, ch := range h.subs {
		select {
		case ch <- chunk:
		default:
			// Subscriber is not keeping up. Drop this chunk for that client
			// rather than stalling the PTY for everyone. The client still has
			// the ring on its next attach.
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
		// The subscriber channels are detached from the map under the lock
		// but closed after it. Detaching under the lock is what keeps that
		// safe: a concurrent publish only ever ranges over the new empty
		// map, so it cannot send on a channel about to close, and cancel's
		// h.subs[id] lookup misses, so it cannot close one twice. Closing
		// them here, after close(h.done), also orders the two for a reader:
		// close(ch) synchronizes-before the receive that observes it closed,
		// so by transitivity that reader's non-blocking Done check sees done
		// closed.
		h.mu.Lock()
		h.exitReason = reason
		h.closed = true
		close(h.done)
		subs := h.subs
		h.subs = map[int]chan []byte{}
		h.mu.Unlock()

		for _, ch := range subs {
			close(ch)
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
	h.mu.Lock()
	snapshot := h.buf.Snapshot()
	ch := make(chan []byte, subscriberBuffer)
	if h.closed {
		h.mu.Unlock()
		close(ch)
		return snapshot, ch, func() {}
	}
	id := h.nextID
	h.nextID++
	h.subs[id] = ch
	h.mu.Unlock()

	var once sync.Once
	return snapshot, ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if c, ok := h.subs[id]; ok {
				delete(h.subs, id)
				close(c)
			}
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
