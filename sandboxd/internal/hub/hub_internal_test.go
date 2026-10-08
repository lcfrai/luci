//go:build linux || darwin

package hub

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/sandboxd/internal/ring"
)

// newTestHub builds a Hub with no PTY behind it, so these tests exercise the
// fan-out path (publish/Subscribe) directly and deterministically, with no
// dependency on a real process's timing.
func newTestHub() *Hub {
	return &Hub{
		buf:  ring.New(ring.DefaultCapacity),
		subs: make(map[int]*subscriber),
		done: make(chan struct{}),
	}
}

// throttleToRoom spins until ch has room for at least one more send. It gives
// a reader that is momentarily slow to be scheduled a chance to catch up, so
// a writer that only ever runs slightly ahead of a draining reader can't
// trigger subscriberBuffer's explicit overflow-disconnect path by accident —
// these tests want to pin the atomicity and overflow behaviours on purpose,
// not stumble into overflow through their own scheduling luck.
func throttleToRoom(ch <-chan []byte) {
	for len(ch) >= subscriberBuffer-1 {
		runtime.Gosched()
	}
}

// TestSubscribeRacingThePumpNeverDuplicatesOrDropsAChunk pins the invariant
// Subscribe's doc comment promises: a chunk lands in exactly one place
// relative to a concurrent Subscribe — either in the replay or live, never
// both, never neither.
func TestSubscribeRacingThePumpNeverDuplicatesOrDropsAChunk(t *testing.T) {
	h := newTestHub()
	const n = 2000
	last := []byte(strconv.Itoa(n-1) + "\n")

	// Handed the subscriber's channel once it exists, so the writer can keep
	// itself from outrunning it (see throttleToRoom) without a data race on
	// a plain shared variable.
	subHandoff := make(chan (<-chan []byte), 1)

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		var out <-chan []byte
		for i := 0; i < n; i++ {
			h.publish([]byte(strconv.Itoa(i) + "\n"))
			if out == nil {
				select {
				case out = <-subHandoff:
				default:
				}
			}
			if out != nil {
				throttleToRoom(out)
			}
		}
	}()

	// Let some chunks land before subscribing, so replay is non-empty and
	// Subscribe genuinely races the remainder of the writer's run.
	time.Sleep(time.Millisecond)
	replay, out, cancel := h.Subscribe()
	defer cancel()
	subHandoff <- out

	var acc bytes.Buffer
	acc.Write(replay)
	deadline := time.After(5 * time.Second)
	for !bytes.Contains(acc.Bytes(), last) {
		select {
		case b, ok := <-out:
			if !ok {
				t.Fatalf("channel closed before the last chunk appeared; got %d bytes", acc.Len())
			}
			acc.Write(b)
		case <-deadline:
			t.Fatalf("did not see the last chunk within 5s; got %d bytes", acc.Len())
		}
	}
	<-writerDone

	got := strings.Fields(acc.String())
	if len(got) != n {
		t.Fatalf("got %d chunks, want %d — replay and live together must contain every chunk exactly once", len(got), n)
	}
	for i, s := range got {
		if s != strconv.Itoa(i) {
			t.Fatalf("chunk at position %d = %q, want %q — out of order, dropped, or duplicated", i, s, strconv.Itoa(i))
		}
	}
}

// TestSlowSubscriberDoesNotStallTheHub pins that publish never blocks on a
// subscriber who never reads: once that subscriber's buffer is full its
// subscription closes with an overflow signal — the PTY and every other
// subscriber keep moving regardless.
func TestSlowSubscriberDoesNotStallTheHub(t *testing.T) {
	h := newTestHub()

	// A subscriber that never drains.
	_, stuckOut, overflow, stuckCancel := h.SubscribeWithOverflow()
	defer stuckCancel()

	// A subscriber that drains concurrently — it must still get everything.
	_, out, cancel := h.Subscribe()
	defer cancel()

	const total = subscriberBuffer + 50

	recvDone := make(chan struct{})
	var received []byte
	go func() {
		defer close(recvDone)
		count := 0
		for count < total {
			select {
			case b, ok := <-out:
				if !ok {
					return
				}
				received = append(received, b...)
				count++
			case <-time.After(5 * time.Second):
				return
			}
		}
	}()

	publishDone := make(chan struct{})
	go func() {
		defer close(publishDone)
		for i := 0; i < total; i++ {
			h.publish([]byte(strconv.Itoa(i) + "\n"))
			throttleToRoom(out)
		}
	}()

	select {
	case <-publishDone:
	case <-time.After(5 * time.Second):
		t.Fatal("publish blocked — a stuck subscriber stalled the hub")
	}
	<-recvDone

	if got := len(stuckOut); got != subscriberBuffer {
		t.Fatalf("stuck subscriber channel holds %d chunks, want exactly %d (full, not blocked past it)", got, subscriberBuffer)
	}
	select {
	case <-overflow:
	default:
		t.Fatal("slow subscriber was not explicitly told output was lost")
	}
	for range stuckOut {
	}
	// A closed stream never resumes with bytes after the gap.
	h.publish([]byte("after overflow\n"))
	if _, open := <-stuckOut; open {
		t.Fatal("overflowed stream resumed")
	}
	stuckCancel() // cancellation after overflow is idempotent

	got := strings.Fields(string(received))
	if len(got) != total {
		t.Fatalf("draining subscriber received %d/%d chunks", len(got), total)
	}
	for i, s := range got {
		if s != strconv.Itoa(i) {
			t.Fatalf("chunk at position %d = %q, want %q", i, s, strconv.Itoa(i))
		}
	}

	// The hub is still live after the overflow.
	replay, _, cancel3 := h.Subscribe()
	defer cancel3()
	if len(replay) == 0 {
		t.Fatal("hub appears dead after overflow: fresh Subscribe got an empty replay")
	}
}
