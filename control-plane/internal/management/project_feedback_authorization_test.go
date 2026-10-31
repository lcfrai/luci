package management

import (
	"net/http/httptest"
	"testing"
)

func TestFeedbackRoutesRequireReadEditSecretsButNotExecuteOrDeploy(t *testing.T) {
	base := "/v1/product/projects/p-one/feedback"
	reader := TeamPermissions{Read: true}
	editor := TeamPermissions{Read: true, Edit: true}
	fetcher := TeamPermissions{Read: true, Edit: true, Secrets: true}
	for _, suffix := range []string{"", "/preview", "/import", "/selection"} {
		method := "POST"
		if suffix == "" {
			method = "GET"
		}
		request := httptest.NewRequest(method, base+suffix, nil)
		scope, ok := ClassifyTeamRoute(request)
		if !ok || scope.ProjectID != "p-one" || scope.Collection != "" {
			t.Fatal("wrong project feedback boundary")
		}
		if !authorizeTeamRoute(httptest.NewRecorder(), request, fetcher) || authorizeTeamRoute(httptest.NewRecorder(), request, reader) != (suffix == "") || authorizeTeamRoute(httptest.NewRecorder(), request, editor) != (suffix == "") {
			t.Fatal("feedback authority drift", suffix)
		}
		request = httptest.NewRequest("DELETE", base+suffix, nil)
		if authorizeTeamRoute(httptest.NewRecorder(), request, fetcher) {
			t.Fatal("unknown method admitted")
		}
	}
}
