package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func coordinatorPolicyFixture(t *testing.T) (*Server, project.Project, http.Handler, context.Context) {
	t.Helper()
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "coordinator-policy")
	s.SetCoordinatorAuthorizer(func(ctx context.Context, id string) error {
		if id != p.ID {
			return errors.New("foreign")
		}
		return ctx.Err()
	})
	mux := http.NewServeMux()
	s.Register(mux)
	return s, p, contextHistoryActorHandler(mux, management.User{ID: "alice", Username: "alice", DisplayName: "Alice", Role: "member"}), orchestratorActor(t, "alice")
}
func coordinatorProposal(t *testing.T, s *Server, p project.Project, ctx context.Context, blocking bool) (OrchestratorTurn, OrchestratorView) {
	t.Helper()
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Make a shared accessible notebook", "")
	spec := snapshot.Specification
	spec.Purpose = "A shared accessible notebook"
	spec.Audience = "Editors"
	answer := OrchestratorAnswer{Summary: "Draft the notebook scope", Options: []string{}, Impact: []string{"Review the specification"}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Notebook", Specification: &spec}, Blocking: blocking}
	if blocking {
		answer.Question = "Should outside guests be allowed?"
	}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	return turn, view
}
func coordinatorSavePolicy(t *testing.T, h http.Handler, id string, revision int, mode string, targets []string) CoordinatorPolicy {
	t.Helper()
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+id+"/orchestrator/policy", map[string]any{"expectedRevision": revision, "mode": mode, "allowedTargets": targets}, 200).Body.Bytes(), &policy)
	return policy
}
func coordinatorAdvanceBody(view OrchestratorView, revision int) map[string]any {
	return map[string]any{"expectedRevision": view.Revision, "expectedContextDigest": view.Context.Digest, "expectedPolicyRevision": revision}
}

func TestCoordinatorPolicyDefaultsOptInAndExactDraftRestartReadback(t *testing.T) {
	s, p, h, ctx := coordinatorPolicyFixture(t)
	base := "/v1/product/projects/" + p.ID + "/orchestrator"
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", base+"/policy", nil, 200).Body.Bytes(), &policy)
	if policy.Mode != "review" || policy.Revision != 0 {
		t.Fatalf("unsafe default %#v", policy)
	}
	turn, view := coordinatorProposal(t, s, p, ctx, false)
	route := base + "/turns/" + turn.ID + "/advance"
	journeyRequest(t, h, "POST", route, coordinatorAdvanceBody(view, 1), 409)
	policy = coordinatorSavePolicy(t, h, p.ID, 0, "auto-drafts", []string{"specification"})
	var applied OrchestratorView
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", route, coordinatorAdvanceBody(view, policy.Revision), 200).Body.Bytes(), &applied)
	if applied.Context.Specification.Revision != 1 || applied.Context.Specification.Status != "draft" || applied.Context.Specification.Digest != "" || applied.Turns[0].AppliedRevision != 1 {
		t.Fatal("draft invented approval or lost attribution", applied)
	}
	path, err := s.orchestratorPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.SetCoordinatorAuthorizer(func(ctx context.Context, id string) error { return ctx.Err() })
	mux := http.NewServeMux()
	restarted.Register(mux)
	again := contextHistoryActorHandler(mux, management.User{ID: "alice", Username: "alice", Role: "member"})
	journeyRequest(t, again, "POST", route, coordinatorAdvanceBody(view, policy.Revision), 200)
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("response-loss replay rewrote amendment", err)
	}
	got, err := restarted.readCoordinatorPolicy(p)
	if err != nil || got.Revision != policy.Revision || got.Actor.ID != "alice" {
		t.Fatal("policy lost across restart", err)
	}
}
func TestCoordinatorPolicyRefusesScopeStalenessQuestionsAndOtherActor(t *testing.T) {
	for _, kind := range []string{"target", "context", "policy", "question", "actor", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			s, p, h, ctx := coordinatorPolicyFixture(t)
			turn, view := coordinatorProposal(t, s, p, ctx, kind == "question")
			targets := []string{"specification"}
			if kind == "target" {
				targets = []string{"plan"}
			}
			policy := coordinatorSavePolicy(t, h, p.ID, 0, "auto-drafts", targets)
			body := coordinatorAdvanceBody(view, policy.Revision)
			status := 409
			if kind == "context" {
				body["expectedContextDigest"] = strings.Repeat("f", 64)
			}
			if kind == "policy" {
				coordinatorSavePolicy(t, h, p.ID, policy.Revision, "review", targets)
			}
			if kind == "actor" {
				mux := http.NewServeMux()
				s.Register(mux)
				h = contextHistoryActorHandler(mux, management.User{ID: "bob", Username: "bob", Role: "member"})
				status = 403
			}
			if kind == "cancelled" {
				s.SetCoordinatorAuthorizer(func(context.Context, string) error { return context.Canceled })
				status = 403
			}
			path, _ := s.orchestratorPath(p.ID, false)
			before, _ := os.ReadFile(path)
			journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/turns/"+turn.ID+"/advance", body, status)
			after, _ := os.ReadFile(path)
			if string(before) != string(after) {
				t.Fatal("refusal mutated conversation")
			}
			state, err := s.readApprovedJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state.Specification != nil && state.Specification.Revision > 0 {
				t.Fatal("refusal saved amendment")
			}
		})
	}
}
func TestCoordinatorPolicyReauthorizesAfterWaitingForApplyLock(t *testing.T) {
	s, p, h, ctx := coordinatorPolicyFixture(t)
	turn, view := coordinatorProposal(t, s, p, ctx, false)
	policy := coordinatorSavePolicy(t, h, p.ID, 0, "auto-drafts", []string{"specification"})
	var allowed atomic.Bool
	allowed.Store(true)
	var calls atomic.Int32
	entered := make(chan struct{})
	s.SetCoordinatorAuthorizer(func(context.Context, string) error {
		if calls.Add(1) == 1 {
			close(entered)
		}
		if !allowed.Load() {
			return errors.New("revoked")
		}
		return nil
	})
	s.mu.Lock()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRequest("POST", "/v1/product/projects/"+p.ID+"/orchestrator/turns/"+turn.ID+"/advance", coordinatorTestBody(coordinatorAdvanceBody(view, policy.Revision)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		done <- w
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		s.mu.Unlock()
		t.Fatal("no entry")
	}
	allowed.Store(false)
	s.mu.Unlock()
	select {
	case w := <-done:
		if w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocked after lock released")
	}
	got, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || got.Revision != view.Revision {
		t.Fatal("revoked request changed ledger", err)
	}
}

func coordinatorTestBody(v any) *bytes.Reader { raw, _ := json.Marshal(v); return bytes.NewReader(raw) }
