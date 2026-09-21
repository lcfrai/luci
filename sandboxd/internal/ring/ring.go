// Package ring holds the most recent N bytes written to it, discarding the
// oldest. It is the scrollback a re-attaching client replays. It knows nothing
// about terminal semantics: bytes in, the same bytes out.
package ring

import "sync"

// DefaultCapacity is the scrollback retained per session. 256 KiB is roughly
// 3000 lines of dense agent output — enough to re-establish context on return
// without holding a whole transcript in the sandbox's memory.
const DefaultCapacity = 256 * 1024

// Ring is a fixed-capacity, overwrite-oldest byte buffer. Safe for concurrent
// use.
type Ring struct {
	mu    sync.Mutex
	buf   []byte
	start int  // index of the oldest byte
	size  int  // bytes currently retained
	full  bool // true once size == cap(buf)
}

// New returns a Ring retaining at most capacity bytes. A capacity below 1 is
// raised to 1 so the zero case cannot panic.
func New(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{buf: make([]byte, capacity)}
}

// Write appends p, discarding the oldest bytes if necessary. It always reports
// len(p) consumed and never returns an error, as io.Writer requires of a sink
// that cannot fail.
func (r *Ring) Write(p []byte) (int, error) {
	total := len(p)
	if total == 0 {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	c := len(r.buf)
	// Only the last c bytes can survive; drop the rest up front.
	if total > c {
		p = p[total-c:]
	}
	n := len(p)

	// p lands across at most two contiguous runs: from the write cursor to
	// the end of the backing array, then (if it wraps) from the start. Two
	// copies do what a per-byte loop did, without a modulo per byte.
	end := (r.start + r.size) % c
	head := copy(r.buf[end:], p)
	if head < n {
		copy(r.buf, p[head:])
	}

	if r.full {
		// Every byte just written overwrote a live one, so the oldest
		// surviving byte moves forward by the same count.
		r.start = (r.start + n) % c
		return total, nil
	}
	r.size += n
	if r.size >= c {
		// This write pushed size past capacity: the excess overwrote the
		// oldest bytes, so size caps at capacity and start advances past
		// what it overwrote.
		overflow := r.size - c
		r.size = c
		r.full = true
		r.start = (r.start + overflow) % c
	}
	return total, nil
}

// Snapshot returns a copy of the retained bytes, oldest first.
func (r *Ring) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]byte, r.size)
	// The retained bytes may wrap the backing array once; copy the run up
	// to its end, then whatever spilled back around to the start.
	head := copy(out, r.buf[r.start:])
	if head < r.size {
		copy(out[head:], r.buf[:r.size-head])
	}
	return out
}

// Len reports how many bytes are currently retained.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}
