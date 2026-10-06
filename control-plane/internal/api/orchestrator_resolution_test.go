package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func orchestratorResolutionRequest(t *testing.T, e *sessionsEnv, id string, body any, resolve bool) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	suffix := "/assist/orchestrator"
	if resolve {
		suffix = "/orchestrator/resolve"
	}
	r := httptest.NewRequest(http.MethodPost, sessionsTarget("/v1/product/projects/"+id+suffix), bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	return r.WithContext(orchestratorExecutionActor(t))
}

func resolveOrchestratorTest(t *testing.T, e *sessionsEnv, id string, body any) orchestratorResolutionResponse {
	t.Helper()
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, orchestratorResolutionRequest(t, e, id, body, true))
	var resolved orchestratorResolutionResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &resolved) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if resolved.ProjectID != id || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("resolution identity or cache policy lost", resolved)
	}
	return resolved
}

func TestOrchestratorResolutionFencesDelayedOriginalWithoutProviderAccess(t *testing.T) {
	e, p := repositoryFixture(t)
	e.be.mu.Lock()
	baseline := len(e.be.specs)
	e.be.mu.Unlock()
	request := orchestratorRequestFor(t, e, p.ID, "Continue our project", "never-sent-resolution", "", product.AgentConnection{ID: "missing-provider"})
	original := orchestratorResolutionRequest(t, e, p.ID, request, false)
	ready, release := make(chan struct{}), make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { close(ready); <-release; w := httptest.NewRecorder(); e.mux.ServeHTTP(w, original); done <- w }()
	<-ready
	// Recovery does not consult this deliberately missing provider route.
	resolved := resolveOrchestratorTest(t, e, p.ID, request)
	if resolved.Status != "not-sent" || resolved.Job != nil || resolved.TurnID != "" || resolved.JobID != "" {
		t.Fatal(resolved)
	}
	close(release)
	select {
	case response := <-done:
		if code := sessionsProblemCode(t, response, 409); code != "orchestrator_request_closed" {
			t.Fatal(code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("late original did not finish")
	}
	if repeated := resolveOrchestratorTest(t, e, p.ID, request); repeated.Status != "not-sent" {
		t.Fatal(repeated)
	}
	jobs, err := e.api.projectAssistance(p.ID)
	if err != nil || len(jobs) != 0 {
		t.Fatal("fence created an assistant job", jobs, err)
	}
	e.be.mu.Lock()
	allocations := len(e.be.specs)
	e.be.mu.Unlock()
	if allocations != baseline {
		t.Fatal("request recovery dispatched a provider")
	}
}

func TestOrchestratorResolutionMissingOrCorruptJobRemainsUnknownWithoutRepair(t *testing.T) {
	e, p := repositoryFixture(t)
	e.be.mu.Lock()
	baseline := len(e.be.specs)
	e.be.mu.Unlock()
	body := orchestratorRequestFor(t, e, p.ID, "Review this work", "reserved-resolution", "", product.AgentConnection{ID: "missing-provider"})
	data, _ := json.Marshal(body)
	var req orchestratorRequest
	_ = json.Unmarshal(data, &req)
	turn, snapshot, _, err := e.api.deps.Product.ReserveOrchestratorTurn(orchestratorExecutionActor(t), p.ID, orchestratorTurnSelection(req), "assist-resolution-original")
	if err != nil {
		t.Fatal(err)
	}
	resolved := resolveOrchestratorTest(t, e, p.ID, body)
	if resolved.Status != "unknown" || resolved.Job != nil || resolved.JobID != turn.JobID || resolved.TurnID != turn.ID {
		t.Fatal(resolved)
	}
	if _, err = e.api.assistanceStore().Read(turn.JobID); !os.IsNotExist(err) {
		t.Fatal("missing job was repaired", err)
	}
	contextBytes, _ := json.Marshal(snapshot)
	job := assistance.Job{ID: turn.JobID, ProjectID: p.ID, Kind: "orchestrator", OrchestratorTurnID: turn.ID, OrchestratorContext: contextBytes, Status: "queued", CreatedAt: turn.CreatedAt}
	if err = e.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	resolved = resolveOrchestratorTest(t, e, p.ID, body)
	if resolved.Status != "accepted" || resolved.Job == nil || resolved.Job.ID != job.ID || resolved.Job.Status != "queued" {
		t.Fatal(resolved)
	}
	// Identical key with altered route/message/context must never return the job.
	for _, field := range []string{"message", "model", "harness", "authMode", "agentConnectionId", "replyToTurnId"} {
		changed := map[string]any{}
		for key, value := range body {
			changed[key] = value
		}
		changed[field] = "different-selection"
		if result := resolveOrchestratorTest(t, e, p.ID, changed); result.Status != "unknown" || result.Job != nil {
			t.Fatal("altered request acquired receipt", field, result)
		}
	}
	_, err = e.api.assistanceStore().Update(job.ID, func(current *assistance.Job) error {
		current.OrchestratorContext = json.RawMessage(`{"projectId":"foreign"}`)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved = resolveOrchestratorTest(t, e, p.ID, body)
	if resolved.Status != "unknown" || resolved.Job != nil {
		t.Fatal("foreign context job accepted", resolved)
	}
	retained, err := e.api.deps.Product.OrchestratorTurn(p.ID, turn.ID)
	if err != nil || retained.Status != "reserved" {
		t.Fatal("resolution repaired original turn", retained, err)
	}
	e.be.mu.Lock()
	allocations := len(e.be.specs)
	e.be.mu.Unlock()
	if allocations != baseline {
		t.Fatal("receipt resolution dispatched provider")
	}
}

func TestOrchestratorResolutionWaitsForAtomicAdmissionBeforeReadingJob(t *testing.T) {
	e, p := repositoryFixture(t)
	body := orchestratorRequestFor(t, e, p.ID, "Review current work", "inflight-resolution", "", product.AgentConnection{ID: "missing-provider"})
	data, _ := json.Marshal(body)
	var req orchestratorRequest
	_ = json.Unmarshal(data, &req)
	e.api.orchestratorAdmissionMu.Lock()
	turn, snapshot, _, err := e.api.deps.Product.ReserveOrchestratorTurn(orchestratorExecutionActor(t), p.ID, orchestratorTurnSelection(req), "assist-inflight-original")
	if err != nil {
		e.api.orchestratorAdmissionMu.Unlock()
		t.Fatal(err)
	}
	r := orchestratorResolutionRequest(t, e, p.ID, body, true)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { w := httptest.NewRecorder(); e.mux.ServeHTTP(w, r); done <- w }()
	select {
	case <-done:
		e.api.orchestratorAdmissionMu.Unlock()
		t.Fatal("resolution bypassed in-flight admission")
	case <-time.After(20 * time.Millisecond):
	}
	contextBytes, _ := json.Marshal(snapshot)
	job := assistance.Job{ID: turn.JobID, ProjectID: p.ID, Kind: "orchestrator", OrchestratorTurnID: turn.ID, OrchestratorContext: contextBytes, Status: "queued", CreatedAt: turn.CreatedAt}
	err = e.api.assistanceStore().Create(job)
	e.api.orchestratorAdmissionMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-done:
		var result orchestratorResolutionResponse
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Status != "accepted" || result.Job == nil || result.Job.ID != job.ID {
			t.Fatal(w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resolution did not follow completed admission")
	}
}

func TestOrchestratorResolutionConcurrentOriginalAllocatesAtMostOnce(t *testing.T) {
	e, p := repositoryFixture(t)
	e.be.mu.Lock()
	baseline := len(e.be.specs)
	e.be.mu.Unlock()
	route := saveAssistanceGateway(t, e)
	e.be.onCreate = func(spec backend.Spec) {
		orchestratorOutput(t, spec, product.OrchestratorAnswer{Summary: "Review the saved project definition.", Options: []string{}, Impact: []string{}, NextAction: "review-context"})
	}
	body := orchestratorRequestFor(t, e, p.ID, "Continue our work", "concurrent-resolution", "", route)
	send, resolve := orchestratorResolutionRequest(t, e, p.ID, body, false), orchestratorResolutionRequest(t, e, p.ID, body, true)
	var sent, resolved httptest.ResponseRecorder
	sent = *httptest.NewRecorder()
	resolved = *httptest.NewRecorder()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; e.mux.ServeHTTP(&sent, send) }()
	go func() { defer wg.Done(); <-start; e.mux.ServeHTTP(&resolved, resolve) }()
	close(start)
	wg.Wait()
	var result orchestratorResolutionResponse
	if resolved.Code != 200 || json.Unmarshal(resolved.Body.Bytes(), &result) != nil {
		t.Fatal(resolved.Code, resolved.Body.String())
	}
	want := baseline
	if result.Status == "not-sent" {
		if code := sessionsProblemCode(t, &sent, 409); code != "orchestrator_request_closed" {
			t.Fatal(code)
		}
	} else if result.Status == "accepted" {
		if sent.Code != 202 || result.Job == nil {
			t.Fatal(sent.Code, sent.Body.String(), result)
		}
		var job assistance.Job
		if err := json.Unmarshal(sent.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		if job.ID != result.Job.ID {
			t.Fatal("concurrent resolve acquired different job")
		}
		if complete := awaitAssist(t, e, job); complete.Status != "succeeded" {
			t.Fatal(complete.Message)
		}
		want = baseline + 1
	} else {
		t.Fatal("serialized valid original was uncertain", result)
	}
	e.be.mu.Lock()
	allocations := len(e.be.specs)
	e.be.mu.Unlock()
	if allocations != want {
		t.Fatalf("provider allocations=%d, want %d", allocations, want)
	}
	// Repeated resolution, including after source drift, never acquires credentials.
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("The source changed after the original request.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if again := resolveOrchestratorTest(t, e, p.ID, body); again.Status != result.Status {
		t.Fatal(again)
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if after != allocations {
		t.Fatal("resolution replayed paid work")
	}
}

func resolutionViewerContext(t *testing.T, id, projectID string) context.Context {
	t.Helper()
	actor, policy, ok := management.ActorFromContext(orchestratorExecutionActor(t))
	if !ok {
		t.Fatal("fixture actor missing")
	}
	actor.ID, actor.Username, actor.Role = id, id, "member"
	const token = "orchestrator-resolution-private-worker-test-token"
	var ctx context.Context
	handler := management.WorkerHandler(token, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	for name, value := range map[string]any{"X-Luci-Actor": actor, "X-Luci-Policy": policy} {
		raw, _ := json.Marshal(value)
		r.Header.Set(name, base64.RawURLEncoding.EncodeToString(raw))
	}
	handler.ServeHTTP(httptest.NewRecorder(), r)
	if ctx == nil {
		t.Fatal("viewer actor context unavailable")
	}
	return management.WithTeamAccess(ctx, management.TeamAccess{Team: management.Team{ID: "team-aaaaaaaaaaaaaaaaaaaaaa"}, Member: management.TeamMember{UserID: id, Role: "viewer", Revision: 2}, Permissions: management.PermissionsForTeam("viewer", management.TeamGrants{}), ProjectID: projectID, ProjectRevision: 2})
}

func TestOrchestratorResolutionViewerOwnReceiptAndForeignActorRefusal(t *testing.T) {
	e, p := repositoryFixture(t)
	e.be.mu.Lock()
	baseline := len(e.be.specs)
	e.be.mu.Unlock()
	body := orchestratorRequestFor(t, e, p.ID, "Our original paid request", "viewer-resolution", "", product.AgentConnection{ID: "disconnected-provider"})
	raw, _ := json.Marshal(body)
	var req orchestratorRequest
	_ = json.Unmarshal(raw, &req)
	turn, snapshot, _, err := e.api.deps.Product.ReserveOrchestratorTurn(orchestratorExecutionActor(t), p.ID, orchestratorTurnSelection(req), "assist-viewer-original")
	if err != nil {
		t.Fatal(err)
	}
	contextBytes, _ := json.Marshal(snapshot)
	job := assistance.Job{ID: turn.JobID, ProjectID: p.ID, Kind: "orchestrator", OrchestratorTurnID: turn.ID, OrchestratorContext: contextBytes, Status: "queued", CreatedAt: turn.CreatedAt}
	if err = e.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	before, err := e.api.deps.Product.OrchestratorView(context.Background(), p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range []string{"u-reviewed-task-fixture", "bob"} {
		r := orchestratorResolutionRequest(t, e, p.ID, body, true).WithContext(resolutionViewerContext(t, actor, p.ID))
		w := httptest.NewRecorder()
		e.mux.ServeHTTP(w, r)
		var result orchestratorResolutionResponse
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		if actor == "bob" {
			if result.Status != "unknown" || result.Job != nil || result.JobID != "" || result.TurnID != "" {
				t.Fatal("foreign viewer acquired another actor receipt", result)
			}
		} else if result.Status != "accepted" || result.Job == nil || result.Job.ID != job.ID {
			t.Fatal("demoted viewer could not recover own receipt", result)
		}
	}
	after, err := e.api.deps.Product.OrchestratorView(context.Background(), p.ID, 0)
	if err != nil || after.StorageDigest != before.StorageDigest {
		t.Fatal("viewer resolution mutated another actor ledger", err)
	}
	e.be.mu.Lock()
	allocations := len(e.be.specs)
	e.be.mu.Unlock()
	if allocations != baseline {
		t.Fatal("viewer resolution dispatched provider")
	}
}
