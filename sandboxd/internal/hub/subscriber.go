//go:build linux || darwin

package hub

import "sync"

const (
	// Admission counts bytes, not PTY reads: a finite paste can be echoed as
	// thousands of tiny reads before the websocket writer gets scheduled.
	subscriberByteBudget = 1 << 20
	subscriberPageBytes  = 32 << 10
)

// subscriber coalesces output into bounded pages. pending includes the page
// currently offered to out, until a receiver accepts it. There is at most
// 1 MiB of pending payload. Two partially occupied pages (the queued tail
// and the offered page) can have almost 64 KiB of slack at low occupancy;
// total allocated page storage is bounded by 1 MiB + 32 KiB because the byte
// budget is a page multiple and all other queued pages are full. The websocket
// writer may separately hold one accepted page; replay has its own bounded
// scrollback allocation. Queue metadata holds at most 32 page references.
//
// Only run closes out. finish drains accepted output after process exit;
// cancel and overflow interrupt even a blocked delivery and discard the queue.
// No subscriber ever blocks the PTY publisher on a network consumer.
type subscriber struct {
	out      chan []byte
	overflow chan struct{}
	wake     chan struct{}
	stop     chan struct{}

	mu       sync.Mutex
	pages    [][]byte
	pending  int
	draining bool
	stopped  bool
}

func newSubscriber() *subscriber {
	s := &subscriber{out: make(chan []byte), overflow: make(chan struct{}), wake: make(chan struct{}, 1), stop: make(chan struct{})}
	go s.run()
	return s
}

func (s *subscriber) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// enqueue copies the caller's bytes before returning; PTY read storage can be
// reused immediately. A rejected write explicitly retires this entire stream.
func (s *subscriber) enqueue(p []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.draining {
		return false
	}
	if len(p) > subscriberByteBudget-s.pending {
		close(s.overflow)
		s.cancelLocked()
		return false
	}
	s.pending += len(p)
	for len(p) > 0 {
		last := len(s.pages) - 1
		if last < 0 || len(s.pages[last]) == subscriberPageBytes {
			s.pages = append(s.pages, make([]byte, 0, subscriberPageBytes))
			last++
		}
		n := min(len(p), subscriberPageBytes-len(s.pages[last]))
		s.pages[last] = append(s.pages[last], p[:n]...)
		p = p[n:]
	}
	s.signal()
	return true
}

func (s *subscriber) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.draining = true
	s.signal()
}

func (s *subscriber) cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelLocked()
}

func (s *subscriber) cancelLocked() {
	if !s.stopped {
		s.stopped = true
		s.pages = nil
		close(s.stop)
	}
}

func (s *subscriber) run() {
	defer func() {
		s.mu.Lock()
		s.pages = nil
		s.pending = 0
		s.mu.Unlock()
		close(s.out)
	}()
	for {
		s.mu.Lock()
		if s.stopped || len(s.pages) == 0 && s.draining {
			s.mu.Unlock()
			return
		}
		if len(s.pages) == 0 {
			s.mu.Unlock()
			select {
			case <-s.wake:
			case <-s.stop:
				return
			}
			continue
		}
		page := s.pages[0]
		s.pages[0] = nil
		s.pages = s.pages[1:]
		if len(s.pages) == 0 {
			s.pages = nil
		}
		s.mu.Unlock()

		select {
		case s.out <- page:
			s.mu.Lock()
			s.pending -= len(page)
			s.mu.Unlock()
		case <-s.stop:
			return
		}
	}
}
