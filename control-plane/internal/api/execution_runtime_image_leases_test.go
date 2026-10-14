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
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
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

func TestRuntimePullPhasesRetainExactAllocationAcrossRestart(t *testing.T) {
	g := previewTestGate(t)
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.EngineID = httpCheckEngine
		s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(64 << 30))
		return s
	}
	p := runtimeImagePrepared(g, "pull-one")
	p.PullBundle = &runtimeoci.PullBundle{ManifestSHA256: strings.Repeat("b", 64)}
	lease := runtimeImageReserve(t, g, p)
	if lease.DistributionMode != "compressed-pull" || lease.PullManifestSHA256 != p.PullBundle.ManifestSHA256 {
		t.Fatal("compressed pull reservation did not retain package identity", lease)
	}
	var err error
	lease, err = g.transitionRuntimeImage(context.Background(), lease, "building")
	if err != nil || lease.ImportStage != "staging" {
		t.Fatal("pull staging intent missing", lease, err)
	}
	before := lease
	lease, err = g.advanceRuntimeImagePull(context.Background(), lease, "staging", "bootstrap-loading", "")
	if err != nil {
		t.Fatal(err)
	}
	g = newExecutionAdmission(g.api)
	if _, err := g.advanceRuntimeImagePull(context.Background(), before, "staging", "bootstrap-loading", ""); err == nil {
		t.Fatal("restart replayed a completed bootstrap intent")
	}
	stored, err := g.runtimeImageLease(context.Background(), lease.ID)
	if err != nil || stored != lease {
		t.Fatal("restart lost exact charged pull phase", stored, err)
	}
	if _, err := g.advanceRuntimeImagePull(context.Background(), lease, "bootstrap-loading", "bootstrap-loaded", "not-an-id"); err == nil {
		t.Fatal("invalid bootstrap output entered durable ledger")
	}
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

func TestRuntimeImageColdPullChargeRefusesLowHostHeadroomWithoutRaisingRunCap(t *testing.T) {
	g := previewTestGate(t)
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.EngineID = httpCheckEngine
		s.Execution.ImageVirtualSizeBytes = nil
		s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(64 << 30))
		s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(9 << 30))
		return s
	}
	p := runtimeImagePrepared(g, "cold-first")
	p.AdmissionMemoryMiB = p.Spec.Resources.MemoryMiB + 2048
	d, _, err := g.reserveRuntimeImage(context.Background(), p, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
	if err != nil || d.Code != "memory_pressure" || d.Evidence.RequiredBytes != uint64(10<<30) {
		t.Fatalf("cold pull was not charged: %+v %v", d, err)
	}
	if p.Spec.Resources.MemoryMiB != 4096 {
		t.Fatal("cold allowance changed Docker RUN cap")
	}
	p.AdmissionMemoryMiB = p.Spec.Resources.MemoryMiB
	d, lease, err := g.reserveRuntimeImage(context.Background(), p, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, executionFixturePolicy())
	if err != nil || d.State != "admitted" || lease.AdmissionMemoryMiB != 4096 {
		t.Fatalf("warm capped build did not admit: %+v %+v %v", d, lease, err)
	}
}

func TestPackagedRuntimeLoadAllowanceRefusesAndSurvivesReopen(t *testing.T) {
	g := previewTestGate(t)
	available := uint64(7 << 30)
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.EngineID = httpCheckEngine
		s.Execution.ImageVirtualSizeBytes = nil
		s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(64 << 30))
		s.Host.Memory.EffectiveAvailableBytes = executionPointer(available)
		return s
	}
	p := runtimeImagePrepared(g, "packaged-load")
	p.Spec.Resources.MemoryMiB = 1024
	p.AdmissionMemoryMiB = 4096 // verified 2 GiB archive + 1.5 GiB layers + 512 MiB daemon allowance
	p.Bundle = &runtimeoci.Bundle{}
	actor := management.ExecutionActorRef{Local: true}
	policy := executionFixturePolicy()
	d, _, err := g.reserveRuntimeImage(context.Background(), p, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.Code != "memory_pressure" || d.Evidence.RequiredBytes != uint64(8<<30) || !strings.Contains(d.Message, "You can keep using Luci") {
		t.Fatal("load staging pressure did not refuse before Docker effect", d, err)
	}
	if view := g.reservationObservation(context.Background()); view.Count != 0 {
		t.Fatal("capacity refusal retained an unstarted charge", view)
	}
	available = 9 << 30
	d, lease, err := g.reserveRuntimeImage(context.Background(), p, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.State != "admitted" || lease.DistributionMode != "packaged" || lease.AdmissionMemoryMiB != 4096 || p.Spec.Resources.MemoryMiB != 1024 {
		t.Fatal("import allowance changed worker limit or lease identity", d, lease, err)
	}
	lease, err = g.transitionRuntimeImage(context.Background(), lease, "building")
	if err != nil || lease.ImportStage != "importing" {
		t.Fatal("import phase did not retain exact charge", lease, err)
	}
	g = newExecutionAdmission(g.api)
	if view := g.reservationObservation(context.Background()); view.Count != 1 || view.MemoryMiB != 4096 {
		t.Fatal("restart dropped uncertain import allowance", view)
	}
}
func TestRuntimeImageAfterPullChecksRemainingWorkButRetainsCompetingCharge(t *testing.T) {
	g := previewTestGate(t)
	available := uint64(11 << 30)
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.EngineID = httpCheckEngine
		s.Execution.ImageVirtualSizeBytes = nil
		s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(64 << 30))
		s.Host.Memory.EffectiveAvailableBytes = executionPointer(available)
		return s
	}
	p := runtimeImagePrepared(g, "cold-stage")
	p.AdmissionMemoryMiB = p.Spec.Resources.MemoryMiB + 2048
	actor := management.ExecutionActorRef{Local: true}
	policy := executionFixturePolicy()
	first, lease, err := g.reserveRuntimeImage(context.Background(), p, actor, management.ExecutionAuthority{}, policy)
	if err != nil || first.State != "admitted" {
		t.Fatal("cold work was not initially admitted", first, err)
	}
	lease, err = g.transitionRuntimeImage(context.Background(), lease, "building")
	if err != nil {
		t.Fatal(err)
	}
	available = uint64(9 << 30) // base pulls have consumed observed host memory
	full, _, err := g.reserveRuntimeImage(context.Background(), p, actor, management.ExecutionAuthority{}, policy)
	if err != nil || full.Code != "memory_pressure" {
		t.Fatal("fixture did not expose cold allowance double count", full, err)
	}
	remaining, after, err := g.reserveRuntimeImageAfterPull(context.Background(), p, actor, management.ExecutionAuthority{}, policy)
	if err != nil || remaining.State != "admitted" || after != lease {
		t.Fatal("remaining capped build did not pass without rewriting lease", remaining, after, err)
	}
	if view := g.reservationObservation(context.Background()); view.Count != 1 || view.MemoryMiB != p.AdmissionMemoryMiB {
		t.Fatal("competing work lost the conservative retained claim", view)
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
