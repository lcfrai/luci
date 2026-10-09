package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func executionPointer[T any](v T) *T { return &v }
func executionFixtureSnapshot() resourcebudget.Snapshot {
	now := time.Now()
	return resourcebudget.Snapshot{Host: resourcebudget.HostSnapshot{ObservedAt: now, Status: "ready", CPU: resourcebudget.CPU{EffectiveMillis: executionPointer(int64(8000))}, Memory: resourcebudget.Memory{EffectiveTotalBytes: executionPointer(uint64(16) << 30), EffectiveAvailableBytes: executionPointer(uint64(12) << 30)}, Disks: []resourcebudget.Disk{{Role: "state", ObservedAt: now, AvailableBytes: executionPointer(uint64(100) << 30)}}}, Execution: resourcebudget.ExecutionSnapshot{ObservedAt: now, Status: "ready", InventoryComplete: true, TotalMemoryBytes: executionPointer(uint64(16) << 30), CPUMillis: executionPointer(int64(8000)), StorageDriver: "overlay2", HostStorage: executionFixtureHostStorage(now), ImageVirtualSizeBytes: executionPointer(uint64(2) << 30), DaemonDisk: &resourcebudget.Disk{Role: "execution", ObservedAt: now, AvailableBytes: executionPointer(uint64(100) << 30)}, Containers: []resourcebudget.ContainerUsage{}}}
}
func executionFixturePolicy() product.SwarmSchedulerPolicy {
	return product.SwarmSchedulerPolicy{MaxActive: 2, MaxPerProject: 2, HeadroomPercent: 25, MinHeadroomMiB: 2048, DiskReserveMiB: 2048}
}
func executionTestGate(t *testing.T, shared string) *executionAdmission {
	t.Helper()
	env := sessionsNewEnv(t)
	env.api.deps.ExecutionStateDir = shared
	env.api.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	env.api.deps.ResourceSnapshot = executionFixtureSnapshot
	env.api.execution = newExecutionAdmission(env.api)
	return env.api.execution
}
func TestExecutionAdmissionSerializesIndependentControllersBeforeAnyAllocation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "capacity")
	left, right := executionTestGate(t, root), executionTestGate(t, root)
	var wg sync.WaitGroup
	results := make(chan executionDecision, 12)
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g := left
			if i%2 == 0 {
				g = right
			}
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-independent"}
			d, err := g.reserve(context.Background(), string(rune('a'+i)), actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy())
			results <- d
			failures <- err
		}(i)
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	admitted := 0
	for d := range results {
		if d.State == "admitted" {
			admitted++
		} else if d.State != "waiting" {
			t.Fatal("unexpected decision", d)
		}
	}
	if admitted != 2 {
		t.Fatalf("shared durable capacity admitted %d; wanted2", admitted)
	}
}
func TestExecutionAdmissionRechecksExistingLeaseForNewPressure(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	policy := executionFixturePolicy()
	d, err := g.reserve(context.Background(), "queue-one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.State != "admitted" {
		t.Fatal(d, err)
	}
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(2) << 30)
		return s
	}
	d, err = g.reserve(context.Background(), "queue-one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.State != "waiting" || d.Code != "memory_pressure" {
		t.Fatal("old reservation bypassed current pressure", d, err)
	}
	g.api.deps.ResourceSnapshot = executionFixtureSnapshot
	d, err = g.reserve(context.Background(), "queue-one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.State != "admitted" {
		t.Fatal("same reservation could not resume safely", d, err)
	}
	count := 0
	_ = g.mutate(context.Background(), func(l *executionLeaseLedger) error { count = len(l.Leases); return nil })
	if count != 1 {
		t.Fatal("rechecking duplicated lease", count)
	}
}
func TestExecutionAdmissionDoesNotReplayUnconfirmedChildAfterReopen(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	policy := executionFixturePolicy()
	policy.MaxActive = 1
	if d, err := g.reserve(context.Background(), "queue-one", actor.ProjectID, "subscription", 1, actor, management.ExecutionAuthority{}, policy); err != nil || d.State != "admitted" {
		t.Fatal(d, err)
	}
	if err := g.bind(context.Background(), "queue-one", "s-12345678"); err != nil {
		t.Fatal(err)
	}
	reopened := newExecutionAdmission(g.api)
	if err := reopened.release(context.Background(), "queue-one"); !errors.Is(err, ErrExecutionLeaseHeld) {
		t.Fatal("unconfirmed child released capacity", err)
	}
	d, err := reopened.reserve(context.Background(), "queue-two", actor.ProjectID, "subscription", 1, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.State != "waiting" || d.Code != "global_slot_limit" {
		t.Fatal("restart admitted replacement despite unknown child", d, err)
	}
}
func TestExecutionAdmissionHonorsActualPolicyAndMissingTelemetry(t *testing.T) {
	for _, test := range []struct {
		name, code string
		change     func(*resourcebudget.Snapshot)
	}{
		{"unknown effective host CPU", "resource_observation_unavailable", func(s *resourcebudget.Snapshot) { s.Host.CPU.EffectiveMillis = nil }},
		{"stale host disk", "disk_observation_unavailable", func(s *resourcebudget.Snapshot) { s.Host.Disks[0].ObservedAt = time.Now().Add(-time.Minute) }},
		{"stale", "resource_observation_unavailable", func(s *resourcebudget.Snapshot) { s.Host.ObservedAt = time.Now().Add(-time.Minute) }},
		{"incomplete engine inventory", "resource_observation_unavailable", func(s *resourcebudget.Snapshot) { s.Execution.InventoryComplete = false }},
		{"unknown VM disk", "execution_disk_unavailable", func(s *resourcebudget.Snapshot) { s.Execution.DaemonDisk = nil }},
		{"allocation disk demand", "execution_disk_pressure", func(s *resourcebudget.Snapshot) {
			s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(3) << 30)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { s := executionFixtureSnapshot(); test.change(&s); return s }
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
			d, err := g.reserve(context.Background(), "q", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
			if err != nil || d.State != "waiting" || d.Code != test.code {
				t.Fatal(d, err)
			}
		})
	}
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	g.api.deps.Resources = backend.Resources{MemoryMiB: 4096, CPUMillis: 4000}
	actor := management.ExecutionActorRef{UserID: "u-owner", ProjectID: "p-one"}
	authority := management.ExecutionAuthority{Policy: management.Policy{MaxSessionsPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000}}
	d, err := g.reserve(context.Background(), "q-one", actor.ProjectID, "api", 4, actor, authority, executionFixturePolicy())
	if err != nil || d.State != "admitted" || d.Resources.MemoryMiB != 1024 || d.Resources.CPUMillis != 1000 {
		t.Fatal("actual management policy was not retained", d, err)
	}
	d, err = g.reserve(context.Background(), "q-two", actor.ProjectID, "api", 4, actor, authority, executionFixturePolicy())
	if err != nil || d.State != "waiting" || d.Code != "account_session_limit" {
		t.Fatal("management default single-session quota bypassed", d, err)
	}
}

func TestExecutionAdmissionAncillaryWorkloadsConsumeResourcesNotAgentSlots(t *testing.T) {
	for _, bounded := range []bool{true, false} {
		t.Run(map[bool]string{true: "bounded services", false: "unbounded measured services"}[bounded], func(t *testing.T) {
			g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				s := executionFixtureSnapshot()
				for _, kind := range []string{"project-service", "preview", "check", "other"} {
					row := resourcebudget.ContainerUsage{ID: kind, Kind: kind, Owned: kind != "other", ObservedAt: time.Now(), MemoryUsageBytes: executionPointer(uint64(128) << 20), CPUUsageMillis: executionPointer(int64(100))}
					if bounded {
						row.MemoryLimitBytes = executionPointer(uint64(256) << 20)
						row.CPULimitMillis = executionPointer(int64(250))
					}
					s.Execution.Containers = append(s.Execution.Containers, row)
				}
				return s
			}
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
			policy := executionFixturePolicy()
			policy.MaxActive = 1
			policy.MaxPerProject = 1
			d, e := g.reserve(context.Background(), "q", actor.ProjectID, "api", 1, actor, management.ExecutionAuthority{}, policy)
			if e != nil || d.State != "admitted" {
				t.Fatal("ordinary services occupied agent slots", d, e)
			}
			g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				s := executionFixtureSnapshot()
				s.Execution.Containers = []resourcebudget.ContainerUsage{{ID: "other", Kind: "other", ObservedAt: time.Now(), MemoryUsageBytes: executionPointer(uint64(15) << 30), CPUUsageMillis: executionPointer(int64(0))}}
				return s
			}
			d, e = g.reserve(context.Background(), "q", actor.ProjectID, "api", 1, actor, management.ExecutionAuthority{}, policy)
			if e != nil || d.State != "waiting" || d.Code != "memory_pressure" {
				t.Fatal("actual unrelated workload was ignored", d, e)
			}
		})
	}
}
func TestExecutionAdmissionNeverTreatsMissingAncillaryStatsAsZero(t *testing.T) {
	for _, stale := range []bool{false, true} {
		g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
		g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
			s := executionFixtureSnapshot()
			r := resourcebudget.ContainerUsage{ID: "other", Kind: "other", ObservedAt: time.Now()}
			if stale {
				r.MemoryUsageBytes = executionPointer(uint64(1))
				r.CPUUsageMillis = executionPointer(int64(1))
				r.ObservedAt = time.Now().Add(-time.Minute)
			}
			s.Execution.Containers = []resourcebudget.ContainerUsage{r}
			return s
		}
		actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
		d, e := g.reserve(context.Background(), "q", actor.ProjectID, "api", 1, actor, management.ExecutionAuthority{}, executionFixturePolicy())
		if e != nil || d.State != "waiting" || d.Code != "execution_limits_unknown" {
			t.Fatal(d, e)
		}
	}
}
func TestExecutionAdmissionLegacySubscriptionHoldsProviderCapacity(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	if e := g.api.deps.Store.Create(registry.Session{ID: "s-legacy", SandboxID: "s-legacy", ProjectID: "p-old", Harness: "codex", AuthMode: "chatgpt", Status: registry.StatusRunning}); e != nil {
		t.Fatal(e)
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-new"}
	policy := executionFixturePolicy()
	policy.MaxActive = 4
	d, e := g.reserve(context.Background(), "q", actor.ProjectID, "codex-subscription", 1, actor, management.ExecutionAuthority{}, policy)
	if e != nil || d.State != "waiting" || d.Code != "provider_limit" {
		t.Fatal("legacy subscription bypassed singleton provider", d, e)
	}
}
func TestExecutionAdmissionSharedTeamQuotaCannotBeBypassedByAnotherMember(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	policy := executionFixturePolicy()
	authority := management.ExecutionAuthority{Policy: management.Policy{MaxSessionsPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000}}
	actor := management.ExecutionActorRef{UserID: "u-one", TeamID: "team-one", ProjectID: "p-one"}
	if d, e := g.reserve(context.Background(), "q-one", actor.ProjectID, "api", 2, actor, authority, policy); e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	actor.UserID = "u-two"
	d, e := g.reserve(context.Background(), "q-two", actor.ProjectID, "api", 2, actor, authority, policy)
	if e != nil || d.State != "waiting" || d.Code != "account_session_limit" {
		t.Fatal("team worker quota bypassed through a second member", d, e)
	}
}
func TestExecutionAdmissionReplacementRetainsOneExactSessionReservation(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	policy := executionFixturePolicy()
	policy.MaxActive = 1
	policy.MaxPerProject = 1
	actor := management.ExecutionActorRef{UserID: "u-one", ProjectID: "p-one"}
	auth := management.ExecutionAuthority{Policy: management.Policy{MaxSessionsPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000}}
	if d, e := g.reserve(context.Background(), "manual-one", actor.ProjectID, "api", 1, actor, auth, policy); e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	if e := g.bind(context.Background(), "manual-one", "s-one"); e != nil {
		t.Fatal(e)
	}
	prior := registry.Session{ID: "s-one", SandboxID: "s-one", ProjectID: actor.ProjectID, Status: registry.StatusStarting}
	if e := g.api.deps.Store.Create(prior); e != nil {
		t.Fatal(e)
	}
	if e := g.markBackendStarting(context.Background(), "manual-one", prior.ID, prior.SandboxID); e != nil {
		t.Fatal(e)
	}
	if e := g.markBackendSettled(context.Background(), "manual-one", prior.ID, prior.SandboxID); e != nil {
		t.Fatal(e)
	}
	var priorErr error
	prior, priorErr = g.api.deps.Store.Update(prior.ID, func(row *registry.Session) error { row.Status = registry.StatusRunning; return nil })
	if priorErr != nil {
		t.Fatal(priorErr)
	}
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.Containers = []resourcebudget.ContainerUsage{{ID: "owned", SandboxID: "s-one", Owned: true, Kind: "agent", MemoryLimitBytes: executionPointer(uint64(1) << 30), CPULimitMillis: executionPointer(int64(1000))}}
		return s
	}
	d, e := g.reserveReplacement(context.Background(), "restart-one", actor.ProjectID, "api", 1, actor, auth, policy, prior)
	if e != nil || d.State != "admitted" {
		t.Fatal("single-session account cannot replace itself", d, e)
	}
	if e := g.release(context.Background(), "restart-one"); !errors.Is(e, ErrExecutionLeaseHeld) {
		t.Fatal("failed restart released old live capacity", e)
	}
	if e := g.bindSandbox(context.Background(), "restart-one", prior.ID, "s-next"); e != nil {
		t.Fatal(e)
	}
	if e := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 1 || l.Leases[0].sandboxID() != "s-next" {
			t.Fatal("replacement left duplicate or wrong sandbox", l.Leases)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
func TestExecutionAdmissionRefusesLinkedPrivateState(t *testing.T) {
	for _, kind := range []string{"root symlink", "lock hardlink", "ledger symlink"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "capacity")
			g := executionTestGate(t, root)
			outside := filepath.Join(base, "outside")
			if e := os.MkdirAll(outside, 0700); e != nil {
				t.Fatal(e)
			}
			if kind == "root symlink" {
				if e := os.Symlink(outside, root); e != nil {
					t.Skip(e)
				}
			} else {
				if e := os.MkdirAll(root, 0700); e != nil {
					t.Fatal(e)
				}
				victim := filepath.Join(outside, "victim")
				if e := os.WriteFile(victim, []byte("untouched"), 0600); e != nil {
					t.Fatal(e)
				}
				var e error
				if kind == "lock hardlink" {
					e = os.Link(victim, filepath.Join(root, "leases.lock"))
				} else {
					e = os.Symlink(victim, filepath.Join(root, "leases.json"))
				}
				if e != nil {
					t.Skip(e)
				}
			}
			if e := g.mutate(context.Background(), func(l *executionLeaseLedger) error { t.Fatal("linked execution state reached mutation"); return nil }); e == nil {
				t.Fatal("linked state accepted")
			}
		})
	}
}

func TestExecutionAdmissionRejectsCorruptOwnershipAndResourceLedgerWithoutRepair(t *testing.T) {
	for _, kind := range []string{"huge memory", "huge cpu", "scope", "actor", "state", "unknown field", "trailing data"} {
		t.Run(kind, func(t *testing.T) {
			g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
			actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
			if d, e := g.reserve(context.Background(), "q", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); e != nil || d.State != "admitted" {
				t.Fatal(d, e)
			}
			original, e := os.ReadFile(g.path)
			if e != nil {
				t.Fatal(e)
			}
			var l executionLeaseLedger
			if e = json.Unmarshal(original, &l); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "huge memory":
				l.Leases[0].Resources.MemoryMiB = int(^uint(0) >> 1)
			case "huge cpu":
				l.Leases[0].Resources.CPUMillis = int(^uint(0) >> 1)
			case "scope":
				l.Leases[0].Scope = string(bytes.Repeat([]byte("z"), 64))
			case "actor":
				l.Leases[0].Actor.ProjectID = "p-other"
			case "state":
				l.Leases[0].State = "probably-ended"
			}
			malformed, e := json.Marshal(l)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "unknown field" {
				malformed = append([]byte(`{"ignoredAuthority":true,`), malformed[1:]...)
			}
			if kind == "trailing data" {
				malformed = append(malformed, []byte(` {"extra":true}`)...)
			}
			if e = os.WriteFile(g.path, malformed, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = g.reserve(context.Background(), "q-two", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); e == nil {
				t.Fatal("corrupt ledger accepted")
			}
			retained, e := os.ReadFile(g.path)
			if e != nil || !bytes.Equal(retained, malformed) {
				t.Fatal("invalid ownership was silently repaired", e)
			}
		})
	}
}

type executionGoneBackend struct {
	backend.Backend
	checked string
}

func (b *executionGoneBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	b.checked = id
	return backend.Sandbox{}, backend.ErrNotFound
}
func TestExecutionAdmissionReleasesConfirmedSessionBeforeRegistryForget(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-one"}
	if d, e := g.reserve(context.Background(), "q", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy()); e != nil || d.State != "admitted" {
		t.Fatal(d, e)
	}
	if e := g.bindSandbox(context.Background(), "q", "s-one", "s-new-sandbox"); e != nil {
		t.Fatal(e)
	}
	if e := g.api.deps.Store.Create(registry.Session{ID: "s-one", SandboxID: "s-new-sandbox", ProjectID: actor.ProjectID, Status: registry.StatusExited}); e != nil {
		t.Fatal(e)
	}
	gone := &executionGoneBackend{Backend: g.api.deps.Backend}
	g.api.deps.Backend = gone
	if e := g.releaseSession(context.Background(), "s-one"); e != nil {
		t.Fatal(e)
	}
	if gone.checked != "s-new-sandbox" {
		t.Fatal("terminal release inspected the wrong sandbox", gone.checked)
	}
	if e := g.api.deps.Store.Delete("s-one"); e != nil {
		t.Fatal(e)
	}
	if e := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
		if len(l.Leases) != 0 {
			t.Fatal("forgotten registry stranded held lease", l.Leases)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
