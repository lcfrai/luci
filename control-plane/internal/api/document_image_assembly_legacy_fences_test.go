package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Valid798-era mixed history: document quota filled first, then one independently
// counted check fence. No daemon/process or fabricated real-world ACK is used.
func assemblyLegacyMixedFenceLedger(g *executionAdmission) (executionLeaseLedger, checkExecutionFenceRecord) {
	now := time.Now().UTC()
	l := executionLeaseLedger{Version: 1}
	for i := 0; i < backend.MaxLaunchFences; i++ {
		lease := documentCapacityFixture()
		lease.Scope = g.scope
		lease.Identity.ScopeDigest = g.scope
		lease.ID = fmt.Sprintf("document-history-%d", i)
		lease.Identity.AllocationID = lease.ID
		lease.Identity.InstanceID = httpCheckInstance
		intent := fmt.Sprintf("%032x", i+1)
		proof := backend.DocumentExtractionFenceProof{Identity: lease.Identity, IntentID: intent, FenceRef: fmt.Sprintf("%064x", i+1), ConfigDigest: strings.Repeat("a", 64), AcknowledgedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), ObservedAt: now.Add(-30 * time.Second).Format(time.RFC3339Nano)}
		l.DocumentFences = append(l.DocumentFences, documentExtractionFenceRecord{Scope: g.scope, Identity: lease.Identity, Actor: lease.Actor, IntentID: intent, State: "retired", Proof: proof, CreatedAt: lease.CreatedAt, UpdatedAt: now})
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: "project-one"}
	identity := backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: "check-legacy", CheckID: "check-legacy", ScopeDigest: g.scope, ProjectID: actor.ProjectID, CommandDigest: strings.Repeat("b", 64)}
	lease := checkExecutionLease{ID: identity.AllocationID, Scope: g.scope, Actor: actor, Identity: identity, Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}, CreateDiskBytes: 1, State: "fencing", CreatedAt: now.Add(-time.Minute), UpdatedAt: now}
	record := checkExecutionFenceRecord{Scope: g.scope, Identity: identity, Actor: actor, IntentID: strings.Repeat("e", 32), State: "intent", CreatedAt: lease.CreatedAt, UpdatedAt: now}
	l.Checks = []checkExecutionLease{lease}
	l.CheckFences = []checkExecutionFenceRecord{record}
	return l, record
}

func TestAssemblyLegacyMixedFenceHistoryRemainsReadableAndExistingCleanupSettles(t *testing.T) {
	g, _, _, _ := assemblySharedFixture(t)
	l, record := assemblyLegacyMixedFenceLedger(g)
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeExecutionLedger(data)
	if err != nil || executionFenceCount(decoded) != backend.MaxLaunchFences+1 {
		t.Fatal("valid legacy mixed fence history became unreadable", err)
	}
	if err = g.mutate(context.Background(), func(current *executionLeaseLedger) error { *current = l; return nil }); err != nil {
		t.Fatal(err)
	}
	read, err := g.readReservationLedger(context.Background())
	if err != nil || executionFenceCount(read) != backend.MaxLaunchFences+1 {
		t.Fatal("legacy reservation read refused", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	proof := backend.CheckFenceProof{Identity: record.Identity, IntentID: record.IntentID, FenceRef: strings.Repeat("f", 64), ConfigDigest: strings.Repeat("d", 64), AcknowledgedAt: now, ObservedAt: now}
	if err = g.publishCheckFence(context.Background(), record, proof); err != nil {
		t.Fatal("existing intent could not publish its typed mock ACK", err)
	}
	if err = g.completeFencedCheck(context.Background(), record, proof, product.ProjectCheckCommandOutcome{AllocationID: record.Identity.AllocationID, CleanupConfirmed: true, Status: "failed", Stage: "recovery"}); err != nil {
		t.Fatal("existing exact cleanup could not settle", err)
	}
	read, err = g.readReservationLedger(context.Background())
	if err != nil || executionFenceCount(read) != backend.MaxLaunchFences+1 || read.CheckFences[0].State != "retired" || read.Checks[0].State != "finished" || len(read.DocumentFences) != backend.MaxLaunchFences {
		t.Fatal("cleanup pruned historical identities or failed settlement", read, err)
	}
}

func TestAssemblyLegacyMixedHistoryRefusesNewFenceIntentsAcrossPurposes(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	l, old := assemblyLegacyMixedFenceLedger(g)
	check := l.Checks[0]
	check.ID = "check-new"
	check.Identity.AllocationID = check.ID
	check.Identity.CheckID = check.ID
	check.State = "creating"
	l.Checks = append(l.Checks, check)
	document := documentCapacityFixture()
	document.Scope = g.scope
	document.Identity.ScopeDigest = g.scope
	document.ID = "document-new"
	document.Identity.AllocationID = document.ID
	document.Identity.InstanceID = httpCheckInstance
	l.Documents = append(l.Documents, document)
	p, _ := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	assembly := documentImageAssemblyLease{ID: p.Spec.Identity.AllocationID, Scope: g.scope, Actor: req.Actor, Identity: p.Spec.Identity, Resources: backend.DocumentImageAssemblyResources(), BaseImageID: p.Spec.BaseImageID, CreateDiskBytes: 1, CaptureDiskBytes: backend.DocumentImageAssemblyOutputMaxBytes, State: "creating", CreatedAt: req.CreatedAt, UpdatedAt: req.CreatedAt, ComputeDeadline: req.ComputeDeadline, WholeDeadline: req.WholeDeadline}
	l.ImageAssemblies = append(l.ImageAssemblies, assembly)
	if err := g.mutate(context.Background(), func(current *executionLeaseLedger) error { *current = l; return nil }); err != nil {
		t.Fatal("valid legacy mixed baseline rejected", err)
	}
	if _, err := g.reserveCheckFence(context.Background(), check, check.Actor); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatal("check did not enforce combined new-fence quota", err)
	}
	if _, err := g.reserveDocumentFence(context.Background(), document); err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatal("document did not enforce combined new-fence quota", err)
	}
	if err := h.HoldFence(context.Background(), backend.DocumentImageAssemblyFenceRequest{Identity: assembly.Identity, IntentID: strings.Repeat("a", 32)}); err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatal("assembler did not enforce combined new-fence quota", err)
	}
	// Reusing the exact preexisting intent is settlement, not a new allocation.
	if same, err := g.reserveCheckFence(context.Background(), l.Checks[0], old.Actor); err != nil || same.IntentID != old.IntentID {
		t.Fatal("current quota stranded an existing cleanup intent", same, err)
	}
	read, err := g.readReservationLedger(context.Background())
	if err != nil || executionFenceCount(read) != backend.MaxLaunchFences+1 || len(read.ImageAssemblyFences) != 0 || read.ImageAssemblies[0].State != "creating" {
		t.Fatal("new intent slipped through or historical ownership changed", read, err)
	}
}
