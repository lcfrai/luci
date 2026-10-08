package api

import (
	"context"
	"testing"
)

func TestSwarmPendingLaunchPreservesPreBackendAttemptUntilAtomicRetirement(t *testing.T) {
	f, leaseID, _ := missingChildFixture(t)
	a, op := f.env.api, f.ops[0]
	pending, err := a.projectSwarmPendingLaunch(context.Background(), f.owner.ID, f.run.ID, op.ID, op.ChildID)
	if err != nil || !pending {
		t.Fatal("bound pre-backend claim could lose its recovery handle", pending, err)
	}
	// The read-only replacement check must leave the reservation intact. A
	// controller paused before its launch marker still owns this exact child.
	ledger, err := a.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].ID != leaseID || ledger.Leases[0].State != "allocating" {
		t.Fatal("pending check mutated the unacknowledged launch", ledger, err)
	}
	missingChildRecover(t, f, 200)
	pending, err = a.projectSwarmPendingLaunch(context.Background(), f.owner.ID, f.run.ID, op.ID, op.ChildID)
	if err != nil || pending {
		t.Fatal("explicit durable retirement did not unblock a fresh attempt", pending, err)
	}
	if err := a.execution.markBackendStarting(context.Background(), leaseID, op.ChildID, op.ChildID); err == nil {
		t.Fatal("retired pre-backend allocator could still dispatch")
	}
}
