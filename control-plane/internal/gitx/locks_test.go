package gitx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
)

func TestLocksSerialiseOneProject(t *testing.T) {
	var l gitx.Locks
	ctx := context.Background()
	unlockA, err := l.Lock(ctx, "p-1")
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan func(), 1)
	go func() {
		u, err := l.Lock(ctx, "p-1")
		if err != nil {
			t.Error(err)
		}
		got <- u
	}()
	select {
	case <-got:
		t.Fatal("B took p-1 while A held it")
	case <-time.After(50 * time.Millisecond):
	}
	unlockA()
	var unlockB func()
	select {
	case unlockB = <-got:
	case <-time.After(time.Second):
		t.Fatal("B did not get p-1 within 1s of A's unlock")
	}
	unlockA() // idempotent: must not free B's hold
	if u, ok := l.TryLock("p-1"); ok {
		u()
		t.Fatal("A's second unlock released B's hold")
	}
	unlockB()
}

func TestLocksLetOtherProjectsRun(t *testing.T) {
	var l gitx.Locks
	unlockA, err := l.Lock(context.Background(), "p-1")
	if err != nil {
		t.Fatal(err)
	}
	defer unlockA()
	done := make(chan error, 1)
	go func() {
		u, err := l.Lock(context.Background(), "p-2")
		if err == nil {
			u()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("p-2 waited behind p-1")
	}
}

func TestLocksLockHonoursTheContext(t *testing.T) {
	var l gitx.Locks
	unlockA, err := l.Lock(context.Background(), "p-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	time.AfterFunc(20*time.Millisecond, cancel)
	if _, err := l.Lock(ctx, "p-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("B's Lock = %v, want context.Canceled", err)
	}
	unlockA()
	u, ok := l.TryLock("p-1")
	if !ok {
		t.Fatal("the cancelled waiter swallowed A's release: C cannot take p-1")
	}
	u()
}

func TestLocksTryLockNeverWaits(t *testing.T) {
	var l gitx.Locks
	for i := 0; i < 1000; i++ {
		u, ok := l.TryLock("p-1")
		if !ok {
			t.Fatalf("try %d: TryLock on a free p-1 failed", i)
		}
		u()
	}
	unlockA, err := l.Lock(context.Background(), "p-1")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	u, ok := l.TryLock("p-1")
	if ok || u != nil {
		t.Fatal("TryLock took a held p-1")
	}
	if d := time.Since(start); d > 100*time.Millisecond { // the part says 1 ms; a loaded machine pauses longer, and a waiting TryLock never returns
		t.Fatalf("TryLock on a held lock took %v; it must not wait", d)
	}
	u2, ok := l.TryLock("p-2")
	if !ok {
		t.Fatal("TryLock p-2 failed while only p-1 was held")
	}
	u2()
	unlockA()
	u3, ok := l.TryLock("p-1")
	if !ok {
		t.Fatal("TryLock p-1 failed after A's unlock")
	}
	u3()
}
