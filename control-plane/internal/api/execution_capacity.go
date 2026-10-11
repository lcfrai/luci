package api

import (
	"errors"
	"runtime"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// executionCapacityDemand is computed while the shared ledger lock is held.
// It includes every account's pending workers and checks; observed containers
// already contribute their hard ceiling or fresh measured workload demand.
// PendingDiskBytes carries individually pinned check image-copy demand.
type executionCapacityDemand struct {
	PendingMemory, AllMemory uint64
	CPU                      int64
	PendingAllocations       int
	PendingDiskBytes         uint64
	RequiredCreateDiskBytes  uint64
	CreateAcknowledged       bool
}

func freshExecutionCapacity(snap resourcebudget.Snapshot, now time.Time) bool {
	return snap.Host.Fresh(now, 5*time.Second) && snap.Execution.Fresh(now, 5*time.Second) &&
		snap.Host.Memory.EffectiveAvailableBytes != nil && snap.Host.Memory.EffectiveTotalBytes != nil &&
		snap.Execution.TotalMemoryBytes != nil && snap.Execution.CPUMillis != nil && *snap.Execution.CPUMillis > 0 &&
		snap.Host.CPU.EffectiveMillis != nil && *snap.Host.CPU.EffectiveMillis > 0 && snap.Execution.InventoryComplete
}

// A blank reason admits the requested resources. Callers retain their own
// worker/provider or per-user check policy limits, but cannot bypass any host,
// guest, CPU, disk, freshness, or Windows backing-volume prerequisite here.
func evaluateExecutionCapacity(snap resourcebudget.Snapshot, policy product.SwarmSchedulerPolicy, resources backend.Resources, demand executionCapacityDemand, now time.Time) (string, string) {
	result := evaluateExecutionCapacityWithEvidence(snap, policy, resources, demand, now)
	return result.Code, result.Message
}

type executionCapacityResult struct {
	Code, Message string
	Evidence      registry.StartAdmissionEvidence
}

func evaluateExecutionCapacityWithEvidence(snap resourcebudget.Snapshot, policy product.SwarmSchedulerPolicy, resources backend.Resources, demand executionCapacityDemand, now time.Time) executionCapacityResult {
	refused := func(code, message string) executionCapacityResult {
		return executionCapacityResult{Code: code, Message: message}
	}
	if !freshExecutionCapacity(snap, now) {
		return refused("resource_observation_unavailable", "Current machine capacity is not available.")
	}
	required := uint64(resources.MemoryMiB) << 20
	headroom := max(uint64(policy.MinHeadroomMiB)<<20, *snap.Host.Memory.EffectiveTotalBytes*uint64(policy.HeadroomPercent)/100)
	executionHeadroom := max(uint64(policy.MinHeadroomMiB)<<20, *snap.Execution.TotalMemoryBytes*uint64(policy.HeadroomPercent)/100)
	if *snap.Host.Memory.EffectiveAvailableBytes < headroom+required+demand.PendingMemory || *snap.Execution.TotalMemoryBytes < demand.AllMemory+required+executionHeadroom {
		result := refused("memory_pressure", "Available memory cannot preserve the configured headroom for another worker.")
		// Preserve the existing host-first predicate, using its exact operands.
		result.Evidence = registry.StartAdmissionEvidence{Metric: "host_available_memory", ObservedAt: snap.Host.ObservedAt.UTC(), DecidedAt: now.UTC(), ObservedBytes: *snap.Host.Memory.EffectiveAvailableBytes, RequiredBytes: headroom + required + demand.PendingMemory}
		if *snap.Host.Memory.EffectiveAvailableBytes >= headroom+required+demand.PendingMemory {
			result.Evidence = registry.StartAdmissionEvidence{Metric: "execution_total_memory", ObservedAt: snap.Execution.ObservedAt.UTC(), DecidedAt: now.UTC(), ObservedBytes: *snap.Execution.TotalMemoryBytes, RequiredBytes: demand.AllMemory + required + executionHeadroom}
		}
		return result
	}
	cpuCeiling := *snap.Execution.CPUMillis * int64(100-policy.HeadroomPercent) / 100
	if snap.Host.CPU.EffectiveMillis != nil {
		cpuCeiling = min(cpuCeiling, *snap.Host.CPU.EffectiveMillis*int64(100-policy.HeadroomPercent)/100)
	}
	if snap.Host.CPU.UtilizationPercent != nil && *snap.Host.CPU.UtilizationPercent > float64(100-policy.HeadroomPercent) {
		result := refused("cpu_pressure", "Current CPU pressure needs to fall before another worker starts.")
		result.Evidence = registry.StartAdmissionEvidence{Metric: "host_cpu_utilization_percent", ObservedAt: snap.Host.ObservedAt.UTC(), DecidedAt: now.UTC(), ObservedPercent: *snap.Host.CPU.UtilizationPercent, MaximumPercent: float64(100 - policy.HeadroomPercent)}
		return result
	}
	if demand.CPU+int64(resources.CPUMillis) > cpuCeiling {
		return refused("cpu_capacity", "Configured worker CPU limits exceed the available execution capacity.")
	}
	if len(snap.Host.Disks) == 0 {
		return refused("disk_observation_unavailable", "Disk capacity must be measured before another worker starts.")
	}
	for _, disk := range snap.Host.Disks {
		if disk.AvailableBytes == nil || disk.ObservedAt.IsZero() || disk.ObservedAt.After(now) || now.Sub(disk.ObservedAt) > 5*time.Second {
			return refused("disk_observation_unavailable", "Current host storage capacity is unavailable. Waiting for a fresh observation.")
		}
		if *disk.AvailableBytes < uint64(policy.DiskReserveMiB)<<20 {
			return refused("disk_pressure", "Available disk is below the configured working reserve.")
		}
	}
	typedDemandOnly := demand.RequiredCreateDiskBytes != 0 && demand.PendingAllocations == 0
	if (!typedDemandOnly && snap.Execution.ImageVirtualSizeBytes == nil) || snap.Execution.DaemonDisk == nil || snap.Execution.DaemonDisk.AvailableBytes == nil {
		return refused("execution_disk_unavailable", "Execution storage capacity and sandbox image demand must be observed before another worker starts.")
	}
	if snap.Execution.DaemonDisk.ObservedAt.IsZero() || snap.Execution.DaemonDisk.ObservedAt.After(now) || now.Sub(snap.Execution.DaemonDisk.ObservedAt) > 5*time.Second {
		return refused("execution_disk_unavailable", "Current execution storage capacity is not available. Waiting for a fresh observation.")
	}
	// An explicitly bounded typed build can prepare the first sandbox image.
	// It does not need that image to exist beforehand. Any pending worker still
	// needs the real configured-image estimate; typed demand cannot erase it.
	createDisk := executionCreateDiskEstimate{Bytes: demand.RequiredCreateDiskBytes}
	if !typedDemandOnly {
		var estimateErr error
		createDisk, estimateErr = estimateExecutionCreateDisk(snap.Execution)
		if estimateErr != nil {
			return refused("execution_disk_estimate_unavailable", estimateErr.Error())
		}
	}
	diskDemand, bounded := executionPendingDiskDemand(createDisk.Bytes, demand.PendingAllocations, policy.DiskReserveMiB)
	if !bounded {
		return refused("execution_disk_estimate_unavailable", "Pending execution storage demand exceeds supported capacity accounting.")
	}
	if demand.CreateAcknowledged {
		// The exact immutable check already has its durable Create ACK. Its
		// image copies occupy measured disk; Start does not create them again.
		// Keep every other pending allocation and the working reserve charged.
		diskDemand -= createDisk.Bytes
	} else if demand.RequiredCreateDiskBytes != 0 {
		// The typed check adapter pins its actual configured image before
		// admission. Worker callers retain the sampled configured-image demand.
		diskDemand -= createDisk.Bytes
		if demand.RequiredCreateDiskBytes > ^uint64(0)-diskDemand {
			return refused("execution_disk_estimate_unavailable", "Pending execution storage demand exceeds supported capacity accounting.")
		}
		diskDemand += demand.RequiredCreateDiskBytes
	}
	if demand.PendingDiskBytes > ^uint64(0)-diskDemand {
		return refused("execution_disk_estimate_unavailable", "Pending execution storage demand exceeds supported capacity accounting.")
	}
	diskDemand += demand.PendingDiskBytes
	if storageErr := resourcebudget.ValidateHostStorage(snap.Execution.HostStorage, runtime.GOOS == "windows" || snap.Host.Platform == "windows", diskDemand, now, 5*time.Second); storageErr != nil {
		code := "unavailable"
		var detail *resourcebudget.LocalStorageError
		if errors.As(storageErr, &detail) {
			code = detail.Code
		}
		return refused("host_storage_"+code, storageErr.Error())
	}
	if *snap.Execution.DaemonDisk.AvailableBytes < diskDemand {
		return refused("execution_disk_pressure", "Execution storage cannot preserve the working reserve for the pending sandbox copies.")
	}
	return executionCapacityResult{}
}
