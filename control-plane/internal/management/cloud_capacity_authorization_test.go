package management

import (
	"net/http/httptest"
	"testing"
)

func TestCloudCapacitySeparatesNewSpendFromRetainedReadAndDisable(t *testing.T) {
	control := TeamPermissions{Read: true, Edit: true, Secrets: true}
	execute := control
	execute.Execute = true
	for _, tc := range []struct {
		name, method, tail string
		permissions        TeamPermissions
		allow              bool
	}{
		{"read-without-execution", "GET", "", control, true},
		{"read-without-secrets", "GET", "", TeamPermissions{Read: true, Edit: true}, false},
		{"review-needs-execution", "POST", "", control, false},
		{"review-with-current-grants", "POST", "", execute, true},
		{"enable-needs-execution", "PUT", "", control, false},
		{"disable-after-execution-revoked", "POST", "/disable", control, true},
		{"disable-without-control", "POST", "/disable", TeamPermissions{Read: true, Secrets: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/v1/product/projects/project-one/cloud-capacity/automation"+tc.tail, nil)
			if got := authorizeTeamRoute(httptest.NewRecorder(), req, tc.permissions); got != tc.allow {
				t.Fatalf("authorization=%v, want %v", got, tc.allow)
			}
		})
	}
}
