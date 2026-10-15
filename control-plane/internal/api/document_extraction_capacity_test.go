package api

import (
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func documentCapacityFixture() documentExtractionLease {
	now := time.Now().UTC().Add(-10 * time.Minute)
	actor := management.ExecutionActorRef{ProjectID: "project-one", Local: true}
	identity := backend.DocumentExtractionIdentity{Protocol: 1, Profile: backend.DocumentExtractionProfile, EngineID: "engine", InstanceID: "instance", AllocationID: "document-one", ScopeDigest: strings.Repeat("a", 64), ActorDigest: backend.DocumentExtractionActorDigest("", "", "project-one", true), ProjectID: "project-one", DocumentID: "doc-" + strings.Repeat("b", 64), AttemptID: "extract-one", OriginalDigest: strings.Repeat("c", 64), CommandDigest: strings.Repeat("d", 64)}
	return documentExtractionLease{ID: identity.AllocationID, Scope: identity.ScopeDigest, Actor: actor, Identity: identity, ImageID: "sha256:" + strings.Repeat("e", 64), Resources: backend.DocumentExtractionResources(), CreateDiskBytes: 12345, State: "creating", CreatedAt: now, UpdatedAt: now, ComputeDeadline: now.Add(backend.DocumentComputeBudget), WholeDeadline: now.Add(backend.DocumentWholeBudget)}
}
func TestDocumentUnknownExpiredCapacityNeverDisappears(t *testing.T) {
	lease := documentCapacityFixture()
	for _, state := range []string{"reserved", "creating", "created", "starting", "running", "cleaning"} {
		t.Run(state, func(t *testing.T) {
			value := lease
			value.State = state
			if state != "reserved" && state != "creating" {
				value.Ref = strings.Repeat("f", 64)
			}
			if !validDocumentExtractionLease(value) {
				t.Fatal("valid expired ownership refused")
			}
			var demand executionCapacityDemand
			if err := addDocumentCapacity(resourcebudget.Snapshot{Host: resourcebudget.HostSnapshot{Platform: "windows"}}, []documentExtractionLease{value}, "", "", &demand); err != nil {
				t.Fatal(err)
			}
			if demand.PendingMemory != 512<<20 || demand.AllMemory != 512<<20 || demand.CPU != 1000 || demand.PendingDiskBytes != 12345 || demand.PendingHostLaunchBytes != 2<<30 {
				t.Fatal("unknown cleanup/expiry released demand", demand)
			}
			var view executionReservationView
			addDocumentReservationView(&view, []documentExtractionLease{value})
			if view.Count != 1 || view.MemoryMiB != 512 {
				t.Fatal("unknown ownership hidden from totals")
			}
		})
	}
}
func TestDocumentObservedDedupRequiresExactEngineCIDAndPurpose(t *testing.T) {
	lease := documentCapacityFixture()
	lease.State = "running"
	lease.Ref = strings.Repeat("f", 64)
	for _, kind := range []string{"document-extraction", "check", "agent"} {
		snap := resourcebudget.Snapshot{Execution: resourcebudget.ExecutionSnapshot{EngineID: "engine", Containers: []resourcebudget.ContainerUsage{{ID: lease.Ref, Kind: kind}}}}
		var demand executionCapacityDemand
		if err := addDocumentCapacity(snap, []documentExtractionLease{lease}, "", "", &demand); err != nil {
			t.Fatal(err)
		}
		if (demand.PendingMemory == 0) != (kind == "document-extraction") {
			t.Fatal("foreign purpose deduplicated document", kind)
		}
		snap.Execution.EngineID = "foreign"
		demand = executionCapacityDemand{}
		_ = addDocumentCapacity(snap, []documentExtractionLease{lease}, "", "", &demand)
		if demand.PendingMemory == 0 {
			t.Fatal("foreign engine discarded demand")
		}
	}
}
