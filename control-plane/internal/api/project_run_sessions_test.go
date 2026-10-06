package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestProjectRunSessionPairsRefusePartialAndUnconfiguredBeforeAllocation(t *testing.T) {
	env := sessionsNewEnv(t)
	for _, body := range []string{
		`{"worktreeId":"w-4a77b1c9","projectRunId":"run-test"}`,
		`{"worktreeId":"w-4a77b1c9","projectRunOperationId":"op-test"}`,
		`{"worktreeId":"w-4a77b1c9","projectRunId":"run-test","projectRunOperationId":"op-test"}`,
	} {
		w := env.do(t, "POST", "/v1/sessions", body)
		if w.Code != 400 && w.Code != 409 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	rows, err := env.st.List()
	if err != nil || len(rows) != 0 || len(env.be.specs) != 0 {
		t.Fatal("invalid run pair allocated or persisted a child", err)
	}
}

func reservedProjectRunSession(t *testing.T, subscription bool) (*sessionsEnv, product.ProjectRun, product.ProjectRunOperation, sessionCreateRequest, registry.Session) {
	return reservedProjectRunSessionFor(t, subscription, "codex")
}

func reservedProjectRunSessionFor(t *testing.T, subscription bool, provider string) (*sessionsEnv, product.ProjectRun, product.ProjectRunOperation, sessionCreateRequest, registry.Session) {
	t.Helper()
	env, p, wt, _ := sessionsPrimaryBranchFixture(t)
	selection := reviewedSessionPlan(t, env, p.ID)
	connectionID := ""
	if subscription {
		if err := os.MkdirAll(env.conversations, 0700); err != nil {
			t.Fatal(err)
		}
		if err := env.api.writeCodexConnection(&codexConnection{ID: "fixture-private-run-generation", SourceSessionID: "s-fixture-signin", Auth: json.RawMessage(codexFixtureAuth)}); err != nil {
			t.Fatal(err)
		}
	} else {
		connectionID = sessionSaveConnection(t, env, 0, "", provider, true).ID
	}
	env.api.deps.Product.SetProjectRunSelectionResolver(env.api.ResolveProjectRunSelection)
	env.api.deps.Product.SetProjectRunSessionObserver(env.api.ObserveProjectRunSessions)
	body, _ := json.Marshal(map[string]any{"idempotencyKey": "create-project-run-session-001", "worktreeId": wt.ID, "harness": provider, "permissionMode": "ask", "agentConnectionId": connectionID, "outcome": "preview", "implementationContext": selection})
	base := "/v1/product/projects/" + p.ID + "/runs"
	w := env.do(t, "POST", base, string(body))
	var result struct {
		Run       product.ProjectRun          `json:"run"`
		Operation product.ProjectRunOperation `json:"operation"`
	}
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal("create run", w.Code, w.Body.String())
	}
	if len(env.be.specs) != 0 {
		t.Fatal("saving a run allocated a coding child")
	}
	body, _ = json.Marshal(map[string]any{"expectedRevision": result.Run.Revision, "idempotencyKey": "reserve-project-run-session-001", "kind": "session"})
	w = env.do(t, "POST", base+"/"+result.Run.ID+"/operations", string(body))
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal("reserve operation", w.Code, w.Body.String())
	}
	run := result.Run
	req := sessionCreateRequest{WorktreeID: wt.ID, Harness: run.Input.Route.Harness, Model: run.Input.Route.Model, PermissionMode: run.Input.Route.PermissionMode, Effort: run.Input.Route.Effort, AgentConnectionID: &connectionID, ImplementationContext: &selection, ProjectRunID: run.ID, ProjectRunOperationID: result.Operation.ID}
	bound, err := env.api.deps.Product.BindImplementationContext(context.Background(), selection, wt.ID, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := env.api.ResolveProjectRunSelection(context.Background(), product.ProjectRunSelectionRequest{ProjectID: p.ID, WorktreeID: wt.ID, Harness: req.Harness, Model: req.Model, PermissionMode: req.PermissionMode, Effort: req.Effort, AgentConnectionID: req.AgentConnectionID})
	if err != nil {
		t.Fatal(err)
	}
	candidate := registry.Session{
		ProjectID: p.ID, WorktreeID: wt.ID, Harness: req.Harness, Model: req.Model, Effort: req.Effort, PermissionMode: req.PermissionMode,
		AgentConnectionID: selected.AgentConnectionID, AgentConnectionRevision: selected.AgentConnectionRevision,
		AgentConnectionKind: selected.AgentConnectionKind, AgentConnectionProtocol: selected.AgentConnectionProtocol,
		AuthMode: selected.AuthMode, CodexConnectionID: selected.CodexConnectionID,
		ImplementationContext: bound, OpeningPrompt: product.ImplementationOpeningPrompt(bound),
		ProjectRunID: run.ID, ProjectRunOperationID: result.Operation.ID,
	}
	return env, run, result.Operation, req, candidate
}

func TestProjectRunReservedSessionConcurrentDispatchAndReplayAllocateOnce(t *testing.T) {
	for _, subscription := range []bool{false, true} {
		name := "gateway"
		if subscription {
			name = "subscription"
		}
		t.Run(name, func(t *testing.T) {
			env, run, op, req, _ := reservedProjectRunSession(t, subscription)
			sessionsStubReady(t, nil)
			done := sessionsWatchDone(t)
			observed := make(chan bool, 1)
			env.be.onCreate = func(spec backend.Spec) {
				child, err := env.st.Get(spec.SessionID)
				observed <- err == nil && child.ID == op.ChildID && child.ProjectRunID == run.ID && child.ProjectRunOperationID == op.ID
			}
			body, _ := json.Marshal(req)
			responses := make(chan *httptest.ResponseRecorder, 2)
			for i := 0; i < 2; i++ {
				go func() { responses <- env.do(t, "POST", "/v1/sessions", string(body)) }()
			}
			created, reused := 0, 0
			for i := 0; i < 2; i++ {
				w := <-responses
				if w.Code == 201 {
					created++
				} else if w.Code == 200 {
					reused++
				} else {
					t.Fatal("dispatch", w.Code, w.Body.String())
				}
				var child sessionJSON
				if json.Unmarshal(w.Body.Bytes(), &child) != nil || child.ID != op.ChildID || child.ProjectRunID != run.ID || child.ProjectRunOperationID != op.ID {
					t.Fatal("dispatch returned a different reserved child")
				}
			}
			sessionsAwaitWatch(t, done)
			if created != 1 || reused != 1 || len(env.be.specs) != 1 || !<-observed {
				t.Fatal("duplicate dispatch allocated twice or lost pre-allocation run binding")
			}
			if err := os.WriteFile(filepath.Join(env.dir, "coded-result.txt"), []byte("real code changes after allocation\n"), 0600); err != nil {
				t.Fatal(err)
			}
			w := env.do(t, "POST", "/v1/sessions", string(body))
			if w.Code != 200 || len(env.be.specs) != 1 {
				t.Fatal("a completed create could not recover its immutable child after source changes", w.Code, w.Body.String())
			}
			req.Model = "different-model"
			changed, _ := json.Marshal(req)
			if w := env.do(t, "POST", "/v1/sessions", string(changed)); w.Code != 409 || len(env.be.specs) != 1 {
				t.Fatal("different model reused a reserved operation", w.Code)
			}
		})
	}
}

func TestProjectRunInterruptedDispatchDoesNotReplayMissingOrStoredChild(t *testing.T) {
	for _, storeChild := range []bool{false, true} {
		name := "before-registry"
		if storeChild {
			name = "after-registry"
		}
		t.Run(name, func(t *testing.T) {
			env, run, op, req, candidate := reservedProjectRunSession(t, false)
			admission, err := env.api.deps.Product.BeginProjectRunSession(run.ProjectID, run.ID, op.ID, candidate, env.st.Get)
			if err != nil {
				t.Fatal(err)
			}
			if storeChild {
				candidate.ID, candidate.SandboxID, candidate.Status = admission.SessionID, admission.SessionID, registry.StatusStarting
				if err := env.st.Create(candidate); err != nil {
					admission.Release()
					t.Fatal(err)
				}
			}
			admission.Release()
			body, _ := json.Marshal(req)
			w := env.do(t, "POST", "/v1/sessions", string(body))
			want := 409
			if storeChild {
				want = 200
			}
			if w.Code != want || len(env.be.specs) != 0 {
				t.Fatal("interrupted dispatch replayed or lost its reserved child", w.Code, w.Body.String())
			}
		})
	}
}

func TestProjectRunChangedAttemptSourceRefusesBeforeAllocation(t *testing.T) {
	env, _, _, req, _ := reservedProjectRunSession(t, false)
	if err := os.WriteFile(filepath.Join(env.dir, "outside-edit.txt"), []byte("source changed since reservation\n"), 0600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(req)
	w := env.do(t, "POST", "/v1/sessions", string(body))
	rows, err := env.st.List()
	if w.Code != 409 || len(env.be.specs) != 0 || err != nil || len(rows) != 0 {
		t.Fatal("changed source allocated an unreserved attempt", w.Code, w.Body.String(), err)
	}
}

func TestProjectRunConcurrentDifferentPromptsCannotReuseLateAdmission(t *testing.T) {
	env, _, _, req, _ := reservedProjectRunSessionFor(t, false, "claude-code")
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	arrived, proceed := make(chan struct{}, 2), make(chan struct{})
	env.api.deps.GitLineEndings = func(context.Context) (GitLineEndings, error) {
		arrived <- struct{}{}
		<-proceed
		return GitLineEndings{AutoCRLF: "false", EOL: "lf"}, nil
	}
	responses := make(chan *httptest.ResponseRecorder, 2)
	for _, prompt := range []string{"First request direction.", "Conflicting request direction."} {
		changed := req
		changed.OpeningPrompt = prompt
		body, _ := json.Marshal(changed)
		go func(body string) { responses <- env.do(t, "POST", "/v1/sessions", body) }(string(body))
	}
	<-arrived
	<-arrived
	close(proceed)
	created, refused := 0, 0
	for i := 0; i < 2; i++ {
		w := <-responses
		switch w.Code {
		case 201:
			created++
		case 409:
			refused++
		default:
			t.Fatal(w.Code, w.Body.String())
		}
	}
	sessionsAwaitWatch(t, done)
	if created != 1 || refused != 1 || len(env.be.specs) != 1 {
		t.Fatal("concurrent mismatched prompt reused a child or allocated twice")
	}
}

type projectRunMutatingStore struct {
	registry.Store
	afterCreate func()
}

func (s projectRunMutatingStore) Create(child registry.Session) error {
	if err := s.Store.Create(child); err != nil {
		return err
	}
	s.afterCreate()
	return nil
}

func TestProjectRunRevalidatesSourceAfterRegistryWriteBeforeBackendAllocation(t *testing.T) {
	env, _, _, req, _ := reservedProjectRunSession(t, false)
	var writeErr error
	env.api.deps.Store = projectRunMutatingStore{Store: env.st, afterCreate: func() {
		writeErr = os.WriteFile(filepath.Join(env.dir, "late-source-change.txt"), []byte("changed at allocation boundary\n"), 0600)
	}}
	body, _ := json.Marshal(req)
	w := env.do(t, "POST", "/v1/sessions", string(body))
	rows, err := env.st.List()
	if writeErr != nil || err != nil || w.Code != 409 || len(env.be.specs) != 0 || len(rows) != 1 || rows[0].Status != registry.StatusExited {
		t.Fatal("late source change crossed the allocation boundary", w.Code, w.Body.String(), writeErr, err)
	}
}

func TestProjectRunOperationsRejectUnreservedConversationResumes(t *testing.T) {
	env, _, _, req, _ := reservedProjectRunSession(t, false)
	req.Resume = "s-unreserved-conversation"
	body, _ := json.Marshal(req)
	w := env.do(t, "POST", "/v1/sessions", string(body))
	rows, err := env.st.List()
	if w.Code != 409 || err != nil || len(rows) != 0 || len(env.be.specs) != 0 {
		t.Fatal("run operation accepted an unreserved transcript", w.Code, w.Body.String(), err)
	}
}

type projectRunObservingBackend struct {
	backend.Backend
	state backend.State
	err   error
}

func (b projectRunObservingBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	return backend.Sandbox{ID: id, State: b.state}, b.err
}

func TestProjectRunChildObservationIsReadOnlyAndMissingRuntimeStaysUnknown(t *testing.T) {
	for _, test := range []struct {
		name, sandboxID, reason string
		state                   backend.State
		err                     error
	}{
		{name: "unallocated", reason: "project-run-outcome-unknown"},
		{name: "missing", sandboxID: "s-00000001", err: backend.ErrNotFound, reason: "project-run-outcome-unknown"},
		{name: "stopped", sandboxID: "s-00000001", state: backend.StateStopped, reason: "exited"},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			sess := registry.Session{ID: "s-00000001", ProjectID: "p-00000001", ProjectRunID: "run-001", ProjectRunOperationID: "op-001", Status: registry.StatusStarting, SandboxID: test.sandboxID}
			if err := env.st.Create(sess); err != nil {
				t.Fatal(err)
			}
			before, _ := env.st.Get(sess.ID)
			env.api.deps.Backend = projectRunObservingBackend{Backend: env.be, state: test.state, err: test.err}
			rows, err := env.api.ObserveProjectRunSessions(context.Background(), sess.ProjectID, sess.ProjectRunID, sess.ProjectRunOperationID)
			if err != nil || len(rows) != 1 || rows[0].Status != registry.StatusExited || rows[0].ExitReason != test.reason || len(env.be.specs) != 0 {
				t.Fatal("runtime observation invented progress or allocated a child", rows, err)
			}
			after, _ := env.st.Get(sess.ID)
			if after.Status != before.Status || after.ExitReason != before.ExitReason || after.UpdatedAt != before.UpdatedAt {
				t.Fatal("read-only observation rewrote private lifecycle state")
			}
		})
	}
}

func TestProjectRunSelectionResolvesExactRoutesWithoutAllocating(t *testing.T) {
	for _, provider := range []string{"codex", "claude-code", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			env, p, wt, _ := sessionsPrimaryBranchFixture(t)
			connection := sessionSaveConnection(t, env, 0, "", provider, true)
			selection, err := env.api.ResolveProjectRunSelection(context.Background(), product.ProjectRunSelectionRequest{
				ProjectID: p.ID, WorktreeID: wt.ID, Harness: provider, AgentConnectionID: &connection.ID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if selection.ProjectID != p.ID || selection.WorktreeID != wt.ID || selection.Path != p.Path || selection.AgentConnectionID != connection.ID || selection.AgentConnectionRevision != connection.Revision || selection.AgentConnectionProtocol == "" || selection.PermissionMode != "ask" || selection.Model == "" || selection.Model == "default" {
				t.Fatal("selection lost explicit route", selection)
			}
			rows, err := env.st.List()
			if err != nil || len(rows) != 0 || len(env.be.specs) != 0 {
				t.Fatal("selection allocated a child", err)
			}
			raw, _ := json.Marshal(selection)
			if strings.Contains(string(raw), sessionGatewayKey) || strings.Contains(string(raw), p.Path) {
				t.Fatal("selection leaked private state")
			}
			if _, err := env.api.ResolveProjectRunSelection(context.Background(), product.ProjectRunSelectionRequest{ProjectID: "p-00000000", WorktreeID: wt.ID, Harness: provider}); err == nil {
				t.Fatal("foreign project accepted worktree")
			}
			sessionSaveConnection(t, env, 1, connection.ID, provider, false)
			if _, err := env.api.ResolveProjectRunSelection(context.Background(), product.ProjectRunSelectionRequest{ProjectID: p.ID, WorktreeID: wt.ID, Harness: provider, AgentConnectionID: &connection.ID}); err == nil {
				t.Fatal("disabled explicit connection silently replaced")
			}
		})
	}
}

func TestProjectRunChildCannotRestartOrEscapeThroughIndependentResume(t *testing.T) {
	env := sessionsNewEnv(t)
	sess := sessionsFullRecord()
	sess.ProjectRunID, sess.ProjectRunOperationID = "run-001", "op-001"
	if err := env.st.Create(sess); err != nil {
		t.Fatal(err)
	}
	w := env.do(t, "POST", "/v1/sessions/"+sess.ID+"/restart", "{}")
	if w.Code != 409 || env.be.destroyCount() != 0 || len(env.be.specs) != 0 {
		t.Fatal("run restart replaced a child attempt", w.Code, w.Body.String())
	}
	for _, req := range []sessionCreateRequest{
		{},
		{ProjectRunID: "run-other", ProjectRunOperationID: "op-new"},
		{ProjectRunID: sess.ProjectRunID, ProjectRunOperationID: sess.ProjectRunOperationID},
	} {
		w := httptest.NewRecorder()
		if validateProjectRunResume(w, req, sess) || w.Code != 409 {
			t.Fatal("resume escaped the immutable run attempt")
		}
	}
	if validateProjectRunResume(httptest.NewRecorder(), sessionCreateRequest{ProjectRunID: sess.ProjectRunID, ProjectRunOperationID: "op-new"}, sess) {
		t.Fatal("new operation resumed an unreserved earlier conversation")
	}
}

func TestProjectRunSessionProjectionKeepsPairAndOmitsPrivateGeneration(t *testing.T) {
	sess := registry.Session{ProjectRunID: "run-001", ProjectRunOperationID: "op-001", CodexConnectionID: "private-auth-generation-001"}
	raw, err := json.Marshal(toJSON(sess, sessionBasePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"projectRunId":"run-001"`) || !strings.Contains(string(raw), `"projectRunOperationId":"op-001"`) || strings.Contains(string(raw), sess.CodexConnectionID) || strings.Contains(string(raw), "codexConnectionId") {
		t.Fatal("session projection lost run binding or exposed private auth generation")
	}
	selection, _ := json.Marshal(product.ProjectRunSelection{Path: "/private/controller/path", CodexConnectionID: sess.CodexConnectionID})
	if strings.Contains(string(selection), "private") {
		t.Fatal("selection projection leaked private resolution fields")
	}
}

func TestProjectRunReplayMatchesImmutableSelectionWithoutReadingCurrentSource(t *testing.T) {
	bound := &registry.ImplementationContext{
		ProjectID: "p-00000001", WorktreeID: "w-00000001", BriefRevision: 1, PlanRevision: 2,
		SpecificationRevision: 3, DesignPlanRevision: 4, TaskIDs: []string{"one", "two"}, TaskTitles: []string{"First task", "Second task"},
		SourceDigest: "original-starting-source", SourceCommit: "original-starting-commit",
	}
	connection := "agent-00000001"
	sess := registry.Session{
		ProjectRunID: "run-001", ProjectRunOperationID: "op-001", WorktreeID: bound.WorktreeID,
		Harness: "codex", Model: "gateway-codex", PermissionMode: "ask", AgentConnectionID: connection,
		ImplementationContext: bound, OpeningPrompt: product.ImplementationOpeningPrompt(bound),
	}
	req := sessionCreateRequest{
		ProjectRunID: sess.ProjectRunID, ProjectRunOperationID: sess.ProjectRunOperationID,
		WorktreeID: bound.WorktreeID, Harness: "codex", Model: "default", AgentConnectionID: &connection,
		ImplementationContext: product.ImplementationContextSelection(bound),
	}
	if !projectRunSessionRequestMatches(req, sess) {
		t.Fatal("same reserved selection did not reuse existing child")
	}
	for _, alter := range []func(*sessionCreateRequest){
		func(r *sessionCreateRequest) { r.Model = "other-model" },
		func(r *sessionCreateRequest) { r.PermissionMode = "bypass" },
		func(r *sessionCreateRequest) { r.WorktreeID = "w-other" },
		func(r *sessionCreateRequest) { r.OpeningPrompt = "different direction" },
		func(r *sessionCreateRequest) {
			changed := *r.ImplementationContext
			changed.PlanRevision++
			r.ImplementationContext = &changed
		},
	} {
		changed := req
		alter(&changed)
		if projectRunSessionRequestMatches(changed, sess) {
			t.Fatal("changed immutable request reused existing child")
		}
	}
}
