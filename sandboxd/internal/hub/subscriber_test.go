//go:build linux || darwin

package hub

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func drainSubscriber(t *testing.T, out <-chan []byte) []byte {
	t.Helper()
	var got []byte
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b, ok := <-out:
			if !ok {
				return got
			}
			if len(b) == 0 || len(b) > subscriberPageBytes {
				t.Fatalf("live frame length %d is outside the bounded page size", len(b))
			}
			got = append(got, b...)
		case <-deadline:
			t.Fatal("subscriber did not drain or close")
		}
	}
}

func TestFragmentedPasteDoesNotOverflowBeforeItsByteBudget(t *testing.T) {
	h := newTestHub()
	_, out, overflow, cancel := h.SubscribeWithOverflow()
	defer cancel()
	want := make([]byte, 100*1024)
	input := []byte{0}
	// No reader is scheduled until the whole finite paste is queued. The old
	// 32-fragment channel fails after 33 bytes here, irrespective of capacity.
	for i := range want {
		input[0] = byte(i % 251)
		want[i] = input[0]
		h.publish(input)
	}
	input[0] = 255 // Published bytes must not alias reused PTY read storage.
	select {
	case <-overflow:
		t.Fatal("100 KiB of one-byte PTY fragments exceeded a 1 MiB byte budget")
	default:
	}
	h.finish("exit status 0")
	if got := drainSubscriber(t, out); !bytes.Equal(got, want) {
		t.Fatalf("fragmented output differs: received %d/%d bytes", len(got), len(want))
	}
	select {
	case <-h.Done():
	default:
		t.Fatal("stream closed before process completion became visible")
	}
}

func TestSubscriberBudgetIncludesBlockedOfferAndBoundsAllocation(t *testing.T) {
	s := newSubscriber()
	defer s.cancel()
	for i := 0; i < subscriberByteBudget; i++ {
		if !s.enqueue([]byte{byte(i)}) {
			t.Fatalf("byte budget exhausted early at %d bytes", i)
		}
	}
	s.mu.Lock()
	pending := s.pending
	allocated := subscriberPageBytes // At most one offered page outside pages.
	for _, page := range s.pages {
		allocated += cap(page)
	}
	pages := len(s.pages)
	s.mu.Unlock()
	if pending != subscriberByteBudget || allocated > subscriberByteBudget+subscriberPageBytes || pages > subscriberByteBudget/subscriberPageBytes {
		t.Fatalf("queue bound: pending=%d allocated-upper-bound=%d pages=%d", pending, allocated, pages)
	}
	if s.enqueue([]byte("!")) {
		t.Fatal("byte beyond the full payload budget was accepted")
	}
	select {
	case <-s.overflow:
	default:
		t.Fatal("budget overflow was not explicit")
	}
	drainSubscriber(t, s.out)
	if s.enqueue([]byte("later")) {
		t.Fatal("overflowed stream resumed with a gap")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != 0 || len(s.pages) != 0 {
		t.Fatal("closed subscriber retained queued output")
	}
}

func TestCancelAfterFinishInterruptsUndrainedSubscriber(t *testing.T) {
	h := newTestHub()
	_, out, overflow, cancel := h.SubscribeWithOverflow()
	h.publish(bytes.Repeat([]byte("x"), 100*1024))
	h.finish("exit status 3")
	// finish removed the view from the live map. Cancel must still stop its
	// relay; no network receiver is required to drain output after disconnect.
	cancel()
	cancel()
	drainSubscriber(t, out)
	select {
	case <-overflow:
		t.Fatal("normal cancellation was misreported as overload")
	default:
	}
	if h.ExitReason() != "exit status 3" {
		t.Fatal("cancelling a completed view changed its process exit reason")
	}
}

func TestSubscriberCancelFinishAndPublishRaceClosesOnce(t *testing.T) {
	for n := 0; n < 100; n++ {
		h := newTestHub()
		_, out, _, cancel := h.SubscribeWithOverflow()
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				h.publish([]byte("output"))
			}
		}()
		go func() { defer wg.Done(); h.finish("exit status 0") }()
		go func() { defer wg.Done(); cancel(); cancel() }()
		wg.Wait()
		drainSubscriber(t, out)
	}
}

func TestBlockedOneByteOfferAndTailStayBoundedAndCancelReleasesBoth(t *testing.T) {
	s := newSubscriber()
	defer s.cancel()
	if !s.enqueue([]byte("a")) {
		t.Fatal("first byte refused")
	}
	// With no receiver, pending=1/pages=0 proves the relay holds the first
	// page outside the queue. Force a second tiny page rather than relying on
	// the scheduler to produce this worst-slack shape during a flood.
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		offered := s.pending == 1 && len(s.pages) == 0
		s.mu.Unlock()
		if offered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("relay never offered its first page")
		}
		time.Sleep(time.Millisecond)
	}
	if !s.enqueue([]byte("b")) {
		t.Fatal("tail byte refused")
	}
	s.mu.Lock()
	pending, count := s.pending, len(s.pages)
	allocated := subscriberPageBytes
	for _, page := range s.pages {
		allocated += cap(page)
	}
	s.mu.Unlock()
	if pending != 2 || count != 1 || allocated != 2*subscriberPageBytes {
		t.Fatalf("partial-page shape: pending=%d queued-pages=%d allocated=%d", pending, count, allocated)
	}
	s.finish()
	s.cancel()
	drainSubscriber(t, s.out)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != 0 || len(s.pages) != 0 {
		t.Fatal("interrupted delivery retained the offered page or tail")
	}
}
