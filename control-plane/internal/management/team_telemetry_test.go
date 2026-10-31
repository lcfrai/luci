package management

import (
	"net/http/httptest"
	"testing"
)

func TestTelemetryReadRouteRequiresCurrentProjectReadEditDeployAndSecrets(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/product/projects/p-one/kubernetes/deployments/workload-one/telemetry/read", nil)
	scope, required, ok := classifyTeamRoute(r)
	if !ok || scope.ProjectID != "p-one" || required != (teamRead|teamEdit|teamDeploy|teamSecrets) {
		t.Fatal("telemetry read granted ambient project/secret authority", scope, required, ok)
	}
	for _, method := range []string{"GET", "PUT", "PATCH", "DELETE"} {
		if _, _, ok := classifyTeamRoute(httptest.NewRequest(method, r.URL.Path, nil)); ok {
			t.Fatal("unexpected telemetry mutation/read alias accepted", method)
		}
	}
}
