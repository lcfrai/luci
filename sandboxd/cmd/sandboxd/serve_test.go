package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// F3. sandboxd outlived the process it ran: main served until something killed
// it, so a shell that ran `exit` left a container Up, a session `running`, and
// every attach getting an Exit frame and a close that the cockpit answered by
// reconnecting — 634 relay-ends in eleven minutes
// (docs/verification/2026-09-27-phase-2-results.md, finding F3).
//
// The truth lives in the process. These three tests pin the daemon's half of
// that: when the process ends, sandboxd stops serving and returns, so the
// container stops and Docker — the one thing the control plane already reads —
// reports it stopped.

// blockForever is a serve function that never returns on its own, which is what
// http.Server.ListenAndServe does until something closes the listener.
func blockForever(stop <-chan struct{}) func() error {
	return func() error {
		<-stop
		return nil
	}
}

func TestServeSessionReturnsWhenTheProcessExits(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	processDone := make(chan struct{})
	var shutdowns atomic.Int32

	done := make(chan error, 1)
	go func() {
		done <- serveSession(blockForever(stop), func(context.Context) error {
			shutdowns.Add(1)
			return nil
		}, processDone, time.Millisecond)
	}()

	close(processDone)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveSession: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveSession did not return within 5s of the process exiting; sandboxd is outliving its process (F3)")
	}
	if got := shutdowns.Load(); got != 1 {
		t.Errorf("shutdown called %d times, want 1: a listener left open keeps taking attaches a dead session cannot serve", got)
	}
}

// The other half of the same property, and the one a careless fix breaks: a
// session must outlive the CLIENT that is looking at it. Nothing but the
// process ending may end the daemon.
func TestServeSessionKeepsServingWhileTheProcessRuns(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	var shutdowns atomic.Int32

	done := make(chan error, 1)
	go func() {
		done <- serveSession(blockForever(stop), func(context.Context) error {
			shutdowns.Add(1)
			return nil
		}, make(chan struct{}), time.Millisecond)
	}()

	select {
	case err := <-done:
		t.Fatalf("serveSession returned (%v) while the process was still running", err)
	case <-time.After(200 * time.Millisecond):
	}
	if got := shutdowns.Load(); got != 0 {
		t.Errorf("shutdown called %d times while the process was still running, want 0", got)
	}
}

// A listener that fails is still the other way out, and its error has to reach
// main so the exit code says so.
func TestServeSessionReturnsTheListenerError(t *testing.T) {
	t.Parallel()
	want := errors.New("listen tcp :8080: bind: address already in use")

	err := serveSession(func() error { return want }, func(context.Context) error {
		t.Error("shutdown was called for a listener that never started")
		return nil
	}, make(chan struct{}), time.Millisecond)

	if !errors.Is(err, want) {
		t.Fatalf("serveSession = %v, want %v", err, want)
	}
}

// The linger is the only reason the Exit frame ever reaches an attached
// cockpit: fanOut writes it when the hub closes (internal/hub/server.go), and
// the daemon returning is the container stopping. Deleting the wait keeps every
// test above green — all three inject their own linger and none asserts that any
// time passes — so it is pinned here, and it is pinned as "time passed", not as
// "a timer was constructed".
//
// What fails without it: linger 0, the process ends, PID 1 returns before the
// frame leaves the socket, and a pane that is already attached never learns why
// its session ended. That pane has no other channel for ExitReason.
func TestServeSessionWaitsForTheExitFrameToLeave(t *testing.T) {
	t.Parallel()
	const linger = 300 * time.Millisecond
	stop := make(chan struct{})
	defer close(stop)
	processDone := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- serveSession(blockForever(stop), func(context.Context) error { return nil },
			processDone, linger)
	}()

	start := time.Now()
	close(processDone)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveSession: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveSession did not return within 5s of the process exiting")
	}
	// The clock starts before the goroutine can have noticed, so this can only
	// be early if the wait is gone.
	if elapsed := time.Since(start); elapsed < linger {
		t.Errorf("serveSession returned %v after the process ended, inside the %v linger: "+
			"PID 1 leaves before fanOut's Exit frame can reach an attached cockpit", elapsed, linger)
	}
}

// The linger is ONE window, not one per step. Shutdown blocks only while a
// non-hijacked request (/healthz) is in flight, and when it does it must spend
// the same linger the wait spends, not a first copy of it.
//
// What fails without it: exitLinger says two seconds and the comment above it
// reasons about two seconds, but a health check landing at the wrong instant
// makes it four — a container that takes twice as long to stop as the number
// anything else is sized against.
func TestTheLingerBoundsTheWholeWaitNotEachHalfOfIt(t *testing.T) {
	t.Parallel()
	const linger = 500 * time.Millisecond
	stop := make(chan struct{})
	defer close(stop)
	processDone := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		// What http.Server.Shutdown does with a request it is waiting for: it
		// returns when the context it was given runs out, not before.
		done <- serveSession(blockForever(stop), func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}, processDone, linger)
	}()

	start := time.Now()
	close(processDone)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveSession: %v; a shutdown that ran out of time is a warning, not a failure", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serveSession did not return within 10s of the process exiting")
	}
	if elapsed := time.Since(start); elapsed >= 2*linger {
		t.Errorf("the post-exit wait took %v, at or past 2 x the %v linger: "+
			"shutdown's budget and the linger are two windows, not one", elapsed, linger)
	}
}
