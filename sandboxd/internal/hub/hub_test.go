//go:build linux || darwin

package hub_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/hub"
)

// collect drains out until want appears or the deadline passes, seeding the
// accumulator with replay.
func collect(t *testing.T, replay []byte, out <-chan []byte, want []byte, d time.Duration) string {
	t.Helper()
	var acc bytes.Buffer
	acc.Write(replay)
	if bytes.Contains(acc.Bytes(), want) {
		return acc.String()
	}
	deadline := time.After(d)
	for {
		select {
		case b, ok := <-out:
			if !ok {
				t.Fatalf("channel closed before %q appeared; got %q", want, acc.String())
			}
			acc.Write(b)
			if bytes.Contains(acc.Bytes(), want) {
				return acc.String()
			}
		case <-deadline:
			t.Fatalf("did not see %q within %s; got %q", want, d, acc.String())
		}
	}
}

func TestHubStreamsOutputToASubscriber(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	replay, out, cancel := h.Subscribe()
	defer cancel()

	if _, err := h.Write([]byte("echo panacea-ok\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("panacea-ok"), 8*time.Second)
}

func TestReAttachReplaysEarlierOutput(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	replay, out, cancel := h.Subscribe()
	if _, err := h.Write([]byte("echo first-output\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("first-output"), 8*time.Second)

	// Detach. The process must keep running.
	cancel()

	replay2, out2, cancel2 := h.Subscribe()
	defer cancel2()
	if !bytes.Contains(replay2, []byte("first-output")) {
		t.Fatalf("replay after re-attach = %q, want it to contain %q", replay2, "first-output")
	}

	// And the same PTY still accepts input.
	if _, err := h.Write([]byte("echo second-output\n")); err != nil {
		t.Fatalf("Write after re-attach: %v", err)
	}
	collect(t, replay2, out2, []byte("second-output"), 8*time.Second)
}

func TestTwoSubscribersBothReceiveLiveOutput(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	r1, o1, c1 := h.Subscribe()
	defer c1()
	r2, o2, c2 := h.Subscribe()
	defer c2()

	if _, err := h.Write([]byte("echo both-see-this\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, r1, o1, []byte("both-see-this"), 8*time.Second)
	collect(t, r2, o2, []byte("both-see-this"), 8*time.Second)
}

func TestResizeReachesTheProcess(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	replay, out, cancel := h.Subscribe()
	defer cancel()

	if err := h.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if _, err := h.Write([]byte("stty size\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("40 120"), 8*time.Second)
}

func TestDoneClosesAndExitReasonIsSetWhenTheProcessExits(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh", Args: []string{"-c", "exit 3"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	select {
	case <-h.Done():
	case <-time.After(8 * time.Second):
		t.Fatal("Done() was not closed within 8s of the process exiting")
	}
	if got := h.ExitReason(); got == "" {
		t.Fatal("ExitReason() is empty after the process exited")
	}
}

func TestSubscriberChannelClosesWhenTheProcessExits(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh", Args: []string{"-c", "echo bye; exit 0"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	_, out, cancel := h.Subscribe()
	defer cancel()

	deadline := time.After(8 * time.Second)
	for {
		select {
		case _, ok := <-out:
			if !ok {
				return // closed, as required
			}
		case <-deadline:
			t.Fatal("subscriber channel was not closed within 8s of process exit")
		}
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	_, _, cancel := h.Subscribe()
	cancel()
	cancel() // must not panic or close a closed channel
}

func TestSlowSubscriberDoesNotStallTheHub(t *testing.T) {
	h, err := hub.Start(hub.Config{Command: "/bin/sh"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()

	// A subscriber that never reads.
	_, _, stuck := h.Subscribe()
	defer stuck()

	// A subscriber that does read must still get output.
	replay, out, cancel := h.Subscribe()
	defer cancel()

	if _, err := h.Write([]byte("for i in 1 2 3 4 5; do echo line-$i; done; echo tail-marker\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	collect(t, replay, out, []byte("tail-marker"), 10*time.Second)
}
