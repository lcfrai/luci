package management

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTeamPermissionsKeepCapabilitiesExplicitAndProjectGrantsCannotEscalate(t *testing.T) {
	for _, role := range []string{"owner", "editor", "viewer"} {
		got := PermissionsForTeam(role, TeamGrants{})
		if !got.Read || got.Edit != (role != "viewer") || got.Execute || got.Secrets || got.Deploy || got.Admin {
			t.Fatalf("%s inherited an ungranted capability: %+v", role, got)
		}
	}
	all := TeamGrants{Execute: true, Secrets: true, Deploy: true, Admin: true}
	if got := PermissionsForTeam("super-admin", all); got != (TeamPermissions{}) {
		t.Fatalf("instance role became team authority: %+v", got)
	}
	team := PermissionsForTeam("editor", TeamGrants{Execute: true})
	project := PermissionsForTeam("owner", all)
	if got := IntersectTeamPermissions(team, project); got != team {
		t.Fatalf("project escalated team permissions: %+v", got)
	}
	if got := IntersectTeamPermissions(project, PermissionsForTeam("viewer", TeamGrants{})); got != (TeamPermissions{Read: true}) {
		t.Fatalf("project viewer received write authority: %+v", got)
	}
	if got := IntersectTeamPermissions(project, TeamPermissions{}); got != (TeamPermissions{}) {
		t.Fatalf("missing project grant received authority: %+v", got)
	}
}

func TestTeamRouteCapabilitiesSeparateContextExecutionSecretsAndDeployment(t *testing.T) {
	viewer := PermissionsForTeam("viewer", TeamGrants{})
	editor := PermissionsForTeam("editor", TeamGrants{})
	executor := PermissionsForTeam("editor", TeamGrants{Execute: true})
	secretEditor := PermissionsForTeam("editor", TeamGrants{Secrets: true})
	deployer := PermissionsForTeam("editor", TeamGrants{Deploy: true})
	full := PermissionsForTeam("owner", TeamGrants{Execute: true, Secrets: true, Deploy: true, Admin: true})
	for _, tc := range []struct {
		name, method, path string
		permissions        TeamPermissions
		allow              bool
	}{
		{"viewer can inspect own import receipt without provisioning", "GET", "/v1/product/project-imports/imp-000000000000000000000001", viewer, true},
		{"viewer cannot retry an import", "POST", "/v1/product/projects", viewer, false},
		{"viewer reads reviewed context", "GET", "/v1/product/projects/p-one/specification", viewer, true},
		{"viewer reads context attribution", "HEAD", "/v1/product/projects/p-one/context-history", viewer, true},
		{"viewer reads context storage", "GET", "/v1/product/projects/p-one/context", viewer, true},
		{"viewer reads context archive", "GET", "/v1/product/projects/p-one/context/archive", viewer, true},
		{"viewer cannot prune context history", "POST", "/v1/product/projects/p-one/context/archive/prune", viewer, false},
		{"editor prunes context history", "POST", "/v1/product/projects/p-one/context/archive/prune", editor, true},
		{"editor repairs context exports", "POST", "/v1/product/projects/p-one/context/exports/repair", editor, true},
		{"viewer reads evidence", "GET", "/v1/product/projects/p-one/runs/run-one", viewer, true},
		{"viewer cannot edit", "POST", "/v1/product/projects/p-one/brief", viewer, false},
		{"editor saves definition", "POST", "/v1/product/projects/p-one/specification", editor, true},
		{"editor records acceptance", "POST", "/v1/product/projects/p-one/runs/run-one/accept", editor, true},
		{"run reservation allocates nothing", "POST", "/v1/product/projects/p-one/runs/run-one/operations", editor, true},
		{"check join executes nothing", "POST", "/v1/product/projects/p-one/runs/run-one/checks", editor, true},
		{"editor cannot execute checks", "POST", "/v1/product/projects/p-one/checks", editor, false},
		{"executor runs checks", "POST", "/v1/product/projects/p-one/checks", executor, true},
		{"editor cannot invoke assistant", "POST", "/v1/product/projects/p-one/assist/repository", editor, false},
		{"assistant requires provider credential use", "POST", "/v1/product/projects/p-one/assist/repository", executor, false},
		{"authorized provider use invokes assistant", "POST", "/v1/product/projects/p-one/assist/repository", full, true},
		{"viewer recovers own repository request without provider authority", "GET", "/v1/product/projects/p-one/assist/repository/requests/analysis-request-001", viewer, true},
		{"repository request lookup cannot dispatch", "POST", "/v1/product/projects/p-one/assist/repository/requests/analysis-request-001", full, false},
		{"viewer reads project orchestrator", "GET", "/v1/product/projects/p-one/orchestrator", viewer, true},
		{"viewer cannot change paid finding preference", "POST", "/v1/product/projects/p-one/orchestrator/findings/preferences", viewer, false},
		{"editor cannot change paid finding preference", "POST", "/v1/product/projects/p-one/orchestrator/findings/preferences", editor, false},
		{"executor changes own paid finding preference", "POST", "/v1/product/projects/p-one/orchestrator/findings/preferences", executor, true},
		{"viewer exports project conversation", "GET", "/v1/product/projects/p-one/orchestrator/export", viewer, true},
		{"viewer cannot rotate project conversation", "POST", "/v1/product/projects/p-one/orchestrator/rotate", viewer, false},
		{"editor rotates conversation without execution", "POST", "/v1/product/projects/p-one/orchestrator/rotate", editor, true},
		{"viewer resolves own prior request without execution", "POST", "/v1/product/projects/p-one/orchestrator/resolve", viewer, true},
		{"editor resolves own request without execution or secrets", "POST", "/v1/product/projects/p-one/orchestrator/resolve", editor, true},
		{"editor reconciles retained result without execution", "POST", "/v1/product/projects/p-one/assist/assist-one/reconcile", editor, true},
		{"orchestrator requires provider credentials", "POST", "/v1/product/projects/p-one/assist/orchestrator", executor, false},
		{"authorized orchestrator inference", "POST", "/v1/product/projects/p-one/assist/orchestrator", full, true},
		{"viewer cannot apply orchestrator amendment", "POST", "/v1/product/projects/p-one/orchestrator/turns/turn-one/apply", viewer, false},
		{"editor may review context amendment", "POST", "/v1/product/projects/p-one/orchestrator/turns/turn-one/apply", editor, true},
		{"editor cannot allocate coding", "POST", "/v1/sessions", editor, false},
		{"executor allocates coding", "POST", "/v1/sessions", executor, true},
		{"viewer sees session metadata", "GET", "/v1/sessions/s-one", viewer, true},
		{"viewer inspects frozen inputs without execution or secrets", "GET", "/v1/sessions/s-one/reviewed-context", viewer, true},
		{"captured inspection cannot mutate", "POST", "/v1/sessions/s-one/reviewed-context", full, false},
		{"attach GET is writable", "GET", "/v1/sessions/s-one/attach", viewer, false},
		{"editor cannot attach terminal", "GET", "/v1/sessions/s-one/attach", editor, false},
		{"executor attaches terminal", "GET", "/v1/sessions/s-one/attach", executor, true},
		{"viewer cannot become executor through a grant", "GET", "/v1/sessions/s-one/attach", PermissionsForTeam("viewer", TeamGrants{Execute: true}), false},
		{"viewer sees sanitized configuration", "GET", "/v1/product/projects/p-one/configuration", viewer, true},
		{"executor cannot change secrets", "POST", "/v1/product/projects/p-one/configuration", executor, false},
		{"secrets grant updates configuration", "POST", "/v1/product/projects/p-one/configuration", secretEditor, true},
		{"editor cannot replace route keys", "POST", "/v1/setup/agent-connections", editor, false},
		{"project secrets grant cannot replace shared route keys", "POST", "/v1/setup/agent-connections", secretEditor, false},
		{"team admin with secrets updates route keys", "POST", "/v1/setup/agent-connections", full, true},
		{"subscription sign in needs execution too", "POST", "/v1/setup/agents/codex/sign-in", secretEditor, false},
		{"all grants allow subscription sign in", "POST", "/v1/setup/agents/codex/sign-in", full, true},
		{"execution is not deployment", "POST", "/v1/product/projects/p-one/releases", executor, false},
		{"deploy creates local release", "POST", "/v1/product/projects/p-one/releases", deployer, true},
		{"deploy publishes git", "POST", "/v1/product/projects/p-one/git/push", deployer, true},
		{"editor cannot publish git", "POST", "/v1/product/projects/p-one/git/push", editor, false},
		{"data backup exposes sensitive application state", "GET", "/v1/platform/projects/p-one/backups/backup-one", deployer, false},
		{"privileged data backup", "GET", "/v1/platform/projects/p-one/backups/backup-one", full, true},
		{"backup import needs data authority", "POST", "/v1/platform/projects/p-one/backups/import", editor, false},
		{"privileged backup import", "POST", "/v1/platform/projects/p-one/backups/import", full, true},
		{"application rows are sensitive", "GET", "/v1/platform/projects/p-one/data/tables/users/rows", viewer, false},
		{"application rows require explicit grants", "GET", "/v1/platform/projects/p-one/data/tables/users/rows", full, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authorizeTeamRoute(w, httptest.NewRequest(tc.method, tc.path, nil), tc.permissions)
			if got != tc.allow {
				t.Fatalf("allowed=%v want %v: %s", got, tc.allow, w.Body.String())
			}
			if !got && w.Code != http.StatusForbidden {
				t.Fatalf("denial status=%d", w.Code)
			}
		})
	}
}

func TestTeamRouteClassificationRequiresOwnedOpaqueLookupsAndCollectionProjection(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         TeamRouteScope
	}{
		{"GET", "/v1/projects", TeamRouteScope{Collection: "projects"}},
		{"GET", "/v1/sessions", TeamRouteScope{Collection: "sessions"}},
		{"GET", "/v1/machines", TeamRouteScope{Collection: "machines"}},
		{"HEAD", "/v1/harnesses", TeamRouteScope{Collection: "harnesses"}},
		{"GET", "/v1/projects/p-first/branches", TeamRouteScope{ProjectID: "p-first"}},
		{"PATCH", "/v1/projects/p-first/worktrees/wt-second", TeamRouteScope{ProjectID: "p-first", WorktreeID: "wt-second"}},
		{"GET", "/v1/sessions/s-opaque/attach", TeamRouteScope{SessionID: "s-opaque"}},
		{"POST", "/v1/sessions", TeamRouteScope{WorktreeFromBody: true}},
		{"POST", "/v1/product/projects", TeamRouteScope{CreatesProject: true}},
		{"POST", "/v1/product/projects/restore", TeamRouteScope{CreatesProject: true}},
		{"GET", "/v1/product/project-imports/imp-000000000000000000000001", TeamRouteScope{}},
		{"POST", "/v1/product/projects/p-first/assist/job-other/cancel", TeamRouteScope{ProjectID: "p-first"}},
		{"GET", "/v1/platform/projects/p-first/backups/backup-other", TeamRouteScope{ProjectID: "p-first"}},
		{"POST", "/v1/platform/projects/p-first/deployments/release-other/stop", TeamRouteScope{ProjectID: "p-first"}},
		{"GET", "/v1/platform/projects/p-first/data/buckets/assets/objects/nested/file.txt", TeamRouteScope{ProjectID: "p-first"}},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			got, ok := ClassifyTeamRoute(httptest.NewRequest(tc.method, tc.path, nil))
			if !ok || got != tc.want {
				t.Fatalf("scope=%+v known=%v want %+v", got, ok, tc.want)
			}
		})
	}
}

func TestTeamRoutesFailClosedForUnknownMethodsBridgesAndNoncanonicalPaths(t *testing.T) {
	full := PermissionsForTeam("owner", TeamGrants{Execute: true, Secrets: true, Deploy: true, Admin: true})
	for _, endpoint := range []string{
		"GET /v1/product/projects/p-one/future-secrets",
		"DELETE /v1/product/projects/p-one/brief",
		"POST /v1/setup/config",
		"POST /v1/setup/shutdown",
		"POST /v1/setup/workspace/activate",
		"POST /v1/setup/workspace/local",
		"POST /v1/setup/workspace/auth",
		"GET /v1/setup/workspace/admin/users",
		"POST /v1/setup/workspace/preview",
		"POST /v1/setup/cloud/create",
		"GET /v1/product/projects/p-one/releases/release-one/preview/write",
		"GET /v1/platform/projects/p-one/browser/application/release-one/write",
		"GET /v1/sessions//s-one",
		"GET /v1/sessions/../projects",
		"GET /v1/sessions/./s-one",
		"GET /v1/sessions/s%2Fone",
		"GET /v1/sessions/%73-one",
		"GET /api/v1/projects",
	} {
		t.Run(endpoint, func(t *testing.T) {
			method, url, _ := strings.Cut(endpoint, " ")
			r := httptest.NewRequest(method, url, nil)
			if _, ok := ClassifyTeamRoute(r); ok {
				t.Fatal("unsafe/unclassified route received an ownership scope")
			}
			w := httptest.NewRecorder()
			if authorizeTeamRoute(w, r, full) || w.Code != http.StatusForbidden {
				t.Fatalf("unclassified route was permitted: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestPreviewCancellationTeamRouteCapabilities(t *testing.T) {
	path := "/v1/product/projects/p-one/releases/release-one/cancel"
	for _, tc := range []struct {
		name, method string
		permissions  TeamPermissions
		allow        bool
	}{
		{"current deploy without execute", "POST", PermissionsForTeam("editor", TeamGrants{Deploy: true}), true},
		{"revoked deploy", "POST", PermissionsForTeam("editor", TeamGrants{}), false},
		{"execute cannot substitute for deploy", "POST", PermissionsForTeam("editor", TeamGrants{Execute: true}), false},
		{"viewer deploy cannot substitute for edit", "POST", PermissionsForTeam("viewer", TeamGrants{Deploy: true}), false},
		{"missing read", "POST", TeamPermissions{Edit: true, Deploy: true}, false},
		{"GET cannot cancel", "GET", PermissionsForTeam("editor", TeamGrants{Deploy: true}), false},
		{"DELETE cannot cancel", "DELETE", PermissionsForTeam("editor", TeamGrants{Deploy: true}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, path, strings.NewReader(`{"expectedRevision":7}`))
			w := httptest.NewRecorder()
			if got := authorizeTeamRoute(w, r, tc.permissions); got != tc.allow {
				t.Fatalf("cancel allowed=%v want %v: %s", got, tc.allow, w.Body.String())
			}
			if tc.allow {
				if scope, ok := ClassifyTeamRoute(r); !ok || scope != (TeamRouteScope{ProjectID: "p-one"}) {
					t.Fatalf("cancel lost exact project ownership: scope=%+v known=%v", scope, ok)
				}
			} else if w.Code != http.StatusForbidden {
				t.Fatalf("cancel denial status=%d", w.Code)
			}
		})
	}
}
