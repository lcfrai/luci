package product

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func runOrchestratorActor() context.Context {
	var ctx context.Context
	h := contextHistoryActorHandler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() }), management.User{ID: "run-orchestrator-owner", Username: "owner", DisplayName: "Context owner", Role: "member"})
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	return ctx
}

func reserveRunOrchestratorQuestion(t *testing.T, s *Server, projectID string) OrchestratorTurn {
	t.Helper()
	ctx := runOrchestratorActor()
	view, err := s.OrchestratorView(ctx, projectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	turn, _, existing, err := s.ReserveOrchestratorTurn(ctx, projectID, OrchestratorTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "orchestrator-run-admission-fixture", Message: "Review this product decision.", RouteDigest: strings.Repeat("b", 64)}, "assist-run-admission-fixture")
	if err != nil || existing {
		t.Fatal(existing, err)
	}
	return turn
}

func runOrchestratorQuestionAnswer() *OrchestratorAnswer {
	return &OrchestratorAnswer{Summary: "The requested access model needs a decision.", Question: "Should teammates share all saved notes?", Rationale: "This changes the reviewed access rules.", Blocking: true, Options: []string{"Share team notes", "Keep notes private"}, Impact: []string{"Definition"}, NextAction: "answer-question"}
}

func TestProjectRunOrchestratorQuestionBlocksNewWorkButKeepsHistoryAndCancellation(t *testing.T) {
	s, mux, p, req, _, calls := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201))
	opRequest := map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "run-before-material-question", "kind": "session"}
	reserved := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", opRequest, 201))
	turn := reserveRunOrchestratorQuestion(t, s, p.ID)
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", runOrchestratorQuestionAnswer()); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	newRequest := req
	newRequest.IdempotencyKey = "new-run-after-material-question"
	w := journeyRequest(t, mux, "POST", base, newRequest, 409)
	if !strings.Contains(w.Body.String(), ErrOrchestratorQuestion.Error()) {
		t.Fatal(w.Body.String())
	}
	w = journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", map[string]any{"expectedRevision": reserved.Revision, "idempotencyKey": "next-op-after-material-question", "kind": "session"}, 409)
	if !strings.Contains(w.Body.String(), ErrOrchestratorQuestion.Error()) || calls.Load() != before {
		t.Fatal("blocked work resolved a provider route", w.Body.String())
	}
	journeyRequest(t, mux, "GET", base, nil, 200)
	journeyRequest(t, mux, "GET", base+"/"+run.ID, nil, 200)
	// Existing receipts may be recovered without rebinding inputs or dispatching.
	journeyRequest(t, mux, "POST", base, req, 200)
	journeyRequest(t, mux, "POST", base+"/"+run.ID+"/operations", opRequest, 200)
	cancelled := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/"+run.ID+"/cancel", map[string]int{"expectedRevision": reserved.Revision}, 200))
	if cancelled.State != "cancelled" || len(cancelled.Operations) != 1 {
		t.Fatal(cancelled)
	}
	if !errors.Is(s.OrchestratorExecutionGate(p.ID), ErrOrchestratorQuestion) {
		t.Fatal("run cancellation cleared product ambiguity")
	}
}

func TestProjectRunOrchestratorQuestionPublicationWaitsForExistingExecutionAdmission(t *testing.T) {
	s, _, p, _, _, _ := projectRunHTTPFixture(t)
	turn := reserveRunOrchestratorQuestion(t, s, p.ID)
	release, err := s.AcquireGitSessionAdmission(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	started, finished := make(chan struct{}), make(chan error, 1)
	go func() {
		close(started)
		finished <- s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", runOrchestratorQuestionAnswer())
	}()
	<-started
	select {
	case err := <-finished:
		t.Fatalf("question publication raced admitted backend allocation: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	if err := s.OrchestratorExecutionGate(p.ID); err != nil {
		t.Fatal("unpublished question blocked an already admitted operation", err)
	}
	release()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("question completion did not resume after allocation admission")
	}
	if !errors.Is(s.OrchestratorExecutionGate(p.ID), ErrOrchestratorQuestion) {
		t.Fatal("published question failed to fence the next operation")
	}
}
