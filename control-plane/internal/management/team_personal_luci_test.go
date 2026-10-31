package management

import (
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

func TestPersonalTeamRoutesCarryOnlyCollectionReadBeforePerProjectAdmission(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"GET", "/v1/product/personal-luci"}, {"GET", "/v1/product/personal-luci/export"}, {"GET", "/v1/product/personal-luci/turns/personal-one"}, {"PUT", "/v1/product/personal-luci/preferences"}, {"POST", "/v1/product/personal-luci/continue"}, {"POST", "/v1/product/personal-luci/turns"}, {"POST", "/v1/product/personal-luci/turns/personal-one/handoff"}} {
		scope, required, ok := classifyTeamRoute(httptest.NewRequest(route.method, route.path, nil))
		if !ok || scope.Collection != "personal-luci" || scope.ProjectID != "" || required != teamRead {
			t.Fatal("collection route invented project execution authority", route, scope, required)
		}
	}
}
func TestPersonalResponseGrantSetIncludesHistoricalComputeGoalAndHandoffSources(t *testing.T) {
	raw := []byte(`{"preferences":{"computeProjectId":"compute-one","projectIds":["project-one"]},"turns":[{"context":{"preferences":{"computeProjectId":"compute-old","projectIds":["project-old"]},"goals":[{"projects":[{"projectId":"goal-source"}]}]},"handoff":{"projectId":"destination"}},{"withheld":true,"context":{"preferences":{"computeProjectId":"","projectIds":null}}}],"message":"{\"projectId\":\"not-a-source\"}"}`)
	ids, err := personalResponseProjects(raw)
	sort.Strings(ids)
	want := []string{"compute-old", "compute-one", "destination", "goal-source", "project-old", "project-one"}
	if err != nil || !reflect.DeepEqual(ids, want) {
		t.Fatal("missing original source grant or prompt text became authority", ids, err)
	}
	if _, err := personalResponseProjects([]byte(`{"projectIds":["../foreign"]}`)); err == nil {
		t.Fatal("unsafe reference accepted")
	}
}
