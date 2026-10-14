package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func orchestratorActor(t *testing.T, id string) context.Context {
	t.Helper()
	var ctx context.Context
	handler := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ctx = r.Context() }), management.User{ID: id, Username: id, DisplayName: id, Role: "member"})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "http://localhost/", nil))
	if ctx == nil {
		t.Fatal("trusted actor missing")
	}
	return ctx
}
func reserveOrchestratorTest(t *testing.T, s *Server, id string, ctx context.Context, message, reply string) (OrchestratorTurn, OrchestratorContext) {
	t.Helper()
	view, err := s.OrchestratorView(ctx, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	turn, snapshot, existing, err := s.ReserveOrchestratorTurn(ctx, id, OrchestratorTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: newID("request-"), Message: message, ReplyToTurnID: reply, RouteDigest: strings.Repeat("a", 64)}, newID("assist-"))
	if err != nil || existing {
		t.Fatal("reserve", err, existing)
	}
	return turn, snapshot
}
func TestOrchestratorMemoryLimitFailureIsDurableAndIdempotent(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "orchestrator-memory-limit")
	ctx := orchestratorActor(t, "alice")
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Clarify the feature", "")
	if err := s.CompleteOrchestratorTurnWithFailure(p.ID, turn.ID, turn.JobID, "failed", nil, "memory_limit"); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteOrchestratorTurnWithFailure(p.ID, turn.ID, turn.JobID, "failed", nil, "memory_limit"); err != nil {
		t.Fatal("replay", err)
	}
	if err := s.CompleteOrchestratorTurnWithFailure(p.ID, turn.ID, turn.JobID, "failed", nil, "provider-output"); !errors.Is(err, ErrOrchestratorInvalid) {
		t.Fatal("untrusted cause accepted", err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Turns) != 1 || view.Turns[0].FailureCode != "memory_limit" {
		t.Fatal(view.Turns)
	}
}
func TestOrchestratorLocalModelFailureCompletesWithoutManualReconcile(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "orchestrator-local-model-failure")
	ctx := orchestratorActor(t, "alice")
	for _, code := range []string{"local_model_context_limit", "local_model_context_unverified", "local_model_structured_output_invalid", "local_model_project_answer_invalid"} {
		turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Draft the specification", "")
		if err := s.CompleteOrchestratorTurnWithFailure(p.ID, turn.ID, turn.JobID, "failed", nil, code); err != nil {
			t.Fatal(code, err)
		}
		if err := s.CompleteOrchestratorTurnWithFailure(p.ID, turn.ID, turn.JobID, "failed", nil, code); err != nil {
			t.Fatal("terminal replay", code, err)
		}
		saved, err := s.OrchestratorTurn(p.ID, turn.ID)
		if err != nil || saved.Status != "failed" || saved.FailureCode != code || saved.Answer != nil {
			t.Fatal("failure did not reach durable conversation", code, saved, err)
		}
	}
}

func TestOrchestratorCompleteTaskListRequiresObservableAcceptanceAndNoRoutineQuestion(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "orchard-complete-task-list")
	ctx := orchestratorActor(t, "alice")
	_, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Draft a local task list with add, complete, delete and reload persistence", "")
	specification := snapshot.Specification
	specification.Requirements = []SpecificationRequirement{
		{ID: "req-001", Title: "Add a task", Priority: "must", Acceptance: "must"},
		{ID: "req-002", Title: "Complete a task", Priority: "must", Acceptance: "must"},
		{ID: "req-003", Title: "Delete a task", Priority: "must", Acceptance: "must"},
		{ID: "req-004", Title: "Persist tasks", Priority: "must", Acceptance: "must"},
	}
	answer := OrchestratorAnswer{Summary: "A local task list can be reviewed as a draft.", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Four task behaviors", Specification: &specification}, NextAction: "specification"}
	if err := ValidateOrchestratorAnswer(snapshot, answer); err == nil {
		t.Fatal("placeholder priority labels were accepted as observable criteria")
	}
	criteria := []string{"Adding a named task shows it in the list.", "Toggling a task visibly changes its completion state.", "Deleting a task removes it from the list.", "Reloading restores the remaining saved tasks."}
	for i := range specification.Requirements {
		specification.Requirements[i].Acceptance = criteria[i]
	}
	answer.Question = "Should we make HTML or JavaScript first?"
	answer.Rationale = "Choose a routine implementation order."
	if err := ValidateOrchestratorAnswer(snapshot, answer); err == nil {
		t.Fatal("routine nonblocking question was attached to a complete draft")
	}
	answer.Question, answer.Rationale = "", ""
	if err := ValidateOrchestratorAnswer(snapshot, answer); err != nil {
		t.Fatal("complete observable draft was refused", err)
	}
}
func TestOrchestratorDurableTurnIdentityScopeAndReadOnlyRecovery(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "orchestrator-scope")
	ctx := orchestratorActor(t, "alice")
	snapshot, err := s.OrchestratorSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	req := OrchestratorTurnRequest{ExpectedRevision: 0, ExpectedContextDigest: snapshot.Digest, IdempotencyKey: "same-browser-request", Message: "Build a private team notebook", RouteDigest: strings.Repeat("a", 64)}
	turn, _, existing, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-owned-first")
	if err != nil || existing {
		t.Fatal(err)
	}
	again, _, existing, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-must-not-allocate")
	if err != nil || !existing || again.ID != turn.ID || again.JobID != "assist-owned-first" {
		t.Fatal("duplicate allocated new ownership", err)
	}
	req.Message = "Different paid work"
	if _, _, _, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-other"); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("key reuse accepted", err)
	}
	answer := OrchestratorAnswer{Summary: "The saved understanding is still a draft.", Question: "Who can read notes?", Rationale: "Visibility determines access control.", Options: []string{"Only the author", "Team members"}, Blocking: true, Impact: []string{"Data permissions"}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal("completion replay", err)
	}
	path, err := s.orchestratorPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	view, err := restarted.OrchestratorView(ctx, p.ID, 0)
	if err != nil || len(view.Turns) != 1 || view.Turns[0].Actor.ID != "alice" || len(view.BlockingQuestions) != 1 {
		t.Fatal("restart lost discussion", err)
	}
	after, _ := os.ReadFile(path)
	afterInfo, _ := os.Stat(path)
	if string(before) != string(after) || !info.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("GET repaired/wrote history")
	}
	other := createJourneyProject(t, mux, "orchestrator-other")
	destination, err := s.orchestratorPath(other.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OrchestratorView(ctx, other.ID, 0); err == nil {
		t.Fatal("copied private conversation adopted")
	}
	if _, err := os.Stat(filepath.Join(p.Path, ".luci", "orchestrator.json")); !os.IsNotExist(err) {
		t.Fatal("private conversation exported to project")
	}
}
func TestOrchestratorFollowupQuestionReplacesResolvedParent(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "followup-questions")
	ctx := orchestratorActor(t, "alice")
	first, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Clarify sharing", "")
	a := OrchestratorAnswer{Summary: "Clarify sharing.", Question: "Private or team?", Blocking: true, Options: []string{}, Impact: []string{}}
	if err := s.CompleteOrchestratorTurn(p.ID, first.ID, first.JobID, "succeeded", &a); err != nil {
		t.Fatal(err)
	}
	second, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Team shared", first.ID)
	a.Question = "May viewers export notes?"
	if err := s.CompleteOrchestratorTurn(p.ID, second.ID, second.JobID, "succeeded", &a); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || len(view.BlockingQuestions) != 1 || view.BlockingQuestions[0].TurnID != second.ID {
		t.Fatal("answered parent stayed blocked", err)
	}
	third, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "No, editors only", second.ID)
	a.Blocking = false
	a.Question = ""
	if err := s.CompleteOrchestratorTurn(p.ID, third.ID, third.JobID, "succeeded", &a); err != nil {
		t.Fatal(err)
	}
	if err := s.OrchestratorExecutionGate(p.ID); err != nil {
		t.Fatal("follow-up resolution trapped work", err)
	}
}
func TestOrchestratorAppliesAllAmendmentTargetsWithCASAndPrivateReview(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "review-amendments")
	ctx := orchestratorActor(t, "alice")
	mux := http.NewServeMux()
	s.Register(mux)
	handler := contextHistoryActorHandler(mux, management.User{ID: "alice", Username: "alice", DisplayName: "Alice", Role: "member"})
	base := "/v1/product/projects/" + p.ID
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Build a shared notebook", "")
	spec := snapshot.Specification
	spec.Purpose = "A shared notebook"
	spec.Audience = "Editors"
	spec.Requirements = []SpecificationRequirement{{ID: "AC-notes", Title: "Shared notes", Priority: "must", Acceptance: "Editors can save and reload a note"}}
	answer := OrchestratorAnswer{Summary: "The proposed scope describes shared notes.", Options: []string{}, Impact: []string{"Definition review is required"}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Define notes", Specification: &spec}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, handler, "POST", base+"/orchestrator/turns/"+turn.ID+"/apply", map[string]any{"expectedRevision": view.Revision - 1, "expectedContextDigest": view.Context.Digest}, 409)
	var applied OrchestratorView
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/orchestrator/turns/"+turn.ID+"/apply", map[string]any{"expectedRevision": view.Revision, "expectedContextDigest": view.Context.Digest}, 200).Body.Bytes(), &applied)
	if applied.Context.Specification.Status != "draft" || applied.Context.Specification.Revision != 1 || applied.Context.Brief.Revision != 1 {
		t.Fatal("amendment invented approval")
	}
	replay := journeyRequest(t, handler, "POST", base+"/orchestrator/turns/"+turn.ID+"/apply", map[string]any{"expectedRevision": view.Revision, "expectedContextDigest": view.Context.Digest}, 200)
	var repeated OrchestratorView
	decodeEvidenceResponse(t, replay.Body.Bytes(), &repeated)
	if repeated.Context.Specification.Revision != 1 {
		t.Fatal("lost response doubled apply")
	}
	journeyRequest(t, handler, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 1}, 200)
	designTurn, designContext := reserveOrchestratorTest(t, s, p.ID, ctx, "Plan the notebook screens", "")
	design := designContext.DesignPlan
	design.Screens = []DesignScreen{{ID: "notes", Name: "Team notes", Purpose: "Write and read notes", RequirementIDs: []string{"AC-notes"}, States: []string{"loading", "empty", "ready", "error"}, Widths: []int{390, 1440}}}
	answer.Proposal = &OrchestratorProposal{Target: "design-plan", Summary: "Design accessible notes", DesignPlan: &design}
	if err := s.CompleteOrchestratorTurn(p.ID, designTurn.ID, designTurn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, _ = s.OrchestratorView(ctx, p.ID, 0)
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/orchestrator/turns/"+designTurn.ID+"/apply", map[string]any{"expectedRevision": view.Revision, "expectedContextDigest": view.Context.Digest}, 200).Body.Bytes(), &applied)
	if applied.Context.DesignPlan.Status != "draft" || applied.Context.Specification.Status != "approved" {
		t.Fatal("design changed definition authority")
	}
	planTurn, planContext := reserveOrchestratorTest(t, s, p.ID, ctx, "Prepare implementation tasks", "")
	plan := planContext.Plan
	plan.Tasks = []ProjectTask{{ID: "notes-task", Title: "Implement shared notes", Status: "todo", RequirementIDs: []string{"AC-notes"}}}
	answer.Proposal = &OrchestratorProposal{Target: "plan", Summary: "Build shared notes", Plan: &plan}
	if err := s.CompleteOrchestratorTurn(p.ID, planTurn.ID, planTurn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, _ = s.OrchestratorView(ctx, p.ID, 0)
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/orchestrator/turns/"+planTurn.ID+"/apply", map[string]any{"expectedRevision": view.Revision, "expectedContextDigest": view.Context.Digest}, 200).Body.Bytes(), &applied)
	if applied.Context.Plan.Revision != 1 || applied.Context.Plan.SpecificationDigest == "" || applied.Context.Plan.Tasks[0].Status != "todo" {
		t.Fatal("task amendment lost review binding")
	}
	var history struct {
		Events []ContextChangeEvent `json:"events"`
	}
	data := journeyRequest(t, handler, "GET", base+"/context-history", nil, 200).Body.Bytes()
	if json.Unmarshal(data, &history) != nil || len(history.Events) < 4 {
		t.Fatal("amendments omitted private actor attribution")
	}
}

func TestOrchestratorApplyRecoversCanonicalSaveWithoutReapplyingAfterExportFailure(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "amendment-export-failure")
	ctx := orchestratorActor(t, "alice")
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Create an accessible notebook", "")
	spec := snapshot.Specification
	spec.Purpose = "An accessible notebook"
	spec.Audience = "Editors"
	answer := OrchestratorAnswer{Summary: "Propose the notebook definition.", Options: []string{}, Impact: []string{"Review the saved definition"}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Define the notebook", Specification: &spec}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(p.Path, ".luci/design.json")
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(target); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err = os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err == nil || !strings.Contains(err.Error(), "may have been saved") {
		t.Fatal("failed generated export hid uncertain outcome", err)
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || state.Specification == nil || state.Specification.Revision != 1 || state.Specification.Status != "draft" {
		t.Fatal("canonical amendment wasn't durable", err)
	}
	before, _ := os.ReadFile(filepath.Join(p.Path, ".luci/state.json"))
	ledger, err := s.readOrchestrator(p)
	if err != nil || ledger.Turns[0].ApplyIntent == "" || ledger.Turns[0].AppliedRevision != 0 {
		t.Fatal("missing write-ahead recovery intent", err)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	// The original mutation retry finalizes only the observed canonical target.
	if err = restarted.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal("could not finalize exact persisted amendment", err)
	}
	after, _ := os.ReadFile(filepath.Join(p.Path, ".luci/state.json"))
	if string(before) != string(after) {
		t.Fatal("recovery applied the amendment twice")
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatal("receipt recovery silently repaired exports", err)
	}
	recovered, err := restarted.OrchestratorView(ctx, p.ID, 0)
	if err != nil || recovered.Turns[0].AppliedRevision != 1 {
		t.Fatal("recovered receipt missing", err)
	}
}

func TestOrchestratorAmendmentRejectsConcurrentHumanContextChange(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "concurrent-amendment")
	ctx := orchestratorActor(t, "alice")
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Propose a notebook", "")
	spec := snapshot.Specification
	spec.Purpose = "Assistant suggestion"
	spec.Audience = "Editors"
	answer := OrchestratorAnswer{Summary: "Review the draft suggestion.", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Draft definition", Specification: &spec}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/brief", map[string]any{"revision": 0, "purpose": "A teammate's newer purpose"}, 200)
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("assistant overwrote newer context", err)
	}
	state, err := readJourney(p.Path, p.ID)
	if err != nil || state.Brief.Purpose != "A teammate's newer purpose" {
		t.Fatal("human change was lost", err)
	}
}
