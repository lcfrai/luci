//go:build linux || darwin

package hub

import (
	"bytes"
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

// TestSubscribeRacingThePumpNeverDuplicatesOrDropsAChunk pins the invariant
// Subscribe's doc comment promises: a chunk lands in exactly one place
// relative to a concurrent Subscribe — either in the replay or live, never
// both, never neither.
func TestSubscribeRacingThePumpNeverDuplicatesOrDropsAChunk(t *testing.T) {
	h := newTestHub()
	const n = 2000
	last := []byte(strconv.Itoa(n-1) + "\n")

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for i := 0; i < n; i++ {
			h.publish([]byte(strconv.Itoa(i) + "\n"))

		}
	}()

	// Let some chunks land before subscribing, so replay is non-empty and
	// Subscribe genuinely races the remainder of the writer's run.
	time.Sleep(time.Millisecond)
	replay, out, cancel := h.Subscribe()
	defer cancel()

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
	_, stuckOut, overflow, stuckCancel := h.SubscribeWithOverflow()
	defer stuckCancel()
	_, out, fastOverflow, cancel := h.SubscribeWithOverflow()
	defer cancel()

	// Exceed the byte budget while a second subscriber acknowledges each
	// complete page. Scheduling cannot accidentally turn that reader into the
	// deliberately stuck subscriber, and the publisher must never wait for it.
	const total = subscriberByteBudget/subscriberPageBytes + 50
	payload := bytes.Repeat([]byte("x"), subscriberPageBytes)
	expected := bytes.Repeat(payload, total)
	progress := make(chan struct{}, 1)
	recvDone := make(chan []byte, 1)
	go func() {
		var received []byte
		for b := range out {
			before := len(received) / subscriberPageBytes
			received = append(received, b...)
			if len(received)/subscriberPageBytes > before {
				progress <- struct{}{}
			}
		}
		recvDone <- received
	}()

	publishDone := make(chan struct{})
	go func() {
		defer close(publishDone)
		for i := 0; i < total; i++ {
			h.publish(payload)
			<-progress
		}
	}()
	select {
	case <-publishDone:
	case <-time.After(5 * time.Second):
		t.Fatal("publish or the healthy subscriber stalled behind a stuck subscriber")
	}
	select {
	case <-overflow:
	default:
		t.Fatal("the subscriber exceeding its byte budget was not told output was lost")
	}
	select {
	case <-fastOverflow:
		t.Fatal("the draining subscriber overflowed")
	default:
	}
	for range stuckOut {
	}
	h.publish(nil)
	if _, open := <-stuckOut; open {
		t.Fatal("overflowed stream resumed")
	}
	stuckCancel()

	// Overflow retires only its view; the hub remains available for reconnect.
	replay, _, cancel3 := h.Subscribe()
	cancel3()
	if !bytes.Equal(replay, expected[len(expected)-ring.DefaultCapacity:]) {
		t.Fatal("fresh replay did not retain the exact recent bytes after overflow")
	}
	h.finish("exit status 0")
	select {
	case received := <-recvDone:
		if !bytes.Equal(received, expected) {
			t.Fatalf("healthy subscriber received %d bytes, expected exact %d-byte stream", len(received), len(expected))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("healthy subscriber did not drain and close after exit")
	}
}
