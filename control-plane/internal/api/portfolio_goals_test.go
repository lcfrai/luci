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

func TestPortfolioGoalsAPIBindsCurrentActorAndExactProjectControlAuthority(t *testing.T) {
	e := sessionsNewEnv(t)
	s, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e.api.deps.Product = s
	request := httptest.NewRequest("GET", sessionsTarget("/v1/product/portfolio-goals"), nil).WithContext(orchestratorExecutionActor(t))
	if _, err := e.api.portfolioGoalAccess(httptest.NewRequest("GET", sessionsTarget("/v1/product/portfolio-goals"), nil)); err == nil {
		t.Fatal("anonymous actor accepted")
	}
	actor, _, _ := management.ActorFromContext(request.Context())
	e.api.deps.AuthorizeProjectRead = func(_ context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if ref.UserID != actor.ID {
			t.Fatal("substituted actor")
		}
		return management.ExecutionAuthority{}, nil
	}
	access, err := e.api.portfolioGoalAccess(request)
	if err != nil {
		t.Fatal(err)
	}
	id := e.loc.projects[0].ID
	if access(request.Context(), id, false) != nil {
		t.Fatal("read-only goal unavailable")
	}
	if access(request.Context(), id, true) == nil {
		t.Fatal("missing current control authorizer was inferred from read access")
	}
	checked := 0
	e.api.deps.AuthorizeExecutionControl = func(_ context.Context, ref management.ExecutionActorRef) error {
		checked++
		if ref.UserID != actor.ID || ref.ProjectID != id || ref.Local {
			t.Fatal("control authority lost exact authenticated scope", ref)
		}
		return management.ErrExecutionAuthority
	}
	if access(request.Context(), id, true) == nil || checked != 1 {
		t.Fatal("queue control denial bypassed")
	}
	if access(request.Context(), "p-unlisted", true) == nil || checked != 1 {
		t.Fatal("unlisted project reached control authorization")
	}
}

func TestPortfolioGoalsAPIPreservesDraftConflictAndRejectsForeignProjectOrMalformedBody(t *testing.T) {
	e := sessionsNewEnv(t)
	s, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e.api.deps.Product = s
	goal := product.PortfolioGoal{Title: "Ship appointments", Outcome: "Customers can book a slot", Projects: []product.PortfolioGoalProject{{ProjectID: e.loc.projects[0].ID, Priority: "normal", DependsOn: []string{}}}}
	call := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", sessionsTarget("/v1/product/portfolio-goals"), strings.NewReader(body)).WithContext(orchestratorExecutionActor(t))
		w := httptest.NewRecorder()
		e.api.handlePortfolioGoals(w, r)
		return w
	}
	encoded, _ := json.Marshal(product.PortfolioGoalsSave{Goal: goal})
	w := call(string(encoded))
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	w = call(string(encoded))
	if w.Code != 409 {
		t.Fatal("stale private ledger save accepted", w.Code)
	}
	goal.Projects[0].ProjectID = "p-unlisted"
	encoded, _ = json.Marshal(product.PortfolioGoalsSave{ExpectedRevision: 1, Goal: goal})
	if w = call(string(encoded)); w.Code != 403 {
		t.Fatal("foreign selected project accepted", w.Code)
	}
	if w = call(`{"unexpected":"field"}`); w.Code != 400 {
		t.Fatal("unknown request fields accepted", w.Code)
	}
	if w = call(`{} {}`); w.Code != 400 {
		t.Fatal("multiple request bodies accepted", w.Code)
	}
}
