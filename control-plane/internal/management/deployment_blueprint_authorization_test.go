package management

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSavedBlueprintTeamRoutesSeparateReadingEditingAndDeployment(t *testing.T) {
	path := "/v1/product/projects/p-one/deployment-blueprint/saved"
	for _, row := range []struct {
		method      string
		permissions TeamPermissions
		allowed     bool
	}{
		{http.MethodGet, TeamPermissions{Read: true}, true},
		{http.MethodPut, TeamPermissions{Read: true}, false},
		{http.MethodPut, TeamPermissions{Read: true, Edit: true}, true},
		{http.MethodPut, TeamPermissions{Read: true, Deploy: true, Execute: true}, false},
		{http.MethodDelete, TeamPermissions{Read: true, Edit: true, Deploy: true}, false},
	} {
		r := httptest.NewRequest(row.method, path, nil)
		w := httptest.NewRecorder()
		if got := authorizeTeamRoute(w, r, row.permissions); got != row.allowed {
			t.Fatalf("%s permissions%+v allowed%v want%v", row.method, row.permissions, got, row.allowed)
		}
	}
}
