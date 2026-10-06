package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func orchestratorExecutionActor(t *testing.T) context.Context {
	t.Helper()
	const secret = "orchestrator-execution-fixture-worker-private-token"
	actor, _ := json.Marshal(management.User{ID: "u-reviewed-task-fixture", Username: "reviewer", DisplayName: "Reviewed task owner", Role: "super-admin"})
	policy, _ := json.Marshal(management.Policy{MaxSessionsPerUser: 10, MaxConcurrentChecksPerUser: 2, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"codex", "shell"}, BackingMode: "docker", InstanceName: "Orchestrator execution fixture"})
	var ctx context.Context
	h := management.WorkerHandler(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ctx = r.Context(); w.WriteHeader(204) }))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(actor))
	r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if ctx == nil || w.Code != 204 {
		t.Fatalf("trusted actor context unavailable: %d", w.Code)
	}
	return ctx
}

func executionOrchestratorTurn(t *testing.T, env *sessionsEnv, projectID, reply string, answer *product.OrchestratorAnswer) product.OrchestratorTurn {
	t.Helper()
	ctx := orchestratorExecutionActor(t)
	view, err := env.api.deps.Product.OrchestratorView(ctx, projectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	jobID := "assist-execution-" + strings.Repeat("a", 16)
	if reply != "" {
		jobID = "assist-execution-" + strings.Repeat("b", 16)
	}
	turn, _, existing, err := env.api.deps.Product.ReserveOrchestratorTurn(ctx, projectID, product.OrchestratorTurnRequest{
		ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest,
		IdempotencyKey: jobID, Message: "Keep this reviewed implementation coherent.", ReplyToTurnID: reply, RouteDigest: strings.Repeat("a", 64),
	}, jobID)
	if err != nil || existing {
		t.Fatal("reserve conversation turn", existing, err)
	}
	if err := env.api.deps.Product.CompleteOrchestratorTurn(projectID, turn.ID, jobID, "succeeded", answer); err != nil {
		t.Fatal(err)
	}
	return turn
}

func executionBlockingAnswer() *product.OrchestratorAnswer {
	return &product.OrchestratorAnswer{Summary: "An implementation decision is unresolved.", Question: "Should saved notes be private to each person or shared with the team?", Rationale: "Storage and permissions depend on this decision.", Blocking: true, Options: []string{"Private per person", "Shared with the team"}, Impact: []string{"Definition", "Data access"}, NextAction: "answer-question"}
}

func TestOrchestratorQuestionBlocksReservedAndDirectReviewedChildren(t *testing.T) {
	for _, changeContext := range []bool{false, true} {
		name := "answer-resumes-current-reviewed-selection"
		if changeContext {
			name = "answer-cannot-restore-stale-approvals"
		}
		t.Run(name, func(t *testing.T) {
			env, run, _, req, _ := reservedProjectRunSession(t, false)
			question := executionOrchestratorTurn(t, env, run.ProjectID, "", executionBlockingAnswer())
			body, _ := json.Marshal(req)
			w := env.do(t, "POST", "/v1/sessions", string(body))
			if w.Code != 409 || !strings.Contains(w.Body.String(), "unresolved material question") {
				t.Fatal(w.Code, w.Body.String())
			}
			// Omitting the coordinator IDs cannot bypass the same reviewed-task gate.
			direct := req
			direct.ProjectRunID, direct.ProjectRunOperationID = "", ""
			directBody, _ := json.Marshal(direct)
			if w := env.do(t, "POST", "/v1/sessions", string(directBody)); w.Code != 409 {
				t.Fatal(w.Code, w.Body.String())
			}
			rows, err := env.st.List()
			if err != nil || len(rows) != 0 || len(env.be.specs) != 0 {
				t.Fatal("ambiguity allocated a child", rows, err)
			}
			if w := env.do(t, "GET", "/v1/product/projects/"+run.ProjectID+"/runs/"+run.ID, ""); w.Code != 200 {
				t.Fatal("ambiguity blocked history", w.Code)
			}
			if changeContext {
				var spec product.ProjectSpecification
				w := env.do(t, "GET", "/v1/product/projects/"+run.ProjectID+"/specification", "")
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &spec) != nil {
					t.Fatal(w.Code, w.Body.String())
				}
				spec.Purpose = "Shared notes now need explicitly reviewed access rules"
				changed, _ := json.Marshal(map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec})
				if w := env.do(t, "POST", "/v1/product/projects/"+run.ProjectID+"/specification", string(changed)); w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
			}
			executionOrchestratorTurn(t, env, run.ProjectID, question.ID, &product.OrchestratorAnswer{Summary: "The owner confirmed team sharing.", Impact: []string{"Continue with reviewed context"}, NextAction: "review-context"})
			if err := env.api.deps.Product.OrchestratorExecutionGate(run.ProjectID); err != nil {
				t.Fatal("answered question stayed blocked", err)
			}
			if changeContext {
				w := env.do(t, "POST", "/v1/sessions", string(body))
				if w.Code != 409 || len(env.be.specs) != 0 {
					t.Fatal("answer restored invalidated approval", w.Code, w.Body.String())
				}
			} else {
				sessionsStubReady(t, nil)
				done := sessionsWatchDone(t)
				w := env.do(t, "POST", "/v1/sessions", string(body))
				if w.Code != 201 || len(env.be.specs) != 1 {
					t.Fatal("current reviewed selection did not recover", w.Code, w.Body.String())
				}
				sessionsAwaitWatch(t, done)
			}
		})
	}
}

func TestOrchestratorQuestionPreservesManualTerminalAndReviewedSessionCleanup(t *testing.T) {
	env, run, _, req, _ := reservedProjectRunSession(t, false)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(req)
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	executionOrchestratorTurn(t, env, run.ProjectID, "", executionBlockingAnswer())
	if !errors.Is(env.api.deps.Product.OrchestratorExecutionGate(run.ProjectID), product.ErrOrchestratorQuestion) {
		t.Fatal("question did not block")
	}
	// Recovering the already allocated immutable child is a read, not a new CLI.
	if w := env.do(t, "POST", "/v1/sessions", string(body)); w.Code != 200 || len(env.be.specs) != 1 {
		t.Fatal("existing child replay was blocked or allocated", w.Code)
	}
	if w := env.do(t, "GET", "/v1/sessions/"+created.ID, ""); w.Code != 200 {
		t.Fatal("session history blocked", w.Code)
	}
	if w := env.do(t, "DELETE", "/v1/sessions/"+created.ID, ""); w.Code != 200 && w.Code != 204 {
		t.Fatal("cleanup blocked", w.Code, w.Body.String())
	}
	done = sessionsWatchDone(t)
	manual, _ := json.Marshal(map[string]any{"worktreeId": req.WorktreeID, "harness": "shell"})
	w := env.do(t, "POST", "/v1/sessions", string(manual))
	if w.Code != 201 || len(env.be.specs) != 2 {
		t.Fatal("manual terminal was blocked", w.Code, w.Body.String())
	}
	sessionsAwaitWatch(t, done)
}

func TestOrchestratorQuestionBlocksDirectReviewedRestartAndResumeWithoutDestroyingSource(t *testing.T) {
	env, p, wt, _ := sessionsPrimaryBranchFixture(t)
	selection := reviewedSessionPlan(t, env, p.ID)
	connection := sessionSaveConnection(t, env, 0, "", "codex", true)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "codex", "agentConnectionId": connection.ID, "implementationContext": selection})
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	source, err := env.st.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	executionOrchestratorTurn(t, env, p.ID, "", executionBlockingAnswer())
	beforeDestroy := env.be.destroyCount()
	w := env.do(t, "POST", "/v1/sessions/"+created.ID+"/restart", "{}")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "unresolved material question") {
		t.Fatal(w.Code, w.Body.String())
	}
	// Resume inherits its source's reviewed context even when the public request
	// omits implementationContext; it cannot masquerade as a manual conversation.
	resume := sessionCreateRequest{WorktreeID: wt.ID, Harness: "codex", Resume: created.ID}
	w = httptest.NewRecorder()
	if _, ok := env.api.bindSessionImplementation(w, httptest.NewRequest("POST", "/v1/sessions", nil), &resume, p.Path, source); ok || w.Code != 409 || !strings.Contains(w.Body.String(), "unresolved material question") {
		t.Fatal("resume bypassed material question", w.Code, w.Body.String())
	}
	if env.be.destroyCount() != beforeDestroy || len(env.be.specs) != 1 {
		t.Fatal("blocked continuation changed the source sandbox")
	}
}
