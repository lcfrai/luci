package api

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

const workerStorageRefreshBudget = 4 * time.Second
const workerStorageRefreshInterval = 100 * time.Millisecond
const workerStorageRefreshTargetAge = 2 * time.Second

// At most one request in this API process waits for background publication.
// Other requests retain ordinary fail-closed admission, rather than queueing
// timers or spawning another observer. No Docker or WSL probe runs here.
var workerStorageRefreshActive atomic.Bool

// This is a readiness wait, never capacity admission. Call only before branch
// and child-lifecycle locks. Every caller still runs the unchanged atomic
// reservation, including its final authority/resource checks before allocation.
// A missing shared observation cannot distinguish stale data from changed pins;
// leave that case to ordinary refusal. Only known non-null disk lag is waitable.
func (a *API) awaitWorkerStorageRefresh(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.deps.ResourceSnapshot == nil || a.deps.Product == nil || a.deps.Backend == nil {
		return nil
	}
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		return nil // Existing admission reports unavailable policy.
	}
	resources := a.sessionResources(ctx)
	kind := a.deps.Backend.Kind()
	initial := a.deps.ResourceSnapshot()
	now := time.Now()
	disk := initial.Execution.DaemonDisk
	if !workerStorageRefreshable(initial, policy, resources, kind, now) || now.Sub(disk.ObservedAt) <= workerStorageRefreshTargetAge {
		return nil
	}
	if !workerStorageRefreshActive.CompareAndSwap(false, true) {
		return nil // Occupied wait slot is not evidence of available capacity.
	}
	defer workerStorageRefreshActive.Store(false)
	deadline := time.Now().Add(workerStorageRefreshBudget)
	timer := time.NewTimer(workerStorageRefreshBudget)
	defer timer.Stop()
	ticker := time.NewTicker(workerStorageRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil // Original admission reports the still-unavailable sample.
		case <-ticker.C:
			current := a.deps.ResourceSnapshot()
			now = time.Now()
			if err := ctx.Err(); err != nil {
				return err
			}
			if !now.Before(deadline) {
				return nil
			}
			if current.Execution.EngineID != initial.Execution.EngineID ||
				!workerStorageRefreshable(current, policy, resources, kind, now) {
				return nil // Pressure, identity, confirmation or unknown data wins.
			}
			// Require a real newer sample, with three seconds of the existing
			// five-second admission window left for normal request preparation.
			// Never restamp cached capacity or extend its admission lifetime.
			// Slow preparation can still exhaust it and must fail final admission.
			if current.Execution.DaemonDisk.ObservedAt.After(disk.ObservedAt) &&
				now.Sub(current.Execution.DaemonDisk.ObservedAt) <= workerStorageRefreshTargetAge {
				return nil
			}
		}
	}
}

func workerStorageRefreshable(snap resourcebudget.Snapshot, policy product.SwarmSchedulerPolicy, resources backend.Resources, kind backend.Kind, now time.Time) bool {
	disk := snap.Execution.DaemonDisk
	if kind != backend.KindDocker || snap.Host.Platform != "windows" || snap.Execution.EngineID == "" ||
		disk == nil || disk.AvailableBytes == nil || disk.ObservedAt.IsZero() || disk.ObservedAt.After(now) ||
		snap.Execution.HostStorage == nil || !snap.Execution.HostStorage.Required || snap.Execution.HostStorage.State != "confirmed" {
		return false
	}
	estimate, err := estimateExecutionCreateDisk(snap.Execution)
	if err != nil {
		return false
	}
	diskDemand, bounded := executionPendingDiskDemand(estimate.Bytes, 0, policy.DiskReserveMiB)
	if !bounded || *disk.AvailableBytes < diskDemand || resourcebudget.ValidateHostStorage(snap.Execution.HostStorage, true, diskDemand, now, 5*time.Second) != nil {
		return false
	}
	demand := executionCapacityDemand{HostLaunchBytes: windowsDockerHostLaunchBytes(snap, kind)}
	for _, container := range snap.Execution.Containers {
		memory, cpu, known := executionWorkloadDemand(container, now)
		if !known || memory > ^uint64(0)-demand.AllMemory || cpu < 0 || demand.CPU > int64(^uint64(0)>>1)-cpu {
			return false
		}
		demand.AllMemory += memory
		demand.CPU += cpu
	}
	// Classify actual metrics without altering any timestamp or disk value.
	// The full ledger, pending demand and all limits remain atomic admission's
	// responsibility; a readiness wait never creates or approves a lease.
	code, _ := evaluateExecutionCapacity(snap, policy, resources, demand, now)
	return code == "execution_disk_unavailable" || code == ""
}
