package api

import (
	"errors"
	"math"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// executionCreateDiskEstimate is an admission allowance, not observed usage or
// a quota on subsequent agent writes. Keep it separate from the raw image-size
// observation: image virtual size is neither container allocation nor free disk.
type executionCreateDiskEstimate struct {
	Bytes uint64
	Basis string
}

func estimateExecutionCreateDisk(snapshot resourcebudget.ExecutionSnapshot) (executionCreateDiskEstimate, error) {
	if snapshot.ImageVirtualSizeBytes == nil || *snapshot.ImageVirtualSizeBytes == 0 {
		return executionCreateDiskEstimate{}, errors.New("a positive sandbox image size is required to estimate execution storage")
	}
	var copies uint64
	var basis string
	switch snapshot.StorageDriver {
	case "vfs":
		// Moby creates both an init layer and the writable container layer.
		// VFS deep-copies each parent, so one image copy undercounts creation.
		// https://github.com/moby/moby/blob/v28.3.3/layer/layer_store.go#L520-L531
		// https://github.com/moby/moby/blob/v28.3.3/layer/layer_store.go#L645-L657
		// https://github.com/moby/moby/blob/v28.3.3/daemon/graphdriver/vfs/driver.go#L180-L192
		copies, basis = 2, "vfs-init-and-writable-layer-copies"
	case "overlay", "overlay2", "overlayfs", "fuse-overlayfs":
		// These copy-on-write drivers share lower image layers. Reserve a full
		// image-sized allowance for creation/copy-up, plus the separate policy
		// reserve. This is deliberately not a promise about arbitrary builds.
		copies, basis = 1, "overlay-image-sized-create-allowance"
	default:
		return executionCreateDiskEstimate{}, errors.New("the observed storage driver has no supported sandbox disk estimate; waiting for a supported execution storage observation")
	}
	if *snapshot.ImageVirtualSizeBytes > math.MaxUint64/copies {
		return executionCreateDiskEstimate{}, errors.New("sandbox disk estimate exceeds supported capacity accounting")
	}
	return executionCreateDiskEstimate{Bytes: *snapshot.ImageVirtualSizeBytes * copies, Basis: basis}, nil
}

// Pending allocations have not appeared in the engine's measured disk usage.
// Include all of them and the proposed child, while rejecting arithmetic that
// could wrap into a small, apparently safe reservation.
func executionPendingDiskDemand(perAllocation uint64, pending, reserveMiB int) (uint64, bool) {
	if perAllocation == 0 || pending < 0 || reserveMiB < 0 || uint64(reserveMiB) > math.MaxUint64>>20 {
		return 0, false
	}
	allocations := uint64(pending) + 1
	if allocations == 0 || perAllocation > math.MaxUint64/allocations {
		return 0, false
	}
	copies, reserve := perAllocation*allocations, uint64(reserveMiB)<<20
	if copies > math.MaxUint64-reserve {
		return 0, false
	}
	return copies + reserve, true
}
