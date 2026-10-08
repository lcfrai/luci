package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSwarmCleanupPendingRequiresOriginalStoppedOwnedAllocation(t *testing.T) {
	identity := backend.LaunchFenceIdentity{Protocol: backend.LaunchFenceProtocol, EngineID: "controlled-cleanup-engine", InstanceID: "11111111-1111-4111-8111-111111111111"}
	var probe *controlledLaunchFencer
	f, row, baseline := settledTaskFixture(t, func(f *swarmSessionFixture) {
		probe = &controlledLaunchFencer{Backend: f.env.be, identity: identity}
		f.env.api.deps.Backend = probe
	})
	a := f.env.api
	row.Status = registry.StatusExited
	for _, scenario := range []string{"exact stopped", "running", "starting", "unknown", "foreign id", "foreign ref", "missing ref", "foreign kind", "foreign instance", "missing managed label", "foreign session label", "foreign sandbox label", "foreign instance label", "missing row ref", "changed request", "legacy receipt", "unacknowledged create", "different engine", "different engine instance", "identity unavailable", "lookup unavailable", "ambiguous absent", "canceled", "corrupt ledger"} {
		t.Run(scenario, func(t *testing.T) {
			encoded, err := json.Marshal(baseline)
			if err != nil {
				t.Fatal(err)
			}
			var ledger executionLeaseLedger
			if err = json.Unmarshal(encoded, &ledger); err != nil {
				t.Fatal(err)
			}
			current := row.Clone()
			observed := backend.Sandbox{ID: row.SandboxID, Ref: row.SandboxRef, Kind: backend.KindDocker, State: backend.StateStopped, Instance: identity.InstanceID,
				Labels: map[string]string{backend.LabelManaged: "true", backend.LabelInstance: identity.InstanceID, backend.LabelSandboxID: row.SandboxID, backend.LabelSessionID: row.ID}}
			probe.mu.Lock()
			probe.identity, probe.identityErr = identity, nil
			probe.mu.Unlock()
			var lookupErr error
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "running":
				observed.State = backend.StateRunning
			case "starting":
				observed.State = backend.StateStarting
			case "unknown":
				observed.State = ""
			case "foreign id":
				observed.ID += "-other"
			case "foreign ref":
				observed.Ref += "-other"
			case "missing ref":
				observed.Ref = ""
			case "foreign kind":
				observed.Kind = "other"
			case "foreign instance":
				observed.Instance += "-other"
			case "missing managed label":
				delete(observed.Labels, backend.LabelManaged)
			case "foreign session label":
				observed.Labels[backend.LabelSessionID] += "-other"
			case "foreign sandbox label":
				observed.Labels[backend.LabelSandboxID] += "-other"
			case "foreign instance label":
				observed.Labels[backend.LabelInstance] += "-other"
			case "missing row ref":
				current.SandboxRef = ""
			case "changed request":
				current.Model += "-other"
			case "legacy receipt":
				ledger.Leases[0].FenceProtocol, ledger.Leases[0].FenceEngineID, ledger.Leases[0].FenceInstanceID = 0, "", ""
			case "unacknowledged create":
				ledger.Leases[0].State = "backend-dispatching"
			case "different engine":
				probe.identity.EngineID += "-other"
			case "different engine instance":
				probe.identity.InstanceID = "22222222-2222-4222-8222-222222222222"
			case "identity unavailable":
				probe.identityErr = backend.ErrUnavailable
			case "lookup unavailable":
				lookupErr = backend.ErrUnavailable
			case "ambiguous absent":
				lookupErr = errors.Join(backend.ErrNotFound, backend.ErrUnavailable)
			case "canceled":
				cancel()
			}
			if _, err := f.store.Update(row.ID, func(v *registry.Session) error { *v = current; return nil }); err != nil {
				t.Fatal(err)
			}
			encoded, err = json.Marshal(ledger)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "corrupt ledger" {
				encoded = []byte("{broken")
			}
			if err = writeExecutionLedger(a.execution.path, encoded); err != nil {
				t.Fatal(err)
			}
			lookups := 0
			probe.Backend = swarmObserverBackend{Backend: f.env.be, get: func(_ context.Context, id string) (backend.Sandbox, error) {
				lookups++
				if id != row.SandboxID {
					t.Fatal("lookup escaped exact retained child")
				}
				return observed, lookupErr
			}}
			err = a.execution.release(ctx, ledger.Leases[0].ID)
			if err == nil {
				t.Fatal("retained or unverified allocation was released")
			}
			// Only the positive original stopped-object proof receives a typed
			// wrapper. All other held outcomes preserve the broad global stop.
			local := errors.Unwrap(err) == ErrExecutionLeaseHeld
			if local != (scenario == "exact stopped") {
				t.Fatalf("local=%v for %s: %v", local, scenario, err)
			}
			if scenario == "exact stopped" && (!errors.Is(err, ErrExecutionLeaseHeld) || lookups != 1) {
				t.Fatal("local marker changed capacity semantics or repeated lookup", err, lookups)
			}
			if scenario == "corrupt ledger" {
				got, e := os.ReadFile(a.execution.path)
				if e != nil || string(got) != string(encoded) {
					t.Fatal("corrupt shared ledger was rewritten", e)
				}
			} else {
				after, e := a.execution.readFenceLedger()
				if e != nil || !reflect.DeepEqual(after, ledger) {
					t.Fatal("classification changed retained private capacity or proof", e)
				}
			}
			if len(probe.requests) != 0 || probe.verifyCalls != 0 {
				t.Fatal("cleanup classification performed recovery effects")
			}
		})
	}
}
