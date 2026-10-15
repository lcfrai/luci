package management

import (
	"net/http/httptest"
	"testing"
)

func TestTeamControlRoutesSeparateAdminAndProjectAuthority(t *testing.T) {
	viewer := TeamPermissions{Read: true}
	editor := TeamPermissions{Read: true, Edit: true}
	admin := TeamPermissions{Read: true, Edit: true, Admin: true}
	for _, tc := range []struct {
		method, path string
		permissions  TeamPermissions
		allowed      bool
		project      string
	}{
		{"GET", "/v1/product/team-controls", viewer, true, ""},
		{"POST", "/v1/product/team-controls/preview", editor, false, ""},
		{"POST", "/v1/product/team-controls/compare", editor, false, ""},
		{"POST", "/v1/product/team-controls/import", editor, false, ""},
		{"POST", "/v1/product/team-controls/preview", admin, true, ""},
		{"POST", "/v1/product/team-controls/compare", admin, true, ""},
		{"POST", "/v1/product/team-controls/import", admin, true, ""},
		{"GET", "/v1/product/projects/p-one/control-assessment", viewer, true, "p-one"},
		{"GET", "/v1/product/projects/p-one/control-assessment/report.xlsx", viewer, true, "p-one"},
		{"POST", "/v1/product/projects/p-one/control-assessment", viewer, false, "p-one"},
		{"POST", "/v1/product/projects/p-one/control-assessment", editor, true, "p-one"},
		{"POST", "/v1/product/projects/p-one/control-assessment/gaps/SEC-1/proposal", editor, true, "p-one"},
		{"GET", "/v1/product/projects/p-one/distribute/artifacts", viewer, true, "p-one"},
		{"POST", "/v1/product/projects/p-one/distribute/artifacts/prepare", viewer, false, "p-one"},
		{"POST", "/v1/product/projects/p-one/distribute/artifacts", editor, true, "p-one"},
		{"GET", "/v1/product/projects/p-one/distribute/artifacts/artifact-one", viewer, true, "p-one"},
		{"GET", "/v1/product/projects/p-one/distribute/artifacts/artifact-one/exports/docx", viewer, true, "p-one"},
		{"POST", "/v1/product/projects/p-one/distribute/artifacts/artifact-one/revise", viewer, false, "p-one"},
		{"POST", "/v1/product/projects/p-one/distribute/artifacts/artifact-one/review", editor, true, "p-one"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		scope, capability, known := classifyTeamRoute(req)
		if !known || scope.ProjectID != tc.project {
			t.Fatalf("unclassified %s %s: %+v", tc.method, tc.path, scope)
		}
		allowed := (capability&teamRead == 0 || tc.permissions.Read) && (capability&teamEdit == 0 || tc.permissions.Edit) && (capability&teamAdmin == 0 || tc.permissions.Admin)
		if allowed != tc.allowed {
			t.Fatalf("grant mismatch %s %s: %v", tc.method, tc.path, allowed)
		}
	}
}
