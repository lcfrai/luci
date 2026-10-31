package management

import (
	"net/http/httptest"
	"testing"
)

func TestKubernetesTeamRoutesRequireIndependentDeployAndSecretSetupGrants(t *testing.T) {
	base := "/v1/product/projects/p-one/kubernetes/"
	for _, row := range []struct {
		method, path string
		permissions  TeamPermissions
		allowed      bool
	}{
		{"GET", "targets", TeamPermissions{Read: true}, false},
		{"GET", "targets", TeamPermissions{Read: true, Edit: true, Deploy: true}, true},
		{"PUT", "targets", TeamPermissions{Read: true, Edit: true, Deploy: true}, false},
		{"PUT", "targets", TeamPermissions{Read: true, Edit: true, Deploy: true, Secrets: true}, true},
		{"POST", "review", TeamPermissions{Read: true, Edit: true, Execute: true, Secrets: true}, false},
		{"POST", "deployments/workload-one/apply", TeamPermissions{Read: true, Edit: true, Deploy: true}, true},
		{"POST", "deployments/workload-one/observe", TeamPermissions{Read: true, Edit: true, Deploy: true}, true},
		{"DELETE", "deployments/workload-one", TeamPermissions{Read: true, Edit: true, Deploy: true, Secrets: true}, false},
	} {
		r := httptest.NewRequest(row.method, base+row.path, nil)
		w := httptest.NewRecorder()
		if got := authorizeTeamRoute(w, r, row.permissions); got != row.allowed {
			t.Fatal("independent cluster authority widened", row.method, row.path, row.permissions, got)
		}
	}
}
