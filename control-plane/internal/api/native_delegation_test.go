package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
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
func nativeQueuedFixture(t *testing.T) (*swarmSessionFixture, management.ExecutionActorRef, management.Policy) {
	t.Helper()
	f := newSwarmSessionFixtureTasks(t)
	image := "sha256:" + strings.Repeat("7", 64)
	f.options.SandboxImage, f.env.api.deps.Image = image, image
	f.reopen(t)
	user := management.User{ID: "native-helper-fixture-owner", Username: "owner", Role: "super-admin"}
	policy := management.Policy{AllowHostCommands: true, MaxSessionsPerUser: 8, MaxConcurrentChecksPerUser: 2, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"codex", "shell"}, BackingMode: "docker", InstanceName: "Native helper fixture"}
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
	prepare := product.OrchestrationPrepareRequest{IdempotencyKey: "native-queue-contract-prepare-123", WorktreeID: f.run.WorktreeID, ImplementationContext: selection, Outcome: "preview", Route: product.OrchestrationRouteRequest{NativeDelegation: product.NativeDelegationCodexHelpers2, Harness: "codex", Model: f.run.Input.Route.Model, PermissionMode: "bypass", ExecutionLocation: "local-docker", AgentConnectionID: &f.connectionID}}
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
	for _, changed := range []bool{false, true} {
		name := "captured-spec"; if changed { name = "changed-during-final-authorization" }
		t.Run(name, func(t *testing.T) {
			f, actor, policy := nativeQueuedFixture(t); a := f.env.api
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
				if calls < 3 || got.State != "failed" || got.ReasonCode != "native_helper_profile_changed" || controlledAllocationCount(f.env.be) != 0 { t.Fatal("late image change reached backend", calls, got) }
				row, readErr := f.store.Get(got.SessionID)
				if readErr != nil || row.Status != registry.StatusExited || row.StartFailure != registry.NewSessionStartFailure("admission", "native_helper_profile_changed") { t.Fatal("final refusal lost exact owned settlement", row, readErr) }
				replay, replayErr := a.dispatchQueuedSwarm(context.Background(), dispatch)
				if replayErr != nil || replay != got || controlledAllocationCount(f.env.be) != 0 { t.Fatal("late refusal replay escaped reservation", replay, replayErr) }
			} else {
				sessionsAwaitWatch(t, done)
				if got.State != "running" || controlledAllocationCount(f.env.be) != 1 { t.Fatal("captured task did not reach fake backend", got) }
				f.env.be.mu.Lock(); spec := f.env.be.specs[0]; f.env.be.mu.Unlock()
				if spec.Image != f.run.Input.Route.NativeDelegationImage || spec.Command != "/usr/local/bin/node" || len(spec.Args) == 0 || spec.Args[0] != "/usr/local/lib/luci/codex-native-task.cjs" || !slices.Contains(spec.Args, "agents.enabled=true") || !slices.Contains(spec.Args, "agents.max_concurrent_threads_per_session=2") || slices.Contains(spec.Args, "agents.enabled=false") { t.Fatal("actual Spec lost captured profile/image", spec.Image, spec.Command, spec.Args) }
			}
		})
	}
}
