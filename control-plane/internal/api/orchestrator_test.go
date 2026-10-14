package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func orchestratorRequestFor(t *testing.T, e *sessionsEnv, id, message, key, reply string, route product.AgentConnection) map[string]any {
	t.Helper()
	view, err := e.api.deps.Product.OrchestratorView(orchestratorExecutionActor(t), id, 0)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"expectedRevision": view.Revision, "expectedContextDigest": view.Context.Digest, "idempotencyKey": "orchestration-" + key, "message": message, "replyToTurnId": reply, "harness": "codex", "agentConnectionId": route.ID}
}
func orchestratorDo(t *testing.T, e *sessionsEnv, id string, body any, status int) assistance.Job {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", sessionsTarget("/v1/product/projects/"+id+"/assist/orchestrator"), bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(orchestratorExecutionActor(t))
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("orchestrator %d want%d: %s", w.Code, status, w.Body.String())
	}
	var job assistance.Job
	if status < 300 && json.Unmarshal(w.Body.Bytes(), &job) != nil {
		t.Fatal("invalid job")
	}
	return job
}
func orchestratorOutput(t *testing.T, spec backend.Spec, answer product.OrchestratorAnswer) {
	t.Helper()
	data, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	assistOutput(t, spec, data)
}
func orchestratorPrompt(t *testing.T, spec backend.Spec) []byte {
	t.Helper()
	for _, m := range spec.Mounts {
		if m.ContainerPath == "/luci-input" {
			if !m.ReadOnly {
				t.Fatal("orchestrator input is writable")
			}
			data, err := os.ReadFile(filepath.Join(m.HostPath, "prompt.txt"))
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
	}
	t.Fatal("missing orchestrator input")
	return nil
}

func TestOrchestratorSchemaFencesDraftDigestAndPriority(t *testing.T) {
	data, err := orchestratorResponseSchema(true)
	if err != nil {
		t.Fatal(err)
	}
	base, err := assistance.StrictSchema(product.OrchestratorAnswer{})
	if err != nil || len(data) > 2*len(base)+500 || len(data) > 32<<10 {
		t.Fatal("conditional question contract exceeded the bounded two-branch schema", err)
	}
	upstream, err := orchestratorResponseSchema(false)
	if err != nil || !bytes.Equal(upstream, base) {
		t.Fatal("existing nonnative upstream harness schema changed", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	branches := schema["anyOf"].([]any)
	if len(branches) != 2 {
		t.Fatal("blocking/question relation missing from model contract")
	}
	for _, raw := range branches {
		branch := raw.(map[string]any)
		if branch["type"] != "object" || branch["additionalProperties"] != false || len(branch["required"].([]any)) != 8 {
			t.Fatal("conditional branch lost the complete strict answer object")
		}
	}
	closed := branches[0].(map[string]any)["properties"].(map[string]any)
	open := branches[1].(map[string]any)["properties"].(map[string]any)
	if !reflect.DeepEqual(closed["blocking"].(map[string]any)["enum"], []any{false}) || !reflect.DeepEqual(closed["question"].(map[string]any)["enum"], []any{""}) || !reflect.DeepEqual(open["blocking"].(map[string]any)["enum"], []any{true}) || open["question"].(map[string]any)["minLength"] != float64(8) {
		t.Fatal("model contract permits an unanswered nonblocking or empty blocking question")
	}
	for _, properties := range []map[string]any{closed, open} {
		proposal := properties["proposal"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)["properties"].(map[string]any)
		specification := proposal["specification"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)["properties"].(map[string]any)
		for name, expected := range map[string]string{"digest": "", "status": "draft"} {
			values := specification[name].(map[string]any)["enum"].([]any)
			if len(values) != 1 || values[0] != expected {
				t.Fatal("model contract omitted a server-owned draft field", name, values)
			}
		}
		requirement := specification["requirements"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
		priority := requirement["priority"].(map[string]any)["enum"].([]any)
		if !reflect.DeepEqual(priority, []any{"must", "should", "could"}) {
			t.Fatal("model contract omitted exact saved priority values", priority)
		}
		if requirement["acceptance"].(map[string]any)["minLength"] != float64(12) {
			t.Fatal("model contract permits a priority label as acceptance")
		}
	}
}

func TestOrchestratorPromptHistorySeparatesOldRejectedAnswerAndValidQuestion(t *testing.T) {
	e, p := repositoryFixture(t)
	snapshot, err := e.api.deps.Product.OrchestratorSnapshot(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	old := product.OrchestratorAnswer{Summary: "A draft task list was returned.", Question: "Should HTML, JavaScript or CSS come first?", Blocking: false, Rationale: "Choose an implementation order.", NextAction: "specification"}
	spec := snapshot.Specification
	spec.Requirements = []product.SpecificationRequirement{{ID: "req-001", Title: "Add a task", Priority: "must", Acceptance: "must"}}
	old.Proposal = &product.OrchestratorProposal{Target: "specification", Summary: "Task list draft", Specification: &spec}
	valid := product.OrchestratorAnswer{Summary: "Sharing needs an owner decision.", Question: "Who may view the saved tasks?", Blocking: true, Rationale: "Access changes privacy.", NextAction: "answer-question"}
	current := product.OrchestratorTurn{ID: "turn-current", Sequence: 3}
	history := []product.OrchestratorTurn{
		{ID: "turn-old", Sequence: 1, Status: "succeeded", Message: "Draft a task list", Context: snapshot, Answer: &old},
		{ID: "turn-question", Sequence: 2, Status: "succeeded", Message: "Clarify sharing", Context: snapshot, Answer: &valid},
	}
	previous := orchestratorPromptHistory(history, current)
	if len(previous) != 2 || previous[0].Answer != nil || previous[0].AnswerState != "historical-answer-fails-current-contract" || previous[1].Answer == nil || previous[1].Answer.Question != valid.Question {
		t.Fatal("historical model output was treated as an accepted decision or a genuine question was lost")
	}
	encoded, _ := json.Marshal(previous)
	if bytes.Contains(encoded, []byte(old.Question)) || !bytes.Contains(encoded, []byte("Draft a task list")) || !bytes.Contains(encoded, []byte(valid.Question)) {
		t.Fatal("model excerpt copied a rejected question or lost an authorized discussion", string(encoded))
	}
	validDraft := valid
	validDraft.Question, validDraft.Blocking = "", false
	validDraft.Proposal = &product.OrchestratorProposal{Target: "specification", Summary: "Concrete task list", Specification: &spec}
	validDraft.Proposal.Specification.Requirements[0].Acceptance = "Adding a named task shows it in the visible list."
	history[1].Answer = &validDraft
	previous = orchestratorPromptHistory(history, current)
	if previous[1].AnswerState != "proposal-unapplied" || previous[1].Answer == nil || previous[1].Answer.Proposal != nil {
		t.Fatal("reviewable but unapplied draft appeared as accepted context")
	}
}

func TestOrchestratorAnswerRejectsRoutineQuestionWithCompleteOrchardDraft(t *testing.T) {
	e, p := repositoryFixture(t)
	snapshot, err := e.api.deps.Product.OrchestratorSnapshot(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	spec := snapshot.Specification
	spec.Requirements = []product.SpecificationRequirement{
		{ID: "req-001", Title: "Add tasks", Priority: "must", Acceptance: "Adding a named task shows it in the list."},
		{ID: "req-002", Title: "Complete tasks", Priority: "must", Acceptance: "Toggling completion changes the task's visible state."},
		{ID: "req-003", Title: "Delete tasks", Priority: "must", Acceptance: "Deleting a task removes it from the visible list."},
		{ID: "req-004", Title: "Persist tasks", Priority: "must", Acceptance: "Reloading restores the remaining saved tasks."},
	}
	contextBytes, _ := json.Marshal(snapshot)
	job := assistance.Job{ProjectID: p.ID, OrchestratorContext: contextBytes}
	answer := product.OrchestratorAnswer{
		Summary: "Review the complete Orchard task list draft.", Question: "Which of the following steps should be prioritized first in the development of Orchard: creating the HTML structure, implementing the JavaScript functions, or styling the interface with CSS?", Blocking: false,
		Rationale: "Choose the routine implementation order.", Options: []string{"HTML", "JavaScript", "CSS"},
		Proposal: &product.OrchestratorProposal{Target: "specification", Summary: "Four observable task behaviors", Specification: &spec}, NextAction: "specification",
	}
	data, _ := json.Marshal(answer)
	if _, err := decodeOrchestratorAnswer(job, data); !errors.Is(err, errOrchestratorAnswerContract) {
		t.Fatal("nonblocking routine question escaped the application contract", err)
	}
	answer.Question = ""
	data, _ = json.Marshal(answer)
	if _, err := decodeOrchestratorAnswer(job, data); err != nil {
		t.Fatal("complete observable draft without an unnecessary question was refused", err)
	}
	answer.Proposal = nil
	answer.Blocking, answer.Question = true, "Who may view the saved tasks?"
	data, _ = json.Marshal(answer)
	if _, err := decodeOrchestratorAnswer(job, data); err != nil {
		t.Fatal("genuine consequential clarification was refused", err)
	}
}

func TestOrchestratorCompletedLocalModelInvalidPriorityRetainsSafeCause(t *testing.T) {
	e, p := repositoryFixture(t)
	route := saveAssistanceGateway(t, e)
	useMissingAssistantRoot(t, e)
	before, err := e.api.deps.Product.OrchestratorSnapshot(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	proposal := before.Specification
	proposal.Status, proposal.Digest = "draft", strings.Repeat("a", 64)
	proposal.Requirements = append(proposal.Requirements, product.SpecificationRequirement{ID: "req-001", Title: "Keep tasks", Priority: "High", Acceptance: "A task survives reload."})
	contextBytes, _ := json.Marshal(before)
	check := assistance.Job{ProjectID: p.ID, OrchestratorContext: contextBytes}
	answer := product.OrchestratorAnswer{Summary: "Review the Orchard task list.", Options: []string{}, Impact: []string{"Definition"}, Proposal: &product.OrchestratorProposal{Target: "specification", Summary: "Keep tasks", Specification: &proposal}, NextAction: "specification"}
	for _, invalid := range []product.ProjectSpecification{proposal, func() product.ProjectSpecification { copy := proposal; copy.Digest = ""; return copy }()} {
		answer.Proposal.Specification = &invalid
		data, _ := json.Marshal(answer)
		if _, err := decodeOrchestratorAnswer(check, data); !errors.Is(err, errOrchestratorAnswerContract) {
			t.Fatal("generated digest or priority escaped product response validation", err)
		}
	}
	answer.Proposal.Specification = &proposal
	e.be.onCreate = func(spec backend.Spec) {
		orchestratorOutput(t, spec, answer)
	}
	request := orchestratorRequestFor(t, e, p.ID, "Propose a complete local task list.", "invalid-priority", "", route)
	request["harness"] = "opencode"
	job := orchestratorDo(t, e, p.ID, request, 202)
	got := awaitAssist(t, e, job)
	if got.Status != "failed" || got.FailureCode != "local_model_project_answer_invalid" || got.OrchestratorResult != nil || !strings.Contains(got.Message, "draft rules") {
		t.Fatal("post-generation contract rejection was mistaken for a connection failure", got.Status, got.FailureCode, got.Message)
	}
	turn, err := e.api.deps.Product.OrchestratorTurn(p.ID, job.OrchestratorTurnID)
	if err != nil || turn.Status != "failed" || turn.FailureCode != got.FailureCode || turn.Answer != nil {
		t.Fatal("safe post-result failure did not reach the durable conversation", err, turn)
	}
	after, err := e.api.deps.Product.OrchestratorSnapshot(context.Background(), p.ID)
	if err != nil || after.Digest != before.Digest {
		t.Fatal("invalid priority changed project context", err)
	}
}

func TestOrchestratorUsesSelectedProviderRetainsAnswersAndDoesNotApplyProposals(t *testing.T) {
	for _, harness := range []string{"codex", "claude-code", "opencode"} {
		t.Run(harness, func(t *testing.T) {
			e, p := repositoryFixture(t)
			route := saveAssistanceGateway(t, e)
			useMissingAssistantRoot(t, e)
			before, err := e.api.deps.Product.OrchestratorSnapshot(context.Background(), p.ID)
			if err != nil {
				t.Fatal(err)
			}
			request := orchestratorRequestFor(t, e, p.ID, "Help us choose who can view stored notes.", "initial-turn", "", route)
			request["harness"] = harness
			started := make(chan backend.Spec, 1)
			release := make(chan struct{})
			released := false
			defer func() {
				if !released {
					close(release)
				}
			}()
			e.be.onCreate = func(spec backend.Spec) { started <- spec; <-release }
			job := orchestratorDo(t, e, p.ID, request, 202)
			var spec backend.Spec
			select {
			case spec = <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("orchestrator did not reach existing provider runtime")
			}
			if job.Kind != "orchestrator" || job.OrchestratorTurnID == "" || job.AgentConnectionID != route.ID || job.AgentConnectionRevision != route.Revision || job.Harness != harness || job.Model == "default" {
				t.Fatal("actual selected provider provenance lost", job)
			}
			for _, mount := range spec.Mounts {
				relative, _ := filepath.Rel(p.Path, mount.HostPath)
				if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					t.Fatal("orchestrator mounted editable project")
				}
			}
			prompt := orchestratorPrompt(t, spec)
			if !bytes.Contains(prompt, []byte(before.Digest)) || !bytes.Contains(prompt, []byte("\"designPlan\"")) || !bytes.Contains(prompt, []byte("\"plan\"")) || bytes.Contains(prompt, []byte("fixture-gateway-private-key")) {
				t.Fatal("missing immutable cross-phase context or leaked credential")
			}
			replayed := orchestratorDo(t, e, p.ID, request, 200)
			if replayed.ID != job.ID || replayed.SessionID != job.SessionID {
				t.Fatal("same turn allocated another provider job")
			}
			changed := map[string]any{}
			for k, v := range request {
				changed[k] = v
			}
			changed["message"] = "A different instruction with the same key"
			orchestratorDo(t, e, p.ID, changed, 409)
			orchestratorOutput(t, spec, product.OrchestratorAnswer{Summary: "Access rules affect design and implementation.", Question: "Are stored notes private or shared with the team?", Options: []string{"Private", "Shared"}, Rationale: "Data visibility changes authorization.", Blocking: true, Impact: []string{"Privacy", "Access control"}, NextAction: "answer-question"})
			close(release)
			released = true
			got := awaitAssist(t, e, job)
			if got.Status != "succeeded" || len(got.OrchestratorResult) == 0 || got.Result != nil {
				t.Fatal("structured cross-phase response not retained", got.Message)
			}
			history, err := e.api.deps.Product.OrchestratorHistory(p.ID)
			if err != nil || len(history) != 1 || history[0].Answer == nil || history[0].Actor.ID != "u-reviewed-task-fixture" {
				t.Fatal("conversation lost trusted author or completed question", err)
			}
			proposal := before.Specification
			proposal.Purpose = "Team members collaborate on shared notes"
			proposal.Status = "draft"
			e.be.onCreate = func(spec backend.Spec) {
				started <- spec
				orchestratorOutput(t, spec, product.OrchestratorAnswer{Summary: "The team chose shared notes. Review this definition amendment.", Options: []string{}, Rationale: "The answer changes access requirements.", Impact: []string{"Definition"}, Proposal: &product.OrchestratorProposal{Target: "specification", Summary: "Record shared notes", Specification: &proposal}, NextAction: "review-context"})
			}
			reply := orchestratorRequestFor(t, e, p.ID, "Shared with our team, including invited editors.", "answer-turn", job.OrchestratorTurnID, route)
			reply["harness"] = harness
			second := orchestratorDo(t, e, p.ID, reply, 202)
			secondSpec := <-started
			prompt = orchestratorPrompt(t, secondSpec)
			if !bytes.Contains(prompt, []byte("Are stored notes private or shared")) || !bytes.Contains(prompt, []byte("Shared with our team")) || !bytes.Contains(prompt, []byte(job.OrchestratorTurnID)) {
				t.Fatal("provider lost prior consequential question or human answer")
			}
			second = awaitAssist(t, e, second)
			if second.Status != "succeeded" {
				t.Fatal("reviewable amendment was rejected", second.Message)
			}
			after, err := e.api.deps.Product.OrchestratorSnapshot(context.Background(), p.ID)
			if err != nil || !reflect.DeepEqual(before.Specification, after.Specification) || !reflect.DeepEqual(before.DesignPlan, after.DesignPlan) || !reflect.DeepEqual(before.Plan, after.Plan) {
				t.Fatal("assistant silently applied or approved a proposal", err)
			}
			history, err = e.api.deps.Product.OrchestratorHistory(p.ID)
			if err != nil || len(history) != 2 || history[0].QuestionResolvedBy != second.OrchestratorTurnID || history[1].Answer.Proposal == nil {
				t.Fatal("answer/proposal history was not durable", err)
			}
		})
	}
}

func TestOrchestratorMissingSelectedConnectionRecordsFailureWithoutFallback(t *testing.T) {
	e, p := repositoryFixture(t)
	request := orchestratorRequestFor(t, e, p.ID, "Help continue this project.", "missing-route", "", product.AgentConnection{ID: "agent-aaaaaaaaaaaaaaaaaaaaaaaa"})
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	orchestratorDo(t, e, p.ID, request, 409)
	history, err := e.api.deps.Product.OrchestratorHistory(p.ID)
	if err != nil || len(history) != 1 || history[0].Status != "failed" || history[0].Answer != nil {
		t.Fatal("missing connection fabricated an answer or left an unconfirmed turn", err)
	}
	job := orchestratorDo(t, e, p.ID, request, 200)
	if job.Status != "failed" || job.SessionID != "" {
		t.Fatal("failed reservation silently retried inference")
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if after != before {
		t.Fatal("missing selected provider fell back to saved subscription")
	}
}

func TestOrchestratorRecoveryFailsOrphanReservationWithoutNewInference(t *testing.T) {
	e, p := repositoryFixture(t)
	// Reproduce an installation interrupted between reserving the turn and
	// creating its first assistant job, before any private assistant root exists.
	useMissingAssistantRoot(t, e)
	ctx := orchestratorExecutionActor(t)
	view, err := e.api.deps.Product.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	turn, _, existing, err := e.api.deps.Product.ReserveOrchestratorTurn(ctx, p.ID, product.OrchestratorTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "interrupted-reservation", Message: "Continue our work.", RouteDigest: strings.Repeat("a", 64)}, "assist-orphan-turn")
	if err != nil || existing {
		t.Fatal(err)
	}
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	w := e.do(t, "POST", "/v1/product/projects/"+p.ID+"/assist/"+turn.JobID+"/reconcile", `{}`)
	if w.Code != 200 {
		t.Fatal("explicit orphan reconciliation failed", w.Code, w.Body.String())
	}
	job, err := e.api.assistanceStore().Read(turn.JobID)
	if err != nil || job.Status != "failed" || job.SessionID != "" {
		t.Fatal("orphan turn did not become an explicit interruption", err)
	}
	history, err := e.api.deps.Product.OrchestratorHistory(p.ID)
	if err != nil || len(history) != 1 || history[0].Status != "failed" {
		t.Fatal("orphan ledger not reconciled", err)
	}
	e.api.recoverAssistance()
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if after != before {
		t.Fatal("startup recovery repeated inference")
	}
}

func useMissingAssistantRoot(t *testing.T, e *sessionsEnv) {
	t.Helper()
	e.conversations = filepath.Join(t.TempDir(), "fresh-private-state", "conversations")
	e.api.deps.ConversationsDir = e.conversations
	if _, err := os.Lstat(e.conversations); !os.IsNotExist(err) {
		t.Fatal("fresh assistant root already exists", err)
	}
}

func TestOrchestratorFreshInstallPersistsFailuresAndStartsSelectedProvider(t *testing.T) {
	e, p := repositoryFixture(t)
	useMissingAssistantRoot(t, e)
	missing := orchestratorRequestFor(t, e, p.ID, "Help us continue.", "fresh-missing-route", "", product.AgentConnection{ID: "agent-aaaaaaaaaaaaaaaaaaaaaaaa"})
	data, _ := json.Marshal(missing)
	r := httptest.NewRequest("POST", sessionsTarget("/v1/product/projects/"+p.ID+"/assist/orchestrator"), bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(orchestratorExecutionActor(t))
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	if code := sessionsProblemCode(t, w, 409); code != "assistant_connection_required" {
		t.Fatal("fresh storage hid the actual missing provider error", code)
	}
	history, err := e.api.deps.Product.OrchestratorHistory(p.ID)
	if err != nil || len(history) != 1 || history[0].Status != "failed" {
		t.Fatal("fresh installation left its first turn reserved", err)
	}
	job, err := e.api.assistanceStore().Read(history[0].JobID)
	if err != nil || job.Status != "failed" || job.SessionID != "" {
		t.Fatal("missing provider failure was not durable", err)
	}
	if replay := orchestratorDo(t, e, p.ID, missing, 200); replay.ID != job.ID || replay.Status != "failed" {
		t.Fatal("retry did not retain the same failed admission")
	}

	// The user can configure a provider and continue after that first failure.
	route := saveAssistanceGateway(t, e)
	e.be.onCreate = func(spec backend.Spec) {
		orchestratorOutput(t, spec, product.OrchestratorAnswer{Summary: "The next step is reviewing the project definition.", Options: []string{}, Rationale: "The definition needs review before implementation.", Impact: []string{}, NextAction: "review-context"})
	}
	request := orchestratorRequestFor(t, e, p.ID, "Continue with our configured provider.", "fresh-configured-route", "", route)
	job = orchestratorDo(t, e, p.ID, request, 202)
	if got := awaitAssist(t, e, job); got.Status != "succeeded" || got.AgentConnectionID != route.ID {
		t.Fatal("fresh private storage could not run the selected provider", got.Message)
	}
}

func TestOrchestratorAdmissionFailureReleasesProviderBeforePublishingTurn(t *testing.T) {
	e, p := repositoryFixture(t)
	body := orchestratorRequestFor(t, e, p.ID, "Continue this project.", "lock-ordering", "", product.AgentConnection{ID: "agent-aaaaaaaaaaaaaaaaaaaaaaaa"})
	// A reviewed coding admission owns this shared source guard before obtaining
	// the provider mutex. Completion must release the latter before waiting for
	// an exclusive source guard, or these two requests deadlock each other.
	release, err := e.api.deps.Product.AcquireGitSessionAdmission(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	data, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", sessionsTarget("/v1/product/projects/"+p.ID+"/assist/orchestrator"), bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(orchestratorExecutionActor(t))
	done := make(chan int, 1)
	go func() { w := httptest.NewRecorder(); e.mux.ServeHTTP(w, r); done <- w.Code }()
	deadline := time.Now().Add(5 * time.Second)
	failed := false
	for time.Now().Before(deadline) {
		jobs, readErr := e.api.projectAssistance(p.ID)
		if readErr == nil && len(jobs) == 1 && jobs[0].Status == "failed" {
			failed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !failed {
		t.Fatal("admission failure did not reach durable job completion")
	}
	acquired := make(chan struct{})
	go func() { e.api.codexAccountMu.Lock(); close(acquired); e.api.codexAccountMu.Unlock() }()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		release()
		released = true
		t.Fatal("assistant held provider lease while waiting for source completion guard")
	}
	release()
	released = true
	select {
	case status := <-done:
		if status != 409 {
			t.Fatal("unexpected missing provider status", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("failed turn did not finish after source guard released")
	}
}

func TestOrchestratorStructuredOutputRejectsForgedOrIncompleteProvenance(t *testing.T) {
	e, p := repositoryFixture(t)
	snapshot, err := e.api.deps.Product.OrchestratorSnapshot(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	contextBytes, _ := json.Marshal(snapshot)
	job := assistance.Job{ProjectID: p.ID, Kind: "orchestrator", OrchestratorContext: contextBytes}
	valid := product.OrchestratorAnswer{Summary: "Clarify the delivery scope.", Question: "Who approves publication?", Options: []string{"Owner", "Team"}, Rationale: "Approval changes release permissions.", Blocking: true, Impact: []string{"Delivery"}, NextAction: "answer-question"}
	raw, _ := json.Marshal(valid)
	if _, err = decodeOrchestratorAnswer(job, raw); err != nil {
		t.Fatal("valid structured question refused", err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"trailing output", append(append([]byte{}, raw...), []byte(" {}")...)},
		{"missing structured fields", []byte(`{"summary":"Done"}`)},
		{"unknown execution field", bytes.Replace(raw, []byte(`"summary":`), []byte(`"execute":"deploy","summary":`), 1)},
		{"oversized provider output", bytes.Repeat([]byte("x"), assistance.MaxResultBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeOrchestratorAnswer(job, tc.data); err == nil {
				t.Fatal("untrusted structured output accepted")
			}
		})
	}
	for _, tc := range []string{"foreign-project", "forged-approval", "wrong-revision"} {
		t.Run(tc, func(t *testing.T) {
			proposal := snapshot.Specification
			proposal.Status = "draft"
			proposal.Digest = ""
			switch tc {
			case "foreign-project":
				proposal.ProjectID = "p-foreign"
			case "forged-approval":
				proposal.Status = "approved"
			case "wrong-revision":
				proposal.Revision++
			}
			answer := valid
			answer.Proposal = &product.OrchestratorProposal{Target: "specification", Summary: "Review amendment", Specification: &proposal}
			data, _ := json.Marshal(answer)
			if _, err := decodeOrchestratorAnswer(job, data); err == nil {
				t.Fatal("forged context authority accepted")
			}
		})
	}
}

func TestOrchestratorTerminalJobReconcilesConversationWithoutInference(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	view, err := e.api.deps.Product.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot, _, err := e.api.deps.Product.ReserveOrchestratorTurn(ctx, p.ID, product.OrchestratorTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "durable-result-recovery", Message: "Clarify what remains.", RouteDigest: strings.Repeat("a", 64)}, "assist-durable-result")
	if err != nil {
		t.Fatal(err)
	}
	contextBytes, _ := json.Marshal(snapshot)
	answer, _ := json.Marshal(product.OrchestratorAnswer{Summary: "Review the current definition before coding.", Options: []string{}, Impact: []string{"Definition"}, NextAction: "review-definition"})
	job := assistance.Job{ID: turn.JobID, ProjectID: p.ID, Status: "succeeded", Kind: "orchestrator", CreatedAt: turn.CreatedAt, OrchestratorTurnID: turn.ID, OrchestratorContext: contextBytes, OrchestratorResult: answer}
	if err = e.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	// Observing the job is read-only even when its conversation write was lost.
	decodeAssist(t, e, "GET", "/v1/product/projects/"+p.ID+"/assist/"+job.ID, "", 200)
	pending, err := e.api.deps.Product.OrchestratorTurn(p.ID, turn.ID)
	if err != nil || pending.Status != "reserved" {
		t.Fatal("GET repaired private state implicitly", err)
	}
	w := e.do(t, "POST", "/v1/product/projects/"+p.ID+"/assist/"+job.ID+"/reconcile", `{}`)
	if w.Code != 200 {
		t.Fatal("explicit terminal reconciliation failed", w.Code, w.Body.String())
	}
	e.api.recoverAssistance()
	e.api.recoverAssistance()
	history, err := e.api.deps.Product.OrchestratorHistory(p.ID)
	if err != nil || len(history) != 1 || history[0].Status != "succeeded" || history[0].Answer == nil {
		t.Fatal("durable terminal result did not repair reserved conversation", err)
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if after != before {
		t.Fatal("terminal result recovery started another inference")
	}
}

func TestOrchestratorStructuredOutputFailureRecordsWithoutNewInference(t *testing.T) {
	e, p := repositoryFixture(t)
	ctx := orchestratorExecutionActor(t)
	view, err := e.api.deps.Product.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	turn, _, _, err := e.api.deps.Product.ReserveOrchestratorTurn(ctx, p.ID, product.OrchestratorTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "structured-output-failed", Message: "Draft the saved specification.", RouteDigest: strings.Repeat("a", 64)}, "assist-structured-failed")
	if err != nil {
		t.Fatal(err)
	}
	job := assistance.Job{ID: turn.JobID, ProjectID: p.ID, Status: "failed", Kind: "orchestrator", OrchestratorTurnID: turn.ID, FailureCode: "local_model_structured_output_invalid"}
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	if err := e.api.completeOrchestratorJob(job); err != nil {
		t.Fatal("terminal failure did not settle the conversation", err)
	}
	if err := e.api.completeOrchestratorJob(job); err != nil {
		t.Fatal("duplicate completion was not idempotent", err)
	}
	saved, err := e.api.deps.Product.OrchestratorTurn(p.ID, turn.ID)
	if err != nil || saved.Status != "failed" || saved.FailureCode != job.FailureCode || saved.Answer != nil {
		t.Fatal("terminal result remained unconfirmed", saved, err)
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if after != before {
		t.Fatal("terminal failure started a new provider attempt")
	}
}

func TestOrchestratorCancellationWhileWaitingForProviderNeverAllocates(t *testing.T) {
	e, p := repositoryFixture(t)
	route := saveAssistanceGateway(t, e)
	body := orchestratorRequestFor(t, e, p.ID, "Continue this project.", "cancel-before-provider", "", route)
	data, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", sessionsTarget("/v1/product/projects/"+p.ID+"/assist/orchestrator"), bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(orchestratorExecutionActor(t))
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	e.api.codexAccountMu.Lock()
	locked := true
	defer func() {
		if locked {
			e.api.codexAccountMu.Unlock()
		}
	}()
	done := make(chan int, 1)
	go func() { w := httptest.NewRecorder(); e.mux.ServeHTTP(w, r); done <- w.Code }()
	var job assistance.Job
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		jobs, err := e.api.projectAssistance(p.ID)
		if err == nil && len(jobs) == 1 {
			job = jobs[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job.ID == "" {
		t.Fatal("turn was not durably reserved before waiting for provider")
	}
	w := e.do(t, "POST", "/v1/product/projects/"+p.ID+"/assist/"+job.ID+"/cancel", `{}`)
	if w.Code != 200 {
		t.Fatal("queued cancellation failed", w.Code, w.Body.String())
	}
	e.api.codexAccountMu.Unlock()
	locked = false
	select {
	case status := <-done:
		if status != 409 {
			t.Fatal("canceled admission did not fail", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled admission did not finish")
	}
	saved, err := e.api.assistanceStore().Read(job.ID)
	if err != nil || saved.Status != "cancelled" || saved.SessionID != "" {
		t.Fatal("admission overwrote cancellation", err)
	}
	turn, err := e.api.deps.Product.OrchestratorTurn(p.ID, job.OrchestratorTurnID)
	if err != nil || turn.Status != "cancelled" {
		t.Fatal("conversation cancellation was overwritten", err)
	}
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if after != before {
		t.Fatal("cancelled queued turn allocated provider runtime")
	}
}
