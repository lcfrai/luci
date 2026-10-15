package api

import (
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// Shared worker/check/preview/image admission calls this under the same host
// ledger lock. Unknown or absent references retain all demand until retirement.
func addDocumentCapacity(snapshot resourcebudget.Snapshot, documents []documentExtractionLease, skipScope, skipID string, demand *executionCapacityDemand) error {
	if len(documents) > 4096 {
		return errors.New("document capacity history exceeds its bound")
	}
	for _, lease := range documents {
		if !validDocumentExtractionLease(lease) {
			return errors.New("document capacity ownership is invalid")
		}
		if lease.State == "finished" || lease.Scope == skipScope && lease.ID == skipID {
			continue
		}
		observed := false
		for _, row := range snapshot.Execution.Containers {
			if lease.Ref != "" && lease.Identity.EngineID == snapshot.Execution.EngineID && row.ID == lease.Ref && row.Kind == "document-extraction" {
				observed = true
				break
			}
		}
		if !observed {
			memory := uint64(lease.Resources.MemoryMiB) << 20
			if demand.PendingMemory > ^uint64(0)-memory || demand.AllMemory > ^uint64(0)-memory || demand.PendingDiskBytes > ^uint64(0)-lease.CreateDiskBytes {
				return errors.New("document capacity accounting overflow")
			}
			demand.PendingMemory += memory
			demand.AllMemory += memory
			demand.CPU += int64(lease.Resources.CPUMillis)
			demand.PendingDiskBytes += lease.CreateDiskBytes
		}
		allowance := lease.HostLaunchBytes
		if allowance == 0 && snapshot.Host.Platform == "windows" {
			allowance = 2048 << 20
		}
		if demand.PendingHostLaunchBytes > ^uint64(0)-allowance {
			return errors.New("document host reserve accounting overflow")
		}
		demand.PendingHostLaunchBytes += allowance
	}
	return nil
}
func addDocumentReservationView(view *executionReservationView, documents []documentExtractionLease) {
	for _, lease := range documents {
		if lease.State == "finished" {
			continue
		}
		view.Count++
		view.MemoryMiB += lease.Resources.MemoryMiB
		view.CPUMillis += lease.Resources.CPUMillis
	}
}
