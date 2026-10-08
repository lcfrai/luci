package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestExecutionFenceLedgerRejectsCorruptionAndForeignOwnership(t *testing.T) {
	f, _, _ := fencedTaskFixture(t)
	req := fencedRecoveryRequest(t, f)
	_, release, err := f.env.api.acquireSwarmFenceRecovery(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	release()
	ledger, err := f.env.api.execution.readFenceLedger()
	if err != nil || len(ledger.Fences) != 1 {
		t.Fatal(ledger, err)
	}
	valid, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.env.api.execution.bindSandbox(context.Background(), ledger.Leases[0].ID, req.ChildID, req.ChildID); err == nil {
		t.Fatal("pending recovery was downgraded to an unstarted reservation")
	}
	afterBind, err := f.env.api.execution.readFenceLedger()
	if err != nil || afterBind.Fences[0] != ledger.Fences[0] || afterBind.Leases[0] != ledger.Leases[0] {
		t.Fatal("refused late bind corrupted the permanent intent or lease", afterBind, err)
	}
	for _, test := range []struct {
		name   string
		change func(*executionLeaseLedger)
	}{
		{"foreign project", func(l *executionLeaseLedger) { l.Fences[0].ProjectID = "foreign-project" }},
		{"foreign engine", func(l *executionLeaseLedger) { l.Leases[0].FenceEngineID = "different-engine" }},
		{"missing intent", func(l *executionLeaseLedger) { l.Fences = nil }},
		{"missing lease", func(l *executionLeaseLedger) { l.Leases = nil }},
		{"false retirement", func(l *executionLeaseLedger) { l.Fences[0].State = "retired" }},
		{"protocol downgrade", func(l *executionLeaseLedger) { l.Leases[0].FenceProtocol = 0 }},
		{"unknown protocol", func(l *executionLeaseLedger) { l.Leases[0].FenceProtocol = 99 }},
		{"intent rebinding", func(l *executionLeaseLedger) { l.Fences[0].Intent.ID = "00000000000000000000000000000000" }},
		{"oversized resource", func(l *executionLeaseLedger) { l.Leases[0].Resources.MemoryMiB = 1 << 60 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var changed executionLeaseLedger
			if err := json.Unmarshal(valid, &changed); err != nil {
				t.Fatal(err)
			}
			test.change(&changed)
			data, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeExecutionLedger(data); err == nil {
				t.Fatal("corrupt private ownership was accepted")
			}
		})
	}
	if _, err := decodeExecutionLedger(append(valid, []byte(" {}")...)); err == nil {
		t.Fatal("trailing private data was accepted")
	}
}

func TestExecutionFenceHostQuotaIncludesOtherScopesBeforeAnyBackendMutation(t *testing.T) {
	f, b, _ := fencedTaskFixture(t)
	g := f.env.api.execution
	req := fencedRecoveryRequest(t, f)
	intent, release, err := f.env.api.acquireSwarmFenceRecovery(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	release()
	initial, err := g.readFenceLedger()
	if err != nil {
		t.Fatal(err)
	}
	baseRecord, baseLease := initial.Fences[0], initial.Leases[0]
	// This account's pending launch is retained without an intent. The other
	// 64 durable slots belong to distinct account scopes on this same host.
	if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
		l.Fences = nil
		l.Leases[0].State, l.Leases[0].FenceIntentID = "backend-dispatching", ""
		for i := 0; i < backend.MaxLaunchFences; i++ {
			record, lease := baseRecord, baseLease
			record.Scope = fixtureFenceHash(fmt.Sprintf("foreign-account-%d", i))
			record.ChildID = fmt.Sprintf("s-foreign-%d", i)
			record.Intent.ID = fmt.Sprintf("%032x", i+1)
			record.Intent.ScopeDigest = record.Scope
			record.Intent.LeaseID = fmt.Sprintf("foreign-lease-%d", i)
			record.Intent.FenceRequest.IntentID = record.Intent.ID
			record.Intent.FenceRequest.ScopeDigest = record.Scope
			record.Intent.FenceRequest.SessionID, record.Intent.FenceRequest.SandboxID = record.ChildID, record.ChildID
			record.Intent.FenceRequest.OperationDigest = product.ProjectSwarmFenceOperationDigest(fenceRecordRequest(record))
			lease.ID, lease.Scope, lease.SessionID, lease.SandboxID, lease.FenceIntentID = record.Intent.LeaseID, record.Scope, record.ChildID, record.ChildID, record.Intent.ID
			l.Fences, l.Leases = append(l.Fences, record), append(l.Leases, lease)
		}
		return validateExecutionFenceRecords(*l)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.readFenceLedger(); err != nil {
		t.Fatal("valid full host ledger did not reload", err)
	}
	_, release, err = f.env.api.acquireSwarmFenceRecovery(context.Background(), req)
	if release != nil {
		release()
	}
	if err != errLaunchFenceLimit {
		t.Fatal("cross-account permanent marker limit was bypassed", err)
	}
	if len(b.requests) != 0 {
		t.Fatal("backend mutated despite full host quota")
	}
	after, err := g.readFenceLedger()
	if err != nil || len(after.Fences) != backend.MaxLaunchFences || after.Leases[0].FenceIntentID != "" {
		t.Fatal("quota refusal changed the original launch", after, err)
	}
	if intent.ID == "" {
		t.Fatal("initial real pending intent missing")
	}
}
