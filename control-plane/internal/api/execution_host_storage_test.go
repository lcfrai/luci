package api

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func executionFixtureHostStorage(now time.Time) *resourcebudget.HostStorageBound {
	return &resourcebudget.HostStorageBound{Required: true, State: "confirmed", MountedVolumeCount: 1, Disk: &resourcebudget.Disk{Role: "docker-host-storage", Basis: "synthetic-confirmed-fixture", ObservedAt: now, AvailableBytes: executionPointer(uint64(100) << 30)}}
}

func TestExecutionWindowsHostStorageRefusesNewWorkWithoutReleasingExistingLease(t *testing.T) {
	for _, test := range []struct {
		name, code string
		change     func(*resourcebudget.HostStorageBound) *resourcebudget.HostStorageBound
	}{
		{"missing", "unavailable", func(*resourcebudget.HostStorageBound) *resourcebudget.HostStorageBound { return nil }},
		{"confirmation", "confirmation_required", func(b *resourcebudget.HostStorageBound) *resourcebudget.HostStorageBound {
			b.State = "confirmation-required"
			return b
		}},
		{"relocated", "confirmation_required", func(b *resourcebudget.HostStorageBound) *resourcebudget.HostStorageBound {
			b.State = "invalidated"
			return b
		}},
		{"stale", "unavailable", func(b *resourcebudget.HostStorageBound) *resourcebudget.HostStorageBound {
			b.Disk.ObservedAt = time.Now().Add(-time.Minute)
			return b
		}},
		{"unmeasured", "unavailable", func(b *resourcebudget.HostStorageBound) *resourcebudget.HostStorageBound {
			b.Disk.AvailableBytes = nil
			return b
		}},
		{"low-host-large-guest", "insufficient_capacity", func(b *resourcebudget.HostStorageBound) *resourcebudget.HostStorageBound {
			b.Disk.AvailableBytes = executionPointer(uint64(1) << 30)
			return b
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
			if d, err := g.reserve(context.Background(), "existing", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil || d.State != "admitted" {
				t.Fatal(d, err)
			}
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				snapshot := executionFixtureSnapshot()
				snapshot.Host.Platform = "windows"
				snapshot.Execution.HostStorage = test.change(snapshot.Execution.HostStorage)
				return snapshot // Guest free space remains 100 GiB.
			}
			for _, id := range []string{"existing", "new-work"} {
				d, err := g.reserve(context.Background(), id, actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
				if err != nil || d.State != "waiting" || d.Code != "host_storage_"+test.code {
					t.Fatal("Windows storage bound was bypassed", d, err)
				}
				assertExecutionLeaseCount(t, g, 1)
			}
		})
	}
}

func TestExecutionWindowsHostStorageCountsDriverAwarePendingCopiesAcrossReopen(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	available := uint64(6) << 30
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		snapshot := executionFixtureSnapshot()
		snapshot.Host.Platform = "windows"
		snapshot.Execution.StorageDriver = "vfs"
		snapshot.Execution.HostStorage.Disk.AvailableBytes = executionPointer(available)
		return snapshot
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	if d, err := g.reserve(context.Background(), "one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil || d.State != "admitted" {
		t.Fatal("VFS exact two-copy boundary did not admit", d, err)
	}
	reopened := newExecutionAdmission(g.api)
	available = uint64(10)<<30 - 1
	if d, err := reopened.reserve(context.Background(), "two", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil || d.Code != "host_storage_insufficient_capacity" {
		t.Fatal("pending create demand was lost after reopen", d, err)
	}
	assertExecutionLeaseCount(t, reopened, 1)
	available++
	if d, err := reopened.reserve(context.Background(), "two", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil || d.State != "admitted" {
		t.Fatal("fresh Windows lower bound could not admit exact pending demand", d, err)
	}
}

func TestExecutionNonWindowsHostDoesNotInventBackingRequirement(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		snapshot := executionFixtureSnapshot()
		snapshot.Host.Platform = "linux"
		snapshot.Execution.HostStorage = nil
		return snapshot
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	d, err := g.reserve(context.Background(), "one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if runtime.GOOS == "windows" {
		if err != nil || d.Code != "host_storage_unavailable" {
			t.Fatal("missing telemetry bypassed actual Windows host requirement", d, err)
		}
	} else if err != nil || d.State != "admitted" {
		t.Fatal("non-Windows admission gained a fictitious Windows backing requirement", d, err)
	}
}
