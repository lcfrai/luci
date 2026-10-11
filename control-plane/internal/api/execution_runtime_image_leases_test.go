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

func runtimeImagePrepared(g *executionAdmission, id string) backend.PreparedRuntimeImage {
	return backend.PreparedRuntimeImage{Spec: backend.RuntimeImageSpec{Identity: backend.RuntimeImageIdentity{Protocol: backend.RuntimeImageExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: id, JobID: "job-" + id, ScopeDigest: g.scope, SourceDigest: strings.Repeat("a", 64)}, Resources: backend.Resources{MemoryMiB: 4096, CPUMillis: 2000}, ImageTag: "luci/sandbox:fixture"}, RequiredDiskBytes: backend.RuntimeImageRequiredDiskBytes, StorageDriver: "overlay2"}
}
func runtimeImageReserve(t *testing.T, g *executionAdmission, p backend.PreparedRuntimeImage) runtimeImageExecutionLease {
	t.Helper()
	d, v, e := g.reserveRuntimeImage(context.Background(), p, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	return v
}
func TestRuntimeImageColdAdmissionPreservesFortyGiBFloor(t *testing.T) {
	for _, disk := range []uint64{39, 40} {
		t.Run(string(rune('a'+disk-39)), func(t *testing.T) {
			g := previewTestGate(t)
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				s := executionFixtureSnapshot()
				s.Execution.EngineID = httpCheckEngine
				s.Execution.ImageVirtualSizeBytes = nil
				s.Execution.DaemonDisk.AvailableBytes = executionPointer(disk << 30)
				return s
			}
			p := runtimeImagePrepared(g, "runtime-first")
			d, _, e := g.reserveRuntimeImage(context.Background(), p, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
			if e != nil {
				t.Fatal(e)
			}
			if disk == 40 && d.State != "admitted" || disk == 39 && d.Code != "execution_disk_pressure" {
				t.Fatal("cold installer weakened forty-GiB demand or required an existing worker image", disk, d)
			}
		})
	}
}
func TestRuntimeImageUnknownBuildSurvivesRestartAndChargesEveryAllocationPath(t *testing.T) {
	g := previewTestGate(t)
	p := runtimeImagePrepared(g, "runtime-one")
	p.Spec.Resources.MemoryMiB = 6144
	v := runtimeImageReserve(t, g, p)
	var e error
	v, e = g.transitionRuntimeImage(context.Background(), v, "building")
	if e != nil {
		t.Fatal(e)
	}
	g = newExecutionAdmission(g.api)
	if _, e = g.completeRuntimeImage(context.Background(), v, false, ""); e == nil {
		t.Fatal("unconfirmed cancellation released installer")
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-work"}
	g.api.deps.Resources = backend.Resources{MemoryMiB: 3072, CPUMillis: 1000}
	d, e := g.reserve(context.Background(), "queue-work", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.Code != "memory_pressure" {
		t.Fatal("worker ignored installer", d, e)
	}
	check := backend.PreparedCheck{Spec: backend.CheckSpec{Identity: backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: "check-work", CheckID: "check-work", ProjectID: actor.ProjectID, ScopeDigest: g.scope, CommandDigest: strings.Repeat("a", 64)}, Resources: g.api.deps.Resources}, StorageDriver: "overlay2", ImageVirtualSizeBytes: 1 << 30}
	d, e = g.reserveCheck(context.Background(), check, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.Code != "memory_pressure" {
		t.Fatal("check ignored installer", d, e)
	}
	preview := previewTestPrepared(g, actor.ProjectID, "preview-work", "preview", 3072, 1000)
	d, _, e = g.reservePreview(context.Background(), preview, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.Code != "memory_pressure" {
		t.Fatal("preview ignored installer", d, e)
	}
	view := g.reservationObservation(context.Background())
	if view.Status != "ready" || view.Count != 1 || view.MemoryMiB != 6144 {
		t.Fatal("installer hidden from shared resource view", view)
	}
	done, e := g.completeRuntimeImage(context.Background(), v, true, "sha256:"+strings.Repeat("c", 64))
	if e != nil {
		t.Fatal(e)
	}
	d, e = g.reserve(context.Background(), "queue-work", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.State != "admitted" {
		t.Fatal("settled build did not release only its own capacity", d, e)
	}
	if e = g.acknowledgeRuntimeImageCompletion(context.Background(), done); e != nil {
		t.Fatal(e)
	}
	view = g.reservationObservation(context.Background())
	if view.Count != 1 || view.MemoryMiB != 3072 {
		t.Fatal("installer acknowledgement lost unrelated worker", view)
	}
}
func TestRuntimeImageEngineWidePublicationSlotSurvivesScopeAndReopen(t *testing.T) {
	g := previewTestGate(t)
	p := runtimeImagePrepared(g, "runtime-first")
	v := runtimeImageReserve(t, g, p)
	other := executionTestGate(t, filepath.Dir(g.path))
	other.api.deps.ResourceSnapshot = g.api.deps.ResourceSnapshot
	candidate := runtimeImagePrepared(other, "runtime-other")
	checkBlocked := func() {
		t.Helper()
		d, _, e := other.reserveRuntimeImage(context.Background(), candidate, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
		if e != nil || d.Code != "runtime_install_retained" {
			t.Fatal("another account bypassed installer slot", d, e)
		}
	}
	checkBlocked()
	var e error
	v, e = g.transitionRuntimeImage(context.Background(), v, "building")
	if e != nil {
		t.Fatal(e)
	}
	done, e := g.completeRuntimeImage(context.Background(), v, true, "sha256:"+strings.Repeat("d", 64))
	if e != nil {
		t.Fatal(e)
	}
	other = newExecutionAdmission(other.api)
	checkBlocked()
	if view := g.reservationObservation(context.Background()); view.Count != 0 {
		t.Fatal("settled image still charges memory", view)
	}
	stale := done
	stale.Revision--
	if e = g.acknowledgeRuntimeImageCompletion(context.Background(), stale); e == nil {
		t.Fatal("stale acknowledgement retired publication slot")
	}
	if e = g.acknowledgeRuntimeImageCompletion(context.Background(), done); e != nil {
		t.Fatal(e)
	}
	d, _, e := other.reserveRuntimeImage(context.Background(), candidate, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
	if e != nil || d.State != "admitted" {
		t.Fatal("acknowledged publication kept installer slot", d, e)
	}
}
func TestRuntimeImageRejectsProjectImpersonationUnknownDemandAndStalePhase(t *testing.T) {
	g := previewTestGate(t)
	p := runtimeImagePrepared(g, "runtime-one")
	for _, actor := range []management.ExecutionActorRef{{Local: true, ProjectID: "p-fake"}, {Local: true, UserID: "fake"}, {}} {
		if _, _, e := g.reserveRuntimeImage(context.Background(), p, actor, management.ExecutionAuthority{}, executionFixturePolicy()); e == nil {
			t.Fatal("installer accepted invalid actor scope", actor)
		}
	}
	tooSmall := p
	tooSmall.RequiredDiskBytes = 1 << 30
	if _, _, e := g.reserveRuntimeImage(context.Background(), tooSmall, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy()); e == nil {
		t.Fatal("installer understated bundled demand")
	}
	v := runtimeImageReserve(t, g, p)
	running, e := g.transitionRuntimeImage(context.Background(), v, "building")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = g.transitionRuntimeImage(context.Background(), v, "building"); e == nil {
		t.Fatal("same operation launched twice")
	}
	if _, e = g.completeRuntimeImage(context.Background(), v, true, ""); e == nil {
		t.Fatal("stale no-launch proof released active build")
	}
	if _, e = g.runtimeImageLease(context.Background(), "unrelated"); !errors.Is(e, errRuntimeImageLeaseAbsent) {
		t.Fatal(e)
	}
	if actual, e := g.runtimeImageLease(context.Background(), v.ID); e != nil || actual != running {
		t.Fatal("failed operations changed retained installer", actual, e)
	}
}
