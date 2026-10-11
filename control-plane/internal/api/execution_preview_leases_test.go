package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func previewTestGate(t *testing.T) *executionAdmission {
	t.Helper()
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.EngineID = httpCheckEngine
		return s
	}
	return g
}
func previewTestPrepared(g *executionAdmission, project, id, kind string, memory, cpu int) backend.PreparedPreview {
	image := ""
	if kind == "preview" {
		image = "sha256:" + strings.Repeat("f", 64)
	}
	return backend.PreparedPreview{Spec: backend.PreviewSpec{Identity: backend.PreviewIdentity{Protocol: backend.PreviewExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: id, ReleaseID: "release-" + id, ScopeDigest: g.scope, ProjectID: project, SourceDigest: strings.Repeat("a", 64), Commit: strings.Repeat("b", 40)}, Kind: kind, Resources: backend.Resources{MemoryMiB: memory, CPUMillis: cpu}, ImageID: image}, RequiredDiskBytes: 1 << 30, StorageDriver: "overlay2"}
}
func reservePreviewTest(t *testing.T, g *executionAdmission, p backend.PreparedPreview) previewExecutionLease {
	t.Helper()
	d, v, e := g.reservePreview(context.Background(), p, management.ExecutionActorRef{Local: true, ProjectID: p.Spec.Identity.ProjectID}, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	return v
}
func TestPreviewReservationsShareWorkerCheckAndPreviewBudgetAfterReopen(t *testing.T) {
	g := previewTestGate(t)
	p := previewTestPrepared(g, "p-one", "preview-build-a", "preview-build", 6144, 4000)
	v := reservePreviewTest(t, g, p)
	var err error
	v, err = g.transitionPreview(context.Background(), v, "building", "", "")
	if err != nil {
		t.Fatal(err)
	}
	g = newExecutionAdmission(g.api)
	if _, err = g.completePreview(context.Background(), v, false, ""); err == nil {
		t.Fatal("unknown builder released capacity")
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-two"}
	g.api.deps.Resources = backend.Resources{MemoryMiB: 3072, CPUMillis: 1000}
	d, e := g.reserve(context.Background(), "queue-two", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.Code != "memory_pressure" {
		t.Fatal("worker ignored retained builder", d, e)
	}
	check := backend.PreparedCheck{Spec: backend.CheckSpec{Identity: backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: "check-two", CheckID: "check-two", ProjectID: actor.ProjectID, ScopeDigest: g.scope, CommandDigest: strings.Repeat("a", 64)}, Resources: g.api.deps.Resources}, StorageDriver: "overlay2", ImageVirtualSizeBytes: 1 << 30}
	d, e = g.reserveCheck(context.Background(), check, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.Code != "memory_pressure" {
		t.Fatal("check ignored retained builder", d, e)
	}
	other := previewTestPrepared(g, "p-three", "preview-three", "preview", 3072, 1000)
	d, _, e = g.reservePreview(context.Background(), other, management.ExecutionActorRef{Local: true, ProjectID: "p-three"}, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.Code != "memory_pressure" {
		t.Fatal("preview ignored retained builder", d, e)
	}
	view := g.reservationObservation(context.Background())
	if view.executionReservationView == nil || view.executionReservationView.Count != 1 || view.executionReservationView.MemoryMiB != 6144 {
		t.Fatalf("shared preview resources hidden: %+v", view)
	}
	done, e := g.completePreview(context.Background(), v, true, "")
	if e != nil {
		t.Fatal(e)
	}
	d, e = g.reserve(context.Background(), "queue-two", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.State != "admitted" {
		t.Fatal("confirmed build completion did not free capacity", d, e)
	}
	if e = g.acknowledgePreviewRelease(context.Background(), []previewExecutionLease{done}); e != nil {
		t.Fatal(e)
	}
}
func TestPreviewAdmissionCountsForeignControllerAndNeverConfusesObservedIdentity(t *testing.T) {
	g := previewTestGate(t)
	p := previewTestPrepared(g, "p-one", "preview-a", "preview", 1024, 1000)
	v := reservePreviewTest(t, g, p)
	var err error
	v, err = g.transitionPreview(context.Background(), v, "creating", "", p.Spec.ImageID)
	if err != nil {
		t.Fatal(err)
	}
	v, err = g.transitionPreview(context.Background(), v, "created", strings.Repeat("c", 64), p.Spec.ImageID)
	if err != nil {
		t.Fatal(err)
	}
	for _, sameEngine := range []bool{false, true} {
		snap := executionFixtureSnapshot()
		snap.Execution.EngineID = "foreign"
		if sameEngine {
			snap.Execution.EngineID = httpCheckEngine
		}
		snap.Execution.Containers = []resourcebudget.ContainerUsage{{ID: v.Ref}}
		demand := executionCapacityDemand{}
		if err = addPreviewCapacity(snap, []previewExecutionLease{v}, "", "", &demand); err != nil {
			t.Fatal(err)
		}
		want := uint64(1024) << 20
		if sameEngine {
			want = 0
		}
		if demand.PendingMemory != want {
			t.Fatal("immutable same-engine observation mismatch", sameEngine, demand)
		}
	}
	forged := v
	forged.Identity.SourceDigest = strings.Repeat("e", 64)
	if _, err = g.transitionPreview(context.Background(), forged, "starting", v.Ref, v.Image); err == nil {
		t.Fatal("stale source rewrote allocation")
	}
	if _, err = g.completePreview(context.Background(), v, true, v.Image); err == nil {
		t.Fatal("mere container acknowledgement released capacity")
	}
}
func TestPreviewAdmissionKeepsHostGuestCPUAndDiskGuards(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		change     func(*resourcebudget.Snapshot)
	}{
		{"host memory", "memory_pressure", func(s *resourcebudget.Snapshot) {
			s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(1) << 30)
		}},
		{"guest memory", "memory_pressure", func(s *resourcebudget.Snapshot) { s.Execution.TotalMemoryBytes = executionPointer(uint64(1) << 30) }},
		{"busy CPU", "cpu_pressure", func(s *resourcebudget.Snapshot) { s.Host.CPU.UtilizationPercent = executionPointer(75.01) }},
		{"daemon disk", "execution_disk_pressure", func(s *resourcebudget.Snapshot) {
			s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(1) << 30)
		}},
		{"missing inventory", "resource_observation_unavailable", func(s *resourcebudget.Snapshot) { s.Execution.InventoryComplete = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := previewTestGate(t)
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				s := executionFixtureSnapshot()
				s.Execution.EngineID = httpCheckEngine
				tc.change(&s)
				return s
			}
			p := previewTestPrepared(g, "p-one", "preview-build-a", "preview-build", 1024, 1000)
			d, _, e := g.reservePreview(context.Background(), p, management.ExecutionActorRef{Local: true, ProjectID: "p-one"}, management.ExecutionAuthority{}, executionFixturePolicy())
			if e != nil || d.Code != tc.code {
				t.Fatal(d, e)
			}
			if _, e = g.previewLease(context.Background(), p.Spec.Identity.AllocationID); !errors.Is(e, errPreviewLeaseAbsent) {
				t.Fatal("refused request retained allocation", e)
			}
		})
	}
}
