package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// The production lock helper evaluates Done after a failed nonblocking lock
// acquisition. This wrapper only reports that boundary; cancellation belongs
// to the test caller and happens after both real readers have reached it.
type reservationLockWaitContext struct {
	context.Context
	attached chan struct{}
	once     sync.Once
}

func (c *reservationLockWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.attached) })
	return c.Context.Done()
}

func TestExecutionReservationLedgerCancelledWaitPreservesLivePeer(t *testing.T) {
	g := reservationTestGate(t)
	original, err := g.readFenceLedger()
	if err != nil || len(original.Leases) != 1 || original.Leases[0].Resources.MemoryMiB != 1024 || original.Leases[0].Resources.CPUMillis != 1000 {
		t.Fatal("fixture did not retain the exact nonzero charge", original, err)
	}
	backendFixture := g.api.deps.Backend.(*sessionsBackend)
	be := &schedulerDisplayBackend{Backend: backendFixture}
	g.api.deps.Backend = be
	var snapshotCalls atomic.Int32
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		snapshotCalls.Add(1)
		return executionFixtureSnapshot()
	}
	before := reservationTree(t, filepath.Dir(g.path))
	file, err := os.Open(filepath.Join(filepath.Dir(g.path), "leases.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	unlock, err := lockExecutionLedger(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	var unlockOnce sync.Once
	release := func() { unlockOnce.Do(unlock) }
	defer release()

	type result struct {
		ledger executionLeaseLedger
		err    error
	}
	type reader struct {
		ctx    *reservationLockWaitContext
		cancel context.CancelFunc
		result chan result
		done   chan struct{}
	}
	startReader := func() reader {
		base, cancel := context.WithCancel(context.Background())
		r := reader{ctx: &reservationLockWaitContext{Context: base, attached: make(chan struct{})}, cancel: cancel, result: make(chan result, 1), done: make(chan struct{})}
		go func() {
			defer close(r.done)
			// Directly exercise the real reader with the caller context. Unlike
			// reservationObservation, this helper adds no 100 ms child timeout.
			ledger, err := g.readReservationLedger(r.ctx)
			r.result <- result{ledger, err}
		}()
		return r
	}
	cancelled, peer := startReader(), startReader()
	defer func() {
		cancelled.cancel()
		peer.cancel()
		release()
		for _, r := range []reader{cancelled, peer} {
			select {
			case <-r.done:
			case <-time.After(2 * time.Second):
				t.Error("reservation reader did not settle during fixture cleanup")
			}
		}
	}()
	for _, r := range []reader{cancelled, peer} {
		select {
		case <-r.ctx.attached:
		case <-r.done:
			t.Fatal("reader exited before reaching the held production lock")
		case <-time.After(2 * time.Second):
			t.Fatal("reader did not attach to the held production lock wait")
		}
		if r.ctx.Err() != nil {
			t.Fatal("reader was cancelled before the external cancellation boundary")
		}
		if _, hasDeadline := r.ctx.Deadline(); hasDeadline {
			t.Fatal("fixture imposed an observation timeout")
		}
	}
	cancelled.cancel()
	select {
	case got := <-cancelled.result:
		if !errors.Is(got.err, context.Canceled) || !reflect.DeepEqual(got.ledger, executionLeaseLedger{}) {
			t.Fatal("cancelled waiting reader returned demand or a different error", got.ledger, got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("external cancellation did not settle the attached lock waiter")
	}
	select {
	case got := <-peer.result:
		t.Fatal("uncancelled peer escaped the still-held lock", got.ledger, got.err)
	default:
	}
	if peer.ctx.Err() != nil || before != reservationTree(t, filepath.Dir(g.path)) {
		t.Fatal("cancelling one waiter affected its peer or retained ledger bytes/modes/timestamps")
	}
	release()
	select {
	case got := <-peer.result:
		if got.err != nil || !reflect.DeepEqual(got.ledger, original) {
			t.Fatal("live peer did not read the exact original charge after unlock", got.ledger, got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("live peer did not proceed after the exact lock was released")
	}
	if before != reservationTree(t, filepath.Dir(g.path)) || snapshotCalls.Load() != 0 || be.reads.Load() != 0 || controlledAllocationCount(backendFixture) != 0 || backendFixture.destroyCount() != 0 {
		t.Fatal("readers changed retained demand, sampled capacity or accessed execution backend")
	}
	t.Log("two real readers reached the held lock wait; external cancellation settled one while the peer remained waiting; unlock admitted the peer with the exact retained 1024 MiB/1000 mCPU charge")
}
