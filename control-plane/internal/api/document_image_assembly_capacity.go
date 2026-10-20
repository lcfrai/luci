package api

import (
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"math"
)

// Called by every shared admission while holding its existing host ledger lock.
// Active, expired and uncertain assembly leases retain all unobserved demand.
func addDocumentImageAssemblyCapacity(snapshot resourcebudget.Snapshot, values []documentImageAssemblyLease, skipScope, skipID string, demand *executionCapacityDemand) error {
	if len(values) > 4096 || demand == nil {
		return errors.New("assembly capacity bound")
	}
	for _, lease := range values {
		if !validDocumentImageAssemblyLease(lease) {
			return errors.New("assembly capacity ownership invalid")
		}
		if lease.State == "finished" || lease.Scope == skipScope && lease.ID == skipID {
			continue
		}
		observed := false
		for _, row := range snapshot.Execution.Containers {
			if lease.Ref != "" && lease.Identity.EngineID == snapshot.Execution.EngineID && row.ID == lease.Ref && row.Owned && row.Kind == "document-image-assembly" && row.MemoryLimitBytes != nil && *row.MemoryLimitBytes == 2<<30 && row.CPULimitMillis != nil && *row.CPULimitMillis == 1000 {
				observed = true
				break
			}
		}
		if !observed {
			memory := uint64(lease.Resources.MemoryMiB) << 20
			if demand.PendingMemory > ^uint64(0)-memory || demand.AllMemory > ^uint64(0)-memory || demand.PendingDiskBytes > ^uint64(0)-lease.CreateDiskBytes || demand.CPU > math.MaxInt64-int64(lease.Resources.CPUMillis) {
				return errors.New("assembly demand overflow")
			}
			demand.PendingMemory += memory
			demand.AllMemory += memory
			demand.PendingDiskBytes += lease.CreateDiskBytes
			demand.CPU += int64(lease.Resources.CPUMillis)
		}
		// Capture remains a host-disk reservation even when container limits were
		// observed. No reader/lease timeout can make that partial output disappear.
		if demand.PendingDiskBytes > ^uint64(0)-lease.CaptureDiskBytes || demand.PendingHostLaunchBytes > ^uint64(0)-lease.HostLaunchBytes {
			return errors.New("assembly capture reserve overflow")
		}
		demand.PendingDiskBytes += lease.CaptureDiskBytes
		if lease.Stage!=nil&&lease.State!="staging"{if demand.PendingDiskBytes>^uint64(0)-lease.Stage.DirectoryBytes{return errors.New("assembly input staging demand overflow")};demand.PendingDiskBytes+=lease.Stage.DirectoryBytes}
		demand.PendingHostLaunchBytes += lease.HostLaunchBytes
	}
	return nil
}
func addDocumentImageAssemblyReservationView(view *executionReservationView, values []documentImageAssemblyLease) {
	for _, value := range values {
		if value.State == "finished" {
			continue
		}
		view.Count++
		view.MemoryMiB += value.Resources.MemoryMiB
		view.CPUMillis += value.Resources.CPUMillis
	}
}
