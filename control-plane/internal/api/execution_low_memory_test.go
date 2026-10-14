package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func responsiveTestPolicy() product.SwarmSchedulerPolicy {
	return product.SwarmSchedulerPolicy{MaxActive: 4, MaxPerProject: 2, HeadroomPercent: 25, MinHeadroomMiB: 512, MaxHeadroomMiB: 1024, DiskReserveMiB: 2048}
}

func TestResponsiveLocalDefaultsPreserveSavedAdminSizesAndExplicitCeilings(t *testing.T) {
	env := sessionsNewEnv(t)
	env.api.deps.Resources = backend.DefaultLocalResources
	env.api.deps.SessionMemoryUsesAccountPolicy = true
	env.api.deps.SessionCPUUsesAccountPolicy = true
	ctx := management.WithExecutionAuthority(context.Background(), management.ExecutionAuthority{Actor: management.User{ID: "user-one"}, Policy: management.Policy{SessionMemoryMiB: 3072, SessionCPUMillis: 1500}})
	if got := env.api.sessionResources(ctx); got.MemoryMiB != 3072 || got.CPUMillis != 1500 {
		t.Fatal("saved admin override reduced", got)
	}
	env.api.deps.SessionMemoryUsesAccountPolicy = false
	if got := env.api.sessionResources(ctx); got.MemoryMiB != 1024 || got.CPUMillis != 1500 {
		t.Fatal("explicit per-dimension ceiling lost", got)
	}
}

func TestResponsiveBackgroundQueueUsesReauthorizedAdminSizing(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	g.api.deps.Resources = backend.DefaultLocalResources
	g.api.deps.SessionMemoryUsesAccountPolicy, g.api.deps.SessionCPUUsesAccountPolicy = true, true
	actor := management.ExecutionActorRef{UserID: "user-one", ProjectID: "p-one"}
	authority := management.ExecutionAuthority{Policy: management.Policy{MaxSessionsPerUser: 2, SessionMemoryMiB: 3072, SessionCPUMillis: 1500}}
	d, err := g.reserve(context.Background(), "queue-admin", actor.ProjectID, "api", 4, actor, authority, responsiveTestPolicy())
	if err != nil || d.State != "admitted" || d.Resources.MemoryMiB != 3072 || d.Resources.CPUMillis != 1500 {
		t.Fatal("background authority lost requested size", d, err)
	}
}

func TestResponsiveNeverEstimatesUnknownLegacyWorkerFromSmallerDefaults(t *testing.T) {
	for _, kind := range []string{"worker", "check", "preview", "runtime"} {
		t.Run(kind, func(t *testing.T) {
			g := previewTestGate(t)
			g.api.deps.Resources = backend.DefaultLocalResources
			if err := g.api.deps.Store.Create(registry.Session{ID: "s-legacy", SandboxID: "s-legacy", ProjectID: "p-old", Status: registry.StatusRunning, Harness: "codex"}); err != nil {
				t.Fatal(err)
			}
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-new"}
			var d executionDecision
			var err error
			switch kind {
			case "worker":
				d, err = g.reserve(context.Background(), "queue-new", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, responsiveTestPolicy())
			case "preview":
				d, _, err = g.reservePreview(context.Background(), previewTestPrepared(g, actor.ProjectID, "preview-new", "preview", 1024, 1000), actor, management.ExecutionAuthority{}, responsiveTestPolicy())
			case "runtime":
				d, _, err = g.reserveRuntimeImage(context.Background(), runtimeImagePrepared(g, "runtime-new"), management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, responsiveTestPolicy())
			case "check":
				p := backend.PreparedCheck{Spec: backend.CheckSpec{Identity: backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: "check-new", CheckID: "check-new", ProjectID: actor.ProjectID, ScopeDigest: g.scope, CommandDigest: strings.Repeat("a", 64)}, Resources: backend.DefaultLocalResources}, StorageDriver: "overlay2", ImageVirtualSizeBytes: 1 << 30}
				d, err = g.reserveCheck(context.Background(), p, actor, management.ExecutionAuthority{}, responsiveTestPolicy())
			}
			if err != nil || d.State != "waiting" || d.Code != "execution_legacy_demand_unknown" {
				t.Fatal("legacy allocation inferred from smaller current size", kind, d, err)
			}
		})
	}
}

func TestResponsiveCapacityOnFourEightSixteenAndThirtyTwoGiBMachines(t *testing.T) {
	for _, total := range []uint64{4, 8, 16, 32} {
		t.Run(string(rune('a'+total)), func(t *testing.T) {
			s := executionFixtureSnapshot()
			s.Host.Memory.EffectiveTotalBytes = executionPointer(total << 30)
			s.Execution.TotalMemoryBytes = executionPointer(total << 30)
			s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(2) << 30)
			p := responsiveTestPolicy()
			if p.MemoryHeadroomBytes(total<<30) != 1<<30 {
				t.Fatal("reserve grows with machine size", total)
			}
			if code, _ := evaluateExecutionCapacity(s, p, backend.DefaultLocalResources, executionCapacityDemand{}, time.Now()); code != "" {
				t.Fatal("small bounded work excluded despite exact capacity", total, code)
			}
			*s.Host.Memory.EffectiveAvailableBytes--
			r := evaluateExecutionCapacityWithEvidence(s, p, backend.DefaultLocalResources, executionCapacityDemand{}, time.Now())
			if r.Code != "memory_pressure" || r.Evidence.RequiredBytes != 2<<30 {
				t.Fatal("reserve boundary weakened", r)
			}
			*s.Host.Memory.EffectiveAvailableBytes = 2 << 30
			code, _ := evaluateExecutionCapacity(s, p, backend.DefaultLocalResources, executionCapacityDemand{PendingMemory: 1 << 30, AllMemory: 1 << 30}, time.Now())
			if code != "memory_pressure" {
				t.Fatal("pending work omitted", code)
			}
		})
	}
}

func TestLocal044WindowsDockerLaunchChargesHostOutsideContainerLimit(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	g.api.deps.Resources = backend.Resources{MemoryMiB: 512, CPUMillis: 1000}
	available := uint64(2015 * 1000000) // observed Windows preflight, in decimal bytes
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Host.Platform = "windows"
		s.Host.Memory.EffectiveAvailableBytes = &available
		return s
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-local044"}
	reserve := func(id string) executionDecision {
		t.Helper()
		d, err := g.reserve(context.Background(), id, actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, responsiveTestPolicy())
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := reserve("local044-first"); d.State != "waiting" || d.Code != "memory_pressure" || d.Evidence.RequiredBytes != 3584<<20 {
		t.Fatal("container-only preflight would admit the crashing launch", d)
	}
	available = 3480 * 1000000 // reported idle recovery still needs launch headroom
	if d := reserve("local044-first"); d.State != "waiting" || d.Code != "memory_pressure" {
		t.Fatal("persistent host pressure was ignored after the worker exited", d)
	}
	available = 4 << 30
	if d := reserve("local044-first"); d.State != "admitted" || d.Resources.MemoryMiB != 512 {
		t.Fatal("bounded launch allowance changed the worker cap or blocked sufficient capacity", d)
	}
	// This observation is specific to Windows Docker Desktop. Linux containers
	// and the host's lightweight project routes must not inherit a RAM wall.
	g2 := executionTestGate(t, filepath.Join(t.TempDir(), "linux-capacity"))
	g2.api.deps.Resources = backend.Resources{MemoryMiB: 512, CPUMillis: 1000}
	g2.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Host.Platform = "linux"
		v := uint64(2015 * 1000000)
		s.Host.Memory.EffectiveAvailableBytes = &v
		return s
	}
	d, err := g2.reserve(context.Background(), "local044-linux", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, responsiveTestPolicy())
	if err != nil || d.State != "admitted" {
		t.Fatal("unobserved Linux launch allowance added", d, err)
	}
}

func TestLocal044SecondManagedOwnerCannotReuseFirstUnobservedLaunchAllowance(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	g.api.deps.Resources = backend.Resources{MemoryMiB: 512, CPUMillis: 1000}
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Host.Platform = "windows"
		s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(4) << 30)
		return s
	}
	first := management.ExecutionActorRef{UserID: "person-one", ProjectID: "p-one"}
	second := management.ExecutionActorRef{UserID: "person-two", ProjectID: "p-two"}
	authority := management.ExecutionAuthority{Policy: management.Policy{MaxSessionsPerUser: 2, SessionMemoryMiB: 512, SessionCPUMillis: 1000}}
	decision, err := g.reserve(context.Background(), "local044-first-owner", first.ProjectID, "harness:codex", 4, first, authority, responsiveTestPolicy())
	if err != nil || decision.State != "admitted" {
		t.Fatal("first finite launch could not reserve", decision, err)
	}
	other := &executionAdmission{api: g.api, path: g.path, scope: strings.Repeat("a", 64)}
	decision, err = other.reserve(context.Background(), "local044-second-owner", second.ProjectID, "harness:codex", 4, second, authority, responsiveTestPolicy())
	if err != nil || decision.Code != "memory_pressure" || decision.Evidence.RequiredBytes != 6144<<20 {
		t.Fatal("second managed owner spent the first owner's unobserved Docker/WSL allowance", decision, err)
	}
}

func TestLocal044PendingCheckAndPreviewKeepSeparateHostAllowances(t *testing.T) {
	snap := executionFixtureSnapshot()
	snap.Host.Platform = "windows"
	demand := executionCapacityDemand{}
	check := checkExecutionLease{Resources: backend.Resources{MemoryMiB: 512, CPUMillis: 1000}, HostLaunchBytes: 2 << 30, State: "reserved", UpdatedAt: time.Now()}
	preview := previewExecutionLease{Resources: backend.Resources{MemoryMiB: 512, CPUMillis: 1000}, HostLaunchBytes: 2 << 30, State: "reserved", UpdatedAt: time.Now()}
	if err := addCheckCapacity(snap, []checkExecutionLease{check}, "", "", &demand); err != nil {
		t.Fatal(err)
	}
	if err := addPreviewCapacity(snap, []previewExecutionLease{preview}, "", "", &demand); err != nil {
		t.Fatal(err)
	}
	if demand.PendingHostLaunchBytes != 4<<30 || demand.PendingMemory != 1<<30 {
		t.Fatal("pending check and preview spent each other's host launch headroom", demand)
	}
}

func TestResponsiveCapacityRetainsGuestCPUAndFreshnessGuards(t *testing.T) {
	for _, c := range []struct {
		name, want string
		change     func(*resourcebudget.Snapshot, *executionCapacityDemand)
	}{
		{"guest contention", "memory_pressure", func(s *resourcebudget.Snapshot, d *executionCapacityDemand) {
			s.Execution.TotalMemoryBytes = executionPointer(uint64(4) << 30)
			d.AllMemory = 3 << 30
		}},
		{"CPU pressure", "cpu_pressure", func(s *resourcebudget.Snapshot, _ *executionCapacityDemand) {
			s.Host.CPU.UtilizationPercent = executionPointer(float64(90))
		}},
		{"CPU demand", "cpu_capacity", func(_ *resourcebudget.Snapshot, d *executionCapacityDemand) { d.CPU = 6000 }},
		{"inventory unknown", "resource_observation_unavailable", func(s *resourcebudget.Snapshot, _ *executionCapacityDemand) { s.Execution.InventoryComplete = false }},
		{"stale host", "resource_observation_unavailable", func(s *resourcebudget.Snapshot, _ *executionCapacityDemand) {
			s.Host.ObservedAt = time.Now().Add(-6 * time.Second)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, demand := executionFixtureSnapshot(), executionCapacityDemand{}
			c.change(&s, &demand)
			if code, _ := evaluateExecutionCapacity(s, responsiveTestPolicy(), backend.DefaultLocalResources, demand, time.Now()); code != c.want {
				t.Fatal(code, c.want)
			}
		})
	}
}

func TestResponsiveReservationRefusalRecoversWithoutDuplicateAfterReopen(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	available := uint64(1536) << 20
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Host.Memory.EffectiveAvailableBytes = &available
		return s
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	reserve := func(g *executionAdmission) executionDecision {
		t.Helper()
		d, err := g.reserve(context.Background(), "queue-low-memory", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, responsiveTestPolicy())
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := reserve(g); d.State != "waiting" || d.Code != "memory_pressure" {
		t.Fatal(d)
	}
	available = 2 << 30
	if d := reserve(g); d.State != "admitted" {
		t.Fatal(d)
	}
	reopened := newExecutionAdmission(g.api)
	if d := reserve(reopened); d.State != "admitted" {
		t.Fatal(d)
	}
	if err := reopened.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 1 {
			t.Fatal("duplicate reservation", l)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeBuildBudgetIsSeparateAndReadbackMatchesActualAllocation(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	a.deps.Resources = backend.Resources{MemoryMiB: 4096, CPUMillis: 2000}
	f.build = func(_ context.Context, p backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
		if p.Spec.Resources != backend.DefaultRuntimeBuildResources {
			t.Fatal("build inherited worker demand", p.Spec.Resources)
		}
		return backend.RuntimeImageBuildResult{Settled: true, ImageID: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}, nil
	}
	if out, err := a.installRuntimeImage(context.Background(), req, installerPublish); err != nil || out.Phase != "ready" {
		t.Fatal(out, err)
	}
	a.deps.RuntimeBuildResources = backend.Resources{MemoryMiB: 2048, CPUMillis: 1500}
	if got := a.runtimeBuildResources(nil); got != a.deps.RuntimeBuildResources {
		t.Fatal("explicit build override lost", got)
	}
	p := management.Policy{CheckMemoryMiB: 512, CheckCPUMillis: 500}
	if got := a.runtimeBuildResources(&p); got.MemoryMiB != 512 || got.CPUMillis != 500 {
		t.Fatal("admin ceiling lost", got)
	}
	data, err := json.Marshal(a.queuedResourceView(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		Limits struct {
			Worker, RuntimeBuild struct {
				MemoryMiB int `json:"memoryMiB"`
				CPUMillis int `json:"cpuMillis"`
			}
		}
	}
	if err := json.Unmarshal(data, &view); err != nil {
		t.Fatal(err)
	}
	if view.Limits.Worker.MemoryMiB != 4096 || view.Limits.RuntimeBuild.MemoryMiB != 2048 || view.Limits.RuntimeBuild.CPUMillis != 1500 {
		t.Fatal("readback differs from enforced limits", string(data))
	}
}
