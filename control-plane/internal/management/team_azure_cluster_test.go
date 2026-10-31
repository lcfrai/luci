package management

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAzureClusterTeamRoutesRequireIndependentDeployAndSecretsWithoutExecutionGrant(t *testing.T) {
	for _, endpoint := range []string{"GET /v1/product/projects/p-one/azure-clusters/commissions", "POST /v1/product/projects/p-one/azure-clusters/review", "POST /v1/product/projects/p-one/azure-clusters/commissions/aks-original/access", "POST /v1/product/projects/p-one/azure-clusters/commissions/aks-original/namespace", "POST /v1/product/projects/p-one/azure-clusters/commissions/aks-original/observe"} {
		method, path, _ := strings.Cut(endpoint, " ")
		full := TeamPermissions{Read: true, Edit: true, Deploy: true, Secrets: true}
		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()
		if !authorizeTeamRoute(w, req, full) {
			t.Fatal("complete independent deployment grant refused", endpoint, w.Body.String())
		}
		scope, ok := ClassifyTeamRoute(req)
		if !ok || scope != (TeamRouteScope{ProjectID: "p-one"}) {
			t.Fatal("project scope lost", endpoint, scope)
		}
		for _, p := range []TeamPermissions{{Edit: true, Deploy: true, Secrets: true}, {Read: true, Deploy: true, Secrets: true}, {Read: true, Edit: true, Execute: true, Secrets: true}, {Read: true, Edit: true, Deploy: true, Execute: true}} {
			req := httptest.NewRequest(method, path, nil)
			w := httptest.NewRecorder()
			if authorizeTeamRoute(w, req, p) {
				t.Fatal("missing read/edit/deploy/secrets was substituted", endpoint, p)
			}
		}
	}
}
