package management

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPortfolioGoalResponsesRecheckOriginalProjectsIncludingNoActionApplications(t *testing.T) {
	data := []byte(`{"goals":[{"projects":[{"projectId":"p-current"}]}],"applications":[{"projectIds":["p-original"],"results":[]}]}`)
	ids, err := portfolioGoalResponseProjects(data)
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if err != nil || len(ids) != 2 || !seen["p-current"] || !seen["p-original"] {
		t.Fatal("retargeting or empty actions hid original project ownership", ids, err)
	}
	for _, malformed := range []string{
		`{"projectIds":["p-one"],"results":[{"action":{"projectId":"p-hidden"}}]}`,
		`{"allocations":[{"projectId":"p-one"}],"actions":[{"projectId":"p-hidden"}]}`,
		`{"projectIds":[],"results":[]}`,
		`{"unrecognized":"response"}`,
	} {
		if _, err := portfolioGoalResponseProjects([]byte(malformed)); err == nil {
			t.Fatal("unverifiable response accepted", malformed)
		}
	}
}

func TestPortfolioGoalsCollectionDefersEachWriteToProjectControlAndRechecksResponseRead(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{"GET", "/v1/product/portfolio-goals"}, {"POST", "/v1/product/portfolio-goals"},
		{"POST", "/v1/product/portfolio-goals/goal-one/review"}, {"POST", "/v1/product/portfolio-goals/goal-one/apply"},
		{"GET", "/v1/product/portfolio-goals/applications/portfolio-one"},
	} {
		r := httptest.NewRequest(route.method, route.path, nil)
		scope, ok := ClassifyTeamRoute(r)
		if !ok || scope.Collection != "portfolio-goals" || scope.ProjectID != "" ||
			!authorizeTeamRoute(httptest.NewRecorder(), r, TeamPermissions{Read: true}) || authorizeTeamRoute(httptest.NewRecorder(), r, TeamPermissions{}) {
			t.Fatal("wrong portfolio collection scope", route, scope)
		}
	}
	f := newTeamProxyFixture(t)
	capture := newTeamCapture()
	capture.WriteHeader(200)
	_, _ = capture.Write([]byte(`{"projectIds":["p-alpha"],"results":[],"private":"retained goal detail"}`))
	f.g.mu.Lock()
	row := f.g.state.Teams[f.team.ID]
	delete(row.Projects["p-alpha"], f.member.ID)
	f.g.state.Teams[f.team.ID] = row
	f.g.mu.Unlock()
	w := httptest.NewRecorder()
	f.g.recheckTeamPortfolioGoals(w, capture, f.team.ID, f.member.ID, true)
	if w.Code != 403 || strings.Contains(w.Body.String(), "retained goal detail") || !strings.Contains(w.Body.String(), "not rolled back") {
		t.Fatal("late revocation leaked result or implied rollback", w.Code, w.Body.String())
	}
}
