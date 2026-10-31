package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestPortfolioFocusModesUseExistingRoutesAndCurrentProjectControl(t *testing.T) {
	e := sessionsNewEnv(t)
	s, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e.api.deps.Product = s
	ctx := orchestratorExecutionActor(t)
	id := e.loc.projects[0].ID
	goal, err := s.SavePortfolioGoal(ctx, product.PortfolioGoalsSave{Goal: product.PortfolioGoal{Title: "Focus", Outcome: "Book an appointment", Projects: []product.PortfolioGoalProject{{ProjectID: id, Priority: "normal", RequestedSlots: 1, DependsOn: []string{}}}}}, func(context.Context, string, bool) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	reviewRequest := httptest.NewRequest("POST", sessionsTarget("/v1/product/portfolio-goals/"+goal.ID+"/review"), strings.NewReader(`{"mode":"focus","expectedGoalRevision":1,"focus":{"upstreamRuns":[]}}`)).WithContext(ctx)
	reviewRequest.SetPathValue("goalId", goal.ID)
	w := httptest.NewRecorder()
	e.api.handlePortfolioGoalReview(w, reviewRequest)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var plan product.PortfolioFocusPlan
	if json.Unmarshal(w.Body.Bytes(), &plan) != nil || plan.Profile.GoalID != goal.ID || plan.ExecutionStarted {
		t.Fatal("wrong focus review", w.Body.String())
	}
	request := product.PortfolioGoalApply{Mode: "focus-install", GoalID: goal.ID, ExpectedGoalRevision: 1, ExpectedPlanDigest: plan.Digest, IdempotencyKey: "api-explicit-focus", Focus: &product.PortfolioFocusRequest{ExpectedRevision: 0}}
	body, _ := json.Marshal(request)
	apply := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", sessionsTarget("/v1/product/portfolio-goals/"+goal.ID+"/apply"), strings.NewReader(string(body))).WithContext(ctx)
		r.SetPathValue("goalId", goal.ID)
		w := httptest.NewRecorder()
		e.api.handlePortfolioGoalApply(w, r)
		return w
	}
	if response := apply(); response.Code != 403 {
		t.Fatal("read access became focus control", response.Code, response.Body.String())
	}
	checks := 0
	e.api.deps.AuthorizeExecutionControl = func(_ context.Context, actor management.ExecutionActorRef) error {
		checks++
		if actor.ProjectID != id || actor.UserID == "" || actor.Local {
			t.Fatal("focus write lost identity", actor)
		}
		return nil
	}
	if response := apply(); response.Code != 200 || checks != 2 || !strings.Contains(response.Body.String(), `"revision":1`) {
		t.Fatal("explicit control installation failed", response.Code, response.Body.String(), checks)
	}
}
