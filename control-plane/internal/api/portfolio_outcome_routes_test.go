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

func TestPortfolioGoalsGETJoinsOnlyActorAuthorizedWorkWithoutExecutionControl(t *testing.T) {
	e := sessionsNewEnv(t)
	s, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e.api.deps.Product = s
	e.api.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
		t.Fatal("read requested execution control")
		return nil
	}
	goal := product.PortfolioGoal{Title: "Ship appointments", Outcome: "Customers can book", Projects: []product.PortfolioGoalProject{{ProjectID: e.loc.projects[0].ID, Priority: "normal", DependsOn: []string{}}}}
	body, _ := json.Marshal(product.PortfolioGoalsSave{Goal: goal})
	post := httptest.NewRequest("POST", sessionsTarget("/v1/product/portfolio-goals"), strings.NewReader(string(body))).WithContext(orchestratorExecutionActor(t))
	w := httptest.NewRecorder()
	e.api.handlePortfolioGoals(w, post)
	if w.Code != 200 {
		t.Fatal("private goal save", w.Code, w.Body.String())
	}
	get := httptest.NewRequest("GET", sessionsTarget("/v1/product/portfolio-goals"), nil).WithContext(orchestratorExecutionActor(t))
	w = httptest.NewRecorder()
	e.api.handlePortfolioGoals(w, get)
	var result product.PortfolioGoalsView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.OutcomeRouting == nil || len(result.OutcomeRouting.Routes) != 1 {
		t.Fatal("existing GET did not join bounded routes", w.Code, w.Body.String())
	}
	// The fake catalogue deliberately has no physical product repository. It
	// must yield unknown, without turning a missing source into idle/completed.
	route := result.OutcomeRouting.Routes[0]
	if route.ProjectID != goal.Projects[0].ProjectID || route.GoalID != result.Goals[0].ID || route.GoalRevision != result.Goals[0].Revision || route.Status != "unknown" {
		t.Fatal("wrong actor/project goal join", route)
	}
}
