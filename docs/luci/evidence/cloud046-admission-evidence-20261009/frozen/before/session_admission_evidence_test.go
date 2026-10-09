package api

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// The first admission succeeds. Recording the exact child changes the external
// sample before final admission, without changing policy or bypassing dispatch.
func TestAdmissionEvidenceRetainsFinalSampleAcrossReopen(t *testing.T) {
	for _, kind := range []string{"cpu", "host-memory", "execution-memory"} {
		t.Run(kind, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t, "api")
			a := f.env.api
			a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{})
			if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
			a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
				return management.ExecutionAuthority{}, nil
			}
			var created atomic.Bool
			var initialReads, finalReads atomic.Int64
			a.deps.Store = &queuedStartFailureStore{FileStore: f.store, afterCreate: func() { created.Store(true) }}
			initial, final := executionFixtureSnapshot(), executionFixtureSnapshot()
			sampleAt := time.Now().Add(-100 * time.Millisecond).UTC()
			final.Host.ObservedAt, final.Execution.ObservedAt = sampleAt, sampleAt
			code, metric := "memory_pressure", "host_available_memory"
			observed, required := float64(uint64(512)<<20), float64(uint64(5)<<30)
			switch kind {
			case "cpu":
				code, metric = "cpu_pressure", "host_cpu_utilization_percent"
				final.Host.CPU.UtilizationPercent = executionPointer(81.25)
				observed, required = 81.25, 75
			case "host-memory":
				final.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(512) << 20)
			case "execution-memory":
				metric = "execution_total_memory"
				final.Execution.TotalMemoryBytes = executionPointer(uint64(2) << 30)
				observed, required = float64(uint64(2)<<30), float64(uint64(3)<<30)
			}
			a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				if created.Load() {
					finalReads.Add(1)
					return final
				}
				initialReads.Add(1)
				return initial
			}
			actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
			policy, err := a.deps.Product.SwarmSchedulerPolicy()
			if err != nil {
				t.Fatal(err)
			}
			admitted, err := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-evidence", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
			if err != nil || admitted.State != "admitted" {
				t.Fatal("initial capacity did not admit", admitted, err)
			}
			result, err := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-evidence", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
			if err != nil || result.State != "failed" || result.ReasonCode != code || controlledAllocationCount(f.env.be) != 0 || initialReads.Load() == 0 || finalReads.Load() == 0 {
				t.Fatal("final refusal did not retain a never-allocated attempt", result, err)
			}
			check := func(store registry.Store) string {
				t.Helper()
				row, err := store.Get(result.SessionID)
				if err != nil || row.Status != registry.StatusExited || row.StartFailure != registry.NewSessionStartFailure("admission", code) {
					t.Fatal("canonical failure changed", err, row.StartFailure)
				}
				raw, err := json.Marshal(row)
				if err != nil {
					t.Fatal(err)
				}
				var stored map[string]json.RawMessage
				if err := json.Unmarshal(raw, &stored); err != nil {
					t.Fatal(err)
				}
				var evidence map[string]any
				if len(stored["startAdmissionEvidence"]) == 0 || json.Unmarshal(stored["startAdmissionEvidence"], &evidence) != nil {
					t.Fatal("actual final admission operands were not retained")
				}
				observedKey, requiredKey := "observedBytes", "requiredBytes"
				if kind == "cpu" {
					observedKey, requiredKey = "observedPercent", "maximumPercent"
				}
				if evidence["metric"] != metric || evidence[observedKey] != observed || evidence[requiredKey] != required || evidence["observedAt"] != sampleAt.Format(time.RFC3339Nano) {
					t.Fatal("evidence was resampled or did not capture the actual refusal", evidence)
				}
				decided, err := time.Parse(time.RFC3339Nano, evidence["decidedAt"].(string))
				if err != nil || decided.Before(sampleAt) || decided.Sub(sampleAt) > 5*time.Second {
					t.Fatal("decision time does not bind the accepted sample", evidence)
				}
				return string(stored["startAdmissionEvidence"])
			}
			before := check(f.store)
			reopened, err := registry.NewFileStore(f.storePath)
			if err != nil || check(reopened) != before {
				t.Fatal("reopening changed the actual decision evidence", err)
			}
		})
	}
}
