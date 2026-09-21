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
	for _, b := range p {
		end := (r.start + r.size) % c
		r.buf[end] = b
		if r.full {
			r.start = (r.start + 1) % c
		} else {
			r.size++
			if r.size == c {
				r.full = true
			}
		}
	}
	return total, nil
}

// Snapshot returns a copy of the retained bytes, oldest first.
func (r *Ring) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]byte, r.size)
	c := len(r.buf)
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(r.start+i)%c]
	}
	return out
}

// Len reports how many bytes are currently retained.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}
