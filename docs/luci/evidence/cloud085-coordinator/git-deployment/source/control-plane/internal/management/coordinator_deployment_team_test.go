package management

import (
	"net/http/httptest"
	"testing"
)

func TestTeamCoordinatorDeploymentPreparationRequiresCurrentDeploy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		permissions TeamPermissions
		allow       bool
	}{
		{"viewer", TeamPermissions{Read: true}, false},
		{"editor", TeamPermissions{Read: true, Edit: true}, false},
		{"executor", TeamPermissions{Read: true, Edit: true, Execute: true, Secrets: true}, false},
		{"deployer", TeamPermissions{Read: true, Edit: true, Deploy: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/v1/product/projects/p-one/orchestrator/deployment-plan", nil)
			rule, ok := ClassifyTeamRoute(request)
			if !ok || rule.ProjectID != "p-one" {
				t.Fatal("missing scoped preparation route", rule, ok)
			}
			if got := authorizeTeamRoute(httptest.NewRecorder(), request, tc.permissions); got != tc.allow {
				t.Fatal("Deploy boundary", got, tc.allow)
			}
		})
	}
}
