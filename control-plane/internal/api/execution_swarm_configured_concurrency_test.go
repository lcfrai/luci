package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Production API, shared admission ledger, scheduler, real Git task checkouts
// and durable FileStore; backend/usage/readiness are controlled fixtures.
// No Docker, microVM, provider inference or performance claim.
func TestConfiguredConcurrencySixAcrossFourProjectsKeepsFairnessAndRecovery(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	projects := []*swarmSessionFixture{f}
	for i := 1; i < 4; i++ {
		projects = append(projects, f.anotherProject(t, fmt.Sprintf("configured-project-%d", i)))
	}
	a := f.env.api
	be := &releaseFairnessBackend{controlledLaunchFencer: &controlledLaunchFencer{Backend: f.env.be, identity: backend.LaunchFenceIdentity{Protocol: 1, EngineID: "configured-concurrency-engine", InstanceID: "11111111-1111-4111-8111-111111111111"}, proofs: map[string]backend.LaunchFenceProof{}}, rows: map[string]backend.Sandbox{}}
	a.deps.Backend, a.deps.ResourceSnapshot = be, be.snapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	policy := product.SwarmSchedulerPolicy{MaxActive: 6, MaxPerProject: 2, HeadroomPercent: 25, MinHeadroomMiB: 2048, DiskReserveMiB: 2048}
	configure := func(paused bool) {
		t.Helper()
		var settings struct {
			Revision int `json:"revision"`
		}
		w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &settings) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		body, _ := json.Marshal(map[string]any{"expectedRevision": settings.Revision, "policy": policy, "dispatchPaused": paused})
		w = f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(body))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	configure(true)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	for _, p := range projects {
		body, _ := json.Marshal(map[string]any{"expectedRevision": p.run.Revision, "idempotencyKey": "configured-six-" + p.owner.ID, "taskIds": []string{"api", "ui", "join"}, "priority": "normal"})
		w := p.env.do(t, "POST", p.base()+"/"+p.run.ID+"/swarm/queue", string(body))
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	configure(false)
	for range 3 {
		if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	configure(true)
	for range 6 {
		sessionsAwaitWatch(t, done)
	}
	rows, err := f.store.List()
	if err != nil || len(rows) != 6 {
		t.Fatalf("six distinct children: %d %v", len(rows), err)
	}
	counts := map[string]int{}
	for _, r := range rows {
		if r.Status != registry.StatusRunning || r.ImplementationContext == nil || len(r.ImplementationContext.TaskIDs) != 1 || r.ImplementationContext.TaskIDs[0] == "join" {
			t.Fatal("wrong task state or dependency launch", r)
		}
		counts[r.ProjectID]++
	}
	if len(counts) != 4 {
		t.Fatal("a project starved before sixth start", counts)
	}
	for _, n := range counts {
		if n > 2 {
			t.Fatal("project ceiling exceeded", counts)
		}
	}
	ledger, err := a.execution.readReservationLedger(context.Background())
	if err != nil || len(ledger.Leases) != 6 {
		t.Fatal("shared reservations differ from six children", ledger, err)
	}
	var totalMemory, totalCPU int
	for _, l := range ledger.Leases {
		totalMemory += l.Resources.MemoryMiB
		totalCPU += l.Resources.CPUMillis
	}
	if totalMemory != 6144 || totalCPU != 6000 {
		t.Fatal("incorrect exact demand", totalMemory, totalCPU)
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: projects[3].owner.ID}
	d, err := a.execution.reserve(context.Background(), "over-cap-probe", actor.ProjectID, "separate-provider", 6, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.Code != "global_slot_limit" {
		t.Fatal("seventh claim bypassed configured cap", d, err)
	}
	var cancelled product.SwarmQueueItem
	var cancelProject string
	for _, p := range projects {
		snap, e := a.deps.Product.ProjectSwarmQueueSnapshot(p.owner.ID)
		if e != nil {
			t.Fatal(e)
		}
		for _, item := range snap.Items {
			if item.State == "queued" && item.TaskID == "ui" && cancelled.ID == "" {
				cancelled = item
				cancelProject = p.owner.ID
			}
		}
	}
	if cancelled.ID == "" {
		t.Fatal("expected unallocated UI task")
	}
	body, _ := json.Marshal(map[string]any{"expectedRevision": cancelled.Revision})
	w := f.env.do(t, "POST", "/v1/product/projects/"+cancelProject+"/swarm-queue/"+cancelled.ID+"/cancel", string(body))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	beforeCreates := len(f.env.be.specs)
	f.reopen(t)
	a = f.env.api
	configure(false)
	for range 2 {
		if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	configure(true)
	if len(f.env.be.specs) != beforeCreates {
		t.Fatal("reopen replayed a worker")
	}
	ledger, err = a.execution.readReservationLedger(context.Background())
	if err != nil || len(ledger.Leases) != 6 {
		t.Fatal("restart lost reservations", ledger, err)
	}
	snap, err := a.deps.Product.ProjectSwarmQueueSnapshot(cancelProject)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range snap.Items {
		if item.ID == cancelled.ID {
			found = true
			if item.State != "cancelled" || item.ChildID != "" {
				t.Fatal("cancelled task started on reopen", item)
			}
		}
	}
	if !found {
		t.Fatal("cancelled history lost")
	}
	// Even with a larger configured ceiling, six 1-CPU workers fill this
	// fixture's 8-CPU daemon minus25% headroom. Configuration cannot bypass it.
	policy.MaxActive = 24
	d, err = a.execution.reserve(context.Background(), "resource-cap-probe", "p-independent", "independent-provider", 24, management.ExecutionActorRef{Local: true, ProjectID: "p-independent"}, management.ExecutionAuthority{}, policy)
	if err != nil || d.Code != "cpu_capacity" {
		t.Fatal("larger configuration bypassed actual resource ceiling", d, err)
	}
	if len(f.env.be.specs) != 6 {
		t.Fatal("configuration performed unexpected allocation", len(f.env.be.specs))
	}
}

func TestConfiguredProviderCeilingAboveSixteenSurvivesLedgerReopen(t *testing.T) {
	g := previewTestGate(t)
	policy := executionFixturePolicy()
	policy.MaxActive = 24
	policy.MaxPerProject = 6
	actor := management.ExecutionActorRef{Local: true, ProjectID: "p-configured"}
	d, err := g.reserve(context.Background(), "configured-provider-claim", actor.ProjectID, "api", 24, actor, management.ExecutionAuthority{}, policy)
	if err != nil || d.State != "admitted" {
		t.Fatal(d, err)
	}
	reopened := newExecutionAdmission(g.api)
	ledger, err := reopened.readReservationLedger(context.Background())
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].ProviderLimit != 24 {
		t.Fatal("configured provider ceiling was not durable", ledger, err)
	}
}
