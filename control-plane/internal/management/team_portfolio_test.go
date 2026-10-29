package management

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTeamPortfolioFiltersInterventionsAndCountsWithRevokedProject(t *testing.T) {
	raw := []byte(`{"version":1,"status":"ready","totalProjects":2,"omittedProjects":0,"omittedInterventions":0,"projects":[{"id":"p-allowed","name":"Allowed"},{"id":"p-revoked","name":"Hidden name"}],"interventions":[{"id":"one","projectId":"p-allowed"},{"id":"two","projectId":"p-revoked","reason":"Hidden reason"}]}`)
	got, err := filterPortfolioRows(raw, func(id string) bool { return id == "p-allowed" })
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "Hidden") || strings.Contains(string(data), "p-revoked") || got.TotalProjects != 1 || len(got.Interventions) != 1 || got.Status != "partial" {
		t.Fatalf("revoked content/count leak: %s", data)
	}
}

func TestTeamPortfolioRejectsOrphanOrDuplicateProjectEvidence(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"status":"ready","totalProjects":1,"projects":[{"id":"p-one"}],"interventions":[{"id":"one","projectId":"p-hidden"}]}`,
		`{"version":1,"status":"ready","totalProjects":2,"projects":[{"id":"p-one"},{"id":"p-one"}],"interventions":[]}`,
	} {
		if _, err := filterPortfolioRows([]byte(raw), func(string) bool { return true }); err == nil {
			t.Fatal("ambiguous source accepted")
		}
	}
}

func TestPortfolioAndBlueprintTeamRoutesNeedReadOnlyAuthority(t *testing.T) {
	for _, route := range []struct{ method, path, collection, project string }{
		{"GET", "/v1/product/portfolio", "portfolio", ""},
		{"POST", "/v1/product/projects/p-one/deployment-blueprint", "", "p-one"},
	} {
		r := httptest.NewRequest(route.method, route.path, nil)
		scope, ok := ClassifyTeamRoute(r)
		if !ok || scope.Collection != route.collection || scope.ProjectID != route.project ||
			!authorizeTeamRoute(httptest.NewRecorder(), r, TeamPermissions{Read: true}) || authorizeTeamRoute(httptest.NewRecorder(), r, TeamPermissions{}) {
			t.Fatalf("wrong route authority: %+v", scope)
		}
	}
}
