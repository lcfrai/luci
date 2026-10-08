package api

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestExecutionCreateDiskEstimateUsesObservedDriverWithoutChangingRawTelemetry(t *testing.T) {
	for _, driver := range []string{"vfs", "overlay", "overlay2", "overlayfs", "fuse-overlayfs"} {
		t.Run(driver, func(t *testing.T) {
			raw := uint64(2294619632)
			snapshot := resourcebudget.ExecutionSnapshot{StorageDriver: driver, ImageVirtualSizeBytes: &raw, ImageSizeBasis: "image-inspect-raw-observation"}
			estimate, err := estimateExecutionCreateDisk(snapshot)
			want, basis := raw, "overlay-image-sized-create-allowance"
			if driver == "vfs" {
				want, basis = 2*raw, "vfs-init-and-writable-layer-copies"
			}
			if err != nil || estimate.Bytes != want || estimate.Basis != basis {
				t.Fatalf("driver %q estimate=%+v error=%v; want %d (%s)", driver, estimate, err, want, basis)
			}
			if raw != 2294619632 || snapshot.ImageSizeBasis != "image-inspect-raw-observation" || snapshot.StorageDriver != driver {
				t.Fatal("estimate overwrote raw engine telemetry")
			}
		})
	}
}

func TestExecutionCreateDiskEstimateRejectsUnknownAndOverflow(t *testing.T) {
	for _, test := range []struct {
		name, driver string
		size         *uint64
	}{
		{"missing driver", "", executionPointer(uint64(1))},
		{"unknown driver", "unknown-cow", executionPointer(uint64(1))},
		{"unreviewed driver", "windowsfilter", executionPointer(uint64(1))},
		{"driver not exact", "VFS", executionPointer(uint64(1))},
		{"missing image", "vfs", nil},
		{"zero image", "vfs", executionPointer(uint64(0))},
		{"copy overflow", "vfs", executionPointer(uint64(math.MaxUint64/2 + 1))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := estimateExecutionCreateDisk(resourcebudget.ExecutionSnapshot{StorageDriver: test.driver, ImageVirtualSizeBytes: test.size}); err == nil || got.Bytes != 0 {
				t.Fatal("unbounded or unknown estimate was accepted", got, err)
			}
		})
	}
	for _, test := range []struct {
		name    string
		bytes   uint64
		pending int
		reserve int
	}{
		{"multiply overflow", math.MaxUint64/2 + 1, 1, 0},
		{"reserve overflow", math.MaxUint64, 0, 1},
		{"negative pending", 1, -1, 0},
		{"negative reserve", 1, 0, -1},
		{"zero allocation", 0, 0, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := executionPendingDiskDemand(test.bytes, test.pending, test.reserve); ok || got != 0 {
				t.Fatal("invalid pending demand became an admissible small value", got, ok)
			}
		})
	}
	if got, ok := executionPendingDiskDemand(math.MaxUint64-(1<<20), 0, 1); !ok || got != math.MaxUint64 {
		t.Fatal("valid upper boundary was not retained", got, ok)
	}
}

func TestExecutionAdmissionVFSAccountsForBothCopiesAndDurablePendingWorkers(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "capacity")
	left := executionTestGate(t, shared)
	available := uint64(5) << 30
	observe := func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.StorageDriver = "vfs"
		s.Execution.DaemonDisk.AvailableBytes = executionPointer(available)
		return s
	}
	left.api.deps.ResourceSnapshot = observe
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	reserve := func(g *executionAdmission, id string, actor management.ExecutionActorRef, state, code string) {
		t.Helper()
		d, err := g.reserve(context.Background(), id, actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
		if err != nil || d.State != state || d.Code != code {
			t.Fatalf("%s with %d bytes: %+v %v; want %s/%s", id, available, d, err, state, code)
		}
	}
	// Two 2 GiB copies plus the 2 GiB reserve: the old one-copy gate admitted
	// this 5 GiB case, despite creation alone consuming approximately 4 GiB.
	reserve(left, "one", actor, "waiting", "execution_disk_pressure")
	assertExecutionLeaseCount(t, left, 0)
	available = uint64(6)<<30 - 1
	reserve(left, "one", actor, "waiting", "execution_disk_pressure")
	available++
	reserve(left, "one", actor, "admitted", "")
	// Rechecking an existing reservation replaces its demand, rather than
	// double-charging the same proposed worker.
	reserve(newExecutionAdmission(left.api), "one", actor, "admitted", "")
	if err := left.bind(context.Background(), "one", "s-unconfirmed"); err != nil {
		t.Fatal(err)
	}
	// Another independently opened controller must account for that unknown
	// first worker even though no engine container has appeared yet.
	right := executionTestGate(t, shared)
	right.api.deps.ResourceSnapshot = observe
	other := management.ExecutionActorRef{Local: true, ProjectID: "p-two"}
	available = uint64(10)<<30 - 1
	reserve(right, "two", other, "waiting", "execution_disk_pressure")
	assertExecutionLeaseCount(t, right, 1)
	available++
	reserve(right, "two", other, "admitted", "")
	assertExecutionLeaseCount(t, right, 2)
}

func TestExecutionAdmissionDiskCountsOnlyUnobservedCopies(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	if d, err := g.reserve(context.Background(), "one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil || d.State != "admitted" {
		t.Fatal(d, err)
	}
	if err := g.bind(context.Background(), "one", "s-observed"); err != nil {
		t.Fatal(err)
	}
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.StorageDriver = "vfs"
		s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(6) << 30)
		s.Execution.Containers = []resourcebudget.ContainerUsage{{ID: "s-observed", SandboxID: "s-observed", Owned: true, Kind: "agent", ObservedAt: time.Now(), MemoryLimitBytes: executionPointer(uint64(1) << 30), CPULimitMillis: executionPointer(int64(1000))}}
		return s
	}
	// Existing observed container allocation is already reflected by measured
	// free space. Charge only the new 4 GiB create allowance plus 2 GiB reserve.
	d, err := g.reserve(context.Background(), "two", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if err != nil || d.State != "admitted" {
		t.Fatal("observed container allocation was double-counted", d, err)
	}
}

func TestExecutionAdmissionUnknownOrOverflowingDiskEstimateRetainsQueue(t *testing.T) {
	for _, test := range []struct {
		name, driver string
		image        uint64
	}{
		{"unknown driver", "unverified-cow", 2 << 30},
		{"missing driver", "", 2 << 30},
		{"vfs overflow", "vfs", math.MaxUint64},
		{"reserve overflow", "overlay2", math.MaxUint64},
		{"zero image", "overlay2", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				s := executionFixtureSnapshot()
				s.Execution.StorageDriver = test.driver
				s.Execution.ImageVirtualSizeBytes = executionPointer(test.image)
				s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(math.MaxUint64))
				return s
			}
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
			d, err := g.reserve(context.Background(), "one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
			if err != nil || d.State != "waiting" || d.Code != "execution_disk_estimate_unavailable" {
				t.Fatal("unsafe disk estimate admitted allocation", d, err)
			}
			assertExecutionLeaseCount(t, g, 0)
		})
	}
}

func assertExecutionLeaseCount(t *testing.T, g *executionAdmission, want int) {
	t.Helper()
	if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != want {
			t.Errorf("execution leases=%d, want %d", len(l.Leases), want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
