package api

import (
	"math"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestColdTypedExecutionDemandRetainsCapacityAndPendingWorkerRequirements(t *testing.T) {
	for _, tc := range []struct {
		name      string
		demand    executionCapacityDemand
		available uint64
		image     *uint64
		want      string
	}{
		{"first runtime with no sandbox image", executionCapacityDemand{RequiredCreateDiskBytes: 32 << 30}, 40 << 30, nil, ""},
		{"typed reservation plus reserve cannot fit", executionCapacityDemand{RequiredCreateDiskBytes: 32 << 30}, (34 << 30) - 1, nil, "execution_disk_pressure"},
		{"other held typed work remains charged", executionCapacityDemand{RequiredCreateDiskBytes: 32 << 30, PendingDiskBytes: 7 << 30}, 40 << 30, nil, "execution_disk_pressure"},
		{"ordinary worker still needs image", executionCapacityDemand{}, 40 << 30, nil, "execution_disk_unavailable"},
		{"pending worker still needs image", executionCapacityDemand{RequiredCreateDiskBytes: 32 << 30, PendingAllocations: 1}, 40 << 30, nil, "execution_disk_unavailable"},
		{"pending worker with image is charged", executionCapacityDemand{RequiredCreateDiskBytes: 32 << 30, PendingAllocations: 1}, (36 << 30) - 1, executionPointer(uint64(2) << 30), "execution_disk_pressure"},
		{"typed reserve overflow refused", executionCapacityDemand{RequiredCreateDiskBytes: math.MaxUint64}, math.MaxUint64, nil, "execution_disk_estimate_unavailable"},
		{"other pending disk overflow refused", executionCapacityDemand{RequiredCreateDiskBytes: 32 << 30, PendingDiskBytes: math.MaxUint64}, math.MaxUint64, nil, "execution_disk_estimate_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := executionFixtureSnapshot()
			snapshot.Execution.StorageDriver = "overlay2"
			snapshot.Execution.ImageVirtualSizeBytes = tc.image
			snapshot.Execution.DaemonDisk.AvailableBytes = executionPointer(tc.available)
			code, _ := evaluateExecutionCapacity(snapshot, executionFixturePolicy(), backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}, tc.demand, time.Now())
			if code != tc.want {
				t.Fatalf("capacity reason=%q, want %q", code, tc.want)
			}
		})
	}
}
