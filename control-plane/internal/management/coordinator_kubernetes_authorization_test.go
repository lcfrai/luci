package management

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoordinatorKubernetesPreparationRequiresProjectEditDeployAndSecrets(t *testing.T) {
	path := "/v1/product/projects/p-one/orchestrator/kubernetes-plan"
	full := TeamPermissions{Read: true, Edit: true, Deploy: true, Secrets: true}
	for _, row := range []struct {
		permissions TeamPermissions
		allowed     bool
	}{{full, true}, {TeamPermissions{Read: true, Edit: true, Deploy: true}, false}, {TeamPermissions{Read: true, Edit: true, Secrets: true}, false}, {TeamPermissions{Read: true, Deploy: true, Secrets: true}, false}, {TeamPermissions{Read: true, Execute: true}, false}} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		scope, ok := ClassifyTeamRoute(request)
		if !ok || scope.ProjectID != "p-one" {
			t.Fatal("preparation lost its project scope")
		}
		if got := authorizeTeamRoute(httptest.NewRecorder(), request, row.permissions); got != row.allowed {
			t.Fatalf("permissions%+v allowed%v want%v", row.permissions, got, row.allowed)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		request := httptest.NewRequest(method, path, nil)
		if authorizeTeamRoute(httptest.NewRecorder(), request, full) {
			t.Fatal("unclassified method admitted", method)
		}
	}
}
