package api

import (
	"context"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestExpiredAssemblyAlwaysRetainsCaptureAndHostLaunchReservations(t *testing.T) {
	req, h, executor, _ := assemblyRuntimeFixture(t)
	prepared, err := executor.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := h.Admit(context.Background(), prepared, req)
	if err != nil {
		t.Fatal(err)
	}
	lease.CreatedAt = lease.CreatedAt.Add(-time.Hour)
	lease.UpdatedAt = lease.CreatedAt
	lease.ComputeDeadline = lease.ComputeDeadline.Add(-time.Hour)
	lease.WholeDeadline = lease.WholeDeadline.Add(-time.Hour)
	lease.State = "cleaning"
	lease.Ref = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	memory, cpu := uint64(2<<30), int64(1000)
	snapshot := resourcebudget.Snapshot{Execution: resourcebudget.ExecutionSnapshot{EngineID: lease.Identity.EngineID, Containers: []resourcebudget.ContainerUsage{{ID: lease.Ref, Owned: true, Kind: "document-image-assembly", MemoryLimitBytes: &memory, CPULimitMillis: &cpu}}}}
	var demand executionCapacityDemand
	if err = addDocumentImageAssemblyCapacity(snapshot, []documentImageAssemblyLease{lease}, "", "", &demand); err != nil {
		t.Fatal(err)
	}
	if demand.PendingMemory != 0 || demand.PendingDiskBytes != 2<<30 || demand.PendingHostLaunchBytes != 2<<30 {
		t.Fatal("expired observed assembly lost capture/host reservation", demand)
	}
	for _, change := range []func(*resourcebudget.ContainerUsage){func(r *resourcebudget.ContainerUsage) { r.Owned = false }, func(r *resourcebudget.ContainerUsage) { r.Kind = "document" }, func(r *resourcebudget.ContainerUsage) { r.MemoryLimitBytes = nil }} {
		changed := snapshot
		changed.Execution.Containers = append([]resourcebudget.ContainerUsage{}, snapshot.Execution.Containers...)
		change(&changed.Execution.Containers[0])
		demand = executionCapacityDemand{}
		if addDocumentImageAssemblyCapacity(changed, []documentImageAssemblyLease{lease}, "", "", &demand) != nil || demand.PendingMemory != 2<<30 || demand.PendingDiskBytes != lease.CreateDiskBytes+(2<<30) {
			t.Fatal("foreign/unknown observation erased reservation", demand)
		}
	}
}
