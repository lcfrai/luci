package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// These tests exercise the typed allocator against reviewed Git/context inputs.
// The backend is controlled; they do not claim provider/model or Docker proof.
func TestQueuedDispatchRechecksFreshAuthorityAfterDurableClaim(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	calls := 0
	a.deps.AuthorizeExecution = func(ctx context.Context, got management.ExecutionActorRef, harness string, secrets bool) (management.ExecutionAuthority, error) {
		if got != actor || harness != "codex" || !secrets {
			t.Error("queued identity/route authority changed", got, harness, secrets)
		}
		calls++
		if calls >= 3 {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{}, nil
	}
	policy, e := a.deps.Product.SwarmSchedulerPolicy()
	if e != nil {
		t.Fatal(e)
	}
	admission, e := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-authority", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
	if e != nil || admission.State != "admitted" {
		t.Fatal(admission, e)
	}
	got, e := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-authority", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
	if e != nil || got.State != "failed" || got.ReasonCode != "execution_authority_changed" || got.SessionID == "" {
		t.Fatal("pre-allocation refusal lost its exact diagnostic", got, e)
	}
	if controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("revoked authority allocated a backend")
	}
	row, e := f.store.Get(got.SessionID)
	if e != nil || row.Status != registry.StatusExited || row.StartFailure != registry.NewSessionStartFailure("admission", "execution_authority_changed") {
		t.Fatal("failed final authorization left live registry state", row, e)
	}
	// A later retry must not recreate the exact claimed operation.
	calls = 0
	repeated, e := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-authority", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
	if e != nil || repeated != got {
		t.Fatal("duplicate dispatch changed the recorded failure", repeated, e)
	}
	if controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("failed claimed operation replayed a backend")
	}
}

func TestQueuedAdmissionRefusesRevokedActorBeforeCapacityOrAllocation(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	a := f.env.api
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, errors.New("controlled-secret/private/authority")
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	policy, e := a.deps.Product.SwarmSchedulerPolicy()
	if e != nil {
		t.Fatal(e)
	}
	got, e := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-revoked", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
	if e != nil || got.State != "blocked" || got.ReasonCode != "access_changed" || strings.Contains(got.Message, "controlled-secret") || strings.Contains(got.Message, "/private/") {
		t.Fatal(got, e)
	}
	if a.execution.snapshot.Load() != nil || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("revoked queue actor acquired capacity or allocated backend")
	}
}

func controlledAllocationCount(b *sessionsBackend) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.specs)
}

func TestQueuedOpenCodePreparesExactSavedRouteBeforeClaimOrAllocation(t *testing.T) {
	for _, local := range []bool{true, false} {
		t.Run(map[bool]string{true: "named local refuses unverified engine", false: "named remote retains captured context"}[local], func(t *testing.T) {
			f := newSwarmSessionFixtureRoute(t, "opencode", func(env *sessionsEnv) string {
				endpoint := "http://127.0.0.1:4000/v1"
				if local {
					endpoint = "http://127.0.0.1:11434/v1"
				}
				body, err := json.Marshal(map[string]any{"action": "save", "expectedRevision": 0, "name": "Controlled named model", "kind": "compatible-api", "enabled": true,
					"models": product.AgentConnectionModels{OpenCode: "exact-model:latest"}, "endpoints": product.AgentConnectionEndpoints{ChatCompletions: endpoint}})
				if err != nil {
					t.Fatal(err)
				}
				w := env.do(t, "POST", "/v1/setup/agent-connections", string(body))
				var catalogue struct {
					Connections []product.AgentConnection `json:"connections"`
				}
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalogue) != nil || len(catalogue.Connections) != 1 {
					t.Fatal("named fixture connection was not saved", w.Code)
				}
				return catalogue.Connections[0].ID
			}, "api")
			a := f.env.api
			a.deps.ResourceSnapshot = executionFixtureSnapshot
			a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
			actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
			policy, err := a.deps.Product.SwarmSchedulerPolicy()
			if err != nil {
				t.Fatal(err)
			}
			queueID := "queue-named-opencode"
			admitted, err := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: queueID, ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
			if err != nil || admitted.State != "admitted" {
				t.Fatal(admitted, err)
			}
			// This controlled backend deliberately has no LaunchFencer. A local
			// route must refuse before any real host model request or child claim.
			var done <-chan string
			if !local {
				sessionsStubReady(t, nil)
				done = sessionsWatchDone(t)
			}
			got, err := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: queueID, ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
			if err != nil {
				t.Fatal(err)
			}
			if local {
				rows, readErr := f.store.List()
				if got.State != "blocked" || got.ReasonCode != "provider_unavailable" || got.SessionID != "" || readErr != nil || len(rows) != 0 || controlledAllocationCount(f.env.be) != 0 {
					t.Fatal("local preflight crossed durable admission or Backend.Create", got, readErr)
				}
				current, readErr := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
				if readErr != nil || current.Revision != f.run.Revision || current.Input.Route.Connection != f.run.Input.Route.Connection {
					t.Fatal("refusal changed the captured run or connection", readErr)
				}
				return
			}
			sessionsAwaitWatch(t, done)
			if got.State != "running" || got.SessionID != f.ops[0].ChildID || controlledAllocationCount(f.env.be) != 1 {
				t.Fatal("named remote route no longer dispatches", got)
			}
			spec := f.env.be.spec(t, 0)
			var config struct {
				Instructions []string `json:"instructions"`
				Model        string `json:"model"`
				Agent        map[string]any `json:"agent"`
			}
			if json.Unmarshal([]byte(spec.Env["OPENCODE_CONFIG_CONTENT"]), &config) != nil || len(config.Instructions) != 0 || config.Model != "luci-gateway/exact-model:latest" || config.Agent["build"] != nil {
				t.Fatal("queued remote route lost cleared instructions or gained local policy")
			}
			row, readErr := f.store.Get(got.SessionID)
			if readErr != nil || row.ImplementationContext == nil || row.ReviewedContext == nil || row.OpeningPrompt != product.ImplementationOpeningPrompt(row.ImplementationContext) || row.PermissionMode != "bypass" || !strings.Contains(strings.Join(spec.Args, "\n"), row.OpeningPrompt) {
				t.Fatal("queued OpenCode lost captured task or explicit permissions", readErr)
			}
		})
	}
}
