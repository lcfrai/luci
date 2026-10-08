package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

const httpCheckVFSImageBytes = uint64(2294629077)

// The real HTTP/product/ledger path receives a different fresh observation only
// after the controlled backend has acknowledged Create. No Docker is used.
type diskAdmissionCheckExecutor struct {
	*httpCheckExecutor
	afterCreate func(backend.PreparedCheck, string)
}

func (b *diskAdmissionCheckExecutor) PrepareCheck(ctx context.Context, spec backend.CheckSpec) (backend.PreparedCheck, error) {
	p, err := b.httpCheckExecutor.PrepareCheck(ctx, spec)
	p.ImageVirtualSizeBytes, p.StorageDriver = httpCheckVFSImageBytes, "vfs"
	return p, err
}

func (b *diskAdmissionCheckExecutor) CreateCheck(ctx context.Context, p backend.PreparedCheck) (string, error) {
	ref, err := b.httpCheckExecutor.CreateCheck(ctx, p)
	if err == nil && b.afterCreate != nil {
		b.afterCreate(p, ref)
	}
	return ref, err
}

func checkDiskPolicy(t *testing.T, f *swarmSessionFixture) {
	t.Helper()
	var settings struct {
		Revision int `json:"revision"`
	}
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
		t.Fatal("scheduler policy read", w.Code)
	}
	policy := executionFixturePolicy()
	policy.DiskReserveMiB = 1024
	body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": policy, "dispatchPaused": true})
	if w = f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body)); w.Code != 200 {
		t.Fatal("scheduler policy write", w.Code)
	}
}

func checkDiskSnapshot(free uint64) resourcebudget.Snapshot {
	s := httpCheckSnapshot(12)
	s.Host.Memory.EffectiveTotalBytes = executionPointer(uint64(32) << 30)
	s.Execution.TotalMemoryBytes = executionPointer(uint64(32) << 30)
	s.Execution.StorageDriver = "vfs"
	s.Execution.ImageVirtualSizeBytes = executionPointer(httpCheckVFSImageBytes)
	s.Host.Disks[0].AvailableBytes = executionPointer(free)
	s.Execution.DaemonDisk.AvailableBytes = executionPointer(free)
	return s
}

func TestProjectCheckHTTPDiskAdmissionAcrossCreate(t *testing.T) {
	const reserve = uint64(1 << 30)
	for _, tc := range []struct {
		name        string
		beforeFree  uint64
		afterFree   uint64
		change      func(*resourcebudget.Snapshot)
		createErr   error
		wantStatus  string
		wantCode    string
		wantCreates int
		wantStarts  int
		wantRemoves int
		wantHeld    bool
	}{
		{name: "new-create-needs-both-vfs-copies", beforeFree: reserve + 2*httpCheckVFSImageBytes - 1, wantStatus: "blocked", wantCode: "execution_disk_pressure"},
		{name: "acknowledged-create-needs-no-second-copy", afterFree: reserve + (512 << 20), wantStatus: "passed", wantCreates: 1, wantStarts: 1, wantRemoves: 1},
		{name: "acknowledged-visible-create-needs-no-second-copy", afterFree: reserve + (512 << 20), change: func(s *resourcebudget.Snapshot) {
			s.Execution.Containers = []resourcebudget.ContainerUsage{{ID: "own-ref", Kind: "check", MemoryLimitBytes: executionPointer(reserve), CPULimitMillis: executionPointer(int64(1000))}}
		}, wantStatus: "passed", wantCreates: 1, wantStarts: 1, wantRemoves: 1},
		{name: "created-still-needs-working-reserve", afterFree: reserve - 1, wantStatus: "blocked", wantCode: "disk_pressure", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-host-memory-headroom", change: func(s *resourcebudget.Snapshot) {
			s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(9<<30) - 1)
		}, wantStatus: "blocked", wantCode: "memory_pressure", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-execution-memory", change: func(s *resourcebudget.Snapshot) {
			s.Execution.TotalMemoryBytes = executionPointer(uint64(2 << 30))
		}, wantStatus: "blocked", wantCode: "memory_pressure", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-fresh-host", change: func(s *resourcebudget.Snapshot) {
			s.Host.ObservedAt = time.Now().Add(-10 * time.Second)
		}, wantStatus: "blocked", wantCode: "resource_observation_unavailable", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-fresh-execution", change: func(s *resourcebudget.Snapshot) {
			s.Execution.ObservedAt = time.Now().Add(-10 * time.Second)
		}, wantStatus: "blocked", wantCode: "resource_observation_unavailable", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-fresh-disk", change: func(s *resourcebudget.Snapshot) {
			s.Execution.DaemonDisk.ObservedAt = time.Now().Add(-10 * time.Second)
		}, wantStatus: "blocked", wantCode: "execution_disk_unavailable", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-complete-inventory", change: func(s *resourcebudget.Snapshot) {
			s.Execution.InventoryComplete = false
		}, wantStatus: "blocked", wantCode: "resource_observation_unavailable", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-same-engine", change: func(s *resourcebudget.Snapshot) {
			s.Execution.EngineID = "other-engine"
		}, wantStatus: "blocked", wantCode: "resource_observation_unavailable", wantCreates: 1, wantRemoves: 1},
		{name: "created-still-needs-other-workload-limits", change: func(s *resourcebudget.Snapshot) {
			s.Execution.Containers = []resourcebudget.ContainerUsage{{ID: "other-ref", Kind: "agent", Owned: true}}
		}, wantStatus: "blocked", wantCode: "execution_limits_unknown", wantCreates: 1, wantRemoves: 1},
		{name: "unacknowledged-create-retains-capacity", createErr: backend.ErrUnavailable, wantStatus: "unknown", wantCode: "check_create_unconfirmed", wantCreates: 1, wantHeld: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			base := newHTTPCheckExecutor(f.env.be)
			base.createErr = tc.createErr
			be := &diskAdmissionCheckExecutor{httpCheckExecutor: base}
			wireHTTPCheckFixture(t, f, base, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
			checkDiskPolicy(t, f)
			f.env.api.deps.Backend = be
			var created atomic.Pointer[string]
			be.afterCreate = func(_ backend.PreparedCheck, ref string) { created.Store(&ref) }
			f.env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				free := tc.beforeFree
				ref := created.Load()
				if ref != nil {
					free = tc.afterFree
				}
				if free == 0 {
					free = 16 << 30
				}
				s := checkDiskSnapshot(free)
				if ref != nil && tc.change != nil {
					tc.change(&s)
					for i := range s.Execution.Containers {
						if s.Execution.Containers[i].ID == "own-ref" {
							s.Execution.Containers[i].ID = *ref
						}
					}
				}
				return s
			}
			started := checkHTTPStart(t, f, "user-check")
			row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
				return row.Status != "queued" && row.Status != "running" && row.ResourcesHeld != nil && *row.ResourcesHeld == tc.wantHeld
			})
			checkHTTPIdle(t, f, row)
			if row.Status != tc.wantStatus || row.ReasonCode != tc.wantCode {
				t.Fatalf("status=%s code=%s, want status=%s code=%s", row.Status, row.ReasonCode, tc.wantStatus, tc.wantCode)
			}
			if !tc.wantHeld {
				checkHTTPCompletionAcknowledged(t, f, row)
			}
			_, creates, starts, _, removes := base.counts()
			if creates != tc.wantCreates || starts != tc.wantStarts || removes != tc.wantRemoves {
				t.Fatalf("Create/Start/Remove=%d/%d/%d, want %d/%d/%d", creates, starts, removes, tc.wantCreates, tc.wantStarts, tc.wantRemoves)
			}
			if tc.wantStarts == 0 && (row.Commands[0].ExitCode != nil || row.Commands[0].Output != "") {
				t.Fatal("unstarted check fabricated command evidence")
			}
		})
	}
}
