package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// These are metadata/fake-backend refusal tests, not model or Docker proof.
func TestNativeDelegationDispatchRefusesUnknownImageAndUnsupportedBackend(t *testing.T) {
	for _, name := range []string{"mutable", "changed", "unsupported-backend", "unknown-profile"} {
		t.Run(name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			image := "sha256:" + strings.Repeat("7", 64)
			run := product.ProjectRun{Input: product.ProjectRunInput{Route: product.ProjectRunRoute{Harness: "codex", PermissionMode: "bypass", NativeDelegation: product.NativeDelegationCodexHelpers2, NativeDelegationImage: image}}}
			switch name {
			case "mutable": run.Input.Route.NativeDelegationImage = "luci/sandbox:dev"
			case "changed": image = "sha256:" + strings.Repeat("8", 64)
			case "unsupported-backend": env.be.kind = backend.KindKata
			case "unknown-profile": run.Input.Route.NativeDelegation = "other"
			}
			profile, err := env.api.nativeDelegationDispatchProfile(run, "op-not-allocated", image)
			if err == nil || profile != "" || len(env.be.specs) != 0 || env.api.execution.snapshot.Load() != nil { t.Fatal("unsupported profile acquired execution effects", profile, err) }
		})
	}
}

func TestNativeDelegationDispatchLeavesLegacyTaskPolicyUnchanged(t *testing.T) {
	env := sessionsNewEnv(t)
	profile, err := env.api.nativeDelegationDispatchProfile(product.ProjectRun{}, "", "luci/sandbox:dev")
	if err != nil || profile != "" || len(env.be.specs) != 0 { t.Fatal("legacy task acquired a helper grant", profile, err) }
}

// Reuse the ordinary owned Git/HTTP queue fixture, then create a fresh reviewed
// helper run through real trusted-worker prepare/approval/create handlers. The
// backend is fake: neither the wrapper nor a provider/CLI is executed.
func nativeQueuedFixture(t *testing.T, managed bool) (*swarmSessionFixture, management.ExecutionActorRef, management.Policy) {
	t.Helper()
	f := newSwarmSessionFixtureTasks(t)
	image := "sha256:" + strings.Repeat("7", 64)
	f.options.SandboxImage, f.env.api.deps.Image = image, image
	f.reopen(t)
	selectedModel := f.run.Input.Route.Model
	if managed {
		body, _ := json.Marshal(map[string]any{"action": "save", "expectedRevision": 1, "name": "Synthetic managed native helper", "kind": "codex-subscription", "enabled": true})
		w := f.env.do(t, "POST", "/v1/setup/agent-connections", string(body))
		var catalog struct { Connections []product.AgentConnection }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalog) != nil { t.Fatal("synthetic managed connection", w.Code, w.Body.String()) }
		found := false
		for _, connection := range catalog.Connections {
			if connection.Kind == "codex-subscription" { f.connectionID, found = connection.ID, true }
		}
		if !found { t.Fatal("synthetic managed connection missing") }
		// Opaque fake OAuth bytes in this test-owned private store only. No
		// actual sign-in, provider request or credential inspection occurs.
		if err := os.MkdirAll(f.env.api.deps.ConversationsDir, 0700); err != nil { t.Fatal(err) }
		if err := f.env.api.writeCodexConnection(&codexConnection{ID: "codex-native-helper-fixture", SourceSessionID: "s-stopped-native-helper-fixture", Auth: json.RawMessage(codexFixtureAuth)}); err != nil { t.Fatal(err) }
		selectedModel = "default"
	}
	user := management.User{ID: "native-helper-fixture-owner", Username: "owner", Role: "super-admin"}
	policy := management.Policy{AllowHostCommands: true, MaxSessionsPerUser: 8, MaxConcurrentChecksPerUser: 2, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"codex", "shell"}, BackingMode: "docker", InstanceName: "Native helper fixture"}
	f.env.api.deps.Product.SetCoordinatorAuthorizer(func(ctx context.Context, projectID string) error {
		actor, _, ok := management.ActorFromContext(ctx)
		if ctx.Err() != nil || !ok || actor.Disabled || actor.ID != user.ID || actor.Role != user.Role || projectID != f.owner.ID {
			return product.ErrProjectRunInvalid
		}
		return nil
	})
	f.env.api.deps.AuthorizeExecution = func(_ context.Context, ref management.ExecutionActorRef, _ string, _ bool) (management.ExecutionAuthority, error) {
		if ref.UserID != user.ID || ref.ProjectID != f.owner.ID { t.Error("fixture current authority scope changed", ref) }
		return management.ExecutionAuthority{Actor: user, Policy: policy}, nil
	}
	const token = "native-helper-fixture-trusted-worker-token-at-least-forty-characters"
	request := func(path string, value any, status int) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(value); if err != nil { t.Fatal(err) }
		r := httptest.NewRequest("POST", sessionsTarget(path), strings.NewReader(string(body)))
		r.Header.Set("Content-Type", "application/json"); r.Header.Set("Authorization", "Bearer "+token)
		actorJSON, _ := json.Marshal(user); policyJSON, _ := json.Marshal(policy)
		r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(actorJSON)); r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policyJSON))
		w := httptest.NewRecorder(); management.WorkerHandler(token, f.env.mux).ServeHTTP(w, r)
		if w.Code != status { t.Fatal("native queued fixture", path, w.Code, w.Body.String()) }
		return w
	}
	selection := *product.ImplementationContextSelection(&f.run.Input.Implementation)
	var plan product.ProjectPlan
	w := request("/v1/product/projects/"+f.owner.ID+"/plan", map[string]any{"expectedRevision": selection.PlanRevision, "expectedSpecificationRevision": selection.SpecificationRevision, "tasks": []product.ProjectTask{{ID: "api", Title: "Implement captured API", Role: "coding", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"api-result.txt"}, Acceptance: "The reviewed value is stored and restored."}}}, 200)
	if json.Unmarshal(w.Body.Bytes(), &plan) != nil { t.Fatal("plan response") }
	selection.PlanRevision, selection.TaskIDs = plan.Revision, []string{"api"}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Native helper fixture", "-c", "user.email=native-helper@example.invalid", "commit", "-m", "Capture native helper fixture inputs"}} {
		if _, err := product.RunCommand(context.Background(), "git", args, f.owner.Path); err != nil { t.Fatal(err) }
	}
	prepare := product.OrchestrationPrepareRequest{IdempotencyKey: "native-queue-contract-prepare-123", WorktreeID: f.run.WorktreeID, ImplementationContext: selection, Outcome: "preview", Route: product.OrchestrationRouteRequest{NativeDelegation: product.NativeDelegationCodexHelpers2, Harness: "codex", Model: selectedModel, PermissionMode: "bypass", ExecutionLocation: "local-docker", AgentConnectionID: &f.connectionID}}
	endpoint := "/v1/product/projects/"+f.owner.ID+"/orchestration-contracts"
	var view product.OrchestrationContractView
	if json.Unmarshal(request(endpoint+"/prepare", prepare, 201).Body.Bytes(), &view) != nil { t.Fatal("prepare response") }
	if json.Unmarshal(request(endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": view.Contract.Revision, "expectedDigest": view.Contract.Digest}, 200).Body.Bytes(), &view) != nil || view.Contract.Approval == nil { t.Fatal("approval response") }
	contract := product.OrchestrationContractSelection{ID: view.Contract.ID, Revision: view.Contract.Revision, Digest: view.Contract.Digest, ApprovalID: view.Contract.Approval.ID}
	f.readRun(t, request(f.base(), map[string]any{"idempotencyKey": "native-queue-run-create-123", "nativeDelegation": product.NativeDelegationCodexHelpers2, "worktreeId": prepare.WorktreeID, "harness": "codex", "model": prepare.Route.Model, "permissionMode": "bypass", "agentConnectionId": f.connectionID, "outcome": "preview", "implementationContext": selection, "orchestrationContract": contract}, 201), 201)
	f.readRun(t, request(f.base()+"/"+f.run.ID+"/swarm", map[string]any{"expectedRevision": f.run.Revision, "maxParallel": 1}, 201), 201)
	var reserved struct { Run product.ProjectRun `json:"run"`; Operation product.ProjectRunOperation `json:"operation"` }
	w = request(f.base()+"/"+f.run.ID+"/swarm/tasks/api/reserve", map[string]any{"expectedRevision": f.run.Revision, "idempotencyKey": "native-queue-task-reserve-123"}, 201)
	if json.Unmarshal(w.Body.Bytes(), &reserved) != nil || reserved.Operation.Allocation != "ready" { t.Fatal("reserve response") }
	f.run, f.ops = reserved.Run, []product.ProjectRunOperation{reserved.Operation}
	return f, management.ExecutionActorRef{UserID: user.ID, ProjectID: f.owner.ID}, policy
}

func TestNativeDelegationQueuedDispatchSpecAndFinalCurrentImage(t *testing.T) {
	for _, test := range []struct { name string; managed, changed bool }{{name: "captured-spec"}, {name: "managed-captured-spec", managed: true}, {name: "changed-during-final-authorization", changed: true}} {
		t.Run(test.name, func(t *testing.T) {
			changed := test.changed
			f, actor, policy := nativeQueuedFixture(t, test.managed); a := f.env.api
			if controlledAllocationCount(f.env.be) != 0 { t.Fatal("fixture allocated a backend before dispatch") }
			a.deps.ResourceSnapshot = executionFixtureSnapshot; a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
			calls := 0
			a.deps.AuthorizeExecution = func(_ context.Context, got management.ExecutionActorRef, harness string, secrets bool) (management.ExecutionAuthority, error) {
				if got != actor || harness != "codex" || !secrets { t.Error("current authority scope changed", got, harness, secrets) }
				calls++
				if changed && calls == 3 { a.deps.Image = "sha256:" + strings.Repeat("8", 64) }
				return management.ExecutionAuthority{Actor: management.User{ID: actor.UserID, Role: "super-admin"}, Policy: policy}, nil
			}
			scheduler, err := a.deps.Product.SwarmSchedulerPolicy(); if err != nil { t.Fatal(err) }
			admission, err := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-native-helper-fixture", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: scheduler})
			if err != nil || admission.State != "admitted" { t.Fatal("queued admission", admission, err) }
			if !changed { sessionsStubReady(t, nil) }
			var done <-chan string
			if !changed { done = sessionsWatchDone(t) }
			dispatch := product.SwarmDispatchRequest{QueueID: "queue-native-helper-fixture", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor}
			got, err := a.dispatchQueuedSwarm(context.Background(), dispatch)
			if err != nil { t.Fatal(err) }
			if changed {
				if controlledAllocationCount(f.env.be) != 0 { t.Fatal("late image change reached Backend.Create") }
				if calls < 3 || got.State != "failed" || got.ReasonCode != "admission_unavailable" { t.Fatal("late image change lost normalized admission refusal", calls, got) }
				row, readErr := f.store.Get(got.SessionID)
				if readErr != nil || row.Status != registry.StatusExited || row.StartFailure != registry.NewSessionStartFailure("admission", "native_helper_profile_changed") { t.Fatal("final refusal lost exact owned settlement", row, readErr) }
				replay, replayErr := a.dispatchQueuedSwarm(context.Background(), dispatch)
				if controlledAllocationCount(f.env.be) != 0 { t.Fatal("late refusal replay reached Backend.Create") }
				if replayErr != nil || replay != got { t.Fatal("late refusal replay changed retained result", replay, replayErr) }
			} else {
				sessionsAwaitWatch(t, done)
				if got.State != "running" || controlledAllocationCount(f.env.be) != 1 { t.Fatal("captured task did not reach fake backend", got) }
				f.env.be.mu.Lock(); spec := f.env.be.specs[0]; f.env.be.mu.Unlock()
				prefix := []string{"/usr/local/lib/luci/codex-native-task.cjs", "-c", `cli_auth_credentials_store="file"`, "-c", "agents.enabled=true", "-c", "agents.max_concurrent_threads_per_session=2", "-c", "features.multi_agent_v2=false"}
				if spec.Image != f.run.Input.Route.NativeDelegationImage || spec.Command != "/usr/local/bin/node" || len(spec.Args) < len(prefix)+4 || !slices.Equal(spec.Args[:len(prefix)], prefix) || slices.Contains(spec.Args, "agents.enabled=false") { t.Fatal("actual Spec lost captured profile/image", spec.Image, spec.Command, spec.Args) }
				authArgs := spec.Args[len(prefix):len(prefix)+4]
				if test.managed {
					if !slices.Equal(authArgs, []string{"-c", `forced_login_method="chatgpt"`, "-c", `model_provider="openai"`}) || spec.Env["OPENAI_API_KEY"] != "" || spec.Env["CODEX_API_KEY"] != "" { t.Fatal("managed auth decoration crossed wrapper/policy boundary") }
				} else {
					if !slices.Equal(authArgs, []string{"-c", `model_provider="luci-openai"`, "-c", `model_providers.luci-openai={name="Luci selected API",base_url="http://host.docker.internal:4000/v1",env_key="OPENAI_API_KEY",wire_api="responses",supports_websockets=false}`}) { t.Fatal("API auth decoration crossed wrapper/policy boundary", authArgs) }
				}
			}
		})
	}
}
